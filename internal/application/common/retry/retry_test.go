package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestRetryExecutor_SuccessOnFirstAttempt(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
		Jitter:        false,
	}

	executor := NewRetryExecutor(config)
	callCount := 0

	err := executor.Execute(ctx, func(ctx context.Context) error {
		callCount++
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 call, got: %d", callCount)
	}
}

func TestRetryExecutor_SuccessAfterRetries(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
		Jitter:        false,
	}

	executor := NewRetryExecutor(config)
	callCount := 0

	err := executor.Execute(ctx, func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary error")
		}
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls, got: %d", callCount)
	}
}

func TestRetryExecutor_FailureAfterMaxRetries(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxRetries:    2,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
		Jitter:        false,
	}

	executor := NewRetryExecutor(config)
	callCount := 0

	err := executor.Execute(ctx, func(ctx context.Context) error {
		callCount++
		return errors.New("timeout")
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if callCount != 3 { // Initial attempt + 2 retries
		t.Errorf("Expected 3 calls, got: %d", callCount)
	}

	expectedMsg := "operation failed after 2 retries"
	if err.Error() != expectedMsg+": timeout" {
		t.Errorf("Expected error message to contain '%s', got: %v", expectedMsg, err)
	}
}

func TestRetryExecutor_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
		Jitter:        false,
	}

	executor := NewRetryExecutor(config)
	callCount := 0

	err := executor.Execute(ctx, func(ctx context.Context) error {
		callCount++
		return errors.New("validation error")
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if callCount != 1 {
		t.Errorf("Expected 1 call (no retries for non-retryable error), got: %d", callCount)
	}
}

func TestRetryExecutor_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      500 * time.Millisecond,
		BackoffFactor: 2.0,
		Jitter:        false,
	}

	executor := NewRetryExecutor(config)
	callCount := 0

	// Cancel context after first failure
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := executor.Execute(ctx, func(ctx context.Context) error {
		callCount++
		return errors.New("timeout")
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}

	if callCount > 2 {
		t.Errorf("Expected at most 2 calls, got: %d", callCount)
	}
}

func TestRetryExecutor_ExponentialBackoff(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        false,
	}

	executor := NewRetryExecutor(config)

	testCases := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 10 * time.Millisecond},
		{2, 20 * time.Millisecond},
		{3, 40 * time.Millisecond},
		{4, 80 * time.Millisecond},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("attempt_%d", tc.attempt), func(t *testing.T) {
			delay := executor.calculateDelay(tc.attempt)
			if delay != tc.expected {
				t.Errorf("Expected delay %v for attempt %d, got: %v", tc.expected, tc.attempt, delay)
			}
		})
	}
}

func TestRetryExecutor_MaxDelayEnforced(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:    10,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
		Jitter:        false,
	}

	executor := NewRetryExecutor(config)

	// After enough attempts, delay should cap at MaxDelay
	delay := executor.calculateDelay(10)
	if delay > config.MaxDelay {
		t.Errorf("Expected delay to be capped at %v, got: %v", config.MaxDelay, delay)
	}
}

func TestRetryExecutor_JitterAddsVariation(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
	}

	executor := NewRetryExecutor(config)

	// Calculate delay multiple times for the same attempt
	delays := make([]time.Duration, 10)
	baseDelay := 100 * time.Millisecond

	for i := range 10 {
		delays[i] = executor.calculateDelay(1)
	}

	// Check that jitter adds variation (at least some delays should be different)
	// Jitter should be within ±25% of base delay
	minExpected := time.Duration(float64(baseDelay) * 0.75)
	maxExpected := time.Duration(float64(baseDelay) * 1.25)

	for _, delay := range delays {
		if delay < minExpected || delay > maxExpected {
			t.Errorf("Delay %v is outside expected jitter range [%v, %v]", delay, minExpected, maxExpected)
		}
	}
}

func TestDefaultRetryableChecker_DatabaseErrors(t *testing.T) {
	checker := &DefaultRetryableChecker{}

	retryableErrors := []error{
		errors.New("connection refused"),
		errors.New("connection reset"),
		errors.New("timeout"),
		errors.New("deadlock"),
		errors.New("connection lost"),
		errors.New("too many connections"),
		errors.New("database is locked"),
	}

	for _, err := range retryableErrors {
		if !checker.IsRetryable(err) {
			t.Errorf("Expected error to be retryable: %v", err)
		}
	}
}

