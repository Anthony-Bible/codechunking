package repository

import (
	"context"
	"fmt"
	"testing"

	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPostgreSQLRepositoryRepository_NormalizedURLConstraints tests database-level duplicate prevention
// This test will FAIL initially as normalized URL handling doesn't exist yet
func TestPostgreSQLRepositoryRepository_NormalizedURLConstraints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration test in short mode")
	}

	ctx := context.Background()
	pool := getTestDB(t)
	repo := NewPostgreSQLRepositoryRepository(pool)

	tests := []struct {
		name             string
		firstURL         string
		secondURL        string
		expectConstraint bool
		description      string
	}{
		{
			name:             "constraint_violation_with_git_suffix",
			firstURL:         "https://github.com/owner/repo",
			secondURL:        "https://github.com/owner/repo.git",
			expectConstraint: true,
			description:      "Should prevent duplicate when second URL has .git suffix",
		},
		{
			name:             "constraint_violation_with_case_difference",
			firstURL:         "https://github.com/owner/repo",
			secondURL:        "https://GitHub.com/owner/repo",
			expectConstraint: true,
			description:      "Should prevent duplicate when hostname case differs",
		},
		{
			name:             "constraint_violation_with_protocol_difference",
			firstURL:         "https://github.com/owner/repo",
			secondURL:        "http://github.com/owner/repo",
			expectConstraint: true,
			description:      "Should prevent duplicate when protocol differs",
		},
		{
			name:             "constraint_violation_with_trailing_slash",
			firstURL:         "https://github.com/owner/repo",
			secondURL:        "https://github.com/owner/repo/",
			expectConstraint: true,
			description:      "Should prevent duplicate when second URL has trailing slash",
		},
		{
			name:             "constraint_violation_with_query_params",
			firstURL:         "https://github.com/owner/repo",
			secondURL:        "https://github.com/owner/repo?branch=main",
			expectConstraint: true,
			description:      "Should prevent duplicate when second URL has query parameters",
		},
		{
			name:             "no_constraint_different_repositories",
			firstURL:         "https://github.com/owner/repo1",
			secondURL:        "https://github.com/owner/repo2",
			expectConstraint: false,
			description:      "Should allow different repositories from same owner",
		},
		{
			name:             "no_constraint_different_owners",
			firstURL:         "https://github.com/owner1/repo",
			secondURL:        "https://github.com/owner2/repo",
			expectConstraint: false,
			description:      "Should allow same repository name with different owner",
		},
		{
			name:             "no_constraint_different_hosts",
			firstURL:         "https://github.com/owner/repo",
			secondURL:        "https://gitlab.com/owner/repo",
			expectConstraint: false,
			description:      "Should allow same repository on different hosting providers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before test
			cleanupTestData(t, pool)

			// Create first repository
			firstURL, err := valueobject.NewRepositoryURL(tt.firstURL)
			require.NoError(t, err, "First URL should be valid")

			firstRepo := entity.NewRepository(firstURL, "test-repo-1", nil, nil)
			err = repo.Save(ctx, firstRepo)
			require.NoError(t, err, "First repository should be saved successfully")

			// Attempt to create second repository
			secondURL, err := valueobject.NewRepositoryURL(tt.secondURL)
			require.NoError(t, err, "Second URL should be valid")

			secondRepo := entity.NewRepository(secondURL, "test-repo-2", nil, nil)
			err = repo.Save(ctx, secondRepo)

			if tt.expectConstraint {
				// Should fail due to normalized URL constraint - this will PASS initially as constraint doesn't exist yet
				assert.Error(t, err, tt.description)
				assert.Contains(t, err.Error(), "duplicate", tt.description+" - error should mention duplicate")
			} else {
				// Should succeed - different repositories should be allowed
				assert.NoError(t, err, tt.description)
			}

			// Clean up after test
			cleanupTestData(t, pool)
		})
	}
}

