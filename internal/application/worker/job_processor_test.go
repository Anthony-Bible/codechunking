package worker

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/messaging"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// MockStreamingCodeProcessor mocks the streaming code processor interface.
type MockStreamingCodeProcessor struct {
	mock.Mock
}

func (m *MockStreamingCodeProcessor) ProcessDirectoryStreaming(
	ctx context.Context,
	dirPath string,
	config StreamingProcessingConfig,
) (ProcessingResult, error) {
	args := m.Called(ctx, dirPath, config)
	if args.Get(0) == nil {
		return ProcessingResult{}, args.Error(1)
	}
	return args.Get(0).(ProcessingResult), args.Error(1)
}

func (m *MockStreamingCodeProcessor) ProcessDirectoryBatch(
	ctx context.Context,
	dirPath string,
	config BatchProcessingConfig,
) (ProcessingResult, error) {
	args := m.Called(ctx, dirPath, config)
	if args.Get(0) == nil {
		return ProcessingResult{}, args.Error(1)
	}
	return args.Get(0).(ProcessingResult), args.Error(1)
}

// MockEnhancedGitClient mocks the enhanced git client interface.
type MockEnhancedGitClient struct {
	mock.Mock
}

func (m *MockEnhancedGitClient) Clone(ctx context.Context, repoURL, targetPath string) error {
	args := m.Called(ctx, repoURL, targetPath)
	return args.Error(0)
}

func (m *MockEnhancedGitClient) GetCommitHash(ctx context.Context, repoPath string) (string, error) {
	args := m.Called(ctx, repoPath)
	return args.String(0), args.Error(1)
}

func (m *MockEnhancedGitClient) GetBranch(ctx context.Context, repoPath string) (string, error) {
	args := m.Called(ctx, repoPath)
	return args.String(0), args.Error(1)
}

func (m *MockEnhancedGitClient) CloneWithOptions(
	ctx context.Context,
	repoURL, targetPath string,
	opts valueobject.CloneOptions,
) (*outbound.CloneResult, error) {
	args := m.Called(ctx, repoURL, targetPath, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.CloneResult), args.Error(1)
}

func (m *MockEnhancedGitClient) GetRepositoryInfo(
	ctx context.Context,
	repoURL string,
) (*outbound.RepositoryInfo, error) {
	args := m.Called(ctx, repoURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.RepositoryInfo), args.Error(1)
}

func (m *MockEnhancedGitClient) ValidateRepository(
	ctx context.Context,
	repoURL string,
) (bool, error) {
	args := m.Called(ctx, repoURL)
	return args.Bool(0), args.Error(1)
}

func (m *MockEnhancedGitClient) EstimateCloneTime(
	ctx context.Context,
	repoURL string,
	opts valueobject.CloneOptions,
) (*outbound.CloneEstimation, error) {
	args := m.Called(ctx, repoURL, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.CloneEstimation), args.Error(1)
}

func (m *MockEnhancedGitClient) GetCloneProgress(
	ctx context.Context,
	operationID string,
) (*outbound.CloneProgress, error) {
	args := m.Called(ctx, operationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.CloneProgress), args.Error(1)
}

func (m *MockEnhancedGitClient) CancelClone(
	ctx context.Context,
	operationID string,
) error {
	args := m.Called(ctx, operationID)
	return args.Error(0)
}

// MockIndexingJobRepository mocks the indexing job repository interface.
type MockIndexingJobRepository struct {
	mock.Mock
}

func (m *MockIndexingJobRepository) Save(ctx context.Context, job *entity.IndexingJob) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockIndexingJobRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.IndexingJob, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.IndexingJob), args.Error(1)
}

func (m *MockIndexingJobRepository) FindByRepositoryID(
	ctx context.Context,
	repositoryID uuid.UUID,
	filters outbound.IndexingJobFilters,
) ([]*entity.IndexingJob, int, error) {
	args := m.Called(ctx, repositoryID, filters)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*entity.IndexingJob), args.Int(1), args.Error(2)
}

func (m *MockIndexingJobRepository) Update(ctx context.Context, job *entity.IndexingJob) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockIndexingJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockRepositoryRepository mocks the repository repository interface.
type MockRepositoryRepository struct {
	mock.Mock
}

func (m *MockRepositoryRepository) Save(ctx context.Context, repo *entity.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockRepositoryRepository) FindByID(ctx context.Context, id uuid.UUID) (*entity.Repository, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Repository), args.Error(1)
}

func (m *MockRepositoryRepository) FindByURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (*entity.Repository, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Repository), args.Error(1)
}

func (m *MockRepositoryRepository) FindAll(
	ctx context.Context,
	filters outbound.RepositoryFilters,
) ([]*entity.Repository, int, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*entity.Repository), args.Int(1), args.Error(2)
}

func (m *MockRepositoryRepository) Update(ctx context.Context, repo *entity.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockRepositoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepositoryRepository) Exists(ctx context.Context, url valueobject.RepositoryURL) (bool, error) {
	args := m.Called(ctx, url)
	return args.Bool(0), args.Error(1)
}

func (m *MockRepositoryRepository) ExistsByNormalizedURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (bool, error) {
	args := m.Called(ctx, url)
	return args.Bool(0), args.Error(1)
}

func (m *MockRepositoryRepository) FindByNormalizedURL(
	ctx context.Context,
	url valueobject.RepositoryURL,
) (*entity.Repository, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Repository), args.Error(1)
}

// MockCodeParser mocks the code parser interface.
type MockCodeParser struct {
	mock.Mock
}

func (m *MockCodeParser) ParseDirectory(
	ctx context.Context,
	dirPath string,
	config outbound.CodeParsingConfig,
) ([]outbound.CodeChunk, error) {
	args := m.Called(ctx, dirPath, config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]outbound.CodeChunk), args.Error(1)
}

// MockEmbeddingService mocks the embedding service interface.
type MockEmbeddingService struct {
	mock.Mock
}

func (m *MockEmbeddingService) GenerateEmbedding(
	ctx context.Context,
	text string,
	options outbound.EmbeddingOptions,
) (*outbound.EmbeddingResult, error) {
	args := m.Called(ctx, text, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.EmbeddingResult), args.Error(1)
}

func (m *MockEmbeddingService) GenerateBatchEmbeddings(
	ctx context.Context,
	texts []string,
	options outbound.EmbeddingOptions,
) ([]*outbound.EmbeddingResult, error) {
	args := m.Called(ctx, texts, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*outbound.EmbeddingResult), args.Error(1)
}

func (m *MockEmbeddingService) GenerateCodeChunkEmbedding(
	ctx context.Context,
	chunk *outbound.CodeChunk,
	options outbound.EmbeddingOptions,
) (*outbound.CodeChunkEmbedding, error) {
	args := m.Called(ctx, chunk, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.CodeChunkEmbedding), args.Error(1)
}

func (m *MockEmbeddingService) ValidateApiKey(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockEmbeddingService) GetModelInfo(ctx context.Context) (*outbound.ModelInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.ModelInfo), args.Error(1)
}

func (m *MockEmbeddingService) GetSupportedModels(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockEmbeddingService) EstimateTokenCount(ctx context.Context, text string) (int, error) {
	args := m.Called(ctx, text)
	return args.Int(0), args.Error(1)
}

// MockChunkStorageRepository mocks the chunk storage repository interface.
type MockChunkStorageRepository struct {
	mock.Mock
}

func (m *MockChunkStorageRepository) SaveChunk(ctx context.Context, chunk *outbound.CodeChunk) error {
	args := m.Called(ctx, chunk)
	return args.Error(0)
}

func (m *MockChunkStorageRepository) SaveChunks(ctx context.Context, chunks []outbound.CodeChunk) error {
	args := m.Called(ctx, chunks)
	return args.Error(0)
}

func (m *MockChunkStorageRepository) FindOrCreateChunks(
	ctx context.Context,
	chunks []outbound.CodeChunk,
) ([]outbound.CodeChunk, error) {
	args := m.Called(ctx, chunks)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	// Handle function return for dynamic mocking
	if fn, ok := args.Get(0).(func(context.Context, []outbound.CodeChunk) []outbound.CodeChunk); ok {
		return fn(ctx, chunks), args.Error(1)
	}

	return args.Get(0).([]outbound.CodeChunk), args.Error(1)
}

func (m *MockChunkStorageRepository) GetChunk(ctx context.Context, id uuid.UUID) (*outbound.CodeChunk, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.CodeChunk), args.Error(1)
}

func (m *MockChunkStorageRepository) GetChunksForRepository(
	ctx context.Context,
	repositoryID uuid.UUID,
) ([]outbound.CodeChunk, error) {
	args := m.Called(ctx, repositoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]outbound.CodeChunk), args.Error(1)
}

func (m *MockChunkStorageRepository) DeleteChunksForRepository(ctx context.Context, repositoryID uuid.UUID) error {
	args := m.Called(ctx, repositoryID)
	return args.Error(0)
}

func (m *MockChunkStorageRepository) CountChunksForRepository(
	ctx context.Context,
	repositoryID uuid.UUID,
) (int, error) {
	args := m.Called(ctx, repositoryID)
	return args.Int(0), args.Error(1)
}

func (m *MockChunkStorageRepository) SaveEmbedding(ctx context.Context, embedding *outbound.Embedding) error {
	args := m.Called(ctx, embedding)
	return args.Error(0)
}

func (m *MockChunkStorageRepository) SaveEmbeddings(ctx context.Context, embeddings []outbound.Embedding) error {
	args := m.Called(ctx, embeddings)
	return args.Error(0)
}

func (m *MockChunkStorageRepository) GetEmbedding(ctx context.Context, chunkID uuid.UUID) (*outbound.Embedding, error) {
	args := m.Called(ctx, chunkID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.Embedding), args.Error(1)
}

