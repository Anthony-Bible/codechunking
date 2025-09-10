//go:build disabled
// +build disabled

package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRealCodebaseIntegration tests the complete integration pipeline with real codebase files
// This is the main integration test that validates end-to-end processing of actual code
func TestRealCodebaseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real codebase integration test in short mode")
	}

	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	testFixturesPath := filepath.Join("testdata", "real_codebases")

	// Test with real codebase samples
	testCases := []struct {
		name         string
		language     valueobject.Language
		fixtureDir   string
		expectedType outbound.SemanticConstructType
		minChunks    int
	}{
		{
			name:         "Go microservice components",
			language:     mustNewLanguage(valueobject.LanguageGo),
			fixtureDir:   "go_microservice",
			expectedType: outbound.ConstructFunction,
			minChunks:    5,
		},
		{
			name:         "Python Flask application",
			language:     mustNewLanguage(valueobject.LanguagePython),
			fixtureDir:   "python_flask_app",
			expectedType: outbound.ConstructClass,
			minChunks:    3,
		},
		{
			name:         "JavaScript React components",
			language:     mustNewLanguage(valueobject.LanguageJavaScript),
			fixtureDir:   "js_react_components",
			expectedType: outbound.ConstructClass,
			minChunks:    2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixtureDir := filepath.Join(testFixturesPath, tc.fixtureDir)

			// This should fail because test fixtures don't exist yet
			files, err := findCodeFiles(fixtureDir, tc.language)
			require.NoError(t, err, "Should find real code files in fixtures")
			require.Greater(t, len(files), 0, "Should have at least one real code file")

			parser, err := factory.CreateParser(ctx, tc.language)
			require.NoError(t, err)

			totalChunks := 0
			for _, file := range files {
				content, err := os.ReadFile(file)
				require.NoError(t, err, "Should read real code file")

				tree, err := parser.Parse(ctx, content)
				require.NoError(t, err, "Should parse real code successfully")

				converter := NewTreeSitterParseTreeConverter()
				domainTree, err := converter.ConvertToDomain(ctx, tree)
				require.NoError(t, err, "Should convert real code parse tree")

				extractor := NewSemanticCodeChunkExtractor()
				chunks, err := extractor.Extract(ctx, domainTree)
				require.NoError(t, err, "Should extract chunks from real code")

				totalChunks += len(chunks)

				// Validate chunk quality with real code
				for _, chunk := range chunks {
					assert.NotEmpty(t, chunk.ID(), "Real code chunk should have ID")
					assert.NotEmpty(t, chunk.Type, "Real code chunk should have type")
					assert.NotEmpty(t, chunk.Content, "Real code chunk should have content")
					assert.Equal(t, tc.language.Name(), chunk.Language.Name())
					assert.NotNil(t, chunk.Metadata, "Real code chunk should have metadata")

					// Real code should have meaningful names
					assert.NotEmpty(t, chunk.Name, "Real code chunk should have meaningful name")

					// Validate positions are reasonable for real code
					assert.Greater(t, chunk.EndByte, chunk.StartByte)
					assert.GreaterOrEqual(t, chunk.EndPosition.Line, chunk.StartPosition.Line)
				}
			}

			assert.GreaterOrEqual(t, totalChunks, tc.minChunks,
				"Should extract minimum expected chunks from real codebase")
		})
	}
}

// TestFileSystemIntegration tests reading and processing files from the filesystem
func TestFileSystemIntegration(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	// Create a temporary directory with real code samples
	tempDir := t.TempDir()

	// This will fail because we haven't created the real code samples yet
	realCodeSamples := getRealCodeSamples()

	for filename, content := range realCodeSamples {
		filePath := filepath.Join(tempDir, filename)

		// Create directory if needed
		dir := filepath.Dir(filePath)
		err := os.MkdirAll(dir, 0o755)
		require.NoError(t, err)

		err = os.WriteFile(filePath, []byte(content), 0o644)
		require.NoError(t, err)
	}

	// Process all files in the temporary directory
	err = filepath.WalkDir(tempDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Determine language from file extension
		lang := detectLanguageFromPath(path)
		if lang.Name() == "unknown" {
			return nil
		}

		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		content, err := os.ReadFile(path)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, content)
		require.NoError(t, err, "Should parse real file: %s", path)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(ctx, tree)
		require.NoError(t, err, "Should convert real file: %s", path)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err, "Should extract from real file: %s", path)

		assert.Greater(t, len(chunks), 0, "Should extract chunks from real file: %s", path)

		return nil
	})
	require.NoError(t, err)
}

