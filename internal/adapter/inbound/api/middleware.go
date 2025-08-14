package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"codechunking/internal/adapter/inbound/api/util"
	"github.com/google/uuid"
)

// Logger defines the interface for logging middleware
type Logger interface {
	Infof(template string, args ...interface{})
	Errorf(template string, args ...interface{})
	WithField(key string, value interface{}) Logger
	WithFields(fields map[string]interface{}) Logger
}

// fieldEntry represents a single key-value field for efficient chaining.
// This approach reduces memory allocations by avoiding map copying until log time.
type fieldEntry struct {
	key   string
	value interface{}
}

// DefaultLogger provides a structured logger implementation with optimized field handling.
// It uses a field chain approach to minimize allocations when creating logger contexts
// with WithField/WithFields operations, only materializing the final field map when
// actually logging a message.
type DefaultLogger struct {
	output     io.Writer              // Where to write log output
	fields     map[string]interface{} // Base fields from initialization (may be nil)
	fieldChain []fieldEntry           // Chained field additions to avoid premature map copying
}

// NewDefaultLogger creates a new default logger that writes to stdout
func NewDefaultLogger() Logger {
	return &DefaultLogger{
		output: os.Stdout,
		fields: nil, // Start with no fields to avoid initial allocation
	}
}

// WithField adds a field to the logger context using efficient field chaining.
// This method avoids copying the entire fields map, instead adding to a field chain
// that is only materialized when actually logging. This significantly reduces
// allocations in hot paths where many logger contexts are created but not all are used.
func (l *DefaultLogger) WithField(key string, value interface{}) Logger {
	// Create a new logger that shares the base fields and adds to the chain
	newChain := make([]fieldEntry, len(l.fieldChain)+1)
	copy(newChain, l.fieldChain)
	newChain[len(l.fieldChain)] = fieldEntry{key: key, value: value}

	return &DefaultLogger{
		output:     l.output,
		fields:     l.fields, // Share the base fields map to avoid copying
		fieldChain: newChain,
	}
}

// WithFields adds multiple fields to the logger context using efficient field chaining
func (l *DefaultLogger) WithFields(fields map[string]interface{}) Logger {
	// Create a new logger that shares the base fields and adds to the chain
	newChain := make([]fieldEntry, len(l.fieldChain)+len(fields))
	copy(newChain, l.fieldChain)

	i := len(l.fieldChain)
	for k, v := range fields {
		newChain[i] = fieldEntry{key: k, value: v}
		i++
	}

	return &DefaultLogger{
		output:     l.output,
		fields:     l.fields, // Share the base fields map
		fieldChain: newChain,
	}
}

// Infof logs an info message with structured fields
func (l *DefaultLogger) Infof(template string, args ...interface{}) {
	l.logWithLevel("INFO", template, args...)
}

// Errorf logs an error message with structured fields
func (l *DefaultLogger) Errorf(template string, args ...interface{}) {
	l.logWithLevel("ERROR", template, args...)
}

// logWithLevel logs a message with the specified level and fields
func (l *DefaultLogger) logWithLevel(level, template string, args ...interface{}) {
	if l.output == nil {
		return
	}

	timestamp := time.Now().Format(time.RFC3339)
	message := fmt.Sprintf(template, args...)

	// Build structured log entry
	logEntry := fmt.Sprintf("[%s] %s: %s", timestamp, level, message)

	// Add fields if any - materialize the field chain at log time
	allFields := l.materializeFields()
	for key, value := range allFields {
		logEntry += fmt.Sprintf(" %s=%v", key, value)
	}

	_, _ = fmt.Fprintln(l.output, logEntry)
}

// materializeFields combines base fields and field chain into a single map for logging
func (l *DefaultLogger) materializeFields() map[string]interface{} {
	if l.fields == nil && len(l.fieldChain) == 0 {
		return nil
	}

	// Calculate total capacity needed
	totalFields := len(l.fieldChain)
	if l.fields != nil {
		totalFields += len(l.fields)
	}

	if totalFields == 0 {
		return nil
	}

	// Allocate map only when actually logging
	result := make(map[string]interface{}, totalFields)

	// Add base fields first
	if l.fields != nil {
		for k, v := range l.fields {
			result[k] = v
		}
	}

	// Add chained fields (may override base fields)
	for _, entry := range l.fieldChain {
		result[entry.key] = entry.value
	}

	return result
}

// NewTestLogger creates a logger for testing that writes to the given writer
func NewTestLogger(output io.Writer) Logger {
	return &DefaultLogger{
		output: output,
		fields: nil, // Start with no fields to avoid initial allocation
	}
}

// requestIDKey is the context key for request IDs
type requestIDKey struct{}

