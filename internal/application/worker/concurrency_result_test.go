package worker

import (
	"codechunking/internal/port/outbound"
	"errors"
	"testing"
	"time"
)

// sentinel errors used across all subtests
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

// ---------------------------------------------------------------------------
// BothSucceeded
// ---------------------------------------------------------------------------

// TestConcurrencyResult_BothSucceeded_BothNil verifies that BothSucceeded
// returns true when both ZoektErr and EmbeddingErr are nil.
func TestConcurrencyResult_BothSucceeded_BothNil(t *testing.T) {
	r := makeResult(nil, nil)

	got := r.BothSucceeded()

	if !got {
		t.Errorf("BothSucceeded() = %v; want true when both errors are nil", got)
	}
}

// TestConcurrencyResult_BothSucceeded_ZoektErrSet verifies that BothSucceeded
// returns false when ZoektErr is non-nil and EmbeddingErr is nil.
func TestConcurrencyResult_BothSucceeded_ZoektErrSet(t *testing.T) {
	r := makeResult(errZoekt, nil)

	got := r.BothSucceeded()

	if got {
		t.Errorf("BothSucceeded() = %v; want false when ZoektErr is non-nil", got)
	}
}

// TestConcurrencyResult_BothSucceeded_EmbeddingErrSet verifies that BothSucceeded
// returns false when EmbeddingErr is non-nil and ZoektErr is nil.
func TestConcurrencyResult_BothSucceeded_EmbeddingErrSet(t *testing.T) {
	r := makeResult(nil, errEmbedding)

	got := r.BothSucceeded()

	if got {
		t.Errorf("BothSucceeded() = %v; want false when EmbeddingErr is non-nil", got)
	}
}

// TestConcurrencyResult_BothSucceeded_BothErrSet verifies that BothSucceeded
// returns false when both ZoektErr and EmbeddingErr are non-nil.
func TestConcurrencyResult_BothSucceeded_BothErrSet(t *testing.T) {
	r := makeResult(errZoekt, errEmbedding)

	got := r.BothSucceeded()

	if got {
		t.Errorf("BothSucceeded() = %v; want false when both errors are non-nil", got)
	}
}

// ---------------------------------------------------------------------------
// AnySucceeded
// ---------------------------------------------------------------------------

// TestConcurrencyResult_AnySucceeded_BothNil verifies that AnySucceeded returns
// true when both errors are nil (both operations succeeded).
func TestConcurrencyResult_AnySucceeded_BothNil(t *testing.T) {
	r := makeResult(nil, nil)

	got := r.AnySucceeded()

	if !got {
		t.Errorf("AnySucceeded() = %v; want true when both errors are nil", got)
	}
}

// TestConcurrencyResult_AnySucceeded_OnlyZoektNil verifies that AnySucceeded
// returns true when only ZoektErr is nil (Zoekt succeeded, embedding failed).
func TestConcurrencyResult_AnySucceeded_OnlyZoektNil(t *testing.T) {
	r := makeResult(nil, errEmbedding)

	got := r.AnySucceeded()

	if !got {
		t.Errorf("AnySucceeded() = %v; want true when only ZoektErr is nil", got)
	}
}

// TestConcurrencyResult_AnySucceeded_OnlyEmbeddingNil verifies that AnySucceeded
// returns true when only EmbeddingErr is nil (embedding succeeded, Zoekt failed).
func TestConcurrencyResult_AnySucceeded_OnlyEmbeddingNil(t *testing.T) {
	r := makeResult(errZoekt, nil)

	got := r.AnySucceeded()

	if !got {
		t.Errorf("AnySucceeded() = %v; want true when only EmbeddingErr is nil", got)
	}
}

// TestConcurrencyResult_AnySucceeded_BothErrSet verifies that AnySucceeded
// returns false when both ZoektErr and EmbeddingErr are non-nil.
func TestConcurrencyResult_AnySucceeded_BothErrSet(t *testing.T) {
	r := makeResult(errZoekt, errEmbedding)

	got := r.AnySucceeded()

	if got {
		t.Errorf("AnySucceeded() = %v; want false when both errors are non-nil", got)
	}
}

// ---------------------------------------------------------------------------
// BothFailed
// ---------------------------------------------------------------------------

// TestConcurrencyResult_BothFailed_BothErrSet verifies that BothFailed returns
// true when both ZoektErr and EmbeddingErr are non-nil.
func TestConcurrencyResult_BothFailed_BothErrSet(t *testing.T) {
	r := makeResult(errZoekt, errEmbedding)

	got := r.BothFailed()

	if !got {
		t.Errorf("BothFailed() = %v; want true when both errors are non-nil", got)
	}
}

// TestConcurrencyResult_BothFailed_ZoektErrNil verifies that BothFailed returns
// false when ZoektErr is nil (Zoekt succeeded).
func TestConcurrencyResult_BothFailed_ZoektErrNil(t *testing.T) {
	r := makeResult(nil, errEmbedding)

	got := r.BothFailed()

	if got {
		t.Errorf("BothFailed() = %v; want false when ZoektErr is nil", got)
	}
}

// TestConcurrencyResult_BothFailed_EmbeddingErrNil verifies that BothFailed
// returns false when EmbeddingErr is nil (embedding succeeded).
func TestConcurrencyResult_BothFailed_EmbeddingErrNil(t *testing.T) {
	r := makeResult(errZoekt, nil)

	got := r.BothFailed()

	if got {
		t.Errorf("BothFailed() = %v; want false when EmbeddingErr is nil", got)
	}
}

// TestConcurrencyResult_BothFailed_BothNil verifies that BothFailed returns false
// when both errors are nil (both operations succeeded).
func TestConcurrencyResult_BothFailed_BothNil(t *testing.T) {
	r := makeResult(nil, nil)

	got := r.BothFailed()

	if got {
		t.Errorf("BothFailed() = %v; want false when both errors are nil", got)
	}
}
