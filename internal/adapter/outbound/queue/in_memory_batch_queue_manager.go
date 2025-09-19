package queue

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"sync"
	"time"
)

// InMemoryBatchQueueManager is a simple in-memory implementation of BatchQueueManager
// designed primarily for testing and development use cases.
type InMemoryBatchQueueManager struct {
	// Core configuration
	config *outbound.BatchConfig

	// Queue storage - in-memory priority queues
	queues map[outbound.RequestPriority][]*outbound.EmbeddingRequest

	// State management
	isRunning    bool
	isProcessing bool

	// Component dependencies
	metricsTracker  *QueueMetricsTracker
	statsCalculator *QueueStatsCalculator
	healthMonitor   *QueueHealthMonitor
	batchOptimizer  *BatchOptimizer

	// Synchronization
	mu sync.RWMutex

	// Optional dependencies for actual processing
	batchProcessor   outbound.BatchProcessor
	embeddingService outbound.EmbeddingService
}

// NewInMemoryBatchQueueManager creates a new in-memory batch queue manager.
func NewInMemoryBatchQueueManager() outbound.BatchQueueManager {
	config := DefaultBatchConfig()
	return &InMemoryBatchQueueManager{
		config: config,
		queues: map[outbound.RequestPriority][]*outbound.EmbeddingRequest{
			outbound.PriorityRealTime:    {},
			outbound.PriorityInteractive: {},
			outbound.PriorityBackground:  {},
			outbound.PriorityBatch:       {},
		},
		isRunning:       false,
		metricsTracker:  NewQueueMetricsTracker(),
		statsCalculator: NewQueueStatsCalculator(),
		healthMonitor:   NewQueueHealthMonitor(config),
		batchOptimizer:  NewBatchOptimizer(config),
	}
}

// NewInMemoryBatchQueueManagerWithDeps creates a new in-memory batch queue manager with dependencies.
func NewInMemoryBatchQueueManagerWithDeps(
	batchProcessor outbound.BatchProcessor,
	embeddingService outbound.EmbeddingService,
) outbound.BatchQueueManager {
	manager, ok := NewInMemoryBatchQueueManager().(*InMemoryBatchQueueManager)
	if !ok {
		// This should never happen in practice, but satisfy the linter
		panic("failed to create InMemoryBatchQueueManager")
	}
	manager.batchProcessor = batchProcessor
	manager.embeddingService = embeddingService
	return manager
}

