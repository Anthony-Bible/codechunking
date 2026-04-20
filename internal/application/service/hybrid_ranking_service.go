package service

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/application/service/ranking"
	"sort"

	"github.com/google/uuid"
)

// HybridRanker merges and ranks results from semantic and text search engines.
type HybridRanker interface {
	Rank(semanticResults []dto.SearchResultDTO, textResults []dto.SearchResultDTO) []dto.SearchResultDTO
}

// HybridRankingService implements HybridRanker using Reciprocal Rank Fusion (RRF).
type HybridRankingService struct{}

type engineHit struct {
	result dto.SearchResultDTO
	rank   int
}

type mergedSearchResult struct {
	semantic    *engineHit
	text        *engineHit
	hasSemantic bool
	hasText     bool
}

// NewHybridRankingService creates a new HybridRankingService using Reciprocal Rank Fusion.
func NewHybridRankingService() *HybridRankingService {
	return &HybridRankingService{}
}

// Rank merges semantic and text results using Reciprocal Rank Fusion (RRF).
// It deduplicates by chunk ID when available, falls back to repository-qualified
// file paths for unresolved chunks, tags each merged result with its source engine,
// and returns results sorted by EngineScore descending.
func (h *HybridRankingService) Rank(
	semanticResults []dto.SearchResultDTO,
	textResults []dto.SearchResultDTO,
) []dto.SearchResultDTO {
	byKey := make(map[string]*mergedSearchResult)
	order := make([]string, 0)

	for i, r := range semanticResults {
		mergeSearchResult(byKey, &order, r, i+1, true)
	}

	for i, r := range textResults {
		mergeSearchResult(byKey, &order, r, i+1, false)
	}

	results := make([]dto.SearchResultDTO, 0, len(order))
	for _, key := range order {
		m := byKey[key]
		rrfScore := calculateMergedRRFScore(m)
		r := buildMergedResult(m)
		r.EngineScore = rrfScore
		r.SimilarityScore = rrfScore
		results = append(results, r)
	}

	sort.Slice(results, func(i, j int) bool {
		switch {
		case results[i].EngineScore != results[j].EngineScore:
			return results[i].EngineScore > results[j].EngineScore
		case results[i].Repository.Name != results[j].Repository.Name:
			return results[i].Repository.Name < results[j].Repository.Name
		case results[i].FilePath != results[j].FilePath:
			return results[i].FilePath < results[j].FilePath
		case results[i].StartLine != results[j].StartLine:
			return results[i].StartLine < results[j].StartLine
		case results[i].EndLine != results[j].EndLine:
			return results[i].EndLine < results[j].EndLine
		case results[i].ChunkID != results[j].ChunkID:
			return results[i].ChunkID.String() < results[j].ChunkID.String()
		case results[i].SourceEngine != results[j].SourceEngine:
			return results[i].SourceEngine < results[j].SourceEngine
		default:
			return results[i].Content < results[j].Content
		}
	})

	return results
}

func mergeSearchResult(
	byKey map[string]*mergedSearchResult,
	order *[]string,
	result dto.SearchResultDTO,
	rank int,
	fromSemantic bool,
) {
	key := dedupKeyForResult(result)
	m, exists := byKey[key]
	if !exists {
		*order = append(*order, key)
		m = &mergedSearchResult{}
		byKey[key] = m
	}

	hit := &engineHit{result: result, rank: rank}
	if fromSemantic {
		if m.semantic == nil || rank < m.semantic.rank {
			m.semantic = hit
		}
		m.hasSemantic = true
		return
	}

	if m.text == nil || rank < m.text.rank {
		m.text = hit
	}
	m.hasText = true
}

func dedupKeyForResult(result dto.SearchResultDTO) string {
	if result.ChunkID != uuid.Nil {
		return "chunk:" + result.ChunkID.String()
	}

	if result.Repository.Name != "" {
		return "file:" + result.Repository.Name + "|" + result.FilePath
	}

	return "file:" + result.FilePath
}

func calculateMergedRRFScore(m *mergedSearchResult) float64 {
	var score float64
	if m.semantic != nil {
		score += ranking.CalculateRRFScore(m.semantic.rank)
	}
	if m.text != nil {
		score += ranking.CalculateRRFScore(m.text.rank)
	}
	return score
}

func buildMergedResult(m *mergedSearchResult) dto.SearchResultDTO {
	var result dto.SearchResultDTO
	if m.semantic != nil {
		result = m.semantic.result
	} else if m.text != nil {
		result = m.text.result
	}
	result.SourceEngine = mergedSourceEngine(m.hasSemantic, m.hasText)
	return result
}

func mergedSourceEngine(hasSemantic, hasText bool) string {
	switch {
	case hasSemantic && hasText:
		return "both"
	case hasSemantic:
		return "embedding"
	default:
		return "zoekt"
	}
}
