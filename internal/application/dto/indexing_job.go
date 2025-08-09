package dto

import (
	"time"

	"github.com/google/uuid"
)

// IndexingJobResponse represents the response containing indexing job information
type IndexingJobResponse struct {
	ID             uuid.UUID  `json:"id"`
	RepositoryID   uuid.UUID  `json:"repository_id"`
	Status         string     `json:"status"`
	StartedAt      *time.Time `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
	FilesProcessed int        `json:"files_processed"`
	ChunksCreated  int        `json:"chunks_created"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Duration       *string    `json:"duration,omitempty"` // Human-readable duration
}

// IndexingJobListResponse represents the response for listing indexing jobs
type IndexingJobListResponse struct {
	Jobs       []IndexingJobResponse `json:"jobs"`
	Pagination PaginationResponse    `json:"pagination"`
}

// IndexingJobListQuery represents query parameters for listing indexing jobs
type IndexingJobListQuery struct {
	Limit  int `form:"limit" validate:"omitempty,min=1,max=50"`
	Offset int `form:"offset" validate:"omitempty,min=0"`
}

// DefaultIndexingJobListQuery returns default values for indexing job list query
func DefaultIndexingJobListQuery() IndexingJobListQuery {
	return IndexingJobListQuery{
		Limit:  10,
		Offset: 0,
	}
}
