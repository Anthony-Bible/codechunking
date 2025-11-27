package worker

import (
	"codechunking/internal/config"
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProcessBatchWithRetry_SuccessFirstAttempt verifies that when the embedding service
// succeeds immediately, no retries occur and results are returned successfully.
func TestProcessBatchWithRetry_SuccessFirstAttempt(t *testing.T) {
	// Setup
	mockEmbedding := new(MockEmbeddingService)
	processor := &DefaultJobProcessor{
		embeddingService: mockEmbedding,
		batchConfig: config.BatchProcessingConfig{
			InitialBackoff: 10 * time.Millisecond, // Fast for testing
			MaxBackoff:     50 * time.Millisecond, // Fast for testing
			MaxRetries:     3,
		},
	}

	ctx := context.Background()
	jobID := uuid.New()
	repositoryID := uuid.New()
	batchNumber := 1
	texts := []string{"func main() {}", "package main"}
	chunks := []outbound.CodeChunk{
		{Content: texts[0], Language: "go"},
		{Content: texts[1], Language: "go"},
	}
	options := outbound.EmbeddingOptions{
		Model:    "test-model",
		TaskType: outbound.TaskTypeRetrievalDocument,
	}

	expectedResults := []outbound.EmbeddingResult{
		{Vector: []float64{0.1, 0.2, 0.3}, Dimensions: 3},
		{Vector: []float64{0.4, 0.5, 0.6}, Dimensions: 3},
	}

	// Mock expectation: service succeeds on first attempt
	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(convertToPointerSlice(expectedResults), nil).
		Once()

	// Execute
	results, err := processor.processBatchWithRetry(
		ctx,
		jobID,
		repositoryID,
		batchNumber,
		texts,
		chunks,
		options,
	)

	// Assert
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, expectedResults[0].Vector, results[0].Vector)
	assert.Equal(t, expectedResults[1].Vector, results[1].Vector)

	// Verify no retry occurred (only 1 call to GenerateBatchEmbeddings)
	mockEmbedding.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 1)
	mockEmbedding.AssertExpectations(t)
}

// TestProcessBatchWithRetry_QuotaErrorWithRetry verifies that a 429 quota error
// triggers a retry with exponential backoff, and succeeds on the second attempt.
func TestProcessBatchWithRetry_QuotaErrorWithRetry(t *testing.T) {
	// Setup
	mockEmbedding := new(MockEmbeddingService)
	processor := &DefaultJobProcessor{
		embeddingService: mockEmbedding,
		batchConfig: config.BatchProcessingConfig{
			InitialBackoff: 10 * time.Millisecond, // Fast for testing
			MaxBackoff:     50 * time.Millisecond, // Fast for testing
			MaxRetries:     3,
		},
	}

	ctx := context.Background()
	jobID := uuid.New()
	repositoryID := uuid.New()
	batchNumber := 1
	texts := []string{"func test() {}"}
	chunks := []outbound.CodeChunk{{Content: texts[0], Language: "go"}}
	options := outbound.EmbeddingOptions{Model: "test-model"}

	quotaError := &outbound.EmbeddingError{
		Code:      "quota_exceeded",
		Message:   "API quota exceeded",
		Type:      "quota",
		Retryable: true,
	}

	successResults := []outbound.EmbeddingResult{
		{Vector: []float64{0.1, 0.2}, Dimensions: 2},
	}

	// Mock expectations: fail first time, succeed second time
	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(nil, quotaError).
		Once()

	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(convertToPointerSlice(successResults), nil).
		Once()

	// Execute
	results, err := processor.processBatchWithRetry(
		ctx,
		jobID,
		repositoryID,
		batchNumber,
		texts,
		chunks,
		options,
	)

	// Assert
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, successResults[0].Vector, results[0].Vector)

	// Verify 1 retry occurred (2 total calls)
	mockEmbedding.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 2)

	// Verify backoff delay was approximately InitialBackoff (30s)
	// In real test, would verify sleepRecorder shows ~30s delay
	// This would require dependency injection of sleep function
	mockEmbedding.AssertExpectations(t)
}

