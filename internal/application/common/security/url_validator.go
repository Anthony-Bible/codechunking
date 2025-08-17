package security

import (
	"errors"
	"fmt"
	"strings"
)

// URLSecurityValidator provides centralized URL security validation
// to eliminate code duplication across the codebase.
type URLSecurityValidator struct {
	config         *Config
	stringPatterns *StringPatternsData
}

// NewURLSecurityValidator creates a new URL security validator.
func NewURLSecurityValidator(config *Config) *URLSecurityValidator {
	if config == nil {
		config = DefaultConfig()
	}
	return &URLSecurityValidator{
		config:         config,
		stringPatterns: GetStringPatterns(),
	}
}

// ValidateURLSecurity performs comprehensive security validation on a raw URL.
func (v *URLSecurityValidator) ValidateURLSecurity(rawURL string) error {
	if err := v.ValidateSQLInjectionPatterns(rawURL); err != nil {
		return err
	}

	if err := v.ValidatePathTraversalPatterns(rawURL); err != nil {
		return err
	}

	if err := v.ValidateHostFormatSecurity(rawURL); err != nil {
		return err
	}

	if err := v.ValidateURLEncodingBypass(rawURL); err != nil {
		return err
	}

	return nil
}

// ValidateSQLInjectionPatterns checks for SQL injection patterns in URLs.
func (v *URLSecurityValidator) ValidateSQLInjectionPatterns(rawURL string) error {
	// Check for SQL comments that are definitely malicious
	if strings.Contains(rawURL, "--") || strings.Contains(rawURL, "/*") || strings.Contains(rawURL, "*/") {
		return errors.New("SQL comment pattern detected in URL")
	}

	// Check for common SQL injection patterns using lowercase URL
	lowercaseURL := strings.ToLower(rawURL)
	for _, pattern := range v.stringPatterns.SQLPatterns {
		if strings.Contains(lowercaseURL, pattern) {
			return errors.New("SQL injection pattern detected in URL")
		}
	}

	return nil
}

// ValidatePathTraversalPatterns checks for path traversal attempts in URLs.
func (v *URLSecurityValidator) ValidatePathTraversalPatterns(rawURL string) error {
	// Check for basic path traversal patterns
	if strings.Contains(rawURL, "..") {
		// Allow if it's just in a normal path context, but block if it's suspicious
		if strings.Contains(rawURL, "../") || strings.Contains(rawURL, "..\\") {
			return errors.New("path traversal pattern detected")
		}
	}

	// Check for URL-encoded path traversal
	if strings.Contains(rawURL, "%2e%2e") || strings.Contains(rawURL, "%2E%2E") {
		return errors.New("URL-encoded path traversal detected")
	}

	return nil
}

// ValidateHostFormatSecurity checks for security issues in host format.
func (v *URLSecurityValidator) ValidateHostFormatSecurity(rawURL string) error {
	// Security: no user info in URL (@ symbol)
	if strings.Contains(rawURL, "@") {
		return errors.New("invalid host format: @ symbol not allowed")
	}

	return nil
}

// ValidateURLEncodingBypass checks for URL encoding bypass attempts.
func (v *URLSecurityValidator) ValidateURLEncodingBypass(rawURL string) error {
	// Check for URL-encoded path separators that could be used to bypass validation
	if strings.Contains(rawURL, "%2F") || strings.Contains(rawURL, "%2f") {
		return errors.New("invalid encoding detected")
	}

	// Check for other potentially dangerous URL-encoded characters
	dangerousEncodings := []string{
		"%2E%2E", "%2e%2e", // encoded ..
		"%5C", "%5c", // encoded backslash
		"%7C", "%7c", // encoded pipe
		"%3C", "%3c", // encoded <
		"%3E", "%3e", // encoded >
	}

	upperURL := strings.ToUpper(rawURL)
	for _, encoding := range dangerousEncodings {
		if strings.Contains(upperURL, strings.ToUpper(encoding)) {
			return errors.New("invalid encoding detected")
		}
	}

	// Additional URL encoding checks for double-encoding attacks
	if strings.Contains(rawURL, "%2E%2E") || strings.Contains(rawURL, "%252") {
		return errors.New("invalid encoding detected")
	}

	return nil
}

// ValidateURLLength checks if URL exceeds maximum allowed length.
func (v *URLSecurityValidator) ValidateURLLength(rawURL string) error {
	if len(rawURL) > v.config.MaxURLLength {
		return fmt.Errorf("URL too long: %d characters (max: %d)", len(rawURL), v.config.MaxURLLength)
	}
	return nil
}

// GetDefaultValidator returns a default URL security validator instance.
func GetDefaultValidator() *URLSecurityValidator {
	return NewURLSecurityValidator(nil)
}
