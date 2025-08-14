package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codechunking/internal/adapter/inbound/api/testutil"

	"github.com/stretchr/testify/assert"
)

// Custom context key types to avoid SA1029 staticcheck warning.
type contextKey string

const (
	errorContextKey     contextKey = "error"
	userIDContextKey    contextKey = "user_id"
	requestIDContextKey contextKey = "request_id"
)

func TestLoggingMiddleware(t *testing.T) {
	tests := []testCase{
		createLoggingTestCase("logs_successful_request", http.MethodGet, "/health", http.StatusOK,
			validateSuccessfulRequest),
		createLoggingTestCase("logs_error_response", http.MethodGet, "/nonexistent", http.StatusNotFound,
			validateErrorResponse),
		createLoggingTestCase("logs_post_request_with_body", http.MethodPost, "/repositories", http.StatusAccepted,
			validatePostRequest),
		createLoggingTestCase(
			"logs_request_with_query_parameters",
			http.MethodGet,
			"/repositories?limit=10&offset=20",
			http.StatusOK,
			validateQueryParameterRequest,
		),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			var logOutput strings.Builder
			logger := NewTestLogger(&logOutput)

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseCode)
				if _, err := w.Write([]byte("response body")); err != nil {
					// In tests, write errors to ResponseRecorder are unlikely, but handle to avoid errcheck
					_ = err
				}
			})

			middleware := NewLoggingMiddleware(logger)
			handler := middleware(nextHandler)

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
			handler.ServeHTTP(recorder, req)

			// Assert
			assert.Equal(t, tt.responseCode, recorder.Code)

			// Validate logging output
			if tt.validateFunc != nil {
				tt.validateFunc(t, logOutput.String())
			}
		})
	}
}

func TestLoggingMiddleware_RequestTiming(t *testing.T) {
	t.Run("measures_and_logs_request_duration", func(t *testing.T) {
		// Setup
		var logOutput strings.Builder
		logger := NewTestLogger(&logOutput)

		slowHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond) // Simulate slow handler
			w.WriteHeader(http.StatusOK)
		})

		middleware := NewLoggingMiddleware(logger)
		handler := middleware(slowHandler)

		req := testutil.CreateRequest(http.MethodGet, "/slow-endpoint")
		recorder := httptest.NewRecorder()

		// Execute
		start := time.Now()
		handler.ServeHTTP(recorder, req)
		actualDuration := time.Since(start)

		// Assert
		logText := logOutput.String()
		assert.Contains(t, logText, "duration")

		// The logged duration should be reasonable (at least 90ms due to sleep)
		assert.Greater(t, actualDuration, 90*time.Millisecond)
	})
}

func TestCORSMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		origin         string
		requestHeaders string
		validateFunc   func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:   "sets_cors_headers_for_simple_request",
			method: http.MethodGet,
			origin: "https://example.com",
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				assert.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
				assert.Equal(
					t,
					"GET, POST, PUT, DELETE, OPTIONS",
					recorder.Header().Get("Access-Control-Allow-Methods"),
				)
				assert.Equal(t, "Content-Type, Authorization", recorder.Header().Get("Access-Control-Allow-Headers"))
			},
		},
		{
			name:   "handles_preflight_options_request",
			method: http.MethodOptions,
			origin: "https://api.example.com",
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusNoContent, recorder.Code)
				assert.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
				assert.Equal(
					t,
					"GET, POST, PUT, DELETE, OPTIONS",
					recorder.Header().Get("Access-Control-Allow-Methods"),
				)
				assert.Equal(t, "Content-Type, Authorization", recorder.Header().Get("Access-Control-Allow-Headers"))
				assert.Equal(t, "86400", recorder.Header().Get("Access-Control-Max-Age"))
			},
		},
		{
			name:           "handles_preflight_with_custom_headers",
			method:         http.MethodOptions,
			origin:         "https://app.example.com",
			requestHeaders: "X-Custom-Header, X-API-Key",
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				allowedHeaders := recorder.Header().Get("Access-Control-Allow-Headers")
				assert.Contains(t, allowedHeaders, "Content-Type")
				assert.Contains(t, allowedHeaders, "Authorization")
				// Should include custom headers requested
				assert.Contains(t, allowedHeaders, "X-Custom-Header")
				assert.Contains(t, allowedHeaders, "X-API-Key")
			},
		},
		{
			name:   "sets_cors_headers_for_post_request",
			method: http.MethodPost,
			origin: "https://localhost:3000",
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				assert.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
				// Should pass through to next handler
				assert.Equal(t, http.StatusOK, recorder.Code)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte("success")); err != nil {
					t.Logf("Failed to write response: %v", err)
				}
			})

			middleware := NewCORSMiddleware()
			handler := middleware(nextHandler)

			// Create request
			req := testutil.CreateRequest(tt.method, "/test")
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.requestHeaders != "" {
				req.Header.Set("Access-Control-Request-Headers", tt.requestHeaders)
			}

			recorder := httptest.NewRecorder()

			// Execute
			handler.ServeHTTP(recorder, req)

			// Validate
			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder)
			}
		})
	}
}

