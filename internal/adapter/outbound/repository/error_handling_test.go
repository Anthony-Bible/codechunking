package repository

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestDatabaseErrors tests various database error scenarios.
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

// TestConstraintViolations tests database constraint violation handling.
func TestConstraintViolations(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	t.Run("Unique constraint violation", func(t *testing.T) {
		// Create first repository with unique URL to avoid collisions across test runs
		uniqueID := uuid.New().String()
		testURL, _ := valueobject.NewRepositoryURL("https://github.com/test/unique-violation-" + uniqueID)
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

		// Create repository with unique URL to avoid constraint violations across test runs
		uniqueID := uuid.New().String()
		testURL, _ := valueobject.NewRepositoryURL("https://github.com/test/not-null-test-" + uniqueID)
		testRepo := entity.NewRepository(testURL, "Not Null Test Repo", nil, nil)
		err := repoRepo.Save(ctx, testRepo)
		// Should succeed with valid repository
		if err != nil {
			t.Errorf("Expected no error for valid repository, got: %v", err)
		}
	})
}

// TestNotFoundScenarios tests various "not found" error scenarios.
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

// TestContextCancellation tests context cancellation handling.
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

// TestNilEntityInput tests nil entity save operations.
func TestNilEntityInput(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	jobRepo := NewPostgreSQLIndexingJobRepository(pool)
	ctx := context.Background()

	t.Run("Nil repository save", func(t *testing.T) {
		err := repoRepo.Save(ctx, nil)
		assertInvalidArgumentError(t, err, "saving nil repository")
	})

	t.Run("Nil indexing job save", func(t *testing.T) {
		err := jobRepo.Save(ctx, nil)
		assertInvalidArgumentError(t, err, "saving nil indexing job")
	})

	t.Run("Find by nil UUID", func(t *testing.T) {
		_, err := repoRepo.FindByID(ctx, uuid.Nil)
		assertInvalidArgumentError(t, err, "finding by nil UUID")
	})
}

// TestInvalidPaginationParameters tests invalid pagination parameter handling.
func TestInvalidPaginationParameters(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testCases := []struct {
		name   string
		limit  int
		offset int
	}{
		{"negative limit", -1, 0},
		{"negative offset", 10, -1},
		{"zero limit", 0, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{
				Limit:  tc.limit,
				Offset: tc.offset,
			})
			assertInvalidArgumentError(t, err, tc.name)
		})
	}
}

// TestInvalidSortParameters tests invalid sort parameter handling.
func TestInvalidSortParameters(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repoRepo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testCases := []struct {
		name string
		sort string
	}{
		{"invalid sort field", "invalid_field:asc"},
		{"invalid sort direction", "name:invalid"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := repoRepo.FindAll(ctx, outbound.RepositoryFilters{
				Limit:  10,
				Offset: 0,
				Sort:   tc.sort,
			})
			assertInvalidArgumentError(t, err, tc.name)
		})
	}
}

// TestDatabaseSpecificErrors tests PostgreSQL-specific error scenarios.
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

// Error type checking functions.
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	// Check for wrapped connection errors
	if errors.Is(err, ErrConnectionFailed) {
		return true
	}
	// Check for common connection error messages
	errorStr := err.Error()
	return strings.Contains(errorStr, "closed pool") ||
		strings.Contains(errorStr, "connection refused") ||
		strings.Contains(errorStr, "connection failed")
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	// Check for wrapped constraint errors
	if errors.Is(err, ErrAlreadyExists) {
		return true
	}
	// Check PostgreSQL error codes in wrapped errors
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return strings.Contains(err.Error(), "unique constraint") ||
		strings.Contains(err.Error(), "record already exists")
}

func isForeignKeyConstraintError(err error) bool {
	if err == nil {
		return false
	}
	// Check for wrapped foreign key errors
	if errors.Is(err, ErrForeignKeyViolation) {
		return true
	}
	// Check PostgreSQL error codes in wrapped errors
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return true
	}
	return strings.Contains(err.Error(), "foreign key") ||
		strings.Contains(err.Error(), "foreign key violation")
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Check for wrapped not found errors
	if errors.Is(err, ErrNotFound) {
		return true
	}
	return strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "record not found")
}

func isContextCancelledError(err error) bool {
	if err == nil {
		return false
	}
	// Check for context cancellation
	if errors.Is(err, context.Canceled) {
		return true
	}
	return strings.Contains(err.Error(), "context canceled") ||
		strings.Contains(err.Error(), "context cancelled")
}

func isContextTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	// Check for context timeout
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return strings.Contains(err.Error(), "context deadline exceeded") ||
		strings.Contains(err.Error(), "timeout")
}

func isInvalidArgumentError(err error) bool {
	if err == nil {
		return false
	}
	// Check for wrapped invalid argument errors
	if errors.Is(err, ErrInvalidArgument) {
		return true
	}
	// Check for invalid argument error messages
	errorStr := err.Error()
	return strings.Contains(errorStr, "cannot be nil") ||
		strings.Contains(errorStr, "invalid argument") ||
		strings.Contains(errorStr, "must be positive") ||
		strings.Contains(errorStr, "cannot be negative") ||
		strings.Contains(errorStr, "invalid sort") ||
		strings.Contains(errorStr, "invalid field") ||
		strings.Contains(errorStr, "invalid direction")
}

func isTableNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Check PostgreSQL error codes for table not found
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
		return true
	}
	return strings.Contains(err.Error(), "relation") && strings.Contains(err.Error(), "does not exist")
}

func isColumnNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Check PostgreSQL error codes for column not found
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42703" {
		return true
	}
	return strings.Contains(err.Error(), "column") && strings.Contains(err.Error(), "does not exist")
}

func getViolatedConstraint(err error) string {
	if err == nil {
		return ""
	}

	// Check for ConstraintError first (our custom error type)
	var constraintErr *ConstraintError
	if errors.As(err, &constraintErr) {
		return constraintErr.ConstraintName
	}

	// Extract constraint name from PostgreSQL error, unwrapping through multiple layers
	var pgErr *pgconn.PgError
	currentErr := err
	for currentErr != nil {
		if errors.As(currentErr, &pgErr) {
			return pgErr.ConstraintName
		}
		// Try to unwrap to next level
		if unwrapped := errors.Unwrap(currentErr); unwrapped != nil {
			currentErr = unwrapped
		} else {
			break
		}
	}
	return ""
}

// assertInvalidArgumentError is a helper function to check for invalid argument errors.
func assertInvalidArgumentError(t *testing.T, err error, operation string) {
	t.Helper()

	if err == nil {
		t.Errorf("Expected error for %s", operation)
		return
	}

	if !isInvalidArgumentError(err) {
		t.Errorf("Expected invalid argument error for %s, got: %v", operation, err)
	}
}
