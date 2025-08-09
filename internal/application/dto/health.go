package dto

import "time"

// HealthResponse represents the response for health check endpoint
type HealthResponse struct {
	Status       string                      `json:"status"`
	Timestamp    time.Time                   `json:"timestamp"`
	Version      string                      `json:"version"`
	Dependencies map[string]DependencyStatus `json:"dependencies,omitempty"`
}

// DependencyStatus represents the status of a dependency
type DependencyStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// HealthStatus represents possible health statuses
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// DependencyStatusValue represents possible dependency statuses
type DependencyStatusValue string

const (
	DependencyStatusHealthy   DependencyStatusValue = "healthy"
	DependencyStatusUnhealthy DependencyStatusValue = "unhealthy"
)
