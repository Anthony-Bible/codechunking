package gemini

import (
	"codechunking/internal/port/outbound"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBatchEmbeddingClient(t *testing.T) {
	t.Run("success with provided directories", func(t *testing.T) {
		// Create temporary directories
		tempDir := t.TempDir()
		inputDir := filepath.Join(tempDir, "input")
		outputDir := filepath.Join(tempDir, "output")

		// Create base client
		baseClient, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		// Create batch client
		batchClient, err := NewBatchEmbeddingClient(baseClient, inputDir, outputDir)
		require.NoError(t, err)
		assert.NotNil(t, batchClient)
		assert.Equal(t, inputDir, batchClient.inputDir)
		assert.Equal(t, outputDir, batchClient.outputDir)

		// Verify directories were created
		assert.DirExists(t, inputDir)
		assert.DirExists(t, outputDir)
	})

	t.Run("success with default directories", func(t *testing.T) {
		// Create base client
		baseClient, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		// Create batch client with empty directories (should use defaults)
		batchClient, err := NewBatchEmbeddingClient(baseClient, "", "")
		require.NoError(t, err)
		assert.NotNil(t, batchClient)
		assert.NotEmpty(t, batchClient.inputDir)
		assert.NotEmpty(t, batchClient.outputDir)
	})

	t.Run("error when base client is nil", func(t *testing.T) {
		batchClient, err := NewBatchEmbeddingClient(nil, "", "")
		assert.Error(t, err)
		assert.Nil(t, batchClient)
		assert.Contains(t, err.Error(), "base client cannot be nil")
	})
}

func TestBatchEmbeddingClient_CreateBatchEmbeddingJob(t *testing.T) {
	t.Run("error with empty texts", func(t *testing.T) {
		// Create base client
		baseClient, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		// Create batch client
		tempDir := t.TempDir()
		batchClient, err := NewBatchEmbeddingClient(baseClient,
			filepath.Join(tempDir, "input"),
			filepath.Join(tempDir, "output"))
		require.NoError(t, err)

		// Try to create job with empty texts
		ctx := context.Background()
		job, err := batchClient.CreateBatchEmbeddingJob(ctx, []string{}, outbound.EmbeddingOptions{})

		assert.Error(t, err)
		assert.Nil(t, job)

		// Check error type
		embErr, ok := err.(*outbound.EmbeddingError)
		assert.True(t, ok)
		assert.Equal(t, "empty_texts", embErr.Code)
		assert.Equal(t, "validation", embErr.Type)
		assert.False(t, embErr.Retryable)
	})
}

func TestBatchEmbeddingClient_GetBatchJobStatus(t *testing.T) {
	t.Run("error with empty job ID", func(t *testing.T) {
		// Create base client
		baseClient, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		// Create batch client
		tempDir := t.TempDir()
		batchClient, err := NewBatchEmbeddingClient(baseClient,
			filepath.Join(tempDir, "input"),
			filepath.Join(tempDir, "output"))
		require.NoError(t, err)

		// Try to get status with empty job ID
		ctx := context.Background()
		job, err := batchClient.GetBatchJobStatus(ctx, "")

		assert.Error(t, err)
		assert.Nil(t, job)

		// Check error type
		embErr, ok := err.(*outbound.EmbeddingError)
		assert.True(t, ok)
		assert.Equal(t, "empty_job_id", embErr.Code)
		assert.Equal(t, "validation", embErr.Type)
		assert.False(t, embErr.Retryable)
	})
}

func TestBatchEmbeddingClient_CancelBatchJob(t *testing.T) {
	t.Run("error with empty job ID", func(t *testing.T) {
		// Create base client
		baseClient, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		// Create batch client
		tempDir := t.TempDir()
		batchClient, err := NewBatchEmbeddingClient(baseClient,
			filepath.Join(tempDir, "input"),
			filepath.Join(tempDir, "output"))
		require.NoError(t, err)

		// Try to cancel with empty job ID
		ctx := context.Background()
		err = batchClient.CancelBatchJob(ctx, "")

		assert.Error(t, err)

		// Check error type
		embErr, ok := err.(*outbound.EmbeddingError)
		assert.True(t, ok)
		assert.Equal(t, "empty_job_id", embErr.Code)
		assert.Equal(t, "validation", embErr.Type)
		assert.False(t, embErr.Retryable)
	})
}

func TestBatchEmbeddingClient_DeleteBatchJob(t *testing.T) {
	t.Run("error with empty job ID", func(t *testing.T) {
		// Create base client
		baseClient, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		// Create batch client
		tempDir := t.TempDir()
		batchClient, err := NewBatchEmbeddingClient(baseClient,
			filepath.Join(tempDir, "input"),
			filepath.Join(tempDir, "output"))
		require.NoError(t, err)

		// Try to delete with empty job ID
		ctx := context.Background()
		err = batchClient.DeleteBatchJob(ctx, "")

		assert.Error(t, err)

		// Check error type
		embErr, ok := err.(*outbound.EmbeddingError)
		assert.True(t, ok)
		assert.Equal(t, "empty_job_id", embErr.Code)
		assert.Equal(t, "validation", embErr.Type)
		assert.False(t, embErr.Retryable)
	})
}

func TestBatchEmbeddingClient_writeRequestsToFile(t *testing.T) {
	t.Run("success writing requests to file", func(t *testing.T) {
		// Create base client
		baseClient, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		// Create batch client
		tempDir := t.TempDir()
		batchClient, err := NewBatchEmbeddingClient(baseClient,
			filepath.Join(tempDir, "input"),
			filepath.Join(tempDir, "output"))
		require.NoError(t, err)

		// Create test requests
		requests := []*outbound.BatchEmbeddingRequest{
			{RequestID: "req_1", Text: "Hello world"},
			{RequestID: "req_2", Text: "Test embedding"},
		}

		// Write requests to file
		ctx := context.Background()
		filepath, err := batchClient.writeRequestsToFile(ctx, requests, outbound.EmbeddingOptions{})

		require.NoError(t, err)
		assert.NotEmpty(t, filepath)
		assert.FileExists(t, filepath)

		// Verify file content
		content, err := os.ReadFile(filepath)
		require.NoError(t, err)
		assert.NotEmpty(t, content)
		assert.Contains(t, string(content), "req_1")
		assert.Contains(t, string(content), "Hello world")
		assert.Contains(t, string(content), "req_2")
		assert.Contains(t, string(content), "Test embedding")
	})
}

func TestBatchJobState_IsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		state    outbound.BatchJobState
		expected bool
	}{
		{"pending is not terminal", outbound.BatchJobStatePending, false},
		{"processing is not terminal", outbound.BatchJobStateProcessing, false},
		{"completed is terminal", outbound.BatchJobStateCompleted, true},
		{"failed is terminal", outbound.BatchJobStateFailed, true},
		{"cancelled is terminal", outbound.BatchJobStateCancelled, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.IsTerminal())
		})
	}
}

