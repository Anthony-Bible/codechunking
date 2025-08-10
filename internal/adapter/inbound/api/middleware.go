package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
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

// DefaultLogger provides a structured logger implementation
type DefaultLogger struct {
	output io.Writer
	fields map[string]interface{}
}

// NewDefaultLogger creates a new default logger that writes to stdout
func NewDefaultLogger() Logger {
	return &DefaultLogger{
		output: os.Stdout,
		fields: make(map[string]interface{}),
	}
}

// WithField adds a field to the logger context
func (l *DefaultLogger) WithField(key string, value interface{}) Logger {
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	newFields[key] = value

	return &DefaultLogger{
		output: l.output,
		fields: newFields,
	}
}

// WithFields adds multiple fields to the logger context
func (l *DefaultLogger) WithFields(fields map[string]interface{}) Logger {
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &DefaultLogger{
		output: l.output,
		fields: newFields,
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

	// Add fields if any
	if len(l.fields) > 0 {
		for key, value := range l.fields {
			logEntry += fmt.Sprintf(" %s=%v", key, value)
		}
	}

	fmt.Fprintln(l.output, logEntry)
}

// NewTestLogger creates a logger for testing that writes to the given writer
func NewTestLogger(output io.Writer) Logger {
	return &DefaultLogger{
		output: output,
		fields: make(map[string]interface{}),
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

// CORSMiddleware adds CORS headers
func NewCORSMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

			// Build allowed headers - start with defaults and add requested headers
			allowedHeaders := "Content-Type, Authorization"
			if requestedHeaders := r.Header.Get("Access-Control-Request-Headers"); requestedHeaders != "" {
				allowedHeaders += ", " + requestedHeaders
			}
			w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
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
				w.Write([]byte(errorResponse))
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
					w.Write([]byte(errorResponse))
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

// SecurityMiddleware adds basic security headers
func NewSecurityMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Security headers for production
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", "default-src 'self'")

			// Only set HSTS if over HTTPS
			if r.TLS != nil {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Middleware type for middleware chains
type Middleware func(http.Handler) http.Handler

// CORSConfig holds CORS middleware configuration
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         int
}

// NewCORSMiddlewareWithConfig creates CORS middleware with custom config
func NewCORSMiddlewareWithConfig(config CORSConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers based on config
			if len(config.AllowedOrigins) > 0 {
				w.Header().Set("Access-Control-Allow-Origin", config.AllowedOrigins[0])
			}
			if len(config.AllowedMethods) > 0 {
				methods := ""
				for i, method := range config.AllowedMethods {
					if i > 0 {
						methods += ", "
					}
					methods += method
				}
				w.Header().Set("Access-Control-Allow-Methods", methods)
			}
			if len(config.AllowedHeaders) > 0 {
				headers := ""
				for i, header := range config.AllowedHeaders {
					if i > 0 {
						headers += ", "
					}
					headers += header
				}
				w.Header().Set("Access-Control-Allow-Headers", headers)
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
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
