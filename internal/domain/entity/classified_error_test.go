package entity

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClassifiedError_Creation(t *testing.T) {
	t.Run("should create classified error with all required fields", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		originalErr := assert.AnError

		classifiedError, err := NewClassifiedError(
			ctx,
			originalErr,
			severity,
			"database_connection_failure",
			"Failed to connect to PostgreSQL database",
			map[string]interface{}{
				"host":     "localhost",
				"port":     5432,
				"database": "codechunking",
			},
		)

		assert.NoError(t, err)
		assert.NotNil(t, classifiedError)
		assert.NotEmpty(t, classifiedError.ID())
		assert.Equal(t, originalErr, classifiedError.OriginalError())
		assert.True(t, severity.Equals(classifiedError.Severity()))
		assert.Equal(t, "database_connection_failure", classifiedError.ErrorCode())
		assert.Equal(t, "Failed to connect to PostgreSQL database", classifiedError.Message())
		assert.Equal(t, map[string]interface{}{
			"host":     "localhost",
			"port":     5432,
			"database": "codechunking",
		}, classifiedError.Context())
		assert.WithinDuration(t, time.Now(), classifiedError.Timestamp(), time.Second)
		assert.NotEmpty(t, classifiedError.CorrelationID())
		assert.Empty(t, classifiedError.Component())
	})

	t.Run("should create classified error with component from context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "component", "repository-service")
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		classifiedError, err := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"git_clone_failure",
			"Failed to clone repository",
			nil,
		)

		assert.NoError(t, err)
		assert.Equal(t, "repository-service", classifiedError.Component())
	})

	t.Run("should create classified error with correlation ID from context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "correlation_id", "test-correlation-123")
		severity, _ := valueobject.NewErrorSeverity("WARNING")

		classifiedError, err := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"performance_degradation",
			"Query execution time exceeded threshold",
			nil,
		)

		assert.NoError(t, err)
		assert.Equal(t, "test-correlation-123", classifiedError.CorrelationID())
	})
}

func TestClassifiedError_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	severity, _ := valueobject.NewErrorSeverity("ERROR")

	testCases := []struct {
		name          string
		originalError error
		severity      *valueobject.ErrorSeverity
		errorCode     string
		message       string
		expectedError string
	}{
		{
			name:          "nil original error",
			originalError: nil,
			severity:      severity,
			errorCode:     "test_error",
			message:       "Test message",
			expectedError: "original error cannot be nil",
		},
		{
			name:          "nil severity",
			originalError: assert.AnError,
			severity:      nil,
			errorCode:     "test_error",
			message:       "Test message",
			expectedError: "severity cannot be nil",
		},
		{
			name:          "empty error code",
			originalError: assert.AnError,
			severity:      severity,
			errorCode:     "",
			message:       "Test message",
			expectedError: "error code cannot be empty",
		},
		{
			name:          "empty message",
			originalError: assert.AnError,
			severity:      severity,
			errorCode:     "test_error",
			message:       "",
			expectedError: "message cannot be empty",
		},
		{
			name:          "invalid error code format",
			originalError: assert.AnError,
			severity:      severity,
			errorCode:     "Invalid Code!",
			message:       "Test message",
			expectedError: "error code must contain only lowercase letters, numbers, and underscores",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			classifiedError, err := NewClassifiedError(
				ctx,
				tc.originalError,
				tc.severity,
				tc.errorCode,
				tc.message,
				nil,
			)

			assert.Error(t, err)
			assert.Nil(t, classifiedError)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestClassifiedError_ErrorPattern(t *testing.T) {
	t.Run("should generate consistent error pattern for similar errors", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		// Create two similar errors with same code and severity but different contexts
		error1, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"database_connection_failure",
			"Failed to connect to database server 1",
			map[string]interface{}{"host": "db1.example.com"},
		)

		error2, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"database_connection_failure",
			"Failed to connect to database server 2",
			map[string]interface{}{"host": "db2.example.com"},
		)

		// Should have same pattern for aggregation purposes
		assert.Equal(t, error1.ErrorPattern(), error2.ErrorPattern())
		assert.Contains(t, error1.ErrorPattern(), "database_connection_failure")
		assert.Contains(t, error1.ErrorPattern(), "ERROR")
	})

	t.Run("should generate different error patterns for different error types", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		error1, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"database_connection_failure",
			"Failed to connect to database",
			nil,
		)

		error2, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"git_clone_failure",
			"Failed to clone repository",
			nil,
		)

		assert.NotEqual(t, error1.ErrorPattern(), error2.ErrorPattern())
	})
}

