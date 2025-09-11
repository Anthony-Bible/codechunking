package cache

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"context"
	"fmt"
	"sync"
	"time"
)

// ConcurrentParserPool manages a pool of parsers for concurrent processing.
type ConcurrentParserPool struct {
	parsers       map[string][]treesitter.ObservableTreeSitterParser
	factory       *treesitter.ParserFactoryImpl
	cache         *ParseResultCache
	mu            sync.RWMutex
	semaphores    map[string]chan struct{}
	maxConcurrent int
	stats         *PoolStatistics
}

// PoolStatistics tracks concurrent parsing performance.
type PoolStatistics struct {
	TotalRequests      int64
	ConcurrentPeaks    int64
	AverageWaitTime    time.Duration
	ParsedSuccessfully int64
	CacheHitRate       float64
	LastUpdated        time.Time
}

// ParseRequest represents a concurrent parsing request.
type ParseRequest struct {
	Language   valueobject.Language
	SourceCode []byte
	Context    context.Context
	ResultChan chan *ParseResponse
}

// ParseResponse contains the result of a concurrent parsing operation.
type ParseResponse struct {
	Result    *treesitter.ParseResult
	Error     error
	FromCache bool
	Duration  time.Duration
}

// NewConcurrentParserPool creates a new concurrent parser pool.
func NewConcurrentParserPool(
	factory *treesitter.ParserFactoryImpl,
	cacheSize, maxConcurrent int,
) *ConcurrentParserPool {
	return &ConcurrentParserPool{
		parsers:       make(map[string][]treesitter.ObservableTreeSitterParser),
		factory:       factory,
		cache:         NewParseResultCache(cacheSize),
		semaphores:    make(map[string]chan struct{}),
		maxConcurrent: maxConcurrent,
		stats: &PoolStatistics{
			LastUpdated: time.Now(),
		},
	}
}

// ParseConcurrently parses source code using cached results or concurrent parsing.
func (p *ConcurrentParserPool) ParseConcurrently(
	ctx context.Context,
	language valueobject.Language,
	sourceCode []byte,
) (*treesitter.ParseResult, error) {
	startTime := time.Now()
	p.stats.TotalRequests++

	// First check cache
	if result, found := p.cache.Get(ctx, language, sourceCode); found {
		p.updateCacheHitRate(true)

		slogger.Debug(ctx, "Returning cached parse result", slogger.Fields{
			"language":    language.Name(),
			"duration_ms": time.Since(startTime).Milliseconds(),
		})

		return result, nil
	}

	p.updateCacheHitRate(false)

	// Get or create semaphore for this language
	sem := p.getSemaphore(language.Name())

	// Wait for available slot
	waitStart := time.Now()
	select {
	case sem <- struct{}{}:
		waitTime := time.Since(waitStart)
		p.updateWaitTime(waitTime)

		slogger.Debug(ctx, "Acquired parser slot", slogger.Fields{
			"language":     language.Name(),
			"wait_time_ms": waitTime.Milliseconds(),
		})
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Ensure we release the semaphore
	defer func() { <-sem }()

	// Parse the source code
	result, err := p.parseWithParser(ctx, language, sourceCode)
	if err != nil {
		return nil, err
	}

	// Cache the result
	p.cache.Put(ctx, language, sourceCode, result)
	p.stats.ParsedSuccessfully++

	duration := time.Since(startTime)
	slogger.Debug(ctx, "Concurrent parsing completed", slogger.Fields{
		"language":    language.Name(),
		"duration_ms": duration.Milliseconds(),
		"from_cache":  false,
	})

	return result, nil
}

// ParseBatch processes multiple parse requests concurrently.
func (p *ConcurrentParserPool) ParseBatch(ctx context.Context, requests []ParseRequest) []*ParseResponse {
	if len(requests) == 0 {
		return nil
	}

	responses := make([]*ParseResponse, len(requests))
	var wg sync.WaitGroup

	for i, req := range requests {
		wg.Add(1)
		go func(index int, request ParseRequest) {
			defer wg.Done()

			startTime := time.Now()
			result, err := p.ParseConcurrently(request.Context, request.Language, request.SourceCode)

			response := &ParseResponse{
				Result:    result,
				Error:     err,
				FromCache: false, // Cache status handled in ParseConcurrently
				Duration:  time.Since(startTime),
			}

			responses[index] = response

			// Send to result channel if provided
			if request.ResultChan != nil {
				select {
				case request.ResultChan <- response:
				case <-request.Context.Done():
				}
			}
		}(i, req)
	}

	wg.Wait()

	slogger.Info(ctx, "Batch parsing completed", slogger.Fields{
		"total_requests": len(requests),
		"successful":     p.countSuccessful(responses),
	})

	return responses
}

// parseWithParser performs the actual parsing with a parser instance.
func (p *ConcurrentParserPool) parseWithParser(
	ctx context.Context,
	language valueobject.Language,
	sourceCode []byte,
) (*treesitter.ParseResult, error) {
	// Get or create parser for this language
	parser, err := p.getParser(ctx, language)
	if err != nil {
		return nil, fmt.Errorf("failed to get parser for %s: %w", language.Name(), err)
	}

	// Perform parsing
	result, err := parser.Parse(ctx, sourceCode)
	if err != nil {
		slogger.Error(ctx, "Parser failed", slogger.Fields{
			"language": language.Name(),
			"error":    err.Error(),
		})
		return nil, err
	}

	return result, nil
}

// getParser retrieves or creates a parser for the specified language.
func (p *ConcurrentParserPool) getParser(
	ctx context.Context,
	language valueobject.Language,
) (treesitter.ObservableTreeSitterParser, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	langName := language.Name()

	// Try to reuse an existing parser
	if parsers, exists := p.parsers[langName]; exists && len(parsers) > 0 {
		// Pop a parser from the pool
		parser := parsers[len(parsers)-1]
		p.parsers[langName] = parsers[:len(parsers)-1]
		return parser, nil
	}

	// Create a new parser
	parser, err := p.factory.CreateParser(ctx, language)
	if err != nil {
		return nil, err
	}

	slogger.Debug(ctx, "Created new parser instance", slogger.Fields{
		"language": langName,
	})

	return parser, nil
}

// getSemaphore gets or creates a semaphore for the specified language.
func (p *ConcurrentParserPool) getSemaphore(language string) chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	if sem, exists := p.semaphores[language]; exists {
		return sem
	}

	// Create new semaphore with max concurrent limit
	sem := make(chan struct{}, p.maxConcurrent)
	p.semaphores[language] = sem
	return sem
}

