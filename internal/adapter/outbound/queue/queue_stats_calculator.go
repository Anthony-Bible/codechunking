package queue

import (
	"codechunking/internal/port/outbound"
	"time"
)

// QueueStatsCalculator handles calculation of queue statistics and metrics.
type QueueStatsCalculator struct{}

// NewQueueStatsCalculator creates a new queue statistics calculator.
func NewQueueStatsCalculator() *QueueStatsCalculator {
	return &QueueStatsCalculator{}
}

// CalculateQueueStats calculates comprehensive queue statistics.
func (qsc *QueueStatsCalculator) CalculateQueueStats(
	queueSize int,
	config *outbound.BatchConfig,
	totalProcessed int64,
	totalBatches int64,
	totalErrors int64,
	timeoutCount int64,
	deadlineExceeded int64,
	startTime time.Time,
	lastProcessed time.Time,
) *outbound.QueueStats {
	stats := &outbound.QueueStats{
		QueueSize:              queueSize,
		TotalQueueSize:         queueSize,
		TotalRequestsProcessed: totalProcessed,
		TotalBatchesCreated:    totalBatches,
		TotalErrors:            totalErrors,
		TimeoutCount:           timeoutCount,
		DeadlineExceededCount:  deadlineExceeded,
		QueueStartTime:         startTime,
		LastProcessedAt:        lastProcessed,
		UptimeDuration:         time.Since(startTime),
	}

	// Calculate average requests per batch
	stats.AverageRequestsPerBatch = qsc.calculateAverageRequestsPerBatch(totalProcessed, totalBatches)

	// Calculate throughput metrics
	stats.RequestsPerSecond, stats.BatchesPerSecond = qsc.calculateThroughputMetrics(
		totalProcessed, totalBatches, startTime)

	// Calculate queue utilization
	stats.QueueUtilization = qsc.calculateQueueUtilization(queueSize, config.MaxQueueSize)

	// Calculate batch efficiency
	stats.BatchSizeEfficiency = qsc.calculateBatchSizeEfficiency(totalProcessed, totalBatches)

	return stats
}

// calculateAverageRequestsPerBatch calculates the average number of requests per batch.
func (qsc *QueueStatsCalculator) calculateAverageRequestsPerBatch(totalProcessed, totalBatches int64) float64 {
	if totalBatches == 0 {
		return 0.0
	}
	return float64(totalProcessed) / float64(totalBatches)
}

// calculateThroughputMetrics calculates requests per second and batches per second.
func (qsc *QueueStatsCalculator) calculateThroughputMetrics(
	totalProcessed, totalBatches int64,
	startTime time.Time,
) (float64, float64) {
	uptime := time.Since(startTime)
	if uptime <= 0 {
		return 0.0, 0.0
	}

	requestsPerSecond := calculateRequestsPerSecond(totalProcessed, uptime)
	batchesPerSecond := calculateBatchesPerSecond(totalBatches, uptime)
	return requestsPerSecond, batchesPerSecond
}

// calculateQueueUtilization calculates the current queue utilization percentage.
func (qsc *QueueStatsCalculator) calculateQueueUtilization(queueSize, maxQueueSize int) float64 {
	if maxQueueSize <= 0 {
		return 0.0
	}
	return float64(queueSize) / float64(maxQueueSize)
}

// calculateBatchSizeEfficiency calculates batch processing efficiency.
func (qsc *QueueStatsCalculator) calculateBatchSizeEfficiency(totalProcessed, totalBatches int64) float64 {
	return calculateBatchEfficiency(totalProcessed, totalBatches)
}

// GetQueueLoadMetrics provides additional load-related metrics.
func (qsc *QueueStatsCalculator) GetQueueLoadMetrics(
	queueSize int,
	config *outbound.BatchConfig,
) *QueueLoadMetrics {
	utilization := qsc.calculateQueueUtilization(queueSize, config.MaxQueueSize)

	return &QueueLoadMetrics{
		TotalQueueSize:    queueSize,
		QueueUtilization:  utilization,
		IsNearCapacity:    utilization >= 0.9,
		CapacityRemaining: max(0, config.MaxQueueSize-queueSize),
		LoadLevel:         qsc.determineLoadLevel(utilization),
	}
}

// determineLoadLevel determines the current load level based on utilization.
func (qsc *QueueStatsCalculator) determineLoadLevel(utilization float64) string {
	switch {
	case utilization < 0.3:
		return "low"
	case utilization < 0.7:
		return "medium"
	case utilization < 0.9:
		return "high"
	default:
		return "critical"
	}
}

// QueueLoadMetrics provides additional metrics about queue load and capacity.
type QueueLoadMetrics struct {
	TotalQueueSize    int     `json:"total_queue_size"`
	QueueUtilization  float64 `json:"queue_utilization"`
	IsNearCapacity    bool    `json:"is_near_capacity"`
	CapacityRemaining int     `json:"capacity_remaining"`
	LoadLevel         string  `json:"load_level"`
}
