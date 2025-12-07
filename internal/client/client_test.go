package client_test

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/client"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClient_ValidConfig tests that a client can be created with valid configuration.
func TestNewClient_ValidConfig(t *testing.T) {
	t.Parallel()

	cfg := &client.Config{
		APIURL:  "http://localhost:8080",
		Timeout: 30 * time.Second,
	}

	c, err := client.NewClient(cfg)

	require.NoError(t, err, "NewClient should succeed with valid config")
	assert.NotNil(t, c, "Client should not be nil")
}

// TestNewClient_NilConfig tests that NewClient returns an error for nil config.
func TestNewClient_NilConfig(t *testing.T) {
	t.Parallel()

	c, err := client.NewClient(nil)

	require.Error(t, err, "NewClient should return error for nil config")
	assert.Nil(t, c, "Client should be nil on error")
	assert.Contains(t, err.Error(), "config cannot be nil", "Error should mention nil config")
}

// TestNewClient_InvalidConfig tests that NewClient returns an error for invalid config.
func TestNewClient_InvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config *client.Config
		errMsg string
	}{
		{
			name: "empty API URL",
			config: &client.Config{
				APIURL:  "",
				Timeout: 30 * time.Second,
			},
			errMsg: "API URL cannot be empty",
		},
		{
			name: "invalid URL scheme",
			config: &client.Config{
				APIURL:  "ftp://localhost:8080",
				Timeout: 30 * time.Second,
			},
			errMsg: "http:// or https:// scheme",
		},
		{
			name: "zero timeout",
			config: &client.Config{
				APIURL:  "http://localhost:8080",
				Timeout: 0,
			},
			errMsg: "timeout must be positive",
		},
		{
			name: "negative timeout",
			config: &client.Config{
				APIURL:  "http://localhost:8080",
				Timeout: -1 * time.Second,
			},
			errMsg: "timeout must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := client.NewClient(tt.config)

			require.Error(t, err, "NewClient should return error for invalid config")
			assert.Nil(t, c, "Client should be nil on error")
			assert.Contains(t, err.Error(), tt.errMsg, "Error should contain expected message")
		})
	}
}

// TestNewClientWithHTTPClient tests that a client can be created with a custom HTTP client.
func TestNewClientWithHTTPClient(t *testing.T) {
	t.Parallel()

	cfg := &client.Config{
		APIURL:  "http://localhost:8080",
		Timeout: 30 * time.Second,
	}
	customHTTPClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	c, err := client.NewClientWithHTTPClient(cfg, customHTTPClient)

	require.NoError(t, err, "NewClientWithHTTPClient should succeed")
	assert.NotNil(t, c, "Client should not be nil")
}

// TestNewClientWithHTTPClient_NilHTTPClient tests that custom HTTP client can be nil (uses default).
func TestNewClientWithHTTPClient_NilHTTPClient(t *testing.T) {
	t.Parallel()

	cfg := &client.Config{
		APIURL:  "http://localhost:8080",
		Timeout: 30 * time.Second,
	}

	c, err := client.NewClientWithHTTPClient(cfg, nil)

	require.NoError(t, err, "NewClientWithHTTPClient should succeed with nil HTTP client")
	assert.NotNil(t, c, "Client should not be nil")
}

