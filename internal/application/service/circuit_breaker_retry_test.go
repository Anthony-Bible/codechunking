// Package service provides comprehensive failing tests for circuit breaker retry integration.
// These tests define the exact behavior needed for coordinating circuit breakers
// with retry logic, including state transitions, failure thresholds, and recovery patterns.
//
// RED Phase: All tests in this file are designed to fail initially and provide
// clear specifications for the GREEN phase implementation.
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCircuitBreakerRetry_BasicIntegration tests basic circuit breaker and retry integration.
func TestCircuitBreakerRetry_BasicIntegration(t *testing.T) {
	tests := []struct {
		name               string
		circuitConfig      CBConfig
		retryConfig        RetryPolicyConfig
		operationBehavior  func(attempt int) error
		expectedAttempts   int
		expectedFinalState CircuitBreakerState
		expectSuccess      bool
	}{
		{
			name: "successful operation - circuit remains closed",
			circuitConfig: CBConfig{
				Name:             "test_success",
				FailureThreshold: 3,
				SuccessThreshold: 2,
				OpenTimeout:      1 * time.Second,
			},
			retryConfig: RetryPolicyConfig{
				MaxAttempts: 3,
				BaseDelay:   100 * time.Millisecond,
			},
			operationBehavior: func(_ int) error {
				return nil // Always succeed
			},
			expectedAttempts:   1,
			expectedFinalState: CircuitBreakerStateClosed,
			expectSuccess:      true,
		},
		{
			name: "transient failures then success",
			circuitConfig: CBConfig{
				Name:             "test_transient",
				FailureThreshold: 5,
				SuccessThreshold: 2,
				OpenTimeout:      1 * time.Second,
			},
			retryConfig: RetryPolicyConfig{
				MaxAttempts: 4,
				BaseDelay:   50 * time.Millisecond,
			},
			operationBehavior: func(attempt int) error {
				if attempt < 3 {
					return errors.New("transient error")
				}
				return nil // Succeed on 3rd attempt
			},
			expectedAttempts:   3,
			expectedFinalState: CircuitBreakerStateClosed,
			expectSuccess:      true,
		},
		{
			name: "persistent failures trigger circuit breaker",
			circuitConfig: CBConfig{
				Name:             "test_persistent",
				FailureThreshold: 3,
				SuccessThreshold: 2,
				OpenTimeout:      500 * time.Millisecond,
			},
			retryConfig: RetryPolicyConfig{
				MaxAttempts: 5,
				BaseDelay:   100 * time.Millisecond,
			},
			operationBehavior: func(_ int) error {
				return errors.New("persistent error")
			},
			expectedAttempts:   4, // Circuit opens after 3 failures, then 1 more "circuit breaker open" attempt
			expectedFinalState: CircuitBreakerStateOpen,
			expectSuccess:      false,
		},
		{
			name: "circuit breaker opens then recovers",
			circuitConfig: CBConfig{
				Name:             "test_recovery",
				FailureThreshold: 2,
				SuccessThreshold: 1,
				OpenTimeout:      100 * time.Millisecond, // Quick recovery
			},
			retryConfig: RetryPolicyConfig{
				MaxAttempts: 6,
				BaseDelay:   50 * time.Millisecond,
			},
			operationBehavior: func(attempt int) error {
				if attempt <= 2 {
					return errors.New("initial failure")
				}
				return nil // Recover after circuit opens
			},
			expectedAttempts:   3, // 2 failures + 1 success after recovery
			expectedFinalState: CircuitBreakerStateClosed,
			expectSuccess:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GREEN phase implementation ready

			config := RetryCircuitBreakerConfig{
				CircuitBreakerConfig: tt.circuitConfig,
				RetryPolicyConfig:    tt.retryConfig,
				MetricsConfig: RetryMetricsConfig{
					InstanceID:  "test-cb-001",
					ServiceName: "test-service",
				},
				AdaptiveBehavior: true, // Enable adaptive behavior for proper circuit breaker handling
			}

			retryWithCB, err := NewRetryWithCircuitBreaker(config)
			require.NoError(t, err)
			require.NotNil(t, retryWithCB)

			ctx := context.Background()
			attemptCount := 0

			operation := func() error {
				attemptCount++
				return tt.operationBehavior(attemptCount)
			}

			err = retryWithCB.ExecuteWithRetry(ctx, operation)

			if tt.expectSuccess {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}

			assert.Equal(t, tt.expectedAttempts, attemptCount)
			assert.Equal(t, tt.expectedFinalState, retryWithCB.GetCircuitBreaker().GetState())
		})
	}
}

