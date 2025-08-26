// Package worker provides OpenTelemetry metrics integration for acknowledgment operations.
// This module implements enterprise-grade metrics collection using OpenTelemetry histograms
// and counters for comprehensive observability of message acknowledgment patterns.
package worker

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metric names following OpenTelemetry semantic conventions.
const (
	AckDurationHistogramName = "worker_ack_duration_seconds"
	AckCounterName           = "worker_ack_total"
	NackCounterName          = "worker_nack_total"
	DuplicateCounterName     = "worker_duplicate_messages_total"
	RetryHistogramName       = "worker_ack_retry_duration_seconds"
	TimeoutCounterName       = "worker_ack_timeout_total"
)

// Common attribute keys for consistent labeling.
const (
	AttrAckResult     = "ack_result"     // success, failure, timeout
	AttrFailureType   = "failure_type"   // network, validation, processing
	AttrRetryAttempt  = "retry_attempt"  // 1, 2, 3, etc.
	AttrMessageType   = "message_type"   // indexing_job, retry, dlq
	AttrCorrelationID = "correlation_id" // for distributed tracing
	AttrWorkerID      = "worker_id"      // worker instance identifier
	AttrBatchSize     = "batch_size"     // number of messages in batch operation
)

// AckMetrics provides OpenTelemetry-based metrics collection for acknowledgment operations.
type AckMetrics struct {
	// Histograms for timing measurements
	ackDuration   metric.Float64Histogram
	retryDuration metric.Float64Histogram

	// Counters for event counting
	ackTotal       metric.Int64Counter
	nackTotal      metric.Int64Counter
	duplicateTotal metric.Int64Counter
	timeoutTotal   metric.Int64Counter

	// Worker identification for consistent labeling
	workerID string
}

