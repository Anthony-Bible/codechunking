package valueobject

import (
	"strings"
	"testing"
	"time"
	"unicode"

	"codechunking/internal/domain/normalization"
)

// FuzzNormalizeRepositoryURL tests URL normalization with random inputs
// This test will FAIL initially as the NormalizeRepositoryURL function doesn't exist yet.
func FuzzNormalizeRepositoryURL(f *testing.F) {
	// Seed fuzzer with known test cases
	seedURLs := []string{
		"https://github.com/owner/repo",
		"https://github.com/owner/repo.git",
		"https://GitHub.com/owner/repo",
		"http://github.com/owner/repo",
		"https://github.com/owner/repo/",
		"https://github.com/owner/repo?branch=main",
		"https://github.com/owner/repo#readme",
		"https://gitlab.com/owner/repo.git",
		"https://bitbucket.org/owner/repo",
		"HTTPS://GITHUB.COM/OWNER/REPO.GIT",
		"https://github.com/owner/repo.git/?tab=readme#documentation",
	}

	for _, url := range seedURLs {
		f.Add(url)
	}

	f.Fuzz(func(t *testing.T, url string) {
		// Skip empty strings and very short strings that are clearly invalid
		if len(url) < 10 {
			return
		}

		// Skip strings that contain control characters that would never be in a valid URL
		if containsControlCharacters(url) {
			return
		}

		// Skip strings that are clearly not URLs (no protocol)
		if !strings.Contains(url, "://") {
			return
		}

		// This function doesn't exist yet - test should FAIL
		normalized, err := normalization.NormalizeRepositoryURL(url)
		if err != nil {
			// If there's an error, it should be a meaningful validation error
			if len(err.Error()) == 0 {
				t.Errorf("Error message should not be empty for URL: %s", url)
			}

			// Normalized result should be empty when there's an error
			if normalized != "" {
				t.Errorf("Normalized URL should be empty when error occurs, got: %s for URL: %s", normalized, url)
			}
			return
		}

		// If normalization succeeded, perform consistency checks
		validateNormalizedURL(t, url, normalized)
	})
}

// FuzzRepositoryURLCreationWithNormalization tests RepositoryURL creation with fuzzing
// This test will FAIL initially as normalization isn't integrated into RepositoryURL creation.
func FuzzRepositoryURLCreationWithNormalization(f *testing.F) {
	// Seed with valid repository URLs
	seedURLs := []string{
		"https://github.com/user/repo",
		"https://gitlab.com/user/repo",
		"https://bitbucket.org/user/repo",
	}

	for _, url := range seedURLs {
		f.Add(url)
	}

	f.Fuzz(func(t *testing.T, url string) {
		// Skip obviously invalid inputs
		if len(url) < 15 || containsControlCharacters(url) {
			return
		}

		// Only test with supported hosts to avoid unnecessary noise
		if !containsSupportedHost(url) {
			return
		}

		// Attempt to create RepositoryURL
		repoURL, err := NewRepositoryURL(url)
		if err != nil {
			// If creation failed, the error should be meaningful
			if len(err.Error()) == 0 {
				t.Errorf("Error message should not be empty for URL: %s", url)
			}
			return
		}

		// If creation succeeded, verify the URL is properly normalized
		normalizedURL := repoURL.String()

		// Basic invariants that should hold for all normalized URLs
		validateNormalizedRepositoryURL(t, url, normalizedURL)

		// Test that creating a RepositoryURL from the normalized URL produces the same result
		repoURL2, err2 := NewRepositoryURL(normalizedURL)
		if err2 != nil {
			t.Errorf(
				"Creating RepositoryURL from normalized URL should not fail: %s -> %s (error: %v)",
				url,
				normalizedURL,
				err2,
			)
			return
		}

		if repoURL.String() != repoURL2.String() {
			t.Errorf("Normalization should be idempotent: %s -> %s -> %s", url, normalizedURL, repoURL2.String())
		}
	})
}

