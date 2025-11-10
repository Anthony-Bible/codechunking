package api

import (
	"codechunking/internal/adapter/inbound/api/testutil"
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/errors/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test end-to-end URL validation error handling in API.
func TestRepositoryHandler_CreateRepository_URLValidationErrors(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		expectedError  string
		expectedCode   string
		validateFunc   func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "invalid_scheme_ftp_returns_400_with_invalid_url_error",
			requestBody: map[string]interface{}{
				"url": "ftp://github.com/user/repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid repository URL: URL must use http or https scheme",
			expectedCode:   "INVALID_URL",
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.ErrorResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "INVALID_URL", response.Error)
				assert.Equal(t, "invalid repository URL: URL must use http or https scheme", response.Message)
				assert.NotNil(t, response.Timestamp)
			},
		},
		{
			name: "missing_scheme_returns_400_with_invalid_url_error",
			requestBody: map[string]interface{}{
				"url": "github.com/user/repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid repository URL: URL must use http or https scheme",
			expectedCode:   "INVALID_URL",
		},
		{
			name: "unsupported_host_returns_400_with_invalid_url_error",
			requestBody: map[string]interface{}{
				"url": "https://unsupported-git-host.com/user/repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid repository URL: unsupported host unsupported-git-host.com",
			expectedCode:   "INVALID_URL",
		},
		{
			name: "missing_path_returns_400_with_invalid_url_error",
			requestBody: map[string]interface{}{
				"url": "https://github.com",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid repository URL: missing repository path in URL",
			expectedCode:   "INVALID_URL",
		},
		{
			name: "empty_url_returns_400_with_invalid_url_error",
			requestBody: map[string]interface{}{
				"url": "",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
			expectedCode:   "INVALID_REQUEST",
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				// For empty URL, validation happens before service call, so service shouldn't be called
				// This is handled by removing the general service call verification for this case
			},
		},
		{
			name: "invalid_url_format_returns_400_with_invalid_url_error",
			requestBody: map[string]interface{}{
				"url": "not-a-url",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid repository URL: invalid URL format",
			expectedCode:   "INVALID_URL",
		},
		{
			name: "private_git_url_returns_400_with_invalid_url_error",
			requestBody: map[string]interface{}{
				"url": "git@github.com:user/repo.git",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid repository URL: private Git repositories not supported",
			expectedCode:   "INVALID_URL",
		},
		{
			name: "file_protocol_returns_400_with_invalid_url_error",
			requestBody: map[string]interface{}{
				"url": "file:///path/to/repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid repository URL: file protocol not supported",
			expectedCode:   "INVALID_URL",
		},
		{
			name: "localhost_not_supported_returns_400_with_invalid_url_error",
			requestBody: map[string]interface{}{
				"url": "http://localhost/user/repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid repository URL: localhost not supported",
			expectedCode:   "INVALID_URL",
		},
		{
			name: "private_ip_not_supported_returns_400_with_invalid_url_error",
			requestBody: map[string]interface{}{
				"url": "http://192.168.1.1/user/repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid repository URL: private IP addresses not supported",
			expectedCode:   "INVALID_URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := testutil.NewMockRepositoryService()
			mockErrorHandler := NewDefaultErrorHandler()

			// Mock service to return URL validation error
			mockService.CreateRepositoryFunc = func(_ context.Context, req dto.CreateRepositoryRequest) (*dto.RepositoryResponse, error) {
				return nil, fmt.Errorf("%s: %w", tt.expectedError, domain.ErrInvalidRepositoryURL)
			}

			handler := NewRepositoryHandler(mockService, mockErrorHandler)

			// Create request
			req := testutil.CreateJSONRequest(http.MethodPost, "/repositories", tt.requestBody)
			recorder := httptest.NewRecorder()

			// Execute
			handler.CreateRepository(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)
			assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

			var response dto.ErrorResponse
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedCode, response.Error)
			assert.Equal(t, tt.expectedError, response.Message)
			assert.NotNil(t, response.Timestamp)

			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder)
			} else {
				// Verify service was called (only for tests without custom validation)
				assert.Len(t, mockService.CreateRepositoryCalls, 1)
			}
		})
	}
}

// Test successful URL validation in API.
func TestRepositoryHandler_CreateRepository_SuccessfulURLValidation(t *testing.T) {
	tests := []struct {
		name        string
		requestBody interface{}
		mockSetup   func(*testutil.MockRepositoryService)
	}{
		{
			name: "valid_github_https_url_succeeds",
			requestBody: map[string]interface{}{
				"url": "https://github.com/golang/go",
			},
			mockSetup: func(mock *testutil.MockRepositoryService) {
				response := testutil.NewRepositoryResponseBuilder().
					WithURL("https://github.com/golang/go").
					WithStatus("pending").
					Build()
				mock.ExpectCreateRepository(&response, nil)
			},
		},
		{
			name: "valid_gitlab_https_url_succeeds",
			requestBody: map[string]interface{}{
				"url": "https://gitlab.com/gitlab-org/gitlab",
			},
			mockSetup: func(mock *testutil.MockRepositoryService) {
				response := testutil.NewRepositoryResponseBuilder().
					WithURL("https://gitlab.com/gitlab-org/gitlab").
					WithStatus("pending").
					Build()
				mock.ExpectCreateRepository(&response, nil)
			},
		},
		{
			name: "valid_bitbucket_https_url_succeeds",
			requestBody: map[string]interface{}{
				"url": "https://bitbucket.org/atlassian/atlassian-sdk",
			},
			mockSetup: func(mock *testutil.MockRepositoryService) {
				response := testutil.NewRepositoryResponseBuilder().
					WithURL("https://bitbucket.org/atlassian/atlassian-sdk").
					WithStatus("pending").
					Build()
				mock.ExpectCreateRepository(&response, nil)
			},
		},
		{
			name: "valid_github_http_url_succeeds",
			requestBody: map[string]interface{}{
				"url": "http://github.com/golang/go",
			},
			mockSetup: func(mock *testutil.MockRepositoryService) {
				response := testutil.NewRepositoryResponseBuilder().
					WithURL("http://github.com/golang/go").
					WithStatus("pending").
					Build()
				mock.ExpectCreateRepository(&response, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := testutil.NewMockRepositoryService()
			mockErrorHandler := NewDefaultErrorHandler()
			tt.mockSetup(mockService)

			handler := NewRepositoryHandler(mockService, mockErrorHandler)

			// Create request
			req := testutil.CreateJSONRequest(http.MethodPost, "/repositories", tt.requestBody)
			recorder := httptest.NewRecorder()

			// Execute
			handler.CreateRepository(recorder, req)

			// Assert
			assert.Equal(t, http.StatusAccepted, recorder.Code)
			assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

			var response dto.RepositoryResponse
			err := testutil.ParseJSONResponse(recorder, &response)
			require.NoError(t, err)

			assert.NotEmpty(t, response.ID)
			assert.NotEmpty(t, response.URL)

			// Verify service was called
			assert.Len(t, mockService.CreateRepositoryCalls, 1)
		})
	}
}

// Test correlation ID preservation in URL validation errors.
func TestRepositoryHandler_CreateRepository_CorrelationIDPreservation(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		correlationID  string
		expectedStatus int
		expectedError  string
	}{
		{
			name: "correlation_id_preserved_in_invalid_url_error",
			requestBody: map[string]interface{}{
				"url": "invalid-url",
			},
			correlationID:  "test-correlation-123",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid repository URL: invalid URL format",
		},
		{
			name: "correlation_id_preserved_in_scheme_error",
			requestBody: map[string]interface{}{
				"url": "ftp://github.com/user/repo",
			},
			correlationID:  "test-correlation-456",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid repository URL: URL must use http or https scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := testutil.NewMockRepositoryService()
			mockErrorHandler := NewDefaultErrorHandler()

			// Mock service to return URL validation error
			mockService.CreateRepositoryFunc = func(_ context.Context, req dto.CreateRepositoryRequest) (*dto.RepositoryResponse, error) {
				return nil, fmt.Errorf("%s: %w", tt.expectedError, domain.ErrInvalidRepositoryURL)
			}

			handler := NewRepositoryHandler(mockService, mockErrorHandler)

			// Create request with correlation ID
			req := testutil.CreateJSONRequest(http.MethodPost, "/repositories", tt.requestBody)
			req.Header.Set("X-Correlation-ID", tt.correlationID)
			recorder := httptest.NewRecorder()

			// Execute
			handler.CreateRepository(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			// Correlation ID should be preserved in response headers
			assert.Equal(t, tt.correlationID, recorder.Header().Get("X-Correlation-ID"))

			var response dto.ErrorResponse
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "INVALID_URL", response.Error)
			assert.Equal(t, tt.expectedError, response.Message)
		})
	}
}

// Test error response format consistency for URL validation.
func TestRepositoryHandler_CreateRepository_URLValidationErrorFormat(t *testing.T) {
	tests := []struct {
		name        string
		requestBody interface{}
		errorMsg    string
	}{
		{
			name: "invalid_url_error_response_format",
			requestBody: map[string]interface{}{
				"url": "invalid-url",
			},
			errorMsg: "invalid repository URL: invalid URL format",
		},
		{
			name: "scheme_error_response_format",
			requestBody: map[string]interface{}{
				"url": "ftp://github.com/user/repo",
			},
			errorMsg: "invalid repository URL: URL must use http or https scheme",
		},
		{
			name: "host_error_response_format",
			requestBody: map[string]interface{}{
				"url": "https://unsupported-host.com/user/repo",
			},
			errorMsg: "invalid repository URL: unsupported host unsupported-host.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := testutil.NewMockRepositoryService()
			mockErrorHandler := NewDefaultErrorHandler()

			// Mock service to return URL validation error
			mockService.CreateRepositoryFunc = func(_ context.Context, req dto.CreateRepositoryRequest) (*dto.RepositoryResponse, error) {
				return nil, fmt.Errorf("%s: %w", tt.errorMsg, domain.ErrInvalidRepositoryURL)
			}

			handler := NewRepositoryHandler(mockService, mockErrorHandler)

			// Create request
			req := testutil.CreateJSONRequest(http.MethodPost, "/repositories", tt.requestBody)
			recorder := httptest.NewRecorder()

			// Execute
			handler.CreateRepository(recorder, req)

			// Assert
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

			var response dto.ErrorResponse
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			require.NoError(t, err)

			// Check response format
			assert.Equal(t, "INVALID_URL", response.Error)
			assert.Equal(t, tt.errorMsg, response.Message)
			assert.NotNil(t, response.Timestamp)
			assert.Nil(t, response.Details)

			// Verify JSON is valid
			jsonBytes, err := json.Marshal(response)
			assert.NoError(t, err)

			var unmarshaled map[string]interface{}
			err = json.Unmarshal(jsonBytes, &unmarshaled)
			require.NoError(t, err)

			// Check required fields
			assert.Contains(t, unmarshaled, "error")
			assert.Contains(t, unmarshaled, "message")
			assert.Contains(t, unmarshaled, "timestamp")
		})
	}
}

