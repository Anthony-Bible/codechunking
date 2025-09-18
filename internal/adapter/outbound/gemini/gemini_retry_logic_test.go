package gemini

import (
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// TestRetryConfigValidation tests that retry configuration is properly validated.
func TestRetryConfigValidation_ShouldFailForInvalidMaxAttempts(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: -1, // Invalid: negative attempts
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      true,
	}

	err := config.Validate()

	if err == nil {
		t.Fatal("expected validation error for negative MaxAttempts, got nil")
	}
	if !contains(err.Error(), "MaxAttempts must be positive") {
		t.Errorf("expected error about MaxAttempts, got: %v", err)
	}
}

func TestRetryConfigValidation_ShouldFailForZeroBaseDelay(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   0, // Invalid: zero delay
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      true,
	}

	err := config.Validate()

	if err == nil {
		t.Fatal("expected validation error for zero BaseDelay, got nil")
	}
	if !contains(err.Error(), "BaseDelay must be positive") {
		t.Errorf("expected error about BaseDelay, got: %v", err)
	}
}

func TestRetryConfigValidation_ShouldFailForInvalidMultiplier(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  0.5, // Invalid: multiplier less than 1.0
		Jitter:      true,
	}

	err := config.Validate()

	if err == nil {
		t.Fatal("expected validation error for invalid Multiplier, got nil")
	}
	if !contains(err.Error(), "Multiplier must be at least 1.0") {
		t.Errorf("expected error about Multiplier, got: %v", err)
	}
}

func TestRetryConfigValidation_ShouldFailWhenMaxDelayLessThanBaseDelay(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   30 * time.Second,
		MaxDelay:    10 * time.Second, // Invalid: less than BaseDelay
		Multiplier:  2.0,
		Jitter:      true,
	}

	err := config.Validate()

	if err == nil {
		t.Fatal("expected validation error for MaxDelay < BaseDelay, got nil")
	}
	if !contains(err.Error(), "MaxDelay must be greater than or equal to BaseDelay") {
		t.Errorf("expected error about MaxDelay, got: %v", err)
	}
}

// TestRetryConfigDefaults tests that default values are properly applied.
func TestRetryConfigDefaults_ShouldApplyCorrectDefaults(t *testing.T) {
	config := &RetryConfig{}

	config.ApplyDefaults()

	expectedMaxAttempts := 3
	expectedBaseDelay := time.Second
	expectedMaxDelay := 30 * time.Second
	expectedMultiplier := 2.0
	expectedJitter := true

	if config.MaxAttempts != expectedMaxAttempts {
		t.Errorf("expected MaxAttempts %d, got %d", expectedMaxAttempts, config.MaxAttempts)
	}
	if config.BaseDelay != expectedBaseDelay {
		t.Errorf("expected BaseDelay %v, got %v", expectedBaseDelay, config.BaseDelay)
	}
	if config.MaxDelay != expectedMaxDelay {
		t.Errorf("expected MaxDelay %v, got %v", expectedMaxDelay, config.MaxDelay)
	}
	if config.Multiplier != expectedMultiplier {
		t.Errorf("expected Multiplier %f, got %f", expectedMultiplier, config.Multiplier)
	}
	if config.Jitter != expectedJitter {
		t.Errorf("expected Jitter %v, got %v", expectedJitter, config.Jitter)
	}
}

// TestErrorCategorization tests that errors are correctly categorized as retryable or non-retryable.
func TestErrorCategorization_ShouldIdentifyRetryableHTTPErrors(t *testing.T) {
	retryableStatusCodes := []int{
		http.StatusTooManyRequests,     // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout,      // 504
	}

	for _, statusCode := range retryableStatusCodes {
		t.Run(fmt.Sprintf("StatusCode_%d", statusCode), func(t *testing.T) {
			isRetryable := IsHTTPErrorRetryable(statusCode)

			if !isRetryable {
				t.Errorf("expected status code %d to be retryable, got false", statusCode)
			}
		})
	}
}

func TestErrorCategorization_ShouldIdentifyNonRetryableHTTPErrors(t *testing.T) {
	nonRetryableStatusCodes := []int{
		http.StatusBadRequest,   // 400
		http.StatusUnauthorized, // 401
		http.StatusForbidden,    // 403
		http.StatusNotFound,     // 404
	}

	for _, statusCode := range nonRetryableStatusCodes {
		t.Run(fmt.Sprintf("StatusCode_%d", statusCode), func(t *testing.T) {
			isRetryable := IsHTTPErrorRetryable(statusCode)

			if isRetryable {
				t.Errorf("expected status code %d to be non-retryable, got true", statusCode)
			}
		})
	}
}

