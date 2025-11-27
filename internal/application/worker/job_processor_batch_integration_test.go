//go:build integration

// Package worker provides integration tests for the batch chunking system.
//
// # Batch Processing Integration Tests
//
// This file contains comprehensive end-to-end integration tests for the batch chunking
// system, validating the complete flow from chunk creation through batch processing,
// progress persistence, and resume functionality.
//
// # Test Coverage
//
// The tests verify:
//   - Small repository processing (single batch)
//   - Large repository processing (multiple batches)
//   - Resume functionality after interruption
//   - Production-scale processing (8269 chunks)
//   - Error handling and recovery
//
// # Prerequisites
//
// To run these tests:
//   - PostgreSQL database must be running on localhost:5432
//   - Database: codechunking, User: dev, Password: dev
//   - Run with: go test -tags=integration ./internal/application/worker -v
//
// # Test Architecture
//
// Tests use:
//   - Real PostgreSQL database for persistence layer
//   - Mock embedding service for fast, deterministic embeddings
//   - Real batch progress tracking via database
//   - Real chunk storage operations
//
// # Performance Notes
//
// The production scale test (8269 chunks) may take several seconds to complete.
// Use -short flag to skip this test: go test -short -tags=integration ./internal/application/worker -v
package worker

import (
	"codechunking/internal/adapter/outbound/repository"
	"codechunking/internal/config"
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a test database connection pool for integration tests.
func setupTestDB(t *testing.T) *pgxpool.Pool {
	dbConfig := repository.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "codechunking",
		Username: "dev",
		Password: "dev",
		Schema:   "codechunking",
	}

	pool, err := repository.NewDatabaseConnection(dbConfig)
	require.NoError(t, err, "Failed to create test database connection")

	// Clean up existing test data
	cleanupTestData(t, pool)

	return pool
}

// cleanupTestData removes all test data from the database.
func cleanupTestData(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()
	queries := []string{
		"DELETE FROM codechunking.batch_job_progress WHERE 1=1",
		"DELETE FROM codechunking.embeddings WHERE 1=1",
		"DELETE FROM codechunking.code_chunks WHERE 1=1",
		"DELETE FROM codechunking.indexing_jobs WHERE 1=1",
		"DELETE FROM codechunking.repositories WHERE 1=1",
	}

	for _, query := range queries {
		_, err := pool.Exec(ctx, query)
		if err != nil {
			t.Logf("Warning: Failed to clean up with query %s: %v", query, err)
		}
	}
}

// createTestRepository creates a test repository entity and persists it to the database.
func createTestRepository(t *testing.T, pool *pgxpool.Pool) *entity.Repository {
	uniqueID := uuid.New().String()
	testURL, err := valueobject.NewRepositoryURL("https://github.com/test/repo-" + uniqueID)
	require.NoError(t, err, "Failed to create test URL")

	description := "Test repository for batch integration testing"
	defaultBranch := "main"
	repo := entity.NewRepository(testURL, "Test Repository", &description, &defaultBranch)

	// Persist repository to database
	repoRepo := repository.NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()
	err = repoRepo.Save(ctx, repo)
	require.NoError(t, err, "Failed to save test repository")

	return repo
}

// createTestChunksForIntegration creates a specified number of test code chunks for integration tests.
func createTestChunksForIntegration(count int, repositoryID uuid.UUID) []outbound.CodeChunk {
	chunks := make([]outbound.CodeChunk, count)
	for i := 0; i < count; i++ {
		chunks[i] = outbound.CodeChunk{
			ID:           fmt.Sprintf("chunk-%d", i+1),
			RepositoryID: repositoryID,
			Content:      fmt.Sprintf("Test content for chunk %d with some code snippet", i+1),
			FilePath:     fmt.Sprintf("/test/file-%d.go", i+1),
			StartLine:    i * 10,
			EndLine:      (i + 1) * 10,
			Language:     "go",
			Type:         "function",
			Size:         256,
			Hash:         fmt.Sprintf("hash-%d", i+1),
			CreatedAt:    time.Now(),
		}
	}
	return chunks
}

// IntegrationMockEmbeddingService provides a mock implementation for integration testing.
type IntegrationMockEmbeddingService struct {
	generateBatchCallCount int
	generateCallCount      int
}

