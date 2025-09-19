package goparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"

	forest "github.com/alexaandru/go-sitter-forest"
	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RED PHASE TESTS: Tree-sitter Conversion - Function Declaration Node Types
//
// These tests define the expected behavior for converting from string-based parsing
// to proper tree-sitter AST traversal for Go function declarations.

func TestExtractFunctions_ShouldLookForFunctionDeclarationNodeType(t *testing.T) {
	// RED PHASE: This test demonstrates that the parser should look for "function_declaration"
	// nodes instead of searching for "function" strings.
	//
	// According to tree-sitter Go grammar:
	// - function_declaration is the correct node type
	// - "function" is NOT a valid node type name

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()
	source := `package main

func HelloWorld() {
	println("Hello, World!")
}

func processData(data string) int {
	return len(data)
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, source)

	// RED PHASE: The parser should find function_declaration nodes in the AST
	functions, err := parser.ExtractFunctions(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	// This test will FAIL because the current implementation:
	// 1. Uses GetNodesByType("function_declaration") but the tree has empty nodes
	// 2. Falls back to string parsing instead of proper AST traversal
	// 3. Doesn't verify that tree-sitter actually parsed function_declaration nodes

	assert.Len(t, functions, 2, "Should extract 2 function_declaration nodes from tree-sitter AST")

	functionNames := make([]string, len(functions))
	for i, fn := range functions {
		functionNames[i] = fn.Name
	}

	assert.Contains(t, functionNames, "HelloWorld", "Should extract HelloWorld from function_declaration node")
	assert.Contains(t, functionNames, "processData", "Should extract processData from function_declaration node")

	// RED PHASE: Verify that extraction was based on AST nodes, not string parsing
	for _, fn := range functions {
		assert.Equal(t, outbound.ConstructFunction, fn.Type, "Should identify as function construct type")
		// StartByte/EndByte should come from actual AST node positions, not hardcoded values
		assert.Greater(t, fn.EndByte, fn.StartByte, "EndByte should be greater than StartByte from AST")
	}
}

func TestExtractFunctions_ShouldAccessNameFieldFromFunctionDeclarationNode(t *testing.T) {
	// RED PHASE: This test demonstrates that function names should be extracted
	// from the "name" field of function_declaration nodes, which contains an "identifier"
	//
	// Tree-sitter Go grammar structure:
	// function_declaration -> field "name" -> identifier

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()
	source := `package main

func calculateSum(a, b int) int {
	return a + b
}

func validateInput(input string) bool {
	return len(input) > 0
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, source)

	functions, err := parser.ExtractFunctions(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	// This test will FAIL because the current implementation:
	// 1. extractFunctionNameFromNode looks for child.Type == "identifier"
	// 2. But it should look for the "name" FIELD which contains an "identifier"
	// 3. Tree-sitter fields are accessed differently than just checking child types

	assert.Len(t, functions, 2, "Should extract functions using proper field access")

	expectedFunctions := map[string]bool{
		"calculateSum":  false,
		"validateInput": false,
	}

	for _, fn := range functions {
		if _, exists := expectedFunctions[fn.Name]; exists {
			expectedFunctions[fn.Name] = true
		}
	}

	for name, found := range expectedFunctions {
		assert.True(t, found, "Function %s should be extracted from name field of function_declaration node", name)
	}
}

func TestExtractMethods_ShouldLookForMethodDeclarationNodeType(t *testing.T) {
	// RED PHASE: This test demonstrates that methods should be extracted from
	// "method_declaration" nodes, not "function_declaration" nodes
	//
	// Tree-sitter Go grammar distinguishes between:
	// - function_declaration: standalone functions
	// - method_declaration: methods with receivers

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()
	source := `package main

type Person struct {
	name string
	age  int
}

func (p Person) GetName() string {
	return p.name
}

func (p *Person) SetAge(age int) {
	p.age = age
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, source)

	functions, err := parser.ExtractFunctions(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	// This test will FAIL because the current implementation:
	// 1. GetNodesByType("method_declaration") returns empty nodes
	// 2. Falls back to string parsing that doesn't properly distinguish methods
	// 3. Doesn't verify method_declaration vs function_declaration node types

	methodFunctions := make([]outbound.SemanticCodeChunk, 0)
	for _, fn := range functions {
		if fn.Type == outbound.ConstructMethod {
			methodFunctions = append(methodFunctions, fn)
		}
	}

	assert.Len(t, methodFunctions, 2, "Should extract 2 method_declaration nodes")

	methodNames := make([]string, len(methodFunctions))
	for i, method := range methodFunctions {
		methodNames[i] = method.Name
	}

	assert.Contains(t, methodNames, "GetName", "Should extract GetName from method_declaration node")
	assert.Contains(t, methodNames, "SetAge", "Should extract SetAge from method_declaration node")
}

func TestExtractMethods_ShouldAccessNameFieldFromMethodDeclaration(t *testing.T) {
	// RED PHASE: This test demonstrates that method names should be extracted
	// from the "name" field of method_declaration nodes, which contains a "_field_identifier"
	//
	// Tree-sitter Go grammar structure:
	// method_declaration -> field "name" -> _field_identifier
	// This is DIFFERENT from functions which use "identifier"

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()
	source := `package main

type Calculator struct {
	value int
}

func (c Calculator) Add(x int) int {
	return c.value + x
}

func (c *Calculator) Multiply(x int) {
	c.value *= x
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, source)

	functions, err := parser.ExtractFunctions(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	// This test will FAIL because the current implementation:
	// 1. extractMethodNameFromNode looks for "field_identifier" or "_field_identifier"
	// 2. But it should access the "name" FIELD from the method_declaration node
	// 3. Methods use _field_identifier while functions use identifier

	methodFunctions := make([]outbound.SemanticCodeChunk, 0)
	for _, fn := range functions {
		if fn.Type == outbound.ConstructMethod {
			methodFunctions = append(methodFunctions, fn)
		}
	}

	assert.Len(t, methodFunctions, 2, "Should extract methods using proper field access")

	expectedMethods := map[string]bool{
		"Add":      false,
		"Multiply": false,
	}

	for _, method := range methodFunctions {
		if _, exists := expectedMethods[method.Name]; exists {
			expectedMethods[method.Name] = true
		}
	}

	for name, found := range expectedMethods {
		assert.True(t, found, "Method %s should be extracted from name field containing _field_identifier", name)
	}
}

func TestExtractClasses_ShouldLookForTypeDeclarationNodes(t *testing.T) {
	// RED PHASE: This test demonstrates that structs should be extracted from
	// "type_declaration" nodes, not by searching for "struct" strings
	//
	// Tree-sitter Go grammar structure:
	// type_declaration -> type_spec -> struct_type

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()
	source := `package main

type User struct {
	id   int
	name string
}

type Product struct {
	sku   string
	price float64
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, source)

	classes, err := parser.ExtractClasses(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	// This test will FAIL because the current implementation:
	// 1. Uses simple string parsing instead of tree-sitter node traversal
	// 2. Doesn't look for type_declaration nodes in the AST
	// 3. Doesn't verify the type_declaration -> type_spec -> struct_type hierarchy

	assert.Len(t, classes, 2, "Should extract 2 struct definitions from type_declaration nodes")

	structNames := make([]string, len(classes))
	for i, class := range classes {
		structNames[i] = class.Name
	}

	assert.Contains(t, structNames, "User", "Should extract User from type_declaration -> type_spec")
	assert.Contains(t, structNames, "Product", "Should extract Product from type_declaration -> type_spec")

	// RED PHASE: Verify extraction was based on AST nodes, not string parsing
	for _, class := range classes {
		assert.Equal(t, outbound.ConstructStruct, class.Type, "Should identify as struct construct type")
		// StartByte/EndByte should come from actual type_declaration node positions
		assert.Greater(t, class.EndByte, class.StartByte, "EndByte should be greater than StartByte from AST")
	}
}

func TestExtractClasses_ShouldTraverseTypeDeclarationToTypeSpecToStructType(t *testing.T) {
	// RED PHASE: This test demonstrates the proper AST traversal hierarchy for structs:
	// type_declaration -> type_spec -> struct_type
	//
	// The parser should:
	// 1. Find type_declaration nodes
	// 2. Look for type_spec children within type_declaration
	// 3. Verify that type_spec contains struct_type (not interface_type)
	// 4. Extract the name from type_spec's "name" field (_type_identifier)

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()
	source := `package main

type Employee struct {
	id         int
	name       string
	department string
}

type Manager struct {
	Employee
	teamSize int
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, source)

	classes, err := parser.ExtractClasses(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	// This test will FAIL because the current implementation:
	// 1. containsStructType doesn't properly traverse the AST hierarchy
	// 2. extractStructNameFromNode looks for "type_identifier" directly instead of the proper field
	// 3. The parser doesn't verify the type_declaration -> type_spec -> struct_type path

	assert.Len(t, classes, 2, "Should extract structs by traversing type_declaration -> type_spec -> struct_type")

	expectedStructs := map[string]bool{
		"Employee": false,
		"Manager":  false,
	}

	for _, class := range classes {
		if _, exists := expectedStructs[class.Name]; exists {
			expectedStructs[class.Name] = true
		}
	}

	for name, found := range expectedStructs {
		assert.True(t, found, "Struct %s should be extracted via proper AST traversal", name)
	}

	// RED PHASE: Verify that extraction distinguishes structs from interfaces
	for _, class := range classes {
		assert.Equal(t, outbound.ConstructStruct, class.Type, "Should correctly identify struct vs interface")
	}
}

func TestExtractInterfaces_ShouldLookForTypeDeclarationWithInterfaceType(t *testing.T) {
	// RED PHASE: This test demonstrates that interfaces should be extracted from
	// type_declaration -> type_spec -> interface_type nodes
	//
	// Tree-sitter Go grammar structure:
	// type_declaration -> type_spec -> interface_type (not struct_type)

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()
	source := `package main

type Writer interface {
	Write([]byte) (int, error)
}

type Reader interface {
	Read([]byte) (int, error)
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, source)

	interfaces, err := parser.ExtractInterfaces(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	// This test will FAIL because the current implementation:
	// 1. Uses simple string parsing instead of tree-sitter node traversal
	// 2. Doesn't properly traverse type_declaration -> type_spec -> interface_type
	// 3. Doesn't distinguish interface_type from struct_type in type_spec

	assert.Len(t, interfaces, 2, "Should extract interfaces from type_declaration -> type_spec -> interface_type")

	interfaceNames := make([]string, len(interfaces))
	for i, iface := range interfaces {
		interfaceNames[i] = iface.Name
	}

	assert.Contains(t, interfaceNames, "Writer", "Should extract Writer interface from AST")
	assert.Contains(t, interfaceNames, "Reader", "Should extract Reader interface from AST")

	// RED PHASE: Verify proper interface identification
	for _, iface := range interfaces {
		assert.Equal(t, outbound.ConstructInterface, iface.Type, "Should identify as interface construct type")
		assert.True(t, iface.IsAbstract, "Interfaces should be marked as abstract")
	}
}

func TestExtractVariables_ShouldLookForVarDeclarationNodes(t *testing.T) {
	// RED PHASE: This test demonstrates that variables should be extracted from
	// "var_declaration" nodes in the AST, not by string parsing
	//
	// Tree-sitter Go grammar has var_declaration nodes for variable declarations

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()
	source := `package main

var globalConfig string = "production"
var maxConnections int = 100

const defaultTimeout = 30
const debugMode bool = false`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, source)

	variables, err := parser.ExtractVariables(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	// This test will FAIL because the current implementation:
	// 1. Uses simple string parsing instead of looking for var_declaration nodes
	// 2. Doesn't leverage tree-sitter's AST structure for variable extraction
	// 3. Should distinguish between var_declaration and const_declaration nodes

	assert.GreaterOrEqual(
		t,
		len(variables),
		4,
		"Should extract variables from var_declaration and const_declaration nodes",
	)

	variableNames := make([]string, len(variables))
	for i, variable := range variables {
		variableNames[i] = variable.Name
	}

	assert.Contains(t, variableNames, "globalConfig", "Should extract globalConfig from var_declaration node")
	assert.Contains(t, variableNames, "maxConnections", "Should extract maxConnections from var_declaration node")
	assert.Contains(t, variableNames, "defaultTimeout", "Should extract defaultTimeout from const_declaration node")
	assert.Contains(t, variableNames, "debugMode", "Should extract debugMode from const_declaration node")
}

func TestExtractImports_ShouldLookForImportDeclarationNodes(t *testing.T) {
	// RED PHASE: This test demonstrates that imports should be extracted from
	// "import_declaration" nodes which contain "import_spec" children
	//
	// Tree-sitter Go grammar structure:
	// import_declaration -> import_spec (with fields: name, path)

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()
	source := `package main

import "fmt"
import "strings"
import (
	"context"
	"time"
	json "encoding/json"
	. "path/filepath"
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, source)

	imports, err := parser.ExtractImports(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	// This test will FAIL because the current implementation:
	// 1. Uses simple string parsing instead of tree-sitter node traversal
	// 2. Doesn't look for import_declaration -> import_spec nodes
	// 3. Doesn't properly extract the "name" and "path" fields from import_spec

	assert.GreaterOrEqual(t, len(imports), 6, "Should extract imports from import_declaration nodes")

	importPaths := make([]string, len(imports))
	for i, imp := range imports {
		importPaths[i] = imp.Path
	}

	assert.Contains(t, importPaths, "fmt", "Should extract fmt from import_spec")
	assert.Contains(t, importPaths, "strings", "Should extract strings from import_spec")
	assert.Contains(t, importPaths, "context", "Should extract context from import_spec")
	assert.Contains(t, importPaths, "encoding/json", "Should extract encoding/json from import_spec")
	assert.Contains(t, importPaths, "path/filepath", "Should extract path/filepath from import_spec")

	// RED PHASE: Verify proper alias and wildcard detection from import_spec fields
	aliasedImports := make(map[string]string)
	wildcardImports := make([]string, 0)

	for _, imp := range imports {
		if imp.Alias != "" && imp.Alias != "." && imp.Alias != "_" {
			aliasedImports[imp.Path] = imp.Alias
		}
		if imp.IsWildcard {
			wildcardImports = append(wildcardImports, imp.Path)
		}
	}

	assert.Equal(t, "json", aliasedImports["encoding/json"], "Should extract alias from import_spec name field")
	assert.Contains(t, wildcardImports, "path/filepath", "Should detect wildcard import from import_spec name field")
}

func TestExtractModules_ShouldLookForPackageClauseNode(t *testing.T) {
	// RED PHASE: This test demonstrates that package names should be extracted from
	// "package_clause" nodes, not by string parsing
	//
	// Tree-sitter Go grammar structure:
	// package_clause -> _package_identifier

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()
	source := `package myservice

import "fmt"

func main() {
	fmt.Println("Hello")
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, source)

	modules, err := parser.ExtractModules(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	// This test will FAIL because the current implementation:
	// 1. Uses simple string parsing instead of looking for package_clause nodes
	// 2. Doesn't extract the _package_identifier from the package_clause
	// 3. Should verify that package_clause is properly parsed in the AST

	assert.Len(t, modules, 1, "Should extract package from package_clause node")

	packageModule := modules[0]
	assert.Equal(
		t,
		"myservice",
		packageModule.Name,
		"Should extract package name from package_clause -> _package_identifier",
	)
	assert.Equal(t, outbound.ConstructPackage, packageModule.Type, "Should identify as package construct type")

	// RED PHASE: Verify extraction was based on AST nodes, not string parsing
	assert.Greater(
		t,
		packageModule.EndByte,
		packageModule.StartByte,
		"EndByte should be greater than StartByte from AST",
	)
}

func TestASTraversal_ShouldDistinguishNodeTypesCorrectly(t *testing.T) {
	// RED PHASE: This test demonstrates that the parser should correctly distinguish
	// between different node types using proper tree-sitter AST traversal
	//
	// The parser should NOT use string parsing fallbacks when tree-sitter nodes are available

	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	ctx := context.Background()
	source := `package main

import "fmt"

type User struct {
	name string
}

type Writer interface {
	Write([]byte) (int, error)
}

var globalVar = "test"

func (u User) GetName() string {
	return u.name
}

func ProcessData(data string) {
	fmt.Println(data)
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, source)

	// This test will FAIL because the current implementation falls back to string parsing
	// when tree-sitter nodes are empty, instead of ensuring proper AST parsing first

	// Extract all construct types
	functions, err := parser.ExtractFunctions(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	classes, err := parser.ExtractClasses(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	interfaces, err := parser.ExtractInterfaces(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	variables, err := parser.ExtractVariables(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	imports, err := parser.ExtractImports(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	modules, err := parser.ExtractModules(ctx, parseTree, outbound.SemanticExtractionOptions{})
	require.NoError(t, err)

	// RED PHASE: Verify that each construct type was extracted from proper AST nodes
	assert.Len(t, functions, 2, "Should extract 1 function_declaration + 1 method_declaration from AST")
	assert.Len(t, classes, 1, "Should extract 1 struct from type_declaration -> type_spec -> struct_type")
	assert.Len(t, interfaces, 1, "Should extract 1 interface from type_declaration -> type_spec -> interface_type")
	assert.Len(t, variables, 1, "Should extract 1 variable from var_declaration")
	assert.Len(t, imports, 1, "Should extract 1 import from import_declaration -> import_spec")
	assert.Len(t, modules, 1, "Should extract 1 package from package_clause")

	// RED PHASE: Verify proper type distinction based on AST node types
	methodCount := 0
	functionCount := 0
	for _, fn := range functions {
		switch fn.Type {
		case outbound.ConstructMethod:
			methodCount++
		case outbound.ConstructFunction:
			functionCount++
		case outbound.ConstructClass, outbound.ConstructStruct, outbound.ConstructInterface,
			outbound.ConstructEnum, outbound.ConstructVariable, outbound.ConstructConstant,
			outbound.ConstructField, outbound.ConstructProperty, outbound.ConstructModule,
			outbound.ConstructPackage, outbound.ConstructNamespace, outbound.ConstructType,
			outbound.ConstructComment, outbound.ConstructDecorator, outbound.ConstructAttribute,
			outbound.ConstructLambda, outbound.ConstructClosure, outbound.ConstructGenerator,
			outbound.ConstructAsyncFunction:
			// Other construct types are not expected in function extraction
		}
	}

	assert.Equal(t, 1, methodCount, "Should distinguish method_declaration from function_declaration")
	assert.Equal(t, 1, functionCount, "Should distinguish function_declaration from method_declaration")

	// RED PHASE: Verify that all constructs have proper AST-based positions
	allConstructs := []outbound.SemanticCodeChunk{}
	allConstructs = append(allConstructs, functions...)
	allConstructs = append(allConstructs, classes...)
	allConstructs = append(allConstructs, interfaces...)
	allConstructs = append(allConstructs, variables...)
	allConstructs = append(allConstructs, modules...)

	for _, construct := range allConstructs {
		assert.Greater(t, construct.EndByte, construct.StartByte,
			"Construct %s should have AST-based positions, not hardcoded values", construct.Name)
	}
}

func TestTreeSitterParseResult_ShouldContainProperASTNodes(t *testing.T) {
	// RED PHASE: This test demonstrates that the ParseSource method should return
	// a ParseTree with actual tree-sitter AST nodes, not empty placeholder nodes
	//
	// The current implementation creates empty ParseTree structures instead of
	// using real tree-sitter parsing

	source := `package main

func main() {
	println("Hello")
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, source)

	// This test will FAIL because the current implementation:
	// 1. Creates empty ParseTree structures in Parse() method
	// 2. GetNodesByType() returns empty slices because no real AST nodes exist
	// 3. The tree-sitter library is not actually being used to parse Go source code

	// RED PHASE: Verify that the tree contains actual parsed nodes
	packageNodes := parseTree.GetNodesByType("package_clause")
	functionNodes := parseTree.GetNodesByType("function_declaration")

	assert.NotEmpty(t, packageNodes, "ParseTree should contain actual package_clause nodes from tree-sitter")
	assert.NotEmpty(
		t,
		functionNodes,
		"ParseTree should contain actual function_declaration nodes from tree-sitter",
	)

	// RED PHASE: Verify that nodes have proper tree-sitter properties
	if len(functionNodes) > 0 {
		funcNode := functionNodes[0]
		assert.NotEmpty(t, funcNode.Type, "AST node should have proper type from tree-sitter")
		assert.NotEmpty(t, funcNode.Children, "Function node should have children (name, parameters, body)")

		// Verify that we can access node text from the original source
		nodeText := parseTree.GetNodeText(funcNode)
		assert.Contains(t, nodeText, "func", "Node text should contain actual source code")
		assert.Contains(t, nodeText, "main", "Node text should contain function name")
	}
}

// Helper functions for test support

func createMockParseTreeFromSource(
	t *testing.T,
	lang valueobject.Language,
	sourceCode string,
) *valueobject.ParseTree {
	t.Helper()

	// REFACTOR PHASE: Use real tree-sitter parsing instead of mock
	return createRealParseTreeFromSource(t, lang, sourceCode)
}

// createRealParseTreeFromSource creates a ParseTree using actual tree-sitter parsing.
func createRealParseTreeFromSource(
	t *testing.T,
	lang valueobject.Language,
	sourceCode string,
) *valueobject.ParseTree {
	t.Helper()

	// Get Go grammar from forest
	grammar := forest.GetLanguage("go")
	require.NotNil(t, grammar, "Failed to get Go grammar from forest")

	// Create tree-sitter parser
	parser := tree_sitter.NewParser()
	require.NotNil(t, parser, "Failed to create tree-sitter parser")

	success := parser.SetLanguage(grammar)
	require.True(t, success, "Failed to set Go language")

	// Parse the source code
	tree, err := parser.ParseString(context.Background(), nil, []byte(sourceCode))
	require.NoError(t, err, "Failed to parse Go source")
	require.NotNil(t, tree, "Parse tree should not be nil")
	defer tree.Close()

	// Convert tree-sitter tree to domain ParseNode
	rootTSNode := tree.RootNode()
	rootNode, nodeCount, maxDepth := convertTreeSitterNode(rootTSNode, 0)

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

	// Create and return ParseTree
	parseTree, err := valueobject.NewParseTree(
		context.Background(),
		lang,
		rootNode,
		[]byte(sourceCode),
		metadata,
	)
	require.NoError(t, err, "Failed to create ParseTree")

	return parseTree
}

func convertTreeSitterNode(node tree_sitter.Node, depth int) (*valueobject.ParseNode, int, int) {
	if node.IsNull() {
		return nil, 0, depth
	}

	// Convert tree-sitter node to domain ParseNode
	parseNode := &valueobject.ParseNode{
		Type:      node.Type(),
		StartByte: safeUintToUint32(node.StartByte()),
		EndByte:   safeUintToUint32(node.EndByte()),
		StartPos: valueobject.Position{
			Row:    safeUintToUint32(node.StartPoint().Row),
			Column: safeUintToUint32(node.StartPoint().Column),
		},
		EndPos: valueobject.Position{
			Row:    safeUintToUint32(node.EndPoint().Row),
			Column: safeUintToUint32(node.EndPoint().Column),
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

		childParseNode, childNodeCount, childMaxDepth := convertTreeSitterNode(childNode, depth+1)
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

// safeUintToUint32 safely converts uint to uint32 with bounds checking.
func safeUintToUint32(val uint) uint32 {
	if val > uint(^uint32(0)) {
		return ^uint32(0) // Return max uint32 if overflow would occur
	}
	return uint32(val)
}
