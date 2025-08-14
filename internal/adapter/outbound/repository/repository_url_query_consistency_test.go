package repository

import (
	"context"
	"testing"

	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"

	"github.com/google/uuid"
)

// TestFindByURL_UsesRawURLForQuery_ConsistentWithStorage tests that FindByURL uses
// the same raw URL format that was used during storage operations.
// This test will fail initially because FindByURL currently uses url.String() (normalized)
// while Save/Update operations store url.Raw().
func TestFindByURL_UsesRawURLForQuery_ConsistentWithStorage(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Test case 1: Raw URL with mixed case and .git suffix
	// The raw URL contains uppercase letters and .git suffix
	rawURLString := "https://GitHub.com/TestOwner/TestRepo.git"
	rawURL, err := valueobject.NewRepositoryURL(rawURLString)
	if err != nil {
		t.Fatalf("Failed to create raw URL: %v", err)
	}

	// Verify the URL normalization behavior for our test
	if rawURL.Raw() == rawURL.String() {
		t.Skip("Test invalid: Raw and normalized URLs are identical for test case")
	}

	// Create and save repository using raw URL
	testRepo := entity.NewRepository(rawURL, "Test Repository", nil, nil)
	err = repo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	t.Run("FindByURL with raw URL should succeed after fix", func(t *testing.T) {
		// This should work after the fix (when FindByURL uses url.Raw() for query)
		// Currently fails because FindByURL uses url.String() (normalized)
		// but database stores url.Raw()
		foundRepo, err := repo.FindByURL(ctx, rawURL)
		if err != nil {
			t.Errorf("Expected no error when finding by raw URL, got: %v", err)
		}
		if foundRepo == nil {
			t.Error("Expected to find repository by raw URL after fix, but got nil")
			t.Logf("Raw URL used in query: %s", rawURL.Raw())
			t.Logf("Normalized URL that's currently used: %s", rawURL.String())
			t.Logf("Fix required: Change FindByURL line 116 from url.String() to url.Raw()")
		}
		if foundRepo != nil && !foundRepo.URL().Equal(rawURL) {
			t.Errorf("Expected found repository URL to equal original, got %s, want %s",
				foundRepo.URL().String(), rawURL.String())
		}
	})

	t.Run("FindByURL with normalized URL should not find raw-stored repository", func(t *testing.T) {
		// Create a new RepositoryURL with the normalized form
		normalizedURLString := rawURL.String() // This gives us the normalized form
		normalizedURL, err := valueobject.NewRepositoryURL(normalizedURLString)
		if err != nil {
			t.Fatalf("Failed to create normalized URL: %v", err)
		}

		// This demonstrates the inconsistency - normalized URL won't find raw-stored repo
		foundRepo, err := repo.FindByURL(ctx, normalizedURL)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if foundRepo != nil {
			t.Error("Expected NOT to find repository when querying with normalized URL for raw-stored repo")
			t.Logf("This test demonstrates why raw URL consistency is needed")
		}
	})
}

// TestExists_UsesRawURLForQuery_ConsistentWithStorage tests that Exists uses
// the same raw URL format that was used during storage operations.
// This test will fail initially because Exists currently uses url.String() (normalized)
// while Save/Update operations store url.Raw().
func TestExists_UsesRawURLForQuery_ConsistentWithStorage(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Test case: Raw URL with different casing
	rawURLString := "https://GITHUB.COM/owner/repository"
	rawURL, err := valueobject.NewRepositoryURL(rawURLString)
	if err != nil {
		t.Fatalf("Failed to create raw URL: %v", err)
	}

	// Verify this test case has different raw vs normalized URLs
	if rawURL.Raw() == rawURL.String() {
		t.Skip("Test invalid: Raw and normalized URLs are identical for test case")
	}

	// Create and save repository
	testRepo := entity.NewRepository(rawURL, "Test Repository", nil, nil)
	err = repo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	t.Run("Exists with raw URL should return true after fix", func(t *testing.T) {
		// This should return true after the fix (when Exists uses url.Raw() for query)
		// Currently returns false because Exists uses url.String() (normalized)
		// but database stores url.Raw()
		exists, err := repo.Exists(ctx, rawURL)
		if err != nil {
			t.Errorf("Expected no error when checking existence by raw URL, got: %v", err)
		}
		if !exists {
			t.Error("Expected repository to exist when checking by raw URL after fix")
			t.Logf("Raw URL used in query: %s", rawURL.Raw())
			t.Logf("Normalized URL that's currently used: %s", rawURL.String())
			t.Logf("Fix required: Change Exists line 345 from url.String() to url.Raw()")
		}
	})

	t.Run("Exists with normalized URL should return false for raw-stored repository", func(t *testing.T) {
		// Create URL object with normalized form
		normalizedURLString := rawURL.String() // Normalized form
		normalizedURL, err := valueobject.NewRepositoryURL(normalizedURLString)
		if err != nil {
			t.Fatalf("Failed to create normalized URL: %v", err)
		}

		// This demonstrates the inconsistency
		exists, err := repo.Exists(ctx, normalizedURL)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if exists {
			t.Error("Expected repository NOT to exist when checking with normalized URL for raw-stored repo")
			t.Logf("This demonstrates the raw vs normalized URL storage/query mismatch")
		}
	})
}

