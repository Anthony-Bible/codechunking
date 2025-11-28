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
)

// TestJobProcessor_CountTokensForChunks tests that the job processor calls CountTokensBatch
// on the embedding service before generating embeddings.
func TestJobProcessor_CountTokensForChunks(t *testing.T) {
	t.Parallel()

	// Setup
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()

	// Create test chunks
	chunks := []outbound.CodeChunk{
		{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			FilePath:     "test.go",
			Content:      "func main() { fmt.Println(\"hello\") }",
			StartLine:    1,
			EndLine:      1,
			Type:         "function",
		},
		{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			FilePath:     "test.go",
			Content:      "type User struct { Name string }",
			StartLine:    3,
			EndLine:      3,
			Type:         "type",
		},
	}

	// Create mocks
	mockEmbeddingService := &MockEmbeddingService{}
	mockChunkRepo := &MockChunkStorageRepository{}
	mockRepoRepo := &MockRepositoryRepository{}
	mockIndexingJobRepo := &MockIndexingJobRepository{}
	mockGitClient := &MockEnhancedGitClient{}
	mockCodeParser := &MockCodeParser{}

	// Expected token count results
	expectedTokenCounts := []*outbound.TokenCountResult{
		{
			TotalTokens: 10,
			Model:       "gemini-embedding-001",
		},
		{
			TotalTokens: 6,
			Model:       "gemini-embedding-001",
		},
	}

	// Extract texts from chunks for verification
	expectedTexts := []string{
		chunks[0].Content,
		chunks[1].Content,
	}

	// Mock expectations
	// 1. CountTokensWithCallback should be called BEFORE GenerateBatchEmbeddings
	mockEmbeddingService.On("CountTokensWithCallback",
		mock.Anything, // ctx
		mock.MatchedBy(func(chunks []outbound.CodeChunk) bool {
			return len(chunks) == 2
		}),
		"gemini-embedding-001",
		mock.Anything, // callback
	).Run(func(args mock.Arguments) {
		// Extract arguments
		chunks := args.Get(1).([]outbound.CodeChunk)
		callback := args.Get(3).(outbound.TokenCountCallback)

		// Simulate the callback being invoked for each chunk
		for i := range chunks {
			result := expectedTokenCounts[i]
			err := callback(i, &chunks[i], result)
			if err != nil {
				continue
			}
		}
	}).Return(nil).Once()

	// 2. Chunks should be saved with token counts populated
	mockChunkRepo.On("SaveChunks",
		mock.Anything, // ctx
		mock.MatchedBy(func(chunks []outbound.CodeChunk) bool {
			// Verify that we're saving both chunks with the correct token counts
			if len(chunks) != 2 {
				return false
			}
			// Check first chunk has token count and timestamp
			if chunks[0].TokenCount != 10 {
				return false
			}
			if chunks[0].TokenCountedAt == nil {
				return false
			}
			// Check second chunk has token count and timestamp
			if chunks[1].TokenCount != 6 {
				return false
			}
			if chunks[1].TokenCountedAt == nil {
				return false
			}
			// Verify chunks have RepositoryID set
			if chunks[0].RepositoryID == uuid.Nil {
				return false
			}
			if chunks[1].RepositoryID == uuid.Nil {
				return false
			}
			return true
		}),
	).Return(nil).Once()

	// 3. Then embedding generation should proceed
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		mock.Anything, // ctx
		expectedTexts,
		mock.Anything, // options
	).Return([]*outbound.EmbeddingResult{
		{Vector: make([]float64, 768), Dimensions: 768, Model: "gemini-embedding-001"},
		{Vector: make([]float64, 768), Dimensions: 768, Model: "gemini-embedding-001"},
	}, nil).Once()

	// Mock chunk storage
	mockChunkRepo.On("SaveChunkWithEmbedding",
		mock.Anything, // ctx
		mock.Anything, // chunk
		mock.Anything, // embedding
	).Return(nil)

	// Create processor config
	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-workspace",
		MaxConcurrentJobs: 1,
		JobTimeout:        30 * time.Second,
	}

	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		UseTestEmbeddings:    false,
		FallbackToSequential: true,
		TokenCounting: config.TokenCountingConfig{
			Enabled:           true,
			Mode:              "all",
			SamplePercent:     10,
			MaxTokensPerChunk: 8192,
		},
	}

	// Create processor
	processor := NewDefaultJobProcessor(
		processorConfig,
		mockIndexingJobRepo,
		mockRepoRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbeddingService,
		mockChunkRepo,
		&JobProcessorBatchOptions{
			BatchConfig: &batchConfig,
		},
	).(*DefaultJobProcessor)

	// Execute - this should call token counting before embeddings
	err := processor.generateEmbeddings(ctx, indexingJobID, repositoryID, chunks)

	// Assert
	assert.NoError(t, err, "Expected no error from generateEmbeddings")
	mockEmbeddingService.AssertExpectations(t)
	mockChunkRepo.AssertExpectations(t)

	// Verify call order: CountTokensWithCallback should be called before GenerateBatchEmbeddings
	calls := mockEmbeddingService.Calls
	countTokensIdx, generateEmbeddingsIdx := -1, -1
	for i, call := range calls {
		if call.Method == "CountTokensWithCallback" {
			countTokensIdx = i
		}
		if call.Method == "GenerateBatchEmbeddings" {
			generateEmbeddingsIdx = i
		}
	}

	assert.NotEqual(t, -1, countTokensIdx, "CountTokensWithCallback should have been called")
	assert.NotEqual(t, -1, generateEmbeddingsIdx, "GenerateBatchEmbeddings should have been called")
	assert.Less(
		t,
		countTokensIdx,
		generateEmbeddingsIdx,
		"CountTokensWithCallback must be called BEFORE GenerateBatchEmbeddings",
	)
}

