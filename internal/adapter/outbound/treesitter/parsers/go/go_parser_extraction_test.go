package goparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestObservableGoParser_ExtractClasses_SimpleStruct tests extraction of basic Go structs.
// This is a RED PHASE test that defines expected behavior for struct extraction.
// Based on tree-sitter Go grammar: struct_type contains field_declaration_list.
func TestObservableGoParser_ExtractClasses_SimpleStruct(t *testing.T) {
	sourceCode := `package main

type User struct {
	ID   int    ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
	age  int    // private field
}

type Product struct {
	Title string
	Price float64
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result, err := parser.ExtractClasses(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, 2, "Should extract 2 struct definitions")

	// Test User struct
	userStruct := result[0]
	assert.Equal(t, outbound.ConstructStruct, userStruct.Type)
	assert.Equal(t, "User", userStruct.Name)
	assert.Equal(t, "main.User", userStruct.QualifiedName)
	assert.Equal(t, outbound.Public, userStruct.Visibility)
	assert.False(t, userStruct.IsAbstract)
	assert.False(t, userStruct.IsGeneric)
	assert.Contains(t, userStruct.Content, "type User struct")
	assert.NotEmpty(t, userStruct.ChunkID)
	assert.NotEmpty(t, userStruct.Hash)

	// Test Product struct
	productStruct := result[1]
	assert.Equal(t, outbound.ConstructStruct, productStruct.Type)
	assert.Equal(t, "Product", productStruct.Name)
	assert.Equal(t, "main.Product", productStruct.QualifiedName)
	assert.Equal(t, outbound.Public, productStruct.Visibility)
}

// TestObservableGoParser_ExtractClasses_GenericStruct tests extraction of generic Go structs.
// This is a RED PHASE test that defines expected behavior for generic struct extraction.
func TestObservableGoParser_ExtractClasses_GenericStruct(t *testing.T) {
	sourceCode := `package main

type Container[T any] struct {
	items []T
	size  int
}

type Pair[K comparable, V any] struct {
	Key   K
	Value V
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result, err := parser.ExtractClasses(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, 2, "Should extract 2 generic struct definitions")

	// Test Container struct
	containerStruct := result[0]
	assert.Equal(t, "Container", containerStruct.Name)
	assert.True(t, containerStruct.IsGeneric)
	require.Len(t, containerStruct.GenericParameters, 1)
	assert.Equal(t, "T", containerStruct.GenericParameters[0].Name)
	assert.Contains(t, containerStruct.GenericParameters[0].Constraints, "any")

	// Test Pair struct
	pairStruct := result[1]
	assert.Equal(t, "Pair", pairStruct.Name)
	assert.True(t, pairStruct.IsGeneric)
	require.Len(t, pairStruct.GenericParameters, 2)
	assert.Equal(t, "K", pairStruct.GenericParameters[0].Name)
	assert.Contains(t, pairStruct.GenericParameters[0].Constraints, "comparable")
	assert.Equal(t, "V", pairStruct.GenericParameters[1].Name)
	assert.Contains(t, pairStruct.GenericParameters[1].Constraints, "any")
}

// TestObservableGoParser_ExtractClasses_EmbeddedStruct tests extraction of structs with embedded fields.
// This is a RED PHASE test that defines expected behavior for embedded struct extraction.
func TestObservableGoParser_ExtractClasses_EmbeddedStruct(t *testing.T) {
	sourceCode := `package main

type Base struct {
	ID        int
	CreatedAt time.Time
}

type User struct {
	Base
	Name  string
	Email string
}

type Admin struct {
	User
	Permissions []string
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result, err := parser.ExtractClasses(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, 3, "Should extract 3 struct definitions")

	// Validate that embedded structs are detected
	userStruct := result[1] // User struct
	assert.Equal(t, "User", userStruct.Name)
	assert.Contains(t, userStruct.Content, "Base") // Should contain embedded struct

	adminStruct := result[2] // Admin struct
	assert.Equal(t, "Admin", adminStruct.Name)
	assert.Contains(t, adminStruct.Content, "User") // Should contain embedded struct
}

// TestObservableGoParser_ExtractClasses_PrivateStructFiltering tests visibility filtering for structs.
// This is a RED PHASE test that defines expected behavior for private struct filtering.
func TestObservableGoParser_ExtractClasses_PrivateStructFiltering(t *testing.T) {
	sourceCode := `package main

type PublicStruct struct {
	Field string
}

type privateStruct struct {
	field string
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	// Test with IncludePrivate: false
	optionsNoPrivate := outbound.SemanticExtractionOptions{
		IncludePrivate:  false,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result, err := parser.ExtractClasses(context.Background(), parseTree, optionsNoPrivate)
	require.NoError(t, err)
	require.Len(t, result, 1, "Should only extract public struct when IncludePrivate is false")
	assert.Equal(t, "PublicStruct", result[0].Name)

	// Test with IncludePrivate: true
	optionsIncludePrivate := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result, err = parser.ExtractClasses(context.Background(), parseTree, optionsIncludePrivate)
	require.NoError(t, err)
	require.Len(t, result, 2, "Should extract both public and private structs when IncludePrivate is true")
	assert.Equal(t, "PublicStruct", result[0].Name)
	assert.Equal(t, "privateStruct", result[1].Name)
	assert.Equal(t, outbound.Private, result[1].Visibility)
}

// TestObservableGoParser_ExtractInterfaces_SimpleInterface tests extraction of basic Go interfaces.
// This is a RED PHASE test that defines expected behavior for interface extraction.
// Based on tree-sitter Go grammar: interface_type contains method specifications.
func TestObservableGoParser_ExtractInterfaces_SimpleInterface(t *testing.T) {
	sourceCode := `package main

type Reader interface {
	Read([]byte) (int, error)
}

type Writer interface {
	Write([]byte) (int, error)
	Close() error
}

type ReadWriter interface {
	Reader
	Writer
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result, err := parser.ExtractInterfaces(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, 3, "Should extract 3 interface definitions")

	// Test Reader interface
	readerInterface := result[0]
	assert.Equal(t, outbound.ConstructInterface, readerInterface.Type)
	assert.Equal(t, "Reader", readerInterface.Name)
	assert.Equal(t, "main.Reader", readerInterface.QualifiedName)
	assert.Equal(t, outbound.Public, readerInterface.Visibility)
	assert.True(t, readerInterface.IsAbstract)
	assert.False(t, readerInterface.IsGeneric)
	assert.Contains(t, readerInterface.Content, "type Reader interface")
	assert.NotEmpty(t, readerInterface.ChunkID)
	assert.NotEmpty(t, readerInterface.Hash)

	// Test Writer interface
	writerInterface := result[1]
	assert.Equal(t, "Writer", writerInterface.Name)
	assert.Contains(t, writerInterface.Content, "Write")
	assert.Contains(t, writerInterface.Content, "Close")

	// Test ReadWriter interface (embedded interfaces)
	readWriterInterface := result[2]
	assert.Equal(t, "ReadWriter", readWriterInterface.Name)
	assert.Contains(t, readWriterInterface.Content, "Reader")
	assert.Contains(t, readWriterInterface.Content, "Writer")
}

// TestObservableGoParser_ExtractInterfaces_GenericInterface tests extraction of generic Go interfaces.
// This is a RED PHASE test that defines expected behavior for generic interface extraction.
func TestObservableGoParser_ExtractInterfaces_GenericInterface(t *testing.T) {
	sourceCode := `package main

type Comparable[T any] interface {
	Compare(T) int
}

type Container[T any] interface {
	Add(T)
	Get(int) (T, bool)
	Size() int
}

type Mapper[K comparable, V any] interface {
	Set(K, V)
	Get(K) (V, bool)
	Delete(K)
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result, err := parser.ExtractInterfaces(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, 3, "Should extract 3 generic interface definitions")

	// Test Comparable interface
	comparableInterface := result[0]
	assert.Equal(t, "Comparable", comparableInterface.Name)
	assert.True(t, comparableInterface.IsGeneric)
	require.Len(t, comparableInterface.GenericParameters, 1)
	assert.Equal(t, "T", comparableInterface.GenericParameters[0].Name)
	assert.Contains(t, comparableInterface.GenericParameters[0].Constraints, "any")

	// Test Mapper interface (multiple type parameters)
	mapperInterface := result[2]
	assert.Equal(t, "Mapper", mapperInterface.Name)
	assert.True(t, mapperInterface.IsGeneric)
	require.Len(t, mapperInterface.GenericParameters, 2)
	assert.Equal(t, "K", mapperInterface.GenericParameters[0].Name)
	assert.Contains(t, mapperInterface.GenericParameters[0].Constraints, "comparable")
	assert.Equal(t, "V", mapperInterface.GenericParameters[1].Name)
	assert.Contains(t, mapperInterface.GenericParameters[1].Constraints, "any")
}

// TestObservableGoParser_ExtractInterfaces_EmptyInterface tests extraction of empty interfaces.
// This is a RED PHASE test that defines expected behavior for empty interface extraction.
func TestObservableGoParser_ExtractInterfaces_EmptyInterface(t *testing.T) {
	sourceCode := `package main

type Any interface{}

type Empty interface {
}

type Marker interface {
	// Marker interface
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result, err := parser.ExtractInterfaces(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, 3, "Should extract 3 interface definitions including empty ones")

	// Test that empty interfaces are properly identified
	for _, iface := range result {
		assert.Equal(t, outbound.ConstructInterface, iface.Type)
		assert.True(t, iface.IsAbstract)
		assert.Contains(t, []string{"Any", "Empty", "Marker"}, iface.Name)
	}
}

// TestObservableGoParser_ExtractVariables_GlobalVariables tests extraction of global variable declarations.
// This is a RED PHASE test that defines expected behavior for variable extraction.
// Based on tree-sitter Go grammar: var_declaration contains var_spec nodes.
func TestObservableGoParser_ExtractVariables_GlobalVariables(t *testing.T) {
	sourceCode := `package main

var GlobalVar = "test"
var PublicNumber int = 42
var privateString string

var (
	BatchVar1 = 100
	BatchVar2 = "batch"
	batchVar3 int
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result, err := parser.ExtractVariables(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, 6, "Should extract 6 variable declarations")

	// Test individual variables
	expectedVars := []struct {
		name       string
		visibility outbound.VisibilityModifier
		varType    outbound.SemanticConstructType
	}{
		{"GlobalVar", outbound.Public, outbound.ConstructVariable},
		{"PublicNumber", outbound.Public, outbound.ConstructVariable},
		{"privateString", outbound.Private, outbound.ConstructVariable},
		{"BatchVar1", outbound.Public, outbound.ConstructVariable},
		{"BatchVar2", outbound.Public, outbound.ConstructVariable},
		{"batchVar3", outbound.Private, outbound.ConstructVariable},
	}

	for i, expected := range expectedVars {
		assert.Equal(t, expected.varType, result[i].Type)
		assert.Equal(t, expected.name, result[i].Name)
		assert.Equal(t, "main."+expected.name, result[i].QualifiedName)
		assert.Equal(t, expected.visibility, result[i].Visibility)
		assert.True(t, result[i].IsStatic)
		assert.NotEmpty(t, result[i].ChunkID)
		assert.NotEmpty(t, result[i].Hash)
	}
}

// TestObservableGoParser_ExtractVariables_Constants tests extraction of constant declarations.
// This is a RED PHASE test that defines expected behavior for constant extraction.
// Based on tree-sitter Go grammar: const_declaration contains const_spec nodes.
func TestObservableGoParser_ExtractVariables_Constants(t *testing.T) {
	sourceCode := `package main

const Pi = 3.14159
const MaxRetries int = 5
const debug = false

const (
	StatusOK     = 200
	StatusError  = 500
	statusPending = 100
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result, err := parser.ExtractVariables(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, 6, "Should extract 6 constant declarations")

	// Test constant-specific properties
	expectedConsts := []struct {
		name       string
		visibility outbound.VisibilityModifier
	}{
		{"Pi", outbound.Public},
		{"MaxRetries", outbound.Public},
		{"debug", outbound.Private},
		{"StatusOK", outbound.Public},
		{"StatusError", outbound.Public},
		{"statusPending", outbound.Private},
	}

	for i, expected := range expectedConsts {
		assert.Equal(t, outbound.ConstructConstant, result[i].Type)
		assert.Equal(t, expected.name, result[i].Name)
		assert.Equal(t, expected.visibility, result[i].Visibility)
		assert.True(t, result[i].IsStatic)
	}
}

// TestObservableGoParser_ExtractVariables_TypeAliases tests extraction of type alias declarations.
// This is a RED PHASE test that defines expected behavior for type alias extraction.
func TestObservableGoParser_ExtractVariables_TypeAliases(t *testing.T) {
	sourceCode := `package main

type UserID int64
type Email string
type userRole string

type StringSlice []string
type UserMap map[string]User`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result, err := parser.ExtractVariables(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, 5, "Should extract 5 type alias declarations")

	// Test type aliases
	expectedTypes := []struct {
		name         string
		visibility   outbound.VisibilityModifier
		expectedType string
	}{
		{"UserID", outbound.Public, "int64"},
		{"Email", outbound.Public, "string"},
		{"userRole", outbound.Private, "string"},
		{"StringSlice", outbound.Public, "[]string"},
		{"UserMap", outbound.Public, "map[string]User"},
	}

	for i, expected := range expectedTypes {
		assert.Equal(t, outbound.ConstructType, result[i].Type)
		assert.Equal(t, expected.name, result[i].Name)
		assert.Equal(t, expected.visibility, result[i].Visibility)
		assert.Equal(t, expected.expectedType, result[i].ReturnType)
		assert.True(t, result[i].IsStatic)
	}
}

// TestObservableGoParser_ExtractImports_SimpleImports tests extraction of simple import statements.
// This is a RED PHASE test that defines expected behavior for import extraction.
// Based on tree-sitter Go grammar: import_declaration contains import_spec nodes.
func TestObservableGoParser_ExtractImports_SimpleImports(t *testing.T) {
	sourceCode := `package main

import "fmt"
import "os"
import "strings"`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result, err := parser.ExtractImports(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, 3, "Should extract 3 import declarations")

	expectedPaths := []string{"fmt", "os", "strings"}
	for i, expected := range expectedPaths {
		assert.Equal(t, expected, result[i].Path)
		assert.Empty(t, result[i].Alias)
		assert.False(t, result[i].IsWildcard)
		assert.Empty(t, result[i].ImportedSymbols)
		assert.NotEmpty(t, result[i].Hash)
		assert.Contains(t, result[i].Content, expected)
	}
}

// TestObservableGoParser_ExtractImports_ImportBlock tests extraction of import blocks.
// This is a RED PHASE test that defines expected behavior for import block extraction.
func TestObservableGoParser_ExtractImports_ImportBlock(t *testing.T) {
	sourceCode := `package main

import (
	"fmt"
	"os"
	"strings"
	"encoding/json"
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result, err := parser.ExtractImports(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, 4, "Should extract 4 import declarations from block")

	expectedPaths := []string{"fmt", "os", "strings", "encoding/json"}
	for i, expected := range expectedPaths {
		assert.Equal(t, expected, result[i].Path)
		assert.Empty(t, result[i].Alias)
		assert.False(t, result[i].IsWildcard)
	}
}

// TestObservableGoParser_ExtractImports_NamedImports tests extraction of aliased/named imports.
// This is a RED PHASE test that defines expected behavior for named import extraction.
func TestObservableGoParser_ExtractImports_NamedImports(t *testing.T) {
	sourceCode := `package main

import (
	"fmt"
	. "strings"
	_ "database/sql/driver"
	json "encoding/json"
	mypackage "github.com/user/package"
)`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result, err := parser.ExtractImports(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, 5, "Should extract 5 import declarations with various alias types")

	expectedImports := []struct {
		path       string
		alias      string
		isWildcard bool
	}{
		{"fmt", "", false},
		{"strings", ".", true},                          // dot import
		{"database/sql/driver", "_", false},             // blank identifier
		{"encoding/json", "json", false},                // named alias
		{"github.com/user/package", "mypackage", false}, // named alias
	}

	for i, expected := range expectedImports {
		assert.Equal(t, expected.path, result[i].Path)
		assert.Equal(t, expected.alias, result[i].Alias)
		assert.Equal(t, expected.isWildcard, result[i].IsWildcard)
	}
}

// TestObservableGoParser_ExtractModules_PackageDeclaration tests extraction of package declarations.
// This is a RED PHASE test that defines expected behavior for module/package extraction.
// Based on tree-sitter Go grammar: package_clause contains package identifier.
func TestObservableGoParser_ExtractModules_PackageDeclaration(t *testing.T) {
	sourceCode := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result, err := parser.ExtractModules(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, result, 1, "Should extract 1 package declaration")

	pkg := result[0]
	assert.Equal(t, outbound.ConstructPackage, pkg.Type)
	assert.Equal(t, "main", pkg.Name)
	assert.Equal(t, "main", pkg.QualifiedName) // Package name is the qualified name
	assert.Equal(t, outbound.Public, pkg.Visibility)
	assert.Contains(t, pkg.Content, "package main")
	assert.NotEmpty(t, pkg.ChunkID)
	assert.NotEmpty(t, pkg.Hash)
	assert.True(t, pkg.IsStatic)
}

// TestObservableGoParser_ExtractModules_DifferentPackageNames tests extraction of various package names.
// This is a RED PHASE test that defines expected behavior for different package name extraction.
func TestObservableGoParser_ExtractModules_DifferentPackageNames(t *testing.T) {
	testCases := []struct {
		name        string
		sourceCode  string
		expectedPkg string
	}{
		{
			name: "main package",
			sourceCode: `package main

func main() {}`,
			expectedPkg: "main",
		},
		{
			name: "library package",
			sourceCode: `package utils

func Helper() {}`,
			expectedPkg: "utils",
		},
		{
			name: "test package",
			sourceCode: `package mypackage_test

import "testing"

func TestSomething(t *testing.T) {}`,
			expectedPkg: "mypackage_test",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			language, err := valueobject.NewLanguage(valueobject.LanguageGo)
			require.NoError(t, err)

			parseTree := createMockParseTreeFromSource(t, language, tc.sourceCode)
			parserInterface, err := NewGoParser()
			require.NoError(t, err)
			parser := parserInterface.(*ObservableGoParser)

			options := outbound.SemanticExtractionOptions{
				IncludePrivate:  true,
				IncludeTypeInfo: true,
				MaxDepth:        10,
			}

			result, err := parser.ExtractModules(context.Background(), parseTree, options)
			require.NoError(t, err)
			require.Len(t, result, 1, "Should extract 1 package declaration")

			pkg := result[0]
			assert.Equal(t, tc.expectedPkg, pkg.Name)
			assert.Equal(t, tc.expectedPkg, pkg.QualifiedName)
		})
	}
}

// TestObservableGoParser_ErrorHandling_NilInputs tests error handling for extraction methods.
// This is a RED PHASE test that defines expected behavior for error conditions.
func TestObservableGoParser_ErrorHandling_NilInputs(t *testing.T) {
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	t.Run("ExtractClasses with nil parse tree", func(t *testing.T) {
		result, err := parser.ExtractClasses(context.Background(), nil, options)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("ExtractInterfaces with nil parse tree", func(t *testing.T) {
		result, err := parser.ExtractInterfaces(context.Background(), nil, options)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("ExtractVariables with nil parse tree", func(t *testing.T) {
		result, err := parser.ExtractVariables(context.Background(), nil, options)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("ExtractImports with nil parse tree", func(t *testing.T) {
		result, err := parser.ExtractImports(context.Background(), nil, options)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("ExtractModules with nil parse tree", func(t *testing.T) {
		result, err := parser.ExtractModules(context.Background(), nil, options)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// TestObservableGoParser_ErrorHandling_EmptySource tests error handling for empty source code.
// This is a RED PHASE test that defines expected behavior for empty source handling.
func TestObservableGoParser_ErrorHandling_EmptySource(t *testing.T) {
	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, "")
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	t.Run("ExtractClasses with empty source", func(t *testing.T) {
		result, err := parser.ExtractClasses(context.Background(), parseTree, options)
		require.NoError(t, err)
		assert.Empty(t, result, "Should return empty slice for empty source")
	})

	t.Run("ExtractInterfaces with empty source", func(t *testing.T) {
		result, err := parser.ExtractInterfaces(context.Background(), parseTree, options)
		require.NoError(t, err)
		assert.Empty(t, result, "Should return empty slice for empty source")
	})

	t.Run("ExtractVariables with empty source", func(t *testing.T) {
		result, err := parser.ExtractVariables(context.Background(), parseTree, options)
		require.NoError(t, err)
		assert.Empty(t, result, "Should return empty slice for empty source")
	})

	t.Run("ExtractImports with empty source", func(t *testing.T) {
		result, err := parser.ExtractImports(context.Background(), parseTree, options)
		require.NoError(t, err)
		assert.Empty(t, result, "Should return empty slice for empty source")
	})

	t.Run("ExtractModules with empty source", func(t *testing.T) {
		result, err := parser.ExtractModules(context.Background(), parseTree, options)
		require.NoError(t, err)
		assert.Empty(t, result, "Should return empty slice for empty source")
	})
}

// TestObservableGoParser_PrivateVisibilityFiltering tests visibility filtering across all extraction methods.
// This is a RED PHASE test that defines expected behavior for private element filtering.
func TestObservableGoParser_PrivateVisibilityFiltering(t *testing.T) {
	sourceCode := `package main

type PublicStruct struct {
	Field string
}

type privateStruct struct {
	field string
}

type PublicInterface interface {
	Method()
}

type privateInterface interface {
	method()
}

var PublicVar = "public"
var privateVar = "private"

const PublicConst = 42
const privateConst = 24`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	// Test with IncludePrivate: false
	optionsNoPrivate := outbound.SemanticExtractionOptions{
		IncludePrivate:  false,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	// Test classes (structs)
	classes, err := parser.ExtractClasses(context.Background(), parseTree, optionsNoPrivate)
	require.NoError(t, err)
	require.Len(t, classes, 1, "Should only extract public struct")
	assert.Equal(t, "PublicStruct", classes[0].Name)

	// Test interfaces
	interfaces, err := parser.ExtractInterfaces(context.Background(), parseTree, optionsNoPrivate)
	require.NoError(t, err)
	require.Len(t, interfaces, 1, "Should only extract public interface")
	assert.Equal(t, "PublicInterface", interfaces[0].Name)

	// Test variables and constants
	variables, err := parser.ExtractVariables(context.Background(), parseTree, optionsNoPrivate)
	require.NoError(t, err)
	publicCount := 0
	for _, v := range variables {
		if v.Visibility == outbound.Public {
			publicCount++
		}
	}
	assert.Positive(t, publicCount, "Should extract some public variables/constants")

	// Test with IncludePrivate: true
	optionsIncludePrivate := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	// Test that private elements are included
	classesWithPrivate, err := parser.ExtractClasses(context.Background(), parseTree, optionsIncludePrivate)
	require.NoError(t, err)
	assert.Greater(t, len(classesWithPrivate), len(classes), "Should extract more structs when including private")

	interfacesWithPrivate, err := parser.ExtractInterfaces(context.Background(), parseTree, optionsIncludePrivate)
	require.NoError(t, err)
	assert.Greater(
		t,
		len(interfacesWithPrivate),
		len(interfaces),
		"Should extract more interfaces when including private",
	)
}

// TestObservableGoParser_Integration_ComplexFile tests extraction across a complex Go file.
// This is a RED PHASE test that defines expected behavior for complex file extraction.
func TestObservableGoParser_Integration_ComplexFile(t *testing.T) {
	sourceCode := `package mypackage

import (
	"fmt"
	"strings"
	json "encoding/json"
)

const (
	MaxUsers = 1000
	Version  = "1.0.0"
)

var (
	GlobalCounter int
	userCache     map[string]*User
)

type UserID int64

type User struct {
	ID    UserID ` + "`json:\"id\"`" + `
	Name  string ` + "`json:\"name\"`" + `
	Email string ` + "`json:\"email\"`" + `
}

type Repository[T any] interface {
	Save(T) error
	FindByID(UserID) (T, error)
	Delete(UserID) error
}

type UserRepository struct {
	users map[UserID]*User
}

func (r *UserRepository) Save(user *User) error {
	r.users[user.ID] = user
	return nil
}

func NewUser(name, email string) *User {
	return &User{
		Name:  name,
		Email: email,
	}
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parserInterface, err := NewGoParser()
	require.NoError(t, err)
	parser := parserInterface.(*ObservableGoParser)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	// Test package extraction
	modules, err := parser.ExtractModules(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, modules, 1)
	assert.Equal(t, "mypackage", modules[0].Name)

	// Test import extraction
	imports, err := parser.ExtractImports(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, imports, 3)

	// Test struct extraction
	classes, err := parser.ExtractClasses(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, classes, 2) // User and UserRepository

	structNames := make([]string, len(classes))
	for i, class := range classes {
		structNames[i] = class.Name
	}
	assert.Contains(t, structNames, "User")
	assert.Contains(t, structNames, "UserRepository")

	// Test interface extraction
	interfaces, err := parser.ExtractInterfaces(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, interfaces, 1)
	assert.Equal(t, "Repository", interfaces[0].Name)
	assert.True(t, interfaces[0].IsGeneric)

	// Test variable/constant extraction
	variables, err := parser.ExtractVariables(context.Background(), parseTree, options)
	require.NoError(t, err)
	assert.NotEmpty(t, variables, "Should extract variables, constants, and type aliases")

	// Verify that all extracted items have proper qualified names
	for _, class := range classes {
		assert.True(t, strings.HasPrefix(class.QualifiedName, "mypackage."))
	}
	for _, iface := range interfaces {
		assert.True(t, strings.HasPrefix(iface.QualifiedName, "mypackage."))
	}
	for _, variable := range variables {
		assert.True(t, strings.HasPrefix(variable.QualifiedName, "mypackage."))
	}
}