// TestRepositoryURLConsistency_StorageAndQueryUseSameFormat tests various URL formats
// to ensure that what is stored can be consistently retrieved using the same format.
func TestRepositoryURLConsistency_StorageAndQueryUseSameFormat(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Test cases with different raw URL formats that normalize differently
	testCases := []struct {
		name   string
		rawURL string
	}{
		{
			name:   "Mixed case with .git suffix",
			rawURL: "https://GitHub.com/Owner/Repo.git",
		},
		{
			name:   "All uppercase host",
			rawURL: "https://GITHUB.COM/owner/repo",
		},
		{
			name:   "Mixed case path components",
			rawURL: "https://github.com/OwnerName/RepoName",
		},
		{
			name:   "With .git and mixed case",
			rawURL: "https://Github.Com/User/Project.git",
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create unique URL to avoid conflicts
			uniqueRawURL := tc.rawURL + "-" + uuid.New().String()[:8]
			rawURL, err := valueobject.NewRepositoryURL(uniqueRawURL)
			if err != nil {
				t.Fatalf("Failed to create raw URL: %v", err)
			}

			// Skip if raw and normalized are the same (no point testing)
			if rawURL.Raw() == rawURL.String() {
				t.Skipf("Raw and normalized URLs are identical: %s", uniqueRawURL)
			}

			// Create and save repository
			repoName := "Test Repo " + string(rune('A'+i))
			testRepo := entity.NewRepository(rawURL, repoName, nil, nil)
			err = repo.Save(ctx, testRepo)
			if err != nil {
				t.Fatalf("Failed to save test repository: %v", err)
			}

			t.Run("FindByURL should work with exact raw URL after fix", func(t *testing.T) {
				foundRepo, err := repo.FindByURL(ctx, rawURL)
				if err != nil {
					t.Errorf("Expected no error finding by raw URL, got: %v", err)
				}
				if foundRepo == nil {
					t.Error("Expected to find repository by exact raw URL after fix")
					t.Logf("Raw URL: %s", rawURL.Raw())
					t.Logf("Normalized URL (currently used): %s", rawURL.String())
				}
			})

			t.Run("Exists should return true with exact raw URL after fix", func(t *testing.T) {
				exists, err := repo.Exists(ctx, rawURL)
				if err != nil {
					t.Errorf("Expected no error checking existence by raw URL, got: %v", err)
				}
				if !exists {
					t.Error("Expected repository to exist when checked by exact raw URL after fix")
					t.Logf("Raw URL: %s", rawURL.Raw())
					t.Logf("Normalized URL (currently used): %s", rawURL.String())
				}
			})
		})
	}
}

// TestExistsByNormalizedURL_UnaffectedByRawURLFix verifies that ExistsByNormalizedURL
// continues to work correctly with normalized URLs and is not affected by the raw URL fix.
func TestExistsByNormalizedURL_UnaffectedByRawURLFix(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Create URL with raw and normalized differences
	rawURLString := "https://GitHub.com/TestUser/TestProject.git"
	rawURL, err := valueobject.NewRepositoryURL(rawURLString)
	if err != nil {
		t.Fatalf("Failed to create URL: %v", err)
	}

	// Save repository (stores raw URL)
	testRepo := entity.NewRepository(rawURL, "Test Repository", nil, nil)
	err = repo.Save(ctx, testRepo)
	if err != nil {
		t.Fatalf("Failed to save test repository: %v", err)
	}

	t.Run("ExistsByNormalizedURL should find repository by normalized URL", func(t *testing.T) {
		// This should work because ExistsByNormalizedURL uses the normalized_url column
		exists, err := repo.ExistsByNormalizedURL(ctx, rawURL)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if !exists {
			t.Error("Expected ExistsByNormalizedURL to find repository")
			t.Log("ExistsByNormalizedURL should be unaffected by raw URL consistency fix")
		}
	})

	t.Run("FindByNormalizedURL should find repository by normalized URL", func(t *testing.T) {
		// This should work because FindByNormalizedURL uses the normalized_url column
		foundRepo, err := repo.FindByNormalizedURL(ctx, rawURL)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if foundRepo == nil {
			t.Error("Expected FindByNormalizedURL to find repository")
			t.Log("FindByNormalizedURL should be unaffected by raw URL consistency fix")
		}
	})
}

