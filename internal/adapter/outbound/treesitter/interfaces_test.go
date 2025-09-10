//go:build disabled
// +build disabled

package treesitter

import (
	"codechunking/internal/adapter/outbound/treesitter/utils"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: Interfaces moved to interfaces.go for production use

// =============================================================================
// INTERFACE DEFINITION TESTS
// =============================================================================

// TestLanguageParser_InterfaceDefinition tests that the LanguageParser interface is properly defined.
// This is a RED PHASE test that ensures the interface contract exists.
func TestLanguageParser_InterfaceDefinition(t *testing.T) {
	t.Run("interface should have all required methods", func(t *testing.T) {
		// This test will fail initially because we haven't implemented the interface yet
		var parser LanguageParser

		// Test that the interface can be assigned (compilation test)
		require.Nil(t, parser) // parser should be nil initially

		// Test method signatures exist (this will fail during compilation if methods are missing)
		ctx := context.Background()
		options := outbound.SemanticExtractionOptions{}

		if parser != nil {
			// These calls will panic with nil pointer, but the important part is type checking
			_, _ = parser.ExtractFunctions(ctx, nil, options)
			_, _ = parser.ExtractClasses(ctx, nil, options)
			_, _ = parser.ExtractInterfaces(ctx, nil, options)
			_, _ = parser.ExtractVariables(ctx, nil, options)
			_, _ = parser.ExtractImports(ctx, nil, options)
			_, _ = parser.ExtractModules(ctx, nil, options)
			_ = parser.GetSupportedLanguage()
			_ = parser.GetSupportedConstructTypes()
			goLang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
			_ = parser.IsSupported(goLang)
		}
	})
}

// TestGoLanguageParser_ExtendedInterface tests the Go-specific parser interface.
// This is a RED PHASE test that defines Go-specific parsing behavior.
func TestGoLanguageParser_ExtendedInterface(t *testing.T) {
	t.Run("Go parser should extend base LanguageParser", func(t *testing.T) {
		// This test will fail because GoLanguageParser doesn't exist yet
		var goParser GoLanguageParser

		require.Nil(t, goParser)

		// Test that GoLanguageParser can be used as LanguageParser
		var baseParser LanguageParser = goParser
		require.Nil(t, baseParser)
	})
}

// =============================================================================
// GO PARSER IMPLEMENTATION TESTS
// =============================================================================

// TestGoParser_Creation tests Go parser instantiation.
// This is a RED PHASE test that defines how Go parsers should be created.
func TestGoParser_Creation(t *testing.T) {
	t.Run("should create Go parser successfully", func(t *testing.T) {
		// This will fail because NewGoParser doesn't exist yet
		parser := NewGoParser()
		require.NotNil(t, parser)
		goLang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
		assert.Equal(t, goLang, parser.GetSupportedLanguage())
	})

	t.Run("should support Go language", func(t *testing.T) {
		parser := NewGoParser()
		require.NotNil(t, parser)

		goLang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
		jsLang, _ := valueobject.NewLanguage(valueobject.LanguageJavaScript)
		pyLang, _ := valueobject.NewLanguage(valueobject.LanguagePython)

		assert.True(t, parser.IsSupported(goLang))
		assert.False(t, parser.IsSupported(jsLang))
		assert.False(t, parser.IsSupported(pyLang))
	})

	t.Run("should return expected construct types", func(t *testing.T) {
		parser := NewGoParser()
		require.NotNil(t, parser)

		supportedTypes := parser.GetSupportedConstructTypes()
		expectedTypes := []outbound.SemanticConstructType{
			outbound.ConstructFunction,
			outbound.ConstructMethod,
			outbound.ConstructStruct,
			outbound.ConstructInterface,
			outbound.ConstructVariable,
			outbound.ConstructConstant,
			outbound.ConstructPackage,
		}

		assert.ElementsMatch(t, expectedTypes, supportedTypes)
	})
}

// TestGoParser_FunctionExtraction tests Go function extraction behavior.
// This is a RED PHASE test that defines expected function parsing behavior.
func TestGoParser_FunctionExtraction(t *testing.T) {
	sourceCode := `package main

import "fmt"

func Add(a int, b int) int {
	return a + b
}

func (c Calculator) Multiply(x, y float64) float64 {
	return x * y
}

func main() {
	fmt.Println("Hello, World!")
}`

	t.Run("should extract regular functions", func(t *testing.T) {
		parser := NewGoParser()
		require.NotNil(t, parser)

		// Create mock parse tree (this will need to be implemented)
		parseTree := createMockGoParseTree(t, sourceCode)
		ctx := context.Background()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeComments: false,
		}

		functions, err := parser.ExtractFunctions(ctx, parseTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 3) // Add, Multiply (method), main

		// Test Add function
		addFunc := findFunctionByName(functions, "Add")
		require.NotNil(t, addFunc)
		assert.Equal(t, outbound.ConstructFunction, addFunc.Type)
		assert.Equal(t, "Add", addFunc.Name)
		assert.Equal(t, "main.Add", addFunc.QualifiedName)
		assert.Equal(t, outbound.Public, addFunc.Visibility)
		assert.Len(t, addFunc.Parameters, 2)
		assert.Equal(t, "int", addFunc.ReturnType)

		// Test method
		multiplyMethod := findFunctionByName(functions, "Multiply")
		require.NotNil(t, multiplyMethod)
		assert.Equal(t, outbound.ConstructMethod, multiplyMethod.Type)
		assert.Equal(t, "Calculator.Multiply", multiplyMethod.QualifiedName)
	})

	t.Run("should respect privacy options", func(t *testing.T) {
		parser := NewGoParser()
		require.NotNil(t, parser)

		parseTree := createMockGoParseTree(t, `package main

func PublicFunc() {}
func privateFunc() {}`)

		ctx := context.Background()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate: false,
		}

		functions, err := parser.ExtractFunctions(ctx, parseTree, options)
		require.NoError(t, err)

		// Should only include public functions
		publicFuncs := filterPublicFunctions(functions)
		assert.Len(t, publicFuncs, 1)
		assert.Equal(t, "PublicFunc", publicFuncs[0].Name)
	})
}

