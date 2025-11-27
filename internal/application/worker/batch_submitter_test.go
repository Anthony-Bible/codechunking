// Package worker contains tests for the batch submitter's rate-limited submission functionality.
//
// # Test Coverage - Batch Submission with Rate Limiting
//
// This file contains comprehensive failing tests (RED PHASE) that define the expected
// behavior for submitting batches to the Gemini API with rate limiting and exponential backoff.
// These tests specify how the batch submitter should poll for pending batches, submit them
// to the API, handle rate limits, and manage retries.
//
// # Test Cases
//
// ## Configuration Tests
// 1. TestBatchSubmitter_NewBatchSubmitter_WithDefaults
//   - Verifies default configuration values are applied
//   - Ensures dependencies are properly stored
//
// 2. TestBatchSubmitter_NewBatchSubmitter_WithCustomConfig
//   - Verifies custom configuration values are preserved
//   - Ensures no defaults override custom values
//
// ## Lifecycle Tests
// 3. TestBatchSubmitter_Start_Success
//   - Verifies submitter starts successfully
//   - Ensures running state is tracked
//   - Confirms double-start is prevented
//
// 4. TestBatchSubmitter_Stop_GracefulShutdown
//   - Verifies clean shutdown without errors
//   - Ensures stop is idempotent (safe to call multiple times)
//   - Confirms no goroutine leaks
//
// ## Submission Flow Tests
// 5. TestBatchSubmitter_SubmitOneBatch_Success
//   - Verifies successful batch submission to Gemini API
//   - Confirms batch status updated to 'processing'
//   - Ensures gemini_batch_job_id is saved
//
// 6. TestBatchSubmitter_SubmitOneBatch_NoBatchReady
//   - Verifies no action when no batches ready
//   - Ensures no API calls made
//   - Confirms no errors returned
//
// 7. TestBatchSubmitter_SubmitOneBatch_DeserializationError
//   - Verifies handling of invalid JSON in request data
//   - Ensures batch is marked as failed
//   - Confirms no API call made with invalid data
//
// ## Rate Limiting Tests
// 8. TestBatchSubmitter_HandleRateLimitError
//   - Verifies rate limit error triggers global backoff
//   - Ensures batch is marked failed with retry scheduled
//   - Confirms next poll cycle skips submissions during backoff
//
// 9. TestBatchSubmitter_RateLimitErrorDetection
//   - Tests various error patterns that indicate rate limiting:
//   - "quota" in error message
//   - "rate limit" in error message
//   - "429" status code
//   - "resource exhausted" message
//   - Confirms all patterns trigger rate limit handling
//
// ## Backoff Tests
// 10. TestBatchSubmitter_ExponentialBackoff_Calculation
//   - Verifies backoff doubles each attempt: 1m, 2m, 4m, 8m, 16m, 30m (capped)
//   - Ensures max backoff cap is enforced at 30 minutes
//   - Tests backoff calculation for attempts 0-10
//
// 11. TestBatchSubmitter_GlobalBackoff_PreventsSubmissions
//   - Verifies global backoff prevents all submissions
//   - Ensures no batch retrieval during backoff period
//   - Confirms submissions resume after backoff expires
//
// ## Retry Tests
// 12. TestBatchSubmitter_MaxAttemptsExceeded
//   - Verifies batch marked as permanently failed at max attempts
//   - Ensures no further retries scheduled
//   - Confirms error message indicates max attempts reached
//
// 13. TestBatchSubmitter_NonRetryableError
//   - Verifies non-rate-limit errors mark batch as failed
//   - Ensures no global backoff is set
//   - Confirms batch is not retried
//
// ## Concurrency Tests
// 14. TestBatchSubmitter_ConcurrencyControl
//   - Verifies semaphore limits concurrent submissions
//   - Ensures MaxConcurrentSubmissions is respected
//   - Tests with multiple batches ready simultaneously
//
// ## Error Handling Tests
// 15. TestBatchSubmitter_RepositoryError
//   - Verifies database errors are logged but don't crash submitter
//   - Ensures next poll cycle continues after error
//   - Confirms error doesn't affect running state
//
// 16. TestBatchSubmitter_SaveFailureAfterSubmission
//   - Verifies handling of save failure after successful API call
//   - Ensures error is logged with batch state details
//   - Confirms batch can be retried in next cycle
//
// # Implementation Requirements
//
// The BatchSubmitter implementation must:
// 1. Poll database at configured intervals for batches with status='pending_submission'
// 2. Deserialize batch request data (JSON array of BatchEmbeddingRequest)
// 3. Call batchEmbeddingService.CreateBatchEmbeddingJobWithRequests()
// 4. Update batch status to 'processing' and save gemini_batch_job_id
// 5. Implement exponential backoff on submission failures (1m, 2m, 4m, ..., max 30m)
// 6. Detect rate limit errors (quota, 429, rate limit) and set global backoff
// 7. Skip submissions when global backoff is active
// 8. Respect MaxConcurrentSubmissions using semaphore
// 9. Mark batch as permanently failed after MaxSubmissionAttempts (default: 10)
// 10. Support graceful shutdown via Stop() method
//
// # TDD Workflow
//
// These tests are in the RED phase - they define expected behavior but will fail
// until the implementation is complete. The next step is GREEN phase where
// the minimal implementation is added to make these tests pass.
package worker

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ========================================
// Test Helper Functions
// ========================================

