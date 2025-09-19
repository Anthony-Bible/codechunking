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
	queues map[outbound.RequestPriority][]*outbound.EmbeddingRequest,
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
		RealTimeQueueSize:      len(queues[outbound.PriorityRealTime]),
		InteractiveQueueSize:   len(queues[outbound.PriorityInteractive]),
		BackgroundQueueSize:    len(queues[outbound.PriorityBackground]),
		BatchQueueSize:         len(queues[outbound.PriorityBatch]),
		TotalQueueSize:         qsc.calculateTotalQueueSize(queues),
		TotalRequestsProcessed: totalProcessed,
		TotalBatchesCreated:    totalBatches,
		TotalErrors:            totalErrors,
		TimeoutCount:           timeoutCount,
		DeadlineExceededCount:  deadlineExceeded,
		QueueStartTime:         startTime,
		LastProcessedAt:        lastProcessed,
		UptimeDuration:         time.Since(startTime),
		PriorityDistribution:   make(map[outbound.RequestPriority]int),
	}

	// Calculate average requests per batch
	stats.AverageRequestsPerBatch = qsc.calculateAverageRequestsPerBatch(totalProcessed, totalBatches)

	// Calculate priority distribution
	stats.PriorityDistribution = qsc.calculatePriorityDistribution(queues)

	// Calculate throughput metrics
	stats.RequestsPerSecond, stats.BatchesPerSecond = qsc.calculateThroughputMetrics(
		totalProcessed, totalBatches, startTime)

	// Calculate queue utilization
	stats.QueueUtilization = qsc.calculateQueueUtilization(stats.TotalQueueSize, config.MaxQueueSize)

	// Calculate batch efficiency
	stats.BatchSizeEfficiency = qsc.calculateBatchSizeEfficiency(totalProcessed, totalBatches)

	return stats
}

// calculateTotalQueueSize calculates the total number of requests across all queues.
func (qsc *QueueStatsCalculator) calculateTotalQueueSize(
	queues map[outbound.RequestPriority][]*outbound.EmbeddingRequest,
) int {
	total := 0
	for _, requests := range queues {
		total += len(requests)
	}
	return total
}

// calculateAverageRequestsPerBatch calculates the average number of requests per batch.
func (qsc *QueueStatsCalculator) calculateAverageRequestsPerBatch(totalProcessed, totalBatches int64) float64 {
	if totalBatches == 0 {
		return 0.0
	}
	return float64(totalProcessed) / float64(totalBatches)
}

// calculatePriorityDistribution calculates the distribution of requests across priorities.
func (qsc *QueueStatsCalculator) calculatePriorityDistribution(
	queues map[outbound.RequestPriority][]*outbound.EmbeddingRequest,
) map[outbound.RequestPriority]int {
	distribution := make(map[outbound.RequestPriority]int)
	for priority, requests := range queues {
		distribution[priority] = len(requests)
	}
	return distribution
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
func (qsc *QueueStatsCalculator) calculateQueueUtilization(totalQueueSize, maxQueueSize int) float64 {
	if maxQueueSize <= 0 {
		return 0.0
	}
	return float64(totalQueueSize) / float64(maxQueueSize)
}

// calculateBatchSizeEfficiency calculates batch processing efficiency.
func (qsc *QueueStatsCalculator) calculateBatchSizeEfficiency(totalProcessed, totalBatches int64) float64 {
	return calculateBatchEfficiency(totalProcessed, totalBatches)
}

// GetQueueLoadMetrics provides additional load-related metrics.
func (qsc *QueueStatsCalculator) GetQueueLoadMetrics(
	queues map[outbound.RequestPriority][]*outbound.EmbeddingRequest,
	config *outbound.BatchConfig,
) *QueueLoadMetrics {
	totalSize := qsc.calculateTotalQueueSize(queues)
	utilization := qsc.calculateQueueUtilization(totalSize, config.MaxQueueSize)

	return &QueueLoadMetrics{
		TotalQueueSize:      totalSize,
		QueueUtilization:    utilization,
		IsNearCapacity:      utilization >= 0.9,
		CapacityRemaining:   max(0, config.MaxQueueSize-totalSize),
		LoadLevel:           qsc.determineLoadLevel(utilization),
		PriorityBreakdown:   qsc.calculatePriorityDistribution(queues),
		PriorityUtilization: qsc.calculatePriorityUtilization(queues, config.MaxQueueSize),
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

// calculatePriorityUtilization calculates utilization for each priority.
func (qsc *QueueStatsCalculator) calculatePriorityUtilization(
	queues map[outbound.RequestPriority][]*outbound.EmbeddingRequest,
	maxQueueSize int,
) map[outbound.RequestPriority]float64 {
	utilization := make(map[outbound.RequestPriority]float64)

	// Assume equal allocation per priority for utilization calculation
	maxPerPriority := float64(maxQueueSize) / 4.0

	for priority, requests := range queues {
		if maxPerPriority > 0 {
			utilization[priority] = float64(len(requests)) / maxPerPriority
		}
	}

	return utilization
}

// QueueLoadMetrics provides additional metrics about queue load and capacity.
type QueueLoadMetrics struct {
	TotalQueueSize      int                                  `json:"total_queue_size"`
	QueueUtilization    float64                              `json:"queue_utilization"`
	IsNearCapacity      bool                                 `json:"is_near_capacity"`
	CapacityRemaining   int                                  `json:"capacity_remaining"`
	LoadLevel           string                               `json:"load_level"`
	PriorityBreakdown   map[outbound.RequestPriority]int     `json:"priority_breakdown"`
	PriorityUtilization map[outbound.RequestPriority]float64 `json:"priority_utilization"`
}
