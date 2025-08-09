package service

import (
	"context"

	"codechunking/internal/application/dto"
	"codechunking/internal/application/registry"
	"codechunking/internal/port/inbound"

	"github.com/google/uuid"
)

// RepositoryServiceAdapter adapts the application service layer to the inbound port
// It implements the inbound.RepositoryService interface by delegating to application services
type RepositoryServiceAdapter struct {
	serviceRegistry *registry.ServiceRegistry
}

// NewRepositoryServiceAdapter creates a new RepositoryServiceAdapter
func NewRepositoryServiceAdapter(serviceRegistry *registry.ServiceRegistry) inbound.RepositoryService {
	return &RepositoryServiceAdapter{
		serviceRegistry: serviceRegistry,
	}
}

// CreateRepository handles repository creation
func (a *RepositoryServiceAdapter) CreateRepository(ctx context.Context, request dto.CreateRepositoryRequest) (*dto.RepositoryResponse, error) {
	service := a.serviceRegistry.CreateRepositoryService()
	return service.CreateRepository(ctx, request)
}

// GetRepository retrieves a repository by ID
func (a *RepositoryServiceAdapter) GetRepository(ctx context.Context, id uuid.UUID) (*dto.RepositoryResponse, error) {
	service := a.serviceRegistry.GetRepositoryService()
	return service.GetRepository(ctx, id)
}

// ListRepositories lists repositories with filtering and pagination
func (a *RepositoryServiceAdapter) ListRepositories(ctx context.Context, listQuery dto.RepositoryListQuery) (*dto.RepositoryListResponse, error) {
	service := a.serviceRegistry.ListRepositoriesService()
	return service.ListRepositories(ctx, listQuery)
}

// DeleteRepository soft deletes a repository
func (a *RepositoryServiceAdapter) DeleteRepository(ctx context.Context, id uuid.UUID) error {
	service := a.serviceRegistry.DeleteRepositoryService()
	return service.DeleteRepository(ctx, id)
}

// GetRepositoryJobs lists indexing jobs for a repository
func (a *RepositoryServiceAdapter) GetRepositoryJobs(ctx context.Context, repositoryID uuid.UUID, listQuery dto.IndexingJobListQuery) (*dto.IndexingJobListResponse, error) {
	service := a.serviceRegistry.ListIndexingJobsService()
	return service.ListIndexingJobs(ctx, repositoryID, listQuery)
}

// GetIndexingJob retrieves a specific indexing job
func (a *RepositoryServiceAdapter) GetIndexingJob(ctx context.Context, repositoryID, jobID uuid.UUID) (*dto.IndexingJobResponse, error) {
	service := a.serviceRegistry.GetIndexingJobService()
	return service.GetIndexingJob(ctx, repositoryID, jobID)
}
