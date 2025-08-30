package service

import (
	"context"
	"fmt"
	"time"
)

// ShutdownMetricsCollector provides OpenTelemetry metrics collection for graceful shutdown operations.
// This follows the same pattern as the existing OTEL disk metrics implementation.
type ShutdownMetricsCollector struct {
	enabled bool
}

// NewShutdownMetricsCollector creates a new shutdown metrics collector.
func NewShutdownMetricsCollector(enabled bool) *ShutdownMetricsCollector {
	return &ShutdownMetricsCollector{
		enabled: enabled,
	}
}

// RecordShutdownStart records the start of a graceful shutdown operation.
func (s *ShutdownMetricsCollector) RecordShutdownStart(_ context.Context, _ ShutdownPhase, _ int) {
	if !s.enabled {
		return
	}

	// Following the same pattern as disk metrics, this would integrate with OTEL
	// For now, we implement the interface but the actual OTEL integration
	// would be added in a similar way to otel_disk_metrics.go

	// TODO: Add actual OpenTelemetry metrics recording
	// This would record:
	// - shutdown_operations_total{phase="<phase>"} counter
	// - shutdown_components_total gauge
	// - shutdown_start_timestamp gauge
}

// RecordComponentShutdown records the completion of an individual component shutdown.
func (s *ShutdownMetricsCollector) RecordComponentShutdown(
	_ context.Context,
	_ string,
	_ time.Duration,
	_ bool,
) {
	if !s.enabled {
		return
	}

	// TODO: Add actual OpenTelemetry metrics recording
	// This would record:
	// - component_shutdown_duration_seconds{component="<name>",status="<success|failure>"} histogram
	// - component_shutdown_total{component="<name>",status="<success|failure>"} counter
}

// RecordShutdownPhaseTransition records transitions between shutdown phases.
func (s *ShutdownMetricsCollector) RecordShutdownPhaseTransition(
	_ context.Context,
	_, _ ShutdownPhase,
	_ time.Duration,
) {
	if !s.enabled {
		return
	}

	// TODO: Add actual OpenTelemetry metrics recording
	// This would record:
	// - shutdown_phase_duration_seconds{from="<from>",to="<to>"} histogram
	// - shutdown_phase_transitions_total{from="<from>",to="<to>"} counter
}

// RecordShutdownComplete records the completion of the entire shutdown process.
func (s *ShutdownMetricsCollector) RecordShutdownComplete(
	_ context.Context,
	_ time.Duration,
	_ ShutdownMetrics,
) {
	if !s.enabled {
		return
	}

	// TODO: Add actual OpenTelemetry metrics recording
	// This would record:
	// - shutdown_total_duration_seconds histogram
	// - shutdown_completion_total{status="success|timeout|force"} counter
	// - shutdown_component_failure_rate gauge
	// - shutdown_timeout_total counter (if applicable)
}

// RecordResourceCleanup records resource cleanup metrics.
func (s *ShutdownMetricsCollector) RecordResourceCleanup(
	_ context.Context,
	_ ResourceType,
	_ time.Duration,
	_ bool,
) {
	if !s.enabled {
		return
	}

	// TODO: Add actual OpenTelemetry metrics recording
	// This would record:
	// - resource_cleanup_duration_seconds{type="<type>",status="<success|failure>"} histogram
	// - resource_cleanup_total{type="<type>",status="<success|failure>"} counter
}

// RecordWorkerShutdown records worker shutdown metrics.
func (s *ShutdownMetricsCollector) RecordWorkerShutdown(
	_ context.Context,
	_ string,
	_ int,
	_ int64,
	_ time.Duration,
) {
	if !s.enabled {
		return
	}

	// TODO: Add actual OpenTelemetry metrics recording
	// This would record:
	// - worker_shutdown_duration_seconds{worker_id="<id>"} histogram
	// - worker_active_jobs_at_shutdown{worker_id="<id>"} gauge
	// - worker_completed_jobs_at_shutdown{worker_id="<id>"} gauge
}