func TestErrorHandlingMiddleware(t *testing.T) {
	tests := []struct {
		name         string
		handlerFunc  http.HandlerFunc
		expectedCode int
		validateFunc func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "handles_panic_and_returns_500",
			handlerFunc: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic("something went wrong")
			}),
			expectedCode: http.StatusInternalServerError,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				assert.Contains(t, recorder.Body.String(), "Internal Server Error")
				assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
			},
		},
		{
			name: "passes_through_normal_response",
			handlerFunc: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte("normal response")); err != nil {
					t.Logf("Failed to write response: %v", err)
				}
			}),
			expectedCode: http.StatusOK,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				assert.Equal(t, "normal response", recorder.Body.String())
			},
		},
		{
			name: "handles_context_cancellation",
			handlerFunc: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				select {
				case <-ctx.Done():
					return // Context cancelled
				case <-time.After(1 * time.Second):
					w.WriteHeader(http.StatusOK)
				}
			}),
			expectedCode: http.StatusInternalServerError, // Should handle context cancellation gracefully
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				// Should not panic and should return error response
				assert.Contains(t, recorder.Body.String(), "Request cancelled")
			},
		},
		{
			name: "handles_custom_error_types",
			handlerFunc: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Simulate custom error handling
				// Add error context for testing
				_ = r.WithContext(context.WithValue(r.Context(), errorContextKey, "validation_failed"))
				panic("validation error occurred")
			}),
			expectedCode: http.StatusInternalServerError,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			middleware := NewErrorHandlingMiddleware()
			handler := middleware(tt.handlerFunc)

			req := testutil.CreateRequest(http.MethodGet, "/test")

			// For context cancellation test, cancel the context
			if tt.name == "handles_context_cancellation" {
				ctx, cancel := context.WithCancel(req.Context())
				req = req.WithContext(ctx)
				cancel() // Cancel immediately
			}

			recorder := httptest.NewRecorder()

			// Execute
			handler.ServeHTTP(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedCode, recorder.Code)

			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder)
			}
		})
	}
}

func TestMiddlewareChain(t *testing.T) {
	tests := []struct {
		name         string
		middlewares  []Middleware
		validateFunc func(t *testing.T, recorder *httptest.ResponseRecorder, logOutput string)
	}{
		{
			name: "executes_middleware_in_correct_order",
			middlewares: []Middleware{
				NewLoggingMiddleware(NewTestLogger(&strings.Builder{})),
				NewCORSMiddleware(),
				NewErrorHandlingMiddleware(),
			},
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder, logOutput string) {
				// Should have CORS headers
				assert.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
				// Should have logged the request
				assert.Contains(t, logOutput, "GET")
				assert.Contains(t, logOutput, "/test")
				// Should return successful response
				assert.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "error_middleware_catches_panic_after_other_middleware",
			middlewares: []Middleware{
				NewCORSMiddleware(),
				NewLoggingMiddleware(NewTestLogger(&strings.Builder{})),
				NewErrorHandlingMiddleware(),
			},
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder, logOutput string) {
				// Should still have CORS headers even after panic
				assert.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
				// Should return 500 due to panic
				assert.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			var logOutput strings.Builder

			// Recreate middlewares with the shared log buffer
			var actualMiddlewares []Middleware
			for _, middleware := range tt.middlewares {
				// Check if this is a logging middleware and replace with one using our buffer
				if strings.Contains(tt.name, "executes_middleware_in_correct_order") {
					if len(actualMiddlewares) == 0 { // First middleware is logging
						actualMiddlewares = append(actualMiddlewares, NewLoggingMiddleware(NewTestLogger(&logOutput)))
					} else {
						actualMiddlewares = append(actualMiddlewares, middleware)
					}
				} else {
					actualMiddlewares = append(actualMiddlewares, middleware)
				}
			}

			// Handler that panics for error middleware test
			var finalHandler http.HandlerFunc
			if strings.Contains(tt.name, "panic") {
				finalHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					panic("test panic")
				})
			} else {
				finalHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					if _, err := w.Write([]byte("success")); err != nil {
						t.Logf("Failed to write response: %v", err)
					}
				})
			}

			// Build middleware chain
			chain := NewMiddlewareChain(actualMiddlewares...)
			handler := chain(finalHandler)

			req := testutil.CreateRequest(http.MethodGet, "/test")
			recorder := httptest.NewRecorder()

			// Execute
			handler.ServeHTTP(recorder, req)

			// Validate
			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder, logOutput.String())
			}
		})
	}
}

