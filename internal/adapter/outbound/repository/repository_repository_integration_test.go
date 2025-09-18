//go:build integration
// +build integration

package repository

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// setupTestDB creates a test database connection pool.
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

	// Clean up existing test data
	cleanupTestData(t, pool)

	return pool
}

// cleanupTestData removes test data from database.
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

// createTestRepository creates a test repository entity with a unique URL.
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

// TestRepositoryRepository_Save tests saving repositories to database.
func TestRepositoryRepository_Save(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)

	tests := []struct {
		name        string
		repository  *entity.Repository
		expectError bool
	}{
		{
			name:        "Valid repository should save successfully",
			repository:  createTestRepository(t),
			expectError: false,
		},
		{
			name: "Repository with nil description should save successfully",
			repository: func() *entity.Repository {
				uniqueID := uuid.New().String()
				testURL, _ := valueobject.NewRepositoryURL("https://github.com/test/repo-no-desc-" + uniqueID)
				return entity.NewRepository(testURL, "Test Repository No Desc", nil, nil)
			}(),
			expectError: false,
		},
		{
			name: "Repository with empty name should save successfully",
			repository: func() *entity.Repository {
				uniqueID := uuid.New().String()
				testURL, _ := valueobject.NewRepositoryURL("https://github.com/test/repo-empty-name-" + uniqueID)
				return entity.NewRepository(testURL, "", nil, nil)
			}(),
			expectError: false,
		},
		{
			name: "Duplicate URL should fail with constraint error",
			repository: func() *entity.Repository {
				testURL, _ := valueobject.NewRepositoryURL("https://github.com/test/duplicate")
				repo1 := entity.NewRepository(testURL, "First Repo", nil, nil)
				// Save first repository
				ctx := context.Background()
				_ = repo.Save(ctx, repo1) // This may fail, but we need it for the duplicate test

				// Create second repository with same URL
				return entity.NewRepository(testURL, "Second Repo", nil, nil)
			}(),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			err := repo.Save(ctx, tt.repository)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestRepositoryRepository_FindByID_ExistingRepository tests finding an existing repository by ID.
func TestRepositoryRepository_FindByID_ExistingRepository(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Create and save test repository
	testRepo := createTestRepository(t)
	err := repo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	foundRepo, err := repo.FindByID(ctx, testRepo.ID())

	assertNoError(t, err, "find existing repository by ID")
	verifyRepositoryFound(t, foundRepo, testRepo)
}

// TestRepositoryRepository_FindByID_NonExistingRepository tests finding a non-existing repository by ID.
func TestRepositoryRepository_FindByID_NonExistingRepository(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	nonExistentID := uuid.New()
	foundRepo, err := repo.FindByID(ctx, nonExistentID)

	assertNoError(t, err, "find non-existing repository by ID")
	if foundRepo != nil {
		t.Error("Expected nil repository but got one")
	}
}

// TestRepositoryRepository_FindByID_NilUUID tests finding repository with nil UUID.
func TestRepositoryRepository_FindByID_NilUUID(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	foundRepo, err := repo.FindByID(ctx, uuid.Nil)

	assertError(t, err, "find repository with nil UUID")
	if foundRepo != nil {
		t.Error("Expected nil repository on error")
	}
}

// TestRepositoryRepository_FindByURL_ExistingRepository tests finding an existing repository by URL.
func TestRepositoryRepository_FindByURL_ExistingRepository(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Create and save test repository
	testRepo := createTestRepository(t)
	err := repo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	foundRepo, err := repo.FindByURL(ctx, testRepo.URL())

	assertNoError(t, err, "find existing repository by URL")
	verifyRepositoryFound(t, foundRepo, testRepo)
}

// TestRepositoryRepository_FindByURL_NonExistingRepository tests finding a non-existing repository by URL.
func TestRepositoryRepository_FindByURL_NonExistingRepository(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	nonExistentURL, _ := valueobject.NewRepositoryURL("https://github.com/nonexistent/repo")
	foundRepo, err := repo.FindByURL(ctx, nonExistentURL)

	assertNoError(t, err, "find non-existing repository by URL")
	if foundRepo != nil {
		t.Error("Expected nil repository but got one")
	}
}

// TestRepositoryRepository_FindAll tests finding repositories with filters and pagination.
//

func TestRepositoryRepository_FindAll(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Create and save multiple test repositories
	for i := range 15 {
		uniqueID := uuid.New().String()
		testURL, _ := valueobject.NewRepositoryURL("https://github.com/test/repo-findall-" + uniqueID)
		description := "Test repository " + string(rune('A'+i))
		testRepo := entity.NewRepository(testURL, "Test Repository "+string(rune('A'+i)), &description, nil)

		// Set different statuses for testing filters
		if i < 5 {
			// Valid transition path: pending -> cloning -> processing -> completed
			_ = testRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
			_ = testRepo.UpdateStatus(valueobject.RepositoryStatusProcessing)
			_ = testRepo.UpdateStatus(valueobject.RepositoryStatusCompleted)
		} else if i < 10 {
			// Valid transition path: pending -> failed
			_ = testRepo.UpdateStatus(valueobject.RepositoryStatusFailed)
		}
		// Rest remain as pending

		err := repo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save test repository %d: %v", i, err)
		}
	}

	tests := []struct {
		name          string
		filters       outbound.RepositoryFilters
		expectedCount int
		expectedTotal int
		expectError   bool
	}{
		{
			name: "No filters should return all repositories with pagination",
			filters: outbound.RepositoryFilters{
				Limit:  10,
				Offset: 0,
			},
			expectedCount: 10,
			expectedTotal: 15,
			expectError:   false,
		},
		{
			name: "Second page should return remaining repositories",
			filters: outbound.RepositoryFilters{
				Limit:  10,
				Offset: 10,
			},
			expectedCount: 5,
			expectedTotal: 15,
			expectError:   false,
		},
		{
			name: "Filter by completed status",
			filters: outbound.RepositoryFilters{
				Status: &[]valueobject.RepositoryStatus{valueobject.RepositoryStatusCompleted}[0],
				Limit:  10,
				Offset: 0,
			},
			expectedCount: 5,
			expectedTotal: 5,
			expectError:   false,
		},
		{
			name: "Filter by failed status",
			filters: outbound.RepositoryFilters{
				Status: &[]valueobject.RepositoryStatus{valueobject.RepositoryStatusFailed}[0],
				Limit:  10,
				Offset: 0,
			},
			expectedCount: 5,
			expectedTotal: 5,
			expectError:   false,
		},
		{
			name: "Filter by pending status",
			filters: outbound.RepositoryFilters{
				Status: &[]valueobject.RepositoryStatus{valueobject.RepositoryStatusPending}[0],
				Limit:  10,
				Offset: 0,
			},
			expectedCount: 5,
			expectedTotal: 5,
			expectError:   false,
		},
		{
			name: "Small limit should limit results",
			filters: outbound.RepositoryFilters{
				Limit:  3,
				Offset: 0,
			},
			expectedCount: 3,
			expectedTotal: 15,
			expectError:   false,
		},
		{
			name: "Sort by name ascending",
			filters: outbound.RepositoryFilters{
				Limit:  5,
				Offset: 0,
				Sort:   "name:asc",
			},
			expectedCount: 5,
			expectedTotal: 15,
			expectError:   false,
		},
		{
			name: "Sort by created_at descending",
			filters: outbound.RepositoryFilters{
				Limit:  5,
				Offset: 0,
				Sort:   "created_at:desc",
			},
			expectedCount: 5,
			expectedTotal: 15,
			expectError:   false,
		},
		{
			name: "Invalid sort field should return error",
			filters: outbound.RepositoryFilters{
				Limit:  5,
				Offset: 0,
				Sort:   "invalid_field:asc",
			},
			expectedCount: 0,
			expectedTotal: 0,
			expectError:   true,
		},
		{
			name: "Negative offset should return error",
			filters: outbound.RepositoryFilters{
				Limit:  10,
				Offset: -1,
			},
			expectedCount: 0,
			expectedTotal: 0,
			expectError:   true,
		},
		{
			name: "Zero or negative limit should return error",
			filters: outbound.RepositoryFilters{
				Limit:  0,
				Offset: 0,
			},
			expectedCount: 0,
			expectedTotal: 0,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repos, total, err := repo.FindAll(ctx, tt.filters)

			if tt.expectError {
				validateErrorCase(t, repos, total, err)
				return
			}

			validateSuccessCase(t, repos, total, err, tt.expectedCount, tt.expectedTotal)
			validateStatusFilter(t, repos, tt.filters.Status)
			validateSortOrder(t, repos, tt.filters.Sort)
		})
	}
}

