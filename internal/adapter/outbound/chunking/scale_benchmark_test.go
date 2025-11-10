package chunking

import (
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"strings"
	"testing"
)

// BenchmarkSmallFileChunking benchmarks chunking performance for small files (< 1KB).
func BenchmarkSmallFileChunking(b *testing.B) {
	ctx := context.Background()

	// Create small semantic chunks (< 1KB total)
	smallChunks := generateSmallFileChunks()

	benchmarks := []struct {
		name        string
		overlapSize int
		strategy    outbound.ChunkingStrategyType
	}{
		{"Function_NoOverlap", 0, outbound.StrategyFunction},
		{"Function_SmallOverlap", 100, outbound.StrategyFunction},
		{"Class_NoOverlap", 0, outbound.StrategyClass},
		{"Class_SmallOverlap", 200, outbound.StrategyClass},
		{"SizeBased_NoSplit", 0, outbound.StrategySizeBased},
		{"SizeBased_WithSplit", 200, outbound.StrategySizeBased},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:        bm.strategy,
					OverlapSize:     bm.overlapSize,
					MaxChunkSize:    2000,
					EnableSplitting: bm.strategy == outbound.StrategySizeBased,
				}

				err := runChunkingStrategy(ctx, smallChunks, config, bm.strategy)
				if err != nil {
					b.Fatalf("Small file chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkMediumFileChunking benchmarks chunking performance for medium files (1-10 KB).
func BenchmarkMediumFileChunking(b *testing.B) {
	ctx := context.Background()

	// Create medium-size semantic chunks (1-10 KB total)
	mediumChunks := generateMediumFileChunks()

	benchmarks := []struct {
		name        string
		fileSize    int
		overlapSize int
		strategy    outbound.ChunkingStrategyType
	}{
		{"Small_1KB_Function", 1024, 0, outbound.StrategyFunction},
		{"Small_1KB_Class", 1024, 100, outbound.StrategyClass},
		{"Medium_5KB_Function", 5120, 200, outbound.StrategyFunction},
		{"Medium_5KB_Class", 5120, 300, outbound.StrategyClass},
		{"Large_10KB_Function", 10240, 400, outbound.StrategyFunction},
		{"Large_10KB_Class", 10240, 500, outbound.StrategyClass},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			chunks := filterChunksBySize(mediumChunks, bm.fileSize)

			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:        bm.strategy,
					OverlapSize:     bm.overlapSize,
					MaxChunkSize:    bm.fileSize / 2,
					EnableSplitting: true,
				}

				err := runChunkingStrategy(ctx, chunks, config, bm.strategy)
				if err != nil {
					b.Fatalf("Medium file chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkLargeFileChunking benchmarks chunking performance for large files (> 10 KB).
func BenchmarkLargeFileChunking(b *testing.B) {
	ctx := context.Background()

	// Create large semantic chunks (> 10 KB total)
	largeChunks := generateLargeFileChunks()

	benchmarks := []struct {
		name        string
		fileSize    int
		overlapSize int
		strategy    outbound.ChunkingStrategyType
		splitting   bool
	}{
		{"Large_15KB_Function", 15360, 200, outbound.StrategyFunction, false},
		{"Large_15KB_Class", 15360, 300, outbound.StrategyClass, true},
		{"VeryLarge_25KB_Function", 25600, 400, outbound.StrategyFunction, true},
		{"VeryLarge_25KB_Class", 25600, 600, outbound.StrategyClass, true},
		{"Massive_50KB_Function", 51200, 800, outbound.StrategyFunction, true},
		{"Massive_50KB_Class", 51200, 1000, outbound.StrategyClass, true},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			chunks := filterChunksBySize(largeChunks, bm.fileSize)

			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:        bm.strategy,
					OverlapSize:     bm.overlapSize,
					MaxChunkSize:    bm.fileSize / 3,
					EnableSplitting: bm.splitting,
				}

				err := runChunkingStrategy(ctx, chunks, config, bm.strategy)
				if err != nil {
					b.Fatalf("Large file chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkManySmallFunctions benchmarks performance with many small functions.
func BenchmarkManySmallFunctions(b *testing.B) {
	ctx := context.Background()

	// Create many small functions (typical of utility classes)
	smallFunctionCounts := []int{10, 25, 50, 100, 200, 500}

	for _, count := range smallFunctionCounts {
		b.Run(fmt.Sprintf("SmallFunctions_%d", count), func(b *testing.B) {
			functions := generateManySmallFunctions(count)

			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:             outbound.StrategyFunction,
					OverlapSize:          200,
					IncludeDocumentation: true,
					PreserveDependencies: true,
				}

				err := runFunctionChunking(ctx, functions, config)
				if err != nil {
					b.Fatalf("Many small functions chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkFewLargeFunctions benchmarks performance with few large functions.
func BenchmarkFewLargeFunctions(b *testing.B) {
	ctx := context.Background()

	// Create few large functions (typical of processing modules)
	sizes := []int{5000, 10000, 20000, 30000} // 5KB to 30KB

	for _, size := range sizes {
		b.Run(fmt.Sprintf("LargeFunctions_%dKB", size/1024), func(b *testing.B) {
			functions := generateFewLargeFunctions(3, size)

			benchmarks := []struct {
				name        string
				overlapSize int
				splitting   bool
			}{
				{"NoOverlap", 0, false},
				{"SmallOverlap", 200, false},
				{"WithSplitting", 400, true},
				{"LargeOverlap", 800, true},
			}

			for _, bm := range benchmarks {
				b.Run(bm.name, func(b *testing.B) {
					b.ResetTimer()
					b.ReportAllocs()
					for range b.N {
						config := outbound.ChunkingConfiguration{
							Strategy:        outbound.StrategyFunction,
							OverlapSize:     bm.overlapSize,
							MaxChunkSize:    4000,
							EnableSplitting: bm.splitting,
						}

						err := runFunctionChunking(ctx, functions, config)
						if err != nil {
							b.Fatalf("Few large functions chunking failed: %v", err)
						}
					}
				})
			}
		})
	}
}

// BenchmarkComplexClassHierarchy benchmarks performance with complex class hierarchies.
func BenchmarkComplexClassHierarchy(b *testing.B) {
	ctx := context.Background()

	// Generate complex class hierarchy with inheritance and composition
	hierarchies := []struct {
		name    string
		depth   int
		width   int // classes per level
		methods int // methods per class
	}{
		{"Shallow_Wide", 3, 5, 10},
		{"Deep_Narrow", 8, 2, 5},
		{"Balanced", 5, 3, 8},
		{"Complex", 6, 4, 12},
	}

	for _, h := range hierarchies {
		b.Run(h.name, func(b *testing.B) {
			classes := generateComplexClassHierarchy(h.depth, h.width, h.methods)

			benchmarks := []struct {
				name        string
				overlapSize int
				strategy    outbound.ChunkingStrategyType
			}{
				{"Function_NoOverlap", 0, outbound.StrategyFunction},
				{"Function_Overlap", 300, outbound.StrategyFunction},
				{"Class_NoOverlap", 0, outbound.StrategyClass},
				{"Class_Overlap", 500, outbound.StrategyClass},
				{"Hybrid", 400, outbound.StrategyHybrid},
			}

			for _, bm := range benchmarks {
				b.Run(bm.name, func(b *testing.B) {
					b.ResetTimer()
					b.ReportAllocs()
					for range b.N {
						config := outbound.ChunkingConfiguration{
							Strategy:        bm.strategy,
							OverlapSize:     bm.overlapSize,
							MaxChunkSize:    3000,
							EnableSplitting: true,
						}

						err := runChunkingStrategy(ctx, classes, config, bm.strategy)
						if err != nil {
							b.Fatalf("Complex class hierarchy chunking failed: %v", err)
						}
					}
				})
			}
		})
	}
}

// BenchmarkMemoryScaling benchmarks memory usage scaling with file size.
func BenchmarkMemoryScaling(b *testing.B) {
	ctx := context.Background()

	sizes := []int{
		1 * 1024,   // 1KB
		5 * 1024,   // 5KB
		10 * 1024,  // 10KB
		25 * 1024,  // 25KB
		50 * 1024,  // 50KB
		100 * 1024, // 100KB
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%dKB", size/1024), func(b *testing.B) {
			singleFunction := outbound.SemanticCodeChunk{
				ChunkID:       fmt.Sprintf("large-func-%d", size),
				Name:          fmt.Sprintf("processFunction%d", size),
				Signature:     fmt.Sprintf("func processFunction%d(data []byte) error", size),
				Content:       generateLargeFunctionContent(size),
				Language:      mustCreateLanguage("go"),
				StartByte:     0,
				EndByte:       uint32(size),
				Type:          outbound.ConstructFunction,
				QualifiedName: fmt.Sprintf("app.processFunction%d", size),
			}

			chunker := NewLargeFunctionChunkerWithDefaults()

			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:        outbound.StrategySizeBased,
					MaxChunkSize:    size / 4,
					OverlapSize:     200,
					EnableSplitting: true,
				}

				_, err := chunker.ChunkCode(ctx, []outbound.SemanticCodeChunk{singleFunction}, config)
				if err != nil {
					b.Fatalf("Memory scaling chunking failed: %v", err)
				}
			}
		})
	}
}

// Helper functions for generating test data

func generateSmallFileChunks() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Small utility functions
	for i := range 5 {
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("small-func-%d", i),
			Name:          fmt.Sprintf("utilityFunc%d", i),
			Signature:     fmt.Sprintf("func utilityFunc%d() error", i),
			Content:       generateSmallFunctionContent(),
			Language:      mustCreateLanguage("go"),
			StartByte:     uint32(i * 200),
			EndByte:       uint32((i + 1) * 200),
			Type:          outbound.ConstructFunction,
			QualifiedName: fmt.Sprintf("util.utilityFunc%d", i),
		})
	}

	return chunks
}

func generateMediumFileChunks() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Medium-sized functions and classes
	classes := []int{1, 5, 10, 25, 50} // KB

	for _, size := range classes {
		content := generateMediumClassContent(size * 1024)
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("medium-class-%dKB", size),
			Name:          fmt.Sprintf("Processor%d", size),
			Signature:     fmt.Sprintf("class Processor%d:", size),
			Content:       content,
			Language:      mustCreateLanguage("python"),
			StartByte:     uint32(size * 1024),
			EndByte:       uint32(2 * size * 1024),
			Type:          outbound.ConstructClass,
			QualifiedName: fmt.Sprintf("models.Processor%d", size),
		})
	}

	return chunks
}

func generateLargeFileChunks() []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Large processing modules
	sizes := []int{15, 25, 50, 100} // KB

	for _, size := range sizes {
		content := generateLargeModuleContent(size * 1024)
		chunks = append(chunks, outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("large-module-%dKB", size),
			Name:          fmt.Sprintf("DataProcessor%d", size),
			Signature:     fmt.Sprintf("func DataProcessor%d(data []byte) error", size),
			Content:       content,
			Language:      mustCreateLanguage("go"),
			StartByte:     0,
			EndByte:       uint32(size * 1024),
			Type:          outbound.ConstructFunction,
			QualifiedName: fmt.Sprintf("processor.DataProcessor%d", size),
		})
	}

	return chunks
}

func generateManySmallFunctions(count int) []outbound.SemanticCodeChunk {
	chunks := make([]outbound.SemanticCodeChunk, count)
	for i := range count {
		chunks[i] = outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("util-func-%d", i),
			Name:          fmt.Sprintf("utilFunc%d", i),
			Signature:     fmt.Sprintf("func utilFunc%d(input string) string", i),
			Content:       generateSmallFunctionContent(),
			Language:      mustCreateLanguage("go"),
			StartByte:     uint32(i * 150),
			EndByte:       uint32((i + 1) * 150),
			Type:          outbound.ConstructFunction,
			QualifiedName: fmt.Sprintf("util.utilFunc%d", i),
			CalledFunctions: []outbound.FunctionCall{
				{Name: fmt.Sprintf("helperFunc%d", i%5)},
			},
		}
	}

	return chunks
}

func generateFewLargeFunctions(count, size int) []outbound.SemanticCodeChunk {
	chunks := make([]outbound.SemanticCodeChunk, count)
	for i := range count {
		chunks[i] = outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("large-func-%d", i),
			Name:          fmt.Sprintf("processFunction%d", i),
			Signature:     fmt.Sprintf("func processFunction%d(data []byte) error", i),
			Content:       generateLargeFunctionContent(size),
			Language:      mustCreateLanguage("go"),
			StartByte:     uint32(i * size),
			EndByte:       uint32((i + 1) * size),
			Type:          outbound.ConstructFunction,
			QualifiedName: fmt.Sprintf("processor.processFunction%d", i),
		}
	}

	return chunks
}