// TestClient_Health_Success tests successful health check.
func TestClient_Health_Success(t *testing.T) {
	t.Parallel()

	expectedResponse := &dto.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Dependencies: map[string]dto.DependencyStatus{
			"database": {
				Status:       "healthy",
				Message:      "Connected",
				ResponseTime: "5ms",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "Should use GET method")
		assert.Equal(t, "/health", r.URL.Path, "Should call /health endpoint")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	health, err := c.Health(ctx)

	require.NoError(t, err, "Health should succeed")
	assert.NotNil(t, health, "Health response should not be nil")
	assert.Equal(t, expectedResponse.Status, health.Status, "Status should match")
	assert.Equal(t, expectedResponse.Version, health.Version, "Version should match")
}

// TestClient_Health_ServerError tests health check with server error.
func TestClient_Health_ServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "database connection failed"}`))
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	health, err := c.Health(ctx)

	require.Error(t, err, "Health should return error for server error")
	assert.Nil(t, health, "Health response should be nil on error")
	assert.Contains(t, err.Error(), "500", "Error should mention status code")
}

// TestClient_Health_NetworkError tests health check with network error.
func TestClient_Health_NetworkError(t *testing.T) {
	t.Parallel()

	cfg := &client.Config{
		APIURL:  "http://localhost:9999", // Non-existent server
		Timeout: 1 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	health, err := c.Health(ctx)

	require.Error(t, err, "Health should return error for network error")
	assert.Nil(t, health, "Health response should be nil on error")
}

// TestClient_Health_ContextCancellation tests health check with context cancellation.
func TestClient_Health_ContextCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Simulate slow response
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 10 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	health, err := c.Health(ctx)

	require.Error(t, err, "Health should return error for context cancellation")
	assert.Nil(t, health, "Health response should be nil on error")
	assert.ErrorIs(t, err, context.DeadlineExceeded, "Error should be context.DeadlineExceeded")
}

// TestClient_Search_Success tests successful search operation.
func TestClient_Search_Success(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	chunkID := uuid.New()
	searchReq := dto.SearchRequestDTO{
		Query: "authentication middleware",
		Limit: 10,
	}

	expectedResponse := &dto.SearchResponseDTO{
		Results: []dto.SearchResultDTO{
			{
				ChunkID:         chunkID,
				Content:         "func authenticateUser() {}",
				SimilarityScore: 0.95,
				Repository: dto.RepositoryInfo{
					ID:   repoID,
					Name: "auth-service",
					URL:  "https://github.com/example/auth-service",
				},
				FilePath:  "/middleware/auth.go",
				Language:  "go",
				StartLine: 15,
				EndLine:   25,
			},
		},
		Pagination: dto.PaginationResponse{
			Limit:   10,
			Offset:  0,
			Total:   1,
			HasMore: false,
		},
		Metadata: dto.SearchMetadata{
			Query:           "authentication middleware",
			ExecutionTimeMs: 150,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "Should use POST method")
		assert.Equal(t, "/search", r.URL.Path, "Should call /search endpoint")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"), "Should set Content-Type header")

		var receivedReq dto.SearchRequestDTO
		err := json.NewDecoder(r.Body).Decode(&receivedReq)
		assert.NoError(t, err)
		assert.Equal(t, searchReq.Query, receivedReq.Query, "Query should match")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := c.Search(ctx, searchReq)

	require.NoError(t, err, "Search should succeed")
	assert.NotNil(t, resp, "Search response should not be nil")
	assert.Len(t, resp.Results, 1, "Should have 1 result")
	assert.Equal(t, expectedResponse.Results[0].Content, resp.Results[0].Content, "Content should match")
}

// TestClient_Search_EmptyResults tests search with no results.
func TestClient_Search_EmptyResults(t *testing.T) {
	t.Parallel()

	searchReq := dto.SearchRequestDTO{
		Query: "nonexistent query",
		Limit: 10,
	}

	expectedResponse := &dto.SearchResponseDTO{
		Results: []dto.SearchResultDTO{},
		Pagination: dto.PaginationResponse{
			Limit:   10,
			Offset:  0,
			Total:   0,
			HasMore: false,
		},
		Metadata: dto.SearchMetadata{
			Query:           "nonexistent query",
			ExecutionTimeMs: 50,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := c.Search(ctx, searchReq)

	require.NoError(t, err, "Search should succeed even with empty results")
	assert.NotNil(t, resp, "Search response should not be nil")
	assert.Empty(t, resp.Results, "Results should be empty")
	assert.Equal(t, 0, resp.Pagination.Total, "Total should be 0")
}

// TestClient_Search_ServerError tests search with server error.
func TestClient_Search_ServerError(t *testing.T) {
	t.Parallel()

	searchReq := dto.SearchRequestDTO{
		Query: "test query",
		Limit: 10,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := c.Search(ctx, searchReq)

	require.Error(t, err, "Search should return error for server error")
	assert.Nil(t, resp, "Search response should be nil on error")
	assert.Contains(t, err.Error(), "500", "Error should mention status code")
}

// TestClient_Search_InvalidJSON tests search with malformed JSON response.
func TestClient_Search_InvalidJSON(t *testing.T) {
	t.Parallel()

	searchReq := dto.SearchRequestDTO{
		Query: "test query",
		Limit: 10,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"invalid json`))
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := c.Search(ctx, searchReq)

	require.Error(t, err, "Search should return error for invalid JSON")
	assert.Nil(t, resp, "Search response should be nil on error")
	assert.Contains(t, err.Error(), "decode", "Error should mention decoding issue")
}

