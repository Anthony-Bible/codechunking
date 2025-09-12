package queue

import (
	"codechunking/internal/port/outbound"
	"context"
	"time"
)

// BatchOptimizer provides intelligent batch sizing for queue management.
type BatchOptimizer struct {
	config *outbound.BatchConfig
}

// NewBatchOptimizer creates a new batch optimizer.
func NewBatchOptimizer(config *outbound.BatchConfig) *BatchOptimizer {
	return &BatchOptimizer{
		config: config,
	}
}

// GetOptimalBatchSize returns the optimal batch size for a priority and current load.
func (bo *BatchOptimizer) GetOptimalBatchSize(
	ctx context.Context,
	priority outbound.RequestPriority,
	currentQueueSize int,
	queueSizes map[outbound.RequestPriority]int,
) int {
	// Use dynamic sizing if enabled, otherwise static
	if bo.config.EnableDynamicSizing {
		return bo.getDynamicBatchSize(priority, currentQueueSize, queueSizes)
	}

	return bo.getStaticBatchSize(priority, currentQueueSize)
}

// getStaticBatchSize provides static batch sizing based on priority.
func (bo *BatchOptimizer) getStaticBatchSize(priority outbound.RequestPriority, queueSize int) int {
	switch priority {
	case outbound.PriorityRealTime:
		// Small batches for low latency (1-5 requests)
		return min(5, max(1, queueSize))
	case outbound.PriorityInteractive:
		// Medium batches for responsive UI (5-25 requests)
		return min(25, max(5, queueSize/2))
	case outbound.PriorityBackground:
		// Large-medium batches (25-100 requests)
		return min(100, max(25, queueSize))
	case outbound.PriorityBatch:
		// Large batches for efficiency (50-200 requests)
		return min(200, max(50, queueSize))
	default:
		return min(bo.config.MaxBatchSize, max(bo.config.MinBatchSize, queueSize/2))
	}
}

// getDynamicBatchSize uses load-based dynamic batch sizing.
func (bo *BatchOptimizer) getDynamicBatchSize(
	priority outbound.RequestPriority,
	currentQueueSize int,
	queueSizes map[outbound.RequestPriority]int,
) int {
	baseSize := bo.getStaticBatchSize(priority, currentQueueSize)

	// Calculate total system load
	totalLoad := 0
	for _, size := range queueSizes {
		totalLoad += size
	}

	// Adjust based on system load
	loadFactor := bo.calculateLoadFactor(totalLoad)
	adjustedSize := int(float64(baseSize) * loadFactor)

	// Apply priority-based adjustments
	priorityMultiplier := bo.getPriorityMultiplier(priority)
	finalSize := int(float64(adjustedSize) * priorityMultiplier)

	// Ensure within bounds
	if finalSize < bo.config.MinBatchSize {
		finalSize = bo.config.MinBatchSize
	}
	if finalSize > bo.config.MaxBatchSize {
		finalSize = bo.config.MaxBatchSize
	}
	if finalSize > currentQueueSize {
		finalSize = currentQueueSize
	}

	return finalSize
}

// calculateLoadFactor returns a multiplier based on system load.
func (bo *BatchOptimizer) calculateLoadFactor(totalLoad int) float64 {
	utilization := float64(totalLoad) / float64(bo.config.MaxQueueSize)

	switch {
	case utilization < 0.3: // Low load
		return 0.7 // Smaller batches for lower latency
	case utilization < 0.7: // Medium load
		return 1.0 // Normal batch sizes
	case utilization < 0.9: // High load
		return 1.3 // Larger batches for efficiency
	default: // Critical load
		return 1.5 // Maximum efficiency needed
	}
}