// GenerateEmbedding generates a single test embedding.
func (m *IntegrationMockEmbeddingService) GenerateEmbedding(
	ctx context.Context,
	text string,
	options outbound.EmbeddingOptions,
) (*outbound.EmbeddingResult, error) {
	m.generateCallCount++

	// Generate deterministic 768-dimensional vector
	vector := make([]float64, 768)
	contentHash := 0
	for _, char := range text {
		contentHash = contentHash*31 + int(char)
	}

	for i := range 768 {
		vector[i] = float64(((contentHash * (i + 1)) % 1000)) / 1000.0
	}

	return &outbound.EmbeddingResult{
		Vector:      vector,
		Dimensions:  768,
		TokenCount:  len(text) / 4,
		Model:       "gemini-embedding-001",
		GeneratedAt: time.Now(),
	}, nil
}

// GenerateBatchEmbeddings generates batch embeddings for multiple texts.
func (m *IntegrationMockEmbeddingService) GenerateBatchEmbeddings(
	ctx context.Context,
	texts []string,
	options outbound.EmbeddingOptions,
) ([]*outbound.EmbeddingResult, error) {
	m.generateBatchCallCount++

	results := make([]*outbound.EmbeddingResult, len(texts))
	for i, text := range texts {
		result, err := m.GenerateEmbedding(ctx, text, options)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}
	return results, nil
}

// GenerateCodeChunkEmbedding generates an embedding for a code chunk.
func (m *IntegrationMockEmbeddingService) GenerateCodeChunkEmbedding(
	ctx context.Context,
	chunk *outbound.CodeChunk,
	options outbound.EmbeddingOptions,
) (*outbound.CodeChunkEmbedding, error) {
	result, err := m.GenerateEmbedding(ctx, chunk.Content, options)
	if err != nil {
		return nil, err
	}

	return &outbound.CodeChunkEmbedding{
		EmbeddingResult: result,
		ChunkID:         chunk.ID,
		SourceFile:      chunk.FilePath,
		Language:        chunk.Language,
		ChunkType:       chunk.Type,
	}, nil
}

// ValidateApiKey validates the API key configuration (mock implementation).
func (m *IntegrationMockEmbeddingService) ValidateApiKey(ctx context.Context) error {
	return nil
}

// GetModelInfo returns information about the embedding model.
func (m *IntegrationMockEmbeddingService) GetModelInfo(ctx context.Context) (*outbound.ModelInfo, error) {
	return &outbound.ModelInfo{
		Name:       "gemini-embedding-001",
		Dimensions: 768,
		MaxTokens:  8192,
	}, nil
}

// GetSupportedModels returns a list of supported embedding models.
func (m *IntegrationMockEmbeddingService) GetSupportedModels(ctx context.Context) ([]string, error) {
	return []string{"gemini-embedding-001"}, nil
}

// EstimateTokenCount estimates the number of tokens in the given text.
func (m *IntegrationMockEmbeddingService) EstimateTokenCount(ctx context.Context, text string) (int, error) {
	return len(text) / 4, nil
}

// createJobProcessorWithBatchConfig creates a job processor with batch processing enabled.
func createJobProcessorWithBatchConfig(
	t *testing.T,
	pool *pgxpool.Pool,
	mockEmbedding outbound.EmbeddingService,
	maxBatchSize int,
) *DefaultJobProcessor {
	// Create repositories
	batchProgressRepo := repository.NewPostgreSQLBatchProgressRepository(pool)
	chunkStorageRepo := repository.NewPostgreSQLChunkRepository(pool)
	repositoryRepo := repository.NewPostgreSQLRepositoryRepository(pool)

	// Create mock indexing job repository
	indexingJobRepo := &MockIndexingJobRepository{}

	// Create mock git client
	gitClient := &MockEnhancedGitClient{}

	// Create mock code parser
	codeParser := &MockCodeParser{}

	// Configure batch processing
	batchConfig := config.BatchProcessingConfig{
		Enabled:              true,
		ThresholdChunks:      10,
		FallbackToSequential: true,
		UseTestEmbeddings:    false, // Use real embeddings (via mock service)
		MaxBatchSize:         maxBatchSize,
		InitialBackoff:       0, // Instant retry for tests
		MaxBackoff:           0, // Instant retry for tests
		MaxRetries:           3, // Allow retries
		EnableBatchChunking:  true,
	}

	// Create job processor config
	processorConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test-workspace",
		MaxConcurrentJobs: 1,
		JobTimeout:        5 * time.Minute,
	}

	// Create job processor with batch support
	processor := NewDefaultJobProcessor(
		processorConfig,
		indexingJobRepo,
		repositoryRepo,
		gitClient,
		codeParser,
		mockEmbedding,
		chunkStorageRepo,
		&JobProcessorBatchOptions{
			BatchConfig:       &batchConfig,
			BatchProgressRepo: batchProgressRepo,
		},
	).(*DefaultJobProcessor)

	return processor
}

