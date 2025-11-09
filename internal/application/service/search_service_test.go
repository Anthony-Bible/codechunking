package service

import (
	"codechunking/internal/application/common/logging"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/dto"
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

// MockVectorStorageRepository is a mock implementation of VectorStorageRepository for testing.
type MockVectorStorageRepository struct {
	mock.Mock
}

func (m *MockVectorStorageRepository) VectorSimilaritySearch(
	ctx context.Context,
	queryVector []float64,
	options outbound.SimilaritySearchOptions,
) ([]outbound.VectorSimilarityResult, error) {
	args := m.Called(ctx, queryVector, options)
	return args.Get(0).([]outbound.VectorSimilarityResult), args.Error(1)
}

// Implement other required methods with panics since we only test VectorSimilaritySearch.
func (m *MockVectorStorageRepository) BulkInsertEmbeddings(
	ctx context.Context,
	embeddings []outbound.VectorEmbedding,
	options outbound.BulkInsertOptions,
) (*outbound.BulkInsertResult, error) {
	panic("not implemented for search tests")
}

func (m *MockVectorStorageRepository) UpsertEmbedding(
	ctx context.Context,
	embedding outbound.VectorEmbedding,
	options outbound.UpsertOptions,
) error {
	panic("not implemented for search tests")
}

func (m *MockVectorStorageRepository) FindEmbeddingByChunkID(
	ctx context.Context,
	chunkID uuid.UUID,
	modelVersion string,
) (*outbound.VectorEmbedding, error) {
	panic("not implemented for search tests")
}

func (m *MockVectorStorageRepository) FindEmbeddingsByRepositoryID(
	ctx context.Context,
	repositoryID uuid.UUID,
	filters outbound.EmbeddingFilters,
) ([]outbound.VectorEmbedding, error) {
	panic("not implemented for search tests")
}

func (m *MockVectorStorageRepository) DeleteEmbeddingsByChunkIDs(
	ctx context.Context,
	chunkIDs []uuid.UUID,
	options outbound.DeleteOptions,
) (*outbound.BulkDeleteResult, error) {
	panic("not implemented for search tests")
}

func (m *MockVectorStorageRepository) DeleteEmbeddingsByRepositoryID(
	ctx context.Context,
	repositoryID uuid.UUID,
	options outbound.DeleteOptions,
) (*outbound.BulkDeleteResult, error) {
	panic("not implemented for search tests")
}

func (m *MockVectorStorageRepository) GetStorageStatistics(
	ctx context.Context,
	options outbound.StatisticsOptions,
) (*outbound.StorageStatistics, error) {
	panic("not implemented for search tests")
}

func (m *MockVectorStorageRepository) BeginTransaction(ctx context.Context) (outbound.VectorTransaction, error) {
	panic("not implemented for search tests")
}

// MockEmbeddingService is a mock implementation of EmbeddingService for testing.
type MockEmbeddingService struct {
	mock.Mock
}

func (m *MockEmbeddingService) GenerateEmbedding(
	ctx context.Context,
	text string,
	options outbound.EmbeddingOptions,
) (*outbound.EmbeddingResult, error) {
	args := m.Called(ctx, text, options)
	return args.Get(0).(*outbound.EmbeddingResult), args.Error(1)
}

func (m *MockEmbeddingService) GenerateBatchEmbeddings(
	ctx context.Context,
	texts []string,
	options outbound.EmbeddingOptions,
) ([]*outbound.EmbeddingResult, error) {
	args := m.Called(ctx, texts, options)
	return args.Get(0).([]*outbound.EmbeddingResult), args.Error(1)
}

func (m *MockEmbeddingService) GenerateCodeChunkEmbedding(
	ctx context.Context,
	chunk *outbound.CodeChunk,
	options outbound.EmbeddingOptions,
) (*outbound.CodeChunkEmbedding, error) {
	args := m.Called(ctx, chunk, options)
	return args.Get(0).(*outbound.CodeChunkEmbedding), args.Error(1)
}

func (m *MockEmbeddingService) ValidateApiKey(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockEmbeddingService) GetModelInfo(ctx context.Context) (*outbound.ModelInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).(*outbound.ModelInfo), args.Error(1)
}

func (m *MockEmbeddingService) GetSupportedModels(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockEmbeddingService) EstimateTokenCount(ctx context.Context, text string) (int, error) {
	args := m.Called(ctx, text)
	return args.Int(0), args.Error(1)
}

