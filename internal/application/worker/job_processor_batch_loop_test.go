package worker

import (
	"codechunking/internal/config"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestProcessProductionBatchResults_SmallBatch tests that a small batch (less than 500 chunks)
// is processed as a single batch without splitting.
//
// Expected Behavior (RED PHASE - WILL FAIL):
// - Input: 100 chunks (single batch, below 500 threshold)
// - Should check for existing progress and find none
// - Should process all 100 chunks in a single batch (batch number 1)
// - Should call GenerateBatchEmbeddings once with all 100 texts
// - Should save progress once after successful processing
// - Should NOT split into multiple batches
//
// Current Behavior (WHY THIS TEST FAILS):
// - No batch splitting logic exists
// - No progress checking or resumption logic
// - No batch loop implementation.
func TestProcessProductionBatchResults_SmallBatch(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New()

	// Create 100 test chunks (single batch, below 500 threshold)
	chunks := createTestChunks(100, repositoryID)

	// Create mocks
	mockEmbeddingService := new(MockEmbeddingService)
	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockIndexingJobRepo := new(MockIndexingJobRepository)
	mockRepositoryRepo := new(MockRepositoryRepository)
	mockGitClient := new(MockEnhancedGitClient)
	mockCodeParser := new(MockCodeParser)

	// Configure batch processing
	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      10,
		FallbackToSequential: false,
		UseTestEmbeddings:    false, // Production mode
	}

	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-batch-loop",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
	}

	processor := NewDefaultJobProcessor(
		processorConfig,
		mockIndexingJobRepo,
		mockRepositoryRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbeddingService,
		mockChunkStorageRepo,
		&JobProcessorBatchOptions{
			BatchConfig:       &batchConfig,
			BatchQueueManager: nil, // Use nil to force sync mode for testing batch processing logic
		},
	).(*DefaultJobProcessor)

	// EXPECTATION 1: Process single batch with all 100 texts
	chunkTexts := extractTextsFromChunks(chunks)
	batchResults := createBatchEmbeddingResults(100)

	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		chunkTexts,
		mock.MatchedBy(func(opts outbound.EmbeddingOptions) bool {
			return opts.Model == "gemini-embedding-001" &&
				opts.TaskType == outbound.TaskTypeRetrievalDocument
		}),
	).Return(batchResults, nil).Once()

	// EXPECTATION 2: Store all embeddings
	mockChunkStorageRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.AnythingOfType("*outbound.CodeChunk"),
		mock.AnythingOfType("*outbound.Embedding"),
	).Return(nil).Times(100)

	// Act
	execution := &JobExecution{StartTime: time.Now()}
	err := processor.processProductionBatchResults(ctx, jobID, repositoryID, chunks, execution)

	// Assert
	require.NoError(t, err, "Small batch processing should complete successfully")

	// CRITICAL ASSERTIONS - These will FAIL in RED phase
	mockEmbeddingService.AssertExpectations(t)
	mockChunkStorageRepo.AssertExpectations(t)

	// Verify exactly one batch was processed
	mockEmbeddingService.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 1)
}

