package service

import (
	"codechunking/internal/application/common/logging"
	"codechunking/internal/domain/entity"
	"context"
)

// AlertingService handles alert delivery concerns.
type AlertingService struct {
	logger         logging.ApplicationLogger
	metrics        MetricsRecorder
	circuitBreaker CircuitBreaker
}

// NewAlertingService creates a new AlertingService.
func NewAlertingService(logger logging.ApplicationLogger) *AlertingService {
	return &AlertingService{
		logger: logger,
	}
}

// NewAlertingServiceWithMetrics creates a new AlertingService with metrics.
func NewAlertingServiceWithMetrics(logger logging.ApplicationLogger, metrics MetricsRecorder) *AlertingService {
	return &AlertingService{
		logger:  logger,
		metrics: metrics,
	}
}

// NewAlertingServiceWithCircuitBreaker creates a new AlertingService with circuit breaker.
func NewAlertingServiceWithCircuitBreaker(logger logging.ApplicationLogger, cb CircuitBreaker) *AlertingService {
	return &AlertingService{
		logger:         logger,
		circuitBreaker: cb,
	}
}

// RecordAlertMetrics records metrics for alerts.
func (s *AlertingService) RecordAlertMetrics(ctx context.Context, alert *entity.Alert) error {
	if s.metrics != nil {
		s.metrics.RecordAlertMetrics(ctx, alert)
	}
	return nil
}

// LogStructuredAlert logs an alert with structured logging.
func (s *AlertingService) LogStructuredAlert(ctx context.Context, alert *entity.Alert) error {
	if s.logger == nil {
		return nil
	}

	// Convert alert to JSON for structured logging
	alertJSON, err := alert.MarshalJSON()
	if err != nil {
		return err
	}

	// Extract correlation ID from context
	correlationID := s.extractCorrelationID(ctx)

	// Log with structured format
	s.logger.Error(ctx, "Alert generated", map[string]interface{}{
		"alert_json":     string(alertJSON),
		"correlation_id": correlationID,
	})

	return nil
}

func (s *AlertingService) extractCorrelationID(ctx context.Context) string {
	// Try string key first
	if corrID := ctx.Value("correlation_id"); corrID != nil {
		if strID, ok := corrID.(string); ok {
			return strID
		}
	}

	// Try custom ContextKey type
	type ContextKey string
	const correlationIDKey ContextKey = "correlation_id"
	if corrID := ctx.Value(correlationIDKey); corrID != nil {
		if strID, ok := corrID.(string); ok {
			return strID
		}
	}

	return ""
}

// SendAlertWithCircuitBreaker sends an alert using circuit breaker pattern.
func (s *AlertingService) SendAlertWithCircuitBreaker(ctx context.Context, _ *entity.Alert) error {
	if s.circuitBreaker != nil {
		return s.circuitBreaker.Execute(ctx, func() error {
			// Minimal implementation - would integrate with actual alerting system
			return nil
		})
	}
	return nil
}