// getPriorityMultiplier returns a priority-based batch size multiplier.
func (bo *BatchOptimizer) getPriorityMultiplier(priority outbound.RequestPriority) float64 {
	if bo.config.PriorityWeights != nil {
		if weight, exists := bo.config.PriorityWeights[priority]; exists {
			// Higher priority weight means smaller batches for lower latency
			return 2.0 - weight // Convert weight to inverse multiplier
		}
	}

	// Default multipliers
	switch priority {
	case outbound.PriorityRealTime:
		return 0.8 // Smaller batches for low latency
	case outbound.PriorityInteractive:
		return 0.9 // Slightly smaller batches
	case outbound.PriorityBackground:
		return 1.1 // Slightly larger batches
	case outbound.PriorityBatch:
		return 1.3 // Larger batches for efficiency
	default:
		return 1.0
	}
}

// AnalyzeCurrentLoad analyzes current queue load and suggests optimizations.
func (bo *BatchOptimizer) AnalyzeCurrentLoad(
	ctx context.Context,
	queueSizes map[outbound.RequestPriority]int,
	processingMetrics *QueueProcessingMetrics,
) (*LoadAnalysis, error) {
	totalLoad := 0
	for _, size := range queueSizes {
		totalLoad += size
	}

	analysis := &LoadAnalysis{
		TotalQueueSize:       totalLoad,
		PriorityDistribution: queueSizes,
		LoadLevel:            bo.calculateLoadLevel(totalLoad),
		Bottlenecks:          bo.identifyBottlenecks(queueSizes, processingMetrics),
		Recommendations:      []string{},
	}

	// Add recommendations based on load analysis
	if analysis.LoadLevel == LoadLevelHigh {
		analysis.Recommendations = append(analysis.Recommendations,
			"Consider increasing MaxBatchSize to improve throughput")
	}

	if len(analysis.Bottlenecks) > 0 {
		analysis.Recommendations = append(analysis.Recommendations,
			"Detected bottlenecks in processing - consider increasing concurrency")
	}

	// Analyze priority distribution
	if totalLoad > 0 {
		realTimeRatio := float64(queueSizes[outbound.PriorityRealTime]) / float64(totalLoad)
		if realTimeRatio > 0.5 {
			analysis.Recommendations = append(analysis.Recommendations,
				"High real-time priority load - consider dedicated processing resources")
		}
	}

	return analysis, nil
}

// calculateLoadLevel determines the current load level.
func (bo *BatchOptimizer) calculateLoadLevel(totalSize int) LoadLevel {
	capacity := bo.config.MaxQueueSize
	utilization := float64(totalSize) / float64(capacity)

	switch {
	case utilization < 0.3:
		return LoadLevelLow
	case utilization < 0.7:
		return LoadLevelMedium
	case utilization < 0.9:
		return LoadLevelHigh
	default:
		return LoadLevelCritical
	}
}

// identifyBottlenecks identifies processing bottlenecks.
func (bo *BatchOptimizer) identifyBottlenecks(
	queueSizes map[outbound.RequestPriority]int,
	metrics *QueueProcessingMetrics,
) []string {
	var bottlenecks []string

	if metrics == nil {
		return bottlenecks
	}

	// Check for processing time bottlenecks
	if metrics.AverageProcessingTime > 5*time.Second {
		bottlenecks = append(bottlenecks, "High processing time")
	}

	// Check for memory bottlenecks
	if metrics.MemoryUsagePercent > 80.0 {
		bottlenecks = append(bottlenecks, "High memory usage")
	}

	// Check for queue backpressure
	if metrics.BackpressureEvents > 10 {
		bottlenecks = append(bottlenecks, "Queue backpressure")
	}

	// Check for error rate bottlenecks
	if metrics.ErrorRate > 0.05 { // 5% error rate
		bottlenecks = append(bottlenecks, "High error rate")
	}

	return bottlenecks
}

