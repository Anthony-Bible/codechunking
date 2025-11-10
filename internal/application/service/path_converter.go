package service

import (
	"codechunking/internal/application/common/slogger"
	"context"
	"fmt"
	"regexp"
	"strings"
)

// MigrationConfig holds configuration for path migration operations.
type MigrationConfig struct {
	// Batch processing
	BatchSize int `json:"batch_size"`
	// Timeouts
	LockTimeoutSeconds  int `json:"lock_timeout_seconds"`
	QueryTimeoutSeconds int `json:"query_timeout_seconds"`
	// Performance
	MaxRetries    int  `json:"max_retries"`
	RetryDelayMs  int  `json:"retry_delay_ms"`
	EnableBackoff bool `json:"enable_backoff"`
	// Validation
	StrictValidation bool `json:"strict_validation"`
	SkipMalformed    bool `json:"skip_malformed"`
}

// DefaultMigrationConfig returns default configuration for migration.
func DefaultMigrationConfig() *MigrationConfig {
	return &MigrationConfig{
		BatchSize:           1000,
		LockTimeoutSeconds:  30,
		QueryTimeoutSeconds: 10,
		MaxRetries:          3,
		RetryDelayMs:        100,
		EnableBackoff:       true,
		StrictValidation:    false,
		SkipMalformed:       true,
	}
}

// PathConverter handles conversion of absolute UUID-prefixed paths to relative paths.
type PathConverter struct {
	config *MigrationConfig
	// Pre-compiled regex patterns for performance
	primaryPattern      *regexp.Regexp
	alternativePatterns []*regexp.Regexp
}

// NewPathConverter creates a new PathConverter instance.
func NewPathConverter(config *MigrationConfig) (*PathConverter, error) {
	if config == nil {
		config = DefaultMigrationConfig()
	}

	// Compile primary pattern
	primaryPattern, err := regexp.Compile(
		`^/tmp/codechunking-workspace/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/(.+)$`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to compile primary UUID pattern: %w", err)
	}

	// Compile alternative patterns
	altPatternStrings := []string{
		`^/var/tmp/codechunking-[^/]+/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/(.+)$`,
		`^/tmp/codechunking-[^/]+/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/(.+)$`,
	}

	var alternativePatterns []*regexp.Regexp
	for _, pattern := range altPatternStrings {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile alternative pattern '%s': %w", pattern, err)
		}
		alternativePatterns = append(alternativePatterns, compiled)
	}

	return &PathConverter{
		config:              config,
		primaryPattern:      primaryPattern,
		alternativePatterns: alternativePatterns,
	}, nil
}

// ConvertToRelativePath converts absolute UUID-prefixed paths to relative paths.
func (pc *PathConverter) ConvertToRelativePath(ctx context.Context, absolutePath string) (string, error) {
	if absolutePath == "" {
		if pc.config.StrictValidation {
			return "", &MigrationError{
				Type:    ErrorTypeValidation,
				Message: "empty path provided",
				Path:    absolutePath,
			}
		}
		return absolutePath, nil
	}

	// If already relative (doesn't start with /), return as-is
	if !strings.HasPrefix(absolutePath, "/") {
		slogger.Debug(ctx, "Path already relative, returning as-is", slogger.Fields{
			"path": absolutePath,
		})
		return absolutePath, nil
	}

	// Try primary pattern first
	if matches := pc.primaryPattern.FindStringSubmatch(absolutePath); len(matches) == 2 {
		slogger.Debug(ctx, "Converted path using primary pattern", slogger.Fields2(
			"from", absolutePath,
			"to", matches[1],
		))
		return matches[1], nil
	}

	// Try alternative patterns
	for i, pattern := range pc.alternativePatterns {
		if matches := pattern.FindStringSubmatch(absolutePath); len(matches) == 2 {
			slogger.Debug(ctx, "Converted path using alternative pattern", slogger.Fields3(
				"from", absolutePath,
				"to", matches[1],
				"pattern_index", i,
			))
			return matches[1], nil
		}
	}

	// Handle malformed paths
	if strings.Contains(absolutePath, "/tmp/codechunking-workspace/") {
		err := &MigrationError{
			Type:    ErrorTypeMalformedPath,
			Message: "malformed UUID path",
			Path:    absolutePath,
		}

		if pc.config.SkipMalformed {
			slogger.Warn(ctx, "Skipping malformed path", slogger.Fields{
				"path":  absolutePath,
				"error": err.Error(),
			})
			return "", err
		}

		return "", err
	}

	// Not a UUID-prefixed path, return as-is
	slogger.Debug(ctx, "Path does not match UUID patterns, returning as-is", slogger.Fields{
		"path": absolutePath,
	})
	return absolutePath, nil
}

// IsUUIDPrefixedPath checks if a path contains a UUID prefix.
func (pc *PathConverter) IsUUIDPrefixedPath(ctx context.Context, path string) bool {
	if path == "" {
		return false
	}

	// Check primary pattern
	if pc.primaryPattern.MatchString(path) {
		return true
	}

	// Check alternative patterns
	for _, pattern := range pc.alternativePatterns {
		if pattern.MatchString(path) {
			return true
		}
	}

	return false
}

// ValidatePath performs comprehensive path validation.
func (pc *PathConverter) ValidatePath(ctx context.Context, path string) error {
	if path == "" {
		return &MigrationError{
			Type:    ErrorTypeValidation,
			Message: "empty path",
			Path:    path,
		}
	}

	// Check for obviously invalid patterns
	if strings.Contains(path, "..") {
		return &MigrationError{
			Type:    ErrorTypeValidation,
			Message: "path contains parent directory references",
			Path:    path,
		}
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return &MigrationError{
			Type:    ErrorTypeValidation,
			Message: "path contains null bytes",
			Path:    path,
		}
	}

	// Validate UTF-8 encoding
	if !isValidUTF8(path) {
		return &MigrationError{
			Type:    ErrorTypeValidation,
			Message: "path contains invalid UTF-8 sequences",
			Path:    path,
		}
	}

	return nil
}

// isValidUTF8 checks if a string contains valid UTF-8.
func isValidUTF8(s string) bool {
	// Go strings are always valid UTF-8, but we can add additional checks
	// for control characters or other problematic sequences
	for _, r := range s {
		if r == '\u0000' {
			return false
		}
	}
	return true
}

// GetConfig returns the current configuration.
func (pc *PathConverter) GetConfig() *MigrationConfig {
	return pc.config
}
