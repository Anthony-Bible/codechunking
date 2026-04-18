package service

import (
	"codechunking/internal/application/dto"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHybridRankingService defines the contract for the HybridRankingService.
//
// Constructor contract:
//
//	NewHybridRankingService(semanticWeight float64, textWeight float64) *HybridRankingService
//
// Method contract:
//
//	(*HybridRankingService).Rank(semanticResults []dto.SearchResultDTO, textResults []dto.SearchResultDTO) []dto.SearchResultDTO
func TestHybridRankingService(t *testing.T) {
	t.Run("Empty_Semantic_And_Empty_Text_Returns_Empty", func(t *testing.T) {
		ranker := NewHybridRankingService(0.7, 0.3)
		require.NotNil(t, ranker)

		results := ranker.Rank([]dto.SearchResultDTO{}, []dto.SearchResultDTO{})

		assert.Empty(t, results, "ranking empty inputs must produce an empty slice")
	})

	t.Run("Invalid_Weights_Sum_Not_1_Panics_Or_Returns_Error", func(t *testing.T) {
		assert.Panics(t, func() {
			NewHybridRankingService(0.5, 0.1)
		}, "constructor must panic when weights do not sum to 1.0")
	})

	t.Run("Min_Max_Normalization_Scores_Are_In_0_1_Range", func(t *testing.T) {
		ranker := NewHybridRankingService(0.7, 0.3)

		semanticResults := []dto.SearchResultDTO{
			{ChunkID: uuid.New(), FilePath: "a.go", Content: "a", EngineScore: 100.0},
			{ChunkID: uuid.New(), FilePath: "b.go", Content: "b", EngineScore: 50.0},
			{ChunkID: uuid.New(), FilePath: "c.go", Content: "c", EngineScore: 10.0},
		}
		textResults := []dto.SearchResultDTO{
			{ChunkID: uuid.New(), FilePath: "d.go", Content: "d", EngineScore: 200.0},
			{ChunkID: uuid.New(), FilePath: "e.go", Content: "e", EngineScore: 50.0},
		}

		results := ranker.Rank(semanticResults, textResults)

		for _, r := range results {
			assert.GreaterOrEqual(t, r.EngineScore, 0.0,
				"normalized combined score must be >= 0: file=%s score=%f", r.FilePath, r.EngineScore)
			assert.LessOrEqual(t, r.EngineScore, 1.0,
				"normalized combined score must be <= 1: file=%s score=%f", r.FilePath, r.EngineScore)
		}
	})

	// Table-driven tests for source engine tagging and result count.
	t.Run("SourceEngineTagging", func(t *testing.T) {
		semID := uuid.New()
		textID := uuid.New()
		sharedPath := "internal/db/connection.go"

		cases := []struct {
			name            string
			semanticResults []dto.SearchResultDTO
			textResults     []dto.SearchResultDTO
			wantLen         int
			wantEngines     []string // all results must have one of these source engines
		}{
			{
				name: "Semantic_Only_Tags_Embedding",
				semanticResults: []dto.SearchResultDTO{
					{ChunkID: uuid.New(), FilePath: "internal/auth/auth.go", Content: "func Authenticate() {}", EngineScore: 0.9},
					{ChunkID: uuid.New(), FilePath: "internal/auth/token.go", Content: "func ValidateToken() {}", EngineScore: 0.75},
				},
				textResults: []dto.SearchResultDTO{},
				wantLen:     2,
				wantEngines: []string{"embedding"},
			},
			{
				name:            "Text_Only_Tags_Zoekt",
				semanticResults: []dto.SearchResultDTO{},
				textResults: []dto.SearchResultDTO{
					{ChunkID: uuid.New(), FilePath: "cmd/server/main.go", Content: "func main() {}", EngineScore: 0.8},
				},
				wantLen:     1,
				wantEngines: []string{"zoekt"},
			},
			{
				name: "Both_Engines_No_Overlap_Combined_And_Sorted",
				semanticResults: []dto.SearchResultDTO{
					{ChunkID: semID, FilePath: "a.go", Content: "sem result", EngineScore: 1.0},
				},
				textResults: []dto.SearchResultDTO{
					{ChunkID: textID, FilePath: "b.go", Content: "text result", EngineScore: 1.0},
				},
				wantLen:     2,
				wantEngines: []string{"embedding", "zoekt"},
			},
			{
				name: "Same_FilePath_Deduplicated_Tags_Both",
				semanticResults: []dto.SearchResultDTO{
					{ChunkID: uuid.New(), FilePath: sharedPath, Content: "db connect sem", EngineScore: 0.9},
				},
				textResults: []dto.SearchResultDTO{
					{ChunkID: uuid.New(), FilePath: sharedPath, Content: "db connect text", EngineScore: 0.8},
				},
				wantLen:     1,
				wantEngines: []string{"both"},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				ranker := NewHybridRankingService(0.7, 0.3)
				results := ranker.Rank(tc.semanticResults, tc.textResults)

				require.Len(t, results, tc.wantLen)

				engineSet := map[string]bool{}
				for _, r := range results {
					engineSet[r.SourceEngine] = true
				}
				for _, want := range tc.wantEngines {
					assert.True(t, engineSet[want], "expected source engine %q in results", want)
				}

				// Results must be sorted descending by engine score.
				for i := 1; i < len(results); i++ {
					assert.GreaterOrEqual(t, results[i-1].EngineScore, results[i].EngineScore,
						"results must be ordered by descending engine score")
				}
			})
		}
	})

	// Table-driven tests for weight application.
	t.Run("WeightApplication", func(t *testing.T) {
		cases := []struct {
			name           string
			semanticWeight float64
			textWeight     float64
			wantHigherFile string // file path that must score higher
			wantLowerFile  string
		}{
			{
				name:           "Default_70_Semantic_30_Text_Semantic_Wins",
				semanticWeight: 0.7,
				textWeight:     0.3,
				wantHigherFile: "sem.go",
				wantLowerFile:  "txt.go",
			},
			{
				name:           "Inverted_30_Semantic_70_Text_Text_Wins",
				semanticWeight: 0.3,
				textWeight:     0.7,
				wantHigherFile: "txt.go",
				wantLowerFile:  "sem.go",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				ranker := NewHybridRankingService(tc.semanticWeight, tc.textWeight)
				require.NotNil(t, ranker)

				semResult := dto.SearchResultDTO{ChunkID: uuid.New(), FilePath: "sem.go", Content: "s", EngineScore: 1.0}
				txtResult := dto.SearchResultDTO{ChunkID: uuid.New(), FilePath: "txt.go", Content: "t", EngineScore: 1.0}

				results := ranker.Rank([]dto.SearchResultDTO{semResult}, []dto.SearchResultDTO{txtResult})
				require.Len(t, results, 2)

				higherIdx, lowerIdx := -1, -1
				for i, r := range results {
					switch r.FilePath {
					case tc.wantHigherFile:
						higherIdx = i
					case tc.wantLowerFile:
						lowerIdx = i
					}
				}
				require.NotEqual(t, -1, higherIdx, "expected result %q in output", tc.wantHigherFile)
				require.NotEqual(t, -1, lowerIdx, "expected result %q in output", tc.wantLowerFile)
				assert.Greater(t, results[higherIdx].EngineScore, results[lowerIdx].EngineScore,
					"%q must score higher than %q", tc.wantHigherFile, tc.wantLowerFile)
			})
		}
	})

	t.Run("Deduplication_Keeps_Highest_Combined_Score", func(t *testing.T) {
		ranker := NewHybridRankingService(0.5, 0.5)
		sharedPath := "pkg/util/strings.go"

		results := ranker.Rank(
			[]dto.SearchResultDTO{{ChunkID: uuid.New(), FilePath: sharedPath, Content: "sem", EngineScore: 1.0}},
			[]dto.SearchResultDTO{{ChunkID: uuid.New(), FilePath: sharedPath, Content: "text", EngineScore: 1.0}},
		)

		require.Len(t, results, 1)
		assert.Greater(t, results[0].EngineScore, 0.0, "deduplicated combined score must be positive")
	})

	t.Run("Result_Engine_Score_Reflects_Combined_Normalized_Score", func(t *testing.T) {
		ranker := NewHybridRankingService(0.7, 0.3)

		results := ranker.Rank(
			[]dto.SearchResultDTO{{ChunkID: uuid.New(), FilePath: "x.go", Content: "x", EngineScore: 5.0}},
			[]dto.SearchResultDTO{{ChunkID: uuid.New(), FilePath: "y.go", Content: "y", EngineScore: 5.0}},
		)
		require.Len(t, results, 2)

		for _, r := range results {
			assert.Greater(t, r.EngineScore, 0.0,
				"engine score must be positive for result with non-zero raw score: file=%s", r.FilePath)
		}
	})
}
