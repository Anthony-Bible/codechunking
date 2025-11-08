package pythonparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// RED PHASE: Integration Tests for Chunked Parsing
// =============================================================================
// These tests define the expected end-to-end behavior when parsing extremely
// large Python files that would normally crash tree-sitter.
//
// PROBLEM: Tree-sitter's Python parser crashes with 'Assertion length <= 1024 failed'
// when parsing files with 50k+ functions due to external scanner buffer overflow.
//
// SOLUTION: The Python parser should automatically detect large files, chunk them,
// parse each chunk separately, and merge the results transparently.
// =============================================================================

// TestPythonParser_LargeFileChunking tests end-to-end parsing of extremely large files.
// RED PHASE: This test will fail because the chunking integration doesn't exist yet.
//
// EXPECTED BEHAVIOR:
// - Parse a file with 10000 functions without crashing
// - All 10000 functions should be extracted correctly
// - The parser should transparently use chunking under the hood
// - No observable difference in output compared to non-chunked parsing.
func TestPythonParser_LargeFileChunking(t *testing.T) {
	t.Run("parse 10000 functions without crash", func(t *testing.T) {
		ctx := context.Background()

		// Generate Python file with 10000 functions
		var sb strings.Builder
		sb.WriteString("# Large Python file stress test\n")
		sb.WriteString("# This file contains 10000 functions\n\n")

		for i := range 10000 {
			// Format function name with padding
			funcNum := i + 1
			sb.WriteString("def function_")
			sb.WriteString(paddedNumber(funcNum, 5))
			sb.WriteString("():\n")
			sb.WriteString("    \"\"\"Function number ")
			sb.WriteString(paddedNumber(funcNum, 5))
			sb.WriteString(".\"\"\"\n")
			sb.WriteString("    return ")
			sb.WriteString(paddedNumber(funcNum, 5))
			sb.WriteString("\n\n")
		}

		sourceCode := sb.String()
		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		// Create parse tree from source
		parseTree := createMockParseTreeFromSource(t, language, sourceCode)

		// Create parser
		parser, err := NewPythonParser()
		require.NoError(t, err)

		// Extract functions with IncludePrivate: true
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:       true,
			IncludeDocumentation: true,
			IncludeTypeInfo:      true,
			MaxDepth:             10,
		}

		// This should NOT crash (RED PHASE: will crash without chunking)
		functions, err := parser.ExtractFunctions(ctx, parseTree, options)
		require.NoError(t, err, "Parsing 10000 functions should not crash")

		// Verify all 10000 functions were extracted
		assert.Len(t, functions, 10000, "Should extract all 10000 functions")

		// Verify first function
		firstFunc := functions[0]
		assert.Equal(t, "function_00001", firstFunc.Name)
		assert.Contains(t, firstFunc.Documentation, "Function number 00001")

		// Verify middle function
		middleFunc := functions[5000]
		assert.Equal(t, "function_05001", middleFunc.Name)

		// Verify last function
		lastFunc := functions[9999]
		assert.Equal(t, "function_10000", lastFunc.Name)
		assert.Contains(t, lastFunc.Documentation, "Function number 10000")
	})

	t.Run("parse 5000 classes with methods", func(t *testing.T) {
		ctx := context.Background()

		// Generate Python file with 5000 classes, each with 2 methods
		var sb strings.Builder
		sb.WriteString("# Large Python class file\n\n")

		for i := range 5000 {
			classNum := i + 1
			sb.WriteString("class Class")
			sb.WriteString(paddedNumber(classNum, 4))
			sb.WriteString(":\n")
			sb.WriteString("    \"\"\"Class number ")
			sb.WriteString(paddedNumber(classNum, 4))
			sb.WriteString(".\"\"\"\n\n")
			sb.WriteString("    def method_one(self):\n")
			sb.WriteString("        pass\n\n")
			sb.WriteString("    def method_two(self):\n")
			sb.WriteString("        pass\n\n")
		}

		sourceCode := sb.String()
		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewPythonParser()
		require.NoError(t, err)

		options := outbound.SemanticExtractionOptions{
			IncludePrivate:       true,
			IncludeDocumentation: true,
			MaxDepth:             10,
		}

		// Extract classes (should not crash)
		classes, err := parser.ExtractClasses(ctx, parseTree, options)
		require.NoError(t, err, "Parsing 5000 classes should not crash")

		// Verify all 5000 classes extracted
		assert.Len(t, classes, 5000, "Should extract all 5000 classes")

		// Verify first class
		assert.Equal(t, "Class0001", classes[0].Name)

		// Extract methods (10000 total: 5000 classes * 2 methods each)
		functions, err := parser.ExtractFunctions(ctx, parseTree, options)
		require.NoError(t, err)

		// Should have 10000 methods
		assert.Len(t, functions, 10000, "Should extract all 10000 methods")
	})

	t.Run("handles extremely long function names", func(t *testing.T) {
		ctx := context.Background()

		// Generate file with very long function names (buffer overflow risk)
		var sb strings.Builder
		sb.WriteString("# File with long identifiers\n\n")

		for i := range 1000 {
			// Very long function name (200 characters)
			sb.WriteString("def ")
			sb.WriteString(strings.Repeat("very_long_function_name_with_descriptive_suffix_", 4))
			sb.WriteString("_num_")
			sb.WriteString(paddedNumber(i+1, 4))
			sb.WriteString("():\n")
			sb.WriteString("    pass\n\n")
		}

		sourceCode := sb.String()
		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewPythonParser()
		require.NoError(t, err)

		options := outbound.SemanticExtractionOptions{
			IncludePrivate: true,
		}

		// Should handle long names without buffer overflow
		functions, err := parser.ExtractFunctions(ctx, parseTree, options)
		require.NoError(t, err, "Should handle long function names")
		assert.Len(t, functions, 1000)
	})
}