// TestMultiFileProjectProcessing tests processing related files in a project structure
func TestMultiFileProjectProcessing(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	// This should fail because we haven't created the multi-file project structure yet
	projectDir := filepath.Join("testdata", "real_projects", "go_web_service")

	// Expected project structure:
	// main.go - entry point
	// handlers/ - HTTP handlers
	// models/ - data models
	// services/ - business logic
	// utils/ - utility functions

	expectedFiles := []string{
		"main.go",
		"handlers/user_handler.go",
		"handlers/health_handler.go",
		"models/user.go",
		"services/user_service.go",
		"utils/validator.go",
	}

	allChunks := make(map[string][]outbound.SemanticCodeChunk)

	for _, relativeFile := range expectedFiles {
		filePath := filepath.Join(projectDir, relativeFile)

		// This will fail because files don't exist
		_, err := os.Stat(filePath)
		require.NoError(t, err, "Real project file should exist: %s", filePath)

		content, err := os.ReadFile(filePath)
		require.NoError(t, err)

		lang := detectLanguageFromPath(filePath)
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, content)
		require.NoError(t, err, "Should parse project file: %s", filePath)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(ctx, tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		allChunks[relativeFile] = chunks
		assert.Greater(t, len(chunks), 0, "Should extract from project file: %s", filePath)
	}

	// Validate cross-file relationships
	totalChunks := 0
	for _, chunks := range allChunks {
		totalChunks += len(chunks)
	}
	assert.Greater(t, totalChunks, 10, "Should extract substantial chunks from real project")

	// Verify main.go has import dependencies
	mainChunks := allChunks["main.go"]
	hasImports := false
	for _, chunk := range mainChunks {
		if len(chunk.Dependencies) > 0 {
			hasImports = true
			break
		}
	}
	assert.True(t, hasImports, "Main file should have import dependencies")
}

// TestRepositoryLevelProcessing tests processing an entire small repository
func TestRepositoryLevelProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping repository-level test in short mode")
	}

	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	// This should fail because we haven't set up the repository fixture
	repoPath := filepath.Join("testdata", "real_repositories", "sample_cli_tool")

	// Expected repository structure with real code from a small CLI tool
	_, err = os.Stat(repoPath)
	require.NoError(t, err, "Sample repository should exist")

	// Process the entire repository
	allChunks := make(map[valueobject.Language][]outbound.SemanticCodeChunk)
	fileCount := 0

	err = filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || strings.Contains(path, ".git") {
			return nil
		}

		lang := detectLanguageFromPath(path)
		if lang.Name() == "unknown" {
			return nil
		}

		fileCount++
		content, err := os.ReadFile(path)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		tree, err := parser.Parse(ctx, content)
		require.NoError(t, err, "Should parse repository file: %s", path)

		converter := NewTreeSitterParseTreeConverter()
		domainTree, err := converter.ConvertToDomain(ctx, tree)
		require.NoError(t, err)

		extractor := NewSemanticCodeChunkExtractor()
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)

		allChunks[lang] = append(allChunks[lang], chunks...)
		return nil
	})
	require.NoError(t, err)

	// Validate repository processing results
	assert.Greater(t, fileCount, 5, "Should process multiple files from repository")
	assert.Greater(t, len(allChunks), 0, "Should extract chunks from repository")

	// Verify we have a good mix of construct types
	constructTypes := make(map[outbound.SemanticConstructType]int)
	for _, languageChunks := range allChunks {
		for _, chunk := range languageChunks {
			constructTypes[chunk.Type]++
		}
	}
	assert.Greater(t, len(constructTypes), 2, "Repository should have diverse construct types")
}

