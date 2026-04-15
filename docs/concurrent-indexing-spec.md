# Concurrent Indexing Architecture Specification

## Overview

This specification describes the design for concurrent indexing of both Zoekt (full-text search) and embedding (semantic search) engines in the CodeChunking system. The architecture enables parallel processing of both indexing pipelines after repository parsing, reducing total indexing time while providing graceful handling of partial failures.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                     Repository Indexing Job                      │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────────────┐
│  Stage 1: Sequential Initialization                             │
│  - Clone repository (if needed)                                 │
│  - Parse repository with tree-sitter                            │
│  - Extract code chunks                                          │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────────────┐
│                    Stage 2: Concurrent Indexing                   │
│                    ┌────────────────────┐                        │
│                    │   errgroup.Group   │                        │
│                    │   with context     │                        │
│                    └─────────┬──────────┘                        │
│                              │                                   │
│          ┌───────────────────┴───────────────────┐              │
│          │                                       │              │
│          v                                       v              │
│  ┌───────────────────────┐         ┌───────────────────────┐  │
│  │   Goroutine 1:        │         │   Goroutine 2:        │  │
│  │   Zoekt Indexer      │         │   Embedding Generator │ │
│  │                       │         │                       │  │
│  │ - Build index config  │         │ - Generate embeddings │  │
│  │ - Call zoekt-git-index│         │ - Batch or sequential │  │
│  │ - Write shard files   │         │ - Store in pgvector   │  │
│  └───────────┬───────────┘         └───────────┬───────────┘  │
│              │                                 │               │
│              │                                 │               │
│              └─────────────┬───────────────────┘               │
│                            │                                   │
└────────────────────────────┼───────────────────────────────────┘
                             │
                             v
                   ┌─────────────────────┐
                   │  errgroup.Wait()    │
                   │  Aggregate errors   │
                   └─────────┬───────────┘
                             │
                             v
┌─────────────────────────────────────────────────────────────────┐
│           Stage 3: Status Aggregation & Update                   │
│  - Determine Zoekt index status (completed/failed/partial)      │
│  - Determine Embedding index status (completed/failed/partial)   │
│  - Update Repository entity with both statuses                  │
│  - Calculate shard counts, commit hashes for Zoekt               │
│  - Calculate chunk counts for Embeddings                         │
└─────────────────────────────────────────────────────────────────┘
```

## Goroutine Coordination

### Using errgroup.Group

```go
package service

import (
    "golang.org/x/sync/errgroup"
)

type ConcurrentIndexer struct {
    zoektIndexer   outbound.ZoektIndexer
    embeddingService outbound.EmbeddingService
    repoRepo       outbound.RepositoryRepository
    chunkRepo      outbound.ChunkRepository
}

func (c *ConcurrentIndexer) Index(ctx context.Context, repo *Repository, chunks []Chunk) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    g, ctx := errgroup.WithContext(ctx)

    // Channel to communicate Zoekt result
    zoektResult := make(chan *ZoektIndexResult, 1)

    // Goroutine 1: Zoekt indexing
    g.Go(func() error {
        result, err := c.indexWithZoekt(ctx, repo)
        if err != nil {
            return fmt.Errorf("zoekt indexing failed: %w", err)
        }
        zoektResult <- result
        return nil
    })

    // Goroutine 2: Embedding generation
    embeddingResult := make(chan *EmbeddingResult, 1)
    g.Go(func() error {
        result, err := c.indexWithEmbeddings(ctx, chunks)
        if err != nil {
            return fmt.Errorf("embedding generation failed: %w", err)
        }
        embeddingResult <- result
        return nil
    })

    // Wait for both goroutines to complete
    if err := g.Wait(); err != nil {
        // Handle partial failure scenarios
        return c.handlePartialFailure(ctx, repo, zoektResult, embeddingResult, err)
    }

    // Both succeeded - aggregate results
    return c.aggregateSuccess(ctx, repo, <-zoektResult, <-embeddingResult)
}

