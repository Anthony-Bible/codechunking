package service

import (
	"codechunking/internal/port/inbound"
	"context"
	"database/sql"
	"os"
	"time"
)

// GracefulShutdownService defines the interface for coordinating graceful shutdown
// across all application components with proper signal handling and resource cleanup.
type GracefulShutdownService interface {
	// Start initializes signal handlers and begins monitoring for shutdown signals.
	// Should register handlers for SIGTERM, SIGINT, and SIGHUP.
	Start(ctx context.Context) error

	// Shutdown initiates the graceful shutdown process with the configured timeout.
	// Returns error if shutdown cannot complete within timeout or critical errors occur.
	Shutdown(ctx context.Context) error

	// RegisterShutdownHook adds a component that needs cleanup during shutdown.
	// Components are shut down in reverse order of registration (LIFO).
	RegisterShutdownHook(name string, shutdownFunc ShutdownHookFunc) error

	// GetShutdownStatus returns current status of shutdown process.
	GetShutdownStatus() ShutdownStatus

	// GetShutdownMetrics returns metrics about shutdown performance and health.
	GetShutdownMetrics() ShutdownMetrics

	// IsShuttingDown returns true if shutdown process has been initiated.
	IsShuttingDown() bool

	// ForceShutdown immediately terminates all components without graceful cleanup.
	// Should only be used when graceful shutdown timeout is exceeded.
	ForceShutdown(ctx context.Context) error
}

// ShutdownHookFunc defines a function that performs cleanup for a component.
// Should return error if cleanup fails, but shutdown process continues.
type ShutdownHookFunc func(ctx context.Context) error

// ShutdownStatus represents the current state of the shutdown process.
type ShutdownStatus struct {
	IsShuttingDown    bool                      `json:"is_shutting_down"`
	ShutdownPhase     ShutdownPhase             `json:"shutdown_phase"`
	StartTime         time.Time                 `json:"start_time"`
	ElapsedTime       time.Duration             `json:"elapsed_time"`
	TimeoutRemaining  time.Duration             `json:"timeout_remaining"`
	ComponentsTotal   int                       `json:"components_total"`
	ComponentsClosed  int                       `json:"components_closed"`
	ComponentsFailed  int                       `json:"components_failed"`
	ComponentStatuses []ComponentShutdownStatus `json:"component_statuses"`
	LastError         string                    `json:"last_error,omitempty"`
	SignalReceived    os.Signal                 `json:"signal_received,omitempty"`
}

// ShutdownPhase represents the current phase of shutdown process.
type ShutdownPhase string

const (
	ShutdownPhaseIdle            ShutdownPhase = "idle"
	ShutdownPhaseDraining        ShutdownPhase = "draining"         // Stop accepting new work
	ShutdownPhaseJobCompletion   ShutdownPhase = "job_completion"   // Wait for in-flight jobs
	ShutdownPhaseResourceCleanup ShutdownPhase = "resource_cleanup" // Clean up resources
	ShutdownPhaseForceClose      ShutdownPhase = "force_close"      // Force shutdown on timeout
	ShutdownPhaseCompleted       ShutdownPhase = "completed"
	ShutdownPhaseFailed          ShutdownPhase = "failed"
)