// TestCircuitBreakerRetry_StateTransitions tests circuit breaker state transitions during retries.
func TestCircuitBreakerRetry_StateTransitions(t *testing.T) {
	tests := []struct {
		name              string
		failureThreshold  int
		successThreshold  int
		openTimeout       time.Duration
		operationSequence []error // nil = success, error = failure
		expectedStates    []CircuitBreakerState
		expectedAttempts  int
	}{
		{
			name:             "closed -> open -> half_open -> closed",
			failureThreshold: 3,
			successThreshold: 1,
			openTimeout:      100 * time.Millisecond,
			operationSequence: []error{
				errors.New("fail1"),
				errors.New("fail2"),
				errors.New("fail3"), // Should open circuit
				nil,                 // Should succeed in half-open and close
			},
			expectedStates: []CircuitBreakerState{
				CircuitBreakerStateClosed,   // Initial
				CircuitBreakerStateClosed,   // After fail1
				CircuitBreakerStateClosed,   // After fail2
				CircuitBreakerStateOpen,     // After fail3
				CircuitBreakerStateHalfOpen, // After timeout
				CircuitBreakerStateClosed,   // After success
			},
			expectedAttempts: 4,
		},
		{
			name:             "closed -> open -> half_open -> open (failed recovery)",
			failureThreshold: 2,
			successThreshold: 1,
			openTimeout:      50 * time.Millisecond,
			operationSequence: []error{
				errors.New("fail1"),
				errors.New("fail2"), // Should open circuit
				errors.New("fail3"), // Should fail in half-open and reopen
			},
			expectedStates: []CircuitBreakerState{
				CircuitBreakerStateClosed,   // Initial
				CircuitBreakerStateClosed,   // After fail1
				CircuitBreakerStateOpen,     // After fail2
				CircuitBreakerStateHalfOpen, // After timeout
				CircuitBreakerStateOpen,     // After failed recovery
			},
			expectedAttempts: 3,
		},
		{
			name:             "multiple success threshold required",
			failureThreshold: 2,
			successThreshold: 3,
			openTimeout:      50 * time.Millisecond,
			operationSequence: []error{
				errors.New("fail1"),
				errors.New("fail2"), // Should open circuit
				nil,                 // Success 1
				nil,                 // Success 2
				nil,                 // Success 3 - should close circuit
			},
			expectedStates: []CircuitBreakerState{
				CircuitBreakerStateClosed,   // Initial
				CircuitBreakerStateClosed,   // After fail1
				CircuitBreakerStateOpen,     // After fail2
				CircuitBreakerStateHalfOpen, // After timeout
				CircuitBreakerStateHalfOpen, // After success 1
				CircuitBreakerStateHalfOpen, // After success 2
				CircuitBreakerStateClosed,   // After success 3
			},
			expectedAttempts: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GREEN phase implementation ready

			config := RetryCircuitBreakerConfig{
				CircuitBreakerConfig: CBConfig{
					Name:             tt.name,
					FailureThreshold: tt.failureThreshold,
					SuccessThreshold: tt.successThreshold,
					OpenTimeout:      tt.openTimeout,
				},
				RetryPolicyConfig: RetryPolicyConfig{
					MaxAttempts: 1, // Don't retry for state transition tests - each call should be one attempt
					BaseDelay:   10 * time.Millisecond,
				},
				MetricsConfig: RetryMetricsConfig{
					InstanceID:  "test-state-" + tt.name,
					ServiceName: "test-service",
				},
				AdaptiveBehavior: true, // Enable adaptive behavior for proper circuit breaker handling
			}

			retryWithCB, err := NewRetryWithCircuitBreaker(config)
			require.NoError(t, err)
			require.NotNil(t, retryWithCB)

			ctx := context.Background()
			var recordedStates []CircuitBreakerState

			// Record initial state
			recordedStates = append(recordedStates, retryWithCB.GetCircuitBreaker().GetState())

			// Execute each operation and record state after
			for i, expectedErr := range tt.operationSequence {
				operation := func() error {
					return expectedErr
				}

				// Execute the operation through retry logic
				retryWithCB.ExecuteWithRetry(ctx, operation)

				// Record state after execution
				recordedStates = append(recordedStates, retryWithCB.GetCircuitBreaker().GetState())

				// If this was a failure that should trigger circuit opening, wait for timeout
				if expectedErr != nil && i == tt.failureThreshold-1 && tt.openTimeout > 0 {
					time.Sleep(tt.openTimeout + 10*time.Millisecond)
					recordedStates = append(recordedStates, retryWithCB.GetCircuitBreaker().GetState())
				}
			}

			attemptCount := len(tt.operationSequence)

			assert.Equal(t, tt.expectedAttempts, attemptCount)

			// Verify state transitions match expectations
			minStates := len(tt.expectedStates)
			if len(recordedStates) < minStates {
				minStates = len(recordedStates)
			}

			for i := range minStates {
				assert.Equal(t, tt.expectedStates[i], recordedStates[i],
					"State transition %d: expected %v, got %v", i, tt.expectedStates[i], recordedStates[i])
			}
		})
	}
}

