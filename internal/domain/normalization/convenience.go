package normalization

import (
	"codechunking/internal/application/common/security"
)

// defaultNormalizer is a shared instance for backward compatibility.
var defaultNormalizer *URLNormalizer

func init() {
	// Initialize with default configuration for backward compatibility
	defaultNormalizer = NewURLNormalizer(DefaultConfig(), security.DefaultConfig())
}

// NormalizeRepositoryURL provides backward compatibility with the original function signature.
// It uses the default normalizer configuration for consistency with existing code.
func NormalizeRepositoryURL(rawURL string) (string, error) {
	return defaultNormalizer.Normalize(rawURL)
}

// GetDefaultNormalizer returns the default normalizer instance for advanced usage.
func GetDefaultNormalizer() *URLNormalizer {
	return defaultNormalizer
}

// SetDefaultNormalizer allows customization of the default normalizer.
func SetDefaultNormalizer(normalizer *URLNormalizer) {
	defaultNormalizer = normalizer
}
