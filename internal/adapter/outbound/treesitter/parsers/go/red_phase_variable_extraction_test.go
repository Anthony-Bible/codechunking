package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoParserVariableExtractionComprehensive contains comprehensive tests that validate
// the expected behavior for Go variable and constant extraction.
//
// This test suite validates the complete implementation based on tree-sitter grammar analysis.
// Originally designed as a Red phase TDD test, it now serves as a comprehensive regression
// test to ensure quality and correctness of the Go parser's variable/constant extraction.
//
// Key behaviors these tests validate:
// 1. Position calculation accuracy (exact byte offsets from tree-sitter)
// 2. Field access correctly handling multiple variable names
// 3. Correct handling of grouped vs single declarations
// 4. Grammar-specific field access patterns for var_spec vs const_spec
// 5. Extraction order preserving source document order.
func TestGoParserVariableExtractionComprehensive(t *testing.T) {
	ctx := context.Background()
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)
	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name               string
		sourceCode         string
		expectedChunksFunc func(t *testing.T) []outbound.SemanticCodeChunk
		description        string
		grammarInsight     string // Specific tree-sitter grammar insight being tested
		expectedFailure    string // What should fail and why
	}{
		{
			name:            "Grammar insight: var_spec multiple name fields",
			description:     "Test that var_spec has multiple 'name' fields (one per identifier) and field access correctly handles this",
			grammarInsight:  "According to grammar analysis, var_spec has multiple 'name' fields - one per identifier",
			expectedFailure: "Current implementation likely fails to extract all variable names from GetMultipleFieldsByName",
			sourceCode:      `var identifier1, identifier2, identifier3 bool`,
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `var identifier1, identifier2, identifier3 bool`
				startByte, endByte := calculateExpectedVariablePositions(t, sourceCode, "identifier1")

				// All three variables must be extracted with correct field access
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "var:identifier1",
						Type:          outbound.ConstructVariable,
						Name:          "identifier1",
						QualifiedName: "identifier1",
						Visibility:    outbound.Private,
						Content:       "var identifier1, identifier2, identifier3 bool",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
					{
						ChunkID:       "var:identifier2",
						Type:          outbound.ConstructVariable,
						Name:          "identifier2",
						QualifiedName: "identifier2",
						Visibility:    outbound.Private,
						Content:       "var identifier1, identifier2, identifier3 bool",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
					{
						ChunkID:       "var:identifier3",
						Type:          outbound.ConstructVariable,
						Name:          "identifier3",
						QualifiedName: "identifier3",
						Visibility:    outbound.Private,
						Content:       "var identifier1, identifier2, identifier3 bool",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
				}
			},
		},
		{
			name:            "Grammar insight: const_spec sequence of identifiers",
			description:     "Test that const_spec has one 'name' field containing sequence of identifiers",
			grammarInsight:  "According to grammar analysis, const_spec has one 'name' field with sequence of identifiers",
			expectedFailure: "Current implementation may not correctly handle const_spec identifier sequence extraction",
			sourceCode:      `const name1, name2 = "value1", "value2"`,
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `const name1, name2 = "value1", "value2"`
				startByte, endByte := calculateExpectedVariablePositions(t, sourceCode, "name1")

				// Both constants must be extracted from the sequence
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "const:name1",
						Type:          outbound.ConstructConstant,
						Name:          "name1",
						QualifiedName: "name1",
						Visibility:    outbound.Private,
						Content:       "const name1, name2 = \"value1\", \"value2\"",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
					{
						ChunkID:       "const:name2",
						Type:          outbound.ConstructConstant,
						Name:          "name2",
						QualifiedName: "name2",
						Visibility:    outbound.Private,
						Content:       "const name1, name2 = \"value1\", \"value2\"",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
				}
			},
		},
		{
			name:            "Position calculation precision test",
			description:     "Test that position calculations are exact and not off by 1 (common tree-sitter issue)",
			grammarInsight:  "Tree-sitter byte positions should be used exactly as reported, no manual adjustments",
			expectedFailure: "Current implementation likely has off-by-1 errors in position calculations",
			sourceCode:      `var precise int`,
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `var precise int`
				startByte, endByte := calculateExpectedVariablePositions(t, sourceCode, "precise")

				// Position must be exactly what tree-sitter reports
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "var:precise",
						Type:          outbound.ConstructVariable,
						Name:          "precise",
						QualifiedName: "precise",
						Visibility:    outbound.Private,
						Content:       "var precise int",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
				}
			},
		},
		{
			name:            "Grouped vs single declaration position logic",
			description:     "Test that grouped declarations use spec positions, single declarations use declaration positions",
			grammarInsight:  "Grouped declarations (with parentheses) should use var_spec positions, single should use var_declaration positions",
			expectedFailure: "Current implementation incorrectly calculates positions based on declaration grouping",
			sourceCode: `var single int
var (
	grouped string
)`,
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `var single int
var (
	grouped string
)`
				// Single declaration should use declaration positions
				startByteSingle, endByteSingle := calculateExpectedVariablePositions(t, sourceCode, "single")
				// Grouped declaration should use spec positions
				startByteGrouped, endByteGrouped := calculateExpectedVariablePositions(t, sourceCode, "grouped")

				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "var:single",
						Type:          outbound.ConstructVariable,
						Name:          "single",
						QualifiedName: "single",
						Visibility:    outbound.Private,
						Content:       "var single int", // Full declaration for single
						StartByte:     startByteSingle,
						EndByte:       endByteSingle,
						Language:      valueobject.Go,
					},
					{
						ChunkID:       "var:grouped",
						Type:          outbound.ConstructVariable,
						Name:          "grouped",
						QualifiedName: "grouped",
						Visibility:    outbound.Private,
						Content:       "grouped string", // Just spec content for grouped
						StartByte:     startByteGrouped,
						EndByte:       endByteGrouped,
						Language:      valueobject.Go,
					},
				}
			},
		},
		{
			name:            "Field access pattern consistency",
			description:     "Test that field access correctly extracts all identifiers using proper tree-sitter field methods",
			grammarInsight:  "GetMultipleFieldsByName should work consistently for both var_spec and const_spec nodes",
			expectedFailure: "Current field access implementation may miss identifiers or return duplicates",
			sourceCode:      `var alpha, beta, gamma, delta string`,
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `var alpha, beta, gamma, delta string`
				startByte, endByte := calculateExpectedVariablePositions(t, sourceCode, "alpha")

				// All four variables must be found with correct names
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "var:alpha",
						Type:          outbound.ConstructVariable,
						Name:          "alpha",
						QualifiedName: "alpha",
						Visibility:    outbound.Private,
						Content:       "var alpha, beta, gamma, delta string",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
					{
						ChunkID:       "var:beta",
						Type:          outbound.ConstructVariable,
						Name:          "beta",
						QualifiedName: "beta",
						Visibility:    outbound.Private,
						Content:       "var alpha, beta, gamma, delta string",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
					{
						ChunkID:       "var:gamma",
						Type:          outbound.ConstructVariable,
						Name:          "gamma",
						QualifiedName: "gamma",
						Visibility:    outbound.Private,
						Content:       "var alpha, beta, gamma, delta string",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
					{
						ChunkID:       "var:delta",
						Type:          outbound.ConstructVariable,
						Name:          "delta",
						QualifiedName: "delta",
						Visibility:    outbound.Private,
						Content:       "var alpha, beta, gamma, delta string",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
				}
			},
		},
		{
			name:            "Complex multi-line grouped declarations",
			description:     "Test that complex grouped declarations with different types are handled correctly",
			grammarInsight:  "Each spec in a grouped declaration should have its own position and content extraction",
			expectedFailure: "Current implementation may not handle complex grouped declarations correctly",
			sourceCode: `var (
	first    int    = 1
	second   string = "test"
	third    bool   // uninitialized
)`,
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `var (
	first    int    = 1
	second   string = "test"
	third    bool   // uninitialized
)`
				startByteFirst, endByteFirst := calculateExpectedVariablePositions(t, sourceCode, "first")
				startByteSecond, endByteSecond := calculateExpectedVariablePositions(t, sourceCode, "second")
				startByteThird, endByteThird := calculateExpectedVariablePositions(t, sourceCode, "third")

				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "var:first",
						Type:          outbound.ConstructVariable,
						Name:          "first",
						QualifiedName: "first",
						Visibility:    outbound.Private,
						Content:       "first    int    = 1",
						StartByte:     startByteFirst,
						EndByte:       endByteFirst,
						Language:      valueobject.Go,
					},
					{
						ChunkID:       "var:second",
						Type:          outbound.ConstructVariable,
						Name:          "second",
						QualifiedName: "second",
						Visibility:    outbound.Private,
						Content:       "second   string = \"test\"",
						StartByte:     startByteSecond,
						EndByte:       endByteSecond,
						Language:      valueobject.Go,
					},
					{
						ChunkID:       "var:third",
						Type:          outbound.ConstructVariable,
						Name:          "third",
						QualifiedName: "third",
						Visibility:    outbound.Private,
						Content:       "third    bool   // uninitialized",
						StartByte:     startByteThird,
						EndByte:       endByteThird,
						Language:      valueobject.Go,
					},
				}
			},
		},
		{
			name:            "Mixed constant and variable extraction order",
			description:     "Test that constants and variables are extracted in source order with correct categorization",
			grammarInsight:  "Different node types (var_declaration, const_declaration) should be processed consistently",
			expectedFailure: "Current implementation may not maintain source order or correctly categorize constructs",
			sourceCode: `const FIRST = 1
var second int
const THIRD = "test"
var fourth bool`,
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `const FIRST = 1
var second int
const THIRD = "test"
var fourth bool`

				startByteFirst, endByteFirst := calculateExpectedVariablePositions(t, sourceCode, "FIRST")
				startByteSecond, endByteSecond := calculateExpectedVariablePositions(t, sourceCode, "second")
				startByteThird, endByteThird := calculateExpectedVariablePositions(t, sourceCode, "THIRD")
				startByteFourth, endByteFourth := calculateExpectedVariablePositions(t, sourceCode, "fourth")

				// Should be extracted in source order with correct types
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "const:FIRST",
						Type:          outbound.ConstructConstant,
						Name:          "FIRST",
						QualifiedName: "FIRST",
						Visibility:    outbound.Public,
						Content:       "const FIRST = 1",
						StartByte:     startByteFirst,
						EndByte:       endByteFirst,
						Language:      valueobject.Go,
					},
					{
						ChunkID:       "var:second",
						Type:          outbound.ConstructVariable,
						Name:          "second",
						QualifiedName: "second",
						Visibility:    outbound.Private,
						Content:       "var second int",
						StartByte:     startByteSecond,
						EndByte:       endByteSecond,
						Language:      valueobject.Go,
					},
					{
						ChunkID:       "const:THIRD",
						Type:          outbound.ConstructConstant,
						Name:          "THIRD",
						QualifiedName: "THIRD",
						Visibility:    outbound.Public,
						Content:       "const THIRD = \"test\"",
						StartByte:     startByteThird,
						EndByte:       endByteThird,
						Language:      valueobject.Go,
					},
					{
						ChunkID:       "var:fourth",
						Type:          outbound.ConstructVariable,
						Name:          "fourth",
						QualifiedName: "fourth",
						Visibility:    outbound.Private,
						Content:       "var fourth bool",
						StartByte:     startByteFourth,
						EndByte:       endByteFourth,
						Language:      valueobject.Go,
					},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Log test details for debugging
			t.Logf("RED PHASE TEST: %s", tt.name)
			t.Logf("Description: %s", tt.description)
			t.Logf("Grammar Insight: %s", tt.grammarInsight)
			t.Logf("Expected Failure: %s", tt.expectedFailure)

			parseTree, err := parser.Parse(ctx, []byte(tt.sourceCode))
			require.NoError(t, err)
			require.NotNil(t, parseTree)

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
			require.NoError(t, err)
			require.NotNil(t, domainTree)

			adapter := treesitter.NewSemanticTraverserAdapter()
			options := &treesitter.SemanticExtractionOptions{
				IncludeVariables: true,
				IncludeConstants: true,
				IncludePrivate:   true,
			}

			chunks := adapter.ExtractCodeChunks(domainTree, options)

			// Filter to only variable/constant chunks for this test
			var varConstChunks []outbound.SemanticCodeChunk
			for _, chunk := range chunks {
				if chunk.Type == outbound.ConstructVariable || chunk.Type == outbound.ConstructConstant {
					varConstChunks = append(varConstChunks, chunk)
				}
			}

			expectedChunks := tt.expectedChunksFunc(t)

			// Log what we found vs what we expected
			t.Logf("Found %d chunks, expected %d", len(varConstChunks), len(expectedChunks))
			for i, chunk := range varConstChunks {
				t.Logf("  Actual[%d]: %s (%s) at %d-%d", i, chunk.Name, chunk.Type, chunk.StartByte, chunk.EndByte)
			}
			for i, chunk := range expectedChunks {
				t.Logf("  Expected[%d]: %s (%s) at %d-%d", i, chunk.Name, chunk.Type, chunk.StartByte, chunk.EndByte)
			}

			// These assertions will likely fail - that's the point of Red phase TDD
			assert.Len(t, varConstChunks, len(expectedChunks), "Number of extracted chunks should match expected")

			// Compare each expected chunk with actual chunks
			for i, expected := range expectedChunks {
				if i >= len(varConstChunks) {
					t.Errorf("Missing expected chunk: %s (%s)", expected.Name, expected.Type)
					continue
				}

				actual := varConstChunks[i]
				logChunkMismatches(t, expected, actual)
				assertChunkMatches(t, expected, actual)
			}
		})
	}
}

