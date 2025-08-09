package common

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ValidRepositoryStatuses defines all valid repository statuses
var ValidRepositoryStatuses = map[string]bool{
	"":           true, // Empty means no filter
	"pending":    true,
	"cloning":    true,
	"processing": true,
	"completed":  true,
	"failed":     true,
	"archived":   true,
}

// ValidJobStatuses defines all valid job statuses
var ValidJobStatuses = map[string]bool{
	"pending":   true,
	"running":   true,
	"completed": true,
	"failed":    true,
	"cancelled": true,
}

// ValidationError represents a validation error with context
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface
func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// ValidateRepositoryURL validates that a repository URL is not empty
func ValidateRepositoryURL(url string) error {
	if strings.TrimSpace(url) == "" {
		return ValidationError{Field: "url", Message: "URL is required"}
	}
	return nil
}

// ValidateRepositoryName validates repository name constraints
func ValidateRepositoryName(name *string) error {
	if name != nil && len(*name) > 255 {
		return ValidationError{Field: "name", Message: "name exceeds maximum length of 255 characters"}
	}
	return nil
}

// ValidateRepositoryStatus validates repository status
func ValidateRepositoryStatus(status string) error {
	if !ValidRepositoryStatuses[status] {
		return ValidationError{Field: "status", Message: fmt.Sprintf("invalid status: %s", status)}
	}
	return nil
}

// ValidateJobStatus validates job status
func ValidateJobStatus(status string) error {
	if !ValidJobStatuses[status] {
		return ValidationError{Field: "status", Message: fmt.Sprintf("invalid status: %s", status)}
	}
	return nil
}

// ValidateUUID validates that a UUID is not nil/empty
func ValidateUUID(id uuid.UUID, fieldName string) error {
	if id == uuid.Nil {
		return ValidationError{Field: fieldName, Message: fmt.Sprintf("%s is required", fieldName)}
	}
	return nil
}

// ValidatePaginationLimit validates pagination limit constraints
func ValidatePaginationLimit(limit int, maxLimit int, fieldName string) error {
	if limit > maxLimit {
		return ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("limit exceeds maximum of %d", maxLimit),
		}
	}
	return nil
}
