package api

import (
	"codechunking/internal/adapter/inbound/api/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouteRegistry_RegisterRoutes(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func() *RouteRegistry
		validateFunc func(t *testing.T, registry *RouteRegistry)
	}{
		{
			name: "registers_all_api_routes_with_go_122_patterns",
			setupFunc: func() *RouteRegistry {
				mockHealthService := testutil.NewMockHealthService()
				mockRepositoryService := testutil.NewMockRepositoryService()
				mockErrorHandler := testutil.NewMockErrorHandler()

				healthHandler := NewHealthHandler(mockHealthService, mockErrorHandler)
				repositoryHandler := NewRepositoryHandler(mockRepositoryService, mockErrorHandler)

				registry := NewRouteRegistry()
				registry.RegisterAPIRoutes(healthHandler, repositoryHandler)
				return registry
			},
			validateFunc: func(t *testing.T, registry *RouteRegistry) {
				// Test Go 1.22+ pattern syntax is used correctly
				expectedRoutes := map[string]string{
					"GET /health":                          "health endpoint",
					"POST /repositories":                   "create repository",
					"GET /repositories":                    "list repositories",
					"GET /repositories/{id}":               "get repository",
					"DELETE /repositories/{id}":            "delete repository",
					"GET /repositories/{id}/jobs":          "list repository jobs",
					"GET /repositories/{id}/jobs/{job_id}": "get specific job",
				}

				for pattern, description := range expectedRoutes {
					assert.True(t, registry.HasRoute(pattern),
						"Should register route %s (%s)", pattern, description)
				}

				// Verify total route count
				assert.Equal(t, len(expectedRoutes), registry.RouteCount())
			},
		},
		{
			name: "route_patterns_use_correct_go_122_syntax",
			setupFunc: func() *RouteRegistry {
				mockHealthService := testutil.NewMockHealthService()
				mockRepositoryService := testutil.NewMockRepositoryService()
				mockErrorHandler := testutil.NewMockErrorHandler()

				healthHandler := NewHealthHandler(mockHealthService, mockErrorHandler)
				repositoryHandler := NewRepositoryHandler(mockRepositoryService, mockErrorHandler)

				registry := NewRouteRegistry()
				registry.RegisterAPIRoutes(healthHandler, repositoryHandler)
				return registry
			},
			validateFunc: func(t *testing.T, registry *RouteRegistry) {
				// Test that route patterns follow Go 1.22+ ServeMux syntax
				patterns := registry.GetPatterns()

				// Should use method prefixes
				hasMethodPrefixes := false
				for _, pattern := range patterns {
					if strings.HasPrefix(pattern, "GET ") ||
						strings.HasPrefix(pattern, "POST ") ||
						strings.HasPrefix(pattern, "DELETE ") {
						hasMethodPrefixes = true
						break
					}
				}
				assert.True(t, hasMethodPrefixes, "Routes should use Go 1.22+ method prefixes")

				// Should use {id} syntax for path parameters
				hasPathParams := false
				for _, pattern := range patterns {
					if strings.Contains(pattern, "{id}") || strings.Contains(pattern, "{job_id}") {
						hasPathParams = true
						break
					}
				}
				assert.True(t, hasPathParams, "Routes should use Go 1.22+ {param} syntax")

				// Should not contain Gorilla mux specific patterns
				for _, pattern := range patterns {
					assert.NotContains(t, pattern, "{id:[0-9]+}",
						"Should not use Gorilla mux regex patterns: %s", pattern)
					assert.NotContains(t, pattern, "{id:.*}",
						"Should not use Gorilla mux wildcard patterns: %s", pattern)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute
			registry := tt.setupFunc()

			// Validate
			if tt.validateFunc != nil {
				tt.validateFunc(t, registry)
			}
		})
	}
}

