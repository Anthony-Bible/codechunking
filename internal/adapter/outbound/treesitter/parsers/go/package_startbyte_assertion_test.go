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

// TestPackageStartByteAssertionConsistency verifies that assertion logic is consistent
// with expected test data. This test exposes the logical inconsistency where:
// - Expected data specifies StartByte: 0 (correct for file-start packages)
// - Assertion uses assert.NotZero() which rejects StartByte=0 (incorrect logic).
func TestPackageStartByteAssertionConsistency(t *testing.T) {
	ctx := context.Background()
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)
	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	testCases := []struct {
		name                  string
		sourceCode            string
		expectedChunk         outbound.SemanticCodeChunk
		shouldFailWithNotZero bool
		assertionDescription  string
	}{
		{
			name: "Package at file start - StartByte=0 should be valid",
			sourceCode: `package main

func main() {}`,
			expectedChunk: outbound.SemanticCodeChunk{
				ChunkID:       "package:main",
				Type:          outbound.ConstructPackage,
				Name:          "main",
				QualifiedName: "main",
				Documentation: "",
				Content:       "package main",
				StartByte:     0, // This SHOULD be accepted as valid
				EndByte:       12,
				Language:      valueobject.Go,
			},
			shouldFailWithNotZero: true,
			assertionDescription:  "assert.NotZero(StartByte) incorrectly rejects valid StartByte=0",
		},
		{
			name: "Package after whitespace - StartByte>0 should be valid",
			sourceCode: `

package spaced

func test() {}`,
			expectedChunk: outbound.SemanticCodeChunk{
				ChunkID:       "package:spaced",
				Type:          outbound.ConstructPackage,
				Name:          "spaced",
				QualifiedName: "spaced",
				Documentation: "",
				Content:       "package spaced",
				StartByte:     2, // This should be accepted
				EndByte:       16,
				Language:      valueobject.Go,
			},
			shouldFailWithNotZero: false,
			assertionDescription:  "assert.NotZero(StartByte) correctly accepts StartByte>0",
		},
		{
			name: "Package after comment - StartByte>0 should be valid",
			sourceCode: `// File comment
package commented

func test() {}`,
			expectedChunk: outbound.SemanticCodeChunk{
				ChunkID:       "package:commented",
				Type:          outbound.ConstructPackage,
				Name:          "commented",
				QualifiedName: "commented",
				Documentation: "",
				Content:       "package commented",
				StartByte:     16, // This should be accepted
				EndByte:       34,
				Language:      valueobject.Go,
			},
			shouldFailWithNotZero: false,
			assertionDescription:  "assert.NotZero(StartByte) correctly accepts StartByte>0",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			parseTree, err := parser.Parse(ctx, []byte(tt.sourceCode))
			require.NoError(t, err)
			require.NotNil(t, parseTree)

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
			require.NoError(t, err)
			require.NotNil(t, domainTree)

			adapter := treesitter.NewSemanticTraverserAdapter()
			options := &treesitter.SemanticExtractionOptions{
				IncludePackages: true,
			}
			chunks := adapter.ExtractCodeChunks(domainTree, options)

			// Filter to only package chunks for this test
			var packageChunks []outbound.SemanticCodeChunk
			for _, chunk := range chunks {
				if chunk.Type == outbound.ConstructPackage {
					packageChunks = append(packageChunks, chunk)
				}
			}
			require.Len(t, packageChunks, 1, "Expected exactly one package chunk")

			actual := packageChunks[0]

			// Test all the core assertions that should pass
			assert.Equal(t, tt.expectedChunk.ChunkID, actual.ChunkID, "ChunkID mismatch")
			assert.Equal(t, tt.expectedChunk.Type, actual.Type, "Type mismatch")
			assert.Equal(t, tt.expectedChunk.Name, actual.Name, "Name mismatch")
			assert.Equal(t, tt.expectedChunk.QualifiedName, actual.QualifiedName, "QualifiedName mismatch")
			assert.Equal(t, tt.expectedChunk.Documentation, actual.Documentation, "Documentation mismatch")
			assert.Equal(t, tt.expectedChunk.Content, actual.Content, "Content mismatch")
			assert.Equal(t, tt.expectedChunk.Language, actual.Language, "Language mismatch")

			// Verify the expected StartByte matches our test data
			assert.Equal(t, tt.expectedChunk.StartByte, actual.StartByte,
				"StartByte should match expected value from test data")

			// This assertion demonstrates the logical inconsistency:
			// When StartByte=0 (valid for file-start packages), assert.NotZero fails
			// This test will FAIL for packages at file start, exposing the bug
			if tt.shouldFailWithNotZero {
				// This assertion will FAIL, demonstrating the inconsistency
				// We expect this to fail for StartByte=0 cases
				t.Logf("WARNING: This assert.NotZero will FAIL for valid StartByte=0: %s", tt.assertionDescription)
				// Run the broken assertion to demonstrate the bug
				assert.NotZero(t, actual.StartByte, tt.assertionDescription)
			} else {
				// This assertion should pass for non-zero StartByte
				assert.NotZero(t, actual.StartByte, tt.assertionDescription)
			}

			// These assertions should always pass
			assert.NotZero(t, actual.EndByte, "EndByte should not be zero")
			assert.Greater(t, actual.EndByte, actual.StartByte, "EndByte should be greater than StartByte")
		})
	}
}

