package repository

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/service"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MigrationRepository defines the interface for path migration operations.
type MigrationRepository interface {
	service.MigrationRepositoryInterface
}

// PathMigration represents a path migration record.
type PathMigration struct {
	ChunkID   uuid.UUID
	RepoID    uuid.UUID
	OldPath   string
	NewPath   string
	Status    string
	CreatedAt time.Time
	// Enhanced fields for better tracking
	RepositoryName   string   `json:"repository_name,omitempty"`
	ValidationErrors []string `json:"validation_errors,omitempty"`
}

// PathUpdate represents a path update operation.
type PathUpdate struct {
	ChunkID uuid.UUID
	RepoID  uuid.UUID
	OldPath string
	NewPath string
}

// PathMigrationRepository implements MigrationRepository.
type PathMigrationRepository struct {
	pool   *pgxpool.Pool
	config *service.MigrationConfig
	// Enhanced pattern matching
	uuidPattern      *regexp.Regexp
	workspacePattern *regexp.Regexp
}

// NewPathMigrationRepository creates a new PathMigrationRepository.
func NewPathMigrationRepository(pool *pgxpool.Pool, config *service.MigrationConfig) *PathMigrationRepository {
	if config == nil {
		config = service.DefaultMigrationConfig()
	}

	// Compile enhanced UUID pattern for better validation
	uuidPattern := regexp.MustCompile(
		`^/tmp/codechunking-workspace/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/(.+)$`,
	)

	// Compile workspace pattern for repository name extraction
	workspacePattern := regexp.MustCompile(
		`^/tmp/codechunking-workspace/([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})/(.+)$`,
	)

	return &PathMigrationRepository{
		pool:             pool,
		config:           config,
		uuidPattern:      uuidPattern,
		workspacePattern: workspacePattern,
	}
}

