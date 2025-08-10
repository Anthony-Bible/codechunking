package middleware

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStructuredLoggingMiddleware_RequestLogging tests comprehensive HTTP request logging
func TestStructuredLoggingMiddleware_RequestLogging(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		query          string
		requestBody    string
		responseStatus int
		responseBody   string
		headers        map[string]string
		expectedFields []string
	}{
		{
			name:           "GET request with query params",
			method:         "GET",
			path:           "/repositories",
			query:          "limit=10&offset=20",
			requestBody:    "",
			responseStatus: 200,
			responseBody:   `{"repositories": []}`,
			headers: map[string]string{
				"User-Agent":    "TestClient/1.0",
				"Authorization": "Bearer token123",
			},
			expectedFields: []string{"method", "path", "query", "status", "duration", "user_agent", "client_ip", "correlation_id"},
		},
		{
			name:           "POST request with body",
			method:         "POST",
			path:           "/repositories",
			requestBody:    `{"url": "https://github.com/user/repo"}`,
			responseStatus: 201,
			responseBody:   `{"id": "repo-123", "url": "https://github.com/user/repo"}`,
			headers: map[string]string{
				"Content-Type": "application/json",
				"User-Agent":   "API-Client/2.0",
			},
			expectedFields: []string{"method", "path", "status", "duration", "request_size", "response_size", "content_type"},
		},
		{
			name:           "ERROR response",
			method:         "POST",
			path:           "/repositories",
			requestBody:    `{"invalid": "data"}`,
			responseStatus: 400,
			responseBody:   `{"error": "Invalid repository URL"}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			expectedFields: []string{"method", "path", "status", "duration", "error_response"},
		},
		{
			name:           "Large request handling",
			method:         "POST",
			path:           "/search",
			requestBody:    strings.Repeat("x", 1024*1024), // 1MB request
			responseStatus: 413,
			responseBody:   `{"error": "Request too large"}`,
			expectedFields: []string{"method", "path", "status", "request_size", "large_request"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test handler that returns expected response
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.responseStatus)
				w.Write([]byte(tt.responseBody))
			})

			// Create structured logging middleware
			config := LoggingConfig{
				EnableRequestBody:  true,
				EnableResponseBody: true,
				MaxBodySize:        1024 * 1024 * 2, // 2MB
				SensitiveHeaders:   []string{"Authorization", "X-API-Key"},
				LogLevel:           "INFO",
				EnablePerfMetrics:  true, // Enable performance metrics for the test
			}

			middleware := NewStructuredLoggingMiddleware(config)
			wrappedHandler := middleware(handler)

			// Create request
			var body io.Reader
			if tt.requestBody != "" {
				body = strings.NewReader(tt.requestBody)
			}
			req := httptest.NewRequest(tt.method, tt.path+"?"+tt.query, body)

			// Add headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Execute request
			recorder := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(recorder, req)

			// Verify response
			assert.Equal(t, tt.responseStatus, recorder.Code)

			// Verify structured log was created
			logOutput := getMiddlewareLogOutput()
			assert.NotEmpty(t, logOutput, "Expected structured log output")

			var logEntry HTTPLogEntry
			err := json.Unmarshal([]byte(logOutput), &logEntry)
			require.NoError(t, err, "Log output should be valid JSON")

			// Verify required fields are present
			for _, field := range tt.expectedFields {
				assert.Contains(t, logEntry.Request, field, "Expected field %s in request metadata", field)
			}

			// Verify correlation ID was generated and set in response
			assert.NotEmpty(t, logEntry.CorrelationID)
			assert.Equal(t, logEntry.CorrelationID, recorder.Header().Get("X-Correlation-ID"))

			// Verify timing information
			assert.Greater(t, logEntry.Request["duration"], 0.0)
			assert.NotEmpty(t, logEntry.Timestamp)

			// Verify sensitive headers are masked
			if authHeader, exists := tt.headers["Authorization"]; exists {
				assert.NotContains(t, logOutput, authHeader, "Sensitive headers should be masked")
				assert.Contains(t, logEntry.Request, "authorization_present")
			}
		})
	}
}

// TestStructuredLoggingMiddleware_CorrelationIDHandling tests correlation ID generation and propagation
func TestStructuredLoggingMiddleware_CorrelationIDHandling(t *testing.T) {
	tests := []struct {
		name                  string
		incomingCorrelationID string
		expectNewID           bool
		expectSameID          bool
	}{
		{
			name:                  "generate new correlation ID when none provided",
			incomingCorrelationID: "",
			expectNewID:           true,
			expectSameID:          false,
		},
		{
			name:                  "use existing correlation ID when provided",
			incomingCorrelationID: "existing-correlation-123",
			expectNewID:           false,
			expectSameID:          true,
		},
		{
			name:                  "handle invalid correlation ID format",
			incomingCorrelationID: "invalid-format!@#",
			expectNewID:           true,
			expectSameID:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify correlation ID is available in context
				correlationID := GetCorrelationIDFromContext(r.Context())
				assert.NotEmpty(t, correlationID, "Correlation ID should be available in handler context")

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status": "ok"}`))
			})

			config := LoggingConfig{
				LogLevel: "INFO",
			}

			middleware := NewStructuredLoggingMiddleware(config)
			wrappedHandler := middleware(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.incomingCorrelationID != "" {
				req.Header.Set("X-Correlation-ID", tt.incomingCorrelationID)
			}

			recorder := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(recorder, req)

			// Verify response correlation ID
			responseCorrelationID := recorder.Header().Get("X-Correlation-ID")
			assert.NotEmpty(t, responseCorrelationID)

			if tt.expectSameID {
				assert.Equal(t, tt.incomingCorrelationID, responseCorrelationID)
			}

			if tt.expectNewID {
				assert.NotEqual(t, tt.incomingCorrelationID, responseCorrelationID)
				assert.True(t, isValidUUID(responseCorrelationID), "Generated correlation ID should be valid UUID")
			}

			// Verify log entry contains correct correlation ID
			logOutput := getMiddlewareLogOutput()
			var logEntry HTTPLogEntry
			err := json.Unmarshal([]byte(logOutput), &logEntry)
			require.NoError(t, err)

			assert.Equal(t, responseCorrelationID, logEntry.CorrelationID)
		})
	}
}

