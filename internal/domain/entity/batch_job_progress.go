package entity

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Batch job progress status constants.
const (
	StatusPending           = "pending"
	StatusPendingSubmission = "pending_submission"
	StatusProcessing        = "processing"
	StatusCompleted         = "completed"
	StatusFailed            = "failed"
	StatusRetryScheduled    = "retry_scheduled"
)

// Gemini batch job ID validation constants.
const (
	GeminiBatchIDPrefix    = "batches/"
	geminiBatchIDMinLength = len(GeminiBatchIDPrefix)
)

// BatchJobProgress represents the progress of a batch embedding job for a single batch.
type BatchJobProgress struct {
	id                  uuid.UUID
	repositoryID        *uuid.UUID
	indexingJobID       uuid.UUID
	batchNumber         int
	totalBatches        int
	chunksProcessed     int
	status              string
	retryCount          int
	nextRetryAt         *time.Time
	errorMessage        *string
	geminiBatchJobID    *string
	geminiFileURI       *string
	batchRequestData    []byte
	submissionAttempts  int
	lastSubmissionError *string
	nextSubmissionAt    *time.Time
	createdAt           time.Time
	updatedAt           time.Time
}

var (
	ErrInvalidRepositoryID  = errors.New("invalid repository ID")
	ErrInvalidIndexingJobID = errors.New("invalid indexing job ID")
	ErrInvalidBatchNumber   = errors.New("invalid batch number")
	ErrInvalidTotalBatches  = errors.New("invalid total batches")
)

// NewBatchJobProgress creates a new BatchJobProgress entity.
func NewBatchJobProgress(
	repositoryID uuid.UUID,
	indexingJobID uuid.UUID,
	batchNumber, totalBatches int,
) *BatchJobProgress {
	now := time.Now()
	return &BatchJobProgress{
		id:              uuid.New(),
		repositoryID:    &repositoryID,
		indexingJobID:   indexingJobID,
		batchNumber:     batchNumber,
		totalBatches:    totalBatches,
		chunksProcessed: 0,
		status:          StatusPending,
		retryCount:      0,
		nextRetryAt:     nil,
		errorMessage:    nil,
		createdAt:       now,
		updatedAt:       now,
	}
}

// RestoreBatchJobProgress creates a BatchJobProgress entity from stored data.
func RestoreBatchJobProgress(
	id uuid.UUID,
	repositoryID *uuid.UUID,
	indexingJobID uuid.UUID,
	batchNumber int,
	totalBatches int,
	chunksProcessed int,
	status string,
	retryCount int,
	nextRetryAt *time.Time,
	errorMessage *string,
	geminiBatchJobID *string,
	geminiFileURI *string,
	createdAt time.Time,
	updatedAt time.Time,
	batchRequestData []byte,
	submissionAttempts int,
	nextSubmissionAt *time.Time,
) *BatchJobProgress {
	return &BatchJobProgress{
		id:                 id,
		repositoryID:       repositoryID,
		indexingJobID:      indexingJobID,
		batchNumber:        batchNumber,
		totalBatches:       totalBatches,
		chunksProcessed:    chunksProcessed,
		status:             status,
		retryCount:         retryCount,
		nextRetryAt:        nextRetryAt,
		errorMessage:       errorMessage,
		geminiBatchJobID:   geminiBatchJobID,
		geminiFileURI:      geminiFileURI,
		batchRequestData:   batchRequestData,
		submissionAttempts: submissionAttempts,
		nextSubmissionAt:   nextSubmissionAt,
		createdAt:          createdAt,
		updatedAt:          updatedAt,
	}
}

// ID returns the batch job progress ID.
func (b *BatchJobProgress) ID() uuid.UUID {
	return b.id
}

// RepositoryID returns the repository ID.
func (b *BatchJobProgress) RepositoryID() *uuid.UUID {
	return b.repositoryID
}

// IndexingJobID returns the indexing job ID.
func (b *BatchJobProgress) IndexingJobID() uuid.UUID {
	return b.indexingJobID
}

// BatchNumber returns the batch number.
func (b *BatchJobProgress) BatchNumber() int {
	return b.batchNumber
}

// TotalBatches returns the total number of batches.
func (b *BatchJobProgress) TotalBatches() int {
	return b.totalBatches
}

// ChunksProcessed returns the number of chunks processed.
func (b *BatchJobProgress) ChunksProcessed() int {
	return b.chunksProcessed
}

// Status returns the current status.
func (b *BatchJobProgress) Status() string {
	return b.status
}

// RetryCount returns the number of retries.
func (b *BatchJobProgress) RetryCount() int {
	return b.retryCount
}

// NextRetryAt returns the next retry timestamp.
func (b *BatchJobProgress) NextRetryAt() *time.Time {
	return b.nextRetryAt
}

// ErrorMessage returns the error message.
func (b *BatchJobProgress) ErrorMessage() *string {
	return b.errorMessage
}

// GeminiBatchJobID returns the Gemini batch job ID.
func (b *BatchJobProgress) GeminiBatchJobID() *string {
	return b.geminiBatchJobID
}

// GeminiFileURI returns the Gemini uploaded file URI.
func (b *BatchJobProgress) GeminiFileURI() *string {
	return b.geminiFileURI
}

// SetGeminiFileURI sets the Gemini file URI after successful upload.
func (b *BatchJobProgress) SetGeminiFileURI(uri string) {
	b.geminiFileURI = &uri
	b.updatedAt = time.Now()
}

// HasUploadedFile returns true if the batch has an uploaded file URI.
func (b *BatchJobProgress) HasUploadedFile() bool {
	return b.geminiFileURI != nil && *b.geminiFileURI != ""
}

// CreatedAt returns the creation timestamp.
func (b *BatchJobProgress) CreatedAt() time.Time {
	return b.createdAt
}