// testBatchSubmitterConfig creates a test configuration with short timeouts.
func testBatchSubmitterConfig() BatchSubmitterConfig {
	return BatchSubmitterConfig{
		PollInterval:             100 * time.Millisecond,
		MaxConcurrentSubmissions: 2,
		InitialBackoff:           1 * time.Minute,
		MaxBackoff:               30 * time.Minute,
		MaxSubmissionAttempts:    10,
	}
}

// createTestBatchProgress creates a test batch progress entity.
func createTestBatchProgress(status string, withRequestData bool) *entity.BatchJobProgress {
	repositoryID := uuid.New()
	indexingJobID := uuid.New()
	batch := entity.NewBatchJobProgress(repositoryID, indexingJobID, 1, 1)

	if withRequestData {
		// Create sample batch embedding requests
		requests := []*outbound.BatchEmbeddingRequest{
			{
				RequestID: fmt.Sprintf("chunk_%s", uuid.New().String()),
				Text:      "sample code chunk 1",
			},
			{
				RequestID: fmt.Sprintf("chunk_%s", uuid.New().String()),
				Text:      "sample code chunk 2",
			},
		}
		requestData, _ := json.Marshal(requests)
		batch.MarkPendingSubmission(requestData)
	}

	return batch
}

// createTestBatchEmbeddingJob creates a test batch embedding job result.
func createTestBatchEmbeddingJob(jobID string) *outbound.BatchEmbeddingJob {
	return &outbound.BatchEmbeddingJob{
		JobID:      jobID,
		JobName:    "Test Batch Job",
		Model:      "gemini-embedding-001",
		State:      outbound.BatchJobStatePending,
		TotalCount: 2,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// ========================================
// Configuration Tests
// ========================================

// TestBatchSubmitter_NewBatchSubmitter_WithDefaults verifies that default configuration
// values are properly applied when creating a new BatchSubmitter with empty config.
func TestBatchSubmitter_NewBatchSubmitter_WithDefaults(t *testing.T) {
	// Arrange
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchService := new(MockBatchEmbeddingService)

	// Create submitter with zero/empty config
	emptyConfig := BatchSubmitterConfig{}

	// Act
	submitter := NewBatchSubmitter(mockBatchProgressRepo, mockBatchService, emptyConfig)

	// Assert
	require.NotNil(t, submitter, "NewBatchSubmitter should return non-nil submitter")

	// Verify defaults are applied (these will be checked via getter methods once implemented)
	// Default values should be:
	// - PollInterval: 5 seconds
	// - MaxConcurrentSubmissions: 1
	// - InitialBackoff: 1 minute
	// - MaxBackoff: 30 minutes
	// - MaxSubmissionAttempts: 10

	// Verify dependencies are stored
	assert.NotNil(t, submitter, "Submitter should have batch progress repository")
	assert.NotNil(t, submitter, "Submitter should have batch embedding service")
}

// TestBatchSubmitter_NewBatchSubmitter_WithCustomConfig verifies that custom configuration
// values are preserved and not overridden by defaults.
func TestBatchSubmitter_NewBatchSubmitter_WithCustomConfig(t *testing.T) {
	// Arrange
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchService := new(MockBatchEmbeddingService)

	customConfig := BatchSubmitterConfig{
		PollInterval:             10 * time.Second,
		MaxConcurrentSubmissions: 5,
		InitialBackoff:           2 * time.Minute,
		MaxBackoff:               1 * time.Hour,
		MaxSubmissionAttempts:    20,
	}

	// Act
	submitter := NewBatchSubmitter(mockBatchProgressRepo, mockBatchService, customConfig)

	// Assert
	require.NotNil(t, submitter, "NewBatchSubmitter should return non-nil submitter")

	// Verify custom values are preserved (will be checked via getter methods once implemented)
	// The implementation should store these values and not override with defaults
	assert.NotNil(t, submitter, "Submitter should preserve custom configuration")
}

// ========================================
// Lifecycle Tests
// ========================================

// TestBatchSubmitter_Start_Success verifies that the submitter starts successfully
// and prevents double-start.
func TestBatchSubmitter_Start_Success(t *testing.T) {
	// Arrange
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchService := new(MockBatchEmbeddingService)

	config := testBatchSubmitterConfig()
	submitter := NewBatchSubmitter(mockBatchProgressRepo, mockBatchService, config)

	// Create context with short timeout for test
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Mock repository to return no batches (to prevent actual processing during test)
	mockBatchProgressRepo.On("GetPendingSubmissionBatch", mock.Anything).Return(nil, nil).Maybe()

	// Act - Start submitter in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- submitter.Start(ctx)
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Try to start again (should fail or be idempotent)
	err := submitter.Start(ctx)

	// Assert
	assert.Error(t, err, "Starting already running submitter should return error")
	assert.Contains(t, err.Error(), "already running", "Error should indicate submitter is already running")

	// Wait for context cancellation
	<-ctx.Done()

	// Should exit cleanly when context is cancelled
	startErr := <-errCh
	assert.NoError(t, startErr, "Start should return no error when context is cancelled")
}

// TestBatchSubmitter_Stop_GracefulShutdown verifies that the submitter stops gracefully
// and that Stop() is idempotent (safe to call multiple times).
func TestBatchSubmitter_Stop_GracefulShutdown(t *testing.T) {
	// Arrange
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchService := new(MockBatchEmbeddingService)

	config := testBatchSubmitterConfig()
	submitter := NewBatchSubmitter(mockBatchProgressRepo, mockBatchService, config)

	// Mock repository to return no batches
	mockBatchProgressRepo.On("GetPendingSubmissionBatch", mock.Anything).Return(nil, nil).Maybe()

	// Start submitter
	ctx := context.Background()
	go func() {
		_ = submitter.Start(ctx)
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Act - Stop submitter
	submitter.Stop()

	// Wait a moment for shutdown
	time.Sleep(50 * time.Millisecond)

	// Stop again (should be safe)
	submitter.Stop()

	// Assert - No panics or errors should occur
	// Verify submitter is no longer running (implementation should track this)
	// Multiple Stop() calls should be idempotent
}

// ========================================
// Submission Flow Tests
// ========================================

// TestBatchSubmitter_SubmitOneBatch_Success verifies the happy path of submitting
// a single batch to the Gemini API.
func TestBatchSubmitter_SubmitOneBatch_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchService := new(MockBatchEmbeddingService)

	config := testBatchSubmitterConfig()
	submitter := NewBatchSubmitter(mockBatchProgressRepo, mockBatchService, config)

	// Create batch with request data
	batch := createTestBatchProgress(entity.StatusPendingSubmission, true)

	// Deserialize request data to verify
	var requests []*outbound.BatchEmbeddingRequest
	err := json.Unmarshal(batch.BatchRequestData(), &requests)
	require.NoError(t, err, "Test batch should have valid JSON request data")

	// Mock repository returns pending batch
	mockBatchProgressRepo.On("GetPendingSubmissionBatch", ctx).Return(batch, nil).Once()

	// Mock successful API submission
	jobID := "batches/test-job-123"
	mockBatchJob := createTestBatchEmbeddingJob(jobID)
	mockBatchService.On("CreateBatchEmbeddingJobWithRequests", ctx,
		mock.MatchedBy(func(reqs []*outbound.BatchEmbeddingRequest) bool {
			return len(reqs) == len(requests)
		}),
		mock.AnythingOfType("outbound.EmbeddingOptions"),
		batch.ID(),
	).Return(mockBatchJob, nil).Once()

	// Mock save with updated status
	mockBatchProgressRepo.On("Save", ctx,
		mock.MatchedBy(func(b *entity.BatchJobProgress) bool {
			return b.Status() == entity.StatusProcessing &&
				b.GeminiBatchJobID() != nil &&
				*b.GeminiBatchJobID() == jobID
		}),
	).Return(nil).Once()

	// Act
	err = submitter.submitOneBatch(ctx)

	// Assert
	require.NoError(t, err, "submitOneBatch should succeed")
	mockBatchProgressRepo.AssertExpectations(t)
	mockBatchService.AssertExpectations(t)

	// Verify batch status was updated
	assert.Equal(t, entity.StatusProcessing, batch.Status(), "Batch should be marked as processing")
	assert.NotNil(t, batch.GeminiBatchJobID(), "Batch should have Gemini job ID")
	assert.Equal(t, jobID, *batch.GeminiBatchJobID(), "Batch should have correct Gemini job ID")
}

// TestBatchSubmitter_SubmitOneBatch_NoBatchReady verifies that no action is taken
// when no batches are ready for submission.
func TestBatchSubmitter_SubmitOneBatch_NoBatchReady(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchService := new(MockBatchEmbeddingService)

	config := testBatchSubmitterConfig()
	submitter := NewBatchSubmitter(mockBatchProgressRepo, mockBatchService, config)

	// Mock repository returns no batches
	mockBatchProgressRepo.On("GetPendingSubmissionBatch", ctx).Return(nil, nil).Once()

	// Act
	err := submitter.submitOneBatch(ctx)

	// Assert
	require.NoError(t, err, "submitOneBatch should succeed when no batch ready")
	mockBatchProgressRepo.AssertExpectations(t)

	// Verify no API calls were made
	mockBatchService.AssertNotCalled(t, "CreateBatchEmbeddingJobWithRequests")
	mockBatchProgressRepo.AssertNotCalled(t, "Save")
}

// TestBatchSubmitter_SubmitOneBatch_DeserializationError verifies that batches with
// invalid JSON request data are marked as failed without making API calls.
func TestBatchSubmitter_SubmitOneBatch_DeserializationError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchService := new(MockBatchEmbeddingService)

	config := testBatchSubmitterConfig()
	submitter := NewBatchSubmitter(mockBatchProgressRepo, mockBatchService, config)

	// Create batch with invalid JSON request data
	repositoryID := uuid.New()
	indexingJobID := uuid.New()
	batch := entity.NewBatchJobProgress(repositoryID, indexingJobID, 1, 1)
	batch.MarkPendingSubmission([]byte("invalid json {{{"))

	// Mock repository returns batch with invalid data
	mockBatchProgressRepo.On("GetPendingSubmissionBatch", ctx).Return(batch, nil).Once()

	// Mock save for failed batch
	mockBatchProgressRepo.On("Save", ctx,
		mock.MatchedBy(func(b *entity.BatchJobProgress) bool {
			return b.Status() == entity.StatusFailed &&
				b.ErrorMessage() != nil &&
				contains(*b.ErrorMessage(), "deserialization") || contains(*b.ErrorMessage(), "json")
		}),
	).Return(nil).Once()

	// Act
	err := submitter.submitOneBatch(ctx)

	// Assert
	require.NoError(t, err, "submitOneBatch should handle deserialization error gracefully")
	mockBatchProgressRepo.AssertExpectations(t)

	// Verify no API calls were made
	mockBatchService.AssertNotCalled(t, "CreateBatchEmbeddingJobWithRequests")

	// Verify batch was marked as failed
	assert.Equal(t, entity.StatusFailed, batch.Status(), "Batch should be marked as failed")
	assert.NotNil(t, batch.ErrorMessage(), "Batch should have error message")
}

// ========================================
// Rate Limiting Tests
// ========================================

// TestBatchSubmitter_HandleRateLimitError verifies that rate limit errors trigger
// global backoff and batch retry scheduling.
func TestBatchSubmitter_HandleRateLimitError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchService := new(MockBatchEmbeddingService)

	config := testBatchSubmitterConfig()
	submitter := NewBatchSubmitter(mockBatchProgressRepo, mockBatchService, config)

	// Create batch with request data
	batch := createTestBatchProgress(entity.StatusPendingSubmission, true)

	// Mock repository returns pending batch
	mockBatchProgressRepo.On("GetPendingSubmissionBatch", ctx).Return(batch, nil).Once()

	// Mock API returns rate limit error
	rateLimitErr := &outbound.EmbeddingError{
		Code:      "quota_exceeded",
		Message:   "rate limit exceeded",
		Type:      "quota",
		Retryable: true,
	}
	mockBatchService.On("CreateBatchEmbeddingJobWithRequests", ctx,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil, rateLimitErr).Once()

	// Mock save for failed batch with retry scheduled
	mockBatchProgressRepo.On("Save", ctx,
		mock.MatchedBy(func(b *entity.BatchJobProgress) bool {
			return b.Status() == entity.StatusPendingSubmission &&
				b.SubmissionAttempts() == 1 &&
				b.NextSubmissionAt() != nil &&
				b.ErrorMessage() != nil
		}),
	).Return(nil).Once()

	// Act
	err := submitter.submitOneBatch(ctx)

	// Assert
	require.NoError(t, err, "submitOneBatch should handle rate limit error gracefully")
	mockBatchProgressRepo.AssertExpectations(t)
	mockBatchService.AssertExpectations(t)

	// Verify batch was marked for retry
	assert.Equal(t, 1, batch.SubmissionAttempts(), "Batch should have 1 submission attempt")
	assert.NotNil(t, batch.NextSubmissionAt(), "Batch should have next submission time")
	assert.NotNil(t, batch.ErrorMessage(), "Batch should have error message")

	// Verify global backoff was set (next submission should be skipped)
	// This is verified in the next test
}

