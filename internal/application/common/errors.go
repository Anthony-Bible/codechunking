package common

import "fmt"

// ServiceError represents a service-level error with context
type ServiceError struct {
	Operation string
	Cause     error
}

// Error implements the error interface
func (e ServiceError) Error() string {
	return fmt.Sprintf("failed to %s: %v", e.Operation, e.Cause)
}

// Unwrap returns the underlying error
func (e ServiceError) Unwrap() error {
	return e.Cause
}

// WrapServiceError wraps an error with service operation context
func WrapServiceError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return ServiceError{
		Operation: operation,
		Cause:     err,
	}
}

// Common error operations for consistent messaging
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
