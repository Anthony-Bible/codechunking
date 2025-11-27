// Package worker provides job processing capabilities for the code chunking system.
//
// # Idempotent Job Processing
//
// The job processor implements idempotent message handling to safely handle NATS message
// redelivery scenarios. This is critical for reliability in distributed systems where
// messages may be redelivered due to:
//   - Worker crashes or restarts
//   - Network timeouts or connection issues
//   - NATS server redelivery policies
//   - Manual reprocessing of failed jobs
//
// # Idempotency Strategy
//
// The processor checks repository status before processing to determine the appropriate action:
//
//   - Completed: Skip reprocessing (idempotent - work already done)
//   - Archived: Skip processing (repository no longer active)
//   - Failed: Reset to pending and retry (allows recovery from failures)
//   - Cloning/Processing: Resume from current state (interrupted job recovery)
//   - Pending: Continue with normal processing flow
//
// # State Transitions
//
// Normal flow: pending → cloning → processing → completed
// Retry flow: failed → pending → cloning → processing → completed
// Interrupted flow: Resume from cloning or processing state
//
// # Thread Safety
//
// The processor uses mutexes and atomic operations to safely handle concurrent job execution.
// The semaphore ensures that the maximum number of concurrent jobs is not exceeded.
package worker

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/config"
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/messaging"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/inbound"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const (
	jobStatusFailed    = "failed"
	jobStatusRunning   = "running"
	jobStatusCompleted = "completed"
	unknownCommitHash  = "unknown"
)

// JobProcessorConfig holds configuration for the job processor.
type JobProcessorConfig struct {
	WorkspaceDir      string
	MaxConcurrentJobs int
	JobTimeout        time.Duration
	MaxMemoryMB       int
	MaxDiskUsageMB    int64
	CleanupInterval   time.Duration
	RetryAttempts     int
	RetryBackoff      time.Duration
}

// JobProcessorBatchOptions contains optional batch processing dependencies.
// All fields are optional and nil values indicate the dependency is not available.
type JobProcessorBatchOptions struct {
	BatchConfig           *config.BatchProcessingConfig
	BatchQueueManager     outbound.BatchQueueManager
	BatchProgressRepo     outbound.BatchProgressRepository
	BatchEmbeddingService outbound.BatchEmbeddingService
}

// JobExecution tracks the execution of a single job.
type JobExecution struct {
	JobID        uuid.UUID
	RepositoryID uuid.UUID
	StartTime    time.Time
	Status       string
	Progress     *JobProgress
	mu           sync.RWMutex
}

// JobProgress tracks job processing progress.
type JobProgress struct {
	FilesProcessed    int64
	ChunksGenerated   int64
	EmbeddingsCreated int64
	BytesProcessed    int64
	CurrentFile       string
	Stage             string
}

// DefaultJobProcessor is the default implementation.
type DefaultJobProcessor struct {
	config                JobProcessorConfig
	batchConfig           config.BatchProcessingConfig
	indexingJobRepo       outbound.IndexingJobRepository
	repositoryRepo        outbound.RepositoryRepository
	gitClient             outbound.GitClient
	codeParser            outbound.CodeParser
	embeddingService      outbound.EmbeddingService
	batchEmbeddingService outbound.BatchEmbeddingService
	chunkStorageRepo      outbound.ChunkStorageRepository
	batchQueueManager     outbound.BatchQueueManager
	batchProgressRepo     outbound.BatchProgressRepository
	activeJobs            map[string]*JobExecution
	jobsMu                sync.RWMutex
	semaphore             chan struct{}
	metrics               inbound.JobProcessorMetrics
	healthStatus          inbound.JobProcessorHealthStatus
}

// NewDefaultJobProcessor creates a new default job processor.
// The batchOptions parameter is optional and can be nil for minimal configuration.
func NewDefaultJobProcessor(
	processorConfig JobProcessorConfig,
	indexingJobRepo outbound.IndexingJobRepository,
	repositoryRepo outbound.RepositoryRepository,
	gitClient outbound.GitClient,
	codeParser outbound.CodeParser,
	embeddingService outbound.EmbeddingService,
	chunkStorageRepo outbound.ChunkStorageRepository,
	batchOptions *JobProcessorBatchOptions,
) inbound.JobProcessor {
	// Handle nil batch options
	if batchOptions == nil {
		batchOptions = &JobProcessorBatchOptions{}
	}

	// Use defaults for nil config
	batchConfig := config.BatchProcessingConfig{
		Enabled:              false,
		ThresholdChunks:      10,
		FallbackToSequential: true,
	}
	if batchOptions.BatchConfig != nil {
		batchConfig = *batchOptions.BatchConfig
	}

	// Ensure proper semaphore initialization
	maxConcurrent := processorConfig.MaxConcurrentJobs
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}

	processor := &DefaultJobProcessor{
		config:                processorConfig,
		batchConfig:           batchConfig,
		indexingJobRepo:       indexingJobRepo,
		repositoryRepo:        repositoryRepo,
		gitClient:             gitClient,
		codeParser:            codeParser,
		embeddingService:      embeddingService,
		batchEmbeddingService: batchOptions.BatchEmbeddingService,
		chunkStorageRepo:      chunkStorageRepo,
		batchQueueManager:     batchOptions.BatchQueueManager,
		batchProgressRepo:     batchOptions.BatchProgressRepo,
		activeJobs:            make(map[string]*JobExecution),
		semaphore:             make(chan struct{}, maxConcurrent),
	}

	return processor
}

// ProcessJob processes an indexing job message.
func (p *DefaultJobProcessor) ProcessJob(ctx context.Context, message messaging.EnhancedIndexingJobMessage) error {
	if err := p.validateAndPrepareJob(&message); err != nil {
		return err
	}

	// Acquire semaphore for concurrency control
	p.semaphore <- struct{}{}
	defer func() {
		<-p.semaphore
	}()

	// Create context with timeout
	jobCtx, cancel := context.WithTimeout(ctx, p.config.JobTimeout)
	defer cancel()

	execution := p.createJobExecution(message)
	workspacePath := filepath.Join(p.config.WorkspaceDir, message.MessageID)
	defer p.cleanupWorkspace(workspacePath, message.MessageID)

	chunks, err := p.executeJobPipeline(jobCtx, message, workspacePath, execution)
	if err != nil {
		return err
	}

	// If chunks is nil, job was skipped (completed/archived state)
	if chunks != nil {
		p.finalizeJobCompletion(jobCtx, message, execution, chunks)
	}

	return nil
}

// validateAndPrepareJob validates the message and checks system resources.
func (p *DefaultJobProcessor) validateAndPrepareJob(message *messaging.EnhancedIndexingJobMessage) error {
	if err := p.validateMessage(*message); err != nil {
		return err
	}

	jobID := uuid.New()
	if message.MessageID == "" {
		message.MessageID = jobID.String()
	}

	if p.isMemoryPressureHigh() {
		return errors.New("memory pressure too high, rejecting job")
	}

	if p.config.MaxMemoryMB > 0 || p.config.MaxDiskUsageMB > 0 {
		return errors.New("resource limits enforcement not implemented yet")
	}

	return nil
}

// createJobExecution creates and registers a job execution.
func (p *DefaultJobProcessor) createJobExecution(message messaging.EnhancedIndexingJobMessage) *JobExecution {
	execution := &JobExecution{
		JobID:        uuid.New(),
		RepositoryID: message.RepositoryID,
		StartTime:    time.Now(),
		Status:       jobStatusRunning,
		Progress:     &JobProgress{},
	}

	p.jobsMu.Lock()
	p.activeJobs[message.MessageID] = execution
	p.healthStatus.ActiveJobs = len(p.activeJobs)
	p.jobsMu.Unlock()

	return execution
}

// executeJobPipeline executes the main job processing pipeline.
// It implements idempotent processing by checking repository status before proceeding.
// Returns nil chunks if the repository should be skipped (completed/archived).
func (p *DefaultJobProcessor) executeJobPipeline(
	ctx context.Context,
	message messaging.EnhancedIndexingJobMessage,
	workspacePath string,
	execution *JobExecution,
) ([]outbound.CodeChunk, error) {
	// Fetch repository to check current status for idempotent processing
	repo, err := p.repositoryRepo.FindByID(ctx, message.RepositoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repository for status check: %w", err)
	}
	if repo == nil {
		return nil, fmt.Errorf("repository not found: %s", message.RepositoryID.String())
	}

	// Handle repository status - this enables idempotent message redelivery
	shouldSkip, err := p.handleRepositoryStatus(ctx, repo, message.RepositoryID)
	if err != nil {
		return nil, err
	}
	if shouldSkip {
		return nil, nil
	}

	if err := p.setupWorkspace(workspacePath); err != nil {
		p.updateJobStatus(message.MessageID, jobStatusFailed)
		p.markRepositoryFailed(ctx, message.RepositoryID)
		return nil, err
	}

	// Transition to cloning if not already in cloning or processing state
	if p.shouldTransitionFrom(repo.Status(), valueobject.RepositoryStatusCloning) {
		if err := p.transitionToCloning(ctx, message.RepositoryID, message.MessageID); err != nil {
			return nil, err
		}
	}

	if err := p.cloneRepository(ctx, message, workspacePath); err != nil {
		p.updateJobStatus(message.MessageID, jobStatusFailed)
		p.markRepositoryFailed(ctx, message.RepositoryID)
		return nil, err
	}

	// Transition to processing if not already in processing state
	if p.shouldTransitionFrom(repo.Status(), valueobject.RepositoryStatusProcessing) {
		if err := p.transitionToProcessing(ctx, message.RepositoryID, message.MessageID); err != nil {
			return nil, err
		}
	}

	chunkSize := p.getChunkSize(message)
	config := outbound.CodeParsingConfig{ChunkSizeBytes: chunkSize}

	chunks, err := p.parseCode(ctx, workspacePath, config)
	if err != nil {
		p.updateJobStatus(message.MessageID, jobStatusFailed)
		p.markRepositoryFailed(ctx, message.RepositoryID)
		return nil, err
	}

	if err := p.generateEmbeddings(ctx, message.IndexingJobID, message.RepositoryID, chunks); err != nil {
		p.updateJobStatus(message.MessageID, jobStatusFailed)
		p.markRepositoryFailed(ctx, message.RepositoryID)
		return nil, err
	}

	return chunks, nil
}

