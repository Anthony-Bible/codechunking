package outbound

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBatchQueueManagerInterface_QueueEmbeddingRequest tests basic request queuing functionality.
func TestBatchQueueManagerInterface_QueueEmbeddingRequest(t *testing.T) {
	tests := []struct {
		name        string
		request     *EmbeddingRequest
		expectError bool
		errorType   string
	}{
		{
			name: "should queue valid real-time request",
			request: &EmbeddingRequest{
				RequestID:   "req-001",
				Text:        "Sample text for embedding",
				Priority:    PriorityRealTime,
				Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
			},
			expectError: false,
		},
		{
			name: "should queue valid interactive request",
			request: &EmbeddingRequest{
				RequestID:   "req-002",
				Text:        "Interactive text processing",
				Priority:    PriorityInteractive,
				Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
			},
			expectError: false,
		},
		{
			name: "should queue valid background request",
			request: &EmbeddingRequest{
				RequestID:   "req-003",
				Text:        "Background processing text",
				Priority:    PriorityBackground,
				Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
			},
			expectError: false,
		},
		{
			name: "should queue valid batch request",
			request: &EmbeddingRequest{
				RequestID:   "req-004",
				Text:        "Batch processing text",
				Priority:    PriorityBatch,
				Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
			},
			expectError: false,
		},
		{
			name: "should reject request with empty RequestID",
			request: &EmbeddingRequest{
				RequestID:   "",
				Text:        "Sample text",
				Priority:    PriorityInteractive,
				Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
			},
			expectError: true,
			errorType:   "validation",
		},
		{
			name: "should reject request with empty text",
			request: &EmbeddingRequest{
				RequestID:   "req-005",
				Text:        "",
				Priority:    PriorityInteractive,
				Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
			},
			expectError: true,
			errorType:   "validation",
		},
		{
			name: "should reject request with invalid priority",
			request: &EmbeddingRequest{
				RequestID:   "req-006",
				Text:        "Sample text",
				Priority:    RequestPriority("invalid"),
				Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
			},
			expectError: true,
			errorType:   "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// RED PHASE: This test will fail until implementation is provided
			var manager BatchQueueManager
			require.Nil(
				t,
				manager,
				"BatchQueueManager implementation not yet available - this test defines expected behavior",
			)

			// Define expected behavior without calling unimplemented methods
			t.Logf("Expected behavior: When implementation exists, QueueEmbeddingRequest should:")
			if tt.expectError {
				t.Logf("- Return error of type '%s' for invalid input: %+v", tt.errorType, tt.request)
			} else {
				t.Logf("- Successfully queue valid request: %+v", tt.request)
			}

			// This assertion will fail in Red Phase, defining the need for implementation
			assert.Fail(t, "Implementation needed", "BatchQueueManager.QueueEmbeddingRequest not implemented")
		})
	}
}

