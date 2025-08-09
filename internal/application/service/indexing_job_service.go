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

// CreateIndexingJobService handles indexing job creation operations.
// It manages the complete lifecycle of creating indexing jobs including
// validation, persistence, and message publishing.
type CreateIndexingJobService struct {
	indexingJobRepo  outbound.IndexingJobRepository // Repository for job persistence
	repositoryRepo   outbound.RepositoryRepository  // Repository for validation
	messagePublisher outbound.MessagePublisher      // Publisher for async processing
}

// NewCreateIndexingJobService creates a new instance of CreateIndexingJobService.
// It requires repositories for both indexing jobs and repositories, plus a message publisher.
func NewCreateIndexingJobService(indexingJobRepo outbound.IndexingJobRepository, repositoryRepo outbound.RepositoryRepository, messagePublisher outbound.MessagePublisher) *CreateIndexingJobService {
	if indexingJobRepo == nil {
		panic("indexingJobRepo cannot be nil")
	}
	if repositoryRepo == nil {
		panic("repositoryRepo cannot be nil")
	}
	if messagePublisher == nil {
		panic("messagePublisher cannot be nil")
	}
	return &CreateIndexingJobService{
		indexingJobRepo:  indexingJobRepo,
		repositoryRepo:   repositoryRepo,
		messagePublisher: messagePublisher,
	}
}

// CreateIndexingJob creates a new indexing job for a repository.
// The repository must exist and not be in processing status for job creation to succeed.
func (s *CreateIndexingJobService) CreateIndexingJob(ctx context.Context, request dto.CreateIndexingJobRequest) (*dto.IndexingJobResponse, error) {
	// Check if repository exists
	repository, err := s.repositoryRepo.FindByID(ctx, request.RepositoryID)
	if err != nil {
		return nil, common.WrapServiceError(common.OpRetrieveRepository, err)
	}

	// Check if repository is eligible for indexing (not already processing)
	if repository.Status() == valueobject.RepositoryStatusProcessing {
		return nil, domain_errors.ErrRepositoryProcessing
	}

	// Create indexing job entity
	job := entity.NewIndexingJob(request.RepositoryID)

	// Save indexing job
	if err := s.indexingJobRepo.Save(ctx, job); err != nil {
		return nil, common.WrapServiceError(common.OpSaveIndexingJob, err)
	}

	// Publish indexing job to message queue
	if err := s.messagePublisher.PublishIndexingJob(ctx, request.RepositoryID, repository.URL().String()); err != nil {
		return nil, common.WrapServiceError(common.OpPublishJob, err)
	}

	return common.EntityToIndexingJobResponse(job), nil
}

// GetIndexingJobService handles indexing job retrieval operations.
// It provides read-only access to indexing job data with repository ownership validation.
type GetIndexingJobService struct {
	indexingJobRepo outbound.IndexingJobRepository // Repository for job data access
	repositoryRepo  outbound.RepositoryRepository  // Repository for ownership validation
}

// NewGetIndexingJobService creates a new instance of GetIndexingJobService.
// It requires repositories for both indexing jobs and repositories for validation.
func NewGetIndexingJobService(indexingJobRepo outbound.IndexingJobRepository, repositoryRepo outbound.RepositoryRepository) *GetIndexingJobService {
	if indexingJobRepo == nil {
		panic("indexingJobRepo cannot be nil")
	}
	if repositoryRepo == nil {
		panic("repositoryRepo cannot be nil")
	}
	return &GetIndexingJobService{
		indexingJobRepo: indexingJobRepo,
		repositoryRepo:  repositoryRepo,
	}
}

// GetIndexingJob retrieves an indexing job with repository ownership validation.
// Ensures the job belongs to the specified repository before returning it.
func (s *GetIndexingJobService) GetIndexingJob(ctx context.Context, repositoryID, jobID uuid.UUID) (*dto.IndexingJobResponse, error) {
	// Verify repository exists
	_, err := s.repositoryRepo.FindByID(ctx, repositoryID)
	if err != nil {
		return nil, common.WrapServiceError(common.OpRetrieveRepository, err)
	}

	// Find indexing job
	job, err := s.indexingJobRepo.FindByID(ctx, jobID)
	if err != nil {
		return nil, common.WrapServiceError(common.OpRetrieveIndexingJob, err)
	}

	// Verify job belongs to the repository
	if job.RepositoryID() != repositoryID {
		return nil, domain_errors.ErrJobNotFound
	}

	return common.EntityToIndexingJobResponse(job), nil
}

// UpdateIndexingJobService handles indexing job update operations.
// It manages state transitions and progress updates for indexing jobs
// while enforcing business rules around valid state transitions.
type UpdateIndexingJobService struct {
	indexingJobRepo outbound.IndexingJobRepository // Repository for job persistence
}

// NewUpdateIndexingJobService creates a new instance of UpdateIndexingJobService.
// It requires an indexing job repository for data persistence operations.
func NewUpdateIndexingJobService(indexingJobRepo outbound.IndexingJobRepository) *UpdateIndexingJobService {
	if indexingJobRepo == nil {
		panic("indexingJobRepo cannot be nil")
	}
	return &UpdateIndexingJobService{
		indexingJobRepo: indexingJobRepo,
	}
}