// TestBatchProcessing_EndToEnd_SmallRepository tests batch processing with a small repository (single batch).
func TestBatchProcessing_EndToEnd_SmallRepository(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()

	// Create test repository
	repo := createTestRepository(t, pool)
	repoID := repo.ID()
	jobID := uuid.New()

	// Create test chunks (150 chunks - single batch)
	chunks := createTestChunksForIntegration(150, repoID)

	// Create mock embedding service
	mockEmbedding := &IntegrationMockEmbeddingService{}

	// Create job processor with batch config (500 chunks per batch)
	processor := createJobProcessorWithBatchConfig(t, pool, mockEmbedding, 500)

	// Execute: Process batches
	err := processor.processProductionBatchResults(ctx, jobID, repoID, chunks, nil)

	// Assert: No errors
	assert.NoError(t, err, "Batch processing should complete without errors")

	// Verify batch progress in database
	batchProgressRepo := repository.NewPostgreSQLBatchProgressRepository(pool)
	batches, err := batchProgressRepo.GetByJobID(ctx, jobID)
	assert.NoError(t, err)
	assert.Len(t, batches, 1, "Should create 1 batch for 150 chunks")

	// Verify batch completed
	assert.Equal(t, "completed", batches[0].Status(), "Batch should be completed")
	assert.Equal(t, 150, batches[0].ChunksProcessed(), "All 150 chunks should be processed")

	// Verify all embeddings were stored
	chunkRepo := repository.NewPostgreSQLChunkRepository(pool)
	savedChunks, err := chunkRepo.GetChunksForRepository(ctx, repoID)
	assert.NoError(t, err)
	assert.Len(t, savedChunks, 150, "All 150 chunks should be saved")

	// Verify mock was called once for batch processing
	assert.Equal(t, 1, mockEmbedding.generateBatchCallCount, "Should call GenerateBatchEmbeddings once")
}

// TestBatchProcessing_EndToEnd_LargeRepository tests batch processing with multiple batches.
func TestBatchProcessing_EndToEnd_LargeRepository(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()

	// Create test repository
	repo := createTestRepository(t, pool)
	repoID := repo.ID()
	jobID := uuid.New()

	// Create test chunks (1500 chunks - 3 batches of 500)
	chunks := createTestChunksForIntegration(1500, repoID)

	// Create mock embedding service
	mockEmbedding := &IntegrationMockEmbeddingService{}

	// Create job processor with batch config (500 chunks per batch)
	processor := createJobProcessorWithBatchConfig(t, pool, mockEmbedding, 500)

	// Execute: Process batches
	err := processor.processProductionBatchResults(ctx, jobID, repoID, chunks, nil)

	// Assert: No errors
	assert.NoError(t, err, "Batch processing should complete without errors")

	// Verify batch progress in database
	batchProgressRepo := repository.NewPostgreSQLBatchProgressRepository(pool)
	batches, err := batchProgressRepo.GetByJobID(ctx, jobID)
	assert.NoError(t, err)
	assert.Len(t, batches, 3, "Should create 3 batches for 1500 chunks")

	// Verify all batches completed
	batchNumbers := make([]int, 0, 3)
	for _, batch := range batches {
		assert.Equal(t, "completed", batch.Status(), "All batches should be completed")
		assert.Equal(t, 500, batch.ChunksProcessed(), "Each batch should process 500 chunks")
		batchNumbers = append(batchNumbers, batch.BatchNumber())
	}

	// Verify batch numbers are sequential (1, 2, 3)
	assert.Equal(t, []int{1, 2, 3}, batchNumbers, "Batch numbers should be sequential")

	// Verify all embeddings were stored
	chunkRepo := repository.NewPostgreSQLChunkRepository(pool)
	savedChunks, err := chunkRepo.GetChunksForRepository(ctx, repoID)
	assert.NoError(t, err)
	assert.Len(t, savedChunks, 1500, "All 1500 chunks should be saved")

	// Verify mock was called 3 times for batch processing
	assert.Equal(t, 3, mockEmbedding.generateBatchCallCount, "Should call GenerateBatchEmbeddings 3 times")
}

