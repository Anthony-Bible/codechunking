package entity

import (
	"time"

	"codechunking/internal/domain/valueobject"

	"github.com/google/uuid"
)

// IndexingJob represents an asynchronous job for processing a repository
type IndexingJob struct {
	id             uuid.UUID
	repositoryID   uuid.UUID
	status         valueobject.JobStatus
	startedAt      *time.Time
	completedAt    *time.Time
	errorMessage   *string
	filesProcessed int
	chunksCreated  int
	createdAt      time.Time
	updatedAt      time.Time
	deletedAt      *time.Time
}

// NewIndexingJob creates a new IndexingJob entity
func NewIndexingJob(repositoryID uuid.UUID) *IndexingJob {
	now := time.Now()
	return &IndexingJob{
		id:             uuid.New(),
		repositoryID:   repositoryID,
		status:         valueobject.JobStatusPending,
		filesProcessed: 0,
		chunksCreated:  0,
		createdAt:      now,
		updatedAt:      now,
	}
}

// RestoreIndexingJob creates an IndexingJob entity from stored data
func RestoreIndexingJob(
	id uuid.UUID,
	repositoryID uuid.UUID,
	status valueobject.JobStatus,
	startedAt *time.Time,
	completedAt *time.Time,
	errorMessage *string,
	filesProcessed int,
	chunksCreated int,
	createdAt time.Time,
	updatedAt time.Time,
	deletedAt *time.Time,
) *IndexingJob {
	return &IndexingJob{
		id:             id,
		repositoryID:   repositoryID,
		status:         status,
		startedAt:      startedAt,
		completedAt:    completedAt,
		errorMessage:   errorMessage,
		filesProcessed: filesProcessed,
		chunksCreated:  chunksCreated,
		createdAt:      createdAt,
		updatedAt:      updatedAt,
		deletedAt:      deletedAt,
	}
}

// ID returns the job ID
func (j *IndexingJob) ID() uuid.UUID {
	return j.id
}

// RepositoryID returns the repository ID
func (j *IndexingJob) RepositoryID() uuid.UUID {
	return j.repositoryID
}

// Status returns the current job status
func (j *IndexingJob) Status() valueobject.JobStatus {
	return j.status
}

// StartedAt returns the job start timestamp
func (j *IndexingJob) StartedAt() *time.Time {
	return j.startedAt
}

// CompletedAt returns the job completion timestamp
func (j *IndexingJob) CompletedAt() *time.Time {
	return j.completedAt
}

// ErrorMessage returns the error message if the job failed
func (j *IndexingJob) ErrorMessage() *string {
	return j.errorMessage
}

// FilesProcessed returns the number of files processed
func (j *IndexingJob) FilesProcessed() int {
	return j.filesProcessed
}

// ChunksCreated returns the number of chunks created
func (j *IndexingJob) ChunksCreated() int {
	return j.chunksCreated
}

// CreatedAt returns the creation timestamp
func (j *IndexingJob) CreatedAt() time.Time {
	return j.createdAt
}

// UpdatedAt returns the last update timestamp
func (j *IndexingJob) UpdatedAt() time.Time {
	return j.updatedAt
}

// DeletedAt returns the deletion timestamp
func (j *IndexingJob) DeletedAt() *time.Time {
	return j.deletedAt
}

// IsDeleted returns true if the job is soft-deleted
func (j *IndexingJob) IsDeleted() bool {
	return j.deletedAt != nil
}

// IsTerminal returns true if the job is in a terminal state
func (j *IndexingJob) IsTerminal() bool {
	return j.status.IsTerminal()
}

// Duration returns the job duration if completed
func (j *IndexingJob) Duration() *time.Duration {
	if j.startedAt == nil || j.completedAt == nil {
		return nil
	}
	duration := j.completedAt.Sub(*j.startedAt)
	return &duration
}

// Start marks the job as started
func (j *IndexingJob) Start() error {
	if !j.status.CanTransitionTo(valueobject.JobStatusRunning) {
		return NewDomainError("cannot start job in current status", "INVALID_STATUS_TRANSITION")
	}

	now := time.Now()
	j.status = valueobject.JobStatusRunning
	j.startedAt = &now
	j.updatedAt = now
	return nil
}

// Complete marks the job as completed successfully
func (j *IndexingJob) Complete(filesProcessed, chunksCreated int) error {
	if !j.status.CanTransitionTo(valueobject.JobStatusCompleted) {
		return NewDomainError("cannot complete job in current status", "INVALID_STATUS_TRANSITION")
	}

	now := time.Now()
	j.status = valueobject.JobStatusCompleted
	j.completedAt = &now
	j.filesProcessed = filesProcessed
	j.chunksCreated = chunksCreated
	j.errorMessage = nil // Clear any previous error
	j.updatedAt = now
	return nil
}

// Fail marks the job as failed with an error message
func (j *IndexingJob) Fail(errorMessage string) error {
	if !j.status.CanTransitionTo(valueobject.JobStatusFailed) {
		return NewDomainError("cannot fail job in current status", "INVALID_STATUS_TRANSITION")
	}

	now := time.Now()
	j.status = valueobject.JobStatusFailed
	j.completedAt = &now
	j.errorMessage = &errorMessage
	j.updatedAt = now
	return nil
}

// Cancel marks the job as cancelled
func (j *IndexingJob) Cancel() error {
	if !j.status.CanTransitionTo(valueobject.JobStatusCancelled) {
		return NewDomainError("cannot cancel job in current status", "INVALID_STATUS_TRANSITION")
	}

	now := time.Now()
	j.status = valueobject.JobStatusCancelled
	j.completedAt = &now
	j.updatedAt = now
	return nil
}

// UpdateProgress updates the job progress
func (j *IndexingJob) UpdateProgress(filesProcessed, chunksCreated int) {
	j.filesProcessed = filesProcessed
	j.chunksCreated = chunksCreated
	j.updatedAt = time.Now()
}

// Archive soft-deletes the job
func (j *IndexingJob) Archive() error {
	if j.IsDeleted() {
		return NewDomainError("job is already archived", "ALREADY_ARCHIVED")
	}

	now := time.Now()
	j.deletedAt = &now
	j.updatedAt = now
	return nil
}

// Equal compares two IndexingJob entities
func (j *IndexingJob) Equal(other *IndexingJob) bool {
	if other == nil {
		return false
	}
	return j.id == other.id
}
