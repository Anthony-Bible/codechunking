package ranking

import (
	"codechunking/internal/application/dto"
)

// CalculateRRFScore computes the Reciprocal Rank Fusion score for a given rank.
// The formula used is 1 / (k + rank), where k is the standard constant 60.0.
// rank is 1-based (first result has rank 1).
func CalculateRRFScore(rank int) float64 {
	const k = 60.0
	return 1.0 / (k + float64(rank))
}

// ApplyRRFRanks applies RRF scoring to a slice of search results based on their position.
// It modifies the results in-place, updating both EngineScore and SimilarityScore.
func ApplyRRFRanks(results []dto.SearchResultDTO) {
	for i := range results {
		score := CalculateRRFScore(i + 1)
		results[i].EngineScore = score
		results[i].SimilarityScore = score
	}
}
