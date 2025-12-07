package commands_test

import (
	"bytes"
	"codechunking/internal/application/dto"
	"codechunking/internal/client"
	"codechunking/internal/client/commands"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewHealthCmd verifies that NewHealthCmd creates a health command with correct metadata.
// This test ensures the health command is properly initialized.
func TestNewHealthCmd(t *testing.T) {
	t.Parallel()

	cmd := commands.NewHealthCmd()

	require.NotNil(t, cmd, "NewHealthCmd should return a non-nil command")
	assert.Equal(t, "health", cmd.Use, "health command Use should be 'health'")
	assert.NotEmpty(t, cmd.Short, "health command Short description should not be empty")
	assert.Contains(t, cmd.Short, "health", "Short description should mention health")
	assert.NotNil(t, cmd.RunE, "health command should have a RunE function")
}

// TestHealthCmd_Success verifies the health command outputs success JSON when the server is healthy.
// This test ensures proper integration with the API client and correct JSON formatting.
func TestHealthCmd_Success(t *testing.T) {
	t.Parallel()

	// Create mock server that returns healthy response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path, "should request /health endpoint")
		assert.Equal(t, http.MethodGet, r.Method, "should use GET method")

		response := dto.HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now(),
			Version:   "1.0.0",
			Dependencies: map[string]dto.DependencyStatus{
				"database": {
					Status:  "healthy",
					Message: "Connected",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	// Create command with custom output writer
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"health", "--api-url", server.URL})

	// Execute command
	err := rootCmd.Execute()
	require.NoError(t, err, "health command should execute without error")

	// Parse and verify JSON output
	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON")

	// Verify response envelope
	assert.True(t, response.Success, "response should indicate success")
	assert.Nil(t, response.Error, "response should not contain error")
	assert.NotZero(t, response.Timestamp, "response should have timestamp")

	// Verify health data
	require.NotNil(t, response.Data, "response should contain data")
	dataJSON, err := json.Marshal(response.Data)
	require.NoError(t, err)

	var healthData dto.HealthResponse
	err = json.Unmarshal(dataJSON, &healthData)
	require.NoError(t, err, "data should be a valid HealthResponse")
	assert.Equal(t, "healthy", healthData.Status, "health status should be 'healthy'")
	assert.Equal(t, "1.0.0", healthData.Version, "version should match server response")
	assert.Contains(t, healthData.Dependencies, "database", "should include database dependency")
}

// TestHealthCmd_ServerError verifies error JSON output when server returns an error.
// This test ensures proper error handling and JSON error formatting.
func TestHealthCmd_ServerError(t *testing.T) {
	t.Parallel()

	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	// Create command with custom output writer
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"health", "--api-url", server.URL})

	// Execute command - should not panic but may return error
	_ = rootCmd.Execute()

	// Parse and verify JSON output
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on error")

	// Verify error response envelope
	assert.False(t, response.Success, "response should indicate failure")
	assert.Nil(t, response.Data, "response should not contain data on error")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotZero(t, response.Timestamp, "response should have timestamp")

	// Verify error details
	assert.NotEmpty(t, response.Error.Code, "error should have a code")
	assert.NotEmpty(t, response.Error.Message, "error should have a message")
	assert.Contains(t, response.Error.Message, "500", "error message should mention status code")
}

// TestHealthCmd_ConnectionError verifies error JSON output when connection fails.
// This test ensures network errors are properly handled and formatted.
func TestHealthCmd_ConnectionError(t *testing.T) {
	t.Parallel()

	// Use invalid URL that will cause connection error
	invalidURL := "http://localhost:1"

	// Create command with custom output writer
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"health", "--api-url", invalidURL, "--timeout", "100ms"})

	// Execute command - should not panic
	_ = rootCmd.Execute()

	// Parse and verify JSON output
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on connection error")

	// Verify error response envelope
	assert.False(t, response.Success, "response should indicate failure")
	assert.Nil(t, response.Data, "response should not contain data on error")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotZero(t, response.Timestamp, "response should have timestamp")

	// Verify error details
	assert.NotEmpty(t, response.Error.Code, "error should have a code")
	assert.NotEmpty(t, response.Error.Message, "error should have a message")
	// Error code should indicate connection problem
	assert.Contains(t, []string{"CONNECTION_ERROR", "NETWORK_ERROR", "TIMEOUT_ERROR"},
		response.Error.Code, "error code should indicate connection/network issue")
}