// TestBatchProcessing_EndToEnd_WithResume tests batch processing resume functionality.
func TestBatchProcessing_EndToEnd_WithResume(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()

	// Create test repository
	repo := createTestRepository(t, pool)
	repoID := repo.ID()
	jobID := uuid.New()

	// Create test chunks (1500 chunks - 3 batches of 500)
	chunks := createTestChunksForIntegration(1500, repoID)

	// Create mock embedding service
	mockEmbedding := &IntegrationMockEmbeddingService{}

	// Create job processor with batch config (500 chunks per batch)
	processor := createJobProcessorWithBatchConfig(t, pool, mockEmbedding, 500)

	// Step 1: Process first batch only
	firstBatch := chunks[0:500]
	err := processor.processProductionBatchResults(ctx, jobID, repoID, firstBatch, nil)
	assert.NoError(t, err, "First batch should complete without errors")

	// Verify first batch was saved
	batchProgressRepo := repository.NewPostgreSQLBatchProgressRepository(pool)
	batches, err := batchProgressRepo.GetByJobID(ctx, jobID)
	assert.NoError(t, err)
	assert.Len(t, batches, 1, "Should have 1 batch after first processing")
	assert.Equal(t, 1, batches[0].BatchNumber(), "First batch should be batch 1")
	assert.Equal(t, "completed", batches[0].Status(), "First batch should be completed")

	// Reset mock call count to track resume calls
	initialCallCount := mockEmbedding.generateBatchCallCount

	// Step 2: Resume processing with all chunks (should skip batch 1)
	err = processor.processProductionBatchResults(ctx, jobID, repoID, chunks, nil)
	assert.NoError(t, err, "Resume processing should complete without errors")

	// Verify all batches were processed
	batches, err = batchProgressRepo.GetByJobID(ctx, jobID)
	assert.NoError(t, err)
	assert.Len(t, batches, 3, "Should have 3 batches after resume")

	// Verify batch numbers and completion status
	for i, batch := range batches {
		assert.Equal(t, i+1, batch.BatchNumber(), "Batch numbers should be sequential")
		assert.Equal(t, "completed", batch.Status(), "All batches should be completed")
	}

	// Verify only batches 2 and 3 were processed on resume
	resumeCallCount := mockEmbedding.generateBatchCallCount - initialCallCount
	assert.Equal(t, 2, resumeCallCount, "Should process only batches 2 and 3 on resume")

	// Verify all embeddings were stored
	chunkRepo := repository.NewPostgreSQLChunkRepository(pool)
	savedChunks, err := chunkRepo.GetChunksForRepository(ctx, repoID)
	assert.NoError(t, err)
	assert.Len(t, savedChunks, 1500, "All 1500 chunks should be saved")
}