func generateComplexClassHierarchy(depth, width, methods int) []outbound.SemanticCodeChunk {
	var chunks []outbound.SemanticCodeChunk

	// Generate hierarchical class structure
	for level := range depth {
		for class := range width {
			classID := fmt.Sprintf("L%dC%d", level, class)
			content := generateClassWithMethods(level, class, methods)

			chunks = append(chunks, outbound.SemanticCodeChunk{
				ChunkID:       classID,
				Name:          fmt.Sprintf("Class%s", classID),
				Signature:     fmt.Sprintf("class Class%s", classID),
				Content:       content,
				Language:      mustCreateLanguage("python"),
				StartByte:     uint32(level*width*1000 + class*1000),
				EndByte:       uint32(level*width*1000 + (class+1)*1000),
				Type:          outbound.ConstructClass,
				QualifiedName: fmt.Sprintf("models.Class%s", classID),
			})

			// Add methods
			for method := range methods {
				methodID := fmt.Sprintf("L%dC%dM%d", level, class, method)
				chunks = append(chunks, outbound.SemanticCodeChunk{
					ChunkID:       methodID,
					Name:          fmt.Sprintf("method%d", method),
					Signature:     fmt.Sprintf("    def method%d(self, param):", method),
					Content:       generateMethodContent(method),
					Language:      mustCreateLanguage("python"),
					StartByte:     uint32(level*width*1000 + class*1000 + 200 + method*50),
					EndByte:       uint32(level*width*1000 + class*1000 + 200 + (method+1)*50),
					Type:          outbound.ConstructMethod,
					QualifiedName: fmt.Sprintf("models.Class%s.method%d", classID, method),
				})
			}
		}
	}

	return chunks
}

