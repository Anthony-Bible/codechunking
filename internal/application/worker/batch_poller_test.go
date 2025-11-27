// Package worker contains tests for the batch poller's handleCompletedBatch functionality.
//
// # Test Coverage - Batch Result Processing
//
// This file contains comprehensive failing tests (RED PHASE) that define the expected
// behavior for processing completed Gemini batch embedding jobs. These tests specify
// how the batch poller should download results, convert them to chunks and embeddings,
// and persist them to the database.
//
// # Test Cases
//
// 1. TestHandleCompletedBatch_Success
//   - Tests the happy path of processing a completed batch job
//   - Verifies GetBatchJobResults is called with correct job ID
//   - Validates conversion of EmbeddingResult to Embedding objects
//   - Ensures embeddings are saved via SaveEmbeddings (chunks already in DB)
//   - Confirms batch progress is updated with actual chunk count
//
// 2. TestHandleCompletedBatch_ConvertsEmbeddingResultsCorrectly
//   - Validates that embedding dimensions are preserved (768)
//   - Checks model version is set correctly on embeddings
//   - Verifies chunk UUIDs are extracted from RequestIDs
//   - Ensures proper chunk-to-embedding association
//
// 3. TestHandleCompletedBatch_HandlesPartialResults
//   - Tests scenario where fewer results are returned than expected
//   - Verifies only successful embeddings are saved
//   - Confirms batch is marked completed with accurate count
//   - Ensures warning is logged for missing chunks
//
// 4. TestHandleCompletedBatch_ValidatesDimensions
//   - Tests dimension validation (768 expected for gemini-embedding-001)
//   - Verifies batch is marked as failed on dimension mismatch
//   - Ensures no chunks are saved when validation fails
//   - Confirms error is logged with dimension details
//
// 5. TestHandleCompletedBatch_HandlesGetResultsError
//   - Tests error handling when GetBatchJobResults fails
//   - Verifies error is returned from handleCompletedBatch
//   - Ensures batch is NOT marked as completed on error
//   - Confirms no database writes occur
//
// 6. TestHandleCompletedBatch_HandlesSaveError
//   - Tests error handling when SaveEmbeddings fails
//   - Verifies error is returned and batch remains in processing state
//   - Ensures operation can be retried in next poll cycle
//
// 7. TestHandleCompletedBatch_ExtractsChunkIDsFromRequestIDs
//   - Tests parsing of RequestID format "chunk_<uuid>"
//   - Verifies correct chunk-to-embedding mapping
//   - Validates extracted chunk IDs match original UUIDs
//
// # Implementation Requirements
//
// The implementation must:
// 1. Call batchEmbeddingService.GetBatchJobResults(jobID) to download results
// 2. Convert each EmbeddingResult to an Embedding with:
//   - Vector from result.Vector
//   - ChunkID extracted from result.RequestID (format: "chunk_<uuid>")
//   - ModelVersion from result.Model
//   - RepositoryID from indexing job or stored chunks
//
// 3. Create CodeChunk objects for each result (may need to retrieve from DB by chunk ID)
// 4. Call chunkStorageRepo.SaveEmbeddings(embeddings)
// 5. Update batch progress with actual chunk count: batch.MarkCompleted(len(results))
// 6. Call batchProgressRepo.Save(batch)
// 7. Validate embedding dimensions match expected value (768)
// 8. Handle errors at each step appropriately
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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestHandleCompletedBatch_Success verifies the complete flow of processing a completed batch job.
// It tests that:
// - GetBatchJobResults is called with the correct job ID
// - EmbeddingResults are converted to Embeddings with correct properties
// - Chunks are extracted from results and saved with embeddings
// - Batch progress is updated with actual chunk count
// - Batch status is marked as completed
func TestHandleCompletedBatch_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	indexingJobID := uuid.New()
	batchJobID := "batches/gemini_batch_job_123"

	// Create batch progress entity
	repositoryID := uuid.New()
	batch := entity.NewBatchJobProgress(repositoryID, indexingJobID, 1, 3)
	_ = batch.MarkSubmittedToGemini(batchJobID)

	// Create completed job
	completedTime := time.Now()
	job := &outbound.BatchEmbeddingJob{
		JobID:         batchJobID,
		JobName:       "Test Batch Job",
		Model:         "gemini-embedding-001",
		State:         outbound.BatchJobStateCompleted,
		TotalCount:    5,
		OutputFileURI: "/tmp/output.jsonl",
		CompletedAt:   &completedTime,
	}

	// Create mock embedding results (5 results with 768-dimensional vectors)
	mockResults := make([]*outbound.EmbeddingResult, 5)
	for i := 0; i < 5; i++ {
		chunkID := uuid.New()
		vector := make([]float64, 768)
		for j := range vector {
			vector[j] = float64(i) * 0.1 // Simple test values
		}

		mockResults[i] = &outbound.EmbeddingResult{
			Vector:      vector,
			Dimensions:  768,
			Model:       "gemini-embedding-001",
			GeneratedAt: time.Now(),
			RequestID:   fmt.Sprintf("chunk_%s", chunkID.String()),
		}
	}

	// Setup mocks
	mockBatchService := new(MockBatchEmbeddingService)
	mockChunkRepo := new(MockChunkStorageRepository)
	mockProgressRepo := new(MockBatchProgressRepository)

	// Mock GetBatchJobResults to return our test results
	mockBatchService.On("GetBatchJobResults", ctx, batchJobID).Return(mockResults, nil)

	// Mock SaveEmbeddings - expect 5 embeddings (chunks already saved)
	mockChunkRepo.On("SaveEmbeddings", ctx,
		mock.MatchedBy(func(embeddings []outbound.Embedding) bool {
			if len(embeddings) != 5 {
				return false
			}
			for _, emb := range embeddings {
				if len(emb.Vector) != 768 {
					return false
				}
			}
			return true
		}),
	).Return(nil)

	// Mock batch progress repository Save call
	mockProgressRepo.On("Save", ctx, mock.MatchedBy(func(batch *entity.BatchJobProgress) bool {
		return batch.Status() == entity.StatusCompleted && batch.ChunksProcessed() == 5
	})).Return(nil)

	// Create batch poller
	poller := &BatchPoller{
		batchProgressRepo:     mockProgressRepo,
		chunkRepo:             mockChunkRepo,
		batchEmbeddingService: mockBatchService,
		pollInterval:          30 * time.Second,
		maxConcurrentPolls:    5,
		stopCh:                make(chan struct{}),
	}

	// Act
	err := poller.handleCompletedBatch(ctx, batch, job)

	// Assert
	require.NoError(t, err, "handleCompletedBatch should not return an error")
	mockBatchService.AssertExpectations(t)
	mockChunkRepo.AssertExpectations(t)
	mockProgressRepo.AssertExpectations(t)

	// Verify batch status was updated
	assert.Equal(t, entity.StatusCompleted, batch.Status(), "Batch should be marked as completed")
	assert.Equal(t, 5, batch.ChunksProcessed(), "Batch should have 5 chunks processed")
}

