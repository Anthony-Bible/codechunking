# Plan: Issue #42 — Concurrent Zoekt + Embedding Indexing

## Context

The worker pipeline currently indexes repositories sequentially (embedding generation only). Issue #42 requires running Zoekt indexing **concurrently** alongside embedding generation using `errgroup.Group`, with graceful handling of partial failures (one engine fails, job still succeeds). The foundation from issue #41 is complete: `ZoektIndexer` port, adapter, domain model fields, DB migration, and config all exist but are **not wired** into the worker pipeline.

## Approach

Integrate concurrent indexing directly into `DefaultJobProcessor` (not a separate service). The Zoekt indexer is an optional dependency — when nil or disabled via config, the pipeline falls back to current embedding-only behavior.

## Implementation Steps (with TDD agents)

### Step 1: `ConcurrencyResult` type — TDD Red Phase ✅
**Agent:** `@red-phase-tester`

Write failing tests for `ConcurrencyResult` value type:
- `BothSucceeded()` — true when both nil errors
- `AnySucceeded()` — true when at least one nil error
- `BothFailed()` — true when both non-nil errors

**New test file:** `internal/application/worker/concurrency_result_test.go`

### Step 2: `ConcurrencyResult` type — TDD Green Phase
**Agent:** `@green-phase-implementer`

**New file:** `internal/application/worker/concurrency_result.go`

```go
type ConcurrencyResult struct {
    ZoektResult       *outbound.ZoektIndexResult
    ZoektErr          error
    ZoektDuration     time.Duration
    EmbeddingErr      error
    EmbeddingDuration time.Duration
    ChunksProcessed   int
}
```
Minimal implementation to make tests pass.

### Step 3: `ConcurrencyResult` — TDD Refactor Phase
**Agent:** `@tdd-refactor-specialist`

Clean up implementation if needed.

### Step 4: `ConcurrencyResult` — TDD Review
**Agent:** `@tdd-review-agent`

Verify completeness, no unnecessary mocks, no skipped tests.

---

### Step 5: `JobProcessorZoektOptions` + `shouldRunConcurrentIndexing` — TDD Red Phase
**Agent:** `@red-phase-tester`

Write failing tests for:
- `shouldRunConcurrentIndexing()` returns false when indexer is nil
- `shouldRunConcurrentIndexing()` returns false when config disabled
- `shouldRunConcurrentIndexing()` returns true when indexer set + config enabled

**Test file:** `internal/application/worker/concurrent_indexing_test.go`

### Step 6: Add ZoektIndexer dependency — TDD Green Phase
**Agent:** `@green-phase-implementer`

**Modify:** `internal/application/worker/job_processor.go`
- Add `zoektIndexer outbound.ZoektIndexer` and `zoektConfig config.ZoektConfig` fields to `DefaultJobProcessor` struct (~line 114)
- Add `JobProcessorZoektOptions` struct (follows `JobProcessorBatchOptions` pattern)
- Add parameter to `NewDefaultJobProcessor` constructor (~line 135)
- Implement `shouldRunConcurrentIndexing() bool`

### Step 7: Refactor + Review
**Agent:** `@tdd-refactor-specialist` then `@tdd-review-agent`

---

### Step 8: `runConcurrentIndexing` — TDD Red Phase
**Agent:** `@red-phase-tester`

Write failing tests in `concurrent_indexing_test.go` for:
- Both engines succeed — verify both called, result has no errors
- Zoekt fails, embedding succeeds — `AnySucceeded()` true, `ZoektErr` set
- Embedding fails, Zoekt succeeds — `AnySucceeded()` true, `EmbeddingErr` set
- Both fail — `BothFailed()` true

Uses `MockZoektIndexer` (testify mock implementing `outbound.ZoektIndexer`).

### Step 9: Implement `runConcurrentIndexing` — TDD Green Phase
**Agent:** `@green-phase-implementer`

