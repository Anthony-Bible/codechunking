package dto

import (
	"time"

	"github.com/google/uuid"
)

// CreateRepositoryRequest represents the request to create a new repository.
type CreateRepositoryRequest struct {
	URL           string  `json:"url"                      validate:"required,url"`
	Name          string  `json:"name,omitempty"           validate:"omitempty,max=255"`
	Description   *string `json:"description,omitempty"    validate:"omitempty,max=1000"`
	DefaultBranch *string `json:"default_branch,omitempty" validate:"omitempty,max=100"`
}

// RepositoryResponse represents the response containing repository information.
type RepositoryResponse struct {
	ID             uuid.UUID  `json:"id"`
	URL            string     `json:"url"`
	Name           string     `json:"name"`
	Description    *string    `json:"description"`
	DefaultBranch  *string    `json:"default_branch"`
	LastIndexedAt  *time.Time `json:"last_indexed_at"`
	LastCommitHash *string    `json:"last_commit_hash"`
	TotalFiles     int        `json:"total_files"`
	TotalChunks    int        `json:"total_chunks"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// RepositoryListResponse represents the response for listing repositories.
type RepositoryListResponse struct {
	Repositories []RepositoryResponse `json:"repositories"`
	Pagination   PaginationResponse   `json:"pagination"`
}

// RepositoryListQuery represents query parameters for listing repositories.
type RepositoryListQuery struct {
	Status string `form:"status" validate:"omitempty,oneof=pending cloning processing completed failed archived"`
	Limit  int    `form:"limit"  validate:"omitempty,min=1,max=100"`
	Offset int    `form:"offset" validate:"omitempty,min=0"`
	Sort   string `form:"sort"   validate:"omitempty,oneof='created_at:asc' 'created_at:desc' 'updated_at:asc' 'updated_at:desc' 'name:asc' 'name:desc'"`
}

// DefaultRepositoryListQuery returns default values for repository list query.
func DefaultRepositoryListQuery() RepositoryListQuery {
	return RepositoryListQuery{
		Limit:  20,
		Offset: 0,
		Sort:   "created_at:desc",
	}
}

// PaginationResponse represents pagination metadata.
type PaginationResponse struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}