func TestBatchEmbeddingClient_matchesFilter(t *testing.T) {
	// Create base client
	baseClient, err := NewClient(&ClientConfig{
		APIKey: "test-api-key",
		Model:  DefaultModel,
	})
	require.NoError(t, err)

	// Create batch client
	tempDir := t.TempDir()
	batchClient, err := NewBatchEmbeddingClient(baseClient,
		filepath.Join(tempDir, "input"),
		filepath.Join(tempDir, "output"))
	require.NoError(t, err)

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	job := &outbound.BatchEmbeddingJob{
		JobID:     "test-job",
		Model:     DefaultModel,
		State:     outbound.BatchJobStateCompleted,
		CreatedAt: now,
	}

	t.Run("matches when filter is nil", func(t *testing.T) {
		assert.True(t, batchClient.matchesFilter(job, &outbound.BatchJobFilter{}))
	})

	t.Run("matches state filter", func(t *testing.T) {
		filter := &outbound.BatchJobFilter{
			States: []outbound.BatchJobState{outbound.BatchJobStateCompleted},
		}
		assert.True(t, batchClient.matchesFilter(job, filter))
	})

	t.Run("does not match state filter", func(t *testing.T) {
		filter := &outbound.BatchJobFilter{
			States: []outbound.BatchJobState{outbound.BatchJobStatePending},
		}
		assert.False(t, batchClient.matchesFilter(job, filter))
	})

	t.Run("matches model filter", func(t *testing.T) {
		filter := &outbound.BatchJobFilter{
			Model: DefaultModel,
		}
		assert.True(t, batchClient.matchesFilter(job, filter))
	})

	t.Run("does not match model filter", func(t *testing.T) {
		filter := &outbound.BatchJobFilter{
			Model: "different-model",
		}
		assert.False(t, batchClient.matchesFilter(job, filter))
	})

	t.Run("matches created after filter", func(t *testing.T) {
		filter := &outbound.BatchJobFilter{
			CreatedAfter: &yesterday,
		}
		assert.True(t, batchClient.matchesFilter(job, filter))
	})

	t.Run("does not match created after filter", func(t *testing.T) {
		filter := &outbound.BatchJobFilter{
			CreatedAfter: &tomorrow,
		}
		assert.False(t, batchClient.matchesFilter(job, filter))
	})

	t.Run("matches created before filter", func(t *testing.T) {
		filter := &outbound.BatchJobFilter{
			CreatedBefore: &tomorrow,
		}
		assert.True(t, batchClient.matchesFilter(job, filter))
	})

	t.Run("does not match created before filter", func(t *testing.T) {
		filter := &outbound.BatchJobFilter{
			CreatedBefore: &yesterday,
		}
		assert.False(t, batchClient.matchesFilter(job, filter))
	})
}

func TestBatchEmbeddingClient_WaitForBatchJob(t *testing.T) {
	t.Run("context cancellation", func(t *testing.T) {
		// Create base client
		baseClient, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		// Create batch client
		tempDir := t.TempDir()
		batchClient, err := NewBatchEmbeddingClient(baseClient,
			filepath.Join(tempDir, "input"),
			filepath.Join(tempDir, "output"))
		require.NoError(t, err)

		// Create context with immediate cancellation
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Try to wait for job
		job, err := batchClient.WaitForBatchJob(ctx, "test-job", 100*time.Millisecond)

		assert.Error(t, err)
		assert.Nil(t, job)

		// Check error type
		embErr, ok := err.(*outbound.EmbeddingError)
		assert.True(t, ok)
		assert.Equal(t, "context_cancelled", embErr.Code)
		assert.Equal(t, "timeout", embErr.Type)
		assert.False(t, embErr.Retryable)
	})
}