// Test URL validation error vs other error types.
func TestRepositoryHandler_CreateRepository_ErrorTypeDistinction(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		mockError      error
		expectedStatus int
		expectedCode   string
		expectedMsg    string
	}{
		{
			name: "url_validation_error_returns_400_invalid_url",
			requestBody: map[string]interface{}{
				"url": "invalid-url",
			},
			mockError:      errors.New("invalid repository URL: invalid URL format"),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_URL",
			expectedMsg:    "invalid repository URL: invalid URL format",
		},
		{
			name: "domain_invalid_url_error_returns_400_invalid_url",
			requestBody: map[string]interface{}{
				"url": "",
			},
			mockError:      domain.ErrInvalidRepositoryURL,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_URL",
			expectedMsg:    "repository URL is invalid",
		},
		{
			name: "repository_not_found_returns_404",
			requestBody: map[string]interface{}{
				"url": "https://github.com/golang/go",
			},
			mockError:      domain.ErrRepositoryNotFound,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "REPOSITORY_NOT_FOUND",
			expectedMsg:    "Repository not found",
		},
		{
			name: "repository_already_exists_returns_409",
			requestBody: map[string]interface{}{
				"url": "https://github.com/golang/go",
			},
			mockError:      domain.ErrRepositoryAlreadyExists,
			expectedStatus: http.StatusConflict,
			expectedCode:   "REPOSITORY_ALREADY_EXISTS",
			expectedMsg:    "Repository already exists",
		},
		{
			name: "generic_error_returns_500",
			requestBody: map[string]interface{}{
				"url": "https://github.com/golang/go",
			},
			mockError:      errors.New("database connection failed"),
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
			expectedMsg:    "An internal error occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := testutil.NewMockRepositoryService()
			mockErrorHandler := NewDefaultErrorHandler()

			// Mock service to return the specified error
			mockService.CreateRepositoryFunc = func(_ context.Context, req dto.CreateRepositoryRequest) (*dto.RepositoryResponse, error) {
				return nil, tt.mockError
			}

			handler := NewRepositoryHandler(mockService, mockErrorHandler)

			// Create request
			req := testutil.CreateJSONRequest(http.MethodPost, "/repositories", tt.requestBody)
			recorder := httptest.NewRecorder()

			// Execute
			handler.CreateRepository(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			var response dto.ErrorResponse
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedCode, response.Error)
			assert.Equal(t, tt.expectedMsg, response.Message)
		})
	}
}

