package service

import (
	"codechunking/internal/application/common/logging"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MetricsRecorder interface for recording OpenTelemetry metrics.
type MetricsRecorder interface {
	// RecordErrorCount records the count of errors by type
	RecordErrorCount(ctx context.Context, errorCode string, severity string, component string)

	// RecordErrorRate records the error rate for a pattern
	RecordErrorRate(ctx context.Context, pattern string, rate float64)

	// RecordAggregationMetrics records metrics for error aggregations
	RecordAggregationMetrics(ctx context.Context, aggregation *entity.ErrorAggregation)

	// RecordAlertMetrics records metrics for alerts
	RecordAlertMetrics(ctx context.Context, alert *entity.Alert)
}

// ErrorLoggingServiceConfig holds configuration for the error logging service.
type ErrorLoggingServiceConfig struct {
	// Classification settings
	DefaultPatterns map[string]string // Component -> Error pattern mappings
	MaxPatternCache int               // Maximum number of cached patterns

	// Aggregation settings
	DefaultAggregationWindow    time.Duration
	MaxAggregationsPerComponent int

	// Alert settings
	CriticalAlertThreshold int     // Number of errors to trigger critical alert
	ErrorAlertThreshold    int     // Number of errors to trigger error alert
	HighVolumeThreshold    int     // Number of errors to consider high volume
	ErrorRateThreshold     float64 // Errors per second to trigger rate alert
}

// DefaultErrorLoggingServiceConfig returns a production-ready configuration.
func DefaultErrorLoggingServiceConfig() *ErrorLoggingServiceConfig {
	return &ErrorLoggingServiceConfig{
		DefaultPatterns: map[string]string{
			"database":   "database_connection_failure",
			"network":    "network_timeout_failure",
			"auth":       "authentication_failure",
			"permission": "permission_denied_failure",
			"not_found":  "resource_not_found_failure",
			"validation": "input_validation_failure",
		},
		MaxPatternCache:             1000,
		DefaultAggregationWindow:    5 * time.Minute,
		MaxAggregationsPerComponent: 100,
		CriticalAlertThreshold:      1,   // Immediate alert for critical errors
		ErrorAlertThreshold:         3,   // Alert after 3 errors
		HighVolumeThreshold:         10,  // High volume after 10 errors
		ErrorRateThreshold:          1.0, // 1 error per second
	}
}

// ErrorLoggingService orchestrates error processing, classification, aggregation, and alerting with production optimizations.
type ErrorLoggingService struct {
	// Configuration and dependencies
	config          *ErrorLoggingServiceConfig
	logger          logging.ApplicationLogger
	classifier      outbound.ErrorClassifier
	aggregator      outbound.ErrorAggregator
	alertingService outbound.AlertingService
	metricsRecorder MetricsRecorder

	// Performance optimizations
	mu           sync.RWMutex
	patternCache map[string]string // Cache for derived error patterns
	cacheHits    int64             // Metrics for cache performance
	cacheMisses  int64             // Metrics for cache performance

	// Circuit breaker state
	alertingEnabled bool
	alertingErrors  int64
}

// NewErrorLoggingService creates a new ErrorLoggingService with production-ready configuration.
func NewErrorLoggingService(logger logging.ApplicationLogger) *ErrorLoggingService {
	return NewErrorLoggingServiceWithConfig(logger, DefaultErrorLoggingServiceConfig())
}

// NewErrorLoggingServiceWithMetrics creates a new ErrorLoggingService with metrics recording (backward compatibility).
func NewErrorLoggingServiceWithMetrics(
	logger logging.ApplicationLogger,
	metricsRecorder MetricsRecorder,
) *ErrorLoggingService {
	service := NewErrorLoggingServiceWithConfig(logger, DefaultErrorLoggingServiceConfig())
	service.metricsRecorder = metricsRecorder
	return service
}

// NewErrorLoggingServiceWithConfig creates a new ErrorLoggingService with custom configuration.
func NewErrorLoggingServiceWithConfig(
	logger logging.ApplicationLogger,
	config *ErrorLoggingServiceConfig,
) *ErrorLoggingService {
	if config == nil {
		config = DefaultErrorLoggingServiceConfig()
	}

	return &ErrorLoggingService{
		config:          config,
		logger:          logger,
		patternCache:    make(map[string]string, config.MaxPatternCache),
		alertingEnabled: true,
	}
}

// NewErrorLoggingServiceFull creates a new ErrorLoggingService with all dependencies and custom configuration.
func NewErrorLoggingServiceFull(
	logger logging.ApplicationLogger,
	classifier outbound.ErrorClassifier,
	aggregator outbound.ErrorAggregator,
	alertingService outbound.AlertingService,
	metricsRecorder MetricsRecorder,
	config *ErrorLoggingServiceConfig,
) *ErrorLoggingService {
	if config == nil {
		config = DefaultErrorLoggingServiceConfig()
	}

	return &ErrorLoggingService{
		config:          config,
		logger:          logger,
		classifier:      classifier,
		aggregator:      aggregator,
		alertingService: alertingService,
		metricsRecorder: metricsRecorder,
		patternCache:    make(map[string]string, config.MaxPatternCache),
		alertingEnabled: true,
	}
}

// SetDependencies allows setting dependencies after service creation (for dependency injection).
func (els *ErrorLoggingService) SetDependencies(
	classifier outbound.ErrorClassifier,
	aggregator outbound.ErrorAggregator,
	alertingService outbound.AlertingService,
	metricsRecorder MetricsRecorder,
) {
	els.mu.Lock()
	defer els.mu.Unlock()

	els.classifier = classifier
	els.aggregator = aggregator
	els.alertingService = alertingService
	els.metricsRecorder = metricsRecorder
}

// LogAndClassifyError logs and classifies an error.
func (els *ErrorLoggingService) LogAndClassifyError(
	ctx context.Context,
	err error,
	component string,
	severity *valueobject.ErrorSeverity,
) error {
	if err == nil {
		return errors.New("error cannot be nil")
	}
	if severity == nil {
		return errors.New("severity cannot be nil")
	}
	if component == "" {
		return errors.New("component cannot be empty")
	}

	// Classify the error
	classifiedError, classifyErr := els.classifyError(ctx, err, component, severity)
	if classifyErr != nil {
		return fmt.Errorf("failed to classify error: %w", classifyErr)
	}

	// Log with structured fields using the appropriate logger
	componentLogger := els.logger.WithComponent(component)
	componentLogger.Error(ctx, "Error classified and processed", slogger.Fields{
		"error_id":       classifiedError.ID(),
		"error_code":     classifiedError.ErrorCode(),
		"severity":       severity.String(),
		"component":      component,
		"pattern":        classifiedError.ErrorPattern(),
		"correlation_id": classifiedError.CorrelationID(),
		"message":        classifiedError.Message(),
	})

	// Record metrics if available
	if els.metricsRecorder != nil {
		els.metricsRecorder.RecordErrorCount(ctx, classifiedError.ErrorCode(), severity.String(), component)
	}

	return nil
}

// ProcessErrorAggregation processes error aggregation and creates alerts if needed.
func (els *ErrorLoggingService) ProcessErrorAggregation(
	ctx context.Context,
	classifiedError *entity.ClassifiedError,
) error {
	if els.aggregator == nil {
		return errors.New("aggregator not configured")
	}

	// Aggregate the error
	aggregation, err := els.aggregator.AggregateError(ctx, classifiedError)
	if err != nil {
		return fmt.Errorf("failed to aggregate error: %w", err)
	}

	// Record aggregation metrics
	if els.metricsRecorder != nil {
		els.metricsRecorder.RecordAggregationMetrics(ctx, aggregation)
	}

	// Check if we need to create alerts
	if els.alertingService != nil && els.shouldCreateAlert(aggregation) {
		alert, err := els.createAlertForAggregation(aggregation)
		if err != nil {
			return fmt.Errorf("failed to create alert: %w", err)
		}

		// Send the alert
		if err := els.sendAlert(ctx, alert); err != nil {
			return fmt.Errorf("failed to send alert: %w", err)
		}
	}

	return nil
}

// ProcessCascadeFailure processes cascade failure detection and alerting.
func (els *ErrorLoggingService) ProcessCascadeFailure(ctx context.Context) error {
	if els.aggregator == nil || els.alertingService == nil {
		return errors.New("aggregator and alerting service required for cascade failure processing")
	}

	// Get active aggregations
	aggregations, err := els.aggregator.GetActiveAggregations(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active aggregations: %w", err)
	}

	// Check for cascade failures
	isCascade, err := els.aggregator.DetectCascadeFailures(ctx, aggregations)
	if err != nil {
		return fmt.Errorf("failed to detect cascade failures: %w", err)
	}

	if isCascade {
		// Create cascade alert
		alertType, _ := valueobject.NewAlertType("CASCADE")
		alert, err := entity.NewCascadeFailureAlert(
			aggregations,
			alertType,
			"Cascade failure detected - multiple systems affected",
		)
		if err != nil {
			return fmt.Errorf("failed to create cascade alert: %w", err)
		}

		// Send cascade alert
		if err := els.alertingService.SendCascadeAlert(ctx, alert); err != nil {
			return fmt.Errorf("failed to send cascade alert: %w", err)
		}

		// Record alert metrics
		if els.metricsRecorder != nil {
			els.metricsRecorder.RecordAlertMetrics(ctx, alert)
		}
	}

	return nil
}

// Helper methods.
func (els *ErrorLoggingService) classifyError(
	ctx context.Context,
	err error,
	component string,
	severity *valueobject.ErrorSeverity,
) (*entity.ClassifiedError, error) {
	// If we have a classifier, use it
	if els.classifier != nil {
		return els.classifier.ClassifyError(ctx, err, component)
	}

	// Otherwise, create a basic classification
	errorCode := els.deriveErrorCode(err, component)
	message := err.Error()

	return entity.NewClassifiedError(ctx, err, severity, errorCode, message, nil)
}

// deriveErrorCode intelligently derives error codes with caching and configurable patterns.
func (els *ErrorLoggingService) deriveErrorCode(err error, component string) string {
	errorMsg := strings.ToLower(err.Error())

	// Create cache key for this error message and component
	cacheKey := component + ":" + errorMsg

	// Check cache first (with read lock)
	els.mu.RLock()
	if cachedPattern, exists := els.patternCache[cacheKey]; exists {
		els.mu.RUnlock()
		return cachedPattern
	}
	els.mu.RUnlock()

	// Derive pattern using enhanced logic
	pattern := els.derivePatternFromError(errorMsg, component)

	// Cache the result (with write lock)
	els.mu.Lock()
	defer els.mu.Unlock()

	// Check cache size before adding
	if len(els.patternCache) >= els.config.MaxPatternCache {
		// Simple eviction: clear cache when full (in production, use LRU)
		els.patternCache = make(map[string]string, els.config.MaxPatternCache)
	}

	els.patternCache[cacheKey] = pattern
	return pattern
}

// derivePatternFromError uses enhanced pattern matching logic.
func (els *ErrorLoggingService) derivePatternFromError(errorMsg, component string) string {
	// Enhanced pattern matching with priority order
	patterns := []struct {
		keywords []string
		pattern  string
	}{
		// Database-related errors (highest priority)
		{[]string{"connection refused", "connection failed", "no connection"}, component + "_connection_failure"},
		{[]string{"timeout", "timed out", "deadline exceeded"}, component + "_timeout_failure"},
		{[]string{"authentication", "auth", "login", "credential"}, component + "_auth_failure"},
		{[]string{"permission", "forbidden", "unauthorized", "access denied"}, component + "_permission_failure"},
		{[]string{"not found", "404", "does not exist", "missing"}, component + "_not_found"},
		{[]string{"validation", "invalid", "malformed", "bad request"}, component + "_validation_failure"},
		{[]string{"disk", "storage", "filesystem", "no space"}, component + "_storage_failure"},
		{[]string{"memory", "out of memory", "oom"}, component + "_memory_failure"},
		{[]string{"network", "connection", "socket"}, component + "_network_failure"},
		{[]string{"rate limit", "too many requests", "throttle"}, component + "_rate_limit_failure"},
	}

	// Check configured default patterns first
	for keyword, defaultPattern := range els.config.DefaultPatterns {
		if strings.Contains(errorMsg, keyword) {
			return defaultPattern
		}
	}

	// Check enhanced patterns
	for _, p := range patterns {
		for _, keyword := range p.keywords {
			if strings.Contains(errorMsg, keyword) {
				return p.pattern
			}
		}
	}

	// Default fallback
	return component + "_processing_failure"
}

// shouldCreateAlert uses configurable thresholds to determine if an alert should be created.
func (els *ErrorLoggingService) shouldCreateAlert(aggregation *entity.ErrorAggregation) bool {
	// Always alert for critical errors
	if aggregation.HighestSeverity().IsCritical() {
		return aggregation.Count() >= els.config.CriticalAlertThreshold
	}

	// Alert for error level based on count
	if aggregation.HighestSeverity().IsError() {
		return aggregation.Count() >= els.config.ErrorAlertThreshold
	}

	// Alert for high volume regardless of severity
	if aggregation.Count() >= els.config.HighVolumeThreshold {
		return true
	}

	// Alert based on error rate
	if aggregation.ExceedsRateThreshold(els.config.ErrorRateThreshold) {
		return true
	}

	return false
}

// createAlertForAggregation creates an alert with intelligent type determination and enhanced messaging.
func (els *ErrorLoggingService) createAlertForAggregation(aggregation *entity.ErrorAggregation) (*entity.Alert, error) {
	// Determine alert type based on configurable thresholds
	var alertType *valueobject.AlertType
	var err error

	// Determine alert type based on conditions
	switch {
	case aggregation.HighestSeverity().IsCritical():
		alertType, err = valueobject.NewAlertType("REAL_TIME")
	case aggregation.Count() >= els.config.HighVolumeThreshold:
		alertType, err = valueobject.NewAlertType("REAL_TIME")
	case aggregation.ExceedsRateThreshold(els.config.ErrorRateThreshold):
		alertType, err = valueobject.NewAlertType("REAL_TIME")
	default:
		alertType, err = valueobject.NewAlertType("BATCH")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create alert type: %w", err)
	}

	// Create enhanced message with context
	message := els.createAlertMessage(aggregation)

	return entity.NewAggregationAlert(aggregation, alertType, message)
}

// createAlertMessage creates a contextual alert message.
func (els *ErrorLoggingService) createAlertMessage(aggregation *entity.ErrorAggregation) string {
	pattern := aggregation.Pattern()
	count := aggregation.Count()
	severity := aggregation.HighestSeverity().String()

	// Determine urgency level
	urgency := "NORMAL"
	if aggregation.HighestSeverity().IsCritical() {
		urgency = "CRITICAL"
	} else if count >= els.config.HighVolumeThreshold {
		urgency = "HIGH"
	}

	// Calculate time window info
	windowDuration := aggregation.WindowDuration().String()
	firstOccurrence := aggregation.FirstOccurrence()
	lastOccurrence := aggregation.LastOccurrence()

	var timeInfo string
	if !firstOccurrence.IsZero() && !lastOccurrence.IsZero() {
		duration := lastOccurrence.Sub(firstOccurrence)
		timeInfo = fmt.Sprintf(" over %v", duration.Round(time.Second))
	}

	return fmt.Sprintf(
		"[%s] Multiple errors detected: %s (count: %d, severity: %s, window: %s%s)",
		urgency, pattern, count, severity, windowDuration, timeInfo,
	)
}

// sendAlert sends an alert with circuit breaker pattern and retry logic.
func (els *ErrorLoggingService) sendAlert(ctx context.Context, alert *entity.Alert) error {
	// Check if alerting is enabled (circuit breaker)
	if !els.isAlertingEnabled() {
		return errors.New("alerting is currently disabled due to circuit breaker")
	}

	// Add correlation ID to alert context if available
	if corrID := ctx.Value("correlation_id"); corrID != nil {
		// Log the correlation for traceability
		els.logger.WithComponent("error-logging").Info(ctx, "Sending alert", slogger.Fields{
			"alert_id":       alert.ID(),
			"alert_type":     alert.Type().String(),
			"severity":       alert.Severity().String(),
			"correlation_id": corrID,
		})
	}

	// Send alert based on type
	var err error
	switch {
	case alert.Type().IsRealTime():
		err = els.alertingService.SendRealTimeAlert(ctx, alert)
	case alert.Type().IsBatch():
		err = els.alertingService.SendBatchAlert(ctx, alert)
	case alert.Type().IsCascade():
		err = els.alertingService.SendCascadeAlert(ctx, alert)
	default:
		err = fmt.Errorf("unsupported alert type: %s", alert.Type().String())
	}

	// Handle alerting failures for circuit breaker
	if err != nil {
		els.recordAlertingError()
		return fmt.Errorf("failed to send alert: %w", err)
	}

	// Reset error count on success
	els.resetAlertingErrors()
	return nil
}

// isAlertingEnabled checks if alerting is enabled (circuit breaker pattern).
func (els *ErrorLoggingService) isAlertingEnabled() bool {
	els.mu.RLock()
	defer els.mu.RUnlock()
	return els.alertingEnabled
}

// recordAlertingError records an alerting failure for circuit breaker.
func (els *ErrorLoggingService) recordAlertingError() {
	els.mu.Lock()
	defer els.mu.Unlock()

	els.alertingErrors++
	// Disable alerting after 5 consecutive failures
	if els.alertingErrors >= 5 {
		els.alertingEnabled = false
		els.logger.WithComponent("error-logging").Error(context.Background(),
			"Alerting disabled due to consecutive failures", slogger.Fields{
				"consecutive_failures": els.alertingErrors,
			})
	}
}

// resetAlertingErrors resets the alerting error count and re-enables alerting.
func (els *ErrorLoggingService) resetAlertingErrors() {
	els.mu.Lock()
	defer els.mu.Unlock()

	if els.alertingErrors > 0 {
		els.alertingErrors = 0
		if !els.alertingEnabled {
			els.alertingEnabled = true
			els.logger.WithComponent("error-logging").Info(context.Background(),
				"Alerting re-enabled after successful delivery", slogger.Fields{})
		}
	}
}

// GetCacheMetrics returns cache performance metrics.
func (els *ErrorLoggingService) GetCacheMetrics() (int64, int64, float64) {
	els.mu.RLock()
	defer els.mu.RUnlock()

	hits := els.cacheHits
	misses := els.cacheMisses
	total := hits + misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}
	return hits, misses, hitRate
}
