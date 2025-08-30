package service

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// circuitState represents the current state of the circuit breaker.
type circuitState int32

const (
	stateClosed circuitState = iota
	stateOpen
	stateHalfOpen
)

// reliableCircuitBreaker implements ReliableCircuitBreaker interface.
type reliableCircuitBreaker struct {
	failureThreshold     int
	recoveryTimeout      time.Duration
	state                int32 // atomic circuitState
	failureCount         int64 // atomic
	consecutiveFailures  int64 // atomic
	consecutiveSuccesses int64 // atomic
	lastFailureTime      int64 // atomic unix nano
	lastSuccessTime      int64 // atomic unix nano
	totalCalls           int64 // atomic
	successfulCalls      int64 // atomic
	failedCalls          int64 // atomic
	rejectedCalls        int64 // atomic
	stateTransitions     int64 // atomic
	timeInOpenState      int64 // atomic nanoseconds
	timeInHalfOpenState  int64 // atomic nanoseconds
	lastStateChange      int64 // atomic unix nano
	mu                   sync.RWMutex
}

// NewReliableCircuitBreaker creates a new circuit breaker with the specified configuration.
func NewReliableCircuitBreaker(failureThreshold int, recoveryTimeout time.Duration) ReliableCircuitBreaker {
	return &reliableCircuitBreaker{
		failureThreshold: failureThreshold,
		recoveryTimeout:  recoveryTimeout,
		state:            int32(stateClosed),
		lastStateChange:  time.Now().UnixNano(),
	}
}

// Execute wraps a function call with circuit breaker protection.
func (cb *reliableCircuitBreaker) Execute(fn func() error) error {
	atomic.AddInt64(&cb.totalCalls, 1)

	// Check if circuit is open
	if cb.IsOpen() {
		// Check if we should transition to half-open
		if cb.shouldAttemptRecovery() {
			cb.transitionToHalfOpen()
		} else {
			atomic.AddInt64(&cb.rejectedCalls, 1)
			return errors.New("circuit breaker is open")
		}
	}

	// Execute the function
	start := time.Now()
	err := fn()
	duration := time.Since(start)

	// Update statistics
	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess(duration)
	return nil
}

// onFailure handles a failed function execution.
func (cb *reliableCircuitBreaker) onFailure() {
	atomic.AddInt64(&cb.failedCalls, 1)
	atomic.AddInt64(&cb.consecutiveFailures, 1)
	atomic.StoreInt64(&cb.consecutiveSuccesses, 0)
	atomic.StoreInt64(&cb.lastFailureTime, time.Now().UnixNano())

	currentState := circuitState(atomic.LoadInt32(&cb.state))
	consecutiveFailures := atomic.LoadInt64(&cb.consecutiveFailures)

	if currentState == stateClosed && consecutiveFailures >= int64(cb.failureThreshold) {
		cb.transitionToOpen()
	} else if currentState == stateHalfOpen {
		// Failed during recovery attempt - go back to open
		cb.transitionToOpen()
	}
}

// onSuccess handles a successful function execution.
func (cb *reliableCircuitBreaker) onSuccess(duration time.Duration) {
	atomic.AddInt64(&cb.successfulCalls, 1)
	atomic.AddInt64(&cb.consecutiveSuccesses, 1)
	atomic.StoreInt64(&cb.consecutiveFailures, 0)
	atomic.StoreInt64(&cb.lastSuccessTime, time.Now().UnixNano())

	currentState := circuitState(atomic.LoadInt32(&cb.state))
	if currentState == stateHalfOpen {
		// Successful recovery attempt - close the circuit
		cb.transitionToClosed()
	}
}

// shouldAttemptRecovery determines if the circuit should attempt recovery.
func (cb *reliableCircuitBreaker) shouldAttemptRecovery() bool {
	lastStateChange := atomic.LoadInt64(&cb.lastStateChange)
	return time.Since(time.Unix(0, lastStateChange)) >= cb.recoveryTimeout
}

// transitionToOpen transitions the circuit to open state.
func (cb *reliableCircuitBreaker) transitionToOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if atomic.CompareAndSwapInt32(&cb.state, int32(stateClosed), int32(stateOpen)) ||
		atomic.CompareAndSwapInt32(&cb.state, int32(stateHalfOpen), int32(stateOpen)) {
		now := time.Now().UnixNano()
		atomic.StoreInt64(&cb.lastStateChange, now)
		atomic.AddInt64(&cb.stateTransitions, 1)
	}
}

// transitionToHalfOpen transitions the circuit to half-open state.
func (cb *reliableCircuitBreaker) transitionToHalfOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if atomic.CompareAndSwapInt32(&cb.state, int32(stateOpen), int32(stateHalfOpen)) {
		now := time.Now().UnixNano()
		lastChange := atomic.LoadInt64(&cb.lastStateChange)
		timeInOpen := now - lastChange
		atomic.AddInt64(&cb.timeInOpenState, timeInOpen)
		atomic.StoreInt64(&cb.lastStateChange, now)
		atomic.AddInt64(&cb.stateTransitions, 1)
	}
}

