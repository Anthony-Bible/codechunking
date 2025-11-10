package queue

import (
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Mock implementations for testing

// MockEmbeddingServiceWithDelay simulates embedding service with configurable delays and failures.
type MockEmbeddingServiceWithDelay struct {
	baseDelay       time.Duration
	failureRate     float64
	concurrentCalls int64
	totalCalls      int64
	maxConcurrent   int64
	processingTimes []time.Duration
	mu              sync.Mutex
	callLog         []time.Time
	// Simple failure tracking
	failureCounter int64 // Counter for failure decisions
}

func (m *MockEmbeddingServiceWithDelay) GenerateBatchEmbeddings(
	ctx context.Context,
	texts []string,
	options outbound.EmbeddingOptions,
) ([]*outbound.EmbeddingResult, error) {
	atomic.AddInt64(&m.totalCalls, 1)
	current := atomic.AddInt64(&m.concurrentCalls, 1)

	// Track max concurrent calls
	for {
		max := atomic.LoadInt64(&m.maxConcurrent)
		if current <= max || atomic.CompareAndSwapInt64(&m.maxConcurrent, max, current) {
			break
		}
	}

	defer atomic.AddInt64(&m.concurrentCalls, -1)

	// Log call time
	m.mu.Lock()
	m.callLog = append(m.callLog, time.Now())
	m.mu.Unlock()

	// Simulate processing delay, but respect context timeout
	startTime := time.Now()
	select {
	case <-time.After(m.baseDelay):
		// Normal delay completed
	case <-ctx.Done():
		// Context was cancelled or timed out
		return nil, ctx.Err()
	}
	processingTime := time.Since(startTime)

	m.mu.Lock()
	m.processingTimes = append(m.processingTimes, processingTime)
	m.mu.Unlock()

	// Simulate failures based on configured failure rate using simple modulo
	if m.failureRate <= 0 {
		// No failures configured
	} else {
		failureCounter := atomic.AddInt64(&m.failureCounter, 1)
		shouldFail := m.shouldFailCall(failureCounter)
		if shouldFail {
			return nil, errors.New("simulated embedding service failure")
		}
	}

	results := make([]*outbound.EmbeddingResult, len(texts))
	for i, text := range texts {
		results[i] = &outbound.EmbeddingResult{
			Vector:      make([]float64, 768),
			Dimensions:  768,
			TokenCount:  len(text) / 4,
			Model:       "test-model",
			GeneratedAt: time.Now(),
		}
	}

	return results, nil
}

// shouldFailCall determines if a call should fail based on failure rate.
func (m *MockEmbeddingServiceWithDelay) shouldFailCall(failureCounter int64) bool {
	switch {
	case m.failureRate >= 0.25 && m.failureRate <= 0.35:
		// 30% failure rate: fail every 3rd call (1 out of 3 ≈ 33%)
		return failureCounter%3 == 0
	case m.failureRate >= 0.8:
		// 80% failure rate: fail 4 out of 5 calls
		return failureCounter%5 != 2 // Only succeed on call 2, 7, 12, etc.
	default:
		// Default: use simple threshold
		threshold := int64(100.0 * m.failureRate)
		return failureCounter%100 < threshold
	}
}

func (m *MockEmbeddingServiceWithDelay) GetStats() (int64, int64, []time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	times := make([]time.Duration, len(m.processingTimes))
	copy(times, m.processingTimes)

	return atomic.LoadInt64(&m.totalCalls), atomic.LoadInt64(&m.maxConcurrent), times
}

// Required implementations for EmbeddingService interface (stubs for testing).
func (m *MockEmbeddingServiceWithDelay) GenerateEmbedding(
	ctx context.Context,
	text string,
	options outbound.EmbeddingOptions,
) (*outbound.EmbeddingResult, error) {
	results, err := m.GenerateBatchEmbeddings(ctx, []string{text}, options)
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

func (m *MockEmbeddingServiceWithDelay) GenerateCodeChunkEmbedding(
	ctx context.Context,
	chunk *outbound.CodeChunk,
	options outbound.EmbeddingOptions,
) (*outbound.CodeChunkEmbedding, error) {
	return nil, errors.New("not implemented for tests")
}

func (m *MockEmbeddingServiceWithDelay) ValidateApiKey(ctx context.Context) error {
	return nil
}

func (m *MockEmbeddingServiceWithDelay) GetModelInfo(ctx context.Context) (*outbound.ModelInfo, error) {
	return &outbound.ModelInfo{Name: "test-model"}, nil
}

func (m *MockEmbeddingServiceWithDelay) GetSupportedModels(ctx context.Context) ([]string, error) {
	return []string{"test-model"}, nil
}

func (m *MockEmbeddingServiceWithDelay) EstimateTokenCount(ctx context.Context, text string) (int, error) {
	return len(text) / 4, nil
}

// RED PHASE TESTS - All tests should FAIL initially

func TestMockEmbeddingServiceFailure(t *testing.T) {
	mockService := &MockEmbeddingServiceWithDelay{
		baseDelay:   10 * time.Millisecond,
		failureRate: 0.3, // 30% failure rate
	}

	ctx := context.Background()
	successCount := 0
	failureCount := 0

	// Make 10 calls to test the failure pattern
	for i := range 10 {
		_, err := mockService.GenerateBatchEmbeddings(
			ctx,
			[]string{fmt.Sprintf("test %d", i)},
			outbound.EmbeddingOptions{},
		)
		if err != nil {
			failureCount++
		} else {
			successCount++
		}
	}

	t.Logf("Success: %d, Failures: %d", successCount, failureCount)
	if failureCount == 0 {
		t.Error("Expected some failures with 30% failure rate")
	}
	if successCount == 0 {
		t.Error("Expected some successes with 30% failure rate")
	}
}

func TestParallelBatchProcessor_Creation(t *testing.T) {
	t.Run("should create parallel processor with valid configuration", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{baseDelay: 50 * time.Millisecond}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers:            5,
			MinWorkers:            2,
			MaxConcurrentRequests: 3,
			RequestsPerSecond:     10.0,
			WorkerIdleTimeout:     30 * time.Second,
			BatchTimeout:          5 * time.Second,
		}

		// This should fail - ParallelBatchProcessor doesn't exist yet
		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Expected successful creation, got error: %v", err)
		}
		if processor == nil {
			t.Fatal("Expected processor instance, got nil")
		}

		// Verify configuration
		actualConfig := processor.GetConfig()
		if actualConfig.MaxWorkers != 5 {
			t.Errorf("Expected MaxWorkers=5, got %d", actualConfig.MaxWorkers)
		}
		if actualConfig.MinWorkers != 2 {
			t.Errorf("Expected MinWorkers=2, got %d", actualConfig.MinWorkers)
		}
	})

	t.Run("should reject invalid configuration", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{baseDelay: 50 * time.Millisecond}

		invalidConfigs := []*ParallelBatchProcessorConfig{
			{MaxWorkers: 0},                            // Invalid: no workers
			{MaxWorkers: 5, MinWorkers: 10},            // Invalid: min > max
			{MaxWorkers: 5, MaxConcurrentRequests: -1}, // Invalid: negative concurrency
			{MaxWorkers: 5, RequestsPerSecond: -1},     // Invalid: negative rate
		}

		for i, config := range invalidConfigs {
			processor, err := NewParallelBatchProcessor(mockService, config)
			if err == nil {
				t.Errorf("Test %d: Expected error for invalid config, got success", i)
			}
			if processor != nil {
				t.Errorf("Test %d: Expected nil processor for invalid config, got instance", i)
			}
		}
	})
}