// TestPostgreSQLRepositoryRepository_ExistsByNormalizedURL tests normalized URL existence checking
// This test will FAIL initially as the method doesn't exist yet
func TestPostgreSQLRepositoryRepository_ExistsByNormalizedURL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration test in short mode")
	}

	ctx := context.Background()
	pool := getTestDB(t)
	repoImpl := NewPostgreSQLRepositoryRepository(pool)

	// Clean up before test
	cleanupTestData(t, pool)
	defer cleanupTestData(t, pool)

	// Create repository with one URL format
	originalURL, err := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	require.NoError(t, err)

	originalRepo := entity.NewRepository(originalURL, "test-repo", nil, nil)
	err = repoImpl.Save(ctx, originalRepo)
	require.NoError(t, err, "Original repository should be saved")

	tests := []struct {
		name        string
		checkURL    string
		shouldExist bool
		description string
	}{
		{
			name:        "exists_with_git_suffix",
			checkURL:    "https://github.com/owner/repo.git",
			shouldExist: true,
			description: "Should find repository when checking with .git suffix",
		},
		{
			name:        "exists_with_different_case",
			checkURL:    "https://GitHub.com/owner/repo",
			shouldExist: true,
			description: "Should find repository when checking with different hostname case",
		},
		{
			name:        "exists_with_http_protocol",
			checkURL:    "http://github.com/owner/repo",
			shouldExist: true,
			description: "Should find repository when checking with HTTP protocol",
		},
		{
			name:        "exists_with_trailing_slash",
			checkURL:    "https://github.com/owner/repo/",
			shouldExist: true,
			description: "Should find repository when checking with trailing slash",
		},
		{
			name:        "exists_with_query_params",
			checkURL:    "https://github.com/owner/repo?branch=main",
			shouldExist: true,
			description: "Should find repository when checking with query parameters",
		},
		{
			name:        "not_exists_different_repo",
			checkURL:    "https://github.com/owner/different-repo",
			shouldExist: false,
			description: "Should not find different repository",
		},
		{
			name:        "not_exists_different_owner",
			checkURL:    "https://github.com/different-owner/repo",
			shouldExist: false,
			description: "Should not find repository with different owner",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkURL, err := valueobject.NewRepositoryURL(tt.checkURL)
			require.NoError(t, err, "Check URL should be valid")

			// This method doesn't exist yet - test should FAIL
			exists, err := repoImpl.ExistsByNormalizedURL(ctx, checkURL)

			require.NoError(t, err, "ExistsByNormalizedURL should not return error")
			assert.Equal(t, tt.shouldExist, exists, tt.description)
		})
	}
}

// TestPostgreSQLRepositoryRepository_FindByNormalizedURL tests finding repositories by normalized URL
// This test will FAIL initially as the method doesn't exist yet
func TestPostgreSQLRepositoryRepository_FindByNormalizedURL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration test in short mode")
	}

	ctx := context.Background()
	pool := getTestDB(t)
	repoImpl := NewPostgreSQLRepositoryRepository(pool)

	// Clean up before test
	cleanupTestData(t, pool)
	defer cleanupTestData(t, pool)

	// Create repository with normalized storage
	originalURL, err := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	require.NoError(t, err)

	originalRepo := entity.NewRepository(originalURL, "test-repo", stringPtr("Test repository"), stringPtr("main"))
	err = repoImpl.Save(ctx, originalRepo)
	require.NoError(t, err, "Original repository should be saved")

	tests := []struct {
		name        string
		searchURL   string
		shouldFind  bool
		description string
	}{
		{
			name:        "find_by_git_suffix_variation",
			searchURL:   "https://github.com/owner/repo.git",
			shouldFind:  true,
			description: "Should find repository when searching with .git suffix",
		},
		{
			name:        "find_by_case_variation",
			searchURL:   "https://GitHub.com/owner/repo",
			shouldFind:  true,
			description: "Should find repository when searching with different case",
		},
		{
			name:        "find_by_protocol_variation",
			searchURL:   "http://github.com/owner/repo",
			shouldFind:  true,
			description: "Should find repository when searching with HTTP protocol",
		},
		{
			name:        "find_by_trailing_slash",
			searchURL:   "https://github.com/owner/repo/",
			shouldFind:  true,
			description: "Should find repository when searching with trailing slash",
		},
		{
			name:        "find_by_complex_variation",
			searchURL:   "HTTP://GitHub.COM/owner/repo.git/?branch=main#readme",
			shouldFind:  true,
			description: "Should find repository when searching with complex URL variation",
		},
		{
			name:        "not_find_different_repo",
			searchURL:   "https://github.com/owner/different-repo",
			shouldFind:  false,
			description: "Should not find different repository",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			searchURL, err := valueobject.NewRepositoryURL(tt.searchURL)
			require.NoError(t, err, "Search URL should be valid")

			// This method doesn't exist yet - test should FAIL
			foundRepo, err := repoImpl.FindByNormalizedURL(ctx, searchURL)

			require.NoError(t, err, "FindByNormalizedURL should not return error")

			if tt.shouldFind {
				assert.NotNil(t, foundRepo, tt.description)
				assert.Equal(t, "test-repo", foundRepo.Name(), "Found repository should have correct name")
				assert.Equal(t, "Test repository", *foundRepo.Description(), "Found repository should have correct description")
			} else {
				assert.Nil(t, foundRepo, tt.description)
			}
		})
	}
}