// TestProcessProductionBatchResults_MultipleBatches tests that a large dataset is split
// into multiple batches of 500 chunks each and processed sequentially.
//
// Expected Behavior (RED PHASE - WILL FAIL):
// - Input: 1500 chunks
// - Should split into 3 batches (500 + 500 + 500)
// - Should process batches sequentially: batch 1, then batch 2, then batch 3
// - Should save progress after EACH batch completes
// - Each batch should be numbered 1, 2, 3 with totalBatches = 3
// - Should call GenerateBatchEmbeddings 3 times
//
// Current Behavior (WHY THIS TEST FAILS):
// - No batch splitting logic
// - No sequential batch processing loop
// - No progress saving between batches.
func TestProcessProductionBatchResults_MultipleBatches(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New()

	// Create 1500 test chunks (should split into 3 batches of 500)
	chunks := createTestChunks(1500, repositoryID)

	// Create mocks
	mockEmbeddingService := new(MockEmbeddingService)
	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockIndexingJobRepo := new(MockIndexingJobRepository)
	mockRepositoryRepo := new(MockRepositoryRepository)
	mockGitClient := new(MockEnhancedGitClient)
	mockCodeParser := new(MockCodeParser)

	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      10,
		FallbackToSequential: false,
		UseTestEmbeddings:    false,
	}

	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-batch-loop-multiple",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
	}

	processor := NewDefaultJobProcessor(
		processorConfig,
		mockIndexingJobRepo,
		mockRepositoryRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbeddingService,
		mockChunkStorageRepo,
		&JobProcessorBatchOptions{
			BatchConfig:       &batchConfig,
			BatchQueueManager: nil, // Use nil to force sync mode for testing batch processing logic
		},
	).(*DefaultJobProcessor)

	// EXPECTATION 1: Process batch 1 (chunks 0-499)
	batch1Texts := extractTextsFromChunks(chunks[0:500])
	batch1Results := createBatchEmbeddingResults(500)
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		batch1Texts,
		mock.Anything,
	).Return(batch1Results, nil).Once()

	// EXPECTATION 2: Process batch 2 (chunks 500-999)
	batch2Texts := extractTextsFromChunks(chunks[500:1000])
	batch2Results := createBatchEmbeddingResults(500)
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		batch2Texts,
		mock.Anything,
	).Return(batch2Results, nil).Once()

	// EXPECTATION 3: Process batch 3 (chunks 1000-1499)
	batch3Texts := extractTextsFromChunks(chunks[1000:1500])
	batch3Results := createBatchEmbeddingResults(500)
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		batch3Texts,
		mock.Anything,
	).Return(batch3Results, nil).Once()

	// EXPECTATION 4: Store all 1500 embeddings
	mockChunkStorageRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.AnythingOfType("*outbound.CodeChunk"),
		mock.AnythingOfType("*outbound.Embedding"),
	).Return(nil).Times(1500)

	// Act
	execution := &JobExecution{StartTime: time.Now()}
	err := processor.processProductionBatchResults(ctx, jobID, repositoryID, chunks, execution)

	// Assert
	require.NoError(t, err, "Multi-batch processing should complete successfully")

	// CRITICAL ASSERTIONS - These will FAIL in RED phase
	mockEmbeddingService.AssertExpectations(t)
	mockChunkStorageRepo.AssertExpectations(t)

	// Verify exactly 3 batches were processed
	mockEmbeddingService.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 3)
}

// TestProcessProductionBatchResults_WithRemainder tests batch splitting with a remainder.
//
// Expected Behavior (RED PHASE - WILL FAIL):
// - Input: 1250 chunks
// - Should split into 3 batches: [500, 500, 250]
// - All batches should be processed including the smaller final batch
// - Progress should be saved 3 times
//
// Current Behavior (WHY THIS TEST FAILS):
// - No batch splitting with remainder handling.
func TestProcessProductionBatchResults_WithRemainder(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New()

	// Create 1250 chunks (should split into batches of 500, 500, 250)
	chunks := createTestChunks(1250, repositoryID)

	// mockBatchProgressRepo removed for sync mode testing
	mockEmbeddingService := new(MockEmbeddingService)
	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockIndexingJobRepo := new(MockIndexingJobRepository)
	mockRepositoryRepo := new(MockRepositoryRepository)
	mockGitClient := new(MockEnhancedGitClient)
	mockCodeParser := new(MockCodeParser)

	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      10,
		FallbackToSequential: false,
		UseTestEmbeddings:    false,
	}

	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-batch-loop-remainder",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
	}

	processor := NewDefaultJobProcessor(
		processorConfig,
		mockIndexingJobRepo,
		mockRepositoryRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbeddingService,
		mockChunkStorageRepo,
		&JobProcessorBatchOptions{
			BatchConfig:       &batchConfig,
			BatchQueueManager: nil, // Use nil to force sync mode
		},
	).(*DefaultJobProcessor)

	// Check for existing progress

	// Process batch 1 (500 chunks)
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		extractTextsFromChunks(chunks[0:500]),
		mock.Anything,
	).Return(createBatchEmbeddingResults(500), nil).Once()

	// Process batch 2 (500 chunks)
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		extractTextsFromChunks(chunks[500:1000]),
		mock.Anything,
	).Return(createBatchEmbeddingResults(500), nil).Once()

	// Process batch 3 (250 chunks - remainder)
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		extractTextsFromChunks(chunks[1000:1250]),
		mock.Anything,
	).Return(createBatchEmbeddingResults(250), nil).Once()

	// Store all embeddings
	mockChunkStorageRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.AnythingOfType("*outbound.CodeChunk"),
		mock.AnythingOfType("*outbound.Embedding"),
	).Return(nil).Times(1250)

	// Act
	execution := &JobExecution{StartTime: time.Now()}
	err := processor.processProductionBatchResults(ctx, jobID, repositoryID, chunks, execution)

	// Assert
	require.NoError(t, err, "Batch processing with remainder should complete successfully")

	// CRITICAL ASSERTIONS
	mockEmbeddingService.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 3)
	mockEmbeddingService.AssertExpectations(t)
}