// TestJobProcessor_TokenCountingModes tests different token counting modes.
func TestJobProcessor_TokenCountingModes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		mode              string
		samplePercent     int
		totalChunks       int
		expectedCountCall bool
		expectedSamples   int
	}{
		{
			name:              "Mode all - count all chunks",
			mode:              "all",
			samplePercent:     0,
			totalChunks:       10,
			expectedCountCall: true,
			expectedSamples:   10,
		},
		{
			name:              "Mode sample - count 10% of chunks",
			mode:              "sample",
			samplePercent:     10,
			totalChunks:       100,
			expectedCountCall: true,
			expectedSamples:   10,
		},
		{
			name:              "Mode sample - count 20% of chunks",
			mode:              "sample",
			samplePercent:     20,
			totalChunks:       50,
			expectedCountCall: true,
			expectedSamples:   10,
		},
		{
			name:              "Mode on_demand - skip token counting",
			mode:              "on_demand",
			samplePercent:     0,
			totalChunks:       10,
			expectedCountCall: false,
			expectedSamples:   0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Setup
			ctx := context.Background()
			repositoryID := uuid.New()
			indexingJobID := uuid.New()

			// Create test chunks
			chunks := make([]outbound.CodeChunk, tc.totalChunks)
			for i := range tc.totalChunks {
				chunks[i] = outbound.CodeChunk{
					ID:           uuid.New().String(),
					RepositoryID: repositoryID,
					FilePath:     "test.go",
					Content:      "func test() {}",
					StartLine:    i + 1,
					EndLine:      i + 1,
					Type:         "function",
				}
			}

			// Create mocks
			mockEmbeddingService := &MockEmbeddingService{}
			mockChunkRepo := &MockChunkStorageRepository{}

			// Configure expectations based on mode
			if tc.expectedCountCall {
				// CountTokensWithCallback should be called with sampled chunks
				mockEmbeddingService.On("CountTokensWithCallback",
					mock.Anything,
					mock.MatchedBy(func(chunks []outbound.CodeChunk) bool {
						return len(chunks) == tc.expectedSamples
					}),
					"gemini-embedding-001",
					mock.Anything, // callback
				).Run(func(args mock.Arguments) {
					// Extract arguments
					chunks := args.Get(1).([]outbound.CodeChunk)
					callback := args.Get(3).(outbound.TokenCountCallback)

					// Simulate the callback being invoked for each chunk
					for i := range chunks {
						result := &outbound.TokenCountResult{
							TotalTokens: 5,
							Model:       "gemini-embedding-001",
						}
						_ = callback(i, &chunks[i], result)
					}
				}).Return(nil).Once()

				// SaveChunks should be called with chunks that have token counts populated
				mockChunkRepo.On("SaveChunks",
					mock.Anything,
					mock.MatchedBy(func(chunks []outbound.CodeChunk) bool {
						// Verify we have the expected number of chunks
						if len(chunks) != tc.expectedSamples {
							return false
						}
						// Verify all chunks have token count and timestamp
						for i := range chunks {
							if chunks[i].TokenCount == 0 {
								return false
							}
							if chunks[i].TokenCountedAt == nil {
								return false
							}
							if chunks[i].RepositoryID == uuid.Nil {
								return false
							}
						}
						return true
					}),
				).Return(nil).Once()
			}

			// Embedding generation should always proceed - create results that match chunk count
			generatedResults := make([]*outbound.EmbeddingResult, tc.totalChunks)
			for i := range generatedResults {
				generatedResults[i] = &outbound.EmbeddingResult{
					Vector:     make([]float64, 768),
					Dimensions: 768,
					Model:      "gemini-embedding-001",
				}
			}
			mockEmbeddingService.On("GenerateBatchEmbeddings",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(generatedResults, nil)

			mockChunkRepo.On("SaveChunkWithEmbedding",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(nil)

			// Create processor with token counting config
			processorConfig := JobProcessorConfig{
				WorkspaceDir:      "/tmp/test-workspace",
				MaxConcurrentJobs: 1,
				JobTimeout:        30 * time.Second,
			}

			batchConfig := config.BatchProcessingConfig{
				Enabled:              true,
				ThresholdChunks:      1,
				UseTestEmbeddings:    false,
				FallbackToSequential: true,
				TokenCounting: config.TokenCountingConfig{
					Enabled:       tc.mode != "on_demand",
					Mode:          tc.mode,
					SamplePercent: tc.samplePercent,
				},
			}

			// Create processor (this will fail until implementation exists)
			processor := NewDefaultJobProcessor(
				processorConfig,
				&MockIndexingJobRepository{},
				&MockRepositoryRepository{},
				&MockEnhancedGitClient{},
				&MockCodeParser{},
				mockEmbeddingService,
				mockChunkRepo,
				&JobProcessorBatchOptions{
					BatchConfig: &batchConfig,
				},
			).(*DefaultJobProcessor)

			// Execute
			err := processor.generateEmbeddings(ctx, indexingJobID, repositoryID, chunks)

			// Assert
			assert.NoError(t, err)
			mockEmbeddingService.AssertExpectations(t)
			mockChunkRepo.AssertExpectations(t)
		})
	}
}

