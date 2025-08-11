package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codechunking/internal/adapter/inbound/api/testutil"

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
					assert.False(t, strings.Contains(pattern, "{id:[0-9]+}"),
						"Should not use Gorilla mux regex patterns: %s", pattern)
					assert.False(t, strings.Contains(pattern, "{id:.*}"),
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
			// Setup
			mockHealthService := testutil.NewMockHealthService()
			mockRepositoryService := testutil.NewMockRepositoryService()
			mockErrorHandler := testutil.NewMockErrorHandler()

			if tt.setupMocks != nil {
				tt.setupMocks(mockRepositoryService)
			}

			// Create a custom handler that captures path parameters
			var capturedParams map[string]string
			paramCapture := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedParams = ExtractPathParams(r)
				w.WriteHeader(http.StatusOK)
			})

			_ = NewHealthHandler(mockHealthService, mockErrorHandler)
			_ = NewRepositoryHandler(mockRepositoryService, mockErrorHandler)

			registry := NewRouteRegistry()

			// Override handlers with parameter capture for testing
			if strings.Contains(tt.path, "/jobs/") {
				if err := registry.RegisterRoute("GET /repositories/{id}/jobs/{job_id}", paramCapture); err != nil {
					t.Errorf("Failed to register route: %v", err)
				}
			} else if strings.Contains(tt.path, "/jobs") {
				if err := registry.RegisterRoute("GET /repositories/{id}/jobs", paramCapture); err != nil {
					t.Errorf("Failed to register route: %v", err)
				}
			} else if strings.Contains(tt.path, "/repositories/") {
				if err := registry.RegisterRoute("GET /repositories/{id}", paramCapture); err != nil {
					t.Errorf("Failed to register route: %v", err)
				}
			}

			mux := registry.BuildServeMux()
			require.NotNil(t, mux)

			// Create request
			req := testutil.CreateRequest(tt.method, tt.path)
			recorder := httptest.NewRecorder()

			// Execute
			mux.ServeHTTP(recorder, req)

			// Assert
			assert.Equal(t, http.StatusOK, recorder.Code)

			// Verify extracted parameters
			for key, expectedValue := range tt.expectedParams {
				actualValue, exists := capturedParams[key]
				assert.True(t, exists, "Parameter %s should be extracted", key)
				assert.Equal(t, expectedValue, actualValue, "Parameter %s should have correct value", key)
			}
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
		assert.Error(t, err)
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
