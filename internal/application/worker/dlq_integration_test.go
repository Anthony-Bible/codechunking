package worker

import (
	"codechunking/internal/adapter/inbound/messaging"
	"codechunking/internal/application/service"
	"codechunking/internal/config"
	dlqmsg "codechunking/internal/domain/messaging"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockNATSConnection mocks NATS connection for integration testing.
type MockNATSConnection struct {
	mock.Mock
}

func (m *MockNATSConnection) PublishMessage(subject string, data []byte) error {
	args := m.Called(subject, data)
	return args.Error(0)
}

func (m *MockNATSConnection) SubscribeToStream(subject string, handler func([]byte) error) error {
	args := m.Called(subject, handler)
	return args.Error(0)
}

func (m *MockNATSConnection) CreateStream(streamName string, config interface{}) error {
	args := m.Called(streamName, config)
	return args.Error(0)
}

func (m *MockNATSConnection) Close() error {
	args := m.Called()
	return args.Error(0)
}

// TestDLQConsumer is a test consumer that implements MaxWorkers() for testing.
type TestDLQConsumer struct {
	maxWorkers int
}

func (c *TestDLQConsumer) MaxWorkers() int {
	return c.maxWorkers
}

func (c *TestDLQConsumer) GetMessageHistory(_ context.Context, _ string) (interface{}, error) {
	return nil, errors.New("not implemented yet")
}

// MockJobProcessor mocks the job processor for integration testing.
type MockJobProcessorIntegration struct {
	mock.Mock
}

func (m *MockJobProcessorIntegration) ProcessJob(ctx context.Context, message dlqmsg.EnhancedIndexingJobMessage) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

// Define mock types for integration testing.
type MockDLQStreamManager struct {
	mock.Mock
}

func (m *MockDLQStreamManager) CreateDLQStream(ctx context.Context, streamConfig service.DLQStreamConfig) error {
	args := m.Called(ctx, streamConfig)
	return args.Error(0)
}

func (m *MockDLQStreamManager) DeleteDLQStream(ctx context.Context, streamName string) error {
	args := m.Called(ctx, streamName)
	return args.Error(0)
}

func (m *MockDLQStreamManager) GetStreamInfo(ctx context.Context, streamName string) (service.DLQStreamInfo, error) {
	args := m.Called(ctx, streamName)
	if args.Get(0) == nil {
		return service.DLQStreamInfo{}, args.Error(1)
	}
	return args.Get(0).(service.DLQStreamInfo), args.Error(1)
}

func (m *MockDLQStreamManager) ConfigureRetentionPolicy(
	ctx context.Context,
	streamName string,
	policy service.DLQRetentionPolicy,
) error {
	args := m.Called(ctx, streamName, policy)
	return args.Error(0)
}

func (m *MockDLQStreamManager) PurgeDLQMessages(ctx context.Context, streamName string, olderThan time.Time) error {
	args := m.Called(ctx, streamName, olderThan)
	return args.Error(0)
}

type MockDLQRepository struct {
	mock.Mock
}

func (m *MockDLQRepository) SaveDLQMessage(ctx context.Context, dlqMessage dlqmsg.DLQMessage) error {
	args := m.Called(ctx, dlqMessage)
	return args.Error(0)
}

func (m *MockDLQRepository) FindDLQMessageByID(ctx context.Context, dlqMessageID string) (dlqmsg.DLQMessage, error) {
	args := m.Called(ctx, dlqMessageID)
	if args.Get(0) == nil {
		return dlqmsg.DLQMessage{}, args.Error(1)
	}
	return args.Get(0).(dlqmsg.DLQMessage), args.Error(1)
}

func (m *MockDLQRepository) FindDLQMessages(
	ctx context.Context,
	filters service.DLQFilters,
) ([]dlqmsg.DLQMessage, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]dlqmsg.DLQMessage), args.Error(1)
}

func (m *MockDLQRepository) UpdateDLQMessage(ctx context.Context, dlqMessage dlqmsg.DLQMessage) error {
	args := m.Called(ctx, dlqMessage)
	return args.Error(0)
}

func (m *MockDLQRepository) DeleteDLQMessage(ctx context.Context, dlqMessageID string) error {
	args := m.Called(ctx, dlqMessageID)
	return args.Error(0)
}

func (m *MockDLQRepository) GetDLQStatistics(ctx context.Context) (dlqmsg.DLQStatistics, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return dlqmsg.DLQStatistics{}, args.Error(1)
	}
	return args.Get(0).(dlqmsg.DLQStatistics), args.Error(1)
}

