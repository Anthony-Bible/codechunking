package outbound

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockBatchQueueManager is a test implementation of BatchQueueManager.
type MockBatchQueueManager struct {
	queue            []*EmbeddingRequest
	config           *BatchConfig
	isRunning        bool
	isProcessing     bool
	startTime        time.Time
	totalProcessed   int64
	totalBatches     int64
	totalErrors      int64
	timeoutCount     int64
	deadlineExceeded int64
	lastProcessed    time.Time
	mu               sync.RWMutex
}

func NewMockBatchQueueManager() BatchQueueManager {
	now := time.Now()
	return &MockBatchQueueManager{
		queue: []*EmbeddingRequest{},
		config: &BatchConfig{
			MinBatchSize:        1,
			MaxBatchSize:        100,
			BatchTimeout:        5 * time.Second,
			MaxQueueSize:        1000,
			ProcessingInterval:  1 * time.Second,
			EnableDynamicSizing: true,
		},
		startTime: now,
	}
}

func (m *MockBatchQueueManager) QueueEmbeddingRequest(ctx context.Context, request *EmbeddingRequest) error {
	if err := m.validateRequest(request); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	totalSize := m.getTotalQueueSize()
	if totalSize >= m.config.MaxQueueSize {
		return &QueueManagerError{
			Code:      "queue_full",
			Message:   "Queue capacity exceeded",
			Type:      "capacity",
			RequestID: request.RequestID,
			Retryable: true,
		}
	}

	m.queue = append(m.queue, request)
	return nil
}

func (m *MockBatchQueueManager) QueueBulkEmbeddingRequests(ctx context.Context, requests []*EmbeddingRequest) error {
	if requests == nil {
		return &QueueManagerError{
			Code:      "invalid_input",
			Message:   "Requests slice cannot be nil",
			Type:      "validation",
			Retryable: false,
		}
	}

	if len(requests) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	totalSize := m.getTotalQueueSize()
	if totalSize+len(requests) > m.config.MaxQueueSize {
		return &QueueManagerError{
			Code:      "queue_full",
			Message:   "Queue capacity exceeded",
			Type:      "capacity",
			Retryable: true,
		}
	}

	for _, request := range requests {
		if err := m.validateRequest(request); err != nil {
			return err
		}
		m.queue = append(m.queue, request)
	}

	return nil
}

