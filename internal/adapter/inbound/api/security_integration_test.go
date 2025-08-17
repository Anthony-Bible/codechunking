package api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codechunking/internal/adapter/inbound/api/testutil"
	"codechunking/internal/application/dto"
	"codechunking/internal/port/inbound"

	"github.com/stretchr/testify/assert"
)

// TestRepositoryHandler_SecurityIntegration tests end-to-end security validation.
func TestRepositoryHandler_SecurityIntegration(t *testing.T) {
	// These tests should FAIL initially - security validation integration is not implemented
	tests := []struct {
		name           string
		method         string
		path           string
		body           interface{}
		queryParams    map[string]string
		expectedStatus int
		expectedError  string
		shouldPassAuth bool
	}{
		// XSS prevention integration tests
		{
			name:   "POST_repositories_rejects_XSS_in_name",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "https://github.com/user/repo",
				Name: "<script>alert('xss')</script>",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malicious content detected",
			shouldPassAuth: false,
		},
		{
			name:   "POST_repositories_rejects_XSS_in_description",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:         "https://github.com/user/repo",
				Name:        "normal-repo",
				Description: func(s string) *string { return &s }("<img src='x' onerror='alert(1)'>"),
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malicious content detected",
			shouldPassAuth: false,
		},
		// SQL injection prevention integration tests
		{
			name:   "GET_repositories_rejects_SQL_injection_in_sort",
			method: http.MethodGet,
			path:   "/repositories",
			queryParams: map[string]string{
				"sort": "created_at:asc; DROP TABLE repositories;--",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malicious SQL detected",
			shouldPassAuth: false,
		},
		{
			name:   "GET_repositories_rejects_SQL_injection_in_status",
			method: http.MethodGet,
			path:   "/repositories",
			queryParams: map[string]string{
				"status": "pending' UNION SELECT password FROM users;--",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malicious SQL detected",
			shouldPassAuth: false,
		},
		{
			name:   "GET_repositories_rejects_SQL_injection_in_limit",
			method: http.MethodGet,
			path:   "/repositories",
			queryParams: map[string]string{
				"limit": "10; DELETE FROM repositories;--",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malicious SQL detected",
			shouldPassAuth: false,
		},
		// URL validation integration tests
		{
			name:   "POST_repositories_rejects_javascript_protocol",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "javascript:alert('xss')",
				Name: "normal-repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malicious protocol",
			shouldPassAuth: false,
		},
		{
			name:   "POST_repositories_rejects_file_protocol",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "file:///etc/passwd",
				Name: "normal-repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malicious protocol",
			shouldPassAuth: false,
		},
		{
			name:   "POST_repositories_rejects_path_traversal_in_URL",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "https://github.com/../../../etc/passwd",
				Name: "normal-repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "path traversal detected",
			shouldPassAuth: false,
		},
		// Control character injection tests
		{
			name:   "POST_repositories_rejects_null_byte_injection",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "https://github.com/user/repo",
				Name: "repo\x00malicious",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "control characters detected",
			shouldPassAuth: false,
		},
		{
			name:   "POST_repositories_rejects_newline_injection",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "https://github.com/user/repo",
				Name: "repo\nmalicious",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "control characters detected",
			shouldPassAuth: false,
		},
		// Unicode attack prevention tests
		{
			name:   "POST_repositories_rejects_unicode_override_attack",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "https://github.com/user/repo",
				Name: "repo\u202Emalicious",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "suspicious unicode detected",
			shouldPassAuth: false,
		},
		{
			name:   "POST_repositories_rejects_homograph_attack",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "https://github.com/user/reроsitory", // Cyrillic characters
				Name: "normal-repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "suspicious unicode detected",
			shouldPassAuth: false,
		},
		// Payload size attacks
		{
			name:   "POST_repositories_rejects_oversized_payload",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:         "https://github.com/user/repo",
				Name:        strings.Repeat("a", 100000), // Very large name
				Description: func(s string) *string { return &s }("Normal description"),
			},
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectedError:  "payload too large",
			shouldPassAuth: false,
		},
		// Valid requests that should pass security validation
		{
			name:   "POST_repositories_allows_valid_request",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:         "https://github.com/user/repo",
				Name:        "my-awesome-repo",
				Description: func(s string) *string { return &s }("This is a valid repository description"),
			},
			expectedStatus: http.StatusAccepted,
			shouldPassAuth: true,
		},
		{
			name:   "GET_repositories_allows_valid_query_params",
			method: http.MethodGet,
			path:   "/repositories",
			queryParams: map[string]string{
				"status": "pending",
				"limit":  "10",
				"offset": "0",
				"sort":   "created_at:asc",
			},
			expectedStatus: http.StatusOK,
			shouldPassAuth: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create handler with security middleware
			handler := createSecureRepositoryHandler(nil)

			// Create request
			var req *http.Request
			if tt.body != nil {
				bodyBytes, _ := json.Marshal(tt.body)
				req = testutil.CreateRequestWithBody(tt.method, tt.path, bytes.NewReader(bodyBytes))
			} else {
				req = testutil.CreateRequest(tt.method, tt.path)
			}

			// Add query parameters
			if tt.queryParams != nil {
				q := req.URL.Query()
				for key, value := range tt.queryParams {
					q.Add(key, value)
				}
				req.URL.RawQuery = q.Encode()
			}

			recorder := httptest.NewRecorder()

			// Execute
			handler.ServeHTTP(recorder, req)

			// Assert status code
			assert.Equal(t, tt.expectedStatus, recorder.Code,
				"Expected status code mismatch for test: %s", tt.name)

			// Assert error message for security violations
			if tt.expectedError != "" {
				responseBody := recorder.Body.String()
				assert.Contains(t, responseBody, tt.expectedError,
					"Expected error message not found in response for test: %s", tt.name)
			}

			// For valid requests that should pass, expect success status codes
			if tt.shouldPassAuth {
				assert.True(t, recorder.Code == http.StatusAccepted || recorder.Code == http.StatusOK,
					"Valid request should return success status code")
			}
		})
	}
}