// Helper functions for TestRepositoryRepository_FindAll to reduce nesting complexity

// validateErrorCase validates that an error case returns expected error state.
func validateErrorCase(t *testing.T, repos []*entity.Repository, total int, err error) {
	if err == nil {
		t.Error("Expected error but got none")
	}
	if repos != nil {
		t.Error("Expected nil repositories on error")
	}
	if total != 0 {
		t.Errorf("Expected total count 0 on error, got %d", total)
	}
}

// validateSuccessCase validates that a success case returns expected results.
func validateSuccessCase(
	t *testing.T,
	repos []*entity.Repository,
	total int,
	err error,
	expectedCount, expectedTotal int,
) {
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
		return
	}
	if len(repos) != expectedCount {
		t.Errorf("Expected %d repositories, got %d", expectedCount, len(repos))
	}
	if total != expectedTotal {
		t.Errorf("Expected total count %d, got %d", expectedTotal, total)
	}

	validateRepositoryEntities(t, repos)
}

// validateRepositoryEntities validates that repositories are valid entities.
func validateRepositoryEntities(t *testing.T, repos []*entity.Repository) {
	for i, r := range repos {
		if r == nil {
			t.Errorf("Repository at index %d is nil", i)
			continue
		}
		if r.ID() == uuid.Nil {
			t.Errorf("Repository at index %d has nil ID", i)
		}
	}
}