// FuzzURLEquivalenceDetection tests that equivalent URLs are detected as such
// This test will FAIL initially as comprehensive normalization doesn't exist yet.
func FuzzURLEquivalenceDetection(f *testing.F) {
	// Seed with pairs of URLs that should be equivalent
	equivalentPairs := [][]string{
		{"https://github.com/user/repo", "https://github.com/user/repo.git"},
		{"https://github.com/user/repo", "https://GitHub.com/user/repo"},
		{"https://github.com/user/repo", "http://github.com/user/repo"},
		{"https://github.com/user/repo", "https://github.com/user/repo/"},
		{"https://github.com/user/repo", "https://github.com/user/repo?branch=main"},
		{"https://github.com/user/repo", "https://github.com/user/repo#readme"},
	}

	for _, pair := range equivalentPairs {
		f.Add(pair[0], pair[1])
	}

	f.Fuzz(func(t *testing.T, url1, url2 string) {
		// Validate input for fuzz testing
		if !isValidFuzzInput(url1, url2) {
			return
		}

		// Attempt to normalize both URLs
		normalized1, normalized2, err1, err2 := normalizeURLPair(url1, url2)

		// If both normalizations succeed, check for equivalence
		if err1 == nil && err2 == nil {
			if normalized1 == normalized2 {
				// URLs are equivalent - verify RepositoryURL objects are equal
				validateEquivalentURLs(t, url1, url2, normalized1, normalized2)
			} else {
				// URLs are not equivalent - verify RepositoryURL objects are different
				validateNonEquivalentURLs(t, url1, url2, normalized1, normalized2)
			}
		}
	})
}

// FuzzNormalizationPerformance tests performance characteristics of URL normalization
// This test will FAIL initially as the normalization function doesn't exist yet.
func FuzzNormalizationPerformance(f *testing.F) {
	// Seed with URLs of varying complexity
	seedURLs := []string{
		"https://github.com/user/repo",
		"https://github.com/user/repo.git",
		"https://github.com/user-name/repo-name.git",
		"https://github.com/organization/very-long-repository-name-with-many-hyphens.git",
		"HTTPS://GITHUB.COM/USER/REPO.GIT?BRANCH=MAIN&TAB=README#DOCUMENTATION",
	}

	for _, url := range seedURLs {
		f.Add(url)
	}

	f.Fuzz(func(t *testing.T, url string) {
		// Skip obviously invalid inputs
		if len(url) < 10 || containsControlCharacters(url) {
			return
		}

		// Only test with supported hosts to focus on performance
		if !containsSupportedHost(url) {
			return
		}

		// Measure normalization performance
		start := time.Now()

		// This function doesn't exist yet - test should FAIL
		normalized, err := normalization.NormalizeRepositoryURL(url)

		elapsed := time.Since(start)

		// Normalization should be fast (< 1ms for reasonable URLs)
		if elapsed > time.Millisecond {
			t.Errorf("Normalization took too long (%v) for URL: %s", elapsed, url)
		}

		// If normalization succeeded, result should be reasonable
		if err == nil && len(normalized) > 10000 {
			t.Errorf("Normalized URL is unreasonably long (%d characters) for input: %s", len(normalized), url)
		}
	})
}

// FuzzSecurityVulnerabilities tests for security issues in URL normalization
// This test will FAIL initially as comprehensive security validation doesn't exist yet.
func FuzzSecurityVulnerabilities(f *testing.F) {
	// Seed with potentially problematic URLs
	seedURLs := []string{
		"https://github.com/owner/../../../etc/passwd",
		"https://github.com/owner/repo%2E%2E%2F%2E%2E",
		"https://github.com/owner/repo\x00",
		"https://github.com/owner/repo\r\n\r\n",
		"https://github.com/owner/repo?param=value&param2=<script>",
		"javascript:alert(1)//github.com/owner/repo",
		"https://github.com/owner/repo';DROP TABLE repositories;--",
	}

	for _, url := range seedURLs {
		f.Add(url)
	}

	f.Fuzz(func(t *testing.T, url string) {
		// Test potentially malicious URLs
		// This function doesn't exist yet - test should FAIL
		normalized, err := normalization.NormalizeRepositoryURL(url)

		if err == nil {
			// If normalization succeeded, verify the result is safe
			validateSecurityOfNormalizedURL(t, url, normalized)
		}

		// Whether normalization succeeds or fails, it shouldn't panic
		// This is implicitly tested by the fuzzer - if the function panics, the test fails
	})
}

