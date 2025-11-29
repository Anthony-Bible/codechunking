package worker

import (
	"codechunking/internal/config"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// ============================================================================
// COMPREHENSIVE ERROR HANDLING TESTS FOR BATCH EMBEDDING SYSTEM
// ============================================================================

// TestDefaultJobProcessor_ConfigurationErrors_NegativeThreshold tests when batch threshold is negative.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_ConfigurationErrors_NegativeThreshold() {
	// ARRANGE: Create processor with negative batch threshold
	invalidBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      -10, // Invalid: negative threshold
		FallbackToSequential: true,
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig: invalidBatchConfig,
	}

	// Create test chunks - should fail due to invalid configuration
	chunks := []outbound.CodeChunk{
		{
			ID:       "chunk-1",
			Content:  "test content",
			FilePath: "test.go",
		},
	}

	jobID := uuid.New() // negative-threshold-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// ACT & ASSERT: This should fail and return specific error about negative threshold
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	// EXPECT: Error should indicate invalid configuration (this will fail initially)
	suite.Error(err, "Should return error for negative batch threshold")
	suite.Contains(err.Error(), "negative", "Error message should mention negative threshold")
	suite.Contains(err.Error(), "threshold", "Error message should mention threshold")
}

// TestDefaultJobProcessor_ConfigurationErrors_ZeroThreshold tests when batch threshold is zero.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_ConfigurationErrors_ZeroThreshold() {
	// ARRANGE: Create processor with zero batch threshold
	invalidBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      0, // Invalid: zero threshold
		FallbackToSequential: true,
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig: invalidBatchConfig,
	}

	chunks := []outbound.CodeChunk{
		{
			ID:       "chunk-1",
			Content:  "test content",
			FilePath: "test.go",
		},
	}

	jobID := uuid.New() // zero-threshold-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// ACT & ASSERT: Should fail with error (service not available or invalid config)
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.Error(err, "Should return error when processor is not fully configured")
	suite.Contains(err.Error(), "embedding service", "Error should relate to embedding service availability")
}

// TestDefaultJobProcessor_BatchQueueManagerErrors_NilManager tests when BatchQueueManager is nil.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_BatchQueueManagerErrors_NilManager() {
	// ARRANGE: Create processor with valid config but nil BatchQueueManager
	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: true,
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: nil, // Critical error: nil manager
	}

	// Create chunks that would normally trigger batch processing
	chunks := []outbound.CodeChunk{
		{
			ID:       "chunk-1",
			Content:  "test content",
			FilePath: "test.go",
		},
	}

	jobID := uuid.New() // nil-manager-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// ACT & ASSERT: Should fail with specific error about nil BatchQueueManager
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.Error(err, "Should return error for nil BatchQueueManager")
	suite.Contains(err.Error(), "not available", "Error should indicate BatchQueueManager not available")
}

// TestDefaultJobProcessor_BatchQueueManagerErrors_QueueFull tests when queue submission fails due to full queue.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_BatchQueueManagerErrors_QueueFull() {
	// ARRANGE: Create processor with BatchQueueManager that returns queue full error
	mockBatchQueueManager := &MockBatchQueueManager{}

	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: true, // Should attempt fallback
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: mockBatchQueueManager,
	}

	chunks := []outbound.CodeChunk{
		{
			ID:       "chunk-1",
			Content:  "test content",
			FilePath: "test.go",
		},
	}

	jobID := uuid.New() // queue-full-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// EXPECT: Mock queue submission fails with queue full error
	queueFullError := errors.New("queue is full")
	mockBatchQueueManager.On("QueueBulkEmbeddingRequests", ctx, mock.AnythingOfType("[]*outbound.EmbeddingRequest")).
		Return(queueFullError).Once()

	// ACT & ASSERT: Should attempt fallback since it's enabled
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	// This should initially fail (method doesn't exist), but we expect fallback behavior when implemented
	suite.Error(err, "Should handle queue full error")

	mockBatchQueueManager.AssertExpectations(suite.T())
}