// validateStatusFilter checks if repositories match the status filter.
func validateStatusFilter(t *testing.T, repos []*entity.Repository, expectedStatus *valueobject.RepositoryStatus) {
	if expectedStatus == nil {
		return
	}

	for i, r := range repos {
		if r.Status() != *expectedStatus {
			t.Errorf("Repository at index %d has status %s, expected %s",
				i, r.Status(), *expectedStatus)
		}
	}
}

// validateSortOrder checks if repositories are sorted correctly.
func validateSortOrder(t *testing.T, repos []*entity.Repository, sortBy string) {
	if sortBy == "name:asc" && len(repos) > 1 {
		for i := 1; i < len(repos); i++ {
			if repos[i-1].Name() > repos[i].Name() {
				t.Error("Repositories not sorted by name ascending")
				break
			}
		}
	}
}

// TestRepositoryRepository_UpdateName_ValidNames tests updating repository names with valid values.
func TestRepositoryRepository_UpdateName_ValidNames(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testCases := []string{
		"Updated Repository Name",
		"",
		"Repo-Name_With.Special@Characters#123",
	}

	for _, newName := range testCases {
		t.Run("Update to: "+newName, func(t *testing.T) {
			testRepo := createAndSaveTestRepo(ctx, t, repo)
			originalUpdatedAt := testRepo.UpdatedAt()

			testRepo.UpdateName(newName)
			err := repo.Update(ctx, testRepo)
			assertNoError(t, err, "update repository name")

			updatedRepo := verifyBasicUpdate(ctx, t, repo, testRepo.ID(), originalUpdatedAt)

			if updatedRepo.Name() != newName {
				t.Errorf("Expected updated name %s, got %s", newName, updatedRepo.Name())
			}
		})
	}
}

// TestRepositoryRepository_UpdateName_LongNames tests updating repository names with very long values.
func TestRepositoryRepository_UpdateName_LongNames(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	longName := "This is a very long repository name that exceeds normal expectations but should still be handled properly by the system"

	testRepo := createAndSaveTestRepo(ctx, t, repo)
	originalUpdatedAt := testRepo.UpdatedAt()

	testRepo.UpdateName(longName)
	err := repo.Update(ctx, testRepo)
	assertNoError(t, err, "update repository with long name")

	updatedRepo := verifyBasicUpdate(ctx, t, repo, testRepo.ID(), originalUpdatedAt)

	if updatedRepo.Name() != longName {
		t.Errorf("Expected updated name %s, got %s", longName, updatedRepo.Name())
	}
}

// TestRepositoryRepository_UpdateName_UnicodeNames tests updating repository names with unicode values.
func TestRepositoryRepository_UpdateName_UnicodeNames(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	unicodeName := "Repository with 中文 and émojis 🚀"

	testRepo := createAndSaveTestRepo(ctx, t, repo)
	originalUpdatedAt := testRepo.UpdatedAt()

	testRepo.UpdateName(unicodeName)
	err := repo.Update(ctx, testRepo)
	assertNoError(t, err, "update repository with unicode name")

	updatedRepo := verifyBasicUpdate(ctx, t, repo, testRepo.ID(), originalUpdatedAt)

	if updatedRepo.Name() != unicodeName {
		t.Errorf("Expected updated name %s, got %s", unicodeName, updatedRepo.Name())
	}
}

