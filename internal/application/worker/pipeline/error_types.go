package pipeline

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ProcessingErrorCategory represents the category of an error in the pipeline.
type ProcessingErrorCategory string

const (
	ProcessingTransientError   ProcessingErrorCategory = "processing_transient"
	ProcessingPermanentError   ProcessingErrorCategory = "processing_permanent"
	ProcessingResourceError    ProcessingErrorCategory = "processing_resource"
	ProcessingIntegrationError ProcessingErrorCategory = "processing_integration"
	ProcessingPipelineError    ProcessingErrorCategory = "processing_pipeline"
)

// ProcessingErrorCode represents specific error codes for categorization.
type ProcessingErrorCode string

const (
	// Transient errors.
	ProcessingNetworkTimeout          ProcessingErrorCode = "processing_network_timeout"
	ProcessingServiceUnavailable      ProcessingErrorCode = "processing_service_unavailable"
	ProcessingRateLimitExceeded       ProcessingErrorCode = "processing_rate_limit_exceeded"
	ProcessingContextCancelled        ProcessingErrorCode = "processing_context_cancelled"
	ProcessingContextDeadlineExceeded ProcessingErrorCode = "processing_context_deadline_exceeded"

	// Permanent errors.
	ProcessingInvalidConfiguration ProcessingErrorCode = "processing_invalid_configuration"
	ProcessingAuthenticationFailed ProcessingErrorCode = "processing_authentication_failed"
	ProcessingAuthorizationDenied  ProcessingErrorCode = "processing_authorization_denied"
	ProcessingDataCorruption       ProcessingErrorCode = "processing_data_corruption"

	// Resource errors.
	ProcessingMemoryLimitExceededError ProcessingErrorCode = "processing_memory_limit_exceeded"
	ProcessingDiskFull                 ProcessingErrorCode = "processing_disk_full"
	ProcessingCPULimitExceeded         ProcessingErrorCode = "processing_cpu_limit_exceeded"

	// Integration errors.
	ProcessingComponentFailure      ProcessingErrorCode = "processing_component_failure"
	ProcessingDependencyUnavailable ProcessingErrorCode = "processing_dependency_unavailable"
	ProcessingAPIContractViolation  ProcessingErrorCode = "processing_api_contract_violation"

	// Pipeline errors.
	ProcessingStageFailure    ProcessingErrorCode = "processing_stage_failure"
	ProcessingPipelineTimeout ProcessingErrorCode = "processing_pipeline_timeout"
)

// ProcessingRetryStrategy defines the retry approach.
type ProcessingRetryStrategy string

const (
	ProcessingExponentialBackoff ProcessingRetryStrategy = "processing_exponential"
	ProcessingLinearBackoff      ProcessingRetryStrategy = "processing_linear"
	ProcessingFixedBackoff       ProcessingRetryStrategy = "processing_fixed"
)

// EnhancedPipelineError represents a structured error with metadata.
type EnhancedPipelineError struct {
	Code          ProcessingErrorCode
	Category      ProcessingErrorCategory
	Message       string
	Retryable     bool
	RetryCount    int
	Component     string
	Operation     string
	Parameters    map[string]interface{}
	CorrelationID string
	WrappedError  error
}

func (e *EnhancedPipelineError) Error() string {
	return e.Message
}

func (e *EnhancedPipelineError) Unwrap() error {
	return e.WrappedError
}

// ProcessingErrorClassifier provides error classification capabilities.
type ProcessingErrorClassifier struct{}

// ClassifyError determines the category and retry recommendation for an error.
func (ec *ProcessingErrorClassifier) ClassifyError(err error) (*EnhancedPipelineError, bool) {
	var enhancedErr *EnhancedPipelineError
	if errors.As(err, &enhancedErr) {
		return enhancedErr, true
	}

	// Default classification logic
	switch {
	case errors.Is(err, context.Canceled):
		return &EnhancedPipelineError{
			Code:         ProcessingContextCancelled,
			Category:     ProcessingTransientError,
			Message:      err.Error(),
			Retryable:    false,
			WrappedError: err,
		}, true
	case errors.Is(err, context.DeadlineExceeded):
		return &EnhancedPipelineError{
			Code:         ProcessingContextDeadlineExceeded,
			Category:     ProcessingTransientError,
			Message:      err.Error(),
			Retryable:    true,
			WrappedError: err,
		}, true
	case errors.Is(err, ErrMemoryLimitExceeded):
		return &EnhancedPipelineError{
			Code:         ProcessingMemoryLimitExceededError,
			Category:     ProcessingResourceError,
			Message:      err.Error(),
			Retryable:    true,
			WrappedError: err,
		}, true
	default:
		// Try to classify based on error message patterns
		msg := err.Error()
		switch {
		case containsAny(msg, []string{"timeout", "timed out"}):
			return &EnhancedPipelineError{
				Code:         ProcessingNetworkTimeout,
				Category:     ProcessingTransientError,
				Message:      msg,
				Retryable:    true,
				WrappedError: err,
			}, true
		case containsAny(msg, []string{"unavailable", "not found"}):
			return &EnhancedPipelineError{
				Code:         ProcessingServiceUnavailable,
				Category:     ProcessingTransientError,
				Message:      msg,
				Retryable:    true,
				WrappedError: err,
			}, true
		case containsAny(msg, []string{"authentication", "unauthorized"}):
			return &EnhancedPipelineError{
				Code:         ProcessingAuthenticationFailed,
				Category:     ProcessingPermanentError,
				Message:      msg,
				Retryable:    false,
				WrappedError: err,
			}, true
		default:
			return nil, false
		}
	}
}