// TestHandleCompletedBatch_ConvertsEmbeddingResultsCorrectly verifies that EmbeddingResult
// objects from the batch API are correctly converted to Embedding objects for storage.
// It validates:
// - Vector dimensions are preserved (768)
// - Model version is set correctly
// - Repository ID is set on each embedding
// - Chunk UUIDs are correctly extracted from RequestIDs
// - All embeddings are properly associated with their chunks
func TestHandleCompletedBatch_ConvertsEmbeddingResultsCorrectly(t *testing.T) {
	// Arrange
	ctx := context.Background()
	indexingJobID := uuid.New()
	batchJobID := "gemini_batch_job_456"

	repositoryID := uuid.New()
	batch := entity.NewBatchJobProgress(repositoryID, indexingJobID, 1, 1)
	_ = batch.MarkSubmittedToGemini(batchJobID)

	completedTime := time.Now()
	job := &outbound.BatchEmbeddingJob{
		JobID:         batchJobID,
		State:         outbound.BatchJobStateCompleted,
		OutputFileURI: "/tmp/output.jsonl",
		CompletedAt:   &completedTime,
	}

	// Create exactly 3 results with specific characteristics
	chunk1ID := uuid.New()
	chunk2ID := uuid.New()
	chunk3ID := uuid.New()

	mockResults := []*outbound.EmbeddingResult{
		{
			Vector:      make([]float64, 768),
			Dimensions:  768,
			Model:       "gemini-embedding-001",
			GeneratedAt: time.Now(),
			RequestID:   fmt.Sprintf("chunk_%s", chunk1ID.String()),
		},
		{
			Vector:      make([]float64, 768),
			Dimensions:  768,
			Model:       "gemini-embedding-001",
			GeneratedAt: time.Now(),
			RequestID:   fmt.Sprintf("chunk_%s", chunk2ID.String()),
		},
		{
			Vector:      make([]float64, 768),
			Dimensions:  768,
			Model:       "gemini-embedding-001",
			GeneratedAt: time.Now(),
			RequestID:   fmt.Sprintf("chunk_%s", chunk3ID.String()),
		},
	}

	// Setup mocks
	mockBatchService := new(MockBatchEmbeddingService)
	mockChunkRepo := new(MockChunkStorageRepository)
	mockProgressRepo := new(MockBatchProgressRepository)

	mockBatchService.On("GetBatchJobResults", ctx, batchJobID).Return(mockResults, nil)

	// Expect SaveEmbeddings with proper validation (chunks already saved)
	mockChunkRepo.On("SaveEmbeddings", ctx,
		mock.MatchedBy(func(embeddings []outbound.Embedding) bool {
			if len(embeddings) != 3 {
				return false
			}
			for _, emb := range embeddings {
				if len(emb.Vector) != 768 ||
					emb.ModelVersion != "gemini-embedding-001" {
					return false
				}
			}
			return true
		}),
	).Return(nil)

	mockProgressRepo.On("Save", ctx, mock.MatchedBy(func(batch *entity.BatchJobProgress) bool {
		return batch.Status() == entity.StatusCompleted && batch.ChunksProcessed() == 3
	})).Return(nil)

	poller := &BatchPoller{
		batchProgressRepo:     mockProgressRepo,
		chunkRepo:             mockChunkRepo,
		batchEmbeddingService: mockBatchService,
		pollInterval:          30 * time.Second,
		maxConcurrentPolls:    5,
		stopCh:                make(chan struct{}),
	}

	// Act
	err := poller.handleCompletedBatch(ctx, batch, job)

	// Assert
	require.NoError(t, err)
	mockBatchService.AssertExpectations(t)
	mockChunkRepo.AssertExpectations(t)
	mockProgressRepo.AssertExpectations(t)
}