func TestErrorCategorization_ShouldIdentifyRetryableNetworkErrors(t *testing.T) {
	retryableErrors := []error{
		&TimeoutError{Err: errors.New("network timeout")},
		&ConnectionError{Err: errors.New("connection refused")},
		context.DeadlineExceeded,
	}

	for i, err := range retryableErrors {
		t.Run(fmt.Sprintf("Error_%d", i), func(t *testing.T) {
			isRetryable := IsNetworkErrorRetryable(err)

			if !isRetryable {
				t.Errorf("expected error %v to be retryable, got false", err)
			}
		})
	}
}

func TestErrorCategorization_ShouldIdentifyNonRetryableNetworkErrors(t *testing.T) {
	nonRetryableErrors := []error{
		context.Canceled,
		&ValidationError{Err: errors.New("invalid input")},
	}

	for i, err := range nonRetryableErrors {
		t.Run(fmt.Sprintf("Error_%d", i), func(t *testing.T) {
			isRetryable := IsNetworkErrorRetryable(err)

			if isRetryable {
				t.Errorf("expected error %v to be non-retryable, got true", err)
			}
		})
	}
}

// TestExponentialBackoffCalculation tests that backoff delays are calculated correctly.
func TestExponentialBackoffCalculation_ShouldCalculateCorrectDelayWithoutJitter(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	expectedDelays := []time.Duration{
		time.Second,     // attempt 1
		2 * time.Second, // attempt 2
		4 * time.Second, // attempt 3
		8 * time.Second, // attempt 4
	}

	for attempt := 1; attempt <= 4; attempt++ {
		delay := config.CalculateBackoffDelay(attempt)
		expected := expectedDelays[attempt-1]

		if delay != expected {
			t.Errorf("attempt %d: expected delay %v, got %v", attempt, expected, delay)
		}
	}
}

func TestExponentialBackoffCalculation_ShouldRespectMaxDelay(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 10,
		BaseDelay:   time.Second,
		MaxDelay:    5 * time.Second, // Low max delay
		Multiplier:  2.0,
		Jitter:      false,
	}

	// After few attempts, delay should be capped at MaxDelay
	for attempt := 5; attempt <= 8; attempt++ {
		delay := config.CalculateBackoffDelay(attempt)

		if delay > config.MaxDelay {
			t.Errorf("attempt %d: delay %v exceeds MaxDelay %v", attempt, delay, config.MaxDelay)
		}
		if delay != config.MaxDelay {
			t.Errorf("attempt %d: expected delay to equal MaxDelay %v, got %v", attempt, config.MaxDelay, delay)
		}
	}
}

func TestExponentialBackoffCalculation_ShouldApplyJitterWhenEnabled(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      true,
	}

	// Generate multiple delays for the same attempt to test jitter variation
	delays := make([]time.Duration, 10)
	for i := range 10 {
		delays[i] = config.CalculateBackoffDelay(2)
	}

	// Check that we have variation (jitter is working)
	hasVariation := false
	firstDelay := delays[0]
	for i := 1; i < len(delays); i++ {
		if delays[i] != firstDelay {
			hasVariation = true
			break
		}
	}

	if !hasVariation {
		t.Error("expected jitter to create delay variation, got identical delays")
	}

	// Check that all delays are within reasonable bounds (50-100% of calculated delay)
	baseDelayForAttempt := config.BaseDelay * time.Duration(config.Multiplier) // 2 seconds for attempt 2
	minExpected := baseDelayForAttempt / 2                                     // 50% of base
	maxExpected := baseDelayForAttempt                                         // 100% of base

	for i, delay := range delays {
		if delay < minExpected || delay > maxExpected {
			t.Errorf("delay %d: %v is outside expected jitter range [%v, %v]", i, delay, minExpected, maxExpected)
		}
	}
}

// TestRetryWithRateLimitHeaders tests that retry-after headers are respected.
func TestRetryWithRateLimitHeaders_ShouldRespectRetryAfterHeader(t *testing.T) {
	rateLimitError := &outbound.EmbeddingError{
		Code:      "rate_limit_exceeded",
		Type:      "quota",
		Message:   "Rate limit exceeded",
		Retryable: true,
	}

	retryAfterSeconds := 60
	headers := map[string]string{
		"Retry-After": strconv.Itoa(retryAfterSeconds),
	}

	delay := CalculateRateLimitDelay(rateLimitError, headers)
	expectedDelay := time.Duration(retryAfterSeconds) * time.Second

	if delay != expectedDelay {
		t.Errorf("expected delay %v from Retry-After header, got %v", expectedDelay, delay)
	}
}

