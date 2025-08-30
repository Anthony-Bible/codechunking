package dto

import "time"

// ErrorResponse represents a standardized error response.
type ErrorResponse struct {
	Error     string      `json:"error"`
	Message   string      `json:"message"`
	Details   interface{} `json:"details,omitempty"`
	Timestamp time.Time   `json:"timestamp,omitempty"`
}

// ErrorCode represents standard error codes.
type ErrorCode string

const (
	// ErrorCodeInvalidRequest indicates that the request contains invalid parameters or data.
	ErrorCodeInvalidRequest ErrorCode = "INVALID_REQUEST"
	// ErrorCodeInvalidURL indicates that the provided repository URL is malformed or invalid.
	ErrorCodeInvalidURL ErrorCode = "INVALID_URL"
	// ErrorCodeRepositoryExists indicates that a repository with the same URL already exists.
	ErrorCodeRepositoryExists ErrorCode = "REPOSITORY_ALREADY_EXISTS"
	// ErrorCodeRepositoryNotFound indicates that the requested repository could not be found.
	ErrorCodeRepositoryNotFound ErrorCode = "REPOSITORY_NOT_FOUND"
	// ErrorCodeRepositoryProcessing indicates that the repository is currently being processed.
	ErrorCodeRepositoryProcessing ErrorCode = "REPOSITORY_PROCESSING"
	// ErrorCodeJobNotFound indicates that the requested indexing job could not be found.
	ErrorCodeJobNotFound ErrorCode = "JOB_NOT_FOUND"
	// ErrorCodeInternalError indicates an unexpected internal server error occurred.
	ErrorCodeInternalError ErrorCode = "INTERNAL_ERROR"
	// ErrorCodeServiceUnavailable indicates that the service is temporarily unavailable.
	ErrorCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
)

// NewErrorResponse creates a new error response.
func NewErrorResponse(code ErrorCode, message string, details interface{}) ErrorResponse {
	return ErrorResponse{
		Error:     string(code),
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// ValidationError represents a validation error with field details.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// ValidationErrorDetails represents multiple validation errors.
type ValidationErrorDetails struct {
	Errors []ValidationError `json:"errors"`
}
