package service

import (
	"codechunking/internal/config"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPathMigration_RedPhase tests the data migration functionality for converting absolute paths to relative paths.
// These tests are written to FAIL initially because the migration functionality doesn't exist yet.
func TestPathMigration_RedPhase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping migration test in short mode")
	}

	pool := getMigrationTestPool(t)
	defer pool.Close()

	ctx := context.Background()

	// Clean up any existing test data
	cleanupMigrationTestData(t, pool, ctx)

	t.Run("detect_uuid_prefixed_paths", func(t *testing.T) {
		// This test will FAIL because the migration detection logic doesn't exist yet
		testUUID := uuid.New()
		repoID := uuid.New()

		// Insert test data with UUID-prefixed paths
		testPath := fmt.Sprintf("/tmp/codechunking-workspace/%s/app/src/main.go", testUUID.String())
		insertTestChunk(t, pool, ctx, repoID, testPath)

		// This function doesn't exist yet - test will fail
		detected, err := detectUUIDPaths(ctx, pool)
		require.NoError(t, err)
		assert.Positive(t, detected, "Should detect UUID-prefixed paths")
	})

	t.Run("convert_absolute_to_relative_paths", func(t *testing.T) {
		testUUID := uuid.New()
		_ = testUUID // Mark as used to avoid lint error
		absolutePath := fmt.Sprintf("/tmp/codechunking-workspace/%s/components/Button.tsx", testUUID.String())
		expectedRelative := "components/Button.tsx"

		// This function doesn't exist yet - test will fail
		relativePath, err := convertToRelativePath(absolutePath)
		require.NoError(t, err)
		assert.Equal(t, expectedRelative, relativePath, "Should convert absolute path to relative path")
	})

	t.Run("migration_with_various_uuid_formats", func(t *testing.T) {
		_ = uuid.New() // Mark as used to avoid lint error
		testCases := []struct {
			name     string
			input    string
			expected string
		}{
			{
				name:     "standard_uuid_with_workspace",
				input:    "/tmp/codechunking-workspace/550e8400-e29b-41d4-a716-446655440000/src/utils/helpers.ts",
				expected: "src/utils/helpers.ts",
			},
			{
				name:     "uuid_with_different_workspace_prefix",
				input:    "/var/tmp/codechunking-12345/6ba7b810-9dad-11d1-80b4-00c04fd430c8/lib/api.js",
				expected: "lib/api.js",
			},
			{
				name:     "nested_structure_with_uuid",
				input:    "/tmp/codechunking-workspace/f47ac10b-58cc-4372-a567-0e02b2c3d479/packages/core/src/index.ts",
				expected: "packages/core/src/index.ts",
			},
			{
				name:     "already_relative_path_unchanged",
				input:    "src/components/Header.jsx",
				expected: "src/components/Header.jsx",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// This function doesn't exist yet - test will fail
				result, err := convertToRelativePath(tc.input)
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("database_migration_execution", func(t *testing.T) {
		// Setup test data
		repoID := uuid.New()
		testUUID := uuid.New()

		// Insert multiple chunks with UUID paths
		testPaths := []string{
			fmt.Sprintf("/tmp/codechunking-workspace/%s/package.json", testUUID.String()),
			fmt.Sprintf("/tmp/codechunking-workspace/%s/src/index.js", testUUID.String()),
			fmt.Sprintf("/tmp/codechunking-workspace/%s/tests/unit.test.js", testUUID.String()),
		}

		for _, path := range testPaths {
			insertTestChunk(t, pool, ctx, repoID, path)
		}

		// This function doesn't exist yet - test will fail
		migratedCount, err := executePathMigrationForRepo(ctx, pool, repoID)
		require.NoError(t, err)
		assert.Equal(t, len(testPaths), migratedCount, "Should migrate all UUID-prefixed paths for this repository")

		// Verify migration results
		for _, originalPath := range testPaths {
			var newPath string
			err := pool.QueryRow(ctx,
				"SELECT file_path FROM code_chunks WHERE repository_id = $1 AND file_path = $2",
				repoID, originalPath).Scan(&newPath)
			assert.Error(t, err, "Original UUID path should not exist after migration")
			// Check for "no rows in result set" error (could be sql.ErrNoRows or pgx variant)
			if err != nil {
				assert.Contains(t, err.Error(), "no rows in result set")
			}
		}
	})

	t.Run("migration_rollback_functionality", func(t *testing.T) {
		// Setup test data
		repoID := uuid.New()
		testUUID := uuid.New()
		originalPath := fmt.Sprintf("/tmp/codechunking-workspace/%s/config/database.yml", testUUID.String())

		insertTestChunk(t, pool, ctx, repoID, originalPath)

		// This function doesn't exist yet - test will fail
		backupData, err := createMigrationBackup(ctx, pool)
		require.NoError(t, err)
		require.NotEmpty(t, backupData, "Should create backup before migration")

		// Execute migration
		_, err = executePathMigration(ctx, pool)
		require.NoError(t, err)

		// This function doesn't exist yet - test will fail
		err = rollbackPathMigration(ctx, pool, backupData)
		require.NoError(t, err)

		// Verify rollback
		var restoredPath string
		err = pool.QueryRow(ctx,
			"SELECT file_path FROM code_chunks WHERE repository_id = $1",
			repoID).Scan(&restoredPath)
		require.NoError(t, err)
		assert.Equal(t, originalPath, restoredPath, "Should restore original paths after rollback")
	})

	t.Run("performance_with_large_dataset", func(t *testing.T) {
		// Setup large dataset
		repoID := uuid.New()
		baseUUID := uuid.New()
		chunkCount := 10000

		// Insert test data in batches
		for i := range chunkCount {
			path := fmt.Sprintf("/tmp/codechunking-workspace/%s/src/file_%d.go", baseUUID.String(), i)
			insertTestChunk(t, pool, ctx, repoID, path)

			// Commit in batches to avoid memory issues
			if i%1000 == 0 {
				time.Sleep(1 * time.Millisecond) // Small delay to prevent overwhelming
			}
		}

		start := time.Now()

		// Use repository-scoped migration for better test isolation
		migratedCount, err := executePathMigrationForRepo(ctx, pool, repoID)
		require.NoError(t, err)

		duration := time.Since(start)

		assert.Equal(t, chunkCount, migratedCount, "Should migrate all chunks for this repository")
		assert.Less(t, duration, 30*time.Second, "Migration should complete within 30 seconds")
	})

	t.Run("error_handling_malformed_paths", func(t *testing.T) {
		repoID := uuid.New()
		malformedPaths := []string{
			"",                                    // Empty path
			"/tmp/codechunking-workspace/",        // Missing UUID
			"/tmp/codechunking-workspace/invalid", // Invalid UUID format
			"not-a-path",                          // Invalid path format
		}

		for _, path := range malformedPaths {
			insertTestChunk(t, pool, ctx, repoID, path)
		}

		// Use repository-scoped migration to avoid picking up existing data
		migratedCount, err := executePathMigrationForRepo(ctx, pool, repoID)
		require.NoError(t, err)

		// Should skip malformed paths
		assert.Equal(t, 0, migratedCount, "Should not migrate malformed paths")
	})

	t.Run("concurrent_migration_safety", func(t *testing.T) {
		repoID := uuid.New()
		testUUID := uuid.New()

		// Insert test data
		for i := range 100 {
			path := fmt.Sprintf("/tmp/codechunking-workspace/%s/concurrent/file_%d.ts", testUUID.String(), i)
			insertTestChunk(t, pool, ctx, repoID, path)
		}

		// Run multiple concurrent migrations on the same data
		concurrency := 5
		results := make(chan int, concurrency)

		for range concurrency {
			go func() {
				// Use repository-scoped migration to avoid picking up existing data
				count, err := executePathMigrationForRepo(ctx, pool, repoID)
				if err != nil {
					results <- -1 // Error case
				} else {
					results <- count
				}
			}()
		}

		// Collect results
		successCounts := []int{}
		errorCount := 0
		for range concurrency {
			result := <-results
			if result == -1 {
				errorCount++
			} else {
				successCounts = append(successCounts, result)
			}
		}

		// With proper locking, we should have some errors due to lock contention
		// and only one successful migration with actual migrations
		totalMigrated := 0
		for _, count := range successCounts {
			totalMigrated += count
		}

		// The total migrated should be exactly 100 (the test data size) since only one migration should succeed
		assert.Equal(t, 100, totalMigrated, "Total migrated should equal test data size")
		// We should have fewer successful migrations than concurrency due to locking
		assert.Less(t, len(successCounts), concurrency, "Should have fewer successes than concurrency due to locking")
		// We should have at least some errors due to lock contention
		assert.Positive(t, errorCount, "Should have some errors due to lock contention")
	})
}

// Simple migration lock for concurrent tests.
var migrationLock = make(chan struct{}, 1)

// Helper functions - use implementations from path_migration.go

// Test setup helpers

func getMigrationTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	// Use test database configuration
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "dev",
			Password: "dev",
			Name:     "codechunking",
			SSLMode:  "disable",
		},
	}

	dbURL := cfg.Database.DSN()
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Skipf("Could not connect to test database: %v", err)
	}

	// Test connection
	err = pool.Ping(context.Background())
	if err != nil {
		pool.Close()
		t.Skipf("Could not ping test database: %v", err)
	}

	return pool
}

func cleanupMigrationTestData(t *testing.T, pool *pgxpool.Pool, ctx context.Context) {
	t.Helper()

	// Set search path
	_, err := pool.Exec(ctx, "SET search_path TO codechunking, public")
	if err != nil {
		t.Logf("Warning: failed to set search path: %v", err)
		return
	}

	// Simple cleanup - just remove all test data (this is a test environment)
	_, err = pool.Exec(ctx, "DELETE FROM code_chunks WHERE file_path LIKE '/tmp/codechunking-workspace/%'")
	if err != nil {
		t.Logf("Warning: failed to cleanup test data: %v", err)
	}
}

func insertTestChunk(t *testing.T, pool *pgxpool.Pool, ctx context.Context, repoID uuid.UUID, filePath string) {
	t.Helper()

	chunkID := uuid.New()

	// Set search path
	_, err := pool.Exec(ctx, "SET search_path TO codechunking, public")
	require.NoError(t, err)

	// Insert repository if it doesn't exist - use unique URL
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
