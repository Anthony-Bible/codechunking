package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codechunking/internal/adapter/inbound/api"
	"codechunking/internal/adapter/inbound/api/testutil"
	"codechunking/internal/application/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_GetHealth(t *testing.T) {
	tests := []struct {
		name           string
		mockSetup      func(*testutil.MockHealthService)
		expectedStatus int
		expectedError  string
		validateFunc   func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "healthy_service_returns_200_with_health_data",
			mockSetup: func(mock *testutil.MockHealthService) {
				response := testutil.NewHealthResponseBuilder().
					WithStatus("healthy").
					WithVersion("1.0.0").
					WithDependency("database", "healthy").
					WithDependency("nats", "healthy").
					WithDependency("gemini_api", "healthy").
					Build()
				mock.ExpectGetHealth(&response, nil)
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

				var response dto.HealthResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, "healthy", response.Status)
				assert.Equal(t, "1.0.0", response.Version)
				assert.NotNil(t, response.Dependencies)

				// Check dependencies
				if deps := response.Dependencies; deps != nil {
					assert.Equal(t, "healthy", deps["database"].Status)
					assert.Equal(t, "healthy", deps["nats"].Status)
					assert.Equal(t, "healthy", deps["gemini_api"].Status)
				}

				// Timestamp should be recent
				assert.WithinDuration(t, time.Now(), response.Timestamp, 5*time.Second)
			},
		},
		{
			name: "degraded_service_returns_200_with_degraded_status",
			mockSetup: func(mock *testutil.MockHealthService) {
				response := testutil.NewHealthResponseBuilder().
					WithStatus("degraded").
					WithVersion("1.0.0").
					WithDependency("database", "healthy").
					WithDependency("nats", "unhealthy").
					WithDependency("gemini_api", "healthy").
					Build()
				mock.ExpectGetHealth(&response, nil)
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.HealthResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, "degraded", response.Status)

				// Check that one dependency is unhealthy
				if deps := response.Dependencies; deps != nil {
					assert.Equal(t, "unhealthy", deps["nats"].Status)
				}
			},
		},
		{
			name: "unhealthy_service_returns_503_service_unavailable",
			mockSetup: func(mock *testutil.MockHealthService) {
				response := testutil.NewHealthResponseBuilder().
					WithStatus("unhealthy").
					WithVersion("1.0.0").
					WithDependency("database", "unhealthy").
					WithDependency("nats", "unhealthy").
					WithDependency("gemini_api", "unhealthy").
					Build()
				mock.ExpectGetHealth(&response, nil)
			},
			expectedStatus: http.StatusServiceUnavailable,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

				var response dto.HealthResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, "unhealthy", response.Status)

				// All dependencies should be unhealthy
				if deps := response.Dependencies; deps != nil {
					assert.Equal(t, "unhealthy", deps["database"].Status)
					assert.Equal(t, "unhealthy", deps["nats"].Status)
					assert.Equal(t, "unhealthy", deps["gemini_api"].Status)
				}
			},
		},
		{
			name: "health_service_error_returns_500_internal_server_error",
			mockSetup: func(mock *testutil.MockHealthService) {
				mock.ExpectGetHealth(nil, errors.New("health check failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "service error",
		},
		{
			name: "healthy_service_without_dependencies_returns_200",
			mockSetup: func(mock *testutil.MockHealthService) {
				response := testutil.NewHealthResponseBuilder().
					WithStatus("healthy").
					WithVersion("1.0.0").
					Build()
				mock.ExpectGetHealth(&response, nil)
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.HealthResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, "healthy", response.Status)
				assert.Equal(t, "1.0.0", response.Version)

				// Dependencies can be nil or empty
				if response.Dependencies != nil {
					assert.Empty(t, response.Dependencies)
				}
			},
		},
		{
			name: "health_check_with_custom_version_returns_correct_version",
			mockSetup: func(mock *testutil.MockHealthService) {
				response := testutil.NewHealthResponseBuilder().
					WithStatus("healthy").
					WithVersion("2.1.0-beta").
					Build()
				mock.ExpectGetHealth(&response, nil)
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.HealthResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, "2.1.0-beta", response.Version)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockHealthService := testutil.NewMockHealthService()
			mockErrorHandler := testutil.NewMockErrorHandler()
			tt.mockSetup(mockHealthService)

			handler := api.NewHealthHandler(mockHealthService, mockErrorHandler)

			// Create request
			req := testutil.CreateRequest(http.MethodGet, "/health")
			recorder := httptest.NewRecorder()

			// Execute
			handler.GetHealth(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.expectedError != "" {
				assert.Contains(t, recorder.Body.String(), tt.expectedError)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder)
			}

			// Verify service was called
			assert.Len(t, mockHealthService.GetHealthCalls, 1)
		})
	}
}

