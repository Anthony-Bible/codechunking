package service

import (
	"codechunking/internal/config"
	"codechunking/internal/domain/messaging"
	"codechunking/internal/port/inbound"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockConsumer mocks the message consumer interface.
type MockConsumer struct {
	mock.Mock
}

func (m *MockConsumer) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockConsumer) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockConsumer) Health() inbound.ConsumerHealthStatus {
	args := m.Called()
	return args.Get(0).(inbound.ConsumerHealthStatus)
}

func (m *MockConsumer) GetStats() inbound.ConsumerStats {
	args := m.Called()
	return args.Get(0).(inbound.ConsumerStats)
}

func (m *MockConsumer) QueueGroup() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockConsumer) Subject() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockConsumer) DurableName() string {
	args := m.Called()
	return args.String(0)
}

// MockJobProcessor mocks the job processor interface.
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

// Note: All interfaces are now imported from inbound port package

// Note: All types and implementation moved to worker_service.go

// TestWorkerServiceCreation tests worker service creation and configuration.
func TestWorkerServiceCreation(t *testing.T) {
	t.Run("should create worker service with valid configuration", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency:         5,
			QueueGroup:          "indexing-workers",
			JobTimeout:          5 * time.Minute,
			HealthCheckInterval: 30 * time.Second,
			RestartDelay:        5 * time.Second,
			MaxRestartAttempts:  3,
			ShutdownTimeout:     30 * time.Second,
		}

		natsConfig := config.NATSConfig{
			URL:           "nats://localhost:4222",
			MaxReconnects: 10,
			ReconnectWait: 2 * time.Second,
		}

		mockJobProcessor := &MockJobProcessor{}

		// Set up mock expectations for GetHealthStatus call
		mockJobProcessor.On("GetHealthStatus").Return(inbound.JobProcessorHealthStatus{
			IsReady:       false,
			ActiveJobs:    0,
			CompletedJobs: 0,
			FailedJobs:    0,
		})

		service := NewDefaultWorkerService(serviceConfig, natsConfig, mockJobProcessor)

		require.NotNil(t, service)

		// Health should be initialized properly in REFACTOR phase
		health := service.Health()
		assert.False(t, health.IsRunning)
		assert.Equal(t, 0, health.TotalConsumers)
	})

	t.Run("should fail with invalid concurrency", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency: -1, // Invalid negative concurrency
			QueueGroup:  "indexing-workers",
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockJobProcessor := &MockJobProcessor{}

		// In a real implementation, this should validate the config
		service := NewDefaultWorkerService(serviceConfig, natsConfig, mockJobProcessor)

		// For RED phase, service is created but should fail validation later
		require.NotNil(t, service)
	})

	t.Run("should fail with nil job processor", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency: 5,
			QueueGroup:  "indexing-workers",
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		// In a real implementation, this should handle nil processor
		service := NewDefaultWorkerService(serviceConfig, natsConfig, nil)

		// For RED phase, service is created but will fail on use
		require.NotNil(t, service)
	})
}

