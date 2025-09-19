package repository

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// setupTestDB creates a test database connection.
func setupTestDB(t *testing.T) *pgxpool.Pool {
	config := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "codechunking",
		Username: "dev",
		Password: "dev",
		Schema:   "public",
	}
	// This will fail because NewDatabaseConnection doesn't exist yet
	pool, err := NewDatabaseConnection(config)
	if err != nil {
		t.Fatalf("Failed to create test database connection: %v", err)
	}
	return pool
}

// cleanupTestData removes all test data from the database.
func cleanupTestData(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()
	queries := []string{
		"DELETE FROM codechunking.embeddings WHERE 1=1",
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
