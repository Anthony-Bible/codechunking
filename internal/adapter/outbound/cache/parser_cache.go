package cache

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ParseResultCache provides thread-safe caching of parse results with LRU eviction.
type ParseResultCache struct {
	cache    map[string]*CacheEntry
	lruOrder []string
	maxSize  int
	mu       sync.RWMutex
	stats    *CacheStatistics
}

// CacheEntry represents a cached parse result with metadata.
type CacheEntry struct {
	Result      *treesitter.ParseResult
	Language    valueobject.Language
	SourceHash  string
	CreatedAt   time.Time
	AccessedAt  time.Time
	AccessCount int64
}

// CacheStatistics tracks cache performance metrics.
type CacheStatistics struct {
	Hits        int64
	Misses      int64
	Evictions   int64
	TotalItems  int64
	HitRate     float64
	LastUpdated time.Time
}

// NewParseResultCache creates a new cache with specified maximum size.
func NewParseResultCache(maxSize int) *ParseResultCache {
	return &ParseResultCache{
		cache:    make(map[string]*CacheEntry),
		lruOrder: make([]string, 0, maxSize),
		maxSize:  maxSize,
		stats: &CacheStatistics{
			LastUpdated: time.Now(),
		},
	}
}

// Get retrieves a cached parse result if available.
func (c *ParseResultCache) Get(
	ctx context.Context,
	language valueobject.Language,
	sourceCode []byte,
) (*treesitter.ParseResult, bool) {
	key := c.generateKey(language, sourceCode)

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.cache[key]
	if !exists {
		c.stats.Misses++
		c.updateHitRate()

		slogger.Debug(ctx, "Cache miss for parse result", slogger.Fields{
			"language": language.Name(),
			"key":      key[:8] + "...",
		})

		return nil, false
	}

	// Update access metadata
	entry.AccessedAt = time.Now()
	entry.AccessCount++
	c.stats.Hits++
	c.updateHitRate()

	// Move to front of LRU order
	c.moveToFront(key)

	slogger.Debug(ctx, "Cache hit for parse result", slogger.Fields{
		"language":     language.Name(),
		"key":          key[:8] + "...",
		"access_count": entry.AccessCount,
	})

	return entry.Result, true
}

// Put stores a parse result in the cache.
func (c *ParseResultCache) Put(
	ctx context.Context,
	language valueobject.Language,
	sourceCode []byte,
	result *treesitter.ParseResult,
) {
	if result == nil {
		return
	}

	key := c.generateKey(language, sourceCode)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict
	if len(c.cache) >= c.maxSize {
		c.evictLRU(ctx)
	}

	entry := &CacheEntry{
		Result:      result,
		Language:    language,
		SourceHash:  key,
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		AccessCount: 0,
	}

	c.cache[key] = entry
	c.lruOrder = append(c.lruOrder, key)
	c.stats.TotalItems++

	slogger.Debug(ctx, "Cached parse result", slogger.Fields{
		"language":    language.Name(),
		"key":         key[:8] + "...",
		"cache_size":  len(c.cache),
		"source_size": len(sourceCode),
	})
}

// generateKey creates a cache key from language and source code.
func (c *ParseResultCache) generateKey(language valueobject.Language, sourceCode []byte) string {
	hash := sha256.Sum256(sourceCode)
	sourceHash := hex.EncodeToString(hash[:])
	return fmt.Sprintf("%s:%s", language.Name(), sourceHash[:16])
}

// moveToFront moves a key to the front of the LRU order.
func (c *ParseResultCache) moveToFront(key string) {
	// Remove from current position
	for i, k := range c.lruOrder {
		if k == key {
			c.lruOrder = append(c.lruOrder[:i], c.lruOrder[i+1:]...)
			break
		}
	}

	// Add to front
	c.lruOrder = append([]string{key}, c.lruOrder...)
}

// evictLRU removes the least recently used entry.
func (c *ParseResultCache) evictLRU(ctx context.Context) {
	if len(c.lruOrder) == 0 {
		return
	}

	// Remove from the end (least recently used)
	evictKey := c.lruOrder[len(c.lruOrder)-1]
	c.lruOrder = c.lruOrder[:len(c.lruOrder)-1]

	entry, exists := c.cache[evictKey]
	if exists {
		delete(c.cache, evictKey)
		c.stats.Evictions++

		slogger.Debug(ctx, "Evicted cache entry", slogger.Fields{
			"language":     entry.Language.Name(),
			"key":          evictKey[:8] + "...",
			"access_count": entry.AccessCount,
			"age_seconds":  time.Since(entry.CreatedAt).Seconds(),
		})
	}
}

// updateHitRate recalculates the cache hit rate.
func (c *ParseResultCache) updateHitRate() {
	total := c.stats.Hits + c.stats.Misses
	if total > 0 {
		c.stats.HitRate = float64(c.stats.Hits) / float64(total)
	}
	c.stats.LastUpdated = time.Now()
}

// GetStatistics returns current cache statistics.
func (c *ParseResultCache) GetStatistics() *CacheStatistics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid race conditions
	stats := *c.stats
	stats.TotalItems = int64(len(c.cache))
	return &stats
}

// Clear removes all entries from the cache.
func (c *ParseResultCache) Clear(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cleared := len(c.cache)
	c.cache = make(map[string]*CacheEntry)
	c.lruOrder = make([]string, 0, c.maxSize)

	// Reset statistics but preserve hit rate history
	c.stats.TotalItems = 0

	slogger.Info(ctx, "Parse result cache cleared", slogger.Fields{
		"entries_cleared": cleared,
	})
}

// Resize changes the maximum cache size.
func (c *ParseResultCache) Resize(ctx context.Context, newSize int) {
	if newSize <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	oldSize := c.maxSize
	c.maxSize = newSize

	// Evict entries if new size is smaller
	for len(c.cache) > newSize {
		c.evictLRU(ctx)
	}

	slogger.Info(ctx, "Cache resized", slogger.Fields{
		"old_size":     oldSize,
		"new_size":     newSize,
		"current_size": len(c.cache),
	})
}
