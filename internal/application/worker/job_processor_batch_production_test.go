package worker

import (
	"codechunking/internal/config"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestProductionBatchProcessingCreatesRealBatchJob tests that when use_test_embeddings is false,
// the system calls GenerateBatchEmbeddings() on the embedding service instead of falling back
// to sequential processing.
//
// Expected Behavior (RED PHASE - CURRENTLY FAILS):
// - When use_test_embeddings = false
// - And batch processing is enabled
// - And chunk count exceeds threshold
// - THEN the system should call embeddingService.GenerateBatchEmbeddings()
// - AND should NOT fall back to sequential processing
//
// Current Behavior (WHY THIS TEST FAILS):
// - The processBatchResultsWithStorage() method logs "Production batch processing not implemented".
// - It falls back to sequential processing (line 979-986 in job_processor.go).
// - GenerateBatchEmbeddings() is never called.
func TestProductionBatchProcessingCreatesRealBatchJob(t *testing.T) {
	// Arrange: Create processor with production batch configuration
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New() // Use UUID string for job ID

	// Create test chunks that exceed the batch threshold
	chunks := createTestChunks(50, repositoryID) // 50 chunks > 10 threshold

	// Create mock repositories
	mockIndexingJobRepo := new(MockIndexingJobRepository)
	mockRepositoryRepo := new(MockRepositoryRepository)
	mockGitClient := new(MockEnhancedGitClient)
	mockCodeParser := new(MockCodeParser)
	mockEmbeddingService := new(MockEmbeddingService)
	mockChunkStorageRepo := new(MockChunkStorageRepository)

	// Configure batch processing for PRODUCTION mode
	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      10,    // Threshold for batch processing
		FallbackToSequential: true,  // Allow fallback on error
		UseTestEmbeddings:    false, // PRODUCTION MODE - use real embeddings
	}

	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-batch-workspace",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
	}

	// Create processor with batch configuration
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
			BatchQueueManager: nil, // No batch queue manager for this test
		},
	).(*DefaultJobProcessor)

	// Prepare test embeddings results for batch processing
	expectedBatchResults := createBatchEmbeddingResults(50)

	// Extract chunk contents for batch processing
	chunkTexts := make([]string, len(chunks))
	for i, chunk := range chunks {
		chunkTexts[i] = chunk.Content
	}

	// EXPECTATION: In production mode, the system should call GenerateBatchEmbeddings
	// This is the key assertion - the test FAILS because this is never called
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		chunkTexts,
		mock.MatchedBy(func(opts outbound.EmbeddingOptions) bool {
			return opts.Model == "gemini-embedding-001" &&
				opts.TaskType == outbound.TaskTypeRetrievalDocument
		}),
	).Return(expectedBatchResults, nil).Once()

	// Mock chunk storage to accept the embeddings
	mockChunkStorageRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.AnythingOfType("*outbound.CodeChunk"),
		mock.AnythingOfType("*outbound.Embedding"),
	).Return(nil)

	// Act: Generate embeddings (should use batch processing)
	err := processor.generateEmbeddings(ctx, jobID, repositoryID, chunks)

	// Assert: Verify batch processing was used
	require.NoError(t, err, "Batch processing should complete successfully")

	// CRITICAL ASSERTION - This will FAIL in RED phase
	// The mock should have been called, but it won't be because the code
	// currently falls back to sequential processing
	mockEmbeddingService.AssertExpectations(t)

	// Verify that GenerateBatchEmbeddings was called exactly once
	mockEmbeddingService.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 1)

	// Verify that GenerateEmbedding (sequential) was NOT called
	mockEmbeddingService.AssertNotCalled(t, "GenerateEmbedding", mock.Anything, mock.Anything, mock.Anything)
}

