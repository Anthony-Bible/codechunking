package worker

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/entity"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// BatchSubmitterConfig holds configuration for batch submission.
type BatchSubmitterConfig struct {
	PollInterval             time.Duration
	MaxConcurrentSubmissions int
	InitialBackoff           time.Duration
	MaxBackoff               time.Duration
	MaxSubmissionAttempts    int
}

// BatchSubmitter handles rate-limited submission of batches to the Gemini API.
type BatchSubmitter struct {
	batchProgressRepo     outbound.BatchProgressRepository
	batchEmbeddingService outbound.BatchEmbeddingService
	config                BatchSubmitterConfig

	stopCh  chan struct{}
	wg      sync.WaitGroup
	mu      sync.Mutex
	running bool

	// Global backoff state
	globalBackoffUntil time.Time
	globalBackoffMu    sync.RWMutex

	// Semaphore for concurrency control
	semaphore chan struct{}
}

// NewBatchSubmitter creates a new batch submitter with default values applied.
func NewBatchSubmitter(
	batchProgressRepo outbound.BatchProgressRepository,
	batchEmbeddingService outbound.BatchEmbeddingService,
	config BatchSubmitterConfig,
) *BatchSubmitter {
	// Apply defaults
	if config.PollInterval == 0 {
		config.PollInterval = 5 * time.Second
	}
	if config.MaxConcurrentSubmissions == 0 {
		config.MaxConcurrentSubmissions = 1
	}
	if config.InitialBackoff == 0 {
		config.InitialBackoff = 1 * time.Minute
	}
	if config.MaxBackoff == 0 {
		config.MaxBackoff = 30 * time.Minute
	}
	if config.MaxSubmissionAttempts == 0 {
		config.MaxSubmissionAttempts = 10
	}

	return &BatchSubmitter{
		batchProgressRepo:     batchProgressRepo,
		batchEmbeddingService: batchEmbeddingService,
		config:                config,
		stopCh:                make(chan struct{}),
		semaphore:             make(chan struct{}, config.MaxConcurrentSubmissions),
	}
}

// Start begins the batch submission loop.
func (s *BatchSubmitter) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("batch submitter is already running")
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.submissionLoop(ctx)
	}()

	return nil
}

// Stop gracefully stops the batch submitter.
func (s *BatchSubmitter) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()
}

// submissionLoop is the main polling loop for batch submission.
func (s *BatchSubmitter) submissionLoop(ctx context.Context) {
	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			// Check if in global backoff
			if s.isInGlobalBackoff() {
				continue
			}

			// submitOneBatch handles its own concurrency control
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				_ = s.submitOneBatch(ctx)
			}()
		}
	}
}

