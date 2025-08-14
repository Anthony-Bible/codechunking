package common

import (
	"encoding/json"
	"strings"
	"testing"
)

// FuzzInputSanitization tests input sanitization with random malicious inputs.
func FuzzInputSanitization(f *testing.F) {
	// Seed corpus with known malicious patterns
	maliciousInputs := []string{
		"<script>alert('xss')</script>",
		"'; DROP TABLE repositories;--",
		"javascript:alert('xss')",
		"<img src='x' onerror='alert(1)'>",
		"data:text/html,<script>alert('xss')</script>",
		"repo\x00malicious",
		"repo\nmalicious",
		"repo\u202Emalicious",
		"reроsitory", // Cyrillic characters
		"../../../etc/passwd",
		"%2E%2E/%2E%2E/etc/passwd",
		strings.Repeat("a", 10000),
		"UNION SELECT * FROM users",
		"<iframe src='javascript:alert(1)'></iframe>",
	}

	for _, input := range maliciousInputs {
		f.Add(input)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// This fuzzing test should FAIL initially because comprehensive input sanitization is not implemented

		// Test repository name validation (this will currently fail for many malicious inputs)
		name := input
		err := ValidateRepositoryName(&name)

		if containsMaliciousInput(input) {
			if err == nil {
				t.Errorf("SECURITY FAIL: Malicious input was accepted in repository name: %q", input)
			}
		}

		// Test various input validation scenarios
		validateInputSafety(t, input)
	})
}

// FuzzJSONInputSanitization tests JSON input sanitization.
func FuzzJSONInputSanitization(f *testing.F) {
	// Seed with malicious JSON payloads
	maliciousJSONs := []string{
		`{"name": "<script>alert('xss')</script>"}`,
		`{"description": "'; DROP TABLE repositories;--"}`,
		`{"url": "javascript:alert('xss')"}`,
		`{"name": "repo\x00malicious"}`,
		`{"description": "<img onerror='alert(1)' src='x'>"}`,
		`{"url": "https://github.com/user\u202E/repo"}`,
		`{"name": "` + strings.Repeat("a", 1000) + `"}`,
	}

	for _, jsonStr := range maliciousJSONs {
		f.Add(jsonStr)
	}

	f.Fuzz(func(t *testing.T, jsonInput string) {
		// This test should FAIL initially - comprehensive JSON sanitization not implemented

		var data map[string]interface{}
		err := json.Unmarshal([]byte(jsonInput), &data)
		if err != nil {
			// Invalid JSON - should be rejected
			return
		}

		// Check if any field contains malicious content
		for key, value := range data {
			if strValue, ok := value.(string); ok {
				if containsMaliciousInput(strValue) {
					// This malicious content should be detected and rejected
					validationErr := ValidateJSONField(key, strValue)
					if validationErr == nil {
						t.Errorf(
							"SECURITY FAIL: Malicious content in JSON field %s was not detected: %q",
							key,
							strValue,
						)
					}
				}
			}
		}
	})
}

// FuzzQueryParameterSanitization tests query parameter sanitization.
func FuzzQueryParameterSanitization(f *testing.F) {
	// Seed with malicious query parameters
	maliciousQueries := []string{
		"'; DROP TABLE repositories;--",
		"<script>alert('xss')</script>",
		"UNION SELECT password FROM users",
		"repo' OR '1'='1",
		"../../../etc/passwd",
		"repo\x00malicious",
		strings.Repeat("a", 5000),
	}

	for _, query := range maliciousQueries {
		f.Add(query)
	}

	f.Fuzz(func(t *testing.T, queryParam string) {
		// This test should FAIL initially - query parameter sanitization not comprehensive

		// Test sort parameter validation
		if strings.Contains(queryParam, ":") {
			err := ValidateSortParameter(queryParam)
			if containsMaliciousInput(queryParam) && err == nil {
				t.Errorf("SECURITY FAIL: Malicious sort parameter was accepted: %q", queryParam)
			}
		}

		// Test status parameter validation
		err := ValidateRepositoryStatus(queryParam)
		if containsMaliciousInput(queryParam) && err == nil {
			t.Errorf("SECURITY FAIL: Malicious status parameter was accepted: %q", queryParam)
		}
	})
}

// FuzzUnicodeAttacks tests Unicode-based attacks.
func FuzzUnicodeAttacks(f *testing.F) {
	// Seed with Unicode attack vectors
	unicodeAttacks := []string{
		"repo\u202Emalicious", // Right-to-left override
		"repo\u202Dmalicious", // Left-to-right override
		"repo\u200Bmalicious", // Zero-width space
		"repo\u200Cmalicious", // Zero-width non-joiner
		"repo\u0300malicious", // Combining grave accent
		"reроsitory",          // Cyrillic homograph
		"gοοgle.com",          // Greek omicron instead of 'o'
		"раураl.com",          // Cyrillic 'а' instead of 'a'
		"ѕcript",              // Cyrillic 's'
		"аdmin",               // Cyrillic 'а'
	}

	for _, attack := range unicodeAttacks {
		f.Add(attack)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// This test should FAIL initially - Unicode attack detection not implemented

		name := input
		err := ValidateRepositoryName(&name)

		// Check for suspicious Unicode characters
		if containsSuspiciousUnicode(input) {
			if err == nil {
				t.Errorf("SECURITY FAIL: Unicode attack was not detected: %q", input)
			}
		}
	})
}

