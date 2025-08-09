package service

import (
	"context"
	"fmt"

	"codechunking/internal/application/common"
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/entity"
	domain_errors "codechunking/internal/domain/errors/domain"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"

	"github.com/google/uuid"
)

// CreateRepositoryService handles repository creation operations.
// It manages the complete lifecycle of creating a new repository including
// validation, persistence, and publishing indexing jobs.
type CreateRepositoryService struct {
	repositoryRepo   outbound.RepositoryRepository // Repository for persistence operations
	messagePublisher outbound.MessagePublisher     // Publisher for async indexing jobs
}

// NewCreateRepositoryService creates a new instance of CreateRepositoryService.
// It requires a repository repository for data persistence and a message publisher
// for publishing indexing jobs to the message queue.
func NewCreateRepositoryService(repositoryRepo outbound.RepositoryRepository, messagePublisher outbound.MessagePublisher) *CreateRepositoryService {
	if repositoryRepo == nil {
		panic("repositoryRepo cannot be nil")
	}
	if messagePublisher == nil {
		panic("messagePublisher cannot be nil")
	}
	return &CreateRepositoryService{
		repositoryRepo:   repositoryRepo,
		messagePublisher: messagePublisher,
	}
}

// CreateRepository creates a new repository and publishes an indexing job.
// It validates the repository URL, ensures it doesn't already exist, creates the entity,
// saves it to the repository, publishes an indexing job, and returns the response.
func (s *CreateRepositoryService) CreateRepository(ctx context.Context, request dto.CreateRepositoryRequest) (*dto.RepositoryResponse, error) {
	// Validate and create repository URL
	repositoryURL, err := valueobject.NewRepositoryURL(request.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}

	// Check if repository already exists
	exists, err := s.repositoryRepo.Exists(ctx, repositoryURL)
	if err != nil {
		return nil, common.WrapServiceError(common.OpCheckRepositoryExists, err)
	}
	if exists {
		return nil, domain_errors.ErrRepositoryAlreadyExists
	}

	// Auto-generate name from URL if not provided
	name := request.Name
	if name == "" {
		name = common.ExtractRepositoryNameFromURL(request.URL)
	}

	// Create repository entity
	repository := entity.NewRepository(repositoryURL, name, request.Description, request.DefaultBranch)

	// Save repository
	if err := s.repositoryRepo.Save(ctx, repository); err != nil {
		return nil, common.WrapServiceError(common.OpSaveRepository, err)
	}

	// Publish indexing job
	if err := s.messagePublisher.PublishIndexingJob(ctx, repository.ID(), repository.URL().String()); err != nil {
		return nil, common.WrapServiceError(common.OpPublishJob, err)
	}

	// Return response
	return common.EntityToRepositoryResponse(repository), nil
}

// GetRepositoryService handles repository retrieval operations.
// It provides read-only access to repository data.
type GetRepositoryService struct {
	repositoryRepo outbound.RepositoryRepository // Repository for data access
}

// NewGetRepositoryService creates a new instance of GetRepositoryService.
// It requires a repository repository for data access operations.
func NewGetRepositoryService(repositoryRepo outbound.RepositoryRepository) *GetRepositoryService {
	if repositoryRepo == nil {
		panic("repositoryRepo cannot be nil")
	}
	return &GetRepositoryService{
		repositoryRepo: repositoryRepo,
	}
}

// GetRepository retrieves a repository by its unique ID.
// Returns an error if the repository is not found or if there's a database error.
func (s *GetRepositoryService) GetRepository(ctx context.Context, id uuid.UUID) (*dto.RepositoryResponse, error) {
	repository, err := s.repositoryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, common.WrapServiceError(common.OpRetrieveRepository, err)
	}

	return common.EntityToRepositoryResponse(repository), nil
}

// UpdateRepositoryService handles repository update operations.
// It manages metadata updates while enforcing business rules around
// repository state transitions.
type UpdateRepositoryService struct {
	repositoryRepo outbound.RepositoryRepository // Repository for persistence operations
}

// NewUpdateRepositoryService creates a new instance of UpdateRepositoryService.
// It requires a repository repository for data persistence operations.
func NewUpdateRepositoryService(repositoryRepo outbound.RepositoryRepository) *UpdateRepositoryService {
	if repositoryRepo == nil {
		panic("repositoryRepo cannot be nil")
	}
	return &UpdateRepositoryService{
		repositoryRepo: repositoryRepo,
	}
}

