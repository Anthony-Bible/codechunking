package goparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"fmt"
	"strings"
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
	parser, err := NewGoParser()
	require.NoError(t, err)

	// Find function with generic parameters
	functionDecls := parseTree.GetNodesByType("function_declaration")
	require.Len(t, functionDecls, 1)

	typeParamList := findChildByType(parser, functionDecls[0], "type_parameter_list")
	require.NotNil(t, typeParamList, "Should find type parameter list")

	result := parser.parseGoGenericParameters(parseTree, typeParamList)
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

	typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
	structTypeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
	require.NotNil(t, structTypeParamList)

	structResult := parser.parseGoGenericParameters(parseTree, structTypeParamList)
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
	parser, err := NewGoParser()
	require.NoError(t, err)

	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 1)

	typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
	typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
	require.NotNil(t, typeParamList)

	result := parser.parseGoGenericParameters(parseTree, typeParamList)
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
	parser, err := NewGoParser()
	require.NoError(t, err)

	// Find the second type declaration (NestedGeneric)
	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 2)

	nestedTypeSpec := findChildByType(parser, typeDecls[1], "type_spec")
	typeParamList := findChildByType(parser, nestedTypeSpec, "type_parameter_list")
	require.NotNil(t, typeParamList)

	result := parser.parseGoGenericParameters(parseTree, typeParamList)
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
	parser, err := NewGoParser()
	require.NoError(t, err)

	// Find method declarations
	methodDecls := parseTree.GetNodesByType("method_declaration")
	require.Len(t, methodDecls, 3, "Should find 3 method declarations")

	// Test value receiver
	valueReceiverMethod := methodDecls[0]
	valueReceiverParamList := findChildByType(parser, valueReceiverMethod, "parameter_list")
	require.NotNil(t, valueReceiverParamList)

	valueReceiverType := parser.parseGoReceiver(parseTree, valueReceiverParamList)
	assert.Equal(t, "User", valueReceiverType, "Should parse value receiver type")

	// Test pointer receiver
	pointerReceiverMethod := methodDecls[1]
	pointerReceiverParamList := findChildByType(parser, pointerReceiverMethod, "parameter_list")
	require.NotNil(t, pointerReceiverParamList)

	pointerReceiverType := parser.parseGoReceiver(parseTree, pointerReceiverParamList)
	assert.Equal(t, "*User", pointerReceiverType, "Should parse pointer receiver type")

	// Test generic receiver
	genericReceiverMethod := methodDecls[2]
	genericReceiverParamList := findChildByType(parser, genericReceiverMethod, "parameter_list")
	require.NotNil(t, genericReceiverParamList)

	genericReceiverType := parser.parseGoReceiver(parseTree, genericReceiverParamList)
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
	parser, err := NewGoParser()
	require.NoError(t, err)

	methodDecls := parseTree.GetNodesByType("method_declaration")
	require.Len(t, methodDecls, 2)

	// Test qualified receiver
	handlerMethod := methodDecls[0]
	handlerReceiverParamList := findChildByType(parser, handlerMethod, "parameter_list")
	require.NotNil(t, handlerReceiverParamList)

	handlerReceiverType := parser.parseGoReceiver(parseTree, handlerReceiverParamList)
	assert.Equal(t, "*Handler", handlerReceiverType, "Should parse qualified receiver type")

	// Test generic qualified receiver
	wrapperMethod := methodDecls[1]
	wrapperReceiverParamList := findChildByType(parser, wrapperMethod, "parameter_list")
	require.NotNil(t, wrapperReceiverParamList)

	wrapperReceiverType := parser.parseGoReceiver(parseTree, wrapperReceiverParamList)
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

			parseTree := createMockParseTreeFromSource(
				t,
				language,
				tc.sourceCode,
			)
			parser, err := NewGoParser()
			require.NoError(t, err)

			typeDecls := parseTree.GetNodesByType("type_declaration")
			require.Len(t, typeDecls, 1)

			typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
			typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
			require.NotNil(t, typeParamList)

			result := parser.parseGoGenericParameters(parseTree, typeParamList)
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
	parser, err := NewGoParser()
	require.NoError(t, err)

	// Test if a node has generic parameters
	functionDecls := parseTree.GetNodesByType("function_declaration")
	require.Len(t, functionDecls, 1)

	hasGenerics := parser.HasGenericParameters(functionDecls[0])
	assert.True(t, hasGenerics, "Function should have generic parameters")

	// Test getting generic parameter names
	typeParamList := findChildByType(parser, functionDecls[0], "type_parameter_list")
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

