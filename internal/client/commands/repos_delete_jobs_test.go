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
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReposCmd_HasDeleteSubcommand verifies the repos command has a delete subcommand.
func TestReposCmd_HasDeleteSubcommand(t *testing.T) {
	t.Parallel()

	cmd := commands.NewReposCmd()

	subcommandNames := make(map[string]bool)
	for _, subcmd := range cmd.Commands() {
		subcommandNames[subcmd.Use] = true
	}

	assert.True(t, subcommandNames["delete"], "repos should have 'delete' subcommand")
}

// TestReposCmd_HasJobsSubcommand verifies the repos command has a jobs subcommand.
func TestReposCmd_HasJobsSubcommand(t *testing.T) {
	t.Parallel()

	cmd := commands.NewReposCmd()

	subcommandNames := make(map[string]bool)
	for _, subcmd := range cmd.Commands() {
		subcommandNames[subcmd.Use] = true
	}

	assert.True(t, subcommandNames["jobs"], "repos should have 'jobs' subcommand")
}

// TestReposDeleteCmd_Success verifies the repos delete command outputs success JSON on 204.
func TestReposDeleteCmd_Success(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
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
	assert.True(t, resp.Success, "response should be success")
}

// TestReposDeleteCmd_InvalidUUID verifies the repos delete command rejects non-UUID args.
func TestReposDeleteCmd_InvalidUUID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"repos", "delete", "not-a-uuid", "--api-url", "http://localhost:9999"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var resp client.Response
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))
	assert.False(t, resp.Success, "should fail for invalid UUID")
	require.NotNil(t, resp.Error)
	assert.Equal(t, "INVALID_ARGUMENT", resp.Error.Code)
}

// TestReposDeleteCmd_NoArgs verifies the repos delete command requires an argument.
func TestReposDeleteCmd_NoArgs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"repos", "delete", "--api-url", "http://localhost:9999"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var resp client.Response
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))
	assert.False(t, resp.Success, "should fail with no args")
}

// TestReposJobsCmd_Success verifies the repos jobs command outputs success JSON with job list.
func TestReposJobsCmd_Success(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	jobID := uuid.New()
	startedAt := time.Now()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, repoID.String())
		assert.Contains(t, r.URL.Path, "/jobs")

		resp := dto.IndexingJobListResponse{
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
			Pagination: dto.PaginationResponse{Limit: 10, Offset: 0, Total: 1},
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
	rootCmd.SetArgs([]string{"repos", "jobs", repoID.String(), "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var resp client.Response
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))
	assert.True(t, resp.Success, "response should be success")
}

// TestReposJobsCmd_InvalidUUID verifies the repos jobs command rejects invalid UUIDs.
func TestReposJobsCmd_InvalidUUID(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"repos", "jobs", "not-a-uuid", "--api-url", "http://localhost:9999"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var resp client.Response
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))
	assert.False(t, resp.Success, "should fail for invalid UUID")
	require.NotNil(t, resp.Error)
	assert.Equal(t, "INVALID_ARGUMENT", resp.Error.Code)
}

// TestReposJobsCmd_NoArgs verifies repos jobs command requires an argument.
func TestReposJobsCmd_NoArgs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"repos", "jobs", "--api-url", "http://localhost:9999"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var resp client.Response
	require.NoError(t, json.Unmarshal(buf.Bytes(), &resp))
	assert.False(t, resp.Success, "should fail with no args")
}

// TestReposJobsCmd_HasPaginationFlags verifies repos jobs command exposes limit and offset flags.
func TestReposJobsCmd_HasPaginationFlags(t *testing.T) {
	t.Parallel()

	cmd := commands.NewReposJobsCmd()

	assert.NotNil(t, cmd.Flags().Lookup("limit"), "jobs command should have --limit flag")
	assert.NotNil(t, cmd.Flags().Lookup("offset"), "jobs command should have --offset flag")
}
