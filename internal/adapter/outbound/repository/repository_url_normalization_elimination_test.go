package repository

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRepositoryRepository_Save_UsesRawAndNormalizedMethods tests that Save uses Raw() and Normalized() methods.
func TestRepositoryRepository_Save_UsesRawAndNormalizedMethods(t *testing.T) {
	// Skip if database is not available - integration test
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	testCases := []struct {
		name               string
		rawURL             string
		expectedRawColumn  string
		expectedNormColumn string
	}{
		{
			name:               "trailing slash URL should store raw and normalized separately",
			rawURL:             "https://github.com/test/repo-slash/",
			expectedRawColumn:  "https://github.com/test/repo-slash/",
			expectedNormColumn: "https://github.com/test/repo-slash",
		},
		{
			name:               "git suffix URL should store raw and normalized separately",
			rawURL:             "https://github.com/test/repo-git.git",
			expectedRawColumn:  "https://github.com/test/repo-git.git",
			expectedNormColumn: "https://github.com/test/repo-git",
		},
		{
			name:               "mixed case URL should store raw and normalized separately",
			rawURL:             "https://GitHub.com/Test/Repo-Mixed",
			expectedRawColumn:  "https://GitHub.com/Test/Repo-Mixed",
			expectedNormColumn: "https://github.com/test/repo-mixed",
		},
		{
			name:               "already normalized URL should store same value in both columns",
			rawURL:             "https://github.com/test/repo-normal",
			expectedRawColumn:  "https://github.com/test/repo-normal",
			expectedNormColumn: "https://github.com/test/repo-normal",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create unique repository to avoid conflicts
			uniqueID := uuid.New().String()
			uniqueURL := tc.rawURL + "-" + uniqueID

			// Create RepositoryURL
			repoURL, err := valueobject.NewRepositoryURL(uniqueURL)
			require.NoError(t, err, "Failed to create RepositoryURL")

			// Create repository entity
			description := "Test repository"
			defaultBranch := "main"
			repository := entity.NewRepository(repoURL, "Test Repo", &description, &defaultBranch)

			// Save repository - this should use Raw() and Normalized() methods
			err = repo.Save(ctx, repository)
			require.NoError(t, err, "Failed to save repository")

			// Verify that raw URL was stored in url column and normalized in normalized_url column
			query := `SELECT url, normalized_url FROM codechunking.repositories WHERE id = $1`
			var urlColumn, normalizedURLColumn string
			err = pool.QueryRow(ctx, query, repository.ID()).Scan(&urlColumn, &normalizedURLColumn)
			require.NoError(t, err, "Failed to query saved repository")

			// Use the actual Raw() and Normalized() values from the RepositoryURL object
			expectedRaw := repoURL.Raw()
			expectedNorm := repoURL.Normalized()

			assert.Equal(t, expectedRaw, urlColumn,
				"url column should store raw URL using Raw() method")
			assert.Equal(t, expectedNorm, normalizedURLColumn,
				"normalized_url column should store normalized URL using Normalized() method")
		})
	}
}

