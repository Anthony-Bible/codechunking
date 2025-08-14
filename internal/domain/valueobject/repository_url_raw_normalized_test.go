package valueobject

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRepositoryURL_Raw tests that Raw() method returns the original input URL.
func TestRepositoryURL_Raw(t *testing.T) {
	tests := []struct {
		name     string
		rawInput string
		expected string
	}{
		{
			name:     "github URL with trailing slash should preserve raw input",
			rawInput: "https://github.com/golang/go/",
			expected: "https://github.com/golang/go/",
		},
		{
			name:     "github URL without trailing slash should preserve raw input",
			rawInput: "https://github.com/golang/go",
			expected: "https://github.com/golang/go",
		},
		{
			name:     "github URL with .git suffix should preserve raw input",
			rawInput: "https://github.com/golang/go.git",
			expected: "https://github.com/golang/go.git",
		},
		{
			name:     "mixed case URL should preserve raw input",
			rawInput: "https://GitHub.com/GoLang/Go",
			expected: "https://GitHub.com/GoLang/Go",
		},
		{
			name:     "URL with query parameters should preserve raw input",
			rawInput: "https://github.com/golang/go?tab=readme",
			expected: "https://github.com/golang/go?tab=readme",
		},
		{
			name:     "URL with fragment should preserve raw input",
			rawInput: "https://github.com/golang/go#readme",
			expected: "https://github.com/golang/go#readme",
		},
		{
			name:     "gitlab URL should preserve raw input",
			rawInput: "https://gitlab.com/gitlab-org/gitlab/",
			expected: "https://gitlab.com/gitlab-org/gitlab/",
		},
		{
			name:     "bitbucket URL should preserve raw input",
			rawInput: "https://bitbucket.org/atlassian/stash.git",
			expected: "https://bitbucket.org/atlassian/stash.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create RepositoryURL - this should succeed
			repoURL, err := NewRepositoryURL(tt.rawInput)
			require.NoError(t, err, "Failed to create RepositoryURL")

			// Test Raw() method - THIS WILL FAIL because Raw() method doesn't exist yet
			actual := repoURL.Raw()
			assert.Equal(t, tt.expected, actual,
				"Raw() should return original input URL unchanged")
		})
	}
}

// TestRepositoryURL_Normalized tests that Normalized() method returns normalized form.
func TestRepositoryURL_Normalized(t *testing.T) {
	tests := []struct {
		name         string
		rawInput     string
		expectedNorm string
	}{
		{
			name:         "github URL with trailing slash should be normalized",
			rawInput:     "https://github.com/golang/go/",
			expectedNorm: "https://github.com/golang/go",
		},
		{
			name:         "github URL without trailing slash should remain same",
			rawInput:     "https://github.com/golang/go",
			expectedNorm: "https://github.com/golang/go",
		},
		{
			name:         "github URL with .git suffix should be normalized",
			rawInput:     "https://github.com/golang/go.git",
			expectedNorm: "https://github.com/golang/go",
		},
		{
			name:         "mixed case URL should be normalized to lowercase",
			rawInput:     "https://GitHub.com/GoLang/Go",
			expectedNorm: "https://github.com/golang/go",
		},
		{
			name:         "URL with query parameters should be normalized",
			rawInput:     "https://github.com/golang/go?tab=readme",
			expectedNorm: "https://github.com/golang/go",
		},
		{
			name:         "URL with fragment should be normalized",
			rawInput:     "https://github.com/golang/go#readme",
			expectedNorm: "https://github.com/golang/go",
		},
		{
			name:         "gitlab URL with trailing slash should be normalized",
			rawInput:     "https://gitlab.com/gitlab-org/gitlab/",
			expectedNorm: "https://gitlab.com/gitlab-org/gitlab",
		},
		{
			name:         "bitbucket URL with .git should be normalized",
			rawInput:     "https://bitbucket.org/atlassian/stash.git",
			expectedNorm: "https://bitbucket.org/atlassian/stash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create RepositoryURL - this should succeed
			repoURL, err := NewRepositoryURL(tt.rawInput)
			require.NoError(t, err, "Failed to create RepositoryURL")

			// Test Normalized() method - THIS WILL FAIL because Normalized() method doesn't exist yet
			actual := repoURL.Normalized()
			assert.Equal(t, tt.expectedNorm, actual,
				"Normalized() should return properly normalized URL")
		})
	}
}

