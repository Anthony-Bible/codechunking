package worker

import (
	"codechunking/internal/config"
	"codechunking/internal/domain/messaging"
	"codechunking/internal/port/inbound"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockIntegrationNATSClient mocks the NATS client for integration tests.
type MockIntegrationNATSClient struct {
	mock.Mock

	publishedMessages  []messaging.EnhancedIndexingJobMessage
	messageCallbacks   map[string]func(messaging.EnhancedIndexingJobMessage)
	queueSubscriptions map[string][]string // subject -> queue groups
	isConnected        bool
	mu                 sync.RWMutex
}

func NewMockIntegrationNATSClient() *MockIntegrationNATSClient {
	return &MockIntegrationNATSClient{
		publishedMessages:  make([]messaging.EnhancedIndexingJobMessage, 0),
		messageCallbacks:   make(map[string]func(messaging.EnhancedIndexingJobMessage)),
		queueSubscriptions: make(map[string][]string),
		isConnected:        true,
	}
}

func (m *MockIntegrationNATSClient) PublishMessage(subject string, message messaging.EnhancedIndexingJobMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	args := m.Called(subject, message)
	if args.Error(0) != nil {
		return args.Error(0)
	}

	m.publishedMessages = append(m.publishedMessages, message)

	// Simulate message delivery to subscribers
	if callbacks, exists := m.messageCallbacks[subject]; exists {
		go callbacks(message)
	}

	return nil
}

func (m *MockIntegrationNATSClient) Subscribe(
	subject, queueGroup string,
	callback func(messaging.EnhancedIndexingJobMessage),
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	args := m.Called(subject, queueGroup, callback)
	if args.Error(0) != nil {
		return args.Error(0)
	}

	m.messageCallbacks[subject] = callback

	if _, exists := m.queueSubscriptions[subject]; !exists {
		m.queueSubscriptions[subject] = make([]string, 0)
	}
	m.queueSubscriptions[subject] = append(m.queueSubscriptions[subject], queueGroup)

	return nil
}

func (m *MockIntegrationNATSClient) GetPublishedMessages() []messaging.EnhancedIndexingJobMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]messaging.EnhancedIndexingJobMessage, len(m.publishedMessages))
	copy(result, m.publishedMessages)
	return result
}

func (m *MockIntegrationNATSClient) SimulateConnectionLoss() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isConnected = false
}

func (m *MockIntegrationNATSClient) SimulateConnectionRestore() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isConnected = true
}

func (m *MockIntegrationNATSClient) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isConnected
}

// IntegrationWorkerSystem represents the complete worker system for integration testing.
type IntegrationWorkerSystem struct {
	NATSClient       *MockIntegrationNATSClient
	JobProcessor     inbound.JobProcessor
	WorkerService    WorkerService
	IndexingJobRepo  outbound.IndexingJobRepository
	RepositoryRepo   outbound.RepositoryRepository
	MessagePublisher outbound.MessagePublisher
	Config           IntegrationConfig
}

// WorkerServiceConfig holds configuration for the worker service (copied for integration tests).
type WorkerServiceConfig struct {
	Concurrency         int
	QueueGroup          string
	JobTimeout          time.Duration
	HealthCheckInterval time.Duration
	RestartDelay        time.Duration
	MaxRestartAttempts  int
	ShutdownTimeout     time.Duration
}

// WorkerServiceHealthStatus represents the health status of the worker service.
type WorkerServiceHealthStatus struct {
	IsRunning             bool                             `json:"is_running"`
	TotalConsumers        int                              `json:"total_consumers"`
	HealthyConsumers      int                              `json:"healthy_consumers"`
	UnhealthyConsumers    int                              `json:"unhealthy_consumers"`
	ConsumerHealthDetails []ConsumerHealthStatus           `json:"consumer_health_details"`
	JobProcessorHealth    inbound.JobProcessorHealthStatus `json:"job_processor_health"`
	LastHealthCheck       time.Time                        `json:"last_health_check"`
	ServiceUptime         time.Duration                    `json:"service_uptime"`
	LastError             string                           `json:"last_error,omitempty"`
}