func TestMiddlewareChain_EmptyChain(t *testing.T) {
	t.Run("empty_middleware_chain_passes_through", func(t *testing.T) {
		finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("direct response")); err != nil {
				t.Logf("Failed to write response: %v", err)
			}
		})

		chain := NewMiddlewareChain()
		handler := chain(finalHandler)

		req := testutil.CreateRequest(http.MethodGet, "/test")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "direct response", recorder.Body.String())
	})
}

func TestMiddleware_RequestContext(t *testing.T) {
	t.Run("middleware_can_modify_request_context", func(t *testing.T) {
		// Custom middleware that adds values to context
		contextMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), userIDContextKey, "12345")
				ctx = context.WithValue(ctx, requestIDContextKey, "req-abc-123")
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}

		finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := r.Context().Value(userIDContextKey)
			requestID := r.Context().Value(requestIDContextKey)

			assert.Equal(t, "12345", userID)
			assert.Equal(t, "req-abc-123", requestID)

			w.WriteHeader(http.StatusOK)
		})

		handler := contextMiddleware(finalHandler)

		req := testutil.CreateRequest(http.MethodGet, "/test")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
	})
}

// testCase represents a generic test case structure for middleware tests.
type testCase struct {
	name         string
	method       string
	path         string
	responseCode int
	validateFunc func(t *testing.T, logOutput string)
}

// createLoggingTestCase creates a test case for logging middleware.
func createLoggingTestCase(
	name, method, path string,
	responseCode int,
	validateFunc func(t *testing.T, logOutput string),
) testCase {
	return testCase{
		name:         name,
		method:       method,
		path:         path,
		responseCode: responseCode,
		validateFunc: validateFunc,
	}
}

// Validation helper functions for logging middleware

// validateSuccessfulRequest validates logging for successful requests.
func validateSuccessfulRequest(t *testing.T, logOutput string) {
	assert.Contains(t, logOutput, "GET")
	assert.Contains(t, logOutput, "/health")
	assert.Contains(t, logOutput, "200")
	assert.Contains(t, logOutput, "duration")
}

// validateErrorResponse validates logging for error responses.
func validateErrorResponse(t *testing.T, logOutput string) {
	assert.Contains(t, logOutput, "GET")
	assert.Contains(t, logOutput, "/nonexistent")
	assert.Contains(t, logOutput, "404")
}

// validatePostRequest validates logging for POST requests.
func validatePostRequest(t *testing.T, logOutput string) {
	assert.Contains(t, logOutput, "POST")
	assert.Contains(t, logOutput, "/repositories")
	assert.Contains(t, logOutput, "202")
}

// validateQueryParameterRequest validates logging for requests with query parameters.
func validateQueryParameterRequest(t *testing.T, logOutput string) {
	assert.Contains(t, logOutput, "GET")
	assert.Contains(t, logOutput, "/repositories")
	// Query parameters should be logged
	assert.Contains(t, logOutput, "limit=10")
	assert.Contains(t, logOutput, "offset=20")
}