func (c *ConcurrentIndexer) indexWithZoekt(ctx context.Context, repo *Repository) (*ZoektIndexResult, error) {
    config := &outbound.ZoektRepositoryConfig{
        Name:       repo.Name(),
        Path:       repo.LocalPath(),
        CommitHash: repo.LatestCommitHash(),
        Branch:     repo.DefaultBranch(),
        IndexDir:   c.getZoektIndexDir(),
        // Additional config from environment/file
    }

    return c.zoektIndexer.Index(ctx, config)
}

func (c *ConcurrentIndexer) indexWithEmbeddings(ctx context.Context, chunks []Chunk) (*EmbeddingResult, error) {
    return c.embeddingService.GenerateBatch(ctx, chunks)
}
```

## Failure Handling

### Scenario 1: Zoekt Fails, Embedding Succeeds

```go
func (c *ConcurrentIndexer) handlePartialFailure(
    ctx context.Context,
    repo *Repository,
    zoektResult chan *ZoektIndexResult,
    embeddingResult chan *EmbeddingResult,
    primaryErr error,
) error {
    // Check which engine completed successfully
    zoektOk := false
    embeddingOk := false

    var zoektRes *ZoektIndexResult
    var embeddingRes *EmbeddingResult

    select {
    case zoektRes = <-zoektResult:
        zoektOk = true
    default:
    }

    select {
    case embeddingRes = <-embeddingResult:
        embeddingOk = true
    default:
    }

    // Determine job-level result
    if zoektOk || embeddingOk {
        // Job succeeds (partial success), mark failed engine status
        zoektStatus := valueobject.Failed
        if zoektOk {
            zoektStatus = valueobject.Completed
        }

        embeddingStatus := valueobject.Failed
        if embeddingOk {
            embeddingStatus = valueobject.Completed
        }

        // Update repository with mixed status
        return c.updateRepositoryStatus(ctx, repo, zoektStatus, embeddingStatus, zoektRes, embeddingRes)
    }

    // Both failed - job fails completely
    return fmt.Errorf("all indexing engines failed: %w", primaryErr)
}
```

### Scenario 2: Embedding Fails, Zoekt Succeeds

Same as Scenario 1, but with reversed states.

### Scenario 3: Both Fail

Job fails completely. Retry according to worker retry configuration.

## Repository Status Aggregation

```go
func (c *ConcurrentIndexer) aggregateSuccess(
    ctx context.Context,
    repo *Repository,
    zoektResult *ZoektIndexResult,
    embeddingResult *EmbeddingResult,
) error {
    // Update Zoekt status
    if zoektResult != nil && zoektResult.ShardCount > 0 {
        repo.UpdateZoektStatus(
            valueobject.Completed,
            zoektResult.ShardCount,
            repo.LatestCommitHash(),
        )
    } else {
        repo.UpdateZoektStatus(valueobject.Failed, 0, nil)
    }

    // Update Embedding status
    if embeddingResult != nil && embeddingResult.ChunkCount > 0 {
        repo.UpdateEmbeddingStatus(valueobject.Completed)
    } else {
        repo.UpdateEmbeddingStatus(valueobject.Failed)
    }

    // Save updated repository
    return c.repoRepo.Update(ctx, repo)
}

