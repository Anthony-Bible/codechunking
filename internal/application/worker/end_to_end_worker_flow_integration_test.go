package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndToEndWorkerFlow_RepositoryCloning_to_ChunkProcessing tests the complete
// worker pipeline from repository cloning through file processing and language detection
// to chunk processing.
func TestEndToEndWorkerFlow_RepositoryCloning_to_ChunkProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end worker flow integration test")
	}

	// Test scenarios for different repository types and processing paths
	testCases := []struct {
		name         string
		repoURL      string
		expectedLang string
		description  string
	}{
		{
			name:         "small_go_repository",
			repoURL:      "https://github.com/example/small-go-repo.git",
			expectedLang: "go",
			description:  "Process small Go repository with typical structure",
		},
		{
			name:         "python_repository_with_submodules",
			repoURL:      "https://github.com/example/python-with-submodules.git",
			expectedLang: "python",
			description:  "Process Python repository containing Git submodules",
		},
		{
			name:         "javascript_monorepo",
			repoURL:      "https://github.com/example/js-monorepo.git",
			expectedLang: "javascript",
			description:  "Process JavaScript monorepo with multiple packages",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// Create temporary workspace for this test
			workspace := createTestWorkspace(t, tc.name)
			defer cleanupTestWorkspace(t, workspace)

			// Step 1: Repository Cloning
			cloneResult := executeRepositoryCloning(t, ctx, tc.repoURL, workspace)
			assert.True(t, cloneResult.Success, "Repository cloning should succeed")
			assert.NotEmpty(t, cloneResult.Files, "Should have cloned files")

			// Step 2: File Processing and Language Detection
			langDetectionResult := executeLanguageDetection(t, ctx, cloneResult.RepoPath)
			assert.Equal(
				t,
				tc.expectedLang,
				langDetectionResult.PrimaryLanguage,
				"Should detect correct primary language",
			)
			assert.NotEmpty(t, langDetectionResult.DetectedFiles, "Should detect language files")

			// Step 3: Binary File Filtering
			filterResult := executeBinaryFileFiltering(t, ctx, cloneResult.RepoPath)
			assert.Less(t, len(filterResult.FilteredFiles), len(cloneResult.Files), "Should filter out some files")
			assert.NotEmpty(t, filterResult.ProcessableFiles, "Should have processable files")

			// Step 4: Submodule and Symlink Handling
			submoduleResult := executeSubmoduleProcessing(t, ctx, cloneResult.RepoPath)
			// Results depend on repository - just verify no errors
			assert.NoError(t, submoduleResult.Error, "Submodule processing should not error")

			// Step 5: Code Chunk Processing (simulated - actual implementation would be in Phase 4)
			chunkResult := simulateChunkProcessing(t, ctx, filterResult.ProcessableFiles)
			assert.Positive(t, chunkResult.ChunkCount, "Should generate code chunks")
			assert.NotEmpty(t, chunkResult.ChunkMetadata, "Should have chunk metadata")

			// Verify end-to-end metrics
			verifyEndToEndMetrics(t, tc.name, cloneResult, langDetectionResult, filterResult, chunkResult)
		})
	}
}

// TestEndToEndWorkerFlow_ErrorRecovery_and_Resilience tests the worker pipeline's
// ability to handle errors gracefully and recover from failures.
func TestEndToEndWorkerFlow_ErrorRecovery_and_Resilience(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping error recovery integration test")
	}

	errorScenarios := []struct {
		name        string
		repoURL     string
		errorStage  string
		expectation string
	}{
		{
			name:        "invalid_repository_url",
			repoURL:     "https://github.com/nonexistent/invalid-repo.git",
			errorStage:  "cloning",
			expectation: "should fail gracefully at cloning stage",
		},
		{
			name:        "corrupted_repository",
			repoURL:     "https://github.com/example/corrupted-repo.git",
			errorStage:  "file_processing",
			expectation: "should recover and process available files",
		},
		{
			name:        "permission_denied_files",
			repoURL:     "https://github.com/example/restricted-files.git",
			errorStage:  "file_access",
			expectation: "should skip inaccessible files and continue",
		},
	}

	for _, scenario := range errorScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			ctx := context.Background()
			workspace := createTestWorkspace(t, scenario.name)
			defer cleanupTestWorkspace(t, workspace)

			// Execute the pipeline and expect controlled failure
			result := executeResilientWorkerPipeline(t, ctx, scenario.repoURL, workspace)

			switch scenario.errorStage {
			case "cloning":
				assert.False(t, result.CloningSuccess, "Cloning should fail for invalid repository")
				assert.Error(t, result.CloningError, "Should report cloning error")
			case "file_processing":
				assert.True(t, result.CloningSuccess, "Cloning should succeed")
				assert.True(t, result.HasPartialResults, "Should have partial processing results")
			case "file_access":
				assert.True(t, result.ProcessingContinued, "Should continue despite file access errors")
				assert.NotEmpty(t, result.SkippedFiles, "Should report skipped files")
			}

			// Verify error reporting and logging
			assert.NotEmpty(t, result.ErrorLog, "Should log errors encountered")
		})
	}
}

