// Package service provides OpenTelemetry metrics integration for disk operations.
// This module implements enterprise-grade metrics collection using OpenTelemetry histograms,
// counters, and gauges for comprehensive observability of disk management operations.
package service

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metric names following OpenTelemetry semantic conventions.
const (
	DiskUsageGaugeName                    = "disk_usage_bytes"
	DiskOperationDurationHistogramName    = "disk_operation_duration_seconds"
	DiskOperationCounterName              = "disk_operation_total"
	DiskAlertCounterName                  = "disk_alert_total"
	DiskHealthCheckDurationHistogramName  = "disk_health_check_duration_seconds"
	DiskHealthStatusGaugeName             = "disk_health_status"
	CleanupOperationDurationHistogramName = "disk_cleanup_operation_duration_seconds"
	CleanupItemsCounterName               = "disk_cleanup_items_total"
	CleanupBytesFreedCounterName          = "disk_cleanup_bytes_freed_bytes_total"
	RetentionPolicyDurationHistogramName  = "disk_retention_policy_duration_seconds"
	RetentionPolicyCounterName            = "disk_retention_policy_total"
)

// Common attribute keys for consistent labeling.
const (
	AttrDiskPath         = "disk_path"
	AttrOperationType    = "operation_type"
	AttrDiskOperation    = "disk_operation"
	AttrOperationResult  = "operation_result"
	AttrAlertLevel       = "alert_level"
	AttrAlertType        = "alert_type"
	AttrCorrelationID    = "correlation_id"
	AttrInstanceID       = "instance_id"
	AttrOverallHealth    = "overall_health"
	AttrPathsChecked     = "paths_checked"
	AttrActiveAlerts     = "active_alerts"
	AttrCleanupStrategy  = "cleanup_strategy"
	AttrDryRun           = "dry_run"
	AttrPolicyOperation  = "policy_operation"
	AttrPolicyID         = "policy_id"
	AttrComplianceStatus = "compliance_status"
)

