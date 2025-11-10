//go:build integration

package messaging

import (
	"codechunking/internal/config"
	"codechunking/internal/domain/messaging"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestNATSConsumerIntegration_ConnectionEstablishment tests that the consumer
// can establish a real connection to NATS JetStream server.
func TestNATSConsumerIntegration_ConnectionEstablishment(t *testing.T) {
	t.Run("should establish connection to NATS JetStream server on Start", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "indexing.job",
			QueueGroup:    "workers",
			DurableName:   "indexing-consumer",
			AckWait:       30 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL:           "nats://localhost:4222",
			MaxReconnects: 3,
			ReconnectWait: 2 * time.Second,
		}

		mockProcessor := &MockJobProcessor{}

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// This should fail initially because current implementation doesn't connect to NATS
		err = consumer.Start(ctx)
		require.NoError(t, err)

		// Verify real NATS connection exists
		assert.True(t, consumer.IsConnected(), "Consumer should report connected state")

		// Verify internal NATS connection object exists
		require.NotNil(t, consumer.connection, "Internal NATS connection should exist")
		assert.True(t, consumer.connection.IsConnected(), "NATS connection should be established")

		// Cleanup
		err = consumer.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("should fail gracefully when NATS server is unavailable", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "indexing.job",
			QueueGroup:    "workers",
			DurableName:   "indexing-consumer",
			AckWait:       5 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL:           "nats://localhost:9999", // Invalid port
			MaxReconnects: 1,
			ReconnectWait: 100 * time.Millisecond,
		}

		mockProcessor := &MockJobProcessor{}

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// This should fail because NATS server is unavailable
		err = consumer.Start(ctx)
		require.Error(t, err, "Start should fail when NATS server is unavailable")
		assert.Contains(t, err.Error(), "connection refused")

		// Consumer should not report connected state
		assert.False(t, consumer.IsConnected())
	})

	t.Run("should handle connection timeout during establishment", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "indexing.job",
			QueueGroup:    "workers",
			DurableName:   "indexing-consumer",
			AckWait:       30 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL:           "nats://10.255.255.1:4222", // Non-routable address
			MaxReconnects: 0,
			ReconnectWait: time.Second,
		}

		mockProcessor := &MockJobProcessor{}

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		// Should fail due to connection timeout
		err = consumer.Start(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")
	})
}

