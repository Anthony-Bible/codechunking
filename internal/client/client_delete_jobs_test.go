package client_test

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/client"
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

// TestClient_DeleteRepository_Success tests successful repository deletion.
func TestClient_DeleteRepository_Success(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "Should use DELETE method")
		expectedPath := fmt.Sprintf("/repositories/%s", repoID.String())
		assert.Equal(t, expectedPath, r.URL.Path, "Should call correct endpoint")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	err = c.DeleteRepository(context.Background(), repoID)
	require.NoError(t, err, "DeleteRepository should succeed on 204")
}

// TestClient_DeleteRepository_NotFound tests deletion of a non-existent repository.
func TestClient_DeleteRepository_NotFound(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "repository not found"}`))
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	err = c.DeleteRepository(context.Background(), repoID)
	require.Error(t, err, "DeleteRepository should return error for 404")
	assert.Contains(t, err.Error(), "404", "Error should mention status code")
}

// TestClient_ListRepositoryJobs_Success tests successful listing of repository jobs.
func TestClient_ListRepositoryJobs_Success(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()
	startedAt := time.Now()

	expected := &dto.IndexingJobListResponse{
		Jobs: []dto.IndexingJobResponse{
			{
				ID:             jobID,
				RepositoryID:   repoID,
				Status:         "completed",
				StartedAt:      &startedAt,
				FilesProcessed: 10,
				ChunksCreated:  50,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			},
		},
		Pagination: dto.PaginationResponse{Limit: 10, Offset: 0, Total: 1, HasMore: false},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "Should use GET method")
		expectedPath := fmt.Sprintf("/repositories/%s/jobs", repoID.String())
		assert.Equal(t, expectedPath, r.URL.Path, "Should call correct endpoint")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	resp, err := c.ListRepositoryJobs(context.Background(), repoID, dto.IndexingJobListQuery{Limit: 10})
	require.NoError(t, err, "ListRepositoryJobs should succeed")
	require.NotNil(t, resp, "Response should not be nil")
	assert.Len(t, resp.Jobs, 1, "Should return one job")
	assert.Equal(t, jobID, resp.Jobs[0].ID, "Job ID should match")
	assert.Equal(t, "completed", resp.Jobs[0].Status, "Status should match")
}

// TestClient_ListRepositoryJobs_WithPagination tests job listing with pagination.
func TestClient_ListRepositoryJobs_WithPagination(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "5", r.URL.Query().Get("limit"), "Should pass limit param")
		assert.Equal(t, "10", r.URL.Query().Get("offset"), "Should pass offset param")

		resp := dto.IndexingJobListResponse{
			Jobs:       []dto.IndexingJobResponse{},
			Pagination: dto.PaginationResponse{Limit: 5, Offset: 10, Total: 10, HasMore: false},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	query := dto.IndexingJobListQuery{Limit: 5, Offset: 10}
	resp, err := c.ListRepositoryJobs(context.Background(), repoID, query)
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestClient_ListRepositoryJobs_NotFound tests job listing for a non-existent repository.
func TestClient_ListRepositoryJobs_NotFound(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &client.Config{APIURL: server.URL, Timeout: 5 * time.Second}
	c, err := client.NewClient(cfg)
	require.NoError(t, err)

	resp, err := c.ListRepositoryJobs(context.Background(), repoID, dto.IndexingJobListQuery{Limit: 10})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "404")
}
