package chunking

import (
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"testing"
)

// BenchmarkFunctionChunkingWithOverlap benchmarks function-level chunking with various overlap sizes.
func BenchmarkFunctionChunkingWithOverlap(b *testing.B) {
	chunker := NewFunctionChunker()
	ctx := context.Background()

	// Create test semantic chunks
	smallFunction := outbound.SemanticCodeChunk{
		ChunkID:         "small-func",
		Name:            "smallFunction",
		Signature:       "func smallFunction() error",
		Content:         "func smallFunction() error {\n    return fmt.Errorf(\"small error\")\n}",
		Language:        mustCreateLanguage("go"),
		StartByte:       0,
		EndByte:         80,
		CalledFunctions: []outbound.FunctionCall{{Name: "fmt.Errorf"}},
		Type:            outbound.ConstructFunction,
	}

	mediumFunction := outbound.SemanticCodeChunk{
		ChunkID:   "medium-func",
		Name:      "mediumFunction",
		Signature: "func mediumFunction(data []byte) (string, error)",
		Content: `func mediumFunction(data []byte) (string, error) {
    if len(data) == 0 {
        return "", errors.New("empty data")
    }

    reader := bytes.NewReader(data)
    buf := new(bytes.Buffer)

    for {
        chunk := make([]byte, 1024)
        n, err := reader.Read(chunk)
        if err != nil {
            if err == io.EOF {
                break
            }
            return "", fmt.Errorf("read error: %w", err)
        }

        if n > 0 {
            buf.Write(chunk[:n])
        }
    }

    return buf.String(), nil
}`,
		Language:  mustCreateLanguage("go"),
		StartByte: 100,
		EndByte:   400,
		CalledFunctions: []outbound.FunctionCall{
			{Name: "bytes.NewReader"},
			{Name: "bytes.NewBuffer"},
			{Name: "reader.Read"},
			{Name: "buf.Write"},
		},
		Type: outbound.ConstructFunction,
	}

	largeFunction := outbound.SemanticCodeChunk{
		ChunkID:   "large-func",
		Name:      "largeFunction",
		Signature: "func largeFunction(source io.Reader, destination io.Writer) error",
		Content: `func largeFunction(source io.Reader, destination io.Writer) error {
    scanner := bufio.NewScanner(source)
    var processedCount int

    for scanner.Scan() {
        line := scanner.Text()
        if processed := processLine(line); !processed {
            continue
        }

        // Write statistics
        stats := generateStatistics()
        if err := writeStats(destination, stats); err != nil {
            return fmt.Errorf("writing stats failed: %w", err)
        }

        processedCount++
        if processedCount%1000 == 0 {
            log.Printf("Processed %d lines", processedCount)
        }
    }

    if err := scanner.Err(); err != nil {
        return fmt.Errorf("scanner error: %w", err)
    }

    return nil
}`,
		Language:  mustCreateLanguage("go"),
		StartByte: 500,
		EndByte:   800,
		CalledFunctions: []outbound.FunctionCall{
			{Name: "bufio.NewScanner"},
			{Name: "processLine"},
			{Name: "generateStatistics"},
			{Name: "writeStats"},
			{Name: "log.Printf"},
		},
		Type: outbound.ConstructFunction,
	}

	functions := []outbound.SemanticCodeChunk{smallFunction, mediumFunction, largeFunction}

	benchmarks := []struct {
		name        string
		overlapSize int
	}{
		{"NoOverlap", 0},
		{"Overlap100", 100},
		{"Overlap200", 200},
		{"Overlap500", 500},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:             outbound.StrategyFunction,
					OverlapSize:          bm.overlapSize,
					IncludeDocumentation: true,
				}

				_, err := chunker.ChunkByFunction(ctx, functions, config)
				if err != nil {
					b.Fatalf("Chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkFunctionChunkingMemoryAllocation benchmarks memory allocation patterns in function chunking.
func BenchmarkFunctionChunkingMemoryAllocation(b *testing.B) {
	chunker := NewFunctionChunker()
	ctx := context.Background()

	// Create test functions with varying complexity
	functions := make([]outbound.SemanticCodeChunk, 10)
	for i := range 10 {
		content := createLargeFunctionContent(i * 100) // Varying sizes
		functions[i] = outbound.SemanticCodeChunk{
			ChunkID:   fmt.Sprintf("function-%d", i),
			Name:      fmt.Sprintf("function%d", i),
			Signature: fmt.Sprintf("func function%d(data []byte) error", i),
			Content:   content,
			Language:  mustCreateLanguage("go"),
			StartByte: uint32(i * 1000),
			EndByte:   uint32((i + 1) * 1000),
			CalledFunctions: []outbound.FunctionCall{
				{Name: fmt.Sprintf("helperFunction%d", i%3)},
			},
			Type: outbound.ConstructFunction,
		}
	}

	benchmarks := []struct {
		name        string
		overlapSize int
	}{
		{"NoOverlap", 0},
		{"SmallOverlap", 100},
		{"LargeOverlap", 500},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:             outbound.StrategyFunction,
					OverlapSize:          bm.overlapSize,
					IncludeDocumentation: true,
					PreserveDependencies: true,
				}

				_, err := chunker.ChunkByFunction(ctx, functions, config)
				if err != nil {
					b.Fatalf("Chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkFunctionChunkingWithDependencies tests performance impact of dependency preservation.
func BenchmarkFunctionChunkingWithDependencies(b *testing.B) {
	chunker := NewFunctionChunker()
	ctx := context.Background()

	// Create functions with complex dependency chains
	mainFunction := outbound.SemanticCodeChunk{
		ChunkID:   "main-func",
		Name:      "mainFunction",
		Signature: "func mainFunction() error",
		Content:   createMainFunctionContent(),
		Language:  mustCreateLanguage("go"),
		StartByte: 0,
		EndByte:   600,
		CalledFunctions: []outbound.FunctionCall{
			{Name: "validateInput"},
			{Name: "processData"},
			{Name: "generateOutput"},
			{Name: "saveResults"},
		},
		Type: outbound.ConstructFunction,
	}

	// Create helper functions
	helperFunctions := []outbound.SemanticCodeChunk{
		{
			ChunkID:   "validateInput",
			Name:      "validateInput",
			Signature: "func validateInput(input string) error",
			Content:   createHelperFunctionContent("validateInput"),
			Language:  mustCreateLanguage("go"),
			StartByte: 700,
			EndByte:   800,
			Type:      outbound.ConstructFunction,
		},
		{
			ChunkID:   "processData",
			Name:      "processData",
			Signature: "func processData(input string) ([]byte, error)",
			Content:   createHelperFunctionContent("processData"),
			Language:  mustCreateLanguage("go"),
			StartByte: 900,
			EndByte:   1000,
			Type:      outbound.ConstructFunction,
		},
		{
			ChunkID:   "generateOutput",
			Name:      "generateOutput",
			Signature: "func generateOutput(data []byte) string",
			Content:   createHelperFunctionContent("generateOutput"),
			Language:  mustCreateLanguage("go"),
			StartByte: 1100,
			EndByte:   1200,
			Type:      outbound.ConstructFunction,
		},
		{
			ChunkID:   "saveResults",
			Name:      "saveResults",
			Signature: "func saveResults(output string) error",
			Content:   createHelperFunctionContent("saveResults"),
			Language:  mustCreateLanguage("go"),
			StartByte: 1300,
			EndByte:   1400,
			Type:      outbound.ConstructFunction,
		},
	}

	functions := append([]outbound.SemanticCodeChunk{mainFunction}, helperFunctions...)

	benchmarks := []struct {
		name                 string
		preserveDeps         bool
		overlapSize          int
		includeDocumentation bool
	}{
		{"NoDepsNoOverlap", false, 0, false},
		{"PreserveDeps", true, 0, false},
		{"PreserveDepsWithOverlap", true, 200, false},
		{"PreserveDepsWithDoc", true, 0, true},
		{"FullFeatures", true, 200, true},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:             outbound.StrategyFunction,
					PreserveDependencies: bm.preserveDeps,
					OverlapSize:          bm.overlapSize,
					IncludeDocumentation: bm.includeDocumentation,
				}

				_, err := chunker.ChunkByFunction(ctx, functions, config)
				if err != nil {
					b.Fatalf("Chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkChunkSizeScaling benchmarks function chunking with different number of functions.
func BenchmarkChunkSizeScaling(b *testing.B) {
	chunker := NewFunctionChunker()
	ctx := context.Background()

	sizes := []int{
		5, 10, 25, 50, 100, 200,
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Functions%d", size), func(b *testing.B) {
			// Create functions of the specified size
			functions := make([]outbound.SemanticCodeChunk, size)
			for i := range size {
				functions[i] = outbound.SemanticCodeChunk{
					ChunkID:   fmt.Sprintf("func-%d", i),
					Name:      fmt.Sprintf("function%d", i),
					Signature: fmt.Sprintf("func function%d() error", i),
					Content:   createFunctionContent(i, 50),
					Language:  mustCreateLanguage("go"),
					StartByte: uint32(i * 500),
					EndByte:   uint32((i + 1) * 500),
					CalledFunctions: []outbound.FunctionCall{
						{Name: fmt.Sprintf("helper%d", i%5)},
					},
					Type: outbound.ConstructFunction,
				}
			}

			b.ResetTimer()
			b.ReportAllocs()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:             outbound.StrategyFunction,
					OverlapSize:          200,
					IncludeDocumentation: true,
					PreserveDependencies: true,
				}

				_, err := chunker.ChunkByFunction(ctx, functions, config)
				if err != nil {
					b.Fatalf("Chunking failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkOverlapSizeGranular provides fine-grained overlap size benchmarks.
func BenchmarkOverlapSizeGranular(b *testing.B) {
	chunker := NewFunctionChunker()
	ctx := context.Background()

	// Create moderately complex test functions
	functions := make([]outbound.SemanticCodeChunk, 3)
	for i := range 3 {
		functions[i] = outbound.SemanticCodeChunk{
			ChunkID:   fmt.Sprintf("func-%d", i),
			Name:      fmt.Sprintf("function%d", i),
			Signature: fmt.Sprintf("func function%d(data interface{}) error", i),
			Content:   createComplexFunctionContent(i),
			Language:  mustCreateLanguage("go"),
			StartByte: uint32(i * 800),
			EndByte:   uint32((i + 1) * 800),
			CalledFunctions: []outbound.FunctionCall{
				{Name: "validate"},
				{Name: "transform"},
				{Name: "persist"},
			},
			Type: outbound.ConstructFunction,
		}
	}

	overlapSizes := []int{0, 50, 100, 150, 200, 300, 400, 500, 750, 1000}

	for _, overlap := range overlapSizes {
		b.Run(fmt.Sprintf("Overlap%d", overlap), func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				config := outbound.ChunkingConfiguration{
					Strategy:             outbound.StrategyFunction,
					OverlapSize:          overlap,
					IncludeDocumentation: true,
				}

				_, err := chunker.ChunkByFunction(ctx, functions, config)
				if err != nil {
					b.Fatalf("Chunking failed: %v", err)
				}
			}
		})
	}
}

// Helper functions for creating test content

func createLargeFunctionContent(size int) string {
	template := `func largeFunction%d(data []byte) error {
    if len(data) == 0 {
        return errors.New("empty data")
    }

    // Process data in chunks
    for i := 0; i < len(data); i += 1024 {
        end := i + 1024
        if end > len(data) {
            end = len(data)
        }

        chunk := data[i:end]
        if err := processChunk%d(chunk); err != nil {
            return fmt.Errorf("failed to process chunk %d: %%w", err, i/1024)
        }
    }

    return nil
}`

	return fmt.Sprintf(template, size, size)
}

func createMainFunctionContent() string {
	return `func mainFunction() error {
    input := os.Getenv("INPUT_DATA")
    if input == "" {
        return errors.New("input data required")
    }

    if err := validateInput(input); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    data, err := processData(input)
    if err != nil {
        return fmt.Errorf("processing failed: %w", err)
    }

    output := generateOutput(data)
    if err := saveResults(output); err != nil {
        return fmt.Errorf("saving failed: %w", err)
    }

    return nil
}`
}

func createHelperFunctionContent(funcName string) string {
	template := `func %s(input string) error {
    if input == "" {
        return errors.New("input cannot be empty")
    }

    // Validation and processing logic
    result := someInternalFunction(input)
    return storeResult(result)
}`
	return fmt.Sprintf(template, funcName)
}

func createFunctionContent(index, size int) string {
	return fmt.Sprintf(`func function%d() error {
    // Function implementation %d
    data := getData()
    result := processData(data)
    return saveResult(result)
}`, index, size)
}

func createComplexFunctionContent(index int) string {
	return fmt.Sprintf(`func function%d(data interface{}) error {
    if data == nil {
        return errors.New("data cannot be nil")
    }

    // Validate the data
    if err := validate(data); err != nil {
        return fmt.Errorf("validation failed: %%w", err)
    }

    // Transform the data
    transformed, err := transform(data)
    if err != nil {
        return fmt.Errorf("transformation failed: %%w", err)
    }

    // Persist the result
    if err := persist(transformed); err != nil {
        return fmt.Errorf("persistence failed: %%w", err)
    }

    return nil
}`, index)
}
