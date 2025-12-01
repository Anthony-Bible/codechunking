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

// TestSaveChunks_MultiRow_SingleChunk tests that SaveChunks correctly handles a single chunk.
// This test verifies that the multi-row INSERT implementation works correctly for the edge case of a single chunk.
func TestSaveChunks_MultiRow_SingleChunk(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/single-chunk-multirow-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "single-chunk-test-repo", "Test repository for single chunk multi-row INSERT", "completed")
	require.NoError(t, err)

	// Create a single chunk
	now := time.Now()
	chunks := []outbound.CodeChunk{
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "single_chunk.go",
			Content:        "func SingleChunk() { }",
			Language:       "go",
			StartLine:      1,
			EndLine:        1,
			Hash:           "hash-single-chunk-1",
			Type:           "function",
			EntityName:     "SingleChunk",
			CreatedAt:      now,
			TokenCount:     10,
			TokenCountedAt: &now,
		},
	}

	// Save the single chunk using SaveChunks (should use multi-row INSERT for single chunk too)
	err = repo.SaveChunks(ctx, chunks)
	require.NoError(t, err, "SaveChunks should work correctly for a single chunk")

	// Verify the chunk was saved correctly
	chunkID, err := uuid.Parse(chunks[0].ID)
	require.NoError(t, err)

	var retrievedContent string
	var retrievedLanguage string
	var tokenCount *int
	err = pool.QueryRow(ctx, `
		SELECT content, language, token_count
		FROM codechunking.code_chunks
		WHERE id = $1 AND deleted_at IS NULL
	`, chunkID).Scan(&retrievedContent, &retrievedLanguage, &tokenCount)
	require.NoError(t, err, "Should retrieve the saved chunk")

	assert.Equal(t, chunks[0].Content, retrievedContent, "Content should match")
	assert.Equal(t, chunks[0].Language, retrievedLanguage, "Language should match")
	require.NotNil(t, tokenCount, "Token count should be saved")
	assert.Equal(t, 10, *tokenCount, "Token count should match")

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestSaveChunks_MultiRow_MultipleChunks tests that SaveChunks correctly saves multiple chunks in a single operation.
// This test verifies that the multi-row INSERT implementation batches multiple chunks together.
func TestSaveChunks_MultiRow_MultipleChunks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/multiple-chunks-multirow-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "multiple-chunks-test-repo", "Test repository for multiple chunks multi-row INSERT", "completed")
	require.NoError(t, err)

	// Create 5 chunks
	now := time.Now()
	chunks := make([]outbound.CodeChunk, 5)
	for i := range chunks {
		chunks[i] = outbound.CodeChunk{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "multiple_chunks.go",
			Content:        "func TestFunc" + string(rune('A'+i)) + "() { }",
			Language:       "go",
			StartLine:      i*2 + 1,
			EndLine:        i*2 + 1,
			Hash:           "hash-multiple-" + uuid.New().String()[:8],
			Type:           "function",
			EntityName:     "TestFunc" + string(rune('A'+i)),
			CreatedAt:      now,
			TokenCount:     (i + 1) * 5, // 5, 10, 15, 20, 25
			TokenCountedAt: &now,
		}
	}

	// Save all chunks in a single operation
	err = repo.SaveChunks(ctx, chunks)
	require.NoError(t, err, "SaveChunks should work correctly for multiple chunks")

	// Verify all chunks were saved correctly
	for i, chunk := range chunks {
		chunkID, err := uuid.Parse(chunk.ID)
		require.NoError(t, err)

		var retrievedContent string
		var tokenCount *int
		err = pool.QueryRow(ctx, `
			SELECT content, token_count
			FROM codechunking.code_chunks
			WHERE id = $1 AND deleted_at IS NULL
		`, chunkID).Scan(&retrievedContent, &tokenCount)
		require.NoError(t, err, "Should retrieve chunk %d", i)

		assert.Equal(t, chunk.Content, retrievedContent, "Content should match for chunk %d", i)
		require.NotNil(t, tokenCount, "Token count should be saved for chunk %d", i)
		assert.Equal(t, (i+1)*5, *tokenCount, "Token count should match for chunk %d", i)
	}

	// Verify correct number of chunks were saved
	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM codechunking.code_chunks
		WHERE repository_id = $1 AND deleted_at IS NULL
	`, repositoryID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 5, count, "Should have exactly 5 chunks")

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestSaveChunks_MultiRow_EmptySlice tests that SaveChunks handles an empty slice correctly.
// This test verifies that SaveChunks is a no-op for empty input and returns nil error.
func TestSaveChunks_MultiRow_EmptySlice(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Test with nil slice
	err := repo.SaveChunks(ctx, nil)
	assert.NoError(t, err, "SaveChunks should return nil for nil slice")

	// Test with empty slice
	err = repo.SaveChunks(ctx, []outbound.CodeChunk{})
	assert.NoError(t, err, "SaveChunks should return nil for empty slice")
}

// TestSaveChunks_MultiRow_LargeBatch tests that SaveChunks handles a large batch of chunks.
// This test verifies that the multi-row INSERT implementation can handle typical token counting batch sizes (50+ chunks).
func TestSaveChunks_MultiRow_LargeBatch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/large-batch-multirow-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "large-batch-test-repo", "Test repository for large batch multi-row INSERT", "completed")
	require.NoError(t, err)

	// Create 50 chunks (typical batch size for token counting)
	now := time.Now()
	chunks := make([]outbound.CodeChunk, 50)
	for i := range chunks {
		chunks[i] = outbound.CodeChunk{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "large_batch.go",
			Content:        "func LargeFunc" + string(rune('0'+i%10)) + "() { }",
			Language:       "go",
			StartLine:      i*2 + 1,
			EndLine:        i*2 + 1,
			Hash:           "hash-large-" + uuid.New().String()[:8],
			Type:           "function",
			EntityName:     "LargeFunc" + string(rune('0'+i%10)),
			CreatedAt:      now,
			TokenCount:     (i + 1) * 2,
			TokenCountedAt: &now,
		}
	}

	// Save all chunks in a single operation
	startTime := time.Now()
	err = repo.SaveChunks(ctx, chunks)
	duration := time.Since(startTime)
	require.NoError(t, err, "SaveChunks should work correctly for large batch")

	t.Logf("SaveChunks for 50 chunks took: %v", duration)

	// Verify all chunks were saved correctly
	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM codechunking.code_chunks
		WHERE repository_id = $1 AND deleted_at IS NULL
	`, repositoryID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 50, count, "Should have exactly 50 chunks")

	// Spot check a few chunks to verify data integrity
	testIndices := []int{0, 24, 49} // First, middle, last
	for _, i := range testIndices {
		chunkID, err := uuid.Parse(chunks[i].ID)
		require.NoError(t, err)

		var tokenCount *int
		err = pool.QueryRow(ctx, `
			SELECT token_count
			FROM codechunking.code_chunks
			WHERE id = $1 AND deleted_at IS NULL
		`, chunkID).Scan(&tokenCount)
		require.NoError(t, err, "Should retrieve chunk %d", i)

		require.NotNil(t, tokenCount, "Token count should be saved for chunk %d", i)
		assert.Equal(t, (i+1)*2, *tokenCount, "Token count should match for chunk %d", i)
	}

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestSaveChunks_MultiRow_PreservesChunkData tests that all chunk fields are correctly saved.
// This test verifies that the multi-row INSERT implementation preserves all chunk field values.
func TestSaveChunks_MultiRow_PreservesChunkData(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/preserve-data-multirow-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "preserve-data-test-repo", "Test repository for data preservation multi-row INSERT", "completed")
	require.NoError(t, err)

	// Create chunks with comprehensive field values
	now := time.Now()
	chunks := []outbound.CodeChunk{
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "preserve_data.go",
			Content:        "func CompleteExample() {\n\t// Full function\n\treturn\n}",
			Language:       "go",
			StartLine:      10,
			EndLine:        13,
			Hash:           "hash-preserve-complete",
			Type:           "function",
			EntityName:     "CompleteExample",
			ParentEntity:   "PackageMain",
			QualifiedName:  "main.CompleteExample",
			Signature:      "func CompleteExample()",
			Visibility:     "public",
			CreatedAt:      now,
			TokenCount:     25,
			TokenCountedAt: &now,
		},
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "preserve_data.go",
			Content:        "type DataStruct struct {\n\tField string\n}",
			Language:       "go",
			StartLine:      20,
			EndLine:        22,
			Hash:           "hash-preserve-struct",
			Type:           "struct",
			EntityName:     "DataStruct",
			ParentEntity:   "PackageMain",
			QualifiedName:  "main.DataStruct",
			Signature:      "type DataStruct struct",
			Visibility:     "public",
			CreatedAt:      now,
			TokenCount:     15,
			TokenCountedAt: &now,
		},
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "preserve_data.go",
			Content:        "func (d *DataStruct) privateMethod() { }",
			Language:       "go",
			StartLine:      30,
			EndLine:        30,
			Hash:           "hash-preserve-private",
			Type:           "method",
			EntityName:     "privateMethod",
			ParentEntity:   "DataStruct",
			QualifiedName:  "main.DataStruct.privateMethod",
			Signature:      "func (d *DataStruct) privateMethod()",
			Visibility:     "private",
			CreatedAt:      now,
			TokenCount:     12,
			TokenCountedAt: &now,
		},
	}

	// Save all chunks
	err = repo.SaveChunks(ctx, chunks)
	require.NoError(t, err, "SaveChunks should preserve all chunk data")

	// Verify all fields for each chunk
	for i, chunk := range chunks {
		chunkID, err := uuid.Parse(chunk.ID)
		require.NoError(t, err)

		var (
			retrievedContent       string
			retrievedLanguage      string
			retrievedFilePath      string
			retrievedType          string
			retrievedEntityName    string
			retrievedParentEntity  string
			retrievedQualifiedName string
			retrievedSignature     string
			retrievedVisibility    string
			retrievedStartLine     int
			retrievedEndLine       int
			retrievedHash          string
			tokenCount             *int
			tokenCountedAt         *time.Time
		)

		err = pool.QueryRow(ctx, `
			SELECT content, language, file_path, chunk_type, entity_name, parent_entity,
				   qualified_name, signature, visibility, start_line, end_line, content_hash,
				   token_count, token_counted_at
			FROM codechunking.code_chunks
			WHERE id = $1 AND deleted_at IS NULL
		`, chunkID).Scan(
			&retrievedContent, &retrievedLanguage, &retrievedFilePath, &retrievedType,
			&retrievedEntityName, &retrievedParentEntity, &retrievedQualifiedName,
			&retrievedSignature, &retrievedVisibility, &retrievedStartLine, &retrievedEndLine,
			&retrievedHash, &tokenCount, &tokenCountedAt,
		)
		require.NoError(t, err, "Should retrieve chunk %d", i)

		// Verify all fields
		assert.Equal(t, chunk.Content, retrievedContent, "Content should match for chunk %d", i)
		assert.Equal(t, chunk.Language, retrievedLanguage, "Language should match for chunk %d", i)
		assert.Equal(t, chunk.FilePath, retrievedFilePath, "FilePath should match for chunk %d", i)
		assert.Equal(t, chunk.Type, retrievedType, "Type should match for chunk %d", i)
		assert.Equal(t, chunk.EntityName, retrievedEntityName, "EntityName should match for chunk %d", i)
		assert.Equal(t, chunk.ParentEntity, retrievedParentEntity, "ParentEntity should match for chunk %d", i)
		assert.Equal(t, chunk.QualifiedName, retrievedQualifiedName, "QualifiedName should match for chunk %d", i)
		assert.Equal(t, chunk.Signature, retrievedSignature, "Signature should match for chunk %d", i)
		assert.Equal(t, chunk.Visibility, retrievedVisibility, "Visibility should match for chunk %d", i)
		assert.Equal(t, chunk.StartLine, retrievedStartLine, "StartLine should match for chunk %d", i)
		assert.Equal(t, chunk.EndLine, retrievedEndLine, "EndLine should match for chunk %d", i)
		assert.Equal(t, chunk.Hash, retrievedHash, "Hash should match for chunk %d", i)

		// Verify token count fields
		require.NotNil(t, tokenCount, "Token count should be saved for chunk %d", i)
		assert.Equal(t, chunk.TokenCount, *tokenCount, "Token count should match for chunk %d", i)
		require.NotNil(t, tokenCountedAt, "Token counted at should be saved for chunk %d", i)
		assert.WithinDuration(
			t,
			*chunk.TokenCountedAt,
			*tokenCountedAt,
			time.Second,
			"Token counted at should match for chunk %d",
			i,
		)
	}

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestSaveChunks_MultiRow_ConflictHandling tests that SaveChunks correctly handles conflicts.
// This test verifies that the multi-row INSERT implementation properly uses ON CONFLICT to handle duplicate chunks.
func TestSaveChunks_MultiRow_ConflictHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/conflict-multirow-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "conflict-test-repo", "Test repository for conflict handling multi-row INSERT", "completed")
	require.NoError(t, err)

	// Create initial chunks with token counts
	now := time.Now()
	initialChunks := []outbound.CodeChunk{
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "conflict.go",
			Content:        "func ConflictTest() { }",
			Language:       "go",
			StartLine:      1,
			EndLine:        1,
			Hash:           "hash-conflict-1",
			Type:           "function",
			EntityName:     "ConflictTest",
			CreatedAt:      now,
			TokenCount:     100,
			TokenCountedAt: &now,
		},
	}

	// Save initial chunks
	err = repo.SaveChunks(ctx, initialChunks)
	require.NoError(t, err, "Initial save should succeed")

	// Save the same chunk again with different token count (should preserve existing token count per COALESCE logic)
	duplicateChunks := []outbound.CodeChunk{
		{
			ID:             uuid.New().String(), // Different ID
			RepositoryID:   repositoryID,
			FilePath:       "conflict.go",
			Content:        "func ConflictTest() { }",
			Language:       "go",
			StartLine:      1,
			EndLine:        1,
			Hash:           "hash-conflict-1", // Same hash triggers conflict
			Type:           "function",
			EntityName:     "ConflictTest",
			CreatedAt:      now,
			TokenCount:     999, // Different token count (should be ignored)
			TokenCountedAt: &now,
		},
	}

	// Save duplicate chunks
	err = repo.SaveChunks(ctx, duplicateChunks)
	require.NoError(t, err, "Duplicate save should succeed (ON CONFLICT handling)")

	// Verify the original token count is preserved
	var tokenCount *int
	err = pool.QueryRow(ctx, `
		SELECT token_count
		FROM codechunking.code_chunks
		WHERE repository_id = $1 AND file_path = $2 AND content_hash = $3 AND deleted_at IS NULL
	`, repositoryID, "conflict.go", "hash-conflict-1").Scan(&tokenCount)
	require.NoError(t, err)

	require.NotNil(t, tokenCount, "Token count should be preserved")
	assert.Equal(t, 100, *tokenCount, "Original token count should be preserved, not overwritten")

	// Verify only one chunk exists (not duplicated)
	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM codechunking.code_chunks
		WHERE repository_id = $1 AND deleted_at IS NULL
	`, repositoryID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Should have exactly 1 chunk (not duplicated)")

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}

// TestSaveChunks_MultiRow_MixedTokenCounts tests SaveChunks with chunks having varied token count states.
// This test verifies that the multi-row INSERT handles chunks with and without token counts correctly.
func TestSaveChunks_MultiRow_MixedTokenCounts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	repo := NewPostgreSQLChunkRepository(pool)
	ctx := context.Background()

	// Create test repository
	repositoryID := uuid.New()
	testURL := "https://github.com/test/mixed-tokens-multirow-" + repositoryID.String()[:8]

	_, err := pool.Exec(ctx, `
		INSERT INTO codechunking.repositories (id, url, normalized_url, name, description, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repositoryID, testURL, testURL, "mixed-tokens-test-repo", "Test repository for mixed token counts", "completed")
	require.NoError(t, err)

	// Create chunks with mixed token count states
	now := time.Now()
	chunks := []outbound.CodeChunk{
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "mixed.go",
			Content:        "func WithTokens() { }",
			Language:       "go",
			StartLine:      1,
			EndLine:        1,
			Hash:           "hash-with-tokens",
			Type:           "function",
			EntityName:     "WithTokens",
			CreatedAt:      now,
			TokenCount:     50,
			TokenCountedAt: &now,
		},
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "mixed.go",
			Content:        "func WithoutTokens() { }",
			Language:       "go",
			StartLine:      5,
			EndLine:        5,
			Hash:           "hash-without-tokens",
			Type:           "function",
			EntityName:     "WithoutTokens",
			CreatedAt:      now,
			TokenCount:     0,
			TokenCountedAt: nil,
		},
		{
			ID:             uuid.New().String(),
			RepositoryID:   repositoryID,
			FilePath:       "mixed.go",
			Content:        "func ZeroTokens() { }",
			Language:       "go",
			StartLine:      10,
			EndLine:        10,
			Hash:           "hash-zero-tokens",
			Type:           "function",
			EntityName:     "ZeroTokens",
			CreatedAt:      now,
			TokenCount:     0,
			TokenCountedAt: &now, // Zero tokens but has timestamp
		},
	}

	// Save all chunks
	err = repo.SaveChunks(ctx, chunks)
	require.NoError(t, err, "SaveChunks should handle mixed token count states")

	// Verify each chunk's token count state
	for i, chunk := range chunks {
		chunkID, err := uuid.Parse(chunk.ID)
		require.NoError(t, err)

		var tokenCount *int
		var tokenCountedAt *time.Time
		err = pool.QueryRow(ctx, `
			SELECT token_count, token_counted_at
			FROM codechunking.code_chunks
			WHERE id = $1 AND deleted_at IS NULL
		`, chunkID).Scan(&tokenCount, &tokenCountedAt)
		require.NoError(t, err, "Should retrieve chunk %d", i)

		// Verify based on original chunk state
		if i == 0 {
			// Chunk with token count
			require.NotNil(t, tokenCount, "Chunk 0 should have token count")
			assert.Equal(t, 50, *tokenCount)
			require.NotNil(t, tokenCountedAt, "Chunk 0 should have timestamp")
		} else if i == 1 {
			// Chunk without token count
			assert.Nil(t, tokenCountedAt, "Chunk 1 should not have timestamp")
		} else if i == 2 {
			// Chunk with zero tokens but timestamp
			require.NotNil(t, tokenCountedAt, "Chunk 2 should have timestamp")
		}
	}

	// Cleanup
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM codechunking.code_chunks WHERE repository_id = $1", repositoryID)
		pool.Exec(ctx, "DELETE FROM codechunking.repositories WHERE id = $1", repositoryID)
	})
}
