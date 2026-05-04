package openai

import (
	"fmt"
	"math"
)

// truncateAndRenorm returns the first k coordinates of vec, L2-renormalized
// so the result is unit length. Zero-vector inputs return a zero-prefix
// rather than dividing by zero (zero embeddings can occur from poorly
// tokenized inputs and must not crash the pipeline).
//
// Caller must ensure k <= len(vec); the function panics otherwise because
// growing a vector is not a math operation we can perform.
func truncateAndRenorm(vec []float64, k int) []float64 {
	if k > len(vec) {
		panic(fmt.Sprintf("openai: truncateAndRenorm: k=%d > len(vec)=%d", k, len(vec)))
	}
	out := make([]float64, k)
	copy(out, vec[:k])

	var sumSquares float64
	for _, v := range out {
		sumSquares += v * v
	}
	if sumSquares == 0 {
		return out
	}
	norm := math.Sqrt(sumSquares)
	for i := range out {
		out[i] /= norm
	}
	return out
}

// applyDimensionPolicy enforces the configured-vs-returned dimension contract
// for a single vector. When configuredDim == 0, no policy is applied (some
// callers pass through whatever the server returned).
//
// Behavior matrix:
//
//	returned == configured           → no-op, returns vec unchanged
//	returned >  configured && trunc  → prefix-truncate + L2 renormalize
//	returned >  configured && !trunc → error (strict mode footgun guard)
//	returned <  configured           → error in BOTH modes (cannot grow signal)
func applyDimensionPolicy(vec []float64, configuredDim int, truncate bool) ([]float64, error) {
	if configuredDim == 0 {
		return vec, nil
	}
	switch {
	case len(vec) == configuredDim:
		return vec, nil
	case len(vec) > configuredDim:
		if !truncate {
			return nil, fmt.Errorf(
				"openai: response dimension mismatch: want %d, got %d (does the server support the dimensions parameter? set truncate_dimensions: true if the model is MRL-trained)",
				configuredDim, len(vec),
			)
		}
		return truncateAndRenorm(vec, configuredDim), nil
	default:
		return nil, fmt.Errorf(
			"openai: response dimension smaller than configured: want %d, got %d (cannot synthesize signal)",
			configuredDim, len(vec),
		)
	}
}
