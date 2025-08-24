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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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
		return nil, args.Int(1), args.Error(2)
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
		return nil, args.Int(1), args.Error(2)
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

// MockGitClient mocks the git client interface.
type MockGitClient struct {
	mock.Mock
}

func (m *MockGitClient) Clone(ctx context.Context, repoURL, targetPath string) error {
	args := m.Called(ctx, repoURL, targetPath)
	return args.Error(0)
}

func (m *MockGitClient) GetCommitHash(ctx context.Context, repoPath string) (string, error) {
	args := m.Called(ctx, repoPath)
	return args.String(0), args.Error(1)
}

func (m *MockGitClient) GetBranch(ctx context.Context, repoPath string) (string, error) {
	args := m.Called(ctx, repoPath)
	return args.String(0), args.Error(1)
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

// MockEmbeddingGenerator mocks the embedding generator interface.
type MockEmbeddingGenerator struct {
	mock.Mock
}

func (m *MockEmbeddingGenerator) GenerateEmbedding(ctx context.Context, text string) ([]float64, error) {
	args := m.Called(ctx, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float64), args.Error(1)
}

// CodeParsingConfig holds configuration for code parsing.
type CodeParsingConfig struct {
	ChunkSizeBytes   int
	MaxFileSizeBytes int64
	FileFilters      []string
	IncludeTests     bool
	ExcludeVendor    bool
}

// CodeChunk represents a parsed code chunk.
type CodeChunk struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"file_path"`
	StartLine int       `json:"start_line"`
	EndLine   int       `json:"end_line"`
	Content   string    `json:"content"`
	Language  string    `json:"language"`
	Size      int       `json:"size"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
}

// GitClient defines the interface for git operations.
type GitClient interface {
	Clone(ctx context.Context, repoURL, targetPath string) error
	GetCommitHash(ctx context.Context, repoPath string) (string, error)
	GetBranch(ctx context.Context, repoPath string) (string, error)
}

// CodeParser defines the interface for parsing code.
type CodeParser interface {
	ParseDirectory(ctx context.Context, dirPath string, config CodeParsingConfig) ([]CodeChunk, error)
}

// EmbeddingGenerator defines the interface for generating embeddings.
type EmbeddingGenerator interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float64, error)
}

// TestJobProcessorCreation tests job processor creation and configuration.
func TestJobProcessorCreation(t *testing.T) {
	t.Run("should create job processor with valid configuration", func(t *testing.T) {
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

		mockIndexingJobRepo := &MockIndexingJobRepository{}
		mockRepositoryRepo := &MockRepositoryRepository{}
		mockGitClient := &MockGitClient{}
		mockCodeParser := &MockCodeParser{}
		mockEmbeddingGenerator := &MockEmbeddingGenerator{}

		processor := NewDefaultJobProcessor(
			config,
			mockIndexingJobRepo,
			mockRepositoryRepo,
			mockGitClient,
			mockCodeParser,
			mockEmbeddingGenerator,
		)

		require.NotNil(t, processor)

		// Health status should be empty in RED phase
		health := processor.GetHealthStatus()
		assert.False(t, health.IsReady)
		assert.Equal(t, 0, health.ActiveJobs)
	})

	t.Run("should fail with invalid workspace directory", func(t *testing.T) {
		config := JobProcessorConfig{
			WorkspaceDir:      "", // Invalid empty workspace
			MaxConcurrentJobs: 5,
			JobTimeout:        5 * time.Minute,
		}

		mockIndexingJobRepo := &MockIndexingJobRepository{}
		mockRepositoryRepo := &MockRepositoryRepository{}
		mockGitClient := &MockGitClient{}
		mockCodeParser := &MockCodeParser{}
		mockEmbeddingGenerator := &MockEmbeddingGenerator{}

		// In a real implementation, this should validate the config
		processor := NewDefaultJobProcessor(
			config,
			mockIndexingJobRepo,
			mockRepositoryRepo,
			mockGitClient,
			mockCodeParser,
			mockEmbeddingGenerator,
		)

		// For now, processor is created but should fail validation later
		require.NotNil(t, processor)
	})

	t.Run("should fail with nil dependencies", func(t *testing.T) {
		config := JobProcessorConfig{
			WorkspaceDir:      "/tmp/workspace",
			MaxConcurrentJobs: 5,
			JobTimeout:        5 * time.Minute,
		}

		// In a real implementation, this should panic or return error
		processor := NewDefaultJobProcessor(
			config,
			nil, // nil dependency
			nil,
			nil,
			nil,
			nil,
		)

		// For RED phase, processor is created but will fail on use
		require.NotNil(t, processor)
	})
}

