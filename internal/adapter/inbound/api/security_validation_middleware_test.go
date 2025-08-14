package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codechunking/internal/adapter/inbound/api/testutil"
	"codechunking/internal/application/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecurityValidationMiddleware tests comprehensive input validation middleware.
func TestSecurityValidationMiddleware(t *testing.T) {
	// These tests should FAIL initially - security validation middleware is not implemented
	tests := []struct {
		name           string
		method         string
		path           string
		body           interface{}
		headers        map[string]string
		queryParams    map[string]string
		expectedStatus int
		expectedError  string
	}{
		// XSS attack prevention tests
		{
			name:   "rejects_XSS_in_repository_creation",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:         "https://github.com/user/repo",
				Name:        "<script>alert('xss')</script>",
				Description: stringPtr("Normal description"),
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malicious content detected",
		},
		{
			name:   "rejects_XSS_in_description_field",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:         "https://github.com/user/repo",
				Name:        "normal-repo",
				Description: stringPtr("<img src='x' onerror='alert(1)'>"),
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malicious content detected",
		},
		{
			name:   "rejects_javascript_protocol_in_URL",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "javascript:alert('xss')",
				Name: "normal-repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malicious protocol",
		},
		// SQL injection prevention tests
		{
			name:   "rejects_SQL_injection_in_sort_parameter",
			method: http.MethodGet,
			path:   "/repositories",
			queryParams: map[string]string{
				"sort": "created_at:asc; DROP TABLE repositories;--",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malicious SQL detected",
		},
		{
			name:   "rejects_SQL_injection_in_status_filter",
			method: http.MethodGet,
			path:   "/repositories",
			queryParams: map[string]string{
				"status": "pending' UNION SELECT password FROM users;--",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malicious SQL detected",
		},
		{
			name:   "rejects_SQL_injection_in_limit_parameter",
			method: http.MethodGet,
			path:   "/repositories",
			queryParams: map[string]string{
				"limit": "10; DELETE FROM repositories;--",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malicious SQL detected",
		},
		// Control character injection tests
		{
			name:   "rejects_null_byte_injection",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "https://github.com/user/repo",
				Name: "repo\x00malicious",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "control characters detected",
		},
		{
			name:   "rejects_newline_injection",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "https://github.com/user/repo",
				Name: "repo\nmalicious",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "control characters detected",
		},
		// Unicode attack prevention tests
		{
			name:   "rejects_right_to_left_override_attack",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "https://github.com/user/repo",
				Name: "repo\u202Emalicious",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "suspicious unicode detected",
		},
		{
			name:   "rejects_homograph_attack",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "https://github.com/user/reроsitory", // Contains Cyrillic characters
				Name: "normal-repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "suspicious unicode detected",
		},
		// Header injection tests
		{
			name:   "rejects_header_injection_attempt",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "https://github.com/user/repo",
				Name: "normal-repo",
			},
			headers: map[string]string{
				"X-Custom-Header": "value\\r\\nX-Injected-Header: malicious",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "header injection detected",
		},
		// Path traversal prevention tests
		{
			name:   "rejects_path_traversal_in_URL",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "https://github.com/../../../etc/passwd",
				Name: "normal-repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "path traversal detected",
		},
		{
			name:   "rejects_encoded_path_traversal",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:  "https://github.com/%2E%2E/%2E%2E/etc/passwd",
				Name: "normal-repo",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "path traversal detected",
		},
		// Large payload DOS prevention tests
		{
			name:   "rejects_extremely_large_payload",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:         "https://github.com/user/repo",
				Name:        strings.Repeat("a", 100000), // Extremely large name
				Description: stringPtr("Normal description"),
			},
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectedError:  "payload too large",
		},
		// Valid requests that should pass
		{
			name:   "allows_valid_repository_creation",
			method: http.MethodPost,
			path:   "/repositories",
			body: dto.CreateRepositoryRequest{
				URL:         "https://github.com/user/repo",
				Name:        "my-awesome-repo",
				Description: stringPtr("This is a normal repository description"),
			},
			expectedStatus: http.StatusAccepted,
		},
		{
			name:   "allows_valid_query_parameters",
			method: http.MethodGet,
			path:   "/repositories",
			queryParams: map[string]string{
				"status": "pending",
				"limit":  "10",
				"offset": "0",
				"sort":   "created_at:asc",
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This middleware doesn't exist yet - tests should FAIL initially
			securityMiddleware := NewSecurityValidationMiddleware()

			// Mock handler that should only be called for valid requests
			handler := securityMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.expectedStatus)
				_, _ = w.Write([]byte(`{"message": "success"}`))
			}))

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

			// Add headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			recorder := httptest.NewRecorder()

			// Execute
			handler.ServeHTTP(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code, "Expected status code mismatch")

			if tt.expectedError != "" {
				var errorResp map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &errorResp)
				require.NoError(t, err, "Should be able to parse error response")

				errorMsg, ok := errorResp["error"].(string)
				require.True(t, ok && errorMsg != "", "Error message should be present and non-empty")
				assert.Contains(t, errorMsg, tt.expectedError, "Error message should indicate security issue")
			}
		})
	}
}

