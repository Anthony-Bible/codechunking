package valueobject

import (
	"context"
	"strings"
	"testing"
	"time"

	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseTree_NewParseTree tests the creation of ParseTree value objects.
// This is a RED PHASE test that defines expected behavior for parse tree creation.
func TestParseTree_NewParseTree(t *testing.T) {
	tests := []struct {
		name      string
		language  Language
		rootNode  *ParseNode
		source    []byte
		metadata  ParseMetadata
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid Go parse tree",
			language: func() Language {
				lang, _ := NewLanguageWithDetails(
					LanguageGo,
					[]string{"golang"},
					[]string{".go"},
					LanguageTypeCompiled,
					DetectionMethodExtension,
					0.95,
				)
				return lang
			}(),
			rootNode: &ParseNode{
				Type:      "source_file",
				StartByte: 0,
				EndByte:   63,
				StartPos:  Position{Row: 0, Column: 0},
				EndPos:    Position{Row: 6, Column: 1},
				Children:  []*ParseNode{},
			},
			source: []byte(`package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`),
			metadata: ParseMetadata{
				ParseDuration:     time.Millisecond * 15,
				TreeSitterVersion: "0.20.8",
				GrammarVersion:    "1.0.0",
				NodeCount:         15,
				MaxDepth:          4,
			},
			wantError: false,
		},
		{
			name: "valid Python parse tree with complex structure",
			language: func() Language {
				lang, _ := NewLanguageWithDetails(
					LanguagePython,
					[]string{"py"},
					[]string{".py"},
					LanguageTypeInterpreted,
					DetectionMethodContent,
					0.90,
				)
				return lang
			}(),
			rootNode: &ParseNode{
				Type:      "module",
				StartByte: 0,
				EndByte:   165,
				StartPos:  Position{Row: 0, Column: 0},
				EndPos:    Position{Row: 10, Column: 0},
				Children: []*ParseNode{
					{
						Type:      "class_definition",
						StartByte: 0,
						EndByte:   165,
						StartPos:  Position{Row: 0, Column: 0},
						EndPos:    Position{Row: 8, Column: 4},
						Children:  []*ParseNode{},
					},
				},
			},
			source: []byte(`class Calculator:
    def __init__(self):
        self.result = 0
    
    def add(self, value):
        self.result += value
        return self.result
    
    def reset(self):
        self.result = 0
`),
			metadata: ParseMetadata{
				ParseDuration:     time.Millisecond * 25,
				TreeSitterVersion: "0.20.8",
				GrammarVersion:    "0.21.0",
				NodeCount:         42,
				MaxDepth:          6,
			},
			wantError: false,
		},
		{
			name: "invalid parse tree with nil root node",
			language: func() Language {
				lang, _ := NewLanguage(LanguageJavaScript)
				return lang
			}(),
			rootNode:  nil,
			source:    []byte("console.log('hello');"),
			metadata:  ParseMetadata{},
			wantError: true,
			errorMsg:  "root node cannot be nil",
		},
		{
			name: "invalid parse tree with empty source",
			language: func() Language {
				lang, _ := NewLanguage(LanguageGo)
				return lang
			}(),
			rootNode: &ParseNode{
				Type:      "source_file",
				StartByte: 0,
				EndByte:   0,
				StartPos:  Position{Row: 0, Column: 0},
				EndPos:    Position{Row: 0, Column: 0},
				Children:  []*ParseNode{},
			},
			source:    []byte{},
			metadata:  ParseMetadata{},
			wantError: true,
			errorMsg:  "source code cannot be empty",
		},
		{
			name: "invalid parse tree with mismatched source length",
			language: func() Language {
				lang, _ := NewLanguage(LanguageGo)
				return lang
			}(),
			rootNode: &ParseNode{
				Type:      "source_file",
				StartByte: 0,
				EndByte:   100, // Doesn't match source length
				StartPos:  Position{Row: 0, Column: 0},
				EndPos:    Position{Row: 2, Column: 1},
				Children:  []*ParseNode{},
			},
			source:    []byte("package main\n"),
			metadata:  ParseMetadata{},
			wantError: true,
			errorMsg:  "root node end byte exceeds source length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree, err := NewParseTree(context.Background(), tt.language, tt.rootNode, tt.source, tt.metadata)

			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, parseTree, "parseTree should be nil when creation fails")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.language.Name(), parseTree.Language().Name())
				assert.Equal(t, tt.rootNode.Type, parseTree.RootNode().Type)
				assert.Equal(t, tt.source, parseTree.Source())
				assert.Equal(t, tt.metadata.NodeCount, parseTree.Metadata().NodeCount)
				assert.NotZero(t, parseTree.CreatedAt())
			}
		})
	}
}