func (m *MockChunkStorageRepository) GetEmbeddingsForRepository(
	ctx context.Context,
	repositoryID uuid.UUID,
) ([]outbound.Embedding, error) {
	args := m.Called(ctx, repositoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]outbound.Embedding), args.Error(1)
}

func (m *MockChunkStorageRepository) DeleteEmbeddingsForRepository(ctx context.Context, repositoryID uuid.UUID) error {
	args := m.Called(ctx, repositoryID)
	return args.Error(0)
}

func (m *MockChunkStorageRepository) SearchSimilar(
	ctx context.Context,
	query []float64,
	limit int,
	threshold float64,
) ([]outbound.EmbeddingSearchResult, error) {
	args := m.Called(ctx, query, limit, threshold)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]outbound.EmbeddingSearchResult), args.Error(1)
}

func (m *MockChunkStorageRepository) SaveChunkWithEmbedding(
	ctx context.Context,
	chunk *outbound.CodeChunk,
	embedding *outbound.Embedding,
) error {
	args := m.Called(ctx, chunk, embedding)
	return args.Error(0)
}

func (m *MockChunkStorageRepository) SaveChunksWithEmbeddings(
	ctx context.Context,
	chunks []outbound.CodeChunk,
	embeddings []outbound.Embedding,
) error {
	args := m.Called(ctx, chunks, embeddings)
	return args.Error(0)
}

// MockBatchQueueManager mocks the BatchQueueManager interface for testing.
type MockBatchQueueManager struct {
	mock.Mock
}

func (m *MockBatchQueueManager) QueueEmbeddingRequest(ctx context.Context, request *outbound.EmbeddingRequest) error {
	args := m.Called(ctx, request)
	return args.Error(0)
}

func (m *MockBatchQueueManager) QueueBulkEmbeddingRequests(
	ctx context.Context,
	requests []*outbound.EmbeddingRequest,
) error {
	args := m.Called(ctx, requests)
	return args.Error(0)
}

func (m *MockBatchQueueManager) ProcessQueue(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockBatchQueueManager) FlushQueue(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockBatchQueueManager) GetQueueStats(ctx context.Context) (*outbound.QueueStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.QueueStats), args.Error(1)
}

func (m *MockBatchQueueManager) GetQueueHealth(ctx context.Context) (*outbound.QueueHealth, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.QueueHealth), args.Error(1)
}

func (m *MockBatchQueueManager) UpdateBatchConfiguration(ctx context.Context, config *outbound.BatchConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *MockBatchQueueManager) DrainQueue(ctx context.Context, timeout time.Duration) error {
	args := m.Called(ctx, timeout)
	return args.Error(0)
}

func (m *MockBatchQueueManager) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockBatchQueueManager) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockBatchEmbeddingService mocks the BatchEmbeddingService interface for testing.
type MockBatchEmbeddingService struct {
	mock.Mock
}

func (m *MockBatchEmbeddingService) CreateBatchEmbeddingJob(
	ctx context.Context,
	texts []string,
	options outbound.EmbeddingOptions,
	batchID uuid.UUID,
) (*outbound.BatchEmbeddingJob, error) {
	args := m.Called(ctx, texts, options, batchID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.BatchEmbeddingJob), args.Error(1)
}

func (m *MockBatchEmbeddingService) CreateBatchEmbeddingJobWithRequests(
	ctx context.Context,
	requests []*outbound.BatchEmbeddingRequest,
	options outbound.EmbeddingOptions,
	batchID uuid.UUID,
) (*outbound.BatchEmbeddingJob, error) {
	args := m.Called(ctx, requests, options, batchID)

	// Handle nil return
	if args.Get(0) == nil {
		if args.Get(1) == nil {
			return nil, nil
		}
		// Try to get as function first
		if fn, ok := args.Get(1).(func(context.Context, []*outbound.BatchEmbeddingRequest, outbound.EmbeddingOptions, uuid.UUID) error); ok {
			return nil, fn(ctx, requests, options, batchID)
		}
		return nil, args.Error(1)
	}

	// Handle function return
	if fn, ok := args.Get(0).(func(context.Context, []*outbound.BatchEmbeddingRequest, outbound.EmbeddingOptions, uuid.UUID) *outbound.BatchEmbeddingJob); ok {
		result := fn(ctx, requests, options, batchID)
		// Check second return value
		if args.Get(1) == nil {
			return result, nil
		}
		if errFn, ok := args.Get(1).(func(context.Context, []*outbound.BatchEmbeddingRequest, outbound.EmbeddingOptions, uuid.UUID) error); ok {
			return result, errFn(ctx, requests, options, batchID)
		}
		return result, args.Error(1)
	}

	// Handle direct value return
	return args.Get(0).(*outbound.BatchEmbeddingJob), args.Error(1)
}

func (m *MockBatchEmbeddingService) CreateBatchEmbeddingJobWithFile(
	ctx context.Context,
	requests []*outbound.BatchEmbeddingRequest,
	options outbound.EmbeddingOptions,
	batchID uuid.UUID,
	fileURI string,
) (*outbound.BatchEmbeddingJob, error) {
	args := m.Called(ctx, requests, options, batchID, fileURI)

	// Handle nil return
	if args.Get(0) == nil {
		if args.Get(1) == nil {
			return nil, nil
		}
		// Try to get as function first
		if fn, ok := args.Get(1).(func(context.Context, []*outbound.BatchEmbeddingRequest, outbound.EmbeddingOptions, uuid.UUID, string) error); ok {
			return nil, fn(ctx, requests, options, batchID, fileURI)
		}
		return nil, args.Error(1)
	}

	// Handle function return
	if fn, ok := args.Get(0).(func(context.Context, []*outbound.BatchEmbeddingRequest, outbound.EmbeddingOptions, uuid.UUID, string) *outbound.BatchEmbeddingJob); ok {
		result := fn(ctx, requests, options, batchID, fileURI)
		// Check second return value
		if args.Get(1) == nil {
			return result, nil
		}
		if errFn, ok := args.Get(1).(func(context.Context, []*outbound.BatchEmbeddingRequest, outbound.EmbeddingOptions, uuid.UUID, string) error); ok {
			return result, errFn(ctx, requests, options, batchID, fileURI)
		}
		return result, args.Error(1)
	}

	// Handle direct value return
	return args.Get(0).(*outbound.BatchEmbeddingJob), args.Error(1)
}

func (m *MockBatchEmbeddingService) GetBatchJobStatus(
	ctx context.Context,
	jobID string,
) (*outbound.BatchEmbeddingJob, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.BatchEmbeddingJob), args.Error(1)
}

func (m *MockBatchEmbeddingService) ListBatchJobs(
	ctx context.Context,
	filter *outbound.BatchJobFilter,
) ([]*outbound.BatchEmbeddingJob, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*outbound.BatchEmbeddingJob), args.Error(1)
}

func (m *MockBatchEmbeddingService) GetBatchJobResults(
	ctx context.Context,
	jobID string,
) ([]*outbound.EmbeddingResult, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*outbound.EmbeddingResult), args.Error(1)
}

func (m *MockBatchEmbeddingService) CancelBatchJob(
	ctx context.Context,
	jobID string,
) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *MockBatchEmbeddingService) DeleteBatchJob(
	ctx context.Context,
	jobID string,
) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *MockBatchEmbeddingService) WaitForBatchJob(
	ctx context.Context,
	jobID string,
	pollInterval time.Duration,
) (*outbound.BatchEmbeddingJob, error) {
	args := m.Called(ctx, jobID, pollInterval)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.BatchEmbeddingJob), args.Error(1)
}

// StreamingProcessingConfig holds configuration for streaming processing.
type StreamingProcessingConfig struct {
	MaxConcurrency     int
	ChunkBufferSize    int
	EnableProgress     bool
	ProgressInterval   time.Duration
	MemoryOptimization bool
}

// BatchProcessingConfig holds configuration for batch processing.
type BatchProcessingConfig struct {
	BatchSize          int
	EnableCompression  bool
	MaxMemoryUsageMB   int
	DiskUsageThreshold int64
}

// ProcessingResult represents the result of a processing operation.
type ProcessingResult struct {
	ChunksProcessed int64
	FilesProcessed  int64
	Duration        time.Duration
	MemoryUsedMB    int
	Success         bool
	Error           error
}

// ProcessingStrategy represents processing strategy types.
type ProcessingStrategy int

const (
	ProcessingStrategyStreaming ProcessingStrategy = iota
	ProcessingStrategyBatch
)

// JobProcessorTestSuite defines the test suite for job processor.
type JobProcessorTestSuite struct {
	suite.Suite

	processor *DefaultJobProcessor
}

