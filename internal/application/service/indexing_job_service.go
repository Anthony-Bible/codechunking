package service

import (
	"codechunking/internal/application/common"
	"codechunking/internal/application/defaults"
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"strings"

	domainerrors "codechunking/internal/domain/errors/domain"

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
func NewCreateIndexingJobService(
	indexingJobRepo outbound.IndexingJobRepository,
	repositoryRepo outbound.RepositoryRepository,
	messagePublisher outbound.MessagePublisher,
) *CreateIndexingJobService {
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
func (s *CreateIndexingJobService) CreateIndexingJob(
	ctx context.Context,
	request dto.CreateIndexingJobRequest,
) (*dto.IndexingJobResponse, error) {
	// Validate that the repository exists and retrieve its current state
	repository, err := s.retrieveAndValidateRepository(ctx, request.RepositoryID)
	if err != nil {
		return nil, err
	}

	// Ensure repository is in a state that allows indexing job creation
	if eligibilityErr := s.validateRepositoryEligibilityForIndexing(repository); eligibilityErr != nil {
		return nil, eligibilityErr
	}

	// Create new indexing job entity
	job := entity.NewIndexingJob(request.RepositoryID)

	// Persist the job to ensure it's saved before queuing for processing
	if persistErr := s.persistIndexingJob(ctx, job); persistErr != nil {
		return nil, persistErr
	}

	// Queue the job for asynchronous processing
	if queueErr := s.queueJobForProcessing(ctx, request.RepositoryID, repository.URL().String()); queueErr != nil {
		return nil, queueErr
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
func NewGetIndexingJobService(
	indexingJobRepo outbound.IndexingJobRepository,
	repositoryRepo outbound.RepositoryRepository,
) *GetIndexingJobService {
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
func (s *GetIndexingJobService) GetIndexingJob(
	ctx context.Context,
	repositoryID, jobID uuid.UUID,
) (*dto.IndexingJobResponse, error) {
	// Validate that the repository exists (ensures user has access to valid repository)
	if err := s.validateRepositoryExists(ctx, repositoryID); err != nil {
		return nil, err
	}

	// Retrieve the requested indexing job
	job, err := s.retrieveIndexingJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	// Ensure the job belongs to the specified repository (prevents cross-repository access)
	err = s.validateJobOwnership(job, repositoryID)
	if err != nil {
		return nil, err
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
func (s *UpdateIndexingJobService) UpdateIndexingJob(
	ctx context.Context,
	jobID uuid.UUID,
	request dto.UpdateIndexingJobRequest,
) (*dto.IndexingJobResponse, error) {
	// Retrieve the indexing job to be updated
	job, err := s.retrieveIndexingJobForUpdate(ctx, jobID)
	if err != nil {
		return nil, err
	}

	// Parse and validate the target status
	targetStatus, err := s.parseAndValidateTargetStatus(request.Status)
	if err != nil {
		return nil, err
	}

	// Validate that the status transition is allowed
	err = s.validateStatusTransition(job, targetStatus)
	if err != nil {
		return nil, err
	}

	// Update job based on new status (only if status actually changes)
	if job.Status() != targetStatus {
		err = s.updateJobStatus(job, targetStatus, request)
		if err != nil {
			return nil, err
		}
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
	err = s.indexingJobRepo.Update(ctx, job)
	if err != nil {
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
func NewListIndexingJobsService(
	indexingJobRepo outbound.IndexingJobRepository,
	repositoryRepo outbound.RepositoryRepository,
) *ListIndexingJobsService {
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
func (s *ListIndexingJobsService) ListIndexingJobs(
	ctx context.Context,
	repositoryID uuid.UUID,
	query dto.IndexingJobListQuery,
) (*dto.IndexingJobListResponse, error) {
	// Verify repository exists
	_, err := s.repositoryRepo.FindByID(ctx, repositoryID)
	if err != nil {
		return nil, common.WrapServiceError(common.OpRetrieveRepository, err)
	}

	// Apply defaults
	defaults.ApplyIndexingJobListDefaults(&query)

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

// jobStatusTransitionHandler encapsulates the logic for handling job status transitions.
type jobStatusTransitionHandler struct {
	service *UpdateIndexingJobService
}

// newJobStatusTransitionHandler creates a new job status transition handler.
func newJobStatusTransitionHandler(service *UpdateIndexingJobService) *jobStatusTransitionHandler {
	return &jobStatusTransitionHandler{service: service}
}

// handleTransition manages status transitions with comprehensive business logic.
func (h *jobStatusTransitionHandler) handleTransition(
	job *entity.IndexingJob,
	targetStatus valueobject.JobStatus,
	request dto.UpdateIndexingJobRequest,
) error {
	// Define transition handlers for each supported status
	transitionHandlers := map[valueobject.JobStatus]func() error{
		valueobject.JobStatusPending: func() error {
			// Pending status doesn't require special transition logic
			return nil
		},
		valueobject.JobStatusRunning: func() error {
			return h.service.transitionJobToRunning(job)
		},
		valueobject.JobStatusCompleted: func() error {
			return h.service.transitionJobToCompleted(job, request)
		},
		valueobject.JobStatusFailed: func() error {
			return h.service.transitionJobToFailed(job, request)
		},
		valueobject.JobStatusCancelled: func() error {
			return h.service.transitionJobToCancelled(job)
		},
	}

	handler, isSupported := transitionHandlers[targetStatus]
	if !isSupported {
		return fmt.Errorf("unsupported job status transition from '%s' to '%s': this transition is not implemented",
			job.Status().String(), targetStatus.String())
	}

	return handler()
}

// updateJobStatus handles the status-specific updates for indexing jobs with improved business logic.
func (s *UpdateIndexingJobService) updateJobStatus(
	job *entity.IndexingJob,
	targetStatus valueobject.JobStatus,
	request dto.UpdateIndexingJobRequest,
) error {
	handler := newJobStatusTransitionHandler(s)
	return handler.handleTransition(job, targetStatus, request)
}

// transitionJobToRunning handles the business logic for starting a job.
func (s *UpdateIndexingJobService) transitionJobToRunning(job *entity.IndexingJob) error {
	if err := job.Start(); err != nil {
		return common.WrapServiceError(common.OpUpdateIndexingJob,
			fmt.Errorf("failed to start indexing job: %w", err))
	}
	return nil
}

// transitionJobToCompleted handles the business logic for completing a job.
func (s *UpdateIndexingJobService) transitionJobToCompleted(
	job *entity.IndexingJob,
	request dto.UpdateIndexingJobRequest,
) error {
	// Extract completion metrics with defaults
	filesProcessed := s.extractFilesProcessed(request)
	chunksCreated := s.extractChunksCreated(request)

	if err := job.Complete(filesProcessed, chunksCreated); err != nil {
		return common.WrapServiceError(common.OpUpdateIndexingJob,
			fmt.Errorf("failed to complete indexing job with %d files processed and %d chunks created: %w",
				filesProcessed, chunksCreated, err))
	}
	return nil
}

// transitionJobToFailed handles the business logic for marking a job as failed.
func (s *UpdateIndexingJobService) transitionJobToFailed(
	job *entity.IndexingJob,
	request dto.UpdateIndexingJobRequest,
) error {
	errorMessage := s.extractErrorMessage(request)

	if err := job.Fail(errorMessage); err != nil {
		return common.WrapServiceError(common.OpUpdateIndexingJob,
			fmt.Errorf("failed to mark indexing job as failed: %w", err))
	}
	return nil
}

// transitionJobToCancelled handles the business logic for cancelling a job.
func (s *UpdateIndexingJobService) transitionJobToCancelled(job *entity.IndexingJob) error {
	if err := job.Cancel(); err != nil {
		return common.WrapServiceError(common.OpUpdateIndexingJob,
			fmt.Errorf("failed to cancel indexing job: %w", err))
	}
	return nil
}

// jobMetricsExtractor safely extracts metrics and error information from update requests.
type jobMetricsExtractor struct{}

// newJobMetricsExtractor creates a new job metrics extractor.
func newJobMetricsExtractor() *jobMetricsExtractor {
	return &jobMetricsExtractor{}
}

// extractFilesProcessed safely extracts files processed count with sensible defaults.
func (jme *jobMetricsExtractor) extractFilesProcessed(request dto.UpdateIndexingJobRequest) int {
	if request.FilesProcessed != nil {
		return *request.FilesProcessed
	}
	return 0
}

// extractChunksCreated safely extracts chunks created count with sensible defaults.
func (jme *jobMetricsExtractor) extractChunksCreated(request dto.UpdateIndexingJobRequest) int {
	if request.ChunksCreated != nil {
		return *request.ChunksCreated
	}
	return 0
}

// extractErrorMessage safely extracts error message with context-aware defaults.
func (jme *jobMetricsExtractor) extractErrorMessage(request dto.UpdateIndexingJobRequest) string {
	if request.ErrorMessage != nil && strings.TrimSpace(*request.ErrorMessage) != "" {
		return strings.TrimSpace(*request.ErrorMessage)
	}
	return "Indexing job failed due to an unspecified error during processing"
}

// Legacy methods for backward compatibility that delegate to the extractor.
func (s *UpdateIndexingJobService) extractFilesProcessed(request dto.UpdateIndexingJobRequest) int {
	extractor := newJobMetricsExtractor()
	return extractor.extractFilesProcessed(request)
}

func (s *UpdateIndexingJobService) extractChunksCreated(request dto.UpdateIndexingJobRequest) int {
	extractor := newJobMetricsExtractor()
	return extractor.extractChunksCreated(request)
}

func (s *UpdateIndexingJobService) extractErrorMessage(request dto.UpdateIndexingJobRequest) string {
	extractor := newJobMetricsExtractor()
	return extractor.extractErrorMessage(request)
}

// Helper methods for CreateIndexingJobService

// retrieveAndValidateRepository retrieves a repository with enhanced error context and validation.
func (s *CreateIndexingJobService) retrieveAndValidateRepository(
	ctx context.Context,
	repositoryID uuid.UUID,
) (*entity.Repository, error) {
	repository, err := s.repositoryRepo.FindByID(ctx, repositoryID)
	if err != nil {
		return nil, common.WrapServiceError(
			common.OpRetrieveRepository,
			fmt.Errorf(
				"failed to retrieve repository for indexing job creation: repository ID %s was not found in the system or is not accessible. Please verify the repository exists and you have permission to access it: %w",
				repositoryID,
				err,
			),
		)
	}
	return repository, nil
}

// repositoryEligibilityChecker encapsulates the business rules for repository indexing eligibility.
type repositoryEligibilityChecker struct{}

// newRepositoryEligibilityChecker creates a new repository eligibility checker.
func newRepositoryEligibilityChecker() *repositoryEligibilityChecker {
	return &repositoryEligibilityChecker{}
}

// checkEligibility validates whether a repository is eligible for indexing job creation.
func (rec *repositoryEligibilityChecker) checkEligibility(repository *entity.Repository) error {
	currentStatus := repository.Status()

	// Check for ineligible states and return appropriate domain errors
	switch currentStatus {
	case valueobject.RepositoryStatusProcessing, valueobject.RepositoryStatusCloning:
		return domainerrors.ErrRepositoryProcessing
	case valueobject.RepositoryStatusPending:
		// Pending repositories are eligible for indexing job creation
		return nil
	case valueobject.RepositoryStatusCompleted:
		// Completed repositories are eligible for re-indexing
		return nil
	case valueobject.RepositoryStatusFailed:
		// Failed repositories are eligible for retry indexing
		return nil
	case valueobject.RepositoryStatusArchived:
		// Archived repositories are not eligible for indexing
		return domainerrors.ErrRepositoryProcessing
	default:
		// Unknown status - allow for forward compatibility
		return nil
	}
}

// validateRepositoryEligibilityForIndexing checks if a repository can have an indexing job created.
func (s *CreateIndexingJobService) validateRepositoryEligibilityForIndexing(repository *entity.Repository) error {
	checker := newRepositoryEligibilityChecker()
	return checker.checkEligibility(repository)
}

// persistIndexingJob saves an indexing job to the repository.
func (s *CreateIndexingJobService) persistIndexingJob(ctx context.Context, job *entity.IndexingJob) error {
	if err := s.indexingJobRepo.Save(ctx, job); err != nil {
		return common.WrapServiceError(common.OpSaveIndexingJob,
			fmt.Errorf("failed to save indexing job to database: %w", err))
	}
	return nil
}

// queueJobForProcessing publishes an indexing job to the message queue for async processing.
func (s *CreateIndexingJobService) queueJobForProcessing(
	ctx context.Context,
	repositoryID uuid.UUID,
	repositoryURL string,
) error {
	if err := s.messagePublisher.PublishIndexingJob(ctx, repositoryID, repositoryURL); err != nil {
		return common.WrapServiceError(common.OpPublishJob,
			fmt.Errorf("failed to queue indexing job for processing: %w", err))
	}
	return nil
}

// Helper methods for GetIndexingJobService

// validateRepositoryExists ensures a repository exists before proceeding with job operations.
func (s *GetIndexingJobService) validateRepositoryExists(ctx context.Context, repositoryID uuid.UUID) error {
	_, err := s.repositoryRepo.FindByID(ctx, repositoryID)
	if err != nil {
		return common.WrapServiceError(common.OpRetrieveRepository,
			fmt.Errorf("repository with ID %s not found: %w", repositoryID, err))
	}
	return nil
}

// retrieveIndexingJob fetches an indexing job by ID.
func (s *GetIndexingJobService) retrieveIndexingJob(ctx context.Context, jobID uuid.UUID) (*entity.IndexingJob, error) {
	job, err := s.indexingJobRepo.FindByID(ctx, jobID)
	if err != nil {
		return nil, common.WrapServiceError(common.OpRetrieveIndexingJob,
			fmt.Errorf("indexing job with ID %s not found: %w", jobID, err))
	}
	return job, nil
}

// validateJobOwnership ensures an indexing job belongs to the specified repository with detailed error context.
func (s *GetIndexingJobService) validateJobOwnership(job *entity.IndexingJob, repositoryID uuid.UUID) error {
	if job.RepositoryID() != repositoryID {
		return domainerrors.ErrJobNotFound
	}
	return nil
}

// Helper methods for UpdateIndexingJobService

// retrieveIndexingJobForUpdate retrieves an indexing job with error context for updates.
func (s *UpdateIndexingJobService) retrieveIndexingJobForUpdate(
	ctx context.Context,
	jobID uuid.UUID,
) (*entity.IndexingJob, error) {
	job, err := s.indexingJobRepo.FindByID(ctx, jobID)
	if err != nil {
		return nil, common.WrapServiceError(common.OpRetrieveIndexingJob,
			fmt.Errorf("indexing job with ID %s not found or cannot be updated: %w", jobID, err))
	}
	return job, nil
}

// parseAndValidateTargetStatus parses a status string and validates it's a valid job status.
func (s *UpdateIndexingJobService) parseAndValidateTargetStatus(statusStr string) (valueobject.JobStatus, error) {
	if statusStr == "" {
		return "", errors.New("job status is required and cannot be empty")
	}

	targetStatus, err := valueobject.NewJobStatus(statusStr)
	if err != nil {
		return "", fmt.Errorf("invalid job status '%s': %w", statusStr, err)
	}

	return targetStatus, nil
}

// validateStatusTransition ensures a status transition is valid according to business rules.
func (s *UpdateIndexingJobService) validateStatusTransition(
	job *entity.IndexingJob,
	targetStatus valueobject.JobStatus,
) error {
	currentStatus := job.Status()

	// Allow same status for progress updates
	if currentStatus == targetStatus {
		return nil
	}

	// Check if transition is allowed by business rules
	if !currentStatus.CanTransitionTo(targetStatus) {
		return fmt.Errorf("invalid status transition: cannot change job status from '%s' to '%s'",
			currentStatus.String(), targetStatus.String())
	}

	return nil
}

// Helper functions for conversion have been moved to common/converter.go
