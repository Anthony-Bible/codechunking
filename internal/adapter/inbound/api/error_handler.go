// Package api provides HTTP error handling with detailed validation responses.
// This error handler improves user experience by returning specific validation errors
// instead of generic "Internal server error" messages for repository URL validation failures.
//
// Example improved response:
//
//	Before: {"error": "INTERNAL_ERROR", "message": "An internal error occurred"}
//	After:  {"error": "INVALID_URL", "message": "invalid repository URL: URL must use http or https scheme"}
package api

import (
	"codechunking/internal/application/common"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/errors/domain"
	"errors"
	"net/http"
	"strings"
)

// ErrorHandler defines methods for handling HTTP errors.
type ErrorHandler interface {
	HandleValidationError(w http.ResponseWriter, r *http.Request, err error)
	HandleServiceError(w http.ResponseWriter, r *http.Request, err error)
}

// ErrorHandlingConfig defines the configuration for handling specific error types.
// It centralizes error response logic to reduce duplication and improve maintainability.
type ErrorHandlingConfig struct {
	LogMessage      string
	ErrorType       string
	HTTPStatus      int
	ErrorCode       dto.ErrorCode
	ResponseMessage string
	UseDetailedMsg  bool
}

// DefaultErrorHandler implements ErrorHandler with standard HTTP error responses.
type DefaultErrorHandler struct {
	errorConfigs map[error]ErrorHandlingConfig
}

// NewDefaultErrorHandler creates a new DefaultErrorHandler with predefined error configurations.
func NewDefaultErrorHandler() ErrorHandler {
	configs := map[error]ErrorHandlingConfig{
		domain.ErrInvalidRepositoryURL: {
			LogMessage:     "Invalid repository URL",
			ErrorType:      "invalid_url",
			HTTPStatus:     http.StatusBadRequest,
			ErrorCode:      dto.ErrorCodeInvalidURL,
			UseDetailedMsg: true,
		},
		domain.ErrRepositoryNotFound: {
			LogMessage:      "Repository not found",
			ErrorType:       "not_found",
			HTTPStatus:      http.StatusNotFound,
			ErrorCode:       dto.ErrorCodeRepositoryNotFound,
			ResponseMessage: "Repository not found",
		},
		domain.ErrRepositoryAlreadyExists: {
			LogMessage:      "Repository already exists",
			ErrorType:       "already_exists",
			HTTPStatus:      http.StatusConflict,
			ErrorCode:       dto.ErrorCodeRepositoryExists,
			ResponseMessage: "Repository already exists",
		},
		domain.ErrRepositoryProcessing: {
			LogMessage:      "Repository currently processing",
			ErrorType:       "processing",
			HTTPStatus:      http.StatusConflict,
			ErrorCode:       dto.ErrorCodeRepositoryProcessing,
			ResponseMessage: "Repository is currently being processed",
		},
		domain.ErrJobNotFound: {
			LogMessage:      "Indexing job not found",
			ErrorType:       "job_not_found",
			HTTPStatus:      http.StatusNotFound,
			ErrorCode:       dto.ErrorCodeJobNotFound,
			ResponseMessage: "Indexing job not found",
		},
	}

	return &DefaultErrorHandler{
		errorConfigs: configs,
	}
}

// extractDetailedErrorMessage extracts the detailed error message from wrapped domain errors.
//
// This function handles the common pattern where domain errors are wrapped with detailed
// validation messages like: "invalid repository URL: URL must use http or https scheme: repository URL is invalid"
//
// It recursively unwraps errors and removes domain error suffixes to return the user-friendly
// detailed message that explains what went wrong.
func extractDetailedErrorMessage(err error) string {
	errorMsg := err.Error()

	// Keep removing the domain error suffix until it's gone (handles nested wrapping)
	for errors.Is(err, domain.ErrInvalidRepositoryURL) {
		suffix := ": " + domain.ErrInvalidRepositoryURL.Error()
		if strings.HasSuffix(errorMsg, suffix) {
			errorMsg = strings.TrimSuffix(errorMsg, suffix)
			// Try to unwrap to see if there's more nesting
			if unwrapped := errors.Unwrap(err); unwrapped != nil {
				err = unwrapped
			} else {
				break
			}
		} else {
			break
		}
	}

	return errorMsg
}

