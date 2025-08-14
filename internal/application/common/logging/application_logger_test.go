package logging

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplicationLogger_CreateStructuredLogger tests creation of structured logger.
func TestApplicationLogger_CreateStructuredLogger(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		wantJSON bool
		wantErr  bool
	}{
		{
			name: "create logger with JSON format",
			config: Config{
				Level:  "INFO",
				Format: "json",
				Output: "stdout",
			},
			wantJSON: true,
			wantErr:  false,
		},
		{
			name: "create logger with text format",
			config: Config{
				Level:  "DEBUG",
				Format: "text",
				Output: "stdout",
			},
			wantJSON: false,
			wantErr:  false,
		},
		{
			name: "create logger with invalid level",
			config: Config{
				Level:  "INVALID",
				Format: "json",
				Output: "stdout",
			},
			wantJSON: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewApplicationLogger(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, logger)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, logger)

			// Test that logger implements expected interface
			assert.Implements(t, (*ApplicationLogger)(nil), logger)
		})
	}
}

// TestApplicationLogger_LogLevels tests different log levels.
func TestApplicationLogger_LogLevels(t *testing.T) {
	config := Config{
		Level:  "DEBUG",
		Format: "json",
		Output: "buffer", // Special output for testing
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	ctx := context.Background()
	correlationID := "test-correlation-123"
	ctx = WithCorrelationID(ctx, correlationID)

	tests := []struct {
		name    string
		logFunc func()
		level   string
		message string
	}{
		{
			name: "debug log",
			logFunc: func() {
				logger.Debug(ctx, "debug message", Fields{"debug_field": "debug_value"})
			},
			level:   "DEBUG",
			message: "debug message",
		},
		{
			name: "info log",
			logFunc: func() {
				logger.Info(ctx, "info message", Fields{"info_field": "info_value"})
			},
			level:   "INFO",
			message: "info message",
		},
		{
			name: "warn log",
			logFunc: func() {
				logger.Warn(ctx, "warn message", Fields{"warn_field": "warn_value"})
			},
			level:   "WARN",
			message: "warn message",
		},
		{
			name: "error log",
			logFunc: func() {
				logger.Error(ctx, "error message", Fields{"error_field": "error_value"})
			},
			level:   "ERROR",
			message: "error message",
		},
		{
			name: "error with error object",
			logFunc: func() {
				logger.ErrorWithError(ctx, errors.New("test error"), "operation failed", Fields{"operation": "test_op"})
			},
			level:   "ERROR",
			message: "operation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail until implementation exists
			tt.logFunc()

			// Get captured log output
			output := getLoggerOutput(logger)
			assert.NotEmpty(t, output, "Expected log output to be captured")

			// Parse JSON log entry
			var logEntry LogEntry
			err := json.Unmarshal([]byte(output), &logEntry)
			assert.NoError(t, err, "Log output should be valid JSON")

			// Verify log entry structure
			assert.Equal(t, tt.level, logEntry.Level)
			assert.Equal(t, tt.message, logEntry.Message)
			assert.Equal(t, correlationID, logEntry.CorrelationID)
			assert.NotEmpty(t, logEntry.Timestamp)
			assert.NotEmpty(t, logEntry.Component)
		})
	}
}

// TestApplicationLogger_CorrelationIDGeneration tests correlation ID handling.
func TestApplicationLogger_CorrelationIDGeneration(t *testing.T) {
	config := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	tests := []struct {
		name                string
		ctx                 context.Context
		expectCorrelationID bool
		expectedID          string
	}{
		{
			name:                "context with correlation ID",
			ctx:                 WithCorrelationID(context.Background(), "existing-correlation-123"),
			expectCorrelationID: true,
			expectedID:          "existing-correlation-123",
		},
		{
			name:                "context without correlation ID - should generate one",
			ctx:                 context.Background(),
			expectCorrelationID: true,
			expectedID:          "", // Will be generated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger.Info(tt.ctx, "test message", Fields{})

			output := getLoggerOutput(logger)
			var logEntry LogEntry
			err := json.Unmarshal([]byte(output), &logEntry)
			require.NoError(t, err)

			if tt.expectCorrelationID {
				if tt.expectedID != "" {
					assert.Equal(t, tt.expectedID, logEntry.CorrelationID)
				} else {
					assert.NotEmpty(t, logEntry.CorrelationID)
					assert.True(t, isValidUUID(logEntry.CorrelationID), "Generated correlation ID should be valid UUID")
				}
			} else {
				assert.Empty(t, logEntry.CorrelationID)
			}
		})
	}
}