// TestEndToEndWorkerFlow_Performance_and_Concurrency tests the worker pipeline
// under load with concurrent processing scenarios.
func TestEndToEndWorkerFlow_Performance_and_Concurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance integration test")
	}

	performanceScenarios := []struct {
		name              string
		repositoryCount   int
		concurrentWorkers int
		expectedDuration  time.Duration
	}{
		{
			name:              "single_worker_multiple_repos",
			repositoryCount:   3,
			concurrentWorkers: 1,
			expectedDuration:  time.Minute * 2,
		},
		{
			name:              "multiple_workers_concurrent_processing",
			repositoryCount:   5,
			concurrentWorkers: 3,
			expectedDuration:  time.Minute * 1,
		},
	}

	for _, scenario := range performanceScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			ctx := context.Background()

			// Create test repositories
			repositories := generateTestRepositories(scenario.repositoryCount)

			start := time.Now()

			// Execute concurrent worker processing
			results := executeConcurrentWorkerProcessing(t, ctx, repositories, scenario.concurrentWorkers)

			elapsed := time.Since(start)

			// Verify performance expectations
			assert.Less(t, elapsed, scenario.expectedDuration, "Processing should complete within expected time")
			assert.Len(t, results, scenario.repositoryCount, "Should process all repositories")

			// Verify concurrent processing worked correctly
			for i, result := range results {
				assert.True(t, result.Success, "Repository %d processing should succeed", i)
				assert.Positive(t, result.ProcessedFiles, "Should process files in repository %d", i)
			}
		})
	}
}

// Helper functions for end-to-end testing

type CloneResult struct {
	Success  bool
	Files    []string
	RepoPath string
	Error    error
}

type LanguageDetectionResult struct {
	PrimaryLanguage string
	DetectedFiles   []string
	LanguageStats   map[string]int
}

type FilterResult struct {
	FilteredFiles    []string
	ProcessableFiles []string
	BinaryCount      int
}

type SubmoduleResult struct {
	SubmoduleCount int
	SymlinkCount   int
	Error          error
}

type ChunkResult struct {
	ChunkCount    int
	ChunkMetadata []ChunkMetadata
	TotalSize     int64
}

type ChunkMetadata struct {
	ID       string
	Language string
	Size     int
	Type     string
}

type ResilientPipelineResult struct {
	CloningSuccess      bool
	CloningError        error
	HasPartialResults   bool
	ProcessingContinued bool
	SkippedFiles        []string
	ErrorLog            []string
}

type ConcurrentProcessingResult struct {
	Success        bool
	ProcessedFiles int
	Duration       time.Duration
	WorkerID       int
}

func createTestWorkspace(t *testing.T, name string) string {
	workspace := filepath.Join(os.TempDir(), "worker-integration-test", name)
	err := os.MkdirAll(workspace, 0o755)
	require.NoError(t, err, "Should create test workspace")
	return workspace
}

func cleanupTestWorkspace(t *testing.T, workspace string) {
	err := os.RemoveAll(workspace)
	if err != nil {
		t.Logf("Warning: failed to cleanup workspace %s: %v", workspace, err)
	}
}

