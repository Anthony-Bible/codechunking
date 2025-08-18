package testutil

import (
	"context"
	"sync"
	"time"

	"codechunking/internal/application/dto"
	"codechunking/internal/port/outbound"

	"github.com/google/uuid"
)

const (
	// Test data constants for NATS health monitoring.
	testPublishedCount    = 1000
	testMessageCount      = 5000
	testMessagesFailed    = 10
	testClusterNodes      = 150
	testReconnects        = 15
	testReconnectsAlt     = 8
	testMessageCountAlt   = 1000
	testMessagesFailedAlt = 250
	testMessageCountLarge = 2500
	testMessagesFailedLow = 125
)

// MockMessagePublisherWithHealthMonitoring provides complete NATS health monitoring capabilities for testing.
type MockMessagePublisherWithHealthMonitoring struct {
	mu sync.RWMutex

	// MessagePublisher interface
	PublishIndexingJobFunc func(ctx context.Context, repositoryID uuid.UUID, repositoryURL string) error

	// Health monitoring data
	connectionHealth outbound.MessagePublisherHealthStatus
	messageMetrics   outbound.MessagePublisherMetrics

	// Test control
	publishCalls     []PublishIndexingJobCall
	healthCheckCalls []time.Time
	errorMode        string
	simulateLatency  time.Duration
}

type PublishIndexingJobCall struct {
	Ctx           context.Context
	RepositoryID  uuid.UUID
	RepositoryURL string
	Timestamp     time.Time
}

// NewMockMessagePublisherWithHealthMonitoring creates a new mock with health monitoring.
func NewMockMessagePublisherWithHealthMonitoring() *MockMessagePublisherWithHealthMonitoring {
	return &MockMessagePublisherWithHealthMonitoring{
		connectionHealth: outbound.MessagePublisherHealthStatus{
			Connected:        true,
			Uptime:           "1h30m",
			Reconnects:       0,
			JetStreamEnabled: true,
			CircuitBreaker:   "closed",
		},
		messageMetrics: outbound.MessagePublisherMetrics{
			PublishedCount: 0,
			FailedCount:    0,
			AverageLatency: "2ms",
		},
		publishCalls:     make([]PublishIndexingJobCall, 0),
		healthCheckCalls: make([]time.Time, 0),
	}
}

// MessagePublisher interface implementation.
func (m *MockMessagePublisherWithHealthMonitoring) PublishIndexingJob(
	ctx context.Context,
	repositoryID uuid.UUID,
	repositoryURL string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the call
	m.publishCalls = append(m.publishCalls, PublishIndexingJobCall{
		Ctx:           ctx,
		RepositoryID:  repositoryID,
		RepositoryURL: repositoryURL,
		Timestamp:     time.Now(),
	})

	// Simulate latency if configured
	if m.simulateLatency > 0 {
		time.Sleep(m.simulateLatency)
	}

	// Use custom function if provided
	if m.PublishIndexingJobFunc != nil {
		return m.PublishIndexingJobFunc(ctx, repositoryID, repositoryURL)
	}

	// Update metrics based on success/failure
	if m.errorMode == "publish_failure" {
		m.messageMetrics.FailedCount++
		return &PublishError{message: "simulated publish failure"}
	}

	m.messageMetrics.PublishedCount++
	return nil
}

// MessagePublisherHealth interface implementation.
func (m *MockMessagePublisherWithHealthMonitoring) GetConnectionHealth() outbound.MessagePublisherHealthStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.healthCheckCalls = append(m.healthCheckCalls, time.Now())

	// Simulate latency for health checks
	if m.simulateLatency > 0 {
		time.Sleep(m.simulateLatency)
	}

	// Apply error modes to connection health
	switch m.errorMode {
	case "disconnected":
		m.connectionHealth.Connected = false
		m.connectionHealth.LastError = "connection lost"
		m.connectionHealth.CircuitBreaker = "open"
		m.connectionHealth.JetStreamEnabled = false
	case "high_reconnects":
		m.connectionHealth.Reconnects = 25
		m.connectionHealth.LastError = "frequent disconnections"
	case "circuit_breaker_open":
		m.connectionHealth.CircuitBreaker = "open"
		m.connectionHealth.LastError = "too many failures"
	case "jetstream_disabled":
		m.connectionHealth.JetStreamEnabled = false
		m.connectionHealth.LastError = "JetStream not available"
	}

	return m.connectionHealth
}

func (m *MockMessagePublisherWithHealthMonitoring) GetMessageMetrics() outbound.MessagePublisherMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Apply error modes to metrics
	switch m.errorMode {
	case "high_failure_rate":
		m.messageMetrics.FailedCount = 500
		m.messageMetrics.PublishedCount = 1000
		m.messageMetrics.AverageLatency = "25ms"
	case "slow_performance":
		m.messageMetrics.AverageLatency = "100ms"
	}

	return m.messageMetrics
}