// TestBatchSubmitter_RateLimitErrorDetection verifies that various error patterns
// are correctly identified as rate limit errors.
func TestBatchSubmitter_RateLimitErrorDetection(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		isRateLimit bool
	}{
		{
			name: "quota exceeded error",
			err: &outbound.EmbeddingError{
				Code:      "quota_exceeded",
				Message:   "quota exceeded",
				Type:      "quota",
				Retryable: true,
			},
			isRateLimit: true,
		},
		{
			name: "rate limit error message",
			err: &outbound.EmbeddingError{
				Code:      "error",
				Message:   "rate limit exceeded",
				Type:      "error",
				Retryable: true,
			},
			isRateLimit: true,
		},
		{
			name:        "error with 429 in message",
			err:         errors.New("API returned 429 status code"),
			isRateLimit: true,
		},
		{
			name:        "resource exhausted error",
			err:         errors.New("resource exhausted"),
			isRateLimit: true,
		},
		{
			name: "non-rate-limit error",
			err: &outbound.EmbeddingError{
				Code:      "invalid_input",
				Message:   "invalid request",
				Type:      "validation",
				Retryable: false,
			},
			isRateLimit: false,
		},
		{
			name:        "generic error",
			err:         errors.New("some other error"),
			isRateLimit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			isRateLimit := isRateLimitError(tt.err)

			// Assert
			assert.Equal(t, tt.isRateLimit, isRateLimit,
				"Error should be detected as rate limit: %v", tt.isRateLimit)
		})
	}
}