// TestStructuredLoggingMiddleware_ErrorHandling tests error logging and handling
func TestStructuredLoggingMiddleware_ErrorHandling(t *testing.T) {
	tests := []struct {
		name            string
		handlerBehavior func(w http.ResponseWriter, r *http.Request)
		expectPanic     bool
		expectError     bool
		expectedStatus  int
		expectedLevel   string
	}{
		{
			name: "handler panic recovery",
			handlerBehavior: func(w http.ResponseWriter, r *http.Request) {
				panic("test panic")
			},
			expectPanic:    true,
			expectError:    true,
			expectedStatus: 500,
			expectedLevel:  "ERROR",
		},
		{
			name: "context cancellation",
			handlerBehavior: func(w http.ResponseWriter, r *http.Request) {
				// Simulate context cancellation
				ctx, cancel := context.WithCancel(r.Context())
				cancel()
				select {
				case <-ctx.Done():
					w.WriteHeader(http.StatusRequestTimeout)
					w.Write([]byte(`{"error": "Request cancelled"}`))
				}
			},
			expectError:    true,
			expectedStatus: 408,
			expectedLevel:  "WARN",
		},
		{
			name: "application error",
			handlerBehavior: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "Invalid input"}`))
			},
			expectError:    false, // 4xx is client error, not server error
			expectedStatus: 400,
			expectedLevel:  "INFO",
		},
		{
			name: "server error",
			handlerBehavior: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "Database connection failed"}`))
			},
			expectError:    true,
			expectedStatus: 500,
			expectedLevel:  "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(tt.handlerBehavior)

			config := LoggingConfig{
				LogLevel:        "DEBUG",
				EnablePanicLogs: true,
			}

			middleware := NewStructuredLoggingMiddleware(config)
			wrappedHandler := middleware(handler)

			req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"test": "data"}`))
			recorder := httptest.NewRecorder()

			// Execute request (should not panic even if handler panics)
			require.NotPanics(t, func() {
				wrappedHandler.ServeHTTP(recorder, req)
			})

			// Verify response status
			if tt.expectPanic {
				assert.Equal(t, http.StatusInternalServerError, recorder.Code)
			} else {
				assert.Equal(t, tt.expectedStatus, recorder.Code)
			}

			// Verify structured log
			logOutput := getMiddlewareLogOutput()
			var logEntry HTTPLogEntry
			err := json.Unmarshal([]byte(logOutput), &logEntry)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedLevel, logEntry.Level)
			assert.NotEmpty(t, logEntry.CorrelationID)

			if tt.expectError {
				assert.Contains(t, logEntry.Message, "error", "Error logs should contain error information")
			}

			if tt.expectPanic {
				assert.Contains(t, logEntry.Request, "panic_recovered")
				assert.Contains(t, logEntry.Request, "stack_trace")
			}
		})
	}
}

// TestStructuredLoggingMiddleware_PerformanceLogging tests performance metrics logging
func TestStructuredLoggingMiddleware_PerformanceLogging(t *testing.T) {
	tests := []struct {
		name              string
		simulateDelay     time.Duration
		expectSlowLog     bool
		expectPerfMetrics bool
		requestSize       int
		responseSize      int
	}{
		{
			name:              "fast request",
			simulateDelay:     time.Millisecond * 10,
			expectSlowLog:     false,
			expectPerfMetrics: true,
			requestSize:       100,
			responseSize:      500,
		},
		{
			name:              "slow request",
			simulateDelay:     time.Second * 2,
			expectSlowLog:     true,
			expectPerfMetrics: true,
			requestSize:       1024,
			responseSize:      2048,
		},
		{
			name:              "large request",
			simulateDelay:     time.Millisecond * 100,
			expectSlowLog:     false,
			expectPerfMetrics: true,
			requestSize:       1024 * 1024, // 1MB
			responseSize:      1024 * 512,  // 512KB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Simulate processing delay
				time.Sleep(tt.simulateDelay)

				response := strings.Repeat("x", tt.responseSize)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(response))
			})

			config := LoggingConfig{
				LogLevel:             "INFO",
				SlowRequestThreshold: time.Second, // 1 second threshold
				EnablePerfMetrics:    true,
				MaxBodySize:          1024 * 1024 * 2, // 2MB
			}

			middleware := NewStructuredLoggingMiddleware(config)
			wrappedHandler := middleware(handler)

			requestBody := strings.Repeat("x", tt.requestSize)
			req := httptest.NewRequest("POST", "/test", strings.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			start := time.Now()
			wrappedHandler.ServeHTTP(recorder, req)
			actualDuration := time.Since(start)

			// Verify response
			assert.Equal(t, http.StatusOK, recorder.Code)

			// Verify log output
			logOutput := getMiddlewareLogOutput()
			var logEntry HTTPLogEntry
			err := json.Unmarshal([]byte(logOutput), &logEntry)
			require.NoError(t, err)

			// Verify performance metrics
			if tt.expectPerfMetrics {
				assert.Contains(t, logEntry.Request, "duration")
				assert.Contains(t, logEntry.Request, "request_size")
				assert.Contains(t, logEntry.Request, "response_size")

				duration := logEntry.Request["duration"].(float64)
				assert.InDelta(t, actualDuration.Seconds()*1000, duration, 100, "Duration should be close to actual")

				assert.Equal(t, float64(tt.requestSize), logEntry.Request["request_size"])
				assert.Equal(t, float64(tt.responseSize), logEntry.Request["response_size"])
			}

			// Verify slow request handling
			if tt.expectSlowLog {
				assert.Contains(t, logEntry.Request, "slow_request")
				assert.Equal(t, true, logEntry.Request["slow_request"])
				assert.Contains(t, logEntry.Message, "slow", "Slow requests should be marked in log message")
			}

			// Verify large request handling
			if tt.requestSize > 1024*512 { // 512KB threshold
				assert.Contains(t, logEntry.Request, "large_request")
				assert.Equal(t, true, logEntry.Request["large_request"])
			}
		})
	}
}

// TestStructuredLoggingMiddleware_SecurityLogging tests security-related logging
func TestStructuredLoggingMiddleware_SecurityLogging(t *testing.T) {
	tests := []struct {
		name              string
		headers           map[string]string
		requestBody       string
		expectSecurityLog bool
		securityIssues    []string
	}{
		{
			name: "suspicious user agent",
			headers: map[string]string{
				"User-Agent": "curl/7.68.0",
			},
			expectSecurityLog: true,
			securityIssues:    []string{"suspicious_user_agent"},
		},
		{
			name: "missing authorization",
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			requestBody:       `{"sensitive": "data"}`,
			expectSecurityLog: true,
			securityIssues:    []string{"missing_authorization"},
		},
		{
			name: "potential SQL injection in body",
			headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer valid-token",
			},
			requestBody:       `{"query": "SELECT * FROM users WHERE id = '1' OR '1'='1'"}`,
			expectSecurityLog: true,
			securityIssues:    []string{"potential_sql_injection"},
		},
		{
			name: "legitimate request",
			headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer valid-token",
				"User-Agent":    "MyApp/1.0",
			},
			requestBody:       `{"url": "https://github.com/user/repo"}`,
			expectSecurityLog: false,
			securityIssues:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status": "ok"}`))
			})

			config := LoggingConfig{
				LogLevel:              "INFO",
				EnableSecurityLogging: true,
				SuspiciousPatterns:    []string{"curl", "wget", "python-requests"},
			}

			middleware := NewStructuredLoggingMiddleware(config)
			wrappedHandler := middleware(handler)

			var body io.Reader
			if tt.requestBody != "" {
				body = strings.NewReader(tt.requestBody)
			}
			req := httptest.NewRequest("POST", "/api/repositories", body)

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			recorder := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(recorder, req)

			// Verify response
			assert.Equal(t, http.StatusOK, recorder.Code)

			// Verify security logging
			logOutput := getMiddlewareLogOutput()
			var logEntry HTTPLogEntry
			err := json.Unmarshal([]byte(logOutput), &logEntry)
			require.NoError(t, err)

			if tt.expectSecurityLog {
				assert.Contains(t, logEntry.Request, "security_issues")
				securityIssues := logEntry.Request["security_issues"].([]interface{})

				for _, expectedIssue := range tt.securityIssues {
					found := false
					for _, issue := range securityIssues {
						if issue.(string) == expectedIssue {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected security issue %s not found", expectedIssue)
				}
			} else {
				assert.NotContains(t, logEntry.Request, "security_issues")
			}
		})
	}
}