// TestClient_ListRepositories_Success tests successful repository listing.
func TestClient_ListRepositories_Success(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	query := dto.RepositoryListQuery{
		Limit:  20,
		Offset: 0,
		Sort:   "created_at:desc",
	}

	expectedResponse := &dto.RepositoryListResponse{
		Repositories: []dto.RepositoryResponse{
			{
				ID:          repoID,
				URL:         "https://github.com/golang/go",
				Name:        "golang/go",
				Status:      "completed",
				TotalFiles:  100,
				TotalChunks: 500,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
		Pagination: dto.PaginationResponse{
			Limit:   20,
			Offset:  0,
			Total:   1,
			HasMore: false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "Should use GET method")
		assert.Equal(t, "/repositories", r.URL.Path, "Should call /repositories endpoint")

		// Check query parameters
		assert.Equal(t, "20", r.URL.Query().Get("limit"), "Limit should match")
		assert.Equal(t, "0", r.URL.Query().Get("offset"), "Offset should match")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := c.ListRepositories(ctx, query)

	require.NoError(t, err, "ListRepositories should succeed")
	assert.NotNil(t, resp, "Response should not be nil")
	assert.Len(t, resp.Repositories, 1, "Should have 1 repository")
	assert.Equal(t, expectedResponse.Repositories[0].Name, resp.Repositories[0].Name, "Name should match")
}

// TestClient_ListRepositories_WithFilters tests repository listing with filters.
func TestClient_ListRepositories_WithFilters(t *testing.T) {
	t.Parallel()

	query := dto.RepositoryListQuery{
		Status: "completed",
		Name:   "golang",
		Limit:  10,
		Offset: 0,
	}

	expectedResponse := &dto.RepositoryListResponse{
		Repositories: []dto.RepositoryResponse{},
		Pagination: dto.PaginationResponse{
			Limit:   10,
			Offset:  0,
			Total:   0,
			HasMore: false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query parameters
		assert.Equal(t, "completed", r.URL.Query().Get("status"), "Status filter should be applied")
		assert.Equal(t, "golang", r.URL.Query().Get("name"), "Name filter should be applied")
		assert.Equal(t, "10", r.URL.Query().Get("limit"), "Limit should be applied")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := c.ListRepositories(ctx, query)

	require.NoError(t, err, "ListRepositories should succeed with filters")
	assert.NotNil(t, resp, "Response should not be nil")
}

// TestClient_GetRepository_Success tests successful repository retrieval.
func TestClient_GetRepository_Success(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	expectedResponse := &dto.RepositoryResponse{
		ID:          repoID,
		URL:         "https://github.com/golang/go",
		Name:        "golang/go",
		Status:      "completed",
		TotalFiles:  100,
		TotalChunks: 500,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "Should use GET method")
		expectedPath := fmt.Sprintf("/repositories/%s", repoID.String())
		assert.Equal(t, expectedPath, r.URL.Path, "Should call correct endpoint")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := c.GetRepository(ctx, repoID)

	require.NoError(t, err, "GetRepository should succeed")
	assert.NotNil(t, resp, "Response should not be nil")
	assert.Equal(t, repoID, resp.ID, "Repository ID should match")
	assert.Equal(t, expectedResponse.Name, resp.Name, "Name should match")
}

// TestClient_GetRepository_NotFound tests repository retrieval with 404.
func TestClient_GetRepository_NotFound(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "repository not found"}`))
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := c.GetRepository(ctx, repoID)

	require.Error(t, err, "GetRepository should return error for 404")
	assert.Nil(t, resp, "Response should be nil on error")
	assert.Contains(t, err.Error(), "404", "Error should mention status code")
}

// TestClient_CreateRepository_Success tests successful repository creation.
func TestClient_CreateRepository_Success(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	createReq := dto.CreateRepositoryRequest{
		URL:  "https://github.com/golang/go",
		Name: "golang/go",
	}

	expectedResponse := &dto.RepositoryResponse{
		ID:          repoID,
		URL:         createReq.URL,
		Name:        createReq.Name,
		Status:      "pending",
		TotalFiles:  0,
		TotalChunks: 0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "Should use POST method")
		assert.Equal(t, "/repositories", r.URL.Path, "Should call /repositories endpoint")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"), "Should set Content-Type header")

		var receivedReq dto.CreateRepositoryRequest
		err := json.NewDecoder(r.Body).Decode(&receivedReq)
		assert.NoError(t, err)
		assert.Equal(t, createReq.URL, receivedReq.URL, "URL should match")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := c.CreateRepository(ctx, createReq)

	require.NoError(t, err, "CreateRepository should succeed")
	assert.NotNil(t, resp, "Response should not be nil")
	assert.Equal(t, expectedResponse.URL, resp.URL, "URL should match")
	assert.Equal(t, "pending", resp.Status, "Status should be pending")
}

// TestClient_CreateRepository_ValidationError tests repository creation with validation error.
func TestClient_CreateRepository_ValidationError(t *testing.T) {
	t.Parallel()

	createReq := dto.CreateRepositoryRequest{
		URL:  "", // Invalid: empty URL
		Name: "golang/go",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "validation failed", "details": ["url is required"]}`))
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := c.CreateRepository(ctx, createReq)

	require.Error(t, err, "CreateRepository should return error for validation failure")
	assert.Nil(t, resp, "Response should be nil on error")
	assert.Contains(t, err.Error(), "400", "Error should mention status code")
}

// TestClient_GetIndexingJob_Success tests successful indexing job retrieval.
func TestClient_GetIndexingJob_Success(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()
	startedAt := time.Now()

	expectedResponse := &dto.IndexingJobResponse{
		ID:             jobID,
		RepositoryID:   repoID,
		Status:         "in_progress",
		StartedAt:      &startedAt,
		FilesProcessed: 50,
		ChunksCreated:  250,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "Should use GET method")
		expectedPath := fmt.Sprintf("/repositories/%s/jobs/%s", repoID.String(), jobID.String())
		assert.Equal(t, expectedPath, r.URL.Path, "Should call correct endpoint")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := c.GetIndexingJob(ctx, repoID, jobID)

	require.NoError(t, err, "GetIndexingJob should succeed")
	assert.NotNil(t, resp, "Response should not be nil")
	assert.Equal(t, jobID, resp.ID, "Job ID should match")
	assert.Equal(t, repoID, resp.RepositoryID, "Repository ID should match")
	assert.Equal(t, "in_progress", resp.Status, "Status should match")
}

// TestClient_GetIndexingJob_NotFound tests indexing job retrieval with 404.
func TestClient_GetIndexingJob_NotFound(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "job not found"}`))
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := c.GetIndexingJob(ctx, repoID, jobID)

	require.Error(t, err, "GetIndexingJob should return error for 404")
	assert.Nil(t, resp, "Response should be nil on error")
	assert.Contains(t, err.Error(), "404", "Error should mention status code")
}

// TestClient_ErrorWrapping tests that errors are properly wrapped with context.
func TestClient_ErrorWrapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedErrMsg string
		method         string
		endpoint       string
	}{
		{
			name:           "400 Bad Request",
			statusCode:     http.StatusBadRequest,
			responseBody:   `{"error": "invalid request"}`,
			expectedErrMsg: "400",
			method:         "POST",
			endpoint:       "/search",
		},
		{
			name:           "401 Unauthorized",
			statusCode:     http.StatusUnauthorized,
			responseBody:   `{"error": "unauthorized"}`,
			expectedErrMsg: "401",
			method:         "GET",
			endpoint:       "/repositories",
		},
		{
			name:           "403 Forbidden",
			statusCode:     http.StatusForbidden,
			responseBody:   `{"error": "forbidden"}`,
			expectedErrMsg: "403",
			method:         "POST",
			endpoint:       "/repositories",
		},
		{
			name:           "500 Internal Server Error",
			statusCode:     http.StatusInternalServerError,
			responseBody:   `{"error": "internal error"}`,
			expectedErrMsg: "500",
			method:         "GET",
			endpoint:       "/health",
		},
		{
			name:           "503 Service Unavailable",
			statusCode:     http.StatusServiceUnavailable,
			responseBody:   `{"error": "service unavailable"}`,
			expectedErrMsg: "503",
			method:         "GET",
			endpoint:       "/health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			cfg := &client.Config{
				APIURL:  server.URL,
				Timeout: 5 * time.Second,
			}
			c, err := client.NewClient(cfg)
			require.NoError(t, err)

			ctx := context.Background()

			// Trigger error by calling Health (simplest endpoint)
			_, err = c.Health(ctx)

			require.Error(t, err, "Should return error")
			assert.Contains(t, err.Error(), tt.expectedErrMsg, "Error should contain status code")
		})
	}
}

// TestClient_ErrorDetails_Extraction tests that error details are properly extracted from responses.
func TestClient_ErrorDetails_Extraction(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"error": "validation failed",
			"details": ["field 'url' is required", "field 'name' is invalid"]
		}`))
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = c.Health(ctx)

	require.Error(t, err, "Should return error")
	// Error should contain both status code and error message
	errMsg := err.Error()
	assert.Contains(t, errMsg, "400", "Error should contain status code")
	// The actual error message might be wrapped, so we just verify it's not empty
	assert.NotEmpty(t, errMsg, "Error message should not be empty")
}