// TestRepositoryHandler_ContentTypeValidation tests content type validation integration.
func TestRepositoryHandler_ContentTypeValidation(t *testing.T) {
	// These tests should FAIL initially - content type validation not integrated
	tests := []struct {
		name           string
		contentType    string
		body           string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "rejects_text_plain_content_type",
			contentType:    "text/plain",
			body:           "this is not json",
			expectedStatus: http.StatusUnsupportedMediaType,
			expectedError:  "unsupported content type",
		},
		{
			name:           "rejects_xml_content_type",
			contentType:    "application/xml",
			body:           "<xml>data</xml>",
			expectedStatus: http.StatusUnsupportedMediaType,
			expectedError:  "unsupported content type",
		},
		{
			name:           "allows_json_content_type",
			contentType:    "application/json",
			body:           `{"url": "https://github.com/user/repo", "name": "test-repo"}`,
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "allows_json_with_charset",
			contentType:    "application/json; charset=utf-8",
			body:           `{"url": "https://github.com/user/repo", "name": "test-repo"}`,
			expectedStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create handler with security middleware
			handler := createSecureRepositoryHandler(nil)

			req := testutil.CreateRequestWithBody(http.MethodPost, "/repositories", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.expectedError != "" {
				assert.Contains(t, recorder.Body.String(), tt.expectedError)
			}
		})
	}
}

// TestRepositoryHandler_RateLimiting tests rate limiting integration.
func TestRepositoryHandler_RateLimiting(t *testing.T) {
	// Test rate limiting integration
	t.Run("enforces_rate_limits", func(t *testing.T) {
		// Create handler with security middleware
		handler := createSecureRepositoryHandler(nil)

		var successCount, blockedCount int
		clientIP := "192.168.1.100:12345"

		// Make multiple requests from same IP
		for range 20 {
			req := testutil.CreateRequest(http.MethodGet, "/repositories")
			req.RemoteAddr = clientIP
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			switch recorder.Code {
			case http.StatusOK:
				successCount++
			case http.StatusTooManyRequests:
				blockedCount++
			}
		}

		// Should have blocked some requests due to rate limiting
		assert.Positive(t, blockedCount, "Rate limiting should block excessive requests")
		assert.Less(t, successCount, 20, "Not all requests should succeed")
	})
}

