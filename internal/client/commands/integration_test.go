//go:build integration
// +build integration

package commands_test

import (
	"bytes"
	"codechunking/internal/application/dto"
	"codechunking/internal/client"
	"codechunking/internal/client/commands"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClientIntegration_HealthToSearch tests a complete workflow from health check to search.
// This test verifies that the CLI can successfully execute multiple commands in sequence
// with a shared mock server, demonstrating the integration of health check followed by
// a semantic search operation. This scenario is common when users first verify server
// health before performing searches.
func TestClientIntegration_HealthToSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Track which endpoints were called to verify workflow order
	var callOrder []string

	// Create mock server that handles multiple endpoints
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, r.URL.Path)

		switch r.URL.Path {
		case "/health":
			response := dto.HealthResponse{
				Status:    "healthy",
				Timestamp: time.Now(),
				Version:   "1.0.0",
				Dependencies: map[string]dto.DependencyStatus{
					"database": {Status: "healthy", Message: "Connected"},
					"nats":     {Status: "healthy", Message: "Connected"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			assert.NoError(t, json.NewEncoder(w).Encode(response))

		case "/search":
			// Verify this is a POST request with JSON body
			assert.Equal(t, http.MethodPost, r.Method, "search should use POST method")
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			// Parse request body
			var searchReq dto.SearchRequestDTO
			err := json.NewDecoder(r.Body).Decode(&searchReq)
			assert.NoError(t, err, "should decode search request")
			assert.NotEmpty(t, searchReq.Query, "search query should not be empty")

			// Return search results
			repoID := uuid.New()
			response := dto.SearchResponseDTO{
				Results: []dto.SearchResultDTO{
					{
						ChunkID:         uuid.New(),
						Content:         "function processData() { return true; }",
						Language:        "javascript",
						FilePath:        "/src/processor.js",
						SimilarityScore: 0.95,
						Repository: dto.RepositoryInfo{
							ID:   repoID,
							Name: "test/repo",
							URL:  "https://github.com/test/repo",
						},
					},
				},
				Pagination: dto.PaginationResponse{
					Limit:   10,
					Offset:  0,
					Total:   1,
					HasMore: false,
				},
				Metadata: dto.SearchMetadata{
					Query:           searchReq.Query,
					ExecutionTimeMs: 50,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			assert.NoError(t, json.NewEncoder(w).Encode(response))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: Execute health check command
	var healthOut bytes.Buffer
	healthCmd := commands.NewRootCmd()
	healthCmd.SetOut(&healthOut)
	healthCmd.SetErr(&healthOut)
	healthCmd.SetArgs([]string{"health", "--api-url", server.URL, "--timeout", "5s"})

	err := healthCmd.ExecuteContext(ctx)
	require.NoError(t, err, "health command should succeed")

	// Verify health response
	var healthResponse client.Response
	err = json.Unmarshal(healthOut.Bytes(), &healthResponse)
	require.NoError(t, err, "health output should be valid JSON")
	assert.True(t, healthResponse.Success, "health check should succeed")
	assert.Nil(t, healthResponse.Error, "health check should not have errors")

	// Step 2: Execute search command
	var searchOut bytes.Buffer
	searchCmd := commands.NewRootCmd()
	searchCmd.SetOut(&searchOut)
	searchCmd.SetErr(&searchOut)
	searchCmd.SetArgs([]string{
		"search", "authentication function",
		"--api-url", server.URL,
		"--timeout", "5s",
		"--limit", "10",
		"--threshold", "0.7",
	})

	err = searchCmd.ExecuteContext(ctx)
	require.NoError(t, err, "search command should succeed")

	// Verify search response
	var searchResponse client.Response
	err = json.Unmarshal(searchOut.Bytes(), &searchResponse)
	require.NoError(t, err, "search output should be valid JSON")
	assert.True(t, searchResponse.Success, "search should succeed")
	assert.Nil(t, searchResponse.Error, "search should not have errors")
	assert.NotNil(t, searchResponse.Data, "search should return data")

	// Verify workflow order
	require.Len(t, callOrder, 2, "should have called 2 endpoints")
	assert.Equal(t, "/health", callOrder[0], "first call should be health")
	assert.Equal(t, "/search", callOrder[1], "second call should be search")

	// Verify search results structure
	dataJSON, err := json.Marshal(searchResponse.Data)
	require.NoError(t, err)
	var searchData dto.SearchResponseDTO
	err = json.Unmarshal(dataJSON, &searchData)
	require.NoError(t, err, "search data should be valid SearchResponseDTO")
	assert.NotEmpty(t, searchData.Results, "should have search results")
	assert.Equal(t, 1, searchData.Pagination.Total, "should have correct total count")
	assert.GreaterOrEqual(t, searchData.Results[0].SimilarityScore, 0.7, "similarity should meet threshold")
}

// TestClientIntegration_RepoLifecycle tests the complete repository lifecycle workflow.
// This test verifies:
// 1. Adding a new repository (POST /repositories)
// 2. Retrieving the repository by ID (GET /repositories/:id)
// 3. Listing repositories with the new repo included (GET /repositories)
// This scenario demonstrates a common user workflow when indexing a new codebase.
func TestClientIntegration_RepoLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Store created repository for GET and LIST verification
	var createdRepo dto.RepositoryResponse
	description := "Integration test repository"
	branch := "main"

	// Create mock server that handles repository operations
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repositories":
			if r.Method == http.MethodPost {
				// Handle repository creation
				var createReq dto.CreateRepositoryRequest
				err := json.NewDecoder(r.Body).Decode(&createReq)
				assert.NoError(t, err, "should decode create request")
				assert.NotEmpty(t, createReq.URL, "URL should not be empty")

				// Create and store repository response
				createdRepo = dto.RepositoryResponse{
					ID:            uuid.New(),
					URL:           createReq.URL,
					Name:          createReq.Name,
					Description:   &description,
					DefaultBranch: &branch,
					Status:        "pending",
					TotalFiles:    0,
					TotalChunks:   0,
					CreatedAt:     time.Now(),
					UpdatedAt:     time.Now(),
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				assert.NoError(t, json.NewEncoder(w).Encode(createdRepo))

			} else if r.Method == http.MethodGet {
				// Handle repository list
				response := dto.RepositoryListResponse{
					Repositories: []dto.RepositoryResponse{createdRepo},
					Pagination: dto.PaginationResponse{
						Total:   1,
						Limit:   dto.DefaultRepositoryLimit,
						Offset:  0,
						HasMore: false,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				assert.NoError(t, json.NewEncoder(w).Encode(response))
			}

		default:
			// Handle GET /repositories/:id
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				assert.NoError(t, json.NewEncoder(w).Encode(createdRepo))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: Add repository
	var addOut bytes.Buffer
	addCmd := commands.NewRootCmd()
	addCmd.SetOut(&addOut)
	addCmd.SetErr(&addOut)
	addCmd.SetArgs([]string{
		"repos", "add", "https://github.com/test/integration-repo",
		"--api-url", server.URL,
		"--name", "integration-test",
		"--description", description,
		"--branch", branch,
		"--timeout", "5s",
	})

	err := addCmd.ExecuteContext(ctx)
	require.NoError(t, err, "repos add should succeed")

	var addResponse client.Response
	err = json.Unmarshal(addOut.Bytes(), &addResponse)
	require.NoError(t, err, "add output should be valid JSON")
	assert.True(t, addResponse.Success, "add should succeed")
	assert.NotNil(t, addResponse.Data, "add should return repository data")

	// Extract repository ID for next steps
	dataJSON, err := json.Marshal(addResponse.Data)
	require.NoError(t, err)
	var addedRepo dto.RepositoryResponse
	err = json.Unmarshal(dataJSON, &addedRepo)
	require.NoError(t, err)
	repoID := addedRepo.ID

	// Step 2: Get repository by ID
	var getOut bytes.Buffer
	getCmd := commands.NewRootCmd()
	getCmd.SetOut(&getOut)
	getCmd.SetErr(&getOut)
	getCmd.SetArgs([]string{
		"repos", "get", repoID.String(),
		"--api-url", server.URL,
		"--timeout", "5s",
	})

	err = getCmd.ExecuteContext(ctx)
	require.NoError(t, err, "repos get should succeed")

	var getResponse client.Response
	err = json.Unmarshal(getOut.Bytes(), &getResponse)
	require.NoError(t, err, "get output should be valid JSON")
	assert.True(t, getResponse.Success, "get should succeed")
	assert.NotNil(t, getResponse.Data, "get should return repository data")

	// Verify retrieved repository matches created repository
	dataJSON, err = json.Marshal(getResponse.Data)
	require.NoError(t, err)
	var retrievedRepo dto.RepositoryResponse
	err = json.Unmarshal(dataJSON, &retrievedRepo)
	require.NoError(t, err)
	assert.Equal(t, repoID, retrievedRepo.ID, "retrieved repo ID should match")
	assert.Equal(t, "integration-test", retrievedRepo.Name, "retrieved repo name should match")

	// Step 3: List repositories
	var listOut bytes.Buffer
	listCmd := commands.NewRootCmd()
	listCmd.SetOut(&listOut)
	listCmd.SetErr(&listOut)
	listCmd.SetArgs([]string{
		"repos", "list",
		"--api-url", server.URL,
		"--timeout", "5s",
		"--limit", "10",
	})

	err = listCmd.ExecuteContext(ctx)
	require.NoError(t, err, "repos list should succeed")

	var listResponse client.Response
	err = json.Unmarshal(listOut.Bytes(), &listResponse)
	require.NoError(t, err, "list output should be valid JSON")
	assert.True(t, listResponse.Success, "list should succeed")
	assert.NotNil(t, listResponse.Data, "list should return data")

	// Verify list includes created repository
	dataJSON, err = json.Marshal(listResponse.Data)
	require.NoError(t, err)
	var listData dto.RepositoryListResponse
	err = json.Unmarshal(dataJSON, &listData)
	require.NoError(t, err)
	assert.NotEmpty(t, listData.Repositories, "list should include repositories")
	assert.Equal(t, 1, listData.Pagination.Total, "should have 1 total repository")

	// Verify JSON output envelope correctness for all three operations
	assert.NotZero(t, addResponse.Timestamp, "add response should have timestamp")
	assert.NotZero(t, getResponse.Timestamp, "get response should have timestamp")
	assert.NotZero(t, listResponse.Timestamp, "list response should have timestamp")
}

// TestClientIntegration_SearchWithFilters tests search with multiple filter flags.
// This test verifies that all filter parameters are correctly passed to the API
// and that the JSON output envelope is properly formatted. Tests include:
// - Repository ID filtering
// - Language filtering
// - File type filtering
// - Similarity threshold
// - Pagination (limit/offset)
// - Sort order
func TestClientIntegration_SearchWithFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoID := uuid.New()
	var receivedRequest dto.SearchRequestDTO

	// Create mock server that captures search request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Capture search request for verification
		err := json.NewDecoder(r.Body).Decode(&receivedRequest)
		assert.NoError(t, err, "should decode search request")

		// Return filtered results
		response := dto.SearchResponseDTO{
			Results: []dto.SearchResultDTO{
				{
					ChunkID:         uuid.New(),
					Content:         "def authenticate(user): return True",
					Language:        "python",
					FilePath:        "/auth/validator.py",
					SimilarityScore: 0.92,
					Type:            "function",
					Repository: dto.RepositoryInfo{
						ID:   repoID,
						Name: "test/auth-service",
						URL:  "https://github.com/test/auth-service",
					},
				},
			},
			Pagination: dto.PaginationResponse{
				Total:   1,
				Limit:   20,
				Offset:  5,
				HasMore: false,
			},
			Metadata: dto.SearchMetadata{
				Query:           receivedRequest.Query,
				ExecutionTimeMs: 45,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute search with comprehensive filters
	var searchOut bytes.Buffer
	searchCmd := commands.NewRootCmd()
	searchCmd.SetOut(&searchOut)
	searchCmd.SetErr(&searchOut)
	searchCmd.SetArgs([]string{
		"search", "authentication logic",
		"--api-url", server.URL,
		"--timeout", "5s",
		"--limit", "20",
		"--offset", "5",
		"--repo-ids", repoID.String(),
		"--repo-names", "test/auth-service,test/user-service",
		"--languages", "python,go",
		"--file-types", ".py,.go",
		"--threshold", "0.85",
		"--sort", "relevance",
		"--types", "function,method",
		"--entity-name", "authenticate",
		"--visibility", "public",
	})

	err := searchCmd.ExecuteContext(ctx)
	require.NoError(t, err, "search command should succeed")

	// Verify JSON output
	var response client.Response
	err = json.Unmarshal(searchOut.Bytes(), &response)
	require.NoError(t, err, "search output should be valid JSON")
	assert.True(t, response.Success, "search should succeed")
	assert.NotNil(t, response.Data, "search should return data")
	assert.Nil(t, response.Error, "search should not have errors")
	assert.NotZero(t, response.Timestamp, "response should have timestamp")

	// Verify all filters were sent to API
	assert.Equal(t, "authentication logic", receivedRequest.Query, "query should match")
	assert.Equal(t, 20, receivedRequest.Limit, "limit should match")
	assert.Equal(t, 5, receivedRequest.Offset, "offset should match")
	assert.Contains(t, receivedRequest.RepositoryIDs, repoID, "repo ID should be included")
	assert.ElementsMatch(t, []string{"test/auth-service", "test/user-service"},
		receivedRequest.RepositoryNames, "repo names should match")
	assert.ElementsMatch(t, []string{"python", "go"},
		receivedRequest.Languages, "languages should match")
	assert.ElementsMatch(t, []string{".py", ".go"},
		receivedRequest.FileTypes, "file types should match")
	assert.Equal(t, 0.85, receivedRequest.SimilarityThreshold, "threshold should match")
	assert.Equal(t, "relevance", receivedRequest.Sort, "sort should match")
	assert.ElementsMatch(t, []string{"function", "method"},
		receivedRequest.Types, "types should match")
	assert.Equal(t, "authenticate", receivedRequest.EntityName, "entity name should match")
	assert.ElementsMatch(t, []string{"public"},
		receivedRequest.Visibility, "visibility should match")

	// Verify search results structure
	dataJSON, err := json.Marshal(response.Data)
	require.NoError(t, err)
	var searchData dto.SearchResponseDTO
	err = json.Unmarshal(dataJSON, &searchData)
	require.NoError(t, err)
	assert.NotEmpty(t, searchData.Results, "should have results")
	assert.Equal(t, "python", searchData.Results[0].Language, "language should be python")
	assert.GreaterOrEqual(t, searchData.Results[0].SimilarityScore, 0.85,
		"similarity should meet threshold")
}

// TestClientIntegration_JobStatusCheck tests the workflow of adding a repo and checking job status.
// This test verifies:
// 1. Repository creation returns a repository ID
// 2. Job can be retrieved using repository ID and job ID
// 3. Job status information is correctly formatted
// This scenario is common when monitoring indexing progress.
func TestClientIntegration_JobStatusCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoID := uuid.New()
	jobID := uuid.New()
	description := "Test repository"
	branch := "main"

	// Create mock server that handles repo creation and job retrieval
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/repositories":
			// Handle repository creation
			response := dto.RepositoryResponse{
				ID:            repoID,
				URL:           "https://github.com/test/repo",
				Name:          "test/repo",
				Description:   &description,
				DefaultBranch: &branch,
				Status:        "pending",
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}
			w.WriteHeader(http.StatusCreated)
			assert.NoError(t, json.NewEncoder(w).Encode(response))

		default:
			// Handle job retrieval /repositories/:id/jobs/:jobId
			if r.Method == http.MethodGet {
				startedAt := time.Now().Add(-4 * time.Minute)
				response := dto.IndexingJobResponse{
					ID:             jobID,
					RepositoryID:   repoID,
					Status:         "processing",
					CreatedAt:      time.Now().Add(-5 * time.Minute),
					UpdatedAt:      time.Now(),
					StartedAt:      &startedAt,
					FilesProcessed: 45,
					ChunksCreated:  234,
				}
				w.WriteHeader(http.StatusOK)
				assert.NoError(t, json.NewEncoder(w).Encode(response))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: Add repository
	var addOut bytes.Buffer
	addCmd := commands.NewRootCmd()
	addCmd.SetOut(&addOut)
	addCmd.SetErr(&addOut)
	addCmd.SetArgs([]string{
		"repos", "add", "https://github.com/test/repo",
		"--api-url", server.URL,
		"--name", "test/repo",
		"--timeout", "5s",
	})

	err := addCmd.ExecuteContext(ctx)
	require.NoError(t, err, "repos add should succeed")

	var addResponse client.Response
	err = json.Unmarshal(addOut.Bytes(), &addResponse)
	require.NoError(t, err, "add output should be valid JSON")
	assert.True(t, addResponse.Success, "add should succeed")

	// Step 2: Get job status
	var jobOut bytes.Buffer
	jobCmd := commands.NewRootCmd()
	jobCmd.SetOut(&jobOut)
	jobCmd.SetErr(&jobOut)
	jobCmd.SetArgs([]string{
		"jobs", "get", repoID.String(), jobID.String(),
		"--api-url", server.URL,
		"--timeout", "5s",
	})

	err = jobCmd.ExecuteContext(ctx)
	require.NoError(t, err, "jobs get should succeed")

	// Verify job response
	var jobResponse client.Response
	err = json.Unmarshal(jobOut.Bytes(), &jobResponse)
	require.NoError(t, err, "job output should be valid JSON")
	assert.True(t, jobResponse.Success, "job retrieval should succeed")
	assert.NotNil(t, jobResponse.Data, "job should return data")
	assert.Nil(t, jobResponse.Error, "job should not have errors")
	assert.NotZero(t, jobResponse.Timestamp, "response should have timestamp")

	// Verify job details
	dataJSON, err := json.Marshal(jobResponse.Data)
	require.NoError(t, err)
	var jobData dto.IndexingJobResponse
	err = json.Unmarshal(dataJSON, &jobData)
	require.NoError(t, err)
	assert.Equal(t, jobID, jobData.ID, "job ID should match")
	assert.Equal(t, repoID, jobData.RepositoryID, "repository ID should match")
	assert.Equal(t, "processing", jobData.Status, "job should be processing")
	assert.NotNil(t, jobData.StartedAt, "job should have started_at timestamp")
	assert.Equal(t, 45, jobData.FilesProcessed, "files processed should match")
	assert.Equal(t, 234, jobData.ChunksCreated, "chunks created should match")
}

// TestClientIntegration_ErrorScenarios tests various error conditions and error code classification.
// This test verifies that different HTTP status codes and error conditions result in
// appropriate error codes in the JSON output envelope:
// - 404 Not Found -> NOT_FOUND
// - 500 Internal Server Error -> SERVER_ERROR
// - Connection refused -> CONNECTION_ERROR
// - Request timeout -> TIMEOUT_ERROR
// - Invalid UUID -> INVALID_ARGUMENT
func TestClientIntegration_ErrorScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("404 Not Found returns NOT_FOUND error code", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Repository not found"))
		}))
		defer server.Close()

		var out bytes.Buffer
		cmd := commands.NewRootCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{
			"repos", "get", uuid.New().String(),
			"--api-url", server.URL,
			"--timeout", "5s",
		})

		_ = cmd.ExecuteContext(ctx)

		var response client.Response
		err := json.Unmarshal(out.Bytes(), &response)
		require.NoError(t, err, "output should be valid JSON")
		assert.False(t, response.Success, "should indicate failure")
		assert.NotNil(t, response.Error, "should have error")
		assert.Equal(t, "NOT_FOUND", response.Error.Code, "error code should be NOT_FOUND")
		assert.Contains(t, response.Error.Message, "404", "error message should mention 404")
	})

	t.Run("500 Internal Server Error returns SERVER_ERROR error code", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal server error"))
		}))
		defer server.Close()

		var out bytes.Buffer
		cmd := commands.NewRootCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{
			"health",
			"--api-url", server.URL,
			"--timeout", "5s",
		})

		_ = cmd.ExecuteContext(ctx)

		var response client.Response
		err := json.Unmarshal(out.Bytes(), &response)
		require.NoError(t, err, "output should be valid JSON")
		assert.False(t, response.Success, "should indicate failure")
		assert.NotNil(t, response.Error, "should have error")
		assert.Equal(t, "SERVER_ERROR", response.Error.Code, "error code should be SERVER_ERROR")
		assert.Contains(t, response.Error.Message, "500", "error message should mention 500")
	})

	t.Run("Connection refused returns CONNECTION_ERROR error code", func(t *testing.T) {
		// Use port 1 which should be closed
		invalidURL := "http://localhost:1"

		var out bytes.Buffer
		cmd := commands.NewRootCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{
			"health",
			"--api-url", invalidURL,
			"--timeout", "1s",
		})

		_ = cmd.ExecuteContext(ctx)

		var response client.Response
		err := json.Unmarshal(out.Bytes(), &response)
		require.NoError(t, err, "output should be valid JSON")
		assert.False(t, response.Success, "should indicate failure")
		assert.NotNil(t, response.Error, "should have error")
		assert.Equal(t, "CONNECTION_ERROR", response.Error.Code,
			"error code should be CONNECTION_ERROR")
	})

	t.Run("Request timeout returns TIMEOUT_ERROR error code", func(t *testing.T) {
		// Server that delays longer than client timeout
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer server.Close()

		var out bytes.Buffer
		cmd := commands.NewRootCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{
			"health",
			"--api-url", server.URL,
			"--timeout", "10ms",
		})

		_ = cmd.ExecuteContext(ctx)

		var response client.Response
		err := json.Unmarshal(out.Bytes(), &response)
		require.NoError(t, err, "output should be valid JSON")
		assert.False(t, response.Success, "should indicate failure")
		assert.NotNil(t, response.Error, "should have error")
		assert.Contains(t, []string{"TIMEOUT_ERROR", "CONNECTION_ERROR"},
			response.Error.Code, "error code should indicate timeout or connection issue")
	})

	t.Run("Invalid UUID argument returns INVALID_ARGUMENT error code", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		var out bytes.Buffer
		cmd := commands.NewRootCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{
			"repos", "get", "not-a-valid-uuid",
			"--api-url", server.URL,
			"--timeout", "5s",
		})

		_ = cmd.ExecuteContext(ctx)

		var response client.Response
		err := json.Unmarshal(out.Bytes(), &response)
		require.NoError(t, err, "output should be valid JSON")
		assert.False(t, response.Success, "should indicate failure")
		assert.NotNil(t, response.Error, "should have error")
		assert.Equal(t, "INVALID_ARGUMENT", response.Error.Code,
			"error code should be INVALID_ARGUMENT")
		assert.Contains(t, response.Error.Message, "UUID", "error message should mention UUID")
	})

	t.Run("Invalid API URL returns INVALID_CONFIG error code", func(t *testing.T) {
		var out bytes.Buffer
		cmd := commands.NewRootCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{
			"health",
			"--api-url", "not-a-valid-url",
			"--timeout", "5s",
		})

		_ = cmd.ExecuteContext(ctx)

		var response client.Response
		err := json.Unmarshal(out.Bytes(), &response)
		require.NoError(t, err, "output should be valid JSON")
		assert.False(t, response.Success, "should indicate failure")
		assert.NotNil(t, response.Error, "should have error")
		assert.Equal(t, "INVALID_CONFIG", response.Error.Code,
			"error code should be INVALID_CONFIG")
	})
}

