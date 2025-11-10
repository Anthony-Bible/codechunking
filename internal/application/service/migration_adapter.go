package service

import (
	"codechunking/internal/application/common/slogger"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PathMigrationRepositoryAdapter adapts the existing repository functions to work with the new interface
// This provides backward compatibility for legacy functions.
type PathMigrationRepositoryAdapter struct {
	pool   *pgxpool.Pool
	config *MigrationConfig
}

// NewPathMigrationRepositoryAdapter creates a new adapter for legacy compatibility.
func NewPathMigrationRepositoryAdapter(pool *pgxpool.Pool) MigrationRepositoryInterface {
	config := DefaultMigrationConfig()
	return &PathMigrationRepositoryAdapter{
		pool:   pool,
		config: config,
	}
}

// GetUUIDPrefixedPaths retrieves all chunks with UUID-prefixed paths.
func (a *PathMigrationRepositoryAdapter) GetUUIDPrefixedPaths(ctx context.Context) ([]PathMigrationRecord, error) {
	// Set search path
	_, err := a.pool.Exec(ctx, "SET search_path TO codechunking, public")
	if err != nil {
		return nil, NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to set search path", err)
	}

	query := `
		SELECT id, repository_id, file_path, created_at
		FROM code_chunks 
		WHERE file_path LIKE '/tmp/codechunking-workspace/%'
		ORDER BY created_at
	`

	rows, err := a.pool.Query(ctx, query)
	if err != nil {
		return nil, NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to query UUID paths", err)
	}
	defer rows.Close()

	var migrations []PathMigrationRecord
	for rows.Next() {
		var m PathMigrationRecord
		err := rows.Scan(&m.ChunkID, &m.RepoID, &m.OldPath, &m.CreatedAt)
		if err != nil {
			slogger.Warn(ctx, "Failed to scan migration row", slogger.Fields3(
				"error", err.Error(),
				"operation", "get_uuid_paths",
				"chunk_id", m.ChunkID.String(),
			))
			continue
		}
		m.Status = "pending"
		migrations = append(migrations, m)
	}

	return migrations, nil
}

// GetUUIDPrefixedPathsForRepo retrieves UUID-prefixed paths for a specific repository.
func (a *PathMigrationRepositoryAdapter) GetUUIDPrefixedPathsForRepo(
	ctx context.Context,
	repoID uuid.UUID,
) ([]PathMigrationRecord, error) {
	// Set search path
	_, err := a.pool.Exec(ctx, "SET search_path TO codechunking, public")
	if err != nil {
		return nil, NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to set search path", err)
	}

	query := `
		SELECT id, repository_id, file_path, created_at
		FROM code_chunks 
		WHERE repository_id = $1 AND file_path LIKE '/tmp/codechunking-workspace/%'
		ORDER BY created_at
	`

	rows, err := a.pool.Query(ctx, query, repoID)
	if err != nil {
		return nil, NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to query repository UUID paths", err)
	}
	defer rows.Close()

	var migrations []PathMigrationRecord
	for rows.Next() {
		var m PathMigrationRecord
		err := rows.Scan(&m.ChunkID, &m.RepoID, &m.OldPath, &m.CreatedAt)
		if err != nil {
			slogger.Warn(ctx, "Failed to scan repository migration row", slogger.Fields3(
				"error", err.Error(),
				"repo_id", repoID.String(),
				"chunk_id", m.ChunkID.String(),
			))
			continue
		}
		m.Status = "pending"
		migrations = append(migrations, m)
	}

	return migrations, nil
}

// BatchUpdatePaths updates paths in batches.
func (a *PathMigrationRepositoryAdapter) BatchUpdatePaths(
	ctx context.Context,
	updates []PathUpdateRecord,
) (*MigrationStats, error) {
	stats := &MigrationStats{
		StartTime:   time.Now(),
		TotalChunks: len(updates),
	}

	// Set search path
	_, err := a.pool.Exec(ctx, "SET search_path TO codechunking, public")
	if err != nil {
		return stats, NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to set search path", err)
	}

	// Process updates
	for _, update := range updates {
		result, err := a.pool.Exec(
			ctx,
			"UPDATE code_chunks SET file_path = $1 WHERE id = $2",
			update.NewPath,
			update.ChunkID,
		)
		if err != nil {
			stats.FailedChunks++
			stats.AddError(
				*NewMigrationErrorWithChunk(ErrorTypeDatabase, "failed to update chunk", update.ChunkID, update.RepoID),
			)
			continue
		}

		if result.RowsAffected() > 0 {
			stats.MigratedChunks++
		} else {
			stats.SkippedChunks++
		}
	}

	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	return stats, nil
}

// TransactionalPathMigration performs migration within a single transaction.
func (a *PathMigrationRepositoryAdapter) TransactionalPathMigration(
	ctx context.Context,
	updates []PathUpdateRecord,
) error {
	// Set search path
	_, err := a.pool.Exec(ctx, "SET search_path TO codechunking, public")
	if err != nil {
		return NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to set search path", err)
	}

	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return NewMigrationErrorWithCause(ErrorTypeTransaction, "failed to begin transaction", err)
	}
	defer tx.Rollback(ctx)

	for _, update := range updates {
		_, err := tx.Exec(ctx, "UPDATE code_chunks SET file_path = $1 WHERE id = $2", update.NewPath, update.ChunkID)
		if err != nil {
			return NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to update chunk in transaction", err)
		}
	}

	return tx.Commit(ctx)
}