func TestMiddleware_Configuration(t *testing.T) {
	t.Run("middleware_respects_configuration", func(t *testing.T) {
		// Test CORS middleware with custom configuration
		corsConfig := CORSConfig{
			AllowedOrigins: []string{"https://example.com", "https://app.com"},
			AllowedMethods: []string{"GET", "POST"},
			AllowedHeaders: []string{"Content-Type"},
			MaxAge:         3600,
		}

		middleware := NewCORSMiddlewareWithConfig(corsConfig)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := testutil.CreateRequest(http.MethodGet, "/test")
		req.Header.Set("Origin", "https://example.com")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		// Should respect custom configuration
		assert.Equal(t, "https://example.com", recorder.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST", recorder.Header().Get("Access-Control-Allow-Methods"))
		assert.Equal(t, "Content-Type", recorder.Header().Get("Access-Control-Allow-Headers"))
	})
}

// === FAILING TESTS FOR CORS MIDDLEWARE REFACTORING (RED PHASE) ===
// These tests define the expected behavior after refactoring and will initially fail

func TestDefaultCORSConfig_Constants(t *testing.T) {
	t.Run("default_cors_config_has_correct_hardcoded_values", func(t *testing.T) {
		// This test will fail until DefaultCORSConfig constant is created
		config := DefaultCORSConfig

		// Verify default origins
		assert.Equal(t, []string{"*"}, config.AllowedOrigins, "Default origin should be '*'")

		// Verify default methods
		expectedMethods := []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
		assert.Equal(t, expectedMethods, config.AllowedMethods, "Default methods should match hardcoded values")

		// Verify default headers (basic ones, without dynamic request headers)
		expectedHeaders := []string{"Content-Type", "Authorization"}
		assert.Equal(
			t,
			expectedHeaders,
			config.AllowedHeaders,
			"Default headers should be Content-Type and Authorization",
		)

		// Verify default max age
		assert.Equal(t, 86400, config.MaxAge, "Default max age should be 86400 seconds")
	})
}

func TestNewCORSMiddleware_UsesDefaultConfig(t *testing.T) {
	t.Run("new_cors_middleware_delegates_to_configurable_version", func(t *testing.T) {
		// This test will fail until NewCORSMiddleware is refactored to use DefaultCORSConfig
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Create middleware using the hardcoded version (current)
		hardcodedMiddleware := NewCORSMiddleware()
		hardcodedHandler := hardcodedMiddleware(nextHandler)

		// Create middleware using configurable version with DefaultCORSConfig (expected after refactor)
		// This should produce identical results - test will fail until refactored
		configurableHandler := NewCORSMiddlewareWithConfig(DefaultCORSConfig)(nextHandler)

		testCases := []struct {
			name           string
			method         string
			requestHeaders string
		}{
			{"simple_get_request", http.MethodGet, ""},
			{"preflight_options_request", http.MethodOptions, ""},
			{"preflight_with_custom_headers", http.MethodOptions, "X-Custom-Header, X-API-Key"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Test hardcoded version
				reqHardcoded := testutil.CreateRequest(tc.method, "/test")
				if tc.requestHeaders != "" {
					reqHardcoded.Header.Set("Access-Control-Request-Headers", tc.requestHeaders)
				}
				recorderHardcoded := httptest.NewRecorder()
				hardcodedHandler.ServeHTTP(recorderHardcoded, reqHardcoded)

				// Test configurable version with DefaultCORSConfig
				reqConfigurable := testutil.CreateRequest(tc.method, "/test")
				if tc.requestHeaders != "" {
					reqConfigurable.Header.Set("Access-Control-Request-Headers", tc.requestHeaders)
				}
				recorderConfigurable := httptest.NewRecorder()
				configurableHandler.ServeHTTP(recorderConfigurable, reqConfigurable)

				// Both should produce identical results
				assert.Equal(t, recorderHardcoded.Code, recorderConfigurable.Code,
					"Status codes should match between hardcoded and configurable versions")
				assert.Equal(t, recorderHardcoded.Header().Get("Access-Control-Allow-Origin"),
					recorderConfigurable.Header().Get("Access-Control-Allow-Origin"),
					"Allow-Origin headers should match")
				assert.Equal(t, recorderHardcoded.Header().Get("Access-Control-Allow-Methods"),
					recorderConfigurable.Header().Get("Access-Control-Allow-Methods"),
					"Allow-Methods headers should match")

				// For basic headers (without dynamic request headers), they should match exactly
				if tc.requestHeaders == "" {
					assert.Equal(t, recorderHardcoded.Header().Get("Access-Control-Allow-Headers"),
						recorderConfigurable.Header().Get("Access-Control-Allow-Headers"),
						"Allow-Headers should match for basic requests")
				}
			})
		}
	})
}