// NewAckMetrics creates a new OpenTelemetry metrics collector for acknowledgment operations.
func NewAckMetrics(workerID string) (*AckMetrics, error) {
	meter := otel.Meter("codechunking/worker", metric.WithInstrumentationVersion("1.0.0"))

	// Histogram bucket boundaries following OpenTelemetry best practices for latency measurements.
	// ackLatencyBuckets defines bucket boundaries for acknowledgment operation latencies.
	// Optimized for fast message processing operations (1ms to 10s range).
	// Covers typical acknowledgment times from sub-millisecond to several seconds.
	ackLatencyBuckets := []float64{
		0.001, // 1ms
		0.005, // 5ms
		0.01,  // 10ms
		0.025, // 25ms
		0.05,  // 50ms
		0.1,   // 100ms
		0.25,  // 250ms
		0.5,   // 500ms
		1.0,   // 1s
		2.5,   // 2.5s
		5.0,   // 5s
		10.0,  // 10s
	}

	// retryLatencyBuckets defines bucket boundaries for retry operation latencies.
	// Optimized for retry operations including backoff delays (100ms to 2min range).
	// Covers typical retry times from quick retries to extended backoff periods.
	retryLatencyBuckets := []float64{
		0.1,   // 100ms
		0.5,   // 500ms
		1.0,   // 1s
		2.0,   // 2s
		5.0,   // 5s
		10.0,  // 10s
		30.0,  // 30s
		60.0,  // 1min
		120.0, // 2min
	}

	// Create acknowledgment duration histogram with appropriate buckets for message processing
	ackDuration, err := meter.Float64Histogram(
		AckDurationHistogramName,
		metric.WithDescription("Duration of acknowledgment operations in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(ackLatencyBuckets...),
	)
	if err != nil {
		return nil, err
	}

	// Create retry duration histogram
	retryDuration, err := meter.Float64Histogram(
		RetryHistogramName,
		metric.WithDescription("Duration of acknowledgment retry operations in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(retryLatencyBuckets...),
	)
	if err != nil {
		return nil, err
	}

	// Create acknowledgment success counter
	ackTotal, err := meter.Int64Counter(
		AckCounterName,
		metric.WithDescription("Total number of message acknowledgments"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	// Create negative acknowledgment counter
	nackTotal, err := meter.Int64Counter(
		NackCounterName,
		metric.WithDescription("Total number of negative acknowledgments"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	// Create duplicate message counter
	duplicateTotal, err := meter.Int64Counter(
		DuplicateCounterName,
		metric.WithDescription("Total number of duplicate messages detected"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	// Create timeout counter
	timeoutTotal, err := meter.Int64Counter(
		TimeoutCounterName,
		metric.WithDescription("Total number of acknowledgment timeouts"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	return &AckMetrics{
		ackDuration:    ackDuration,
		retryDuration:  retryDuration,
		ackTotal:       ackTotal,
		nackTotal:      nackTotal,
		duplicateTotal: duplicateTotal,
		timeoutTotal:   timeoutTotal,
		workerID:       workerID,
	}, nil
}

// RecordAckSuccess records a successful acknowledgment with timing information.
func (m *AckMetrics) RecordAckSuccess(ctx context.Context, duration time.Duration, correlationID, messageType string) {
	attributes := []attribute.KeyValue{
		attribute.String(AttrAckResult, "success"),
		attribute.String(AttrCorrelationID, correlationID),
		attribute.String(AttrMessageType, messageType),
		attribute.String(AttrWorkerID, m.workerID),
	}

	m.ackDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
	m.ackTotal.Add(ctx, 1, metric.WithAttributes(attributes...))
}

// RecordAckFailure records a failed acknowledgment with failure details.
func (m *AckMetrics) RecordAckFailure(
	ctx context.Context,
	duration time.Duration,
	failureType, correlationID, messageType string,
) {
	attributes := []attribute.KeyValue{
		attribute.String(AttrAckResult, "failure"),
		attribute.String(AttrFailureType, failureType),
		attribute.String(AttrCorrelationID, correlationID),
		attribute.String(AttrMessageType, messageType),
		attribute.String(AttrWorkerID, m.workerID),
	}

	m.ackDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
	m.ackTotal.Add(ctx, 1, metric.WithAttributes(attributes...))
}

// RecordNack records a negative acknowledgment with retry information.
func (m *AckMetrics) RecordNack(
	ctx context.Context,
	duration time.Duration,
	failureType, correlationID, messageType string,
	retryAttempt int,
) {
	attributes := []attribute.KeyValue{
		attribute.String(AttrFailureType, failureType),
		attribute.String(AttrCorrelationID, correlationID),
		attribute.String(AttrMessageType, messageType),
		attribute.Int(AttrRetryAttempt, retryAttempt),
		attribute.String(AttrWorkerID, m.workerID),
	}

	m.ackDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
	m.nackTotal.Add(ctx, 1, metric.WithAttributes(attributes...))
}

// RecordDuplicateMessage records detection of a duplicate message.
func (m *AckMetrics) RecordDuplicateMessage(ctx context.Context, correlationID, messageType string) {
	attributes := []attribute.KeyValue{
		attribute.String(AttrCorrelationID, correlationID),
		attribute.String(AttrMessageType, messageType),
		attribute.String(AttrWorkerID, m.workerID),
	}

	m.duplicateTotal.Add(ctx, 1, metric.WithAttributes(attributes...))
}

// RecordAckTimeout records an acknowledgment timeout event.
func (m *AckMetrics) RecordAckTimeout(ctx context.Context, correlationID, messageType string) {
	attributes := []attribute.KeyValue{
		attribute.String(AttrCorrelationID, correlationID),
		attribute.String(AttrMessageType, messageType),
		attribute.String(AttrWorkerID, m.workerID),
	}

	m.timeoutTotal.Add(ctx, 1, metric.WithAttributes(attributes...))
}

// RecordRetryAttempt records timing information for acknowledgment retry operations.
func (m *AckMetrics) RecordRetryAttempt(
	ctx context.Context,
	duration time.Duration,
	retryAttempt int,
	correlationID, messageType string,
) {
	attributes := []attribute.KeyValue{
		attribute.Int(AttrRetryAttempt, retryAttempt),
		attribute.String(AttrCorrelationID, correlationID),
		attribute.String(AttrMessageType, messageType),
		attribute.String(AttrWorkerID, m.workerID),
	}

	m.retryDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
}

// RecordBatchAckOperation records metrics for batch acknowledgment operations.
func (m *AckMetrics) RecordBatchAckOperation(
	ctx context.Context,
	batchSize int,
	duration time.Duration,
	successCount int,
	failureCount int,
) {
	baseAttributes := []attribute.KeyValue{
		attribute.String(AttrWorkerID, m.workerID),
		attribute.String(AttrMessageType, "batch"),
		attribute.Int(AttrBatchSize, batchSize),
	}

	// Record overall batch timing
	m.ackDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(
		append(baseAttributes, attribute.String(AttrAckResult, "batch"))...,
	))

	// Record successful acknowledgments
	if successCount > 0 {
		m.ackTotal.Add(ctx, int64(successCount), metric.WithAttributes(
			append(baseAttributes, attribute.String(AttrAckResult, "success"))...,
		))
	}

	// Record failed acknowledgments
	if failureCount > 0 {
		m.ackTotal.Add(ctx, int64(failureCount), metric.WithAttributes(
			append(baseAttributes, attribute.String(AttrAckResult, "failure"))...,
		))
	}
}