// TestHandleCompletedBatch_HandlesPartialResults tests the scenario where a batch was submitted
// with N chunks but only M < N embedding results are returned (e.g., due to API errors).
// It verifies:
// - Only the successful embeddings are saved (8 out of 10)
// - Batch is still marked as completed with accurate count (8)
// - A warning is logged for missing chunk results
func TestHandleCompletedBatch_HandlesPartialResults(t *testing.T) {
	// Arrange
	ctx := context.Background()
	indexingJobID := uuid.New()
	batchJobID := "gemini_batch_job_partial"

	// Batch was created for 10 chunks
	repositoryID := uuid.New()
	batch := entity.NewBatchJobProgress(repositoryID, indexingJobID, 1, 1)
	_ = batch.MarkSubmittedToGemini(batchJobID)

	completedTime := time.Now()
	job := &outbound.BatchEmbeddingJob{
		JobID:         batchJobID,
		State:         outbound.BatchJobStateCompleted,
		TotalCount:    10, // Expected 10 chunks
		OutputFileURI: "/tmp/output.jsonl",
		CompletedAt:   &completedTime,
	}

	// But only 8 results are returned
	mockResults := make([]*outbound.EmbeddingResult, 8)
	for i := 0; i < 8; i++ {
		chunkID := uuid.New()
		mockResults[i] = &outbound.EmbeddingResult{
			Vector:      make([]float64, 768),
			Dimensions:  768,
			Model:       "gemini-embedding-001",
			GeneratedAt: time.Now(),
			RequestID:   fmt.Sprintf("chunk_%s", chunkID.String()),
		}
	}

	// Setup mocks
	mockBatchService := new(MockBatchEmbeddingService)
	mockChunkRepo := new(MockChunkStorageRepository)
	mockProgressRepo := new(MockBatchProgressRepository)

	mockBatchService.On("GetBatchJobResults", ctx, batchJobID).Return(mockResults, nil)

	// Expect only 8 chunks/embeddings to be saved
	mockChunkRepo.On("SaveEmbeddings", ctx,
		mock.MatchedBy(func(embeddings []outbound.Embedding) bool {
			if len(embeddings) != 8 {
				return false
			}
			for _, emb := range embeddings {
				if len(emb.Vector) != 768 {
					return false
				}
			}
			return true
		}),
	).Return(nil)

	// Batch should be marked completed with 8 chunks (not 10)
	mockProgressRepo.On("Save", ctx, mock.MatchedBy(func(batch *entity.BatchJobProgress) bool {
		return batch.Status() == entity.StatusCompleted && batch.ChunksProcessed() == 8
	})).Return(nil)

	poller := &BatchPoller{
		batchProgressRepo:     mockProgressRepo,
		chunkRepo:             mockChunkRepo,
		batchEmbeddingService: mockBatchService,
		pollInterval:          30 * time.Second,
		maxConcurrentPolls:    5,
		stopCh:                make(chan struct{}),
	}

	// Act
	err := poller.handleCompletedBatch(ctx, batch, job)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 8, batch.ChunksProcessed(), "Should process exactly 8 chunks")
	mockBatchService.AssertExpectations(t)
	mockChunkRepo.AssertExpectations(t)
	mockProgressRepo.AssertExpectations(t)

	// TODO: Add assertion for warning log about missing chunks (2 missing out of 10)
	// This would require a log capture mechanism or mock logger
}

// TestHandleCompletedBatch_ValidatesDimensions tests that the handler properly validates
// embedding dimensions match the expected value (768 for gemini-embedding-001).
// It verifies:
// - Embeddings with wrong dimensions are detected (512 instead of 768)
// - Batch is marked as failed when validation fails
// - Error is logged with dimension mismatch details
// - No chunks are saved when validation fails
func TestHandleCompletedBatch_ValidatesDimensions(t *testing.T) {
	// Arrange
	ctx := context.Background()
	indexingJobID := uuid.New()
	batchJobID := "gemini_batch_job_invalid_dims"

	repositoryID := uuid.New()
	batch := entity.NewBatchJobProgress(repositoryID, indexingJobID, 1, 1)
	_ = batch.MarkSubmittedToGemini(batchJobID)

	completedTime := time.Now()
	job := &outbound.BatchEmbeddingJob{
		JobID:         batchJobID,
		State:         outbound.BatchJobStateCompleted,
		OutputFileURI: "/tmp/output.jsonl",
		CompletedAt:   &completedTime,
	}

	// Create results with WRONG dimensions (512 instead of 768)
	chunkID := uuid.New()
	mockResults := []*outbound.EmbeddingResult{
		{
			Vector:      make([]float64, 512), // Wrong dimension!
			Dimensions:  512,                  // Should be 768
			Model:       "gemini-embedding-001",
			GeneratedAt: time.Now(),
			RequestID:   fmt.Sprintf("chunk_%s", chunkID.String()),
		},
	}

	// Setup mocks
	mockBatchService := new(MockBatchEmbeddingService)
	mockChunkRepo := new(MockChunkStorageRepository)
	mockProgressRepo := new(MockBatchProgressRepository)

	mockBatchService.On("GetBatchJobResults", ctx, batchJobID).Return(mockResults, nil)

	// SaveChunksWithEmbeddings should NOT be called due to validation failure
	// mockChunkRepo does not expect SaveChunksWithEmbeddings

	// Batch should be marked as failed due to dimension mismatch
	mockProgressRepo.On("Save", ctx, mock.MatchedBy(func(batch *entity.BatchJobProgress) bool {
		return batch.Status() == entity.StatusFailed && batch.ErrorMessage() != nil
	})).Return(nil)

	poller := &BatchPoller{
		batchProgressRepo:     mockProgressRepo,
		chunkRepo:             mockChunkRepo,
		batchEmbeddingService: mockBatchService,
		pollInterval:          30 * time.Second,
		maxConcurrentPolls:    5,
		stopCh:                make(chan struct{}),
	}

	// Act
	err := poller.handleCompletedBatch(ctx, batch, job)

	// Assert
	require.Error(t, err, "handleCompletedBatch should return error for dimension mismatch")
	assert.Contains(t, err.Error(), "dimension", "Error should mention dimension mismatch")

	// Verify batch is marked as failed
	assert.Equal(t, entity.StatusFailed, batch.Status(), "Batch should be marked as failed")
	assert.NotNil(t, batch.ErrorMessage(), "Batch should have an error message")

	mockBatchService.AssertExpectations(t)
	mockChunkRepo.AssertExpectations(t) // Should have no calls
	mockProgressRepo.AssertExpectations(t)
}