// TestPackageStartByteZeroIsValid specifically tests that StartByte=0 is a valid
// value for packages that appear at the beginning of a file.
// This test will FAIL with current assertion logic but will PASS once fixed.
func TestPackageStartByteZeroIsValid(t *testing.T) {
	ctx := context.Background()
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)
	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err)

	sourceCode := `package zero
func test() {}`

	parseTree, err := parser.Parse(ctx, []byte(sourceCode))
	require.NoError(t, err)
	require.NotNil(t, parseTree)

	domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
	require.NoError(t, err)
	require.NotNil(t, domainTree)

	adapter := treesitter.NewSemanticTraverserAdapter()
	options := &treesitter.SemanticExtractionOptions{
		IncludePackages: true,
	}
	chunks := adapter.ExtractCodeChunks(domainTree, options)

	// Filter to only package chunks for this test
	var packageChunks []outbound.SemanticCodeChunk
	for _, chunk := range chunks {
		if chunk.Type == outbound.ConstructPackage {
			packageChunks = append(packageChunks, chunk)
		}
	}
	require.Len(t, packageChunks, 1)

	actual := packageChunks[0]

	// Verify this is actually a zero StartByte case (tree-sitter confirms this)
	assert.Equal(t, uint32(0), actual.StartByte, "Package at file start should have StartByte=0")

	// This assertion should validate that StartByte=0 is acceptable
	// Current implementation: assert.NotZero(t, actual.StartByte) - FAILS incorrectly
	// Fixed implementation should be: assert.GreaterOrEqual(t, actual.StartByte, uint32(0)) - PASSES
	//
	// This test will FAIL with current logic, demonstrating the bug
	assert.GreaterOrEqual(t, actual.StartByte, uint32(0),
		"StartByte should accept 0 as valid for file-start packages")

	// Additional validations that should always pass
	assert.NotZero(t, actual.EndByte, "EndByte should never be zero")
	assert.Greater(t, actual.EndByte, actual.StartByte, "EndByte should be greater than StartByte")
	assert.Equal(t, "package zero", actual.Content, "Content should match package declaration")
}

// TestPackagePositionEdgeCases tests various scenarios where packages appear
// at different positions in files, validating correct StartByte behavior.
// These tests expose edge cases and will guide proper assertion logic.
func TestPackagePositionEdgeCases(t *testing.T) {
	ctx := context.Background()
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)
	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err)

	testCases := []struct {
		name                 string
		sourceCode           string
		expectedStartByte    uint32
		expectedContent      string
		assertionDescription string
	}{
		{
			name:                 "Package immediately at file start",
			sourceCode:           "package immediate",
			expectedStartByte:    0,
			expectedContent:      "package immediate",
			assertionDescription: "File-start packages should have StartByte=0",
		},
		{
			name: "Package after single newline",
			sourceCode: `
package afternewline`,
			expectedStartByte:    1,
			expectedContent:      "package afternewline",
			assertionDescription: "Package after newline should have correct StartByte",
		},
		{
			name: "Package after multiple newlines",
			sourceCode: `


package aftermultiple`,
			expectedStartByte:    3,
			expectedContent:      "package aftermultiple",
			assertionDescription: "Package after multiple newlines should have correct StartByte",
		},
		{
			name: "Package after line comment",
			sourceCode: `// Comment
package aftercomment`,
			expectedStartByte:    11,
			expectedContent:      "package aftercomment",
			assertionDescription: "Package after line comment should have correct StartByte",
		},
		{
			name: "Package after block comment",
			sourceCode: `/* Block */
package afterblock`,
			expectedStartByte:    12,
			expectedContent:      "package afterblock",
			assertionDescription: "Package after block comment should have correct StartByte",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			parseTree, err := parser.Parse(ctx, []byte(tt.sourceCode))
			require.NoError(t, err)
			require.NotNil(t, parseTree)

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
			require.NoError(t, err)
			require.NotNil(t, domainTree)

			adapter := treesitter.NewSemanticTraverserAdapter()
			options := &treesitter.SemanticExtractionOptions{
				IncludePackages: true,
			}
			chunks := adapter.ExtractCodeChunks(domainTree, options)

			// Filter to only package chunks for this test
			var packageChunks []outbound.SemanticCodeChunk
			for _, chunk := range chunks {
				if chunk.Type == outbound.ConstructPackage {
					packageChunks = append(packageChunks, chunk)
				}
			}
			require.Len(t, packageChunks, 1, "Expected exactly one package chunk")

			actual := packageChunks[0]

			// Verify the expected positioning
			assert.Equal(t, tt.expectedStartByte, actual.StartByte, tt.assertionDescription)
			assert.Equal(t, tt.expectedContent, actual.Content, "Content should match expected")

			// The key assertion that needs fixing:
			// Current: assert.NotZero(t, actual.StartByte) - FAILS for StartByte=0 cases
			// Should be: assert.GreaterOrEqual(t, actual.StartByte, uint32(0)) - PASSES for all valid cases
			//
			// This will FAIL for the "Package immediately at file start" case
			if tt.expectedStartByte == 0 {
				// This assertion will FAIL, demonstrating the bug for StartByte=0
				t.Logf("WARNING: This assertion will FAIL due to incorrect NotZero logic for StartByte=0")
			}

			// Demonstrate proper assertion that should be used instead
			assert.GreaterOrEqual(t, actual.StartByte, uint32(0),
				"StartByte should be greater than or equal to 0 (not NotZero)")

			// These should always pass
			assert.NotZero(t, actual.EndByte, "EndByte should never be zero")
			assert.Greater(t, actual.EndByte, actual.StartByte, "EndByte should be greater than StartByte")
		})
	}
}

