package worker

import (
	"codechunking/internal/config"
	"codechunking/internal/domain/entity"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestJobProcessor_SubmitBatchJobAsync_QueuesForSubmission verifies that submitBatchJobAsync
// queues batches for later submission instead of immediately calling Gemini API.
//
// RED PHASE EXPECTATION:
// - Batch progress should have status = "pending_submission"
// - Batch progress should have request data stored (not nil)
// - BatchEmbeddingService.CreateBatchEmbeddingJobWithRequests() should NOT be called
// - Batch progress should be saved to repository
//
// CURRENT BEHAVIOR (WHY THIS FAILS):
// - submitBatchJobAsync() immediately calls CreateBatchEmbeddingJobWithRequests()
// - Batch progress status is set to "processing" after submission
// - No request data is stored - batch is submitted immediately.
func TestJobProcessor_SubmitBatchJobAsync_QueuesForSubmission(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()
	batchNumber := 1
	totalBatches := 3

	// Create test chunks
	chunks := []outbound.CodeChunk{
		{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			Content:      "function test1() { return 42; }",
			FilePath:     "/test/file1.go",
			Language:     "go",
		},
		{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			Content:      "function test2() { return 'hello'; }",
			FilePath:     "/test/file2.go",
			Language:     "go",
		},
	}

	// Convert to saved chunks (with actual IDs from DB)
	savedChunks := []outbound.CodeChunk{
		{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			Content:      chunks[0].Content,
			FilePath:     chunks[0].FilePath,
			Language:     chunks[0].Language,
		},
		{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			Content:      chunks[1].Content,
			FilePath:     chunks[1].FilePath,
			Language:     chunks[1].Language,
		},
	}

	// Create mocks
	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchEmbeddingService := new(MockBatchEmbeddingService)

	// Mock chunk storage - FindOrCreateChunks returns saved chunks
	mockChunkStorageRepo.On("FindOrCreateChunks", ctx, mock.MatchedBy(func(c []outbound.CodeChunk) bool {
		return len(c) == 2
	})).Return(savedChunks, nil)

	// KEY EXPECTATION: Batch progress should be saved with pending_submission status
	var capturedProgress *entity.BatchJobProgress
	mockBatchProgressRepo.On("Save", ctx, mock.MatchedBy(func(p *entity.BatchJobProgress) bool {
		capturedProgress = p
		return p.Status() == entity.StatusPendingSubmission &&
			p.BatchRequestData() != nil &&
			p.SubmissionAttempts() == 0 &&
			p.IndexingJobID() == indexingJobID &&
			p.BatchNumber() == batchNumber
	})).Return(nil)

	// Create processor with batch support
	batchConfig := config.BatchProcessingConfig{
		Enabled:           true,
		ThresholdChunks:   10,
		UseTestEmbeddings: false,
	}

	processor := &DefaultJobProcessor{
		chunkStorageRepo:      mockChunkStorageRepo,
		batchProgressRepo:     mockBatchProgressRepo,
		batchEmbeddingService: mockBatchEmbeddingService,
		batchConfig:           batchConfig,
	}

	// Embedding options
	options := outbound.EmbeddingOptions{
		Model:    "gemini-embedding-001",
		TaskType: outbound.TaskTypeRetrievalDocument,
	}

	// Act
	err := processor.submitBatchJobAsync(
		ctx,
		indexingJobID,
		repositoryID,
		batchNumber,
		totalBatches,
		chunks,
		options,
	)

	// Assert
	require.NoError(t, err)

	// CRITICAL ASSERTION: Gemini API should NOT be called
	mockBatchEmbeddingService.AssertNotCalled(t, "CreateBatchEmbeddingJobWithRequests")

	// Verify batch progress was saved
	mockBatchProgressRepo.AssertExpectations(t)

	// Verify captured progress has correct status
	require.NotNil(t, capturedProgress, "Batch progress should be captured")
	assert.Equal(t, entity.StatusPendingSubmission, capturedProgress.Status(), "Status should be pending_submission")
	assert.NotNil(t, capturedProgress.BatchRequestData(), "Request data should be stored")
	assert.Equal(t, 0, capturedProgress.SubmissionAttempts(), "Submission attempts should be 0")
}

// TestJobProcessor_SubmitBatchJobAsync_SerializesRequestData verifies that request data
// is properly serialized and can be deserialized back to BatchEmbeddingRequest array.
//
// RED PHASE EXPECTATION:
// - Request data should be valid JSON
// - Request data should deserialize to []*outbound.BatchEmbeddingRequest
// - Each request should have correct chunk ID encoded as request_id
// - Each request should have correct text content
//
// CURRENT BEHAVIOR (WHY THIS FAILS):
// - No request data is stored (current implementation submits immediately)
// - BatchRequestData() returns nil.
func TestJobProcessor_SubmitBatchJobAsync_SerializesRequestData(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()

	// Create test chunks with known IDs
	chunkID1 := uuid.New()
	chunkID2 := uuid.New()

	chunks := []outbound.CodeChunk{
		{
			ID:           chunkID1.String(),
			RepositoryID: repositoryID,
			Content:      "test content 1",
			FilePath:     "/test1.go",
			Language:     "go",
		},
		{
			ID:           chunkID2.String(),
			RepositoryID: repositoryID,
			Content:      "test content 2",
			FilePath:     "/test2.go",
			Language:     "go",
		},
	}

	savedChunks := chunks // Assume FindOrCreateChunks returns same chunks

	// Create mocks
	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchEmbeddingService := new(MockBatchEmbeddingService)

	mockChunkStorageRepo.On("FindOrCreateChunks", ctx, mock.Anything).Return(savedChunks, nil)

	var capturedProgress *entity.BatchJobProgress
	mockBatchProgressRepo.On("Save", ctx, mock.Anything).Run(func(args mock.Arguments) {
		capturedProgress = args.Get(1).(*entity.BatchJobProgress)
	}).Return(nil)

	processor := &DefaultJobProcessor{
		chunkStorageRepo:      mockChunkStorageRepo,
		batchProgressRepo:     mockBatchProgressRepo,
		batchEmbeddingService: mockBatchEmbeddingService,
		batchConfig: config.BatchProcessingConfig{
			Enabled:           true,
			UseTestEmbeddings: false,
		},
	}

	options := outbound.EmbeddingOptions{
		Model:    "gemini-embedding-001",
		TaskType: outbound.TaskTypeRetrievalDocument,
	}

	// Act
	err := processor.submitBatchJobAsync(ctx, indexingJobID, repositoryID, 1, 1, chunks, options)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, capturedProgress, "Progress should be captured")

	// Verify request data is not nil
	requestData := capturedProgress.BatchRequestData()
	require.NotNil(t, requestData, "Request data should be stored")

	// Deserialize request data
	var requests []*outbound.BatchEmbeddingRequest
	err = json.Unmarshal(requestData, &requests)
	require.NoError(t, err, "Request data should be valid JSON")

	// Verify request structure
	require.Len(t, requests, 2, "Should have 2 requests")

	// Verify first request
	expectedRequestID1 := EncodeChunkIDToRequestID(chunkID1)
	assert.Equal(t, expectedRequestID1, requests[0].RequestID, "Request ID should be encoded chunk UUID")
	assert.Equal(t, "test content 1", requests[0].Text, "Request text should match chunk content")

	// Verify second request
	expectedRequestID2 := EncodeChunkIDToRequestID(chunkID2)
	assert.Equal(t, expectedRequestID2, requests[1].RequestID, "Request ID should be encoded chunk UUID")
	assert.Equal(t, "test content 2", requests[1].Text, "Request text should match chunk content")
}

