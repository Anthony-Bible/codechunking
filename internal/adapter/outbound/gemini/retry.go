package gemini

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RetryConfig holds the retry configuration.
type RetryConfig struct {
	MaxAttempts int           // Maximum number of retry attempts
	BaseDelay   time.Duration // Base delay between retries
	MaxDelay    time.Duration // Maximum delay between retries
	Multiplier  float64       // Backoff multiplier
	Jitter      bool          // Whether to apply jitter to delays
}

// Validate validates the retry configuration.
func (r *RetryConfig) Validate() error {
	if r.MaxAttempts <= 0 {
		return errors.New("MaxAttempts must be positive")
	}
	if r.BaseDelay <= 0 {
		return errors.New("BaseDelay must be positive")
	}
	if r.Multiplier < 1.0 {
		return errors.New( //nolint:staticcheck // Test expects capitalized error message
			"Multiplier must be at least 1.0",
		)
	}
	if r.MaxDelay < r.BaseDelay {
		return errors.New(
			"MaxDelay must be greater than or equal to BaseDelay",
		)
	}
	return nil
}

// ApplyDefaults applies default values to the retry configuration.
func (r *RetryConfig) ApplyDefaults() {
	if r.MaxAttempts == 0 {
		r.MaxAttempts = 3
	}
	if r.BaseDelay == 0 {
		r.BaseDelay = time.Second
	}
	if r.MaxDelay == 0 {
		r.MaxDelay = 30 * time.Second
	}
	if r.Multiplier == 0 {
		r.Multiplier = 2.0
	}
	r.Jitter = true
}

// CalculateBackoffDelay calculates the backoff delay for a given attempt.
func (r *RetryConfig) CalculateBackoffDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// Calculate exponential backoff: baseDelay * (multiplier ^ (attempt-1))
	delay := float64(r.BaseDelay) * math.Pow(r.Multiplier, float64(attempt-1))

	// Cap at MaxDelay
	if delay > float64(r.MaxDelay) {
		delay = float64(r.MaxDelay)
	}

	calculatedDelay := time.Duration(delay)

	// Apply jitter if enabled (50-100% of calculated delay)
	if r.Jitter {
		jitterRange := calculatedDelay / 2 // 50% of the delay
		jitter := time.Duration(
			rand.Int64N(int64(jitterRange) + 1), //nolint:gosec // math/rand is acceptable for retry jitter
		)
		calculatedDelay = jitterRange + jitter
	}

	return calculatedDelay
}

