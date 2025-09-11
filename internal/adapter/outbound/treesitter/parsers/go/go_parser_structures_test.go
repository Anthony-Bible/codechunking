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

func TestGoParserFocusedStructExtraction(t *testing.T) {
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
			name: "Simple struct with basic fields",
			sourceCode: `
// Person represents a person with name and age
type Person struct {
	Name string
	Age  int
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "struct:Person",
					Type:          outbound.ConstructStruct,
					Name:          "Person",
					QualifiedName: "Person",
					Visibility:    outbound.Public,
					Documentation: "Person represents a person with name and age",
					Content:       "type Person struct {\n\tName string\n\tAge  int\n}",
					StartByte:     1,
					EndByte:       67,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Struct with struct tags",
			sourceCode: `
// User represents a user in the system
type User struct {
	ID       int    ` + "`json:\"id\" xml:\"id\"`" + `
	Username string ` + "`json:\"username\" xml:\"username\"`" + `
	Email    string ` + "`json:\"email,omitempty\" xml:\"email\"`" + `
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "struct:User",
					Type:          outbound.ConstructStruct,
					Name:          "User",
					QualifiedName: "User",
					Visibility:    outbound.Public,
					Documentation: "User represents a user in the system",
					Content:       "type User struct {\n\tID       int    `json:\"id\" xml:\"id\"`\n\tUsername string `json:\"username\" xml:\"username\"`\n\tEmail    string `json:\"email,omitempty\" xml:\"email\"`\n}",
					StartByte:     1,
					EndByte:       155,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Struct with embedded structs and anonymous fields",
			sourceCode: `
// Address represents a physical address
type Address struct {
	Street string
	City   string
}

// Employee represents an employee with embedded address
type Employee struct {
	Person
	Address
	Company string
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "struct:Address",
					Type:          outbound.ConstructStruct,
					Name:          "Address",
					QualifiedName: "Address",
					Visibility:    outbound.Public,
					Documentation: "Address represents a physical address",
					Content:       "type Address struct {\n\tStreet string\n\tCity   string\n}",
					StartByte:     1,
					EndByte:       75,
					Language:      valueobject.Go,
				},
				{
					ChunkID:       "struct:Employee",
					Type:          outbound.ConstructStruct,
					Name:          "Employee",
					QualifiedName: "Employee",
					Visibility:    outbound.Public,
					Documentation: "Employee represents an employee with embedded address",
					Content:       "type Employee struct {\n\tPerson\n\tAddress\n\tCompany string\n}",
					StartByte:     77,
					EndByte:       172,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Generic struct with type parameters",
			sourceCode: `
// Container holds a value of any type
type Container[T any] struct {
	Value T
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "struct:Container",
					Type:          outbound.ConstructStruct,
					Name:          "Container",
					QualifiedName: "Container",
					Visibility:    outbound.Public,
					Documentation: "Container holds a value of any type",
					Content:       "type Container[T any] struct {\n\tValue T\n}",
					StartByte:     1,
					EndByte:       69,
					Language:      valueobject.Go,
					GenericParameters: []outbound.GenericParameter{
						{Name: "T", Constraints: []string{"any"}},
					},
				},
			},
		},
		{
			name: "Nested structs",
			sourceCode: `
// Company represents a company with nested address
type Company struct {
	Name    string
	Address struct {
		Street string
		City   string
	}
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "struct:Company",
					Type:          outbound.ConstructStruct,
					Name:          "Company",
					QualifiedName: "Company",
					Visibility:    outbound.Public,
					Documentation: "Company represents a company with nested address",
					Content:       "type Company struct {\n\tName    string\n\tAddress struct {\n\t\tStreet string\n\t\tCity   string\n\t}\n}",
					StartByte:     1,
					EndByte:       113,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Private struct",
			sourceCode: `
// person represents a person (private)
type person struct {
	name string
	age  int
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "struct:person",
					Type:          outbound.ConstructStruct,
					Name:          "person",
					QualifiedName: "person",
					Visibility:    outbound.Private,
					Documentation: "person represents a person (private)",
					Content:       "type person struct {\n\tname string\n\tage  int\n}",
					StartByte:     1,
					EndByte:       69,
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
				IncludeStructs: true,
			}

			chunks := adapter.ExtractCodeChunks(domainTree, options)

			// Filter to only struct chunks for this test
			var structChunks []outbound.SemanticCodeChunk
			for _, chunk := range chunks {
				if chunk.Type == outbound.ConstructStruct {
					structChunks = append(structChunks, chunk)
				}
			}

			assert.Len(t, structChunks, len(tt.expectedChunks))

			for i, expected := range tt.expectedChunks {
				if i < len(structChunks) {
					actual := structChunks[i]
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

					if len(expected.GenericParameters) > 0 {
						assert.Equal(t, expected.GenericParameters, actual.GenericParameters)
					}
				}
			}
		})
	}
}

