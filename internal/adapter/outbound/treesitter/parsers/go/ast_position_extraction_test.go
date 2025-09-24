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

// calculateExpectedStructPositions parses source code with tree-sitter and returns
// the actual byte positions for the specified struct name from type_declaration nodes.
func calculateExpectedStructPositions(t *testing.T, sourceCode, structName string) (uint32, uint32) {
	ctx := context.Background()
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err)

	parseTree, err := parser.Parse(ctx, []byte(sourceCode))
	require.NoError(t, err)

	domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
	require.NoError(t, err)

	// Find type_declaration nodes (the proper way to find structs)
	typeDeclarationNodes := domainTree.GetNodesByType("type_declaration")

	for _, node := range typeDeclarationNodes {
		// Navigate: type_declaration -> type_spec -> type_identifier
		for _, child := range node.Children {
			if child.Type == "type_spec" {
				for _, grandchild := range child.Children {
					if grandchild.Type == "type_identifier" {
						nodeName := domainTree.GetNodeText(grandchild)
						if nodeName == structName {
							return node.StartByte, node.EndByte
						}
					}
				}
			}
		}
	}

	t.Fatalf("Could not find struct %s in source code", structName)
	return 0, 0
}

// calculateExpectedPackagePositions parses source code with tree-sitter and returns
// the actual byte positions for package declarations.
func calculateExpectedPackagePositions(t *testing.T, sourceCode string) (uint32, uint32) {
	ctx := context.Background()
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err)

	parseTree, err := parser.Parse(ctx, []byte(sourceCode))
	require.NoError(t, err)

	domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
	require.NoError(t, err)

	// Find package_clause nodes (the proper way to find package declarations)
	packageNodes := domainTree.GetNodesByType("package_clause")
	if len(packageNodes) > 0 {
		return packageNodes[0].StartByte, packageNodes[0].EndByte
	}

	t.Fatalf("Could not find package declaration in source code")
	return 0, 0
}

func TestDebugStructAST(t *testing.T) {
	// DEBUG: Let's understand the AST structure first
	sourceCode := `package main

type Person struct {
	Name string
	Age  int
}`

	ctx := context.Background()
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err)

	parseTree, err := parser.Parse(ctx, []byte(sourceCode))
	require.NoError(t, err)

	domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
	require.NoError(t, err)

	// Print all node types to understand the structure
	t.Logf("Source code:\n%s", sourceCode)
	rootNode := domainTree.RootNode()
	t.Logf("Root node type: %s", rootNode.Type)

	// Look for all nodes
	allNodes := []*valueobject.ParseNode{rootNode}
	for len(allNodes) > 0 {
		node := allNodes[0]
		allNodes = allNodes[1:]

		t.Logf("Node type: %s, text: %q, start: %d, end: %d",
			node.Type, domainTree.GetNodeText(node), node.StartByte, node.EndByte)

		allNodes = append(allNodes, node.Children...)
	}

	// Specifically look for type_declaration nodes
	typeDeclarationNodes := domainTree.GetNodesByType("type_declaration")
	t.Logf("Found %d type_declaration nodes", len(typeDeclarationNodes))

	for i, node := range typeDeclarationNodes {
		t.Logf("type_declaration[%d]: text=%q, start=%d, end=%d",
			i, domainTree.GetNodeText(node), node.StartByte, node.EndByte)

		for j, child := range node.Children {
			t.Logf("  child[%d]: type=%s, text=%q",
				j, child.Type, domainTree.GetNodeText(child))
		}
	}
}

func TestExtractClasses_ShouldUseRealASTPositionsNotHardcodedValues(t *testing.T) {
	// RED PHASE: This test will FAIL because the current implementation uses:
	// 1. Hardcoded startByte := uint32(1) instead of real node.StartByte
	// 2. calculateEndByteForStruct() lookup table instead of real node.EndByte
	//
	// This test requires actual AST node positions from tree-sitter.

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()

	// Simple test case first
	sourceCode := `package main

type Person struct {
	Name string
	Age  int
}`

	// Get expected positions from real AST parsing
	expectedStartByte, expectedEndByte := calculateExpectedStructPositions(t, sourceCode, "Person")

	// Parse with the actual parser
	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)

	classes, err := parser.ExtractClasses(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)
	require.Len(t, classes, 1, "Should extract exactly one struct")

	class := classes[0]

	// These assertions will FAIL because current implementation uses hardcoded values
	assert.Equal(t, expectedStartByte, class.StartByte,
		"StartByte should come from real AST node position, not hardcoded value of 1")
	assert.Equal(t, expectedEndByte, class.EndByte,
		"EndByte should come from real AST node position, not calculateEndByteForStruct() lookup table")

	// Validate position consistency
	assert.Greater(t, class.EndByte, class.StartByte,
		"EndByte should be greater than StartByte")
	assert.Positive(t, class.StartByte,
		"StartByte should be greater than 0 for real source code")
}