// GetMigrationProgress returns migration progress information.
func (a *PathMigrationRepositoryAdapter) GetMigrationProgress(ctx context.Context) (*MigrationProgress, error) {
	// Set search path
	_, err := a.pool.Exec(ctx, "SET search_path TO codechunking, public")
	if err != nil {
		return nil, NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to set search path", err)
	}

	progress := &MigrationProgress{
		StartTime: time.Now(),
	}

	// Get total chunks with UUID paths
	uuidPattern := "/tmp/codechunking-workspace/%"

	err = a.pool.QueryRow(ctx, "SELECT COUNT(*) FROM code_chunks WHERE file_path LIKE $1", uuidPattern).
		Scan(&progress.TotalChunks)
	if err != nil {
		return nil, NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to count total chunks", err)
	}

	// For now, assume no chunks migrated yet
	progress.MigratedChunks = 0
	progress.FailedChunks = 0

	return progress, nil
}

// ValidateMigrationData validates migration data before processing.
func (a *PathMigrationRepositoryAdapter) ValidateMigrationData(ctx context.Context) (*MigrationValidation, error) {
	// Set search path
	_, err := a.pool.Exec(ctx, "SET search_path TO codechunking, public")
	if err != nil {
		return nil, NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to set search path", err)
	}

	validation := &MigrationValidation{}

	// Count valid chunks (non-empty paths)
	err = a.pool.QueryRow(ctx, "SELECT COUNT(*) FROM code_chunks WHERE file_path IS NOT NULL AND file_path != ''").
		Scan(&validation.ValidChunks)
	if err != nil {
		return nil, NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to count valid chunks", err)
	}

	// Count invalid chunks (empty or null paths)
	err = a.pool.QueryRow(ctx, "SELECT COUNT(*) FROM code_chunks WHERE file_path IS NULL OR file_path = ''").
		Scan(&validation.InvalidChunks)
	if err != nil {
		return nil, NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to count invalid chunks", err)
	}

	// Add validation errors
	if validation.InvalidChunks > 0 {
		validation.Errors = append(
			validation.Errors,
			fmt.Sprintf("Found %d chunks with empty or null paths", validation.InvalidChunks),
		)
	}

	return validation, nil
}

// CreateBackup creates a backup of original paths before migration.
func (a *PathMigrationRepositoryAdapter) CreateBackup(
	ctx context.Context,
	repoID *uuid.UUID,
) (map[string]string, error) {
	// Set search path
	_, err := a.pool.Exec(ctx, "SET search_path TO codechunking, public")
	if err != nil {
		return nil, NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to set search path", err)
	}

	backup := make(map[string]string)

	var query string
	var args []interface{}

	if repoID != nil {
		query = "SELECT id, file_path FROM code_chunks WHERE repository_id = $1 AND file_path LIKE '/tmp/codechunking-workspace/%'"
		args = []interface{}{*repoID}
	} else {
		query = "SELECT id, file_path FROM code_chunks WHERE file_path LIKE '/tmp/codechunking-workspace/%'"
		args = []interface{}{}
	}

	rows, err := a.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to query backup data", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var path string
		err := rows.Scan(&id, &path)
		if err != nil {
			slogger.Warn(ctx, "Failed to scan backup row", slogger.Fields3(
				"error", err.Error(),
				"operation", "create_backup",
				"chunk_id", id.String(),
			))
			continue
		}
		backup[id.String()] = path
	}

	return backup, nil
}

// RestoreFromBackup restores original paths from backup data.
func (a *PathMigrationRepositoryAdapter) RestoreFromBackup(ctx context.Context, backupData map[string]string) error {
	// Set search path
	_, err := a.pool.Exec(ctx, "SET search_path TO codechunking, public")
	if err != nil {
		return NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to set search path", err)
	}

	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return NewMigrationErrorWithCause(ErrorTypeTransaction, "failed to begin restore transaction", err)
	}
	defer tx.Rollback(ctx)

	for chunkIDStr, originalPath := range backupData {
		chunkID, err := uuid.Parse(chunkIDStr)
		if err != nil {
			slogger.Warn(ctx, "Invalid chunk ID in backup", slogger.Fields3(
				"error", err.Error(),
				"operation", "restore_backup",
				"chunk_id_str", chunkIDStr,
			))
			continue
		}

		_, err = tx.Exec(ctx, "UPDATE code_chunks SET file_path = $1 WHERE id = $2", originalPath, chunkID)
		if err != nil {
			return NewMigrationErrorWithCause(ErrorTypeDatabase, "failed to restore chunk", err)
		}
	}

	return tx.Commit(ctx)
}

// AcquireMigrationLock acquires an advisory lock for migration.
func (a *PathMigrationRepositoryAdapter) AcquireMigrationLock(ctx context.Context) (bool, error) {
	const lockKey = 12345

	var lockAcquired bool
	err := a.pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockKey).Scan(&lockAcquired)
	if err != nil {
		return false, NewMigrationErrorWithCause(ErrorTypeLock, "failed to try migration lock", err)
	}

	if !lockAcquired {
		return false, NewMigrationError(ErrorTypeLock, "migration already in progress")
	}

	return true, nil
}

// ReleaseMigrationLock releases the advisory lock.
func (a *PathMigrationRepositoryAdapter) ReleaseMigrationLock(ctx context.Context) error {
	const lockKey = 12345

	_, err := a.pool.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockKey)
	if err != nil {
		return NewMigrationErrorWithCause(ErrorTypeLock, "failed to release migration lock", err)
	}

	return nil
}