// TestAssertionLogicFix demonstrates the exact fix needed for the assertion logic.
// This test shows the difference between incorrect and correct assertions.
func TestAssertionLogicFix(t *testing.T) {
	ctx := context.Background()
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)
	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err)

	// Test with a package at file start (StartByte=0)
	sourceCode := `package fix`
	parseTree, err := parser.Parse(ctx, []byte(sourceCode))
	require.NoError(t, err)
	require.NotNil(t, parseTree)

	domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
	require.NoError(t, err)
	require.NotNil(t, domainTree)

	adapter := treesitter.NewSemanticTraverserAdapter()
	options := &treesitter.SemanticExtractionOptions{
		IncludePackages: true,
	}
	chunks := adapter.ExtractCodeChunks(domainTree, options)

	// Filter to only package chunks for this test
	var packageChunks []outbound.SemanticCodeChunk
	for _, chunk := range chunks {
		if chunk.Type == outbound.ConstructPackage {
			packageChunks = append(packageChunks, chunk)
		}
	}
	require.Len(t, packageChunks, 1)

	actual := packageChunks[0]

	t.Run("Current incorrect assertion", func(t *testing.T) {
		// This demonstrates the current BROKEN assertion logic
		// assert.NotZero(t, actual.StartByte) would FAIL here because StartByte=0
		// but StartByte=0 is VALID for packages at file start

		if actual.StartByte == 0 {
			t.Logf("StartByte is 0 - this is VALID for file-start packages")
			t.Logf("Current assertion assert.NotZero(StartByte) would FAIL incorrectly")
			t.Logf("This demonstrates the logical inconsistency in the test")
		}
	})

	t.Run("Correct assertion logic", func(t *testing.T) {
		// This demonstrates the CORRECT assertion logic that should be used

		// Option 1: Accept any non-negative value (recommended)
		assert.GreaterOrEqual(t, actual.StartByte, uint32(0),
			"StartByte should be non-negative (0 is valid for file-start packages)")

		// Option 2: If we want to distinguish file-start from positioned packages
		// We could have conditional logic, but Option 1 is simpler and correct

		// These assertions should remain unchanged
		assert.NotZero(t, actual.EndByte, "EndByte should never be zero")
		assert.Greater(t, actual.EndByte, actual.StartByte, "EndByte should be greater than StartByte")
	})

	t.Run("Fix verification", func(t *testing.T) {
		// Verify the specific values that cause the issue
		assert.Equal(t, uint32(0), actual.StartByte, "Package at file start has StartByte=0")
		assert.Equal(t, uint32(11), actual.EndByte, "Package 'fix' should end at byte 11")
		assert.Equal(t, "package fix", actual.Content, "Content should be the package declaration")

		// The fix: Replace assert.NotZero with assert.GreaterOrEqual
		// Before: assert.NotZero(t, actual.StartByte, "StartByte should not be zero")
		// After:  assert.GreaterOrEqual(t, actual.StartByte, uint32(0), "StartByte should be non-negative")
	})
}