func TestRouteRegistry_ServeMuxIntegration(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		expectedCode int
		setupMocks   func(*testutil.MockHealthService, *testutil.MockRepositoryService)
	}{
		{
			name:         "health_route_matches_exact_path",
			method:       http.MethodGet,
			path:         "/health",
			expectedCode: http.StatusOK,
			setupMocks: func(health *testutil.MockHealthService, repo *testutil.MockRepositoryService) {
				response := testutil.NewHealthResponseBuilder().
					WithStatus("healthy").
					WithVersion("1.0.0").
					Build()
				health.ExpectGetHealth(&response, nil)
			},
		},
		{
			name:         "repository_list_route_matches_exact_path",
			method:       http.MethodGet,
			path:         "/repositories",
			expectedCode: http.StatusOK,
			setupMocks: func(health *testutil.MockHealthService, repo *testutil.MockRepositoryService) {
				response := testutil.NewRepositoryListResponseBuilder().Build()
				repo.ExpectListRepositories(&response, nil)
			},
		},
		{
			name:         "repository_create_route_matches_post_method",
			method:       http.MethodPost,
			path:         "/repositories",
			expectedCode: http.StatusAccepted,
			setupMocks: func(health *testutil.MockHealthService, repo *testutil.MockRepositoryService) {
				response := testutil.NewRepositoryResponseBuilder().Build()
				repo.ExpectCreateRepository(&response, nil)
			},
		},
		{
			name:         "repository_get_route_matches_with_uuid_param",
			method:       http.MethodGet,
			path:         "/repositories/123e4567-e89b-12d3-a456-426614174000",
			expectedCode: http.StatusOK,
			setupMocks: func(health *testutil.MockHealthService, repo *testutil.MockRepositoryService) {
				response := testutil.NewRepositoryResponseBuilder().Build()
				repo.ExpectGetRepository(&response, nil)
			},
		},
		{
			name:         "repository_delete_route_matches_with_uuid_param",
			method:       http.MethodDelete,
			path:         "/repositories/123e4567-e89b-12d3-a456-426614174000",
			expectedCode: http.StatusNoContent,
			setupMocks: func(health *testutil.MockHealthService, repo *testutil.MockRepositoryService) {
				repo.ExpectDeleteRepository(nil)
			},
		},
		{
			name:         "repository_jobs_route_matches_with_nested_params",
			method:       http.MethodGet,
			path:         "/repositories/123e4567-e89b-12d3-a456-426614174000/jobs",
			expectedCode: http.StatusOK,
			setupMocks: func(health *testutil.MockHealthService, repo *testutil.MockRepositoryService) {
				response := testutil.NewIndexingJobListResponseBuilder().Build()
				repo.ExpectGetRepositoryJobs(&response, nil)
			},
		},
		{
			name:         "specific_job_route_matches_with_double_nested_params",
			method:       http.MethodGet,
			path:         "/repositories/123e4567-e89b-12d3-a456-426614174000/jobs/456e7890-e89b-12d3-a456-426614174001",
			expectedCode: http.StatusOK,
			setupMocks: func(health *testutil.MockHealthService, repo *testutil.MockRepositoryService) {
				response := testutil.NewIndexingJobResponseBuilder().Build()
				repo.ExpectGetIndexingJob(&response, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockHealthService := testutil.NewMockHealthService()
			mockRepositoryService := testutil.NewMockRepositoryService()
			mockErrorHandler := testutil.NewMockErrorHandler()

			if tt.setupMocks != nil {
				tt.setupMocks(mockHealthService, mockRepositoryService)
			}

			healthHandler := NewHealthHandler(mockHealthService, mockErrorHandler)
			repositoryHandler := NewRepositoryHandler(mockRepositoryService, mockErrorHandler)

			registry := NewRouteRegistry()
			registry.RegisterAPIRoutes(healthHandler, repositoryHandler)

			mux := registry.BuildServeMux()
			require.NotNil(t, mux)

			// Create request
			var req *http.Request
			if tt.method == http.MethodPost {
				body := strings.NewReader(`{"url": "https://github.com/test/repo"}`)
				req = testutil.CreateRequestWithBody(tt.method, tt.path, body)
			} else {
				req = testutil.CreateRequest(tt.method, tt.path)
			}

			recorder := httptest.NewRecorder()

			// Execute
			mux.ServeHTTP(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedCode, recorder.Code,
				"Route %s %s should return status %d", tt.method, tt.path, tt.expectedCode)
		})
	}
}

// Helper function to create parameter capture handler.
func createParamCaptureHandler() (http.HandlerFunc, *map[string]string) {
	var capturedParams map[string]string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedParams = ExtractPathParams(r)
		w.WriteHeader(http.StatusOK)
	})
	return handler, &capturedParams
}

