package dto

import "github.com/google/uuid"

// CreateIndexingJobRequest represents the request to create a new indexing job.
type CreateIndexingJobRequest struct {
	RepositoryID uuid.UUID `json:"repository_id" validate:"required"`
}

// UpdateRepositoryRequest represents the request to update repository metadata.
type UpdateRepositoryRequest struct {
	Name          *string `json:"name,omitempty"           validate:"omitempty,max=255"`
	Description   *string `json:"description,omitempty"    validate:"omitempty,max=1000"`
	DefaultBranch *string `json:"default_branch,omitempty" validate:"omitempty,max=100"`
}

// UpdateIndexingJobRequest represents the request to update an indexing job.
type UpdateIndexingJobRequest struct {
	Status         string  `json:"status"                    validate:"required,oneof=pending running completed failed cancelled"`
	FilesProcessed *int    `json:"files_processed,omitempty" validate:"omitempty,min=0"`
	ChunksCreated  *int    `json:"chunks_created,omitempty"  validate:"omitempty,min=0"`
	ErrorMessage   *string `json:"error_message,omitempty"   validate:"omitempty,max=1000"`
}
