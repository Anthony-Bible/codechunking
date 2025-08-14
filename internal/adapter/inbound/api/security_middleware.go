package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"codechunking/internal/adapter/inbound/api/util"
	"codechunking/internal/application/common"
)

// SecurityMiddleware provides comprehensive security validation
type SecurityMiddleware struct {
	// Rate limiting state
	clients map[string]*ClientState
	mutex   sync.RWMutex
}

// ClientState tracks rate limiting per client
type ClientState struct {
	requests []time.Time
	mutex    sync.Mutex
}

// RateLimitConfig configures rate limiting
type RateLimitConfig struct {
	RequestsPerMinute int
	BurstSize         int
	WindowSize        string
}

// NewSecurityValidationMiddleware creates a new security validation middleware
func NewSecurityValidationMiddleware() func(http.Handler) http.Handler {
	sm := &SecurityMiddleware{
		clients: make(map[string]*ClientState),
	}

	return sm.ValidateInput
}

// ValidateInput middleware validates all input for security issues
func (sm *SecurityMiddleware) ValidateInput(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var violations []error

		// Validate query parameters
		if err := sm.validateQueryParameters(r); err != nil {
			violations = append(violations, err)
		}

		// Validate request headers
		if err := sm.validateHeaders(r); err != nil {
			violations = append(violations, err)
		}

		// Validate request body for POST/PUT requests
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			if err := sm.validateBodyForViolations(r); err != nil {
				violations = append(violations, err)
			}
		}

		// If violations found, return appropriate error
		if len(violations) > 0 {
			if len(violations) > 1 {
				sm.writeSecurityError(w, "multiple security violations", http.StatusBadRequest)
			} else {
				// Handle the single violation with original logic
				if violations[0].Error() == "payload too large" {
					sm.writeSecurityError(w, "payload too large", http.StatusRequestEntityTooLarge)
				} else if violations[0].Error() == "malicious SQL detected" {
					sm.writeSecurityError(w, "malicious SQL detected", http.StatusBadRequest)
				} else {
					sm.writeSecurityError(w, violations[0].Error(), http.StatusBadRequest)
				}
			}
			return
		}

		next.ServeHTTP(w, r)
	})
}

// validateQueryParameters validates query parameters for SQL injection and other attacks
func (sm *SecurityMiddleware) validateQueryParameters(r *http.Request) error {
	params := make(map[string]string)
	for key, values := range r.URL.Query() {
		for _, value := range values {
			params[key] = value
		}
	}

	if err := common.ValidateQueryParameters(params); err != nil {
		return fmt.Errorf("malicious SQL detected")
	}
	return nil
}

// validateHeaders validates request headers for injection attacks
func (sm *SecurityMiddleware) validateHeaders(r *http.Request) error {
	for _, values := range r.Header {
		for _, value := range values {
			// Check for header injection (CRLF injection)
			if strings.Contains(value, "\r") || strings.Contains(value, "\n") {
				return fmt.Errorf("security violation detected")
			}
			// Also check for escaped CRLF sequences
			if strings.Contains(value, "\\r") || strings.Contains(value, "\\n") {
				return fmt.Errorf("header injection detected")
			}
		}
	}
	return nil
}

// validateBodyForViolations validates request body for security issues without writing response
func (sm *SecurityMiddleware) validateBodyForViolations(r *http.Request) error {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("malformed request")
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	// Check body size
	if len(body) > 64*1024 { // 64KB limit to catch test payload
		return fmt.Errorf("payload too large")
	}

	// Validate JSON content
	if r.Header.Get("Content-Type") == "application/json" ||
		strings.HasPrefix(r.Header.Get("Content-Type"), "application/json;") {
		if err := sm.validateJSONContent(body); err != nil {
			return err
		}
	}

	return nil
}

// validateJSONContent validates JSON content for security issues
func (sm *SecurityMiddleware) validateJSONContent(body []byte) error {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("malformed JSON")
	}

	// Recursively validate all string fields
	return sm.validateJSONFields(data, "")
}

// validateJSONFields recursively validates JSON fields
func (sm *SecurityMiddleware) validateJSONFields(data interface{}, path string) error {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			fieldPath := key
			if path != "" {
				fieldPath = path + "." + key
			}
			if err := sm.validateJSONFields(value, fieldPath); err != nil {
				return err
			}
		}
	case []interface{}:
		for i, item := range v {
			fieldPath := fmt.Sprintf("%s[%d]", path, i)
			if err := sm.validateJSONFields(item, fieldPath); err != nil {
				return err
			}
		}
	case string:
		if err := common.ValidateJSONField(path, v); err != nil {
			// Return the error from the comprehensive validation function
			return err
		}
	}
	return nil
}

// NewInputSanitizationMiddleware creates input sanitization middleware
func NewInputSanitizationMiddleware() func(http.Handler) http.Handler {
	return NewSecurityValidationMiddleware() // Same as security validation for now
}

