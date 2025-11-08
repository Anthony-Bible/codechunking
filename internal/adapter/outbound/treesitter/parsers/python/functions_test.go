package pythonparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// RED PHASE TESTS: extractModuleNameFromDocstring Helper Function
// =============================================================================
// These tests define expected behavior for the extractModuleNameFromDocstring
// function which extracts module names from Python docstrings by parsing
// patterns like "module for [keyword]".
//
// CURRENT STATE: These tests WILL FAIL because the function does not exist yet.
//
// CONTEXT: The TestPythonParser_ExtractModules test expects the module name
// "utility" to be extracted from the docstring "This is a module for utility functions",
// but the current implementation in extractModuleName() returns hardcoded "models".
//
// EXPECTED IMPLEMENTATION: The function should use pattern matching to extract
// keywords from docstrings following common Python module documentation patterns.
// =============================================================================

// TestExtractModuleNameFromDocstring_BasicModuleForPattern tests the basic pattern.
// RED PHASE: This test defines the primary use case - extracting module name
// from "This is a module for utility functions" should return "utility".
func TestExtractModuleNameFromDocstring_BasicModuleForPattern(t *testing.T) {
	docstring := "This is a module for utility functions"

	// EXPECTED: "utility" - the keyword immediately after "module for"
	// ACTUAL: Function does not exist yet, test will fail to compile/run
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "utility", result,
		"Should extract 'utility' from 'module for utility functions' pattern")
}

// TestExtractModuleNameFromDocstring_DifferentWordOrder tests varied phrasing.
// RED PHASE: Tests that the pattern works with different word ordering like
// "A module for database operations" returning "database".
func TestExtractModuleNameFromDocstring_DifferentWordOrder(t *testing.T) {
	docstring := "A module for database operations"

	// EXPECTED: "database" - works with articles like "A"
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "database", result,
		"Should extract 'database' from 'A module for database operations' pattern")
}

// TestExtractModuleNameFromDocstring_NoMatch tests when pattern is not found.
// RED PHASE: Defines behavior when docstring doesn't contain the "module for" pattern.
// Should return empty string when no recognizable pattern is found.
func TestExtractModuleNameFromDocstring_NoMatch(t *testing.T) {
	docstring := "Just some random text"

	// EXPECTED: "" - no "module for" pattern found
	result := extractModuleNameFromDocstring(docstring)

	assert.Empty(t, result,
		"Should return empty string when no 'module for' pattern is found")
}

// TestExtractModuleNameFromDocstring_EmptyString tests empty docstring.
// RED PHASE: Edge case for empty input - should return empty string.
func TestExtractModuleNameFromDocstring_EmptyString(t *testing.T) {
	docstring := ""

	// EXPECTED: "" - empty input should return empty output
	result := extractModuleNameFromDocstring(docstring)

	assert.Empty(t, result,
		"Should return empty string for empty docstring")
}

// TestExtractModuleNameFromDocstring_OnlyForWithoutModule tests "for" without "module".
// RED PHASE: Tests that the function requires both "module" and "for" keywords.
// "This is for testing" should NOT match because "module" is missing.
func TestExtractModuleNameFromDocstring_OnlyForWithoutModule(t *testing.T) {
	docstring := "This is for testing"

	// EXPECTED: "" - missing "module" keyword, so pattern doesn't match
	result := extractModuleNameFromDocstring(docstring)

	assert.Empty(t, result,
		"Should return empty string when 'module' keyword is missing")
}

// TestExtractModuleNameFromDocstring_MultipleWordsAfterFor tests extraction of first word only.
// RED PHASE: When the pattern is "module for user authentication", should extract
// only the first keyword "user", not "user authentication".
func TestExtractModuleNameFromDocstring_MultipleWordsAfterFor(t *testing.T) {
	docstring := "This is a module for user authentication"

	// EXPECTED: "user" - extract only the first word after "for"
	// This matches typical noun-based module naming (e.g., "user module")
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "user", result,
		"Should extract only first word 'user' from 'module for user authentication'")
}

// TestExtractModuleNameFromDocstring_CaseInsensitivity tests case handling.
// RED PHASE: The pattern matching should be case-insensitive for "module" and "for",
// but preserve the original case of the extracted keyword.
// "Module For TESTING functions" should return "TESTING" (preserving original case).
func TestExtractModuleNameFromDocstring_CaseInsensitivity(t *testing.T) {
	docstring := "Module For TESTING functions"

	// EXPECTED: "TESTING" - case-insensitive pattern match, preserve keyword case
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "TESTING", result,
		"Should match pattern case-insensitively but preserve extracted keyword case")
}