func TestGoParserFocusedInterfaceExtraction(t *testing.T) {
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
			name: "Simple interface with method signatures",
			sourceCode: `
// Writer interface for writing data
type Writer interface {
	Write(data []byte) (int, error)
	Close() error
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "interface:Writer",
					Type:          outbound.ConstructInterface,
					Name:          "Writer",
					QualifiedName: "Writer",
					Visibility:    outbound.Public,
					Documentation: "Writer interface for writing data",
					Content:       "type Writer interface {\n\tWrite(data []byte) (int, error)\n\tClose() error\n}",
					StartByte:     1,
					EndByte:       95,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Embedded interfaces",
			sourceCode: `
// ReadWriter combines Reader and Writer interfaces
type ReadWriter interface {
	Reader
	Writer
	Flush() error
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "interface:ReadWriter",
					Type:          outbound.ConstructInterface,
					Name:          "ReadWriter",
					QualifiedName: "ReadWriter",
					Visibility:    outbound.Public,
					Documentation: "ReadWriter combines Reader and Writer interfaces",
					Content:       "type ReadWriter interface {\n\tReader\n\tWriter\n\tFlush() error\n}",
					StartByte:     1,
					EndByte:       93,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Generic interface with type constraints",
			sourceCode: `
// Comparable interface for comparable types
type Comparable[T comparable] interface {
	Compare(other T) int
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "interface:Comparable",
					Type:          outbound.ConstructInterface,
					Name:          "Comparable",
					QualifiedName: "Comparable",
					Visibility:    outbound.Public,
					Documentation: "Comparable interface for comparable types",
					Content:       "type Comparable[T comparable] interface {\n\tCompare(other T) int\n}",
					StartByte:     1,
					EndByte:       95,
					Language:      valueobject.Go,
					GenericParameters: []outbound.GenericParameter{
						{Name: "T", Constraints: []string{"comparable"}},
					},
				},
			},
		},
		{
			name: "Empty interface",
			sourceCode: `
// Any represents any type
type Any interface{}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "interface:Any",
					Type:          outbound.ConstructInterface,
					Name:          "Any",
					QualifiedName: "Any",
					Visibility:    outbound.Public,
					Documentation: "Any represents any type",
					Content:       "type Any interface{}",
					StartByte:     1,
					EndByte:       43,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Private interface",
			sourceCode: `
// reader is a private interface for reading
type reader interface {
	Read() []byte
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "interface:reader",
					Type:          outbound.ConstructInterface,
					Name:          "reader",
					QualifiedName: "reader",
					Visibility:    outbound.Private,
					Documentation: "reader is a private interface for reading",
					Content:       "type reader interface {\n\tRead() []byte\n}",
					StartByte:     1,
					EndByte:       67,
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
				IncludeInterfaces: true,
			}

			chunks := adapter.ExtractCodeChunks(domainTree, options)

			// Filter to only interface chunks for this test
			var interfaceChunks []outbound.SemanticCodeChunk
			for _, chunk := range chunks {
				if chunk.Type == outbound.ConstructInterface {
					interfaceChunks = append(interfaceChunks, chunk)
				}
			}

			assert.Len(t, interfaceChunks, len(tt.expectedChunks))

			for i, expected := range tt.expectedChunks {
				if i < len(interfaceChunks) {
					actual := interfaceChunks[i]
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

					if len(expected.GenericParameters) > 0 {
						assert.Equal(t, expected.GenericParameters, actual.GenericParameters)
					}
				}
			}
		})
	}
}