// TestStructuredLoggingMiddleware_FilteringAndSampling tests log filtering and sampling
func TestStructuredLoggingMiddleware_FilteringAndSampling(t *testing.T) {
	tests := []struct {
		name         string
		config       LoggingConfig
		requests     int
		expectedLogs int
		path         string
		method       string
		shouldLog    bool
	}{
		{
			name: "health check filtering",
			config: LoggingConfig{
				LogLevel:     "INFO",
				ExcludePaths: []string{"/health", "/metrics"},
			},
			requests:     5,
			expectedLogs: 0,
			path:         "/health",
			method:       "GET",
			shouldLog:    false,
		},
		{
			name: "sampling rate 50%",
			config: LoggingConfig{
				LogLevel:   "INFO",
				SampleRate: 0.5,
			},
			requests:     100,
			expectedLogs: 50, // Approximately 50% should be logged
			path:         "/api/test",
			method:       "GET",
			shouldLog:    true, // Some should log
		},
		{
			name: "debug level filtering",
			config: LoggingConfig{
				LogLevel: "ERROR",
			},
			requests:     10,
			expectedLogs: 0, // INFO level requests shouldn't log at ERROR level
			path:         "/api/test",
			method:       "GET",
			shouldLog:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status": "ok"}`))
			})

			middleware := NewStructuredLoggingMiddleware(tt.config)
			wrappedHandler := middleware(handler)

			logCount := 0
			for i := 0; i < tt.requests; i++ {
				req := httptest.NewRequest(tt.method, tt.path, nil)
				recorder := httptest.NewRecorder()

				wrappedHandler.ServeHTTP(recorder, req)

				if hasLogOutput() {
					logCount++
				}
				clearLogOutput()
			}

			if tt.config.SampleRate > 0 && tt.config.SampleRate < 1 {
				// For sampling, verify approximate count with tolerance
				tolerance := int(float64(tt.requests) * 0.2) // 20% tolerance
				assert.InDelta(t, tt.expectedLogs, logCount, float64(tolerance),
					"Sampling should produce approximately expected log count")
			} else {
				assert.Equal(t, tt.expectedLogs, logCount,
					"Exact log count should match expected for non-sampled logging")
			}
		})
	}
}

// Test helper functions - implementations are now in logging_middleware.go
