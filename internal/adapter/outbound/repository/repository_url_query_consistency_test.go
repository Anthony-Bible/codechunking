package repository

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"context"
	"testing"

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

// urlTestCase represents a test case for URL consistency testing.
type urlTestCase struct {
	name   string
	rawURL string
}

// getURLTestCases returns test cases with different raw URL formats that normalize differently.
func getURLTestCases() []urlTestCase {
	return []urlTestCase{
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
}

// createUniqueTestURL creates a unique repository URL for testing to avoid conflicts.
func createUniqueTestURL(baseURL string) (valueobject.RepositoryURL, error) {
	uniqueRawURL := baseURL + "-" + uuid.New().String()[:8]
	return valueobject.NewRepositoryURL(uniqueRawURL)
}

// skipIfURLsIdentical skips the test if raw and normalized URLs are identical.
func skipIfURLsIdentical(t *testing.T, url valueobject.RepositoryURL, uniqueURL string) {
	if url.Raw() == url.String() {
		t.Skipf("Raw and normalized URLs are identical: %s", uniqueURL)
	}
}

// saveTestRepository creates and saves a test repository with the given URL and name.
func saveTestRepository(
	ctx context.Context,
	repo *PostgreSQLRepositoryRepository,
	url valueobject.RepositoryURL,
	name string,
) error {
	testRepo := entity.NewRepository(url, name, nil, nil)
	return repo.Save(ctx, testRepo)
}

// TestRepositoryURLConsistency_FindByURL_ExactRawURL tests that FindByURL works with exact raw URLs.
func TestRepositoryURLConsistency_FindByURL_ExactRawURL(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testCases := getURLTestCases()

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rawURL, err := createUniqueTestURL(tc.rawURL)
			if err != nil {
				t.Fatalf("Failed to create raw URL: %v", err)
			}

			skipIfURLsIdentical(t, rawURL, tc.rawURL)

			repoName := "Test Repo " + string(rune('A'+i))
			err = saveTestRepository(ctx, repo, rawURL, repoName)
			if err != nil {
				t.Fatalf("Failed to save test repository: %v", err)
			}

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
	}
}