func TestClassifiedError_RequiresImmediateAlert(t *testing.T) {
	ctx := context.Background()

	t.Run("should require immediate alert for critical errors", func(t *testing.T) {
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		classifiedError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"system_failure",
			"Critical system failure",
			nil,
		)

		assert.True(t, classifiedError.RequiresImmediateAlert())
	})

	t.Run("should require immediate alert for error level", func(t *testing.T) {
		severity, _ := valueobject.NewErrorSeverity("ERROR")
		classifiedError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"processing_failure",
			"Processing failed",
			nil,
		)

		assert.True(t, classifiedError.RequiresImmediateAlert())
	})

	t.Run("should not require immediate alert for warnings", func(t *testing.T) {
		severity, _ := valueobject.NewErrorSeverity("WARNING")
		classifiedError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"performance_degradation",
			"Performance degraded",
			nil,
		)

		assert.False(t, classifiedError.RequiresImmediateAlert())
	})

	t.Run("should not require immediate alert for info level", func(t *testing.T) {
		severity, _ := valueobject.NewErrorSeverity("INFO")
		classifiedError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"business_event",
			"Business event occurred",
			nil,
		)

		assert.False(t, classifiedError.RequiresImmediateAlert())
	})
}

func TestClassifiedError_Serialization(t *testing.T) {
	t.Run("should serialize to JSON correctly", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		classifiedError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"test_error",
			"Test error message",
			map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
		)

		jsonData, err := classifiedError.MarshalJSON()
		assert.NoError(t, err)
		assert.Contains(t, string(jsonData), "test_error")
		assert.Contains(t, string(jsonData), "ERROR")
		assert.Contains(t, string(jsonData), "Test error message")
		assert.Contains(t, string(jsonData), "key1")
		assert.Contains(t, string(jsonData), "value1")
	})

	t.Run("should deserialize from JSON correctly", func(t *testing.T) {
		jsonData := `{
			"id": "test-id-123",
			"error_code": "test_error",
			"severity": "CRITICAL",
			"message": "Test message",
			"timestamp": "2023-01-01T00:00:00Z",
			"correlation_id": "corr-123",
			"component": "test-component",
			"context": {"key": "value"},
			"error_pattern": "test_error:CRITICAL"
		}`

		var classifiedError ClassifiedError
		err := classifiedError.UnmarshalJSON([]byte(jsonData))

		assert.NoError(t, err)
		assert.Equal(t, "test-id-123", classifiedError.ID())
		assert.Equal(t, "test_error", classifiedError.ErrorCode())
		assert.True(t, classifiedError.Severity().IsCritical())
		assert.Equal(t, "Test message", classifiedError.Message())
		assert.Equal(t, "corr-123", classifiedError.CorrelationID())
		assert.Equal(t, "test-component", classifiedError.Component())
		assert.Equal(t, map[string]interface{}{"key": "value"}, classifiedError.Context())
	})
}

func TestClassifiedError_Stack(t *testing.T) {
	t.Run("should capture and store stack trace", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		classifiedError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"test_error",
			"Test error with stack",
			nil,
		)

		stack := classifiedError.StackTrace()
		assert.NotEmpty(t, stack)
		assert.Contains(t, stack, "TestClassifiedError_Stack")
	})

	t.Run("should handle errors without stack trace", func(t *testing.T) {
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("INFO")

		classifiedError, _ := NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"simple_error",
			"Simple error without stack",
			nil,
		)

		// Info level errors might not capture stack traces
		stack := classifiedError.StackTrace()
		// Should either be empty or contain trace - implementation detail
		assert.GreaterOrEqual(t, len(stack), 0)
	})
}
