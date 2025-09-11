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

// TestPythonParser_ExtractFunctions_GeneratorFunctions tests extraction of Python generator functions.
// This test covers generator functions with yield statements.
func TestPythonParser_ExtractFunctions_GeneratorFunctions(t *testing.T) {
	sourceCode := `def fibonacci_generator(n):
    """Generate first n fibonacci numbers."""
    a, b = 0, 1
    count = 0
    while count < n:
        yield a
        a, b = b, a + b
        count += 1

def simple_generator():
    yield "hello"
    yield "world"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find 2 generator functions
	require.Len(t, functions, 2)

	// Test fibonacci generator
	fibGen := testhelpers.FindChunkByName(functions, "fibonacci_generator")
	require.NotNil(t, fibGen, "Should find 'fibonacci_generator' function")
	assert.Equal(t, outbound.ConstructFunction, fibGen.Type)
	assert.Equal(t, outbound.Public, fibGen.Visibility)
	assert.Equal(t, "Generate first n fibonacci numbers.", fibGen.Documentation)

	// Test simple generator
	simpleGen := testhelpers.FindChunkByName(functions, "simple_generator")
	require.NotNil(t, simpleGen, "Should find 'simple_generator' function")
	assert.Equal(t, outbound.ConstructFunction, simpleGen.Type)
	assert.Equal(t, outbound.Public, simpleGen.Visibility)
}

// TestPythonParser_ExtractFunctions_DecoratedFunctions tests extraction of decorated Python functions.
// This test covers functions with single and multiple decorators.
func TestPythonParser_ExtractFunctions_DecoratedFunctions(t *testing.T) {
	sourceCode := `@cache
def expensive_calculation(n):
    """Cached expensive calculation."""
    return sum(range(n))

@property
@cached_property  
def computed_value(self):
    """Property with multiple decorators."""
    return self._compute()

@staticmethod
@deprecated("Use new_function instead")
def legacy_function():
    """Deprecated static function."""
    pass
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find 3 decorated functions
	require.Len(t, functions, 3)

	// Test single decorator
	expensiveFunc := testhelpers.FindChunkByName(functions, "expensive_calculation")
	require.NotNil(t, expensiveFunc, "Should find 'expensive_calculation' function")
	assert.Equal(t, outbound.ConstructFunction, expensiveFunc.Type)
	assert.Equal(t, "Cached expensive calculation.", expensiveFunc.Documentation)
	if len(expensiveFunc.Annotations) > 0 {
		assert.Equal(t, "cache", expensiveFunc.Annotations[0].Name)
	}

	// Test multiple decorators
	computedFunc := testhelpers.FindChunkByName(functions, "computed_value")
	require.NotNil(t, computedFunc, "Should find 'computed_value' function")
	assert.Equal(t, outbound.ConstructFunction, computedFunc.Type)
	assert.Equal(t, "Property with multiple decorators.", computedFunc.Documentation)

	// Test deprecated function
	legacyFunc := testhelpers.FindChunkByName(functions, "legacy_function")
	require.NotNil(t, legacyFunc, "Should find 'legacy_function' function")
	assert.Equal(t, outbound.ConstructFunction, legacyFunc.Type)
	assert.Equal(t, "Deprecated static function.", legacyFunc.Documentation)
}

