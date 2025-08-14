package query

import "github.com/google/uuid"

// GetRepositoryQuery represents a query to retrieve a single repository.
type GetRepositoryQuery struct {
	ID uuid.UUID `validate:"required"`
}

// ListRepositoriesQuery represents a query to list repositories with filtering and pagination.
type ListRepositoriesQuery struct {
	Status string `validate:"omitempty,oneof=pending cloning processing completed failed archived"`
	Limit  int    `validate:"omitempty,min=1,max=100"`
	Offset int    `validate:"omitempty,min=0"`
	Sort   string `validate:"omitempty,oneof='created_at:asc' 'created_at:desc' 'updated_at:asc' 'updated_at:desc' 'name:asc' 'name:desc'"`
}

// GetIndexingJobQuery represents a query to retrieve a single indexing job.
type GetIndexingJobQuery struct {
	RepositoryID uuid.UUID `validate:"required"`
	JobID        uuid.UUID `validate:"required"`
}

// ListIndexingJobsQuery represents a query to list indexing jobs for a repository.
type ListIndexingJobsQuery struct {
	RepositoryID uuid.UUID `validate:"required"`
	Limit        int       `validate:"omitempty,min=1,max=50"`
	Offset       int       `validate:"omitempty,min=0"`
}
