package javascriptparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJavaScriptParser_NewJavaScriptParser tests creation of JavaScript parser.
// This is a RED PHASE test that defines expected behavior for JavaScript parser creation.
func TestJavaScriptParser_NewJavaScriptParser(t *testing.T) {
	parser, err := NewJavaScriptParser()
	require.NoError(t, err, "Creating JavaScript parser should not fail")
	require.NotNil(t, parser, "JavaScript parser should not be nil")

	// Test supported language
	lang := parser.GetSupportedLanguage()
	assert.Equal(t, valueobject.LanguageJavaScript, lang.Name())
}

// TestJavaScriptParser_GetSupportedConstructTypes tests supported construct types.
// This is a RED PHASE test that defines expected JavaScript construct types.
func TestJavaScriptParser_GetSupportedConstructTypes(t *testing.T) {
	parser, err := NewJavaScriptParser()
	require.NoError(t, err)

	types := parser.GetSupportedConstructTypes()
	expectedTypes := []outbound.SemanticConstructType{
		outbound.ConstructFunction,
		outbound.ConstructMethod,
		outbound.ConstructClass,
		outbound.ConstructVariable,
		outbound.ConstructConstant,
		outbound.ConstructProperty,
		outbound.ConstructModule,
		outbound.ConstructNamespace,
		outbound.ConstructLambda,
		outbound.ConstructAsyncFunction,
		outbound.ConstructGenerator,
	}

	require.Len(t, types, len(expectedTypes))
	for _, expectedType := range expectedTypes {
		assert.Contains(t, types, expectedType)
	}
}

// TestJavaScriptParser_IsSupported tests language support checking.
// This is a RED PHASE test that defines expected JavaScript language support behavior.
func TestJavaScriptParser_IsSupported(t *testing.T) {
	parser, err := NewJavaScriptParser()
	require.NoError(t, err)

	// Should support JavaScript
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)
	assert.True(t, parser.IsSupported(jsLang))

	// Should not support TypeScript (handled by separate parser)
	tsLang, err := valueobject.NewLanguage(valueobject.LanguageTypeScript)
	require.NoError(t, err)
	assert.False(t, parser.IsSupported(tsLang))

	// Should not support Go
	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)
	assert.False(t, parser.IsSupported(goLang))
}