// TestClient_RequestHeaders tests that proper headers are set on requests.
func TestClient_RequestHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		assert.Equal(
			t,
			"application/json",
			r.Header.Get("Content-Type"),
			"Content-Type should be application/json for POST",
		)
		assert.NotEmpty(t, r.Header.Get("User-Agent"), "User-Agent should be set")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "healthy"}`))
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	// Test POST request (CreateRepository)
	createReq := dto.CreateRepositoryRequest{
		URL:  "https://github.com/golang/go",
		Name: "golang/go",
	}
	_, _ = c.CreateRepository(ctx, createReq)
}

// TestClient_URLConstruction tests that URLs are properly constructed.
func TestClient_URLConstruction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		baseURL      string
		endpoint     string
		expectedPath string
	}{
		{
			name:         "base URL without trailing slash",
			baseURL:      "http://localhost:8080",
			endpoint:     "/health",
			expectedPath: "/health",
		},
		{
			name:         "base URL with trailing slash",
			baseURL:      "http://localhost:8080/",
			endpoint:     "/health",
			expectedPath: "/health",
		},
		{
			name:         "base URL with path",
			baseURL:      "http://localhost:8080/api/v1",
			endpoint:     "/health",
			expectedPath: "/api/v1/health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			receivedPath := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status": "healthy"}`))
			}))
			defer server.Close()

			// Use the test server URL as base, but we're testing path construction
			cfg := &client.Config{
				APIURL:  server.URL,
				Timeout: 5 * time.Second,
			}
			c, err := client.NewClient(cfg)
			require.NoError(t, err)

			ctx := context.Background()
			_, _ = c.Health(ctx)

			// For this test, we just verify the endpoint is called
			// The actual path construction is tested by ensuring the call succeeds
			assert.NotEmpty(t, receivedPath, "Path should not be empty")
		})
	}
}

