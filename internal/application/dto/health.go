package dto

import "time"

// HealthResponse represents the response for health check endpoint.
type HealthResponse struct {
	Status       string                      `json:"status"`
	Timestamp    time.Time                   `json:"timestamp"`
	Version      string                      `json:"version"`
	Dependencies map[string]DependencyStatus `json:"dependencies,omitempty"`
}

// DependencyStatus represents the status of a dependency.
type DependencyStatus struct {
	Status       string                 `json:"status"`
	Message      string                 `json:"message,omitempty"`
	ResponseTime string                 `json:"response_time,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// NATSHealthDetails represents detailed NATS health information.
type NATSHealthDetails struct {
	Connected        bool                   `json:"connected"`
	Uptime           string                 `json:"uptime,omitempty"`
	Reconnects       int                    `json:"reconnects"`
	LastError        string                 `json:"last_error,omitempty"`
	JetStreamEnabled bool                   `json:"jetstream_enabled"`
	CircuitBreaker   string                 `json:"circuit_breaker"`
	MessageMetrics   NATSMessageMetrics     `json:"message_metrics"`
	ServerInfo       map[string]interface{} `json:"server_info,omitempty"`
}

// NATSMessageMetrics represents NATS message publishing metrics.
type NATSMessageMetrics struct {
	PublishedCount int64  `json:"published_count"`
	FailedCount    int64  `json:"failed_count"`
	AverageLatency string `json:"average_latency"`
}

// HealthStatus represents possible health statuses.
type HealthStatus string

const (
	// HealthStatusHealthy indicates that the service is operating normally.
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusDegraded indicates that the service is operational but with reduced functionality.
	HealthStatusDegraded HealthStatus = "degraded"
	// HealthStatusUnhealthy indicates that the service is not operational.
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// DependencyStatusValue represents possible dependency statuses.
type DependencyStatusValue string

const (
	// DependencyStatusHealthy indicates that a dependency is functioning properly.
	DependencyStatusHealthy DependencyStatusValue = "healthy"
	// DependencyStatusUnhealthy indicates that a dependency is not functioning properly.
	DependencyStatusUnhealthy DependencyStatusValue = "unhealthy"
)