// GetUUIDPrefixedPaths retrieves all chunks with UUID-prefixed paths.
func (r *PathMigrationRepository) GetUUIDPrefixedPaths(ctx context.Context) ([]service.PathMigrationRecord, error) {
	slogger.Info(
		ctx,
		"Retrieving UUID-prefixed paths with enhanced detection",
		slogger.Field("operation", "get_uuid_paths"),
	)

	if err := r.setSearchPath(ctx); err != nil {
		slogger.Error(ctx, "Failed to set search path", slogger.Fields2(
			"error", err.Error(),
			"operation", "get_uuid_paths",
		))
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to set search path", err)
	}

	// Enhanced query with better pattern matching
	query := `
		SELECT id, repository_id, file_path, created_at
		FROM code_chunks 
		WHERE file_path ~ '^/tmp/codechunking-workspace/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/.*'
		ORDER BY created_at
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		slogger.Error(ctx, "Failed to query UUID-prefixed paths", slogger.Fields2(
			"error", err.Error(),
			"operation", "get_uuid_paths",
		))
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to query UUID paths", err)
	}
	defer rows.Close()

	var migrations []service.PathMigrationRecord
	for rows.Next() {
		var m service.PathMigrationRecord
		err := rows.Scan(&m.ChunkID, &m.RepoID, &m.OldPath, &m.CreatedAt)
		if err != nil {
			slogger.Warn(ctx, "Failed to scan migration row", slogger.Fields3(
				"error", err.Error(),
				"operation", "get_uuid_paths",
				"chunk_id", m.ChunkID.String(),
			))
			continue
		}

		// Enhanced path processing
		relativePath, _, validationErrors := r.extractPathComponents(ctx, m.OldPath)
		m.NewPath = relativePath

		// Set status based on validation
		if len(validationErrors) > 0 {
			m.Status = "validation_failed"
			slogger.Warn(ctx, "Path validation failed", slogger.Fields3(
				"chunk_id", m.ChunkID.String(),
				"old_path", m.OldPath,
				"errors", strings.Join(validationErrors, ", "),
			))
		} else {
			m.Status = "pending"
		}

		migrations = append(migrations, m)
	}

	if err := rows.Err(); err != nil {
		slogger.Error(ctx, "Row iteration error", slogger.Fields2(
			"error", err.Error(),
			"operation", "get_uuid_paths",
		))
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "row iteration error", err)
	}

	slogger.Info(ctx, "Retrieved UUID-prefixed paths with enhanced processing", slogger.Fields2(
		"count", len(migrations),
		"operation", "get_uuid_paths",
	))

	return migrations, nil
}

// GetUUIDPrefixedPathsForRepo retrieves UUID-prefixed paths for a specific repository.
func (r *PathMigrationRepository) GetUUIDPrefixedPathsForRepo(
	ctx context.Context,
	repoID uuid.UUID,
) ([]service.PathMigrationRecord, error) {
	slogger.Info(ctx, "Retrieving UUID-prefixed paths for repository", slogger.Fields2(
		"repo_id", repoID.String(),
		"operation", "get_uuid_paths_repo",
	))

	if err := r.setSearchPath(ctx); err != nil {
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to set search path", err)
	}

	query := `
		SELECT id, repository_id, file_path, created_at
		FROM code_chunks 
		WHERE repository_id = $1 AND file_path ~ '^/tmp/codechunking-workspace/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/.*'
		ORDER BY created_at
	`

	rows, err := r.pool.Query(ctx, query, repoID)
	if err != nil {
		slogger.Error(ctx, "Failed to query repository UUID paths", slogger.Fields3(
			"error", err.Error(),
			"repo_id", repoID.String(),
			"operation", "get_uuid_paths_repo",
		))
		return nil, service.NewMigrationErrorWithCause(
			service.ErrorTypeDatabase,
			"failed to query repository UUID paths",
			err,
		)
	}
	defer rows.Close()

	var migrations []service.PathMigrationRecord
	for rows.Next() {
		var m service.PathMigrationRecord
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

	if err := rows.Err(); err != nil {
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "row iteration error", err)
	}

	slogger.Info(ctx, "Retrieved repository UUID paths", slogger.Fields3(
		"repo_id", repoID.String(),
		"count", len(migrations),
		"operation", "get_uuid_paths_repo",
	))

	return migrations, nil
}

// BatchUpdatePaths updates paths in batches with proper error handling and statistics.
func (r *PathMigrationRepository) BatchUpdatePaths(
	ctx context.Context,
	updates []service.PathUpdateRecord,
) (*service.MigrationStats, error) {
	stats := &service.MigrationStats{
		StartTime:   time.Now(),
		TotalChunks: len(updates),
	}

	slogger.Info(ctx, "Starting batch path updates", slogger.Fields3(
		"total_updates", len(updates),
		"batch_size", r.config.BatchSize,
		"operation", "batch_update_paths",
	))

	if err := r.setSearchPath(ctx); err != nil {
		return stats, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to set search path", err)
	}

	// Process in batches
	for i := 0; i < len(updates); i += r.config.BatchSize {
		end := i + r.config.BatchSize
		if end > len(updates) {
			end = len(updates)
		}

		batch := updates[i:end]
		batchStats, err := r.processBatch(ctx, batch)
		if err != nil {
			stats.AddError(
				*service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "batch processing failed", err),
			)
			return stats, err
		}

		stats.MigratedChunks += batchStats.MigratedChunks
		stats.FailedChunks += batchStats.FailedChunks
		stats.SkippedChunks += batchStats.SkippedChunks
		stats.BatchesProcessed++

		slogger.Debug(ctx, "Processed batch", slogger.Fields3(
			"batch_index", i/r.config.BatchSize,
			"batch_size", len(batch),
			"migrated", batchStats.MigratedChunks,
		))

		stats.EndTime = time.Now()
		stats.Duration = stats.EndTime.Sub(stats.StartTime)

		slogger.Info(ctx, "Completed batch path updates", slogger.Fields{
			"total_migrated": stats.MigratedChunks,
			"total_failed":   stats.FailedChunks,
			"total_skipped":  stats.SkippedChunks,
			"duration_ms":    stats.Duration.Milliseconds(),
		})
	}

	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	slogger.Info(ctx, "Completed batch path updates", slogger.Fields{
		"total_migrated": stats.MigratedChunks,
		"total_failed":   stats.FailedChunks,
		"total_skipped":  stats.SkippedChunks,
		"duration_ms":    stats.Duration.Milliseconds(),
	})

	return stats, nil
}

// TransactionalPathMigration performs migration within a single transaction.
func (r *PathMigrationRepository) TransactionalPathMigration(
	ctx context.Context,
	updates []service.PathUpdateRecord,
) error {
	slogger.Info(ctx, "Starting transactional path migration", slogger.Fields2(
		"total_updates", len(updates),
		"operation", "transactional_migration",
	))

	if err := r.setSearchPath(ctx); err != nil {
		return service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to set search path", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		slogger.Error(ctx, "Failed to begin transaction", slogger.Fields2(
			"error", err.Error(),
			"operation", "transactional_migration",
		))
		return service.NewMigrationErrorWithCause(service.ErrorTypeTransaction, "failed to begin transaction", err)
	}
	defer tx.Rollback(ctx)

	// Process all updates in the transaction
	migrated := 0
	failed := 0

	for _, update := range updates {
		result, err := tx.Exec(
			ctx,
			"UPDATE code_chunks SET file_path = $1 WHERE id = $2",
			update.NewPath,
			update.ChunkID,
		)
		if err != nil {
			failed++
			slogger.Warn(ctx, "Failed to update chunk in transaction", slogger.Fields3(
				"error", err.Error(),
				"chunk_id", update.ChunkID.String(),
				"old_path", update.OldPath,
			))
			continue
		}

		if result.RowsAffected() > 0 {
			migrated++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		slogger.Error(ctx, "Failed to commit transaction", slogger.Fields3(
			"error", err.Error(),
			"migrated", migrated,
			"failed", failed,
		))
		return service.NewMigrationErrorWithCause(service.ErrorTypeTransaction, "failed to commit transaction", err)
	}

	slogger.Info(ctx, "Transaction completed successfully", slogger.Fields3(
		"migrated", migrated,
		"failed", failed,
		"operation", "transactional_migration",
	))

	return nil
}

// GetMigrationProgress returns migration progress information with enhanced tracking.
func (r *PathMigrationRepository) GetMigrationProgress(ctx context.Context) (*service.MigrationProgress, error) {
	slogger.Debug(ctx, "Getting migration progress", slogger.Field("operation", "get_progress"))

	if err := r.setSearchPath(ctx); err != nil {
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to set search path", err)
	}

	progress := &service.MigrationProgress{
		StartTime: time.Now(),
	}

	// Get total chunks with UUID paths using regex pattern
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM code_chunks WHERE file_path ~ '^/tmp/codechunking-workspace/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/.*'").
		Scan(&progress.TotalChunks)
	if err != nil {
		slogger.Error(ctx, "Failed to count total chunks", slogger.Fields2(
			"error", err.Error(),
			"operation", "get_progress",
		))
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to count total chunks", err)
	}

	// Get chunks that have been migrated (paths without UUID prefix)
	err = r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM code_chunks WHERE file_path NOT ~ '^/tmp/codechunking-workspace/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/.*' AND file_path IS NOT NULL AND file_path != ''").
		Scan(&progress.MigratedChunks)
	if err != nil {
		slogger.Error(ctx, "Failed to count migrated chunks", slogger.Fields2(
			"error", err.Error(),
			"operation", "get_progress",
		))
		return nil, service.NewMigrationErrorWithCause(
			service.ErrorTypeDatabase,
			"failed to count migrated chunks",
			err,
		)
	}

	// Calculate failed chunks (total - migrated, but only counting originally UUID paths)
	if progress.MigratedChunks > progress.TotalChunks {
		progress.MigratedChunks = progress.TotalChunks
	}
	progress.FailedChunks = progress.TotalChunks - progress.MigratedChunks

	// Calculate progress percentage
	if progress.TotalChunks > 0 {
		progress.ProgressPercentage = float64(progress.MigratedChunks) / float64(progress.TotalChunks) * 100
	}

	slogger.Debug(ctx, "Migration progress retrieved", slogger.Fields{
		"total_chunks":        progress.TotalChunks,
		"migrated_chunks":     progress.MigratedChunks,
		"failed_chunks":       progress.FailedChunks,
		"progress_percentage": progress.ProgressPercentage,
	})

	return progress, nil
}

// ValidateMigrationData validates migration data before processing.
func (r *PathMigrationRepository) ValidateMigrationData(ctx context.Context) (*service.MigrationValidation, error) {
	slogger.Debug(ctx, "Validating migration data", slogger.Field("operation", "validate_data"))

	if err := r.setSearchPath(ctx); err != nil {
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to set search path", err)
	}

	validation := &service.MigrationValidation{}

	// Count valid chunks (non-empty paths)
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM code_chunks WHERE file_path IS NOT NULL AND file_path != ''").
		Scan(&validation.ValidChunks)
	if err != nil {
		slogger.Error(ctx, "Failed to count valid chunks", slogger.Fields2(
			"error", err.Error(),
			"operation", "validate_data",
		))
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to count valid chunks", err)
	}

	// Count invalid chunks (empty or null paths)
	err = r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM code_chunks WHERE file_path IS NULL OR file_path = ''").
		Scan(&validation.InvalidChunks)
	if err != nil {
		slogger.Error(ctx, "Failed to count invalid chunks", slogger.Fields2(
			"error", err.Error(),
			"operation", "validate_data",
		))
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to count invalid chunks", err)
	}

	// Add validation errors
	if validation.InvalidChunks > 0 {
		validation.Errors = append(
			validation.Errors,
			fmt.Sprintf("Found %d chunks with empty or null paths", validation.InvalidChunks),
		)
	}

	slogger.Info(ctx, "Migration data validation completed", slogger.Fields3(
		"valid_chunks", validation.ValidChunks,
		"invalid_chunks", validation.InvalidChunks,
		"operation", "validate_data",
	))

	return validation, nil
}

// CreateBackup creates a backup of original paths before migration with enhanced validation.
func (r *PathMigrationRepository) CreateBackup(ctx context.Context, repoID *uuid.UUID) (map[string]string, error) {
	slogger.Info(ctx, "Creating migration backup", slogger.Fields2(
		"repo_id", func() string {
			if repoID != nil {
				return repoID.String()
			}
			return "all"
		}(),
		"operation", "create_backup",
	))

	if err := r.setSearchPath(ctx); err != nil {
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to set search path", err)
	}

	backup := make(map[string]string)

	var query string
	var args []interface{}

	if repoID != nil {
		query = "SELECT id, file_path FROM code_chunks WHERE repository_id = $1 AND file_path ~ '^/tmp/codechunking-workspace/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/.*'"
		args = []interface{}{*repoID}
	} else {
		query = "SELECT id, file_path FROM code_chunks WHERE file_path ~ '^/tmp/codechunking-workspace/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/.*'"
		args = []interface{}{}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		slogger.Error(ctx, "Failed to query backup data", slogger.Fields2(
			"error", err.Error(),
			"operation", "create_backup",
		))
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to query backup data", err)
	}
	defer rows.Close()

	backupCount := 0
	skippedCount := 0

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
			skippedCount++
			continue
		}

		// Validate the path before adding to backup
		if validationErrors := r.validateUUIDPath(ctx, path); len(validationErrors) > 0 {
			slogger.Warn(ctx, "Skipping invalid path in backup", slogger.Fields3(
				"chunk_id", id.String(),
				"path", path,
				"validation_errors", strings.Join(validationErrors, ", "),
			))
			skippedCount++
			continue
		}

		backup[id.String()] = path
		backupCount++
	}

	if err := rows.Err(); err != nil {
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "backup row iteration error", err)
	}

	slogger.Info(ctx, "Migration backup created", slogger.Fields3(
		"backup_size", len(backup),
		"backup_count", backupCount,
		"skipped_count", skippedCount,
	))

	return backup, nil
}

// RestoreFromBackup restores original paths from backup data.
func (r *PathMigrationRepository) RestoreFromBackup(ctx context.Context, backupData map[string]string) error {
	slogger.Info(ctx, "Restoring from backup", slogger.Fields2(
		"backup_size", len(backupData),
		"operation", "restore_backup",
	))

	if err := r.setSearchPath(ctx); err != nil {
		return service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to set search path", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		slogger.Error(ctx, "Failed to begin restore transaction", slogger.Fields2(
			"error", err.Error(),
			"operation", "restore_backup",
		))
		return service.NewMigrationErrorWithCause(
			service.ErrorTypeTransaction,
			"failed to begin restore transaction",
			err,
		)
	}
	defer tx.Rollback(ctx)

	restored := 0
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
			slogger.Error(ctx, "Failed to restore chunk", slogger.Fields3(
				"error", err.Error(),
				"operation", "restore_backup",
				"chunk_id", chunkID.String(),
			))
			return service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to restore chunk", err)
		}
		restored++
	}

	if err := tx.Commit(ctx); err != nil {
		slogger.Error(ctx, "Failed to commit restore transaction", slogger.Fields2(
			"error", err.Error(),
			"restored_count", restored,
		))
		return service.NewMigrationErrorWithCause(
			service.ErrorTypeTransaction,
			"failed to commit restore transaction",
			err,
		)
	}

	slogger.Info(ctx, "Backup restoration completed", slogger.Fields2(
		"restored_count", restored,
		"operation", "restore_backup",
	))

	return nil
}

// AcquireMigrationLock acquires an advisory lock for migration.
func (r *PathMigrationRepository) AcquireMigrationLock(ctx context.Context) (bool, error) {
	const lockKey = 12345

	var lockAcquired bool
	err := r.pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockKey).Scan(&lockAcquired)
	if err != nil {
		slogger.Error(ctx, "Failed to acquire migration lock", slogger.Fields2(
			"error", err.Error(),
			"lock_key", lockKey,
		))
		return false, service.NewMigrationErrorWithCause(service.ErrorTypeLock, "failed to try migration lock", err)
	}

	if !lockAcquired {
		slogger.Warn(ctx, "Migration already in progress", slogger.Field("lock_key", lockKey))
		return false, service.NewMigrationError(service.ErrorTypeLock, "migration already in progress")
	}

	slogger.Info(ctx, "Migration lock acquired", slogger.Field("lock_key", lockKey))
	return true, nil
}

// ReleaseMigrationLock releases the advisory lock.
func (r *PathMigrationRepository) ReleaseMigrationLock(ctx context.Context) error {
	const lockKey = 12345

	_, err := r.pool.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockKey)
	if err != nil {
		slogger.Error(ctx, "Failed to release migration lock", slogger.Fields2(
			"error", err.Error(),
			"lock_key", lockKey,
		))
		return service.NewMigrationErrorWithCause(service.ErrorTypeLock, "failed to release migration lock", err)
	}

	slogger.Info(ctx, "Migration lock released", slogger.Field("lock_key", lockKey))
	return nil
}

// Helper methods

// setSearchPath sets the database search path.
func (r *PathMigrationRepository) setSearchPath(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, "SET search_path TO codechunking, public")
	return err
}

// processBatch processes a batch of updates with enhanced error handling.
func (r *PathMigrationRepository) processBatch(
	ctx context.Context,
	batch []service.PathUpdateRecord,
) (*service.MigrationStats, error) {
	stats := &service.MigrationStats{}

	for i, update := range batch {
		// Validate update data before processing
		if validationErrors := r.validatePathUpdate(ctx, update); len(validationErrors) > 0 {
			stats.FailedChunks++
			stats.AddError(
				*service.NewMigrationErrorWithChunk(service.ErrorTypeValidation, fmt.Sprintf("validation failed: %v", validationErrors), update.ChunkID, update.RepoID),
			)
			slogger.Warn(ctx, "Skipping invalid update in batch", slogger.Fields3(
				"batch_index", i,
				"chunk_id", update.ChunkID.String(),
				"validation_errors", strings.Join(validationErrors, ", "),
			))
			continue
		}

		result, err := r.pool.Exec(
			ctx,
			"UPDATE code_chunks SET file_path = $1 WHERE id = $2",
			update.NewPath,
			update.ChunkID,
		)
		if err != nil {
			stats.FailedChunks++
			stats.AddError(
				*service.NewMigrationErrorWithChunk(service.ErrorTypeDatabase, fmt.Sprintf("failed to update chunk: %v", err), update.ChunkID, update.RepoID),
			)
			slogger.Error(ctx, "Failed to update chunk in batch", slogger.Fields3(
				"error", err.Error(),
				"chunk_id", update.ChunkID.String(),
				"operation", "process_batch",
			))
			continue
		}

		if result.RowsAffected() > 0 {
			stats.MigratedChunks++
			slogger.Debug(ctx, "Successfully updated chunk", slogger.Fields3(
				"chunk_id", update.ChunkID.String(),
				"new_path", update.NewPath,
				"operation", "process_batch",
			))
		} else {
			stats.SkippedChunks++
			slogger.Debug(ctx, "No rows affected for chunk update", slogger.Fields2(
				"chunk_id", update.ChunkID.String(),
				"operation", "process_batch",
			))
		}
	}

	return stats, nil
}

// validatePathUpdate validates a single path update before processing.
func (r *PathMigrationRepository) validatePathUpdate(ctx context.Context, update service.PathUpdateRecord) []string {
	var errors []string

	// Validate chunk ID
	if update.ChunkID == uuid.Nil {
		errors = append(errors, "invalid chunk ID (nil UUID)")
	}

	// Validate repository ID
	if update.RepoID == uuid.Nil {
		errors = append(errors, "invalid repository ID (nil UUID)")
	}

	// Validate paths
	if strings.TrimSpace(update.OldPath) == "" {
		errors = append(errors, "old path is empty")
	}

	if strings.TrimSpace(update.NewPath) == "" {
		errors = append(errors, "new path is empty")
	}

	// Validate new path format (should not contain UUID prefix)
	if r.uuidPattern.MatchString(update.NewPath) {
		errors = append(errors, "new path still contains UUID prefix")
	}

	// Validate new path doesn't contain invalid characters
	if strings.Contains(update.NewPath, "..") {
		errors = append(errors, "new path contains invalid '..' sequence")
	}

	return errors
}

// extractPathComponents extracts relative path and repository name from UUID-prefixed path.
func (r *PathMigrationRepository) extractPathComponents(
	ctx context.Context,
	fullPath string,
) (relativePath, repoName string, validationErrors []string) {
	// Clean the path first
	cleanedPath := r.cleanPath(fullPath)

	// Extract UUID and relative path using workspace pattern
	matches := r.workspacePattern.FindStringSubmatch(cleanedPath)
	if len(matches) < 3 {
		validationErrors = append(validationErrors, fmt.Sprintf("invalid UUID path format: %s", fullPath))
		return "", "", validationErrors
	}

	uuidStr := matches[1]
	relativePath = matches[2]

	// Validate UUID format
	if _, err := uuid.Parse(uuidStr); err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("invalid UUID format: %s", uuidStr))
		return "", "", validationErrors
	}

	// Extract repository name from relative path (first directory component)
	parts := strings.Split(relativePath, "/")
	if len(parts) > 0 && parts[0] != "" {
		repoName = parts[0]
	}

	slogger.Debug(ctx, "Extracted path components", slogger.Fields3(
		"full_path", fullPath,
		"relative_path", relativePath,
		"repo_name", repoName,
	))

	return relativePath, repoName, validationErrors
}

// cleanPath removes redundant path components and normalizes the path.
func (r *PathMigrationRepository) cleanPath(path string) string {
	// Remove trailing slashes
	cleaned := strings.TrimRight(path, "/")

	// Normalize multiple slashes
	for strings.Contains(cleaned, "//") {
		cleaned = strings.ReplaceAll(cleaned, "//", "/")
	}

	return cleaned
}

// validateUUIDPath validates UUID-prefixed path format.
func (r *PathMigrationRepository) validateUUIDPath(ctx context.Context, path string) []string {
	var errors []string

	// Check if path matches UUID pattern
	if !r.uuidPattern.MatchString(path) {
		errors = append(errors, fmt.Sprintf("path does not match UUID pattern: %s", path))
		return errors
	}

	// Extract and validate UUID
	matches := r.uuidPattern.FindStringSubmatch(path)
	if len(matches) < 2 {
		errors = append(errors, fmt.Sprintf("unable to extract UUID from path: %s", path))
		return errors
	}

	uuidStr := matches[1]
	if _, err := uuid.Parse(uuidStr); err != nil {
		errors = append(errors, fmt.Sprintf("invalid UUID format: %s", uuidStr))
	}

	return errors
}

// GetMigrationStatistics returns detailed migration statistics.
func (r *PathMigrationRepository) GetMigrationStatistics(ctx context.Context) (*service.MigrationStats, error) {
	slogger.Debug(ctx, "Getting migration statistics", slogger.Field("operation", "get_statistics"))

	if err := r.setSearchPath(ctx); err != nil {
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to set search path", err)
	}

	stats := &service.MigrationStats{
		StartTime: time.Now(),
	}

	// Get total chunks with UUID paths
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM code_chunks WHERE file_path ~ '^/tmp/codechunking-workspace/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/.*'").
		Scan(&stats.TotalChunks)
	if err != nil {
		slogger.Error(ctx, "Failed to count total chunks", slogger.Fields2(
			"error", err.Error(),
			"operation", "get_statistics",
		))
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to count total chunks", err)
	}

	// Get migrated chunks (paths without UUID prefix)
	err = r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM code_chunks WHERE file_path NOT ~ '^/tmp/codechunking-workspace/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/.*' AND file_path IS NOT NULL AND file_path != ''").
		Scan(&stats.MigratedChunks)
	if err != nil {
		slogger.Error(ctx, "Failed to count migrated chunks", slogger.Fields2(
			"error", err.Error(),
			"operation", "get_statistics",
		))
		return nil, service.NewMigrationErrorWithCause(
			service.ErrorTypeDatabase,
			"failed to count migrated chunks",
			err,
		)
	}

	// Calculate failed and skipped chunks
	if stats.MigratedChunks > stats.TotalChunks {
		stats.MigratedChunks = stats.TotalChunks
	}
	stats.FailedChunks = stats.TotalChunks - stats.MigratedChunks
	stats.SkippedChunks = 0 // Will be updated during actual migration

	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	slogger.Info(ctx, "Migration statistics retrieved", slogger.Fields{
		"total_chunks":    stats.TotalChunks,
		"migrated_chunks": stats.MigratedChunks,
		"failed_chunks":   stats.FailedChunks,
		"success_rate":    stats.SuccessRate(),
		"progress":        stats.Progress(),
	})

	return stats, nil
}

// RollbackMigration rolls back a migration using backup data.
func (r *PathMigrationRepository) RollbackMigration(
	ctx context.Context,
	backupData map[string]string,
) (*service.MigrationStats, error) {
	slogger.Info(ctx, "Starting migration rollback", slogger.Fields2(
		"backup_size", len(backupData),
		"operation", "rollback_migration",
	))

	stats := &service.MigrationStats{
		StartTime:   time.Now(),
		TotalChunks: len(backupData),
	}

	if err := r.setSearchPath(ctx); err != nil {
		return stats, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to set search path", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		slogger.Error(ctx, "Failed to begin rollback transaction", slogger.Fields2(
			"error", err.Error(),
			"operation", "rollback_migration",
		))
		return stats, service.NewMigrationErrorWithCause(
			service.ErrorTypeTransaction,
			"failed to begin rollback transaction",
			err,
		)
	}
	defer tx.Rollback(ctx)

	for chunkIDStr, originalPath := range backupData {
		chunkID, err := uuid.Parse(chunkIDStr)
		if err != nil {
			stats.FailedChunks++
			stats.AddError(
				*service.NewMigrationErrorWithCause(service.ErrorTypeValidation, fmt.Sprintf("invalid chunk ID in backup: %s", chunkIDStr), err),
			)
			slogger.Warn(ctx, "Invalid chunk ID in backup during rollback", slogger.Fields3(
				"error", err.Error(),
				"chunk_id_str", chunkIDStr,
				"operation", "rollback_migration",
			))
			continue
		}

		result, err := tx.Exec(ctx, "UPDATE code_chunks SET file_path = $1 WHERE id = $2", originalPath, chunkID)
		if err != nil {
			stats.FailedChunks++
			stats.AddError(
				*service.NewMigrationErrorWithChunk(service.ErrorTypeDatabase, fmt.Sprintf("failed to rollback chunk: %v", err), chunkID, uuid.Nil),
			)
			slogger.Error(ctx, "Failed to rollback chunk", slogger.Fields3(
				"error", err.Error(),
				"chunk_id", chunkID.String(),
				"operation", "rollback_migration",
			))
			continue
		}

		if result.RowsAffected() > 0 {
			stats.MigratedChunks++
			slogger.Debug(ctx, "Successfully rolled back chunk", slogger.Fields3(
				"chunk_id", chunkID.String(),
				"original_path", originalPath,
				"operation", "rollback_migration",
			))
		} else {
			stats.SkippedChunks++
			slogger.Debug(ctx, "No rows affected for chunk rollback", slogger.Fields2(
				"chunk_id", chunkID.String(),
				"operation", "rollback_migration",
			))
		}
	}

	if err := tx.Commit(ctx); err != nil {
		slogger.Error(ctx, "Failed to commit rollback transaction", slogger.Fields3(
			"error", err.Error(),
			"rolled_back", stats.MigratedChunks,
			"failed", stats.FailedChunks,
		))
		return stats, service.NewMigrationErrorWithCause(
			service.ErrorTypeTransaction,
			"failed to commit rollback transaction",
			err,
		)
	}

	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	slogger.Info(ctx, "Migration rollback completed", slogger.Fields{
		"rolled_back": stats.MigratedChunks,
		"failed":      stats.FailedChunks,
		"skipped":     stats.SkippedChunks,
		"duration_ms": stats.Duration.Milliseconds(),
	})

	return stats, nil
}

// ValidateMigrationReadiness performs comprehensive validation before migration.
func (r *PathMigrationRepository) ValidateMigrationReadiness(
	ctx context.Context,
) (*service.MigrationValidation, error) {
	slogger.Info(ctx, "Validating migration readiness", slogger.Field("operation", "validate_readiness"))

	if err := r.setSearchPath(ctx); err != nil {
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to set search path", err)
	}

	validation := &service.MigrationValidation{
		Errors:   []string{},
		Warnings: []string{},
	}

	// Count total chunks
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM code_chunks").Scan(&validation.ValidChunks)
	if err != nil {
		slogger.Error(ctx, "Failed to count total chunks", slogger.Fields2(
			"error", err.Error(),
			"operation", "validate_readiness",
		))
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to count total chunks", err)
	}

	// Count chunks with UUID paths
	var uuidPathChunks int
	err = r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM code_chunks WHERE file_path ~ '^/tmp/codechunking-workspace/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/.*'").
		Scan(&uuidPathChunks)
	if err != nil {
		slogger.Error(ctx, "Failed to count UUID path chunks", slogger.Fields2(
			"error", err.Error(),
			"operation", "validate_readiness",
		))
		return nil, service.NewMigrationErrorWithCause(
			service.ErrorTypeDatabase,
			"failed to count UUID path chunks",
			err,
		)
	}

	// Count chunks with empty or null paths
	err = r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM code_chunks WHERE file_path IS NULL OR file_path = ''").
		Scan(&validation.InvalidChunks)
	if err != nil {
		slogger.Error(ctx, "Failed to count invalid chunks", slogger.Fields2(
			"error", err.Error(),
			"operation", "validate_readiness",
		))
		return nil, service.NewMigrationErrorWithCause(service.ErrorTypeDatabase, "failed to count invalid chunks", err)
	}

	// Check for malformed UUID paths
	var malformedPaths int
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM code_chunks 
		WHERE file_path LIKE '/tmp/codechunking-workspace/%' 
		AND file_path NOT ~ '^/tmp/codechunking-workspace/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/.*'
	`).Scan(&malformedPaths)
	if err != nil {
		slogger.Error(ctx, "Failed to count malformed paths", slogger.Fields2(
			"error", err.Error(),
			"operation", "validate_readiness",
		))
		return nil, service.NewMigrationErrorWithCause(
			service.ErrorTypeDatabase,
			"failed to count malformed paths",
			err,
		)
	}

	// Add validation results
	if uuidPathChunks == 0 {
		validation.Warnings = append(
			validation.Warnings,
			"No UUID-prefixed paths found - migration may not be necessary",
		)
	}

	if validation.InvalidChunks > 0 {
		validation.Errors = append(
			validation.Errors,
			fmt.Sprintf("Found %d chunks with empty or null paths", validation.InvalidChunks),
		)
	}

	if malformedPaths > 0 {
		validation.Errors = append(
			validation.Errors,
			fmt.Sprintf("Found %d chunks with malformed UUID paths", malformedPaths),
		)
	}

	// Check database connectivity and locks
	if !r.config.SkipMalformed {
		validation.Warnings = append(
			validation.Warnings,
			"Strict validation enabled - malformed paths will cause migration to fail",
		)
	}

	slogger.Info(ctx, "Migration readiness validation completed", slogger.Fields{
		"total_chunks":     validation.ValidChunks,
		"uuid_path_chunks": uuidPathChunks,
		"invalid_chunks":   validation.InvalidChunks,
		"malformed_paths":  malformedPaths,
		"error_count":      len(validation.Errors),
		"warning_count":    len(validation.Warnings),
	})

	return validation, nil
}