// shouldUseBatchProcessing determines if batch processing should be used for the given chunks.
// This method implements the smart routing logic that considers configuration thresholds,
// queue manager availability, and chunk count to make the decision.
//
//nolint:unused // Reserved for future batch processing optimization
func (p *DefaultJobProcessor) shouldUseBatchProcessing(chunks []outbound.CodeChunk) bool {
	// Check if we would want to use batch processing based on configuration and chunk count
	meetsThreshold := len(chunks) > p.batchConfig.ThresholdChunks
	batchEnabled := p.batchConfig.Enabled

	// Check if batch processing is desired but manager is not available
	// This allows the calling code to detect the nil manager condition
	batchDesired := meetsThreshold && batchEnabled
	return batchDesired && p.batchQueueManager != nil
}

// shouldTransitionFrom determines if a repository should transition to a target status.
// It returns true if the current status is different from the target status AND
// the repository is not already in a more advanced state.
// For example:
// - shouldTransitionFrom(pending, cloning) = true (should transition)
// - shouldTransitionFrom(cloning, cloning) = false (already there)
// - shouldTransitionFrom(processing, cloning) = false (already past cloning).
func (p *DefaultJobProcessor) shouldTransitionFrom(
	currentStatus valueobject.RepositoryStatus,
	targetStatus valueobject.RepositoryStatus,
) bool {
	// Don't transition if already at target status
	if currentStatus == targetStatus {
		return false
	}

	// Special case: don't transition to cloning if already processing
	// (processing is further along than cloning)
	if targetStatus == valueobject.RepositoryStatusCloning &&
		currentStatus == valueobject.RepositoryStatusProcessing {
		return false
	}

	return true
}

// handleRepositoryStatus checks the repository status and determines if processing should continue.
// It implements idempotent behavior for NATS message redelivery:
// - Completed/Archived: Skip reprocessing (idempotent)
// - Failed: Reset to pending for retry
// - Cloning/Processing: Resume from current state (interrupted job)
// - Pending: Continue with normal processing
// Returns (shouldSkip=true, nil) if processing should be skipped.
// Returns (shouldSkip=false, nil) if processing should continue.
// Returns (_, error) if an error occurred during status handling.
func (p *DefaultJobProcessor) handleRepositoryStatus(
	ctx context.Context,
	repo *entity.Repository,
	repositoryID uuid.UUID,
) (bool, error) {
	currentStatus := repo.Status()

	switch currentStatus {
	case valueobject.RepositoryStatusCompleted:
		// Idempotent: repository already successfully indexed, skip reprocessing
		slogger.Info(ctx, "Repository already completed, skipping reprocessing", slogger.Fields{
			"repository_id": repositoryID.String(),
			"status":        currentStatus.String(),
		})
		return true, nil

	case valueobject.RepositoryStatusArchived:
		// Archived: skip processing for archived repositories
		slogger.Info(ctx, "Repository is archived, skipping processing", slogger.Fields{
			"repository_id": repositoryID.String(),
			"status":        currentStatus.String(),
		})
		return true, nil

	case valueobject.RepositoryStatusFailed:
		// Retry scenario: reset to pending before retrying
		if err := p.updateRepositoryStatus(ctx, repositoryID, "pending"); err != nil {
			return false, fmt.Errorf(
				"failed to reset failed repository to pending for retry (status=%s): %w",
				currentStatus.String(),
				err,
			)
		}
		slogger.Info(ctx, "Reset failed repository to pending for retry", slogger.Fields{
			"repository_id":   repositoryID.String(),
			"previous_status": currentStatus.String(),
			"new_status":      "pending",
		})
		return false, nil

	case valueobject.RepositoryStatusCloning, valueobject.RepositoryStatusProcessing:
		// Interrupted job scenario: resume from current state
		slogger.Info(ctx, "Resuming interrupted job from current state", slogger.Fields{
			"repository_id": repositoryID.String(),
			"status":        currentStatus.String(),
		})
		return false, nil

	case valueobject.RepositoryStatusPending:
		// Normal flow: continue with processing
		slogger.Debug(ctx, "Repository in pending state, starting normal processing", slogger.Fields{
			"repository_id": repositoryID.String(),
			"status":        currentStatus.String(),
		})
		return false, nil

	default:
		// Unknown status: log warning but allow processing to continue
		slogger.Warn(ctx, "Repository has unknown status, attempting to process", slogger.Fields{
			"repository_id": repositoryID.String(),
			"status":        currentStatus.String(),
		})
		return false, nil
	}
}

// handleTransitionFailure handles common error handling when a status transition fails.
// It logs the error, updates the job status to failed, and marks the repository as failed
// (only if it's not already in failed state to avoid redundant operations).
func (p *DefaultJobProcessor) handleTransitionFailure(
	ctx context.Context,
	repoID uuid.UUID,
	jobID string,
	targetStatus string,
	err error,
) error {
	slogger.Error(ctx, "Failed to update repository status", slogger.Fields{
		"repository_id": repoID.String(),
		"target_status": targetStatus,
		"error":         err.Error(),
	})

	p.updateJobStatus(jobID, jobStatusFailed)

	// Avoid redundant database fetch if repository is already failed
	repo, fetchErr := p.repositoryRepo.FindByID(ctx, repoID)
	if fetchErr == nil && repo != nil && repo.Status() != valueobject.RepositoryStatusFailed {
		p.markRepositoryFailed(ctx, repoID)
	}

	return fmt.Errorf("failed to update repository status to %s: %w", targetStatus, err)
}

// transitionToCloning updates repository status to cloning.
func (p *DefaultJobProcessor) transitionToCloning(ctx context.Context, repoID uuid.UUID, jobID string) error {
	if err := p.updateRepositoryStatus(ctx, repoID, "cloning"); err != nil {
		return p.handleTransitionFailure(ctx, repoID, jobID, "cloning", err)
	}
	return nil
}

// transitionToProcessing updates repository status to processing.
func (p *DefaultJobProcessor) transitionToProcessing(ctx context.Context, repoID uuid.UUID, jobID string) error {
	if err := p.updateRepositoryStatus(ctx, repoID, "processing"); err != nil {
		return p.handleTransitionFailure(ctx, repoID, jobID, "processing", err)
	}
	return nil
}

// finalizeJobCompletion marks the job and repository as completed.
func (p *DefaultJobProcessor) finalizeJobCompletion(
	ctx context.Context,
	message messaging.EnhancedIndexingJobMessage,
	execution *JobExecution,
	chunks []outbound.CodeChunk,
) {
	p.updateJobStatus(message.MessageID, jobStatusCompleted)
	if execution.Progress != nil {
		execution.Progress.Stage = jobStatusCompleted
	}

	fileCount := len(chunks)
	p.markRepositoryCompleted(ctx, message.RepositoryID, unknownCommitHash, fileCount, len(chunks))
	p.updateMetrics(int64(len(chunks)))
}

// GetHealthStatus returns the current health status.
func (p *DefaultJobProcessor) GetHealthStatus() inbound.JobProcessorHealthStatus {
	p.jobsMu.RLock()
	defer p.jobsMu.RUnlock()

	status := p.healthStatus
	status.ActiveJobs = len(p.activeJobs)
	return status
}

// GetMetrics returns job processing metrics.
func (p *DefaultJobProcessor) GetMetrics() inbound.JobProcessorMetrics {
	return p.metrics
}

// Cleanup performs cleanup operations.
func (p *DefaultJobProcessor) Cleanup() error {
	p.jobsMu.Lock()
	defer p.jobsMu.Unlock()

	p.activeJobs = make(map[string]*JobExecution)

	// Clean up workspace directory
	if err := os.RemoveAll(p.config.WorkspaceDir); err != nil {
		return fmt.Errorf("failed to remove workspace directory: %w", err)
	}

	// Recreate workspace directory
	if err := os.MkdirAll(p.config.WorkspaceDir, 0o750); err != nil {
		return fmt.Errorf("failed to recreate workspace directory: %w", err)
	}

	return nil
}

// validateMessage validates the job message.
func (p *DefaultJobProcessor) validateMessage(message messaging.EnhancedIndexingJobMessage) error {
	if message.RepositoryID == uuid.Nil {
		return errors.New("repository ID is required")
	}
	if message.RepositoryURL == "" {
		return errors.New("repository URL is required")
	}
	return nil
}

// setupWorkspace creates the workspace directory.
func (p *DefaultJobProcessor) setupWorkspace(workspacePath string) error {
	if err := os.MkdirAll(workspacePath, 0o750); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}
	return nil
}

// cloneRepository clones the repository.
func (p *DefaultJobProcessor) cloneRepository(
	ctx context.Context,
	message messaging.EnhancedIndexingJobMessage,
	workspacePath string,
) error {
	if err := p.gitClient.Clone(ctx, message.RepositoryURL, workspacePath); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}
	return nil
}

// getChunkSize determines the chunk size from message or uses default.
func (p *DefaultJobProcessor) getChunkSize(message messaging.EnhancedIndexingJobMessage) int {
	chunkSize := 1024
	if message.ProcessingMetadata.ChunkSizeBytes > 0 {
		chunkSize = message.ProcessingMetadata.ChunkSizeBytes
	}
	return chunkSize
}

// parseCode parses the code directory.
func (p *DefaultJobProcessor) parseCode(
	ctx context.Context,
	workspacePath string,
	config outbound.CodeParsingConfig,
) ([]outbound.CodeChunk, error) {
	repositoryRoot, err := p.findRepositoryRoot(workspacePath)
	if err != nil {
		slogger.Warn(ctx, "Failed to find repository root, using workspace path", slogger.Fields{
			"workspace_path": workspacePath,
			"error":          err.Error(),
		})
		repositoryRoot = workspacePath
	}

	chunks, err := p.codeParser.ParseDirectory(ctx, repositoryRoot, config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse directory: %w", err)
	}
	return chunks, nil
}

// findRepositoryRoot looks for the actual repository root within the workspace.
func (p *DefaultJobProcessor) findRepositoryRoot(workspacePath string) (string, error) {
	return findGitRepositoryRoot(workspacePath)
}

// isQuotaError checks if the error is related to Google Gemini API quota exhaustion.
func isQuotaError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Check for common quota error patterns
	quotaPatterns := []string{
		"quota exceeded",
		"quota_exceeded",
		"resource exhausted",
		"billing required",
		"you exceeded your current quota",
		"insufficient quota",
		"429",
		"resource_exhausted",
	}

	for _, pattern := range quotaPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// findGitRepositoryRoot searches for a .git directory in the workspace.
