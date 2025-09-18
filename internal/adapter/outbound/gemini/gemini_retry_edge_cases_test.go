package gemini

import (
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestRetryEdgeCases tests edge cases and boundary conditions for retry logic.
func TestRetryEdgeCases_ShouldHandleZeroMaxAttempts(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 0, // Edge case: zero attempts
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutor(config)
	ctx := context.Background()

	attemptCount := 0
	operation := func() error {
		attemptCount++
		return &outbound.EmbeddingError{
			Code:      "server_error",
			Type:      "server",
			Message:   "Server error",
			Retryable: true,
		}
	}

	err := executor.ExecuteWithRetry(ctx, operation)

	// Should not execute at all with zero max attempts
	if attemptCount != 0 {
		t.Errorf("expected 0 attempts with MaxAttempts=0, got %d", attemptCount)
	}

	// Should return an error indicating no attempts were made
	if err == nil {
		t.Fatal("expected error for zero max attempts, got nil")
	}

	if !IsZeroAttemptsError(err) {
		t.Errorf("expected zero attempts error, got: %v", err)
	}
}

func TestRetryEdgeCases_ShouldHandleSingleAttempt(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 1, // Edge case: single attempt (no retries)
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutor(config)
	ctx := context.Background()

	attemptCount := 0
	operation := func() error {
		attemptCount++
		return &outbound.EmbeddingError{
			Code:      "server_error",
			Type:      "server",
			Message:   "Server error",
			Retryable: true,
		}
	}

	err := executor.ExecuteWithRetry(ctx, operation)

	// Should attempt exactly once
	if attemptCount != 1 {
		t.Errorf("expected 1 attempt with MaxAttempts=1, got %d", attemptCount)
	}

	// Should return the original error without retrying
	if err == nil {
		t.Fatal("expected error after single failed attempt, got nil")
	}

	embeddingErr := &outbound.EmbeddingError{}
	ok := errors.As(err, &embeddingErr)
	if !ok || embeddingErr.Code != "server_error" {
		t.Errorf("expected original server error, got: %v", err)
	}
}

func TestRetryEdgeCases_ShouldHandleVeryLargeMaxAttempts(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 1000000,               // Edge case: very large max attempts
		BaseDelay:   1 * time.Millisecond,  // Very small delay to speed up test
		MaxDelay:    10 * time.Millisecond, // Cap at small delay
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutor(config)

	// Context with timeout to prevent test from running forever
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	attemptCount := 0
	operation := func() error {
		attemptCount++
		if attemptCount == 5 {
			return nil // Success after 5 attempts
		}
		return &outbound.EmbeddingError{
			Code:      "server_error",
			Type:      "server",
			Message:   "Server error",
			Retryable: true,
		}
	}

	err := executor.ExecuteWithRetry(ctx, operation)
	// Should succeed before hitting max attempts or timeout
	if err != nil {
		t.Errorf("expected success with large max attempts, got error: %v", err)
	}
	if attemptCount != 5 {
		t.Errorf("expected 5 attempts until success, got %d", attemptCount)
	}
}

func TestRetryEdgeCases_ShouldHandleNilOperation(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutor(config)
	ctx := context.Background()

	err := executor.ExecuteWithRetry(ctx, nil) // Nil operation

	// Should return error for nil operation
	if err == nil {
		t.Fatal("expected error for nil operation, got nil")
	}

	if !IsNilOperationError(err) {
		t.Errorf("expected nil operation error, got: %v", err)
	}
}

// TestRetryBackoffEdgeCases tests edge cases in backoff calculation.
func TestRetryBackoffEdgeCases_ShouldHandleExtremeMultiplier(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   time.Millisecond,
		MaxDelay:    time.Second,
		Multiplier:  1000.0, // Extreme multiplier
		Jitter:      false,
	}

	// Even with extreme multiplier, should be capped at MaxDelay
	for attempt := 1; attempt <= 5; attempt++ {
		delay := config.CalculateBackoffDelay(attempt)

		if delay > config.MaxDelay {
			t.Errorf("attempt %d: delay %v exceeds MaxDelay %v with extreme multiplier",
				attempt, delay, config.MaxDelay)
		}
	}
}

