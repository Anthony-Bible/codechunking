package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSemanticTraverserAdapter_ExtractGoFunctions_RealParsing_Basic
// Parses real Go source via the observable parser and validates function extraction.
func TestSemanticTraverserAdapter_ExtractGoFunctions_RealParsing_Basic(t *testing.T) {
	ctx := context.Background()

	sourceCode := `package main

import "fmt"

func Add(a int, b int) int {
    return a + b
}

func main() {
    fmt.Println("Hello, World!")
}`

	// Initialize language and factory
	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	factory, err := NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)

	// Create a real tree-sitter parser and parse the source to a port parse tree
	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err)

	portResult, err := parser.Parse(ctx, []byte(sourceCode))
	require.NoError(t, err)
	require.NotNil(t, portResult)
	require.NotNil(t, portResult.ParseTree)

	// Convert port parse tree to domain parse tree expected by the adapter/language parsers
	domainTree, err := ConvertPortParseTreeToDomain(portResult.ParseTree)
	require.NoError(t, err)
	require.NotNil(t, domainTree)

	// Create the semantic traverser adapter (will use LanguageDispatcher → GoParser)
	adapter := NewSemanticTraverserAdapterWithFactory(factory)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeComments:      true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
		PreservationStrategy: outbound.PreserveModerate,
	}

	// Extract functions using the real parse tree
	functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
	require.NoError(t, err)
	require.NotEmpty(t, functions)

	// Validate presence of Add and main functions
	var addFunc *outbound.SemanticCodeChunk
	var mainFunc *outbound.SemanticCodeChunk
	for i := range functions {
		fn := functions[i]
		if fn.Name == "Add" {
			addFunc = &fn
		}
		if fn.Name == "main" {
			mainFunc = &fn
		}
	}

	require.NotNil(t, addFunc, "Add function should be extracted")
	assert.Equal(t, outbound.ConstructFunction, addFunc.Type)
	assert.Equal(t, "main.Add", addFunc.QualifiedName)
	assert.Equal(t, outbound.Public, addFunc.Visibility)
	assert.Equal(t, "int", addFunc.ReturnType)
	require.Len(t, addFunc.Parameters, 2)
	assert.Equal(t, "a", addFunc.Parameters[0].Name)
	assert.Equal(t, "int", addFunc.Parameters[0].Type)
	assert.Equal(t, "b", addFunc.Parameters[1].Name)
	assert.Equal(t, "int", addFunc.Parameters[1].Type)
	assert.Contains(t, addFunc.Content, "func Add(")

	require.NotNil(t, mainFunc, "main function should be extracted")
	assert.Equal(t, outbound.ConstructFunction, mainFunc.Type)
	assert.Equal(t, "main.main", mainFunc.QualifiedName)
	assert.Equal(t, outbound.Private, mainFunc.Visibility)
	assert.Empty(t, mainFunc.ReturnType)
}