// MockChunkRepository is a mock implementation for retrieving chunk information.
type MockChunkRepository struct {
	mock.Mock
}

func (m *MockChunkRepository) FindChunksByIDs(ctx context.Context, chunkIDs []uuid.UUID) ([]ChunkInfo, error) {
	args := m.Called(ctx, chunkIDs)
	return args.Get(0).([]ChunkInfo), args.Error(1)
}

// TestSearchService exercises the main search orchestration path and error branches.
func TestSearchService(t *testing.T) {
	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)
	t.Run("Valid_Semantic_Search_Success", func(t *testing.T) {
		// Setup mocks
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		// Create service instance
		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)
		require.NotNil(t, searchService, "SearchService should be created successfully")

		// Setup test data
		ctx := context.Background()
		searchRequest := dto.SearchRequestDTO{
			Query:               "implement authentication middleware",
			Limit:               10,
			Offset:              0,
			SimilarityThreshold: 0.7,
			Sort:                "similarity:desc",
		}

		// Expected query vector from embedding service
		expectedVector := []float64{0.1, 0.2, 0.3, 0.4, 0.5}

		// Mock embedding generation
		mockEmbedding := &outbound.EmbeddingResult{
			Vector:      expectedVector,
			Dimensions:  len(expectedVector),
			Model:       "gemini-embedding-001",
			GeneratedAt: time.Now(),
		}
		mockEmbeddingService.On("GenerateEmbedding", ctx, searchRequest.Query, mock.AnythingOfType("outbound.EmbeddingOptions")).
			Return(mockEmbedding, nil)

		// Mock vector similarity search
		chunkID1 := uuid.New()
		chunkID2 := uuid.New()
		mockVectorResults := []outbound.VectorSimilarityResult{
			{
				Embedding: outbound.VectorEmbedding{
					ChunkID: chunkID1,
				},
				Similarity: 0.95,
				Distance:   0.05,
				Rank:       1,
			},
			{
				Embedding: outbound.VectorEmbedding{
					ChunkID: chunkID2,
				},
				Similarity: 0.85,
				Distance:   0.15,
				Rank:       2,
			},
		}

		expectedSearchOptions := outbound.SimilaritySearchOptions{
			UsePartitionedTable: true,
			MaxResults:          10,
			MinSimilarity:       0.7,
		}

		mockVectorRepo.On("VectorSimilaritySearch", ctx, expectedVector, expectedSearchOptions).
			Return(mockVectorResults, nil)

		// Mock chunk repository
		repoID := uuid.New()
		mockChunks := []ChunkInfo{
			{
				ChunkID: chunkID1,
				Content: "func authenticateUser(token string) error { return validateToken(token) }",
				Repository: dto.RepositoryInfo{
					ID:   repoID,
					Name: "auth-service",
					URL:  "https://github.com/example/auth-service.git",
				},
				FilePath:  "/middleware/auth.go",
				Language:  "go",
				StartLine: 10,
				EndLine:   15,
			},
			{
				ChunkID: chunkID2,
				Content: "func requireAuth() middleware.Handler { return func(ctx context.Context) { /* auth logic */ } }",
				Repository: dto.RepositoryInfo{
					ID:   repoID,
					Name: "auth-service",
					URL:  "https://github.com/example/auth-service.git",
				},
				FilePath:  "/middleware/require.go",
				Language:  "go",
				StartLine: 25,
				EndLine:   30,
			},
		}

		mockChunkRepo.On("FindChunksByIDs", ctx, []uuid.UUID{chunkID1, chunkID2}).
			Return(mockChunks, nil)

		// Execute search
		result, err := searchService.Search(ctx, searchRequest)

		// Verify results
		require.NoError(t, err, "Search should succeed")
		require.NotNil(t, result, "Search result should not be nil")

		assert.Len(t, result.Results, 2, "Should return 2 search results")
		assert.Equal(t, 10, result.Pagination.Limit, "Pagination limit should match request")
		assert.Equal(t, 0, result.Pagination.Offset, "Pagination offset should match request")
		assert.Equal(t, 2, result.Pagination.Total, "Pagination total should match result count")
		assert.False(t, result.Pagination.HasMore, "Should not have more results")

		// Verify first result (highest similarity)
		firstResult := result.Results[0]
		assert.Equal(t, chunkID1, firstResult.ChunkID, "First result should have highest similarity chunk")
		assert.Equal(t, 0.95, firstResult.SimilarityScore, "First result similarity should be 0.95")
		assert.Contains(t, firstResult.Content, "authenticateUser", "Content should contain function name")
		assert.Equal(t, "go", firstResult.Language, "Language should be go")
		assert.Equal(t, "/middleware/auth.go", firstResult.FilePath, "File path should match")

		// Verify search metadata
		assert.Equal(t, searchRequest.Query, result.Metadata.Query, "Metadata query should match request")
		assert.Positive(t, result.Metadata.ExecutionTimeMs, "Execution time should be positive")

		// Verify all mocks were called as expected
		mockEmbeddingService.AssertExpectations(t)
		mockVectorRepo.AssertExpectations(t)
		mockChunkRepo.AssertExpectations(t)
	})

	t.Run("Search_With_Repository_Filtering", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		ctx := context.Background()
		filterRepoID := uuid.New()
		searchRequest := dto.SearchRequestDTO{
			Query:               "database connection",
			Limit:               5,
			RepositoryIDs:       []uuid.UUID{filterRepoID},
			SimilarityThreshold: 0.8,
		}

		// Mock embedding generation
		queryVector := []float64{0.2, 0.4, 0.6}
		mockEmbedding := &outbound.EmbeddingResult{
			Vector: queryVector,
		}
		mockEmbeddingService.On("GenerateEmbedding", ctx, searchRequest.Query, mock.AnythingOfType("outbound.EmbeddingOptions")).
			Return(mockEmbedding, nil)

		// Verify that repository filtering is applied to search options
		expectedSearchOptions := outbound.SimilaritySearchOptions{
			UsePartitionedTable: true,
			MaxResults:          5,
			MinSimilarity:       0.8,
			RepositoryIDs:       []uuid.UUID{filterRepoID},
		}

		mockVectorRepo.On("VectorSimilaritySearch", ctx, queryVector, expectedSearchOptions).
			Return([]outbound.VectorSimilarityResult{}, nil)

		mockChunkRepo.On("FindChunksByIDs", ctx, []uuid.UUID{}).
			Return([]ChunkInfo{}, nil)

		result, err := searchService.Search(ctx, searchRequest)

		require.NoError(t, err, "Filtered search should succeed")
		assert.Empty(t, result.Results, "Should return no results for this test case")

		mockVectorRepo.AssertExpectations(t)
		mockEmbeddingService.AssertExpectations(t)
	})

	t.Run("Search_With_Repository_Names_Filtering", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		ctx := context.Background()
		searchRequest := dto.SearchRequestDTO{
			Query:           "authentication middleware",
			Limit:           10,
			RepositoryNames: []string{"golang/go", "facebook/react"},
		}

		// Mock embedding generation
		queryVector := []float64{0.1, 0.3, 0.5}
		mockEmbedding := &outbound.EmbeddingResult{Vector: queryVector}
		mockEmbeddingService.On("GenerateEmbedding", ctx, searchRequest.Query, mock.AnythingOfType("outbound.EmbeddingOptions")).
			Return(mockEmbedding, nil)

		// Mock repository name resolution - these should be resolved to UUIDs
		resolvedRepoIDs := []uuid.UUID{uuid.New(), uuid.New()}

		// Verify that repository names are resolved to IDs and applied to search options
		expectedSearchOptions := outbound.SimilaritySearchOptions{
			UsePartitionedTable: true,
			MaxResults:          10,
			MinSimilarity:       0.7, // Default threshold
			RepositoryIDs:       resolvedRepoIDs,
		}

		mockVectorRepo.On("VectorSimilaritySearch", ctx, queryVector, expectedSearchOptions).
			Return([]outbound.VectorSimilarityResult{}, nil)

		mockChunkRepo.On("FindChunksByIDs", ctx, []uuid.UUID{}).
			Return([]ChunkInfo{}, nil)

		result, err := searchService.Search(ctx, searchRequest)

		require.NoError(t, err, "Repository names filtered search should succeed")
		assert.NotNil(t, result, "Result should not be nil")

		mockVectorRepo.AssertExpectations(t)
		mockEmbeddingService.AssertExpectations(t)
	})

	t.Run("Search_With_Repository_Names_And_Repository_IDs_Mixed", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		ctx := context.Background()
		existingRepoID := uuid.New()
		searchRequest := dto.SearchRequestDTO{
			Query:           "mixed filtering test",
			Limit:           15,
			RepositoryIDs:   []uuid.UUID{existingRepoID},
			RepositoryNames: []string{"golang/go"},
		}

		queryVector := []float64{0.2, 0.4, 0.6}
		mockEmbedding := &outbound.EmbeddingResult{Vector: queryVector}
		mockEmbeddingService.On("GenerateEmbedding", ctx, searchRequest.Query, mock.AnythingOfType("outbound.EmbeddingOptions")).
			Return(mockEmbedding, nil)

		// Both existing IDs and resolved names should be combined
		resolvedRepoID := uuid.New()
		expectedSearchOptions := outbound.SimilaritySearchOptions{
			UsePartitionedTable: true,
			MaxResults:          15,
			MinSimilarity:       0.7,
			RepositoryIDs:       []uuid.UUID{existingRepoID, resolvedRepoID}, // Combined
		}

		mockVectorRepo.On("VectorSimilaritySearch", ctx, queryVector, expectedSearchOptions).
			Return([]outbound.VectorSimilarityResult{}, nil)

		mockChunkRepo.On("FindChunksByIDs", ctx, []uuid.UUID{}).
			Return([]ChunkInfo{}, nil)

		result, err := searchService.Search(ctx, searchRequest)

		require.NoError(t, err, "Mixed filtering search should succeed")
		assert.NotNil(t, result, "Result should not be nil")

		mockVectorRepo.AssertExpectations(t)
		mockEmbeddingService.AssertExpectations(t)
	})

	t.Run("Search_Repository_Name_Resolution_Failure", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		ctx := context.Background()
		searchRequest := dto.SearchRequestDTO{
			Query:           "test query",
			RepositoryNames: []string{"nonexistent/repo"},
		}

		queryVector := []float64{0.1, 0.2, 0.3}
		mockEmbedding := &outbound.EmbeddingResult{Vector: queryVector}
		mockEmbeddingService.On("GenerateEmbedding", ctx, searchRequest.Query, mock.AnythingOfType("outbound.EmbeddingOptions")).
			Return(mockEmbedding, nil)

		result, err := searchService.Search(ctx, searchRequest)

		// Should fail when repository name cannot be resolved
		assert.Error(t, err, "Should return error when repository name resolution fails")
		assert.Nil(t, result, "Result should be nil on resolution failure")
		assert.Contains(t, err.Error(), "failed to resolve repository names", "Error message should indicate resolution failure")

		mockEmbeddingService.AssertExpectations(t)
	})

	t.Run("Search_Repository_Name_Duplicate_Handling", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		ctx := context.Background()
		searchRequest := dto.SearchRequestDTO{
			Query:           "duplicate test",
			RepositoryNames: []string{"golang/go", "golang/go"}, // Duplicate
		}

		queryVector := []float64{0.1, 0.2, 0.3}
		mockEmbedding := &outbound.EmbeddingResult{Vector: queryVector}
		mockEmbeddingService.On("GenerateEmbedding", ctx, searchRequest.Query, mock.AnythingOfType("outbound.EmbeddingOptions")).
			Return(mockEmbedding, nil)

		result, err := searchService.Search(ctx, searchRequest)

		// Should fail due to duplicate repository names
		assert.Error(t, err, "Should return error for duplicate repository names")
		assert.Nil(t, result, "Result should be nil for duplicate names")
		assert.Contains(t, err.Error(), "repository_names cannot contain duplicates", "Error message should indicate duplicate names")

		mockEmbeddingService.AssertExpectations(t)
	})

	t.Run("Search_With_Language_And_FileType_Filtering", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		ctx := context.Background()
		searchRequest := dto.SearchRequestDTO{
			Query:     "error handling",
			Languages: []string{"go", "python"},
			FileTypes: []string{".go", ".py"},
		}

		queryVector := []float64{0.1, 0.3, 0.5}
		mockEmbedding := &outbound.EmbeddingResult{Vector: queryVector}
		mockEmbeddingService.On("GenerateEmbedding", ctx, searchRequest.Query, mock.AnythingOfType("outbound.EmbeddingOptions")).
			Return(mockEmbedding, nil)

		// The service should apply language and file type filters through chunk filtering
		mockVectorRepo.On("VectorSimilaritySearch", ctx, queryVector, mock.AnythingOfType("outbound.SimilaritySearchOptions")).
			Return([]outbound.VectorSimilarityResult{}, nil)

		mockChunkRepo.On("FindChunksByIDs", ctx, mock.AnythingOfType("[]uuid.UUID")).
			Return([]ChunkInfo{}, nil)

		result, err := searchService.Search(ctx, searchRequest)

		require.NoError(t, err, "Language filtered search should succeed")
		assert.NotNil(t, result, "Result should not be nil")

		mockVectorRepo.AssertExpectations(t)
		mockEmbeddingService.AssertExpectations(t)
	})

	t.Run("Search_Embedding_Generation_Failure", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		ctx := context.Background()
		searchRequest := dto.SearchRequestDTO{
			Query: "test query",
		}

		// Mock embedding service failure
		embeddingError := errors.New("embedding service unavailable")
		mockEmbeddingService.On("GenerateEmbedding", ctx, searchRequest.Query, mock.AnythingOfType("outbound.EmbeddingOptions")).
			Return((*outbound.EmbeddingResult)(nil), embeddingError)

		result, err := searchService.Search(ctx, searchRequest)

		assert.Error(t, err, "Should return error when embedding generation fails")
		assert.Nil(t, result, "Result should be nil on error")
		assert.Contains(
			t,
			err.Error(),
			"failed to generate embedding",
			"Error message should indicate embedding failure",
		)

		mockEmbeddingService.AssertExpectations(t)
	})

	t.Run("Search_Vector_Search_Failure", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		ctx := context.Background()
		searchRequest := dto.SearchRequestDTO{
			Query: "test query",
		}

		queryVector := []float64{0.1, 0.2, 0.3}
		mockEmbedding := &outbound.EmbeddingResult{Vector: queryVector}
		mockEmbeddingService.On("GenerateEmbedding", ctx, searchRequest.Query, mock.AnythingOfType("outbound.EmbeddingOptions")).
			Return(mockEmbedding, nil)

		// Mock vector search failure
		searchError := errors.New("vector search database error")
		mockVectorRepo.On("VectorSimilaritySearch", ctx, queryVector, mock.AnythingOfType("outbound.SimilaritySearchOptions")).
			Return(([]outbound.VectorSimilarityResult)(nil), searchError)

		result, err := searchService.Search(ctx, searchRequest)

		assert.Error(t, err, "Should return error when vector search fails")
		assert.Nil(t, result, "Result should be nil on error")
		assert.Contains(t, err.Error(), "vector search failed", "Error message should indicate search failure")

		mockEmbeddingService.AssertExpectations(t)
		mockVectorRepo.AssertExpectations(t)
	})

	t.Run("Search_Chunk_Retrieval_Failure", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		ctx := context.Background()
		searchRequest := dto.SearchRequestDTO{
			Query: "test query",
		}

		queryVector := []float64{0.1, 0.2, 0.3}
		mockEmbedding := &outbound.EmbeddingResult{Vector: queryVector}
		mockEmbeddingService.On("GenerateEmbedding", ctx, searchRequest.Query, mock.AnythingOfType("outbound.EmbeddingOptions")).
			Return(mockEmbedding, nil)

		chunkID := uuid.New()
		mockVectorResults := []outbound.VectorSimilarityResult{
			{
				Embedding:  outbound.VectorEmbedding{ChunkID: chunkID},
				Similarity: 0.9,
			},
		}
		mockVectorRepo.On("VectorSimilaritySearch", ctx, queryVector, mock.AnythingOfType("outbound.SimilaritySearchOptions")).
			Return(mockVectorResults, nil)

		// Mock chunk retrieval failure
		chunkError := errors.New("chunk database error")
		mockChunkRepo.On("FindChunksByIDs", ctx, []uuid.UUID{chunkID}).
			Return(([]ChunkInfo)(nil), chunkError)

		result, err := searchService.Search(ctx, searchRequest)

		assert.Error(t, err, "Should return error when chunk retrieval fails")
		assert.Nil(t, result, "Result should be nil on error")
		assert.Contains(
			t,
			err.Error(),
			"failed to retrieve chunks",
			"Error message should indicate chunk retrieval failure",
		)

		mockEmbeddingService.AssertExpectations(t)
		mockVectorRepo.AssertExpectations(t)
		mockChunkRepo.AssertExpectations(t)
	})

	t.Run("Search_Sort_Results_By_Similarity_Descending", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		ctx := context.Background()
		searchRequest := dto.SearchRequestDTO{
			Query: "test query",
			Sort:  "similarity:desc",
		}

		queryVector := []float64{0.1, 0.2, 0.3}
		mockEmbedding := &outbound.EmbeddingResult{Vector: queryVector}
		mockEmbeddingService.On("GenerateEmbedding", ctx, searchRequest.Query, mock.AnythingOfType("outbound.EmbeddingOptions")).
			Return(mockEmbedding, nil)

		// Return results in non-sorted order to test sorting
		chunkID1 := uuid.New()
		chunkID2 := uuid.New()
		chunkID3 := uuid.New()
		mockVectorResults := []outbound.VectorSimilarityResult{
			{Embedding: outbound.VectorEmbedding{ChunkID: chunkID1}, Similarity: 0.7},  // Middle
			{Embedding: outbound.VectorEmbedding{ChunkID: chunkID2}, Similarity: 0.95}, // Highest
			{Embedding: outbound.VectorEmbedding{ChunkID: chunkID3}, Similarity: 0.6},  // Lowest
		}
		mockVectorRepo.On("VectorSimilaritySearch", ctx, queryVector, mock.AnythingOfType("outbound.SimilaritySearchOptions")).
			Return(mockVectorResults, nil)

		mockChunks := []ChunkInfo{
			{ChunkID: chunkID1, Content: "middle result"},
			{ChunkID: chunkID2, Content: "highest result"},
			{ChunkID: chunkID3, Content: "lowest result"},
		}
		mockChunkRepo.On("FindChunksByIDs", ctx, mock.AnythingOfType("[]uuid.UUID")).
			Return(mockChunks, nil)

		result, err := searchService.Search(ctx, searchRequest)

		require.NoError(t, err, "Search should succeed")
		require.Len(t, result.Results, 3, "Should return 3 results")

		// Verify results are sorted by similarity descending
		assert.Equal(t, 0.95, result.Results[0].SimilarityScore, "First result should have highest similarity")
		assert.Equal(t, 0.7, result.Results[1].SimilarityScore, "Second result should have middle similarity")
		assert.Equal(t, 0.6, result.Results[2].SimilarityScore, "Third result should have lowest similarity")

		assert.Contains(t, result.Results[0].Content, "highest result", "First result content should match")
		assert.Contains(t, result.Results[1].Content, "middle result", "Second result content should match")
		assert.Contains(t, result.Results[2].Content, "lowest result", "Third result content should match")
	})

	t.Run("Search_Sort_Results_By_FilePath_Ascending", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		ctx := context.Background()
		searchRequest := dto.SearchRequestDTO{
			Query: "test query",
			Sort:  "file_path:asc",
		}

		queryVector := []float64{0.1, 0.2, 0.3}
		mockEmbedding := &outbound.EmbeddingResult{Vector: queryVector}
		mockEmbeddingService.On("GenerateEmbedding", ctx, searchRequest.Query, mock.AnythingOfType("outbound.EmbeddingOptions")).
			Return(mockEmbedding, nil)

		chunkID1 := uuid.New()
		chunkID2 := uuid.New()
		chunkID3 := uuid.New()
		mockVectorResults := []outbound.VectorSimilarityResult{
			{Embedding: outbound.VectorEmbedding{ChunkID: chunkID1}, Similarity: 0.8},
			{Embedding: outbound.VectorEmbedding{ChunkID: chunkID2}, Similarity: 0.9},
			{Embedding: outbound.VectorEmbedding{ChunkID: chunkID3}, Similarity: 0.7},
		}
		mockVectorRepo.On("VectorSimilaritySearch", ctx, queryVector, mock.AnythingOfType("outbound.SimilaritySearchOptions")).
			Return(mockVectorResults, nil)

		// Return chunks with file paths in non-alphabetical order to test sorting
		mockChunks := []ChunkInfo{
			{ChunkID: chunkID1, FilePath: "/src/middleware.go", Content: "middleware content"},
			{ChunkID: chunkID2, FilePath: "/src/auth.go", Content: "auth content"},   // Should be first
			{ChunkID: chunkID3, FilePath: "/src/utils.go", Content: "utils content"}, // Should be last
		}
		mockChunkRepo.On("FindChunksByIDs", ctx, mock.AnythingOfType("[]uuid.UUID")).
			Return(mockChunks, nil)

		result, err := searchService.Search(ctx, searchRequest)

		require.NoError(t, err, "Search should succeed")
		require.Len(t, result.Results, 3, "Should return 3 results")

		// Verify results are sorted by file path ascending
		assert.Equal(t, "/src/auth.go", result.Results[0].FilePath, "First result should be auth.go")
		assert.Equal(t, "/src/middleware.go", result.Results[1].FilePath, "Second result should be middleware.go")
		assert.Equal(t, "/src/utils.go", result.Results[2].FilePath, "Third result should be utils.go")

		assert.Contains(t, result.Results[0].Content, "auth content", "First result content should match")
		assert.Contains(t, result.Results[1].Content, "middleware content", "Second result content should match")
		assert.Contains(t, result.Results[2].Content, "utils content", "Third result content should match")
	})

	t.Run("Search_Pagination_Applied_Correctly", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		ctx := context.Background()
		searchRequest := dto.SearchRequestDTO{
			Query:  "test query",
			Limit:  2, // Only return 2 results
			Offset: 1, // Skip first result
		}

		queryVector := []float64{0.1, 0.2, 0.3}
		mockEmbedding := &outbound.EmbeddingResult{Vector: queryVector}
		mockEmbeddingService.On("GenerateEmbedding", ctx, searchRequest.Query, mock.AnythingOfType("outbound.EmbeddingOptions")).
			Return(mockEmbedding, nil)

		// Return more results than the limit to test pagination
		chunkIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
		mockVectorResults := []outbound.VectorSimilarityResult{
			{Embedding: outbound.VectorEmbedding{ChunkID: chunkIDs[0]}, Similarity: 0.9},
			{Embedding: outbound.VectorEmbedding{ChunkID: chunkIDs[1]}, Similarity: 0.8},
			{Embedding: outbound.VectorEmbedding{ChunkID: chunkIDs[2]}, Similarity: 0.7},
			{Embedding: outbound.VectorEmbedding{ChunkID: chunkIDs[3]}, Similarity: 0.6},
		}

		// Verify that the vector search receives the correct limit and offset
		mockVectorRepo.On("VectorSimilaritySearch", ctx, queryVector, mock.MatchedBy(func(opts outbound.SimilaritySearchOptions) bool {
			return opts.MaxResults >= 3 // Should request enough results to handle offset
		})).
			Return(mockVectorResults, nil)

		// All chunks should be retrieved, pagination applied after
		mockChunks := []ChunkInfo{
			{ChunkID: chunkIDs[0], Content: "result 0"},
			{ChunkID: chunkIDs[1], Content: "result 1"},
			{ChunkID: chunkIDs[2], Content: "result 2"},
			{ChunkID: chunkIDs[3], Content: "result 3"},
		}
		mockChunkRepo.On("FindChunksByIDs", ctx, chunkIDs).
			Return(mockChunks, nil)

		result, err := searchService.Search(ctx, searchRequest)

		require.NoError(t, err, "Paginated search should succeed")

		// Should return only 2 results (limit) starting from index 1 (offset)
		assert.Len(t, result.Results, 2, "Should return exactly 2 results due to limit")

		// Should contain the second and third results (offset 1)
		assert.Contains(t, result.Results[0].Content, "result 1", "First paginated result should be result 1")
		assert.Contains(t, result.Results[1].Content, "result 2", "Second paginated result should be result 2")

		// Verify pagination metadata
		assert.Equal(t, 2, result.Pagination.Limit, "Pagination limit should match request")
		assert.Equal(t, 1, result.Pagination.Offset, "Pagination offset should match request")
		assert.Equal(t, 4, result.Pagination.Total, "Total should reflect all available results")
		assert.True(t, result.Pagination.HasMore, "Should indicate more results available")
	})

	t.Run("Search_Empty_Query_After_Trimming", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		ctx := context.Background()
		searchRequest := dto.SearchRequestDTO{
			Query: "   ", // Whitespace only
		}

		result, err := searchService.Search(ctx, searchRequest)

		assert.Error(t, err, "Should return error for empty query after trimming")
		assert.Nil(t, result, "Result should be nil on validation error")
		assert.Contains(t, err.Error(), "query cannot be empty", "Error message should indicate empty query")
	})

	t.Run("Search_Request_Validation_Applied", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		ctx := context.Background()

		validationTests := []struct {
			name          string
			searchRequest dto.SearchRequestDTO
			expectedError string
		}{
			{
				name: "Invalid_Limit_Too_High",
				searchRequest: dto.SearchRequestDTO{
					Query: "test",
					Limit: 101,
				},
				expectedError: "limit cannot exceed 100",
			},
			{
				name: "Invalid_Negative_Offset",
				searchRequest: dto.SearchRequestDTO{
					Query:  "test",
					Offset: -1,
				},
				expectedError: "offset must be non-negative",
			},
			{
				name: "Invalid_Similarity_Threshold",
				searchRequest: dto.SearchRequestDTO{
					Query:               "test",
					SimilarityThreshold: 1.5,
				},
				expectedError: "similarity_threshold must be between 0.0 and 1.0",
			},
		}

		for _, tt := range validationTests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := searchService.Search(ctx, tt.searchRequest)

				assert.Error(t, err, "Should return validation error")
				assert.Nil(t, result, "Result should be nil on validation error")
				assert.Contains(
					t,
					err.Error(),
					tt.expectedError,
					"Error message should contain expected validation error",
				)
			})
		}
	})
}