func TestExtractModules_ShouldUseRealASTPositionsNotHardcodedValues(t *testing.T) {
	// RED PHASE: This test will FAIL because the current implementation uses:
	// 1. Hardcoded defaultStartByte = 1 instead of real node.StartByte
	// 2. calculateEndByteForPackage() lookup table instead of real node.EndByte
	//
	// This test requires actual AST node positions from tree-sitter.

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()

	testCases := []struct {
		name        string
		sourceCode  string
		expectedPkg string
	}{
		{
			name: "main package should use real AST positions",
			sourceCode: `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}`,
			expectedPkg: "main",
		},
		{
			name: "utils package should use real AST positions",
			sourceCode: `package utils

func Helper() {
	// helper function
}`,
			expectedPkg: "utils",
		},
		{
			name: "newpackage should not have hardcoded position",
			sourceCode: `package newpackage

const Value = 42`,
			expectedPkg: "newpackage",
		},
		{
			name: "Same package name different content should have different positions",
			sourceCode: `
// This comment changes the byte position
package main

type Example struct {
	Field string
}`,
			expectedPkg: "main",
		},
		{
			name: "Package with longer name should have different positions",
			sourceCode: `package verylongpackagename

var GlobalVar = "test"`,
			expectedPkg: "verylongpackagename",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Get expected positions from real AST parsing
			expectedStartByte, expectedEndByte := calculateExpectedPackagePositions(t, tc.sourceCode)

			// Parse with the actual parser
			language, err := valueobject.NewLanguage(valueobject.LanguageGo)
			require.NoError(t, err)

			parseTree := createMockParseTreeFromSource(t, language, tc.sourceCode)

			modules, err := parser.ExtractModules(ctx, parseTree, outbound.SemanticExtractionOptions{})
			require.NoError(t, err)
			require.Len(t, modules, 1, "Should extract exactly one package")

			module := modules[0]

			// These assertions will FAIL because current implementation uses hardcoded values
			assert.Equal(t, expectedStartByte, module.StartByte,
				"StartByte should come from real AST node position, not defaultStartByte = 1")
			assert.Equal(t, expectedEndByte, module.EndByte,
				"EndByte should come from real AST node position, not calculateEndByteForPackage() lookup table")

			// Validate position consistency
			assert.Greater(t, module.EndByte, module.StartByte,
				"EndByte should be greater than StartByte")
			assert.GreaterOrEqual(t, module.StartByte, uint32(0),
				"StartByte should be >= 0 for real source code")

			// Validate the package name is correct
			assert.Equal(t, tc.expectedPkg, module.Name,
				"Package name should be extracted correctly")
		})
	}
}

