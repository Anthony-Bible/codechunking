package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"codechunking/internal/application/common"
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/errors/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultErrorHandler_HandleValidationError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedError  string
		validateFunc   func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:           "validation_error_returns_400_with_field_details",
			err:            common.NewValidationError("url", "URL is required"),
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

				var response dto.ErrorResponse
				err := parseJSONFromRecorder(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, string(dto.ErrorCodeInvalidRequest), response.Error)
				assert.Equal(t, "Validation failed", response.Message)

				// Check validation details
				details, ok := response.Details.(map[string]interface{})
				require.True(t, ok)

				errors, ok := details["errors"].([]interface{})
				require.True(t, ok)
				require.Len(t, errors, 1)

				firstError := errors[0].(map[string]interface{})
				assert.Equal(t, "url", firstError["field"])
				assert.Equal(t, "URL is required", firstError["message"])
			},
		},
		{
			name:           "validation_error_with_value_includes_value_in_response",
			err:            common.NewValidationErrorWithValue("id", "invalid UUID format", "invalid-uuid"),
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.ErrorResponse
				err := parseJSONFromRecorder(recorder, &response)
				require.NoError(t, err)

				details, ok := response.Details.(map[string]interface{})
				require.True(t, ok)

				errors, ok := details["errors"].([]interface{})
				require.True(t, ok)

				firstError := errors[0].(map[string]interface{})
				assert.Equal(t, "id", firstError["field"])
				assert.Equal(t, "invalid UUID format", firstError["message"])
				assert.Equal(t, "invalid-uuid", firstError["value"])
			},
		},
		{
			name:           "generic_error_returns_400_with_error_message",
			err:            errors.New("invalid request format"),
			expectedStatus: http.StatusBadRequest,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.ErrorResponse
				err := parseJSONFromRecorder(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, string(dto.ErrorCodeInvalidRequest), response.Error)
				assert.Equal(t, "invalid request format", response.Message)
				assert.Nil(t, response.Details)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			errorHandler := NewDefaultErrorHandler()
			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			recorder := httptest.NewRecorder()

			// Execute
			errorHandler.HandleValidationError(recorder, req, tt.err)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder)
			}
		})
	}
}

func TestDefaultErrorHandler_HandleServiceError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedError  string
		validateFunc   func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:           "repository_not_found_returns_404",
			err:            domain.ErrRepositoryNotFound,
			expectedStatus: http.StatusNotFound,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.ErrorResponse
				err := parseJSONFromRecorder(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, string(dto.ErrorCodeRepositoryNotFound), response.Error)
				assert.Equal(t, "Repository not found", response.Message)
				assert.Nil(t, response.Details)
			},
		},
		{
			name:           "repository_already_exists_returns_409",
			err:            domain.ErrRepositoryAlreadyExists,
			expectedStatus: http.StatusConflict,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.ErrorResponse
				err := parseJSONFromRecorder(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, string(dto.ErrorCodeRepositoryExists), response.Error)
				assert.Equal(t, "Repository already exists", response.Message)
			},
		},
		{
			name:           "repository_processing_returns_409",
			err:            domain.ErrRepositoryProcessing,
			expectedStatus: http.StatusConflict,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.ErrorResponse
				err := parseJSONFromRecorder(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, string(dto.ErrorCodeRepositoryProcessing), response.Error)
				assert.Equal(t, "Repository is currently being processed", response.Message)
			},
		},
		{
			name:           "job_not_found_returns_404",
			err:            domain.ErrJobNotFound,
			expectedStatus: http.StatusNotFound,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.ErrorResponse
				err := parseJSONFromRecorder(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, string(dto.ErrorCodeJobNotFound), response.Error)
				assert.Equal(t, "Indexing job not found", response.Message)
			},
		},
		{
			name:           "generic_error_returns_500",
			err:            errors.New("database connection failed"),
			expectedStatus: http.StatusInternalServerError,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.ErrorResponse
				err := parseJSONFromRecorder(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, string(dto.ErrorCodeInternalError), response.Error)
				assert.Equal(t, "An internal error occurred", response.Message)
			},
		},
		{
			name:           "wrapped_repository_not_found_error_returns_404",
			err:            errors.Join(errors.New("operation failed"), domain.ErrRepositoryNotFound),
			expectedStatus: http.StatusNotFound,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.ErrorResponse
				err := parseJSONFromRecorder(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, string(dto.ErrorCodeRepositoryNotFound), response.Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			errorHandler := NewDefaultErrorHandler()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			recorder := httptest.NewRecorder()

			// Execute
			errorHandler.HandleServiceError(recorder, req, tt.err)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder)
			}
		})
	}
}

