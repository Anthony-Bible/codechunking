package api

import (
	"codechunking/internal/adapter/inbound/api/testutil"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper functions.
func createHTTPSRequest(method, path string) *http.Request {
	req := testutil.CreateRequest(method, path)
	req.TLS = &tls.ConnectionState{
		Version: tls.VersionTLS12,
	}
	return req
}

func createHTTPRequest(method, path string) *http.Request {
	return testutil.CreateRequest(method, path)
}

func assertSecurityHeader(t *testing.T, w *httptest.ResponseRecorder, headerName, expectedValue string) {
	t.Helper()
	actualValue := w.Header().Get(headerName)
	assert.Equal(
		t,
		expectedValue,
		actualValue,
		"Header %s should have value %s, got %s",
		headerName,
		expectedValue,
		actualValue,
	)
}

func assertNoHeader(t *testing.T, w *httptest.ResponseRecorder, headerName string) {
	t.Helper()
	actualValue := w.Header().Get(headerName)
	assert.Empty(t, actualValue, "Header %s should not be set, but got %s", headerName, actualValue)
}

func TestNewUnifiedSecurityMiddleware_DefaultConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		request     *http.Request
		expectHSTS  bool
		description string
	}{
		{
			name:        "default_config_with_https_sets_all_headers_including_hsts",
			request:     createHTTPSRequest("GET", "/test"),
			expectHSTS:  true,
			description: "HTTPS request with default config should set all security headers including HSTS",
		},
		{
			name:        "default_config_with_http_sets_all_headers_except_hsts",
			request:     createHTTPRequest("GET", "/test"),
			expectHSTS:  false,
			description: "HTTP request with default config should set all security headers except HSTS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create middleware with default configuration (nil config)
			middleware := NewUnifiedSecurityMiddleware(nil)

			// Create test handler
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			})

			handler := middleware(nextHandler)
			w := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(w, tt.request)

			// Validate response status
			assert.Equal(t, http.StatusOK, w.Code, "Middleware should not change response status")

			// Validate all security headers match current NewSecurityMiddleware() defaults
			assertSecurityHeader(t, w, "X-Content-Type-Options", "nosniff")
			assertSecurityHeader(t, w, "X-Frame-Options", "DENY")
			assertSecurityHeader(t, w, "X-XSS-Protection", "1; mode=block")
			assertSecurityHeader(t, w, "Referrer-Policy", "strict-origin-when-cross-origin")
			assertSecurityHeader(t, w, "Content-Security-Policy", "default-src 'self'")

			// HSTS should only be set for HTTPS requests
			if tt.expectHSTS {
				assertSecurityHeader(t, w, "Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			} else {
				assertNoHeader(t, w, "Strict-Transport-Security")
			}
		})
	}
}

func TestNewUnifiedSecurityMiddleware_CustomReferrerPolicy(t *testing.T) {
	tests := []struct {
		name           string
		referrerPolicy string
		expected       string
	}{
		{
			name:           "custom_referrer_policy_no_referrer",
			referrerPolicy: "no-referrer",
			expected:       "no-referrer",
		},
		{
			name:           "custom_referrer_policy_same_origin",
			referrerPolicy: "same-origin",
			expected:       "same-origin",
		},
		{
			name:           "custom_referrer_policy_origin",
			referrerPolicy: "origin",
			expected:       "origin",
		},
		{
			name:           "empty_referrer_policy_uses_default",
			referrerPolicy: "",
			expected:       "strict-origin-when-cross-origin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &SecurityHeadersConfig{
				ReferrerPolicy: tt.referrerPolicy,
			}

			middleware := NewUnifiedSecurityMiddleware(config)
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(nextHandler)
			w := httptest.NewRecorder()
			req := createHTTPSRequest("GET", "/test")

			handler.ServeHTTP(w, req)

			assertSecurityHeader(t, w, "Referrer-Policy", tt.expected)
		})
	}
}