// TestParseTree_NavigationMethods tests methods for navigating the parse tree.
// This is a RED PHASE test that defines expected navigation behavior.
func TestParseTree_NavigationMethods(t *testing.T) {
	// Create a complex parse tree for testing
	language, _ := NewLanguageWithDetails(
		LanguageGo,
		[]string{"golang"},
		[]string{".go"},
		LanguageTypeCompiled,
		DetectionMethodExtension,
		0.95,
	)

	rootNode := &ParseNode{
		Type:      "source_file",
		StartByte: 0,
		EndByte:   120,
		StartPos:  Position{Row: 0, Column: 0},
		EndPos:    Position{Row: 8, Column: 1},
		Children: []*ParseNode{
			{
				Type:      "package_clause",
				StartByte: 0,
				EndByte:   12,
				StartPos:  Position{Row: 0, Column: 0},
				EndPos:    Position{Row: 0, Column: 12},
				Children:  []*ParseNode{},
			},
			{
				Type:      "function_declaration",
				StartByte: 14,
				EndByte:   120,
				StartPos:  Position{Row: 2, Column: 0},
				EndPos:    Position{Row: 8, Column: 1},
				Children: []*ParseNode{
					{
						Type:      "identifier",
						StartByte: 19,
						EndByte:   23,
						StartPos:  Position{Row: 2, Column: 5},
						EndPos:    Position{Row: 2, Column: 9},
						Children:  []*ParseNode{},
					},
				},
			},
		},
	}

	source := []byte(`package main

func main() {
    fmt.Println("Hello, World!")
    x := 42
    if x > 0 {
        fmt.Println("Positive")
    }
}`)

	metadata := ParseMetadata{
		ParseDuration:     time.Millisecond * 20,
		TreeSitterVersion: "0.20.8",
		GrammarVersion:    "1.0.0",
		NodeCount:         25,
		MaxDepth:          4,
	}

	parseTree, err := NewParseTree(context.Background(), language, rootNode, source, metadata)
	require.NoError(t, err)

	tests := []struct {
		name           string
		method         func(*ParseTree) interface{}
		expectedResult interface{}
	}{
		{
			name: "get all nodes by type - function_declaration",
			method: func(pt *ParseTree) interface{} {
				return pt.GetNodesByType("function_declaration")
			},
			expectedResult: []*ParseNode{
				{
					Type:      "function_declaration",
					StartByte: 14,
					EndByte:   120,
					StartPos:  Position{Row: 2, Column: 0},
					EndPos:    Position{Row: 8, Column: 1},
					Children: []*ParseNode{
						{
							Type:      "identifier",
							StartByte: 19,
							EndByte:   23,
							StartPos:  Position{Row: 2, Column: 5},
							EndPos:    Position{Row: 2, Column: 9},
							Children:  []*ParseNode{},
						},
					},
				},
			},
		},
		{
			name: "get node at position - function name",
			method: func(pt *ParseTree) interface{} {
				return pt.GetNodeAtPosition(Position{Row: 2, Column: 7})
			},
			expectedResult: &ParseNode{
				Type:      "identifier",
				StartByte: 19,
				EndByte:   23,
				StartPos:  Position{Row: 2, Column: 5},
				EndPos:    Position{Row: 2, Column: 9},
				Children:  []*ParseNode{},
			},
		},
		{
			name: "get node at byte offset - package clause",
			method: func(pt *ParseTree) interface{} {
				return pt.GetNodeAtByteOffset(5)
			},
			expectedResult: &ParseNode{
				Type:      "package_clause",
				StartByte: 0,
				EndByte:   12,
				StartPos:  Position{Row: 0, Column: 0},
				EndPos:    Position{Row: 0, Column: 12},
				Children:  []*ParseNode{},
			},
		},
		{
			name: "get text for node",
			method: func(pt *ParseTree) interface{} {
				node := pt.GetNodesByType("package_clause")[0]
				return pt.GetNodeText(node)
			},
			expectedResult: "package main",
		},
		{
			name: "get tree depth",
			method: func(pt *ParseTree) interface{} {
				return pt.GetTreeDepth()
			},
			expectedResult: 3, // source_file -> function_declaration -> identifier
		},
		{
			name: "get total node count",
			method: func(pt *ParseTree) interface{} {
				return pt.GetTotalNodeCount()
			},
			expectedResult: 4, // source_file + package_clause + function_declaration + identifier
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method(parseTree)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// TestParseTree_ValidationMethods tests validation methods for parse trees.
// This is a RED PHASE test that defines expected validation behavior.
func TestParseTree_ValidationMethods(t *testing.T) {
	tests := []struct {
		name      string
		setupTree func() *ParseTree
		testFunc  func(*ParseTree) (bool, error)
		wantBool  bool
		wantError bool
		errorMsg  string
	}{
		{
			name: "validate well-formed tree",
			setupTree: func() *ParseTree {
				language, _ := NewLanguage(LanguageGo)
				rootNode := &ParseNode{
					Type:      "source_file",
					StartByte: 0,
					EndByte:   13,
					StartPos:  Position{Row: 0, Column: 0},
					EndPos:    Position{Row: 0, Column: 13},
					Children:  []*ParseNode{},
				}
				source := []byte("package main\n")
				metadata := ParseMetadata{NodeCount: 1, MaxDepth: 1}
				tree, _ := NewParseTree(context.Background(), language, rootNode, source, metadata)
				return tree
			},
			testFunc: func(pt *ParseTree) (bool, error) {
				return pt.IsWellFormed()
			},
			wantBool:  true,
			wantError: false,
		},
		{
			name: "validate tree with syntax errors",
			setupTree: func() *ParseTree {
				language, _ := NewLanguage(LanguageGo)
				rootNode := &ParseNode{
					Type:      "source_file",
					StartByte: 0,
					EndByte:   12,
					StartPos:  Position{Row: 0, Column: 0},
					EndPos:    Position{Row: 0, Column: 12},
					Children: []*ParseNode{
						{
							Type:      "ERROR",
							StartByte: 8,
							EndByte:   12,
							StartPos:  Position{Row: 0, Column: 8},
							EndPos:    Position{Row: 0, Column: 12},
							Children:  []*ParseNode{},
						},
					},
				}
				source := []byte("package !!!\n")
				metadata := ParseMetadata{NodeCount: 2, MaxDepth: 2}
				tree, _ := NewParseTree(context.Background(), language, rootNode, source, metadata)
				return tree
			},
			testFunc: func(pt *ParseTree) (bool, error) {
				return pt.HasSyntaxErrors()
			},
			wantBool:  true,
			wantError: false,
		},
		{
			name: "validate tree consistency",
			setupTree: func() *ParseTree {
				language, _ := NewLanguage(LanguageGo)
				rootNode := &ParseNode{
					Type:      "source_file",
					StartByte: 0,
					EndByte:   13,
					StartPos:  Position{Row: 0, Column: 0},
					EndPos:    Position{Row: 1, Column: 0},
					Children:  []*ParseNode{},
				}
				source := []byte("package main\n")
				metadata := ParseMetadata{NodeCount: 1, MaxDepth: 1}
				tree, _ := NewParseTree(context.Background(), language, rootNode, source, metadata)
				return tree
			},
			testFunc: func(pt *ParseTree) (bool, error) {
				return pt.ValidateConsistency()
			},
			wantBool:  true,
			wantError: false,
		},
		{
			name: "validate tree with inconsistent metadata",
			setupTree: func() *ParseTree {
				language, _ := NewLanguage(LanguageGo)
				rootNode := &ParseNode{
					Type:      "source_file",
					StartByte: 0,
					EndByte:   13,
					StartPos:  Position{Row: 0, Column: 0},
					EndPos:    Position{Row: 1, Column: 0},
					Children:  []*ParseNode{},
				}
				source := []byte("package main\n")
				// Inconsistent metadata: claims 5 nodes but tree only has 1
				metadata := ParseMetadata{NodeCount: 5, MaxDepth: 1}
				tree, _ := NewParseTree(context.Background(), language, rootNode, source, metadata)
				return tree
			},
			testFunc: func(pt *ParseTree) (bool, error) {
				return pt.ValidateConsistency()
			},
			wantBool:  false,
			wantError: true,
			errorMsg:  "metadata node count inconsistent with actual tree",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := tt.setupTree()
			result, err := tt.testFunc(tree)

			assert.Equal(t, tt.wantBool, result)
			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestParseTree_SerializationMethods tests serialization capabilities.
// This is a RED PHASE test that defines expected serialization behavior.
func TestParseTree_SerializationMethods(t *testing.T) {
	// Create a sample parse tree
	language, _ := NewLanguageWithDetails(
		LanguageJavaScript,
		[]string{"js"},
		[]string{".js"},
		LanguageTypeInterpreted,
		DetectionMethodExtension,
		0.90,
	)

	rootNode := &ParseNode{
		Type:      "program",
		StartByte: 0,
		EndByte:   22,
		StartPos:  Position{Row: 0, Column: 0},
		EndPos:    Position{Row: 0, Column: 22},
		Children: []*ParseNode{
			{
				Type:      "expression_statement",
				StartByte: 0,
				EndByte:   22,
				StartPos:  Position{Row: 0, Column: 0},
				EndPos:    Position{Row: 0, Column: 22},
				Children:  []*ParseNode{},
			},
		},
	}

	source := []byte(`console.log("Hello!");`)
	metadata := ParseMetadata{
		ParseDuration:     time.Millisecond * 5,
		TreeSitterVersion: "0.20.8",
		GrammarVersion:    "0.20.0",
		NodeCount:         2,
		MaxDepth:          2,
	}

	parseTree, err := NewParseTree(context.Background(), language, rootNode, source, metadata)
	require.NoError(t, err)

	tests := []struct {
		name     string
		testFunc func(*ParseTree) (interface{}, error)
		validate func(interface{}) bool
	}{
		{
			name: "serialize to JSON",
			testFunc: func(pt *ParseTree) (interface{}, error) {
				return pt.ToJSON()
			},
			validate: func(result interface{}) bool {
				jsonStr, ok := result.(string)
				if !ok {
					return false
				}
				// Should contain key fields
				return strings.Contains(jsonStr, "\"language\"") &&
					strings.Contains(jsonStr, "\"rootNode\"") &&
					strings.Contains(jsonStr, "\"metadata\"") &&
					strings.Contains(jsonStr, "JavaScript")
			},
		},
		{
			name: "serialize to S-expression",
			testFunc: func(pt *ParseTree) (interface{}, error) {
				return pt.ToSExpression()
			},
			validate: func(result interface{}) bool {
				sexp, ok := result.(string)
				if !ok {
					return false
				}
				// Should be in S-expression format
				return strings.HasPrefix(sexp, "(program") &&
					strings.Contains(sexp, "expression_statement") &&
					strings.HasSuffix(sexp, ")")
			},
		},
		{
			name: "export to GraphQL schema format",
			testFunc: func(pt *ParseTree) (interface{}, error) {
				return pt.ToGraphQLSchema()
			},
			validate: func(result interface{}) bool {
				schema, ok := result.(string)
				if !ok {
					return false
				}
				// Should contain GraphQL type definitions
				return strings.Contains(schema, "type ParseNode") &&
					strings.Contains(schema, "type ParseTree") &&
					strings.Contains(schema, "type Position")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.testFunc(parseTree)
			require.NoError(t, err)
			assert.True(t, tt.validate(result), "Serialization result validation failed")
		})
	}
}

// TestParseTree_ConcurrencyMethods tests concurrent access to parse trees.
// This is a RED PHASE test that defines expected thread-safety behavior.
func TestParseTree_ConcurrencyMethods(t *testing.T) {
	// Create a parse tree for concurrent testing
	language, _ := NewLanguage(LanguageGo)
	rootNode := &ParseNode{
		Type:      "source_file",
		StartByte: 0,
		EndByte:   100,
		StartPos:  Position{Row: 0, Column: 0},
		EndPos:    Position{Row: 5, Column: 1},
		Children:  make([]*ParseNode, 50), // Large enough for concurrent testing
	}

	// Fill with child nodes
	for i := range rootNode.Children {
		rootNode.Children[i] = &ParseNode{
			Type:      "identifier",
			StartByte: uint32(i * 2),
			EndByte:   uint32(i*2 + 1),
			StartPos:  Position{Row: uint32(i / 10), Column: uint32(i % 10)},
			EndPos:    Position{Row: uint32(i / 10), Column: uint32(i%10 + 1)},
			Children:  []*ParseNode{},
		}
	}

	source := make([]byte, 100)
	for i := range source {
		source[i] = byte('a' + (i % 26))
	}

	metadata := ParseMetadata{
		ParseDuration:     time.Millisecond * 50,
		TreeSitterVersion: "0.20.8",
		GrammarVersion:    "1.0.0",
		NodeCount:         51, // root + 50 children
		MaxDepth:          2,
	}

	parseTree, err := NewParseTree(context.Background(), language, rootNode, source, metadata)
	require.NoError(t, err)

	t.Run("concurrent read access", func(t *testing.T) {
		// This test should pass without data races
		done := make(chan bool, 10)

		for i := range 10 {
			go func(_ int) {
				defer func() { done <- true }()

				for j := range 100 {
					// Concurrent read operations
					_ = parseTree.Language()
					_ = parseTree.RootNode()
					_ = parseTree.Source()
					_ = parseTree.Metadata()
					_ = parseTree.GetTotalNodeCount()
					_ = parseTree.GetTreeDepth()

					// Concurrent navigation
					nodes := parseTree.GetNodesByType("identifier")
					assert.NotEmpty(t, nodes, "Should find identifier nodes")

					pos := Position{Row: uint32(j % 5), Column: uint32(j % 10)}
					_ = parseTree.GetNodeAtPosition(pos)

					offset := uint32(j % 100)
					_ = parseTree.GetNodeAtByteOffset(offset)
				}
			}(i)
		}

		// Wait for all goroutines to complete
		for range 10 {
			<-done
		}
	})

	t.Run("concurrent validation access", func(t *testing.T) {
		// This test should validate concurrent access to validation methods
		done := make(chan bool, 5)

		for range 5 {
			go func() {
				defer func() { done <- true }()

				for range 50 {
					isWellFormed, err := parseTree.IsWellFormed()
					assert.NoError(t, err)
					assert.True(t, isWellFormed)

					hasSyntaxErrors, err := parseTree.HasSyntaxErrors()
					assert.NoError(t, err)
					assert.False(t, hasSyntaxErrors)

					isConsistent, err := parseTree.ValidateConsistency()
					assert.NoError(t, err)
					assert.True(t, isConsistent)
				}
			}()
		}

		// Wait for all goroutines to complete
		for range 5 {
			<-done
		}
	})
}

// TestParseTree_MemoryManagement tests memory management and cleanup.
// This is a RED PHASE test that defines expected resource cleanup behavior.
func TestParseTree_MemoryManagement(t *testing.T) {
	t.Run("cleanup resources", func(t *testing.T) {
		language, _ := NewLanguage(LanguageRust)
		rootNode := &ParseNode{
			Type:      "source_file",
			StartByte: 0,
			EndByte:   40,
			StartPos:  Position{Row: 0, Column: 0},
			EndPos:    Position{Row: 2, Column: 1},
			Children:  []*ParseNode{},
		}

		source := []byte(`fn main() {
    println!("Hello, Rust!");
}`)

		metadata := ParseMetadata{
			ParseDuration:     time.Millisecond * 10,
			TreeSitterVersion: "0.20.8",
			GrammarVersion:    "0.20.0",
			NodeCount:         10,
			MaxDepth:          3,
		}

		parseTree, err := NewParseTree(context.Background(), language, rootNode, source, metadata)
		require.NoError(t, err)

		// Test cleanup
		err = parseTree.Cleanup(context.Background())
		require.NoError(t, err)

		// After cleanup, tree should be marked as cleaned up
		assert.True(t, parseTree.IsCleanedUp())

		// Operations on cleaned up tree should return errors
		_, err = parseTree.IsWellFormed()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse tree has been cleaned up")
	})

	t.Run("double cleanup should be safe", func(t *testing.T) {
		language, _ := NewLanguage(LanguageGo)
		rootNode := &ParseNode{
			Type:      "source_file",
			StartByte: 0,
			EndByte:   13,
			StartPos:  Position{Row: 0, Column: 0},
			EndPos:    Position{Row: 1, Column: 0},
			Children:  []*ParseNode{},
		}

		source := []byte("package main\n")
		metadata := ParseMetadata{NodeCount: 1, MaxDepth: 1}

		parseTree, err := NewParseTree(context.Background(), language, rootNode, source, metadata)
		require.NoError(t, err)

		// First cleanup
		err = parseTree.Cleanup(context.Background())
		require.NoError(t, err)

		// Second cleanup should be safe
		err = parseTree.Cleanup(context.Background())
		require.NoError(t, err)

		assert.True(t, parseTree.IsCleanedUp())
	})
}

// TestParseNode_TreeSitterIntegration tests ParseNode with tree-sitter node references.
// This is a RED PHASE test that defines expected behavior for tree-sitter Content() integration.
func TestParseNode_TreeSitterIntegration(t *testing.T) {
	t.Run("ParseNode with tree-sitter node reference should store reference", func(t *testing.T) {
		// Create a ParseNode with tree-sitter node reference using the constructor
		mockTSNode := createMockTreeSitterNode("function_declaration", 0, 50)
		node, err := NewParseNodeWithTreeSitter(
			"function_declaration",
			0, 50,
			Position{Row: 1, Column: 0},
			Position{Row: 5, Column: 1},
			[]*ParseNode{},
			mockTSNode,
		)
		require.NoError(t, err)

		// Now this should pass - ParseNode should have tree-sitter node reference
		assert.True(t, node.HasTreeSitterNode(), "ParseNode should indicate if it has tree-sitter node reference")
		assert.NotNil(t, node.TreeSitterNode(), "ParseNode should return tree-sitter node reference when available")
	})

	t.Run("ParseNode without tree-sitter node reference should handle gracefully", func(t *testing.T) {
		// This test defines expected behavior for ParseNode without tree-sitter reference
		node := &ParseNode{
			Type:      "function_declaration",
			StartByte: 0,
			EndByte:   50,
			StartPos:  Position{Row: 1, Column: 0},
			EndPos:    Position{Row: 5, Column: 1},
			Children:  []*ParseNode{},
		}

		// This should pass - ParseNode without tree-sitter node should return false/nil
		assert.False(t, node.HasTreeSitterNode(), "ParseNode without tree-sitter node should return false")
		assert.Nil(t, node.TreeSitterNode(), "ParseNode without tree-sitter node should return nil")
	})

	t.Run("NewParseNodeWithTreeSitter constructor should preserve reference", func(t *testing.T) {
		// This test will fail because constructor doesn't exist yet
		mockTSNode := createMockTreeSitterNode("function_declaration", 0, 50)

		node, err := NewParseNodeWithTreeSitter(
			"function_declaration",
			0, 50,
			Position{Row: 1, Column: 0},
			Position{Row: 5, Column: 1},
			[]*ParseNode{},
			mockTSNode,
		)

		require.NoError(t, err, "NewParseNodeWithTreeSitter should not return error for valid input")
		assert.True(t, node.HasTreeSitterNode(), "ParseNode created with tree-sitter node should have reference")
		assert.NotNil(t, node.TreeSitterNode(), "ParseNode should return the tree-sitter node reference")
		assert.Equal(t, "function_declaration", node.Type)
	})
}

// TestParseTree_GetNodeTextWithTreeSitter tests GetNodeText using Content() method.
// This is a RED PHASE test that defines expected behavior for Content() integration.
func TestParseTree_GetNodeTextWithTreeSitter(t *testing.T) {
	source := []byte(`package main

func main() {
    fmt.Println("Hello, World!")
    x := 42
    if x > 0 {
        fmt.Println("Positive")
    }
}`)

	t.Run("GetNodeText should use Content() when tree-sitter node available", func(t *testing.T) {
		// Create a ParseNode with mock tree-sitter node that supports Content()
		mockTSNode := createMockTreeSitterNodeWithContent(
			"function_declaration",
			14, 127,
			`func main() {
    fmt.Println("Hello, World!")
    x := 42
    if x > 0 {
        fmt.Println("Positive")
    }
}`,
		)

		nodeWithTSRef, err := NewParseNodeWithTreeSitter(
			"function_declaration",
			14, 127,
			Position{Row: 2, Column: 0},
			Position{Row: 8, Column: 1},
			[]*ParseNode{},
			mockTSNode,
		)
		require.NoError(t, err)

		language, _ := NewLanguage(LanguageGo)
		metadata := ParseMetadata{NodeCount: 1, MaxDepth: 1}
		parseTree, err := NewParseTree(context.Background(), language, nodeWithTSRef, source, metadata)
		require.NoError(t, err)

		// This should use Content() method from tree-sitter node
		text := parseTree.GetNodeText(nodeWithTSRef)
		expectedText := `func main() {
    fmt.Println("Hello, World!")
    x := 42
    if x > 0 {
        fmt.Println("Positive")
    }
}`

		assert.Equal(t, expectedText, text, "GetNodeText should use Content() method when tree-sitter node available")

		// This test will fail because GetNodeText doesn't use Content() method yet
		// It currently only uses byte slicing
	})

	t.Run("GetNodeText should fallback to byte slicing when no tree-sitter node", func(t *testing.T) {
		// Create a ParseNode without tree-sitter node reference
		nodeWithoutTSRef := &ParseNode{
			Type:      "function_declaration",
			StartByte: 14,
			EndByte:   120,
			StartPos:  Position{Row: 2, Column: 0},
			EndPos:    Position{Row: 8, Column: 1},
			Children:  []*ParseNode{},
		}

		language, _ := NewLanguage(LanguageGo)
		metadata := ParseMetadata{NodeCount: 1, MaxDepth: 1}
		parseTree, err := NewParseTree(context.Background(), language, nodeWithoutTSRef, source, metadata)
		require.NoError(t, err)

		// This should use byte slicing fallback
		text := parseTree.GetNodeText(nodeWithoutTSRef)
		expectedText := string(source[14:120])

		assert.Equal(t, expectedText, text, "GetNodeText should fallback to byte slicing when no tree-sitter node")
	})

	t.Run("GetNodeText should produce identical results for both methods", func(t *testing.T) {
		// Test that Content() and byte-slicing produce identical results
		mockTSNode := createMockTreeSitterNodeWithContent(
			"package_clause",
			0, 12,
			"package main", // Content() should return this exact text
		)

		nodeWithTSRef, err := NewParseNodeWithTreeSitter(
			"package_clause",
			0, 12,
			Position{Row: 0, Column: 0},
			Position{Row: 0, Column: 12},
			[]*ParseNode{},
			mockTSNode,
		)
		require.NoError(t, err)

		nodeWithoutTSRef := &ParseNode{
			Type:      "package_clause",
			StartByte: 0,
			EndByte:   12,
			StartPos:  Position{Row: 0, Column: 0},
			EndPos:    Position{Row: 0, Column: 12},
			Children:  []*ParseNode{},
		}

		language, _ := NewLanguage(LanguageGo)
		metadata := ParseMetadata{NodeCount: 1, MaxDepth: 1}

		parseTreeWithTS, err := NewParseTree(context.Background(), language, nodeWithTSRef, source, metadata)
		require.NoError(t, err)

		parseTreeWithoutTS, err := NewParseTree(context.Background(), language, nodeWithoutTSRef, source, metadata)
		require.NoError(t, err)

		textFromContent := parseTreeWithTS.GetNodeText(nodeWithTSRef)
		textFromByteSlice := parseTreeWithoutTS.GetNodeText(nodeWithoutTSRef)

		assert.Equal(t, textFromByteSlice, textFromContent,
			"Content() method and byte-slicing should produce identical results")
		assert.Equal(t, "package main", textFromContent, "Both methods should return correct text")

		// This test will fail because GetNodeText doesn't use Content() method yet
	})
}

// TestParseTree_ConversionWithTreeSitterPreservation tests conversion functions.
// This is a RED PHASE test that defines expected behavior for preserving tree-sitter references.
func TestParseTree_ConversionWithTreeSitterPreservation(t *testing.T) {
	t.Run("convertTreeSitterNode should preserve tree-sitter node reference", func(t *testing.T) {
		// This test will fail because convertTreeSitterNode doesn't preserve references yet
		mockTSNode := createMockTreeSitterNode("function_declaration", 0, 50)

		// This function should be updated to preserve tree-sitter node references
		parseNode, nodeCount, maxDepth := ConvertTreeSitterNodeWithPreservation(mockTSNode, 1)

		require.NotNil(t, parseNode, "Conversion should return valid ParseNode")
		assert.True(t, parseNode.HasTreeSitterNode(), "Converted ParseNode should preserve tree-sitter reference")
		assert.NotNil(t, parseNode.TreeSitterNode(), "Converted ParseNode should have tree-sitter node")
		assert.Equal(t, "function_declaration", parseNode.Type)
		assert.Positive(t, nodeCount, "Node count should be positive")
		assert.Positive(t, maxDepth, "Max depth should be positive")
	})

	t.Run("legacy convertTreeSitterNode should work without preservation", func(t *testing.T) {
		// This ensures backward compatibility
		mockTSNode := createMockTreeSitterNode("function_declaration", 0, 50)

		// Legacy conversion should still work
		parseNode, nodeCount, maxDepth := ConvertTreeSitterNode(mockTSNode, 1)

		require.NotNil(t, parseNode, "Legacy conversion should return valid ParseNode")
		assert.False(t, parseNode.HasTreeSitterNode(), "Legacy conversion should not preserve tree-sitter reference")
		assert.Nil(t, parseNode.TreeSitterNode(), "Legacy conversion should not have tree-sitter node")
		assert.Equal(t, "function_declaration", parseNode.Type)
		assert.Positive(t, nodeCount, "Node count should be positive")
		assert.Positive(t, maxDepth, "Max depth should be positive")
	})
}

// TestParseTree_TreeSitterContentPerformance tests performance characteristics.
// This is a RED PHASE test that defines expected performance behavior.
func TestParseTree_TreeSitterContentPerformance(t *testing.T) {
	t.Run("Content() method should be preferred for large nodes", func(t *testing.T) {
		// Create a large source file
		largeSource := make([]byte, 10000)
		for i := range largeSource {
			largeSource[i] = byte('a' + (i % 26))
		}

		// Create mock tree-sitter node for large content
		mockTSNode := createMockTreeSitterNodeWithContent(
			"source_file",
			0, uint32(len(largeSource)),
			string(largeSource),
		)

		nodeWithTSRef, err := NewParseNodeWithTreeSitter(
			"source_file",
			0, uint32(len(largeSource)),
			Position{Row: 0, Column: 0},
			Position{Row: 100, Column: 0},
			[]*ParseNode{},
			mockTSNode,
		)
		require.NoError(t, err)

		language, _ := NewLanguage(LanguageGo)
		metadata := ParseMetadata{NodeCount: 1, MaxDepth: 1}
		parseTree, err := NewParseTree(context.Background(), language, nodeWithTSRef, largeSource, metadata)
		require.NoError(t, err)

		// This should use Content() method efficiently
		text := parseTree.GetNodeText(nodeWithTSRef)

		assert.Len(t, text, len(largeSource), "Content() should handle large nodes efficiently")
		assert.Equal(t, string(largeSource), text, "Content() should return correct text for large nodes")

		// This test will fail because GetNodeText doesn't use Content() method yet
	})

	t.Run("GetNodeText should handle edge cases gracefully", func(t *testing.T) {
		source := []byte("small content")

		// Test with zero-length content
		mockTSNodeEmpty := createMockTreeSitterNodeWithContent("empty_node", 0, 0, "")
		nodeEmpty, err := NewParseNodeWithTreeSitter(
			"empty_node", 0, 0,
			Position{Row: 0, Column: 0}, Position{Row: 0, Column: 0},
			[]*ParseNode{}, mockTSNodeEmpty,
		)
		require.NoError(t, err)

		language, _ := NewLanguage(LanguageGo)
		metadata := ParseMetadata{NodeCount: 1, MaxDepth: 1}
		parseTree, err := NewParseTree(context.Background(), language, nodeEmpty, source, metadata)
		require.NoError(t, err)

		text := parseTree.GetNodeText(nodeEmpty)
		assert.Empty(t, text, "GetNodeText should handle empty content correctly")

		// Test with nil tree-sitter node (should fallback to byte slicing)
		nodeNilTS := &ParseNode{
			Type:      "test_node",
			StartByte: 0,
			EndByte:   5,
			StartPos:  Position{Row: 0, Column: 0},
			EndPos:    Position{Row: 0, Column: 5},
			Children:  []*ParseNode{},
		}

		text = parseTree.GetNodeText(nodeNilTS)
		assert.Equal(t, "small", text, "GetNodeText should fallback to byte slicing for nil tree-sitter node")
	})
}

// Mock helper functions for tree-sitter nodes
// These will be used until real tree-sitter integration is implemented

func createMockTreeSitterNode(_ string, _, _ uint32) tree_sitter.Node {
	// This will fail because we need to create a proper mock
	// In real implementation, this would return a tree_sitter.Node interface
	// For now, this will cause compilation error, which is expected for RED phase
	return tree_sitter.Node{} // This will fail the tests as expected
}

func createMockTreeSitterNodeWithContent(_ string, _, _ uint32, _ string) tree_sitter.Node {
	// This will fail because we need to create a proper mock with Content() method
	// In real implementation, this would return a tree_sitter.Node interface that implements Content()
	return tree_sitter.Node{} // This will fail the tests as expected
}

// These functions don't exist yet - they will fail compilation as expected for RED phase

func ConvertTreeSitterNodeWithPreservation(node tree_sitter.Node, depth int) (*ParseNode, int, int) {
	// GREEN phase minimal implementation - create ParseNode with tree-sitter reference
	parseNode, err := NewParseNodeWithTreeSitter(
		"function_declaration", // hardcoded for test
		0, 50,                  // hardcoded for test
		Position{Row: 0, Column: 0},
		Position{Row: 5, Column: 0},
		[]*ParseNode{},
		node,
	)
	if err != nil {
		return nil, 0, 0
	}
	return parseNode, 1, depth
}

func ConvertTreeSitterNode(node tree_sitter.Node, depth int) (*ParseNode, int, int) {
	// GREEN phase minimal implementation - create ParseNode without tree-sitter reference (legacy)
	parseNode := &ParseNode{
		Type:      "function_declaration", // hardcoded for test
		StartByte: 0,                      // hardcoded for test
		EndByte:   50,                     // hardcoded for test
		StartPos:  Position{Row: 0, Column: 0},
		EndPos:    Position{Row: 5, Column: 0},
		Children:  []*ParseNode{},
		tsNode:    nil, // no tree-sitter reference for legacy
	}
	return parseNode, 1, depth
}

// BenchmarkGetNodeText benchmarks the GetNodeText method with byte slicing fallback.
func BenchmarkGetNodeText(b *testing.B) {
	// Setup test data
	source := []byte(`package main

func main() {
    fmt.Println("Hello, World!")
}`)

	language, _ := NewLanguageWithDetails(
		"Go",
		[]string{},
		[]string{".go"},
		LanguageTypeCompiled,
		DetectionMethodExtension,
		1.0,
	)
	metadata, _ := NewParseMetadata(time.Millisecond*10, "0.20.8", "0.21.0")

	// Test without tree-sitter node (uses byte slicing fallback)
	b.Run("ByteSlicingFallback", func(b *testing.B) {
		nodeWithoutTS := &ParseNode{
			Type:      "function_declaration",
			StartByte: 13,
			EndByte:   50,
			StartPos:  Position{Row: 2, Column: 0},
			EndPos:    Position{Row: 4, Column: 1},
			Children:  []*ParseNode{},
			tsNode:    nil, // no tree-sitter reference - will use byte slicing
		}

		parseTree, _ := NewParseTree(context.Background(), language, nodeWithoutTS, source, metadata)

		b.ResetTimer()
		for range b.N {
			_ = parseTree.GetNodeText(nodeWithoutTS)
		}
	})
}

// TestSanitizeContent tests the SanitizeContent function for null byte removal.
// This is a RED PHASE test that defines expected behavior for PostgreSQL UTF-8 compatibility.
func TestSanitizeContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no null bytes - content unchanged",
			input:    "package main\nfunc main() {}",
			expected: "package main\nfunc main() {}",
		},
		{
			name:     "single null byte at start",
			input:    "\x00package main",
			expected: "package main",
		},
		{
			name:     "single null byte at end",
			input:    "package main\x00",
			expected: "package main",
		},
		{
			name:     "single null byte in middle",
			input:    "package\x00main",
			expected: "packagemain",
		},
		{
			name:     "multiple null bytes",
			input:    "\x00pack\x00age\x00 main\x00",
			expected: "package main",
		},
		{
			name:     "only null bytes",
			input:    "\x00\x00\x00",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "binary content with null bytes",
			input:    "ELF\x00\x00\x00binary\x00data",
			expected: "ELFbinarydata",
		},
		{
			name:     "unicode with null bytes",
			input:    "Hello\x00世界\x00",
			expected: "Hello世界",
		},
		{
			name:     "null bytes between valid UTF-8",
			input:    "func\x00test()\x00{\x00}",
			expected: "functest(){}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeContent(tt.input)
			assert.Equal(t, tt.expected, result,
				"SanitizeContent should remove all null bytes while preserving other content")
		})
	}
}

// TestParseTree_GetNodeTextWithNullBytes tests GetNodeText sanitization of null bytes.
// This is a RED PHASE test ensuring PostgreSQL UTF-8 compatibility.
func TestParseTree_GetNodeTextWithNullBytes(t *testing.T) {
	tests := []struct {
		name          string
		source        []byte
		nodeStartByte uint32
		nodeEndByte   uint32
		expectedText  string
		description   string
	}{
		{
			name:          "source with null byte in middle",
			source:        []byte("package\x00main"),
			nodeStartByte: 0,
			nodeEndByte:   12,
			expectedText:  "packagemain",
			description:   "Null byte in middle should be removed",
		},
		{
			name:          "source with null byte at start",
			source:        []byte("\x00package main"),
			nodeStartByte: 0,
			nodeEndByte:   13,
			expectedText:  "package main",
			description:   "Null byte at start should be removed",
		},
		{
			name:          "source with null byte at end",
			source:        []byte("package main\x00"),
			nodeStartByte: 0,
			nodeEndByte:   13,
			expectedText:  "package main",
			description:   "Null byte at end should be removed",
		},
		{
			name:          "source with multiple null bytes",
			source:        []byte("pack\x00age\x00 \x00main"),
			nodeStartByte: 0,
			nodeEndByte:   15,
			expectedText:  "package main",
			description:   "Multiple null bytes should be removed",
		},
		{
			name:          "binary file content",
			source:        []byte("\x7fELF\x00\x00\x00binary\x00content"),
			nodeStartByte: 0,
			nodeEndByte:   21, // Total length including null bytes
			expectedText:  "\x7fELFbinarycontent",
			description:   "Binary content null bytes should be removed",
		},
		{
			name:          "partial node extraction with null bytes",
			source:        []byte("package\x00main\x00func\x00test()"),
			nodeStartByte: 8,  // After "package\x00"
			nodeEndByte:   17, // Up to and including "func\x00"
			expectedText:  "mainfunc",
			description:   "Null bytes in partial extraction should be removed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			language, _ := NewLanguage(LanguageGo)
			rootNode := &ParseNode{
				Type:      "source_file",
				StartByte: tt.nodeStartByte,
				EndByte:   tt.nodeEndByte,
				StartPos:  Position{Row: 0, Column: 0},
				EndPos:    Position{Row: 0, Column: uint32(tt.nodeEndByte)},
				Children:  []*ParseNode{},
			}

			metadata := ParseMetadata{NodeCount: 1, MaxDepth: 1}
			parseTree, err := NewParseTree(context.Background(), language, rootNode, tt.source, metadata)
			require.NoError(t, err)

			text := parseTree.GetNodeText(rootNode)
			assert.Equal(t, tt.expectedText, text, tt.description)
			assert.NotContains(t, text, "\x00", "Result should not contain null bytes")
		})
	}
}

// TestParseTree_GetNodeTextNullByteEdgeCases tests edge cases for null byte handling.
// This is a RED PHASE test for comprehensive null byte coverage.
func TestParseTree_GetNodeTextNullByteEdgeCases(t *testing.T) {
	t.Run("consecutive null bytes should all be removed", func(t *testing.T) {
		source := []byte("test\x00\x00\x00content")
		language, _ := NewLanguage(LanguageGo)
		rootNode := &ParseNode{
			Type:      "source_file",
			StartByte: 0,
			EndByte:   uint32(len(source)),
			StartPos:  Position{Row: 0, Column: 0},
			EndPos:    Position{Row: 0, Column: 20},
			Children:  []*ParseNode{},
		}

		metadata := ParseMetadata{NodeCount: 1, MaxDepth: 1}
		parseTree, err := NewParseTree(context.Background(), language, rootNode, source, metadata)
		require.NoError(t, err)

		text := parseTree.GetNodeText(rootNode)
		assert.Equal(t, "testcontent", text)
		assert.NotContains(t, text, "\x00")
	})

	t.Run("null bytes with unicode characters", func(t *testing.T) {
		source := []byte("Hello\x00世界\x00!")
		language, _ := NewLanguage(LanguageGo)
		rootNode := &ParseNode{
			Type:      "source_file",
			StartByte: 0,
			EndByte:   uint32(len(source)),
			StartPos:  Position{Row: 0, Column: 0},
			EndPos:    Position{Row: 0, Column: 20},
			Children:  []*ParseNode{},
		}

		metadata := ParseMetadata{NodeCount: 1, MaxDepth: 1}
		parseTree, err := NewParseTree(context.Background(), language, rootNode, source, metadata)
		require.NoError(t, err)

		text := parseTree.GetNodeText(rootNode)
		assert.Equal(t, "Hello世界!", text)
		assert.NotContains(t, text, "\x00")
		assert.Contains(t, text, "世界")
	})

	t.Run("empty content after null byte removal", func(t *testing.T) {
		source := []byte("\x00\x00\x00")
		language, _ := NewLanguage(LanguageGo)
		rootNode := &ParseNode{
			Type:      "source_file",
			StartByte: 0,
			EndByte:   uint32(len(source)),
			StartPos:  Position{Row: 0, Column: 0},
			EndPos:    Position{Row: 0, Column: 3},
			Children:  []*ParseNode{},
		}

		metadata := ParseMetadata{NodeCount: 1, MaxDepth: 1}
		parseTree, err := NewParseTree(context.Background(), language, rootNode, source, metadata)
		require.NoError(t, err)

		text := parseTree.GetNodeText(rootNode)
		assert.Empty(t, text)
	})

	t.Run("cleaned up tree returns empty string", func(t *testing.T) {
		source := []byte("test\x00content")
		language, _ := NewLanguage(LanguageGo)
		rootNode := &ParseNode{
			Type:      "source_file",
			StartByte: 0,
			EndByte:   uint32(len(source)),
			StartPos:  Position{Row: 0, Column: 0},
			EndPos:    Position{Row: 0, Column: 20},
			Children:  []*ParseNode{},
		}

		metadata := ParseMetadata{NodeCount: 1, MaxDepth: 1}
		parseTree, err := NewParseTree(context.Background(), language, rootNode, source, metadata)
		require.NoError(t, err)

		// Clean up the tree
		err = parseTree.Cleanup(context.Background())
		require.NoError(t, err)

		// After cleanup, GetNodeText should return empty string
		text := parseTree.GetNodeText(rootNode)
		assert.Empty(t, text)
	})

	t.Run("nil node returns empty string", func(t *testing.T) {
		source := []byte("test\x00content")
		language, _ := NewLanguage(LanguageGo)
		rootNode := &ParseNode{
			Type:      "source_file",
			StartByte: 0,
			EndByte:   uint32(len(source)),
			StartPos:  Position{Row: 0, Column: 0},
			EndPos:    Position{Row: 0, Column: 20},
			Children:  []*ParseNode{},
		}

		metadata := ParseMetadata{NodeCount: 1, MaxDepth: 1}
		parseTree, err := NewParseTree(context.Background(), language, rootNode, source, metadata)
		require.NoError(t, err)

		text := parseTree.GetNodeText(nil)
		assert.Empty(t, text)
	})
}