// getDiskOperationLatencyBuckets returns bucket boundaries for disk operation latencies.
// Optimized for fast disk operations (1ms to 10s range).
func getDiskOperationLatencyBuckets() []float64 {
	return []float64{
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
}

// getCleanupDurationBuckets returns bucket boundaries for cleanup operation latencies.
// Optimized for long-running cleanup operations (1s to 2hr range).
func getCleanupDurationBuckets() []float64 {
	return []float64{
		1.0,    // 1s
		5.0,    // 5s
		10.0,   // 10s
		60.0,   // 1min
		1200.0, // 20min - bucket index 4
		1800.0, // 30min
		3600.0, // 1hr
		7200.0, // 2hr
	}
}

// getHealthCheckLatencyBuckets returns bucket boundaries for health check latencies.
// Optimized for moderate health check operations (10ms to 30s range).
func getHealthCheckLatencyBuckets() []float64 {
	return []float64{
		0.01, // 10ms
		0.05, // 50ms
		0.1,  // 100ms
		1.0,  // 1s - bucket index 3
		2.5,  // 2.5s
		5.0,  // 5s
		10.0, // 10s
		30.0, // 30s
	}
}

// getPolicyOperationDurationBuckets returns bucket boundaries for policy operation latencies.
// Optimized for policy operations (30s to 1hr range).
func getPolicyOperationDurationBuckets() []float64 {
	return []float64{
		30.0,   // 30s
		60.0,   // 1min
		120.0,  // 2min
		600.0,  // 10min - bucket index 3
		900.0,  // 15min
		1200.0, // 20min
		1800.0, // 30min
		3600.0, // 1hr
	}
}

// DiskMetrics provides OpenTelemetry-based metrics collection for disk operations.
type DiskMetrics struct {
	// Gauges for current state measurements
	diskUsageGauge        metric.Float64Gauge
	diskHealthStatusGauge metric.Int64Gauge

	// Histograms for timing measurements
	diskOperationDuration    metric.Float64Histogram
	diskHealthCheckDuration  metric.Float64Histogram
	cleanupOperationDuration metric.Float64Histogram
	retentionPolicyDuration  metric.Float64Histogram

	// Counters for event counting
	diskOperationCounter     metric.Int64Counter
	diskAlertCounter         metric.Int64Counter
	cleanupItemsCounter      metric.Int64Counter
	cleanupBytesFreedCounter metric.Int64Counter
	retentionPolicyCounter   metric.Int64Counter

	// Instance identification for consistent labeling
	instanceID string

	// Cached attributes for performance optimization
	baseInstanceAttr attribute.KeyValue
}

// instrumentCreator provides helpers for creating metric instruments with consistent patterns.
type instrumentCreator struct {
	meter metric.Meter
}

// newInstrumentCreator creates a new instrument creator helper.
// This helper encapsulates the meter and provides convenient methods for creating
// different types of metric instruments with consistent options.
func newInstrumentCreator(meter metric.Meter) *instrumentCreator {
	return &instrumentCreator{meter: meter}
}

// createFloat64Gauge creates a Float64Gauge instrument with the specified configuration.
// Float64Gauge instruments are used for measurements that represent a value at a point in time.
func (ic *instrumentCreator) createFloat64Gauge(name, description, unit string) (metric.Float64Gauge, error) {
	return ic.meter.Float64Gauge(name, metric.WithDescription(description), metric.WithUnit(unit))
}

// createInt64Gauge creates an Int64Gauge instrument with the specified configuration.
// Int64Gauge instruments are used for discrete measurements at a point in time.
func (ic *instrumentCreator) createInt64Gauge(name, description, unit string) (metric.Int64Gauge, error) {
	return ic.meter.Int64Gauge(name, metric.WithDescription(description), metric.WithUnit(unit))
}

// createFloat64Histogram creates a Float64Histogram instrument with explicit bucket boundaries.
// Histogram instruments are used to measure distributions of values over time.
// All histograms use seconds as the unit.
func (ic *instrumentCreator) createFloat64Histogram(
	name, description string,
	buckets []float64,
) (metric.Float64Histogram, error) {
	return ic.meter.Float64Histogram(
		name,
		metric.WithDescription(description),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(buckets...),
	)
}

// createInt64Counter creates an Int64Counter instrument with the specified configuration.
// Counter instruments are used to measure cumulative totals that only increase over time.
func (ic *instrumentCreator) createInt64Counter(name, description, unit string) (metric.Int64Counter, error) {
	return ic.meter.Int64Counter(name, metric.WithDescription(description), metric.WithUnit(unit))
}

// createGauges creates all gauge instruments in a single batch operation.
// Returns both Float64Gauge and Int64Gauge or the first error encountered.
func (ic *instrumentCreator) createGauges() (metric.Float64Gauge, metric.Int64Gauge, error) {
	diskUsageGauge, err := ic.createFloat64Gauge(
		DiskUsageGaugeName,
		"Current disk usage percentage",
		"%",
	)
	if err != nil {
		return nil, nil, err
	}

	diskHealthStatusGauge, err := ic.createInt64Gauge(
		DiskHealthStatusGaugeName,
		"Current disk health status (0=good, 1=warning, 2=critical, 3=failed)",
		"1",
	)
	if err != nil {
		return nil, nil, err
	}

	return diskUsageGauge, diskHealthStatusGauge, nil
}

// createHistograms creates all histogram instruments in a single batch operation.
// Returns all 4 Float64Histogram instruments or the first error encountered.
func (ic *instrumentCreator) createHistograms() (
	metric.Float64Histogram, metric.Float64Histogram, metric.Float64Histogram, metric.Float64Histogram, error,
) {
	diskOperationDuration, err := ic.createFloat64Histogram(
		DiskOperationDurationHistogramName,
		"Duration of disk operations in seconds",
		getDiskOperationLatencyBuckets(),
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	diskHealthCheckDuration, err := ic.createFloat64Histogram(
		DiskHealthCheckDurationHistogramName,
		"Duration of disk health check operations in seconds",
		getHealthCheckLatencyBuckets(),
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	cleanupOperationDuration, err := ic.createFloat64Histogram(
		CleanupOperationDurationHistogramName,
		"Duration of disk cleanup operations in seconds",
		getCleanupDurationBuckets(),
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	retentionPolicyDuration, err := ic.createFloat64Histogram(
		RetentionPolicyDurationHistogramName,
		"Duration of retention policy operations in seconds",
		getPolicyOperationDurationBuckets(),
	)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return diskOperationDuration, diskHealthCheckDuration, cleanupOperationDuration, retentionPolicyDuration, nil
}

// createCounters creates all counter instruments in a single batch operation.
// Returns all 5 Int64Counter instruments or the first error encountered.
func (ic *instrumentCreator) createCounters() (
	metric.Int64Counter, metric.Int64Counter, metric.Int64Counter, metric.Int64Counter, metric.Int64Counter, error,
) {
	diskOperationCounter, err := ic.createInt64Counter(
		DiskOperationCounterName,
		"Total number of disk operations",
		"1",
	)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	diskAlertCounter, err := ic.createInt64Counter(
		DiskAlertCounterName,
		"Total number of disk alerts",
		"1",
	)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	cleanupItemsCounter, err := ic.createInt64Counter(
		CleanupItemsCounterName,
		"Total number of items cleaned up",
		"1",
	)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	cleanupBytesFreedCounter, err := ic.createInt64Counter(
		CleanupBytesFreedCounterName,
		"Total bytes freed by cleanup operations",
		"By",
	)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	retentionPolicyCounter, err := ic.createInt64Counter(
		RetentionPolicyCounterName,
		"Total number of retention policy operations",
		"1",
	)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	return diskOperationCounter, diskAlertCounter, cleanupItemsCounter, cleanupBytesFreedCounter, retentionPolicyCounter, nil
}

// NewDiskMetrics creates a new OpenTelemetry metrics collector for disk operations
// using the default global meter provider.
func NewDiskMetrics(instanceID string) (*DiskMetrics, error) {
	if instanceID == "" {
		return nil, errors.New("instance ID cannot be empty")
	}
	return NewDiskMetricsWithProvider(instanceID, otel.GetMeterProvider())
}

// NewDiskMetricsWithProvider creates a new OpenTelemetry metrics collector for disk operations
// with a specific meter provider.
func NewDiskMetricsWithProvider(instanceID string, provider metric.MeterProvider) (*DiskMetrics, error) {
	if instanceID == "" {
		return nil, errors.New("instance ID cannot be empty")
	}

	meter := provider.Meter("codechunking/service", metric.WithInstrumentationVersion("1.0.0"))
	creator := newInstrumentCreator(meter)

	// Create all instruments in batches
	diskUsageGauge, diskHealthStatusGauge, err := creator.createGauges()
	if err != nil {
		return nil, err
	}

	diskOperationDuration, diskHealthCheckDuration, cleanupOperationDuration, retentionPolicyDuration, err := creator.createHistograms()
	if err != nil {
		return nil, err
	}

	diskOperationCounter, diskAlertCounter, cleanupItemsCounter, cleanupBytesFreedCounter, retentionPolicyCounter, err := creator.createCounters()
	if err != nil {
		return nil, err
	}

	return &DiskMetrics{
		diskUsageGauge:           diskUsageGauge,
		diskHealthStatusGauge:    diskHealthStatusGauge,
		diskOperationDuration:    diskOperationDuration,
		diskHealthCheckDuration:  diskHealthCheckDuration,
		cleanupOperationDuration: cleanupOperationDuration,
		retentionPolicyDuration:  retentionPolicyDuration,
		diskOperationCounter:     diskOperationCounter,
		diskAlertCounter:         diskAlertCounter,
		cleanupItemsCounter:      cleanupItemsCounter,
		cleanupBytesFreedCounter: cleanupBytesFreedCounter,
		retentionPolicyCounter:   retentionPolicyCounter,
		instanceID:               instanceID,
		baseInstanceAttr:         attribute.String(AttrInstanceID, instanceID),
	}, nil
}

// buildDiskUsageAttrs builds optimized attribute slice for disk usage metrics.
// This method reuses the cached instance attribute to improve performance.
func (m *DiskMetrics) buildDiskUsageAttrs(path, operationType string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrDiskPath, path),
		attribute.String(AttrOperationType, operationType),
		m.baseInstanceAttr, // Reuse cached attribute
	}
}

// buildDiskOperationAttrs builds optimized attribute slice for disk operation metrics.
// Includes correlation ID for distributed tracing and reuses cached instance attribute.
func (m *DiskMetrics) buildDiskOperationAttrs(operation, path, result, correlationID string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrDiskOperation, operation),
		attribute.String(AttrDiskPath, path),
		attribute.String(AttrOperationResult, result),
		attribute.String(AttrCorrelationID, correlationID),
		m.baseInstanceAttr, // Reuse cached attribute
	}
}

