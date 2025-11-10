//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestSearchAPI_FilePathLeakage tests specifically for UUID path leakage in search API responses
func TestSearchAPI_FilePathLeakage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping search API filepath leakage test")
	}

	t.Run("search API response contains no UUID paths", func(t *testing.T) {
		// This test specifically verifies that search API responses never contain UUID-based paths

		ctx := context.Background()
		repoID := uuid.New()

		// Setup: Create test data with various path formats
		testScenarios := []struct {
			name             string
			inputPaths       []string
			shouldContain    []string // paths that should be in results
			shouldNotContain []string // paths that should never be in results
		}{
			{
				name: "clean relative paths",
				inputPaths: []string{
					"src/main.go",
					"internal/auth/middleware.go",
					"pkg/utils/helpers.go",
				},
				shouldContain: []string{
					"src/main.go",
					"internal/auth/middleware.go",
					"pkg/utils/helpers.go",
				},
				shouldNotContain: []string{
					repoID.String(),
					"codechunking-workspace",
					"/tmp/",
				},
			},
			{
				name: "mixed clean and dirty paths",
				inputPaths: []string{
					"src/main.go",
					"/tmp/workspace/" + repoID.String() + "/internal/config.go",
					"/var/folders/xyz/codechunking-workspace/" + repoID.String() + "/pkg/api/handlers.go",
				},
				shouldContain: []string{
					"src/main.go",
					"internal/config.go",
					"pkg/api/handlers.go",
				},
				shouldNotContain: []string{
					repoID.String(),
					"codechunking-workspace",
					"/tmp/",
					"/var/folders/",
				},
			},
		}

		for _, scenario := range testScenarios {
			t.Run(scenario.name, func(t *testing.T) {
				// Store test chunks with various path formats
				chunks := createTestChunksFromPaths(t, scenario.inputPaths, repoID)
				storeTestChunksInDatabase(ctx, t, chunks, repoID)

				// Execute search API call
				searchResponse := executeSearchAPIRequest(ctx, t, "function")

				// Verify response structure
				assert.NotEmpty(t, searchResponse.Results, "Search should return results")

				// Verify no UUID leakage
				for _, result := range searchResponse.Results {
					// Check that required clean paths are present
					found := false
					for _, expectedPath := range scenario.shouldContain {
						if result.FilePath == expectedPath {
							found = true
							break
						}
					}
					if len(scenario.shouldContain) > 0 {
						assert.True(t, found, "Result should contain one of expected clean paths: %s", result.FilePath)
					}

					// Check that forbidden paths are not present
					for _, forbiddenPath := range scenario.shouldNotContain {
						assert.NotContains(t, result.FilePath, forbiddenPath,
							"Search result should not contain forbidden path '%s': %s", forbiddenPath, result.FilePath)
					}
				}

				// Cleanup
				cleanupTestChunksFromDatabase(ctx, t, repoID)
			})
		}
	})

	t.Run("search API with various query types", func(t *testing.T) {
		// Test different search query types to ensure none leak UUID paths

		ctx := context.Background()
		repoID := uuid.New()

		// Create test data
		testPaths := []string{
			"src/main.go",
			"internal/service.go",
			"pkg/utils.go",
		}
		chunks := createTestChunksFromPaths(t, testPaths, repoID)
		storeTestChunksInDatabase(ctx, t, chunks, repoID)
		defer cleanupTestChunksFromDatabase(ctx, t, repoID)

		// Test various search queries
		searchQueries := []string{
			"function",
			"main",
			"service",
			"utils",
			"package",
		}

		for _, query := range searchQueries {
			t.Run("query_"+query, func(t *testing.T) {
				searchResponse := executeSearchAPIRequest(ctx, t, query)

				// Verify no UUID leakage regardless of query
				for _, result := range searchResponse.Results {
					assert.NotContains(t, result.FilePath, repoID.String(),
						"Query '%s' should not return UUID paths: %s", query, result.FilePath)
					assert.NotContains(t, result.FilePath, "codechunking-workspace",
						"Query '%s' should not return workspace paths: %s", query, result.FilePath)
				}
			})
		}
	})

	t.Run("search API pagination preserves clean paths", func(t *testing.T) {
		// Test that pagination doesn't introduce path leakage

		ctx := context.Background()
		repoID := uuid.New()

		// Create many test chunks
		var testPaths []string
		for i := 0; i < 25; i++ {
			testPaths = append(testPaths, fmt.Sprintf("src/file%d.go", i))
		}
		chunks := createTestChunksFromPaths(t, testPaths, repoID)
		storeTestChunksInDatabase(ctx, t, chunks, repoID)
		defer cleanupTestChunksFromDatabase(ctx, t, repoID)

		// Test paginated searches
		paginationTests := []struct {
			name   string
			limit  int
			offset int
		}{
			{"first_page", 10, 0},
			{"second_page", 10, 10},
			{"third_page", 10, 20},
			{"small_limit", 5, 0},
			{"large_limit", 50, 0},
		}

		for _, test := range paginationTests {
			t.Run(test.name, func(t *testing.T) {
				searchRequest := map[string]interface{}{
					"query":  "function",
					"limit":  test.limit,
					"offset": test.offset,
				}

				searchResponse := executeSearchAPIRequestWithParams(ctx, t, searchRequest)

				// Verify pagination doesn't break path cleanliness
				for _, result := range searchResponse.Results {
					assert.NotContains(t, result.FilePath, repoID.String(),
						"Paginated result should not contain UUID: %s", result.FilePath)
					assert.NotContains(t, result.FilePath, "codechunking-workspace",
						"Paginated result should not contain workspace: %s", result.FilePath)
				}
			})
		}
	})
}

