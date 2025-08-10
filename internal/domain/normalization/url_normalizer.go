package normalization

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"codechunking/internal/application/common/security"
)

// URLNormalizer provides comprehensive URL normalization capabilities for repository URLs.
// It includes security validation, caching, and configurable normalization rules.
type URLNormalizer struct {
	config          *Config
	securityConfig  *security.Config
	normalizedCache map[string]string
	cacheMutex      sync.RWMutex
	maxCacheSize    int
	cacheEnabled    bool
}

// Config holds configuration options for URL normalization
type Config struct {
	// AllowedProtocols specifies which protocols are allowed (default: https, http)
	AllowedProtocols []string

	// SupportedHosts maps hostnames to whether they're supported
	SupportedHosts map[string]bool

	// NormalizeToHTTPS controls whether all URLs should be normalized to HTTPS
	NormalizeToHTTPS bool

	// RemoveTrailingSlash controls whether trailing slashes should be removed
	RemoveTrailingSlash bool

	// RemoveGitSuffix controls whether .git suffixes should be removed
	RemoveGitSuffix bool

	// StripQueryParams controls whether query parameters should be removed
	StripQueryParams bool

	// StripFragment controls whether URL fragments should be removed
	StripFragment bool

	// CaseSensitiveHosts controls whether host comparison is case-sensitive
	CaseSensitiveHosts bool

	// MaxURLLength sets the maximum allowed URL length
	MaxURLLength int
}

// DefaultConfig returns the default normalization configuration
func DefaultConfig() *Config {
	return &Config{
		AllowedProtocols: []string{"https", "http"},
		SupportedHosts: map[string]bool{
			"github.com":    true,
			"gitlab.com":    true,
			"bitbucket.org": true,
		},
		NormalizeToHTTPS:    true,
		RemoveTrailingSlash: true,
		RemoveGitSuffix:     true,
		StripQueryParams:    true,
		StripFragment:       true,
		CaseSensitiveHosts:  false,
		MaxURLLength:        2048,
	}
}

// NewURLNormalizer creates a new URL normalizer with the given configuration
func NewURLNormalizer(config *Config, securityConfig *security.Config) *URLNormalizer {
	if config == nil {
		config = DefaultConfig()
	}
	if securityConfig == nil {
		securityConfig = security.DefaultConfig()
	}

	return &URLNormalizer{
		config:          config,
		securityConfig:  securityConfig,
		normalizedCache: make(map[string]string),
		maxCacheSize:    1000,
		cacheEnabled:    true,
	}
}

// NewURLNormalizerWithCache creates a new URL normalizer with caching enabled
func NewURLNormalizerWithCache(config *Config, securityConfig *security.Config, maxCacheSize int) *URLNormalizer {
	normalizer := NewURLNormalizer(config, securityConfig)
	normalizer.maxCacheSize = maxCacheSize
	normalizer.cacheEnabled = maxCacheSize > 0
	return normalizer
}

// Normalize performs comprehensive URL normalization for duplicate detection.
// It applies security validation, normalization rules, and caching for performance.
func (n *URLNormalizer) Normalize(rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("repository URL cannot be empty")
	}

	// Check cache first if enabled
	if n.cacheEnabled {
		n.cacheMutex.RLock()
		if normalized, exists := n.normalizedCache[rawURL]; exists {
			n.cacheMutex.RUnlock()
			return normalized, nil
		}
		n.cacheMutex.RUnlock()
	}

	// Perform normalization
	normalized, err := n.normalizeURL(rawURL)
	if err != nil {
		return "", err
	}

	// Cache the result if caching is enabled
	if n.cacheEnabled {
		n.cacheMutex.Lock()
		// Implement simple cache eviction when max size is reached
		if len(n.normalizedCache) >= n.maxCacheSize {
			// Clear oldest entries (simple approach - in production might use LRU)
			for k := range n.normalizedCache {
				delete(n.normalizedCache, k)
				if len(n.normalizedCache) < n.maxCacheSize/2 {
					break
				}
			}
		}
		n.normalizedCache[rawURL] = normalized
		n.cacheMutex.Unlock()
	}

	return normalized, nil
}