// TestApplicationLogger_StructuredFields tests structured field logging.
func TestApplicationLogger_StructuredFields(t *testing.T) {
	config := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	ctx := WithCorrelationID(context.Background(), "test-correlation")

	testFields := Fields{
		"user_id":        "user-123",
		"operation":      "create_repository",
		"repository_url": "https://github.com/user/repo",
		"duration":       time.Millisecond * 150,
		"success":        true,
		"error_count":    0,
	}

	logger.Info(ctx, "Repository created successfully", testFields)

	output := getLoggerOutput(logger)
	var logEntry LogEntry
	err = json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)

	// Verify all fields are present in metadata
	assert.Equal(t, "user-123", logEntry.Metadata["user_id"])
	assert.Equal(t, "create_repository", logEntry.Metadata["operation"])
	assert.Equal(t, "https://github.com/user/repo", logEntry.Metadata["repository_url"])
	assert.Contains(t, logEntry.Metadata, "duration")
	assert.Equal(t, true, logEntry.Metadata["success"])
	assert.Equal(t, float64(0), logEntry.Metadata["error_count"]) // JSON unmarshals numbers as float64
}

// TestApplicationLogger_ContextualLogging tests context-aware logging.
func TestApplicationLogger_ContextualLogging(t *testing.T) {
	config := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	// Create context with multiple values
	ctx := context.Background()
	ctx = WithCorrelationID(ctx, "correlation-123")
	ctx = WithRequestID(ctx, "request-456")
	ctx = WithUserContext(ctx, UserContext{
		UserID:    "user-789",
		ClientIP:  "192.168.1.100",
		UserAgent: "TestClient/1.0",
	})

	logger.Info(ctx, "User operation completed", Fields{
		"operation": "get_repositories",
		"count":     5,
	})

	output := getLoggerOutput(logger)
	var logEntry LogEntry
	err = json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)

	// Verify correlation and request IDs
	assert.Equal(t, "correlation-123", logEntry.CorrelationID)
	assert.Equal(t, "request-456", logEntry.Context["request_id"])

	// Verify user context
	assert.Equal(t, "user-789", logEntry.Context["user_id"])
	assert.Equal(t, "192.168.1.100", logEntry.Context["client_ip"])
	assert.Equal(t, "TestClient/1.0", logEntry.Context["user_agent"])

	// Verify operation metadata
	assert.Equal(t, "get_repositories", logEntry.Metadata["operation"])
	assert.Equal(t, float64(5), logEntry.Metadata["count"])
}

// TestApplicationLogger_ComponentLogging tests component-specific logging.
func TestApplicationLogger_ComponentLogging(t *testing.T) {
	config := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	tests := []struct {
		name      string
		component string
		operation string
	}{
		{
			name:      "API handler logging",
			component: "api-handler",
			operation: "create_repository",
		},
		{
			name:      "Service layer logging",
			component: "indexing-service",
			operation: "process_repository",
		},
		{
			name:      "NATS publisher logging",
			component: "nats-publisher",
			operation: "publish_job",
		},
		{
			name:      "Repository logging",
			component: "repository",
			operation: "save_entity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithCorrelationID(context.Background(), "test-correlation")
			componentLogger := logger.WithComponent(tt.component)

			componentLogger.Info(ctx, "Operation executed", Fields{
				"operation": tt.operation,
				"duration":  "5ms",
			})

			output := getLoggerOutput(componentLogger)
			var logEntry LogEntry
			err := json.Unmarshal([]byte(output), &logEntry)
			require.NoError(t, err)

			assert.Equal(t, tt.component, logEntry.Component)
			assert.Equal(t, tt.operation, logEntry.Metadata["operation"])
			assert.Equal(t, "Operation executed", logEntry.Message)
		})
	}
}

// TestApplicationLogger_ErrorLogging tests error logging capabilities.
func TestApplicationLogger_ErrorLogging(t *testing.T) {
	config := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	ctx := WithCorrelationID(context.Background(), "error-correlation")

	testError := errors.New("database connection failed")

	logger.ErrorWithError(ctx, testError, "Failed to save repository", Fields{
		"repository_id": "repo-123",
		"operation":     "save",
		"retry_count":   3,
		"last_attempt":  time.Now().UTC(),
	})

	output := getLoggerOutput(logger)
	var logEntry LogEntry
	err = json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "ERROR", logEntry.Level)
	assert.Equal(t, "Failed to save repository", logEntry.Message)
	assert.Equal(t, "database connection failed", logEntry.Error)
	assert.Equal(t, "repo-123", logEntry.Metadata["repository_id"])
	assert.Equal(t, "save", logEntry.Metadata["operation"])
	assert.Equal(t, float64(3), logEntry.Metadata["retry_count"])
	assert.Contains(t, logEntry.Metadata, "last_attempt")
}