// TestNATSConsumerIntegration_StreamAndConsumerCreation tests JetStream
// stream and durable consumer creation.
func TestNATSConsumerIntegration_StreamAndConsumerCreation(t *testing.T) {
	t.Run("should create durable consumer on INDEXING stream", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "indexing.job",
			QueueGroup:    "workers",
			DurableName:   "indexing-consumer",
			AckWait:       30 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 100,
			ReplayPolicy:  "instant",
			DeliverPolicy: "all",
		}

		natsConfig := config.NATSConfig{
			URL:           "nats://localhost:4222",
			MaxReconnects: 3,
			ReconnectWait: 2 * time.Second,
		}

		mockProcessor := &MockJobProcessor{}

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx := context.Background()
		err = consumer.Start(ctx)
		require.NoError(t, err)
		defer consumer.Stop(ctx)

		// Verify JetStream context exists
		require.NotNil(t, consumer.jsContext, "JetStream context should be created")

		// Verify durable consumer was created on the stream
		consumerInfo, err := consumer.jsContext.ConsumerInfo("INDEXING", "indexing-consumer")
		require.NoError(t, err, "Durable consumer should exist on INDEXING stream")
		assert.Equal(t, "indexing-consumer", consumerInfo.Name)
		assert.Equal(t, "indexing.job", consumerInfo.Config.FilterSubject)
		assert.Equal(t, "workers", consumerInfo.Config.DeliverGroup)
	})

	t.Run("should handle stream creation if INDEXING stream doesn't exist", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "indexing.job",
			QueueGroup:    "workers",
			DurableName:   "test-consumer-" + uuid.New().String()[:8],
			AckWait:       30 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL:           "nats://localhost:4222",
			MaxReconnects: 3,
			ReconnectWait: 2 * time.Second,
		}

		mockProcessor := &MockJobProcessor{}

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx := context.Background()
		err = consumer.Start(ctx)
		require.NoError(t, err)
		defer consumer.Stop(ctx)

		// Verify INDEXING stream exists after consumer start
		streamInfo, err := consumer.jsContext.StreamInfo("INDEXING")
		require.NoError(t, err, "INDEXING stream should exist or be created")
		assert.Equal(t, "INDEXING", streamInfo.Config.Name)
		assert.Contains(t, streamInfo.Config.Subjects, "indexing.>")
	})

	t.Run("should configure consumer with correct parameters", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:           "indexing.job",
			QueueGroup:        "workers",
			DurableName:       "test-params-consumer",
			AckWait:           45 * time.Second,
			MaxDeliver:        5,
			MaxAckPending:     200,
			ReplayPolicy:      "instant",
			DeliverPolicy:     "new",
			MaxRequestBatch:   50,
			MaxRequestExpires: 10 * time.Second,
			InactiveThreshold: 5 * time.Minute,
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockProcessor := &MockJobProcessor{}

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx := context.Background()
		err = consumer.Start(ctx)
		require.NoError(t, err)
		defer consumer.Stop(ctx)

		// Verify consumer configuration matches expectations
		consumerInfo, err := consumer.jsContext.ConsumerInfo("INDEXING", "test-params-consumer")
		require.NoError(t, err)

		config := consumerInfo.Config
		assert.Equal(t, 45*time.Second, config.AckWait)
		assert.Equal(t, 5, config.MaxDeliver)
		assert.Equal(t, 200, config.MaxAckPending)
		assert.Equal(t, nats.ReplayInstantPolicy, config.ReplayPolicy)
		assert.Equal(t, nats.DeliverNewPolicy, config.DeliverPolicy)
		assert.Equal(t, 50, config.MaxRequestBatch)
		assert.Equal(t, 10*time.Second, config.MaxRequestExpires)
		assert.Equal(t, 5*time.Minute, config.InactiveThreshold)
	})
}

