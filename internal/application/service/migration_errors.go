package service

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MigrationErrorType represents different types of migration errors.
type MigrationErrorType string

const (
	// ErrorTypeValidation indicates a validation error.
	ErrorTypeValidation MigrationErrorType = "validation"
	// ErrorTypeMalformedPath indicates a malformed path error.
	ErrorTypeMalformedPath MigrationErrorType = "malformed_path"
	// ErrorTypeDatabase indicates a database operation error.
	ErrorTypeDatabase MigrationErrorType = "database"
	// ErrorTypeLock indicates a locking error.
	ErrorTypeLock MigrationErrorType = "lock"
	// ErrorTypeTimeout indicates a timeout error.
	ErrorTypeTimeout MigrationErrorType = "timeout"
	// ErrorTypeRetry indicates a retry exhaustion error.
	ErrorTypeRetry MigrationErrorType = "retry_exhausted"
	// ErrorTypeTransaction indicates a transaction error.
	ErrorTypeTransaction MigrationErrorType = "transaction"
)

// MigrationError represents a migration-specific error with context.
type MigrationError struct {
	Type      MigrationErrorType `json:"type"`
	Message   string             `json:"message"`
	Path      string             `json:"path,omitempty"`
	ChunkID   *uuid.UUID         `json:"chunk_id,omitempty"`
	RepoID    *uuid.UUID         `json:"repo_id,omitempty"`
	Operation string             `json:"operation,omitempty"`
	Retries   int                `json:"retries,omitempty"`
	Cause     error              `json:"cause,omitempty"`
	Timestamp time.Time          `json:"timestamp"`
}

// Error implements the error interface.
func (e *MigrationError) Error() string {
	if e.ChunkID != nil {
		return fmt.Sprintf("migration error [%s] on chunk %s: %s", e.Type, e.ChunkID.String(), e.Message)
	}
	if e.Path != "" {
		return fmt.Sprintf("migration error [%s] for path '%s': %s", e.Type, e.Path, e.Message)
	}
	return fmt.Sprintf("migration error [%s]: %s", e.Type, e.Message)
}

// Unwrap returns the underlying cause.
func (e *MigrationError) Unwrap() error {
	return e.Cause
}

// IsRetryable checks if the error is retryable.
func (e *MigrationError) IsRetryable() bool {
	switch e.Type {
	case ErrorTypeDatabase, ErrorTypeTimeout, ErrorTypeLock:
		return true
	case ErrorTypeValidation, ErrorTypeMalformedPath, ErrorTypeTransaction:
		return false
	default:
		return false
	}
}

// NewMigrationError creates a new migration error.
func NewMigrationError(errorType MigrationErrorType, message string) *MigrationError {
	return &MigrationError{
		Type:      errorType,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// NewMigrationErrorWithPath creates a new migration error with path context.
func NewMigrationErrorWithPath(errorType MigrationErrorType, message, path string) *MigrationError {
	return &MigrationError{
		Type:      errorType,
		Message:   message,
		Path:      path,
		Timestamp: time.Now(),
	}
}

// NewMigrationErrorWithChunk creates a new migration error with chunk context.
func NewMigrationErrorWithChunk(
	errorType MigrationErrorType,
	message string,
	chunkID, repoID uuid.UUID,
) *MigrationError {
	return &MigrationError{
		Type:      errorType,
		Message:   message,
		ChunkID:   &chunkID,
		RepoID:    &repoID,
		Timestamp: time.Now(),
	}
}

// NewMigrationErrorWithCause creates a new migration error with an underlying cause.
func NewMigrationErrorWithCause(errorType MigrationErrorType, message string, cause error) *MigrationError {
	return &MigrationError{
		Type:      errorType,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// MigrationStats tracks migration statistics.
type MigrationStats struct {
	TotalChunks      int              `json:"total_chunks"`
	MigratedChunks   int              `json:"migrated_chunks"`
	FailedChunks     int              `json:"failed_chunks"`
	SkippedChunks    int              `json:"skipped_chunks"`
	Duration         time.Duration    `json:"duration"`
	StartTime        time.Time        `json:"start_time"`
	EndTime          time.Time        `json:"end_time"`
	Errors           []MigrationError `json:"errors,omitempty"`
	BatchesProcessed int              `json:"batches_processed"`
}

// Progress calculates migration progress percentage.
func (s *MigrationStats) Progress() float64 {
	if s.TotalChunks == 0 {
		return 100.0
	}
	completed := s.MigratedChunks + s.FailedChunks + s.SkippedChunks
	return float64(completed) / float64(s.TotalChunks) * 100.0
}

// SuccessRate calculates the success rate percentage.
func (s *MigrationStats) SuccessRate() float64 {
	completed := s.MigratedChunks + s.FailedChunks + s.SkippedChunks
	if completed == 0 {
		return 0.0
	}
	return float64(s.MigratedChunks) / float64(completed) * 100.0
}

// AddError adds an error to the stats.
func (s *MigrationStats) AddError(err MigrationError) {
	s.Errors = append(s.Errors, err)
}

// MigrationResult represents the result of a migration operation.
type MigrationResult struct {
	Success bool           `json:"success"`
	Stats   MigrationStats `json:"stats"`
	Error   error          `json:"error,omitempty"`
}

// IsSuccess returns true if the migration was successful.
func (r *MigrationResult) IsSuccess() bool {
	return r.Success && r.Error == nil
}