// TestGoParser_StructExtraction tests Go struct extraction behavior.
// This is a RED PHASE test that defines expected struct parsing behavior.
func TestGoParser_StructExtraction(t *testing.T) {
	sourceCode := `package main

type User struct {
	ID   int    ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

type Calculator struct {
	precision int
}

type privateStruct struct {
	data string
}`

	t.Run("should extract struct definitions", func(t *testing.T) {
		parser := NewGoParser()
		require.NotNil(t, parser)

		parseTree := createMockGoParseTree(t, sourceCode)
		ctx := context.Background()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate: true,
		}

		structs, err := parser.ExtractClasses(ctx, parseTree, options)
		require.NoError(t, err)
		require.Len(t, structs, 3)

		// Test User struct
		userStruct := findStructByName(structs, "User")
		require.NotNil(t, userStruct)
		assert.Equal(t, outbound.ConstructStruct, userStruct.Type)
		assert.Equal(t, "User", userStruct.Name)
		assert.Equal(t, "main.User", userStruct.QualifiedName)
		assert.Equal(t, outbound.Public, userStruct.Visibility)
	})
}

// TestGoParser_InterfaceExtraction tests Go interface extraction behavior.
// This is a RED PHASE test that defines expected interface parsing behavior.
func TestGoParser_InterfaceExtraction(t *testing.T) {
	sourceCode := `package main

type Reader interface {
	Read([]byte) (int, error)
}

type Writer interface {
	Write([]byte) (int, error)
}

type ReadWriter interface {
	Reader
	Writer
}`

	t.Run("should extract interface definitions", func(t *testing.T) {
		parser := NewGoParser()
		require.NotNil(t, parser)

		parseTree := createMockGoParseTree(t, sourceCode)
		ctx := context.Background()
		options := outbound.SemanticExtractionOptions{}

		interfaces, err := parser.ExtractInterfaces(ctx, parseTree, options)
		require.NoError(t, err)
		require.Len(t, interfaces, 3)

		// Test Reader interface
		readerInterface := findInterfaceByName(interfaces, "Reader")
		require.NotNil(t, readerInterface)
		assert.Equal(t, outbound.ConstructInterface, readerInterface.Type)
		assert.Equal(t, "Reader", readerInterface.Name)
		assert.Equal(t, "main.Reader", readerInterface.QualifiedName)
	})
}

