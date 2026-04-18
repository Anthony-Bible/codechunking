package repository

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func envOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envIntOr(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

// setupTestDB creates a test database connection, skipping if unavailable.
func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg := DatabaseConfig{
		Host:     envOr("CODECHUNK_DATABASE_HOST", "localhost"),
		Port:     envIntOr("CODECHUNK_DATABASE_PORT", 5432),
		Database: envOr("CODECHUNK_DATABASE_NAME", "codechunking"),
		Username: envOr("CODECHUNK_DATABASE_USER", "dev"),
		Password: envOr("CODECHUNK_DATABASE_PASSWORD", "dev"),
		Schema:   envOr("CODECHUNK_DATABASE_SCHEMA", "codechunking"),
	}
	pool, err := NewDatabaseConnection(cfg)
	if err != nil {
		t.Skipf("skipping: postgres unavailable at %s:%d (%v)", cfg.Host, cfg.Port, err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// cleanupTestData removes test data from the database.
// In dev mode, it only cleans test repositories to preserve real data.
// In CI mode, it cleans everything for complete isolation.
func cleanupTestData(t *testing.T, pool *pgxpool.Pool) {
	// Check if we're in development mode - only clean test repositories
	if os.Getenv("TEST_MODE") == "dev" {
		cleanupTestRepositories(t, pool)
		return
	}

	// CI/test mode - clean everything for complete isolation
	ctx := context.Background()
	queries := []string{
		"DELETE FROM codechunking.batch_job_progress WHERE 1=1",
		"DELETE FROM codechunking.embeddings WHERE 1=1",
		"DELETE FROM codechunking.embeddings_partitioned WHERE 1=1", // Fix: Include partitioned table
		"DELETE FROM codechunking.code_chunks WHERE 1=1",
		"DELETE FROM codechunking.indexing_jobs WHERE 1=1",
		"DELETE FROM codechunking.repositories WHERE 1=1",
	}
	for _, query := range queries {
		_, err := pool.Exec(ctx, query)
		if err != nil {
			t.Logf("Warning: Failed to clean up with query %s: %v", query, err)
		}
	}
}

// createTestRepository creates a test repository entity with unique identifiers.
func createTestRepository(t *testing.T) *entity.Repository {
	// Generate unique URL using UUID to avoid collisions
	uniqueID := uuid.New().String()
	testURL, err := valueobject.NewRepositoryURL("https://github.com/test/repo-" + uniqueID)
	if err != nil {
		t.Fatalf("Failed to create test URL: %v", err)
	}
	description := "Test repository for unit testing"
	defaultBranch := "main"
	return entity.NewRepository(testURL, "Test Repository", &description, &defaultBranch)
}

// cleanupTestRepositories removes only repositories with test URLs and all their associated data.
// This preserves any non-test repositories and their data.
func cleanupTestRepositories(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()

	// Manually cascade delete because embeddings_partitioned lacks foreign key constraints
	queries := []string{
		// Delete embeddings from test repositories (partitioned table first - no FK)
		`DELETE FROM codechunking.embeddings_partitioned
		 WHERE repository_id IN (
			 SELECT id FROM codechunking.repositories
			 WHERE url LIKE 'https://github.com/test/%' OR url LIKE '%test/repo-%'
		 )`,
		// Delete embeddings from test repositories (legacy table - has FK)
		`DELETE FROM codechunking.embeddings
		 WHERE chunk_id IN (
			 SELECT c.id FROM codechunking.code_chunks c
			 JOIN codechunking.repositories r ON c.repository_id = r.id
			 WHERE r.url LIKE 'https://github.com/test/%' OR r.url LIKE '%test/repo-%'
		 )`,
		// Delete chunks from test repositories (has FK to repositories)
		`DELETE FROM codechunking.code_chunks
		 WHERE repository_id IN (
			 SELECT id FROM codechunking.repositories
			 WHERE url LIKE 'https://github.com/test/%' OR url LIKE '%test/repo-%'
		 )`,
		// Delete indexing jobs from test repositories (has FK to repositories)
		`DELETE FROM codechunking.indexing_jobs
		 WHERE repository_id IN (
			 SELECT id FROM codechunking.repositories
			 WHERE url LIKE 'https://github.com/test/%' OR url LIKE '%test/repo-%'
		 )`,
		// Finally delete the test repositories
		`DELETE FROM codechunking.repositories
		 WHERE url LIKE 'https://github.com/test/%' OR url LIKE '%test/repo-%'`,
	}

	for _, query := range queries {
		_, err := pool.Exec(ctx, query)
		if err != nil {
			t.Logf("Warning: Failed to clean up with query %s: %v", query, err)
		}
	}
}
