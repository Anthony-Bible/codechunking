package outbound

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockSemanticTraverser implements SemanticTraverser for testing integration
type MockSemanticTraverser struct {
	ShouldFailExtractFunctions  bool
	ShouldFailExtractClasses    bool
	ShouldFailExtractModules    bool
	ShouldFailExtractInterfaces bool
	ShouldFailExtractVariables  bool
	ShouldFailExtractComments   bool
	ShouldFailExtractImports    bool

	ExpectedFunctions  []SemanticCodeChunk
	ExpectedClasses    []SemanticCodeChunk
	ExpectedModules    []SemanticCodeChunk
	ExpectedInterfaces []SemanticCodeChunk
	ExpectedVariables  []SemanticCodeChunk
	ExpectedComments   []SemanticCodeChunk
	ExpectedImports    []ImportDeclaration
	ExpectedConstructs []SemanticConstructType
}

func (m *MockSemanticTraverser) ExtractFunctions(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options SemanticExtractionOptions,
) ([]SemanticCodeChunk, error) {
	if m.ShouldFailExtractFunctions {
		return nil, errors.New("mock extract functions failure")
	}
	return m.ExpectedFunctions, nil
}

func (m *MockSemanticTraverser) ExtractClasses(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options SemanticExtractionOptions,
) ([]SemanticCodeChunk, error) {
	if m.ShouldFailExtractClasses {
		return nil, errors.New("mock extract classes failure")
	}
	return m.ExpectedClasses, nil
}

func (m *MockSemanticTraverser) ExtractModules(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options SemanticExtractionOptions,
) ([]SemanticCodeChunk, error) {
	if m.ShouldFailExtractModules {
		return nil, errors.New("mock extract modules failure")
	}
	return m.ExpectedModules, nil
}

func (m *MockSemanticTraverser) ExtractInterfaces(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options SemanticExtractionOptions,
) ([]SemanticCodeChunk, error) {
	if m.ShouldFailExtractInterfaces {
		return nil, errors.New("mock extract interfaces failure")
	}
	return m.ExpectedInterfaces, nil
}

func (m *MockSemanticTraverser) ExtractVariables(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options SemanticExtractionOptions,
) ([]SemanticCodeChunk, error) {
	if m.ShouldFailExtractVariables {
		return nil, errors.New("mock extract variables failure")
	}
	return m.ExpectedVariables, nil
}

func (m *MockSemanticTraverser) ExtractComments(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options SemanticExtractionOptions,
) ([]SemanticCodeChunk, error) {
	if m.ShouldFailExtractComments {
		return nil, errors.New("mock extract comments failure")
	}
	return m.ExpectedComments, nil
}

func (m *MockSemanticTraverser) ExtractImports(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	options SemanticExtractionOptions,
) ([]ImportDeclaration, error) {
	if m.ShouldFailExtractImports {
		return nil, errors.New("mock extract imports failure")
	}
	return m.ExpectedImports, nil
}

func (m *MockSemanticTraverser) GetSupportedConstructTypes(
	ctx context.Context,
	language valueobject.Language,
) ([]SemanticConstructType, error) {
	return m.ExpectedConstructs, nil
}