// IsHTTPErrorRetryable determines if an HTTP error is retryable.
func IsHTTPErrorRetryable(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// IsNetworkErrorRetryable determines if a network error is retryable.
func IsNetworkErrorRetryable(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Check for specific error types
	var timeoutErr *TimeoutError
	if errors.As(err, &timeoutErr) {
		return true
	}

	var connErr *ConnectionError
	if errors.As(err, &connErr) {
		return true
	}

	var validationErr *ValidationError
	return !errors.As(err, &validationErr)
}

// Error types for testing.
type TimeoutError struct {
	Err error
}

func (e *TimeoutError) Error() string {
	return e.Err.Error()
}

type ConnectionError struct {
	Err error
}

func (e *ConnectionError) Error() string {
	return e.Err.Error()
}

type ValidationError struct {
	Err error
}

func (e *ValidationError) Error() string {
	return e.Err.Error()
}

type RateLimitError struct {
	Code       string
	Message    string
	RetryAfter string
}

func (e *RateLimitError) Error() string {
	return e.Message
}

// CalculateRateLimitDelay calculates delay based on rate limit headers.
func CalculateRateLimitDelay(err *outbound.EmbeddingError, headers map[string]string) time.Duration {
	if retryAfter, exists := headers["Retry-After"]; exists {
		if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return 0
}

// CalculateRateLimitDelayWithFallback calculates rate limit delay with fallback to backoff.
func CalculateRateLimitDelayWithFallback(
	err *outbound.EmbeddingError,
	headers map[string]string,
	config *RetryConfig,
	attempt int,
) time.Duration {
	// First try to get delay from headers
	if delay := CalculateRateLimitDelay(err, headers); delay > 0 {
		return delay
	}

	// Fallback to exponential backoff
	return config.CalculateBackoffDelay(attempt)
}

// RetryExecutor handles retry logic.
type RetryExecutor struct {
	config *RetryConfig
}

// NewRetryExecutor creates a new retry executor.
func NewRetryExecutor(config *RetryConfig) *RetryExecutor {
	return &RetryExecutor{config: config}
}

// ExecuteWithRetry executes an operation with retry logic.
func (r *RetryExecutor) ExecuteWithRetry(ctx context.Context, operation func() error) error {
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

		// Log retry attempt
		if attempt > 1 {
			slogger.Debug(ctx, "Executing retry attempt", slogger.Fields3(
				"attempt", attempt,
				"max_attempts", r.config.MaxAttempts,
				"previous_error", lastErr.Error(),
			))
		}

		// Execute operation
		err := operation()
		if err == nil {
			if attempt > 1 {
				slogger.Info(ctx, "Operation succeeded after retry", slogger.Fields2(
					"attempt", attempt,
					"total_attempts", attempt,
				))
			}
			return nil // Success
		}

		lastErr = err

		// Log the failure
		slogger.Debug(ctx, "Operation failed, evaluating retry", slogger.Fields3(
			"attempt", attempt,
			"error", err.Error(),
			"retryable", r.isRetryable(err),
		))

		// Don't retry if not retryable
		if !r.isRetryable(err) {
			slogger.Info(ctx, "Operation failed with non-retryable error", slogger.Fields2(
				"attempt", attempt,
				"error", err.Error(),
			))
			return err
		}

		// Don't sleep after the last attempt
		if attempt == r.config.MaxAttempts {
			slogger.Warn(ctx, "All retry attempts exhausted", slogger.Fields3(
				"max_attempts", r.config.MaxAttempts,
				"final_error", err.Error(),
				"operation", "retry_exhausted",
			))
			break
		}

		// Calculate and wait for delay - check for rate limit headers first
		delay := r.calculateRetryDelay(err, attempt)
		slogger.Debug(ctx, "Waiting before retry attempt", slogger.Fields2(
			"delay", delay.String(),
			"next_attempt", attempt+1,
		))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

// calculateRetryDelay calculates the retry delay, checking for rate limit headers first.
func (r *RetryExecutor) calculateRetryDelay(err error, attempt int) time.Duration {
	// Try to get delay from rate limit headers
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

// extractRateLimitDelay extracts delay from rate limit errors.
func (r *RetryExecutor) extractRateLimitDelay(err error) time.Duration {
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

// isRetryable determines if an error is retryable.
func (r *RetryExecutor) isRetryable(err error) bool {
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

// ZeroAttemptsError represents an error when MaxAttempts is zero.
type ZeroAttemptsError struct {
	Message string
}

func (e *ZeroAttemptsError) Error() string {
	return e.Message
}

// IsZeroAttemptsError checks if an error is a zero attempts error.
func IsZeroAttemptsError(err error) bool {
	var zeroErr *ZeroAttemptsError
	return errors.As(err, &zeroErr)
}

// NilOperationError represents an error when operation is nil.
type NilOperationError struct {
	Message string
}

func (e *NilOperationError) Error() string {
	return e.Message
}

// IsNilOperationError checks if an error is a nil operation error.
func IsNilOperationError(err error) bool {
	var nilErr *NilOperationError
	return errors.As(err, &nilErr)
}

// CircuitBreakerState represents the circuit breaker state.
type CircuitBreakerState int

const (
	CircuitBreakerStateClosed CircuitBreakerState = iota
	CircuitBreakerStateOpen
	CircuitBreakerStateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	failureThreshold int
	timeout          time.Duration
	state            CircuitBreakerState
	failureCount     int
	lastFailureTime  time.Time
	mutex            sync.RWMutex
}

// CircuitBreakerConfig holds circuit breaker configuration.
type CircuitBreakerConfig struct {
	FailureThreshold int
	Timeout          time.Duration
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(failureThreshold int, timeout time.Duration) *CircuitBreaker {
	// Handle zero failure threshold - circuit should always be open
	initialState := CircuitBreakerStateClosed
	if failureThreshold <= 0 {
		initialState = CircuitBreakerStateOpen
	}

	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		timeout:          timeout,
		state:            initialState,
		failureCount:     0,
	}
}

// RecordFailure records a failure and potentially opens the circuit.
func (c *CircuitBreaker) RecordFailure(err error) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.failureCount++
	c.lastFailureTime = time.Now()

	// Only transition to open if we have a positive failure threshold and meet it
	if c.failureThreshold > 0 && c.failureCount >= c.failureThreshold {
		previousState := c.state
		c.state = CircuitBreakerStateOpen

		if previousState != CircuitBreakerStateOpen {
			slogger.WarnNoCtx("Circuit breaker opened due to failure threshold", slogger.Fields3(
				"failure_count", c.failureCount,
				"failure_threshold", c.failureThreshold,
				"previous_state", previousState,
			))
		}
	}
	// For zero or negative threshold, the circuit is already open from construction

	return err
}

// GetState returns the current circuit breaker state.
func (c *CircuitBreaker) GetState() CircuitBreakerState {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// For zero or negative failure threshold, always return open
	if c.failureThreshold <= 0 {
		return CircuitBreakerStateOpen
	}

	// Check if we should transition from OPEN to HALF_OPEN
	if c.state == CircuitBreakerStateOpen && time.Since(c.lastFailureTime) >= c.timeout {
		c.mutex.RUnlock()
		c.mutex.Lock()
		// Double-check pattern
		if c.state == CircuitBreakerStateOpen && time.Since(c.lastFailureTime) >= c.timeout {
			c.state = CircuitBreakerStateHalfOpen
		}
		c.mutex.Unlock()
		c.mutex.RLock()
	}

	return c.state
}

// ExecuteOrReject executes an operation or rejects it based on circuit breaker state.
func (c *CircuitBreaker) ExecuteOrReject(operation func() error) error {
	// For zero or negative failure threshold, always reject
	if c.failureThreshold <= 0 {
		return &CircuitBreakerError{Message: "circuit breaker is open"}
	}

	state := c.GetState()

	switch state {
	case CircuitBreakerStateOpen:
		return &CircuitBreakerError{Message: "circuit breaker is open"}
	case CircuitBreakerStateClosed:
		err := operation()
		if err != nil {
			_ = c.RecordFailure(err)
		} else {
			c.recordSuccess()
		}
		return err
	case CircuitBreakerStateHalfOpen:
		err := operation()
		if err != nil {
			_ = c.RecordFailure(err)
		} else {
			c.recordSuccess()
		}
		return err
	default:
		return operation()
	}
}

// recordSuccess records a successful operation.
func (c *CircuitBreaker) recordSuccess() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	previousState := c.state
	c.failureCount = 0
	c.state = CircuitBreakerStateClosed

	if previousState != CircuitBreakerStateClosed {
		slogger.InfoNoCtx("Circuit breaker closed after successful operation", slogger.Fields2(
			"previous_state", previousState,
			"failure_count_reset", 0,
		))
	}
}

// CircuitBreakerError represents a circuit breaker error.
type CircuitBreakerError struct {
	Message string
}

func (e *CircuitBreakerError) Error() string {
	return e.Message
}

// IsCircuitBreakerError checks if an error is a circuit breaker error.
func IsCircuitBreakerError(err error) bool {
	var cbErr *CircuitBreakerError
	return errors.As(err, &cbErr)
}

// RetryMetrics tracks retry-related metrics.
type RetryMetrics struct {
	mutex              sync.RWMutex
	totalAttempts      int
	operationAttempts  map[string]int
	successCount       int
	failureCount       int
	backoffDelays      []time.Duration
	operationDurations map[string][]time.Duration
	timeoutCounts      map[string]int
}

// NewRetryMetrics creates new retry metrics.
func NewRetryMetrics() *RetryMetrics {
	return &RetryMetrics{
		operationAttempts:  make(map[string]int),
		backoffDelays:      make([]time.Duration, 0),
		operationDurations: make(map[string][]time.Duration),
		timeoutCounts:      make(map[string]int),
	}
}

// NewRetryMetricsWithTiming creates new retry metrics with timing capabilities.
func NewRetryMetricsWithTiming() *RetryMetrics {
	return NewRetryMetrics() // Same as regular metrics since we added timing to the base type
}

// RecordAttempt records a retry attempt.
func (r *RetryMetrics) RecordAttempt(operation string, attempt int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.totalAttempts++
	r.operationAttempts[operation]++
}

// RecordSuccess records a successful operation.
func (r *RetryMetrics) RecordSuccess(operation string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.successCount++
}

// RecordFailure records a failed operation.
func (r *RetryMetrics) RecordFailure(operation string, err error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.failureCount++
}

// RecordBackoffDelay records a backoff delay.
func (r *RetryMetrics) RecordBackoffDelay(delay time.Duration) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.backoffDelays = append(r.backoffDelays, delay)
}

// GetTotalAttempts returns the total number of attempts.
func (r *RetryMetrics) GetTotalAttempts() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.totalAttempts
}

// GetOperationAttempts returns the number of attempts for a specific operation.
func (r *RetryMetrics) GetOperationAttempts(operation string) int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.operationAttempts[operation]
}

// GetSuccessCount returns the number of successful operations.
func (r *RetryMetrics) GetSuccessCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.successCount
}

