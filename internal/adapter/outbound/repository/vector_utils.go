package repository

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	// zeroVectorThreshold defines the tolerance for treating a vector as having zero magnitude.
	// Vectors with magnitude below this threshold are considered zero vectors to avoid
	// division by zero during normalization. This value is consistent with floating-point
	// precision used in the test suite (1e-10).
	zeroVectorThreshold = 1e-10
)

// vectorToString converts a float64 slice to pgvector string format.
// This function ensures consistent vector formatting across all repository implementations.
// Format: [1.0,2.0,3.0] (pgvector standard format).
func VectorToString(vector []float64) string {
	if len(vector) == 0 {
		return "[]"
	}

	var sb strings.Builder
	sb.WriteByte('[')
	for i, val := range vector {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.FormatFloat(val, 'f', -1, 64))
	}
	sb.WriteByte(']')
	return sb.String()
}

// stringToVector converts a pgvector string format to float64 slice.
// This function handles parsing of pgvector format back to Go slice.
func StringToVector(vectorStr string) ([]float64, error) {
	if vectorStr == "" || vectorStr == "[]" {
		return []float64{}, nil
	}

	// Remove brackets
	vectorStr = strings.Trim(vectorStr, "[]")
	if vectorStr == "" {
		return []float64{}, nil
	}

	// Split by comma
	parts := strings.Split(vectorStr, ",")
	vector := make([]float64, len(parts))

	for i, part := range parts {
		val, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid vector element %q: %w", part, err)
		}
		vector[i] = val
	}

	return vector, nil
}

// NormalizeVector normalizes a vector to unit length (L2 norm = 1.0).
// This is required for Gemini embeddings which are not normalized by the API.
// For vectors with 768 or 1536 dimensions, Gemini documentation recommends normalization.
//
// The function:
// - Calculates the L2 norm (magnitude): sqrt(sum of squares)
// - Divides each component by the magnitude to create a unit vector
// - Returns the original vector unchanged if magnitude is zero (to avoid division by zero)
// - Returns nil or empty for nil or empty input vectors
//
// Normalization ensures that cosine similarity calculations work correctly
// and improves the quality of semantic search results.
func NormalizeVector(vector []float64) []float64 {
	// Handle nil vector
	if vector == nil {
		return nil
	}

	// Handle empty vector
	if len(vector) == 0 {
		return []float64{}
	}

	// Calculate magnitude (L2 norm): sqrt(sum of squares)
	var sumOfSquares float64
	for _, val := range vector {
		sumOfSquares += val * val
	}
	magnitude := math.Sqrt(sumOfSquares)

	// Handle zero vector (magnitude is effectively zero)
	if magnitude < zeroVectorThreshold {
		// Return copy of original vector to avoid division by zero
		result := make([]float64, len(vector))
		copy(result, vector)
		return result
	}

	// Normalize: divide each component by magnitude
	normalized := make([]float64, len(vector))
	for i, val := range vector {
		normalized[i] = val / magnitude
	}

	return normalized
}