// TestProductionBatchProcessingStoresResultsCorrectly tests that batch embedding results
// are properly stored in the database with correct chunk IDs and embeddings.
//
// Expected Behavior (RED PHASE - CURRENTLY FAILS):
// - When batch embeddings are generated
// - AND batch returns 50 embedding results
// - THEN all 50 embeddings should be stored with correct chunk IDs
// - AND each embedding should have 768 dimensions (gemini-embedding-001)
// - AND repository ID should be correctly associated
//
// Current Behavior (WHY THIS TEST FAILS):
// - Batch processing is not implemented.
// - System falls back to sequential processing.
// - No batch storage logic exists.
func TestProductionBatchProcessingStoresResultsCorrectly(t *testing.T) {
	// Arrange: Create processor with production batch configuration
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New() // Use UUID string for job ID

	// Create exactly 50 test chunks
	chunks := createTestChunks(50, repositoryID)

	// Create mock repositories
	mockIndexingJobRepo := new(MockIndexingJobRepository)
	mockRepositoryRepo := new(MockRepositoryRepository)
	mockGitClient := new(MockEnhancedGitClient)
	mockCodeParser := new(MockCodeParser)
	mockEmbeddingService := new(MockEmbeddingService)
	mockChunkStorageRepo := new(MockChunkStorageRepository)

	// Configure batch processing for PRODUCTION mode
	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      10,
		FallbackToSequential: true,
		UseTestEmbeddings:    false, // PRODUCTION MODE
	}

	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-batch-storage-workspace",
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
			BatchQueueManager: nil,
		},
	).(*DefaultJobProcessor)

	// Prepare batch embedding results (50 embeddings)
	batchResults := createBatchEmbeddingResults(50)

	// Extract chunk texts
	chunkTexts := make([]string, len(chunks))
	for i, chunk := range chunks {
		chunkTexts[i] = chunk.Content
	}

	// Mock GenerateBatchEmbeddings to return our test results
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		chunkTexts,
		mock.Anything,
	).Return(batchResults, nil).Once()

	// Track storage calls to verify all 50 chunks are stored
	var storedChunks []*outbound.CodeChunk
	var storedEmbeddings []*outbound.Embedding

	mockChunkStorageRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.AnythingOfType("*outbound.CodeChunk"),
		mock.AnythingOfType("*outbound.Embedding"),
	).Run(func(args mock.Arguments) {
		chunk := args.Get(1).(*outbound.CodeChunk)
		embedding := args.Get(2).(*outbound.Embedding)
		storedChunks = append(storedChunks, chunk)
		storedEmbeddings = append(storedEmbeddings, embedding)
	}).Return(nil)

	// Act: Generate embeddings with batch processing
	err := processor.generateEmbeddings(ctx, jobID, repositoryID, chunks)

	// Assert: Verify all embeddings were stored correctly
	require.NoError(t, err, "Batch processing should complete successfully")

	// CRITICAL ASSERTIONS - These will FAIL in RED phase

	// Verify exactly 50 chunks were stored
	assert.Len(t, storedChunks, 50, "Should store exactly 50 chunks")
	assert.Len(t, storedEmbeddings, 50, "Should store exactly 50 embeddings")

	// Verify each embedding has correct dimensions
	for i := range storedEmbeddings {
		embedding := storedEmbeddings[i]
		assert.Len(t, embedding.Vector, 768,
			"Embedding %d should have 768 dimensions (gemini-embedding-001)", i)
		assert.Equal(t, repositoryID, embedding.RepositoryID,
			"Embedding %d should have correct repository ID", i)
		assert.Equal(t, "gemini-embedding-001", embedding.ModelVersion,
			"Embedding %d should use gemini-embedding-001 model", i)
	}

	// Verify chunk IDs match between chunks and embeddings
	for i := range storedChunks {
		chunkUUID, err := uuid.Parse(storedChunks[i].ID)
		require.NoError(t, err, "Chunk ID should be valid UUID")
		assert.Equal(t, chunkUUID, storedEmbeddings[i].ChunkID,
			"Embedding %d should reference correct chunk ID", i)
	}

	// Verify mocks were called as expected
	mockEmbeddingService.AssertExpectations(t)
	mockChunkStorageRepo.AssertExpectations(t)
}