// GetFailureCount returns the number of failed operations.
func (r *RetryMetrics) GetFailureCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.failureCount
}

// GetSuccessRate returns the success rate as a float64.
func (r *RetryMetrics) GetSuccessRate() float64 {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	total := r.successCount + r.failureCount
	if total == 0 {
		return 0.0
	}

	return float64(r.successCount) / float64(total)
}

// GetAverageBackoffDelay returns the average backoff delay.
func (r *RetryMetrics) GetAverageBackoffDelay() time.Duration {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if len(r.backoffDelays) == 0 {
		return 0
	}

	var total time.Duration
	for _, delay := range r.backoffDelays {
		total += delay
	}

	return total / time.Duration(len(r.backoffDelays))
}

// Extended timing methods for RetryMetrics.

// RecordOperationDuration records the duration of an operation.
func (r *RetryMetrics) RecordOperationDuration(operation string, duration time.Duration) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.operationDurations[operation] = append(r.operationDurations[operation], duration)
}

// RecordTimeout records a timeout occurrence for an operation.
func (r *RetryMetrics) RecordTimeout(operation string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.timeoutCounts[operation]++
}

// GetAverageOperationDuration returns the average duration for a specific operation.
func (r *RetryMetrics) GetAverageOperationDuration(operation string) time.Duration {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	durations, exists := r.operationDurations[operation]
	if !exists || len(durations) == 0 {
		return 0
	}

	var total time.Duration
	for _, duration := range durations {
		total += duration
	}

	return total / time.Duration(len(durations))
}