// TestNATSConsumerIntegration_MessageSubscriptionAndProcessing tests actual
// message subscription and processing with real NATS messages.
func TestNATSConsumerIntegration_MessageSubscriptionAndProcessing(t *testing.T) {
	t.Run("should subscribe to messages and process them via handleMessage", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "indexing.job",
			QueueGroup:    "workers",
			DurableName:   "test-subscription-consumer",
			AckWait:       30 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockProcessor := &MockJobProcessor{}
		processedMessages := make([]messaging.EnhancedIndexingJobMessage, 0)
		var processingMutex sync.Mutex

		mockProcessor.On("ProcessJob", mock.Anything, mock.AnythingOfType("messaging.EnhancedIndexingJobMessage")).
			Run(func(args mock.Arguments) {
				processingMutex.Lock()
				defer processingMutex.Unlock()
				msg := args.Get(1).(messaging.EnhancedIndexingJobMessage)
				processedMessages = append(processedMessages, msg)
			}).Return(nil)

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx := context.Background()
		err = consumer.Start(ctx)
		require.NoError(t, err)
		defer consumer.Stop(ctx)

		// Verify subscription exists
		require.NotNil(t, consumer.subscription, "NATS subscription should exist")
		assert.True(t, consumer.subscription.IsValid(), "Subscription should be valid")
		assert.Equal(t, "indexing.job", consumer.subscription.Subject)

		// Create test message
		testMessage := messaging.EnhancedIndexingJobMessage{
			MessageID:     "test-msg-" + uuid.New().String()[:8],
			CorrelationID: "test-corr-" + uuid.New().String()[:8],
			SchemaVersion: "2.0",
			Timestamp:     time.Now(),
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/test-repo.git",
			Priority:      messaging.JobPriorityNormal,
			RetryAttempt:  0,
			MaxRetries:    3,
			ProcessingMetadata: messaging.ProcessingMetadata{
				ChunkSizeBytes: 1024,
			},
			ProcessingContext: messaging.ProcessingContext{
				TimeoutSeconds: 300,
			},
		}

		// Publish message via JetStream
		msgData, err := json.Marshal(testMessage)
		require.NoError(t, err)

		_, err = consumer.jsContext.Publish("indexing.job", msgData)
		require.NoError(t, err)

		// Wait for message processing
		assert.Eventually(t, func() bool {
			processingMutex.Lock()
			defer processingMutex.Unlock()
			return len(processedMessages) > 0
		}, 5*time.Second, 100*time.Millisecond, "Message should be processed")

		// Verify the message was processed correctly
		processingMutex.Lock()
		defer processingMutex.Unlock()
		require.Len(t, processedMessages, 1)
		assert.Equal(t, testMessage.MessageID, processedMessages[0].MessageID)
		assert.Equal(t, testMessage.RepositoryURL, processedMessages[0].RepositoryURL)

		mockProcessor.AssertExpectations(t)
	})

	t.Run("should handle multiple messages in queue group", func(t *testing.T) {
		const numMessages = 5
		const numConsumers = 2

		processedCount := make([]int32, numConsumers)
		consumers := make([]*NATSConsumer, numConsumers)
		mockProcessors := make([]*MockJobProcessor, numConsumers)

		// Create multiple consumers in same queue group
		for i := range numConsumers {
			consumerConfig := ConsumerConfig{
				Subject:       "indexing.job",
				QueueGroup:    "load-balance-test",
				DurableName:   fmt.Sprintf("load-balance-consumer-%d", i),
				AckWait:       30 * time.Second,
				MaxDeliver:    3,
				MaxAckPending: 100,
			}

			natsConfig := config.NATSConfig{
				URL: "nats://localhost:4222",
			}

			mockProcessors[i] = &MockJobProcessor{}
			consumerIndex := i // Capture for closure

			mockProcessors[i].On("ProcessJob", mock.Anything, mock.AnythingOfType("messaging.EnhancedIndexingJobMessage")).
				Run(func(_ mock.Arguments) {
					processedCount[consumerIndex]++
				}).
				Return(nil)

			var err error
			consumers[i], err = NewNATSConsumer(consumerConfig, natsConfig, mockProcessors[i])
			require.NoError(t, err)

			ctx := context.Background()
			err = consumers[i].Start(ctx)
			require.NoError(t, err)

			defer consumers[i].Stop(ctx)
		}

		// Publish multiple messages
		firstConsumerJS := consumers[0].jsContext
		for msgNum := range numMessages {
			testMessage := messaging.EnhancedIndexingJobMessage{
				MessageID:     fmt.Sprintf("load-test-msg-%d", msgNum),
				CorrelationID: "load-test-correlation",
				SchemaVersion: "2.0",
				Timestamp:     time.Now(),
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/example/load-test-repo.git",
				Priority:      messaging.JobPriorityNormal,
			}

			msgData, err := json.Marshal(testMessage)
			require.NoError(t, err)

			_, err = firstConsumerJS.Publish("indexing.job", msgData)
			require.NoError(t, err)
		}

		// Wait for all messages to be processed
		assert.Eventually(t, func() bool {
			totalProcessed := int32(0)
			for i := range numConsumers {
				totalProcessed += processedCount[i]
			}
			return int(totalProcessed) == numMessages
		}, 10*time.Second, 100*time.Millisecond, "All messages should be processed")

		// Verify load balancing: each consumer should process at least one message
		totalProcessed := int32(0)
		for i := range numConsumers {
			assert.Positive(t, processedCount[i], "Consumer %d should process at least one message", i)
			totalProcessed += processedCount[i]
		}
		assert.Equal(t, int32(numMessages), totalProcessed, "Total processed messages should match sent messages")
	})

	t.Run("should handle message processing errors gracefully", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "indexing.job",
			QueueGroup:    "error-test-workers",
			DurableName:   "error-test-consumer",
			AckWait:       5 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockProcessor := &MockJobProcessor{}
		processingError := errors.New("simulated processing error")
		mockProcessor.On("ProcessJob", mock.Anything, mock.AnythingOfType("messaging.EnhancedIndexingJobMessage")).
			Return(processingError)

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx := context.Background()
		err = consumer.Start(ctx)
		require.NoError(t, err)
		defer consumer.Stop(ctx)

		// Create and publish test message
		testMessage := messaging.EnhancedIndexingJobMessage{
			MessageID:     "error-test-msg",
			CorrelationID: "error-test-corr",
			SchemaVersion: "2.0",
			Timestamp:     time.Now(),
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/error-test.git",
			Priority:      messaging.JobPriorityNormal,
		}

		msgData, err := json.Marshal(testMessage)
		require.NoError(t, err)

		_, err = consumer.jsContext.Publish("indexing.job", msgData)
		require.NoError(t, err)

		// Wait for processing attempt and verify error handling
		assert.Eventually(t, func() bool {
			health := consumer.Health()
			return health.ErrorCount > 0
		}, 10*time.Second, 100*time.Millisecond, "Error count should increase")

		health := consumer.Health()
		assert.Positive(t, health.ErrorCount, "Error count should be greater than 0")
		assert.NotEmpty(t, health.LastError, "Last error should be recorded")

		stats := consumer.GetStats()
		assert.Positive(t, stats.MessagesFailed, "Failed message count should increase")
	})
}