// Helper function to setup test registry with mocks.
func setupTestRegistryWithMocks(
	setupMocks func(*testutil.MockRepositoryService),
) (*testutil.MockRepositoryService, *testutil.MockHealthService, *testutil.MockErrorHandler) {
	mockHealthService := testutil.NewMockHealthService()
	mockRepositoryService := testutil.NewMockRepositoryService()
	mockErrorHandler := testutil.NewMockErrorHandler()

	if setupMocks != nil {
		setupMocks(mockRepositoryService)
	}

	return mockRepositoryService, mockHealthService, mockErrorHandler
}

// Helper function to register route based on path pattern.
func registerRouteForPath(registry *RouteRegistry, path string, handler http.HandlerFunc) error {
	if strings.Contains(path, "/jobs/") {
		return registry.RegisterRoute("GET /repositories/{id}/jobs/{job_id}", handler)
	}
	if strings.Contains(path, "/jobs") {
		return registry.RegisterRoute("GET /repositories/{id}/jobs", handler)
	}
	if strings.Contains(path, "/repositories/") {
		return registry.RegisterRoute("GET /repositories/{id}", handler)
	}
	return nil
}

// Helper function to assert extracted parameters.
func assertExtractedParams(t *testing.T, expectedParams map[string]string, capturedParams map[string]string) {
	for key, expectedValue := range expectedParams {
		actualValue, exists := capturedParams[key]
		assert.True(t, exists, "Parameter %s should be extracted", key)
		assert.Equal(t, expectedValue, actualValue, "Parameter %s should have correct value", key)
	}
}

func TestRouteRegistry_PathParameterExtraction(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		expectedParams map[string]string
		setupMocks     func(*testutil.MockRepositoryService)
	}{
		{
			name:   "extracts_repository_id_parameter",
			method: http.MethodGet,
			path:   "/repositories/123e4567-e89b-12d3-a456-426614174000",
			expectedParams: map[string]string{
				"id": "123e4567-e89b-12d3-a456-426614174000",
			},
			setupMocks: func(repo *testutil.MockRepositoryService) {
				response := testutil.NewRepositoryResponseBuilder().Build()
				repo.ExpectGetRepository(&response, nil)
			},
		},
		{
			name:   "extracts_repository_id_from_jobs_route",
			method: http.MethodGet,
			path:   "/repositories/456e7890-e89b-12d3-a456-426614174001/jobs",
			expectedParams: map[string]string{
				"id": "456e7890-e89b-12d3-a456-426614174001",
			},
			setupMocks: func(repo *testutil.MockRepositoryService) {
				response := testutil.NewIndexingJobListResponseBuilder().Build()
				repo.ExpectGetRepositoryJobs(&response, nil)
			},
		},
		{
			name:   "extracts_both_repository_id_and_job_id_parameters",
			method: http.MethodGet,
			path:   "/repositories/789e0123-e89b-12d3-a456-426614174002/jobs/abc123de-e89b-12d3-a456-426614174003",
			expectedParams: map[string]string{
				"id":     "789e0123-e89b-12d3-a456-426614174002",
				"job_id": "abc123de-e89b-12d3-a456-426614174003",
			},
			setupMocks: func(repo *testutil.MockRepositoryService) {
				response := testutil.NewIndexingJobResponseBuilder().Build()
				repo.ExpectGetIndexingJob(&response, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test dependencies
			_, _, _ = setupTestRegistryWithMocks(tt.setupMocks)

			// Create parameter capture handler
			paramCapture, capturedParams := createParamCaptureHandler()

			// Setup registry and register route
			registry := NewRouteRegistry()
			if err := registerRouteForPath(registry, tt.path, paramCapture); err != nil {
				t.Errorf("Failed to register route: %v", err)
				return
			}

			// Build ServeMux and create request
			mux := registry.BuildServeMux()
			require.NotNil(t, mux)
			req := testutil.CreateRequest(tt.method, tt.path)
			recorder := httptest.NewRecorder()

			// Execute request
			mux.ServeHTTP(recorder, req)

			// Assert response and parameters
			assert.Equal(t, http.StatusOK, recorder.Code)
			assertExtractedParams(t, tt.expectedParams, *capturedParams)
		})
	}
}