// ComponentShutdownStatus tracks shutdown status for individual components.
type ComponentShutdownStatus struct {
	Name        string        `json:"name"`
	Status      string        `json:"status"` // "pending", "in_progress", "completed", "failed"
	StartTime   time.Time     `json:"start_time"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
	ForceKilled bool          `json:"force_killed"`
}

// ShutdownMetrics provides observability into shutdown performance and health.
type ShutdownMetrics struct {
	TotalShutdowns       int64                      `json:"total_shutdowns"`
	SuccessfulShutdowns  int64                      `json:"successful_shutdowns"`
	FailedShutdowns      int64                      `json:"failed_shutdowns"`
	ForceShutdowns       int64                      `json:"force_shutdowns"`
	AverageShutdownTime  time.Duration              `json:"average_shutdown_time"`
	LastShutdownDuration time.Duration              `json:"last_shutdown_duration"`
	TimeoutCount         int64                      `json:"timeout_count"`
	ComponentFailureRate float64                    `json:"component_failure_rate"`
	SignalCounts         map[string]int64           `json:"signal_counts"`
	ComponentMetrics     []ComponentShutdownMetrics `json:"component_metrics"`
}

// ComponentShutdownMetrics tracks performance metrics for component shutdown.
type ComponentShutdownMetrics struct {
	Name                string        `json:"name"`
	TotalShutdowns      int64         `json:"total_shutdowns"`
	SuccessfulShutdowns int64         `json:"successful_shutdowns"`
	FailedShutdowns     int64         `json:"failed_shutdowns"`
	AverageTime         time.Duration `json:"average_time"`
	MaxTime             time.Duration `json:"max_time"`
	LastError           string        `json:"last_error,omitempty"`
}

// GracefulShutdownConfig holds configuration for graceful shutdown behavior.
type GracefulShutdownConfig struct {
	// GracefulTimeout is the maximum time to wait for graceful shutdown
	GracefulTimeout time.Duration `json:"graceful_timeout"`

	// DrainTimeout is the maximum time to wait for request draining
	DrainTimeout time.Duration `json:"drain_timeout"`

	// JobCompletionTimeout is the maximum time to wait for in-flight jobs
	JobCompletionTimeout time.Duration `json:"job_completion_timeout"`

	// ResourceCleanupTimeout is the maximum time for resource cleanup
	ResourceCleanupTimeout time.Duration `json:"resource_cleanup_timeout"`

	// SignalHandlingEnabled controls whether to handle OS signals
	SignalHandlingEnabled bool `json:"signal_handling_enabled"`

	// Signals to handle for shutdown (default: SIGTERM, SIGINT)
	Signals []os.Signal `json:"signals"`

	// ComponentShutdownConcurrency controls parallel component shutdown
	ComponentShutdownConcurrency int `json:"component_shutdown_concurrency"`

	// MetricsEnabled controls shutdown metrics collection
	MetricsEnabled bool `json:"metrics_enabled"`

	// LogShutdownProgress controls detailed shutdown logging
	LogShutdownProgress bool `json:"log_shutdown_progress"`
}

// ResourceCleanupManager defines the interface for coordinating resource cleanup
// during graceful shutdown, ensuring all resources are properly released.
type ResourceCleanupManager interface {
	// RegisterResource registers a resource that needs cleanup during shutdown.
	RegisterResource(name string, resource CleanupResource) error

	// UnregisterResource removes a resource from cleanup management.
	UnregisterResource(name string) error

	// CleanupAll performs cleanup of all registered resources with timeout.
	CleanupAll(ctx context.Context) error

	// CleanupResource performs cleanup of a specific resource.
	CleanupResource(ctx context.Context, name string) error

	// GetResourceStatus returns the status of all managed resources.
	GetResourceStatus() []ResourceStatus

	// GetCleanupMetrics returns metrics about resource cleanup performance.
	GetCleanupMetrics() ResourceCleanupMetrics

	// SetCleanupOrder sets the order in which resources should be cleaned up.
	SetCleanupOrder(order []string) error

	// IsCleanupComplete returns true if all resources have been cleaned up.
	IsCleanupComplete() bool
}

// CleanupResource defines the interface for a resource that can be cleaned up.
type CleanupResource interface {
	// Cleanup performs the resource cleanup operation.
	Cleanup(ctx context.Context) error

	// GetResourceType returns the type of the resource for categorization.
	GetResourceType() ResourceType

	// GetResourceInfo returns detailed information about the resource.
	GetResourceInfo() ResourceInfo

	// CanCleanupConcurrently returns true if this resource can be cleaned up
	// concurrently with other resources.
	CanCleanupConcurrently() bool

	// GetCleanupPriority returns the priority for cleanup ordering (higher = earlier).
	GetCleanupPriority() int

	// IsHealthy returns true if the resource is in a healthy state.
	IsHealthy(ctx context.Context) bool
}

// ResourceType categorizes different types of resources for cleanup.
type ResourceType string

const (
	ResourceTypeDatabase   ResourceType = "database"
	ResourceTypeMessaging  ResourceType = "messaging"
	ResourceTypeFileSystem ResourceType = "filesystem"
	ResourceTypeNetwork    ResourceType = "network"
	ResourceTypeCache      ResourceType = "cache"
	ResourceTypeWorkerPool ResourceType = "worker_pool"
	ResourceTypeScheduler  ResourceType = "scheduler"
	ResourceTypeMetrics    ResourceType = "metrics"
	ResourceTypeUnknown    ResourceType = "unknown"
)

// ResourceInfo provides detailed information about a resource.
type ResourceInfo struct {
	Name        string            `json:"name"`
	Type        ResourceType      `json:"type"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	LastUsed    time.Time         `json:"last_used"`
	Priority    int               `json:"priority"`
	Concurrent  bool              `json:"concurrent"`
}

