package valueobject

import (
	"codechunking/internal/domain/normalization"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestURLNormalizer_NormalizeURL tests comprehensive URL normalization
// This test will FAIL initially as the NormalizeURL function doesn't exist yet.
func TestURLNormalizer_NormalizeURL(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput string
		expectError    bool
		description    string
	}{
		{
			name:           "remove_git_suffix",
			input:          "https://github.com/owner/repo.git",
			expectedOutput: "https://github.com/owner/repo",
			expectError:    false,
			description:    "Should remove .git suffix from GitHub URLs",
		},
		{
			name:           "normalize_case_in_hostname",
			input:          "https://GitHub.com/owner/repo",
			expectedOutput: "https://github.com/owner/repo",
			expectError:    false,
			description:    "Should normalize hostname to lowercase",
		},
		{
			name:           "normalize_case_mixed_hostname",
			input:          "https://GiTHuB.CoM/owner/repo",
			expectedOutput: "https://github.com/owner/repo",
			expectError:    false,
			description:    "Should normalize mixed case hostname to lowercase",
		},
		{
			name:           "remove_trailing_slash",
			input:          "https://github.com/owner/repo/",
			expectedOutput: "https://github.com/owner/repo",
			expectError:    false,
			description:    "Should remove trailing slash from repository URLs",
		},
		{
			name:           "remove_multiple_trailing_slashes",
			input:          "https://github.com/owner/repo///",
			expectedOutput: "https://github.com/owner/repo",
			expectError:    false,
			description:    "Should remove multiple trailing slashes",
		},
		{
			name:           "standardize_protocol_to_https",
			input:          "http://github.com/owner/repo",
			expectedOutput: "https://github.com/owner/repo",
			expectError:    false,
			description:    "Should standardize HTTP protocol to HTTPS for supported hosts",
		},
		{
			name:           "remove_query_parameters",
			input:          "https://github.com/owner/repo?branch=main&tab=readme",
			expectedOutput: "https://github.com/owner/repo",
			expectError:    false,
			description:    "Should remove query parameters from repository URLs",
		},
		{
			name:           "remove_fragment_identifiers",
			input:          "https://github.com/owner/repo#readme",
			expectedOutput: "https://github.com/owner/repo",
			expectError:    false,
			description:    "Should remove fragment identifiers from repository URLs",
		},
		{
			name:           "complex_normalization_all_variations",
			input:          "HTTP://GitHub.COM/owner/repo.git/?branch=main&tab=readme#documentation///",
			expectedOutput: "https://github.com/owner/repo",
			expectError:    false,
			description:    "Should handle complex URL with all normalization requirements",
		},
		{
			name:           "gitlab_url_normalization",
			input:          "HTTPS://GitLab.com/owner/repo.git/",
			expectedOutput: "https://gitlab.com/owner/repo",
			expectError:    false,
			description:    "Should normalize GitLab URLs with same rules",
		},
		{
			name:           "bitbucket_url_normalization",
			input:          "HTTP://BitBucket.org/owner/repo.git/?tab=source",
			expectedOutput: "https://bitbucket.org/owner/repo",
			expectError:    false,
			description:    "Should normalize Bitbucket URLs with same rules",
		},
		{
			name:           "normalize_path_case_for_owner_repo",
			input:          "https://github.com/MyOrg/MyRepo",
			expectedOutput: "https://github.com/myorg/myrepo",
			expectError:    false,
			description:    "Should normalize case in owner and repository names for duplicate detection",
		},
		{
			name:           "empty_url_error",
			input:          "",
			expectedOutput: "",
			expectError:    true,
			description:    "Should return error for empty URL",
		},
		{
			name:           "invalid_url_format_error",
			input:          "not-a-valid-url",
			expectedOutput: "",
			expectError:    true,
			description:    "Should return error for invalid URL format",
		},
		{
			name:           "unsupported_host_error",
			input:          "https://example.com/owner/repo",
			expectedOutput: "",
			expectError:    true,
			description:    "Should return error for unsupported Git hosting provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This function doesn't exist yet - test should FAIL
			normalized, err := normalization.NormalizeRepositoryURL(tt.input)

			if tt.expectError {
				require.Error(t, err, tt.description)
				assert.Empty(t, normalized, "Normalized URL should be empty when error occurs")
			} else {
				require.NoError(t, err, tt.description)
				assert.Equal(t, tt.expectedOutput, normalized, tt.description)
			}
		})
	}
}