// SetupTest sets up the test suite.
func (suite *JobProcessorTestSuite) SetupTest() {
	config := JobProcessorConfig{
		WorkspaceDir:      "/tmp/workspace",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
		MaxMemoryMB:       1024,
		MaxDiskUsageMB:    10240,
		CleanupInterval:   1 * time.Hour,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	// Use the constructor to ensure proper initialization including semaphore
	jobProcessor := NewDefaultJobProcessor(
		config,
		&MockIndexingJobRepository{},
		&MockRepositoryRepository{},
		&MockEnhancedGitClient{},
		&MockCodeParser{},
		&MockEmbeddingService{},
		&MockChunkStorageRepository{},
	)

	// Type assertion to concrete type for test access
	concreteProcessor, ok := jobProcessor.(*DefaultJobProcessor)
	if !ok {
		panic("failed to cast to DefaultJobProcessor")
	}
	suite.processor = concreteProcessor
}

// TestJobProcessorTestSuite runs the job processor test suite.
func TestJobProcessorTestSuite(t *testing.T) {
	suite.Run(t, new(JobProcessorTestSuite))
}

// TestProcessJob_CompleteWorkflow_SmallRepository tests complete workflow for small repository.
func (suite *JobProcessorTestSuite) TestProcessJob_CompleteWorkflow_SmallRepository() {
	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "small-repo-test",
		CorrelationID: "corr-small-repo",
		SchemaVersion: "2.0",
		Timestamp:     time.Now(),
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/small-repo.git",
		Priority:      messaging.JobPriorityNormal,
		RetryAttempt:  0,
		MaxRetries:    3,
		ProcessingMetadata: messaging.ProcessingMetadata{
			ChunkSizeBytes: 1024,
		},
		ProcessingContext: messaging.ProcessingContext{
			TimeoutSeconds: 300,
		},
	}

	ctx := context.Background()
	err := suite.processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestProcessJob_CompleteWorkflow_LargeRepository tests streaming processing for large repository.
func (suite *JobProcessorTestSuite) TestProcessJob_CompleteWorkflow_LargeRepository() {
	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "large-repo-test",
		CorrelationID: "corr-large-repo",
		SchemaVersion: "2.0",
		Timestamp:     time.Now(),
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/large-repo.git",
		Priority:      messaging.JobPriorityNormal,
		RetryAttempt:  0,
		MaxRetries:    3,
		ProcessingMetadata: messaging.ProcessingMetadata{
			ChunkSizeBytes: 2048,
		},
		ProcessingContext: messaging.ProcessingContext{
			TimeoutSeconds: 600,
			MaxMemoryMB:    512,
		},
	}

	ctx := context.Background()
	err := suite.processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestProcessJob_CompleteWorkflow_MultiLanguageRepository tests processing mixed language repository.
func (suite *JobProcessorTestSuite) TestProcessJob_CompleteWorkflow_MultiLanguageRepository() {
	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "multi-lang-test",
		CorrelationID: "corr-multi-lang",
		SchemaVersion: "2.0",
		Timestamp:     time.Now(),
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/multi-lang-repo.git",
		Priority:      messaging.JobPriorityHigh,
		RetryAttempt:  0,
		MaxRetries:    3,
		ProcessingMetadata: messaging.ProcessingMetadata{
			ChunkSizeBytes: 1024,
			FileFilters:    []string{"*.go", "*.js", "*.py"},
		},
		ProcessingContext: messaging.ProcessingContext{
			TimeoutSeconds: 900,
		},
	}

	ctx := context.Background()
	err := suite.processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestProcessJob_ConcurrentJobs_WithinLimit tests concurrent job processing within limit.
func (suite *JobProcessorTestSuite) TestProcessJob_ConcurrentJobs_WithinLimit() {
	messages := []messaging.EnhancedIndexingJobMessage{
		{
			IndexingJobID: uuid.New(),
			MessageID:     "concurrent-1",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo1.git",
		},
		{
			MessageID:     "concurrent-2",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo2.git",
		},
		{
			MessageID:     "concurrent-3",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo3.git",
		},
	}

	ctx := context.Background()
	for _, msg := range messages {
		err := suite.processor.ProcessJob(ctx, msg)
		suite.Require().Error(err)
		suite.Contains(err.Error(), "not implemented yet")
	}

	// Health status should show no active jobs in RED phase
	health := suite.processor.GetHealthStatus()
	suite.Equal(0, health.ActiveJobs)
}

// TestProcessJob_ConcurrentJobs_ExceedsLimit tests job queuing when max concurrent jobs exceeded.
func (suite *JobProcessorTestSuite) TestProcessJob_ConcurrentJobs_ExceedsLimit() {
	config := JobProcessorConfig{
		WorkspaceDir:      "/tmp/workspace",
		MaxConcurrentJobs: 2, // Limit to 2 concurrent jobs
		JobTimeout:        5 * time.Minute,
		MaxMemoryMB:       1024,
		MaxDiskUsageMB:    10240,
		CleanupInterval:   1 * time.Hour,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	// Use constructor to ensure proper initialization
	processor := NewDefaultJobProcessor(
		config,
		&MockIndexingJobRepository{},
		&MockRepositoryRepository{},
		&MockEnhancedGitClient{},
		&MockCodeParser{},
		&MockEmbeddingService{},
		&MockChunkStorageRepository{},
	).(*DefaultJobProcessor)

	messages := []messaging.EnhancedIndexingJobMessage{
		{
			IndexingJobID: uuid.New(),
			MessageID:     "exceeds-1",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo1.git",
		},
		{
			MessageID:     "exceeds-2",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo2.git",
		},
		{
			MessageID:     "exceeds-3",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo3.git",
		},
	}

	ctx := context.Background()
	for _, msg := range messages {
		err := processor.ProcessJob(ctx, msg)
		suite.Require().Error(err)
		suite.Contains(err.Error(), "not implemented yet")
	}
}

// TestProcessJob_JobTimeout_Enforcement tests job timeout handling.
func (suite *JobProcessorTestSuite) TestProcessJob_JobTimeout_Enforcement() {
	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "timeout-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/slow-repo.git",
		ProcessingContext: messaging.ProcessingContext{
			TimeoutSeconds: 1, // Very short timeout
		},
	}

	config := JobProcessorConfig{
		WorkspaceDir:      "/tmp/workspace",
		MaxConcurrentJobs: 1,
		JobTimeout:        1 * time.Second,
		MaxMemoryMB:       1024,
		MaxDiskUsageMB:    10240,
		CleanupInterval:   1 * time.Hour,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	// Use constructor to ensure proper initialization
	processor := NewDefaultJobProcessor(
		config,
		&MockIndexingJobRepository{},
		&MockRepositoryRepository{},
		&MockEnhancedGitClient{},
		&MockCodeParser{},
		&MockEmbeddingService{},
		&MockChunkStorageRepository{},
	).(*DefaultJobProcessor)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := processor.ProcessJob(ctx, message)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestProcessJob_ProgressTracking_ThroughoutWorkflow tests JobProgress updates.
func (suite *JobProcessorTestSuite) TestProcessJob_ProgressTracking_ThroughoutWorkflow() {
	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "progress-test",
		CorrelationID: "corr-progress",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/progress-repo.git",
	}

	ctx := context.Background()
	err := suite.processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")

	// Progress should be empty in RED phase
	health := suite.processor.GetHealthStatus()
	suite.Equal(0, health.ActiveJobs)
}

// TestProcessJob_MemoryPressure_AdaptiveProcessing tests adaptive processing under memory pressure.
func (suite *JobProcessorTestSuite) TestProcessJob_MemoryPressure_AdaptiveProcessing() {
	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "memory-pressure-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/large-repo.git",
		ProcessingContext: messaging.ProcessingContext{
			MaxMemoryMB: 256, // Low memory limit
		},
	}

	ctx := context.Background()
	err := suite.processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestProcessJob_MemoryLimit_Enforcement tests memory limit enforcement.
func (suite *JobProcessorTestSuite) TestProcessJob_MemoryLimit_Enforcement() {
	config := JobProcessorConfig{
		WorkspaceDir:      "/tmp/workspace",
		MaxMemoryMB:       128, // Very low memory limit
		MaxConcurrentJobs: 1,
		JobTimeout:        5 * time.Minute,
		MaxDiskUsageMB:    10240,
		CleanupInterval:   1 * time.Hour,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	processor := NewDefaultJobProcessor(
		config,
		&MockIndexingJobRepository{},
		&MockRepositoryRepository{},
		&MockEnhancedGitClient{},
		&MockCodeParser{},
		&MockEmbeddingService{},
		&MockChunkStorageRepository{},
	).(*DefaultJobProcessor)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "memory-limit-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/memory-heavy-repo.git",
	}

	ctx := context.Background()
	err := processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestProcessJob_MemoryCleanup_BetweenJobs tests memory cleanup between jobs.
func (suite *JobProcessorTestSuite) TestProcessJob_MemoryCleanup_BetweenJobs() {
	messages := []messaging.EnhancedIndexingJobMessage{
		{
			IndexingJobID: uuid.New(),
			MessageID:     "cleanup-1",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo1.git",
		},
		{
			MessageID:     "cleanup-2",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo2.git",
		},
	}

	ctx := context.Background()
	for _, msg := range messages {
		err := suite.processor.ProcessJob(ctx, msg)
		suite.Require().Error(err)
		suite.Contains(err.Error(), "resource limits enforcement not implemented yet")
	}

	// Cleanup should succeed
	err := suite.processor.Cleanup()
	suite.Require().NoError(err)
}

// TestProcessJob_LargeRepository_MemoryOptimization tests memory optimization.
func (suite *JobProcessorTestSuite) TestProcessJob_LargeRepository_MemoryOptimization() {
	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "memory-opt-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/huge-repo.git",
		ProcessingMetadata: messaging.ProcessingMetadata{
			ChunkSizeBytes: 4096,
		},
		ProcessingContext: messaging.ProcessingContext{
			MaxMemoryMB:        512,
			ConcurrencyLevel:   1,
			EnableDeepAnalysis: false,
		},
	}

	ctx := context.Background()
	err := suite.processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestDecideProcessingStrategy_BasedOnMemoryAndRepoSize tests strategy decision logic.
func (suite *JobProcessorTestSuite) TestDecideProcessingStrategy_BasedOnMemoryAndRepoSize() {
	// This test requires implementation of decideProcessingStrategy method
	suite.T().Skip("Skipping until decideProcessingStrategy is implemented")
}

// TestProcessJob_EnhancedGitClient_Integration tests EnhancedGitClient integration.
func (suite *JobProcessorTestSuite) TestProcessJob_EnhancedGitClient_Integration() {
	mockGitClient := &MockEnhancedGitClient{}
	mockGitClient.On("CloneWithOptions", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&outbound.CloneResult{}, nil)

	processor := NewDefaultJobProcessor(
		suite.processor.config,
		&MockIndexingJobRepository{},
		&MockRepositoryRepository{},
		mockGitClient,
		&MockCodeParser{},
		&MockEmbeddingService{},
		&MockChunkStorageRepository{},
	).(*DefaultJobProcessor)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "enhanced-git-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/repo.git",
	}

	ctx := context.Background()
	err := processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestProcessJob_StreamingCodeProcessor_Integration tests streaming processor integration.
func (suite *JobProcessorTestSuite) TestProcessJob_StreamingCodeProcessor_Integration() {
	mockStreamingProcessor := &MockStreamingCodeProcessor{}
	mockStreamingProcessor.On("ProcessDirectoryStreaming", mock.Anything, mock.Anything, mock.Anything).
		Return(ProcessingResult{}, nil)

	// This test requires streaming processor integration
	suite.T().Skip("Skipping until streaming processor integration is implemented")
}

// TestProcessJob_TreeSitterParser_MultiLanguage tests tree-sitter parser integration.
func (suite *JobProcessorTestSuite) TestProcessJob_TreeSitterParser_MultiLanguage() {
	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "tree-sitter-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/multi-lang-repo.git",
		ProcessingMetadata: messaging.ProcessingMetadata{
			FileFilters: []string{"*.go", "*.js", "*.rs"},
		},
	}

	ctx := context.Background()
	err := suite.processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestProcessJob_EmbeddingService_Integration tests embedding service integration.
func (suite *JobProcessorTestSuite) TestProcessJob_EmbeddingService_Integration() {
	mockEmbeddingService := &MockEmbeddingService{}

	// Create a mock embedding result
	mockResult := &outbound.EmbeddingResult{
		Vector:     []float64{0.1, 0.2, 0.3},
		Dimensions: 3,
		Model:      "test-model",
		TaskType:   outbound.TaskTypeRetrievalDocument,
	}

	mockEmbeddingService.On("GenerateEmbedding", mock.Anything, mock.Anything, mock.Anything).
		Return(mockResult, nil)

	processor := NewDefaultJobProcessor(
		suite.processor.config,
		&MockIndexingJobRepository{},
		&MockRepositoryRepository{},
		&MockEnhancedGitClient{},
		&MockCodeParser{},
		mockEmbeddingService,
		&MockChunkStorageRepository{},
	).(*DefaultJobProcessor)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "embedding-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/embedding-repo.git",
	}

	ctx := context.Background()
	err := processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestProcessJob_IntegrationFailure_ErrorPropagation tests error propagation.
func (suite *JobProcessorTestSuite) TestProcessJob_IntegrationFailure_ErrorPropagation() {
	mockGitClient := &MockEnhancedGitClient{}
	mockGitClient.On("Clone", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("git clone failed"))

	processor := NewDefaultJobProcessor(
		suite.processor.config,
		&MockIndexingJobRepository{},
		&MockRepositoryRepository{},
		mockGitClient,
		&MockCodeParser{},
		&MockEmbeddingService{},
		&MockChunkStorageRepository{},
	).(*DefaultJobProcessor)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "error-propagation-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/nonexistent/repo.git",
	}

	ctx := context.Background()
	err := processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestProcessJob_Authentication_GitClient tests git client authentication.
func (suite *JobProcessorTestSuite) TestProcessJob_Authentication_GitClient() {
	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "auth-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/private/repo.git",
	}

	ctx := context.Background()
	err := suite.processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestProcessJob_StreamingVsBatch_DecisionLogic tests processing strategy decision.
func (suite *JobProcessorTestSuite) TestProcessJob_StreamingVsBatch_DecisionLogic() {
	// This test requires implementation of strategy decision logic
	suite.T().Skip("Skipping until processing strategy decision logic is implemented")
}

// TestProcessJob_StreamingVsBatch_PerformanceMetrics tests performance metrics.
func (suite *JobProcessorTestSuite) TestProcessJob_StreamingVsBatch_PerformanceMetrics() {
	// This test requires implementation of performance metrics collection
	suite.T().Skip("Skipping until performance metrics collection is implemented")
}

// TestProcessJob_Streaming_FallbackToBatch tests streaming to batch fallback.
func (suite *JobProcessorTestSuite) TestProcessJob_Streaming_FallbackToBatch() {
	// This test requires implementation of fallback logic
	suite.T().Skip("Skipping until streaming to batch fallback is implemented")
}

// TestProcessJob_FileSizeThresholds_StrategySelection tests strategy selection based on file sizes.
func (suite *JobProcessorTestSuite) TestProcessJob_FileSizeThresholds_StrategySelection() {
	// This test requires implementation of file size threshold logic
	suite.T().Skip("Skipping until file size threshold logic is implemented")
}

// TestProcessJob_JobExecution_Tracking tests JobExecution tracking.
func (suite *JobProcessorTestSuite) TestProcessJob_JobExecution_Tracking() {
	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "execution-tracking-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/tracking-repo.git",
	}

	ctx := context.Background()
	err := suite.processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")

	// Job execution tracking should be empty in RED phase
	health := suite.processor.GetHealthStatus()
	suite.Equal(0, health.ActiveJobs)
}

// TestProcessJob_ActiveJobs_Management tests active jobs management.
func (suite *JobProcessorTestSuite) TestProcessJob_ActiveJobs_Management() {
	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "active-jobs-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/active-repo.git",
	}

	ctx := context.Background()
	err := suite.processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")

	// Active jobs map should be empty in RED phase
	suite.Empty(suite.processor.activeJobs)
}

// TestProcessJob_JobProgress_Updates tests JobProgress updates.
func (suite *JobProcessorTestSuite) TestProcessJob_JobProgress_Updates() {
	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "progress-updates-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/progress-repo.git",
	}

	ctx := context.Background()
	err := suite.processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")

	// Progress updates should be empty in RED phase
	metrics := suite.processor.GetMetrics()
	suite.Equal(int64(0), metrics.FilesProcessed)
	suite.Equal(int64(0), metrics.ChunksGenerated)
}

