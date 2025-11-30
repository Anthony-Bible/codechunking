package outbound

import (
	"context"
	"time"
)

// BatchQueueManager defines the interface for managing embedding request queues and bridging
// individual requests to batch processing using NATS JetStream. This interface handles
// dynamic batch sizing and integration with existing batch processing systems.
type BatchQueueManager interface {
	// QueueEmbeddingRequest queues an individual embedding request for batch processing.
	// The request will be batched with others based on current system load.
	QueueEmbeddingRequest(ctx context.Context, request *EmbeddingRequest) error

	// QueueBulkEmbeddingRequests queues multiple embedding requests efficiently.
	// Requests are batched appropriately based on configuration.
	QueueBulkEmbeddingRequests(ctx context.Context, requests []*EmbeddingRequest) error

	// ProcessQueue processes pending requests and creates optimized batches.
	// Returns the number of batches created and any processing errors encountered.
	ProcessQueue(ctx context.Context) (int, error)

	// FlushQueue immediately processes all pending requests regardless of batch size optimization.
	// Used for shutdown scenarios or when immediate processing is required.
	FlushQueue(ctx context.Context) error

	// GetQueueStats returns current statistics about queue state and processing metrics.
	GetQueueStats(ctx context.Context) (*QueueStats, error)

	// GetQueueHealth returns health status of the queue manager and its dependencies.
	GetQueueHealth(ctx context.Context) (*QueueHealth, error)

	// UpdateBatchConfiguration updates batch processing configuration dynamically.
	// Allows runtime adjustment of batch sizes and processing parameters.
	UpdateBatchConfiguration(ctx context.Context, config *BatchConfig) error

	// DrainQueue gracefully drains the queue over the specified timeout period.
	// Processes remaining requests and prevents new ones from being accepted.
	DrainQueue(ctx context.Context, timeout time.Duration) error

	// Start initializes the queue manager and begins background processing.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the queue manager and completes pending operations.
	Stop(ctx context.Context) error
}

// EmbeddingRequest represents an individual embedding request to be queued and batched.
type EmbeddingRequest struct {
	RequestID     string                 `json:"request_id"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Text          string                 `json:"text"`
	Options       EmbeddingOptions       `json:"options"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	SubmittedAt   time.Time              `json:"submitted_at"`
	Deadline      *time.Time             `json:"deadline,omitempty"`
	CallbackURL   string                 `json:"callback_url,omitempty"`
}

// BatchConfig holds configuration parameters for batch processing optimization.
type BatchConfig struct {
	// MinBatchSize is the minimum number of requests before creating a batch.
	MinBatchSize int `json:"min_batch_size"`

	// MaxBatchSize is the maximum number of requests per batch.
	MaxBatchSize int `json:"max_batch_size"`

	// BatchTimeout is the maximum time to wait before processing a partial batch.
	BatchTimeout time.Duration `json:"batch_timeout"`

	// EnableDynamicSizing allows automatic batch size adjustment based on load.
	EnableDynamicSizing bool `json:"enable_dynamic_sizing"`

	// MaxQueueSize is the maximum number of requests that can be queued.
	MaxQueueSize int `json:"max_queue_size"`

	// ProcessingInterval defines how often the queue processor runs.
	ProcessingInterval time.Duration `json:"processing_interval"`
}

// QueueStats provides detailed statistics about queue operations and performance.
type QueueStats struct {
	// Queue size metrics
	QueueSize      int `json:"queue_size"`
	TotalQueueSize int `json:"total_queue_size"`

	// Processing metrics
	TotalRequestsProcessed  int64         `json:"total_requests_processed"`
	TotalBatchesCreated     int64         `json:"total_batches_created"`
	AverageRequestsPerBatch float64       `json:"average_requests_per_batch"`
	AverageProcessingTime   time.Duration `json:"average_processing_time"`
	AverageBatchLatency     time.Duration `json:"average_batch_latency"`

	// Error and timeout metrics
	TotalErrors           int64 `json:"total_errors"`
	TimeoutCount          int64 `json:"timeout_count"`
	DeadlineExceededCount int64 `json:"deadline_exceeded_count"`

	// Throughput metrics
	RequestsPerSecond float64 `json:"requests_per_second"`
	BatchesPerSecond  float64 `json:"batches_per_second"`

	// Queue efficiency metrics
	QueueUtilization    float64 `json:"queue_utilization"`
	BatchSizeEfficiency float64 `json:"batch_size_efficiency"`

	// Timing information
	LastProcessedAt time.Time     `json:"last_processed_at"`
	QueueStartTime  time.Time     `json:"queue_start_time"`
	UptimeDuration  time.Duration `json:"uptime_duration"`
}

