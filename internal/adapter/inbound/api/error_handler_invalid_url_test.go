package api

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/errors/domain"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ErrorResponseTestCase defines a test case for error responses.
type ErrorResponseTestCase struct {
	name           string
	err            error
	expectedStatus int
	expectedCode   string
	expectedMsg    string
	validateFunc   func(t *testing.T, recorder *httptest.ResponseRecorder)
}

// assertErrorResponse validates common error response properties.
func assertErrorResponse(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	expectedStatus int,
	expectedCode, expectedMsg string,
) {
	assert.Equal(t, expectedStatus, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	var response dto.ErrorResponse
	err := parseJSONFromRecorder(recorder, &response)
	require.NoError(t, err)

	assert.Equal(t, expectedCode, response.Error)
	assert.Equal(t, expectedMsg, response.Message)
	assert.NotNil(t, response.Timestamp)
}

// createTestRequest creates a test HTTP request.
func createTestRequest() *http.Request {
	return httptest.NewRequest(http.MethodPost, "/repositories", nil)
}

// createTestRequestWithHeaders creates a test HTTP request with custom headers.
func createTestRequestWithHeaders(headers map[string]string) *http.Request {
	req := createTestRequest()
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return req
}

// Test cases for ErrInvalidRepositoryURL error handling.
func TestDefaultErrorHandler_HandleInvalidRepositoryURL(t *testing.T) {
	tests := []ErrorResponseTestCase{
		{
			name: "invalid_repository_url_returns_400_with_detailed_message",
			err: fmt.Errorf(
				"invalid repository URL: URL must use http or https scheme: %w",
				domain.ErrInvalidRepositoryURL,
			),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   string(dto.ErrorCodeInvalidURL),
			expectedMsg:    "invalid repository URL: URL must use http or https scheme",
		},
		{
			name: "wrapped_invalid_repository_url_error_returns_400",
			err: fmt.Errorf(
				"validation failed: %w",
				fmt.Errorf(
					"invalid repository URL: URL must use http or https scheme: %w",
					domain.ErrInvalidRepositoryURL,
				),
			),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   string(dto.ErrorCodeInvalidURL),
			expectedMsg:    "validation failed: invalid repository URL: URL must use http or https scheme",
		},
		{
			name:           "invalid_url_with_custom_message_returns_400",
			err:            errors.New("invalid repository URL: unsupported host example.com"),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   string(dto.ErrorCodeInvalidURL),
			expectedMsg:    "invalid repository URL: unsupported host example.com",
		},
		{
			name:           "invalid_url_missing_path_returns_400",
			err:            errors.New("invalid repository URL: missing repository path in URL"),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   string(dto.ErrorCodeInvalidURL),
			expectedMsg:    "invalid repository URL: missing repository path in URL",
		},
		{
			name:           "invalid_url_private_git_returns_400",
			err:            errors.New("invalid repository URL: private Git repositories not supported"),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   string(dto.ErrorCodeInvalidURL),
			expectedMsg:    "invalid repository URL: private Git repositories not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			errorHandler := NewDefaultErrorHandler()
			req := createTestRequest()
			recorder := httptest.NewRecorder()

			// Execute
			errorHandler.HandleServiceError(recorder, req, tt.err)

			// Assert
			assertErrorResponse(t, recorder, tt.expectedStatus, tt.expectedCode, tt.expectedMsg)

			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder)
			}
		})
	}
}

// CorrelationIDTestCase defines a test case for correlation ID preservation.
type CorrelationIDTestCase struct {
	name           string
	err            error
	expectedStatus int
	setupHeaders   map[string]string
}

// Test correlation ID preservation in error responses.
func TestDefaultErrorHandler_CorrelationIDPreservation(t *testing.T) {
	tests := []CorrelationIDTestCase{
		{
			name: "correlation_id_preserved_in_invalid_url_error",
			err: fmt.Errorf(
				"invalid repository URL: URL must use http or https scheme: %w",
				domain.ErrInvalidRepositoryURL,
			),
			expectedStatus: http.StatusBadRequest,
			setupHeaders:   map[string]string{"X-Correlation-ID": "test-correlation-123"},
		},
		{
			name:           "correlation_id_preserved_in_validation_error",
			err:            errors.New("invalid repository URL: URL must use http or https scheme"),
			expectedStatus: http.StatusBadRequest,
			setupHeaders:   map[string]string{"X-Correlation-ID": "test-correlation-456"},
		},
		{
			name:           "correlation_id_preserved_in_service_error",
			err:            domain.ErrRepositoryNotFound,
			expectedStatus: http.StatusNotFound,
			setupHeaders:   map[string]string{"X-Correlation-ID": "test-correlation-789"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			errorHandler := NewDefaultErrorHandler()
			req := createTestRequestWithHeaders(tt.setupHeaders)
			recorder := httptest.NewRecorder()

			// Execute
			errorHandler.HandleServiceError(recorder, req, tt.err)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)
			assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

			// Correlation ID should be echoed back in response headers
			assert.Equal(t, tt.setupHeaders["X-Correlation-ID"], recorder.Header().Get("X-Correlation-ID"))
		})
	}
}