// Helper functions for fuzzing validation

// isValidFuzzInput validates that both URLs are suitable for fuzz testing.
func isValidFuzzInput(url1, url2 string) bool {
	// Skip obviously invalid inputs
	if len(url1) < 15 || len(url2) < 15 {
		return false
	}

	if containsControlCharacters(url1) || containsControlCharacters(url2) {
		return false
	}

	// Only test with supported hosts
	if !containsSupportedHost(url1) || !containsSupportedHost(url2) {
		return false
	}

	return true
}

// normalizeURLPair attempts to normalize both URLs and returns the results.
func normalizeURLPair(url1, url2 string) (string, string, error, error) {
	normalized1, err1 := normalization.NormalizeRepositoryURL(url1)
	normalized2, err2 := normalization.NormalizeRepositoryURL(url2)
	return normalized1, normalized2, err1, err2
}

// createAndCompareRepositoryURLs creates RepositoryURL objects and compares them for equality.
// Returns: areEqual, createErr1, createErr2
func createAndCompareRepositoryURLs(url1, url2 string) (bool, error, error) {
	repoURL1, createErr1 := NewRepositoryURL(url1)
	repoURL2, createErr2 := NewRepositoryURL(url2)

	if createErr1 != nil || createErr2 != nil {
		return false, createErr1, createErr2
	}

	return repoURL1.Equal(repoURL2), nil, nil
}

// validateEquivalentURLs tests that URLs with equivalent normalized forms create equal RepositoryURL objects.
func validateEquivalentURLs(t *testing.T, url1, url2, normalized1, normalized2 string) {
	areEqual, createErr1, createErr2 := createAndCompareRepositoryURLs(url1, url2)

	if createErr1 == nil && createErr2 == nil {
		if !areEqual {
			t.Errorf(
				"Equivalent URLs should create equal RepositoryURL objects:\nURL1: %s -> %s\nURL2: %s -> %s",
				url1,
				normalized1,
				url2,
				normalized2,
			)
		}
	}
}

// validateNonEquivalentURLs tests that URLs with different normalized forms create different RepositoryURL objects.
func validateNonEquivalentURLs(t *testing.T, url1, url2, normalized1, normalized2 string) {
	areEqual, createErr1, createErr2 := createAndCompareRepositoryURLs(url1, url2)

	if createErr1 == nil && createErr2 == nil {
		if areEqual {
			t.Errorf("Non-equivalent URLs should create different RepositoryURL objects:\nURL1: %s -> %s\nURL2: %s -> %s",
				url1, normalized1, url2, normalized2)
		}
	}
}

// containsControlCharacters checks if the string contains control characters.
func containsControlCharacters(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r' {
			return true
		}
	}
	return false
}

// containsSupportedHost checks if the URL contains a supported Git hosting provider.
func containsSupportedHost(url string) bool {
	url = strings.ToLower(url)
	return strings.Contains(url, "github.com") ||
		strings.Contains(url, "gitlab.com") ||
		strings.Contains(url, "bitbucket.org")
}

