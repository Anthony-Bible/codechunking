package pythonparser

import (
	"codechunking/internal/adapter/outbound/treesitter/parsers/testhelpers"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RED PHASE: Parameter extraction edge cases
// These tests expose limitations of the current string-based parameter extraction approach
// (checking for '*' prefix using strings.HasPrefix) and should guide refactor to use node types.

// TestPythonParser_ExtractFunctions_ParametersWithDefaults tests parameter extraction with default values.
// This is a RED PHASE test that defines expected behavior for parameters with default values.
func TestPythonParser_ExtractFunctions_ParametersWithDefaults(t *testing.T) {
	sourceCode := `# params_defaults.py

def greet(name="World", count=1):
    """Function with default parameters."""
    pass

def configure(host="localhost", port=8080, debug=False):
    """Multiple defaults."""
    pass

def process(data, timeout=30.5, retries=3):
    """Mix of required and optional parameters."""
    pass
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo:      true,
		IncludeDocumentation: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Test greet function with string and int defaults
	greetFunc := testhelpers.FindChunkByName(functions, "greet")
	require.NotNil(t, greetFunc, "Should find 'greet' function")
	require.Len(t, greetFunc.Parameters, 2)

	assert.Equal(t, "name", greetFunc.Parameters[0].Name)
	assert.Equal(t, `"World"`, greetFunc.Parameters[0].DefaultValue, "Should extract string default value")
	assert.True(t, greetFunc.Parameters[0].IsOptional, "Parameter with default should be optional")

	assert.Equal(t, "count", greetFunc.Parameters[1].Name)
	assert.Equal(t, "1", greetFunc.Parameters[1].DefaultValue, "Should extract int default value")
	assert.True(t, greetFunc.Parameters[1].IsOptional)

	// Test configure function with multiple types of defaults
	configFunc := testhelpers.FindChunkByName(functions, "configure")
	require.NotNil(t, configFunc, "Should find 'configure' function")
	require.Len(t, configFunc.Parameters, 3)

	assert.Equal(t, "host", configFunc.Parameters[0].Name)
	assert.Equal(t, `"localhost"`, configFunc.Parameters[0].DefaultValue)

	assert.Equal(t, "port", configFunc.Parameters[1].Name)
	assert.Equal(t, "8080", configFunc.Parameters[1].DefaultValue)

	assert.Equal(t, "debug", configFunc.Parameters[2].Name)
	assert.Equal(t, "False", configFunc.Parameters[2].DefaultValue, "Should extract boolean default")

	// Test process function with mix of required and optional
	processFunc := testhelpers.FindChunkByName(functions, "process")
	require.NotNil(t, processFunc, "Should find 'process' function")
	require.Len(t, processFunc.Parameters, 3)

	assert.Equal(t, "data", processFunc.Parameters[0].Name)
	assert.Empty(t, processFunc.Parameters[0].DefaultValue, "Required parameter should have no default")
	assert.False(t, processFunc.Parameters[0].IsOptional, "Required parameter should not be optional")

	assert.Equal(t, "timeout", processFunc.Parameters[1].Name)
	assert.Equal(t, "30.5", processFunc.Parameters[1].DefaultValue, "Should extract float default")
	assert.True(t, processFunc.Parameters[1].IsOptional)

	assert.Equal(t, "retries", processFunc.Parameters[2].Name)
	assert.Equal(t, "3", processFunc.Parameters[2].DefaultValue)
	assert.True(t, processFunc.Parameters[2].IsOptional)
}

// TestPythonParser_ExtractFunctions_ComplexDefaults tests complex default values.
// This is a RED PHASE test for parameters with None, empty collections, and complex expressions.
func TestPythonParser_ExtractFunctions_ComplexDefaults(t *testing.T) {
	sourceCode := `# complex_defaults.py

def fetch(url, headers=None, timeout=None):
    """Parameters with None defaults."""
    pass

def aggregate(items, result=[]):
    """Mutable default (anti-pattern but common)."""
    pass

def setup(config={}, options={}):
    """Dict defaults."""
    pass

def compute(x, factor=2 * 3):
    """Expression as default."""
    pass
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Test None defaults
	fetchFunc := testhelpers.FindChunkByName(functions, "fetch")
	require.NotNil(t, fetchFunc, "Should find 'fetch' function")
	require.Len(t, fetchFunc.Parameters, 3)

	assert.Equal(t, "headers", fetchFunc.Parameters[1].Name)
	assert.Equal(t, "None", fetchFunc.Parameters[1].DefaultValue, "Should extract None default")
	assert.True(t, fetchFunc.Parameters[1].IsOptional)

	assert.Equal(t, "timeout", fetchFunc.Parameters[2].Name)
	assert.Equal(t, "None", fetchFunc.Parameters[2].DefaultValue)

	// Test list default
	aggFunc := testhelpers.FindChunkByName(functions, "aggregate")
	require.NotNil(t, aggFunc, "Should find 'aggregate' function")
	require.Len(t, aggFunc.Parameters, 2)

	assert.Equal(t, "result", aggFunc.Parameters[1].Name)
	assert.Equal(t, "[]", aggFunc.Parameters[1].DefaultValue, "Should extract empty list default")

	// Test dict defaults
	setupFunc := testhelpers.FindChunkByName(functions, "setup")
	require.NotNil(t, setupFunc, "Should find 'setup' function")
	require.Len(t, setupFunc.Parameters, 2)

	assert.Equal(t, "config", setupFunc.Parameters[0].Name)
	assert.Equal(t, "{}", setupFunc.Parameters[0].DefaultValue, "Should extract empty dict default")

	assert.Equal(t, "options", setupFunc.Parameters[1].Name)
	assert.Equal(t, "{}", setupFunc.Parameters[1].DefaultValue)

	// Test expression default
	computeFunc := testhelpers.FindChunkByName(functions, "compute")
	require.NotNil(t, computeFunc, "Should find 'compute' function")
	require.Len(t, computeFunc.Parameters, 2)

	assert.Equal(t, "factor", computeFunc.Parameters[1].Name)
	// Should extract the entire expression, not evaluate it
	assert.Contains(t, computeFunc.Parameters[1].DefaultValue, "2", "Should preserve expression as default")
	assert.Contains(t, computeFunc.Parameters[1].DefaultValue, "3")
}

// TestPythonParser_ExtractFunctions_ComplexTypeHints tests complex type annotations.
// This is a RED PHASE test for generic types, unions, and callables.
func TestPythonParser_ExtractFunctions_ComplexTypeHints(t *testing.T) {
	sourceCode := `# complex_types.py

from typing import List, Dict, Optional, Union, Callable, Tuple

def process_items(items: List[str]) -> List[str]:
    """Generic list type."""
    pass

def transform_mapping(data: Dict[str, int]) -> Dict[str, int]:
    """Generic dict type."""
    pass

def optional_param(value: Optional[str] = None) -> str:
    """Optional type with default."""
    pass

def union_param(data: Union[str, int, None]) -> Union[str, int]:
    """Union types."""
    pass

def callback_param(handler: Callable[[int, str], bool]) -> None:
    """Callable type."""
    pass

def tuple_param(coords: Tuple[float, float, float]) -> float:
    """Tuple type with specific elements."""
    pass

def nested_generic(data: Dict[str, List[int]]) -> None:
    """Nested generic types."""
    pass
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Test generic list type
	processFunc := testhelpers.FindChunkByName(functions, "process_items")
	require.NotNil(t, processFunc, "Should find 'process_items' function")
	require.Len(t, processFunc.Parameters, 1)

	assert.Equal(t, "items", processFunc.Parameters[0].Name)
	assert.Equal(t, "List[str]", processFunc.Parameters[0].Type, "Should extract full generic type")
	assert.Equal(t, "List[str]", processFunc.ReturnType, "Should extract generic return type")

	// Test generic dict type
	transformFunc := testhelpers.FindChunkByName(functions, "transform_mapping")
	require.NotNil(t, transformFunc, "Should find 'transform_mapping' function")
	require.Len(t, transformFunc.Parameters, 1)

	assert.Equal(t, "data", transformFunc.Parameters[0].Name)
	assert.Equal(
		t,
		"Dict[str, int]",
		transformFunc.Parameters[0].Type,
		"Should extract dict generic with key/value types",
	)

	// Test Optional type with default
	optionalFunc := testhelpers.FindChunkByName(functions, "optional_param")
	require.NotNil(t, optionalFunc, "Should find 'optional_param' function")
	require.Len(t, optionalFunc.Parameters, 1)

	assert.Equal(t, "value", optionalFunc.Parameters[0].Name)
	assert.Equal(t, "Optional[str]", optionalFunc.Parameters[0].Type, "Should extract Optional type")
	assert.Equal(t, "None", optionalFunc.Parameters[0].DefaultValue)
	assert.True(t, optionalFunc.Parameters[0].IsOptional)

	// Test Union type
	unionFunc := testhelpers.FindChunkByName(functions, "union_param")
	require.NotNil(t, unionFunc, "Should find 'union_param' function")
	require.Len(t, unionFunc.Parameters, 1)

	assert.Equal(t, "data", unionFunc.Parameters[0].Name)
	assert.Equal(t, "Union[str, int, None]", unionFunc.Parameters[0].Type, "Should extract full Union type")

	// Test Callable type
	callbackFunc := testhelpers.FindChunkByName(functions, "callback_param")
	require.NotNil(t, callbackFunc, "Should find 'callback_param' function")
	require.Len(t, callbackFunc.Parameters, 1)

	assert.Equal(t, "handler", callbackFunc.Parameters[0].Name)
	assert.Equal(t, "Callable[[int, str], bool]", callbackFunc.Parameters[0].Type, "Should extract Callable signature")

	// Test Tuple type
	tupleFunc := testhelpers.FindChunkByName(functions, "tuple_param")
	require.NotNil(t, tupleFunc, "Should find 'tuple_param' function")
	require.Len(t, tupleFunc.Parameters, 1)

	assert.Equal(t, "coords", tupleFunc.Parameters[0].Name)
	assert.Equal(
		t,
		"Tuple[float, float, float]",
		tupleFunc.Parameters[0].Type,
		"Should extract Tuple with element types",
	)

	// Test nested generic
	nestedFunc := testhelpers.FindChunkByName(functions, "nested_generic")
	require.NotNil(t, nestedFunc, "Should find 'nested_generic' function")
	require.Len(t, nestedFunc.Parameters, 1)

	assert.Equal(t, "data", nestedFunc.Parameters[0].Name)
	assert.Equal(t, "Dict[str, List[int]]", nestedFunc.Parameters[0].Type, "Should extract nested generic types")
}

// TestPythonParser_ExtractFunctions_KeywordOnlyParameters tests keyword-only parameters.
// This is a RED PHASE test for parameters after * separator (Python 3+).
// TODO: Requires new field 'IsKeywordOnly' in Parameter struct
func TestPythonParser_ExtractFunctions_KeywordOnlyParameters(t *testing.T) {
	sourceCode := `# keyword_only.py

def connect(host, port, *, ssl=True, verify=True):
    """Parameters after * are keyword-only."""
    pass

def create_user(username, *, email, password, is_admin=False):
    """Mix of keyword-only required and optional."""
    pass

def process(data, *, verbose):
    """Keyword-only without default (required at call site)."""
    pass

def configure(*, timeout=30, retries=3):
    """All keyword-only parameters."""
    pass
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Test basic keyword-only with defaults
	connectFunc := testhelpers.FindChunkByName(functions, "connect")
	require.NotNil(t, connectFunc, "Should find 'connect' function")
	require.Len(t, connectFunc.Parameters, 4)

	assert.Equal(t, "host", connectFunc.Parameters[0].Name)
	assert.False(t, connectFunc.Parameters[0].IsOptional, "Regular param should not be optional")
	// TODO: assert.False(t, connectFunc.Parameters[0].IsKeywordOnly, "Param before * should not be keyword-only")

	assert.Equal(t, "port", connectFunc.Parameters[1].Name)
	// TODO: assert.False(t, connectFunc.Parameters[1].IsKeywordOnly)

	assert.Equal(t, "ssl", connectFunc.Parameters[2].Name)
	assert.Equal(t, "True", connectFunc.Parameters[2].DefaultValue)
	assert.True(t, connectFunc.Parameters[2].IsOptional)
	// TODO: assert.True(t, connectFunc.Parameters[2].IsKeywordOnly, "Param after * should be keyword-only")

	assert.Equal(t, "verify", connectFunc.Parameters[3].Name)
	assert.Equal(t, "True", connectFunc.Parameters[3].DefaultValue)
	// TODO: assert.True(t, connectFunc.Parameters[3].IsKeywordOnly)

	// Test mix of required and optional keyword-only
	createUserFunc := testhelpers.FindChunkByName(functions, "create_user")
	require.NotNil(t, createUserFunc, "Should find 'create_user' function")
	require.Len(t, createUserFunc.Parameters, 4)

	assert.Equal(t, "username", createUserFunc.Parameters[0].Name)
	// TODO: assert.False(t, createUserFunc.Parameters[0].IsKeywordOnly)

	assert.Equal(t, "email", createUserFunc.Parameters[1].Name)
	assert.Empty(t, createUserFunc.Parameters[1].DefaultValue, "Keyword-only can be required")
	assert.False(t, createUserFunc.Parameters[1].IsOptional, "No default = not optional")
	// TODO: assert.True(t, createUserFunc.Parameters[1].IsKeywordOnly, "After * = keyword-only")

	assert.Equal(t, "password", createUserFunc.Parameters[2].Name)
	// TODO: assert.True(t, createUserFunc.Parameters[2].IsKeywordOnly)

	assert.Equal(t, "is_admin", createUserFunc.Parameters[3].Name)
	assert.Equal(t, "False", createUserFunc.Parameters[3].DefaultValue)
	// TODO: assert.True(t, createUserFunc.Parameters[3].IsKeywordOnly)

	// Test keyword-only required parameter
	processFunc := testhelpers.FindChunkByName(functions, "process")
	require.NotNil(t, processFunc, "Should find 'process' function")
	require.Len(t, processFunc.Parameters, 2)

	assert.Equal(t, "verbose", processFunc.Parameters[1].Name)
	assert.Empty(t, processFunc.Parameters[1].DefaultValue)
	// TODO: assert.True(t, processFunc.Parameters[1].IsKeywordOnly)

	// Test all keyword-only
	configureFunc := testhelpers.FindChunkByName(functions, "configure")
	require.NotNil(t, configureFunc, "Should find 'configure' function")
	require.Len(t, configureFunc.Parameters, 2)

	assert.Equal(t, "timeout", configureFunc.Parameters[0].Name)
	// TODO: assert.True(t, configureFunc.Parameters[0].IsKeywordOnly, "All params should be keyword-only")

	assert.Equal(t, "retries", configureFunc.Parameters[1].Name)
	// TODO: assert.True(t, configureFunc.Parameters[1].IsKeywordOnly)
}

// TestPythonParser_ExtractFunctions_PositionalOnlyParameters tests positional-only parameters.
// This is a RED PHASE test for parameters before / separator (Python 3.8+).
// TODO: Requires new field 'IsPositionalOnly' in Parameter struct
func TestPythonParser_ExtractFunctions_PositionalOnlyParameters(t *testing.T) {
	sourceCode := `# positional_only.py

def divide(numerator, denominator, /):
    """Parameters before / are positional-only."""
    pass

def power(base, exponent, /, modulo=None):
    """Mix of positional-only and optional."""
    pass

def connect(host, port, /, timeout=30):
    """Positional-only required + optional after."""
    pass

def api_call(endpoint, /, *, headers=None):
    """Positional-only before /, keyword-only after *."""
    pass
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Test basic positional-only
	divideFunc := testhelpers.FindChunkByName(functions, "divide")
	require.NotNil(t, divideFunc, "Should find 'divide' function")
	require.Len(t, divideFunc.Parameters, 2)

	assert.Equal(t, "numerator", divideFunc.Parameters[0].Name)
	// TODO: assert.True(t, divideFunc.Parameters[0].IsPositionalOnly, "Param before / should be positional-only")

	assert.Equal(t, "denominator", divideFunc.Parameters[1].Name)
	// TODO: assert.True(t, divideFunc.Parameters[1].IsPositionalOnly)

	// Test positional-only with optional after
	powerFunc := testhelpers.FindChunkByName(functions, "power")
	require.NotNil(t, powerFunc, "Should find 'power' function")
	require.Len(t, powerFunc.Parameters, 3)

	assert.Equal(t, "base", powerFunc.Parameters[0].Name)
	// TODO: assert.True(t, powerFunc.Parameters[0].IsPositionalOnly)

	assert.Equal(t, "exponent", powerFunc.Parameters[1].Name)
	// TODO: assert.True(t, powerFunc.Parameters[1].IsPositionalOnly)

	assert.Equal(t, "modulo", powerFunc.Parameters[2].Name)
	assert.Equal(t, "None", powerFunc.Parameters[2].DefaultValue)
	assert.True(t, powerFunc.Parameters[2].IsOptional)
	// TODO: assert.False(t, powerFunc.Parameters[2].IsPositionalOnly, "Param after / is not positional-only")

	// Test positional-only + keyword-only in same function
	apiCallFunc := testhelpers.FindChunkByName(functions, "api_call")
	require.NotNil(t, apiCallFunc, "Should find 'api_call' function")
	require.Len(t, apiCallFunc.Parameters, 2)

	assert.Equal(t, "endpoint", apiCallFunc.Parameters[0].Name)
	// TODO: assert.True(t, apiCallFunc.Parameters[0].IsPositionalOnly, "Before / = positional-only")
	// TODO: assert.False(t, apiCallFunc.Parameters[0].IsKeywordOnly)

	assert.Equal(t, "headers", apiCallFunc.Parameters[1].Name)
	// TODO: assert.False(t, apiCallFunc.Parameters[1].IsPositionalOnly)
	// TODO: assert.True(t, apiCallFunc.Parameters[1].IsKeywordOnly, "After * = keyword-only")
}

// TestPythonParser_ExtractFunctions_MixedParameterTypes tests all parameter types together.
// This is a RED PHASE test for the most complex parameter signatures.
func TestPythonParser_ExtractFunctions_MixedParameterTypes(t *testing.T) {
	sourceCode := `# mixed_params.py

def complex_function(pos_only, /, normal, default="value", *args, kw_only, kw_default=42, **kwargs):
    """All parameter types in one function."""
    pass

def typed_complex(
    pos_only: int,
    /,
    normal: str,
    default: Optional[str] = None,
    *args: int,
    kw_only: bool,
    kw_default: float = 3.14,
    **kwargs: Any
) -> Dict[str, Any]:
    """Fully typed complex signature."""
    pass
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Test untyped complex function
	complexFunc := testhelpers.FindChunkByName(functions, "complex_function")
	require.NotNil(t, complexFunc, "Should find 'complex_function' function")
	require.Len(t, complexFunc.Parameters, 7)

	// pos_only
	assert.Equal(t, "pos_only", complexFunc.Parameters[0].Name)
	assert.False(t, complexFunc.Parameters[0].IsOptional)
	assert.False(t, complexFunc.Parameters[0].IsVariadic)
	// TODO: assert.True(t, complexFunc.Parameters[0].IsPositionalOnly)
	// TODO: assert.False(t, complexFunc.Parameters[0].IsKeywordOnly)

	// normal
	assert.Equal(t, "normal", complexFunc.Parameters[1].Name)
	assert.False(t, complexFunc.Parameters[1].IsOptional)
	// TODO: assert.False(t, complexFunc.Parameters[1].IsPositionalOnly)
	// TODO: assert.False(t, complexFunc.Parameters[1].IsKeywordOnly)

	// default
	assert.Equal(t, "default", complexFunc.Parameters[2].Name)
	assert.Equal(t, `"value"`, complexFunc.Parameters[2].DefaultValue)
	assert.True(t, complexFunc.Parameters[2].IsOptional)
	// TODO: assert.False(t, complexFunc.Parameters[2].IsKeywordOnly)

	// *args
	assert.Equal(t, "args", complexFunc.Parameters[3].Name)
	assert.True(t, complexFunc.Parameters[3].IsVariadic, "Should detect *args as variadic")
	// TODO: assert.False(t, complexFunc.Parameters[3].IsKeywordOnly)

	// kw_only
	assert.Equal(t, "kw_only", complexFunc.Parameters[4].Name)
	assert.False(t, complexFunc.Parameters[4].IsOptional, "No default = required")
	assert.False(t, complexFunc.Parameters[4].IsVariadic)
	// TODO: assert.True(t, complexFunc.Parameters[4].IsKeywordOnly, "After *args = keyword-only")

	// kw_default
	assert.Equal(t, "kw_default", complexFunc.Parameters[5].Name)
	assert.Equal(t, "42", complexFunc.Parameters[5].DefaultValue)
	assert.True(t, complexFunc.Parameters[5].IsOptional)
	// TODO: assert.True(t, complexFunc.Parameters[5].IsKeywordOnly)

	// **kwargs
	assert.Equal(t, "kwargs", complexFunc.Parameters[6].Name)
	assert.True(t, complexFunc.Parameters[6].IsVariadic, "Should detect **kwargs as variadic")

	// Test fully typed complex function
	typedComplexFunc := testhelpers.FindChunkByName(functions, "typed_complex")
	require.NotNil(t, typedComplexFunc, "Should find 'typed_complex' function")
	require.Len(t, typedComplexFunc.Parameters, 7)

	// Verify types are extracted for all parameters
	assert.Equal(t, "pos_only", typedComplexFunc.Parameters[0].Name)
	assert.Equal(t, "int", typedComplexFunc.Parameters[0].Type, "Should extract type for positional-only")

	assert.Equal(t, "normal", typedComplexFunc.Parameters[1].Name)
	assert.Equal(t, "str", typedComplexFunc.Parameters[1].Type)

	assert.Equal(t, "default", typedComplexFunc.Parameters[2].Name)
	assert.Equal(t, "Optional[str]", typedComplexFunc.Parameters[2].Type, "Should extract Optional type")
	assert.Equal(t, "None", typedComplexFunc.Parameters[2].DefaultValue)

	assert.Equal(t, "args", typedComplexFunc.Parameters[3].Name)
	assert.Equal(t, "int", typedComplexFunc.Parameters[3].Type, "Should extract type for *args")
	assert.True(t, typedComplexFunc.Parameters[3].IsVariadic)

	assert.Equal(t, "kw_only", typedComplexFunc.Parameters[4].Name)
	assert.Equal(t, "bool", typedComplexFunc.Parameters[4].Type)

	assert.Equal(t, "kw_default", typedComplexFunc.Parameters[5].Name)
	assert.Equal(t, "float", typedComplexFunc.Parameters[5].Type)
	assert.Equal(t, "3.14", typedComplexFunc.Parameters[5].DefaultValue)

	assert.Equal(t, "kwargs", typedComplexFunc.Parameters[6].Name)
	assert.Equal(t, "Any", typedComplexFunc.Parameters[6].Type, "Should extract type for **kwargs")
	assert.True(t, typedComplexFunc.Parameters[6].IsVariadic)

	// Check return type
	assert.Equal(t, "Dict[str, Any]", typedComplexFunc.ReturnType, "Should extract complex return type")
}

// TestPythonParser_ExtractFunctions_TypedDefaults tests parameters with both type and default.
// This is a RED PHASE test for typed parameters with default values.
func TestPythonParser_ExtractFunctions_TypedDefaults(t *testing.T) {
	sourceCode := `# typed_defaults.py

def greet(name: str = "World", count: int = 1) -> str:
    """Typed parameters with defaults."""
    pass

def configure(
    host: str = "localhost",
    port: int = 8080,
    timeout: float = 30.0,
    debug: bool = False
) -> None:
    """Multiple typed defaults."""
    pass

def fetch(url: str, headers: Optional[Dict[str, str]] = None) -> Response:
    """Complex type with default."""
    pass
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Test basic typed defaults
	greetFunc := testhelpers.FindChunkByName(functions, "greet")
	require.NotNil(t, greetFunc, "Should find 'greet' function")
	require.Len(t, greetFunc.Parameters, 2)

	assert.Equal(t, "name", greetFunc.Parameters[0].Name)
	assert.Equal(t, "str", greetFunc.Parameters[0].Type, "Should extract type")
	assert.Equal(t, `"World"`, greetFunc.Parameters[0].DefaultValue, "Should extract default")
	assert.True(t, greetFunc.Parameters[0].IsOptional)

	assert.Equal(t, "count", greetFunc.Parameters[1].Name)
	assert.Equal(t, "int", greetFunc.Parameters[1].Type)
	assert.Equal(t, "1", greetFunc.Parameters[1].DefaultValue)
	assert.True(t, greetFunc.Parameters[1].IsOptional)

	assert.Equal(t, "str", greetFunc.ReturnType)

	// Test multiple typed defaults
	configureFunc := testhelpers.FindChunkByName(functions, "configure")
	require.NotNil(t, configureFunc, "Should find 'configure' function")
	require.Len(t, configureFunc.Parameters, 4)

	assert.Equal(t, "host", configureFunc.Parameters[0].Name)
	assert.Equal(t, "str", configureFunc.Parameters[0].Type)
	assert.Equal(t, `"localhost"`, configureFunc.Parameters[0].DefaultValue)

	assert.Equal(t, "port", configureFunc.Parameters[1].Name)
	assert.Equal(t, "int", configureFunc.Parameters[1].Type)
	assert.Equal(t, "8080", configureFunc.Parameters[1].DefaultValue)

	assert.Equal(t, "timeout", configureFunc.Parameters[2].Name)
	assert.Equal(t, "float", configureFunc.Parameters[2].Type)
	assert.Equal(t, "30.0", configureFunc.Parameters[2].DefaultValue)

	assert.Equal(t, "debug", configureFunc.Parameters[3].Name)
	assert.Equal(t, "bool", configureFunc.Parameters[3].Type)
	assert.Equal(t, "False", configureFunc.Parameters[3].DefaultValue)

	// Test complex type with default
	fetchFunc := testhelpers.FindChunkByName(functions, "fetch")
	require.NotNil(t, fetchFunc, "Should find 'fetch' function")
	require.Len(t, fetchFunc.Parameters, 2)

	assert.Equal(t, "url", fetchFunc.Parameters[0].Name)
	assert.Equal(t, "str", fetchFunc.Parameters[0].Type)
	assert.Empty(t, fetchFunc.Parameters[0].DefaultValue)
	assert.False(t, fetchFunc.Parameters[0].IsOptional)

	assert.Equal(t, "headers", fetchFunc.Parameters[1].Name)
	assert.Equal(t, "Optional[Dict[str, str]]", fetchFunc.Parameters[1].Type, "Should extract nested generic type")
	assert.Equal(t, "None", fetchFunc.Parameters[1].DefaultValue)
	assert.True(t, fetchFunc.Parameters[1].IsOptional)
}