// TestJobProcessor_TokenCountingFailsGracefully tests that token counting failures
// don't block embedding generation.
func TestJobProcessor_TokenCountingFailsGracefully(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		tokenCountErr error
		expectWarning bool
		expectSuccess bool
	}{
		{
			name:          "Network error - warn and continue",
			tokenCountErr: errors.New("network timeout"),
			expectWarning: true,
			expectSuccess: true,
		},
		{
			name:          "API error - warn and continue",
			tokenCountErr: errors.New("API rate limit exceeded"),
			expectWarning: true,
			expectSuccess: true,
		},
		{
			name:          "Quota exceeded - warn and continue",
			tokenCountErr: errors.New("quota exceeded"),
			expectWarning: true,
			expectSuccess: true,
		},
		{
			name:          "Success - no warning",
			tokenCountErr: nil,
			expectWarning: false,
			expectSuccess: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Setup
			ctx := context.Background()
			repositoryID := uuid.New()
			indexingJobID := uuid.New()

			chunks := []outbound.CodeChunk{
				{
					ID:           uuid.New().String(),
					RepositoryID: repositoryID,
					FilePath:     "test.go",
					Content:      "func test() {}",
					StartLine:    1,
					EndLine:      1,
					Type:         "function",
				},
			}

			// Create mocks
			mockEmbeddingService := &MockEmbeddingService{}
			mockChunkRepo := &MockChunkStorageRepository{}

			// Mock CountTokensWithCallback - may fail or succeed
			if tc.tokenCountErr != nil {
				mockEmbeddingService.On("CountTokensWithCallback",
					mock.Anything,
					mock.Anything,
					"gemini-embedding-001",
					mock.Anything, // callback
				).Return(tc.tokenCountErr).Once()

				// SaveChunks should NOT be called on error (graceful degradation)
				// (no expectation set means test fails if called)
			} else {
				mockEmbeddingService.On("CountTokensWithCallback",
					mock.Anything,
					mock.Anything,
					"gemini-embedding-001",
					mock.Anything, // callback
				).Run(func(args mock.Arguments) {
					// Extract arguments
					chunks := args.Get(1).([]outbound.CodeChunk)
					callback := args.Get(3).(outbound.TokenCountCallback)

					// Simulate the callback being invoked for each chunk
					for i := range chunks {
						result := &outbound.TokenCountResult{
							TotalTokens: 5,
							Model:       "gemini-embedding-001",
						}
						_ = callback(i, &chunks[i], result)
					}
				}).Return(nil).Once()

				// SaveChunks should be called with chunk that has token count populated
				mockChunkRepo.On("SaveChunks",
					mock.Anything,
					mock.MatchedBy(func(chunks []outbound.CodeChunk) bool {
						if len(chunks) != 1 {
							return false
						}
						return chunks[0].TokenCount == 5 &&
							chunks[0].TokenCountedAt != nil &&
							chunks[0].RepositoryID != uuid.Nil
					}),
				).Return(nil).Once()
			}

			// Embedding generation MUST always proceed regardless of token counting result
			mockEmbeddingService.On("GenerateBatchEmbeddings",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return([]*outbound.EmbeddingResult{
				{Vector: make([]float64, 768), Dimensions: 768, Model: "gemini-embedding-001"},
			}, nil).Once()

			mockChunkRepo.On("SaveChunkWithEmbedding",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(nil)

			// Create processor
			processorConfig := JobProcessorConfig{
				WorkspaceDir:      "/tmp/test-workspace",
				MaxConcurrentJobs: 1,
				JobTimeout:        30 * time.Second,
			}

			batchConfig := config.BatchProcessingConfig{
				Enabled:              true,
				ThresholdChunks:      1,
				UseTestEmbeddings:    false,
				FallbackToSequential: true,
				TokenCounting: config.TokenCountingConfig{
					Enabled: true,
					Mode:    "all",
				},
			}

			processor := NewDefaultJobProcessor(
				processorConfig,
				&MockIndexingJobRepository{},
				&MockRepositoryRepository{},
				&MockEnhancedGitClient{},
				&MockCodeParser{},
				mockEmbeddingService,
				mockChunkRepo,
				&JobProcessorBatchOptions{
					BatchConfig: &batchConfig,
				},
			).(*DefaultJobProcessor)

			// Execute
			err := processor.generateEmbeddings(ctx, indexingJobID, repositoryID, chunks)

			// Assert
			if tc.expectSuccess {
				assert.NoError(t, err, "Embedding generation should succeed even if token counting fails")
			} else {
				assert.Error(t, err, "Expected error")
			}

			// Verify that GenerateBatchEmbeddings was called (embeddings must proceed)
			mockEmbeddingService.AssertCalled(t, "GenerateBatchEmbeddings", mock.Anything, mock.Anything, mock.Anything)
			mockEmbeddingService.AssertExpectations(t)
		})
	}
}