// OptimizeBatchConfiguration suggests optimal configuration based on current metrics.
func (bo *BatchOptimizer) OptimizeBatchConfiguration(
	ctx context.Context,
	currentMetrics *QueueProcessingMetrics,
) (*outbound.BatchConfig, []string) {
	optimized := *bo.config // Copy current config
	var recommendations []string

	if currentMetrics == nil {
		return &optimized, recommendations
	}

	// Optimize based on throughput
	if currentMetrics.ThroughputItemsPerSecond < 10.0 {
		// Low throughput - increase batch sizes
		optimized.MaxBatchSize = min(optimized.MaxBatchSize*2, 200)
		recommendations = append(recommendations, "Increased MaxBatchSize to improve throughput")
	}

	// Optimize based on latency
	if currentMetrics.AverageProcessingTime > 10*time.Second {
		// High latency - decrease batch sizes
		optimized.MaxBatchSize = max(optimized.MaxBatchSize/2, optimized.MinBatchSize)
		recommendations = append(recommendations, "Decreased MaxBatchSize to reduce latency")
	}

	// Optimize based on error rate
	if currentMetrics.ErrorRate > 0.1 { // 10% error rate
		// High error rate - be more conservative
		optimized.MaxBatchSize = max(optimized.MaxBatchSize/2, optimized.MinBatchSize)
		optimized.BatchTimeout *= 2
		recommendations = append(recommendations, "Reduced batch size and increased timeout due to high error rate")
	}

	// Optimize based on memory usage
	if currentMetrics.MemoryUsagePercent > 85.0 {
		// High memory usage - reduce batch sizes
		optimized.MaxBatchSize = max(optimized.MaxBatchSize*3/4, optimized.MinBatchSize)
		recommendations = append(recommendations, "Reduced batch size due to high memory usage")
	}

	return &optimized, recommendations
}

// EstimateOptimalBatchSize provides a quick estimate for optimal batch size.
func (bo *BatchOptimizer) EstimateOptimalBatchSize(
	priority outbound.RequestPriority,
	expectedThroughput float64,
	targetLatency time.Duration,
) int {
	// Start with static recommendation
	baseSize := bo.getStaticBatchSize(priority, 100) // Assume moderate queue

	// Adjust based on expected throughput
	if expectedThroughput > 50 { // High throughput expected
		baseSize = int(float64(baseSize) * 1.5)
	} else if expectedThroughput < 5 { // Low throughput expected
		baseSize = int(float64(baseSize) * 0.7)
	}

	// Adjust based on target latency
	if targetLatency < 500*time.Millisecond { // Strict latency requirements
		baseSize = max(baseSize/2, bo.config.MinBatchSize)
	} else if targetLatency > 10*time.Second { // Relaxed latency requirements
		baseSize = min(baseSize*2, bo.config.MaxBatchSize)
	}

	// Ensure within bounds
	if baseSize < bo.config.MinBatchSize {
		baseSize = bo.config.MinBatchSize
	}
	if baseSize > bo.config.MaxBatchSize {
		baseSize = bo.config.MaxBatchSize
	}

	return baseSize
}

// LoadLevel represents the current queue load level.
type LoadLevel string

const (
	LoadLevelLow      LoadLevel = "low"
	LoadLevelMedium   LoadLevel = "medium"
	LoadLevelHigh     LoadLevel = "high"
	LoadLevelCritical LoadLevel = "critical"
)

// LoadAnalysis represents the analysis of current queue load.
type LoadAnalysis struct {
	TotalQueueSize       int                              `json:"total_queue_size"`
	PriorityDistribution map[outbound.RequestPriority]int `json:"priority_distribution"`
	LoadLevel            LoadLevel                        `json:"load_level"`
	Bottlenecks          []string                         `json:"bottlenecks"`
	Recommendations      []string                         `json:"recommendations"`
}

// QueueProcessingMetrics represents processing metrics for analysis.
type QueueProcessingMetrics struct {
	AverageProcessingTime    time.Duration `json:"average_processing_time"`
	ThroughputItemsPerSecond float64       `json:"throughput_items_per_second"`
	ErrorRate                float64       `json:"error_rate"`
	MemoryUsagePercent       float64       `json:"memory_usage_percent"`
	BackpressureEvents       int           `json:"backpressure_events"`
}