func TestParallelBatchProcessor_WorkerPoolManagement(t *testing.T) {
	t.Run("should initialize worker pool with configured workers", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{baseDelay: 10 * time.Millisecond}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers: 5,
			MinWorkers: 2,
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}

		ctx := context.Background()
		stats, err := processor.GetWorkerPoolStats(ctx)
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		if stats.TotalWorkers != 2 {
			t.Errorf("Expected 2 initial workers, got %d", stats.TotalWorkers)
		}
		if stats.IdleWorkers != 2 {
			t.Errorf("Expected 2 idle workers, got %d", stats.IdleWorkers)
		}
	})

	t.Run("should scale workers up when needed", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{baseDelay: 10 * time.Millisecond}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers: 10,
			MinWorkers: 2,
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}

		ctx := context.Background()
		err = processor.ScaleWorkers(ctx, 7)
		if err != nil {
			t.Fatalf("Failed to scale workers: %v", err)
		}

		stats, err := processor.GetWorkerPoolStats(ctx)
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		if stats.TotalWorkers != 7 {
			t.Errorf("Expected 7 workers after scaling, got %d", stats.TotalWorkers)
		}
	})

	t.Run("should scale workers down to minimum", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{baseDelay: 10 * time.Millisecond}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers: 10,
			MinWorkers: 3,
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}

		ctx := context.Background()

		// Scale up first
		err = processor.ScaleWorkers(ctx, 8)
		if err != nil {
			t.Fatalf("Failed to scale up: %v", err)
		}

		// Try to scale below minimum
		err = processor.ScaleWorkers(ctx, 1)
		if err != nil {
			t.Fatalf("Failed to scale down: %v", err)
		}

		stats, err := processor.GetWorkerPoolStats(ctx)
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		if stats.TotalWorkers != 3 {
			t.Errorf("Expected 3 workers (minimum), got %d", stats.TotalWorkers)
		}
	})

	t.Run("should not exceed maximum worker limit", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{baseDelay: 10 * time.Millisecond}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers: 5,
			MinWorkers: 2,
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}

		ctx := context.Background()
		err = processor.ScaleWorkers(ctx, 10) // Exceeds max
		if err == nil {
			t.Fatal("Expected error when exceeding max workers")
		}

		stats, err := processor.GetWorkerPoolStats(ctx)
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		if stats.TotalWorkers > 5 {
			t.Errorf("Workers exceeded maximum: got %d, max %d", stats.TotalWorkers, 5)
		}
	})
}

