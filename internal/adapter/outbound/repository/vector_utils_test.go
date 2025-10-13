package repository

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalizeVector_NormalVector tests normalization of a typical vector.
// A normalized vector should have a magnitude (L2 norm) of 1.0 while preserving direction.
func TestNormalizeVector_NormalVector(t *testing.T) {
	// Given a vector with known magnitude
	vector := []float64{3.0, 4.0, 0.0}
	// Original magnitude = sqrt(3^2 + 4^2) = sqrt(9 + 16) = sqrt(25) = 5.0

	// When we normalize the vector
	normalized := NormalizeVector(vector)

	// Then the magnitude should be 1.0 (within floating point tolerance)
	magnitude := calculateMagnitude(normalized)
	assert.InDelta(t, 1.0, magnitude, 1e-10, "normalized vector should have magnitude of 1.0")

	// And the direction should be preserved (proportions between components)
	// Original ratio: 3:4:0, normalized should maintain this ratio
	require.Len(t, normalized, 3, "normalized vector should have same length as input")
	assert.InDelta(t, 0.6, normalized[0], 1e-10, "first component should be 3/5")
	assert.InDelta(t, 0.8, normalized[1], 1e-10, "second component should be 4/5")
	assert.InDelta(t, 0.0, normalized[2], 1e-10, "third component should be 0")

	// And the original vector should not be modified
	assert.Equal(t, 3.0, vector[0], "original vector should remain unchanged")
	assert.Equal(t, 4.0, vector[1], "original vector should remain unchanged")
}

// TestNormalizeVector_ZeroVector tests that a zero vector is handled safely.
// A zero vector has no direction, so it should be returned unchanged to avoid division by zero.
func TestNormalizeVector_ZeroVector(t *testing.T) {
	// Given a zero vector
	vector := []float64{0.0, 0.0, 0.0}

	// When we normalize the vector
	normalized := NormalizeVector(vector)

	// Then it should return the original zero vector (not NaN or panic)
	require.Len(t, normalized, 3, "normalized zero vector should have same length as input")
	assert.Equal(t, 0.0, normalized[0], "zero vector component should remain 0")
	assert.Equal(t, 0.0, normalized[1], "zero vector component should remain 0")
	assert.Equal(t, 0.0, normalized[2], "zero vector component should remain 0")

	// And no component should be NaN
	for i, val := range normalized {
		assert.False(t, math.IsNaN(val), "component %d should not be NaN", i)
	}
}

// TestNormalizeVector_UnitVector tests that an already normalized vector remains unchanged.
// A unit vector already has magnitude 1.0, so normalization should be idempotent.
func TestNormalizeVector_UnitVector(t *testing.T) {
	// Given a unit vector (already normalized)
	vector := []float64{1.0, 0.0, 0.0}
	// Magnitude = sqrt(1^2) = 1.0

	// When we normalize the vector
	normalized := NormalizeVector(vector)

	// Then it should remain essentially unchanged
	require.Len(t, normalized, 3, "normalized vector should have same length as input")
	assert.InDelta(t, 1.0, normalized[0], 1e-10, "unit vector should remain unchanged")
	assert.InDelta(t, 0.0, normalized[1], 1e-10, "unit vector should remain unchanged")
	assert.InDelta(t, 0.0, normalized[2], 1e-10, "unit vector should remain unchanged")

	// And magnitude should still be 1.0
	magnitude := calculateMagnitude(normalized)
	assert.InDelta(t, 1.0, magnitude, 1e-10, "unit vector should maintain magnitude of 1.0")
}

// TestNormalizeVector_LargeMagnitude tests normalization of vectors with large magnitude.
// This ensures the function works correctly with large values that Gemini might produce.
func TestNormalizeVector_LargeMagnitude(t *testing.T) {
	// Given a vector with large magnitude
	vector := []float64{1000.0, 2000.0, 3000.0}
	// Original magnitude = sqrt(1000^2 + 2000^2 + 3000^2) = sqrt(14000000) ≈ 3741.66

	// When we normalize the vector
	normalized := NormalizeVector(vector)

	// Then the magnitude should be 1.0
	magnitude := calculateMagnitude(normalized)
	assert.InDelta(t, 1.0, magnitude, 1e-10, "normalized vector with large magnitude should have magnitude of 1.0")

	// And the direction should be preserved (ratios maintained)
	require.Len(t, normalized, 3, "normalized vector should have same length as input")
	// The ratios 1000:2000:3000 = 1:2:3 should be preserved
	assert.InDelta(t, normalized[0]*2, normalized[1], 1e-10, "ratio between components should be preserved")
	assert.InDelta(t, normalized[0]*3, normalized[2], 1e-10, "ratio between components should be preserved")
}

