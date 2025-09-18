package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPipelineMinimalIntegration exercises the basic parser/convert/extract pipeline.
func TestPipelineMinimalIntegration(t *testing.T) {
	t.Run("TreeSitterParserFactory", func(t *testing.T) {
		factory, err := NewTreeSitterParserFactory(context.Background())
		require.NoError(t, err)
		require.NotNil(t, factory)

		// Test factory can create supported languages
		ctx := context.Background()

		// Test Go parser creation
		goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, goLang)
		require.NoError(t, err)
		require.NotNil(t, parser)

		// Test parsing basic Go code
		goCode := []byte(`package main

func main() {
	println("Hello, World!")
}`)

		parseResult, err := parser.Parse(ctx, goCode)
		require.NoError(t, err)
		require.NotNil(t, parseResult)
		require.True(t, parseResult.Success)
		require.NotNil(t, parseResult.ParseTree)
	})

	t.Run("TreeSitterParseTreeConverter", func(t *testing.T) {
		converter := NewTreeSitterParseTreeConverter()
		require.NotNil(t, converter)

		// Create a simple parse result to convert
		factory, err := NewTreeSitterParserFactory(context.Background())
		require.NoError(t, err)
		ctx := context.Background()
		goLang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
		parser, _ := factory.CreateParser(ctx, goLang)

		goCode := []byte(`package main`)
		parseResult, err := parser.Parse(ctx, goCode)
		require.NoError(t, err)

		// Test conversion
		domainTree, err := converter.ConvertToDomain(parseResult)
		require.NoError(t, err)
		require.NotNil(t, domainTree)

		// Verify the domain tree has basic properties
		assert.Equal(t, goLang.Name(), domainTree.Language().Name())
		assert.NotNil(t, domainTree.RootNode())
	})

	t.Run("SemanticCodeChunkExtractor", func(t *testing.T) {
		extractor := NewSemanticCodeChunkExtractor()
		require.NotNil(t, extractor)

		// Create a domain parse tree
		converter := NewTreeSitterParseTreeConverter()
		factory, err := NewTreeSitterParserFactory(context.Background())
		require.NoError(t, err)
		ctx := context.Background()
		goLang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
		parser, _ := factory.CreateParser(ctx, goLang)

		goCode := []byte(`package main

func hello() {
	println("hello")
}`)
		parseResult, _ := parser.Parse(ctx, goCode)
		domainTree, err := converter.ConvertToDomain(parseResult)
		require.NoError(t, err)

		// Test extraction
		chunks, err := extractor.Extract(ctx, domainTree)
		require.NoError(t, err)
		require.NotNil(t, chunks)

		// Baseline guarantee: extraction returns a slice without erroring.
		assert.IsType(t, []outbound.SemanticCodeChunk{}, chunks)
	})

	t.Run("EndToEndFileProcessing", func(t *testing.T) {
		// Test the full pipeline similar to real integration tests
		factory, err := NewTreeSitterParserFactory(context.Background())
		require.NoError(t, err)
		converter := NewTreeSitterParseTreeConverter()
		extractor := NewSemanticCodeChunkExtractor()

		ctx := context.Background()

		// Test with different languages
		testCases := []struct {
			name     string
			language string
			code     []byte
		}{
			{
				name:     "Go",
				language: valueobject.LanguageGo,
				code: []byte(`package main
import "fmt"
func main() {
	fmt.Println("Hello")
}`),
			},
			{
				name:     "JavaScript",
				language: valueobject.LanguageJavaScript,
				code: []byte(`function greet(name) {
	console.log("Hello " + name);
}
greet("World");`),
			},
			{
				name:     "Python",
				language: valueobject.LanguagePython,
				code: []byte(`def greet(name):
    print(f"Hello {name}")

greet("World")`),
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				lang, err := valueobject.NewLanguage(tc.language)
				require.NoError(t, err)

				// Parse
				parser, err := factory.CreateParser(ctx, lang)
				require.NoError(t, err)

				parseResult, err := parser.Parse(ctx, tc.code)
				require.NoError(t, err)
				require.True(t, parseResult.Success, "Parse should succeed for %s", tc.name)

				// Convert
				domainTree, err := converter.ConvertToDomain(parseResult)
				require.NoError(t, err)
				require.NotNil(t, domainTree, "Domain tree should not be nil for %s", tc.name)

				// Extract
				chunks, err := extractor.Extract(ctx, domainTree)
				require.NoError(t, err)
				require.NotNil(t, chunks, "Chunks should not be nil for %s", tc.name)

				// We only assert the absence of errors for the smoke test suite.
				t.Logf("Successfully processed %s code with %d semantic chunks", tc.name, len(chunks))
			})
		}
	})
}