// TestRepositoryURLConsistency_Exists_ExactRawURL tests that Exists returns true with exact raw URLs.
func TestRepositoryURLConsistency_Exists_ExactRawURL(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testCases := getURLTestCases()

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rawURL, err := createUniqueTestURL(tc.rawURL)
			if err != nil {
				t.Fatalf("Failed to create raw URL: %v", err)
			}

			skipIfURLsIdentical(t, rawURL, tc.rawURL)

			repoName := "Test Repo " + string(rune('A'+i))
			err = saveTestRepository(ctx, repo, rawURL, repoName)
			if err != nil {
				t.Fatalf("Failed to save test repository: %v", err)
			}

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

// Test data for URL consistency testing.
type urlConsistencyTestData struct {
	TestURLs    []string
	SavedRepos  []*entity.Repository
	RepoService *PostgreSQLRepositoryRepository
	Context     context.Context
}

// TestSetupURLConsistencyTestData_ShouldCreateTestDataStructure tests that the helper
// function creates proper test data for URL consistency testing.
// This test will fail initially because setupURLConsistencyTestData() doesn't exist.
func TestSetupURLConsistencyTestData_ShouldCreateTestDataStructure(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// This should call the helper function that doesn't exist yet
	testData := setupURLConsistencyTestData(ctx, repo)

	// Verify the test data structure
	if testData == nil {
		t.Fatal("Expected setupURLConsistencyTestData to return non-nil test data")
	}

	if len(testData.TestURLs) == 0 {
		t.Error("Expected setupURLConsistencyTestData to provide test URLs")
	}

	if testData.RepoService == nil {
		t.Error("Expected setupURLConsistencyTestData to set RepoService")
	}

	if testData.Context == nil {
		t.Error("Expected setupURLConsistencyTestData to set Context")
	}

	// Verify test URLs have different raw vs normalized forms
	for i, urlStr := range testData.TestURLs {
		url, err := valueobject.NewRepositoryURL(urlStr)
		if err != nil {
			t.Fatalf("URL %d should be valid: %v", i, err)
		}
		if url.Raw() == url.String() {
			t.Errorf("URL %d should have different raw and normalized forms: %s", i, urlStr)
		}
	}
}

// TestCreateAndSaveRepositories_ShouldCreateRepositoriesFromURLs tests that the helper
// function properly creates and saves repositories from the test URLs.
// This test will fail initially because createAndSaveRepositories() doesn't exist.
func TestCreateAndSaveRepositories_ShouldCreateRepositoriesFromURLs(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testURLs := []string{
		"https://GitHub.com/test1/repo1.git",
		"https://GITHUB.COM/test2/repo2",
	}

	// This should call the helper function that doesn't exist yet
	savedRepos, err := createAndSaveRepositories(ctx, repo, testURLs)
	if err != nil {
		t.Fatalf("Expected createAndSaveRepositories to succeed, got error: %v", err)
	}

	if len(savedRepos) != len(testURLs) {
		t.Errorf("Expected %d saved repositories, got %d", len(testURLs), len(savedRepos))
	}

	// Verify each repository was created properly
	for i, savedRepo := range savedRepos {
		if savedRepo == nil {
			t.Errorf("Repository %d should not be nil", i)
			continue
		}

		expectedURL, _ := valueobject.NewRepositoryURL(testURLs[i])
		if !savedRepo.URL().Equal(expectedURL) {
			t.Errorf("Repository %d URL mismatch: got %s, want %s",
				i, savedRepo.URL().String(), expectedURL.String())
		}

		if savedRepo.Name() == "" {
			t.Errorf("Repository %d should have a name", i)
		}
	}
}

// TestValidateRepositoryFindability_ShouldTestFindByURLForAllRepos tests that the helper
// function properly validates that all repositories can be found by their URLs.
// This test will fail initially because validateRepositoryFindability() doesn't exist.
func TestValidateRepositoryFindability_ShouldTestFindByURLForAllRepos(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Create some test repositories manually for this test
	testURL1, _ := valueobject.NewRepositoryURL("https://GitHub.com/find1/repo1.git")
	testURL2, _ := valueobject.NewRepositoryURL("https://GITHUB.COM/find2/repo2")

	testRepo1 := entity.NewRepository(testURL1, "Test Repo 1", nil, nil)
	testRepo2 := entity.NewRepository(testURL2, "Test Repo 2", nil, nil)

	repo.Save(ctx, testRepo1)
	repo.Save(ctx, testRepo2)

	savedRepos := []*entity.Repository{testRepo1, testRepo2}

	// This should call the helper function that doesn't exist yet
	result := validateRepositoryFindability(ctx, repo, savedRepos)

	// Verify the validation result structure
	if result == nil {
		t.Fatal("Expected validateRepositoryFindability to return a result")
	}

	if result.TestName == "" {
		t.Error("Expected validation result to have a test name")
	}

	if len(result.Errors) == 0 && len(result.Successes) == 0 {
		t.Error("Expected validation result to have either errors or successes")
	}

	// The result should indicate whether repositories were found or not
	totalResults := len(result.Errors) + len(result.Successes)
	if totalResults != len(savedRepos) {
		t.Errorf("Expected validation result for %d repositories, got %d total results",
			len(savedRepos), totalResults)
	}
}

// TestValidateRepositoryExistence_ShouldTestExistsForAllRepos tests that the helper
// function properly validates that all repositories exist when checked by their URLs.
// This test will fail initially because validateRepositoryExistence() doesn't exist.
func TestValidateRepositoryExistence_ShouldTestExistsForAllRepos(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Create some test repositories manually for this test
	testURL1, _ := valueobject.NewRepositoryURL("https://GitHub.com/exist1/repo1.git")
	testURL2, _ := valueobject.NewRepositoryURL("https://GITHUB.COM/exist2/repo2")

	testRepo1 := entity.NewRepository(testURL1, "Test Repo 1", nil, nil)
	testRepo2 := entity.NewRepository(testURL2, "Test Repo 2", nil, nil)

	repo.Save(ctx, testRepo1)
	repo.Save(ctx, testRepo2)

	savedRepos := []*entity.Repository{testRepo1, testRepo2}

	// This should call the helper function that doesn't exist yet
	result := validateRepositoryExistence(ctx, repo, savedRepos)

	// Verify the validation result structure
	if result == nil {
		t.Fatal("Expected validateRepositoryExistence to return a result")
	}

	if result.TestName == "" {
		t.Error("Expected validation result to have a test name")
	}

	if len(result.Errors) == 0 && len(result.Successes) == 0 {
		t.Error("Expected validation result to have either errors or successes")
	}

	// The result should indicate whether repositories exist or not
	totalResults := len(result.Errors) + len(result.Successes)
	if totalResults != len(savedRepos) {
		t.Errorf("Expected validation result for %d repositories, got %d total results",
			len(savedRepos), totalResults)
	}
}

// TestAssertRepositoryQueryResult_ShouldFormatAndReportResults tests that the helper
// function properly formats and reports query validation results.
// This test will fail initially because assertRepositoryQueryResult() doesn't exist.
func TestAssertRepositoryQueryResult_ShouldFormatAndReportResults(t *testing.T) {
	// This is a unit test for the assertion helper, so no database needed

	// Create mock validation result
	mockResult := &repositoryQueryValidationResult{
		TestName: "Mock Test",
		Errors: []repositoryQueryError{
			{RepositoryIndex: 0, Message: "Repository 0 not found", URL: "https://example.com/repo1"},
			{RepositoryIndex: 1, Message: "Repository 1 existence check failed", URL: "https://example.com/repo2"},
		},
		Successes: []repositoryQuerySuccess{
			{RepositoryIndex: 2, Message: "Repository 2 found successfully", URL: "https://example.com/repo3"},
		},
	}

	// Create a test recorder to capture test failures
	mockT := &testing.T{}

	// This should call the helper function that doesn't exist yet
	assertRepositoryQueryResult(mockT, mockResult)

	// We can't easily verify the exact test failure messages without more complex mocking,
	// but we can verify the function was called without panicking
	t.Log("assertRepositoryQueryResult should process the validation result without panicking")
}

// repositoryQueryValidationResult represents the result of validating repository queries.
// This struct will be used by the helper functions that don't exist yet.
type repositoryQueryValidationResult struct {
	TestName  string
	Errors    []repositoryQueryError
	Successes []repositoryQuerySuccess
}

type repositoryQueryError struct {
	RepositoryIndex int
	Message         string
	URL             string
}

type repositoryQuerySuccess struct {
	RepositoryIndex int
	Message         string
	URL             string
}

// TestURLStorageMethodsConsistency_RefactoredVersion tests the refactored version
// that uses helper functions to reduce cognitive complexity.
// This test will fail initially because the helper functions don't exist.
func TestURLStorageMethodsConsistency_RefactoredVersion(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	// This test verifies the specific lines that need to be fixed:
	// - Line 116 in FindByURL: should use url.Raw() instead of url.String()
	// - Line 345 in Exists: should use url.Raw() instead of url.String()

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Step 1: Setup test data using helper function
	testData := setupURLConsistencyTestData(ctx, repo)

	// Step 2: Create and save repositories using helper function
	savedRepos, err := createAndSaveRepositories(ctx, repo, testData.TestURLs)
	if err != nil {
		t.Fatalf("Failed to create and save repositories: %v", err)
	}

	// Step 3: Validate FindByURL functionality using helper function
	t.Run("All saved repositories should be findable by their raw URLs after fix", func(t *testing.T) {
		result := validateRepositoryFindability(ctx, repo, savedRepos)
		assertRepositoryQueryResult(t, result)
	})

	// Step 4: Validate Exists functionality using helper function
	t.Run("All saved repositories should exist when checked by their raw URLs after fix", func(t *testing.T) {
		result := validateRepositoryExistence(ctx, repo, savedRepos)
		assertRepositoryQueryResult(t, result)
	})
}

// setupURLConsistencyTestData creates test data for URL consistency testing.
func setupURLConsistencyTestData(ctx context.Context, repo *PostgreSQLRepositoryRepository) *urlConsistencyTestData {
	testURLs := []string{
		"https://GitHub.com/user1/repo1.git",
		"https://GITHUB.COM/user2/repo2",
		"https://Github.Com/User3/Repo3.git",
	}

	return &urlConsistencyTestData{
		TestURLs:    testURLs,
		SavedRepos:  []*entity.Repository{},
		RepoService: repo,
		Context:     ctx,
	}
}

// createAndSaveRepositories creates and saves repositories from the provided URLs.
func createAndSaveRepositories(
	ctx context.Context,
	repo *PostgreSQLRepositoryRepository,
	testURLs []string,
) ([]*entity.Repository, error) {
	var savedRepos []*entity.Repository
	for i, urlStr := range testURLs {
		url, err := valueobject.NewRepositoryURL(urlStr)
		if err != nil {
			return nil, err
		}

		testRepo := entity.NewRepository(url, "Test Repo "+string(rune('A'+i)), nil, nil)
		err = repo.Save(ctx, testRepo)
		if err != nil {
			return nil, err
		}
		savedRepos = append(savedRepos, testRepo)
	}
	return savedRepos, nil
}

// validateRepositoryFindability validates that all repositories can be found by their URLs.
func validateRepositoryFindability(
	ctx context.Context,
	repo *PostgreSQLRepositoryRepository,
	savedRepos []*entity.Repository,
) *repositoryQueryValidationResult {
	result := &repositoryQueryValidationResult{
		TestName:  "FindByURL validation",
		Errors:    []repositoryQueryError{},
		Successes: []repositoryQuerySuccess{},
	}

	for i, savedRepo := range savedRepos {
		foundRepo, err := repo.FindByURL(ctx, savedRepo.URL())
		switch {
		case err != nil:
			result.Errors = append(result.Errors, repositoryQueryError{
				RepositoryIndex: i,
				Message:         "Expected no error, got: " + err.Error(),
				URL:             savedRepo.URL().String(),
			})
		case foundRepo == nil:
			result.Errors = append(result.Errors, repositoryQueryError{
				RepositoryIndex: i,
				Message:         "Expected to find repository by raw URL after fix",
				URL:             savedRepo.URL().String(),
			})
		default:
			result.Successes = append(result.Successes, repositoryQuerySuccess{
				RepositoryIndex: i,
				Message:         "Repository found successfully",
				URL:             savedRepo.URL().String(),
			})
		}
	}

	return result
}

// validateRepositoryExistence validates that all repositories exist when checked by their URLs.
func validateRepositoryExistence(
	ctx context.Context,
	repo *PostgreSQLRepositoryRepository,
	savedRepos []*entity.Repository,
) *repositoryQueryValidationResult {
	result := &repositoryQueryValidationResult{
		TestName:  "Exists validation",
		Errors:    []repositoryQueryError{},
		Successes: []repositoryQuerySuccess{},
	}

	for i, savedRepo := range savedRepos {
		exists, err := repo.Exists(ctx, savedRepo.URL())
		switch {
		case err != nil:
			result.Errors = append(result.Errors, repositoryQueryError{
				RepositoryIndex: i,
				Message:         "Expected no error, got: " + err.Error(),
				URL:             savedRepo.URL().String(),
			})
		case !exists:
			result.Errors = append(result.Errors, repositoryQueryError{
				RepositoryIndex: i,
				Message:         "Expected repository to exist when checked by raw URL after fix",
				URL:             savedRepo.URL().String(),
			})
		default:
			result.Successes = append(result.Successes, repositoryQuerySuccess{
				RepositoryIndex: i,
				Message:         "Repository existence confirmed",
				URL:             savedRepo.URL().String(),
			})
		}
	}

	return result
}

// assertRepositoryQueryResult formats and reports query validation results.
func assertRepositoryQueryResult(t *testing.T, result *repositoryQueryValidationResult) {
	for _, errorResult := range result.Errors {
		t.Errorf("Repository %d: %s", errorResult.RepositoryIndex, errorResult.Message)
		t.Logf("Raw URL: %s", errorResult.URL)
	}

	for _, successResult := range result.Successes {
		t.Logf("Repository %d: %s", successResult.RepositoryIndex, successResult.Message)
	}
}

// TestURLStorageMethodsConsistency ensures that after the fix, the URL field queries
// are consistent with how URLs are stored in the url column.
// This is the refactored version with reduced cognitive complexity using helper functions.
func TestURLStorageMethodsConsistency(t *testing.T) {
	t.Skip("Integration test - requires database setup")

	// This test verifies the specific lines that need to be fixed:
	// - Line 116 in FindByURL: should use url.Raw() instead of url.String()
	// - Line 345 in Exists: should use url.Raw() instead of url.String()

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Step 1: Setup test data using helper function
	testData := setupURLConsistencyTestData(ctx, repo)

	// Step 2: Create and save repositories using helper function
	savedRepos, err := createAndSaveRepositories(ctx, repo, testData.TestURLs)
	if err != nil {
		t.Fatalf("Failed to create and save repositories: %v", err)
	}

	// Step 3: Validate FindByURL functionality using helper function
	t.Run("All saved repositories should be findable by their raw URLs after fix", func(t *testing.T) {
		result := validateRepositoryFindability(ctx, repo, savedRepos)
		assertRepositoryQueryResult(t, result)
	})

	// Step 4: Validate Exists functionality using helper function
	t.Run("All saved repositories should exist when checked by their raw URLs after fix", func(t *testing.T) {
		result := validateRepositoryExistence(ctx, repo, savedRepos)
		assertRepositoryQueryResult(t, result)
	})
}
