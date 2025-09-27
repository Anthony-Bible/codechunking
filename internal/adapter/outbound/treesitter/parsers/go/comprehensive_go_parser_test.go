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

// calculateExpectedPositions parses source code with tree-sitter and returns
// the actual byte positions for the specified function name.
func calculateExpectedPositions(t *testing.T, sourceCode, functionName string) (uint32, uint32) {
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

	// Find function nodes in the parse tree
	functionNodes := domainTree.GetNodesByType("function_declaration")
	methodNodes := domainTree.GetNodesByType("method_declaration")
	allNodes := make([]*valueobject.ParseNode, 0, len(functionNodes)+len(methodNodes))
	allNodes = append(allNodes, functionNodes...)
	allNodes = append(allNodes, methodNodes...)

	for _, node := range allNodes {
		// Look for identifier child nodes to find the function name
		for _, child := range node.Children {
			if child.Type == "identifier" || child.Type == "field_identifier" {
				nodeName := domainTree.GetNodeText(child)
				if nodeName == functionName {
					return node.StartByte, node.EndByte
				}
			}
		}
	}

	t.Fatalf("Function '%s' not found in source code", functionName)
	return 0, 0
}

// calculateExpectedStructPositionsForComprehensiveTest parses source code with tree-sitter and returns
// the actual byte positions for the specified struct name for the comprehensive test.
// This function provides dynamic position calculation to ensure tests remain accurate
// when the parser implementation changes.
func calculateExpectedStructPositionsForComprehensiveTest(
	t *testing.T,
	sourceCode, structName string,
) (uint32, uint32) {
	t.Helper() // Mark this as a test helper function

	ctx := context.Background()

	// Initialize tree-sitter parser factory
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err, "failed to create TreeSitterParserFactory")

	// Create Go language parser
	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err, "failed to create Go language value object")

	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err, "failed to create Go parser")

	// Parse the source code
	parseTree, err := parser.Parse(ctx, []byte(sourceCode))
	require.NoError(t, err, "failed to parse Go source code")

	// Convert to domain parse tree
	domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
	require.NoError(t, err, "failed to convert parse tree to domain")

	// Find the struct by name using a more structured approach
	startByte, endByte, found := findStructPositionByName(domainTree, structName)
	if !found {
		t.Fatalf("Struct '%s' not found in source code", structName)
	}

	// Log debug information about positions for test debugging
	t.Logf("DEBUG: Found struct '%s' at positions StartByte=%d (0x%x), EndByte=%d (0x%x)",
		structName, startByte, startByte, endByte, endByte)

	return startByte, endByte
}

// calculateExpectedInterfacePositionsForComprehensiveTest parses source code with tree-sitter and returns
// the actual byte positions for the specified interface name for the comprehensive test.
// This function provides dynamic position calculation to ensure tests remain accurate
// when the parser implementation changes.
func calculateExpectedInterfacePositionsForComprehensiveTest(
	t *testing.T,
	sourceCode, interfaceName string,
) (uint32, uint32) {
	t.Helper() // Mark this as a test helper function

	ctx := context.Background()

	// Initialize tree-sitter parser factory
	factory, err := treesitter.NewTreeSitterParserFactory(ctx)
	require.NoError(t, err, "failed to create TreeSitterParserFactory")

	// Create Go language parser
	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err, "failed to create Go language value object")

	parser, err := factory.CreateParser(ctx, goLang)
	require.NoError(t, err, "failed to create Go parser")

	// Parse the source code
	parseTree, err := parser.Parse(ctx, []byte(sourceCode))
	require.NoError(t, err, "failed to parse Go source code")

	// Convert to domain parse tree
	domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseTree.ParseTree)
	require.NoError(t, err, "failed to convert parse tree to domain")

	// Find the interface by name using a more structured approach
	startByte, endByte, found := findInterfacePositionByName(domainTree, interfaceName)
	if !found {
		t.Fatalf("Interface '%s' not found in source code", interfaceName)
	}

	// Log debug information about positions for test debugging
	t.Logf("DEBUG: Found interface '%s' at positions StartByte=%d (0x%x), EndByte=%d (0x%x)",
		interfaceName, startByte, startByte, endByte, endByte)

	return startByte, endByte
}