// TestProductionBatchProcessingLogsBatchJobID tests that when batch processing runs,
// relevant information is logged including production mode indicator and job details.
//
// Expected Behavior (RED PHASE - CURRENTLY FAILS):
// - When batch processing is invoked in production mode
// - THEN logs should contain "PRODUCTION MODE" or similar indicator
// - AND logs should contain batch job ID or correlation ID
// - AND logs should contain chunk count
// - AND should NOT contain "falling back to sequential"
//
// Current Behavior (WHY THIS TEST FAILS):
// - Logs contain "Production batch processing not implemented - falling back to sequential".
// - No production mode indicator.
// - No batch job tracking.
func TestProductionBatchProcessingLogsBatchJobID(t *testing.T) {
	// Arrange: Create processor with production batch configuration
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New() // Use UUID string for job ID

	chunks := createTestChunks(50, repositoryID)

	mockIndexingJobRepo := new(MockIndexingJobRepository)
	mockRepositoryRepo := new(MockRepositoryRepository)
	mockGitClient := new(MockEnhancedGitClient)
	mockCodeParser := new(MockCodeParser)
	mockEmbeddingService := new(MockEmbeddingService)
	mockChunkStorageRepo := new(MockChunkStorageRepository)

	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      10,
		FallbackToSequential: true,
		UseTestEmbeddings:    false, // PRODUCTION MODE
	}

	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-batch-logging-workspace",
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
			BatchQueueManager: nil,
		},
	).(*DefaultJobProcessor)

	// Prepare batch results
	batchResults := createBatchEmbeddingResults(50)
	chunkTexts := make([]string, len(chunks))
	for i, chunk := range chunks {
		chunkTexts[i] = chunk.Content
	}

	// Mock batch embeddings
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		chunkTexts,
		mock.Anything,
	).Return(batchResults, nil).Once()

	// Mock storage
	mockChunkStorageRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.AnythingOfType("*outbound.CodeChunk"),
		mock.AnythingOfType("*outbound.Embedding"),
	).Return(nil)

	// Act: Generate embeddings
	err := processor.generateEmbeddings(ctx, jobID, repositoryID, chunks)

	// Assert: Verify successful completion
	require.NoError(t, err, "Batch processing should complete successfully")

	// CRITICAL ASSERTIONS - These will FAIL in RED phase
	// In the green phase, we would verify that:
	// 1. Logs contain "PRODUCTION MODE" indicator
	// 2. Logs contain the batch job ID
	// 3. Logs contain chunk count (50)
	// 4. Logs do NOT contain "falling back to sequential"
	//
	// For now, we verify that the batch processing path was taken by checking mock calls
	mockEmbeddingService.AssertExpectations(t)
	mockEmbeddingService.AssertCalled(t, "GenerateBatchEmbeddings", ctx, chunkTexts, mock.Anything)
}