func findGitRepositoryRoot(workspacePath string) (string, error) {
	entries, err := os.ReadDir(workspacePath)
	if err != nil {
		return "", fmt.Errorf("failed to read workspace directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			gitPath := filepath.Join(workspacePath, entry.Name(), ".git")
			if _, err := os.Stat(gitPath); err == nil {
				return filepath.Join(workspacePath, entry.Name()), nil
			}
		}
	}

	return workspacePath, nil
}

// generateEmbeddings creates embeddings for code chunks.
func (p *DefaultJobProcessor) generateEmbeddings(
	ctx context.Context,
	indexingJobID uuid.UUID,
	repositoryID uuid.UUID,
	chunks []outbound.CodeChunk,
) error {
	// Convert UUID to string for backward compatibility with existing code
	jobID := indexingJobID.String()
	execution := p.getJobExecution(jobID)
	if execution == nil {
		// Create a dummy execution for progress tracking in direct test calls
		execution = &JobExecution{
			JobID:        uuid.New(),
			RepositoryID: repositoryID,
			StartTime:    time.Now(),
			Status:       jobStatusRunning,
			Progress:     &JobProgress{},
		}
	}

	p.initializeEmbeddingProgress(execution, len(chunks))

	slogger.Info(ctx, "Starting embedding generation", slogger.Fields{
		"job_id":            jobID,
		"chunk_count":       len(chunks),
		"has_queue_manager": p.batchQueueManager != nil,
		"batch_enabled":     p.batchConfig.Enabled,
		"threshold_chunks":  p.batchConfig.ThresholdChunks,
	})

	// Use smart routing to choose between batch and sequential processing
	return p.generateEmbeddingsWithBatch(ctx, indexingJobID, repositoryID, chunks)
}

func (p *DefaultJobProcessor) getJobExecution(jobID string) *JobExecution {
	p.jobsMu.RLock()
	defer p.jobsMu.RUnlock()
	return p.activeJobs[jobID]
}

func (p *DefaultJobProcessor) initializeEmbeddingProgress(execution *JobExecution, chunkCount int) {
	if execution.Progress == nil {
		execution.Progress = &JobProgress{}
	}
	atomic.AddInt64(&execution.Progress.ChunksGenerated, int64(chunkCount))
	execution.Progress.Stage = "parsing_complete"
}

//nolint:unused // Reserved for future use in parallel chunk processing
func (p *DefaultJobProcessor) generateSingleEmbedding(
	ctx context.Context,
	jobID string,
	chunk outbound.CodeChunk,
	chunkIndex int,
) (*outbound.EmbeddingResult, error) {
	slogger.Debug(ctx, "Starting embedding generation for chunk", slogger.Fields{
		"job_id":     jobID,
		"chunk_id":   chunk.ID,
		"chunk_num":  chunkIndex + 1,
		"file_path":  chunk.FilePath,
		"chunk_size": len(chunk.Content),
	})

	options := outbound.EmbeddingOptions{
		Model:    "gemini-embedding-001",
		TaskType: outbound.TaskTypeRetrievalDocument,
		Timeout:  120 * time.Second,
	}

	result, err := p.embeddingService.GenerateEmbedding(ctx, chunk.Content, options)
	if err != nil {
		slogger.Error(ctx, "Failed to generate embedding", slogger.Fields{
			"job_id":    jobID,
			"chunk_id":  chunk.ID,
			"chunk_num": chunkIndex + 1,
			"file_path": chunk.FilePath,
			"error":     err.Error(),
		})
		return nil, fmt.Errorf("failed to generate embedding for chunk %d (%s): %w", chunkIndex, chunk.ID, err)
	}

	slogger.Debug(ctx, "Embedding generated successfully", slogger.Fields{
		"job_id":     jobID,
		"chunk_id":   chunk.ID,
		"dimensions": result.Dimensions,
	})

	return result, nil
}

func (p *DefaultJobProcessor) storeSingleEmbedding(
	ctx context.Context,
	jobID string,
	repositoryID uuid.UUID,
	chunk outbound.CodeChunk,
	result *outbound.EmbeddingResult,
	chunkIndex int,
) error {
	var chunkUUID uuid.UUID
	var err error

	// Handle both string IDs and UUID formats for TDD compatibility
	if len(chunk.ID) == 36 {
		// UUID format
		chunkUUID, err = uuid.Parse(chunk.ID)
		if err != nil {
			slogger.Error(ctx, "Invalid chunk UUID format", slogger.Fields{
				"job_id":   jobID,
				"chunk_id": chunk.ID,
				"error":    err.Error(),
			})
			return fmt.Errorf("invalid chunk UUID format for chunk %d: %w", chunkIndex, err)
		}
	} else {
		// String ID format - generate a UUID for storage (TDD compatibility)
		chunkUUID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(chunk.ID))
	}

	// Ensure chunk carries repository ID for persistence
	chunk.RepositoryID = repositoryID

	embedding := &outbound.Embedding{
		ID:           uuid.New(),
		ChunkID:      chunkUUID,
		RepositoryID: repositoryID,
		Vector:       result.Vector,
		ModelVersion: "gemini-embedding-001",
		CreatedAt:    time.Now().Format(time.RFC3339),
	}

	slogger.Debug(ctx, "Saving chunk and embedding to database", slogger.Fields{
		"job_id":        jobID,
		"chunk_id":      chunk.ID,
		"embedding_id":  embedding.ID.String(),
		"vector_length": len(result.Vector),
	})

	dbCtx, dbCancel := context.WithTimeout(ctx, 30*time.Second)
	defer dbCancel()

	if err := p.chunkStorageRepo.SaveChunkWithEmbedding(dbCtx, &chunk, embedding); err != nil {
		slogger.Error(ctx, "Failed to store chunk and embedding", slogger.Fields{
			"job_id":       jobID,
			"chunk_id":     chunk.ID,
			"embedding_id": embedding.ID.String(),
			"error":        err.Error(),
		})
		return fmt.Errorf("failed to store chunk and embedding for chunk %d (%s): %w", chunkIndex, chunk.ID, err)
	}

	slogger.Debug(ctx, "Chunk and embedding saved successfully", slogger.Fields{
		"job_id":       jobID,
		"chunk_id":     chunk.ID,
		"embedding_id": embedding.ID.String(),
	})

	return nil
}

// validateBatchProcessingConfig validates the batch processing configuration.
// This maintains backward compatibility with existing test configs that may have zero values.
func (p *DefaultJobProcessor) validateBatchProcessingConfig() error {
	// Validate threshold chunks
	if err := p.validateThresholdChunks(); err != nil {
		return err
	}

	// Validate queue limits if configured
	if err := p.validateQueueLimits(); err != nil {
		return err
	}

	// Validate batch sizes if configured
	if err := p.validateBatchSizes(); err != nil {
		return err
	}

	return nil
}

// validateThresholdChunks validates the threshold chunks configuration.
func (p *DefaultJobProcessor) validateThresholdChunks() error {
	if p.batchConfig.ThresholdChunks < 0 {
		return fmt.Errorf("negative batch threshold is invalid: %d", p.batchConfig.ThresholdChunks)
	}

	// Zero threshold is only invalid if BatchSizes is configured (enhanced config)
	if p.batchConfig.ThresholdChunks == 0 && len(p.batchConfig.BatchSizes) > 0 {
		return errors.New("zero batch threshold is invalid")
	}

	return nil
}

// validateQueueLimits validates queue limit configuration.
func (p *DefaultJobProcessor) validateQueueLimits() error {
	limits := p.batchConfig.QueueLimits

	// Check for negative values
	if limits.MaxQueueSize < 0 {
		return errors.New("queue limits max_queue_size cannot be negative")
	}
	if limits.MaxWaitTime < 0 {
		return errors.New("queue limits max_wait_time cannot be negative")
	}

	// If both are zero, skip validation (test config or disabled)
	if limits.MaxQueueSize == 0 && limits.MaxWaitTime == 0 {
		return nil
	}

	// If only one is set, the other must be positive too
	if limits.MaxQueueSize == 0 {
		return errors.New("queue limits max_queue_size must be positive when wait time is configured")
	}
	if limits.MaxWaitTime == 0 {
		return errors.New("queue limits max_wait_time must be positive when max queue size is configured")
	}

	return nil
}

// validateBatchSizes validates batch size configuration for all priorities.
func (p *DefaultJobProcessor) validateBatchSizes() error {
	for priority, sizeConfig := range p.batchConfig.BatchSizes {
		if sizeConfig.Min <= 0 {
			return fmt.Errorf("batch size min must be positive for priority %s", priority)
		}
		if sizeConfig.Max <= 0 {
			return fmt.Errorf("batch size max must be positive for priority %s", priority)
		}
		if sizeConfig.Min > sizeConfig.Max {
			return fmt.Errorf("batch size min cannot be greater than max for priority %s", priority)
		}
		if sizeConfig.Timeout <= 0 {
			return fmt.Errorf("batch size timeout must be positive for priority %s", priority)
		}
	}

	return nil
}

// validateChunks validates chunk data before processing.
func (p *DefaultJobProcessor) validateChunks(ctx context.Context, chunks []outbound.CodeChunk) error {
	if len(chunks) == 0 {
		return errors.New("no chunks provided for processing")
	}

	for i, chunk := range chunks {
		// Check context periodically during validation of large chunks
		if i%100 == 0 && ctx.Err() != nil {
			return fmt.Errorf("context cancelled during chunk validation: %w", ctx.Err())
		}

		// Validate chunk content is not empty
		if strings.TrimSpace(chunk.Content) == "" {
			return fmt.Errorf("chunk %d has empty content [file=%s, lines=%d-%d, type=%s, entity=%s]",
				i, chunk.FilePath, chunk.StartLine, chunk.EndLine, chunk.Type, chunk.EntityName)
		}

		// Validate chunk ID is not empty
		if strings.TrimSpace(chunk.ID) == "" {
			return fmt.Errorf("chunk %d has empty ID", i)
		}

		// Validate chunk file path is not empty
		if strings.TrimSpace(chunk.FilePath) == "" {
			return fmt.Errorf("chunk %d has empty file path", i)
		}

		// Validate chunk size is reasonable (memory exhaustion check)
		if len(chunk.Content) > 10*1024*1024 { // 10MB limit per chunk
			return fmt.Errorf("chunk %d content too large: %d bytes", i, len(chunk.Content))
		}
	}

	return nil
}

// shouldFallbackOnError determines if fallback should be attempted based on error type.
func (p *DefaultJobProcessor) shouldFallbackOnError(err error) bool {
	errMsg := err.Error()

	// Fall back for network, queue, and temporary errors
	fallbackErrors := []string{
		"queue is full",
		"connection refused",
		"network",
		"timeout",
		"context deadline exceeded",
		"temporary",
		"unavailable",
	}

	for _, fallbackErr := range fallbackErrors {
		if strings.Contains(errMsg, fallbackErr) {
			return true
		}
	}

	// Don't fall back for configuration errors or data validation errors
	if strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "validation") {
		return false
	}

	return true
}