// TestJobProcessor_SubmitBatchJobAsync_RequestDataFormat verifies the exact format
// of serialized request data matches Gemini API expectations.
//
// RED PHASE EXPECTATION:
// - RequestID format should be "chunk_<uuid_without_hyphens>"
// - Text field should contain chunk content
// - Metadata field should be present (may be empty)
//
// CURRENT BEHAVIOR (WHY THIS FAILS):
// - No request data is stored.
func TestJobProcessor_SubmitBatchJobAsync_RequestDataFormat(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()

	// Use a known UUID for predictable request ID
	testUUID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	chunks := []outbound.CodeChunk{
		{
			ID:           testUUID.String(),
			RepositoryID: repositoryID,
			Content:      "function example() { return true; }",
			FilePath:     "/example.js",
			Language:     "javascript",
		},
	}

	// Create mocks
	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchEmbeddingService := new(MockBatchEmbeddingService)

	mockChunkStorageRepo.On("FindOrCreateChunks", ctx, mock.Anything).Return(chunks, nil)

	var capturedProgress *entity.BatchJobProgress
	mockBatchProgressRepo.On("Save", ctx, mock.Anything).Run(func(args mock.Arguments) {
		capturedProgress = args.Get(1).(*entity.BatchJobProgress)
	}).Return(nil)

	processor := &DefaultJobProcessor{
		chunkStorageRepo:      mockChunkStorageRepo,
		batchProgressRepo:     mockBatchProgressRepo,
		batchEmbeddingService: mockBatchEmbeddingService,
		batchConfig: config.BatchProcessingConfig{
			Enabled:           true,
			UseTestEmbeddings: false,
		},
	}

	options := outbound.EmbeddingOptions{
		Model:    "gemini-embedding-001",
		TaskType: outbound.TaskTypeRetrievalDocument,
	}

	// Act
	err := processor.submitBatchJobAsync(ctx, indexingJobID, repositoryID, 1, 1, chunks, options)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, capturedProgress)

	requestData := capturedProgress.BatchRequestData()
	require.NotNil(t, requestData)

	var requests []*outbound.BatchEmbeddingRequest
	err = json.Unmarshal(requestData, &requests)
	require.NoError(t, err)
	require.Len(t, requests, 1)

	// Verify RequestID format (chunk_<uuid_without_hyphens>)
	expectedRequestID := "chunk_550e8400e29b41d4a716446655440000"
	assert.Equal(t, expectedRequestID, requests[0].RequestID, "RequestID should be chunk_<uuid_without_hyphens>")

	// Verify text content
	assert.Equal(t, "function example() { return true; }", requests[0].Text, "Text should match chunk content")
}