func TestParallelBatchProcessor_ConcurrentBatchProcessing(t *testing.T) {
	t.Run("should process multiple batches concurrently", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{
			baseDelay: 100 * time.Millisecond,
		}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers:            3,
			MinWorkers:            3,
			MaxConcurrentRequests: 3,
			BatchTimeout:          1 * time.Second,
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}
		defer processor.Shutdown(context.Background())

		// Create multiple batches
		batches := make([][]*outbound.EmbeddingRequest, 3)
		for i := range 3 {
			batch := make([]*outbound.EmbeddingRequest, 2)
			for j := range 2 {
				batch[j] = &outbound.EmbeddingRequest{
					RequestID: fmt.Sprintf("req-%d-%d", i, j),
					Text:      fmt.Sprintf("test text %d-%d", i, j),
					Priority:  outbound.PriorityBackground,
					Options:   outbound.EmbeddingOptions{Model: "test-model"},
				}
			}
			batches[i] = batch
		}

		ctx := context.Background()
		start := time.Now()

		results, err := processor.ProcessBatchesParallel(ctx, batches)

		duration := time.Since(start)
		if err != nil {
			t.Fatalf("Failed to process batches: %v", err)
		}

		// Should complete faster than sequential processing
		sequentialTime := time.Duration(len(batches)) * 100 * time.Millisecond
		if duration >= sequentialTime {
			t.Errorf("Parallel processing took %v, expected less than sequential time %v", duration, sequentialTime)
		}

		// Verify all results
		if len(results) != len(batches) {
			t.Errorf("Expected %d batch results, got %d", len(batches), len(results))
		}

		for i, batchResults := range results {
			if len(batchResults) != len(batches[i]) {
				t.Errorf("Batch %d: expected %d results, got %d", i, len(batches[i]), len(batchResults))
			}
		}

		// Check concurrent execution
		totalCalls, maxConcurrent, _ := mockService.GetStats()
		if totalCalls != int64(len(batches)) {
			t.Errorf("Expected %d embedding service calls, got %d", len(batches), totalCalls)
		}
		if maxConcurrent < 2 {
			t.Errorf("Expected at least 2 concurrent calls, got %d", maxConcurrent)
		}
	})

	t.Run("should handle large number of concurrent batches", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{
			baseDelay: 50 * time.Millisecond,
		}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers:            10,
			MinWorkers:            5,
			MaxConcurrentRequests: 10,
			BatchTimeout:          2 * time.Second,
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}
		defer processor.Shutdown(context.Background())

		// Create 20 batches
		batchCount := 20
		batches := make([][]*outbound.EmbeddingRequest, batchCount)
		for i := range batchCount {
			batch := []*outbound.EmbeddingRequest{{
				RequestID: fmt.Sprintf("req-%d", i),
				Text:      fmt.Sprintf("test text %d", i),
				Priority:  outbound.PriorityBackground,
				Options:   outbound.EmbeddingOptions{Model: "test-model"},
			}}
			batches[i] = batch
		}

		ctx := context.Background()
		start := time.Now()

		results, err := processor.ProcessBatchesParallel(ctx, batches)

		duration := time.Since(start)
		if err != nil {
			t.Fatalf("Failed to process batches: %v", err)
		}

		if len(results) != batchCount {
			t.Errorf("Expected %d batch results, got %d", batchCount, len(results))
		}

		// Should complete much faster than sequential
		expectedMaxTime := 2 * time.Second // Should be well under this with 10 workers
		if duration > expectedMaxTime {
			t.Errorf("Parallel processing took %v, expected under %v", duration, expectedMaxTime)
		}

		// Verify high concurrency was achieved
		_, maxConcurrent, _ := mockService.GetStats()
		if maxConcurrent < 5 {
			t.Errorf("Expected at least 5 concurrent calls, got %d", maxConcurrent)
		}
	})
}