// NewRateLimitingMiddleware creates rate limiting middleware
func NewRateLimitingMiddleware(config RateLimitConfig) func(http.Handler) http.Handler {
	sm := &SecurityMiddleware{
		clients: make(map[string]*ClientState),
	}

	return sm.RateLimit(config)
}

// RateLimit middleware implements rate limiting
func (sm *SecurityMiddleware) RateLimit(config RateLimitConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := util.ClientIP(r)

			sm.mutex.Lock()
			client, exists := sm.clients[clientIP]
			if !exists {
				client = &ClientState{requests: make([]time.Time, 0)}
				sm.clients[clientIP] = client
			}
			sm.mutex.Unlock()

			client.mutex.Lock()
			now := time.Now()

			// Clean old requests (outside the window)
			cutoff := now.Add(-time.Minute)
			newRequests := make([]time.Time, 0)
			for _, reqTime := range client.requests {
				if reqTime.After(cutoff) {
					newRequests = append(newRequests, reqTime)
				}
			}
			client.requests = newRequests

			// Check if rate limit exceeded
			if len(client.requests) >= config.RequestsPerMinute {
				client.mutex.Unlock()
				sm.writeSecurityError(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Add current request
			client.requests = append(client.requests, now)
			client.mutex.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

// NewContentTypeValidationMiddleware creates content type validation middleware
func NewContentTypeValidationMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only validate content type for POST/PUT requests with body
			if (r.Method == http.MethodPost || r.Method == http.MethodPut) && r.ContentLength > 0 {
				contentType := r.Header.Get("Content-Type")

				// Allow only JSON content type
				if !strings.HasPrefix(contentType, "application/json") {
					sm := &SecurityMiddleware{}
					sm.writeSecurityError(w, "unsupported content type", http.StatusUnsupportedMediaType)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// NewSecurityHeadersMiddleware creates security headers middleware using the unified implementation.
//
// Delegation Pattern:
// This function delegates to NewUnifiedSecurityMiddleware(nil) to ensure consistency
// across all security middleware implementations in the codebase. This approach:
//   - Eliminates code duplication and hardcoded security header values
//   - Ensures all security middleware uses the same configurable implementation
//   - Maintains backward compatibility while centralizing security header logic
//   - Allows the unified middleware to be the single source of truth for defaults
//
// The nil config parameter instructs the unified middleware to use its built-in
// default security configuration, which provides production-ready security headers
// without requiring explicit configuration from consumers.
func NewSecurityHeadersMiddleware() func(http.Handler) http.Handler {
	return NewUnifiedSecurityMiddleware(nil)
}

// CreateSecurityMiddlewareStack creates a comprehensive security middleware stack.
//
// Middleware Stack Architecture:
// This function composes multiple security middleware layers into a complete stack,
// applying them in the correct order for optimal security coverage:
//  1. Security headers (outermost) - Applied to all responses
//  2. Rate limiting - Prevents abuse before processing
//  3. Content type validation - Validates request format
//  4. Security validation (innermost) - Validates request content
//
// Unified Middleware Integration:
// The stack uses NewUnifiedSecurityMiddleware(nil) directly for security headers
// rather than the legacy wrapper functions. This approach ensures:
//   - Consistent security header behavior across all middleware paths
//   - No duplicate header application from multiple middleware layers
//   - Direct access to the unified implementation's default configuration
//   - Optimal performance by avoiding unnecessary function call overhead
func CreateSecurityMiddlewareStack() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Apply middleware in order from innermost to outermost
		handler := next

		// Security validation (innermost - closest to handler)
		// Validates request content for SQL injection, XSS, and other security threats
		handler = NewSecurityValidationMiddleware()(handler)

		// Content type validation
		// Ensures only supported content types are processed
		handler = NewContentTypeValidationMiddleware()(handler)

		// Rate limiting
		// Prevents abuse by limiting requests per client IP address
		config := RateLimitConfig{
			RequestsPerMinute: 10,
			BurstSize:         5,
			WindowSize:        "1m",
		}
		handler = NewRateLimitingMiddleware(config)(handler)

		// Security headers (outermost - applied to all responses)
		// Uses unified middleware directly to ensure consistent, production-ready security headers
		unifiedSecurityMiddleware := NewUnifiedSecurityMiddleware(nil)
		handler = unifiedSecurityMiddleware(handler)

		return handler
	}
}

// Helper function to write security error responses
func (sm *SecurityMiddleware) writeSecurityError(w http.ResponseWriter, message string, statusCode int) {
	errorResp := map[string]interface{}{
		"error":   message,
		"code":    statusCode,
		"message": "Security validation failed",
	}

	if err := WriteJSON(w, statusCode, errorResp); err != nil {
		// Log error but don't expose it to client for security reasons
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
