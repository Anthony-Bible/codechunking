// Package queue - Helper functions for parallel batch processor
// This file contains extracted helper functions to improve modularity and maintainability.
package queue

import (
	"codechunking/internal/port/outbound"
	"context"
	"sync/atomic"
	"time"
)

// rateLimiterAcquire handles rate limiting with context support.
// It implements token bucket algorithm to prevent API quota exhaustion.
func (r *rateLimiter) acquireWithContext(ctx context.Context) error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	now := time.Now()
	timeSinceLastRequest := now.Sub(r.lastRequest)
	minInterval := time.Duration(float64(time.Second) / r.rps)

	if timeSinceLastRequest < minInterval {
		sleepTime := minInterval - timeSinceLastRequest
		r.lastRequest = now.Add(sleepTime)
		atomic.AddInt64(&r.hits, 1)
		r.mu.Unlock()

		select {
		case <-time.After(sleepTime):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	} else {
		r.lastRequest = now
		r.mu.Unlock()
	}

	return nil
}

// circuitBreakerRecordFailure updates circuit breaker state on failure.
// Opens circuit when error rate exceeds threshold over minimum sample size.
func (cb *circuitBreaker) recordFailure() {
	if cb == nil {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	total := cb.failures + cb.successes

	// Only evaluate circuit breaker after minimum sample size
	if total >= 5 && float64(cb.failures)/float64(total) >= cb.errorThreshold {
		cb.isOpen = true
		cb.lastFailure = time.Now()
	}
}

// circuitBreakerRecordSuccess updates circuit breaker state on success.
func (cb *circuitBreaker) recordSuccess() {
	if cb == nil {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successes++
}

// estimateBatchProcessingCost calculates approximate cost for batch processing.
// Uses token-based estimation with configurable rate per token.
func estimateBatchProcessingCost(requests []*outbound.EmbeddingRequest) float64 {
	if len(requests) == 0 {
		return 0.0
	}

	// Simple cost estimation based on number of tokens
	totalCost := 0.0
	costPerToken := 0.00001 // $0.00001 per token (example rate)

	for _, req := range requests {
		// Estimate tokens (rough approximation: 4 characters per token)
		estimatedTokens := len(req.Text) / 4
		if estimatedTokens < 1 {
			estimatedTokens = 1
		}
		totalCost += float64(estimatedTokens) * costPerToken
	}

	// Add base request cost
	baseRequestCost := 0.001 // $0.001 per request
	totalCost += baseRequestCost

	return totalCost
}

// estimateBatchProcessingLatency predicts processing time based on batch characteristics.
// Accounts for base API latency and batch size economies of scale.
func estimateBatchProcessingLatency(requests []*outbound.EmbeddingRequest) time.Duration {
	if len(requests) == 0 {
		return 0
	}

	// Base latency for API call
	baseLatency := 200 * time.Millisecond

	// Processing time increases with batch size but has economies of scale
	batchSize := len(requests)
	processingTime := time.Duration(float64(batchSize)*5.0) * time.Millisecond // 5ms per item

	// Larger batches get better efficiency
	if batchSize > 10 {
		processingTime = time.Duration(float64(processingTime) * 0.8) // 20% efficiency gain
	}
	if batchSize > 50 {
		processingTime = time.Duration(float64(processingTime) * 0.9) // Additional 10% efficiency
	}

	totalLatency := baseLatency + processingTime
	return totalLatency
}
