package service

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// MigrationRepositoryInterface defines the interface for path migration operations
// This avoids circular dependencies between service and repository layers.
type MigrationRepositoryInterface interface {
	GetUUIDPrefixedPaths(ctx context.Context) ([]PathMigrationRecord, error)
	GetUUIDPrefixedPathsForRepo(ctx context.Context, repoID uuid.UUID) ([]PathMigrationRecord, error)
	BatchUpdatePaths(ctx context.Context, updates []PathUpdateRecord) (*MigrationStats, error)
	TransactionalPathMigration(ctx context.Context, updates []PathUpdateRecord) error
	GetMigrationProgress(ctx context.Context) (*MigrationProgress, error)
	ValidateMigrationData(ctx context.Context) (*MigrationValidation, error)
	CreateBackup(ctx context.Context, repoID *uuid.UUID) (map[string]string, error)
	RestoreFromBackup(ctx context.Context, backupData map[string]string) error
	AcquireMigrationLock(ctx context.Context) (bool, error)
	ReleaseMigrationLock(ctx context.Context) error
}

// PathMigrationRecord represents a path migration record.
type PathMigrationRecord struct {
	ChunkID   uuid.UUID
	RepoID    uuid.UUID
	OldPath   string
	NewPath   string
	Status    string
	CreatedAt time.Time
}

// PathUpdateRecord represents a path update operation.
type PathUpdateRecord struct {
	ChunkID uuid.UUID
	RepoID  uuid.UUID
	OldPath string
	NewPath string
}

// MigrationProgress tracks migration progress.
type MigrationProgress struct {
	TotalChunks        int
	MigratedChunks     int
	FailedChunks       int
	StartTime          time.Time
	EstimatedFinish    time.Time
	ProgressPercentage float64 `json:"progress_percentage"`
}

// MigrationValidation contains validation results.
type MigrationValidation struct {
	ValidChunks   int
	InvalidChunks int
	Errors        []string
	Warnings      []string
}
