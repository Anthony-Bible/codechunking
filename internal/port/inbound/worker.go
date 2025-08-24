package inbound

import (
	"codechunking/internal/domain/messaging"
	"context"
	"time"
)

// WorkerService manages multiple consumers and job processing.
type WorkerService interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health() WorkerServiceHealthStatus
	GetMetrics() WorkerServiceMetrics
	AddConsumer(consumer Consumer) error
	RemoveConsumer(consumerID string) error
	GetConsumers() []ConsumerInfo
	RestartConsumer(consumerID string) error
}

// Consumer interface for message consumption.
type Consumer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health() ConsumerHealthStatus
	GetStats() ConsumerStats
	QueueGroup() string
	Subject() string
	DurableName() string
}

// WorkerServiceHealthStatus represents the health status of the worker service.
type WorkerServiceHealthStatus struct {
	IsRunning             bool                     `json:"is_running"`
	TotalConsumers        int                      `json:"total_consumers"`
	HealthyConsumers      int                      `json:"healthy_consumers"`
	UnhealthyConsumers    int                      `json:"unhealthy_consumers"`
	ConsumerHealthDetails []ConsumerHealthStatus   `json:"consumer_health_details"`
	JobProcessorHealth    JobProcessorHealthStatus `json:"job_processor_health"`
	LastHealthCheck       time.Time                `json:"last_health_check"`
	ServiceUptime         time.Duration            `json:"service_uptime"`
	LastError             string                   `json:"last_error,omitempty"`
}

// WorkerServiceMetrics represents metrics for the worker service.
type WorkerServiceMetrics struct {
	TotalMessagesProcessed int64               `json:"total_messages_processed"`
	TotalMessagesFailed    int64               `json:"total_messages_failed"`
	AverageProcessingTime  time.Duration       `json:"average_processing_time"`
	ConsumerMetrics        []ConsumerStats     `json:"consumer_metrics"`
	JobProcessorMetrics    JobProcessorMetrics `json:"job_processor_metrics"`
	ServiceStartTime       time.Time           `json:"service_start_time"`
	LastRestartTime        time.Time           `json:"last_restart_time"`
	RestartCount           int64               `json:"restart_count"`
}

// ConsumerInfo holds information about a consumer.
type ConsumerInfo struct {
	ID          string               `json:"id"`
	QueueGroup  string               `json:"queue_group"`
	Subject     string               `json:"subject"`
	DurableName string               `json:"durable_name"`
	IsRunning   bool                 `json:"is_running"`
	Health      ConsumerHealthStatus `json:"health"`
	Stats       ConsumerStats        `json:"stats"`
	StartTime   time.Time            `json:"start_time"`
}

// ConsumerHealthStatus represents the health status of a consumer.
type ConsumerHealthStatus struct {
	IsRunning       bool      `json:"is_running"`
	IsConnected     bool      `json:"is_connected"`
	LastMessageTime time.Time `json:"last_message_time"`
	MessagesHandled int64     `json:"messages_handled"`
	ErrorCount      int64     `json:"error_count"`
	LastError       string    `json:"last_error,omitempty"`
	QueueGroup      string    `json:"queue_group"`
	Subject         string    `json:"subject"`
}

// ConsumerStats holds consumer statistics.
type ConsumerStats struct {
	MessagesReceived   int64         `json:"messages_received"`
	MessagesProcessed  int64         `json:"messages_processed"`
	MessagesFailed     int64         `json:"messages_failed"`
	AverageProcessTime time.Duration `json:"average_process_time"`
	LastProcessTime    time.Duration `json:"last_process_time"`
	ActiveSince        time.Time     `json:"active_since"`
	BytesReceived      int64         `json:"bytes_received"`
	MessageRate        float64       `json:"message_rate"`
}

// JobProcessorHealthStatus represents job processor health.
type JobProcessorHealthStatus struct {
	IsReady        bool          `json:"is_ready"`
	ActiveJobs     int           `json:"active_jobs"`
	CompletedJobs  int64         `json:"completed_jobs"`
	FailedJobs     int64         `json:"failed_jobs"`
	AverageJobTime time.Duration `json:"average_job_time"`
	LastJobTime    time.Time     `json:"last_job_time"`
	ResourceUsage  ResourceUsage `json:"resource_usage"`
	LastError      string        `json:"last_error,omitempty"`
}

// JobProcessorMetrics represents job processor metrics.
type JobProcessorMetrics struct {
	TotalJobsProcessed    int64         `json:"total_jobs_processed"`
	TotalJobsFailed       int64         `json:"total_jobs_failed"`
	AverageProcessingTime time.Duration `json:"average_processing_time"`
	FilesProcessed        int64         `json:"files_processed"`
	ChunksGenerated       int64         `json:"chunks_generated"`
	EmbeddingsCreated     int64         `json:"embeddings_created"`
	BytesProcessed        int64         `json:"bytes_processed"`
	DiskUsage             int64         `json:"disk_usage"`
}

// ResourceUsage represents resource consumption metrics.
type ResourceUsage struct {
	MemoryMB     int     `json:"memory_mb"`
	CPUPercent   float64 `json:"cpu_percent"`
	DiskUsageMB  int64   `json:"disk_usage_mb"`
	NetworkInMB  int64   `json:"network_in_mb"`
	NetworkOutMB int64   `json:"network_out_mb"`
}

// JobProcessor defines the interface for processing indexing jobs.
type JobProcessor interface {
	ProcessJob(ctx context.Context, message messaging.EnhancedIndexingJobMessage) error
	GetHealthStatus() JobProcessorHealthStatus
	GetMetrics() JobProcessorMetrics
	Cleanup() error
}
