package chunking

import (
	"codechunking/internal/port/outbound"
	"context"
	"testing"
)

// Test LargeFunctionChunker addSplitChunkOverlap method for proper context addition.
func TestLargeFunctionChunkerAddSplitChunkOverlap(t *testing.T) {
	tests := []struct {
		name            string
		chunks          []outbound.EnhancedCodeChunk
		overlapSize     int
		expectedContext map[int]string // chunk index -> expected preceding context
		shouldModify    map[int]bool   // chunk index -> should be modified
	}{
		{
			name: "should add overlap between split chunks",
			chunks: []outbound.EnhancedCodeChunk{
				{
					ID:       "chunk-1",
					Content:  "function part1() {\n    const x = 1;\n    return x;\n}",
					Language: mustCreateLanguage("javascript"),
					PreservedContext: outbound.PreservedContext{
						PrecedingContext: "",
					},
				},
				{
					ID:       "chunk-2",
					Content:  "function part2() {\n    const y = 2;\n    return y;\n}",
					Language: mustCreateLanguage("javascript"),
					PreservedContext: outbound.PreservedContext{
						PrecedingContext: "",
					},
				},
				{
					ID:       "chunk-3",
					Content:  "function part3() {\n    const z = 3;\n    return z;\n}",
					Language: mustCreateLanguage("javascript"),
					PreservedContext: outbound.PreservedContext{
						PrecedingContext: "",
					},
				},
			},
			overlapSize: 500,
			expectedContext: map[int]string{
				0: "",                                                       // First chunk should have no preceding context
				1: "function part1() {\n    const x = 1;\n    return x;\n}", // Full content fits in 500 bytes
				2: "function part2() {\n    const y = 2;\n    return y;\n}", // Full content fits in 500 bytes
			},
			shouldModify: map[int]bool{0: false, 1: true, 2: true},
		},
		{
			name: "should not add overlap when size is zero",
			chunks: []outbound.EnhancedCodeChunk{
				{
					ID:       "chunk-1",
					Content:  "function part1() {}",
					Language: mustCreateLanguage("javascript"),
				},
				{
					ID:       "chunk-2",
					Content:  "function part2() {}",
					Language: mustCreateLanguage("javascript"),
				},
			},
			overlapSize: 0,
			expectedContext: map[int]string{
				0: "",
				1: "",
			},
			shouldModify: map[int]bool{0: false, 1: false},
		},
		{
			name: "should not add overlap when size is negative",
			chunks: []outbound.EnhancedCodeChunk{
				{
					ID:       "chunk-1",
					Content:  "function part1() {}",
					Language: mustCreateLanguage("javascript"),
				},
				{
					ID:       "chunk-2",
					Content:  "function part2() {}",
					Language: mustCreateLanguage("javascript"),
				},
			},
			overlapSize: -100,
			expectedContext: map[int]string{
				0: "",
				1: "",
			},
			shouldModify: map[int]bool{0: false, 1: false},
		},
		{
			name: "should append to existing context",
			chunks: []outbound.EnhancedCodeChunk{
				{
					ID:       "chunk-1",
					Content:  "function part1() {\n    console.log('hello');\n}",
					Language: mustCreateLanguage("javascript"),
					PreservedContext: outbound.PreservedContext{
						PrecedingContext: "",
					},
				},
				{
					ID:       "chunk-2",
					Content:  "function part2() {}",
					Language: mustCreateLanguage("javascript"),
					PreservedContext: outbound.PreservedContext{
						PrecedingContext: "existing context",
					},
				},
			},
			overlapSize: 500,
			expectedContext: map[int]string{
				0: "",
				1: "function part1() {\n    console.log('hello');\n}\nexisting context", // Full content appended to existing
			},
			shouldModify: map[int]bool{0: false, 1: true},
		},
		{
			name: "should handle single chunk gracefully",
			chunks: []outbound.EnhancedCodeChunk{
				{
					ID:       "chunk-1",
					Content:  "function onlyChunk() {}",
					Language: mustCreateLanguage("javascript"),
				},
			},
			overlapSize: 500,
			expectedContext: map[int]string{
				0: "",
			},
			shouldModify: map[int]bool{0: false},
		},
		{
			name:            "should handle empty chunks list",
			chunks:          []outbound.EnhancedCodeChunk{},
			overlapSize:     500,
			expectedContext: map[int]string{},
			shouldModify:    map[int]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewLargeFunctionChunkerWithDefaults()
			ctx := context.Background()
			config := outbound.ChunkingConfiguration{
				OverlapSize: tt.overlapSize,
			}

			chunker.addSplitChunkOverlap(ctx, tt.chunks, config)

			for i, expected := range tt.expectedContext {
				if i >= len(tt.chunks) {
					continue
				}

				actual := tt.chunks[i].PreservedContext.PrecedingContext
				if actual != expected {
					t.Errorf("Chunk %d preceding context = %q, want %q", i, actual, expected)
				}
			}
		})
	}
}

