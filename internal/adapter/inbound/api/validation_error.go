package api

import "fmt"

// ValidationError represents a validation error with field details
type ValidationError struct {
	Field   string
	Message string
	Value   string
}

// Error implements the error interface
func (e ValidationError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("validation error on field '%s': %s (value: %s)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// NewValidationError creates a new ValidationError
func NewValidationError(field, message string) ValidationError {
	return ValidationError{
		Field:   field,
		Message: message,
	}
}

// NewValidationErrorWithValue creates a new ValidationError with a value
func NewValidationErrorWithValue(field, message, value string) ValidationError {
	return ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	}
}