func TestRetryBackoffEdgeCases_ShouldHandleMinimalMultiplier(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  1.0, // Minimal multiplier - no exponential growth
		Jitter:      false,
	}

	// With multiplier 1.0, all delays should equal BaseDelay
	for attempt := 1; attempt <= 5; attempt++ {
		delay := config.CalculateBackoffDelay(attempt)

		if delay != config.BaseDelay {
			t.Errorf("attempt %d: expected delay %v with multiplier 1.0, got %v",
				attempt, config.BaseDelay, delay)
		}
	}
}

func TestRetryBackoffEdgeCases_ShouldHandleEqualBaseAndMaxDelay(t *testing.T) {
	sameDelay := 5 * time.Second
	config := &RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   sameDelay,
		MaxDelay:    sameDelay, // Same as BaseDelay
		Multiplier:  2.0,
		Jitter:      false,
	}

	// All delays should equal the base/max delay
	for attempt := 1; attempt <= 5; attempt++ {
		delay := config.CalculateBackoffDelay(attempt)

		if delay != sameDelay {
			t.Errorf("attempt %d: expected delay %v when base equals max, got %v",
				attempt, sameDelay, delay)
		}
	}
}

// TestRetryCircuitBreakerEdgeCases tests edge cases for circuit breaker.
func TestRetryCircuitBreakerEdgeCases_ShouldHandleZeroFailureThreshold(t *testing.T) {
	breaker := NewCircuitBreaker(0, time.Second) // Zero failure threshold

	// Should always be open (trip immediately)
	if breaker.GetState() != CircuitBreakerStateOpen {
		t.Error("circuit breaker with zero failure threshold should always be open")
	}

	// Should reject all operations
	operation := func() error {
		return nil // Would succeed if allowed
	}

	err := breaker.ExecuteOrReject(operation)
	if err == nil {
		t.Error("expected rejection with zero failure threshold, got success")
	}
}

func TestRetryCircuitBreakerEdgeCases_ShouldHandleVeryHighFailureThreshold(t *testing.T) {
	breaker := NewCircuitBreaker(1000000, time.Second) // Very high threshold

	// Should remain closed even after many failures
	for range 100 {
		_ = breaker.RecordFailure(&outbound.EmbeddingError{
			Code: "server_error",
			Type: "server",
		})
	}

	if breaker.GetState() != CircuitBreakerStateClosed {
		t.Error("circuit breaker should remain closed with very high failure threshold")
	}

	// Should still allow operations
	executed := false
	operation := func() error {
		executed = true
		return nil
	}

	err := breaker.ExecuteOrReject(operation)
	if err != nil {
		t.Errorf("expected operation to succeed with high threshold, got error: %v", err)
	}
	if !executed {
		t.Error("operation should have been executed")
	}
}

func TestRetryCircuitBreakerEdgeCases_ShouldHandleZeroTimeout(t *testing.T) {
	breaker := NewCircuitBreaker(2, 0) // Zero timeout

	// Open the circuit
	for range 2 {
		_ = breaker.RecordFailure(&outbound.EmbeddingError{Code: "server_error"})
	}

	// With zero timeout, should immediately become half-open
	if breaker.GetState() != CircuitBreakerStateHalfOpen {
		t.Error("circuit breaker should be half-open immediately with zero timeout")
	}
}

// TestRetryMetricsEdgeCases tests edge cases for retry metrics.
func TestRetryMetricsEdgeCases_ShouldHandleNoOperations(t *testing.T) {
	metrics := NewRetryMetrics()

	// All metrics should return zero/empty values when no operations recorded
	if metrics.GetTotalAttempts() != 0 {
		t.Errorf("expected 0 total attempts, got %d", metrics.GetTotalAttempts())
	}

	if metrics.GetSuccessCount() != 0 {
		t.Errorf("expected 0 success count, got %d", metrics.GetSuccessCount())
	}

	if metrics.GetFailureCount() != 0 {
		t.Errorf("expected 0 failure count, got %d", metrics.GetFailureCount())
	}

	// Success rate should be 0 when no operations
	successRate := metrics.GetSuccessRate()
	if successRate != 0.0 {
		t.Errorf("expected success rate 0.0 with no operations, got %f", successRate)
	}
}