// Test LargeFunctionChunker extractSplitOverlapContext method for proper ending context extraction.
func TestLargeFunctionChunkerExtractSplitOverlapContext(t *testing.T) {
	tests := []struct {
		name          string
		chunk         *outbound.EnhancedCodeChunk
		maxSize       int
		expected      string
		shouldContain []string // Strings that should be contained in result
		shouldExclude []string // Strings that should NOT be contained in result
	}{
		{
			name: "should extract last few lines within size limit",
			chunk: &outbound.EnhancedCodeChunk{
				Content: `function largeFunction() {
    const variable1 = "value1";
    const variable2 = "value2";
    console.log(variable1);
    return variable2;}`,
			},
			maxSize:       50,
			expected:      "    return variable2;}",
			shouldContain: []string{"return variable2"},
			shouldExclude: []string{"const variable1"},
		},
		{
			name: "should extract multiple lines when size permits",
			chunk: &outbound.EnhancedCodeChunk{
				Content: `def python_function():
    x = 1
    y = 2
    z = x + y
    return z`,
			},
			maxSize:       100,
			expected:      "def python_function():\n    x = 1\n    y = 2\n    z = x + y\n    return z", // Full content fits in 100 bytes (62 bytes total)
			shouldContain: []string{"z = x + y", "return z"},
			shouldExclude: []string{}, // All content is included when it fits
		},
		{
			name: "should handle single line content",
			chunk: &outbound.EnhancedCodeChunk{
				Content: "return result;",
			},
			maxSize:       50,
			expected:      "return result;",
			shouldContain: []string{"return result"},
			shouldExclude: []string{},
		},
		{
			name: "should respect size limits strictly",
			chunk: &outbound.EnhancedCodeChunk{
				Content: `function test() {
    const veryLongVariableName = "this is a very long string that exceeds the size limit";
    return veryLongVariableName;}`,
			},
			maxSize:       30,
			expected:      "", // Even the last line exceeds 30 bytes, so nothing fits
			shouldContain: []string{},
			shouldExclude: []string{"const veryLongVariableName", "return"},
		},
		{
			name: "should handle empty content gracefully",
			chunk: &outbound.EnhancedCodeChunk{
				Content: "",
			},
			maxSize:       50,
			expected:      "",
			shouldContain: []string{},
			shouldExclude: []string{},
		},
		{
			name: "should handle whitespace-only content",
			chunk: &outbound.EnhancedCodeChunk{
				Content: "   \n\t  ",
			},
			maxSize:       50,
			expected:      "   \n\t  ",
			shouldContain: []string{},
			shouldExclude: []string{},
		},
		{
			name: "should take earlier lines when last lines are too long",
			chunk: &outbound.EnhancedCodeChunk{
				Content: `function test() {
    const x = 1;
    const veryLongLineThatExceedsSizeLimitAndShouldNotBeIncluded = "very long content";
    return x;}`,
			},
			maxSize:       20,
			expected:      "    return x;}", // Last line fits in 20 bytes (14 bytes + newline)
			shouldContain: []string{"return x"},
			shouldExclude: []string{"veryLongLineThatExceeds", "const x"},
		},
		{
			name: "should return empty when maxSize is zero",
			chunk: &outbound.EnhancedCodeChunk{
				Content: "function test() { return; }",
			},
			maxSize:       0,
			expected:      "",
			shouldContain: []string{},
			shouldExclude: []string{"function"},
		},
		{
			name: "should return empty when maxSize is negative",
			chunk: &outbound.EnhancedCodeChunk{
				Content: "function test() { return; }",
			},
			maxSize:       -100,
			expected:      "",
			shouldContain: []string{},
			shouldExclude: []string{"function"},
		},
		{
			name: "should handle multiline comments properly",
			chunk: &outbound.EnhancedCodeChunk{
				Content: `function withComments() {
    const x = 1;
    // This is a comment
    /* Multi-line
       comment */
    return x;}`,
			},
			maxSize:       50,
			expected:      "       comment */\n    return x;}", // Last 2 lines fit in 50 bytes (31 bytes total)
			shouldContain: []string{"return x", "comment"},
			shouldExclude: []string{"This is a comment", "const x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewLargeFunctionChunkerWithDefaults()
			result := chunker.extractSplitOverlapContext(tt.chunk, tt.maxSize)

			if result != tt.expected {
				t.Errorf("extractSplitOverlapContext() = %q, want %q", result, tt.expected)
			}

			// Check that required strings are contained
			for _, shouldContain := range tt.shouldContain {
				if !containsString(result, shouldContain) {
					t.Errorf("extractSplitOverlapContext() result should contain %q, got %q", shouldContain, result)
				}
			}

			// Check that excluded strings are not contained
			for _, shouldExclude := range tt.shouldExclude {
				if containsString(result, shouldExclude) {
					t.Errorf("extractSplitOverlapContext() result should NOT contain %q, got %q", shouldExclude, result)
				}
			}

			// Check total size doesn't exceed maxSize
			if tt.maxSize > 0 && len(result) > tt.maxSize {
				t.Errorf("extractSplitOverlapContext() result size %d exceeds maximum %d", len(result), tt.maxSize)
			}
		})
	}
}

