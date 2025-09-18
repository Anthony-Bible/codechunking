package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestWorkerService_Start tests that Start method now works correctly.
func TestWorkerService_Start(t *testing.T) {
	t.Run("should successfully start service and update status", func(t *testing.T) {
		service := createTestWorkerService(t, WorkerServiceConfig{
			Concurrency: 2,
			QueueGroup:  "test-workers",
		})

		// Initially not running
		assert.False(t, service.Health().IsRunning)

		ctx := context.Background()
		err := service.Start(ctx)

		// Service start should succeed with healthy status
		require.NoError(t, err)

		// Verify service is running
		health := service.Health()
		assert.True(t, health.IsRunning)
		assert.Equal(t, 0, health.TotalConsumers) // No consumers added yet
	})

	t.Run("should be idempotent when called multiple times", func(t *testing.T) {
		service := createTestWorkerService(t, WorkerServiceConfig{
			Concurrency: 2,
			QueueGroup:  "test-workers",
		})

		ctx := context.Background()

		// First start
		err1 := service.Start(ctx)
		require.NoError(t, err1)
		assert.True(t, service.Health().IsRunning)

		// Second start should not fail
		err2 := service.Start(ctx)
		require.NoError(t, err2)
		assert.True(t, service.Health().IsRunning)
	})
}

// TestWorkerService_Stop tests that Stop method now works correctly.
func TestWorkerService_Stop(t *testing.T) {
	t.Run("should successfully stop service and update status", func(t *testing.T) {
		service := createTestWorkerService(t, WorkerServiceConfig{
			Concurrency: 2,
			QueueGroup:  "test-workers",
		})

		ctx := context.Background()

		// Start first
		err := service.Start(ctx)
		require.NoError(t, err)
		assert.True(t, service.Health().IsRunning)

		// Stop service
		err = service.Stop(ctx)
		require.NoError(t, err)

		// Verify service is stopped
		health := service.Health()
		assert.False(t, health.IsRunning)
		assert.Equal(t, 0, health.TotalConsumers)
	})

	t.Run("should be idempotent when called on stopped service", func(t *testing.T) {
		service := createTestWorkerService(t, WorkerServiceConfig{
			Concurrency: 2,
			QueueGroup:  "test-workers",
		})

		ctx := context.Background()

		// Service is already stopped initially
		assert.False(t, service.Health().IsRunning)

		// Stop should not fail
		err := service.Stop(ctx)
		require.NoError(t, err)
		assert.False(t, service.Health().IsRunning)
	})
}

// TestWorkerService_AddConsumer tests that AddConsumer method now works correctly.
func TestWorkerService_AddConsumer(t *testing.T) {
	t.Run("should successfully add valid consumer", func(t *testing.T) {
		service := createTestWorkerService(t, WorkerServiceConfig{
			Concurrency: 5,
			QueueGroup:  "test-workers",
		})

		mockConsumer := createMockConsumer("test-consumer-1", "test-workers")

		err := service.AddConsumer(mockConsumer)
		require.NoError(t, err)

		// Verify consumer was added
		health := service.Health()
		assert.Equal(t, 1, health.TotalConsumers)

		consumers := service.GetConsumers()
		assert.Len(t, consumers, 1)
		assert.Equal(t, "test-consumer-1", consumers[0].ID)
	})

	t.Run("should reject duplicate consumer IDs", func(t *testing.T) {
		service := createTestWorkerService(t, WorkerServiceConfig{
			Concurrency: 5,
			QueueGroup:  "test-workers",
		})

		mockConsumer1 := createMockConsumer("duplicate-id", "test-workers")
		mockConsumer2 := createMockConsumer("duplicate-id", "test-workers")

		// First consumer should succeed
		err1 := service.AddConsumer(mockConsumer1)
		require.NoError(t, err1)

		// Second consumer with same ID should fail
		err2 := service.AddConsumer(mockConsumer2)
		require.Error(t, err2)
		assert.Contains(t, err2.Error(), "already exists")
	})

	t.Run("should enforce concurrency limits", func(t *testing.T) {
		service := createTestWorkerService(t, WorkerServiceConfig{
			Concurrency: 2, // Limit to 2 consumers
			QueueGroup:  "test-workers",
		})

		consumer1 := createMockConsumer("consumer-1", "test-workers")
		consumer2 := createMockConsumer("consumer-2", "test-workers")
		consumer3 := createMockConsumer("consumer-3", "test-workers")

		// First two should succeed
		err1 := service.AddConsumer(consumer1)
		require.NoError(t, err1)

		err2 := service.AddConsumer(consumer2)
		require.NoError(t, err2)

		// Third should be rejected
		err3 := service.AddConsumer(consumer3)
		require.Error(t, err3)
		assert.Contains(t, err3.Error(), "maximum concurrency limit")
	})
}