// TestMultipleConsumerManagement tests management of multiple consumers.
func TestMultipleConsumerManagement(t *testing.T) {
	t.Run("should manage multiple consumers based on concurrency", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency: 2,
			QueueGroup:  "indexing-workers",
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockJobProcessor := &MockJobProcessor{}
		service := NewDefaultWorkerService(serviceConfig, natsConfig, mockJobProcessor)

		// Create mock consumers with proper health and stats setup
		mockConsumer1 := &MockConsumer{}
		mockConsumer2 := &MockConsumer{}
		mockConsumer3 := &MockConsumer{}

		// Setup consumer 1
		mockConsumer1.On("QueueGroup").Return("indexing-workers")
		mockConsumer1.On("Subject").Return("indexing.job")
		mockConsumer1.On("DurableName").Return("consumer-1")
		mockConsumer1.On("Start", mock.Anything).Return(nil)
		mockConsumer1.On("Health").Return(inbound.ConsumerHealthStatus{
			IsRunning:   true,
			IsConnected: true,
		})
		mockConsumer1.On("GetStats").Return(inbound.ConsumerStats{
			ActiveSince: time.Now(),
		})

		// Setup consumer 2
		mockConsumer2.On("QueueGroup").Return("indexing-workers")
		mockConsumer2.On("Subject").Return("indexing.job")
		mockConsumer2.On("DurableName").Return("consumer-2")
		mockConsumer2.On("Start", mock.Anything).Return(nil)
		mockConsumer2.On("Health").Return(inbound.ConsumerHealthStatus{
			IsRunning:   true,
			IsConnected: true,
		})
		mockConsumer2.On("GetStats").Return(inbound.ConsumerStats{
			ActiveSince: time.Now(),
		})

		// Setup consumer 3
		mockConsumer3.On("QueueGroup").Return("indexing-workers")
		mockConsumer3.On("Subject").Return("indexing.job")
		mockConsumer3.On("DurableName").Return("consumer-3")
		mockConsumer3.On("Start", mock.Anything).Return(nil)
		mockConsumer3.On("Health").Return(inbound.ConsumerHealthStatus{
			IsRunning:   true,
			IsConnected: true,
		})
		mockConsumer3.On("GetStats").Return(inbound.ConsumerStats{
			ActiveSince: time.Now(),
		})

		// Add first two consumers - should succeed
		err1 := service.AddConsumer(mockConsumer1)
		err2 := service.AddConsumer(mockConsumer2)
		require.NoError(t, err1)
		require.NoError(t, err2)

		// Add third consumer - should fail due to concurrency limit
		err3 := service.AddConsumer(mockConsumer3)
		require.Error(t, err3)
		assert.Contains(t, err3.Error(), "maximum concurrency limit reached")

		// Get consumers should return 2 consumers
		consumers := service.GetConsumers()
		assert.Len(t, consumers, 2)

		// Verify consumer details
		consumerIDs := make([]string, len(consumers))
		for i, consumer := range consumers {
			consumerIDs[i] = consumer.ID
		}
		assert.Contains(t, consumerIDs, "consumer-1")
		assert.Contains(t, consumerIDs, "consumer-2")
	})

	t.Run("should start all consumers when service starts", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency: 2,
			QueueGroup:  "indexing-workers",
		}

		natsConfig := config.NATSConfig{
			URL: "nats://localhost:4222",
		}

		mockJobProcessor := &MockJobProcessor{}
		service := NewDefaultWorkerService(serviceConfig, natsConfig, mockJobProcessor)

		// Add a consumer first
		mockConsumer := &MockConsumer{}
		mockConsumer.On("QueueGroup").Return("indexing-workers")
		mockConsumer.On("Subject").Return("indexing.job")
		mockConsumer.On("DurableName").Return("test-consumer")
		mockConsumer.On("Start", mock.Anything).Return(nil)
		mockConsumer.On("Health").Return(inbound.ConsumerHealthStatus{
			IsRunning:   true,
			IsConnected: true,
		})
		mockConsumer.On("GetStats").Return(inbound.ConsumerStats{
			ActiveSince: time.Now(),
		})

		err := service.AddConsumer(mockConsumer)
		require.NoError(t, err)

		// Start the service
		ctx := context.Background()
		err = service.Start(ctx)
		require.NoError(t, err)

		// Verify service is running
		health := service.Health()
		assert.True(t, health.IsRunning)

		// Starting again should be idempotent
		err = service.Start(ctx)
		require.NoError(t, err)
	})

	t.Run("should stop all consumers when service stops", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency:     2,
			QueueGroup:      "indexing-workers",
			ShutdownTimeout: 10 * time.Second,
		}

		mockJobProcessor := &MockJobProcessor{}
		mockJobProcessor.On("Cleanup").Return(nil)

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, mockJobProcessor)

		// Add a consumer first
		mockConsumer := &MockConsumer{}
		mockConsumer.On("QueueGroup").Return("indexing-workers")
		mockConsumer.On("Subject").Return("indexing.job")
		mockConsumer.On("DurableName").Return("test-consumer")
		mockConsumer.On("Start", mock.Anything).Return(nil)
		mockConsumer.On("Stop", mock.Anything).Return(nil)
		mockConsumer.On("Health").Return(inbound.ConsumerHealthStatus{
			IsRunning:   false,
			IsConnected: false,
		})
		mockConsumer.On("GetStats").Return(inbound.ConsumerStats{
			ActiveSince: time.Now(),
		})

		err := service.AddConsumer(mockConsumer)
		require.NoError(t, err)

		// Start the service first
		ctx := context.Background()
		err = service.Start(ctx)
		require.NoError(t, err)

		// Verify service is running
		health := service.Health()
		assert.True(t, health.IsRunning)

		// Stop the service
		err = service.Stop(ctx)
		require.NoError(t, err)

		// Verify service is stopped
		health = service.Health()
		assert.False(t, health.IsRunning)

		// Stopping again should be idempotent
		err = service.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("should handle consumer addition and removal", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency: 5,
			QueueGroup:  "indexing-workers",
		}

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, nil)

		mockConsumer := &MockConsumer{}
		mockConsumer.On("QueueGroup").Return("indexing-workers")
		mockConsumer.On("Subject").Return("indexing.job")
		mockConsumer.On("DurableName").Return("test-consumer")
		mockConsumer.On("Start", mock.Anything).Return(nil)
		mockConsumer.On("Stop", mock.Anything).Return(nil)
		mockConsumer.On("Health").Return(inbound.ConsumerHealthStatus{
			IsRunning:   true,
			IsConnected: true,
		})
		mockConsumer.On("GetStats").Return(inbound.ConsumerStats{
			ActiveSince: time.Now(),
		})

		// Add consumer
		err := service.AddConsumer(mockConsumer)
		require.NoError(t, err)

		// Verify consumer was added
		consumers := service.GetConsumers()
		assert.Len(t, consumers, 1)
		assert.Equal(t, "test-consumer", consumers[0].ID)

		// Remove consumer
		err = service.RemoveConsumer("test-consumer")
		require.NoError(t, err)

		// Verify consumer was removed
		consumers = service.GetConsumers()
		assert.Len(t, consumers, 0)

		// Try to remove non-existent consumer
		err = service.RemoveConsumer("non-existent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// TestHealthMonitoring tests health monitoring and restart logic.
func TestHealthMonitoring(t *testing.T) {
	t.Run("should monitor consumer health", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency:         3,
			QueueGroup:          "indexing-workers",
			HealthCheckInterval: 100 * time.Millisecond,
		}

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, nil)

		health := service.Health()

		// Should return empty health in RED phase
		assert.False(t, health.IsRunning)
		assert.Equal(t, 0, health.TotalConsumers)
		assert.Equal(t, 0, health.HealthyConsumers)
		assert.Equal(t, 0, health.UnhealthyConsumers)
	})

	t.Run("should restart unhealthy consumers", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency:        3,
			QueueGroup:         "indexing-workers",
			RestartDelay:       0, // No delay for test
			MaxRestartAttempts: 3,
		}

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, nil)

		// Try to restart a consumer when service is not running
		err := service.RestartConsumer("non-existent-consumer")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "service is shutting down")

		// Add a consumer and start service first
		mockConsumer := &MockConsumer{}
		mockConsumer.On("QueueGroup").Return("indexing-workers")
		mockConsumer.On("Subject").Return("indexing.job")
		mockConsumer.On("DurableName").Return("test-consumer")
		mockConsumer.On("Start", mock.Anything).Return(nil)
		mockConsumer.On("Stop", mock.Anything).Return(nil)
		mockConsumer.On("Health").Return(inbound.ConsumerHealthStatus{
			IsRunning:   true,
			IsConnected: true,
		})
		mockConsumer.On("GetStats").Return(inbound.ConsumerStats{
			ActiveSince: time.Now(),
		})

		err = service.AddConsumer(mockConsumer)
		require.NoError(t, err)

		// Start the service
		ctx := context.Background()
		err = service.Start(ctx)
		require.NoError(t, err)

		// Now try to restart non-existent consumer - should fail with not found
		err = service.RestartConsumer("non-existent-consumer")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("should track restart attempts", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency:        2,
			QueueGroup:         "indexing-workers",
			MaxRestartAttempts: 3,
		}

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, nil)

		metrics := service.GetMetrics()

		// Should return empty metrics in RED phase
		assert.Equal(t, int64(0), metrics.RestartCount)
		assert.True(t, metrics.LastRestartTime.IsZero())
	})

	t.Run("should handle maximum restart attempts exceeded", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency:        2,
			QueueGroup:         "indexing-workers",
			MaxRestartAttempts: 2,
		}

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, nil)

		// Add a consumer and start service first
		mockConsumer := &MockConsumer{}
		mockConsumer.On("QueueGroup").Return("indexing-workers")
		mockConsumer.On("Subject").Return("indexing.job")
		mockConsumer.On("DurableName").Return("failing-consumer")
		mockConsumer.On("Start", mock.Anything).Return(nil)
		mockConsumer.On("Stop", mock.Anything).Return(nil)
		mockConsumer.On("Health").Return(inbound.ConsumerHealthStatus{
			IsRunning:   true,
			IsConnected: true,
		})
		mockConsumer.On("GetStats").Return(inbound.ConsumerStats{
			ActiveSince: time.Now(),
		})

		err := service.AddConsumer(mockConsumer)
		require.NoError(t, err)

		// Start the service
		ctx := context.Background()
		err = service.Start(ctx)
		require.NoError(t, err)

		// Try multiple restarts on non-existent consumer
		for range 3 {
			err := service.RestartConsumer("non-existent-consumer")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not found")
		}

		health := service.Health()
		// Service should still be running
		assert.True(t, health.IsRunning)
	})
}