// Test LargeFunctionChunker addSplitChunkOverlapIntegration tests the integration.
func TestLargeFunctionChunkerAddSplitChunkOverlapIntegration(t *testing.T) {
	tests := []struct {
		name            string
		semanticChunks  []outbound.SemanticCodeChunk
		config          outbound.ChunkingConfiguration
		expectedContext map[int]string // chunk index -> expected preceding context
		expectErrors    bool
	}{
		{
			name: "should add overlap in real split scenario",
			semanticChunks: []outbound.SemanticCodeChunk{
				{
					Name:      "largeFunction",
					Type:      outbound.ConstructFunction,
					Content:   "function largeFunction() { /* very long content that will be split */ }",
					Language:  mustCreateLanguage("javascript"),
					StartByte: 0,
					EndByte:   50000, // Large enough to trigger splitting
				},
			},
			config: outbound.ChunkingConfiguration{
				OverlapSize:     300,
				MaxChunkSize:    1000,
				EnableSplitting: true,
			},
			expectedContext: map[int]string{
				0: "", // First chunk has no preceding context
				1: "", // Should contain ending lines from previous chunk
				2: "", // Should contain ending lines from previous chunk
			},
			expectErrors: false,
		},
		{
			name: "should not add overlap when splitting disabled",
			semanticChunks: []outbound.SemanticCodeChunk{
				{
					Name:      "smallFunction",
					Type:      outbound.ConstructFunction,
					Content:   "function smallFunction() { return; }",
					Language:  mustCreateLanguage("javascript"),
					StartByte: 0,
					EndByte:   100,
				},
			},
			config: outbound.ChunkingConfiguration{
				OverlapSize:     300,
				MaxChunkSize:    1000,
				EnableSplitting: false,
			},
			expectedContext: map[int]string{
				0: "", // Single chunk, no overlap
			},
			expectErrors: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewLargeFunctionChunkerWithDefaults()
			ctx := context.Background()

			enhancedChunks, err := chunker.ChunkCode(ctx, tt.semanticChunks, tt.config)
			if tt.expectErrors {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			for i, expected := range tt.expectedContext {
				if i >= len(enhancedChunks) {
					continue
				}

				actual := enhancedChunks[i].PreservedContext.PrecedingContext
				if expected != "" && actual == "" {
					t.Errorf("Chunk %d should have overlap context but doesn't", i)
				}
			}

			// Check that overlap was added between chunks when configured
			if tt.config.OverlapSize > 0 && len(enhancedChunks) > 1 {
				for i := 1; i < len(enhancedChunks); i++ {
					if enhancedChunks[i].PreservedContext.PrecedingContext == "" {
						t.Errorf("Chunk %d should have overlap context from previous chunk", i)
					}
				}
			}
		})
	}
}