// WorkerServiceMetrics represents metrics for the worker service.
type WorkerServiceMetrics struct {
	TotalMessagesProcessed int64                       `json:"total_messages_processed"`
	TotalMessagesFailed    int64                       `json:"total_messages_failed"`
	AverageProcessingTime  time.Duration               `json:"average_processing_time"`
	ConsumerMetrics        []ConsumerStats             `json:"consumer_metrics"`
	JobProcessorMetrics    inbound.JobProcessorMetrics `json:"job_processor_metrics"`
	ServiceStartTime       time.Time                   `json:"service_start_time"`
	LastRestartTime        time.Time                   `json:"last_restart_time"`
	RestartCount           int64                       `json:"restart_count"`
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

// Note: JobProcessor types are defined in job_processor_test.go and shared in the package

// IntegrationConfig holds configuration for integration tests.
type IntegrationConfig struct {
	WorkerConfig    WorkerServiceConfig
	NATSConfig      config.NATSConfig
	ProcessorConfig JobProcessorConfig
}

// WorkerService interface for integration tests.
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

// Consumer interface for integration tests.
type Consumer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health() ConsumerHealthStatus
	GetStats() ConsumerStats
	QueueGroup() string
	Subject() string
	DurableName() string
}

// MessageAcknowledgmentResult represents the result of message acknowledgment.
type MessageAcknowledgmentResult struct {
	MessageID   string
	Success     bool
	Error       error
	ProcessTime time.Duration
}

// DeadLetterMessage represents a message that couldn't be processed.
type DeadLetterMessage struct {
	OriginalMessage  messaging.EnhancedIndexingJobMessage
	FailureReason    string
	AttemptCount     int
	FirstFailureTime time.Time
	LastFailureTime  time.Time
}

// LoadBalancingStats tracks load balancing statistics.
type LoadBalancingStats struct {
	ConsumerMessageCounts map[string]int64
	TotalMessages         int64
	DistributionVariance  float64
}

// NewIntegrationWorkerSystem creates a new integration worker system (placeholder for RED phase).
func NewIntegrationWorkerSystem(config IntegrationConfig) (*IntegrationWorkerSystem, error) {
	return &IntegrationWorkerSystem{
		NATSClient: NewMockIntegrationNATSClient(),
		Config:     config,
	}, errors.New("IntegrationWorkerSystem not implemented yet")
}

// Placeholder methods (will be implemented in GREEN phase).
func (s *IntegrationWorkerSystem) StartSystem(_ context.Context) error {
	return errors.New("StartSystem not implemented yet")
}

func (s *IntegrationWorkerSystem) StopSystem(_ context.Context) error {
	return errors.New("StopSystem not implemented yet")
}

func (s *IntegrationWorkerSystem) PublishTestMessage(_ messaging.EnhancedIndexingJobMessage) error {
	return errors.New("PublishTestMessage not implemented yet")
}

func (s *IntegrationWorkerSystem) WaitForMessageProcessing(_ time.Duration) error {
	return errors.New("WaitForMessageProcessing not implemented yet")
}

func (s *IntegrationWorkerSystem) GetLoadBalancingStats() LoadBalancingStats {
	return LoadBalancingStats{}
}

func (s *IntegrationWorkerSystem) SimulateConsumerFailure(_ string) error {
	return errors.New("SimulateConsumerFailure not implemented yet")
}

func (s *IntegrationWorkerSystem) GetDeadLetterMessages() []DeadLetterMessage {
	return []DeadLetterMessage{}
}

func (s *IntegrationWorkerSystem) GetAcknowledgmentResults() []MessageAcknowledgmentResult {
	return []MessageAcknowledgmentResult{}
}

// TestEndToEndMessageFlow tests complete message flow from NATS to job processing.
func TestEndToEndMessageFlow(t *testing.T) {
	t.Run("should process message from NATS to repository update", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency:         3,
				QueueGroup:          "indexing-workers",
				JobTimeout:          5 * time.Minute,
				HealthCheckInterval: 10 * time.Second,
			},
			NATSConfig: config.NATSConfig{
				URL:           "nats://localhost:4222",
				MaxReconnects: 10,
				ReconnectWait: 2 * time.Second,
			},
			ProcessorConfig: JobProcessorConfig{
				WorkspaceDir:      "/tmp/integration-workspace",
				MaxConcurrentJobs: 3,
				JobTimeout:        5 * time.Minute,
			},
		}

		system, err := NewIntegrationWorkerSystem(config)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
		require.NotNil(t, system) // System should still be created for further testing

		ctx := context.Background()
		err = system.StartSystem(ctx)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should handle multiple message types", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 2,
				QueueGroup:  "indexing-workers",
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		// Create different types of messages
		messages := []messaging.EnhancedIndexingJobMessage{
			{
				MessageID:     "high-priority-msg",
				CorrelationID: "test-corr-1",
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/example/high-priority.git",
				Priority:      messaging.JobPriorityHigh,
			},
			{
				MessageID:     "normal-priority-msg",
				CorrelationID: "test-corr-2",
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/example/normal-priority.git",
				Priority:      messaging.JobPriorityNormal,
			},
			{
				MessageID:     "low-priority-msg",
				CorrelationID: "test-corr-3",
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/example/low-priority.git",
				Priority:      messaging.JobPriorityLow,
			},
		}

		for _, msg := range messages {
			err := system.PublishTestMessage(msg)
			// Should fail in RED phase
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not implemented yet")
		}
	})

	t.Run("should update indexing job status throughout processing", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 1,
				QueueGroup:  "indexing-workers",
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "status-tracking-msg",
			CorrelationID: "status-test-corr",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/status-test.git",
			Priority:      messaging.JobPriorityNormal,
		}

		err = system.PublishTestMessage(message)
		require.Error(t, err) // Should fail in RED phase

		// Wait for processing
		err = system.WaitForMessageProcessing(10 * time.Second)
		require.Error(t, err) // Should fail in RED phase
	})
}