// TestApplicationLogger_PerformanceLogging tests performance metrics logging.
func TestApplicationLogger_PerformanceLogging(t *testing.T) {
	config := Config{
		Level:  "INFO",
		Format: "json",
		Output: "buffer",
	}

	logger, err := NewApplicationLogger(config)
	require.NoError(t, err)

	ctx := WithCorrelationID(context.Background(), "perf-correlation")

	// Test operation timing
	start := time.Now()
	time.Sleep(time.Millisecond * 10) // Simulate work
	duration := time.Since(start)

	logger.LogPerformance(ctx, "repository_indexing", duration, Fields{
		"repository_count": 5,
		"chunk_count":      150,
		"embedding_count":  150,
		"memory_usage":     "45MB",
		"cpu_usage":        "23%",
	})

	output := getLoggerOutput(logger)
	var logEntry LogEntry
	err = json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "INFO", logEntry.Level)
	assert.Contains(t, logEntry.Message, "Performance metrics")
	assert.Contains(t, logEntry.Metadata, "operation")
	assert.Contains(t, logEntry.Metadata, "duration")
	assert.Equal(t, float64(5), logEntry.Metadata["repository_count"])
	assert.Equal(t, float64(150), logEntry.Metadata["chunk_count"])
	assert.Equal(t, "45MB", logEntry.Metadata["memory_usage"])
}

// TestApplicationLogger_ConfigurationValidation tests configuration validation.
func TestApplicationLogger_ConfigurationValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration",
			config: Config{
				Level:            "INFO",
				Format:           "json",
				Output:           "stdout",
				EnableColors:     false,
				TimestampFormat:  "rfc3339",
				EnableStackTrace: true,
			},
			wantErr: false,
		},
		{
			name: "invalid log level",
			config: Config{
				Level:  "INVALID",
				Format: "json",
				Output: "stdout",
			},
			wantErr: true,
			errMsg:  "invalid log level",
		},
		{
			name: "invalid format",
			config: Config{
				Level:  "INFO",
				Format: "invalid",
				Output: "stdout",
			},
			wantErr: true,
			errMsg:  "invalid log format",
		},
		{
			name: "invalid output",
			config: Config{
				Level:  "INFO",
				Format: "json",
				Output: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid log output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewApplicationLogger(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, logger)
				if tt.errMsg != "" {
					assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errMsg))
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, logger)
			}
		})
	}
}

// TestApplicationLogger_LogFiltering tests log level filtering.
func TestApplicationLogger_LogFiltering(t *testing.T) {
	tests := []struct {
		name        string
		configLevel string
		logLevel    string
		shouldLog   bool
	}{
		{"DEBUG config allows DEBUG", "DEBUG", "DEBUG", true},
		{"DEBUG config allows INFO", "DEBUG", "INFO", true},
		{"DEBUG config allows WARN", "DEBUG", "WARN", true},
		{"DEBUG config allows ERROR", "DEBUG", "ERROR", true},
		{"INFO config filters DEBUG", "INFO", "DEBUG", false},
		{"INFO config allows INFO", "INFO", "INFO", true},
		{"INFO config allows WARN", "INFO", "WARN", true},
		{"INFO config allows ERROR", "INFO", "ERROR", true},
		{"WARN config filters DEBUG", "WARN", "DEBUG", false},
		{"WARN config filters INFO", "WARN", "INFO", false},
		{"WARN config allows WARN", "WARN", "WARN", true},
		{"WARN config allows ERROR", "WARN", "ERROR", true},
		{"ERROR config filters DEBUG", "ERROR", "DEBUG", false},
		{"ERROR config filters INFO", "ERROR", "INFO", false},
		{"ERROR config filters WARN", "ERROR", "WARN", false},
		{"ERROR config allows ERROR", "ERROR", "ERROR", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Level:  tt.configLevel,
				Format: "json",
				Output: "buffer",
			}

			logger, err := NewApplicationLogger(config)
			require.NoError(t, err)

			ctx := WithCorrelationID(context.Background(), "filter-test")

			// Log message at specified level
			switch tt.logLevel {
			case "DEBUG":
				logger.Debug(ctx, "debug message", Fields{})
			case "INFO":
				logger.Info(ctx, "info message", Fields{})
			case "WARN":
				logger.Warn(ctx, "warn message", Fields{})
			case "ERROR":
				logger.Error(ctx, "error message", Fields{})
			}

			output := getLoggerOutput(logger)

			if tt.shouldLog {
				assert.NotEmpty(
					t,
					output,
					"Expected log output for level %s with config %s",
					tt.logLevel,
					tt.configLevel,
				)

				var logEntry LogEntry
				err := json.Unmarshal([]byte(output), &logEntry)
				assert.NoError(t, err)
				assert.Equal(t, tt.logLevel, logEntry.Level)
			} else {
				assert.Empty(t, output, "Expected no log output for level %s with config %s", tt.logLevel, tt.configLevel)
			}
		})
	}
}

// Helper functions for testing

func isValidUUID(s string) bool {
	// Simple UUID validation for testing
	return len(s) == 36 && strings.Count(s, "-") == 4
}
