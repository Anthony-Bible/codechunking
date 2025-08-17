package common

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/entity"
	"strings"
)

// EntityToRepositoryResponse converts a repository entity to response DTO.
func EntityToRepositoryResponse(repository *entity.Repository) *dto.RepositoryResponse {
	return &dto.RepositoryResponse{
		ID:             repository.ID(),
		URL:            repository.URL().String(),
		Name:           repository.Name(),
		Description:    repository.Description(),
		DefaultBranch:  repository.DefaultBranch(),
		LastIndexedAt:  repository.LastIndexedAt(),
		LastCommitHash: repository.LastCommitHash(),
		TotalFiles:     repository.TotalFiles(),
		TotalChunks:    repository.TotalChunks(),
		Status:         repository.Status().String(),
		CreatedAt:      repository.CreatedAt(),
		UpdatedAt:      repository.UpdatedAt(),
	}
}

// EntityToIndexingJobResponse converts an indexing job entity to response DTO.
func EntityToIndexingJobResponse(job *entity.IndexingJob) *dto.IndexingJobResponse {
	response := &dto.IndexingJobResponse{
		ID:             job.ID(),
		RepositoryID:   job.RepositoryID(),
		Status:         job.Status().String(),
		StartedAt:      job.StartedAt(),
		CompletedAt:    job.CompletedAt(),
		ErrorMessage:   job.ErrorMessage(),
		FilesProcessed: job.FilesProcessed(),
		ChunksCreated:  job.ChunksCreated(),
		CreatedAt:      job.CreatedAt(),
		UpdatedAt:      job.UpdatedAt(),
	}

	// Calculate duration if job has started and completed
	if job.StartedAt() != nil && job.CompletedAt() != nil {
		duration := job.CompletedAt().Sub(*job.StartedAt())
		durationStr := duration.String()
		response.Duration = &durationStr
	}

	return response
}

// ExtractRepositoryNameFromURL extracts a repository name from a URL
// This helper function generates a reasonable default name for repositories.
func ExtractRepositoryNameFromURL(url string) string {
	// Extract owner/repo from GitHub-style URLs
	url = strings.TrimSuffix(url, "/")
	url = strings.TrimSuffix(url, ".git")

	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}

	if len(parts) >= 1 {
		return parts[len(parts)-1]
	}

	return "unknown"
}

// CreatePaginationResponse creates a standardized pagination response.
func CreatePaginationResponse(limit, offset, total int) dto.PaginationResponse {
	return dto.PaginationResponse{
		Limit:   limit,
		Offset:  offset,
		Total:   total,
		HasMore: offset+limit < total,
	}
}
