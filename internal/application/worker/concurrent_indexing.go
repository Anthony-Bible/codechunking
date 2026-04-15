package worker

import (
	"context"
	"time"

	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

// shouldRunConcurrentIndexing reports whether both Zoekt and embedding indexing
// should run concurrently. Returns false when the Zoekt indexer is nil or when
// concurrent indexing is disabled in config.
func (p *DefaultJobProcessor) shouldRunConcurrentIndexing() bool {
	return p.zoektIndexer != nil && p.zoektConfig.ConcurrentIndexing.Enabled
}

// runConcurrentIndexing runs Zoekt indexing and embedding generation in two
// parallel goroutines. It uses a plain errgroup.Group (NOT errgroup.WithContext)
// so that one goroutine's failure does not cancel the other. Each goroutine
// writes exclusively to its own fields — no shared mutation.
//
// In production, the embedding goroutine calls p.generateEmbeddings with the
// provided chunks and IDs. In tests, p.embeddingFnOverride is used instead.
func (p *DefaultJobProcessor) runConcurrentIndexing(
	ctx context.Context,
	repoName string,
	workspacePath string,
	commitHash string,
	chunks []outbound.CodeChunk,
	indexingJobID uuid.UUID,
	repositoryID uuid.UUID,
) ConcurrencyResult {
	result := ConcurrencyResult{
		ChunksProcessed: len(chunks),
	}

	var eg errgroup.Group

	// Goroutine 1: Zoekt indexing
	eg.Go(func() error {
		start := time.Now()
		zoektCfg := &outbound.ZoektRepositoryConfig{
			Name:       repoName,
			Path:       workspacePath,
			CommitHash: commitHash,
			IndexDir:   p.zoektConfig.Indexing.IndexDir,
		}
		zoektResult, err := p.zoektIndexer.Index(ctx, zoektCfg)
		result.ZoektDuration = time.Since(start)
		result.ZoektResult = zoektResult
		result.ZoektErr = err
		if err != nil {
			slogger.Warn(ctx, "Zoekt indexing failed (partial failure allowed)", slogger.Fields{
				"repo":     repoName,
				"error":    err.Error(),
				"duration": result.ZoektDuration,
			})
		} else {
			slogger.Info(ctx, "Zoekt indexing completed", slogger.Fields{
				"repo":     repoName,
				"duration": result.ZoektDuration,
			})
		}
		// Always return nil so errgroup does not short-circuit the embedding goroutine.
		return nil
	})

	// Goroutine 2: Embedding generation
	eg.Go(func() error {
		start := time.Now()
		var err error
		if p.embeddingFnOverride != nil {
			// Test hook: use injected function instead of real embedding pipeline.
			err = p.embeddingFnOverride(ctx)
		} else {
			err = p.generateEmbeddings(ctx, indexingJobID, repositoryID, chunks)
		}
		result.EmbeddingDuration = time.Since(start)
		result.EmbeddingErr = err
		if err != nil {
			slogger.Warn(ctx, "Embedding generation failed (partial failure allowed)", slogger.Fields{
				"repo":     repoName,
				"error":    err.Error(),
				"duration": result.EmbeddingDuration,
			})
		} else {
			slogger.Info(ctx, "Embedding generation completed", slogger.Fields{
				"repo":     repoName,
				"duration": result.EmbeddingDuration,
			})
		}
		// Always return nil so errgroup does not short-circuit the Zoekt goroutine.
		return nil
	})

	// Both goroutines always return nil, so Wait never errors.
	_ = eg.Wait()

	return result
}
