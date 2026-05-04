package openai

import (
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrBatchNotSupported is returned by every method on BatchStub. The wiring
// layer in cmd/worker.go inspects the embedding service's batch capability
// at startup; when async batches are unsupported the BatchSubmitter and
// BatchPoller goroutines are not launched, and embedding work is routed
// through the synchronous /v1/embeddings array endpoint instead.
//
// The async file-based Batches API is intentionally not implemented for
// OpenAI in this iteration: most OpenAI-compatible servers (vLLM, Ollama,
// Azure OpenAI for many models) do not implement it, so providing only sync
// keeps a single adapter usable across the entire OpenAI-compatible
// ecosystem. Adding it later does not change any existing call sites.
var ErrBatchNotSupported = errors.New(
	"openai adapter: file-based Batches API is not implemented; use the synchronous embeddings endpoint",
)

// BatchStub satisfies outbound.BatchEmbeddingService by returning
// ErrBatchNotSupported from every method.
type BatchStub struct{}

// NewBatchStub returns a BatchStub.
func NewBatchStub() *BatchStub { return &BatchStub{} }

// CreateBatchEmbeddingJob always returns ErrBatchNotSupported; the OpenAI
// adapter does not implement the file-based async Batches API.
func (s *BatchStub) CreateBatchEmbeddingJob(
	_ context.Context, _ []string, _ outbound.EmbeddingOptions, _ uuid.UUID,
) (*outbound.BatchEmbeddingJob, error) {
	return nil, ErrBatchNotSupported
}

// CreateBatchEmbeddingJobWithRequests always returns ErrBatchNotSupported.
func (s *BatchStub) CreateBatchEmbeddingJobWithRequests(
	_ context.Context, _ []*outbound.BatchEmbeddingRequest, _ outbound.EmbeddingOptions, _ uuid.UUID,
) (*outbound.BatchEmbeddingJob, error) {
	return nil, ErrBatchNotSupported
}

// CreateBatchEmbeddingJobWithFile always returns ErrBatchNotSupported.
func (s *BatchStub) CreateBatchEmbeddingJobWithFile(
	_ context.Context, _ []*outbound.BatchEmbeddingRequest, _ outbound.EmbeddingOptions, _ uuid.UUID, _ string,
) (*outbound.BatchEmbeddingJob, error) {
	return nil, ErrBatchNotSupported
}

// GetBatchJobStatus always returns ErrBatchNotSupported.
func (s *BatchStub) GetBatchJobStatus(_ context.Context, _ string) (*outbound.BatchEmbeddingJob, error) {
	return nil, ErrBatchNotSupported
}

// GetBatchJobStatuses always returns ErrBatchNotSupported.
func (s *BatchStub) GetBatchJobStatuses(
	_ context.Context, _ []string,
) (map[string]*outbound.BatchEmbeddingJob, error) {
	return nil, ErrBatchNotSupported
}

// ListBatchJobs always returns ErrBatchNotSupported.
func (s *BatchStub) ListBatchJobs(
	_ context.Context, _ *outbound.BatchJobFilter,
) ([]*outbound.BatchEmbeddingJob, error) {
	return nil, ErrBatchNotSupported
}

// GetBatchJobResults always returns ErrBatchNotSupported.
func (s *BatchStub) GetBatchJobResults(_ context.Context, _ string) ([]*outbound.EmbeddingResult, error) {
	return nil, ErrBatchNotSupported
}

// CancelBatchJob always returns ErrBatchNotSupported.
func (s *BatchStub) CancelBatchJob(_ context.Context, _ string) error { return ErrBatchNotSupported }

// DeleteBatchJob always returns ErrBatchNotSupported.
func (s *BatchStub) DeleteBatchJob(_ context.Context, _ string) error { return ErrBatchNotSupported }

// WaitForBatchJob always returns ErrBatchNotSupported.
func (s *BatchStub) WaitForBatchJob(
	_ context.Context, _ string, _ time.Duration,
) (*outbound.BatchEmbeddingJob, error) {
	return nil, ErrBatchNotSupported
}