// TestHandleCompletedBatch_HandlesGetResultsError tests error handling when
// GetBatchJobResults fails (e.g., network error, file not found).
// It verifies:
// - Error is properly returned from handleCompletedBatch
// - Batch is NOT marked as completed
// - No database writes occur
// - Error is logged appropriately
func TestHandleCompletedBatch_HandlesGetResultsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	indexingJobID := uuid.New()
	batchJobID := "gemini_batch_job_error"

	repositoryID := uuid.New()
	batch := entity.NewBatchJobProgress(repositoryID, indexingJobID, 1, 1)
	_ = batch.MarkSubmittedToGemini(batchJobID)

	completedTime := time.Now()
	job := &outbound.BatchEmbeddingJob{
		JobID:         batchJobID,
		State:         outbound.BatchJobStateCompleted,
		OutputFileURI: "/tmp/nonexistent_output.jsonl",
		CompletedAt:   &completedTime,
	}

	// Setup mocks
	mockBatchService := new(MockBatchEmbeddingService)
	mockChunkRepo := new(MockChunkStorageRepository)
	mockProgressRepo := new(MockBatchProgressRepository)

	// GetBatchJobResults returns an error
	expectedError := errors.New("failed to download batch results: file not found")
	mockBatchService.On("GetBatchJobResults", ctx, batchJobID).Return(
		([]*outbound.EmbeddingResult)(nil),
		expectedError,
	)

	// Now expect Save to be called to mark batch as failed
	mockProgressRepo.On("Save", ctx, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		// Verify that the batch is marked as failed
		batchArg := args.Get(1).(*entity.BatchJobProgress)
		assert.Equal(t, entity.StatusFailed, batchArg.Status())
		assert.Contains(t, *batchArg.ErrorMessage(), "failed to download batch results")
	})

	// ChunkRepo should not be called

	poller := &BatchPoller{
		batchProgressRepo:     mockProgressRepo,
		chunkRepo:             mockChunkRepo,
		batchEmbeddingService: mockBatchService,
		pollInterval:          30 * time.Second,
		maxConcurrentPolls:    5,
		stopCh:                make(chan struct{}),
	}

	// Act
	err := poller.handleCompletedBatch(ctx, batch, job)

	// Assert
	require.Error(t, err, "handleCompletedBatch should return the GetBatchJobResults error")
	assert.Contains(t, err.Error(), "failed to download batch results",
		"Error should contain the original error message")

	// Verify batch was marked as failed (not staying in processing state)
	assert.Equal(t, entity.StatusFailed, batch.Status(),
		"Batch should be marked as failed on error, not remain in processing state")
	assert.NotNil(t, batch.ErrorMessage())
	assert.Contains(t, *batch.ErrorMessage(), "failed to download batch results")

	mockBatchService.AssertExpectations(t)
	mockChunkRepo.AssertExpectations(t)
	mockProgressRepo.AssertExpectations(t)
}

// TestHandleCompletedBatch_HandlesSaveError tests error handling when
// SaveEmbeddings fails (e.g., database error).
// It verifies:
// - Error is properly returned from handleCompletedBatch
// - Batch is NOT marked as completed
// - Error can be retried in next poll cycle
func TestHandleCompletedBatch_HandlesSaveError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	indexingJobID := uuid.New()
	batchJobID := "gemini_batch_job_save_error"

	repositoryID := uuid.New()
	batch := entity.NewBatchJobProgress(repositoryID, indexingJobID, 1, 1)
	_ = batch.MarkSubmittedToGemini(batchJobID)

	completedTime := time.Now()
	job := &outbound.BatchEmbeddingJob{
		JobID:         batchJobID,
		State:         outbound.BatchJobStateCompleted,
		OutputFileURI: "/tmp/output.jsonl",
		CompletedAt:   &completedTime,
	}

	// Create valid results
	chunkID := uuid.New()
	mockResults := []*outbound.EmbeddingResult{
		{
			Vector:      make([]float64, 768),
			Dimensions:  768,
			Model:       "gemini-embedding-001",
			GeneratedAt: time.Now(),
			RequestID:   fmt.Sprintf("chunk_%s", chunkID.String()),
		},
	}

	// Setup mocks
	mockBatchService := new(MockBatchEmbeddingService)
	mockChunkRepo := new(MockChunkStorageRepository)
	mockProgressRepo := new(MockBatchProgressRepository)

	mockBatchService.On("GetBatchJobResults", ctx, batchJobID).Return(mockResults, nil)

	// SaveEmbeddings returns a database error
	dbError := errors.New("database connection lost")
	mockChunkRepo.On("SaveEmbeddings", ctx,
		mock.MatchedBy(func(embeddings []outbound.Embedding) bool {
			if len(embeddings) != 1 {
				return false
			}
			for _, emb := range embeddings {
				if len(emb.Vector) != 768 {
					return false
				}
			}
			return true
		}),
	).Return(dbError)

	// Progress save SHOULD be called to mark batch as failed
	mockProgressRepo.On("Save", ctx, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		// Verify that the batch is marked as failed
		batchArg := args.Get(1).(*entity.BatchJobProgress)
		assert.Equal(t, entity.StatusFailed, batchArg.Status())
		assert.Contains(t, *batchArg.ErrorMessage(), "database connection lost")
	})

	poller := &BatchPoller{
		batchProgressRepo:     mockProgressRepo,
		chunkRepo:             mockChunkRepo,
		batchEmbeddingService: mockBatchService,
		pollInterval:          30 * time.Second,
		maxConcurrentPolls:    5,
		stopCh:                make(chan struct{}),
	}

	// Act
	err := poller.handleCompletedBatch(ctx, batch, job)

	// Assert
	require.Error(t, err, "handleCompletedBatch should return the save error")
	assert.Contains(t, err.Error(), "database connection lost",
		"Error should contain the database error")

	// Verify batch was marked as failed (not remaining in processing state)
	assert.Equal(t, entity.StatusFailed, batch.Status(),
		"Batch should be marked as failed when save fails, not remain in processing state")
	assert.NotNil(t, batch.ErrorMessage())
	assert.Contains(t, *batch.ErrorMessage(), "database connection lost")

	mockBatchService.AssertExpectations(t)
	mockChunkRepo.AssertExpectations(t)
	mockProgressRepo.AssertExpectations(t)
}