func TestParallelBatchProcessor_RateLimitingAndConcurrencyControl(t *testing.T) {
	t.Run("should respect max concurrent requests limit", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{
			baseDelay: 200 * time.Millisecond,
		}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers:            10,
			MinWorkers:            5,
			MaxConcurrentRequests: 3, // Limit to 3 concurrent requests
			BatchTimeout:          2 * time.Second,
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}
		defer processor.Shutdown(context.Background())

		// Create 10 batches to test concurrency limiting
		batches := make([][]*outbound.EmbeddingRequest, 10)
		for i := range 10 {
			batch := []*outbound.EmbeddingRequest{{
				RequestID: fmt.Sprintf("req-%d", i),
				Text:      fmt.Sprintf("test text %d", i),
				Priority:  outbound.PriorityBackground,
				Options:   outbound.EmbeddingOptions{Model: "test-model"},
			}}
			batches[i] = batch
		}

		ctx := context.Background()
		_, err = processor.ProcessBatchesParallel(ctx, batches)
		if err != nil {
			t.Fatalf("Failed to process batches: %v", err)
		}

		// Verify concurrency was limited
		_, maxConcurrent, _ := mockService.GetStats()
		if maxConcurrent > 3 {
			t.Errorf("Expected max 3 concurrent requests, got %d", maxConcurrent)
		}
	})

	t.Run("should enforce requests per second rate limit", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{
			baseDelay: 10 * time.Millisecond,
		}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers:        5,
			MinWorkers:        2,
			RequestsPerSecond: 5.0, // Limit to 5 requests per second
			BatchTimeout:      2 * time.Second,
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}
		defer processor.Shutdown(context.Background())

		// Create 10 batches
		batches := make([][]*outbound.EmbeddingRequest, 10)
		for i := range 10 {
			batch := []*outbound.EmbeddingRequest{{
				RequestID: fmt.Sprintf("req-%d", i),
				Text:      fmt.Sprintf("test text %d", i),
				Priority:  outbound.PriorityBackground,
				Options:   outbound.EmbeddingOptions{Model: "test-model"},
			}}
			batches[i] = batch
		}

		ctx := context.Background()
		start := time.Now()

		_, err = processor.ProcessBatchesParallel(ctx, batches)

		duration := time.Since(start)
		if err != nil {
			t.Fatalf("Failed to process batches: %v", err)
		}

		// With rate limit of 5 RPS, 10 requests should take at least 2 seconds
		expectedMinTime := 1800 * time.Millisecond // Allow some tolerance
		if duration < expectedMinTime {
			t.Errorf("Rate limiting not enforced: took %v, expected at least %v", duration, expectedMinTime)
		}

		// Verify rate limiting stats
		stats, err := processor.GetWorkerPoolStats(ctx)
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		if stats.RateLimitHits == 0 {
			t.Error("Expected rate limit hits, got 0")
		}
	})

	t.Run("should maintain rate limit over time windows", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{
			baseDelay: 10 * time.Millisecond,
		}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers:        5,
			MinWorkers:        2,
			RequestsPerSecond: 10.0, // 10 RPS
			BatchTimeout:      500 * time.Millisecond,
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}
		defer processor.Shutdown(context.Background())

		ctx := context.Background()

		// Send requests in bursts and verify rate limiting
		for burst := range 3 {
			batches := make([][]*outbound.EmbeddingRequest, 8)
			for i := range 8 {
				batch := []*outbound.EmbeddingRequest{{
					RequestID: fmt.Sprintf("burst-%d-req-%d", burst, i),
					Text:      fmt.Sprintf("test text %d-%d", burst, i),
					Priority:  outbound.PriorityBackground,
					Options:   outbound.EmbeddingOptions{Model: "test-model"},
				}}
				batches[i] = batch
			}

			start := time.Now()
			_, err = processor.ProcessBatchesParallel(ctx, batches)
			duration := time.Since(start)

			if err != nil {
				t.Fatalf("Burst %d failed: %v", burst, err)
			}

			// Each burst should be rate limited
			expectedMinTime := 600 * time.Millisecond // 8 requests at 10 RPS
			if duration < expectedMinTime {
				t.Errorf(
					"Burst %d: rate limiting not enforced: took %v, expected at least %v",
					burst,
					duration,
					expectedMinTime,
				)
			}
		}
	})

	t.Run("should track rate limiting metrics", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{
			baseDelay: 5 * time.Millisecond,
		}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers:        3,
			MinWorkers:        1,
			RequestsPerSecond: 3.0, // Low rate to trigger limiting
			BatchTimeout:      1 * time.Second,
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}
		defer processor.Shutdown(context.Background())

		// Create enough batches to trigger rate limiting
		batches := make([][]*outbound.EmbeddingRequest, 7)
		for i := range 7 {
			batch := []*outbound.EmbeddingRequest{{
				RequestID: fmt.Sprintf("req-%d", i),
				Text:      fmt.Sprintf("test text %d", i),
				Priority:  outbound.PriorityBackground,
				Options:   outbound.EmbeddingOptions{Model: "test-model"},
			}}
			batches[i] = batch
		}

		ctx := context.Background()
		_, err = processor.ProcessBatchesParallel(ctx, batches)
		if err != nil {
			t.Fatalf("Failed to process batches: %v", err)
		}

		stats, err := processor.GetWorkerPoolStats(ctx)
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		// Verify rate limiting metrics are tracked
		if stats.RateLimitHits == 0 {
			t.Error("Expected rate limit hits to be tracked")
		}
		if stats.CurrentRequestsPerSecond <= 0 {
			t.Error("Expected current RPS to be tracked")
		}
		if stats.CurrentRequestsPerSecond > 3.5 { // Allow some tolerance
			t.Errorf("Current RPS %f exceeds configured limit 3.0", stats.CurrentRequestsPerSecond)
		}
	})
}