// Test different URL validation scenarios.
func TestDefaultErrorHandler_URLValidationScenarios(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectedError  string
		expectedStatus int
	}{
		{
			name:           "invalid_scheme_ftp_returns_400",
			url:            "ftp://github.com/user/repo",
			expectedError:  "invalid repository URL: URL must use http or https scheme",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing_scheme_returns_400",
			url:            "github.com/user/repo",
			expectedError:  "invalid repository URL: URL must use http or https scheme",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unsupported_host_returns_400",
			url:            "https://unsupported-git-host.com/user/repo",
			expectedError:  "invalid repository URL: unsupported host unsupported-git-host.com",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing_path_returns_400",
			url:            "https://github.com",
			expectedError:  "invalid repository URL: missing repository path in URL",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty_url_returns_400",
			url:            "",
			expectedError:  "invalid repository URL: URL cannot be empty",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid_url_format_returns_400",
			url:            "not-a-url",
			expectedError:  "invalid repository URL: invalid URL format",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "private_git_url_returns_400",
			url:            "git@github.com:user/repo.git",
			expectedError:  "invalid repository URL: private Git repositories not supported",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "file_protocol_returns_400",
			url:            "file:///path/to/repo",
			expectedError:  "invalid repository URL: file protocol not supported",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			errorHandler := NewDefaultErrorHandler()
			req := httptest.NewRequest(http.MethodPost, "/repositories", nil)
			recorder := httptest.NewRecorder()

			// Create error that would be returned by URL validation
			err := errors.New(tt.expectedError)

			// Execute
			errorHandler.HandleServiceError(recorder, req, err)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			var response dto.ErrorResponse
			parseErr := parseJSONFromRecorder(recorder, &response)
			require.NoError(t, parseErr)

			assert.Equal(t, string(dto.ErrorCodeInvalidURL), response.Error)
			assert.Equal(t, tt.expectedError, response.Message)
			assert.NotNil(t, response.Timestamp)
		})
	}
}

// Test error response format consistency.
func TestDefaultErrorHandler_ErrorResponseFormat(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedFields []string
	}{
		{
			name:           "invalid_url_error_has_all_required_fields",
			err:            domain.ErrInvalidRepositoryURL,
			expectedStatus: http.StatusBadRequest,
			expectedFields: []string{"error", "message", "timestamp"},
		},
		{
			name:           "validation_error_has_details_field",
			err:            errors.New("invalid repository URL: URL must use http or https scheme"),
			expectedStatus: http.StatusBadRequest,
			expectedFields: []string{"error", "message", "timestamp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			errorHandler := NewDefaultErrorHandler()
			req := httptest.NewRequest(http.MethodPost, "/repositories", nil)
			recorder := httptest.NewRecorder()

			// Execute
			errorHandler.HandleServiceError(recorder, req, tt.err)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			var response dto.ErrorResponse
			err := parseJSONFromRecorder(recorder, &response)
			require.NoError(t, err)

			// Check that response is valid JSON
			jsonBytes, err := json.Marshal(response)
			assert.NoError(t, err)

			var unmarshaled map[string]interface{}
			err = json.Unmarshal(jsonBytes, &unmarshaled)
			require.NoError(t, err)

			// Check required fields are present
			for _, field := range tt.expectedFields {
				_, exists := unmarshaled[field]
				assert.True(t, exists, "Field %s should be present in error response", field)
			}

			// Check error code is valid
			assert.NotEmpty(t, response.Error)
			assert.NotEmpty(t, response.Message)
			assert.NotZero(t, response.Timestamp)
		})
	}
}

// Test error handler behavior with different error types.
func TestDefaultErrorHandler_ErrorTypeHandling(t *testing.T) {
	tests := []struct {
		name                 string
		err                  error
		expectedStatus       int
		expectedErrorCode    string
		isURLValidationError bool
	}{
		{
			name: "domain_invalid_url_error_handled_correctly",
			err: fmt.Errorf(
				"invalid repository URL: custom validation message: %w",
				domain.ErrInvalidRepositoryURL,
			),
			expectedStatus:       http.StatusBadRequest,
			expectedErrorCode:    string(dto.ErrorCodeInvalidURL),
			isURLValidationError: true,
		},
		{
			name:                 "custom_invalid_url_error_handled_correctly",
			err:                  errors.New("invalid repository URL: custom validation message"),
			expectedStatus:       http.StatusBadRequest,
			expectedErrorCode:    string(dto.ErrorCodeInvalidURL),
			isURLValidationError: true,
		},
		{
			name:                 "non_url_error_returns_500",
			err:                  errors.New("some other error"),
			expectedStatus:       http.StatusInternalServerError,
			expectedErrorCode:    string(dto.ErrorCodeInternalError),
			isURLValidationError: false,
		},
		{
			name:                 "repository_not_found_returns_404",
			err:                  domain.ErrRepositoryNotFound,
			expectedStatus:       http.StatusNotFound,
			expectedErrorCode:    string(dto.ErrorCodeRepositoryNotFound),
			isURLValidationError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			errorHandler := NewDefaultErrorHandler()
			req := httptest.NewRequest(http.MethodPost, "/repositories", nil)
			recorder := httptest.NewRecorder()

			// Execute
			errorHandler.HandleServiceError(recorder, req, tt.err)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			var response dto.ErrorResponse
			err := parseJSONFromRecorder(recorder, &response)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedErrorCode, response.Error)

			// For URL validation errors, ensure the message starts with "invalid repository URL:"
			if tt.isURLValidationError {
				assert.Contains(t, response.Message, "invalid repository URL:")
			}
		})
	}
}