// TestBatchQueueManagerInterface_QueueBulkEmbeddingRequests tests bulk request queuing functionality.
func TestBatchQueueManagerInterface_QueueBulkEmbeddingRequests(t *testing.T) {
	tests := []struct {
		name         string
		requests     []*EmbeddingRequest
		expectError  bool
		expectedType string
	}{
		{
			name: "should queue multiple valid requests with mixed priorities",
			requests: []*EmbeddingRequest{
				{
					RequestID:   "bulk-001",
					Text:        "Real-time request",
					Priority:    PriorityRealTime,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "bulk-002",
					Text:        "Interactive request",
					Priority:    PriorityInteractive,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "bulk-003",
					Text:        "Background request",
					Priority:    PriorityBackground,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
			},
			expectError: false,
		},
		{
			name:        "should handle empty request slice",
			requests:    []*EmbeddingRequest{},
			expectError: false,
		},
		{
			name:         "should reject nil request slice",
			requests:     nil,
			expectError:  true,
			expectedType: "validation",
		},
		{
			name: "should reject slice containing invalid requests",
			requests: []*EmbeddingRequest{
				{
					RequestID:   "valid-001",
					Text:        "Valid request",
					Priority:    PriorityInteractive,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "", // Invalid - empty RequestID
					Text:        "Invalid request",
					Priority:    PriorityInteractive,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
			},
			expectError:  true,
			expectedType: "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// RED PHASE: This test will fail until implementation is provided
			var manager BatchQueueManager
			require.Nil(
				t,
				manager,
				"BatchQueueManager implementation not yet available - this test defines expected behavior",
			)

			// Define expected behavior without calling unimplemented methods
			t.Logf("Expected behavior: When implementation exists, QueueBulkEmbeddingRequests should:")
			if tt.expectError {
				t.Logf("- Return error of type '%s' for invalid input: %d requests", tt.expectedType, len(tt.requests))
			} else {
				t.Logf("- Successfully queue %d requests with mixed priorities", len(tt.requests))
			}

			// This assertion will fail in Red Phase, defining the need for implementation
			assert.Fail(t, "Implementation needed", "BatchQueueManager.QueueBulkEmbeddingRequests not implemented")
		})
	}
}

// TestBatchQueueManagerInterface_ProcessQueue tests queue processing functionality.
func TestBatchQueueManagerInterface_ProcessQueue(t *testing.T) {
	tests := []struct {
		name              string
		setupRequests     []*EmbeddingRequest
		expectedBatches   int
		expectError       bool
		expectedErrorType string
	}{
		{
			name: "should process queued requests and create batches based on priority",
			setupRequests: []*EmbeddingRequest{
				// Real-time requests should be processed first, in smallest batches
				{
					RequestID:   "rt-001",
					Text:        "Real-time 1",
					Priority:    PriorityRealTime,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "rt-002",
					Text:        "Real-time 2",
					Priority:    PriorityRealTime,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				// Interactive requests should be processed next
				{
					RequestID:   "int-001",
					Text:        "Interactive 1",
					Priority:    PriorityInteractive,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "int-002",
					Text:        "Interactive 2",
					Priority:    PriorityInteractive,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "int-003",
					Text:        "Interactive 3",
					Priority:    PriorityInteractive,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				// Background requests should be processed with medium-size batches
				{
					RequestID:   "bg-001",
					Text:        "Background 1",
					Priority:    PriorityBackground,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "bg-002",
					Text:        "Background 2",
					Priority:    PriorityBackground,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				// Batch requests should be processed with largest batches for efficiency
				{
					RequestID:   "batch-001",
					Text:        "Batch 1",
					Priority:    PriorityBatch,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "batch-002",
					Text:        "Batch 2",
					Priority:    PriorityBatch,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "batch-003",
					Text:        "Batch 3",
					Priority:    PriorityBatch,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
			},
			expectedBatches: 4, // Expected: 1 for RT, 1 for Interactive, 1 for Background, 1 for Batch
			expectError:     false,
		},
		{
			name:            "should handle empty queue gracefully",
			setupRequests:   []*EmbeddingRequest{},
			expectedBatches: 0,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail until implementation is provided
			var manager BatchQueueManager
			require.Nil(
				t,
				manager,
				"BatchQueueManager implementation not yet available - this test defines expected behavior",
			)

			ctx := context.Background()

			// Setup: Queue the test requests
			if len(tt.setupRequests) > 0 {
				err := manager.QueueBulkEmbeddingRequests(ctx, tt.setupRequests)
				require.NoError(t, err, "Setup should succeed")
			}

			// Act: Process the queue
			batchCount, err := manager.ProcessQueue(ctx)

			// Assert: Verify expected outcomes
			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErrorType != "" {
					var qmErr *QueueManagerError
					if assert.ErrorAs(t, err, &qmErr) {
						assert.Equal(t, tt.expectedErrorType, qmErr.Type)
					}
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBatches, batchCount, "Should create expected number of batches")
			}
		})
	}
}

// TestBatchQueueManagerInterface_PriorityOrdering tests that requests are processed in correct priority order.
func TestBatchQueueManagerInterface_PriorityOrdering(t *testing.T) {
	// This test will fail until implementation is provided
	var manager BatchQueueManager
	require.Nil(t, manager, "BatchQueueManager implementation not yet available - this test defines expected behavior")

	ctx := context.Background()

	// Queue requests in reverse priority order to test proper sorting
	requests := []*EmbeddingRequest{
		{
			RequestID:   "batch-001",
			Text:        "Batch priority",
			Priority:    PriorityBatch,
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "bg-001",
			Text:        "Background priority",
			Priority:    PriorityBackground,
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "int-001",
			Text:        "Interactive priority",
			Priority:    PriorityInteractive,
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "rt-001",
			Text:        "Real-time priority",
			Priority:    PriorityRealTime,
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
	}

	// Queue all requests
	err := manager.QueueBulkEmbeddingRequests(ctx, requests)
	require.NoError(t, err)

	// Get queue stats to verify priority ordering
	stats, err := manager.GetQueueStats(ctx)
	require.NoError(t, err)

	// Verify that real-time has highest priority in processing
	assert.Equal(t, 1, stats.RealTimeQueueSize, "Should have 1 real-time request")
	assert.Equal(t, 1, stats.InteractiveQueueSize, "Should have 1 interactive request")
	assert.Equal(t, 1, stats.BackgroundQueueSize, "Should have 1 background request")
	assert.Equal(t, 1, stats.BatchQueueSize, "Should have 1 batch request")
	assert.Equal(t, 4, stats.TotalQueueSize, "Should have 4 total requests")

	// Process queue and verify correct ordering
	batchCount, err := manager.ProcessQueue(ctx)
	assert.NoError(t, err)
	assert.Positive(t, batchCount, "Should create at least one batch")
}

// TestBatchQueueManagerInterface_BatchSizeOptimization tests dynamic batch sizing based on priority and load.
func TestBatchQueueManagerInterface_BatchSizeOptimization(t *testing.T) {
	tests := []struct {
		name             string
		config           *BatchConfig
		requests         []*EmbeddingRequest
		expectedBehavior string
	}{
		{
			name: "should use small batches for real-time requests",
			config: &BatchConfig{
				MinBatchSize:        1,
				MaxBatchSize:        100,
				BatchTimeout:        100 * time.Millisecond,
				EnableDynamicSizing: true,
				PriorityWeights: map[RequestPriority]float64{
					PriorityRealTime:    1.0,
					PriorityInteractive: 0.7,
					PriorityBackground:  0.4,
					PriorityBatch:       0.1,
				},
			},
			requests: []*EmbeddingRequest{
				{
					RequestID:   "rt-1",
					Text:        "RT 1",
					Priority:    PriorityRealTime,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "rt-2",
					Text:        "RT 2",
					Priority:    PriorityRealTime,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "rt-3",
					Text:        "RT 3",
					Priority:    PriorityRealTime,
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
			},
			expectedBehavior: "small batch sizes for low latency",
		},
		{
			name: "should use large batches for batch priority requests",
			config: &BatchConfig{
				MinBatchSize:        1,
				MaxBatchSize:        100,
				BatchTimeout:        5 * time.Second,
				EnableDynamicSizing: true,
				PriorityWeights: map[RequestPriority]float64{
					PriorityRealTime:    1.0,
					PriorityInteractive: 0.7,
					PriorityBackground:  0.4,
					PriorityBatch:       0.1,
				},
			},
			requests:         generateBatchRequests(50), // Generate 50 batch priority requests
			expectedBehavior: "large batch sizes for cost efficiency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail until implementation is provided
			var manager BatchQueueManager
			require.Nil(
				t,
				manager,
				"BatchQueueManager implementation not yet available - this test defines expected behavior",
			)

			ctx := context.Background()

			// Configure the batch manager
			err := manager.UpdateBatchConfiguration(ctx, tt.config)
			require.NoError(t, err, "Configuration should succeed")

			// Queue the test requests
			err = manager.QueueBulkEmbeddingRequests(ctx, tt.requests)
			require.NoError(t, err, "Queuing should succeed")

			// Process the queue
			batchCount, err := manager.ProcessQueue(ctx)
			assert.NoError(t, err)

			// Verify appropriate batch behavior based on priority
			stats, err := manager.GetQueueStats(ctx)
			require.NoError(t, err)

			if len(tt.requests) > 0 && batchCount > 0 {
				avgBatchSize := stats.AverageRequestsPerBatch

				// For real-time requests, expect smaller batch sizes
				if tt.requests[0].Priority == PriorityRealTime {
					assert.LessOrEqual(t, avgBatchSize, 5.0, "Real-time requests should use small batches")
				}

				// For batch priority requests, expect larger batch sizes
				if tt.requests[0].Priority == PriorityBatch {
					assert.GreaterOrEqual(
						t,
						avgBatchSize,
						10.0,
						"Batch requests should use larger batches for efficiency",
					)
				}
			}
		})
	}
}

// TestBatchQueueManagerInterface_DeadlineHandling tests handling of request deadlines.
func TestBatchQueueManagerInterface_DeadlineHandling(t *testing.T) {
	// This test will fail until implementation is provided
	var manager BatchQueueManager
	require.Nil(t, manager, "BatchQueueManager implementation not yet available - this test defines expected behavior")

	ctx := context.Background()

	now := time.Now()
	requests := []*EmbeddingRequest{
		{
			RequestID:   "deadline-001",
			Text:        "Request with near deadline",
			Priority:    PriorityInteractive,
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: now,
			Deadline:    &[]time.Time{now.Add(100 * time.Millisecond)}[0], // Very tight deadline
		},
		{
			RequestID:   "deadline-002",
			Text:        "Request with far deadline",
			Priority:    PriorityInteractive,
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: now,
			Deadline:    &[]time.Time{now.Add(10 * time.Second)}[0], // Loose deadline
		},
		{
			RequestID:   "no-deadline",
			Text:        "Request without deadline",
			Priority:    PriorityInteractive,
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: now,
			Deadline:    nil, // No deadline
		},
	}

	err := manager.QueueBulkEmbeddingRequests(ctx, requests)
	require.NoError(t, err)

	// Process queue - requests with tighter deadlines should be prioritized
	batchCount, err := manager.ProcessQueue(ctx)
	assert.NoError(t, err)
	assert.Positive(t, batchCount, "Should process requests")

	// Check stats for deadline handling
	stats, err := manager.GetQueueStats(ctx)
	require.NoError(t, err)

	// Should track deadline exceeded requests
	assert.GreaterOrEqual(t, stats.DeadlineExceededCount, int64(0), "Should track deadline exceeded count")
}

// TestBatchQueueManagerInterface_QueueCapacityLimits tests queue capacity and backpressure handling.
func TestBatchQueueManagerInterface_QueueCapacityLimits(t *testing.T) {
	// This test will fail until implementation is provided
	var manager BatchQueueManager
	require.Nil(t, manager, "BatchQueueManager implementation not yet available - this test defines expected behavior")

	ctx := context.Background()

	// Configure small queue size for testing limits
	config := &BatchConfig{
		MinBatchSize: 1,
		MaxBatchSize: 10,
		MaxQueueSize: 5, // Very small queue for testing
		BatchTimeout: 1 * time.Second,
	}

	err := manager.UpdateBatchConfiguration(ctx, config)
	require.NoError(t, err)

	// Fill queue to capacity
	requests := make([]*EmbeddingRequest, 5)
	for i := range 5 {
		requests[i] = &EmbeddingRequest{
			RequestID:   uuid.New().String(),
			Text:        "Request text",
			Priority:    PriorityInteractive,
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		}
	}

	err = manager.QueueBulkEmbeddingRequests(ctx, requests)
	assert.NoError(t, err, "Should be able to fill queue to capacity")

	// Attempt to add one more request - should fail
	extraRequest := &EmbeddingRequest{
		RequestID:   "overflow-request",
		Text:        "This should fail",
		Priority:    PriorityInteractive,
		Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
		SubmittedAt: time.Now(),
	}

	err = manager.QueueEmbeddingRequest(ctx, extraRequest)
	assert.Error(t, err, "Should reject request when queue is full")

	var qmErr *QueueManagerError
	if assert.ErrorAs(t, err, &qmErr) {
		assert.True(t, qmErr.IsQueueFull(), "Should indicate queue is full")
	}

	// Verify health reflects backpressure
	health, err := manager.GetQueueHealth(ctx)
	require.NoError(t, err)
	assert.True(t, health.QueueBackpressure, "Should indicate backpressure")
	assert.Equal(t, HealthStatusDegraded, health.Status, "Health should be degraded under backpressure")
}

// TestBatchQueueManagerInterface_GracefulShutdown tests graceful shutdown and draining.
func TestBatchQueueManagerInterface_GracefulShutdown(t *testing.T) {
	// This test will fail until implementation is provided
	var manager BatchQueueManager
	require.Nil(t, manager, "BatchQueueManager implementation not yet available - this test defines expected behavior")

	ctx := context.Background()

	// Start the manager
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Queue some requests
	requests := []*EmbeddingRequest{
		{
			RequestID:   "shutdown-001",
			Text:        "Request 1",
			Priority:    PriorityInteractive,
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "shutdown-002",
			Text:        "Request 2",
			Priority:    PriorityInteractive,
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
	}

	err = manager.QueueBulkEmbeddingRequests(ctx, requests)
	require.NoError(t, err)

	// Drain queue with timeout
	drainTimeout := 5 * time.Second
	err = manager.DrainQueue(ctx, drainTimeout)
	assert.NoError(t, err, "Should drain queue successfully")

	// Verify queue is empty after draining
	stats, err := manager.GetQueueStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.TotalQueueSize, "Queue should be empty after draining")

	// Stop the manager
	err = manager.Stop(ctx)
	assert.NoError(t, err, "Should stop gracefully")
}

// TestBatchQueueManagerInterface_HealthMonitoring tests health monitoring capabilities.
func TestBatchQueueManagerInterface_HealthMonitoring(t *testing.T) {
	// This test will fail until implementation is provided
	var manager BatchQueueManager
	require.Nil(t, manager, "BatchQueueManager implementation not yet available - this test defines expected behavior")

	ctx := context.Background()

	// Get health when not started - should indicate unhealthy
	health, err := manager.GetQueueHealth(ctx)
	require.NoError(t, err)
	assert.Equal(t, HealthStatusUnhealthy, health.Status, "Should be unhealthy when not started")

	// Start manager
	err = manager.Start(ctx)
	require.NoError(t, err)

	// Health should improve after starting
	health, err = manager.GetQueueHealth(ctx)
	require.NoError(t, err)
	assert.Equal(t, HealthStatusHealthy, health.Status, "Should be healthy after starting")

	// Verify component health is tracked
	assert.NotZero(t, health.NATSConnectionHealth.Status, "Should track NATS connection health")
	assert.NotZero(t, health.EmbeddingServiceHealth.Status, "Should track embedding service health")
	assert.NotZero(t, health.BatchAnalyzerHealth.Status, "Should track batch analyzer health")

	// Verify performance metrics are tracked
	assert.GreaterOrEqual(t, health.ProcessingLagSeconds, 0.0, "Should track processing lag")
	assert.GreaterOrEqual(t, health.ErrorRate, 0.0, "Should track error rate")

	// Stop manager
	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

// TestBatchQueueManagerInterface_IntegrationWithEmbeddingService tests integration with existing EmbeddingService.
func TestBatchQueueManagerInterface_IntegrationWithEmbeddingService(t *testing.T) {
	// This test will fail until implementation is provided
	var manager BatchQueueManager
	var processor BatchProcessor
	require.Nil(t, manager, "BatchQueueManager implementation not yet available - this test defines expected behavior")
	require.Nil(t, processor, "BatchProcessor implementation not yet available - this test defines expected behavior")

	ctx := context.Background()

	// Create requests that should be processed by the embedding service
	requests := []*EmbeddingRequest{
		{
			RequestID: "embed-001",
			Text:      "This is a sample text for embedding generation",
			Priority:  PriorityInteractive,
			Options: EmbeddingOptions{
				Model:           "gemini-embedding-001",
				TaskType:        TaskTypeRetrievalDocument,
				EnableBatching:  true,
				NormalizeVector: true,
			},
			SubmittedAt: time.Now(),
		},
		{
			RequestID: "embed-002",
			Text:      "Another text for batch processing",
			Priority:  PriorityInteractive,
			Options: EmbeddingOptions{
				Model:           "gemini-embedding-001",
				TaskType:        TaskTypeRetrievalDocument,
				EnableBatching:  true,
				NormalizeVector: true,
			},
			SubmittedAt: time.Now(),
		},
	}

	// Test batch processing integration
	results, err := processor.ProcessBatch(ctx, requests)
	assert.NoError(t, err, "Should process batch successfully")
	assert.Len(t, results, len(requests), "Should return result for each request")

	for i, result := range results {
		assert.Equal(t, requests[i].RequestID, result.RequestID, "Result should match request ID")
		assert.NotNil(t, result.Result, "Should have embedding result")
		assert.NotZero(t, result.ProcessedAt, "Should have processing timestamp")
	}

	// Test cost estimation
	cost, err := processor.EstimateBatchCost(ctx, requests)
	assert.NoError(t, err, "Should estimate cost successfully")
	assert.Greater(t, cost, 0.0, "Cost should be positive")

	// Test latency estimation
	latency, err := processor.EstimateBatchLatency(ctx, requests)
	assert.NoError(t, err, "Should estimate latency successfully")
	assert.Greater(t, latency, 0*time.Nanosecond, "Latency should be positive")
}

// TestBatchQueueManagerInterface_MetricsAndStatistics tests comprehensive metrics collection.
func TestBatchQueueManagerInterface_MetricsAndStatistics(t *testing.T) {
	// This test will fail until implementation is provided
	var manager BatchQueueManager
	require.Nil(t, manager, "BatchQueueManager implementation not yet available - this test defines expected behavior")

	ctx := context.Background()

	// Start manager and process some requests
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Queue requests with different priorities
	requests := []*EmbeddingRequest{
		{
			RequestID:   "metrics-rt",
			Text:        "Real-time",
			Priority:    PriorityRealTime,
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "metrics-int",
			Text:        "Interactive",
			Priority:    PriorityInteractive,
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "metrics-bg",
			Text:        "Background",
			Priority:    PriorityBackground,
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "metrics-batch",
			Text:        "Batch",
			Priority:    PriorityBatch,
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
	}

	err = manager.QueueBulkEmbeddingRequests(ctx, requests)
	require.NoError(t, err)

	// Process the queue
	batchCount, err := manager.ProcessQueue(ctx)
	require.NoError(t, err)
	assert.Positive(t, batchCount, "Should process batches")

	// Get comprehensive statistics
	stats, err := manager.GetQueueStats(ctx)
	require.NoError(t, err)

	// Verify queue size metrics are tracked
	assert.GreaterOrEqual(t, stats.TotalQueueSize, 0, "Should track total queue size")

	// Verify processing metrics are tracked
	assert.GreaterOrEqual(t, stats.TotalRequestsProcessed, int64(0), "Should track processed requests")
	assert.GreaterOrEqual(t, stats.TotalBatchesCreated, int64(0), "Should track created batches")

	// Verify throughput metrics are calculated
	assert.GreaterOrEqual(t, stats.RequestsPerSecond, 0.0, "Should calculate requests per second")
	assert.GreaterOrEqual(t, stats.BatchesPerSecond, 0.0, "Should calculate batches per second")

	// Verify efficiency metrics are tracked
	assert.GreaterOrEqual(t, stats.QueueUtilization, 0.0, "Should track queue utilization")
	assert.LessOrEqual(t, stats.QueueUtilization, 1.0, "Queue utilization should be <= 1.0")

	// Verify priority distribution is tracked
	assert.NotNil(t, stats.PriorityDistribution, "Should track priority distribution")

	// Verify timing information
	assert.NotZero(t, stats.QueueStartTime, "Should track queue start time")
	assert.GreaterOrEqual(t, stats.UptimeDuration, 0*time.Nanosecond, "Should track uptime")

	// Stop manager
	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

// generateBatchRequests is a helper function to generate batch priority requests for testing.
func generateBatchRequests(count int) []*EmbeddingRequest {
	requests := make([]*EmbeddingRequest, count)
	for i := range count {
		requests[i] = &EmbeddingRequest{
			RequestID:   uuid.New().String(),
			Text:        "Batch request text",
			Priority:    PriorityBatch,
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		}
	}
	return requests
}