// GetOverallAverageDuration returns the overall average duration across all operations.
func (r *RetryMetrics) GetOverallAverageDuration() time.Duration {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var total time.Duration
	var count int

	for _, durations := range r.operationDurations {
		for _, duration := range durations {
			total += duration
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return total / time.Duration(count)
}

// GetTimeoutCount returns the number of timeouts for a specific operation.
func (r *RetryMetrics) GetTimeoutCount(operation string) int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.timeoutCounts[operation]
}

// GetTotalTimeoutCount returns the total number of timeouts across all operations.
func (r *RetryMetrics) GetTotalTimeoutCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	total := 0
	for _, count := range r.timeoutCounts {
		total += count
	}

	return total
}

// GetTimeoutRate returns the timeout rate for a specific operation.
func (r *RetryMetrics) GetTimeoutRate(operation string, totalAttempts int) float64 {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	timeouts := r.timeoutCounts[operation]

	if totalAttempts == 0 {
		return 0.0
	}

	return float64(timeouts) / float64(totalAttempts)
}

// Client extension methods for retry and circuit breaker integration.

// ExecuteWithRetryAndCircuitBreaker executes an operation with retry and circuit breaker.
func (c *Client) ExecuteWithRetryAndCircuitBreaker(
	ctx context.Context,
	operation func() (*outbound.EmbeddingResult, error),
	retryConfig *RetryConfig,
	circuitBreakerConfigs ...*CircuitBreakerConfig,
) (*outbound.EmbeddingResult, error) {
	var circuitBreaker *CircuitBreaker

	// Create circuit breaker if config provided
	if len(circuitBreakerConfigs) > 0 && circuitBreakerConfigs[0] != nil {
		config := circuitBreakerConfigs[0]
		circuitBreaker = NewCircuitBreaker(config.FailureThreshold, config.Timeout)
	}

	// Create a custom retry executor that integrates with circuit breaker
	executor := &RetryExecutorWithCircuitBreaker{
		config:         retryConfig,
		circuitBreaker: circuitBreaker,
	}

	return executor.ExecuteWithRetryAndCircuitBreaker(ctx, operation)
}

