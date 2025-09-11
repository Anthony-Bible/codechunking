package goparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/adapter/outbound/treesitter/parsers/testhelpers"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoParserFocusedAdvancedFeatures(t *testing.T) {
	ctx := context.Background()
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)
	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name           string
		sourceCode     string
		expectedChunks []outbound.SemanticCodeChunk
	}{
		{
			name: "Type alias declarations",
			sourceCode: `
// MyInt is an alias for int
type MyInt = int

// Reader is an alias for io.Reader
type Reader = io.Reader
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "type:MyInt",
					Type:          outbound.ConstructType,
					Name:          "MyInt",
					QualifiedName: "MyInt",
					Visibility:    outbound.Public,
					Documentation: "MyInt is an alias for int",
					Content:       "type MyInt = int",
					StartByte:     1,
					EndByte:       17,
					Language:      valueobject.Go,
				},
				{
					ChunkID:       "type:Reader",
					Type:          outbound.ConstructType,
					Name:          "Reader",
					QualifiedName: "Reader",
					Visibility:    outbound.Public,
					Documentation: "Reader is an alias for io.Reader",
					Content:       "type Reader = io.Reader",
					StartByte:     19,
					EndByte:       42,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Type definitions",
			sourceCode: `
// MyString is a custom string type
type MyString string

// Point represents a 2D point
type Point struct {
	X, Y int
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "type:MyString",
					Type:          outbound.ConstructType,
					Name:          "MyString",
					QualifiedName: "MyString",
					Visibility:    outbound.Public,
					Documentation: "MyString is a custom string type",
					Content:       "type MyString string",
					StartByte:     1,
					EndByte:       21,
					Language:      valueobject.Go,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree, err := parser.Parse(ctx, []byte(tt.sourceCode))
			require.NoError(t, err)
			require.NotNil(t, parseTree)

			domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
			require.NoError(t, err)
			require.NotNil(t, domainTree)

			adapter := treesitter.NewSemanticTraverserAdapter()
			options := &treesitter.SemanticExtractionOptions{
				IncludeVariables: true,
			}

			chunks := adapter.ExtractCodeChunks(domainTree, options)

			assert.Len(t, chunks, len(tt.expectedChunks))

			for i, expected := range tt.expectedChunks {
				if i < len(chunks) {
					actual := chunks[i]
					assert.Equal(t, expected.ChunkID, actual.ChunkID)
					assert.Equal(t, expected.Type, actual.Type)
					assert.Equal(t, expected.Name, actual.Name)
					assert.Equal(t, expected.QualifiedName, actual.QualifiedName)
					assert.Equal(t, expected.Visibility, actual.Visibility)
					assert.Equal(t, expected.Documentation, actual.Documentation)
					assert.Equal(t, expected.Content, actual.Content)
					assert.Equal(t, expected.StartByte, actual.StartByte)
					assert.Equal(t, expected.EndByte, actual.EndByte)
					assert.Equal(t, expected.Language, actual.Language)
				}
			}
		})
	}
}

func TestGoParserFocusedIntegration(t *testing.T) {
	ctx := context.Background()
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)
	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	sourceCode := `
// Package example demonstrates various Go constructs
package example

import (
	"fmt"
	"errors"
)

// Person represents a person
type Person struct {
	Name string ` + "`json:\"name\"`" + `
	Age  int    ` + "`json:\"age\"`" + `
}

// String returns the string representation
func (p *Person) String() string {
	return fmt.Sprintf("%s (%d)", p.Name, p.Age)
}

// Add adds two integers
func Add(a, b int) int {
	return a + b
}

var counter int = 0
const MaxValue = 100
`

	parseTree, err := parser.Parse(ctx, []byte(sourceCode))
	require.NoError(t, err)
	require.NotNil(t, parseTree)

	domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
	require.NoError(t, err)
	require.NotNil(t, domainTree)

	adapter := treesitter.NewSemanticTraverserAdapter()
	options := &treesitter.SemanticExtractionOptions{
		IncludeFunctions:  true,
		IncludeStructs:    true,
		IncludeInterfaces: false,
		IncludeVariables:  true,
		IncludeConstants:  true,
		IncludePackages:   true,
	}

	chunks := adapter.ExtractCodeChunks(domainTree, options)

	// Verify we have the expected number of chunks
	assert.Len(t, chunks, 6)

	// Verify package chunk
	packageChunk := testhelpers.FindChunkByType(chunks, outbound.ConstructPackage)
	assert.NotNil(t, packageChunk)
	assert.Equal(t, outbound.ConstructPackage, packageChunk.Type)
	assert.Equal(t, "example", packageChunk.Name)
	assert.Equal(t, "Package example demonstrates various Go constructs", packageChunk.Documentation)

	// Verify struct chunk
	structChunk := testhelpers.FindChunkByType(chunks, outbound.ConstructStruct)
	assert.NotNil(t, structChunk)
	assert.Equal(t, "Person", structChunk.Name)

	// Verify method chunk
	methodChunk := testhelpers.FindChunkByType(chunks, outbound.ConstructMethod)
	assert.NotNil(t, methodChunk)
	assert.Equal(t, "String", methodChunk.Name)
	assert.Equal(t, "Person.String", methodChunk.QualifiedName)

	// Verify function chunk
	functionChunk := testhelpers.FindChunkByType(chunks, outbound.ConstructFunction)
	assert.NotNil(t, functionChunk)
	assert.Equal(t, "Add", functionChunk.Name)
	assert.Len(t, functionChunk.Parameters, 2)

	// Verify variable chunk
	variableChunk := testhelpers.FindChunkByType(chunks, outbound.ConstructVariable)
	assert.NotNil(t, variableChunk)
	assert.Equal(t, "counter", variableChunk.Name)

	// Verify constant chunk
	constantChunk := testhelpers.FindChunkByType(chunks, outbound.ConstructConstant)
	assert.NotNil(t, constantChunk)
	assert.Equal(t, "MaxValue", constantChunk.Name)
}

func TestGoParserFocusedErrorHandling(t *testing.T) {
	ctx := context.Background()
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err)
	require.NotNil(t, factory)

	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)
	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name        string
		sourceCode  string
		expectError bool
	}{
		{
			name:        "Invalid Go syntax",
			sourceCode:  `func invalid( { return }`,
			expectError: true,
		},
		{
			name:        "Unsupported language",
			sourceCode:  `invalid language test`,
			expectError: true,
		},
		{
			name:        "Malformed function signature",
			sourceCode:  `func test(a int, b) int { return a + b }`,
			expectError: true,
		},
		{
			name:        "Valid Go code should not error",
			sourceCode:  `func valid() int { return 42 }`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseTree, err := parser.Parse(ctx, []byte(tt.sourceCode))

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, parseTree)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, parseTree)

				domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
				require.NoError(t, err)
				require.NotNil(t, domainTree)

				adapter := treesitter.NewSemanticTraverserAdapter()
				options := &treesitter.SemanticExtractionOptions{}
				chunks := adapter.ExtractCodeChunks(domainTree, options)
				assert.NotNil(t, chunks)
			}
		})
	}
}