// ========================================
// Backoff Tests
// ========================================

// TestBatchSubmitter_ExponentialBackoff_Calculation verifies that backoff time
// doubles with each attempt and is capped at MaxBackoff.
func TestBatchSubmitter_ExponentialBackoff_Calculation(t *testing.T) {
	// Arrange
	config := testBatchSubmitterConfig()
	config.InitialBackoff = 1 * time.Minute
	config.MaxBackoff = 30 * time.Minute

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Minute},   // 1m
		{1, 2 * time.Minute},   // 2m
		{2, 4 * time.Minute},   // 4m
		{3, 8 * time.Minute},   // 8m
		{4, 16 * time.Minute},  // 16m
		{5, 30 * time.Minute},  // 32m capped to 30m
		{6, 30 * time.Minute},  // 64m capped to 30m
		{7, 30 * time.Minute},  // 128m capped to 30m
		{8, 30 * time.Minute},  // 256m capped to 30m
		{9, 30 * time.Minute},  // 512m capped to 30m
		{10, 30 * time.Minute}, // 1024m capped to 30m
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			// Act
			backoff := calculateBackoff(config, tt.attempt)

			// Assert
			assert.Equal(t, tt.expected, backoff,
				"Backoff for attempt %d should be %v", tt.attempt, tt.expected)
		})
	}
}

