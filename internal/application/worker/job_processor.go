package worker

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/messaging"
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
	// Basic validation
	if err := p.validateMessage(message); err != nil {
		return err
	}

	jobID := uuid.New()
	if message.MessageID == "" {
		message.MessageID = jobID.String()
	}

	// Check memory pressure before starting job
	if p.isMemoryPressureHigh() {
		return errors.New("memory pressure too high, rejecting job")
	}

	// Acquire semaphore for concurrency control
	p.semaphore <- struct{}{}
	defer func() {
		<-p.semaphore
	}()

	// Create context with timeout
	jobCtx, cancel := context.WithTimeout(ctx, p.config.JobTimeout)
	defer cancel()

	execution := &JobExecution{
		JobID:        jobID,
		RepositoryID: message.RepositoryID,
		StartTime:    time.Now(),
		Status:       jobStatusRunning,
		Progress:     &JobProgress{},
	}

	p.jobsMu.Lock()
	p.activeJobs[message.MessageID] = execution
	p.healthStatus.ActiveJobs = len(p.activeJobs)
	p.jobsMu.Unlock()

	// Ensure cleanup of workspace directory
	workspacePath := filepath.Join(p.config.WorkspaceDir, message.MessageID)
	defer p.cleanupWorkspace(workspacePath, message.MessageID)

	if err := p.setupWorkspace(workspacePath); err != nil {
		p.updateJobStatus(message.MessageID, jobStatusFailed)
		return err
	}

	if err := p.cloneRepository(jobCtx, message, workspacePath); err != nil {
		p.updateJobStatus(message.MessageID, jobStatusFailed)
		return err
	}

	chunkSize := p.getChunkSize(message)
	config := outbound.CodeParsingConfig{
		ChunkSizeBytes: chunkSize,
	}

	chunks, err := p.parseCode(jobCtx, workspacePath, config)
	if err != nil {
		p.updateJobStatus(message.MessageID, jobStatusFailed)
		return err
	}

	if err := p.generateEmbeddings(jobCtx, message.MessageID, message.RepositoryID, chunks); err != nil {
		p.updateJobStatus(message.MessageID, jobStatusFailed)
		return err
	}

	p.updateJobStatus(message.MessageID, jobStatusCompleted)
	if execution.Progress != nil {
		execution.Progress.Stage = jobStatusCompleted
	}

	// Update metrics atomically
	p.updateMetrics(int64(len(chunks)))

	return nil
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
	chunks, err := p.codeParser.ParseDirectory(ctx, workspacePath, config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse directory: %w", err)
	}
	return chunks, nil
}

// generateEmbeddings creates embeddings for code chunks.
func (p *DefaultJobProcessor) generateEmbeddings(
	ctx context.Context,
	jobID string,
	repositoryID uuid.UUID,
	chunks []outbound.CodeChunk,
) error {
	p.jobsMu.RLock()
	execution := p.activeJobs[jobID]
	p.jobsMu.RUnlock()

	// Check if execution exists (job may have been removed from activeJobs)
	if execution == nil {
		return fmt.Errorf("job execution not found for jobID: %s", jobID)
	}

	// Ensure Progress is initialized
	if execution.Progress == nil {
		execution.Progress = &JobProgress{}
	}

	atomic.AddInt64(&execution.Progress.ChunksGenerated, int64(len(chunks)))
	execution.Progress.Stage = "parsing_complete"

	slogger.Info(ctx, "Starting embedding generation", slogger.Fields{
		"job_id":      jobID,
		"chunk_count": len(chunks),
	})

	for i, chunk := range chunks {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if execution.Progress != nil {
			execution.Progress.CurrentFile = chunk.FilePath
		}
		// Generate embedding using Gemini API
		options := outbound.EmbeddingOptions{
			Model:    "text-embedding-004",
			TaskType: outbound.TaskTypeRetrievalDocument,
			Timeout:  30 * time.Second,
		}

		result, err := p.embeddingService.GenerateEmbedding(ctx, chunk.Content, options)
		if err != nil {
			return fmt.Errorf("failed to generate embedding for chunk %d: %w", i, err)
		}

		// Parse chunk ID as UUID for database storage
		chunkUUID, err := uuid.Parse(chunk.ID)
		if err != nil {
			return fmt.Errorf("invalid chunk ID format for chunk %d: %w", i, err)
		}

		// Create embedding struct for database storage
		embedding := &outbound.Embedding{
			ID:           uuid.New(),
			ChunkID:      chunkUUID,
			RepositoryID: repositoryID,
			Vector:       result.Vector,
			ModelVersion: "text-embedding-004",
			CreatedAt:    time.Now().Format(time.RFC3339),
		}

		// Store chunk and embedding in database using transactional method
		if err := p.chunkStorageRepo.SaveChunkWithEmbedding(ctx, &chunk, embedding); err != nil {
			return fmt.Errorf("failed to store chunk and embedding for chunk %d: %w", i, err)
		}

		slogger.Debug(ctx, "Generated and stored embedding", slogger.Fields{
			"chunk_id":    chunk.ID,
			"dimensions":  result.Dimensions,
			"vector_size": len(result.Vector),
		})
		if execution.Progress != nil {
			atomic.AddInt64(&execution.Progress.EmbeddingsCreated, 1)
		}

		// Log progress every 10 chunks
		if (i+1)%10 == 0 || i == len(chunks)-1 {
			slogger.Info(ctx, "Embedding generation progress", slogger.Fields{
				"job_id":       jobID,
				"completed":    i + 1,
				"total":        len(chunks),
				"current_file": chunk.FilePath,
			})
		}
	}

	slogger.Info(ctx, "Embedding generation completed", slogger.Fields{
		"job_id":           jobID,
		"total_embeddings": len(chunks),
	})

	return nil
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