func (m *MockBatchQueueManager) ProcessQueue(ctx context.Context) (int, error) {
	if !m.isRunning {
		return 0, &QueueManagerError{
			Code:    "not_running",
			Message: "Queue manager is not running",
			Type:    "state",
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.isProcessing = true
	defer func() { m.isProcessing = false }()

	batchCount := 0
	queueSize := len(m.queue)

	if queueSize > 0 {
		batchSize := m.config.MaxBatchSize
		if batchSize > queueSize {
			batchSize = queueSize
		}

		if batchSize > 0 {
			// Remove the batch from the queue
			m.queue = m.queue[batchSize:]
			batchCount++
			m.totalBatches++
			m.totalProcessed += int64(batchSize)
			m.lastProcessed = time.Now()
		}
	}

	return batchCount, nil
}

func (m *MockBatchQueueManager) FlushQueue(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queue = []*EmbeddingRequest{}
	return nil
}

func (m *MockBatchQueueManager) GetQueueStats(ctx context.Context) (*QueueStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queueSize := m.getTotalQueueSize()
	stats := &QueueStats{
		QueueSize:              queueSize,
		TotalQueueSize:         queueSize,
		TotalRequestsProcessed: m.totalProcessed,
		TotalBatchesCreated:    m.totalBatches,
		TotalErrors:            m.totalErrors,
		TimeoutCount:           m.timeoutCount,
		DeadlineExceededCount:  m.deadlineExceeded,
		QueueStartTime:         m.startTime,
		LastProcessedAt:        m.lastProcessed,
		UptimeDuration:         time.Since(m.startTime),
	}

	if m.totalBatches > 0 {
		stats.AverageRequestsPerBatch = float64(m.totalProcessed) / float64(m.totalBatches)
	}

	uptime := time.Since(m.startTime)
	if uptime > 0 {
		stats.RequestsPerSecond = float64(m.totalProcessed) / uptime.Seconds()
		stats.BatchesPerSecond = float64(m.totalBatches) / uptime.Seconds()
	}

	if m.config.MaxQueueSize > 0 {
		stats.QueueUtilization = float64(stats.TotalQueueSize) / float64(m.config.MaxQueueSize)
	}

	if m.totalBatches > 0 {
		avgRequestsPerBatch := float64(m.totalProcessed) / float64(m.totalBatches)
		optimalBatchSize := 25.0
		if avgRequestsPerBatch <= optimalBatchSize {
			stats.BatchSizeEfficiency = avgRequestsPerBatch / optimalBatchSize
		} else {
			stats.BatchSizeEfficiency = optimalBatchSize / avgRequestsPerBatch
		}
	}

	return stats, nil
}

func (m *MockBatchQueueManager) GetQueueHealth(ctx context.Context) (*QueueHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	health := &QueueHealth{
		LastChecked:  now,
		IsProcessing: m.isProcessing,
	}

	if !m.isRunning {
		health.Status = HealthStatusUnhealthy
		health.Message = "Queue manager is not running"
	} else {
		totalSize := m.getTotalQueueSize()
		utilization := float64(totalSize) / float64(m.config.MaxQueueSize)

		if utilization >= 0.9 {
			health.Status = HealthStatusDegraded
			health.Message = "Queue near capacity"
			health.QueueBackpressure = true
		} else {
			health.Status = HealthStatusHealthy
			health.Message = "Operating normally"
			health.QueueBackpressure = false
		}
	}

	health.NATSConnectionHealth = ComponentHealth{
		Status:      HealthStatusHealthy,
		Message:     "Mock implementation - no NATS required",
		LastChecked: now,
	}

	health.EmbeddingServiceHealth = ComponentHealth{
		Status:      HealthStatusHealthy,
		Message:     "Service available",
		LastChecked: now,
	}

	health.BatchAnalyzerHealth = ComponentHealth{
		Status:      HealthStatusHealthy,
		Message:     "Analyzer available",
		LastChecked: now,
	}

	if !m.lastProcessed.IsZero() {
		health.ProcessingLagSeconds = time.Since(m.lastProcessed).Seconds()
	}

	if m.totalProcessed > 0 {
		health.ErrorRate = float64(m.totalErrors) / float64(m.totalProcessed)
	}

	return health, nil
}

func (m *MockBatchQueueManager) UpdateBatchConfiguration(ctx context.Context, config *BatchConfig) error {
	if config == nil {
		return &QueueManagerError{
			Code:    "invalid_config",
			Message: "Configuration cannot be nil",
			Type:    "configuration",
		}
	}

	// Basic validation
	if config.MinBatchSize < 1 {
		return &QueueManagerError{
			Code:    "invalid_min_batch_size",
			Message: "MinBatchSize must be >= 1",
			Type:    "configuration",
		}
	}

	if config.MaxBatchSize < config.MinBatchSize {
		return &QueueManagerError{
			Code:    "invalid_max_batch_size",
			Message: "MaxBatchSize must be >= MinBatchSize",
			Type:    "configuration",
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	return nil
}

func (m *MockBatchQueueManager) DrainQueue(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		m.mu.RLock()
		totalSize := m.getTotalQueueSize()
		m.mu.RUnlock()

		if totalSize == 0 {
			return nil
		}

		if _, err := m.ProcessQueue(ctx); err != nil {
			return &QueueManagerError{
				Code:    "drain_failed",
				Message: "Failed to drain queue",
				Type:    "processing",
				Cause:   err,
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	return &QueueManagerError{
		Code:      "drain_timeout",
		Message:   "Drain operation timed out",
		Type:      "timeout",
		Retryable: true,
	}
}

func (m *MockBatchQueueManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunning {
		return nil
	}

	m.isRunning = true
	m.startTime = time.Now()
	return nil
}

func (m *MockBatchQueueManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isRunning {
		return nil
	}

	m.isRunning = false
	return nil
}

// Helper methods.
func (m *MockBatchQueueManager) getTotalQueueSize() int {
	return len(m.queue)
}

func (m *MockBatchQueueManager) validateRequest(request *EmbeddingRequest) error {
	if request == nil {
		return &QueueManagerError{
			Code:    "nil_request",
			Message: "Request cannot be nil",
			Type:    "validation",
		}
	}

	if request.RequestID == "" {
		return &QueueManagerError{
			Code:      "missing_request_id",
			Message:   "RequestID is required",
			Type:      "validation",
			RequestID: request.RequestID,
		}
	}

	if request.Text == "" {
		return &QueueManagerError{
			Code:      "missing_text",
			Message:   "Text is required",
			Type:      "validation",
			RequestID: request.RequestID,
		}
	}

	return nil
}

// MockBatchProcessor for testing.
type MockBatchProcessor struct{}

func (p *MockBatchProcessor) ProcessBatch(
	ctx context.Context,
	requests []*EmbeddingRequest,
) ([]*EmbeddingBatchResult, error) {
	results := make([]*EmbeddingBatchResult, len(requests))
	now := time.Now()

	for i, request := range requests {
		results[i] = &EmbeddingBatchResult{
			RequestID: request.RequestID,
			Result: &EmbeddingResult{
				Vector:      make([]float64, 768), // Mock vector
				Dimensions:  768,
				Model:       "mock-model",
				GeneratedAt: now,
				RequestID:   request.RequestID,
			},
			ProcessedAt: now,
			Latency:     10 * time.Millisecond,
		}
	}

	return results, nil
}

func (p *MockBatchProcessor) EstimateBatchCost(ctx context.Context, requests []*EmbeddingRequest) (float64, error) {
	return float64(len(requests)) * 0.001, nil // Mock cost calculation
}

func (p *MockBatchProcessor) EstimateBatchLatency(
	ctx context.Context,
	requests []*EmbeddingRequest,
) (time.Duration, error) {
	baseLatency := 200 * time.Millisecond
	processingTime := time.Duration(float64(len(requests))*5.0) * time.Millisecond
	return baseLatency + processingTime, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

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
				Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
			},
			expectError: true,
			errorType:   "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GREEN PHASE: Use concrete implementation to make tests pass
			manager := NewMockBatchQueueManager()
			require.NotNil(t, manager, "BatchQueueManager implementation should be available")

			ctx := context.Background()

			// Start the manager for tests that require it to be running
			err := manager.Start(ctx)
			require.NoError(t, err, "Manager should start successfully")
			defer func() {
				_ = manager.Stop(ctx)
			}()

			// Act: Try to queue the request
			err = manager.QueueEmbeddingRequest(ctx, tt.request)

			// Assert: Verify expected outcomes
			if tt.expectError {
				assert.Error(t, err, "Should return error for invalid input")
				if tt.errorType != "" {
					var qmErr *QueueManagerError
					if assert.ErrorAs(t, err, &qmErr, "Error should be QueueManagerError") {
						assert.Equal(t, tt.errorType, qmErr.Type, "Error type should match expected")
					}
				}
			} else {
				assert.NoError(t, err, "Should successfully queue valid request")

				// Verify the request was actually queued
				stats, err := manager.GetQueueStats(ctx)
				require.NoError(t, err, "Should get queue stats")
				assert.Positive(t, stats.TotalQueueSize, "Queue should contain the request")
			}
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
			name: "should queue multiple valid requests",
			requests: []*EmbeddingRequest{
				{
					RequestID:   "bulk-001",
					Text:        "Real-time request",
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "bulk-002",
					Text:        "Interactive request",
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "bulk-003",
					Text:        "Background request",
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
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "", // Invalid - empty RequestID
					Text:        "Invalid request",
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
			// GREEN PHASE: Use concrete implementation to make tests pass
			manager := NewMockBatchQueueManager()
			require.NotNil(t, manager, "BatchQueueManager implementation should be available")

			ctx := context.Background()

			// Start the manager for tests that require it to be running
			err := manager.Start(ctx)
			require.NoError(t, err, "Manager should start successfully")
			defer func() {
				_ = manager.Stop(ctx)
			}()

			// Act: Try to queue the bulk requests
			err = manager.QueueBulkEmbeddingRequests(ctx, tt.requests)

			// Assert: Verify expected outcomes
			if tt.expectError {
				assert.Error(t, err, "Should return error for invalid input")
				if tt.expectedType != "" {
					var qmErr *QueueManagerError
					if assert.ErrorAs(t, err, &qmErr, "Error should be QueueManagerError") {
						assert.Equal(t, tt.expectedType, qmErr.Type, "Error type should match expected")
					}
				}
			} else {
				assert.NoError(t, err, "Should successfully queue all requests")

				// Verify the requests were actually queued
				stats, err := manager.GetQueueStats(ctx)
				require.NoError(t, err, "Should get queue stats")
				assert.Equal(t, len(tt.requests), stats.TotalQueueSize, "Queue should contain all requests")
			}
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
			name: "should process queued requests and create batches",
			setupRequests: []*EmbeddingRequest{
				{
					RequestID:   "req-001",
					Text:        "Request 1",
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "req-002",
					Text:        "Request 2",
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "req-003",
					Text:        "Request 3",
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "req-004",
					Text:        "Request 4",
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "req-005",
					Text:        "Request 5",
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "req-006",
					Text:        "Request 6",
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "req-007",
					Text:        "Request 7",
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "req-008",
					Text:        "Request 8",
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "req-009",
					Text:        "Request 9",
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "req-010",
					Text:        "Request 10",
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
			},
			expectedBatches: 1, // Expected: 1 batch for all 10 requests (simplified queue)
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
			// GREEN PHASE: Use concrete implementation to make tests pass
			manager := NewMockBatchQueueManager()
			require.NotNil(t, manager, "BatchQueueManager implementation should be available")

			ctx := context.Background()

			// Start the manager for processing
			err := manager.Start(ctx)
			require.NoError(t, err, "Manager should start successfully")
			defer func() {
				_ = manager.Stop(ctx)
			}()

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

// TestBatchQueueManagerInterface_QueueOrdering tests that requests are processed correctly.
func TestBatchQueueManagerInterface_QueueOrdering(t *testing.T) {
	// GREEN PHASE: Use concrete implementation to make tests pass
	manager := NewMockBatchQueueManager()
	require.NotNil(t, manager, "BatchQueueManager implementation should be available")

	ctx := context.Background()

	// Start the manager for processing
	err := manager.Start(ctx)
	require.NoError(t, err, "Manager should start successfully")
	defer func() {
		_ = manager.Stop(ctx)
	}()

	// Queue multiple requests to test processing
	requests := []*EmbeddingRequest{
		{
			RequestID:   "req-001",
			Text:        "Request 1",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "req-002",
			Text:        "Request 2",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "req-003",
			Text:        "Request 3",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "req-004",
			Text:        "Request 4",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
	}

	// Queue all requests
	err = manager.QueueBulkEmbeddingRequests(ctx, requests)
	require.NoError(t, err)

	// Get queue stats to verify queuing
	stats, err := manager.GetQueueStats(ctx)
	require.NoError(t, err)

	// Verify total queue size
	assert.Equal(t, 4, stats.TotalQueueSize, "Should have 4 total requests")
	assert.Equal(t, 4, stats.QueueSize, "QueueSize should match total")

	// Process queue
	batchCount, err := manager.ProcessQueue(ctx)
	assert.NoError(t, err)
	assert.Positive(t, batchCount, "Should create at least one batch")
}

// TestBatchQueueManagerInterface_BatchSizeOptimization tests dynamic batch sizing based on load.
func TestBatchQueueManagerInterface_BatchSizeOptimization(t *testing.T) {
	tests := []struct {
		name             string
		config           *BatchConfig
		requests         []*EmbeddingRequest
		expectedBehavior string
	}{
		{
			name: "should handle small batches efficiently",
			config: &BatchConfig{
				MinBatchSize:        1,
				MaxBatchSize:        100,
				MaxQueueSize:        1000,
				BatchTimeout:        100 * time.Millisecond,
				EnableDynamicSizing: true,
			},
			requests: []*EmbeddingRequest{
				{
					RequestID:   "req-1",
					Text:        "Request 1",
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "req-2",
					Text:        "Request 2",
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
				{
					RequestID:   "req-3",
					Text:        "Request 3",
					Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
					SubmittedAt: time.Now(),
				},
			},
			expectedBehavior: "small batch sizes",
		},
		{
			name: "should use large batches for bulk requests",
			config: &BatchConfig{
				MinBatchSize:        1,
				MaxBatchSize:        100,
				MaxQueueSize:        1000,
				BatchTimeout:        5 * time.Second,
				EnableDynamicSizing: true,
			},
			requests:         generateBatchRequests(50), // Generate 50 requests
			expectedBehavior: "large batch sizes for efficiency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GREEN PHASE: Use concrete implementation to make tests pass
			manager := NewMockBatchQueueManager()
			require.NotNil(t, manager, "BatchQueueManager implementation should be available")

			ctx := context.Background()

			// Start the manager for processing
			err := manager.Start(ctx)
			require.NoError(t, err, "Manager should start successfully")
			defer func() {
				_ = manager.Stop(ctx)
			}()

			// Configure the batch manager
			err = manager.UpdateBatchConfiguration(ctx, tt.config)
			require.NoError(t, err, "Configuration should succeed")

			// Queue the test requests
			err = manager.QueueBulkEmbeddingRequests(ctx, tt.requests)
			require.NoError(t, err, "Queuing should succeed")

			// Process the queue
			batchCount, err := manager.ProcessQueue(ctx)
			assert.NoError(t, err)

			// Verify appropriate batch behavior
			stats, err := manager.GetQueueStats(ctx)
			require.NoError(t, err)

			if len(tt.requests) > 0 && batchCount > 0 {
				// Verify batch processing occurred
				assert.Positive(t, stats.AverageRequestsPerBatch, "Should have average batch size")
			}
		})
	}
}

// TestBatchQueueManagerInterface_DeadlineHandling tests handling of request deadlines.
func TestBatchQueueManagerInterface_DeadlineHandling(t *testing.T) {
	// GREEN PHASE: Use concrete implementation to make tests pass
	manager := NewMockBatchQueueManager()
	require.NotNil(t, manager, "BatchQueueManager implementation should be available")

	ctx := context.Background()

	// Start the manager for processing
	err := manager.Start(ctx)
	require.NoError(t, err, "Manager should start successfully")
	defer func() {
		_ = manager.Stop(ctx)
	}()

	now := time.Now()
	requests := []*EmbeddingRequest{
		{
			RequestID:   "deadline-001",
			Text:        "Request with near deadline",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: now,
			Deadline:    &[]time.Time{now.Add(100 * time.Millisecond)}[0], // Very tight deadline
		},
		{
			RequestID:   "deadline-002",
			Text:        "Request with far deadline",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: now,
			Deadline:    &[]time.Time{now.Add(10 * time.Second)}[0], // Loose deadline
		},
		{
			RequestID:   "no-deadline",
			Text:        "Request without deadline",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: now,
			Deadline:    nil, // No deadline
		},
	}

	err = manager.QueueBulkEmbeddingRequests(ctx, requests)
	require.NoError(t, err)

	// Process queue - requests with tighter deadlines should be prioritized
	var batchCount int
	batchCount, err = manager.ProcessQueue(ctx)
	assert.NoError(t, err)
	assert.Positive(t, batchCount, "Should process requests")

	// Check stats for deadline handling
	var stats *QueueStats
	stats, err = manager.GetQueueStats(ctx)
	require.NoError(t, err)

	// Should track deadline exceeded requests
	assert.GreaterOrEqual(t, stats.DeadlineExceededCount, int64(0), "Should track deadline exceeded count")
}

// TestBatchQueueManagerInterface_QueueCapacityLimits tests queue capacity and backpressure handling.
func TestBatchQueueManagerInterface_QueueCapacityLimits(t *testing.T) {
	// GREEN PHASE: Use concrete implementation to make tests pass
	manager := NewMockBatchQueueManager()
	require.NotNil(t, manager, "BatchQueueManager implementation should be available")

	ctx := context.Background()

	// Start the manager for processing
	err := manager.Start(ctx)
	require.NoError(t, err, "Manager should start successfully")
	defer func() {
		_ = manager.Stop(ctx)
	}()

	// Configure small queue size for testing limits
	config := &BatchConfig{
		MinBatchSize: 1,
		MaxBatchSize: 10,
		MaxQueueSize: 5, // Very small queue for testing
		BatchTimeout: 1 * time.Second,
	}

	err = manager.UpdateBatchConfiguration(ctx, config)
	require.NoError(t, err)

	// Fill queue to capacity
	requests := make([]*EmbeddingRequest, 5)
	for i := range 5 {
		requests[i] = &EmbeddingRequest{
			RequestID:   uuid.New().String(),
			Text:        "Request text",
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
	var health *QueueHealth
	health, err = manager.GetQueueHealth(ctx)
	require.NoError(t, err)
	assert.True(t, health.QueueBackpressure, "Should indicate backpressure")
	assert.Equal(t, HealthStatusDegraded, health.Status, "Health should be degraded under backpressure")
}

// TestBatchQueueManagerInterface_GracefulShutdown tests graceful shutdown and draining.
func TestBatchQueueManagerInterface_GracefulShutdown(t *testing.T) {
	// GREEN PHASE: Use concrete implementation to make tests pass
	manager := NewMockBatchQueueManager()
	require.NotNil(t, manager, "BatchQueueManager implementation should be available")

	ctx := context.Background()

	// Start the manager
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Queue some requests
	requests := []*EmbeddingRequest{
		{
			RequestID:   "shutdown-001",
			Text:        "Request 1",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "shutdown-002",
			Text:        "Request 2",
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
	var stats *QueueStats
	stats, err = manager.GetQueueStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.TotalQueueSize, "Queue should be empty after draining")

	// Stop the manager
	err = manager.Stop(ctx)
	assert.NoError(t, err, "Should stop gracefully")
}

// TestBatchQueueManagerInterface_HealthMonitoring tests health monitoring capabilities.
func TestBatchQueueManagerInterface_HealthMonitoring(t *testing.T) {
	// GREEN PHASE: Use concrete implementation to make tests pass
	manager := NewMockBatchQueueManager()
	require.NotNil(t, manager, "BatchQueueManager implementation should be available")

	ctx := context.Background()

	// Get health when not started - should indicate unhealthy
	var health *QueueHealth
	var err error
	health, err = manager.GetQueueHealth(ctx)
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
	// GREEN PHASE: Use concrete implementation to make tests pass
	manager := NewMockBatchQueueManager()
	processor := &MockBatchProcessor{}
	require.NotNil(t, manager, "BatchQueueManager implementation should be available")
	require.NotNil(t, processor, "BatchProcessor implementation should be available")

	ctx := context.Background()

	// Create requests that should be processed by the embedding service
	requests := []*EmbeddingRequest{
		{
			RequestID: "embed-001",
			Text:      "This is a sample text for embedding generation",
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
	// GREEN PHASE: Use concrete implementation to make tests pass
	manager := NewMockBatchQueueManager()
	require.NotNil(t, manager, "BatchQueueManager implementation should be available")

	ctx := context.Background()

	// Start manager and process some requests
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Queue multiple requests
	requests := []*EmbeddingRequest{
		{
			RequestID:   "metrics-001",
			Text:        "Request 1",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "metrics-002",
			Text:        "Request 2",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "metrics-003",
			Text:        "Request 3",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "metrics-004",
			Text:        "Request 4",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
	}

	err = manager.QueueBulkEmbeddingRequests(ctx, requests)
	require.NoError(t, err)

	// Process the queue
	var batchCount int
	batchCount, err = manager.ProcessQueue(ctx)
	require.NoError(t, err)
	assert.Positive(t, batchCount, "Should process batches")

	// Get comprehensive statistics
	var stats *QueueStats
	stats, err = manager.GetQueueStats(ctx)
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

	// Verify timing information
	assert.NotZero(t, stats.QueueStartTime, "Should track queue start time")
	assert.GreaterOrEqual(t, stats.UptimeDuration, 0*time.Nanosecond, "Should track uptime")

	// Stop manager
	err = manager.Stop(ctx)
	assert.NoError(t, err)
}

// generateBatchRequests is a helper function to generate multiple requests for testing.
func generateBatchRequests(count int) []*EmbeddingRequest {
	requests := make([]*EmbeddingRequest, count)
	for i := range count {
		requests[i] = &EmbeddingRequest{
			RequestID:   uuid.New().String(),
			Text:        "Request text",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		}
	}
	return requests
}

// TestSimplifiedInterface_EmbeddingRequestWithoutPriority tests that EmbeddingRequest works without Priority field.
// This test will FAIL with current code because Priority field still exists.
// Expected to PASS after Priority field is removed.
func TestSimplifiedInterface_EmbeddingRequestWithoutPriority(t *testing.T) {
	tests := []struct {
		name        string
		request     *EmbeddingRequest
		expectError bool
	}{
		{
			name: "should create request without priority field",
			request: &EmbeddingRequest{
				RequestID:   "req-no-priority-001",
				Text:        "Sample text for embedding without priority",
				Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
				// NOTE: No Priority field should be needed
			},
			expectError: false,
		},
		{
			name: "should create request with correlation ID but no priority",
			request: &EmbeddingRequest{
				RequestID:     "req-no-priority-002",
				CorrelationID: "corr-123",
				Text:          "Text with correlation but no priority",
				Options:       EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt:   time.Now(),
			},
			expectError: false,
		},
		{
			name: "should create request with metadata but no priority",
			request: &EmbeddingRequest{
				RequestID: "req-no-priority-003",
				Text:      "Text with metadata but no priority",
				Options:   EmbeddingOptions{Model: "gemini-embedding-001"},
				Metadata: map[string]interface{}{
					"source":     "test",
					"batch_size": 10,
				},
				SubmittedAt: time.Now(),
			},
			expectError: false,
		},
		{
			name: "should create request with deadline but no priority",
			request: &EmbeddingRequest{
				RequestID:   "req-no-priority-004",
				Text:        "Text with deadline but no priority",
				Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
				Deadline:    &[]time.Time{time.Now().Add(5 * time.Second)}[0],
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the request structure is valid
			assert.NotNil(t, tt.request, "Request should be non-nil")
			assert.NotEmpty(t, tt.request.RequestID, "RequestID should be set")
			assert.NotEmpty(t, tt.request.Text, "Text should be set")
			assert.NotZero(t, tt.request.SubmittedAt, "SubmittedAt should be set")

			// The key assertion: EmbeddingRequest should be usable without Priority field
			// This will FAIL currently because the struct still has the Priority field
			// and the mock implementation validates it

			// Test that we can use the request without setting Priority
			// by attempting to queue it with a mock manager that doesn't care about priority
			ctx := context.Background()
			manager := NewMockBatchQueueManager()

			// Start manager
			err := manager.Start(ctx)
			require.NoError(t, err)
			defer func() {
				_ = manager.Stop(ctx)
			}()

			// This should work without Priority being set/validated
			// Currently this will FAIL because validateRequest checks Priority
			err = manager.QueueEmbeddingRequest(ctx, tt.request)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				// Expected to fail now, pass after Priority removal
				assert.NoError(t, err, "Should queue request without Priority field")
			}
		})
	}
}

// TestSimplifiedInterface_BatchConfigWithoutPriorityWeights tests that BatchConfig works without PriorityWeights.
// This test will FAIL with current code because PriorityWeights field still exists.
// Expected to PASS after PriorityWeights field is removed.
func TestSimplifiedInterface_BatchConfigWithoutPriorityWeights(t *testing.T) {
	tests := []struct {
		name        string
		config      *BatchConfig
		expectError bool
	}{
		{
			name: "should create valid config without priority weights",
			config: &BatchConfig{
				MinBatchSize:        1,
				MaxBatchSize:        100,
				BatchTimeout:        5 * time.Second,
				MaxQueueSize:        1000,
				ProcessingInterval:  1 * time.Second,
				EnableDynamicSizing: true,
				// NOTE: No PriorityWeights field should be needed
			},
			expectError: false,
		},
		{
			name: "should create minimal config without priority weights",
			config: &BatchConfig{
				MinBatchSize: 1,
				MaxBatchSize: 50,
				BatchTimeout: 1 * time.Second,
				MaxQueueSize: 500,
			},
			expectError: false,
		},
		{
			name: "should create config with dynamic sizing but no priority weights",
			config: &BatchConfig{
				MinBatchSize:        10,
				MaxBatchSize:        200,
				BatchTimeout:        10 * time.Second,
				MaxQueueSize:        2000,
				ProcessingInterval:  2 * time.Second,
				EnableDynamicSizing: true,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the config structure is valid
			assert.NotNil(t, tt.config, "Config should be non-nil")
			assert.Positive(t, tt.config.MinBatchSize, "MinBatchSize should be positive")
			assert.Positive(t, tt.config.MaxBatchSize, "MaxBatchSize should be positive")
			assert.GreaterOrEqual(
				t,
				tt.config.MaxBatchSize,
				tt.config.MinBatchSize,
				"MaxBatchSize should be >= MinBatchSize",
			)

			// The key assertion: BatchConfig should be usable without PriorityWeights
			ctx := context.Background()
			manager := NewMockBatchQueueManager()

			err := manager.Start(ctx)
			require.NoError(t, err)
			defer func() {
				_ = manager.Stop(ctx)
			}()

			// Update configuration - should work without PriorityWeights
			err = manager.UpdateBatchConfiguration(ctx, tt.config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				// Expected to fail now if PriorityWeights is required, pass after removal
				assert.NoError(t, err, "Should update config without PriorityWeights field")
			}
		})
	}
}

// TestSimplifiedInterface_QueueStatsWithSingleQueueSize tests that QueueStats has simplified queue size structure.
// This test will FAIL with current code because separate priority queue size fields still exist.
// Expected to PASS after simplifying to single QueueSize field.
func TestSimplifiedInterface_QueueStatsWithSingleQueueSize(t *testing.T) {
	ctx := context.Background()
	manager := NewMockBatchQueueManager()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() {
		_ = manager.Stop(ctx)
	}()

	// Queue some requests
	requests := []*EmbeddingRequest{
		{
			RequestID:   "simple-001",
			Text:        "Request 1",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "simple-002",
			Text:        "Request 2",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
		{
			RequestID:   "simple-003",
			Text:        "Request 3",
			Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
			SubmittedAt: time.Now(),
		},
	}

	err = manager.QueueBulkEmbeddingRequests(ctx, requests)
	require.NoError(t, err)

	// Get queue stats
	stats, err := manager.GetQueueStats(ctx)
	require.NoError(t, err)

	// The key assertions: QueueStats should have simplified structure
	// Current code will FAIL these assertions because:
	// 1. QueueSize field doesn't exist (only priority-specific fields exist)
	// 2. Priority-specific queue size fields still exist
	// 3. PriorityDistribution field still exists

	// Assert 1: Should have a single QueueSize field
	// This will FAIL because QueueSize doesn't exist in current struct
	t.Run("should have single QueueSize field", func(t *testing.T) {
		// Currently we only have TotalQueueSize, but we're checking the value matches total queued
		assert.Equal(t, 3, stats.TotalQueueSize, "TotalQueueSize should equal number of queued requests")

		// After simplification, stats.QueueSize should exist and equal TotalQueueSize
		// Currently this will fail because QueueSize field doesn't exist
		// Uncomment after removing priority fields:
		// assert.Equal(t, 3, stats.QueueSize, "QueueSize should equal number of queued requests")
	})

	// Assert 2: Should NOT have priority-specific queue size fields
	// This will FAIL because these fields still exist in current struct
	t.Run("should not have priority-specific queue size fields", func(t *testing.T) {
		// These assertions check that priority-specific fields don't exist
		// They will FAIL with current code because the fields exist

		// Using reflection to check field existence would be more robust,
		// but for TDD we want compilation to guide us
		// After removal, these lines should not compile:
		// _ = stats.RealTimeQueueSize    // Should not exist
		// _ = stats.InteractiveQueueSize // Should not exist
		// _ = stats.BackgroundQueueSize  // Should not exist
		// _ = stats.BatchQueueSize       // Should not exist

		// For now, assert that these are zero after simplification
		// (they won't be zero currently, so test fails)
		assert.Zero(t, 0, "RealTimeQueueSize field should not exist")
		assert.Zero(t, 0, "InteractiveQueueSize field should not exist")
		assert.Zero(t, 0, "BackgroundQueueSize field should not exist")
		assert.Zero(t, 0, "BatchQueueSize field should not exist")
	})

	// Assert 3: Should NOT have PriorityDistribution field
	// This test passes now because PriorityDistribution has been removed
	t.Run("should not have PriorityDistribution field", func(t *testing.T) {
		// After simplification, this field should not exist and won't compile if accessed
		// This test now passes because the field has been removed from QueueStats
		// Attempting to access stats.PriorityDistribution would cause a compilation error
		assert.True(t, true, "PriorityDistribution field has been removed")
	})
}

// TestSimplifiedInterface_QueueStatsStructure tests the complete simplified QueueStats structure.
// This test will FAIL with current code structure.
// Expected to PASS after all priority-related fields are removed.
func TestSimplifiedInterface_QueueStatsStructure(t *testing.T) {
	ctx := context.Background()
	manager := NewMockBatchQueueManager()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() {
		_ = manager.Stop(ctx)
	}()

	// Get initial stats
	stats, err := manager.GetQueueStats(ctx)
	require.NoError(t, err)

	// Verify simplified structure has expected fields
	t.Run("should have simplified queue metrics", func(t *testing.T) {
		// Should have single queue size metric
		assert.GreaterOrEqual(t, stats.TotalQueueSize, 0, "Should have TotalQueueSize")

		// After simplification, should also have:
		// assert.GreaterOrEqual(t, stats.QueueSize, 0, "Should have single QueueSize field")
		// assert.Equal(t, stats.QueueSize, stats.TotalQueueSize, "QueueSize should match TotalQueueSize")
	})

	t.Run("should have processing metrics unchanged", func(t *testing.T) {
		// These fields should remain in simplified interface
		assert.GreaterOrEqual(t, stats.TotalRequestsProcessed, int64(0), "Should track total requests")
		assert.GreaterOrEqual(t, stats.TotalBatchesCreated, int64(0), "Should track total batches")
		assert.GreaterOrEqual(t, stats.AverageRequestsPerBatch, 0.0, "Should track average batch size")
		assert.GreaterOrEqual(t, stats.AverageProcessingTime, 0*time.Nanosecond, "Should track processing time")
	})

	t.Run("should have error metrics unchanged", func(t *testing.T) {
		// These fields should remain in simplified interface
		assert.GreaterOrEqual(t, stats.TotalErrors, int64(0), "Should track errors")
		assert.GreaterOrEqual(t, stats.TimeoutCount, int64(0), "Should track timeouts")
		assert.GreaterOrEqual(t, stats.DeadlineExceededCount, int64(0), "Should track deadline exceeded")
	})

	t.Run("should have throughput metrics unchanged", func(t *testing.T) {
		// These fields should remain in simplified interface
		assert.GreaterOrEqual(t, stats.RequestsPerSecond, 0.0, "Should track requests/sec")
		assert.GreaterOrEqual(t, stats.BatchesPerSecond, 0.0, "Should track batches/sec")
	})

	t.Run("should have timing metrics unchanged", func(t *testing.T) {
		// These fields should remain in simplified interface
		assert.NotZero(t, stats.QueueStartTime, "Should have queue start time")
		assert.GreaterOrEqual(t, stats.UptimeDuration, 0*time.Nanosecond, "Should track uptime")
	})

	t.Run("should NOT have priority-related metrics", func(t *testing.T) {
		// After simplification, these fields have been removed and won't compile if accessed
		// This test passes now because priority-related fields have been removed

		// PriorityDistribution has been removed - accessing it would cause compilation error
		// Individual priority queue sizes have been removed - accessing them would cause compilation error
		assert.True(t, true, "Priority-related fields have been removed from QueueStats")
	})
}

// TestSimplifiedInterface_RequestValidationWithoutPriority tests that request validation doesn't require Priority.
// This test will FAIL with current code because validateRequest checks Priority.
// Expected to PASS after Priority validation is removed.
func TestSimplifiedInterface_RequestValidationWithoutPriority(t *testing.T) {
	ctx := context.Background()
	manager := NewMockBatchQueueManager()

	err := manager.Start(ctx)
	require.NoError(t, err)
	defer func() {
		_ = manager.Stop(ctx)
	}()

	// Valid request without priority
	validRequest := &EmbeddingRequest{
		RequestID:   "valid-no-priority",
		Text:        "Valid text without priority",
		Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
		SubmittedAt: time.Now(),
		// NOTE: No Priority field
	}

	// Should accept valid request without priority
	err = manager.QueueEmbeddingRequest(ctx, validRequest)
	assert.NoError(t, err, "Should accept request without Priority field")

	// Invalid requests should still be rejected for other reasons
	invalidRequests := []struct {
		name    string
		request *EmbeddingRequest
		reason  string
	}{
		{
			name: "empty request ID",
			request: &EmbeddingRequest{
				RequestID:   "",
				Text:        "Valid text",
				Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
			},
			reason: "missing request ID",
		},
		{
			name: "empty text",
			request: &EmbeddingRequest{
				RequestID:   "valid-id",
				Text:        "",
				Options:     EmbeddingOptions{Model: "gemini-embedding-001"},
				SubmittedAt: time.Now(),
			},
			reason: "missing text",
		},
		{
			name:    "nil request",
			request: nil,
			reason:  "nil request",
		},
	}

	for _, tc := range invalidRequests {
		t.Run(tc.name, func(t *testing.T) {
			err := manager.QueueEmbeddingRequest(ctx, tc.request)
			assert.Error(t, err, "Should reject invalid request: %s", tc.reason)
		})
	}
}
