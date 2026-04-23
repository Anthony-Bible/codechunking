// Package gemini contains tests for the GetBatchJobStatuses bulk method.
//
// These tests verify bulk status lookup behavior, including the zero-input
// contract and validation of blank or whitespace-only job IDs.
package gemini

import (
	"codechunking/internal/port/outbound"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newBulkStatusTestClient is a helper that builds a BatchEmbeddingClient wired to a
// temp directory, suitable for the bulk-status RED-phase tests below.
func newBulkStatusTestClient(t *testing.T) *BatchEmbeddingClient {
	t.Helper()

	base, err := NewClient(&ClientConfig{
		APIKey: "test-api-key",
		Model:  DefaultModel,
	})
	require.NoError(t, err)

	tmp := t.TempDir()
	client, err := NewBatchEmbeddingClient(base,
		filepath.Join(tmp, "input"),
		filepath.Join(tmp, "output"),
	)
	require.NoError(t, err)

	return client
}

// TestBatchEmbeddingClient_GetBatchJobStatuses_EmptyIDs verifies the zero-input contract:
// an empty ID slice must return an empty (non-nil) map with no error and must not issue
// any outbound API call.
//
// Contract: []string{} → (map[string]*BatchEmbeddingJob{}, nil).
func TestBatchEmbeddingClient_GetBatchJobStatuses_EmptyIDs(t *testing.T) {
	client := newBulkStatusTestClient(t)
	ctx := context.Background()

	result, err := client.GetBatchJobStatuses(ctx, []string{})

	require.NoError(t, err, "empty ID slice must not produce an error")
	assert.NotNil(t, result, "returned map must not be nil for empty input")
	assert.Empty(t, result, "returned map must be empty for empty input")
}

// TestBatchEmbeddingClient_GetBatchJobStatuses_ValidationError verifies that any blank
// or whitespace-only job ID in the slice is rejected with a typed validation error
// before any network call is attempted.
//
// Contract: slice containing "" or "   " → (*outbound.EmbeddingError{Type:"validation"}, nil map).
func TestBatchEmbeddingClient_GetBatchJobStatuses_ValidationError(t *testing.T) {
	client := newBulkStatusTestClient(t)
	ctx := context.Background()

	cases := []struct {
		name   string
		jobIDs []string
	}{
		{
			name:   "single empty string",
			jobIDs: []string{""},
		},
		{
			name:   "mix of valid and empty string",
			jobIDs: []string{"batches/valid_job_123", ""},
		},
		{
			name:   "whitespace-only string",
			jobIDs: []string{strings.Repeat(" ", 3)},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, callErr := client.GetBatchJobStatuses(ctx, tc.jobIDs)

			assert.Error(t, callErr, "blank job ID in slice must produce an error")
			assert.Nil(t, result, "result map must be nil on validation error")

			var embErr *outbound.EmbeddingError
			require.ErrorAs(t, callErr, &embErr,
				"error must be *outbound.EmbeddingError, got %T", callErr)
			assert.Equal(t, "validation", embErr.Type,
				"error type must be 'validation' for blank job ID")
			assert.False(t, embErr.Retryable,
				"validation errors must not be retryable")
		})
	}
}