// RecordDatabaseShutdown records database shutdown metrics.
func (s *ShutdownMetricsCollector) RecordDatabaseShutdown(
	_ context.Context,
	_ string,
	_ int,
	_ int,
	_ time.Duration,
) {
	if !s.enabled {
		return
	}

	// TODO: Add actual OpenTelemetry metrics recording
	// This would record:
	// - database_shutdown_duration_seconds{database="<name>"} histogram
	// - database_active_connections_at_shutdown{database="<name>"} gauge
	// - database_pending_transactions_at_shutdown{database="<name>"} gauge
}

// RecordNATSShutdown records NATS shutdown metrics.
func (s *ShutdownMetricsCollector) RecordNATSShutdown(
	_ context.Context,
	_ string,
	_ int64,
	_ time.Duration,
) {
	if !s.enabled {
		return
	}

	// TODO: Add actual OpenTelemetry metrics recording
	// This would record:
	// - nats_shutdown_duration_seconds{consumer="<name>"} histogram
	// - nats_pending_messages_at_shutdown{consumer="<name>"} gauge
}

// SetEnabled enables or disables metrics collection.
func (s *ShutdownMetricsCollector) SetEnabled(enabled bool) {
	s.enabled = enabled
}

// IsEnabled returns whether metrics collection is enabled.
func (s *ShutdownMetricsCollector) IsEnabled() bool {
	return s.enabled
}

// GetMetricLabels creates standardized metric labels for shutdown operations.
func (s *ShutdownMetricsCollector) GetMetricLabels(
	operation string,
	component string,
	phase ShutdownPhase,
) map[string]string {
	labels := map[string]string{
		"operation": operation,
		"component": component,
	}

	if phase != "" {
		labels["phase"] = string(phase)
	}

	return labels
}

// FormatDuration formats duration for metrics in a consistent way.
func (s *ShutdownMetricsCollector) FormatDuration(duration time.Duration) float64 {
	return duration.Seconds()
}

// GetHealthMetrics returns health-related metrics for shutdown components.
func (s *ShutdownMetricsCollector) GetHealthMetrics(_ context.Context) map[string]interface{} {
	if !s.enabled {
		return nil
	}

	return map[string]interface{}{
		"metrics_collector_enabled": s.enabled,
		"metrics_collector_status":  "healthy",
		"last_updated":              time.Now().UTC().Format(time.RFC3339),
	}
}

// RecordHealthCheck records health check results for shutdown components.
func (s *ShutdownMetricsCollector) RecordHealthCheck(
	_ context.Context,
	_ string,
	_ bool,
	_ time.Duration,
) {
	if !s.enabled {
		return
	}

	// TODO: Add actual OpenTelemetry metrics recording
	// This would record:
	// - component_health_check_duration_seconds{component="<name>",status="<healthy|unhealthy>"} histogram
	// - component_health_status{component="<name>"} gauge (1 for healthy, 0 for unhealthy)
	// Status would be determined by the healthy parameter
}

// CreateMetricsSummary creates a summary of all shutdown metrics for reporting.
func (s *ShutdownMetricsCollector) CreateMetricsSummary(
	_ context.Context,
	metrics ShutdownMetrics,
) map[string]interface{} {
	if !s.enabled {
		return nil
	}

	return map[string]interface{}{
		"total_shutdowns":        metrics.TotalShutdowns,
		"successful_shutdowns":   metrics.SuccessfulShutdowns,
		"failed_shutdowns":       metrics.FailedShutdowns,
		"force_shutdowns":        metrics.ForceShutdowns,
		"average_shutdown_time":  metrics.AverageShutdownTime.Milliseconds(),
		"last_shutdown_duration": metrics.LastShutdownDuration.Milliseconds(),
		"timeout_count":          metrics.TimeoutCount,
		"component_failure_rate": fmt.Sprintf("%.2f", metrics.ComponentFailureRate*100),
		"signal_counts":          metrics.SignalCounts,
		"component_count":        len(metrics.ComponentMetrics),
	}
}
