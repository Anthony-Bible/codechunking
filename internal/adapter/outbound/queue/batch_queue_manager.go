package queue

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// NATSBatchQueueManager implements the BatchQueueManager interface using NATS JetStream
// for reliable message processing and priority-based queue management.
type NATSBatchQueueManager struct {
	// Core dependencies
	embeddingService outbound.EmbeddingService
	batchProcessor   outbound.BatchProcessor
	natsConn         *nats.Conn
	batchOptimizer   *BatchOptimizer

	// Queue management
	queues       map[outbound.RequestPriority][]*outbound.EmbeddingRequest
	config       *outbound.BatchConfig
	isRunning    bool
	isProcessing bool

	// Synchronization
	mu sync.RWMutex

	// Statistics and monitoring
	stats         *outbound.QueueStats
	health        *outbound.QueueHealth
	startTime     time.Time
	lastProcessed time.Time

	// Metrics counters
	totalProcessed   int64
	totalBatches     int64
	totalErrors      int64
	timeoutCount     int64
	deadlineExceeded int64
}

// NewNATSBatchQueueManager creates a new NATS-based batch queue manager.
func NewNATSBatchQueueManager(
	embeddingService outbound.EmbeddingService,
	batchProcessor outbound.BatchProcessor,
	natsConn *nats.Conn,
) outbound.BatchQueueManager {
	now := time.Now()

	// Use configuration utilities for default config
	defaultConfig := DefaultBatchConfig()

	// Create batch optimizer
	batchOptimizer := NewBatchOptimizer(defaultConfig)

	return &NATSBatchQueueManager{
		embeddingService: embeddingService,
		batchProcessor:   batchProcessor,
		natsConn:         natsConn,
		batchOptimizer:   batchOptimizer,
		queues: map[outbound.RequestPriority][]*outbound.EmbeddingRequest{
			outbound.PriorityRealTime:    {},
			outbound.PriorityInteractive: {},
			outbound.PriorityBackground:  {},
			outbound.PriorityBatch:       {},
		},
		config:    defaultConfig,
		isRunning: false,
		startTime: now,
		stats: &outbound.QueueStats{
			QueueStartTime:       now,
			PriorityDistribution: make(map[outbound.RequestPriority]int),
		},
		health: &outbound.QueueHealth{
			Status:      outbound.HealthStatusUnhealthy,
			Message:     "Not started",
			LastChecked: now,
			NATSConnectionHealth: outbound.ComponentHealth{
				Status:      outbound.HealthStatusUnhealthy,
				Message:     "Not connected",
				LastChecked: now,
			},
			EmbeddingServiceHealth: outbound.ComponentHealth{
				Status:      outbound.HealthStatusHealthy,
				Message:     "Service available",
				LastChecked: now,
			},
			BatchAnalyzerHealth: outbound.ComponentHealth{
				Status:      outbound.HealthStatusHealthy,
				Message:     "Analyzer available",
				LastChecked: now,
			},
		},
	}
}

