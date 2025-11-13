package service

import (
	"codechunking/internal/application/common/slogger"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PathMigrationService handles the business logic for path migration.
type PathMigrationService struct {
	repo          MigrationRepositoryInterface
	pathConverter *PathConverter
	retryExecutor *RetryExecutor
	config        *MigrationConfig
}

// NewPathMigrationService creates a new path migration service with repository injection.
func NewPathMigrationService(
	repo MigrationRepositoryInterface,
	config *MigrationConfig,
) (*PathMigrationService, error) {
	if config == nil {
		config = DefaultMigrationConfig()
	}

	// Create path converter
	pathConverter, err := NewPathConverter(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create path converter: %w", err)
	}

	// Create retry executor
	retryConfig := &RetryConfig{
		MaxRetries:    config.MaxRetries,
		InitialDelay:  time.Duration(config.RetryDelayMs) * time.Millisecond,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        config.EnableBackoff,
	}
	retryExecutor := NewRetryExecutor(retryConfig)

	return &PathMigrationService{
		repo:          repo,
		pathConverter: pathConverter,
		retryExecutor: retryExecutor,
		config:        config,
	}, nil
}

// DetectUUIDPaths counts chunks with UUID-prefixed paths in the database.
func (s *PathMigrationService) DetectUUIDPaths(ctx context.Context) (int, error) {
	slogger.Info(ctx, "Detecting UUID-prefixed paths", slogger.Field("operation", "detect_uuid_paths"))

	migrations, err := s.repo.GetUUIDPrefixedPaths(ctx)
	if err != nil {
		slogger.Error(ctx, "Failed to detect UUID paths", slogger.Fields2(
			"error", err.Error(),
			"operation", "detect_uuid_paths",
		))
		return 0, err
	}

	count := len(migrations)
	slogger.Info(ctx, "UUID path detection completed", slogger.Fields2(
		"count", count,
		"operation", "detect_uuid_paths",
	))

	return count, nil
}

// DetectUUIDPathsForRepo counts UUID-prefixed paths for a specific repository.
func (s *PathMigrationService) DetectUUIDPathsForRepo(ctx context.Context, repoID uuid.UUID) (int, error) {
	slogger.Info(ctx, "Detecting UUID-prefixed paths for repository", slogger.Fields2(
		"repo_id", repoID.String(),
		"operation", "detect_uuid_paths_repo",
	))

	migrations, err := s.repo.GetUUIDPrefixedPathsForRepo(ctx, repoID)
	if err != nil {
		slogger.Error(ctx, "Failed to detect repository UUID paths", slogger.Fields3(
			"error", err.Error(),
			"repo_id", repoID.String(),
			"operation", "detect_uuid_paths_repo",
		))
		return 0, err
	}

	count := len(migrations)
	slogger.Info(ctx, "Repository UUID path detection completed", slogger.Fields3(
		"repo_id", repoID.String(),
		"count", count,
		"operation", "detect_uuid_paths_repo",
	))

	return count, nil
}

// ConvertToRelativePath converts absolute UUID-prefixed paths to relative paths.
func (s *PathMigrationService) ConvertToRelativePath(ctx context.Context, absolutePath string) (string, error) {
	return s.pathConverter.ConvertToRelativePath(ctx, absolutePath)
}

// ExecutePathMigration performs the migration of UUID-prefixed paths to relative paths.
func (s *PathMigrationService) ExecutePathMigration(ctx context.Context) (*MigrationResult, error) {
	slogger.Info(ctx, "Starting full path migration", slogger.Field("operation", "execute_migration"))

	result := &MigrationResult{
		Stats: MigrationStats{
			StartTime: time.Now(),
		},
	}

	// Acquire migration lock
	lockAcquired, err := s.repo.AcquireMigrationLock(ctx)
	if err != nil {
		result.Error = err
		slogger.Error(ctx, "Failed to acquire migration lock", slogger.Fields2(
			"error", err.Error(),
			"operation", "execute_migration",
		))
		return result, err
	}
	if !lockAcquired {
		result.Error = NewMigrationError(ErrorTypeLock, "migration already in progress")
		return result, result.Error
	}
	defer s.repo.ReleaseMigrationLock(ctx)

	// Get all UUID-prefixed paths
	migrations, err := s.repo.GetUUIDPrefixedPaths(ctx)
	if err != nil {
		result.Error = err
		return result, err
	}

	result.Stats.TotalChunks = len(migrations)
	if result.Stats.TotalChunks == 0 {
		slogger.Info(ctx, "No UUID-prefixed paths found for migration", slogger.Field("operation", "execute_migration"))
		result.Success = true
		result.Stats.EndTime = time.Now()
		result.Stats.Duration = result.Stats.EndTime.Sub(result.Stats.StartTime)
		return result, nil
	}

	// Convert migrations to updates
	updates, err := s.prepareUpdates(ctx, migrations)
	if err != nil {
		result.Error = err
		return result, err
	}

	// Execute batch migration with retry
	err = s.retryExecutor.Execute(ctx, func(ctx context.Context) error {
		stats, err := s.repo.BatchUpdatePaths(ctx, updates)
		if err != nil {
			return err
		}
		result.Stats = *stats
		return nil
	})
	if err != nil {
		result.Error = err
		slogger.Error(ctx, "Migration failed", slogger.Fields2(
			"error", err.Error(),
			"operation", "execute_migration",
		))
		return result, err
	}

	result.Stats.EndTime = time.Now()
	result.Stats.Duration = result.Stats.EndTime.Sub(result.Stats.StartTime)
	result.Success = true

	slogger.Info(ctx, "Path migration completed successfully", slogger.Fields3(
		"total_migrated", result.Stats.MigratedChunks,
		"total_failed", result.Stats.FailedChunks,
		"duration_ms", result.Stats.Duration.Milliseconds(),
	))

	return result, nil
}

// ExecutePathMigrationForRepo performs migration only for a specific repository.
func (s *PathMigrationService) ExecutePathMigrationForRepo(
	ctx context.Context,
	repoID uuid.UUID,
) (*MigrationResult, error) {
	slogger.Info(ctx, "Starting repository-specific path migration", slogger.Fields2(
		"repo_id", repoID.String(),
		"operation", "execute_migration_repo",
	))

	result := &MigrationResult{
		Stats: MigrationStats{
			StartTime: time.Now(),
		},
	}

	// Acquire migration lock
	lockAcquired, err := s.repo.AcquireMigrationLock(ctx)
	if err != nil {
		result.Error = err
		return result, err
	}
	if !lockAcquired {
		result.Error = NewMigrationError(ErrorTypeLock, "migration already in progress")
		return result, result.Error
	}
	defer s.repo.ReleaseMigrationLock(ctx)

	// Get UUID-prefixed paths for this repository
	migrations, err := s.repo.GetUUIDPrefixedPathsForRepo(ctx, repoID)
	if err != nil {
		result.Error = err
		return result, err
	}

	result.Stats.TotalChunks = len(migrations)
	if result.Stats.TotalChunks == 0 {
		slogger.Info(ctx, "No UUID-prefixed paths found for repository", slogger.Fields2(
			"repo_id", repoID.String(),
			"operation", "execute_migration_repo",
		))
		result.Success = true
		result.Stats.EndTime = time.Now()
		result.Stats.Duration = result.Stats.EndTime.Sub(result.Stats.StartTime)
		return result, nil
	}

	// Convert migrations to updates
	updates, err := s.prepareUpdates(ctx, migrations)
	if err != nil {
		result.Error = err
		return result, err
	}

	// Execute batch migration with retry
	err = s.retryExecutor.Execute(ctx, func(ctx context.Context) error {
		stats, err := s.repo.BatchUpdatePaths(ctx, updates)
		if err != nil {
			return err
		}
		result.Stats = *stats
		return nil
	})
	if err != nil {
		result.Error = err
		return result, err
	}

	result.Stats.EndTime = time.Now()
	result.Stats.Duration = result.Stats.EndTime.Sub(result.Stats.StartTime)
	result.Success = true

	slogger.Info(ctx, "Repository path migration completed", slogger.Fields{
		"repo_id":        repoID.String(),
		"total_migrated": result.Stats.MigratedChunks,
		"total_failed":   result.Stats.FailedChunks,
		"duration_ms":    result.Stats.Duration.Milliseconds(),
	})

	return result, nil
}

// CreateMigrationBackup creates a backup of original paths before migration.
func (s *PathMigrationService) CreateMigrationBackup(ctx context.Context) (map[string]string, error) {
	slogger.Info(ctx, "Creating migration backup", slogger.Field("operation", "create_backup"))

	backup, err := s.repo.CreateBackup(ctx, nil)
	if err != nil {
		slogger.Error(ctx, "Failed to create migration backup", slogger.Fields2(
			"error", err.Error(),
			"operation", "create_backup",
		))
		return nil, err
	}

	slogger.Info(ctx, "Migration backup created successfully", slogger.Fields2(
		"backup_size", len(backup),
		"operation", "create_backup",
	))

	return backup, nil
}

// CreateMigrationBackupForRepo creates a backup for a specific repository.
func (s *PathMigrationService) CreateMigrationBackupForRepo(
	ctx context.Context,
	repoID uuid.UUID,
) (map[string]string, error) {
	slogger.Info(ctx, "Creating repository migration backup", slogger.Fields2(
		"repo_id", repoID.String(),
		"operation", "create_backup_repo",
	))

	backup, err := s.repo.CreateBackup(ctx, &repoID)
	if err != nil {
		slogger.Error(ctx, "Failed to create repository backup", slogger.Fields3(
			"error", err.Error(),
			"repo_id", repoID.String(),
			"operation", "create_backup_repo",
		))
		return nil, err
	}

	slogger.Info(ctx, "Repository backup created successfully", slogger.Fields3(
		"repo_id", repoID.String(),
		"backup_size", len(backup),
		"operation", "create_backup_repo",
	))

	return backup, nil
}

// RollbackPathMigration restores original paths from backup data.
func (s *PathMigrationService) RollbackPathMigration(ctx context.Context, backupData map[string]string) error {
	slogger.Info(ctx, "Starting path migration rollback", slogger.Fields2(
		"backup_size", len(backupData),
		"operation", "rollback_migration",
	))

	err := s.repo.RestoreFromBackup(ctx, backupData)
	if err != nil {
		slogger.Error(ctx, "Migration rollback failed", slogger.Fields2(
			"error", err.Error(),
			"operation", "rollback_migration",
		))
		return err
	}

	slogger.Info(ctx, "Path migration rollback completed", slogger.Field("operation", "rollback_migration"))
	return nil
}

// GetMigrationProgress returns migration progress information.
func (s *PathMigrationService) GetMigrationProgress(ctx context.Context) (*MigrationProgress, error) {
	return s.repo.GetMigrationProgress(ctx)
}

// ValidateMigrationData validates migration data before processing.
func (s *PathMigrationService) ValidateMigrationData(ctx context.Context) (*MigrationValidation, error) {
	return s.repo.ValidateMigrationData(ctx)
}

// Helper methods

// prepareUpdates converts PathMigrationRecord to PathUpdateRecord.
func (s *PathMigrationService) prepareUpdates(
	ctx context.Context,
	migrations []PathMigrationRecord,
) ([]PathUpdateRecord, error) {
	updates := make([]PathUpdateRecord, 0, len(migrations))

	for _, migration := range migrations {
		// Validate the old path
		if err := s.pathConverter.ValidatePath(ctx, migration.OldPath); err != nil {
			if s.config.SkipMalformed {
				slogger.Warn(ctx, "Skipping malformed path", slogger.Fields3(
					"path", migration.OldPath,
					"chunk_id", migration.ChunkID.String(),
					"error", err.Error(),
				))
				continue
			}
			return nil, err
		}

		// Convert to relative path
		newPath, err := s.pathConverter.ConvertToRelativePath(ctx, migration.OldPath)
		if err != nil {
			if s.config.SkipMalformed {
				slogger.Warn(ctx, "Skipping path conversion", slogger.Fields3(
					"path", migration.OldPath,
					"chunk_id", migration.ChunkID.String(),
					"error", err.Error(),
				))
				continue
			}
			return nil, err
		}

		updates = append(updates, PathUpdateRecord{
			ChunkID: migration.ChunkID,
			RepoID:  migration.RepoID,
			OldPath: migration.OldPath,
			NewPath: newPath,
		})
	}

	return updates, nil
}

// GetConfig returns the current migration configuration.
func (s *PathMigrationService) GetConfig() *MigrationConfig {
	return s.config
}

// Legacy functions for backward compatibility - these work with the new architecture

// detectUUIDPaths counts chunks with UUID-prefixed paths in the database.
func detectUUIDPaths(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	// Create repository adapter for legacy function
	repo := NewPathMigrationRepositoryAdapter(pool)
	service, err := NewPathMigrationService(repo, nil)
	if err != nil {
		return 0, err
	}
	return service.DetectUUIDPaths(ctx)
}

// convertToRelativePath converts absolute UUID-prefixed paths to relative paths.
func convertToRelativePath(absolutePath string) (string, error) {
	converter, err := NewPathConverter(nil)
	if err != nil {
		return "", err
	}
	return converter.ConvertToRelativePath(context.Background(), absolutePath)
}

// executePathMigration performs the actual migration of UUID-prefixed paths to relative paths.
func executePathMigration(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	// Create repository adapter for legacy function
	repo := NewPathMigrationRepositoryAdapter(pool)
	service, err := NewPathMigrationService(repo, nil)
	if err != nil {
		return 0, err
	}
	result, err := service.ExecutePathMigration(ctx)
	if err != nil {
		return 0, err
	}
	return result.Stats.MigratedChunks, nil
}

// createMigrationBackup creates a backup of original paths before migration.
func createMigrationBackup(ctx context.Context, pool *pgxpool.Pool) (map[string]string, error) {
	// Create repository adapter for legacy function
	repo := NewPathMigrationRepositoryAdapter(pool)
	service, err := NewPathMigrationService(repo, nil)
	if err != nil {
		return nil, err
	}
	return service.CreateMigrationBackup(ctx)
}

// executePathMigrationForRepo performs migration only for a specific repository.
func executePathMigrationForRepo(ctx context.Context, pool *pgxpool.Pool, repoID uuid.UUID) (int, error) {
	// Create repository adapter for legacy function
	repo := NewPathMigrationRepositoryAdapter(pool)
	service, err := NewPathMigrationService(repo, nil)
	if err != nil {
		return 0, err
	}
	result, err := service.ExecutePathMigrationForRepo(ctx, repoID)
	if err != nil {
		return 0, err
	}
	return result.Stats.MigratedChunks, nil
}

// rollbackPathMigration restores original paths from backup data.
func rollbackPathMigration(ctx context.Context, pool *pgxpool.Pool, backupData map[string]string) error {
	// Create repository adapter for legacy function
	repo := NewPathMigrationRepositoryAdapter(pool)
	service, err := NewPathMigrationService(repo, nil)
	if err != nil {
		return err
	}
	return service.RollbackPathMigration(ctx, backupData)
}