// TestJobExecution tests job execution with repository status updates.
func TestJobExecution(t *testing.T) {
	t.Run("should execute job and update status to running", func(t *testing.T) {
		// Create enhanced job message
		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "test-msg-123",
			CorrelationID: "test-corr-456",
			SchemaVersion: "2.0",
			Timestamp:     time.Now(),
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
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

		config := JobProcessorConfig{
			WorkspaceDir:      "/tmp/workspace",
			MaxConcurrentJobs: 5,
			JobTimeout:        5 * time.Minute,
		}

		mockIndexingJobRepo := &MockIndexingJobRepository{}
		mockRepositoryRepo := &MockRepositoryRepository{}
		mockGitClient := &MockGitClient{}
		mockCodeParser := &MockCodeParser{}
		mockEmbeddingGenerator := &MockEmbeddingGenerator{}

		// Mock expectations for job status updates
		mockIndexingJobRepo.On("GetByID", mock.Anything, mock.AnythingOfType("uuid.UUID")).
			Return(nil, errors.New("job not found"))
		mockIndexingJobRepo.On("Create", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(nil)
		mockIndexingJobRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).Return(nil)

		processor := NewDefaultJobProcessor(
			config,
			mockIndexingJobRepo,
			mockRepositoryRepo,
			mockGitClient,
			mockCodeParser,
			mockEmbeddingGenerator,
		)

		ctx := context.Background()
		err := processor.ProcessJob(ctx, message)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should handle job timeout", func(t *testing.T) {
		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "test-msg-timeout",
			CorrelationID: "test-corr-timeout",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/slow-repo.git",
			ProcessingContext: messaging.ProcessingContext{
				TimeoutSeconds: 1, // Very short timeout
			},
		}

		config := JobProcessorConfig{
			WorkspaceDir: "/tmp/workspace",
			JobTimeout:   1 * time.Second,
		}

		mockIndexingJobRepo := &MockIndexingJobRepository{}
		processor := NewDefaultJobProcessor(
			config,
			mockIndexingJobRepo,
			nil,
			nil,
			nil,
			nil,
		)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := processor.ProcessJob(ctx, message)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should handle context cancellation", func(t *testing.T) {
		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "test-msg-cancel",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
		}

		config := JobProcessorConfig{
			WorkspaceDir: "/tmp/workspace",
		}

		processor := NewDefaultJobProcessor(
			config,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := processor.ProcessJob(ctx, message)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})
}

// TestProgressTracking tests progress tracking and metrics collection.
func TestProgressTracking(t *testing.T) {
	t.Run("should track job processing progress", func(t *testing.T) {
		config := JobProcessorConfig{
			WorkspaceDir: "/tmp/workspace",
		}

		processor := NewDefaultJobProcessor(
			config,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		// Should return empty metrics in RED phase
		metrics := processor.GetMetrics()
		assert.Equal(t, int64(0), metrics.TotalJobsProcessed)
		assert.Equal(t, int64(0), metrics.FilesProcessed)
		assert.Equal(t, int64(0), metrics.ChunksGenerated)
	})

	t.Run("should update metrics after job completion", func(t *testing.T) {
		config := JobProcessorConfig{
			WorkspaceDir: "/tmp/workspace",
		}

		processor := NewDefaultJobProcessor(
			config,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		// Process a job first
		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "test-msg-metrics",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
		}

		ctx := context.Background()
		err := processor.ProcessJob(ctx, message)

		// Should fail in RED phase
		require.Error(t, err)

		// Metrics should still be empty in RED phase
		metrics := processor.GetMetrics()
		assert.Equal(t, int64(0), metrics.TotalJobsProcessed)
	})

	t.Run("should calculate average processing time", func(t *testing.T) {
		config := JobProcessorConfig{
			WorkspaceDir: "/tmp/workspace",
		}

		processor := NewDefaultJobProcessor(
			config,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		// Should return zero average in RED phase
		metrics := processor.GetMetrics()
		assert.Equal(t, time.Duration(0), metrics.AverageProcessingTime)
	})
}

// TestErrorHandling tests error handling and job failure scenarios.
func TestErrorHandling(t *testing.T) {
	t.Run("should handle git clone failure", func(t *testing.T) {
		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "test-msg-git-fail",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/nonexistent/repo.git",
		}

		config := JobProcessorConfig{
			WorkspaceDir: "/tmp/workspace",
		}

		mockIndexingJobRepo := &MockIndexingJobRepository{}
		mockGitClient := &MockGitClient{}

		// Mock git clone failure
		mockGitClient.On("Clone", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).
			Return(errors.New("repository not found"))

		processor := NewDefaultJobProcessor(
			config,
			mockIndexingJobRepo,
			nil,
			mockGitClient,
			nil,
			nil,
		)

		ctx := context.Background()
		err := processor.ProcessJob(ctx, message)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should handle code parsing failure", func(t *testing.T) {
		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "test-msg-parse-fail",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
		}

		config := JobProcessorConfig{
			WorkspaceDir: "/tmp/workspace",
		}

		mockCodeParser := &MockCodeParser{}

		// Mock parsing failure
		mockCodeParser.On("ParseDirectory", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("CodeParsingConfig")).
			Return(nil, errors.New("parsing failed"))

		processor := NewDefaultJobProcessor(
			config,
			nil,
			nil,
			nil,
			mockCodeParser,
			nil,
		)

		ctx := context.Background()
		err := processor.ProcessJob(ctx, message)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should handle embedding generation failure", func(t *testing.T) {
		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "test-msg-embed-fail",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
		}

		config := JobProcessorConfig{
			WorkspaceDir: "/tmp/workspace",
		}

		mockEmbeddingGenerator := &MockEmbeddingGenerator{}

		// Mock embedding failure
		mockEmbeddingGenerator.On("GenerateEmbedding", mock.Anything, mock.AnythingOfType("string")).
			Return(nil, errors.New("embedding generation failed"))

		processor := NewDefaultJobProcessor(
			config,
			nil,
			nil,
			nil,
			nil,
			mockEmbeddingGenerator,
		)

		ctx := context.Background()
		err := processor.ProcessJob(ctx, message)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should handle database update failure", func(t *testing.T) {
		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "test-msg-db-fail",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
		}

		config := JobProcessorConfig{
			WorkspaceDir: "/tmp/workspace",
		}

		mockIndexingJobRepo := &MockIndexingJobRepository{}

		// Mock database failure
		mockIndexingJobRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.IndexingJob")).
			Return(errors.New("database connection failed"))

		processor := NewDefaultJobProcessor(
			config,
			mockIndexingJobRepo,
			nil,
			nil,
			nil,
			nil,
		)

		ctx := context.Background()
		err := processor.ProcessJob(ctx, message)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})
}