func (c *ConcurrentIndexer) updateRepositoryStatus(
    ctx context.Context,
    repo *Repository,
    zoektStatus valueobject.ZoektIndexStatus,
    embeddingStatus valueobject.EmbeddingIndexStatus,
    zoektRes *ZoektIndexResult,
    embeddingRes *EmbeddingResult,
) error {
    // Update Zoekt metadata
    shardCount := 0
    commit := ""
    if zoektRes != nil {
        shardCount = zoektRes.ShardCount
        commit = repo.LatestCommitHash()
    }
    repo.UpdateZoektStatus(zoektStatus, shardCount, &commit)

    // Update Embedding status
    repo.UpdateEmbeddingStatus(embeddingStatus)

    // Save
    return c.repoRepo.Update(ctx, repo)
}
```

## Repository Entity Methods

### IsFullyIndexed()

```go
func (r *Repository) IsFullyIndexed() bool {
    return r.zoektIndexStatus == valueobject.Completed &&
        r.embeddingIndexStatus == valueobject.Completed
}
```

### IsPartiallyIndexed()

```go
func (r *Repository) IsPartiallyIndexed() bool {
    return (r.zoektIndexStatus == valueobject.Completed &&
            r.embeddingIndexStatus != valueobject.Completed) ||
        (r.zoektIndexStatus != valueobject.Completed &&
            r.embeddingIndexStatus == valueobject.Completed)
}
```

### AvailableEngines()

```go
func (r *Repository) AvailableEngines() []string {
    engines := []string{}

    if r.zoektIndexStatus == valueobject.Completed {
        engines = append(engines, "zoekt")
    }

    if r.embeddingIndexStatus == valueobject.Completed {
        engines = append(engines, "embedding")
    }

    // Defaults to both if both are incomplete but not failed
    if len(engines) == 0 &&
        r.zoektIndexStatus != valueobject.Failed &&
        r.embeddingIndexStatus != valueobject.Failed {
        engines = []string{"zoekt", "embedding"}
    }

    return engines
}
```

## Configuration

### Concurrent Indexing Settings

```yaml
zoekt:
  concurrent_indexing:
    enabled: true
    max_concurrent_jobs: 5    # Max parallel repository indexing jobs
    retry_attempts: 3         # Retry attempts for failed indexing
```

### Worker Configuration

```yaml
worker:
  concurrency: 5              # Worker goroutines for job processing
  job_timeout: 30m           # Overall timeout per job
```

## Error Propagation

### Context Cancellation

When one goroutine fails with an unrecoverable error, `errgroup.Wait()` will:
1. Cancel the context shared by all goroutines
2. Wait for ongoing goroutines to handle cancellation gracefully
3. Return the first non-nil error encountered

Example cancellation handling:

```go
func (c *ConcurrentIndexer) indexWithZoekt(ctx context.Context, repo *Repository) (*ZoektIndexResult, error) {
    // Check if context is already cancelled
    if err := ctx.Err(); err != nil {
        return nil, err
    }

    // Execute indexing with context-aware timeout
    result, err := c.zoektIndexer.Index(ctx, config)
    if ctx.Err() != nil {
        // Context was cancelled by another goroutine
        return nil, ctx.Err()
    }
    return result, err
}
```

## Performance Considerations

### Resource Utilization

- **Parallelism**: Both engines run simultaneously across available CPU cores
- **I/O Overlap**: Zoekt shard writes and embedding API calls can overlap
- **Memory**: Need to ensure adequate memory for concurrent operations

### Time Savings

With typical repository indexing:
- Sequential: `ZoektTime + EmbeddingTime`
- Concurrent: `max(ZoektTime, EmbeddingTime)`

Expected reduction: 30-50% for balanced workloads.

### Limits

- `zoekt.concurrent_indexing.max_concurrent_jobs` limits parallel repository indexing
- Worker concurrency controls overall job parallelism
- System resources should be monitored for bottlenecks

## Implementation Checklist

### Phase 1 (Current Issue)
- [x] Define ZoektIndexer port interface
- [x] Define ZoektSearcher port interface
- [x] Implement Zoekt gRPC client adapter
- [x] Implement Zoekt indexer adapter (CLI-based)
- [x] Add search mode parameter
- [ ] Write concurrent indexing spec (this document)

### Phase 2 (Future)
- [ ] Implement ConcurrentIndexer service
- [ ] Integrate with worker pipeline
- [ ] Add dual status tracking to repository operations
- [ ] Update job processing to use concurrent indexing

### Phase 3 (Future)
- [ ] Implement search mode routing
- [ ] Add hybrid search result ranking
- [ ] Implement incremental indexing
- [ ] Add indexing job monitoring UI

## References

- [errgroup package](https://pkg.go.dev/golang.org/x/sync/errgroup)
- [Zoekt documentation](https://github.com/sourcegraph/zoekt)
- [Go context package](https://pkg.go.dev/context)