// submitOneBatch submits a single batch to the Gemini API.
func (s *BatchSubmitter) submitOneBatch(ctx context.Context) error {
	// Check global backoff
	if s.isInGlobalBackoff() {
		return nil
	}

	// Acquire semaphore for concurrency control
	select {
	case s.semaphore <- struct{}{}:
		defer func() { <-s.semaphore }()
	case <-ctx.Done():
		return nil
	}

	// Get pending batch
	batch, err := s.batchProgressRepo.GetPendingSubmissionBatch(ctx)
	if err != nil {
		slogger.Error(ctx, "Failed to get pending submission batch", slogger.Fields{
			"error": err.Error(),
		})
		return nil
	}

	if batch == nil {
		return nil
	}

	// Deserialize request data
	var requests []*outbound.BatchEmbeddingRequest
	if err := json.Unmarshal(batch.BatchRequestData(), &requests); err != nil {
		slogger.Error(ctx, "Failed to deserialize batch request data", slogger.Fields{
			"error":    err.Error(),
			"batch_id": batch.ID().String(),
		})

		// Mark batch as failed
		errorMsg := fmt.Sprintf("JSON deserialization failed: %v", err)
		batch.MarkFailed(errorMsg)

		if saveErr := s.batchProgressRepo.Save(ctx, batch); saveErr != nil {
			slogger.Error(ctx, "Failed to save failed batch", slogger.Fields{
				"error":    saveErr.Error(),
				"batch_id": batch.ID().String(),
			})
		}

		return nil
	}

	// Submit to API
	options := outbound.EmbeddingOptions{
		Model:          "gemini-embedding-001",
		Dimensionality: 768,
	}

	var job *outbound.BatchEmbeddingJob

	// Check if file was already uploaded (for retry scenarios)
	if !batch.HasUploadedFile() {
		// First time - upload file and create batch job in one step
		slogger.Info(ctx, "First submission attempt - uploading file and creating batch job", slogger.Fields{
			"batch_id": batch.ID().String(),
		})

		var createErr error
		job, createErr = s.batchEmbeddingService.CreateBatchEmbeddingJobWithRequests(
			ctx,
			requests,
			options,
			batch.ID(),
		)
		if createErr != nil {
			return s.handleSubmissionError(ctx, batch, createErr)
		}

		// Save the file URI to the database for potential retry scenarios
		// The uploadBatchFile method handles ALREADY_EXISTS internally, and we persist
		// the file URI here so that if batch job creation fails, we can skip re-upload on retry
		if job.InputFileURI != "" {
			batch.SetGeminiFileURI(job.InputFileURI)
			slogger.Info(ctx, "File uploaded successfully, saving file URI", slogger.Fields{
				"batch_id": batch.ID().String(),
				"file_uri": job.InputFileURI,
			})

			// Save the batch with the file URI before proceeding with job creation status update
			// This ensures that even if the next steps fail, we have the file URI persisted
			if saveErr := s.batchProgressRepo.Save(ctx, batch); saveErr != nil {
				slogger.Error(ctx, "Failed to save batch after file upload", slogger.Fields{
					"error":    saveErr.Error(),
					"batch_id": batch.ID().String(),
					"file_uri": job.InputFileURI,
				})
				// Don't fail the whole operation, continue with batch job tracking
			}
		}
	} else {
		// Retry scenario - file was already uploaded, just create the batch job
		slogger.Info(ctx, "Retry attempt - using existing uploaded file", slogger.Fields{
			"batch_id": batch.ID().String(),
			"file_uri": *batch.GeminiFileURI(),
		})

		var createErr error
		job, createErr = s.batchEmbeddingService.CreateBatchEmbeddingJobWithFile(
			ctx,
			requests,
			options,
			batch.ID(),
			*batch.GeminiFileURI(),
		)
		if createErr != nil {
			return s.handleSubmissionError(ctx, batch, createErr)
		}
	}

	// Update batch status with validated Gemini job ID
	if err := batch.MarkSubmittedToGemini(job.JobID); err != nil {
		slogger.Error(ctx, "Failed to mark batch as submitted - invalid job ID", slogger.Fields{
			"error":    err.Error(),
			"batch_id": batch.ID().String(),
			"job_id":   job.JobID,
		})
		// Mark as failed since we can't track this batch without a valid job ID
		errorMsg := fmt.Sprintf("invalid Gemini batch job ID: %v", err)
		batch.MarkFailed(errorMsg)
		if saveErr := s.batchProgressRepo.Save(ctx, batch); saveErr != nil {
			slogger.Error(ctx, "Failed to save batch after invalid job ID", slogger.Fields{
				"error":    saveErr.Error(),
				"batch_id": batch.ID().String(),
			})
		}
		return nil
	}

	if saveErr := s.batchProgressRepo.Save(ctx, batch); saveErr != nil {
		slogger.Error(ctx, "Failed to save batch after successful submission", slogger.Fields{
			"error":               saveErr.Error(),
			"batch_id":            batch.ID().String(),
			"status":              batch.Status(),
			"gemini_batch_job_id": job.JobID,
		})
		return nil
	}

	return nil
}

// handleSubmissionError handles errors during batch submission.
func (s *BatchSubmitter) handleSubmissionError(ctx context.Context, batch *entity.BatchJobProgress, err error) error {
	retryable := isRateLimitError(err)

	// If rate limit error, set global backoff
	if retryable {
		backoffDuration := calculateBackoff(s.config, batch.SubmissionAttempts())
		backoffUntil := time.Now().Add(backoffDuration)
		s.setGlobalBackoff(backoffUntil)

		slogger.Error(ctx, "Rate limit error, setting global backoff", slogger.Fields{
			"error":            err.Error(),
			"batch_id":         batch.ID().String(),
			"backoff_until":    backoffUntil.Format(time.RFC3339),
			"backoff_duration": backoffDuration.String(),
		})
	}

	s.handleSubmissionFailure(ctx, batch, err, retryable)
	return nil
}

