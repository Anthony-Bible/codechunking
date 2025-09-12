package queue

import (
	"codechunking/internal/port/outbound"
	"context"
	"time"
)

// EmbeddingServiceBatchProcessor implements BatchProcessor using the EmbeddingService.
type EmbeddingServiceBatchProcessor struct {
	embeddingService outbound.EmbeddingService
}

// NewEmbeddingServiceBatchProcessor creates a new batch processor that uses the embedding service.
func NewEmbeddingServiceBatchProcessor(embeddingService outbound.EmbeddingService) outbound.BatchProcessor {
	return &EmbeddingServiceBatchProcessor{
		embeddingService: embeddingService,
	}
}

// ProcessBatch processes a batch of embedding requests and returns results.
func (p *EmbeddingServiceBatchProcessor) ProcessBatch(
	ctx context.Context,
	requests []*outbound.EmbeddingRequest,
) ([]*outbound.EmbeddingBatchResult, error) {
	if len(requests) == 0 {
		return []*outbound.EmbeddingBatchResult{}, nil
	}

	results := make([]*outbound.EmbeddingBatchResult, len(requests))
	startTime := time.Now()

	// Extract texts from requests
	texts := make([]string, len(requests))
	for i, req := range requests {
		texts[i] = req.Text
	}

	// Use the embedding service to generate batch embeddings
	embeddingResults, err := p.embeddingService.GenerateBatchEmbeddings(ctx, texts, requests[0].Options)
	if err != nil {
		// If batch processing fails, return error results for all requests
		for i, req := range requests {
			results[i] = &outbound.EmbeddingBatchResult{
				RequestID:   req.RequestID,
				Error:       err.Error(),
				ProcessedAt: time.Now(),
				Latency:     time.Since(startTime),
			}
		}
		return results, err
	}

	// Convert embedding results to batch results
	for i, req := range requests {
		result := &outbound.EmbeddingBatchResult{
			RequestID:   req.RequestID,
			ProcessedAt: time.Now(),
			Latency:     time.Since(startTime),
		}

		if i < len(embeddingResults) {
			result.Result = embeddingResults[i]
		} else {
			result.Error = "Missing embedding result for request"
		}

		results[i] = result
	}

	return results, nil
}

// EstimateBatchCost estimates the cost of processing a batch.
func (p *EmbeddingServiceBatchProcessor) EstimateBatchCost(
	ctx context.Context,
	requests []*outbound.EmbeddingRequest,
) (float64, error) {
	if len(requests) == 0 {
		return 0.0, nil
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

	return totalCost, nil
}

// EstimateBatchLatency estimates the processing time for a batch.
func (p *EmbeddingServiceBatchProcessor) EstimateBatchLatency(
	ctx context.Context,
	requests []*outbound.EmbeddingRequest,
) (time.Duration, error) {
	if len(requests) == 0 {
		return 0, nil
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
	return totalLatency, nil
}
