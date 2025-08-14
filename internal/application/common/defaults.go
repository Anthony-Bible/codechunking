package common

import "codechunking/internal/application/dto"

// Default pagination limits.
const (
	DefaultRepositoryListLimit  = 20
	DefaultIndexingJobListLimit = 10
	MaxRepositoryListLimit      = 100
	MaxIndexingJobListLimit     = 50
	DefaultRepositoryListSort   = "created_at:desc"
)

// ApplyRepositoryListDefaults applies default values to repository list query.
func ApplyRepositoryListDefaults(query *dto.RepositoryListQuery) {
	if query.Limit == 0 {
		query.Limit = DefaultRepositoryListLimit
	}
	if query.Sort == "" {
		query.Sort = DefaultRepositoryListSort
	}
}

// ApplyIndexingJobListDefaults applies default values to indexing job list query.
func ApplyIndexingJobListDefaults(query *dto.IndexingJobListQuery) {
	if query.Limit == 0 {
		query.Limit = DefaultIndexingJobListLimit
	}
}
