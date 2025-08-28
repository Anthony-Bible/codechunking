package language

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// DetectionCache provides caching for language detection results.
type DetectionCache struct {
	entries map[string]*cacheEntry
	mutex   sync.RWMutex

	// Configuration
	maxSize int
	ttl     time.Duration

	// Observability
	tracer trace.Tracer

	// Metrics
	hitCounter      metric.Int64Counter
	missCounter     metric.Int64Counter
	evictionCounter metric.Int64Counter
	sizeGauge       metric.Int64Gauge
}

type cacheEntry struct {
	language    valueobject.Language
	timestamp   time.Time
	accessCount int64
}

// NewDetectionCache creates a new cache instance.
func NewDetectionCache(maxSize int, ttl time.Duration) *DetectionCache {
	tracer := otel.Tracer("language-detection-cache")
	meter := otel.Meter("language-detection-cache")

	hitCounter, _ := meter.Int64Counter(
		"cache_hits_total",
		metric.WithDescription("Total number of cache hits"),
	)

	missCounter, _ := meter.Int64Counter(
		"cache_misses_total",
		metric.WithDescription("Total number of cache misses"),
	)

	evictionCounter, _ := meter.Int64Counter(
		"cache_evictions_total",
		metric.WithDescription("Total number of cache evictions"),
	)

	sizeGauge, _ := meter.Int64Gauge(
		"cache_size",
		metric.WithDescription("Current number of entries in cache"),
	)

	return &DetectionCache{
		entries:         make(map[string]*cacheEntry),
		maxSize:         maxSize,
		ttl:             ttl,
		tracer:          tracer,
		hitCounter:      hitCounter,
		missCounter:     missCounter,
		evictionCounter: evictionCounter,
		sizeGauge:       sizeGauge,
	}
}

// Get retrieves a language from the cache.
func (c *DetectionCache) Get(ctx context.Context, key string) (valueobject.Language, bool) {
	ctx, span := c.tracer.Start(ctx, "cache.Get")
	defer span.End()

	span.SetAttributes(attribute.String("cache_key", key))

	c.mutex.RLock()
	entry, found := c.entries[key]
	c.mutex.RUnlock()

	if !found {
		c.missCounter.Add(ctx, 1)
		span.SetAttributes(attribute.Bool("cache_hit", false))
		return valueobject.Language{}, false
	}

	// Check if entry has expired
	if time.Since(entry.timestamp) > c.ttl {
		c.mutex.Lock()
		delete(c.entries, key)
		c.mutex.Unlock()

		c.missCounter.Add(ctx, 1)
		c.sizeGauge.Record(ctx, int64(len(c.entries)))
		span.SetAttributes(attribute.Bool("cache_hit", false), attribute.Bool("expired", true))
		return valueobject.Language{}, false
	}

	// Update access count
	c.mutex.Lock()
	entry.accessCount++
	c.mutex.Unlock()

	c.hitCounter.Add(ctx, 1)
	span.SetAttributes(
		attribute.Bool("cache_hit", true),
		attribute.String("language", entry.language.Name()),
	)

	return entry.language, true
}

// Put stores a language in the cache.
func (c *DetectionCache) Put(ctx context.Context, key string, language valueobject.Language) {
	ctx, span := c.tracer.Start(ctx, "cache.Put")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache_key", key),
		attribute.String("language", language.Name()),
	)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if we need to evict entries
	if len(c.entries) >= c.maxSize {
		c.evictLeastUsed(ctx)
	}

	c.entries[key] = &cacheEntry{
		language:    language,
		timestamp:   time.Now(),
		accessCount: 1,
	}

	c.sizeGauge.Record(ctx, int64(len(c.entries)))
}

// evictLeastUsed removes the least frequently used entry.
func (c *DetectionCache) evictLeastUsed(ctx context.Context) {
	var leastUsedKey string
	var minAccessCount int64 = -1
	oldestTime := time.Now()

	for key, entry := range c.entries {
		if minAccessCount == -1 || entry.accessCount < minAccessCount ||
			(entry.accessCount == minAccessCount && entry.timestamp.Before(oldestTime)) {
			leastUsedKey = key
			minAccessCount = entry.accessCount
			oldestTime = entry.timestamp
		}
	}

	if leastUsedKey != "" {
		delete(c.entries, leastUsedKey)
		c.evictionCounter.Add(ctx, 1)
	}
}

// Clear removes all entries from the cache.
func (c *DetectionCache) Clear(ctx context.Context) {
	ctx, span := c.tracer.Start(ctx, "cache.Clear")
	defer span.End()

	c.mutex.Lock()
	c.entries = make(map[string]*cacheEntry)
	c.mutex.Unlock()

	c.sizeGauge.Record(ctx, 0)
}

// Size returns the current number of entries in the cache.
func (c *DetectionCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.entries)
}
