package valueobject

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// RepositoryURL represents a validated Git repository URL
type RepositoryURL struct {
	value string
}

// supportedHosts contains the list of supported Git hosting providers
var supportedHosts = map[string]bool{
	"github.com":    true,
	"gitlab.com":    true,
	"bitbucket.org": true,
}

// gitURLPattern matches valid Git repository URLs
var gitURLPattern = regexp.MustCompile(`^https?://(github\.com|gitlab\.com|bitbucket\.org)/.+/.+$`)

// NewRepositoryURL creates a new RepositoryURL after validation
func NewRepositoryURL(rawURL string) (RepositoryURL, error) {
	if rawURL == "" {
		return RepositoryURL{}, fmt.Errorf("repository URL cannot be empty")
	}

	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return RepositoryURL{}, fmt.Errorf("invalid URL format: %w", err)
	}

	// Check scheme
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return RepositoryURL{}, fmt.Errorf("URL must use http or https scheme, got: %s", parsedURL.Scheme)
	}

	// Check if host is supported
	if !supportedHosts[parsedURL.Host] {
		return RepositoryURL{}, fmt.Errorf("unsupported host: %s. Supported hosts: github.com, gitlab.com, bitbucket.org", parsedURL.Host)
	}

	// Validate URL pattern
	if !gitURLPattern.MatchString(rawURL) {
		return RepositoryURL{}, fmt.Errorf("invalid repository URL format")
	}

	// Validate path structure
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) < 2 {
		return RepositoryURL{}, fmt.Errorf("repository URL must include owner and repository name")
	}

	// Remove .git suffix if present
	normalizedURL := strings.TrimSuffix(rawURL, ".git")

	return RepositoryURL{value: normalizedURL}, nil
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
