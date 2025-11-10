package valueobject

import (
	"codechunking/internal/application/common/security"
	domainerrors "codechunking/internal/domain/errors/domain"
	"codechunking/internal/domain/normalization"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const (
	// MinPathPartsForRepoURL is the minimum number of path parts required for a repository URL.
	MinPathPartsForRepoURL = 2
)

// RepositoryURL represents a validated Git repository URL with dual storage.
// It maintains both the original raw input URL and its normalized form to eliminate
// redundant normalization operations while enabling efficient duplicate detection.
//
// The normalization happens once during creation and the results are cached,
// providing significant performance benefits for operations that need normalized URLs.
type RepositoryURL struct {
	raw        string // Original input URL as provided by user
	normalized string // Normalized URL used for deduplication and storage
}

// URLValidator handles URL validation with configurable security policies.
type URLValidator struct {
	config             *security.Config
	unicodeValidator   *security.UnicodeValidator
	injectionValidator *security.InjectionValidator
}

// newURLValidator creates a new URL validator with default configuration.
func newURLValidator() *URLValidator {
	config := security.DefaultConfig()
	return &URLValidator{
		config:             config,
		unicodeValidator:   security.NewUnicodeValidator(config),
		injectionValidator: security.NewInjectionValidator(config),
	}
}

// NewRepositoryURL creates a new RepositoryURL after comprehensive validation.
// Performs normalization once during creation and caches both raw and normalized forms,
// eliminating redundant normalization operations in subsequent method calls.
// This provides significant performance benefits for applications that frequently
// access normalized URLs or perform duplicate detection.
func NewRepositoryURL(rawURL string) (RepositoryURL, error) {
	validator := newURLValidator()
	return validator.validateAndCreate(rawURL)
}

// validateAndCreate performs comprehensive URL validation and creates RepositoryURL.
func (uv *URLValidator) validateAndCreate(rawURL string) (RepositoryURL, error) {
	if rawURL == "" {
		return RepositoryURL{}, fmt.Errorf(
			"invalid repository URL: URL cannot be empty: %w",
			domainerrors.ErrInvalidRepositoryURL,
		)
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
		return RepositoryURL{}, fmt.Errorf(
			"invalid repository URL: invalid URL format: %w",
			domainerrors.ErrInvalidRepositoryURL,
		)
	}

	// Perform structural validation (including port checks)
	if validateErr := uv.validateURLStructure(rawURL, parsedURL); validateErr != nil {
		return RepositoryURL{}, validateErr
	}

	// Use comprehensive normalization
	normalizedURL, err := normalization.NormalizeRepositoryURL(rawURL)
	if err != nil {
		return RepositoryURL{}, err
	}

	return RepositoryURL{raw: rawURL, normalized: normalizedURL}, nil
}

// validateSchemeBeforeParsing validates URL scheme before parsing to catch malicious protocols.
func (uv *URLValidator) validateSchemeBeforeParsing(rawURL string) error {
	// Check for malicious schemes early
	if err := uv.checkMaliciousSchemes(rawURL); err != nil {
		return err
	}

	// Ensure URL has a valid scheme format (must be http:// or https://)
	if err := uv.validateHTTPScheme(rawURL); err != nil {
		return err
	}

	return nil
}

// validateURLEncodingBypass checks for URL encoding bypass attempts.
func (uv *URLValidator) validateURLEncodingBypass(rawURL string) error {
	return security.GetDefaultValidator().ValidateURLEncodingBypass(rawURL)
}

// performSecurityValidation runs security checks appropriate for repository URLs.
func (uv *URLValidator) performSecurityValidation(rawURL string) error {
	// Check for control characters (but allow normal URL characters)
	if err := uv.validateURLControlCharacters(rawURL); err != nil {
		return errors.New("URL contains invalid control characters")
	}

	// Check for path traversal attacks (but allow normal path separators)
	if err := uv.validateURLPathTraversal(rawURL); err != nil {
		return errors.New("path traversal detected")
	}

	// Check for Unicode attacks
	if err := uv.unicodeValidator.ValidateUnicodeAttacks(rawURL); err != nil {
		return errors.New("suspicious unicode detected")
	}

	// URL-specific XSS validation (allowing legitimate URL characters)
	if err := uv.validateURLForXSS(rawURL); err != nil {
		return errors.New("invalid characters detected")
	}

	// URL-specific SQL injection validation (allowing legitimate URL characters)
	if err := uv.validateURLForSQLInjection(rawURL); err != nil {
		return errors.New("invalid characters detected")
	}

	// Additional URL encoding checks handled by centralized validator

	return nil
}

// validateURLControlCharacters checks for dangerous control characters in URLs.
func (uv *URLValidator) validateURLControlCharacters(rawURL string) error {
	// Allow normal URL control characters but block dangerous ones
	for _, r := range rawURL {
		if r == '\x00' || r == '\r' || r == '\n' || (r >= '\x01' && r <= '\x08') || (r >= '\x0b' && r <= '\x0c') ||
			(r >= '\x0e' && r <= '\x1f') {
			return fmt.Errorf("dangerous control character detected: %U", r)
		}
	}

	// Check for URL-encoded null bytes and other dangerous control chars
	dangerousEncodings := []string{
		"%00",
		"%0d",
		"%0a",
		"%01",
		"%02",
		"%03",
		"%04",
		"%05",
		"%06",
		"%07",
		"%08",
		"%0b",
		"%0c",
		"%0e",
		"%0f",
	}
	lowercaseURL := strings.ToLower(rawURL)
	for _, encoding := range dangerousEncodings {
		if strings.Contains(lowercaseURL, encoding) {
			return errors.New("dangerous URL-encoded control character detected")
		}
	}

	return nil
}

// validateURLPathTraversal checks for path traversal in URL context.
func (uv *URLValidator) validateURLPathTraversal(rawURL string) error {
	return security.GetDefaultValidator().ValidatePathTraversalPatterns(rawURL)
}

// validateURLForXSS checks for XSS patterns that are dangerous in URL context.
func (uv *URLValidator) validateURLForXSS(rawURL string) error {
	lowercaseURL := strings.ToLower(rawURL)

	// Check for script tags in URL (definitely malicious)
	if strings.Contains(lowercaseURL, "<script") || strings.Contains(lowercaseURL, "</script") {
		return errors.New("script tag detected in URL")
	}

	// Check for javascript event handlers in URL
	dangerousEvents := []string{"onload=", "onerror=", "onclick=", "onmouseover=", "onfocus="}
	for _, event := range dangerousEvents {
		if strings.Contains(lowercaseURL, event) {
			return errors.New("dangerous event handler detected in URL")
		}
	}

	// Allow # and ? which are legitimate URL components
	// Only block if they're part of obviously malicious patterns

	return nil
}

// validateURLForSQLInjection checks for SQL injection patterns dangerous in URL context.
func (uv *URLValidator) validateURLForSQLInjection(rawURL string) error {
	return security.GetDefaultValidator().ValidateSQLInjectionPatterns(rawURL)
}

// validateURLStructure validates the URL structure and Git hosting requirements.
func (uv *URLValidator) validateURLStructure(rawURL string, parsedURL *url.URL) error {
	// Check scheme
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return fmt.Errorf(
			"invalid repository URL: URL must use http or https scheme: %w",
			domainerrors.ErrInvalidRepositoryURL,
		)
	}

	// Security: no non-standard ports
	if parsedURL.Port() != "" {
		return fmt.Errorf(
			"invalid repository URL: non-standard port not allowed: %w",
			domainerrors.ErrInvalidRepositoryURL,
		)
	}

	// Security: validate host format using centralized validator
	if err := security.GetDefaultValidator().ValidateHostFormatSecurity(rawURL); err != nil {
		return err
	}

	// Extract hostname without port and normalize to lowercase for supported host check
	hostname := strings.ToLower(parsedURL.Hostname())
	if !security.GetSupportedHosts()[hostname] {
		return fmt.Errorf(
			"invalid repository URL: unsupported host %s: %w",
			hostname,
			domainerrors.ErrInvalidRepositoryURL,
		)
	}

	// Note: We rely on individual structural validations rather than a rigid regex pattern
	// to support case-insensitive hostnames and various URL variations

	// Validate path structure
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) < MinPathPartsForRepoURL {
		return fmt.Errorf(
			"invalid repository URL: missing repository path in URL: %w",
			domainerrors.ErrInvalidRepositoryURL,
		)
	}

	return nil
}