// TestJobProcessor_SubmitBatchJobAsync_ChunkSaveError verifies error handling
// when chunk repository fails to save chunks.
//
// RED PHASE EXPECTATION:
// - Error should be propagated
// - No batch progress should be created
// - No batch should be submitted
//
// CURRENT BEHAVIOR:
// - Error is handled, but we need to ensure no partial state is created.
func TestJobProcessor_SubmitBatchJobAsync_ChunkSaveError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()

	chunks := []outbound.CodeChunk{
		{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			Content:      "test",
			FilePath:     "/test.go",
			Language:     "go",
		},
	}

	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchEmbeddingService := new(MockBatchEmbeddingService)

	// Mock chunk save to return error
	expectedError := assert.AnError
	mockChunkStorageRepo.On("FindOrCreateChunks", ctx, mock.Anything).Return(nil, expectedError)

	processor := &DefaultJobProcessor{
		chunkStorageRepo:      mockChunkStorageRepo,
		batchProgressRepo:     mockBatchProgressRepo,
		batchEmbeddingService: mockBatchEmbeddingService,
		batchConfig: config.BatchProcessingConfig{
			Enabled:           true,
			UseTestEmbeddings: false,
		},
	}

	options := outbound.EmbeddingOptions{
		Model:    "gemini-embedding-001",
		TaskType: outbound.TaskTypeRetrievalDocument,
	}

	// Act
	err := processor.submitBatchJobAsync(ctx, indexingJobID, repositoryID, 1, 1, chunks, options)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find/create chunks")

	// Verify no batch progress was saved
	mockBatchProgressRepo.AssertNotCalled(t, "Save")

	// Verify no Gemini API call was made
	mockBatchEmbeddingService.AssertNotCalled(t, "CreateBatchEmbeddingJobWithRequests")
}

