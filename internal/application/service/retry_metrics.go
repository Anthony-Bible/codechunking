package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// Metric names following OpenTelemetry semantic conventions for retry operations.
const (
	RetryAttemptCounterName           = "retry_attempt_total"
	RetrySuccessCounterName           = "retry_success_total"
	RetryFailureCounterName           = "retry_failure_total"
	RetryExhaustionCounterName        = "retry_exhaustion_total"
	RetryDelayHistogramName           = "retry_delay_duration_seconds"
	RetryTotalDurationHistogramName   = "retry_total_duration_seconds"
	RetryBackoffDelayHistogramName    = "retry_backoff_delay_seconds"
	RetryCircuitBreakerCounterName    = "retry_circuit_breaker_total"
	RetryPolicyChangesCounterName     = "retry_policy_changes_total"
	RetryJitterApplicationCounterName = "retry_jitter_application_total"
	RetryMaxAttemptsGaugeName         = "retry_max_attempts"
	RetryCurrentAttemptGaugeName      = "retry_current_attempt"
)

// Common attribute keys for consistent retry metrics labeling.
const (
	AttrRetryAttempt     = "retry_attempt"
	AttrRetryMaxAttempts = "retry_max_attempts"
	AttrFailureType      = "failure_type"
	AttrRetryPolicyType  = "retry_policy_type"
	AttrBackoffAlgorithm = "backoff_algorithm"
	AttrJitterStrategy   = "jitter_strategy"
	AttrRetryResult      = "retry_result"
	AttrCircuitState     = "circuit_state"
	AttrOperationName    = "operation_name"
	AttrServiceName      = "service_name"
	AttrRetryReason      = "retry_reason"
	AttrRetryExhausted   = "retry_exhausted"
)

// RetryMetrics defines the interface for retry observability.
type RetryMetrics interface {
	// RecordRetryAttempt records a retry attempt with context.
	RecordRetryAttempt(ctx context.Context, attempt int, failureType FailureType, operationName string)

	// RecordRetrySuccess records a successful retry outcome.
	RecordRetrySuccess(ctx context.Context, totalAttempts int, totalDuration time.Duration, operationName string)

	// RecordRetryFailure records a failed retry attempt.
	RecordRetryFailure(ctx context.Context, attempt int, failureType FailureType, err error, operationName string)

	// RecordRetryExhaustion records when retry attempts are exhausted.
	RecordRetryExhaustion(
		ctx context.Context,
		maxAttempts int,
		totalDuration time.Duration,
		lastError error,
		operationName string,
	)

	// RecordRetryDelay records the delay before a retry attempt.
	RecordRetryDelay(ctx context.Context, delay time.Duration, attempt int, backoffAlgorithm string)

	// RecordCircuitBreakerEvent records circuit breaker state changes affecting retries.
	RecordCircuitBreakerEvent(ctx context.Context, state string, operationName string)

	// RecordPolicyChange records when retry policy is changed during operation.
	RecordPolicyChange(ctx context.Context, oldPolicy, newPolicy string, operationName string)

	// GetInstanceID returns the metrics instance identifier.
	GetInstanceID() string
}

// RetryMetricsConfig holds configuration for retry metrics collection.
type RetryMetricsConfig struct {
	InstanceID          string
	ServiceName         string
	ServiceVersion      string
	EnableDelayBuckets  bool
	CustomDelayBuckets  []float64
	EnableJitterMetrics bool
	MaxAttempts         int
}

// DefaultRetryMetrics implements the RetryMetrics interface using OpenTelemetry.
type DefaultRetryMetrics struct {
	config RetryMetricsConfig
	mu     sync.RWMutex

	// OpenTelemetry instruments
	attemptCounter         metric.Int64Counter
	successCounter         metric.Int64Counter
	failureCounter         metric.Int64Counter
	exhaustionCounter      metric.Int64Counter
	delayHistogram         metric.Float64Histogram
	totalDurationHistogram metric.Float64Histogram
	circuitBreakerCounter  metric.Int64Counter
	policyChangesCounter   metric.Int64Counter
}

// NewRetryMetrics creates a new RetryMetrics instance with default meter provider.
func NewRetryMetrics(config RetryMetricsConfig) (RetryMetrics, error) {
	if config.InstanceID == "" {
		return nil, errors.New("instance ID cannot be empty")
	}
	if config.ServiceName == "" {
		return nil, errors.New("service name cannot be empty")
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", config.ServiceName),
			attribute.String("service.version", config.ServiceVersion),
		),
	)
	if err != nil {
		return nil, err
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewManualReader()),
	)

	return NewRetryMetricsWithProvider(config, provider)
}

