package commands_test

import (
	"bytes"
	"codechunking/internal/application/dto"
	"codechunking/internal/client"
	"codechunking/internal/client/commands"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReposCmd_HasQuerySubcommand verifies the repos command has a query subcommand.
func TestReposCmd_HasQuerySubcommand(t *testing.T) {
	t.Parallel()

	cmd := commands.NewReposCmd()

	subcommandNames := make(map[string]bool)
	for _, subcmd := range cmd.Commands() {
		subcommandNames[subcmd.Use] = true
	}

	assert.True(t, subcommandNames["query"], "repos should have 'query' subcommand")
}

// TestReposQueryCmd_Success verifies repos query outputs JSON results on success.
func TestReposQueryCmd_Success(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	chunkID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "should use POST for search")
		assert.Equal(t, "/search", r.URL.Path, "should call /search endpoint")

		// Verify the repo ID is in the request body
		var req dto.SearchRequestDTO
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Len(t, req.RepositoryIDs, 1, "should filter by exactly one repo ID")
		assert.Equal(t, repoID, req.RepositoryIDs[0], "should use the provided repo ID")
		assert.Equal(t, "find authentication", req.Query, "should pass through the query")

		resp := dto.SearchResponseDTO{
			Results: []dto.SearchResultDTO{
				{
					ChunkID:         chunkID,
					Content:         "func authenticate(token string) bool { ... }",
					FilePath:        "auth/auth.go",
					Language:        "go",
					SimilarityScore: 0.95,
					Type:            "function",
					EntityName:      "authenticate",
				},
			},
			Pagination: dto.PaginationResponse{Limit: 10, Offset: 0, Total: 1, HasMore: false},
			Metadata:   dto.SearchMetadata{Query: "find authentication", ExecutionTimeMs: 30},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	var buf bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"repos", "query", repoID.String(), "find authentication", "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var resp client.Response
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))
	assert.True(t, resp.Success, "response should be success")
	assert.NotNil(t, resp.Data, "response should have data")
}

// TestReposQueryCmd_InvalidRepoUUID verifies repos query rejects invalid repo UUID.
func TestReposQueryCmd_InvalidRepoUUID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"repos", "query", "not-a-uuid", "find auth", "--api-url", "http://localhost:9999"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var resp client.Response
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))
	assert.False(t, resp.Success, "should fail for invalid UUID")
	require.NotNil(t, resp.Error)
	assert.Equal(t, "INVALID_ARGUMENT", resp.Error.Code)
}

// TestReposQueryCmd_NoArgs verifies repos query requires exactly two arguments.
func TestReposQueryCmd_NoArgs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"repos", "query", "--api-url", "http://localhost:9999"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var resp client.Response
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))
	assert.False(t, resp.Success, "should fail with no args")
}

// TestReposQueryCmd_OnlyRepoID verifies repos query requires a query argument too.
func TestReposQueryCmd_OnlyRepoID(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()

	var buf bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"repos", "query", repoID.String(), "--api-url", "http://localhost:9999"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var resp client.Response
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))
	assert.False(t, resp.Success, "should fail with only repo ID (no query)")
}

// TestReposQueryCmd_HasFlags verifies repos query exposes useful filter flags.
func TestReposQueryCmd_HasFlags(t *testing.T) {
	t.Parallel()

	cmd := commands.NewReposQueryCmd()

	assert.NotNil(t, cmd.Flags().Lookup("limit"), "query command should have --limit flag")
	assert.NotNil(t, cmd.Flags().Lookup("offset"), "query command should have --offset flag")
	assert.NotNil(t, cmd.Flags().Lookup("threshold"), "query command should have --threshold flag")
	assert.NotNil(t, cmd.Flags().Lookup("languages"), "query command should have --languages flag")
	assert.NotNil(t, cmd.Flags().Lookup("types"), "query command should have --types flag")
	assert.NotNil(t, cmd.Flags().Lookup("sort"), "query command should have --sort flag")
}

// TestReposQueryCmd_WithFilters verifies flag values are forwarded to the search request.
func TestReposQueryCmd_WithFilters(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req dto.SearchRequestDTO
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, 5, req.Limit, "limit flag should be forwarded")
		assert.Equal(t, 10, req.Offset, "offset flag should be forwarded")
		assert.InDelta(t, 0.85, req.SimilarityThreshold, 0.001, "threshold flag should be forwarded")
		assert.Equal(t, []string{"go"}, req.Languages, "languages flag should be forwarded")
		assert.Equal(t, []string{"function"}, req.Types, "types flag should be forwarded")
		assert.Equal(t, "file_path:asc", req.Sort, "sort flag should be forwarded")
		// L2 fix: verify the repo UUID is always scoped correctly even with filters
		require.Len(t, req.RepositoryIDs, 1, "should scope to exactly one repo")
		assert.Equal(t, repoID, req.RepositoryIDs[0], "repo ID should be forwarded even with filters")

		resp := dto.SearchResponseDTO{
			Results:    []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{Limit: 5},
			Metadata:   dto.SearchMetadata{Query: req.Query},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	var buf bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{
		"repos", "query", repoID.String(), "find auth",
		"--limit", "5",
		"--offset", "10",
		"--threshold", "0.85",
		"--languages", "go",
		"--types", "function",
		"--sort", "file_path:asc",
		"--api-url", server.URL,
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var resp client.Response
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))
	assert.True(t, resp.Success, "should succeed with valid filters")
}

// TestNewReposQueryCmd verifies the command metadata is correct.
func TestNewReposQueryCmd(t *testing.T) {
	t.Parallel()

	cmd := commands.NewReposQueryCmd()

	require.NotNil(t, cmd)
	assert.Equal(t, "query", cmd.Use, "command Use should be 'query'")
	assert.NotEmpty(t, cmd.Short, "command should have a short description")
	assert.NotNil(t, cmd.RunE, "command should have RunE set")
}

// TestReposQueryCmd_DeleteReturnsArchived verifies the repos delete command returns "archived" status.
// This ensures the status field in the response reflects the actual domain operation.
func TestReposDeleteCmd_ReturnsArchivedStatus(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	var buf bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"repos", "delete", repoID.String(), "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var resp client.Response
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))
	require.True(t, resp.Success)
	dataJSON, _ := json.Marshal(resp.Data)
	var data map[string]string
	require.NoError(t, json.Unmarshal(dataJSON, &data))
	assert.Equal(t, "archived", data["status"], "delete response should say 'archived', not 'deleted'")
}
