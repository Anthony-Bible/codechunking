package service

import (
	"codechunking/internal/application/common"
	"codechunking/internal/application/defaults"
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"log/slog"
	"time"

	domainerrors "codechunking/internal/domain/errors/domain"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

const (
	// DuplicateDetectionCacheTTL is the time-to-live for duplicate detection cache entries.
	DuplicateDetectionCacheTTL = 5 // minutes
	// MaxDuplicateDetectionCacheSize is the maximum number of entries in the duplicate detection cache.
	MaxDuplicateDetectionCacheSize = 1000
)

// generateOperationID creates a unique identifier for batch operations.
func generateOperationID() string {
	return uuid.New().String()[:8]
}

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
func NewCreateRepositoryService(
	repositoryRepo outbound.RepositoryRepository,
	messagePublisher outbound.MessagePublisher,
) *CreateRepositoryService {
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

// NewCreateRepositoryServiceWithNormalizedDuplicateDetection creates a new instance with enhanced duplicate detection.
// This is an alias to NewCreateRepositoryService since all services now use normalized duplicate detection by default.
func NewCreateRepositoryServiceWithNormalizedDuplicateDetection(
	repositoryRepo outbound.RepositoryRepository,
	messagePublisher outbound.MessagePublisher,
) *CreateRepositoryService {
	return NewCreateRepositoryService(repositoryRepo, messagePublisher)
}

// CreateRepository creates a new repository and publishes an indexing job.
// It validates the repository URL, ensures it doesn't already exist, creates the entity,
// saves it to the repository, publishes an indexing job, and returns the response.
func (s *CreateRepositoryService) CreateRepository(
	ctx context.Context,
	request dto.CreateRepositoryRequest,
) (*dto.RepositoryResponse, error) {
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
		return nil, domainerrors.ErrRepositoryAlreadyExists
	}

	// Auto-generate name from URL if not provided
	name := request.Name
	if name == "" {
		name = common.ExtractRepositoryNameFromURL(request.URL)
	}

	// Create repository entity
	repository := entity.NewRepository(repositoryURL, name, request.Description, request.DefaultBranch)

	// Save repository
	if err = s.repositoryRepo.Save(ctx, repository); err != nil {
		return nil, common.WrapServiceError(common.OpSaveRepository, err)
	}

	// Publish indexing job
	if err = s.messagePublisher.PublishIndexingJob(ctx, repository.ID(), repository.URL().String()); err != nil {
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
func (s *UpdateRepositoryService) UpdateRepository(
	ctx context.Context,
	id uuid.UUID,
	request dto.UpdateRepositoryRequest,
) (*dto.RepositoryResponse, error) {
	// Find repository
	repository, err := s.repositoryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, common.WrapServiceError(common.OpRetrieveRepository, err)
	}

	// Check if repository can be updated (not processing)
	if repository.Status() == valueobject.RepositoryStatusProcessing {
		return nil, domainerrors.ErrRepositoryProcessing
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
	if err = s.repositoryRepo.Update(ctx, repository); err != nil {
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
		return domainerrors.ErrRepositoryProcessing
	}

	// Archive repository
	if err = repository.Archive(); err != nil {
		return common.WrapServiceError(common.OpDeleteRepository, err)
	}

	// Save updated repository
	if err = s.repositoryRepo.Update(ctx, repository); err != nil {
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
func (s *ListRepositoriesService) ListRepositories(
	ctx context.Context,
	query dto.RepositoryListQuery,
) (*dto.RepositoryListResponse, error) {
	// Apply defaults
	defaults.ApplyRepositoryListDefaults(&query)

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

// RepositoryFinderService handles repository finding operations with normalization support.
// It provides normalized URL-based lookup functionality for duplicate detection.
type RepositoryFinderService struct {
	repositoryRepo outbound.RepositoryRepository // Repository for data access
}

// NewRepositoryFinderServiceWithNormalization creates a new instance of RepositoryFinderService.
// It provides enhanced repository finding capabilities using normalized URL matching.
func NewRepositoryFinderServiceWithNormalization(
	repositoryRepo outbound.RepositoryRepository,
) *RepositoryFinderService {
	if repositoryRepo == nil {
		panic("repositoryRepo cannot be nil")
	}
	return &RepositoryFinderService{
		repositoryRepo: repositoryRepo,
	}
}

// FindByNormalizedURL finds a repository by its normalized URL.
// Returns the repository if found, nil if not found, or an error if there's a database issue.
func (s *RepositoryFinderService) FindByNormalizedURL(ctx context.Context, url string) (*entity.Repository, error) {
	// Validate and create repository URL
	repositoryURL, err := valueobject.NewRepositoryURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}

	// Find by normalized URL
	repository, err := s.repositoryRepo.FindByNormalizedURL(ctx, repositoryURL)
	if err != nil {
		return nil, common.WrapServiceError(common.OpRetrieveRepository, err)
	}

	return repository, nil
}

// DuplicateCheckResult represents the result of a duplicate check for a single URL.
type DuplicateCheckResult struct {
	URL                string
	IsDuplicate        bool
	ExistingRepository *entity.Repository
	Error              error
	NormalizedURL      string
	ProcessingTime     time.Duration
}

// DuplicateDetectionMetrics holds monitoring metrics for duplicate detection operations.
type DuplicateDetectionMetrics struct {
	TotalChecks       int64
	DuplicatesFound   int64
	TotalErrors       int64
	AvgProcessingTime time.Duration
	CacheHitRate      float64
}

// DuplicateCheckCacheEntry represents a cached duplicate check result.
type DuplicateCheckCacheEntry struct {
	IsDuplicate  bool
	RepositoryID *uuid.UUID
	CachedAt     time.Time
	TTL          time.Duration
}

// IsExpired checks if the cache entry has expired.
func (e *DuplicateCheckCacheEntry) IsExpired() bool {
	return time.Since(e.CachedAt) > e.TTL
}

// PerformantDuplicateDetectionService provides performance-optimized duplicate detection
// with batch processing, caching, and concurrent operations.
type PerformantDuplicateDetectionService struct {
	repositoryRepo outbound.RepositoryRepository
	logger         *slog.Logger
	metrics        *DuplicateDetectionMetrics
	duplicateCache map[string]*DuplicateCheckCacheEntry
	cacheTTL       time.Duration
	maxCacheSize   int
}

// NewPerformantDuplicateDetectionService creates a performance-optimized duplicate detection service.
func NewPerformantDuplicateDetectionService(
	repositoryRepo outbound.RepositoryRepository,
) *PerformantDuplicateDetectionService {
	if repositoryRepo == nil {
		panic("repositoryRepo cannot be nil")
	}
	return &PerformantDuplicateDetectionService{
		repositoryRepo: repositoryRepo,
		logger:         slog.Default().With("service", "duplicate_detection"),
		metrics:        &DuplicateDetectionMetrics{},
		duplicateCache: make(map[string]*DuplicateCheckCacheEntry),
		cacheTTL:       DuplicateDetectionCacheTTL * time.Minute, // Cache results for 5 minutes
		maxCacheSize:   MaxDuplicateDetectionCacheSize,           // Maximum cache entries
	}
}

// BatchCheckDuplicates performs efficient batch duplicate detection for multiple URLs.
// It uses the errgroup pattern for concurrent processing with proper context cancellation,
// bounded worker pool via semaphore, and early error propagation for critical failures.
//
// The implementation uses errgroup.WithContext to:
// - Enable proper context cancellation across all goroutines
// - Ensure early termination when any worker encounters a critical error
// - Coordinate cleanup and resource management across concurrent operations
//
// Concurrency is bounded using a semaphore pattern to prevent resource exhaustion
// while maintaining high throughput for batch operations.
func (s *PerformantDuplicateDetectionService) BatchCheckDuplicates(
	ctx context.Context,
	urls []string,
) ([]DuplicateCheckResult, error) {
	if len(urls) == 0 {
		s.logger.InfoContext(ctx, "Batch duplicate check called with empty URL list")
		return []DuplicateCheckResult{}, nil
	}

	startTime := time.Now()
	operationID := generateOperationID()
	s.logger.InfoContext(ctx, "Starting batch duplicate detection",
		"url_count", len(urls),
		"operation_id", operationID)

	results := make([]DuplicateCheckResult, len(urls))

	// Use errgroup with context for proper cancellation and error handling
	g, groupCtx := errgroup.WithContext(ctx)

	// Create bounded worker pool using semaphore pattern
	const maxConcurrentWorkers = 10
	workerSemaphore := make(chan struct{}, maxConcurrentWorkers)

	// Launch workers for each URL
	for i, url := range urls {
		// Capture loop variables for closure
		urlIndex, rawURL := i, url
		g.Go(func() error {
			return s.processSingleURLForDuplicateCheck(groupCtx, rawURL, urlIndex, workerSemaphore, results)
		})
	}

	// Wait for all goroutines to complete or first error
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Collect results and compute metrics
	var totalProcessingTime time.Duration
	duplicatesFound := int64(0)
	errorsFound := int64(0)

	for _, result := range results {
		// Update metrics
		totalProcessingTime += result.ProcessingTime
		if result.IsDuplicate {
			duplicatesFound++
		}
		if result.Error != nil {
			errorsFound++
		}
	}

	// Update service metrics
	s.metrics.TotalChecks += int64(len(urls))
	s.metrics.DuplicatesFound += duplicatesFound
	s.metrics.TotalErrors += errorsFound
	s.metrics.AvgProcessingTime = totalProcessingTime / time.Duration(len(urls))

	// Log final batch operation results
	operationTime := time.Since(startTime)
	s.logger.InfoContext(ctx, "Batch duplicate detection completed",
		"url_count", len(urls),
		"duplicates_found", duplicatesFound,
		"errors", errorsFound,
		"total_operation_time", operationTime,
		"avg_per_url", s.metrics.AvgProcessingTime,
		"throughput_per_second", float64(len(urls))/operationTime.Seconds())

	return results, nil
}

// processSingleURLForDuplicateCheck handles the duplicate detection logic for a single URL
// within the errgroup worker pool. It manages semaphore acquisition/release and proper
// context cancellation while performing URL validation, normalization, and database lookups.
//
// The function uses the errgroup context for all operations to ensure proper cancellation
// propagation and early termination when the context is canceled or other workers encounter errors.
func (s *PerformantDuplicateDetectionService) processSingleURLForDuplicateCheck(
	ctx context.Context,
	rawURL string,
	resultIndex int,
	semaphore chan struct{},
	results []DuplicateCheckResult,
) error {
	urlStartTime := time.Now()

	// Acquire semaphore with context cancellation support
	select {
	case semaphore <- struct{}{}:
		defer func() { <-semaphore }() // Release semaphore on function exit
	case <-ctx.Done():
		return ctx.Err()
	}

	result := DuplicateCheckResult{
		URL: rawURL,
	}

	// Validate and normalize the repository URL
	repositoryURL, err := valueobject.NewRepositoryURL(rawURL)
	if err != nil {
		result.Error = fmt.Errorf("invalid URL: %w", err)
		result.ProcessingTime = time.Since(urlStartTime)
		s.logger.WarnContext(ctx, "URL validation failed",
			"url", rawURL,
			"error", err.Error(),
			"processing_time", result.ProcessingTime)
		results[resultIndex] = result
		return nil // URL validation errors don't halt batch processing
	}

	result.NormalizedURL = repositoryURL.String()

	// Check for existing repository using efficient existence query
	// Use errgroup context for proper cancellation behavior
	exists, err := s.repositoryRepo.ExistsByNormalizedURL(ctx, repositoryURL)
	if err != nil {
		result.Error = fmt.Errorf("failed to check duplicate: %w", err)
		result.ProcessingTime = time.Since(urlStartTime)
		s.logger.ErrorContext(ctx, "Duplicate check failed",
			"url", rawURL,
			"normalized_url", result.NormalizedURL,
			"error", err.Error(),
			"processing_time", result.ProcessingTime)
		// Check if error is due to context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Database errors are critical and should halt batch processing
		return fmt.Errorf("failed to check duplicate: %w", err)
	}

	result.IsDuplicate = exists

	// Fetch full repository details if duplicate exists
	if exists {
		existingRepo, findErr := s.repositoryRepo.FindByNormalizedURL(ctx, repositoryURL)
		if findErr != nil {
			result.Error = fmt.Errorf("failed to fetch existing repository: %w", findErr)
			result.ProcessingTime = time.Since(urlStartTime)
			s.logger.ErrorContext(ctx, "Failed to fetch existing repository details",
				"url", rawURL,
				"normalized_url", result.NormalizedURL,
				"error", findErr.Error(),
				"processing_time", result.ProcessingTime)
			// Check if error is due to context cancellation
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Database errors are critical and should halt batch processing
			return fmt.Errorf("failed to fetch existing repository: %w", err)
		}

		result.ExistingRepository = existingRepo
		s.logger.InfoContext(ctx, "Duplicate repository detected",
			"url", rawURL,
			"normalized_url", result.NormalizedURL,
			"existing_id", existingRepo.ID().String(),
			"processing_time", time.Since(urlStartTime))
	} else {
		s.logger.DebugContext(ctx, "No duplicate found",
			"url", rawURL,
			"normalized_url", result.NormalizedURL,
			"processing_time", time.Since(urlStartTime))
	}

	result.ProcessingTime = time.Since(urlStartTime)
	results[resultIndex] = result
	return nil
}

// GetMetrics returns the current duplicate detection metrics for monitoring.
func (s *PerformantDuplicateDetectionService) GetMetrics() DuplicateDetectionMetrics {
	return *s.metrics
}

// ResetMetrics resets all metrics counters (useful for periodic reporting).
func (s *PerformantDuplicateDetectionService) ResetMetrics() {
	s.metrics = &DuplicateDetectionMetrics{}
	s.logger.Info("Duplicate detection metrics reset")
}

// Helper functions have been moved to common/converter.go for better reusability
