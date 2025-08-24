package messaging

import (
	"codechunking/internal/config"
	"codechunking/internal/domain/messaging"
	"codechunking/internal/port/inbound"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockJobProcessor mocks the job processor interface for consumer tests.
type MockJobProcessor struct {
	mock.Mock
}

func (m *MockJobProcessor) ProcessJob(ctx context.Context, message messaging.EnhancedIndexingJobMessage) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockJobProcessor) GetHealthStatus() inbound.JobProcessorHealthStatus {
	args := m.Called()
	return args.Get(0).(inbound.JobProcessorHealthStatus)
}

func (m *MockJobProcessor) GetMetrics() inbound.JobProcessorMetrics {
	args := m.Called()
	return args.Get(0).(inbound.JobProcessorMetrics)
}

func (m *MockJobProcessor) Cleanup() error {
	args := m.Called()
	return args.Error(0)
}

// Note: All types are now imported from inbound port package

// Note: NATSConsumer implementation moved to consumer.go

// TestConsumerCreation tests consumer creation with queue group configuration.
func TestConsumerCreation(t *testing.T) {
	t.Run("should create consumer with valid queue group configuration", func(t *testing.T) {
		// This test should fail initially because NATSConsumer doesn't exist yet
		consumerConfig := ConsumerConfig{
			Subject:           "indexing.job",
			QueueGroup:        "indexing-workers",
			DurableName:       "indexing-consumer",
			AckWait:           30 * time.Second,
			MaxDeliver:        3,
			MaxAckPending:     100,
			ReplayPolicy:      "instant",
			DeliverPolicy:     "all",
			RateLimitBps:      1000,
			MaxWaiting:        500,
			MaxRequestBatch:   100,
			InactiveThreshold: 5 * time.Minute,
		}

		natsConfig := config.NATSConfig{
			URL:           "nats://localhost:4222",
			MaxReconnects: 10,
			ReconnectWait: 2 * time.Second,
		}

		mockProcessor := &MockJobProcessor{}

		// Consumer should be created successfully in REFACTOR phase
		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)

		require.NoError(t, err) // Should succeed in REFACTOR phase
		assert.NotNil(t, consumer)
		assert.Equal(t, "indexing-workers", consumer.QueueGroup())
		assert.Equal(t, "indexing.job", consumer.Subject())
		assert.Equal(t, "indexing-consumer", consumer.DurableName())
	})

	t.Run("should fail with empty queue group", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "", // Invalid empty queue group
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockProcessor := &MockJobProcessor{}

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)

		require.Error(t, err)
		assert.Nil(t, consumer)
		// In RED phase, this will fail with "not implemented yet" but that's expected
	})

	t.Run("should fail with invalid subject", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:    "", // Invalid empty subject
			QueueGroup: "indexing-workers",
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockProcessor := &MockJobProcessor{}

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)

		require.Error(t, err)
		assert.Nil(t, consumer)
		// In RED phase, will fail with "not implemented yet"
	})

	t.Run("should fail with nil job processor", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "indexing-workers",
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, nil)

		require.Error(t, err)
		assert.Nil(t, consumer)
		// In RED phase, will fail with "not implemented yet"
	})
}

// TestConsumerSubscription tests message subscription and deserialization.
func TestConsumerSubscription(t *testing.T) {
	t.Run("should subscribe to subject with queue group", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "indexing-workers",
			AckWait:    30 * time.Second,
			MaxDeliver: 3,
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockProcessor := &MockJobProcessor{}

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)
		require.NotNil(t, consumer)

		ctx := context.Background()
		err = consumer.Start(ctx)

		// Should succeed in REFACTOR phase
		require.NoError(t, err)

		// Verify consumer is running
		health := consumer.Health()
		assert.True(t, health.IsRunning)
		assert.True(t, health.IsConnected)
	})

	t.Run("should deserialize enhanced indexing job message", func(t *testing.T) {
		// Create a valid enhanced message
		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "test-msg-123",
			CorrelationID: "test-corr-456",
			SchemaVersion: "2.0",
			Timestamp:     time.Now(),
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
			Priority:      messaging.JobPriorityHigh,
			RetryAttempt:  0,
			MaxRetries:    3,
			ProcessingMetadata: messaging.ProcessingMetadata{
				ChunkSizeBytes: 1024,
			},
			ProcessingContext: messaging.ProcessingContext{
				TimeoutSeconds: 300,
			},
		}

		jsonData, err := json.Marshal(message)
		require.NoError(t, err)

		consumerConfig := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "indexing-workers",
		}

		mockProcessor := &MockJobProcessor{}
		mockProcessor.On("ProcessJob", mock.Anything, mock.AnythingOfType("messaging.EnhancedIndexingJobMessage")).
			Return(nil)

		consumer := &NATSConsumer{
			config:       consumerConfig,
			jobProcessor: mockProcessor,
		}

		// Test message handling
		err = consumer.handleMessage(&nats.Msg{
			Subject: "indexing.job",
			Data:    jsonData,
		})

		// Should succeed in REFACTOR phase - message processing works
		require.NoError(t, err)
	})

	t.Run("should handle invalid message format", func(t *testing.T) {
		invalidData := []byte("invalid json")

		consumerConfig := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "indexing-workers",
		}

		consumer := &NATSConsumer{
			config: consumerConfig,
		}

		err := consumer.handleMessage(&nats.Msg{
			Subject: "indexing.job",
			Data:    invalidData,
		})

		// Should still fail in REFACTOR phase due to invalid JSON
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal message")
	})
}

