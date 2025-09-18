package queue

import (
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEmbeddingService provides a mock implementation for testing.
type mockEmbeddingService struct{}

func (m *mockEmbeddingService) GenerateEmbedding(
	ctx context.Context,
	text string,
	options outbound.EmbeddingOptions,
) (*outbound.EmbeddingResult, error) {
	return &outbound.EmbeddingResult{
		Vector:      []float64{0.1, 0.2, 0.3, 0.4, 0.5},
		Dimensions:  5,
		TokenCount:  len(text) / 4,
		Model:       options.Model,
		GeneratedAt: time.Now(),
	}, nil
}

func (m *mockEmbeddingService) GenerateBatchEmbeddings(
	ctx context.Context,
	texts []string,
	options outbound.EmbeddingOptions,
) ([]*outbound.EmbeddingResult, error) {
	results := make([]*outbound.EmbeddingResult, len(texts))
	for i, text := range texts {
		results[i] = &outbound.EmbeddingResult{
			Vector:      []float64{0.1, 0.2, 0.3, 0.4, 0.5},
			Dimensions:  5,
			TokenCount:  len(text) / 4,
			Model:       options.Model,
			GeneratedAt: time.Now(),
		}
	}
	return results, nil
}

func (m *mockEmbeddingService) GenerateCodeChunkEmbedding(
	ctx context.Context,
	chunk *outbound.CodeChunk,
	options outbound.EmbeddingOptions,
) (*outbound.CodeChunkEmbedding, error) {
	// Not implemented for queue tests
	return &outbound.CodeChunkEmbedding{}, nil
}

func (m *mockEmbeddingService) ValidateApiKey(ctx context.Context) error {
	return nil
}

func (m *mockEmbeddingService) GetModelInfo(ctx context.Context) (*outbound.ModelInfo, error) {
	return &outbound.ModelInfo{Name: "test-model"}, nil
}

func (m *mockEmbeddingService) GetSupportedModels(ctx context.Context) ([]string, error) {
	return []string{"test-model"}, nil
}

func (m *mockEmbeddingService) EstimateTokenCount(ctx context.Context, text string) (int, error) {
	return len(text) / 4, nil
}

// Test helper to create a manager with mocks.
func createTestManager() outbound.BatchQueueManager {
	embeddingService := &mockEmbeddingService{}
	batchProcessor := NewEmbeddingServiceBatchProcessor(embeddingService)

	return NewNATSBatchQueueManager(embeddingService, batchProcessor, nil)
}

// Helper to check error expectations.
func checkBulkRequestError(t *testing.T, err error, expectError bool, expectedType string) bool {
	t.Helper()
	if !expectError {
		return false
	}

	require.Error(t, err)
	if expectedType != "" {
		var qmErr *outbound.QueueManagerError
		require.ErrorAs(t, err, &qmErr)
		assert.Equal(t, expectedType, qmErr.Type)
	}
	return true
}

func TestNATSBatchQueueManager_QueueEmbeddingRequest(t *testing.T) {
	tests := []struct {
		name        string
		request     *outbound.EmbeddingRequest
		expectError bool
		errorType   string
	}{
		{
			name: "should queue valid real-time request",
			request: &outbound.EmbeddingRequest{
				RequestID:   "req-001",
				Text:        "Sample text for embedding",
				Priority:    outbound.PriorityRealTime,
				Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
			},
			expectError: false,
		},
		{
			name: "should reject request with empty RequestID",
			request: &outbound.EmbeddingRequest{
				RequestID:   "",
				Text:        "Sample text",
				Priority:    outbound.PriorityInteractive,
				Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
			},
			expectError: true,
			errorType:   "validation",
		},
		{
			name: "should reject request with empty text",
			request: &outbound.EmbeddingRequest{
				RequestID:   "req-005",
				Text:        "",
				Priority:    outbound.PriorityInteractive,
				Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
			},
			expectError: true,
			errorType:   "validation",
		},
		{
			name: "should reject request with invalid priority",
			request: &outbound.EmbeddingRequest{
				RequestID:   "req-006",
				Text:        "Sample text",
				Priority:    outbound.RequestPriority("invalid"),
				Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
			},
			expectError: true,
			errorType:   "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := createTestManager()
			ctx := context.Background()

			// Start the manager
			err := manager.Start(ctx)
			require.NoError(t, err)

			// Test the queue operation
			err = manager.QueueEmbeddingRequest(ctx, tt.request)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != "" {
					var qmErr *outbound.QueueManagerError
					if assert.ErrorAs(t, err, &qmErr) {
						assert.Equal(t, tt.errorType, qmErr.Type)
					}
				}
			} else {
				assert.NoError(t, err)

				// Verify request was queued
				stats, err := manager.GetQueueStats(ctx)
				require.NoError(t, err)
				assert.Positive(t, stats.TotalQueueSize)
			}

			// Stop the manager
			err = manager.Stop(ctx)
			assert.NoError(t, err)
		})
	}
}