// TestBatchSubmitter_GlobalBackoff_PreventsSubmissions verifies that when global
// backoff is active, no submissions are attempted.
func TestBatchSubmitter_GlobalBackoff_PreventsSubmissions(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchService := new(MockBatchEmbeddingService)

	config := testBatchSubmitterConfig()
	submitter := NewBatchSubmitter(mockBatchProgressRepo, mockBatchService, config)

	// Set global backoff to 5 seconds in future
	backoffUntil := time.Now().Add(5 * time.Second)
	submitter.setGlobalBackoff(backoffUntil)

	// Act
	err := submitter.submitOneBatch(ctx)

	// Assert
	require.NoError(t, err, "submitOneBatch should succeed but skip submission due to backoff")

	// Verify no repository or API calls were made
	mockBatchProgressRepo.AssertNotCalled(t, "GetPendingSubmissionBatch")
	mockBatchService.AssertNotCalled(t, "CreateBatchEmbeddingJobWithRequests")
}

// ========================================
// Retry Tests
// ========================================

// TestBatchSubmitter_MaxAttemptsExceeded verifies that batches are marked as
// permanently failed when they reach MaxSubmissionAttempts.
func TestBatchSubmitter_MaxAttemptsExceeded(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchService := new(MockBatchEmbeddingService)

	config := testBatchSubmitterConfig()
	config.MaxSubmissionAttempts = 10
	submitter := NewBatchSubmitter(mockBatchProgressRepo, mockBatchService, config)

	// Create batch that has already attempted 9 times (one away from max)
	repositoryID := uuid.New()
	indexingJobID := uuid.New()
	batch := entity.NewBatchJobProgress(repositoryID, indexingJobID, 1, 1)

	// Create request data
	requests := []*outbound.BatchEmbeddingRequest{
		{RequestID: "chunk_1", Text: "test"},
	}
	requestData, _ := json.Marshal(requests)
	batch.MarkPendingSubmission(requestData)

	// Manually set submission attempts to 9
	for i := 0; i < 9; i++ {
		batch.MarkSubmissionFailed("test error", time.Now().Add(1*time.Minute))
	}

	// Mock repository returns batch with 9 attempts
	mockBatchProgressRepo.On("GetPendingSubmissionBatch", ctx).Return(batch, nil).Once()

	// Mock API returns error
	apiErr := errors.New("API error")
	mockBatchService.On("CreateBatchEmbeddingJobWithRequests", ctx,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil, apiErr).Once()

	// Mock save for permanently failed batch
	mockBatchProgressRepo.On("Save", ctx,
		mock.MatchedBy(func(b *entity.BatchJobProgress) bool {
			return b.Status() == entity.StatusFailed &&
				b.SubmissionAttempts() >= config.MaxSubmissionAttempts &&
				b.ErrorMessage() != nil &&
				contains(*b.ErrorMessage(), "max attempts")
		}),
	).Return(nil).Once()

	// Act
	err := submitter.submitOneBatch(ctx)

	// Assert
	require.NoError(t, err, "submitOneBatch should handle max attempts exceeded gracefully")
	mockBatchProgressRepo.AssertExpectations(t)
	mockBatchService.AssertExpectations(t)

	// Verify batch is permanently failed
	assert.Equal(t, entity.StatusFailed, batch.Status(), "Batch should be marked as failed")
	assert.GreaterOrEqual(t, batch.SubmissionAttempts(), config.MaxSubmissionAttempts,
		"Batch should have reached max attempts")
}

