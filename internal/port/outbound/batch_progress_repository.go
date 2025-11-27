package outbound

import (
	"codechunking/internal/domain/entity"
	"context"

	"github.com/google/uuid"
)

// BatchProgressRepository defines the interface for batch job progress persistence.
type BatchProgressRepository interface {
	// Save persists a batch job progress record
	Save(ctx context.Context, progress *entity.BatchJobProgress) error

	// GetByID retrieves a batch job progress by its ID
	GetByID(ctx context.Context, id uuid.UUID) (*entity.BatchJobProgress, error)

	// GetByJobID retrieves all batch progress records for a given indexing job
	GetByJobID(ctx context.Context, jobID uuid.UUID) ([]*entity.BatchJobProgress, error)

	// GetPendingGeminiBatches retrieves all batches waiting for Gemini API completion
	// Returns batches with status='processing' AND gemini_batch_job_id IS NOT NULL
	GetPendingGeminiBatches(ctx context.Context) ([]*entity.BatchJobProgress, error)

	// GetNextRetryBatch retrieves the next batch ready for retry
	GetNextRetryBatch(ctx context.Context) (*entity.BatchJobProgress, error)

	// GetPendingSubmissionBatch retrieves a single batch ready for submission.
	// Uses SELECT ... FOR UPDATE SKIP LOCKED for distributed safety.
	// Returns nil if no batches are ready for submission.
	GetPendingSubmissionBatch(ctx context.Context) (*entity.BatchJobProgress, error)

	// GetPendingSubmissionCount returns the count of batches awaiting submission.
	GetPendingSubmissionCount(ctx context.Context) (int, error)

	// UpdateStatus updates the status of a batch job progress
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error

	// MarkCompleted marks a batch as completed
	MarkCompleted(ctx context.Context, id uuid.UUID, chunksProcessed int) error

	// Delete removes a batch job progress record
	Delete(ctx context.Context, id uuid.UUID) error
}