// TestInputSanitizationMiddleware tests input sanitization for all endpoints.
func TestInputSanitizationMiddleware(t *testing.T) {
	// These tests should FAIL initially - input sanitization middleware is not implemented
	tests := []struct {
		name           string
		endpoint       string
		payload        string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "sanitizes_HTML_tags_in_JSON",
			endpoint:       "/repositories",
			payload:        `{"url": "https://github.com/user/repo", "name": "<script>alert('xss')</script>"}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malicious content",
		},
		{
			name:           "rejects_malformed_JSON_with_injection",
			endpoint:       "/repositories",
			payload:        `{"url": "https://github.com/user/repo", "name": "normal"}; DROP TABLE repositories;--`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malformed JSON",
		},
		{
			name:           "rejects_nested_injection_attempts",
			endpoint:       "/repositories",
			payload:        `{"url": "https://github.com/user/repo", "metadata": {"description": "<img onerror='alert(1)' src='x'>"}}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "malicious content",
		},
		{
			name:           "allows_properly_escaped_content",
			endpoint:       "/repositories",
			payload:        `{"url": "https://github.com/user/repo", "name": "repo-with-special-chars_123"}`,
			expectedStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This middleware doesn't exist yet - tests should FAIL initially
			sanitizationMiddleware := NewInputSanitizationMiddleware()

			handler := sanitizationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.expectedStatus)
				_, _ = w.Write([]byte(`{"message": "success"}`))
			}))

			req := testutil.CreateRequestWithBody(http.MethodPost, tt.endpoint, strings.NewReader(tt.payload))
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.expectedError != "" {
				assert.Contains(t, recorder.Body.String(), tt.expectedError)
			}
		})
	}
}

// TestRateLimitingMiddleware tests rate limiting for DOS prevention.
func TestRateLimitingMiddleware(t *testing.T) {
	// These tests should FAIL initially - rate limiting middleware is not implemented
	tests := []struct {
		name            string
		requestCount    int
		requestInterval string
		expectedBlocked int
		expectedAllowed int
		rateLimitConfig RateLimitConfig
	}{
		{
			name:            "blocks_excessive_requests_from_same_IP",
			requestCount:    20,
			requestInterval: "1s",
			expectedBlocked: 10,
			expectedAllowed: 10,
			rateLimitConfig: RateLimitConfig{
				RequestsPerMinute: 10,
				BurstSize:         5,
				WindowSize:        "1m",
			},
		},
		{
			name:            "allows_requests_within_rate_limit",
			requestCount:    5,
			requestInterval: "1s",
			expectedBlocked: 0,
			expectedAllowed: 5,
			rateLimitConfig: RateLimitConfig{
				RequestsPerMinute: 10,
				BurstSize:         10,
				WindowSize:        "1m",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This middleware doesn't exist yet - tests should FAIL initially
			rateLimitMiddleware := NewRateLimitingMiddleware(tt.rateLimitConfig)

			handler := rateLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"message": "success"}`))
			}))

			var blockedCount, allowedCount int

			for range tt.requestCount {
				req := testutil.CreateRequest(http.MethodGet, "/repositories")
				req.RemoteAddr = "192.168.1.100:12345" // Same IP for all requests
				recorder := httptest.NewRecorder()

				handler.ServeHTTP(recorder, req)

				switch recorder.Code {
				case http.StatusTooManyRequests:
					blockedCount++
				case http.StatusOK:
					allowedCount++
				}
			}

			assert.Equal(t, tt.expectedBlocked, blockedCount, "Blocked request count mismatch")
			assert.Equal(t, tt.expectedAllowed, allowedCount, "Allowed request count mismatch")
		})
	}
}

// TestContentTypeValidationMiddleware tests content type validation.
func TestContentTypeValidationMiddleware(t *testing.T) {
	// These tests should FAIL initially - content type validation middleware is not implemented
	tests := []struct {
		name           string
		method         string
		contentType    string
		body           string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "rejects_non_JSON_content_type_for_POST",
			method:         http.MethodPost,
			contentType:    "text/plain",
			body:           "this is not json",
			expectedStatus: http.StatusUnsupportedMediaType,
			expectedError:  "unsupported content type",
		},
		{
			name:           "rejects_XML_content_type",
			method:         http.MethodPost,
			contentType:    "application/xml",
			body:           "<xml>data</xml>",
			expectedStatus: http.StatusUnsupportedMediaType,
			expectedError:  "unsupported content type",
		},
		{
			name:           "allows_valid_JSON_content_type",
			method:         http.MethodPost,
			contentType:    "application/json",
			body:           `{"url": "https://github.com/user/repo"}`,
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "allows_JSON_with_charset",
			method:         http.MethodPost,
			contentType:    "application/json; charset=utf-8",
			body:           `{"url": "https://github.com/user/repo"}`,
			expectedStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This middleware doesn't exist yet - tests should FAIL initially
			contentTypeMiddleware := NewContentTypeValidationMiddleware()

			handler := contentTypeMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.expectedStatus)
				_, _ = w.Write([]byte(`{"message": "success"}`))
			}))

			req := testutil.CreateRequestWithBody(tt.method, "/repositories", strings.NewReader(tt.body))
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

// TestSecurityHeadersMiddleware tests security headers middleware.
func TestSecurityHeadersMiddleware(t *testing.T) {
	// These tests should FAIL initially - security headers middleware is not implemented
	t.Run("adds_security_headers", func(t *testing.T) {
		securityMiddleware := NewSecurityHeadersMiddleware()

		handler := securityMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
		}))

		req := testutil.CreateRequest(http.MethodGet, "/repositories")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		// Should add security headers
		assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", recorder.Header().Get("X-Frame-Options"))
		assert.Equal(t, "1; mode=block", recorder.Header().Get("X-XSS-Protection"))
		assert.Equal(t, "strict-origin-when-cross-origin", recorder.Header().Get("Referrer-Policy"))
		assert.Contains(t, recorder.Header().Get("Content-Security-Policy"), "default-src 'self'")
	})
}

// All middleware functions are now implemented in security_middleware.go

// stringPtr is a helper function to create string pointers.
func stringPtr(s string) *string {
	return &s
}