// transitionToClosed transitions the circuit to closed state.
func (cb *reliableCircuitBreaker) transitionToClosed() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if atomic.CompareAndSwapInt32(&cb.state, int32(stateHalfOpen), int32(stateClosed)) {
		now := time.Now().UnixNano()
		lastChange := atomic.LoadInt64(&cb.lastStateChange)
		timeInHalfOpen := now - lastChange
		atomic.AddInt64(&cb.timeInHalfOpenState, timeInHalfOpen)
		atomic.StoreInt64(&cb.lastStateChange, now)
		atomic.AddInt64(&cb.stateTransitions, 1)
	}
}

// IsOpen returns true if circuit is currently open.
func (cb *reliableCircuitBreaker) IsOpen() bool {
	return circuitState(atomic.LoadInt32(&cb.state)) == stateOpen
}

// IsHalfOpen returns true if circuit is in half-open state.
func (cb *reliableCircuitBreaker) IsHalfOpen() bool {
	return circuitState(atomic.LoadInt32(&cb.state)) == stateHalfOpen
}

// IsClosed returns true if circuit is closed.
func (cb *reliableCircuitBreaker) IsClosed() bool {
	return circuitState(atomic.LoadInt32(&cb.state)) == stateClosed
}

// GetState returns current circuit state as string.
func (cb *reliableCircuitBreaker) GetState() string {
	state := circuitState(atomic.LoadInt32(&cb.state))
	switch state {
	case stateClosed:
		return "closed"
	case stateOpen:
		return "open"
	case stateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ForceOpen manually opens the circuit.
func (cb *reliableCircuitBreaker) ForceOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := atomic.LoadInt32(&cb.state)
	atomic.StoreInt32(&cb.state, int32(stateOpen))
	if oldState != int32(stateOpen) {
		atomic.StoreInt64(&cb.lastStateChange, time.Now().UnixNano())
		atomic.AddInt64(&cb.stateTransitions, 1)
	}
}

// ForceClose manually closes the circuit.
func (cb *reliableCircuitBreaker) ForceClose() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := atomic.LoadInt32(&cb.state)
	atomic.StoreInt32(&cb.state, int32(stateClosed))
	if oldState != int32(stateClosed) {
		atomic.StoreInt64(&cb.lastStateChange, time.Now().UnixNano())
		atomic.AddInt64(&cb.stateTransitions, 1)
	}
}

// Reset resets all circuit breaker statistics and closes the circuit.
func (cb *reliableCircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.StoreInt32(&cb.state, int32(stateClosed))
	atomic.StoreInt64(&cb.failureCount, 0)
	atomic.StoreInt64(&cb.consecutiveFailures, 0)
	atomic.StoreInt64(&cb.consecutiveSuccesses, 0)
	atomic.StoreInt64(&cb.lastFailureTime, 0)
	atomic.StoreInt64(&cb.lastSuccessTime, 0)
	atomic.StoreInt64(&cb.totalCalls, 0)
	atomic.StoreInt64(&cb.successfulCalls, 0)
	atomic.StoreInt64(&cb.failedCalls, 0)
	atomic.StoreInt64(&cb.rejectedCalls, 0)
	atomic.StoreInt64(&cb.stateTransitions, 0)
	atomic.StoreInt64(&cb.timeInOpenState, 0)
	atomic.StoreInt64(&cb.timeInHalfOpenState, 0)
	atomic.StoreInt64(&cb.lastStateChange, time.Now().UnixNano())
}

// GetStatistics returns comprehensive statistics about circuit breaker operation.
func (cb *reliableCircuitBreaker) GetStatistics() CircuitBreakerStatistics {
	totalCalls := atomic.LoadInt64(&cb.totalCalls)
	successfulCalls := atomic.LoadInt64(&cb.successfulCalls)
	failedCalls := atomic.LoadInt64(&cb.failedCalls)
	rejectedCalls := atomic.LoadInt64(&cb.rejectedCalls)

	var successRate, failureRate, rejectionRate float64
	if totalCalls > 0 {
		successRate = float64(successfulCalls) / float64(totalCalls)
		failureRate = float64(failedCalls) / float64(totalCalls)
		rejectionRate = float64(rejectedCalls) / float64(totalCalls)
	}

	return CircuitBreakerStatistics{
		TotalCalls:           totalCalls,
		SuccessfulCalls:      successfulCalls,
		FailedCalls:          failedCalls,
		RejectedCalls:        rejectedCalls,
		SuccessRate:          successRate,
		FailureRate:          failureRate,
		RejectionRate:        rejectionRate,
		AverageCallTime:      0, // Would need additional tracking for call timing
		LastFailureTime:      time.Unix(0, atomic.LoadInt64(&cb.lastFailureTime)),
		LastSuccessTime:      time.Unix(0, atomic.LoadInt64(&cb.lastSuccessTime)),
		StateTransitions:     atomic.LoadInt64(&cb.stateTransitions),
		TimeInOpenState:      time.Duration(atomic.LoadInt64(&cb.timeInOpenState)),
		TimeInHalfOpenState:  time.Duration(atomic.LoadInt64(&cb.timeInHalfOpenState)),
		ConsecutiveFailures:  atomic.LoadInt64(&cb.consecutiveFailures),
		ConsecutiveSuccesses: atomic.LoadInt64(&cb.consecutiveSuccesses),
	}
}
