package worker

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// shouldRunConcurrentIndexing reports whether both Zoekt and embedding indexing
// should run concurrently. Returns false when the Zoekt indexer is nil or when
// concurrent indexing is disabled in config.
func (p *DefaultJobProcessor) shouldRunConcurrentIndexing() bool {
	return p.zoektIndexer != nil && p.zoektConfig.ConcurrentIndexing.Enabled
}

// runConcurrentIndexing runs Zoekt indexing and embedding generation concurrently.
// Each goroutine writes exclusively to its own fields — no shared mutation.
// Errors are captured in ConcurrencyResult so a failure in one engine does not
// abort the other.
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
		CommitHash:      commitHash,
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
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
	}()

	go func() {
		defer wg.Done()
		start := time.Now()
		err := p.embeddingFn(ctx, indexingJobID, repositoryID, chunks)
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
	}()

	wg.Wait()

	return result
}
