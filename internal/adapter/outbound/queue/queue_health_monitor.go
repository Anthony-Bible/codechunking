package queue

import (
	"codechunking/internal/port/outbound"
	"time"
)

// QueueHealthMonitor handles health monitoring and status reporting for queue components.
type QueueHealthMonitor struct {
	config *outbound.BatchConfig
}

// NewQueueHealthMonitor creates a new queue health monitor.
func NewQueueHealthMonitor(config *outbound.BatchConfig) *QueueHealthMonitor {
	return &QueueHealthMonitor{
		config: config,
	}
}

// CalculateQueueHealth calculates comprehensive queue health status.
func (qhm *QueueHealthMonitor) CalculateQueueHealth(
	isRunning bool,
	isProcessing bool,
	totalQueueSize int,
	totalErrors int64,
	totalProcessed int64,
	lastProcessed time.Time,
) *outbound.QueueHealth {
	now := time.Now()
	health := &outbound.QueueHealth{
		LastChecked:  now,
		IsProcessing: isProcessing,
	}

	// Determine overall health status
	health.Status, health.Message, health.QueueBackpressure = qhm.determineHealthStatus(
		isRunning, totalQueueSize, totalErrors, totalProcessed)

	// Set component health
	health.NATSConnectionHealth = qhm.getNATSConnectionHealth(now)
	health.EmbeddingServiceHealth = qhm.getEmbeddingServiceHealth(now)
	health.BatchAnalyzerHealth = qhm.getBatchAnalyzerHealth(now)

	// Calculate performance indicators
	health.ProcessingLagSeconds = qhm.calculateProcessingLag(lastProcessed)
	health.ErrorRate = qhm.calculateErrorRate(totalErrors, totalProcessed)

	return health
}

// determineHealthStatus determines the overall health status of the queue.
func (qhm *QueueHealthMonitor) determineHealthStatus(
	isRunning bool,
	totalQueueSize int,
	totalErrors int64,
	totalProcessed int64,
) (outbound.HealthStatus, string, bool) {
	// Check if queue manager is running
	if !isRunning {
		return outbound.HealthStatusUnhealthy, "Queue manager is not running", false
	}

	// Calculate utilization
	utilization := float64(totalQueueSize) / float64(qhm.config.MaxQueueSize)

	// Check for critical capacity issues
	if utilization >= 0.95 {
		return outbound.HealthStatusUnhealthy, "Queue at critical capacity", true
	}

	// Check for high error rate
	errorRate := qhm.calculateErrorRate(totalErrors, totalProcessed)
	if totalProcessed > 100 && errorRate > 0.1 { // 10% error rate with meaningful sample size
		return outbound.HealthStatusDegraded, "High error rate detected", utilization >= 0.9
	}

	// Check for capacity pressure
	if utilization >= 0.9 {
		return outbound.HealthStatusDegraded, "Queue near capacity", true
	}

	// Check for moderate capacity pressure
	if utilization >= 0.7 {
		return outbound.HealthStatusHealthy, "Operating normally with moderate load", false
	}

	// Normal operation
	return outbound.HealthStatusHealthy, "Operating normally", false
}

// getNATSConnectionHealth returns the health status of NATS connection.
func (qhm *QueueHealthMonitor) getNATSConnectionHealth(now time.Time) outbound.ComponentHealth {
	// For in-memory implementation, NATS is not required
	return outbound.ComponentHealth{
		Status:      outbound.HealthStatusHealthy,
		Message:     "In-memory implementation - no NATS required",
		LastChecked: now,
	}
}

// getEmbeddingServiceHealth returns the health status of the embedding service.
func (qhm *QueueHealthMonitor) getEmbeddingServiceHealth(now time.Time) outbound.ComponentHealth {
	// For basic implementation, assume service is available
	// In a real implementation, this would ping the service
	return outbound.ComponentHealth{
		Status:      outbound.HealthStatusHealthy,
		Message:     "Service available",
		LastChecked: now,
	}
}

// getBatchAnalyzerHealth returns the health status of the batch analyzer.
func (qhm *QueueHealthMonitor) getBatchAnalyzerHealth(now time.Time) outbound.ComponentHealth {
	// For basic implementation, assume analyzer is available
	return outbound.ComponentHealth{
		Status:      outbound.HealthStatusHealthy,
		Message:     "Analyzer available",
		LastChecked: now,
	}
}

// calculateProcessingLag calculates the processing lag in seconds.
func (qhm *QueueHealthMonitor) calculateProcessingLag(lastProcessed time.Time) float64 {
	return calculateProcessingLag(lastProcessed)
}

