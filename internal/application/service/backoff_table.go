package service

import (
	"context"
	"math"
	"math/rand/v2"
	"sync"
	"time"
)

// BackoffTableKey represents a unique key for backoff table caching.
type BackoffTableKey struct {
	BaseDelay         time.Duration
	MaxDelay          time.Duration
	BackoffMultiplier float64
	MaxAttempts       int
}

// BackoffTable holds pre-computed delay values for retry attempts.
type BackoffTable struct {
	delays []time.Duration
	key    BackoffTableKey
}

// GetDelay returns the delay for a specific attempt number (1-indexed).
func (bt *BackoffTable) GetDelay(attempt int) time.Duration {
	if attempt < 1 || attempt > len(bt.delays) {
		return 0
	}
	return bt.delays[attempt-1]
}

// GetMaxAttempts returns the maximum number of attempts in this table.
func (bt *BackoffTable) GetMaxAttempts() int {
	return len(bt.delays)
}

// BackoffTableCache caches pre-computed backoff tables for performance.
type BackoffTableCache struct {
	cache sync.Map // map[BackoffTableKey]*BackoffTable
}

// NewBackoffTableCache creates a new backoff table cache.
func NewBackoffTableCache() *BackoffTableCache {
	return &BackoffTableCache{}
}

// GetTable retrieves or creates a backoff table for the given parameters.
func (btc *BackoffTableCache) GetTable(
	baseDelay time.Duration,
	maxDelay time.Duration,
	backoffMultiplier float64,
	maxAttempts int,
) *BackoffTable {
	key := BackoffTableKey{
		BaseDelay:         baseDelay,
		MaxDelay:          maxDelay,
		BackoffMultiplier: backoffMultiplier,
		MaxAttempts:       maxAttempts,
	}

	if cached, ok := btc.cache.Load(key); ok {
		if table, ok := cached.(*BackoffTable); ok {
			return table
		}
		// This should never happen with our cache setup, but handle gracefully
		return btc.createTable(key)
	}

	// Create new table
	table := btc.createTable(key)
	btc.cache.Store(key, table)
	return table
}

// createTable creates a new backoff table with pre-computed delays.
func (btc *BackoffTableCache) createTable(key BackoffTableKey) *BackoffTable {
	delays := make([]time.Duration, key.MaxAttempts)

	for attempt := range key.MaxAttempts {
		var delay time.Duration

		if key.BackoffMultiplier > 0 {
			// Exponential backoff
			delay = time.Duration(float64(key.BaseDelay) * math.Pow(key.BackoffMultiplier, float64(attempt)))
		} else {
			// Linear or fixed backoff
			if attempt == 0 {
				delay = key.BaseDelay
			} else {
				delay = time.Duration(int64(key.BaseDelay) * int64(attempt+1))
			}
		}

		// Apply max delay limit
		if key.MaxDelay > 0 && delay > key.MaxDelay {
			delay = key.MaxDelay
		}

		delays[attempt] = delay
	}

	return &BackoffTable{
		delays: delays,
		key:    key,
	}
}

// Clear removes all cached tables.
func (btc *BackoffTableCache) Clear() {
	btc.cache.Range(func(key, _ interface{}) bool {
		btc.cache.Delete(key)
		return true
	})
}

// Size returns the number of cached tables.
func (btc *BackoffTableCache) Size() int {
	count := 0
	btc.cache.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// OptimizedBackoffPolicy uses pre-computed backoff tables for improved performance.
type OptimizedBackoffPolicy struct {
	config RetryPolicyConfig
	cache  *BackoffTableCache
	table  *BackoffTable
	mu     sync.RWMutex
}

// NewOptimizedBackoffPolicy creates a new optimized backoff policy with pre-computed delays.
func NewOptimizedBackoffPolicy(config RetryPolicyConfig, cache *BackoffTableCache) RetryPolicy {
	if cache == nil {
		cache = NewBackoffTableCache()
	}

	table := cache.GetTable(
		config.BaseDelay,
		config.MaxDelay,
		config.BackoffMultiplier,
		config.MaxAttempts,
	)

	return &OptimizedBackoffPolicy{
		config: config,
		cache:  cache,
		table:  table,
	}
}

func (p *OptimizedBackoffPolicy) ShouldRetry(ctx context.Context, _ error, attempt int) (bool, time.Duration) {
	// Check context cancellation first
	if ctx.Err() != nil {
		return false, 0
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Check if we've exceeded max attempts
	if attempt >= p.config.MaxAttempts {
		return false, 0
	}

	// Get pre-computed delay
	delay := p.table.GetDelay(attempt)

	// Apply jitter if enabled
	if p.config.JitterEnabled {
		jitterAmount := float64(delay) * (float64(p.config.JitterMaxPercent) / 100.0)
		jitter := time.Duration(rand.Float64() * jitterAmount) //nolint:gosec // Non-cryptographic use for retry jitter
		delay += jitter
	}

	return true, delay
}

func (p *OptimizedBackoffPolicy) GetMaxAttempts() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config.MaxAttempts
}

func (p *OptimizedBackoffPolicy) GetPolicyName() string {
	return "optimized_backoff"
}

func (p *OptimizedBackoffPolicy) Reset() {
	// No internal state to reset for optimized backoff
}

// GetFastRetryTableKey returns configuration for quick recovery scenarios (3 attempts, 100ms base).
func GetFastRetryTableKey() BackoffTableKey {
	return BackoffTableKey{
		BaseDelay:         100 * time.Millisecond,
		MaxDelay:          1 * time.Second,
		BackoffMultiplier: 2.0,
		MaxAttempts:       3,
	}
}

// GetStandardRetryTableKey returns configuration for typical retry scenarios (5 attempts, 500ms base).
func GetStandardRetryTableKey() BackoffTableKey {
	return BackoffTableKey{
		BaseDelay:         500 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		MaxAttempts:       5,
	}
}

// GetSlowRetryTableKey returns configuration for expensive operations (3 attempts, 2s base).
func GetSlowRetryTableKey() BackoffTableKey {
	return BackoffTableKey{
		BaseDelay:         2 * time.Second,
		MaxDelay:          60 * time.Second,
		BackoffMultiplier: 3.0,
		MaxAttempts:       3,
	}
}

// GetOptimizedPolicy creates an optimized policy for common scenarios.
func GetOptimizedPolicy(scenario BackoffTableKey, cache *BackoffTableCache) RetryPolicy {
	config := RetryPolicyConfig{
		MaxAttempts:       scenario.MaxAttempts,
		BaseDelay:         scenario.BaseDelay,
		MaxDelay:          scenario.MaxDelay,
		BackoffMultiplier: scenario.BackoffMultiplier,
	}

	return NewOptimizedBackoffPolicy(config, cache)
}