// TestExtractModuleNameFromDocstring_WithPunctuation tests keyword followed by punctuation.
// RED PHASE: Tests extraction when the keyword is followed by punctuation like a period.
// "This is a module for testing." should return "testing" (punctuation stripped).
func TestExtractModuleNameFromDocstring_WithPunctuation(t *testing.T) {
	docstring := "This is a module for testing."

	// EXPECTED: "testing" - punctuation should be stripped from extracted keyword
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "testing", result,
		"Should extract 'testing' and strip trailing punctuation")
}

// TestExtractModuleNameFromDocstring_WithComma tests keyword followed by comma.
// RED PHASE: Tests that commas are properly handled.
// "A module for parsing, processing and validation" should return "parsing".
func TestExtractModuleNameFromDocstring_WithComma(t *testing.T) {
	docstring := "A module for parsing, processing and validation"

	// EXPECTED: "parsing" - extract first keyword, strip comma
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "parsing", result,
		"Should extract 'parsing' and strip trailing comma")
}

// TestExtractModuleNameFromDocstring_MultilineDocstring tests multiline input.
// RED PHASE: Tests extraction from multiline docstrings where the pattern
// appears on the first line.
func TestExtractModuleNameFromDocstring_MultilineDocstring(t *testing.T) {
	docstring := `This is a module for authentication functions.

It provides user login and session management.
Additional details here.`

	// EXPECTED: "authentication" - should find pattern in first line
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "authentication", result,
		"Should extract from multiline docstring")
}

// TestExtractModuleNameFromDocstring_PatternInMiddle tests pattern not at start.
// RED PHASE: Tests that the pattern can appear anywhere in the docstring,
// not just at the beginning.
func TestExtractModuleNameFromDocstring_PatternInMiddle(t *testing.T) {
	docstring := "Welcome! This is a module for configuration management."

	// EXPECTED: "configuration" - pattern can appear after other text
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "configuration", result,
		"Should find pattern even when not at start of docstring")
}

// TestExtractModuleNameFromDocstring_MultipleModuleForPatterns tests first match priority.
// RED PHASE: When multiple "module for" patterns exist, should return the first match.
// "This is a module for testing and a module for validation" should return "testing".
func TestExtractModuleNameFromDocstring_MultipleModuleForPatterns(t *testing.T) {
	docstring := "This is a module for testing and a module for validation"

	// EXPECTED: "testing" - use first match when multiple patterns exist
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "testing", result,
		"Should return first match when multiple 'module for' patterns exist")
}

// TestExtractModuleNameFromDocstring_WithExtraWhitespace tests whitespace handling.
// RED PHASE: Tests that extra whitespace is properly handled.
// "module  for   parsing" (multiple spaces) should still extract "parsing".
func TestExtractModuleNameFromDocstring_WithExtraWhitespace(t *testing.T) {
	docstring := "This is a  module  for   parsing   operations"

	// EXPECTED: "parsing" - handle multiple spaces gracefully
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "parsing", result,
		"Should handle extra whitespace in pattern")
}

// TestExtractModuleNameFromDocstring_LowercaseModule tests lowercase "module".
// RED PHASE: Tests the common case where "module" is lowercase.
// "This is a module for testing" should work just as well as "Module".
func TestExtractModuleNameFromDocstring_LowercaseModule(t *testing.T) {
	docstring := "This is a module for testing"

	// EXPECTED: "testing"
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "testing", result,
		"Should work with lowercase 'module' keyword")
}

// TestExtractModuleNameFromDocstring_UppercaseModule tests uppercase "MODULE".
// RED PHASE: Tests case-insensitive matching with uppercase.
// "This is a MODULE for testing" should also extract "testing".
func TestExtractModuleNameFromDocstring_UppercaseModule(t *testing.T) {
	docstring := "This is a MODULE for testing"

	// EXPECTED: "testing" - case-insensitive pattern matching
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "testing", result,
		"Should work with uppercase 'MODULE' keyword")
}

// TestExtractModuleNameFromDocstring_MixedCaseFor tests mixed case "For".
// RED PHASE: Tests case-insensitive matching for the "for" keyword.
// "module For testing" should extract "testing".
func TestExtractModuleNameFromDocstring_MixedCaseFor(t *testing.T) {
	docstring := "This is a module For testing"

	// EXPECTED: "testing" - "For" with capital F should still match
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "testing", result,
		"Should work with mixed case 'For' keyword")
}

// TestExtractModuleNameFromDocstring_NumberInKeyword tests keyword with number.
// RED PHASE: Tests extraction of keywords that contain numbers.
// "module for oauth2 authentication" should extract "oauth2".
func TestExtractModuleNameFromDocstring_NumberInKeyword(t *testing.T) {
	docstring := "This is a module for oauth2 authentication"

	// EXPECTED: "oauth2" - alphanumeric keywords should be extracted
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "oauth2", result,
		"Should extract alphanumeric keywords like 'oauth2'")
}