// logChunkMismatches logs specific differences between expected and actual chunks for debugging.
func logChunkMismatches(t *testing.T, expected, actual outbound.SemanticCodeChunk) {
	if expected.Name != actual.Name {
		t.Logf("NAME MISMATCH: expected '%s', got '%s'", expected.Name, actual.Name)
	}
	if expected.StartByte != actual.StartByte {
		t.Logf("START POSITION MISMATCH for %s: expected %d (0x%x), got %d (0x%x)",
			expected.Name, expected.StartByte, expected.StartByte, actual.StartByte, actual.StartByte)
	}
	if expected.EndByte != actual.EndByte {
		t.Logf("END POSITION MISMATCH for %s: expected %d (0x%x), got %d (0x%x)",
			expected.Name, expected.EndByte, expected.EndByte, actual.EndByte, actual.EndByte)
	}
	if expected.Content != actual.Content {
		t.Logf("CONTENT MISMATCH for %s: expected '%s', got '%s'",
			expected.Name, expected.Content, actual.Content)
	}
}

// assertChunkMatches performs all the core assertions for chunk comparison.
func assertChunkMatches(t *testing.T, expected, actual outbound.SemanticCodeChunk) {
	assert.Equal(t, expected.ChunkID, actual.ChunkID, "ChunkID must match expected format")
	assert.Equal(t, expected.Type, actual.Type, "Construct type must be correct")
	assert.Equal(t, expected.Name, actual.Name, "Variable/constant name must be extracted correctly")
	assert.Equal(t, expected.QualifiedName, actual.QualifiedName, "Qualified name must match")
	assert.Equal(t, expected.Visibility, actual.Visibility, "Visibility must be determined correctly")
	assert.Equal(t, expected.Content, actual.Content, "Content must match expected text")
	assert.Equal(t, expected.StartByte, actual.StartByte, "Start position must be calculated correctly")
	assert.Equal(t, expected.EndByte, actual.EndByte, "End position must be calculated correctly")
	assert.Equal(t, expected.Language, actual.Language, "Language must be Go")
}