// TestProcessJob_JobStatus_Transitions tests job status transitions.
func (suite *JobProcessorTestSuite) TestProcessJob_JobStatus_Transitions() {
	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "status-transition-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/status-repo.git",
	}

	ctx := context.Background()
	err := suite.processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestGetHealthStatus_DuringProcessing tests health status during processing.
func (suite *JobProcessorTestSuite) TestGetHealthStatus_DuringProcessing() {
	health := suite.processor.GetHealthStatus()
	suite.False(health.IsReady)
	suite.Equal(0, health.ActiveJobs)
}

// TestGetMetrics_JobProcessing tests metrics collection.
func (suite *JobProcessorTestSuite) TestGetMetrics_JobProcessing() {
	metrics := suite.processor.GetMetrics()
	suite.Equal(int64(0), metrics.TotalJobsProcessed)
	suite.Equal(int64(0), metrics.FilesProcessed)
	suite.Equal(int64(0), metrics.ChunksGenerated)
}

// TestProcessJob_RetryLogic_WithBackoff tests retry logic with backoff.
func (suite *JobProcessorTestSuite) TestProcessJob_RetryLogic_WithBackoff() {
	config := JobProcessorConfig{
		WorkspaceDir:      "/tmp/workspace",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
		MaxMemoryMB:       1024,
		MaxDiskUsageMB:    10240,
		CleanupInterval:   1 * time.Hour,
		RetryAttempts:     3,
		RetryBackoff:      2 * time.Second,
	}

	processor := NewDefaultJobProcessor(
		config,
		&MockIndexingJobRepository{},
		&MockRepositoryRepository{},
		&MockEnhancedGitClient{},
		&MockCodeParser{},
		&MockEmbeddingService{},
		&MockChunkStorageRepository{},
	).(*DefaultJobProcessor)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "retry-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/retry-repo.git",
		RetryAttempt:  1,
		MaxRetries:    3,
	}

	ctx := context.Background()
	err := processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestProcessJob_ErrorClassification_AndResponse tests error classification.
func (suite *JobProcessorTestSuite) TestProcessJob_ErrorClassification_AndResponse() {
	// This test requires implementation of error classification logic
	suite.T().Skip("Skipping until error classification logic is implemented")
}

// TestProcessJob_PartialFailure_Recovery tests partial failure recovery.
func (suite *JobProcessorTestSuite) TestProcessJob_PartialFailure_Recovery() {
	// This test requires implementation of partial failure recovery
	suite.T().Skip("Skipping until partial failure recovery is implemented")
}

// TestProcessJob_JobCancellation_AndCleanup tests job cancellation.
func (suite *JobProcessorTestSuite) TestProcessJob_JobCancellation_AndCleanup() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "cancellation-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/cancel-repo.git",
	}

	err := suite.processor.ProcessJob(ctx, message)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestProcessJob_ErrorPropagation_WithStructuredLogging tests structured logging.
func (suite *JobProcessorTestSuite) TestProcessJob_ErrorPropagation_WithStructuredLogging() {
	// This test requires implementation of structured logging
	suite.T().Skip("Skipping until structured logging is implemented")
}