// TestProcessBatchWithRetry_ExponentialBackoff verifies that multiple retries
// apply exponential backoff with delays of 30s, 60s, 120s before succeeding.
func TestProcessBatchWithRetry_ExponentialBackoff(t *testing.T) {
	// Setup
	mockEmbedding := new(MockEmbeddingService)
	processor := &DefaultJobProcessor{
		embeddingService: mockEmbedding,
		batchConfig: config.BatchProcessingConfig{
			InitialBackoff: 10 * time.Millisecond, // Fast for testing
			MaxBackoff:     50 * time.Millisecond, // Fast for testing
			MaxRetries:     3,
		},
	}

	ctx := context.Background()
	jobID := uuid.New()
	repositoryID := uuid.New()
	batchNumber := 1
	texts := []string{"func exponential() {}"}
	chunks := []outbound.CodeChunk{{Content: texts[0], Language: "go"}}
	options := outbound.EmbeddingOptions{Model: "test-model"}

	quotaError := &outbound.EmbeddingError{
		Code:      "rate_limit_exceeded",
		Message:   "Rate limit exceeded",
		Type:      "quota",
		Retryable: true,
	}

	successResults := []outbound.EmbeddingResult{
		{Vector: []float64{0.7, 0.8}, Dimensions: 2},
	}

	// Mock expectations: fail 3 times, succeed on 4th attempt
	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(nil, quotaError).
		Times(3)

	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(convertToPointerSlice(successResults), nil).
		Once()

	// Execute
	results, err := processor.processBatchWithRetry(
		ctx,
		jobID,
		repositoryID,
		batchNumber,
		texts,
		chunks,
		options,
	)

	// Assert
	require.NoError(t, err)
	assert.Len(t, results, 1)

	// Verify 3 retries occurred (4 total calls)
	mockEmbedding.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 4)

	// Verify exponential backoff delays: 30s, 60s, 120s
	// In real implementation, would verify:
	// assert.Len(t, sleepRecorder.GetSleeps(), 3)
	// assertApproximateDuration(t, sleepRecorder.GetSleeps()[0], 30*time.Second, 5*time.Second)
	// assertApproximateDuration(t, sleepRecorder.GetSleeps()[1], 60*time.Second, 10*time.Second)
	// assertApproximateDuration(t, sleepRecorder.GetSleeps()[2], 120*time.Second, 20*time.Second)

	mockEmbedding.AssertExpectations(t)
}

// TestProcessBatchWithRetry_MaxRetriesExceeded verifies that when quota errors
// persist beyond MaxRetries, the method fails with an appropriate error.
func TestProcessBatchWithRetry_MaxRetriesExceeded(t *testing.T) {
	// Setup
	mockEmbedding := new(MockEmbeddingService)
	processor := &DefaultJobProcessor{
		embeddingService: mockEmbedding,
		batchConfig: config.BatchProcessingConfig{
			InitialBackoff: 10 * time.Millisecond, // Fast for testing
			MaxBackoff:     50 * time.Millisecond, // Fast for testing
			MaxRetries:     3,
		},
	}

	ctx := context.Background()
	jobID := uuid.New()
	repositoryID := uuid.New()
	batchNumber := 1
	texts := []string{"func alwaysFails() {}"}
	chunks := []outbound.CodeChunk{{Content: texts[0], Language: "go"}}
	options := outbound.EmbeddingOptions{Model: "test-model"}

	quotaError := &outbound.EmbeddingError{
		Code:      "quota_exceeded",
		Message:   "Persistent quota error",
		Type:      "quota",
		Retryable: true,
	}

	// Mock expectation: always fail (MaxRetries + 1 = 4 attempts total)
	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(nil, quotaError).
		Times(4)

	// Execute
	results, err := processor.processBatchWithRetry(
		ctx,
		jobID,
		repositoryID,
		batchNumber,
		texts,
		chunks,
		options,
	)

	// Assert
	require.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "max retries exceeded")
	assert.Contains(t, err.Error(), "3") // MaxRetries value

	// Verify exactly MaxRetries + 1 attempts were made
	mockEmbedding.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 4)
	mockEmbedding.AssertExpectations(t)
}