// ResourceStatus tracks the status of a resource during cleanup.
type ResourceStatus struct {
	Name        string        `json:"name"`
	Type        ResourceType  `json:"type"`
	Status      string        `json:"status"` // "registered", "cleaning", "cleaned", "failed"
	IsHealthy   bool          `json:"is_healthy"`
	StartTime   time.Time     `json:"start_time,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
	Error       string        `json:"error,omitempty"`
	RetryCount  int           `json:"retry_count"`
	LastRetry   time.Time     `json:"last_retry,omitempty"`
	ForceKilled bool          `json:"force_killed"`
}

// ResourceCleanupMetrics provides observability into resource cleanup performance.
type ResourceCleanupMetrics struct {
	TotalResources      int                          `json:"total_resources"`
	CleanedResources    int                          `json:"cleaned_resources"`
	FailedResources     int                          `json:"failed_resources"`
	AverageCleanupTime  time.Duration                `json:"average_cleanup_time"`
	TotalCleanupTime    time.Duration                `json:"total_cleanup_time"`
	ResourceTypeMetrics map[ResourceType]TypeMetrics `json:"resource_type_metrics"`
	ConcurrentCleanups  int                          `json:"concurrent_cleanups"`
	RetryCount          int                          `json:"retry_count"`
	ForceKillCount      int                          `json:"force_kill_count"`
	LastCleanupTime     time.Time                    `json:"last_cleanup_time"`
}

// TypeMetrics provides metrics for a specific resource type.
type TypeMetrics struct {
	Count              int           `json:"count"`
	SuccessCount       int           `json:"success_count"`
	FailureCount       int           `json:"failure_count"`
	AverageCleanupTime time.Duration `json:"average_cleanup_time"`
	MaxCleanupTime     time.Duration `json:"max_cleanup_time"`
}

// ResourceCleanupConfig holds configuration for resource cleanup behavior.
type ResourceCleanupConfig struct {
	// CleanupTimeout is the maximum time to wait for all resources to cleanup
	CleanupTimeout time.Duration `json:"cleanup_timeout"`

	// ResourceTimeout is the maximum time to wait for a single resource cleanup
	ResourceTimeout time.Duration `json:"resource_timeout"`

	// MaxConcurrentCleanups limits parallel cleanup operations
	MaxConcurrentCleanups int `json:"max_concurrent_cleanups"`

	// RetryEnabled controls whether failed cleanups should be retried
	RetryEnabled bool `json:"retry_enabled"`

	// MaxRetries is the maximum number of retry attempts for failed cleanups
	MaxRetries int `json:"max_retries"`

	// RetryDelay is the delay between retry attempts
	RetryDelay time.Duration `json:"retry_delay"`

	// ForceKillTimeout is the time after which resources are force-killed
	ForceKillTimeout time.Duration `json:"force_kill_timeout"`

	// HealthCheckEnabled controls whether resource health is checked before cleanup
	HealthCheckEnabled bool `json:"health_check_enabled"`

	// CleanupOrder specifies the order in which resource types should be cleaned
	CleanupOrder []ResourceType `json:"cleanup_order"`
}

// WorkerShutdownIntegrator defines the interface for coordinating graceful shutdown
// between the GracefulShutdownService and WorkerService components.
type WorkerShutdownIntegrator interface {
	// IntegrateWorkerService registers a worker service with graceful shutdown management.
	IntegrateWorkerService(workerService inbound.WorkerService) error

	// InitiateWorkerShutdown begins the graceful shutdown process for all registered workers.
	InitiateWorkerShutdown(ctx context.Context) error

	// GetWorkerShutdownStatus returns the current status of worker shutdown operations.
	GetWorkerShutdownStatus() WorkerShutdownStatus

	// RegisterJobCompletionHook adds a hook that waits for job completion before shutdown.
	RegisterJobCompletionHook(timeout time.Duration) error

	// ForceStopWorkers immediately stops all workers without graceful cleanup.
	ForceStopWorkers(ctx context.Context) error

	// GetActiveJobCount returns the number of jobs currently being processed.
	GetActiveJobCount() int
}

// WorkerShutdownStatus tracks the status of worker shutdown operations.
type WorkerShutdownStatus struct {
	Phase                  WorkerShutdownPhase      `json:"phase"`
	TotalWorkers           int                      `json:"total_workers"`
	StoppedWorkers         int                      `json:"stopped_workers"`
	WorkersWithErrors      int                      `json:"workers_with_errors"`
	ActiveJobs             int                      `json:"active_jobs"`
	PendingJobs            int                      `json:"pending_jobs"`
	CompletedJobs          int                      `json:"completed_jobs"`
	WorkerStatuses         []WorkerShutdownInfo     `json:"worker_statuses"`
	ConsumerStatuses       []ConsumerShutdownInfo   `json:"consumer_statuses"`
	JobProcessorStatus     JobProcessorShutdownInfo `json:"job_processor_status"`
	ShutdownStartTime      time.Time                `json:"shutdown_start_time"`
	ElapsedTime            time.Duration            `json:"elapsed_time"`
	EstimatedTimeRemaining time.Duration            `json:"estimated_time_remaining"`
	LastError              string                   `json:"last_error,omitempty"`
}

// WorkerShutdownPhase represents the current phase of worker shutdown.
type WorkerShutdownPhase string

const (
	WorkerShutdownPhaseIdle             WorkerShutdownPhase = "idle"
	WorkerShutdownPhaseDrainingNew      WorkerShutdownPhase = "draining_new"      // Stop accepting new jobs
	WorkerShutdownPhaseJobCompletion    WorkerShutdownPhase = "job_completion"    // Wait for active jobs to complete
	WorkerShutdownPhaseConsumerShutdown WorkerShutdownPhase = "consumer_shutdown" // Stop NATS consumers
	WorkerShutdownPhaseWorkerStop       WorkerShutdownPhase = "worker_stop"       // Stop worker services
	WorkerShutdownPhaseResourceCleanup  WorkerShutdownPhase = "resource_cleanup"  // Clean up resources
	WorkerShutdownPhaseCompleted        WorkerShutdownPhase = "completed"
	WorkerShutdownPhaseFailed           WorkerShutdownPhase = "failed"
	WorkerShutdownPhaseForceStop        WorkerShutdownPhase = "force_stop"
)

// WorkerShutdownInfo tracks shutdown status for individual worker services.
type WorkerShutdownInfo struct {
	WorkerID          string                            `json:"worker_id"`
	WorkerType        string                            `json:"worker_type"`
	Status            string                            `json:"status"` // "running", "draining", "stopping", "stopped", "failed"
	ActiveJobs        int                               `json:"active_jobs"`
	CompletedJobs     int64                             `json:"completed_jobs"`
	FailedJobs        int64                             `json:"failed_jobs"`
	LastJobTime       time.Time                         `json:"last_job_time"`
	ShutdownStartTime time.Time                         `json:"shutdown_start_time"`
	ShutdownDuration  time.Duration                     `json:"shutdown_duration"`
	Consumers         []inbound.ConsumerInfo            `json:"consumers"`
	Health            inbound.WorkerServiceHealthStatus `json:"health"`
	Metrics           inbound.WorkerServiceMetrics      `json:"metrics"`
	Error             string                            `json:"error,omitempty"`
}

// ConsumerShutdownInfo tracks shutdown status for NATS consumers.
type ConsumerShutdownInfo struct {
	ConsumerID        string                       `json:"consumer_id"`
	Subject           string                       `json:"subject"`
	QueueGroup        string                       `json:"queue_group"`
	Status            string                       `json:"status"` // "active", "draining", "stopped", "error"
	PendingMessages   int64                        `json:"pending_messages"`
	ProcessedMessages int64                        `json:"processed_messages"`
	LastMessageTime   time.Time                    `json:"last_message_time"`
	DrainStartTime    time.Time                    `json:"drain_start_time"`
	DrainDuration     time.Duration                `json:"drain_duration"`
	Health            inbound.ConsumerHealthStatus `json:"health"`
	Stats             inbound.ConsumerStats        `json:"stats"`
	Error             string                       `json:"error,omitempty"`
}

// JobProcessorShutdownInfo tracks job processor shutdown status.
type JobProcessorShutdownInfo struct {
	Status                string                           `json:"status"` // "active", "draining", "stopped", "error"
	ActiveJobs            int                              `json:"active_jobs"`
	QueuedJobs            int                              `json:"queued_jobs"`
	CompletedJobs         int64                            `json:"completed_jobs"`
	FailedJobs            int64                            `json:"failed_jobs"`
	AverageJobDuration    time.Duration                    `json:"average_job_duration"`
	LongestRunningJob     time.Duration                    `json:"longest_running_job"`
	ShutdownStartTime     time.Time                        `json:"shutdown_start_time"`
	DrainTimeout          time.Duration                    `json:"drain_timeout"`
	RemainingDrainTime    time.Duration                    `json:"remaining_drain_time"`
	Health                inbound.JobProcessorHealthStatus `json:"health"`
	Metrics               inbound.JobProcessorMetrics      `json:"metrics"`
	ResourceCleanupStatus ResourceCleanupStatus            `json:"resource_cleanup_status"`
	Error                 string                           `json:"error,omitempty"`
}

// ResourceCleanupStatus tracks resource cleanup for job processor.
type ResourceCleanupStatus struct {
	TempFilesDeleted          bool          `json:"temp_files_deleted"`
	WorkspaceCleared          bool          `json:"workspace_cleared"`
	DatabaseConnectionsClosed bool          `json:"database_connections_closed"`
	CachesCleared             bool          `json:"caches_cleared"`
	MetricsExported           bool          `json:"metrics_exported"`
	CleanupStartTime          time.Time     `json:"cleanup_start_time"`
	CleanupDuration           time.Duration `json:"cleanup_duration"`
	CleanupErrors             []string      `json:"cleanup_errors,omitempty"`
}

// WorkerShutdownConfig holds configuration for worker shutdown behavior.
type WorkerShutdownConfig struct {
	// DrainTimeout is the maximum time to wait for job completion
	DrainTimeout time.Duration `json:"drain_timeout"`

	// ConsumerShutdownTimeout is the maximum time to wait for consumer cleanup
	ConsumerShutdownTimeout time.Duration `json:"consumer_shutdown_timeout"`

	// WorkerStopTimeout is the maximum time to wait for worker service shutdown
	WorkerStopTimeout time.Duration `json:"worker_stop_timeout"`

	// ResourceCleanupTimeout is the maximum time for resource cleanup
	ResourceCleanupTimeout time.Duration `json:"resource_cleanup_timeout"`

	// ForceStopTimeout triggers force stop if graceful shutdown exceeds this time
	ForceStopTimeout time.Duration `json:"force_stop_timeout"`

	// JobCompletionCheckInterval controls how often to check job completion
	JobCompletionCheckInterval time.Duration `json:"job_completion_check_interval"`

	// MaxConcurrentShutdowns limits parallel worker shutdowns
	MaxConcurrentShutdowns int `json:"max_concurrent_shutdowns"`

	// PreserveLongRunningJobs controls whether to wait for long-running jobs
	PreserveLongRunningJobs bool `json:"preserve_long_running_jobs"`

	// LongRunningJobThreshold defines what constitutes a long-running job
	LongRunningJobThreshold time.Duration `json:"long_running_job_threshold"`

	// EnableProgressLogging controls detailed shutdown progress logging
	EnableProgressLogging bool `json:"enable_progress_logging"`
}

// DatabaseShutdownManager defines the interface for coordinating database cleanup during shutdown.
type DatabaseShutdownManager interface {
	// RegisterDatabase registers a database connection for shutdown management.
	RegisterDatabase(name string, db DatabaseConnection) error

	// DrainConnections gracefully drains active database connections.
	DrainConnections(ctx context.Context) error

	// CommitPendingTransactions attempts to commit all pending transactions.
	CommitPendingTransactions(ctx context.Context) error

	// RollbackActiveTransactions rolls back active transactions during emergency shutdown.
	RollbackActiveTransactions(ctx context.Context) error

	// CloseAllConnections forcibly closes all database connections.
	CloseAllConnections(ctx context.Context) error

	// GetDatabaseStatus returns the current status of database connections.
	GetDatabaseStatus() []DatabaseConnectionStatus

	// GetShutdownMetrics returns metrics about database shutdown performance.
	GetShutdownMetrics() DatabaseShutdownMetrics
}

// DatabaseConnection defines the interface for a database connection that can be shutdown.
type DatabaseConnection interface {
	// GetActiveConnections returns the number of active connections.
	GetActiveConnections() int

	// GetActiveTxns returns the number of active transactions.
	GetActiveTxns() int

	// SetMaxConnections sets the maximum number of connections allowed.
	SetMaxConnections(maxConnections int) error

	// DrainConnections gracefully drains connections over the specified timeout.
	DrainConnections(ctx context.Context, timeout time.Duration) error

	// CommitTxns attempts to commit all pending transactions.
	CommitTxns(ctx context.Context) error

	// RollbackTxns rolls back all active transactions.
	RollbackTxns(ctx context.Context) error

	// Close forcibly closes the database connection pool.
	Close() error

	// GetConnectionInfo returns detailed connection information.
	GetConnectionInfo() DatabaseConnectionInfo

	// IsHealthy returns true if the database connection is healthy.
	IsHealthy(ctx context.Context) bool
}

// DatabaseConnectionStatus tracks the status of a database connection during shutdown.
type DatabaseConnectionStatus struct {
	Name               string        `json:"name"`
	Status             string        `json:"status"` // "active", "draining", "closed", "error"
	ActiveConnections  int           `json:"active_connections"`
	MaxConnections     int           `json:"max_connections"`
	ActiveTransactions int           `json:"active_transactions"`
	PendingTxns        int           `json:"pending_txns"`
	CommittedTxns      int64         `json:"committed_txns"`
	RolledBackTxns     int64         `json:"rolled_back_txns"`
	DrainStartTime     time.Time     `json:"drain_start_time,omitempty"`
	DrainDuration      time.Duration `json:"drain_duration,omitempty"`
	LastActivity       time.Time     `json:"last_activity"`
	IsHealthy          bool          `json:"is_healthy"`
	Error              string        `json:"error,omitempty"`
}

// DatabaseConnectionInfo provides detailed information about a database connection.
type DatabaseConnectionInfo struct {
	Name            string            `json:"name"`
	Driver          string            `json:"driver"`
	DSN             string            `json:"dsn,omitempty"` // May be redacted for security
	MaxOpenConns    int               `json:"max_open_conns"`
	MaxIdleConns    int               `json:"max_idle_conns"`
	ConnMaxLifetime time.Duration     `json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration     `json:"conn_max_idle_time"`
	Stats           sql.DBStats       `json:"stats"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
}

// DatabaseShutdownMetrics provides observability into database shutdown performance.
type DatabaseShutdownMetrics struct {
	TotalDatabases         int                        `json:"total_databases"`
	SuccessfulShutdowns    int                        `json:"successful_shutdowns"`
	FailedShutdowns        int                        `json:"failed_shutdowns"`
	TotalConnectionsClosed int64                      `json:"total_connections_closed"`
	TotalTxnsCommitted     int64                      `json:"total_txns_committed"`
	TotalTxnsRolledBack    int64                      `json:"total_txns_rolled_back"`
	AverageDrainTime       time.Duration              `json:"average_drain_time"`
	MaxDrainTime           time.Duration              `json:"max_drain_time"`
	DatabaseMetrics        map[string]DatabaseMetrics `json:"database_metrics"`
	TimeoutCount           int64                      `json:"timeout_count"`
	ForceCloseCount        int64                      `json:"force_close_count"`
	LastShutdownTime       time.Time                  `json:"last_shutdown_time"`
}

// DatabaseMetrics tracks metrics for an individual database.
type DatabaseMetrics struct {
	Name                string        `json:"name"`
	ConnectionsClosed   int64         `json:"connections_closed"`
	TxnsCommitted       int64         `json:"txns_committed"`
	TxnsRolledBack      int64         `json:"txns_rolled_back"`
	DrainTime           time.Duration `json:"drain_time"`
	ShutdownAttempts    int64         `json:"shutdown_attempts"`
	SuccessfulShutdowns int64         `json:"successful_shutdowns"`
	TimeoutOccurred     bool          `json:"timeout_occurred"`
	ForceCloseOccurred  bool          `json:"force_close_occurred"`
	LastShutdownError   string        `json:"last_shutdown_error,omitempty"`
}

// NATSShutdownManager defines the interface for coordinating NATS cleanup during shutdown.
type NATSShutdownManager interface {
	// RegisterConsumer registers a NATS consumer for shutdown management.
	RegisterConsumer(name string, consumer NATSConsumer) error

	// DrainAllConsumers gracefully drains all registered NATS consumers.
	DrainAllConsumers(ctx context.Context) error

	// StopConsumers stops all consumers without draining.
	StopConsumers(ctx context.Context) error

	// FlushPendingMessages ensures all pending messages are processed or acknowledged.
	FlushPendingMessages(ctx context.Context) error

	// CloseConnections closes all NATS connections.
	CloseConnections(ctx context.Context) error

	// GetConsumerStatus returns the current status of all consumers.
	GetConsumerStatus() []NATSConsumerStatus

	// GetShutdownMetrics returns metrics about NATS shutdown performance.
	GetShutdownMetrics() NATSShutdownMetrics
}

// NATSConsumer defines the interface for a NATS consumer that can be shutdown.
type NATSConsumer interface {
	// GetPendingMessages returns the number of pending messages.
	GetPendingMessages() int64

	// GetProcessedMessages returns the number of processed messages.
	GetProcessedMessages() int64

	// Drain gracefully drains the consumer over the specified timeout.
	Drain(ctx context.Context, timeout time.Duration) error

	// Stop immediately stops the consumer.
	Stop(ctx context.Context) error

	// FlushPending ensures all pending messages are processed.
	FlushPending(ctx context.Context) error

	// Close closes the consumer connection.
	Close() error

	// GetConsumerInfo returns detailed consumer information.
	GetConsumerInfo() NATSConsumerInfo

	// IsConnected returns true if the consumer is connected to NATS.
	IsConnected() bool
}

// NATSConsumerStatus tracks the status of a NATS consumer during shutdown.
type NATSConsumerStatus struct {
	Name              string        `json:"name"`
	Subject           string        `json:"subject"`
	QueueGroup        string        `json:"queue_group"`
	Status            string        `json:"status"` // "active", "draining", "stopped", "error"
	PendingMessages   int64         `json:"pending_messages"`
	ProcessedMessages int64         `json:"processed_messages"`
	DroppedMessages   int64         `json:"dropped_messages"`
	DrainStartTime    time.Time     `json:"drain_start_time,omitempty"`
	DrainDuration     time.Duration `json:"drain_duration,omitempty"`
	LastMessageTime   time.Time     `json:"last_message_time"`
	IsConnected       bool          `json:"is_connected"`
	Error             string        `json:"error,omitempty"`
}

// NATSConsumerInfo provides detailed information about a NATS consumer.
type NATSConsumerInfo struct {
	Name              string                 `json:"name"`
	Subject           string                 `json:"subject"`
	QueueGroup        string                 `json:"queue_group"`
	DurableName       string                 `json:"durable_name"`
	ConsumerConfig    map[string]interface{} `json:"consumer_config"`
	StreamInfo        map[string]interface{} `json:"stream_info"`
	ConnectionURL     string                 `json:"connection_url"`
	Metadata          map[string]string      `json:"metadata,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	LastReconnectTime time.Time              `json:"last_reconnect_time,omitempty"`
	ReconnectCount    int64                  `json:"reconnect_count"`
}

