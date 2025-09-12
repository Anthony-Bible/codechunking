package queue

import (
	"codechunking/internal/port/outbound"
	"time"
)

// Configuration validation and default management utilities

// DefaultBatchConfig creates a default batch configuration with sensible defaults.
func DefaultBatchConfig() *outbound.BatchConfig {
	return &outbound.BatchConfig{
		MinBatchSize:        1,
		MaxBatchSize:        100,
		BatchTimeout:        5 * time.Second,
		MaxQueueSize:        1000,
		ProcessingInterval:  1 * time.Second,
		EnableDynamicSizing: true,
		PriorityWeights: map[outbound.RequestPriority]float64{
			outbound.PriorityRealTime:    1.0,
			outbound.PriorityInteractive: 0.7,
			outbound.PriorityBackground:  0.4,
			outbound.PriorityBatch:       0.1,
		},
	}
}

// ValidateBatchConfig validates a batch configuration and returns detailed errors.
func ValidateBatchConfig(config *outbound.BatchConfig) error {
	if config == nil {
		return createConfigurationError("invalid_config", "Configuration cannot be nil")
	}

	// Validate batch size constraints
	if config.MinBatchSize < 1 {
		return createConfigurationError(
			"invalid_min_batch_size",
			"MinBatchSize must be >= 1",
		)
	}

	if config.MaxBatchSize < config.MinBatchSize {
		return createConfigurationError(
			"invalid_max_batch_size",
			"MaxBatchSize must be >= MinBatchSize",
		)
	}

	// Validate timeout values
	if config.BatchTimeout <= 0 {
		return createConfigurationError(
			"invalid_batch_timeout",
			"BatchTimeout must be positive",
		)
	}

	if config.ProcessingInterval <= 0 {
		return createConfigurationError(
			"invalid_processing_interval",
			"ProcessingInterval must be positive",
		)
	}

	// Validate queue size
	if config.MaxQueueSize < 1 {
		return createConfigurationError(
			"invalid_max_queue_size",
			"MaxQueueSize must be >= 1",
		)
	}

	// Validate priority weights
	if config.PriorityWeights != nil {
		if err := validatePriorityWeights(config.PriorityWeights); err != nil {
			return err
		}
	}

	// Validate reasonable upper bounds to prevent resource exhaustion
	if config.MaxBatchSize > 1000 {
		return createConfigurationError(
			"max_batch_size_too_large",
			"MaxBatchSize should not exceed 1000 to prevent memory issues",
		)
	}

	if config.MaxQueueSize > 100000 {
		return createConfigurationError(
			"max_queue_size_too_large",
			"MaxQueueSize should not exceed 100,000 to prevent memory issues",
		)
	}

	if config.BatchTimeout > 5*time.Minute {
		return createConfigurationError(
			"batch_timeout_too_long",
			"BatchTimeout should not exceed 5 minutes",
		)
	}

	return nil
}

// validatePriorityWeights validates the priority weights configuration.
func validatePriorityWeights(weights map[outbound.RequestPriority]float64) error {
	requiredPriorities := []outbound.RequestPriority{
		outbound.PriorityRealTime,
		outbound.PriorityInteractive,
		outbound.PriorityBackground,
		outbound.PriorityBatch,
	}

	// Check that all required priorities have weights
	for _, priority := range requiredPriorities {
		weight, exists := weights[priority]
		if !exists {
			return createConfigurationError(
				"missing_priority_weight",
				"Missing weight for priority: "+string(priority),
			)
		}

		// Validate weight range
		if weight < 0.0 || weight > 1.0 {
			return createConfigurationError(
				"invalid_priority_weight",
				"Priority weight must be between 0.0 and 1.0 for: "+string(priority),
			)
		}
	}

	// Validate that real-time has the highest weight
	if weights[outbound.PriorityRealTime] < weights[outbound.PriorityInteractive] ||
		weights[outbound.PriorityRealTime] < weights[outbound.PriorityBackground] ||
		weights[outbound.PriorityRealTime] < weights[outbound.PriorityBatch] {
		return createConfigurationError(
			"invalid_priority_ordering",
			"Real-time priority must have the highest weight",
		)
	}

	return nil
}

// NormalizeConfig normalizes and applies defaults to missing configuration values.
func NormalizeConfig(config *outbound.BatchConfig) *outbound.BatchConfig {
	if config == nil {
		return DefaultBatchConfig()
	}

	normalized := *config // Copy the config

	// Apply defaults for zero values
	if normalized.MinBatchSize == 0 {
		normalized.MinBatchSize = 1
	}

	if normalized.MaxBatchSize == 0 {
		normalized.MaxBatchSize = 100
	}

	if normalized.BatchTimeout == 0 {
		normalized.BatchTimeout = 5 * time.Second
	}

	if normalized.MaxQueueSize == 0 {
		normalized.MaxQueueSize = 1000
	}

	if normalized.ProcessingInterval == 0 {
		normalized.ProcessingInterval = 1 * time.Second
	}

	// Initialize priority weights if missing
	if normalized.PriorityWeights == nil {
		normalized.PriorityWeights = map[outbound.RequestPriority]float64{
			outbound.PriorityRealTime:    1.0,
			outbound.PriorityInteractive: 0.7,
			outbound.PriorityBackground:  0.4,
			outbound.PriorityBatch:       0.1,
		}
	} else {
		// Fill in missing priority weights with defaults
		defaults := DefaultBatchConfig().PriorityWeights
		for priority, defaultWeight := range defaults {
			if _, exists := normalized.PriorityWeights[priority]; !exists {
				normalized.PriorityWeights[priority] = defaultWeight
			}
		}
	}

	return &normalized
}

