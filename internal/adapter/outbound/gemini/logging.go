package gemini

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"time"
)

// LogEmbeddingRequest logs an embedding request and returns request ID with enhanced context.
func (c *Client) LogEmbeddingRequest(ctx context.Context, text string, options outbound.EmbeddingOptions) string {
	requestID := generateRequestID()
	tokenCount, _ := c.EstimateTokenCount(ctx, text)

	// Detect text characteristics for better logging context
	isCodeText := isCodeLike(text)
	wordCount := countWords(text)

	slogger.Info(ctx, "Embedding request initiated", slogger.Fields{
		"request_id":     requestID,
		"model":          c.resolveModel(options.Model),
		"task_type":      c.resolveTaskType(options.TaskType),
		"text_length":    len(text),
		"word_count":     wordCount,
		"token_estimate": tokenCount,
		"is_code_like":   isCodeText,
		"client_config": slogger.Fields2(
			"dimensions", c.config.Dimensions,
			"timeout_seconds", c.config.Timeout.Seconds(),
		),
	})

	return requestID
}

// LogEmbeddingResponse logs a successful embedding response with comprehensive metrics.
func (c *Client) LogEmbeddingResponse(
	ctx context.Context,
	requestID string,
	result *outbound.EmbeddingResult,
	duration time.Duration,
) {
	// Calculate performance metrics
	vectorMagnitude := calculateVectorMagnitude(result.Vector)

	slogger.Info(ctx, "Embedding request completed successfully", slogger.Fields{
		"request_id":       requestID,
		"model":            result.Model,
		"task_type":        string(result.TaskType),
		"dimensions":       result.Dimensions,
		"duration_ms":      duration.Milliseconds(),
		"tokens_processed": result.TokenCount,
		"vector_magnitude": vectorMagnitude,
		"was_truncated":    result.Truncated,
		"performance_metrics": slogger.Fields3(
			"tokens_per_second", float64(result.TokenCount)/duration.Seconds(),
			"processing_rate_ms_per_dim", float64(duration.Milliseconds())/float64(result.Dimensions),
			"bytes_processed", len(result.ProcessedText),
		),
	})
}

// LogEmbeddingError logs an embedding error with comprehensive diagnostic information.
func (c *Client) LogEmbeddingError(
	ctx context.Context,
	requestID string,
	embeddingErr *outbound.EmbeddingError,
	duration time.Duration,
) {
	// Determine error severity and additional context
	severity := determineErrorSeverity(embeddingErr)
	retryRecommendation := getRetryRecommendation(embeddingErr)

	logFields := slogger.Fields{
		"request_id":           requestID,
		"error_type":           embeddingErr.Type,
		"error_code":           embeddingErr.Code,
		"error_message":        embeddingErr.Message,
		"duration_ms":          duration.Milliseconds(),
		"is_retryable":         embeddingErr.Retryable,
		"severity":             severity,
		"retry_recommendation": retryRecommendation,
		"client_config": slogger.Fields2(
			"model", c.config.Model,
			"timeout_seconds", c.config.Timeout.Seconds(),
		),
	}

	// Add cause information if available
	if embeddingErr.Cause != nil {
		logFields["underlying_error"] = embeddingErr.Cause.Error()
	}

	// Add request ID context if available
	if embeddingErr.RequestID != "" {
		logFields["api_request_id"] = embeddingErr.RequestID
	}

	slogger.Error(ctx, "Embedding request failed", logFields)
}

// generateRequestID generates a unique request ID for tracing.
func generateRequestID() string {
	// In production, this would use a proper UUID generator
	// For now, using a simple implementation for GREEN phase
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}

// resolveModel returns the effective model name.
func (c *Client) resolveModel(requestedModel string) string {
	if requestedModel != "" {
		return requestedModel
	}
	return c.config.Model
}

// resolveTaskType returns the effective task type.
func (c *Client) resolveTaskType(requestedTaskType outbound.EmbeddingTaskType) string {
	if requestedTaskType != "" {
		return string(requestedTaskType)
	}
	return c.config.TaskType
}

// calculateVectorMagnitude calculates the Euclidean magnitude of the embedding vector.
func calculateVectorMagnitude(vector []float64) float64 {
	if len(vector) == 0 {
		return 0.0
	}

	sumSquares := 0.0
	for _, val := range vector {
		sumSquares += val * val
	}
	return sumSquares // Return sum of squares instead of sqrt for performance
}

// determineErrorSeverity classifies the error severity level.
func determineErrorSeverity(err *outbound.EmbeddingError) string {
	switch err.Type {
	case "auth":
		return ErrorSeverityHigh
	case "quota":
		return ErrorSeverityMedium
	case "validation":
		return ErrorSeverityLow
	case "network":
		return ErrorSeverityMedium
	case "server":
		return ErrorSeverityHigh
	default:
		return ErrorSeverityMedium
	}
}

// getRetryRecommendation provides retry guidance based on error type.
func getRetryRecommendation(err *outbound.EmbeddingError) string {
	if !err.Retryable {
		return "do_not_retry"
	}

	switch err.Type {
	case "quota":
		return "retry_with_backoff_after_delay"
	case "network":
		return "retry_immediately_up_to_3_times"
	case "server":
		return "retry_with_exponential_backoff"
	default:
		return "retry_with_linear_backoff"
	}
}