// buildDiskAlertAttrs builds optimized attribute slice for disk alert metrics.
// Includes alert severity and type information for better observability.
func (m *DiskMetrics) buildDiskAlertAttrs(path, alertLevel, alertType, correlationID string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrDiskPath, path),
		attribute.String(AttrAlertLevel, alertLevel),
		attribute.String(AttrAlertType, alertType),
		attribute.String(AttrCorrelationID, correlationID),
		m.baseInstanceAttr, // Reuse cached attribute
	}
}

// buildCleanupOperationAttrs builds optimized attribute slice for cleanup operation metrics.
// Includes strategy, dry-run flag, and operation result for comprehensive tracking.
func (m *DiskMetrics) buildCleanupOperationAttrs(
	strategy, path, result, correlationID string,
	dryRun bool,
) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrCleanupStrategy, strategy),
		attribute.String(AttrDiskPath, path),
		attribute.Bool(AttrDryRun, dryRun),
		attribute.String(AttrOperationResult, result),
		attribute.String(AttrCorrelationID, correlationID),
		m.baseInstanceAttr, // Reuse cached attribute
	}
}

// buildRetentionPolicyAttrs builds optimized attribute slice for retention policy metrics.
// Includes policy ID, compliance status, and operation details for policy tracking.
func (m *DiskMetrics) buildRetentionPolicyAttrs(
	operation, policyID, path, complianceStatus, result, correlationID string,
	dryRun bool,
) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrPolicyOperation, operation),
		attribute.String(AttrPolicyID, policyID),
		attribute.String(AttrDiskPath, path),
		attribute.String(AttrComplianceStatus, complianceStatus),
		attribute.Bool(AttrDryRun, dryRun),
		attribute.String(AttrOperationResult, result),
		attribute.String(AttrCorrelationID, correlationID),
		m.baseInstanceAttr, // Reuse cached attribute
	}
}