// processSequentialEmbeddingsWithFallback handles sequential embedding processing with fallback error handling.
func (p *DefaultJobProcessor) processSequentialEmbeddingsWithFallback(
	ctx context.Context,
	indexingJobID uuid.UUID,
	repositoryID uuid.UUID,
	chunks []outbound.CodeChunk,
	execution *JobExecution,
) error {
	// Convert UUID to string for logging
	jobID := indexingJobID.String()
	err := p.processSequentialEmbeddings(ctx, indexingJobID, repositoryID, chunks, execution)
	if err != nil {
		slogger.Error(ctx, "Sequential fallback processing failed", slogger.Fields{
			"job_id": jobID,
			"error":  err.Error(),
		})
		return fmt.Errorf("both batch and sequential fallback failed: %w", err)
	}
	return nil
}

// processBatchResultsWithStorage processes batch embedding results with storage error handling.
// This method routes to either test mode (with fake embeddings) or production mode (with real API calls)
// based on the configuration. This separation allows for reliable testing without external dependencies.
func (p *DefaultJobProcessor) processBatchResultsWithStorage(
	ctx context.Context,
	indexingJobID uuid.UUID,
	repositoryID uuid.UUID,
	chunks []outbound.CodeChunk,
	requests []*outbound.EmbeddingRequest,
	execution *JobExecution,
) error {
	// Convert UUID to string for backward compatibility with existing code
	jobID := indexingJobID.String()
	if p.batchConfig.UseTestEmbeddings {
		// Test mode: simulate successful batch processing with dummy results
		slogger.Info(ctx, "Processing batch embedding results (TEST MODE)", slogger.Fields{
			"job_id":        jobID,
			"chunk_count":   len(chunks),
			"request_count": len(requests),
		})

		return p.processEmbeddingsWithStorage(ctx, jobID, repositoryID, chunks, execution,
			func(ctx context.Context, chunk outbound.CodeChunk) (*outbound.EmbeddingResult, error) {
				return p.createTestEmbeddingResult(chunk.Content), nil
			}, "batch")
	}

	// Production mode: use real batch embeddings
	slogger.Info(ctx, "Processing batch embedding results (PRODUCTION MODE)", slogger.Fields{
		"job_id":        jobID,
		"chunk_count":   len(chunks),
		"request_count": len(requests),
	})

	return p.processProductionBatchResults(ctx, indexingJobID, repositoryID, chunks, execution)
}

// processProductionBatchResults processes batch embeddings in production mode.
// This method orchestrates the complete batch processing workflow:
// 1. Check for existing progress and resume from last completed batch
// 2. Split chunks into batches of MaxBatchSize (500)
// 3. Process each batch with retry logic
// 4. Save progress after each batch
// 5. Store embeddings in the database
//
// Returns nil on success, error if batch processing fails.
func (p *DefaultJobProcessor) processProductionBatchResults(
	ctx context.Context,
	indexingJobID uuid.UUID,
	repositoryID uuid.UUID,
	chunks []outbound.CodeChunk,
	execution *JobExecution,
) error {
	// Convert UUID to string for logging
	jobID := indexingJobID.String()

	// Determine batch size (default to 500 if not configured)
	maxBatchSize := p.batchConfig.MaxBatchSize
	if maxBatchSize <= 0 {
		maxBatchSize = 500
	}

	// Split chunks into batches
	batches := p.splitChunksIntoBatches(chunks, maxBatchSize)
	totalBatches := len(batches)

	// Configure embedding options
	embeddingOptions := p.getBatchEmbeddingOptions()

	// Process each batch starting from resume point
	// Use async submission if batch progress repository is available
	useAsyncSubmission := p.batchProgressRepo != nil

	// For sync mode (fallback), embedding service is required
	if !useAsyncSubmission && p.embeddingService == nil {
		return errors.New("embedding service not available for sync batch processing")
	}

	// Check for existing progress and resume from last completed batch
	// Only do this for sync mode, async mode always starts from batch 1
	startBatch := 0
	if !useAsyncSubmission {
		var err error
		startBatch, err = p.resumeFromLastBatch(ctx, indexingJobID)
		if err != nil {
			return err
		}
	}

	// Process batches with fallback support
	err := p.processBatchLoop(
		ctx,
		jobID,
		indexingJobID,
		repositoryID,
		batches,
		startBatch,
		totalBatches,
		embeddingOptions,
		useAsyncSubmission,
		execution,
	)
	if err != nil {
		return err
	}

	if useAsyncSubmission {
		slogger.Info(ctx, "All batches submitted asynchronously", slogger.Fields{
			"job_id":        jobID,
			"total_batches": totalBatches,
			"total_chunks":  len(chunks),
		})
	} else {
		slogger.Info(ctx, "All batches completed successfully (sync mode)", slogger.Fields{
			"job_id":        jobID,
			"total_batches": totalBatches,
			"total_chunks":  len(chunks),
		})
	}

	return nil
}

// processBatchLoop processes batches in a loop with fallback support.
// This method is extracted from processProductionBatchResults to keep function length under 100 lines.
func (p *DefaultJobProcessor) processBatchLoop(
	ctx context.Context,
	jobID string,
	indexingJobID uuid.UUID,
	repositoryID uuid.UUID,
	batches [][]outbound.CodeChunk,
	startBatch int,
	totalBatches int,
	embeddingOptions outbound.EmbeddingOptions,
	useAsyncSubmission bool,
	execution *JobExecution,
) error {
	slogger.Info(ctx, "Starting batch processing loop", slogger.Fields{
		"job_id":        jobID,
		"total_batches": totalBatches,
		"start_batch":   startBatch + 1,
		"async_mode":    useAsyncSubmission,
	})

	for i := startBatch; i < totalBatches; i++ {
		batchNumber := i + 1 // 1-indexed
		batch := batches[i]

		slogger.Info(ctx, "Processing batch", slogger.Fields{
			"job_id":        jobID,
			"batch_number":  batchNumber,
			"batch_size":    len(batch),
			"total_batches": totalBatches,
			"async_mode":    useAsyncSubmission,
		})

		// Extract texts from batch
		texts := p.extractTextsFromChunks(batch)

		var err error
		if useAsyncSubmission {
			err = p.processAsyncBatch(
				ctx,
				jobID,
				indexingJobID,
				repositoryID,
				batchNumber,
				totalBatches,
				batch,
				embeddingOptions,
			)
		} else {
			err = p.processSyncBatch(
				ctx,
				jobID,
				indexingJobID,
				repositoryID,
				batchNumber,
				totalBatches,
				batch,
				texts,
				embeddingOptions,
				execution,
			)
		}
		if err != nil {
			// Check if fallback is enabled and the error is retryable
			if p.batchConfig.FallbackToSequential && p.shouldFallbackOnError(err) {
				slogger.Warn(ctx, "Batch processing failed, falling back to sequential", slogger.Fields{
					"job_id":       jobID,
					"batch_number": batchNumber,
					"error":        err.Error(),
				})
				// Fall back to sequential processing for remaining chunks (including current failed batch)
				remainingChunks := p.collectRemainingChunks(batches, i)
				return p.processSequentialEmbeddingsWithFallback(
					ctx,
					indexingJobID,
					repositoryID,
					remainingChunks,
					execution,
				)
			}
			return err
		}
	}

	return nil
}

// extractTextsFromChunks extracts text content from code chunks for embedding generation.
func (p *DefaultJobProcessor) extractTextsFromChunks(chunks []outbound.CodeChunk) []string {
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}
	return texts
}

// getBatchEmbeddingOptions returns the standard configuration for batch embedding operations.
func (p *DefaultJobProcessor) getBatchEmbeddingOptions() outbound.EmbeddingOptions {
	return outbound.EmbeddingOptions{
		Model:             "gemini-embedding-001",
		TaskType:          outbound.TaskTypeRetrievalDocument,
		IncludeTokenCount: true,
	}
}

// collectRemainingChunks collects all chunks from the current batch onwards.
// This is used when batch processing fails and we need to fall back to sequential
// processing for all remaining chunks (including the failed batch).
func (p *DefaultJobProcessor) collectRemainingChunks(
	batches [][]outbound.CodeChunk,
	fromIndex int,
) []outbound.CodeChunk {
	var remaining []outbound.CodeChunk
	for i := fromIndex; i < len(batches); i++ {
		remaining = append(remaining, batches[i]...)
	}
	return remaining
}

// storeEmbeddingWithProgress stores a single embedding and updates progress tracking.
func (p *DefaultJobProcessor) storeEmbeddingWithProgress(
	ctx context.Context,
	jobID string,
	repositoryID uuid.UUID,
	chunk outbound.CodeChunk,
	result *outbound.EmbeddingResult,
	index int,
	total int,
	execution *JobExecution,
) error {
	// Store the embedding
	if err := p.storeSingleEmbedding(ctx, jobID, repositoryID, chunk, result, index); err != nil {
		return err
	}

	// Update progress counter
	if execution != nil && execution.Progress != nil {
		atomic.AddInt64(&execution.Progress.EmbeddingsCreated, 1)
	}

	// Log progress
	p.logEmbeddingProgress(ctx, jobID, index+1, total, chunk.FilePath)

	return nil
}

