package goparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoTypeParser_ParseGoGenericParameters_BasicConstraints tests parsing of basic generic type parameters.
// This is a RED PHASE test that defines expected behavior for Go generic parameter parsing.
func TestGoTypeParser_ParseGoGenericParameters_BasicConstraints(t *testing.T) {
	sourceCode := `package main

func GenericFunction[T any, U comparable](item T, key U) T {
	return item
}

type GenericStruct[T any, U comparable, V int | string] struct {
	Value T
	Key   U
	Data  V
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoTypeParser()

	// Find function with generic parameters
	functionDecls := parseTree.GetNodesByType("function_declaration")
	require.Len(t, functionDecls, 1)

	typeParamList := findChildByType(functionDecls[0], "type_parameter_list")
	require.NotNil(t, typeParamList, "Should find type parameter list")

	result := parser.ParseGoGenericParameters(parseTree, typeParamList)
	require.Len(t, result, 2, "Should parse 2 generic parameters")

	// Validate T parameter
	tParam := result[0]
	assert.Equal(t, "T", tParam.Name)
	assert.Equal(t, []string{"any"}, tParam.Constraints)

	// Validate U parameter
	uParam := result[1]
	assert.Equal(t, "U", uParam.Name)
	assert.Equal(t, []string{"comparable"}, uParam.Constraints)

	// Test struct with more complex constraints
	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 1)

	typeSpec := findChildByType(typeDecls[0], "type_spec")
	structTypeParamList := findChildByType(typeSpec, "type_parameter_list")
	require.NotNil(t, structTypeParamList)

	structResult := parser.ParseGoGenericParameters(parseTree, structTypeParamList)
	require.Len(t, structResult, 3, "Should parse 3 generic parameters from struct")

	// Validate V parameter with union constraint
	vParam := structResult[2]
	assert.Equal(t, "V", vParam.Name)
	assert.Equal(t, []string{"int", "string"}, vParam.Constraints) // Union types should be split
}

// TestGoTypeParser_ParseGoGenericParameters_ComplexConstraints tests parsing of complex generic constraints.
// This is a RED PHASE test that defines expected behavior for complex Go generic constraint parsing.
func TestGoTypeParser_ParseGoGenericParameters_ComplexConstraints(t *testing.T) {
	sourceCode := `package main

import "io"

type ComplexGeneric[
	T interface{ 
		Read([]byte) (int, error)
		Write([]byte) (int, error)
	},
	U ~int | ~string | ~float64,
	V interface{ 
		String() string
		Compare(V) int
	},
	W io.Reader,
] struct {
	reader T
	value  U
	item   V
	stream W
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoTypeParser()

	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 1)

	typeSpec := findChildByType(typeDecls[0], "type_spec")
	typeParamList := findChildByType(typeSpec, "type_parameter_list")
	require.NotNil(t, typeParamList)

	result := parser.ParseGoGenericParameters(parseTree, typeParamList)
	require.Len(t, result, 4, "Should parse 4 complex generic parameters")

	// Validate T parameter with interface constraint
	tParam := result[0]
	assert.Equal(t, "T", tParam.Name)
	assert.Contains(t, tParam.Constraints, "interface{")
	// Should capture the full interface definition

	// Validate U parameter with union constraint using ~ (approximation)
	uParam := result[1]
	assert.Equal(t, "U", uParam.Name)
	assert.Contains(t, uParam.Constraints, "~int")
	assert.Contains(t, uParam.Constraints, "~string")
	assert.Contains(t, uParam.Constraints, "~float64")

	// Validate V parameter with interface constraint
	vParam := result[2]
	assert.Equal(t, "V", vParam.Name)
	assert.Contains(t, vParam.Constraints, "interface{")

	// Validate W parameter with named interface constraint
	wParam := result[3]
	assert.Equal(t, "W", wParam.Name)
	assert.Equal(t, []string{"io.Reader"}, wParam.Constraints)
}

