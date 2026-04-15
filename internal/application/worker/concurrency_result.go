package worker

import (
	"codechunking/internal/port/outbound"
	"time"
)

// ConcurrencyResult holds the outcome of running Zoekt indexing and embedding
// generation concurrently. Each engine populates its own fields with no shared
// mutation between goroutines.
type ConcurrencyResult struct {
	ZoektResult       *outbound.ZoektIndexResult
	ZoektErr          error
	ZoektDuration     time.Duration
	EmbeddingErr      error
	EmbeddingDuration time.Duration
	ChunksProcessed   int
}

// BothSucceeded returns true when both Zoekt and embedding completed without error.
func (r ConcurrencyResult) BothSucceeded() bool {
	return r.ZoektErr == nil && r.EmbeddingErr == nil
}

// AnySucceeded returns true when at least one engine completed without error.
func (r ConcurrencyResult) AnySucceeded() bool {
	return r.ZoektErr == nil || r.EmbeddingErr == nil
}

// BothFailed returns true when both engines returned a non-nil error.
func (r ConcurrencyResult) BothFailed() bool {
	return r.ZoektErr != nil && r.EmbeddingErr != nil
}
