package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createParseTree is a helper function to create a parse tree from Go source code.
func createParseTree(t *testing.T, sourceCode string) *valueobject.ParseTree {
	ctx := context.Background()
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err)

	parseResult, err := parser.Parse(ctx, []byte(sourceCode))
	require.NoError(t, err)
	require.True(t, parseResult.Success)

	domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseResult.ParseTree)
	require.NoError(t, err)

	return domainTree
}

func TestTreeSitterQueryEngine_QueryPackageDeclarations(t *testing.T) {
	tests := []struct {
		name          string
		sourceCode    string
		expectedCount int
		expectedNames []string
		expectedBytes [][]uint32 // [startByte, endByte] pairs
	}{
		{
			name:          "simple package declaration",
			sourceCode:    "package main\n\nfunc main() {}\n",
			expectedCount: 1,
			expectedNames: []string{"main"},
		},
		{
			name:          "package with underscore",
			sourceCode:    "package my_package\n\ntype MyStruct struct {}\n",
			expectedCount: 1,
			expectedNames: []string{"my_package"},
		},
		{
			name:          "package in test file",
			sourceCode:    "package mypackage_test\n\nimport \"testing\"\n\nfunc TestSomething(t *testing.T) {}\n",
			expectedCount: 1,
			expectedNames: []string{"mypackage_test"},
		},
		{
			name:          "package with documentation",
			sourceCode:    "// Package utils provides utility functions\n// for common operations.\npackage utils\n\nconst Version = \"1.0.0\"\n",
			expectedCount: 1,
			expectedNames: []string{"utils"},
		},
	}

	// Create the TreeSitterQueryEngine implementation
	engine := NewTreeSitterQueryEngine()
	require.NotNil(t, engine, "TreeSitterQueryEngine should be implemented")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree := createParseTree(t, tt.sourceCode)

			// Test the implementation
			packageNodes := engine.QueryPackageDeclarations(parseTree)

			assert.Len(t, packageNodes, tt.expectedCount)

			for i, expectedName := range tt.expectedNames {
				require.Less(t, i, len(packageNodes))

				// Verify the node type is correct
				assert.Equal(t, "package_clause", packageNodes[i].Type)

				// Verify the package name can be extracted from children
				packageNameNode := findPackageIdentifierInNode(parseTree, packageNodes[i])
				require.NotNil(t, packageNameNode, "package identifier node should exist")

				actualName := parseTree.GetNodeText(packageNameNode)
				assert.Equal(t, expectedName, actualName)

				// Verify byte positions are valid (can be 0 for first element)
				assert.GreaterOrEqual(t, packageNodes[i].StartByte, uint32(0))
				assert.Greater(t, packageNodes[i].EndByte, packageNodes[i].StartByte)
				assert.LessOrEqual(t, packageNodes[i].EndByte, uint32(len(tt.sourceCode)))
			}
		})
	}
}

func TestTreeSitterQueryEngine_QueryImportDeclarations(t *testing.T) {
	tests := []struct {
		name          string
		sourceCode    string
		expectedCount int
		expectedPaths []string
	}{
		{
			name:          "single import",
			sourceCode:    "package main\n\nimport \"fmt\"\n\nfunc main() {}\n",
			expectedCount: 1,
			expectedPaths: []string{"fmt"},
		},
		{
			name: "multiple imports with parentheses",
			sourceCode: `package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {}
`,
			expectedCount: 1,                   // One grouped import declaration
			expectedPaths: []string{"grouped"}, // Can't easily extract paths from grouped declaration
		},
		{
			name: "imports with aliases",
			sourceCode: `package main

import (
	"fmt"
	f "fmt"
	. "os"
	_ "encoding/json"
)

func main() {}
`,
			expectedCount: 1,                   // One grouped import declaration
			expectedPaths: []string{"grouped"}, // Can't easily extract paths from grouped declaration
		},
		{
			name: "import with relative path",
			sourceCode: `package main

import (
	"../utils"
	"./internal/config"
)

func main() {}
`,
			expectedCount: 1,                   // One grouped import declaration
			expectedPaths: []string{"grouped"}, // Can't easily extract paths from grouped declaration
		},
	}

	engine := NewTreeSitterQueryEngine()
	require.NotNil(t, engine, "TreeSitterQueryEngine should be implemented")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree := createParseTree(t, tt.sourceCode)

			// Test the implementation
			importNodes := engine.QueryImportDeclarations(parseTree)

			assert.Len(t, importNodes, tt.expectedCount)

			for i := range tt.expectedPaths {
				require.Less(t, i, len(importNodes))

				// Verify the node type is correct
				assert.Equal(t, "import_declaration", importNodes[i].Type)

				// Verify byte positions are valid (can be 0 for first element)
				assert.GreaterOrEqual(t, importNodes[i].StartByte, uint32(0))
				assert.Greater(t, importNodes[i].EndByte, importNodes[i].StartByte)
				assert.LessOrEqual(t, importNodes[i].EndByte, uint32(len(tt.sourceCode)))

				// TODO: In the green phase, verify the actual import path can be extracted
				// expectedPath := tt.expectedPaths[i]
				// actualPath := extractImportPathFromNode(parseTree, importNodes[i])
				// assert.Equal(t, expectedPath, actualPath)
			}
		})
	}
}

