package repository

import (
	"fmt"
	"strconv"
	"strings"
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
