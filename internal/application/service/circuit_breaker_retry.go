package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState int

const (
	CircuitBreakerStateClosed CircuitBreakerState = iota
	CircuitBreakerStateOpen
	CircuitBreakerStateHalfOpen
)

const (
	CircuitBreakerStateClosedStr   = "closed"
	CircuitBreakerStateOpenStr     = "open"
	CircuitBreakerStateHalfOpenStr = "half_open"
	unknownStateStr                = "unknown"
)

// String returns the string representation of circuit breaker state.
func (cbs CircuitBreakerState) String() string {
	switch cbs {
	case CircuitBreakerStateClosed:
		return CircuitBreakerStateClosedStr
	case CircuitBreakerStateOpen:
		return CircuitBreakerStateOpenStr
	case CircuitBreakerStateHalfOpen:
		return CircuitBreakerStateHalfOpenStr
	default:
		return unknownStateStr
	}
}

// CircuitBreaker defines the interface for circuit breaker functionality.
type CircuitBreaker interface {
	// Execute executes a function with circuit breaker protection.
	Execute(ctx context.Context, operation func() error) error

	// IsOpen returns true if the circuit breaker is open.
	IsOpen() bool

	// GetState returns the current circuit breaker state.
	GetState() CircuitBreakerState

	// GetFailureCount returns the current failure count.
	GetFailureCount() int

	// GetSuccessCount returns the current success count.
	GetSuccessCount() int

	// Reset manually resets the circuit breaker to closed state.
	Reset()

	// GetName returns the circuit breaker name.
	GetName() string
}

// CBConfig holds configuration for circuit breaker behavior.
type CBConfig struct {
	Name                  string
	FailureThreshold      int
	SuccessThreshold      int
	OpenTimeout           time.Duration
	MaxConcurrentRequests int32
	OnStateChange         func(name string, from, to CircuitBreakerState)
}

// RetryWithCircuitBreaker combines retry logic with circuit breaker protection.
type RetryWithCircuitBreaker interface {
	// ExecuteWithRetry executes an operation with both retry and circuit breaker protection.
	ExecuteWithRetry(ctx context.Context, operation func() error) error

	// GetRetryPolicy returns the current retry policy.
	GetRetryPolicy() RetryPolicy

	// GetCircuitBreaker returns the circuit breaker instance.
	GetCircuitBreaker() CircuitBreaker

	// GetMetrics returns the metrics collector.
	GetMetrics() RetryMetrics

	// UpdatePolicy updates the retry policy (for adaptive behavior).
	UpdatePolicy(policy RetryPolicy) error
}

// RetryCircuitBreakerConfig holds configuration for integrated retry with circuit breaker.
type RetryCircuitBreakerConfig struct {
	CircuitBreakerConfig CBConfig
	RetryPolicyConfig    RetryPolicyConfig
	MetricsConfig        RetryMetricsConfig
	AdaptiveBehavior     bool
	StateChangeCallback  func(operationName string, state CircuitBreakerState, retryAttempt int)
}

// DefaultCircuitBreaker implements a basic circuit breaker.
type DefaultCircuitBreaker struct {
	config       CBConfig
	state        CircuitBreakerState
	failureCount int
	successCount int
	lastFailure  time.Time
	mu           sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration.
func NewCircuitBreaker(config CBConfig) CircuitBreaker {
	return &DefaultCircuitBreaker{
		config: config,
		state:  CircuitBreakerStateClosed,
	}
}

func (cb *DefaultCircuitBreaker) Execute(ctx context.Context, operation func() error) error {
	// Check if context is canceled
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Check current state and decide whether to allow operation
	if !cb.canExecute() {
		return errors.New("circuit breaker open")
	}

	// Execute the operation
	err := operation()

	// Update state based on result
	cb.recordResult(err)

	return err
}

func (cb *DefaultCircuitBreaker) canExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitBreakerStateClosed:
		return true
	case CircuitBreakerStateOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailure) > cb.config.OpenTimeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			defer cb.mu.Unlock()
			cb.state = CircuitBreakerStateHalfOpen
			return true
		}
		return false
	case CircuitBreakerStateHalfOpen:
		return true
	default:
		return false
	}
}

func (cb *DefaultCircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
		return
	}

	cb.recordSuccess()
}

func (cb *DefaultCircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.lastFailure = time.Now()

	// Handle state transitions on failure
	switch cb.state {
	case CircuitBreakerStateClosed:
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.transitionToOpen()
		}
	case CircuitBreakerStateHalfOpen:
		// Go back to open on failure in half-open state
		cb.transitionToOpen()
	case CircuitBreakerStateOpen:
		// Already open, no state transition needed
	}
}

func (cb *DefaultCircuitBreaker) recordSuccess() {
	cb.successCount++

	// Reset failure count on success
	switch cb.state {
	case CircuitBreakerStateClosed:
		cb.failureCount = 0
	case CircuitBreakerStateHalfOpen:
		// Check if we have enough successes to close
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.transitionToClosed()
		}
	case CircuitBreakerStateOpen:
		// No action needed for open state on success - transitions are handled elsewhere
	}
}

func (cb *DefaultCircuitBreaker) transitionToOpen() {
	oldState := cb.state
	cb.state = CircuitBreakerStateOpen
	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(cb.config.Name, oldState, cb.state)
	}
}

func (cb *DefaultCircuitBreaker) transitionToClosed() {
	oldState := cb.state
	cb.state = CircuitBreakerStateClosed
	cb.failureCount = 0
	cb.successCount = 0
	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(cb.config.Name, oldState, cb.state)
	}
}