// UpdateIndexingJob updates an indexing job with status transition validation.
// Validates status transitions and updates job state based on the new status.
func (s *UpdateIndexingJobService) UpdateIndexingJob(ctx context.Context, jobID uuid.UUID, request dto.UpdateIndexingJobRequest) (*dto.IndexingJobResponse, error) {
	// Find indexing job
	job, err := s.indexingJobRepo.FindByID(ctx, jobID)
	if err != nil {
		return nil, common.WrapServiceError(common.OpRetrieveIndexingJob, err)
	}

	// Parse target status
	targetStatus, err := valueobject.NewJobStatus(request.Status)
	if err != nil {
		return nil, fmt.Errorf("invalid job status: %w", err)
	}

	// Validate status transition
	if !job.Status().CanTransitionTo(targetStatus) {
		return nil, fmt.Errorf("invalid status transition from %s to %s", job.Status().String(), targetStatus.String())
	}

	// Update job based on new status
	if err := s.updateJobStatus(job, targetStatus, request); err != nil {
		return nil, err
	}

	// Update progress if provided and job is running
	if job.Status() == valueobject.JobStatusRunning {
		if request.FilesProcessed != nil {
			job.UpdateProgress(*request.FilesProcessed, job.ChunksCreated())
		}
		if request.ChunksCreated != nil {
			job.UpdateProgress(job.FilesProcessed(), *request.ChunksCreated)
		}
	}

	// Save updated job
	if err := s.indexingJobRepo.Update(ctx, job); err != nil {
		return nil, common.WrapServiceError(common.OpSaveIndexingJob, err)
	}

	return common.EntityToIndexingJobResponse(job), nil
}

// ListIndexingJobsService handles indexing job listing operations.
// It provides paginated access to indexing job collections for specific repositories
// with repository ownership validation.
type ListIndexingJobsService struct {
	indexingJobRepo outbound.IndexingJobRepository // Repository for job data access
	repositoryRepo  outbound.RepositoryRepository  // Repository for ownership validation
}

// NewListIndexingJobsService creates a new instance of ListIndexingJobsService.
// It requires repositories for both indexing jobs and repositories for validation.
func NewListIndexingJobsService(indexingJobRepo outbound.IndexingJobRepository, repositoryRepo outbound.RepositoryRepository) *ListIndexingJobsService {
	if indexingJobRepo == nil {
		panic("indexingJobRepo cannot be nil")
	}
	if repositoryRepo == nil {
		panic("repositoryRepo cannot be nil")
	}
	return &ListIndexingJobsService{
		indexingJobRepo: indexingJobRepo,
		repositoryRepo:  repositoryRepo,
	}
}

// ListIndexingJobs retrieves a paginated list of indexing jobs for a repository.
// Validates that the repository exists before retrieving its jobs.
func (s *ListIndexingJobsService) ListIndexingJobs(ctx context.Context, repositoryID uuid.UUID, query dto.IndexingJobListQuery) (*dto.IndexingJobListResponse, error) {
	// Verify repository exists
	_, err := s.repositoryRepo.FindByID(ctx, repositoryID)
	if err != nil {
		return nil, common.WrapServiceError(common.OpRetrieveRepository, err)
	}

	// Apply defaults
	common.ApplyIndexingJobListDefaults(&query)

	// Convert query to filters
	filters := outbound.IndexingJobFilters{
		Limit:  query.Limit,
		Offset: query.Offset,
	}

	// Retrieve indexing jobs
	jobs, total, err := s.indexingJobRepo.FindByRepositoryID(ctx, repositoryID, filters)
	if err != nil {
		return nil, common.WrapServiceError(common.OpListIndexingJobs, err)
	}

	// Convert to response DTOs
	jobDTOs := make([]dto.IndexingJobResponse, len(jobs))
	for i, job := range jobs {
		jobDTOs[i] = *common.EntityToIndexingJobResponse(job)
	}

	// Create pagination response
	pagination := common.CreatePaginationResponse(query.Limit, query.Offset, total)

	return &dto.IndexingJobListResponse{
		Jobs:       jobDTOs,
		Pagination: pagination,
	}, nil
}

// Helper functions

// updateJobStatus handles the status-specific updates for indexing jobs
func (s *UpdateIndexingJobService) updateJobStatus(job *entity.IndexingJob, targetStatus valueobject.JobStatus, request dto.UpdateIndexingJobRequest) error {
	switch targetStatus {
	case valueobject.JobStatusRunning:
		if err := job.Start(); err != nil {
			return common.WrapServiceError(common.OpUpdateIndexingJob, err)
		}
	case valueobject.JobStatusCompleted:
		filesProcessed := 0
		chunksCreated := 0
		if request.FilesProcessed != nil {
			filesProcessed = *request.FilesProcessed
		}
		if request.ChunksCreated != nil {
			chunksCreated = *request.ChunksCreated
		}
		if err := job.Complete(filesProcessed, chunksCreated); err != nil {
			return common.WrapServiceError(common.OpUpdateIndexingJob, err)
		}
	case valueobject.JobStatusFailed:
		errorMessage := ""
		if request.ErrorMessage != nil {
			errorMessage = *request.ErrorMessage
		}
		if err := job.Fail(errorMessage); err != nil {
			return common.WrapServiceError(common.OpUpdateIndexingJob, err)
		}
	case valueobject.JobStatusCancelled:
		if err := job.Cancel(); err != nil {
			return common.WrapServiceError(common.OpUpdateIndexingJob, err)
		}
	}
	return nil
}

// Helper functions for conversion have been moved to common/converter.go