// TestNATSConsumerIntegration_ErrorHandlingAndReconnection tests reconnection
// logic and error recovery scenarios.
func TestNATSConsumerIntegration_ErrorHandlingAndReconnection(t *testing.T) {
	t.Run("should attempt reconnection when connection is lost", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "indexing.job",
			QueueGroup:    "reconnect-test-workers",
			DurableName:   "reconnect-test-consumer",
			AckWait:       10 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL:           "nats://localhost:4222",
			MaxReconnects: 5,
			ReconnectWait: 500 * time.Millisecond,
		}

		mockProcessor := &MockJobProcessor{}

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx := context.Background()
		err = consumer.Start(ctx)
		require.NoError(t, err)
		defer consumer.Stop(ctx)

		// Verify initial connection
		assert.True(t, consumer.IsConnected(), "Consumer should be initially connected")

		// Simulate connection loss by closing underlying connection
		consumer.connection.Close()

		// Wait for reconnection attempt
		assert.Eventually(t, func() bool {
			return consumer.IsConnected()
		}, 10*time.Second, 100*time.Millisecond, "Consumer should reconnect after connection loss")

		// Verify consumer info reflects reconnection
		info := consumer.GetConsumerInfo()
		assert.Positive(t, info.ReconnectCount, "Reconnect count should increase")
		assert.False(t, info.LastReconnectTime.IsZero(), "Last reconnect time should be set")
	})

	t.Run("should handle subscription errors and recovery", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "indexing.job",
			QueueGroup:    "sub-error-test-workers",
			DurableName:   "sub-error-test-consumer",
			AckWait:       10 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL:           "nats://localhost:4222",
			MaxReconnects: 3,
			ReconnectWait: 500 * time.Millisecond,
		}

		mockProcessor := &MockJobProcessor{}

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx := context.Background()
		err = consumer.Start(ctx)
		require.NoError(t, err)
		defer consumer.Stop(ctx)

		// Force subscription error by unsubscribing
		err = consumer.subscription.Unsubscribe()
		require.NoError(t, err)

		// Consumer should detect subscription error and attempt recovery
		assert.Eventually(t, func() bool {
			return consumer.subscription != nil && consumer.subscription.IsValid()
		}, 10*time.Second, 100*time.Millisecond, "Subscription should be recovered")
	})

	t.Run("should report unhealthy status during connection issues", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "indexing.job",
			QueueGroup:    "health-test-workers",
			DurableName:   "health-test-consumer",
			AckWait:       5 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL:           "nats://localhost:4222",
			MaxReconnects: 1,
			ReconnectWait: 100 * time.Millisecond,
		}

		mockProcessor := &MockJobProcessor{}

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx := context.Background()
		err = consumer.Start(ctx)
		require.NoError(t, err)
		defer consumer.Stop(ctx)

		// Verify healthy status initially
		health := consumer.Health()
		assert.True(t, health.IsConnected, "Consumer should be healthy initially")

		// Simulate connection problem
		consumer.connection.Close()

		// Health status should reflect connection issues
		assert.Eventually(t, func() bool {
			health := consumer.Health()
			return !health.IsConnected
		}, 5*time.Second, 100*time.Millisecond, "Consumer should report unhealthy status during connection issues")
	})
}