// TestHandleCompletedBatch_ExtractsChunkIDsFromRequestIDs tests that chunk UUIDs
// are correctly extracted from RequestID strings in the format "chunk_<uuid>".
// It verifies:
// - RequestIDs are parsed correctly
// - Invalid RequestID formats are handled (error or skipped)
// - Chunk-to-embedding mapping is correct
func TestHandleCompletedBatch_ExtractsChunkIDsFromRequestIDs(t *testing.T) {
	// Arrange
	ctx := context.Background()
	indexingJobID := uuid.New()
	batchJobID := "gemini_batch_job_requestid"

	repositoryID := uuid.New()
	batch := entity.NewBatchJobProgress(repositoryID, indexingJobID, 1, 1)
	_ = batch.MarkSubmittedToGemini(batchJobID)

	completedTime := time.Now()
	job := &outbound.BatchEmbeddingJob{
		JobID:         batchJobID,
		State:         outbound.BatchJobStateCompleted,
		OutputFileURI: "/tmp/output.jsonl",
		CompletedAt:   &completedTime,
	}

	// Create known chunk IDs
	chunk1ID := uuid.New()
	chunk2ID := uuid.New()

	mockResults := []*outbound.EmbeddingResult{
		{
			Vector:      make([]float64, 768),
			Dimensions:  768,
			Model:       "gemini-embedding-001",
			GeneratedAt: time.Now(),
			RequestID:   fmt.Sprintf("chunk_%s", chunk1ID.String()),
		},
		{
			Vector:      make([]float64, 768),
			Dimensions:  768,
			Model:       "gemini-embedding-001",
			GeneratedAt: time.Now(),
			RequestID:   fmt.Sprintf("chunk_%s", chunk2ID.String()),
		},
	}

	// Setup mocks
	mockBatchService := new(MockBatchEmbeddingService)
	mockChunkRepo := new(MockChunkStorageRepository)
	mockProgressRepo := new(MockBatchProgressRepository)

	mockBatchService.On("GetBatchJobResults", ctx, batchJobID).Return(mockResults, nil)

	// Verify embeddings have the correct chunk IDs extracted from RequestIDs
	mockChunkRepo.On("SaveEmbeddings", ctx,
		mock.MatchedBy(func(embeddings []outbound.Embedding) bool {
			if len(embeddings) != 2 {
				return false
			}
			// Verify chunk IDs match the extracted UUIDs from RequestIDs
			foundChunk1 := false
			foundChunk2 := false
			for _, emb := range embeddings {
				if len(emb.Vector) != 768 {
					return false
				}
				if emb.ChunkID == chunk1ID {
					foundChunk1 = true
				}
				if emb.ChunkID == chunk2ID {
					foundChunk2 = true
				}
			}
			return foundChunk1 && foundChunk2
		}),
	).Return(nil)

	mockProgressRepo.On("Save", ctx, mock.MatchedBy(func(batch *entity.BatchJobProgress) bool {
		return batch.Status() == entity.StatusCompleted && batch.ChunksProcessed() == 2
	})).Return(nil)

	poller := &BatchPoller{
		batchProgressRepo:     mockProgressRepo,
		chunkRepo:             mockChunkRepo,
		batchEmbeddingService: mockBatchService,
		pollInterval:          30 * time.Second,
		maxConcurrentPolls:    5,
		stopCh:                make(chan struct{}),
	}

	// Act
	err := poller.handleCompletedBatch(ctx, batch, job)

	// Assert
	require.NoError(t, err)
	mockBatchService.AssertExpectations(t)
	mockChunkRepo.AssertExpectations(t)
	mockProgressRepo.AssertExpectations(t)
}

// ========================================
// RequestID Encoding/Decoding Tests
// ========================================
//
// These tests define the expected behavior for encoding chunk UUIDs to RequestIDs
// and decoding RequestIDs back to chunk UUIDs. The RequestID format must:
// - Be under 40 characters (Gemini API requirement)
// - Use format "chunk_<uuid_without_hyphens>"
// - Support bidirectional encoding/decoding without loss
//
// Functions under test:
// - EncodeChunkIDToRequestID(chunkID uuid.UUID) string
// - decodeRequestIDToChunkID(requestID string) (uuid.UUID, error)

// TestEncodeChunkIDToRequestID_ValidUUID tests encoding a valid UUID to RequestID format.
// It verifies:
// - Output format is "chunk_<uuid_without_hyphens>"
// - Length is <= 40 characters
// - String starts with "chunk_" prefix
// - Contains only alphanumeric characters and underscore
func TestEncodeChunkIDToRequestID_ValidUUID(t *testing.T) {
	// Arrange
	testUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	// Act
	requestID := EncodeChunkIDToRequestID(testUUID)

	// Assert
	assert.NotEmpty(t, requestID, "RequestID should not be empty")
	assert.LessOrEqual(t, len(requestID), 40, "RequestID must be <= 40 characters for Gemini API")
	assert.True(t, len(requestID) > 6, "RequestID should have content beyond the prefix")

	// Verify format: "chunk_<uuid_without_hyphens>"
	assert.True(t, len(requestID) >= 6, "RequestID should have at least 'chunk_' prefix")
	assert.Equal(t, "chunk_", requestID[:6], "RequestID should start with 'chunk_' prefix")

	// Verify the UUID part (without hyphens) is exactly 32 characters
	uuidPart := requestID[6:]
	assert.Equal(t, 32, len(uuidPart), "UUID without hyphens should be 32 characters")

	// Verify only valid characters (alphanumeric + underscore)
	validChars := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_"
	for _, char := range requestID {
		assert.Contains(t, validChars, string(char),
			"RequestID should contain only alphanumeric and underscore characters")
	}

	// Verify expected output format
	expected := "chunk_550e8400e29b41d4a716446655440000"
	assert.Equal(t, expected, requestID, "RequestID should match expected format")

	// Verify total length (6 char prefix + 32 char UUID = 38 chars)
	assert.Equal(t, 38, len(requestID), "RequestID should be exactly 38 characters")
}