// TestURLNormalizer_EquivalentURLs tests that different URL formats for the same repository
// are normalized to the same canonical form
// This test will FAIL initially as the normalization functionality doesn't exist.
func TestURLNormalizer_EquivalentURLs(t *testing.T) {
	tests := []struct {
		name              string
		urls              []string
		expectedCanonical string
		description       string
	}{
		{
			name: "github_url_variations",
			urls: []string{
				"https://github.com/owner/repo",
				"https://github.com/owner/repo.git",
				"https://GitHub.com/owner/repo",
				"https://GITHUB.COM/owner/repo.git",
				"http://github.com/owner/repo",
				"https://github.com/owner/repo/",
				"https://github.com/owner/repo///",
				"https://github.com/owner/repo?branch=main",
				"https://github.com/owner/repo#readme",
				"https://github.com/owner/repo.git/?tab=readme#documentation",
			},
			expectedCanonical: "https://github.com/owner/repo",
			description:       "All GitHub URL variations should normalize to same canonical form",
		},
		{
			name: "gitlab_url_variations",
			urls: []string{
				"https://gitlab.com/owner/repo",
				"https://gitlab.com/owner/repo.git",
				"https://GitLab.com/owner/repo",
				"http://gitlab.com/owner/repo",
				"https://gitlab.com/owner/repo/",
				"https://gitlab.com/owner/repo?tab=activity",
			},
			expectedCanonical: "https://gitlab.com/owner/repo",
			description:       "All GitLab URL variations should normalize to same canonical form",
		},
		{
			name: "bitbucket_url_variations",
			urls: []string{
				"https://bitbucket.org/owner/repo",
				"https://bitbucket.org/owner/repo.git",
				"https://BitBucket.org/owner/repo",
				"http://bitbucket.org/owner/repo",
				"https://bitbucket.org/owner/repo/",
				"https://bitbucket.org/owner/repo?tab=source",
			},
			expectedCanonical: "https://bitbucket.org/owner/repo",
			description:       "All Bitbucket URL variations should normalize to same canonical form",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizedURLs := make([]string, len(tt.urls))

			for i, url := range tt.urls {
				// This function doesn't exist yet - test should FAIL
				normalized, err := normalization.NormalizeRepositoryURL(url)
				require.NoError(t, err, "URL %s should be valid and normalizable", url)
				normalizedURLs[i] = normalized
			}

			// All normalized URLs should be identical
			for i, normalized := range normalizedURLs {
				assert.Equal(t, tt.expectedCanonical, normalized,
					"URL %s should normalize to %s but got %s", tt.urls[i], tt.expectedCanonical, normalized)
			}

			// Verify all URLs normalize to the same value
			for i := 1; i < len(normalizedURLs); i++ {
				assert.Equal(t, normalizedURLs[0], normalizedURLs[i],
					"URLs %s and %s should normalize to the same value", tt.urls[0], tt.urls[i])
			}
		})
	}
}

// TestURLNormalizer_PreserveDistinctRepositories tests that different repositories
// are NOT normalized to the same URL
// This test will FAIL initially as the normalization functionality doesn't exist.
func TestURLNormalizer_PreserveDistinctRepositories(t *testing.T) {
	distinctRepositories := []string{
		"https://github.com/owner/repo1",
		"https://github.com/owner/repo2",
		"https://github.com/owner1/repo",
		"https://github.com/owner2/repo",
		"https://gitlab.com/owner/repo",
		"https://bitbucket.org/owner/repo",
	}

	normalizedURLs := make([]string, len(distinctRepositories))

	for i, url := range distinctRepositories {
		// This function doesn't exist yet - test should FAIL
		normalized, err := normalization.NormalizeRepositoryURL(url)
		require.NoError(t, err, "URL %s should be valid", url)
		normalizedURLs[i] = normalized
	}

	// Ensure all normalized URLs are unique (no false duplicates)
	seen := make(map[string]bool)
	for i, normalized := range normalizedURLs {
		assert.False(t, seen[normalized],
			"Repository %s should have unique normalized URL %s, but it's already seen",
			distinctRepositories[i], normalized)
		seen[normalized] = true
	}

	// Verify we have as many unique normalized URLs as distinct repositories
	assert.Len(t, seen, len(distinctRepositories),
		"Should have %d unique normalized URLs for %d distinct repositories",
		len(distinctRepositories), len(distinctRepositories))
}