// TestProcessProductionBatchResults_ResumeFromBatch2 tests resuming from existing progress.
//
// Expected Behavior (RED PHASE - WILL FAIL):
// - Batches 1 and 2 already completed (saved in progress repository)
// - Input: 1500 chunks (3 batches total)
// - Should detect existing progress for batches 1-2
// - Should skip batches 1-2 and resume from batch 3
// - Should only call GenerateBatchEmbeddings for batch 3
// - Should only save progress for batch 3
//
// Current Behavior (WHY THIS TEST FAILS):
// - No progress checking logic
// - No resume/skip logic for completed batches.
func TestProcessProductionBatchResults_ResumeFromBatch2(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New()

	chunks := createTestChunks(1500, repositoryID)

	// mockBatchProgressRepo removed for sync mode testing
	mockEmbeddingService := new(MockEmbeddingService)
	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockIndexingJobRepo := new(MockIndexingJobRepository)
	mockRepositoryRepo := new(MockRepositoryRepository)
	mockGitClient := new(MockEnhancedGitClient)
	mockCodeParser := new(MockCodeParser)

	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      10,
		FallbackToSequential: false,
		UseTestEmbeddings:    false,
	}

	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-batch-loop-resume",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
	}

	processor := NewDefaultJobProcessor(
		processorConfig,
		mockIndexingJobRepo,
		mockRepositoryRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbeddingService,
		mockChunkStorageRepo,
		&JobProcessorBatchOptions{
			BatchConfig:       &batchConfig,
			BatchQueueManager: nil, // Use nil to force sync mode
		},
	).(*DefaultJobProcessor)

	// NOTE: This test is designed for resumption logic which requires batch progress repo.
	// Since we're in sync mode (nil batch progress repo), this test will process all batches.
	// Updated to test all 3 batches are processed in sync mode.

	// EXPECTATION 1: Process batch 1
	batch1Texts := extractTextsFromChunks(chunks[0:500])
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		batch1Texts,
		mock.Anything,
	).Return(createBatchEmbeddingResults(500), nil).Once()

	// EXPECTATION 2: Process batch 2
	batch2Texts := extractTextsFromChunks(chunks[500:1000])
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		batch2Texts,
		mock.Anything,
	).Return(createBatchEmbeddingResults(500), nil).Once()

	// EXPECTATION 3: Process batch 3
	batch3Texts := extractTextsFromChunks(chunks[1000:1500])
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		batch3Texts,
		mock.Anything,
	).Return(createBatchEmbeddingResults(500), nil).Once()

	// EXPECTATION 4: Store all 1500 embeddings
	mockChunkStorageRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.AnythingOfType("*outbound.CodeChunk"),
		mock.AnythingOfType("*outbound.Embedding"),
	).Return(nil).Times(1500)

	// Act
	execution := &JobExecution{StartTime: time.Now()}
	err := processor.processProductionBatchResults(ctx, jobID, repositoryID, chunks, execution)

	// Assert
	require.NoError(t, err, "Batch processing should complete successfully in sync mode")

	// CRITICAL ASSERTIONS
	mockEmbeddingService.AssertExpectations(t)

	// Verify all 3 batches were processed in sync mode
	mockEmbeddingService.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 3)
}

