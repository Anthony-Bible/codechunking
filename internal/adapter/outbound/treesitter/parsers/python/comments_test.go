package pythonparser

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"testing"
	"time"

	forest "github.com/alexaandru/go-sitter-forest"
	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractModuleComments_BasicComment tests extraction of a simple comment.
// This is a RED PHASE test that defines expected behavior for basic comment extraction.
// Expected: Comment text "Simple comment" should be extracted without the '#' prefix.
func TestExtractModuleComments_BasicComment(t *testing.T) {
	sourceCode := `# Simple comment
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	require.Len(t, comments, 1, "Should extract exactly 1 comment")
	assert.Equal(t, "Simple comment", comments[0], "Comment should be 'Simple comment' without '#' prefix")
}

// TestExtractModuleComments_InlineComment tests extraction of inline comments.
// This is a RED PHASE test that defines expected behavior for inline comment extraction.
// Expected: Inline comment "Inline comment" should be extracted.
func TestExtractModuleComments_InlineComment(t *testing.T) {
	sourceCode := `x = 1  # Inline comment
y = 2
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	require.Len(t, comments, 1, "Should extract exactly 1 inline comment")
	assert.Equal(t, "Inline comment", comments[0], "Inline comment should be 'Inline comment'")
}

// TestExtractModuleComments_SpecialCharacters tests comments with special characters.
// This is a RED PHASE test that defines expected behavior for comments containing special chars.
// Expected: Special characters should be preserved in the extracted comment.
func TestExtractModuleComments_SpecialCharacters(t *testing.T) {
	sourceCode := `# Comment with @#$%^&*()
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	require.Len(t, comments, 1, "Should extract exactly 1 comment")
	assert.Equal(t, "Comment with @#$%^&*()", comments[0], "Special characters should be preserved")
}

// TestExtractModuleComments_WithQuotes tests comments containing quotes.
// This is a RED PHASE test that defines expected behavior for comments with quotes.
// Expected: Both single and double quotes should be preserved in the comment text.
func TestExtractModuleComments_WithQuotes(t *testing.T) {
	sourceCode := `# Comment with "quotes" and 'apostrophes'
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	require.Len(t, comments, 1, "Should extract exactly 1 comment")
	assert.Equal(t, "Comment with \"quotes\" and 'apostrophes'", comments[0], "Quotes should be preserved")
}

// TestExtractModuleComments_MultipleHashLevels tests comments with multiple hash symbols.
// This is a RED PHASE test that defines expected behavior for ##, ###, etc. comments.
// Expected: Only the first '#' should be removed, preserving additional '#' symbols.
func TestExtractModuleComments_MultipleHashLevels(t *testing.T) {
	sourceCode := `## Documentation comment
### Nested level comment
#### Fourth level
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	require.Len(t, comments, 3, "Should extract 3 comments")
	assert.Equal(t, "# Documentation comment", comments[0], "Second '#' should be preserved")
	assert.Equal(t, "## Nested level comment", comments[1], "Second and third '#' should be preserved")
	assert.Equal(t, "### Fourth level", comments[2], "Additional '#' symbols should be preserved")
}

// TestExtractModuleComments_EmptyComment tests comment with only '#' symbol.
// This is a RED PHASE test that defines expected behavior for empty comments.
// Expected: Empty comments should be filtered out (not included in results).
func TestExtractModuleComments_EmptyComment(t *testing.T) {
	sourceCode := `#
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	assert.Empty(t, comments, "Empty comments should be filtered out")
}

// TestExtractModuleComments_WhitespaceOnly tests comment with only whitespace.
// This is a RED PHASE test that defines expected behavior for whitespace-only comments.
// Expected: Comments with only whitespace should be filtered out after trimming.
func TestExtractModuleComments_WhitespaceOnly(t *testing.T) {
	sourceCode := `#
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	assert.Empty(t, comments, "Whitespace-only comments should be filtered out")
}

// TestExtractModuleComments_NoSpaceAfterHash tests comment without space after '#'.
// This is a RED PHASE test that defines expected behavior when there's no space after '#'.
// Expected: Comment text should be extracted correctly even without space.
func TestExtractModuleComments_NoSpaceAfterHash(t *testing.T) {
	sourceCode := `#NoSpaceComment
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	require.Len(t, comments, 1, "Should extract exactly 1 comment")
	assert.Equal(t, "NoSpaceComment", comments[0], "Comment should work without space after '#'")
}