// TestPipelineRealCodeSamples runs the pipeline against bundled sample snippets.
func TestPipelineRealCodeSamples(t *testing.T) {
	factory, err := NewTreeSitterParserFactory(context.Background())
	require.NoError(t, err)
	converter := NewTreeSitterParseTreeConverter()
	extractor := NewSemanticCodeChunkExtractor()

	ctx := context.Background()
	realCodeSamples := getRealCodeSamples()

	// Test a few sample files to ensure the pipeline works
	sampleFiles := []struct {
		filename string
		language string
	}{
		{"main.go", valueobject.LanguageGo},
		{"utils/helper.py", valueobject.LanguagePython},
		{"components/App.js", valueobject.LanguageJavaScript},
	}

	for _, sample := range sampleFiles {
		t.Run(sample.filename, func(t *testing.T) {
			content, exists := realCodeSamples[sample.filename]
			require.True(t, exists, "Sample file %s should exist", sample.filename)

			lang, err := valueobject.NewLanguage(sample.language)
			require.NoError(t, err)

			// Parse
			parser, err := factory.CreateParser(ctx, lang)
			require.NoError(t, err)

			parseResult, err := parser.Parse(ctx, []byte(content))
			require.NoError(t, err)
			assert.True(t, parseResult.Success, "Parse should succeed for %s", sample.filename)

			// Convert
			domainTree, err := converter.ConvertToDomain(parseResult)
			require.NoError(t, err)
			require.NotNil(t, domainTree, "Domain tree should not be nil for %s", sample.filename)

			// Extract
			chunks, err := extractor.Extract(ctx, domainTree)
			require.NoError(t, err)
			require.NotNil(t, chunks, "Chunks should not be nil for %s", sample.filename)

			t.Logf("Successfully processed %s with %d chunks", sample.filename, len(chunks))
		})
	}
}

// TestPipelinePerformance checks basic latency expectations.
func TestPipelinePerformance(t *testing.T) {
	factory, err := NewTreeSitterParserFactory(context.Background())
	require.NoError(t, err)
	converter := NewTreeSitterParseTreeConverter()
	extractor := NewSemanticCodeChunkExtractor()

	ctx := context.Background()

	goCode := []byte(`package main
import "fmt"
func main() {
	fmt.Println("Performance test")
}`)

	lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)

	// Measure parsing performance
	start := time.Now()
	parser, err := factory.CreateParser(ctx, lang)
	require.NoError(t, err)

	parseResult, err := parser.Parse(ctx, goCode)
	require.NoError(t, err)

	domainTree, err := converter.ConvertToDomain(parseResult)
	require.NoError(t, err)

	chunks, err := extractor.Extract(ctx, domainTree)
	require.NoError(t, err)

	elapsed := time.Since(start)

	// Guardrail: ensure the pipeline completes within an upper bound.
	assert.Less(t, elapsed, 5*time.Second, "Processing should complete within 5 seconds")
	assert.NotNil(t, chunks)

	t.Logf("Treesitter pipeline completed in %v", elapsed)
}

// getRealCodeSamples provides minimal inline samples for pipeline validation.
func getRealCodeSamples() map[string]string {
	return map[string]string{
		"main.go": `package main
import "fmt"
func main() { fmt.Println("Hello") }`,
		"utils/helper.py": `def add(a, b):
    return a + b`,
		"components/App.js": `function App(){ return null } export default App;`,
	}
}