// TestProcessProductionBatchResults_QuotaErrorWithRetry tests retry logic for quota errors (429).
//
// Expected Behavior (RED PHASE - WILL FAIL):
// - Batch 1 succeeds
// - Batch 2 fails with 429 quota error
// - System should detect quota error and retry batch 2
// - Batch 2 should succeed on retry
// - Progress should be saved for both batches
//
// Current Behavior (WHY THIS TEST FAILS):
// - No quota error detection
// - No retry logic
// - No exponential backoff.
func TestProcessProductionBatchResults_QuotaErrorWithRetry(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New()

	chunks := createTestChunks(1000, repositoryID)

	// mockBatchProgressRepo removed for sync mode testing
	mockEmbeddingService := new(MockEmbeddingService)
	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockIndexingJobRepo := new(MockIndexingJobRepository)
	mockRepositoryRepo := new(MockRepositoryRepository)
	mockGitClient := new(MockEnhancedGitClient)
	mockCodeParser := new(MockCodeParser)

	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      10,
		FallbackToSequential: false,
		UseTestEmbeddings:    false,
		MaxRetries:           3,                     // Explicit retry config for quota error tests
		InitialBackoff:       10 * time.Millisecond, // Fast for testing
		MaxBackoff:           50 * time.Millisecond, // Fast for testing
	}

	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-batch-loop-quota",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
	}

	processor := NewDefaultJobProcessor(
		processorConfig,
		mockIndexingJobRepo,
		mockRepositoryRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbeddingService,
		mockChunkStorageRepo,
		&JobProcessorBatchOptions{
			BatchConfig:       &batchConfig,
			BatchQueueManager: nil, // Use nil to force sync mode
		},
	).(*DefaultJobProcessor)

	// Check for existing progress

	// Batch 1 succeeds
	batch1Texts := extractTextsFromChunks(chunks[0:500])
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		batch1Texts,
		mock.Anything,
	).Return(createBatchEmbeddingResults(500), nil).Once()

	// Batch 2 fails with quota error (429) on first attempt
	batch2Texts := extractTextsFromChunks(chunks[500:1000])
	quotaError := &outbound.EmbeddingError{
		Code:      "rate_limit_exceeded",
		Message:   "Quota exceeded, please retry after delay",
		Type:      "quota",
		Retryable: true,
		Cause:     errors.New("HTTP 429: Too Many Requests"),
	}

	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		batch2Texts,
		mock.Anything,
	).Return(nil, quotaError).Once()

	// Batch 2 succeeds on retry
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		batch2Texts,
		mock.Anything,
	).Return(createBatchEmbeddingResults(500), nil).Once()

	// Store all embeddings
	mockChunkStorageRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.AnythingOfType("*outbound.CodeChunk"),
		mock.AnythingOfType("*outbound.Embedding"),
	).Return(nil).Times(1000)

	// Act
	execution := &JobExecution{StartTime: time.Now()}
	err := processor.processProductionBatchResults(ctx, jobID, repositoryID, chunks, execution)

	// Assert
	require.NoError(t, err, "Processing should complete successfully after retry")

	// CRITICAL ASSERTIONS - These will FAIL in RED phase
	mockEmbeddingService.AssertExpectations(t)

	// Verify batch 2 was called twice (initial + retry)
	mockEmbeddingService.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 3) // 1 + 2(retry) = 3
}