// processEmbeddingsWithStorage processes embeddings with storage error handling.
func (p *DefaultJobProcessor) processEmbeddingsWithStorage(
	ctx context.Context,
	jobID string,
	repositoryID uuid.UUID,
	chunks []outbound.CodeChunk,
	execution *JobExecution,
	generator func(ctx context.Context, chunk outbound.CodeChunk) (*outbound.EmbeddingResult, error),
	processingType string,
) error {
	if execution == nil {
		// If no execution provided (direct test call), create a dummy one for progress tracking
		execution = &JobExecution{
			JobID:        uuid.New(),
			RepositoryID: repositoryID,
			StartTime:    time.Now(),
			Status:       jobStatusRunning,
			Progress:     &JobProgress{},
		}
	}

	p.initializeEmbeddingProgress(execution, len(chunks))

	slogger.Info(ctx, "Starting "+processingType+" embedding generation", slogger.Fields{
		"job_id":      jobID,
		"chunk_count": len(chunks),
	})

	for i, chunk := range chunks {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			slogger.Warn(ctx, "Embedding generation cancelled due to context cancellation", slogger.Fields{
				"job_id":      jobID,
				"processing":  processingType,
				"chunk_index": i + 1,
				"total":       len(chunks),
			})
			return fmt.Errorf("context cancelled during %s processing: %w", processingType, ctx.Err())
		default:
		}

		// Generate embedding using the provided generator
		result, err := generator(ctx, chunk)
		if err != nil {
			slogger.Error(ctx, "Failed to generate embedding", slogger.Fields{
				"job_id":      jobID,
				"processing":  processingType,
				"chunk_index": i + 1,
				"chunk_id":    chunk.ID,
				"error":       err.Error(),
			})
			return fmt.Errorf("failed to generate embedding for chunk %d (%s): %w", i, chunk.ID, err)
		}

		// Store the embedding with enhanced error handling
		if err := p.storeSingleEmbeddingWithErrorHandling(ctx, jobID, repositoryID, chunk, result, i); err != nil {
			// Check if it's a storage error that should continue processing vs. fatal error
			if strings.Contains(err.Error(), "database") || strings.Contains(err.Error(), "connection") {
				slogger.Error(ctx, "Storage error, continuing with remaining chunks", slogger.Fields{
					"job_id":      jobID,
					"chunk_index": i + 1,
					"chunk_id":    chunk.ID,
					"error":       err.Error(),
				})
				// Continue processing other chunks instead of failing completely
				continue
			}
			return fmt.Errorf("failed to store embedding for chunk %d (%s): %w", i, chunk.ID, err)
		}

		if execution.Progress != nil {
			atomic.AddInt64(&execution.Progress.EmbeddingsCreated, 1)
		}

		p.logEmbeddingProgress(ctx, jobID, i+1, len(chunks), chunk.FilePath)
	}

	slogger.Info(ctx, processingType+" embedding generation completed", slogger.Fields{
		"job_id":           jobID,
		"total_embeddings": len(chunks),
	})

	return nil
}

// storeSingleEmbeddingWithErrorHandling stores embedding with comprehensive error handling.
func (p *DefaultJobProcessor) storeSingleEmbeddingWithErrorHandling(
	ctx context.Context,
	jobID string,
	repositoryID uuid.UUID,
	chunk outbound.CodeChunk,
	result *outbound.EmbeddingResult,
	chunkIndex int,
) error {
	// Create context with timeout for database operations
	dbCtx, dbCancel := context.WithTimeout(ctx, 30*time.Second)
	defer dbCancel()

	var chunkUUID uuid.UUID
	var err error

	// Handle both string IDs and UUID formats for TDD compatibility
	if len(chunk.ID) == 36 {
		// UUID format
		chunkUUID, err = uuid.Parse(chunk.ID)
		if err != nil {
			slogger.Error(ctx, "Invalid chunk UUID format", slogger.Fields{
				"job_id":   jobID,
				"chunk_id": chunk.ID,
				"error":    err.Error(),
			})
			return fmt.Errorf("invalid chunk UUID format for chunk %d: %w", chunkIndex, err)
		}
	} else {
		// String ID format - generate a UUID for storage (TDD compatibility)
		chunkUUID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(chunk.ID))
	}

	// Ensure chunk carries repository ID for persistence
	chunk.RepositoryID = repositoryID

	embedding := &outbound.Embedding{
		ID:           uuid.New(),
		ChunkID:      chunkUUID,
		RepositoryID: repositoryID,
		Vector:       result.Vector,
		ModelVersion: "gemini-embedding-001",
		CreatedAt:    time.Now().Format(time.RFC3339),
	}

	slogger.Debug(ctx, "Saving chunk and embedding to database", slogger.Fields{
		"job_id":        jobID,
		"chunk_id":      chunk.ID,
		"embedding_id":  embedding.ID.String(),
		"vector_length": len(result.Vector),
	})

	// Handle storage errors explicitly
	if err := p.chunkStorageRepo.SaveChunkWithEmbedding(dbCtx, &chunk, embedding); err != nil {
		errMsg := err.Error()

		// Handle different types of storage errors
		if strings.Contains(errMsg, "database connection failed") || strings.Contains(errMsg, "connection refused") {
			return fmt.Errorf("database connection failed: %w", err)
		}
		if strings.Contains(errMsg, "constraint violation") || strings.Contains(errMsg, "duplicate key") {
			return fmt.Errorf("constraint violation: %w", err)
		}
		if strings.Contains(errMsg, "timeout") {
			return fmt.Errorf("database timeout: %w", err)
		}

		// Generic storage error
		return fmt.Errorf("storage error: %w", err)
	}

	slogger.Debug(ctx, "Chunk and embedding saved successfully", slogger.Fields{
		"job_id":       jobID,
		"chunk_id":     chunk.ID,
		"embedding_id": embedding.ID.String(),
	})

	return nil
}

// countTokensForChunks counts tokens for chunks based on configuration mode.
// Modes: "all" - count all chunks, "sample" - count X% of chunks, "on_demand" - skip counting.
// Errors are logged but do not fail the job (graceful degradation).
func (p *DefaultJobProcessor) countTokensForChunks(ctx context.Context, chunks []outbound.CodeChunk) {
	if !p.batchConfig.TokenCounting.Enabled {
		return
	}

	mode := p.batchConfig.TokenCounting.Mode
	if mode == "on_demand" {
		slogger.Debug(ctx, "Token counting mode is on_demand, skipping", slogger.Field("mode", mode))
		return
	}

	// Select chunks to count based on mode
	var chunksToCount []outbound.CodeChunk
	switch mode {
	case "all":
		chunksToCount = chunks
	case "sample":
		samplePercent := p.batchConfig.TokenCounting.SamplePercent
		if samplePercent <= 0 || samplePercent > 100 {
			slogger.Warn(
				ctx,
				"Invalid sample_percent, skipping token counting",
				slogger.Field("sample_percent", samplePercent),
			)
			return
		}
		sampleSize := (len(chunks) * samplePercent) / 100
		if sampleSize == 0 {
			sampleSize = 1
		}
		chunksToCount = chunks[:sampleSize]
	default:
		slogger.Warn(ctx, "Unknown token counting mode, skipping", slogger.Field("mode", mode))
		return
	}

	if len(chunksToCount) == 0 {
		return
	}

	slogger.Info(ctx, "Starting token counting", slogger.Fields{
		"mode":            mode,
		"total_chunks":    len(chunks),
		"chunks_to_count": len(chunksToCount),
		"sample_percent":  p.batchConfig.TokenCounting.SamplePercent,
	})

	// Extract texts for counting
	texts := make([]string, len(chunksToCount))
	for i, chunk := range chunksToCount {
		texts[i] = chunk.Content
	}

	// Call CountTokensBatch
	results, err := p.embeddingService.CountTokensBatch(ctx, texts, "gemini-embedding-001")
	if err != nil {
		slogger.Warn(
			ctx,
			"Token counting failed, continuing with embedding generation",
			slogger.Field("error", err.Error()),
		)
		return
	}

	// Build updates
	now := time.Now()
	updates := make([]outbound.ChunkTokenUpdate, len(chunksToCount))
	totalTokens := 0
	oversizedChunks := 0
	maxTokensPerChunk := p.batchConfig.TokenCounting.MaxTokensPerChunk
	if maxTokensPerChunk <= 0 {
		maxTokensPerChunk = 8192 // Default to Gemini embedding model limit
	}

	for i, result := range results {
		chunkID, parseErr := uuid.Parse(chunksToCount[i].ID)
		if parseErr != nil {
			slogger.Warn(ctx, "Invalid chunk ID for token count update", slogger.Fields{
				"chunk_id": chunksToCount[i].ID,
				"error":    parseErr.Error(),
			})
			continue
		}
		updates[i] = outbound.ChunkTokenUpdate{
			ChunkID:        chunkID,
			TokenCount:     result.TotalTokens,
			TokenCountedAt: &now,
		}
		totalTokens += result.TotalTokens

		// Check for oversized chunks
		if result.TotalTokens > maxTokensPerChunk {
			oversizedChunks++
			slogger.Warn(ctx, "Chunk exceeds max token limit", slogger.Fields{
				"chunk_id":             chunkID.String(),
				"token_count":          result.TotalTokens,
				"max_tokens_per_chunk": maxTokensPerChunk,
				"file_path":            chunksToCount[i].FilePath,
			})
		}
	}

	// Update repository
	if err := p.chunkStorageRepo.UpdateTokenCounts(ctx, updates); err != nil {
		slogger.Warn(ctx, "Failed to persist token counts, continuing", slogger.Field("error", err.Error()))
		return
	}

	avgTokens := 0
	if len(updates) > 0 {
		avgTokens = totalTokens / len(updates)
	}

	// Emit metrics
	p.emitTokenCountingMetrics(ctx, len(chunksToCount), totalTokens, avgTokens, oversizedChunks)

	slogger.Info(ctx, "Token counting completed successfully", slogger.Fields{
		"chunks_counted":   len(updates),
		"total_tokens":     totalTokens,
		"average_tokens":   avgTokens,
		"oversized_chunks": oversizedChunks,
	})
}

// emitTokenCountingMetrics emits OpenTelemetry metrics for token counting operations.
// This provides observability into token usage, API call patterns, and oversized chunk detection.
func (p *DefaultJobProcessor) emitTokenCountingMetrics(
	ctx context.Context,
	chunksProcessed int,
	totalTokens int,
	avgTokens int,
	oversizedChunks int,
) {
	// Emit total tokens counted (counter metric)
	slogger.Info(ctx, "Token counting metrics", slogger.Fields{
		"metric_name":      "codechunking_tokens_counted_total",
		"metric_type":      "counter",
		"metric_value":     totalTokens,
		"chunks_processed": chunksProcessed,
	})

	// Emit token count API calls (counter metric)
	// Each batch API call processes multiple chunks, so we emit 1 API call per operation
	slogger.Info(ctx, "Token counting API metrics", slogger.Fields{
		"metric_name":      "codechunking_token_count_api_calls_total",
		"metric_type":      "counter",
		"metric_value":     1,
		"chunks_processed": chunksProcessed,
	})

	// Emit tokens per chunk distribution (histogram metric)
	// Log the average as a representative sample for histogram
	slogger.Info(ctx, "Token distribution metrics", slogger.Fields{
		"metric_name":      "codechunking_tokens_per_chunk",
		"metric_type":      "histogram",
		"metric_value":     avgTokens,
		"total_tokens":     totalTokens,
		"chunks_processed": chunksProcessed,
	})

	// Emit oversized chunks counter
	if oversizedChunks > 0 {
		slogger.Info(ctx, "Oversized chunk metrics", slogger.Fields{
			"metric_name":      "codechunking_chunks_over_limit_total",
			"metric_type":      "counter",
			"metric_value":     oversizedChunks,
			"chunks_processed": chunksProcessed,
		})
	}
}