// UpdateRepository updates repository metadata (name, description, default branch).
// The repository must not be in processing status to be updated.
func (s *UpdateRepositoryService) UpdateRepository(ctx context.Context, id uuid.UUID, request dto.UpdateRepositoryRequest) (*dto.RepositoryResponse, error) {
	// Find repository
	repository, err := s.repositoryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, common.WrapServiceError(common.OpRetrieveRepository, err)
	}

	// Check if repository can be updated (not processing)
	if repository.Status() == valueobject.RepositoryStatusProcessing {
		return nil, domain_errors.ErrRepositoryProcessing
	}

	// Update fields
	if request.Name != nil {
		repository.UpdateName(*request.Name)
	}
	if request.Description != nil {
		repository.UpdateDescription(request.Description)
	}
	if request.DefaultBranch != nil {
		repository.UpdateDefaultBranch(request.DefaultBranch)
	}

	// Save updated repository
	if err := s.repositoryRepo.Update(ctx, repository); err != nil {
		return nil, common.WrapServiceError(common.OpSaveRepository, err)
	}

	return common.EntityToRepositoryResponse(repository), nil
}

// DeleteRepositoryService handles repository deletion operations.
// It implements soft deletion by archiving repositories rather than
// physically removing them from storage.
type DeleteRepositoryService struct {
	repositoryRepo outbound.RepositoryRepository // Repository for persistence operations
}

// NewDeleteRepositoryService creates a new instance of DeleteRepositoryService.
// It requires a repository repository for data persistence operations.
func NewDeleteRepositoryService(repositoryRepo outbound.RepositoryRepository) *DeleteRepositoryService {
	if repositoryRepo == nil {
		panic("repositoryRepo cannot be nil")
	}
	return &DeleteRepositoryService{
		repositoryRepo: repositoryRepo,
	}
}

// DeleteRepository soft deletes a repository by marking it as archived.
// The repository must not be in processing status to be deleted.
func (s *DeleteRepositoryService) DeleteRepository(ctx context.Context, id uuid.UUID) error {
	// Find repository
	repository, err := s.repositoryRepo.FindByID(ctx, id)
	if err != nil {
		return common.WrapServiceError(common.OpRetrieveRepository, err)
	}

	// Check if repository can be deleted (not processing)
	if repository.Status() == valueobject.RepositoryStatusProcessing {
		return domain_errors.ErrRepositoryProcessing
	}

	// Archive repository
	if err := repository.Archive(); err != nil {
		return common.WrapServiceError(common.OpDeleteRepository, err)
	}

	// Save updated repository
	if err := s.repositoryRepo.Update(ctx, repository); err != nil {
		return common.WrapServiceError(common.OpSaveRepository, err)
	}

	return nil
}

// ListRepositoriesService handles repository listing operations.
// It provides paginated access to repository collections with filtering
// and sorting capabilities.
type ListRepositoriesService struct {
	repositoryRepo outbound.RepositoryRepository // Repository for data access
}

// NewListRepositoriesService creates a new instance of ListRepositoriesService.
// It requires a repository repository for data access operations.
func NewListRepositoriesService(repositoryRepo outbound.RepositoryRepository) *ListRepositoriesService {
	if repositoryRepo == nil {
		panic("repositoryRepo cannot be nil")
	}
	return &ListRepositoriesService{
		repositoryRepo: repositoryRepo,
	}
}

// ListRepositories retrieves a paginated list of repositories with optional filtering.
// Supports filtering by status and sorting by various fields.
func (s *ListRepositoriesService) ListRepositories(ctx context.Context, query dto.RepositoryListQuery) (*dto.RepositoryListResponse, error) {
	// Apply defaults
	common.ApplyRepositoryListDefaults(&query)

	// Convert query to filters
	filters := outbound.RepositoryFilters{
		Limit:  query.Limit,
		Offset: query.Offset,
		Sort:   query.Sort,
	}

	// Parse status filter if provided
	if query.Status != "" {
		status, err := valueobject.NewRepositoryStatus(query.Status)
		if err != nil {
			return nil, fmt.Errorf("invalid repository status: %w", err)
		}
		filters.Status = &status
	}

	// Retrieve repositories
	repositories, total, err := s.repositoryRepo.FindAll(ctx, filters)
	if err != nil {
		return nil, common.WrapServiceError(common.OpListRepositories, err)
	}

	// Convert to response DTOs
	repositoryDTOs := make([]dto.RepositoryResponse, len(repositories))
	for i, repo := range repositories {
		repositoryDTOs[i] = *common.EntityToRepositoryResponse(repo)
	}

	// Create pagination response
	pagination := common.CreatePaginationResponse(query.Limit, query.Offset, total)

	return &dto.RepositoryListResponse{
		Repositories: repositoryDTOs,
		Pagination:   pagination,
	}, nil
}

// Helper functions have been moved to common/converter.go for better reusability
