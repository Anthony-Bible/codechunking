package gemini

import (
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_EnableBatchProcessing(t *testing.T) {
	t.Run("successfully enables batch processing", func(t *testing.T) {
		// Create client
		client, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		// Verify batch processing is disabled by default
		assert.False(t, client.IsBatchProcessingEnabled())
		assert.Nil(t, client.batchClient)

		// Enable batch processing
		tempDir := t.TempDir()
		inputDir := filepath.Join(tempDir, "input")
		outputDir := filepath.Join(tempDir, "output")
		pollInterval := 2 * time.Second

		err = client.EnableBatchProcessing(inputDir, outputDir, pollInterval)
		require.NoError(t, err)

		// Verify batch processing is enabled
		assert.True(t, client.IsBatchProcessingEnabled())
		assert.NotNil(t, client.batchClient)
		assert.True(t, client.useBatchAPI)
		assert.Equal(t, pollInterval, client.batchPollInterval)
	})

	t.Run("uses default poll interval when zero", func(t *testing.T) {
		client, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		tempDir := t.TempDir()
		err = client.EnableBatchProcessing(filepath.Join(tempDir, "input"), filepath.Join(tempDir, "output"), 0)
		require.NoError(t, err)

		// Should use default 5 second interval
		assert.Equal(t, 5*time.Second, client.batchPollInterval)
	})

	t.Run("uses default directories when empty", func(t *testing.T) {
		client, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		err = client.EnableBatchProcessing("", "", 1*time.Second)
		require.NoError(t, err)

		// Should have created batch client with default directories
		assert.True(t, client.IsBatchProcessingEnabled())
		assert.NotNil(t, client.batchClient)
	})
}

func TestClient_DisableBatchProcessing(t *testing.T) {
	t.Run("successfully disables batch processing", func(t *testing.T) {
		// Create client and enable batch processing
		client, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		tempDir := t.TempDir()
		err = client.EnableBatchProcessing(
			filepath.Join(tempDir, "input"),
			filepath.Join(tempDir, "output"),
			2*time.Second,
		)
		require.NoError(t, err)
		assert.True(t, client.IsBatchProcessingEnabled())

		// Disable batch processing
		client.DisableBatchProcessing()

		// Verify batch processing is disabled
		assert.False(t, client.IsBatchProcessingEnabled())
		assert.Nil(t, client.batchClient)
		assert.False(t, client.useBatchAPI)
	})
}

func TestClient_GenerateBatchEmbeddings_ModeSelection(t *testing.T) {
	t.Run("uses sequential mode when batch processing is disabled", func(t *testing.T) {
		// Create client without batch processing
		client, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		ctx := context.Background()
		texts := []string{"test text"}
		options := outbound.EmbeddingOptions{}

		// This will fail because we don't have a real API key, but we can verify
		// it tries to use sequential mode by checking the error message
		_, err = client.GenerateBatchEmbeddings(ctx, texts, options)

		// Should get an error from the API call, not from batch job creation
		assert.Error(t, err)
		// The error should be from GenerateEmbedding, not from batch job creation
		assert.Contains(t, err.Error(), "failed to generate embedding for text")
	})

	t.Run("validates empty texts array", func(t *testing.T) {
		client, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		ctx := context.Background()
		_, err = client.GenerateBatchEmbeddings(ctx, []string{}, outbound.EmbeddingOptions{})

		assert.Error(t, err)
		embErr := &outbound.EmbeddingError{}
		ok := errors.As(err, &embErr)
		require.True(t, ok)
		assert.Equal(t, "empty_texts", embErr.Code)
		assert.Equal(t, "validation", embErr.Type)
	})
}

func TestClient_IsBatchProcessingEnabled(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*Client)
		expected bool
	}{
		{
			name:     "disabled by default",
			setup:    func(c *Client) {},
			expected: false,
		},
		{
			name: "enabled after EnableBatchProcessing",
			setup: func(c *Client) {
				tempDir := filepath.Join(t.TempDir(), "batch")
				_ = c.EnableBatchProcessing(
					filepath.Join(tempDir, "input"),
					filepath.Join(tempDir, "output"),
					1*time.Second,
				)
			},
			expected: true,
		},
		{
			name: "disabled after DisableBatchProcessing",
			setup: func(c *Client) {
				tempDir := filepath.Join(t.TempDir(), "batch")
				_ = c.EnableBatchProcessing(
					filepath.Join(tempDir, "input"),
					filepath.Join(tempDir, "output"),
					1*time.Second,
				)
				c.DisableBatchProcessing()
			},
			expected: false,
		},
		{
			name: "false when useBatchAPI is true but batchClient is nil",
			setup: func(c *Client) {
				c.useBatchAPI = true
				c.batchClient = nil
			},
			expected: false,
		},
		{
			name: "false when batchClient exists but useBatchAPI is false",
			setup: func(c *Client) {
				tempDir := filepath.Join(t.TempDir(), "batch")
				_ = c.EnableBatchProcessing(
					filepath.Join(tempDir, "input"),
					filepath.Join(tempDir, "output"),
					1*time.Second,
				)
				c.useBatchAPI = false
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(&ClientConfig{
				APIKey: "test-api-key",
				Model:  DefaultModel,
			})
			require.NoError(t, err)

			tt.setup(client)

			assert.Equal(t, tt.expected, client.IsBatchProcessingEnabled())
		})
	}
}

func TestClient_generateBatchEmbeddingsSequential(t *testing.T) {
	t.Run("validates empty texts", func(t *testing.T) {
		client, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		ctx := context.Background()
		// The method will be called through GenerateBatchEmbeddings
		// with empty array validation happening first
		results, err := client.generateBatchEmbeddingsSequential(ctx, []string{}, outbound.EmbeddingOptions{})

		// Sequential processing with empty array should return empty results
		assert.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestClient_BatchProcessing_ConfigPersistence(t *testing.T) {
	t.Run("configuration persists across multiple calls", func(t *testing.T) {
		client, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		tempDir := t.TempDir()
		inputDir := filepath.Join(tempDir, "input")
		outputDir := filepath.Join(tempDir, "output")
		pollInterval := 3 * time.Second

		// Enable batch processing
		err = client.EnableBatchProcessing(inputDir, outputDir, pollInterval)
		require.NoError(t, err)

		// Make multiple calls to IsBatchProcessingEnabled
		for range 5 {
			assert.True(t, client.IsBatchProcessingEnabled())
			assert.Equal(t, pollInterval, client.batchPollInterval)
		}

		// Disable and check again
		client.DisableBatchProcessing()
		for range 5 {
			assert.False(t, client.IsBatchProcessingEnabled())
		}
	})

	t.Run("can re-enable after disable", func(t *testing.T) {
		client, err := NewClient(&ClientConfig{
			APIKey: "test-api-key",
			Model:  DefaultModel,
		})
		require.NoError(t, err)

		tempDir := t.TempDir()

		// Enable, disable, enable again
		err = client.EnableBatchProcessing(
			filepath.Join(tempDir, "input1"),
			filepath.Join(tempDir, "output1"),
			1*time.Second,
		)
		require.NoError(t, err)
		assert.True(t, client.IsBatchProcessingEnabled())

		client.DisableBatchProcessing()
		assert.False(t, client.IsBatchProcessingEnabled())

		err = client.EnableBatchProcessing(
			filepath.Join(tempDir, "input2"),
			filepath.Join(tempDir, "output2"),
			2*time.Second,
		)
		require.NoError(t, err)
		assert.True(t, client.IsBatchProcessingEnabled())
		assert.Equal(t, 2*time.Second, client.batchPollInterval)
	})
}