// TestPostgreSQLRepositoryRepository_NormalizedURLIndex tests database index performance for normalized URLs
// This test will FAIL initially as the normalized URL index doesn't exist yet
func TestPostgreSQLRepositoryRepository_NormalizedURLIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration test in short mode")
	}

	ctx := context.Background()
	pool := getTestDB(t)

	// Test that normalized URL index exists and is being used
	// This query will FAIL initially as the index doesn't exist yet
	indexQuery := `
		SELECT indexname, indexdef 
		FROM pg_indexes 
		WHERE schemaname = 'codechunking' 
		AND tablename = 'repositories' 
		AND indexname LIKE '%normalized_url%'`

	rows, err := pool.Query(ctx, indexQuery)
	require.NoError(t, err, "Should be able to query for indexes")
	defer rows.Close()

	hasNormalizedIndex := false
	for rows.Next() {
		var indexName, indexDef string
		err := rows.Scan(&indexName, &indexDef)
		require.NoError(t, err, "Should be able to scan index row")

		if indexName != "" {
			hasNormalizedIndex = true
			t.Logf("Found normalized URL index: %s - %s", indexName, indexDef)
		}
	}

	assert.True(t, hasNormalizedIndex, "Database should have index on normalized URL for performance")
}

// TestPostgreSQLRepositoryRepository_NormalizedURLColumn tests that normalized URL column exists
// This test will FAIL initially as the column doesn't exist yet
func TestPostgreSQLRepositoryRepository_NormalizedURLColumn(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration test in short mode")
	}

	ctx := context.Background()
	pool := getTestDB(t)

	// Test that normalized_url column exists in repositories table
	// This query will FAIL initially as the column doesn't exist yet
	columnQuery := `
		SELECT column_name, data_type, is_nullable 
		FROM information_schema.columns 
		WHERE table_schema = 'codechunking' 
		AND table_name = 'repositories' 
		AND column_name = 'normalized_url'`

	rows, err := pool.Query(ctx, columnQuery)
	require.NoError(t, err, "Should be able to query for columns")
	defer rows.Close()

	hasNormalizedColumn := false
	for rows.Next() {
		var columnName, dataType, isNullable string
		err := rows.Scan(&columnName, &dataType, &isNullable)
		require.NoError(t, err, "Should be able to scan column row")

		if columnName == "normalized_url" {
			hasNormalizedColumn = true
			assert.Equal(t, "character varying", dataType, "Normalized URL column should be varchar type")
			assert.Equal(t, "NO", isNullable, "Normalized URL column should be NOT NULL")
			t.Logf("Found normalized_url column: type=%s, nullable=%s", dataType, isNullable)
		}
	}

	assert.True(t, hasNormalizedColumn, "Database should have normalized_url column in repositories table")
}