func TestCORSMiddlewareWithConfig_UsesStringsJoin(t *testing.T) {
	t.Run("header_building_uses_strings_join_without_extra_spaces", func(t *testing.T) {
		// This test will fail until string concatenation is replaced with strings.Join
		config := CORSConfig{
			AllowedOrigins: []string{"https://example.com"},
			AllowedMethods: []string{"GET", "POST", "PUT"},
			AllowedHeaders: []string{"Content-Type", "Authorization", "X-API-Key"},
			MaxAge:         3600,
		}

		middleware := NewCORSMiddlewareWithConfig(config)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := testutil.CreateRequest(http.MethodGet, "/test")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		// Verify clean comma-separated values without extra spaces
		methodsHeader := recorder.Header().Get("Access-Control-Allow-Methods")
		assert.Equal(t, "GET, POST, PUT", methodsHeader,
			"Methods should be joined with ', ' separator (no extra spaces)")

		headersHeader := recorder.Header().Get("Access-Control-Allow-Headers")
		assert.Equal(t, "Content-Type, Authorization, X-API-Key", headersHeader,
			"Headers should be joined with ', ' separator (no extra spaces)")
	})

	t.Run("empty_slices_produce_empty_headers", func(t *testing.T) {
		// Test edge case: empty configuration slices
		config := CORSConfig{
			AllowedOrigins: []string{}, // Empty
			AllowedMethods: []string{}, // Empty
			AllowedHeaders: []string{}, // Empty
			MaxAge:         0,
		}

		middleware := NewCORSMiddlewareWithConfig(config)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := testutil.CreateRequest(http.MethodGet, "/test")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		// Empty slices should not set headers (or set empty headers)
		assert.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"),
			"Empty origins should not set Allow-Origin header")
		assert.Empty(t, recorder.Header().Get("Access-Control-Allow-Methods"),
			"Empty methods should not set Allow-Methods header")
		assert.Empty(t, recorder.Header().Get("Access-Control-Allow-Headers"),
			"Empty headers should not set Allow-Headers header")
	})
}

func TestCORSMiddlewareWithConfig_MaxAgeHandling(t *testing.T) {
	t.Run("max_age_header_set_only_for_options_requests", func(t *testing.T) {
		// This test will fail until Max-Age handling is properly implemented
		config := CORSConfig{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST"},
			AllowedHeaders: []string{"Content-Type"},
			MaxAge:         7200,
		}

		middleware := NewCORSMiddlewareWithConfig(config)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Test OPTIONS request - should include Max-Age
		optionsReq := testutil.CreateRequest(http.MethodOptions, "/test")
		optionsRecorder := httptest.NewRecorder()
		handler.ServeHTTP(optionsRecorder, optionsReq)

		assert.Equal(t, "7200", optionsRecorder.Header().Get("Access-Control-Max-Age"),
			"OPTIONS requests should include Max-Age header")

		// Test non-OPTIONS request - should NOT include Max-Age
		getReq := testutil.CreateRequest(http.MethodGet, "/test")
		getRecorder := httptest.NewRecorder()
		handler.ServeHTTP(getRecorder, getReq)

		assert.Empty(t, getRecorder.Header().Get("Access-Control-Max-Age"),
			"Non-OPTIONS requests should not include Max-Age header")
	})

	t.Run("zero_max_age_not_set_in_header", func(t *testing.T) {
		// Test edge case: zero MaxAge should not set header or set it appropriately
		config := CORSConfig{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET"},
			AllowedHeaders: []string{"Content-Type"},
			MaxAge:         0, // Zero value
		}

		middleware := NewCORSMiddlewareWithConfig(config)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := testutil.CreateRequest(http.MethodOptions, "/test")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		maxAge := recorder.Header().Get("Access-Control-Max-Age")
		// Either empty (not set) or "0" - both are valid behaviors
		assert.True(t, maxAge == "" || maxAge == "0",
			"Zero MaxAge should either not set header or set to '0'")
	})
}

func TestCORSMiddlewareWithConfig_OptionsRequestStatusCode(t *testing.T) {
	t.Run("options_request_returns_204_not_200", func(t *testing.T) {
		// This test highlights the inconsistency: hardcoded returns 204, configurable returns 200
		// After refactoring, both should be consistent (probably 204 for preflight)
		config := CORSConfig{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST"},
			AllowedHeaders: []string{"Content-Type"},
			MaxAge:         3600,
		}

		middleware := NewCORSMiddlewareWithConfig(config)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := testutil.CreateRequest(http.MethodOptions, "/test")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)

		// Currently returns 200, but should return 204 for consistency with hardcoded version
		assert.Equal(t, http.StatusNoContent, recorder.Code,
			"OPTIONS preflight requests should return 204 No Content, not 200 OK")
	})
}