// TestConsumerGroupLoadBalancing tests load balancing across multiple consumers.
func TestConsumerGroupLoadBalancing(t *testing.T) {
	t.Run("should distribute messages evenly across consumers", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 5, // 5 consumers
				QueueGroup:  "indexing-workers",
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		ctx := context.Background()
		err = system.StartSystem(ctx)
		require.Error(t, err) // Should fail in RED phase

		// Send 50 messages
		messageCount := 50
		for range messageCount {
			message := messaging.EnhancedIndexingJobMessage{
				MessageID:     uuid.New().String(),
				CorrelationID: uuid.New().String(),
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/example/load-test.git",
				Priority:      messaging.JobPriorityNormal,
			}

			err := system.PublishTestMessage(message)
			require.Error(t, err) // Should fail in RED phase
		}

		// Wait for all messages to be processed
		err = system.WaitForMessageProcessing(30 * time.Second)
		require.Error(t, err) // Should fail in RED phase

		// Check load balancing stats
		stats := system.GetLoadBalancingStats()

		// Should return empty stats in RED phase
		assert.Equal(t, int64(0), stats.TotalMessages)
		assert.Empty(t, stats.ConsumerMessageCounts)
	})

	t.Run("should handle consumer failure and redistribute messages", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 3,
				QueueGroup:  "indexing-workers",
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		ctx := context.Background()
		err = system.StartSystem(ctx)
		require.Error(t, err) // Should fail in RED phase

		// Simulate consumer failure
		err = system.SimulateConsumerFailure("consumer-1")
		require.Error(t, err) // Should fail in RED phase

		// Send messages and verify redistribution
		for range 20 {
			message := messaging.EnhancedIndexingJobMessage{
				MessageID:     uuid.New().String(),
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/example/failover-test.git",
			}

			err := system.PublishTestMessage(message)
			require.Error(t, err) // Should fail in RED phase
		}

		stats := system.GetLoadBalancingStats()
		// Should return empty stats in RED phase
		assert.Equal(t, int64(0), stats.TotalMessages)
	})

	t.Run("should maintain queue group isolation", func(t *testing.T) {
		config1 := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 2,
				QueueGroup:  "indexing-workers",
			},
		}

		config2 := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 2,
				QueueGroup:  "priority-workers",
			},
		}

		system1, err1 := NewIntegrationWorkerSystem(config1)
		system2, err2 := NewIntegrationWorkerSystem(config2)

		require.Error(t, err1) // Should fail in RED phase
		require.Error(t, err2) // Should fail in RED phase

		ctx := context.Background()
		err1 = system1.StartSystem(ctx)
		err2 = system2.StartSystem(ctx)

		require.Error(t, err1) // Should fail in RED phase
		require.Error(t, err2) // Should fail in RED phase

		// Send messages to both systems
		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "isolation-test",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/isolation-test.git",
		}

		err1 = system1.PublishTestMessage(message)
		err2 = system2.PublishTestMessage(message)

		require.Error(t, err1) // Should fail in RED phase
		require.Error(t, err2) // Should fail in RED phase
	})
}

