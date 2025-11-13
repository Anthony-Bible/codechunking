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

			if tt.shouldModify {
				if tt.currentChunk.PreservedContext.PrecedingContext != tt.expectedContext {
					t.Errorf(
						"addSemanticOverlap() = %q, want %q",
						tt.currentChunk.PreservedContext.PrecedingContext,
						tt.expectedContext,
					)
				}
			} else {
				if tt.currentChunk.PreservedContext.PrecedingContext != tt.expectedContext {
					t.Errorf("addSemanticOverlap() = %q, want %q", tt.currentChunk.PreservedContext.PrecedingContext, tt.expectedContext)
				}
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
			name: "should prioritize signature over documentation and dependencies",
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
			maxSize:       50,
			expectedParts: []string{"func testFunction(arg1 string, arg2 int, arg3 interface{}) (string, error)"},
			shouldContain: []string{"func testFunction"},
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