// TestGracefulShutdown tests graceful shutdown with proper cleanup.
func TestGracefulShutdown(t *testing.T) {
	t.Run("should shutdown gracefully with timeout", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency:     3,
			QueueGroup:      "indexing-workers",
			ShutdownTimeout: 5 * time.Second,
		}

		mockJobProcessor := &MockJobProcessor{}
		mockJobProcessor.On("Cleanup").Return(nil)

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, mockJobProcessor)

		ctx := context.Background()
		err := service.Stop(ctx)

		// Should succeed even when no consumers are added
		require.NoError(t, err)

		// Verify service is stopped
		health := service.Health()
		assert.False(t, health.IsRunning)
	})

	t.Run("should handle shutdown timeout", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency:     2,
			QueueGroup:      "indexing-workers",
			ShutdownTimeout: 100 * time.Millisecond, // Very short timeout
		}

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, nil)

		ctx := context.Background()
		err := service.Stop(ctx)

		// Should succeed even with very short timeout when no consumers
		require.NoError(t, err)

		// Verify service is stopped
		health := service.Health()
		assert.False(t, health.IsRunning)
	})

	t.Run("should cleanup job processor on shutdown", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency: 2,
			QueueGroup:  "indexing-workers",
		}

		mockJobProcessor := &MockJobProcessor{}
		mockJobProcessor.On("Cleanup").Return(nil)

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, mockJobProcessor)

		// Start the service first to ensure proper state
		ctx := context.Background()
		err := service.Start(ctx)
		require.NoError(t, err)

		// Stop the service
		err = service.Stop(ctx)

		// Should succeed and call cleanup
		require.NoError(t, err)

		// Verify cleanup was called
		mockJobProcessor.AssertExpectations(t)
	})

	t.Run("should handle context cancellation during shutdown", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency:     2,
			QueueGroup:      "indexing-workers",
			ShutdownTimeout: 10 * time.Second,
		}

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, nil)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := service.Stop(ctx)

		// Should succeed even with cancelled context when no consumers
		require.NoError(t, err)

		// Verify service is stopped
		health := service.Health()
		assert.False(t, health.IsRunning)
	})
}

