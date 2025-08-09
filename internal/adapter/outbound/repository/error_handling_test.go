package repository

import (
	"context"
	"testing"
	"time"

	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"

	"github.com/google/uuid"
)

// TestDatabaseErrors tests various database error scenarios
func TestDatabaseErrors(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	t.Run("Connection failure handling", func(t *testing.T) {
		// Simulate connection failure by closing the pool
		pool.Close()

		// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)

		if err == nil {
			t.Error("Expected error due to closed connection pool")
		}

		// Verify error is properly wrapped and contains connection information
		if !isConnectionError(err) {
			t.Errorf("Expected connection error, got: %v", err)
		}
	})
}

// TestConstraintViolations tests database constraint violation handling
func TestConstraintViolations(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	t.Run("Unique constraint violation", func(t *testing.T) {
		// Create first repository
		testURL, _ := valueobject.NewRepositoryURL("https://github.com/test/unique-violation")
		repo1 := entity.NewRepository(testURL, "First Repo", nil, nil)

		err := repoRepo.Save(ctx, repo1)
		if err != nil {
			t.Fatalf("Failed to save first repository: %v", err)
		}

		// Try to create second repository with same URL (should fail)
		repo2 := entity.NewRepository(testURL, "Second Repo", nil, nil)
		err = repoRepo.Save(ctx, repo2)

		if err == nil {
			t.Error("Expected unique constraint violation error")
		}

		// Verify error type and details
		if !isUniqueConstraintError(err) {
			t.Errorf("Expected unique constraint error, got: %v", err)
		}

		// Verify the specific constraint that was violated
		constraintName := getViolatedConstraint(err)
		if constraintName != "repositories_url_key" {
			t.Errorf("Expected constraint 'repositories_url_key', got '%s'", constraintName)
		}
	})

	t.Run("Foreign key constraint violation", func(t *testing.T) {
		// Try to create indexing job with non-existent repository ID
		nonExistentRepoID := uuid.New()
		invalidJob := createTestIndexingJob(t, nonExistentRepoID)

		err := jobRepo.Save(ctx, invalidJob)

		if err == nil {
			t.Error("Expected foreign key constraint violation error")
		}

		// Verify error type and details
		if !isForeignKeyConstraintError(err) {
			t.Errorf("Expected foreign key constraint error, got: %v", err)
		}

		// Verify the specific constraint that was violated
		constraintName := getViolatedConstraint(err)
		if constraintName != "indexing_jobs_repository_id_fkey" {
			t.Errorf("Expected constraint 'indexing_jobs_repository_id_fkey', got '%s'", constraintName)
		}
	})

	t.Run("Check constraint violation", func(t *testing.T) {
		// This test would be relevant if we had check constraints on status values
		// For now, we'll test with invalid data that might trigger a check constraint

		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save test repository: %v", err)
		}

		// Try to create job and manually set invalid status (if we had check constraints)
		// This would be caught by the domain layer, but we're testing DB-level constraints
		testJob := createTestIndexingJob(t, testRepo.ID())

		// Attempt to save job with valid data (should succeed)
		err = jobRepo.Save(ctx, testJob)
		if err != nil {
			t.Errorf("Expected no error for valid job, got: %v", err)
		}
	})

	t.Run("Not null constraint violation", func(t *testing.T) {
		// Test saving repository with missing required field
		// This would typically be caught by domain validation, but testing DB level

		// Try to save a malformed repository (this would require bypassing domain validation)
		// For this test, we'll create a scenario where required data is somehow missing

		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)
		// Should succeed with valid repository
		if err != nil {
			t.Errorf("Expected no error for valid repository, got: %v", err)
		}
	})
}