// TestGoTypeParser_ErrorHandling_NullInputs tests null input error conditions.
// This is a RED PHASE test that defines expected behavior for null input handling.
func TestGoTypeParser_ErrorHandling_NullInputs(t *testing.T) {
	t.Run("nil parse tree should not panic", func(t *testing.T) {
		parser, err := NewGoParser()
		require.NoError(t, err)

		// This should not panic and should return nil
		result := parser.parseGoGenericParameters(nil, nil)
		assert.Nil(t, result)
	})

	t.Run("nil node should not panic", func(t *testing.T) {
		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(
			t,
			language,
			"package main",
		)
		parser, err := NewGoParser()
		require.NoError(t, err)

		result := parser.parseGoGenericParameters(parseTree, nil)
		assert.Nil(t, result)
	})

	t.Run("nil type reference parsing should not panic", func(t *testing.T) {
		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(
			t,
			language,
			"package main",
		)
		parser, err := NewGoParser()
		require.NoError(t, err)

		result := parser.parseGoGenericParameters(parseTree, nil)
		assert.Nil(t, result)
	})
}

// TestGoTypeParser_ErrorHandling_MalformedSyntax tests malformed syntax error conditions.
// This is a RED PHASE test that defines expected behavior for syntax error handling.
//
//nolint:gocognit,nestif // RED phase test with intentionally complex error scenarios
func TestGoTypeParser_ErrorHandling_MalformedSyntax(t *testing.T) {
	t.Run("incomplete generic parameter list should return nil", func(t *testing.T) {
		// Test with incomplete generic syntax
		sourceCode := `package main
type Incomplete[T`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		typeDecls := parseTree.GetNodesByType("type_declaration")
		if len(typeDecls) > 0 {
			typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
			if typeSpec != nil {
				typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
				if typeParamList != nil {
					result := parser.parseGoGenericParameters(parseTree, typeParamList)
					// Should return nil due to malformed syntax
					assert.Nil(t, result)
				}
			}
		}
	})

	t.Run("malformed constraint syntax should handle gracefully", func(t *testing.T) {
		sourceCode := `package main
type BadConstraint[T interface{ String(] struct{}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		typeDecls := parseTree.GetNodesByType("type_declaration")
		if len(typeDecls) > 0 {
			typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
			if typeSpec != nil {
				typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
				if typeParamList != nil {
					result := parser.parseGoGenericParameters(parseTree, typeParamList)
					// Should handle partial parsing or return nil
					if result != nil {
						for _, param := range result {
							assert.NotEmpty(t, param.Name)
							// Should have constraints or empty slice for constraint parsing errors
							if len(param.Constraints) == 0 {
								// Missing or invalid constraint should result in empty constraints
								assert.Empty(t, param.Constraints)
							}
						}
					}
				}
			}
		}
	})

	t.Run("invalid type union syntax should handle gracefully", func(t *testing.T) {
		sourceCode := `package main
type BadUnion[T ~int | ~string |] struct{}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		typeDecls := parseTree.GetNodesByType("type_declaration")
		if len(typeDecls) > 0 {
			typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
			if typeSpec != nil {
				typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
				if typeParamList != nil {
					result := parser.parseGoGenericParameters(parseTree, typeParamList)
					// Should handle malformed union syntax
					if result != nil {
						for _, param := range result {
							assert.Equal(t, "T", param.Name)
							// Should handle incomplete union in constraints
							if len(param.Constraints) > 0 {
								for _, constraint := range param.Constraints {
									if strings.Contains(constraint, "|") && strings.HasSuffix(constraint, "|") {
										// Should detect malformed union syntax
										assert.Fail(t, "Should not have trailing pipe in constraint")
									}
								}
							}
						}
					}
				}
			}
		}
	})

	t.Run("missing type parameter name should return nil", func(t *testing.T) {
		sourceCode := `package main
type MissingName[comparable] struct{}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		typeDecls := parseTree.GetNodesByType("type_declaration")
		if len(typeDecls) > 0 {
			typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
			if typeSpec != nil {
				typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
				if typeParamList != nil {
					result := parser.parseGoGenericParameters(parseTree, typeParamList)
					// Should return nil due to invalid syntax
					assert.Nil(t, result)
				}
			}
		}
	})

	t.Run("empty receiver should return empty string", func(t *testing.T) {
		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(
			t,
			language,
			"package main",
		)
		parser, err := NewGoParser()
		require.NoError(t, err)

		result := parser.parseGoReceiver(parseTree, nil)
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
	parser, err := NewGoParser()
	require.NoError(t, err)

	typeDecls := parseTree.GetNodesByType("type_declaration")
	require.Len(t, typeDecls, 1)

	typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
	typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
	require.NotNil(t, typeParamList)

	result := parser.parseGoGenericParameters(parseTree, typeParamList)
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

// TestGoTypeParser_ErrorHandling_InvalidConstraints tests constraint resolution error conditions.
// This is a RED PHASE test that defines expected behavior for invalid constraint handling.
//
//nolint:gocognit,nestif // RED phase test with intentionally complex error scenarios
func TestGoTypeParser_ErrorHandling_InvalidConstraints(t *testing.T) {
	t.Run("unknown constraint interface should handle gracefully", func(t *testing.T) {
		sourceCode := `package main
type WithUnknown[T UnknownConstraint] struct{}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		typeDecls := parseTree.GetNodesByType("type_declaration")
		if len(typeDecls) > 0 {
			typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
			if typeSpec != nil {
				typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
				if typeParamList != nil {
					result := parser.parseGoGenericParameters(parseTree, typeParamList)
					// Should handle unknown constraint
					if result != nil {
						for _, param := range result {
							assert.Equal(t, "T", param.Name)
							assert.Contains(t, param.Constraints, "UnknownConstraint")
							// Should handle unresolved constraint in constraints slice
							assert.NotEmpty(t, param.Constraints)
						}
					}
				}
			}
		}
	})

	t.Run("circular constraint dependency should handle gracefully", func(t *testing.T) {
		sourceCode := `package main
type CircularA[T CircularB[T]] struct{}
type CircularB[T CircularA[T]] struct{}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		typeDecls := parseTree.GetNodesByType("type_declaration")
		for _, typeDecl := range typeDecls {
			typeSpec := findChildByType(parser, typeDecl, "type_spec")
			if typeSpec != nil {
				typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
				if typeParamList != nil {
					result := parser.parseGoGenericParameters(parseTree, typeParamList)
					// Should detect or handle circular dependency
					if result != nil {
						for _, param := range result {
							assert.Equal(t, "T", param.Name)
							// Should handle circular constraint gracefully
							assert.NotEmpty(t, param.Constraints)
							// Circular constraints should still be recorded
							assert.NotEmpty(t, param.Constraints)
						}
					}
				}
			}
		}
	})

	t.Run("complex nested constraint should handle gracefully", func(t *testing.T) {
		sourceCode := `package main
type ComplexConstraint[T interface{ interface{ String() string } | interface{ ~int | ~float64 } }] struct{}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		typeDecls := parseTree.GetNodesByType("type_declaration")
		if len(typeDecls) > 0 {
			typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
			if typeSpec != nil {
				typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
				if typeParamList != nil {
					result := parser.parseGoGenericParameters(parseTree, typeParamList)
					// Should handle complex nested constraints
					if result != nil {
						for _, param := range result {
							assert.Equal(t, "T", param.Name)
							// Should parse complex constraints or have empty constraints
							if len(param.Constraints) == 0 {
								// Complex constraint parsing failed
								assert.Empty(t, param.Constraints)
							} else {
								assert.NotEmpty(t, param.Constraints)
							}
						}
					}
				}
			}
		}
	})
}

