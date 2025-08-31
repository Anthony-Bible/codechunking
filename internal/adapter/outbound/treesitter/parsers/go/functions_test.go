package goparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoFunctionParser_ParseGoFunction_RegularFunction tests parsing of regular Go functions.
// This is a RED PHASE test that defines expected behavior for basic Go function parsing.
func TestGoFunctionParser_ParseGoFunction_RegularFunction(t *testing.T) {
	sourceCode := `package main

import "fmt"

func Add(a int, b int) int {
	return a + b
}

func main() {
	fmt.Println("Hello, World!")
}`

	// Create parse tree and parser
	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoFunctionParser()

	// Find function nodes
	functionNodes := parseTree.GetNodesByType("function_declaration")
	require.Len(t, functionNodes, 2, "Should find 2 function declarations")

	// Test Add function
	addNode := functionNodes[0]
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoFunction(context.Background(), parseTree, addNode, "main", options, time.Now())
	require.NotNil(t, result, "ParseGoFunction should return a result")

	// Validate the parsed function
	assert.Equal(t, outbound.ConstructFunction, result.Type)
	assert.Equal(t, "Add", result.Name)
	assert.Equal(t, "main.Add", result.QualifiedName)
	assert.Equal(t, "int", result.ReturnType)
	assert.Equal(t, outbound.Public, result.Visibility)
	assert.False(t, result.IsGeneric)
	assert.False(t, result.IsAsync)
	assert.False(t, result.IsAbstract)

	// Validate parameters
	require.Len(t, result.Parameters, 2)
	assert.Equal(t, "a", result.Parameters[0].Name)
	assert.Equal(t, "int", result.Parameters[0].Type)
	assert.False(t, result.Parameters[0].IsVariadic)
	assert.Equal(t, "b", result.Parameters[1].Name)
	assert.Equal(t, "int", result.Parameters[1].Type)
	assert.False(t, result.Parameters[1].IsVariadic)
}