// TestGoTypeParser_ParseGoGenericParameters_NestedGenerics tests parsing of nested generic parameters.
// This is a RED PHASE test that defines expected behavior for nested Go generic parameter parsing.
func TestGoTypeParser_ParseGoGenericParameters_NestedGenerics(t *testing.T) {
	sourceCode := `package main

type Container[T any] struct {
	items []T
}

type NestedGeneric[
	T Container[U],
	U comparable,
	V map[U][]T,
] struct {
	data V
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoTypeParser()

	// Find the second type declaration (NestedGeneric)
	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 2)

	nestedTypeSpec := findChildByType(typeDecls[1], "type_spec")
	typeParamList := findChildByType(nestedTypeSpec, "type_parameter_list")
	require.NotNil(t, typeParamList)

	result := parser.ParseGoGenericParameters(parseTree, typeParamList)
	require.Len(t, result, 3, "Should parse 3 nested generic parameters")

	// Validate T parameter with generic constraint
	tParam := result[0]
	assert.Equal(t, "T", tParam.Name)
	assert.Equal(t, []string{"Container[U]"}, tParam.Constraints)

	// Validate U parameter
	uParam := result[1]
	assert.Equal(t, "U", uParam.Name)
	assert.Equal(t, []string{"comparable"}, uParam.Constraints)

	// Validate V parameter with complex map type
	vParam := result[2]
	assert.Equal(t, "V", vParam.Name)
	assert.Equal(t, []string{"map[U][]T"}, vParam.Constraints)
}

// TestGoTypeParser_ParseGoReceiver_ValueAndPointer tests parsing of method receivers.
// This is a RED PHASE test that defines expected behavior for Go method receiver parsing.
func TestGoTypeParser_ParseGoReceiver_ValueAndPointer(t *testing.T) {
	sourceCode := `package main

type User struct {
	Name string
	Age  int
}

func (u User) GetName() string {
	return u.Name
}

func (u *User) SetName(name string) {
	u.Name = name
}

type Container[T any] struct {
	items []T
}

func (c *Container[T]) Add(item T) {
	c.items = append(c.items, item)
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoTypeParser()

	// Find method declarations
	methodDecls := parseTree.GetNodesByType("method_declaration")
	require.Len(t, methodDecls, 3, "Should find 3 method declarations")

	// Test value receiver
	valueReceiverMethod := methodDecls[0]
	valueReceiverParamList := findChildByType(valueReceiverMethod, "parameter_list")
	require.NotNil(t, valueReceiverParamList)

	valueReceiverType := parser.ParseGoReceiver(parseTree, valueReceiverParamList)
	assert.Equal(t, "User", valueReceiverType, "Should parse value receiver type")

	// Test pointer receiver
	pointerReceiverMethod := methodDecls[1]
	pointerReceiverParamList := findChildByType(pointerReceiverMethod, "parameter_list")
	require.NotNil(t, pointerReceiverParamList)

	pointerReceiverType := parser.ParseGoReceiver(parseTree, pointerReceiverParamList)
	assert.Equal(t, "*User", pointerReceiverType, "Should parse pointer receiver type")

	// Test generic receiver
	genericReceiverMethod := methodDecls[2]
	genericReceiverParamList := findChildByType(genericReceiverMethod, "parameter_list")
	require.NotNil(t, genericReceiverParamList)

	genericReceiverType := parser.ParseGoReceiver(parseTree, genericReceiverParamList)
	assert.Equal(t, "*Container[T]", genericReceiverType, "Should parse generic receiver type")
}

// TestGoTypeParser_ParseGoReceiver_QualifiedTypes tests parsing of receivers with qualified types.
// This is a RED PHASE test that defines expected behavior for qualified Go receiver type parsing.
func TestGoTypeParser_ParseGoReceiver_QualifiedTypes(t *testing.T) {
	sourceCode := `package main

import (
	"context"
	"net/http"
)

type Handler struct {
	ctx context.Context
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handler implementation
}

type Wrapper[T context.Context] struct {
	value T
}

func (w *Wrapper[T]) GetContext() T {
	return w.value
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoTypeParser()

	methodDecls := parseTree.GetNodesByType("method_declaration")
	require.Len(t, methodDecls, 2)

	// Test qualified receiver
	handlerMethod := methodDecls[0]
	handlerReceiverParamList := findChildByType(handlerMethod, "parameter_list")
	require.NotNil(t, handlerReceiverParamList)

	handlerReceiverType := parser.ParseGoReceiver(parseTree, handlerReceiverParamList)
	assert.Equal(t, "*Handler", handlerReceiverType, "Should parse qualified receiver type")

	// Test generic qualified receiver
	wrapperMethod := methodDecls[1]
	wrapperReceiverParamList := findChildByType(wrapperMethod, "parameter_list")
	require.NotNil(t, wrapperReceiverParamList)

	wrapperReceiverType := parser.ParseGoReceiver(parseTree, wrapperReceiverParamList)
	assert.Equal(t, "*Wrapper[T]", wrapperReceiverType, "Should parse generic qualified receiver type")
}

// TestGoTypeParser_TypeParameterConstraintParsing tests parsing of various constraint types.
// This is a RED PHASE test that defines expected behavior for Go type constraint parsing.
func TestGoTypeParser_TypeParameterConstraintParsing(t *testing.T) {
	testCases := []struct {
		name           string
		sourceCode     string
		expectedParams []outbound.GenericParameter
	}{
		{
			name: "basic constraints",
			sourceCode: `package main
type Basic[T any, U comparable] struct{}`,
			expectedParams: []outbound.GenericParameter{
				{Name: "T", Constraints: []string{"any"}},
				{Name: "U", Constraints: []string{"comparable"}},
			},
		},
		{
			name: "union constraints",
			sourceCode: `package main
type Union[T int | string | float64] struct{}`,
			expectedParams: []outbound.GenericParameter{
				{Name: "T", Constraints: []string{"int", "string", "float64"}},
			},
		},
		{
			name: "approximation constraints",
			sourceCode: `package main
type Approx[T ~int | ~string] struct{}`,
			expectedParams: []outbound.GenericParameter{
				{Name: "T", Constraints: []string{"~int", "~string"}},
			},
		},
		{
			name: "interface constraints",
			sourceCode: `package main
type InterfaceConstraint[T interface{ String() string }] struct{}`,
			expectedParams: []outbound.GenericParameter{
				{Name: "T", Constraints: []string{"interface{ String() string }"}},
			},
		},
		{
			name: "named interface constraints",
			sourceCode: `package main
import "io"
type NamedInterface[T io.Reader] struct{}`,
			expectedParams: []outbound.GenericParameter{
				{Name: "T", Constraints: []string{"io.Reader"}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			language, err := valueobject.NewLanguage(valueobject.LanguageGo)
			require.NoError(t, err)

			parseTree := createMockParseTreeFromSource(t, language, tc.sourceCode)
			parser := NewGoTypeParser()

			typeDecls := parseTree.GetNodesByType("type_declaration")
			require.Len(t, typeDecls, 1)

			typeSpec := findChildByType(typeDecls[0], "type_spec")
			typeParamList := findChildByType(typeSpec, "type_parameter_list")
			require.NotNil(t, typeParamList)

			result := parser.ParseGoGenericParameters(parseTree, typeParamList)
			require.Len(t, result, len(tc.expectedParams))

			for i, expected := range tc.expectedParams {
				assert.Equal(t, expected.Name, result[i].Name)
				assert.Equal(t, expected.Constraints, result[i].Constraints)
			}
		})
	}
}

// TestGoTypeParser_GenericTypeInferenceHelpers tests helper functions for generic type inference.
// This is a RED PHASE test that defines expected behavior for Go generic type helper functions.
func TestGoTypeParser_GenericTypeInferenceHelpers(t *testing.T) {
	sourceCode := `package main

func GenericFunc[T any, U comparable](a T, b U) (T, U) {
	return a, b
}

type GenericType[T any] struct {
	Value T
}

func (g GenericType[T]) Method() T {
	return g.Value
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoTypeParser()

	// Test if a node has generic parameters
	functionDecls := parseTree.GetNodesByType("function_declaration")
	require.Len(t, functionDecls, 1)

	hasGenerics := parser.HasGenericParameters(functionDecls[0])
	assert.True(t, hasGenerics, "Function should have generic parameters")

	// Test getting generic parameter names
	typeParamList := findChildByType(functionDecls[0], "type_parameter_list")
	require.NotNil(t, typeParamList)

	paramNames := parser.GetGenericParameterNames(parseTree, typeParamList)
	assert.Equal(t, []string{"T", "U"}, paramNames, "Should extract parameter names")

	// Test constraint parsing
	constraints := parser.ParseConstraints(parseTree, typeParamList)
	expectedConstraints := map[string][]string{
		"T": {"any"},
		"U": {"comparable"},
	}
	assert.Equal(t, expectedConstraints, constraints, "Should parse constraints correctly")
}

// TestGoTypeParser_ErrorHandling tests error conditions for type parsing.
// This is a RED PHASE test that defines expected behavior for error handling.
func TestGoTypeParser_ErrorHandling(t *testing.T) {
	t.Run("nil parse tree should not panic", func(t *testing.T) {
		parser := NewGoTypeParser()

		result := parser.ParseGoGenericParameters(nil, nil)
		assert.Nil(t, result)
	})

	t.Run("nil node should not panic", func(t *testing.T) {
		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, "package main")
		parser := NewGoTypeParser()

		result := parser.ParseGoGenericParameters(parseTree, nil)
		assert.Nil(t, result)
	})

	t.Run("malformed generic parameters should handle gracefully", func(t *testing.T) {
		sourceCode := `package main
type Incomplete[T`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser := NewGoTypeParser()

		typeDecls := parseTree.GetNodesByType("type_declaration")
		if len(typeDecls) > 0 {
			typeSpec := findChildByType(typeDecls[0], "type_spec")
			if typeSpec != nil {
				typeParamList := findChildByType(typeSpec, "type_parameter_list")
				if typeParamList != nil {
					result := parser.ParseGoGenericParameters(parseTree, typeParamList)
					// Should either return nil/empty slice or partial results, but not panic
					if result != nil {
						for _, param := range result {
							assert.NotEmpty(t, param.Name, "Parameter name should not be empty if parsed")
						}
					}
				}
			}
		}
	})

	t.Run("empty receiver should return empty string", func(t *testing.T) {
		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, "package main")
		parser := NewGoTypeParser()

		result := parser.ParseGoReceiver(parseTree, nil)
		assert.Empty(t, result)
	})
}