func (cb *DefaultCircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func (cb *DefaultCircuitBreaker) GetFailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failureCount
}

func (cb *DefaultCircuitBreaker) GetSuccessCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.successCount
}

func (cb *DefaultCircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := cb.state
	cb.state = CircuitBreakerStateClosed
	cb.failureCount = 0
	cb.successCount = 0

	if cb.config.OnStateChange != nil && oldState != cb.state {
		cb.config.OnStateChange(cb.config.Name, oldState, cb.state)
	}
}

func (cb *DefaultCircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == CircuitBreakerStateOpen
}

func (cb *DefaultCircuitBreaker) GetName() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.config.Name
}

// DefaultRetryWithCircuitBreaker combines retry policy with circuit breaker.
type DefaultRetryWithCircuitBreaker struct {
	retryPolicy    RetryPolicy
	circuitBreaker CircuitBreaker
	metrics        RetryMetrics
	config         RetryCircuitBreakerConfig
	mu             sync.RWMutex
}

// NewRetryWithCircuitBreaker creates a new retry with circuit breaker instance.
func NewRetryWithCircuitBreaker(config RetryCircuitBreakerConfig) (RetryWithCircuitBreaker, error) {
	// Create circuit breaker
	circuitBreaker := NewCircuitBreaker(config.CircuitBreakerConfig)

	// Create retry policy
	retryPolicy := NewExponentialBackoffPolicy(config.RetryPolicyConfig)

	// Create metrics
	metrics, err := NewRetryMetrics(config.MetricsConfig)
	if err != nil {
		return nil, err
	}

	return &DefaultRetryWithCircuitBreaker{
		retryPolicy:    retryPolicy,
		circuitBreaker: circuitBreaker,
		metrics:        metrics,
		config:         config,
	}, nil
}

func (r *DefaultRetryWithCircuitBreaker) ExecuteWithRetry(ctx context.Context, operation func() error) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var lastErr error
	attempt := 1
	startTime := time.Now()

	for {
		// Check circuit breaker state first
		cbState := r.circuitBreaker.GetState()
		if cbState == CircuitBreakerStateOpen {
			// Record circuit breaker event
			r.metrics.RecordCircuitBreakerEvent(ctx, CircuitBreakerStateOpenStr, "retry_operation")
			return errors.New("circuit breaker open")
		}

		// Execute through circuit breaker
		err := r.circuitBreaker.Execute(ctx, operation)

		if err == nil {
			// Success - record metrics and return
			totalDuration := time.Since(startTime)
			r.metrics.RecordRetrySuccess(ctx, attempt, totalDuration, "retry_operation")
			return nil
		}

		lastErr = err

		// Check if we should retry
		shouldRetry, delay := r.retryPolicy.ShouldRetry(ctx, err, attempt)
		if !shouldRetry {
			// Exhausted - record metrics and return error
			totalDuration := time.Since(startTime)
			r.metrics.RecordRetryExhaustion(
				ctx,
				r.retryPolicy.GetMaxAttempts(),
				totalDuration,
				lastErr,
				"retry_operation",
			)
			return lastErr
		}

		// Record retry attempt and delay
		classifier := NewFailureClassifier()
		failureType := classifier.Classify(err)
		r.metrics.RecordRetryAttempt(ctx, attempt, failureType, "retry_operation")
		r.metrics.RecordRetryFailure(ctx, attempt, failureType, err, "retry_operation")
		r.metrics.RecordRetryDelay(ctx, delay, attempt, r.retryPolicy.GetPolicyName())

		// Wait for delay (respecting context cancellation)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}

		attempt++
	}
}

func (r *DefaultRetryWithCircuitBreaker) GetRetryPolicy() RetryPolicy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.retryPolicy
}

func (r *DefaultRetryWithCircuitBreaker) GetCircuitBreaker() CircuitBreaker {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.circuitBreaker
}

func (r *DefaultRetryWithCircuitBreaker) GetMetrics() RetryMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.metrics
}

func (r *DefaultRetryWithCircuitBreaker) UpdatePolicy(policy RetryPolicy) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if policy == nil {
		return errors.New("policy cannot be nil")
	}

	oldPolicy := r.retryPolicy.GetPolicyName()
	r.retryPolicy = policy
	newPolicy := policy.GetPolicyName()

	// Record policy change
	r.metrics.RecordPolicyChange(context.Background(), oldPolicy, newPolicy, "retry_operation")

	return nil
}

// NewAdaptiveRetryPolicy creates an adaptive retry policy based on circuit breaker state.
func NewAdaptiveRetryPolicy(config RetryPolicyConfig, circuitState CircuitBreakerState) RetryPolicy {
	// Adjust policy based on circuit breaker state
	adaptiveConfig := config

	switch circuitState {
	case CircuitBreakerStateOpen:
		// More aggressive backoff when circuit is open
		adaptiveConfig.MaxAttempts = 1
		adaptiveConfig.BaseDelay = config.BaseDelay * 2
	case CircuitBreakerStateHalfOpen:
		// Conservative retries when half-open
		adaptiveConfig.MaxAttempts = config.MaxAttempts / 2
		if adaptiveConfig.MaxAttempts < 1 {
			adaptiveConfig.MaxAttempts = 1
		}
		adaptiveConfig.BaseDelay = config.BaseDelay * 3
	case CircuitBreakerStateClosed:
		// Normal retry behavior when closed
		adaptiveConfig = config
	}

	return NewExponentialBackoffPolicy(adaptiveConfig)
}