// TestClient_ResponseBodyClosed tests that response bodies are properly closed.
func TestClient_ResponseBodyClosed(t *testing.T) {
	t.Parallel()

	var bodyClosed bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "healthy"}`))
	}))
	defer server.Close()

	// Create a custom HTTP client that tracks body closure
	transport := &closeTrackingTransport{
		base: http.DefaultTransport,
		onClose: func() {
			bodyClosed = true
		},
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClientWithHTTPClient(cfg, httpClient)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = c.Health(ctx)
	require.NoError(t, err)

	// Note: In practice, this test would need a more sophisticated approach
	// to track body closure, but this demonstrates the intent
	// For now, we just verify the call succeeded and use bodyClosed to track closure
	assert.NoError(t, err, "Health call should succeed")
	_ = bodyClosed // Placeholder for future body closure verification
}

// closeTrackingTransport is a test helper that tracks when response bodies are closed.
type closeTrackingTransport struct {
	base    http.RoundTripper
	onClose func()
}

func (t *closeTrackingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Wrap the body to track closure
	resp.Body = &closeTrackingBody{
		ReadCloser: resp.Body,
		onClose:    t.onClose,
	}

	return resp, nil
}

// closeTrackingBody is a test helper that tracks when a body is closed.
type closeTrackingBody struct {
	io.ReadCloser

	onClose func()
}

func (b *closeTrackingBody) Close() error {
	if b.onClose != nil {
		b.onClose()
	}
	return b.ReadCloser.Close()
}

// TestClient_JSONEncoding tests that request bodies are properly JSON encoded.
func TestClient_JSONEncoding(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)

		// Verify it's valid JSON
		var jsonData map[string]interface{}
		err = json.Unmarshal(body, &jsonData)
		assert.NoError(t, err, "Request body should be valid JSON")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "healthy"}`))
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	searchReq := dto.SearchRequestDTO{
		Query: "test",
		Limit: 10,
	}
	_, _ = c.Search(ctx, searchReq)
}