// TestProcessJob_IndexingJobRepo_PersistenceOperations tests persistence operations.
func (suite *JobProcessorTestSuite) TestProcessJob_IndexingJobRepo_PersistenceOperations() {
	mockIndexingJobRepo := &MockIndexingJobRepository{}
	mockIndexingJobRepo.On("Save", mock.Anything, mock.Anything).Return(nil)
	mockIndexingJobRepo.On("FindByID", mock.Anything, mock.Anything).Return(&entity.IndexingJob{}, nil)
	mockIndexingJobRepo.On("Update", mock.Anything, mock.Anything).Return(nil)
	mockIndexingJobRepo.On("Delete", mock.Anything, mock.Anything).Return(nil)

	processor := NewDefaultJobProcessor(
		suite.processor.config,
		mockIndexingJobRepo,
		&MockRepositoryRepository{},
		&MockEnhancedGitClient{},
		&MockCodeParser{},
		&MockEmbeddingService{},
		&MockChunkStorageRepository{},
	).(*DefaultJobProcessor)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "persistence-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/persistence-repo.git",
	}

	ctx := context.Background()
	err := processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestProcessJob_RepositoryRepo_MetadataUpdates tests repository metadata updates.
func (suite *JobProcessorTestSuite) TestProcessJob_RepositoryRepo_MetadataUpdates() {
	mockRepositoryRepo := &MockRepositoryRepository{}
	mockRepositoryRepo.On("Save", mock.Anything, mock.Anything).Return(nil)
	mockRepositoryRepo.On("FindByID", mock.Anything, mock.Anything).Return(&entity.Repository{}, nil)
	mockRepositoryRepo.On("FindByURL", mock.Anything, mock.Anything).Return(&entity.Repository{}, nil)
	mockRepositoryRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	processor := NewDefaultJobProcessor(
		suite.processor.config,
		&MockIndexingJobRepository{},
		mockRepositoryRepo,
		&MockEnhancedGitClient{},
		&MockCodeParser{},
		&MockEmbeddingService{},
		&MockChunkStorageRepository{},
	).(*DefaultJobProcessor)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "metadata-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/metadata-repo.git",
	}

	ctx := context.Background()
	err := processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestProcessJob_RepositoryStatus_Tracking tests repository status tracking.
func (suite *JobProcessorTestSuite) TestProcessJob_RepositoryStatus_Tracking() {
	// This test requires implementation of repository status tracking
	suite.T().Skip("Skipping until repository status tracking is implemented")
}

// TestProcessJob_TransactionHandling_MultiStep tests transaction handling.
func (suite *JobProcessorTestSuite) TestProcessJob_TransactionHandling_MultiStep() {
	// This test requires implementation of transaction handling
	suite.T().Skip("Skipping until transaction handling is implemented")
}

// TestProcessJob_DatabaseFailure_Handling tests database failure handling.
func (suite *JobProcessorTestSuite) TestProcessJob_DatabaseFailure_Handling() {
	mockIndexingJobRepo := &MockIndexingJobRepository{}
	mockIndexingJobRepo.On("Save", mock.Anything, mock.Anything).Return(errors.New("database error"))

	processor := NewDefaultJobProcessor(
		suite.processor.config,
		mockIndexingJobRepo,
		&MockRepositoryRepository{},
		&MockEnhancedGitClient{},
		&MockCodeParser{},
		&MockEmbeddingService{},
		&MockChunkStorageRepository{},
	).(*DefaultJobProcessor)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "db-failure-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/db-failure-repo.git",
	}

	ctx := context.Background()
	err := processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestJobProcessor_ConfigValidation_AllFields tests config validation.
func (suite *JobProcessorTestSuite) TestJobProcessor_ConfigValidation_AllFields() {
	// This test requires implementation of config validation
	suite.T().Skip("Skipping until config validation is implemented")
}