// TestJobProcessor_RelativePathPipeline tests the job processor's handling of relative paths
func TestJobProcessor_RelativePathPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping job processor relative path test")
	}

	t.Run("job processor maintains relative paths throughout pipeline", func(t *testing.T) {
		// Test the complete job processor pipeline from job creation to completion

		ctx := context.Background()
		repoID := uuid.New()
		jobID := uuid.New()

		// Create test repository
		testRepo := createTestRepositoryForJobProcessor(t)
		defer cleanupTestRepository(t, testRepo)

		// Execute job processor pipeline
		jobResult := executeJobProcessorPipeline(ctx, t, jobID, repoID, testRepo.LocalPath)

		// Verify job completed successfully
		assert.True(t, jobResult.Success, "Job processor should complete successfully")
		assert.Positive(t, jobResult.ChunksProcessed, "Should process chunks")

		// Verify all stages used relative paths
		assert.True(t, jobResult.UsedRelativePathsInCloning, "Cloning should use relative paths")
		assert.True(t, jobResult.UsedRelativePathsInParsing, "Parsing should use relative paths")
		assert.True(t, jobResult.UsedRelativePathsInStorage, "Storage should use relative paths")

		// Verify final database state
		dbChunks := retrieveAllChunksForRepository(ctx, t, repoID)
		for _, chunk := range dbChunks {
			assert.NotContains(t, chunk.FilePath, repoID.String(),
				"Final database should not contain UUID paths: %s", chunk.FilePath)
			assert.NotContains(t, chunk.FilePath, "codechunking-workspace",
				"Final database should not contain workspace paths: %s", chunk.FilePath)
		}
	})

	t.Run("job processor error handling preserves path integrity", func(t *testing.T) {
		// Test that job processor errors don't cause path leakage

		ctx := context.Background()
		repoID := uuid.New()
		jobID := uuid.New()

		// Create repository that will cause processing errors
		errorRepo := createRepositoryWithProcessingErrors(t)
		defer cleanupTestRepository(t, errorRepo)

		// Execute job processor with expected errors
		jobResult := executeJobProcessorPipelineWithErrors(ctx, t, jobID, repoID, errorRepo.LocalPath)

		// Even with errors, no paths should leak
		if jobResult.PartialChunksProcessed > 0 {
			dbChunks := retrieveAllChunksForRepository(ctx, t, repoID)
			for _, chunk := range dbChunks {
				assert.NotContains(t, chunk.FilePath, repoID.String(),
					"Even with errors, database should not contain UUID paths: %s", chunk.FilePath)
			}
		}

		// Verify error logging doesn't contain sensitive paths
		assert.NotContains(t, jobResult.ErrorLog, repoID.String(),
			"Error logs should not contain workspace UUID")
		assert.NotContains(t, jobResult.ErrorLog, "codechunking-workspace",
			"Error logs should not contain workspace directory")
	})

	t.Run("concurrent job processing maintains path isolation", func(t *testing.T) {
		// Test that concurrent job processing doesn't mix up paths between repositories

		ctx := context.Background()
		repoCount := 3

		var repos []TestRepository
		var repoIDs []uuid.UUID
		var jobIDs []uuid.UUID

		// Create multiple repositories and jobs
		for i := 0; i < repoCount; i++ {
			repo := createTestRepositoryForJobProcessor(t)
			repos = append(repos, repo)
			repoIDs = append(repoIDs, uuid.New())
			jobIDs = append(jobIDs, uuid.New())
		}

		defer func() {
			for _, repo := range repos {
				cleanupTestRepository(t, repo)
			}
		}()

		// Execute jobs concurrently
		results := make(chan LeakageJobProcessorResult, repoCount)
		for i := 0; i < repoCount; i++ {
			go func(index int) {
				result := executeJobProcessorPipeline(ctx, t, jobIDs[index], repoIDs[index], repos[index].LocalPath)
				results <- result
			}(i)
		}

		// Collect results
		var allResults []LeakageJobProcessorResult
		for i := 0; i < repoCount; i++ {
			result := <-results
			allResults = append(allResults, result)
		}

		// Verify path isolation
		for i, result := range allResults {
			if result.Success && result.ChunksProcessed > 0 {
				dbChunks := retrieveAllChunksForRepository(ctx, t, repoIDs[i])
				for _, chunk := range dbChunks {
					// Should not contain UUID from any repository
					for j, otherRepoID := range repoIDs {
						if i != j {
							assert.NotContains(
								t,
								chunk.FilePath,
								otherRepoID.String(),
								"Repository %d chunk should not contain UUID from repository %d: %s",
								i,
								j,
								chunk.FilePath,
							)
						}
					}
				}
			}
		}
	})
}