// TestProductionBatchFallsBackOnAPIFailure tests that if GenerateBatchEmbeddings() fails,
// the system falls back to sequential processing when configured to do so.
//
// Expected Behavior (RED PHASE - CURRENTLY FAILS):
// - When GenerateBatchEmbeddings() returns an error
// - AND FallbackToSequential is true
// - THEN system should fall back to sequential processing
// - AND sequential processing should use GenerateEmbedding() calls
// - AND job should still complete successfully
//
// Current Behavior (WHY THIS TEST FAILS):
// - GenerateBatchEmbeddings is never called in the first place.
// - System goes directly to sequential processing.
// - No fallback logic exists.
func TestProductionBatchFallsBackOnAPIFailure(t *testing.T) {
	// Arrange: Create processor with production batch configuration
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New() // Use UUID string for job ID

	chunks := createTestChunks(50, repositoryID)

	mockIndexingJobRepo := new(MockIndexingJobRepository)
	mockRepositoryRepo := new(MockRepositoryRepository)
	mockGitClient := new(MockEnhancedGitClient)
	mockCodeParser := new(MockCodeParser)
	mockEmbeddingService := new(MockEmbeddingService)
	mockChunkStorageRepo := new(MockChunkStorageRepository)

	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      10,
		FallbackToSequential: true,  // Enable fallback
		UseTestEmbeddings:    false, // PRODUCTION MODE
	}

	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-batch-fallback-workspace",
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
			BatchQueueManager: nil,
		},
	).(*DefaultJobProcessor)

	// Extract chunk texts
	chunkTexts := make([]string, len(chunks))
	for i, chunk := range chunks {
		chunkTexts[i] = chunk.Content
	}

	// EXPECTATION 1: Batch processing should be attempted first and fail
	batchError := errors.New("batch API temporarily unavailable")
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		chunkTexts,
		mock.Anything,
	).Return(nil, batchError).Once()

	// EXPECTATION 2: After batch failure, should fall back to sequential processing
	// Mock sequential embedding calls (one per chunk)
	for _, chunk := range chunks {
		mockEmbeddingService.On("GenerateEmbedding",
			ctx,
			chunk.Content,
			mock.MatchedBy(func(opts outbound.EmbeddingOptions) bool {
				return opts.Model == "gemini-embedding-001"
			}),
		).Return(&outbound.EmbeddingResult{
			Vector:      make([]float64, 768),
			Dimensions:  768,
			TokenCount:  len(chunk.Content) / 4,
			Model:       "gemini-embedding-001",
			GeneratedAt: time.Now(),
		}, nil).Once()
	}

	// Mock storage for sequential processing
	mockChunkStorageRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.AnythingOfType("*outbound.CodeChunk"),
		mock.AnythingOfType("*outbound.Embedding"),
	).Return(nil)

	// Act: Generate embeddings (should try batch, then fall back to sequential)
	err := processor.generateEmbeddings(ctx, jobID, repositoryID, chunks)

	// Assert: Verify fallback behavior
	require.NoError(t, err, "Job should complete successfully even after batch failure")

	// CRITICAL ASSERTIONS - These will FAIL in RED phase

	// Verify batch was attempted first
	mockEmbeddingService.AssertCalled(t, "GenerateBatchEmbeddings", ctx, chunkTexts, mock.Anything)
	mockEmbeddingService.AssertNumberOfCalls(t, "GenerateBatchEmbeddings", 1)

	// Verify fallback to sequential processing occurred
	mockEmbeddingService.AssertNumberOfCalls(t, "GenerateEmbedding", 50)

	// Verify all expectations were met
	mockEmbeddingService.AssertExpectations(t)
	mockChunkStorageRepo.AssertExpectations(t)
}