func TestNewUnifiedSecurityMiddleware_CustomCSPPolicy(t *testing.T) {
	tests := []struct {
		name      string
		cspPolicy string
		expected  string
	}{
		{
			name:      "basic_csp_policy",
			cspPolicy: "basic",
			expected:  "default-src 'self'",
		},
		{
			name:      "comprehensive_csp_policy",
			cspPolicy: "comprehensive",
			expected:  "default-src 'self'; script-src 'self'; object-src 'none'",
		},
		{
			name:      "custom_csp_string",
			cspPolicy: "default-src 'none'; script-src 'self' 'unsafe-inline'",
			expected:  "default-src 'none'; script-src 'self' 'unsafe-inline'",
		},
		{
			name:      "empty_csp_policy_uses_default",
			cspPolicy: "",
			expected:  "default-src 'self'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &SecurityHeadersConfig{
				CSPPolicy: tt.cspPolicy,
			}

			middleware := NewUnifiedSecurityMiddleware(config)
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(nextHandler)
			w := httptest.NewRecorder()
			req := createHTTPSRequest("GET", "/test")

			handler.ServeHTTP(w, req)

			assertSecurityHeader(t, w, "Content-Security-Policy", tt.expected)
		})
	}
}

func TestNewUnifiedSecurityMiddleware_HStsConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		config        *SecurityHeadersConfig
		request       *http.Request
		expectedHSTS  string
		expectHSTSSet bool
		description   string
	}{
		{
			name: "hsts_disabled_with_https_should_not_set_header",
			config: &SecurityHeadersConfig{
				HStsEnabled: boolPtr(false),
			},
			request:       createHTTPSRequest("GET", "/test"),
			expectedHSTS:  "",
			expectHSTSSet: false,
			description:   "When HSTS is disabled, header should not be set even for HTTPS requests",
		},
		{
			name: "hsts_enabled_with_http_should_not_set_header",
			config: &SecurityHeadersConfig{
				HStsEnabled: boolPtr(true),
			},
			request:       createHTTPRequest("GET", "/test"),
			expectedHSTS:  "",
			expectHSTSSet: false,
			description:   "When request is HTTP, HSTS should not be set even if enabled",
		},
		{
			name: "hsts_enabled_with_https_uses_default_values",
			config: &SecurityHeadersConfig{
				HStsEnabled: boolPtr(true),
			},
			request:       createHTTPSRequest("GET", "/test"),
			expectedHSTS:  "max-age=31536000; includeSubDomains",
			expectHSTSSet: true,
			description:   "Default HSTS values should be used when enabled for HTTPS",
		},
		{
			name: "hsts_with_custom_max_age",
			config: &SecurityHeadersConfig{
				HStsEnabled: boolPtr(true),
				HStsMaxAge:  7776000, // 90 days
			},
			request:       createHTTPSRequest("GET", "/test"),
			expectedHSTS:  "max-age=7776000; includeSubDomains",
			expectHSTSSet: true,
			description:   "Custom max-age should be used when specified",
		},
		{
			name: "hsts_without_subdomains",
			config: &SecurityHeadersConfig{
				HStsEnabled:    boolPtr(true),
				HStsSubdomains: boolPtr(false),
			},
			request:       createHTTPSRequest("GET", "/test"),
			expectedHSTS:  "max-age=31536000",
			expectHSTSSet: true,
			description:   "HSTS without includeSubDomains when disabled",
		},
		{
			name: "hsts_with_preload",
			config: &SecurityHeadersConfig{
				HStsEnabled: boolPtr(true),
				HStsPreload: boolPtr(true),
			},
			request:       createHTTPSRequest("GET", "/test"),
			expectedHSTS:  "max-age=31536000; includeSubDomains; preload",
			expectHSTSSet: true,
			description:   "HSTS with preload directive when enabled",
		},
		{
			name: "hsts_with_all_custom_options",
			config: &SecurityHeadersConfig{
				HStsEnabled:    boolPtr(true),
				HStsMaxAge:     15552000, // 6 months
				HStsSubdomains: boolPtr(false),
				HStsPreload:    boolPtr(true),
			},
			request:       createHTTPSRequest("GET", "/test"),
			expectedHSTS:  "max-age=15552000; preload",
			expectHSTSSet: true,
			description:   "All HSTS options should work together",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewUnifiedSecurityMiddleware(tt.config)
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(nextHandler)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, tt.request)

			if tt.expectHSTSSet {
				assertSecurityHeader(t, w, "Strict-Transport-Security", tt.expectedHSTS)
			} else {
				assertNoHeader(t, w, "Strict-Transport-Security")
			}
		})
	}
}

