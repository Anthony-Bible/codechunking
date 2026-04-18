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
	var (
		wg                sync.WaitGroup
		zoektResult       *outbound.ZoektIndexResult
		zoektErr          error
		zoektDuration     time.Duration
		embeddingErr      error
		embeddingDuration time.Duration
	)

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
		zoektResult, zoektErr = p.zoektIndexer.Index(ctx, zoektCfg)
		zoektDuration = time.Since(start)
		if zoektErr != nil {
			slogger.Warn(ctx, "Zoekt indexing failed (partial failure allowed)", slogger.Fields{
				"repo":     repoName,
				"error":    zoektErr.Error(),
				"duration": zoektDuration,
			})
		} else {
			slogger.Info(ctx, "Zoekt indexing completed", slogger.Fields{
				"repo":     repoName,
				"duration": zoektDuration,
			})
		}
	}()

	go func() {
		defer wg.Done()
		start := time.Now()
		embeddingErr = p.embeddingFn(ctx, indexingJobID, repositoryID, chunks)
		embeddingDuration = time.Since(start)
		if embeddingErr != nil {
			slogger.Warn(ctx, "Embedding generation failed (partial failure allowed)", slogger.Fields{
				"repo":     repoName,
				"error":    embeddingErr.Error(),
				"duration": embeddingDuration,
			})
		} else {
			slogger.Info(ctx, "Embedding generation completed", slogger.Fields{
				"repo":     repoName,
				"duration": embeddingDuration,
			})
		}
	}()

	wg.Wait()

	return ConcurrencyResult{
		ChunksProcessed:   len(chunks),
		CommitHash:        commitHash,
		ZoektResult:       zoektResult,
		ZoektErr:          zoektErr,
		ZoektDuration:     zoektDuration,
		EmbeddingErr:      embeddingErr,
		EmbeddingDuration: embeddingDuration,
	}
}