// TestRawVsNormalizedURLDemonstration demonstrates the current problem by showing
// the difference between raw and normalized URL storage/query behavior.
func TestRawVsNormalizedURLDemonstration(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	t.Run("Demonstrate current inconsistency problem", func(t *testing.T) {
		// Raw URL with characteristics that will be normalized differently
		rawURLString := "https://GitHub.com/DemoUser/DemoRepo.git"
		rawURL, err := valueobject.NewRepositoryURL(rawURLString)
		if err != nil {
			t.Fatalf("Failed to create URL: %v", err)
		}

		// Show the difference between raw and normalized
		t.Logf("Raw URL (stored in db): %s", rawURL.Raw())
		t.Logf("Normalized URL (used in current queries): %s", rawURL.String())

		if rawURL.Raw() == rawURL.String() {
			t.Skip("URLs are identical, can't demonstrate problem")
		}

		// Save repository - this stores the raw URL
		testRepo := entity.NewRepository(rawURL, "Demo Repository", nil, nil)
		err = repo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save repository: %v", err)
		}

		// Try to find with current implementation - this will fail
		// because it queries with normalized URL against raw URL storage
		foundRepo, err := repo.FindByURL(ctx, rawURL)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if foundRepo == nil {
			t.Log("CURRENT BUG DEMONSTRATED: Repository not found due to raw vs normalized URL mismatch")
			t.Log("The fix should make FindByURL and Exists use url.Raw() instead of url.String()")
		} else {
			t.Error("Expected to demonstrate the bug, but repository was found")
		}

		// Verify that ExistsByNormalizedURL works (it should, as it uses normalized_url column)
		exists, err := repo.ExistsByNormalizedURL(ctx, rawURL)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !exists {
			t.Error("ExistsByNormalizedURL should work regardless of the raw URL issue")
		}
	})
}

// TestURLStorageMethodsConsistency ensures that after the fix, the URL field queries
// are consistent with how URLs are stored in the url column.
func TestURLStorageMethodsConsistency(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	// This test verifies the specific lines that need to be fixed:
	// - Line 116 in FindByURL: should use url.Raw() instead of url.String()
	// - Line 345 in Exists: should use url.Raw() instead of url.String()

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Create test data with URLs that have different raw vs normalized forms
	testURLs := []string{
		"https://GitHub.com/user1/repo1.git",
		"https://GITHUB.COM/user2/repo2",
		"https://Github.Com/User3/Repo3.git",
	}

	var savedRepos []*entity.Repository
	for i, urlStr := range testURLs {
		url, err := valueobject.NewRepositoryURL(urlStr)
		if err != nil {
			t.Fatalf("Failed to create URL %s: %v", urlStr, err)
		}

		testRepo := entity.NewRepository(url, "Test Repo "+string(rune('A'+i)), nil, nil)
		err = repo.Save(ctx, testRepo)
		if err != nil {
			t.Fatalf("Failed to save repository %d: %v", i, err)
		}
		savedRepos = append(savedRepos, testRepo)
	}

	t.Run("All saved repositories should be findable by their raw URLs after fix", func(t *testing.T) {
		for i, savedRepo := range savedRepos {
			foundRepo, err := repo.FindByURL(ctx, savedRepo.URL())
			if err != nil {
				t.Errorf("Repository %d: Expected no error, got: %v", i, err)
			}
			if foundRepo == nil {
				t.Errorf("Repository %d: Expected to find repository by raw URL after fix", i)
				t.Logf("Raw URL: %s", savedRepo.URL().Raw())
				t.Logf("Current query uses: %s", savedRepo.URL().String())
			}
		}
	})

	t.Run("All saved repositories should exist when checked by their raw URLs after fix", func(t *testing.T) {
		for i, savedRepo := range savedRepos {
			exists, err := repo.Exists(ctx, savedRepo.URL())
			if err != nil {
				t.Errorf("Repository %d: Expected no error, got: %v", i, err)
			}
			if !exists {
				t.Errorf("Repository %d: Expected repository to exist when checked by raw URL after fix", i)
				t.Logf("Raw URL: %s", savedRepo.URL().Raw())
				t.Logf("Current query uses: %s", savedRepo.URL().String())
			}
		}
	})
}