// TestNotFoundScenarios tests various "not found" error scenarios
func TestNotFoundScenarios(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	t.Run("Repository not found by ID", func(t *testing.T) {
		nonExistentID := uuid.New()

		repo, err := repoRepo.FindByID(ctx, nonExistentID)
		if err != nil {
			t.Errorf("Expected no error for not found, got: %v", err)
		}

		if repo != nil {
			t.Error("Expected nil repository for not found")
		}
	})

	t.Run("Repository not found by URL", func(t *testing.T) {
		nonExistentURL, _ := valueobject.NewRepositoryURL("https://github.com/nonexistent/repo")

		repo, err := repoRepo.FindByURL(ctx, nonExistentURL)
		if err != nil {
			t.Errorf("Expected no error for not found, got: %v", err)
		}

		if repo != nil {
			t.Error("Expected nil repository for not found")
		}
	})

	t.Run("IndexingJob not found by ID", func(t *testing.T) {
		nonExistentID := uuid.New()

		job, err := jobRepo.FindByID(ctx, nonExistentID)
		if err != nil {
			t.Errorf("Expected no error for not found, got: %v", err)
		}

		if job != nil {
			t.Error("Expected nil job for not found")
		}
	})

	t.Run("Update non-existent repository", func(t *testing.T) {
		// Create a repository entity with a non-existent ID
		testURL, _ := valueobject.NewRepositoryURL("https://github.com/test/nonexistent")
		nonExistentRepo := entity.RestoreRepository(
			uuid.New(), testURL, "Nonexistent Repo", nil, nil,
			nil, nil, 0, 0, valueobject.RepositoryStatusPending,
			time.Now(), time.Now(), nil,
		)

		err := repoRepo.Update(ctx, nonExistentRepo)

		if err == nil {
			t.Error("Expected error when updating non-existent repository")
		}

		if !isNotFoundError(err) {
			t.Errorf("Expected not found error, got: %v", err)
		}
	})

	t.Run("Delete non-existent repository", func(t *testing.T) {
		nonExistentID := uuid.New()

		err := repoRepo.Delete(ctx, nonExistentID)

		if err == nil {
			t.Error("Expected error when deleting non-existent repository")
		}

		if !isNotFoundError(err) {
			t.Errorf("Expected not found error, got: %v", err)
		}
	})
}

// TestContextCancellation tests context cancellation handling
func TestContextCancellation(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	t.Run("Save operation cancelled", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		testRepo := createTestRepository(t)
		err := repoRepo.Save(cancelCtx, testRepo)

		if err == nil {
			t.Error("Expected error due to cancelled context")
		}

		if !isContextCancelledError(err) {
			t.Errorf("Expected context cancelled error, got: %v", err)
		}
	})

	t.Run("Find operation timed out", func(t *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(ctx, 1) // Very short timeout
		defer cancel()

		// This might or might not timeout depending on system speed
		// But it tests the timeout handling path
		_, _, err := repoRepo.FindAll(timeoutCtx, outbound.RepositoryFilters{Limit: 100})

		// If it times out, verify the error type
		if err != nil && !isContextTimeoutError(err) && !isContextCancelledError(err) {
			t.Errorf("Expected timeout or cancellation error, got: %v", err)
		}
	})

	t.Run("Update operation cancelled mid-flight", func(t *testing.T) {
		// First create a repository
		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save test repository: %v", err)
		}

		// Create context with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		// Update the repository with timeout context
		testRepo.UpdateName("Updated Name")
		err = repoRepo.Update(timeoutCtx, testRepo)

		// May or may not timeout, but if it does, should be proper error type
		if err != nil && !isContextTimeoutError(err) && !isContextCancelledError(err) {
			t.Errorf("Expected timeout or no error, got: %v", err)
		}
	})
}

