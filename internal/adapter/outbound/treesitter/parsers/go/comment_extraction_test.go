package goparser

import (
	"codechunking/internal/domain/valueobject"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTreeSitterQueryEngine_QueryComments tests basic comment extraction using tree-sitter
// instead of manual string parsing. This test will fail because QueryComments method
// doesn't exist in the TreeSitterQueryEngine interface yet.
func TestTreeSitterQueryEngine_QueryComments(t *testing.T) {
	tests := []struct {
		name          string
		sourceCode    string
		expectedCount int
		expectedTexts []string // Expected comment text without markers
	}{
		{
			name: "line comments",
			sourceCode: `package main

// This is a line comment
// Another line comment
func main() {}
`,
			expectedCount: 2,
			expectedTexts: []string{"This is a line comment", "Another line comment"},
		},
		{
			name: "block comments",
			sourceCode: `package main

/* This is a block comment */
/*
Multi-line
block comment
*/
func main() {}
`,
			expectedCount: 2,
			expectedTexts: []string{"This is a block comment", "Multi-line block comment"},
		},
		{
			name: "mixed comment types",
			sourceCode: `package main

// Line comment
/* Block comment */
// Another line comment
func main() {}
`,
			expectedCount: 3,
			expectedTexts: []string{"Line comment", "Block comment", "Another line comment"},
		},
		{
			name: "package documentation",
			sourceCode: `// Package main demonstrates comment extraction.
// It shows how tree-sitter can properly parse comments.
package main

func main() {}
`,
			expectedCount: 2,
			expectedTexts: []string{
				"Package main demonstrates comment extraction.",
				"It shows how tree-sitter can properly parse comments.",
			},
		},
	}

	engine := NewTreeSitterQueryEngine()
	require.NotNil(t, engine, "TreeSitterQueryEngine should be implemented")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree := createParseTree(t, tt.sourceCode)

			// This test will FAIL because QueryComments method doesn't exist yet
			commentNodes := engine.QueryComments(parseTree)

			assert.Len(t, commentNodes, tt.expectedCount)

			for i, expectedText := range tt.expectedTexts {
				require.Less(t, i, len(commentNodes))

				// Verify the node type is correct
				assert.Equal(t, "comment", commentNodes[i].Type)

				// Verify the comment text is properly processed without manual string manipulation
				actualText := engine.ProcessCommentText(parseTree, commentNodes[i])
				assert.Equal(t, expectedText, actualText)

				// Verify byte positions are valid
				assert.GreaterOrEqual(t, commentNodes[i].StartByte, uint32(0))
				assert.Greater(t, commentNodes[i].EndByte, commentNodes[i].StartByte)
				assert.LessOrEqual(t, commentNodes[i].EndByte, uint32(len(tt.sourceCode)))
			}
		})
	}
}