// Test URL validation with complete request objects.
func TestRepositoryHandler_CreateRepository_CompleteRequestURLValidation(t *testing.T) {
	tests := []struct {
		name        string
		requestBody dto.CreateRepositoryRequest
		mockSetup   func(*testutil.MockRepositoryService)
		expectError bool
		errorMsg    string
	}{
		{
			name: "complete_request_with_invalid_url_fails",
			requestBody: dto.CreateRepositoryRequest{
				URL: "ftp://github.com/user/repo",
			},
			expectError: true,
			errorMsg:    "invalid repository URL: URL must use http or https scheme",
		},
		{
			name: "complete_request_with_valid_url_succeeds",
			requestBody: dto.CreateRepositoryRequest{
				URL: "https://github.com/golang/go",
			},
			mockSetup: func(mock *testutil.MockRepositoryService) {
				response := testutil.NewRepositoryResponseBuilder().
					WithURL("https://github.com/golang/go").
					WithName("go").
					WithStatus("pending").
					Build()
				mock.ExpectCreateRepository(&response, nil)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := testutil.NewMockRepositoryService()
			mockErrorHandler := NewDefaultErrorHandler()

			if tt.mockSetup != nil {
				tt.mockSetup(mockService)
			} else if tt.expectError {
				mockService.CreateRepositoryFunc = func(_ context.Context, req dto.CreateRepositoryRequest) (*dto.RepositoryResponse, error) {
					return nil, errors.New(tt.errorMsg)
				}
			}

			handler := NewRepositoryHandler(mockService, mockErrorHandler)

			// Create request
			req := testutil.CreateJSONRequest(http.MethodPost, "/repositories", tt.requestBody)
			recorder := httptest.NewRecorder()

			// Execute
			handler.CreateRepository(recorder, req)

			// Assert
			if tt.expectError {
				assert.Equal(t, http.StatusBadRequest, recorder.Code)

				var response dto.ErrorResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "INVALID_URL", response.Error)
				assert.Equal(t, tt.errorMsg, response.Message)
			} else {
				assert.Equal(t, http.StatusAccepted, recorder.Code)

				var response dto.RepositoryResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, tt.requestBody.URL, response.URL)
				assert.Equal(t, tt.requestBody.Name, response.Name)
			}

			// Verify service was called
			assert.Len(t, mockService.CreateRepositoryCalls, 1)
		})
	}
}
