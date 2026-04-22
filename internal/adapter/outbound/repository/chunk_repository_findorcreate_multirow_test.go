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

// TestFindOrCreateChunks_MultiRow_AllNewChunks tests that FindOrCreateChunks returns generated UUIDs for all new chunks.
// This test verifies that when all chunks are new, the function assigns and returns valid UUIDs for each chunk.
func TestFindOrCreateChunks_MultiRow_AllNewChunks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/all-new-chunks-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "all-new-chunks-repo", "Test repository for all new chunks", "completed")
	require.NoError(t, err)

	// Create 5 new chunks (none exist in DB yet)
	now := time.Now()
	inputChunks := make([]outbound.CodeChunk, 5)
	for i := range inputChunks {
		inputChunks[i] = outbound.CodeChunk{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "new_chunks.go",
			Content:        "func NewFunc" + string(rune('A'+i)) + "() { }",
			Language:       "go",
			StartLine:      i*2 + 1,
			EndLine:        i*2 + 1,
			Hash:           "hash-new-" + uuid.New().String()[:8],
			Type:           "function",
			EntityName:     "NewFunc" + string(rune('A'+i)),
			CreatedAt:      now,
			TokenCount:     0,
			TokenCountedAt: nil,
		}
	}

	// Call FindOrCreateChunks - should create all chunks
	resultChunks, err := repo.FindOrCreateChunks(ctx, inputChunks)
	require.NoError(t, err, "FindOrCreateChunks should succeed for all new chunks")
	require.NotNil(t, resultChunks, "Result should not be nil")
	require.Len(t, resultChunks, 5, "Should return 5 chunks")

	// Verify all chunks were created with their generated IDs
	for i, resultChunk := range resultChunks {
		assert.NotEmpty(t, resultChunk.ID, "Chunk %d should have an ID", i)

		// Verify the chunk exists in the database with the returned ID
		var exists bool
		err = pool.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM codechunking.code_chunks WHERE id = $1 AND deleted_at IS NULL)
		`, resultChunk.ID).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "Chunk %d with ID %s should exist in database", i, resultChunk.ID)

		// Verify content matches
		var retrievedContent string
		err = pool.QueryRow(ctx, `
			SELECT content FROM codechunking.code_chunks WHERE id = $1 AND deleted_at IS NULL
		`, resultChunk.ID).Scan(&retrievedContent)
		require.NoError(t, err)
		assert.Equal(t, inputChunks[i].Content, retrievedContent, "Content should match for chunk %d", i)
	}

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestFindOrCreateChunks_MultiRow_AllExistingChunks tests that FindOrCreateChunks returns existing IDs for all existing chunks.
// This test verifies that when all chunks already exist (based on repo/path/hash), the function returns their existing IDs.
func TestFindOrCreateChunks_MultiRow_AllExistingChunks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/all-existing-chunks-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "all-existing-chunks-repo", "Test repository for all existing chunks", "completed")
	require.NoError(t, err)

	// Create and save initial chunks
	now := time.Now()
	existingChunks := make([]outbound.CodeChunk, 3)
	existingIDs := make([]string, 3)
	for i := range existingChunks {
		chunkID := uuid.New()
		existingIDs[i] = chunkID.String()

		existingChunks[i] = outbound.CodeChunk{
			ID:             chunkID.String(),
			RepositoryID:   repositoryID,
			FilePath:       "existing.go",
			Content:        "func ExistingFunc" + string(rune('A'+i)) + "() { }",
			Language:       "go",
			StartLine:      i*2 + 1,
			EndLine:        i*2 + 1,
			Hash:           "hash-existing-" + string(rune('1'+i)), // Stable hashes
			Type:           "function",
			EntityName:     "ExistingFunc" + string(rune('A'+i)),
			CreatedAt:      now,
			TokenCount:     100 + i*10, // Token counts: 100, 110, 120
			TokenCountedAt: &now,
		}
	}

	// Save the chunks first
	err = repo.SaveChunks(ctx, existingChunks)
	require.NoError(t, err, "SaveChunks should succeed")

	// Now call FindOrCreateChunks with chunks that have the SAME repo/path/hash but DIFFERENT IDs
	duplicateChunks := make([]outbound.CodeChunk, 3)
	for i := range duplicateChunks {
		duplicateChunks[i] = outbound.CodeChunk{
			ID:             uuid.New().String(), // DIFFERENT ID (should be ignored)
			RepositoryID:   repositoryID,
			FilePath:       "existing.go",
			Content:        "func ExistingFunc" + string(rune('A'+i)) + "() { }",
			Language:       "go",
			StartLine:      i*2 + 1,
			EndLine:        i*2 + 1,
			Hash:           "hash-existing-" + string(rune('1'+i)), // SAME hash (triggers conflict)
			Type:           "function",
			EntityName:     "ExistingFunc" + string(rune('A'+i)),
			CreatedAt:      now,
			TokenCount:     999, // Different token count (should be ignored)
			TokenCountedAt: &now,
		}
	}

	// Call FindOrCreateChunks - should return existing IDs
	resultChunks, err := repo.FindOrCreateChunks(ctx, duplicateChunks)
	require.NoError(t, err, "FindOrCreateChunks should succeed for all existing chunks")
	require.NotNil(t, resultChunks, "Result should not be nil")
	require.Len(t, resultChunks, 3, "Should return 3 chunks")

	// Verify that returned IDs match the ORIGINAL existing IDs, not the new ones we provided
	for i, resultChunk := range resultChunks {
		assert.Equal(t, existingIDs[i], resultChunk.ID, "Chunk %d should return existing ID", i)
		assert.NotEqual(t, duplicateChunks[i].ID, resultChunk.ID, "Chunk %d should NOT use new ID", i)
	}

	// Verify only 3 chunks exist in total (no duplicates created)
	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM codechunking.code_chunks
		WHERE repository_id = $1 AND deleted_at IS NULL
	`, repositoryID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 3, count, "Should have exactly 3 chunks (no duplicates)")

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestFindOrCreateChunks_MultiRow_MixedNewAndExisting tests that FindOrCreateChunks handles a mix of new and existing chunks.
// This test verifies that the function correctly identifies existing chunks and creates new ones in a single batch.
func TestFindOrCreateChunks_MultiRow_MixedNewAndExisting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/mixed-chunks-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "mixed-chunks-repo", "Test repository for mixed chunks", "completed")
	require.NoError(t, err)

	// Create and save 2 existing chunks
	now := time.Now()
	existingChunks := []outbound.CodeChunk{
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "mixed.go",
			Content:        "func ExistingA() { }",
			Language:       "go",
			StartLine:      1,
			EndLine:        1,
			Hash:           "hash-existing-A",
			Type:           "function",
			EntityName:     "ExistingA",
			CreatedAt:      now,
			TokenCount:     50,
			TokenCountedAt: &now,
		},
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "mixed.go",
			Content:        "func ExistingB() { }",
			Language:       "go",
			StartLine:      5,
			EndLine:        5,
			Hash:           "hash-existing-B",
			Type:           "function",
			EntityName:     "ExistingB",
			CreatedAt:      now,
			TokenCount:     60,
			TokenCountedAt: &now,
		},
	}

	err = repo.SaveChunks(ctx, existingChunks)
	require.NoError(t, err, "SaveChunks should succeed")

	// Store existing IDs for verification
	existingID_A := existingChunks[0].ID
	existingID_B := existingChunks[1].ID

	// Create mixed batch: [new, existing-A, new, existing-B, new]
	mixedChunks := []outbound.CodeChunk{
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "mixed.go",
			Content:        "func NewC() { }",
			Language:       "go",
			StartLine:      10,
			EndLine:        10,
			Hash:           "hash-new-C",
			Type:           "function",
			EntityName:     "NewC",
			CreatedAt:      now,
			TokenCount:     0,
			TokenCountedAt: nil,
		},
		{
			ID:             uuid.New().String(), // Different ID
			RepositoryID:   repositoryID,
			FilePath:       "mixed.go",
			Content:        "func ExistingA() { }",
			Language:       "go",
			StartLine:      1,
			EndLine:        1,
			Hash:           "hash-existing-A", // Same hash as existing chunk
			Type:           "function",
			EntityName:     "ExistingA",
			CreatedAt:      now,
			TokenCount:     999,
			TokenCountedAt: &now,
		},
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "mixed.go",
			Content:        "func NewD() { }",
			Language:       "go",
			StartLine:      15,
			EndLine:        15,
			Hash:           "hash-new-D",
			Type:           "function",
			EntityName:     "NewD",
			CreatedAt:      now,
			TokenCount:     0,
			TokenCountedAt: nil,
		},
		{
			ID:             uuid.New().String(), // Different ID
			RepositoryID:   repositoryID,
			FilePath:       "mixed.go",
			Content:        "func ExistingB() { }",
			Language:       "go",
			StartLine:      5,
			EndLine:        5,
			Hash:           "hash-existing-B", // Same hash as existing chunk
			Type:           "function",
			EntityName:     "ExistingB",
			CreatedAt:      now,
			TokenCount:     999,
			TokenCountedAt: &now,
		},
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "mixed.go",
			Content:        "func NewE() { }",
			Language:       "go",
			StartLine:      20,
			EndLine:        20,
			Hash:           "hash-new-E",
			Type:           "function",
			EntityName:     "NewE",
			CreatedAt:      now,
			TokenCount:     0,
			TokenCountedAt: nil,
		},
	}

	// Call FindOrCreateChunks
	resultChunks, err := repo.FindOrCreateChunks(ctx, mixedChunks)
	require.NoError(t, err, "FindOrCreateChunks should succeed for mixed chunks")
	require.NotNil(t, resultChunks, "Result should not be nil")
	require.Len(t, resultChunks, 5, "Should return 5 chunks")

	// Verify that existing chunks return existing IDs
	assert.Equal(t, existingID_A, resultChunks[1].ID, "Index 1 should return existing ID for chunk A")
	assert.Equal(t, existingID_B, resultChunks[3].ID, "Index 3 should return existing ID for chunk B")

	// Verify that new chunks have valid IDs (not the ones we provided)
	assert.NotEmpty(t, resultChunks[0].ID, "Index 0 should have an ID")
	assert.NotEmpty(t, resultChunks[2].ID, "Index 2 should have an ID")
	assert.NotEmpty(t, resultChunks[4].ID, "Index 4 should have an ID")

	// Verify total count (should be 5: 2 existing + 3 new)
	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM codechunking.code_chunks
		WHERE repository_id = $1 AND deleted_at IS NULL
	`, repositoryID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 5, count, "Should have exactly 5 chunks total")

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestFindOrCreateChunks_MultiRow_ReturningOrderMatchesInput tests that returned chunk IDs match input order.
// This is CRITICAL for batch embedding workflow - chunks[i] must get result[i]'s ID.
func TestFindOrCreateChunks_MultiRow_ReturningOrderMatchesInput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/order-preservation-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "order-preservation-repo", "Test repository for order preservation", "completed")
	require.NoError(t, err)

	// Create chunks with unique, identifiable content
	now := time.Now()
	inputChunks := []outbound.CodeChunk{
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "order.go",
			Content:        "func OrderA() { }",
			Language:       "go",
			StartLine:      1,
			EndLine:        1,
			Hash:           "hash-order-A",
			Type:           "function",
			EntityName:     "OrderA",
			CreatedAt:      now,
			TokenCount:     0,
			TokenCountedAt: nil,
		},
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "order.go",
			Content:        "func OrderB() { }",
			Language:       "go",
			StartLine:      5,
			EndLine:        5,
			Hash:           "hash-order-B",
			Type:           "function",
			EntityName:     "OrderB",
			CreatedAt:      now,
			TokenCount:     0,
			TokenCountedAt: nil,
		},
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "order.go",
			Content:        "func OrderC() { }",
			Language:       "go",
			StartLine:      10,
			EndLine:        10,
			Hash:           "hash-order-C",
			Type:           "function",
			EntityName:     "OrderC",
			CreatedAt:      now,
			TokenCount:     0,
			TokenCountedAt: nil,
		},
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "order.go",
			Content:        "func OrderD() { }",
			Language:       "go",
			StartLine:      15,
			EndLine:        15,
			Hash:           "hash-order-D",
			Type:           "function",
			EntityName:     "OrderD",
			CreatedAt:      now,
			TokenCount:     0,
			TokenCountedAt: nil,
		},
	}

	// Call FindOrCreateChunks
	resultChunks, err := repo.FindOrCreateChunks(ctx, inputChunks)
	require.NoError(t, err, "FindOrCreateChunks should succeed")
	require.NotNil(t, resultChunks, "Result should not be nil")
	require.Len(t, resultChunks, 4, "Should return 4 chunks")

	// CRITICAL: Verify that each result chunk corresponds to the input chunk at the same index
	// This is verified by checking that the content matches
	for i, resultChunk := range resultChunks {
		// Query the database to get the content for the returned ID
		var retrievedContent string
		var retrievedEntityName string
		err = pool.QueryRow(ctx, `
			SELECT content, entity_name
			FROM codechunking.code_chunks
			WHERE id = $1 AND deleted_at IS NULL
		`, resultChunk.ID).Scan(&retrievedContent, &retrievedEntityName)
		require.NoError(t, err, "Should retrieve chunk %d", i)

		// The content at result[i] must match input[i]
		assert.Equal(t, inputChunks[i].Content, retrievedContent,
			"Result chunk at index %d must correspond to input chunk at index %d (content mismatch)", i, i)
		assert.Equal(t, inputChunks[i].EntityName, retrievedEntityName,
			"Result chunk at index %d must correspond to input chunk at index %d (entity name mismatch)", i, i)

		// Also verify the in-memory result chunk has matching content
		assert.Equal(t, inputChunks[i].Content, resultChunk.Content,
			"In-memory result chunk at index %d should preserve input content", i)
	}

	// Now test with mixed new/existing to ensure order is still preserved
	// Create a new batch that references some existing chunks in different order
	mixedInputChunks := []outbound.CodeChunk{
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "order.go",
			Content:        "func OrderC() { }", // Existing chunk (index 2 from before)
			Language:       "go",
			StartLine:      10,
			EndLine:        10,
			Hash:           "hash-order-C",
			Type:           "function",
			EntityName:     "OrderC",
			CreatedAt:      now,
			TokenCount:     0,
			TokenCountedAt: nil,
		},
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "order.go",
			Content:        "func OrderE() { }", // New chunk
			Language:       "go",
			StartLine:      20,
			EndLine:        20,
			Hash:           "hash-order-E",
			Type:           "function",
			EntityName:     "OrderE",
			CreatedAt:      now,
			TokenCount:     0,
			TokenCountedAt: nil,
		},
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "order.go",
			Content:        "func OrderA() { }", // Existing chunk (index 0 from before)
			Language:       "go",
			StartLine:      1,
			EndLine:        1,
			Hash:           "hash-order-A",
			Type:           "function",
			EntityName:     "OrderA",
			CreatedAt:      now,
			TokenCount:     0,
			TokenCountedAt: nil,
		},
	}

	// Call FindOrCreateChunks again
	mixedResultChunks, err := repo.FindOrCreateChunks(ctx, mixedInputChunks)
	require.NoError(t, err, "FindOrCreateChunks should succeed for mixed batch")
	require.NotNil(t, mixedResultChunks, "Result should not be nil")
	require.Len(t, mixedResultChunks, 3, "Should return 3 chunks")

	// Verify order preservation for mixed batch
	for i, resultChunk := range mixedResultChunks {
		var retrievedContent string
		err = pool.QueryRow(ctx, `
			SELECT content
			FROM codechunking.code_chunks
			WHERE id = $1 AND deleted_at IS NULL
		`, resultChunk.ID).Scan(&retrievedContent)
		require.NoError(t, err, "Should retrieve chunk %d from mixed batch", i)

		assert.Equal(t, mixedInputChunks[i].Content, retrievedContent,
			"Mixed batch: Result chunk at index %d must correspond to input chunk at index %d", i, i)
	}

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestFindOrCreateChunks_MultiRow_LargeBatch tests that FindOrCreateChunks handles a large batch correctly.
// This test verifies that the function can process 100+ chunks efficiently (typical batch embedding size).
func TestFindOrCreateChunks_MultiRow_LargeBatch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/large-batch-findorcreate-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "large-batch-findorcreate-repo", "Test repository for large batch FindOrCreateChunks", "completed")
	require.NoError(t, err)

	// Create 100 chunks
	now := time.Now()
	largeChunks := make([]outbound.CodeChunk, 100)
	for i := range largeChunks {
		largeChunks[i] = outbound.CodeChunk{
			ID:           uuid.New().String(),
			RepositoryID: repositoryID,
			FilePath:     "large_batch.go",
			Content: "func LargeFunc" + string(
				rune('0'+(i%10)),
			) + "() { /* chunk " + string(
				rune('0'+(i/10)),
			) + " */ }",
			Language:       "go",
			StartLine:      i*2 + 1,
			EndLine:        i*2 + 1,
			Hash:           "hash-large-findorcreate-" + uuid.New().String()[:8],
			Type:           "function",
			EntityName:     "LargeFunc" + string(rune('0'+(i%10))),
			CreatedAt:      now,
			TokenCount:     0,
			TokenCountedAt: nil,
		}
	}

	// Call FindOrCreateChunks - should create all 100 chunks
	startTime := time.Now()
	resultChunks, err := repo.FindOrCreateChunks(ctx, largeChunks)
	duration := time.Since(startTime)
	require.NoError(t, err, "FindOrCreateChunks should succeed for large batch")

	t.Logf("FindOrCreateChunks for 100 chunks took: %v", duration)

	require.NotNil(t, resultChunks, "Result should not be nil")
	require.Len(t, resultChunks, 100, "Should return 100 chunks")

	// Verify all chunks were created
	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM codechunking.code_chunks
		WHERE repository_id = $1 AND deleted_at IS NULL
	`, repositoryID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 100, count, "Should have exactly 100 chunks")

	// Spot check order preservation for first, middle, and last chunks
	testIndices := []int{0, 50, 99}
	for _, i := range testIndices {
		var retrievedContent string
		err = pool.QueryRow(ctx, `
			SELECT content
			FROM codechunking.code_chunks
			WHERE id = $1 AND deleted_at IS NULL
		`, resultChunks[i].ID).Scan(&retrievedContent)
		require.NoError(t, err, "Should retrieve chunk %d", i)
		assert.Equal(t, largeChunks[i].Content, retrievedContent, "Content should match for chunk %d", i)
	}

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestFindOrCreateChunks_MultiRow_EmptySlice tests that FindOrCreateChunks handles empty input correctly.
// This test verifies that the function is a no-op for empty/nil input.
func TestFindOrCreateChunks_MultiRow_EmptySlice(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Test with nil slice
	result, err := repo.FindOrCreateChunks(ctx, nil)
	assert.NoError(t, err, "FindOrCreateChunks should return nil error for nil slice")
	assert.Nil(t, result, "FindOrCreateChunks should return nil for nil slice")

	// Test with empty slice
	result, err = repo.FindOrCreateChunks(ctx, []outbound.CodeChunk{})
	assert.NoError(t, err, "FindOrCreateChunks should return nil error for empty slice")
	assert.Nil(t, result, "FindOrCreateChunks should return nil for empty slice")
}