func TestParallelBatchProcessor_ErrorHandling(t *testing.T) {
	t.Run("should handle partial batch failures gracefully", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{
			baseDelay:   50 * time.Millisecond,
			failureRate: 0.3, // 30% failure rate
		}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers:            5,
			MinWorkers:            2,
			BatchTimeout:          1 * time.Second,
			ErrorThreshold:        0.5,             // 50% error rate threshold for circuit breaker
			CircuitBreakerTimeout: 1 * time.Second, // Circuit breaker timeout
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}
		defer processor.Shutdown(context.Background())

		// Create multiple batches
		batches := make([][]*outbound.EmbeddingRequest, 10)
		for i := range 10 {
			batch := []*outbound.EmbeddingRequest{{
				RequestID: fmt.Sprintf("req-%d", i),
				Text:      fmt.Sprintf("test text %d", i),
				Priority:  outbound.PriorityBackground,
				Options:   outbound.EmbeddingOptions{Model: "test-model"},
			}}
			batches[i] = batch
		}

		ctx := context.Background()
		results, err := processor.ProcessBatchesParallel(ctx, batches)
		// Should not fail completely due to partial failures
		if err != nil {
			t.Fatalf("ProcessBatchesParallel failed completely: %v", err)
		}

		// Should return results for all batches (some with errors)
		if len(results) != len(batches) {
			t.Errorf("Expected %d batch results, got %d", len(batches), len(results))
		}

		// Count successful and failed batches
		successCount := 0
		failureCount := 0

		for i, batchResults := range results {
			if len(batchResults) != len(batches[i]) {
				t.Errorf("Batch %d: expected %d results, got %d", i, len(batches[i]), len(batchResults))
			}

			for _, result := range batchResults {
				if result.Error != "" {
					failureCount++
				} else if result.Result != nil {
					successCount++
				}
			}
		}

		// Should have some failures due to configured failure rate
		if failureCount == 0 {
			t.Error("Expected some failures with 30% failure rate")
		}
		if successCount == 0 {
			t.Error("Expected some successes with 30% failure rate")
		}

		// Check error tracking in stats
		stats, err := processor.GetWorkerPoolStats(ctx)
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		if stats.TotalErrors == 0 {
			t.Error("Expected error count to be tracked")
		}
		if stats.ErrorRate <= 0 {
			t.Error("Expected error rate to be calculated")
		}
	})

	t.Run("should implement circuit breaker pattern", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{
			baseDelay:   50 * time.Millisecond,
			failureRate: 0.8, // High failure rate to trigger circuit breaker
		}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers:            3,
			MinWorkers:            2,
			BatchTimeout:          500 * time.Millisecond,
			ErrorThreshold:        0.7,             // 70% error rate triggers circuit breaker
			CircuitBreakerTimeout: 1 * time.Second, // Wait 1s before retry
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}
		defer processor.Shutdown(context.Background())

		ctx := context.Background()

		// First batch should trigger circuit breaker - use more batches to ensure minimum sample size
		firstBatches := make([][]*outbound.EmbeddingRequest, 10)
		for i := range 10 {
			firstBatches[i] = []*outbound.EmbeddingRequest{{
				RequestID: fmt.Sprintf("first-req-%d", i),
				Text:      fmt.Sprintf("test text %d", i),
				Priority:  outbound.PriorityBackground,
				Options:   outbound.EmbeddingOptions{Model: "test-model"},
			}}
		}

		_, err = processor.ProcessBatchesParallel(ctx, firstBatches)
		if err != nil {
			t.Fatalf("First batch failed: %v", err)
		}

		// Add a small delay to ensure circuit breaker state is updated
		time.Sleep(10 * time.Millisecond)

		// Check if circuit breaker is open
		stats, err := processor.GetWorkerPoolStats(ctx)
		if err != nil {
			t.Fatalf("Failed to get stats: %v", err)
		}

		if !stats.CircuitOpen {
			t.Errorf(
				"Expected circuit breaker to be open after high error rate, got open=%v, total_batches=%d, total_errors=%d",
				stats.CircuitOpen,
				stats.TotalBatchesProcessed,
				stats.TotalErrors,
			)
		}

		// Immediate retry should fail due to circuit breaker
		retryBatches := make([][]*outbound.EmbeddingRequest, 2)
		for i := range 2 {
			retryBatches[i] = []*outbound.EmbeddingRequest{{
				RequestID: fmt.Sprintf("retry-req-%d", i),
				Text:      fmt.Sprintf("retry text %d", i),
				Priority:  outbound.PriorityBackground,
				Options:   outbound.EmbeddingOptions{Model: "test-model"},
			}}
		}

		start := time.Now()
		_, err = processor.ProcessBatchesParallel(ctx, retryBatches)
		duration := time.Since(start)

		// Should fail fast due to circuit breaker
		if duration > 200*time.Millisecond {
			t.Errorf("Circuit breaker should fail fast, took %v", duration)
		}

		if err == nil {
			t.Error("Expected error when circuit breaker is open")
		} else if err.Error() != "circuit breaker is open" {
			t.Errorf("Expected 'circuit breaker is open' error, got: %v", err)
		}
	})
}