// TestCrossLanguageProjectProcessing tests projects with multiple programming languages
func TestCrossLanguageProjectProcessing(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	// This should fail because we haven't created the polyglot project
	projectDir := filepath.Join("testdata", "real_projects", "polyglot_webapp")

	// Expected structure: Go backend, JavaScript frontend, Python scripts
	languageFiles := map[valueobject.Language][]string{
		mustNewLanguage(valueobject.LanguageGo): {
			"backend/main.go",
			"backend/handlers/api.go",
			"backend/models/user.go",
		},
		mustNewLanguage(valueobject.LanguageJavaScript): {
			"frontend/src/App.js",
			"frontend/src/components/UserList.js",
			"frontend/src/utils/api.js",
		},
		mustNewLanguage(valueobject.LanguagePython): {
			"scripts/migrate.py",
			"scripts/seed_data.py",
		},
	}

	languageChunks := make(map[valueobject.Language][]outbound.SemanticCodeChunk)

	for lang, files := range languageFiles {
		parser, err := factory.CreateParser(ctx, lang)
		require.NoError(t, err)

		for _, relativeFile := range files {
			filePath := filepath.Join(projectDir, relativeFile)

			// This will fail because files don't exist
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "Should read polyglot project file: %s", filePath)

			tree, err := parser.Parse(ctx, content)
			require.NoError(t, err, "Should parse polyglot file: %s", filePath)

			converter := NewTreeSitterParseTreeConverter()
			domainTree, err := converter.ConvertToDomain(ctx, tree)
			require.NoError(t, err)

			extractor := NewSemanticCodeChunkExtractor()
			chunks, err := extractor.Extract(ctx, domainTree)
			require.NoError(t, err)

			languageChunks[lang] = append(languageChunks[lang], chunks...)
		}
	}

	// Validate cross-language processing
	assert.Equal(t, 3, len(languageChunks), "Should process all three languages")

	for lang, chunks := range languageChunks {
		assert.Greater(t, len(chunks), 0, "Should have chunks for language: %s", lang.Name())

		// Validate language consistency
		for _, chunk := range chunks {
			assert.Equal(t, lang.Name(), chunk.Language.Name())
		}
	}

	// Calculate total chunks across languages
	totalChunks := 0
	for _, chunks := range languageChunks {
		totalChunks += len(chunks)
	}
	assert.Greater(t, totalChunks, 15, "Should extract substantial chunks from polyglot project")
}

// TestPerformanceWithRealCodebases benchmarks performance on real codebases
func TestPerformanceWithRealCodebases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	// This should fail because performance test fixtures don't exist
	perfTestDir := filepath.Join("testdata", "performance", "large_codebases")

	testCases := []struct {
		name      string
		directory string
		language  valueobject.Language
		maxTime   time.Duration
		minFiles  int
	}{
		{
			name:      "Large Go service",
			directory: "go_large_service",
			language:  mustNewLanguage(valueobject.LanguageGo),
			maxTime:   30 * time.Second,
			minFiles:  50,
		},
		{
			name:      "Python Django application",
			directory: "python_django_app",
			language:  mustNewLanguage(valueobject.LanguagePython),
			maxTime:   45 * time.Second,
			minFiles:  30,
		},
		{
			name:      "JavaScript Node.js API",
			directory: "js_nodejs_api",
			language:  mustNewLanguage(valueobject.LanguageJavaScript),
			maxTime:   25 * time.Second,
			minFiles:  25,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testDir := filepath.Join(perfTestDir, tc.directory)

			// This will fail because test directories don't exist
			_, err := os.Stat(testDir)
			require.NoError(t, err, "Performance test directory should exist: %s", testDir)

			parser, err := factory.CreateParser(ctx, tc.language)
			require.NoError(t, err)

			startTime := time.Now()
			var totalChunks int
			var fileCount int

			err = filepath.WalkDir(testDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				if d.IsDir() || !isCodeFile(path, tc.language) {
					return nil
				}

				fileCount++
				content, err := os.ReadFile(path)
				require.NoError(t, err)

				tree, err := parser.Parse(ctx, content)
				require.NoError(t, err)

				converter := NewTreeSitterParseTreeConverter()
				domainTree, err := converter.ConvertToDomain(ctx, tree)
				require.NoError(t, err)

				extractor := NewSemanticCodeChunkExtractor()
				chunks, err := extractor.Extract(ctx, domainTree)
				require.NoError(t, err)

				totalChunks += len(chunks)
				return nil
			})
			require.NoError(t, err)

			duration := time.Since(startTime)

			// Performance assertions
			assert.Less(t, duration, tc.maxTime, "Should complete within time limit")
			assert.GreaterOrEqual(t, fileCount, tc.minFiles, "Should process minimum expected files")
			assert.Greater(t, totalChunks, fileCount, "Should extract more chunks than files")

			// Memory usage check
			var m runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m)
			assert.Less(t, m.Alloc, uint64(500*1024*1024), "Should use less than 500MB memory")

			t.Logf("Performance results for %s: %d files, %d chunks, %v duration, %d MB memory",
				tc.name, fileCount, totalChunks, duration, m.Alloc/(1024*1024))
		})
	}
}