// TestFindOrCreateChunks_MultiRow_PreservesExistingTokenCounts tests that FindOrCreateChunks preserves existing token counts.
// This test verifies that when chunks already exist with token counts, those counts are NOT overwritten.
func TestFindOrCreateChunks_MultiRow_PreservesExistingTokenCounts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/preserve-tokens-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "preserve-tokens-repo", "Test repository for preserving token counts", "completed")
	require.NoError(t, err)

	// Create and save chunks with token counts
	now := time.Now()
	existingChunks := []outbound.CodeChunk{
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "preserve.go",
			Content:        "func TokenFunc() { }",
			Language:       "go",
			StartLine:      1,
			EndLine:        1,
			Hash:           "hash-preserve-tokens",
			Type:           "function",
			EntityName:     "TokenFunc",
			CreatedAt:      now,
			TokenCount:     150, // Original token count
			TokenCountedAt: &now,
		},
	}

	err = repo.SaveChunks(ctx, existingChunks)
	require.NoError(t, err, "SaveChunks should succeed")

	originalChunkID := existingChunks[0].ID

	// Verify original token count was saved
	var originalTokenCount *int
	err = pool.QueryRow(ctx, `
		SELECT token_count
		FROM codechunking.code_chunks
		WHERE id = $1 AND deleted_at IS NULL
	`, originalChunkID).Scan(&originalTokenCount)
	require.NoError(t, err)
	require.NotNil(t, originalTokenCount)
	assert.Equal(t, 150, *originalTokenCount, "Original token count should be 150")

	// Call FindOrCreateChunks with the same chunk but different token count
	duplicateChunks := []outbound.CodeChunk{
		{
			ID:             uuid.New().String(), // Different ID
			RepositoryID:   repositoryID,
			FilePath:       "preserve.go",
			Content:        "func TokenFunc() { }",
			Language:       "go",
			StartLine:      1,
			EndLine:        1,
			Hash:           "hash-preserve-tokens", // Same hash triggers conflict
			Type:           "function",
			EntityName:     "TokenFunc",
			CreatedAt:      now,
			TokenCount:     999, // Different token count (should be ignored)
			TokenCountedAt: &now,
		},
	}

	resultChunks, err := repo.FindOrCreateChunks(ctx, duplicateChunks)
	require.NoError(t, err, "FindOrCreateChunks should succeed")
	require.NotNil(t, resultChunks, "Result should not be nil")
	require.Len(t, resultChunks, 1, "Should return 1 chunk")

	// Verify the returned ID is the original one
	assert.Equal(t, originalChunkID, resultChunks[0].ID, "Should return existing chunk ID")

	// Verify the token count was preserved (not overwritten)
	var preservedTokenCount *int
	err = pool.QueryRow(ctx, `
		SELECT token_count
		FROM codechunking.code_chunks
		WHERE id = $1 AND deleted_at IS NULL
	`, originalChunkID).Scan(&preservedTokenCount)
	require.NoError(t, err)
	require.NotNil(t, preservedTokenCount)
	assert.Equal(t, 150, *preservedTokenCount, "Token count should be preserved as 150, not overwritten to 999")

	// Verify only one chunk exists (no duplicates)
	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM codechunking.code_chunks
		WHERE repository_id = $1 AND deleted_at IS NULL
	`, repositoryID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Should have exactly 1 chunk (no duplicates)")

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestGetChunkIDsWithEmbeddings_ReturnsOnlyChunksWithEmbeddings tests that the method
// returns only the IDs of chunks that have non-deleted embeddings for the given model.
func TestGetChunkIDsWithEmbeddings_ReturnsOnlyChunksWithEmbeddings(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/get-chunk-ids-embeddings-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "get-chunk-ids-embeddings-repo", "Test repo for GetChunkIDsWithEmbeddings", "completed")
	require.NoError(t, err)

	// Create 3 chunks
	now := time.Now()
	chunkIDs := make([]uuid.UUID, 3)
	for i := range chunkIDs {
		chunkIDs[i] = uuid.New()
	}

	for i, cid := range chunkIDs {
		_, err = pool.Exec(ctx, `
			INSERT INTO codechunking.code_chunks (id, repository_id, file_path, content, language, start_line, end_line, content_hash, chunk_type, entity_name, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, cid, repositoryID, "file.go", "func Chunk"+string(rune('A'+i))+"() {}", "go", i+1, i+1,
			"hash-embed-"+string(rune('A'+i)), "function", "Chunk"+string(rune('A'+i)), now)
		require.NoError(t, err)
	}

	// Insert embeddings for chunks 0 and 1 only
	modelVersion := "gemini-embedding-001"
	for _, i := range []int{0, 1} {
		_, err = pool.Exec(ctx, `
			INSERT INTO codechunking.embeddings_partitioned (chunk_id, repository_id, embedding, model_version)
			VALUES ($1, $2, array_fill(0.0, ARRAY[768])::codechunking.vector, $3)
		`, chunkIDs[i], repositoryID, modelVersion)
		require.NoError(t, err)
	}

	// Call GetChunkIDsWithEmbeddings with all 3 IDs
	result, err := repo.GetChunkIDsWithEmbeddings(ctx, repositoryID, chunkIDs, modelVersion)
	require.NoError(t, err)

	// Assert: only chunks 0 and 1 are returned
	assert.Len(t, result, 2, "Should return exactly 2 chunk IDs with embeddings")
	assert.Contains(t, result, chunkIDs[0], "chunk 0 should be in result")
	assert.Contains(t, result, chunkIDs[1], "chunk 1 should be in result")
	assert.NotContains(t, result, chunkIDs[2], "chunk 2 should NOT be in result (no embedding)")

	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.embeddings_partitioned WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestGetChunkIDsWithEmbeddings_EmptySliceInput_ReturnsEmptyMap tests that the method
