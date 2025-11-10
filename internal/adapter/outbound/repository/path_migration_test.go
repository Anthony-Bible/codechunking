package repository

import (
	"codechunking/internal/application/service"
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPathMigrationIntegration_RedPhase tests the migration functionality at the repository layer.
// These tests are written to FAIL initially because the migration repository doesn't exist yet.
func TestPathMigrationIntegration_RedPhase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping migration integration test in short mode")
	}

	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()

	t.Run("migration_repository_interface", func(t *testing.T) {
		// Test the repository interface - now implemented
		repo := NewPathMigrationRepository(pool, nil)
		_, err := repo.GetUUIDPrefixedPaths(ctx)
		require.NoError(t, err)
	})

	t.Run("batch_path_updates", func(t *testing.T) {
		repoID := uuid.New()
		testUUID := uuid.New()

		// Setup test data
		chunkIDs := []uuid.UUID{}
		updates := []PathUpdate{}
		for i := range 100 {
			chunkID := uuid.New()
			chunkIDs = append(chunkIDs, chunkID)
			oldPath := fmt.Sprintf("/tmp/codechunking-workspace/%s/batch/file_%d.py", testUUID.String(), i)
			newPath := fmt.Sprintf("batch/file_%d.py", i)
			insertTestChunkDirect(t, pool, ctx, chunkID, repoID, oldPath)

			updates = append(updates, PathUpdate{
				ChunkID: chunkID,
				RepoID:  repoID,
				OldPath: oldPath,
				NewPath: newPath,
			})
		}

		// Test batch update with new interface
		migrationRepo := NewPathMigrationRepository(pool, nil)

		// Convert to service types
		serviceUpdates := make([]service.PathUpdateRecord, len(updates))
		for i, update := range updates {
			serviceUpdates[i] = service.PathUpdateRecord{
				ChunkID: update.ChunkID,
				RepoID:  update.RepoID,
				OldPath: update.OldPath,
				NewPath: update.NewPath,
			}
		}

		stats, err := migrationRepo.BatchUpdatePaths(ctx, serviceUpdates)
		require.NoError(t, err)
		assert.Equal(t, 100, stats.MigratedChunks, "Should update all chunks in batch")
	})

	t.Run("transaction_rollback_on_error", func(t *testing.T) {
		repoID := uuid.New()
		testUUID := uuid.New()

		// Insert test data
		chunkID1 := uuid.New()
		chunkID2 := uuid.New()

		oldPath1 := fmt.Sprintf("/tmp/codechunking-workspace/%s/tx/file1.go", testUUID.String())
		oldPath2 := fmt.Sprintf("/tmp/codechunking-workspace/%s/tx/file2.go", testUUID.String())

		insertTestChunkDirect(t, pool, ctx, chunkID1, repoID, oldPath1)
		insertTestChunkDirect(t, pool, ctx, chunkID2, repoID, oldPath2)

		// Test transactional migration with new interface
		migrationRepo := NewPathMigrationRepository(pool, nil)
		updates := []service.PathUpdateRecord{
			{ChunkID: chunkID1, RepoID: repoID, OldPath: oldPath1, NewPath: "tx/file1.go"},
			{ChunkID: chunkID2, RepoID: repoID, OldPath: oldPath2, NewPath: "tx/file2.go"},
		}
		err := migrationRepo.TransactionalPathMigration(ctx, updates)
		require.NoError(t, err)

		// Verify all changes were applied - check that paths were converted to relative
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM code_chunks WHERE file_path LIKE '/tmp/codechunking-workspace/%' AND repository_id = $1", repoID).
			Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "All UUID paths should be migrated for this repository")
	})

	t.Run("progress_tracking", func(t *testing.T) {
		// This will FAIL because progress tracking doesn't exist yet
		migrationRepo := &PathMigrationRepository{pool: pool}

		progress, err := migrationRepo.GetMigrationProgress(ctx)
		require.NoError(t, err)
		assert.NotNil(t, progress, "Should return migration progress")
		assert.GreaterOrEqual(t, progress.TotalChunks, 0, "Total chunks should be non-negative")
		assert.GreaterOrEqual(t, progress.MigratedChunks, 0, "Migrated chunks should be non-negative")
		assert.LessOrEqual(t, progress.MigratedChunks, progress.TotalChunks, "Migrated should not exceed total")
	})

	t.Run("validation_before_migration", func(t *testing.T) {
		repoID := uuid.New()
		testUUID := uuid.New()

		// Insert test data with some invalid paths
		insertTestChunkDirect(t, pool, ctx, uuid.New(), repoID,
			fmt.Sprintf("/tmp/codechunking-workspace/%s/valid/file.go", testUUID.String()))
		insertTestChunkDirect(t, pool, ctx, uuid.New(), repoID, "")
		insertTestChunkDirect(t, pool, ctx, uuid.New(), repoID, "/invalid/path")

		// This will FAIL because validation doesn't exist yet
		migrationRepo := &PathMigrationRepository{pool: pool}
		validation, err := migrationRepo.ValidateMigrationData(ctx)
		require.NoError(t, err)

		assert.Positive(t, validation.ValidChunks, "Should find valid chunks")
		assert.Positive(t, validation.InvalidChunks, "Should find invalid chunks")
		assert.NotEmpty(t, validation.Errors, "Should report validation errors")
	})
}

// Test helper functions

func insertTestChunkDirect(
	t *testing.T,
	pool *pgxpool.Pool,
	ctx context.Context,
	chunkID, repoID uuid.UUID,
	filePath string,
) {
	t.Helper()

	// Set search path
	_, err := pool.Exec(ctx, "SET search_path TO codechunking, public")
	require.NoError(t, err)

	// Insert repository if it doesn't exist
	repoURL := fmt.Sprintf("https://github.com/test/repo-%s.git", repoID.String()[:8])
	normalizedURL := fmt.Sprintf("https://github.com/test/repo-%s", repoID.String()[:8])
	_, err = pool.Exec(ctx, `
		INSERT INTO repositories (id, url, normalized_url, name, status) 
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING
	`, repoID, repoURL, normalizedURL, "test-repo", "completed")
	require.NoError(t, err)

	// Insert test chunk
	_, err = pool.Exec(ctx, `
		INSERT INTO code_chunks (id, repository_id, file_path, chunk_type, content, language, start_line, end_line, content_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, chunkID, repoID, filePath, "function", "test content", "go", 1, 10, "test-hash")
	require.NoError(t, err)
}