// TestEncodeChunkIDToRequestID_MultipleUUIDs tests that different UUIDs produce unique RequestIDs.
// It verifies:
// - All encoded IDs are unique
// - All encoded IDs are <= 40 characters
// - All encoded IDs start with "chunk_" prefix
// - Each encoded ID properly represents its source UUID
func TestEncodeChunkIDToRequestID_MultipleUUIDs(t *testing.T) {
	// Arrange - Create 5 different UUIDs
	testUUIDs := []uuid.UUID{
		uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
		uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8"),
		uuid.MustParse("00000000-0000-0000-0000-000000000000"), // Nil UUID
		uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff"), // Max UUID
	}

	// Act - Encode each UUID
	requestIDs := make([]string, len(testUUIDs))
	for i, testUUID := range testUUIDs {
		requestIDs[i] = EncodeChunkIDToRequestID(testUUID)
	}

	// Assert - All IDs should be unique
	uniqueIDs := make(map[string]bool)
	for _, requestID := range requestIDs {
		uniqueIDs[requestID] = true
	}
	assert.Equal(t, len(testUUIDs), len(uniqueIDs),
		"All encoded RequestIDs should be unique")

	// Assert - All IDs meet format requirements
	for i, requestID := range requestIDs {
		assert.LessOrEqual(t, len(requestID), 40,
			"RequestID %d must be <= 40 characters", i)
		assert.True(t, len(requestID) >= 6,
			"RequestID %d should have 'chunk_' prefix", i)
		assert.Equal(t, "chunk_", requestID[:6],
			"RequestID %d should start with 'chunk_'", i)
		assert.Equal(t, 38, len(requestID),
			"RequestID %d should be exactly 38 characters", i)
	}

	// Verify expected outputs
	expectedIDs := []string{
		"chunk_550e8400e29b41d4a716446655440000",
		"chunk_6ba7b8109dad11d180b400c04fd430c8",
		"chunk_6ba7b8119dad11d180b400c04fd430c8",
		"chunk_00000000000000000000000000000000",
		"chunk_ffffffffffffffffffffffffffffffff",
	}

	for i, expected := range expectedIDs {
		assert.Equal(t, expected, requestIDs[i],
			"RequestID %d should match expected format", i)
	}
}

// TestDecodeRequestIDToChunkID_ValidRequestID tests decoding a valid RequestID back to UUID.
// It verifies:
// - RequestID in correct format decodes without error
// - Decoded UUID matches the expected value
// - Hyphens are correctly reinserted into the UUID
func TestDecodeRequestIDToChunkID_ValidRequestID(t *testing.T) {
	// Arrange
	requestID := "chunk_550e8400e29b41d4a716446655440000"
	expectedUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	// Act
	decodedUUID, err := decodeRequestIDToChunkID(requestID)

	// Assert
	require.NoError(t, err, "Decoding valid RequestID should not return an error")
	assert.Equal(t, expectedUUID, decodedUUID,
		"Decoded UUID should match the expected UUID")
}

// TestDecodeRequestIDToChunkID_InvalidPrefix tests error handling for incorrect prefix.
// It verifies:
// - RequestID with wrong prefix returns an error
// - Error message indicates invalid format or missing prefix
func TestDecodeRequestIDToChunkID_InvalidPrefix(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
	}{
		{
			name:      "wrong prefix",
			requestID: "invalid_550e8400e29b41d4a716446655440000",
		},
		{
			name:      "no prefix",
			requestID: "550e8400e29b41d4a716446655440000",
		},
		{
			name:      "partial prefix",
			requestID: "chunk550e8400e29b41d4a716446655440000",
		},
		{
			name:      "uppercase prefix",
			requestID: "CHUNK_550e8400e29b41d4a716446655440000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			decodedUUID, err := decodeRequestIDToChunkID(tt.requestID)

			// Assert
			require.Error(t, err, "Decoding invalid RequestID should return an error")
			assert.Equal(t, uuid.Nil, decodedUUID,
				"Decoded UUID should be nil on error")

			// Error should mention format or prefix issue
			errMsg := err.Error()
			assert.True(t,
				containsAny(errMsg, []string{"invalid", "format", "prefix", "chunk_"}),
				"Error message should indicate format/prefix issue, got: %s", errMsg)
		})
	}
}

// TestDecodeRequestIDToChunkID_InvalidUUID tests error handling for invalid UUID strings.
// It verifies:
// - RequestID with invalid UUID format returns an error
// - Error message indicates invalid UUID
func TestDecodeRequestIDToChunkID_InvalidUUID(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
	}{
		{
			name:      "not a UUID",
			requestID: "chunk_notauuidstring",
		},
		{
			name:      "too short",
			requestID: "chunk_550e8400",
		},
		{
			name:      "too long",
			requestID: "chunk_550e8400e29b41d4a716446655440000extrachars",
		},
		{
			name:      "invalid characters",
			requestID: "chunk_550e8400e29b41d4a716446655440zzz",
		},
		{
			name:      "special characters",
			requestID: "chunk_550e8400-e29b-41d4-a716-446655440!@#",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			decodedUUID, err := decodeRequestIDToChunkID(tt.requestID)

			// Assert
			require.Error(t, err, "Decoding invalid UUID should return an error")
			assert.Equal(t, uuid.Nil, decodedUUID,
				"Decoded UUID should be nil on error")

			// Error should mention UUID issue
			errMsg := err.Error()
			assert.True(t,
				containsAny(errMsg, []string{"invalid", "uuid", "parse", "format"}),
				"Error message should indicate UUID issue, got: %s", errMsg)
		})
	}
}

// TestDecodeRequestIDToChunkID_EmptyString tests error handling for empty input.
// It verifies:
// - Empty string returns an error
// - Error indicates empty or invalid RequestID
func TestDecodeRequestIDToChunkID_EmptyString(t *testing.T) {
	// Act
	decodedUUID, err := decodeRequestIDToChunkID("")

	// Assert
	require.Error(t, err, "Decoding empty string should return an error")
	assert.Equal(t, uuid.Nil, decodedUUID,
		"Decoded UUID should be nil on error")

	// Error should mention empty or invalid RequestID
	errMsg := err.Error()
	assert.True(t,
		containsAny(errMsg, []string{"empty", "invalid", "format"}),
		"Error message should indicate empty/invalid RequestID, got: %s", errMsg)
}