// findInterfacePositionByName searches for an interface declaration by name and returns its position.
// This helper function encapsulates the tree traversal logic for better maintainability.
func findInterfacePositionByName(
	parseTree *valueobject.ParseTree,
	interfaceName string,
) (uint32, uint32, bool) {
	if parseTree == nil || interfaceName == "" {
		return 0, 0, false
	}

	// Find all type_declaration nodes
	typeDeclarations := parseTree.GetNodesByType("type_declaration")

	for _, typeDecl := range typeDeclarations {
		if typeDecl == nil {
			continue
		}

		// Look for type_spec children
		for _, child := range typeDecl.Children {
			if child == nil || child.Type != "type_spec" {
				continue
			}

			// Check if this type_spec contains an interface_type
			hasInterface := false
			for _, grandchild := range child.Children {
				if grandchild != nil && grandchild.Type == "interface_type" {
					hasInterface = true
					break
				}
			}

			if !hasInterface {
				continue
			}

			// Find the type_identifier within the type_spec
			for _, grandchild := range child.Children {
				if grandchild == nil || grandchild.Type != "type_identifier" {
					continue
				}

				// Check if this is the interface we're looking for
				nodeName := parseTree.GetNodeText(grandchild)
				if nodeName == interfaceName {
					return typeDecl.StartByte, typeDecl.EndByte, true
				}
			}
		}
	}

	return 0, 0, false
}

// findStructPositionByName searches for a struct declaration by name and returns its position.
// This helper function encapsulates the tree traversal logic for better maintainability.
func findStructPositionByName(
	parseTree *valueobject.ParseTree,
	structName string,
) (uint32, uint32, bool) {
	if parseTree == nil || structName == "" {
		return 0, 0, false
	}

	// Find all type_declaration nodes
	typeDeclarations := parseTree.GetNodesByType("type_declaration")

	for _, typeDecl := range typeDeclarations {
		if typeDecl == nil {
			continue
		}

		// Look for type_spec children
		for _, child := range typeDecl.Children {
			if child == nil || child.Type != "type_spec" {
				continue
			}

			// Find the type_identifier within the type_spec
			for _, grandchild := range child.Children {
				if grandchild == nil || grandchild.Type != "type_identifier" {
					continue
				}

				// Check if this is the struct we're looking for
				nodeName := parseTree.GetNodeText(grandchild)
				if nodeName == structName {
					return typeDecl.StartByte, typeDecl.EndByte, true
				}
			}
		}
	}

	return 0, 0, false
}