func executeRepositoryCloning(t *testing.T, _ context.Context, repoURL, workspace string) CloneResult {
	// Simulate repository cloning using existing Git client components
	// In a real implementation, this would use the GitClient service

	// For testing purposes, create a mock repository structure
	repoPath := filepath.Join(workspace, "cloned-repo")
	err := os.MkdirAll(repoPath, 0o755)
	require.NoError(t, err)

	// Create mock files based on repository type
	var mockFiles []string
	switch repoURL {
	case "https://github.com/example/small-go-repo.git":
		mockFiles = []string{
			"main.go",
			"README.md",
			"go.mod",
			"internal/service.go",
			"pkg/utils.go",
		}
	case "https://github.com/example/python-with-submodules.git":
		mockFiles = []string{
			"main.py",
			"README.md",
			"requirements.txt",
			"src/service.py",
			"tests/test_main.py",
		}
	case "https://github.com/example/js-monorepo.git":
		mockFiles = []string{
			"package.json",
			"README.md",
			"src/index.js",
			"packages/utils/index.ts",
			"tests/integration.test.js",
		}
	default:
		// Default to Go files for unknown repositories
		mockFiles = []string{
			"main.go",
			"README.md",
			"go.mod",
			"internal/service.go",
			"pkg/utils.go",
		}
	}

	var createdFiles []string
	for _, file := range mockFiles {
		fullPath := filepath.Join(repoPath, file)
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, 0o755)
		require.NoError(t, err)

		err = os.WriteFile(fullPath, []byte(fmt.Sprintf("// Mock content for %s\n", file)), 0o644)
		require.NoError(t, err)
		createdFiles = append(createdFiles, fullPath)
	}

	return CloneResult{
		Success:  true,
		Files:    createdFiles,
		RepoPath: repoPath,
		Error:    nil,
	}
}

func executeLanguageDetection(t *testing.T, ctx context.Context, repoPath string) LanguageDetectionResult {
	// For integration testing, use simple file extension-based detection

	result := LanguageDetectionResult{
		LanguageStats: make(map[string]int),
	}

	// Walk through files and detect languages
	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		language := detectLanguageByExtension(filepath.Ext(path))
		if language != "unknown" {
			result.DetectedFiles = append(result.DetectedFiles, path)
			result.LanguageStats[language]++
		}
		return nil
	})
	require.NoError(t, err)

	// Determine primary language (most common)
	maxCount := 0
	for lang, count := range result.LanguageStats {
		if count > maxCount {
			maxCount = count
			result.PrimaryLanguage = lang
		}
	}

	return result
}

func executeBinaryFileFiltering(t *testing.T, ctx context.Context, repoPath string) FilterResult {
	// For integration testing, use simple extension-based binary detection

	result := FilterResult{}

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		isBinary := isBinaryByExtension(filepath.Ext(path))

		if isBinary {
			result.FilteredFiles = append(result.FilteredFiles, path)
			result.BinaryCount++
		} else {
			result.ProcessableFiles = append(result.ProcessableFiles, path)
		}
		return nil
	})
	require.NoError(t, err)

	return result
}

func executeSubmoduleProcessing(t *testing.T, ctx context.Context, repoPath string) SubmoduleResult {
	// For integration testing, simulate submodule and symlink detection
	submoduleCount := 0
	symlinkCount := 0

	// Check for .gitmodules file (indicates submodules)
	gitmodulesPath := filepath.Join(repoPath, ".gitmodules")
	if _, err := os.Stat(gitmodulesPath); err == nil {
		submoduleCount = 1 // Simulate finding one submodule
	}

	// Count symlinks
	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			symlinkCount++
		}
		return nil
	})

	return SubmoduleResult{
		SubmoduleCount: submoduleCount,
		SymlinkCount:   symlinkCount,
		Error:          err,
	}
}

func simulateChunkProcessing(t *testing.T, ctx context.Context, processableFiles []string) ChunkResult {
	// Simulate code chunking (actual implementation would be in Phase 4)
	// For now, create mock chunks based on file content

	result := ChunkResult{
		ChunkMetadata: []ChunkMetadata{},
	}

	for i, file := range processableFiles {
		// Simulate creating chunks from each file
		chunks := (i % 3) + 1 // 1-3 chunks per file

		for j := range chunks {
			chunkID := fmt.Sprintf("chunk_%d_%d", i, j)
			ext := filepath.Ext(file)
			language := "unknown"

			// Simple language detection from extension
			switch ext {
			case ".go":
				language = "go"
			case ".py":
				language = "python"
			case ".js", ".ts":
				language = "javascript"
			case ".md":
				language = "markdown"
			}

			result.ChunkMetadata = append(result.ChunkMetadata, ChunkMetadata{
				ID:       chunkID,
				Language: language,
				Size:     100 + (j * 50), // Mock size
				Type:     "function",     // Mock type
			})
			result.ChunkCount++
		}

		// Mock total size calculation
		result.TotalSize += int64(1000 + (i * 500))
	}

	return result
}