// TestJobProcessor_TokenCountingUpdatesPersistence tests that token counts
// are persisted to the chunk repository.
func TestJobProcessor_TokenCountingUpdatesPersistence(t *testing.T) {
	t.Parallel()

	// Setup
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()

	chunk1ID := uuid.New()
	chunk2ID := uuid.New()

	chunks := []outbound.CodeChunk{
		{
			ID:           chunk1ID.String(),
			RepositoryID: repositoryID,
			FilePath:     "file1.go",
			Content:      "package main",
			StartLine:    1,
			EndLine:      1,
			Type:         "package",
		},
		{
			ID:           chunk2ID.String(),
			RepositoryID: repositoryID,
			FilePath:     "file2.go",
			Content:      "func test() { println(\"hello world\") }",
			StartLine:    1,
			EndLine:      1,
			Type:         "function",
		},
	}

	// Create mocks
	mockEmbeddingService := &MockEmbeddingService{}
	mockChunkRepo := &MockChunkStorageRepository{}

	// Token counting returns specific counts
	mockEmbeddingService.On("CountTokensWithCallback",
		mock.Anything,
		mock.MatchedBy(func(chunks []outbound.CodeChunk) bool {
			return len(chunks) == 2
		}),
		"gemini-embedding-001",
		mock.Anything, // callback
	).Run(func(args mock.Arguments) {
		// Extract arguments
		chunks := args.Get(1).([]outbound.CodeChunk)
		callback := args.Get(3).(outbound.TokenCountCallback)

		// Simulate the callback being invoked for each chunk
		tokenResults := []*outbound.TokenCountResult{
			{TotalTokens: 2, Model: "gemini-embedding-001"},
			{TotalTokens: 8, Model: "gemini-embedding-001"},
		}
		for i := range chunks {
			_ = callback(i, &chunks[i], tokenResults[i])
		}
	}).Return(nil).Once()

	// Verify that SaveChunks is called with chunks that have token counts populated
	var capturedChunks []outbound.CodeChunk
	mockChunkRepo.On("SaveChunks",
		mock.Anything,
		mock.MatchedBy(func(chunks []outbound.CodeChunk) bool {
			capturedChunks = chunks
			return len(chunks) == 2
		}),
	).Return(nil).Once()

	// Mock embedding generation
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return([]*outbound.EmbeddingResult{
		{Vector: make([]float64, 768), Dimensions: 768, Model: "gemini-embedding-001"},
		{Vector: make([]float64, 768), Dimensions: 768, Model: "gemini-embedding-001"},
	}, nil)

	mockChunkRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)

	// Create processor
	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-workspace",
		MaxConcurrentJobs: 1,
		JobTimeout:        30 * time.Second,
	}

	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		UseTestEmbeddings:    false,
		FallbackToSequential: true,
		TokenCounting: config.TokenCountingConfig{
			Enabled: true,
			Mode:    "all",
		},
	}

	processor := NewDefaultJobProcessor(
		processorConfig,
		&MockIndexingJobRepository{},
		&MockRepositoryRepository{},
		&MockEnhancedGitClient{},
		&MockCodeParser{},
		mockEmbeddingService,
		mockChunkRepo,
		&JobProcessorBatchOptions{
			BatchConfig: &batchConfig,
		},
	).(*DefaultJobProcessor)

	// Execute
	err := processor.generateEmbeddings(ctx, indexingJobID, repositoryID, chunks)

	// Assert
	assert.NoError(t, err)
	mockEmbeddingService.AssertExpectations(t)
	mockChunkRepo.AssertExpectations(t)

	// Verify the captured chunks have token counts populated
	assert.Len(t, capturedChunks, 2, "Should have 2 chunks saved")

	// Find chunks by chunk ID
	var chunk1, chunk2 *outbound.CodeChunk
	for i := range capturedChunks {
		if capturedChunks[i].ID == chunk1ID.String() {
			chunk1 = &capturedChunks[i]
		}
		if capturedChunks[i].ID == chunk2ID.String() {
			chunk2 = &capturedChunks[i]
		}
	}

	assert.NotNil(t, chunk1, "Should have chunk1")
	assert.NotNil(t, chunk2, "Should have chunk2")
	assert.Equal(t, 2, chunk1.TokenCount, "Chunk1 should have 2 tokens")
	assert.Equal(t, 8, chunk2.TokenCount, "Chunk2 should have 8 tokens")
	assert.NotNil(t, chunk1.TokenCountedAt, "Chunk1 should have timestamp")
	assert.NotNil(t, chunk2.TokenCountedAt, "Chunk2 should have timestamp")
	assert.Equal(t, repositoryID, chunk1.RepositoryID, "Chunk1 should have RepositoryID set")
	assert.Equal(t, repositoryID, chunk2.RepositoryID, "Chunk2 should have RepositoryID set")
}