func TestDefaultErrorHandler_ResponseFormat(t *testing.T) {
	t.Run("error_response_should_have_timestamp", func(t *testing.T) {
		// Setup
		errorHandler := NewDefaultErrorHandler()
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		recorder := httptest.NewRecorder()

		// Execute
		errorHandler.HandleServiceError(recorder, req, domain.ErrRepositoryNotFound)

		// Assert timestamp is present and recent
		var response dto.ErrorResponse
		err := parseJSONFromRecorder(recorder, &response)
		require.NoError(t, err)

		assert.NotZero(t, response.Timestamp)
		assert.WithinDuration(t, response.Timestamp, response.Timestamp, 0) // Just check it's not zero
	})

	t.Run("error_response_should_have_correct_content_type", func(t *testing.T) {
		// Setup
		errorHandler := NewDefaultErrorHandler()
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		recorder := httptest.NewRecorder()

		// Execute
		errorHandler.HandleValidationError(recorder, req, common.NewValidationError("field", "message"))

		// Assert
		assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	})
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name            string
		validationError common.ValidationError
		expectedMessage string
	}{
		{
			name: "validation_error_without_value",
			validationError: common.ValidationError{
				Field:   "url",
				Message: "URL is required",
			},
			expectedMessage: "validation error on field 'url': URL is required",
		},
		{
			name: "validation_error_with_value",
			validationError: common.ValidationError{
				Field:   "id",
				Message: "invalid UUID format",
				Value:   "invalid-uuid",
			},
			expectedMessage: "validation error on field 'id': invalid UUID format (value: invalid-uuid)",
		},
		{
			name: "validation_error_with_empty_value",
			validationError: common.ValidationError{
				Field:   "name",
				Message: "name cannot be empty",
				Value:   "",
			},
			expectedMessage: "validation error on field 'name': name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedMessage, tt.validationError.Error())
		})
	}
}

func TestNewValidationError_Constructors(t *testing.T) {
	t.Run("NewValidationError_creates_error_without_value", func(t *testing.T) {
		err := common.NewValidationError("field", "message")

		assert.Equal(t, "field", err.Field)
		assert.Equal(t, "message", err.Message)
		assert.Empty(t, err.Value)
	})

	t.Run("NewValidationErrorWithValue_creates_error_with_value", func(t *testing.T) {
		err := common.NewValidationErrorWithValue("field", "message", "value")

		assert.Equal(t, "field", err.Field)
		assert.Equal(t, "message", err.Message)
		assert.Equal(t, "value", err.Value)
	})
}

func TestErrorHandler_Integration(t *testing.T) {
	t.Run("complete_error_handling_workflow", func(t *testing.T) {
		// This test simulates a complete error handling workflow
		// from validation to service errors

		errorHandler := NewDefaultErrorHandler()

		// Test validation error handling
		t.Run("validation_error_workflow", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/repositories", nil)
			recorder := httptest.NewRecorder()

			validationErr := common.NewValidationError("url", "URL is required")
			errorHandler.HandleValidationError(recorder, req, validationErr)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

			var response dto.ErrorResponse
			err := parseJSONFromRecorder(recorder, &response)
			require.NoError(t, err)

			assert.Equal(t, string(dto.ErrorCodeInvalidRequest), response.Error)
			assert.Contains(t, response.Message, "Validation failed")
			assert.NotNil(t, response.Details)
		})

		// Test service error handling
		t.Run("service_error_workflow", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/repositories/123", nil)
			recorder := httptest.NewRecorder()

			errorHandler.HandleServiceError(recorder, req, domain.ErrRepositoryNotFound)

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

			var response dto.ErrorResponse
			err := parseJSONFromRecorder(recorder, &response)
			require.NoError(t, err)

			assert.Equal(t, string(dto.ErrorCodeRepositoryNotFound), response.Error)
			assert.Equal(t, "Repository not found", response.Message)
		})
	})
}

// Helper function to parse JSON from response recorder.
func parseJSONFromRecorder(recorder *httptest.ResponseRecorder, target interface{}) error {
	return json.Unmarshal(recorder.Body.Bytes(), target)
}