// generateEmbeddingsWithBatch creates embeddings for code chunks using batch processing
// with smart routing decisions based on repository size and available services.
func (p *DefaultJobProcessor) generateEmbeddingsWithBatch(
	ctx context.Context,
	indexingJobID uuid.UUID,
	repositoryID uuid.UUID,
	chunks []outbound.CodeChunk,
) error {
	// Convert UUID to string for backward compatibility with existing code
	jobID := indexingJobID.String()
	// Check for context cancellation first
	if err := ctx.Err(); err != nil {
		slogger.Error(ctx, "Context cancelled before processing", slogger.Fields{
			"job_id": jobID,
			"error":  err.Error(),
		})
		return fmt.Errorf("context cancelled: %w", err)
	}

	// Validate batch processing configuration
	if err := p.validateBatchProcessingConfig(); err != nil {
		slogger.Error(ctx, "Invalid batch processing configuration", slogger.Fields{
			"job_id": jobID,
			"error":  err.Error(),
		})
		return err
	}

	// Validate chunks data before processing
	if err := p.validateChunks(ctx, chunks); err != nil {
		slogger.Error(ctx, "Chunk validation failed", slogger.Fields{
			"job_id": jobID,
			"error":  err.Error(),
		})
		return err
	}

	// Try to get job execution for progress tracking, but don't fail if not found
	execution := p.getJobExecution(jobID)
	if execution == nil {
		// Create a dummy execution for progress tracking in direct test calls
		execution = &JobExecution{
			JobID:        uuid.New(),
			RepositoryID: repositoryID,
			StartTime:    time.Now(),
			Status:       jobStatusRunning,
			Progress:     &JobProgress{},
		}
	}

	// Check if batch processing would normally be used
	meetsThreshold := len(chunks) >= p.batchConfig.ThresholdChunks
	batchEnabled := p.batchConfig.Enabled
	batchDesired := meetsThreshold && batchEnabled

	slogger.Info(ctx, "Making embedding processing decision", slogger.Fields{
		"job_id":            jobID,
		"chunk_count":       len(chunks),
		"batch_enabled":     p.batchConfig.Enabled,
		"threshold_chunks":  p.batchConfig.ThresholdChunks,
		"has_queue_manager": p.batchQueueManager != nil,
		"batch_desired":     batchDesired,
	})

	if batchDesired {
		return p.processBatchEmbeddings(ctx, indexingJobID, repositoryID, chunks, execution)
	} else {
		// Fallback to sequential processing (inline implementation for test compatibility)
		return p.processSequentialEmbeddings(ctx, indexingJobID, repositoryID, chunks, execution)
	}
}

// processBatchEmbeddings handles batch embedding submission and result processing.
func (p *DefaultJobProcessor) processBatchEmbeddings(
	ctx context.Context,
	indexingJobID uuid.UUID,
	repositoryID uuid.UUID,
	chunks []outbound.CodeChunk,
	execution *JobExecution,
) error {
	// Convert UUID to string for backward compatibility with existing code
	jobID := indexingJobID.String()

	// PRE-FLIGHT: Count tokens before embedding generation (non-blocking)
	p.countTokensForChunks(ctx, chunks)

	// Check context before starting batch processing
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before batch processing: %w", err)
	}

	// Check memory usage before batch processing (memory exhaustion check)
	if p.isMemoryPressureHigh() {
		slogger.Warn(ctx, "High memory pressure detected", slogger.Fields{
			"job_id":      jobID,
			"chunk_count": len(chunks),
		})
	}

	// Convert CodeChunk objects to EmbeddingRequest objects
	requests := p.convertChunksToEmbeddingRequests(ctx, jobID, chunks)

	// Check if BatchQueueManager is available
	if p.batchQueueManager == nil {
		// No queue manager - use direct batch processing
		slogger.Info(ctx, "Using direct batch processing (no queue manager)", slogger.Fields{
			"job_id":        jobID,
			"chunk_count":   len(chunks),
			"request_count": len(requests),
		})
		return p.processBatchResultsWithStorage(ctx, indexingJobID, repositoryID, chunks, requests, execution)
	}

	// Submit batch to queue manager with enhanced error handling
	err := p.batchQueueManager.QueueBulkEmbeddingRequests(ctx, requests)
	//nolint:nestif // Complex error handling for different failure scenarios
	if err != nil {
		slogger.Error(ctx, "Batch queue submission failed", slogger.Fields{
			"job_id":        jobID,
			"request_count": len(requests),
			"error":         err.Error(),
		})

		// Analyze error type for appropriate handling
		if p.shouldFallbackOnError(err) && p.batchConfig.FallbackToSequential {
			slogger.Info(ctx, "Attempting fallback to sequential processing", slogger.Fields{
				"job_id":     jobID,
				"error_type": "queue_submission_failed",
				"error":      err.Error(),
			})
			return p.processSequentialEmbeddingsWithFallback(ctx, indexingJobID, repositoryID, chunks, execution)
		}

		// Handle specific error types with appropriate messages
		if strings.Contains(err.Error(), "queue is full") {
			return fmt.Errorf("batch queue full: %w", err)
		}
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "network") {
			return fmt.Errorf("network connectivity error: %w", err)
		}
		if strings.Contains(err.Error(), "partial batch failure") {
			return fmt.Errorf("partial batch failure: %w", err)
		}
		if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "timeout") {
			return fmt.Errorf("batch processing failed (timeout): %w", err)
		}
		if strings.Contains(err.Error(), "memory") || strings.Contains(err.Error(), "cannot allocate") {
			return fmt.Errorf("memory exhaustion: %w", err)
		}
		if strings.Contains(err.Error(), "connection pool") {
			return fmt.Errorf("connection pool exhaustion: %w", err)
		}

		return fmt.Errorf("batch processing failed: %w", err)
	}

	// Process batch results (simplified for TDD - assumes immediate results)
	return p.processBatchResultsWithStorage(ctx, indexingJobID, repositoryID, chunks, requests, execution)
}

// convertChunksToEmbeddingRequests converts CodeChunk objects to EmbeddingRequest objects.
// This method is optimized for performance with pre-allocation and efficient memory usage.
func (p *DefaultJobProcessor) convertChunksToEmbeddingRequests(
	ctx context.Context,
	jobID string,
	chunks []outbound.CodeChunk,
) []*outbound.EmbeddingRequest {
	// Pre-allocate slice with known capacity to avoid multiple allocations
	requests := make([]*outbound.EmbeddingRequest, 0, len(chunks))

	// Cache submission time to ensure consistency across all requests
	submittedAt := time.Now()

	// Cache workspace directory to avoid repeated map lookups
	workspaceDir := p.config.WorkspaceDir

	for _, chunk := range chunks {
		request := p.createEmbeddingRequest(jobID, chunk, workspaceDir, submittedAt)
		requests = append(requests, request)
	}

	slogger.Debug(ctx, "Converted chunks to embedding requests", slogger.Fields{
		"job_id":        jobID,
		"chunk_count":   len(chunks),
		"request_count": len(requests),
	})

	return requests
}

// createEmbeddingRequest creates a single embedding request with optimized configuration.
func (p *DefaultJobProcessor) createEmbeddingRequest(
	jobID string,
	chunk outbound.CodeChunk,
	workspaceDir string,
	submittedAt time.Time,
) *outbound.EmbeddingRequest {
	return &outbound.EmbeddingRequest{
		RequestID:     p.generateRequestID(jobID, chunk.ID),
		CorrelationID: jobID,
		Text:          chunk.Content,
		Priority:      p.getRequestPriority(),
		Options:       p.getEmbeddingOptions(),
		Metadata:      p.createRequestMetadata(chunk, workspaceDir),
		SubmittedAt:   submittedAt,
	}
}

// generateRequestID creates a unique request ID for the chunk.
func (p *DefaultJobProcessor) generateRequestID(jobID, chunkID string) string {
	return fmt.Sprintf("%s-%s", jobID, chunkID)
}

// getRequestPriority determines the priority for embedding requests.
func (p *DefaultJobProcessor) getRequestPriority() outbound.RequestPriority {
	return outbound.PriorityBackground
}

// getEmbeddingOptions returns the configured embedding options.
func (p *DefaultJobProcessor) getEmbeddingOptions() outbound.EmbeddingOptions {
	return outbound.EmbeddingOptions{
		Model:            "gemini-embedding-001",
		TaskType:         outbound.TaskTypeRetrievalDocument,
		TruncateStrategy: outbound.TruncateEnd,
		MaxTokens:        8192,
		Timeout:          120 * time.Second,
		RetryAttempts:    3,
		EnableBatching:   true,
		NormalizeVector:  true,
	}
}

// createRequestMetadata creates metadata for the embedding request.
func (p *DefaultJobProcessor) createRequestMetadata(
	chunk outbound.CodeChunk,
	workspaceDir string,
) map[string]interface{} {
	return map[string]interface{}{
		"chunk_id":      chunk.ID,
		"file_path":     chunk.FilePath,
		"workspace_dir": workspaceDir,
	}
}

// createTestEmbeddingResult creates a dummy embedding result for TDD compatibility.
// NOTE: This function generates proper 768-dimensional vectors to match gemini-embedding-001 model.
func (p *DefaultJobProcessor) createTestEmbeddingResult(content string) *outbound.EmbeddingResult {
	// Generate a 768-dimensional test vector with deterministic values based on content
	vector := make([]float64, 768)
	contentHash := 0
	for _, char := range content {
		contentHash = contentHash*31 + int(char)
	}

	// Create deterministic but varied test vector
	for i := range 768 {
		vector[i] = float64(((contentHash * (i + 1)) % 1000)) / 1000.0 // Values between 0 and 1
	}

	return &outbound.EmbeddingResult{
		Vector:      vector, // Proper 768-dimensional vector for gemini-embedding-001
		Dimensions:  768,
		TokenCount:  len(content) / 4, // Rough estimate
		Model:       "gemini-embedding-001",
		GeneratedAt: time.Now(),
	}
}

