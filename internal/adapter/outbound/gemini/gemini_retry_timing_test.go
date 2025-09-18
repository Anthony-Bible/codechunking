package gemini

import (
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"testing"
	"time"
)

// TestRetryTimingValidation tests timing aspects of retry logic.
func TestRetryTimingValidation_ShouldRespectActualBackoffTimings(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutor(config)
	ctx := context.Background()

	attemptTimes := []time.Time{}
	operation := func() error {
		attemptTimes = append(attemptTimes, time.Now())
		return &outbound.EmbeddingError{
			Code:      "server_error",
			Type:      "server",
			Message:   "Server error",
			Retryable: true,
		}
	}

	startTime := time.Now()
	_ = executor.ExecuteWithRetry(ctx, operation)

	// Verify we have correct number of attempts
	if len(attemptTimes) != config.MaxAttempts {
		t.Errorf("expected %d attempts, got %d", config.MaxAttempts, len(attemptTimes))
	}

	// Verify timing between attempts respects backoff
	expectedDelays := []time.Duration{
		0,                      // First attempt is immediate
		100 * time.Millisecond, // Second attempt after base delay
		200 * time.Millisecond, // Third attempt after 2x base delay
	}

	for i := 1; i < len(attemptTimes); i++ {
		actualDelay := attemptTimes[i].Sub(attemptTimes[i-1])
		expectedDelay := expectedDelays[i]

		// Allow 10ms tolerance for timing precision
		tolerance := 10 * time.Millisecond
		if actualDelay < expectedDelay-tolerance || actualDelay > expectedDelay+tolerance {
			t.Errorf("attempt %d: expected delay ~%v, got %v", i+1, expectedDelay, actualDelay)
		}
	}

	// Verify total execution time is reasonable
	totalDuration := time.Since(startTime)
	expectedTotal := 300 * time.Millisecond            // 0 + 100 + 200
	maxExpected := expectedTotal + 50*time.Millisecond // Allow overhead

	if totalDuration > maxExpected {
		t.Errorf("total execution time %v exceeds expected maximum %v", totalDuration, maxExpected)
	}
}

func TestRetryTimingValidation_ShouldApplyJitterCorrectly(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 2,
		BaseDelay:   200 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		Multiplier:  2.0,
		Jitter:      true,
	}

	executor := NewRetryExecutor(config)
	ctx := context.Background()

	// Run multiple iterations to test jitter variation
	delays := make([]time.Duration, 5)
	for run := range 5 {
		attemptTimes := []time.Time{}
		operation := func() error {
			attemptTimes = append(attemptTimes, time.Now())
			if len(attemptTimes) < 2 {
				return &outbound.EmbeddingError{
					Code:      "server_error",
					Type:      "server",
					Message:   "Server error",
					Retryable: true,
				}
			}
			return nil // Success on second attempt
		}

		_ = executor.ExecuteWithRetry(ctx, operation)

		if len(attemptTimes) >= 2 {
			delays[run] = attemptTimes[1].Sub(attemptTimes[0])
		}
	}

	// Verify jitter creates variation in delays
	baseDelay := config.BaseDelay
	minExpectedDelay := baseDelay / 2 // 50% of base with jitter
	maxExpectedDelay := baseDelay     // 100% of base with jitter

	hasVariation := false
	allWithinRange := true
	for i, delay := range delays {
		if delay < minExpectedDelay || delay > maxExpectedDelay {
			allWithinRange = false
			t.Errorf(
				"delay %d: %v outside expected jitter range [%v, %v]",
				i,
				delay,
				minExpectedDelay,
				maxExpectedDelay,
			)
		}

		if i > 0 && delay != delays[0] {
			hasVariation = true
		}
	}

	if !hasVariation {
		t.Error("expected jitter to create variation in delays, but all delays were identical")
	}

	if !allWithinRange {
		t.Error("some delays were outside expected jitter range")
	}
}

