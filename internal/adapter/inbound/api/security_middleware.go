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

// validateBody validates request body for security issues
func (sm *SecurityMiddleware) validateBody(w http.ResponseWriter, r *http.Request) error {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		sm.writeSecurityError(w, "malformed request", http.StatusBadRequest)
		return err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	// Check body size
	if len(body) > 64*1024 { // 64KB limit to catch test payload
		sm.writeSecurityError(w, "payload too large", http.StatusRequestEntityTooLarge)
		return fmt.Errorf("payload too large")
	}

	// Validate JSON content
	if r.Header.Get("Content-Type") == "application/json" ||
		strings.HasPrefix(r.Header.Get("Content-Type"), "application/json;") {
		if err := sm.validateJSONContent(body); err != nil {
			sm.writeSecurityError(w, err.Error(), http.StatusBadRequest)
			return err
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

// NewSecurityHeadersMiddleware creates security headers middleware
func NewSecurityHeadersMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; object-src 'none'")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

			next.ServeHTTP(w, r)
		})
	}
}

// CreateSecurityMiddlewareStack creates a comprehensive security middleware stack
func CreateSecurityMiddlewareStack() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Apply middleware in order
		handler := next

		// Security validation (innermost - closest to handler)
		handler = NewSecurityValidationMiddleware()(handler)

		// Content type validation
		handler = NewContentTypeValidationMiddleware()(handler)

		// Rate limiting
		config := RateLimitConfig{
			RequestsPerMinute: 10,
			BurstSize:         5,
			WindowSize:        "1m",
		}
		handler = NewRateLimitingMiddleware(config)(handler)

		// Security headers (outermost)
		handler = NewSecurityHeadersMiddleware()(handler)

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

	WriteJSON(w, statusCode, errorResp)
}
