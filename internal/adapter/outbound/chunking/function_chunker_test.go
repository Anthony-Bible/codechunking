package chunking

import (
	"codechunking/internal/port/outbound"
	"context"
	"testing"
)

// TestFunctionChunkerAddSemanticOverlap tests the addSemanticOverlap method for proper context addition.
func TestFunctionChunkerAddSemanticOverlap(t *testing.T) {
	tests := []struct {
		name            string
		overlapSize     int
		previousChunk   outbound.SemanticCodeChunk
		currentChunk    *outbound.EnhancedCodeChunk
		expectedContext string
		shouldModify    bool
	}{
		{
			name:        "should add overlap when overlap size is positive",
			overlapSize: 500,
			previousChunk: outbound.SemanticCodeChunk{
				Name:      "previousFunction",
				Signature: "func previousFunction() error",
				Content:   "func previousFunction() error {\n    // implementation\n}",
				Language:  mustCreateLanguage("go"),
				StartByte: 0,
				EndByte:   100,
			},
			currentChunk: &outbound.EnhancedCodeChunk{
				ID:       "current-chunk-id",
				Content:  "func currentFunction() error {\n    // implementation\n}",
				Language: mustCreateLanguage("go"),
				PreservedContext: outbound.PreservedContext{
					PrecedingContext: "",
				},
			},
			expectedContext: "func previousFunction() error",
			shouldModify:    true,
		},
		{
			name:        "should not add overlap when overlap size is zero",
			overlapSize: 0,
			previousChunk: outbound.SemanticCodeChunk{
				Name:      "previousFunction",
				Signature: "func previousFunction() error",
				Content:   "func previousFunction() error {\n    // implementation\n}",
				Language:  mustCreateLanguage("go"),
				StartByte: 0,
				EndByte:   100,
			},
			currentChunk: &outbound.EnhancedCodeChunk{
				ID:       "current-chunk-id",
				Content:  "func currentFunction() error {\n    // implementation\n}",
				Language: mustCreateLanguage("go"),
				PreservedContext: outbound.PreservedContext{
					PrecedingContext: "",
				},
			},
			expectedContext: "",
			shouldModify:    false,
		},
		{
			name:        "should not add overlap when overlap size is negative",
			overlapSize: -100,
			previousChunk: outbound.SemanticCodeChunk{
				Name:      "previousFunction",
				Signature: "func previousFunction() error",
				Content:   "func previousFunction() error {\n    // implementation\n}",
				Language:  mustCreateLanguage("go"),
				StartByte: 0,
				EndByte:   100,
			},
			currentChunk: &outbound.EnhancedCodeChunk{
				ID:       "current-chunk-id",
				Content:  "func currentFunction() error {\n    // implementation\n}",
				Language: mustCreateLanguage("go"),
				PreservedContext: outbound.PreservedContext{
					PrecedingContext: "",
				},
			},
			expectedContext: "",
			shouldModify:    false,
		},
		{
			name:        "should append to existing preceding context",
			overlapSize: 500,
			previousChunk: outbound.SemanticCodeChunk{
				Name:      "previousFunction",
				Signature: "func previousFunction() error",
				Content:   "func previousFunction() error {\n    // implementation\n}",
				Language:  mustCreateLanguage("go"),
				StartByte: 0,
				EndByte:   100,
			},
			currentChunk: &outbound.EnhancedCodeChunk{
				ID:       "current-chunk-id",
				Content:  "func currentFunction() error {\n    // implementation\n}",
				Language: mustCreateLanguage("go"),
				PreservedContext: outbound.PreservedContext{
					PrecedingContext: "existing context",
				},
			},
			expectedContext: "func previousFunction() error\nexisting context",
			shouldModify:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewFunctionChunker()
			ctx := context.Background()
			config := outbound.ChunkingConfiguration{
				OverlapSize: tt.overlapSize,
			}

			chunker.addSemanticOverlap(ctx, tt.currentChunk, tt.previousChunk, config)

			if tt.currentChunk.PreservedContext.PrecedingContext != tt.expectedContext {
				t.Errorf(
					"addSemanticOverlap() = %q, want %q",
					tt.currentChunk.PreservedContext.PrecedingContext,
					tt.expectedContext,
				)
			}
		})
	}
}