// TestJobProcessor_SubmitBatchJobAsync_BatchProgressSaveError verifies error handling
// when batch progress repository fails to save.
//
// RED PHASE EXPECTATION:
// - Error should be propagated
// - Chunks should already be saved (can't roll back)
// - Gemini API should NOT be called
//
// CURRENT BEHAVIOR (WHY THIS FAILS):
// - Currently, if batch progress save fails, chunks are saved but no progress record exists
// - With new implementation, this should happen BEFORE Gemini submission.
func TestJobProcessor_SubmitBatchJobAsync_BatchProgressSaveError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()

	chunks := []outbound.CodeChunk{
		{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			Content:      "test",
			FilePath:     "/test.go",
			Language:     "go",
		},
	}

	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchEmbeddingService := new(MockBatchEmbeddingService)

	// Mock successful chunk save
	mockChunkStorageRepo.On("FindOrCreateChunks", ctx, mock.Anything).Return(chunks, nil)

	// Mock batch progress save to return error
	expectedError := assert.AnError
	mockBatchProgressRepo.On("Save", ctx, mock.Anything).Return(expectedError)

	processor := &DefaultJobProcessor{
		chunkStorageRepo:      mockChunkStorageRepo,
		batchProgressRepo:     mockBatchProgressRepo,
		batchEmbeddingService: mockBatchEmbeddingService,
		batchConfig: config.BatchProcessingConfig{
			Enabled:           true,
			UseTestEmbeddings: false,
		},
	}

	options := outbound.EmbeddingOptions{
		Model:    "gemini-embedding-001",
		TaskType: outbound.TaskTypeRetrievalDocument,
	}

	// Act
	err := processor.submitBatchJobAsync(ctx, indexingJobID, repositoryID, 1, 1, chunks, options)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save batch progress")

	// Verify chunks were saved
	mockChunkStorageRepo.AssertExpectations(t)

	// Verify Gemini API was NOT called (new behavior - queue first, submit later)
	mockBatchEmbeddingService.AssertNotCalled(t, "CreateBatchEmbeddingJobWithRequests")
}

// TestJobProcessor_ProcessJob_WithBatching_QueuesAllBatches verifies that when processing
// a repository with enough chunks to trigger batching, all batches are queued with
// pending_submission status and no Gemini API calls are made.
//
// RED PHASE EXPECTATION:
// - Multiple batches should be created
// - All batches should have status = pending_submission
// - All batches should have request data stored
// - No Gemini API calls should be made during job processing
//
// CURRENT BEHAVIOR (WHY THIS FAILS):
// - Batches are submitted immediately to Gemini API
// - Status is set to "processing" after submission
// - CreateBatchEmbeddingJobWithRequests is called for each batch.
func TestJobProcessor_ProcessJob_WithBatching_QueuesAllBatches(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()

	// Create enough chunks to trigger multiple batches
	// Threshold = 10, batch size = 100, so 250 chunks = 3 batches
	numChunks := 250
	chunks := make([]outbound.CodeChunk, numChunks)
	for i := range numChunks {
		chunks[i] = outbound.CodeChunk{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			Content:      "test content " + string(rune(i)),
			FilePath:     "/test.go",
			Language:     "go",
		}
	}

	// Create mocks
	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchEmbeddingService := new(MockBatchEmbeddingService)
	mockCodeParser := new(MockCodeParser)

	// Mock code parser to return our chunks
	mockCodeParser.On("ParseDirectory", ctx, mock.Anything).Return(chunks, nil)

	// Mock chunk storage - return saved chunks
	mockChunkStorageRepo.On("FindOrCreateChunks", ctx, mock.Anything).
		Return(func(ctx context.Context, chunks []outbound.CodeChunk) []outbound.CodeChunk {
			return chunks
		}, nil)

	// Track saved batch progress records
	var savedBatches []*entity.BatchJobProgress
	mockBatchProgressRepo.On("Save", ctx, mock.MatchedBy(func(p *entity.BatchJobProgress) bool {
		return p.Status() == entity.StatusPendingSubmission
	})).Run(func(args mock.Arguments) {
		p := args.Get(1).(*entity.BatchJobProgress)
		savedBatches = append(savedBatches, p)
	}).Return(nil)

	processor := &DefaultJobProcessor{
		chunkStorageRepo:      mockChunkStorageRepo,
		batchProgressRepo:     mockBatchProgressRepo,
		batchEmbeddingService: mockBatchEmbeddingService,
		codeParser:            mockCodeParser,
		batchConfig: config.BatchProcessingConfig{
			Enabled:           true,
			ThresholdChunks:   10,
			MaxBatchSize:      100,
			UseTestEmbeddings: false,
		},
	}

	// Act
	err := processor.generateEmbeddings(ctx, indexingJobID, repositoryID, chunks)

	// Assert
	require.NoError(t, err)

	// Verify that batches were created (250 chunks / 100 batch size = 3 batches)
	expectedBatchCount := 3
	assert.Len(t, savedBatches, expectedBatchCount, "Should create 3 batches")

	// Verify all batches have pending_submission status
	for i, batch := range savedBatches {
		assert.Equal(
			t,
			entity.StatusPendingSubmission,
			batch.Status(),
			"Batch %d should have pending_submission status",
			i+1,
		)
		assert.NotNil(t, batch.BatchRequestData(), "Batch %d should have request data", i+1)
		assert.Equal(t, 0, batch.SubmissionAttempts(), "Batch %d should have 0 submission attempts", i+1)
	}

	// CRITICAL ASSERTION: Gemini API should NOT be called during job processing
	mockBatchEmbeddingService.AssertNotCalled(t, "CreateBatchEmbeddingJobWithRequests")
}