// TestNATSConsumerIntegration_MessageAcknowledgment tests message acknowledgment
// functionality with JetStream.
func TestNATSConsumerIntegration_MessageAcknowledgment(t *testing.T) {
	t.Run("should acknowledge messages after successful processing", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "indexing.job",
			QueueGroup:    "ack-test-workers",
			DurableName:   "ack-test-consumer",
			AckWait:       10 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockProcessor := &MockJobProcessor{}
		mockProcessor.On("ProcessJob", mock.Anything, mock.AnythingOfType("messaging.EnhancedIndexingJobMessage")).
			Return(nil) // Successful processing

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx := context.Background()
		err = consumer.Start(ctx)
		require.NoError(t, err)
		defer consumer.Stop(ctx)

		// Create and publish test message
		testMessage := messaging.EnhancedIndexingJobMessage{
			MessageID:     "ack-test-msg",
			CorrelationID: "ack-test-corr",
			SchemaVersion: "2.0",
			Timestamp:     time.Now(),
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/ack-test.git",
			Priority:      messaging.JobPriorityNormal,
		}

		msgData, err := json.Marshal(testMessage)
		require.NoError(t, err)

		_, err = consumer.jsContext.Publish("indexing.job", msgData)
		require.NoError(t, err)

		// Wait for message processing
		assert.Eventually(t, func() bool {
			stats := consumer.GetStats()
			return stats.MessagesProcessed > 0
		}, 10*time.Second, 100*time.Millisecond, "Message should be processed and acknowledged")

		// Verify message was acknowledged (no redelivery)
		time.Sleep(2 * time.Second) // Wait beyond ack wait time

		stats := consumer.GetStats()
		assert.Equal(t, int64(1), stats.MessagesProcessed, "Should process message only once")
		assert.Equal(t, int64(0), stats.MessagesFailed, "Should not have failed messages")

		// Verify consumer info shows successful acknowledgment
		consumerInfo, err := consumer.jsContext.ConsumerInfo("INDEXING", "ack-test-consumer")
		require.NoError(t, err)
		assert.Positive(t, consumerInfo.AckFloor.Consumer, "Ack floor should advance after acknowledgment")
	})

	t.Run("should not acknowledge messages after processing failure", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "nack-test.job",
			QueueGroup:    "nack-test-workers",
			DurableName:   "nack-test-consumer",
			AckWait:       2 * time.Second, // Short ack wait for faster test
			MaxDeliver:    2,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockProcessor := &MockJobProcessor{}
		processingError := errors.New("simulated processing failure")
		mockProcessor.On("ProcessJob", mock.Anything, mock.AnythingOfType("messaging.EnhancedIndexingJobMessage")).
			Return(processingError) // Processing failure

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx := context.Background()
		err = consumer.Start(ctx)
		require.NoError(t, err)
		defer consumer.Stop(ctx)

		// Create and publish test message
		testMessage := messaging.EnhancedIndexingJobMessage{
			MessageID:     "nack-test-msg",
			CorrelationID: "nack-test-corr",
			SchemaVersion: "2.0",
			Timestamp:     time.Now(),
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/nack-test.git",
			Priority:      messaging.JobPriorityNormal,
		}

		msgData, err := json.Marshal(testMessage)
		require.NoError(t, err)

		_, err = consumer.jsContext.Publish("nack-test.job", msgData)
		require.NoError(t, err)

		// Wait for message processing attempts (should be redelivered due to no ack)
		assert.Eventually(t, func() bool {
			stats := consumer.GetStats()
			return stats.MessagesReceived >= 2 // Original + redelivery
		}, 15*time.Second, 100*time.Millisecond, "Message should be redelivered due to processing failure")

		stats := consumer.GetStats()
		assert.GreaterOrEqual(
			t,
			stats.MessagesReceived,
			int64(2),
			"Should receive message multiple times due to redelivery",
		)
		assert.Positive(t, stats.MessagesFailed, "Should have failed message processing attempts")
	})

	t.Run("should handle ack timeout and redelivery", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "timeout-test.job",
			QueueGroup:    "timeout-test-workers",
			DurableName:   "timeout-test-consumer",
			AckWait:       1 * time.Second, // Very short ack wait
			MaxDeliver:    3,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockProcessor := &MockJobProcessor{}
		// Simulate slow processing that exceeds ack wait
		mockProcessor.On("ProcessJob", mock.Anything, mock.AnythingOfType("messaging.EnhancedIndexingJobMessage")).
			Run(func(_ mock.Arguments) {
				time.Sleep(3 * time.Second) // Longer than ack wait
			}).Return(nil)

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx := context.Background()
		err = consumer.Start(ctx)
		require.NoError(t, err)
		defer consumer.Stop(ctx)

		// Create and publish test message
		testMessage := messaging.EnhancedIndexingJobMessage{
			MessageID:     "timeout-test-msg",
			CorrelationID: "timeout-test-corr",
			SchemaVersion: "2.0",
			Timestamp:     time.Now(),
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/timeout-test.git",
			Priority:      messaging.JobPriorityNormal,
		}

		msgData, err := json.Marshal(testMessage)
		require.NoError(t, err)

		_, err = consumer.jsContext.Publish("timeout-test.job", msgData)
		require.NoError(t, err)

		// Wait for redelivery due to ack timeout
		assert.Eventually(t, func() bool {
			stats := consumer.GetStats()
			return stats.MessagesReceived >= 2 // Original + redelivery
		}, 15*time.Second, 100*time.Millisecond, "Message should be redelivered due to ack timeout")

		stats := consumer.GetStats()
		assert.GreaterOrEqual(
			t,
			stats.MessagesReceived,
			int64(2),
			"Should receive message multiple times due to ack timeout",
		)
	})
}

