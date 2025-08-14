package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewSecurityHeadersMiddleware_DelegatestoUnified tests that the legacy
// NewSecurityHeadersMiddleware function delegates to the unified implementation
// instead of setting hardcoded headers.
func TestNewSecurityHeadersMiddleware_DelegatestoUnified(t *testing.T) {
	tests := []struct {
		name        string
		request     *http.Request
		expectHSTS  bool
		description string
	}{
		{
			name:        "delegates_to_unified_for_https_requests",
			request:     createHTTPSRequest("GET", "/test"),
			expectHSTS:  true,
			description: "HTTPS requests should delegate to unified middleware with HSTS",
		},
		{
			name:        "delegates_to_unified_for_http_requests",
			request:     createHTTPRequest("GET", "/test"),
			expectHSTS:  false,
			description: "HTTP requests should delegate to unified middleware without HSTS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// FAILING TEST: This test expects NewSecurityHeadersMiddleware to delegate
			// to the unified implementation, but it currently sets hardcoded headers.

			legacyMiddleware := NewSecurityHeadersMiddleware()
			unifiedMiddleware := NewUnifiedSecurityMiddleware(nil)

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			})

			// Apply both middlewares to separate handlers
			legacyHandler := legacyMiddleware(nextHandler)
			unifiedHandler := unifiedMiddleware(nextHandler)

			// Execute requests
			legacyRecorder := httptest.NewRecorder()
			unifiedRecorder := httptest.NewRecorder()

			legacyHandler.ServeHTTP(legacyRecorder, tt.request)
			unifiedHandler.ServeHTTP(unifiedRecorder, tt.request)

			// EXPECTATION: Headers should be identical because legacy delegates to unified
			securityHeaders := []string{
				"X-Content-Type-Options",
				"X-Frame-Options",
				"X-XSS-Protection",
				"Referrer-Policy",
				"Content-Security-Policy",
			}

			for _, header := range securityHeaders {
				legacyValue := legacyRecorder.Header().Get(header)
				unifiedValue := unifiedRecorder.Header().Get(header)
				assert.Equal(
					t,
					unifiedValue,
					legacyValue,
					"Header %s should be identical between legacy and unified middleware (delegation should ensure this)",
					header,
				)
			}

			// HSTS header handling
			legacyHSTS := legacyRecorder.Header().Get("Strict-Transport-Security")
			unifiedHSTS := unifiedRecorder.Header().Get("Strict-Transport-Security")

			if tt.expectHSTS {
				assert.NotEmpty(t, legacyHSTS, "Legacy middleware should set HSTS for HTTPS requests")
				assert.Equal(t, unifiedHSTS, legacyHSTS, "HSTS header should be identical (delegation)")
			} else {
				assert.Empty(t, legacyHSTS, "Legacy middleware should not set HSTS for HTTP requests")
				assert.Equal(t, unifiedHSTS, legacyHSTS, "HSTS absence should be identical (delegation)")
			}
		})
	}
}

