// Package queue implements parallel batch processing for embedding generation.
// This package provides production-ready parallel processing with rate limiting,
// circuit breaker patterns, and comprehensive error handling.
package queue

import (
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// parallelBatchProcessor implements ParallelBatchProcessor interface.
// It provides production-ready parallel processing with advanced features:
//   - Dynamic worker pool management with scaling
//   - Rate limiting to prevent API quota exhaustion
//   - Circuit breaker pattern for resilient error handling
//   - Comprehensive metrics and monitoring
//   - Graceful shutdown and resource management
type parallelBatchProcessor struct {
	// Core dependencies
	embeddingService outbound.EmbeddingService
	config           *ParallelBatchProcessorConfig

	// Worker pool management
	workers    []*worker
	workerPool chan *worker
	mu         sync.RWMutex

	// State management
	running   bool
	shutdown  chan bool
	startTime time.Time

	// Rate limiting and concurrency control
	rateLimiter          *rateLimiter
	concurrencySemaphore chan struct{}

	// Circuit breaker for fault tolerance
	circuitBreaker *circuitBreaker

	// Performance metrics and monitoring
	stats *WorkerPoolStats
}

// worker represents a processing unit in the worker pool.
// Each worker tracks its state and usage for pool management.
type worker struct {
	id       int       // Unique identifier for the worker
	busy     bool      // Whether the worker is currently processing
	lastUsed time.Time // Last time this worker was used (for idle cleanup)
}

// rateLimiter implements token bucket rate limiting to prevent API abuse.
// It ensures requests don't exceed the configured requests per second limit.
type rateLimiter struct {
	rps         float64    // Maximum requests per second allowed
	lastRequest time.Time  // Timestamp of the last request
	hits        int64      // Total number of rate limit hits (atomic)
	mu          sync.Mutex // Protects timing calculations
}

// circuitBreaker implements the circuit breaker pattern for fault tolerance.
// It automatically opens when error rates exceed thresholds, preventing cascade failures.
type circuitBreaker struct {
	errorThreshold float64       // Error rate that triggers circuit opening (0.0-1.0)
	timeout        time.Duration // How long to wait before attempting to close circuit
	isOpen         bool          // Whether the circuit is currently open
	lastFailure    time.Time     // Timestamp of the last failure that opened the circuit
	failures       int64         // Total failure count (atomic)
	successes      int64         // Total success count (atomic)
	mu             sync.RWMutex  // Protects circuit state changes
}

// NewParallelBatchProcessor creates a new production-ready parallel batch processor.
//
// The processor supports:
//   - Dynamic worker pool with configurable min/max workers
//   - Rate limiting to prevent API quota exhaustion
//   - Circuit breaker pattern for fault tolerance
//   - Comprehensive monitoring and metrics
//   - Graceful shutdown and resource cleanup
//
// Parameters:
//   - embeddingService: The embedding service to use for batch processing
//   - config: Configuration parameters for the processor
//
// Returns:
//   - ParallelBatchProcessor: The configured processor instance
//   - error: Configuration validation error, if any
func NewParallelBatchProcessor(
	embeddingService outbound.EmbeddingService,
	config *ParallelBatchProcessorConfig,
) (ParallelBatchProcessor, error) {
	if embeddingService == nil {
		return nil, errors.New("embedding service cannot be nil")
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	processor := &parallelBatchProcessor{
		embeddingService: embeddingService,
		config:           config,
		workerPool:       make(chan *worker, config.MaxWorkers),
		shutdown:         make(chan bool),
		running:          true,
		startTime:        time.Now(),
		stats: &WorkerPoolStats{
			TotalWorkers: config.MinWorkers,
			IdleWorkers:  config.MinWorkers,
		},
	}

	// Initialize rate limiter
	if config.RequestsPerSecond > 0 {
		processor.rateLimiter = &rateLimiter{
			rps:         config.RequestsPerSecond,
			lastRequest: time.Now(),
		}
	}

	// Initialize concurrency semaphore
	if config.MaxConcurrentRequests > 0 {
		processor.concurrencySemaphore = make(chan struct{}, config.MaxConcurrentRequests)
	}

	// Initialize circuit breaker
	if config.ErrorThreshold > 0 {
		processor.circuitBreaker = &circuitBreaker{
			errorThreshold: config.ErrorThreshold,
			timeout:        config.CircuitBreakerTimeout,
		}
	}

	// Initialize minimum workers
	processor.initializeWorkers()

	return processor, nil
}

// validateConfig validates the processor configuration for correctness and consistency.
// It ensures all configuration parameters are within valid ranges and logically consistent.
func validateConfig(config *ParallelBatchProcessorConfig) error {
	if config == nil {
		return errors.New("config cannot be nil")
	}

	// Validate worker pool configuration
	if config.MaxWorkers <= 0 {
		return errors.New("MaxWorkers must be greater than 0")
	}
	if config.MinWorkers < 0 {
		return errors.New("MinWorkers cannot be negative")
	}
	if config.MinWorkers > config.MaxWorkers {
		return errors.New("MinWorkers cannot be greater than MaxWorkers")
	}

	// Validate concurrency and rate limiting configuration
	if config.MaxConcurrentRequests < 0 {
		return errors.New("MaxConcurrentRequests cannot be negative")
	}
	if config.RequestsPerSecond < 0 {
		return errors.New("RequestsPerSecond cannot be negative")
	}

	// Validate circuit breaker configuration
	if config.ErrorThreshold < 0 || config.ErrorThreshold > 1 {
		return errors.New("ErrorThreshold must be between 0.0 and 1.0")
	}
	if config.CircuitBreakerTimeout < 0 {
		return errors.New("CircuitBreakerTimeout cannot be negative")
	}

	// Validate timeout configuration
	if config.BatchTimeout < 0 {
		return errors.New("BatchTimeout cannot be negative")
	}
	if config.WorkerIdleTimeout < 0 {
		return errors.New("WorkerIdleTimeout cannot be negative")
	}

	return nil
}

func (p *parallelBatchProcessor) initializeWorkers() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.workers = make([]*worker, p.config.MinWorkers)
	for i := range p.config.MinWorkers {
		worker := &worker{
			id:       i,
			busy:     false,
			lastUsed: time.Now(),
		}
		p.workers[i] = worker
		p.workerPool <- worker
	}
}

// ProcessBatchesParallel processes multiple batches concurrently with fault tolerance.
//
// This method implements production-ready parallel processing with:
//   - Concurrent execution across multiple worker goroutines
//   - Rate limiting to prevent API quota exhaustion
//   - Circuit breaker pattern for fault tolerance
//   - Timeout handling for individual batches
//   - Comprehensive error tracking and reporting
//
// The method processes batches independently and returns partial results even if some batches fail.
// Only critical failures (timeouts, circuit breaker open) cause the entire operation to fail.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - batches: Slice of batch requests to process concurrently
//
// Returns:
//   - [][]*outbound.EmbeddingBatchResult: Results for each batch (matches input order)
//   - error: Critical processing error (timeouts, shutdown, circuit breaker)
func (p *parallelBatchProcessor) ProcessBatchesParallel(
	ctx context.Context,
	batches [][]*outbound.EmbeddingRequest,
) ([][]*outbound.EmbeddingBatchResult, error) {
	if !p.running {
		return nil, errors.New("processor is shutdown")
	}

	if p.circuitBreaker != nil && p.circuitBreaker.isOpen {
		if time.Since(p.circuitBreaker.lastFailure) < p.circuitBreaker.timeout {
			return nil, errors.New("circuit breaker is open")
		}
		// Reset circuit breaker after timeout
		p.circuitBreaker.isOpen = false
	}

	results := make([][]*outbound.EmbeddingBatchResult, len(batches))

	// Process batches with concurrency control
	var wg sync.WaitGroup
	var mu sync.Mutex
	var processingErrors []error // Collect processing errors to return

	// Use semaphore to control concurrency if configured
	semaphore := make(chan struct{}, len(batches))
	if p.concurrencySemaphore != nil {
		semaphore = make(chan struct{}, cap(p.concurrencySemaphore))
	}

	for i, batch := range batches {
		wg.Add(1)
		go func(idx int, batchReqs []*outbound.EmbeddingRequest) {
			defer wg.Done()

			// Check for cancellation
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}

			// Rate limiting
			if p.rateLimiter != nil {
				if err := p.rateLimiter.acquireWithContext(ctx); err != nil {
					return
				}
			}

			// Create context with timeout if configured
			batchCtx := ctx
			if p.config.BatchTimeout > 0 {
				var cancel context.CancelFunc
				batchCtx, cancel = context.WithTimeout(ctx, p.config.BatchTimeout)
				defer cancel()
			}

			// Process the batch
			batchResults, err := p.processSingleBatch(batchCtx, batchReqs)

			// Handle timeout specifically
			isTimeout := false
			if p.config.BatchTimeout > 0 && err != nil &&
				(errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "timeout") ||
					strings.Contains(err.Error(), "deadline")) {
				isTimeout = true
				for j := range batchResults {
					if batchResults[j].Error == "" {
						batchResults[j].Error = "batch timeout exceeded"
					}
				}
			}

			mu.Lock()
			results[idx] = batchResults
			// Only collect timeout errors for return - partial batch failures should not cause ProcessBatchesParallel to fail
			if isTimeout {
				processingErrors = append(processingErrors, fmt.Errorf("batch %d timeout exceeded", idx))
			}
			mu.Unlock()

			// Check for partial failures in batch results
			hasErrors := false
			if err != nil {
				hasErrors = true
			} else {
				// Check if any individual results have errors
				for _, result := range batchResults {
					if result.Error != "" {
						hasErrors = true
						break
					}
				}
			}

			// Update circuit breaker based on whether batch had errors
			if p.circuitBreaker != nil {
				if hasErrors {
					p.circuitBreaker.recordFailure()
				} else {
					p.circuitBreaker.recordSuccess()
				}
			}

			// Update stats
			atomic.AddInt64(&p.stats.TotalBatchesProcessed, 1)
			if hasErrors {
				atomic.AddInt64(&p.stats.TotalErrors, 1)
				now := time.Now()
				p.stats.LastErrorTime = &now
			}
		}(i, batch)
	}

	wg.Wait()

	// Check if context was cancelled
	if ctx.Err() != nil {
		return results, ctx.Err()
	}

	// Return first significant error if any occurred
	if len(processingErrors) > 0 {
		return results, processingErrors[0]
	}

	return results, nil
}