// Test Integration with Semantic Traverser
func TestChunkingStrategy_SemanticTraverserIntegration(t *testing.T) {
	ctx := context.Background()

	t.Run("integrate_function_extraction_with_chunking", func(t *testing.T) {
		// This test should FAIL because no integration implementation exists
		semanticTraverser := &MockSemanticTraverser{
			ExpectedFunctions: []SemanticCodeChunk{
				createTestSemanticCodeChunk("ProcessData", ConstructFunction),
				createTestSemanticCodeChunk("ValidateInput", ConstructFunction),
				createTestSemanticCodeChunk("HandleError", ConstructFunction),
			},
		}

		chunkingStrategy := &MockCodeChunkingStrategy{
			ExpectedChunks: []EnhancedCodeChunk{
				createTestEnhancedCodeChunk("ProcessData", ChunkFunction, StrategyFunction),
				createTestEnhancedCodeChunk("ValidateInput", ChunkFunction, StrategyFunction),
				createTestEnhancedCodeChunk("HandleError", ChunkFunction, StrategyFunction),
			},
		}

		// Create a mock parse tree
		parseTree := createTestParseTree()

		extractionOptions := SemanticExtractionOptions{
			IncludePrivate:       true,
			IncludeComments:      true,
			IncludeDocumentation: true,
			IncludeTypeInfo:      true,
			IncludeDependencies:  true,
			PreservationStrategy: PreserveModerate,
			ChunkingStrategy:     ChunkByFunction,
		}

		// Extract semantic chunks
		semanticChunks, err := semanticTraverser.ExtractFunctions(ctx, parseTree, extractionOptions)
		require.NoError(t, err)
		assert.Len(t, semanticChunks, 3)

		// Apply chunking strategy
		config := ChunkingConfiguration{
			Strategy:             StrategyFunction,
			ContextPreservation:  PreserveModerate,
			MaxChunkSize:         1500,
			MinChunkSize:         200,
			IncludeImports:       true,
			IncludeDocumentation: true,
			PreserveDependencies: true,
			QualityThreshold:     0.8,
		}

		enhancedChunks, err := chunkingStrategy.ChunkCode(ctx, semanticChunks, config)
		require.NoError(t, err)
		assert.Len(t, enhancedChunks, 3)

		// Verify integration - semantic chunks should be preserved in enhanced chunks
		for i, chunk := range enhancedChunks {
			assert.Equal(t, ChunkFunction, chunk.ChunkType)
			assert.Equal(t, StrategyFunction, chunk.ChunkingStrategy)
			assert.NotEmpty(t, chunk.SemanticConstructs)
			assert.Equal(t, semanticChunks[i].Name, chunk.SemanticConstructs[0].Name)
		}
	})

	t.Run("integrate_class_extraction_with_hybrid_chunking", func(t *testing.T) {
		// This test should FAIL because no integration implementation exists
		semanticTraverser := &MockSemanticTraverser{
			ExpectedClasses: []SemanticCodeChunk{
				createTestSemanticCodeChunk("UserService", ConstructClass),
				createTestSemanticCodeChunk("DatabaseConnection", ConstructClass),
			},
			ExpectedFunctions: []SemanticCodeChunk{
				createTestSemanticCodeChunk("InitializeApp", ConstructFunction),
				createTestSemanticCodeChunk("ShutdownApp", ConstructFunction),
			},
		}

		chunkingStrategy := &MockCodeChunkingStrategy{
			ExpectedChunks: []EnhancedCodeChunk{
				createTestEnhancedCodeChunk("UserService", ChunkClass, StrategyHybrid),
				createTestEnhancedCodeChunk("DatabaseConnection", ChunkClass, StrategyHybrid),
				createTestEnhancedCodeChunk("InitializeApp", ChunkFunction, StrategyHybrid),
				createTestEnhancedCodeChunk("ShutdownApp", ChunkFunction, StrategyHybrid),
			},
		}

		parseTree := createTestParseTree()

		extractionOptions := SemanticExtractionOptions{
			IncludePrivate:       false,
			IncludeComments:      true,
			IncludeDocumentation: true,
			PreservationStrategy: PreserveMaximal,
			ChunkingStrategy:     ChunkByClass,
		}

		// Extract both classes and functions
		classChunks, err := semanticTraverser.ExtractClasses(ctx, parseTree, extractionOptions)
		require.NoError(t, err)

		functionChunks, err := semanticTraverser.ExtractFunctions(ctx, parseTree, extractionOptions)
		require.NoError(t, err)

		// Combine semantic chunks
		allSemanticChunks := append(classChunks, functionChunks...)

		config := ChunkingConfiguration{
			Strategy:             StrategyHybrid,
			ContextPreservation:  PreserveMaximal,
			MaxChunkSize:         3000,
			MinChunkSize:         100,
			PreferredBoundaries:  []BoundaryType{BoundaryClass, BoundaryFunction},
			IncludeImports:       true,
			IncludeDocumentation: true,
			PreserveDependencies: true,
			EnableSplitting:      false,
			QualityThreshold:     0.85,
		}

		enhancedChunks, err := chunkingStrategy.ChunkCode(ctx, allSemanticChunks, config)
		require.NoError(t, err)
		assert.Len(t, enhancedChunks, 4)

		// Verify hybrid strategy produces different chunk types
		chunkTypes := make(map[ChunkType]int)
		for _, chunk := range enhancedChunks {
			chunkTypes[chunk.ChunkType]++
			assert.Equal(t, StrategyHybrid, chunk.ChunkingStrategy)
			assert.Equal(t, PreserveMaximal, chunk.ContextStrategy)
		}

		assert.Equal(t, 2, chunkTypes[ChunkClass], "Should have 2 class chunks")
		assert.Equal(t, 2, chunkTypes[ChunkFunction], "Should have 2 function chunks")
	})

	t.Run("integrate_cross_language_extraction_with_adaptive_chunking", func(t *testing.T) {
		// This test should FAIL because no integration implementation exists

		// Go semantic chunks
		goTraverser := &MockSemanticTraverser{
			ExpectedFunctions: []SemanticCodeChunk{
				createTestSemanticCodeChunkWithLanguage("ProcessRequest", ConstructFunction, valueobject.LanguageGo),
				createTestSemanticCodeChunkWithLanguage("ValidateData", ConstructFunction, valueobject.LanguageGo),
			},
			ExpectedInterfaces: []SemanticCodeChunk{
				createTestSemanticCodeChunkWithLanguage("Handler", ConstructInterface, valueobject.LanguageGo),
			},
		}

		// Python semantic chunks
		pythonTraverser := &MockSemanticTraverser{
			ExpectedClasses: []SemanticCodeChunk{
				createTestSemanticCodeChunkWithLanguage("DataProcessor", ConstructClass, valueobject.LanguagePython),
			},
			ExpectedFunctions: []SemanticCodeChunk{
				createTestSemanticCodeChunkWithLanguage("process_data", ConstructFunction, valueobject.LanguagePython),
			},
		}

		// JavaScript semantic chunks
		jsTraverser := &MockSemanticTraverser{
			ExpectedFunctions: []SemanticCodeChunk{
				createTestSemanticCodeChunkWithLanguage(
					"processRequest",
					ConstructFunction,
					valueobject.LanguageJavaScript,
				),
				createTestSemanticCodeChunkWithLanguage(
					"validateInput",
					ConstructFunction,
					valueobject.LanguageJavaScript,
				),
			},
		}

		adaptiveStrategy := &MockCodeChunkingStrategy{
			ExpectedChunks: []EnhancedCodeChunk{
				createTestEnhancedCodeChunkWithLanguage("ProcessRequest", ChunkFunction, valueobject.LanguageGo),
				createTestEnhancedCodeChunkWithLanguage("Handler", ChunkInterface, valueobject.LanguageGo),
				createTestEnhancedCodeChunkWithLanguage("DataProcessor", ChunkClass, valueobject.LanguagePython),
				createTestEnhancedCodeChunkWithLanguage(
					"processRequest",
					ChunkFunction,
					valueobject.LanguageJavaScript,
				),
			},
		}

		parseTree := createTestParseTree()

		// Extract from different languages
		goFunctions, err := goTraverser.ExtractFunctions(ctx, parseTree, SemanticExtractionOptions{})
		require.NoError(t, err)

		goInterfaces, err := goTraverser.ExtractInterfaces(ctx, parseTree, SemanticExtractionOptions{})
		require.NoError(t, err)

		pythonClasses, err := pythonTraverser.ExtractClasses(ctx, parseTree, SemanticExtractionOptions{})
		require.NoError(t, err)

		jsFunctions, err := jsTraverser.ExtractFunctions(ctx, parseTree, SemanticExtractionOptions{})
		require.NoError(t, err)

		// Combine all semantic chunks
		allSemanticChunks := append(goFunctions, goInterfaces...)
		allSemanticChunks = append(allSemanticChunks, pythonClasses...)
		allSemanticChunks = append(allSemanticChunks, jsFunctions...)

		config := ChunkingConfiguration{
			Strategy:             StrategyAdaptive,
			ContextPreservation:  PreserveModerate,
			MaxChunkSize:         2000,
			MinChunkSize:         150,
			PreferredBoundaries:  []BoundaryType{BoundaryFunction, BoundaryClass, BoundaryInterface},
			IncludeImports:       true,
			IncludeDocumentation: true,
			PreserveDependencies: true,
			EnableSplitting:      true,
			QualityThreshold:     0.75,
			LanguageSpecificOptions: map[string]interface{}{
				"go_prefer_interface_grouping":  true,
				"python_prefer_class_methods":   true,
				"js_prefer_function_clustering": true,
			},
		}

		enhancedChunks, err := adaptiveStrategy.ChunkCode(ctx, allSemanticChunks, config)
		require.NoError(t, err)
		assert.Len(t, enhancedChunks, 4)

		// Verify adaptive strategy handles different languages appropriately
		languageChunks := make(map[string][]EnhancedCodeChunk)
		for _, chunk := range enhancedChunks {
			langName := chunk.Language.Name()
			languageChunks[langName] = append(languageChunks[langName], chunk)
			assert.Equal(t, StrategyAdaptive, chunk.ChunkingStrategy)
		}

		assert.Len(t, languageChunks, 3, "Should have chunks from 3 different languages")
		assert.Contains(t, languageChunks, valueobject.LanguageGo)
		assert.Contains(t, languageChunks, valueobject.LanguagePython)
		assert.Contains(t, languageChunks, valueobject.LanguageJavaScript)
	})

	t.Run("integration_with_import_and_dependency_resolution", func(t *testing.T) {
		// This test should FAIL because no integration implementation exists
		semanticTraverser := &MockSemanticTraverser{
			ExpectedFunctions: []SemanticCodeChunk{
				{
					ID:       "func1",
					Name:     "ProcessData",
					Type:     ConstructFunction,
					Language: createTestLanguage(),
					Content:  "func ProcessData(ctx context.Context) error { return helper.Process(ctx) }",
					Dependencies: []DependencyReference{
						{Name: "helper", Type: "package", Path: "internal/helper"},
						{Name: "context", Type: "standard", Path: "context"},
					},
					CalledFunctions: []FunctionCall{
						{Name: "helper.Process"},
					},
				},
			},
			ExpectedImports: []ImportDeclaration{
				{
					Path:    "context",
					Content: "import \"context\"",
				},
				{
					Path:    "internal/helper",
					Content: "import \"internal/helper\"",
				},
				{
					Path:    "fmt",
					Content: "import \"fmt\"",
				},
			},
		}

		chunkingStrategy := &MockCodeChunkingStrategy{
			ExpectedChunks: []EnhancedCodeChunk{
				{
					ID:               "chunk1",
					ChunkType:        ChunkFunction,
					ChunkingStrategy: StrategyFunction,
					ContextStrategy:  PreserveModerate,
					Dependencies: []ChunkDependency{
						{
							Type:         DependencyImport,
							TargetSymbol: "context",
							Relationship: "imports",
							Strength:     1.0,
							IsResolved:   true,
						},
						{
							Type:         DependencyCall,
							TargetSymbol: "helper.Process",
							Relationship: "calls",
							Strength:     0.9,
							IsResolved:   true,
						},
					},
					PreservedContext: PreservedContext{
						ImportStatements: []ImportDeclaration{
							{Path: "context", Content: "import \"context\""},
							{Path: "internal/helper", Content: "import \"internal/helper\""},
						},
						CalledFunctions: []FunctionCall{
							{Name: "helper.Process"},
						},
					},
				},
			},
		}

		parseTree := createTestParseTree()

		extractionOptions := SemanticExtractionOptions{
			IncludeDependencies:  true,
			PreservationStrategy: PreserveModerate,
		}

		// Extract imports first
		imports, err := semanticTraverser.ExtractImports(ctx, parseTree, extractionOptions)
		require.NoError(t, err)
		assert.Len(t, imports, 3)

		// Extract functions
		functions, err := semanticTraverser.ExtractFunctions(ctx, parseTree, extractionOptions)
		require.NoError(t, err)
		assert.Len(t, functions, 1)

		config := ChunkingConfiguration{
			Strategy:             StrategyFunction,
			ContextPreservation:  PreserveModerate,
			MaxChunkSize:         1000,
			IncludeImports:       true,
			PreserveDependencies: true,
			QualityThreshold:     0.8,
		}

		enhancedChunks, err := chunkingStrategy.ChunkCode(ctx, functions, config)
		require.NoError(t, err)
		assert.Len(t, enhancedChunks, 1)

		chunk := enhancedChunks[0]

		// Verify dependency resolution integration
		assert.NotEmpty(t, chunk.Dependencies)
		assert.NotEmpty(t, chunk.PreservedContext.ImportStatements)
		assert.NotEmpty(t, chunk.PreservedContext.CalledFunctions)

		// Check that dependencies are resolved
		resolvedDeps := 0
		for _, dep := range chunk.Dependencies {
			if dep.IsResolved {
				resolvedDeps++
			}
		}
		assert.Greater(t, resolvedDeps, 0, "Some dependencies should be resolved")

		// Verify imports are preserved
		importPaths := make(map[string]bool)
		for _, imp := range chunk.PreservedContext.ImportStatements {
			importPaths[imp.Path] = true
		}
		assert.True(t, importPaths["context"])
		assert.True(t, importPaths["internal/helper"])
	})
}