func TestRouteRegistry_MethodNotAllowed(t *testing.T) {
	t.Run("returns_405_for_unsupported_methods", func(t *testing.T) {
		// Setup
		mockHealthService := testutil.NewMockHealthService()
		mockRepositoryService := testutil.NewMockRepositoryService()
		mockErrorHandler := testutil.NewMockErrorHandler()

		healthHandler := NewHealthHandler(mockHealthService, mockErrorHandler)
		repositoryHandler := NewRepositoryHandler(mockRepositoryService, mockErrorHandler)

		registry := NewRouteRegistry()
		registry.RegisterAPIRoutes(healthHandler, repositoryHandler)
		mux := registry.BuildServeMux()

		tests := []struct {
			method string
			path   string
		}{
			{http.MethodPost, "/health"},                                            // Only GET allowed
			{http.MethodDelete, "/health"},                                          // Only GET allowed
			{http.MethodPut, "/repositories"},                                       // Only GET and POST allowed
			{http.MethodPatch, "/repositories"},                                     // Only GET and POST allowed
			{http.MethodPost, "/repositories/123e4567-e89b-12d3-a456-426614174000"}, // Only GET and DELETE allowed
		}

		for _, test := range tests {
			req := testutil.CreateRequest(test.method, test.path)
			recorder := httptest.NewRecorder()

			mux.ServeHTTP(recorder, req)

			assert.Equal(t, http.StatusMethodNotAllowed, recorder.Code,
				"Method %s on path %s should return 405", test.method, test.path)
		}
	})
}

func TestRouteRegistry_NotFound(t *testing.T) {
	t.Run("returns_404_for_unregistered_routes", func(t *testing.T) {
		// Setup
		mockHealthService := testutil.NewMockHealthService()
		mockRepositoryService := testutil.NewMockRepositoryService()
		mockErrorHandler := testutil.NewMockErrorHandler()

		healthHandler := NewHealthHandler(mockHealthService, mockErrorHandler)
		repositoryHandler := NewRepositoryHandler(mockRepositoryService, mockErrorHandler)

		registry := NewRouteRegistry()
		registry.RegisterAPIRoutes(healthHandler, repositoryHandler)
		mux := registry.BuildServeMux()

		tests := []string{
			"/unknown",
			"/health/status",
			"/repositories/123/invalid",
			"/api/v1/health", // Wrong prefix
		}

		for _, path := range tests {
			req := testutil.CreateRequest(http.MethodGet, path)
			recorder := httptest.NewRecorder()

			mux.ServeHTTP(recorder, req)

			assert.Equal(t, http.StatusNotFound, recorder.Code,
				"Path %s should return 404", path)
		}
	})
}

func TestRouteRegistry_RouteConflicts(t *testing.T) {
	t.Run("detects_conflicting_route_patterns", func(t *testing.T) {
		registry := NewRouteRegistry()

		// Register initial route
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		err := registry.RegisterRoute("GET /repositories/{id}", handler)
		require.NoError(t, err)

		// Try to register conflicting route
		conflictingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusConflict)
		})

		err = registry.RegisterRoute("GET /repositories/{repo_id}", conflictingHandler)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "route conflict")
	})
}

