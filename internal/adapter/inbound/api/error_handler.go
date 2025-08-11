package api

import (
	"errors"
	"net/http"

	"codechunking/internal/application/common"
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/errors/domain"
)

// ErrorHandler defines methods for handling HTTP errors
type ErrorHandler interface {
	HandleValidationError(w http.ResponseWriter, r *http.Request, err error)
	HandleServiceError(w http.ResponseWriter, r *http.Request, err error)
}

// DefaultErrorHandler implements ErrorHandler with standard HTTP error responses
type DefaultErrorHandler struct{}

// NewDefaultErrorHandler creates a new DefaultErrorHandler
func NewDefaultErrorHandler() ErrorHandler {
	return &DefaultErrorHandler{}
}

// HandleValidationError handles validation errors by returning 400 Bad Request
func (h *DefaultErrorHandler) HandleValidationError(w http.ResponseWriter, r *http.Request, err error) {
	var validationErr common.ValidationError
	if errors.As(err, &validationErr) {
		response := dto.NewErrorResponse(
			dto.ErrorCodeInvalidRequest,
			"Validation failed",
			dto.ValidationErrorDetails{
				Errors: []dto.ValidationError{validationErr.ToDTO()},
			},
		)
		h.writeErrorResponse(w, http.StatusBadRequest, response)
		return
	}

	// Generic validation error
	response := dto.NewErrorResponse(
		dto.ErrorCodeInvalidRequest,
		err.Error(),
		nil,
	)
	h.writeErrorResponse(w, http.StatusBadRequest, response)
}

// HandleServiceError handles service errors by mapping them to appropriate HTTP status codes
func (h *DefaultErrorHandler) HandleServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrRepositoryNotFound):
		response := dto.NewErrorResponse(
			dto.ErrorCodeRepositoryNotFound,
			"Repository not found",
			nil,
		)
		h.writeErrorResponse(w, http.StatusNotFound, response)

	case errors.Is(err, domain.ErrRepositoryAlreadyExists):
		response := dto.NewErrorResponse(
			dto.ErrorCodeRepositoryExists,
			"Repository already exists",
			nil,
		)
		h.writeErrorResponse(w, http.StatusConflict, response)

	case errors.Is(err, domain.ErrRepositoryProcessing):
		response := dto.NewErrorResponse(
			dto.ErrorCodeRepositoryProcessing,
			"Repository is currently being processed",
			nil,
		)
		h.writeErrorResponse(w, http.StatusConflict, response)

	case errors.Is(err, domain.ErrJobNotFound):
		response := dto.NewErrorResponse(
			dto.ErrorCodeJobNotFound,
			"Indexing job not found",
			nil,
		)
		h.writeErrorResponse(w, http.StatusNotFound, response)

	default:
		// Generic internal server error
		response := dto.NewErrorResponse(
			dto.ErrorCodeInternalError,
			"An internal error occurred",
			nil,
		)
		h.writeErrorResponse(w, http.StatusInternalServerError, response)
	}
}

// writeErrorResponse writes an error response as JSON
func (h *DefaultErrorHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, response dto.ErrorResponse) {
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