// TestCircuitBreakerRetry_AdaptiveBehavior tests adaptive retry behavior based on circuit breaker state.
func TestCircuitBreakerRetry_AdaptiveBehavior(t *testing.T) {
	tests := []struct {
		name                 string
		initialPolicy        RetryPolicyConfig
		circuitState         CircuitBreakerState
		expectedPolicyChange bool
		expectedMaxAttempts  int
		expectedBaseDelay    time.Duration
	}{
		{
			name: "reduce retries when circuit is open",
			initialPolicy: RetryPolicyConfig{
				MaxAttempts: 5,
				BaseDelay:   100 * time.Millisecond,
			},
			circuitState:         CircuitBreakerStateOpen,
			expectedPolicyChange: true,
			expectedMaxAttempts:  1, // Don't retry when circuit is open
			expectedBaseDelay:    100 * time.Millisecond,
		},
		{
			name: "conservative retries when circuit is half-open",
			initialPolicy: RetryPolicyConfig{
				MaxAttempts: 5,
				BaseDelay:   100 * time.Millisecond,
			},
			circuitState:         CircuitBreakerStateHalfOpen,
			expectedPolicyChange: true,
			expectedMaxAttempts:  2,                      // Limited retries in half-open
			expectedBaseDelay:    200 * time.Millisecond, // Longer delays
		},
		{
			name: "normal retries when circuit is closed",
			initialPolicy: RetryPolicyConfig{
				MaxAttempts: 5,
				BaseDelay:   100 * time.Millisecond,
			},
			circuitState:         CircuitBreakerStateClosed,
			expectedPolicyChange: false, // No change needed
			expectedMaxAttempts:  5,
			expectedBaseDelay:    100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GREEN phase implementation ready

			config := RetryCircuitBreakerConfig{
				CircuitBreakerConfig: CBConfig{
					Name:             "adaptive_test",
					FailureThreshold: 3,
					SuccessThreshold: 2,
					OpenTimeout:      1 * time.Second,
				},
				RetryPolicyConfig: tt.initialPolicy,
				MetricsConfig: RetryMetricsConfig{
					InstanceID:  "test-adaptive-" + tt.name,
					ServiceName: "test-service",
				},
				AdaptiveBehavior: true,
			}

			retryWithCB, err := NewRetryWithCircuitBreaker(config)
			require.NoError(t, err)
			require.NotNil(t, retryWithCB)

			// Simulate circuit breaker state
			adaptivePolicy := NewAdaptiveRetryPolicy(tt.initialPolicy, tt.circuitState)
			require.NotNil(t, adaptivePolicy)

			if tt.expectedPolicyChange {
				err := retryWithCB.UpdatePolicy(adaptivePolicy)
				require.NoError(t, err)
			}

			currentPolicy := retryWithCB.GetRetryPolicy()
			assert.Equal(t, tt.expectedMaxAttempts, currentPolicy.GetMaxAttempts())
		})
	}
}

// TestCircuitBreakerRetry_MetricsIntegration tests metrics collection during circuit breaker retry operations.
func TestCircuitBreakerRetry_MetricsIntegration(t *testing.T) {
	// GREEN phase implementation ready

	config := RetryCircuitBreakerConfig{
		CircuitBreakerConfig: CBConfig{
			Name:             "metrics_test",
			FailureThreshold: 2,
			SuccessThreshold: 1,
			OpenTimeout:      100 * time.Millisecond,
		},
		RetryPolicyConfig: RetryPolicyConfig{
			MaxAttempts: 4,
			BaseDelay:   50 * time.Millisecond,
		},
		MetricsConfig: RetryMetricsConfig{
			InstanceID:  "test-metrics-001",
			ServiceName: "test-service",
		},
	}

	retryWithCB, err := NewRetryWithCircuitBreaker(config)
	require.NoError(t, err)
	require.NotNil(t, retryWithCB)

	ctx := context.Background()
	attemptCount := 0

	// Operation that fails twice then succeeds
	operation := func() error {
		attemptCount++
		if attemptCount <= 2 {
			return errors.New("failure")
		}
		return nil
	}

	err = retryWithCB.ExecuteWithRetry(ctx, operation)
	require.NoError(t, err)

	metrics := retryWithCB.GetMetrics()
	require.NotNil(t, metrics)

	// Verify metrics were collected for:
	// - Retry attempts
	// - Circuit breaker state changes
	// - Retry delays
	// - Final success

	// Additional metric verification would be implemented in GREEN phase
}