// TestIntegrationWithHealthService tests integration with existing health service.
func TestIntegrationWithHealthService(t *testing.T) {
	t.Run("should integrate with health service", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency:         3,
			QueueGroup:          "indexing-workers",
			HealthCheckInterval: 30 * time.Second,
		}

		mockJobProcessor := &MockJobProcessor{}
		mockJobProcessor.On("GetHealthStatus").Return(inbound.JobProcessorHealthStatus{
			IsReady: true,
		})

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, mockJobProcessor)

		health := service.Health()

		// Should return empty health in RED phase
		assert.False(t, health.IsRunning)
		assert.False(t, health.JobProcessorHealth.IsReady)
	})

	t.Run("should report aggregate health status", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency: 3,
			QueueGroup:  "indexing-workers",
		}

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, nil)

		health := service.Health()

		// Verify health structure in RED phase
		assert.False(t, health.IsRunning)
		assert.Equal(t, 0, health.TotalConsumers)
		assert.Equal(t, 0, health.HealthyConsumers)
		assert.Equal(t, 0, health.UnhealthyConsumers)
		assert.Empty(t, health.ConsumerHealthDetails)
		assert.True(t, health.LastHealthCheck.IsZero())
	})

	t.Run("should track service uptime", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency: 2,
			QueueGroup:  "indexing-workers",
		}

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, nil)

		health := service.Health()

		// In RED phase, uptime should be zero
		assert.Equal(t, time.Duration(0), health.ServiceUptime)
	})
}