// Test helper methods.
func (m *MockMessagePublisherWithHealthMonitoring) SetConnectionHealth(health outbound.MessagePublisherHealthStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectionHealth = health
}

func (m *MockMessagePublisherWithHealthMonitoring) SetMessageMetrics(metrics outbound.MessagePublisherMetrics) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messageMetrics = metrics
}

func (m *MockMessagePublisherWithHealthMonitoring) SetErrorMode(mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorMode = mode
}

func (m *MockMessagePublisherWithHealthMonitoring) SetSimulateLatency(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.simulateLatency = duration
}

func (m *MockMessagePublisherWithHealthMonitoring) GetPublishCalls() []PublishIndexingJobCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	calls := make([]PublishIndexingJobCall, len(m.publishCalls))
	copy(calls, m.publishCalls)
	return calls
}

func (m *MockMessagePublisherWithHealthMonitoring) GetHealthCheckCalls() []time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	calls := make([]time.Time, len(m.healthCheckCalls))
	copy(calls, m.healthCheckCalls)
	return calls
}

func (m *MockMessagePublisherWithHealthMonitoring) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.publishCalls = make([]PublishIndexingJobCall, 0)
	m.healthCheckCalls = make([]time.Time, 0)
	m.errorMode = ""
	m.simulateLatency = 0
	m.connectionHealth = outbound.MessagePublisherHealthStatus{
		Connected:        true,
		Uptime:           "1h30m",
		Reconnects:       0,
		JetStreamEnabled: true,
		CircuitBreaker:   "closed",
	}
	m.messageMetrics = outbound.MessagePublisherMetrics{
		PublishedCount: 0,
		FailedCount:    0,
		AverageLatency: "2ms",
	}
}

// PublishError represents a publishing error for testing.
type PublishError struct {
	message string
}

func (e *PublishError) Error() string {
	return e.message
}

// NATSHealthResponseBuilder helps create complex NATS health responses for testing.
type NATSHealthResponseBuilder struct {
	natsHealth dto.NATSHealthDetails
}

func NewNATSHealthResponseBuilder() *NATSHealthResponseBuilder {
	return &NATSHealthResponseBuilder{
		natsHealth: dto.NATSHealthDetails{
			Connected:        true,
			Uptime:           "1h30m",
			Reconnects:       0,
			LastError:        "",
			JetStreamEnabled: true,
			CircuitBreaker:   "closed",
			MessageMetrics: dto.NATSMessageMetrics{
				PublishedCount: testPublishedCount,
				FailedCount:    0,
				AverageLatency: "2ms",
			},
			ServerInfo: make(map[string]interface{}),
		},
	}
}

func (b *NATSHealthResponseBuilder) WithConnection(connected bool) *NATSHealthResponseBuilder {
	b.natsHealth.Connected = connected
	return b
}

func (b *NATSHealthResponseBuilder) WithUptime(uptime string) *NATSHealthResponseBuilder {
	b.natsHealth.Uptime = uptime
	return b
}

func (b *NATSHealthResponseBuilder) WithReconnects(count int) *NATSHealthResponseBuilder {
	b.natsHealth.Reconnects = count
	return b
}

func (b *NATSHealthResponseBuilder) WithLastError(error string) *NATSHealthResponseBuilder {
	b.natsHealth.LastError = error
	return b
}

func (b *NATSHealthResponseBuilder) WithJetStream(enabled bool) *NATSHealthResponseBuilder {
	b.natsHealth.JetStreamEnabled = enabled
	return b
}

func (b *NATSHealthResponseBuilder) WithCircuitBreaker(state string) *NATSHealthResponseBuilder {
	b.natsHealth.CircuitBreaker = state
	return b
}

func (b *NATSHealthResponseBuilder) WithMessageMetrics(
	published, failed int64,
	avgLatency string,
) *NATSHealthResponseBuilder {
	b.natsHealth.MessageMetrics = dto.NATSMessageMetrics{
		PublishedCount: published,
		FailedCount:    failed,
		AverageLatency: avgLatency,
	}
	return b
}

func (b *NATSHealthResponseBuilder) WithServerInfo(info map[string]interface{}) *NATSHealthResponseBuilder {
	b.natsHealth.ServerInfo = info
	return b
}

func (b *NATSHealthResponseBuilder) WithServerVersion(version string) *NATSHealthResponseBuilder {
	if b.natsHealth.ServerInfo == nil {
		b.natsHealth.ServerInfo = make(map[string]interface{})
	}
	b.natsHealth.ServerInfo["version"] = version
	return b
}