// TestDefaultJobProcessor_BatchQueueManagerErrors_QueueSubmissionTimeout tests when queue submission times out.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_BatchQueueManagerErrors_QueueSubmissionTimeout() {
	// ARRANGE: Create processor with BatchQueueManager that times out
	mockBatchQueueManager := &MockBatchQueueManager{}

	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: false, // Fallback disabled to test error propagation
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  1 * time.Millisecond, // Very short timeout
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: mockBatchQueueManager,
	}

	chunks := []outbound.CodeChunk{
		{
			ID:       "chunk-1",
			Content:  "test content",
			FilePath: "test.go",
		},
	}

	jobID := uuid.New() // timeout-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// EXPECT: Mock queue submission times out
	timeoutError := errors.New("context deadline exceeded")
	mockBatchQueueManager.On("QueueBulkEmbeddingRequests", ctx, mock.AnythingOfType("[]*outbound.EmbeddingRequest")).
		Return(timeoutError).Once()

	// ACT & ASSERT: Should fail with timeout error since fallback is disabled
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.Error(err, "Should fail with timeout error when fallback disabled")
	suite.Contains(err.Error(), "batch processing failed", "Error should indicate batch processing failure")

	mockBatchQueueManager.AssertExpectations(suite.T())
}

// TestDefaultJobProcessor_BatchQueueManagerErrors_NetworkConnectivity tests network connectivity failures.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_BatchQueueManagerErrors_NetworkConnectivity() {
	// ARRANGE: Create processor with BatchQueueManager that experiences network issues
	mockBatchQueueManager := &MockBatchQueueManager{}

	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: true, // Fallback enabled
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: mockBatchQueueManager,
	}

	chunks := []outbound.CodeChunk{
		{
			ID:       "chunk-1",
			Content:  "test content",
			FilePath: "test.go",
		},
	}

	jobID := uuid.New() // network-error-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// EXPECT: Mock queue submission fails with network error
	networkError := errors.New("connection refused: NATS server unavailable")
	mockBatchQueueManager.On("QueueBulkEmbeddingRequests", ctx, mock.AnythingOfType("[]*outbound.EmbeddingRequest")).
		Return(networkError).Once()

	// ACT & ASSERT: Should attempt fallback when network error occurs
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.Error(err, "Should handle network error with fallback")

	mockBatchQueueManager.AssertExpectations(suite.T())
}

// TestDefaultJobProcessor_BatchQueueManagerErrors_PartialBatchFailure tests partial batch processing failures.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_BatchQueueManagerErrors_PartialBatchFailure() {
	// ARRANGE: Create processor with BatchQueueManager that partially fails
	mockBatchQueueManager := &MockBatchQueueManager{}

	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: false, // No fallback to test partial failure handling
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: mockBatchQueueManager,
	}

	// Create multiple chunks to test partial failure scenarios
	chunks := []outbound.CodeChunk{
		{ID: "chunk-1", Content: "content 1", FilePath: "file1.go"},
		{ID: "chunk-2", Content: "content 2", FilePath: "file2.go"},
		{ID: "chunk-3", Content: "content 3", FilePath: "file3.go"},
	}

	jobID := uuid.New() // partial-failure-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// EXPECT: Mock queue submission fails with partial batch error
	partialError := errors.New("partial batch failure: 1 of 3 requests failed")
	mockBatchQueueManager.On("QueueBulkEmbeddingRequests", ctx, mock.AnythingOfType("[]*outbound.EmbeddingRequest")).
		Return(partialError).Once()

	// ACT & ASSERT: Should handle partial batch failure appropriately
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.Error(err, "Should handle partial batch failure")
	suite.Contains(err.Error(), "partial batch failure", "Error should mention partial batch failure")

	mockBatchQueueManager.AssertExpectations(suite.T())
}

// ============================================================================
// DATA CONVERSION ERROR TESTS
// ============================================================================

