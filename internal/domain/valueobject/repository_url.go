package valueobject

import (
	"fmt"
	"net/url"
	"strings"

	"codechunking/internal/application/common/security"
	"codechunking/internal/domain/normalization"
)

// RepositoryURL represents a validated Git repository URL
type RepositoryURL struct {
	value string
}

// URLValidator handles URL validation with configurable security policies
type URLValidator struct {
	config             *security.Config
	unicodeValidator   *security.UnicodeValidator
	injectionValidator *security.InjectionValidator
}

// newURLValidator creates a new URL validator with default configuration
func newURLValidator() *URLValidator {
	config := security.DefaultConfig()
	return &URLValidator{
		config:             config,
		unicodeValidator:   security.NewUnicodeValidator(config),
		injectionValidator: security.NewInjectionValidator(config),
	}
}

// NewRepositoryURL creates a new RepositoryURL after comprehensive validation
func NewRepositoryURL(rawURL string) (RepositoryURL, error) {
	validator := newURLValidator()
	return validator.validateAndCreate(rawURL)
}

// validateAndCreate performs comprehensive URL validation and creates RepositoryURL
func (uv *URLValidator) validateAndCreate(rawURL string) (RepositoryURL, error) {
	if rawURL == "" {
		return RepositoryURL{}, fmt.Errorf("repository URL cannot be empty")
	}

	// Security validation: URL length limit using centralized validator
	validator := security.GetDefaultValidator()
	if err := validator.ValidateURLLength(rawURL); err != nil {
		return RepositoryURL{}, err
	}

	// Early scheme validation before parsing (highest priority)
	if err := uv.validateSchemeBeforeParsing(rawURL); err != nil {
		return RepositoryURL{}, err
	}

	// CRITICAL: Perform security validation on raw URL BEFORE any parsing
	if err := uv.performSecurityValidation(rawURL); err != nil {
		return RepositoryURL{}, err
	}

	// Check for URL encoding bypass attempts
	if err := uv.validateURLEncodingBypass(rawURL); err != nil {
		return RepositoryURL{}, err
	}

	// Parse URL for structural validation
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return RepositoryURL{}, fmt.Errorf("invalid URL format: %w", err)
	}

	// Perform structural validation (including port checks)
	if err := uv.validateURLStructure(rawURL, parsedURL); err != nil {
		return RepositoryURL{}, err
	}

	// Use comprehensive normalization
	normalizedURL, err := normalization.NormalizeRepositoryURL(rawURL)
	if err != nil {
		return RepositoryURL{}, err
	}

	return RepositoryURL{value: normalizedURL}, nil
}

// validateSchemeBeforeParsing validates URL scheme before parsing to catch malicious protocols
func (uv *URLValidator) validateSchemeBeforeParsing(rawURL string) error {
	// Check for malicious schemes early
	maliciousSchemes := []string{
		"javascript:", "data:", "file:", "ftp:", "mailto:", "tel:", "sms:",
		"vbscript:", "about:", "chrome:", "chrome-extension:", "resource:",
	}

	lowercaseURL := strings.ToLower(rawURL)
	for _, scheme := range maliciousSchemes {
		if strings.HasPrefix(lowercaseURL, scheme) {
			return fmt.Errorf("malicious protocol detected: %s", scheme[:len(scheme)-1])
		}
	}

	// Ensure URL has a valid scheme format (must be http:// or https://)
	if !strings.HasPrefix(lowercaseURL, "http://") && !strings.HasPrefix(lowercaseURL, "https://") {
		return fmt.Errorf("URL must use http or https scheme")
	}

	return nil
}

// validateURLEncodingBypass checks for URL encoding bypass attempts
func (uv *URLValidator) validateURLEncodingBypass(rawURL string) error {
	return security.GetDefaultValidator().ValidateURLEncodingBypass(rawURL)
}

// performSecurityValidation runs security checks appropriate for repository URLs
func (uv *URLValidator) performSecurityValidation(rawURL string) error {
	// Check for control characters (but allow normal URL characters)
	if err := uv.validateURLControlCharacters(rawURL); err != nil {
		return fmt.Errorf("URL contains invalid control characters")
	}

	// Check for path traversal attacks (but allow normal path separators)
	if err := uv.validateURLPathTraversal(rawURL); err != nil {
		return fmt.Errorf("path traversal detected")
	}

	// Check for Unicode attacks
	if err := uv.unicodeValidator.ValidateUnicodeAttacks(rawURL); err != nil {
		return fmt.Errorf("suspicious unicode detected")
	}

	// URL-specific XSS validation (allowing legitimate URL characters)
	if err := uv.validateURLForXSS(rawURL); err != nil {
		return fmt.Errorf("invalid characters detected")
	}

	// URL-specific SQL injection validation (allowing legitimate URL characters)
	if err := uv.validateURLForSQLInjection(rawURL); err != nil {
		return fmt.Errorf("invalid characters detected")
	}

	// Additional URL encoding checks handled by centralized validator

	return nil
}