// TestRepositoryRepository_UpdateDescription_ValidDescriptions tests updating with valid descriptions.
func TestRepositoryRepository_UpdateDescription_ValidDescriptions(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	validDesc := "Updated repository description"

	testRepo := createAndSaveTestRepo(ctx, t, repo)
	originalUpdatedAt := testRepo.UpdatedAt()

	testRepo.UpdateDescription(&validDesc)
	err := repo.Update(ctx, testRepo)
	assertNoError(t, err, "update repository description")

	updatedRepo := verifyBasicUpdate(ctx, t, repo, testRepo.ID(), originalUpdatedAt)

	if updatedRepo.Description() == nil || *updatedRepo.Description() != validDesc {
		t.Errorf("Expected updated description %s, got %v", validDesc, updatedRepo.Description())
	}
}

// TestRepositoryRepository_UpdateDescription_NilDescription tests setting description to nil.
func TestRepositoryRepository_UpdateDescription_NilDescription(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testRepo := createAndSaveTestRepo(ctx, t, repo)
	originalUpdatedAt := testRepo.UpdatedAt()

	testRepo.UpdateDescription(nil)
	err := repo.Update(ctx, testRepo)
	assertNoError(t, err, "update repository description to nil")

	updatedRepo := verifyBasicUpdate(ctx, t, repo, testRepo.ID(), originalUpdatedAt)

	if updatedRepo.Description() != nil {
		t.Error("Expected description to be nil")
	}
}

// TestRepositoryRepository_UpdateDescription_EmptyDescription tests setting description to empty string.
func TestRepositoryRepository_UpdateDescription_EmptyDescription(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	emptyDesc := ""

	testRepo := createAndSaveTestRepo(ctx, t, repo)
	originalUpdatedAt := testRepo.UpdatedAt()

	testRepo.UpdateDescription(&emptyDesc)
	err := repo.Update(ctx, testRepo)
	assertNoError(t, err, "update repository description to empty")

	updatedRepo := verifyBasicUpdate(ctx, t, repo, testRepo.ID(), originalUpdatedAt)

	if updatedRepo.Description() == nil || *updatedRepo.Description() != emptyDesc {
		t.Errorf("Expected empty description, got %v", updatedRepo.Description())
	}
}

// TestRepositoryRepository_UpdateDescription_LongDescription tests very long descriptions.
func TestRepositoryRepository_UpdateDescription_LongDescription(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	longDesc := "This is a very long description that contains multiple sentences and should test the system's ability to handle larger text content. It includes various punctuation marks, numbers like 123, and should be properly stored and retrieved from the database without any issues."

	testRepo := createAndSaveTestRepo(ctx, t, repo)
	originalUpdatedAt := testRepo.UpdatedAt()

	testRepo.UpdateDescription(&longDesc)
	err := repo.Update(ctx, testRepo)
	assertNoError(t, err, "update repository with long description")

	updatedRepo := verifyBasicUpdate(ctx, t, repo, testRepo.ID(), originalUpdatedAt)

	if updatedRepo.Description() == nil || *updatedRepo.Description() != longDesc {
		t.Errorf("Expected long description to be preserved")
	}
}

// TestRepositoryRepository_UpdateDescription_SpecialCharacters tests descriptions with special characters.
func TestRepositoryRepository_UpdateDescription_SpecialCharacters(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	specialDesc := "Description with special chars: @#$%^&*()_+-=[]{}|;':\",./<>?"

	testRepo := createAndSaveTestRepo(ctx, t, repo)
	originalUpdatedAt := testRepo.UpdatedAt()

	testRepo.UpdateDescription(&specialDesc)
	err := repo.Update(ctx, testRepo)
	assertNoError(t, err, "update repository with special character description")

	updatedRepo := verifyBasicUpdate(ctx, t, repo, testRepo.ID(), originalUpdatedAt)

	if updatedRepo.Description() == nil || *updatedRepo.Description() != specialDesc {
		t.Errorf("Expected special character description to be preserved")
	}
}

