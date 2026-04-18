package slogger

import (
	"codechunking/internal/application/common/logging"
	"context"
	"sync"
	"testing"
)

func newSilentLogger(t *testing.T) logging.ApplicationLogger {
	t.Helper()
	l, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR",
		Format: "json",
		Output: "buffer",
	})
	if err != nil {
		t.Fatalf("failed to create silent logger: %v", err)
	}
	return l
}

// TestLoggerManager_ConcurrentSetAndGet verifies no data race when goroutines
// concurrently call SetLogger and getLogger.
func TestLoggerManager_ConcurrentSetAndGet(t *testing.T) {
	custom := newSilentLogger(t)
	lm := &LoggerManager{}

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			switch i % 3 {
			case 0:
				lm.SetLogger(custom)
			case 1:
				lm.SetLogger(nil)
			case 2:
				// Must not panic even when racing with SetLogger(nil).
				_ = lm.getLogger()
				lm.getLogger().Info(context.Background(), "race test", Fields{"goroutine": i})
			}
		}(i)
	}
	wg.Wait()
}

// TestLoggerManager_SetNilReinitializes verifies that setting nil triggers
// lazy re-initialization on the next getLogger call.
func TestLoggerManager_SetNilReinitializes(t *testing.T) {
	custom := newSilentLogger(t)
	lm := &LoggerManager{}

	lm.SetLogger(custom)
	lm.SetLogger(nil)

	// Must not panic — should auto-initialize.
	logger := lm.getLogger()
	if logger == nil {
		t.Fatal("expected non-nil logger after nil reset and re-init")
	}
	logger.Info(context.Background(), "re-init test", Fields{})
}