// QueueEmbeddingRequest queues an individual embedding request for batch processing.
func (q *NATSBatchQueueManager) QueueEmbeddingRequest(ctx context.Context, request *outbound.EmbeddingRequest) error {
	if err := validateEmbeddingRequest(request); err != nil {
		return createValidationError("invalid_request", "Request validation failed", request.RequestID, err)
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	// Check queue capacity
	totalSize := calculateTotalQueueSize(q.queues)
	if totalSize >= q.config.MaxQueueSize {
		return createQueueCapacityError(request.RequestID, totalSize, q.config.MaxQueueSize)
	}

	// Add to appropriate priority queue
	q.queues[request.Priority] = append(q.queues[request.Priority], request)

	// Update stats
	updatePriorityDistribution(q.stats, request)

	slogger.Info(ctx, "Queued embedding request", slogger.Fields{
		"request_id": request.RequestID,
		"priority":   request.Priority,
		"queue_size": totalSize + 1,
	})

	return nil
}

// QueueBulkEmbeddingRequests queues multiple embedding requests efficiently.
func (q *NATSBatchQueueManager) QueueBulkEmbeddingRequests(
	ctx context.Context,
	requests []*outbound.EmbeddingRequest,
) error {
	if err := validateBulkEmbeddingRequests(requests); err != nil {
		return err
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	// Check capacity for all requests
	totalSize := calculateTotalQueueSize(q.queues)
	if totalSize+len(requests) > q.config.MaxQueueSize {
		return createQueueCapacityError("", totalSize+len(requests), q.config.MaxQueueSize)
	}

	// Add all requests to their appropriate priority queues
	for _, request := range requests {
		q.queues[request.Priority] = append(q.queues[request.Priority], request)
		updatePriorityDistribution(q.stats, request)
	}

	slogger.Info(ctx, "Queued bulk embedding requests", slogger.Fields{
		"request_count":    len(requests),
		"total_queue_size": totalSize + len(requests),
	})

	return nil
}

// ProcessQueue processes pending requests in priority order and creates optimized batches.
func (q *NATSBatchQueueManager) ProcessQueue(ctx context.Context) (int, error) {
	if !q.isRunning {
		return 0, &outbound.QueueManagerError{
			Code:    "not_running",
			Message: "Queue manager is not running",
			Type:    "state",
		}
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	q.isProcessing = true
	defer func() { q.isProcessing = false }()

	batchCount := 0

	// Process in priority order: RealTime, Interactive, Background, Batch
	priorities := getPriorityProcessingOrder()

	for _, priority := range priorities {
		requests := q.queues[priority]
		if len(requests) == 0 {
			continue
		}

		// Determine batch size using the optimizer
		batchSize := q.batchOptimizer.GetOptimalBatchSize(ctx, priority, len(requests), q.getCurrentQueueSizes())
		if batchSize > len(requests) {
			batchSize = len(requests)
		}

		if batchSize > 0 {
			// Create batch from front of queue
			batch := requests[:batchSize]
			q.queues[priority] = requests[batchSize:]

			// Process batch (in actual implementation, this would send to NATS)
			if err := q.processBatch(ctx, batch); err != nil {
				slogger.Error(ctx, "Failed to process batch", slogger.Fields{
					"priority":   priority,
					"batch_size": batchSize,
					"error":      err.Error(),
				})
				q.totalErrors++
				continue
			}

			batchCount++
			q.totalBatches++
			q.totalProcessed += int64(batchSize)
			q.lastProcessed = time.Now()

			slogger.Info(ctx, "Processed batch", slogger.Fields{
				"priority":    priority,
				"batch_size":  batchSize,
				"batch_count": batchCount,
			})
		}
	}

	return batchCount, nil
}

// FlushQueue immediately processes all pending requests regardless of batch size optimization.
func (q *NATSBatchQueueManager) FlushQueue(ctx context.Context) error {
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
func (q *NATSBatchQueueManager) GetQueueStats(ctx context.Context) (*outbound.QueueStats, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stats := &outbound.QueueStats{
		RealTimeQueueSize:      len(q.queues[outbound.PriorityRealTime]),
		InteractiveQueueSize:   len(q.queues[outbound.PriorityInteractive]),
		BackgroundQueueSize:    len(q.queues[outbound.PriorityBackground]),
		BatchQueueSize:         len(q.queues[outbound.PriorityBatch]),
		TotalQueueSize:         q.getTotalQueueSizeUnsafe(),
		TotalRequestsProcessed: q.totalProcessed,
		TotalBatchesCreated:    q.totalBatches,
		TotalErrors:            q.totalErrors,
		TimeoutCount:           q.timeoutCount,
		DeadlineExceededCount:  q.deadlineExceeded,
		QueueStartTime:         q.startTime,
		LastProcessedAt:        q.lastProcessed,
		UptimeDuration:         time.Since(q.startTime),
		PriorityDistribution:   make(map[outbound.RequestPriority]int),
	}

	// Calculate average requests per batch
	if q.totalBatches > 0 {
		stats.AverageRequestsPerBatch = float64(q.totalProcessed) / float64(q.totalBatches)
	}

	// Calculate priority distribution
	for priority, requests := range q.queues {
		stats.PriorityDistribution[priority] = len(requests)
	}

	return stats, nil
}

// GetQueueHealth returns health status of the queue manager and its dependencies.
func (q *NATSBatchQueueManager) GetQueueHealth(ctx context.Context) (*outbound.QueueHealth, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	now := time.Now()
	health := *q.health // Copy current health
	health.LastChecked = now
	health.IsProcessing = q.isProcessing

	// Update health status based on current state
	if !q.isRunning {
		health.Status = outbound.HealthStatusUnhealthy
		health.Message = "Queue manager is not running"
	} else {
		totalSize := calculateTotalQueueSize(q.queues)
		if isQueueNearCapacity(totalSize, q.config.MaxQueueSize, 0.9) {
			health.Status = outbound.HealthStatusDegraded
			health.Message = "Queue near capacity"
			health.QueueBackpressure = true
		} else {
			health.Status = outbound.HealthStatusHealthy
			health.Message = "Operating normally"
			health.QueueBackpressure = false
		}
	}

	// Update NATS connection health
	if q.natsConn != nil && q.natsConn.IsConnected() {
		health.NATSConnectionHealth.Status = outbound.HealthStatusHealthy
		health.NATSConnectionHealth.Message = "Connected"
	} else {
		health.NATSConnectionHealth.Status = outbound.HealthStatusUnhealthy
		health.NATSConnectionHealth.Message = "Disconnected"
	}

	return &health, nil
}

// UpdateBatchConfiguration updates batch processing configuration dynamically.
func (q *NATSBatchQueueManager) UpdateBatchConfiguration(ctx context.Context, config *outbound.BatchConfig) error {
	// Validate and normalize the configuration
	normalizedConfig, err := ValidateAndNormalize(config)
	if err != nil {
		return err
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	q.config = normalizedConfig
	// Update the optimizer with the new config
	q.batchOptimizer = NewBatchOptimizer(normalizedConfig)

	slogger.Info(ctx, "Updated batch configuration", slogger.Fields{
		"min_batch_size": normalizedConfig.MinBatchSize,
		"max_batch_size": normalizedConfig.MaxBatchSize,
		"batch_timeout":  normalizedConfig.BatchTimeout,
		"max_queue_size": normalizedConfig.MaxQueueSize,
	})

	return nil
}

// DrainQueue gracefully drains the queue over the specified timeout period.
func (q *NATSBatchQueueManager) DrainQueue(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		q.mu.RLock()
		totalSize := calculateTotalQueueSize(q.queues)
		q.mu.RUnlock()

		if totalSize == 0 {
			return nil // Queue is empty
		}

		// Process remaining items
		if _, err := q.ProcessQueue(ctx); err != nil {
			return &outbound.QueueManagerError{
				Code:    "drain_failed",
				Message: "Failed to drain queue",
				Type:    "processing",
				Cause:   err,
			}
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
func (q *NATSBatchQueueManager) Start(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.isRunning {
		return nil // Already running
	}

	q.isRunning = true
	q.startTime = time.Now()

	// Update health status
	q.health.Status = outbound.HealthStatusHealthy
	q.health.Message = "Started successfully"

	slogger.Info(ctx, "Started batch queue manager", slogger.Fields{
		"start_time": q.startTime,
	})

	return nil
}

// Stop gracefully shuts down the queue manager and completes pending operations.
func (q *NATSBatchQueueManager) Stop(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.isRunning {
		return nil // Already stopped
	}

	q.isRunning = false
	q.health.Status = outbound.HealthStatusUnhealthy
	q.health.Message = "Stopped"

	slogger.Info(ctx, "Stopped batch queue manager", slogger.Fields{
		"uptime": time.Since(q.startTime),
	})

	return nil
}

// Helper methods

func (q *NATSBatchQueueManager) getTotalQueueSizeUnsafe() int {
	total := 0
	for _, requests := range q.queues {
		total += len(requests)
	}
	return total
}

// getCurrentQueueSizes returns current queue sizes for all priorities.
func (q *NATSBatchQueueManager) getCurrentQueueSizes() map[outbound.RequestPriority]int {
	queueSizes := make(map[outbound.RequestPriority]int)
	for priority, requests := range q.queues {
		queueSizes[priority] = len(requests)
	}
	return queueSizes
}

func (q *NATSBatchQueueManager) processBatch(ctx context.Context, batch []*outbound.EmbeddingRequest) error {
	// For green phase implementation, we just simulate processing
	// In a real implementation, this would send the batch to NATS JetStream
	// and the batch processor would handle the actual embedding generation

	if q.batchProcessor != nil {
		// Process through the batch processor
		_, err := q.batchProcessor.ProcessBatch(ctx, batch)
		return err
	}

	// Minimal simulation for tests
	slogger.Debug(ctx, "Simulated batch processing", slogger.Fields{
		"batch_size": len(batch),
	})

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