func TestRetryWithRateLimitHeaders_ShouldFallBackToBackoffWhenNoHeader(t *testing.T) {
	rateLimitError := &outbound.EmbeddingError{
		Code:      "rate_limit_exceeded",
		Type:      "quota",
		Message:   "Rate limit exceeded",
		Retryable: true,
	}

	headers := map[string]string{} // No Retry-After header
	config := &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	delay := CalculateRateLimitDelayWithFallback(rateLimitError, headers, config, 2)
	expectedDelay := config.CalculateBackoffDelay(2) // Should use exponential backoff

	if delay != expectedDelay {
		t.Errorf("expected fallback delay %v, got %v", expectedDelay, delay)
	}
}

// TestRetryExecutor tests the main retry execution logic.
func TestRetryExecutor_ShouldRetryRetryableErrorsUpToMaxAttempts(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond, // Short delay for testing
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutor(config)
	ctx := context.Background()

	attemptCount := 0
	retryableError := &outbound.EmbeddingError{
		Code:      "server_error",
		Type:      "server",
		Message:   "Internal server error",
		Retryable: true,
	}

	operation := func() error {
		attemptCount++
		return retryableError // Always fail with retryable error
	}

	err := executor.ExecuteWithRetry(ctx, operation)

	// Should have attempted MaxAttempts times
	if attemptCount != config.MaxAttempts {
		t.Errorf("expected %d attempts, got %d", config.MaxAttempts, attemptCount)
	}

	// Should return the original error after max attempts
	if err == nil {
		t.Fatal("expected error after max attempts, got nil")
	}
	if !errors.Is(err, retryableError) {
		t.Errorf("expected original retryable error, got: %v", err)
	}
}

func TestRetryExecutor_ShouldNotRetryNonRetryableErrors(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutor(config)
	ctx := context.Background()

	attemptCount := 0
	nonRetryableError := &outbound.EmbeddingError{
		Code:      "invalid_api_key",
		Type:      "auth",
		Message:   "Invalid API key",
		Retryable: false,
	}

	operation := func() error {
		attemptCount++
		return nonRetryableError
	}

	err := executor.ExecuteWithRetry(ctx, operation)

	// Should have attempted only once
	if attemptCount != 1 {
		t.Errorf("expected 1 attempt for non-retryable error, got %d", attemptCount)
	}

	// Should return the original error immediately
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, nonRetryableError) {
		t.Errorf("expected original non-retryable error, got: %v", err)
	}
}

func TestRetryExecutor_ShouldSucceedOnEventualSuccess(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutor(config)
	ctx := context.Background()

	attemptCount := 0
	operation := func() error {
		attemptCount++
		if attemptCount < 3 {
			return &outbound.EmbeddingError{
				Code:      "server_error",
				Type:      "server",
				Message:   "Temporary server error",
				Retryable: true,
			}
		}
		return nil // Success on third attempt
	}

	err := executor.ExecuteWithRetry(ctx, operation)
	if err != nil {
		t.Errorf("expected eventual success, got error: %v", err)
	}
	if attemptCount != 3 {
		t.Errorf("expected 3 attempts until success, got %d", attemptCount)
	}
}

func TestRetryExecutor_ShouldRespectContextCancellation(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	executor := NewRetryExecutor(config)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
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

	// Should have been cancelled before reaching max attempts
	if attemptCount >= config.MaxAttempts {
		t.Errorf("expected cancellation before max attempts, got %d attempts", attemptCount)
	}

	// Should return context cancellation error
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("expected context cancellation error, got: %v", err)
	}
}

// TestCircuitBreakerIntegration tests the circuit breaker pattern.
func TestCircuitBreaker_ShouldOpenAfterConsecutiveFailures(t *testing.T) {
	failureThreshold := 3
	timeout := time.Second
	breaker := NewCircuitBreaker(failureThreshold, timeout)

	// Cause consecutive failures to open circuit
	for range failureThreshold {
		err := &outbound.EmbeddingError{
			Code:      "server_error",
			Type:      "server",
			Message:   "Server error",
			Retryable: true,
		}
		_ = breaker.RecordFailure(err)
	}

	// Circuit should now be open
	if breaker.GetState() != CircuitBreakerStateOpen {
		t.Errorf(
			"expected circuit breaker to be OPEN after %d failures, got state: %v",
			failureThreshold,
			breaker.GetState(),
		)
	}
}