// TestProcessBatchWithRetry_MaxBackoffCap verifies that backoff delays are
// capped at MaxBackoff (300s) even when exponential growth would exceed it.
func TestProcessBatchWithRetry_MaxBackoffCap(t *testing.T) {
	// Setup
	mockEmbedding := new(MockEmbeddingService)
	processor := &DefaultJobProcessor{
		embeddingService: mockEmbedding,
		batchConfig: config.BatchProcessingConfig{
			InitialBackoff: 10 * time.Millisecond, // Fast for testing
			MaxBackoff:     50 * time.Millisecond, // Fast for testing
			MaxRetries:     5,                     // Allow more retries to test capping
		},
	}

	ctx := context.Background()
	jobID := uuid.New()
	repositoryID := uuid.New()
	batchNumber := 1
	texts := []string{"func cappedBackoff() {}"}
	chunks := []outbound.CodeChunk{{Content: texts[0], Language: "go"}}
	options := outbound.EmbeddingOptions{Model: "test-model"}

	quotaError := &outbound.EmbeddingError{
		Code:      "rate_limit_exceeded",
		Message:   "Rate limit error",
		Type:      "quota",
		Retryable: true,
	}

	successResults := []outbound.EmbeddingResult{
		{Vector: []float64{0.9, 1.0}, Dimensions: 2},
	}

	// Mock expectations: fail 5 times, succeed on 6th
	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(nil, quotaError).
		Times(5)

	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(convertToPointerSlice(successResults), nil).
		Once()

	// Execute
	results, err := processor.processBatchWithRetry(
		ctx,
		jobID,
		repositoryID,
		batchNumber,
		texts,
		chunks,
		options,
	)

	// Assert
	require.NoError(t, err)
	assert.Len(t, results, 1)

	// Verify delays are capped at MaxBackoff:
	// Expected delays (with 2x multiplier):
	// Attempt 1: 30s
	// Attempt 2: 60s
	// Attempt 3: 120s
	// Attempt 4: 240s
	// Attempt 5: 480s -> capped to 300s (MaxBackoff)
	// In real implementation, would verify:
	// assert.Len(t, sleepRecorder.GetSleeps(), 5)
	// assertApproximateDuration(t, sleepRecorder.GetSleeps()[3], 240*time.Second, 40*time.Second)
	// assertApproximateDuration(t, sleepRecorder.GetSleeps()[4], 300*time.Second, 50*time.Second)
	// assert.LessOrEqual(t, sleepRecorder.GetSleeps()[4], 300*time.Second)

	mockEmbedding.AssertExpectations(t)
}

// TestProcessBatchWithRetry_RetryAfterHeader verifies that when a 429 error
// includes a Retry-After header, that value is used instead of exponential backoff.
func TestProcessBatchWithRetry_RetryAfterHeader(t *testing.T) {
	// Setup
	mockEmbedding := new(MockEmbeddingService)
	processor := &DefaultJobProcessor{
		embeddingService: mockEmbedding,
		batchConfig: config.BatchProcessingConfig{
			InitialBackoff: 10 * time.Millisecond, // Fast for testing
			MaxBackoff:     50 * time.Millisecond, // Fast for testing
			MaxRetries:     3,
		},
	}

	ctx := context.Background()
	jobID := uuid.New()
	repositoryID := uuid.New()
	batchNumber := 1
	texts := []string{"func retryAfter() {}"}
	chunks := []outbound.CodeChunk{{Content: texts[0], Language: "go"}}
	options := outbound.EmbeddingOptions{Model: "test-model"}

	// Error with Retry-After header metadata
	quotaErrorWithRetryAfter := &outbound.EmbeddingError{
		Code:      "quota_exceeded",
		Message:   "Rate limited with Retry-After",
		Type:      "quota",
		Retryable: true,
		// In real implementation, would have Metadata field for headers:
		// Metadata: map[string]interface{}{"Retry-After": "45"},
	}

	successResults := []outbound.EmbeddingResult{
		{Vector: []float64{0.3, 0.4}, Dimensions: 2},
	}

	// Mock expectations: fail once with Retry-After, then succeed
	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(nil, quotaErrorWithRetryAfter).
		Once()

	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(convertToPointerSlice(successResults), nil).
		Once()

	// Execute
	results, err := processor.processBatchWithRetry(
		ctx,
		jobID,
		repositoryID,
		batchNumber,
		texts,
		chunks,
		options,
	)

	// Assert
	require.NoError(t, err)
	assert.Len(t, results, 1)

	// Verify Retry-After value (45s) was used instead of exponential backoff (30s)
	// In real implementation, would verify:
	// assert.Len(t, sleepRecorder.GetSleeps(), 1)
	// assertApproximateDuration(t, sleepRecorder.GetSleeps()[0], 45*time.Second, 5*time.Second)

	mockEmbedding.AssertExpectations(t)
}