// TestClientIntegration_FlagInheritance tests that global flags are inherited by subcommands.
// This test verifies that --api-url and --timeout flags set on the root command
// are properly passed down to all subcommands (health, repos, search, jobs).
// This is critical for user experience as users should be able to set global
// configuration once rather than repeating it for every subcommand.
func TestClientIntegration_FlagInheritance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Track requests to verify API URL was used
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")

		// Return minimal valid response for any endpoint
		switch r.URL.Path {
		case "/health":
			response := dto.HealthResponse{
				Status:    "healthy",
				Timestamp: time.Now(),
				Version:   "1.0.0",
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		case "/repositories":
			response := dto.RepositoryListResponse{
				Repositories: []dto.RepositoryResponse{},
				Pagination: dto.PaginationResponse{
					Total:  0,
					Limit:  dto.DefaultRepositoryLimit,
					Offset: 0,
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		case "/search":
			response := dto.SearchResponseDTO{
				Results: []dto.SearchResultDTO{},
				Pagination: dto.PaginationResponse{
					Total: 0,
				},
				Metadata: dto.SearchMetadata{
					Query: "",
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "health command inherits global flags",
			args: []string{"health", "--api-url", server.URL, "--timeout", "5s"},
		},
		{
			name: "repos list inherits global flags",
			args: []string{"repos", "list", "--api-url", server.URL, "--timeout", "5s"},
		},
		{
			name: "search inherits global flags",
			args: []string{"search", "test query", "--api-url", server.URL, "--timeout", "5s"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestCount = 0 // Reset counter

			var out bytes.Buffer
			cmd := commands.NewRootCmd()
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tt.args)

			err := cmd.ExecuteContext(ctx)
			require.NoError(t, err, "command should succeed")

			var response client.Response
			err = json.Unmarshal(out.Bytes(), &response)
			require.NoError(t, err, "output should be valid JSON")
			assert.True(t, response.Success, "should succeed")

			// Verify the server received the request (flag inheritance worked)
			assert.Equal(t, 1, requestCount, "server should have received exactly one request")
		})
	}
}

// TestClientIntegration_RepoAddWithWait tests repository addition with --wait flag.
// This test verifies that when --wait is specified:
// 1. Repository is created successfully
// 2. CLI polls for status updates at the specified interval
// 3. CLI waits until repository reaches 'completed' or 'failed' status
// 4. Polling respects the --wait-timeout duration
// 5. Final output includes the completed repository state
func TestClientIntegration_RepoAddWithWait(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoID := uuid.New()
	description := "Test repository"
	branch := "main"
	pollCount := 0
	statuses := []string{"pending", "cloning", "processing", "completed"}

	// Create mock server that simulates repository status progression
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost && r.URL.Path == "/repositories" {
			// Handle repository creation
			response := dto.RepositoryResponse{
				ID:            repoID,
				URL:           "https://github.com/test/wait-repo",
				Name:          "test/wait-repo",
				Description:   &description,
				DefaultBranch: &branch,
				Status:        "pending",
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}
			w.WriteHeader(http.StatusCreated)
			assert.NoError(t, json.NewEncoder(w).Encode(response))

		} else if r.Method == http.MethodGet {
			// Handle status polling - progress through statuses
			statusIndex := pollCount
			if statusIndex >= len(statuses) {
				statusIndex = len(statuses) - 1
			}

			status := statuses[statusIndex]
			pollCount++

			lastIndexed := time.Now()
			response := dto.RepositoryResponse{
				ID:            repoID,
				URL:           "https://github.com/test/wait-repo",
				Name:          "test/wait-repo",
				Description:   &description,
				DefaultBranch: &branch,
				Status:        status,
				TotalFiles:    100,
				TotalChunks:   500,
				LastIndexedAt: &lastIndexed,
				CreatedAt:     time.Now().Add(-5 * time.Minute),
				UpdatedAt:     time.Now(),
			}
			w.WriteHeader(http.StatusOK)
			assert.NoError(t, json.NewEncoder(w).Encode(response))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Execute repos add with --wait flag
	var out, errOut bytes.Buffer
	cmd := commands.NewRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{
		"repos", "add", "https://github.com/test/wait-repo",
		"--api-url", server.URL,
		"--name", "test/wait-repo",
		"--wait",
		"--poll-interval", "100ms",
		"--wait-timeout", "10s",
		"--timeout", "5s",
	})

	err := cmd.ExecuteContext(ctx)
	require.NoError(t, err, "repos add with wait should succeed")

	// Verify final response shows completed status
	var response client.Response
	err = json.Unmarshal(out.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON")
	assert.True(t, response.Success, "should succeed")
	assert.NotNil(t, response.Data, "should return repository data")

	dataJSON, err := json.Marshal(response.Data)
	require.NoError(t, err)
	var repo dto.RepositoryResponse
	err = json.Unmarshal(dataJSON, &repo)
	require.NoError(t, err)
	assert.Equal(t, "completed", repo.Status, "final status should be completed")
	assert.Greater(t, pollCount, 1, "should have polled multiple times")
}

// TestClientIntegration_OutputToStdout verifies all commands write to stdout, not stderr.
// This test ensures consistent output stream usage across all CLI commands for proper
// piping and redirection in shell scripts. JSON output must always go to stdout.
func TestClientIntegration_OutputToStdout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		switch r.URL.Path {
		case "/health":
			response := dto.HealthResponse{Status: "healthy", Timestamp: time.Now(), Version: "1.0.0"}
			json.NewEncoder(w).Encode(response)
		case "/repositories":
			response := dto.RepositoryListResponse{
				Repositories: []dto.RepositoryResponse{},
				Pagination:   dto.PaginationResponse{Total: 0},
			}
			json.NewEncoder(w).Encode(response)
		case "/search":
			response := dto.SearchResponseDTO{
				Results:    []dto.SearchResultDTO{},
				Pagination: dto.PaginationResponse{Total: 0},
			}
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tests := []struct {
		name string
		args []string
	}{
		{"health", []string{"health", "--api-url", server.URL}},
		{"repos list", []string{"repos", "list", "--api-url", server.URL}},
		{"search", []string{"search", "test", "--api-url", server.URL}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := commands.NewRootCmd()
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(tt.args)

			err := cmd.ExecuteContext(ctx)
			require.NoError(t, err)

			// JSON output must be on stdout
			assert.NotEmpty(t, stdout.String(), "stdout should contain output")

			// Verify it's valid JSON
			var response client.Response
			err = json.Unmarshal(stdout.Bytes(), &response)
			require.NoError(t, err, "stdout should contain valid JSON")

			// stderr should not contain JSON response
			if stderr.Len() > 0 {
				var stderrResponse client.Response
				err = json.Unmarshal(stderr.Bytes(), &stderrResponse)
				assert.Error(t, err, "stderr should not contain JSON response")
			}
		})
	}
}

// TestClientIntegration_ConcurrentCommands tests multiple commands executing concurrently.
// This test verifies that the CLI can handle concurrent execution without race conditions
// or resource conflicts. This is important for users running multiple CLI commands in
// parallel scripts or automation workflows.
func TestClientIntegration_ConcurrentCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		switch r.URL.Path {
		case "/health":
			response := dto.HealthResponse{Status: "healthy", Timestamp: time.Now(), Version: "1.0.0"}
			json.NewEncoder(w).Encode(response)
		case "/repositories":
			response := dto.RepositoryListResponse{
				Repositories: []dto.RepositoryResponse{},
				Pagination:   dto.PaginationResponse{Total: 0},
			}
			json.NewEncoder(w).Encode(response)
		case "/search":
			response := dto.SearchResponseDTO{
				Results:    []dto.SearchResultDTO{},
				Pagination: dto.PaginationResponse{Total: 0},
			}
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run multiple commands concurrently
	concurrency := 5
	errChan := make(chan error, concurrency)
	outputs := make([]io.Reader, concurrency)

	for i := 0; i < concurrency; i++ {
		i := i // Capture loop variable
		go func() {
			var out bytes.Buffer
			outputs[i] = &out

			cmd := commands.NewRootCmd()
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"health", "--api-url", server.URL})

			errChan <- cmd.ExecuteContext(ctx)
		}()
	}

	// Wait for all commands to complete
	for i := 0; i < concurrency; i++ {
		err := <-errChan
		require.NoError(t, err, "concurrent command should succeed")
	}

	// Verify all commands received responses
	assert.GreaterOrEqual(t, requestCount, concurrency,
		"server should have received at least one request per command")
}