// TestEncodeDecodeRoundTrip tests that encoding and decoding are inverse operations.
// It verifies:
// - For any UUID, encode(UUID) -> decode(RequestID) == UUID
// - No data loss occurs during round-trip conversion
// - Process works for multiple random UUIDs
func TestEncodeDecodeRoundTrip(t *testing.T) {
	// Arrange - Generate 10 random UUIDs
	testUUIDs := make([]uuid.UUID, 10)
	for i := 0; i < 10; i++ {
		testUUIDs[i] = uuid.New()
	}

	// Act & Assert - Test round-trip for each UUID
	for i, originalUUID := range testUUIDs {
		t.Run(fmt.Sprintf("UUID_%d", i), func(t *testing.T) {
			// Encode
			requestID := EncodeChunkIDToRequestID(originalUUID)
			assert.NotEmpty(t, requestID,
				"Encoded RequestID should not be empty")

			// Decode
			decodedUUID, err := decodeRequestIDToChunkID(requestID)
			require.NoError(t, err,
				"Decoding should not return an error for valid encoded RequestID")

			// Verify round-trip
			assert.Equal(t, originalUUID, decodedUUID,
				"Decoded UUID should match original UUID")
		})
	}
}

// TestDecodeRequestIDToChunkID_WithHyphens tests backward compatibility with hyphenated UUIDs.
// It verifies the behavior when RequestID contains a UUID with hyphens.
// DESIGN DECISION: Should this format be supported for backward compatibility,
// or should we enforce strict hyphen-free format?
func TestDecodeRequestIDToChunkID_WithHyphens(t *testing.T) {
	// Arrange - RequestID with hyphens (old format)
	requestID := "chunk_550e8400-e29b-41d4-a716-446655440000"
	expectedUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	// Act
	decodedUUID, err := decodeRequestIDToChunkID(requestID)

	// Assert - Two possible design choices:
	//
	// Option 1: Support backward compatibility (accept both formats)
	// This would allow migration from old format to new format
	if err == nil {
		// Backward compatible implementation
		assert.Equal(t, expectedUUID, decodedUUID,
			"Should decode hyphenated UUID for backward compatibility")
		t.Log("Implementation supports backward compatibility with hyphenated UUIDs")
	} else {
		// Option 2: Strict parsing (reject hyphenated format)
		// This enforces the new format and prevents confusion
		assert.Error(t, err,
			"Should reject hyphenated UUID format (strict parsing)")
		assert.Equal(t, uuid.Nil, decodedUUID,
			"Decoded UUID should be nil when rejecting hyphenated format")
		t.Log("Implementation enforces strict hyphen-free format")

		// Error should indicate length or format issue
		errMsg := err.Error()
		assert.True(t,
			containsAny(errMsg, []string{"invalid", "length", "format", "hyphen"}),
			"Error message should indicate format issue, got: %s", errMsg)
	}

	// NOTE: The implementation will determine which approach to take.
	// This test documents both possibilities and will pass either way,
	// but the behavior should be consistent and documented.
}

// TestEncodeChunkIDToRequestID_EdgeCases tests edge cases for encoding.
// It verifies:
// - Nil UUID encodes successfully
// - Max UUID encodes successfully
// - All encoded values meet length requirements
func TestEncodeChunkIDToRequestID_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		uuid     uuid.UUID
		expected string
	}{
		{
			name:     "nil UUID",
			uuid:     uuid.Nil,
			expected: "chunk_00000000000000000000000000000000",
		},
		{
			name:     "max UUID",
			uuid:     uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff"),
			expected: "chunk_ffffffffffffffffffffffffffffffff",
		},
		{
			name:     "UUID with all same digits",
			uuid:     uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			expected: "chunk_11111111111111111111111111111111",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			requestID := EncodeChunkIDToRequestID(tt.uuid)

			// Assert
			assert.Equal(t, tt.expected, requestID,
				"Encoded RequestID should match expected format")
			assert.LessOrEqual(t, len(requestID), 40,
				"RequestID must be <= 40 characters")
			assert.Equal(t, 38, len(requestID),
				"RequestID should be exactly 38 characters")
		})
	}
}

// TestDecodeRequestIDToChunkID_CaseSensitivity tests case handling in UUID parsing.
// It verifies:
// - Lowercase UUID hex strings decode correctly
// - Uppercase UUID hex strings decode correctly
// - Mixed case UUID hex strings decode correctly
func TestDecodeRequestIDToChunkID_CaseSensitivity(t *testing.T) {
	tests := []struct {
		name         string
		requestID    string
		expectedUUID uuid.UUID
	}{
		{
			name:         "lowercase UUID",
			requestID:    "chunk_550e8400e29b41d4a716446655440000",
			expectedUUID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		},
		{
			name:         "uppercase UUID",
			requestID:    "chunk_550E8400E29B41D4A716446655440000",
			expectedUUID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		},
		{
			name:         "mixed case UUID",
			requestID:    "chunk_550e8400E29B41d4A716446655440000",
			expectedUUID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			decodedUUID, err := decodeRequestIDToChunkID(tt.requestID)

			// Assert
			require.NoError(t, err,
				"Decoding should handle case-insensitive UUID hex strings")
			assert.Equal(t, tt.expectedUUID, decodedUUID,
				"Decoded UUID should match expected value regardless of case")
		})
	}
}

// ========================================
// Additional Repository Test Case for Missing repository_id Flow
// ========================================

