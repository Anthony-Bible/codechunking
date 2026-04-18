package service

import (
	"codechunking/internal/application/dto"
	"math"
	"sort"
)

// HybridRanker merges and ranks results from semantic and text search engines.
type HybridRanker interface {
	Rank(semanticResults []dto.SearchResultDTO, textResults []dto.SearchResultDTO) []dto.SearchResultDTO
}

// HybridRankingService implements HybridRanker using weighted min-max normalization.
type HybridRankingService struct {
	semanticWeight float64
	textWeight     float64
}

// NewHybridRankingService creates a new HybridRankingService.
// Panics if the weights do not sum to approximately 1.0 (tolerance ±0.001).
func NewHybridRankingService(semanticWeight float64, textWeight float64) *HybridRankingService {
	if math.Abs(semanticWeight+textWeight-1.0) > 0.001 {
		panic("hybrid ranker weights must sum to 1.0")
	}
	return &HybridRankingService{
		semanticWeight: semanticWeight,
		textWeight:     textWeight,
	}
}

// prepareResultsForNormalization returns a copy of results with EngineScore
// populated from SimilarityScore when requested and EngineScore is not set.
// This keeps normalization robust across callers that provide semantic scores
// in SimilarityScore rather than EngineScore.
func prepareResultsForNormalization(results []dto.SearchResultDTO, useSimilarityFallback bool) []dto.SearchResultDTO {
	prepared := make([]dto.SearchResultDTO, len(results))
	copy(prepared, results)

	if !useSimilarityFallback {
		return prepared
	}

	for i := range prepared {
		if prepared[i].EngineScore == 0 && prepared[i].SimilarityScore != 0 {
			prepared[i].EngineScore = prepared[i].SimilarityScore
		}
	}

	return prepared
}

// Rank merges semantic and text results, normalizes scores, deduplicates by
// FilePath, tags each result with its source engine, and returns results sorted
// by combined EngineScore descending.
func (h *HybridRankingService) Rank(
	semanticResults []dto.SearchResultDTO,
	textResults []dto.SearchResultDTO,
) []dto.SearchResultDTO {
	semanticResultsForNormalization := prepareResultsForNormalization(semanticResults, true)
	semNorm := minMaxNormalize(semanticResultsForNormalization)
	textNorm := minMaxNormalize(textResults)

	// Build a map from FilePath → combined result.
	type merged struct {
		result      dto.SearchResultDTO
		semScore    float64
		textScore   float64
		hasSemantic bool
		hasText     bool
	}
	byPath := make(map[string]*merged)
	// order preserves insertion sequence so the final sort starts from a
	// deterministic base, avoiding non-determinism from map iteration.
	order := make([]string, 0)

	for i, r := range semanticResults {
		r.SourceEngine = "embedding"
		r.EngineScore = h.semanticWeight * semNorm[i]
		if _, exists := byPath[r.FilePath]; !exists {
			order = append(order, r.FilePath)
			byPath[r.FilePath] = &merged{}
		}
		m := byPath[r.FilePath]
		m.result = r
		m.semScore = semNorm[i]
		m.hasSemantic = true
	}

	for i, r := range textResults {
		r.SourceEngine = "zoekt"
		r.EngineScore = h.textWeight * textNorm[i]
		if _, exists := byPath[r.FilePath]; !exists {
			order = append(order, r.FilePath)
			byPath[r.FilePath] = &merged{}
		}
		m := byPath[r.FilePath]
		if !m.hasSemantic {
			m.result = r
		}
		m.textScore = textNorm[i]
		m.hasText = true
	}

	results := make([]dto.SearchResultDTO, 0, len(byPath))
	for _, path := range order {
		m := byPath[path]
		r := m.result

		switch {
		case m.hasSemantic && m.hasText:
			r.SourceEngine = "both"
			r.EngineScore = h.semanticWeight*m.semScore + h.textWeight*m.textScore
		case m.hasSemantic:
			r.SourceEngine = "embedding"
			r.EngineScore = h.semanticWeight * m.semScore
		default:
			r.SourceEngine = "zoekt"
			r.EngineScore = h.textWeight * m.textScore
		}

		results = append(results, r)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].EngineScore > results[j].EngineScore
	})

	return results
}

// minMaxNormalize returns per-element normalized scores in [0,1].
// When all values are equal (or there's only one element), every score is 1.0.
func minMaxNormalize(results []dto.SearchResultDTO) []float64 {
	if len(results) == 0 {
		return nil
	}

	min, max := results[0].EngineScore, results[0].EngineScore
	for _, r := range results[1:] {
		if r.EngineScore < min {
			min = r.EngineScore
		}
		if r.EngineScore > max {
			max = r.EngineScore
		}
	}

	norms := make([]float64, len(results))
	span := max - min
	for i, r := range results {
		if span == 0 {
			norms[i] = 1.0
		} else {
			norms[i] = (r.EngineScore - min) / span
		}
	}
	return norms
}