func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) != -1
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ProcessingCircuitBreakerState represents the state of a circuit breaker.
type ProcessingCircuitBreakerState int

const (
	ProcessingClosed ProcessingCircuitBreakerState = iota
	ProcessingOpen
	ProcessingHalfOpen
)

// ProcessingCircuitBreaker implements the circuit breaker pattern.
type ProcessingCircuitBreaker struct {
	name             string
	state            ProcessingCircuitBreakerState
	failureCount     int
	successCount     int
	lastFailure      time.Time
	mutex            sync.Mutex
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	meter            metric.Meter
	stateCounter     metric.Int64Counter
}

// NewProcessingCircuitBreaker creates a new circuit breaker.
func NewProcessingCircuitBreaker(
	name string,
	failureThreshold, successThreshold int,
	timeout time.Duration,
	meter metric.Meter,
) *ProcessingCircuitBreaker {
	var stateCounter metric.Int64Counter
	if meter != nil {
		stateCounter, _ = meter.Int64Counter(
			"processing_circuit_breaker_state_changes",
			metric.WithDescription("Number of state changes in processing circuit breaker"),
		)
	}

	return &ProcessingCircuitBreaker{
		name:             name,
		state:            ProcessingClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
		meter:            meter,
		stateCounter:     stateCounter,
	}
}

// Execute runs the given function with circuit breaker protection.
func (cb *ProcessingCircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	cb.mutex.Lock()

	// Check if circuit is open
	if cb.state == ProcessingOpen {
		// Check if timeout has passed
		if time.Since(cb.lastFailure) >= cb.timeout {
			cb.state = ProcessingHalfOpen
			if cb.stateCounter != nil {
				cb.stateCounter.Add(ctx, 1, metric.WithAttributes(
					attribute.String("circuit_breaker", cb.name),
					attribute.String("state", "processing_half_open"),
				))
			}
		} else {
			cb.mutex.Unlock()
			return fmt.Errorf("processing circuit breaker %s is open", cb.name)
		}
	}

	cb.mutex.Unlock()

	// Execute function
	err := fn()

	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if err != nil {
		// Handle failure
		cb.failureCount++
		cb.lastFailure = time.Now()

		if cb.failureCount >= cb.failureThreshold && cb.state == ProcessingClosed {
			cb.state = ProcessingOpen
			if cb.stateCounter != nil {
				cb.stateCounter.Add(ctx, 1, metric.WithAttributes(
					attribute.String("circuit_breaker", cb.name),
					attribute.String("state", "processing_open"),
				))
			}
		}

		return err
	}

	// Handle success
	switch cb.state {
	case ProcessingHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.state = ProcessingClosed
			cb.failureCount = 0
			cb.successCount = 0
			if cb.stateCounter != nil {
				cb.stateCounter.Add(ctx, 1, metric.WithAttributes(
					attribute.String("circuit_breaker", cb.name),
					attribute.String("state", "processing_closed"),
				))
			}
		}
	case ProcessingClosed:
		cb.failureCount = 0
	case ProcessingOpen:
		// Reset failure count when transitioning from open to closed via half-open
		cb.failureCount = 0
		cb.successCount = 0
	}

	return nil
}

// GetState returns the current state of the circuit breaker.
func (cb *ProcessingCircuitBreaker) GetState() ProcessingCircuitBreakerState {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	return cb.state
}

// ProcessingRetryConfig holds configuration for retry logic.
type ProcessingRetryConfig struct {
	MaxRetries  int
	Strategy    ProcessingRetryStrategy
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      bool
	RetryBudget int // Maximum number of retries allowed across all operations
}

// ProcessingRetryManager handles retry logic with exponential backoff.
type ProcessingRetryManager struct {
	config ProcessingRetryConfig
	budget int
	mutex  sync.Mutex
}