// TestNewSecurityHeadersMiddleware_NoHardcodedHeaders verifies that the refactored
// NewSecurityHeadersMiddleware does not contain hardcoded header values and
// properly delegates to the unified implementation with default configuration.
func TestNewSecurityHeadersMiddleware_NoHardcodedHeaders(t *testing.T) {
	t.Run("uses_default_unified_configuration", func(t *testing.T) {
		// CORRECTED TEST: This test verifies that NewSecurityHeadersMiddleware
		// delegates to the unified implementation with DEFAULT configuration
		// for backward compatibility. It should match unified default behavior.

		// For backward compatibility, NewSecurityHeadersMiddleware should delegate
		// to NewUnifiedSecurityMiddleware(nil) which uses default configuration
		expectedHeaders := map[string]string{
			"Referrer-Policy":         "strict-origin-when-cross-origin", // Unified default
			"Content-Security-Policy": "default-src 'self'",              // Unified default
			"X-Frame-Options":         "DENY",                            // Unified default
			"X-XSS-Protection":        "1; mode=block",                   // Unified default
			"X-Content-Type-Options":  "nosniff",                         // Unified default
		}

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Legacy middleware should delegate to unified implementation with default config
		legacyMiddleware := NewSecurityHeadersMiddleware()
		handler := legacyMiddleware(nextHandler)

		req := createHTTPSRequest("GET", "/test")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		// These assertions verify that legacy middleware uses unified defaults
		for headerName, expectedValue := range expectedHeaders {
			actualValue := recorder.Header().Get(headerName)
			assert.Equal(
				t,
				expectedValue,
				actualValue,
				"Legacy middleware should delegate to unified implementation with default config for header: %s",
				headerName,
			)
		}

		// HSTS should be present for HTTPS requests
		hstsHeader := recorder.Header().Get("Strict-Transport-Security")
		assert.Contains(t, hstsHeader, "max-age=31536000",
			"HSTS should be set for HTTPS requests by unified default configuration")
	})
}

// TestCreateSecurityMiddlewareStack_UsesUnifiedMiddleware tests that the
// CreateSecurityMiddlewareStack function uses the unified middleware instead
// of calling the legacy NewSecurityHeadersMiddleware directly.
func TestCreateSecurityMiddlewareStack_UsesUnifiedMiddleware(t *testing.T) {
	t.Run("security_stack_uses_unified_implementation", func(t *testing.T) {
		// FAILING TEST: This test expects CreateSecurityMiddlewareStack to use
		// the unified middleware, but it currently calls the legacy implementation.

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})

		// Create handlers using both approaches
		stackMiddleware := CreateSecurityMiddlewareStack()
		stackHandler := stackMiddleware(nextHandler)

		// Create a "proper" unified stack manually
		unifiedSecurityMiddleware := NewUnifiedSecurityMiddleware(nil)
		unifiedHandler := unifiedSecurityMiddleware(nextHandler)

		// Test with HTTPS request
		req := createHTTPSRequest("GET", "/test")

		stackRecorder := httptest.NewRecorder()
		unifiedRecorder := httptest.NewRecorder()

		stackHandler.ServeHTTP(stackRecorder, req)
		unifiedHandler.ServeHTTP(unifiedRecorder, req)

		// EXPECTATION: Security headers should be identical because stack uses unified middleware
		securityHeaders := []string{
			"X-Content-Type-Options",
			"X-Frame-Options",
			"X-XSS-Protection",
			"Referrer-Policy",
			"Content-Security-Policy",
			"Strict-Transport-Security",
		}

		for _, header := range securityHeaders {
			stackValue := stackRecorder.Header().Get(header)
			unifiedValue := unifiedRecorder.Header().Get(header)
			assert.Equal(t, unifiedValue, stackValue,
				"Security stack should use unified middleware for header: %s", header)
		}
	})
}

// TestCreateSecurityMiddlewareStack_ConfigurableHeaders tests that the security
// middleware stack can be made configurable by using the unified implementation.
func TestCreateSecurityMiddlewareStack_ConfigurableHeaders(t *testing.T) {
	t.Run("stack_allows_header_customization", func(t *testing.T) {
		// FAILING TEST: This test expects the security stack to be configurable,
		// but current implementation uses hardcoded values from legacy middleware.

		// This test defines the expected API after refactoring
		// The stack should either accept configuration or use a configurable unified middleware

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Current API limitation: CreateSecurityMiddlewareStack() takes no parameters
		// After refactoring, it should use unified configurable middleware internally
		stackMiddleware := CreateSecurityMiddlewareStack()
		handler := stackMiddleware(nextHandler)

		req := createHTTPSRequest("GET", "/test")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		// EXPECTATION: Headers should match unified middleware defaults, not hardcoded values
		// This validates that the stack uses unified implementation
		expectedHeaders := map[string]string{
			"X-Content-Type-Options":  "nosniff",
			"X-Frame-Options":         "DENY",
			"X-XSS-Protection":        "1; mode=block",
			"Referrer-Policy":         "strict-origin-when-cross-origin",
			"Content-Security-Policy": "default-src 'self'", // Unified default, not hardcoded comprehensive
		}

		for headerName, expectedValue := range expectedHeaders {
			actualValue := recorder.Header().Get(headerName)
			assert.Equal(t, expectedValue, actualValue,
				"Security stack should use unified middleware defaults for header: %s", headerName)
		}

		// HSTS should be set with unified defaults
		hstsValue := recorder.Header().Get("Strict-Transport-Security")
		assert.Equal(t, "max-age=31536000; includeSubDomains", hstsValue,
			"HSTS should use unified middleware default values")
	})
}

