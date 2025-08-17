package security

import (
	"regexp"
)

// Patterns holds all pre-compiled security validation patterns.
type Patterns struct {
	// Git repository patterns
	GitURL *regexp.Regexp

	// XSS attack patterns
	XSSScript   *regexp.Regexp
	XSSEvent    *regexp.Regexp
	XSSProtocol *regexp.Regexp
	XSSEntity   *regexp.Regexp

	// SQL injection patterns
	SQLKeywords *regexp.Regexp
	SQLComment  *regexp.Regexp
	SQLQuotes   *regexp.Regexp
	SQLUnion    *regexp.Regexp

	// Path traversal patterns
	PathTraversal *regexp.Regexp
	PathEncoded   *regexp.Regexp

	// URL encoding patterns
	URLEncoded *regexp.Regexp

	// Control characters pattern
	ControlChars *regexp.Regexp
}

// GetPatterns creates and returns a new instance of compiled patterns.
// This replaces the singleton pattern to avoid global state.
func GetPatterns() *Patterns {
	return compilePatterns()
}

// compilePatterns compiles all security patterns once at startup.
func compilePatterns() *Patterns {
	p := &Patterns{}

	// Git repository URL pattern
	p.GitURL = regexp.MustCompile(`^https?://(github\.com|gitlab\.com|bitbucket\.org)/.+/.+$`)

	// XSS patterns - consolidated and optimized
	p.XSSScript = regexp.MustCompile(`(?i)<\s*/?script[^>]*>|javascript\s*:|\bdata\s*:\s*text/html`)
	p.XSSEvent = regexp.MustCompile(`(?i)\bon\w+\s*=|<\s*(img|svg|iframe|div)[^>]*\son\w+`)
	p.XSSProtocol = regexp.MustCompile(`(?i)^\s*(javascript|data|vbscript)\s*:`)
	p.XSSEntity = regexp.MustCompile(`(?i)&(lt|gt|quot|apos|#\d+|#x[0-9a-f]+);|\\x[0-9a-f]{2}`)

	// SQL injection patterns - optimized for common attacks
	p.SQLKeywords = regexp.MustCompile(`(?i)\b(union|select|insert|update|delete|drop|create|alter|exec|execute)\s+`)
	p.SQLComment = regexp.MustCompile(`(-{2}|/\*|\*/|\#)`)
	p.SQLQuotes = regexp.MustCompile(`['"` + "`" + `].*['"` + "`" + `]`)
	p.SQLUnion = regexp.MustCompile(`(?i)\bunion\s+(all\s+)?select\b`)

	// Path traversal patterns - comprehensive coverage
	p.PathTraversal = regexp.MustCompile(`\.\./|\.\.\\|%2e%2e[/\\]|%252e%252e[/\\]`)
	p.PathEncoded = regexp.MustCompile(`%2[eE]%2[eE]|%252[eE]%252[eE]`)

	// URL encoding detection
	p.URLEncoded = regexp.MustCompile(`%[0-9a-fA-F]{2}`)

	// Control characters (excluding tab)
	p.ControlChars = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)

	return p
}

// StringPatternsData holds commonly used string patterns for efficient matching.
type StringPatternsData struct {
	// XSS string patterns
	XSSPatterns []string

	// SQL injection string patterns
	SQLPatterns []string

	// Path traversal string patterns
	PathPatterns []string

	// Control character encodings
	ControlEncodings []string

	// Suspicious unicode characters
	UnicodeAttacks []rune

	// Supported Git hosts
	SupportedHosts map[string]bool
}