func TestRetryMetricsEdgeCases_ShouldHandleAllFailures(t *testing.T) {
	metrics := NewRetryMetrics()

	// Record only failures
	for range 5 {
		metrics.RecordFailure("operation", &outbound.EmbeddingError{Code: "error"})
	}

	successRate := metrics.GetSuccessRate()
	if successRate != 0.0 {
		t.Errorf("expected success rate 0.0 with all failures, got %f", successRate)
	}

	if metrics.GetSuccessCount() != 0 {
		t.Errorf("expected 0 success count, got %d", metrics.GetSuccessCount())
	}

	if metrics.GetFailureCount() != 5 {
		t.Errorf("expected 5 failure count, got %d", metrics.GetFailureCount())
	}
}

func TestRetryMetricsEdgeCases_ShouldHandleAllSuccesses(t *testing.T) {
	metrics := NewRetryMetrics()

	// Record only successes
	for range 3 {
		metrics.RecordSuccess("operation")
	}

	successRate := metrics.GetSuccessRate()
	if successRate != 1.0 {
		t.Errorf("expected success rate 1.0 with all successes, got %f", successRate)
	}

	if metrics.GetSuccessCount() != 3 {
		t.Errorf("expected 3 success count, got %d", metrics.GetSuccessCount())
	}

	if metrics.GetFailureCount() != 0 {
		t.Errorf("expected 0 failure count, got %d", metrics.GetFailureCount())
	}
}