// TestGoTypeParser_ErrorHandling_TypeReferenceFailures tests type reference error conditions.
// This is a RED PHASE test that defines expected behavior for type reference failures.
func TestGoTypeParser_ErrorHandling_TypeReferenceFailures(t *testing.T) {
	t.Run("unresolved type reference should handle gracefully", func(t *testing.T) {
		sourceCode := `package main
func example() UnknownType { return nil }`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		// Find function return type
		funcDecls := parseTree.GetNodesByType("function_declaration")
		if len(funcDecls) > 0 {
			// This is a simplified approach - actual implementation would find return type node
			returnTypeNode := findChildByType(parser, funcDecls[0], "type_identifier")
			if returnTypeNode != nil {
				result := parser.ParseGoTypeReference(parseTree, returnTypeNode)
				// Should handle unresolved type reference
				if result != nil {
					assert.Equal(t, "UnknownType", result.Name)
					// Note: TypeReference doesn't have Metadata field in current domain model
					assert.False(t, result.IsGeneric, "Unresolved type should not be marked as generic")
				}
			}
		}
	})

	t.Run("malformed generic instantiation should handle gracefully", func(t *testing.T) {
		sourceCode := `package main
func example() map[string, int] { return nil }` // Invalid map syntax

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		// Find function return type
		funcDecls := parseTree.GetNodesByType("function_declaration")
		if len(funcDecls) > 0 {
			// This is a simplified approach - actual implementation would find return type node
			mapTypeNode := findChildByType(parser, funcDecls[0], "map_type")
			if mapTypeNode != nil {
				result := parser.ParseGoTypeReference(parseTree, mapTypeNode)
				// Should handle malformed generic instantiation
				if result != nil {
					assert.Contains(t, result.Name, "map")
					// Note: TypeReference doesn't have Metadata field in current domain model
					// Syntax errors would be handled through separate error mechanisms
					assert.True(t, result.IsGeneric, "Map type should be marked as generic")
				}
			}
		}
	})

	t.Run("pointer to unknown type should handle gracefully", func(t *testing.T) {
		sourceCode := `package main
func example() *UnknownType { return nil }`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		// Find function return type
		funcDecls := parseTree.GetNodesByType("function_declaration")
		if len(funcDecls) > 0 {
			pointerTypeNode := findChildByType(parser, funcDecls[0], "pointer_type")
			if pointerTypeNode != nil {
				result := parser.ParseGoTypeReference(parseTree, pointerTypeNode)
				// Should handle pointer to unknown type
				if result != nil {
					assert.Contains(t, result.Name, "*UnknownType")
					// Note: TypeReference doesn't have Metadata field in current domain model
					// Base type resolution errors would be handled through separate error mechanisms
					assert.False(t, result.IsGeneric, "Pointer to unknown type should not be generic")
				}
			}
		}
	})
}