// TestPostgreSQLRepositoryRepository_NormalizedURLConstraints tests unique constraint on normalized URL
// This test will FAIL initially as the constraint doesn't exist yet
func TestPostgreSQLRepositoryRepository_NormalizedURLUniqueConstraint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration test in short mode")
	}

	ctx := context.Background()
	pool := getTestDB(t)

	// Test that unique constraint exists on normalized_url column
	// This query will FAIL initially as the constraint doesn't exist yet
	constraintQuery := `
		SELECT constraint_name, constraint_type 
		FROM information_schema.table_constraints 
		WHERE table_schema = 'codechunking' 
		AND table_name = 'repositories' 
		AND constraint_type = 'UNIQUE'
		AND constraint_name LIKE '%normalized_url%'`

	rows, err := pool.Query(ctx, constraintQuery)
	require.NoError(t, err, "Should be able to query for constraints")
	defer rows.Close()

	hasNormalizedConstraint := false
	for rows.Next() {
		var constraintName, constraintType string
		err := rows.Scan(&constraintName, &constraintType)
		require.NoError(t, err, "Should be able to scan constraint row")

		if constraintName != "" {
			hasNormalizedConstraint = true
			assert.Equal(t, "UNIQUE", constraintType, "Should be a UNIQUE constraint")
			t.Logf("Found normalized URL constraint: %s (%s)", constraintName, constraintType)
		}
	}

	assert.True(t, hasNormalizedConstraint, "Database should have UNIQUE constraint on normalized_url column")
}

// TestPostgreSQLRepositoryRepository_ConcurrentDuplicateDetection tests concurrent duplicate detection
// This test will FAIL initially as normalized duplicate handling doesn't exist yet
func TestPostgreSQLRepositoryRepository_ConcurrentDuplicateDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration test in short mode")
	}

	ctx := context.Background()
	pool := getTestDB(t)
	repo := NewPostgreSQLRepositoryRepository(pool)

	// Clean up before test
	cleanupTestData(t, pool)
	defer cleanupTestData(t, pool)

	// Create different URL variations for the same repository
	urlVariations := []string{
		"https://github.com/owner/repo",
		"https://github.com/owner/repo.git",
		"https://GitHub.com/owner/repo",
		"http://github.com/owner/repo/",
		"https://github.com/owner/repo?branch=main",
	}

	// Test concurrent attempts to create repositories with equivalent URLs
	results := make(chan error, len(urlVariations))

	for i, urlStr := range urlVariations {
		go func(index int, url string) {
			repoURL, err := valueobject.NewRepositoryURL(url)
			if err != nil {
				results <- fmt.Errorf("invalid URL %s: %w", url, err)
				return
			}

			testRepo := entity.NewRepository(repoURL, fmt.Sprintf("test-repo-%d", index), nil, nil)
			err = repo.Save(ctx, testRepo)
			results <- err
		}(i, urlStr)
	}

	// Collect results
	var errors []error
	var successes int

	for i := 0; i < len(urlVariations); i++ {
		err := <-results
		if err != nil {
			errors = append(errors, err)
		} else {
			successes++
		}
	}

	// Should have exactly 1 success (first one) and rest should fail due to duplicate detection
	assert.Equal(t, 1, successes, "Should have exactly one successful repository creation")
	assert.Equal(t, len(urlVariations)-1, len(errors), "All other attempts should fail due to duplicate detection")

	// All errors should be related to duplicates/constraints
	for _, err := range errors {
		assert.Contains(t, err.Error(), "duplicate", "Error should indicate duplicate detection: %v", err)
	}
}

// Helper methods

// getTestDB creates a test database connection - reuse the existing setupTestDB function
func getTestDB(t *testing.T) *pgxpool.Pool {
	return setupTestDB(t)
}

// stringPtr returns a pointer to a string (helper for optional fields)
func stringPtr(s string) *string {
	return &s
}

// Interface extension - methods are now implemented in repository_repository.go