// TestRetryIntegrationEdgeCases tests integration edge cases.
func TestRetryIntegrationEdgeCases_ShouldHandleRapidContextCancellation(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 10,
		BaseDelay:   time.Second, // Long delay
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutor(config)

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before starting

	operation := func() error {
		return &outbound.EmbeddingError{
			Code:      "server_error",
			Type:      "server",
			Message:   "Server error",
			Retryable: true,
		}
	}

	err := executor.ExecuteWithRetry(ctx, operation)

	// Should return context cancellation error immediately
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestRetryIntegrationEdgeCases_ShouldHandleMixedRetryableNonRetryableErrors(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutor(config)
	ctx := context.Background()

	attemptCount := 0
	operation := func() error {
		attemptCount++
		switch attemptCount {
		case 1, 2:
			// First two attempts: retryable errors
			return &outbound.EmbeddingError{
				Code:      "server_error",
				Type:      "server",
				Message:   "Server error",
				Retryable: true,
			}
		case 3:
			// Third attempt: non-retryable error
			return &outbound.EmbeddingError{
				Code:      "invalid_api_key",
				Type:      "auth",
				Message:   "Invalid API key",
				Retryable: false,
			}
		default:
			// Should not reach here
			return nil
		}
	}

	err := executor.ExecuteWithRetry(ctx, operation)

	// Should stop at non-retryable error (3 attempts)
	if attemptCount != 3 {
		t.Errorf("expected 3 attempts before non-retryable error, got %d", attemptCount)
	}

	// Should return the non-retryable error
	if err == nil {
		t.Fatal("expected non-retryable error, got nil")
	}

	embeddingErr := &outbound.EmbeddingError{}
	ok := errors.As(err, &embeddingErr)
	if !ok || embeddingErr.Code != "invalid_api_key" {
		t.Errorf("expected invalid_api_key error, got: %v", err)
	}
}

func TestRetryIntegrationEdgeCases_ShouldHandleErrorThatBecomesNonRetryable(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutor(config)
	ctx := context.Background()

	attemptCount := 0
	operation := func() error {
		attemptCount++

		// Create an error that changes retryability based on attempts
		retryable := attemptCount < 3 // Becomes non-retryable after 2 attempts

		return &DynamicRetryabilityError{
			Code:      "dynamic_error",
			Message:   fmt.Sprintf("Error on attempt %d", attemptCount),
			Retryable: retryable,
		}
	}

	err := executor.ExecuteWithRetry(ctx, operation)

	// Should stop when error becomes non-retryable (3 attempts)
	if attemptCount != 3 {
		t.Errorf("expected 3 attempts before error becomes non-retryable, got %d", attemptCount)
	}

	// Should return the non-retryable error
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	dynamicErr := &DynamicRetryabilityError{}
	ok := errors.As(err, &dynamicErr)
	if !ok {
		t.Errorf("expected DynamicRetryabilityError, got: %T", err)
	} else if dynamicErr.Retryable {
		t.Error("expected final error to be non-retryable")
	}
}

func TestRetryIntegrationEdgeCases_ShouldHandleCircuitBreakerOpeningDuringRetries(t *testing.T) {
	retryConfig := &RetryConfig{
		MaxAttempts: 10,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	circuitBreakerConfig := &CircuitBreakerConfig{
		FailureThreshold: 3, // Opens after 3 failures
		Timeout:          time.Second,
	}

	client := &Client{
		config: &ClientConfig{
			APIKey: "test-key",
		},
	}

	ctx := context.Background()
	attemptCount := 0

	operation := func() (*outbound.EmbeddingResult, error) {
		attemptCount++
		return nil, &outbound.EmbeddingError{
			Code:      "server_error",
			Type:      "server",
			Message:   "Server error",
			Retryable: true,
		}
	}

	result, err := client.ExecuteWithRetryAndCircuitBreaker(ctx, operation, retryConfig, circuitBreakerConfig)

	// Should stop when circuit breaker opens (at failure threshold)
	if attemptCount > circuitBreakerConfig.FailureThreshold+1 { // +1 for potential race condition
		t.Errorf("expected at most %d attempts before circuit breaker opens, got %d",
			circuitBreakerConfig.FailureThreshold+1, attemptCount)
	}

	// Should return circuit breaker error
	if err == nil {
		t.Fatal("expected circuit breaker error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got: %v", result)
	}

	if !IsCircuitBreakerError(err) {
		t.Errorf("expected circuit breaker error, got: %v", err)
	}
}

// Helper types and functions for edge case tests.
type DynamicRetryabilityError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *DynamicRetryabilityError) Error() string {
	return e.Message
}

func (e *DynamicRetryabilityError) IsRetryable() bool {
	return e.Retryable
}

// Function implementations are now in retry.go

// TestRetryLoggingIntegration tests that retry operations are properly logged.
func TestRetryLoggingIntegration_ShouldLogAllRetryAttempts(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	// Mock logger to capture log entries
	logCapture := NewLogCapture()
	executor := NewRetryExecutorWithLogging(config, logCapture)
	ctx := context.Background()

	attemptCount := 0
	operation := func() error {
		attemptCount++
		return &outbound.EmbeddingError{
			Code:      "server_error",
			Type:      "server",
			Message:   "Server error",
			Retryable: true,
		}
	}

	_ = executor.ExecuteWithRetry(ctx, operation)

	// Verify all attempts were logged
	logEntries := logCapture.GetEntries()

	expectedMinLogs := config.MaxAttempts // At least one log per attempt
	if len(logEntries) < expectedMinLogs {
		t.Errorf("expected at least %d log entries, got %d", expectedMinLogs, len(logEntries))
	}

	// Verify log contents
	for i, entry := range logEntries {
		if i < config.MaxAttempts {
			if !contains(entry.Message, "retry attempt") {
				t.Errorf("log entry %d should mention retry attempt: %s", i, entry.Message)
			}
			if _, hasAttempt := entry.Fields["attempt"]; !hasAttempt {
				t.Errorf("log entry %d should include attempt number in fields", i)
			}
			if _, hasDelay := entry.Fields["delay"]; !hasDelay && i > 0 {
				t.Errorf("log entry %d should include backoff delay for retry attempts", i)
			}
		}
	}
}

func TestRetryLoggingIntegration_ShouldLogCircuitBreakerState(t *testing.T) {
	logCapture := NewLogCapture()
	breaker := NewCircuitBreakerWithLogging(2, time.Second, logCapture)

	// Open the circuit
	for range 2 {
		_ = breaker.RecordFailure(&outbound.EmbeddingError{
			Code: "server_error",
			Type: "server",
		})
	}

	// Verify circuit breaker state changes were logged
	logEntries := logCapture.GetEntries()

	var stateChangeLogs []LogEntry
	for _, entry := range logEntries {
		if contains(entry.Message, "circuit breaker") {
			stateChangeLogs = append(stateChangeLogs, entry)
		}
	}

	if len(stateChangeLogs) == 0 {
		t.Error("expected circuit breaker state change to be logged")
	}

	// Should log transition to OPEN state
	hasOpenStateLog := false
	for _, entry := range stateChangeLogs {
		messageHasOpened := contains(entry.Message, "opened")
		_, fieldsHasOpen := entry.Fields["OPEN"]
		if messageHasOpened || fieldsHasOpen {
			hasOpenStateLog = true
			break
		}
	}

	if !hasOpenStateLog {
		t.Error("expected log entry for circuit breaker opening")
	}
}

// Helper types for logging tests.
type LogEntry struct {
	Message string
	Fields  map[string]interface{}
	Level   string
}

type LogCapture struct {
	entries []LogEntry
}

func NewLogCapture() *LogCapture {
	return &LogCapture{entries: make([]LogEntry, 0)}
}

func (l *LogCapture) GetEntries() []LogEntry {
	return l.entries
}

func (l *LogCapture) AddEntry(message, level string, fields map[string]interface{}) {
	l.entries = append(l.entries, LogEntry{
		Message: message,
		Fields:  fields,
		Level:   level,
	})
}

type RetryExecutorWithLogging struct {
	config *RetryConfig
	logger *LogCapture
}

func NewRetryExecutorWithLogging(config *RetryConfig, logger *LogCapture) *RetryExecutorWithLogging {
	return &RetryExecutorWithLogging{config: config, logger: logger}
}

func (r *RetryExecutorWithLogging) ExecuteWithRetry(ctx context.Context, operation func() error) error {
	if operation == nil {
		return &NilOperationError{Message: "operation cannot be nil"}
	}

	if r.config.MaxAttempts <= 0 {
		return &ZeroAttemptsError{Message: "MaxAttempts is zero, no attempts will be made"}
	}

	var lastErr error

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Log retry attempt - always log each attempt
		logFields := map[string]interface{}{
			"attempt": attempt,
		}
		if attempt > 1 {
			logFields["delay"] = r.config.CalculateBackoffDelay(attempt - 1).String()
		}
		r.logger.AddEntry("retry attempt", "DEBUG", logFields)

		// Execute operation
		err := operation()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Don't retry if not retryable
		if !r.isRetryable(err) {
			return err
		}

		// Don't sleep after the last attempt
		if attempt == r.config.MaxAttempts {
			break
		}

		// Calculate and wait for delay
		delay := r.config.CalculateBackoffDelay(attempt)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

func (r *RetryExecutorWithLogging) isRetryable(err error) bool {
	// Check for dynamic retryability interface
	if dynamicErr, ok := err.(interface{ IsRetryable() bool }); ok {
		return dynamicErr.IsRetryable()
	}

	// Check for EmbeddingError
	var embeddingErr *outbound.EmbeddingError
	if errors.As(err, &embeddingErr) {
		return embeddingErr.Retryable
	}

	// Check for network errors
	return IsNetworkErrorRetryable(err)
}

type CircuitBreakerWithLogging struct {
	failureThreshold int
	timeout          time.Duration
	state            CircuitBreakerState
	failureCount     int
	lastFailureTime  time.Time
	mutex            sync.RWMutex
	logger           *LogCapture
}

func NewCircuitBreakerWithLogging(
	failureThreshold int,
	timeout time.Duration,
	logger *LogCapture,
) *CircuitBreakerWithLogging {
	initialState := CircuitBreakerStateClosed
	if failureThreshold <= 0 {
		initialState = CircuitBreakerStateOpen
	}

	return &CircuitBreakerWithLogging{
		failureThreshold: failureThreshold,
		timeout:          timeout,
		state:            initialState,
		failureCount:     0,
		logger:           logger,
	}
}

func (c *CircuitBreakerWithLogging) RecordFailure(err error) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.failureCount++
	c.lastFailureTime = time.Now()

	// Only transition to open if we have a positive failure threshold and meet it
	if c.failureThreshold > 0 && c.failureCount >= c.failureThreshold {
		previousState := c.state
		c.state = CircuitBreakerStateOpen

		if previousState != CircuitBreakerStateOpen {
			c.logger.AddEntry("circuit breaker opened", "WARN", map[string]interface{}{
				"failure_count":     c.failureCount,
				"failure_threshold": c.failureThreshold,
				"OPEN":              true,
			})
		}
	}

	return err
}

func (c *CircuitBreakerWithLogging) GetState() CircuitBreakerState {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// For zero or negative failure threshold, always return open
	if c.failureThreshold <= 0 {
		return CircuitBreakerStateOpen
	}

	return c.state
}
