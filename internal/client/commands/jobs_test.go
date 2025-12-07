package commands_test

import (
	"bytes"
	"codechunking/internal/application/dto"
	"codechunking/internal/client"
	"codechunking/internal/client/commands"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewJobsCmd verifies that NewJobsCmd creates a jobs parent command with correct metadata.
// This test ensures the jobs command is properly initialized with Use and Short description.
func TestNewJobsCmd(t *testing.T) {
	t.Parallel()

	cmd := commands.NewJobsCmd()

	require.NotNil(t, cmd, "NewJobsCmd should return a non-nil command")
	assert.Equal(t, "jobs", cmd.Use, "jobs command Use should be 'jobs'")
	assert.NotEmpty(t, cmd.Short, "jobs command Short description should not be empty")
	assert.Contains(t, cmd.Short, "job", "Short description should mention jobs")
}

// TestJobsCmd_RegisteredWithRoot verifies jobs command is registered with root command.
// This test ensures the jobs command is accessible from the root CLI.
func TestJobsCmd_RegisteredWithRoot(t *testing.T) {
	t.Parallel()

	rootCmd := commands.NewRootCmd()

	// Find jobs command
	var found bool
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "jobs" {
			found = true
			break
		}
	}

	assert.True(t, found, "jobs command should be registered with root command")
}

// TestJobsCmd_HasGetSubcommand verifies the jobs command has a get subcommand.
// This test ensures the get subcommand is registered under jobs.
func TestJobsCmd_HasGetSubcommand(t *testing.T) {
	t.Parallel()

	cmd := commands.NewJobsCmd()

	subcommands := cmd.Commands()
	require.NotEmpty(t, subcommands, "jobs command should have subcommands")

	// Collect subcommand names
	subcommandNames := make(map[string]bool)
	for _, subcmd := range subcommands {
		subcommandNames[subcmd.Use] = true
	}

	// Verify get subcommand exists
	assert.True(t, subcommandNames["get"], "jobs should have 'get' subcommand")
}

// TestJobsGetCmd_RequiresTwoArguments verifies the jobs get command requires repo-id and job-id arguments.
// This test ensures proper argument count validation.
func TestJobsGetCmd_RequiresTwoArguments(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()
	startedAt := time.Now().Add(-5 * time.Minute)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.IndexingJobResponse{
			ID:             jobID,
			RepositoryID:   repoID,
			Status:         "completed",
			StartedAt:      &startedAt,
			FilesProcessed: 100,
			ChunksCreated:  500,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"jobs", "get", repoID.String(), jobID.String(), "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err, "jobs get command with two arguments should execute")

	// Verify valid JSON response
	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON")
	assert.True(t, response.Success, "should succeed with two valid arguments")
}

// TestJobsGetCmd_MissingArguments verifies error JSON output when arguments are missing.
// This test ensures proper error handling when required arguments are not provided.
func TestJobsGetCmd_MissingArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "no arguments",
			args: []string{"jobs", "get"},
		},
		{
			name: "only repo-id",
			args: []string{"jobs", "get", uuid.New().String()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			rootCmd := commands.NewRootCmd()
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stdout)
			rootCmd.SetArgs(tt.args)

			// Execute command - should fail validation
			_ = rootCmd.Execute()

			// Parse and verify JSON error output
			var response client.Response
			err := json.Unmarshal(stdout.Bytes(), &response)
			require.NoError(t, err, "output should be valid JSON even on error")

			assert.False(t, response.Success, "response should indicate failure")
			assert.NotNil(t, response.Error, "response should contain error")
			assert.NotEmpty(t, response.Error.Message, "error should have a message")
			assert.Contains(t, response.Error.Message, "require", "error message should mention required arguments")
		})
	}
}

// TestJobsGetCmd_InvalidRepoID verifies error JSON output for invalid UUID in repo-id argument.
// This test ensures proper validation of repository ID format.
func TestJobsGetCmd_InvalidRepoID(t *testing.T) {
	t.Parallel()

	jobID := uuid.New()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"jobs", "get", "invalid-uuid", jobID.String()})

	// Execute command - should fail validation
	_ = rootCmd.Execute()

	// Parse and verify JSON error output
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on validation error")

	assert.False(t, response.Success, "response should indicate failure")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotEmpty(t, response.Error.Message, "error should have a message")
	assert.Contains(t, response.Error.Message, "UUID", "error message should mention UUID")
}

