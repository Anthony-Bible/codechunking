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

func TestGoParserFocusedFunctionExtraction(t *testing.T) {
	ctx := context.Background()
	parser, err := NewGoParser()
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name           string
		sourceCode     string
		expectedChunks []outbound.SemanticCodeChunk
	}{
		{
			name: "Regular function with basic parameters",
			sourceCode: `
// Add adds two integers and returns the result
func Add(a int, b int) int {
	return a + b
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "func:Add",
					Type:          outbound.ConstructFunction,
					Name:          "Add",
					QualifiedName: "Add",
					Visibility:    outbound.Public,
					Documentation: "Add adds two integers and returns the result",
					Content:       "func Add(a int, b int) int {\n\treturn a + b\n}",
					StartByte:     1,
					EndByte:       65,
					Parameters:    []outbound.Parameter{{Name: "a", Type: "int"}, {Name: "b", Type: "int"}},
					ReturnType:    "int",
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Method with pointer receiver",
			sourceCode: `
// String returns the string representation of the point
func (p *Point) String() string {
	return fmt.Sprintf("(%d, %d)", p.X, p.Y)
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "method:Point:String",
					Type:          outbound.ConstructMethod,
					Name:          "String",
					QualifiedName: "Point.String",
					Visibility:    outbound.Public,
					Documentation: "String returns the string representation of the point",
					Content:       "func (p *Point) String() string {\n\treturn fmt.Sprintf(\"(%d, %d)\", p.X, p.Y)\n}",
					StartByte:     1,
					EndByte:       105,
					Parameters:    []outbound.Parameter{},
					ReturnType:    "string",
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Method with value receiver",
			sourceCode: `
// Distance calculates the distance from origin
func (p Point) Distance() float64 {
	return math.Sqrt(float64(p.X*p.X + p.Y*p.Y))
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "method:Point:Distance",
					Type:          outbound.ConstructMethod,
					Name:          "Distance",
					QualifiedName: "Point.Distance",
					Visibility:    outbound.Public,
					Documentation: "Distance calculates the distance from origin",
					Content:       "func (p Point) Distance() float64 {\n\treturn math.Sqrt(float64(p.X*p.X + p.Y*p.Y))\n}",
					StartByte:     1,
					EndByte:       111,
					Parameters:    []outbound.Parameter{},
					ReturnType:    "float64",
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Variadic function",
			sourceCode: `
// Sum calculates the sum of all provided numbers
func Sum(nums ...int) int {
	total := 0
	for _, num := range nums {
		total += num
	}
	return total
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "func:Sum",
					Type:          outbound.ConstructFunction,
					Name:          "Sum",
					QualifiedName: "Sum",
					Visibility:    outbound.Public,
					Documentation: "Sum calculates the sum of all provided numbers",
					Content:       "func Sum(nums ...int) int {\n\ttotal := 0\n\tfor _, num := range nums {\n\t\ttotal += num\n\t}\n\treturn total\n}",
					StartByte:     1,
					EndByte:       127,
					Parameters:    []outbound.Parameter{{Name: "nums", Type: "...int"}},
					ReturnType:    "int",
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Generic function with type parameters",
			sourceCode: `
// Map applies a function to each element of a slice
func Map[T any, R any](slice []T, fn func(T) R) []R {
	result := make([]R, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "func:Map",
					Type:          outbound.ConstructFunction,
					Name:          "Map",
					QualifiedName: "Map",
					Visibility:    outbound.Public,
					Documentation: "Map applies a function to each element of a slice",
					Content:       "func Map[T any, R any](slice []T, fn func(T) R) []R {\n\tresult := make([]R, len(slice))\n\tfor i, v := range slice {\n\t\tresult[i] = fn(v)\n\t}\n\treturn result\n}",
					StartByte:     1,
					EndByte:       173,
					Parameters: []outbound.Parameter{
						{Name: "slice", Type: "[]T"},
						{Name: "fn", Type: "func(T) R"},
					},
					ReturnType: "[]R",
					GenericParameters: []outbound.GenericParameter{
						{Name: "T", Constraints: []string{"any"}},
						{Name: "R", Constraints: []string{"any"}},
					},
					Language: valueobject.Go,
				},
			},
		},
		{
			name: "Function with multiple return values",
			sourceCode: `
// Divide divides two numbers and returns quotient and remainder
func Divide(a, b int) (int, int, error) {
	if b == 0 {
		return 0, 0, errors.New("division by zero")
	}
	return a / b, a % b, nil
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "func:Divide",
					Type:          outbound.ConstructFunction,
					Name:          "Divide",
					QualifiedName: "Divide",
					Visibility:    outbound.Public,
					Documentation: "Divide divides two numbers and returns quotient and remainder",
					Content:       "func Divide(a, b int) (int, int, error) {\n\tif b == 0 {\n\t\treturn 0, 0, errors.New(\"division by zero\")\n\t}\n\treturn a / b, a % b, nil\n}",
					StartByte:     1,
					EndByte:       155,
					Parameters:    []outbound.Parameter{{Name: "a", Type: "int"}, {Name: "b", Type: "int"}},
					ReturnType:    "(int, int, error)",
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Anonymous function",
			sourceCode: `
func main() {
	fn := func(x int) int {
		return x * 2
	}
	result := fn(5)
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "func:main",
					Type:          outbound.ConstructFunction,
					Name:          "main",
					QualifiedName: "main",
					Visibility:    outbound.Private,
					Documentation: "",
					Content:       "func main() {\n\tfn := func(x int) int {\n\t\treturn x * 2\n\t}\n\tresult := fn(5)\n}",
					StartByte:     1,
					EndByte:       89,
					Parameters:    []outbound.Parameter{},
					ReturnType:    "",
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Function with complex signature - channels",
			sourceCode: `
// Process processes data from input channel and sends to output channel
func Process(input <-chan int, output chan<- string) {
	for num := range input {
		output <- fmt.Sprintf("Processed: %d", num)
	}
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "func:Process",
					Type:          outbound.ConstructFunction,
					Name:          "Process",
					QualifiedName: "Process",
					Visibility:    outbound.Public,
					Documentation: "Process processes data from input channel and sends to output channel",
					Content:       "func Process(input <-chan int, output chan<- string) {\n\tfor num := range input {\n\t\toutput <- fmt.Sprintf(\"Processed: %d\", num)\n\t}\n}",
					StartByte:     1,
					EndByte:       153,
					Parameters: []outbound.Parameter{
						{Name: "input", Type: "<-chan int"},
						{Name: "output", Type: "chan<- string"},
					},
					ReturnType: "",
					Language:   valueobject.Go,
				},
			},
		},
		{
			name: "Function with interface parameter",
			sourceCode: `
// Handle processes data using the provided handler
func Handle(handler io.Reader) error {
	data := make([]byte, 1024)
	_, err := handler.Read(data)
	return err
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "func:Handle",
					Type:          outbound.ConstructFunction,
					Name:          "Handle",
					QualifiedName: "Handle",
					Visibility:    outbound.Public,
					Documentation: "Handle processes data using the provided handler",
					Content:       "func Handle(handler io.Reader) error {\n\tdata := make([]byte, 1024)\n\t_, err := handler.Read(data)\n\treturn err\n}",
					StartByte:     1,
					EndByte:       133,
					Parameters:    []outbound.Parameter{{Name: "handler", Type: "io.Reader"}},
					ReturnType:    "error",
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Function with function type parameter",
			sourceCode: `
// Apply applies the given function to the value
func Apply(value int, fn func(int) bool) bool {
	return fn(value)
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "func:Apply",
					Type:          outbound.ConstructFunction,
					Name:          "Apply",
					QualifiedName: "Apply",
					Visibility:    outbound.Public,
					Documentation: "Apply applies the given function to the value",
					Content:       "func Apply(value int, fn func(int) bool) bool {\n\treturn fn(value)\n}",
					StartByte:     1,
					EndByte:       89,
					Parameters: []outbound.Parameter{
						{Name: "value", Type: "int"},
						{Name: "fn", Type: "func(int) bool"},
					},
					ReturnType: "bool",
					Language:   valueobject.Go,
				},
			},
		},
		{
			name: "Private function",
			sourceCode: `
// helper is a private helper function
func helper(a int) int {
	return a + 1
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "func:helper",
					Type:          outbound.ConstructFunction,
					Name:          "helper",
					QualifiedName: "helper",
					Visibility:    outbound.Private,
					Documentation: "helper is a private helper function",
					Content:       "func helper(a int) int {\n\treturn a + 1\n}",
					StartByte:     1,
					EndByte:       61,
					Parameters:    []outbound.Parameter{{Name: "a", Type: "int"}},
					ReturnType:    "int",
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
				IncludeFunctions: true,
			}

			chunks := adapter.ExtractCodeChunks(domainTree, options)

			// Filter to only function/method chunks for this test
			var funcChunks []outbound.SemanticCodeChunk
			for _, chunk := range chunks {
				if chunk.Type == outbound.ConstructFunction || chunk.Type == outbound.ConstructMethod {
					funcChunks = append(funcChunks, chunk)
				}
			}

			assert.Len(t, funcChunks, len(tt.expectedChunks))

			for i, expected := range tt.expectedChunks {
				if i < len(funcChunks) {
					actual := funcChunks[i]
					assert.Equal(t, expected.ChunkID, actual.ChunkID)
					assert.Equal(t, expected.Type, actual.Type)
					assert.Equal(t, expected.Name, actual.Name)
					assert.Equal(t, expected.QualifiedName, actual.QualifiedName)
					assert.Equal(t, expected.Visibility, actual.Visibility)
					assert.Equal(t, expected.Documentation, actual.Documentation)
					assert.Equal(t, expected.Content, actual.Content)
					assert.Equal(t, expected.StartByte, actual.StartByte)
					assert.Equal(t, expected.EndByte, actual.EndByte)
					assert.Equal(t, expected.Parameters, actual.Parameters)
					assert.Equal(t, expected.ReturnType, actual.ReturnType)
					assert.Equal(t, expected.Language, actual.Language)

					if len(expected.GenericParameters) > 0 {
						assert.Equal(t, expected.GenericParameters, actual.GenericParameters)
					}
				}
			}
		})
	}
}
