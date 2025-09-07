package streamingcodeprocessor

import (
	"codechunking/internal/application/worker/pipeline"
	"context"
	"log/slog"
)

type ProcessingResilienceManager struct {
	errorClassifier *pipeline.ProcessingErrorClassifier
	circuitBreaker  *pipeline.ProcessingCircuitBreaker
	retryManager    *pipeline.ProcessingRetryManager
	fallbackManager *pipeline.ProcessingFallbackManager
	logger          *slog.Logger
}

func NewProcessingResilienceManager(
	errorClassifier *pipeline.ProcessingErrorClassifier,
	circuitBreaker *pipeline.ProcessingCircuitBreaker,
	retryManager *pipeline.ProcessingRetryManager,
	fallbackManager *pipeline.ProcessingFallbackManager,
	logger *slog.Logger,
) *ProcessingResilienceManager {
	return &ProcessingResilienceManager{
		errorClassifier: errorClassifier,
		circuitBreaker:  circuitBreaker,
		retryManager:    retryManager,
		fallbackManager: fallbackManager,
		logger:          logger,
	}
}

func (rm *ProcessingResilienceManager) Execute(ctx context.Context, operation func() error) error {
	return rm.circuitBreaker.Execute(ctx, func() error {
		return rm.retryManager.ExecuteWithRetry(ctx, operation)
	})
}

func (rm *ProcessingResilienceManager) Close() error {
	return nil
}

func (rm *ProcessingResilienceManager) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"circuit_breaker_state": rm.circuitBreaker.GetState(),
	}
}