// validateNormalizedURL performs consistency checks on normalized URLs.
func validateNormalizedURL(t *testing.T, original, normalized string) {
	// Normalized URL should not be empty
	if len(normalized) == 0 {
		t.Errorf("Normalized URL should not be empty for input: %s", original)
		return
	}

	// Normalized URL should be a valid URL format
	if !strings.Contains(normalized, "://") {
		t.Errorf("Normalized URL should contain protocol for input: %s -> %s", original, normalized)
	}

	// Normalized URL should start with https (assuming we standardize to HTTPS)
	if !strings.HasPrefix(normalized, "https://") {
		t.Errorf("Normalized URL should start with https:// for input: %s -> %s", original, normalized)
	}

	// Normalized URL should not end with trailing slash (assuming we remove them)
	if strings.HasSuffix(normalized, "/") && !strings.HasSuffix(normalized, "://") {
		t.Errorf("Normalized URL should not end with trailing slash for input: %s -> %s", original, normalized)
	}

	// Normalized URL should not contain .git suffix (assuming we remove it)
	if strings.Contains(normalized, ".git") {
		t.Errorf("Normalized URL should not contain .git suffix for input: %s -> %s", original, normalized)
	}

	// Normalized URL should not contain query parameters (assuming we remove them)
	if strings.Contains(normalized, "?") {
		t.Errorf("Normalized URL should not contain query parameters for input: %s -> %s", original, normalized)
	}

	// Normalized URL should not contain fragment identifiers (assuming we remove them)
	if strings.Contains(normalized, "#") {
		t.Errorf("Normalized URL should not contain fragment identifiers for input: %s -> %s", original, normalized)
	}

	// Host should be lowercase
	parts := strings.Split(normalized, "/")
	if len(parts) >= 3 {
		host := parts[2]
		if host != strings.ToLower(host) {
			t.Errorf("Host should be lowercase in normalized URL for input: %s -> %s", original, normalized)
		}
	}
}

// validateNormalizedRepositoryURL performs validation specific to RepositoryURL objects.
func validateNormalizedRepositoryURL(t *testing.T, original, normalized string) {
	// First run the general validation
	validateNormalizedURL(t, original, normalized)

	// Additional RepositoryURL-specific validations
	if !strings.Contains(normalized, "github.com") &&
		!strings.Contains(normalized, "gitlab.com") &&
		!strings.Contains(normalized, "bitbucket.org") {
		t.Errorf("Normalized RepositoryURL should contain supported host for input: %s -> %s", original, normalized)
	}

	// Should have owner/repo structure
	parts := strings.Split(normalized, "/")
	if len(parts) < 5 { // https://host.com/owner/repo
		t.Errorf("Normalized RepositoryURL should have owner/repo structure for input: %s -> %s", original, normalized)
	}
}

// validateSecurityOfNormalizedURL checks for security issues in normalized URLs.
func validateSecurityOfNormalizedURL(t *testing.T, original, normalized string) {
	// Should not contain path traversal attempts
	if strings.Contains(normalized, "..") {
		t.Errorf("Normalized URL should not contain path traversal for input: %s -> %s", original, normalized)
	}

	// Should not contain encoded path traversal attempts
	if strings.Contains(normalized, "%2e%2e") || strings.Contains(normalized, "%2E%2E") {
		t.Errorf("Normalized URL should not contain encoded path traversal for input: %s -> %s", original, normalized)
	}

	// Should not contain control characters
	if containsControlCharacters(normalized) {
		t.Errorf("Normalized URL should not contain control characters for input: %s -> %s", original, normalized)
	}

	// Should not contain script tags or other HTML/JS
	lowerNormalized := strings.ToLower(normalized)
	if strings.Contains(lowerNormalized, "<script") || strings.Contains(lowerNormalized, "javascript:") {
		t.Errorf("Normalized URL should not contain script content for input: %s -> %s", original, normalized)
	}

	// Should not contain SQL injection attempts
	if strings.Contains(lowerNormalized, "drop table") ||
		strings.Contains(lowerNormalized, "'; drop") ||
		strings.Contains(lowerNormalized, "union select") {
		t.Errorf("Normalized URL should not contain SQL injection attempts for input: %s -> %s", original, normalized)
	}
}