func (p *parallelBatchProcessor) processSingleBatch(
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

	atomic.AddInt64(&p.stats.TotalRequestsProcessed, int64(len(requests)))
	return results, nil
}

// GetWorkerPoolStats returns comprehensive runtime statistics for monitoring and debugging.
//
// The returned statistics include:
//   - Worker pool utilization (active/idle/total workers)
//   - Processing metrics (total batches/requests processed)
//   - Error tracking (total errors, error rate, last error time)
//   - Rate limiting metrics (hit count, current rate)
//   - Circuit breaker status (open/closed, thresholds)
//   - Runtime information (uptime, goroutine count)
//
// These metrics are essential for:
//   - Performance monitoring and optimization
//   - Capacity planning and scaling decisions
//   - Debugging performance issues
//   - Alerting and operational visibility
func (p *parallelBatchProcessor) GetWorkerPoolStats(ctx context.Context) (*WorkerPoolStats, error) {
	if !p.running {
		return nil, errors.New("processor is shutdown")
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := &WorkerPoolStats{
		ActiveWorkers:     0,
		IdleWorkers:       len(p.workers),
		TotalWorkers:      len(p.workers),
		QueuedBatches:     0,
		ProcessingBatches: 0,

		TotalBatchesProcessed:  atomic.LoadInt64(&p.stats.TotalBatchesProcessed),
		TotalRequestsProcessed: atomic.LoadInt64(&p.stats.TotalRequestsProcessed),

		GoroutineCount: runtime.NumGoroutine(),
		UptimeDuration: time.Since(p.startTime),

		TotalErrors:   atomic.LoadInt64(&p.stats.TotalErrors),
		LastErrorTime: p.stats.LastErrorTime,
	}

	// Count active workers
	for _, worker := range p.workers {
		if worker.busy {
			stats.ActiveWorkers++
			stats.IdleWorkers--
		}
	}

	// Rate limiting stats
	if p.rateLimiter != nil {
		stats.RateLimitHits = atomic.LoadInt64(&p.rateLimiter.hits)
		stats.CurrentRequestsPerSecond = p.rateLimiter.rps
	}

	// Circuit breaker stats
	if p.circuitBreaker != nil {
		stats.CircuitOpen = p.circuitBreaker.isOpen
		if stats.TotalBatchesProcessed > 0 {
			stats.ErrorRate = float64(stats.TotalErrors) / float64(stats.TotalBatchesProcessed)
		}
	}

	return stats, nil
}

// ScaleWorkers dynamically adjusts the number of workers.
func (p *parallelBatchProcessor) ScaleWorkers(ctx context.Context, targetWorkers int) error {
	if !p.running {
		return errors.New("processor is shutdown")
	}

	if targetWorkers > p.config.MaxWorkers {
		return fmt.Errorf("target workers %d exceeds maximum %d", targetWorkers, p.config.MaxWorkers)
	}

	if targetWorkers < p.config.MinWorkers {
		targetWorkers = p.config.MinWorkers
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	currentWorkers := len(p.workers)

	if targetWorkers > currentWorkers {
		// Scale up
		for i := currentWorkers; i < targetWorkers; i++ {
			worker := &worker{
				id:       i,
				busy:     false,
				lastUsed: time.Now(),
			}
			p.workers = append(p.workers, worker)
			p.workerPool <- worker
		}
	} else if targetWorkers < currentWorkers {
		// Scale down
		workersToRemove := currentWorkers - targetWorkers
		for range workersToRemove {
			if len(p.workers) > 0 {
				p.workers = p.workers[:len(p.workers)-1]
			}
		}
	}

	now := time.Now()
	p.stats.LastScaledAt = &now

	return nil
}

// GetConfig returns current configuration.
func (p *parallelBatchProcessor) GetConfig() *ParallelBatchProcessorConfig {
	return p.config
}

// UpdateConfig updates processor configuration.
func (p *parallelBatchProcessor) UpdateConfig(config *ParallelBatchProcessorConfig) error {
	if err := validateConfig(config); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = config
	return nil
}

// Shutdown gracefully shuts down the processor.
func (p *parallelBatchProcessor) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return errors.New("processor already shutdown")
	}

	p.running = false
	close(p.shutdown)

	return nil
}