// TestNATSConsumerIntegration_GracefulShutdown tests graceful shutdown and
// resource cleanup.
func TestNATSConsumerIntegration_GracefulShutdown(t *testing.T) {
	t.Run("should drain pending messages before shutdown", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "drain-test.job",
			QueueGroup:    "drain-test-workers",
			DurableName:   "drain-test-consumer",
			AckWait:       30 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		processedMessages := make([]string, 0)
		var processingMutex sync.Mutex

		mockProcessor := &MockJobProcessor{}
		mockProcessor.On("ProcessJob", mock.Anything, mock.AnythingOfType("messaging.EnhancedIndexingJobMessage")).
			Run(func(args mock.Arguments) {
				processingMutex.Lock()
				defer processingMutex.Unlock()
				msg := args.Get(1).(messaging.EnhancedIndexingJobMessage)
				processedMessages = append(processedMessages, msg.MessageID)
				time.Sleep(100 * time.Millisecond) // Simulate processing time
			}).Return(nil)

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx := context.Background()
		err = consumer.Start(ctx)
		require.NoError(t, err)

		// Publish multiple messages before shutdown
		const numMessages = 5
		for i := range numMessages {
			testMessage := messaging.EnhancedIndexingJobMessage{
				MessageID:     fmt.Sprintf("drain-test-msg-%d", i),
				CorrelationID: "drain-test-corr",
				SchemaVersion: "2.0",
				Timestamp:     time.Now(),
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/example/drain-test.git",
				Priority:      messaging.JobPriorityNormal,
			}

			msgData, err := json.Marshal(testMessage)
			require.NoError(t, err)

			_, err = consumer.jsContext.Publish("drain-test.job", msgData)
			require.NoError(t, err)
		}

		// Start draining
		drainCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = consumer.Drain(drainCtx, 5*time.Second)
		require.NoError(t, err, "Drain should complete successfully")

		// Verify all messages were processed before drain completed
		processingMutex.Lock()
		processedCount := len(processedMessages)
		processingMutex.Unlock()

		assert.Equal(t, numMessages, processedCount, "All messages should be processed during drain")
		assert.Equal(t, int64(0), consumer.GetPendingMessages(), "No messages should be pending after drain")
	})

	t.Run("should handle drain timeout gracefully", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "drain-timeout-test.job",
			QueueGroup:    "drain-timeout-test-workers",
			DurableName:   "drain-timeout-test-consumer",
			AckWait:       30 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockProcessor := &MockJobProcessor{}
		// Simulate very slow processing that will cause drain timeout
		mockProcessor.On("ProcessJob", mock.Anything, mock.AnythingOfType("messaging.EnhancedIndexingJobMessage")).
			Run(func(_ mock.Arguments) {
				time.Sleep(10 * time.Second) // Much longer than drain timeout
			}).Return(nil)

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx := context.Background()
		err = consumer.Start(ctx)
		require.NoError(t, err)
		defer consumer.Stop(ctx)

		// Publish message that will be slow to process
		testMessage := messaging.EnhancedIndexingJobMessage{
			MessageID:     "drain-timeout-test-msg",
			CorrelationID: "drain-timeout-test-corr",
			SchemaVersion: "2.0",
			Timestamp:     time.Now(),
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/drain-timeout-test.git",
			Priority:      messaging.JobPriorityNormal,
		}

		msgData, err := json.Marshal(testMessage)
		require.NoError(t, err)

		_, err = consumer.jsContext.Publish("drain-timeout-test.job", msgData)
		require.NoError(t, err)

		// Wait for message to start processing
		time.Sleep(200 * time.Millisecond)

		// Start draining with short timeout
		drainCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err = consumer.Drain(drainCtx, 1*time.Second)
		require.Error(t, err, "Drain should timeout and return error")
		assert.Contains(t, err.Error(), "timeout", "Error should indicate timeout")
	})

	t.Run("should close all resources on Stop", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "stop-test.job",
			QueueGroup:    "stop-test-workers",
			DurableName:   "stop-test-consumer",
			AckWait:       30 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockProcessor := &MockJobProcessor{}

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx := context.Background()
		err = consumer.Start(ctx)
		require.NoError(t, err)

		// Verify resources are created
		assert.NotNil(t, consumer.connection, "NATS connection should exist")
		assert.NotNil(t, consumer.subscription, "NATS subscription should exist")
		assert.NotNil(t, consumer.jsContext, "JetStream context should exist")
		assert.True(t, consumer.IsConnected(), "Consumer should be connected")

		// Stop the consumer
		err = consumer.Stop(ctx)
		require.NoError(t, err)

		// Verify resources are cleaned up
		assert.False(t, consumer.IsConnected(), "Consumer should not be connected after stop")

		health := consumer.Health()
		assert.False(t, health.IsRunning, "Consumer should not be running after stop")
		assert.False(t, health.IsConnected, "Consumer should not be connected after stop")
	})

	t.Run("should handle Close method for complete cleanup", func(t *testing.T) {
		consumerConfig := ConsumerConfig{
			Subject:       "close-test.job",
			QueueGroup:    "close-test-workers",
			DurableName:   "close-test-consumer",
			AckWait:       30 * time.Second,
			MaxDeliver:    3,
			MaxAckPending: 100,
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockProcessor := &MockJobProcessor{}

		consumer, err := NewNATSConsumer(consumerConfig, natsConfig, mockProcessor)
		require.NoError(t, err)

		ctx := context.Background()
		err = consumer.Start(ctx)
		require.NoError(t, err)

		// Verify initial state
		assert.True(t, consumer.IsConnected())

		// Close the consumer
		err = consumer.Close()
		require.NoError(t, err)

		// Verify complete cleanup
		assert.False(t, consumer.IsConnected())
		assert.Nil(t, consumer.connection, "Connection should be nil after close")
		assert.Nil(t, consumer.subscription, "Subscription should be nil after close")

		health := consumer.Health()
		assert.False(t, health.IsRunning)
		assert.False(t, health.IsConnected)
	})
}
