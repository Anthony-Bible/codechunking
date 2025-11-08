package pythonparser

import (
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// BenchmarkPythonParser_LargeFile benchmarks parsing extremely large Python files.
// This measures performance characteristics when processing files with tens of thousands of functions.
func BenchmarkPythonParser_LargeFile(b *testing.B) {
	source := generateLargePythonFile(10000) // 10k functions

	parserInterface, err := NewPythonParser()
	require.NoError(b, err)
	parser, ok := parserInterface.(*ObservablePythonParser)
	require.True(b, ok)

	ctx := context.Background()
	parseTree := createMockPythonParseTree(b, source)
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
		MaxDepth:       100,
	}

	b.ResetTimer()
	for range b.N {
		_, _ = parser.ExtractFunctions(ctx, parseTree, options)
	}
}

// BenchmarkPythonParser_DeeplyNestedClasses benchmarks parsing deeply nested class structures.
// This tests performance when dealing with high nesting levels.
func BenchmarkPythonParser_DeeplyNestedClasses(b *testing.B) {
	benchmarks := []struct {
		name  string
		depth int
	}{
		{"depth_10", 10},
		{"depth_50", 50},
		{"depth_100", 100},
		{"depth_500", 500},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			source := generateNestedClasses(bm.depth)

			parserInterface, err := NewPythonParser()
			require.NoError(b, err)
			parser, ok := parserInterface.(*ObservablePythonParser)
			require.True(b, ok)

			ctx := context.Background()
			parseTree := createMockPythonParseTree(b, source)
			options := outbound.SemanticExtractionOptions{
				IncludePrivate: true,
				MaxDepth:       bm.depth + 10,
			}

			b.ResetTimer()
			for range b.N {
				_, _ = parser.ExtractClasses(ctx, parseTree, options)
			}
		})
	}
}

// BenchmarkPythonParser_ManyFunctions benchmarks extraction from files with many functions.
// This tests parser scalability with large numbers of top-level constructs.
func BenchmarkPythonParser_ManyFunctions(b *testing.B) {
	benchmarks := []struct {
		name  string
		count int
	}{
		{"1k_functions", 1000},
		{"5k_functions", 5000},
		{"10k_functions", 10000},
		{"50k_functions", 50000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			source := generateManyFunctions(bm.count)

			parserInterface, err := NewPythonParser()
			require.NoError(b, err)
			parser, ok := parserInterface.(*ObservablePythonParser)
			require.True(b, ok)

			ctx := context.Background()
			parseTree := createMockPythonParseTree(b, source)
			options := outbound.SemanticExtractionOptions{
				IncludePrivate: true,
				MaxDepth:       10,
			}

			b.ResetTimer()
			for range b.N {
				_, _ = parser.ExtractFunctions(ctx, parseTree, options)
			}
		})
	}
}

// BenchmarkPythonParser_MassiveClassWithMethods benchmarks parsing a single class with many methods.
// This tests performance with large class definitions containing thousands of methods.
func BenchmarkPythonParser_MassiveClassWithMethods(b *testing.B) {
	benchmarks := []struct {
		name        string
		methodCount int
	}{
		{"100_methods", 100},
		{"1k_methods", 1000},
		{"5k_methods", 5000},
		{"10k_methods", 10000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			source := generateMassiveClass(bm.methodCount)

			parserInterface, err := NewPythonParser()
			require.NoError(b, err)
			parser, ok := parserInterface.(*ObservablePythonParser)
			require.True(b, ok)

			ctx := context.Background()
			parseTree := createMockPythonParseTree(b, source)
			options := outbound.SemanticExtractionOptions{
				IncludePrivate: true,
				MaxDepth:       10,
			}

			b.ResetTimer()
			for range b.N {
				_, _ = parser.ExtractClasses(ctx, parseTree, options)
			}
		})
	}
}

// BenchmarkPythonParser_ComplexImports benchmarks import statement extraction.
// This tests parser performance with many complex import patterns.
func BenchmarkPythonParser_ComplexImports(b *testing.B) {
	benchmarks := []struct {
		name        string
		importCount int
	}{
		{"100_imports", 100},
		{"500_imports", 500},
		{"1k_imports", 1000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			source := generateComplexImports(bm.importCount)

			parserInterface, err := NewPythonParser()
			require.NoError(b, err)
			parser, ok := parserInterface.(*ObservablePythonParser)
			require.True(b, ok)

			ctx := context.Background()
			parseTree := createMockPythonParseTree(b, source)
			options := outbound.SemanticExtractionOptions{
				IncludePrivate: true,
				MaxDepth:       10,
			}

			b.ResetTimer()
			for range b.N {
				_, _ = parser.ExtractImports(ctx, parseTree, options)
			}
		})
	}
}