func TestCircuitBreaker_ShouldRejectRequestsWhenOpen(t *testing.T) {
	failureThreshold := 2
	timeout := time.Second
	breaker := NewCircuitBreaker(failureThreshold, timeout)

	// Open the circuit
	for range failureThreshold {
		err := &outbound.EmbeddingError{
			Code:      "server_error",
			Type:      "server",
			Message:   "Server error",
			Retryable: true,
		}
		_ = breaker.RecordFailure(err)
	}

	// Attempt to execute should be rejected
	executed := false
	operation := func() error {
		executed = true
		return nil
	}

	err := breaker.ExecuteOrReject(operation)

	if executed {
		t.Error("expected operation to be rejected by open circuit breaker, but it was executed")
	}
	if err == nil {
		t.Fatal("expected circuit breaker rejection error, got nil")
	}
	if !IsCircuitBreakerError(err) {
		t.Errorf("expected circuit breaker error, got: %v", err)
	}
}

func TestCircuitBreaker_ShouldTransitionToHalfOpenAfterTimeout(t *testing.T) {
	failureThreshold := 2
	timeout := 50 * time.Millisecond // Short timeout for testing
	breaker := NewCircuitBreaker(failureThreshold, timeout)

	// Open the circuit
	for range failureThreshold {
		err := &outbound.EmbeddingError{
			Code:      "server_error",
			Type:      "server",
			Message:   "Server error",
			Retryable: true,
		}
		_ = breaker.RecordFailure(err)
	}

	// Wait for timeout
	time.Sleep(timeout + 10*time.Millisecond)

	// Circuit should transition to HALF_OPEN
	if breaker.GetState() != CircuitBreakerStateHalfOpen {
		t.Errorf("expected circuit breaker to be HALF_OPEN after timeout, got state: %v", breaker.GetState())
	}
}

func TestCircuitBreaker_ShouldCloseOnSuccessfulHalfOpenExecution(t *testing.T) {
	failureThreshold := 2
	timeout := 50 * time.Millisecond
	breaker := NewCircuitBreaker(failureThreshold, timeout)

	// Open the circuit
	for range failureThreshold {
		err := &outbound.EmbeddingError{
			Code:      "server_error",
			Type:      "server",
			Message:   "Server error",
			Retryable: true,
		}
		_ = breaker.RecordFailure(err)
	}

	// Wait for half-open state
	time.Sleep(timeout + 10*time.Millisecond)

	// Execute successful operation
	operation := func() error {
		return nil // Success
	}

	err := breaker.ExecuteOrReject(operation)
	if err != nil {
		t.Errorf("expected successful execution in half-open state, got error: %v", err)
	}

	// Circuit should now be closed
	if breaker.GetState() != CircuitBreakerStateClosed {
		t.Errorf(
			"expected circuit breaker to be CLOSED after successful half-open execution, got state: %v",
			breaker.GetState(),
		)
	}
}

func TestCircuitBreaker_ShouldReopenOnHalfOpenFailure(t *testing.T) {
	failureThreshold := 2
	timeout := 50 * time.Millisecond
	breaker := NewCircuitBreaker(failureThreshold, timeout)

	// Open the circuit
	for range failureThreshold {
		err := &outbound.EmbeddingError{
			Code:      "server_error",
			Type:      "server",
			Message:   "Server error",
			Retryable: true,
		}
		_ = breaker.RecordFailure(err)
	}

	// Wait for half-open state
	time.Sleep(timeout + 10*time.Millisecond)

	// Execute failing operation
	operation := func() error {
		return &outbound.EmbeddingError{
			Code:      "server_error",
			Type:      "server",
			Message:   "Still failing",
			Retryable: true,
		}
	}

	_ = breaker.ExecuteOrReject(operation)

	// Circuit should be open again
	if breaker.GetState() != CircuitBreakerStateOpen {
		t.Errorf("expected circuit breaker to reopen after half-open failure, got state: %v", breaker.GetState())
	}
}

// TestRetryMetrics tests that retry metrics are properly tracked.
func TestRetryMetrics_ShouldTrackAttemptCounts(t *testing.T) {
	metrics := NewRetryMetrics()

	// Record some attempts
	metrics.RecordAttempt("operation1", 1)
	metrics.RecordAttempt("operation1", 2)
	metrics.RecordAttempt("operation2", 1)

	totalAttempts := metrics.GetTotalAttempts()
	expectedTotal := 3

	if totalAttempts != expectedTotal {
		t.Errorf("expected total attempts %d, got %d", expectedTotal, totalAttempts)
	}

	operation1Attempts := metrics.GetOperationAttempts("operation1")
	expectedOperation1 := 2

	if operation1Attempts != expectedOperation1 {
		t.Errorf("expected operation1 attempts %d, got %d", expectedOperation1, operation1Attempts)
	}
}