func TestGoParserFunctionExtraction(t *testing.T) {
	ctx := context.Background()
	parser, err := NewGoParser()
	require.NoError(t, err)
	require.NotNil(t, parser)

	tests := []struct {
		name                string
		sourceCode          string
		expectedChunks      []outbound.SemanticCodeChunk
		expectedChunksFunc  func(t *testing.T) []outbound.SemanticCodeChunk
		useDynamicPositions bool
	}{
		{
			name: "Regular function with basic parameters",
			sourceCode: `
// Add adds two integers and returns the result
func Add(a int, b int) int {
	return a + b
}
`,
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// Add adds two integers and returns the result
func Add(a int, b int) int {
	return a + b
}
`
				startByte, endByte := calculateExpectedPositions(t, sourceCode, "Add")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "func:Add",
						Type:          outbound.ConstructFunction,
						Name:          "Add",
						QualifiedName: "Add", // Functions in main/default package use just the function name
						Visibility:    outbound.Public,
						Documentation: "Add adds two integers and returns the result",
						Content:       "func Add(a int, b int) int {\n\treturn a + b\n}",
						StartByte:     startByte, // Calculated dynamically from tree-sitter
						EndByte:       endByte,   // Calculated dynamically from tree-sitter
						Parameters:    []outbound.Parameter{{Name: "a", Type: "int"}, {Name: "b", Type: "int"}},
						ReturnType:    "int",
						Language:      valueobject.Language{}, // Will be set dynamically by implementation
					},
				}
			},
			useDynamicPositions: true,
		},
		{
			name: "Method with pointer receiver",
			sourceCode: `
// String returns the string representation of the point
func (p *Point) String() string {
	return fmt.Sprintf("(%d, %d)", p.X, p.Y)
}
`,
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// String returns the string representation of the point
func (p *Point) String() string {
	return fmt.Sprintf("(%d, %d)", p.X, p.Y)
}
`
				startByte, endByte := calculateExpectedPositions(t, sourceCode, "String")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "method:Point:String",
						Type:          outbound.ConstructMethod,
						Name:          "String",
						QualifiedName: "Point.String",
						Visibility:    outbound.Public,
						Documentation: "String returns the string representation of the point",
						Content:       "func (p *Point) String() string {\n\treturn fmt.Sprintf(\"(%d, %d)\", p.X, p.Y)\n}",
						StartByte:     startByte,
						EndByte:       endByte,
						Parameters:    []outbound.Parameter{},
						ReturnType:    "string",
						Language:      valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
		},
		{
			name: "Method with value receiver",
			sourceCode: `
// Distance calculates the distance from origin
func (p Point) Distance() float64 {
	return math.Sqrt(float64(p.X*p.X + p.Y*p.Y))
}
`,
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// Distance calculates the distance from origin
func (p Point) Distance() float64 {
	return math.Sqrt(float64(p.X*p.X + p.Y*p.Y))
}
`
				startByte, endByte := calculateExpectedPositions(t, sourceCode, "Distance")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "method:Point:Distance",
						Type:          outbound.ConstructMethod,
						Name:          "Distance",
						QualifiedName: "Point.Distance",
						Visibility:    outbound.Public,
						Documentation: "Distance calculates the distance from origin",
						Content:       "func (p Point) Distance() float64 {\n\treturn math.Sqrt(float64(p.X*p.X + p.Y*p.Y))\n}",
						StartByte:     startByte,
						EndByte:       endByte,
						Parameters:    []outbound.Parameter{},
						ReturnType:    "float64",
						Language:      valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
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
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// Sum calculates the sum of all provided numbers
func Sum(nums ...int) int {
	total := 0
	for _, num := range nums {
		total += num
	}
	return total
}
`
				startByte, endByte := calculateExpectedPositions(t, sourceCode, "Sum")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "func:Sum",
						Type:          outbound.ConstructFunction,
						Name:          "Sum",
						QualifiedName: "Sum",
						Visibility:    outbound.Public,
						Documentation: "Sum calculates the sum of all provided numbers",
						Content:       "func Sum(nums ...int) int {\n\ttotal := 0\n\tfor _, num := range nums {\n\t\ttotal += num\n\t}\n\treturn total\n}",
						StartByte:     startByte,
						EndByte:       endByte,
						Parameters:    []outbound.Parameter{{Name: "nums", Type: "...int"}},
						ReturnType:    "int",
						Language:      valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
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
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// Map applies a function to each element of a slice
func Map[T any, R any](slice []T, fn func(T) R) []R {
	result := make([]R, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}
`
				startByte, endByte := calculateExpectedPositions(t, sourceCode, "Map")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "func:Map",
						Type:          outbound.ConstructFunction,
						Name:          "Map",
						QualifiedName: "Map",
						Visibility:    outbound.Public,
						Documentation: "Map applies a function to each element of a slice",
						Content:       "func Map[T any, R any](slice []T, fn func(T) R) []R {\n\tresult := make([]R, len(slice))\n\tfor i, v := range slice {\n\t\tresult[i] = fn(v)\n\t}\n\treturn result\n}",
						StartByte:     startByte,
						EndByte:       endByte,
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
				}
			},
			useDynamicPositions: true,
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
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// Divide divides two numbers and returns quotient and remainder
func Divide(a, b int) (int, int, error) {
	if b == 0 {
		return 0, 0, errors.New("division by zero")
	}
	return a / b, a % b, nil
}
`
				startByte, endByte := calculateExpectedPositions(t, sourceCode, "Divide")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "func:Divide",
						Type:          outbound.ConstructFunction,
						Name:          "Divide",
						QualifiedName: "Divide",
						Visibility:    outbound.Public,
						Documentation: "Divide divides two numbers and returns quotient and remainder",
						Content:       "func Divide(a, b int) (int, int, error) {\n\tif b == 0 {\n\t\treturn 0, 0, errors.New(\"division by zero\")\n\t}\n\treturn a / b, a % b, nil\n}",
						StartByte:     startByte,
						EndByte:       endByte,
						Parameters:    []outbound.Parameter{{Name: "a", Type: "int"}, {Name: "b", Type: "int"}},
						ReturnType:    "(int, int, error)",
						Language:      valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
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
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
func main() {
	fn := func(x int) int {
		return x * 2
	}
	result := fn(5)
}
`
				startByte, endByte := calculateExpectedPositions(t, sourceCode, "main")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "func:main",
						Type:          outbound.ConstructFunction,
						Name:          "main",
						QualifiedName: "main",
						Visibility:    outbound.Private,
						Documentation: "",
						Content:       "func main() {\n\tfn := func(x int) int {\n\t\treturn x * 2\n\t}\n\tresult := fn(5)\n}",
						StartByte:     startByte,
						EndByte:       endByte,
						Parameters:    []outbound.Parameter{},
						ReturnType:    "",
						Language:      valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
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
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// Process processes data from input channel and sends to output channel
func Process(input <-chan int, output chan<- string) {
	for num := range input {
		output <- fmt.Sprintf("Processed: %d", num)
	}
}
`
				startByte, endByte := calculateExpectedPositions(t, sourceCode, "Process")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "func:Process",
						Type:          outbound.ConstructFunction,
						Name:          "Process",
						QualifiedName: "Process",
						Visibility:    outbound.Public,
						Documentation: "Process processes data from input channel and sends to output channel",
						Content:       "func Process(input <-chan int, output chan<- string) {\n\tfor num := range input {\n\t\toutput <- fmt.Sprintf(\"Processed: %d\", num)\n\t}\n}",
						StartByte:     startByte,
						EndByte:       endByte,
						Parameters: []outbound.Parameter{
							{Name: "input", Type: "<-chan int"},
							{Name: "output", Type: "chan<- string"},
						},
						ReturnType: "",
						Language:   valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
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
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// Handle processes data using the provided handler
func Handle(handler io.Reader) error {
	data := make([]byte, 1024)
	_, err := handler.Read(data)
	return err
}
`
				startByte, endByte := calculateExpectedPositions(t, sourceCode, "Handle")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "func:Handle",
						Type:          outbound.ConstructFunction,
						Name:          "Handle",
						QualifiedName: "Handle",
						Visibility:    outbound.Public,
						Documentation: "Handle processes data using the provided handler",
						Content:       "func Handle(handler io.Reader) error {\n\tdata := make([]byte, 1024)\n\t_, err := handler.Read(data)\n\treturn err\n}",
						StartByte:     startByte,
						EndByte:       endByte,
						Parameters:    []outbound.Parameter{{Name: "handler", Type: "io.Reader"}},
						ReturnType:    "error",
						Language:      valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
		},
		{
			name: "Function with function type parameter",
			sourceCode: `
// Apply applies the given function to the value
func Apply(value int, fn func(int) bool) bool {
	return fn(value)
}
`,
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// Apply applies the given function to the value
func Apply(value int, fn func(int) bool) bool {
	return fn(value)
}
`
				startByte, endByte := calculateExpectedPositions(t, sourceCode, "Apply")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "func:Apply",
						Type:          outbound.ConstructFunction,
						Name:          "Apply",
						QualifiedName: "Apply",
						Visibility:    outbound.Public,
						Documentation: "Apply applies the given function to the value",
						Content:       "func Apply(value int, fn func(int) bool) bool {\n\treturn fn(value)\n}",
						StartByte:     startByte,
						EndByte:       endByte,
						Parameters: []outbound.Parameter{
							{Name: "value", Type: "int"},
							{Name: "fn", Type: "func(int) bool"},
						},
						ReturnType: "bool",
						Language:   valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
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
					QualifiedName: "helper", // Functions in main/default package use just the function name
					Visibility:    outbound.Private,
					Documentation: "helper is a private helper function",
					Content:       "func helper(a int) int {\n\treturn a + 1\n}",
					StartByte:     40, // Actual tree-sitter parsed position (0x28)
					EndByte:       80, // Actual tree-sitter parsed position (0x50)
					Parameters:    []outbound.Parameter{{Name: "a", Type: "int"}},
					ReturnType:    "int",
					Language:      valueobject.Language{}, // Will be set dynamically by implementation
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

			var expectedChunks []outbound.SemanticCodeChunk
			if tt.useDynamicPositions {
				expectedChunks = tt.expectedChunksFunc(t)
			} else {
				expectedChunks = tt.expectedChunks
			}
			assert.Len(t, funcChunks, len(expectedChunks))

			for i, expected := range expectedChunks {
				if i >= len(funcChunks) {
					continue
				}
				actual := funcChunks[i]

				// Assert ChunkID - expect consistent func:Name format
				assert.Equal(t, expected.ChunkID, actual.ChunkID)

				// Assert basic properties
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

				// Assert Language with dynamic handling
				if expected.Language.Name() != "" {
					assert.Equal(t, expected.Language, actual.Language)
				} else {
					assert.Equal(t, "Go", actual.Language.Name())
				}

				// Assert GenericParameters if present
				if len(expected.GenericParameters) > 0 {
					assert.Equal(t, expected.GenericParameters, actual.GenericParameters)
				}
			}
		})
	}
}