// TestSecurityMiddleware_BackwardCompatibilityPreserved verifies that the
// refactoring preserves all existing behavior for consumers of the API.
func TestSecurityMiddleware_BackwardCompatibilityPreserved(t *testing.T) {
	tests := []struct {
		name           string
		middlewareFunc func() func(http.Handler) http.Handler
		description    string
	}{
		{
			name:           "NewSecurityHeadersMiddleware_preserves_api",
			middlewareFunc: NewSecurityHeadersMiddleware,
			description:    "Legacy NewSecurityHeadersMiddleware should still work after refactoring",
		},
		{
			name:           "CreateSecurityMiddlewareStack_preserves_api",
			middlewareFunc: CreateSecurityMiddlewareStack,
			description:    "CreateSecurityMiddlewareStack should still work after refactoring",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// EXPECTATION: APIs should continue to work exactly as before
			middleware := tt.middlewareFunc()
			require.NotNil(t, middleware, "Middleware function should not be nil")

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			})

			handler := middleware(nextHandler)
			require.NotNil(t, handler, "Middleware should return a valid handler")

			// Test basic functionality
			req := createHTTPSRequest("GET", "/test")
			recorder := httptest.NewRecorder()

			// Should not panic
			require.NotPanics(t, func() {
				handler.ServeHTTP(recorder, req)
			}, "Middleware should not panic after refactoring")

			// Should set security headers
			assert.NotEmpty(t, recorder.Header().Get("X-Content-Type-Options"), "Should set X-Content-Type-Options")
			assert.NotEmpty(t, recorder.Header().Get("X-Frame-Options"), "Should set X-Frame-Options")
			assert.NotEmpty(t, recorder.Header().Get("X-XSS-Protection"), "Should set X-XSS-Protection")
			assert.NotEmpty(t, recorder.Header().Get("Referrer-Policy"), "Should set Referrer-Policy")
			assert.NotEmpty(t, recorder.Header().Get("Content-Security-Policy"), "Should set Content-Security-Policy")

			// HSTS should be set for HTTPS
			assert.NotEmpty(t, recorder.Header().Get("Strict-Transport-Security"), "Should set HSTS for HTTPS")

			// Response should be successful
			assert.Equal(t, http.StatusOK, recorder.Code, "Middleware should not change response status")
		})
	}
}