// TestGoParser_VariableExtraction tests Go variable extraction behavior.
// This is a RED PHASE test that defines expected variable parsing behavior.
func TestGoParser_VariableExtraction(t *testing.T) {
	sourceCode := `package main

var globalVar = "hello"
const MaxRetries = 3

func main() {
	var localVar int = 42
	const localConst = "local"
}`

	t.Run("should extract global variables and constants", func(t *testing.T) {
		parser := NewGoParser()
		require.NotNil(t, parser)

		parseTree := createMockGoParseTree(t, sourceCode)
		ctx := context.Background()
		options := outbound.SemanticExtractionOptions{}

		variables, err := parser.ExtractVariables(ctx, parseTree, options)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(variables), 2) // At least globalVar and MaxRetries

		// Test global variable
		globalVar := findVariableByName(variables, "globalVar")
		require.NotNil(t, globalVar)
		assert.Equal(t, outbound.ConstructVariable, globalVar.Type)
		assert.Equal(t, "globalVar", globalVar.Name)

		// Test constant
		maxRetriesConst := findVariableByName(variables, "MaxRetries")
		require.NotNil(t, maxRetriesConst)
		assert.Equal(t, outbound.ConstructConstant, maxRetriesConst.Type)
		assert.Equal(t, "MaxRetries", maxRetriesConst.Name)
	})
}

// TestGoParser_ImportExtraction tests Go import extraction behavior.
// This is a RED PHASE test that defines expected import parsing behavior.
func TestGoParser_ImportExtraction(t *testing.T) {
	sourceCode := `package main

import (
	"fmt"
	"context"
	utils "codechunking/internal/utils"
	. "database/sql"
)`

	t.Run("should extract import declarations", func(t *testing.T) {
		parser := NewGoParser()
		require.NotNil(t, parser)

		parseTree := createMockGoParseTree(t, sourceCode)
		ctx := context.Background()
		options := outbound.SemanticExtractionOptions{}

		imports, err := parser.ExtractImports(ctx, parseTree, options)
		require.NoError(t, err)
		require.Len(t, imports, 4)

		// Test standard import
		fmtImport := findImportByPath(imports, "fmt")
		require.NotNil(t, fmtImport)
		assert.Equal(t, "fmt", fmtImport.Path)
		assert.Empty(t, fmtImport.Alias)

		// Test aliased import
		utilsImport := findImportByPath(imports, "codechunking/internal/utils")
		require.NotNil(t, utilsImport)
		assert.Equal(t, "utils", utilsImport.Alias)

		// Test wildcard import
		sqlImport := findImportByPath(imports, "database/sql")
		require.NotNil(t, sqlImport)
		assert.True(t, sqlImport.IsWildcard)
	})
}

// =============================================================================
// LANGUAGE DISPATCHER/FACTORY TESTS
// =============================================================================

// TestLanguageParserFactory_Creation tests the factory pattern for creating parsers.
// This is a RED PHASE test that defines how language parsers should be created.
func TestLanguageParserFactory_Creation(t *testing.T) {
	t.Run("should create parser factory", func(t *testing.T) {
		// This will fail because NewLanguageParserFactory doesn't exist yet
		factory := NewLanguageParserFactory()
		require.NotNil(t, factory)

		supportedLangs := factory.GetSupportedLanguages()
		goLang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
		assert.Contains(t, supportedLangs, goLang)
	})

	t.Run("should create Go parser from factory", func(t *testing.T) {
		factory := NewLanguageParserFactory()
		require.NotNil(t, factory)

		goLang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
		parser, err := factory.CreateParser(goLang)
		require.NoError(t, err)
		require.NotNil(t, parser)

		assert.Equal(t, goLang, parser.GetSupportedLanguage())
		assert.True(t, parser.IsSupported(goLang))
	})

	t.Run("should return error for unsupported language", func(t *testing.T) {
		factory := NewLanguageParserFactory()
		require.NotNil(t, factory)

		rustLang, _ := valueobject.NewLanguage(valueobject.LanguageRust) // Assuming Rust is not supported yet
		parser, err := factory.CreateParser(rustLang)
		require.Error(t, err)
		assert.Nil(t, parser)
		assert.Contains(t, err.Error(), "unsupported language")
	})

	t.Run("should validate language support", func(t *testing.T) {
		factory := NewLanguageParserFactory()
		require.NotNil(t, factory)

		goLang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
		rustLang, _ := valueobject.NewLanguage(valueobject.LanguageRust)

		assert.True(t, factory.IsLanguageSupported(goLang))
		assert.False(t, factory.IsLanguageSupported(rustLang))
	})
}

