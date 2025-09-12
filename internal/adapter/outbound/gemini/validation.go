package gemini

import (
	"errors"
	"regexp"
	"strings"
)

// ValidateAPIKeyFormat validates the format of a Gemini API key.
func ValidateAPIKeyFormat(apiKey string) (bool, error) {
	// Check if empty before trimming
	if apiKey == "" {
		return false, errors.New("API key cannot be empty")
	}

	// Check if only whitespace
	if strings.TrimSpace(apiKey) == "" {
		return false, errors.New("API key cannot be empty or whitespace")
	}

	// Trim whitespace for further checks
	trimmed := strings.TrimSpace(apiKey)

	// Check minimum length
	if len(trimmed) < 10 {
		return false, errors.New("API key is too short")
	}

	// Check for valid characters (alphanumeric, hyphens, underscores, dots)
	validChars := regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
	if !validChars.MatchString(trimmed) {
		return false, errors.New("API key contains invalid characters")
	}

	return true, nil
}