func filterChunksBySize(chunks []outbound.SemanticCodeChunk, targetSize int) []outbound.SemanticCodeChunk {
	var filtered []outbound.SemanticCodeChunk
	for _, chunk := range chunks {
		if int(chunk.EndByte-chunk.StartByte) <= targetSize*2 && int(chunk.EndByte-chunk.StartByte) >= targetSize/2 {
			filtered = append(filtered, chunk)
		}
	}
	return filtered
}

func runChunkingStrategy(
	ctx context.Context,
	chunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
	strategy outbound.ChunkingStrategyType,
) error {
	switch strategy {
	case outbound.StrategyFunction:
		return runFunctionChunking(ctx, chunks, config)
	case outbound.StrategyClass:
		return runClassChunking(ctx, chunks, config)
	case outbound.StrategySizeBased:
		return runSizeBasedChunking(ctx, chunks, config)
	case outbound.StrategyHybrid:
		return runHybridChunking(ctx, chunks, config)
	default:
		return runFunctionChunking(ctx, chunks, config)
	}
}

func runFunctionChunking(
	ctx context.Context,
	chunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) error {
	chunker := NewFunctionChunker()
	_, err := chunker.ChunkByFunction(ctx, chunks, config)
	return err
}

func runClassChunking(
	ctx context.Context,
	chunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) error {
	chunker := NewClassChunker()
	_, err := chunker.ChunkByClass(ctx, chunks, config)
	return err
}