// TestHealthCmd_JSONOutput verifies JSON structure matches Response envelope.
// This test ensures all fields are properly serialized and follow the expected schema.
func TestHealthCmd_JSONOutput(t *testing.T) {
	t.Parallel()

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now(),
			Version:   "test-version",
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	// Create command
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"health", "--api-url", server.URL})

	// Execute
	err := rootCmd.Execute()
	require.NoError(t, err)

	// Parse JSON and verify structure
	var rawJSON map[string]interface{}
	err = json.Unmarshal(stdout.Bytes(), &rawJSON)
	require.NoError(t, err, "output should be valid JSON")

	// Check required fields exist
	assert.Contains(t, rawJSON, "success", "JSON should have 'success' field")
	assert.Contains(t, rawJSON, "timestamp", "JSON should have 'timestamp' field")

	// Success response should have data field
	success, ok := rawJSON["success"].(bool)
	require.True(t, ok, "'success' should be a boolean")
	if success {
		assert.Contains(t, rawJSON, "data", "successful response should have 'data' field")
		assert.NotContains(t, rawJSON, "error", "successful response should not have 'error' field")
	} else {
		assert.Contains(t, rawJSON, "error", "failed response should have 'error' field")
		assert.NotContains(t, rawJSON, "data", "failed response should not have 'data' field")
	}

	// Verify timestamp is valid RFC3339 format
	timestampStr, ok := rawJSON["timestamp"].(string)
	require.True(t, ok, "'timestamp' should be a string")
	_, err = time.Parse(time.RFC3339Nano, timestampStr)
	assert.NoError(t, err, "timestamp should be valid RFC3339 format")
}

// TestHealthCmd_UsesGlobalFlags verifies the command respects global flags.
// This test ensures --api-url and --timeout flags are properly used.
func TestHealthCmd_UsesGlobalFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		serverDelay   time.Duration
		expectSuccess bool
	}{
		{
			name:          "custom api-url flag",
			args:          []string{"health", "--api-url", "http://custom.example.com"},
			serverDelay:   0,
			expectSuccess: false, // Will fail to connect to custom URL
		},
		{
			name:          "timeout flag causes timeout",
			args:          []string{"health", "--timeout", "50ms"},
			serverDelay:   200 * time.Millisecond, // Server slower than timeout
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Create server with optional delay
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.serverDelay > 0 {
					time.Sleep(tt.serverDelay)
				}
				response := dto.HealthResponse{Status: "healthy", Timestamp: time.Now(), Version: "1.0.0"}
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			// Override URL in args if using real server
			if tt.name == "timeout flag causes timeout" {
				tt.args = []string{"health", "--api-url", server.URL, "--timeout", "50ms"}
			}

			var stdout bytes.Buffer
			rootCmd := commands.NewRootCmd()
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stdout)
			rootCmd.SetArgs(tt.args)

			_ = rootCmd.Execute()

			// Verify JSON output regardless of success/failure
			var response client.Response
			err := json.Unmarshal(stdout.Bytes(), &response)
			require.NoError(t, err, "should output valid JSON")

			assert.Equal(t, tt.expectSuccess, response.Success, "success flag should match expected")
		})
	}
}

// TestHealthCmd_ContextCancellation verifies graceful handling of context cancellation.
// This test ensures the command can be interrupted cleanly.
func TestHealthCmd_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Create server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// Check if context was cancelled
		<-r.Context().Done()
	}))
	defer server.Close()

	// Create command with short timeout
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"health", "--api-url", server.URL, "--timeout", "10ms"})

	// Execute - should timeout and handle gracefully
	_ = rootCmd.Execute()

	// Should still output valid JSON
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "should output valid JSON even on timeout")
	assert.False(t, response.Success, "should indicate failure on timeout")
}

// TestHealthCmd_UserAgentHeader verifies correct User-Agent header is sent.
// This test ensures the client identifies itself properly.
func TestHealthCmd_UserAgentHeader(t *testing.T) {
	t.Parallel()

	userAgentReceived := ""

	// Create server that captures User-Agent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgentReceived = r.Header.Get("User-Agent")
		response := dto.HealthResponse{Status: "healthy", Timestamp: time.Now(), Version: "1.0.0"}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Execute command
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"health", "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Verify User-Agent was sent
	assert.NotEmpty(t, userAgentReceived, "User-Agent header should be sent")
	assert.Contains(t, userAgentReceived, "codechunking", "User-Agent should identify as codechunking client")
}