// TestGoTypeParser_GenericConstraintComplexity tests parsing of highly complex generic constraints.
// This is a RED PHASE test that defines expected behavior for complex Go generic constraint parsing.
func TestGoTypeParser_GenericConstraintComplexity(t *testing.T) {
	sourceCode := `package main

type ComplexConstraints[
	T interface {
		~int | ~int32 | ~int64
		String() string
		comparable
	},
	U interface {
		Read([]byte) (int, error)
		Close() error
	} | interface {
		Write([]byte) (int, error)
		Close() error
	},
	V ~[]T | ~map[string]T | ~chan T,
] struct {
	data map[string]interface{}
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewGoTypeParser()

	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 1)

	typeSpec := findChildByType(typeDecls[0], "type_spec")
	typeParamList := findChildByType(typeSpec, "type_parameter_list")
	require.NotNil(t, typeParamList)

	result := parser.ParseGoGenericParameters(parseTree, typeParamList)
	require.Len(t, result, 3, "Should parse 3 complex constraint parameters")

	// Validate T parameter with complex interface constraint
	tParam := result[0]
	assert.Equal(t, "T", tParam.Name)
	assert.NotEmpty(t, tParam.Constraints, "Should have constraints")
	// Complex interface should be captured

	// Validate U parameter with union of interfaces
	uParam := result[1]
	assert.Equal(t, "U", uParam.Name)
	assert.NotEmpty(t, uParam.Constraints, "Should have union interface constraints")

	// Validate V parameter with approximated generic types
	vParam := result[2]
	assert.Equal(t, "V", vParam.Name)
	assert.NotEmpty(t, vParam.Constraints, "Should have approximated type constraints")
}

// Helper functions - these will fail in RED phase as expected

// NewGoTypeParser creates a new Go type parser - this will fail in RED phase.
func NewGoTypeParser() *GoTypeParser {
	panic("NewGoTypeParser not implemented - this is expected in RED phase")
}

// GoTypeParser represents the specialized Go type parser - this will fail in RED phase.
type GoTypeParser struct{}

// ParseGoGenericParameters method signature - this will fail in RED phase.
func (p *GoTypeParser) ParseGoGenericParameters(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) []outbound.GenericParameter {
	panic("ParseGoGenericParameters not implemented - this is expected in RED phase")
}

// ParseGoReceiver method signature - this will fail in RED phase.
func (p *GoTypeParser) ParseGoReceiver(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) string {
	panic("ParseGoReceiver not implemented - this is expected in RED phase")
}

// HasGenericParameters method signature - this will fail in RED phase.
func (p *GoTypeParser) HasGenericParameters(node *valueobject.ParseNode) bool {
	panic("HasGenericParameters not implemented - this is expected in RED phase")
}

// GetGenericParameterNames method signature - this will fail in RED phase.
func (p *GoTypeParser) GetGenericParameterNames(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) []string {
	panic("GetGenericParameterNames not implemented - this is expected in RED phase")
}

// ParseConstraints method signature - this will fail in RED phase.
func (p *GoTypeParser) ParseConstraints(
	parseTree *valueobject.ParseTree,
	node *valueobject.ParseNode,
) map[string][]string {
	panic("ParseConstraints not implemented - this is expected in RED phase")
}