// normalizeURL performs the actual URL normalization logic
func (n *URLNormalizer) normalizeURL(rawURL string) (string, error) {
	// Basic security validation
	if err := n.performSecurityValidation(rawURL); err != nil {
		return "", err
	}

	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %w", err)
	}

	// Validate protocol
	if !n.isAllowedProtocol(parsedURL.Scheme) {
		return "", fmt.Errorf("unsupported protocol: %s", parsedURL.Scheme)
	}

	// Normalize host
	normalizedHost := n.normalizeHost(parsedURL.Host)

	// Check if host is supported
	if !n.isSupportedHost(normalizedHost) {
		supportedHosts := make([]string, 0, len(n.config.SupportedHosts))
		for host := range n.config.SupportedHosts {
			supportedHosts = append(supportedHosts, host)
		}
		return "", fmt.Errorf("unsupported host: %s. Supported hosts: %s", parsedURL.Host, strings.Join(supportedHosts, ", "))
	}

	// Normalize scheme
	normalizedScheme := parsedURL.Scheme
	if n.config.NormalizeToHTTPS && (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") {
		normalizedScheme = "https"
	}

	// Normalize path
	normalizedPath, err := n.normalizePath(parsedURL.Path)
	if err != nil {
		return "", err
	}

	// Build normalized URL
	normalized := fmt.Sprintf("%s://%s%s", normalizedScheme, normalizedHost, normalizedPath)

	// Additional validation on normalized URL
	if err := n.validateNormalizedURL(normalized); err != nil {
		return "", err
	}

	return normalized, nil
}

// performSecurityValidation performs security checks on the raw URL
func (n *URLNormalizer) performSecurityValidation(rawURL string) error {
	// Length validation using security validator
	validator := security.GetDefaultValidator()
	if err := validator.ValidateURLLength(rawURL); err != nil {
		return err
	}

	// Use centralized security validation to eliminate code duplication
	return validator.ValidateURLSecurity(rawURL)
}

// isAllowedProtocol checks if the protocol is in the allowed list
func (n *URLNormalizer) isAllowedProtocol(scheme string) bool {
	scheme = strings.ToLower(scheme)
	for _, allowed := range n.config.AllowedProtocols {
		if scheme == strings.ToLower(allowed) {
			return true
		}
	}
	return false
}

// normalizeHost normalizes the hostname according to configuration
func (n *URLNormalizer) normalizeHost(host string) string {
	if !n.config.CaseSensitiveHosts {
		return strings.ToLower(host)
	}
	return host
}

// isSupportedHost checks if the host is in the supported list
func (n *URLNormalizer) isSupportedHost(host string) bool {
	normalizedHost := host
	if !n.config.CaseSensitiveHosts {
		normalizedHost = strings.ToLower(host)
	}
	return n.config.SupportedHosts[normalizedHost]
}

// normalizePath normalizes the URL path according to configuration
func (n *URLNormalizer) normalizePath(path string) (string, error) {
	// Decode URL encoding
	decodedPath := path
	if decoded, err := url.QueryUnescape(path); err == nil {
		decodedPath = decoded
	}

	// Remove trailing slashes
	if n.config.RemoveTrailingSlash {
		decodedPath = strings.TrimRight(decodedPath, "/")
	}

	// Remove .git suffix
	if n.config.RemoveGitSuffix {
		decodedPath = strings.TrimSuffix(decodedPath, ".git")
	}

	// Additional security check for path traversal in decoded path
	if strings.Contains(decodedPath, "..") {
		return "", fmt.Errorf("path traversal detected in normalized path")
	}

	return decodedPath, nil
}

// validateNormalizedURL performs final validation on the normalized URL
func (n *URLNormalizer) validateNormalizedURL(normalizedURL string) error {
	parsedURL, err := url.Parse(normalizedURL)
	if err != nil {
		return fmt.Errorf("normalized URL is invalid: %w", err)
	}

	// Validate path structure
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) < 2 {
		return fmt.Errorf("repository URL must include owner and repository name")
	}

	return nil
}

// ClearCache clears the normalization cache
func (n *URLNormalizer) ClearCache() {
	if n.cacheEnabled {
		n.cacheMutex.Lock()
		n.normalizedCache = make(map[string]string)
		n.cacheMutex.Unlock()
	}
}

// GetCacheStats returns cache statistics for monitoring
func (n *URLNormalizer) GetCacheStats() CacheStats {
	if !n.cacheEnabled {
		return CacheStats{Enabled: false}
	}

	n.cacheMutex.RLock()
	size := len(n.normalizedCache)
	n.cacheMutex.RUnlock()

	return CacheStats{
		Enabled: true,
		Size:    size,
		MaxSize: n.maxCacheSize,
		HitRate: 0, // Could be implemented with more detailed tracking
	}
}

// CacheStats holds cache statistics
type CacheStats struct {
	Enabled bool
	Size    int
	MaxSize int
	HitRate float64
}

// SetCacheEnabled enables or disables caching
func (n *URLNormalizer) SetCacheEnabled(enabled bool) {
	n.cacheMutex.Lock()
	n.cacheEnabled = enabled
	if !enabled {
		n.normalizedCache = make(map[string]string)
	}
	n.cacheMutex.Unlock()
}
