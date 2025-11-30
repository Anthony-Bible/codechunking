package queue

import (
	"codechunking/internal/port/outbound"
	"context"
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

// GetOptimalBatchSize returns the optimal batch size for current load.
func (bo *BatchOptimizer) GetOptimalBatchSize(
	ctx context.Context,
	currentQueueSize int,
) int {
	// Use dynamic sizing if enabled, otherwise static
	if bo.config.EnableDynamicSizing {
		return bo.getDynamicBatchSize(currentQueueSize)
	}

	return bo.getStaticBatchSize(currentQueueSize)
}

// getStaticBatchSize provides static batch sizing based on queue size.
func (bo *BatchOptimizer) getStaticBatchSize(queueSize int) int {
	// Use configured batch size limits
	return min(bo.config.MaxBatchSize, max(bo.config.MinBatchSize, queueSize/2))
}

// getDynamicBatchSize uses load-based dynamic batch sizing.
func (bo *BatchOptimizer) getDynamicBatchSize(currentQueueSize int) int {
	baseSize := bo.getStaticBatchSize(currentQueueSize)

	// Adjust based on queue load
	loadFactor := bo.calculateLoadFactor(currentQueueSize)
	adjustedSize := int(float64(baseSize) * loadFactor)

	// Ensure within bounds
	if adjustedSize < bo.config.MinBatchSize {
		adjustedSize = bo.config.MinBatchSize
	}
	if adjustedSize > bo.config.MaxBatchSize {
		adjustedSize = bo.config.MaxBatchSize
	}
	if adjustedSize > currentQueueSize {
		adjustedSize = currentQueueSize
	}

	return adjustedSize
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