// TestJobProcessor_ProgressiveTokenCounting_BatchSaving tests that chunks are saved in batches
// of 50 during progressive token counting.
func TestJobProcessor_ProgressiveTokenCounting_BatchSaving(t *testing.T) {
	t.Parallel()

	// Setup
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()

	// Create 100 test chunks to test batching (expecting 2 batches of 50)
	chunks := make([]outbound.CodeChunk, 100)
	for i := range chunks {
		chunks[i] = outbound.CodeChunk{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			FilePath:     "test.go",
			Content:      "func test() {}",
			StartLine:    i + 1,
			EndLine:      i + 1,
			Type:         "function",
		}
	}

	// Create mocks
	mockEmbeddingService := &MockEmbeddingService{}
	mockChunkRepo := &MockChunkStorageRepository{}
	mockRepoRepo := &MockRepositoryRepository{}
	mockIndexingJobRepo := &MockIndexingJobRepository{}
	mockGitClient := &MockEnhancedGitClient{}
	mockCodeParser := &MockCodeParser{}

	// Track SaveChunks calls - expecting 2 calls with batches of 50
	var saveChunksCalls [][]outbound.CodeChunk
	mockChunkRepo.On("SaveChunks",
		mock.Anything,
		mock.MatchedBy(func(chunks []outbound.CodeChunk) bool {
			// Capture the chunks being saved
			saveChunksCalls = append(saveChunksCalls, chunks)
			return true
		}),
	).Return(nil)

	// Mock CountTokensWithCallback - simulate calling callback for each chunk
	mockEmbeddingService.On("CountTokensWithCallback",
		mock.Anything,
		mock.MatchedBy(func(chunks []outbound.CodeChunk) bool {
			return len(chunks) == 100
		}),
		"gemini-embedding-001",
		mock.Anything, // callback
	).Run(func(args mock.Arguments) {
		// Extract arguments
		chunks := args.Get(1).([]outbound.CodeChunk)
		callback := args.Get(3).(outbound.TokenCountCallback)

		// Simulate the callback being invoked for each chunk
		for i := range chunks {
			result := &outbound.TokenCountResult{
				TotalTokens: 5 + i, // Unique token count for each
				Model:       "gemini-embedding-001",
			}
			// Call the callback with each chunk
			err := callback(i, &chunks[i], result)
			if err != nil {
				// Callback errors should not stop processing
				continue
			}
		}
	}).Return(nil).Once()

	// Mock embedding generation (should still be called)
	embeddingResults := make([]*outbound.EmbeddingResult, 100)
	for i := range embeddingResults {
		embeddingResults[i] = &outbound.EmbeddingResult{
			Vector:     make([]float64, 768),
			Dimensions: 768,
			Model:      "gemini-embedding-001",
		}
	}
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(embeddingResults, nil)

	mockChunkRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)

	// Create processor config
	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-workspace",
		MaxConcurrentJobs: 1,
		JobTimeout:        30 * time.Second,
	}

	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		UseTestEmbeddings:    false,
		FallbackToSequential: true,
		TokenCounting: config.TokenCountingConfig{
			Enabled:           true,
			Mode:              "all",
			MaxTokensPerChunk: 8192,
		},
	}

	// Create processor
	processor := NewDefaultJobProcessor(
		processorConfig,
		mockIndexingJobRepo,
		mockRepoRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbeddingService,
		mockChunkRepo,
		&JobProcessorBatchOptions{
			BatchConfig: &batchConfig,
		},
	).(*DefaultJobProcessor)

	// Execute
	err := processor.generateEmbeddings(ctx, indexingJobID, repositoryID, chunks)

	// Assert
	assert.NoError(t, err, "Expected no error from generateEmbeddings")
	mockEmbeddingService.AssertExpectations(t)

	// Verify SaveChunks was called exactly 2 times (for 100 chunks in batches of 50)
	assert.Len(t, saveChunksCalls, 2, "SaveChunks should be called 2 times for 100 chunks")

	// Verify first batch has 50 chunks
	if len(saveChunksCalls) >= 1 {
		assert.Len(t, saveChunksCalls[0], 50, "First batch should have 50 chunks")
		// Verify all chunks in first batch have token counts
		for i, chunk := range saveChunksCalls[0] {
			assert.NotZero(t, chunk.TokenCount, "Chunk %d in batch 1 should have token count", i)
			assert.NotNil(t, chunk.TokenCountedAt, "Chunk %d in batch 1 should have timestamp", i)
			assert.Equal(t, repositoryID, chunk.RepositoryID, "Chunk %d in batch 1 should have RepositoryID", i)
		}
	}

	// Verify second batch has 50 chunks
	if len(saveChunksCalls) >= 2 {
		assert.Len(t, saveChunksCalls[1], 50, "Second batch should have 50 chunks")
		// Verify all chunks in second batch have token counts
		for i, chunk := range saveChunksCalls[1] {
			assert.NotZero(t, chunk.TokenCount, "Chunk %d in batch 2 should have token count", i)
			assert.NotNil(t, chunk.TokenCountedAt, "Chunk %d in batch 2 should have timestamp", i)
			assert.Equal(t, repositoryID, chunk.RepositoryID, "Chunk %d in batch 2 should have RepositoryID", i)
		}
	}
}

