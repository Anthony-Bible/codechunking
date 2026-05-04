package openai

import (
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"math/rand/v2"
	"time"
)

// retryableFunc is any operation that may need to be retried on transient
// errors. The closure must capture its own arguments; the helper only cares
// about the result and error.
type retryableFunc[T any] func() (T, error)

// retryConfig parameterizes the backoff behavior. Zero-valued fields fall
// back to safe defaults — the helper is usable without explicit config.
type retryConfig struct {
	maxAttempts  int           // total attempts including the first; <= 0 means 1
	initialDelay time.Duration // base delay before first retry
	maxDelay     time.Duration // upper bound on per-attempt delay
}

// retryWithBackoff invokes op up to cfg.maxAttempts times. It retries only
// when op returns an *outbound.EmbeddingError with Retryable=true; every
// other error (including bare errors and EmbeddingError with Retryable=false)
// is returned immediately so we don't spend retries on programmer bugs.
// Backoff follows exponential growth (initial * 2^n) capped at maxDelay, with
// up to 30% jitter to avoid thundering-herd retries from concurrent callers.
//
// Context cancellation is honored between attempts — if ctx.Err() is set
// during a sleep, the helper returns the wrapped ctx error rather than
// continuing.
func retryWithBackoff[T any](ctx context.Context, cfg retryConfig, op retryableFunc[T]) (T, error) {
	var zero T
	attempts := cfg.maxAttempts
	if attempts < 1 {
		attempts = 1
	}
	initial := cfg.initialDelay
	if initial <= 0 {
		initial = 500 * time.Millisecond
	}
	maxD := cfg.maxDelay
	if maxD <= 0 {
		maxD = 30 * time.Second
	}

	if err := ctx.Err(); err != nil {
		return zero, err
	}

	var lastErr error
	for attempt := range attempts {
		result, err := op()
		if err == nil {
			return result, nil
		}
		if !isRetryable(err) {
			return zero, err
		}
		lastErr = err
		if attempt == attempts-1 {
			break
		}

		// Cap-during-multiplication: doubling once the delay reaches maxD/2
		// would meet or exceed the cap, so we clamp here. This avoids the
		// int64 overflow that `initial << attempt` produces for large
		// attempt counts (a misconfigured maxRetries could otherwise wrap
		// to a negative duration and trigger a tight retry loop).
		delay := initial
		for range attempt {
			if delay >= maxD/2 {
				delay = maxD
				break
			}
			delay *= 2
		}
		if delay > maxD {
			delay = maxD
		}
		// ±30% jitter; weak rand is fine — spreading retries, not security.
		delay += time.Duration((rand.Float64() - 0.5) * 0.6 * float64(delay)) //nolint:gosec // jitter

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return zero, ctx.Err()
		case <-timer.C:
		}
	}
	return zero, lastErr
}

// isRetryable reports whether err warrants another attempt. Only typed
// *outbound.EmbeddingError values with Retryable=true qualify; bare errors
// (e.g. JSON decode failures) are treated as terminal so we don't waste
// requests on programmer bugs.
func isRetryable(err error) bool {
	var ee *outbound.EmbeddingError
	if errors.As(err, &ee) {
		return ee.Retryable
	}
	return false
}