// TestRepositoryRepository_UpdateStatus_SuccessfulTransitions tests valid status transitions.
func TestRepositoryRepository_UpdateStatus_SuccessfulTransitions(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testCases := []struct {
		name string
		from valueobject.RepositoryStatus
		to   valueobject.RepositoryStatus
	}{
		{"Pending to cloning", valueobject.RepositoryStatusPending, valueobject.RepositoryStatusCloning},
		{"Cloning to processing", valueobject.RepositoryStatusCloning, valueobject.RepositoryStatusProcessing},
		{"Processing to completed", valueobject.RepositoryStatusProcessing, valueobject.RepositoryStatusCompleted},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testRepo := createAndSaveTestRepo(ctx, t, repo)

			// Set initial status if not pending, following proper state machine
			if tc.from != valueobject.RepositoryStatusPending {
				switch tc.from {
				case valueobject.RepositoryStatusCloning:
					err := testRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
					assertNoError(t, err, "set initial status to cloning")
				case valueobject.RepositoryStatusProcessing:
					err := testRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
					assertNoError(t, err, "transition to cloning first")
					err = testRepo.UpdateStatus(valueobject.RepositoryStatusProcessing)
					assertNoError(t, err, "transition to processing")
				case valueobject.RepositoryStatusCompleted:
					err := testRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
					assertNoError(t, err, "transition to cloning first")
					err = testRepo.UpdateStatus(valueobject.RepositoryStatusProcessing)
					assertNoError(t, err, "transition to processing")
					err = testRepo.UpdateStatus(valueobject.RepositoryStatusCompleted)
					assertNoError(t, err, "transition to completed")
				case valueobject.RepositoryStatusFailed:
					err := testRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
					assertNoError(t, err, "transition to cloning first")
					err = testRepo.UpdateStatus(valueobject.RepositoryStatusFailed)
					assertNoError(t, err, "transition to failed")
				case valueobject.RepositoryStatusArchived:
					err := testRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
					assertNoError(t, err, "transition to cloning first")
					err = testRepo.UpdateStatus(valueobject.RepositoryStatusProcessing)
					assertNoError(t, err, "transition to processing")
					err = testRepo.UpdateStatus(valueobject.RepositoryStatusCompleted)
					assertNoError(t, err, "transition to completed")
					err = testRepo.UpdateStatus(valueobject.RepositoryStatusArchived)
					assertNoError(t, err, "transition to archived")
				case valueobject.RepositoryStatusPending:
					// This case is already handled by the outer if condition, but included for exhaustiveness
				}
			}

			originalUpdatedAt := testRepo.UpdatedAt()

			err := testRepo.UpdateStatus(tc.to)
			assertNoError(t, err, "update status on entity")

			err = repo.Update(ctx, testRepo)
			assertNoError(t, err, "update repository status")

			updatedRepo := verifyBasicUpdate(ctx, t, repo, testRepo.ID(), originalUpdatedAt)

			if updatedRepo.Status() != tc.to {
				t.Errorf("Expected status %s, got %s", tc.to, updatedRepo.Status())
			}
		})
	}
}

// TestRepositoryRepository_UpdateStatus_FailureTransitions tests valid failure transitions.
func TestRepositoryRepository_UpdateStatus_FailureTransitions(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testCases := []struct {
		name string
		from valueobject.RepositoryStatus
	}{
		{"Pending to failed", valueobject.RepositoryStatusPending},
		{"Cloning to failed", valueobject.RepositoryStatusCloning},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testRepo := createAndSaveTestRepo(ctx, t, repo)

			// Set initial status if not pending
			if tc.from != valueobject.RepositoryStatusPending {
				err := testRepo.UpdateStatus(tc.from)
				assertNoError(t, err, "set initial status")
			}

			originalUpdatedAt := testRepo.UpdatedAt()

			err := testRepo.UpdateStatus(valueobject.RepositoryStatusFailed)
			assertNoError(t, err, "update status to failed")

			err = repo.Update(ctx, testRepo)
			assertNoError(t, err, "update repository status to failed")

			updatedRepo := verifyBasicUpdate(ctx, t, repo, testRepo.ID(), originalUpdatedAt)

			if updatedRepo.Status() != valueobject.RepositoryStatusFailed {
				t.Errorf("Expected status failed, got %s", updatedRepo.Status())
			}
		})
	}
}

// setupRepositoryForIndexing prepares a repository in processing state for indexing completion tests.
func setupRepositoryForIndexing(
	ctx context.Context,
	t *testing.T,
	repo outbound.RepositoryRepository,
) *entity.Repository {
	testRepo := createTestRepository(t)

	err := testRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
	assertNoError(t, err, "set cloning status")

	err = testRepo.UpdateStatus(valueobject.RepositoryStatusProcessing)
	assertNoError(t, err, "set processing status")

	err = repo.Save(ctx, testRepo)
	assertNoError(t, err, "save repository")

	return testRepo
}

