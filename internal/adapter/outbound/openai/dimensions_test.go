package openai

import (
	"math"
	"strings"
	"testing"
)

func TestTruncateAndRenorm_ProducesUnitVector(t *testing.T) {
	t.Parallel()
	// 3-4-5 triangle: prefix [3,4] has norm 5, so renormalized result is [0.6, 0.8].
	got := truncateAndRenorm([]float64{3, 4, 0, 0}, 2)
	want := []float64{0.6, 0.8}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestTruncateAndRenorm_ZeroVector(t *testing.T) {
	t.Parallel()
	got := truncateAndRenorm([]float64{0, 0, 0}, 2)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	for i, v := range got {
		if v != 0 {
			t.Errorf("got[%d] = %v, want 0 (zero-vector must not divide by zero)", i, v)
		}
	}
}

func TestTruncateAndRenorm_PreservesUnitNorm(t *testing.T) {
	t.Parallel()
	const srcDim = 4096
	const k = 768
	src := make([]float64, srcDim)
	// Fill with arbitrary non-trivial values; we don't require src to be unit
	// length — the post-truncation result must be unit length regardless.
	for i := range src {
		src[i] = math.Sin(float64(i) * 0.01)
	}
	out := truncateAndRenorm(src, k)
	if len(out) != k {
		t.Fatalf("len(out) = %d, want %d", len(out), k)
	}
	var sumSq float64
	for _, v := range out {
		sumSq += v * v
	}
	if math.Abs(math.Sqrt(sumSq)-1.0) > 1e-9 {
		t.Errorf("L2 norm = %v, want 1.0 (within 1e-9)", math.Sqrt(sumSq))
	}
}

func TestApplyDimensionPolicy_NoOpWhenEqual(t *testing.T) {
	t.Parallel()
	in := []float64{1, 2, 3, 4}
	out, err := applyDimensionPolicy(in, 4, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if &in[0] != &out[0] {
		t.Errorf("expected the same underlying slice on equality (no-op), got a copy")
	}
}

func TestApplyDimensionPolicy_PassthroughWhenZeroConfigured(t *testing.T) {
	t.Parallel()
	in := []float64{1, 2, 3, 4, 5}
	out, err := applyDimensionPolicy(in, 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != len(in) {
		t.Errorf("len(out) = %d, want %d", len(out), len(in))
	}
}

func TestApplyDimensionPolicy_RejectsLargerWhenTruncateDisabled(t *testing.T) {
	t.Parallel()
	_, err := applyDimensionPolicy([]float64{1, 2, 3, 4}, 2, false)
	if err == nil {
		t.Fatal("expected dimension-mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "dimension mismatch") {
		t.Errorf("expected error to mention 'dimension mismatch', got %q", err.Error())
	}
}

func TestApplyDimensionPolicy_RejectsSmallerEvenWhenTruncateEnabled(t *testing.T) {
	t.Parallel()
	_, err := applyDimensionPolicy([]float64{1, 2}, 4, true)
	if err == nil {
		t.Fatal("expected error when response is smaller than configured, got nil")
	}
	if !strings.Contains(err.Error(), "smaller") {
		t.Errorf("expected error to mention 'smaller', got %q", err.Error())
	}
}

func TestApplyDimensionPolicy_TruncatesAndRenormalizesWhenEnabled(t *testing.T) {
	t.Parallel()
	out, err := applyDimensionPolicy([]float64{3, 4, 100, 200}, 2, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	if math.Abs(out[0]-0.6) > 1e-9 || math.Abs(out[1]-0.8) > 1e-9 {
		t.Errorf("got %v, want [0.6, 0.8]", out)
	}
}