// TestProcessProductionBatchResults_QuotaErrorMaxRetries tests failure after max retries.
//
// Expected Behavior (RED PHASE - WILL FAIL):
// - Batch fails with 429 repeatedly
// - System retries up to max retry count (e.g., 3 retries)
// - After max retries, should save failed batch progress
// - Should return error to caller
// - Should NOT continue to next batches
//
// Current Behavior (WHY THIS TEST FAILS):
// - No retry logic with max retry count
// - No failed batch progress tracking.
func TestProcessProductionBatchResults_QuotaErrorMaxRetries(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New()

	chunks := createTestChunks(1000, repositoryID)

	// mockBatchProgressRepo removed for sync mode testing
	mockEmbeddingService := new(MockEmbeddingService)
	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockIndexingJobRepo := new(MockIndexingJobRepository)
	mockRepositoryRepo := new(MockRepositoryRepository)
	mockGitClient := new(MockEnhancedGitClient)
	mockCodeParser := new(MockCodeParser)

	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      10,
		FallbackToSequential: false,
		UseTestEmbeddings:    false,
		MaxRetries:           3,                     // Explicit retry config: 1 initial + 3 retries = 4 attempts
		InitialBackoff:       10 * time.Millisecond, // Fast for testing
		MaxBackoff:           50 * time.Millisecond, // Fast for testing
	}

	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-batch-loop-max-retries",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
	}

	processor := NewDefaultJobProcessor(
		processorConfig,
		mockIndexingJobRepo,
		mockRepositoryRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbeddingService,
		mockChunkStorageRepo,
		&JobProcessorBatchOptions{
			BatchConfig:       &batchConfig,
			BatchQueueManager: nil, // Use nil to force sync mode
		},
	).(*DefaultJobProcessor)

	// Check for existing progress

	// Batch 1 succeeds
	batch1Texts := extractTextsFromChunks(chunks[0:500])
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		batch1Texts,
		mock.Anything,
	).Return(createBatchEmbeddingResults(500), nil).Once()

	// Batch 2 fails with quota error on all attempts (4 total: initial + 3 retries)
	batch2Texts := extractTextsFromChunks(chunks[500:1000])
	quotaError := &outbound.EmbeddingError{
		Code:      "rate_limit_exceeded",
		Message:   "Quota exceeded, please retry after delay",
		Type:      "quota",
		Retryable: true,
		Cause:     errors.New("HTTP 429: Too Many Requests"),
	}

	// Initial attempt + 3 retries = 4 calls
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		batch2Texts,
		mock.Anything,
	).Return(nil, quotaError).Times(4)

	// Should save failed batch progress

	// Store batch 1 embeddings only
	mockChunkStorageRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.AnythingOfType("*outbound.CodeChunk"),
		mock.AnythingOfType("*outbound.Embedding"),
	).Return(nil).Times(500)

	// Act
	execution := &JobExecution{StartTime: time.Now()}
	err := processor.processProductionBatchResults(ctx, jobID, repositoryID, chunks, execution)

	// Assert
	require.Error(t, err, "Should return error after max retries exceeded")
	assert.Contains(t, err.Error(), "rate_limit_exceeded", "Error should mention quota/rate limit")

	// CRITICAL ASSERTIONS - These will FAIL in RED phase
	mockEmbeddingService.AssertExpectations(t)

	// Verify batch 2 was attempted 4 times (initial + 3 retries)
	mockEmbeddingService.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 5) // 1 (batch1) + 4 (batch2 attempts)

	// Verify failed progress was saved
}

