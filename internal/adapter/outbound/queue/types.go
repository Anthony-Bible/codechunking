package queue

import (
	"codechunking/internal/port/outbound"
	"context"
	"time"
)

// ParallelBatchProcessorConfig defines configuration for parallel batch processing.
type ParallelBatchProcessorConfig struct {
	// MaxWorkers defines the maximum number of concurrent workers
	MaxWorkers int

	// MinWorkers defines the minimum number of workers to keep alive
	MinWorkers int

	// MaxConcurrentRequests limits the number of concurrent embedding service requests
	MaxConcurrentRequests int

	// RequestsPerSecond limits the rate of requests to the embedding service
	RequestsPerSecond float64

	// WorkerIdleTimeout defines how long idle workers are kept alive
	WorkerIdleTimeout time.Duration

	// BatchTimeout defines maximum time to wait for a batch to complete
	BatchTimeout time.Duration

	// EnableAdaptiveScaling allows workers to scale up/down based on load
	EnableAdaptiveScaling bool

	// ErrorThreshold defines the error rate that triggers circuit breaker behavior
	ErrorThreshold float64

	// CircuitBreakerTimeout defines how long to wait before retrying after circuit breaker opens
	CircuitBreakerTimeout time.Duration
}

// WorkerPoolStats provides detailed metrics about the worker pool and processing performance.
type WorkerPoolStats struct {
	ActiveWorkers     int `json:"active_workers"`
	IdleWorkers       int `json:"idle_workers"`
	TotalWorkers      int `json:"total_workers"`
	QueuedBatches     int `json:"queued_batches"`
	ProcessingBatches int `json:"processing_batches"`

	// Performance metrics
	TotalBatchesProcessed    int64         `json:"total_batches_processed"`
	TotalRequestsProcessed   int64         `json:"total_requests_processed"`
	AverageProcessingTime    time.Duration `json:"average_processing_time"`
	AverageConcurrentBatches float64       `json:"average_concurrent_batches"`

	// Rate limiting metrics
	CurrentRequestsPerSecond float64 `json:"current_requests_per_second"`
	RateLimitHits            int64   `json:"rate_limit_hits"`

	// Error metrics
	TotalErrors   int64      `json:"total_errors"`
	ErrorRate     float64    `json:"error_rate"`
	CircuitOpen   bool       `json:"circuit_open"`
	LastErrorTime *time.Time `json:"last_error_time,omitempty"`

	// Resource metrics
	GoroutineCount   int     `json:"goroutine_count"`
	MemoryUsageBytes int64   `json:"memory_usage_bytes"`
	CPUUsagePercent  float64 `json:"cpu_usage_percent"`

	// Timing information
	UptimeDuration time.Duration `json:"uptime_duration"`
	LastScaledAt   *time.Time    `json:"last_scaled_at,omitempty"`
}

// ParallelBatchProcessor extends BatchProcessor with parallel processing capabilities.
type ParallelBatchProcessor interface {
	outbound.BatchProcessor

	// ProcessBatchesParallel processes multiple batches concurrently
	ProcessBatchesParallel(
		ctx context.Context,
		batches [][]*outbound.EmbeddingRequest,
	) ([][]*outbound.EmbeddingBatchResult, error)

	// GetWorkerPoolStats returns current worker pool statistics
	GetWorkerPoolStats(ctx context.Context) (*WorkerPoolStats, error)

	// ScaleWorkers dynamically adjusts the number of workers
	ScaleWorkers(ctx context.Context, targetWorkers int) error

	// GetConfig returns current configuration
	GetConfig() *ParallelBatchProcessorConfig

	// UpdateConfig updates processor configuration
	UpdateConfig(config *ParallelBatchProcessorConfig) error

	// Shutdown gracefully shuts down the processor and all workers
	Shutdown(ctx context.Context) error

	// IsHealthy returns whether the processor is healthy
	IsHealthy(ctx context.Context) (bool, error)
}