// TestDefaultJobProcessor_DataConversionErrors_NilChunkContent tests when chunk content is nil.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_DataConversionErrors_NilChunkContent() {
	// ARRANGE: Create processor with valid configuration
	mockBatchQueueManager := &MockBatchQueueManager{}

	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: false,
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: mockBatchQueueManager,
	}

	// Create chunk with nil content - should fail validation
	chunks := []outbound.CodeChunk{
		{
			ID:       "chunk-1",
			Content:  "", // Nil/empty content
			FilePath: "test.go",
		},
	}

	jobID := uuid.New() // nil-content-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// ACT & ASSERT: Should fail during chunk validation before queue submission
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.Error(err, "Should return error for nil chunk content")
	suite.Contains(err.Error(), "content", "Error message should mention content")
	suite.Contains(err.Error(), "empty", "Error message should mention empty/nil content")

	// Verify queue was never called due to validation failure
	mockBatchQueueManager.AssertNotCalled(suite.T(), "QueueBulkEmbeddingRequests")
}

// TestDefaultJobProcessor_DataConversionErrors_MissingChunkID tests when chunk ID is missing/empty.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_DataConversionErrors_MissingChunkID() {
	// ARRANGE: Create processor with valid configuration
	mockBatchQueueManager := &MockBatchQueueManager{}

	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: false,
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: mockBatchQueueManager,
	}

	// Create chunk with missing/empty ID - should fail validation
	chunks := []outbound.CodeChunk{
		{
			ID:       "", // Empty ID - should fail validation
			Content:  "test content",
			FilePath: "test.go",
		},
	}

	jobID := uuid.New() // missing-id-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// ACT & ASSERT: Should fail during chunk ID validation
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.Error(err, "Should return error for missing chunk ID")
	suite.Contains(err.Error(), "ID", "Error message should mention ID")
	suite.Contains(err.Error(), "empty", "Error message should mention empty ID")

	mockBatchQueueManager.AssertNotCalled(suite.T(), "QueueBulkEmbeddingRequests")
}

// TestDefaultJobProcessor_DataConversionErrors_MissingFilePath tests when chunk file path is missing.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_DataConversionErrors_MissingFilePath() {
	// ARRANGE: Create processor with valid configuration
	mockBatchQueueManager := &MockBatchQueueManager{}

	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: false,
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: mockBatchQueueManager,
	}

	// Create chunk with missing file path - should fail validation
	chunks := []outbound.CodeChunk{
		{
			ID:       "chunk-1",
			Content:  "test content",
			FilePath: "", // Empty file path - should fail validation
		},
	}

	jobID := uuid.New() // missing-path-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// ACT & ASSERT: Should fail during chunk path validation
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.Error(err, "Should return error for missing file path")
	suite.Contains(err.Error(), "path", "Error message should mention path")
	suite.Contains(err.Error(), "empty", "Error message should mention empty path")

	mockBatchQueueManager.AssertNotCalled(suite.T(), "QueueBulkEmbeddingRequests")
}

// ============================================================================
// STORAGE AND PERSISTENCE ERROR TESTS
// ============================================================================

