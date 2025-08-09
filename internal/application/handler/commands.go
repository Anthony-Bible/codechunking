package handler

import "github.com/google/uuid"

// Repository Commands and Queries

// CreateRepositoryCommand represents a command to create a repository
type CreateRepositoryCommand struct {
	URL           string
	Name          string
	Description   *string
	DefaultBranch *string
}

// UpdateRepositoryCommand represents a command to update a repository
type UpdateRepositoryCommand struct {
	ID            uuid.UUID
	Name          *string
	Description   *string
	DefaultBranch *string
}

// DeleteRepositoryCommand represents a command to delete a repository
type DeleteRepositoryCommand struct {
	ID uuid.UUID
}

// GetRepositoryQuery represents a query to get a repository
type GetRepositoryQuery struct {
	ID uuid.UUID
}

// ListRepositoriesQuery represents a query to list repositories
type ListRepositoriesQuery struct {
	Status string
	Limit  int
	Offset int
	Sort   string
}

// IndexingJob Commands and Queries

// CreateIndexingJobCommand represents a command to create an indexing job
type CreateIndexingJobCommand struct {
	RepositoryID uuid.UUID
}

// UpdateIndexingJobCommand represents a command to update an indexing job
type UpdateIndexingJobCommand struct {
	JobID          uuid.UUID
	Status         string
	FilesProcessed *int
	ChunksCreated  *int
	ErrorMessage   *string
}

// GetIndexingJobQuery represents a query to get an indexing job
type GetIndexingJobQuery struct {
	RepositoryID uuid.UUID
	JobID        uuid.UUID
}

// ListIndexingJobsQuery represents a query to list indexing jobs
type ListIndexingJobsQuery struct {
	RepositoryID uuid.UUID
	Limit        int
	Offset       int
}
