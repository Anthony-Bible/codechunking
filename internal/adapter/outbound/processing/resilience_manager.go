package streamingcodeprocessor

import (
	"context"
	"log/slog"
	"your-module/pipeline"
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

func (rm *ProcessingResilienceManager) WithErrorHandling(ctx context.Context, operation func() error) error {
	if !rm.circuitBreaker.IsClosed() {
		return rm.fallbackManager.ExecuteFallback(ctx)
	}

	err := rm.retryManager.ExecuteWithRetry(ctx, operation)
	if err != nil {
		errorType := rm.errorClassifier.ClassifyError(err)
		if errorType == pipeline.ErrorTypeTransient {
			rm.logger.Warn("Transient error occurred", "error", err)
		} else if errorType == pipeline.ErrorTypePermanent {
			rm.circuitBreaker.RecordFailure()
			rm.logger.Error("Permanent error occurred", "error", err)
		}
		return err
	}

	rm.circuitBreaker.RecordSuccess()
	return nil
}

func (rm *ProcessingResilienceManager) Close() error {
	rm.circuitBreaker.Close()
	rm.retryManager.Close()
	rm.fallbackManager.Close()
	return nil
}

func (rm *ProcessingResilienceManager) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"circuit_breaker_state": rm.circuitBreaker.GetState(),
		"retry_attempts":        rm.retryManager.GetAttemptCount(),
		"error_classification":  rm.errorClassifier.GetMetrics(),
		"fallback_usage":        rm.fallbackManager.GetMetrics(),
	}
}