// TestNormalizeVector_NegativeValues tests normalization with negative vector components.
// Negative values are valid and should be handled correctly.
func TestNormalizeVector_NegativeValues(t *testing.T) {
	// Given a vector with negative values
	vector := []float64{-3.0, 4.0, -5.0}
	// Original magnitude = sqrt(9 + 16 + 25) = sqrt(50) ≈ 7.071

	// When we normalize the vector
	normalized := NormalizeVector(vector)

	// Then the magnitude should be 1.0
	magnitude := calculateMagnitude(normalized)
	assert.InDelta(t, 1.0, magnitude, 1e-10, "normalized vector with negative values should have magnitude of 1.0")

	// And negative signs should be preserved
	require.Len(t, normalized, 3, "normalized vector should have same length as input")
	assert.Less(t, normalized[0], 0.0, "negative component should remain negative")
	assert.Greater(t, normalized[1], 0.0, "positive component should remain positive")
	assert.Less(t, normalized[2], 0.0, "negative component should remain negative")

	// And the direction (including signs) should be preserved
	expectedMagnitude := math.Sqrt(9 + 16 + 25)
	assert.InDelta(t, -3.0/expectedMagnitude, normalized[0], 1e-10, "negative component should be correctly normalized")
	assert.InDelta(t, 4.0/expectedMagnitude, normalized[1], 1e-10, "positive component should be correctly normalized")
	assert.InDelta(t, -5.0/expectedMagnitude, normalized[2], 1e-10, "negative component should be correctly normalized")
}

// TestNormalizeVector_EmptyVector tests handling of empty vector slice.
// An empty slice should be handled gracefully without panicking.
func TestNormalizeVector_EmptyVector(t *testing.T) {
	// Given an empty vector
	vector := []float64{}

	// When we normalize the vector
	normalized := NormalizeVector(vector)

	// Then it should return an empty vector without panicking
	assert.NotNil(t, normalized, "normalized empty vector should not be nil")
	assert.Empty(t, normalized, "normalized empty vector should remain empty")
}

// TestNormalizeVector_NilVector tests handling of nil vector.
// A nil vector should be handled gracefully without panicking.
func TestNormalizeVector_NilVector(t *testing.T) {
	// Given a nil vector
	var vector []float64

	// When we normalize the vector
	normalized := NormalizeVector(vector)

	// Then it should return nil or empty without panicking
	assert.True(t, normalized == nil || len(normalized) == 0, "normalized nil vector should be nil or empty")
}

// TestNormalizeVector_SingleDimension tests normalization of a 1D vector.
// Edge case to ensure the function works with single-dimension vectors.
func TestNormalizeVector_SingleDimension(t *testing.T) {
	// Given a single-dimension vector
	vector := []float64{5.0}

	// When we normalize the vector
	normalized := NormalizeVector(vector)

	// Then it should have magnitude 1.0
	require.Len(t, normalized, 1, "normalized vector should have same length as input")
	assert.InDelta(
		t,
		1.0,
		math.Abs(normalized[0]),
		1e-10,
		"single dimension normalized vector should have magnitude 1.0",
	)

	// And preserve the sign
	assert.Greater(t, normalized[0], 0.0, "positive single value should remain positive")
}

