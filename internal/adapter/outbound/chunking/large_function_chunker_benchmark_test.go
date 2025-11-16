package chunking

import (
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"testing"
)

// BenchmarkLargeFunctionChunkingPerformance benchmarks the performance of large function chunking with various configurations.
func BenchmarkLargeFunctionChunkingPerformance(b *testing.B) {
	chunker := NewLargeFunctionChunkerWithDefaults()
	ctx := context.Background()

	// Create a very large function that exceeds thresholds
	largeFunction := outbound.SemanticCodeChunk{
		ChunkID:       "large-function",
		Name:          "processMassiveDataset",
		Signature:     "func processMassiveDataset(data []byte) error",
		Content:       createMassiveFunctionContent(),
		Language:      mustCreateLanguage("go"),
		StartByte:     0,
		EndByte:       60000, // ~60KB, exceeds optimal threshold
		Type:          outbound.ConstructFunction,
		QualifiedName: "app.processMassiveDataset",
	}

	benchmarks := []struct {
		name             string
		maxChunkSize     int
		optimalThreshold int
		overlapSize      int
		enableSplitting  bool
	}{
		{"SmallChunks", 2000, 5000, 100, true},
		{"MediumChunks", 5000, 10000, 200, true},
		{"LargeChunks", 10000, 20000, 500, true},
		{"NoSplitting", 15000, 20000, 0, false},
		{"MinimalOverlap", 5000, 10000, 50, true},
		{"LargeOverlap", 8000, 15000, 800, true},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:        outbound.StrategySizeBased,
					MaxChunkSize:    bm.maxChunkSize,
					OverlapSize:     bm.overlapSize,
					EnableSplitting: bm.enableSplitting,
				}

				// Override default options
				chunker.options.OptimalThreshold = bm.optimalThreshold

				_, err := chunker.ChunkCode(ctx, []outbound.SemanticCodeChunk{largeFunction}, config)
				if err != nil {
					b.Fatalf("Large function chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkLargeFunctionMemoryAllocation benchmarks memory allocation patterns in large function chunking.
func BenchmarkLargeFunctionMemoryAllocation(b *testing.B) {
	chunker := NewLargeFunctionChunkerWithDefaults()
	ctx := context.Background()

	// Create multiple large functions with varying sizes
	largeFunctions := make([]outbound.SemanticCodeChunk, 5)
	for i := range 5 {
		size := 20000 + i*10000 // From 20KB to 60KB
		largeFunctions[i] = outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("large-function-%d", i),
			Name:          fmt.Sprintf("largeFunction%d", i),
			Signature:     fmt.Sprintf("func largeFunction%d(data []byte) error", i),
			Content:       createLargeFunctionContentForBenchmarks(size),
			Language:      mustCreateLanguage("go"),
			StartByte:     uint32(i * 70000),
			EndByte:       uint32((i + 1) * 70000),
			Type:          outbound.ConstructFunction,
			QualifiedName: fmt.Sprintf("app.largeFunction%d", i),
		}
	}

	benchmarks := []struct {
		name         string
		overlapSize  int
		maxChunkSize int
	}{
		{"NoOverlap_OptimalSize", 0, 5000},
		{"SmallOverlap_OptimalSize", 100, 5000},
		{"LargeOverlap_OptimalSize", 500, 5000},
		{"NoOverlap_LargeSize", 0, 10000},
		{"SmallOverlap_LargeSize", 200, 10000},
		{"LargeOverlap_LargeSize", 800, 10000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:        outbound.StrategySizeBased,
					MaxChunkSize:    bm.maxChunkSize,
					OverlapSize:     bm.overlapSize,
					EnableSplitting: true,
				}

				_, err := chunker.ChunkCode(ctx, largeFunctions, config)
				if err != nil {
					b.Fatalf("Large function chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkOverlapConfigurationGranular tests fine-grained overlap configuration performance.
func BenchmarkOverlapConfigurationGranular(b *testing.B) {
	chunker := NewLargeFunctionChunkerWithDefaults()
	ctx := context.Background()

	// Create a very large function suitable for splitting
	massiveFunction := outbound.SemanticCodeChunk{
		ChunkID:       "massive-function",
		Name:          "massiveFunction",
		Signature:     "func massiveFunction(input interface{}) error",
		Content:       createMassiveFunctionContent(),
		Language:      mustCreateLanguage("go"),
		StartByte:     0,
		EndByte:       80000, // ~80KB
		Type:          outbound.ConstructFunction,
		QualifiedName: "app.massiveFunction",
	}

	overlapSizes := []int{0, 50, 100, 200, 300, 400, 500, 750, 1000, 1500}

	for _, overlap := range overlapSizes {
		b.Run(fmt.Sprintf("Overlap%d", overlap), func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:        outbound.StrategySizeBased,
					MaxChunkSize:    6000,
					OverlapSize:     overlap,
					EnableSplitting: true,
				}

				_, err := chunker.ChunkCode(ctx, []outbound.SemanticCodeChunk{massiveFunction}, config)
				if err != nil {
					b.Fatalf("Large function chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkThresholdConfiguration tests different size thresholds and their performance impact.
func BenchmarkThresholdConfiguration(b *testing.B) {
	ctx := context.Background()

	// Create mixed-size functions
	mixedFunctions := make([]outbound.SemanticCodeChunk, 10)
	for i := range 10 {
		size := 5000 + i*2000 // From 5KB to 23KB
		mixedFunctions[i] = outbound.SemanticCodeChunk{
			ChunkID:       fmt.Sprintf("mixed-func-%d", i),
			Name:          fmt.Sprintf("mixedFunction%d", i),
			Signature:     fmt.Sprintf("func mixedFunction%d(data []byte) error", i),
			Content:       createLargeFunctionContentForBenchmarks(size),
			Language:      mustCreateLanguage("go"),
			StartByte:     uint32(i * 30000),
			EndByte:       uint32((i + 1) * 30000),
			Type:          outbound.ConstructFunction,
			QualifiedName: fmt.Sprintf("app.mixedFunction%d", i),
		}
	}

	thresholds := []struct {
		name    string
		optimal int
		max     int
		min     int
	}{
		{"Conservative", 5000, 15000, 1000},
		{"Balanced", 10000, 20000, 2000},
		{"Aggressive", 15000, 30000, 3000},
		{"Minimal", 3000, 10000, 500},
		{"Generous", 20000, 40000, 5000},
	}

	for _, threshold := range thresholds {
		b.Run(threshold.name, func(b *testing.B) {
			chunker := &LargeFunctionChunker{
				parserFactory: &MockTreeSitterParserFactory{},
				options: SizeChunkingOptions{
					OptimalThreshold: threshold.optimal,
					MaxThreshold:     threshold.max,
					MinChunkSize:     threshold.min,
				},
			}

			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:        outbound.StrategySizeBased,
					MaxChunkSize:    threshold.max / 2,
					OverlapSize:     200,
					EnableSplitting: true,
				}

				_, err := chunker.ChunkCode(ctx, mixedFunctions, config)
				if err != nil {
					b.Fatalf("Large function chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkMultiLanguageLargeFunction tests performance with different languages for large functions.
func BenchmarkMultiLanguageLargeFunction(b *testing.B) {
	ctx := context.Background()

	languages := []struct {
		name     string
		lang     string
		content  string
		sig      string
		funcName string
	}{
		{
			name:     "Go",
			lang:     "go",
			content:  createGoLargeFunctionContent(),
			sig:      "func processLargeDataset(data []byte) error",
			funcName: "processLargeDataset",
		},
		{
			name:     "Python",
			lang:     "python",
			content:  createPythonLargeFunctionContent(),
			sig:      "def process_large_dataset(data: bytes) -> None:",
			funcName: "process_large_dataset",
		},
		{
			name:     "JavaScript",
			lang:     "javascript",
			content:  createJSLargeFunctionContent(),
			sig:      "function processLargeDataset(data) {",
			funcName: "processLargeDataset",
		},
		{
			name:     "TypeScript",
			lang:     "typescript",
			content:  createTSLargeFunctionContent(),
			sig:      "function processLargeDataset(data: Buffer): Promise<void> {",
			funcName: "processLargeDataset",
		},
	}

	for _, lang := range languages {
		b.Run(lang.name, func(b *testing.B) {
			chunker := NewLargeFunctionChunkerWithDefaults()

			largeFunction := outbound.SemanticCodeChunk{
				ChunkID:       fmt.Sprintf("large-func-%s", lang.name),
				Name:          lang.funcName,
				Signature:     lang.sig,
				Content:       lang.content,
				Language:      mustCreateLanguage(lang.lang),
				StartByte:     0,
				EndByte:       uint32(len(lang.content)),
				Type:          outbound.ConstructFunction,
				QualifiedName: fmt.Sprintf("app.%s", lang.funcName),
			}

			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:        outbound.StrategySizeBased,
					MaxChunkSize:    8000,
					OverlapSize:     300,
					EnableSplitting: true,
				}

				_, err := chunker.ChunkCode(ctx, []outbound.SemanticCodeChunk{largeFunction}, config)
				if err != nil {
					b.Fatalf("Large function chunking failed for %s: %v", lang.name, err)
				}
			}
		})
	}
}

// BenchmarkSplitQuality tests the quality and performance of splitting algorithms.
func BenchmarkSplitQuality(b *testing.B) {
	chunker := NewLargeFunctionChunkerWithDefaults()
	ctx := context.Background()

	// Create functions with different complexity patterns
	complexityPatterns := []struct {
		name    string
		content string
	}{
		{"LinearPattern", createLinearPatternFunction()},
		{"NestedLoops", createNestedLoopsFunction()},
		{"ConditionalChains", createConditionalChainsFunction()},
		{"SwitchStatements", createSwitchStatementsFunction()},
		{"ErrorHandling", createErrorHandlingFunction()},
	}

	for _, pattern := range complexityPatterns {
		b.Run(pattern.name, func(b *testing.B) {
			complexFunction := outbound.SemanticCodeChunk{
				ChunkID:       fmt.Sprintf("complex-func-%s", pattern.name),
				Name:          fmt.Sprintf("complexFunction%s", pattern.name),
				Signature:     "func complexFunction() error",
				Content:       pattern.content,
				Language:      mustCreateLanguage("go"),
				StartByte:     0,
				EndByte:       uint32(len(pattern.content)),
				Type:          outbound.ConstructFunction,
				QualifiedName: fmt.Sprintf("app.complexFunction%s", pattern.name),
			}

			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:        outbound.StrategySizeBased,
					MaxChunkSize:    4000,
					OverlapSize:     200,
					EnableSplitting: true,
				}

				chunks, err := chunker.ChunkCode(ctx, []outbound.SemanticCodeChunk{complexFunction}, config)
				if err != nil {
					b.Fatalf("Large function chunking failed: %v", err)
				}

				// Basic quality check
				if len(chunks) == 1 && len(pattern.content) > 4000 {
					b.Fatalf("Expected splitting for large function, got no splits")
				}
			}
		})
	}
}

// Helper functions for creating test content

func createMassiveFunctionContent() string {
	return `func processMassiveDataset(data []byte) error {
    if len(data) == 0 {
        return errors.New("empty data")
    }

    // Phase 1: Data validation and preprocessing
    var validItems []DataItem
    for i := 0; i < len(data); i += 1024 {
        chunk := data[i:min(i+1024, len(data))]
        if err := validateDataChunk(chunk); err != nil {
            return fmt.Errorf("validation failed at chunk %d: %w", i/1024, err)
        }

        items, err := parseDataChunk(chunk)
        if err != nil {
            return fmt.Errorf("parsing failed at chunk %d: %w", i/1024, err)
        }
        validItems = append(validItems, items...)
    }

    // Phase 2: Complex data transformations
    transformedItems := make([]ProcessedItem, 0, len(validItems))
    for idx, item := range validItems {
        // Apply multiple transformation steps
        processed, err := applyTransformations(item)
        if err != nil {
            return fmt.Errorf("transformation failed for item %d: %w", idx, err)
        }

        // Additional processing based on item type
        switch item.Type {
        case ItemTypeA:
            processed, err = processTypeA(processed)
        case ItemTypeB:
            processed, err = processTypeB(processed)
        case ItemTypeC:
            processed, err = processTypeC(processed)
        default:
            return fmt.Errorf("unknown item type: %s", item.Type)
        }

        if err != nil {
            return fmt.Errorf("type-specific processing failed for item %d: %w", idx, err)
        }

        transformedItems = append(transformedItems, processed)

        // Progress logging every 1000 items
        if idx > 0 && idx%1000 == 0 {
            log.Printf("Processed %d/%d items", idx, len(validItems))
        }
    }

    // Phase 3: Aggregation and analysis
    results := make(map[string]interface{})
    for _, item := range transformedItems {
        if err := aggregateItem(&results, item); err != nil {
            return fmt.Errorf("aggregation failed: %w", err)
        }
    }

    // Phase 4: Final validation and cleanup
    if err := validateResults(results); err != nil {
        return fmt.Errorf("final validation failed: %w", err)
    }

    // Save results
    if err := saveResults(results); err != nil {
        return fmt.Errorf("saving results failed: %w", err)
    }

    return nil
}

func validateDataChunk(chunk []byte) error {
    // Implementation for validation
    return nil
}

func parseDataChunk(chunk []byte) ([]DataItem, error) {
    // Implementation for parsing
    return nil, nil
}

func applyTransformations(item DataItem) (ProcessedItem, error) {
    // Implementation for transformations
    return ProcessedItem{}, nil
}

func processTypeA(item ProcessedItem) (ProcessedItem, error) {
    // Implementation for type A processing
    return item, nil
}

func processTypeB(item ProcessedItem) (ProcessedItem, error) {
    // Implementation for type B processing
    return item, nil
}

func processTypeC(item ProcessedItem) (ProcessedItem, error) {
    // Implementation for type C processing
    return item, nil
}

func aggregateItem(results *map[string]interface{}, item ProcessedItem) error {
    // Implementation for aggregation
    return nil
}

func validateResults(results map[string]interface{}) error {
    // Implementation for validation
    return nil
}

func saveResults(results map[string]interface{}) error {
    // Implementation for saving
    return nil
}

// Supporting types
type DataItem struct {
    Type string
    Data []byte
}

type ProcessedItem struct {
    ID     string
    Result interface{}
}`
}

func createLargeFunctionContentForBenchmarks(targetSize int) string {
	template := `func largeFunction%d(data []byte) error {
    if len(data) == 0 {
        return errors.New("empty data")
    }

    // Complex processing logic for large function %d
    var result []byte
    var err error

    // Multiple processing phases
    %s

    // Error handling and cleanup
    if err != nil {
        return fmt.Errorf("processing failed: %%w", err)
    }

    return nil
}`

	// Generate filler content to reach target size
	fillerCode := ""
	for len(fillerCode) < targetSize-500 {
		stepNum := len(fillerCode) / 1000
		fillerCode += fmt.Sprintf(`
    // Processing step %d for function %d
    chunk := data[i:min(i+1024, len(data))]
    if err := processChunk%d(chunk); err != nil {
        return fmt.Errorf("chunk processing %d failed: %%w", stepNum, err)
    }
    i += 1024`, stepNum, targetSize, stepNum, stepNum)
	}

	return fmt.Sprintf(template, targetSize, targetSize, fillerCode)
}

func createGoLargeFunctionContent() string {
	return `func processLargeDataset(data []byte) error {
    if len(data) == 0 {
        return fmt.Errorf("empty dataset")
    }

    // Initialize processing context
    ctx := context.Background()
    processor := NewDataProcessor()

    // Process data in batches
    batchSize := 1000
    for i := 0; i < len(data); i += batchSize {
        end := i + batchSize
        if end > len(data) {
            end = len(data)
        }

        batch := data[i:end]
        if err := processor.ProcessBatch(ctx, batch); err != nil {
            return fmt.Errorf("batch processing failed at index %d: %w", i, err)
        }

        // Memory cleanup
        runtime.GC()
    }

    return processor.Finalize(ctx)
}`
}

func createPythonLargeFunctionContent() string {
	return `def process_large_dataset(data: bytes) -> None:
    """Process a large dataset efficiently."""
    if not data:
        raise ValueError("Empty dataset")

    # Initialize processing components
    processor = DataProcessor()
    batch_size = 1000

    # Process data in batches
    for i in range(0, len(data), batch_size):
        batch = data[i:min(i + batch_size, len(data))]

        try:
            processor.process_batch(batch)
        except Exception as e:
            raise RuntimeError(f"Batch processing failed at index {i}: {e}")

        # Memory management
        if i % (batch_size * 10) == 0:
            gc.collect()

    # Finalize processing
    processor.finalize()`
}

func createJSLargeFunctionContent() string {
	return `function processLargeDataset(data) {
    if (!data || data.length === 0) {
        throw new Error('Empty dataset');
    }

    const processor = new DataProcessor();
    const batchSize = 1000;

    // Process data in batches
    for (let i = 0; i < data.length; i += batchSize) {
        const batch = data.slice(i, Math.min(i + batchSize, data.length));

        try {
            await processor.processBatch(batch);
        } catch (error) {
            throw new Error("Batch processing failed at index " + i + ": " + error.message);
        }

        // Memory management
        if (i % (batchSize * 10) === 0) {
            if (global.gc) global.gc();
        }
    }

    return processor.finalize();
}`
}

func createTSLargeFunctionContent() string {
	return `async function processLargeDataset(data: Buffer): Promise<void> {
    if (!data || data.length === 0) {
        throw new Error('Empty dataset');
    }

    const processor = new DataProcessor();
    const batchSize = 1000;

    // Process data in batches
    for (let i = 0; i < data.length; i += batchSize) {
        const batch = data.slice(i, Math.min(i + batchSize, data.length));

        try {
            await processor.processBatch(batch);
        } catch (error: any) {
            throw new Error("Batch processing failed at index " + i + ": " + error.message);
        }

        // Type-safe memory management
        if (i % (batchSize * 10) === 0) {
            this.clearMemory();
        }
    }

    return processor.finalize() as Promise<void>;
}`
}

func createLinearPatternFunction() string {
	return `func linearPatternFunction(data []byte) error {
    for i := 0; i < len(data); i++ {
        if err := process(data[i]); err != nil {
            return err
        }
        if err := validate(data[i]); err != nil {
            return err
        }
        if err := transform(data[i]); err != nil {
            return err
        }
    }
    return nil
}`
}

func createNestedLoopsFunction() string {
	return `func nestedLoopsFunction(matrix [][]int) error {
    for i := 0; i < len(matrix); i++ {
        for j := 0; j < len(matrix[i]); j++ {
            for k := 0; k < matrix[i][j]; k++ {
                if err := processCell(i, j, k); err != nil {
                    return err
                }
                if err := validateCell(matrix[i][j]); err != nil {
                    return err
                }
            }
        }
    }
    return nil
}`
}

func createConditionalChainsFunction() string {
	return `func conditionalChainsFunction(input ProcessInput) error {
    if input.Type == TypeA {
        if input.Version > 2 {
            if input.Flags&FlagEnabled != 0 {
                if err := processEnabledA(input); err != nil {
                    return err
                }
            }
            if input.Flags&FlagRequired != 0 {
                if err := processRequiredA(input); err != nil {
                    return err
                }
            }
        }
        if input.Flags&FlagOptional != 0 {
            if err := processOptionalA(input); err != nil {
                return err
            }
        }
    } else if input.Type == TypeB {
        // Similar nested conditionals for TypeB
        if input.Version > 1 {
            if err := processB(input); err != nil {
                return err
            }
        }
    }
    return nil
}`
}

func createSwitchStatementsFunction() string {
	return `func switchStatementsFunction(data []DataItem) error {
    for _, item := range data {
        switch item.Type {
        case "string":
            switch item.Subtype {
            case "json":
                if err := processJSONString(item); err != nil {
                    return err
                }
            case "xml":
                if err := processXMLString(item); err != nil {
                    return err
                }
            default:
                if err := processGenericString(item); err != nil {
                    return err
                }
            }
        case "number":
            switch item.Format {
            case "int":
                if err := processIntNumber(item); err != nil {
                    return err
                }
            case "float":
                if err := processFloatNumber(item); err != nil {
                    return err
                }
            default:
                if err := processGenericNumber(item); err != nil {
                    return err
                }
            }
        default:
            if err := processUnknownType(item); err != nil {
                return err
            }
        }
    }
    return nil
}`
}

func createErrorHandlingFunction() string {
	return `func errorHandlingFunction(input ComplexInput) error {
    validator, err := NewValidator(input.Type)
    if err != nil {
        return fmt.Errorf("validator creation failed: %w", err)
    }

    if err := validator.Validate(input); err != nil {
        if IsValidationError(err) {
            return fmt.Errorf("validation error: %w", err)
        }
        if IsConfigError(err) {
            if err := LoadDefaultConfig(); err != nil {
                return fmt.Errorf("failed to load default config: %w", err)
            }
            if err := validator.Validate(input); err != nil {
                return fmt.Errorf("validation still failed with default config: %w", err)
            }
        }
        return fmt.Errorf("unexpected validation error: %w", err)
    }

    processor, err := NewProcessor(input.Type)
    if err != nil {
        return fmt.Errorf("processor creation failed: %w", err)
    }

    result, err := processor.Process(input)
    if err != nil {
        if IsProcessingError(err) {
            if err := processor.Retry(input, 3); err != nil {
                return fmt.Errorf("retry failed: %w", err)
            }
        } else {
            return fmt.Errorf("processing failed: %w", err)
        }
    }

    if err := SaveResult(result); err != nil {
        if IsStorageError(err) {
            if err := ClearCache(); err != nil {
                return fmt.Errorf("cache cleanup failed: %w", err)
            }
            if err := SaveResult(result); err != nil {
                return fmt.Errorf("save retry failed: %w", err)
            }
        }
        return fmt.Errorf("save failed: %w", err)
    }

    return nil
}
`
}