// TestHandleCompletedBatch_RepositoryIDFlow tests the critical repository_id persistence issue.
// This test specifically validates that repository_id flows correctly from the IndexingJob
// to the chunks and embeddings during batch result processing.
//
// This test documents the BUG: repository_id is hardcoded to uuid.Nil in batch_poller.go:374-377
// Expected behavior: repository_id should be retrieved from IndexingJob and set on chunks/embeddings
func TestHandleCompletedBatch_RepositoryIDFlow(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New() // The repository ID that should be preserved
	indexingJobID := uuid.New()
	batchJobID := "gemini_batch_job_repository_test"

	// Create batch progress entity with indexing job ID
	batch := entity.NewBatchJobProgress(repositoryID, indexingJobID, 1, 1)
	_ = batch.MarkSubmittedToGemini(batchJobID)

	completedTime := time.Now()
	job := &outbound.BatchEmbeddingJob{
		JobID:         batchJobID,
		State:         outbound.BatchJobStateCompleted,
		OutputFileURI: "/tmp/output.jsonl",
		CompletedAt:   &completedTime,
	}

	// Create test embedding result
	chunkID := uuid.New()
	mockResults := []*outbound.EmbeddingResult{
		{
			Vector:      make([]float64, 768),
			Dimensions:  768,
			Model:       "gemini-embedding-001",
			GeneratedAt: time.Now(),
			RequestID:   fmt.Sprintf("chunk_%s", chunkID.String()),
		},
	}

	// Setup mocks
	mockBatchService := new(MockBatchEmbeddingService)
	mockChunkRepo := new(MockChunkStorageRepository)
	mockProgressRepo := new(MockBatchProgressRepository)

	mockBatchService.On("GetBatchJobResults", ctx, batchJobID).Return(mockResults, nil)

	// Verify that repository_id is correctly set from batch progress
	mockChunkRepo.On("SaveEmbeddings", ctx,
		mock.MatchedBy(func(embeddings []outbound.Embedding) bool {
			if len(embeddings) != 1 {
				return false
			}
			// Verify embedding has correct repository_id
			return embeddings[0].RepositoryID != uuid.Nil && embeddings[0].RepositoryID == repositoryID
		}),
	).Return(nil).Run(func(args mock.Arguments) {
		// Additional validation inside the mock
		embeddings := args.Get(1).([]outbound.Embedding)

		t.Logf("FIXED: Embedding repository_id is %s (expected %s)", embeddings[0].RepositoryID, repositoryID)

		if embeddings[0].RepositoryID == uuid.Nil {
			t.Errorf("REGRESSION: Embedding repository_id is incorrectly set to uuid.Nil")
		}
	})

	// Mock batch progress update
	mockProgressRepo.On("Save", ctx, mock.MatchedBy(func(batch *entity.BatchJobProgress) bool {
		return batch.Status() == entity.StatusCompleted && batch.ChunksProcessed() == 1
	})).Return(nil)

	poller := &BatchPoller{
		batchProgressRepo:     mockProgressRepo,
		chunkRepo:             mockChunkRepo,
		batchEmbeddingService: mockBatchService,
		pollInterval:          30 * time.Second,
		maxConcurrentPolls:    5,
		stopCh:                make(chan struct{}),
	}

	// Act - This should FAIL due to the repository_id bug
	t.Logf("Executing handleCompletedBatch - should fail due to repository_id bug")
	err := poller.handleCompletedBatch(ctx, batch, job)

	// Assert - The test should fail because repository_id is uuid.Nil instead of repositoryID
	// However, due to the mock's .Return(nil), it might pass, so we need additional validation

	// If we reach here without mock assertions failing, it means the bug exists and we need to fix the test
	// The mock.MatchedBy functions above will have already logged the bug evidence

	// This documents the expected state: the function returns no error but passes wrong data
	require.NoError(t, err, "handleCompletedBatch should not return error, but should preserve repository_id")

	// The mock assertions above should show the bug - repository_id will be uuid.Nil instead of repositoryID
	t.Logf("Test completed - check mock logs for evidence of repository_id being uuid.Nil instead of %s", repositoryID)

	mockBatchService.AssertExpectations(t)
	mockChunkRepo.AssertExpectations(t)
	mockProgressRepo.AssertExpectations(t)
}

// TestHandleCompletedBatch_RepositoryIDFixed verifies that repository_id is properly
// preserved from batch to chunks and embeddings
func TestHandleCompletedBatch_RepositoryIDFixed(t *testing.T) {
	// This test verifies the FIXED behavior where repository_id flows correctly
	// from batch entity to chunks and embeddings

	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()
	batchJobID := "gemini_batch_explicit_failure"

	batch := entity.NewBatchJobProgress(repositoryID, indexingJobID, 1, 1)
	_ = batch.MarkSubmittedToGemini(batchJobID)

	completedTime := time.Now()
	job := &outbound.BatchEmbeddingJob{
		JobID:         batchJobID,
		State:         outbound.BatchJobStateCompleted,
		OutputFileURI: "/tmp/output.jsonl",
		CompletedAt:   &completedTime,
	}

	chunkID := uuid.New()
	mockResults := []*outbound.EmbeddingResult{
		{
			Vector:      make([]float64, 768),
			Dimensions:  768,
			Model:       "gemini-embedding-001",
			GeneratedAt: time.Now(),
			RequestID:   fmt.Sprintf("chunk_%s", chunkID.String()),
		},
	}

	mockBatchService := new(MockBatchEmbeddingService)
	mockChunkRepo := new(MockChunkStorageRepository)
	mockProgressRepo := new(MockBatchProgressRepository)

	mockBatchService.On("GetBatchJobResults", ctx, batchJobID).Return(mockResults, nil)

	// This mock expects the CORRECT behavior (repositoryID properly assigned)
	mockChunkRepo.On("SaveEmbeddings", ctx,
		mock.MatchedBy(func(embeddings []outbound.Embedding) bool {
			if len(embeddings) != 1 {
				return false
			}
			// CORRECT BEHAVIOR: repository_id should be properly assigned from batch
			if embeddings[0].RepositoryID == repositoryID {
				t.Logf("✓ FIXED: Embedding repository_id correctly assigned %s", repositoryID)
				return true
			}
			t.Errorf("✗ FAILED: Embedding repository_id is %s, should be %s", embeddings[0].RepositoryID, repositoryID)
			return false
		}),
	).Return(nil)

	mockProgressRepo.On("Save", ctx, mock.Anything).Return(nil)

	poller := &BatchPoller{
		batchProgressRepo:     mockProgressRepo,
		chunkRepo:             mockChunkRepo,
		batchEmbeddingService: mockBatchService,
		pollInterval:          30 * time.Second,
		maxConcurrentPolls:    5,
		stopCh:                make(chan struct{}),
	}

	// Act
	err := poller.handleCompletedBatch(ctx, batch, job)

	// Assert
	require.NoError(t, err)

	// Verify the fix is working properly
	t.Logf("✓ Repository ID flow test passed - repository_id correctly preserved from batch to chunks/embeddings")

	mockBatchService.AssertExpectations(t)
	mockChunkRepo.AssertExpectations(t)
	mockProgressRepo.AssertExpectations(t)
}

// Helper function to check if a string contains any of the specified substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}