// NewProcessingRetryManager creates a new retry manager.
func NewProcessingRetryManager(config ProcessingRetryConfig) *ProcessingRetryManager {
	return &ProcessingRetryManager{
		config: config,
		budget: config.RetryBudget,
	}
}

// ExecuteWithRetry runs the given function with retry logic.
func (rm *ProcessingRetryManager) ExecuteWithRetry(ctx context.Context, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= rm.config.MaxRetries; attempt++ {
		// Check retry budget
		rm.mutex.Lock()
		if rm.budget <= 0 && attempt > 0 {
			rm.mutex.Unlock()
			return fmt.Errorf("processing retry budget exhausted: %w", lastErr)
		}
		rm.budget--
		rm.mutex.Unlock()

		// Try operation
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		classifier := &ProcessingErrorClassifier{}
		if enhancedErr, ok := classifier.ClassifyError(err); !ok || !enhancedErr.Retryable {
			return err
		}

		// Check context
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Calculate delay
		delay := rm.calculateDelay(attempt)

		// Add jitter if enabled
		if rm.config.Jitter {
			delay = rm.addJitter(delay)
		}

		// Wait or context cancellation
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("processing max retries exceeded: %w", lastErr)
}

func (rm *ProcessingRetryManager) calculateDelay(attempt int) time.Duration {
	switch rm.config.Strategy {
	case ProcessingExponentialBackoff:
		// Prevent integer overflow by checking bounds
		if attempt < 0 || attempt >= 63 {
			return rm.config.MaxDelay
		}
		// Use safe bit shift with explicit uint conversion after bounds check
		delay := rm.config.BaseDelay * time.Duration(1<<uint(attempt))
		if delay > rm.config.MaxDelay {
			return rm.config.MaxDelay
		}
		return delay
	case ProcessingLinearBackoff:
		delay := rm.config.BaseDelay * time.Duration(attempt+1)
		if delay > rm.config.MaxDelay {
			return rm.config.MaxDelay
		}
		return delay
	case ProcessingFixedBackoff:
		return rm.config.BaseDelay
	default:
		return rm.config.BaseDelay
	}
}

func (rm *ProcessingRetryManager) addJitter(delay time.Duration) time.Duration {
	// Use crypto/rand for security-sensitive randomness
	n, err := rand.Int(rand.Reader, big.NewInt(int64(delay)/2))
	if err != nil {
		// Fallback to time-based jitter if crypto/rand fails
		return delay
	}
	return delay/2 + time.Duration(n.Int64())
}

// ProcessingFallbackStrategy defines different fallback approaches.
type ProcessingFallbackStrategy int

const (
	ProcessingStreamingToBatch ProcessingFallbackStrategy = iota
	ProcessingBatchToManual
	ProcessingComponentSubstitution
	ProcessingPartialRecovery
)

// ProcessingFallbackManager handles fallback mechanisms.
type ProcessingFallbackManager struct {
	strategies map[string]ProcessingFallbackStrategy
	logger     *slog.Logger
}

// NewProcessingFallbackManager creates a new fallback manager.
func NewProcessingFallbackManager(logger *slog.Logger) *ProcessingFallbackManager {
	return &ProcessingFallbackManager{
		strategies: make(map[string]ProcessingFallbackStrategy),
		logger:     logger,
	}
}

// RegisterStrategy registers a fallback strategy for a component.
func (fm *ProcessingFallbackManager) RegisterStrategy(component string, strategy ProcessingFallbackStrategy) {
	fm.strategies[component] = strategy
}

// ExecuteWithFallback runs the primary function with fallback options.
func (fm *ProcessingFallbackManager) ExecuteWithFallback(
	ctx context.Context,
	component string,
	primaryFn func() error,
	fallbackFns map[ProcessingFallbackStrategy]func() error,
) error {
	err := primaryFn()
	if err == nil {
		return nil
	}

	// Classify error
	classifier := &ProcessingErrorClassifier{}
	enhancedErr, _ := classifier.ClassifyError(err)

	// Log error with context
	fm.logger.ErrorContext(ctx, "Processing primary operation failed",
		"component", component,
		"error", err.Error(),
		"category", enhancedErr.Category,
		"code", enhancedErr.Code,
	)

	// Determine fallback strategy
	strategy, exists := fm.strategies[component]
	if !exists {
		return err
	}

	// Execute fallback
	fallbackFn, hasFallback := fallbackFns[strategy]
	if !hasFallback {
		return err
	}

	fallbackErr := fallbackFn()
	if fallbackErr != nil {
		return fmt.Errorf("processing primary failed: %w, fallback failed: %w", err, fallbackErr)
	}

	fm.logger.InfoContext(ctx, "Processing fallback operation succeeded", "component", component, "strategy", strategy)
	return nil
}

// ProcessingErrorContext provides rich context for errors.
type ProcessingErrorContext struct {
	Component      string
	Operation      string
	Parameters     map[string]interface{}
	CorrelationID  string
	Timestamp      time.Time
	DiagnosticInfo map[string]interface{}
}

// NewProcessingErrorContext creates a new error context.
func NewProcessingErrorContext(component, operation string, parameters map[string]interface{}) *ProcessingErrorContext {
	return &ProcessingErrorContext{
		Component:      component,
		Operation:      operation,
		Parameters:     parameters,
		Timestamp:      time.Now(),
		DiagnosticInfo: make(map[string]interface{}),
	}
}

// WithCorrelationID adds a correlation ID to the error context.
func (ec *ProcessingErrorContext) WithCorrelationID(id string) *ProcessingErrorContext {
	ec.CorrelationID = id
	return ec
}

// WithDiagnosticInfo adds diagnostic information to the error context.
func (ec *ProcessingErrorContext) WithDiagnosticInfo(info map[string]interface{}) *ProcessingErrorContext {
	// Filter out sensitive information
	for key, value := range info {
		if !isSensitive(key) {
			ec.DiagnosticInfo[key] = value
		}
	}
	return ec
}

func isSensitive(key string) bool {
	sensitiveKeys := []string{"password", "token", "secret", "key", "credential"}
	for _, sensitiveKey := range sensitiveKeys {
		if contains(key, sensitiveKey) {
			return true
		}
	}
	return false
}

// ProcessingErrorAggregator collects and aggregates errors.
type ProcessingErrorAggregator struct {
	errors []error
	mutex  sync.Mutex
}

// NewProcessingErrorAggregator creates a new error aggregator.
func NewProcessingErrorAggregator() *ProcessingErrorAggregator {
	return &ProcessingErrorAggregator{
		errors: make([]error, 0),
	}
}

// AddError adds an error to the aggregator.
func (ea *ProcessingErrorAggregator) AddError(err error) {
	ea.mutex.Lock()
	defer ea.mutex.Unlock()
	ea.errors = append(ea.errors, err)
}

// GetErrors returns all aggregated errors.
func (ea *ProcessingErrorAggregator) GetErrors() []error {
	ea.mutex.Lock()
	defer ea.mutex.Unlock()
	return ea.errors
}

// ClearErrors clears all aggregated errors.
func (ea *ProcessingErrorAggregator) ClearErrors() {
	ea.mutex.Lock()
	defer ea.mutex.Unlock()
	ea.errors = make([]error, 0)
}

// HasErrors checks if there are any aggregated errors.
func (ea *ProcessingErrorAggregator) HasErrors() bool {
	ea.mutex.Lock()
	defer ea.mutex.Unlock()
	return len(ea.errors) > 0
}

// ErrorCount returns the number of aggregated errors.
func (ea *ProcessingErrorAggregator) ErrorCount() int {
	ea.mutex.Lock()
	defer ea.mutex.Unlock()
	return len(ea.errors)
}

// CreateEnhancedPipelineError creates a new enhanced error with context.
func CreateEnhancedPipelineError(
	ctx *ProcessingErrorContext,
	code ProcessingErrorCode,
	category ProcessingErrorCategory,
	message string,
	retryable bool,
	wrappedError error,
) *EnhancedPipelineError {
	return &EnhancedPipelineError{
		Code:          code,
		Category:      category,
		Message:       message,
		Retryable:     retryable,
		Component:     ctx.Component,
		Operation:     ctx.Operation,
		Parameters:    ctx.Parameters,
		CorrelationID: ctx.CorrelationID,
		WrappedError:  wrappedError,
	}
}

// WrapPipelineError wraps an existing error with enhanced context.
func WrapPipelineError(ctx *ProcessingErrorContext, err error, message string) error {
	classifier := &ProcessingErrorClassifier{}
	enhancedErr, ok := classifier.ClassifyError(err)

	if !ok {
		// If we can't classify it, create a basic enhanced error
		enhancedErr = &EnhancedPipelineError{
			Code:          ProcessingDataCorruption,
			Category:      ProcessingPermanentError,
			Message:       err.Error(),
			Retryable:     false,
			Component:     ctx.Component,
			Operation:     ctx.Operation,
			Parameters:    ctx.Parameters,
			CorrelationID: ctx.CorrelationID,
			WrappedError:  err,
		}
	}

	// Create new error with additional context
	return &EnhancedPipelineError{
		Code:          enhancedErr.Code,
		Category:      enhancedErr.Category,
		Message:       message + ": " + enhancedErr.Message,
		Retryable:     enhancedErr.Retryable,
		RetryCount:    enhancedErr.RetryCount + 1,
		Component:     ctx.Component,
		Operation:     ctx.Operation,
		Parameters:    ctx.Parameters,
		CorrelationID: ctx.CorrelationID,
		WrappedError:  err,
	}
}