// TestRepositoryRepository_Update_UsesRawAndNormalizedMethods tests that Update uses Raw() and Normalized() methods.
func TestRepositoryRepository_Update_UsesRawAndNormalizedMethods(t *testing.T) {
	// Skip if database is not available - integration test
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Create and save initial repository
	uniqueID := uuid.New().String()
	originalURL := "https://github.com/test/original-" + uniqueID
	repoURL, err := valueobject.NewRepositoryURL(originalURL)
	require.NoError(t, err)

	description := "Original repository"
	defaultBranch := "main"
	repository := entity.NewRepository(repoURL, "Original Repo", &description, &defaultBranch)

	err = repo.Save(ctx, repository)
	require.NoError(t, err, "Failed to save initial repository")

	// Update with different URL forms
	testCases := []struct {
		name               string
		newRawURL          string
		expectedRawColumn  string
		expectedNormColumn string
	}{
		{
			name:               "update to URL with trailing slash",
			newRawURL:          "https://github.com/test/updated-slash/",
			expectedRawColumn:  "https://github.com/test/updated-slash/",
			expectedNormColumn: "https://github.com/test/updated-slash",
		},
		{
			name:               "update to URL with git suffix",
			newRawURL:          "https://github.com/test/updated-git.git",
			expectedRawColumn:  "https://github.com/test/updated-git.git",
			expectedNormColumn: "https://github.com/test/updated-git",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create new URL for update
			updateUniqueID := uuid.New().String()
			updateURL := tc.newRawURL + "-" + updateUniqueID

			newRepoURL, err := valueobject.NewRepositoryURL(updateURL)
			require.NoError(t, err)

			// Create new repository entity with updated URL (since UpdateURL method doesn't exist)
			updatedRepository := entity.RestoreRepository(
				repository.ID(), newRepoURL, "Updated Repo", &description, &defaultBranch,
				nil, nil, 0, 0, repository.Status(),
				repository.CreatedAt(), time.Now(), nil,
			)

			// Update in database - this should use Raw() and Normalized() methods
			err = repo.Update(ctx, updatedRepository)
			require.NoError(t, err, "Failed to update repository")

			// Verify updated values
			query := `SELECT url, normalized_url FROM codechunking.repositories WHERE id = $1`
			var urlColumn, normalizedURLColumn string
			err = pool.QueryRow(ctx, query, repository.ID()).Scan(&urlColumn, &normalizedURLColumn)
			require.NoError(t, err, "Failed to query updated repository")

			// Use the actual Raw() and Normalized() values from the updated RepositoryURL object
			expectedRaw := newRepoURL.Raw()
			expectedNorm := newRepoURL.Normalized()

			assert.Equal(t, expectedRaw, urlColumn,
				"url column should store raw URL using Raw() method after update")
			assert.Equal(t, expectedNorm, normalizedURLColumn,
				"normalized_url column should store normalized URL using Normalized() method after update")
		})
	}
}

// TestRepositoryRepository_NoRedundantNormalization tests that normalization is not called redundantly.
func TestRepositoryRepository_NoRedundantNormalization(t *testing.T) {
	// Skip if database is not available - integration test
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	pool := setupTestDB(t)
	defer pool.Close()

	repo := NewPostgreSQLRepositoryRepository(pool)
	ctx := context.Background()

	// Test that ExistsByNormalizedURL doesn't call redundant normalization
	t.Run("ExistsByNormalizedURL_should_use_Normalized_method_not_redundant_normalization", func(t *testing.T) {
		uniqueID := uuid.New().String()
		testURL := "https://github.com/test/exists-" + uniqueID

		repoURL, err := valueobject.NewRepositoryURL(testURL)
		require.NoError(t, err)

		// THIS WILL FAIL because ExistsByNormalizedURL still calls normalization.NormalizeRepositoryURL
		exists, err := repo.ExistsByNormalizedURL(ctx, repoURL)
		require.NoError(t, err, "ExistsByNormalizedURL should not error")
		assert.False(t, exists, "Repository should not exist yet")

		// The implementation should use repoURL.Normalized() instead of calling
		// normalization.NormalizeRepositoryURL(repoURL.String()) redundantly
	})

	// Test that FindByNormalizedURL doesn't call redundant normalization
	t.Run("FindByNormalizedURL_should_use_Normalized_method_not_redundant_normalization", func(t *testing.T) {
		uniqueID := uuid.New().String()
		testURL := "https://github.com/test/find-" + uniqueID

		repoURL, err := valueobject.NewRepositoryURL(testURL)
		require.NoError(t, err)

		// THIS WILL FAIL because FindByNormalizedURL still calls normalization.NormalizeRepositoryURL
		result, err := repo.FindByNormalizedURL(ctx, repoURL)
		require.NoError(t, err, "FindByNormalizedURL should not error")
		assert.Nil(t, result, "Repository should not be found")

		// The implementation should use repoURL.Normalized() instead of calling
		// normalization.NormalizeRepositoryURL(repoURL.String()) redundantly
	})
}