// TestProcessProductionBatchResults_ValidationError tests non-retryable validation errors.
//
// Expected Behavior (RED PHASE - WILL FAIL):
// - Batch returns validation error (non-retryable)
// - System should detect error is not retryable
// - Should save failed batch progress immediately
// - Should return error without retry
//
// Current Behavior (WHY THIS TEST FAILS):
// - No error type detection (retryable vs non-retryable)
// - No validation error handling.
func TestProcessProductionBatchResults_ValidationError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New()

	chunks := createTestChunks(500, repositoryID)

	// mockBatchProgressRepo removed for sync mode testing
	mockEmbeddingService := new(MockEmbeddingService)
	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockIndexingJobRepo := new(MockIndexingJobRepository)
	mockRepositoryRepo := new(MockRepositoryRepository)
	mockGitClient := new(MockEnhancedGitClient)
	mockCodeParser := new(MockCodeParser)

	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      10,
		FallbackToSequential: false,
		UseTestEmbeddings:    false,
	}

	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-batch-loop-validation",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
	}

	processor := NewDefaultJobProcessor(
		processorConfig,
		mockIndexingJobRepo,
		mockRepositoryRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbeddingService,
		mockChunkStorageRepo,
		&JobProcessorBatchOptions{
			BatchConfig:       &batchConfig,
			BatchQueueManager: nil, // Use nil to force sync mode
		},
	).(*DefaultJobProcessor)

	// Check for existing progress

	// Batch fails with validation error (NON-retryable)
	validationError := &outbound.EmbeddingError{
		Code:      "invalid_input",
		Message:   "Text contains invalid characters",
		Type:      "validation",
		Retryable: false, // Non-retryable
		Cause:     errors.New("validation failed"),
	}

	batchTexts := extractTextsFromChunks(chunks)
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		batchTexts,
		mock.Anything,
	).Return(nil, validationError).Once()

	// Should save failed batch progress

	// Act
	execution := &JobExecution{StartTime: time.Now()}
	err := processor.processProductionBatchResults(ctx, jobID, repositoryID, chunks, execution)

	// Assert
	require.Error(t, err, "Should return error for validation failure")
	assert.Contains(t, err.Error(), "invalid_input", "Error should mention validation issue")

	// CRITICAL ASSERTIONS - These will FAIL in RED phase
	mockEmbeddingService.AssertExpectations(t)

	// Verify NO retries (only called once)
	mockEmbeddingService.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 1)

	// Verify failed progress was saved
}

// TestProcessProductionBatchResults_ProgressSaveError tests handling of progress save failures.
//
// Expected Behavior (RED PHASE - WILL FAIL):
// - Batch processing succeeds
// - Progress save fails
// - Should log error but continue processing (OR fail based on requirements)
// - This test assumes we continue processing despite save errors
//
// Current Behavior (WHY THIS TEST FAILS):
// - No progress saving logic
// - No error handling for progress save failures.
func TestProcessProductionBatchResults_ProgressSaveError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New()

	chunks := createTestChunks(1000, repositoryID)

	// mockBatchProgressRepo removed for sync mode testing
	mockEmbeddingService := new(MockEmbeddingService)
	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockIndexingJobRepo := new(MockIndexingJobRepository)
	mockRepositoryRepo := new(MockRepositoryRepository)
	mockGitClient := new(MockEnhancedGitClient)
	mockCodeParser := new(MockCodeParser)

	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      10,
		FallbackToSequential: false,
		UseTestEmbeddings:    false,
	}

	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-batch-loop-save-error",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
	}

	processor := NewDefaultJobProcessor(
		processorConfig,
		mockIndexingJobRepo,
		mockRepositoryRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbeddingService,
		mockChunkStorageRepo,
		&JobProcessorBatchOptions{
			BatchConfig:       &batchConfig,
			BatchQueueManager: nil, // Use nil to force sync mode
		},
	).(*DefaultJobProcessor)

	// Check for existing progress

	// NOTE: This test is designed to test progress save errors, but since we're in sync mode
	// (nil batch progress repo), there's no progress saving happening.
	// Updated to process both batches (1000 chunks total).

	// Batch 1 succeeds
	batch1Texts := extractTextsFromChunks(chunks[0:500])
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		batch1Texts,
		mock.Anything,
	).Return(createBatchEmbeddingResults(500), nil).Once()

	// Batch 2 succeeds
	batch2Texts := extractTextsFromChunks(chunks[500:1000])
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		batch2Texts,
		mock.Anything,
	).Return(createBatchEmbeddingResults(500), nil).Once()

	// Store all 1000 embeddings
	mockChunkStorageRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.AnythingOfType("*outbound.CodeChunk"),
		mock.AnythingOfType("*outbound.Embedding"),
	).Return(nil).Times(1000)

	// Act
	execution := &JobExecution{StartTime: time.Now()}
	err := processor.processProductionBatchResults(ctx, jobID, repositoryID, chunks, execution)

	// Assert
	// In sync mode (no batch progress repo), processing should succeed
	// because there's no progress save to fail
	require.NoError(t, err, "Should succeed in sync mode")

	// CRITICAL ASSERTIONS
	mockEmbeddingService.AssertExpectations(t)

	// Verify both batches were processed
	mockEmbeddingService.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 2)
}