func TestNewUnifiedSecurityMiddleware_CustomBasicHeaders(t *testing.T) {
	tests := []struct {
		name          string
		config        *SecurityHeadersConfig
		headerName    string
		expectedValue string
	}{
		{
			name: "custom_x_frame_options",
			config: &SecurityHeadersConfig{
				XFrameOptions: "SAMEORIGIN",
			},
			headerName:    "X-Frame-Options",
			expectedValue: "SAMEORIGIN",
		},
		{
			name: "custom_x_content_type_options",
			config: &SecurityHeadersConfig{
				XContentType: "nosniff",
			},
			headerName:    "X-Content-Type-Options",
			expectedValue: "nosniff",
		},
		{
			name: "custom_x_xss_protection_disabled",
			config: &SecurityHeadersConfig{
				XXSSProtection: "0",
			},
			headerName:    "X-XSS-Protection",
			expectedValue: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewUnifiedSecurityMiddleware(tt.config)
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(nextHandler)
			w := httptest.NewRecorder()
			req := createHTTPSRequest("GET", "/test")

			handler.ServeHTTP(w, req)

			assertSecurityHeader(t, w, tt.headerName, tt.expectedValue)
		})
	}
}

func TestNewUnifiedSecurityMiddleware_BackwardCompatibility(t *testing.T) {
	t.Run("new_security_middleware_wrapper_maintains_compatibility", func(t *testing.T) {
		// Test that the existing NewSecurityMiddleware() function still works
		// and produces identical results to the new unified implementation with default config
		oldMiddleware := NewSecurityMiddleware()
		newMiddleware := NewUnifiedSecurityMiddleware(nil) // default config

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Test with HTTPS request
		httpsReq := createHTTPSRequest("GET", "/test")

		// Test old middleware
		oldHandler := oldMiddleware(nextHandler)
		oldRecorder := httptest.NewRecorder()
		oldHandler.ServeHTTP(oldRecorder, httpsReq)

		// Test new middleware
		newHandler := newMiddleware(nextHandler)
		newRecorder := httptest.NewRecorder()
		newHandler.ServeHTTP(newRecorder, httpsReq)

		// Headers should be identical
		expectedHeaders := []string{
			"X-Content-Type-Options",
			"X-Frame-Options",
			"X-XSS-Protection",
			"Referrer-Policy",
			"Content-Security-Policy",
			"Strict-Transport-Security",
		}

		for _, header := range expectedHeaders {
			oldValue := oldRecorder.Header().Get(header)
			newValue := newRecorder.Header().Get(header)
			assert.Equal(t, oldValue, newValue, "Header %s should be identical between old and new middleware", header)
		}
	})

	t.Run("new_security_middleware_wrapper_with_http_request", func(t *testing.T) {
		// Test backward compatibility with HTTP requests (no HSTS)
		oldMiddleware := NewSecurityMiddleware()
		newMiddleware := NewUnifiedSecurityMiddleware(nil)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Test with HTTP request
		httpReq := createHTTPRequest("GET", "/test")

		// Test old middleware
		oldHandler := oldMiddleware(nextHandler)
		oldRecorder := httptest.NewRecorder()
		oldHandler.ServeHTTP(oldRecorder, httpReq)

		// Test new middleware
		newHandler := newMiddleware(nextHandler)
		newRecorder := httptest.NewRecorder()
		newHandler.ServeHTTP(newRecorder, httpReq)

		// All headers except HSTS should be set and identical
		nonHSTSHeaders := []string{
			"X-Content-Type-Options",
			"X-Frame-Options",
			"X-XSS-Protection",
			"Referrer-Policy",
			"Content-Security-Policy",
		}

		for _, header := range nonHSTSHeaders {
			oldValue := oldRecorder.Header().Get(header)
			newValue := newRecorder.Header().Get(header)
			assert.Equal(t, oldValue, newValue, "Header %s should be identical between old and new middleware", header)
		}

		// HSTS should not be set for either
		assertNoHeader(t, oldRecorder, "Strict-Transport-Security")
		assertNoHeader(t, newRecorder, "Strict-Transport-Security")
	})
}