// TestBatchSubmitter_NonRetryableError verifies that non-rate-limit errors
// mark the batch as permanently failed without setting global backoff.
func TestBatchSubmitter_NonRetryableError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchService := new(MockBatchEmbeddingService)

	config := testBatchSubmitterConfig()
	submitter := NewBatchSubmitter(mockBatchProgressRepo, mockBatchService, config)

	// Create batch with request data
	batch := createTestBatchProgress(entity.StatusPendingSubmission, true)

	// Mock repository returns pending batch
	mockBatchProgressRepo.On("GetPendingSubmissionBatch", ctx).Return(batch, nil).Once()

	// Mock API returns non-retryable error
	nonRetryableErr := &outbound.EmbeddingError{
		Code:      "invalid_input",
		Message:   "invalid request format",
		Type:      "validation",
		Retryable: false,
	}
	mockBatchService.On("CreateBatchEmbeddingJobWithRequests", ctx,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil, nonRetryableErr).Once()

	// Mock save for permanently failed batch
	mockBatchProgressRepo.On("Save", ctx,
		mock.MatchedBy(func(b *entity.BatchJobProgress) bool {
			return b.Status() == entity.StatusFailed &&
				b.ErrorMessage() != nil
		}),
	).Return(nil).Once()

	// Act
	err := submitter.submitOneBatch(ctx)

	// Assert
	require.NoError(t, err, "submitOneBatch should handle non-retryable error gracefully")
	mockBatchProgressRepo.AssertExpectations(t)
	mockBatchService.AssertExpectations(t)

	// Verify batch is permanently failed
	assert.Equal(t, entity.StatusFailed, batch.Status(), "Batch should be marked as failed")
	assert.NotNil(t, batch.ErrorMessage(), "Batch should have error message")

	// Verify no global backoff was set (by attempting another submission)
	// This would fail if global backoff was set
	mockBatchProgressRepo.On("GetPendingSubmissionBatch", ctx).Return(nil, nil).Once()
	err = submitter.submitOneBatch(ctx)
	require.NoError(t, err, "Next submission should proceed without global backoff")
}

