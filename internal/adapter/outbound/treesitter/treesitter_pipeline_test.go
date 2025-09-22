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

// MockParser is a minimal parser implementation for testing.
type MockParser struct {
	language valueobject.Language
}

func NewMockParser(lang valueobject.Language) *MockParser {
	return &MockParser{language: lang}
}

func (m *MockParser) Parse(ctx context.Context, source []byte) (*ParseResult, error) {
	// Create a minimal parse tree for testing
	metadata, err := valueobject.NewParseMetadata(0, "mock", "1.0.0")
	if err != nil {
		return nil, err
	}

	rootNode := &valueobject.ParseNode{
		Type:      "source_file",
		StartByte: 0,
		EndByte:   uint32(len(source)),
		StartPos:  valueobject.Position{Row: 0, Column: 0},
		EndPos:    valueobject.Position{Row: 1, Column: 0},
		Children:  []*valueobject.ParseNode{},
	}

	domainTree, err := valueobject.NewParseTree(ctx, m.language, rootNode, source, metadata)
	if err != nil {
		return nil, err
	}

	portTree, err := ConvertDomainParseTreeToPort(domainTree)
	if err != nil {
		return nil, err
	}

	return &ParseResult{
		Success:   true,
		ParseTree: portTree,
		Duration:  0,
	}, nil
}

func (m *MockParser) ParseSource(
	ctx context.Context,
	language valueobject.Language,
	source []byte,
	options ParseOptions,
) (*ParseResult, error) {
	return m.Parse(ctx, source)
}

func (m *MockParser) GetLanguage() string {
	return m.language.Name()
}

func (m *MockParser) Close() error {
	return nil
}

// LanguageParser interface implementations.
func (m *MockParser) ExtractFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return []outbound.SemanticCodeChunk{}, nil
}

func (m *MockParser) ExtractClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return []outbound.SemanticCodeChunk{}, nil
}

func (m *MockParser) ExtractInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return []outbound.SemanticCodeChunk{}, nil
}

func (m *MockParser) ExtractVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return []outbound.SemanticCodeChunk{}, nil
}

func (m *MockParser) ExtractImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.ImportDeclaration, error) {
	return []outbound.ImportDeclaration{}, nil
}

func (m *MockParser) ExtractModules(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options outbound.SemanticExtractionOptions,
) ([]outbound.SemanticCodeChunk, error) {
	return []outbound.SemanticCodeChunk{}, nil
}

func (m *MockParser) GetSupportedLanguage() valueobject.Language {
	return m.language
}

func (m *MockParser) GetSupportedConstructTypes() []outbound.SemanticConstructType {
	return []outbound.SemanticConstructType{
		outbound.ConstructFunction,
		outbound.ConstructClass,
		outbound.ConstructInterface,
		outbound.ConstructVariable,
	}
}

func (m *MockParser) IsSupported(language valueobject.Language) bool {
	return language.Name() == m.language.Name()
}

// setupTestParsers registers the parsers needed for testing to avoid import cycles.
func setupTestParsers() {
	// Register parsers manually to avoid import cycles in tests
	RegisterParser(valueobject.LanguageGo, func() (ObservableTreeSitterParser, error) {
		goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
		if err != nil {
			return nil, err
		}
		return NewMockParser(goLang), nil
	})
	RegisterParser(valueobject.LanguagePython, func() (ObservableTreeSitterParser, error) {
		pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
		if err != nil {
			return nil, err
		}
		return NewMockParser(pythonLang), nil
	})
	RegisterParser(valueobject.LanguageJavaScript, func() (ObservableTreeSitterParser, error) {
		jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
		if err != nil {
			return nil, err
		}
		return NewMockParser(jsLang), nil
	})
}

// TestPipelineMinimalIntegration exercises the basic parser/convert/extract pipeline.
func TestPipelineMinimalIntegration(t *testing.T) {
	// Setup parsers for testing
	setupTestParsers()
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
		goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)
		parser, err := factory.CreateParser(ctx, goLang)
		require.NoError(t, err)

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
		// Create a factory and adapter that share the same registry
		factory, err := NewTreeSitterParserFactory(context.Background())
		require.NoError(t, err)

		// Create adapter with our factory
		adapter := NewSemanticTraverserAdapterWithFactory(factory)

		// Create a domain parse tree
		converter := NewTreeSitterParseTreeConverter()
		ctx := context.Background()
		goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)
		parser, err := factory.CreateParser(ctx, goLang)
		require.NoError(t, err)

		goCode := []byte(`package main

func hello() {
	println("hello")
}`)
		parseResult, _ := parser.Parse(ctx, goCode)
		domainTree, err := converter.ConvertToDomain(parseResult)
		require.NoError(t, err)

		// Test extraction using the adapter directly
		chunks, err := adapter.ExtractFunctions(ctx, domainTree, outbound.SemanticExtractionOptions{})
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
		adapter := NewSemanticTraverserAdapterWithFactory(factory)

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

				// Extract (using functions as a representative test)
				chunks, err := adapter.ExtractFunctions(ctx, domainTree, outbound.SemanticExtractionOptions{})
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
	// Setup parsers for testing
	setupTestParsers()

	factory, err := NewTreeSitterParserFactory(context.Background())
	require.NoError(t, err)
	converter := NewTreeSitterParseTreeConverter()
	adapter := NewSemanticTraverserAdapterWithFactory(factory)

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
			chunks, err := adapter.ExtractFunctions(ctx, domainTree, outbound.SemanticExtractionOptions{})
			require.NoError(t, err)
			require.NotNil(t, chunks, "Chunks should not be nil for %s", sample.filename)

			t.Logf("Successfully processed %s with %d chunks", sample.filename, len(chunks))
		})
	}
}

// TestPipelinePerformance checks basic latency expectations.
func TestPipelinePerformance(t *testing.T) {
	// Setup parsers for testing
	setupTestParsers()

	factory, err := NewTreeSitterParserFactory(context.Background())
	require.NoError(t, err)
	converter := NewTreeSitterParseTreeConverter()
	adapter := NewSemanticTraverserAdapterWithFactory(factory)

	ctx := context.Background()

	goCode := []byte(`package main
import "fmt"
func main() {
	fmt.Println("Performance test")
}`)

	lang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	// Measure parsing performance
	start := time.Now()
	parser, err := factory.CreateParser(ctx, lang)
	require.NoError(t, err)

	parseResult, err := parser.Parse(ctx, goCode)
	require.NoError(t, err)

	domainTree, err := converter.ConvertToDomain(parseResult)
	require.NoError(t, err)

	chunks, err := adapter.ExtractFunctions(ctx, domainTree, outbound.SemanticExtractionOptions{})
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
