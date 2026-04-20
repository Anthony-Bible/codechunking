//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindChunksByRepositoryPathAndLineRange(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	repositoryID := uuid.New()
	repositoryName := "github.com/example/line-lookup-" + repositoryID.String()[:8]
	repositoryURL := "https://github.com/example/line-lookup-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, repositoryURL, repositoryURL, repositoryName, "Test repository for chunk line lookup", "completed")
	require.NoError(t, err)

	chunkID1 := uuid.New()
	chunkID2 := uuid.New()
	chunkID3 := uuid.New()

	_, err = pool.Exec(ctx, `
		INSERT INTO codechunking.code_chunks (
			id, repository_id, file_path, chunk_type, content, language,
			start_line, end_line, entity_name, content_hash
		) VALUES
			($1,  $2, $3, $4, $5,  $6, $7,  $8,  $9,  $10),
			($11, $2, $3, $4, $12, $6, $13, $14, $15, $16),
			($17, $2, $3, $4, $18, $6, $19, $20, $21, $22)
	`, chunkID1, repositoryID, "server/handler.go", "function", "func helper() {}", "go", 1, 14, "helper", "hash-1",
		chunkID2, "func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {}", 10, 18, "ServeHTTP", "hash-2",
		chunkID3, "func tail() {}", 20, 30, "tail", "hash-3")
	require.NoError(t, err)

	chunks, err := repo.FindChunksByRepositoryPathAndLineRange(ctx, repositoryName, "server/handler.go", 12, 16)
	require.NoError(t, err)
	require.Len(t, chunks, 2)

	assert.Equal(t, chunkID2, chunks[0].ChunkID)
	assert.Equal(t, chunkID1, chunks[1].ChunkID)

	pathChunks, err := repo.FindChunksByRepositoryPath(ctx, repositoryName, "server/handler.go")
	require.NoError(t, err)
	require.Len(t, pathChunks, 3)

	assert.Equal(t, chunkID1, pathChunks[0].ChunkID)
	assert.Equal(t, chunkID2, pathChunks[1].ChunkID)
	assert.Equal(t, chunkID3, pathChunks[2].ChunkID)

	empty, err := repo.FindChunksByRepositoryPathAndLineRange(ctx, repositoryName, "server/handler.go", 0, 0)
	require.NoError(t, err)
	assert.Empty(t, empty)

	emptyPath, err := repo.FindChunksByRepositoryPath(ctx, repositoryName, "server/missing.go")
	require.NoError(t, err)
	assert.Empty(t, emptyPath)

	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}
