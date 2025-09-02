package valueobject

import (
	"context"
	"strings"
	"testing"
	"time"

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