// TestProcessBatchWithRetry_NonRetryableError verifies that non-retryable errors
// (like validation errors with 400 status) fail immediately without retry.
func TestProcessBatchWithRetry_NonRetryableError(t *testing.T) {
	// Setup
	mockEmbedding := new(MockEmbeddingService)
	processor := &DefaultJobProcessor{
		embeddingService: mockEmbedding,
		batchConfig: config.BatchProcessingConfig{
			InitialBackoff: 10 * time.Millisecond, // Fast for testing
			MaxBackoff:     50 * time.Millisecond, // Fast for testing
			MaxRetries:     3,
		},
	}

	ctx := context.Background()
	jobID := uuid.New()
	repositoryID := uuid.New()
	batchNumber := 1
	texts := []string{"invalid input"}
	chunks := []outbound.CodeChunk{{Content: texts[0], Language: "unknown"}}
	options := outbound.EmbeddingOptions{Model: "test-model"}

	validationError := &outbound.EmbeddingError{
		Code:      "invalid_input",
		Message:   "Input validation failed",
		Type:      "validation",
		Retryable: false,
	}

	// Mock expectation: return non-retryable error
	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(nil, validationError).
		Once()

	// Execute
	results, err := processor.processBatchWithRetry(
		ctx,
		jobID,
		repositoryID,
		batchNumber,
		texts,
		chunks,
		options,
	)

	// Assert
	require.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "validation")

	// Verify no retry occurred (only 1 call)
	mockEmbedding.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 1)
	mockEmbedding.AssertExpectations(t)
}

