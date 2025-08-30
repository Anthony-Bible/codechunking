// Package inbound defines the inbound ports (interfaces) for the application layer.
// These ports represent the entry points into the application's core business logic.
package inbound

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/entity"
	"context"

	"github.com/google/uuid"
)

// RepositoryService defines the inbound port for repository operations.
type RepositoryService interface {
	CreateRepository(ctx context.Context, request dto.CreateRepositoryRequest) (*dto.RepositoryResponse, error)
	GetRepository(ctx context.Context, id uuid.UUID) (*dto.RepositoryResponse, error)
	ListRepositories(ctx context.Context, query dto.RepositoryListQuery) (*dto.RepositoryListResponse, error)
	DeleteRepository(ctx context.Context, id uuid.UUID) error
	GetRepositoryJobs(
		ctx context.Context,
		repositoryID uuid.UUID,
		query dto.IndexingJobListQuery,
	) (*dto.IndexingJobListResponse, error)
	GetIndexingJob(ctx context.Context, repositoryID, jobID uuid.UUID) (*dto.IndexingJobResponse, error)
}

// HealthService defines the inbound port for health check operations.
type HealthService interface {
	GetHealth(ctx context.Context) (*dto.HealthResponse, error)
}

// RepositoryMapper provides methods to map between domain entities and DTOs.
type RepositoryMapper interface {
	EntityToDTO(entity *entity.Repository) dto.RepositoryResponse
	DTOToEntity(dto dto.CreateRepositoryRequest) (*entity.Repository, error)
	EntitiesToDTOs(entities []*entity.Repository) []dto.RepositoryResponse
}

// IndexingJobMapper provides methods to map between domain entities and DTOs.
type IndexingJobMapper interface {
	EntityToDTO(entity *entity.IndexingJob) dto.IndexingJobResponse
	EntitiesToDTOs(entities []*entity.IndexingJob) []dto.IndexingJobResponse
}