func TestTreeSitterQueryEngine_QueryFunctionDeclarations(t *testing.T) {
	tests := []struct {
		name          string
		sourceCode    string
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "simple function",
			sourceCode:    "package main\n\nfunc main() {}\n",
			expectedCount: 1,
			expectedNames: []string{"main"},
		},
		{
			name: "multiple functions",
			sourceCode: `package main

func init() {}

func main() {}

func helper() string {
	return "help"
}
`,
			expectedCount: 3,
			expectedNames: []string{"init", "main", "helper"},
		},
		{
			name: "function with parameters and return types",
			sourceCode: `package main

func Add(a, b int) int {
	return a + b
}

func Process(data []string) (result string, err error) {
	return "", nil
}
`,
			expectedCount: 2,
			expectedNames: []string{"Add", "Process"},
		},
		{
			name: "generic function",
			sourceCode: `package main

func GenericFunc[T any](value T) T {
	return value
}

func MultiGeneric[T, U comparable](a T, b U) bool {
	return false
}
`,
			expectedCount: 2,
			expectedNames: []string{"GenericFunc", "MultiGeneric"},
		},
	}

	engine := NewTreeSitterQueryEngine()
	require.NotNil(t, engine, "TreeSitterQueryEngine should be implemented")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree := createParseTree(t, tt.sourceCode)

			// Test the implementation
			functionNodes := engine.QueryFunctionDeclarations(parseTree)

			assert.Len(t, functionNodes, tt.expectedCount)

			for i, expectedName := range tt.expectedNames {
				require.Less(t, i, len(functionNodes))

				// Verify the node type is correct
				assert.Equal(t, "function_declaration", functionNodes[i].Type)

				// Verify the function name can be extracted
				functionNameNode := findFunctionIdentifierInNode(parseTree, functionNodes[i])
				require.NotNil(t, functionNameNode, "function identifier node should exist")

				actualName := parseTree.GetNodeText(functionNameNode)
				assert.Equal(t, expectedName, actualName)

				// Verify byte positions are valid (can be 0 for first element)
				assert.GreaterOrEqual(t, functionNodes[i].StartByte, uint32(0))
				assert.Greater(t, functionNodes[i].EndByte, functionNodes[i].StartByte)
				assert.LessOrEqual(t, functionNodes[i].EndByte, uint32(len(tt.sourceCode)))
			}
		})
	}
}

func TestTreeSitterQueryEngine_QueryMethodDeclarations(t *testing.T) {
	tests := []struct {
		name          string
		sourceCode    string
		expectedCount int
		expectedNames []string
	}{
		{
			name: "simple method",
			sourceCode: `package main

type Person struct {
	Name string
}

func (p Person) GetName() string {
	return p.Name
}
`,
			expectedCount: 1,
			expectedNames: []string{"GetName"},
		},
		{
			name: "multiple methods with pointer receivers",
			sourceCode: `package main

type Counter struct {
	value int
}

func (c *Counter) Increment() {
	c.value++
}

func (c *Counter) Value() int {
	return c.value
}

func (c Counter) String() string {
	return fmt.Sprintf("Counter: %d", c.value)
}
`,
			expectedCount: 3,
			expectedNames: []string{"Increment", "Value", "String"},
		},
		{
			name: "generic method",
			sourceCode: `package main

type Container[T any] struct {
	data T
}

func (c Container[T]) Get() T {
	return c.data
}

func (c *Container[T]) Set(value T) {
	c.data = value
}
`,
			expectedCount: 2,
			expectedNames: []string{"Get", "Set"},
		},
	}

	engine := NewTreeSitterQueryEngine()
	require.NotNil(t, engine, "TreeSitterQueryEngine should be implemented")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree := createParseTree(t, tt.sourceCode)

			// Test the implementation
			methodNodes := engine.QueryMethodDeclarations(parseTree)

			assert.Len(t, methodNodes, tt.expectedCount)

			for i, expectedName := range tt.expectedNames {
				require.Less(t, i, len(methodNodes))

				// Verify the node type is correct
				assert.Equal(t, "method_declaration", methodNodes[i].Type)

				// Verify the method name can be extracted
				methodNameNode := findMethodIdentifierInNode(parseTree, methodNodes[i])
				require.NotNil(t, methodNameNode, "method identifier node should exist")

				actualName := parseTree.GetNodeText(methodNameNode)
				assert.Equal(t, expectedName, actualName)

				// Verify byte positions are valid (can be 0 for first element)
				assert.GreaterOrEqual(t, methodNodes[i].StartByte, uint32(0))
				assert.Greater(t, methodNodes[i].EndByte, methodNodes[i].StartByte)
				assert.LessOrEqual(t, methodNodes[i].EndByte, uint32(len(tt.sourceCode)))
			}
		})
	}
}

