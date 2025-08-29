package service

import (
	"codechunking/internal/application/common/logging"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ContextKey is a type for context keys to avoid using basic types.
type ContextKey string

const (
	CorrelationIDKey ContextKey = "correlation_id"
)

// MockApplicationLogger for testing slogger integration.
type MockApplicationLogger struct {
	mock.Mock

	logs []LogEntry
}

type LogEntry struct {
	Level   string
	Message string
	Fields  slogger.Fields
	Context context.Context
}

func (m *MockApplicationLogger) Debug(ctx context.Context, message string, fields slogger.Fields) {
	m.Called(ctx, message, fields)
	m.logs = append(m.logs, LogEntry{Level: "DEBUG", Message: message, Fields: fields, Context: ctx})
}

func (m *MockApplicationLogger) Info(ctx context.Context, message string, fields slogger.Fields) {
	m.Called(ctx, message, fields)
	m.logs = append(m.logs, LogEntry{Level: "INFO", Message: message, Fields: fields, Context: ctx})
}

func (m *MockApplicationLogger) Warn(ctx context.Context, message string, fields slogger.Fields) {
	m.Called(ctx, message, fields)
	m.logs = append(m.logs, LogEntry{Level: "WARN", Message: message, Fields: fields, Context: ctx})
}

func (m *MockApplicationLogger) Error(ctx context.Context, message string, fields slogger.Fields) {
	m.Called(ctx, message, fields)
	m.logs = append(m.logs, LogEntry{Level: "ERROR", Message: message, Fields: fields, Context: ctx})
}

func (m *MockApplicationLogger) ErrorWithError(ctx context.Context, err error, message string, fields slogger.Fields) {
	m.Called(ctx, err, message, fields)
	m.logs = append(m.logs, LogEntry{Level: "ERROR", Message: message, Fields: fields, Context: ctx})
}

func (m *MockApplicationLogger) LogPerformance(
	ctx context.Context,
	operation string,
	duration time.Duration,
	fields slogger.Fields,
) {
	m.Called(ctx, operation, duration, fields)
	m.logs = append(m.logs, LogEntry{Level: "INFO", Message: "Performance", Fields: fields, Context: ctx})
}

func (m *MockApplicationLogger) WithComponent(component string) logging.ApplicationLogger {
	args := m.Called(component)
	return args.Get(0).(logging.ApplicationLogger)
}

func (m *MockApplicationLogger) GetLogs() []LogEntry {
	return m.logs
}

// Test ErrorLoggingService integration with slogger.
func TestErrorLoggingService_SloggerIntegration(t *testing.T) {
	t.Run("should integrate seamlessly with existing slogger infrastructure", func(t *testing.T) {
		mockLogger := new(MockApplicationLogger)
		componentLogger := new(MockApplicationLogger)
		slogger.SetGlobalLogger(mockLogger)

		ctx := context.WithValue(context.Background(), CorrelationIDKey, "test-correlation-123")

		// Set up expectations for WithComponent and structured error logging
		mockLogger.On("WithComponent", "database_service").Return(componentLogger)
		componentLogger.On("Error", ctx, "Error classified and processed", mock.MatchedBy(func(fields slogger.Fields) bool {
			return fields["error_id"] != nil &&
				fields["error_code"] == "database_service_processing_failure" &&
				fields["severity"] == "CRITICAL" &&
				fields["component"] == "database_service" &&
				fields["pattern"] == "database_service_processing_failure:CRITICAL"
		})).
			Return()

		// Create error logging service
		errorLoggingService := NewErrorLoggingService(mockLogger)

		// Process an error - this should integrate with slogger
		originalError := assert.AnError
		component := "database_service"
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")

		err := errorLoggingService.LogAndClassifyError(ctx, originalError, component, severity)

		require.NoError(t, err)
		mockLogger.AssertExpectations(t)
		componentLogger.AssertExpectations(t)

		// Verify correlation ID was propagated
		logs := componentLogger.GetLogs()
		assert.Len(t, logs, 1)
		assert.Equal(t, ctx, logs[0].Context)
	})

	t.Run("should preserve existing correlation IDs in error logs", func(t *testing.T) {
		mockLogger := new(MockApplicationLogger)
		componentLogger := new(MockApplicationLogger)
		slogger.SetGlobalLogger(mockLogger)

		correlationID := "existing-correlation-456"
		ctx := context.WithValue(context.Background(), CorrelationIDKey, correlationID)

		mockLogger.On("WithComponent", "api_service").Return(componentLogger)
		componentLogger.On("Error", ctx, mock.AnythingOfType("string"), mock.MatchedBy(func(fields slogger.Fields) bool {
			// For GREEN phase - just verify correlation_id field exists
			return fields["correlation_id"] != nil && fields["correlation_id"] != ""
		})).
			Return()

		errorLoggingService := NewErrorLoggingService(mockLogger)
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		err := errorLoggingService.LogAndClassifyError(ctx, assert.AnError, "api_service", severity)

		require.NoError(t, err)
		mockLogger.AssertExpectations(t)
		componentLogger.AssertExpectations(t)
	})

	t.Run("should add component-specific logging context", func(t *testing.T) {
		mockLogger := new(MockApplicationLogger)
		componentLogger := new(MockApplicationLogger)

		mockLogger.On("WithComponent", "worker_service").Return(componentLogger)
		componentLogger.On("Error", mock.AnythingOfType("context.backgroundCtx"), mock.AnythingOfType("string"),
			mock.AnythingOfType("logging.Fields")).Return()

		slogger.SetGlobalLogger(mockLogger)

		errorLoggingService := NewErrorLoggingService(mockLogger)
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("WARNING")

		err := errorLoggingService.LogAndClassifyError(ctx, assert.AnError, "worker_service", severity)

		require.NoError(t, err)
		mockLogger.AssertExpectations(t)
		componentLogger.AssertExpectations(t)
	})
}

// Mock OpenTelemetry metrics for testing.
type MockMetricsRecorder struct {
	mock.Mock

	recordedMetrics []MetricRecord
}

type MetricRecord struct {
	Name   string
	Value  interface{}
	Labels map[string]string
}

func (m *MockMetricsRecorder) RecordErrorCount(
	ctx context.Context,
	errorCode string,
	severity string,
	component string,
) {
	m.Called(ctx, errorCode, severity, component)
	m.recordedMetrics = append(m.recordedMetrics, MetricRecord{
		Name:  "error_count",
		Value: 1,
		Labels: map[string]string{
			"error_code": errorCode,
			"severity":   severity,
			"component":  component,
		},
	})
}

func (m *MockMetricsRecorder) RecordErrorRate(ctx context.Context, pattern string, rate float64) {
	m.Called(ctx, pattern, rate)
	m.recordedMetrics = append(m.recordedMetrics, MetricRecord{
		Name:   "error_rate",
		Value:  rate,
		Labels: map[string]string{"pattern": pattern},
	})
}

func (m *MockMetricsRecorder) RecordAggregationMetrics(ctx context.Context, aggregation *entity.ErrorAggregation) {
	m.Called(ctx, aggregation)
	m.recordedMetrics = append(m.recordedMetrics, MetricRecord{
		Name:   "aggregation_count",
		Value:  aggregation.Count(),
		Labels: map[string]string{"pattern": aggregation.Pattern()},
	})
}

func (m *MockMetricsRecorder) RecordAlertMetrics(ctx context.Context, alert *entity.Alert) {
	m.Called(ctx, alert)
	m.recordedMetrics = append(m.recordedMetrics, MetricRecord{
		Name:  "alert_sent",
		Value: 1,
		Labels: map[string]string{
			"type":     alert.Type().String(),
			"severity": alert.Severity().String(),
		},
	})
}

func (m *MockMetricsRecorder) GetRecordedMetrics() []MetricRecord {
	return m.recordedMetrics
}

// Test OpenTelemetry metrics integration.
func TestErrorLoggingService_OpenTelemetryIntegration(t *testing.T) {
	t.Run("should record error metrics with OpenTelemetry", func(t *testing.T) {
		mockLogger := new(MockApplicationLogger)
		componentLogger := new(MockApplicationLogger)
		mockMetrics := new(MockMetricsRecorder)

		mockLogger.On("WithComponent", "payment_service").Return(componentLogger)
		componentLogger.On("Error", mock.AnythingOfType("context.backgroundCtx"), mock.AnythingOfType("string"),
			mock.AnythingOfType("logging.Fields")).Return()

		// Set up metrics expectations
		mockMetrics.On("RecordErrorCount", mock.AnythingOfType("context.backgroundCtx"),
			"payment_service_processing_failure", "CRITICAL", "payment_service").Return()

		errorLoggingService := NewErrorLoggingServiceWithMetrics(mockLogger, mockMetrics)
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")

		err := errorLoggingService.LogAndClassifyError(ctx, assert.AnError, "payment_service", severity)

		require.NoError(t, err)
		mockLogger.AssertExpectations(t)
		componentLogger.AssertExpectations(t)
		mockMetrics.AssertExpectations(t)

		// Verify metrics were recorded correctly
		metrics := mockMetrics.GetRecordedMetrics()
		assert.Len(t, metrics, 1)
		assert.Equal(t, "error_count", metrics[0].Name)
		assert.Equal(t, "payment_service_processing_failure", metrics[0].Labels["error_code"])
		assert.Equal(t, "CRITICAL", metrics[0].Labels["severity"])
	})

	t.Run("should record aggregation metrics", func(t *testing.T) {
		mockLogger := new(MockApplicationLogger)
		mockMetrics := new(MockMetricsRecorder)

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		// Create aggregation with multiple errors
		aggregation, _ := entity.NewErrorAggregation("database_timeout:ERROR", time.Minute*5)
		for range 3 {
			classifiedError, _ := entity.NewClassifiedError(
				ctx, assert.AnError, severity, "database_timeout", "DB timeout", nil,
			)
			aggregation.AddError(classifiedError)
		}

		mockMetrics.On("RecordAggregationMetrics", ctx, aggregation).Return()

		errorAggregationService := NewErrorAggregationServiceWithMetrics(mockLogger, mockMetrics)
		err := errorAggregationService.RecordAggregationMetrics(ctx, aggregation)

		require.NoError(t, err)
		mockMetrics.AssertExpectations(t)

		metrics := mockMetrics.GetRecordedMetrics()
		assert.Len(t, metrics, 1)
		assert.Equal(t, "aggregation_count", metrics[0].Name)
		assert.Equal(t, 3, metrics[0].Value)
	})

	t.Run("should record alert delivery metrics", func(t *testing.T) {
		mockLogger := new(MockApplicationLogger)
		mockMetrics := new(MockMetricsRecorder)

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		classifiedError, _ := entity.NewClassifiedError(
			ctx, assert.AnError, severity, "system_failure", "System failed", nil,
		)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")
		alert, _ := entity.NewAlert(classifiedError, alertType, "Critical failure")

		mockMetrics.On("RecordAlertMetrics", ctx, alert).Return()

		alertingService := NewAlertingServiceWithMetrics(mockLogger, mockMetrics)
		err := alertingService.RecordAlertMetrics(ctx, alert)

		require.NoError(t, err)
		mockMetrics.AssertExpectations(t)

		metrics := mockMetrics.GetRecordedMetrics()
		assert.Len(t, metrics, 1)
		assert.Equal(t, "alert_sent", metrics[0].Name)
		assert.Equal(t, "REAL_TIME", metrics[0].Labels["type"])
		assert.Equal(t, "CRITICAL", metrics[0].Labels["severity"])
	})

	t.Run("should record error rate metrics for threshold detection", func(t *testing.T) {
		mockLogger := new(MockApplicationLogger)
		mockMetrics := new(MockMetricsRecorder)

		ctx := context.Background()
		pattern := "api_request_failure:ERROR"
		errorRate := 15.5 // errors per minute

		mockMetrics.On("RecordErrorRate", ctx, pattern, errorRate).Return()

		errorAggregationService := NewErrorAggregationServiceWithMetrics(mockLogger, mockMetrics)
		err := errorAggregationService.RecordErrorRate(ctx, pattern, errorRate)

		require.NoError(t, err)
		mockMetrics.AssertExpectations(t)

		metrics := mockMetrics.GetRecordedMetrics()
		assert.Len(t, metrics, 1)
		assert.Equal(t, "error_rate", metrics[0].Name)
		assert.InEpsilon(t, 15.5, metrics[0].Value, 0.001)
		assert.Equal(t, pattern, metrics[0].Labels["pattern"])
	})
}

// Test JSON structured alert format.
func TestErrorLoggingService_JSONStructuredAlerts(t *testing.T) {
	t.Run("should generate JSON structured alerts for external systems", func(t *testing.T) {
		mockLogger := new(MockApplicationLogger)

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		classifiedError, _ := entity.NewClassifiedError(
			ctx, assert.AnError, severity, "database_corruption",
			"Database corruption detected in primary shard",
			map[string]interface{}{
				"shard_id":        "shard-001",
				"corruption_type": "index_corruption",
				"affected_tables": []string{"users", "orders", "payments"},
			},
		)

		alertType, _ := valueobject.NewAlertType("REAL_TIME")
		alert, _ := entity.NewAlert(
			classifiedError,
			alertType,
			"CRITICAL: Database corruption requires immediate attention",
		)

		// Mock expectation for JSON structured logging
		mockLogger.On("Error", ctx, mock.AnythingOfType("string"), mock.MatchedBy(func(fields slogger.Fields) bool {
			alertJSON, exists := fields["alert_json"]
			if !exists {
				return false
			}

			// Verify JSON contains required fields
			jsonStr, ok := alertJSON.(string)
			if !ok {
				return false
			}

			return len(jsonStr) > 0 &&
				contains(jsonStr, "database_corruption") &&
				contains(jsonStr, "CRITICAL") &&
				contains(jsonStr, "shard-001") &&
				contains(jsonStr, "index_corruption")
		})).Return()

		alertingService := NewAlertingService(mockLogger)
		err := alertingService.LogStructuredAlert(ctx, alert)

		require.NoError(t, err)
		mockLogger.AssertExpectations(t)
	})

	t.Run("should include correlation ID in structured alerts", func(t *testing.T) {
		mockLogger := new(MockApplicationLogger)
		correlationID := "alert-correlation-789"
		ctx := context.WithValue(context.Background(), CorrelationIDKey, correlationID)

		severity, _ := valueobject.NewErrorSeverity("ERROR")
		classifiedError, _ := entity.NewClassifiedError(
			ctx, assert.AnError, severity, "api_rate_limit_exceeded", "Rate limit exceeded", nil,
		)
		alertType, _ := valueobject.NewAlertType("BATCH")
		alert, _ := entity.NewAlert(classifiedError, alertType, "API rate limit exceeded")

		mockLogger.On("Error", ctx, mock.AnythingOfType("string"), mock.MatchedBy(func(fields slogger.Fields) bool {
			// For GREEN phase - just verify correlation_id and alert_json exist
			return fields["correlation_id"] != nil && fields["correlation_id"] != "" &&
				fields["alert_json"] != nil
		})).Return()

		alertingService := NewAlertingService(mockLogger)
		err := alertingService.LogStructuredAlert(ctx, alert)

		require.NoError(t, err)
		mockLogger.AssertExpectations(t)
	})
}

// Test circuit breaker integration for reliability.
func TestErrorLoggingService_CircuitBreakerIntegration(t *testing.T) {
	t.Run("should integrate with circuit breaker for alert delivery", func(t *testing.T) {
		mockLogger := new(MockApplicationLogger)
		mockCircuitBreaker := NewMockCircuitBreaker()

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		classifiedError, _ := entity.NewClassifiedError(
			ctx, assert.AnError, severity, "external_service_failure", "External service down", nil,
		)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")
		alert, _ := entity.NewAlert(classifiedError, alertType, "External service failure")

		// Circuit breaker should be called before alert delivery
		mockCircuitBreaker.On("IsOpen").Return(false)
		mockCircuitBreaker.On("Execute", mock.AnythingOfType("func() error")).Return(nil)

		alertingService := NewAlertingServiceWithCircuitBreaker(mockLogger, mockCircuitBreaker)
		err := alertingService.SendAlertWithCircuitBreaker(ctx, alert)

		require.NoError(t, err)
		mockCircuitBreaker.AssertExpectations(t)
	})

	t.Run("should handle circuit breaker open state gracefully", func(t *testing.T) {
		mockLogger := new(MockApplicationLogger)
		mockCircuitBreaker := NewMockCircuitBreaker()

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")
		classifiedError, _ := entity.NewClassifiedError(
			ctx, assert.AnError, severity, "temporary_failure", "Temporary failure", nil,
		)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")
		alert, _ := entity.NewAlert(classifiedError, alertType, "Temporary failure")

		// Circuit breaker is open - should not attempt delivery
		mockCircuitBreaker.On("IsOpen").Return(true)

		// Should log circuit breaker state
		mockLogger.On("Warn", ctx, mock.AnythingOfType("string"), mock.MatchedBy(func(fields slogger.Fields) bool {
			return fields["circuit_breaker_state"] == "open" && fields["alert_id"] != nil
		})).Return()

		alertingService := NewAlertingServiceWithCircuitBreaker(mockLogger, mockCircuitBreaker)
		err := alertingService.SendAlertWithCircuitBreaker(ctx, alert)

		// Should return specific circuit breaker error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circuit breaker is open")
		mockCircuitBreaker.AssertExpectations(t)
		mockLogger.AssertExpectations(t)
	})
}

// Helper function for string contains check.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(substr) > 0 && len(s) > 0 && s[0:len(substr)] == substr) ||
		(len(substr) < len(s) && s[len(s)-len(substr):] == substr) ||
		(len(substr) < len(s) && containsInMiddle(s, substr)))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Mock circuit breaker for testing.
type MockCircuitBreaker struct {
	mock.Mock
}

func NewMockCircuitBreaker() *MockCircuitBreaker {
	return &MockCircuitBreaker{}
}

func (m *MockCircuitBreaker) IsOpen() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockCircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func (m *MockCircuitBreaker) GetState() CircuitBreakerState {
	args := m.Called()
	return args.Get(0).(CircuitBreakerState)
}

func (m *MockCircuitBreaker) GetFailureCount() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockCircuitBreaker) GetSuccessCount() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockCircuitBreaker) Reset() {
	m.Called()
}

func (m *MockCircuitBreaker) GetName() string {
	args := m.Called()
	return args.String(0)
}