// GetStringPatterns returns a new instance of string patterns data.
// This replaces the global StringPatterns variable to avoid global state.
func GetStringPatterns() *StringPatternsData {
	return &StringPatternsData{
		XSSPatterns: []string{
			"<script", "</script>", "<img", "onerror=", "onclick=", "onload=",
			"javascript:", "data:text/html", "<svg", "<iframe", "&lt;script&gt;",
			"<div", "onmouseover=", "onfocus=", "onblur=", "vbscript:",
		},

		SQLPatterns: []string{
			"'; drop", "drop table", "union select", "insert into", "delete from",
			"update ", "select ", "or 1=1", "and 1=1", "waitfor delay",
			"information_schema", "/*", "--", "sleep(", "benchmark(",
		},

		PathPatterns: []string{
			"../", "./", "..\\", ".\\",
			"%2E%2E/", "%2E%2E\\", "%252E%252E/", "%252E%252E\\",
			"%2E%2E%2F", "%2E%2E%5C", "..%2F", "..%5C",
		},

		ControlEncodings: []string{
			"%00", "%01", "%02", "%03", "%04", "%05", "%06", "%07",
			"%08", "%0B", "%0C", "%0E", "%0F", "%10", "%11", "%12",
			"%13", "%14", "%15", "%16", "%17", "%18", "%19", "%1A",
			"%1B", "%1C", "%1D", "%1E", "%1F", "%7F",
		},

		UnicodeAttacks: []rune{
			'\u202D', '\u202E', '\u200B', '\u200C', // Directional override and zero-width
			'ρ', // Greek rho that looks like 'p'
		},

		SupportedHosts: map[string]bool{
			"github.com":    true,
			"gitlab.com":    true,
			"bitbucket.org": true,
		},
	}
}

// HTMLEntities returns HTML entities that could be used for XSS.
func HTMLEntities() map[string]string {
	return map[string]string{
		"&lt;":   "<",
		"&gt;":   ">",
		"&amp;":  "&",
		"&quot;": "\"",
		"&apos;": "'",
		"&#60;":  "<",
		"&#62;":  ">",
		"&#39;":  "'",
		"&#34;":  "\"",
	}
}

// URLEncodings returns common attack characters mapped to their URL-encoded forms.
func URLEncodings() map[string][]string {
	return map[string][]string{
		"<":  {"%3C", "%3c"},
		">":  {"%3E", "%3e"},
		"'":  {"%27"},
		"\"": {"%22"},
		"/":  {"%2F", "%2f"},
		"\\": {"%5C", "%5c"},
		";":  {"%3B", "%3b"},
		" ":  {"%20", "+"},
	}
}

// GetSupportedHosts returns the map of supported Git hosts.
// This provides access to supported hosts without exposing global state.
func GetSupportedHosts() map[string]bool {
	return GetStringPatterns().SupportedHosts
}

// IsPatternCompiled checks if patterns are properly compiled by attempting to create them.
// Since we no longer use global state, this validates pattern compilation directly.
func IsPatternCompiled() bool {
	p := GetPatterns()
	return p != nil && p.GitURL != nil
}

// ValidatePatterns ensures all patterns are properly compiled.
func ValidatePatterns() error {
	p := GetPatterns()

	// Test each pattern with a simple string to ensure it's working
	testPatterns := map[string]*regexp.Regexp{
		"GitURL":        p.GitURL,
		"XSSScript":     p.XSSScript,
		"XSSEvent":      p.XSSEvent,
		"XSSProtocol":   p.XSSProtocol,
		"XSSEntity":     p.XSSEntity,
		"SQLKeywords":   p.SQLKeywords,
		"SQLComment":    p.SQLComment,
		"SQLQuotes":     p.SQLQuotes,
		"SQLUnion":      p.SQLUnion,
		"PathTraversal": p.PathTraversal,
		"PathEncoded":   p.PathEncoded,
		"URLEncoded":    p.URLEncoded,
		"ControlChars":  p.ControlChars,
	}

	for name, pattern := range testPatterns {
		if pattern == nil {
			return &PatternError{Pattern: name, Message: "pattern is nil"}
		}
		// Test that pattern can be used (will panic if invalid)
		_ = pattern.MatchString("test")
	}

	return nil
}

// PatternError represents an error with a security pattern.
type PatternError struct {
	Pattern string
	Message string
}

func (e *PatternError) Error() string {
	return "pattern '" + e.Pattern + "': " + e.Message
}