// TestJobProcessor_ProcessJob_FallbackToSequential_StillWorks verifies that
// repositories below the batch threshold continue to use sequential processing.
//
// RED PHASE EXPECTATION:
// - When chunk count < threshold, use sequential processing
// - No pending_submission batches should be created
// - GenerateEmbedding (sequential) should be called
//
// CURRENT BEHAVIOR:
// - This should still work, but we need to ensure new queueing logic doesn't break it.
func TestJobProcessor_ProcessJob_FallbackToSequential_StillWorks(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()

	// Create fewer chunks than threshold (threshold = 10)
	chunks := []outbound.CodeChunk{
		{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			Content:      "test 1",
			FilePath:     "/test1.go",
			Language:     "go",
		},
		{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			Content:      "test 2",
			FilePath:     "/test2.go",
			Language:     "go",
		},
	}

	// Create mocks
	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchEmbeddingService := new(MockBatchEmbeddingService)
	mockEmbeddingService := new(MockEmbeddingService)

	// Mock sequential embedding generation
	mockEmbeddingService.On("GenerateEmbedding", mock.Anything, mock.Anything, mock.Anything).
		Return(&outbound.EmbeddingResult{
			Vector:     make([]float64, 768),
			Dimensions: 768,
		}, nil)

	mockChunkStorageRepo.On("SaveChunkWithEmbedding", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	processor := &DefaultJobProcessor{
		chunkStorageRepo:      mockChunkStorageRepo,
		batchProgressRepo:     mockBatchProgressRepo,
		batchEmbeddingService: mockBatchEmbeddingService,
		embeddingService:      mockEmbeddingService,
		batchConfig: config.BatchProcessingConfig{
			Enabled:           true,
			ThresholdChunks:   10,
			UseTestEmbeddings: false,
		},
	}

	// Act
	err := processor.generateEmbeddings(ctx, indexingJobID, repositoryID, chunks)

	// Assert
	require.NoError(t, err)

	// Verify sequential processing was used
	mockEmbeddingService.AssertCalled(t, "GenerateEmbedding", ctx, mock.Anything, mock.Anything)

	// Verify no batch progress was created
	mockBatchProgressRepo.AssertNotCalled(t, "Save")

	// Verify no batch embedding service was called
	mockBatchEmbeddingService.AssertNotCalled(t, "CreateBatchEmbeddingJobWithRequests")
}

// TestJobProcessor_SubmitBatchJobAsync_NoBatchEmbeddingService verifies error handling
// when batch embedding service is not configured.
//
// RED PHASE EXPECTATION:
// - Should return error when batchEmbeddingService is nil
// - No chunks should be saved
// - No batch progress should be created
//
// CURRENT BEHAVIOR:
// - Error is returned (this should continue to work).
func TestJobProcessor_SubmitBatchJobAsync_NoBatchEmbeddingService(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()

	chunks := []outbound.CodeChunk{
		{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			Content:      "test",
			FilePath:     "/test.go",
			Language:     "go",
		},
	}

	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockBatchProgressRepo := new(MockBatchProgressRepository)

	processor := &DefaultJobProcessor{
		chunkStorageRepo:      mockChunkStorageRepo,
		batchProgressRepo:     mockBatchProgressRepo,
		batchEmbeddingService: nil, // Not configured
		batchConfig: config.BatchProcessingConfig{
			Enabled:           true,
			UseTestEmbeddings: false,
		},
	}

	options := outbound.EmbeddingOptions{
		Model:    "gemini-embedding-001",
		TaskType: outbound.TaskTypeRetrievalDocument,
	}

	// Act
	err := processor.submitBatchJobAsync(ctx, indexingJobID, repositoryID, 1, 1, chunks, options)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "batch embedding service not configured")

	// Verify no repository calls were made
	mockChunkStorageRepo.AssertNotCalled(t, "FindOrCreateChunks")
	mockBatchProgressRepo.AssertNotCalled(t, "Save")
}