**Modify:** `internal/application/worker/job_processor.go`
- Uses plain `errgroup.Group` (NOT `errgroup.WithContext`) so one goroutine's failure doesn't cancel the other
- Goroutine 1: Builds `ZoektRepositoryConfig` from workspace path + config, calls `p.zoektIndexer.Index()`
- Goroutine 2: Calls existing `p.generateEmbeddings()`
- Each goroutine writes to its own field in `ConcurrencyResult` (no shared mutation)
- Get commit hash via `p.gitClient.GetCommitHash()` for Zoekt config

### Step 10: Refactor + Review
**Agent:** `@tdd-refactor-specialist` then `@tdd-review-agent`

---

### Step 11: Pipeline integration + finalization — TDD Red Phase
**Agent:** `@red-phase-tester`

Write failing tests for:
- `executeJobPipeline` uses concurrent path when enabled
- `executeJobPipeline` falls back to embedding-only when disabled
- `finalizeJobCompletion` with `ConcurrencyResult` updates per-engine status
- Partial success: at least one engine succeeded → repository marked completed
- Both failed → repository marked failed

### Step 12: Pipeline integration — TDD Green Phase
**Agent:** `@green-phase-implementer`

**Modify:** `internal/application/worker/job_processor.go`

1. Change `executeJobPipeline` return to `([]outbound.CodeChunk, *ConcurrencyResult, error)`, update `ProcessJob` accordingly (~line 207)
2. At ~line 326, replace sequential `generateEmbeddings()` with conditional branch:
   ```go
   if p.shouldRunConcurrentIndexing() {
       result := p.runConcurrentIndexing(...)
       if result.BothFailed() { /* mark failed, return error */ }
   } else {
       // existing embedding-only path (unchanged)
   }
   ```
3. Update `finalizeJobCompletion` (~line 496) to accept `*ConcurrencyResult`:
   - When non-nil: call `repo.UpdateZoektStatus()` + `repo.UpdateEmbeddingStatus()` + persist
   - When nil: existing behavior unchanged

### Step 13: Refactor + Review
**Agent:** `@tdd-refactor-specialist` then `@tdd-review-agent`

---

### Step 14: Wire ZoektIndexer in `cmd/worker.go`
**Modify:** `cmd/worker.go` (~line 370)

- Conditionally create `zoekt.NewIndexer()` when `cfg.Zoekt.ConcurrentIndexing.Enabled`
- Pass `&worker.JobProcessorZoektOptions{...}` to `NewDefaultJobProcessor`
- Import `"codechunking/internal/adapter/outbound/zoekt"`

### Step 15: Race condition validation
Run `go test -race ./internal/application/worker/... -timeout 30s` to validate no data races in concurrent paths.

## Key Files
- `internal/application/worker/job_processor.go` — main changes
- `internal/application/worker/concurrency_result.go` — new
- `internal/application/worker/concurrency_result_test.go` — new ✅
- `internal/application/worker/concurrent_indexing_test.go` — new
- `cmd/worker.go` — wiring
- `internal/port/outbound/zoekt_indexer.go` — existing (read only)
- `internal/adapter/outbound/zoekt/indexer.go` — existing (read only)
- `internal/domain/entity/repository.go` — existing (read only, already has methods)
- `internal/config/config.go` — existing (read only, already has config)

## Out of Scope (handled by #43)
- DTO `RepositoryResponse` zoekt fields
- Converter updates for API responses
- Search mode routing
- Hybrid ranking

## Verification
1. `go build ./...` — compiles
2. `go test ./internal/application/worker/... -v -timeout 30s` — unit tests pass
3. `go test ./internal/application/worker/... -race -timeout 30s` — no race conditions
4. `make lint` — passes linter
5. `make test` — all existing tests still pass
6. Manual: `make dev` + `make dev-worker`, submit a repo, verify logs show concurrent indexing with both engine statuses