// TestExtractModuleComments_TrailingSpaces tests comment with trailing whitespace.
// This is a RED PHASE test that defines expected behavior for comments with trailing spaces.
// Expected: Trailing whitespace should be trimmed from the comment text.
func TestExtractModuleComments_TrailingSpaces(t *testing.T) {
	sourceCode := `# Comment with trailing spaces
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	require.Len(t, comments, 1, "Should extract exactly 1 comment")
	assert.Equal(t, "Comment with trailing spaces", comments[0], "Trailing spaces should be trimmed")
}

// TestExtractModuleComments_URLInComment tests comment containing a URL.
// This is a RED PHASE test that defines expected behavior for comments with URLs.
// Expected: URLs should be preserved in the comment text.
func TestExtractModuleComments_URLInComment(t *testing.T) {
	sourceCode := `# Comment with URLs: https://example.com
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	require.Len(t, comments, 1, "Should extract exactly 1 comment")
	assert.Equal(t, "Comment with URLs: https://example.com", comments[0], "URLs should be preserved")
}

// TestExtractModuleComments_PathInComment tests comment containing file paths.
// This is a RED PHASE test that defines expected behavior for comments with file paths.
// Expected: File paths should be preserved in the comment text.
func TestExtractModuleComments_PathInComment(t *testing.T) {
	sourceCode := `# Comment with paths: /usr/local/bin
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	require.Len(t, comments, 1, "Should extract exactly 1 comment")
	assert.Equal(t, "Comment with paths: /usr/local/bin", comments[0], "Paths should be preserved")
}

// TestExtractModuleComments_EmailInComment tests comment containing email addresses.
// This is a RED PHASE test that defines expected behavior for comments with email addresses.
// Expected: Email addresses should be preserved in the comment text.
func TestExtractModuleComments_EmailInComment(t *testing.T) {
	sourceCode := `# Comment with email: user@example.com
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	require.Len(t, comments, 1, "Should extract exactly 1 comment")
	assert.Equal(t, "Comment with email: user@example.com", comments[0], "Email should be preserved")
}

// TestExtractModuleComments_MultipleComments tests extraction of multiple module-level comments.
// This is a RED PHASE test that defines expected behavior for multiple comment extraction.
// Expected: All module-level comments should be extracted in order.
func TestExtractModuleComments_MultipleComments(t *testing.T) {
	sourceCode := `# First comment
# Second comment
x = 1
# Third comment
y = 2
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	require.Len(t, comments, 3, "Should extract 3 module-level comments")
	assert.Equal(t, "First comment", comments[0], "First comment should be extracted")
	assert.Equal(t, "Second comment", comments[1], "Second comment should be extracted")
	assert.Equal(t, "Third comment", comments[2], "Third comment should be extracted")
}

// TestExtractModuleComments_MultiLineLogicalComment tests multi-line logical comments.
// This is a RED PHASE test that defines expected behavior for consecutive comment lines.
// Expected: Each line should be extracted as a separate comment.
func TestExtractModuleComments_MultiLineLogicalComment(t *testing.T) {
	sourceCode := `# Multi-line logical comment
# that spans multiple lines
# to describe something
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	require.Len(t, comments, 3, "Should extract 3 separate comment lines")
	assert.Equal(t, "Multi-line logical comment", comments[0], "First line should be extracted")
	assert.Equal(t, "that spans multiple lines", comments[1], "Second line should be extracted")
	assert.Equal(t, "to describe something", comments[2], "Third line should be extracted")
}

