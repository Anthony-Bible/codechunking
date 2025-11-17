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
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/messaging"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/inbound"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
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
	config           JobProcessorConfig
	indexingJobRepo  outbound.IndexingJobRepository
	repositoryRepo   outbound.RepositoryRepository
	gitClient        outbound.GitClient
	codeParser       outbound.CodeParser
	embeddingService outbound.EmbeddingService
	chunkStorageRepo outbound.ChunkStorageRepository
	activeJobs       map[string]*JobExecution
	jobsMu           sync.RWMutex
	semaphore        chan struct{}
	metrics          inbound.JobProcessorMetrics
	healthStatus     inbound.JobProcessorHealthStatus
}

// NewDefaultJobProcessor creates a new default job processor.
func NewDefaultJobProcessor(
	config JobProcessorConfig,
	indexingJobRepo outbound.IndexingJobRepository,
	repositoryRepo outbound.RepositoryRepository,
	gitClient outbound.GitClient,
	codeParser outbound.CodeParser,
	embeddingService outbound.EmbeddingService,
	chunkStorageRepo outbound.ChunkStorageRepository,
) inbound.JobProcessor {
	// Ensure proper semaphore initialization
	maxConcurrent := config.MaxConcurrentJobs
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}

	processor := &DefaultJobProcessor{
		config:           config,
		indexingJobRepo:  indexingJobRepo,
		repositoryRepo:   repositoryRepo,
		gitClient:        gitClient,
		codeParser:       codeParser,
		embeddingService: embeddingService,
		chunkStorageRepo: chunkStorageRepo,
		activeJobs:       make(map[string]*JobExecution),
		semaphore:        make(chan struct{}, maxConcurrent),
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

	if err := p.generateEmbeddings(ctx, message.MessageID, message.RepositoryID, chunks); err != nil {
		p.updateJobStatus(message.MessageID, jobStatusFailed)
		p.markRepositoryFailed(ctx, message.RepositoryID)
		return nil, err
	}

	return chunks, nil
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
	jobID string,
	repositoryID uuid.UUID,
	chunks []outbound.CodeChunk,
) error {
	execution := p.getJobExecution(jobID)
	if execution == nil {
		return fmt.Errorf("job execution not found for jobID: %s", jobID)
	}

	p.initializeEmbeddingProgress(execution, len(chunks))

	slogger.Info(ctx, "Starting embedding generation", slogger.Fields{
		"job_id":      jobID,
		"chunk_count": len(chunks),
	})

	for i, chunk := range chunks {
		if err := p.processChunkEmbedding(ctx, jobID, repositoryID, chunk, i, len(chunks), execution); err != nil {
			return err
		}

		p.logEmbeddingProgress(ctx, jobID, i+1, len(chunks), chunk.FilePath)
	}

	slogger.Info(ctx, "Embedding generation completed", slogger.Fields{
		"job_id":           jobID,
		"total_embeddings": len(chunks),
	})

	return nil
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

func (p *DefaultJobProcessor) processChunkEmbedding(
	ctx context.Context,
	jobID string,
	repositoryID uuid.UUID,
	chunk outbound.CodeChunk,
	chunkIndex, totalChunks int,
	execution *JobExecution,
) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		slogger.Warn(ctx, "Embedding generation cancelled due to context cancellation", slogger.Fields{
			"job_id":   jobID,
			"chunk_id": chunkIndex + 1,
			"total":    totalChunks,
		})
		return ctx.Err()
	default:
	}

	if execution.Progress != nil {
		execution.Progress.CurrentFile = chunk.FilePath
	}

	// Generate the embedding
	result, err := p.generateSingleEmbedding(ctx, jobID, chunk, chunkIndex)
	if err != nil {
		return err
	}

	// Store the embedding
	if err := p.storeSingleEmbedding(ctx, jobID, repositoryID, chunk, result, chunkIndex); err != nil {
		return err
	}

	if execution.Progress != nil {
		atomic.AddInt64(&execution.Progress.EmbeddingsCreated, 1)
	}

	return nil
}

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
	chunkUUID, err := uuid.Parse(chunk.ID)
	if err != nil {
		slogger.Error(ctx, "Invalid chunk ID format", slogger.Fields{
			"job_id":   jobID,
			"chunk_id": chunk.ID,
			"error":    err.Error(),
		})
		return fmt.Errorf("invalid chunk ID format for chunk %d: %w", chunkIndex, err)
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