// TestWorkerService_RemoveConsumer tests that RemoveConsumer method now works correctly.
func TestWorkerService_RemoveConsumer(t *testing.T) {
	t.Run("should successfully remove existing consumer", func(t *testing.T) {
		service := createTestWorkerService(t, WorkerServiceConfig{
			Concurrency: 3,
			QueueGroup:  "test-workers",
		})

		// Add a consumer first
		mockConsumer := createMockConsumer("test-consumer", "test-workers")
		err := service.AddConsumer(mockConsumer)
		require.NoError(t, err)

		// Verify it was added
		assert.Equal(t, 1, service.Health().TotalConsumers)

		// Remove the consumer
		err = service.RemoveConsumer("test-consumer")
		require.NoError(t, err)

		// Verify it was removed
		assert.Equal(t, 0, service.Health().TotalConsumers)
		mockConsumer.AssertCalled(t, "Stop", mock.Anything)
	})

	t.Run("should return error for non-existent consumer ID", func(t *testing.T) {
		service := createTestWorkerService(t, WorkerServiceConfig{
			Concurrency: 3,
			QueueGroup:  "test-workers",
		})

		err := service.RemoveConsumer("non-existent-consumer")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// TestWorkerService_RestartConsumer tests that RestartConsumer method now works correctly.
func TestWorkerService_RestartConsumer(t *testing.T) {
	t.Run("should successfully restart existing consumer", func(t *testing.T) {
		service := createTestWorkerService(t, WorkerServiceConfig{
			Concurrency:        3,
			QueueGroup:         "test-workers",
			RestartDelay:       1 * time.Millisecond, // Short delay for tests
			MaxRestartAttempts: 3,
		})

		// Start the service first
		ctx := context.Background()
		err := service.Start(ctx)
		require.NoError(t, err)

		// Add a consumer
		mockConsumer := createMockConsumer("test-consumer", "test-workers")
		err = service.AddConsumer(mockConsumer)
		require.NoError(t, err)

		// Restart the consumer
		err = service.RestartConsumer("test-consumer")
		require.NoError(t, err)

		// Verify restart was called
		mockConsumer.AssertCalled(t, "Stop", mock.Anything)
		mockConsumer.AssertCalled(t, "Start", mock.Anything)

		// Verify metrics were updated
		metrics := service.GetMetrics()
		assert.Equal(t, int64(1), metrics.RestartCount)
		assert.False(t, metrics.LastRestartTime.IsZero())
	})

	t.Run("should return error for non-existent consumer ID", func(t *testing.T) {
		service := createTestWorkerService(t, WorkerServiceConfig{
			Concurrency: 3,
			QueueGroup:  "test-workers",
		})

		// Start the service first
		ctx := context.Background()
		err := service.Start(ctx)
		require.NoError(t, err)

		err = service.RestartConsumer("non-existent-consumer")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("should reject restart when service is not running", func(t *testing.T) {
		service := createTestWorkerService(t, WorkerServiceConfig{
			Concurrency: 3,
			QueueGroup:  "test-workers",
		})

		// Don't start service - it should be stopped

		err := service.RestartConsumer("any-consumer")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "service is shutting down")
	})
}