// processEmbeddingsWithGenerator processes embeddings using the provided generator function.
// This method eliminates code duplication between sequential and batch processing.
func (p *DefaultJobProcessor) processEmbeddingsWithGenerator(
	ctx context.Context,
	jobID string,
	repositoryID uuid.UUID,
	chunks []outbound.CodeChunk,
	execution *JobExecution,
	generator func(ctx context.Context, chunk outbound.CodeChunk) (*outbound.EmbeddingResult, error),
	processingType string,
) error {
	if execution == nil {
		// If no execution provided (direct test call), create a dummy one for progress tracking
		execution = &JobExecution{
			JobID:        uuid.New(),
			RepositoryID: repositoryID,
			StartTime:    time.Now(),
			Status:       jobStatusRunning,
			Progress:     &JobProgress{},
		}
	}

	p.initializeEmbeddingProgress(execution, len(chunks))

	slogger.Info(ctx, "Starting "+processingType+" embedding generation", slogger.Fields{
		"job_id":      jobID,
		"chunk_count": len(chunks),
	})

	for i, chunk := range chunks {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			slogger.Warn(ctx, "Embedding generation cancelled due to context cancellation", slogger.Fields{
				"job_id":      jobID,
				"processing":  processingType,
				"chunk_index": i + 1,
				"total":       len(chunks),
			})
			return ctx.Err()
		default:
		}

		// Generate embedding using the provided generator
		result, err := generator(ctx, chunk)
		if err != nil {
			slogger.Error(ctx, "Failed to generate embedding", slogger.Fields{
				"job_id":      jobID,
				"processing":  processingType,
				"chunk_index": i + 1,
				"chunk_id":    chunk.ID,
				"error":       err.Error(),
			})
			return fmt.Errorf("failed to generate embedding for chunk %d (%s): %w", i, chunk.ID, err)
		}

		// Store the embedding
		if err := p.storeSingleEmbedding(ctx, jobID, repositoryID, chunk, result, i); err != nil {
			return err
		}

		if execution.Progress != nil {
			atomic.AddInt64(&execution.Progress.EmbeddingsCreated, 1)
		}

		p.logEmbeddingProgress(ctx, jobID, i+1, len(chunks), chunk.FilePath)
	}

	slogger.Info(ctx, processingType+" embedding generation completed", slogger.Fields{
		"job_id":           jobID,
		"total_embeddings": len(chunks),
	})

	return nil
}

// processSequentialEmbeddings handles sequential embedding processing with proper test/production mode handling.
func (p *DefaultJobProcessor) processSequentialEmbeddings(
	ctx context.Context,
	indexingJobID uuid.UUID,
	repositoryID uuid.UUID,
	chunks []outbound.CodeChunk,
	execution *JobExecution,
) error {
	// Convert UUID to string for backward compatibility
	jobID := indexingJobID.String()

	// PRE-FLIGHT: Count tokens before embedding generation (non-blocking)
	p.countTokensForChunks(ctx, chunks)

	if p.batchConfig.UseTestEmbeddings {
		// Test mode: use test embeddings
		return p.processEmbeddingsWithGenerator(ctx, jobID, repositoryID, chunks, execution,
			func(ctx context.Context, chunk outbound.CodeChunk) (*outbound.EmbeddingResult, error) {
				slogger.Debug(
					ctx,
					"Using test embedding for sequential processing",
					slogger.Field("chunk_id", chunk.ID),
				)
				return p.createTestEmbeddingResult(chunk.Content), nil
			}, "sequential")
	}

	// Production mode: use real embeddings
	return p.processEmbeddingsWithGenerator(ctx, jobID, repositoryID, chunks, execution,
		func(ctx context.Context, chunk outbound.CodeChunk) (*outbound.EmbeddingResult, error) {
			slogger.Debug(
				ctx,
				"Generating real embedding for sequential processing",
				slogger.Field("chunk_id", chunk.ID),
			)

			if p.embeddingService == nil {
				return nil, errors.New("embedding service not initialized")
			}

			embeddingOptions := outbound.EmbeddingOptions{
				Model:             "gemini-embedding-001",
				TaskType:          outbound.TaskTypeRetrievalDocument,
				IncludeTokenCount: true,
			}

			return p.embeddingService.GenerateEmbedding(ctx, chunk.Content, embeddingOptions)
		}, "sequential")
}

func (p *DefaultJobProcessor) logEmbeddingProgress(
	ctx context.Context,
	jobID string,
	completed, total int,
	currentFile string,
) {
	if completed%10 == 0 || completed == total {
		slogger.Info(ctx, "Embedding generation progress", slogger.Fields{
			"job_id":       jobID,
			"completed":    completed,
			"total":        total,
			"current_file": currentFile,
			"progress_pct": fmt.Sprintf("%.1f", float64(completed)/float64(total)*100),
		})
	}
}

// updateMetrics updates the processor metrics.
func (p *DefaultJobProcessor) updateMetrics(chunksCount int64) {
	atomic.AddInt64(&p.metrics.TotalJobsProcessed, 1)
	atomic.AddInt64(&p.metrics.FilesProcessed, chunksCount)
	atomic.AddInt64(&p.metrics.ChunksGenerated, chunksCount)
	atomic.AddInt64(&p.metrics.EmbeddingsCreated, chunksCount)
	p.healthStatus.LastJobTime = time.Now()
	atomic.AddInt64(&p.healthStatus.CompletedJobs, 1)
}

// updateJobStatus safely updates the job status.
func (p *DefaultJobProcessor) updateJobStatus(jobID string, status string) {
	p.jobsMu.RLock()
	execution, exists := p.activeJobs[jobID]
	p.jobsMu.RUnlock()

	if exists {
		execution.mu.Lock()
		execution.Status = status
		execution.mu.Unlock()
	}

	if status != jobStatusRunning {
		p.jobsMu.Lock()
		delete(p.activeJobs, jobID)
		p.healthStatus.ActiveJobs = len(p.activeJobs)
		p.jobsMu.Unlock()
	}
}

// cleanupWorkspace removes the workspace directory for a job.
func (p *DefaultJobProcessor) cleanupWorkspace(workspacePath, jobID string) {
	_ = os.RemoveAll(workspacePath)
}

// isMemoryPressureHigh checks if memory usage is above threshold.
func (p *DefaultJobProcessor) isMemoryPressureHigh() bool {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Check if MaxMemoryMB is within safe range
	if p.config.MaxMemoryMB <= 0 || p.config.MaxMemoryMB > math.MaxInt32 {
		return false
	}

	// Safe conversion with bounds checking
	const bytesPerMB = 1024 * 1024
	allocMB := m.Alloc / bytesPerMB
	var currentMemoryMB int
	if allocMB > uint64(math.MaxInt) {
		currentMemoryMB = math.MaxInt
	} else {
		currentMemoryMB = int(allocMB)
	}
	return currentMemoryMB > p.config.MaxMemoryMB
}

// updateRepositoryStatus updates the repository status in the database.
func (p *DefaultJobProcessor) updateRepositoryStatus(
	ctx context.Context,
	repositoryID uuid.UUID,
	statusStr string,
) error {
	// Import the valueobject package for status validation
	status, err := valueobject.NewRepositoryStatus(statusStr)
	if err != nil {
		return fmt.Errorf("invalid repository status: %w", err)
	}

	// Fetch the repository
	repo, err := p.repositoryRepo.FindByID(ctx, repositoryID)
	if err != nil {
		return fmt.Errorf("failed to find repository: %w", err)
	}
	if repo == nil {
		return fmt.Errorf("repository not found: %s", repositoryID.String())
	}

	// Update the status
	if err := repo.UpdateStatus(status); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Persist the change
	if err := p.repositoryRepo.Update(ctx, repo); err != nil {
		return fmt.Errorf("failed to persist repository: %w", err)
	}

	slogger.Info(ctx, "Repository status updated", slogger.Fields{
		"repository_id": repositoryID.String(),
		"status":        statusStr,
	})

	return nil
}

// markRepositoryFailed marks a repository as failed.
func (p *DefaultJobProcessor) markRepositoryFailed(ctx context.Context, repositoryID uuid.UUID) {
	if err := p.updateRepositoryStatus(ctx, repositoryID, "failed"); err != nil {
		slogger.Error(ctx, "Failed to mark repository as failed", slogger.Fields{
			"repository_id": repositoryID.String(),
			"error":         err.Error(),
		})
	}
}

// markRepositoryCompleted marks a repository as successfully indexed.
func (p *DefaultJobProcessor) markRepositoryCompleted(
	ctx context.Context,
	repositoryID uuid.UUID,
	commitHash string,
	fileCount, chunkCount int,
) {
	// Fetch the repository
	repo, err := p.repositoryRepo.FindByID(ctx, repositoryID)
	if err != nil {
		slogger.Error(ctx, "Failed to find repository for completion", slogger.Fields{
			"repository_id": repositoryID.String(),
			"error":         err.Error(),
		})
		return
	}
	if repo == nil {
		slogger.Error(ctx, "Repository not found for completion", slogger.Fields{
			"repository_id": repositoryID.String(),
		})
		return
	}

	// Mark as completed with metadata
	if err := repo.MarkIndexingCompleted(commitHash, fileCount, chunkCount); err != nil {
		slogger.Error(ctx, "Failed to mark repository as completed", slogger.Fields{
			"repository_id": repositoryID.String(),
			"error":         err.Error(),
		})
		return
	}

	// Persist the change
	if err := p.repositoryRepo.Update(ctx, repo); err != nil {
		slogger.Error(ctx, "Failed to persist completed repository", slogger.Fields{
			"repository_id": repositoryID.String(),
			"error":         err.Error(),
		})
		return
	}

	slogger.Info(ctx, "Repository marked as completed", slogger.Fields{
		"repository_id": repositoryID.String(),
		"total_files":   fileCount,
		"total_chunks":  chunkCount,
		"commit_hash":   commitHash,
	})
}

// splitChunksIntoBatches splits an array of chunks into batches of maxBatchSize.
// Returns an empty slice if chunks is empty.
// Preserves the original order of chunks and handles remainders in the last batch.
func (p *DefaultJobProcessor) splitChunksIntoBatches(
	chunks []outbound.CodeChunk,
	maxBatchSize int,
) [][]outbound.CodeChunk {
	if len(chunks) == 0 {
		return [][]outbound.CodeChunk{}
	}

	batches := [][]outbound.CodeChunk{}
	for i := 0; i < len(chunks); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batches = append(batches, chunks[i:end])
	}
	return batches
}

