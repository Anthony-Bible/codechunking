package queue

import (
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"time"
)

// Enhanced error handling and recovery mechanisms for batch queue management

// QueueError represents different types of queue errors with recovery strategies.
type QueueError struct {
	Type           QueueErrorType
	Code           string
	Message        string
	RequestID      string
	OriginalError  error
	Retryable      bool
	RetryAfter     *time.Duration
	RecoveryAction string
	Context        map[string]interface{}
	Timestamp      time.Time
}

// Error implements the error interface.
func (qe *QueueError) Error() string {
	if qe.RequestID != "" {
		return fmt.Sprintf("[%s:%s] %s (RequestID: %s)", qe.Type, qe.Code, qe.Message, qe.RequestID)
	}
	return fmt.Sprintf("[%s:%s] %s", qe.Type, qe.Code, qe.Message)
}

// Unwrap returns the underlying error.
func (qe *QueueError) Unwrap() error {
	return qe.OriginalError
}

// QueueErrorType represents different categories of errors.
type QueueErrorType string

const (
	ErrorTypeValidation    QueueErrorType = "validation"
	ErrorTypeCapacity      QueueErrorType = "capacity"
	ErrorTypeProcessing    QueueErrorType = "processing"
	ErrorTypeConfiguration QueueErrorType = "configuration"
	ErrorTypeResource      QueueErrorType = "resource"
	ErrorTypeNetwork       QueueErrorType = "network"
	ErrorTypeTimeout       QueueErrorType = "timeout"
	ErrorTypeConcurrency   QueueErrorType = "concurrency"
	ErrorTypeInternal      QueueErrorType = "internal"
)

// QueueErrorHandler handles different types of queue errors with appropriate recovery strategies.
type QueueErrorHandler struct {
	retryStrategies map[QueueErrorType]*RetryStrategy
	circuitBreaker  *CircuitBreaker
	alertManager    AlertManager
}

// RetryStrategy defines how to handle retries for different error types.
type RetryStrategy struct {
	MaxRetries    int
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	Multiplier    float64
	Jitter        bool
	RetryableFunc func(error) bool
}

// CircuitBreaker prevents cascading failures by monitoring error rates.
type CircuitBreaker struct {
	name             string
	maxFailures      int
	timeout          time.Duration
	failureCount     int
	lastFailTime     time.Time
	state            CircuitBreakerState
	successThreshold int
}

// CircuitBreakerState represents the circuit breaker states.
type CircuitBreakerState string

const (
	StateOpen     CircuitBreakerState = "open"
	StateHalfOpen CircuitBreakerState = "half_open"
	StateClosed   CircuitBreakerState = "closed"
)

// AlertManager defines interface for alerting on critical errors.
type AlertManager interface {
	SendAlert(ctx context.Context, alert *Alert) error
}

// Alert represents an error alert.
type Alert struct {
	Severity  AlertSeverity
	Title     string
	Message   string
	ErrorType QueueErrorType
	RequestID string
	Metadata  map[string]interface{}
	Timestamp time.Time
}

// AlertSeverity represents the severity of alerts.
type AlertSeverity string

const (
	SeverityCritical AlertSeverity = "critical"
	SeverityHigh     AlertSeverity = "high"
	SeverityMedium   AlertSeverity = "medium"
	SeverityLow      AlertSeverity = "low"
)

// NewQueueErrorHandler creates a new error handler with default strategies.
func NewQueueErrorHandler(alertManager AlertManager) *QueueErrorHandler {
	return &QueueErrorHandler{
		retryStrategies: getDefaultRetryStrategies(),
		circuitBreaker:  NewCircuitBreaker("queue_processor", 10, 60*time.Second, 3),
		alertManager:    alertManager,
	}
}

// getDefaultRetryStrategies returns default retry strategies for different error types.
func getDefaultRetryStrategies() map[QueueErrorType]*RetryStrategy {
	return map[QueueErrorType]*RetryStrategy{
		ErrorTypeValidation: {
			MaxRetries:    0, // Don't retry validation errors
			RetryableFunc: func(err error) bool { return false },
		},
		ErrorTypeCapacity: {
			MaxRetries:    5,
			BaseDelay:     1 * time.Second,
			MaxDelay:      30 * time.Second,
			Multiplier:    2.0,
			Jitter:        true,
			RetryableFunc: func(err error) bool { return true },
		},
		ErrorTypeProcessing: {
			MaxRetries:    3,
			BaseDelay:     500 * time.Millisecond,
			MaxDelay:      10 * time.Second,
			Multiplier:    1.5,
			Jitter:        true,
			RetryableFunc: func(err error) bool { return true },
		},
		ErrorTypeNetwork: {
			MaxRetries:    5,
			BaseDelay:     2 * time.Second,
			MaxDelay:      60 * time.Second,
			Multiplier:    2.0,
			Jitter:        true,
			RetryableFunc: func(err error) bool { return true },
		},
		ErrorTypeTimeout: {
			MaxRetries:    3,
			BaseDelay:     5 * time.Second,
			MaxDelay:      30 * time.Second,
			Multiplier:    2.0,
			Jitter:        false,
			RetryableFunc: func(err error) bool { return true },
		},
		ErrorTypeConfiguration: {
			MaxRetries:    0, // Don't retry configuration errors
			RetryableFunc: func(err error) bool { return false },
		},
		ErrorTypeResource: {
			MaxRetries:    2,
			BaseDelay:     10 * time.Second,
			MaxDelay:      60 * time.Second,
			Multiplier:    2.0,
			Jitter:        true,
			RetryableFunc: func(err error) bool { return true },
		},
		ErrorTypeConcurrency: {
			MaxRetries:    3,
			BaseDelay:     1 * time.Second,
			MaxDelay:      10 * time.Second,
			Multiplier:    1.5,
			Jitter:        true,
			RetryableFunc: func(err error) bool { return true },
		},
		ErrorTypeInternal: {
			MaxRetries:    1,
			BaseDelay:     2 * time.Second,
			MaxDelay:      10 * time.Second,
			Multiplier:    2.0,
			Jitter:        true,
			RetryableFunc: func(err error) bool { return true },
		},
	}
}