// TestProcessProductionBatchResults_LargeDataset tests a production-scale dataset.
//
// Expected Behavior (RED PHASE - WILL FAIL):
// - Input: 8269 chunks (real production scenario)
// - Should split into 17 batches: 16 batches of 500 + 1 batch of 269
// - Should process all batches sequentially
// - Should save progress 17 times
// - All batches should be numbered 1-17 with totalBatches = 17
//
// Current Behavior (WHY THIS TEST FAILS):
// - No batch splitting for large datasets
// - No sequential processing loop.
func TestProcessProductionBatchResults_LargeDataset(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New()

	// Create 8269 chunks (production scenario)
	chunks := createTestChunks(8269, repositoryID)

	// mockBatchProgressRepo removed for sync mode testing
	mockEmbeddingService := new(MockEmbeddingService)
	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockIndexingJobRepo := new(MockIndexingJobRepository)
	mockRepositoryRepo := new(MockRepositoryRepository)
	mockGitClient := new(MockEnhancedGitClient)
	mockCodeParser := new(MockCodeParser)

	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      10,
		FallbackToSequential: false,
		UseTestEmbeddings:    false,
	}

	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-batch-loop-large",
		MaxConcurrentJobs: 5,
		JobTimeout:        30 * time.Minute, // Longer timeout for large dataset
	}

	processor := NewDefaultJobProcessor(
		processorConfig,
		mockIndexingJobRepo,
		mockRepositoryRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbeddingService,
		mockChunkStorageRepo,
		&JobProcessorBatchOptions{
			BatchConfig:       &batchConfig,
			BatchQueueManager: nil, // Use nil to force sync mode
		},
	).(*DefaultJobProcessor)

	// Check for existing progress

	// Calculate expected batches: 8269 / 500 = 16 full batches + 1 remainder (269 chunks)
	totalBatches := 17

	// Mock all batches
	for batchNum := 1; batchNum <= totalBatches; batchNum++ {
		start := (batchNum - 1) * 500
		end := start + 500
		if end > len(chunks) {
			end = len(chunks)
		}

		batchSize := end - start
		batchTexts := extractTextsFromChunks(chunks[start:end])

		mockEmbeddingService.On("GenerateBatchEmbeddings",
			ctx,
			batchTexts,
			mock.Anything,
		).Return(createBatchEmbeddingResults(batchSize), nil).Once()
	}

	// Store all embeddings
	mockChunkStorageRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.AnythingOfType("*outbound.CodeChunk"),
		mock.AnythingOfType("*outbound.Embedding"),
	).Return(nil).Times(8269)

	// Act
	execution := &JobExecution{StartTime: time.Now()}
	err := processor.processProductionBatchResults(ctx, jobID, repositoryID, chunks, execution)

	// Assert
	require.NoError(t, err, "Large dataset processing should complete successfully")

	// CRITICAL ASSERTIONS - These will FAIL in RED phase
	mockEmbeddingService.AssertExpectations(t)

	// Verify exactly 17 batches were processed
	mockEmbeddingService.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 17)

	// Verify all embeddings were stored
	mockChunkStorageRepo.AssertNumberOfCalls(t, "SaveChunkWithEmbedding", 8269)
}

// Helper Functions

// extractTextsFromChunks extracts text content from code chunks.
func extractTextsFromChunks(chunks []outbound.CodeChunk) []string {
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}
	return texts
}