// TestJobProcessor_SubmitBatchJobAsync_MultipleChunks_CorrectRequestCount verifies
// that the number of requests matches the number of chunks.
//
// RED PHASE EXPECTATION:
// - Request data should contain exactly N requests for N chunks
// - Each request should have unique RequestID
// - Each request should correspond to a chunk
//
// CURRENT BEHAVIOR (WHY THIS FAILS):
// - No request data is stored currently.
func TestJobProcessor_SubmitBatchJobAsync_MultipleChunks_CorrectRequestCount(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()

	// Create 5 chunks
	numChunks := 5
	chunks := make([]outbound.CodeChunk, numChunks)
	for i := range numChunks {
		chunks[i] = outbound.CodeChunk{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			Content:      "content " + string(rune(i)),
			FilePath:     "/test.go",
			Language:     "go",
		}
	}

	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchEmbeddingService := new(MockBatchEmbeddingService)

	mockChunkStorageRepo.On("FindOrCreateChunks", ctx, mock.Anything).Return(chunks, nil)

	var capturedProgress *entity.BatchJobProgress
	mockBatchProgressRepo.On("Save", ctx, mock.Anything).Run(func(args mock.Arguments) {
		capturedProgress = args.Get(1).(*entity.BatchJobProgress)
	}).Return(nil)

	processor := &DefaultJobProcessor{
		chunkStorageRepo:      mockChunkStorageRepo,
		batchProgressRepo:     mockBatchProgressRepo,
		batchEmbeddingService: mockBatchEmbeddingService,
		batchConfig: config.BatchProcessingConfig{
			Enabled:           true,
			UseTestEmbeddings: false,
		},
	}

	options := outbound.EmbeddingOptions{
		Model:    "gemini-embedding-001",
		TaskType: outbound.TaskTypeRetrievalDocument,
	}

	// Act
	err := processor.submitBatchJobAsync(ctx, indexingJobID, repositoryID, 1, 1, chunks, options)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, capturedProgress)

	requestData := capturedProgress.BatchRequestData()
	require.NotNil(t, requestData)

	var requests []*outbound.BatchEmbeddingRequest
	err = json.Unmarshal(requestData, &requests)
	require.NoError(t, err)

	// Verify correct number of requests
	assert.Len(t, requests, numChunks, "Should have request for each chunk")

	// Verify all request IDs are unique
	requestIDs := make(map[string]bool)
	for _, req := range requests {
		assert.False(t, requestIDs[req.RequestID], "Request IDs should be unique")
		requestIDs[req.RequestID] = true
	}
}

