package normalization

import (
	"codechunking/internal/application/common/security"
)

// NormalizeRepositoryURL provides backward compatibility with the original function signature.
// It creates a normalizer with default configuration on each call to avoid global state.
func NormalizeRepositoryURL(rawURL string) (string, error) {
	normalizer := NewURLNormalizer(DefaultConfig(), security.DefaultConfig())
	return normalizer.Normalize(rawURL)
}

// GetDefaultNormalizer returns a new normalizer instance with default configuration.
// This replaces the global instance pattern to avoid global state.
func GetDefaultNormalizer() *URLNormalizer {
	return NewURLNormalizer(DefaultConfig(), security.DefaultConfig())
}
