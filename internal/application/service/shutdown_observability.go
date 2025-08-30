package service

import (
	"codechunking/internal/application/common/slogger"
	"context"
	"time"
)

// ShutdownObservabilityManager provides comprehensive monitoring and logging
// for graceful shutdown operations with production-grade observability.
type ShutdownObservabilityManager struct {
	pool *ShutdownContextPool
}

// NewShutdownObservabilityManager creates a new observability manager.
func NewShutdownObservabilityManager() *ShutdownObservabilityManager {
	return &ShutdownObservabilityManager{
		pool: NewShutdownContextPool(),
	}
}

// LogShutdownStart logs the beginning of a graceful shutdown with context.
func (s *ShutdownObservabilityManager) LogShutdownStart(ctx context.Context, phase ShutdownPhase, componentCount int) {
	slogger.Info(ctx, "Graceful shutdown initiated", slogger.Fields{
		"shutdown_phase":  string(phase),
		"component_count": componentCount,
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
		"operation":       "shutdown_start",
	})
}

// LogShutdownProgress logs progress updates during shutdown phases.
func (s *ShutdownObservabilityManager) LogShutdownProgress(ctx context.Context, status ShutdownStatus) {
	fields := slogger.Fields{
		"shutdown_phase":       string(status.ShutdownPhase),
		"elapsed_time_ms":      status.ElapsedTime.Milliseconds(),
		"timeout_remaining_ms": status.TimeoutRemaining.Milliseconds(),
		"components_total":     status.ComponentsTotal,
		"components_closed":    status.ComponentsClosed,
		"components_failed":    status.ComponentsFailed,
		"operation":            "shutdown_progress",
	}

	if status.LastError != "" {
		fields["last_error"] = status.LastError
	}

	if status.SignalReceived != nil {
		fields["signal_received"] = status.SignalReceived.String()
	}

	slogger.Info(ctx, "Shutdown progress update", fields)
}

// LogComponentShutdown logs individual component shutdown events.
func (s *ShutdownObservabilityManager) LogComponentShutdown(
	ctx context.Context,
	name string,
	duration time.Duration,
	err error,
) {
	fields := slogger.Fields{
		"component_name": name,
		"duration_ms":    duration.Milliseconds(),
		"operation":      "component_shutdown",
	}

	if err != nil {
		fields["error"] = err.Error()
		slogger.Error(ctx, "Component shutdown failed", fields)
	} else {
		slogger.Info(ctx, "Component shutdown completed", fields)
	}
}

// LogShutdownComplete logs successful completion of graceful shutdown.
func (s *ShutdownObservabilityManager) LogShutdownComplete(ctx context.Context, metrics ShutdownMetrics) {
	slogger.Info(ctx, "Graceful shutdown completed successfully", slogger.Fields{
		"total_duration_ms":      metrics.LastShutdownDuration.Milliseconds(),
		"components_succeeded":   metrics.SuccessfulShutdowns,
		"components_failed":      metrics.FailedShutdowns,
		"component_failure_rate": metrics.ComponentFailureRate,
		"timeout_count":          metrics.TimeoutCount,
		"operation":              "shutdown_complete",
	})
}

// LogShutdownTimeout logs when graceful shutdown times out.
func (s *ShutdownObservabilityManager) LogShutdownTimeout(
	ctx context.Context,
	timeoutDuration time.Duration,
	remainingComponents int,
) {
	slogger.Error(ctx, "Graceful shutdown timeout exceeded", slogger.Fields{
		"timeout_duration_ms":   timeoutDuration.Milliseconds(),
		"remaining_components":  remainingComponents,
		"force_shutdown_needed": true,
		"operation":             "shutdown_timeout",
	})
}

// LogForceShutdown logs when force shutdown is triggered.
func (s *ShutdownObservabilityManager) LogForceShutdown(ctx context.Context, reason string) {
	slogger.Error(ctx, "Force shutdown initiated", slogger.Fields{
		"reason":    reason,
		"operation": "force_shutdown",
	})
}

// CreateAuditTrail creates a comprehensive audit trail of the shutdown process.
func (s *ShutdownObservabilityManager) CreateAuditTrail(
	ctx context.Context,
	status ShutdownStatus,
	metrics ShutdownMetrics,
) {
	auditData := slogger.Fields{
		"audit_type":             "graceful_shutdown",
		"shutdown_phase":         string(status.ShutdownPhase),
		"start_time":             status.StartTime.UTC().Format(time.RFC3339),
		"elapsed_time_ms":        status.ElapsedTime.Milliseconds(),
		"components_total":       status.ComponentsTotal,
		"components_closed":      status.ComponentsClosed,
		"components_failed":      status.ComponentsFailed,
		"total_shutdowns":        metrics.TotalShutdowns,
		"successful_shutdowns":   metrics.SuccessfulShutdowns,
		"failed_shutdowns":       metrics.FailedShutdowns,
		"force_shutdowns":        metrics.ForceShutdowns,
		"average_shutdown_ms":    metrics.AverageShutdownTime.Milliseconds(),
		"component_failure_rate": metrics.ComponentFailureRate,
		"operation":              "audit_trail",
	}

	if status.SignalReceived != nil {
		auditData["trigger_signal"] = status.SignalReceived.String()
	}

	if status.LastError != "" {
		auditData["final_error"] = status.LastError
	}

	// Add per-component details for failed components
	failedComponents := make([]map[string]interface{}, 0)
	for _, compStatus := range status.ComponentStatuses {
		if compStatus.Status == string(ShutdownPhaseFailed) || compStatus.ForceKilled {
			failedComponents = append(failedComponents, map[string]interface{}{
				"name":         compStatus.Name,
				"status":       compStatus.Status,
				"duration_ms":  compStatus.Duration.Milliseconds(),
				"error":        compStatus.Error,
				"force_killed": compStatus.ForceKilled,
			})
		}
	}

	if len(failedComponents) > 0 {
		auditData["failed_components"] = failedComponents
	}

	slogger.Info(ctx, "Shutdown audit trail", auditData)
}

// GetPool returns the performance pool for object reuse.
func (s *ShutdownObservabilityManager) GetPool() *ShutdownContextPool {
	return s.pool
}