// TestConsumerLifecycle tests consumer lifecycle management.
func TestConsumerLifecycle(t *testing.T) {
	t.Run("should start consumer successfully", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "indexing-workers",
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockProcessor := &MockJobProcessor{}

		consumer := &NATSConsumer{
			config:       consumerConfig,
			natsConfig:   natsConfig,
			jobProcessor: mockProcessor,
		}

		ctx := context.Background()
		err := consumer.Start(ctx)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")

		// Health should return empty status in RED phase
		health := consumer.Health()
		assert.False(t, health.IsRunning)
		assert.False(t, health.IsConnected)
	})

	t.Run("should stop consumer gracefully", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "indexing-workers",
		}

		consumer := &NATSConsumer{
			config:  consumerConfig,
			running: true,
		}

		ctx := context.Background()
		err := consumer.Stop(ctx)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should handle context cancellation during start", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "indexing-workers",
		}

		consumer := &NATSConsumer{
			config: consumerConfig,
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := consumer.Start(ctx)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})
}

// TestConsumerErrorHandling tests various error scenarios.
func TestConsumerErrorHandling(t *testing.T) {
	t.Run("should handle connection loss", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "indexing-workers",
		}

		consumer := &NATSConsumer{
			config:  consumerConfig,
			running: true,
		}

		health := consumer.Health()
		// In RED phase, should return empty health status
		assert.False(t, health.IsRunning)
		assert.False(t, health.IsConnected)
	})

	t.Run("should handle processing errors", func(t *testing.T) {
		message := messaging.EnhancedIndexingJobMessage{
			MessageID:     "test-msg-123",
			CorrelationID: "test-corr-456",
			SchemaVersion: "2.0",
			Timestamp:     time.Now(),
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/repo.git",
			Priority:      messaging.JobPriorityNormal,
		}

		jsonData, err := json.Marshal(message)
		require.NoError(t, err)

		consumerConfig := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "indexing-workers",
		}

		mockProcessor := &MockJobProcessor{}
		processingError := errors.New("processing failed")
		mockProcessor.On("ProcessJob", mock.Anything, mock.AnythingOfType("messaging.EnhancedIndexingJobMessage")).
			Return(processingError)

		consumer := &NATSConsumer{
			config:       consumerConfig,
			jobProcessor: mockProcessor,
		}

		err = consumer.handleMessage(&nats.Msg{
			Subject: "indexing.job",
			Data:    jsonData,
		})

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})
}

// TestConsumerLoadBalancing tests queue group load balancing behavior.
func TestConsumerLoadBalancing(t *testing.T) {
	t.Run("should distribute messages across queue group members", func(t *testing.T) {
		// This test verifies that messages are distributed across multiple consumers
		// in the same queue group using the load balancing mechanism

		consumerConfig1 := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "indexing-workers",
		}

		consumerConfig2 := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "indexing-workers", // Same queue group
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockProcessor1 := &MockJobProcessor{}
		mockProcessor2 := &MockJobProcessor{}

		consumer1 := &NATSConsumer{
			config:       consumerConfig1,
			natsConfig:   natsConfig,
			jobProcessor: mockProcessor1,
		}

		consumer2 := &NATSConsumer{
			config:       consumerConfig2,
			natsConfig:   natsConfig,
			jobProcessor: mockProcessor2,
		}

		ctx := context.Background()

		err1 := consumer1.Start(ctx)
		err2 := consumer2.Start(ctx)

		// Should fail in RED phase
		require.Error(t, err1)
		require.Error(t, err2)
		assert.Contains(t, err1.Error(), "not implemented yet")
		assert.Contains(t, err2.Error(), "not implemented yet")

		// Both consumers should have same queue group configured
		assert.Equal(t, "indexing-workers", consumer1.QueueGroup())
		assert.Equal(t, "indexing-workers", consumer2.QueueGroup())
	})

	t.Run("should verify queue group isolation", func(t *testing.T) {
		// Test that consumers in different queue groups don't interfere
		consumerConfig1 := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "indexing-workers",
		}

		consumerConfig2 := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "priority-workers", // Different queue group
		}

		consumer1 := &NATSConsumer{config: consumerConfig1}
		consumer2 := &NATSConsumer{config: consumerConfig2}

		assert.Equal(t, "indexing-workers", consumer1.QueueGroup())
		assert.Equal(t, "priority-workers", consumer2.QueueGroup())
		assert.NotEqual(t, consumer1.QueueGroup(), consumer2.QueueGroup())
	})
}

// TestConsumerStats tests consumer statistics collection.
func TestConsumerStats(t *testing.T) {
	t.Run("should collect message processing statistics", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "indexing-workers",
		}

		consumer := &NATSConsumer{
			config: consumerConfig,
		}

		stats := consumer.GetStats()

		// In RED phase, should return empty stats
		assert.Equal(t, int64(0), stats.MessagesReceived)
		assert.Equal(t, int64(0), stats.MessagesProcessed)
		assert.Equal(t, int64(0), stats.MessagesFailed)
		assert.True(t, stats.ActiveSince.IsZero())
	})

	t.Run("should calculate message rate", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:    "indexing.job",
			QueueGroup: "indexing-workers",
		}

		consumer := &NATSConsumer{
			config: consumerConfig,
		}

		stats := consumer.GetStats()

		// In RED phase, should return zero rate
		assert.InEpsilon(t, float64(0), stats.MessageRate, 0.001)
	})
}