// buildHealthCheckAttrs builds optimized attribute slice for health check metrics.
// Includes overall health status and alert counts for system monitoring.
func (m *DiskMetrics) buildHealthCheckAttrs(
	overallHealth, correlationID string,
	pathsChecked, activeAlerts int,
) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrOverallHealth, overallHealth),
		attribute.Int(AttrPathsChecked, pathsChecked),
		attribute.Int(AttrActiveAlerts, activeAlerts),
		attribute.String(AttrCorrelationID, correlationID),
		m.baseInstanceAttr, // Reuse cached attribute
	}
}

// buildHealthStatusAttrs builds optimized attribute slice for health status gauge.
// This simplified version is used specifically for gauge metrics with minimal attributes.
func (m *DiskMetrics) buildHealthStatusAttrs(overallHealth string, activeAlerts int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrOverallHealth, overallHealth),
		attribute.Int(AttrActiveAlerts, activeAlerts),
		m.baseInstanceAttr, // Reuse cached attribute
	}
}

// MetricRecorder provides helper functions for consistent metric recording patterns.
type MetricRecorder struct {
	metrics *DiskMetrics
}

// NewMetricRecorder creates a metric recording helper for advanced use cases.
// The MetricRecorder provides additional convenience methods for metric recording.
func (m *DiskMetrics) NewMetricRecorder() *MetricRecorder {
	return &MetricRecorder{metrics: m}
}

// RecordDiskOperationWithTiming provides a defer-compatible helper for disk operation timing.
// Returns a function that should be called with defer to automatically record duration and result.
func (m *DiskMetrics) RecordDiskOperationWithTiming(
	ctx context.Context,
	operation, path string,
	startTime time.Time,
	result *string,
	correlationID string,
) func() {
	return func() {
		duration := time.Since(startTime)
		m.RecordDiskOperation(ctx, operation, path, duration, *result, correlationID)
	}
}

// RecordRetentionPolicyOperationWithTiming provides defer-compatible helper for policy operation timing.
// Automatically records duration, evaluated items, affected items, and compliance status.
func (m *DiskMetrics) RecordRetentionPolicyOperationWithTiming(
	ctx context.Context,
	operation, policyID, path string,
	startTime time.Time,
	itemsEvaluated, itemsAffected *int64,
	complianceStatus, result *string,
	dryRun bool,
	correlationID string,
) func() {
	return func() {
		duration := time.Since(startTime)
		m.RecordRetentionPolicyOperation(
			ctx, operation, policyID, path, duration,
			*itemsEvaluated, *itemsAffected, *complianceStatus, dryRun, *result, correlationID,
		)
	}
}