// QueueEmbeddingRequest queues an individual embedding request for batch processing.
func (q *InMemoryBatchQueueManager) QueueEmbeddingRequest(
	ctx context.Context,
	request *outbound.EmbeddingRequest,
) error {
	if err := validateEmbeddingRequest(request); err != nil {
		return err
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	// Check queue capacity
	totalSize := q.getTotalQueueSizeUnsafe()
	if totalSize >= q.config.MaxQueueSize {
		return &outbound.QueueManagerError{
			Code:      "queue_full",
			Message:   "Queue capacity exceeded",
			Type:      "capacity",
			RequestID: request.RequestID,
			Retryable: true,
		}
	}

	// Add to appropriate priority queue
	q.queues[request.Priority] = append(q.queues[request.Priority], request)

	slogger.Info(ctx, "Queued embedding request", slogger.Fields{
		"request_id": request.RequestID,
		"priority":   request.Priority,
		"queue_size": totalSize + 1,
	})

	return nil
}

// QueueBulkEmbeddingRequests queues multiple embedding requests efficiently.
func (q *InMemoryBatchQueueManager) QueueBulkEmbeddingRequests(
	ctx context.Context,
	requests []*outbound.EmbeddingRequest,
) error {
	if err := validateBulkEmbeddingRequests(requests); err != nil {
		return err
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	// Check capacity for all requests
	totalSize := q.getTotalQueueSizeUnsafe()
	if totalSize+len(requests) > q.config.MaxQueueSize {
		return &outbound.QueueManagerError{
			Code:      "queue_full",
			Message:   "Queue capacity exceeded",
			Type:      "capacity",
			Retryable: true,
		}
	}

	// Add all requests to their appropriate priority queues
	for _, request := range requests {
		q.queues[request.Priority] = append(q.queues[request.Priority], request)
	}

	slogger.Info(ctx, "Queued bulk embedding requests", slogger.Fields{
		"request_count":    len(requests),
		"total_queue_size": totalSize + len(requests),
	})

	return nil
}

// ProcessQueue processes pending requests in priority order and creates optimized batches.
func (q *InMemoryBatchQueueManager) ProcessQueue(ctx context.Context) (int, error) {
	// Check running status without lock first
	q.mu.RLock()
	isRunning := q.isRunning
	q.mu.RUnlock()

	if !isRunning {
		return 0, &outbound.QueueManagerError{
			Code:    "not_running",
			Message: "Queue manager is not running",
			Type:    "state",
		}
	}

	// Set processing state
	q.mu.Lock()
	q.isProcessing = true
	q.mu.Unlock()
	defer func() {
		q.mu.Lock()
		q.isProcessing = false
		q.mu.Unlock()
	}()

	batchCount := 0
	priorities := getPriorityProcessingOrder()

	for _, priority := range priorities {
		// Get batch for this priority with minimal lock time
		q.mu.Lock()
		requests := q.queues[priority]
		if len(requests) == 0 {
			q.mu.Unlock()
			continue
		}

		// Get current queue sizes for optimization
		queueSizes := make(map[outbound.RequestPriority]int)
		for p, reqs := range q.queues {
			queueSizes[p] = len(reqs)
		}
		q.mu.Unlock()

		// Calculate optimal batch size outside of lock
		batchSize := q.batchOptimizer.GetOptimalBatchSize(ctx, priority, len(requests), queueSizes)
		if batchSize > len(requests) {
			batchSize = len(requests)
		}

		if batchSize > 0 {
			// Extract batch with minimal lock time
			q.mu.Lock()
			currentRequests := q.queues[priority]
			if len(currentRequests) >= batchSize {
				batch := make([]*outbound.EmbeddingRequest, batchSize)
				copy(batch, currentRequests[:batchSize])
				q.queues[priority] = currentRequests[batchSize:]
				q.mu.Unlock()

				// Process batch outside of lock
				if err := q.processBatch(ctx, batch); err != nil {
					slogger.Error(ctx, "Failed to process batch", slogger.Fields{
						"priority":   priority,
						"batch_size": batchSize,
						"error":      err.Error(),
					})
					q.metricsTracker.RecordError()
					continue
				}

				batchCount++
				q.metricsTracker.RecordBatchProcessed(batchSize)

				slogger.Info(ctx, "Processed batch", slogger.Fields{
					"priority":    priority,
					"batch_size":  batchSize,
					"batch_count": batchCount,
				})
			} else {
				// Queue changed while we were calculating - unlock and continue
				q.mu.Unlock()
			}
		}
	}

	return batchCount, nil
}

// FlushQueue immediately processes all pending requests regardless of batch size optimization.
func (q *InMemoryBatchQueueManager) FlushQueue(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Process all remaining requests in all priority queues
	for priority, requests := range q.queues {
		if len(requests) > 0 {
			if err := q.processBatch(ctx, requests); err != nil {
				return &outbound.QueueManagerError{
					Code:    "flush_failed",
					Message: "Failed to flush queue",
					Type:    "processing",
					Cause:   err,
				}
			}
			q.queues[priority] = []*outbound.EmbeddingRequest{}
		}
	}

	return nil
}

// GetQueueStats returns current statistics about queue state and processing metrics.
func (q *InMemoryBatchQueueManager) GetQueueStats(ctx context.Context) (*outbound.QueueStats, error) {
	q.mu.RLock()
	queues := make(map[outbound.RequestPriority][]*outbound.EmbeddingRequest)
	for priority, requests := range q.queues {
		queues[priority] = requests
	}
	q.mu.RUnlock()

	// Get current metrics
	metrics := q.metricsTracker.GetMetrics()

	// Calculate stats using the dedicated calculator
	stats := q.statsCalculator.CalculateQueueStats(
		queues,
		q.config,
		metrics.TotalProcessed,
		metrics.TotalBatches,
		metrics.TotalErrors,
		metrics.TimeoutCount,
		metrics.DeadlineExceeded,
		metrics.StartTime,
		metrics.LastProcessed,
	)

	return stats, nil
}

// GetQueueHealth returns health status of the queue manager and its dependencies.
func (q *InMemoryBatchQueueManager) GetQueueHealth(ctx context.Context) (*outbound.QueueHealth, error) {
	q.mu.RLock()
	totalQueueSize := q.getTotalQueueSizeUnsafe()
	isRunning := q.isRunning
	isProcessing := q.isProcessing
	q.mu.RUnlock()

	// Get current metrics
	metrics := q.metricsTracker.GetMetrics()

	// Calculate health using the dedicated monitor
	health := q.healthMonitor.CalculateQueueHealth(
		isRunning,
		isProcessing,
		totalQueueSize,
		metrics.TotalErrors,
		metrics.TotalProcessed,
		metrics.LastProcessed,
	)

	return health, nil
}

// UpdateBatchConfiguration updates batch processing configuration dynamically.
func (q *InMemoryBatchQueueManager) UpdateBatchConfiguration(ctx context.Context, config *outbound.BatchConfig) error {
	// Validate configuration outside of lock
	normalizedConfig, err := ValidateAndNormalize(config)
	if err != nil {
		return err
	}

	// Update configuration with minimal lock time
	q.mu.Lock()
	q.config = normalizedConfig
	// Update dependent components
	q.healthMonitor = NewQueueHealthMonitor(normalizedConfig)
	q.batchOptimizer = NewBatchOptimizer(normalizedConfig)
	q.mu.Unlock()

	slogger.Info(ctx, "Updated batch configuration", slogger.Fields{
		"min_batch_size": normalizedConfig.MinBatchSize,
		"max_batch_size": normalizedConfig.MaxBatchSize,
		"batch_timeout":  normalizedConfig.BatchTimeout,
		"max_queue_size": normalizedConfig.MaxQueueSize,
	})

	return nil
}

// DrainQueue gracefully drains the queue over the specified timeout period.
func (q *InMemoryBatchQueueManager) DrainQueue(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Check queue size with minimal lock time
		q.mu.RLock()
		totalSize := q.getTotalQueueSizeUnsafe()
		q.mu.RUnlock()

		if totalSize == 0 {
			return nil // Queue is empty
		}

		// Process remaining items (this method has its own locking)
		if _, err := q.ProcessQueue(ctx); err != nil {
			return createProcessingError("drain_failed", "Failed to drain queue", err)
		}

		// Small delay to prevent busy waiting
		time.Sleep(100 * time.Millisecond)
	}

	return &outbound.QueueManagerError{
		Code:      "drain_timeout",
		Message:   "Drain operation timed out",
		Type:      "timeout",
		Retryable: true,
	}
}

// Start initializes the queue manager and begins background processing.
func (q *InMemoryBatchQueueManager) Start(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.isRunning {
		return nil // Already running
	}

	q.isRunning = true
	startTime := time.Now()
	q.metricsTracker.UpdateStartTime(startTime)

	slogger.Info(ctx, "Started in-memory batch queue manager", slogger.Fields{
		"start_time": startTime,
	})

	return nil
}

// Stop gracefully shuts down the queue manager and completes pending operations.
func (q *InMemoryBatchQueueManager) Stop(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.isRunning {
		return nil // Already stopped
	}

	q.isRunning = false

	slogger.Info(ctx, "Stopped in-memory batch queue manager", slogger.Fields{
		"uptime": q.metricsTracker.GetUptime(),
	})

	return nil
}

// Helper methods

func (q *InMemoryBatchQueueManager) getTotalQueueSizeUnsafe() int {
	total := 0
	for _, requests := range q.queues {
		total += len(requests)
	}
	return total
}

func (q *InMemoryBatchQueueManager) processBatch(ctx context.Context, batch []*outbound.EmbeddingRequest) error {
	// For the green phase implementation, we just simulate processing
	// In a real implementation, this would delegate to the batch processor

	if q.batchProcessor != nil {
		// Use the real batch processor if available
		_, err := q.batchProcessor.ProcessBatch(ctx, batch)
		return err
	}

	// Minimal simulation for tests
	slogger.Debug(ctx, "Simulated batch processing", slogger.Fields{
		"batch_size": len(batch),
	})

	return nil
}