// TestRepositoryHandler_SecurityHeaders tests security headers integration.
func TestRepositoryHandler_SecurityHeaders(t *testing.T) {
	// Test security headers integration
	t.Run("adds_security_headers", func(t *testing.T) {
		// Create handler with security middleware
		handler := createSecureRepositoryHandler(nil)

		req := testutil.CreateRequest(http.MethodGet, "/repositories")
		// Set up TLS context for HSTS to work
		req.TLS = &tls.ConnectionState{
			Version: tls.VersionTLS12,
		}
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		// Should add comprehensive security headers
		headers := recorder.Header()
		assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", headers.Get("X-Frame-Options"))
		assert.Equal(t, "1; mode=block", headers.Get("X-XSS-Protection"))
		assert.Equal(t, "strict-origin-when-cross-origin", headers.Get("Referrer-Policy"))
		assert.Contains(t, headers.Get("Content-Security-Policy"), "default-src 'self'")
		assert.Contains(t, headers.Get("Strict-Transport-Security"), "max-age=")
	})
}

// TestRepositoryHandler_ComprehensiveSecurity tests multiple security layers together.
func TestRepositoryHandler_ComprehensiveSecurity(t *testing.T) {
	// Test comprehensive security stack
	t.Run("applies_all_security_measures", func(t *testing.T) {
		// Create handler with full security middleware stack
		handler := createFullSecurityRepositoryHandler(nil)

		// Test multiple attack vectors in one request
		maliciousPayload := dto.CreateRepositoryRequest{
			URL:         "javascript:alert('xss')",                                             // Malicious protocol
			Name:        "<script>alert('xss')</script>",                                       // XSS attempt
			Description: func(s string) *string { return &s }("'; DROP TABLE repositories;--"), // SQL injection attempt
		}

		bodyBytes, _ := json.Marshal(maliciousPayload)
		req := testutil.CreateRequestWithBody(http.MethodPost, "/repositories", bytes.NewReader(bodyBytes))

		// Add malicious headers
		req.Header.Set("X-Custom-Header", "value\r\nX-Injected-Header: malicious")
		req.Header.Set("Content-Type", "application/json")

		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		// Should be blocked by security validation
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "multiple security violations")

		// Should have security headers even for blocked requests
		headers := recorder.Header()
		assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", headers.Get("X-Frame-Options"))
	})
}

// Helper functions for creating secure handlers

func createSecureRepositoryHandler(service inbound.RepositoryService) http.Handler {
	// Create handler with mock service
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simple mock implementation
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repositories":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"message": "success"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/repositories":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"repositories": []}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	// Apply security middleware
	return CreateSecurityMiddlewareStack()(handler)
}

func createFullSecurityRepositoryHandler(service inbound.RepositoryService) http.Handler {
	return createSecureRepositoryHandler(service) // Same as above for now
}

// TestSecurityMiddlewareStack tests the complete security middleware stack.
func TestSecurityMiddlewareStack(t *testing.T) {
	// This test should FAIL initially - security middleware stack not implemented
	tests := []struct {
		name                  string
		request               *http.Request
		expectedStatus        int
		expectedSecurityError string
		shouldBlockRequest    bool
	}{
		{
			name:                  "blocks_multiple_attack_vectors",
			request:               createMaliciousRequest(),
			expectedStatus:        http.StatusBadRequest,
			expectedSecurityError: "multiple security violations",
			shouldBlockRequest:    true,
		},
		{
			name:               "allows_clean_requests",
			request:            createCleanRequest(),
			expectedStatus:     http.StatusOK,
			shouldBlockRequest: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should FAIL - comprehensive security stack not implemented
			securityStack := CreateSecurityMiddlewareStack()

			handler := securityStack(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte(`{"message": "success"}`)); err != nil {
					t.Logf("Failed to write response: %v", err)
				}
			}))

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, tt.request)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.shouldBlockRequest {
				assert.Contains(t, recorder.Body.String(), tt.expectedSecurityError)
			}
		})
	}
}

// Helper functions for creating test requests

func createMaliciousRequest() *http.Request {
	payload := `{
		"url": "javascript:alert('xss')",
		"name": "<script>alert('xss')</script>",
		"description": "'; DROP TABLE repositories;--"
	}`

	req := testutil.CreateRequestWithBody(http.MethodPost, "/repositories", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom-Header", "value\r\nX-Injected-Header: malicious")
	return req
}

func createCleanRequest() *http.Request {
	payload := `{
		"url": "https://github.com/user/repo",
		"name": "my-awesome-repo",
		"description": "A legitimate repository"
	}`

	req := testutil.CreateRequestWithBody(http.MethodPost, "/repositories", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// CreateSecurityMiddlewareStack is now implemented in security_middleware.go