// TestGoFunctionParser_ParseGoFunction_Generic tests parsing of generic Go functions.
// This is a RED PHASE test that defines expected behavior for Go generic function parsing.
func TestGoFunctionParser_ParseGoFunction_Generic(t *testing.T) {
	sourceCode := `package main

func Map[T any, U any](slice []T, fn func(T) U) []U {
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}

func Identity[T comparable](value T) T {
	return value
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoFunctionParser()

	functionNodes := parseTree.GetNodesByType("function_declaration")
	require.Len(t, functionNodes, 2, "Should find 2 function declarations")

	// Test Map function
	mapNode := functionNodes[0]
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoFunction(context.Background(), parseTree, mapNode, "main", options, time.Now())
	require.NotNil(t, result)

	// Validate generic function properties
	assert.Equal(t, "Map", result.Name)
	assert.True(t, result.IsGeneric)
	assert.Equal(t, "[]U", result.ReturnType)

	// Validate generic parameters
	require.Len(t, result.GenericParameters, 2)
	assert.Equal(t, "T", result.GenericParameters[0].Name)
	assert.Equal(t, []string{"any"}, result.GenericParameters[0].Constraints)
	assert.Equal(t, "U", result.GenericParameters[1].Name)
	assert.Equal(t, []string{"any"}, result.GenericParameters[1].Constraints)

	// Validate function parameters
	require.Len(t, result.Parameters, 2)
	assert.Equal(t, "slice", result.Parameters[0].Name)
	assert.Equal(t, "[]T", result.Parameters[0].Type)
	assert.Equal(t, "fn", result.Parameters[1].Name)
	assert.Equal(t, "func(T) U", result.Parameters[1].Type)
}

// TestGoFunctionParser_ParseGoFunction_Variadic tests parsing of variadic Go functions.
// This is a RED PHASE test that defines expected behavior for Go variadic function parsing.
func TestGoFunctionParser_ParseGoFunction_Variadic(t *testing.T) {
	sourceCode := `package main

func Printf(format string, args ...interface{}) (int, error) {
	return fmt.Printf(format, args...)
}

func Sum(numbers ...int) int {
	total := 0
	for _, num := range numbers {
		total += num
	}
	return total
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoFunctionParser()

	functionNodes := parseTree.GetNodesByType("function_declaration")
	require.Len(t, functionNodes, 2, "Should find 2 function declarations")

	// Test Printf function
	printfNode := functionNodes[0]
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoFunction(context.Background(), parseTree, printfNode, "main", options, time.Now())
	require.NotNil(t, result)

	// Validate variadic function
	assert.Equal(t, "Printf", result.Name)
	assert.Equal(t, "(int, error)", result.ReturnType)

	require.Len(t, result.Parameters, 2)
	assert.Equal(t, "format", result.Parameters[0].Name)
	assert.Equal(t, "string", result.Parameters[0].Type)
	assert.False(t, result.Parameters[0].IsVariadic)

	assert.Equal(t, "args", result.Parameters[1].Name)
	assert.Equal(t, "interface{}", result.Parameters[1].Type)
	assert.True(t, result.Parameters[1].IsVariadic)
}

// TestGoFunctionParser_ParseGoMethod_BasicMethod tests parsing of Go methods.
// This is a RED PHASE test that defines expected behavior for Go method parsing.
func TestGoFunctionParser_ParseGoMethod_BasicMethod(t *testing.T) {
	sourceCode := `package main

type Calculator struct {
	result float64
}

func (c *Calculator) Add(value float64) float64 {
	c.result += value
	return c.result
}

func (c Calculator) GetResult() float64 {
	return c.result
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoFunctionParser()

	methodNodes := parseTree.GetNodesByType("method_declaration")
	require.Len(t, methodNodes, 2, "Should find 2 method declarations")

	// Test Add method (pointer receiver)
	addMethodNode := methodNodes[0]
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoMethod(context.Background(), parseTree, addMethodNode, "main", options, time.Now())
	require.NotNil(t, result)

	// Validate method properties
	assert.Equal(t, outbound.ConstructMethod, result.Type)
	assert.Equal(t, "Add", result.Name)
	assert.Equal(t, "main.Calculator.Add", result.QualifiedName)
	assert.Equal(t, "float64", result.ReturnType)
	assert.Equal(t, outbound.Public, result.Visibility)

	// Validate parameters (should include receiver + method parameters)
	require.Len(t, result.Parameters, 2)
	assert.Equal(t, "c", result.Parameters[0].Name)
	assert.Equal(t, "*Calculator", result.Parameters[0].Type)
	assert.Equal(t, "value", result.Parameters[1].Name)
	assert.Equal(t, "float64", result.Parameters[1].Type)
}

// TestGoFunctionParser_ParseGoMethod_GenericMethod tests parsing of generic Go methods.
// This is a RED PHASE test that defines expected behavior for Go generic method parsing.
func TestGoFunctionParser_ParseGoMethod_GenericMethod(t *testing.T) {
	sourceCode := `package main

type Container[T any] struct {
	items []T
}

func (c *Container[T]) Add(item T) {
	c.items = append(c.items, item)
}

func (c *Container[T]) Get(index int) (T, bool) {
	if index < 0 || index >= len(c.items) {
		var zero T
		return zero, false
	}
	return c.items[index], true
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoFunctionParser()

	methodNodes := parseTree.GetNodesByType("method_declaration")
	require.Len(t, methodNodes, 2, "Should find 2 method declarations")

	// Test Add method
	addMethodNode := methodNodes[0]
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParseGoMethod(context.Background(), parseTree, addMethodNode, "main", options, time.Now())
	require.NotNil(t, result)

	// Validate generic method
	assert.Equal(t, "Add", result.Name)
	assert.Equal(t, "main.Container.Add", result.QualifiedName)

	// Validate parameters
	require.Len(t, result.Parameters, 2)
	assert.Equal(t, "c", result.Parameters[0].Name)
	assert.Equal(t, "*Container[T]", result.Parameters[0].Type)
	assert.Equal(t, "item", result.Parameters[1].Name)
	assert.Equal(t, "T", result.Parameters[1].Type)
}

// TestGoFunctionParser_ParseGoParameters_ComplexTypes tests parameter parsing.
// This is a RED PHASE test that defines expected behavior for Go parameter parsing.
func TestGoFunctionParser_ParseGoParameters_ComplexTypes(t *testing.T) {
	sourceCode := `package main

func ComplexFunction(
	simple int,
	slice []string,
	pointer *User,
	channel chan<- int,
	function func(int) string,
	variadic ...interface{},
) (result string, err error) {
	return "", nil
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoFunctionParser()

	functionNodes := parseTree.GetNodesByType("function_declaration")
	require.Len(t, functionNodes, 1)

	result := parser.ParseGoParameters(parseTree, functionNodes[0])
	require.Len(t, result, 6, "Should parse 6 parameters")

	// Validate each parameter type
	expected := []struct {
		name      string
		paramType string
		variadic  bool
	}{
		{"simple", "int", false},
		{"slice", "[]string", false},
		{"pointer", "*User", false},
		{"channel", "chan<- int", false},
		{"function", "func(int) string", false},
		{"variadic", "interface{}", true},
	}

	for i, exp := range expected {
		assert.Equal(t, exp.name, result[i].Name, "Parameter name at index %d", i)
		assert.Equal(t, exp.paramType, result[i].Type, "Parameter type at index %d", i)
		assert.Equal(t, exp.variadic, result[i].IsVariadic, "Parameter variadic flag at index %d", i)
	}
}

// TestGoFunctionParser_ParseGoReturnType_MultipleReturns tests return type parsing.
// This is a RED PHASE test that defines expected behavior for Go return type parsing.
func TestGoFunctionParser_ParseGoReturnType_MultipleReturns(t *testing.T) {
	testCases := []struct {
		name       string
		sourceCode string
		expected   string
	}{
		{
			name: "single return",
			sourceCode: `package main
func SingleReturn() int {
	return 42
}`,
			expected: "int",
		},
		{
			name: "multiple returns",
			sourceCode: `package main
func MultipleReturns() (int, error) {
	return 42, nil
}`,
			expected: "(int, error)",
		},
		{
			name: "named returns",
			sourceCode: `package main
func NamedReturns() (result int, err error) {
	return
}`,
			expected: "(result int, err error)",
		},
		{
			name: "no return",
			sourceCode: `package main
func NoReturn() {
	// do something
}`,
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			language, err := valueobject.NewLanguage(valueobject.LanguageGo)
			require.NoError(t, err)

			parseTree := createMockParseTreeFromSource(t, language, tc.sourceCode)
			parser := NewGoFunctionParser()

			functionNodes := parseTree.GetNodesByType("function_declaration")
			require.Len(t, functionNodes, 1)

			result := parser.ParseGoReturnType(parseTree, functionNodes[0])
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestGoFunctionParser_ParseGoMethodParameters_ReceiverHandling tests method parameter parsing with receivers.
// This is a RED PHASE test that defines expected behavior for Go method parameter parsing.
func TestGoFunctionParser_ParseGoMethodParameters_ReceiverHandling(t *testing.T) {
	sourceCode := `package main

type Service struct {
	name string
}

func (s *Service) ProcessData(data []byte, options map[string]interface{}) (*Result, error) {
	return nil, nil
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoFunctionParser()

	methodNodes := parseTree.GetNodesByType("method_declaration")
	require.Len(t, methodNodes, 1)

	result := parser.ParseGoMethodParameters(parseTree, methodNodes[0], "*Service")
	require.Len(t, result, 3, "Should include receiver + 2 method parameters")

	// Validate receiver parameter
	assert.Equal(t, "s", result[0].Name)
	assert.Equal(t, "*Service", result[0].Type)
	assert.False(t, result[0].IsVariadic)

	// Validate method parameters
	assert.Equal(t, "data", result[1].Name)
	assert.Equal(t, "[]byte", result[1].Type)
	assert.False(t, result[1].IsVariadic)

	assert.Equal(t, "options", result[2].Name)
	assert.Equal(t, "map[string]interface{}", result[2].Type)
	assert.False(t, result[2].IsVariadic)
}

// TestGoFunctionParser_ErrorHandling tests error conditions.
// This is a RED PHASE test that defines expected behavior for error handling.
func TestGoFunctionParser_ErrorHandling(t *testing.T) {
	t.Run("nil parse tree should not panic", func(t *testing.T) {
		parser := NewGoFunctionParser()

		// This should not panic and should return nil
		result := parser.ParseGoFunction(
			context.Background(),
			nil,
			nil,
			"main",
			outbound.SemanticExtractionOptions{},
			time.Now(),
		)
		assert.Nil(t, result)
	})

	t.Run("nil node should not panic", func(t *testing.T) {
		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, "package main")
		parser := NewGoFunctionParser()

		result := parser.ParseGoFunction(
			context.Background(),
			parseTree,
			nil,
			"main",
			outbound.SemanticExtractionOptions{},
			time.Now(),
		)
		assert.Nil(t, result)
	})

	t.Run("malformed function should return nil", func(t *testing.T) {
		// Test with incomplete function syntax
		sourceCode := `package main
func Incomplete(`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser := NewGoFunctionParser()

		// Even if tree-sitter creates nodes for malformed code, our parser should handle it gracefully
		functionNodes := parseTree.GetNodesByType("function_declaration")
		if len(functionNodes) > 0 {
			result := parser.ParseGoFunction(
				context.Background(),
				parseTree,
				functionNodes[0],
				"main",
				outbound.SemanticExtractionOptions{},
				time.Now(),
			)
			// Should either return nil or a partial result, but not panic
			if result != nil {
				assert.NotEmpty(t, result.Name)
			}
		}
	})
}

// TestGoFunctionParser_PrivateVisibilityFiltering tests visibility filtering.
// This is a RED PHASE test that defines expected behavior for private function filtering.
func TestGoFunctionParser_PrivateVisibilityFiltering(t *testing.T) {
	sourceCode := `package main

func PublicFunction() {
	// public function
}

func privateFunction() {
	// private function
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoFunctionParser()

	functionNodes := parseTree.GetNodesByType("function_declaration")
	require.Len(t, functionNodes, 2)

	// Test with IncludePrivate: false
	optionsNoPrivate := outbound.SemanticExtractionOptions{
		IncludePrivate:  false,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	// Should return nil for private function
	privateResult := parser.ParseGoFunction(
		context.Background(),
		parseTree,
		functionNodes[1],
		"main",
		optionsNoPrivate,
		time.Now(),
	)
	assert.Nil(t, privateResult, "Private function should be filtered out when IncludePrivate is false")

	// Should return result for public function
	publicResult := parser.ParseGoFunction(
		context.Background(),
		parseTree,
		functionNodes[0],
		"main",
		optionsNoPrivate,
		time.Now(),
	)
	assert.NotNil(t, publicResult, "Public function should be included")
	assert.Equal(t, "PublicFunction", publicResult.Name)

	// Test with IncludePrivate: true
	optionsIncludePrivate := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	// Should return result for private function
	privateResult2 := parser.ParseGoFunction(
		context.Background(),
		parseTree,
		functionNodes[1],
		"main",
		optionsIncludePrivate,
		time.Now(),
	)
	assert.NotNil(t, privateResult2, "Private function should be included when IncludePrivate is true")
	assert.Equal(t, "privateFunction", privateResult2.Name)
	assert.Equal(t, outbound.Private, privateResult2.Visibility)
}

// Helper function to create mock parse trees - basic implementation for testing.
func createMockParseTreeFromSource(
	t *testing.T,
	_ valueobject.Language,
	_ string,
) *valueobject.ParseTree {
	t.Helper()
	// Return a basic mock implementation for testing purposes
	// This allows tests to run without panicking during refactor phase
	return &valueobject.ParseTree{}
}

// NewGoFunctionParser creates a new Go function parser - this will fail in RED phase.
func NewGoFunctionParser() *GoFunctionParser {
	panic("NewGoFunctionParser not implemented - this is expected in RED phase")
}

// GoFunctionParser represents the specialized Go function parser - this will fail in RED phase.
type GoFunctionParser struct{}

// ParseGoFunction method signature - this will fail in RED phase.
func (p *GoFunctionParser) ParseGoFunction(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	panic("ParseGoFunction not implemented - this is expected in RED phase")
}

// ParseGoMethod method signature - this will fail in RED phase.
func (p *GoFunctionParser) ParseGoMethod(
	ctx context.Context,
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	packageName string,
	options outbound.SemanticExtractionOptions,
	now time.Time,
) *outbound.SemanticCodeChunk {
	panic("ParseGoMethod not implemented - this is expected in RED phase")
}

// ParseGoParameters method signature - this will fail in RED phase.
func (p *GoFunctionParser) ParseGoParameters(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) []outbound.Parameter {
	panic("ParseGoParameters not implemented - this is expected in RED phase")
}

// ParseGoReturnType method signature - this will fail in RED phase.
func (p *GoFunctionParser) ParseGoReturnType(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) string {
	panic("ParseGoReturnType not implemented - this is expected in RED phase")
}

// ParseGoMethodParameters method signature - this will fail in RED phase.
func (p *GoFunctionParser) ParseGoMethodParameters(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
	receiverType string,
) []outbound.Parameter {
	panic("ParseGoMethodParameters not implemented - this is expected in RED phase")
}