func runSizeBasedChunking(
	ctx context.Context,
	chunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) error {
	chunker := NewLargeFunctionChunkerWithDefaults()
	_, err := chunker.ChunkCode(ctx, chunks, config)
	return err
}

func runHybridChunking(
	ctx context.Context,
	chunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) error {
	// Hybrid: try function first, fallback to class
	if err := runFunctionChunking(ctx, chunks, config); err != nil {
		return runClassChunking(ctx, chunks, config)
	}
	return nil
}

// Content generation helper functions

func generateSmallFunctionContent() string {
	return `func smallFunction() error {
    // Simple utility function
    result := processSomething()
    return validateResult(result)
}`
}

func generateMediumClassContent(size int) string {
	template := `class Processor%d:
    """Medium-sized processor with moderate complexity."""

    def __init__(self):
        self.config = {"enabled": True, "version": "1.0"}
        self.cache = {}
        self.metrics = {"processed": 0, "errors": 0}

    def process(self, data):
        """Process data with multiple steps."""
        for item in data:
            try:
                processed = self._transform(item)
                self.cache[item.id] = processed
                self.metrics["processed"] += 1
            except Exception as e:
                self.metrics["errors"] += 1

        return self.metrics

    def _transform(self, item):
        """Transform single item."""
        return {"original": item, "processed": str(item).upper()}

    # Additional methods for size padding
%s
`

	methods := ""
	var methodsSb585 strings.Builder
	for i := range size / 500 {
		methodsSb585.WriteString(fmt.Sprintf(
			"    def method%d(self, param):\n        return param + '_processed_%d'\n",
			i, i,
		))
	}
	methods += methodsSb585.String()

	return fmt.Sprintf(template, size/1000, methods)
}