// TestClientIntegration_JSONEnvelopeConsistency tests JSON envelope consistency across all commands.
// This test verifies that every CLI command output follows the same JSON structure:
// - success: boolean
// - data: object (on success)
// - error: object with code/message/details (on failure)
// - timestamp: RFC3339 formatted string
// This consistency is critical for client applications parsing CLI output.
func TestClientIntegration_JSONEnvelopeConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		switch r.URL.Path {
		case "/health":
			response := dto.HealthResponse{Status: "healthy", Timestamp: time.Now(), Version: "1.0.0"}
			json.NewEncoder(w).Encode(response)
		case "/repositories":
			if r.Method == http.MethodGet {
				response := dto.RepositoryListResponse{
					Repositories: []dto.RepositoryResponse{},
					Pagination:   dto.PaginationResponse{Total: 0},
				}
				json.NewEncoder(w).Encode(response)
			} else if r.Method == http.MethodPost {
				desc := "test"
				response := dto.RepositoryResponse{
					ID: uuid.New(), URL: "https://github.com/test/repo", Name: "test",
					Description: &desc, Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now(),
				}
				json.NewEncoder(w).Encode(response)
			}
		case "/search":
			response := dto.SearchResponseDTO{
				Results:    []dto.SearchResultDTO{},
				Pagination: dto.PaginationResponse{Total: 0},
			}
			json.NewEncoder(w).Encode(response)
		default:
			if r.Method == http.MethodGet {
				response := dto.RepositoryResponse{
					ID: uuid.New(), URL: "https://github.com/test/repo", Name: "test",
					Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now(),
				}
				json.NewEncoder(w).Encode(response)
			}
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testCases := []struct {
		name string
		args []string
	}{
		{"health", []string{"health", "--api-url", server.URL}},
		{"repos list", []string{"repos", "list", "--api-url", server.URL}},
		{"repos get", []string{"repos", "get", uuid.New().String(), "--api-url", server.URL}},
		{
			"repos add",
			[]string{"repos", "add", "https://github.com/test/repo", "--api-url", server.URL, "--name", "test"},
		},
		{"search", []string{"search", "test query", "--api-url", server.URL}},
		{"jobs get", []string{"jobs", "get", uuid.New().String(), uuid.New().String(), "--api-url", server.URL}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := commands.NewRootCmd()
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tc.args)

			_ = cmd.ExecuteContext(ctx)

			// Parse as raw JSON to verify structure
			var rawJSON map[string]interface{}
			err := json.Unmarshal(out.Bytes(), &rawJSON)
			require.NoError(t, err, "output must be valid JSON")

			// Verify required fields exist
			assert.Contains(t, rawJSON, "success", "must have 'success' field")
			assert.Contains(t, rawJSON, "timestamp", "must have 'timestamp' field")

			// Verify success is boolean
			success, ok := rawJSON["success"].(bool)
			require.True(t, ok, "'success' must be boolean")

			// Verify mutually exclusive data/error fields
			if success {
				assert.Contains(t, rawJSON, "data", "success response must have 'data'")
				assert.NotContains(t, rawJSON, "error", "success response must not have 'error'")
			} else {
				assert.Contains(t, rawJSON, "error", "error response must have 'error'")
				assert.NotContains(t, rawJSON, "data", "error response must not have 'data'")

				// Verify error structure
				errorObj, ok := rawJSON["error"].(map[string]interface{})
				require.True(t, ok, "'error' must be object")
				assert.Contains(t, errorObj, "code", "error must have 'code'")
				assert.Contains(t, errorObj, "message", "error must have 'message'")
			}

			// Verify timestamp format
			timestampStr, ok := rawJSON["timestamp"].(string)
			require.True(t, ok, "'timestamp' must be string")
			_, err = time.Parse(time.RFC3339Nano, timestampStr)
			assert.NoError(t, err, "timestamp must be valid RFC3339")
		})
	}
}