// TestURLNormalizer_EdgeCases tests edge cases and potential security issues
// This test will FAIL initially as the normalization functionality doesn't exist.
func TestURLNormalizer_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput string
		expectError    bool
		description    string
	}{
		{
			name:           "international_domain_names",
			input:          "https://github.com/üser/repö",
			expectedOutput: "https://github.com/üser/repö",
			expectError:    false,
			description:    "Should handle international characters in path correctly",
		},
		{
			name:           "very_long_valid_url",
			input:          "https://github.com/" + generateLongRepoName(100) + "/" + generateLongRepoName(100),
			expectedOutput: "https://github.com/" + generateLongRepoName(100) + "/" + generateLongRepoName(100),
			expectError:    false,
			description:    "Should handle very long but valid URLs",
		},
		{
			name:           "url_with_port_number",
			input:          "https://github.com:443/owner/repo",
			expectedOutput: "",
			expectError:    true,
			description:    "Should reject URLs with explicit port numbers for security",
		},
		{
			name:           "url_with_userinfo",
			input:          "https://user:pass@github.com/owner/repo",
			expectedOutput: "",
			expectError:    true,
			description:    "Should reject URLs with user info for security",
		},
		{
			name:           "url_with_encoded_characters",
			input:          "https://github.com/owner/repo%20name",
			expectedOutput: "https://github.com/owner/repo name",
			expectError:    false,
			description:    "Should properly decode URL-encoded characters in path",
		},
		{
			name:           "malicious_encoded_traversal",
			input:          "https://github.com/owner/../../../etc/passwd",
			expectedOutput: "",
			expectError:    true,
			description:    "Should detect and reject path traversal attempts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This function doesn't exist yet - test should FAIL
			normalized, err := normalization.NormalizeRepositoryURL(tt.input)

			if tt.expectError {
				require.Error(t, err, tt.description)
				assert.Empty(t, normalized, "Normalized URL should be empty when error occurs")
			} else {
				require.NoError(t, err, tt.description)
				assert.Equal(t, tt.expectedOutput, normalized, tt.description)
			}
		})
	}
}

// TestRepositoryURL_NormalizedCreation tests that RepositoryURL creation uses normalization
// This test will FAIL initially as normalization isn't integrated into RepositoryURL creation.
func TestRepositoryURL_NormalizedCreation(t *testing.T) {
	tests := []struct {
		name          string
		inputURLs     []string
		shouldBeEqual bool
		description   string
	}{
		{
			name: "equivalent_urls_create_equal_repository_urls",
			inputURLs: []string{
				"https://github.com/owner/repo",
				"https://GitHub.com/owner/repo.git",
				"http://github.com/owner/repo/",
			},
			shouldBeEqual: true,
			description:   "Different variations of same repository should create equal RepositoryURL objects",
		},
		{
			name: "distinct_repos_create_different_repository_urls",
			inputURLs: []string{
				"https://github.com/owner/repo1",
				"https://github.com/owner/repo2",
			},
			shouldBeEqual: false,
			description:   "Different repositories should create different RepositoryURL objects",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var repositoryURLs []RepositoryURL

			for _, inputURL := range tt.inputURLs {
				repoURL, err := NewRepositoryURL(inputURL)
				require.NoError(t, err, "Should be able to create RepositoryURL from %s", inputURL)
				repositoryURLs = append(repositoryURLs, repoURL)
			}

			if tt.shouldBeEqual {
				// All RepositoryURL objects should be equal
				for i := 1; i < len(repositoryURLs); i++ {
					assert.True(t, repositoryURLs[0].Equal(repositoryURLs[i]),
						"RepositoryURL created from %s should equal RepositoryURL created from %s",
						tt.inputURLs[0], tt.inputURLs[i])
				}

				// All should have the same string representation (normalized form)
				for i := 1; i < len(repositoryURLs); i++ {
					assert.Equal(t, repositoryURLs[0].String(), repositoryURLs[i].String(),
						"RepositoryURL from %s should have same string as RepositoryURL from %s",
						tt.inputURLs[0], tt.inputURLs[i])
				}
			} else if len(repositoryURLs) >= 2 {
				// RepositoryURL objects should be different
				assert.False(t, repositoryURLs[0].Equal(repositoryURLs[1]),
					"RepositoryURL created from %s should NOT equal RepositoryURL created from %s",
					tt.inputURLs[0], tt.inputURLs[1])
			}
		})
	}
}

// generateLongRepoName is a helper function for testing long repository names.
func generateLongRepoName(length int) string {
	result := make([]byte, length)
	for i := range result {
		result[i] = 'a' + byte(i%26)
	}
	return string(result)
}