func TestNATSBatchQueueManager_QueueBulkEmbeddingRequests(t *testing.T) {
	tests := []struct {
		name         string
		requests     []*outbound.EmbeddingRequest
		expectError  bool
		expectedType string
	}{
		{
			name: "should queue multiple valid requests with mixed priorities",
			requests: []*outbound.EmbeddingRequest{
				{
					RequestID:   "bulk-001",
					Text:        "Real-time request",
					Priority:    outbound.PriorityRealTime,
					Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "bulk-002",
					Text:        "Interactive request",
					Priority:    outbound.PriorityInteractive,
					Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
			},
			expectError: false,
		},
		{
			name:        "should handle empty request slice",
			requests:    []*outbound.EmbeddingRequest{},
			expectError: false,
		},
		{
			name:         "should reject nil request slice",
			requests:     nil,
			expectError:  true,
			expectedType: "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := createTestManager()
			ctx := context.Background()

			// Start the manager
			err := manager.Start(ctx)
			require.NoError(t, err)

			// Test bulk queuing
			err = manager.QueueBulkEmbeddingRequests(ctx, tt.requests)

			// Use helper to check error expectations
			if checkBulkRequestError(t, err, tt.expectError, tt.expectedType) {
				// Stop manager early for error cases
				_ = manager.Stop(ctx)
				return
			}

			// Handle success cases
			require.NoError(t, err)
			if len(tt.requests) > 0 {
				// Verify requests were queued
				stats, err := manager.GetQueueStats(ctx)
				require.NoError(t, err)
				assert.Equal(t, len(tt.requests), stats.TotalQueueSize)
			}

			// Stop the manager
			err = manager.Stop(ctx)
			assert.NoError(t, err)
		})
	}
}