// UpdatedAt returns the last update timestamp.
func (b *BatchJobProgress) UpdatedAt() time.Time {
	return b.updatedAt
}

// MarkProcessing marks the batch as processing.
func (b *BatchJobProgress) MarkProcessing() {
	b.status = StatusProcessing
	b.updatedAt = time.Now()
}

// MarkSubmittedToGemini marks the batch as submitted to Gemini with a job ID.
// Returns an error if the job ID format is invalid (must start with "batches/").
func (b *BatchJobProgress) MarkSubmittedToGemini(jobID string) error {
	// Validate Gemini batch job ID format
	if len(jobID) < geminiBatchIDMinLength || !strings.HasPrefix(jobID, GeminiBatchIDPrefix) {
		return fmt.Errorf("invalid batch job ID format: expected '%s<id>', got '%s'", GeminiBatchIDPrefix, jobID)
	}
	b.status = StatusProcessing
	b.geminiBatchJobID = &jobID
	b.updatedAt = time.Now()
	return nil
}

// MarkCompleted marks the batch as completed with the number of chunks processed.
func (b *BatchJobProgress) MarkCompleted(chunksProcessed int) {
	b.status = StatusCompleted
	b.chunksProcessed = chunksProcessed
	b.updatedAt = time.Now()
}

// MarkFailed marks the batch as failed with an error message.
func (b *BatchJobProgress) MarkFailed(errorMsg string) {
	b.status = StatusFailed
	b.errorMessage = &errorMsg
	b.updatedAt = time.Now()
}

// ScheduleRetry schedules a retry for the batch.
func (b *BatchJobProgress) ScheduleRetry(retryAt time.Time) {
	b.status = StatusRetryScheduled
	b.retryCount++
	b.nextRetryAt = &retryAt
	b.updatedAt = time.Now()
}

// IsRetryable returns true if the batch is retryable (status is retry_scheduled).
func (b *BatchJobProgress) IsRetryable() bool {
	return b.status == StatusRetryScheduled
}

// ShouldRetry returns true if the batch should be retried now (status is retry_scheduled and retry time has passed).
func (b *BatchJobProgress) ShouldRetry() bool {
	if b.status != StatusRetryScheduled || b.nextRetryAt == nil {
		return false
	}
	return time.Now().After(*b.nextRetryAt)
}

// IncrementRetryCount increments the retry count.
func (b *BatchJobProgress) IncrementRetryCount() {
	b.retryCount++
	b.updatedAt = time.Now()
}

// Validate ensures the batch job progress entity is in a valid state.
func (b *BatchJobProgress) Validate() error {
	if b.id == uuid.Nil {
		return errors.New("invalid batch ID")
	}
	if b.indexingJobID == uuid.Nil {
		return ErrInvalidIndexingJobID
	}
	if b.batchNumber <= 0 {
		return ErrInvalidBatchNumber
	}
	if b.totalBatches <= 0 {
		return ErrInvalidTotalBatches
	}
	if b.batchNumber > b.totalBatches {
		return errors.New("batch number cannot exceed total batches")
	}
	if b.chunksProcessed < 0 {
		return errors.New("chunks processed cannot be negative")
	}
	if b.retryCount < 0 {
		return errors.New("retry count cannot be negative")
	}
	// Validate status
	switch b.status {
	case StatusPending, StatusPendingSubmission, StatusProcessing, StatusCompleted, StatusFailed, StatusRetryScheduled:
		// Valid status
	default:
		return errors.New("invalid status")
	}
	// If status is completed, chunks processed should be positive
	if b.status == StatusCompleted && b.chunksProcessed <= 0 {
		return errors.New("completed batch must have positive chunks processed")
	}
	return nil
}

// HasRepositoryID returns true if the batch has a valid repository ID.
func (b *BatchJobProgress) HasRepositoryID() bool {
	return b.repositoryID != nil && *b.repositoryID != uuid.Nil
}

// GetRequiredRepositoryID returns the repository ID or an error if not set.
func (b *BatchJobProgress) GetRequiredRepositoryID() (uuid.UUID, error) {
	if !b.HasRepositoryID() {
		return uuid.Nil, errors.New("repository ID is required but not set")
	}
	return *b.repositoryID, nil
}

// MarkPendingSubmission marks the batch as pending submission with request data.
func (b *BatchJobProgress) MarkPendingSubmission(requestData []byte) {
	b.status = StatusPendingSubmission
	b.batchRequestData = requestData
	b.submissionAttempts = 0
	b.nextSubmissionAt = nil
	b.updatedAt = time.Now()
}

// MarkSubmissionFailed increments submission attempts and schedules retry.
func (b *BatchJobProgress) MarkSubmissionFailed(errorMsg string, nextAttemptAt time.Time) {
	b.submissionAttempts++
	b.errorMessage = &errorMsg
	b.nextSubmissionAt = &nextAttemptAt
	b.updatedAt = time.Now()
}

// BatchRequestData returns the serialized batch request data.
func (b *BatchJobProgress) BatchRequestData() []byte {
	return b.batchRequestData
}

// SubmissionAttempts returns the number of submission attempts.
func (b *BatchJobProgress) SubmissionAttempts() int {
	return b.submissionAttempts
}

// NextSubmissionAt returns when the next submission should be attempted.
func (b *BatchJobProgress) NextSubmissionAt() *time.Time {
	return b.nextSubmissionAt
}

// IsReadyForSubmission returns true if batch is ready to be submitted.
func (b *BatchJobProgress) IsReadyForSubmission() bool {
	if b.status != StatusPendingSubmission {
		return false
	}
	if b.nextSubmissionAt == nil {
		return true
	}
	return time.Now().After(*b.nextSubmissionAt)
}