// =============================================================================
// INTEGRATION TESTS
// =============================================================================

// TestLanguageParser_Integration tests parser integration with utility functions.
// This is a RED PHASE test that defines how parsers should integrate with existing utilities.
func TestLanguageParser_Integration(t *testing.T) {
	t.Run("should use utility functions for hash generation", func(t *testing.T) {
		parser := NewGoParser()
		require.NotNil(t, parser)

		sourceCode := `package main
func Test() {}`

		parseTree := createMockGoParseTree(t, sourceCode)
		ctx := context.Background()
		options := outbound.SemanticExtractionOptions{}

		functions, err := parser.ExtractFunctions(ctx, parseTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 1)

		function := functions[0]

		// Test that hash is generated using utils.GenerateHash
		expectedHash := utils.GenerateHash(function.Content)
		assert.Equal(t, expectedHash, function.Hash)

		// Test that ID is generated using utils.GenerateID
		assert.Contains(t, function.ID, "function_test")
		assert.NotEmpty(t, function.ExtractedAt)
	})

	t.Run("should handle parse errors gracefully", func(t *testing.T) {
		parser := NewGoParser()
		require.NotNil(t, parser)

		// Test with invalid Go code
		invalidCode := `package main
func {
	// Invalid syntax
}`

		parseTree := createMockGoParseTree(t, invalidCode)
		ctx := context.Background()
		options := outbound.SemanticExtractionOptions{}

		functions, err := parser.ExtractFunctions(ctx, parseTree, options)
		// Should either return error or empty results, not panic
		if err != nil {
			assert.Contains(t, err.Error(), "parse error")
		} else {
			assert.NotNil(t, functions) // May be empty, but should not be nil
		}
	})
}

// =============================================================================
// HELPER FUNCTIONS (These will need implementations)
// =============================================================================

// NewGoParser creates a new Go language parser.
func NewGoParser() LanguageParser {
	// This is a test helper - in real implementation, this would use the factory
	// For now, return nil to satisfy interface testing
	return nil
}

// NewLanguageParserFactory creates a new language parser factory.
func NewLanguageParserFactory() LanguageParserFactory {
	factory, err := NewLanguageDispatcher()
	if err != nil {
		panic(fmt.Sprintf("Failed to create language dispatcher: %v", err))
	}
	return factory
}

// createMockGoParseTree creates a mock parse tree for testing.
// This function doesn't exist yet - it will need a proper implementation.
func createMockGoParseTree(t *testing.T, sourceCode string) *valueobject.ParseTree {
	t.Helper()

	// Create Go language
	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	// Use the real parser function from the main test file
	return createRealParseTreeFromSource(t, language, sourceCode)
}

// Helper functions for finding specific items in results.
func findFunctionByName(functions []outbound.SemanticCodeChunk, name string) *outbound.SemanticCodeChunk {
	for i := range functions {
		if functions[i].Name == name {
			return &functions[i]
		}
	}
	return nil
}

func findStructByName(structs []outbound.SemanticCodeChunk, name string) *outbound.SemanticCodeChunk {
	for i := range structs {
		if structs[i].Name == name && structs[i].Type == outbound.ConstructStruct {
			return &structs[i]
		}
	}
	return nil
}

func findInterfaceByName(interfaces []outbound.SemanticCodeChunk, name string) *outbound.SemanticCodeChunk {
	for i := range interfaces {
		if interfaces[i].Name == name && interfaces[i].Type == outbound.ConstructInterface {
			return &interfaces[i]
		}
	}
	return nil
}

func findVariableByName(variables []outbound.SemanticCodeChunk, name string) *outbound.SemanticCodeChunk {
	for i := range variables {
		if variables[i].Name == name {
			return &variables[i]
		}
	}
	return nil
}

func findImportByPath(imports []outbound.ImportDeclaration, path string) *outbound.ImportDeclaration {
	for i := range imports {
		if imports[i].Path == path {
			return &imports[i]
		}
	}
	return nil
}

func filterPublicFunctions(functions []outbound.SemanticCodeChunk) []outbound.SemanticCodeChunk {
	var publicFuncs []outbound.SemanticCodeChunk
	for _, fn := range functions {
		if fn.Visibility == outbound.Public {
			publicFuncs = append(publicFuncs, fn)
		}
	}
	return publicFuncs
}