// TestExtractModuleNameFromDocstring_UnderscoreInKeyword tests keyword with underscore.
// RED PHASE: Tests extraction of snake_case module names.
// "module for user_management functions" should extract "user_management".
func TestExtractModuleNameFromDocstring_UnderscoreInKeyword(t *testing.T) {
	docstring := "This is a module for user_management functions"

	// EXPECTED: "user_management" - preserve underscores in module names
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "user_management", result,
		"Should extract keywords with underscores like 'user_management'")
}

// TestExtractModuleNameFromDocstring_HyphenInKeyword tests keyword with hyphen.
// RED PHASE: Tests extraction of hyphenated module names.
// "module for test-utils" should extract "test-utils" or just "test".
// Decision: Extract only up to special characters (so "test").
func TestExtractModuleNameFromDocstring_HyphenInKeyword(t *testing.T) {
	docstring := "This is a module for test-utils"

	// EXPECTED: "test" - hyphens act as word boundaries
	// NOTE: This is a design decision - hyphens are less common in Python module names
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "test", result,
		"Should extract up to hyphen as word boundary")
}

// TestExtractModuleNameFromDocstring_StartingWithModuleFor tests direct pattern.
// RED PHASE: Tests when docstring starts directly with "module for".
// "module for testing" (no prefix) should extract "testing".
func TestExtractModuleNameFromDocstring_StartingWithModuleFor(t *testing.T) {
	docstring := "module for testing"

	// EXPECTED: "testing"
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "testing", result,
		"Should work when docstring starts with 'module for'")
}

// TestExtractModuleNameFromDocstring_OnlyModuleFor tests incomplete pattern.
// RED PHASE: Tests edge case where input is just "module for" with no keyword.
// Should return empty string because there's no keyword to extract.
func TestExtractModuleNameFromDocstring_OnlyModuleFor(t *testing.T) {
	docstring := "This is a module for"

	// EXPECTED: "" - no keyword after "for"
	result := extractModuleNameFromDocstring(docstring)

	assert.Empty(t, result,
		"Should return empty string when no keyword follows 'module for'")
}

// TestExtractModuleNameFromDocstring_WithNewlineAfterFor tests newline handling.
// RED PHASE: Tests extraction when keyword appears on next line after "for".
// "module for\nutility" - depends on implementation, likely should not match
// across newlines in simple implementation.
func TestExtractModuleNameFromDocstring_WithNewlineAfterFor(t *testing.T) {
	docstring := "This is a module for\nutility functions"

	// EXPECTED: "" - simple implementation should not match across newlines
	// NOTE: More sophisticated implementation might handle this, but not required
	result := extractModuleNameFromDocstring(docstring)

	assert.Empty(t, result,
		"Should not match pattern across newlines in simple implementation")
}

// TestExtractModuleNameFromDocstring_KeywordOnlyDigits tests all-numeric keyword.
// RED PHASE: Tests extraction when keyword is only digits.
// "module for 2fa" should extract "2fa" if we allow leading digits.
func TestExtractModuleNameFromDocstring_KeywordOnlyDigits(t *testing.T) {
	docstring := "This is a module for 2fa authentication"

	// EXPECTED: "2fa" - allow keywords starting with digits
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "2fa", result,
		"Should extract keywords starting with digits like '2fa'")
}

// TestExtractModuleNameFromDocstring_SpecialCharsInDocstring tests special characters.
// RED PHASE: Tests that special characters in docstring don't break extraction.
// "This is a module (for testing) operations" - parentheses should not interfere.
func TestExtractModuleNameFromDocstring_SpecialCharsInDocstring(t *testing.T) {
	docstring := "This is a module (for testing) operations"

	// EXPECTED: "testing" - should still find pattern inside parentheses
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "testing", result,
		"Should extract correctly even with special characters in docstring")
}

// TestExtractModuleNameFromDocstring_RealWorldExample1 tests real-world docstring.
// RED PHASE: Tests with actual Python module docstring pattern.
func TestExtractModuleNameFromDocstring_RealWorldExample1(t *testing.T) {
	docstring := `
This module provides utility functions for string manipulation.

It includes various helper methods for common string operations.
`

	// EXPECTED: "utility" - extract from natural language description
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "utility", result,
		"Should extract 'utility' from real-world module docstring")
}

