package gemini

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// TokenCacheEntry represents a cached token count with TTL.
type TokenCacheEntry struct {
	TokenCount int
	Model      string
	CachedAt   time.Time
}

// IsExpired checks if the cache entry has expired based on TTL.
func (e *TokenCacheEntry) IsExpired(ttl time.Duration) bool {
	return time.Since(e.CachedAt) > ttl
}

// TokenCache provides thread-safe LRU caching for token counts with TTL support.
type TokenCache struct {
	mu      sync.RWMutex
	cache   map[string]*TokenCacheEntry
	maxSize int
	ttl     time.Duration
	lruList []string // Simple LRU tracking using slice (newest at end)
}

// TokenCacheConfig holds configuration for the token cache.
type TokenCacheConfig struct {
	MaxSize int           // Maximum number of entries (0 = unlimited)
	TTL     time.Duration // Time-to-live for cache entries (0 = no expiration)
}

// NewTokenCache creates a new token cache with the given configuration.
func NewTokenCache(config TokenCacheConfig) *TokenCache {
	maxSize := config.MaxSize
	if maxSize <= 0 {
		maxSize = 1000 // Default max size
	}

	ttl := config.TTL
	if ttl <= 0 {
		ttl = 1 * time.Hour // Default TTL
	}

	return &TokenCache{
		cache:   make(map[string]*TokenCacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
		lruList: make([]string, 0, maxSize),
	}
}

// Get retrieves a token count from the cache if it exists and hasn't expired.
// Returns the entry and true if found and valid, nil and false otherwise.
func (tc *TokenCache) Get(text, model string) (*TokenCacheEntry, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	key := tc.generateKey(text, model)
	entry, exists := tc.cache[key]
	if !exists {
		return nil, false
	}

	// Check if entry has expired
	if entry.IsExpired(tc.ttl) {
		return nil, false
	}

	return entry, true
}

// Set stores a token count in the cache with LRU eviction if necessary.
func (tc *TokenCache) Set(text, model string, tokenCount int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	key := tc.generateKey(text, model)

	// Check if we need to evict the oldest entry
	if len(tc.cache) >= tc.maxSize {
		if _, exists := tc.cache[key]; !exists {
			// Only evict if this is a new entry (not an update)
			tc.evictOldest()
		}
	}

	// Add or update entry
	entry := &TokenCacheEntry{
		TokenCount: tokenCount,
		Model:      model,
		CachedAt:   time.Now(),
	}
	tc.cache[key] = entry

	// Update LRU list
	tc.updateLRU(key)
}

// Clear removes all entries from the cache.
func (tc *TokenCache) Clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.cache = make(map[string]*TokenCacheEntry)
	tc.lruList = make([]string, 0, tc.maxSize)
}

// Size returns the current number of entries in the cache.
func (tc *TokenCache) Size() int {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	return len(tc.cache)
}

// generateKey creates a cache key from text and model using SHA-256 hash.
func (tc *TokenCache) generateKey(text, model string) string {
	h := sha256.New()
	h.Write([]byte(text))
	h.Write([]byte(model))
	return hex.EncodeToString(h.Sum(nil))
}

// evictOldest removes the oldest entry from the cache (LRU eviction).
func (tc *TokenCache) evictOldest() {
	if len(tc.lruList) == 0 {
		return
	}

	// Remove oldest (first) entry
	oldestKey := tc.lruList[0]
	delete(tc.cache, oldestKey)
	tc.lruList = tc.lruList[1:]
}

// updateLRU moves the key to the end of the LRU list (most recently used).
func (tc *TokenCache) updateLRU(key string) {
	// Remove key if it exists in the list
	for i, k := range tc.lruList {
		if k == key {
			tc.lruList = append(tc.lruList[:i], tc.lruList[i+1:]...)
			break
		}
	}

	// Add key to the end (most recently used)
	tc.lruList = append(tc.lruList, key)
}