// TestJavaScriptParser_ExtractFunctions_RegularFunctions tests regular JavaScript function extraction.
// This is a RED PHASE test that defines expected behavior for JavaScript function extraction.
func TestJavaScriptParser_ExtractFunctions_RegularFunctions(t *testing.T) {
	sourceCode := `// math_utils.js

function add(a, b) {
    return a + b;
}

function multiply(x, y) {
    /**
     * Multiply two numbers
     * @param {number} x - First number
     * @param {number} y - Second number
     * @returns {number} The product
     */
    return x * y;
}

const subtract = function(a, b) {
    return a - b;
};

const divide = (x, y) => x / y;

const power = (base, exponent) => {
    return Math.pow(base, exponent);
};
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(
		t,
		language,
		sourceCode,
	)
	parser, err := NewJavaScriptParser()
	require.NoError(t, err)
	_ = parseTree // RED PHASE: will be used after implementation

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find 5 functions (add, multiply, subtract, divide, power)
	require.Len(t, functions, 5)

	// Test regular function declaration
	addFunc := findChunkByName(functions, "add")
	require.NotNil(t, addFunc, "Should find 'add' function")
	assert.Equal(t, outbound.ConstructFunction, addFunc.Type)
	assert.Equal(t, "add", addFunc.Name)
	assert.Equal(t, "math_utils.add", addFunc.QualifiedName)
	assert.False(t, addFunc.IsAsync)
	assert.False(t, addFunc.IsGeneric)
	assert.Len(t, addFunc.Parameters, 2)
	assert.Equal(t, "a", addFunc.Parameters[0].Name)
	assert.Equal(t, "b", addFunc.Parameters[1].Name)

	// Test function with JSDoc documentation
	multiplyFunc := findChunkByName(functions, "multiply")
	require.NotNil(t, multiplyFunc, "Should find 'multiply' function")
	assert.Contains(t, multiplyFunc.Documentation, "Multiply two numbers")
	assert.Contains(t, multiplyFunc.Documentation, "@param {number} x")
	assert.Contains(t, multiplyFunc.Documentation, "@returns {number}")

	// Test function expression
	subtractFunc := findChunkByName(functions, "subtract")
	require.NotNil(t, subtractFunc, "Should find 'subtract' function")
	assert.Equal(t, outbound.ConstructFunction, subtractFunc.Type)
	assert.Equal(t, "subtract", subtractFunc.Name)

	// Test arrow function (single expression)
	divideFunc := findChunkByName(functions, "divide")
	require.NotNil(t, divideFunc, "Should find 'divide' arrow function")
	assert.Equal(t, outbound.ConstructLambda, divideFunc.Type)

	// Test arrow function (block body)
	powerFunc := findChunkByName(functions, "power")
	require.NotNil(t, powerFunc, "Should find 'power' arrow function")
	assert.Equal(t, outbound.ConstructLambda, powerFunc.Type)
}

// TestJavaScriptParser_ExtractFunctions_AsyncFunctions tests async JavaScript function extraction.
// This is a RED PHASE test that defines expected behavior for async function extraction.
func TestJavaScriptParser_ExtractFunctions_AsyncFunctions(t *testing.T) {
	sourceCode := `// async_utils.js

async function fetchData(url) {
    const response = await fetch(url);
    return await response.json();
}

const asyncArrow = async (id) => {
    return await getUserById(id);
};

async function* asyncGenerator() {
    for (let i = 0; i < 10; i++) {
        yield await Promise.resolve(i);
    }
}

const asyncMethod = {
    async processData(data) {
        return await this.transform(data);
    }
};
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(
		t,
		language,
		sourceCode,
	)
	parser, err := NewJavaScriptParser()
	require.NoError(t, err)
	_ = parseTree // RED PHASE: will be used after implementation

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find 4 functions (fetchData, asyncArrow, asyncGenerator, processData)
	require.Len(t, functions, 4)

	// Test async function
	fetchFunc := findChunkByName(functions, "fetchData")
	require.NotNil(t, fetchFunc, "Should find 'fetchData' async function")
	assert.Equal(t, outbound.ConstructAsyncFunction, fetchFunc.Type)
	assert.True(t, fetchFunc.IsAsync)
	assert.Len(t, fetchFunc.Parameters, 1)
	assert.Equal(t, "url", fetchFunc.Parameters[0].Name)

	// Test async arrow function
	arrowFunc := findChunkByName(functions, "asyncArrow")
	require.NotNil(t, arrowFunc, "Should find 'asyncArrow' async function")
	assert.True(t, arrowFunc.IsAsync)
	assert.Equal(t, outbound.ConstructLambda, arrowFunc.Type)

	// Test async generator
	generatorFunc := findChunkByName(functions, "asyncGenerator")
	require.NotNil(t, generatorFunc, "Should find 'asyncGenerator' function")
	assert.Equal(t, outbound.ConstructGenerator, generatorFunc.Type)
	assert.True(t, generatorFunc.IsAsync)

	// Test async method in object
	methodFunc := findChunkByName(functions, "processData")
	require.NotNil(t, methodFunc, "Should find 'processData' method")
	assert.Equal(t, outbound.ConstructMethod, methodFunc.Type)
	assert.True(t, methodFunc.IsAsync)
}