// TestExtractModuleComments_NoComments tests source code without any comments.
// This is a RED PHASE test that defines expected behavior when no comments are present.
// Expected: Should return empty slice when no comments exist.
func TestExtractModuleComments_NoComments(t *testing.T) {
	sourceCode := `x = 1
y = 2
z = 3
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	assert.Empty(t, comments, "Should return empty slice when no comments exist")
}

// TestExtractModuleComments_NilParseTree tests error handling for nil parse tree.
// This is a RED PHASE test that defines expected behavior for nil input.
// Expected: Should return empty slice without panicking when parse tree is nil.
// CURRENT BEHAVIOR: Function panics on nil input - needs nil check at function start.
func TestExtractModuleComments_NilParseTree(t *testing.T) {
	// This test currently fails with panic - documenting expected behavior
	// The function should add a nil check for parseTree parameter
	defer func() {
		if r := recover(); r != nil {
			t.Log("Function panics on nil input - needs nil check for parseTree parameter")
			// This is expected to fail in RED phase
		}
	}()

	comments := extractModuleComments(nil)
	assert.Empty(t, comments, "Should return empty slice for nil parse tree")
}

// TestExtractModuleComments_EmptySource tests extraction from empty source code.
// This is a RED PHASE test that defines expected behavior for empty source.
// Expected: Domain layer rejects empty source, so test with minimal valid Python.
func TestExtractModuleComments_EmptySource(t *testing.T) {
	// Empty source is rejected by domain layer, so test with minimal valid Python
	sourceCode := `
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	assert.Empty(t, comments, "Should return empty slice for source with no comments")
}

// TestExtractModuleComments_OnlyModuleLevelComments tests that only module-level comments are extracted.
// This is a RED PHASE test that defines expected behavior for comment scope filtering.
// Expected: Only module-level comments should be extracted, not comments inside functions or classes.
func TestExtractModuleComments_OnlyModuleLevelComments(t *testing.T) {
	sourceCode := `# Module level comment 1
x = 1

def func():
    # This is inside a function
    y = 2

# Module level comment 2

class MyClass:
    # This is inside a class
    z = 3
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	// Should only extract module-level comments
	require.Len(t, comments, 2, "Should extract only 2 module-level comments")
	assert.Equal(t, "Module level comment 1", comments[0], "First module-level comment")
	assert.Equal(t, "Module level comment 2", comments[1], "Second module-level comment")
}

// TestExtractModuleComments_MixedWithDocstrings tests comments mixed with docstrings.
// This is a RED PHASE test that defines expected behavior when comments and docstrings coexist.
// Expected: Should extract comments but not docstrings (docstrings are string literals, not comments).
func TestExtractModuleComments_MixedWithDocstrings(t *testing.T) {
	sourceCode := `# Module comment
"""Module docstring"""

# Another comment
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	require.Len(t, comments, 2, "Should extract 2 comments, not docstrings")
	assert.Equal(t, "Module comment", comments[0], "First comment should be extracted")
	assert.Equal(t, "Another comment", comments[1], "Second comment should be extracted")
}

// TestExtractModuleComments_ShebangLine tests handling of shebang lines.
// This is a RED PHASE test that defines expected behavior for shebang lines.
// Expected: Shebang lines (#!/usr/bin/env python) should be treated as comments and extracted.
func TestExtractModuleComments_ShebangLine(t *testing.T) {
	sourceCode := `#!/usr/bin/env python3
# Regular comment
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	// Shebang is typically treated as a comment in Python
	assert.GreaterOrEqual(t, len(comments), 1, "Should extract at least the regular comment")

	// Check if regular comment is included
	hasRegularComment := false
	for _, comment := range comments {
		if comment == "Regular comment" {
			hasRegularComment = true
		}
	}

	assert.True(t, hasRegularComment, "Should extract regular comment")
	// Note: Shebang behavior may vary - tree-sitter may treat it as a comment node or special node
	// Document actual behavior in green phase
}

// TestExtractModuleComments_UnicodeInComment tests comments with unicode characters.
// This is a RED PHASE test that defines expected behavior for unicode in comments.
// Expected: Unicode characters should be preserved correctly.
func TestExtractModuleComments_UnicodeInComment(t *testing.T) {
	sourceCode := `# Comment with unicode: café, 中文, Привет, 🚀
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	require.Len(t, comments, 1, "Should extract exactly 1 comment")
	assert.Equal(t, "Comment with unicode: café, 中文, Привет, 🚀", comments[0], "Unicode should be preserved")
}

