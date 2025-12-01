//go:build integration

package repository

import (
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChunkRepository_SaveWithTokenCount tests saving chunks with token count information.
func TestChunkRepository_SaveWithTokenCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/token-count-repo-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "token-count-test-repo", "Test repository for token counting", "completed")
	require.NoError(t, err)

	tests := []struct {
		name             string
		chunk            *outbound.CodeChunk
		expectTokenCount int
		expectCountedAt  bool
		description      string
	}{
		{
			name: "save chunk with token count",
			chunk: &outbound.CodeChunk{
				ID:           uuid.New().String(),
				RepositoryID: repositoryID,
				FilePath:     "test_with_count.go",
				Content:      "func TestWithTokenCount() { fmt.Println(\"test\") }",
				Language:     "go",
				StartLine:    1,
				EndLine:      3,
				Hash:         "hash-with-count-1",
				Type:         "function",
				EntityName:   "TestWithTokenCount",
				CreatedAt:    time.Now(),
				TokenCount:   42,
				TokenCountedAt: func() *time.Time {
					t := time.Now()
					return &t
				}(),
			},
			expectTokenCount: 42,
			expectCountedAt:  true,
			description:      "Saving a chunk with token_count should persist the value",
		},
		{
			name: "save chunk without token count",
			chunk: &outbound.CodeChunk{
				ID:             uuid.New().String(),
				RepositoryID:   repositoryID,
				FilePath:       "test_without_count.go",
				Content:        "func TestWithoutTokenCount() { }",
				Language:       "go",
				StartLine:      1,
				EndLine:        1,
				Hash:           "hash-without-count-1",
				Type:           "function",
				EntityName:     "TestWithoutTokenCount",
				CreatedAt:      time.Now(),
				TokenCount:     0,
				TokenCountedAt: nil,
			},
			expectTokenCount: 0,
			expectCountedAt:  false,
			description:      "Saving a chunk without token_count should leave it NULL",
		},
		{
			name: "save chunk with zero token count but timestamp",
			chunk: &outbound.CodeChunk{
				ID:           uuid.New().String(),
				RepositoryID: repositoryID,
				FilePath:     "test_zero_count.go",
				Content:      "",
				Language:     "go",
				StartLine:    1,
				EndLine:      1,
				Hash:         "hash-zero-count-1",
				Type:         "fragment",
				EntityName:   "",
				CreatedAt:    time.Now(),
				TokenCount:   0,
				TokenCountedAt: func() *time.Time {
					t := time.Now()
					return &t
				}(),
			},
			expectTokenCount: 0,
			expectCountedAt:  true,
			description:      "Saving a chunk with zero token_count but timestamp should persist timestamp",
		},
		{
			name: "save chunk with large token count",
			chunk: &outbound.CodeChunk{
				ID:           uuid.New().String(),
				RepositoryID: repositoryID,
				FilePath:     "test_large_count.go",
				Content:      "// Very large function with many tokens\nfunc TestLargeTokenCount() { /* ... */ }",
				Language:     "go",
				StartLine:    1,
				EndLine:      50,
				Hash:         "hash-large-count-1",
				Type:         "function",
				EntityName:   "TestLargeTokenCount",
				CreatedAt:    time.Now(),
				TokenCount:   8192,
				TokenCountedAt: func() *time.Time {
					t := time.Now()
					return &t
				}(),
			},
			expectTokenCount: 8192,
			expectCountedAt:  true,
			description:      "Saving a chunk with large token_count should handle large values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save the chunk
			err := repo.SaveChunk(ctx, tt.chunk)
			require.NoError(t, err, tt.description)

			// Retrieve and verify
			chunkID, err := uuid.Parse(tt.chunk.ID)
			require.NoError(t, err)

			var tokenCount *int
			var tokenCountedAt *time.Time

			err = pool.QueryRow(ctx, `
				SELECT token_count, token_counted_at
				FROM codechunking.code_chunks
				WHERE id = $1 AND deleted_at IS NULL
			`, chunkID).Scan(&tokenCount, &tokenCountedAt)
			require.NoError(t, err)

			// Verify token count
			if tt.expectTokenCount > 0 {
				require.NotNil(t, tokenCount, "token_count should not be NULL")
				assert.Equal(t, tt.expectTokenCount, *tokenCount, "token_count should match expected value")
			} else if tokenCount != nil {
				assert.Equal(t, 0, *tokenCount, "token_count should be 0 if set")
			}

			// Verify timestamp
			if tt.expectCountedAt {
				require.NotNil(t, tokenCountedAt, "token_counted_at should not be NULL")
				// Verify timestamp is recent (within last minute)
				assert.WithinDuration(t, time.Now(), *tokenCountedAt, time.Minute, "token_counted_at should be recent")
			} else {
				assert.Nil(t, tokenCountedAt, "token_counted_at should be NULL")
			}
		})
	}

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestChunkRepository_UpdateTokenCount tests updating token count on existing chunks.
func TestChunkRepository_UpdateTokenCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/update-token-repo-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "update-token-test-repo", "Test repository for updating token counts", "completed")
	require.NoError(t, err)

	// Create initial chunk without token count
	initialChunk := &outbound.CodeChunk{
		ID:             uuid.New().String(),
		RepositoryID:   repositoryID,
		FilePath:       "test_update.go",
		Content:        "func TestUpdate() { }",
		Language:       "go",
		StartLine:      1,
		EndLine:        1,
		Hash:           "hash-update-1",
		Type:           "function",
		EntityName:     "TestUpdate",
		CreatedAt:      time.Now(),
		TokenCount:     0,
		TokenCountedAt: nil,
	}

	err = repo.SaveChunk(ctx, initialChunk)
	require.NoError(t, err)

	// Verify initial state (no token count)
	chunkID, err := uuid.Parse(initialChunk.ID)
	require.NoError(t, err)

	var tokenCount *int
	var tokenCountedAt *time.Time

	err = pool.QueryRow(ctx, `
		SELECT token_count, token_counted_at
		FROM codechunking.code_chunks
		WHERE id = $1 AND deleted_at IS NULL
	`, chunkID).Scan(&tokenCount, &tokenCountedAt)
	require.NoError(t, err)
	assert.Nil(t, tokenCountedAt, "initial token_counted_at should be NULL")

	// Update chunk with token count
	now := time.Now()
	updatedChunk := &outbound.CodeChunk{
		ID:             initialChunk.ID,
		RepositoryID:   repositoryID,
		FilePath:       "test_update.go",
		Content:        "func TestUpdate() { }",
		Language:       "go",
		StartLine:      1,
		EndLine:        1,
		Hash:           "hash-update-1",
		Type:           "function",
		EntityName:     "TestUpdate",
		CreatedAt:      initialChunk.CreatedAt,
		TokenCount:     15,
		TokenCountedAt: &now,
	}

	err = repo.SaveChunk(ctx, updatedChunk)
	require.NoError(t, err, "Updating token_count on existing chunk should work")

	// Verify updated state
	err = pool.QueryRow(ctx, `
		SELECT token_count, token_counted_at
		FROM codechunking.code_chunks
		WHERE id = $1 AND deleted_at IS NULL
	`, chunkID).Scan(&tokenCount, &tokenCountedAt)
	require.NoError(t, err)

	require.NotNil(t, tokenCount, "updated token_count should not be NULL")
	assert.Equal(t, 15, *tokenCount, "token_count should be updated to new value")
	require.NotNil(t, tokenCountedAt, "updated token_counted_at should not be NULL")
	assert.WithinDuration(t, now, *tokenCountedAt, time.Second, "token_counted_at should match update time")

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestChunkRepository_GetChunksWithTokenCount tests retrieving chunks with token count information.
func TestChunkRepository_GetChunksWithTokenCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/get-token-repo-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "get-token-test-repo", "Test repository for getting token counts", "completed")
	require.NoError(t, err)

	// Create chunks with varying token counts
	now := time.Now()
	chunks := []outbound.CodeChunk{
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "with_count_1.go",
			Content:        "func Test1() { }",
			Language:       "go",
			StartLine:      1,
			EndLine:        1,
			Hash:           "hash-get-1",
			Type:           "function",
			EntityName:     "Test1",
			CreatedAt:      now,
			TokenCount:     10,
			TokenCountedAt: &now,
		},
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "without_count.go",
			Content:        "func Test2() { }",
			Language:       "go",
			StartLine:      1,
			EndLine:        1,
			Hash:           "hash-get-2",
			Type:           "function",
			EntityName:     "Test2",
			CreatedAt:      now,
			TokenCount:     0,
			TokenCountedAt: nil,
		},
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "with_count_2.go",
			Content:        "func Test3() { }",
			Language:       "go",
			StartLine:      1,
			EndLine:        1,
			Hash:           "hash-get-3",
			Type:           "function",
			EntityName:     "Test3",
			CreatedAt:      now,
			TokenCount:     25,
			TokenCountedAt: &now,
		},
	}

	// Save all chunks
	err = repo.SaveChunks(ctx, chunks)
	require.NoError(t, err)

	t.Run("GetChunk should include token_count if set", func(t *testing.T) {
		chunkID, err := uuid.Parse(chunks[0].ID)
		require.NoError(t, err)

		chunk, err := repo.GetChunk(ctx, chunkID)
		require.NoError(t, err, "Retrieved chunk should include token_count if set")

		assert.Equal(t, 10, chunk.TokenCount, "token_count should be 10")
		require.NotNil(t, chunk.TokenCountedAt, "token_counted_at should not be nil")
		assert.WithinDuration(t, now, *chunk.TokenCountedAt, time.Second, "token_counted_at should match saved time")
	})

	t.Run("GetChunk should have nil/zero token_count if not set", func(t *testing.T) {
		chunkID, err := uuid.Parse(chunks[1].ID)
		require.NoError(t, err)

		chunk, err := repo.GetChunk(ctx, chunkID)
		require.NoError(t, err, "Retrieved chunk should have nil/zero token_count if not set")

		assert.Equal(t, 0, chunk.TokenCount, "token_count should be 0 or nil")
		assert.Nil(t, chunk.TokenCountedAt, "token_counted_at should be nil")
	})

	t.Run("GetChunksForRepository should include token_count information", func(t *testing.T) {
		retrievedChunks, err := repo.GetChunksForRepository(ctx, repositoryID)
		require.NoError(t, err, "Retrieved chunks should include token_count if set")
		require.Len(t, retrievedChunks, 3, "should retrieve all 3 chunks")

		// Verify token counts are preserved
		tokenCountMap := make(map[string]int)
		for _, chunk := range retrievedChunks {
			tokenCountMap[chunk.ID] = chunk.TokenCount
		}

		assert.Equal(t, 10, tokenCountMap[chunks[0].ID], "first chunk should have token_count 10")
		assert.Equal(t, 0, tokenCountMap[chunks[1].ID], "second chunk should have token_count 0")
		assert.Equal(t, 25, tokenCountMap[chunks[2].ID], "third chunk should have token_count 25")
	})

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestChunkRepository_BatchOperationsWithTokenCount tests batch operations with token counts.
func TestChunkRepository_BatchOperationsWithTokenCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/batch-token-repo-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "batch-token-test-repo", "Test repository for batch token operations", "completed")
	require.NoError(t, err)

	t.Run("SaveChunks should preserve token_count for all chunks", func(t *testing.T) {
		now := time.Now()
		chunks := make([]outbound.CodeChunk, 10)
		for i := range chunks {
			chunks[i] = outbound.CodeChunk{
				ID:             uuid.New().String(),
				RepositoryID:   repositoryID,
				FilePath:       "batch_test.go",
				Content:        "func Test() { }",
				Language:       "go",
				StartLine:      i + 1,
				EndLine:        i + 1,
				Hash:           "hash-batch-" + uuid.New().String()[:8],
				Type:           "function",
				EntityName:     "Test",
				CreatedAt:      now,
				TokenCount:     (i + 1) * 10, // 10, 20, 30, ...
				TokenCountedAt: &now,
			}
		}

		err := repo.SaveChunks(ctx, chunks)
		require.NoError(t, err, "Batch save should preserve token counts")

		// Verify all chunks have correct token counts
		for i, chunk := range chunks {
			chunkID, err := uuid.Parse(chunk.ID)
			require.NoError(t, err)

			var tokenCount *int
			err = pool.QueryRow(ctx, `
				SELECT token_count
				FROM codechunking.code_chunks
				WHERE id = $1 AND deleted_at IS NULL
			`, chunkID).Scan(&tokenCount)
			require.NoError(t, err)

			expectedCount := (i + 1) * 10
			require.NotNil(t, tokenCount, "chunk %d should have token_count", i)
			assert.Equal(t, expectedCount, *tokenCount, "chunk %d should have correct token_count", i)
		}
	})

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestChunkRepository_FindOrCreateChunksWithTokenCount tests FindOrCreateChunks with token counts.
func TestChunkRepository_FindOrCreateChunksWithTokenCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/find-create-token-repo-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "find-create-token-test-repo", "Test repository for find/create with token counts", "completed")
	require.NoError(t, err)

	now := time.Now()

	t.Run("FindOrCreateChunks should preserve token_count on new chunks", func(t *testing.T) {
		chunks := []outbound.CodeChunk{
			{
				ID:             uuid.New().String(),
				RepositoryID:   repositoryID,
				FilePath:       "find_create_new.go",
				Content:        "func New() { }",
				Language:       "go",
				StartLine:      1,
				EndLine:        1,
				Hash:           "hash-find-create-new",
				Type:           "function",
				EntityName:     "New",
				CreatedAt:      now,
				TokenCount:     50,
				TokenCountedAt: &now,
			},
		}

		resultChunks, err := repo.FindOrCreateChunks(ctx, chunks)
		require.NoError(t, err, "FindOrCreateChunks should work with token_count")
		require.Len(t, resultChunks, 1)

		// Verify token count was saved
		chunkID, err := uuid.Parse(resultChunks[0].ID)
		require.NoError(t, err)

		var tokenCount *int
		err = pool.QueryRow(ctx, `
			SELECT token_count
			FROM codechunking.code_chunks
			WHERE id = $1 AND deleted_at IS NULL
		`, chunkID).Scan(&tokenCount)
		require.NoError(t, err)

		require.NotNil(t, tokenCount, "token_count should be saved")
		assert.Equal(t, 50, *tokenCount, "token_count should match")
	})

	t.Run("FindOrCreateChunks should return existing chunk token_count", func(t *testing.T) {
		// Create initial chunk with token count
		initialChunk := outbound.CodeChunk{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "find_create_existing.go",
			Content:        "func Existing() { }",
			Language:       "go",
			StartLine:      1,
			EndLine:        1,
			Hash:           "hash-find-create-existing",
			Type:           "function",
			EntityName:     "Existing",
			CreatedAt:      now,
			TokenCount:     100,
			TokenCountedAt: &now,
		}

		err := repo.SaveChunk(ctx, &initialChunk)
		require.NoError(t, err)

		// Try to create duplicate with different token count
		duplicateChunk := []outbound.CodeChunk{
			{
				ID:             uuid.New().String(), // Different ID
				RepositoryID:   repositoryID,
				FilePath:       "find_create_existing.go",
				Content:        "func Existing() { }",
				Language:       "go",
				StartLine:      1,
				EndLine:        1,
				Hash:           "hash-find-create-existing", // Same hash
				Type:           "function",
				EntityName:     "Existing",
				CreatedAt:      now,
				TokenCount:     999, // Different token count (should be ignored)
				TokenCountedAt: &now,
			},
		}

		resultChunks, err := repo.FindOrCreateChunks(ctx, duplicateChunk)
		require.NoError(t, err, "FindOrCreateChunks should return existing chunk")
		require.Len(t, resultChunks, 1)

		// Verify original token count is preserved
		chunkID, err := uuid.Parse(resultChunks[0].ID)
		require.NoError(t, err)

		var tokenCount *int
		err = pool.QueryRow(ctx, `
			SELECT token_count
			FROM codechunking.code_chunks
			WHERE id = $1 AND deleted_at IS NULL
		`, chunkID).Scan(&tokenCount)
		require.NoError(t, err)

		require.NotNil(t, tokenCount, "existing token_count should be preserved")
		assert.Equal(t, 100, *tokenCount, "original token_count should be preserved, not updated")
	})

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}
