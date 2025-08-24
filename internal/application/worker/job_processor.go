package worker

import (
	"codechunking/internal/domain/messaging"
	"codechunking/internal/port/inbound"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
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
	Progress     JobProgress
}

// JobProgress tracks job processing progress.
type JobProgress struct {
	FilesProcessed    int
	ChunksGenerated   int
	EmbeddingsCreated int
	BytesProcessed    int64
	CurrentFile       string
	Stage             string
}

// DefaultJobProcessor is the default implementation.
type DefaultJobProcessor struct {
	config             JobProcessorConfig
	indexingJobRepo    outbound.IndexingJobRepository
	repositoryRepo     outbound.RepositoryRepository
	gitClient          outbound.GitClient
	codeParser         outbound.CodeParser
	embeddingGenerator outbound.EmbeddingGenerator
	activeJobs         map[string]*JobExecution
	metrics            inbound.JobProcessorMetrics
	healthStatus       inbound.JobProcessorHealthStatus
}

// NewDefaultJobProcessor creates a new default job processor.
func NewDefaultJobProcessor(
	config JobProcessorConfig,
	indexingJobRepo outbound.IndexingJobRepository,
	repositoryRepo outbound.RepositoryRepository,
	gitClient outbound.GitClient,
	codeParser outbound.CodeParser,
	embeddingGenerator outbound.EmbeddingGenerator,
) inbound.JobProcessor {
	return &DefaultJobProcessor{
		config:             config,
		indexingJobRepo:    indexingJobRepo,
		repositoryRepo:     repositoryRepo,
		gitClient:          gitClient,
		codeParser:         codeParser,
		embeddingGenerator: embeddingGenerator,
		activeJobs:         make(map[string]*JobExecution),
	}
}

// ProcessJob processes an indexing job message.
func (p *DefaultJobProcessor) ProcessJob(ctx context.Context, message messaging.EnhancedIndexingJobMessage) error {
	// Basic validation
	if message.RepositoryID == uuid.Nil {
		return errors.New("repository ID is required")
	}
	if message.RepositoryURL == "" {
		return errors.New("repository URL is required")
	}

	// For GREEN phase, simulate job processing
	p.updateJobStart(message.RepositoryID)

	// Simulate work with a brief delay
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond): // Minimal delay for GREEN phase
	}

	p.updateJobCompletion(true, time.Since(time.Now()))
	return nil
}

// GetHealthStatus returns the current health status.
func (p *DefaultJobProcessor) GetHealthStatus() inbound.JobProcessorHealthStatus {
	return p.healthStatus
}

// GetMetrics returns job processing metrics.
func (p *DefaultJobProcessor) GetMetrics() inbound.JobProcessorMetrics {
	return p.metrics
}

// Cleanup performs cleanup operations.
func (p *DefaultJobProcessor) Cleanup() error {
	// For GREEN phase, simulate successful cleanup
	return nil
}

// updateJobStart updates metrics when a job starts.
func (p *DefaultJobProcessor) updateJobStart(_ uuid.UUID) {
	p.healthStatus.ActiveJobs++
	p.healthStatus.IsReady = true
	p.healthStatus.LastJobTime = time.Now()
}

// updateJobCompletion updates metrics when a job completes.
func (p *DefaultJobProcessor) updateJobCompletion(success bool, duration time.Duration) {
	p.healthStatus.ActiveJobs--
	if p.healthStatus.ActiveJobs < 0 {
		p.healthStatus.ActiveJobs = 0
	}

	if success {
		p.healthStatus.CompletedJobs++
		p.metrics.TotalJobsProcessed++
	} else {
		p.healthStatus.FailedJobs++
		p.metrics.TotalJobsFailed++
	}

	// Update average processing time (simplified)
	p.metrics.AverageProcessingTime = duration
	p.healthStatus.AverageJobTime = duration
}