// logError logs an error with consistent context fields.
func (h *DefaultErrorHandler) logError(r *http.Request, message, errorType string, err error) {
	slogger.Error(r.Context(), message, slogger.Fields{
		"error": err.Error(),
		"path":  r.URL.Path,
		"type":  errorType,
	})
}

// isCustomURLValidationError checks if the error is a custom URL validation error.
func (h *DefaultErrorHandler) isCustomURLValidationError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "invalid repository url:")
}

// handleErrorWithConfig handles an error using its configuration.
func (h *DefaultErrorHandler) handleErrorWithConfig(w http.ResponseWriter, r *http.Request, err error, config ErrorHandlingConfig) {
	h.logError(r, config.LogMessage, config.ErrorType, err)

	message := config.ResponseMessage
	if config.UseDetailedMsg {
		message = extractDetailedErrorMessage(err)
	}

	response := h.createErrorResponse(config.ErrorCode, message, nil)
	h.writeErrorResponse(w, r, config.HTTPStatus, response)
}

// createErrorResponse creates a standardized error response.
func (h *DefaultErrorHandler) createErrorResponse(errorCode dto.ErrorCode, message string, details interface{}) dto.ErrorResponse {
	if details != nil {
		return dto.NewErrorResponse(errorCode, message, details)
	}
	return dto.NewErrorResponse(errorCode, message, nil)
}

// HandleValidationError handles validation errors by returning 400 Bad Request.
func (h *DefaultErrorHandler) HandleValidationError(w http.ResponseWriter, r *http.Request, err error) {
	h.logError(r, "Validation error occurred", "validation", err)

	var validationErr common.ValidationError
	if errors.As(err, &validationErr) {
		response := h.createErrorResponse(
			dto.ErrorCodeInvalidRequest,
			"Validation failed",
			dto.ValidationErrorDetails{
				Errors: []dto.ValidationError{validationErr.ToDTO()},
			},
		)
		h.writeErrorResponse(w, r, http.StatusBadRequest, response)
		return
	}

	// Generic validation error
	response := h.createErrorResponse(dto.ErrorCodeInvalidRequest, err.Error(), nil)
	h.writeErrorResponse(w, r, http.StatusBadRequest, response)
}

// HandleServiceError handles service errors by mapping them to appropriate HTTP status codes.
func (h *DefaultErrorHandler) HandleServiceError(w http.ResponseWriter, r *http.Request, err error) {
	// Check for configured domain errors first
	for domainErr, config := range h.errorConfigs {
		if errors.Is(err, domainErr) {
			h.handleErrorWithConfig(w, r, err, config)
			return
		}
	}

	// Handle custom URL validation errors
	if h.isCustomURLValidationError(err) {
		h.logError(r, "Invalid repository URL", "invalid_url", err)
		response := h.createErrorResponse(dto.ErrorCodeInvalidURL, err.Error(), nil)
		h.writeErrorResponse(w, r, http.StatusBadRequest, response)
		return
	}

	// Default internal server error
	defaultConfig := ErrorHandlingConfig{
		LogMessage:      "Internal server error",
		ErrorType:       "internal",
		HTTPStatus:      http.StatusInternalServerError,
		ErrorCode:       dto.ErrorCodeInternalError,
		ResponseMessage: "An internal error occurred",
	}
	h.handleErrorWithConfig(w, r, err, defaultConfig)
}

// writeErrorResponse writes an error response as JSON with correlation ID preservation.
func (h *DefaultErrorHandler) writeErrorResponse(w http.ResponseWriter, r *http.Request, statusCode int, response dto.ErrorResponse) {
	// Preserve correlation ID if present in request
	if correlationID := r.Header.Get("X-Correlation-ID"); correlationID != "" {
		w.Header().Set("X-Correlation-ID", correlationID)
	}

	if err := WriteJSON(w, statusCode, response); err != nil {
		// If JSON writing fails, fall back to plain text error
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		if _, writeErr := w.Write([]byte("Internal Server Error")); writeErr != nil {
			// At this point, we can't really recover, but we can avoid the errcheck violation
			// Log this in a real application
			_ = writeErr
		}
	}
}