func generateLargeModuleContent(size int) string {
	template := `func DataProcessor%d(data []byte) error {
    if len(data) == 0 {
        return errors.New("empty data")
    }

    // Phase 1: Data validation
    if err := validateData(data); err != nil {
        return fmt.Errorf("validation failed: %%w", err)
    }

    // Phase 2: Processing loops
    %s

    // Phase 3: Final validation
    if err := finalValidation(); err != nil {
        return fmt.Errorf("final validation failed: %%w", err)
    }

    return nil
}
`

	processing := ""
	var processingSb619 strings.Builder
	for i := range size / 100 {
		processingSb619.WriteString(fmt.Sprintf(
			"// Processing step %d\nif err := processStep%d(data[i:i+100]); err != nil {\n    return err\n}\ni += 100\n",
			i,
			i,
		))
	}
	processing += processingSb619.String()

	return fmt.Sprintf(template, size/1000, processing)
}

func generateClassWithMethods(level, class, methods int) string {
	template := `class L%dC%d:
    """Class at level %d, position %d."""

    def __init__(self):
        self.level = %d
        self.position = %d
        self.parent = None

%s
`

	methodContent := ""
	var methodContentSb643 strings.Builder
	for i := range methods {
		methodContentSb643.WriteString(fmt.Sprintf(
			"    def method%d(self, param):\n        return f'processed_{param}_{%d}'\n",
			i, i,
		))
	}
	methodContent += methodContentSb643.String()

	return fmt.Sprintf(template, level, class, level, class, level, class, methodContent)
}

func generateMethodContent(index int) string {
	return fmt.Sprintf(`        return "result_%d"`, index)
}

func generateLargeFunctionContent(size int) string {
	template := `func largeFunction%d(data []byte) error {
    if len(data) == 0 {
        return errors.New("empty data")
    }

    // Complex processing logic
    result := make([]byte, 0, len(data))

    %s

    if err := validateResult(result); err != nil {
        return fmt.Errorf("validation failed: %%w", err)
    }

    return nil
}`

	processing := ""
	var processingSb676 strings.Builder
	for i := range size / 500 {
		processingSb676.WriteString(fmt.Sprintf(
			"chunk := data[i:min(i+500, len(data))]\nif err := processChunk%d(chunk); err != nil {\n    return err\n}\nresult = append(result, chunk...)\ni += 500\n",
			i,
		))
	}
	processing += processingSb676.String()

	return fmt.Sprintf(template, size/1000, processing)
}