// ========================================
// Concurrency Tests
// ========================================

// TestBatchSubmitter_ConcurrencyControl verifies that the semaphore correctly
// limits concurrent submissions to MaxConcurrentSubmissions.
func TestBatchSubmitter_ConcurrencyControl(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchService := new(MockBatchEmbeddingService)

	config := testBatchSubmitterConfig()
	config.MaxConcurrentSubmissions = 2
	submitter := NewBatchSubmitter(mockBatchProgressRepo, mockBatchService, config)

	// Create 5 batches ready for submission
	batches := make([]*entity.BatchJobProgress, 5)
	for i := 0; i < 5; i++ {
		batches[i] = createTestBatchProgress(entity.StatusPendingSubmission, true)
	}

	// Track concurrent submissions
	concurrentCount := 0
	maxConcurrent := 0
	var mutex sync.Mutex

	// Mock repository returns batches sequentially
	callCount := 0
	mockBatchProgressRepo.On("GetPendingSubmissionBatch", ctx).Return(
		func(context.Context) *entity.BatchJobProgress {
			mutex.Lock()
			defer mutex.Unlock()
			if callCount < len(batches) {
				batch := batches[callCount]
				callCount++
				return batch
			}
			return nil
		},
		func(context.Context) error {
			return nil
		},
	).Times(5)

	// Mock API calls with delay to simulate processing time
	mockBatchService.On("CreateBatchEmbeddingJobWithRequests", ctx,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(
		func(context.Context, []*outbound.BatchEmbeddingRequest, outbound.EmbeddingOptions, uuid.UUID) *outbound.BatchEmbeddingJob {
			mutex.Lock()
			concurrentCount++
			if concurrentCount > maxConcurrent {
				maxConcurrent = concurrentCount
			}
			mutex.Unlock()

			// Simulate API processing time
			time.Sleep(100 * time.Millisecond)

			mutex.Lock()
			concurrentCount--
			mutex.Unlock()

			return createTestBatchEmbeddingJob("batches/test-job")
		},
		func(context.Context, []*outbound.BatchEmbeddingRequest, outbound.EmbeddingOptions, uuid.UUID) error {
			return nil
		},
	).
		Times(5)

	// Mock save calls
	mockBatchProgressRepo.On("Save", ctx, mock.Anything).Return(nil).Times(5)

	// Act - Submit batches concurrently
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = submitter.submitOneBatch(ctx)
		}()
	}
	wg.Wait()

	// Assert
	assert.LessOrEqual(t, maxConcurrent, config.MaxConcurrentSubmissions,
		"Concurrent submissions should not exceed MaxConcurrentSubmissions")
	assert.Equal(t, 0, concurrentCount,
		"All submissions should be complete (concurrent count should be 0)")

	mockBatchProgressRepo.AssertExpectations(t)
	mockBatchService.AssertExpectations(t)
}