// verifyIndexingCompletion verifies that indexing completion was properly persisted.
func verifyIndexingCompletion(
	ctx context.Context,
	t *testing.T,
	repo outbound.RepositoryRepository,
	repoID uuid.UUID,
	originalUpdatedAt interface{},
	expectedCommitHash string,
	expectedChunkCount, expectedTokenCount int,
) {
	updatedRepo := verifyBasicUpdate(ctx, t, repo, repoID, originalUpdatedAt)

	// Verify status is completed
	if updatedRepo.Status() != valueobject.RepositoryStatusCompleted {
		t.Errorf("Expected status to be completed, got %s", updatedRepo.Status())
	}

	// Verify indexing metadata
	if updatedRepo.LastCommitHash() != nil && *updatedRepo.LastCommitHash() != expectedCommitHash {
		t.Errorf("Expected commit hash %s, got %s", expectedCommitHash, *updatedRepo.LastCommitHash())
	}

	if updatedRepo.TotalChunks() != expectedChunkCount {
		t.Errorf("Expected chunk count %d, got %d", expectedChunkCount, updatedRepo.TotalChunks())
	}

	if updatedRepo.TotalFiles() != expectedTokenCount {
		t.Errorf("Expected file count %d, got %d", expectedTokenCount, updatedRepo.TotalFiles())
	}
}

// TestRepositoryRepository_MarkIndexingCompleted_ValidData tests marking indexing completed with valid data.
func TestRepositoryRepository_MarkIndexingCompleted_ValidData(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testRepo := setupRepositoryForIndexing(ctx, t, repo)
	originalUpdatedAt := testRepo.UpdatedAt()

	commitHash := "abc123def456"
	chunkCount := 100
	tokenCount := 5000

	err := testRepo.MarkIndexingCompleted(commitHash, tokenCount, chunkCount)
	assertNoError(t, err, "mark indexing completed")

	err = repo.Update(ctx, testRepo)
	assertNoError(t, err, "update repository with indexing completion")

	verifyIndexingCompletion(ctx, t, repo, testRepo.ID(), originalUpdatedAt, commitHash, chunkCount, tokenCount)
}

// TestRepositoryRepository_MarkIndexingCompleted_EmptyCommitHash tests completion with empty commit hash.
func TestRepositoryRepository_MarkIndexingCompleted_EmptyCommitHash(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testRepo := setupRepositoryForIndexing(ctx, t, repo)
	originalUpdatedAt := testRepo.UpdatedAt()

	commitHash := ""
	chunkCount := 50
	tokenCount := 2500

	err := testRepo.MarkIndexingCompleted(commitHash, tokenCount, chunkCount)
	assertNoError(t, err, "mark indexing completed with empty commit hash")

	err = repo.Update(ctx, testRepo)
	assertNoError(t, err, "update repository with empty commit hash completion")

	verifyIndexingCompletion(ctx, t, repo, testRepo.ID(), originalUpdatedAt, commitHash, chunkCount, tokenCount)
}

// TestRepositoryRepository_MarkIndexingCompleted_ZeroCounts tests completion with zero counts.
func TestRepositoryRepository_MarkIndexingCompleted_ZeroCounts(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testRepo := setupRepositoryForIndexing(ctx, t, repo)
	originalUpdatedAt := testRepo.UpdatedAt()

	commitHash := "def789abc012"
	chunkCount := 0
	tokenCount := 0

	err := testRepo.MarkIndexingCompleted(commitHash, tokenCount, chunkCount)
	assertNoError(t, err, "mark indexing completed with zero counts")

	err = repo.Update(ctx, testRepo)
	assertNoError(t, err, "update repository with zero counts completion")

	verifyIndexingCompletion(ctx, t, repo, testRepo.ID(), originalUpdatedAt, commitHash, chunkCount, tokenCount)
}

// TestRepositoryRepository_MarkIndexingCompleted_LargeCounts tests completion with large counts.
func TestRepositoryRepository_MarkIndexingCompleted_LargeCounts(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testRepo := setupRepositoryForIndexing(ctx, t, repo)
	originalUpdatedAt := testRepo.UpdatedAt()

	commitHash := "fedcba987654"
	chunkCount := 999999
	tokenCount := 5000000

	err := testRepo.MarkIndexingCompleted(commitHash, tokenCount, chunkCount)
	assertNoError(t, err, "mark indexing completed with large counts")

	err = repo.Update(ctx, testRepo)
	assertNoError(t, err, "update repository with large counts completion")

	verifyIndexingCompletion(ctx, t, repo, testRepo.ID(), originalUpdatedAt, commitHash, chunkCount, tokenCount)
}

