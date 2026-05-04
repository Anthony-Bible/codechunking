package openai

import (
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetry_SucceedsOnFirstAttempt(t *testing.T) {
	var calls int32
	got, err := retryWithBackoff(context.Background(), retryConfig{maxAttempts: 3}, func() (int, error) {
		atomic.AddInt32(&calls, 1)
		return 42, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetry_RetriesUntilSuccess(t *testing.T) {
	var calls int32
	got, err := retryWithBackoff(context.Background(),
		retryConfig{maxAttempts: 5, initialDelay: time.Millisecond, maxDelay: 5 * time.Millisecond},
		func() (int, error) {
			n := atomic.AddInt32(&calls, 1)
			if n < 3 {
				return 0, &outbound.EmbeddingError{Type: "server", Code: "5xx", Retryable: true}
			}
			return 7, nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 7 {
		t.Errorf("expected 7, got %d", got)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetry_StopsOnNonRetryable(t *testing.T) {
	var calls int32
	_, err := retryWithBackoff(context.Background(),
		retryConfig{maxAttempts: 5, initialDelay: time.Millisecond},
		func() (int, error) {
			atomic.AddInt32(&calls, 1)
			return 0, &outbound.EmbeddingError{Type: "auth", Code: "unauthorized", Retryable: false}
		})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry), got %d", calls)
	}
}

func TestRetry_BareErrorIsNotRetryable(t *testing.T) {
	var calls int32
	_, err := retryWithBackoff(context.Background(),
		retryConfig{maxAttempts: 5, initialDelay: time.Millisecond},
		func() (int, error) {
			atomic.AddInt32(&calls, 1)
			return 0, errors.New("decoding failure")
		})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("expected 1 call for bare error, got %d", calls)
	}
}

func TestRetry_ExhaustsMaxAttempts(t *testing.T) {
	var calls int32
	_, err := retryWithBackoff(context.Background(),
		retryConfig{maxAttempts: 3, initialDelay: time.Millisecond, maxDelay: 5 * time.Millisecond},
		func() (int, error) {
			atomic.AddInt32(&calls, 1)
			return 0, &outbound.EmbeddingError{Type: "quota", Code: "429", Retryable: true}
		})
	if err == nil {
		t.Fatal("expected error after exhaustion")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetry_HonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls int32
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	_, err := retryWithBackoff(ctx,
		retryConfig{maxAttempts: 100, initialDelay: 50 * time.Millisecond, maxDelay: 100 * time.Millisecond},
		func() (int, error) {
			atomic.AddInt32(&calls, 1)
			return 0, &outbound.EmbeddingError{Type: "server", Retryable: true}
		})
	if err == nil {
		t.Fatal("expected ctx.Err()")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// TestRetry_NoOverflowOnLargeMaxAttempts guards against a regression where
// `initial << attempt` overflowed int64 for large attempt counts and
// produced a negative duration, defeating the maxDelay cap and triggering
// a tight retry loop. Cancelling the context shortly after the call
// proves no individual sleep ran at the (overflowed) negative duration:
// if it had, every retry would fire immediately and the call count would
// race ahead of the wall clock budget below.
func TestRetry_NoOverflowOnLargeMaxAttempts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var calls int32
	start := time.Now()
	_, err := retryWithBackoff(ctx,
		retryConfig{
			maxAttempts:  40, // would overflow int64 around attempt 35
			initialDelay: 500 * time.Millisecond,
			maxDelay:     30 * time.Second,
		},
		func() (int, error) {
			atomic.AddInt32(&calls, 1)
			return 0, &outbound.EmbeddingError{Type: "server", Retryable: true}
		})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	// With initialDelay=500ms, the first sleep alone exceeds the 100ms
	// context budget — so we should see at most ~2 calls. If overflow
	// had produced a negative delay, time.After would fire immediately
	// and we'd burn through all 40 attempts well inside 100ms.
	if c := atomic.LoadInt32(&calls); c > 5 {
		t.Errorf("suspected overflow tight-loop: got %d calls in %s", c, elapsed)
	}
}

func TestRetry_DefaultsAppliedWhenZero(t *testing.T) {
	// maxAttempts <= 0 should be treated as 1 (no retry).
	var calls int32
	_, _ = retryWithBackoff(context.Background(), retryConfig{},
		func() (int, error) {
			atomic.AddInt32(&calls, 1)
			return 0, &outbound.EmbeddingError{Retryable: true}
		})
	if calls != 1 {
		t.Errorf("expected 1 call with zero maxAttempts, got %d", calls)
	}
}