// Test Performance Scenarios
func TestChunkingStrategy_PerformanceScenarios(t *testing.T) {
	ctx := context.Background()

	t.Run("large_file_chunking_performance", func(t *testing.T) {
		// This test should FAIL because no implementation exists

		// Create many semantic chunks to simulate a large file
		var largeSemanticChunks []SemanticCodeChunk
		for i := 0; i < 100; i++ {
			chunk := createTestSemanticCodeChunk("Function"+string(rune(i)), ConstructFunction)
			chunk.Content = generateLargeCodeContent(500) // Each function is 500 bytes
			largeSemanticChunks = append(largeSemanticChunks, chunk)
		}

		var expectedChunks []EnhancedCodeChunk
		for i := 0; i < 50; i++ { // Expect compression due to size-based chunking
			chunk := createTestEnhancedCodeChunk("Fragment"+string(rune(i)), ChunkFragment, StrategySizeBased)
			chunk.Size.Bytes = 1000 // Each enhanced chunk is ~1000 bytes
			expectedChunks = append(expectedChunks, chunk)
		}

		chunkingStrategy := &MockCodeChunkingStrategy{
			ExpectedChunks: expectedChunks,
		}

		config := ChunkingConfiguration{
			Strategy:         StrategySizeBased,
			MaxChunkSize:     1000,
			MinChunkSize:     500,
			EnableSplitting:  true,
			QualityThreshold: 0.7,
			PerformanceHints: &ChunkingPerformanceHints{
				MaxConcurrency:     4,
				EnableCaching:      true,
				StreamProcessing:   true,
				MemoryOptimization: true,
				LazyEvaluation:     true,
			},
		}

		startTime := time.Now()

		enhancedChunks, err := chunkingStrategy.ChunkCode(ctx, largeSemanticChunks, config)

		duration := time.Since(startTime)

		require.NoError(t, err)
		assert.Len(t, enhancedChunks, 50)

		// Performance assertions - should complete within reasonable time
		assert.Less(t, duration, 5*time.Second, "Large file chunking should complete within 5 seconds")

		// Verify chunking efficiency
		totalInputSize := len(largeSemanticChunks) * 500
		totalOutputSize := 0
		for _, chunk := range enhancedChunks {
			totalOutputSize += chunk.Size.Bytes
		}

		// Output should be roughly the same size (some compression expected)
		compressionRatio := float64(totalOutputSize) / float64(totalInputSize)
		assert.Greater(t, compressionRatio, 0.8, "Compression ratio should be reasonable")
		assert.Less(t, compressionRatio, 1.2, "Should not expand significantly")
	})

	t.Run("high_complexity_chunking_performance", func(t *testing.T) {
		// This test should FAIL because no implementation exists

		// Create semantic chunks with high complexity (nested structures)
		var complexSemanticChunks []SemanticCodeChunk

		// Main class with many methods
		mainClass := createTestSemanticCodeChunk("ComplexService", ConstructClass)
		mainClass.ChildChunks = make([]SemanticCodeChunk, 20) // 20 methods
		for i := 0; i < 20; i++ {
			method := createTestSemanticCodeChunk("Method"+string(rune(i)), ConstructMethod)
			method.ParentChunk = &mainClass
			mainClass.ChildChunks[i] = method
		}
		complexSemanticChunks = append(complexSemanticChunks, mainClass)

		// Add interfaces with many method signatures
		for i := 0; i < 5; i++ {
			iface := createTestSemanticCodeChunk("Interface"+string(rune(i)), ConstructInterface)
			iface.ChildChunks = make([]SemanticCodeChunk, 10) // 10 method signatures each
			complexSemanticChunks = append(complexSemanticChunks, iface)
		}

		expectedChunks := []EnhancedCodeChunk{
			createTestEnhancedCodeChunk("ComplexService", ChunkClass, StrategyHierarchy),
			createTestEnhancedCodeChunk("ServiceMethods", ChunkMixed, StrategyHierarchy),
			createTestEnhancedCodeChunk("Interfaces", ChunkMixed, StrategyHierarchy),
		}

		// Set high complexity scores
		for i := range expectedChunks {
			expectedChunks[i].QualityMetrics.ComplexityScore = 0.8
			expectedChunks[i].QualityMetrics.CouplingScore = 0.6
			expectedChunks[i].QualityMetrics.CohesionScore = 0.7
		}

		chunkingStrategy := &MockCodeChunkingStrategy{
			ExpectedChunks: expectedChunks,
		}

		config := ChunkingConfiguration{
			Strategy:            StrategyHierarchy,
			ContextPreservation: PreserveMaximal,
			MaxChunkSize:        2000,
			MinChunkSize:        200,
			EnableSplitting:     true,
			QualityThreshold:    0.6, // Lower threshold for high complexity
			PerformanceHints: &ChunkingPerformanceHints{
				MaxConcurrency:     2, // Lower concurrency for complex processing
				EnableCaching:      true,
				MemoryOptimization: true,
			},
		}

		startTime := time.Now()

		enhancedChunks, err := chunkingStrategy.ChunkCode(ctx, complexSemanticChunks, config)

		duration := time.Since(startTime)

		require.NoError(t, err)
		assert.Len(t, enhancedChunks, 3)

		// Performance should still be reasonable even with high complexity
		assert.Less(t, duration, 3*time.Second, "Complex chunking should complete within 3 seconds")

		// Verify complexity handling
		for _, chunk := range enhancedChunks {
			assert.Equal(t, StrategyHierarchy, chunk.ChunkingStrategy)
			assert.Greater(t, chunk.QualityMetrics.ComplexityScore, 0.5)
			assert.NotEmpty(t, chunk.SemanticConstructs, "Should preserve semantic constructs")
		}
	})

	t.Run("memory_optimization_with_streaming", func(t *testing.T) {
		// This test should FAIL because no implementation exists

		// Simulate very large input that would exceed memory if not streamed
		var streamingSemanticChunks []SemanticCodeChunk
		for i := 0; i < 1000; i++ {
			chunk := createTestSemanticCodeChunk("StreamFunction"+string(rune(i)), ConstructFunction)
			chunk.Content = generateLargeCodeContent(1000) // 1KB each = 1MB total
			streamingSemanticChunks = append(streamingSemanticChunks, chunk)
		}

		// Expected output should be processed in smaller batches
		var expectedStreamChunks []EnhancedCodeChunk
		for i := 0; i < 100; i++ { // 10:1 compression ratio
			chunk := createTestEnhancedCodeChunk("StreamChunk"+string(rune(i)), ChunkFragment, StrategySizeBased)
			chunk.Size.Bytes = 10000 // 10KB chunks
			expectedStreamChunks = append(expectedStreamChunks, chunk)
		}

		chunkingStrategy := &MockCodeChunkingStrategy{
			ExpectedChunks: expectedStreamChunks,
		}

		config := ChunkingConfiguration{
			Strategy:         StrategySizeBased,
			MaxChunkSize:     10000,
			MinChunkSize:     5000,
			EnableSplitting:  true,
			QualityThreshold: 0.6,
			PerformanceHints: &ChunkingPerformanceHints{
				StreamProcessing:   true,
				MemoryOptimization: true,
				LazyEvaluation:     true,
				EnableCaching:      false, // Disable caching for streaming
			},
		}

		startTime := time.Now()

		enhancedChunks, err := chunkingStrategy.ChunkCode(ctx, streamingSemanticChunks, config)

		duration := time.Since(startTime)

		require.NoError(t, err)
		assert.Len(t, enhancedChunks, 100)

		// Should complete efficiently despite large input
		assert.Less(t, duration, 10*time.Second, "Streaming chunking should complete within 10 seconds")

		// Verify streaming optimization worked
		totalOutputSize := 0
		for _, chunk := range enhancedChunks {
			totalOutputSize += chunk.Size.Bytes
			assert.LessOrEqual(t, chunk.Size.Bytes, 10000, "No chunk should exceed max size")
			assert.GreaterOrEqual(t, chunk.Size.Bytes, 5000, "No chunk should be below min size")
		}

		expectedTotalSize := 1000000 // 1MB input
		assert.InDelta(t, expectedTotalSize, totalOutputSize, 200000, "Output size should be within 20% of input")
	})

	t.Run("concurrent_chunking_stress_test", func(t *testing.T) {
		// This test should FAIL because no implementation exists

		// Create multiple chunking requests that could run concurrently
		numGoroutines := 10
		chunksPerGoroutine := 50

		results := make(chan []EnhancedCodeChunk, numGoroutines)
		errors := make(chan error, numGoroutines)

		chunkingStrategy := &MockCodeChunkingStrategy{
			ExpectedChunks: []EnhancedCodeChunk{
				createTestEnhancedCodeChunk("ConcurrentChunk", ChunkFunction, StrategyFunction),
			},
		}

		config := ChunkingConfiguration{
			Strategy:         StrategyFunction,
			MaxChunkSize:     1000,
			MinChunkSize:     100,
			QualityThreshold: 0.7,
			PerformanceHints: &ChunkingPerformanceHints{
				MaxConcurrency:   4,
				EnableCaching:    true,
				StreamProcessing: false,
			},
		}

		startTime := time.Now()

		// Launch concurrent chunking operations
		for i := 0; i < numGoroutines; i++ {
			go func(routineID int) {
				var semanticChunks []SemanticCodeChunk
				for j := 0; j < chunksPerGoroutine; j++ {
					chunk := createTestSemanticCodeChunk(
						"Func"+string(rune(routineID))+string(rune(j)),
						ConstructFunction,
					)
					semanticChunks = append(semanticChunks, chunk)
				}

				result, err := chunkingStrategy.ChunkCode(ctx, semanticChunks, config)
				if err != nil {
					errors <- err
					return
				}
				results <- result
			}(i)
		}

		// Collect all results
		var allResults [][]EnhancedCodeChunk
		for i := 0; i < numGoroutines; i++ {
			select {
			case result := <-results:
				allResults = append(allResults, result)
			case err := <-errors:
				t.Fatalf("Concurrent chunking failed: %v", err)
			case <-time.After(30 * time.Second):
				t.Fatal("Concurrent chunking timed out")
			}
		}

		duration := time.Since(startTime)

		// Verify all operations completed successfully
		assert.Len(t, allResults, numGoroutines)

		// Performance should scale reasonably with concurrency
		assert.Less(t, duration, 15*time.Second, "Concurrent chunking should complete within 15 seconds")

		// Verify results consistency
		totalChunks := 0
		for _, result := range allResults {
			assert.NotEmpty(t, result, "Each goroutine should produce chunks")
			totalChunks += len(result)
		}

		expectedTotalChunks := numGoroutines * 1 // 1 chunk per request (based on mock)
		assert.Equal(t, expectedTotalChunks, totalChunks, "Should produce expected total number of chunks")
	})
}

