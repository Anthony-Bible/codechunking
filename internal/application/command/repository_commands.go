package command

import "github.com/google/uuid"

// CreateRepositoryCommand represents a command to create a new repository
type CreateRepositoryCommand struct {
	URL           string  `validate:"required,url"`
	Name          string  `validate:"omitempty,max=255"`
	Description   *string `validate:"omitempty,max=1000"`
	DefaultBranch *string `validate:"omitempty,max=100"`
}

// UpdateRepositoryCommand represents a command to update repository metadata
type UpdateRepositoryCommand struct {
	ID            uuid.UUID `validate:"required"`
	Name          *string   `validate:"omitempty,max=255"`
	Description   *string   `validate:"omitempty,max=1000"`
	DefaultBranch *string   `validate:"omitempty,max=100"`
}

// DeleteRepositoryCommand represents a command to delete a repository
type DeleteRepositoryCommand struct {
	ID uuid.UUID `validate:"required"`
}

// CreateIndexingJobCommand represents a command to create an indexing job
type CreateIndexingJobCommand struct {
	RepositoryID uuid.UUID `validate:"required"`
}

// UpdateIndexingJobCommand represents a command to update an indexing job
type UpdateIndexingJobCommand struct {
	JobID          uuid.UUID `validate:"required"`
	Status         string    `validate:"required,oneof=pending running completed failed cancelled"`
	FilesProcessed *int      `validate:"omitempty,min=0"`
	ChunksCreated  *int      `validate:"omitempty,min=0"`
	ErrorMessage   *string   `validate:"omitempty,max=1000"`
}
