package valueobject

import (
	"strings"
	"testing"
	"unicode"
)

// FuzzRepositoryURL tests URL validation with random inputs to find edge cases.
func FuzzRepositoryURL(f *testing.F) {
	// Seed corpus with known patterns that should be handled securely
	testInputs := []string{
		"https://github.com/user/repo",
		"javascript://github.com/user/repo",
		"file:///etc/passwd",
		"https://github.com/../../../etc/passwd",
		"https://github.com:8080/user/repo",
		"https://github.com@malicious.com/user/repo",
		"https://github.com/user%2Fmalicious/repo",
		"https://github.com/user'; DROP TABLE repositories;--/repo",
		"https://github.com/user/<script>alert('xss')</script>/repo",
		"https://github.com/user\x00/repo",
		"https://github.com/user\n/repo",
		"https://github.com/user/reρo", // Greek rho
		strings.Repeat("a", 10000),
		"data:text/html,<script>alert('xss')</script>",
	}

	for _, input := range testInputs {
		f.Add(input)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// This fuzzing test should FAIL initially because comprehensive security validation is not implemented
		url, err := NewRepositoryURL(input)

		// Security validation should reject dangerous inputs
		if containsMaliciousContent(input) {
			if err == nil {
				t.Errorf("SECURITY FAIL: Malicious URL was accepted: %s", input)
			}
			return
		}

		// If URL was accepted, it should be safe and well-formed
		if err == nil {
			validateAcceptedURL(t, url, input)
		}
	})
}

// containsMaliciousContent checks if input contains patterns that should be rejected.
func containsMaliciousContent(input string) bool {
	maliciousPatterns := []string{
		"javascript:",
		"file:",
		"data:",
		"ftp:",
		"<script",
		"</script>",
		"'; DROP",
		"DROP TABLE",
		"SELECT *",
		"INSERT INTO",
		"DELETE FROM",
		"UNION SELECT",
		"../",
		"%2E%2E",
		"\x00", // null byte
		"\r",   // carriage return
		"\n",   // newline
		"\t",   // tab
	}

	inputLower := strings.ToLower(input)
	for _, pattern := range maliciousPatterns {
		if strings.Contains(inputLower, strings.ToLower(pattern)) {
			return true
		}
	}

	// Check for suspicious unicode characters
	for _, r := range input {
		if unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc) { // Nonspacing/enclosing/combining marks
			return true
		}
		if r > 127 && !unicode.IsLetter(r) && !unicode.IsDigit(r) { // Suspicious non-ASCII
			return true
		}
	}

	// Check for extremely long inputs (potential DOS)
	if len(input) > 2048 {
		return true
	}

	// Check for non-standard ports
	if strings.Contains(input, ":") && !strings.HasPrefix(input, "https://") && !strings.HasPrefix(input, "http://") {
		return true
	}

	// Check for @ symbol (potential redirect/phishing)
	if strings.Contains(input, "@") {
		return true
	}

	return false
}

// validateAcceptedURL ensures accepted URLs meet security standards.
func validateAcceptedURL(t *testing.T, url RepositoryURL, _ string) {
	urlStr := url.String()

	// URL should not contain dangerous characters
	dangerousChars := []string{"\x00", "\r", "\n", "\t", "<", ">", "'", "\"", ";"}
	for _, char := range dangerousChars {
		if strings.Contains(urlStr, char) {
			t.Errorf("SECURITY FAIL: Accepted URL contains dangerous character %q: %s", char, urlStr)
		}
	}

	// URL should have proper length limits
	if len(urlStr) > 2048 {
		t.Errorf("SECURITY FAIL: Accepted URL exceeds length limit: %d characters", len(urlStr))
	}

	// URL should only use safe protocols
	if !strings.HasPrefix(urlStr, "https://") && !strings.HasPrefix(urlStr, "http://") {
		t.Errorf("SECURITY FAIL: Accepted URL uses unsafe protocol: %s", urlStr)
	}

	// URL should not contain encoded path traversal
	if strings.Contains(urlStr, "%2E%2E") || strings.Contains(urlStr, "..") {
		t.Errorf("SECURITY FAIL: Accepted URL contains path traversal: %s", urlStr)
	}
}

