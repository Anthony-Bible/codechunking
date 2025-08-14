package common

import (
	"fmt"
	"strings"
	"unicode"

	"codechunking/internal/application/dto"
)

// ServiceError represents a service-level error with context.
type ServiceError struct {
	Operation string
	Cause     error
}

// Error implements the error interface.
func (e ServiceError) Error() string {
	return fmt.Sprintf("failed to %s: %v", e.Operation, e.Cause)
}

// Unwrap returns the underlying error.
func (e ServiceError) Unwrap() error {
	return e.Cause
}

// WrapServiceError wraps an error with service operation context.
func WrapServiceError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return ServiceError{
		Operation: operation,
		Cause:     err,
	}
}

// Common error operations for consistent messaging.
const (
	OpCreateRepository      = "create repository"
	OpUpdateRepository      = "update repository"
	OpDeleteRepository      = "delete repository"
	OpRetrieveRepository    = "retrieve repository"
	OpListRepositories      = "retrieve repositories"
	OpCreateIndexingJob     = "create indexing job"
	OpUpdateIndexingJob     = "update indexing job"
	OpRetrieveIndexingJob   = "retrieve indexing job"
	OpListIndexingJobs      = "retrieve indexing jobs"
	OpSaveRepository        = "save repository"
	OpSaveIndexingJob       = "save indexing job"
	OpPublishJob            = "publish indexing job"
	OpCheckRepositoryExists = "check if repository exists"
)

// ValidationError limits and constants.
const (
	// MaxFieldLength is the maximum allowed length for a field name.
	MaxFieldLength = 255
	// MaxMessageLength is the maximum allowed length for an error message.
	MaxMessageLength = 1000
	// DefaultField is used when field name cannot be determined or is invalid.
	DefaultField = "unknown_field"
	// DefaultMessage is used when error message cannot be determined or is invalid.
	DefaultMessage = "validation failed"
)

// ValidationError represents a validation error with field details.
// It provides robust error handling with automatic input sanitization
// to ensure error messages are always meaningful and safe.
type ValidationError struct {
	Field   string
	Message string
	Value   string
}

// NewValidationError creates a new ValidationError with field and message.
// Input is automatically sanitized to ensure the error is always valid and safe.
// Long field names are truncated, control characters are removed from fields,
// and newlines are preserved in messages.
func NewValidationError(field, message string) ValidationError {
	return ValidationError{
		Field:   sanitizeField(field),
		Message: sanitizeMessage(message),
		Value:   "",
	}
}

// NewValidationErrorWithValue creates a new ValidationError with field, message, and value.
// Input is automatically sanitized to ensure the error is always valid and safe.
// Long field names are truncated, control characters are removed from fields and values,
// and newlines are preserved in messages and values.
func NewValidationErrorWithValue(field, message, value string) ValidationError {
	return ValidationError{
		Field:   sanitizeField(field),
		Message: sanitizeMessage(message),
		Value:   sanitizeValue(value),
	}
}

// Error implements the error interface.
// Returns a consistently formatted error message that is always safe and meaningful,
// even when dealing with edge cases like empty fields or unusual characters.
func (e ValidationError) Error() string {
	// Ensure we always have valid field and message (defensive programming)
	field := e.Field
	if field == "" {
		field = DefaultField
	}

	message := e.Message
	if message == "" {
		message = DefaultMessage
	}

	if e.Value == "" {
		return fmt.Sprintf("validation error on field '%s': %s", field, message)
	}
	return fmt.Sprintf("validation error on field '%s': %s (value: %s)", field, message, e.Value)
}

// ToDTO converts ValidationError to dto.ValidationError.
func (e ValidationError) ToDTO() dto.ValidationError {
	return dto.ValidationError{
		Field:   e.Field,
		Message: e.Message,
		Value:   e.Value,
	}
}

// sanitizeField cleans and validates field names, ensuring they are always usable.
// Long field names are truncated with ellipsis, control characters are removed,
// and empty/invalid fields are replaced with a default value.
func sanitizeField(field string) string {
	field = strings.TrimSpace(field)
	if field == "" {
		return DefaultField
	}

	// Remove control characters from field names
	field = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1 // Remove control characters
		}
		return r
	}, field)

	// Truncate if too long
	if len(field) > MaxFieldLength {
		return field[:MaxFieldLength-3] + "..."
	}

	// Fallback if field became empty after sanitization
	if strings.TrimSpace(field) == "" {
		return DefaultField
	}

	return field
}

// sanitizeMessage cleans and validates error messages, preserving newlines but
// removing other control characters. Long messages are truncated with ellipsis.
func sanitizeMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return DefaultMessage
	}

	// Remove control characters except newlines and carriage returns
	message = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' {
			return -1 // Remove control characters except newlines
		}
		return r
	}, message)

	// Truncate if too long
	if len(message) > MaxMessageLength {
		return message[:MaxMessageLength-3] + "..."
	}

	// Fallback if message became empty after sanitization
	if strings.TrimSpace(message) == "" {
		return DefaultMessage
	}

	return message
}

// sanitizeValue cleans validation error values, removing control characters
// except newlines. Values can be longer than fields/messages since they
// represent user input that may legitimately be long.
func sanitizeValue(value string) string {
	// Allow empty values as they're often legitimate
	if value == "" {
		return ""
	}

	// Remove control characters except newlines and carriage returns
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' {
			return -1
		}
		return r
	}, value)
}