// TestJobProcessor_ProgressiveTokenCounting_FinalPartialBatch tests progressive token counting
// with a final partial batch (75 chunks = 1 batch of 50 + 1 batch of 25).
func TestJobProcessor_ProgressiveTokenCounting_FinalPartialBatch(t *testing.T) {
	t.Parallel()

	// Setup
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()

	// Create 75 test chunks (1 full batch of 50 + partial batch of 25)
	chunks := make([]outbound.CodeChunk, 75)
	for i := range chunks {
		chunks[i] = outbound.CodeChunk{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			FilePath:     "test.go",
			Content:      "func test() {}",
			StartLine:    i + 1,
			EndLine:      i + 1,
			Type:         "function",
		}
	}

	// Create mocks
	mockEmbeddingService := &MockEmbeddingService{}
	mockChunkRepo := &MockChunkStorageRepository{}
	mockRepoRepo := &MockRepositoryRepository{}
	mockIndexingJobRepo := &MockIndexingJobRepository{}
	mockGitClient := &MockEnhancedGitClient{}
	mockCodeParser := &MockCodeParser{}

	// Track SaveChunks calls - expecting 2 calls (50 + 25)
	var saveChunksCalls [][]outbound.CodeChunk
	mockChunkRepo.On("SaveChunks",
		mock.Anything,
		mock.MatchedBy(func(chunks []outbound.CodeChunk) bool {
			// Capture the chunks being saved
			saveChunksCalls = append(saveChunksCalls, chunks)
			return true
		}),
	).Return(nil)

	// Mock CountTokensWithCallback - simulate calling callback for each chunk
	mockEmbeddingService.On("CountTokensWithCallback",
		mock.Anything,
		mock.MatchedBy(func(chunks []outbound.CodeChunk) bool {
			return len(chunks) == 75
		}),
		"gemini-embedding-001",
		mock.Anything, // callback
	).Run(func(args mock.Arguments) {
		// Extract arguments
		chunks := args.Get(1).([]outbound.CodeChunk)
		callback := args.Get(3).(outbound.TokenCountCallback)

		// Simulate the callback being invoked for each chunk
		for i := range chunks {
			result := &outbound.TokenCountResult{
				TotalTokens: 10 + i,
				Model:       "gemini-embedding-001",
			}
			// Call the callback with each chunk
			err := callback(i, &chunks[i], result)
			if err != nil {
				continue
			}
		}
	}).Return(nil).Once()

	// Mock embedding generation
	embeddingResults := make([]*outbound.EmbeddingResult, 75)
	for i := range embeddingResults {
		embeddingResults[i] = &outbound.EmbeddingResult{
			Vector:     make([]float64, 768),
			Dimensions: 768,
			Model:      "gemini-embedding-001",
		}
	}
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(embeddingResults, nil)

	mockChunkRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)

	// Create processor config
	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-workspace",
		MaxConcurrentJobs: 1,
		JobTimeout:        30 * time.Second,
	}

	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		UseTestEmbeddings:    false,
		FallbackToSequential: true,
		TokenCounting: config.TokenCountingConfig{
			Enabled:           true,
			Mode:              "all",
			MaxTokensPerChunk: 8192,
		},
	}

	// Create processor
	processor := NewDefaultJobProcessor(
		processorConfig,
		mockIndexingJobRepo,
		mockRepoRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbeddingService,
		mockChunkRepo,
		&JobProcessorBatchOptions{
			BatchConfig: &batchConfig,
		},
	).(*DefaultJobProcessor)

	// Execute
	err := processor.generateEmbeddings(ctx, indexingJobID, repositoryID, chunks)

	// Assert
	assert.NoError(t, err, "Expected no error from generateEmbeddings")
	mockEmbeddingService.AssertExpectations(t)

	// Verify SaveChunks was called exactly 2 times (50 + 25)
	assert.Len(t, saveChunksCalls, 2, "SaveChunks should be called 2 times for 75 chunks")

	// Verify first batch has 50 chunks
	if len(saveChunksCalls) >= 1 {
		assert.Len(t, saveChunksCalls[0], 50, "First batch should have 50 chunks")
		for i, chunk := range saveChunksCalls[0] {
			assert.NotZero(t, chunk.TokenCount, "Chunk %d in batch 1 should have token count", i)
			assert.NotNil(t, chunk.TokenCountedAt, "Chunk %d in batch 1 should have timestamp", i)
		}
	}

	// Verify second batch has 25 chunks (partial batch)
	if len(saveChunksCalls) >= 2 {
		assert.Len(t, saveChunksCalls[1], 25, "Second batch should have 25 chunks (partial)")
		for i, chunk := range saveChunksCalls[1] {
			assert.NotZero(t, chunk.TokenCount, "Chunk %d in batch 2 should have token count", i)
			assert.NotNil(t, chunk.TokenCountedAt, "Chunk %d in batch 2 should have timestamp", i)
		}
	}
}

