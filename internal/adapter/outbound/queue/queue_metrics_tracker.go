package queue

import (
	"sync"
	"time"
)

// QueueMetricsTracker handles tracking and management of queue processing metrics.
type QueueMetricsTracker struct {
	// Metrics
	totalProcessed   int64
	totalBatches     int64
	totalErrors      int64
	timeoutCount     int64
	deadlineExceeded int64
	lastProcessed    time.Time
	startTime        time.Time

	// Synchronization
	mu sync.RWMutex
}

// NewQueueMetricsTracker creates a new queue metrics tracker.
func NewQueueMetricsTracker() *QueueMetricsTracker {
	return &QueueMetricsTracker{
		startTime: time.Now(),
	}
}

// RecordBatchProcessed records metrics for a successfully processed batch.
func (qmt *QueueMetricsTracker) RecordBatchProcessed(batchSize int) {
	qmt.mu.Lock()
	defer qmt.mu.Unlock()

	qmt.totalBatches++
	qmt.totalProcessed += int64(batchSize)
	qmt.lastProcessed = time.Now()
}

// RecordError records a processing error.
func (qmt *QueueMetricsTracker) RecordError() {
	qmt.mu.Lock()
	defer qmt.mu.Unlock()

	qmt.totalErrors++
}

// RecordTimeout records a timeout event.
func (qmt *QueueMetricsTracker) RecordTimeout() {
	qmt.mu.Lock()
	defer qmt.mu.Unlock()

	qmt.timeoutCount++
}

// RecordDeadlineExceeded records a deadline exceeded event.
func (qmt *QueueMetricsTracker) RecordDeadlineExceeded() {
	qmt.mu.Lock()
	defer qmt.mu.Unlock()

	qmt.deadlineExceeded++
}

// GetMetrics returns a snapshot of current metrics.
func (qmt *QueueMetricsTracker) GetMetrics() QueueMetrics {
	qmt.mu.RLock()
	defer qmt.mu.RUnlock()

	return QueueMetrics{
		TotalProcessed:   qmt.totalProcessed,
		TotalBatches:     qmt.totalBatches,
		TotalErrors:      qmt.totalErrors,
		TimeoutCount:     qmt.timeoutCount,
		DeadlineExceeded: qmt.deadlineExceeded,
		LastProcessed:    qmt.lastProcessed,
		StartTime:        qmt.startTime,
	}
}

// Reset resets all metrics (useful for testing or restart scenarios).
func (qmt *QueueMetricsTracker) Reset() {
	qmt.mu.Lock()
	defer qmt.mu.Unlock()

	qmt.totalProcessed = 0
	qmt.totalBatches = 0
	qmt.totalErrors = 0
	qmt.timeoutCount = 0
	qmt.deadlineExceeded = 0
	qmt.lastProcessed = time.Time{}
	qmt.startTime = time.Now()
}

// GetTotalProcessed returns the total number of processed requests.
func (qmt *QueueMetricsTracker) GetTotalProcessed() int64 {
	qmt.mu.RLock()
	defer qmt.mu.RUnlock()
	return qmt.totalProcessed
}

// GetTotalBatches returns the total number of processed batches.
func (qmt *QueueMetricsTracker) GetTotalBatches() int64 {
	qmt.mu.RLock()
	defer qmt.mu.RUnlock()
	return qmt.totalBatches
}

// GetTotalErrors returns the total number of errors.
func (qmt *QueueMetricsTracker) GetTotalErrors() int64 {
	qmt.mu.RLock()
	defer qmt.mu.RUnlock()
	return qmt.totalErrors
}

// GetTimeoutCount returns the total number of timeouts.
func (qmt *QueueMetricsTracker) GetTimeoutCount() int64 {
	qmt.mu.RLock()
	defer qmt.mu.RUnlock()
	return qmt.timeoutCount
}

// GetDeadlineExceeded returns the total number of deadline exceeded events.
func (qmt *QueueMetricsTracker) GetDeadlineExceeded() int64 {
	qmt.mu.RLock()
	defer qmt.mu.RUnlock()
	return qmt.deadlineExceeded
}

// GetLastProcessed returns the timestamp of the last processed batch.
func (qmt *QueueMetricsTracker) GetLastProcessed() time.Time {
	qmt.mu.RLock()
	defer qmt.mu.RUnlock()
	return qmt.lastProcessed
}

// GetStartTime returns the tracker start time.
func (qmt *QueueMetricsTracker) GetStartTime() time.Time {
	qmt.mu.RLock()
	defer qmt.mu.RUnlock()
	return qmt.startTime
}

// GetProcessingRate returns the current processing rate (requests per second).
func (qmt *QueueMetricsTracker) GetProcessingRate() float64 {
	qmt.mu.RLock()
	defer qmt.mu.RUnlock()

	uptime := time.Since(qmt.startTime)
	if uptime <= 0 {
		return 0.0
	}

	return calculateRequestsPerSecond(qmt.totalProcessed, uptime)
}

// GetBatchRate returns the current batch processing rate (batches per second).
func (qmt *QueueMetricsTracker) GetBatchRate() float64 {
	qmt.mu.RLock()
	defer qmt.mu.RUnlock()

	uptime := time.Since(qmt.startTime)
	if uptime <= 0 {
		return 0.0
	}

	return calculateBatchesPerSecond(qmt.totalBatches, uptime)
}