// GetRequestID extracts the request ID from the context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// SetRequestID sets the request ID in the context
func SetRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// LoggingMiddleware wraps handlers with request logging and request ID tracking
func NewLoggingMiddleware(logger Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Generate request ID if not present
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}

			// Add request ID to context
			ctx := SetRequestID(r.Context(), requestID)
			r = r.WithContext(ctx)

			// Add request ID to response headers
			w.Header().Set("X-Request-ID", requestID)

			// Create a response writer that captures the status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Call the next handler
			next.ServeHTTP(wrapped, r)

			// Log the request with structured fields
			duration := time.Since(start)
			requestLogger := logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"method":     r.Method,
				"path":       r.URL.Path,
				"status":     wrapped.statusCode,
				"duration":   duration,
				"user_agent": r.Header.Get("User-Agent"),
				"remote_ip":  util.ClientIP(r),
			})

			if r.URL.RawQuery != "" {
				requestLogger = requestLogger.WithField("query", r.URL.RawQuery)
			}

			requestLogger.Infof("HTTP request completed")
		})
	}
}

// ErrorHandlingMiddleware provides centralized error handling with better logging
func NewErrorHandlingMiddleware() func(http.Handler) http.Handler {
	logger := NewDefaultLogger()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for context cancellation before processing
			if r.Context().Err() != nil {
				requestID := GetRequestID(r.Context())
				w.Header().Set("Content-Type", "application/json")
				if requestID != "" {
					w.Header().Set("X-Request-ID", requestID)
				}
				w.WriteHeader(http.StatusInternalServerError)
				errorResponse := fmt.Sprintf(`{"error": "Request cancelled", "request_id": "%s"}`, requestID)
				_, _ = w.Write([]byte(errorResponse))
				return
			}

			defer func() {
				if err := recover(); err != nil {
					requestID := GetRequestID(r.Context())

					// Log the panic with request context
					panicLogger := logger.WithFields(map[string]interface{}{
						"request_id": requestID,
						"method":     r.Method,
						"path":       r.URL.Path,
						"panic":      err,
					})

					panicLogger.Errorf("Panic recovered in HTTP handler")

					// Set appropriate headers
					w.Header().Set("Content-Type", "application/json")
					if requestID != "" {
						w.Header().Set("X-Request-ID", requestID)
					}

					// Return structured error response
					w.WriteHeader(http.StatusInternalServerError)
					errorResponse := fmt.Sprintf(`{"error": "Internal Server Error", "request_id": "%s"}`, requestID)
					_, _ = w.Write([]byte(errorResponse))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// SecurityHeadersConfig defines configuration for unified security headers middleware
type SecurityHeadersConfig struct {
	ReferrerPolicy string // "strict-origin-when-cross-origin", "no-referrer", etc.
	CSPPolicy      string // "basic", "comprehensive", or custom CSP string
	HStsEnabled    *bool  // Enable HSTS header (nil = use default)
	HStsMaxAge     int    // Max age for HSTS in seconds
	HStsSubdomains *bool  // Include subdomains in HSTS (nil = use default)
	HStsPreload    *bool  // Include preload in HSTS (nil = use default)
	XFrameOptions  string // "DENY", "SAMEORIGIN", etc.
	XContentType   string // "nosniff"
	XXSSProtection string // "1; mode=block", "0", etc.
}

// NewUnifiedSecurityMiddleware creates a configurable security headers middleware
func NewUnifiedSecurityMiddleware(config *SecurityHeadersConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Apply default values if config is nil or empty
			cfg := getSecurityConfig(config)

			// Set basic security headers
			if cfg.XContentType != "" {
				w.Header().Set("X-Content-Type-Options", cfg.XContentType)
			}
			if cfg.XFrameOptions != "" {
				w.Header().Set("X-Frame-Options", cfg.XFrameOptions)
			}
			if cfg.XXSSProtection != "" {
				w.Header().Set("X-XSS-Protection", cfg.XXSSProtection)
			}
			if cfg.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", cfg.ReferrerPolicy)
			}

			// Set CSP policy
			cspPolicy := getCSPPolicy(cfg.CSPPolicy)
			if cspPolicy != "" {
				w.Header().Set("Content-Security-Policy", cspPolicy)
			}

			// Set HSTS only if enabled and over HTTPS
			if cfg.HStsEnabled != nil && *cfg.HStsEnabled && r.TLS != nil {
				hstsValue := buildHSTSValue(cfg)
				w.Header().Set("Strict-Transport-Security", hstsValue)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getSecurityConfig returns a config with default values applied
func getSecurityConfig(config *SecurityHeadersConfig) *SecurityHeadersConfig {
	cfg := SecurityHeadersConfig{}
	if config != nil {
		cfg = *config
	}

	// Apply defaults for empty values
	if cfg.ReferrerPolicy == "" {
		cfg.ReferrerPolicy = "strict-origin-when-cross-origin"
	}
	if cfg.CSPPolicy == "" {
		cfg.CSPPolicy = "basic"
	}
	if cfg.XFrameOptions == "" {
		cfg.XFrameOptions = "DENY"
	}
	if cfg.XContentType == "" {
		cfg.XContentType = "nosniff"
	}
	if cfg.XXSSProtection == "" {
		cfg.XXSSProtection = "1; mode=block"
	}
	// HSTS defaults
	if cfg.HStsMaxAge == 0 {
		cfg.HStsMaxAge = 31536000 // 1 year
	}

	// Handle HSTS configuration with pointer bools for better API design
	if config == nil {
		// Nil config means use all defaults for backward compatibility
		cfg.HStsEnabled = boolPtr(true)
		cfg.HStsSubdomains = boolPtr(true)
		cfg.HStsPreload = boolPtr(false)
	} else {
		// Apply defaults for nil pointer values
		if cfg.HStsEnabled == nil {
			cfg.HStsEnabled = boolPtr(true) // Default enable HSTS for empty config
		}
		if cfg.HStsSubdomains == nil {
			cfg.HStsSubdomains = boolPtr(true) // Default include subdomains
		}
		if cfg.HStsPreload == nil {
			cfg.HStsPreload = boolPtr(false) // Default no preload
		}
	}

	return &cfg
}

// boolPtr is a helper function to create bool pointers
func boolPtr(b bool) *bool {
	return &b
}

// getCSPPolicy returns the appropriate CSP policy based on configuration
func getCSPPolicy(policy string) string {
	switch policy {
	case "basic":
		return "default-src 'self'"
	case "comprehensive":
		return "default-src 'self'; script-src 'self'; object-src 'none'"
	case "":
		return "default-src 'self'"
	default:
		// Custom CSP string
		return policy
	}
}

// buildHSTSValue constructs the HSTS header value based on configuration
func buildHSTSValue(cfg *SecurityHeadersConfig) string {
	parts := []string{"max-age=" + strconv.Itoa(cfg.HStsMaxAge)}

	if cfg.HStsSubdomains != nil && *cfg.HStsSubdomains {
		parts = append(parts, "includeSubDomains")
	}
	if cfg.HStsPreload != nil && *cfg.HStsPreload {
		parts = append(parts, "preload")
	}

	return strings.Join(parts, "; ")
}

// SecurityMiddleware adds basic security headers (backward compatibility)
func NewSecurityMiddleware() func(http.Handler) http.Handler {
	// Delegate to unified implementation with default config
	return NewUnifiedSecurityMiddleware(nil)
}

// Middleware type for middleware chains
type Middleware func(http.Handler) http.Handler

// CORS middleware constants
const (
	// DefaultCORSMaxAge represents 24 hours in seconds for CORS preflight cache
	DefaultCORSMaxAge = 86400
)

// CORSConfig holds CORS middleware configuration
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         int
}

// DefaultCORSConfig provides sensible default CORS configuration.
// Allows all origins (*), common HTTP methods, and standard headers.
var DefaultCORSConfig = CORSConfig{
	AllowedOrigins: []string{"*"},
	AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	AllowedHeaders: []string{"Content-Type", "Authorization"},
	MaxAge:         DefaultCORSMaxAge,
}

// NewCORSMiddleware creates CORS middleware with default configuration.
// This is a convenience function that delegates to NewCORSMiddlewareWithConfig.
func NewCORSMiddleware() func(http.Handler) http.Handler {
	return NewCORSMiddlewareWithConfig(DefaultCORSConfig)
}

// NewCORSMiddlewareWithConfig creates CORS middleware with custom config.
// The middleware handles CORS preflight and actual requests according to the provided configuration.
func NewCORSMiddlewareWithConfig(config CORSConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers based on config with safety checks
			if len(config.AllowedOrigins) > 0 {
				w.Header().Set("Access-Control-Allow-Origin", config.AllowedOrigins[0])
			}
			if len(config.AllowedMethods) > 0 {
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
			}
			if len(config.AllowedHeaders) > 0 {
				allowedHeaders := strings.Join(config.AllowedHeaders, ", ")
				// Append any requested headers to maintain backward compatibility with existing clients
				// that may send custom headers not explicitly configured in AllowedHeaders
				if requestedHeaders := r.Header.Get("Access-Control-Request-Headers"); requestedHeaders != "" {
					allowedHeaders += ", " + requestedHeaders
				}
				w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				// Only set MaxAge if it's a positive value to avoid invalid cache durations
				if config.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// NewMiddlewareChain creates a middleware chain
func NewMiddlewareChain(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		handler := next
		// Apply middlewares in reverse order for proper stacking
		for i := len(middlewares) - 1; i >= 0; i-- {
			handler = middlewares[i](handler)
		}
		return handler
	}
}