// TestRepositoryRepository_MarkIndexingCompleted_LongCommitHash tests completion with long commit hash.
func TestRepositoryRepository_MarkIndexingCompleted_LongCommitHash(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testRepo := setupRepositoryForIndexing(ctx, t, repo)
	originalUpdatedAt := testRepo.UpdatedAt()

	commitHash := "abcdef1234567890abcdef1234567890abcdef12"
	chunkCount := 250
	tokenCount := 12500

	err := testRepo.MarkIndexingCompleted(commitHash, tokenCount, chunkCount)
	assertNoError(t, err, "mark indexing completed with long commit hash")

	err = repo.Update(ctx, testRepo)
	assertNoError(t, err, "update repository with long commit hash completion")

	verifyIndexingCompletion(ctx, t, repo, testRepo.ID(), originalUpdatedAt, commitHash, chunkCount, tokenCount)
}

// Helper functions to reduce cognitive complexity

// assertNoError fails the test if err is not nil.
func assertNoError(t *testing.T, err error, operation string) {
	if err != nil {
		t.Fatalf("Failed to %s: %v", operation, err)
	}
}

// assertError fails the test if err is nil.
func assertError(t *testing.T, err error, operation string) {
	if err == nil {
		t.Errorf("Expected error for %s but got none", operation)
	}
}

// createAndSaveTestRepo creates a fresh test repository and saves it.
func createAndSaveTestRepo(ctx context.Context, t *testing.T, repo outbound.RepositoryRepository) *entity.Repository {
	testRepo := createTestRepository(t)
	err := repo.Save(ctx, testRepo)
	assertNoError(t, err, "save test repository")
	return testRepo
}

// verifyBasicUpdate verifies that a repository was updated successfully.
func verifyBasicUpdate(
	ctx context.Context,
	t *testing.T,
	repo outbound.RepositoryRepository,
	repoID uuid.UUID,
	originalUpdatedAt interface{},
) *entity.Repository {
	updatedRepo, err := repo.FindByID(ctx, repoID)
	assertNoError(t, err, "find updated repository")

	if updatedRepo == nil {
		t.Fatal("Expected to find updated repository but got nil")
	}

	// Check that UpdatedAt timestamp changed
	if updatedAt, ok := originalUpdatedAt.(interface{ Equal(interface{}) bool }); ok {
		if updatedAt.Equal(updatedRepo.UpdatedAt()) {
			t.Error("Expected UpdatedAt timestamp to be changed")
		}
	}

	return updatedRepo
}

// verifyRepositoryFound verifies that a found repository matches the expected repository.
func verifyRepositoryFound(t *testing.T, foundRepo, expectedRepo *entity.Repository) {
	if foundRepo == nil {
		t.Error("Expected repository but got nil")
		return
	}

	if foundRepo.ID() != expectedRepo.ID() {
		t.Errorf("Expected repository ID %s, got %s", expectedRepo.ID(), foundRepo.ID())
	}

	if foundRepo.Name() != expectedRepo.Name() {
		t.Errorf("Expected repository name %s, got %s", expectedRepo.Name(), foundRepo.Name())
	}

	if !foundRepo.URL().Equal(expectedRepo.URL()) {
		t.Errorf("Expected repository URL %s, got %s", expectedRepo.URL(), foundRepo.URL())
	}
}

// verifyArchivedRepository verifies that a repository has been properly archived.
func verifyArchivedRepository(ctx context.Context, t *testing.T, repo outbound.RepositoryRepository, repoID uuid.UUID) {
	// Archived repositories should not be found by FindByID
	archivedRepo, err := repo.FindByID(ctx, repoID)
	assertNoError(t, err, "look for archived repository")

	if archivedRepo != nil {
		t.Error("Expected archived repository to not be found by FindByID")
	}
}

// TestRepositoryRepository_Archive_CompletedRepository tests archiving completed repositories.
func TestRepositoryRepository_Archive_CompletedRepository(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testRepo := createTestRepository(t)

	// Setup completed repository
	err := testRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
	assertNoError(t, err, "set cloning status")

	err = testRepo.UpdateStatus(valueobject.RepositoryStatusProcessing)
	assertNoError(t, err, "set processing status")

	err = testRepo.MarkIndexingCompleted("abc123", 100, 5000)
	assertNoError(t, err, "mark indexing completed")

	err = repo.Save(ctx, testRepo)
	assertNoError(t, err, "save completed repository")

	err = testRepo.Archive()
	assertNoError(t, err, "archive completed repository")

	err = repo.Update(ctx, testRepo)
	assertNoError(t, err, "update archived repository")

	verifyArchivedRepository(ctx, t, repo, testRepo.ID())
}