func TestRetryTimingValidation_ShouldHandleMaxDelayCapProperly(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 6,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    1 * time.Second, // Cap at 1 second
		Multiplier:  2.0,
		Jitter:      false,
	}

	// Test delay calculation for multiple attempts
	expectedDelays := []time.Duration{
		500 * time.Millisecond,  // attempt 1: base delay
		1000 * time.Millisecond, // attempt 2: 2x base = 1s (equals max)
		1000 * time.Millisecond, // attempt 3: capped at max delay
		1000 * time.Millisecond, // attempt 4: capped at max delay
		1000 * time.Millisecond, // attempt 5: capped at max delay
	}

	for attempt := 1; attempt <= 5; attempt++ {
		delay := config.CalculateBackoffDelay(attempt)
		expected := expectedDelays[attempt-1]

		if delay != expected {
			t.Errorf("attempt %d: expected delay %v, got %v", attempt, expected, delay)
		}

		// Ensure delay never exceeds MaxDelay
		if delay > config.MaxDelay {
			t.Errorf("attempt %d: delay %v exceeds MaxDelay %v", attempt, delay, config.MaxDelay)
		}
	}
}

// TestRetryTimingUnderLoad tests retry behavior under different load conditions.
func TestRetryTimingUnderLoad_ShouldMaintainBackoffUnderConcurrentOperations(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutor(config)

	// Run multiple concurrent retry operations
	concurrentOps := 5
	results := make(chan time.Duration, concurrentOps)

	for i := range concurrentOps {
		go func(operationID int) {
			ctx := context.Background()
			startTime := time.Now()
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
			duration := time.Since(startTime)
			results <- duration
		}(i)
	}

	// Collect results
	durations := make([]time.Duration, concurrentOps)
	for i := range concurrentOps {
		durations[i] = <-results
	}

	// All operations should take approximately the same time
	// (50ms + 100ms = 150ms total backoff time, plus overhead)
	expectedMinDuration := 150 * time.Millisecond
	expectedMaxDuration := 250 * time.Millisecond // Allow overhead

	for i, duration := range durations {
		if duration < expectedMinDuration || duration > expectedMaxDuration {
			t.Errorf("concurrent operation %d: duration %v outside expected range [%v, %v]",
				i, duration, expectedMinDuration, expectedMaxDuration)
		}
	}
}

