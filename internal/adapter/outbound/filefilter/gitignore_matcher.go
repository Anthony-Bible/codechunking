package filefilter

import (
	"path/filepath"
	"regexp"
	"strings"
)

// GitignoreMatcher provides advanced gitignore pattern matching capabilities.
// This implements proper gitignore semantics according to the Git specification.
type GitignoreMatcher struct {
	// Cache compiled patterns for performance
	compiledPatterns map[string]*regexp.Regexp
}

// NewGitignoreMatcher creates a new advanced gitignore matcher.
func NewGitignoreMatcher() *GitignoreMatcher {
	return &GitignoreMatcher{
		compiledPatterns: make(map[string]*regexp.Regexp),
	}
}

// MatchPattern matches a gitignore pattern against a file path.
// This follows the gitignore specification for proper pattern matching.
func (m *GitignoreMatcher) MatchPattern(pattern, path string) bool {
	// Normalize the path
	path = strings.TrimPrefix(path, "./")
	path = filepath.ToSlash(path)

	// Handle empty pattern
	if pattern == "" {
		return false
	}

	// Handle negation (should be handled by caller, but just in case)
	pattern = strings.TrimPrefix(pattern, "!")

	// Convert gitignore pattern to regex and match
	return m.matchWithRegex(pattern, path)
}

// matchWithRegex converts a gitignore pattern to regex and matches against path.
func (m *GitignoreMatcher) matchWithRegex(pattern, path string) bool {
	// Check if we have a cached compiled pattern
	if regex, exists := m.compiledPatterns[pattern]; exists {
		return regex.MatchString(path)
	}

	// Convert gitignore pattern to regex
	regexPattern := m.gitignoreToRegex(pattern)

	// Compile and cache the regex
	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		// If regex compilation fails, fall back to simple matching
		return m.fallbackMatch(pattern, path)
	}

	// Cache the compiled pattern
	m.compiledPatterns[pattern] = regex

	return regex.MatchString(path)
}

// gitignoreToRegex converts a gitignore pattern to a regular expression.
func (m *GitignoreMatcher) gitignoreToRegex(pattern string) string {
	// Handle rooted patterns (starting with /)
	isRooted := strings.HasPrefix(pattern, "/")
	if isRooted {
		pattern = strings.TrimPrefix(pattern, "/")
	}

	// Handle directory patterns (ending with /)
	isDirectory := strings.HasSuffix(pattern, "/")
	if isDirectory {
		pattern = strings.TrimSuffix(pattern, "/")
	}

	// Escape special regex characters before processing wildcards
	regex := m.escapeRegexChars(pattern)

	// Handle gitignore wildcard patterns
	regex = m.convertWildcards(regex)

	// Build the final regex based on pattern type
	var finalRegex string

	if isRooted {
		// Rooted patterns match from the beginning
		finalRegex = "^" + regex
	} else {
		// Non-rooted patterns can match at any level
		finalRegex = "(^|/)" + regex
	}

	// Add appropriate ending - both patterns need the same ending for gitignore semantics
	finalRegex += "($|/.*)"

	return finalRegex
}

// escapeRegexChars escapes special regex characters except wildcards.
func (m *GitignoreMatcher) escapeRegexChars(s string) string {
	// Characters that need escaping in regex (excluding * and ? which we handle separately)
	chars := []string{".", "+", "(", ")", "{", "}", "^", "$", "|", "\\"}

	for _, char := range chars {
		s = strings.ReplaceAll(s, char, "\\"+char)
	}

	return s
}

// convertWildcards converts gitignore wildcards to regex equivalents.
func (m *GitignoreMatcher) convertWildcards(s string) string {
	// Handle different wildcard patterns

	// 1. Handle **/ (matches zero or more directories)
	s = regexp.MustCompile(`\*\*/`).ReplaceAllString(s, `([^/]*/)*`)

	// 2. Handle /** (matches everything inside a directory)
	s = regexp.MustCompile(`/\*\*`).ReplaceAllString(s, `/.*`)

	// 3. Handle ** in the middle (matches zero or more path segments)
	s = regexp.MustCompile(`\*\*`).ReplaceAllString(s, `.*`)

	// 4. Handle bracket expressions like [cod] or [0-9]
	// These are already valid regex, but we need to be careful not to double-escape

	// 5. Handle single * (matches anything except slash)
	s = strings.ReplaceAll(s, "*", `[^/]*`)

	// 6. Handle ? (matches single character except slash)
	s = strings.ReplaceAll(s, "?", `[^/]`)

	return s
}