// TestRepositoryRepository_Archive_FailedRepository tests archiving failed repositories.
func TestRepositoryRepository_Archive_FailedRepository(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testRepo := createTestRepository(t)

	err := testRepo.UpdateStatus(valueobject.RepositoryStatusFailed)
	assertNoError(t, err, "set failed status")

	err = repo.Save(ctx, testRepo)
	assertNoError(t, err, "save failed repository")

	err = testRepo.Archive()
	assertNoError(t, err, "archive failed repository")

	err = repo.Update(ctx, testRepo)
	assertNoError(t, err, "update archived repository")

	verifyArchivedRepository(ctx, t, repo, testRepo.ID())
}

// TestRepositoryRepository_Archive_PendingRepository tests that archiving pending repositories fails.
func TestRepositoryRepository_Archive_PendingRepository(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testRepo := createAndSaveTestRepo(ctx, t, repo)

	err := testRepo.Archive()
	assertError(t, err, "archive pending repository")
}

// TestRepositoryRepository_Archive_CloningRepository tests that archiving cloning repositories fails.
func TestRepositoryRepository_Archive_CloningRepository(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testRepo := createTestRepository(t)

	err := testRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
	assertNoError(t, err, "set cloning status")

	err = repo.Save(ctx, testRepo)
	assertNoError(t, err, "save cloning repository")

	err = testRepo.Archive()
	assertError(t, err, "archive cloning repository")
}

// TestRepositoryRepository_Archive_ProcessingRepository tests that archiving processing repositories fails.
func TestRepositoryRepository_Archive_ProcessingRepository(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testRepo := createTestRepository(t)

	err := testRepo.UpdateStatus(valueobject.RepositoryStatusCloning)
	assertNoError(t, err, "set cloning status")

	err = testRepo.UpdateStatus(valueobject.RepositoryStatusProcessing)
	assertNoError(t, err, "set processing status")

	err = repo.Save(ctx, testRepo)
	assertNoError(t, err, "save processing repository")

	err = testRepo.Archive()
	assertError(t, err, "archive processing repository")
}

// TestRepositoryRepository_Delete tests soft deleting repositories.
func TestRepositoryRepository_Delete(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	tests := []struct {
		name        string
		setupRepo   func() uuid.UUID
		expectError bool
	}{
		{
			name: "Delete existing repository should succeed",
			setupRepo: func() uuid.UUID {
				testRepo := createTestRepository(t)
				err := repo.Save(ctx, testRepo)
				if err != nil {
					t.Fatalf("Failed to save test repository: %v", err)
				}
				return testRepo.ID()
			},
			expectError: false,
		},
		{
			name:        "Delete non-existing repository should return error",
			setupRepo:   uuid.New,
			expectError: true,
		},
		{
			name: "Delete with nil UUID should return error",
			setupRepo: func() uuid.UUID {
				return uuid.Nil
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoID := tt.setupRepo()

			err := repo.Delete(ctx, repoID)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				// Verify repository is soft deleted (not returned by normal queries)
				deletedRepo, err := repo.FindByID(ctx, repoID)
				if err == nil && deletedRepo != nil {
					t.Error("Expected deleted repository to not be found")
				}
			}
		})
	}
}

// TestRepositoryRepository_Exists tests checking repository existence.
func TestRepositoryRepository_Exists(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	// This will fail because PostgreSQLRepositoryRepository doesn't exist yet
	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Create and save test repository
	testRepo := createTestRepository(t)
	err := repo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	nonExistentURL, _ := valueobject.NewRepositoryURL("https://github.com/nonexistent/repo")

	tests := []struct {
		name        string
		url         valueobject.RepositoryURL
		expectFound bool
		expectError bool
	}{
		{
			name:        "Existing repository URL should return true",
			url:         testRepo.URL(),
			expectFound: true,
			expectError: false,
		},
		{
			name:        "Non-existing repository URL should return false",
			url:         nonExistentURL,
			expectFound: false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := repo.Exists(ctx, tt.url)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if exists != tt.expectFound {
					t.Errorf("Expected exists to be %v, got %v", tt.expectFound, exists)
				}
			}
		})
	}
}