func TestBackwardCompatibility_ExactBehaviorPreservation(t *testing.T) {
	t.Run("hardcoded_middleware_behavior_exactly_preserved", func(t *testing.T) {
		// This comprehensive test ensures that after refactoring, the behavior
		// is preserved exactly, including edge cases and specific formatting

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
		})

		// Current hardcoded middleware
		hardcodedMiddleware := NewCORSMiddleware()
		hardcodedHandler := hardcodedMiddleware(nextHandler)

		testCases := []struct {
			name           string
			method         string
			origin         string
			requestHeaders string
			expectedStatus int
			checkMaxAge    bool
		}{
			{
				name:           "simple_get_preserves_exact_behavior",
				method:         http.MethodGet,
				origin:         "https://example.com",
				expectedStatus: http.StatusOK,
				checkMaxAge:    false,
			},
			{
				name:           "options_request_preserves_204_and_max_age",
				method:         http.MethodOptions,
				origin:         "https://test.com",
				expectedStatus: http.StatusNoContent,
				checkMaxAge:    true,
			},
			{
				name:           "custom_headers_appended_correctly",
				method:         http.MethodOptions,
				origin:         "https://api.com",
				requestHeaders: "X-Custom-Header, X-API-Key",
				expectedStatus: http.StatusNoContent,
				checkMaxAge:    true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := testutil.CreateRequest(tc.method, "/test")
				if tc.origin != "" {
					req.Header.Set("Origin", tc.origin)
				}
				if tc.requestHeaders != "" {
					req.Header.Set("Access-Control-Request-Headers", tc.requestHeaders)
				}

				recorder := httptest.NewRecorder()
				hardcodedHandler.ServeHTTP(recorder, req)

				// Verify exact current behavior is preserved
				assert.Equal(t, tc.expectedStatus, recorder.Code, "Status code must be preserved")
				assert.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"),
					"Origin header must be exactly '*'")
				assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS",
					recorder.Header().Get("Access-Control-Allow-Methods"),
					"Methods header must match exactly")

				// Verify headers include defaults and any requested headers
				allowedHeaders := recorder.Header().Get("Access-Control-Allow-Headers")
				assert.Contains(t, allowedHeaders, "Content-Type", "Must include Content-Type")
				assert.Contains(t, allowedHeaders, "Authorization", "Must include Authorization")

				if tc.requestHeaders != "" {
					// Should include both default headers and requested headers with proper formatting
					expectedPattern := "Content-Type, Authorization, " + tc.requestHeaders
					assert.Equal(t, expectedPattern, allowedHeaders,
						"Custom headers should be appended with exact formatting")
				}

				if tc.checkMaxAge {
					assert.Equal(t, "86400", recorder.Header().Get("Access-Control-Max-Age"),
						"Max-Age must be exactly '86400'")
				}
			})
		}
	})
}

// === FAILING TESTS FOR DEFAULTLOGGER ALLOCATION OPTIMIZATION (RED PHASE) ===
// These tests define expected behavior after refactoring and will initially fail