// ValidateAndNormalize validates a configuration first, then normalizes it if valid.
// This ensures invalid configurations are rejected before normalization.
func ValidateAndNormalize(config *outbound.BatchConfig) (*outbound.BatchConfig, error) {
	// Reject nil config - this should be an explicit error
	if config == nil {
		return nil, createConfigurationError("invalid_config", "Configuration cannot be nil")
	}

	// First validate the input as-is (for explicit invalid values)
	if err := validateConfigurationStrict(config); err != nil {
		return nil, err
	}

	// Then normalize and do full validation
	normalized := NormalizeConfig(config)

	if err := ValidateBatchConfig(normalized); err != nil {
		return nil, err
	}

	return normalized, nil
}

// validateConfigurationStrict validates non-zero values without normalization.
func validateConfigurationStrict(config *outbound.BatchConfig) error {
	if config == nil {
		return createConfigurationError("invalid_config", "Configuration cannot be nil")
	}

	// Only validate non-zero values (zero values will be normalized)
	if config.MinBatchSize < 1 {
		return createConfigurationError(
			"invalid_min_batch_size",
			"MinBatchSize must be >= 1",
		)
	}

	if config.MinBatchSize != 0 && config.MaxBatchSize != 0 && config.MaxBatchSize < config.MinBatchSize {
		return createConfigurationError(
			"invalid_max_batch_size",
			"MaxBatchSize must be >= MinBatchSize",
		)
	}

	return nil
}

// createConfigurationError creates a standardized configuration error.
func createConfigurationError(code, message string) *outbound.QueueManagerError {
	return &outbound.QueueManagerError{
		Code:      code,
		Message:   message,
		Type:      "configuration",
		Retryable: false,
	}
}

// GetRecommendedBatchSize provides recommended batch sizes for different scenarios.
func GetRecommendedBatchSize(priority outbound.RequestPriority, currentLoad int) int {
	switch priority {
	case outbound.PriorityRealTime:
		// Small batches for low latency (1-5 requests)
		return min(5, max(1, currentLoad))
	case outbound.PriorityInteractive:
		// Medium batches for responsive UI (5-25 requests)
		return min(25, max(5, currentLoad/2))
	case outbound.PriorityBackground:
		// Large-medium batches (25-100 requests)
		return min(100, max(25, currentLoad))
	case outbound.PriorityBatch:
		// Large batches for efficiency (50-200 requests)
		return min(200, max(50, currentLoad))
	default:
		// Default to moderate batch size
		return min(50, max(10, currentLoad/2))
	}
}

// EstimateOptimalQueueSize estimates the optimal queue size based on system parameters.
func EstimateOptimalQueueSize(
	expectedRequestsPerSecond int,
	averageProcessingTimeMs int,
	targetLatencyMs int,
) int {
	// Calculate minimum queue size needed to handle expected load
	minQueueSize := expectedRequestsPerSecond * averageProcessingTimeMs / 1000

	// Add buffer for burst traffic
	bufferMultiplier := 2.0

	// Consider target latency constraints
	latencyFactor := 1.0
	if targetLatencyMs < 1000 { // Less than 1 second
		latencyFactor = 0.5 // Smaller queue for better latency
	} else if targetLatencyMs > 5000 { // More than 5 seconds
		latencyFactor = 2.0 // Larger queue is acceptable
	}

	estimatedSize := int(float64(minQueueSize) * bufferMultiplier * latencyFactor)

	// Apply reasonable bounds
	if estimatedSize < 100 {
		estimatedSize = 100
	}
	if estimatedSize > 10000 {
		estimatedSize = 10000
	}

	return estimatedSize
}

// IsConfigurationOptimal checks if a configuration is well-optimized.
func IsConfigurationOptimal(config *outbound.BatchConfig) (bool, []string) {
	var warnings []string

	// Check for potentially suboptimal settings
	if config.MinBatchSize == config.MaxBatchSize {
		warnings = append(warnings, "MinBatchSize equals MaxBatchSize - no batch size flexibility")
	}

	if config.BatchTimeout > 30*time.Second {
		warnings = append(warnings, "BatchTimeout is very long - may impact response times")
	}

	if config.MaxBatchSize > 500 {
		warnings = append(warnings, "MaxBatchSize is very large - may cause memory pressure")
	}

	if !config.EnableDynamicSizing {
		warnings = append(warnings, "Dynamic sizing is disabled - may miss optimization opportunities")
	}

	// Check priority weight distribution
	if weights := config.PriorityWeights; weights != nil {
		realTimeWeight := weights[outbound.PriorityRealTime]
		batchWeight := weights[outbound.PriorityBatch]

		if realTimeWeight-batchWeight < 0.3 {
			warnings = append(warnings, "Priority weight difference is small - may not provide enough prioritization")
		}
	}

	return len(warnings) == 0, warnings
}
