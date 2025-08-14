package repository

import (
	"context"
	"testing"

	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"

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

// TestRepositoryRepository_FindByID tests finding repositories by ID.
func TestRepositoryRepository_FindByID(t *testing.T) {
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

	tests := []struct {
		name        string
		id          uuid.UUID
		expectFound bool
		expectError bool
	}{
		{
			name:        "Existing repository ID should return repository",
			id:          testRepo.ID(),
			expectFound: true,
			expectError: false,
		},
		{
			name:        "Non-existing repository ID should return not found",
			id:          uuid.New(),
			expectFound: false,
			expectError: false,
		},
		{
			name:        "Nil UUID should return error",
			id:          uuid.Nil,
			expectFound: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foundRepo, err := repo.FindByID(ctx, tt.id)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if foundRepo != nil {
					t.Error("Expected nil repository on error")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				if tt.expectFound {
					if foundRepo == nil {
						t.Error("Expected repository but got nil")
					} else {
						if foundRepo.ID() != tt.id {
							t.Errorf("Expected repository ID %s, got %s", tt.id, foundRepo.ID())
						}
						if foundRepo.Name() != testRepo.Name() {
							t.Errorf("Expected repository name %s, got %s", testRepo.Name(), foundRepo.Name())
						}
						if !foundRepo.URL().Equal(testRepo.URL()) {
							t.Errorf("Expected repository URL %s, got %s", testRepo.URL(), foundRepo.URL())
						}
					}
				} else {
					if foundRepo != nil {
						t.Error("Expected nil repository but got one")
					}
				}
			}
		})
	}
}

// TestRepositoryRepository_FindByURL tests finding repositories by URL.
func TestRepositoryRepository_FindByURL(t *testing.T) {
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
			name:        "Existing repository URL should return repository",
			url:         testRepo.URL(),
			expectFound: true,
			expectError: false,
		},
		{
			name:        "Non-existing repository URL should return not found",
			url:         nonExistentURL,
			expectFound: false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foundRepo, err := repo.FindByURL(ctx, tt.url)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if foundRepo != nil {
					t.Error("Expected nil repository on error")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				if tt.expectFound {
					if foundRepo == nil {
						t.Error("Expected repository but got nil")
					} else {
						if !foundRepo.URL().Equal(tt.url) {
							t.Errorf("Expected repository URL %s, got %s", tt.url, foundRepo.URL())
						}
						if foundRepo.ID() != testRepo.ID() {
							t.Errorf("Expected repository ID %s, got %s", testRepo.ID(), foundRepo.ID())
						}
					}
				} else {
					if foundRepo != nil {
						t.Error("Expected nil repository but got one")
					}
				}
			}
		})
	}
}

// TestRepositoryRepository_FindAll tests finding repositories with filters and pagination.
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
				if err == nil {
					t.Error("Expected error but got none")
				}
				if repos != nil {
					t.Error("Expected nil repositories on error")
				}
				if total != 0 {
					t.Errorf("Expected total count 0 on error, got %d", total)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if len(repos) != tt.expectedCount {
					t.Errorf("Expected %d repositories, got %d", tt.expectedCount, len(repos))
				}
				if total != tt.expectedTotal {
					t.Errorf("Expected total count %d, got %d", tt.expectedTotal, total)
				}

				// Verify repositories are valid entities
				for i, r := range repos {
					if r == nil {
						t.Errorf("Repository at index %d is nil", i)
					}
					if r.ID() == uuid.Nil {
						t.Errorf("Repository at index %d has nil ID", i)
					}

					// Check status filter if applied
					if tt.filters.Status != nil {
						if r.Status() != *tt.filters.Status {
							t.Errorf("Repository at index %d has status %s, expected %s",
								i, r.Status(), *tt.filters.Status)
						}
					}
				}

				// Check sort order for name ascending
				if tt.filters.Sort == "name:asc" && len(repos) > 1 {
					for i := 1; i < len(repos); i++ {
						if repos[i-1].Name() > repos[i].Name() {
							t.Error("Repositories not sorted by name ascending")
							break
						}
					}
				}
			}
		})
	}
}

// TestRepositoryRepository_Update tests updating repositories.
func TestRepositoryRepository_Update(t *testing.T) {
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

	tests := []struct {
		name        string
		setupRepo   func(*entity.Repository)
		expectError bool
	}{
		{
			name: "Update repository name should succeed",
			setupRepo: func(r *entity.Repository) {
				r.UpdateName("Updated Repository Name")
			},
			expectError: false,
		},
		{
			name: "Update repository description should succeed",
			setupRepo: func(r *entity.Repository) {
				newDesc := "Updated repository description"
				r.UpdateDescription(&newDesc)
			},
			expectError: false,
		},
		{
			name: "Update repository status should succeed",
			setupRepo: func(r *entity.Repository) {
				_ = r.UpdateStatus(valueobject.RepositoryStatusCloning)
			},
			expectError: false,
		},
		{
			name: "Mark repository as completed should succeed",
			setupRepo: func(r *entity.Repository) {
				_ = r.UpdateStatus(valueobject.RepositoryStatusCloning)
				_ = r.UpdateStatus(valueobject.RepositoryStatusProcessing)
				_ = r.MarkIndexingCompleted("abc123", 100, 500)
			},
			expectError: false,
		},
		{
			name: "Archive repository should succeed",
			setupRepo: func(r *entity.Repository) {
				_ = r.UpdateStatus(valueobject.RepositoryStatusCompleted)
				_ = r.Archive()
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of the test repository
			repoToUpdate, err := repo.FindByID(ctx, testRepo.ID())
			if err != nil {
				t.Fatalf("Failed to find repository for update test: %v", err)
			}

			originalUpdatedAt := repoToUpdate.UpdatedAt()

			// Apply the test modifications
			tt.setupRepo(repoToUpdate)

			// Update the repository
			err = repo.Update(ctx, repoToUpdate)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				// For archived repositories, skip verification since FindByID filters them out
				if repoToUpdate.IsDeleted() {
					// Just verify that the repository was successfully updated in the database
					// by attempting to find it (which should return nil for archived repos)
					archivedRepo, err := repo.FindByID(ctx, repoToUpdate.ID())
					if err != nil {
						t.Errorf("Unexpected error when looking for archived repository: %v", err)
						return
					}
					if archivedRepo != nil {
						t.Error("Expected archived repository to not be found by FindByID")
						return
					}
				} else {
					// Verify the update was persisted for non-archived repositories
					updatedRepo, err := repo.FindByID(ctx, repoToUpdate.ID())
					if err != nil {
						t.Errorf("Failed to find updated repository: %v", err)
						return
					}

					if updatedRepo == nil {
						t.Error("Expected to find updated repository but got nil")
						return
					}

					if updatedRepo.UpdatedAt().Equal(originalUpdatedAt) {
						t.Error("Expected UpdatedAt timestamp to be changed")
					}

					if updatedRepo.Name() != repoToUpdate.Name() {
						t.Errorf("Expected updated name %s, got %s", repoToUpdate.Name(), updatedRepo.Name())
					}

					if updatedRepo.Status() != repoToUpdate.Status() {
						t.Errorf("Expected updated status %s, got %s", repoToUpdate.Status(), updatedRepo.Status())
					}
				}
			}
		})
	}
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
			name: "Delete non-existing repository should return error",
			setupRepo: func() uuid.UUID {
				return uuid.New()
			},
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