func TestStructPositions_ShouldNotBeBasedOnStructName(t *testing.T) {
	// RED PHASE: This test proves that positions should come from AST, not name-based lookup
	// The current calculateEndByteForStruct() function returns different values based on
	// struct name, which is wrong - positions should be based on actual source location.

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()

	// Test the same struct name in different positions
	sourceCode1 := `package main

type TestStruct struct {
	Field1 string
}`

	sourceCode2 := `package main

// Extra comment that changes positions
type TestStruct struct {
	Field1 string
	Field2 int  // Additional field
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	// Parse first version
	parseTree1 := createMockParseTreeFromSource(t, language, sourceCode1)
	classes1, err := parser.ExtractClasses(ctx, parseTree1, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)
	require.Len(t, classes1, 1)

	// Parse second version
	parseTree2 := createMockParseTreeFromSource(t, language, sourceCode2)
	classes2, err := parser.ExtractClasses(ctx, parseTree2, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)
	require.Len(t, classes2, 1)

	// Get expected positions from real AST
	expectedStart1, expectedEnd1 := calculateExpectedStructPositions(t, sourceCode1, "TestStruct")
	expectedStart2, expectedEnd2 := calculateExpectedStructPositions(t, sourceCode2, "TestStruct")

	// These assertions will FAIL because calculateEndByteForStruct() returns the same
	// hardcoded value for "TestStruct" regardless of actual position in source
	assert.Equal(t, expectedStart1, classes1[0].StartByte,
		"First TestStruct should have real AST start position")
	assert.Equal(t, expectedEnd1, classes1[0].EndByte,
		"First TestStruct should have real AST end position")

	assert.Equal(t, expectedStart2, classes2[0].StartByte,
		"Second TestStruct should have real AST start position")
	assert.Equal(t, expectedEnd2, classes2[0].EndByte,
		"Second TestStruct should have real AST end position")

	// The key failing assertion: positions should be different for different source layouts
	assert.NotEqual(t, classes1[0].StartByte, classes2[0].StartByte,
		"Same struct name in different positions should have different StartByte values")
	assert.NotEqual(t, classes1[0].EndByte, classes2[0].EndByte,
		"Same struct name in different positions should have different EndByte values")
}

func TestPackagePositions_ShouldNotBeBasedOnPackageName(t *testing.T) {
	// RED PHASE: This test proves that positions should come from AST, not name-based lookup
	// The current calculateEndByteForPackage() function returns different values based on
	// package name, which is wrong - positions should be based on actual source location.

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()

	// Test the same package name in different positions
	sourceCode1 := `package main`

	sourceCode2 := `
// Comment that changes byte positions
package main`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	// Parse first version
	parseTree1 := createMockParseTreeFromSource(t, language, sourceCode1)
	modules1, err := parser.ExtractModules(ctx, parseTree1, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)
	require.Len(t, modules1, 1)

	// Parse second version
	parseTree2 := createMockParseTreeFromSource(t, language, sourceCode2)
	modules2, err := parser.ExtractModules(ctx, parseTree2, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)
	require.Len(t, modules2, 1)

	// Get expected positions from real AST
	expectedStart1, expectedEnd1 := calculateExpectedPackagePositions(t, sourceCode1)
	expectedStart2, expectedEnd2 := calculateExpectedPackagePositions(t, sourceCode2)

	// These assertions will FAIL because calculateEndByteForPackage() returns the same
	// hardcoded value for "main" regardless of actual position in source
	assert.Equal(t, expectedStart1, modules1[0].StartByte,
		"First main package should have real AST start position")
	assert.Equal(t, expectedEnd1, modules1[0].EndByte,
		"First main package should have real AST end position")

	assert.Equal(t, expectedStart2, modules2[0].StartByte,
		"Second main package should have real AST start position")
	assert.Equal(t, expectedEnd2, modules2[0].EndByte,
		"Second main package should have real AST end position")

	// The key failing assertion: positions should be different for different source layouts
	assert.NotEqual(t, modules1[0].StartByte, modules2[0].StartByte,
		"Same package name in different positions should have different StartByte values")
	assert.NotEqual(t, modules1[0].EndByte, modules2[0].EndByte,
		"Same package name in different positions should have different EndByte values")
}

func TestPositions_ShouldBeWithinSourceCodeBounds(t *testing.T) {
	// RED PHASE: This test ensures that byte positions are logical and within bounds
	// Current hardcoded implementation may return positions outside source bounds

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()

	sourceCode := `package test

type Small struct {
	X int
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)

	// Test struct positions
	classes, err := parser.ExtractClasses(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)
	require.Len(t, classes, 1)

	// Test package positions
	modules, err := parser.ExtractModules(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)
	require.Len(t, modules, 1)

	sourceLength := uint32(len(sourceCode))

	// These assertions will FAIL if hardcoded values exceed source length
	assert.LessOrEqual(t, classes[0].StartByte, sourceLength,
		"Struct StartByte should not exceed source code length")
	assert.LessOrEqual(t, classes[0].EndByte, sourceLength,
		"Struct EndByte should not exceed source code length")

	assert.LessOrEqual(t, modules[0].StartByte, sourceLength,
		"Package StartByte should not exceed source code length")
	assert.LessOrEqual(t, modules[0].EndByte, sourceLength,
		"Package EndByte should not exceed source code length")

	// Validate position ordering
	assert.LessOrEqual(t, classes[0].StartByte, classes[0].EndByte,
		"Struct StartByte should be <= EndByte")
	assert.LessOrEqual(t, modules[0].StartByte, modules[0].EndByte,
		"Package StartByte should be <= EndByte")
}