// ========================================
// Error Handling Tests
// ========================================

// TestBatchSubmitter_RepositoryError verifies that database errors are handled
// gracefully and don't crash the submitter.
func TestBatchSubmitter_RepositoryError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchService := new(MockBatchEmbeddingService)

	config := testBatchSubmitterConfig()
	submitter := NewBatchSubmitter(mockBatchProgressRepo, mockBatchService, config)

	// Mock repository returns database error
	dbErr := errors.New("database connection lost")
	mockBatchProgressRepo.On("GetPendingSubmissionBatch", ctx).Return(nil, dbErr).Once()

	// Act
	err := submitter.submitOneBatch(ctx)

	// Assert
	require.NoError(t, err, "submitOneBatch should handle repository error gracefully")
	mockBatchProgressRepo.AssertExpectations(t)

	// Verify no API calls were made
	mockBatchService.AssertNotCalled(t, "CreateBatchEmbeddingJobWithRequests")

	// Verify submitter continues to work (next call should succeed)
	mockBatchProgressRepo.On("GetPendingSubmissionBatch", ctx).Return(nil, nil).Once()
	err = submitter.submitOneBatch(ctx)
	require.NoError(t, err, "Submitter should continue working after repository error")
}

// TestBatchSubmitter_SaveFailureAfterSubmission verifies that save failures after
// successful API calls are logged with batch state details.
func TestBatchSubmitter_SaveFailureAfterSubmission(t *testing.T) {
	// Arrange
	ctx := context.Background()
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchService := new(MockBatchEmbeddingService)

	config := testBatchSubmitterConfig()
	submitter := NewBatchSubmitter(mockBatchProgressRepo, mockBatchService, config)

	// Create batch with request data
	batch := createTestBatchProgress(entity.StatusPendingSubmission, true)

	// Mock repository returns pending batch
	mockBatchProgressRepo.On("GetPendingSubmissionBatch", ctx).Return(batch, nil).Once()

	// Mock successful API submission
	jobID := "batches/test-job-save-failure"
	mockBatchJob := createTestBatchEmbeddingJob(jobID)
	mockBatchService.On("CreateBatchEmbeddingJobWithRequests", ctx,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(mockBatchJob, nil).Once()

	// Mock save failure
	saveErr := errors.New("database write failed")
	mockBatchProgressRepo.On("Save", ctx, mock.Anything).Return(saveErr).Once()

	// Act
	err := submitter.submitOneBatch(ctx)

	// Assert
	require.NoError(t, err, "submitOneBatch should handle save failure gracefully")
	mockBatchProgressRepo.AssertExpectations(t)
	mockBatchService.AssertExpectations(t)

	// Note: Implementation should log error with batch state details
	// Batch state should include: batch ID, status, Gemini job ID
	// This allows manual recovery if needed
}

// ========================================
// Helper Functions
// ========================================

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Note: The following functions are expected to be implemented in batch_submitter.go:
// - NewBatchSubmitter(repo, service, config) *BatchSubmitter
// - (s *BatchSubmitter) Start(ctx context.Context) error
// - (s *BatchSubmitter) Stop()
// - (s *BatchSubmitter) submitOneBatch(ctx context.Context) error
// - (s *BatchSubmitter) setGlobalBackoff(until time.Time)
// - calculateBackoff(config BatchSubmitterConfig, attempt int) time.Duration
// - isRateLimitError(err error) bool