// TestTreeSitterQueryEngine_QueryDocumentationComments tests tree-sitter-based
// comment-declaration associations instead of manual position-based logic.
// This test will fail because QueryDocumentationComments method doesn't exist yet.
func TestTreeSitterQueryEngine_QueryDocumentationComments(t *testing.T) {
	tests := []struct {
		name             string
		sourceCode       string
		declarationType  string // "function", "method", "type"
		declarationName  string
		expectedComments []string
	}{
		{
			name: "function with documentation",
			sourceCode: `package main

// Add calculates the sum of two integers.
// It returns the result as an integer.
func Add(a, b int) int {
	return a + b
}
`,
			declarationType: "function",
			declarationName: "Add",
			expectedComments: []string{
				"Add calculates the sum of two integers.",
				"It returns the result as an integer.",
			},
		},
		{
			name: "type with documentation",
			sourceCode: `package main

// Person represents a user in the system.
// It contains basic identification information.
type Person struct {
	Name string
	Age  int
}
`,
			declarationType: "type",
			declarationName: "Person",
			expectedComments: []string{
				"Person represents a user in the system.",
				"It contains basic identification information.",
			},
		},
		{
			name: "method with documentation",
			sourceCode: `package main

type Counter struct {
	value int
}

// Increment increases the counter value by 1.
// It modifies the counter in-place.
func (c *Counter) Increment() {
	c.value++
}
`,
			declarationType: "method",
			declarationName: "Increment",
			expectedComments: []string{
				"Increment increases the counter value by 1.",
				"It modifies the counter in-place.",
			},
		},
		{
			name: "declaration without comments",
			sourceCode: `package main

func NoComments() {
	// This internal comment should not be associated
}
`,
			declarationType:  "function",
			declarationName:  "NoComments",
			expectedComments: []string{}, // No documentation comments
		},
		{
			name: "comments separated by blank lines should not be associated",
			sourceCode: `package main

// This comment is separated by a blank line.

func SeparatedComments() {}
`,
			declarationType:  "function",
			declarationName:  "SeparatedComments",
			expectedComments: []string{}, // Should not be associated due to blank line
		},
	}

	engine := NewTreeSitterQueryEngine()
	require.NotNil(t, engine, "TreeSitterQueryEngine should be implemented")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree := createParseTree(t, tt.sourceCode)

			// Find the declaration node using existing query methods
			var declarationNode *valueobject.ParseNode
			switch tt.declarationType {
			case "function":
				functions := engine.QueryFunctionDeclarations(parseTree)
				for _, fn := range functions {
					if getDeclarationName(parseTree, fn) == tt.declarationName {
						declarationNode = fn
						break
					}
				}
			case "method":
				methods := engine.QueryMethodDeclarations(parseTree)
				for _, method := range methods {
					if getDeclarationName(parseTree, method) == tt.declarationName {
						declarationNode = method
						break
					}
				}
			case "type":
				types := engine.QueryTypeDeclarations(parseTree)
				for _, typ := range types {
					if getDeclarationName(parseTree, typ) == tt.declarationName {
						declarationNode = typ
						break
					}
				}
			}

			require.NotNil(t, declarationNode, "Declaration node should be found")

			// This test will FAIL because QueryDocumentationComments method doesn't exist yet
			// This method should use tree-sitter comment associations instead of manual position logic
			documentationComments := engine.QueryDocumentationComments(parseTree, declarationNode)

			assert.Len(t, documentationComments, len(tt.expectedComments))

			for i, expectedComment := range tt.expectedComments {
				require.Less(t, i, len(documentationComments))

				// Verify comment content without manual string manipulation
				actualComment := engine.ProcessCommentText(parseTree, documentationComments[i])
				assert.Equal(t, expectedComment, actualComment)
			}
		})
	}
}

// TestTreeSitterQueryEngine_CommentAssociation tests that comments are properly
// associated with their adjacent declarations using tree-sitter grammar features
// instead of manual byte position comparisons.
func TestTreeSitterQueryEngine_CommentAssociation(t *testing.T) {
	tests := []struct {
		name              string
		sourceCode        string
		expectedFunctions map[string][]string // function name -> associated comments
		expectedTypes     map[string][]string // type name -> associated comments
	}{
		{
			name: "multiple functions with comments",
			sourceCode: `package main

// First function does something.
func First() {}

// Second function does something else.
// It has multiple comment lines.
func Second() {}

func Third() {} // This should not be associated

// Fourth function comment.
func Fourth() {}
`,
			expectedFunctions: map[string][]string{
				"First":  {"First function does something."},
				"Second": {"Second function does something else.", "It has multiple comment lines."},
				"Third":  {}, // No preceding documentation
				"Fourth": {"Fourth function comment."},
			},
		},
		{
			name: "mixed declarations with comments",
			sourceCode: `package main

// UserType represents a user.
type UserType struct {
	ID int
}

// ProcessUser handles user processing.
func ProcessUser(user UserType) {}

// UserCount keeps track of users.
var UserCount int

// StatusActive represents active status.
const StatusActive = "active"
`,
			expectedTypes: map[string][]string{
				"UserType": {"UserType represents a user."},
			},
			expectedFunctions: map[string][]string{
				"ProcessUser": {"ProcessUser handles user processing."},
			},
		},
		{
			name: "block comments with functions",
			sourceCode: `package main

/*
BlockCommented is a function with block comment documentation.
It demonstrates block comment handling.
*/
func BlockCommented() {}

/* Single line block comment */
func SingleBlockComment() {}
`,
			expectedFunctions: map[string][]string{
				"BlockCommented": {
					"BlockCommented is a function with block comment documentation. It demonstrates block comment handling.",
				},
				"SingleBlockComment": {"Single line block comment"},
			},
		},
	}

	engine := NewTreeSitterQueryEngine()
	require.NotNil(t, engine, "TreeSitterQueryEngine should be implemented")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree := createParseTree(t, tt.sourceCode)

			// Test function comment associations
			if tt.expectedFunctions != nil {
				functions := engine.QueryFunctionDeclarations(parseTree)

				for _, functionNode := range functions {
					functionName := getDeclarationName(parseTree, functionNode)
					expectedComments, exists := tt.expectedFunctions[functionName]

					if exists {
						// This test will FAIL because QueryDocumentationComments method doesn't exist
						// and current implementation uses manual position-based logic
						actualComments := engine.QueryDocumentationComments(parseTree, functionNode)

						assert.Len(t, actualComments, len(expectedComments),
							"Function %s should have %d documentation comments", functionName, len(expectedComments))

						for i, expectedComment := range expectedComments {
							if i < len(actualComments) {
								actualComment := engine.ProcessCommentText(parseTree, actualComments[i])
								assert.Equal(t, expectedComment, actualComment,
									"Comment %d for function %s", i, functionName)
							}
						}
					}
				}
			}

			// Test type comment associations
			if tt.expectedTypes != nil {
				types := engine.QueryTypeDeclarations(parseTree)

				for _, typeNode := range types {
					typeName := getDeclarationName(parseTree, typeNode)
					expectedComments, exists := tt.expectedTypes[typeName]

					if exists {
						// This test will FAIL because QueryDocumentationComments method doesn't exist
						actualComments := engine.QueryDocumentationComments(parseTree, typeNode)

						assert.Len(t, actualComments, len(expectedComments),
							"Type %s should have %d documentation comments", typeName, len(expectedComments))

						for i, expectedComment := range expectedComments {
							if i < len(actualComments) {
								actualComment := engine.ProcessCommentText(parseTree, actualComments[i])
								assert.Equal(t, expectedComment, actualComment,
									"Comment %d for type %s", i, typeName)
							}
						}
					}
				}
			}
		})
	}
}

