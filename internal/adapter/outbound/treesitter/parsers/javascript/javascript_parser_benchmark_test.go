package javascriptparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"testing"
)

// BenchmarkJavaScriptParser_LargeFile benchmarks parser performance with very large JavaScript files.
// This benchmark measures throughput and memory allocation for processing large codebases.
func BenchmarkJavaScriptParser_LargeFile(b *testing.B) {
	// Generate a large JavaScript file (10MB+)
	source := generateLargeJavaScriptFile(100000)

	parser, err := NewJavaScriptParser()
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	ctx := context.Background()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
		MaxDepth:       100,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		parseTree := createBenchmarkParseTree(b, source)
		_, err := parser.ExtractFunctions(ctx, parseTree, options)
		if err != nil {
			b.Logf("Expected error for large file: %v", err)
		}
	}
}

// BenchmarkJavaScriptParser_DeeplyNestedStructures benchmarks deeply nested object/function structures.
// This tests parser performance with high recursion depth.
func BenchmarkJavaScriptParser_DeeplyNestedStructures(b *testing.B) {
	source := generateDeeplyNestedObjects(1000)

	parser, err := NewJavaScriptParser()
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	ctx := context.Background()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
		MaxDepth:       100,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		parseTree := createBenchmarkParseTree(b, source)
		_, err := parser.ExtractVariables(ctx, parseTree, options)
		if err != nil {
			b.Logf("Expected error for deeply nested structures: %v", err)
		}
	}
}

// BenchmarkJavaScriptParser_ManyFunctions benchmarks parsing thousands of functions.
// This tests scalability when processing files with many top-level constructs.
func BenchmarkJavaScriptParser_ManyFunctions(b *testing.B) {
	source := generateManyFunctions(50000)

	parser, err := NewJavaScriptParser()
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	ctx := context.Background()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
		MaxDepth:       100,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		parseTree := createBenchmarkParseTree(b, source)
		_, err := parser.ExtractFunctions(ctx, parseTree, options)
		if err != nil {
			b.Logf("Expected error for many functions: %v", err)
		}
	}
}

// BenchmarkJavaScriptParser_LargeClass benchmarks parsing a massive class with many methods.
// This tests performance with large class definitions.
func BenchmarkJavaScriptParser_LargeClass(b *testing.B) {
	source := generateLargeClass(10000)

	parser, err := NewJavaScriptParser()
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	ctx := context.Background()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
		MaxDepth:       100,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		parseTree := createBenchmarkParseTree(b, source)
		_, err := parser.ExtractClasses(ctx, parseTree, options)
		if err != nil {
			b.Logf("Expected error for large class: %v", err)
		}
	}
}

// BenchmarkJavaScriptParser_CircularReferences benchmarks complex circular reference patterns.
// This tests parser performance with interconnected object structures.
func BenchmarkJavaScriptParser_CircularReferences(b *testing.B) {
	source := generateCircularReferences(1000)

	parser, err := NewJavaScriptParser()
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	ctx := context.Background()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
		MaxDepth:       100,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		parseTree := createBenchmarkParseTree(b, source)
		_, err := parser.ExtractVariables(ctx, parseTree, options)
		if err != nil {
			b.Logf("Expected error for circular references: %v", err)
		}
	}
}

// Helper functions for benchmark data generation

// generateLargeJavaScriptFile generates a very large JavaScript file for benchmarking.
func generateLargeJavaScriptFile(functionCount int) string {
	var builder strings.Builder

	for range functionCount {
		builder.WriteString("function veryLongFunctionName")
		builder.WriteString(strings.Repeat("X", 100))
		builder.WriteString("() { return ")
		builder.WriteString(strings.Repeat("'data'", 50))
		builder.WriteString("; }\n")
	}

	return builder.String()
}

// generateDeeplyNestedObjects generates deeply nested object/function structures.
func generateDeeplyNestedObjects(depth int) string {
	var builder strings.Builder
	builder.WriteString("const deepObject = ")

	// Create nested levels
	for i := range depth {
		builder.WriteString("{ level")
		builder.WriteString(strings.Repeat("Deep", i%100))
		builder.WriteString(": ")
	}

	// Close all the braces
	for range depth {
		builder.WriteString("}")
	}
	builder.WriteString(";\n")

	return builder.String()
}

// generateManyFunctions generates thousands of functions for scalability testing.
func generateManyFunctions(count int) string {
	var builder strings.Builder

	for i := range count {
		builder.WriteString("function function")
		builder.WriteString(strings.Repeat("A", 50))
		builder.WriteString("Number")
		builder.WriteString(strings.Repeat(string(rune('0'+i%10)), 10))
		builder.WriteString("() {\n  return ")
		builder.WriteString(strings.Repeat("'value'", 10))
		builder.WriteString(";\n}\n")
	}

	return builder.String()
}

// generateLargeClass generates a massive class with many methods.
func generateLargeClass(methodCount int) string {
	var builder strings.Builder
	builder.WriteString("class MassiveClass {\n")

	for i := range methodCount {
		builder.WriteString("  method")
		builder.WriteString(strings.Repeat("X", 50))
		builder.WriteString("Number")
		builder.WriteString(strings.Repeat(string(rune('0'+i%10)), 10))
		builder.WriteString("() {\n")
		builder.WriteString("    return ")
		builder.WriteString(strings.Repeat("'result'", 20))
		builder.WriteString(";\n  }\n")
	}

	builder.WriteString("}\n")
	return builder.String()
}

