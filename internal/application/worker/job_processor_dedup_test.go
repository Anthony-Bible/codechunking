package worker

import (
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_deduplicateChunksByKey_WithDuplicates verifies that when given chunks
// with duplicate (repository_id, file_path, content_hash) keys, only the first
// occurrence of each unique key is retained.
func Test_deduplicateChunksByKey_WithDuplicates(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repoID := uuid.New()

	// Create chunks with some duplicates by (repository_id, file_path, hash)
	chunks := []outbound.CodeChunk{
		{
			ID:           "chunk-1",
			RepositoryID: repoID,
			FilePath:     "src/main.go",
			Hash:         "hash-abc123",
			Content:      "func main() { ... }",
			StartLine:    1,
			EndLine:      10,
		},
		{
			ID:           "chunk-2",
			RepositoryID: repoID,
			FilePath:     "src/main.go",
			Hash:         "hash-abc123", // Duplicate: same repo, file, hash
			Content:      "func main() { ... }",
			StartLine:    1,
			EndLine:      10,
		},
		{
			ID:           "chunk-3",
			RepositoryID: repoID,
			FilePath:     "src/util.go",
			Hash:         "hash-def456",
			Content:      "func helper() { ... }",
			StartLine:    5,
			EndLine:      15,
		},
		{
			ID:           "chunk-4",
			RepositoryID: repoID,
			FilePath:     "src/main.go",
			Hash:         "hash-xyz789",
			Content:      "func init() { ... }",
			StartLine:    20,
			EndLine:      25,
		},
		{
			ID:           "chunk-5",
			RepositoryID: repoID,
			FilePath:     "src/util.go",
			Hash:         "hash-def456", // Duplicate: same repo, file, hash
			Content:      "func helper() { ... }",
			StartLine:    5,
			EndLine:      15,
		},
	}

	// Act
	result := deduplicateChunksByKey(ctx, chunks)

	// Assert
	require.NotNil(t, result, "result should not be nil")
	assert.Len(t, result, 3, "should have 3 unique chunks (removed 2 duplicates)")

	// Verify we kept the first occurrence of each unique key
	assert.Equal(t, "chunk-1", result[0].ID, "first chunk should be chunk-1")
	assert.Equal(t, "chunk-3", result[1].ID, "second chunk should be chunk-3")
	assert.Equal(t, "chunk-4", result[2].ID, "third chunk should be chunk-4")

	// Verify the unique keys
	assert.Equal(t, "src/main.go", result[0].FilePath)
	assert.Equal(t, "hash-abc123", result[0].Hash)

	assert.Equal(t, "src/util.go", result[1].FilePath)
	assert.Equal(t, "hash-def456", result[1].Hash)

	assert.Equal(t, "src/main.go", result[2].FilePath)
	assert.Equal(t, "hash-xyz789", result[2].Hash)
}

// Test_deduplicateChunksByKey_NoDuplicates verifies that when given chunks
// with all unique keys, the function returns the same chunks unchanged.
func Test_deduplicateChunksByKey_NoDuplicates(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repoID := uuid.New()

	chunks := []outbound.CodeChunk{
		{
			ID:           "chunk-1",
			RepositoryID: repoID,
			FilePath:     "src/main.go",
			Hash:         "hash-abc123",
			Content:      "func main() { ... }",
			StartLine:    1,
			EndLine:      10,
		},
		{
			ID:           "chunk-2",
			RepositoryID: repoID,
			FilePath:     "src/util.go",
			Hash:         "hash-def456",
			Content:      "func helper() { ... }",
			StartLine:    5,
			EndLine:      15,
		},
		{
			ID:           "chunk-3",
			RepositoryID: repoID,
			FilePath:     "src/handlers.go",
			Hash:         "hash-xyz789",
			Content:      "func handler() { ... }",
			StartLine:    20,
			EndLine:      30,
		},
	}

	// Act
	result := deduplicateChunksByKey(ctx, chunks)

	// Assert
	require.NotNil(t, result, "result should not be nil")
	assert.Len(t, result, 3, "should have 3 chunks (no duplicates removed)")

	// Verify all chunks are preserved in order
	assert.Equal(t, "chunk-1", result[0].ID)
	assert.Equal(t, "chunk-2", result[1].ID)
	assert.Equal(t, "chunk-3", result[2].ID)
}

// Test_deduplicateChunksByKey_EmptySlice verifies that when given an empty
// slice of chunks, the function returns an empty slice without errors.
func Test_deduplicateChunksByKey_EmptySlice(t *testing.T) {
	// Arrange
	ctx := context.Background()
	chunks := []outbound.CodeChunk{}

	// Act
	result := deduplicateChunksByKey(ctx, chunks)

	// Assert
	require.NotNil(t, result, "result should not be nil")
	assert.Empty(t, result, "should return empty slice")
}

// Test_deduplicateChunksByKey_PreservesOrder verifies that the function
// preserves the order of first occurrences and keeps the first occurrence
// when duplicates are encountered.
func Test_deduplicateChunksByKey_PreservesOrder(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repoID := uuid.New()

	chunks := []outbound.CodeChunk{
		{
			ID:           "first-occurrence",
			RepositoryID: repoID,
			FilePath:     "src/main.go",
			Hash:         "hash-duplicate",
			Content:      "original content",
			StartLine:    1,
			EndLine:      5,
		},
		{
			ID:           "unique-chunk",
			RepositoryID: repoID,
			FilePath:     "src/util.go",
			Hash:         "hash-unique",
			Content:      "unique content",
			StartLine:    10,
			EndLine:      15,
		},
		{
			ID:           "second-occurrence",
			RepositoryID: repoID,
			FilePath:     "src/main.go",
			Hash:         "hash-duplicate", // Duplicate of first
			Content:      "duplicate content",
			StartLine:    1,
			EndLine:      5,
		},
		{
			ID:           "third-occurrence",
			RepositoryID: repoID,
			FilePath:     "src/main.go",
			Hash:         "hash-duplicate", // Another duplicate
			Content:      "another duplicate",
			StartLine:    1,
			EndLine:      5,
		},
	}

	// Act
	result := deduplicateChunksByKey(ctx, chunks)

	// Assert
	require.NotNil(t, result, "result should not be nil")
	assert.Len(t, result, 2, "should have 2 unique chunks")

	// Verify the first occurrence is kept, not later ones
	assert.Equal(t, "first-occurrence", result[0].ID, "should keep first occurrence of duplicate key")
	assert.Equal(t, "original content", result[0].Content, "should preserve content of first occurrence")

	// Verify the unique chunk is preserved
	assert.Equal(t, "unique-chunk", result[1].ID)
	assert.Equal(t, "unique content", result[1].Content)
}

// Test_deduplicateChunksByKey_DifferentRepositories verifies that chunks
// with the same file path and hash but different repository IDs are treated
// as unique (not duplicates).
func Test_deduplicateChunksByKey_DifferentRepositories(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repoID1 := uuid.New()
	repoID2 := uuid.New()

	chunks := []outbound.CodeChunk{
		{
			ID:           "chunk-repo1",
			RepositoryID: repoID1,
			FilePath:     "src/main.go",
			Hash:         "hash-same",
			Content:      "content in repo 1",
			StartLine:    1,
			EndLine:      10,
		},
		{
			ID:           "chunk-repo2",
			RepositoryID: repoID2,
			FilePath:     "src/main.go",
			Hash:         "hash-same", // Same file path and hash, but different repo
			Content:      "content in repo 2",
			StartLine:    1,
			EndLine:      10,
		},
	}

	// Act
	result := deduplicateChunksByKey(ctx, chunks)

	// Assert
	require.NotNil(t, result, "result should not be nil")
	assert.Len(t, result, 2, "should have 2 chunks (different repositories)")

	// Verify both chunks are preserved
	assert.Equal(t, "chunk-repo1", result[0].ID)
	assert.Equal(t, repoID1, result[0].RepositoryID)

	assert.Equal(t, "chunk-repo2", result[1].ID)
	assert.Equal(t, repoID2, result[1].RepositoryID)
}

// Test_deduplicateChunksByKey_DifferentFilePaths verifies that chunks
// with the same repository ID and hash but different file paths are treated
// as unique (not duplicates).
func Test_deduplicateChunksByKey_DifferentFilePaths(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repoID := uuid.New()

	chunks := []outbound.CodeChunk{
		{
			ID:           "chunk-file1",
			RepositoryID: repoID,
			FilePath:     "src/main.go",
			Hash:         "hash-same",
			Content:      "same hash different file",
			StartLine:    1,
			EndLine:      10,
		},
		{
			ID:           "chunk-file2",
			RepositoryID: repoID,
			FilePath:     "src/util.go", // Different file path
			Hash:         "hash-same",
			Content:      "same hash different file",
			StartLine:    1,
			EndLine:      10,
		},
	}

	// Act
	result := deduplicateChunksByKey(ctx, chunks)

	// Assert
	require.NotNil(t, result, "result should not be nil")
	assert.Len(t, result, 2, "should have 2 chunks (different file paths)")

	// Verify both chunks are preserved
	assert.Equal(t, "chunk-file1", result[0].ID)
	assert.Equal(t, "src/main.go", result[0].FilePath)

	assert.Equal(t, "chunk-file2", result[1].ID)
	assert.Equal(t, "src/util.go", result[1].FilePath)
}

// Test_deduplicateChunksByKey_DifferentHashes verifies that chunks
// with the same repository ID and file path but different hashes are treated
// as unique (not duplicates).
func Test_deduplicateChunksByKey_DifferentHashes(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repoID := uuid.New()

	chunks := []outbound.CodeChunk{
		{
			ID:           "chunk-hash1",
			RepositoryID: repoID,
			FilePath:     "src/main.go",
			Hash:         "hash-abc123",
			Content:      "version 1",
			StartLine:    1,
			EndLine:      10,
		},
		{
			ID:           "chunk-hash2",
			RepositoryID: repoID,
			FilePath:     "src/main.go",
			Hash:         "hash-def456", // Different hash
			Content:      "version 2",
			StartLine:    1,
			EndLine:      10,
		},
	}

	// Act
	result := deduplicateChunksByKey(ctx, chunks)

	// Assert
	require.NotNil(t, result, "result should not be nil")
	assert.Len(t, result, 2, "should have 2 chunks (different hashes)")

	// Verify both chunks are preserved
	assert.Equal(t, "chunk-hash1", result[0].ID)
	assert.Equal(t, "hash-abc123", result[0].Hash)

	assert.Equal(t, "chunk-hash2", result[1].ID)
	assert.Equal(t, "hash-def456", result[1].Hash)
}

// Test_deduplicateChunksByKey_AllDuplicates verifies that when all chunks
// are duplicates of the first one, only the first chunk is returned.
func Test_deduplicateChunksByKey_AllDuplicates(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repoID := uuid.New()

	chunks := []outbound.CodeChunk{
		{
			ID:           "first",
			RepositoryID: repoID,
			FilePath:     "src/main.go",
			Hash:         "hash-same",
			Content:      "content",
			StartLine:    1,
			EndLine:      10,
		},
		{
			ID:           "duplicate-1",
			RepositoryID: repoID,
			FilePath:     "src/main.go",
			Hash:         "hash-same",
			Content:      "content",
			StartLine:    1,
			EndLine:      10,
		},
		{
			ID:           "duplicate-2",
			RepositoryID: repoID,
			FilePath:     "src/main.go",
			Hash:         "hash-same",
			Content:      "content",
			StartLine:    1,
			EndLine:      10,
		},
		{
			ID:           "duplicate-3",
			RepositoryID: repoID,
			FilePath:     "src/main.go",
			Hash:         "hash-same",
			Content:      "content",
			StartLine:    1,
			EndLine:      10,
		},
	}

	// Act
	result := deduplicateChunksByKey(ctx, chunks)

	// Assert
	require.NotNil(t, result, "result should not be nil")
	assert.Len(t, result, 1, "should have only 1 chunk (all others are duplicates)")
	assert.Equal(t, "first", result[0].ID, "should keep the first occurrence")
}

// Test_deduplicateChunksByKey_SingleChunk verifies that when given a single
// chunk, the function returns it unchanged.
func Test_deduplicateChunksByKey_SingleChunk(t *testing.T) {
	// Arrange
	ctx := context.Background()
	repoID := uuid.New()

	chunks := []outbound.CodeChunk{
		{
			ID:           "only-chunk",
			RepositoryID: repoID,
			FilePath:     "src/main.go",
			Hash:         "hash-unique",
			Content:      "func main() { ... }",
			StartLine:    1,
			EndLine:      10,
		},
	}

	// Act
	result := deduplicateChunksByKey(ctx, chunks)

	// Assert
	require.NotNil(t, result, "result should not be nil")
	assert.Len(t, result, 1, "should have 1 chunk")
	assert.Equal(t, "only-chunk", result[0].ID)
	assert.Equal(t, "src/main.go", result[0].FilePath)
	assert.Equal(t, "hash-unique", result[0].Hash)
}
