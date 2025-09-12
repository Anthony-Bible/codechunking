package queue

import (
	"codechunking/internal/port/outbound"
	"errors"
	"time"
)

// Helper utility functions for batch queue management

// Note: min and max functions are already defined in batch_queue_manager.go

// validateEmbeddingRequest validates an individual embedding request.
func validateEmbeddingRequest(request *outbound.EmbeddingRequest) error {
	if request == nil {
		return &outbound.QueueManagerError{
			Code:    "nil_request",
			Message: "Request cannot be nil",
			Type:    "validation",
		}
	}

	if request.RequestID == "" {
		return &outbound.QueueManagerError{
			Code:      "missing_request_id",
			Message:   "RequestID is required",
			Type:      "validation",
			RequestID: request.RequestID,
		}
	}

	if request.Text == "" {
		return &outbound.QueueManagerError{
			Code:      "missing_text",
			Message:   "Text is required",
			Type:      "validation",
			RequestID: request.RequestID,
		}
	}

	// Validate priority
	validPriorities := map[outbound.RequestPriority]bool{
		outbound.PriorityRealTime:    true,
		outbound.PriorityInteractive: true,
		outbound.PriorityBackground:  true,
		outbound.PriorityBatch:       true,
	}

	if !validPriorities[request.Priority] {
		return &outbound.QueueManagerError{
			Code:      "invalid_priority",
			Message:   "Invalid priority value",
			Type:      "validation",
			RequestID: request.RequestID,
		}
	}

	return nil
}

// validateBulkEmbeddingRequests validates a slice of embedding requests.
func validateBulkEmbeddingRequests(requests []*outbound.EmbeddingRequest) error {
	if requests == nil {
		return &outbound.QueueManagerError{
			Code:      "invalid_input",
			Message:   "Requests slice cannot be nil",
			Type:      "validation",
			Retryable: false,
		}
	}

	if len(requests) == 0 {
		return nil // Empty slice is valid, no work to do
	}

	// Validate all requests
	for i, request := range requests {
		if err := validateEmbeddingRequest(request); err != nil {
			queueErr := &outbound.QueueManagerError{}
			if errors.As(err, &queueErr) {
				// Enhance error with bulk context
				return &outbound.QueueManagerError{
					Code:      queueErr.Code,
					Message:   "Bulk request validation failed at index " + string(rune(i)) + ": " + queueErr.Message,
					Type:      "validation",
					RequestID: queueErr.RequestID,
					Retryable: false,
					Cause:     queueErr,
				}
			}
			return err
		}
	}

	return nil
}

// calculateTotalQueueSize calculates the total number of requests across all priority queues.
func calculateTotalQueueSize(queues map[outbound.RequestPriority][]*outbound.EmbeddingRequest) int {
	total := 0
	for _, requests := range queues {
		total += len(requests)
	}
	return total
}

// updatePriorityDistribution updates the priority distribution statistics after queuing a request.
func updatePriorityDistribution(stats *outbound.QueueStats, request *outbound.EmbeddingRequest) {
	if stats.PriorityDistribution == nil {
		stats.PriorityDistribution = make(map[outbound.RequestPriority]int)
	}
	stats.PriorityDistribution[request.Priority]++
}

// calculateQueueUtilization calculates queue utilization percentage.
func calculateQueueUtilization(totalQueueSize, maxQueueSize int) float64 {
	if maxQueueSize <= 0 {
		return 0.0
	}
	return float64(totalQueueSize) / float64(maxQueueSize)
}

// isQueueNearCapacity determines if the queue is approaching capacity.
func isQueueNearCapacity(totalQueueSize, maxQueueSize int, threshold float64) bool {
	if threshold <= 0 || threshold >= 1 {
		threshold = 0.9 // Default to 90% threshold
	}
	return float64(totalQueueSize) >= float64(maxQueueSize)*threshold
}

// calculateBatchEfficiency calculates the efficiency of batch processing.
func calculateBatchEfficiency(totalRequests, totalBatches int64) float64 {
	if totalBatches == 0 {
		return 0.0
	}
	avgRequestsPerBatch := float64(totalRequests) / float64(totalBatches)
	// Efficiency is based on how close we are to optimal batch size (assumed to be 25)
	optimalBatchSize := 25.0
	if avgRequestsPerBatch <= optimalBatchSize {
		return avgRequestsPerBatch / optimalBatchSize
	}
	// Efficiency decreases for batches that are too large
	return optimalBatchSize / avgRequestsPerBatch
}

// estimateProcessingTime estimates the time required to process a batch.
func estimateProcessingTime(batchSize int) time.Duration {
	// Base latency for API call
	baseLatency := 200 * time.Millisecond
	// Processing time increases with batch size
	processingTime := time.Duration(float64(batchSize)*5.0) * time.Millisecond
	return baseLatency + processingTime
}

// calculateRequestsPerSecond calculates the requests per second rate.
func calculateRequestsPerSecond(totalRequests int64, duration time.Duration) float64 {
	if duration <= 0 {
		return 0.0
	}
	seconds := duration.Seconds()
	if seconds == 0 {
		return 0.0
	}
	return float64(totalRequests) / seconds
}

// calculateBatchesPerSecond calculates the batches per second rate.
func calculateBatchesPerSecond(totalBatches int64, duration time.Duration) float64 {
	if duration <= 0 {
		return 0.0
	}
	seconds := duration.Seconds()
	if seconds == 0 {
		return 0.0
	}
	return float64(totalBatches) / seconds
}

// getPriorityProcessingOrder returns the order in which priorities should be processed.
func getPriorityProcessingOrder() []outbound.RequestPriority {
	return []outbound.RequestPriority{
		outbound.PriorityRealTime,
		outbound.PriorityInteractive,
		outbound.PriorityBackground,
		outbound.PriorityBatch,
	}
}

// createQueueCapacityError creates a standardized queue capacity exceeded error.
func createQueueCapacityError(requestID string, currentSize, maxSize int) *outbound.QueueManagerError {
	return &outbound.QueueManagerError{
		Code:      "queue_full",
		Message:   "Queue capacity exceeded",
		Type:      "capacity",
		RequestID: requestID,
		Retryable: true,
	}
}

// createValidationError creates a standardized validation error.
func createValidationError(code, message, requestID string, cause error) *outbound.QueueManagerError {
	return &outbound.QueueManagerError{
		Code:      code,
		Message:   message,
		Type:      "validation",
		RequestID: requestID,
		Retryable: false,
		Cause:     cause,
	}
}

// createProcessingError creates a standardized processing error.
func createProcessingError(code, message string, cause error) *outbound.QueueManagerError {
	return &outbound.QueueManagerError{
		Code:      code,
		Message:   message,
		Type:      "processing",
		Retryable: true,
		Cause:     cause,
	}
}

// validateTimeout validates a timeout duration.
func validateTimeout(timeout time.Duration, minTimeout, maxTimeout time.Duration) error {
	if timeout < minTimeout {
		return errors.New("timeout too short")
	}
	if timeout > maxTimeout {
		return errors.New("timeout too long")
	}
	return nil
}

// calculateProcessingLag calculates the processing lag in seconds.
func calculateProcessingLag(lastProcessed time.Time) float64 {
	if lastProcessed.IsZero() {
		return 0.0
	}
	return time.Since(lastProcessed).Seconds()
}

// calculateErrorRate calculates the error rate percentage.
func calculateErrorRate(totalErrors, totalProcessed int64) float64 {
	if totalProcessed == 0 {
		return 0.0
	}
	return float64(totalErrors) / float64(totalProcessed)
}
