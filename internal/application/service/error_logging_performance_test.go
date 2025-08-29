package service

import (
	"codechunking/internal/application/common/logging"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestErrorLoggingService_NonBlockingProcessing tests that error processing doesn't block the main thread.
func TestErrorLoggingService_NonBlockingProcessing(t *testing.T) {
	t.Run("should process errors asynchronously without blocking caller", func(t *testing.T) {
		mockLogger := new(MockApplicationLogger)
		mockLogger.On("Error", mock.AnythingOfType("*context.valueCtx"), mock.AnythingOfType("string"),
			mock.AnythingOfType("slogger.Fields")).Return()

		// Create async error logging service
		asyncService := NewAsyncErrorLoggingService(mockLogger, 100) // buffer size 100

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		start := time.Now()

		// Process multiple errors quickly - should not block
		for i := range 50 {
			err := asyncService.LogAndClassifyError(ctx, fmt.Errorf("error %d", i), "test-component", severity)
			assert.NoError(t, err)
		}

		elapsed := time.Since(start)

		// Should complete very quickly since it's non-blocking
		assert.Less(t, elapsed, time.Millisecond*100, "Processing took too long: %v", elapsed)

		// Flush and wait for async processing to complete
		asyncService.Flush()
		asyncService.Shutdown(context.Background())
	})

	t.Run("should handle buffer overflow gracefully", func(t *testing.T) {
		mockLogger := new(MockApplicationLogger)
		mockLogger.On("Error", mock.AnythingOfType("*context.valueCtx"), mock.AnythingOfType("string"),
			mock.AnythingOfType("slogger.Fields")).Return().Maybe()
		mockLogger.On("Warn", mock.AnythingOfType("*context.valueCtx"), mock.AnythingOfType("string"),
			mock.MatchedBy(func(fields slogger.Fields) bool {
				return fields["dropped_errors"] != nil
			})).Return()

		// Create service with small buffer to force overflow
		asyncService := NewAsyncErrorLoggingService(mockLogger, 5) // small buffer

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		// Process more errors than buffer can handle
		droppedCount := 0
		for i := range 20 {
			err := asyncService.LogAndClassifyError(ctx, fmt.Errorf("error %d", i), "test-component", severity)
			if err != nil && err.Error() == "error buffer full, dropping error" {
				droppedCount++
			}
		}

		// Should have dropped some errors
		assert.Positive(t, droppedCount, "Expected some errors to be dropped due to buffer overflow")

		asyncService.Shutdown(context.Background())
		mockLogger.AssertExpectations(t)
	})

	t.Run("should maintain performance under high load", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping performance test in short mode")
		}

		mockLogger := new(MockApplicationLogger)
		// Use Maybe() since we can't predict exact call count due to async nature
		mockLogger.On("Error", mock.AnythingOfType("*context.valueCtx"), mock.AnythingOfType("string"),
			mock.AnythingOfType("slogger.Fields")).Return().Maybe()

		asyncService := NewAsyncErrorLoggingService(mockLogger, 1000)

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		// Concurrent error processing test
		var wg sync.WaitGroup
		numGoroutines := 10
		errorsPerGoroutine := 100

		start := time.Now()

		for i := range numGoroutines {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for j := range errorsPerGoroutine {
					err := asyncService.LogAndClassifyError(
						ctx,
						fmt.Errorf("goroutine %d error %d", goroutineID, j),
						"load-test-component",
						severity,
					)
					// In high load, some errors might be dropped - that's acceptable
					_ = err
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(start)

		// Should handle 1000 errors across 10 goroutines in reasonable time
		assert.Less(t, elapsed, time.Second*5, "High load processing took too long: %v", elapsed)

		asyncService.Shutdown(context.Background())
	})
}

// TestErrorBuffer tests memory-efficient error buffering.
func TestErrorBuffer_MemoryEfficiency(t *testing.T) {
	t.Run("should implement memory-efficient circular buffer", func(t *testing.T) {
		bufferSize := 1000
		buffer := NewErrorBuffer(bufferSize)

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		// Fill buffer to capacity
		for i := range bufferSize {
			classifiedError, _ := entity.NewClassifiedError(
				ctx, fmt.Errorf("error %d", i), severity, "test_error", fmt.Sprintf("Error %d", i), nil,
			)

			added := buffer.Add(classifiedError)
			assert.True(t, added, "Should be able to add error %d", i)
		}

		assert.Equal(t, bufferSize, buffer.Size())
		assert.True(t, buffer.IsFull())

		// Add one more - should overwrite oldest
		extraError, _ := entity.NewClassifiedError(
			ctx, errors.New("extra error"), severity, "test_error", "Extra error", nil,
		)

		added := buffer.Add(extraError)
		assert.True(t, added, "Should overwrite oldest error")
		assert.Equal(t, bufferSize, buffer.Size()) // Size should remain same

		// Verify oldest was overwritten by checking first error
		errors := buffer.GetAll()
		assert.Equal(t, "Error 1", errors[0].Message()) // First error should now be Error 1, not Error 0
	})

	t.Run("should provide efficient batch retrieval", func(t *testing.T) {
		buffer := NewErrorBuffer(100)
		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("WARNING")

		// Add some errors
		for i := range 50 {
			classifiedError, _ := entity.NewClassifiedError(
				ctx, fmt.Errorf("error %d", i), severity, "batch_test", fmt.Sprintf("Error %d", i), nil,
			)
			buffer.Add(classifiedError)
		}

		// Batch retrieval should be efficient
		start := time.Now()
		batch := buffer.GetBatch(20) // Get 20 errors
		elapsed := time.Since(start)

		assert.Len(t, batch, 20)
		assert.Less(t, elapsed, time.Millisecond, "Batch retrieval should be very fast")

		// Buffer should have 30 remaining
		assert.Equal(t, 30, buffer.Size())
	})

	t.Run("should support memory usage monitoring", func(t *testing.T) {
		buffer := NewErrorBuffer(500)

		initialMemory := buffer.GetMemoryUsage()
		assert.Positive(t, initialMemory, "Should report initial memory usage")

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		// Add errors and monitor memory growth
		for i := range 100 {
			classifiedError, _ := entity.NewClassifiedError(
				ctx,
				fmt.Errorf("large error with context %d", i),
				severity,
				"memory_test",
				fmt.Sprintf("Large error message with lots of context %d", i),
				map[string]interface{}{
					"large_field_1": fmt.Sprintf("large data %d", i),
					"large_field_2": make([]string, 10), // Some memory overhead
					"timestamp":     time.Now(),
				},
			)
			buffer.Add(classifiedError)
		}

		memoryAfter := buffer.GetMemoryUsage()
		assert.Greater(t, memoryAfter, initialMemory, "Memory usage should increase after adding errors")

		// Clear buffer and verify memory is released
		buffer.Clear()
		memoryAfterClear := buffer.GetMemoryUsage()
		assert.Less(t, memoryAfterClear, memoryAfter, "Memory should be released after clearing")
	})
}

// TestCircuitBreaker tests circuit breaker for alert delivery failures.
func TestCircuitBreaker_AlertDeliveryReliability(t *testing.T) {
	t.Run("should open circuit breaker after consecutive failures", func(t *testing.T) {
		failureThreshold := 5
		recoveryTimeout := time.Millisecond * 100

		circuitBreaker := NewAlertDeliveryCircuitBreaker(failureThreshold, recoveryTimeout)
		assert.False(t, circuitBreaker.IsOpen())

		// Simulate consecutive failures
		for i := range failureThreshold - 1 {
			err := circuitBreaker.Execute(func() error {
				return fmt.Errorf("delivery failed %d", i)
			})
			assert.Error(t, err)
			assert.False(t, circuitBreaker.IsOpen(), "Circuit should not be open yet at failure %d", i)
		}

		// Final failure should open circuit
		err := circuitBreaker.Execute(func() error {
			return errors.New("final failure")
		})
		assert.Error(t, err)
		assert.True(t, circuitBreaker.IsOpen(), "Circuit should be open after threshold failures")
	})

	t.Run("should reject calls when circuit is open", func(t *testing.T) {
		circuitBreaker := NewAlertDeliveryCircuitBreaker(3, time.Minute)

		// Force circuit to open
		for range 3 {
			circuitBreaker.Execute(func() error { return errors.New("failure") })
		}

		assert.True(t, circuitBreaker.IsOpen())

		// Should reject subsequent calls without executing function
		functionCalled := false
		err := circuitBreaker.Execute(func() error {
			functionCalled = true
			return nil
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circuit breaker is open")
		assert.False(t, functionCalled, "Function should not be called when circuit is open")
	})

	t.Run("should recover to half-open state after timeout", func(t *testing.T) {
		failureThreshold := 3
		recoveryTimeout := time.Millisecond * 50 // Short timeout for testing

		circuitBreaker := NewAlertDeliveryCircuitBreaker(failureThreshold, recoveryTimeout)

		// Open the circuit
		for range failureThreshold {
			circuitBreaker.Execute(func() error { return errors.New("failure") })
		}
		assert.True(t, circuitBreaker.IsOpen())

		// Wait for recovery timeout
		time.Sleep(recoveryTimeout + time.Millisecond*10)

		// Should be in half-open state - allows one test call
		functionCalled := false
		err := circuitBreaker.Execute(func() error {
			functionCalled = true
			return nil // Successful call
		})

		assert.NoError(t, err)
		assert.True(t, functionCalled)
		assert.False(t, circuitBreaker.IsOpen(), "Circuit should close after successful call")
	})

	t.Run("should maintain failure statistics", func(t *testing.T) {
		circuitBreaker := NewAlertDeliveryCircuitBreaker(10, time.Minute)

		// Execute mix of successful and failed calls
		for range 7 {
			circuitBreaker.Execute(func() error { return nil }) // Success
		}
		for range 3 {
			circuitBreaker.Execute(func() error { return errors.New("failure") }) // Failure
		}

		stats := circuitBreaker.GetStatistics()
		assert.Equal(t, int64(10), stats.TotalCalls)
		assert.Equal(t, int64(7), stats.SuccessfulCalls)
		assert.Equal(t, int64(3), stats.FailedCalls)
		assert.Equal(t, 0.7, stats.SuccessRate)
		assert.Equal(t, 0.3, stats.FailureRate)
	})
}

// TestAlertDeduplication tests alert deduplication to prevent spam.
func TestAlertDeduplication_SpamPrevention(t *testing.T) {
	t.Run("should deduplicate identical alerts within time window", func(t *testing.T) {
		deduplicationWindow := time.Minute * 5
		deduplicator := NewAlertDeduplicator(deduplicationWindow)

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		// Create identical alerts
		alert1 := createTestAlert(ctx, severity, "database_failure", "Database connection failed")
		alert2 := createTestAlert(ctx, severity, "database_failure", "Database connection failed")

		// First alert should not be duplicate
		isDuplicate, err := deduplicator.IsDuplicate(alert1)
		assert.NoError(t, err)
		assert.False(t, isDuplicate)

		// Record first alert
		err = deduplicator.RecordAlert(alert1)
		assert.NoError(t, err)

		// Second identical alert should be detected as duplicate
		isDuplicate, err = deduplicator.IsDuplicate(alert2)
		assert.NoError(t, err)
		assert.True(t, isDuplicate, "Identical alert should be detected as duplicate")
	})

	t.Run("should allow alerts after deduplication window expires", func(t *testing.T) {
		shortWindow := time.Millisecond * 50
		deduplicator := NewAlertDeduplicator(shortWindow)

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("WARNING")

		alert1 := createTestAlert(ctx, severity, "performance_issue", "Performance degraded")
		alert2 := createTestAlert(ctx, severity, "performance_issue", "Performance degraded")

		// Record first alert
		deduplicator.RecordAlert(alert1)

		// Wait for deduplication window to expire
		time.Sleep(shortWindow + time.Millisecond*10)

		// Second alert should not be duplicate after window expiry
		isDuplicate, err := deduplicator.IsDuplicate(alert2)
		assert.NoError(t, err)
		assert.False(t, isDuplicate, "Alert should not be duplicate after window expires")
	})

	t.Run("should track duplicate count and suppress excessive alerts", func(t *testing.T) {
		deduplicator := NewAlertDeduplicator(time.Minute * 10)

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		baseAlert := createTestAlert(ctx, severity, "recurring_error", "This error keeps happening")
		deduplicator.RecordAlert(baseAlert)

		// Create multiple duplicate alerts
		suppressionThreshold := 5
		for range suppressionThreshold + 2 {
			duplicateAlert := createTestAlert(ctx, severity, "recurring_error", "This error keeps happening")

			isDuplicate, err := deduplicator.IsDuplicate(duplicateAlert)
			assert.NoError(t, err)
			assert.True(t, isDuplicate)

			// Track the duplicate
			deduplicator.TrackDuplicate(duplicateAlert)
		}

		duplicateCount := deduplicator.GetDuplicateCount(baseAlert.DeduplicationKey())
		assert.Equal(t, suppressionThreshold+2, duplicateCount)

		// Should suppress after threshold
		shouldSuppress := deduplicator.ShouldSuppress(baseAlert.DeduplicationKey(), suppressionThreshold)
		assert.True(t, shouldSuppress, "Should suppress after exceeding threshold")
	})

	t.Run("should handle different alert patterns separately", func(t *testing.T) {
		deduplicator := NewAlertDeduplicator(time.Minute * 5)

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		databaseAlert := createTestAlert(ctx, severity, "database_failure", "DB failed")
		apiAlert := createTestAlert(ctx, severity, "api_failure", "API failed")

		// Record database alert
		deduplicator.RecordAlert(databaseAlert)

		// API alert should not be duplicate (different pattern)
		isDuplicate, err := deduplicator.IsDuplicate(apiAlert)
		assert.NoError(t, err)
		assert.False(t, isDuplicate, "Different error types should not be considered duplicates")

		// But another database alert should be duplicate
		anotherDBAlert := createTestAlert(ctx, severity, "database_failure", "DB failed again")
		isDuplicate, err = deduplicator.IsDuplicate(anotherDBAlert)
		assert.NoError(t, err)
		assert.True(t, isDuplicate, "Same error type should be considered duplicate")
	})
}

// TestErrorLoggingService_GracefulShutdown tests graceful shutdown handling.
func TestErrorLoggingService_GracefulShutdown(t *testing.T) {
	t.Run("should process remaining errors before shutdown", func(t *testing.T) {
		mockLogger := new(MockApplicationLogger)

		// Expect all errors to be processed
		mockLogger.On("Error", mock.AnythingOfType("*context.valueCtx"), mock.AnythingOfType("string"),
			mock.AnythingOfType("slogger.Fields")).Return().Times(10)

		asyncService := NewAsyncErrorLoggingService(mockLogger, 20)

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		// Queue up errors
		for i := range 10 {
			asyncService.LogAndClassifyError(ctx, fmt.Errorf("error %d", i), "shutdown-test", severity)
		}

		// Shutdown with timeout - should process all queued errors
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second*2)
		defer cancel()

		err := asyncService.Shutdown(shutdownCtx)
		assert.NoError(t, err, "Shutdown should complete successfully")

		mockLogger.AssertExpectations(t)
	})

	t.Run("should timeout gracefully if shutdown takes too long", func(t *testing.T) {
		mockLogger := new(MockApplicationLogger)

		// Simulate slow processing
		mockLogger.On("Error", mock.AnythingOfType("*context.valueCtx"), mock.AnythingOfType("string"),
			mock.AnythingOfType("slogger.Fields")).Return().WaitUntil(time.After(time.Millisecond * 100))

		asyncService := NewAsyncErrorLoggingService(mockLogger, 20)

		ctx := context.Background()
		severity, _ := valueobject.NewErrorSeverity("ERROR")

		// Queue up many errors
		for i := range 20 {
			asyncService.LogAndClassifyError(ctx, fmt.Errorf("error %d", i), "timeout-test", severity)
		}

		// Short timeout - should timeout gracefully
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
		defer cancel()

		err := asyncService.Shutdown(shutdownCtx)
		assert.Error(t, err, "Should timeout")
		assert.Contains(t, err.Error(), "shutdown timeout")
	})
}

// Helper function to create test alerts.
func createTestAlert(
	ctx context.Context,
	severity *valueobject.ErrorSeverity,
	errorCode, message string,
) *entity.Alert {
	classifiedError, _ := entity.NewClassifiedError(ctx, errors.New("test error"), severity, errorCode, message, nil)
	alertType, _ := valueobject.NewAlertType("REAL_TIME")
	alert, _ := entity.NewAlert(classifiedError, alertType, message)
	return alert
}

// Interface definitions for performance and reliability components
// These should be implemented in the Green phase

type ErrorBuffer interface {
	Add(error *entity.ClassifiedError) bool
	Size() int
	IsFull() bool
	GetAll() []*entity.ClassifiedError
	GetBatch(count int) []*entity.ClassifiedError
	Clear()
	GetMemoryUsage() int64
}

type AlertDeliveryCircuitBreaker interface {
	Execute(fn func() error) error
	IsOpen() bool
	GetStatistics() CircuitBreakerStats
}

type CircuitBreakerStats struct {
	TotalCalls      int64
	SuccessfulCalls int64
	FailedCalls     int64
	SuccessRate     float64
	FailureRate     float64
}

type AlertDeduplicator interface {
	IsDuplicate(alert *entity.Alert) (bool, error)
	RecordAlert(alert *entity.Alert) error
	TrackDuplicate(alert *entity.Alert)
	GetDuplicateCount(deduplicationKey string) int
	ShouldSuppress(deduplicationKey string, threshold int) bool
}

type AsyncErrorLoggingService interface {
	LogAndClassifyError(ctx context.Context, err error, component string, severity *valueobject.ErrorSeverity) error
	Flush()
	Shutdown(ctx context.Context) error
}

// Constructor functions that should be implemented.
func NewErrorBuffer(size int) ErrorBuffer {
	return &mockErrorBuffer{size: size}
}

func NewAlertDeliveryCircuitBreaker(failureThreshold int, recoveryTimeout time.Duration) AlertDeliveryCircuitBreaker {
	return &mockCircuitBreaker{failureThreshold: failureThreshold, recoveryTimeout: recoveryTimeout}
}

func NewAlertDeduplicator(window time.Duration) AlertDeduplicator {
	return &mockDeduplicator{window: window}
}

func NewAsyncErrorLoggingService(logger logging.ApplicationLogger, bufferSize int) AsyncErrorLoggingService {
	return &mockAsyncService{logger: logger, bufferSize: bufferSize}
}

// Mock implementations for testing.
type mockErrorBuffer struct {
	errors []*entity.ClassifiedError
	size   int
}

func (m *mockErrorBuffer) Add(error *entity.ClassifiedError) bool { return true }
func (m *mockErrorBuffer) Size() int                              { return len(m.errors) }
func (m *mockErrorBuffer) IsFull() bool                           { return len(m.errors) >= m.size }
func (m *mockErrorBuffer) GetAll() []*entity.ClassifiedError      { return m.errors }
func (m *mockErrorBuffer) GetBatch(count int) []*entity.ClassifiedError {
	if count > len(m.errors) {
		count = len(m.errors)
	}
	return m.errors[:count]
}
func (m *mockErrorBuffer) Clear()                { m.errors = nil }
func (m *mockErrorBuffer) GetMemoryUsage() int64 { return int64(len(m.errors) * 1000) } // Rough estimate

type mockCircuitBreaker struct {
	failures         int
	isOpen           bool
	failureThreshold int
	recoveryTimeout  time.Duration
	lastFailure      time.Time
	totalCalls       int64
	successfulCalls  int64
	failedCalls      int64
}

func (m *mockCircuitBreaker) Execute(fn func() error) error {
	m.totalCalls++
	if m.isOpen && time.Since(m.lastFailure) < m.recoveryTimeout {
		return errors.New("circuit breaker is open")
	}

	err := fn()
	if err != nil {
		m.failures++
		m.failedCalls++
		m.lastFailure = time.Now()
		if m.failures >= m.failureThreshold {
			m.isOpen = true
		}
		return err
	}

	m.failures = 0
	m.isOpen = false
	m.successfulCalls++
	return nil
}

func (m *mockCircuitBreaker) IsOpen() bool {
	if m.isOpen && time.Since(m.lastFailure) >= m.recoveryTimeout {
		return false // Half-open state
	}
	return m.isOpen
}

func (m *mockCircuitBreaker) GetStatistics() CircuitBreakerStats {
	return CircuitBreakerStats{
		TotalCalls:      m.totalCalls,
		SuccessfulCalls: m.successfulCalls,
		FailedCalls:     m.failedCalls,
		SuccessRate:     float64(m.successfulCalls) / float64(m.totalCalls),
		FailureRate:     float64(m.failedCalls) / float64(m.totalCalls),
	}
}

type mockDeduplicator struct {
	window     time.Duration
	recorded   map[string]time.Time
	duplicates map[string]int
}

func (m *mockDeduplicator) IsDuplicate(alert *entity.Alert) (bool, error) {
	if m.recorded == nil {
		m.recorded = make(map[string]time.Time)
	}

	key := alert.DeduplicationKey()
	if lastSeen, exists := m.recorded[key]; exists {
		if time.Since(lastSeen) < m.window {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockDeduplicator) RecordAlert(alert *entity.Alert) error {
	if m.recorded == nil {
		m.recorded = make(map[string]time.Time)
	}
	m.recorded[alert.DeduplicationKey()] = time.Now()
	return nil
}

func (m *mockDeduplicator) TrackDuplicate(alert *entity.Alert) {
	if m.duplicates == nil {
		m.duplicates = make(map[string]int)
	}
	m.duplicates[alert.DeduplicationKey()]++
}

func (m *mockDeduplicator) GetDuplicateCount(deduplicationKey string) int {
	if m.duplicates == nil {
		return 0
	}
	return m.duplicates[deduplicationKey]
}

func (m *mockDeduplicator) ShouldSuppress(deduplicationKey string, threshold int) bool {
	return m.GetDuplicateCount(deduplicationKey) >= threshold
}

type mockAsyncService struct {
	logger     logging.ApplicationLogger
	bufferSize int
}

func (m *mockAsyncService) LogAndClassifyError(
	ctx context.Context,
	err error,
	component string,
	severity *valueobject.ErrorSeverity,
) error {
	// Simulate async processing - just return nil for now
	return nil
}

func (m *mockAsyncService) Flush() {
	// Simulate flush
}

func (m *mockAsyncService) Shutdown(ctx context.Context) error {
	// Simulate graceful shutdown
	select {
	case <-ctx.Done():
		return fmt.Errorf("shutdown timeout: %w", ctx.Err())
	case <-time.After(time.Millisecond * 10): // Simulate quick shutdown
		return nil
	}
}