// TestErrorHandlingWithRealMalformedFiles tests error handling with actual malformed code files
func TestErrorHandlingWithRealMalformedFiles(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	// This should fail because malformed test files don't exist
	malformedDir := filepath.Join("testdata", "malformed_code")

	testCases := []struct {
		name       string
		filename   string
		language   valueobject.Language
		expectErr  bool
		shouldFail string
	}{
		{
			name:       "Go syntax error",
			filename:   "syntax_error.go",
			language:   mustNewLanguage(valueobject.LanguageGo),
			expectErr:  true,
			shouldFail: "parsing",
		},
		{
			name:       "Python indentation error",
			filename:   "indentation_error.py",
			language:   mustNewLanguage(valueobject.LanguagePython),
			expectErr:  true,
			shouldFail: "parsing",
		},
		{
			name:       "JavaScript missing brackets",
			filename:   "missing_brackets.js",
			language:   mustNewLanguage(valueobject.LanguageJavaScript),
			expectErr:  true,
			shouldFail: "parsing",
		},
		{
			name:       "Truncated Go file",
			filename:   "truncated.go",
			language:   mustNewLanguage(valueobject.LanguageGo),
			expectErr:  true,
			shouldFail: "conversion or extraction",
		},
		{
			name:       "Binary file with code extension",
			filename:   "binary_file.go",
			language:   mustNewLanguage(valueobject.LanguageGo),
			expectErr:  true,
			shouldFail: "parsing",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(malformedDir, tc.filename)

			// This will fail because malformed files don't exist
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "Should read malformed test file: %s", filePath)

			parser, err := factory.CreateParser(ctx, tc.language)
			require.NoError(t, err)

			// Try to parse - might succeed or fail depending on the error type
			tree, parseErr := parser.Parse(ctx, content)

			// Try to convert - might handle malformed trees gracefully
			converter := NewTreeSitterParseTreeConverter()
			domainTree, convertErr := converter.ConvertToDomain(ctx, tree)

			// Try to extract - should handle gracefully even with errors
			extractor := NewSemanticCodeChunkExtractor()
			chunks, extractErr := extractor.Extract(ctx, domainTree)

			if tc.expectErr {
				// At least one step should fail, but shouldn't panic
				hasError := parseErr != nil || convertErr != nil || extractErr != nil
				assert.True(t, hasError, "Should have error in pipeline for malformed file: %s", tc.filename)

				// Should not panic and return a reasonable result
				assert.NotNil(t, chunks, "Should return chunks slice even on error")
			}

			// Log what actually happened for debugging
			t.Logf("File %s: parse_err=%v, convert_err=%v, extract_err=%v, chunks=%d",
				tc.filename, parseErr != nil, convertErr != nil, extractErr != nil, len(chunks))
		})
	}
}