// TestBatchProcessing_EndToEnd_ProductionScale tests production-scale batch processing.
func TestBatchProcessing_EndToEnd_ProductionScale(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping production scale test in short mode")
	}

	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()

	// Create test repository
	repo := createTestRepository(t, pool)
	repoID := repo.ID()
	jobID := uuid.New()

	// Create test chunks (8269 chunks - production scenario)
	// This should create 17 batches (16 of 500, 1 of 269)
	chunks := createTestChunksForIntegration(8269, repoID)

	// Create mock embedding service
	mockEmbedding := &IntegrationMockEmbeddingService{}

	// Create job processor with batch config (500 chunks per batch)
	processor := createJobProcessorWithBatchConfig(t, pool, mockEmbedding, 500)

	// Execute: Process batches
	startTime := time.Now()
	err := processor.processProductionBatchResults(ctx, jobID, repoID, chunks, nil)
	duration := time.Since(startTime)

	// Assert: No errors
	assert.NoError(t, err, "Batch processing should complete without errors")

	t.Logf("Production scale processing completed in %v", duration)

	// Verify batch progress in database
	batchProgressRepo := repository.NewPostgreSQLBatchProgressRepository(pool)
	batches, err := batchProgressRepo.GetByJobID(ctx, jobID)
	assert.NoError(t, err)
	assert.Len(t, batches, 17, "Should create 17 batches for 8269 chunks")

	// Verify all batches completed and count total chunks processed
	totalChunksProcessed := 0
	for i, batch := range batches {
		assert.Equal(t, "completed", batch.Status(), "All batches should be completed")
		assert.Equal(t, i+1, batch.BatchNumber(), "Batch numbers should be sequential")

		// Verify batch sizes (16 batches of 500, 1 batch of 269)
		expectedSize := 500
		if i == 16 { // Last batch
			expectedSize = 269
		}
		assert.Equal(t, expectedSize, batch.ChunksProcessed(), "Batch %d should process %d chunks", i+1, expectedSize)

		totalChunksProcessed += batch.ChunksProcessed()
	}

	// Verify total chunks processed
	assert.Equal(t, 8269, totalChunksProcessed, "Total chunks processed should match input")

	// Verify all embeddings were stored
	chunkRepo := repository.NewPostgreSQLChunkRepository(pool)
	savedChunks, err := chunkRepo.GetChunksForRepository(ctx, repoID)
	assert.NoError(t, err)
	assert.Len(t, savedChunks, 8269, "All 8269 chunks should be saved")

	// Verify mock was called 17 times for batch processing
	assert.Equal(t, 17, mockEmbedding.generateBatchCallCount, "Should call GenerateBatchEmbeddings 17 times")
}

// TestBatchProcessing_EndToEnd_ErrorHandling tests error handling in batch processing.
func TestBatchProcessing_EndToEnd_ErrorHandling(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()

	// Create test repository
	repo := createTestRepository(t, pool)
	repoID := repo.ID()
	jobID := uuid.New()

	// Create test chunks
	chunks := createTestChunksForIntegration(100, repoID)

	// Create mock embedding service that fails
	mockEmbedding := &IntegrationMockEmbeddingServiceWithErrors{
		shouldFail:  true,
		failAtBatch: 1, // Fail on first batch
	}

	// Create job processor
	processor := createJobProcessorWithBatchConfig(t, pool, mockEmbedding, 500)

	// Execute: Should fail
	err := processor.processProductionBatchResults(ctx, jobID, repoID, chunks, nil)

	// Assert: Error occurred
	assert.Error(t, err, "Batch processing should fail when embedding service fails")

	// Verify failed batch progress was saved
	batchProgressRepo := repository.NewPostgreSQLBatchProgressRepository(pool)
	batches, err := batchProgressRepo.GetByJobID(ctx, jobID)
	assert.NoError(t, err)
	assert.NotEmpty(t, batches, "Failed batch should be recorded")

	// Verify batch is marked as failed
	if len(batches) > 0 {
		assert.Equal(t, "failed", batches[0].Status(), "Batch should be marked as failed")
	}
}

// IntegrationMockEmbeddingServiceWithErrors is a mock that can simulate failures.
type IntegrationMockEmbeddingServiceWithErrors struct {
	IntegrationMockEmbeddingService
	shouldFail  bool
	failAtBatch int
	callCount   int
}

// GenerateBatchEmbeddings simulates failures at specific batches.
func (m *IntegrationMockEmbeddingServiceWithErrors) GenerateBatchEmbeddings(
	ctx context.Context,
	texts []string,
	options outbound.EmbeddingOptions,
) ([]*outbound.EmbeddingResult, error) {
	m.callCount++

	if m.shouldFail && m.callCount == m.failAtBatch {
		return nil, fmt.Errorf("simulated embedding service failure at batch %d", m.callCount)
	}

	return m.IntegrationMockEmbeddingService.GenerateBatchEmbeddings(ctx, texts, options)
}