// TestMessageAcknowledgmentScenarios tests message acknowledgment and redelivery.
func TestMessageAcknowledgmentScenarios(t *testing.T) {
	t.Run("should acknowledge successfully processed messages", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 2,
				QueueGroup:  "indexing-workers",
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "ack-success-test",
			CorrelationID: "ack-test-corr",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/ack-success.git",
			Priority:      messaging.JobPriorityNormal,
		}

		err = system.PublishTestMessage(message)
		require.Error(t, err) // Should fail in RED phase

		err = system.WaitForMessageProcessing(5 * time.Second)
		require.Error(t, err) // Should fail in RED phase

		// Check acknowledgment results
		results := system.GetAcknowledgmentResults()
		assert.Empty(t, results) // Should be empty in RED phase
	})

	t.Run("should handle message processing failures and retries", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 1,
				QueueGroup:  "indexing-workers",
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "retry-test-msg",
			CorrelationID: "retry-test-corr",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/nonexistent/repo.git", // Will fail
			Priority:      messaging.JobPriorityNormal,
			RetryAttempt:  0,
			MaxRetries:    3,
		}

		err = system.PublishTestMessage(message)
		require.Error(t, err) // Should fail in RED phase

		err = system.WaitForMessageProcessing(10 * time.Second)
		require.Error(t, err) // Should fail in RED phase

		results := system.GetAcknowledgmentResults()
		assert.Empty(t, results) // Should be empty in RED phase
	})

	t.Run("should handle acknowledgment timeouts", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 1,
				QueueGroup:  "indexing-workers",
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "timeout-test-msg",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/slow-repo.git",
			ProcessingContext: messaging.ProcessingContext{
				TimeoutSeconds: 1, // Very short timeout
			},
		}

		err = system.PublishTestMessage(message)
		require.Error(t, err) // Should fail in RED phase

		err = system.WaitForMessageProcessing(5 * time.Second)
		require.Error(t, err) // Should fail in RED phase

		results := system.GetAcknowledgmentResults()
		assert.Empty(t, results) // Should be empty in RED phase
	})
}

// TestDeadLetterHandling tests dead letter queue integration.
func TestDeadLetterHandling(t *testing.T) {
	t.Run("should move failed messages to dead letter queue", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 2,
				QueueGroup:  "indexing-workers",
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		// Create a message that will fail repeatedly
		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "dead-letter-test",
			CorrelationID: "dead-letter-corr",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/invalid/repo.git",
			Priority:      messaging.JobPriorityNormal,
			RetryAttempt:  0,
			MaxRetries:    2, // Will fail and exhaust retries
		}

		err = system.PublishTestMessage(message)
		require.Error(t, err) // Should fail in RED phase

		err = system.WaitForMessageProcessing(15 * time.Second)
		require.Error(t, err) // Should fail in RED phase

		// Check dead letter messages
		deadLetterMessages := system.GetDeadLetterMessages()
		assert.Empty(t, deadLetterMessages) // Should be empty in RED phase
	})

	t.Run("should track dead letter message metadata", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 1,
				QueueGroup:  "indexing-workers",
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "metadata-test",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/metadata-test.git",
			MaxRetries:    1,
		}

		err = system.PublishTestMessage(message)
		require.Error(t, err) // Should fail in RED phase

		deadLetterMessages := system.GetDeadLetterMessages()
		assert.Empty(t, deadLetterMessages) // Should be empty in RED phase
	})

	t.Run("should handle dead letter queue overflow", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 3,
				QueueGroup:  "indexing-workers",
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		// Send many failing messages
		for range 100 {
			message := messaging.EnhancedIndexingJobMessage{
				MessageID:     uuid.New().String(),
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/invalid/overflow-test.git",
				MaxRetries:    1,
			}

			err := system.PublishTestMessage(message)
			require.Error(t, err) // Should fail in RED phase
		}

		err = system.WaitForMessageProcessing(30 * time.Second)
		require.Error(t, err) // Should fail in RED phase

		deadLetterMessages := system.GetDeadLetterMessages()
		assert.Empty(t, deadLetterMessages) // Should be empty in RED phase
	})
}