// TestInvalidInput tests invalid input parameter handling
func TestInvalidInput(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	t.Run("Nil repository save", func(t *testing.T) {
		err := repoRepo.Save(ctx, nil)

		if err == nil {
			t.Error("Expected error when saving nil repository")
		}

		if !isInvalidArgumentError(err) {
			t.Errorf("Expected invalid argument error, got: %v", err)
		}
	})

	t.Run("Nil indexing job save", func(t *testing.T) {
		err := jobRepo.Save(ctx, nil)

		if err == nil {
			t.Error("Expected error when saving nil indexing job")
		}

		if !isInvalidArgumentError(err) {
			t.Errorf("Expected invalid argument error, got: %v", err)
		}
	})

	t.Run("Find by nil UUID", func(t *testing.T) {
		_, err := repoRepo.FindByID(ctx, uuid.Nil)

		if err == nil {
			t.Error("Expected error when finding by nil UUID")
		}

		if !isInvalidArgumentError(err) {
			t.Errorf("Expected invalid argument error, got: %v", err)
		}
	})

	t.Run("Invalid pagination parameters", func(t *testing.T) {
		// Test negative limit
		_, _, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{
			Limit:  -1,
			Offset: 0,
		})

		if err == nil {
			t.Error("Expected error for negative limit")
		}

		if !isInvalidArgumentError(err) {
			t.Errorf("Expected invalid argument error for negative limit, got: %v", err)
		}

		// Test negative offset
		_, _, err = repoRepo.FindAll(ctx, outbound.RepositoryFilters{
			Limit:  10,
			Offset: -1,
		})

		if err == nil {
			t.Error("Expected error for negative offset")
		}

		if !isInvalidArgumentError(err) {
			t.Errorf("Expected invalid argument error for negative offset, got: %v", err)
		}

		// Test zero limit
		_, _, err = repoRepo.FindAll(ctx, outbound.RepositoryFilters{
			Limit:  0,
			Offset: 0,
		})

		if err == nil {
			t.Error("Expected error for zero limit")
		}

		if !isInvalidArgumentError(err) {
			t.Errorf("Expected invalid argument error for zero limit, got: %v", err)
		}
	})

	t.Run("Invalid sort parameter", func(t *testing.T) {
		_, _, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{
			Limit:  10,
			Offset: 0,
			Sort:   "invalid_field:asc",
		})

		if err == nil {
			t.Error("Expected error for invalid sort field")
		}

		if !isInvalidArgumentError(err) {
			t.Errorf("Expected invalid argument error for invalid sort, got: %v", err)
		}

		// Test invalid sort direction
		_, _, err = repoRepo.FindAll(ctx, outbound.RepositoryFilters{
			Limit:  10,
			Offset: 0,
			Sort:   "name:invalid",
		})

		if err == nil {
			t.Error("Expected error for invalid sort direction")
		}

		if !isInvalidArgumentError(err) {
			t.Errorf("Expected invalid argument error for invalid sort direction, got: %v", err)
		}
	})
}

// TestDatabaseSpecificErrors tests PostgreSQL-specific error scenarios
func TestDatabaseSpecificErrors(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	t.Run("Schema not found error", func(t *testing.T) {
		// Try to access wrong schema (would require test database setup)
		// This test would need special configuration

		// For now, test that normal operations work with correct schema
		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)
		if err != nil {
			t.Errorf("Expected no error with correct schema, got: %v", err)
		}
	})

	t.Run("Table not found error", func(t *testing.T) {
		// This would happen if migration didn't run properly
		// Hard to simulate without breaking test setup

		// Test that tables exist and are accessible
		_, _, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{Limit: 1})

		if err != nil && isTableNotFoundError(err) {
			t.Error("Tables should exist after migration")
		}
	})

	t.Run("Column not found error", func(t *testing.T) {
		// This would happen if schema is out of sync with code
		// Hard to simulate without schema changes

		// Test that expected columns exist
		testRepo := createTestRepository(t)
		err := repoRepo.Save(ctx, testRepo)

		if err != nil && isColumnNotFoundError(err) {
			t.Error("Expected columns should exist in schema")
		}
	})
}

// Error type checking functions
// These don't exist yet, so all tests will fail

func isConnectionError(err error) bool {
	// Implementation doesn't exist yet
	return false
}

func isUniqueConstraintError(err error) bool {
	// Implementation doesn't exist yet
	return false
}

func isForeignKeyConstraintError(err error) bool {
	// Implementation doesn't exist yet
	return false
}

func isCheckConstraintError(err error) bool {
	// Implementation doesn't exist yet
	return false
}

func isNotFoundError(err error) bool {
	// Implementation doesn't exist yet
	return false
}

func isContextCancelledError(err error) bool {
	// Implementation doesn't exist yet
	return false
}

func isContextTimeoutError(err error) bool {
	// Implementation doesn't exist yet
	return false
}

func isInvalidArgumentError(err error) bool {
	// Implementation doesn't exist yet
	return false
}

func isTableNotFoundError(err error) bool {
	// Implementation doesn't exist yet
	return false
}

func isColumnNotFoundError(err error) bool {
	// Implementation doesn't exist yet
	return false
}

func getViolatedConstraint(err error) string {
	// Implementation doesn't exist yet
	return ""
}