// NATSShutdownMetrics provides observability into NATS shutdown performance.
type NATSShutdownMetrics struct {
	TotalConsumers       int                            `json:"total_consumers"`
	SuccessfulDrains     int                            `json:"successful_drains"`
	FailedDrains         int                            `json:"failed_drains"`
	TotalMessagesDrained int64                          `json:"total_messages_drained"`
	TotalMessagesDropped int64                          `json:"total_messages_dropped"`
	AverageDrainTime     time.Duration                  `json:"average_drain_time"`
	MaxDrainTime         time.Duration                  `json:"max_drain_time"`
	ConsumerMetrics      map[string]NATSConsumerMetrics `json:"consumer_metrics"`
	TimeoutCount         int64                          `json:"timeout_count"`
	ForceStopCount       int64                          `json:"force_stop_count"`
	ConnectionDrops      int64                          `json:"connection_drops"`
	LastShutdownTime     time.Time                      `json:"last_shutdown_time"`
}

// NATSConsumerMetrics tracks metrics for an individual NATS consumer.
type NATSConsumerMetrics struct {
	Name              string        `json:"name"`
	MessagesDrained   int64         `json:"messages_drained"`
	MessagesDropped   int64         `json:"messages_dropped"`
	DrainTime         time.Duration `json:"drain_time"`
	DrainAttempts     int64         `json:"drain_attempts"`
	SuccessfulDrains  int64         `json:"successful_drains"`
	TimeoutOccurred   bool          `json:"timeout_occurred"`
	ForceStopOccurred bool          `json:"force_stop_occurred"`
	ConnectionDropped bool          `json:"connection_dropped"`
	LastDrainError    string        `json:"last_drain_error,omitempty"`
}