func TestRouteRegistry_InvalidPatterns(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		expectedError string
	}{
		{
			name:          "rejects_invalid_method",
			pattern:       "INVALID /health",
			expectedError: "invalid HTTP method",
		},
		{
			name:          "rejects_empty_path",
			pattern:       "GET ",
			expectedError: "empty path",
		},
		{
			name:          "rejects_invalid_parameter_syntax",
			pattern:       "GET /repositories/{id",
			expectedError: "invalid parameter syntax",
		},
		{
			name:          "rejects_duplicate_parameters",
			pattern:       "GET /repositories/{id}/users/{id}",
			expectedError: "duplicate parameter name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRouteRegistry()
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

			err := registry.RegisterRoute(tt.pattern, handler)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

// === FAILING TESTS FOR ROUTE VALIDATION ERROR NORMALIZATION (RED PHASE) ===
// These tests define expected behavior after refactoring and will initially fail

func TestRouteValidation_NormalizedErrorMessages(t *testing.T) {
	t.Run("validation_errors_use_normalized_message_constants", func(t *testing.T) {
		// This test will fail until error message constants are defined and used
		registry := NewRouteRegistry()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

		tests := []struct {
			name                string
			pattern             string
			expectedErrorType   string
			expectedConstantRef string
		}{
			{
				name:                "empty_pattern_uses_error_constant",
				pattern:             "",
				expectedErrorType:   "EmptyPattern",
				expectedConstantRef: "ErrEmptyPattern",
			},
			{
				name:                "whitespace_pattern_uses_error_constant",
				pattern:             "   ",
				expectedErrorType:   "WhitespacePattern",
				expectedConstantRef: "ErrWhitespacePattern",
			},
			{
				name:                "invalid_format_uses_error_constant",
				pattern:             "GET/health", // Missing space
				expectedErrorType:   "InvalidFormat",
				expectedConstantRef: "ErrInvalidPatternFormat",
			},
			{
				name:                "empty_method_uses_error_constant",
				pattern:             " /health", // Empty method
				expectedErrorType:   "EmptyMethod",
				expectedConstantRef: "ErrEmptyMethod",
			},
			{
				name:                "invalid_method_uses_error_constant",
				pattern:             "INVALID /health",
				expectedErrorType:   "InvalidMethod",
				expectedConstantRef: "ErrInvalidMethod",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := registry.RegisterRoute(tt.pattern, handler)
				require.Error(t, err, "Should return error for case: %s", tt.name)

				// After refactoring, errors should reference constants and have consistent structure
				errMsg := err.Error()

				// Should contain normalized error structure (will fail until implemented)
				assert.Contains(t, errMsg, tt.expectedConstantRef,
					"Error should reference normalized constant: %s", tt.expectedConstantRef)

				// Should have consistent error message format
				assert.NotContains(t, errMsg, "fmt.Errorf",
					"Error should not contain inline formatting")
			})
		}
	})

	t.Run("parameter_validation_errors_use_helper_functions", func(t *testing.T) {
		// This test will fail until parameter validation error helpers are implemented
		registry := NewRouteRegistry()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

		tests := []struct {
			name               string
			pattern            string
			expectedHelperName string
			expectedErrorCode  string
		}{
			{
				name:               "missing_closing_brace_uses_helper",
				pattern:            "GET /repos/{id",
				expectedHelperName: "newParameterSyntaxError",
				expectedErrorCode:  "PARAM_MISSING_CLOSE_BRACE",
			},
			{
				name:               "empty_parameter_name_uses_helper",
				pattern:            "GET /repos/{}",
				expectedHelperName: "newParameterSyntaxError",
				expectedErrorCode:  "PARAM_EMPTY_NAME",
			},
			{
				name:               "invalid_parameter_name_uses_helper",
				pattern:            "GET /repos/{id-invalid}",
				expectedHelperName: "newParameterSyntaxError",
				expectedErrorCode:  "PARAM_INVALID_NAME",
			},
			{
				name:               "duplicate_parameter_uses_helper",
				pattern:            "GET /repos/{id}/users/{id}",
				expectedHelperName: "newParameterSyntaxError",
				expectedErrorCode:  "PARAM_DUPLICATE_NAME",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := registry.RegisterRoute(tt.pattern, handler)
				require.Error(t, err, "Should return error for case: %s", tt.name)

				errMsg := err.Error()

				// After refactoring, should use centralized error helper functions
				assert.Contains(t, errMsg, tt.expectedHelperName,
					"Error should be created using helper function: %s", tt.expectedHelperName)

				// Should include structured error codes for easier testing and i18n
				assert.Contains(t, errMsg, tt.expectedErrorCode,
					"Error should include structured error code: %s", tt.expectedErrorCode)
			})
		}
	})

	t.Run("route_conflict_errors_use_normalized_messages", func(t *testing.T) {
		// This test will fail until route conflict error messages are normalized
		registry := NewRouteRegistry()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

		// Register first route
		err := registry.RegisterRoute("GET /repos/{id}", handler)
		require.NoError(t, err)

		tests := []struct {
			name               string
			conflictingPattern string
			expectedErrorType  string
			expectedHelper     string
		}{
			{
				name:               "exact_duplicate_uses_helper",
				conflictingPattern: "GET /repos/{id}",
				expectedErrorType:  "ExactDuplicate",
				expectedHelper:     "newRouteConflictError",
			},
			{
				name:               "same_structure_different_param_names_uses_helper",
				conflictingPattern: "GET /repos/{repo_id}",
				expectedErrorType:  "SameStructure",
				expectedHelper:     "newRouteConflictError",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := registry.RegisterRoute(tt.conflictingPattern, handler)
				require.Error(t, err, "Should detect conflict for case: %s", tt.name)

				errMsg := err.Error()

				// Should use centralized conflict error helper
				assert.Contains(t, errMsg, tt.expectedHelper,
					"Should use normalized conflict error helper: %s", tt.expectedHelper)

				// Should have consistent conflict type messaging
				assert.Contains(t, errMsg, tt.expectedErrorType,
					"Should include conflict type: %s", tt.expectedErrorType)
			})
		}
	})
}