// TestJobProcessor_ProgressiveTokenCounting_GracefulDegradation tests that if SaveChunks
// fails during progressive token counting, processing continues and embeddings are still generated.
func TestJobProcessor_ProgressiveTokenCounting_GracefulDegradation(t *testing.T) {
	t.Parallel()

	// Setup
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()

	// Create 100 test chunks
	chunks := make([]outbound.CodeChunk, 100)
	for i := range chunks {
		chunks[i] = outbound.CodeChunk{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			FilePath:     "test.go",
			Content:      "func test() {}",
			StartLine:    i + 1,
			EndLine:      i + 1,
			Type:         "function",
		}
	}

	// Create mocks
	mockEmbeddingService := &MockEmbeddingService{}
	mockChunkRepo := &MockChunkStorageRepository{}
	mockRepoRepo := &MockRepositoryRepository{}
	mockIndexingJobRepo := &MockIndexingJobRepository{}
	mockGitClient := &MockEnhancedGitClient{}
	mockCodeParser := &MockCodeParser{}

	// Track SaveChunks calls
	var saveChunksCallCount int
	mockChunkRepo.On("SaveChunks",
		mock.Anything,
		mock.Anything,
	).Return(func(ctx context.Context, chunks []outbound.CodeChunk) error {
		saveChunksCallCount++
		// First batch fails, second batch succeeds
		if saveChunksCallCount == 1 {
			return errors.New("database connection error")
		}
		return nil
	})

	// Mock CountTokensWithCallback - simulate calling callback for each chunk
	mockEmbeddingService.On("CountTokensWithCallback",
		mock.Anything,
		mock.MatchedBy(func(chunks []outbound.CodeChunk) bool {
			return len(chunks) == 100
		}),
		"gemini-embedding-001",
		mock.Anything, // callback
	).Run(func(args mock.Arguments) {
		// Extract arguments
		chunks := args.Get(1).([]outbound.CodeChunk)
		callback := args.Get(3).(outbound.TokenCountCallback)

		// Simulate the callback being invoked for each chunk
		for i := range chunks {
			result := &outbound.TokenCountResult{
				TotalTokens: 8,
				Model:       "gemini-embedding-001",
			}
			// Call the callback - errors should be logged but not stop processing
			_ = callback(i, &chunks[i], result)
		}
	}).Return(nil).Once()

	// Mock embedding generation - MUST still be called even if SaveChunks fails
	embeddingResults := make([]*outbound.EmbeddingResult, 100)
	for i := range embeddingResults {
		embeddingResults[i] = &outbound.EmbeddingResult{
			Vector:     make([]float64, 768),
			Dimensions: 768,
			Model:      "gemini-embedding-001",
		}
	}
	mockEmbeddingService.On("GenerateBatchEmbeddings",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(embeddingResults, nil).Once()

	mockChunkRepo.On("SaveChunkWithEmbedding",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)

	// Create processor config
	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-workspace",
		MaxConcurrentJobs: 1,
		JobTimeout:        30 * time.Second,
	}

	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      1,
		UseTestEmbeddings:    false,
		FallbackToSequential: true,
		TokenCounting: config.TokenCountingConfig{
			Enabled:           true,
			Mode:              "all",
			MaxTokensPerChunk: 8192,
		},
	}

	// Create processor
	processor := NewDefaultJobProcessor(
		processorConfig,
		mockIndexingJobRepo,
		mockRepoRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbeddingService,
		mockChunkRepo,
		&JobProcessorBatchOptions{
			BatchConfig: &batchConfig,
		},
	).(*DefaultJobProcessor)

	// Execute
	err := processor.generateEmbeddings(ctx, indexingJobID, repositoryID, chunks)

	// Assert - embedding generation should succeed despite SaveChunks failure
	assert.NoError(t, err, "Embedding generation should succeed even if SaveChunks fails during token counting")

	// Verify CountTokensWithCallback was called
	mockEmbeddingService.AssertCalled(
		t,
		"CountTokensWithCallback",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	)

	// Verify GenerateBatchEmbeddings was still called (no blocking from SaveChunks failure)
	mockEmbeddingService.AssertCalled(t, "GenerateBatchEmbeddings", mock.Anything, mock.Anything, mock.Anything)

	// Verify SaveChunks was called at least once (batch saving was attempted)
	assert.GreaterOrEqual(t, saveChunksCallCount, 1, "SaveChunks should be called at least once")
}
