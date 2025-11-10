package dto

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultRepositoryLimit is the default number of repositories to return in a list query.
	DefaultRepositoryLimit = 20

	// MaxNameFilterLength is the maximum length for name filter.
	MaxNameFilterLength = 255
	// MaxURLFilterLength is the maximum length for URL filter.
	MaxURLFilterLength = 500
	// MaxLimitValue is the maximum limit value for pagination.
	MaxLimitValue = 100
	// MinLimitValue is the minimum limit value for pagination.
	MinLimitValue = 1
)

// Valid status values for repository list queries.
const (
	StatusPending    = "pending"
	StatusCloning    = "cloning"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusArchived   = "archived"
)

// Valid sort values for repository list queries.
const (
	SortCreatedAtAsc  = "created_at:asc"
	SortCreatedAtDesc = "created_at:desc"
	SortUpdatedAtAsc  = "updated_at:asc"
	SortUpdatedAtDesc = "updated_at:desc"
	SortNameAsc       = "name:asc"
	SortNameDesc      = "name:desc"
)

// isValidStatus checks if the given status is valid.
func isValidStatus(status string) bool {
	validStatuses := map[string]bool{
		StatusPending:    true,
		StatusCloning:    true,
		StatusProcessing: true,
		StatusCompleted:  true,
		StatusFailed:     true,
		StatusArchived:   true,
	}
	return validStatuses[status]
}

// isValidSort checks if the given sort parameter is valid.
func isValidSort(sort string) bool {
	validSorts := map[string]bool{
		SortCreatedAtAsc:  true,
		SortCreatedAtDesc: true,
		SortUpdatedAtAsc:  true,
		SortUpdatedAtDesc: true,
		SortNameAsc:       true,
		SortNameDesc:      true,
	}
	return validSorts[sort]
}

// CreateRepositoryRequest represents the request to create a new repository.
type CreateRepositoryRequest struct {
	URL           string  `json:"url"                      validate:"required,url"       example:"https://github.com/golang/go"`
	Name          string  `json:"name,omitempty"           validate:"omitempty,max=255"  example:"golang/go"`
	Description   *string `json:"description,omitempty"    validate:"omitempty,max=1000" example:"The Go programming language"`
	DefaultBranch *string `json:"default_branch,omitempty" validate:"omitempty,max=100"  example:"master"`
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
	Name   string `form:"name"   validate:"omitempty,max=255"`
	URL    string `form:"url"    validate:"omitempty,max=500"`
	Limit  int    `form:"limit"  validate:"omitempty,min=1,max=100"`
	Offset int    `form:"offset" validate:"omitempty,min=0"`
	Sort   string `form:"sort"   validate:"omitempty,oneof='created_at:asc' 'created_at:desc' 'updated_at:asc' 'updated_at:desc' 'name:asc' 'name:desc'"`
}

// DefaultRepositoryListQuery returns default values for repository list query.
func DefaultRepositoryListQuery() RepositoryListQuery {
	return RepositoryListQuery{
		Limit:  DefaultRepositoryLimit,
		Offset: 0,
		Sort:   "created_at:desc",
	}
}

// Validate validates the repository list query parameters.
func (q *RepositoryListQuery) Validate() error {
	if len(q.Name) > MaxNameFilterLength {
		return errors.New("name filter cannot exceed 255 characters")
	}
	if len(q.URL) > MaxURLFilterLength {
		return errors.New("url filter cannot exceed 500 characters")
	}
	if q.Status != "" && !isValidStatus(q.Status) {
		return errors.New("status must be one of: pending, cloning, processing, completed, failed, archived")
	}
	if q.Limit < 0 || q.Limit > MaxLimitValue {
		if q.Limit != 0 {
			return errors.New("limit must be between 1 and 100")
		}
	}
	if q.Offset < 0 {
		return errors.New("offset must be non-negative")
	}
	if q.Sort != "" && !isValidSort(q.Sort) {
		return errors.New(
			"sort must be one of: created_at:asc, created_at:desc, updated_at:asc, updated_at:desc, name:asc, name:desc",
		)
	}
	return nil
}

// PaginationResponse represents pagination metadata.
type PaginationResponse struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}
