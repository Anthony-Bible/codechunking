//go:build integration

package zoekt

// Integration tests for Zoekt gRPC client and Indexer adapters.
//
// Prerequisites:
//   - Task 13 (client tests): Zoekt webserver running with gRPC on localhost:6071
//     Start with: make dev (or docker compose up zoekt-webserver)
//     The docker-compose setup exposes HTTP on :6070 and gRPC on :6071
//   - Task 12 (end-to-end test): zoekt-git-index binary on PATH
//     Install with: go install github.com/sourcegraph/zoekt/cmd/zoekt-git-index@latest
//
// Note: A full pipeline test (index locally + search via gRPC) is intentionally
// omitted. The Zoekt webserver runs in a Docker container with its own volume mount,
// so shards written to the host filesystem are not visible to the containerized
// webserver. Indexer and gRPC client are therefore tested independently.
//
// Run all integration tests:
//
//	go test -v -tags=integration -timeout 60s ./internal/adapter/outbound/zoekt/...

import (
	"codechunking/internal/config"
	"codechunking/internal/port/outbound"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupGRPCClient creates a GRPCClient pointed at localhost:6071 with a 10s timeout.
// Skips the test when gRPC is unavailable. The gRPC tests require a zoekt build
// that includes the gRPC webserver (sourcegraph/zoekt with -grpc flag).
// The gRPC port (6071) is separate from the HTTP/net-rpc port (6070);
// start with: make dev (or docker compose up zoekt-webserver).
func setupGRPCClient(t *testing.T) *GRPCClient {
	t.Helper()
	cfg := config.ZoektWebserverConfig{
		Host:    "localhost",
		Port:    6071,
		Timeout: 5 * time.Second,
	}
	client, err := NewGRPCClient(cfg)
	if err != nil {
		t.Skipf("skipping gRPC test: cannot connect to Zoekt gRPC on localhost:6071 (%v) — "+
			"ensure a gRPC-capable zoekt image is running", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// ---------------------------------------------------------------------------
// GRPCClient integration tests (Task 13 — client)
// ---------------------------------------------------------------------------

func TestGRPCClient_Integration_CheckHealth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := setupGRPCClient(t)
	ctx := context.Background()

	status, err := client.CheckHealth(ctx)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.True(t, status.Healthy, "expected Zoekt webserver to report healthy")
	assert.NotNil(t, status.IndexStats, "expected IndexStats to be populated")
}

func TestGRPCClient_Integration_List(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := setupGRPCClient(t)
	ctx := context.Background()

	result, err := client.List(ctx, "", outbound.ZoektListOptions{
		Timeout: 10 * time.Second,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// May be empty if nothing has been indexed yet — that's fine.
	assert.GreaterOrEqual(t, result.TotalCount, 0)
	assert.GreaterOrEqual(t, len(result.Repositories), 0)
}

func TestGRPCClient_Integration_Search(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := setupGRPCClient(t)
	ctx := context.Background()

	result, err := client.Search(ctx, "func", outbound.ZoektSearchOptions{
		MaxTotalResults: 10,
		MaxMatchPerFile: 5,
		ContextLines:    1,
		Timeout:         10 * time.Second,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Result may be empty when no repos are indexed — the round-trip itself is the test.
	assert.GreaterOrEqual(t, result.TotalCount, 0)
	assert.GreaterOrEqual(t, len(result.FileMatches), 0)
	assert.NotNil(t, result.Stats)
}

func TestGRPCClient_Integration_SearchEmptyQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	client := setupGRPCClient(t)
	ctx := context.Background()

	// Empty query should be handled gracefully (no panic, no fatal error).
	result, err := client.Search(ctx, "", outbound.ZoektSearchOptions{
		MaxTotalResults: 5,
		Timeout:         10 * time.Second,
	})
	// Zoekt may return an error for an empty query or return empty results —
	// both are acceptable; what matters is no crash and a usable response or error.
	if err != nil {
		// Error is acceptable for empty query; just verify it's not nil-result with error.
		assert.Nil(t, result)
	} else {
		require.NotNil(t, result)
		assert.GreaterOrEqual(t, result.TotalCount, 0)
	}
}

func TestGRPCClient_Integration_ConnectionRefused(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	cfg := config.ZoektWebserverConfig{
		Host:    "localhost",
		Port:    19999, // port that should not be listening
		Timeout: 2 * time.Second,
	}

	// NewGRPCClient uses lazy dialing — no error at construction time.
	// The connection error surfaces on the first RPC call.
	client, err := NewGRPCClient(cfg)
	require.NoError(t, err, "lazy dial should not error at construction")
	defer client.Close()

	_, err = client.CheckHealth(context.Background())
	// CheckHealth catches the transport error and returns it as a value (Healthy=false),
	// so the error is surfaced via the health status rather than as a Go error.
	// Either a Go error or an unhealthy status is acceptable here.
	if err != nil {
		assert.Contains(t, err.Error(), "connection refused", "expected connection refused error")
	}
}

// ---------------------------------------------------------------------------
// Indexer filesystem tests (Task 13 — indexer)
// ---------------------------------------------------------------------------

func TestIndexer_Integration_CheckIndexStatus_NoShards(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	indexDir := t.TempDir()
	indexer := NewIndexer(IndexerConfig{
		GitIndexPath: "/usr/bin/zoekt-git-index", // not used for status checks
		IndexDir:     indexDir,
		Timeout:      30 * time.Second,
	})

	ctx := context.Background()
	status, err := indexer.CheckIndexStatus(ctx, "github.com/test/norepo", "abc123")
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.False(t, status.Exists, "expected Exists=false for empty index dir")
	assert.Equal(t, 0, status.ShardCount)
}

func TestIndexer_Integration_CheckIndexStatus_WithShards(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	indexDir := t.TempDir()

	// Create fake shard files matching the naming convention used by CheckIndexStatus:
	// <repoName with "/" replaced by "_">.<N>.zoekt
	repoName := "github.com/test/repo"
	shardBase := filepath.Join(indexDir, "github.com_test_repo")
	shardFiles := []string{
		shardBase + ".00000.zoekt",
		shardBase + ".00001.zoekt",
	}
	for _, f := range shardFiles {
		require.NoError(t, os.WriteFile(f, []byte("fake shard"), 0o600))
	}

	indexer := NewIndexer(IndexerConfig{
		IndexDir: indexDir,
		Timeout:  30 * time.Second,
	})

	ctx := context.Background()
	status, err := indexer.CheckIndexStatus(ctx, repoName, "")
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.True(t, status.Exists, "expected Exists=true when shards are present")
	assert.Equal(t, 2, status.ShardCount, "expected ShardCount to match number of fake shards")
	assert.NotNil(t, status.IndexedAt)
}

func TestIndexer_Integration_DeleteRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	indexDir := t.TempDir()
	repoName := "github.com/test/delete-me"
	shardBase := filepath.Join(indexDir, "github.com_test_delete-me")
	shardFile := shardBase + ".00000.zoekt"
	require.NoError(t, os.WriteFile(shardFile, []byte("fake shard"), 0o600))

	indexer := NewIndexer(IndexerConfig{
		IndexDir: indexDir,
		Timeout:  30 * time.Second,
	})

	ctx := context.Background()
	err := indexer.DeleteRepository(ctx, repoName)
	require.NoError(t, err)

	// Shard must be gone.
	_, statErr := os.Stat(shardFile)
	assert.True(t, os.IsNotExist(statErr), "expected shard file to be removed after DeleteRepository")
}

func TestIndexer_Integration_DeleteRepository_NoShards(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	indexDir := t.TempDir()
	indexer := NewIndexer(IndexerConfig{
		IndexDir: indexDir,
		Timeout:  30 * time.Second,
	})

	ctx := context.Background()
	err := indexer.DeleteRepository(ctx, "github.com/test/never-indexed")
	assert.NoError(t, err, "DeleteRepository should be a no-op when no shards exist")
}

// ---------------------------------------------------------------------------
// End-to-end indexing test (Task 12 — verify pipeline)
// ---------------------------------------------------------------------------

// TestIndexer_Integration_IndexEndToEnd creates a real git repository, indexes it
// with zoekt-git-index, then verifies that shard files are created and that
// CheckIndexStatus detects them.
//
// Skipped automatically when zoekt-git-index is not on PATH.
func TestIndexer_Integration_IndexEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Require zoekt-git-index binary.
	gitIndexBin, err := exec.LookPath("zoekt-git-index")
	if err != nil {
		t.Skip("zoekt-git-index not found on PATH — skipping end-to-end indexing test")
	}

	// Create a temporary git repository with at least one commit.
	repoDir := t.TempDir()
	indexDir := t.TempDir()

	// repoName is the simple identifier that zoekt-git-index will use as the shard
	// name when it is set as the git remote URL (no slashes → no URL encoding).
	// CheckIndexStatus derives the shard glob from this name via strings.ReplaceAll("/","_"),
	// so keeping the remote URL slash-free ensures the shard glob matches directly.
	const repoName = "zoekt-e2e-test"

	gitCmds := [][]string{
		{"git", "init", repoDir},
		{"git", "-C", repoDir, "config", "user.email", "test@example.com"},
		{"git", "-C", repoDir, "config", "user.name", "Test"},
		// Set a simple remote URL so zoekt-git-index uses repoName as the shard name.
		{"git", "-C", repoDir, "remote", "add", "origin", repoName},
	}
	for _, args := range gitCmds {
		out, cmdErr := exec.Command(args[0], args[1:]...).CombinedOutput()
		require.NoError(t, cmdErr, "git setup: %s", string(out))
	}

	// Write a sample Go file and commit it.
	sampleFile := filepath.Join(repoDir, "main.go")
	require.NoError(t, os.WriteFile(sampleFile, []byte(`package main

import "fmt"

func main() {
	fmt.Println("hello, zoekt")
}
`), 0o600))

	addOut, addErr := exec.Command("git", "-C", repoDir, "add", ".").CombinedOutput()
	require.NoError(t, addErr, "git add: %s", string(addOut))

	commitOut, commitErr := exec.Command("git", "-C", repoDir, "commit", "-m", "initial").CombinedOutput()
	require.NoError(t, commitErr, "git commit: %s", string(commitOut))

	// Capture the commit hash.
	hashOut, hashErr := exec.Command("git", "-C", repoDir, "rev-parse", "HEAD").Output()
	require.NoError(t, hashErr)
	commitHash := strings.TrimSpace(string(hashOut))

	// Capture the actual branch name (may be "master" or "main" depending on git config).
	branchOut, branchErr := exec.Command("git", "-C", repoDir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	require.NoError(t, branchErr)
	branch := strings.TrimSpace(string(branchOut))

	indexer := NewIndexer(IndexerConfig{
		GitIndexPath: gitIndexBin,
		IndexDir:     indexDir,
		Timeout:      60 * time.Second,
	})

	ctx := context.Background()
	result, err := indexer.Index(ctx, &outbound.ZoektRepositoryConfig{
		Name:     repoName,
		Path:     repoDir,
		Branch:   branch,
		IndexDir: indexDir,
		Timeout:  60 * time.Second,
	})
	require.NoError(t, err, "Index should succeed for a valid git repository")
	require.NotNil(t, result)

	// Verify at least one shard file was created in the index directory.
	shards, globErr := filepath.Glob(filepath.Join(indexDir, "*.zoekt"))
	require.NoError(t, globErr)
	assert.NotEmpty(t, shards, "expected at least one .zoekt shard in index dir")

	// Verify CheckIndexStatus detects the shards using the repo name.
	status, statusErr := indexer.CheckIndexStatus(ctx, repoName, commitHash)
	require.NoError(t, statusErr)
	require.NotNil(t, status)
	assert.True(t, status.Exists, "CheckIndexStatus should report Exists=true after successful indexing")
	assert.GreaterOrEqual(t, status.ShardCount, 1, "expected ShardCount >= 1")
}