// handleSubmissionFailure updates batch state after submission failure.
func (s *BatchSubmitter) handleSubmissionFailure(
	ctx context.Context,
	batch *entity.BatchJobProgress,
	err error,
	retryable bool,
) {
	// This submission attempt failed, so we count it
	attemptNumber := batch.SubmissionAttempts() + 1

	// Check if max attempts exceeded
	if attemptNumber >= s.config.MaxSubmissionAttempts {
		errorMsg := fmt.Sprintf("max attempts (%d) exceeded: %v", s.config.MaxSubmissionAttempts, err)
		// Increment submission attempts counter, then mark as permanently failed
		// Note: MarkSubmissionFailed updates submission_attempts and error_message but keeps status as pending_submission
		// Then MarkFailed changes the status to failed, making this a permanent failure
		batch.MarkSubmissionFailed(errorMsg, time.Now())
		batch.MarkFailed(errorMsg)

		slogger.Error(ctx, "Batch permanently failed after max attempts", slogger.Fields{
			"error":    err.Error(),
			"batch_id": batch.ID().String(),
			"attempts": batch.SubmissionAttempts(),
		})
	} else if retryable {
		// Schedule retry with exponential backoff
		backoffDuration := calculateBackoff(s.config, batch.SubmissionAttempts())
		nextSubmissionAt := time.Now().Add(backoffDuration)

		errorMsg := fmt.Sprintf("Submission failed: %v", err)
		batch.MarkSubmissionFailed(errorMsg, nextSubmissionAt)

		slogger.Error(ctx, "Batch submission failed, scheduling retry", slogger.Fields{
			"error":              err.Error(),
			"batch_id":           batch.ID().String(),
			"attempts":           batch.SubmissionAttempts(),
			"next_submission_at": nextSubmissionAt.Format(time.RFC3339),
		})
	} else {
		// Non-retryable error, mark as failed
		errorMsg := fmt.Sprintf("Non-retryable error: %v", err)
		batch.MarkFailed(errorMsg)

		slogger.Error(ctx, "Batch permanently failed due to non-retryable error", slogger.Fields{
			"error":    err.Error(),
			"batch_id": batch.ID().String(),
		})
	}

	if saveErr := s.batchProgressRepo.Save(ctx, batch); saveErr != nil {
		slogger.Error(ctx, "Failed to save batch after submission failure", slogger.Fields{
			"error":    saveErr.Error(),
			"batch_id": batch.ID().String(),
		})
	}
}

// calculateBackoff calculates exponential backoff duration.
func calculateBackoff(config BatchSubmitterConfig, attempts int) time.Duration {
	// Calculate 2^attempts * InitialBackoff
	backoff := config.InitialBackoff
	for range attempts {
		backoff *= 2
		if backoff >= config.MaxBackoff {
			return config.MaxBackoff
		}
	}
	return backoff
}

// isRateLimitError checks if an error indicates rate limiting.
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's an EmbeddingError with quota issue
	embErr := &outbound.EmbeddingError{}
	if errors.As(err, &embErr) {
		if embErr.IsQuotaError() {
			return true
		}
	}

	// Check error message for rate limit indicators (case-insensitive)
	errMsg := strings.ToLower(err.Error())
	indicators := []string{"quota", "rate limit", "429", "resource exhausted"}

	for _, indicator := range indicators {
		if strings.Contains(errMsg, indicator) {
			return true
		}
	}

	return false
}

// setGlobalBackoff sets the global backoff time.
func (s *BatchSubmitter) setGlobalBackoff(until time.Time) {
	s.globalBackoffMu.Lock()
	defer s.globalBackoffMu.Unlock()
	s.globalBackoffUntil = until
}

// isInGlobalBackoff checks if currently in global backoff period.
func (s *BatchSubmitter) isInGlobalBackoff() bool {
	s.globalBackoffMu.RLock()
	defer s.globalBackoffMu.RUnlock()
	return time.Now().Before(s.globalBackoffUntil)
}

// getGlobalBackoffUntil returns the global backoff time.
func (s *BatchSubmitter) getGlobalBackoffUntil() time.Time {
	s.globalBackoffMu.RLock()
	defer s.globalBackoffMu.RUnlock()
	return s.globalBackoffUntil
}