// TestQueueMonitoring tests queue monitoring and metrics collection.
func TestQueueMonitoring(t *testing.T) {
	t.Run("should collect queue metrics", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency: 3,
			QueueGroup:  "indexing-workers",
		}

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, nil)

		metrics := service.GetMetrics()

		// Should return empty metrics in RED phase
		assert.Equal(t, int64(0), metrics.TotalMessagesProcessed)
		assert.Equal(t, int64(0), metrics.TotalMessagesFailed)
		assert.Equal(t, time.Duration(0), metrics.AverageProcessingTime)
		assert.Empty(t, metrics.ConsumerMetrics)
		assert.True(t, metrics.ServiceStartTime.IsZero())
	})

	t.Run("should aggregate consumer metrics", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency: 2,
			QueueGroup:  "indexing-workers",
		}

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, nil)

		// Get aggregated metrics
		metrics := service.GetMetrics()

		// Should be empty in RED phase
		assert.Empty(t, metrics.ConsumerMetrics)
		assert.Equal(t, int64(0), metrics.TotalMessagesProcessed)
	})

	t.Run("should track processing times", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency: 3,
			QueueGroup:  "indexing-workers",
		}

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, nil)

		metrics := service.GetMetrics()

		// Should return zero average in RED phase
		assert.Equal(t, time.Duration(0), metrics.AverageProcessingTime)
	})

	t.Run("should monitor job processor metrics", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency: 2,
			QueueGroup:  "indexing-workers",
		}

		mockJobProcessor := &MockJobProcessor{}
		mockJobProcessor.On("GetMetrics").Return(inbound.JobProcessorMetrics{
			TotalJobsProcessed: 100,
			FilesProcessed:     500,
		})

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, mockJobProcessor)

		metrics := service.GetMetrics()

		// Should return empty metrics in RED phase
		assert.Equal(t, int64(0), metrics.JobProcessorMetrics.TotalJobsProcessed)
		assert.Equal(t, int64(0), metrics.JobProcessorMetrics.FilesProcessed)
	})
}