// TestPythonParser_ChunkBoundaryCorrectness tests correctness at chunk boundaries.
// RED PHASE: This test will fail because boundary handling doesn't exist yet.
//
// EXPECTED BEHAVIOR:
// - When a class definition spans chunk boundary, entire class is in one chunk
// - All methods of a class are correctly associated with that class
// - Qualified names are correct even across chunk boundaries
// - No data loss at boundaries.
func TestPythonParser_ChunkBoundaryCorrectness(t *testing.T) {
	t.Run("class spanning potential chunk boundary", func(t *testing.T) {
		ctx := context.Background()

		// Create file where a large class might span a chunk boundary
		var sb strings.Builder

		// Add 2000 small functions first (force chunk 1)
		for i := range 2000 {
			sb.WriteString("def small_func_")
			sb.WriteString(paddedNumber(i+1, 4))
			sb.WriteString("():\n    pass\n\n")
		}

		// Add a large class with many methods (might be at boundary)
		sb.WriteString("class LargeClassAtBoundary:\n")
		sb.WriteString("    \"\"\"A large class that might span chunk boundary.\"\"\"\n\n")

		for i := range 100 {
			sb.WriteString("    def method_")
			sb.WriteString(paddedNumber(i+1, 3))
			sb.WriteString("(self):\n")
			sb.WriteString("        \"\"\"Method ")
			sb.WriteString(paddedNumber(i+1, 3))
			sb.WriteString(".\"\"\"\n")
			sb.WriteString("        pass\n\n")
		}

		// Add more functions after
		for i := range 1000 {
			sb.WriteString("def after_func_")
			sb.WriteString(paddedNumber(i+1, 4))
			sb.WriteString("():\n    pass\n\n")
		}

		sourceCode := sb.String()
		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewPythonParser()
		require.NoError(t, err)

		options := outbound.SemanticExtractionOptions{
			IncludePrivate:       true,
			IncludeDocumentation: true,
			MaxDepth:             10,
		}

		// Extract classes
		classes, err := parser.ExtractClasses(ctx, parseTree, options)
		require.NoError(t, err)

		// Find the LargeClassAtBoundary
		var largeClass *outbound.SemanticCodeChunk
		for i := range classes {
			if classes[i].Name == "LargeClassAtBoundary" {
				largeClass = &classes[i]
				break
			}
		}
		require.NotNil(t, largeClass, "Should find LargeClassAtBoundary")

		// Verify class has correct documentation
		assert.Contains(t, largeClass.Documentation,
			"large class that might span chunk boundary")

		// Extract functions (includes methods)
		functions, err := parser.ExtractFunctions(ctx, parseTree, options)
		require.NoError(t, err)

		// Count methods that belong to LargeClassAtBoundary
		methodCount := 0
		for _, fn := range functions {
			if strings.HasPrefix(fn.QualifiedName, "LargeClassAtBoundary.") {
				methodCount++

				// Verify qualified name is correct
				assert.Contains(t, fn.QualifiedName, "LargeClassAtBoundary.method_")

				// Verify it's marked as a method, not a function
				assert.Equal(t, outbound.ConstructMethod, fn.Type,
					"Class methods should be marked as ConstructMethod")
			}
		}

		// Should have all 100 methods
		assert.Equal(t, 100, methodCount,
			"Should extract all 100 methods from LargeClassAtBoundary")
	})

	t.Run("nested class at chunk boundary", func(t *testing.T) {
		ctx := context.Background()

		var sb strings.Builder

		// Add functions to fill first chunk
		for i := range 3000 {
			sb.WriteString("def func_")
			sb.WriteString(paddedNumber(i+1, 4))
			sb.WriteString("():\n    pass\n\n")
		}

		// Add nested class structure
		sb.WriteString("class OuterClass:\n")
		sb.WriteString("    \"\"\"Outer class.\"\"\"\n\n")
		sb.WriteString("    class InnerClass:\n")
		sb.WriteString("        \"\"\"Inner class.\"\"\"\n\n")
		sb.WriteString("        def inner_method(self):\n")
		sb.WriteString("            pass\n\n")
		sb.WriteString("    def outer_method(self):\n")
		sb.WriteString("        pass\n\n")

		sourceCode := sb.String()
		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewPythonParser()
		require.NoError(t, err)

		options := outbound.SemanticExtractionOptions{
			IncludePrivate: true,
			MaxDepth:       10,
		}

		// Extract classes
		classes, err := parser.ExtractClasses(ctx, parseTree, options)
		require.NoError(t, err)

		// Should find both OuterClass and InnerClass
		classNames := make(map[string]bool)
		for _, cls := range classes {
			classNames[cls.Name] = true
		}

		assert.True(t, classNames["OuterClass"], "Should find OuterClass")
		assert.True(t, classNames["InnerClass"], "Should find InnerClass")

		// Extract functions
		functions, err := parser.ExtractFunctions(ctx, parseTree, options)
		require.NoError(t, err)

		// Verify qualified names
		qualifiedNames := make(map[string]bool)
		for _, fn := range functions {
			if strings.Contains(fn.QualifiedName, "Class") {
				qualifiedNames[fn.QualifiedName] = true
			}
		}

		// Should have correctly qualified names for nested structure
		// Note: exact qualified name format depends on implementation
		foundOuterMethod := false
		foundInnerMethod := false

		for qname := range qualifiedNames {
			if strings.Contains(qname, "outer_method") {
				foundOuterMethod = true
			}
			if strings.Contains(qname, "inner_method") {
				foundInnerMethod = true
			}
		}

		assert.True(t, foundOuterMethod, "Should find outer_method")
		assert.True(t, foundInnerMethod, "Should find inner_method")
	})
}