// NewRetryMetricsWithProvider creates a new RetryMetrics instance with a custom meter provider.
func NewRetryMetricsWithProvider(config RetryMetricsConfig, provider metric.MeterProvider) (RetryMetrics, error) {
	if config.InstanceID == "" {
		return nil, errors.New("instance ID cannot be empty")
	}
	if config.ServiceName == "" {
		return nil, errors.New("service name cannot be empty")
	}

	meter := provider.Meter("retry-metrics")

	// Create counters
	attemptCounter, err := meter.Int64Counter(RetryAttemptCounterName,
		metric.WithDescription("Total number of retry attempts"),
	)
	if err != nil {
		return nil, err
	}

	successCounter, err := meter.Int64Counter(RetrySuccessCounterName,
		metric.WithDescription("Total number of successful retries"),
	)
	if err != nil {
		return nil, err
	}

	failureCounter, err := meter.Int64Counter(RetryFailureCounterName,
		metric.WithDescription("Total number of failed retry attempts"),
	)
	if err != nil {
		return nil, err
	}

	exhaustionCounter, err := meter.Int64Counter(RetryExhaustionCounterName,
		metric.WithDescription("Total number of retry exhaustions"),
	)
	if err != nil {
		return nil, err
	}

	circuitBreakerCounter, err := meter.Int64Counter(RetryCircuitBreakerCounterName,
		metric.WithDescription("Total number of circuit breaker events"),
	)
	if err != nil {
		return nil, err
	}

	policyChangesCounter, err := meter.Int64Counter(RetryPolicyChangesCounterName,
		metric.WithDescription("Total number of retry policy changes"),
	)
	if err != nil {
		return nil, err
	}

	// Create histograms
	delayHistogramOptions := []metric.Float64HistogramOption{
		metric.WithDescription("Retry delay duration in seconds"),
	}
	if config.EnableDelayBuckets && len(config.CustomDelayBuckets) > 0 {
		delayHistogramOptions = append(delayHistogramOptions,
			metric.WithExplicitBucketBoundaries(config.CustomDelayBuckets...))
	}

	delayHistogram, err := meter.Float64Histogram(RetryDelayHistogramName, delayHistogramOptions...)
	if err != nil {
		return nil, err
	}

	totalDurationHistogram, err := meter.Float64Histogram(RetryTotalDurationHistogramName,
		metric.WithDescription("Total retry operation duration in seconds"),
	)
	if err != nil {
		return nil, err
	}

	return &DefaultRetryMetrics{
		config:                 config,
		attemptCounter:         attemptCounter,
		successCounter:         successCounter,
		failureCounter:         failureCounter,
		exhaustionCounter:      exhaustionCounter,
		delayHistogram:         delayHistogram,
		totalDurationHistogram: totalDurationHistogram,
		circuitBreakerCounter:  circuitBreakerCounter,
		policyChangesCounter:   policyChangesCounter,
	}, nil
}

func (m *DefaultRetryMetrics) RecordRetryAttempt(
	ctx context.Context,
	attempt int,
	failureType FailureType,
	operationName string,
) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if attempt <= 0 {
		return // Handle gracefully as per test requirements
	}

	m.attemptCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.Int(AttrRetryAttempt, attempt),
			attribute.String(AttrFailureType, failureType.String()),
			attribute.String(AttrOperationName, operationName),
			attribute.String(AttrServiceName, m.config.ServiceName),
		),
	)
}

func (m *DefaultRetryMetrics) RecordRetrySuccess(
	ctx context.Context,
	totalAttempts int,
	totalDuration time.Duration,
	operationName string,
) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.successCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.Int(AttrRetryMaxAttempts, totalAttempts),
			attribute.String(AttrOperationName, operationName),
			attribute.String(AttrServiceName, m.config.ServiceName),
		),
	)

	m.totalDurationHistogram.Record(ctx, totalDuration.Seconds(),
		metric.WithAttributes(
			attribute.String(AttrOperationName, operationName),
			attribute.String(AttrRetryResult, "success"),
		),
	)
}

func (m *DefaultRetryMetrics) RecordRetryFailure(
	ctx context.Context,
	attempt int,
	failureType FailureType,
	err error,
	operationName string,
) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.failureCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.Int(AttrRetryAttempt, attempt),
			attribute.String(AttrFailureType, failureType.String()),
			attribute.String(AttrOperationName, operationName),
			attribute.String(AttrServiceName, m.config.ServiceName),
		),
	)
}

func (m *DefaultRetryMetrics) RecordRetryExhaustion(
	ctx context.Context,
	maxAttempts int,
	totalDuration time.Duration,
	lastError error,
	operationName string,
) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.exhaustionCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.Int(AttrRetryMaxAttempts, maxAttempts),
			attribute.String(AttrOperationName, operationName),
			attribute.String(AttrServiceName, m.config.ServiceName),
			attribute.Bool(AttrRetryExhausted, true),
		),
	)

	m.totalDurationHistogram.Record(ctx, totalDuration.Seconds(),
		metric.WithAttributes(
			attribute.String(AttrOperationName, operationName),
			attribute.String(AttrRetryResult, "exhausted"),
		),
	)
}

func (m *DefaultRetryMetrics) RecordRetryDelay(
	ctx context.Context,
	delay time.Duration,
	attempt int,
	backoffAlgorithm string,
) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.delayHistogram.Record(ctx, delay.Seconds(),
		metric.WithAttributes(
			attribute.Int(AttrRetryAttempt, attempt),
			attribute.String(AttrBackoffAlgorithm, backoffAlgorithm),
			attribute.String(AttrServiceName, m.config.ServiceName),
		),
	)
}

func (m *DefaultRetryMetrics) RecordCircuitBreakerEvent(ctx context.Context, state string, operationName string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.circuitBreakerCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String(AttrCircuitState, state),
			attribute.String(AttrOperationName, operationName),
			attribute.String(AttrServiceName, m.config.ServiceName),
		),
	)
}

func (m *DefaultRetryMetrics) RecordPolicyChange(
	ctx context.Context,
	oldPolicy, newPolicy string,
	operationName string,
) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.policyChangesCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("old_policy", oldPolicy),
			attribute.String("new_policy", newPolicy),
			attribute.String(AttrOperationName, operationName),
			attribute.String(AttrServiceName, m.config.ServiceName),
		),
	)
}

func (m *DefaultRetryMetrics) GetInstanceID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.InstanceID
}
