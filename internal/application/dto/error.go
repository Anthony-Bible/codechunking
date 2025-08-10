package dto

import "time"

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error     string      `json:"error"`
	Message   string      `json:"message"`
	Details   interface{} `json:"details,omitempty"`
	Timestamp time.Time   `json:"timestamp,omitempty"`
}

// ErrorCode represents standard error codes
type ErrorCode string

const (
	ErrorCodeInvalidRequest       ErrorCode = "INVALID_REQUEST"
	ErrorCodeInvalidURL           ErrorCode = "INVALID_URL"
	ErrorCodeRepositoryExists     ErrorCode = "REPOSITORY_ALREADY_EXISTS"
	ErrorCodeRepositoryNotFound   ErrorCode = "REPOSITORY_NOT_FOUND"
	ErrorCodeRepositoryProcessing ErrorCode = "REPOSITORY_PROCESSING"
	ErrorCodeJobNotFound          ErrorCode = "JOB_NOT_FOUND"
	ErrorCodeInternalError        ErrorCode = "INTERNAL_ERROR"
	ErrorCodeServiceUnavailable   ErrorCode = "SERVICE_UNAVAILABLE"
)

// NewErrorResponse creates a new error response
func NewErrorResponse(code ErrorCode, message string, details interface{}) ErrorResponse {
	return ErrorResponse{
		Error:     string(code),
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// ValidationError represents a validation error with field details
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// ValidationErrorDetails represents multiple validation errors
type ValidationErrorDetails struct {
	Errors []ValidationError `json:"errors"`
}