// QueueHealth represents the health status of the queue manager and its dependencies.
type QueueHealth struct {
	// Overall health status
	Status      HealthStatus `json:"status"`
	Message     string       `json:"message"`
	LastChecked time.Time    `json:"last_checked"`

	// Component health
	NATSConnectionHealth   ComponentHealth `json:"nats_connection_health"`
	EmbeddingServiceHealth ComponentHealth `json:"embedding_service_health"`
	BatchAnalyzerHealth    ComponentHealth `json:"batch_analyzer_health"`

	// Performance indicators
	IsProcessing         bool    `json:"is_processing"`
	QueueBackpressure    bool    `json:"queue_backpressure"`
	ProcessingLagSeconds float64 `json:"processing_lag_seconds"`
	ErrorRate            float64 `json:"error_rate"`

	// Resource utilization
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	DiskUsagePercent   float64 `json:"disk_usage_percent"`
}

// HealthStatus represents the overall health state.
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// ComponentHealth represents the health of individual system components.
type ComponentHealth struct {
	Status      HealthStatus  `json:"status"`
	Message     string        `json:"message"`
	LastChecked time.Time     `json:"last_checked"`
	ErrorCount  int64         `json:"error_count"`
	Latency     time.Duration `json:"latency"`
}

// BatchProcessor defines the interface for processing batches of embedding requests.
// This is typically implemented by adapters that integrate with the EmbeddingService.
type BatchProcessor interface {
	// ProcessBatch processes a batch of embedding requests and returns results.
	ProcessBatch(ctx context.Context, requests []*EmbeddingRequest) ([]*EmbeddingBatchResult, error)

	// EstimateBatchCost estimates the cost of processing a batch.
	EstimateBatchCost(ctx context.Context, requests []*EmbeddingRequest) (float64, error)

	// EstimateBatchLatency estimates the processing time for a batch.
	EstimateBatchLatency(ctx context.Context, requests []*EmbeddingRequest) (time.Duration, error)
}

// EmbeddingBatchResult represents the result of processing a batch of embedding requests.
type EmbeddingBatchResult struct {
	RequestID   string                 `json:"request_id"`
	Result      *EmbeddingResult       `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	ProcessedAt time.Time              `json:"processed_at"`
	Latency     time.Duration          `json:"latency"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// QueueManagerError represents errors from the queue manager operations.
type QueueManagerError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Retryable bool   `json:"retryable"`
	Cause     error  `json:"cause,omitempty"`
}

// Error implements the error interface.
func (e *QueueManagerError) Error() string {
	if e.Cause != nil {
		return "queue manager error (" + e.Type + "/" + e.Code + "): " + e.Message + ": " + e.Cause.Error()
	}
	return "queue manager error (" + e.Type + "/" + e.Code + "): " + e.Message
}

// Unwrap returns the underlying cause error.
func (e *QueueManagerError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether the error is retryable.
func (e *QueueManagerError) IsRetryable() bool {
	return e.Retryable
}

// IsQueueFull returns whether the error indicates a full queue.
func (e *QueueManagerError) IsQueueFull() bool {
	return e.Code == "queue_full" || e.Code == "max_queue_size_exceeded"
}

// IsConfigurationError returns whether the error is a configuration error.
func (e *QueueManagerError) IsConfigurationError() bool {
	return e.Type == "configuration" || e.Code == "invalid_config"
}