// NewRepositoryURLWithConfig creates a RepositoryURL with custom security configuration.
// Like NewRepositoryURL, it performs normalization once and caches the results for optimal performance.
func NewRepositoryURLWithConfig(rawURL string, config *security.Config) (RepositoryURL, error) {
	validator := &URLValidator{
		config:             config,
		unicodeValidator:   security.NewUnicodeValidator(config),
		injectionValidator: security.NewInjectionValidator(config),
	}
	return validator.validateAndCreate(rawURL)
}

// ValidateRepositoryURL validates a repository URL without creating the value object.
// Use this when you only need validation without the performance benefits of caching.
func ValidateRepositoryURL(rawURL string) error {
	validator := newURLValidator()
	_, err := validator.validateAndCreate(rawURL)
	return err
}

// String returns the string representation of the repository URL.
// Returns the normalized form to maintain backward compatibility with existing code
// that expects a consistent, normalized representation.
func (r RepositoryURL) String() string {
	return r.normalized
}

// Value returns the underlying string value.
// This is an alias for String() and returns the normalized form for consistency.
func (r RepositoryURL) Value() string {
	return r.normalized
}

// Raw returns the original raw input URL exactly as provided by the user.
// This is useful for display purposes or when the original format needs to be preserved.
// Performance note: This avoids re-normalization since the raw value is cached.
func (r RepositoryURL) Raw() string {
	return r.raw
}