// updateCacheHitRate updates cache hit rate statistics.
func (p *ConcurrentParserPool) updateCacheHitRate(hit bool) {
	cacheStats := p.cache.GetStatistics()
	p.stats.CacheHitRate = cacheStats.HitRate
	p.stats.LastUpdated = time.Now()
}

// updateWaitTime updates average wait time statistics.
func (p *ConcurrentParserPool) updateWaitTime(waitTime time.Duration) {
	// Simple exponential moving average
	alpha := 0.1
	currentAvg := p.stats.AverageWaitTime.Nanoseconds()
	newAvg := int64(alpha*float64(waitTime.Nanoseconds()) + (1-alpha)*float64(currentAvg))
	p.stats.AverageWaitTime = time.Duration(newAvg)
}

// countSuccessful counts successful responses in a batch.
func (p *ConcurrentParserPool) countSuccessful(responses []*ParseResponse) int {
	count := 0
	for _, resp := range responses {
		if resp != nil && resp.Error == nil {
			count++
		}
	}
	return count
}

// GetStatistics returns current pool statistics.
func (p *ConcurrentParserPool) GetStatistics() *PoolStatistics {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy to avoid race conditions
	stats := *p.stats
	return &stats
}

// GetCacheStatistics returns current cache statistics.
func (p *ConcurrentParserPool) GetCacheStatistics() *CacheStatistics {
	return p.cache.GetStatistics()
}

// Shutdown gracefully shuts down the parser pool.
func (p *ConcurrentParserPool) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close all parsers
	totalClosed := 0
	for lang, parsers := range p.parsers {
		for _, parser := range parsers {
			if closer, ok := parser.(interface{ Close() error }); ok {
				_ = closer.Close()
				totalClosed++
			}
		}
		delete(p.parsers, lang)
	}

	// Clear cache
	p.cache.Clear(ctx)

	slogger.Info(ctx, "Parser pool shutdown completed", slogger.Fields{
		"parsers_closed": totalClosed,
		"total_requests": p.stats.TotalRequests,
		"cache_hit_rate": p.stats.CacheHitRate,
	})

	return nil
}