// TestMigration_PathTransformation tests the migration path transformation logic
func TestMigration_PathTransformation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping migration path transformation test")
	}

	t.Run("migration transforms absolute paths to relative paths", func(t *testing.T) {
		// Test the core migration logic

		ctx := context.Background()
		repoID := uuid.New()

		// Create legacy data with absolute paths
		legacyPaths := []string{
			"/tmp/workspace/" + repoID.String() + "/src/main.go",
			"/var/folders/xyz/codechunking-workspace/" + repoID.String() + "/internal/service.go",
			os.TempDir() + "/codechunking-workspace/" + repoID.String() + "/pkg/utils.go",
		}

		expectedRelativePaths := []string{
			"src/main.go",
			"internal/service.go",
			"pkg/utils.go",
		}

		// Store legacy data
		legacyChunks := createTestChunksFromPaths(t, legacyPaths, repoID)
		storeLegacyChunksInDatabase(ctx, t, legacyChunks, repoID)

		// Verify legacy data exists
		beforeChunks := retrieveAllChunksForRepository(ctx, t, repoID)
		assert.Equal(t, len(legacyPaths), len(beforeChunks), "Should have stored legacy chunks")

		// Execute migration
		migrationResult := executePathMigration(ctx, t, repoID)
		assert.True(t, migrationResult.Success, "Migration should succeed")
		assert.Equal(t, len(legacyPaths), migrationResult.MigratedCount, "Should migrate all chunks")

		// Verify migration results
		afterChunks := retrieveAllChunksForRepository(ctx, t, repoID)
		assert.Equal(t, len(expectedRelativePaths), len(afterChunks), "Should have migrated chunks")

		// Verify path transformation
		for i, chunk := range afterChunks {
			assert.Equal(t, expectedRelativePaths[i], chunk.FilePath,
				"Migrated path should match expected relative path")
			assert.NotContains(t, chunk.FilePath, repoID.String(),
				"Migrated path should not contain UUID: %s", chunk.FilePath)
		}

		// Cleanup
		cleanupTestChunksFromDatabase(ctx, t, repoID)
	})

	t.Run("migration handles edge cases correctly", func(t *testing.T) {
		// Test migration edge cases

		ctx := context.Background()
		repoID := uuid.New()

		edgeCasePaths := []string{
			"",                                  // Empty path
			"/tmp/workspace/" + repoID.String(), // Directory path
			"/tmp/workspace/" + repoID.String() + "/", // Trailing slash
			"/tmp/workspace/other-uuid/src/main.go",   // Different UUID
			"src/main.go",                             // Already relative
			"./src/main.go",                           // Relative with ./
			"../src/main.go",                          // Relative with ../
		}

		// Store edge case data
		edgeChunks := createTestChunksFromPaths(t, edgeCasePaths, repoID)
		storeLegacyChunksInDatabase(ctx, t, edgeChunks, repoID)

		// Execute migration
		migrationResult := executePathMigration(ctx, t, repoID)
		assert.True(t, migrationResult.Success, "Edge case migration should succeed")

		// Verify edge case handling
		afterChunks := retrieveAllChunksForRepository(ctx, t, repoID)
		for _, chunk := range afterChunks {
			// All paths should be clean relative paths after migration
			assert.NotContains(t, chunk.FilePath, repoID.String(),
				"Edge case migration should remove UUID: %s", chunk.FilePath)
			assert.False(t, strings.HasPrefix(chunk.FilePath, "/tmp/"),
				"Edge case migration should remove absolute paths: %s", chunk.FilePath)
			assert.False(t, strings.HasPrefix(chunk.FilePath, os.TempDir()),
				"Edge case migration should remove temp paths: %s", chunk.FilePath)
		}

		// Cleanup
		cleanupTestChunksFromDatabase(ctx, t, repoID)
	})

	t.Run("migration is idempotent", func(t *testing.T) {
		// Test that running migration multiple times doesn't cause issues

		ctx := context.Background()
		repoID := uuid.New()

		// Create initial data
		initialPaths := []string{
			"/tmp/workspace/" + repoID.String() + "/src/main.go",
		}

		initialChunks := createTestChunksFromPaths(t, initialPaths, repoID)
		storeLegacyChunksInDatabase(ctx, t, initialChunks, repoID)

		// Run migration first time
		migrationResult1 := executePathMigration(ctx, t, repoID)
		assert.True(t, migrationResult1.Success, "First migration should succeed")

		firstRunChunks := retrieveAllChunksForRepository(ctx, t, repoID)

		// Run migration second time
		migrationResult2 := executePathMigration(ctx, t, repoID)
		assert.True(t, migrationResult2.Success, "Second migration should succeed")

		secondRunChunks := retrieveAllChunksForRepository(ctx, t, repoID)

		// Results should be identical
		assert.Equal(t, len(firstRunChunks), len(secondRunChunks),
			"Idempotent migration should not change chunk count")

		for i := range firstRunChunks {
			assert.Equal(t, firstRunChunks[i].FilePath, secondRunChunks[i].FilePath,
				"Idempotent migration should not change file paths")
		}

		// Cleanup
		cleanupTestChunksFromDatabase(ctx, t, repoID)
	})
}