func TestNATSBatchQueueManager_ProcessQueue(t *testing.T) {
	manager := createTestManager()
	ctx := context.Background()

	// Start the manager
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Queue requests with different priorities
	requests := []*outbound.EmbeddingRequest{
		{
			RequestID:   "rt-001",
			Text:        "Real-time 1",
			Priority:    outbound.PriorityRealTime,
			Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "int-001",
			Text:        "Interactive 1",
			Priority:    outbound.PriorityInteractive,
			Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "batch-001",
			Text:        "Batch 1",
			Priority:    outbound.PriorityBatch,
			Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
	}

	err = manager.QueueBulkEmbeddingRequests(ctx, requests)
	require.NoError(t, err)

	// Verify requests were queued
	stats, err := manager.GetQueueStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, len(requests), stats.TotalQueueSize)

	// Process the queue
	batchCount, err := manager.ProcessQueue(ctx)
	assert.NoError(t, err)
	assert.Positive(t, batchCount)

	// Stop the manager
	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

func TestNATSBatchQueueManager_PriorityOrdering(t *testing.T) {
	manager := createTestManager()
	ctx := context.Background()

	// Start the manager
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Queue requests in reverse priority order to test proper sorting
	requests := []*outbound.EmbeddingRequest{
		{
			RequestID:   "batch-001",
			Text:        "Batch priority",
			Priority:    outbound.PriorityBatch,
			Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "rt-001",
			Text:        "Real-time priority",
			Priority:    outbound.PriorityRealTime,
			Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
	}

	// Queue all requests
	err = manager.QueueBulkEmbeddingRequests(ctx, requests)
	require.NoError(t, err)

	// Get queue stats to verify priority distribution
	stats, err := manager.GetQueueStats(ctx)
	require.NoError(t, err)

	// Verify that requests are distributed to correct priority queues
	assert.Equal(t, 1, stats.RealTimeQueueSize, "Should have 1 real-time request")
	assert.Equal(t, 1, stats.BatchQueueSize, "Should have 1 batch request")
	assert.Equal(t, 2, stats.TotalQueueSize, "Should have 2 total requests")

	// Process queue
	batchCount, err := manager.ProcessQueue(ctx)
	assert.NoError(t, err)
	assert.Positive(t, batchCount, "Should create at least one batch")

	// Stop the manager
	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

func TestNATSBatchQueueManager_HealthMonitoring(t *testing.T) {
	manager := createTestManager()
	ctx := context.Background()

	// Get health when not started - should indicate unhealthy
	health, err := manager.GetQueueHealth(ctx)
	require.NoError(t, err)
	assert.Equal(t, outbound.HealthStatusUnhealthy, health.Status, "Should be unhealthy when not started")

	// Start manager
	err = manager.Start(ctx)
	require.NoError(t, err)

	// Health should improve after starting
	health, err = manager.GetQueueHealth(ctx)
	require.NoError(t, err)
	assert.Equal(t, outbound.HealthStatusHealthy, health.Status, "Should be healthy after starting")

	// Verify component health is tracked
	assert.NotZero(t, health.NATSConnectionHealth.Status, "Should track NATS connection health")
	assert.NotZero(t, health.EmbeddingServiceHealth.Status, "Should track embedding service health")
	assert.NotZero(t, health.BatchAnalyzerHealth.Status, "Should track batch analyzer health")

	// Stop manager
	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

func TestNATSBatchQueueManager_QueueCapacityLimits(t *testing.T) {
	manager := createTestManager()
	ctx := context.Background()

	// Start the manager
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Configure small queue size for testing limits
	config := &outbound.BatchConfig{
		MinBatchSize: 1,
		MaxBatchSize: 10,
		MaxQueueSize: 2, // Very small queue for testing
		BatchTimeout: 1 * time.Second,
	}

	err = manager.UpdateBatchConfiguration(ctx, config)
	require.NoError(t, err)

	// Fill queue to capacity
	requests := []*outbound.EmbeddingRequest{
		{
			RequestID:   "req-001",
			Text:        "Request 1",
			Priority:    outbound.PriorityInteractive,
			Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "req-002",
			Text:        "Request 2",
			Priority:    outbound.PriorityInteractive,
			Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
	}

	err = manager.QueueBulkEmbeddingRequests(ctx, requests)
	assert.NoError(t, err, "Should be able to fill queue to capacity")

	// Attempt to add one more request - should fail
	extraRequest := &outbound.EmbeddingRequest{
		RequestID:   "overflow-request",
		Text:        "This should fail",
		Priority:    outbound.PriorityInteractive,
		Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
		SubmittedAt: time.Now(),
	}

	err = manager.QueueEmbeddingRequest(ctx, extraRequest)
	assert.Error(t, err, "Should reject request when queue is full")

	var qmErr *outbound.QueueManagerError
	if assert.ErrorAs(t, err, &qmErr) {
		assert.True(t, qmErr.IsQueueFull(), "Should indicate queue is full")
	}

	// Stop the manager
	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

func TestNATSBatchQueueManager_UpdateBatchConfiguration(t *testing.T) {
	manager := createTestManager()
	ctx := context.Background()

	// Test valid configuration update
	config := &outbound.BatchConfig{
		MinBatchSize: 5,
		MaxBatchSize: 50,
		BatchTimeout: 2 * time.Second,
		MaxQueueSize: 500,
	}

	err := manager.UpdateBatchConfiguration(ctx, config)
	assert.NoError(t, err)

	// Test invalid configuration - nil
	err = manager.UpdateBatchConfiguration(ctx, nil)
	assert.Error(t, err)

	// Test invalid configuration - MinBatchSize < 1
	invalidConfig := &outbound.BatchConfig{
		MinBatchSize: 0,
		MaxBatchSize: 50,
	}

	err = manager.UpdateBatchConfiguration(ctx, invalidConfig)
	assert.Error(t, err)

	// Test invalid configuration - MaxBatchSize < MinBatchSize
	invalidConfig2 := &outbound.BatchConfig{
		MinBatchSize: 50,
		MaxBatchSize: 10,
	}

	err = manager.UpdateBatchConfiguration(ctx, invalidConfig2)
	assert.Error(t, err)
}

func TestNATSBatchQueueManager_DrainQueue(t *testing.T) {
	manager := createTestManager()
	ctx := context.Background()

	// Start the manager
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Queue some requests
	requests := []*outbound.EmbeddingRequest{
		{
			RequestID:   "drain-001",
			Text:        "Request 1",
			Priority:    outbound.PriorityInteractive,
			Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "drain-002",
			Text:        "Request 2",
			Priority:    outbound.PriorityInteractive,
			Options:     outbound.EmbeddingOptions{Model: "gemini-embedding-001"},
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
	assert.NoError(t, err)
}