// TestSecurityMiddleware_ConsistentHeaderBehavior verifies that all middleware
// paths produce consistent security headers after refactoring.
func TestSecurityMiddleware_ConsistentHeaderBehavior(t *testing.T) {
	t.Run("all_middleware_paths_produce_identical_headers", func(t *testing.T) {
		// FAILING TEST: This test expects all middleware paths to produce identical headers
		// after delegation to unified implementation.

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Create handlers using different middleware paths
		middlewares := map[string]func(http.Handler) http.Handler{
			"NewSecurityHeadersMiddleware":  NewSecurityHeadersMiddleware(),
			"CreateSecurityMiddlewareStack": CreateSecurityMiddlewareStack(),
			"NewUnifiedSecurityMiddleware":  NewUnifiedSecurityMiddleware(nil),
		}

		req := createHTTPSRequest("GET", "/test")
		recorders := make(map[string]*httptest.ResponseRecorder)

		// Execute request through each middleware path
		for name, middleware := range middlewares {
			handler := middleware(nextHandler)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			recorders[name] = recorder
		}

		// Get unified middleware headers as reference
		referenceHeaders := recorders["NewUnifiedSecurityMiddleware"].Header()

		securityHeaders := []string{
			"X-Content-Type-Options",
			"X-Frame-Options",
			"X-XSS-Protection",
			"Referrer-Policy",
			"Content-Security-Policy",
			"Strict-Transport-Security",
		}

		// All middleware paths should produce identical headers
		for middlewareName, recorder := range recorders {
			if middlewareName == "NewUnifiedSecurityMiddleware" {
				continue // Skip reference
			}

			for _, header := range securityHeaders {
				expectedValue := referenceHeaders.Get(header)
				actualValue := recorder.Header().Get(header)
				assert.Equal(t, expectedValue, actualValue,
					"Middleware %s should produce same header %s as unified implementation", middlewareName, header)
			}
		}
	})
}

// TestSecurityMiddleware_DelegationInternals tests the internal implementation
// details to ensure proper delegation is happening.
func TestSecurityMiddleware_DelegationInternals(t *testing.T) {
	t.Run("legacy_middleware_does_not_set_hardcoded_values", func(t *testing.T) {
		// FAILING TEST: This test expects that after refactoring, the legacy middleware
		// will behave differently than the hardcoded values it currently uses.

		// Test specific hardcoded values that should change after delegation
		legacyMiddleware := NewSecurityHeadersMiddleware()
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := legacyMiddleware(nextHandler)
		req := createHTTPSRequest("GET", "/test")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		// Current hardcoded CSP in NewSecurityHeadersMiddleware is comprehensive
		// After delegation to unified, it should be basic default
		currentCSP := recorder.Header().Get("Content-Security-Policy")

		// EXPECTATION: After refactoring, this should NOT be the hardcoded comprehensive CSP
		assert.NotEqual(t, "default-src 'self'; script-src 'self'; object-src 'none'", currentCSP,
			"Legacy middleware should not use hardcoded comprehensive CSP after delegation")

		// EXPECTATION: After refactoring, should use unified default
		assert.Equal(t, "default-src 'self'", currentCSP,
			"Legacy middleware should delegate to unified default CSP")

		// Current hardcoded HSTS includes includeSubDomains
		// After delegation, should match unified default exactly
		currentHSTS := recorder.Header().Get("Strict-Transport-Security")
		assert.Equal(t, "max-age=31536000; includeSubDomains", currentHSTS,
			"Legacy middleware should delegate to unified HSTS default")
	})
}

// TestCreateSecurityMiddlewareStack_NoDuplicateHeaders verifies that after
// refactoring to use unified middleware, there are no duplicate or conflicting headers.
func TestCreateSecurityMiddlewareStack_NoDuplicateHeaders(t *testing.T) {
	t.Run("stack_does_not_duplicate_security_headers", func(t *testing.T) {
		// FAILING TEST: This test expects no header duplication after refactoring
		// Current implementation might have duplicates if it applies both legacy and unified middleware

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		stackMiddleware := CreateSecurityMiddlewareStack()
		handler := stackMiddleware(nextHandler)

		req := createHTTPSRequest("GET", "/test")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		// Check that each security header appears only once
		securityHeaders := []string{
			"X-Content-Type-Options",
			"X-Frame-Options",
			"X-XSS-Protection",
			"Referrer-Policy",
			"Content-Security-Policy",
			"Strict-Transport-Security",
		}

		for _, header := range securityHeaders {
			values := recorder.Header().Values(header)
			assert.Len(t, values, 1,
				"Header %s should appear exactly once (no duplication from multiple middleware layers)", header)
		}
	})
}

// Note: Helper functions createHTTPSRequest and createHTTPRequest are defined in unified_security_middleware_test.go
