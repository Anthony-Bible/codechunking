package gemini

import (
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		embErr := &outbound.EmbeddingError{}
		ok := errors.As(err, &embErr)
		assert.True(t, ok)
		assert.Equal(t, "empty_texts", embErr.Code)
		assert.Equal(t, "validation", embErr.Type)
		assert.False(t, embErr.Retryable)
	})

	t.Run("error when batch size exceeds maximum", func(t *testing.T) {
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

		// Create 501 chunks (exceeds max of 500)
		texts := make([]string, 501)
		for i := range texts {
			texts[i] = fmt.Sprintf("test chunk %d", i)
		}

		// Try to create job with too many chunks
		ctx := context.Background()
		job, err := batchClient.CreateBatchEmbeddingJob(ctx, texts, outbound.EmbeddingOptions{})

		// Should return error
		assert.Error(t, err)
		assert.Nil(t, job)

		// Check error type
		embErr := &outbound.EmbeddingError{}
		ok := errors.As(err, &embErr)
		assert.True(t, ok)
		assert.Equal(t, "batch_size_exceeded", embErr.Code)
		assert.Equal(t, "validation", embErr.Type)
		assert.False(t, embErr.Retryable)

		// Error message should contain actual count
		assert.Contains(t, embErr.Message, "501")
		assert.Contains(t, embErr.Message, "exceeds maximum batch size of 500")
	})

	t.Run("success with exactly maximum batch size", func(t *testing.T) {
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

		// Create exactly 500 chunks (at the limit)
		texts := make([]string, 500)
		for i := range texts {
			texts[i] = fmt.Sprintf("test chunk %d", i)
		}

		// Try to create job with exactly max chunks
		ctx := context.Background()
		job, err := batchClient.CreateBatchEmbeddingJob(ctx, texts, outbound.EmbeddingOptions{})

		// Should succeed (no error about batch size)
		// Note: This test will fail because the implementation doesn't exist yet,
		// but it might fail for other reasons (e.g., API call failures).
		// The important validation is that it should NOT fail due to batch size.
		if err != nil {
			// If there's an error, it should NOT be about batch size
			embErr := &outbound.EmbeddingError{}
			if errors.As(err, &embErr) {
				assert.NotEqual(t, "batch_size_exceeded", embErr.Code,
					"Should not fail with batch_size_exceeded error for exactly 500 chunks")
			}
		} else {
			// If successful, verify job was created
			assert.NotNil(t, job)
		}
	})

	t.Run("warning logged when batch size approaches limit", func(t *testing.T) {
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

		// Create 450 chunks (>400, <500 - should trigger warning)
		texts := make([]string, 450)
		for i := range texts {
			texts[i] = fmt.Sprintf("test chunk %d", i)
		}

		// Try to create job
		ctx := context.Background()
		job, err := batchClient.CreateBatchEmbeddingJob(ctx, texts, outbound.EmbeddingOptions{})

		// Should succeed but log warning
		// Note: We can't easily test log output without a log capture mechanism,
		// but this test documents the expected behavior.
		// The implementation should log a warning for batch sizes > 400.
		if err != nil {
			// If there's an error, it should NOT be about batch size
			embErr := &outbound.EmbeddingError{}
			if errors.As(err, &embErr) {
				assert.NotEqual(t, "batch_size_exceeded", embErr.Code,
					"Should not fail with batch_size_exceeded error for 450 chunks")
			}
		} else {
			// If successful, verify job was created
			assert.NotNil(t, job)
		}

		// TODO: Add log capture to verify warning message contains:
		// - "batch size approaching limit"
		// - "450"
		// - recommendation to split into multiple batches
	})

	t.Run("error with large batch and descriptive message", func(t *testing.T) {
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

		// Create 1000 chunks (significantly exceeds max)
		texts := make([]string, 1000)
		for i := range texts {
			texts[i] = fmt.Sprintf("test chunk %d", i)
		}

		// Try to create job with too many chunks
		ctx := context.Background()
		job, err := batchClient.CreateBatchEmbeddingJob(ctx, texts, outbound.EmbeddingOptions{})

		// Should return error
		assert.Error(t, err)
		assert.Nil(t, job)

		// Check error type
		embErr := &outbound.EmbeddingError{}
		ok := errors.As(err, &embErr)
		assert.True(t, ok)
		assert.Equal(t, "batch_size_exceeded", embErr.Code)
		assert.Equal(t, "validation", embErr.Type)
		assert.False(t, embErr.Retryable)

		// Error message should contain actual count and maximum
		assert.Contains(t, embErr.Message, "1000")
		assert.Contains(t, embErr.Message, "500")
		assert.Contains(t, embErr.Message, "exceeds maximum batch size")

		// Error message should suggest splitting into multiple batches
		messageLower := strings.ToLower(embErr.Message)
		assert.True(t,
			strings.Contains(messageLower, "split") || strings.Contains(messageLower, "multiple"),
			"Error message should suggest splitting into multiple batches")
	})

	t.Run("boundary test with 401 chunks", func(t *testing.T) {
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

		// Create 401 chunks (just above warning threshold of 400)
		texts := make([]string, 401)
		for i := range texts {
			texts[i] = fmt.Sprintf("test chunk %d", i)
		}

		// Try to create job
		ctx := context.Background()
		job, err := batchClient.CreateBatchEmbeddingJob(ctx, texts, outbound.EmbeddingOptions{})

		// Should succeed (not at limit yet) but may log warning
		if err != nil {
			// If there's an error, it should NOT be about batch size
			embErr := &outbound.EmbeddingError{}
			if errors.As(err, &embErr) {
				assert.NotEqual(t, "batch_size_exceeded", embErr.Code,
					"Should not fail with batch_size_exceeded error for 401 chunks")
			}
		} else {
			// If successful, verify job was created
			assert.NotNil(t, job)
		}
	})

	t.Run("boundary test with 499 chunks", func(t *testing.T) {
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

		// Create 499 chunks (just below maximum)
		texts := make([]string, 499)
		for i := range texts {
			texts[i] = fmt.Sprintf("test chunk %d", i)
		}

		// Try to create job
		ctx := context.Background()
		job, err := batchClient.CreateBatchEmbeddingJob(ctx, texts, outbound.EmbeddingOptions{})

		// Should succeed (not exceeding limit)
		if err != nil {
			// If there's an error, it should NOT be about batch size
			embErr := &outbound.EmbeddingError{}
			if errors.As(err, &embErr) {
				assert.NotEqual(t, "batch_size_exceeded", embErr.Code,
					"Should not fail with batch_size_exceeded error for 499 chunks")
			}
		} else {
			// If successful, verify job was created
			assert.NotNil(t, job)
		}
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
		embErr := &outbound.EmbeddingError{}
		ok := errors.As(err, &embErr)
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
		embErr := &outbound.EmbeddingError{}
		ok := errors.As(err, &embErr)
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
		embErr := &outbound.EmbeddingError{}
		ok := errors.As(err, &embErr)
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
		fullPath, filename, err := batchClient.writeRequestsToFile(ctx, requests, outbound.EmbeddingOptions{})

		require.NoError(t, err)
		assert.NotEmpty(t, fullPath)
		assert.NotEmpty(t, filename)
		assert.FileExists(t, fullPath)

		// Verify filename is base name only (not full path)
		assert.Equal(t, filepath.Base(fullPath), filename, "filename should be base name only")

		// Verify filename meets Google Gemini API requirement (max 40 characters)
		assert.LessOrEqual(t, len(filename), 40, "filename must not exceed 40 characters for Google Gemini API")

		// Verify file content uses Google Gemini format (NOT OpenAI format)
		content, err := os.ReadFile(fullPath)
		require.NoError(t, err)
		assert.NotEmpty(t, content)

		// Should use Google Gemini format with "key" and "request" fields
		assert.Contains(t, string(content), `"key":"req_1"`, "should use 'key' field (Google Gemini format)")
		assert.Contains(t, string(content), `"key":"req_2"`, "should use 'key' field (Google Gemini format)")
		assert.Contains(t, string(content), "Hello world")
		assert.Contains(t, string(content), "Test embedding")
		assert.Contains(t, string(content), `"request"`, "should use 'request' field (Google Gemini format)")
		assert.Contains(t, string(content), `"content"`, "should use 'content' field (singular, not plural)")
		assert.Contains(t, string(content), `"parts"`, "should use 'parts' field")

		// Should NOT use plural "contents" or nested "config"
		assert.NotContains(t, string(content), `"contents"`, "should use 'content' (singular), not 'contents' (plural)")
		assert.NotContains(t, string(content), `"config"`, "should not nest output_dimensionality in config object")

		// Should NOT use OpenAI batch format
		assert.NotContains(t, string(content), `"custom_id"`, "should NOT use OpenAI 'custom_id' field")
		assert.NotContains(t, string(content), `"method"`, "should NOT use OpenAI 'method' field")
		assert.NotContains(t, string(content), `"url"`, "should NOT use OpenAI 'url' field")
		assert.NotContains(t, string(content), `"body"`, "should NOT use OpenAI 'body' field")
		assert.NotContains(t, string(content), `:embedContent`, "should NOT include endpoint URL in JSONL")
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
		embErr := &outbound.EmbeddingError{}
		ok := errors.As(err, &embErr)
		assert.True(t, ok)
		assert.Equal(t, "context_cancelled", embErr.Code)
		assert.Equal(t, "timeout", embErr.Type)
		assert.False(t, embErr.Retryable)
	})
}
