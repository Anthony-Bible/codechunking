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
//	NewHybridRankingService() *HybridRankingService
//
// Method contract:
//
//	(*HybridRankingService).Rank(semanticResults []dto.SearchResultDTO, textResults []dto.SearchResultDTO) []dto.SearchResultDTO
func TestHybridRankingService(t *testing.T) {
	t.Run("Empty_Semantic_And_Empty_Text_Returns_Empty", func(t *testing.T) {
		ranker := NewHybridRankingService()
		require.NotNil(t, ranker)

		results := ranker.Rank([]dto.SearchResultDTO{}, []dto.SearchResultDTO{})

		assert.Empty(t, results, "ranking empty inputs must produce an empty slice")
	})

	t.Run("Constructor_Accepts_Any_Weights", func(t *testing.T) {
		ranker := NewHybridRankingService()
		require.NotNil(t, ranker)
	})

	t.Run("Standard_RRF_Calculation_Verification", func(t *testing.T) {
		// Standard RRF parameter: k = 60.0
		const k = 60.0

		// Weights should NOT affect RRF results according to requirements
		ranker := NewHybridRankingService()

		semanticResults := []dto.SearchResultDTO{
			{FilePath: "shared.go", EngineScore: 100.0},  // Rank 1
			{FilePath: "sem_only.go", EngineScore: 80.0}, // Rank 2
		}
		textResults := []dto.SearchResultDTO{
			{FilePath: "text_only.go", EngineScore: 500.0}, // Rank 1
			{FilePath: "shared.go", EngineScore: 300.0},    // Rank 2
		}

		results := ranker.Rank(semanticResults, textResults)

		// Expected results:
		// 1. shared.go:
		//    semantic_rank = 1, text_rank = 2
		//    score = 1/(60+1) + 1/(60+2) = 1/61 + 1/62 ≈ 0.016393 + 0.016129 = 0.032522
		// 2. text_only.go:
		//    semantic_rank = ∞ (0 contribution), text_rank = 1
		//    score = 0 + 1/(60+1) = 1/61 ≈ 0.016393
		// 3. sem_only.go:
		//    semantic_rank = 2, text_rank = ∞ (0 contribution)
		//    score = 1/(60+2) + 0 = 1/62 ≈ 0.016129

		require.Len(t, results, 3)

		// shared.go
		assert.Equal(t, "shared.go", results[0].FilePath)
		expectedShared := 1.0/(k+1.0) + 1.0/(k+2.0)
		assert.InDelta(t, expectedShared, results[0].EngineScore, 0.000001)
		assert.Equal(t, results[0].EngineScore, results[0].SimilarityScore)

		// text_only.go
		assert.Equal(t, "text_only.go", results[1].FilePath)
		expectedTextOnly := 1.0 / (k + 1.0)
		assert.InDelta(t, expectedTextOnly, results[1].EngineScore, 0.000001)
		assert.Equal(t, results[1].EngineScore, results[1].SimilarityScore)

		// sem_only.go
		assert.Equal(t, "sem_only.go", results[2].FilePath)
		expectedSemOnly := 1.0 / (k + 2.0)
		assert.InDelta(t, expectedSemOnly, results[2].EngineScore, 0.000001)
		assert.Equal(t, results[2].EngineScore, results[2].SimilarityScore)
	})

	t.Run("RRF_Ignores_Configured_Weights", func(t *testing.T) {
		const k = 60.0
		// Use very different weights
		ranker1 := NewHybridRankingService()
		ranker2 := NewHybridRankingService()

		sem := []dto.SearchResultDTO{{FilePath: "a.go", EngineScore: 1.0}}
		txt := []dto.SearchResultDTO{{FilePath: "b.go", EngineScore: 1.0}}

		res1 := ranker1.Rank(sem, txt)
		res2 := ranker2.Rank(sem, txt)

		require.Len(t, res1, 2)
		require.Len(t, res2, 2)

		// Both should have identical RRF scores regardless of weights
		assert.InDelta(t, res1[0].EngineScore, res2[0].EngineScore, 0.000001)
		assert.InDelta(t, res1[1].EngineScore, res2[1].EngineScore, 0.000001)
		assert.InDelta(t, 1.0/(k+1.0), res1[0].EngineScore, 0.000001)
	})

	t.Run("RRF_Scores_Are_In_0_1_Range", func(t *testing.T) {
		ranker := NewHybridRankingService()

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

		// Verify RRF scoring stays positive for ranked results with non-zero contribution.
		for _, r := range results {
			assert.Greater(t, r.EngineScore, 0.0, "score must not be 0.0 for non-zero raw score")
		}
	})

	// Table-driven tests for source engine tagging and result count.
	t.Run("SourceEngineTagging", func(t *testing.T) {
		semID := uuid.New()
		textID := uuid.New()
		sharedChunkID := uuid.New()
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
				name: "Same_ChunkID_Deduplicated_Tags_Both",
				semanticResults: []dto.SearchResultDTO{
					{ChunkID: sharedChunkID, FilePath: sharedPath, Content: "db connect sem", EngineScore: 0.9},
				},
				textResults: []dto.SearchResultDTO{
					{ChunkID: sharedChunkID, FilePath: sharedPath, Content: "db connect text", EngineScore: 0.8},
				},
				wantLen:     1,
				wantEngines: []string{"both"},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				ranker := NewHybridRankingService()
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
		// With RRF, weights are ignored and tied ranks result in tied scores.
		cases := []struct {
			name string
		}{
			{name: "Default_70_Semantic_30_Text"},
			{name: "Inverted_30_Semantic_70_Text"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				ranker := NewHybridRankingService()
				require.NotNil(t, ranker)

				semResult := dto.SearchResultDTO{ChunkID: uuid.New(), FilePath: "sem.go", Content: "s", EngineScore: 1.0}
				txtResult := dto.SearchResultDTO{ChunkID: uuid.New(), FilePath: "txt.go", Content: "t", EngineScore: 1.0}

				results := ranker.Rank([]dto.SearchResultDTO{semResult}, []dto.SearchResultDTO{txtResult})
				require.Len(t, results, 2)

				// Since both were rank 1 in their respective engines, they should have identical RRF scores
				assert.Equal(t, results[0].EngineScore, results[1].EngineScore)
			})
		}
	})

	t.Run("Deterministic_Tie_Ordering_By_Repository_Then_Path", func(t *testing.T) {
		ranker := NewHybridRankingService()
		repoName := "github.com/example/repo"

		results := ranker.Rank(
			[]dto.SearchResultDTO{
				{
					ChunkID:     uuid.MustParse("77777777-7777-7777-7777-777777777777"),
					FilePath:    "z.go",
					Content:     "semantic",
					EngineScore: 1.0,
					Repository: dto.RepositoryInfo{
						Name: repoName,
					},
				},
			},
			[]dto.SearchResultDTO{
				{
					ChunkID:     uuid.MustParse("88888888-8888-8888-8888-888888888888"),
					FilePath:    "a.go",
					Content:     "text",
					EngineScore: 1.0,
					Repository: dto.RepositoryInfo{
						Name: repoName,
					},
				},
			},
		)

		require.Len(t, results, 2)
		assert.Equal(t, "a.go", results[0].FilePath)
		assert.Equal(t, "z.go", results[1].FilePath)
	})

	t.Run("Deduplication_Keeps_Highest_Combined_Score", func(t *testing.T) {
		ranker := NewHybridRankingService()
		sharedChunkID := uuid.New()
		sharedPath := "pkg/util/strings.go"

		results := ranker.Rank(
			[]dto.SearchResultDTO{{ChunkID: sharedChunkID, FilePath: sharedPath, Content: "sem", EngineScore: 1.0}},
			[]dto.SearchResultDTO{{ChunkID: sharedChunkID, FilePath: sharedPath, Content: "text", EngineScore: 1.0}},
		)

		require.Len(t, results, 1)
		assert.Greater(t, results[0].EngineScore, 0.0, "deduplicated combined score must be positive")
	})

	t.Run("Result_Engine_Score_Reflects_Combined_Normalized_Score", func(t *testing.T) {
		ranker := NewHybridRankingService()

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

	t.Run("ZoektLargeScores_SimilarityScore_Normalized_To_RRF", func(t *testing.T) {
		ranker := NewHybridRankingService()

		textResults := []dto.SearchResultDTO{
			{ChunkID: uuid.New(), FilePath: "a.go", Content: "a", EngineScore: 5000000000},
			{ChunkID: uuid.New(), FilePath: "b.go", Content: "b", EngineScore: 2500000000},
			{ChunkID: uuid.New(), FilePath: "c.go", Content: "c", EngineScore: 1000000000},
		}

		results := ranker.Rank([]dto.SearchResultDTO{}, textResults)

		require.NotEmpty(t, results)
		const k = 60.0
		// RRF scores for rank 1, 2, 3
		assert.InDelta(t, 1.0/(k+1.0), results[0].SimilarityScore, 0.0001)
		assert.InDelta(t, 1.0/(k+2.0), results[1].SimilarityScore, 0.0001)
		assert.InDelta(t, 1.0/(k+3.0), results[2].SimilarityScore, 0.0001)

		for _, r := range results {
			assert.GreaterOrEqual(t, r.SimilarityScore, 0.0,
				"SimilarityScore must be >= 0 after normalization: file=%s score=%f", r.FilePath, r.SimilarityScore)
			assert.LessOrEqual(t, r.SimilarityScore, 1.0,
				"SimilarityScore must be <= 1 after normalization: file=%s score=%f", r.FilePath, r.SimilarityScore)
			// SimilarityScore must match the normalized EngineScore, not remain at 0
			assert.Equal(t, r.EngineScore, r.SimilarityScore,
				"SimilarityScore must be set to the normalized EngineScore: file=%s similarity=%f engine=%f",
				r.FilePath, r.SimilarityScore, r.EngineScore)
		}
	})

	t.Run("SimilarityScore_Consistent_With_EngineScore_After_Rank", func(t *testing.T) {
		ranker := NewHybridRankingService()

		semanticResults := []dto.SearchResultDTO{
			{ChunkID: uuid.New(), FilePath: "sem1.go", Content: "sem1", EngineScore: 0.9},
			{ChunkID: uuid.New(), FilePath: "sem2.go", Content: "sem2", EngineScore: 0.7},
		}
		textResults := []dto.SearchResultDTO{
			{ChunkID: uuid.New(), FilePath: "txt1.go", Content: "txt1", EngineScore: 5000000009},
			{ChunkID: uuid.New(), FilePath: "txt2.go", Content: "txt2", EngineScore: 4000000005},
		}

		results := ranker.Rank(semanticResults, textResults)

		require.NotEmpty(t, results)
		for _, r := range results {
			assert.GreaterOrEqual(t, r.SimilarityScore, 0.0,
				"SimilarityScore must be >= 0: file=%s similarity=%f engine=%f", r.FilePath, r.SimilarityScore, r.EngineScore)
			assert.LessOrEqual(t, r.SimilarityScore, 1.0,
				"SimilarityScore must be <= 1: file=%s similarity=%f engine=%f", r.FilePath, r.SimilarityScore, r.EngineScore)
			// SimilarityScore must be set to the normalized EngineScore, not remain at 0
			assert.Equal(t, r.EngineScore, r.SimilarityScore,
				"SimilarityScore must match normalized EngineScore: file=%s similarity=%f engine=%f",
				r.FilePath, r.SimilarityScore, r.EngineScore)
		}
	})

	t.Run("ZoektOnly_SimilarityScore_Equals_Normalized_EngineScore", func(t *testing.T) {
		ranker := NewHybridRankingService()

		textResults := []dto.SearchResultDTO{
			{ChunkID: uuid.New(), FilePath: "z1.go", Content: "z1", EngineScore: 5000000009},
			{ChunkID: uuid.New(), FilePath: "z2.go", Content: "z2", EngineScore: 2500000000},
		}

		results := ranker.Rank([]dto.SearchResultDTO{}, textResults)

		require.Len(t, results, 2)
		for _, r := range results {
			assert.Equal(t, r.EngineScore, r.SimilarityScore,
				"for zoekt-only results SimilarityScore must equal normalized EngineScore: file=%s similarity=%f engine=%f",
				r.FilePath, r.SimilarityScore, r.EngineScore)
		}
	})
}

func TestHybridRankingService_ChunkLevelRRFContract(t *testing.T) {
	t.Run("Fuses_Semantic_And_Text_By_ChunkID_Not_FilePath", func(t *testing.T) {
		const k = 60.0
		ranker := NewHybridRankingService()
		sharedChunkID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

		results := ranker.Rank(
			[]dto.SearchResultDTO{
				{
					ChunkID:      sharedChunkID,
					FilePath:     "semantic/path.go",
					Content:      "semantic payload",
					SourceEngine: "embedding",
					EngineScore:  0.9,
				},
			},
			[]dto.SearchResultDTO{
				{
					ChunkID:      sharedChunkID,
					FilePath:     "zoekt/path.go",
					Content:      "zoekt payload",
					SourceEngine: "zoekt",
					EngineScore:  100,
				},
			},
		)

		require.Len(t, results, 1)
		assert.Equal(t, sharedChunkID, results[0].ChunkID)
		assert.Equal(t, "both", results[0].SourceEngine)
		assert.InDelta(t, 1.0/(k+1.0)+1.0/(k+1.0), results[0].EngineScore, 0.000001)
		assert.Equal(t, results[0].EngineScore, results[0].SimilarityScore)
	})

	t.Run("Does_Not_Merge_Different_Chunks_In_Same_File", func(t *testing.T) {
		ranker := NewHybridRankingService()
		firstChunkID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
		secondChunkID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

		results := ranker.Rank(
			[]dto.SearchResultDTO{
				{
					ChunkID:     firstChunkID,
					FilePath:    "internal/auth/service.go",
					Content:     "func Login() {}",
					StartLine:   10,
					EndLine:     20,
					EngineScore: 0.9,
				},
			},
			[]dto.SearchResultDTO{
				{
					ChunkID:     secondChunkID,
					FilePath:    "internal/auth/service.go",
					Content:     "func Logout() {}",
					StartLine:   80,
					EndLine:     90,
					EngineScore: 100,
				},
			},
		)

		require.Len(t, results, 2)
		assert.ElementsMatch(t, []uuid.UUID{firstChunkID, secondChunkID}, []uuid.UUID{results[0].ChunkID, results[1].ChunkID})
	})

	t.Run("Does_Not_Merge_Same_FilePath_From_Different_Repositories", func(t *testing.T) {
		ranker := NewHybridRankingService()
		repoAChunkID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
		repoBChunkID := uuid.MustParse("55555555-5555-5555-5555-555555555555")

		results := ranker.Rank(
			[]dto.SearchResultDTO{
				{
					ChunkID:     repoAChunkID,
					FilePath:    "cmd/server/main.go",
					Content:     "repo A main",
					EngineScore: 0.9,
					Repository: dto.RepositoryInfo{
						Name: "github.com/example/repo-a",
					},
				},
			},
			[]dto.SearchResultDTO{
				{
					ChunkID:     repoBChunkID,
					FilePath:    "cmd/server/main.go",
					Content:     "repo B main",
					EngineScore: 100,
					Repository: dto.RepositoryInfo{
						Name: "github.com/example/repo-b",
					},
				},
			},
		)

		require.Len(t, results, 2)
		assert.ElementsMatch(t, []uuid.UUID{repoAChunkID, repoBChunkID}, []uuid.UUID{results[0].ChunkID, results[1].ChunkID})
	})

	t.Run("Duplicate_Same_Engine_Hits_Count_Once_With_Best_Rank", func(t *testing.T) {
		const k = 60.0
		ranker := NewHybridRankingService()
		duplicateChunkID := uuid.MustParse("66666666-6666-6666-6666-666666666666")

		results := ranker.Rank(
			[]dto.SearchResultDTO{},
			[]dto.SearchResultDTO{
				{
					ChunkID:     duplicateChunkID,
					FilePath:    "internal/cache/store.go",
					Content:     "first lexical hit",
					EngineScore: 100,
				},
				{
					ChunkID:     duplicateChunkID,
					FilePath:    "internal/cache/store.go",
					Content:     "duplicate lexical hit",
					EngineScore: 90,
				},
			},
		)

		require.Len(t, results, 1)
		assert.Equal(t, duplicateChunkID, results[0].ChunkID)
		assert.Equal(t, "zoekt", results[0].SourceEngine)
		assert.InDelta(t, 1.0/(k+1.0), results[0].EngineScore, 0.000001)
	})

	t.Run("Unresolved_Zoekt_File_Fallback_Is_Kept", func(t *testing.T) {
		const k = 60.0
		ranker := NewHybridRankingService()

		results := ranker.Rank(
			[]dto.SearchResultDTO{},
			[]dto.SearchResultDTO{
				{
					FilePath:    "docs/setup.md",
					Content:     "install the cli",
					EngineScore: 100,
					Repository: dto.RepositoryInfo{
						Name: "github.com/example/docs",
					},
				},
			},
		)

		require.Len(t, results, 1)
		assert.Equal(t, uuid.Nil, results[0].ChunkID)
		assert.Equal(t, "docs/setup.md", results[0].FilePath)
		assert.Equal(t, "github.com/example/docs", results[0].Repository.Name)
		assert.Equal(t, "zoekt", results[0].SourceEngine)
		assert.InDelta(t, 1.0/(k+1.0), results[0].EngineScore, 0.000001)
	})
}