// IsHealthy returns whether the processor is healthy.
func (p *parallelBatchProcessor) IsHealthy(ctx context.Context) (bool, error) {
	if !p.running {
		return false, errors.New("processor is shutdown")
	}

	// Check circuit breaker
	if p.circuitBreaker != nil && p.circuitBreaker.isOpen {
		return false, errors.New("circuit breaker is open")
	}

	return true, nil
}

// ProcessBatch implements the BatchProcessor interface.
func (p *parallelBatchProcessor) ProcessBatch(
	ctx context.Context,
	requests []*outbound.EmbeddingRequest,
) ([]*outbound.EmbeddingBatchResult, error) {
	// Delegate to single batch processing
	return p.processSingleBatch(ctx, requests)
}

// EstimateBatchCost estimates the cost of processing a batch.
// Uses helper function for consistent cost calculation logic.
func (p *parallelBatchProcessor) EstimateBatchCost(
	ctx context.Context,
	requests []*outbound.EmbeddingRequest,
) (float64, error) {
	return estimateBatchProcessingCost(requests), nil
}

// EstimateBatchLatency estimates the processing time for a batch.
// Uses helper function for consistent latency prediction logic.
func (p *parallelBatchProcessor) EstimateBatchLatency(
	ctx context.Context,
	requests []*outbound.EmbeddingRequest,
) (time.Duration, error) {
	return estimateBatchProcessingLatency(requests), nil
}

// Helper methods are now implemented in parallel_processor_helpers.go