// returns an empty map without error when given an empty slice of IDs.
func TestGetChunkIDsWithEmbeddings_EmptySliceInput_ReturnsEmptyMap(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	repositoryID := uuid.New()

	// Call with empty slice - should return empty map and nil error
	result, err := repo.GetChunkIDsWithEmbeddings(ctx, repositoryID, []uuid.UUID{}, "gemini-embedding-001")
	require.NoError(t, err)
	assert.Empty(t, result, "Should return empty map for empty input")
}

// TestGetChunkIDsWithEmbeddings_NoneHaveEmbeddings_ReturnsEmptyMap tests that the method
// returns an empty map when none of the queried chunks have embeddings.
func TestGetChunkIDsWithEmbeddings_NoneHaveEmbeddings_ReturnsEmptyMap(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/no-embeddings-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "no-embeddings-repo", "Test repo for no embeddings", "completed")
	require.NoError(t, err)

	// Create 3 chunks with no embeddings
	now := time.Now()
	chunkIDs := make([]uuid.UUID, 3)
	for i := range chunkIDs {
		chunkIDs[i] = uuid.New()
		_, err = pool.Exec(ctx, `
			INSERT INTO codechunking.code_chunks (id, repository_id, file_path, content, language, start_line, end_line, content_hash, chunk_type, entity_name, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, chunkIDs[i], repositoryID, "noembeds.go", "func NoEmbed"+string(rune('A'+i))+"() {}", "go", i+1, i+1,
			"hash-noembed-"+string(rune('A'+i)), "function", "NoEmbed"+string(rune('A'+i)), now)
		require.NoError(t, err)
	}

	// Query - no embeddings inserted, so result should be empty
	result, err := repo.GetChunkIDsWithEmbeddings(ctx, repositoryID, chunkIDs, "gemini-embedding-001")
	require.NoError(t, err)
	assert.Empty(t, result, "Should return empty map when no chunks have embeddings")

	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestGetChunkIDsWithEmbeddings_AllHaveEmbeddings_ReturnsAllIDs tests that the method
// returns all IDs when all queried chunks have embeddings.
func TestGetChunkIDsWithEmbeddings_AllHaveEmbeddings_ReturnsAllIDs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/all-embeddings-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "all-embeddings-repo", "Test repo for all embeddings", "completed")
	require.NoError(t, err)

	// Create 3 chunks and embed all of them
	now := time.Now()
	modelVersion := "gemini-embedding-001"
	chunkIDs := make([]uuid.UUID, 3)
	for i := range chunkIDs {
		chunkIDs[i] = uuid.New()
		_, err = pool.Exec(ctx, `
			INSERT INTO codechunking.code_chunks (id, repository_id, file_path, content, language, start_line, end_line, content_hash, chunk_type, entity_name, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, chunkIDs[i], repositoryID, "allembed.go", "func AllEmbed"+string(rune('A'+i))+"() {}", "go", i+1, i+1,
			"hash-allembed-"+string(rune('A'+i)), "function", "AllEmbed"+string(rune('A'+i)), now)
		require.NoError(t, err)

		_, err = pool.Exec(ctx, `
			INSERT INTO codechunking.embeddings_partitioned (chunk_id, repository_id, embedding, model_version)
			VALUES ($1, $2, array_fill(0.0, ARRAY[768])::codechunking.vector, $3)
		`, chunkIDs[i], repositoryID, modelVersion)
		require.NoError(t, err)
	}

	// All 3 should be returned
	result, err := repo.GetChunkIDsWithEmbeddings(ctx, repositoryID, chunkIDs, modelVersion)
	require.NoError(t, err)
	assert.Len(t, result, 3, "Should return all 3 chunk IDs")
	for _, cid := range chunkIDs {
		assert.Contains(t, result, cid, "chunk %s should be in result", cid)
	}

	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.embeddings_partitioned WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestGetChunkIDsWithEmbeddings_ModelVersionFilter_OnlyMatchingModelReturned tests that
// the method filters by model version, only returning chunks with the queried model's embeddings.
func TestGetChunkIDsWithEmbeddings_ModelVersionFilter_OnlyMatchingModelReturned(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/model-filter-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "model-filter-repo", "Test repo for model filter", "completed")
	require.NoError(t, err)

	// Create 2 chunks
	now := time.Now()
	chunkA := uuid.New()
	chunkB := uuid.New()

	_, err = pool.Exec(ctx, `
		INSERT INTO codechunking.code_chunks (id, repository_id, file_path, content, language, start_line, end_line, content_hash, chunk_type, entity_name, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, chunkA, repositoryID, "model_filter.go", "func ChunkA() {}", "go", 1, 1, "hash-model-A", "function", "ChunkA", now)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO codechunking.code_chunks (id, repository_id, file_path, content, language, start_line, end_line, content_hash, chunk_type, entity_name, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, chunkB, repositoryID, "model_filter.go", "func ChunkB() {}", "go", 5, 5, "hash-model-B", "function", "ChunkB", now)
	require.NoError(t, err)

	// Chunk A has embedding with model "gemini-embedding-001"
	_, err = pool.Exec(ctx, `
		INSERT INTO codechunking.embeddings_partitioned (chunk_id, repository_id, embedding, model_version)
		VALUES ($1, $2, array_fill(0.0, ARRAY[768])::codechunking.vector, $3)
	`, chunkA, repositoryID, "gemini-embedding-001")
	require.NoError(t, err)

	// Chunk B has embedding with a different model "text-embedding-004"
	_, err = pool.Exec(ctx, `
		INSERT INTO codechunking.embeddings_partitioned (chunk_id, repository_id, embedding, model_version)
		VALUES ($1, $2, array_fill(0.0, ARRAY[768])::codechunking.vector, $3)
	`, chunkB, repositoryID, "text-embedding-004")
	require.NoError(t, err)

	// Query for "gemini-embedding-001" only
	result, err := repo.GetChunkIDsWithEmbeddings(ctx, repositoryID, []uuid.UUID{chunkA, chunkB}, "gemini-embedding-001")
	require.NoError(t, err)

	// Only chunk A should be returned
	assert.Len(t, result, 1, "Should return only 1 chunk matching the model")
	assert.Contains(t, result, chunkA, "chunkA should be in result (has matching model)")
	assert.NotContains(t, result, chunkB, "chunkB should NOT be in result (different model)")

	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.embeddings_partitioned WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestGetChunkIDsWithEmbeddings_RespectsDeletedAt_ExcludesDeleted tests that the method
// excludes chunks whose embeddings have been soft-deleted (deleted_at IS NOT NULL).
func TestGetChunkIDsWithEmbeddings_RespectsDeletedAt_ExcludesDeleted(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/deleted-at-filter-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "deleted-at-filter-repo", "Test repo for deleted_at filter", "completed")
	require.NoError(t, err)

	// Create 2 chunks
	now := time.Now()
	chunkA := uuid.New()
	chunkB := uuid.New()
	modelVersion := "gemini-embedding-001"

	_, err = pool.Exec(ctx, `
		INSERT INTO codechunking.code_chunks (id, repository_id, file_path, content, language, start_line, end_line, content_hash, chunk_type, entity_name, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, chunkA, repositoryID, "deleted_at.go", "func ChunkA() {}", "go", 1, 1, "hash-delat-A", "function", "ChunkA", now)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO codechunking.code_chunks (id, repository_id, file_path, content, language, start_line, end_line, content_hash, chunk_type, entity_name, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, chunkB, repositoryID, "deleted_at.go", "func ChunkB() {}", "go", 5, 5, "hash-delat-B", "function", "ChunkB", now)
	require.NoError(t, err)

	// Chunk A has a live (non-deleted) embedding
	_, err = pool.Exec(ctx, `
		INSERT INTO codechunking.embeddings_partitioned (chunk_id, repository_id, embedding, model_version)
		VALUES ($1, $2, array_fill(0.0, ARRAY[768])::codechunking.vector, $3)
	`, chunkA, repositoryID, modelVersion)
	require.NoError(t, err)

	// Chunk B has a soft-deleted embedding
	_, err = pool.Exec(ctx, `
		INSERT INTO codechunking.embeddings_partitioned (chunk_id, repository_id, embedding, model_version, deleted_at)
		VALUES ($1, $2, array_fill(0.0, ARRAY[768])::codechunking.vector, $3, CURRENT_TIMESTAMP)
	`, chunkB, repositoryID, modelVersion)
	require.NoError(t, err)

	// Only chunk A should be returned (chunk B's embedding is soft-deleted)
	result, err := repo.GetChunkIDsWithEmbeddings(ctx, repositoryID, []uuid.UUID{chunkA, chunkB}, modelVersion)
	require.NoError(t, err)

	assert.Len(t, result, 1, "Should return only 1 chunk (not the soft-deleted one)")
	assert.Contains(t, result, chunkA, "chunkA should be in result (live embedding)")
	assert.NotContains(t, result, chunkB, "chunkB should NOT be in result (embedding is soft-deleted)")

	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.embeddings_partitioned WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}