// FuzzRepositoryURLValidation focuses specifically on the validation logic.
func FuzzRepositoryURLValidation(f *testing.F) {
	// Test edge cases in validation
	f.Add("https://github.com/user/repo")
	f.Add("")                        // empty
	f.Add("a")                       // single char
	f.Add(strings.Repeat("x", 1000)) // long string

	f.Fuzz(func(t *testing.T, input string) {
		// This test should FAIL initially - comprehensive validation not implemented
		_, err := NewRepositoryURL(input)

		// Empty input should always be rejected
		if input == "" {
			if err == nil {
				t.Error("FAIL: Empty input should be rejected")
			}
			return
		}

		// Very long inputs should be rejected
		if len(input) > 2048 {
			if err == nil {
				t.Errorf("FAIL: Overly long input should be rejected (length: %d)", len(input))
			}
			return
		}

		// Control characters should be rejected
		for _, r := range input {
			if unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r' {
				if err == nil {
					t.Errorf("FAIL: Input with control character should be rejected: %q", r)
				}
				return
			}
		}
	})
}

// FuzzHostValidation tests host validation specifically.
func FuzzHostValidation(f *testing.F) {
	f.Add("github.com")
	f.Add("gitlab.com")
	f.Add("bitbucket.org")
	f.Add("malicious.com")
	f.Add("github.com.evil.com")
	f.Add("github.com@evil.com")

	f.Fuzz(func(t *testing.T, host string) {
		// This test should FAIL initially - host validation not comprehensive enough
		testURL := "https://" + host + "/user/repo"
		_, err := NewRepositoryURL(testURL)

		validHosts := []string{"github.com", "gitlab.com", "bitbucket.org"}
		isValidHost := false
		for _, validHost := range validHosts {
			if host == validHost {
				isValidHost = true
				break
			}
		}

		if !isValidHost {
			if err == nil {
				t.Errorf("FAIL: Invalid host should be rejected: %s", host)
			}
		}

		// Hosts with suspicious characters should be rejected
		suspiciousChars := []string{"@", "..", "/", "\\", " ", "\t", "\n"}
		for _, char := range suspiciousChars {
			if strings.Contains(host, char) {
				if err == nil {
					t.Errorf("FAIL: Host with suspicious character %q should be rejected: %s", char, host)
				}
			}
		}
	})
}

// BenchmarkRepositoryURLValidation benchmarks URL validation performance.
func BenchmarkRepositoryURLValidation(b *testing.B) {
	testURLs := []string{
		"https://github.com/user/repo",
		"https://gitlab.com/org/project",
		"https://bitbucket.org/team/repository",
		"invalid-url",
		"javascript://github.com/user/repo",
		"https://github.com/user/repo" + strings.Repeat("x", 1000),
	}

	b.ResetTimer()
	for range b.N {
		for _, url := range testURLs {
			_, _ = NewRepositoryURL(url)
		}
	}
}

// BenchmarkSecurityValidation benchmarks security validation performance.
func BenchmarkSecurityValidation(b *testing.B) {
	maliciousURLs := []string{
		"javascript://github.com/user/repo",
		"file:///etc/passwd",
		"https://github.com/../../../etc/passwd",
		"https://github.com/user'; DROP TABLE repositories;--/repo",
		"https://github.com/user/<script>alert('xss')</script>/repo",
		"https://github.com/user\x00malicious/repo",
		"https://github.com@malicious.com/user/repo",
	}

	b.ResetTimer()
	for range b.N {
		for _, url := range maliciousURLs {
			// This benchmark should FAIL initially - security validation not optimized
			_, _ = NewRepositoryURL(url)
		}
	}
}