// TestRealCodeMetadataExtraction tests extraction of rich metadata from real code
func TestRealCodeMetadataExtraction(t *testing.T) {
	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	// This should fail because metadata test files don't exist
	metadataTestDir := filepath.Join("testdata", "metadata_rich")

	testCases := []struct {
		name           string
		filename       string
		language       valueobject.Language
		expectedFields map[string]interface{}
	}{
		{
			name:     "Go with comprehensive documentation",
			filename: "documented_service.go",
			language: mustNewLanguage(valueobject.LanguageGo),
			expectedFields: map[string]interface{}{
				"has_documentation": true,
				"has_parameters":    true,
				"has_return_type":   true,
				"visibility":        "public",
			},
		},
		{
			name:     "Python with type hints and decorators",
			filename: "typed_decorated_class.py",
			language: mustNewLanguage(valueobject.LanguagePython),
			expectedFields: map[string]interface{}{
				"has_decorators": true,
				"has_type_hints": true,
				"has_docstrings": true,
				"is_async":       true,
			},
		},
		{
			name:     "JavaScript with JSDoc and modern features",
			filename: "modern_class.js",
			language: mustNewLanguage(valueobject.LanguageJavaScript),
			expectedFields: map[string]interface{}{
				"has_jsdoc":          true,
				"uses_async_await":   true,
				"has_private_fields": true,
				"is_exported":        true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(metadataTestDir, tc.filename)

			// This will fail because metadata test files don't exist
			content, err := os.ReadFile(filePath)
			require.NoError(t, err, "Should read metadata test file: %s", filePath)

			parser, err := factory.CreateParser(ctx, tc.language)
			require.NoError(t, err)

			tree, err := parser.Parse(ctx, content)
			require.NoError(t, err, "Should parse metadata-rich file: %s", filePath)

			converter := NewTreeSitterParseTreeConverter()
			domainTree, err := converter.ConvertToDomain(ctx, tree)
			require.NoError(t, err)

			extractor := NewSemanticCodeChunkExtractor()
			chunks, err := extractor.Extract(ctx, domainTree)
			require.NoError(t, err)

			// Validate rich metadata extraction
			assert.Greater(t, len(chunks), 0, "Should extract chunks from metadata-rich file")

			foundExpectedMetadata := false
			for _, chunk := range chunks {
				if chunk.Metadata != nil {
					foundExpectedMetadata = true

					// Check for expected metadata fields
					for field, expectedValue := range tc.expectedFields {
						if actualValue, exists := chunk.Metadata[field]; exists {
							assert.Equal(t, expectedValue, actualValue,
								"Metadata field %s should match expected value for chunk %s", field, chunk.Name)
						}
					}
				}

				// Validate comprehensive chunk structure
				assert.NotEmpty(t, chunk.ID, "Chunk should have ID")
				assert.NotEmpty(t, chunk.Content, "Chunk should have content")
				assert.NotEmpty(t, chunk.Hash, "Chunk should have content hash")
				assert.False(t, chunk.ExtractedAt.IsZero(), "Chunk should have extraction timestamp")
			}

			assert.True(t, foundExpectedMetadata, "Should find chunks with rich metadata")
		})
	}
}