// TestBatchConfigurationWithMissingDirectories tests that when batch processing is enabled
// but required directories are missing, the system handles it gracefully.
//
// Expected Behavior (RED PHASE - CURRENTLY FAILS):
//   - When batch.enabled = true in Gemini config
//   - AND batch input/output directories are not configured or missing
//   - THEN system should either:
//     a) Log a warning and fall back to sequential processing, OR
//     b) Create the directories automatically, OR
//     c) Return a clear configuration error
//   - AND should NOT panic or crash
//
// Current Behavior (WHY THIS TEST FAILS):
// - This test exercises Google Gemini Batches API integration
// - Current code doesn't validate directory configuration
// - No directory handling logic exists
//
// Note: This test validates configuration/initialization behavior, not runtime behavior.
func TestBatchConfigurationWithMissingDirectories(t *testing.T) {
	// Arrange: Create processor with batch enabled but directories not configured
	ctx := context.Background()
	repositoryID := uuid.New()
	jobID := uuid.New() // Use UUID string for job ID

	chunks := createTestChunks(50, repositoryID)

	mockIndexingJobRepo := new(MockIndexingJobRepository)
	mockRepositoryRepo := new(MockRepositoryRepository)
	mockGitClient := new(MockEnhancedGitClient)
	mockCodeParser := new(MockCodeParser)
	mockEmbeddingService := new(MockEmbeddingService)
	mockChunkStorageRepo := new(MockChunkStorageRepository)

	// Configure batch processing WITHOUT proper directory setup
	batchConfig := config.BatchProcessingConfig{
		Enabled:              true, // Enabled but directories missing
		ThresholdChunks:      10,
		FallbackToSequential: true,
		UseTestEmbeddings:    false, // PRODUCTION MODE
	}

	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-batch-config-workspace",
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
			BatchQueueManager: nil,
		},
	).(*DefaultJobProcessor)

	// Extract chunk texts
	chunkTexts := make([]string, len(chunks))
	for i, chunk := range chunks {
		chunkTexts[i] = chunk.Content
	}

	// EXPECTATION 1: If batch processing validates configuration at runtime,
	// it should detect missing directories and handle gracefully

	// Setup batch processing expectation (might be called if batch succeeds)
	batchResults := createBatchEmbeddingResults(50)
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		ctx,
		mock.Anything,
		mock.Anything,
	).Return(batchResults, nil).Maybe()

	// Setup fallback to sequential processing
	for _, chunk := range chunks {
		mockEmbeddingService.On("GenerateEmbedding",
			ctx,
			chunk.Content,
			mock.Anything,
		).Return(&outbound.EmbeddingResult{
			Vector:      make([]float64, 768),
			Dimensions:  768,
			TokenCount:  len(chunk.Content) / 4,
			Model:       "gemini-embedding-001",
			GeneratedAt: time.Now(),
		}, nil).Maybe() // Maybe() since it might not be called if batch succeeds
	}

	mockChunkStorageRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.AnythingOfType("*outbound.CodeChunk"),
		mock.AnythingOfType("*outbound.Embedding"),
	).Return(nil).Maybe()

	// Act: Attempt to generate embeddings
	err := processor.generateEmbeddings(ctx, jobID, repositoryID, chunks)

	// Assert: Should handle missing directories gracefully
	// In RED phase, this might fail because:
	// 1. No directory validation exists
	// 2. Batch processing not implemented
	// 3. No fallback logic for configuration issues

	// CRITICAL ASSERTION - This will FAIL in RED phase
	// The system should either:
	// 1. Complete successfully with a warning and fallback, OR
	// 2. Return a clear configuration error
	// It should NOT panic or crash
	assert.True(t,
		err == nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled),
		"Should handle missing directories gracefully (no panic), got error: %v", err)
}

// Helper Functions

// createTestChunks creates n test chunks with realistic content.
//

func createTestChunks(count int, repositoryID uuid.UUID) []outbound.CodeChunk {
	chunks := make([]outbound.CodeChunk, count)
	for i := range count {
		content := fmt.Sprintf("package main\n\nfunc TestFunction%d() {\n\t// Test code here\n\treturn\n}\n", i)
		chunks[i] = outbound.CodeChunk{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			FilePath:     fmt.Sprintf("/test/file_%d.go", i),
			Content:      content,
			Language:     "go",
			StartLine:    1,
			EndLine:      6,
			Size:         len(content),
			Hash:         fmt.Sprintf("hash_%d", i),
			CreatedAt:    time.Now(),
		}
	}
	return chunks
}

// createBatchEmbeddingResults creates n realistic embedding results for batch processing.
func createBatchEmbeddingResults(count int) []*outbound.EmbeddingResult {
	results := make([]*outbound.EmbeddingResult, count)
	for i := range count {
		// Create 768-dimensional vector (gemini-embedding-001 standard)
		vector := make([]float64, 768)
		for j := range 768 {
			// Create deterministic but varied values
			vector[j] = float64((i*768+j)%1000) / 1000.0
		}

		results[i] = &outbound.EmbeddingResult{
			Vector:      vector,
			Dimensions:  768,
			TokenCount:  50, // Typical token count
			Model:       "gemini-embedding-001",
			GeneratedAt: time.Now(),
		}
	}
	return results
}