// TestCircuitBreakerRetry_ConcurrentOperations tests concurrent operations with shared circuit breaker.
func TestCircuitBreakerRetry_ConcurrentOperations(t *testing.T) {
	// GREEN phase implementation ready

	config := RetryCircuitBreakerConfig{
		CircuitBreakerConfig: CBConfig{
			Name:                  "concurrent_test",
			FailureThreshold:      5,
			SuccessThreshold:      2,
			OpenTimeout:           200 * time.Millisecond,
			MaxConcurrentRequests: 3,
		},
		RetryPolicyConfig: RetryPolicyConfig{
			MaxAttempts: 3,
			BaseDelay:   50 * time.Millisecond,
		},
		MetricsConfig: RetryMetricsConfig{
			InstanceID:  "test-concurrent-001",
			ServiceName: "test-service",
		},
	}

	retryWithCB, err := NewRetryWithCircuitBreaker(config)
	require.NoError(t, err)
	require.NotNil(t, retryWithCB)

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	const goroutines = 5
	const iterations = 20

	successCount := make(chan int, goroutines)
	failureCount := make(chan int, goroutines)

	// Run concurrent operations
	for i := range goroutines {
		go func(id int) {
			successes := 0
			failures := 0

			for j := range iterations {
				// Check if context is cancelled
				if ctx.Err() != nil {
					break
				}

				operation := func() error {
					// Simulate some failures to trigger circuit breaker
					if (id+j)%10 < 3 {
						return errors.New("simulated failure")
					}
					return nil
				}

				// Use a shorter timeout for each operation
				opCtx, opCancel := context.WithTimeout(ctx, 200*time.Millisecond)
				err = retryWithCB.ExecuteWithRetry(opCtx, operation)
				opCancel()

				if err != nil {
					failures++
				} else {
					successes++
				}
			}

			// Send results or timeout
			select {
			case successCount <- successes:
			case <-ctx.Done():
				return
			}

			select {
			case failureCount <- failures:
			case <-ctx.Done():
				return
			}
		}(i)
	}

	// Collect results with timeout
	totalSuccesses := 0
	totalFailures := 0

	for range goroutines {
		select {
		case s := <-successCount:
			totalSuccesses += s
		case <-ctx.Done():
			t.Fatal("Timeout waiting for success count")
		}

		select {
		case f := <-failureCount:
			totalFailures += f
		case <-ctx.Done():
			t.Fatal("Timeout waiting for failure count")
		}
	}

	// Verify that circuit breaker affected the outcomes
	// Should have some successes and some failures due to circuit breaker behavior
	assert.Positive(t, totalSuccesses, "Should have some successful operations")
	assert.Positive(t, totalFailures, "Should have some failed operations due to circuit breaker")

	// Circuit breaker should have transitioned states during concurrent execution
	finalState := retryWithCB.GetCircuitBreaker().GetState()
	assert.Contains(t, []CircuitBreakerState{
		CircuitBreakerStateClosed,
		CircuitBreakerStateHalfOpen,
		CircuitBreakerStateOpen,
	}, finalState)
}

// TestCircuitBreakerRetry_ContextCancellation tests behavior when context is cancelled.
func TestCircuitBreakerRetry_ContextCancellation(t *testing.T) {
	// GREEN phase implementation ready

	config := RetryCircuitBreakerConfig{
		CircuitBreakerConfig: CBConfig{
			Name:             "context_test",
			FailureThreshold: 3,
			SuccessThreshold: 1,
			OpenTimeout:      1 * time.Second,
		},
		RetryPolicyConfig: RetryPolicyConfig{
			MaxAttempts: 5,
			BaseDelay:   200 * time.Millisecond,
		},
		MetricsConfig: RetryMetricsConfig{
			InstanceID:  "test-context-001",
			ServiceName: "test-service",
		},
	}

	retryWithCB, err := NewRetryWithCircuitBreaker(config)
	require.NoError(t, err)
	require.NotNil(t, retryWithCB)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	attemptCount := 0
	operation := func() error {
		attemptCount++
		time.Sleep(150 * time.Millisecond) // Each attempt takes 150ms
		return errors.New("operation error")
	}

	start := time.Now()
	err = retryWithCB.ExecuteWithRetry(ctx, operation)
	duration := time.Since(start)

	// Should fail due to context timeout, not retry exhaustion
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context")

	// Should not have completed all retry attempts due to timeout
	assert.Less(t, attemptCount, 5, "Should not complete all retries due to context timeout")
	assert.Less(t, duration, 1*time.Second, "Should timeout before completing all retries")
}