func TestGoParserStructExtraction(t *testing.T) {
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
		name                string
		sourceCode          string
		expectedChunks      []outbound.SemanticCodeChunk
		expectedChunksFunc  func(t *testing.T) []outbound.SemanticCodeChunk
		useDynamicPositions bool
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
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// Person represents a person with name and age
type Person struct {
	Name string
	Age  int
}
`
				startByte, endByte := calculateExpectedStructPositionsForComprehensiveTest(t, sourceCode, "Person")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "struct:Person",
						Type:          outbound.ConstructStruct,
						Name:          "Person",
						QualifiedName: "Person",
						Visibility:    outbound.Public,
						Documentation: "Person represents a person with name and age",
						Content:       "type Person struct {\n\tName string\n\tAge  int\n}",
						StartByte:     startByte, // Calculated dynamically from tree-sitter
						EndByte:       endByte,   // Calculated dynamically from tree-sitter
						Language:      valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
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
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// User represents a user in the system
type User struct {
	ID       int    ` + "`json:\"id\" xml:\"id\"`" + `
	Username string ` + "`json:\"username\" xml:\"username\"`" + `
	Email    string ` + "`json:\"email,omitempty\" xml:\"email\"`" + `
}
`
				startByte, endByte := calculateExpectedStructPositionsForComprehensiveTest(t, sourceCode, "User")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "struct:User",
						Type:          outbound.ConstructStruct,
						Name:          "User",
						QualifiedName: "User",
						Visibility:    outbound.Public,
						Documentation: "User represents a user in the system",
						Content:       "type User struct {\n\tID       int    `json:\"id\" xml:\"id\"`\n\tUsername string `json:\"username\" xml:\"username\"`\n\tEmail    string `json:\"email,omitempty\" xml:\"email\"`\n}",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
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
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
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
`
				startByte1, endByte1 := calculateExpectedStructPositionsForComprehensiveTest(t, sourceCode, "Address")
				startByte2, endByte2 := calculateExpectedStructPositionsForComprehensiveTest(t, sourceCode, "Employee")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "struct:Address",
						Type:          outbound.ConstructStruct,
						Name:          "Address",
						QualifiedName: "Address",
						Visibility:    outbound.Public,
						Documentation: "Address represents a physical address",
						Content:       "type Address struct {\n\tStreet string\n\tCity   string\n}",
						StartByte:     startByte1,
						EndByte:       endByte1,
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
						StartByte:     startByte2,
						EndByte:       endByte2,
						Language:      valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
		},
		{
			name: "Generic struct with type parameters",
			sourceCode: `
// Container holds a value of any type
type Container[T any] struct {
	Value T
}
`,
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// Container holds a value of any type
type Container[T any] struct {
	Value T
}
`
				startByte, endByte := calculateExpectedStructPositionsForComprehensiveTest(t, sourceCode, "Container")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "struct:Container",
						Type:          outbound.ConstructStruct,
						Name:          "Container",
						QualifiedName: "Container",
						Visibility:    outbound.Public,
						Documentation: "Container holds a value of any type",
						Content:       "type Container[T any] struct {\n\tValue T\n}",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
						GenericParameters: []outbound.GenericParameter{
							{Name: "T", Constraints: []string{"any"}},
						},
					},
				}
			},
			useDynamicPositions: true,
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
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// Company represents a company with nested address
type Company struct {
	Name    string
	Address struct {
		Street string
		City   string
	}
}
`
				startByte, endByte := calculateExpectedStructPositionsForComprehensiveTest(t, sourceCode, "Company")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "struct:Company",
						Type:          outbound.ConstructStruct,
						Name:          "Company",
						QualifiedName: "Company",
						Visibility:    outbound.Public,
						Documentation: "Company represents a company with nested address",
						Content:       "type Company struct {\n\tName    string\n\tAddress struct {\n\t\tStreet string\n\t\tCity   string\n\t}\n}",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
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
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// person represents a person (private)
type person struct {
	name string
	age  int
}
`
				startByte, endByte := calculateExpectedStructPositionsForComprehensiveTest(t, sourceCode, "person")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "struct:person",
						Type:          outbound.ConstructStruct,
						Name:          "person",
						QualifiedName: "person",
						Visibility:    outbound.Private,
						Documentation: "person represents a person (private)",
						Content:       "type person struct {\n\tname string\n\tage  int\n}",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
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
				IncludePrivate: true,
			}

			chunks := adapter.ExtractCodeChunks(domainTree, options)

			// Filter to only struct chunks for this test
			var structChunks []outbound.SemanticCodeChunk
			for _, chunk := range chunks {
				if chunk.Type == outbound.ConstructStruct {
					structChunks = append(structChunks, chunk)
				}
			}

			var expectedChunks []outbound.SemanticCodeChunk
			if tt.useDynamicPositions {
				expectedChunks = tt.expectedChunksFunc(t)
			} else {
				expectedChunks = tt.expectedChunks
			}

			assert.Len(t, structChunks, len(expectedChunks))

			for i, expected := range expectedChunks {
				if i < len(structChunks) {
					actual := structChunks[i]

					// Add debug logging to show position differences
					if tt.useDynamicPositions {
						t.Logf("DEBUG: Struct '%s' - Expected: StartByte=%d (0x%x), EndByte=%d (0x%x)",
							expected.Name, expected.StartByte, expected.StartByte, expected.EndByte, expected.EndByte)
						t.Logf("DEBUG: Struct '%s' - Actual: StartByte=%d (0x%x), EndByte=%d (0x%x)",
							actual.Name, actual.StartByte, actual.StartByte, actual.EndByte, actual.EndByte)
					}

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

func TestGoParserInterfaceExtraction(t *testing.T) {
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
		name                string
		sourceCode          string
		expectedChunks      []outbound.SemanticCodeChunk
		expectedChunksFunc  func(t *testing.T) []outbound.SemanticCodeChunk
		useDynamicPositions bool
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
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// Writer interface for writing data
type Writer interface {
	Write(data []byte) (int, error)
	Close() error
}
`
				startByte, endByte := calculateExpectedInterfacePositionsForComprehensiveTest(t, sourceCode, "Writer")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "interface:Writer",
						Type:          outbound.ConstructInterface,
						Name:          "Writer",
						QualifiedName: "Writer",
						Visibility:    outbound.Public,
						Documentation: "Writer interface for writing data",
						Content:       "type Writer interface {\n\tWrite(data []byte) (int, error)\n\tClose() error\n}",
						StartByte:     startByte, // Calculated dynamically from tree-sitter
						EndByte:       endByte,   // Calculated dynamically from tree-sitter
						Language:      valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
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
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// ReadWriter combines Reader and Writer interfaces
type ReadWriter interface {
	Reader
	Writer
	Flush() error
}
`
				startByte, endByte := calculateExpectedInterfacePositionsForComprehensiveTest(
					t,
					sourceCode,
					"ReadWriter",
				)
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "interface:ReadWriter",
						Type:          outbound.ConstructInterface,
						Name:          "ReadWriter",
						QualifiedName: "ReadWriter",
						Visibility:    outbound.Public,
						Documentation: "ReadWriter combines Reader and Writer interfaces",
						Content:       "type ReadWriter interface {\n\tReader\n\tWriter\n\tFlush() error\n}",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
		},
		{
			name: "Generic interface with type constraints",
			sourceCode: `
// Comparable interface for comparable types
type Comparable[T comparable] interface {
	Compare(other T) int
}
`,
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// Comparable interface for comparable types
type Comparable[T comparable] interface {
	Compare(other T) int
}
`
				startByte, endByte := calculateExpectedInterfacePositionsForComprehensiveTest(
					t,
					sourceCode,
					"Comparable",
				)
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "interface:Comparable",
						Type:          outbound.ConstructInterface,
						Name:          "Comparable",
						QualifiedName: "Comparable",
						Visibility:    outbound.Public,
						Documentation: "Comparable interface for comparable types",
						Content:       "type Comparable[T comparable] interface {\n\tCompare(other T) int\n}",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
						GenericParameters: []outbound.GenericParameter{
							{Name: "T", Constraints: []string{"comparable"}},
						},
					},
				}
			},
			useDynamicPositions: true,
		},
		{
			name: "Empty interface",
			sourceCode: `
// Any represents any type
type Any interface{}
`,
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// Any represents any type
type Any interface{}
`
				startByte, endByte := calculateExpectedInterfacePositionsForComprehensiveTest(t, sourceCode, "Any")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "interface:Any",
						Type:          outbound.ConstructInterface,
						Name:          "Any",
						QualifiedName: "Any",
						Visibility:    outbound.Public,
						Documentation: "Any represents any type",
						Content:       "type Any interface{}",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
		},
		{
			name: "Private interface",
			sourceCode: `
// reader is a private interface for reading
type reader interface {
	Read() []byte
}
`,
			expectedChunksFunc: func(t *testing.T) []outbound.SemanticCodeChunk {
				sourceCode := `
// reader is a private interface for reading
type reader interface {
	Read() []byte
}
`
				startByte, endByte := calculateExpectedInterfacePositionsForComprehensiveTest(t, sourceCode, "reader")
				return []outbound.SemanticCodeChunk{
					{
						ChunkID:       "interface:reader",
						Type:          outbound.ConstructInterface,
						Name:          "reader",
						QualifiedName: "reader",
						Visibility:    outbound.Private,
						Documentation: "reader is a private interface for reading",
						Content:       "type reader interface {\n\tRead() []byte\n}",
						StartByte:     startByte,
						EndByte:       endByte,
						Language:      valueobject.Go,
					},
				}
			},
			useDynamicPositions: true,
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
				IncludePrivate:    true,
			}

			chunks := adapter.ExtractCodeChunks(domainTree, options)

			// Filter to only interface chunks for this test
			var interfaceChunks []outbound.SemanticCodeChunk
			for _, chunk := range chunks {
				if chunk.Type == outbound.ConstructInterface {
					interfaceChunks = append(interfaceChunks, chunk)
				}
			}

			var expectedChunks []outbound.SemanticCodeChunk
			if tt.useDynamicPositions {
				expectedChunks = tt.expectedChunksFunc(t)
			} else {
				expectedChunks = tt.expectedChunks
			}

			assert.Len(t, interfaceChunks, len(expectedChunks))

			for i, expected := range expectedChunks {
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

func TestGoParserVariableAndConstantExtraction(t *testing.T) {
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
				IncludePrivate:   true,
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

					// Debug logging for position mismatches
					if expected.StartByte != actual.StartByte {
						t.Logf("StartByte mismatch for %s: expected 0x%x (%d), actual 0x%x (%d)",
							expected.Name, expected.StartByte, expected.StartByte, actual.StartByte, actual.StartByte)
					}
					if expected.EndByte != actual.EndByte {
						t.Logf("EndByte mismatch for %s: expected 0x%x (%d), actual 0x%x (%d)",
							expected.Name, expected.EndByte, expected.EndByte, actual.EndByte, actual.EndByte)
					}

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

func TestGoParserPackageExtraction(t *testing.T) {
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
			name: "Package declaration with documentation",
			sourceCode: `
// Package mathutils provides mathematical utility functions
package mathutils

// Add adds two integers
func Add(a, b int) int {
	return a + b
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "package:mathutils",
					Type:          outbound.ConstructPackage,
					Name:          "mathutils",
					QualifiedName: "mathutils",
					Documentation: "Package mathutils provides mathematical utility functions",
					Content:       "package mathutils",
					StartByte:     55,
					EndByte:       72,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Package with build tags",
			sourceCode: `
//go:build linux && amd64
// +build linux,amd64

// Package osutils provides OS-specific utilities
package osutils
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "package:osutils",
					Type:          outbound.ConstructPackage,
					Name:          "osutils",
					QualifiedName: "osutils",
					Documentation: "Package osutils provides OS-specific utilities",
					Content:       "package osutils",
					StartByte:     55,
					EndByte:       70,
					Language:      valueobject.Go,
				},
			},
		},
		{
			name: "Package with file-level documentation",
			sourceCode: `
// This file contains utility functions for string manipulation
// and processing.

package stringutils

// Reverse reverses a string
func Reverse(s string) string {
	// implementation
	return s
}
`,
			expectedChunks: []outbound.SemanticCodeChunk{
				{
					ChunkID:       "package:stringutils",
					Type:          outbound.ConstructPackage,
					Name:          "stringutils",
					QualifiedName: "stringutils",
					Documentation: "This file contains utility functions for string manipulation and processing.",
					Content:       "package stringutils",
					StartByte:     87,
					EndByte:       106,
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
				IncludePackages: true,
			}

			chunks := adapter.ExtractCodeChunks(domainTree, options)

			// Filter to only package chunks for this test
			var packageChunks []outbound.SemanticCodeChunk
			for _, chunk := range chunks {
				if chunk.Type == outbound.ConstructPackage {
					packageChunks = append(packageChunks, chunk)
				}
			}

			assert.Len(t, packageChunks, len(tt.expectedChunks))

			for i, expected := range tt.expectedChunks {
				if i < len(packageChunks) {
					actual := packageChunks[i]
					assert.Equal(t, expected.ChunkID, actual.ChunkID)
					assert.Equal(t, expected.Type, actual.Type)
					assert.Equal(t, expected.Name, actual.Name)
					assert.Equal(t, expected.QualifiedName, actual.QualifiedName)
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

func TestGoParserAdvancedFeatures(t *testing.T) {
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

func TestGoParserIntegration(t *testing.T) {
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
		IncludePrivate:    true,
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

func TestGoParserErrorHandling(t *testing.T) {
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