// Normalized returns the normalized URL used for deduplication and storage.
// This form has consistent casing, protocol, and removes redundant elements like .git suffix.
// Performance note: This returns the cached normalized value, eliminating redundant computation.
func (r RepositoryURL) Normalized() string {
	return r.normalized
}

// Host returns the hostname of the repository URL (e.g., "github.com").
// Uses the normalized URL for consistent results.
func (r RepositoryURL) Host() string {
	// Safe to ignore error as URL was validated during creation
	parsedURL, _ := url.Parse(r.normalized)
	return parsedURL.Host
}

// Owner returns the repository owner/organization name (e.g., "golang" from "github.com/golang/go").
// Uses the normalized URL for consistent parsing results.
func (r RepositoryURL) Owner() string {
	// Safe to ignore error as URL was validated during creation
	parsedURL, _ := url.Parse(r.normalized)
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) >= 1 {
		return pathParts[0]
	}
	return ""
}

// Name returns the repository name (e.g., "go" from "github.com/golang/go").
// Uses the normalized URL for consistent parsing results.
func (r RepositoryURL) Name() string {
	// Safe to ignore error as URL was validated during creation
	parsedURL, _ := url.Parse(r.normalized)
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) >= MinPathPartsForRepoURL {
		return pathParts[1]
	}
	return ""
}

// FullName returns the full repository name in "owner/name" format (e.g., "golang/go").
// Returns empty string if either owner or name cannot be determined.
func (r RepositoryURL) FullName() string {
	owner := r.Owner()
	name := r.Name()
	if owner != "" && name != "" {
		return fmt.Sprintf("%s/%s", owner, name)
	}
	return ""
}

// CloneURL returns the URL suitable for git clone operations.
// Appends .git suffix to the normalized URL to ensure compatibility with git tooling.
func (r RepositoryURL) CloneURL() string {
	return r.normalized + ".git"
}

// Equal compares two RepositoryURL instances for equality.
// Comparison is based on normalized URLs to ensure that functionally identical
// URLs (e.g., with/without .git suffix) are considered equal.
// Performance note: Uses cached normalized values for efficient comparison.
func (r RepositoryURL) Equal(other RepositoryURL) bool {
	return r.normalized == other.normalized
}

// checkMaliciousSchemes checks for dangerous URL schemes.
func (uv *URLValidator) checkMaliciousSchemes(rawURL string) error {
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
	return nil
}

// validateHTTPScheme ensures URL uses only HTTP/HTTPS schemes.
func (uv *URLValidator) validateHTTPScheme(rawURL string) error {
	lowercaseURL := strings.ToLower(rawURL)
	if !strings.HasPrefix(lowercaseURL, "http://") && !strings.HasPrefix(lowercaseURL, "https://") {
		return fmt.Errorf(
			"invalid repository URL: URL must use http or https scheme: %w",
			domainerrors.ErrInvalidRepositoryURL,
		)
	}
	return nil
}

// NormalizeRepositoryURL is now available in the normalization package
// Use: normalization.NormalizeRepositoryURL(rawURL)