// HandleError processes an error and determines the appropriate recovery action.
func (h *QueueErrorHandler) HandleError(ctx context.Context, err error) (*ErrorRecovery, error) {
	queueErr := h.wrapError(err)

	// Check circuit breaker
	if !h.circuitBreaker.CanExecute() {
		return &ErrorRecovery{
			Action:   RecoveryActionCircuitBreaker,
			WaitTime: h.circuitBreaker.timeout,
			Message:  "Circuit breaker is open, temporarily blocking requests",
		}, queueErr
	}

	// Determine recovery strategy
	strategy, exists := h.retryStrategies[queueErr.Type]
	if !exists {
		strategy = h.retryStrategies[ErrorTypeInternal]
	}

	recovery := &ErrorRecovery{
		Action:     h.determineRecoveryAction(queueErr),
		Retryable:  strategy.RetryableFunc(err),
		MaxRetries: strategy.MaxRetries,
		RetryDelay: strategy.BaseDelay,
		Message:    fmt.Sprintf("Error handled: %s", queueErr.Message),
	}

	// Send alert if necessary
	if h.shouldSendAlert(queueErr) {
		alert := &Alert{
			Severity:  h.determineSeverity(queueErr),
			Title:     fmt.Sprintf("Queue Error: %s", queueErr.Type),
			Message:   queueErr.Message,
			ErrorType: queueErr.Type,
			RequestID: queueErr.RequestID,
			Timestamp: time.Now(),
		}

		if h.alertManager != nil {
			_ = h.alertManager.SendAlert(ctx, alert) // Don't fail on alert errors
		}
	}

	// Record failure for circuit breaker
	h.circuitBreaker.RecordFailure()

	return recovery, queueErr
}

// RecordSuccess records a successful operation for circuit breaker.
func (h *QueueErrorHandler) RecordSuccess() {
	h.circuitBreaker.RecordSuccess()
}

// ErrorRecovery contains information about how to recover from an error.
type ErrorRecovery struct {
	Action     RecoveryAction
	Retryable  bool
	MaxRetries int
	RetryDelay time.Duration
	WaitTime   time.Duration
	Message    string
	Metadata   map[string]interface{}
}

// RecoveryAction defines different recovery strategies.
type RecoveryAction string

const (
	RecoveryActionRetry           RecoveryAction = "retry"
	RecoveryActionBackoff         RecoveryAction = "backoff"
	RecoveryActionCircuitBreaker  RecoveryAction = "circuit_breaker"
	RecoveryActionRejectRequest   RecoveryAction = "reject_request"
	RecoveryActionReduceBatchSize RecoveryAction = "reduce_batch_size"
	RecoveryActionIncreaseTimeout RecoveryAction = "increase_timeout"
	RecoveryActionFallback        RecoveryAction = "fallback"
)

// wrapError converts a standard error into a QueueError.
func (h *QueueErrorHandler) wrapError(err error) *QueueError {
	if err == nil {
		return nil
	}

	// If it's already a QueueError, return as-is
	var queueErr *QueueError
	if errors.As(err, &queueErr) {
		return queueErr
	}

	// If it's an outbound.QueueManagerError, convert it
	var outboundErr *outbound.QueueManagerError
	if errors.As(err, &outboundErr) {
		return &QueueError{
			Type:          QueueErrorType(outboundErr.Type),
			Code:          outboundErr.Code,
			Message:       outboundErr.Message,
			RequestID:     outboundErr.RequestID,
			OriginalError: outboundErr.Cause,
			Retryable:     outboundErr.Retryable,
			Timestamp:     time.Now(),
		}
	}

	// Default error conversion
	return &QueueError{
		Type:          ErrorTypeInternal,
		Code:          "unknown_error",
		Message:       err.Error(),
		OriginalError: err,
		Retryable:     true,
		Timestamp:     time.Now(),
	}
}