// TestJobsGetCmd_InvalidJobID verifies error JSON output for invalid UUID in job-id argument.
// This test ensures proper validation of job ID format.
func TestJobsGetCmd_InvalidJobID(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"jobs", "get", repoID.String(), "invalid-uuid"})

	// Execute command - should fail validation
	_ = rootCmd.Execute()

	// Parse and verify JSON error output
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on validation error")

	assert.False(t, response.Success, "response should indicate failure")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotEmpty(t, response.Error.Message, "error should have a message")
	assert.Contains(t, response.Error.Message, "UUID", "error message should mention UUID")
}

// TestJobsGetCmd_Success verifies the jobs get command retrieves a job and returns JSON success.
// This test ensures proper integration with the API client for job retrieval.
func TestJobsGetCmd_Success(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()
	startedAt := time.Now().Add(-5 * time.Minute)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf("/repositories/%s/jobs/%s", repoID.String(), jobID.String())
		assert.Equal(t, expectedPath, r.URL.Path, "should request correct job path")
		assert.Equal(t, http.MethodGet, r.Method, "should use GET method")

		response := dto.IndexingJobResponse{
			ID:             jobID,
			RepositoryID:   repoID,
			Status:         "completed",
			StartedAt:      &startedAt,
			FilesProcessed: 150,
			ChunksCreated:  500,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"jobs", "get", repoID.String(), jobID.String(), "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err, "jobs get command should execute without error")

	// Parse and verify JSON output
	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON")

	// Verify response envelope
	assert.True(t, response.Success, "response should indicate success")
	assert.Nil(t, response.Error, "response should not contain error")
	assert.NotZero(t, response.Timestamp, "response should have timestamp")

	// Verify job data
	require.NotNil(t, response.Data, "response should contain data")
	dataJSON, err := json.Marshal(response.Data)
	require.NoError(t, err)

	var jobData dto.IndexingJobResponse
	err = json.Unmarshal(dataJSON, &jobData)
	require.NoError(t, err, "data should be a valid IndexingJobResponse")
	assert.Equal(t, jobID, jobData.ID, "job ID should match")
	assert.Equal(t, repoID, jobData.RepositoryID, "repository ID should match")
	assert.Equal(t, "completed", jobData.Status, "status should be completed")
	assert.Equal(t, 150, jobData.FilesProcessed, "files processed should match")
	assert.Equal(t, 500, jobData.ChunksCreated, "chunks created should match")
}

// TestJobsGetCmd_WithAllFields verifies all job fields are properly returned in the response.
// This test ensures complete job information including optional fields is serialized correctly.
func TestJobsGetCmd_WithAllFields(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()
	startedAt := time.Now().Add(-10 * time.Minute)
	completedAt := time.Now().Add(-5 * time.Minute)
	errorMessage := "Test error message"
	duration := "5m0s"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.IndexingJobResponse{
			ID:             jobID,
			RepositoryID:   repoID,
			Status:         "completed",
			StartedAt:      &startedAt,
			CompletedAt:    &completedAt,
			ErrorMessage:   &errorMessage,
			FilesProcessed: 250,
			ChunksCreated:  1000,
			CreatedAt:      time.Now().Add(-15 * time.Minute),
			UpdatedAt:      time.Now().Add(-5 * time.Minute),
			Duration:       &duration,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"jobs", "get", repoID.String(), jobID.String(), "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var response client.Response
	json.Unmarshal(stdout.Bytes(), &response)

	dataJSON, _ := json.Marshal(response.Data)
	var jobData dto.IndexingJobResponse
	json.Unmarshal(dataJSON, &jobData)

	// Verify all fields
	assert.Equal(t, jobID, jobData.ID)
	assert.Equal(t, repoID, jobData.RepositoryID)
	assert.Equal(t, "completed", jobData.Status)
	assert.NotNil(t, jobData.StartedAt, "started_at should be present")
	assert.NotNil(t, jobData.CompletedAt, "completed_at should be present")
	assert.NotNil(t, jobData.ErrorMessage, "error_message should be present")
	assert.Equal(t, errorMessage, *jobData.ErrorMessage)
	assert.Equal(t, 250, jobData.FilesProcessed)
	assert.Equal(t, 1000, jobData.ChunksCreated)
	assert.NotZero(t, jobData.CreatedAt)
	assert.NotZero(t, jobData.UpdatedAt)
	assert.NotNil(t, jobData.Duration, "duration should be present")
	assert.Equal(t, duration, *jobData.Duration)
}

// TestJobsGetCmd_InProgressJob verifies retrieval of a job that is still running.
// This test ensures in-progress jobs are properly represented.
func TestJobsGetCmd_InProgressJob(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()
	startedAt := time.Now().Add(-2 * time.Minute)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.IndexingJobResponse{
			ID:             jobID,
			RepositoryID:   repoID,
			Status:         "processing",
			StartedAt:      &startedAt,
			CompletedAt:    nil,
			FilesProcessed: 50,
			ChunksCreated:  150,
			CreatedAt:      time.Now().Add(-3 * time.Minute),
			UpdatedAt:      time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"jobs", "get", repoID.String(), jobID.String(), "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var response client.Response
	json.Unmarshal(stdout.Bytes(), &response)

	dataJSON, _ := json.Marshal(response.Data)
	var jobData dto.IndexingJobResponse
	json.Unmarshal(dataJSON, &jobData)

	assert.Equal(t, "processing", jobData.Status)
	assert.NotNil(t, jobData.StartedAt, "started_at should be present for in-progress job")
	assert.Nil(t, jobData.CompletedAt, "completed_at should be nil for in-progress job")
	assert.Equal(t, 50, jobData.FilesProcessed)
	assert.Equal(t, 150, jobData.ChunksCreated)
}

// TestJobsGetCmd_CompletedJob verifies retrieval of a successfully completed job.
// This test ensures completed jobs include all final metrics.
func TestJobsGetCmd_CompletedJob(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()
	startedAt := time.Now().Add(-10 * time.Minute)
	completedAt := time.Now().Add(-1 * time.Minute)
	duration := "9m0s"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.IndexingJobResponse{
			ID:             jobID,
			RepositoryID:   repoID,
			Status:         "completed",
			StartedAt:      &startedAt,
			CompletedAt:    &completedAt,
			FilesProcessed: 500,
			ChunksCreated:  2500,
			CreatedAt:      time.Now().Add(-15 * time.Minute),
			UpdatedAt:      completedAt,
			Duration:       &duration,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"jobs", "get", repoID.String(), jobID.String(), "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var response client.Response
	json.Unmarshal(stdout.Bytes(), &response)

	dataJSON, _ := json.Marshal(response.Data)
	var jobData dto.IndexingJobResponse
	json.Unmarshal(dataJSON, &jobData)

	assert.Equal(t, "completed", jobData.Status)
	assert.NotNil(t, jobData.StartedAt)
	assert.NotNil(t, jobData.CompletedAt, "completed_at should be present for completed job")
	assert.Equal(t, 500, jobData.FilesProcessed)
	assert.Equal(t, 2500, jobData.ChunksCreated)
	assert.NotNil(t, jobData.Duration, "duration should be present for completed job")
	assert.Equal(t, duration, *jobData.Duration)
}

// TestJobsGetCmd_FailedJob verifies retrieval of a job that failed with an error message.
// This test ensures failed jobs include error details.
func TestJobsGetCmd_FailedJob(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()
	startedAt := time.Now().Add(-5 * time.Minute)
	completedAt := time.Now().Add(-2 * time.Minute)
	errorMessage := "Git clone failed: repository not found"
	duration := "3m0s"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.IndexingJobResponse{
			ID:             jobID,
			RepositoryID:   repoID,
			Status:         "failed",
			StartedAt:      &startedAt,
			CompletedAt:    &completedAt,
			ErrorMessage:   &errorMessage,
			FilesProcessed: 0,
			ChunksCreated:  0,
			CreatedAt:      time.Now().Add(-6 * time.Minute),
			UpdatedAt:      completedAt,
			Duration:       &duration,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"jobs", "get", repoID.String(), jobID.String(), "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var response client.Response
	json.Unmarshal(stdout.Bytes(), &response)

	dataJSON, _ := json.Marshal(response.Data)
	var jobData dto.IndexingJobResponse
	json.Unmarshal(dataJSON, &jobData)

	assert.Equal(t, "failed", jobData.Status)
	assert.NotNil(t, jobData.ErrorMessage, "error_message should be present for failed job")
	assert.Equal(t, errorMessage, *jobData.ErrorMessage)
	assert.Equal(t, 0, jobData.FilesProcessed)
	assert.Equal(t, 0, jobData.ChunksCreated)
	assert.NotNil(t, jobData.CompletedAt, "completed_at should be present for failed job")
	assert.NotNil(t, jobData.Duration)
}

// TestJobsGetCmd_NotFound verifies error JSON output when job is not found.
// This test ensures proper error handling for non-existent jobs.
func TestJobsGetCmd_NotFound(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Job not found"))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"jobs", "get", repoID.String(), jobID.String(), "--api-url", server.URL})

	// Execute command
	_ = rootCmd.Execute()

	// Parse and verify JSON error output
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on error")

	assert.False(t, response.Success, "response should indicate failure")
	assert.Nil(t, response.Data, "response should not contain data on error")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotEmpty(t, response.Error.Code, "error should have a code")
	assert.Contains(t, response.Error.Message, "404", "error message should mention 404")
}

// TestJobsGetCmd_ServerError verifies error JSON output when server returns an error.
// This test ensures proper error handling for server-side errors.
func TestJobsGetCmd_ServerError(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"jobs", "get", repoID.String(), jobID.String(), "--api-url", server.URL})

	// Execute command
	_ = rootCmd.Execute()

	// Parse and verify JSON error output
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on error")

	assert.False(t, response.Success, "response should indicate failure")
	assert.Nil(t, response.Data, "response should not contain data on error")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotZero(t, response.Timestamp, "response should have timestamp")
	assert.NotEmpty(t, response.Error.Code, "error should have a code")
	assert.NotEmpty(t, response.Error.Message, "error should have a message")
	assert.Contains(t, response.Error.Message, "500", "error message should mention status code")
}

// TestJobsGetCmd_ConnectionError verifies error JSON output when connection fails.
// This test ensures network errors are properly handled and formatted.
func TestJobsGetCmd_ConnectionError(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()
	invalidURL := "http://localhost:1"

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{
		"jobs", "get", repoID.String(), jobID.String(),
		"--api-url", invalidURL,
		"--timeout", "100ms",
	})

	// Execute command
	_ = rootCmd.Execute()

	// Parse and verify JSON error output
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on connection error")

	assert.False(t, response.Success, "response should indicate failure")
	assert.Nil(t, response.Data, "response should not contain data on error")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotZero(t, response.Timestamp, "response should have timestamp")
	assert.NotEmpty(t, response.Error.Code, "error should have a code")
	assert.NotEmpty(t, response.Error.Message, "error should have a message")
	assert.Contains(t, []string{"CONNECTION_ERROR", "NETWORK_ERROR", "TIMEOUT_ERROR"},
		response.Error.Code, "error code should indicate connection/network issue")
}

// TestJobsGetCmd_UsesGlobalFlags verifies the command respects global flags.
// This test ensures --api-url and --timeout flags are properly used.
func TestJobsGetCmd_UsesGlobalFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupServer   func() *httptest.Server
		args          []string
		expectSuccess bool
	}{
		{
			name: "custom api-url flag",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
					// Server that won't be reached due to custom URL
				}))
			},
			args: []string{
				"jobs",
				"get",
				uuid.New().String(),
				uuid.New().String(),
				"--api-url",
				"http://custom.example.com",
			},
			expectSuccess: false,
		},
		{
			name: "timeout flag causes timeout",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					time.Sleep(200 * time.Millisecond)
					response := dto.IndexingJobResponse{
						ID:           uuid.New(),
						RepositoryID: uuid.New(),
						Status:       "completed",
						CreatedAt:    time.Now(),
						UpdatedAt:    time.Now(),
					}
					json.NewEncoder(w).Encode(response)
				}))
			},
			args:          []string{},
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := tt.setupServer()
			defer server.Close()

			repoID := uuid.New()
			jobID := uuid.New()

			args := tt.args
			if tt.name == "timeout flag causes timeout" {
				args = []string{
					"jobs",
					"get",
					repoID.String(),
					jobID.String(),
					"--api-url",
					server.URL,
					"--timeout",
					"50ms",
				}
			}

			var stdout bytes.Buffer
			rootCmd := commands.NewRootCmd()
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stdout)
			rootCmd.SetArgs(args)

			_ = rootCmd.Execute()

			var response client.Response
			err := json.Unmarshal(stdout.Bytes(), &response)
			require.NoError(t, err, "should output valid JSON")

			assert.Equal(t, tt.expectSuccess, response.Success, "success flag should match expected")
		})
	}
}