// TestJavaScriptParser_ExtractFunctions_GeneratorFunctions tests generator function extraction.
// This is a RED PHASE test that defines expected behavior for generator function extraction.
func TestJavaScriptParser_ExtractFunctions_GeneratorFunctions(t *testing.T) {
	sourceCode := `// generators.js

function* simpleGenerator() {
    yield 1;
    yield 2;
    yield 3;
}

function* fibonacciGenerator(n) {
    let a = 0, b = 1;
    for (let i = 0; i < n; i++) {
        yield a;
        [a, b] = [b, a + b];
    }
}

const generatorExpression = function* () {
    yield* [1, 2, 3];
};

class NumberGenerator {
    * range(start, end) {
        for (let i = start; i <= end; i++) {
            yield i;
        }
    }
}
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(
		t,
		language,
		sourceCode,
	)
	parser, err := NewJavaScriptParser()
	require.NoError(t, err)
	_ = parseTree // RED PHASE: will be used after implementation

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find 4 functions (3 generators + 1 method)
	require.Len(t, functions, 4)

	// Test simple generator
	simpleGen := findChunkByName(functions, "simpleGenerator")
	require.NotNil(t, simpleGen, "Should find 'simpleGenerator' function")
	assert.Equal(t, outbound.ConstructGenerator, simpleGen.Type)
	assert.False(t, simpleGen.IsAsync)

	// Test parameterized generator
	fibGen := findChunkByName(functions, "fibonacciGenerator")
	require.NotNil(t, fibGen, "Should find 'fibonacciGenerator' function")
	assert.Equal(t, outbound.ConstructGenerator, fibGen.Type)
	assert.Len(t, fibGen.Parameters, 1)
	assert.Equal(t, "n", fibGen.Parameters[0].Name)

	// Test generator expression
	genExpr := findChunkByName(functions, "generatorExpression")
	require.NotNil(t, genExpr, "Should find generator expression")
	assert.Equal(t, outbound.ConstructGenerator, genExpr.Type)

	// Test generator method
	rangeMethod := findChunkByName(functions, "range")
	require.NotNil(t, rangeMethod, "Should find 'range' generator method")
	assert.Equal(t, outbound.ConstructGenerator, rangeMethod.Type)
	assert.Len(t, rangeMethod.Parameters, 2)
}

// TestJavaScriptParser_ExtractFunctions_HigherOrderFunctions tests higher-order function extraction.
// This is a RED PHASE test that defines expected behavior for higher-order function extraction.
func TestJavaScriptParser_ExtractFunctions_HigherOrderFunctions(t *testing.T) {
	sourceCode := `// higher_order.js

function createMultiplier(factor) {
    return function(x) {
        return x * factor;
    };
}

const memoize = (fn) => {
    const cache = new Map();
    return (...args) => {
        const key = JSON.stringify(args);
        if (cache.has(key)) {
            return cache.get(key);
        }
        const result = fn(...args);
        cache.set(key, result);
        return result;
    };
};

function compose(...functions) {
    return function(x) {
        return functions.reduceRight((acc, fn) => fn(acc), x);
    };
}

// IIFE (Immediately Invoked Function Expression)
const result = (function(x, y) {
    return x + y;
})(5, 3);

// Closure example
function counter() {
    let count = 0;
    return {
        increment: () => ++count,
        decrement: () => --count,
        getValue: () => count
    };
}
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(
		t,
		language,
		sourceCode,
	)
	parser, err := NewJavaScriptParser()
	require.NoError(t, err)
	_ = parseTree // RED PHASE: will be used after implementation

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find multiple functions including nested ones and closures
	assert.GreaterOrEqual(t, len(functions), 6)

	// Test higher-order function that returns a function
	createMultiplierFunc := findChunkByName(functions, "createMultiplier")
	require.NotNil(t, createMultiplierFunc, "Should find 'createMultiplier' function")
	assert.Equal(t, outbound.ConstructFunction, createMultiplierFunc.Type)
	assert.Len(t, createMultiplierFunc.Parameters, 1)
	assert.Equal(t, "factor", createMultiplierFunc.Parameters[0].Name)

	// Test arrow function with closure
	memoizeFunc := findChunkByName(functions, "memoize")
	require.NotNil(t, memoizeFunc, "Should find 'memoize' function")
	assert.Equal(t, outbound.ConstructLambda, memoizeFunc.Type)

	// Test function with rest parameters
	composeFunc := findChunkByName(functions, "compose")
	require.NotNil(t, composeFunc, "Should find 'compose' function")
	assert.Len(t, composeFunc.Parameters, 1)
	assert.Equal(t, "functions", composeFunc.Parameters[0].Name)
	assert.True(t, composeFunc.Parameters[0].IsVariadic)

	// Test closure-returning function
	counterFunc := findChunkByName(functions, "counter")
	require.NotNil(t, counterFunc, "Should find 'counter' function")
	assert.Equal(t, outbound.ConstructClosure, counterFunc.Type)
}