func TestNewUnifiedSecurityMiddleware_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		config      *SecurityHeadersConfig
		description string
	}{
		{
			name:        "nil_config_uses_defaults",
			config:      nil,
			description: "Nil configuration should use default values",
		},
		{
			name:        "empty_config_uses_defaults",
			config:      &SecurityHeadersConfig{},
			description: "Empty configuration should use default values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewUnifiedSecurityMiddleware(tt.config)
			require.NotNil(t, middleware, "Middleware should be created even with nil/empty config")

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(nextHandler)
			w := httptest.NewRecorder()
			req := createHTTPSRequest("GET", "/test")

			// Should not panic and should set default headers
			require.NotPanics(t, func() {
				handler.ServeHTTP(w, req)
			})

			// Should set all default security headers
			assertSecurityHeader(t, w, "X-Content-Type-Options", "nosniff")
			assertSecurityHeader(t, w, "X-Frame-Options", "DENY")
			assertSecurityHeader(t, w, "X-XSS-Protection", "1; mode=block")
			assertSecurityHeader(t, w, "Referrer-Policy", "strict-origin-when-cross-origin")
			assertSecurityHeader(t, w, "Content-Security-Policy", "default-src 'self'")
			assertSecurityHeader(t, w, "Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		})
	}
}

func TestNewUnifiedSecurityMiddleware_TLSContextVariations(t *testing.T) {
	tests := []struct {
		name        string
		setupTLS    func(*http.Request)
		expectHSTS  bool
		description string
	}{
		{
			name: "nil_tls_context_no_hsts",
			setupTLS: func(req *http.Request) {
				req.TLS = nil
			},
			expectHSTS:  false,
			description: "Request with nil TLS context should not set HSTS",
		},
		{
			name: "empty_tls_context_sets_hsts",
			setupTLS: func(req *http.Request) {
				req.TLS = &tls.ConnectionState{}
			},
			expectHSTS:  true,
			description: "Request with empty but non-nil TLS context should set HSTS",
		},
		{
			name: "full_tls_context_sets_hsts",
			setupTLS: func(req *http.Request) {
				req.TLS = &tls.ConnectionState{
					Version:           tls.VersionTLS13,
					HandshakeComplete: true,
					ServerName:        "example.com",
				}
			},
			expectHSTS:  true,
			description: "Request with full TLS context should set HSTS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &SecurityHeadersConfig{
				HStsEnabled: boolPtr(true),
			}

			middleware := NewUnifiedSecurityMiddleware(config)
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(nextHandler)
			w := httptest.NewRecorder()
			req := testutil.CreateRequest("GET", "/test")

			// Setup TLS context as specified
			tt.setupTLS(req)

			handler.ServeHTTP(w, req)

			if tt.expectHSTS {
				assertSecurityHeader(t, w, "Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			} else {
				assertNoHeader(t, w, "Strict-Transport-Security")
			}
		})
	}
}

func TestNewUnifiedSecurityMiddleware_HeadersNotOverwritten(t *testing.T) {
	t.Run("middleware_does_not_overwrite_existing_headers", func(t *testing.T) {
		config := &SecurityHeadersConfig{}
		middleware := NewUnifiedSecurityMiddleware(config)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handler sets a security header before middleware
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			w.WriteHeader(http.StatusOK)
		})

		handler := middleware(nextHandler)
		w := httptest.NewRecorder()
		req := createHTTPSRequest("GET", "/test")

		handler.ServeHTTP(w, req)

		// The middleware should not overwrite the handler's header
		// This test may fail depending on implementation - middleware might run before handler
		// This documents expected behavior that needs to be decided
		assertSecurityHeader(t, w, "X-Frame-Options", "SAMEORIGIN")
	})
}

func TestNewUnifiedSecurityMiddleware_ComprehensiveConfiguration(t *testing.T) {
	t.Run("all_configuration_options_applied_correctly", func(t *testing.T) {
		config := &SecurityHeadersConfig{
			ReferrerPolicy: "no-referrer",
			CSPPolicy:      "comprehensive",
			HStsEnabled:    boolPtr(true),
			HStsMaxAge:     7776000,
			HStsSubdomains: boolPtr(false),
			HStsPreload:    boolPtr(true),
			XFrameOptions:  "SAMEORIGIN",
			XContentType:   "nosniff",
			XXSSProtection: "0",
		}

		middleware := NewUnifiedSecurityMiddleware(config)
		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := middleware(nextHandler)
		w := httptest.NewRecorder()
		req := createHTTPSRequest("GET", "/test")

		handler.ServeHTTP(w, req)

		// Verify all configured headers
		assertSecurityHeader(t, w, "Referrer-Policy", "no-referrer")
		assertSecurityHeader(
			t,
			w,
			"Content-Security-Policy",
			"default-src 'self'; script-src 'self'; object-src 'none'",
		)
		assertSecurityHeader(t, w, "Strict-Transport-Security", "max-age=7776000; preload")
		assertSecurityHeader(t, w, "X-Frame-Options", "SAMEORIGIN")
		assertSecurityHeader(t, w, "X-Content-Type-Options", "nosniff")
		assertSecurityHeader(t, w, "X-XSS-Protection", "0")
	})
}