// TestNewSearchService validates constructor wiring.
func TestNewSearchService(t *testing.T) {
	t.Run("Valid_Service_Creation", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		service := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		assert.NotNil(t, service, "SearchService should be created successfully")
	})

	t.Run("Nil_Vector_Repository_Panic", func(t *testing.T) {
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		assert.Panics(t, func() {
			NewSearchService(nil, mockEmbeddingService, mockChunkRepo)
		}, "Should panic when vector repository is nil")
	})

	t.Run("Nil_Embedding_Service_Panic", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockChunkRepo := new(MockChunkRepository)

		assert.Panics(t, func() {
			NewSearchService(mockVectorRepo, nil, mockChunkRepo)
		}, "Should panic when embedding service is nil")
	})

	t.Run("Nil_Chunk_Repository_Panic", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)

		assert.Panics(t, func() {
			NewSearchService(mockVectorRepo, mockEmbeddingService, nil)
		}, "Should panic when chunk repository is nil")
	})
}

// TestSearchService_Performance documents basic performance expectations.
func TestSearchService_Performance(t *testing.T) {
	// Set up silent logger for tests to avoid logging side effects
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR", // Only log errors, suppress INFO/DEBUG
		Format: "json",
		Output: "buffer", // Output to buffer instead of stdout
	})
	require.NoError(t, err)

	// Set silent logger for test and restore default behavior after test
	slogger.SetGlobalLogger(silentLogger)
	defer slogger.SetGlobalLogger(nil)
	t.Run("Search_Execution_Time_Tracked", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		ctx := context.Background()
		searchRequest := dto.SearchRequestDTO{
			Query: "performance test",
		}

		queryVector := []float64{0.1, 0.2, 0.3}
		mockEmbedding := &outbound.EmbeddingResult{Vector: queryVector}
		mockEmbeddingService.On("GenerateEmbedding", ctx, searchRequest.Query, mock.AnythingOfType("outbound.EmbeddingOptions")).
			Return(mockEmbedding, nil)

		mockVectorRepo.On("VectorSimilaritySearch", ctx, queryVector, mock.AnythingOfType("outbound.SimilaritySearchOptions")).
			Return([]outbound.VectorSimilarityResult{}, nil)

		mockChunkRepo.On("FindChunksByIDs", ctx, []uuid.UUID{}).
			Return([]ChunkInfo{}, nil)

		startTime := time.Now()
		result, err := searchService.Search(ctx, searchRequest)
		endTime := time.Now()

		require.NoError(t, err, "Search should succeed")
		assert.NotNil(t, result.Metadata, "Metadata should be present")
		assert.Positive(t, result.Metadata.ExecutionTimeMs, "Execution time should be positive")

		// Execution time should be reasonable (not wildly different from actual time)
		actualDuration := endTime.Sub(startTime).Milliseconds()
		assert.InDelta(t, actualDuration, result.Metadata.ExecutionTimeMs, float64(actualDuration)*0.5,
			"Reported execution time should be close to actual duration")
	})

	t.Run("Search_Context_Cancellation_Handled", func(t *testing.T) {
		mockVectorRepo := new(MockVectorStorageRepository)
		mockEmbeddingService := new(MockEmbeddingService)
		mockChunkRepo := new(MockChunkRepository)

		searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)

		// Create cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		searchRequest := dto.SearchRequestDTO{
			Query: "test query",
		}

		// Embedding service should respect context cancellation
		mockEmbeddingService.On("GenerateEmbedding", ctx, searchRequest.Query, mock.AnythingOfType("outbound.EmbeddingOptions")).
			Return((*outbound.EmbeddingResult)(nil), context.Canceled)

		result, err := searchService.Search(ctx, searchRequest)

		assert.Error(t, err, "Should return error on cancelled context")
		assert.Nil(t, result, "Result should be nil on cancellation")
		assert.Equal(t, context.Canceled, err, "Should return context cancellation error")

		mockEmbeddingService.AssertExpectations(t)
	})
}