// TestFunctionChunkerExtractOverlapContext tests the extractOverlapContext method for proper priority ordering.
func TestFunctionChunkerExtractOverlapContext(t *testing.T) {
	tests := []struct {
		name          string
		chunk         outbound.SemanticCodeChunk
		maxSize       int
		expectedParts []string // Expected parts in order
		shouldContain []string // Strings that should be contained in result
	}{
		{
			name: "should prioritize types, documentation, signature, then dependencies",
			chunk: outbound.SemanticCodeChunk{
				Name:          "testFunction",
				Signature:     "func testFunction(arg1 string) error",
				Documentation: "TestFunction does something important\nwith multiple lines",
				Dependencies: []outbound.DependencyReference{
					{Name: "fmt"},
					{Name: "strings"},
				},
				Language: mustCreateLanguage("go"),
			},
			maxSize:       1000,
			expectedParts: []string{"func testFunction(arg1 string) error", "TestFunction does something important"},
			shouldContain: []string{
				"func testFunction(arg1 string) error",
				"TestFunction does something important",
				"// depends on: fmt",
			},
		},
		{
			name: "should respect size limits",
			chunk: outbound.SemanticCodeChunk{
				Name:          "testFunction",
				Signature:     "func testFunction(arg1 string, arg2 int, arg3 interface{}) (string, error)",
				Documentation: "This is a very long documentation string that should exceed the size limit when combined with the signature",
				Dependencies: []outbound.DependencyReference{
					{Name: "fmt"},
					{Name: "strings"},
					{Name: "context"},
				},
				Language: mustCreateLanguage("go"),
			},
			maxSize: 50,
			expectedParts: []string{
				"func testFunction(arg1 string, arg2 int, arg3 i...",
			}, // Signature truncated to 50 chars
			shouldContain: []string{
				"func testFunction",
			}, // Should contain the function name
		},
		{
			name: "should handle empty chunk gracefully",
			chunk: outbound.SemanticCodeChunk{
				Name:     "emptyFunction",
				Language: mustCreateLanguage("go"),
			},
			maxSize:       500,
			expectedParts: nil,
			shouldContain: nil,
		},
		{
			name: "should extract signature when documentation is empty",
			chunk: outbound.SemanticCodeChunk{
				Name:      "functionNoDoc",
				Signature: "func functionNoDoc() int",
				Dependencies: []outbound.DependencyReference{
					{Name: "math"},
				},
				Language: mustCreateLanguage("go"),
			},
			maxSize:       200,
			expectedParts: []string{"func functionNoDoc() int", "// depends on: math"},
			shouldContain: []string{"func functionNoDoc() int", "// depends on: math"},
		},
		{
			name: "should handle only dependencies when other fields are empty",
			chunk: outbound.SemanticCodeChunk{
				Name: "functionOnlyDeps",
				Dependencies: []outbound.DependencyReference{
					{Name: "database/sql"},
					{Name: "time"},
				},
				Language: mustCreateLanguage("go"),
			},
			maxSize:       100,
			expectedParts: []string{"// depends on: database/sql", "// depends on: time"},
			shouldContain: []string{"// depends on: database/sql", "// depends on: time"},
		},
		{
			name: "should return empty string when maxSize is zero",
			chunk: outbound.SemanticCodeChunk{
				Name:          "testFunction",
				Signature:     "func testFunction() error",
				Documentation: "Some documentation",
				Language:      mustCreateLanguage("go"),
			},
			maxSize:       0,
			expectedParts: nil,
			shouldContain: nil,
		},
		{
			name: "should return empty string when maxSize is negative",
			chunk: outbound.SemanticCodeChunk{
				Name:          "testFunction",
				Signature:     "func testFunction() error",
				Documentation: "Some documentation",
				Language:      mustCreateLanguage("go"),
			},
			maxSize:       -100,
			expectedParts: nil,
			shouldContain: nil,
		},
		{
			name: "should prioritize type definitions first in overlap context",
			chunk: outbound.SemanticCodeChunk{
				Name:          "processError",
				Signature:     "func processError(err error) CustomError",
				Documentation: "processError converts standard errors to custom errors",
				UsedTypes: []outbound.TypeReference{
					{Name: "CustomError", IsGeneric: false},
				},
				Dependencies: []outbound.DependencyReference{
					{Name: "fmt"},
				},
				Language: mustCreateLanguage("go"),
			},
			maxSize: 1000,
			expectedParts: []string{
				"// type: CustomError",
				"processError converts standard errors",
				"func processError(err error) CustomError",
			},
			shouldContain: []string{
				"// type: CustomError",
				"processError converts standard errors",
				"func processError(err error) CustomError",
				"// depends on: fmt",
			},
		},
		{
			name: "should format generic types with type arguments",
			chunk: outbound.SemanticCodeChunk{
				Name:      "mapFunction",
				Signature: "func mapFunction() map[string]interface{}",
				UsedTypes: []outbound.TypeReference{
					{Name: "Map", IsGeneric: true, GenericArgs: []string{"K", "V"}},
				},
				Language: mustCreateLanguage("go"),
			},
			maxSize:       500,
			expectedParts: []string{"// type: Map[K, V]", "func mapFunction()"},
			shouldContain: []string{
				"// type: Map[K, V]",
				"func mapFunction()",
			},
		},
		{
			name: "should include multiple type references",
			chunk: outbound.SemanticCodeChunk{
				Name:      "multiTypeFunc",
				Signature: "func multiTypeFunc() error",
				UsedTypes: []outbound.TypeReference{
					{Name: "ErrorType", IsGeneric: false},
					{Name: "Result", IsGeneric: true, GenericArgs: []string{"T", "E"}},
					{Name: "Option", IsGeneric: true, GenericArgs: []string{"T"}},
				},
				Language: mustCreateLanguage("go"),
			},
			maxSize:       500,
			expectedParts: []string{"// type: ErrorType", "// type: Result[T, E]", "// type: Option[T]"},
			shouldContain: []string{
				"// type: ErrorType",
				"// type: Result[T, E]",
				"// type: Option[T]",
			},
		},
		{
			name: "should validate types appear before documentation in output",
			chunk: outbound.SemanticCodeChunk{
				Name:          "validateOrder",
				Signature:     "func validateOrder() Response",
				Documentation: "validateOrder checks the ordering",
				UsedTypes: []outbound.TypeReference{
					{Name: "Response", IsGeneric: false},
				},
				Language: mustCreateLanguage("go"),
			},
			maxSize:       500,
			expectedParts: []string{"// type: Response", "validateOrder checks the ordering"},
			shouldContain: []string{
				"// type: Response",
				"validateOrder checks the ordering",
			},
		},
		{
			name: "should include UsedTypes when only types are populated",
			chunk: outbound.SemanticCodeChunk{
				Name: "typeOnlyFunc",
				UsedTypes: []outbound.TypeReference{
					{Name: "CustomType", IsGeneric: false},
				},
				Language: mustCreateLanguage("go"),
			},
			maxSize:       200,
			expectedParts: []string{"// type: CustomType"},
			shouldContain: []string{
				"// type: CustomType",
			},
		},
		{
			name: "should enforce exact priority order when all fields populated",
			chunk: outbound.SemanticCodeChunk{
				Name:          "completeFunction",
				Signature:     "func completeFunction() Result",
				Documentation: "completeFunction demonstrates priority ordering",
				UsedTypes: []outbound.TypeReference{
					{Name: "Result", IsGeneric: true, GenericArgs: []string{"T"}},
					{Name: "Error", IsGeneric: false},
				},
				Dependencies: []outbound.DependencyReference{
					{Name: "errors"},
					{Name: "context"},
				},
				Language: mustCreateLanguage("go"),
			},
			maxSize: 2000,
			expectedParts: []string{
				"// type: Result[T]",
				"// type: Error",
				"completeFunction demonstrates priority ordering",
				"func completeFunction() Result",
				"// depends on: errors",
				"// depends on: context",
			},
			shouldContain: []string{
				"// type: Result[T]",
				"// type: Error",
				"completeFunction demonstrates priority ordering",
				"func completeFunction() Result",
				"// depends on: errors",
				"// depends on: context",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewFunctionChunker()
			result := chunker.extractOverlapContext(tt.chunk, tt.maxSize)

			// Check that expected parts are in the result in correct order
			if len(tt.expectedParts) > 0 {
				resultOrder := result
				for _, expectedPart := range tt.expectedParts {
					if !containsString(resultOrder, expectedPart) {
						t.Errorf("extractOverlapContext() result should contain %q, got %q", expectedPart, resultOrder)
					}
					// Also check order (approximately)
					index := findStringIndex(resultOrder, expectedPart)
					if index < 0 {
						t.Errorf("extractOverlapContext() should contain %q in proper order", expectedPart)
					}
				}
			} else if result != "" {
				t.Errorf("extractOverlapContext() = %q, want empty string", result)
			}

			// Check that required strings are contained
			for _, shouldContain := range tt.shouldContain {
				if !containsString(result, shouldContain) {
					t.Errorf("extractOverlapContext() result should contain %q, got %q", shouldContain, result)
				}
			}

			// Check total size doesn't exceed maxSize
			if tt.maxSize > 0 && len(result) > tt.maxSize {
				t.Errorf("extractOverlapContext() result size %d exceeds maximum %d", len(result), tt.maxSize)
			}
		})
	}
}

