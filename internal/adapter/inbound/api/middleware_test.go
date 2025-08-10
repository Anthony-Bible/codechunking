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

func TestLoggingMiddleware(t *testing.T) {
	tests := []testCase{
		createLoggingTestCase("logs_successful_request", http.MethodGet, "/health", http.StatusOK,
			validateSuccessfulRequest),
		createLoggingTestCase("logs_error_response", http.MethodGet, "/nonexistent", http.StatusNotFound,
			validateErrorResponse),
		createLoggingTestCase("logs_post_request_with_body", http.MethodPost, "/repositories", http.StatusAccepted,
			validatePostRequest),
		createLoggingTestCase("logs_request_with_query_parameters", http.MethodGet, "/repositories?limit=10&offset=20", http.StatusOK,
			validateQueryParameterRequest),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			var logOutput strings.Builder
			logger := NewTestLogger(&logOutput)

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseCode)
				w.Write([]byte("response body"))
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
				assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", recorder.Header().Get("Access-Control-Allow-Methods"))
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
				assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", recorder.Header().Get("Access-Control-Allow-Methods"))
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
				w.Write([]byte("success"))
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
				w.Write([]byte("normal response"))
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
				r = r.WithContext(context.WithValue(r.Context(), "error", "validation_failed"))
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
					w.Write([]byte("success"))
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
			w.Write([]byte("direct response"))
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
				ctx := context.WithValue(r.Context(), "user_id", "12345")
				ctx = context.WithValue(ctx, "request_id", "req-abc-123")
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}

		finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := r.Context().Value("user_id")
			requestID := r.Context().Value("request_id")

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

// middlewareTestFramework provides a unified testing framework for middleware
type middlewareTestFramework struct {
	logOutput *strings.Builder
	logger    Logger // Test logger interface
}

// newMiddlewareTestFramework creates a new middleware testing framework
func newMiddlewareTestFramework() *middlewareTestFramework {
	logOutput := &strings.Builder{}
	return &middlewareTestFramework{
		logOutput: logOutput,
		logger:    NewTestLogger(logOutput),
	}
}

// testCase represents a generic test case structure for middleware tests
type testCase struct {
	name         string
	method       string
	path         string
	responseCode int
	validateFunc func(t *testing.T, logOutput string)
}

// corsTestCase represents a test case structure for CORS middleware tests
type corsTestCase struct {
	name           string
	method         string
	origin         string
	requestHeaders string
	validateFunc   func(t *testing.T, recorder *httptest.ResponseRecorder)
}

// errorTestCase represents a test case structure for error middleware tests
type errorTestCase struct {
	name         string
	handlerFunc  http.HandlerFunc
	expectedCode int
	validateFunc func(t *testing.T, recorder *httptest.ResponseRecorder)
}

// testCaseBuilder provides a fluent interface for building test cases
type testCaseBuilder struct{}

// newTestCaseBuilder creates a new test case builder
func newTestCaseBuilder() *testCaseBuilder {
	return &testCaseBuilder{}
}

// createLoggingTestCase creates a test case for logging middleware
func (tb *testCaseBuilder) createLoggingTestCase(name, method, path string, responseCode int, validateFunc func(t *testing.T, logOutput string)) testCase {
	return testCase{
		name:         name,
		method:       method,
		path:         path,
		responseCode: responseCode,
		validateFunc: validateFunc,
	}
}

// createCORSTestCase creates a test case for CORS middleware
func (tb *testCaseBuilder) createCORSTestCase(name, method, origin, requestHeaders string, validateFunc func(t *testing.T, recorder *httptest.ResponseRecorder)) corsTestCase {
	return corsTestCase{
		name:           name,
		method:         method,
		origin:         origin,
		requestHeaders: requestHeaders,
		validateFunc:   validateFunc,
	}
}

// createErrorTestCase creates a test case for error middleware
func (tb *testCaseBuilder) createErrorTestCase(name string, handlerFunc http.HandlerFunc, expectedCode int, validateFunc func(t *testing.T, recorder *httptest.ResponseRecorder)) errorTestCase {
	return errorTestCase{
		name:         name,
		handlerFunc:  handlerFunc,
		expectedCode: expectedCode,
		validateFunc: validateFunc,
	}
}

// Legacy functions for backward compatibility
func createLoggingTestCase(name, method, path string, responseCode int, validateFunc func(t *testing.T, logOutput string)) testCase {
	return newTestCaseBuilder().createLoggingTestCase(name, method, path, responseCode, validateFunc)
}

func createCORSTestCase(name, method, origin, requestHeaders string, validateFunc func(t *testing.T, recorder *httptest.ResponseRecorder)) corsTestCase {
	return newTestCaseBuilder().createCORSTestCase(name, method, origin, requestHeaders, validateFunc)
}

func createErrorTestCase(name string, handlerFunc http.HandlerFunc, expectedCode int, validateFunc func(t *testing.T, recorder *httptest.ResponseRecorder)) errorTestCase {
	return newTestCaseBuilder().createErrorTestCase(name, handlerFunc, expectedCode, validateFunc)
}

// Validation helper functions for logging middleware

// validateSuccessfulRequest validates logging for successful requests
func validateSuccessfulRequest(t *testing.T, logOutput string) {
	assert.Contains(t, logOutput, "GET")
	assert.Contains(t, logOutput, "/health")
	assert.Contains(t, logOutput, "200")
	assert.Contains(t, logOutput, "duration")
}

// validateErrorResponse validates logging for error responses
func validateErrorResponse(t *testing.T, logOutput string) {
	assert.Contains(t, logOutput, "GET")
	assert.Contains(t, logOutput, "/nonexistent")
	assert.Contains(t, logOutput, "404")
}