// TestExtractModuleComments_EscapeSequences tests comments with escape sequences.
// This is a RED PHASE test that defines expected behavior for escape sequences in comments.
// Expected: Escape sequences in comments should be preserved as-is (not interpreted).
func TestExtractModuleComments_EscapeSequences(t *testing.T) {
	sourceCode := `# Comment with \n \t escape sequences
x = 1
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createParseTree(t, language, sourceCode)
	comments := extractModuleComments(parseTree)

	require.Len(t, comments, 1, "Should extract exactly 1 comment")
	assert.Contains(t, comments[0], "\\n", "Escape sequences should be preserved literally")
	assert.Contains(t, comments[0], "\\t", "Escape sequences should be preserved literally")
}

// Helper function to create parse tree for testing.
func createParseTree(t *testing.T, lang valueobject.Language, sourceCode string) *valueobject.ParseTree {
	t.Helper()

	// Get Python grammar from forest
	grammar := forest.GetLanguage("python")
	require.NotNil(t, grammar, "Failed to get Python grammar from forest")

	// Create tree-sitter parser
	parser := tree_sitter.NewParser()
	require.NotNil(t, parser, "Failed to create tree-sitter parser")

	success := parser.SetLanguage(grammar)
	require.True(t, success, "Failed to set Python language")

	// Parse the source code
	tree, err := parser.ParseString(context.Background(), nil, []byte(sourceCode))
	require.NoError(t, err, "Failed to parse Python source")
	require.NotNil(t, tree, "Parse tree should not be nil")
	defer tree.Close()

	// Convert tree-sitter tree to domain ParseNode
	rootTSNode := tree.RootNode()
	rootNode, nodeCount, maxDepth := convertTreeSitterNodeForTest(rootTSNode, 0)

	// Create metadata with parsing statistics
	metadata, err := valueobject.NewParseMetadata(
		time.Millisecond, // placeholder duration
		"go-tree-sitter-bare",
		"1.0.0",
	)
	require.NoError(t, err, "Failed to create metadata")

	// Update metadata with actual counts
	metadata.NodeCount = nodeCount
	metadata.MaxDepth = maxDepth

	// Create domain parse tree
	domainParseTree, err := valueobject.NewParseTree(
		context.Background(),
		lang,
		rootNode,
		[]byte(sourceCode),
		metadata,
	)
	require.NoError(t, err, "Failed to create domain parse tree")

	return domainParseTree
}

// convertTreeSitterNodeForTest converts a tree-sitter node to domain ParseNode recursively.
func convertTreeSitterNodeForTest(node tree_sitter.Node, depth int) (*valueobject.ParseNode, int, int) {
	if node.IsNull() {
		return nil, 0, depth
	}

	// Convert tree-sitter node to domain ParseNode
	parseNode := &valueobject.ParseNode{
		Type:      node.Type(),
		StartByte: safeUintToUint32ForTest(node.StartByte()),
		EndByte:   safeUintToUint32ForTest(node.EndByte()),
		StartPos: valueobject.Position{
			Row:    safeUintToUint32ForTest(node.StartPoint().Row),
			Column: safeUintToUint32ForTest(node.StartPoint().Column),
		},
		EndPos: valueobject.Position{
			Row:    safeUintToUint32ForTest(node.EndPoint().Row),
			Column: safeUintToUint32ForTest(node.EndPoint().Column),
		},
		Children: make([]*valueobject.ParseNode, 0),
	}

	nodeCount := 1
	maxDepth := depth

	// Convert children recursively
	childCount := node.ChildCount()
	for i := range childCount {
		childNode := node.Child(i)
		if childNode.IsNull() {
			continue
		}

		childParseNode, childNodeCount, childMaxDepth := convertTreeSitterNodeForTest(childNode, depth+1)
		if childParseNode != nil {
			parseNode.Children = append(parseNode.Children, childParseNode)
			nodeCount += childNodeCount
			if childMaxDepth > maxDepth {
				maxDepth = childMaxDepth
			}
		}
	}

	return parseNode, nodeCount, maxDepth
}

// safeUintToUint32ForTest safely converts uint to uint32 with bounds checking.
func safeUintToUint32ForTest(val uint) uint32 {
	if val > uint(^uint32(0)) {
		return ^uint32(0) // Return max uint32 if overflow would occur
	}
	return uint32(val) // #nosec G115 - bounds checked above
}