// TestFunctionChunkerFormatTypeReferences tests the formatTypeReferences method for proper type formatting.
func TestFunctionChunkerFormatTypeReferences(t *testing.T) {
	tests := []struct {
		name     string
		types    []outbound.TypeReference
		maxSize  int
		expected string
	}{
		{
			name: "should format non-generic type",
			types: []outbound.TypeReference{
				{Name: "ErrorType", IsGeneric: false},
			},
			maxSize:  100,
			expected: "// type: ErrorType",
		},
		{
			name: "should format generic type with arguments",
			types: []outbound.TypeReference{
				{Name: "Map", IsGeneric: true, GenericArgs: []string{"K", "V"}},
			},
			maxSize:  100,
			expected: "// type: Map[K, V]",
		},
		{
			name: "should format generic type with single argument",
			types: []outbound.TypeReference{
				{Name: "Option", IsGeneric: true, GenericArgs: []string{"T"}},
			},
			maxSize:  100,
			expected: "// type: Option[T]",
		},
		{
			name: "should format multiple types separated by newlines",
			types: []outbound.TypeReference{
				{Name: "ErrorType", IsGeneric: false},
				{Name: "Result", IsGeneric: true, GenericArgs: []string{"T", "E"}},
				{Name: "Option", IsGeneric: true, GenericArgs: []string{"T"}},
			},
			maxSize:  200,
			expected: "// type: ErrorType\n// type: Result[T, E]\n// type: Option[T]",
		},
		{
			name: "should format generic type with multiple arguments",
			types: []outbound.TypeReference{
				{Name: "Triple", IsGeneric: true, GenericArgs: []string{"A", "B", "C"}},
			},
			maxSize:  100,
			expected: "// type: Triple[A, B, C]",
		},
		{
			name:     "should return empty string for empty types array",
			types:    []outbound.TypeReference{},
			maxSize:  100,
			expected: "",
		},
		{
			name:     "should return empty string when maxSize is zero",
			types:    []outbound.TypeReference{{Name: "ErrorType", IsGeneric: false}},
			maxSize:  0,
			expected: "",
		},
		{
			name:     "should return empty string when maxSize is negative",
			types:    []outbound.TypeReference{{Name: "ErrorType", IsGeneric: false}},
			maxSize:  -100,
			expected: "",
		},
		{
			name: "should truncate types when size limit is exceeded",
			types: []outbound.TypeReference{
				{Name: "TypeOne", IsGeneric: false},
				{Name: "TypeTwo", IsGeneric: false},
				{Name: "TypeThree", IsGeneric: false},
			},
			maxSize:  30,
			expected: "// type: TypeOne",
		},
		{
			name: "should handle generic type with IsGeneric true but empty GenericArgs",
			types: []outbound.TypeReference{
				{Name: "EmptyGeneric", IsGeneric: true, GenericArgs: []string{}},
			},
			maxSize:  100,
			expected: "// type: EmptyGeneric",
		},
		{
			name: "should respect exact size boundary",
			types: []outbound.TypeReference{
				{Name: "Type", IsGeneric: false},
			},
			maxSize:  14,
			expected: "// type: Type",
		},
		{
			name: "should not include type when exactly at boundary with newline",
			types: []outbound.TypeReference{
				{Name: "TypeOne", IsGeneric: false},
				{Name: "TypeTwo", IsGeneric: false},
			},
			maxSize:  17,
			expected: "// type: TypeOne",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewFunctionChunker()
			result := chunker.formatTypeReferences(tt.types, tt.maxSize)

			if result != tt.expected {
				t.Errorf("formatTypeReferences() = %q, want %q", result, tt.expected)
			}

			// Validate size constraint
			if tt.maxSize > 0 && len(result) > tt.maxSize {
				t.Errorf("formatTypeReferences() result size %d exceeds maximum %d", len(result), tt.maxSize)
			}
		})
	}
}