// processAsyncBatch handles async batch submission.
func (p *DefaultJobProcessor) processAsyncBatch(
	ctx context.Context,
	jobID string,
	indexingJobID uuid.UUID,
	repositoryID uuid.UUID,
	batchNumber int,
	totalBatches int,
	chunks []outbound.CodeChunk,
	embeddingOptions outbound.EmbeddingOptions,
) error {
	// Async mode: Submit batch job and return immediately
	// The BatchPoller will handle completion polling
	err := p.submitBatchJobAsync(ctx, indexingJobID, repositoryID, batchNumber, totalBatches, chunks, embeddingOptions)
	if err != nil {
		// Check if this is a quota error and provide clearer messaging
		if isQuotaError(err) {
			slogger.Error(
				ctx,
				"GEMINI QUOTA EXCEEDED - Batch processing failed due to API quota limits",
				slogger.Fields{
					"job_id":                      jobID,
					"batch_number":                batchNumber,
					"error":                       err.Error(),
					"action_required":             "Check Google Cloud Console quota or wait for reset",
					"output_files_will_not_exist": "No output files are created when quota is exceeded",
				},
			)
		} else {
			slogger.Error(ctx, "Failed to submit async batch job", slogger.Fields{
				"job_id":       jobID,
				"batch_number": batchNumber,
				"error":        err.Error(),
			})
		}
		// Save failed progress
		_ = p.saveBatchProgress(
			ctx,
			repositoryID,
			indexingJobID,
			batchNumber,
			totalBatches,
			0,
			jobStatusFailed,
			err.Error(),
		)
		return err
	}

	slogger.Info(ctx, "Batch job submitted asynchronously", slogger.Fields{
		"job_id":       jobID,
		"batch_number": batchNumber,
	})
	return nil
}

// processSyncBatch handles synchronous batch processing.
func (p *DefaultJobProcessor) processSyncBatch(
	ctx context.Context,
	jobID string,
	indexingJobID uuid.UUID,
	repositoryID uuid.UUID,
	batchNumber int,
	totalBatches int,
	batch []outbound.CodeChunk,
	texts []string,
	embeddingOptions outbound.EmbeddingOptions,
	execution *JobExecution,
) error {
	// Sync mode (fallback): Wait for results immediately
	// This is used when batch progress repository is not available (tests)
	results, err := p.processBatchWithRetry(
		ctx,
		indexingJobID,
		repositoryID,
		batchNumber,
		texts,
		batch,
		embeddingOptions,
	)
	if err != nil {
		// Save failed progress
		_ = p.saveBatchProgress(
			ctx,
			repositoryID,
			indexingJobID,
			batchNumber,
			totalBatches,
			0,
			jobStatusFailed,
			err.Error(),
		)
		return err
	}

	// Store embeddings for this batch
	for j, chunk := range batch {
		result := &results[j]
		if err := p.storeEmbeddingWithProgress(ctx, jobID, repositoryID, chunk, result, j, len(batch), execution); err != nil {
			// Save failed progress
			_ = p.saveBatchProgress(
				ctx,
				repositoryID,
				indexingJobID,
				batchNumber,
				totalBatches,
				0,
				jobStatusFailed,
				err.Error(),
			)
			return err
		}
	}

	// Save completed progress for this batch
	err = p.saveBatchProgress(
		ctx,
		repositoryID,
		indexingJobID,
		batchNumber,
		totalBatches,
		len(batch),
		jobStatusCompleted,
	)
	if err != nil {
		return err
	}

	slogger.Info(ctx, "Batch completed successfully (sync mode)", slogger.Fields{
		"job_id":           jobID,
		"batch_number":     batchNumber,
		"chunks_processed": len(batch),
	})
	return nil
}

// submitBatchJobAsync submits a batch embedding job to Gemini API asynchronously.
// Instead of waiting for results, it saves the batch progress with the Gemini job ID
// and returns immediately. The BatchPoller will handle completion polling.
func (p *DefaultJobProcessor) submitBatchJobAsync(
	ctx context.Context,
	indexingJobID uuid.UUID,
	repositoryID uuid.UUID,
	batchNumber int,
	totalBatches int,
	chunks []outbound.CodeChunk,
	options outbound.EmbeddingOptions,
) error {
	// Check if batch embedding service is configured
	if p.batchEmbeddingService == nil {
		return errors.New("batch embedding service not configured")
	}

	slogger.Info(ctx, "Queueing batch job for submission", slogger.Fields{
		"indexing_job_id": indexingJobID,
		"batch_number":    batchNumber,
		"total_batches":   totalBatches,
		"chunk_count":     len(chunks),
	})

	// Set repository ID on all chunks before saving (required by FindOrCreateChunks)
	for i := range chunks {
		chunks[i].RepositoryID = repositoryID
	}

	// FindOrCreateChunks returns chunks with their actual persisted IDs
	// This prevents FK constraint violations when embeddings reference chunk IDs
	slogger.Debug(ctx, "Finding or creating chunks before queueing batch", slogger.Fields{
		"indexing_job_id": indexingJobID,
		"batch_number":    batchNumber,
		"chunk_count":     len(chunks),
	})

	savedChunks, err := p.chunkStorageRepo.FindOrCreateChunks(ctx, chunks)
	if err != nil {
		return fmt.Errorf("failed to find/create chunks: %w", err)
	}

	slogger.Info(ctx, "Chunks saved successfully", slogger.Fields{
		"indexing_job_id": indexingJobID,
		"batch_number":    batchNumber,
		"chunk_count":     len(savedChunks),
	})

	// Create batch requests with the ACTUAL persisted chunk IDs
	requests := make([]*outbound.BatchEmbeddingRequest, len(savedChunks))
	for i, chunk := range savedChunks {
		chunkID, err := uuid.Parse(chunk.ID)
		if err != nil {
			return fmt.Errorf("invalid chunk ID: %w", err)
		}

		requests[i] = &outbound.BatchEmbeddingRequest{
			RequestID: EncodeChunkIDToRequestID(chunkID),
			Text:      chunk.Content,
		}
	}

	// Serialize request data for deferred submission
	requestData, err := json.Marshal(requests)
	if err != nil {
		return fmt.Errorf("failed to serialize request data: %w", err)
	}

	// Create batch progress record
	progress := entity.NewBatchJobProgress(repositoryID, indexingJobID, batchNumber, totalBatches)

	// Mark as pending submission instead of immediately submitting
	progress.MarkPendingSubmission(requestData)

	// Save progress to database - BatchSubmitter will pick it up
	if err := p.batchProgressRepo.Save(ctx, progress); err != nil {
		return fmt.Errorf("failed to save batch progress: %w", err)
	}

	slogger.Info(ctx, "Batch queued for submission", slogger.Fields{
		"batch_id":        progress.ID(),
		"indexing_job_id": indexingJobID,
		"batch_number":    batchNumber,
		"chunk_count":     len(savedChunks),
	})

	return nil
}

// processBatchWithRetry attempts to generate batch embeddings with retry logic for quota errors.
// This method is kept for backward compatibility with existing tests that expect synchronous behavior.
func (p *DefaultJobProcessor) processBatchWithRetry(
	ctx context.Context,
	indexingJobID uuid.UUID,
	repositoryID uuid.UUID,
	batchNumber int,
	texts []string,
	chunks []outbound.CodeChunk,
	options outbound.EmbeddingOptions,
) ([]outbound.EmbeddingResult, error) {
	var lastErr error

	// Use configured max retries (0 means no retries, only initial attempt)
	maxRetries := p.batchConfig.MaxRetries

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Call embedding service
		results, err := p.embeddingService.GenerateBatchEmbeddings(ctx, texts, options)
		if err == nil {
			// Success - convert from pointers to values
			valueResults := make([]outbound.EmbeddingResult, len(results))
			for i, r := range results {
				valueResults[i] = *r
			}
			return valueResults, nil
		}

		lastErr = err

		// Check if error is retryable
		var embErr *outbound.EmbeddingError
		if !errors.As(err, &embErr) || !embErr.Retryable {
			// Non-retryable error - return immediately
			return nil, err
		}

		// Check if we've exhausted retries
		if attempt >= maxRetries {
			break
		}

		// Calculate backoff duration
		backoffDuration := p.calculateBackoff(attempt)

		// Sleep with context cancellation check
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoffDuration):
			// Continue to next retry
		}
	}

	// Max retries exceeded
	return nil, fmt.Errorf("max retries exceeded (%d): %w", maxRetries, lastErr)
}

// calculateBackoff calculates the backoff duration for a retry attempt.
func (p *DefaultJobProcessor) calculateBackoff(retryCount int) time.Duration {
	// If both backoff values are not configured (0), return 0 for instant retry (test mode)
	if p.batchConfig.InitialBackoff <= 0 && p.batchConfig.MaxBackoff <= 0 {
		return 0
	}

	// Set defaults if not configured
	initialBackoff := p.batchConfig.InitialBackoff
	if initialBackoff <= 0 {
		initialBackoff = 30 * time.Second
	}

	maxBackoff := p.batchConfig.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 5 * time.Minute
	}

	// Calculate exponential backoff: InitialBackoff * 2^retryCount
	backoff := initialBackoff
	for range retryCount {
		backoff *= 2
	}

	// Cap at MaxBackoff
	if backoff > maxBackoff {
		backoff = maxBackoff
	}

	return backoff
}

// saveBatchProgress saves batch progress to the repository.
func (p *DefaultJobProcessor) saveBatchProgress(
	ctx context.Context,
	repositoryID uuid.UUID,
	jobID uuid.UUID,
	batchNumber int,
	totalBatches int,
	chunksProcessed int,
	status string,
	errorMsg ...string,
) error {
	// If no batch progress repository, skip saving
	if p.batchProgressRepo == nil {
		return nil
	}

	progress := entity.NewBatchJobProgress(repositoryID, jobID, batchNumber, totalBatches)

	if status == jobStatusCompleted {
		progress.MarkCompleted(chunksProcessed)
	} else if status == jobStatusFailed && len(errorMsg) > 0 {
		progress.MarkFailed(errorMsg[0])
	}

	return p.batchProgressRepo.Save(ctx, progress)
}

// resumeFromLastBatch finds the highest completed batch number for a job.
func (p *DefaultJobProcessor) resumeFromLastBatch(
	ctx context.Context,
	jobID uuid.UUID,
) (int, error) {
	// If no batch progress repository, start from beginning
	if p.batchProgressRepo == nil {
		return 0, nil
	}

	batches, err := p.batchProgressRepo.GetByJobID(ctx, jobID)
	if err != nil {
		return 0, err
	}

	lastBatch := 0
	for _, batch := range batches {
		if batch.Status() == jobStatusCompleted && batch.BatchNumber() > lastBatch {
			lastBatch = batch.BatchNumber()
		}
	}

	return lastBatch, nil
}