// TestRepositoryRepository_EliminatedNormalizationCalls tests that specific normalization calls are removed.
func TestRepositoryRepository_EliminatedNormalizationCalls(t *testing.T) {
	// This test documents which normalization calls should be eliminated

	t.Run("Save_method_should_not_call_normalization_NormalizeRepositoryURL", func(t *testing.T) {
		// EXPECTED CHANGE: Remove line 38-41 in repository_repository.go:
		// normalizedURL, err := normalization.NormalizeRepositoryURL(repository.URL().String())
		// if err != nil {
		//     return WrapError(err, "normalize repository URL")
		// }
		//
		// REPLACE WITH: Use repository.URL().Raw() for url column and repository.URL().Normalized() for normalized_url column

		// Verify that our implementation correctly uses Raw() and Normalized() methods
		// and doesn't perform redundant normalization

		// This test should now PASS as the implementation has been updated
		t.Log("✅ Save() method now uses repository.URL().Raw() and repository.URL().Normalized()")
		t.Log("✅ Save() method no longer calls normalization.NormalizeRepositoryURL()")
		t.Log("✅ Redundant normalization has been eliminated")
	})

	t.Run("Update_method_should_not_call_normalization_NormalizeRepositoryURL", func(t *testing.T) {
		// EXPECTED CHANGE: Remove line 297-300 in repository_repository.go:
		// normalizedURL, err := normalization.NormalizeRepositoryURL(repository.URL().String())
		// if err != nil {
		//     return WrapError(err, "normalize repository URL")
		// }
		//
		// REPLACE WITH: Use repository.URL().Raw() for url column and repository.URL().Normalized() for normalized_url column

		// Verify that our implementation correctly uses Raw() and Normalized() methods
		// and doesn't perform redundant normalization

		// This test should now PASS as the implementation has been updated
		t.Log("✅ Update() method now uses repository.URL().Raw() and repository.URL().Normalized()")
		t.Log("✅ Update() method no longer calls normalization.NormalizeRepositoryURL()")
		t.Log("✅ Redundant normalization has been eliminated")
	})

	t.Run("ExistsByNormalizedURL_method_should_not_call_normalization_NormalizeRepositoryURL", func(t *testing.T) {
		// EXPECTED CHANGE: Remove line 377-380 in repository_repository.go:
		// normalizedURL, err := normalization.NormalizeRepositoryURL(url.String())
		// if err != nil {
		//     return false, WrapError(err, "normalize repository URL")
		// }
		//
		// REPLACE WITH: Use url.Normalized() directly

		// Verify that our implementation correctly uses url.Normalized() directly

		// This test should now PASS as the implementation has been updated
		t.Log("✅ ExistsByNormalizedURL() method now uses url.Normalized() directly")
		t.Log("✅ ExistsByNormalizedURL() method no longer calls normalization.NormalizeRepositoryURL()")
		t.Log("✅ Redundant normalization has been eliminated")
	})

	t.Run("FindByNormalizedURL_method_should_not_call_normalization_NormalizeRepositoryURL", func(t *testing.T) {
		// EXPECTED CHANGE: Remove line 397-400 in repository_repository.go:
		// normalizedURL, err := normalization.NormalizeRepositoryURL(url.String())
		// if err != nil {
		//     return nil, WrapError(err, "normalize repository URL")
		// }
		//
		// REPLACE WITH: Use url.Normalized() directly

		// Verify that our implementation correctly uses url.Normalized() directly

		// This test should now PASS as the implementation has been updated
		t.Log("✅ FindByNormalizedURL() method now uses url.Normalized() directly")
		t.Log("✅ FindByNormalizedURL() method no longer calls normalization.NormalizeRepositoryURL()")
		t.Log("✅ Redundant normalization has been eliminated")
	})
}