// TestJobProcessor_SubmitBatchJobAsync_DeduplicatesChunks verifies that
// submitBatchJobAsync deduplicates chunks by (repository_id, file_path, content_hash)
// before calling FindOrCreateChunks.
//
// RED PHASE EXPECTATION:
// - Chunks with same (repository_id, file_path, hash) should be deduplicated
// - FindOrCreateChunks should receive only unique chunks
// - Duplicate chunks should be removed before persistence
//
// CURRENT BEHAVIOR (WHY THIS FAILS):
// - submitBatchJobAsync calls FindOrCreateChunks directly without deduplication
// - This can cause duplicate chunk errors in the database.
func TestJobProcessor_SubmitBatchJobAsync_DeduplicatesChunks(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repositoryID := uuid.New()
	indexingJobID := uuid.New()

	// Create chunks with duplicates - same repo_id, file_path, and hash
	sharedHash := "abc123hash"
	sharedFilePath := "/test/duplicate.go"

	chunks := []outbound.CodeChunk{
		{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			Content:      "function test() { return 42; }",
			FilePath:     sharedFilePath,
			Language:     "go",
			Hash:         sharedHash, // Same hash
		},
		{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			Content:      "function other() { return 'hello'; }",
			FilePath:     "/test/unique.go",
			Language:     "go",
			Hash:         "xyz789different",
		},
		{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			Content:      "function test() { return 42; }", // Duplicate content
			FilePath:     sharedFilePath,                   // Same file path
			Language:     "go",
			Hash:         sharedHash, // Same hash - THIS IS A DUPLICATE
		},
		{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			Content:      "function another() { return true; }",
			FilePath:     "/test/another.go",
			Language:     "go",
			Hash:         "def456hash",
		},
	}

	// Create mocks
	mockChunkStorageRepo := new(MockChunkStorageRepository)
	mockBatchProgressRepo := new(MockBatchProgressRepository)
	mockBatchEmbeddingService := new(MockBatchEmbeddingService)

	// KEY EXPECTATION: Capture what chunks are passed to FindOrCreateChunks
	var capturedChunks []outbound.CodeChunk
	mockChunkStorageRepo.On("FindOrCreateChunks", ctx, mock.MatchedBy(func(c []outbound.CodeChunk) bool {
		capturedChunks = c
		return true
	})).Return(func(ctx context.Context, chunks []outbound.CodeChunk) []outbound.CodeChunk {
		// Return same chunks with IDs (simulating DB save)
		return chunks
	}, nil)

	mockBatchProgressRepo.On("Save", ctx, mock.Anything).Return(nil)

	processor := &DefaultJobProcessor{
		chunkStorageRepo:      mockChunkStorageRepo,
		batchProgressRepo:     mockBatchProgressRepo,
		batchEmbeddingService: mockBatchEmbeddingService,
		batchConfig: config.BatchProcessingConfig{
			Enabled:           true,
			UseTestEmbeddings: false,
		},
	}

	options := outbound.EmbeddingOptions{
		Model:    "gemini-embedding-001",
		TaskType: outbound.TaskTypeRetrievalDocument,
	}

	// Act
	err := processor.submitBatchJobAsync(ctx, indexingJobID, repositoryID, 1, 1, chunks, options)

	// Assert
	require.NoError(t, err)

	// CRITICAL ASSERTION: FindOrCreateChunks should receive only 3 unique chunks (not 4)
	// Chunks[0] and chunks[2] are duplicates (same repo_id, file_path, hash)
	assert.Len(t, capturedChunks, 3, "Should receive only deduplicated chunks")

	// Verify the duplicate was removed
	// Count how many chunks have the shared hash
	hashCount := 0
	for _, chunk := range capturedChunks {
		if chunk.Hash == sharedHash && chunk.FilePath == sharedFilePath {
			hashCount++
		}
	}
	assert.Equal(t, 1, hashCount, "Should have only one chunk with the duplicate hash/filepath combination")

	// Verify all remaining chunks have unique keys
	seenKeys := make(map[string]bool)
	for _, chunk := range capturedChunks {
		key := chunk.RepositoryID.String() + "|" + chunk.FilePath + "|" + chunk.Hash
		assert.False(t, seenKeys[key], "All chunks should have unique (repo_id, file_path, hash) keys")
		seenKeys[key] = true
	}
}