// fallbackMatch provides simple pattern matching when regex compilation fails.
func (m *GitignoreMatcher) fallbackMatch(pattern, path string) bool {
	// Remove leading slash for rooted patterns
	isRooted := strings.HasPrefix(pattern, "/")
	if isRooted {
		pattern = strings.TrimPrefix(pattern, "/")
	}

	// Handle directory patterns
	if strings.HasSuffix(pattern, "/") {
		pattern = strings.TrimSuffix(pattern, "/")
		return strings.HasPrefix(path, pattern+"/") || path == pattern
	}

	// Simple glob matching
	if strings.Contains(pattern, "*") {
		return m.simpleGlobMatch(pattern, path, isRooted)
	}

	// Exact matching
	if isRooted {
		return path == pattern || strings.HasPrefix(path, pattern+"/")
	}

	return path == pattern || strings.HasSuffix(path, "/"+pattern) ||
		strings.Contains(path, "/"+pattern+"/")
}

// simpleGlobMatch provides basic glob matching for fallback.
func (m *GitignoreMatcher) simpleGlobMatch(pattern, path string, isRooted bool) bool {
	// Handle *.ext patterns
	if strings.HasPrefix(pattern, "*.") {
		ext := pattern[1:] // Remove *
		return strings.HasSuffix(path, ext)
	}

	// Handle **/ patterns
	if strings.Contains(pattern, "**/") {
		return m.handleGlobstarPattern(pattern, path, isRooted)
	}

	// Basic wildcard replacement
	pattern = strings.ReplaceAll(pattern, "*", ".*")
	regex, err := regexp.Compile(pattern)
	if err == nil {
		return regex.MatchString(path)
	}

	// Last resort: contains check
	patternWithoutStars := strings.ReplaceAll(pattern, "*", "")
	return strings.Contains(path, patternWithoutStars)
}

// MatchBracketPattern handles bracket expressions like *.py[cod].
func (m *GitignoreMatcher) MatchBracketPattern(pattern, path string) bool {
	// Convert bracket pattern to regex
	// *.py[cod] becomes .*\.py[cod]
	regexPattern := strings.ReplaceAll(pattern, "*", ".*")

	// Escape dots
	regexPattern = strings.ReplaceAll(regexPattern, ".", "\\.")

	// Add anchors
	regexPattern = "(^|/)" + regexPattern + "$"

	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return false
	}

	return regex.MatchString(path)
}

// ClearCache clears the compiled pattern cache.
func (m *GitignoreMatcher) ClearCache() {
	m.compiledPatterns = make(map[string]*regexp.Regexp)
}

// GetCacheSize returns the number of cached patterns.
func (m *GitignoreMatcher) GetCacheSize() int {
	return len(m.compiledPatterns)
}

// handleGlobstarPattern handles **/ pattern matching extracted from simpleGlobMatch.
func (m *GitignoreMatcher) handleGlobstarPattern(pattern, path string, isRooted bool) bool {
	parts := strings.Split(pattern, "**/")
	if len(parts) != 2 {
		return false
	}

	prefix := parts[0]
	suffix := parts[1]

	if prefix == "" {
		// **/ at start - match suffix anywhere
		return strings.Contains(path, suffix) || strings.HasSuffix(path, suffix)
	}

	// Check if path starts with prefix and contains suffix
	if !strings.HasPrefix(path, prefix) && (isRooted || !strings.Contains(path, prefix)) {
		return false
	}

	remainder := path
	if idx := strings.Index(path, prefix); idx >= 0 {
		remainder = path[idx+len(prefix):]
	}
	return strings.Contains(remainder, suffix) || strings.HasSuffix(remainder, suffix)
}