func TestRetryMetrics_ShouldTrackSuccessAndFailureCounts(t *testing.T) {
	metrics := NewRetryMetrics()

	metrics.RecordSuccess("operation1")
	metrics.RecordSuccess("operation2")
	metrics.RecordFailure("operation1", &outbound.EmbeddingError{Code: "server_error"})

	successCount := metrics.GetSuccessCount()
	failureCount := metrics.GetFailureCount()

	if successCount != 2 {
		t.Errorf("expected success count 2, got %d", successCount)
	}
	if failureCount != 1 {
		t.Errorf("expected failure count 1, got %d", failureCount)
	}
}

func TestRetryMetrics_ShouldCalculateSuccessRate(t *testing.T) {
	metrics := NewRetryMetrics()

	// Record 3 successes and 1 failure
	metrics.RecordSuccess("op1")
	metrics.RecordSuccess("op2")
	metrics.RecordSuccess("op3")
	metrics.RecordFailure("op4", &outbound.EmbeddingError{Code: "error"})

	successRate := metrics.GetSuccessRate()
	expectedRate := 0.75 // 3/4 = 75%

	if successRate != expectedRate {
		t.Errorf("expected success rate %f, got %f", expectedRate, successRate)
	}
}

func TestRetryMetrics_ShouldTrackBackoffDelays(t *testing.T) {
	metrics := NewRetryMetrics()

	delays := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
	}

	for _, delay := range delays {
		metrics.RecordBackoffDelay(delay)
	}

	averageDelay := metrics.GetAverageBackoffDelay()
	expectedAverage := 233333333 * time.Nanosecond // (100+200+400)/3 ms in nanoseconds

	if averageDelay != expectedAverage {
		t.Errorf("expected average delay %v, got %v", expectedAverage, averageDelay)
	}
}

// TestIntegratedRetryBehavior tests the full retry flow integration.
func TestIntegratedRetryBehavior_ShouldHandleRateLimitWithRetryAfter(t *testing.T) {
	client := &Client{
		config: &ClientConfig{
			APIKey: "test-key",
		},
	}

	retryConfig := &RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	ctx := context.Background()
	attemptCount := 0

	// Mock operation that returns rate limit error with retry-after header
	operation := func() (*outbound.EmbeddingResult, error) {
		attemptCount++
		if attemptCount < 3 {
			return nil, &RateLimitError{
				Code:       "rate_limit_exceeded",
				Message:    "Rate limit exceeded",
				RetryAfter: "2", // 2 seconds
			}
		}
		return &outbound.EmbeddingResult{Vector: []float64{1.0, 2.0, 3.0}}, nil
	}

	// This should be implemented in the actual retry logic
	result, err := client.ExecuteWithRetryAndCircuitBreaker(ctx, operation, retryConfig)
	if err != nil {
		t.Errorf("expected eventual success after rate limit, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result after successful retry, got nil")
	}
	if attemptCount != 3 {
		t.Errorf("expected 3 attempts, got %d", attemptCount)
	}
}

func TestIntegratedRetryBehavior_ShouldHandleCircuitBreakerTrip(t *testing.T) {
	client := &Client{
		config: &ClientConfig{
			APIKey: "test-key",
		},
	}

	retryConfig := &RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
		Jitter:      false,
	}

	circuitBreakerConfig := &CircuitBreakerConfig{
		FailureThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}

	ctx := context.Background()
	attemptCount := 0

	// Operation that always fails
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

	if err == nil {
		t.Fatal("expected error due to circuit breaker, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got: %v", result)
	}

	// Should have stopped trying after circuit breaker opened (after 2 failures)
	if attemptCount > circuitBreakerConfig.FailureThreshold {
		t.Errorf("expected at most %d attempts before circuit breaker trip, got %d",
			circuitBreakerConfig.FailureThreshold, attemptCount)
	}
}

func TestIntegratedRetryBehavior_ShouldRespectMaxBackoffDelay(t *testing.T) {
	retryConfig := &RetryConfig{
		MaxAttempts: 10,
		BaseDelay:   time.Second,
		MaxDelay:    5 * time.Second, // Cap at 5 seconds
		Multiplier:  2.0,
		Jitter:      false,
	}

	// Test that delays don't exceed MaxDelay even after many attempts
	for attempt := 5; attempt <= 10; attempt++ {
		delay := retryConfig.CalculateBackoffDelay(attempt)

		if delay > retryConfig.MaxDelay {
			t.Errorf("attempt %d: delay %v exceeds MaxDelay %v", attempt, delay, retryConfig.MaxDelay)
		}
	}
}

// Helper functions and types are now implemented in retry.go