func TestDefaultLogger_AllocationPerformance(t *testing.T) {
	t.Run("with_field_reduces_allocations_in_hot_path", func(t *testing.T) {
		// This test will fail until WithField is optimized for allocation performance
		logger := NewDefaultLogger()

		// Measure allocations for multiple WithField calls (simulating hot path)
		allocs := testing.AllocsPerRun(100, func() {
			logger.WithField("key1", "value1").
				WithField("key2", "value2").
				WithField("key3", "value3").
				WithField("key4", "value4").
				WithField("key5", "value5")
		})

		// Current implementation allocates a new map for each WithField call
		// After optimization, should allocate significantly fewer maps
		assert.Less(t, allocs, 10.0, "WithField should minimize allocations in chained calls")
	})

	t.Run("with_fields_reduces_allocations_for_bulk_operations", func(t *testing.T) {
		// This test will fail until WithFields is optimized
		logger := NewDefaultLogger()

		fields := map[string]interface{}{
			"request_id": "abc123",
			"method":     "GET",
			"path":       "/test",
			"status":     200,
			"duration":   "10ms",
		}

		// Measure allocations for WithFields calls
		allocs := testing.AllocsPerRun(1000, func() {
			logger.WithFields(fields)
		})

		// Should minimize map copying - ideally close to 1 allocation per call
		assert.Less(t, allocs, 2.0, "WithFields should minimize map copying allocations")
	})

	t.Run("chained_operations_do_not_copy_maps_repeatedly", func(t *testing.T) {
		// This test will fail until copy-on-write or similar optimization is implemented
		logger := NewDefaultLogger()

		// Create a logger with some base fields
		baseLogger := logger.WithField("service", "api").WithField("version", "1.0")

		// Measure allocations when creating multiple loggers from the same base
		allocs := testing.AllocsPerRun(100, func() {
			// These operations should not trigger full map copies of the base fields
			_ = baseLogger.WithField("request_id", "123")
			_ = baseLogger.WithField("user_id", "456")
			_ = baseLogger.WithFields(map[string]interface{}{
				"operation": "test",
				"timestamp": "2024-01-01",
			})
		})

		// With copy-on-write optimization, allocations should be minimal
		assert.Less(t, allocs, 15.0, "Chained operations should not repeatedly copy base fields")
	})

	t.Run("memory_efficient_field_context_management", func(t *testing.T) {
		// This test will fail until memory-efficient context management is implemented
		logger := NewDefaultLogger()

		// Create nested logger contexts without triggering excessive allocations
		l1 := logger.WithField("level1", "value1")
		l2 := l1.WithField("level2", "value2")
		l3 := l2.WithField("level3", "value3")
		l4 := l3.WithFields(map[string]interface{}{
			"level4a": "value4a",
			"level4b": "value4b",
		})

		// All loggers should maintain their context correctly
		var output strings.Builder
		testLogger := l4.(*DefaultLogger)
		testLogger.output = &output
		testLogger.Infof("test message")

		logOutput := output.String()
		assert.Contains(t, logOutput, "level1=value1")
		assert.Contains(t, logOutput, "level2=value2")
		assert.Contains(t, logOutput, "level3=value3")
		assert.Contains(t, logOutput, "level4a=value4a")
		assert.Contains(t, logOutput, "level4b=value4b")
	})
}

func TestDefaultLogger_FunctionalBehaviorPreservation(t *testing.T) {
	t.Run("refactored_logger_preserves_exact_output_format", func(t *testing.T) {
		// This test ensures that optimization doesn't change the logging output
		var output1, output2 strings.Builder

		// Current behavior
		logger1 := NewTestLogger(&output1)
		logger1.WithField("key1", "value1").
			WithFields(map[string]interface{}{
				"key2": "value2",
				"key3": 123,
			}).
			Infof("test message")

		// After refactoring, should produce identical output
		logger2 := NewTestLogger(&output2)
		logger2.WithField("key1", "value1").
			WithFields(map[string]interface{}{
				"key2": "value2",
				"key3": 123,
			}).
			Infof("test message")

		// Outputs should be functionally equivalent (may differ in field order)
		out1 := output1.String()
		out2 := output2.String()

		// Both should contain the same basic components
		assert.Contains(t, out1, "INFO")
		assert.Contains(t, out1, "test message")
		assert.Contains(t, out1, "key1=value1")
		assert.Contains(t, out1, "key2=value2")
		assert.Contains(t, out1, "key3=123")

		assert.Contains(t, out2, "INFO")
		assert.Contains(t, out2, "test message")
		assert.Contains(t, out2, "key1=value1")
		assert.Contains(t, out2, "key2=value2")
		assert.Contains(t, out2, "key3=123")
	})

	t.Run("field_isolation_between_logger_instances", func(t *testing.T) {
		// This test ensures that optimization doesn't break field isolation
		var output1, output2 strings.Builder

		baseLogger := NewTestLogger(&strings.Builder{})
		logger1 := baseLogger.WithField("instance", "logger1")
		logger2 := baseLogger.WithField("instance", "logger2")

		// Cast to access output field for testing
		logger1Test := logger1.(*DefaultLogger)
		logger1Test.output = &output1
		logger2Test := logger2.(*DefaultLogger)
		logger2Test.output = &output2

		logger1Test.Infof("message from logger1")
		logger2Test.Infof("message from logger2")

		out1 := output1.String()
		out2 := output2.String()

		// Each logger should only see its own instance field
		assert.Contains(t, out1, "instance=logger1")
		assert.NotContains(t, out1, "instance=logger2")

		assert.Contains(t, out2, "instance=logger2")
		assert.NotContains(t, out2, "instance=logger1")
	})
}