// TestDefaultJobProcessor_StorageErrors_DatabaseConnectionFailure tests database connection failures during storage.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_StorageErrors_DatabaseConnectionFailure() {
	// ARRANGE: Create processor with mocks
	mockBatchQueueManager := &MockBatchQueueManager{}
	mockChunkStorageRepository := &MockChunkStorageRepository{}

	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: false,
		UseTestEmbeddings:    true, // Use test mode to bypass batch API and test storage layer
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: mockBatchQueueManager,
		chunkStorageRepo:  mockChunkStorageRepository,
	}

	chunks := []outbound.CodeChunk{
		{ID: "chunk-1", Content: "test content 1", FilePath: "file1.go"},
		{ID: "chunk-2", Content: "test content 2", FilePath: "file2.go"},
	}

	jobID := uuid.New() // db-failure-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// EXPECT: Batch submission succeeds but storage fails
	mockBatchQueueManager.On("QueueBulkEmbeddingRequests", mock.Anything, mock.AnythingOfType("[]*outbound.EmbeddingRequest")).
		Return(nil).
		Once()

	dbError := errors.New("database connection failed: connection refused")
	// Mock expects separate chunk and embedding parameters (actual interface signature)
	// Use mock.Anything for context as the actual context may be derived with timeout
	mockChunkStorageRepository.On("SaveChunkWithEmbedding", mock.Anything, mock.AnythingOfType("*outbound.CodeChunk"), mock.AnythingOfType("*outbound.Embedding")).
		Return(dbError)

	// ACT & ASSERT: Should continue processing despite database connection failures
	// The resilient design logs errors but doesn't fail the entire batch for transient database issues
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.NoError(err, "Should continue processing despite database connection failures (resilient design)")

	mockBatchQueueManager.AssertExpectations(suite.T())
	mockChunkStorageRepository.AssertExpectations(suite.T())
}

// TestDefaultJobProcessor_StorageErrors_ConstraintViolation tests constraint violations during storage.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_StorageErrors_ConstraintViolation() {
	// ARRANGE: Create processor with mocks
	mockBatchQueueManager := &MockBatchQueueManager{}
	mockChunkStorageRepository := &MockChunkStorageRepository{}

	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: false,
		UseTestEmbeddings:    true, // Use test mode to bypass batch API and test storage layer
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: mockBatchQueueManager,
		chunkStorageRepo:  mockChunkStorageRepository,
	}

	chunks := []outbound.CodeChunk{
		{ID: "duplicate-chunk", Content: "duplicate content", FilePath: "duplicate.go"},
	}

	jobID := uuid.New() // constraint-violation-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// EXPECT: Batch submission succeeds but storage fails due to constraint violation
	mockBatchQueueManager.On("QueueBulkEmbeddingRequests", mock.Anything, mock.AnythingOfType("[]*outbound.EmbeddingRequest")).
		Return(nil).
		Once()

	constraintError := errors.New("constraint violation: duplicate key value violates unique constraint")
	// Mock expects separate chunk and embedding parameters (actual interface signature)
	// Use mock.Anything for context as the actual context may be derived with timeout
	mockChunkStorageRepository.On("SaveChunkWithEmbedding", mock.Anything, mock.AnythingOfType("*outbound.CodeChunk"), mock.AnythingOfType("*outbound.Embedding")).
		Return(constraintError)

	// ACT & ASSERT: Should fail when constraint violation occurs
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.Error(err, "Should return error for constraint violation")
	suite.Contains(err.Error(), "constraint", "Error message should mention constraint")
	suite.Contains(err.Error(), "violation", "Error message should mention violation")

	mockBatchQueueManager.AssertExpectations(suite.T())
	// Note: SaveChunkWithEmbedding may not be called if batch submission fails first
}

// ============================================================================
// FALLBACK AND RECOVERY ERROR TESTS
// ============================================================================

// TestDefaultJobProcessor_FallbackErrors_FallbackDisabled tests when fallback is disabled and batch fails.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_FallbackErrors_FallbackDisabled() {
	// ARRANGE: Create processor with fallback disabled
	mockBatchQueueManager := &MockBatchQueueManager{}
	mockEmbeddingService := &MockEmbeddingService{}

	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: false, // Fallback explicitly disabled
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: mockBatchQueueManager,
		embeddingService:  mockEmbeddingService,
	}

	chunks := []outbound.CodeChunk{
		{ID: "chunk-1", Content: "test content", FilePath: "test.go"},
	}

	jobID := uuid.New() // no-fallback-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// EXPECT: Batch submission fails and fallback is not attempted
	batchError := errors.New("queue submission failed")
	mockBatchQueueManager.On("QueueBulkEmbeddingRequests", ctx, mock.AnythingOfType("[]*outbound.EmbeddingRequest")).
		Return(batchError).Once()

	// ACT & ASSERT: Should fail and not attempt sequential fallback
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.Error(err, "Should fail when batch processing fails and fallback is disabled")
	suite.Contains(err.Error(), "batch processing failed", "Error should indicate batch processing failure")

	// Verify sequential processing was not attempted
	mockEmbeddingService.AssertNotCalled(suite.T(), "GenerateEmbedding")
	mockEmbeddingService.AssertNotCalled(suite.T(), "GenerateBatchEmbeddings")

	mockBatchQueueManager.AssertExpectations(suite.T())
}