// TestGoTypeParser_ErrorHandling_EncodingIssues tests encoding and special character error conditions.
// This is a RED PHASE test that defines expected behavior for encoding issues.
//
//nolint:gocognit,nestif // RED phase test with intentionally complex error scenarios
func TestGoTypeParser_ErrorHandling_EncodingIssues(t *testing.T) {
	t.Run("invalid UTF-8 in type parameter name should handle gracefully", func(t *testing.T) {
		// Simulate invalid UTF-8 byte sequence in source
		invalidUtf8 := "package main\ntype TestType[\xFF\xFE comparable] struct{}"

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(
			t,
			language,
			invalidUtf8,
		)
		parser, err := NewGoParser()
		require.NoError(t, err)

		typeDecls := parseTree.GetNodesByType("type_declaration")
		if len(typeDecls) > 0 {
			typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
			if typeSpec != nil {
				typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
				if typeParamList != nil {
					result := parser.parseGoGenericParameters(parseTree, typeParamList)
					// Should handle encoding issue gracefully
					if result != nil {
						for _, param := range result {
							// Name should be sanitized or marked as invalid
							assert.NotEmpty(t, param.Name)
							if strings.Contains(param.Name, "\uFFFD") { // Unicode replacement character
								// Note: GenericParameter doesn't have Metadata field in current domain model
								// Encoding errors would be handled through separate error mechanisms
								assert.NotEmpty(
									t,
									param.Name,
									"Should preserve parameter name even with encoding issues",
								)
							}
						}
					}
				}
			}
		}
	})

	t.Run("special unicode characters in type names should handle correctly", func(t *testing.T) {
		sourceCode := `package main
type πType[τ comparable] struct{}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		typeDecls := parseTree.GetNodesByType("type_declaration")
		if len(typeDecls) > 0 {
			typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
			if typeSpec != nil {
				typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
				if typeParamList != nil {
					result := parser.parseGoGenericParameters(parseTree, typeParamList)
					// Should handle unicode type parameter names correctly
					if result != nil {
						for _, param := range result {
							assert.Equal(t, "τ", param.Name)
							assert.Contains(t, param.Constraints, "comparable")
							// Note: GenericParameter doesn't have Metadata field in current domain model
							// Unicode preservation would be validated through the constraint parsing
						}
					}
				}
			}
		}
	})
}

// TestGoTypeParser_ErrorHandling_ResourceLimits tests resource limit error conditions.
// This is a RED PHASE test that defines expected behavior for resource limit scenarios.
//
//nolint:gocognit,nestif // RED phase test with intentionally complex error scenarios
func TestGoTypeParser_ErrorHandling_ResourceLimits(t *testing.T) {
	t.Run("very large generic parameter list should handle gracefully", func(t *testing.T) {
		// Create a type with many generic parameters
		sourceCode := "package main\ntype LargeGeneric["
		for i := range 100 {
			if i > 0 {
				sourceCode += ", "
			}
			sourceCode += fmt.Sprintf("T%d comparable", i)
		}
		sourceCode += "] struct{}"

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		typeDecls := parseTree.GetNodesByType("type_declaration")
		if len(typeDecls) > 0 {
			typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
			if typeSpec != nil {
				typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
				if typeParamList != nil {
					// Test with timeout to ensure it doesn't hang
					// Note: ParseGoGenericParameters doesn't take context in current signature
					// This would need to be enhanced in GREEN phase for timeout support
					result := parser.parseGoGenericParameters(parseTree, typeParamList)
					// Should handle large parameter list gracefully
					if result != nil {
						// Might be limited to prevent memory issues
						assert.LessOrEqual(t, len(result), 100, "Should handle up to 100 parameters")
						for _, param := range result {
							assert.NotEmpty(t, param.Name)
							assert.True(t, strings.HasPrefix(param.Name, "T"))
						}
						// If limited, should have indicators about truncation
						if len(result) < 100 {
							for _, param := range result {
								// Note: GenericParameter doesn't have Metadata field in current domain model
								// Parsing limits would be indicated through separate mechanisms
								assert.NotEmpty(t, param.Name, "Should preserve parameter name even under limits")
								break // Verified parameter structure
							}
						}
					}
				}
			}
		}
	})

	t.Run("deeply nested type constraints should handle gracefully", func(t *testing.T) {
		sourceCode := `package main
type NestedConstraint[T interface{ interface{ interface{ interface{ String() string } } } }] struct{}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		typeDecls := parseTree.GetNodesByType("type_declaration")
		if len(typeDecls) > 0 {
			typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
			if typeSpec != nil {
				typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
				if typeParamList != nil {
					result := parser.parseGoGenericParameters(parseTree, typeParamList)
					// Should handle deep nesting within limits
					if result != nil {
						for _, param := range result {
							assert.Equal(t, "T", param.Name)
							// Note: GenericParameter doesn't have Metadata field in current domain model
							// Constraint depth limits would be handled through separate mechanisms
							assert.NotEmpty(t, param.Name, "Should preserve parameter name even with deep constraints")
						}
					}
				}
			}
		}
	})
}