func TestTreeSitterQueryEngine_QueryVariableDeclarations(t *testing.T) {
	tests := []struct {
		name          string
		sourceCode    string
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "simple var declaration",
			sourceCode:    "package main\n\nvar message string\n",
			expectedCount: 1,
			expectedNames: []string{"message"},
		},
		{
			name: "multiple var declarations",
			sourceCode: `package main

var (
	name string
	age  int
	active bool
)

var global = "global value"
`,
			expectedCount: 2,                      // One grouped declaration + one individual declaration
			expectedNames: []string{"", "global"}, // Can't easily extract names from grouped declaration
		},
		{
			name: "var with initialization",
			sourceCode: `package main

var (
	counter = 0
	data    = []string{"a", "b", "c"}
)

func main() {
	var local int = 42
}
`,
			expectedCount: 2,                     // One grouped declaration + one local declaration in function
			expectedNames: []string{"", "local"}, // Can't easily extract names from grouped declaration
		},
	}

	engine := NewTreeSitterQueryEngine()
	require.NotNil(t, engine, "TreeSitterQueryEngine should be implemented")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree := createParseTree(t, tt.sourceCode)

			// Test the implementation
			varNodes := engine.QueryVariableDeclarations(parseTree)

			assert.Len(t, varNodes, tt.expectedCount)

			for i := range tt.expectedNames {
				require.Less(t, i, len(varNodes))

				// Verify the node type is correct
				assert.Equal(t, "var_declaration", varNodes[i].Type)

				// Verify byte positions are valid (can be 0 for first element)
				assert.GreaterOrEqual(t, varNodes[i].StartByte, uint32(0))
				assert.Greater(t, varNodes[i].EndByte, varNodes[i].StartByte)
				assert.LessOrEqual(t, varNodes[i].EndByte, uint32(len(tt.sourceCode)))
			}
		})
	}
}

func TestTreeSitterQueryEngine_QueryConstDeclarations(t *testing.T) {
	tests := []struct {
		name          string
		sourceCode    string
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "simple const declaration",
			sourceCode:    "package main\n\nconst Version = \"1.0.0\"\n",
			expectedCount: 1,
			expectedNames: []string{"Version"},
		},
		{
			name: "multiple const declarations",
			sourceCode: `package main

const (
	StatusOK     = 200
	StatusFailed = 500
	StatusError  = 400
)

const DefaultTimeout = 30
`,
			expectedCount: 2,                              // One grouped declaration + one individual declaration
			expectedNames: []string{"", "DefaultTimeout"}, // Can't easily extract names from grouped declaration
		},
		{
			name: "const with iota",
			sourceCode: `package main

const (
	Red = iota
	Green
	Blue
)
`,
			expectedCount: 1,            // One grouped declaration
			expectedNames: []string{""}, // Can't easily extract names from grouped declaration
		},
	}

	engine := NewTreeSitterQueryEngine()
	require.NotNil(t, engine, "TreeSitterQueryEngine should be implemented")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree := createParseTree(t, tt.sourceCode)

			// Test the implementation
			constNodes := engine.QueryConstDeclarations(parseTree)

			assert.Len(t, constNodes, tt.expectedCount)

			for i := range tt.expectedNames {
				require.Less(t, i, len(constNodes))

				// Verify the node type is correct
				assert.Equal(t, "const_declaration", constNodes[i].Type)

				// Verify byte positions are valid (can be 0 for first element)
				assert.GreaterOrEqual(t, constNodes[i].StartByte, uint32(0))
				assert.Greater(t, constNodes[i].EndByte, constNodes[i].StartByte)
				assert.LessOrEqual(t, constNodes[i].EndByte, uint32(len(tt.sourceCode)))
			}
		})
	}
}