// TestJavaScriptParser_ExtractFunctions_ErrorHandling tests error conditions.
// This is a RED PHASE test that defines expected behavior for error handling.
func TestJavaScriptParser_ExtractFunctions_ErrorHandling(t *testing.T) {
	parser, err := NewJavaScriptParser()
	require.NoError(t, err)

	t.Run("nil parse tree should return error", func(t *testing.T) {
		options := outbound.SemanticExtractionOptions{}

		_, err := parser.ExtractFunctions(context.Background(), nil, options)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse tree cannot be nil")
	})

	t.Run("unsupported language should return error", func(t *testing.T) {
		goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, goLang, "package main")
		options := outbound.SemanticExtractionOptions{}

		_, err = parser.ExtractFunctions(context.Background(), parseTree, options)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported language")
	})

	t.Run("malformed JavaScript code should not panic", func(t *testing.T) {
		malformedCode := `// Incomplete JavaScript code
function incompleteFunction(
    // Missing closing parenthesis and function body
`
		language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, malformedCode)
		options := outbound.SemanticExtractionOptions{}

		// Should not panic, even with malformed code
		functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
		// May return error or empty results, but should not panic
		if err == nil {
			assert.NotNil(t, functions)
		}
	})
}

// TestJavaScriptParser_ExtractFunctions_PrivateVisibilityFiltering tests visibility filtering.
// This is a RED PHASE test that defines expected behavior for private function filtering.
func TestJavaScriptParser_ExtractFunctions_PrivateVisibilityFiltering(t *testing.T) {
	sourceCode := `// visibility_test.js

// Public functions (standard naming)
function publicFunction() {
    return "public";
}

// Private functions (underscore prefix convention)
function _privateFunction() {
    return "private";
}

// Double underscore (very private)
function __veryPrivateFunction() {
    return "very private";
}

class TestClass {
    // Public method
    publicMethod() {
        return "public method";
    }
    
    // Private method (underscore convention)
    _privateMethod() {
        return "private method";
    }
    
    // Private field syntax (ES2022)
    #actualPrivateMethod() {
        return "actually private";
    }
}
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(
		t,
		language,
		sourceCode,
	)
	parser, err := NewJavaScriptParser()
	require.NoError(t, err)

	// Test with IncludePrivate: false
	optionsNoPrivate := outbound.SemanticExtractionOptions{
		IncludePrivate: false,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, optionsNoPrivate)
	require.NoError(t, err)

	// Should only include public functions
	publicFuncFound := findChunkByName(functions, "publicFunction") != nil
	privateFuncFound := findChunkByName(functions, "_privateFunction") != nil
	veryPrivateFuncFound := findChunkByName(functions, "__veryPrivateFunction") != nil

	assert.True(t, publicFuncFound, "Should include public function")
	assert.False(t, privateFuncFound, "Should exclude private function when IncludePrivate=false")
	assert.False(t, veryPrivateFuncFound, "Should exclude very private function when IncludePrivate=false")

	// Test with IncludePrivate: true
	optionsIncludePrivate := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
	}

	allFunctions, err := parser.ExtractFunctions(context.Background(), parseTree, optionsIncludePrivate)
	require.NoError(t, err)

	// Should include all functions
	assert.NotNil(t, findChunkByName(allFunctions, "publicFunction"))
	assert.NotNil(t, findChunkByName(allFunctions, "_privateFunction"))
	assert.NotNil(t, findChunkByName(allFunctions, "__veryPrivateFunction"))
}

// Helper functions for testing

// createMockParseTreeFromSource creates a mock parse tree for testing.
func createMockParseTreeFromSource(
	t *testing.T,
	language valueobject.Language,
	source string,
) *valueobject.ParseTree {
	t.Helper()

	// Use real tree-sitter parsing instead of mock
	return createRealParseTreeFromSource(t, language, source)
}

// createRealParseTreeFromSource creates a ParseTree using actual tree-sitter parsing.
func createRealParseTreeFromSource(
	t *testing.T,
	language valueobject.Language,
	source string,
) *valueobject.ParseTree {
	t.Helper()

	// Create tree-sitter adapter
	adapter, err := NewJavaScriptTreeSitterAdapter()
	if err != nil {
		t.Fatalf("Failed to create JavaScript tree-sitter adapter: %v", err)
	}
	defer adapter.Close()

	// Parse the source code
	parseTree, err := adapter.ParseSource(context.Background(), language, []byte(source))
	if err != nil {
		t.Fatalf("Failed to parse JavaScript source: %v", err)
	}

	return parseTree
}

// findChunkByName finds a semantic chunk by name.
func findChunkByName(chunks []outbound.SemanticCodeChunk, name string) *outbound.SemanticCodeChunk {
	for i, chunk := range chunks {
		if chunk.Name == name {
			return &chunks[i]
		}
	}
	return nil
}
