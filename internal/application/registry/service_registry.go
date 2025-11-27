// Package registry provides service registration and dependency injection for the application.
package registry

import (
	"codechunking/internal/application/service"
	"codechunking/internal/port/outbound"
)

// ServiceRegistry provides centralized service creation and management.
// It acts as a service factory ensuring consistent dependency injection
// patterns across the application.
type ServiceRegistry struct {
	repositoryRepo   outbound.RepositoryRepository
	indexingJobRepo  outbound.IndexingJobRepository
	messagePublisher outbound.MessagePublisher
}

// NewServiceRegistry creates a new service registry with required dependencies.
// All dependencies must be non-nil or the function will panic.
func NewServiceRegistry(
	repositoryRepo outbound.RepositoryRepository,
	indexingJobRepo outbound.IndexingJobRepository,
	messagePublisher outbound.MessagePublisher,
) *ServiceRegistry {
	if repositoryRepo == nil {
		panic("repositoryRepo cannot be nil")
	}
	if indexingJobRepo == nil {
		panic("indexingJobRepo cannot be nil")
	}
	if messagePublisher == nil {
		panic("messagePublisher cannot be nil")
	}

	return &ServiceRegistry{
		repositoryRepo:   repositoryRepo,
		indexingJobRepo:  indexingJobRepo,
		messagePublisher: messagePublisher,
	}
}

// Repository Services

// CreateRepositoryService returns a configured CreateRepositoryService instance.
func (r *ServiceRegistry) CreateRepositoryService() *service.CreateRepositoryService {
	return service.NewCreateRepositoryService(r.repositoryRepo, r.indexingJobRepo, r.messagePublisher)
}

// GetRepositoryService returns a configured GetRepositoryService instance.
func (r *ServiceRegistry) GetRepositoryService() *service.GetRepositoryService {
	return service.NewGetRepositoryService(r.repositoryRepo)
}

// UpdateRepositoryService returns a configured UpdateRepositoryService instance.
func (r *ServiceRegistry) UpdateRepositoryService() *service.UpdateRepositoryService {
	return service.NewUpdateRepositoryService(r.repositoryRepo)
}

// DeleteRepositoryService returns a configured DeleteRepositoryService instance.
func (r *ServiceRegistry) DeleteRepositoryService() *service.DeleteRepositoryService {
	return service.NewDeleteRepositoryService(r.repositoryRepo)
}

// ListRepositoriesService returns a configured ListRepositoriesService instance.
func (r *ServiceRegistry) ListRepositoriesService() *service.ListRepositoriesService {
	return service.NewListRepositoriesService(r.repositoryRepo)
}

// Indexing Job Services

// CreateIndexingJobService returns a configured CreateIndexingJobService instance.
func (r *ServiceRegistry) CreateIndexingJobService() *service.CreateIndexingJobService {
	return service.NewCreateIndexingJobService(r.indexingJobRepo, r.repositoryRepo, r.messagePublisher)
}

// GetIndexingJobService returns a configured GetIndexingJobService instance.
func (r *ServiceRegistry) GetIndexingJobService() *service.GetIndexingJobService {
	return service.NewGetIndexingJobService(r.indexingJobRepo, r.repositoryRepo)
}

// UpdateIndexingJobService returns a configured UpdateIndexingJobService instance.
func (r *ServiceRegistry) UpdateIndexingJobService() *service.UpdateIndexingJobService {
	return service.NewUpdateIndexingJobService(r.indexingJobRepo)
}

// ListIndexingJobsService returns a configured ListIndexingJobsService instance.
func (r *ServiceRegistry) ListIndexingJobsService() *service.ListIndexingJobsService {
	return service.NewListIndexingJobsService(r.indexingJobRepo, r.repositoryRepo)
}
