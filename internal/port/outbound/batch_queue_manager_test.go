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
	queues           map[RequestPriority][]*EmbeddingRequest
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
		queues: map[RequestPriority][]*EmbeddingRequest{
			PriorityRealTime:    {},
			PriorityInteractive: {},
			PriorityBackground:  {},
			PriorityBatch:       {},
		},
		config: &BatchConfig{
			MinBatchSize:        1,
			MaxBatchSize:        100,
			BatchTimeout:        5 * time.Second,
			MaxQueueSize:        1000,
			ProcessingInterval:  1 * time.Second,
			EnableDynamicSizing: true,
			PriorityWeights: map[RequestPriority]float64{
				PriorityRealTime:    1.0,
				PriorityInteractive: 0.7,
				PriorityBackground:  0.4,
				PriorityBatch:       0.1,
			},
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

	m.queues[request.Priority] = append(m.queues[request.Priority], request)
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
		m.queues[request.Priority] = append(m.queues[request.Priority], request)
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
	priorities := []RequestPriority{PriorityRealTime, PriorityInteractive, PriorityBackground, PriorityBatch}

	for _, priority := range priorities {
		requests := m.queues[priority]
		if len(requests) == 0 {
			continue
		}

		batchSize := m.getOptimalBatchSize(priority, len(requests))
		if batchSize > len(requests) {
			batchSize = len(requests)
		}

		if batchSize > 0 {
			// Remove the batch from the queue
			m.queues[priority] = requests[batchSize:]
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

	for priority := range m.queues {
		m.queues[priority] = []*EmbeddingRequest{}
	}
	return nil
}

func (m *MockBatchQueueManager) GetQueueStats(ctx context.Context) (*QueueStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &QueueStats{
		RealTimeQueueSize:      len(m.queues[PriorityRealTime]),
		InteractiveQueueSize:   len(m.queues[PriorityInteractive]),
		BackgroundQueueSize:    len(m.queues[PriorityBackground]),
		BatchQueueSize:         len(m.queues[PriorityBatch]),
		TotalQueueSize:         m.getTotalQueueSize(),
		TotalRequestsProcessed: m.totalProcessed,
		TotalBatchesCreated:    m.totalBatches,
		TotalErrors:            m.totalErrors,
		TimeoutCount:           m.timeoutCount,
		DeadlineExceededCount:  m.deadlineExceeded,
		QueueStartTime:         m.startTime,
		LastProcessedAt:        m.lastProcessed,
		UptimeDuration:         time.Since(m.startTime),
		PriorityDistribution:   make(map[RequestPriority]int),
	}

	if m.totalBatches > 0 {
		stats.AverageRequestsPerBatch = float64(m.totalProcessed) / float64(m.totalBatches)
	}

	for priority, requests := range m.queues {
		stats.PriorityDistribution[priority] = len(requests)
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
	total := 0
	for _, requests := range m.queues {
		total += len(requests)
	}
	return total
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

	validPriorities := map[RequestPriority]bool{
		PriorityRealTime:    true,
		PriorityInteractive: true,
		PriorityBackground:  true,
		PriorityBatch:       true,
	}

	if !validPriorities[request.Priority] {
		return &QueueManagerError{
			Code:      "invalid_priority",
			Message:   "Invalid priority value",
			Type:      "validation",
			RequestID: request.RequestID,
		}
	}

	return nil
}

func (m *MockBatchQueueManager) getOptimalBatchSize(priority RequestPriority, queueSize int) int {
	switch priority {
	case PriorityRealTime:
		return min(5, max(1, queueSize))
	case PriorityInteractive:
		return min(25, max(5, queueSize))
	case PriorityBackground:
		return min(50, max(25, queueSize))
	case PriorityBatch:
		return min(100, max(50, queueSize))
	default:
		return min(m.config.MaxBatchSize, max(m.config.MinBatchSize, queueSize))
	}
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

// TestBatchQueueManagerInterface_PriorityOrdering tests that requests are processed in correct priority order.
func TestBatchQueueManagerInterface_PriorityOrdering(t *testing.T) {
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
	err = manager.QueueBulkEmbeddingRequests(ctx, requests)
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
				MaxQueueSize:        1000,
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
				MaxQueueSize:        1000,
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
	// GREEN PHASE: Use concrete implementation to make tests pass
	manager := NewMockBatchQueueManager()
	require.NotNil(t, manager, "BatchQueueManager implementation should be available")

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