// TestHealthCmd_DependenciesField verifies dependencies are included in output.
// This test ensures the full health response structure is preserved.
func TestHealthCmd_DependenciesField(t *testing.T) {
	t.Parallel()

	// Create server with detailed dependencies
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now(),
			Version:   "1.0.0",
			Dependencies: map[string]dto.DependencyStatus{
				"database": {
					Status:       "healthy",
					Message:      "PostgreSQL connected",
					ResponseTime: "5ms",
				},
				"nats": {
					Status:       "healthy",
					Message:      "NATS JetStream connected",
					ResponseTime: "2ms",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Execute command
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"health", "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// Parse response
	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err)

	// Verify dependencies are included
	dataJSON, err := json.Marshal(response.Data)
	require.NoError(t, err)

	var healthData dto.HealthResponse
	err = json.Unmarshal(dataJSON, &healthData)
	require.NoError(t, err)

	assert.Len(t, healthData.Dependencies, 2, "should have 2 dependencies")
	assert.Contains(t, healthData.Dependencies, "database", "should include database dependency")
	assert.Contains(t, healthData.Dependencies, "nats", "should include nats dependency")
	assert.Equal(t, "healthy", healthData.Dependencies["database"].Status, "database should be healthy")
	assert.Equal(t, "PostgreSQL connected", healthData.Dependencies["database"].Message)
}

// TestHealthCmd_InvalidAPIURL verifies handling of invalid API URLs.
// This test ensures configuration validation errors are properly reported.
func TestHealthCmd_InvalidAPIURL(t *testing.T) {
	t.Parallel()

	// Create command with invalid URL (no scheme)
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"health", "--api-url", "invalid-url-without-scheme"})

	// Execute command - should handle invalid config
	_ = rootCmd.Execute()

	// Should output error JSON
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "should output valid JSON")

	assert.False(t, response.Success, "should indicate failure")
	assert.NotNil(t, response.Error, "should contain error")
	assert.Contains(t, []string{"INVALID_CONFIG", "CONFIG_ERROR", "VALIDATION_ERROR"},
		response.Error.Code, "error code should indicate config issue")
}

// TestHealthCmd_OutputToStdout verifies output is written to stdout, not stderr.
// This test ensures proper stream usage for JSON output.
func TestHealthCmd_OutputToStdout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.HealthResponse{Status: "healthy", Timestamp: time.Now(), Version: "1.0.0"}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"health", "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// JSON output should be on stdout
	assert.NotEmpty(t, stdout.String(), "stdout should contain JSON output")

	// stderr should be empty or only contain non-JSON logs
	stderrContent := stderr.String()
	if stderrContent != "" {
		// If anything in stderr, it should not be valid JSON response
		var response client.Response
		err := json.Unmarshal(stderr.Bytes(), &response)
		assert.Error(t, err, "stderr should not contain JSON response - only stdout should")
	}
}

// TestHealthCmd_Integration verifies end-to-end integration.
// This test simulates a real usage scenario with all components.
func TestHealthCmd_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Parallel()

	// Create realistic mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/health", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.NotEmpty(t, r.Header.Get("User-Agent"))

		// Return realistic response
		response := dto.HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now(),
			Version:   "1.0.0-dev",
			Dependencies: map[string]dto.DependencyStatus{
				"database": {Status: "healthy", Message: "Connected", ResponseTime: "5ms"},
				"nats":     {Status: "healthy", Message: "Connected", ResponseTime: "2ms"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Execute health command
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"health", "--api-url", server.URL, "--timeout", "5s"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Set context on command if supported
	errChan := make(chan error, 1)
	go func() {
		errChan <- rootCmd.ExecuteContext(ctx)
	}()

	select {
	case err := <-errChan:
		require.NoError(t, err, "integration test should complete successfully")
	case <-ctx.Done():
		t.Fatal("integration test timed out")
	}

	// Verify complete workflow
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "should output valid JSON")
	assert.True(t, response.Success, "integration test should succeed")
	assert.NotNil(t, response.Data, "should contain health data")
	assert.Nil(t, response.Error, "should not contain error")

	// Verify health details
	dataJSON, _ := json.Marshal(response.Data)
	var healthData dto.HealthResponse
	json.Unmarshal(dataJSON, &healthData)
	assert.Equal(t, "healthy", healthData.Status)
	assert.Equal(t, "1.0.0-dev", healthData.Version)
	assert.Len(t, healthData.Dependencies, 2)
}