// TestPythonParser_BackwardCompatibility tests that normal-sized files work as before.
// RED PHASE: This test should pass initially but may reveal issues with chunking.
//
// EXPECTED BEHAVIOR:
// - Files under threshold parse normally (no chunking)
// - Performance is not degraded for normal files
// - Output is identical to non-chunked version.
func TestPythonParser_BackwardCompatibility(t *testing.T) {
	t.Run("normal file 100 functions parses normally", func(t *testing.T) {
		ctx := context.Background()

		// Normal-sized file
		var sb strings.Builder
		sb.WriteString("# Normal Python file\n\n")

		for i := range 100 {
			sb.WriteString("def function_")
			sb.WriteString(paddedNumber(i+1, 3))
			sb.WriteString("():\n")
			sb.WriteString("    return ")
			sb.WriteString(paddedNumber(i+1, 3))
			sb.WriteString("\n\n")
		}

		sourceCode := sb.String()
		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewPythonParser()
		require.NoError(t, err)

		options := outbound.SemanticExtractionOptions{
			IncludePrivate: true,
			MaxDepth:       10,
		}

		// Parse normally
		functions, err := parser.ExtractFunctions(ctx, parseTree, options)
		require.NoError(t, err)

		// Should extract all 100 functions
		assert.Len(t, functions, 100, "Should extract all 100 functions")

		// Verify structure is correct
		for i, fn := range functions {
			expectedName := "function_" + paddedNumber(i+1, 3)
			assert.Equal(t, expectedName, fn.Name,
				"Function %d should have correct name", i)
		}
	})

	t.Run("small file performance not degraded", func(t *testing.T) {
		ctx := context.Background()

		// Very small file
		sourceCode := `
def simple_function():
    """A simple function."""
    return 42

class SimpleClass:
    """A simple class."""

    def method(self):
        pass
`

		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewPythonParser()
		require.NoError(t, err)

		options := outbound.SemanticExtractionOptions{
			IncludePrivate:       true,
			IncludeDocumentation: true,
			MaxDepth:             10,
		}

		// Should parse without any chunking overhead
		functions, err := parser.ExtractFunctions(ctx, parseTree, options)
		require.NoError(t, err)
		assert.Len(t, functions, 2) // simple_function + method

		classes, err := parser.ExtractClasses(ctx, parseTree, options)
		require.NoError(t, err)
		assert.Len(t, classes, 1) // SimpleClass
	})

	t.Run("empty file returns empty results", func(t *testing.T) {
		ctx := context.Background()

		sourceCode := "# Empty Python file\n"

		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewPythonParser()
		require.NoError(t, err)

		options := outbound.SemanticExtractionOptions{
			IncludePrivate: true,
		}

		functions, err := parser.ExtractFunctions(ctx, parseTree, options)
		require.NoError(t, err)
		assert.Empty(t, functions, "Empty file should return no functions")

		classes, err := parser.ExtractClasses(ctx, parseTree, options)
		require.NoError(t, err)
		assert.Empty(t, classes, "Empty file should return no classes")
	})

	t.Run("file with only imports and comments", func(t *testing.T) {
		ctx := context.Background()

		sourceCode := `#!/usr/bin/env python3
"""Module with only imports."""

import os
import sys
from typing import List, Dict

# This module only has imports
`

		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewPythonParser()
		require.NoError(t, err)

		options := outbound.SemanticExtractionOptions{
			IncludePrivate: true,
		}

		// Should not crash
		functions, err := parser.ExtractFunctions(ctx, parseTree, options)
		require.NoError(t, err)
		assert.Empty(t, functions)

		// Should extract imports
		imports, err := parser.ExtractImports(ctx, parseTree, options)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(imports), 3, "Should extract imports")
	})
}

// =============================================================================
// Helper Functions
// =============================================================================

// paddedNumber returns a zero-padded string representation of a number.
func paddedNumber(num, width int) string {
	var sb strings.Builder

	// Convert number to string manually
	var numStr string
	if num == 0 {
		numStr = "0"
	} else {
		temp := num
		digits := []rune{}
		for temp > 0 {
			digits = append([]rune{rune('0' + temp%10)}, digits...)
			temp /= 10
		}
		numStr = string(digits)
	}

	// Add padding
	for range width - len(numStr) {
		sb.WriteRune('0')
	}
	sb.WriteString(numStr)

	return sb.String()
}
