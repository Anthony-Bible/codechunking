package worker

import (
	"codechunking/internal/config"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewDefaultJobProcessor_WithBatchProgressRepo tests that the constructor
// properly initializes batchProgressRepo when passed as optional parameter.
func TestNewDefaultJobProcessor_WithBatchProgressRepo(t *testing.T) {
	// Arrange
	mockIndexingJobRepo := &MockIndexingJobRepository{}
	mockRepoRepo := &MockRepositoryRepository{}
	mockGitClient := &MockEnhancedGitClient{}
	mockCodeParser := &MockCodeParser{}
	mockEmbeddingService := &MockEmbeddingService{}
	mockChunkStorage := &MockChunkStorageRepository{}
	mockBatchQueueManager := &MockBatchQueueManager{}
	mockBatchProgressRepo := &MockBatchProgressRepository{}

	batchConfig := config.BatchProcessingConfig{
		UseTestEmbeddings: false,
		ThresholdChunks:   10,
	}

	jobConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test",
		MaxConcurrentJobs: 5,
		JobTimeout:        300 * time.Second,
		MaxMemoryMB:       1024,
		MaxDiskUsageMB:    10240,
		CleanupInterval:   60 * time.Second,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	// Act - Pass all optional parameters via options struct
	processor := NewDefaultJobProcessor(
		jobConfig,
		mockIndexingJobRepo,
		mockRepoRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbeddingService,
		mockChunkStorage,
		&JobProcessorBatchOptions{
			BatchConfig:       &batchConfig,
			BatchQueueManager: mockBatchQueueManager,
			BatchProgressRepo: mockBatchProgressRepo,
		},
	)

	// Assert
	assert.NotNil(t, processor, "processor should not be nil")

	// Access the private field through type assertion
	defaultProcessor, ok := processor.(*DefaultJobProcessor)
	assert.True(t, ok, "processor should be of type *DefaultJobProcessor")
	assert.NotNil(t, defaultProcessor, "defaultProcessor should not be nil")

	// Verify that batchProgressRepo is properly set (will fail if nil)
	assert.NotNil(
		t,
		defaultProcessor.batchProgressRepo,
		"batchProgressRepo should not be nil when passed as parameter",
	)
	assert.Same(
		t,
		mockBatchProgressRepo,
		defaultProcessor.batchProgressRepo,
		"batchProgressRepo should match the one passed to constructor",
	)
}

// TestNewDefaultJobProcessor_WithoutBatchProgressRepo tests that when
// batchProgressRepo is NOT passed, it should be nil (current buggy behavior).
// This test documents the bug that causes the panic.
func TestNewDefaultJobProcessor_WithoutBatchProgressRepo(t *testing.T) {
	// Arrange
	mockIndexingJobRepo := &MockIndexingJobRepository{}
	mockRepoRepo := &MockRepositoryRepository{}
	mockGitClient := &MockEnhancedGitClient{}
	mockCodeParser := &MockCodeParser{}
	mockEmbeddingService := &MockEmbeddingService{}
	mockChunkStorage := &MockChunkStorageRepository{}
	mockBatchQueueManager := &MockBatchQueueManager{}

	batchConfig := config.BatchProcessingConfig{
		UseTestEmbeddings: false,
		ThresholdChunks:   10,
	}

	jobConfig := JobProcessorConfig{
		WorkspaceDir:      "/tmp/test",
		MaxConcurrentJobs: 5,
		JobTimeout:        300 * time.Second,
		MaxMemoryMB:       1024,
		MaxDiskUsageMB:    10240,
		CleanupInterval:   60 * time.Second,
		RetryAttempts:     3,
		RetryBackoff:      5 * time.Second,
	}

	// Act - Pass only two optional parameters (missing batchProgressRepo)
	processor := NewDefaultJobProcessor(
		jobConfig,
		mockIndexingJobRepo,
		mockRepoRepo,
		mockGitClient,
		mockCodeParser,
		mockEmbeddingService,
		mockChunkStorage,
		&JobProcessorBatchOptions{
			BatchConfig:       &batchConfig,
			BatchQueueManager: mockBatchQueueManager,
			// BatchProgressRepo intentionally omitted (nil)
		},
	)

	// Assert
	defaultProcessor := processor.(*DefaultJobProcessor)

	// This assertion should currently PASS (demonstrating the bug)
	// because batchProgressRepo is nil when not passed
	assert.Nil(
		t,
		defaultProcessor.batchProgressRepo,
		"batchProgressRepo is nil when not passed - this is the BUG we're fixing",
	)
}