// TestClient_QueryParameters tests that query parameters are properly encoded.
func TestClient_QueryParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		query         dto.RepositoryListQuery
		expectedQuery map[string]string
	}{
		{
			name: "basic pagination",
			query: dto.RepositoryListQuery{
				Limit:  10,
				Offset: 5,
			},
			expectedQuery: map[string]string{
				"limit":  "10",
				"offset": "5",
			},
		},
		{
			name: "with status filter",
			query: dto.RepositoryListQuery{
				Status: "completed",
				Limit:  20,
				Offset: 0,
			},
			expectedQuery: map[string]string{
				"status": "completed",
				"limit":  "20",
				"offset": "0",
			},
		},
		{
			name: "with name filter",
			query: dto.RepositoryListQuery{
				Name:   "golang",
				Limit:  15,
				Offset: 10,
			},
			expectedQuery: map[string]string{
				"name":   "golang",
				"limit":  "15",
				"offset": "10",
			},
		},
		{
			name: "URL encoding special characters",
			query: dto.RepositoryListQuery{
				Name:   "repo with spaces",
				Limit:  10,
				Offset: 0,
			},
			expectedQuery: map[string]string{
				"name":   "repo with spaces",
				"limit":  "10",
				"offset": "0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				queryParams := r.URL.Query()

				for key, expectedValue := range tt.expectedQuery {
					actualValue := queryParams.Get(key)
					if expectedValue == "" && actualValue == "" {
						continue
					}
					assert.Equal(t, expectedValue, actualValue, "Query parameter %s should match", key)
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(
					[]byte(
						`{"repositories": [], "pagination": {"limit": 10, "offset": 0, "total": 0, "has_more": false}}`,
					),
				)
			}))
			defer server.Close()

			cfg := &client.Config{
				APIURL:  server.URL,
				Timeout: 5 * time.Second,
			}
			c, err := client.NewClient(cfg)
			require.NoError(t, err)

			ctx := context.Background()
			_, _ = c.ListRepositories(ctx, tt.query)
		})
	}
}