// FuzzControlCharacterAttacks tests control character injection.
func FuzzControlCharacterAttacks(f *testing.F) {
	// Seed with control character attacks
	controlAttacks := []string{
		"repo\x00malicious", // Null byte
		"repo\x01malicious", // Start of heading
		"repo\x02malicious", // Start of text
		"repo\x03malicious", // End of text
		"repo\x04malicious", // End of transmission
		"repo\x05malicious", // Enquiry
		"repo\x06malicious", // Acknowledge
		"repo\x07malicious", // Bell
		"repo\x08malicious", // Backspace
		"repo\x0Bmalicious", // Vertical tab
		"repo\x0Cmalicious", // Form feed
		"repo\x0Emalicious", // Shift out
		"repo\x0Fmalicious", // Shift in
		"repo\rmalicious",   // Carriage return
		"repo\nmalicious",   // Line feed
	}

	for _, attack := range controlAttacks {
		f.Add(attack)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// This test should FAIL initially - control character detection not comprehensive

		name := input
		err := ValidateRepositoryName(&name)

		// Check for control characters
		if containsControlCharacters(input) {
			if err == nil {
				t.Errorf("SECURITY FAIL: Control character attack was not detected: %q", input)
			}
		}
	})
}

// FuzzEncodingAttacks tests encoding-based attacks.
func FuzzEncodingAttacks(f *testing.F) {
	// Seed with encoding attack vectors
	encodingAttacks := []string{
		"repo%00malicious",                              // URL encoded null
		"repo%2E%2E%2Fmalicious",                        // URL encoded ../
		"repo%252E%252E%252Fmalicious",                  // Double URL encoded ../
		"repo%3Cscript%3Ealert%281%29%3C%2Fscript%3E",   // URL encoded script
		"repo%27%3B%20DROP%20TABLE%20repositories%3B",   // URL encoded SQL injection
		"repo\\x3cscript\\x3ealert(1)\\x3c/script\\x3e", // Hex encoded script
		"repo&#60;script&#62;alert(1)&#60;/script&#62;", // HTML entity encoded script
	}

	for _, attack := range encodingAttacks {
		f.Add(attack)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// This test should FAIL initially - encoding attack detection not implemented

		name := input
		err := ValidateRepositoryName(&name)

		// Check for suspicious encoding
		if containsSuspiciousEncoding(input) {
			if err == nil {
				t.Errorf("SECURITY FAIL: Encoding attack was not detected: %q", input)
			}
		}
	})
}

// Helper functions for fuzzing tests

func containsMaliciousInput(input string) bool {
	maliciousPatterns := []string{
		"<script", "</script>", "javascript:", "<img", "onerror=", "onclick=",
		"'; DROP", "DROP TABLE", "UNION SELECT", "INSERT INTO", "DELETE FROM",
		"../", "%2E%2E", "\x00", "\r", "\n", "\f", "\v", "\b",
	}

	inputLower := strings.ToLower(input)
	for _, pattern := range maliciousPatterns {
		if strings.Contains(inputLower, strings.ToLower(pattern)) {
			return true
		}
	}

	return containsSuspiciousUnicode(input) || len(input) > 2048
}

// containsSuspiciousUnicode is now implemented in validation.go

// containsControlCharacters is now implemented in validation.go

func containsSuspiciousEncoding(input string) bool {
	suspiciousPatterns := []string{
		"%00", "%2E%2E", "%252E", "%3C", "%3E", "%27", "%22", // URL encoding
		"\\x00", "\\x3c", "\\x3e", "\\x27", "\\x22", // Hex encoding
		"&#", "&lt;", "&gt;", "&quot;", "&apos;", // HTML entities
	}

	inputLower := strings.ToLower(input)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(inputLower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

func validateInputSafety(t *testing.T, input string) {
	// Validate various safety aspects of the input

	// Length check
	if len(input) > 10000 {
		t.Errorf("Input too long for safety: %d characters", len(input))
	}

	// Control character check
	if containsControlCharacters(input) {
		name := input
		err := ValidateRepositoryName(&name)
		if err == nil {
			t.Errorf("Control characters should be rejected: %q", input)
		}
	}

	// Unicode safety check
	if containsSuspiciousUnicode(input) {
		name := input
		err := ValidateRepositoryName(&name)
		if err == nil {
			t.Errorf("Suspicious Unicode should be rejected: %q", input)
		}
	}
}

// ValidateJSONField is now implemented in validation.go

// BenchmarkInputSanitizationFuzzing benchmarks fuzzing performance.
func BenchmarkInputSanitizationFuzzing(b *testing.B) {
	maliciousInputs := []string{
		"<script>alert('xss')</script>",
		"'; DROP TABLE repositories;--",
		"repo\x00malicious",
		strings.Repeat("a", 1000),
		"reроsitory", // Unicode attack
	}

	b.ResetTimer()
	for range b.N {
		for _, input := range maliciousInputs {
			// This benchmark should FAIL initially - sanitization not optimized
			name := input
			_ = ValidateRepositoryName(&name)
		}
	}
}