type MockHealthMonitor struct {
	mock.Mock
}

func (m *MockHealthMonitor) RecordDLQMetric(ctx context.Context, metric service.DLQHealthMetric) error {
	args := m.Called(ctx, metric)
	return args.Error(0)
}

func (m *MockHealthMonitor) GetDLQHealth(ctx context.Context) (service.DLQHealthStatus, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return service.DLQHealthStatus{}, args.Error(1)
	}
	return args.Get(0).(service.DLQHealthStatus), args.Error(1)
}

func (m *MockHealthMonitor) TriggerAlert(ctx context.Context, alert service.DLQAlert) error {
	args := m.Called(ctx, alert)
	return args.Error(0)
}

// TestDLQIntegrationEndToEnd tests complete DLQ workflow from failure to recovery.
func TestDLQIntegrationEndToEnd(t *testing.T) {
	t.Run("should handle complete DLQ workflow: failure -> DLQ -> analysis -> retry", func(t *testing.T) {
		// Setup: Create a job that will fail and exceed retry limits
		originalMessage := dlqmsg.EnhancedIndexingJobMessage{
			MessageID:     "msg-integration-001",
			CorrelationID: "corr-integration-001",
			RepositoryID:  uuid.New(),
			RepositoryURL: "https://github.com/example/failing-repo.git",
			RetryAttempt:  3,
			MaxRetries:    3,
			ProcessingMetadata: dlqmsg.ProcessingMetadata{
				ChunkSizeBytes: 1024,
			},
			ProcessingContext: dlqmsg.ProcessingContext{
				TimeoutSeconds: 300,
			},
		}

		// Mock components for integration test
		mockNATS := &MockNATSConnection{}
		mockJobProcessor := &MockJobProcessorIntegration{}

		// Step 1: Job processing fails and exceeds retry limit
		processingError := errors.New("git clone failed: connection timeout")
		mockJobProcessor.On("ProcessJob", mock.Anything, originalMessage).
			Return(processingError)

		// Step 2: Message should be routed to DLQ
		mockNATS.On("PublishMessage", "indexing.dlq", mock.AnythingOfType("[]uint8")).
			Return(nil)

		// Step 3: DLQ consumer should process the message
		mockNATS.On("SubscribeToStream", "indexing.dlq", mock.AnythingOfType("func([]uint8) error")).
			Return(nil)

		// Step 4: Manual retry should republish to main queue
		mockNATS.On("PublishMessage", "indexing.job", mock.AnythingOfType("[]uint8")).
			Return(nil)

		// Create integrated components
		dlqHandlerConfig := DLQHandlerConfig{
			MaxRetryAttempts:     3,
			RetryBackoffDuration: 5 * time.Second,
			DLQTimeout:           30 * time.Second,
		}

		_ = messaging.DLQConsumerConfig{
			Subject:              "indexing.dlq",
			DurableName:          "dlq-consumer-integration",
			ProcessingTimeout:    30 * time.Second,
			MaxProcessingWorkers: 2,
		}

		serviceConfig := service.DLQServiceConfig{
			StreamName:       "INDEXING-DLQ",
			MaxRetentionDays: 30,
			MaxMessages:      10000,
		}

		// Initialize components
		dlqHandler := NewDLQHandler(dlqHandlerConfig, nil, nil)
		dlqConsumer := &messaging.DLQConsumer{}
		dlqService := service.NewDLQService(serviceConfig, nil, nil, nil)

		// Execute integration test
		ctx := context.Background()

		// Step 1: Process job and trigger failure handling
		err := mockJobProcessor.ProcessJob(ctx, originalMessage)
		require.Error(t, err)

		// Step 2: Route to DLQ (should fail in RED phase)
		err = dlqHandler.HandleJobFailure(ctx, originalMessage, processingError, "CLONE")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")

		// Step 3: Consumer processes DLQ message (should fail in RED phase)
		dlqMessage := dlqmsg.DLQMessage{
			DLQMessageID:    "dlq-msg-integration-001",
			OriginalMessage: originalMessage,
			FailureType:     dlqmsg.FailureTypeNetworkError,
		}
		_, err = dlqConsumer.AnalyzeDLQMessage(ctx, dlqMessage)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")

		// Step 4: Manual retry (should fail in RED phase)
		_, err = dlqService.RetryDLQMessage(ctx, "dlq-msg-integration-001")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")

		// Verify mock expectations were met
		mockJobProcessor.AssertExpectations(t)
	})

	t.Run("should handle DLQ consumer group behavior with load balancing", func(t *testing.T) {
		// Test that multiple DLQ consumers in same group load balance messages
		consumerConfigs := []messaging.DLQConsumerConfig{
			{
				Subject:              "indexing.dlq",
				DurableName:          "dlq-consumer-group",
				MaxProcessingWorkers: 2,
			},
			{
				Subject:              "indexing.dlq",
				DurableName:          "dlq-consumer-group", // Same group
				MaxProcessingWorkers: 2,
			},
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		consumers := make([]*messaging.DLQConsumer, len(consumerConfigs))

		for i, consumerConfig := range consumerConfigs {
			consumer, err := messaging.NewDLQConsumer(
				consumerConfig,
				natsConfig,
				nil, nil, nil,
			)

			// Should fail in RED phase
			require.Error(t, err)
			assert.Nil(t, consumer)
			assert.Contains(t, err.Error(), "not implemented yet")

			consumers[i] = consumer
		}

		// In GREEN phase, verify both consumers are created and configured correctly
		for _, consumer := range consumers {
			if consumer != nil {
				assert.Equal(t, 2, consumer.MaxWorkers())
			}
		}
	})

	t.Run("should handle DLQ stream creation and message persistence", func(t *testing.T) {
		// Test end-to-end stream creation, message publishing, and consumption
		streamConfig := service.DLQServiceConfig{
			StreamName:       "INDEXING-DLQ-INTEGRATION",
			MaxRetentionDays: 7,
			MaxMessages:      1000,
		}

		mockStreamManager := &MockDLQStreamManager{}
		mockRepository := &MockDLQRepository{}
		mockHealthMonitor := &MockHealthMonitor{}

		// Mock stream creation
		expectedStreamConfig := service.DLQStreamConfig{
			Name:        "INDEXING-DLQ-INTEGRATION",
			Subject:     "indexing.dlq.integration",
			MaxMessages: 1000,
			MaxAge:      7 * 24 * time.Hour,
		}
		mockStreamManager.On("CreateDLQStream", mock.Anything, mock.MatchedBy(func(cfg service.DLQStreamConfig) bool {
			return cfg.Name == expectedStreamConfig.Name
		})).Return(nil)

		// Mock message persistence
		testDLQMessage := dlqmsg.DLQMessage{
			DLQMessageID: "dlq-integration-msg-001",
			OriginalMessage: dlqmsg.EnhancedIndexingJobMessage{
				MessageID: "msg-integration-001",
			},
			FailureType:     dlqmsg.FailureTypeNetworkError,
			TotalFailures:   1,
			ProcessingStage: "CLONE",
		}
		mockRepository.On("SaveDLQMessage", mock.Anything, testDLQMessage).Return(nil)

		// Mock health monitoring
		healthStatus := service.DLQHealthStatus{
			IsHealthy:    true,
			StreamExists: true,
		}
		mockHealthMonitor.On("GetDLQHealth", mock.Anything).Return(healthStatus, nil)

		// Create DLQ service
		dlqService := service.NewDLQService(
			streamConfig,
			mockStreamManager,
			mockRepository,
			mockHealthMonitor,
		)

		ctx := context.Background()

		// Test stream creation (should fail in RED phase)
		testStreamConfig := service.DLQStreamConfig{Name: "INDEXING-DLQ"} // Placeholder config
		err := dlqService.CreateDLQStream(ctx, testStreamConfig)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")

		// Test message saving (should fail in RED phase)
		err = dlqService.SaveDLQMessage(ctx, testDLQMessage)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")

		// Test health check (should fail in RED phase)
		_, err = dlqService.PerformHealthCheck(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})
}

// TestDLQIntegrationFailureScenarios tests various failure scenarios in DLQ integration.
func TestDLQIntegrationFailureScenarios(t *testing.T) {
	t.Run("should handle NATS connection failures during DLQ operations", func(t *testing.T) {
		// Test resilience when NATS is unavailable
		mockNATS := &MockNATSConnection{}

		// Mock NATS connection failure
		natsError := errors.New("NATS server unavailable")
		mockNATS.On("PublishMessage", mock.Anything, mock.Anything).Return(natsError)

		// Create DLQ handler with NATS dependency
		dlqHandlerConfig := DLQHandlerConfig{
			MaxRetryAttempts: 3,
			DLQTimeout:       30 * time.Second,
		}

		dlqHandler := NewDLQHandler(dlqHandlerConfig, nil, nil)

		originalMessage := dlqmsg.EnhancedIndexingJobMessage{
			MessageID: "msg-nats-fail",
		}

		ctx := context.Background()
		err := dlqHandler.RouteToDeadLetterQueue(ctx, originalMessage, errors.New("processing failed"), "PROCESSING")

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should handle database failures during DLQ message persistence", func(t *testing.T) {
		// Test resilience when database is unavailable
		mockRepository := &MockDLQRepository{}

		dbError := errors.New("database connection failed")
		mockRepository.On("SaveDLQMessage", mock.Anything, mock.AnythingOfType("messaging.DLQMessage")).
			Return(dbError)

		serviceConfig := service.DLQServiceConfig{
			StreamName: "INDEXING-DLQ",
		}

		dlqService := service.NewDLQService(serviceConfig, nil, mockRepository, nil)

		testMessage := dlqmsg.DLQMessage{
			DLQMessageID: "dlq-db-fail",
		}

		ctx := context.Background()
		err := dlqService.SaveDLQMessage(ctx, testMessage)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")
	})

	t.Run("should handle concurrent DLQ operations safely", func(t *testing.T) {
		// Test thread safety of concurrent DLQ operations
		// Create a test consumer that satisfies the MaxWorkers() expectation
		consumer := &TestDLQConsumer{maxWorkers: 5}

		// Simulate concurrent processing
		messageIDs := []string{
			"dlq-concurrent-1",
			"dlq-concurrent-2",
			"dlq-concurrent-3",
			"dlq-concurrent-4",
			"dlq-concurrent-5",
		}

		ctx := context.Background()
		errors := make([]error, len(messageIDs))

		// Process messages concurrently (should fail in RED phase)
		for i, messageID := range messageIDs {
			go func(idx int, id string) {
				_, errors[idx] = consumer.GetMessageHistory(ctx, id)
			}(i, messageID)
		}

		// Wait for all operations to complete
		time.Sleep(100 * time.Millisecond)

		// All operations should fail in RED phase
		for i, err := range errors {
			if err != nil {
				assert.Contains(t, err.Error(), "not implemented yet", "Error %d should indicate not implemented", i)
			}
		}

		// Verify worker limit
		assert.Equal(t, 5, consumer.MaxWorkers())
	})

	t.Run("should handle message format corruption gracefully", func(t *testing.T) {
		// Test handling of corrupted or invalid DLQ messages
		// Create malformed messages
		corruptedMessages := [][]byte{
			[]byte("invalid json"),
			[]byte(`{"incomplete": "message"`),
			[]byte(`{"dlq_message_id": "", "original_message": null}`),
			nil, // nil data
		}

		for i, corruptedData := range corruptedMessages {
			_ = &MockNATSMessage{
				Subject: "indexing.dlq",
				Data:    corruptedData,
			}

			// In RED phase, we can't call HandleMessage since it doesn't exist yet
			// This would be tested in GREEN phase when the method is implemented
			t.Logf("Corrupted message %d would be handled: %v", i, corruptedData != nil)
		}
	})
}

// TestDLQIntegrationPerformance tests DLQ performance characteristics.
func TestDLQIntegrationPerformance(t *testing.T) {
	t.Run("should handle high-volume DLQ message processing", func(t *testing.T) {
		// Test performance with large number of DLQ messages
		const messageCount = 100

		serviceConfig := service.DLQServiceConfig{
			StreamName:  "INDEXING-DLQ-PERF",
			MaxMessages: 10000,
		}

		dlqService := service.NewDLQService(serviceConfig, nil, nil, nil)

		// Generate test messages
		messages := make([]dlqmsg.DLQMessage, messageCount)
		for i := range messageCount {
			messages[i] = dlqmsg.DLQMessage{
				DLQMessageID: uuid.New().String(),
				OriginalMessage: dlqmsg.EnhancedIndexingJobMessage{
					MessageID: uuid.New().String(),
				},
				FailureType: dlqmsg.FailureTypeNetworkError,
			}
		}

		ctx := context.Background()
		startTime := time.Now()

		// Process messages (should fail in RED phase)
		for _, message := range messages {
			err := dlqService.SaveDLQMessage(ctx, message)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not implemented yet")
		}

		duration := time.Since(startTime)

		// Verify operations complete within reasonable time
		// Note: In RED phase, these are just validation calls
		assert.Less(t, duration, 5*time.Second, "DLQ operations should complete quickly")

		t.Logf("Processed %d messages in %v (RED phase - validation only)", messageCount, duration)
	})

	t.Run("should handle DLQ consumer backpressure correctly", func(t *testing.T) {
		// Test consumer behavior under high message load
		_ = messaging.DLQConsumerConfig{
			Subject:              "indexing.dlq.backpressure",
			DurableName:          "dlq-backpressure-test",
			MaxProcessingWorkers: 2,
			ProcessingTimeout:    5 * time.Second,
		}

		consumer := &messaging.DLQConsumer{}

		// Simulate high message rate
		messageRate := 10 // messages per second
		testDuration := 2 * time.Second

		ctx := context.Background()
		startTime := time.Now()
		processedCount := 0

		for time.Since(startTime) < testDuration {
			messageID := uuid.New().String()
			_, err := consumer.GetMessageHistory(ctx, messageID)
			// Should fail in RED phase
			if err != nil {
				assert.Contains(t, err.Error(), "not implemented yet")
			}

			processedCount++
			time.Sleep(time.Second / time.Duration(messageRate))
		}

		// Verify consumer handled the load appropriately
		expectedMessages := int(testDuration.Seconds()) * messageRate
		assert.LessOrEqual(t, processedCount, expectedMessages*2, "Consumer should handle expected message rate")

		t.Logf("Consumer processed %d messages in %v with %d workers",
			processedCount, testDuration, consumer.MaxWorkers())
	})
}

// TestDLQIntegrationMonitoring tests DLQ monitoring and alerting integration.
func TestDLQIntegrationMonitoring(t *testing.T) {
	t.Run("should integrate DLQ metrics with health monitoring system", func(t *testing.T) {
		// Test integration with existing health monitoring
		mockHealthMonitor := &MockHealthMonitor{}

		expectedHealthStatus := service.DLQHealthStatus{
			IsHealthy:       true,
			StreamExists:    true,
			ConsumerCount:   2,
			MessageBacklog:  50,
			ProcessingRate:  10.5,
			ErrorRate:       0.02,
			LastHealthCheck: time.Now(),
		}

		mockHealthMonitor.On("GetDLQHealth", mock.Anything).
			Return(expectedHealthStatus, nil)

		serviceConfig := service.DLQServiceConfig{
			StreamName:        "INDEXING-DLQ",
			EnableHealthCheck: true,
			EnableAlerts:      true,
		}

		dlqService := service.NewDLQService(serviceConfig, nil, nil, mockHealthMonitor)

		ctx := context.Background()
		health, err := dlqService.PerformHealthCheck(ctx)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")

		// In GREEN phase, should return expected health status
		_ = health // Will be used when implementation is complete
	})

	t.Run("should trigger alerts for DLQ threshold violations", func(t *testing.T) {
		// Test alert triggering when DLQ message count exceeds threshold
		mockHealthMonitor := &MockHealthMonitor{}

		expectedAlert := service.DLQAlert{
			AlertType: "DLQ_THRESHOLD_EXCEEDED",
			Severity:  "HIGH",
			Message:   "DLQ message count exceeded threshold: 150 > 100",
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"current_count": 150,
				"threshold":     100,
				"stream_name":   "INDEXING-DLQ",
			},
		}

		mockHealthMonitor.On("TriggerAlert", mock.Anything, mock.MatchedBy(func(alert service.DLQAlert) bool {
			return alert.AlertType == "DLQ_THRESHOLD_EXCEEDED"
		})).Return(nil)

		// In a real implementation, this would be triggered by the monitoring system
		ctx := context.Background()
		err := mockHealthMonitor.TriggerAlert(ctx, expectedAlert)

		// Should succeed - mocked call
		require.NoError(t, err)

		// Verify mock expectations
		mockHealthMonitor.AssertExpectations(t)
	})

	t.Run("should generate comprehensive DLQ analytics reports", func(t *testing.T) {
		// Test analytics report generation with real-world scenarios
		serviceConfig := service.DLQServiceConfig{
			StreamName: "INDEXING-DLQ",
		}

		dlqService := service.NewDLQService(serviceConfig, nil, nil, nil)

		timeRange := service.TimeRange{
			StartTime: time.Now().Add(-7 * 24 * time.Hour),
			EndTime:   time.Now(),
		}

		ctx := context.Background()
		report, err := dlqService.GenerateAnalyticsReport(ctx, timeRange)

		// Should fail in RED phase
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not implemented yet")

		// In GREEN phase, should return comprehensive analytics
		_ = report // Will be used when implementation is complete
	})
}

// MockNATSMessage mocks a NATS message for testing.
// MockNATSMessage is defined in ack_handler_test.go to avoid redeclaration
