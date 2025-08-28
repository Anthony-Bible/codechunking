package worker

import (
	"codechunking/internal/application/service"
	"codechunking/internal/domain/messaging"
	"context"
	"errors"
	"sync"
	"time"
)

// RetryableJobProcessor extends JobProcessor with retry capabilities.
type RetryableJobProcessor interface {
	// ProcessJobWithRetry processes a job with retry logic and circuit breaker protection.
	ProcessJobWithRetry(ctx context.Context, message messaging.EnhancedIndexingJobMessage) error

	// GetRetryMetrics returns retry metrics for the job processor.
	GetRetryMetrics() service.RetryMetrics

	// GetRetryPolicy returns the current retry policy configuration.
	GetRetryPolicy() service.RetryPolicy

	// UpdateRetryPolicy updates the retry policy (for adaptive behavior).
	UpdateRetryPolicy(policy service.RetryPolicy) error

	// GetFailureStatistics returns statistics about failure types and retry outcomes.
	GetFailureStatistics() map[service.FailureType]FailureStatistics
}

// FailureStatistics holds statistics about specific failure types.
type FailureStatistics struct {
	TotalAttempts     int
	SuccessfulRetries int
	ExhaustedRetries  int
	AverageRetryDelay time.Duration
	LastFailureTime   time.Time
}

// RetryableJobProcessorConfig extends JobProcessorConfig with retry settings.
type RetryableJobProcessorConfig struct {
	JobProcessorConfig       JobProcessorConfig
	RetryPolicyConfig        service.RetryPolicyConfig
	CircuitBreakerConfig     service.CBConfig
	RetryMetricsConfig       service.RetryMetricsConfig
	AdaptiveRetryEnabled     bool
	FailureClassifierEnabled bool
	ProcessFunc              func(ctx context.Context, message messaging.EnhancedIndexingJobMessage) error
}

// DefaultRetryableJobProcessor implements RetryableJobProcessor.
type DefaultRetryableJobProcessor struct {
	config            RetryableJobProcessorConfig
	retryWithCB       service.RetryWithCircuitBreaker
	failureStats      map[service.FailureType]FailureStatistics
	failureClassifier service.FailureClassifier
	mu                sync.RWMutex
}

// NewRetryableJobProcessor creates a new retryable job processor.
func NewRetryableJobProcessor(config RetryableJobProcessorConfig) RetryableJobProcessor {
	// Create retry with circuit breaker configuration
	retryCircuitConfig := service.RetryCircuitBreakerConfig{
		CircuitBreakerConfig: config.CircuitBreakerConfig,
		RetryPolicyConfig:    config.RetryPolicyConfig,
		MetricsConfig:        config.RetryMetricsConfig,
		AdaptiveBehavior:     config.AdaptiveRetryEnabled,
	}

	retryWithCB, err := service.NewRetryWithCircuitBreaker(retryCircuitConfig)
	if err != nil {
		// In GREEN phase, return a basic implementation on error
		// In production, this should be handled more carefully
		panic("failed to create retry with circuit breaker: " + err.Error())
	}

	var classifier service.FailureClassifier
	if config.FailureClassifierEnabled {
		classifier = service.NewFailureClassifier()
	}

	return &DefaultRetryableJobProcessor{
		config:            config,
		retryWithCB:       retryWithCB,
		failureStats:      make(map[service.FailureType]FailureStatistics),
		failureClassifier: classifier,
	}
}

func (p *DefaultRetryableJobProcessor) ProcessJobWithRetry(
	ctx context.Context,
	message messaging.EnhancedIndexingJobMessage,
) error {
	p.mu.RLock()
	processFunc := p.config.ProcessFunc
	p.mu.RUnlock()

	if processFunc == nil {
		return errors.New("no process function configured")
	}

	// Wrap the processing function for retry execution
	operation := func() error {
		return processFunc(ctx, message)
	}

	startTime := time.Now()

	// Execute with retry and circuit breaker
	err := p.retryWithCB.ExecuteWithRetry(ctx, operation)

	// Update statistics
	p.updateFailureStatistics(err, time.Since(startTime))

	return err
}

func (p *DefaultRetryableJobProcessor) updateFailureStatistics(err error, duration time.Duration) {
	if p.failureClassifier == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if err != nil {
		failureType := p.failureClassifier.Classify(err)
		stats := p.failureStats[failureType]
		stats.TotalAttempts++
		stats.LastFailureTime = time.Now()

		// Simple average calculation for demonstration
		if stats.TotalAttempts == 1 {
			stats.AverageRetryDelay = duration
		} else {
			stats.AverageRetryDelay = (stats.AverageRetryDelay + duration) / 2
		}

		stats.ExhaustedRetries++
		p.failureStats[failureType] = stats
	} else {
		// Success case - could be after retries
		// For simplicity, we'll assume it was a successful retry if there were previous failures
		for failureType, stats := range p.failureStats {
			if stats.TotalAttempts > 0 {
				stats.SuccessfulRetries++
				p.failureStats[failureType] = stats
				break // Only increment for one failure type to avoid double counting
			}
		}
	}
}

func (p *DefaultRetryableJobProcessor) GetRetryMetrics() service.RetryMetrics {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.retryWithCB.GetMetrics()
}

func (p *DefaultRetryableJobProcessor) GetRetryPolicy() service.RetryPolicy {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.retryWithCB.GetRetryPolicy()
}

func (p *DefaultRetryableJobProcessor) UpdateRetryPolicy(policy service.RetryPolicy) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.retryWithCB.UpdatePolicy(policy)
}

func (p *DefaultRetryableJobProcessor) GetFailureStatistics() map[service.FailureType]FailureStatistics {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[service.FailureType]FailureStatistics)
	for k, v := range p.failureStats {
		result[k] = v
	}
	return result
}