// TestRepositoryURL_String_BackwardCompatibility tests that String() still returns normalized URL.
func TestRepositoryURL_String_BackwardCompatibility(t *testing.T) {
	tests := []struct {
		name         string
		rawInput     string
		expectedNorm string
	}{
		{
			name:         "String() should return normalized URL for backward compatibility",
			rawInput:     "https://github.com/golang/go/",
			expectedNorm: "https://github.com/golang/go",
		},
		{
			name:         "String() should return normalized URL from .git input",
			rawInput:     "https://github.com/golang/go.git",
			expectedNorm: "https://github.com/golang/go",
		},
		{
			name:         "String() should return normalized URL from mixed case",
			rawInput:     "https://GitHub.com/GoLang/Go",
			expectedNorm: "https://github.com/golang/go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create RepositoryURL
			repoURL, err := NewRepositoryURL(tt.rawInput)
			require.NoError(t, err, "Failed to create RepositoryURL")

			// Test that String() returns normalized URL (backward compatibility)
			actual := repoURL.String()
			assert.Equal(t, tt.expectedNorm, actual,
				"String() should return normalized URL for backward compatibility")
		})
	}
}

// TestRepositoryURL_NormalizedEqualsString tests that Normalized() and String() return same value.
func TestRepositoryURL_NormalizedEqualsString(t *testing.T) {
	testInputs := []string{
		"https://github.com/golang/go/",
		"https://github.com/golang/go.git",
		"https://GitHub.com/GoLang/Go",
		"https://gitlab.com/gitlab-org/gitlab/",
		"https://bitbucket.org/atlassian/stash.git",
	}

	for _, rawInput := range testInputs {
		t.Run("normalized_equals_string_for_"+rawInput, func(t *testing.T) {
			// Create RepositoryURL
			repoURL, err := NewRepositoryURL(rawInput)
			require.NoError(t, err, "Failed to create RepositoryURL")

			// THIS WILL FAIL because Normalized() method doesn't exist yet
			normalized := repoURL.Normalized()
			stringResult := repoURL.String()

			assert.Equal(t, normalized, stringResult,
				"Normalized() and String() should return the same value")
		})
	}
}

// TestRepositoryURL_RawDiffersFromNormalized tests that raw and normalized can be different.
func TestRepositoryURL_RawDiffersFromNormalized(t *testing.T) {
	testCases := []struct {
		name     string
		rawInput string
	}{
		{
			name:     "trailing slash should differ",
			rawInput: "https://github.com/golang/go/",
		},
		{
			name:     "git suffix should differ",
			rawInput: "https://github.com/golang/go.git",
		},
		{
			name:     "mixed case should differ",
			rawInput: "https://GitHub.com/GoLang/Go",
		},
		{
			name:     "query params should differ",
			rawInput: "https://github.com/golang/go?tab=readme",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create RepositoryURL
			repoURL, err := NewRepositoryURL(tc.rawInput)
			require.NoError(t, err, "Failed to create RepositoryURL")

			// THIS WILL FAIL because Raw() and Normalized() methods don't exist yet
			raw := repoURL.Raw()
			normalized := repoURL.Normalized()

			assert.NotEqual(t, raw, normalized,
				"Raw() and Normalized() should differ for input that needs normalization")
		})
	}
}

// TestRepositoryURL_RawEqualsNormalizedWhenAlreadyNormalized tests equal values for already normalized input.
func TestRepositoryURL_RawEqualsNormalizedWhenAlreadyNormalized(t *testing.T) {
	testCases := []string{
		"https://github.com/golang/go",
		"https://gitlab.com/gitlab-org/gitlab",
		"https://bitbucket.org/atlassian/stash",
	}

	for _, rawInput := range testCases {
		t.Run("already_normalized_"+rawInput, func(t *testing.T) {
			// Create RepositoryURL
			repoURL, err := NewRepositoryURL(rawInput)
			require.NoError(t, err, "Failed to create RepositoryURL")

			// THIS WILL FAIL because Raw() and Normalized() methods don't exist yet
			raw := repoURL.Raw()
			normalized := repoURL.Normalized()

			assert.Equal(t, raw, normalized,
				"Raw() and Normalized() should be equal when input is already normalized")
		})
	}
}