// Helper functions for integration tests
func createTestParseTree() *valueobject.ParseTree {
	// Create a simple mock parse tree for testing
	language := createTestLanguage()

	rootNode := &valueobject.ParseNode{
		Type:      "source_file",
		StartByte: 0,
		EndByte:   1000,
		StartPos:  valueobject.Position{Row: 1, Column: 1},
		EndPos:    valueobject.Position{Row: 50, Column: 1},
		Children:  []*valueobject.ParseNode{},
	}

	source := []byte("package main\n\nfunc main() {\n    // test content\n}")

	metadata, _ := valueobject.NewParseMetadata(
		100*time.Millisecond,
		"0.20.0",
		"go-v1.0.0",
	)
	metadata.NodeCount = 10
	metadata.MaxDepth = 3

	parseTree, _ := valueobject.NewParseTree(
		context.Background(),
		language,
		rootNode,
		source,
		metadata,
	)

	return parseTree
}

func createTestSemanticCodeChunkWithLanguage(
	name string,
	constructType SemanticConstructType,
	languageName string,
) SemanticCodeChunk {
	chunk := createTestSemanticCodeChunk(name, constructType)

	language, _ := valueobject.NewLanguage(languageName)
	chunk.Language = language

	// Customize based on language
	switch languageName {
	case valueobject.LanguageGo:
		chunk.Signature = "func " + name + "()"
		chunk.ReturnType = "error"
	case valueobject.LanguagePython:
		chunk.Signature = "def " + name + "(self)"
		chunk.ReturnType = "None"
	case valueobject.LanguageJavaScript:
		chunk.Signature = "function " + name + "()"
		chunk.ReturnType = "undefined"
	}

	return chunk
}