// TestNormalizeVector_GeminiDimensions tests with Gemini embedding dimensions.
// Gemini uses 768-dimensional vectors, so we test with realistic dimensions.
func TestNormalizeVector_GeminiDimensions(t *testing.T) {
	// Given a 768-dimensional vector (like Gemini embeddings)
	vector := make([]float64, 768)
	for i := range vector {
		vector[i] = float64(i%10 + 1) // Values between 1 and 10
	}

	// When we normalize the vector
	normalized := NormalizeVector(vector)

	// Then it should have the correct dimensions
	require.Len(t, normalized, 768, "normalized vector should maintain 768 dimensions")

	// And magnitude should be 1.0
	magnitude := calculateMagnitude(normalized)
	assert.InDelta(t, 1.0, magnitude, 1e-10, "768-dimensional normalized vector should have magnitude of 1.0")

	// And all components should be finite (not NaN or Inf)
	for i, val := range normalized {
		assert.False(t, math.IsNaN(val), "component %d should not be NaN", i)
		assert.False(t, math.IsInf(val, 0), "component %d should not be Inf", i)
	}
}

// TestNormalizeVector_VerySmallValues tests normalization with very small but non-zero values.
// This tests numerical stability with values close to zero.
func TestNormalizeVector_VerySmallValues(t *testing.T) {
	// Given a vector with very small values
	vector := []float64{1e-10, 2e-10, 3e-10}
	// These values are very small but not zero

	// When we normalize the vector
	normalized := NormalizeVector(vector)

	// Then the magnitude should be 1.0
	magnitude := calculateMagnitude(normalized)
	assert.InDelta(t, 1.0, magnitude, 1e-9, "normalized vector with small values should have magnitude of 1.0")

	// And the direction should be preserved
	require.Len(t, normalized, 3, "normalized vector should have same length as input")
	// Ratios 1:2:3 should be preserved
	assert.InDelta(t, normalized[0]*2, normalized[1], 1e-9, "ratio between small components should be preserved")
	assert.InDelta(t, normalized[0]*3, normalized[2], 1e-9, "ratio between small components should be preserved")
}

// TestNormalizeVector_MixedMagnitudes tests with components of vastly different magnitudes.
// This ensures the function handles vectors where some components dominate others.
func TestNormalizeVector_MixedMagnitudes(t *testing.T) {
	// Given a vector with mixed magnitudes
	vector := []float64{1000.0, 0.001, 500.0}

	// When we normalize the vector
	normalized := NormalizeVector(vector)

	// Then the magnitude should be 1.0
	magnitude := calculateMagnitude(normalized)
	assert.InDelta(t, 1.0, magnitude, 1e-10, "normalized vector with mixed magnitudes should have magnitude of 1.0")

	// And all components should be finite
	require.Len(t, normalized, 3, "normalized vector should have same length as input")
	for i, val := range normalized {
		assert.False(t, math.IsNaN(val), "component %d should not be NaN", i)
		assert.False(t, math.IsInf(val, 0), "component %d should not be Inf", i)
	}
}

// TestNormalizeVector_Idempotence tests that normalizing twice produces the same result.
// Applying normalization to an already normalized vector should not change it.
func TestNormalizeVector_Idempotence(t *testing.T) {
	// Given a vector
	vector := []float64{3.0, 4.0, 5.0}

	// When we normalize it twice
	firstNormalization := NormalizeVector(vector)
	secondNormalization := NormalizeVector(firstNormalization)

	// Then both normalizations should produce the same result
	require.Len(t, firstNormalization, len(secondNormalization), "both normalizations should have same length")
	for i := range firstNormalization {
		assert.InDelta(t, firstNormalization[i], secondNormalization[i], 1e-10,
			"normalizing twice should produce the same result at index %d", i)
	}

	// And both should have magnitude 1.0
	mag1 := calculateMagnitude(firstNormalization)
	mag2 := calculateMagnitude(secondNormalization)
	assert.InDelta(t, 1.0, mag1, 1e-10, "first normalization should have magnitude 1.0")
	assert.InDelta(t, 1.0, mag2, 1e-10, "second normalization should have magnitude 1.0")
}

// calculateMagnitude is a helper function to calculate the L2 norm (magnitude) of a vector.
// This is used in tests to verify that normalized vectors have magnitude 1.0.
func calculateMagnitude(vector []float64) float64 {
	if len(vector) == 0 {
		return 0.0
	}

	var sumOfSquares float64
	for _, val := range vector {
		sumOfSquares += val * val
	}

	return math.Sqrt(sumOfSquares)
}
