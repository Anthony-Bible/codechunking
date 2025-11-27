package worker

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/entity"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// BatchPoller polls for pending Gemini batch jobs and processes their results asynchronously.
// This prevents workers from blocking while waiting for batch job completion.
type BatchPoller struct {
	batchProgressRepo     outbound.BatchProgressRepository
	chunkRepo             outbound.ChunkStorageRepository
	batchEmbeddingService outbound.BatchEmbeddingService
	pollInterval          time.Duration
	maxConcurrentPolls    int
	stopCh                chan struct{}
	wg                    sync.WaitGroup
	mu                    sync.Mutex
	running               bool
}

// BatchPollerConfig holds configuration for the batch poller.
type BatchPollerConfig struct {
	PollInterval       time.Duration
	MaxConcurrentPolls int
}

// NewBatchPoller creates a new batch poller instance.
func NewBatchPoller(
	batchProgressRepo outbound.BatchProgressRepository,
	chunkRepo outbound.ChunkStorageRepository,
	batchEmbeddingService outbound.BatchEmbeddingService,
	config BatchPollerConfig,
) *BatchPoller {
	// Set defaults
	if config.PollInterval <= 0 {
		config.PollInterval = 30 * time.Second
	}
	if config.MaxConcurrentPolls <= 0 {
		config.MaxConcurrentPolls = 5
	}

	return &BatchPoller{
		batchProgressRepo:     batchProgressRepo,
		chunkRepo:             chunkRepo,
		batchEmbeddingService: batchEmbeddingService,
		pollInterval:          config.PollInterval,
		maxConcurrentPolls:    config.MaxConcurrentPolls,
		stopCh:                make(chan struct{}),
	}
}

// EncodeChunkIDToRequestID converts a chunk UUID to a Gemini-compatible RequestID.
// Format: "chunk_<uuid_without_hyphens>"
// Example: 550e8400-e29b-41d4-a716-446655440000 -> chunk_550e8400e29b41d4a716446655440000.
func EncodeChunkIDToRequestID(chunkID uuid.UUID) string {
	uuidStr := chunkID.String()
	uuidWithoutHyphens := strings.ReplaceAll(uuidStr, "-", "")
	return "chunk_" + uuidWithoutHyphens
}

// decodeRequestIDToChunkID converts a Gemini RequestID back to a chunk UUID.
// Expected format: "chunk_<uuid>" where uuid can be with or without hyphens
// Returns error if format is invalid or UUID cannot be parsed.
func decodeRequestIDToChunkID(requestID string) (uuid.UUID, error) {
	if requestID == "" {
		return uuid.Nil, errors.New("invalid format: empty RequestID")
	}

	if !strings.HasPrefix(requestID, "chunk_") {
		return uuid.Nil, errors.New("invalid format: missing chunk_ prefix")
	}

	uuidPart := requestID[6:]

	// Handle both formats: with hyphens (36 chars) and without hyphens (32 chars)
	var uuidWithHyphens string
	if len(uuidPart) == 36 {
		// UUID already has hyphens
		uuidWithHyphens = uuidPart
	} else if len(uuidPart) == 32 {
		// UUID without hyphens - add them
		uuidWithHyphens = uuidPart[0:8] + "-" + uuidPart[8:12] + "-" + uuidPart[12:16] + "-" + uuidPart[16:20] + "-" + uuidPart[20:32]
	} else {
		return uuid.Nil, fmt.Errorf("invalid format: UUID part must be 32 or 36 characters, got %d", len(uuidPart))
	}

	parsedUUID, err := uuid.Parse(uuidWithHyphens)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid uuid: %w", err)
	}

	return parsedUUID, nil
}

// Start begins the polling loop in a goroutine.
func (p *BatchPoller) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return errors.New("batch poller already running")
	}
	p.running = true
	p.mu.Unlock()

	slogger.Info(ctx, "Starting batch poller", slogger.Fields{
		"poll_interval":        p.pollInterval,
		"max_concurrent_polls": p.maxConcurrentPolls,
	})

	p.wg.Add(1)
	go p.pollLoop(ctx)

	return nil
}

// Stop gracefully stops the polling loop.
func (p *BatchPoller) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	p.mu.Unlock()

	close(p.stopCh)
	p.wg.Wait()

	slogger.InfoNoCtx("Batch poller stopped", nil)
}

// ProcessBatchNow processes a batch immediately, bypassing the polling loop.
// This is used by the recovery service to process batches that were found to be
// already completed during startup recovery.
func (p *BatchPoller) ProcessBatchNow(ctx context.Context, batch *entity.BatchJobProgress) error {
	// Just call the internal processBatch method
	return p.processBatch(ctx, batch)
}