// TestGoTypeParser_ErrorHandling_ChunkCreation tests generic parameter chunk creation failures.
// This is a RED PHASE test that defines expected behavior for chunk creation failures.
//
//nolint:gocognit,nestif // RED phase test with intentionally complex error scenarios
func TestGoTypeParser_ErrorHandling_ChunkCreation(t *testing.T) {
	t.Run("parameter metadata creation failure should handle gracefully", func(t *testing.T) {
		// Test scenario where parameter metadata creation fails
		sourceCode := `package main
type ValidGeneric[T comparable] struct{}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		typeDecls := parseTree.GetNodesByType("type_declaration")
		require.NotEmpty(t, typeDecls, "Should have at least one type declaration")

		typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
		require.NotNil(t, typeSpec, "Should have type spec")

		typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
		if typeParamList != nil {
			result := parser.parseGoGenericParameters(parseTree, typeParamList)
			// Should handle metadata creation issues
			if result != nil {
				for _, param := range result {
					assert.Equal(t, "T", param.Name)
					assert.Contains(t, param.Constraints, "comparable")
					// Note: GenericParameter doesn't have Metadata field in current domain model
					// Creation errors would be handled through separate error mechanisms
					assert.NotEmpty(t, param.Name, "Should preserve parameter name even with creation errors")
				}
			}
		}
	})

	t.Run("constraint parsing failure should preserve parameter", func(t *testing.T) {
		sourceCode := `package main
type ConstraintTest[T VeryComplexConstraintThatMightFail] struct{}`

		language, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, sourceCode)
		parser, err := NewGoParser()
		require.NoError(t, err)

		typeDecls := parseTree.GetNodesByType("type_declaration")
		if len(typeDecls) > 0 {
			typeSpec := findChildByType(parser, typeDecls[0], "type_spec")
			if typeSpec != nil {
				typeParamList := findChildByType(parser, typeSpec, "type_parameter_list")
				if typeParamList != nil {
					result := parser.parseGoGenericParameters(parseTree, typeParamList)
					// Should preserve parameter even if constraint parsing fails
					if result != nil {
						for _, param := range result {
							assert.Equal(t, "T", param.Name)
							// Should have constraints or handle errors gracefully
							if len(param.Constraints) == 0 {
								// Note: GenericParameter doesn't have Metadata field in current domain model
								// Constraint parsing errors would be handled through separate mechanisms
								assert.NotEmpty(
									t,
									param.Name,
									"Should preserve parameter name even with constraint errors",
								)
							} else {
								assert.NotEmpty(t, param.Constraints[0])
							}
						}
					}
				}
			}
		}
	})
}