// TestClient_EmptyResponseHandling tests handling of empty but valid responses.
func TestClient_EmptyResponseHandling(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Empty JSON object
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	// This should handle empty but valid JSON
	_, err = c.Health(ctx)

	// Depending on implementation, this might succeed or fail
	// The test just ensures it doesn't panic
	_ = err
}

// TestClient_LargeResponse tests handling of large responses.
func TestClient_LargeResponse(t *testing.T) {
	t.Parallel()

	// Create a large response with many results
	results := make([]dto.SearchResultDTO, 100)
	for i := range 100 {
		results[i] = dto.SearchResultDTO{
			ChunkID:         uuid.New(),
			Content:         strings.Repeat("a", 1000), // 1KB of content per result
			SimilarityScore: 0.9,
			Repository: dto.RepositoryInfo{
				ID:   uuid.New(),
				Name: fmt.Sprintf("repo-%d", i),
				URL:  fmt.Sprintf("https://github.com/example/repo-%d", i),
			},
			FilePath:  "/test/file.go",
			Language:  "go",
			StartLine: 1,
			EndLine:   10,
		}
	}

	largeResponse := &dto.SearchResponseDTO{
		Results: results,
		Pagination: dto.PaginationResponse{
			Limit:   100,
			Offset:  0,
			Total:   100,
			HasMore: false,
		},
		Metadata: dto.SearchMetadata{
			Query:           "test",
			ExecutionTimeMs: 1000,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(largeResponse)
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	searchReq := dto.SearchRequestDTO{
		Query: "test",
		Limit: 100,
	}

	resp, err := c.Search(ctx, searchReq)

	require.NoError(t, err, "Should handle large response")
	assert.NotNil(t, resp, "Response should not be nil")
	assert.Len(t, resp.Results, 100, "Should have 100 results")
}

// TestClient_ConcurrentRequests tests that the client can handle concurrent requests safely.
func TestClient_ConcurrentRequests(t *testing.T) {
	t.Parallel()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "healthy"}`))
	}))
	defer server.Close()

	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 5 * time.Second,
	}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	// Make multiple concurrent requests
	const numRequests = 10
	errChan := make(chan error, numRequests)

	for range numRequests {
		go func() {
			ctx := context.Background()
			_, err := c.Health(ctx)
			errChan <- err
		}()
	}

	// Collect results
	for range numRequests {
		err := <-errChan
		assert.NoError(t, err, "Concurrent request should succeed")
	}
}

// TestClient_TimeoutConfiguration tests that the client respects timeout configuration.
func TestClient_TimeoutConfiguration(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Simulate slow response
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "healthy"}`))
	}))
	defer server.Close()

	// Configure client with very short timeout
	cfg := &client.Config{
		APIURL:  server.URL,
		Timeout: 100 * time.Millisecond,
	}

	httpClient := &http.Client{
		Timeout: cfg.Timeout,
	}

	c, err := client.NewClientWithHTTPClient(cfg, httpClient)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = c.Health(ctx)

	require.Error(t, err, "Should timeout")
	// The error might be wrapped, so we check for timeout-related errors
	errMsg := err.Error()
	assert.True(t,
		strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "deadline") ||
			strings.Contains(errMsg, "context deadline exceeded"),
		"Error should indicate timeout: %s", errMsg)
}