func TestHealthHandler_StatusCodeMapping(t *testing.T) {
	tests := []struct {
		name           string
		healthStatus   string
		expectedStatus int
	}{
		{
			name:           "healthy_status_returns_200",
			healthStatus:   "healthy",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "degraded_status_returns_200",
			healthStatus:   "degraded",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unhealthy_status_returns_503",
			healthStatus:   "unhealthy",
			expectedStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockHealthService := testutil.NewMockHealthService()
			mockErrorHandler := testutil.NewMockErrorHandler()

			response := testutil.NewHealthResponseBuilder().
				WithStatus(tt.healthStatus).
				Build()
			mockHealthService.ExpectGetHealth(&response, nil)

			handler := api.NewHealthHandler(mockHealthService, mockErrorHandler)

			// Create request
			req := testutil.CreateRequest(http.MethodGet, "/health")
			recorder := httptest.NewRecorder()

			// Execute
			handler.GetHealth(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)
		})
	}
}

func TestHealthHandler_ResponseHeaders(t *testing.T) {
	t.Run("health_response_should_have_correct_content_type", func(t *testing.T) {
		// Setup
		mockHealthService := testutil.NewMockHealthService()
		mockErrorHandler := testutil.NewMockErrorHandler()

		response := testutil.NewHealthResponseBuilder().
			WithStatus("healthy").
			Build()
		mockHealthService.ExpectGetHealth(&response, nil)

		handler := api.NewHealthHandler(mockHealthService, mockErrorHandler)

		// Create request
		req := testutil.CreateRequest(http.MethodGet, "/health")
		recorder := httptest.NewRecorder()

		// Execute
		handler.GetHealth(recorder, req)

		// Assert headers
		assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
		assert.Equal(t, http.StatusOK, recorder.Code)
	})
}

func TestHealthHandler_ErrorHandling(t *testing.T) {
	t.Run("service_error_should_be_handled_by_error_handler", func(t *testing.T) {
		// Setup
		mockHealthService := testutil.NewMockHealthService()
		mockErrorHandler := testutil.NewMockErrorHandler()

		// Mock health service to return an error
		serviceError := errors.New("database connection failed")
		mockHealthService.ExpectGetHealth(nil, serviceError)

		handler := api.NewHealthHandler(mockHealthService, mockErrorHandler)

		// Create request
		req := testutil.CreateRequest(http.MethodGet, "/health")
		recorder := httptest.NewRecorder()

		// Execute
		handler.GetHealth(recorder, req)

		// Assert error handler was called
		assert.Len(t, mockErrorHandler.HandleServiceErrorCalls, 1)

		// Verify the error passed to error handler
		call := mockErrorHandler.HandleServiceErrorCalls[0]
		assert.Equal(t, serviceError, call.Error)
		assert.Equal(t, req, call.Request)
		assert.Equal(t, recorder, call.ResponseWriter)
	})
}

func TestHealthHandler_Integration(t *testing.T) {
	t.Run("complete_health_check_workflow", func(t *testing.T) {
		// This test simulates a complete health check workflow
		// with realistic dependency statuses

		// Setup
		mockHealthService := testutil.NewMockHealthService()
		mockErrorHandler := testutil.NewMockErrorHandler()

		// Create a realistic health response
		response := dto.HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now(),
			Version:   "1.0.0",
			Dependencies: map[string]dto.DependencyStatus{
				"database": {
					Status:  "healthy",
					Message: "Connected to PostgreSQL",
				},
				"nats": {
					Status:  "healthy",
					Message: "Connected to NATS server",
				},
				"gemini_api": {
					Status:  "healthy",
					Message: "Gemini API responding",
				},
			},
		}

		mockHealthService.ExpectGetHealth(&response, nil)

		handler := api.NewHealthHandler(mockHealthService, mockErrorHandler)

		// Create request
		req := testutil.CreateRequest(http.MethodGet, "/health")
		recorder := httptest.NewRecorder()

		// Execute
		handler.GetHealth(recorder, req)

		// Assert complete response
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

		var actualResponse dto.HealthResponse
		err := testutil.ParseJSONResponse(recorder, &actualResponse)
		require.NoError(t, err)

		// Verify all fields
		assert.Equal(t, "healthy", actualResponse.Status)
		assert.Equal(t, "1.0.0", actualResponse.Version)
		assert.NotZero(t, actualResponse.Timestamp)

		// Verify dependencies
		require.NotNil(t, actualResponse.Dependencies)
		assert.Equal(t, "healthy", actualResponse.Dependencies["database"].Status)
		assert.Equal(t, "Connected to PostgreSQL", actualResponse.Dependencies["database"].Message)
		assert.Equal(t, "healthy", actualResponse.Dependencies["nats"].Status)
		assert.Equal(t, "healthy", actualResponse.Dependencies["gemini_api"].Status)

		// Verify service was called once
		assert.Len(t, mockHealthService.GetHealthCalls, 1)

		// Verify error handler was not called
		assert.Empty(t, mockErrorHandler.HandleServiceErrorCalls)
		assert.Empty(t, mockErrorHandler.HandleValidationErrorCalls)
	})
}