// TestPythonParser_ExtractFunctions_NestedFunctions tests extraction of nested Python functions.
// This test covers functions defined inside other functions.
func TestPythonParser_ExtractFunctions_NestedFunctions(t *testing.T) {
	sourceCode := `def outer_function(x):
    """Function with nested helper functions."""
    
    def inner_helper(y):
        """Inner helper function."""
        return y * 2
        
    def another_inner(z):
        return z + 1
        
    result = inner_helper(x)
    return another_inner(result)

def closure_example():
    """Function demonstrating closure."""
    counter = 0
    
    def increment():
        nonlocal counter
        counter += 1
        return counter
        
    return increment
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find outer functions and potentially nested functions depending on implementation
	require.GreaterOrEqual(t, len(functions), 2, "Should find at least outer functions")

	// Test outer function
	outerFunc := testhelpers.FindChunkByName(functions, "outer_function")
	require.NotNil(t, outerFunc, "Should find 'outer_function' function")
	assert.Equal(t, outbound.ConstructFunction, outerFunc.Type)
	assert.Equal(t, "Function with nested helper functions.", outerFunc.Documentation)

	// Test closure function
	closureFunc := testhelpers.FindChunkByName(functions, "closure_example")
	require.NotNil(t, closureFunc, "Should find 'closure_example' function")
	assert.Equal(t, outbound.ConstructFunction, closureFunc.Type)
	assert.Equal(t, "Function demonstrating closure.", closureFunc.Documentation)
}

// TestPythonParser_ExtractFunctions_TypeHintedFunctions tests extraction of Python functions with comprehensive type hints.
// This test covers modern Python type annotation features.
func TestPythonParser_ExtractFunctions_TypeHintedFunctions(t *testing.T) {
	sourceCode := `from typing import List, Dict, Optional, Union

def process_data(
    items: List[str], 
    config: Dict[str, int],
    optional_param: Optional[bool] = None
) -> Union[str, None]:
    """Process data with comprehensive type hints."""
    if not items:
        return None
    return f"Processed {len(items)} items"

def generic_function[T](data: T) -> T:
    """Generic function with type parameter."""
    return data

async def async_typed_function(url: str, timeout: float = 5.0) -> Dict[str, any]:
    """Async function with type hints."""
    import aiohttp
    async with aiohttp.ClientSession(timeout=aiohttp.ClientTimeout(total=timeout)) as session:
        async with session.get(url) as response:
            return await response.json()
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find 3 functions
	require.Len(t, functions, 3)

	// Test complex type hints
	processFunc := testhelpers.FindChunkByName(functions, "process_data")
	require.NotNil(t, processFunc, "Should find 'process_data' function")
	assert.Equal(t, outbound.ConstructFunction, processFunc.Type)
	assert.Equal(t, "Process data with comprehensive type hints.", processFunc.Documentation)
	require.Len(t, processFunc.Parameters, 3)

	// Check parameter details
	assert.Equal(t, "items", processFunc.Parameters[0].Name)
	assert.Equal(t, "config", processFunc.Parameters[1].Name)
	assert.Equal(t, "optional_param", processFunc.Parameters[2].Name)
	assert.Equal(t, "None", processFunc.Parameters[2].DefaultValue)

	// Test generic function
	genericFunc := testhelpers.FindChunkByName(functions, "generic_function")
	require.NotNil(t, genericFunc, "Should find 'generic_function' function")
	assert.Equal(t, outbound.ConstructFunction, genericFunc.Type)
	assert.Equal(t, "Generic function with type parameter.", genericFunc.Documentation)

	// Test async typed function
	asyncFunc := testhelpers.FindChunkByName(functions, "async_typed_function")
	require.NotNil(t, asyncFunc, "Should find 'async_typed_function' function")
	assert.Equal(t, outbound.ConstructFunction, asyncFunc.Type)
	assert.Equal(t, "Async function with type hints.", asyncFunc.Documentation)
	assert.True(t, asyncFunc.IsAsync)
}

// TestPythonParser_ExtractFunctions_PrivateFunctions tests extraction of private Python functions.
// This test covers functions with underscore prefixes indicating privacy.
func TestPythonParser_ExtractFunctions_PrivateFunctions(t *testing.T) {
	sourceCode := `def public_function():
    """A public function."""
    return _private_helper()

def _private_function():
    """A private function with single underscore."""
    return __very_private_function()

def __very_private_function():
    """A very private function with double underscore."""
    return "secret"

def _internal_helper(data):
    """Internal helper function."""
    return data.strip()
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	// Test with private functions included
	optionsWithPrivate := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	functionsWithPrivate, err := parser.ExtractFunctions(context.Background(), parseTree, optionsWithPrivate)
	require.NoError(t, err)

	// Should find all 4 functions when including private
	require.Len(t, functionsWithPrivate, 4)

	// Test without private functions
	optionsNoPrivate := outbound.SemanticExtractionOptions{
		IncludePrivate:       false,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	functionsNoPrivate, err := parser.ExtractFunctions(context.Background(), parseTree, optionsNoPrivate)
	require.NoError(t, err)

	// Should find only 1 public function when excluding private
	require.Len(t, functionsNoPrivate, 1)

	// Verify public function
	publicFunc := testhelpers.FindChunkByName(functionsWithPrivate, "public_function")
	require.NotNil(t, publicFunc, "Should find 'public_function' function")
	assert.Equal(t, outbound.Public, publicFunc.Visibility)
	assert.Equal(t, "A public function.", publicFunc.Documentation)

	// Verify private function
	privateFunc := testhelpers.FindChunkByName(functionsWithPrivate, "_private_function")
	require.NotNil(t, privateFunc, "Should find '_private_function' function")
	assert.Equal(t, outbound.Private, privateFunc.Visibility)
	assert.Equal(t, "A private function with single underscore.", privateFunc.Documentation)

	// Verify very private function
	veryPrivateFunc := testhelpers.FindChunkByName(functionsWithPrivate, "__very_private_function")
	require.NotNil(t, veryPrivateFunc, "Should find '__very_private_function' function")
	assert.Equal(t, outbound.Private, veryPrivateFunc.Visibility)
	assert.Equal(t, "A very private function with double underscore.", veryPrivateFunc.Documentation)
}