// TestExtractModuleNameFromDocstring_RealWorldExample2 tests alternative phrasing.
// RED PHASE: Tests "module for" in more formal documentation style.
func TestExtractModuleNameFromDocstring_RealWorldExample2(t *testing.T) {
	docstring := "Database Module for PostgreSQL Operations"

	// EXPECTED: "PostgreSQL" - extract keyword even in title case
	result := extractModuleNameFromDocstring(docstring)

	assert.Equal(t, "PostgreSQL", result,
		"Should extract 'PostgreSQL' from title-case documentation")
}

// TestExtractModuleNameFromDocstring_NoModuleKeyword tests "for" without "module".
// RED PHASE: Tests that "for" alone is not sufficient.
// "This is for testing purposes" should NOT match.
func TestExtractModuleNameFromDocstring_NoModuleKeyword(t *testing.T) {
	docstring := "This is for testing purposes"

	// EXPECTED: "" - "module" keyword is required
	result := extractModuleNameFromDocstring(docstring)

	assert.Empty(t, result,
		"Should not match when 'module' keyword is absent")
}

// TestExtractModuleNameFromDocstring_ModuleNotBeforeFor tests wrong order.
// RED PHASE: Tests that word order matters.
// "This is for module testing" should NOT match (wrong order).
func TestExtractModuleNameFromDocstring_ModuleNotBeforeFor(t *testing.T) {
	docstring := "This is for module testing"

	// EXPECTED: "" - "module" must come before "for"
	result := extractModuleNameFromDocstring(docstring)

	assert.Empty(t, result,
		"Should not match when 'for' comes before 'module'")
}

// TestExtractModuleNameFromDocstring_Table tests table-driven approach.
// RED PHASE: Comprehensive table-driven test covering all major scenarios.
func TestExtractModuleNameFromDocstring_Table(t *testing.T) {
	tests := []struct {
		name      string
		docstring string
		expected  string
		reason    string
	}{
		{
			name:      "basic pattern",
			docstring: "This is a module for utility functions",
			expected:  "utility",
			reason:    "Primary use case - extract keyword after 'module for'",
		},
		{
			name:      "different word order",
			docstring: "A module for database operations",
			expected:  "database",
			reason:    "Should work with different articles",
		},
		{
			name:      "no match",
			docstring: "Just some random text",
			expected:  "",
			reason:    "No pattern found",
		},
		{
			name:      "empty string",
			docstring: "",
			expected:  "",
			reason:    "Empty input",
		},
		{
			name:      "only 'for' without 'module'",
			docstring: "This is for testing",
			expected:  "",
			reason:    "Missing 'module' keyword",
		},
		{
			name:      "multiple words after 'for'",
			docstring: "This is a module for user authentication",
			expected:  "user",
			reason:    "Extract only first word",
		},
		{
			name:      "case insensitive pattern",
			docstring: "Module For TESTING functions",
			expected:  "TESTING",
			reason:    "Case-insensitive match, preserve keyword case",
		},
		{
			name:      "with period",
			docstring: "This is a module for testing.",
			expected:  "testing",
			reason:    "Strip trailing punctuation",
		},
		{
			name:      "with comma",
			docstring: "A module for parsing, processing and validation",
			expected:  "parsing",
			reason:    "Extract first keyword, strip comma",
		},
		{
			name:      "multiline docstring",
			docstring: "This is a module for authentication functions.\n\nProvides login.",
			expected:  "authentication",
			reason:    "Find pattern in multiline text",
		},
		{
			name:      "pattern in middle",
			docstring: "Welcome! This is a module for configuration management.",
			expected:  "configuration",
			reason:    "Pattern can appear after other text",
		},
		{
			name:      "lowercase module",
			docstring: "This is a module for testing",
			expected:  "testing",
			reason:    "Lowercase 'module' keyword",
		},
		{
			name:      "uppercase MODULE",
			docstring: "This is a MODULE for testing",
			expected:  "testing",
			reason:    "Uppercase 'MODULE' keyword",
		},
		{
			name:      "number in keyword",
			docstring: "This is a module for oauth2 authentication",
			expected:  "oauth2",
			reason:    "Alphanumeric keywords",
		},
		{
			name:      "underscore in keyword",
			docstring: "This is a module for user_management functions",
			expected:  "user_management",
			reason:    "Snake_case module names",
		},
		{
			name:      "starts with 'module for'",
			docstring: "module for testing",
			expected:  "testing",
			reason:    "Direct pattern at start",
		},
		{
			name:      "only 'module for'",
			docstring: "This is a module for",
			expected:  "",
			reason:    "No keyword after 'for'",
		},
		{
			name:      "keyword with digits prefix",
			docstring: "This is a module for 2fa authentication",
			expected:  "2fa",
			reason:    "Keywords starting with digits",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractModuleNameFromDocstring(tt.docstring)
			assert.Equal(t, tt.expected, result, tt.reason)
		})
	}
}
