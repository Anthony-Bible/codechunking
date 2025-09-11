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

func TestGoParserFocusedVariableAndConstantExtraction(t *testing.T) {
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
			name: "Global variable declarations",
			sourceCode: `
var counter int
var name string
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "var:counter",
					Type:          outbound.ConstructVariable,
					Name:          "counter",
					QualifiedName: "counter",
					Visibility:    outbound.Private,
					Content:       "var counter int",
					StartByte:     1,
					EndByte:       15,
					Language:      valueobject.Go,
				},
				{
					ChunkID:       "var:name",
					Type:          outbound.ConstructVariable,
					Name:          "name",
					QualifiedName: "name",
					Visibility:    outbound.Private,
					Content:       "var name string",
					StartByte:     16,
					EndByte:       31,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Multiple variable declarations",
			sourceCode: `
var a, b, c int
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "var:a",
					Type:          outbound.ConstructVariable,
					Name:          "a",
					QualifiedName: "a",
					Visibility:    outbound.Private,
					Content:       "var a, b, c int",
					StartByte:     1,
					EndByte:       15,
					Language:      valueobject.Go,
				},
				{
					ChunkID:       "var:b",
					Type:          outbound.ConstructVariable,
					Name:          "b",
					QualifiedName: "b",
					Visibility:    outbound.Private,
					Content:       "var a, b, c int",
					StartByte:     1,
					EndByte:       15,
					Language:      valueobject.Go,
				},
				{
					ChunkID:       "var:c",
					Type:          outbound.ConstructVariable,
					Name:          "c",
					QualifiedName: "c",
					Visibility:    outbound.Private,
					Content:       "var a, b, c int",
					StartByte:     1,
					EndByte:       15,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Variable declarations with initialization",
			sourceCode: `
var initializedCounter int = 10
var initializedName string = "test"
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "var:initializedCounter",
					Type:          outbound.ConstructVariable,
					Name:          "initializedCounter",
					QualifiedName: "initializedCounter",
					Visibility:    outbound.Private,
					Content:       "var initializedCounter int = 10",
					StartByte:     1,
					EndByte:       32,
					Language:      valueobject.Go,
				},
				{
					ChunkID:       "var:initializedName",
					Type:          outbound.ConstructVariable,
					Name:          "initializedName",
					QualifiedName: "initializedName",
					Visibility:    outbound.Private,
					Content:       "var initializedName string = \"test\"",
					StartByte:     33,
					EndByte:       67,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Constant declarations",
			sourceCode: `
const maxRetries = 3
const defaultTimeout = time.Second * 30
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "const:maxRetries",
					Type:          outbound.ConstructConstant,
					Name:          "maxRetries",
					QualifiedName: "maxRetries",
					Visibility:    outbound.Private,
					Content:       "const maxRetries = 3",
					StartByte:     1,
					EndByte:       21,
					Language:      valueobject.Go,
				},
				{
					ChunkID:       "const:defaultTimeout",
					Type:          outbound.ConstructConstant,
					Name:          "defaultTimeout",
					QualifiedName: "defaultTimeout",
					Visibility:    outbound.Private,
					Content:       "const defaultTimeout = time.Second * 30",
					StartByte:     22,
					EndByte:       61,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Typed constants",
			sourceCode: `
const (
	MaxInt32 int32 = 1<<31 - 1
	MinInt32 int32 = -1 << 31
)
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "const:MaxInt32",
					Type:          outbound.ConstructConstant,
					Name:          "MaxInt32",
					QualifiedName: "MaxInt32",
					Visibility:    outbound.Public,
					Content:       "MaxInt32 int32 = 1<<31 - 1",
					StartByte:     8,
					EndByte:       33,
					Language:      valueobject.Go,
				},
				{
					ChunkID:       "const:MinInt32",
					Type:          outbound.ConstructConstant,
					Name:          "MinInt32",
					QualifiedName: "MinInt32",
					Visibility:    outbound.Public,
					Content:       "MinInt32 int32 = -1 << 31",
					StartByte:     37,
					EndByte:       62,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Variable groups",
			sourceCode: `
var (
	x int
	y string
	z bool
)
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "var:x",
					Type:          outbound.ConstructVariable,
					Name:          "x",
					QualifiedName: "x",
					Visibility:    outbound.Private,
					Content:       "x int",
					StartByte:     7,
					EndByte:       12,
					Language:      valueobject.Go,
				},
				{
					ChunkID:       "var:y",
					Type:          outbound.ConstructVariable,
					Name:          "y",
					QualifiedName: "y",
					Visibility:    outbound.Private,
					Content:       "y string",
					StartByte:     16,
					EndByte:       24,
					Language:      valueobject.Go,
				},
				{
					ChunkID:       "var:z",
					Type:          outbound.ConstructVariable,
					Name:          "z",
					QualifiedName: "z",
					Visibility:    outbound.Private,
					Content:       "z bool",
					StartByte:     28,
					EndByte:       34,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Public variables and constants",
			sourceCode: `
var PublicVar string
const PublicConst = 42
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "var:PublicVar",
					Type:          outbound.ConstructVariable,
					Name:          "PublicVar",
					QualifiedName: "PublicVar",
					Visibility:    outbound.Public,
					Content:       "var PublicVar string",
					StartByte:     1,
					EndByte:       20,
					Language:      valueobject.Go,
				},
				{
					ChunkID:       "const:PublicConst",
					Type:          outbound.ConstructConstant,
					Name:          "PublicConst",
					QualifiedName: "PublicConst",
					Visibility:    outbound.Public,
					Content:       "const PublicConst = 42",
					StartByte:     21,
					EndByte:       44,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Channel type parsing",
			sourceCode: `
var inputChan <-chan int
var outputChan chan<- string
var bidirectionalChan chan bool
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "var:inputChan",
					Type:          outbound.ConstructVariable,
					Name:          "inputChan",
					QualifiedName: "inputChan",
					Visibility:    outbound.Private,
					Content:       "var inputChan <-chan int",
					StartByte:     1,
					EndByte:       25,
					Language:      valueobject.Go,
				},
				{
					ChunkID:       "var:outputChan",
					Type:          outbound.ConstructVariable,
					Name:          "outputChan",
					QualifiedName: "outputChan",
					Visibility:    outbound.Private,
					Content:       "var outputChan chan<- string",
					StartByte:     26,
					EndByte:       55,
					Language:      valueobject.Go,
				},
				{
					ChunkID:       "var:bidirectionalChan",
					Type:          outbound.ConstructVariable,
					Name:          "bidirectionalChan",
					QualifiedName: "bidirectionalChan",
					Visibility:    outbound.Private,
					Content:       "var bidirectionalChan chan bool",
					StartByte:     56,
					EndByte:       87,
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
				IncludeConstants: true,
			}

			chunks := adapter.ExtractCodeChunks(domainTree, options)

			// Filter to only variable/constant chunks for this test
			var varConstChunks []outbound.SemanticCodeChunk
			for _, chunk := range chunks {
				if chunk.Type == outbound.ConstructVariable || chunk.Type == outbound.ConstructConstant {
					varConstChunks = append(varConstChunks, chunk)
				}
			}

			assert.Len(t, varConstChunks, len(tt.expectedChunks))

			for i, expected := range tt.expectedChunks {
				if i < len(varConstChunks) {
					actual := varConstChunks[i]
					assert.Equal(t, expected.ChunkID, actual.ChunkID)
					assert.Equal(t, expected.Type, actual.Type)
					assert.Equal(t, expected.Name, actual.Name)
					assert.Equal(t, expected.QualifiedName, actual.QualifiedName)
					assert.Equal(t, expected.Visibility, actual.Visibility)
					assert.Equal(t, expected.Content, actual.Content)
					assert.Equal(t, expected.StartByte, actual.StartByte)
					assert.Equal(t, expected.EndByte, actual.EndByte)
					assert.Equal(t, expected.Language, actual.Language)
				}
			}
		})
	}
}