// validateURLControlCharacters checks for dangerous control characters in URLs
func (uv *URLValidator) validateURLControlCharacters(rawURL string) error {
	// Allow normal URL control characters but block dangerous ones
	for _, r := range rawURL {
		if r == '\x00' || r == '\r' || r == '\n' || (r >= '\x01' && r <= '\x08') || (r >= '\x0b' && r <= '\x0c') || (r >= '\x0e' && r <= '\x1f') {
			return fmt.Errorf("dangerous control character detected: %U", r)
		}
	}

	// Check for URL-encoded null bytes and other dangerous control chars
	dangerousEncodings := []string{"%00", "%0d", "%0a", "%01", "%02", "%03", "%04", "%05", "%06", "%07", "%08", "%0b", "%0c", "%0e", "%0f"}
	lowercaseURL := strings.ToLower(rawURL)
	for _, encoding := range dangerousEncodings {
		if strings.Contains(lowercaseURL, encoding) {
			return fmt.Errorf("dangerous URL-encoded control character detected")
		}
	}

	return nil
}

// validateURLPathTraversal checks for path traversal in URL context
func (uv *URLValidator) validateURLPathTraversal(rawURL string) error {
	return security.GetDefaultValidator().ValidatePathTraversalPatterns(rawURL)
}

// validateURLForXSS checks for XSS patterns that are dangerous in URL context
func (uv *URLValidator) validateURLForXSS(rawURL string) error {
	lowercaseURL := strings.ToLower(rawURL)

	// Check for script tags in URL (definitely malicious)
	if strings.Contains(lowercaseURL, "<script") || strings.Contains(lowercaseURL, "</script") {
		return fmt.Errorf("script tag detected in URL")
	}

	// Check for javascript event handlers in URL
	dangerousEvents := []string{"onload=", "onerror=", "onclick=", "onmouseover=", "onfocus="}
	for _, event := range dangerousEvents {
		if strings.Contains(lowercaseURL, event) {
			return fmt.Errorf("dangerous event handler detected in URL")
		}
	}

	// Allow # and ? which are legitimate URL components
	// Only block if they're part of obviously malicious patterns

	return nil
}

// validateURLForSQLInjection checks for SQL injection patterns dangerous in URL context
func (uv *URLValidator) validateURLForSQLInjection(rawURL string) error {
	return security.GetDefaultValidator().ValidateSQLInjectionPatterns(rawURL)
}

// validateURLStructure validates the URL structure and Git hosting requirements
func (uv *URLValidator) validateURLStructure(rawURL string, parsedURL *url.URL) error {
	// Check scheme
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return fmt.Errorf("URL must use http or https scheme, got: %s", parsedURL.Scheme)
	}

	// Security: no non-standard ports
	if parsedURL.Port() != "" {
		return fmt.Errorf("non-standard port not allowed")
	}

	// Security: validate host format using centralized validator
	if err := security.GetDefaultValidator().ValidateHostFormatSecurity(rawURL); err != nil {
		return err
	}

	// Extract hostname without port and normalize to lowercase for supported host check
	hostname := strings.ToLower(parsedURL.Hostname())
	if !security.StringPatterns.SupportedHosts[hostname] {
		return fmt.Errorf("unsupported host: %s. Supported hosts: github.com, gitlab.com, bitbucket.org", hostname)
	}

	// Note: We rely on individual structural validations rather than a rigid regex pattern
	// to support case-insensitive hostnames and various URL variations

	// Validate path structure
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) < 2 {
		return fmt.Errorf("repository URL must include owner and repository name")
	}

	return nil
}

// NewRepositoryURLWithConfig creates a RepositoryURL with custom security configuration
func NewRepositoryURLWithConfig(rawURL string, config *security.Config) (RepositoryURL, error) {
	validator := &URLValidator{
		config:             config,
		unicodeValidator:   security.NewUnicodeValidator(config),
		injectionValidator: security.NewInjectionValidator(config),
	}
	return validator.validateAndCreate(rawURL)
}

// ValidateRepositoryURL validates a repository URL without creating the value object
func ValidateRepositoryURL(rawURL string) error {
	validator := newURLValidator()
	_, err := validator.validateAndCreate(rawURL)
	return err
}

// String returns the string representation of the repository URL
func (r RepositoryURL) String() string {
	return r.value
}

// Value returns the underlying string value
func (r RepositoryURL) Value() string {
	return r.value
}

// Host returns the host of the repository URL
func (r RepositoryURL) Host() string {
	parsedURL, _ := url.Parse(r.value) // Safe to ignore error as URL was validated during creation
	return parsedURL.Host
}

// Owner returns the repository owner/organization name
func (r RepositoryURL) Owner() string {
	parsedURL, _ := url.Parse(r.value)
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) >= 1 {
		return pathParts[0]
	}
	return ""
}

// Name returns the repository name
func (r RepositoryURL) Name() string {
	parsedURL, _ := url.Parse(r.value)
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) >= 2 {
		return pathParts[1]
	}
	return ""
}

// FullName returns the full repository name (owner/name)
func (r RepositoryURL) FullName() string {
	owner := r.Owner()
	name := r.Name()
	if owner != "" && name != "" {
		return fmt.Sprintf("%s/%s", owner, name)
	}
	return ""
}

// CloneURL returns the URL suitable for git clone operations
func (r RepositoryURL) CloneURL() string {
	return r.value + ".git"
}

// Equal compares two RepositoryURL instances
func (r RepositoryURL) Equal(other RepositoryURL) bool {
	return r.value == other.value
}

// NormalizeRepositoryURL is now available in the normalization package
// Use: normalization.NormalizeRepositoryURL(rawURL)