// validatePostRequest validates logging for POST requests
func validatePostRequest(t *testing.T, logOutput string) {
	assert.Contains(t, logOutput, "POST")
	assert.Contains(t, logOutput, "/repositories")
	assert.Contains(t, logOutput, "202")
}

// validateQueryParameterRequest validates logging for requests with query parameters
func validateQueryParameterRequest(t *testing.T, logOutput string) {
	assert.Contains(t, logOutput, "GET")
	assert.Contains(t, logOutput, "/repositories")
	// Query parameters should be logged
	assert.Contains(t, logOutput, "limit=10")
	assert.Contains(t, logOutput, "offset=20")
}

// middlewareTestExecutor provides centralized test execution logic
type middlewareTestExecutor struct {
	framework *middlewareTestFramework
}

// newMiddlewareTestExecutor creates a new middleware test executor
func newMiddlewareTestExecutor(framework *middlewareTestFramework) *middlewareTestExecutor {
	return &middlewareTestExecutor{framework: framework}
}

// executeLoggingTest executes a logging middleware test with improved organization
func (e *middlewareTestExecutor) executeLoggingTest(t *testing.T, tt testCase) {
	nextHandler := createMockHandler(tt.responseCode)
	middleware := NewLoggingMiddleware(e.framework.logger)
	handler := middleware(nextHandler)

	req := createTestRequest(tt.method, tt.path)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, tt.responseCode, recorder.Code)
	if tt.validateFunc != nil {
		tt.validateFunc(t, e.framework.logOutput.String())
	}
}

// executeCORSTest executes a CORS middleware test with improved organization
func (e *middlewareTestExecutor) executeCORSTest(t *testing.T, tt corsTestCase) {
	nextHandler := createMockHandler(http.StatusOK)
	middleware := NewCORSMiddleware()
	handler := middleware(nextHandler)

	req := createTestRequest(tt.method, "/test")
	if tt.origin != "" {
		req.Header.Set("Origin", tt.origin)
	}
	if tt.requestHeaders != "" {
		req.Header.Set("Access-Control-Request-Headers", tt.requestHeaders)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if tt.validateFunc != nil {
		tt.validateFunc(t, recorder)
	}
}

// executeErrorTest executes an error middleware test with improved organization
func (e *middlewareTestExecutor) executeErrorTest(t *testing.T, tt errorTestCase) {
	middleware := NewErrorHandlingMiddleware()
	handler := middleware(tt.handlerFunc)

	req := createTestRequest(http.MethodGet, "/test")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	assert.Equal(t, tt.expectedCode, recorder.Code)
	if tt.validateFunc != nil {
		tt.validateFunc(t, recorder)
	}
}

// Legacy functions for backward compatibility
func executeLoggingMiddlewareTest(t *testing.T, tt testCase) {
	framework := newMiddlewareTestFramework()
	executor := newMiddlewareTestExecutor(framework)
	executor.executeLoggingTest(t, tt)
}

func executeCORSMiddlewareTest(t *testing.T, tt corsTestCase) {
	framework := newMiddlewareTestFramework()
	executor := newMiddlewareTestExecutor(framework)
	executor.executeCORSTest(t, tt)
}

func executeErrorMiddlewareTest(t *testing.T, tt errorTestCase) {
	framework := newMiddlewareTestFramework()
	executor := newMiddlewareTestExecutor(framework)
	executor.executeErrorTest(t, tt)
}

// mockHandlerFactory provides a centralized way to create mock handlers for testing
type mockHandlerFactory struct{}

// newMockHandlerFactory creates a new mock handler factory
func newMockHandlerFactory() *mockHandlerFactory {
	return &mockHandlerFactory{}
}

// createStandardHandler creates a simple mock handler that returns the specified status code
func (m *mockHandlerFactory) createStandardHandler(statusCode int) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Write([]byte("response body"))
	})
}

// createTimingHandler creates a mock handler that sleeps for testing timing
func (m *mockHandlerFactory) createTimingHandler(duration time.Duration) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(duration)
		w.WriteHeader(http.StatusOK)
	})
}

// createPanicHandler creates a handler that panics for testing error handling
func (m *mockHandlerFactory) createPanicHandler(panicMessage string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(panicMessage)
	})
}

// testRequestFactory provides a centralized way to create test requests
type testRequestFactory struct{}

// newTestRequestFactory creates a new test request factory
func newTestRequestFactory() *testRequestFactory {
	return &testRequestFactory{}
}

// createRequest creates a test HTTP request, handling POST requests with JSON body
func (rf *testRequestFactory) createRequest(method, path string) *http.Request {
	if method == http.MethodPost {
		body := strings.NewReader(`{"url": "https://github.com/test/repo"}`)
		return testutil.CreateRequestWithBody(method, path, body)
	}
	return testutil.CreateRequest(method, path)
}

// Legacy utility functions for backward compatibility
var (
	defaultMockFactory    = newMockHandlerFactory()
	defaultRequestFactory = newTestRequestFactory()
)

// createMockHandler creates a simple mock handler that returns the specified status code
func createMockHandler(statusCode int) http.HandlerFunc {
	return defaultMockFactory.createStandardHandler(statusCode)
}

// createTestRequest creates a test HTTP request, handling POST requests with JSON body
func createTestRequest(method, path string) *http.Request {
	return defaultRequestFactory.createRequest(method, path)
}

// createSlowMockHandler creates a mock handler that sleeps for testing timing
func createSlowMockHandler(duration time.Duration) http.HandlerFunc {
	return defaultMockFactory.createTimingHandler(duration)
}

// createPanicHandler creates a handler that panics for testing error handling
func createPanicHandler(panicMessage string) http.HandlerFunc {
	return defaultMockFactory.createPanicHandler(panicMessage)
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