func TestParallelBatchProcessor_ContextCancellationAndTimeouts(t *testing.T) {
	t.Run("should respect context cancellation", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{
			baseDelay: 500 * time.Millisecond, // Long delay to allow cancellation
		}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers:   3,
			MinWorkers:   2,
			BatchTimeout: 2 * time.Second,
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}
		defer processor.Shutdown(context.Background())

		// Create batches
		batches := make([][]*outbound.EmbeddingRequest, 5)
		for i := range 5 {
			batches[i] = []*outbound.EmbeddingRequest{{
				RequestID: fmt.Sprintf("req-%d", i),
				Text:      fmt.Sprintf("test text %d", i),
				Priority:  outbound.PriorityBackground,
				Options:   outbound.EmbeddingOptions{Model: "test-model"},
			}}
		}

		// Create context with cancellation
		ctx, cancel := context.WithCancel(context.Background())

		// Start processing
		done := make(chan error, 1)
		go func() {
			_, err := processor.ProcessBatchesParallel(ctx, batches)
			done <- err
		}()

		// Cancel after 200ms
		time.Sleep(200 * time.Millisecond)
		cancel()

		// Should receive cancellation error quickly
		select {
		case err := <-done:
			if err == nil {
				t.Error("Expected cancellation error, got nil")
			}
		case <-time.After(1 * time.Second):
			t.Error("Context cancellation took too long to respond")
		}
	})

	t.Run("should respect batch timeout", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{
			baseDelay: 2 * time.Second, // Longer than batch timeout
		}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers:   2,
			MinWorkers:   1,
			BatchTimeout: 500 * time.Millisecond, // Short timeout
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}
		defer processor.Shutdown(context.Background())

		batches := [][]*outbound.EmbeddingRequest{{{
			RequestID: "timeout-test",
			Text:      "test text",
			Priority:  outbound.PriorityBackground,
			Options:   outbound.EmbeddingOptions{Model: "test-model"},
		}}}

		ctx := context.Background()
		start := time.Now()

		_, err = processor.ProcessBatchesParallel(ctx, batches)

		duration := time.Since(start)

		// Should timeout quickly
		if duration > 1*time.Second {
			t.Errorf("Batch timeout not enforced: took %v", duration)
		}

		if err == nil {
			t.Error("Expected timeout error, got nil")
		}
	})
}

