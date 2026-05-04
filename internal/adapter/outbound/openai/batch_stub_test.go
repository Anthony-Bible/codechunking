package openai

import (
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestBatchStub_ImplementsInterface(_ *testing.T) {
	var _ outbound.BatchEmbeddingService = (*BatchStub)(nil)
}

func TestClient_ImplementsEmbeddingService(_ *testing.T) {
	var _ outbound.EmbeddingService = (*Client)(nil)
}

func TestBatchStub_AllMethodsReturnErrBatchNotSupported(t *testing.T) {
	s := NewBatchStub()
	ctx := context.Background()

	checks := []struct {
		name string
		err  error
	}{
		{"CreateBatchEmbeddingJob", mustErr(s.CreateBatchEmbeddingJob(ctx, nil, outbound.EmbeddingOptions{}, uuid.Nil))},
		{"CreateBatchEmbeddingJobWithRequests", mustErr(s.CreateBatchEmbeddingJobWithRequests(ctx, nil, outbound.EmbeddingOptions{}, uuid.Nil))},
		{"CreateBatchEmbeddingJobWithFile", mustErr(s.CreateBatchEmbeddingJobWithFile(ctx, nil, outbound.EmbeddingOptions{}, uuid.Nil, ""))},
		{"GetBatchJobStatus", mustErr(s.GetBatchJobStatus(ctx, ""))},
		{"GetBatchJobStatuses", mustErrMap(s.GetBatchJobStatuses(ctx, nil))},
		{"ListBatchJobs", mustErrSlice(s.ListBatchJobs(ctx, nil))},
		{"GetBatchJobResults", mustErrSliceR(s.GetBatchJobResults(ctx, ""))},
		{"CancelBatchJob", s.CancelBatchJob(ctx, "")},
		{"DeleteBatchJob", s.DeleteBatchJob(ctx, "")},
		{"WaitForBatchJob", mustErr(s.WaitForBatchJob(ctx, "", 0))},
	}
	for _, ch := range checks {
		if !errors.Is(ch.err, ErrBatchNotSupported) {
			t.Errorf("%s: expected ErrBatchNotSupported, got %v", ch.name, ch.err)
		}
	}
}

func mustErr[T any](_ T, err error) error                                  { return err }
func mustErrMap(_ map[string]*outbound.BatchEmbeddingJob, err error) error { return err }
func mustErrSlice(_ []*outbound.BatchEmbeddingJob, err error) error        { return err }
func mustErrSliceR(_ []*outbound.EmbeddingResult, err error) error         { return err }