// TestConcurrentRealFileProcessing tests concurrent processing of real files
func TestConcurrentRealFileProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent processing test in short mode")
	}

	ctx := context.Background()
	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	// This should fail because concurrent test directory doesn't exist
	concurrentTestDir := filepath.Join("testdata", "concurrent_processing")

	// Should have multiple files for each language
	languages := []valueobject.Language{
		mustNewLanguage(valueobject.LanguageGo),
		mustNewLanguage(valueobject.LanguagePython),
		mustNewLanguage(valueobject.LanguageJavaScript),
	}

	for _, lang := range languages {
		t.Run(fmt.Sprintf("Concurrent_%s_processing", lang.Name()), func(t *testing.T) {
			langDir := filepath.Join(concurrentTestDir, lang.Name())

			// This will fail because language directories don't exist
			files, err := filepath.Glob(filepath.Join(langDir, "*"+getTestFileExtension(lang)))
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(files), 5, "Should have multiple files for concurrent test")

			parser, err := factory.CreateParser(ctx, lang)
			require.NoError(t, err)

			// Process files concurrently
			resultChan := make(chan []outbound.SemanticCodeChunk, len(files))
			errorChan := make(chan error, len(files))

			for _, file := range files {
				go func(filePath string) {
					content, err := os.ReadFile(filePath)
					if err != nil {
						errorChan <- err
						return
					}

					tree, err := parser.Parse(ctx, content)
					if err != nil {
						errorChan <- err
						return
					}

					converter := NewTreeSitterParseTreeConverter()
					domainTree, err := converter.ConvertToDomain(ctx, tree)
					if err != nil {
						errorChan <- err
						return
					}

					extractor := NewSemanticCodeChunkExtractor()
					chunks, err := extractor.Extract(ctx, domainTree)
					if err != nil {
						errorChan <- err
						return
					}

					resultChan <- chunks
				}(file)
			}

			// Collect results
			totalChunks := 0
			for i := 0; i < len(files); i++ {
				select {
				case chunks := <-resultChan:
					totalChunks += len(chunks)
				case err := <-errorChan:
					require.NoError(t, err, "Concurrent processing should not fail")
				case <-time.After(30 * time.Second):
					t.Fatal("Concurrent processing timeout")
				}
			}

			assert.Greater(t, totalChunks, 0, "Should extract chunks from concurrent processing")
		})
	}
}

// Helper functions

// mustNewLanguage creates a new language or panics - for test use only
func mustNewLanguage(name string) valueobject.Language {
	lang, err := valueobject.NewLanguage(name)
	if err != nil {
		panic(fmt.Sprintf("failed to create language %s: %v", name, err))
	}
	return lang
}