// Supporting types and helper functions

type TestSearchResponse struct {
	Results    []TestSearchResult `json:"results"`
	Pagination TestPagination     `json:"pagination"`
	Metadata   TestSearchMetadata `json:"search_metadata"`
}

type TestSearchResult struct {
	ChunkID         uuid.UUID `json:"chunk_id"`
	Content         string    `json:"content"`
	FilePath        string    `json:"file_path"`
	Language        string    `json:"language"`
	SimilarityScore float64   `json:"similarity_score"`
}

type TestPagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}

type TestSearchMetadata struct {
	Query           string `json:"query"`
	ExecutionTimeMs int64  `json:"execution_time_ms"`
}

type LeakageJobProcessorResult struct {
	Success                    bool
	ChunksProcessed            int
	PartialChunksProcessed     int
	UsedRelativePathsInCloning bool
	UsedRelativePathsInParsing bool
	UsedRelativePathsInStorage bool
	ErrorLog                   string
}

type LeakageMigrationResult struct {
	Success       bool
	MigratedCount int
	Error         error
}

type MigrationResult struct {
	Success       bool
	MigratedCount int
	Error         error
}

// Helper functions that will fail until implemented

func createTestChunksFromPaths(t *testing.T, paths []string, repoID uuid.UUID) []TestChunk {
	// This should fail until actual implementation exists
	t.Fatal(
		"FAILING TEST: createTestChunksFromPaths not implemented - this test should fail until real implementation exists",
	)
	return []TestChunk{}
}