// TestDefaultJobProcessor_FallbackErrors_SequentialAlsoFails tests when fallback sequential processing also fails.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_FallbackErrors_SequentialAlsoFails() {
	// ARRANGE: Create processor with fallback enabled
	mockBatchQueueManager := &MockBatchQueueManager{}
	mockEmbeddingService := &MockEmbeddingService{}

	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: true, // Fallback enabled
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: mockBatchQueueManager,
		embeddingService:  mockEmbeddingService,
	}

	chunks := []outbound.CodeChunk{
		{ID: "chunk-1", Content: "test content", FilePath: "test.go"},
	}

	jobID := uuid.New() // fallback-also-fails-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// EXPECT: Batch submission fails and fallback sequential also fails
	batchError := errors.New("queue service temporarily unavailable")
	mockBatchQueueManager.On("QueueBulkEmbeddingRequests", ctx, mock.AnythingOfType("[]*outbound.EmbeddingRequest")).
		Return(batchError).Once()

	// Sequential fallback will call GenerateEmbedding for each chunk, mock it to fail
	sequentialError := errors.New("embedding generation failed: API error")
	mockEmbeddingService.On("GenerateEmbedding", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("outbound.EmbeddingOptions")).
		Return(nil, sequentialError).
		Once()

	// ACT & ASSERT: Should attempt fallback but both batch and sequential fail
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.Error(err, "Should fail when both batch and sequential fallback fail")

	mockBatchQueueManager.AssertExpectations(suite.T())
	mockEmbeddingService.AssertExpectations(suite.T())
}

// ============================================================================
// CONTEXT AND TIMEOUT ERROR TESTS
// ============================================================================

// TestDefaultJobProcessor_ContextErrors_ContextCancellation tests context cancellation during processing.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_ContextErrors_ContextCancellation() {
	// ARRANGE: Create processor with mocks
	mockBatchQueueManager := &MockBatchQueueManager{}

	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: false,
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: mockBatchQueueManager,
	}

	chunks := []outbound.CodeChunk{
		{ID: "chunk-1", Content: "test content", FilePath: "test.go"},
	}

	jobID := uuid.New() // context-cancellation-test
	repositoryID := uuid.New()

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context immediately to simulate cancellation
	cancel()

	// ACT & ASSERT: Should fail due to cancelled context
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.Error(err, "Should return error for cancelled context")
	suite.Contains(err.Error(), "context", "Error message should mention context")
	suite.Contains(err.Error(), "cancelled", "Error message should mention cancellation")

	// Mock should not be called since context is already cancelled
	mockBatchQueueManager.AssertNotCalled(suite.T(), "QueueBulkEmbeddingRequests")
}

// TestDefaultJobProcessor_ContextErrors_TimeoutDuringBatchSubmission tests timeout during batch submission.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_ContextErrors_TimeoutDuringBatchSubmission() {
	// ARRANGE: Create processor with mocks
	mockBatchQueueManager := &MockBatchQueueManager{}

	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: false,
		UseTestEmbeddings:    true, // Use test mode to test batch queue timeout
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: mockBatchQueueManager,
	}

	chunks := []outbound.CodeChunk{
		{ID: "chunk-1", Content: "test content", FilePath: "test.go"},
	}

	jobID := uuid.New() // timeout-batch-test
	repositoryID := uuid.New()

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Mock batch submission that might take longer than the timeout
	mockBatchQueueManager.On("QueueBulkEmbeddingRequests", mock.Anything, mock.AnythingOfType("[]*outbound.EmbeddingRequest")).
		Return(nil).
		After(100 * time.Millisecond)

		// Longer than context timeout

	// ACT & ASSERT: Should fail due to context timeout
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.Error(err, "Should return error for context timeout")
	suite.Contains(err.Error(), "context", "Error message should mention context")
	// The error may say either "timeout" or "deadline exceeded"
	suite.True(
		strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded"),
		"Error message should mention timeout or deadline: got %s", err.Error())

	mockBatchQueueManager.AssertExpectations(suite.T())
}