// BenchmarkPythonParser_ConcurrentParsing benchmarks concurrent parser usage.
// This measures thread safety and concurrent performance characteristics.
func BenchmarkPythonParser_ConcurrentParsing(b *testing.B) {
	source := generateLargePythonFile(1000)

	parserInterface, err := NewPythonParser()
	require.NoError(b, err)
	parser, ok := parserInterface.(*ObservablePythonParser)
	require.True(b, ok)

	ctx := context.Background()
	parseTree := createMockPythonParseTree(b, source)
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
		MaxDepth:       10,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = parser.ExtractFunctions(ctx, parseTree, options)
		}
	})
}

// Helper: generateLargePythonFile generates a Python file with many functions.
func generateLargePythonFile(functionCount int) string {
	var builder strings.Builder

	for range functionCount {
		builder.WriteString("def very_long_function_name")
		// Reduced from 100 to 15 to prevent tree-sitter buffer overflow
		builder.WriteString(strings.Repeat("_x", 15))
		builder.WriteString("():\n")
		builder.WriteString("    return ")
		builder.WriteString(strings.Repeat("'data'", 50))
		builder.WriteString("\n\n")
	}

	return builder.String()
}

// Helper: generateNestedClasses generates deeply nested Python classes.
func generateNestedClasses(depth int) string {
	var builder strings.Builder

	for i := range depth {
		indent := strings.Repeat("    ", i)
		builder.WriteString(indent + "class Level")
		builder.WriteString(strings.Repeat("Deep", i%100))
		builder.WriteString(":\n")
		builder.WriteString(indent + "    pass\n")
	}

	return builder.String()
}

// Helper: generateManyFunctions generates many simple functions.
func generateManyFunctions(count int) string {
	var builder strings.Builder

	for i := range count {
		builder.WriteString("def function")
		// Reduced from 50 to 10 to prevent tree-sitter buffer overflow
		builder.WriteString(strings.Repeat("_a", 10))
		builder.WriteString("_number")
		builder.WriteString(strings.Repeat(string(rune('0'+i%10)), 10))
		builder.WriteString("():\n")
		builder.WriteString("    return ")
		builder.WriteString(strings.Repeat("'value'", 10))
		builder.WriteString("\n\n")
	}

	return builder.String()
}

// Helper: generateMassiveClass generates a class with many methods.
func generateMassiveClass(methodCount int) string {
	var builder strings.Builder
	builder.WriteString("class MassiveClass:\n")

	for i := range methodCount {
		builder.WriteString("    def method")
		// Reduced from 50 to 10 to prevent tree-sitter buffer overflow
		builder.WriteString(strings.Repeat("_x", 10))
		builder.WriteString("_number")
		builder.WriteString(strings.Repeat(string(rune('0'+i%10)), 10))
		builder.WriteString("(self):\n")
		builder.WriteString("        return ")
		builder.WriteString(strings.Repeat("'result'", 20))
		builder.WriteString("\n\n")
	}

	return builder.String()
}

// Helper: generateComplexImports generates many import statements.
func generateComplexImports(count int) string {
	var builder strings.Builder

	for i := range count {
		// Reduced from i%50 to i%10 to prevent tree-sitter buffer overflow
		moduleName := "module" + strings.Repeat("_a", i%10)
		builder.WriteString("# Module " + moduleName + "\n")

		// Import previous modules
		if i > 0 {
			prevModule := "module" + strings.Repeat("_a", (i-1)%10)
			builder.WriteString("from " + prevModule + " import *\n")
		}

		// Import next module
		if i < count-1 {
			nextModule := "module" + strings.Repeat("_a", (i+1)%10)
			builder.WriteString("import " + nextModule + "\n")
		}

		builder.WriteString("class " + moduleName + "Class:\n")
		builder.WriteString("    def process(self):\n")
		builder.WriteString("        return self.__class__.__name__\n\n")
	}

	return builder.String()
}