// getRealCodeSamples returns real code samples that would typically be found in production
func getRealCodeSamples() map[string]string {
	// For GREEN phase: provide minimal realistic code samples to make tests pass
	return map[string]string{
		"main.go": `package main

import (
	"fmt"
	"log"
)

func main() {
	fmt.Println("Hello, World!")
	processData()
}

func processData() error {
	data := []string{"item1", "item2", "item3"}
	for _, item := range data {
		if err := handleItem(item); err != nil {
			log.Printf("Error handling item %s: %v", item, err)
			return err
		}
	}
	return nil
}

func handleItem(item string) error {
	if item == "" {
		return fmt.Errorf("empty item")
	}
	fmt.Printf("Processing: %s\n", item)
	return nil
}`,

		"services/user_service.go": `package services

import (
	"context"
	"errors"
)

type User struct {
	ID    int    ` + "`json:\"id\"`" + `
	Name  string ` + "`json:\"name\"`" + `
	Email string ` + "`json:\"email\"`" + `
}

type UserService struct {
	repo UserRepository
}

type UserRepository interface {
	GetUser(ctx context.Context, id int) (*User, error)
	CreateUser(ctx context.Context, user *User) error
}

func NewUserService(repo UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetUserByID(ctx context.Context, id int) (*User, error) {
	if id <= 0 {
		return nil, errors.New("invalid user ID")
	}
	return s.repo.GetUser(ctx, id)
}

func (s *UserService) CreateUser(ctx context.Context, name, email string) (*User, error) {
	user := &User{
		Name:  name,
		Email: email,
	}
	if err := s.validateUser(user); err != nil {
		return nil, err
	}
	return user, s.repo.CreateUser(ctx, user)
}

func (s *UserService) validateUser(user *User) error {
	if user.Name == "" {
		return errors.New("name cannot be empty")
	}
	if user.Email == "" {
		return errors.New("email cannot be empty")
	}
	return nil
}`,

		"utils/helper.py": `"""
Utility functions for data processing
"""

import json
from typing import List, Dict, Any, Optional

class DataProcessor:
    """Handles data processing operations"""
    
    def __init__(self, config: Optional[Dict[str, Any]] = None):
        self.config = config or {}
        self.processed_count = 0
    
    def process_items(self, items: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        """Process a list of items and return transformed results"""
        results = []
        for item in items:
            try:
                processed_item = self._process_single_item(item)
                if processed_item:
                    results.append(processed_item)
                    self.processed_count += 1
            except Exception as e:
                print(f"Error processing item {item}: {e}")
        return results
    
    def _process_single_item(self, item: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        """Process a single item with validation"""
        if not self._is_valid_item(item):
            return None
        
        return {
            'id': item.get('id'),
            'name': item.get('name', '').strip(),
            'processed': True,
            'timestamp': self._get_timestamp()
        }
    
    def _is_valid_item(self, item: Dict[str, Any]) -> bool:
        """Validate item structure"""
        return (
            isinstance(item, dict) and
            'id' in item and
            'name' in item and
            item.get('name', '').strip()
        )
    
    def _get_timestamp(self) -> str:
        """Get current timestamp string"""
        import datetime
        return datetime.datetime.now().isoformat()

def load_config(filename: str) -> Dict[str, Any]:
    """Load configuration from JSON file"""
    try:
        with open(filename, 'r') as f:
            return json.load(f)
    except (FileNotFoundError, json.JSONDecodeError) as e:
        print(f"Error loading config from {filename}: {e}")
        return {}

async def async_process_data(data: List[Any]) -> List[Any]:
    """Asynchronously process data items"""
    import asyncio
    
    async def process_item(item):
        # Simulate async processing
        await asyncio.sleep(0.1)
        return {'processed': item, 'status': 'completed'}
    
    tasks = [process_item(item) for item in data]
    return await asyncio.gather(*tasks)`,

		"components/App.js": `import React, { useState, useEffect, useCallback } from 'react';
import './App.css';

const API_BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:3000';

/**
 * Main application component
 * Handles user data display and management
 */
function App() {
    const [users, setUsers] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);

    // Fetch users on component mount
    useEffect(() => {
        fetchUsers();
    }, []);

    const fetchUsers = useCallback(async () => {
        try {
            setLoading(true);
            setError(null);
            
            const response = await fetch(` + "`${API_BASE_URL}/users`" + `);
            if (!response.ok) {
                throw new Error(` + "`Failed to fetch users: ${response.statusText}`" + `);
            }
            
            const userData = await response.json();
            setUsers(userData);
        } catch (err) {
            console.error('Error fetching users:', err);
            setError(err.message);
        } finally {
            setLoading(false);
        }
    }, []);

    const handleUserClick = useCallback((user) => {
        console.log('User clicked:', user);
        // Handle user selection logic here
    }, []);

    const renderUserList = () => {
        if (loading) {
            return <LoadingSpinner />;
        }

        if (error) {
            return <ErrorMessage error={error} onRetry={fetchUsers} />;
        }

        if (users.length === 0) {
            return <EmptyState message="No users found" />;
        }

        return (
            <UserList 
                users={users} 
                onUserClick={handleUserClick}
            />
        );
    };

    return (
        <div className="app">
            <header className="app-header">
                <h1>User Management</h1>
            </header>
            <main className="app-main">
                {renderUserList()}
            </main>
        </div>
    );
}

// Loading spinner component
const LoadingSpinner = () => (
    <div className="loading-spinner">
        <div className="spinner"></div>
        <p>Loading users...</p>
    </div>
);

// Error message component
const ErrorMessage = ({ error, onRetry }) => (
    <div className="error-message">
        <h3>Error</h3>
        <p>{error}</p>
        <button onClick={onRetry} className="retry-button">
            Retry
        </button>
    </div>
);

// Empty state component
const EmptyState = ({ message }) => (
    <div className="empty-state">
        <p>{message}</p>
    </div>
);

// User list component
const UserList = ({ users, onUserClick }) => (
    <div className="user-list">
        {users.map(user => (
            <UserCard 
                key={user.id} 
                user={user} 
                onClick={() => onUserClick(user)}
            />
        ))}
    </div>
);

// User card component
const UserCard = ({ user, onClick }) => (
    <div className="user-card" onClick={onClick}>
        <h4>{user.name}</h4>
        <p>{user.email}</p>
        <small>ID: {user.id}</small>
    </div>
);

export default App;`,

		"config/database.js": `/**
 * Database configuration and connection utilities
 */

const { Pool } = require('pg');
const { MongoClient } = require('mongodb');

class DatabaseConfig {
    constructor(options = {}) {
        this.options = {
            host: options.host || process.env.DB_HOST || 'localhost',
            port: options.port || process.env.DB_PORT || 5432,
            database: options.database || process.env.DB_NAME || 'app_db',
            username: options.username || process.env.DB_USER || 'app_user',
            password: options.password || process.env.DB_PASSWORD || '',
            maxConnections: options.maxConnections || 10,
            idleTimeout: options.idleTimeout || 30000,
            ...options
        };
    }

    createPostgresPool() {
        return new Pool({
            host: this.options.host,
            port: this.options.port,
            database: this.options.database,
            user: this.options.username,
            password: this.options.password,
            max: this.options.maxConnections,
            idleTimeoutMillis: this.options.idleTimeout,
            connectionTimeoutMillis: 2000,
        });
    }

    async createMongoConnection() {
        const uri = ` + "`mongodb://${this.options.username}:${this.options.password}@${this.options.host}:${this.options.port}/${this.options.database}`" + `;
        
        const client = new MongoClient(uri, {
            maxPoolSize: this.options.maxConnections,
            serverSelectionTimeoutMS: 5000,
            socketTimeoutMS: 45000,
        });

        await client.connect();
        return client;
    }
}

// Database connection managers
class ConnectionManager {
    constructor() {
        this.connections = new Map();
        this.config = new DatabaseConfig();
    }

    async getConnection(type = 'postgres') {
        if (this.connections.has(type)) {
            return this.connections.get(type);
        }

        let connection;
        switch (type) {
            case 'postgres':
                connection = this.config.createPostgresPool();
                break;
            case 'mongodb':
                connection = await this.config.createMongoConnection();
                break;
            default:
                throw new Error(` + "`Unsupported database type: ${type}`" + `);
        }

        this.connections.set(type, connection);
        return connection;
    }

    async closeAll() {
        const closePromises = Array.from(this.connections.entries()).map(
            async ([type, connection]) => {
                try {
                    if (type === 'postgres') {
                        await connection.end();
                    } else if (type === 'mongodb') {
                        await connection.close();
                    }
                    console.log(` + "`Closed ${type} connection`" + `);
                } catch (error) {
                    console.error(` + "`Error closing ${type} connection:`, error`" + `);
                }
            }
        );

        await Promise.allSettled(closePromises);
        this.connections.clear();
    }
}

module.exports = {
    DatabaseConfig,
    ConnectionManager,
};`,
	}
}

// findCodeFiles finds all code files in a directory for a specific language
func findCodeFiles(dir string, lang valueobject.Language) ([]string, error) {
	var files []string
	ext := getTestFileExtension(lang)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ext) {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// detectLanguageFromPath detects language from file path
func detectLanguageFromPath(path string) valueobject.Language {
	switch {
	case strings.HasSuffix(path, ".go"):
		return mustNewLanguage(valueobject.LanguageGo)
	case strings.HasSuffix(path, ".py"):
		return mustNewLanguage(valueobject.LanguagePython)
	case strings.HasSuffix(path, ".js"):
		return mustNewLanguage(valueobject.LanguageJavaScript)
	default:
		return mustNewLanguage("unknown")
	}
}

// isCodeFile checks if a file is a code file for the specified language
func isCodeFile(path string, lang valueobject.Language) bool {
	ext := getTestFileExtension(lang)
	return strings.HasSuffix(path, ext)
}

// getTestFileExtension returns the file extension for a language
func getTestFileExtension(lang valueobject.Language) string {
	switch lang.Name() {
	case valueobject.LanguageGo:
		return ".go"
	case valueobject.LanguagePython:
		return ".py"
	case valueobject.LanguageJavaScript:
		return ".js"
	default:
		return ""
	}
}