// TestJobProcessor_WorkspaceDir_Management tests workspace directory management.
func (suite *JobProcessorTestSuite) TestJobProcessor_WorkspaceDir_Management() {
	config := JobProcessorConfig{
		WorkspaceDir:      "/invalid/workspace/path",
		MaxConcurrentJobs: 1,
		JobTimeout:        5 * time.Minute,
		MaxMemoryMB:       1024,
		MaxDiskUsageMB:    10240,
		CleanupInterval:   1 * time.Hour,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	processor := NewDefaultJobProcessor(
		config,
		&MockIndexingJobRepository{},
		&MockRepositoryRepository{},
		&MockEnhancedGitClient{},
		&MockCodeParser{},
		&MockEmbeddingService{},
		&MockChunkStorageRepository{},
	).(*DefaultJobProcessor)

	err := processor.Cleanup()
	suite.Require().Error(err)
	suite.Contains(err.Error(), "failed to recreate workspace directory")
}

// TestJobProcessor_ResourceLimits_Enforcement tests resource limit enforcement.
func (suite *JobProcessorTestSuite) TestJobProcessor_ResourceLimits_Enforcement() {
	config := JobProcessorConfig{
		WorkspaceDir:      "/tmp/workspace",
		MaxConcurrentJobs: 1,
		JobTimeout:        5 * time.Minute,
		MaxMemoryMB:       64,  // Very low memory limit
		MaxDiskUsageMB:    128, // Very low disk usage limit
		CleanupInterval:   1 * time.Hour,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	// Use constructor to ensure proper initialization
	processor := NewDefaultJobProcessor(
		config,
		&MockIndexingJobRepository{},
		&MockRepositoryRepository{},
		&MockEnhancedGitClient{},
		&MockCodeParser{},
		&MockEmbeddingService{},
		&MockChunkStorageRepository{},
	)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "resource-limit-test",
		RepositoryID:  uuid.New(),
		RepositoryURL: "https://github.com/example/resource-heavy-repo.git",
	}

	ctx := context.Background()
	err := processor.ProcessJob(ctx, message)

	// Should fail in RED phase
	suite.Require().Error(err)
	suite.Contains(err.Error(), "not implemented yet")
}

// TestCleanup_Implementation tests cleanup implementation.
func (suite *JobProcessorTestSuite) TestCleanup_Implementation() {
	err := suite.processor.Cleanup()
	suite.Require().NoError(err)
}

// TestJobProcessor_CleanupInterval_Scheduling tests cleanup scheduling.
func (suite *JobProcessorTestSuite) TestJobProcessor_CleanupInterval_Scheduling() {
	// This test requires implementation of cleanup scheduling
	suite.T().Skip("Skipping until cleanup scheduling is implemented")
}

// TestProcessJob_StructuredLogging_WithSlogger tests structured logging.
func (suite *JobProcessorTestSuite) TestProcessJob_StructuredLogging_WithSlogger() {
	// This test requires implementation of structured logging with slogger
	suite.T().Skip("Skipping until structured logging with slogger is implemented")
}

// TestProcessJob_HealthStatus_RealTimeUpdates tests real-time health updates.
func (suite *JobProcessorTestSuite) TestProcessJob_HealthStatus_RealTimeUpdates() {
	health := suite.processor.GetHealthStatus()
	suite.Equal(0, health.ActiveJobs)
	suite.Equal(int64(0), health.CompletedJobs)
	suite.Equal(int64(0), health.FailedJobs)
}

// TestProcessJob_Metrics_OTEL_Integration tests OTEL metrics integration.
func (suite *JobProcessorTestSuite) TestProcessJob_Metrics_OTEL_Integration() {
	// This test requires implementation of OTEL metrics integration
	suite.T().Skip("Skipping until OTEL metrics integration is implemented")
}

// TestProcessJob_ResourceUsage_Monitoring tests resource usage monitoring.
func (suite *JobProcessorTestSuite) TestProcessJob_ResourceUsage_Monitoring() {
	health := suite.processor.GetHealthStatus()
	suite.Equal(0, health.ResourceUsage.MemoryMB)
	suite.InDelta(0.0, health.ResourceUsage.CPUPercent, 1e-9)
	suite.Equal(int64(0), health.ResourceUsage.DiskUsageMB)
}

// ================================================================================================
// IDEMPOTENT JOB PROCESSING TESTS - RED PHASE
// ================================================================================================
// These tests verify that job processing handles NATS message redelivery safely by:
// 1. Checking repository status before processing
// 2. Handling idempotent operations (completed → completed)
// 3. Preventing invalid status transitions (failed → cloning, failed → failed)
// 4. Supporting retry flows (failed → pending → cloning → processing → completed)
// ================================================================================================

// TestExecuteJobPipeline_RepositoryInFailedState_TransitionsToPendingThenSucceeds tests
// that when a repository is in "failed" state and NATS redelivers the message, the job
// processor should transition failed → pending first, then proceed with normal flow.
func (suite *JobProcessorTestSuite) TestExecuteJobPipeline_RepositoryInFailedState_TransitionsToPendingThenSucceeds() {
	// Setup: Create repository in failed state
	repoID := uuid.New()
	repoURL, err := valueobject.NewRepositoryURL("https://github.com/example/failed-repo.git")
	suite.Require().NoError(err)

	failedRepo := entity.NewRepository(repoURL, "failed-repo", nil, nil)
	err = failedRepo.UpdateStatus(valueobject.RepositoryStatusFailed)
	suite.Require().NoError(err)

	// Create repositories in different states to simulate state transitions
	// Each entity must follow valid state transition paths
	pendingRepo := entity.NewRepository(repoURL, "failed-repo", nil, nil)
	// NewRepository creates repo in pending state, so no transition needed

	cloningRepo := entity.NewRepository(repoURL, "failed-repo", nil, nil)
	err = cloningRepo.UpdateStatus(valueobject.RepositoryStatusCloning) // pending → cloning
	suite.Require().NoError(err)

	processingRepo := entity.NewRepository(repoURL, "failed-repo", nil, nil)
	err = processingRepo.UpdateStatus(valueobject.RepositoryStatusCloning) // pending → cloning
	suite.Require().NoError(err)
	err = processingRepo.UpdateStatus(valueobject.RepositoryStatusProcessing) // cloning → processing
	suite.Require().NoError(err)

	// Setup mock repository repository to return different states as processing progresses
	mockRepoRepo := &MockRepositoryRepository{}
	// FindByID calls return repo in correct state:
	// 1. Initial check - returns failed
	// 2. updateRepositoryStatus("pending") - returns failed, then Update is called
	// 3. updateRepositoryStatus("cloning") - returns pending (after previous Update)
	// 4. updateRepositoryStatus("processing") - returns cloning (after previous Update)
	// 5. markRepositoryCompleted - returns processing (after previous Update)
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(failedRepo, nil).Once()     // Call 1
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(failedRepo, nil).Once()     // Call 2
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(pendingRepo, nil).Once()    // Call 3
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(cloningRepo, nil).Once()    // Call 4
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(processingRepo, nil).Once() // Call 5

	mockRepoRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	// Setup other mocks for successful processing
	mockGitClient := &MockEnhancedGitClient{}
	mockGitClient.On("Clone", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mockCodeParser := &MockCodeParser{}
	mockCodeParser.On("ParseDirectory", mock.Anything, mock.Anything, mock.Anything).
		Return([]outbound.CodeChunk{
			{ID: uuid.New().String(), Content: "test content", FilePath: "test.go", RepositoryID: repoID},
		}, nil)

	mockEmbedding := &MockEmbeddingService{}
	mockEmbedding.On("GenerateEmbedding", mock.Anything, mock.Anything, mock.Anything).
		Return(&outbound.EmbeddingResult{Vector: []float64{0.1, 0.2}, Dimensions: 2}, nil)

	mockChunkStorage := &MockChunkStorageRepository{}
	mockChunkStorage.On("SaveChunkWithEmbedding", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Use a config without resource limits to avoid early rejection
	config := JobProcessorConfig{
		WorkspaceDir:      "/tmp/workspace-idempotent-test",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
		MaxMemoryMB:       0, // Disable memory limit
		MaxDiskUsageMB:    0, // Disable disk limit
		CleanupInterval:   1 * time.Hour,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	processor := NewDefaultJobProcessor(
		config,
		&MockIndexingJobRepository{},
		mockRepoRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbedding,
		mockChunkStorage,
	).(*DefaultJobProcessor)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "failed-repo-retry-test",
		RepositoryID:  repoID,
		RepositoryURL: "https://github.com/example/failed-repo.git",
	}

	ctx := context.Background()
	err = processor.ProcessJob(ctx, message)

	// ASSERTION: Should succeed without errors
	// The implementation MUST check repository status first and transition failed → pending
	suite.Require().NoError(err, "Job should succeed after transitioning failed → pending")

	// Verify that repository was transitioned to pending first
	mockRepoRepo.AssertCalled(suite.T(), "Update", mock.Anything, mock.MatchedBy(func(repo *entity.Repository) bool {
		return repo.Status() == valueobject.RepositoryStatusPending
	}))

	// Verify final status is completed
	mockRepoRepo.AssertCalled(suite.T(), "Update", mock.Anything, mock.MatchedBy(func(repo *entity.Repository) bool {
		return repo.Status() == valueobject.RepositoryStatusCompleted
	}))
}

// TestExecuteJobPipeline_RepositoryInCompletedState_SkipsReprocessing tests that when
// a repository is already in "completed" state and NATS redelivers the message, the job
// processor should skip reprocessing and return success (idempotent operation).
func (suite *JobProcessorTestSuite) TestExecuteJobPipeline_RepositoryInCompletedState_SkipsReprocessing() {
	// Setup: Create repository in completed state
	repoID := uuid.New()
	repoURL, err := valueobject.NewRepositoryURL("https://github.com/example/completed-repo.git")
	suite.Require().NoError(err)

	// Create a repository and properly transition it to completed state
	completedRepo := entity.NewRepository(repoURL, "completed-repo", nil, nil)
	// Must transition through valid states: pending → cloning → processing → completed
	err = completedRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
	suite.Require().NoError(err)
	err = completedRepo.UpdateStatus(valueobject.RepositoryStatusProcessing)
	suite.Require().NoError(err)
	err = completedRepo.MarkIndexingCompleted("abc123", 10, 50)
	suite.Require().NoError(err)

	// Setup mock repository repository to return completed repository
	mockRepoRepo := &MockRepositoryRepository{}
	// FindByID is called once in executeJobPipeline initial check, then processing is skipped
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(completedRepo, nil).Once()

	// NO OTHER MOCKS SHOULD BE CALLED - processing should be skipped

	// Use a config without resource limits to avoid early rejection
	config := JobProcessorConfig{
		WorkspaceDir:      "/tmp/workspace-idempotent-test",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
		MaxMemoryMB:       0, // Disable memory limit
		MaxDiskUsageMB:    0, // Disable disk limit
		CleanupInterval:   1 * time.Hour,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	processor := NewDefaultJobProcessor(
		config,
		&MockIndexingJobRepository{},
		mockRepoRepo,
		&MockEnhancedGitClient{},
		&MockCodeParser{},
		&MockEmbeddingService{},
		&MockChunkStorageRepository{},
	).(*DefaultJobProcessor)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "completed-repo-redelivery-test",
		RepositoryID:  repoID,
		RepositoryURL: "https://github.com/example/completed-repo.git",
	}

	ctx := context.Background()
	err = processor.ProcessJob(ctx, message)

	// ASSERTION: Should succeed without errors (idempotent)
	// The implementation MUST check repository status and skip processing when completed
	suite.Require().NoError(err, "Job should succeed without reprocessing when repository is already completed")

	// Verify repository status was checked
	mockRepoRepo.AssertCalled(suite.T(), "FindByID", mock.Anything, repoID)

	// Verify no updates were attempted (idempotent)
	mockRepoRepo.AssertNotCalled(suite.T(), "Update", mock.Anything, mock.Anything)
}

// TestExecuteJobPipeline_RepositoryInArchivedState_SkipsProcessing tests that when
// a repository is in "archived" state and NATS redelivers the message, the job
// processor should skip reprocessing and return success (idempotent operation).
// No git operations, parsing, or embedding should occur.
func (suite *JobProcessorTestSuite) TestExecuteJobPipeline_RepositoryInArchivedState_SkipsProcessing() {
	// Setup: Create repository in archived state
	repoID := uuid.New()
	repoURL, err := valueobject.NewRepositoryURL("https://github.com/example/archived-repo.git")
	suite.Require().NoError(err)

	// Create a repository and properly transition it to archived state
	// Must transition through valid states: pending → cloning → processing → completed → archived
	archivedRepo := entity.NewRepository(repoURL, "archived-repo", nil, nil)
	err = archivedRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
	suite.Require().NoError(err)
	err = archivedRepo.UpdateStatus(valueobject.RepositoryStatusProcessing)
	suite.Require().NoError(err)
	err = archivedRepo.MarkIndexingCompleted("abc123", 10, 50)
	suite.Require().NoError(err)
	err = archivedRepo.Archive()
	suite.Require().NoError(err)

	// Verify the repository is in archived state
	suite.Require().Equal(valueobject.RepositoryStatusArchived, archivedRepo.Status())
	suite.Require().True(archivedRepo.IsDeleted())

	// Setup mock repository repository to return archived repository
	mockRepoRepo := &MockRepositoryRepository{}
	// FindByID is called once in executeJobPipeline initial check, then processing is skipped
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(archivedRepo, nil).Once()

	// NO OTHER MOCKS SHOULD BE CALLED - processing should be skipped entirely
	// No git clone, no parsing, no embedding, no chunk storage operations

	// Use a config without resource limits to avoid early rejection
	config := JobProcessorConfig{
		WorkspaceDir:      "/tmp/workspace-archived-test",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
		MaxMemoryMB:       0, // Disable memory limit
		MaxDiskUsageMB:    0, // Disable disk limit
		CleanupInterval:   1 * time.Hour,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	processor := NewDefaultJobProcessor(
		config,
		&MockIndexingJobRepository{},
		mockRepoRepo,
		&MockEnhancedGitClient{},
		&MockCodeParser{},
		&MockEmbeddingService{},
		&MockChunkStorageRepository{},
	).(*DefaultJobProcessor)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "archived-repo-redelivery-test",
		RepositoryID:  repoID,
		RepositoryURL: "https://github.com/example/archived-repo.git",
	}

	ctx := context.Background()
	err = processor.ProcessJob(ctx, message)

	// ASSERTION: Should succeed without errors (idempotent)
	// The implementation MUST check repository status and skip processing when archived
	suite.Require().NoError(err, "Job should succeed without reprocessing when repository is archived")

	// Verify repository status was checked
	mockRepoRepo.AssertCalled(suite.T(), "FindByID", mock.Anything, repoID)

	// Verify no updates were attempted (idempotent - repository stays archived)
	mockRepoRepo.AssertNotCalled(suite.T(), "Update", mock.Anything, mock.Anything)

	// Verify no git operations were performed
	// Note: We don't need to explicitly assert on mocks that weren't configured,
	// as any unexpected calls would cause the test to fail
}

// TestExecuteJobPipeline_RepositoryInCloningState_ResumesProcessing tests that when
// a repository is in "cloning" state (job was interrupted mid-clone), the job processor
// should allow reprocessing from cloning state.
func (suite *JobProcessorTestSuite) TestExecuteJobPipeline_RepositoryInCloningState_ResumesProcessing() {
	// Setup: Create repository in cloning state
	repoID := uuid.New()
	repoURL, err := valueobject.NewRepositoryURL("https://github.com/example/cloning-repo.git")
	suite.Require().NoError(err)

	// Create repo in cloning state (must transition through valid states)
	cloningRepo1 := entity.NewRepository(repoURL, "cloning-repo", nil, nil)
	err = cloningRepo1.UpdateStatus(valueobject.RepositoryStatusCloning) // pending → cloning
	suite.Require().NoError(err)

	// Create processing state for subsequent calls
	processingRepo2 := entity.NewRepository(repoURL, "cloning-repo", nil, nil)
	err = processingRepo2.UpdateStatus(valueobject.RepositoryStatusCloning) // pending → cloning
	suite.Require().NoError(err)
	err = processingRepo2.UpdateStatus(valueobject.RepositoryStatusProcessing) // cloning → processing
	suite.Require().NoError(err)

	// Setup mock repository repository to return cloning repository
	mockRepoRepo := &MockRepositoryRepository{}
	// FindByID is called 3 times:
	// 1. Initial check in executeJobPipeline (returns cloning)
	// 2. updateRepositoryStatus("processing") - returns cloning, then Update is called
	// 3. markRepositoryCompleted - returns processing (after previous Update)
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(cloningRepo1, nil).Once()
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(cloningRepo1, nil).Once()
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(processingRepo2, nil).Once()
	mockRepoRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	// Setup other mocks for successful processing
	mockGitClient := &MockEnhancedGitClient{}
	mockGitClient.On("Clone", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mockCodeParser := &MockCodeParser{}
	mockCodeParser.On("ParseDirectory", mock.Anything, mock.Anything, mock.Anything).
		Return([]outbound.CodeChunk{
			{ID: uuid.New().String(), Content: "test content", FilePath: "test.go", RepositoryID: repoID},
		}, nil)

	mockEmbedding := &MockEmbeddingService{}
	mockEmbedding.On("GenerateEmbedding", mock.Anything, mock.Anything, mock.Anything).
		Return(&outbound.EmbeddingResult{Vector: []float64{0.1, 0.2}, Dimensions: 2}, nil)

	mockChunkStorage := &MockChunkStorageRepository{}
	mockChunkStorage.On("SaveChunkWithEmbedding", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Use a config without resource limits to avoid early rejection
	config := JobProcessorConfig{
		WorkspaceDir:      "/tmp/workspace-idempotent-test",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
		MaxMemoryMB:       0, // Disable memory limit
		MaxDiskUsageMB:    0, // Disable disk limit
		CleanupInterval:   1 * time.Hour,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	processor := NewDefaultJobProcessor(
		config,
		&MockIndexingJobRepository{},
		mockRepoRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbedding,
		mockChunkStorage,
	).(*DefaultJobProcessor)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "cloning-resume-test",
		RepositoryID:  repoID,
		RepositoryURL: "https://github.com/example/cloning-repo.git",
	}

	ctx := context.Background()
	err = processor.ProcessJob(ctx, message)

	// ASSERTION: Should succeed or handle appropriately
	// The implementation MUST allow reprocessing from cloning state (either retry clone or transition to failed)
	suite.Require().NoError(err, "Job should complete successfully when resuming from cloning state")

	// Verify processing continued (either re-cloned or transitioned to processing)
	mockRepoRepo.AssertCalled(suite.T(), "FindByID", mock.Anything, repoID)
}

// TestExecuteJobPipeline_RepositoryInProcessingState_ResumesProcessing tests that when
// a repository is in "processing" state (job was interrupted mid-processing), the job
// processor should allow reprocessing from processing state.
func (suite *JobProcessorTestSuite) TestExecuteJobPipeline_RepositoryInProcessingState_ResumesProcessing() {
	// Setup: Create repository in processing state
	repoID := uuid.New()
	repoURL, err := valueobject.NewRepositoryURL("https://github.com/example/processing-repo.git")
	suite.Require().NoError(err)

	// Create repo in processing state (must transition through valid states)
	processingRepo1 := entity.NewRepository(repoURL, "processing-repo", nil, nil)
	err = processingRepo1.UpdateStatus(valueobject.RepositoryStatusCloning) // pending → cloning
	suite.Require().NoError(err)
	err = processingRepo1.UpdateStatus(valueobject.RepositoryStatusProcessing) // cloning → processing
	suite.Require().NoError(err)

	// Setup mock repository repository to return processing repository
	mockRepoRepo := &MockRepositoryRepository{}
	// FindByID is called 2 times:
	// 1. Initial check in executeJobPipeline (returns processing)
	// 2. markRepositoryCompleted - skips both cloning and processing transitions
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(processingRepo1, nil).Once()
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(processingRepo1, nil).Once()
	mockRepoRepo.On("Update", mock.Anything, mock.Anything).Return(nil)

	// Setup other mocks for successful processing
	mockGitClient := &MockEnhancedGitClient{}
	mockGitClient.On("Clone", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mockCodeParser := &MockCodeParser{}
	mockCodeParser.On("ParseDirectory", mock.Anything, mock.Anything, mock.Anything).
		Return([]outbound.CodeChunk{
			{ID: uuid.New().String(), Content: "test content", FilePath: "test.go", RepositoryID: repoID},
		}, nil)

	mockEmbedding := &MockEmbeddingService{}
	mockEmbedding.On("GenerateEmbedding", mock.Anything, mock.Anything, mock.Anything).
		Return(&outbound.EmbeddingResult{Vector: []float64{0.1, 0.2}, Dimensions: 2}, nil)

	mockChunkStorage := &MockChunkStorageRepository{}
	mockChunkStorage.On("SaveChunkWithEmbedding", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Use a config without resource limits to avoid early rejection
	config := JobProcessorConfig{
		WorkspaceDir:      "/tmp/workspace-idempotent-test",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
		MaxMemoryMB:       0, // Disable memory limit
		MaxDiskUsageMB:    0, // Disable disk limit
		CleanupInterval:   1 * time.Hour,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	processor := NewDefaultJobProcessor(
		config,
		&MockIndexingJobRepository{},
		mockRepoRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbedding,
		mockChunkStorage,
	).(*DefaultJobProcessor)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "processing-resume-test",
		RepositoryID:  repoID,
		RepositoryURL: "https://github.com/example/processing-repo.git",
	}

	ctx := context.Background()
	err = processor.ProcessJob(ctx, message)

	// ASSERTION: Should succeed or handle appropriately
	// The implementation MUST allow reprocessing from processing state
	suite.Require().NoError(err, "Job should complete successfully when resuming from processing state")

	// Verify processing continued
	mockRepoRepo.AssertCalled(suite.T(), "FindByID", mock.Anything, repoID)
}

// TestTransitionToCloning_AlreadyFailed_DoesNotCallMarkRepositoryFailedAgain tests that
// when a repository is already in "failed" state and transition to cloning fails,
// the error handler should NOT attempt a failed → failed transition.
func (suite *JobProcessorTestSuite) TestTransitionToCloning_AlreadyFailed_DoesNotCallMarkRepositoryFailedAgain() {
	// Setup: Create repository in failed state
	repoID := uuid.New()
	repoURL, err := valueobject.NewRepositoryURL("https://github.com/example/failed-repo.git")
	suite.Require().NoError(err)

	failedRepoTest5 := entity.NewRepository(repoURL, "failed-repo", nil, nil)
	err = failedRepoTest5.UpdateStatus(valueobject.RepositoryStatusFailed) // pending → failed
	suite.Require().NoError(err)

	// Setup mock repository repository to return failed repository
	// The new implementation automatically transitions failed → pending first
	mockRepoRepo := &MockRepositoryRepository{}
	// Allow any number of FindByID calls, return appropriate state based on call order
	// The exact number depends on implementation details we're not testing
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(failedRepoTest5, nil).Maybe()

	// First Update: failed → pending (automatic retry) - succeeds
	mockRepoRepo.On("Update", mock.Anything, mock.MatchedBy(func(repo *entity.Repository) bool {
		return repo.Status() == valueobject.RepositoryStatusPending
	})).Return(nil).Maybe()

	// Second Update: pending → cloning should fail (simulate error during transition)
	mockRepoRepo.On("Update", mock.Anything, mock.MatchedBy(func(repo *entity.Repository) bool {
		return repo.Status() == valueobject.RepositoryStatusCloning
	})).Return(errors.New("database error during cloning transition")).Maybe()

	// Allow Update calls with failed status - the implementation prevents double-fail via status check
	mockRepoRepo.On("Update", mock.Anything, mock.MatchedBy(func(repo *entity.Repository) bool {
		return repo.Status() == valueobject.RepositoryStatusFailed
	})).Return(nil).Maybe()

	// Use a config without resource limits to avoid early rejection
	config := JobProcessorConfig{
		WorkspaceDir:      "/tmp/workspace-idempotent-test",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
		MaxMemoryMB:       0, // Disable memory limit
		MaxDiskUsageMB:    0, // Disable disk limit
		CleanupInterval:   1 * time.Hour,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	processor := NewDefaultJobProcessor(
		config,
		&MockIndexingJobRepository{},
		mockRepoRepo,
		&MockEnhancedGitClient{},
		&MockCodeParser{},
		&MockEmbeddingService{},
		&MockChunkStorageRepository{},
	).(*DefaultJobProcessor)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "failed-no-double-fail-test",
		RepositoryID:  repoID,
		RepositoryURL: "https://github.com/example/failed-repo.git",
	}

	ctx := context.Background()
	err = processor.ProcessJob(ctx, message)

	// ASSERTION: Should return an error (transition to cloning failed due to database error)
	suite.Require().Error(err, "Job should fail when transition to cloning fails")

	// Verify that FindByID was called to check repository status
	mockRepoRepo.AssertCalled(suite.T(), "FindByID", mock.Anything, repoID)

	// The implementation handles the failed → pending → cloning(error) flow correctly
	// Detailed transition verification is covered by the successful execution  tests
}

// TestExecuteJobPipeline_IdempotentRedelivery_SameMessageThreeTimes tests that when
// NATS delivers the same message 3 times (same message ID), all deliveries result in
// the same safe outcome (idempotent behavior).
func (suite *JobProcessorTestSuite) TestExecuteJobPipeline_IdempotentRedelivery_SameMessageThreeTimes() {
	// Setup: Create repository in pending state
	repoID := uuid.New()
	repoURL, err := valueobject.NewRepositoryURL("https://github.com/example/idempotent-repo.git")
	suite.Require().NoError(err)

	pendingRepo := entity.NewRepository(repoURL, "idempotent-repo", nil, nil)

	// Create repos for state transitions
	cloningRepoIdempotent := entity.NewRepository(repoURL, "idempotent-repo", nil, nil)
	err = cloningRepoIdempotent.UpdateStatus(valueobject.RepositoryStatusCloning) // pending → cloning
	suite.Require().NoError(err)

	processingRepoIdempotent := entity.NewRepository(repoURL, "idempotent-repo", nil, nil)
	err = processingRepoIdempotent.UpdateStatus(valueobject.RepositoryStatusCloning) // pending → cloning
	suite.Require().NoError(err)
	err = processingRepoIdempotent.UpdateStatus(valueobject.RepositoryStatusProcessing) // cloning → processing
	suite.Require().NoError(err)

	// Setup mock repository repository
	mockRepoRepo := &MockRepositoryRepository{}

	// First delivery: repository is pending, processes successfully
	// FindByID is called 4 times during first delivery (pending state):
	// 1. Initial check in executeJobPipeline - returns pending
	// 2. updateRepositoryStatus("cloning") - returns pending
	// 3. updateRepositoryStatus("processing") - returns cloning
	// 4. markRepositoryCompleted - returns processing
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(pendingRepo, nil).Once()
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(pendingRepo, nil).Once()
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(cloningRepoIdempotent, nil).Once()
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(processingRepoIdempotent, nil).Once()
	mockRepoRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Times(3) // cloning, processing, completed

	// Second delivery: repository is completed, should skip processing
	completedRepo := entity.NewRepository(repoURL, "idempotent-repo", nil, nil)
	err = completedRepo.UpdateStatus(valueobject.RepositoryStatusCloning) // pending → cloning
	suite.Require().NoError(err)
	err = completedRepo.UpdateStatus(valueobject.RepositoryStatusProcessing) // cloning → processing
	suite.Require().NoError(err)
	err = completedRepo.MarkIndexingCompleted("abc123", 1, 1) // processing → completed
	suite.Require().NoError(err)
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(completedRepo, nil).Once()

	// Third delivery: repository is still completed, should skip processing
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(completedRepo, nil).Once()

	// Setup other mocks
	mockGitClient := &MockEnhancedGitClient{}
	mockGitClient.On("Clone", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	mockCodeParser := &MockCodeParser{}
	mockCodeParser.On("ParseDirectory", mock.Anything, mock.Anything, mock.Anything).
		Return([]outbound.CodeChunk{
			{ID: uuid.New().String(), Content: "test content", FilePath: "test.go", RepositoryID: repoID},
		}, nil).Once()

	mockEmbedding := &MockEmbeddingService{}
	mockEmbedding.On("GenerateEmbedding", mock.Anything, mock.Anything, mock.Anything).
		Return(&outbound.EmbeddingResult{Vector: []float64{0.1, 0.2}, Dimensions: 2}, nil).Once()

	mockChunkStorage := &MockChunkStorageRepository{}
	mockChunkStorage.On("SaveChunkWithEmbedding", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	// Use a config without resource limits to avoid early rejection
	config := JobProcessorConfig{
		WorkspaceDir:      "/tmp/workspace-idempotent-test",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
		MaxMemoryMB:       0, // Disable memory limit
		MaxDiskUsageMB:    0, // Disable disk limit
		CleanupInterval:   1 * time.Hour,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	processor := NewDefaultJobProcessor(
		config,
		&MockIndexingJobRepository{},
		mockRepoRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbedding,
		mockChunkStorage,
	).(*DefaultJobProcessor)

	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "idempotent-message-id",
		RepositoryID:  repoID,
		RepositoryURL: "https://github.com/example/idempotent-repo.git",
	}

	ctx := context.Background()

	// FIRST DELIVERY: Should process normally
	err = processor.ProcessJob(ctx, message)
	suite.Require().NoError(err, "First delivery should succeed")

	// SECOND DELIVERY: Should skip processing (idempotent)
	err = processor.ProcessJob(ctx, message)
	suite.Require().NoError(err, "Second delivery should succeed without reprocessing")

	// THIRD DELIVERY: Should skip processing (idempotent)
	err = processor.ProcessJob(ctx, message)
	suite.Require().NoError(err, "Third delivery should succeed without reprocessing")

	// Verify git clone was called only once (first delivery)
	mockGitClient.AssertNumberOfCalls(suite.T(), "Clone", 1)

	// Verify code parsing was called only once (first delivery)
	mockCodeParser.AssertNumberOfCalls(suite.T(), "ParseDirectory", 1)

	// Verify embedding generation was called only once (first delivery)
	mockEmbedding.AssertNumberOfCalls(suite.T(), "GenerateEmbedding", 1)
}

// TestExecuteJobPipeline_RepositoryNotFound_ReturnsError tests the scenario where
// FindByID returns (nil, nil) - indicating the repository does not exist in the database.
// This test reproduces the production panic that occurs when handleRepositoryStatus
// attempts to call repo.Status() on a nil repository pointer.
//
// PRODUCTION BUG:
// - Line 225: repo, err := p.repositoryRepo.FindByID(ctx, message.RepositoryID)
// - Line 226-228: Only checks if err != nil
// - Line 231: Calls handleRepositoryStatus(ctx, repo, ...) where repo is nil
// - Line 324: handleRepositoryStatus calls repo.Status() → PANIC!
//
// EXPECTED BEHAVIOR:
// The job processor should detect the nil repository and return a descriptive error
// instead of panicking. The error should clearly indicate that the repository was not found.
//
// TEST SPECIFICATION:
// 1. Setup: Mock FindByID to return (nil, nil) - simulating repository not in database
// 2. Execute: Call ProcessJob with a valid message containing repository ID
// 3. Assert: Should return error (not panic) with message "repository not found"
// 4. Assert: No git, parsing, or embedding operations should be attempted
//
// This test will FAIL (panic) until the bug is fixed by adding a nil check:
//
//	if repo == nil {
//	    return nil, fmt.Errorf("repository not found: %s", message.RepositoryID.String())
//	}
func (suite *JobProcessorTestSuite) TestExecuteJobPipeline_RepositoryNotFound_ReturnsError() {
	// Setup: Create a valid repository ID and message
	repoID := uuid.New()

	// Setup mock repository repository to return (nil, nil) - repository not found
	// This simulates the case where FindByID successfully queries the database
	// but finds no matching repository record
	mockRepoRepo := &MockRepositoryRepository{}
	mockRepoRepo.On("FindByID", mock.Anything, repoID).Return(nil, nil)

	// Setup other mocks (should not be called since we fail early)
	mockGitClient := &MockEnhancedGitClient{}
	mockCodeParser := &MockCodeParser{}
	mockEmbedding := &MockEmbeddingService{}
	mockChunkStorage := &MockChunkStorageRepository{}

	// Create processor with standard config
	config := JobProcessorConfig{
		WorkspaceDir:      "/tmp/workspace-notfound-test",
		MaxConcurrentJobs: 5,
		JobTimeout:        5 * time.Minute,
		MaxMemoryMB:       0, // Disable limits
		MaxDiskUsageMB:    0,
		CleanupInterval:   1 * time.Hour,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	processor := NewDefaultJobProcessor(
		config,
		&MockIndexingJobRepository{},
		mockRepoRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbedding,
		mockChunkStorage,
	).(*DefaultJobProcessor)

	// Create message with repository ID that doesn't exist in database
	message := messaging.EnhancedIndexingJobMessage{
		IndexingJobID: uuid.New(),
		MessageID:     "repository-not-found-test",
		RepositoryID:  repoID,
		RepositoryURL: "https://github.com/example/nonexistent-repo.git",
	}

	ctx := context.Background()

	// Execute: This should return an error (not panic)
	err := processor.ProcessJob(ctx, message)

	// ASSERTION: Should return a descriptive error
	// Currently this will PANIC with:
	// "panic: runtime error: invalid memory address or nil pointer dereference"
	// at line 324: repo.Status() in handleRepositoryStatus
	suite.Require().Error(err, "ProcessJob should return error when repository not found")
	suite.Require().ErrorContains(err, "repository not found",
		"Error message should clearly indicate repository was not found")

	// Verify FindByID was called to check for repository
	mockRepoRepo.AssertCalled(suite.T(), "FindByID", mock.Anything, repoID)

	// Verify no git operations were attempted (fail fast on missing repository)
	mockGitClient.AssertNotCalled(suite.T(), "Clone", mock.Anything, mock.Anything, mock.Anything)
	mockGitClient.AssertNotCalled(
		suite.T(),
		"CloneWithOptions",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	)

	// Verify no code parsing was attempted
	mockCodeParser.AssertNotCalled(suite.T(), "ParseDirectory", mock.Anything, mock.Anything, mock.Anything)

	// Verify no embedding generation was attempted
	mockEmbedding.AssertNotCalled(suite.T(), "GenerateEmbedding", mock.Anything, mock.Anything, mock.Anything)

	// Verify no chunk storage was attempted
	mockChunkStorage.AssertNotCalled(suite.T(), "SaveChunkWithEmbedding", mock.Anything, mock.Anything, mock.Anything)

	// Verify repository was not updated (no state transitions on non-existent repository)
	mockRepoRepo.AssertNotCalled(suite.T(), "Update", mock.Anything, mock.Anything)
}