// pollLoop runs the polling cycle.
func (p *BatchPoller) pollLoop(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	// Run once immediately
	if err := p.pollOnce(ctx); err != nil {
		slogger.Error(ctx, "Initial poll failed", slogger.Fields{"error": err.Error()})
	}

	for {
		select {
		case <-ticker.C:
			if err := p.pollOnce(ctx); err != nil {
				slogger.Error(ctx, "Poll cycle failed", slogger.Fields{"error": err.Error()})
			}
		case <-p.stopCh:
			slogger.Info(ctx, "Batch poller received stop signal", nil)
			return
		case <-ctx.Done():
			slogger.Info(ctx, "Batch poller context cancelled", nil)
			return
		}
	}
}

// pollOnce performs a single polling cycle.
func (p *BatchPoller) pollOnce(ctx context.Context) error {
	// Get all pending Gemini batches
	batches, err := p.batchProgressRepo.GetPendingGeminiBatches(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending gemini batches: %w", err)
	}

	if len(batches) == 0 {
		slogger.Debug(ctx, "No pending Gemini batches to poll", nil)
		return nil
	}

	slogger.Info(ctx, "Polling pending Gemini batches", slogger.Fields{
		"batch_count": len(batches),
	})

	// Process batches with concurrency limit
	sem := make(chan struct{}, p.maxConcurrentPolls)
	var wg sync.WaitGroup
	errorsCh := make(chan error, len(batches))

	for _, batch := range batches {
		// Check if we should stop
		select {
		case <-p.stopCh:
			return errors.New("poller stopped")
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Acquire semaphore
		sem <- struct{}{}
		wg.Add(1)

		go func(b *entity.BatchJobProgress) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := p.processBatch(ctx, b); err != nil {
				errorsCh <- err
			}
		}(batch)
	}

	// Wait for all batches to be processed
	wg.Wait()
	close(errorsCh)

	// Collect errors
	var errs []error
	for err := range errorsCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		slogger.Warn(ctx, "Some batch polls failed", slogger.Fields{
			"error_count": len(errs),
			"total_count": len(batches),
		})
	}

	return nil
}

// processBatch processes a single pending batch.
func (p *BatchPoller) processBatch(ctx context.Context, batch *entity.BatchJobProgress) error {
	jobID := batch.GeminiBatchJobID()
	if jobID == nil || *jobID == "" {
		slogger.Warn(ctx, "Batch has nil or empty Gemini job ID", slogger.Fields{
			"batch_id":     batch.ID(),
			"indexing_job": batch.IndexingJobID(),
		})
		return nil
	}

	// Validate Gemini batch job ID format (defense in depth)
	if len(*jobID) < 8 || (*jobID)[:8] != "batches/" {
		slogger.Error(ctx, "Invalid gemini_job_id format in database", slogger.Fields{
			"batch_id":      batch.ID(),
			"gemini_job_id": *jobID,
		})
		return fmt.Errorf("invalid gemini job ID format: %s", *jobID)
	}

	slogger.Debug(ctx, "Checking Gemini batch job status", slogger.Fields{
		"batch_id":        batch.ID(),
		"gemini_job_id":   *jobID,
		"indexing_job_id": batch.IndexingJobID(),
		"batch_number":    batch.BatchNumber(),
	})

	// Get job status from Gemini API using injected batch embedding service
	job, err := p.batchEmbeddingService.GetBatchJobStatus(ctx, *jobID)
	if err != nil {
		slogger.Error(ctx, "Failed to get batch job status", slogger.Fields{
			"batch_id":      batch.ID(),
			"gemini_job_id": *jobID,
			"error":         err.Error(),
		})
		// Don't fail permanently - might be transient error
		return nil
	}

	slogger.Debug(ctx, "Batch job status retrieved", slogger.Fields{
		"batch_id":      batch.ID(),
		"gemini_job_id": *jobID,
		"state":         job.State,
	})

	// Handle different job states
	switch job.State {
	case outbound.BatchJobStateCompleted:
		return p.handleCompletedBatch(ctx, batch, job)
	case outbound.BatchJobStateFailed:
		return p.handleFailedBatch(ctx, batch, job)
	case outbound.BatchJobStateProcessing, outbound.BatchJobStatePending:
		// Still processing - check again next poll
		slogger.Debug(ctx, "Batch job still processing", slogger.Fields{
			"batch_id": batch.ID(),
			"state":    job.State,
		})
		return nil
	default:
		slogger.Warn(ctx, "Unknown batch job state", slogger.Fields{
			"batch_id": batch.ID(),
			"state":    job.State,
		})
		return nil
	}
}