func TestTreeSitterQueryEngine_QueryTypeDeclarations(t *testing.T) {
	tests := []struct {
		name          string
		sourceCode    string
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "simple struct type",
			sourceCode:    "package main\n\ntype Person struct {\n\tName string\n}\n",
			expectedCount: 1,
			expectedNames: []string{"Person"},
		},
		{
			name: "multiple type declarations",
			sourceCode: `package main

type (
	User struct {
		ID   int
		Name string
	}

	Status int

	Handler func(string) error
)

type StringSlice []string
`,
			expectedCount: 2,                           // One grouped declaration + one individual declaration
			expectedNames: []string{"", "StringSlice"}, // Can't easily extract names from grouped declaration
		},
		{
			name: "interface type",
			sourceCode: `package main

type Writer interface {
	Write([]byte) (int, error)
}

type ReadWriter interface {
	Reader
	Writer
}
`,
			expectedCount: 2,
			expectedNames: []string{"Writer", "ReadWriter"},
		},
		{
			name: "generic types",
			sourceCode: `package main

type Stack[T any] struct {
	items []T
}

type Comparer[T comparable] interface {
	Compare(other T) bool
}
`,
			expectedCount: 2,
			expectedNames: []string{"Stack", "Comparer"},
		},
	}

	engine := NewTreeSitterQueryEngine()
	require.NotNil(t, engine, "TreeSitterQueryEngine should be implemented")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree := createParseTree(t, tt.sourceCode)

			// Test the implementation
			typeNodes := engine.QueryTypeDeclarations(parseTree)

			assert.Len(t, typeNodes, tt.expectedCount)

			for i := range tt.expectedNames {
				require.Less(t, i, len(typeNodes))

				// Verify the node type is correct
				assert.Equal(t, "type_declaration", typeNodes[i].Type)

				// Verify byte positions are valid (can be 0 for first element)
				assert.GreaterOrEqual(t, typeNodes[i].StartByte, uint32(0))
				assert.Greater(t, typeNodes[i].EndByte, typeNodes[i].StartByte)
				assert.LessOrEqual(t, typeNodes[i].EndByte, uint32(len(tt.sourceCode)))
			}
		})
	}
}

func TestTreeSitterQueryEngine_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		sourceCode string
		testAll    bool // if true, test all query methods
	}{
		{
			name:       "empty file",
			sourceCode: "",
			testAll:    true,
		},
		{
			name:       "only comments",
			sourceCode: "// This is a comment\n/* Block comment */\n",
			testAll:    true,
		},
		{
			name:       "only package declaration",
			sourceCode: "package main\n",
			testAll:    true,
		},
		{
			name:       "package with only imports",
			sourceCode: "package main\n\nimport \"fmt\"\nimport \"os\"\n",
			testAll:    true,
		},
		{
			name: "mixed declarations",
			sourceCode: `package main

import "fmt"

const Version = "1.0.0"

var global string

type MyStruct struct{}

func main() {}

func (m MyStruct) Method() {}
`,
			testAll: true,
		},
	}

	engine := NewTreeSitterQueryEngine()
	require.NotNil(t, engine, "TreeSitterQueryEngine should be implemented")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sourceCode == "" {
				// Empty source code should be handled gracefully
				t.Skip("Skipping empty source code test as createParseTree requires non-empty source")
				return
			}

			parseTree := createParseTree(t, tt.sourceCode)

			if tt.testAll {
				// Test all query methods to ensure they handle edge cases gracefully

				// All methods should return empty slices for edge cases
				packageNodes := engine.QueryPackageDeclarations(parseTree)
				assert.NotNil(t, packageNodes)

				importNodes := engine.QueryImportDeclarations(parseTree)
				assert.NotNil(t, importNodes)

				functionNodes := engine.QueryFunctionDeclarations(parseTree)
				assert.NotNil(t, functionNodes)

				methodNodes := engine.QueryMethodDeclarations(parseTree)
				assert.NotNil(t, methodNodes)

				varNodes := engine.QueryVariableDeclarations(parseTree)
				assert.NotNil(t, varNodes)

				constNodes := engine.QueryConstDeclarations(parseTree)
				assert.NotNil(t, constNodes)

				typeNodes := engine.QueryTypeDeclarations(parseTree)
				assert.NotNil(t, typeNodes)
			}
		})
	}
}

// Helper functions to find specific identifier nodes within declaration nodes

func findPackageIdentifierInNode(
	parseTree *valueobject.ParseTree,
	packageNode *valueobject.ParseNode,
) *valueobject.ParseNode {
	for _, child := range packageNode.Children {
		if child.Type == "package_identifier" || child.Type == "_package_identifier" {
			return child
		}
	}
	return nil
}

func findFunctionIdentifierInNode(
	parseTree *valueobject.ParseTree,
	functionNode *valueobject.ParseNode,
) *valueobject.ParseNode {
	for _, child := range functionNode.Children {
		if child.Type == "identifier" || child.Type == "field_identifier" {
			return child
		}
	}
	return nil
}

func findMethodIdentifierInNode(
	parseTree *valueobject.ParseTree,
	methodNode *valueobject.ParseNode,
) *valueobject.ParseNode {
	for _, child := range methodNode.Children {
		if child.Type == "identifier" || child.Type == "field_identifier" {
			return child
		}
	}
	return nil
}