func storeTestChunksInDatabase(ctx context.Context, t *testing.T, chunks []TestChunk, repoID uuid.UUID) {
	// This should fail until actual implementation exists
	t.Fatal(
		"FAILING TEST: storeTestChunksInDatabase not implemented - this test should fail until real implementation exists",
	)
}

func executeSearchAPIRequest(ctx context.Context, t *testing.T, query string) TestSearchResponse {
	// This should fail until actual implementation exists
	t.Fatal(
		"FAILING TEST: executeSearchAPIRequest not implemented - this test should fail until real implementation exists",
	)
	return TestSearchResponse{}
}

func cleanupTestChunksFromDatabase(ctx context.Context, t *testing.T, repoID uuid.UUID) {
	// This should fail until actual implementation exists
	t.Fatal(
		"FAILING TEST: cleanupTestChunksFromDatabase not implemented - this test should fail until real implementation exists",
	)
}

func executeSearchAPIRequestWithParams(
	ctx context.Context,
	t *testing.T,
	params map[string]interface{},
) TestSearchResponse {
	// This should fail until actual implementation exists
	t.Fatal(
		"FAILING TEST: executeSearchAPIRequestWithParams not implemented - this test should fail until real implementation exists",
	)
	return TestSearchResponse{}
}

func createTestRepositoryForJobProcessor(t *testing.T) TestRepository {
	// This should fail until actual implementation exists
	t.Fatal(
		"FAILING TEST: createTestRepositoryForJobProcessor not implemented - this test should fail until real implementation exists",
	)
	return TestRepository{}
}

func executeJobProcessorPipeline(
	ctx context.Context,
	t *testing.T,
	jobID, repoID uuid.UUID,
	repoPath string,
) LeakageJobProcessorResult {
	// This should fail until actual implementation exists
	t.Fatal(
		"FAILING TEST: executeJobProcessorPipeline not implemented - this test should fail until real implementation exists",
	)
	return LeakageJobProcessorResult{}
}

func retrieveAllChunksForRepository(ctx context.Context, t *testing.T, repoID uuid.UUID) []TestChunk {
	// This should fail until actual implementation exists
	t.Fatal(
		"FAILING TEST: retrieveAllChunksForRepository not implemented - this test should fail until real implementation exists",
	)
	return []TestChunk{}
}

func createRepositoryWithProcessingErrors(t *testing.T) TestRepository {
	// This should fail until actual implementation exists
	t.Fatal(
		"FAILING TEST: createRepositoryWithProcessingErrors not implemented - this test should fail until real implementation exists",
	)
	return TestRepository{}
}

func executeJobProcessorPipelineWithErrors(
	ctx context.Context,
	t *testing.T,
	jobID, repoID uuid.UUID,
	repoPath string,
) LeakageJobProcessorResult {
	// This should fail until actual implementation exists
	t.Fatal(
		"FAILING TEST: executeJobProcessorPipelineWithErrors not implemented - this test should fail until real implementation exists",
	)
	return LeakageJobProcessorResult{}
}

func storeLegacyChunksInDatabase(ctx context.Context, t *testing.T, chunks []TestChunk, repoID uuid.UUID) {
	// This should fail until actual implementation exists
	t.Fatal(
		"FAILING TEST: storeLegacyChunksInDatabase not implemented - this test should fail until real implementation exists",
	)
}

func executePathMigration(ctx context.Context, t *testing.T, repoID uuid.UUID) LeakageMigrationResult {
	// This should fail until actual implementation exists
	t.Fatal(
		"FAILING TEST: executePathMigration not implemented - this test should fail until real implementation exists",
	)
	return LeakageMigrationResult{}
}
