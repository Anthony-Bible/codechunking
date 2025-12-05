package client_test

import (
	"bytes"
	"codechunking/internal/application/dto"
	"codechunking/internal/client"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewPoller_ValidConfig verifies NewPoller creates a poller with valid configuration.
// This test ensures the constructor accepts valid client and configuration parameters.
func TestNewPoller_ValidConfig(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	pollerConfig := &client.PollerConfig{
		Interval: 2 * time.Second,
		MaxWait:  10 * time.Minute,
	}

	poller, err := client.NewPoller(c, pollerConfig)

	require.NoError(t, err, "NewPoller should not return error with valid config")
	assert.NotNil(t, poller, "poller should not be nil")
}

// TestNewPoller_NilClient verifies NewPoller returns error when client is nil.
// This test ensures proper validation of required dependencies.
func TestNewPoller_NilClient(t *testing.T) {
	t.Parallel()

	pollerConfig := &client.PollerConfig{
		Interval: 5 * time.Second,
		MaxWait:  30 * time.Minute,
	}

	poller, err := client.NewPoller(nil, pollerConfig)

	require.Error(t, err, "NewPoller should return error when client is nil")
	assert.Nil(t, poller, "poller should be nil on error")
	assert.Contains(t, err.Error(), "client", "error should mention client")
}

// TestNewPoller_ZeroInterval verifies NewPoller uses default interval when zero is provided.
// This test ensures sensible defaults are applied for zero-valued configuration.
func TestNewPoller_ZeroInterval(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	pollerConfig := &client.PollerConfig{
		Interval: 0, // Zero interval should use default
		MaxWait:  10 * time.Minute,
	}

	poller, err := client.NewPoller(c, pollerConfig)

	require.NoError(t, err, "NewPoller should accept zero interval and use default")
	assert.NotNil(t, poller, "poller should not be nil")
	// Note: Cannot directly test internal field, but behavior will be tested in WaitForCompletion
}

// TestNewPoller_ZeroMaxWait verifies NewPoller uses default maxWait when zero is provided.
// This test ensures sensible defaults are applied for zero-valued maxWait configuration.
func TestNewPoller_ZeroMaxWait(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	pollerConfig := &client.PollerConfig{
		Interval: 2 * time.Second,
		MaxWait:  0, // Zero maxWait should use default
	}

	poller, err := client.NewPoller(c, pollerConfig)

	require.NoError(t, err, "NewPoller should accept zero maxWait and use default")
	assert.NotNil(t, poller, "poller should not be nil")
}

// TestNewPoller_NilConfig verifies NewPoller accepts nil config and uses all defaults.
// This test ensures the constructor provides sensible defaults when config is omitted.
func TestNewPoller_NilConfig(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	poller, err := client.NewPoller(c, nil) // Nil config should use all defaults

	require.NoError(t, err, "NewPoller should accept nil config and use defaults")
	assert.NotNil(t, poller, "poller should not be nil")
}

// TestPoller_WaitForCompletion_AlreadyCompleted verifies WaitForCompletion returns immediately
// when the repository is already in completed status.
// This test ensures no unnecessary polling occurs for already-completed repositories.
func TestPoller_WaitForCompletion_AlreadyCompleted(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repositories/"+repoID.String(), r.URL.Path)

		resp := dto.RepositoryResponse{
			ID:          repoID,
			URL:         "https://github.com/user/repo",
			Name:        "user/repo",
			Status:      dto.StatusCompleted,
			TotalFiles:  100,
			TotalChunks: 500,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	poller, err := client.NewPoller(c, nil)
	require.NoError(t, err)

	var progress bytes.Buffer
	ctx := context.Background()
	result, err := poller.WaitForCompletion(ctx, repoID, &progress)

	require.NoError(t, err, "WaitForCompletion should succeed for completed repository")
	assert.NotNil(t, result, "result should not be nil")
	assert.Equal(t, dto.StatusCompleted, result.Status, "status should be completed")
	assert.Empty(t, progress.String(), "no progress output expected when already completed")
}

// TestPoller_WaitForCompletion_AlreadyFailed verifies WaitForCompletion returns error
// when the repository is already in failed status.
// This test ensures failed repositories are immediately reported as errors.
func TestPoller_WaitForCompletion_AlreadyFailed(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repositories/"+repoID.String(), r.URL.Path)

		resp := dto.RepositoryResponse{
			ID:        repoID,
			URL:       "https://github.com/user/repo",
			Name:      "user/repo",
			Status:    dto.StatusFailed,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	poller, err := client.NewPoller(c, nil)
	require.NoError(t, err)

	var progress bytes.Buffer
	ctx := context.Background()
	result, err := poller.WaitForCompletion(ctx, repoID, &progress)

	require.Error(t, err, "WaitForCompletion should return error for failed repository")
	assert.NotNil(t, result, "result should be returned even on failure")
	assert.Equal(t, dto.StatusFailed, result.Status, "status should be failed")
	assert.Contains(t, err.Error(), "failed", "error message should mention failure")
}

// TestPoller_WaitForCompletion_TransitionsToCompleted verifies WaitForCompletion polls
// through status transitions until reaching completed state.
// This test simulates a realistic indexing workflow: pending -> cloning -> processing -> completed.
func TestPoller_WaitForCompletion_TransitionsToCompleted(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	callCount := 0
	statuses := []string{dto.StatusPending, dto.StatusCloning, dto.StatusProcessing, dto.StatusCompleted}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repositories/"+repoID.String(), r.URL.Path)

		currentStatus := statuses[callCount]
		if callCount < len(statuses)-1 {
			callCount++
		}

		resp := dto.RepositoryResponse{
			ID:        repoID,
			URL:       "https://github.com/user/repo",
			Name:      "user/repo",
			Status:    currentStatus,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Add progress fields for processing state
		if currentStatus == dto.StatusProcessing || currentStatus == dto.StatusCompleted {
			resp.TotalFiles = 50
			resp.TotalChunks = 250
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	// Use short intervals for faster test execution
	pollerConfig := &client.PollerConfig{
		Interval: 100 * time.Millisecond,
		MaxWait:  10 * time.Second,
	}
	poller, err := client.NewPoller(c, pollerConfig)
	require.NoError(t, err)

	var progress bytes.Buffer
	ctx := context.Background()
	result, err := poller.WaitForCompletion(ctx, repoID, &progress)

	require.NoError(t, err, "WaitForCompletion should succeed after transitions")
	assert.NotNil(t, result, "result should not be nil")
	assert.Equal(t, dto.StatusCompleted, result.Status, "final status should be completed")
	assert.NotEmpty(t, progress.String(), "progress output should be written")

	// Verify progress output contains status updates
	progressOutput := progress.String()
	assert.Contains(t, progressOutput, "pending", "progress should mention pending status")
	assert.Contains(t, progressOutput, "cloning", "progress should mention cloning status")
	assert.Contains(t, progressOutput, "processing", "progress should mention processing status")
}

// TestPoller_WaitForCompletion_TransitionsToFailed verifies WaitForCompletion polls
// until repository transitions to failed state and returns appropriate error.
// This test ensures failure conditions are properly detected during polling.
func TestPoller_WaitForCompletion_TransitionsToFailed(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	callCount := 0
	statuses := []string{dto.StatusPending, dto.StatusCloning, dto.StatusFailed}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repositories/"+repoID.String(), r.URL.Path)

		currentStatus := statuses[callCount]
		if callCount < len(statuses)-1 {
			callCount++
		}

		resp := dto.RepositoryResponse{
			ID:        repoID,
			URL:       "https://github.com/user/repo",
			Name:      "user/repo",
			Status:    currentStatus,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	pollerConfig := &client.PollerConfig{
		Interval: 100 * time.Millisecond,
		MaxWait:  10 * time.Second,
	}
	poller, err := client.NewPoller(c, pollerConfig)
	require.NoError(t, err)

	var progress bytes.Buffer
	ctx := context.Background()
	result, err := poller.WaitForCompletion(ctx, repoID, &progress)

	require.Error(t, err, "WaitForCompletion should return error when repository fails")
	assert.NotNil(t, result, "result should be returned even on failure")
	assert.Equal(t, dto.StatusFailed, result.Status, "final status should be failed")
	assert.Contains(t, err.Error(), "failed", "error message should mention failure")
	assert.NotEmpty(t, progress.String(), "progress output should contain polling updates")
}

// TestPoller_WaitForCompletion_Timeout verifies WaitForCompletion returns timeout error
// when maxWait is exceeded before reaching terminal state.
// This test ensures polling does not continue indefinitely.
func TestPoller_WaitForCompletion_Timeout(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repositories/"+repoID.String(), r.URL.Path)

		// Always return processing status (never completes)
		resp := dto.RepositoryResponse{
			ID:        repoID,
			URL:       "https://github.com/user/repo",
			Name:      "user/repo",
			Status:    dto.StatusProcessing,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	// Use very short maxWait to trigger timeout quickly
	pollerConfig := &client.PollerConfig{
		Interval: 50 * time.Millisecond,
		MaxWait:  200 * time.Millisecond, // Timeout after 200ms
	}
	poller, err := client.NewPoller(c, pollerConfig)
	require.NoError(t, err)

	var progress bytes.Buffer
	ctx := context.Background()
	result, err := poller.WaitForCompletion(ctx, repoID, &progress)

	require.Error(t, err, "WaitForCompletion should return error on timeout")
	assert.NotNil(t, result, "result should contain last known state")
	assert.Equal(t, dto.StatusProcessing, result.Status, "status should be last polled status")
	assert.Contains(t, err.Error(), "timeout", "error message should mention timeout")
	assert.NotEmpty(t, progress.String(), "progress output should contain polling attempts")
}

// TestPoller_WaitForCompletion_ContextCancelled verifies WaitForCompletion respects
// context cancellation and returns appropriate error.
// This test ensures polling can be interrupted by user/system cancellation.
func TestPoller_WaitForCompletion_ContextCancelled(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repositories/"+repoID.String(), r.URL.Path)

		resp := dto.RepositoryResponse{
			ID:        repoID,
			URL:       "https://github.com/user/repo",
			Name:      "user/repo",
			Status:    dto.StatusProcessing,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	pollerConfig := &client.PollerConfig{
		Interval: 100 * time.Millisecond,
		MaxWait:  10 * time.Second,
	}
	poller, err := client.NewPoller(c, pollerConfig)
	require.NoError(t, err)

	// Create context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after short delay
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	var progress bytes.Buffer
	result, err := poller.WaitForCompletion(ctx, repoID, &progress)

	require.Error(t, err, "WaitForCompletion should return error when context is cancelled")
	assert.NotNil(t, result, "result should contain last known state")
	assert.Contains(t, err.Error(), "context", "error message should mention context cancellation")
}

// TestPoller_WaitForCompletion_NetworkError verifies WaitForCompletion handles
// transient network errors gracefully and continues polling.
// This test ensures resilience to temporary API unavailability.
func TestPoller_WaitForCompletion_NetworkError(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repositories/"+repoID.String(), r.URL.Path)

		callCount++

		// First two calls fail with 500 error
		if callCount <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
			return
		}

		// Third call succeeds with completed status
		resp := dto.RepositoryResponse{
			ID:        repoID,
			URL:       "https://github.com/user/repo",
			Name:      "user/repo",
			Status:    dto.StatusCompleted,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	pollerConfig := &client.PollerConfig{
		Interval: 100 * time.Millisecond,
		MaxWait:  10 * time.Second,
	}
	poller, err := client.NewPoller(c, pollerConfig)
	require.NoError(t, err)

	var progress bytes.Buffer
	ctx := context.Background()
	result, err := poller.WaitForCompletion(ctx, repoID, &progress)

	require.NoError(t, err, "WaitForCompletion should recover from transient errors")
	assert.NotNil(t, result, "result should not be nil")
	assert.Equal(t, dto.StatusCompleted, result.Status, "final status should be completed")
	assert.NotEmpty(t, progress.String(), "progress output should contain polling attempts")
}

// TestPoller_WaitForCompletion_ProgressOutput verifies WaitForCompletion writes
// structured JSON progress updates to the progress writer (stderr).
// This test ensures proper progress reporting format for CLI consumption.
func TestPoller_WaitForCompletion_ProgressOutput(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	callCount := 0
	statuses := []string{dto.StatusPending, dto.StatusCloning, dto.StatusCompleted}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		currentStatus := statuses[callCount]
		if callCount < len(statuses)-1 {
			callCount++
		}

		resp := dto.RepositoryResponse{
			ID:        repoID,
			URL:       "https://github.com/user/repo",
			Name:      "user/repo",
			Status:    currentStatus,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	pollerConfig := &client.PollerConfig{
		Interval: 100 * time.Millisecond,
		MaxWait:  10 * time.Second,
	}
	poller, err := client.NewPoller(c, pollerConfig)
	require.NoError(t, err)

	var progress bytes.Buffer
	ctx := context.Background()
	result, err := poller.WaitForCompletion(ctx, repoID, &progress)

	require.NoError(t, err, "WaitForCompletion should succeed")
	assert.Equal(t, dto.StatusCompleted, result.Status)

	// Verify progress output format
	progressOutput := progress.String()
	assert.NotEmpty(t, progressOutput, "progress output should not be empty")

	// Parse progress JSON lines
	lines := bytes.Split(progress.Bytes(), []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		var progressMsg map[string]interface{}
		err := json.Unmarshal(line, &progressMsg)
		require.NoError(t, err, "each progress line should be valid JSON")

		// Verify required fields
		assert.Equal(t, "polling", progressMsg["status"], "status field should be 'polling'")
		assert.Equal(t, repoID.String(), progressMsg["repository_id"], "repository_id should match")
		assert.NotEmpty(t, progressMsg["current_status"], "current_status should be present")
		assert.NotEmpty(t, progressMsg["elapsed"], "elapsed time should be present")
		assert.NotZero(t, progressMsg["poll_count"], "poll_count should be present")
	}
}

// TestIsTerminalStatus_Completed verifies IsTerminalStatus returns true for completed status.
func TestIsTerminalStatus_Completed(t *testing.T) {
	t.Parallel()

	result := client.IsTerminalStatus(dto.StatusCompleted)

	assert.True(t, result, "completed should be a terminal status")
}

// TestIsTerminalStatus_Failed verifies IsTerminalStatus returns true for failed status.
func TestIsTerminalStatus_Failed(t *testing.T) {
	t.Parallel()

	result := client.IsTerminalStatus(dto.StatusFailed)

	assert.True(t, result, "failed should be a terminal status")
}

// TestIsTerminalStatus_Archived verifies IsTerminalStatus returns true for archived status.
func TestIsTerminalStatus_Archived(t *testing.T) {
	t.Parallel()

	result := client.IsTerminalStatus(dto.StatusArchived)

	assert.True(t, result, "archived should be a terminal status")
}

// TestIsTerminalStatus_Pending verifies IsTerminalStatus returns false for pending status.
func TestIsTerminalStatus_Pending(t *testing.T) {
	t.Parallel()

	result := client.IsTerminalStatus(dto.StatusPending)

	assert.False(t, result, "pending should not be a terminal status")
}

// TestIsTerminalStatus_Cloning verifies IsTerminalStatus returns false for cloning status.
func TestIsTerminalStatus_Cloning(t *testing.T) {
	t.Parallel()

	result := client.IsTerminalStatus(dto.StatusCloning)

	assert.False(t, result, "cloning should not be a terminal status")
}

// TestIsTerminalStatus_Processing verifies IsTerminalStatus returns false for processing status.
func TestIsTerminalStatus_Processing(t *testing.T) {
	t.Parallel()

	result := client.IsTerminalStatus(dto.StatusProcessing)

	assert.False(t, result, "processing should not be a terminal status")
}