func TestDefaultRetryableChecker_TemporaryErrors(t *testing.T) {
	checker := &DefaultRetryableChecker{}

	retryableErrors := []error{
		errors.New("temporary error"),
		errors.New("try again later"),
		errors.New("resource temporarily unavailable"),
	}

	for _, err := range retryableErrors {
		if !checker.IsRetryable(err) {
			t.Errorf("Expected error to be retryable: %v", err)
		}
	}
}

func TestDefaultRetryableChecker_NetworkErrors(t *testing.T) {
	checker := &DefaultRetryableChecker{}

	retryableErrors := []error{
		errors.New("network is unreachable"),
		errors.New("no route to host"),
		errors.New("connection timed out"),
	}

	for _, err := range retryableErrors {
		if !checker.IsRetryable(err) {
			t.Errorf("Expected error to be retryable: %v", err)
		}
	}
}

func TestDefaultRetryableChecker_NonRetryableErrors(t *testing.T) {
	checker := &DefaultRetryableChecker{}

	nonRetryableErrors := []error{
		errors.New("validation error"),
		errors.New("invalid input"),
		errors.New("not found"),
		errors.New("unauthorized"),
	}

	for _, err := range nonRetryableErrors {
		if checker.IsRetryable(err) {
			t.Errorf("Expected error to be non-retryable: %v", err)
		}
	}
}

func TestWithRetry_HelperFunction(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	err := WithRetry(ctx, func(ctx context.Context) error {
		callCount++
		if callCount < 2 {
			return errors.New("timeout")
		}
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 calls, got: %d", callCount)
	}
}

func TestWithRetryConfig_HelperFunction(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxRetries:    5,
		InitialDelay:  5 * time.Millisecond,
		MaxDelay:      50 * time.Millisecond,
		BackoffFactor: 2.0,
		Jitter:        false,
	}

	callCount := 0

	err := WithRetryConfig(ctx, config, func(ctx context.Context) error {
		callCount++
		if callCount < 4 {
			return errors.New("temporary failure")
		}
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if callCount != 4 {
		t.Errorf("Expected 4 calls, got: %d", callCount)
	}
}

// Custom retry checker for testing.
type customRetryChecker struct {
	retryableErrors map[string]bool
}

func (c *customRetryChecker) IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	return c.retryableErrors[err.Error()]
}

func TestWithRetryAndChecker_CustomChecker(t *testing.T) {
	ctx := context.Background()
	config := &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
		Jitter:        false,
	}

	checker := &customRetryChecker{
		retryableErrors: map[string]bool{
			"custom retryable error": true,
		},
	}

	callCount := 0

	err := WithRetryAndChecker(ctx, config, checker, func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return errors.New("custom retryable error")
		}
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls, got: %d", callCount)
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries=3, got: %d", config.MaxRetries)
	}

	if config.InitialDelay != 100*time.Millisecond {
		t.Errorf("Expected InitialDelay=100ms, got: %v", config.InitialDelay)
	}

	if config.MaxDelay != 5*time.Second {
		t.Errorf("Expected MaxDelay=5s, got: %v", config.MaxDelay)
	}

	if config.BackoffFactor != 2.0 {
		t.Errorf("Expected BackoffFactor=2.0, got: %f", config.BackoffFactor)
	}

	if !config.Jitter {
		t.Error("Expected Jitter=true")
	}
}

func TestNewRetryExecutor_NilConfigUsesDefault(t *testing.T) {
	executor := NewRetryExecutor(nil)

	if executor.config == nil {
		t.Error("Expected default config, got nil")
	}

	if executor.config.MaxRetries != 3 {
		t.Errorf("Expected default MaxRetries=3, got: %d", executor.config.MaxRetries)
	}
}

func TestNewRetryExecutorWithChecker_NilCheckerUsesDefault(t *testing.T) {
	executor := NewRetryExecutorWithChecker(nil, nil)

	if executor.retryableChecker == nil {
		t.Error("Expected default checker, got nil")
	}

	// Test that default checker is working
	err := errors.New("timeout")
	if !executor.retryableChecker.IsRetryable(err) {
		t.Error("Expected default checker to identify timeout as retryable")
	}
}
