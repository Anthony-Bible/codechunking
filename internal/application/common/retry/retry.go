package retry

import (
	"codechunking/internal/application/common/slogger"
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// RetryConfig defines retry behavior.
type RetryConfig struct {
	MaxRetries    int           `json:"max_retries"`
	InitialDelay  time.Duration `json:"initial_delay"`
	MaxDelay      time.Duration `json:"max_delay"`
	BackoffFactor float64       `json:"backoff_factor"`
	Jitter        bool          `json:"jitter"`
}

// DefaultRetryConfig returns a default retry configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
	}
}

// RetryableOperation represents an operation that can be retried.
type RetryableOperation func(ctx context.Context) error

// RetryableChecker is an interface for custom retry logic.
// Implement this to provide custom error classification.
type RetryableChecker interface {
	IsRetryable(err error) bool
}

// RetryExecutor handles retry logic with exponential backoff.
type RetryExecutor struct {
	config           *RetryConfig
	retryableChecker RetryableChecker
}

// NewRetryExecutor creates a new retry executor with default retry behavior.
func NewRetryExecutor(config *RetryConfig) *RetryExecutor {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &RetryExecutor{
		config:           config,
		retryableChecker: &DefaultRetryableChecker{},
	}
}

// NewRetryExecutorWithChecker creates a new retry executor with custom retry behavior.
func NewRetryExecutorWithChecker(config *RetryConfig, checker RetryableChecker) *RetryExecutor {
	if config == nil {
		config = DefaultRetryConfig()
	}
	if checker == nil {
		checker = &DefaultRetryableChecker{}
	}
	return &RetryExecutor{
		config:           config,
		retryableChecker: checker,
	}
}

// Execute executes an operation with retry logic.
func (r *RetryExecutor) Execute(ctx context.Context, operation RetryableOperation) error {
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := r.calculateDelay(attempt)
			slogger.Debug(ctx, "Retrying operation after delay", slogger.Fields3(
				"attempt", attempt,
				"max_retries", r.config.MaxRetries,
				"delay_ms", delay.Milliseconds(),
			))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Continue with retry
			}
		}

		err := operation(ctx)
		if err == nil {
			if attempt > 0 {
				slogger.Info(ctx, "Operation succeeded after retries", slogger.Fields{
					"attempt": attempt + 1,
				})
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.retryableChecker.IsRetryable(err) {
			slogger.Debug(ctx, "Error is not retryable", slogger.Fields{
				"error":   err.Error(),
				"attempt": attempt + 1,
			})
			return err
		}

		slogger.Warn(ctx, "Operation failed, will retry", slogger.Fields3(
			"error", err.Error(),
			"attempt", attempt+1,
			"max_retries", r.config.MaxRetries,
		))
	}

	return fmt.Errorf("operation failed after %d retries: %w", r.config.MaxRetries, lastErr)
}

// calculateDelay calculates the delay for a given attempt using exponential backoff.
func (r *RetryExecutor) calculateDelay(attempt int) time.Duration {
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.BackoffFactor, float64(attempt-1))

	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}

	// Add jitter if enabled
	if r.config.Jitter {
		// Add random jitter up to ±25% of the delay
		jitterRange := delay * 0.25
		delay += (float64(time.Now().UnixNano()%1000000)/1000000.0 - 0.5) * 2 * jitterRange
	}

	return time.Duration(delay)
}

// DefaultRetryableChecker implements basic retry logic for common transient errors.
type DefaultRetryableChecker struct{}

// IsRetryable checks if an error should be retried based on common patterns.
func (d *DefaultRetryableChecker) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Database connection errors
	if containsAny(errStr, []string{
		"connection refused",
		"connection reset",
		"timeout",
		"deadlock",
		"connection lost",
		"too many connections",
		"database is locked",
	}) {
		return true
	}

	// Temporary errors
	if containsAny(errStr, []string{
		"temporary",
		"try again",
		"resource temporarily unavailable",
	}) {
		return true
	}

	// Network errors
	if containsAny(errStr, []string{
		"network is unreachable",
		"no route to host",
		"connection timed out",
	}) {
		return true
	}

	return false
}

// containsAny checks if the string contains any of the substrings.
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// WithRetry executes a function with retry logic using the default configuration.
func WithRetry(ctx context.Context, operation RetryableOperation) error {
	executor := NewRetryExecutor(DefaultRetryConfig())
	return executor.Execute(ctx, operation)
}

// WithRetryConfig executes a function with custom retry configuration.
func WithRetryConfig(ctx context.Context, config *RetryConfig, operation RetryableOperation) error {
	executor := NewRetryExecutor(config)
	return executor.Execute(ctx, operation)
}

// WithRetryAndChecker executes a function with custom retry configuration and checker.
func WithRetryAndChecker(
	ctx context.Context,
	config *RetryConfig,
	checker RetryableChecker,
	operation RetryableOperation,
) error {
	executor := NewRetryExecutorWithChecker(config, checker)
	return executor.Execute(ctx, operation)
}