// TestJobsGetCmd_JSONOutput verifies JSON structure matches Response envelope.
// This test ensures all fields are properly serialized and follow the expected schema.
func TestJobsGetCmd_JSONOutput(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()
	startedAt := time.Now().Add(-5 * time.Minute)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.IndexingJobResponse{
			ID:             jobID,
			RepositoryID:   repoID,
			Status:         "completed",
			StartedAt:      &startedAt,
			FilesProcessed: 100,
			ChunksCreated:  300,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"jobs", "get", repoID.String(), jobID.String(), "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var rawJSON map[string]interface{}
	err = json.Unmarshal(stdout.Bytes(), &rawJSON)
	require.NoError(t, err, "output should be valid JSON")

	assert.Contains(t, rawJSON, "success", "JSON should have 'success' field")
	assert.Contains(t, rawJSON, "timestamp", "JSON should have 'timestamp' field")

	success, ok := rawJSON["success"].(bool)
	require.True(t, ok, "'success' should be a boolean")
	if success {
		assert.Contains(t, rawJSON, "data", "successful response should have 'data' field")
		assert.NotContains(t, rawJSON, "error", "successful response should not have 'error' field")
	} else {
		assert.Contains(t, rawJSON, "error", "failed response should have 'error' field")
		assert.NotContains(t, rawJSON, "data", "failed response should not have 'data' field")
	}

	timestampStr, ok := rawJSON["timestamp"].(string)
	require.True(t, ok, "'timestamp' should be a string")
	_, err = time.Parse(time.RFC3339Nano, timestampStr)
	assert.NoError(t, err, "timestamp should be valid RFC3339 format")
}

// TestJobsGetCmd_OutputToStdout verifies output is written to stdout, not stderr.
// This test ensures proper stream usage for JSON output.
func TestJobsGetCmd_OutputToStdout(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.IndexingJobResponse{
			ID:           jobID,
			RepositoryID: repoID,
			Status:       "completed",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"jobs", "get", repoID.String(), jobID.String(), "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	assert.NotEmpty(t, stdout.String(), "stdout should contain JSON output")

	stderrContent := stderr.String()
	if stderrContent != "" {
		var response client.Response
		err := json.Unmarshal(stderr.Bytes(), &response)
		assert.Error(t, err, "stderr should not contain JSON response - only stdout should")
	}
}

// TestJobsGetCmd_Integration verifies end-to-end integration.
// This test simulates a real usage scenario with all components.
func TestJobsGetCmd_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()
	startedAt := time.Now().Add(-10 * time.Minute)
	completedAt := time.Now().Add(-2 * time.Minute)
	duration := "8m0s"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf("/repositories/%s/jobs/%s", repoID.String(), jobID.String())
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.NotEmpty(t, r.Header.Get("User-Agent"))

		response := dto.IndexingJobResponse{
			ID:             jobID,
			RepositoryID:   repoID,
			Status:         "completed",
			StartedAt:      &startedAt,
			CompletedAt:    &completedAt,
			FilesProcessed: 350,
			ChunksCreated:  1500,
			CreatedAt:      time.Now().Add(-15 * time.Minute),
			UpdatedAt:      completedAt,
			Duration:       &duration,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs(
		[]string{"jobs", "get", repoID.String(), jobID.String(), "--api-url", server.URL, "--timeout", "5s"},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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

	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "should output valid JSON")
	assert.True(t, response.Success, "integration test should succeed")
	assert.NotNil(t, response.Data, "should contain job data")
	assert.Nil(t, response.Error, "should not contain error")

	dataJSON, _ := json.Marshal(response.Data)
	var jobData dto.IndexingJobResponse
	json.Unmarshal(dataJSON, &jobData)
	assert.Equal(t, jobID, jobData.ID)
	assert.Equal(t, repoID, jobData.RepositoryID)
	assert.Equal(t, "completed", jobData.Status)
	assert.Equal(t, 350, jobData.FilesProcessed)
	assert.Equal(t, 1500, jobData.ChunksCreated)
}