// TestPerformanceScenarios tests performance under various loads.
func TestPerformanceScenarios(t *testing.T) {
	t.Run("should handle high message volume", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 10,
				QueueGroup:  "indexing-workers",
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		ctx := context.Background()
		err = system.StartSystem(ctx)
		require.Error(t, err) // Should fail in RED phase

		// Send 1000 messages
		messageCount := 1000
		startTime := time.Now()

		for range messageCount {
			message := messaging.EnhancedIndexingJobMessage{
				MessageID:     uuid.New().String(),
				CorrelationID: uuid.New().String(),
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/example/perf-test.git",
				Priority:      messaging.JobPriorityNormal,
			}

			err := system.PublishTestMessage(message)
			require.Error(t, err) // Should fail in RED phase
		}

		publishDuration := time.Since(startTime)

		// Wait for processing
		err = system.WaitForMessageProcessing(60 * time.Second)
		require.Error(t, err) // Should fail in RED phase

		totalDuration := time.Since(startTime)

		// In a real implementation, we'd check performance metrics
		t.Logf("Published %d messages in %v", messageCount, publishDuration)
		t.Logf("Total processing time: %v", totalDuration)

		stats := system.GetLoadBalancingStats()
		assert.Equal(t, int64(0), stats.TotalMessages) // Should be empty in RED phase
	})

	t.Run("should maintain performance under concurrent load", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 5,
				QueueGroup:  "indexing-workers",
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		// Concurrent publishers
		var wg sync.WaitGroup
		var totalMessages int64
		publisherCount := 5
		messagesPerPublisher := 100

		for i := range publisherCount {
			wg.Add(1)
			go func(_ int) {
				defer wg.Done()

				for range messagesPerPublisher {
					message := messaging.EnhancedIndexingJobMessage{
						MessageID:     uuid.New().String(),
						CorrelationID: uuid.New().String(),
						RepositoryID:  uuid.New(),
						RepositoryURL: "https://github.com/example/concurrent-test.git",
						Priority:      messaging.JobPriorityNormal,
					}

					err := system.PublishTestMessage(message)
					if err == nil { // In GREEN phase, this would succeed
						atomic.AddInt64(&totalMessages, 1)
					}
					// In RED phase, all will fail
					assert.Error(t, err)
				}
			}(i)
		}

		wg.Wait()

		// In RED phase, no messages should be published
		assert.Equal(t, int64(0), atomic.LoadInt64(&totalMessages))
	})

	t.Run("should handle memory pressure gracefully", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 3,
				QueueGroup:  "indexing-workers",
			},
			ProcessorConfig: JobProcessorConfig{
				MaxMemoryMB:    512,  // Limited memory
				MaxDiskUsageMB: 1024, // Limited disk
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		// Send messages with large processing contexts
		for range 50 {
			message := messaging.EnhancedIndexingJobMessage{
				MessageID:     uuid.New().String(),
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/example/large-repo.git",
				ProcessingMetadata: messaging.ProcessingMetadata{
					ChunkSizeBytes:   10240,  // Large chunks
					MaxFileSizeBytes: 102400, // Large files
				},
				ProcessingContext: messaging.ProcessingContext{
					MaxMemoryMB:      256,
					ConcurrencyLevel: 10,
				},
			}

			err := system.PublishTestMessage(message)
			require.Error(t, err) // Should fail in RED phase
		}

		err = system.WaitForMessageProcessing(30 * time.Second)
		require.Error(t, err) // Should fail in RED phase
	})
}

// TestErrorRecoveryScenarios tests error recovery and resilience.
func TestErrorRecoveryScenarios(t *testing.T) {
	t.Run("should recover from NATS connection loss", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 2,
				QueueGroup:  "indexing-workers",
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		// Simulate connection loss
		system.NATSClient.SimulateConnectionLoss()
		assert.False(t, system.NATSClient.IsConnected())

		// Try to send messages during disconnection
		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "connection-test",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/connection-test.git",
		}

		err = system.PublishTestMessage(message)
		require.Error(t, err) // Should fail in RED phase

		// Restore connection
		system.NATSClient.SimulateConnectionRestore()
		assert.True(t, system.NATSClient.IsConnected())

		// Messages should now work
		err = system.PublishTestMessage(message)
		require.Error(t, err) // Should still fail in RED phase due to implementation
	})

	t.Run("should handle database connection failures", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 2,
				QueueGroup:  "indexing-workers",
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "db-failure-test",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/db-test.git",
		}

		err = system.PublishTestMessage(message)
		require.Error(t, err) // Should fail in RED phase

		err = system.WaitForMessageProcessing(10 * time.Second)
		require.Error(t, err) // Should fail in RED phase
	})

	t.Run("should handle partial system failures", func(t *testing.T) {
		config := IntegrationConfig{
			WorkerConfig: WorkerServiceConfig{
				Concurrency: 3,
				QueueGroup:  "indexing-workers",
			},
		}

		system, err := NewIntegrationWorkerSystem(config)
		require.Error(t, err) // Should fail in RED phase

		ctx := context.Background()
		err = system.StartSystem(ctx)
		require.Error(t, err) // Should fail in RED phase

		// Simulate partial consumer failure
		err = system.SimulateConsumerFailure("consumer-2")
		require.Error(t, err) // Should fail in RED phase

		// System should continue with remaining consumers
		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "partial-failure-test",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/partial-failure.git",
		}

		err = system.PublishTestMessage(message)
		require.Error(t, err) // Should fail in RED phase
	})
}