// calculateErrorRate calculates the error rate percentage.
func (qhm *QueueHealthMonitor) calculateErrorRate(totalErrors, totalProcessed int64) float64 {
	return calculateErrorRate(totalErrors, totalProcessed)
}

// GetDetailedHealthMetrics provides more detailed health metrics for monitoring.
func (qhm *QueueHealthMonitor) GetDetailedHealthMetrics(
	queueSize int,
	totalErrors int64,
	totalProcessed int64,
	timeoutCount int64,
	deadlineExceeded int64,
	lastProcessed time.Time,
) *DetailedHealthMetrics {
	utilization := float64(queueSize) / float64(qhm.config.MaxQueueSize)

	metrics := &DetailedHealthMetrics{
		QueueUtilization:     utilization,
		ErrorRate:            qhm.calculateErrorRate(totalErrors, totalProcessed),
		TimeoutRate:          qhm.calculateTimeoutRate(timeoutCount, totalProcessed),
		DeadlineExceededRate: qhm.calculateDeadlineExceededRate(deadlineExceeded, totalProcessed),
		ProcessingLagSeconds: qhm.calculateProcessingLag(lastProcessed),
		HealthScore:          qhm.calculateHealthScore(utilization, totalErrors, totalProcessed, timeoutCount),
		Recommendations:      qhm.generateHealthRecommendations(utilization, totalErrors, totalProcessed),
	}

	return metrics
}

// calculateTimeoutRate calculates the timeout rate percentage.
func (qhm *QueueHealthMonitor) calculateTimeoutRate(timeoutCount, totalProcessed int64) float64 {
	if totalProcessed == 0 {
		return 0.0
	}
	return float64(timeoutCount) / float64(totalProcessed)
}

// calculateDeadlineExceededRate calculates the deadline exceeded rate percentage.
func (qhm *QueueHealthMonitor) calculateDeadlineExceededRate(deadlineExceeded, totalProcessed int64) float64 {
	if totalProcessed == 0 {
		return 0.0
	}
	return float64(deadlineExceeded) / float64(totalProcessed)
}

// calculateHealthScore calculates an overall health score (0-100).
func (qhm *QueueHealthMonitor) calculateHealthScore(
	utilization float64,
	totalErrors int64,
	totalProcessed int64,
	timeoutCount int64,
) float64 {
	score := 100.0

	// Deduct points for high utilization
	if utilization > 0.9 {
		score -= (utilization - 0.9) * 500 // Up to 50 points for 100% utilization
	} else if utilization > 0.7 {
		score -= (utilization - 0.7) * 100 // Up to 20 points for 90% utilization
	}

	// Deduct points for error rate
	errorRate := qhm.calculateErrorRate(totalErrors, totalProcessed)
	score -= errorRate * 300 // Up to 30 points for 10% error rate

	// Deduct points for timeout rate
	timeoutRate := qhm.calculateTimeoutRate(timeoutCount, totalProcessed)
	score -= timeoutRate * 200 // Up to 20 points for 10% timeout rate

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	return score
}

// generateHealthRecommendations generates recommendations based on current health.
func (qhm *QueueHealthMonitor) generateHealthRecommendations(
	utilization float64,
	totalErrors int64,
	totalProcessed int64,
) []string {
	var recommendations []string

	// Utilization-based recommendations
	if utilization >= 0.9 {
		recommendations = append(recommendations, "Consider increasing MaxQueueSize or processing capacity")
	} else if utilization >= 0.7 {
		recommendations = append(recommendations, "Monitor queue growth - approaching high utilization")
	}

	// Error-based recommendations
	errorRate := qhm.calculateErrorRate(totalErrors, totalProcessed)
	if errorRate > 0.05 { // 5% error rate
		recommendations = append(recommendations, "Investigate source of processing errors")
	}
	if errorRate > 0.1 { // 10% error rate
		recommendations = append(recommendations, "Consider reducing batch sizes to improve stability")
	}

	// Performance recommendations
	if utilization < 0.3 && totalProcessed > 1000 {
		recommendations = append(
			recommendations,
			"Queue utilization is low - consider increasing batch sizes for efficiency",
		)
	}

	return recommendations
}

// DetailedHealthMetrics provides comprehensive health metrics.
type DetailedHealthMetrics struct {
	QueueUtilization     float64  `json:"queue_utilization"`
	ErrorRate            float64  `json:"error_rate"`
	TimeoutRate          float64  `json:"timeout_rate"`
	DeadlineExceededRate float64  `json:"deadline_exceeded_rate"`
	ProcessingLagSeconds float64  `json:"processing_lag_seconds"`
	HealthScore          float64  `json:"health_score"`
	Recommendations      []string `json:"recommendations"`
}