// GetErrorRate returns the current error rate (percentage).
func (qmt *QueueMetricsTracker) GetErrorRate() float64 {
	qmt.mu.RLock()
	defer qmt.mu.RUnlock()

	return calculateErrorRate(qmt.totalErrors, qmt.totalProcessed)
}

// GetAverageBatchSize returns the average batch size.
func (qmt *QueueMetricsTracker) GetAverageBatchSize() float64 {
	qmt.mu.RLock()
	defer qmt.mu.RUnlock()

	if qmt.totalBatches == 0 {
		return 0.0
	}

	return float64(qmt.totalProcessed) / float64(qmt.totalBatches)
}

// GetUptime returns the total uptime of the tracker.
func (qmt *QueueMetricsTracker) GetUptime() time.Duration {
	qmt.mu.RLock()
	defer qmt.mu.RUnlock()

	return time.Since(qmt.startTime)
}

// GetPerformanceMetrics returns comprehensive performance metrics.
func (qmt *QueueMetricsTracker) GetPerformanceMetrics() PerformanceMetrics {
	qmt.mu.RLock()
	defer qmt.mu.RUnlock()

	uptime := time.Since(qmt.startTime)

	return PerformanceMetrics{
		RequestsPerSecond: calculateRequestsPerSecond(qmt.totalProcessed, uptime),
		BatchesPerSecond:  calculateBatchesPerSecond(qmt.totalBatches, uptime),
		ErrorRate:         calculateErrorRate(qmt.totalErrors, qmt.totalProcessed),
		AverageBatchSize:  qmt.getAverageBatchSizeUnsafe(),
		TimeoutRate:       qmt.getTimeoutRateUnsafe(),
		DeadlineRate:      qmt.getDeadlineRateUnsafe(),
		ProcessingLag:     calculateProcessingLag(qmt.lastProcessed),
		Uptime:            uptime,
		TotalThroughput:   qmt.totalProcessed,
	}
}

// getAverageBatchSizeUnsafe returns average batch size without locking.
func (qmt *QueueMetricsTracker) getAverageBatchSizeUnsafe() float64 {
	if qmt.totalBatches == 0 {
		return 0.0
	}
	return float64(qmt.totalProcessed) / float64(qmt.totalBatches)
}

// getTimeoutRateUnsafe returns timeout rate without locking.
func (qmt *QueueMetricsTracker) getTimeoutRateUnsafe() float64 {
	if qmt.totalProcessed == 0 {
		return 0.0
	}
	return float64(qmt.timeoutCount) / float64(qmt.totalProcessed)
}

// getDeadlineRateUnsafe returns deadline exceeded rate without locking.
func (qmt *QueueMetricsTracker) getDeadlineRateUnsafe() float64 {
	if qmt.totalProcessed == 0 {
		return 0.0
	}
	return float64(qmt.deadlineExceeded) / float64(qmt.totalProcessed)
}

// UpdateStartTime updates the start time (useful for restart scenarios).
func (qmt *QueueMetricsTracker) UpdateStartTime(startTime time.Time) {
	qmt.mu.Lock()
	defer qmt.mu.Unlock()

	qmt.startTime = startTime
}

// IncrementProcessed atomically increments the processed count.
func (qmt *QueueMetricsTracker) IncrementProcessed(count int64) {
	qmt.mu.Lock()
	defer qmt.mu.Unlock()

	qmt.totalProcessed += count
	qmt.lastProcessed = time.Now()
}

// IncrementBatches atomically increments the batch count.
func (qmt *QueueMetricsTracker) IncrementBatches(count int64) {
	qmt.mu.Lock()
	defer qmt.mu.Unlock()

	qmt.totalBatches += count
}

// RecordBatchSuccess records a successful batch processing event.
func (qmt *QueueMetricsTracker) RecordBatchSuccess(batchSize int, processingTime time.Duration) {
	qmt.mu.Lock()
	defer qmt.mu.Unlock()

	qmt.totalBatches++
	qmt.totalProcessed += int64(batchSize)
	qmt.lastProcessed = time.Now()
}

// RecordBatchFailure records a failed batch processing event.
func (qmt *QueueMetricsTracker) RecordBatchFailure(batchSize int, errorType string) {
	qmt.mu.Lock()
	defer qmt.mu.Unlock()

	qmt.totalErrors++

	// Track specific error types if needed
	switch errorType {
	case "timeout":
		qmt.timeoutCount++
	case "deadline_exceeded":
		qmt.deadlineExceeded++
	}
}

// QueueMetrics represents a snapshot of queue metrics.
type QueueMetrics struct {
	TotalProcessed   int64     `json:"total_processed"`
	TotalBatches     int64     `json:"total_batches"`
	TotalErrors      int64     `json:"total_errors"`
	TimeoutCount     int64     `json:"timeout_count"`
	DeadlineExceeded int64     `json:"deadline_exceeded"`
	LastProcessed    time.Time `json:"last_processed"`
	StartTime        time.Time `json:"start_time"`
}

// PerformanceMetrics represents comprehensive performance metrics.
type PerformanceMetrics struct {
	RequestsPerSecond float64       `json:"requests_per_second"`
	BatchesPerSecond  float64       `json:"batches_per_second"`
	ErrorRate         float64       `json:"error_rate"`
	AverageBatchSize  float64       `json:"average_batch_size"`
	TimeoutRate       float64       `json:"timeout_rate"`
	DeadlineRate      float64       `json:"deadline_rate"`
	ProcessingLag     float64       `json:"processing_lag_seconds"`
	Uptime            time.Duration `json:"uptime"`
	TotalThroughput   int64         `json:"total_throughput"`
}