// TestResourceCleanup tests resource cleanup and memory management.
func TestResourceCleanup(t *testing.T) {
	t.Run("should cleanup workspace after job completion", func(t *testing.T) {
		config := JobProcessorConfig{
			WorkspaceDir:    "/tmp/workspace",
			CleanupInterval: 1 * time.Minute,
		}

		processor := NewDefaultJobProcessor(
			config,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		err := processor.Cleanup()

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should handle cleanup failure gracefully", func(t *testing.T) {
		config := JobProcessorConfig{
			WorkspaceDir: "/invalid/path",
		}

		processor := NewDefaultJobProcessor(
			config,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		err := processor.Cleanup()

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should monitor resource usage", func(t *testing.T) {
		config := JobProcessorConfig{
			WorkspaceDir:   "/tmp/workspace",
			MaxMemoryMB:    1024,
			MaxDiskUsageMB: 10240,
		}

		processor := NewDefaultJobProcessor(
			config,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		// Should return empty resource usage in RED phase
		health := processor.GetHealthStatus()
		assert.Equal(t, 0, health.ResourceUsage.MemoryMB)
		assert.Equal(t, float64(0), health.ResourceUsage.CPUPercent)
		assert.Equal(t, int64(0), health.ResourceUsage.DiskUsageMB)
	})
}

// TestConcurrentJobProcessing tests concurrent job processing capabilities.
func TestConcurrentJobProcessing(t *testing.T) {
	t.Run("should handle multiple concurrent jobs", func(t *testing.T) {
		config := JobProcessorConfig{
			WorkspaceDir:      "/tmp/workspace",
			MaxConcurrentJobs: 3,
		}

		processor := NewDefaultJobProcessor(
			config,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		// Create multiple job messages
		messages := []messaging.EnhancedIndexingJobMessage{
			{
				MessageID:     "job-1",
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/example/repo1.git",
			},
			{
				MessageID:     "job-2",
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/example/repo2.git",
			},
			{
				MessageID:     "job-3",
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/example/repo3.git",
			},
		}

		ctx := context.Background()

		// Process jobs concurrently
		for _, msg := range messages {
			err := processor.ProcessJob(ctx, msg)

			// Should fail in RED phase
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not implemented yet")
		}

		// Health status should show no active jobs in RED phase
		health := processor.GetHealthStatus()
		assert.Equal(t, 0, health.ActiveJobs)
	})

	t.Run("should limit concurrent job execution", func(t *testing.T) {
		config := JobProcessorConfig{
			WorkspaceDir:      "/tmp/workspace",
			MaxConcurrentJobs: 1, // Only allow one concurrent job
		}

		processor := NewDefaultJobProcessor(
			config,
			nil,
			nil,
			nil,
			nil,
			nil,
		)

		ctx := context.Background()

		// Try to process multiple jobs
		msg1 := messaging.EnhancedIndexingJobMessage{
			MessageID:     "job-limit-1",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo1.git",
		}

		msg2 := messaging.EnhancedIndexingJobMessage{
			MessageID:     "job-limit-2",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo2.git",
		}

		err1 := processor.ProcessJob(ctx, msg1)
		err2 := processor.ProcessJob(ctx, msg2)

		// Should fail in RED phase
		require.Error(t, err1)
		require.Error(t, err2)
		assert.Contains(t, err1.Error(), "not implemented yet")
		assert.Contains(t, err2.Error(), "not implemented yet")
	})
}