// RecordCleanupOperationWithTiming provides defer-compatible helper for cleanup operation timing.
// Automatically records duration, items cleaned, bytes freed, and dry-run status.
func (m *DiskMetrics) RecordCleanupOperationWithTiming(
	ctx context.Context,
	strategy, path string,
	startTime time.Time,
	itemsCleaned, bytesFreed *int64,
	dryRun bool,
	result *string,
	correlationID string,
) func() {
	return func() {
		duration := time.Since(startTime)
		m.RecordCleanupOperation(
			ctx, strategy, path, duration, *itemsCleaned, *bytesFreed, dryRun, *result, correlationID,
		)
	}
}

// RecordDiskUsage records current disk usage metrics.
func (m *DiskMetrics) RecordDiskUsage(
	ctx context.Context,
	path string,
	_, _ int64,
	usagePercent float64,
	operationType string,
) {
	attributes := m.buildDiskUsageAttrs(path, operationType)
	m.diskUsageGauge.Record(ctx, usagePercent, metric.WithAttributes(attributes...))
}

// RecordDiskOperation records disk operation timing and success metrics.
func (m *DiskMetrics) RecordDiskOperation(
	ctx context.Context,
	operation, path string,
	duration time.Duration,
	result, correlationID string,
) {
	attributes := m.buildDiskOperationAttrs(operation, path, result, correlationID)
	m.diskOperationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
	m.diskOperationCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

// RecordDiskAlert records disk alert metrics by severity.
func (m *DiskMetrics) RecordDiskAlert(ctx context.Context, path, alertLevel, alertType, correlationID string) {
	attributes := m.buildDiskAlertAttrs(path, alertLevel, alertType, correlationID)
	m.diskAlertCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

// RecordDiskHealthCheck records disk health check metrics.
func (m *DiskMetrics) RecordDiskHealthCheck(
	ctx context.Context,
	_ []string,
	duration time.Duration,
	overallHealth string,
	pathsChecked, activeAlerts int,
	correlationID string,
) {
	attributes := m.buildHealthCheckAttrs(overallHealth, correlationID, pathsChecked, activeAlerts)
	m.diskHealthCheckDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))

	// Record health status as integer gauge
	var healthValue int64
	switch overallHealth {
	case DiskHealthGoodStr:
		healthValue = 0
	case DiskHealthWarningStr:
		healthValue = 1
	case DiskHealthCriticalStr:
		healthValue = 2
	case string(ShutdownPhaseFailed):
		healthValue = 3
	default:
		healthValue = -1
	}

	healthStatusAttributes := m.buildHealthStatusAttrs(overallHealth, activeAlerts)
	m.diskHealthStatusGauge.Record(ctx, healthValue, metric.WithAttributes(healthStatusAttributes...))
}

// RecordCleanupOperation records cleanup operation metrics.
func (m *DiskMetrics) RecordCleanupOperation(
	ctx context.Context,
	strategy, path string,
	duration time.Duration,
	itemsCleaned, bytesFreed int64,
	dryRun bool,
	result, correlationID string,
) {
	attributes := m.buildCleanupOperationAttrs(strategy, path, result, correlationID, dryRun)
	m.cleanupOperationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
	m.cleanupItemsCounter.Add(ctx, itemsCleaned, metric.WithAttributes(attributes...))
	m.cleanupBytesFreedCounter.Add(ctx, bytesFreed, metric.WithAttributes(attributes...))
}

// RecordRetentionPolicyOperation records retention policy operation metrics.
func (m *DiskMetrics) RecordRetentionPolicyOperation(
	ctx context.Context,
	operation, policyID, path string,
	duration time.Duration,
	_, _ int64,
	complianceStatus string,
	dryRun bool,
	result, correlationID string,
) {
	attributes := m.buildRetentionPolicyAttrs(
		operation,
		policyID,
		path,
		complianceStatus,
		result,
		correlationID,
		dryRun,
	)
	m.retentionPolicyDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
	m.retentionPolicyCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}