// handleCompletedBatch processes a completed batch job.
func (p *BatchPoller) handleCompletedBatch(
	ctx context.Context,
	batch *entity.BatchJobProgress,
	job *outbound.BatchEmbeddingJob,
) (returnErr error) {
	fields := slogger.Fields{
		"batch_id":        batch.ID(),
		"indexing_job_id": batch.IndexingJobID(),
		"batch_number":    batch.BatchNumber(),
	}
	if geminiJobID := batch.GeminiBatchJobID(); geminiJobID != nil {
		fields["gemini_job_id"] = *geminiJobID
	}
	slogger.Info(ctx, "Batch job completed, processing results", fields)

	// Process batch results with error tracking and automatic cleanup
	return p.processBatchWithErrorTracking(ctx, batch)
}

// processBatchWithErrorTracking processes batch results with comprehensive error tracking and cleanup.
func (p *BatchPoller) processBatchWithErrorTracking(ctx context.Context, batch *entity.BatchJobProgress) error {
	// Track processing outcome with defer statement to ensure database is always updated
	var processErr error
	var chunksProcessed int
	defer func() {
		if processErr != nil {
			// Failed - mark batch as failed
			errMsg := processErr.Error()
			batch.MarkFailed(errMsg)
			if saveErr := p.batchProgressRepo.Save(ctx, batch); saveErr != nil {
				processErr = fmt.Errorf("failed to save failed batch after processing error [%s]: %w", errMsg, saveErr)
			}
			// Log the failure with full context
			errorFields := slogger.Fields{
				"batch_id": batch.ID(),
				"error":    errMsg,
			}
			if geminiJobID := batch.GeminiBatchJobID(); geminiJobID != nil {
				errorFields["gemini_job_id"] = *geminiJobID
			}
			slogger.Error(ctx, "Batch processing failed", errorFields)
		} else {
			// Success - mark as completed
			slogger.Info(ctx, "Batch marked as completed", slogger.Fields{
				"batch_id":         batch.ID(),
				"chunks_processed": chunksProcessed,
			})
		}
	}()

	// Process the actual batch results
	processErr = p.processBatchResults(ctx, batch, &chunksProcessed)
	return processErr
}

// processBatchResults handles the core batch processing logic.
func (p *BatchPoller) processBatchResults(
	ctx context.Context,
	batch *entity.BatchJobProgress,
	chunksProcessed *int,
) error {
	// Step 1: Download and validate batch results
	results, err := p.downloadAndValidateBatchResults(ctx, batch)
	if err != nil {
		return err
	}

	// Step 2: Validate repository ID
	repositoryID, err := p.validateRepositoryID(batch)
	if err != nil {
		return err
	}

	// Step 3: Convert results to embeddings only (chunks already exist in DB)
	embeddings, err := p.convertResultsToEmbeddings(results, repositoryID)
	if err != nil {
		return err
	}

	// Step 4: Save embeddings (chunks were saved before batch submission)
	err = p.chunkRepo.SaveEmbeddings(ctx, embeddings)
	if err != nil {
		return fmt.Errorf("failed to save embeddings: %w", err)
	}

	// Step 5: Update batch progress
	*chunksProcessed = len(results)
	batch.MarkCompleted(*chunksProcessed)
	if err := p.batchProgressRepo.Save(ctx, batch); err != nil {
		return fmt.Errorf("failed to save completed batch: %w", err)
	}

	return nil
}

// downloadAndValidateBatchResults downloads batch results and validates embedding dimensions.
func (p *BatchPoller) downloadAndValidateBatchResults(
	ctx context.Context,
	batch *entity.BatchJobProgress,
) ([]*outbound.EmbeddingResult, error) {
	// Download batch results
	geminiJobID := batch.GeminiBatchJobID()
	if geminiJobID == nil {
		return nil, errors.New("batch has no Gemini job ID")
	}
	results, err := p.batchEmbeddingService.GetBatchJobResults(ctx, *geminiJobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch job results: %w", err)
	}

	// Validate embedding dimensions
	const expectedDimensions = 768
	for _, result := range results {
		if len(result.Vector) != expectedDimensions {
			return nil, fmt.Errorf(
				"invalid embedding dimensions: expected %d, got %d",
				expectedDimensions,
				len(result.Vector),
			)
		}
	}

	return results, nil
}