// ============================================================================
// RESOURCE MANAGEMENT ERROR TESTS
// ============================================================================

// TestDefaultJobProcessor_ResourceErrors_MemoryExhaustion tests memory exhaustion during large batch processing.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_ResourceErrors_MemoryExhaustion() {
	// ARRANGE: Create processor with mocks
	mockBatchQueueManager := &MockBatchQueueManager{}

	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: false,
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: mockBatchQueueManager,
	}

	// Create a large number of chunks with substantial content to simulate memory pressure
	chunks := make([]outbound.CodeChunk, 1000)    // Large batch that could cause memory issues
	largeContent := string(make([]byte, 10*1024)) // 10KB per chunk

	for i := range 1000 {
		chunks[i] = outbound.CodeChunk{
			ID:       fmt.Sprintf("large-chunk-%d", i),
			Content:  largeContent,
			FilePath: fmt.Sprintf("large_file_%d.go", i),
		}
	}

	jobID := uuid.New() // memory-exhaustion-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// EXPECT: Batch submission fails due to memory exhaustion
	memoryError := errors.New("cannot allocate memory: out of memory")
	mockBatchQueueManager.On("QueueBulkEmbeddingRequests", ctx, mock.AnythingOfType("[]*outbound.EmbeddingRequest")).
		Return(memoryError).Once()

	// ACT & ASSERT: Should fail with memory-related error
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.Error(err, "Should return error for memory exhaustion")
	suite.Contains(err.Error(), "memory", "Error message should mention memory")

	mockBatchQueueManager.AssertExpectations(suite.T())
}

// TestDefaultJobProcessor_ResourceErrors_ConnectionPoolExhaustion tests connection pool exhaustion.
func (suite *JobProcessorTestSuite) TestDefaultJobProcessor_ResourceErrors_ConnectionPoolExhaustion() {
	// ARRANGE: Create processor with mocks
	mockBatchQueueManager := &MockBatchQueueManager{}

	validBatchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		FallbackToSequential: false,
		QueueLimits: config.QueueLimitsConfig{
			MaxQueueSize: 1000,
			MaxWaitTime:  30 * time.Second,
		},
	}

	processor := &DefaultJobProcessor{
		batchConfig:       validBatchConfig,
		batchQueueManager: mockBatchQueueManager,
	}

	chunks := []outbound.CodeChunk{
		{ID: "chunk-1", Content: "test content", FilePath: "test.go"},
	}

	jobID := uuid.New() // connection-pool-exhaustion-test
	repositoryID := uuid.New()
	ctx := context.Background()

	// EXPECT: Batch submission fails due to connection pool exhaustion
	poolError := errors.New("connection pool exhausted: max connections reached")
	mockBatchQueueManager.On("QueueBulkEmbeddingRequests", ctx, mock.AnythingOfType("[]*outbound.EmbeddingRequest")).
		Return(poolError).Once()

	// ACT & ASSERT: Should fail with connection pool error
	err := processor.generateEmbeddingsWithBatch(ctx, jobID, repositoryID, chunks)

	suite.Error(err, "Should return error for connection pool exhaustion")
	suite.Contains(err.Error(), "connection", "Error message should mention connection")
	suite.Contains(err.Error(), "pool", "Error message should mention connection pool")

	mockBatchQueueManager.AssertExpectations(suite.T())
}