func (b *NATSHealthResponseBuilder) WithClusterInfo(clusterName string, connections int) *NATSHealthResponseBuilder {
	if b.natsHealth.ServerInfo == nil {
		b.natsHealth.ServerInfo = make(map[string]interface{})
	}
	b.natsHealth.ServerInfo["cluster"] = clusterName
	b.natsHealth.ServerInfo["connections"] = connections
	return b
}

func (b *NATSHealthResponseBuilder) Build() dto.NATSHealthDetails {
	return b.natsHealth
}

// Enhanced HealthResponseBuilder that supports NATS details.
type EnhancedHealthResponseBuilder struct {
	response dto.HealthResponse
}

func NewEnhancedHealthResponseBuilder() *EnhancedHealthResponseBuilder {
	return &EnhancedHealthResponseBuilder{
		response: dto.HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now(),
			Version:   "1.0.0",
		},
	}
}

func (b *EnhancedHealthResponseBuilder) WithStatus(status string) *EnhancedHealthResponseBuilder {
	b.response.Status = status
	return b
}

func (b *EnhancedHealthResponseBuilder) WithVersion(version string) *EnhancedHealthResponseBuilder {
	b.response.Version = version
	return b
}

func (b *EnhancedHealthResponseBuilder) WithNATSHealth(
	natsHealth dto.NATSHealthDetails,
	status, message, responseTime string,
) *EnhancedHealthResponseBuilder {
	if b.response.Dependencies == nil {
		b.response.Dependencies = make(map[string]dto.DependencyStatus)
	}

	b.response.Dependencies["nats"] = dto.DependencyStatus{
		Status:       status,
		Message:      message,
		ResponseTime: responseTime,
		Details: map[string]interface{}{
			"nats_health": natsHealth,
		},
	}
	return b
}

func (b *EnhancedHealthResponseBuilder) WithDatabaseHealth(
	status, message, responseTime string,
) *EnhancedHealthResponseBuilder {
	if b.response.Dependencies == nil {
		b.response.Dependencies = make(map[string]dto.DependencyStatus)
	}

	b.response.Dependencies["database"] = dto.DependencyStatus{
		Status:       status,
		Message:      message,
		ResponseTime: responseTime,
	}
	return b
}

func (b *EnhancedHealthResponseBuilder) BuildEnhanced() dto.HealthResponse {
	return b.response
}

// Test scenario builders for common NATS health scenarios.
func CreateHealthyNATSScenario() dto.HealthResponse {
	natsHealth := NewNATSHealthResponseBuilder().
		WithConnection(true).
		WithUptime("2h15m30s").
		WithReconnects(1).
		WithJetStream(true).
		WithCircuitBreaker("closed").
		WithMessageMetrics(testMessageCount, testMessagesFailed, "1.8ms").
		WithServerVersion("2.9.0").
		WithClusterInfo("production", testClusterNodes).
		Build()

	return NewEnhancedHealthResponseBuilder().
		WithStatus("healthy").
		WithVersion("1.0.0").
		WithDatabaseHealth("healthy", "Connected to PostgreSQL", "3ms").
		WithNATSHealth(natsHealth, "healthy", "Connected to NATS server", "2ms").
		BuildEnhanced()
}

func CreateUnhealthyNATSScenario() dto.HealthResponse {
	natsHealth := NewNATSHealthResponseBuilder().
		WithConnection(false).
		WithUptime("0s").
		WithReconnects(testReconnects).
		WithLastError("connection timeout").
		WithJetStream(false).
		WithCircuitBreaker("open").
		WithMessageMetrics(testMessageCountAlt, testMessagesFailedAlt, "0ms").
		Build()

	return NewEnhancedHealthResponseBuilder().
		WithStatus("unhealthy").
		WithVersion("1.0.0").
		WithDatabaseHealth("unhealthy", "Database connection failed", "timeout").
		WithNATSHealth(natsHealth, "unhealthy", "NATS server disconnected", "timeout").
		BuildEnhanced()
}

func CreateDegradedNATSScenario() dto.HealthResponse {
	natsHealth := NewNATSHealthResponseBuilder().
		WithConnection(true).
		WithUptime("45m").
		WithReconnects(testReconnectsAlt).
		WithLastError("temporary network issue").
		WithJetStream(true).
		WithCircuitBreaker("half-open").
		WithMessageMetrics(testMessageCountLarge, testMessagesFailedLow, "12ms").
		WithServerVersion("2.9.0").
		Build()

	return NewEnhancedHealthResponseBuilder().
		WithStatus("degraded").
		WithVersion("1.0.0").
		WithDatabaseHealth("healthy", "Connected to PostgreSQL", "3ms").
		WithNATSHealth(natsHealth, "unhealthy", "NATS connection unstable - high reconnect rate", "12ms").
		BuildEnhanced()
}