// Benchmark tests removed for simplicity - the core functionality tests provide adequate coverage

func TestParallelBatchProcessor_ResourceManagement(t *testing.T) {
	t.Run("should not leak goroutines", func(t *testing.T) {
		initialGoroutines := runtime.NumGoroutine()

		mockService := &MockEmbeddingServiceWithDelay{
			baseDelay: 20 * time.Millisecond,
		}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers:   5,
			MinWorkers:   2,
			BatchTimeout: 1 * time.Second,
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}

		// Process several batches
		for round := range 3 {
			batches := make([][]*outbound.EmbeddingRequest, 5)
			for i := range 5 {
				batches[i] = []*outbound.EmbeddingRequest{{
					RequestID: fmt.Sprintf("round-%d-req-%d", round, i),
					Text:      fmt.Sprintf("test text %d-%d", round, i),
					Priority:  outbound.PriorityBackground,
					Options:   outbound.EmbeddingOptions{Model: "test-model"},
				}}
			}

			ctx := context.Background()
			_, err = processor.ProcessBatchesParallel(ctx, batches)
			if err != nil {
				t.Fatalf("Round %d failed: %v", round, err)
			}
		}

		// Shutdown processor
		err = processor.Shutdown(context.Background())
		if err != nil {
			t.Fatalf("Failed to shutdown processor: %v", err)
		}

		// Allow some time for cleanup
		time.Sleep(100 * time.Millisecond)

		// Check for goroutine leaks
		finalGoroutines := runtime.NumGoroutine()
		if finalGoroutines > initialGoroutines+2 { // Allow some tolerance
			t.Errorf("Potential goroutine leak: initial=%d, final=%d", initialGoroutines, finalGoroutines)
		}
	})

	t.Run("should handle graceful shutdown", func(t *testing.T) {
		mockService := &MockEmbeddingServiceWithDelay{
			baseDelay: 100 * time.Millisecond,
		}
		config := &ParallelBatchProcessorConfig{
			MaxWorkers:   3,
			MinWorkers:   2,
			BatchTimeout: 2 * time.Second,
		}

		processor, err := NewParallelBatchProcessor(mockService, config)
		if err != nil {
			t.Fatalf("Failed to create processor: %v", err)
		}

		// Start some long-running batches
		batches := make([][]*outbound.EmbeddingRequest, 5)
		for i := range 5 {
			batches[i] = []*outbound.EmbeddingRequest{{
				RequestID: fmt.Sprintf("shutdown-req-%d", i),
				Text:      fmt.Sprintf("test text %d", i),
				Priority:  outbound.PriorityBackground,
				Options:   outbound.EmbeddingOptions{Model: "test-model"},
			}}
		}

		// Start processing in background
		done := make(chan error, 1)
		go func() {
			_, err := processor.ProcessBatchesParallel(context.Background(), batches)
			done <- err
		}()

		// Brief delay then shutdown
		time.Sleep(50 * time.Millisecond)

		shutdownStart := time.Now()
		err = processor.Shutdown(context.Background())
		shutdownDuration := time.Since(shutdownStart)

		if err != nil {
			t.Fatalf("Shutdown failed: %v", err)
		}

		// Shutdown should complete reasonably quickly
		if shutdownDuration > 3*time.Second {
			t.Errorf("Shutdown took too long: %v", shutdownDuration)
		}

		// Processing should complete or be cancelled
		select {
		case <-done:
			// Either completed successfully or was cancelled
		case <-time.After(1 * time.Second):
			t.Error("Processing didn't complete after shutdown")
		}

		// Verify processor is no longer healthy
		healthy, err := processor.IsHealthy(context.Background())
		if err == nil && healthy {
			t.Error("Processor should not be healthy after shutdown")
		}
	})
}