// generateCircularReferences generates complex circular reference patterns.
func generateCircularReferences(objectCount int) string {
	var builder strings.Builder

	for i := range objectCount {
		builder.WriteString("const obj")
		builder.WriteString(strings.Repeat("A", i%50))
		builder.WriteString(" = {\n")
		builder.WriteString("  self: null,\n")
		builder.WriteString("  next: null,\n")
		builder.WriteString("  process: function() {\n")
		builder.WriteString("    if (this.self === this) {\n")
		builder.WriteString("      return this.next && this.next.process();\n")
		builder.WriteString("    }\n")
		builder.WriteString("  }\n")
		builder.WriteString("};\n")

		// Create circular references
		if i > 0 {
			prevIndex := i - 1
			builder.WriteString("obj")
			builder.WriteString(strings.Repeat("A", i%50))
			builder.WriteString(".self = obj")
			builder.WriteString(strings.Repeat("A", prevIndex%50))
			builder.WriteString(";\n")
		}
	}

	return builder.String()
}

// createBenchmarkParseTree creates a parse tree for benchmarking.
func createBenchmarkParseTree(b *testing.B, source string) *valueobject.ParseTree {
	ctx := context.Background()

	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	if err != nil {
		b.Fatalf("Failed to create language: %v", err)
	}

	rootNode := &valueobject.ParseNode{
		Type:      "program",
		StartByte: 0,
		EndByte:   uint32(len(source)),
		Children:  []*valueobject.ParseNode{},
	}

	metadata, err := valueobject.NewParseMetadata(0, "go-tree-sitter-bare", "1.0.0")
	if err != nil {
		b.Fatalf("Failed to create metadata: %v", err)
	}

	parseTree, err := valueobject.NewParseTree(ctx, jsLang, rootNode, []byte(source), metadata)
	if err != nil {
		b.Fatalf("Failed to create parse tree: %v", err)
	}

	return parseTree
}

// BenchmarkJavaScriptParser_MemoryExhaustion_ExtremelyLargeFile benchmarks parser with extremely large JavaScript files.
// This measures performance and memory allocation when processing very large codebases (10MB+).
func BenchmarkJavaScriptParser_MemoryExhaustion_ExtremelyLargeFile(b *testing.B) {
	// Generate a very large JavaScript file (10MB+)
	source := generateLargeJavaScriptFile(100000)

	parser, err := NewJavaScriptParser()
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	ctx := context.Background()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
		MaxDepth:       100,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		parseTree := createBenchmarkParseTree(b, source)
		_, err := parser.ExtractFunctions(ctx, parseTree, options)
		if err != nil {
			b.Logf("Operation resulted in error (may be expected): %v", err)
		}
	}
}

// BenchmarkJavaScriptParser_MemoryExhaustion_DeeplyNestedObjects benchmarks deeply nested object structures.
// This measures parser performance and memory usage with extreme recursion depth (1000 levels).
func BenchmarkJavaScriptParser_MemoryExhaustion_DeeplyNestedObjects(b *testing.B) {
	// Generate deeply nested object structures (1000 levels)
	source := generateDeeplyNestedObjects(1000)

	parser, err := NewJavaScriptParser()
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	ctx := context.Background()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
		MaxDepth:       100,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		parseTree := createBenchmarkParseTree(b, source)
		_, err := parser.ExtractVariables(ctx, parseTree, options)
		if err != nil {
			b.Logf("Operation resulted in error (may be expected): %v", err)
		}
	}
}

// BenchmarkJavaScriptParser_MemoryExhaustion_ThousandsOfFunctions benchmarks parser with massive function counts.
// This measures scalability when processing files with 50,000+ function definitions.
func BenchmarkJavaScriptParser_MemoryExhaustion_ThousandsOfFunctions(b *testing.B) {
	// Generate 50,000 functions
	source := generateManyFunctions(50000)

	parser, err := NewJavaScriptParser()
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	ctx := context.Background()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
		MaxDepth:       100,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		parseTree := createBenchmarkParseTree(b, source)
		_, err := parser.ExtractFunctions(ctx, parseTree, options)
		if err != nil {
			b.Logf("Operation resulted in error (may be expected): %v", err)
		}
	}
}

// BenchmarkJavaScriptParser_MemoryExhaustion_MassiveClassWithMethods benchmarks massive class definitions.
// This measures performance and memory allocation when processing classes with 10,000+ methods.
func BenchmarkJavaScriptParser_MemoryExhaustion_MassiveClassWithMethods(b *testing.B) {
	// Generate a massive class with 10,000 methods
	source := generateLargeClass(10000)

	parser, err := NewJavaScriptParser()
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	ctx := context.Background()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
		MaxDepth:       100,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		parseTree := createBenchmarkParseTree(b, source)
		_, err := parser.ExtractClasses(ctx, parseTree, options)
		if err != nil {
			b.Logf("Operation resulted in error (may be expected): %v", err)
		}
	}
}

// BenchmarkJavaScriptParser_MemoryExhaustion_CircularReferenceNightmare benchmarks complex circular reference patterns.
// This measures parser performance with 1000 interconnected objects that could cause memory issues.
func BenchmarkJavaScriptParser_MemoryExhaustion_CircularReferenceNightmare(b *testing.B) {
	// Generate complex circular references (1000 objects)
	source := generateCircularReferences(1000)

	parser, err := NewJavaScriptParser()
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	ctx := context.Background()
	options := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
		MaxDepth:       100,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		parseTree := createBenchmarkParseTree(b, source)
		_, err := parser.ExtractVariables(ctx, parseTree, options)
		if err != nil {
			b.Logf("Operation resulted in error (may be expected): %v", err)
		}
	}
}
