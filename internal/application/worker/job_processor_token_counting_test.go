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

	// Extract texts from chunks for CountTokensBatch call
	expectedTexts := []string{
		chunks[0].Content,
		chunks[1].Content,
	}

	// Mock expectations
	// 1. CountTokensBatch should be called BEFORE GenerateBatchEmbeddings
	mockEmbeddingService.On("CountTokensBatch",
		mock.Anything, // ctx
		expectedTexts,
		"gemini-embedding-001",
	).Return(expectedTokenCounts, nil).Once()

	// 2. Chunks should be updated with token counts
	mockChunkRepo.On("UpdateTokenCounts",
		mock.Anything, // ctx
		mock.MatchedBy(func(updates []outbound.ChunkTokenUpdate) bool {
			// Verify that we're updating both chunks with the correct token counts
			if len(updates) != 2 {
				return false
			}
			// Check first chunk
			if updates[0].TokenCount != 10 {
				return false
			}
			// Check second chunk
			if updates[1].TokenCount != 6 {
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

	// Verify call order: CountTokensBatch should be called before GenerateBatchEmbeddings
	calls := mockEmbeddingService.Calls
	countTokensIdx, generateEmbeddingsIdx := -1, -1
	for i, call := range calls {
		if call.Method == "CountTokensBatch" {
			countTokensIdx = i
		}
		if call.Method == "GenerateBatchEmbeddings" {
			generateEmbeddingsIdx = i
		}
	}

	assert.NotEqual(t, -1, countTokensIdx, "CountTokensBatch should have been called")
	assert.NotEqual(t, -1, generateEmbeddingsIdx, "GenerateBatchEmbeddings should have been called")
	assert.Less(
		t,
		countTokensIdx,
		generateEmbeddingsIdx,
		"CountTokensBatch must be called BEFORE GenerateBatchEmbeddings",
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
				// CountTokensBatch should be called with sampled chunks
				mockEmbeddingService.On("CountTokensBatch",
					mock.Anything,
					mock.MatchedBy(func(texts []string) bool {
						return len(texts) == tc.expectedSamples
					}),
					"gemini-embedding-001",
				).Return(func(ctx context.Context, texts []string, model string) []*outbound.TokenCountResult {
					results := make([]*outbound.TokenCountResult, len(texts))
					for i := range results {
						results[i] = &outbound.TokenCountResult{
							TotalTokens: 5,
							Model:       model,
						}
					}
					return results
				}, nil).Once()

				// UpdateTokenCounts should be called for sampled chunks
				mockChunkRepo.On("UpdateTokenCounts",
					mock.Anything,
					mock.MatchedBy(func(updates []outbound.ChunkTokenUpdate) bool {
						return len(updates) == tc.expectedSamples
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

			// Mock CountTokensBatch - may fail or succeed
			if tc.tokenCountErr != nil {
				mockEmbeddingService.On("CountTokensBatch",
					mock.Anything,
					mock.Anything,
					"gemini-embedding-001",
				).Return(nil, tc.tokenCountErr).Once()

				// UpdateTokenCounts should NOT be called on error
				// (no expectation set means test fails if called)
			} else {
				mockEmbeddingService.On("CountTokensBatch",
					mock.Anything,
					mock.Anything,
					"gemini-embedding-001",
				).Return([]*outbound.TokenCountResult{
					{TotalTokens: 5, Model: "gemini-embedding-001"},
				}, nil).Once()

				mockChunkRepo.On("UpdateTokenCounts",
					mock.Anything,
					mock.Anything,
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
	mockEmbeddingService.On("CountTokensBatch",
		mock.Anything,
		[]string{chunks[0].Content, chunks[1].Content},
		"gemini-embedding-001",
	).Return([]*outbound.TokenCountResult{
		{TotalTokens: 2, Model: "gemini-embedding-001"},
		{TotalTokens: 8, Model: "gemini-embedding-001"},
	}, nil).Once()

	// Verify that UpdateTokenCounts is called with correct values
	var capturedUpdates []outbound.ChunkTokenUpdate
	mockChunkRepo.On("UpdateTokenCounts",
		mock.Anything,
		mock.MatchedBy(func(updates []outbound.ChunkTokenUpdate) bool {
			capturedUpdates = updates
			return len(updates) == 2
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

	// Verify the captured token count updates
	assert.Len(t, capturedUpdates, 2, "Should have 2 token count updates")

	// Find updates by chunk ID
	var update1, update2 *outbound.ChunkTokenUpdate
	for i := range capturedUpdates {
		if capturedUpdates[i].ChunkID.String() == chunk1ID.String() {
			update1 = &capturedUpdates[i]
		}
		if capturedUpdates[i].ChunkID.String() == chunk2ID.String() {
			update2 = &capturedUpdates[i]
		}
	}

	assert.NotNil(t, update1, "Should have update for chunk1")
	assert.NotNil(t, update2, "Should have update for chunk2")
	assert.Equal(t, 2, update1.TokenCount, "Chunk1 should have 2 tokens")
	assert.Equal(t, 8, update2.TokenCount, "Chunk2 should have 8 tokens")
	assert.NotNil(t, update1.TokenCountedAt, "Chunk1 should have timestamp")
	assert.NotNil(t, update2.TokenCountedAt, "Chunk2 should have timestamp")
}