// determineRecoveryAction determines the best recovery action for an error.
func (h *QueueErrorHandler) determineRecoveryAction(queueErr *QueueError) RecoveryAction {
	switch queueErr.Type {
	case ErrorTypeValidation:
		return RecoveryActionRejectRequest
	case ErrorTypeCapacity:
		return RecoveryActionBackoff
	case ErrorTypeProcessing:
		return RecoveryActionRetry
	case ErrorTypeConfiguration:
		return RecoveryActionRejectRequest
	case ErrorTypeResource:
		return RecoveryActionReduceBatchSize
	case ErrorTypeNetwork:
		return RecoveryActionRetry
	case ErrorTypeTimeout:
		return RecoveryActionIncreaseTimeout
	case ErrorTypeConcurrency:
		return RecoveryActionBackoff
	case ErrorTypeInternal:
		return RecoveryActionRetry
	default:
		return RecoveryActionRetry
	}
}

// shouldSendAlert determines if an alert should be sent for this error.
func (h *QueueErrorHandler) shouldSendAlert(queueErr *QueueError) bool {
	switch queueErr.Type {
	case ErrorTypeValidation:
		return false // Don't alert on validation errors
	case ErrorTypeCapacity:
		return true // Alert on capacity issues
	case ErrorTypeProcessing:
		return true // Alert on processing failures
	case ErrorTypeConfiguration:
		return true // Alert on configuration errors
	case ErrorTypeResource:
		return true // Alert on resource issues
	case ErrorTypeNetwork:
		return true // Alert on network issues
	case ErrorTypeTimeout:
		return true // Alert on timeouts
	case ErrorTypeConcurrency:
		return true // Alert on concurrency issues
	case ErrorTypeInternal:
		return true // Alert on internal errors
	default:
		return true
	}
}

// determineSeverity determines the alert severity for an error.
func (h *QueueErrorHandler) determineSeverity(queueErr *QueueError) AlertSeverity {
	switch queueErr.Type {
	case ErrorTypeValidation:
		return SeverityLow
	case ErrorTypeCapacity:
		return SeverityHigh
	case ErrorTypeProcessing:
		return SeverityMedium
	case ErrorTypeConfiguration:
		return SeverityHigh
	case ErrorTypeResource:
		return SeverityCritical
	case ErrorTypeNetwork:
		return SeverityHigh
	case ErrorTypeTimeout:
		return SeverityMedium
	case ErrorTypeConcurrency:
		return SeverityMedium
	case ErrorTypeInternal:
		return SeverityMedium
	default:
		return SeverityMedium
	}
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(name string, maxFailures int, timeout time.Duration, successThreshold int) *CircuitBreaker {
	return &CircuitBreaker{
		name:             name,
		maxFailures:      maxFailures,
		timeout:          timeout,
		state:            StateClosed,
		successThreshold: successThreshold,
	}
}

// CanExecute checks if the circuit breaker allows execution.
func (cb *CircuitBreaker) CanExecute() bool {
	now := time.Now()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if now.Sub(cb.lastFailTime) > cb.timeout {
			cb.state = StateHalfOpen
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}

	return false
}

// RecordSuccess records a successful operation.
func (cb *CircuitBreaker) RecordSuccess() {
	switch cb.state {
	case StateHalfOpen:
		cb.failureCount = 0
		cb.state = StateClosed
	case StateClosed:
		cb.failureCount = 0
	case StateOpen:
		// Open state, no action needed on success
	}
}

// RecordFailure records a failed operation.
func (cb *CircuitBreaker) RecordFailure() {
	cb.failureCount++
	cb.lastFailTime = time.Now()

	if cb.failureCount >= cb.maxFailures {
		cb.state = StateOpen
	}
}

// GetState returns the current circuit breaker state.
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	return cb.state
}

// CreateValidationError creates a validation error.
func CreateValidationError(code, message, requestID string, originalErr error) *QueueError {
	return &QueueError{
		Type:          ErrorTypeValidation,
		Code:          code,
		Message:       message,
		RequestID:     requestID,
		OriginalError: originalErr,
		Retryable:     false,
		Timestamp:     time.Now(),
	}
}

// CreateCapacityError creates a capacity error.
func CreateCapacityError(message, requestID string, originalErr error) *QueueError {
	return &QueueError{
		Type:          ErrorTypeCapacity,
		Code:          "queue_full",
		Message:       message,
		RequestID:     requestID,
		OriginalError: originalErr,
		Retryable:     true,
		RetryAfter:    &[]time.Duration{5 * time.Second}[0],
		Timestamp:     time.Now(),
	}
}

// CreateProcessingError creates a processing error.
func CreateProcessingError(code, message string, originalErr error) *QueueError {
	return &QueueError{
		Type:          ErrorTypeProcessing,
		Code:          code,
		Message:       message,
		OriginalError: originalErr,
		Retryable:     true,
		Timestamp:     time.Now(),
	}
}