// Test LargeFunctionChunker edge cases for overlap functionality.
func TestLargeFunctionChunkerOverlapEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(*testing.T, *LargeFunctionChunker)
	}{
		{
			name: "should handle chunks with only whitespace and newlines",
			testFunc: func(t *testing.T, chunker *LargeFunctionChunker) {
				chunk := &outbound.EnhancedCodeChunk{
					Content: "\n\n\n   \n\t\n\n",
				}
				result := chunker.extractSplitOverlapContext(chunk, 100)
				expected := "\n\n\n   \n\t\n\n"
				if result != expected {
					t.Errorf(
						"extractSplitOverlapContext() should preserve whitespace, got %q want %q",
						result,
						expected,
					)
				}
			},
		},
		{
			name: "should handle chunks where last line is very long",
			testFunc: func(t *testing.T, chunker *LargeFunctionChunker) {
				longLine := "const veryLongVariableName = 'this string is very long and should be handled properly'"
				chunk := &outbound.EnhancedCodeChunk{
					Content: "function test() {\n    " + longLine + "\n}",
				}
				result := chunker.extractSplitOverlapContext(chunk, 30)
				if len(result) > 30 {
					t.Errorf("extractSplitOverlapContext() should respect size limit for long lines, got %q", result)
				}
			},
		},
		{
			name: "should handle chunks with mixed programming language syntax",
			testFunc: func(t *testing.T, chunker *LargeFunctionChunker) {
				chunk := &outbound.EnhancedCodeChunk{
					Content: `function hybridSyntax() {
    // JavaScript-style comment
    if (condition) {
        $result = some_function(); // PHP-style
        return $result;
    }
}`,
				}
				result := chunker.extractSplitOverlapContext(chunk, 100)
				if !containsString(result, "return") && !containsString(result, "$result") {
					t.Errorf("extractSplitOverlapContext() should work with mixed syntax, got %q", result)
				}
			},
		},
		{
			name: "should handle nil chunk gracefully",
			testFunc: func(t *testing.T, chunker *LargeFunctionChunker) {
				result := chunker.extractSplitOverlapContext(nil, 100)
				if result != "" {
					t.Errorf("extractSplitOverlapContext() should return empty string for nil chunk, got %q", result)
				}
			},
		},
		{
			name: "should maintain order of extracted lines",
			testFunc: func(t *testing.T, chunker *LargeFunctionChunker) {
				chunk := &outbound.EnhancedCodeChunk{
					Content: `function test() {
    line1();
    line2();
    line3();
}`,
				}
				result := chunker.extractSplitOverlapContext(chunk, 100)
				expected := "function test() {\n    line1();\n    line2();\n    line3();\n}" // Full content fits in 100 bytes
				if result != expected {
					t.Errorf(
						"extractSplitOverlapContext() should maintain line order, got %q want %q",
						result,
						expected,
					)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewLargeFunctionChunkerWithDefaults()
			tt.testFunc(t, chunker)
		})
	}
}

// TestLargeFunctionChunkerShouldSplitFunction tests split decision logic.
func TestLargeFunctionChunkerShouldSplitFunction(t *testing.T) {
	tests := []struct {
		name           string
		chunk          outbound.SemanticCodeChunk
		config         outbound.ChunkingConfiguration
		expectedSplit  bool
		expectedChunks int
	}{
		{
			name: "should recommend split for large function",
			chunk: outbound.SemanticCodeChunk{
				Name:     "largeFunction",
				Content:  generateLargeContent(20000), // 20KB
				Language: mustCreateLanguage("javascript"),
			},
			config:         outbound.ChunkingConfiguration{MaxChunkSize: 1000},
			expectedSplit:  true,
			expectedChunks: 20, // 20KB / 1KB = 20 chunks
		},
		{
			name: "should not split small function",
			chunk: outbound.SemanticCodeChunk{
				Name:     "smallFunction",
				Content:  generateLargeContent(500), // 500 bytes
				Language: mustCreateLanguage("javascript"),
			},
			config:         outbound.ChunkingConfiguration{MaxChunkSize: 1000},
			expectedSplit:  false,
			expectedChunks: 1,
		},
		{
			name: "should use config max size over default",
			chunk: outbound.SemanticCodeChunk{
				Name:     "mediumFunction",
				Content:  generateLargeContent(8000), // 8KB
				Language: mustCreateLanguage("javascript"),
			},
			config:         outbound.ChunkingConfiguration{MaxChunkSize: 500},
			expectedSplit:  true,
			expectedChunks: 16, // 8KB / 500B = 16 chunks
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewLargeFunctionChunkerWithDefaults()
			ctx := context.Background()

			shouldSplit, recommendation, err := chunker.ShouldSplitFunction(ctx, tt.chunk, tt.config)
			if err != nil {
				t.Fatalf("ShouldSplitFunction() returned error: %v", err)
			}

			if shouldSplit != tt.expectedSplit {
				t.Errorf("ShouldSplitFunction() = %v, want %v", shouldSplit, tt.expectedSplit)
			}

			if recommendation.EstimatedChunks != tt.expectedChunks {
				t.Errorf("Recommended chunks = %d, want %d", recommendation.EstimatedChunks, tt.expectedChunks)
			}
		})
	}
}
