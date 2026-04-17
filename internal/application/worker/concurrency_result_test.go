package worker

import (
	"codechunking/internal/port/outbound"
	"errors"
	"testing"
	"time"
)

// sentinel errors used across all subtests.
var (
	errZoekt     = errors.New("zoekt indexing failed")
	errEmbedding = errors.New("embedding generation failed")
)

// makeResult is a small helper that constructs a ConcurrencyResult so that
// individual test cases stay focused on the single field(s) they care about.
func makeResult(zoektErr, embeddingErr error) ConcurrencyResult {
	return ConcurrencyResult{
		ZoektResult: &outbound.ZoektIndexResult{
			FileCount:  10,
			ShardCount: 2,
			Duration:   50 * time.Millisecond,
		},
		ZoektErr:          zoektErr,
		ZoektDuration:     50 * time.Millisecond,
		EmbeddingErr:      embeddingErr,
		EmbeddingDuration: 100 * time.Millisecond,
		ChunksProcessed:   42,
	}
}

func TestConcurrencyResult_BothSucceeded(t *testing.T) {
	tests := []struct {
		name         string
		zoektErr     error
		embeddingErr error
		want         bool
	}{
		{"both nil", nil, nil, true},
		{"zoekt error only", errZoekt, nil, false},
		{"embedding error only", nil, errEmbedding, false},
		{"both errors set", errZoekt, errEmbedding, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := makeResult(tc.zoektErr, tc.embeddingErr)
			if got := r.BothSucceeded(); got != tc.want {
				t.Errorf("BothSucceeded() = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestConcurrencyResult_AnySucceeded(t *testing.T) {
	tests := []struct {
		name         string
		zoektErr     error
		embeddingErr error
		want         bool
	}{
		{"both nil", nil, nil, true},
		{"zoekt error only", errZoekt, nil, true},
		{"embedding error only", nil, errEmbedding, true},
		{"both errors set", errZoekt, errEmbedding, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := makeResult(tc.zoektErr, tc.embeddingErr)
			if got := r.AnySucceeded(); got != tc.want {
				t.Errorf("AnySucceeded() = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestConcurrencyResult_BothFailed(t *testing.T) {
	tests := []struct {
		name         string
		zoektErr     error
		embeddingErr error
		want         bool
	}{
		{"both errors set", errZoekt, errEmbedding, true},
		{"zoekt nil only", nil, errEmbedding, false},
		{"embedding nil only", errZoekt, nil, false},
		{"both nil", nil, nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := makeResult(tc.zoektErr, tc.embeddingErr)
			if got := r.BothFailed(); got != tc.want {
				t.Errorf("BothFailed() = %v; want %v", got, tc.want)
			}
		})
	}
}