// TestContextTimeoutDuringRetry tests context timeout behavior during retry backoff.
func TestContextTimeoutDuringRetry_ShouldCancelDuringBackoffDelay(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   200 * time.Millisecond, // Long delay
		MaxDelay:    5 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutor(config)

	// Context that times out during the first backoff delay
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	attemptCount := 0
	startTime := time.Now()

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
	duration := time.Since(startTime)

	// Should have been cancelled during backoff delay
	if err == nil {
		t.Fatal("expected context timeout error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}

	// Should have only attempted once before timeout
	if attemptCount != 1 {
		t.Errorf("expected 1 attempt before timeout, got %d", attemptCount)
	}

	// Should have completed quickly due to timeout
	maxExpectedDuration := 150 * time.Millisecond // Context timeout + some overhead
	if duration > maxExpectedDuration {
		t.Errorf("execution took too long %v, expected under %v", duration, maxExpectedDuration)
	}
}

func TestContextTimeoutDuringRetry_ShouldCompleteIfTimeoutIsLongerThanRetries(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 2,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutor(config)

	// Context with long timeout (longer than total retry time)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

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

	// Should have exhausted all retry attempts
	if attemptCount != config.MaxAttempts {
		t.Errorf("expected %d attempts, got %d", config.MaxAttempts, attemptCount)
	}

	// Should return the original error, not context timeout
	if err == nil {
		t.Fatal("expected retry exhaustion error, got nil")
	}

	embeddingErr := &outbound.EmbeddingError{}
	ok := errors.As(err, &embeddingErr)
	if !ok {
		t.Errorf("expected EmbeddingError, got: %T", err)
	} else if embeddingErr.Code != "server_error" {
		t.Errorf("expected original server_error, got: %s", embeddingErr.Code)
	}
}

// TestRetryAfterHeaderTiming tests timing behavior with Retry-After headers.
func TestRetryAfterHeaderTiming_ShouldWaitForRetryAfterDuration(t *testing.T) {
	rateLimitError := &RateLimitWithHeadersError{
		Code:    "rate_limit_exceeded",
		Message: "Rate limit exceeded",
		Headers: map[string]string{
			"Retry-After": "1", // 1 second
		},
	}

	config := &RetryConfig{
		MaxAttempts: 2,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutorWithRateLimitHandling(config)
	ctx := context.Background()

	attemptTimes := []time.Time{}
	operation := func() error {
		attemptTimes = append(attemptTimes, time.Now())
		if len(attemptTimes) < 2 {
			return rateLimitError
		}
		return nil // Success on second attempt
	}

	_ = executor.ExecuteWithRetry(ctx, operation)

	// Verify we waited approximately the Retry-After duration
	if len(attemptTimes) < 2 {
		t.Fatal("expected at least 2 attempts")
	}

	actualDelay := attemptTimes[1].Sub(attemptTimes[0])
	expectedDelay := 1 * time.Second
	tolerance := 50 * time.Millisecond

	if actualDelay < expectedDelay-tolerance || actualDelay > expectedDelay+tolerance {
		t.Errorf("expected delay ~%v from Retry-After header, got %v", expectedDelay, actualDelay)
	}
}

func TestRetryAfterHeaderTiming_ShouldFallbackToBackoffWhenRetryAfterMissing(t *testing.T) {
	rateLimitError := &RateLimitWithHeadersError{
		Code:    "rate_limit_exceeded",
		Message: "Rate limit exceeded",
		Headers: map[string]string{}, // No Retry-After header
	}

	config := &RetryConfig{
		MaxAttempts: 2,
		BaseDelay:   150 * time.Millisecond,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutorWithRateLimitHandling(config)
	ctx := context.Background()

	attemptTimes := []time.Time{}
	operation := func() error {
		attemptTimes = append(attemptTimes, time.Now())
		if len(attemptTimes) < 2 {
			return rateLimitError
		}
		return nil // Success on second attempt
	}

	_ = executor.ExecuteWithRetry(ctx, operation)

	// Should fallback to exponential backoff delay
	if len(attemptTimes) < 2 {
		t.Fatal("expected at least 2 attempts")
	}

	actualDelay := attemptTimes[1].Sub(attemptTimes[0])
	expectedDelay := config.BaseDelay // First retry uses base delay
	tolerance := 50 * time.Millisecond

	if actualDelay < expectedDelay-tolerance || actualDelay > expectedDelay+tolerance {
		t.Errorf("expected fallback delay ~%v, got %v", expectedDelay, actualDelay)
	}
}

func TestRetryAfterHeaderTiming_ShouldRespectMaxDelayEvenWithRetryAfter(t *testing.T) {
	rateLimitError := &RateLimitWithHeadersError{
		Code:    "rate_limit_exceeded",
		Message: "Rate limit exceeded",
		Headers: map[string]string{
			"Retry-After": "60", // 60 seconds - longer than max delay
		},
	}

	config := &RetryConfig{
		MaxAttempts: 2,
		BaseDelay:   1 * time.Second,
		MaxDelay:    5 * time.Second, // Shorter than Retry-After
		Multiplier:  2.0,
		Jitter:      false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	executor := NewRetryExecutorWithRateLimitHandling(config)

	startTime := time.Now()
	operation := func() error {
		if time.Since(startTime) < 4*time.Second {
			return rateLimitError
		}
		return nil
	}

	_ = executor.ExecuteWithRetry(ctx, operation)

	// Should have capped the delay at MaxDelay, not used full Retry-After
	totalDuration := time.Since(startTime)
	maxExpected := config.MaxDelay + 500*time.Millisecond // MaxDelay + overhead

	if totalDuration > maxExpected {
		t.Errorf("total duration %v suggests Retry-After was not capped at MaxDelay %v",
			totalDuration, config.MaxDelay)
	}
}

// TestCircuitBreakerTimingBehavior tests timing aspects of circuit breaker.
func TestCircuitBreakerTimingBehavior_ShouldRespectTimeoutBeforeHalfOpen(t *testing.T) {
	timeout := 100 * time.Millisecond
	breaker := NewCircuitBreaker(2, timeout)

	// Open the circuit
	for range 2 {
		_ = breaker.RecordFailure(&outbound.EmbeddingError{
			Code: "server_error",
			Type: "server",
		})
	}

	// Verify circuit is open
	if breaker.GetState() != CircuitBreakerStateOpen {
		t.Fatal("expected circuit to be open")
	}

	// Should reject immediately when open
	startTime := time.Now()
	err := breaker.ExecuteOrReject(func() error { return nil })
	immediateRejectTime := time.Since(startTime)

	if err == nil {
		t.Error("expected rejection when circuit is open")
	}

	if immediateRejectTime > 10*time.Millisecond {
		t.Errorf("rejection should be immediate, took %v", immediateRejectTime)
	}

	// Wait for timeout but not quite enough
	time.Sleep(timeout / 2)

	// Should still be open and reject
	if breaker.GetState() != CircuitBreakerStateOpen {
		t.Error("circuit should still be open before timeout")
	}

	// Wait for full timeout
	time.Sleep(timeout/2 + 10*time.Millisecond)

	// Now should transition to half-open
	if breaker.GetState() != CircuitBreakerStateHalfOpen {
		t.Errorf("expected circuit to be half-open after timeout, got state: %v", breaker.GetState())
	}
}

func TestCircuitBreakerTimingBehavior_ShouldResetTimeoutOnNewFailures(t *testing.T) {
	timeout := 200 * time.Millisecond
	breaker := NewCircuitBreaker(2, timeout)

	// Open circuit with initial failures
	for range 2 {
		_ = breaker.RecordFailure(&outbound.EmbeddingError{Code: "server_error"})
	}

	openTime := time.Now()

	// Wait halfway through timeout
	time.Sleep(timeout / 2)

	// Add another failure while open (should reset timeout)
	_ = breaker.RecordFailure(&outbound.EmbeddingError{Code: "server_error"})

	// Wait original timeout duration from first opening
	time.Sleep(timeout/2 + 10*time.Millisecond)

	// Should still be open because timeout was reset
	if breaker.GetState() != CircuitBreakerStateOpen {
		t.Error("circuit should still be open - timeout should have been reset by new failure")
	}

	// Wait additional time for reset timeout
	totalElapsed := time.Since(openTime)
	remainingWait := timeout*3/2 - totalElapsed + 20*time.Millisecond
	if remainingWait > 0 {
		time.Sleep(remainingWait)
	}

	// Now should be half-open
	if breaker.GetState() != CircuitBreakerStateHalfOpen {
		t.Errorf("expected half-open after reset timeout, got: %v", breaker.GetState())
	}
}

// Additional helper types and functions for timing tests.
type RateLimitWithHeadersError struct {
	Code    string
	Message string
	Headers map[string]string
}

func (e *RateLimitWithHeadersError) Error() string {
	return e.Message
}

func (e *RateLimitWithHeadersError) GetHeaders() map[string]string {
	return e.Headers
}

type RetryExecutorWithRateLimitHandling struct {
	config *RetryConfig
}

func NewRetryExecutorWithRateLimitHandling(config *RetryConfig) *RetryExecutorWithRateLimitHandling {
	return &RetryExecutorWithRateLimitHandling{config: config}
}

func (r *RetryExecutorWithRateLimitHandling) ExecuteWithRetry(ctx context.Context, operation func() error) error {
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

		// Calculate and wait for delay - handle rate limit headers
		delay := r.calculateRetryDelay(err, attempt)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

func (r *RetryExecutorWithRateLimitHandling) isRetryable(err error) bool {
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

func (r *RetryExecutorWithRateLimitHandling) calculateRetryDelay(err error, attempt int) time.Duration {
	// Try to extract rate limit delay
	if delay := r.extractRateLimitDelay(err); delay > 0 {
		// Respect MaxDelay even for rate limit headers
		if delay > r.config.MaxDelay {
			return r.config.MaxDelay
		}
		return delay
	}

	// Fallback to exponential backoff
	return r.config.CalculateBackoffDelay(attempt)
}

func (r *RetryExecutorWithRateLimitHandling) extractRateLimitDelay(err error) time.Duration {
	// Check for rate limit errors with headers interface
	if rateLimitErr, ok := err.(interface{ GetHeaders() map[string]string }); ok {
		headers := rateLimitErr.GetHeaders()
		if headers != nil {
			return CalculateRateLimitDelay(nil, headers)
		}
	}

	// Check for RateLimitError with RetryAfter field
	rateLimitErr := &RateLimitError{}
	if errors.As(err, &rateLimitErr) && rateLimitErr.RetryAfter != "" {
		headers := map[string]string{"Retry-After": rateLimitErr.RetryAfter}
		return CalculateRateLimitDelay(nil, headers)
	}

	return 0
}

// TestRetryMetricsTiming tests timing aspects of retry metrics.
func TestRetryMetricsTiming_ShouldTrackRetryDurations(t *testing.T) {
	metrics := NewRetryMetricsWithTiming()

	operation1Duration := 150 * time.Millisecond
	operation2Duration := 300 * time.Millisecond

	metrics.RecordOperationDuration("operation1", operation1Duration)
	metrics.RecordOperationDuration("operation2", operation2Duration)
	metrics.RecordOperationDuration("operation1", operation1Duration) // Another instance

	avgDuration := metrics.GetAverageOperationDuration("operation1")
	expectedAvg := operation1Duration // Both were the same duration

	if avgDuration != expectedAvg {
		t.Errorf("expected average duration %v for operation1, got %v", expectedAvg, avgDuration)
	}

	totalAvg := metrics.GetOverallAverageDuration()
	expectedTotalAvg := (operation1Duration*2 + operation2Duration) / 3

	if totalAvg != expectedTotalAvg {
		t.Errorf("expected overall average duration %v, got %v", expectedTotalAvg, totalAvg)
	}
}

func TestRetryMetricsTiming_ShouldTrackTimeoutOccurrences(t *testing.T) {
	metrics := NewRetryMetricsWithTiming()

	metrics.RecordTimeout("operation1")
	metrics.RecordTimeout("operation2")
	metrics.RecordTimeout("operation1") // Second timeout for operation1

	operation1Timeouts := metrics.GetTimeoutCount("operation1")
	if operation1Timeouts != 2 {
		t.Errorf("expected 2 timeouts for operation1, got %d", operation1Timeouts)
	}

	totalTimeouts := metrics.GetTotalTimeoutCount()
	if totalTimeouts != 3 {
		t.Errorf("expected 3 total timeouts, got %d", totalTimeouts)
	}

	timeoutRate := metrics.GetTimeoutRate("operation1", 5) // 5 total attempts
	expectedRate := 2.0 / 5.0                              // 2 timeouts out of 5 attempts
	if timeoutRate != expectedRate {
		t.Errorf("expected timeout rate %f for operation1, got %f", expectedRate, timeoutRate)
	}
}

// Timing functionality is now implemented in retry.go