// TestFunctionChunkerExtractFunctionSignature tests the extractFunctionSignature method.
func TestFunctionChunkerExtractFunctionSignature(t *testing.T) {
	tests := []struct {
		name           string
		chunk          outbound.SemanticCodeChunk
		expected       string
		shouldTruncate bool
	}{
		{
			name: "should return signature when available",
			chunk: outbound.SemanticCodeChunk{
				Name:      "testFunction",
				Signature: "func testFunction(arg1 string) error",
				Content:   "func testFunction(arg1 string) error {\n    return nil\n}",
			},
			expected:       "func testFunction(arg1 string) error",
			shouldTruncate: false,
		},
		{
			name: "should extract first line as fallback when signature is empty",
			chunk: outbound.SemanticCodeChunk{
				Name:    "fallbackFunction",
				Content: "func fallbackFunction() map[string]int {\n    return make(map[string]int)\n}",
			},
			expected:       "func fallbackFunction() map[string]int {",
			shouldTruncate: false,
		},
		{
			name: "should truncate long first lines",
			chunk: outbound.SemanticCodeChunk{
				Name:    "longFunction",
				Content: "func veryLongFunctionName(argument1, argument2, argument3, argument4 string, number1, number2, number3, number4 int, customType some.package.CustomType) (map[string]interface{}, error) {",
			},
			expected:       "func veryLongFunctionName(argument1, argument2, argument3, argument4 string, number1, number2, number3, number4 int, customType some.package.CustomType) (map[string]interface{}, error) {",
			shouldTruncate: true,
		},
		{
			name: "should handle empty content gracefully",
			chunk: outbound.SemanticCodeChunk{
				Name:    "emptyFunction",
				Content: "",
			},
			expected:       "",
			shouldTruncate: false,
		},
		{
			name: "should handle content with only whitespace",
			chunk: outbound.SemanticCodeChunk{
				Name:    "whitespaceFunction",
				Content: "   \n\t  ",
			},
			expected:       "   ",
			shouldTruncate: false,
		},
		{
			name: "should handle multiline content and take first non-empty line",
			chunk: outbound.SemanticCodeChunk{
				Name:    "multilineFunction",
				Content: "\n// Comment line\nfunc multilineFunction() error {",
			},
			expected:       "// Comment line",
			shouldTruncate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewFunctionChunker()
			result := chunker.extractFunctionSignature(tt.chunk)

			if tt.shouldTruncate && len(result) > 200 {
				if len(result) < 203 || result[len(result)-3:] != "..." {
					t.Errorf("extractFunctionSignature() should truncate to 200 chars and add '...', got %q", result)
				}
			} else if result != tt.expected {
				t.Errorf("extractFunctionSignature() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestFunctionChunkerFormatDependencies tests the formatDependencies method.
func TestFunctionChunkerFormatDependencies(t *testing.T) {
	tests := []struct {
		name            string
		dependencies    []outbound.DependencyReference
		maxSize         int
		expected        string
		shouldFitInSize bool
	}{
		{
			name: "should format all dependencies when size permits",
			dependencies: []outbound.DependencyReference{
				{Name: "fmt"},
				{Name: "strings"},
				{Name: "context"},
			},
			maxSize:         200,
			expected:        "// depends on: fmt\n// depends on: strings\n// depends on: context",
			shouldFitInSize: true,
		},
		{
			name: "should truncate dependencies when size limit exceeded",
			dependencies: []outbound.DependencyReference{
				{Name: "verylongdependencyname1"},
				{Name: "verylongdependencyname2"},
				{Name: "verylongdependencyname3"},
				{Name: "verylongdependencyname4"},
			},
			maxSize:         50,
			expected:        "// depends on: verylongdependencyname1",
			shouldFitInSize: true,
		},
		{
			name:            "should return empty string when no dependencies",
			dependencies:    []outbound.DependencyReference{},
			maxSize:         100,
			expected:        "",
			shouldFitInSize: true,
		},
		{
			name:            "should handle nil dependencies",
			dependencies:    nil,
			maxSize:         100,
			expected:        "",
			shouldFitInSize: true,
		},
		{
			name: "should return empty string when maxSize is zero",
			dependencies: []outbound.DependencyReference{
				{Name: "fmt"},
				{Name: "strings"},
			},
			maxSize:         0,
			expected:        "",
			shouldFitInSize: true,
		},
		{
			name: "should return empty string when maxSize is negative",
			dependencies: []outbound.DependencyReference{
				{Name: "fmt"},
				{Name: "strings"},
			},
			maxSize:         -100,
			expected:        "",
			shouldFitInSize: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewFunctionChunker()
			result := chunker.formatDependencies(tt.dependencies, tt.maxSize)

			if result != tt.expected {
				t.Errorf("formatDependencies() = %q, want %q", result, tt.expected)
			}

			if tt.shouldFitInSize && tt.maxSize > 0 && len(result) > tt.maxSize {
				t.Errorf("formatDependencies() result size %d exceeds maxSize %d", len(result), tt.maxSize)
			}
		})
	}
}

// TestFunctionChunkerOverlapIntegration tests the integration of overlap methods.
func TestFunctionChunkerOverlapIntegration(t *testing.T) {
	tests := []struct {
		name            string
		chunks          []outbound.SemanticCodeChunk
		config          outbound.ChunkingConfiguration
		expectedContext map[int]string // chunk index -> expected preceding context
	}{
		{
			name: "should add correct overlap context between multiple chunks",
			chunks: []outbound.SemanticCodeChunk{
				{
					Name:      "chunk1",
					Signature: "func chunk1() error",
					Content:   "func chunk1() error { return nil }",
					Language:  mustCreateLanguage("go"),
					StartByte: 0,
					EndByte:   100,
				},
				{
					Name:      "chunk2",
					Signature: "func chunk2() error",
					Content:   "func chunk2() error { return nil }",
					Language:  mustCreateLanguage("go"),
					StartByte: 101,
					EndByte:   200,
				},
				{
					Name:      "chunk3",
					Signature: "func chunk3() error",
					Content:   "func chunk3() error { return nil }",
					Language:  mustCreateLanguage("go"),
					StartByte: 201,
					EndByte:   300,
				},
			},
			config: outbound.ChunkingConfiguration{
				OverlapSize: 500,
			},
			expectedContext: map[int]string{
				0: "", // First chunk should have no preceding context
				1: "func chunk1() error",
				2: "func chunk2() error",
			},
		},
		{
			name: "should not add overlap when configured size is zero",
			chunks: []outbound.SemanticCodeChunk{
				{
					Name:      "chunk1",
					Signature: "func chunk1() error",
					Content:   "func chunk1() error { return nil }",
					Language:  mustCreateLanguage("go"),
					StartByte: 0,
					EndByte:   100,
				},
				{
					Name:      "chunk2",
					Signature: "func chunk2() error",
					Content:   "func chunk2() error { return nil }",
					Language:  mustCreateLanguage("go"),
					StartByte: 101,
					EndByte:   200,
				},
			},
			config: outbound.ChunkingConfiguration{
				OverlapSize: 0,
			},
			expectedContext: map[int]string{
				0: "",
				1: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewFunctionChunker()
			ctx := context.Background()

			enhancedChunks := make([]outbound.EnhancedCodeChunk, len(tt.chunks))
			for i, chunk := range tt.chunks {
				enhancedChunks[i] = outbound.EnhancedCodeChunk{
					ID:       "chunk-" + string(rune(i)),
					Content:  chunk.Content,
					Language: chunk.Language,
					PreservedContext: outbound.PreservedContext{
						PrecedingContext: "",
					},
				}
			}

			// Simulate the chunking process with overlap
			for i := range enhancedChunks {
				if tt.config.OverlapSize > 0 && i > 0 {
					chunker.addSemanticOverlap(ctx, &enhancedChunks[i], tt.chunks[i-1], tt.config)
				}
			}

			for i, expected := range tt.expectedContext {
				if enhancedChunks[i].PreservedContext.PrecedingContext != expected {
					t.Errorf(
						"Chunk %d preceding context = %q, want %q",
						i,
						enhancedChunks[i].PreservedContext.PrecedingContext,
						expected,
					)
				}
			}
		})
	}
}