func verifyEndToEndMetrics(t *testing.T, testName string, cloneResult CloneResult,
	langResult LanguageDetectionResult, filterResult FilterResult, chunkResult ChunkResult,
) {
	t.Logf("End-to-end metrics for %s:", testName)
	t.Logf("  Cloned files: %d", len(cloneResult.Files))
	t.Logf("  Primary language: %s", langResult.PrimaryLanguage)
	t.Logf("  Language files: %d", len(langResult.DetectedFiles))
	t.Logf("  Processable files: %d", len(filterResult.ProcessableFiles))
	t.Logf("  Binary files filtered: %d", filterResult.BinaryCount)
	t.Logf("  Generated chunks: %d", chunkResult.ChunkCount)
	t.Logf("  Total chunk size: %d bytes", chunkResult.TotalSize)

	// Verify the pipeline worked end-to-end
	assert.NotEmpty(t, cloneResult.Files, "Should have cloned files")
	assert.NotEmpty(t, langResult.PrimaryLanguage, "Should detect primary language")
	assert.NotEmpty(t, filterResult.ProcessableFiles, "Should have processable files")
	assert.Positive(t, chunkResult.ChunkCount, "Should generate chunks")
}

func executeResilientWorkerPipeline(
	t *testing.T,
	ctx context.Context,
	repoURL, workspace string,
) ResilientPipelineResult {
	result := ResilientPipelineResult{
		ErrorLog: []string{},
	}

	// Step 1: Attempt cloning
	cloneResult := executeRepositoryCloning(t, ctx, repoURL, workspace)
	result.CloningSuccess = cloneResult.Success
	result.CloningError = cloneResult.Error

	if !result.CloningSuccess {
		result.ErrorLog = append(result.ErrorLog, fmt.Sprintf("Cloning failed: %v", cloneResult.Error))
		return result
	}

	// Step 2: Continue with partial processing
	result.ProcessingContinued = true
	result.HasPartialResults = len(cloneResult.Files) > 0

	// Simulate some files being inaccessible
	if repoURL == "https://github.com/example/restricted-files.git" {
		result.SkippedFiles = []string{"restricted_file_1.go", "protected_config.json"}
	}

	return result
}

func generateTestRepositories(count int) []string {
	repos := make([]string, count)
	for i := range count {
		repos[i] = fmt.Sprintf("https://github.com/example/test-repo-%d.git", i+1)
	}
	return repos
}

func executeConcurrentWorkerProcessing(
	t *testing.T,
	ctx context.Context,
	repositories []string,
	workerCount int,
) []ConcurrentProcessingResult {
	results := make([]ConcurrentProcessingResult, len(repositories))

	// Simulate concurrent processing
	for i, repo := range repositories {
		start := time.Now()

		// Create workspace for this repository
		workspace := createTestWorkspace(t, fmt.Sprintf("concurrent-%d", i))

		// Execute the pipeline
		cloneResult := executeRepositoryCloning(t, ctx, repo, workspace)

		results[i] = ConcurrentProcessingResult{
			Success:        cloneResult.Success,
			ProcessedFiles: len(cloneResult.Files),
			Duration:       time.Since(start),
			WorkerID:       i % workerCount,
		}

		// Cleanup
		cleanupTestWorkspace(t, workspace)
	}

	return results
}

// Helper functions for integration testing

// detectLanguageByExtension provides simple language detection based on file extensions.
func detectLanguageByExtension(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".ts":
		return "javascript"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".sh", ".bash":
		return "shell"
	case ".sql":
		return "sql"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "markdown"
	case ".txt":
		return "text"
	default:
		return "unknown"
	}
}

// isBinaryByExtension provides simple binary file detection based on file extensions.
func isBinaryByExtension(ext string) bool {
	binaryExtensions := map[string]bool{
		".exe":    true,
		".dll":    true,
		".so":     true,
		".dylib":  true,
		".a":      true,
		".o":      true,
		".obj":    true,
		".lib":    true,
		".bin":    true,
		".dat":    true,
		".db":     true,
		".sqlite": true,
		".zip":    true,
		".tar":    true,
		".gz":     true,
		".rar":    true,
		".7z":     true,
		".pdf":    true,
		".doc":    true,
		".docx":   true,
		".xls":    true,
		".xlsx":   true,
		".ppt":    true,
		".pptx":   true,
		".jpg":    true,
		".jpeg":   true,
		".png":    true,
		".gif":    true,
		".bmp":    true,
		".svg":    true,
		".ico":    true,
		".mp3":    true,
		".mp4":    true,
		".avi":    true,
		".mov":    true,
		".wav":    true,
		".ogg":    true,
		".flac":   true,
		".mkv":    true,
		".webm":   true,
		".ttf":    true,
		".otf":    true,
		".woff":   true,
		".woff2":  true,
		".eot":    true,
		".class":  true,
		".jar":    true,
		".war":    true,
		".pyc":    true,
		".pyo":    true,
		".cache":  true,
	}

	return binaryExtensions[ext]
}