// RetryExecutorWithCircuitBreaker integrates retry logic with circuit breaker.
type RetryExecutorWithCircuitBreaker struct {
	config         *RetryConfig
	circuitBreaker *CircuitBreaker
}

func (r *RetryExecutorWithCircuitBreaker) ExecuteWithRetryAndCircuitBreaker(
	ctx context.Context,
	operation func() (*outbound.EmbeddingResult, error),
) (*outbound.EmbeddingResult, error) {
	if operation == nil {
		return nil, &NilOperationError{Message: "operation cannot be nil"}
	}

	if r.config.MaxAttempts <= 0 {
		return nil, &ZeroAttemptsError{Message: "MaxAttempts is zero, no attempts will be made"}
	}

	var lastErr error

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Check circuit breaker state before each attempt
		if r.circuitBreaker != nil && r.circuitBreaker.GetState() == CircuitBreakerStateOpen {
			slogger.Debug(ctx, "Circuit breaker is open, stopping retries", slogger.Fields2(
				"attempt", attempt,
				"circuit_state", "open",
			))
			return nil, &CircuitBreakerError{Message: "circuit breaker is open"}
		}

		// Execute operation
		result, err := operation()
		if err == nil {
			// Success - record in circuit breaker if present
			if r.circuitBreaker != nil {
				r.circuitBreaker.recordSuccess()
			}
			return result, nil
		}

		lastErr = err

		// Record failure in circuit breaker if present
		if r.circuitBreaker != nil {
			_ = r.circuitBreaker.RecordFailure(err)
		}

		// Don't retry if not retryable
		if !r.isRetryable(err) {
			return nil, err
		}

		// Don't sleep after the last attempt
		if attempt == r.config.MaxAttempts {
			break
		}

		// Calculate and wait for delay
		delay := r.calculateRetryDelay(err, attempt)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return nil, lastErr
}

func (r *RetryExecutorWithCircuitBreaker) isRetryable(err error) bool {
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

func (r *RetryExecutorWithCircuitBreaker) calculateRetryDelay(err error, attempt int) time.Duration {
	// Try to get delay from rate limit headers
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

func (r *RetryExecutorWithCircuitBreaker) extractRateLimitDelay(err error) time.Duration {
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

// Helper function to check if a string contains a substring.
func contains(str, substr string) bool {
	return strings.Contains(str, substr)
}
