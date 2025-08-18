package security

import (
	"time"
)

const (
	logLevelDEBUG  = "DEBUG"
	severityHigh   = "HIGH"
	severityMedium = "MEDIUM"

	// Default security config values.
	defaultMaxURLLength      = 2048
	defaultMaxBodySizeKB     = 64
	defaultRequestsPerMinute = 60
	defaultBurstSize         = 10
	defaultCacheSize         = 1000
	defaultCacheTTLMinutes   = 5

	// Constants for calculations.
	bytesPerKB = 1024
)

// Config holds all configurable security parameters.
type Config struct {
	// URL validation settings
	MaxURLLength int

	// Request body settings
	MaxBodySize int64

	// Rate limiting settings
	RateLimit RateLimitConfig

	// Content validation settings
	EnableXSSProtection    bool
	EnableSQLInjection     bool
	EnableControlCharCheck bool
	EnableUnicodeCheck     bool
	EnablePathTraversal    bool

	// Logging settings
	LogSecurityViolations bool
	LogLevel              string

	// Performance settings
	EnableValidationCache bool
	CacheSize             int
	CacheTTL              time.Duration
}

// RateLimitConfig configures rate limiting behavior.
type RateLimitConfig struct {
	RequestsPerMinute int
	BurstSize         int
	WindowSize        time.Duration
	Enabled           bool
}

// ValidationLimits holds limits for various validation operations.
type ValidationLimits struct {
	MaxFieldLength       int
	MaxArrayLength       int
	MaxNestingDepth      int
	MaxValidationTime    time.Duration
	MaxPatternComplexity int
}

// DefaultConfig returns a production-ready security configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxURLLength: defaultMaxURLLength,
		MaxBodySize:  defaultMaxBodySizeKB * bytesPerKB, // 64KB

		RateLimit: RateLimitConfig{
			RequestsPerMinute: defaultRequestsPerMinute,
			BurstSize:         defaultBurstSize,
			WindowSize:        time.Minute,
			Enabled:           true,
		},

		EnableXSSProtection:    true,
		EnableSQLInjection:     true,
		EnableControlCharCheck: true,
		EnableUnicodeCheck:     true,
		EnablePathTraversal:    true,

		LogSecurityViolations: true,
		LogLevel:              "INFO",

		EnableValidationCache: true,
		CacheSize:             defaultCacheSize,
		CacheTTL:              defaultCacheTTLMinutes * time.Minute,
	}
}

// DevelopmentConfig returns a development-friendly configuration with relaxed limits.
func DevelopmentConfig() *Config {
	config := DefaultConfig()
	config.RateLimit.RequestsPerMinute = 300
	config.RateLimit.BurstSize = 50
	config.LogLevel = logLevelDEBUG
	return config
}

// TestConfig returns a configuration suitable for testing.
func TestConfig() *Config {
	config := DefaultConfig()
	config.RateLimit.Enabled = false
	config.EnableValidationCache = false
	config.LogSecurityViolations = false
	return config
}

// Validate ensures the configuration is valid and sets reasonable defaults.
func (c *Config) Validate() error {
	if c.MaxURLLength <= 0 {
		c.MaxURLLength = 2048
	}

	if c.MaxBodySize <= 0 {
		c.MaxBodySize = defaultMaxBodySizeKB * bytesPerKB
	}

	if c.RateLimit.RequestsPerMinute <= 0 {
		c.RateLimit.RequestsPerMinute = 60
	}

	if c.RateLimit.BurstSize <= 0 {
		c.RateLimit.BurstSize = 10
	}

	if c.RateLimit.WindowSize <= 0 {
		c.RateLimit.WindowSize = time.Minute
	}

	if c.CacheSize <= 0 {
		c.CacheSize = 1000
	}

	if c.CacheTTL <= 0 {
		c.CacheTTL = defaultCacheTTLMinutes * time.Minute
	}

	return nil
}

// IsFeatureEnabled checks if a security feature is enabled.
func (c *Config) IsFeatureEnabled(feature string) bool {
	switch feature {
	case "xss":
		return c.EnableXSSProtection
	case "sql":
		return c.EnableSQLInjection
	case "control":
		return c.EnableControlCharCheck
	case "unicode":
		return c.EnableUnicodeCheck
	case "path":
		return c.EnablePathTraversal
	case "cache":
		return c.EnableValidationCache
	case "rate_limit":
		return c.RateLimit.Enabled
	default:
		return false
	}
}