// TestTreeSitterQueryEngine_ProcessCommentText tests that comment text processing
// uses tree-sitter features instead of manual string manipulation.
// This test will fail because ProcessCommentText method doesn't exist yet.
func TestTreeSitterQueryEngine_ProcessCommentText(t *testing.T) {
	tests := []struct {
		name         string
		rawComment   string
		expectedText string
	}{
		{
			name:         "single line comment",
			rawComment:   "// This is a comment",
			expectedText: "This is a comment",
		},
		{
			name:         "block comment",
			rawComment:   "/* This is a block comment */",
			expectedText: "This is a block comment",
		},
		{
			name: "multi-line block comment",
			rawComment: `/* This is a
multi-line
block comment */`,
			expectedText: "This is a multi-line block comment",
		},
		{
			name:         "comment with extra whitespace",
			rawComment:   "//   Spaced comment   ",
			expectedText: "Spaced comment",
		},
		{
			name: "block comment with internal formatting",
			rawComment: `/*
 * This is a formatted
 * block comment
 */`,
			expectedText: "This is a formatted block comment",
		},
	}

	engine := NewTreeSitterQueryEngine()
	require.NotNil(t, engine, "TreeSitterQueryEngine should be implemented")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal source file with the comment
			sourceCode := "package main\n\n" + tt.rawComment + "\nfunc main() {}\n"
			parseTree := createParseTree(t, sourceCode)

			// Get the comment node
			commentNodes := engine.QueryComments(parseTree)
			require.Len(t, commentNodes, 1, "Should find exactly one comment")

			// This test will FAIL because ProcessCommentText method doesn't exist yet
			// This method should use tree-sitter comment processing instead of manual string manipulation
			actualText := engine.ProcessCommentText(parseTree, commentNodes[0])
			assert.Equal(t, tt.expectedText, actualText)
		})
	}
}

// TestTreeSitterQueryEngine_CommentEdgeCases tests edge cases for comment extraction
// that should be handled properly by tree-sitter instead of manual logic.
func TestTreeSitterQueryEngine_CommentEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		sourceCode  string
		description string
	}{
		{
			name: "nested comments in code",
			sourceCode: `package main

// Function documentation
func Process() {
	// Internal comment - should not be documentation
	if true {
		/* Block comment inside function */
	}
}
`,
			description: "Internal comments should not be associated with function documentation",
		},
		{
			name: "comments between imports and functions",
			sourceCode: `package main

import "fmt"

// This comment is after imports
// Should be associated with function
func main() {
	fmt.Println("hello")
}
`,
			description: "Comments between imports and functions should be properly associated",
		},
		{
			name: "multiple comment blocks",
			sourceCode: `package main

// First comment block
// continues here

// Second comment block
// after blank line
func TestFunction() {}
`,
			description: "Only immediately preceding comment block should be associated",
		},
		{
			name: "inline comments",
			sourceCode: `package main

func Process() int { // Inline comment
	return 42 // Another inline comment
}
`,
			description: "Inline comments should not be treated as documentation",
		},
	}

	engine := NewTreeSitterQueryEngine()
	require.NotNil(t, engine, "TreeSitterQueryEngine should be implemented")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree := createParseTree(t, tt.sourceCode)

			// This test will FAIL because the methods don't exist yet
			// Test that comment extraction works correctly for edge cases
			allComments := engine.QueryComments(parseTree)
			assert.NotNil(t, allComments, "Should return valid comment slice")

			functions := engine.QueryFunctionDeclarations(parseTree)
			for _, functionNode := range functions {
				docComments := engine.QueryDocumentationComments(parseTree, functionNode)
				assert.NotNil(t, docComments, "Should return valid documentation comment slice")

				// Each comment should be processable without manual string manipulation
				for _, comment := range docComments {
					processedText := engine.ProcessCommentText(parseTree, comment)
					assert.NotEmpty(t, processedText, "Processed comment text should not be empty")
				}
			}

			t.Logf("Test case: %s - %s", tt.name, tt.description)
		})
	}
}