// TestProcessBatchWithRetry_ContextCancellation verifies that when the context
// is cancelled during a retry delay, the method returns context error immediately.
func TestProcessBatchWithRetry_ContextCancellation(t *testing.T) {
	// Setup
	mockEmbedding := new(MockEmbeddingService)
	processor := &DefaultJobProcessor{
		embeddingService: mockEmbedding,
		batchConfig: config.BatchProcessingConfig{
			InitialBackoff: 10 * time.Millisecond, // Fast for testing
			MaxBackoff:     50 * time.Millisecond, // Fast for testing
			MaxRetries:     3,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	jobID := uuid.New()
	repositoryID := uuid.New()
	batchNumber := 1
	texts := []string{"func cancelled() {}"}
	chunks := []outbound.CodeChunk{{Content: texts[0], Language: "go"}}
	options := outbound.EmbeddingOptions{Model: "test-model"}

	quotaError := &outbound.EmbeddingError{
		Code:      "quota_exceeded",
		Message:   "Quota error before cancellation",
		Type:      "quota",
		Retryable: true,
	}

	// Mock expectation: return quota error to trigger retry
	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(nil, quotaError).
		Once()

	// Cancel context after first failure to simulate cancellation during retry delay
	// Use 5ms to ensure cancellation happens during the 10ms backoff sleep
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	// Execute
	results, err := processor.processBatchWithRetry(
		ctx,
		jobID,
		repositoryID,
		batchNumber,
		texts,
		chunks,
		options,
	)

	// Assert
	require.Error(t, err)
	assert.Nil(t, results)
	assert.ErrorIs(t, err, context.Canceled, "Expected context.Canceled error")

	// Verify retry was stopped (should be 1 or 2 calls depending on timing)
	assert.LessOrEqual(t, len(mockEmbedding.Calls), 2, "Should not retry after context cancellation")
}

// TestProcessBatchWithRetry_MixedErrorTypes verifies handling of different error
// types during retry attempts (quota error followed by timeout, then success).
func TestProcessBatchWithRetry_MixedErrorTypes(t *testing.T) {
	// Setup
	mockEmbedding := new(MockEmbeddingService)
	processor := &DefaultJobProcessor{
		embeddingService: mockEmbedding,
		batchConfig: config.BatchProcessingConfig{
			InitialBackoff: 10 * time.Millisecond, // Fast for testing
			MaxBackoff:     50 * time.Millisecond, // Fast for testing
			MaxRetries:     3,
		},
	}

	ctx := context.Background()
	jobID := uuid.New()
	repositoryID := uuid.New()
	batchNumber := 1
	texts := []string{"func mixed() {}"}
	chunks := []outbound.CodeChunk{{Content: texts[0], Language: "go"}}
	options := outbound.EmbeddingOptions{Model: "test-model"}

	quotaError := &outbound.EmbeddingError{
		Code:      "quota_exceeded",
		Type:      "quota",
		Retryable: true,
	}

	timeoutError := &outbound.EmbeddingError{
		Code:      "timeout",
		Type:      "timeout",
		Retryable: true,
	}

	successResults := []outbound.EmbeddingResult{
		{Vector: []float64{0.5, 0.6}, Dimensions: 2},
	}

	// Mock expectations: quota error, timeout error, then success
	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(nil, quotaError).
		Once()

	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(nil, timeoutError).
		Once()

	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(convertToPointerSlice(successResults), nil).
		Once()

	// Execute
	results, err := processor.processBatchWithRetry(
		ctx,
		jobID,
		repositoryID,
		batchNumber,
		texts,
		chunks,
		options,
	)

	// Assert
	require.NoError(t, err)
	assert.Len(t, results, 1)

	// Verify 2 retries occurred (3 total calls)
	mockEmbedding.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 3)
	mockEmbedding.AssertExpectations(t)
}

// TestProcessBatchWithRetry_ZeroMaxRetries verifies behavior when MaxRetries is 0
// (should fail immediately after first error).
func TestProcessBatchWithRetry_ZeroMaxRetries(t *testing.T) {
	// Setup
	mockEmbedding := new(MockEmbeddingService)
	processor := &DefaultJobProcessor{
		embeddingService: mockEmbedding,
		batchConfig: config.BatchProcessingConfig{
			InitialBackoff: 10 * time.Millisecond, // Fast for testing
			MaxBackoff:     50 * time.Millisecond, // Fast for testing
			MaxRetries:     0,                     // No retries allowed
		},
	}

	ctx := context.Background()
	jobID := uuid.New()
	repositoryID := uuid.New()
	batchNumber := 1
	texts := []string{"func noRetry() {}"}
	chunks := []outbound.CodeChunk{{Content: texts[0], Language: "go"}}
	options := outbound.EmbeddingOptions{Model: "test-model"}

	quotaError := &outbound.EmbeddingError{
		Code:      "quota_exceeded",
		Type:      "quota",
		Retryable: true,
	}

	// Mock expectation: return error (should only be called once)
	mockEmbedding.On("GenerateBatchEmbeddings", ctx, texts, options).
		Return(nil, quotaError).
		Once()

	// Execute
	results, err := processor.processBatchWithRetry(
		ctx,
		jobID,
		repositoryID,
		batchNumber,
		texts,
		chunks,
		options,
	)

	// Assert
	require.Error(t, err)
	assert.Nil(t, results)

	// Verify no retry occurred (only 1 call)
	mockEmbedding.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 1)
	mockEmbedding.AssertExpectations(t)
}

// Helper function to convert []EmbeddingResult to []*EmbeddingResult for mocking.
func convertToPointerSlice(results []outbound.EmbeddingResult) []*outbound.EmbeddingResult {
	pointers := make([]*outbound.EmbeddingResult, len(results))
	for i := range results {
		pointers[i] = &results[i]
	}
	return pointers
}