// validateRepositoryID extracts and validates the repository ID from batch.
func (p *BatchPoller) validateRepositoryID(batch *entity.BatchJobProgress) (*uuid.UUID, error) {
	repositoryID := batch.RepositoryID()
	if repositoryID == nil {
		return nil, errors.New("batch has no repository ID")
	}
	return repositoryID, nil
}

// convertResultsToChunksAndEmbeddings converts batch results to chunks and embeddings with proper repository ID assignment.
func (p *BatchPoller) convertResultsToChunksAndEmbeddings(
	results []*outbound.EmbeddingResult,
	repositoryID *uuid.UUID,
) ([]outbound.CodeChunk, []outbound.Embedding, error) {
	chunks := make([]outbound.CodeChunk, len(results))
	embeddings := make([]outbound.Embedding, len(results))

	for i, result := range results {
		// Extract chunk ID from request ID
		chunkID, err := decodeRequestIDToChunkID(result.RequestID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode request ID: %w", err)
		}

		// Create minimal chunk (repository will use ON CONFLICT DO NOTHING if exists)
		chunks[i] = outbound.CodeChunk{
			ID:           chunkID.String(),
			RepositoryID: *repositoryID, // Use repository ID from batch
		}

		// Create embedding
		embeddings[i] = outbound.Embedding{
			ID:           uuid.New(),
			ChunkID:      chunkID,
			RepositoryID: *repositoryID, // Use repository ID from batch
			Vector:       result.Vector,
			ModelVersion: result.Model,
			CreatedAt:    result.GeneratedAt.Format(time.RFC3339),
		}
	}

	return chunks, embeddings, nil
}

// convertResultsToEmbeddings converts batch results to embeddings only (chunks already exist in DB).
// This is used when chunks have been pre-saved before batch submission.
func (p *BatchPoller) convertResultsToEmbeddings(
	results []*outbound.EmbeddingResult,
	repositoryID *uuid.UUID,
) ([]outbound.Embedding, error) {
	embeddings := make([]outbound.Embedding, len(results))

	for i, result := range results {
		// Extract chunk ID from request ID
		chunkID, err := decodeRequestIDToChunkID(result.RequestID)
		if err != nil {
			return nil, fmt.Errorf("failed to decode request ID: %w", err)
		}

		// Create embedding (chunk already exists in database)
		embeddings[i] = outbound.Embedding{
			ID:           uuid.New(),
			ChunkID:      chunkID,
			RepositoryID: *repositoryID,
			Vector:       result.Vector,
			ModelVersion: result.Model,
			CreatedAt:    result.GeneratedAt.Format(time.RFC3339),
		}
	}

	return embeddings, nil
}

// handleFailedBatch processes a failed batch job.
func (p *BatchPoller) handleFailedBatch(
	ctx context.Context,
	batch *entity.BatchJobProgress,
	job *outbound.BatchEmbeddingJob,
) error {
	errorMsg := fmt.Sprintf("Gemini batch job failed: state=%s", job.State)
	if job.ErrorFileURI != "" {
		errorMsg = fmt.Sprintf("%s, error_file=%s", errorMsg, job.ErrorFileURI)
	}

	errorFields := slogger.Fields{
		"batch_id":    batch.ID(),
		"error":       errorMsg,
		"retry_count": batch.RetryCount(),
	}
	if geminiJobID := batch.GeminiBatchJobID(); geminiJobID != nil {
		errorFields["gemini_job_id"] = *geminiJobID
	}
	slogger.Error(ctx, "Batch job failed", errorFields)

	// Check if we should retry
	maxRetries := 3 // TODO: Make this configurable
	if batch.RetryCount() < maxRetries {
		// Schedule retry with exponential backoff
		backoff := time.Duration(1<<uint(batch.RetryCount())) * time.Minute
		retryAt := time.Now().Add(backoff)
		batch.ScheduleRetry(retryAt)

		slogger.Info(ctx, "Scheduling batch retry", slogger.Fields{
			"batch_id":    batch.ID(),
			"retry_at":    retryAt,
			"retry_count": batch.RetryCount(),
		})
	} else {
		// Max retries exceeded - mark as permanently failed
		batch.MarkFailed(errorMsg)

		slogger.Error(ctx, "Batch permanently failed after max retries", slogger.Fields{
			"batch_id":    batch.ID(),
			"retry_count": batch.RetryCount(),
			"max_retries": maxRetries,
		})
	}

	if err := p.batchProgressRepo.Save(ctx, batch); err != nil {
		return fmt.Errorf("failed to save failed batch: %w", err)
	}

	return nil
}