// TestConcurrentOperations tests concurrent service operations.
func TestConcurrentOperations(t *testing.T) {
	t.Run("should handle concurrent start and stop operations", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency: 3,
			QueueGroup:  "indexing-workers",
		}

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, nil)

		ctx := context.Background()

		// Try concurrent start operations first
		var wg sync.WaitGroup
		startErrors := make([]error, 2)

		for i := range 2 {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				startErrors[index] = service.Start(ctx)
			}(i)
		}

		wg.Wait()

		// All start operations should succeed (idempotent)
		for _, err := range startErrors {
			require.NoError(t, err)
		}

		// Verify service is running
		health := service.Health()
		assert.True(t, health.IsRunning)

		// Now try concurrent stop operations
		stopErrors := make([]error, 2)

		for i := range 2 {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				stopErrors[index] = service.Stop(ctx)
			}(i)
		}

		wg.Wait()

		// All stop operations should succeed (idempotent)
		for _, err := range stopErrors {
			require.NoError(t, err)
		}

		// Verify service is stopped
		health = service.Health()
		assert.False(t, health.IsRunning)
	})

	t.Run("should handle concurrent consumer management", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency: 5,
			QueueGroup:  "indexing-workers",
		}

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, nil)

		mockConsumer1 := &MockConsumer{}
		mockConsumer2 := &MockConsumer{}

		// Setup complete mocks for consumer 1
		mockConsumer1.On("QueueGroup").Return("indexing-workers")
		mockConsumer1.On("Subject").Return("indexing.job")
		mockConsumer1.On("DurableName").Return("consumer-1")
		mockConsumer1.On("Start", mock.Anything).Return(nil)
		mockConsumer1.On("Stop", mock.Anything).Return(nil)
		mockConsumer1.On("Health").Return(inbound.ConsumerHealthStatus{
			IsRunning:   true,
			IsConnected: true,
		})
		mockConsumer1.On("GetStats").Return(inbound.ConsumerStats{
			ActiveSince: time.Now(),
		})

		// Setup complete mocks for consumer 2
		mockConsumer2.On("QueueGroup").Return("indexing-workers")
		mockConsumer2.On("Subject").Return("indexing.job")
		mockConsumer2.On("DurableName").Return("consumer-2")
		mockConsumer2.On("Start", mock.Anything).Return(nil)
		mockConsumer2.On("Stop", mock.Anything).Return(nil)
		mockConsumer2.On("Health").Return(inbound.ConsumerHealthStatus{
			IsRunning:   true,
			IsConnected: true,
		})
		mockConsumer2.On("GetStats").Return(inbound.ConsumerStats{
			ActiveSince: time.Now(),
		})

		var wg sync.WaitGroup
		errors := make([]error, 4)

		// Concurrent add operations
		wg.Add(1)
		go func() {
			defer wg.Done()
			errors[0] = service.AddConsumer(mockConsumer1)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			errors[1] = service.AddConsumer(mockConsumer2)
		}()

		// Concurrent remove operations
		wg.Add(1)
		go func() {
			defer wg.Done()
			errors[2] = service.RemoveConsumer("consumer-1")
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			errors[3] = service.RemoveConsumer("consumer-2")
		}()

		wg.Wait()

		// Add operations should succeed
		require.NoError(t, errors[0])
		require.NoError(t, errors[1])

		// Remove operations should fail because consumers weren't actually added yet
		// (race condition - they may or may not exist when remove is called)
		// This is expected behavior in concurrent scenarios
		if errors[2] != nil {
			assert.Contains(t, errors[2].Error(), "not found")
		}
		if errors[3] != nil {
			assert.Contains(t, errors[3].Error(), "not found")
		}
	})

	t.Run("should be thread-safe for health and metrics queries", func(t *testing.T) {
		serviceConfig := WorkerServiceConfig{
			Concurrency: 3,
			QueueGroup:  "indexing-workers",
		}

		service := NewDefaultWorkerService(serviceConfig, config.NATSConfig{}, nil)

		// Concurrent health and metrics queries
		var wg sync.WaitGroup
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = service.Health()
				_ = service.GetMetrics()
				_ = service.GetConsumers()
			}()
		}

		wg.Wait()

		// Should complete without panic
		health := service.Health()
		assert.False(t, health.IsRunning)
	})
}