// TestGoParser_ExtractPrecedingComments_ShouldUseTreeSitter tests that the existing
// extractPrecedingComments method should be refactored to use TreeSitterQueryEngine
// instead of manual position-based logic.
func TestGoParser_ExtractPrecedingComments_ShouldUseTreeSitter(t *testing.T) {
	tests := []struct {
		name             string
		sourceCode       string
		expectedComments []string
	}{
		{
			name: "function with preceding comments",
			sourceCode: `package main

// This function adds two numbers.
// It returns the sum.
func Add(a, b int) int {
	return a + b
}
`,
			expectedComments: []string{
				"This function adds two numbers.",
				"It returns the sum.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree := createParseTree(t, tt.sourceCode)
			parser := &GoParser{}

			// Get the function node
			engine := NewTreeSitterQueryEngine()
			functions := engine.QueryFunctionDeclarations(parseTree)
			require.Len(t, functions, 1)
			functionNode := functions[0]

			// This test will FAIL because extractPrecedingComments currently uses manual position logic
			// Instead of byte position comparison, it should use TreeSitterQueryEngine.QueryDocumentationComments
			actualComments := parser.extractPrecedingComments(functionNode, parseTree)

			// The current implementation will likely return raw comments with markers
			// The expected behavior is to return clean comment text using tree-sitter processing
			assert.Len(t, actualComments, len(tt.expectedComments))
			for i, expectedComment := range tt.expectedComments {
				if i < len(actualComments) {
					// This assertion will fail because current implementation returns raw comment text
					assert.Equal(t, expectedComment, actualComments[i])
				}
			}
		})
	}
}

// TestExtractDocumentationForTypeDeclaration_ShouldUseTreeSitter tests that the existing
// extractDocumentationForTypeDeclaration function should be refactored to use
// TreeSitterQueryEngine instead of manual sibling traversal.
func TestExtractDocumentationForTypeDeclaration_ShouldUseTreeSitter(t *testing.T) {
	tests := []struct {
		name            string
		sourceCode      string
		typeName        string
		expectedComment string
	}{
		{
			name: "type with documentation",
			sourceCode: `package main

// User represents a system user.
// It contains basic user information.
type User struct {
	Name string
	Age  int
}
`,
			typeName:        "User",
			expectedComment: "User represents a system user. It contains basic user information.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree := createParseTree(t, tt.sourceCode)

			// Get the type declaration node
			engine := NewTreeSitterQueryEngine()
			types := engine.QueryTypeDeclarations(parseTree)
			require.Len(t, types, 1)
			typeNode := types[0]

			// This test will FAIL because extractDocumentationForTypeDeclaration uses manual sibling traversal
			// Instead of manually walking backwards through siblings, it should use TreeSitterQueryEngine
			actualComment := extractDocumentationForTypeDeclaration(parseTree, typeNode)

			// The current implementation uses manual string processing and sibling traversal
			// The expected behavior is to use tree-sitter comment associations
			assert.Equal(t, tt.expectedComment, actualComment)
		})
	}
}

// Helper function to extract declaration names from nodes.
func getDeclarationName(parseTree *valueobject.ParseTree, declarationNode *valueobject.ParseNode) string {
	// Handle different declaration types
	switch declarationNode.Type {
	case "function_declaration", "method_declaration":
		// For functions and methods, look for identifier
		for _, child := range declarationNode.Children {
			if child.Type == "identifier" {
				return parseTree.GetNodeText(child)
			}
		}
	case "type_declaration":
		// For type declarations, look for type_spec and then identifier
		for _, child := range declarationNode.Children {
			if child.Type == "type_spec" {
				for _, grandchild := range child.Children {
					if grandchild.Type == "type_identifier" {
						return parseTree.GetNodeText(grandchild)
					}
				}
			}
		}
	}

	// Fallback: look for any identifier
	for _, child := range declarationNode.Children {
		if child.Type == "identifier" || child.Type == "field_identifier" || child.Type == "type_identifier" {
			return parseTree.GetNodeText(child)
		}
	}
	return ""
}