func TestRouteValidation_ErrorMessageConstants(t *testing.T) {
	t.Run("error_constants_are_defined_and_reusable", func(t *testing.T) {
		// This test will fail until error message constants are properly defined

		// After refactoring, there should be a constants section with reusable error messages
		// These constants should be used consistently across all validation functions

		tests := []struct {
			constantName   string
			expectedValue  string
			usageLocations []string
		}{
			{
				constantName:   "ErrEmptyPattern",
				expectedValue:  "ErrEmptyPattern: route pattern cannot be empty",
				usageLocations: []string{"validatePattern"},
			},
			{
				constantName:   "ErrWhitespacePattern",
				expectedValue:  "ErrWhitespacePattern: route pattern cannot be only whitespace",
				usageLocations: []string{"validatePattern"},
			},
			{
				constantName:   "ErrInvalidPatternFormat",
				expectedValue:  "ErrInvalidPatternFormat: invalid route pattern format: must have format 'METHOD /path'",
				usageLocations: []string{"validatePattern"},
			},
			{
				constantName:   "ErrEmptyMethod",
				expectedValue:  "ErrEmptyMethod: HTTP method cannot be empty",
				usageLocations: []string{"validatePattern"},
			},
			{
				constantName:   "ErrInvalidMethod",
				expectedValue:  "ErrInvalidMethod: invalid HTTP method 'INVALID' in pattern 'INVALID /health': must be one of GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS",
				usageLocations: []string{"validatePattern", "isValidHTTPMethod"},
			},
		}

		for _, tt := range tests {
			t.Run("constant_"+tt.constantName, func(t *testing.T) {
				// This assertion will fail until constants are defined
				// We're testing that the constants exist and have expected values

				// For now, we'll test that the error messages are consistent across usage
				registry := NewRouteRegistry()
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

				var err error
				switch tt.constantName {
				case "ErrEmptyPattern":
					err = registry.RegisterRoute("", handler)
				case "ErrWhitespacePattern":
					err = registry.RegisterRoute("   ", handler)
				case "ErrInvalidPatternFormat":
					err = registry.RegisterRoute("GET/health", handler) // No space
				case "ErrEmptyMethod":
					err = registry.RegisterRoute(" /health", handler) // Empty method
				case "ErrInvalidMethod":
					err = registry.RegisterRoute("INVALID /health", handler)
				}

				require.Error(t, err, "Should return error to test constant usage")

				// After refactoring, error messages should exactly match constants
				assert.Equal(t, tt.expectedValue, err.Error(),
					"Error message should match defined constant: %s", tt.constantName)
			})
		}
	})

	t.Run("error_message_templates_are_consistent", func(t *testing.T) {
		// This test will fail until error message templates are implemented
		registry := NewRouteRegistry()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

		// Test that error messages with parameters use consistent templates
		templateTests := []struct {
			name             string
			pattern          string
			expectedTemplate string
			expectedParams   []string
		}{
			{
				name:             "invalid_method_with_pattern",
				pattern:          "INVALID /health",
				expectedTemplate: "invalid HTTP method '%s' in pattern '%s': %s",
				expectedParams: []string{
					"INVALID",
					"INVALID /health",
					"must be one of GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS",
				},
			},
			{
				name:             "path_without_leading_slash",
				pattern:          "GET health",
				expectedTemplate: "path '%s' in pattern '%s' must start with '/'",
				expectedParams:   []string{"health", "GET health"},
			},
			{
				name:             "double_slashes_in_path",
				pattern:          "GET /repos//users",
				expectedTemplate: "path '%s' in pattern '%s' contains double slashes",
				expectedParams:   []string{"/repos//users", "GET /repos//users"},
			},
		}

		for _, tt := range templateTests {
			t.Run(tt.name, func(t *testing.T) {
				err := registry.RegisterRoute(tt.pattern, handler)
				require.Error(t, err, "Should return error for case: %s", tt.name)

				errMsg := err.Error()

				// After refactoring, should use consistent templates
				// This will fail until template-based error construction is implemented
				for _, param := range tt.expectedParams {
					assert.Contains(t, errMsg, param,
						"Error should contain parameter: %s", param)
				}

				// Template structure should be consistent (no inline fmt.Sprintf calls)
				assert.NotContains(t, errMsg, "fmt.Sprintf",
					"Should use pre-built templates, not inline formatting")
			})
		}
	})
}
