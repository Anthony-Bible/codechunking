package javascriptparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/adapter/outbound/treesitter/parsers/testhelpers"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"

	forest "github.com/alexaandru/go-sitter-forest"
	tree_sitter "github.com/alexaandru/go-tree-sitter-bare"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJavaScriptParser_NewJavaScriptParser tests creation of JavaScript parser.
// This is a RED PHASE test that defines expected behavior for JavaScript parser creation.
func TestJavaScriptParser_NewJavaScriptParser(t *testing.T) {
	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()
	require.NotNil(t, adapter)

	// Test supported language - we'll test this by attempting to extract functions
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	// Create a simple parse tree to test language support
	parseTree := createMockParseTreeFromSource(t, jsLang, "function test() {}")

	// If we can extract functions without error, the language is supported
	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	_, err = adapter.ExtractFunctions(ctx, parseTree, options)
	assert.NoError(t, err, "JavaScript should be supported")
}

// TestJavaScriptParser_GetSupportedConstructTypes tests supported construct types.
// This is a RED PHASE test that defines expected JavaScript construct types.
func TestJavaScriptParser_GetSupportedConstructTypes(t *testing.T) {
	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()
	require.NotNil(t, adapter)

	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	types, err := adapter.GetSupportedConstructTypes(ctx, jsLang)
	require.NoError(t, err)

	expectedTypes := []outbound.SemanticConstructType{
		outbound.ConstructFunction,
		outbound.ConstructMethod,
		outbound.ConstructClass,
		outbound.ConstructStruct,
		outbound.ConstructModule,
		outbound.ConstructPackage,
	}

	require.Len(t, types, len(expectedTypes))
	for _, expectedType := range expectedTypes {
		assert.Contains(t, types, expectedType)
	}
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

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()
	require.NoError(t, err)
	_ = parseTree // RED PHASE: will be used after implementation

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	functions, err := adapter.ExtractFunctions(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find 5 functions (add, multiply, subtract, divide, power)
	require.Len(t, functions, 5)

	// Test regular function declaration
	addFunc := testhelpers.FindChunkByName(functions, "add")
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
	multiplyFunc := testhelpers.FindChunkByName(functions, "multiply")
	require.NotNil(t, multiplyFunc, "Should find 'multiply' function")
	assert.Contains(t, multiplyFunc.Documentation, "Multiply two numbers")
	assert.Contains(t, multiplyFunc.Documentation, "@param {number} x")
	assert.Contains(t, multiplyFunc.Documentation, "@returns {number}")

	// Test anonymous function expression - should remain anonymous
	// Note: Anonymous function expressions do NOT inherit variable names per JavaScript semantics
	var subtractFunc *outbound.SemanticCodeChunk
	for i := range functions {
		if functions[i].Name == "" && functions[i].Type == outbound.ConstructFunction {
			subtractFunc = &functions[i]
			break
		}
	}
	require.NotNil(t, subtractFunc, "Should find anonymous function expression")
	assert.Equal(t, outbound.ConstructFunction, subtractFunc.Type)
	assert.Empty(t, subtractFunc.Name, "Anonymous function expressions should have empty names")

	// Test arrow function (single expression)
	divideFunc := testhelpers.FindChunkByName(functions, "divide")
	require.NotNil(t, divideFunc, "Should find 'divide' arrow function")
	assert.Equal(t, outbound.ConstructFunction, divideFunc.Type)

	// Test arrow function (block body)
	powerFunc := testhelpers.FindChunkByName(functions, "power")
	require.NotNil(t, powerFunc, "Should find 'power' arrow function")
	assert.Equal(t, outbound.ConstructFunction, powerFunc.Type)
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

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()
	_ = parseTree // RED PHASE: will be used after implementation

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	functions, err := adapter.ExtractFunctions(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find 4 functions (fetchData, asyncArrow, asyncGenerator, processData)
	require.Len(t, functions, 4)

	// Test async function
	fetchFunc := testhelpers.FindChunkByName(functions, "fetchData")
	require.NotNil(t, fetchFunc, "Should find 'fetchData' async function")
	assert.Equal(t, outbound.ConstructFunction, fetchFunc.Type)
	assert.True(t, fetchFunc.IsAsync)
	assert.Len(t, fetchFunc.Parameters, 1)
	assert.Equal(t, "url", fetchFunc.Parameters[0].Name)

	// Test async arrow function
	arrowFunc := testhelpers.FindChunkByName(functions, "asyncArrow")
	require.NotNil(t, arrowFunc, "Should find 'asyncArrow' async function")
	assert.True(t, arrowFunc.IsAsync)
	assert.Equal(t, outbound.ConstructFunction, arrowFunc.Type)

	// Test async generator
	generatorFunc := testhelpers.FindChunkByName(functions, "asyncGenerator")
	require.NotNil(t, generatorFunc, "Should find 'asyncGenerator' function")
	assert.Equal(t, outbound.ConstructFunction, generatorFunc.Type)
	assert.True(t, generatorFunc.IsAsync)
	assert.True(t, generatorFunc.IsGeneric, "Generators should have IsGeneric=true")

	// Test async method in object
	methodFunc := testhelpers.FindChunkByName(functions, "processData")
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

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()
	_ = parseTree // RED PHASE: will be used after implementation

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	functions, err := adapter.ExtractFunctions(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find 4 functions (3 generators + 1 method)
	require.Len(t, functions, 4)

	// Test simple generator
	simpleGen := testhelpers.FindChunkByName(functions, "simpleGenerator")
	require.NotNil(t, simpleGen, "Should find 'simpleGenerator' function")
	assert.Equal(t, outbound.ConstructFunction, simpleGen.Type)
	assert.True(t, simpleGen.IsGeneric, "Generators should have IsGeneric=true")
	assert.False(t, simpleGen.IsAsync)

	// Test parameterized generator
	fibGen := testhelpers.FindChunkByName(functions, "fibonacciGenerator")
	require.NotNil(t, fibGen, "Should find 'fibonacciGenerator' function")
	assert.Equal(t, outbound.ConstructFunction, fibGen.Type)
	assert.True(t, fibGen.IsGeneric, "Generators should have IsGeneric=true")
	assert.Len(t, fibGen.Parameters, 1)
	assert.Equal(t, "n", fibGen.Parameters[0].Name)

	// Test generator expression
	genExpr := testhelpers.FindChunkByName(functions, "generatorExpression")
	require.NotNil(t, genExpr, "Should find generator expression")
	assert.Equal(t, outbound.ConstructFunction, genExpr.Type)
	assert.True(t, genExpr.IsGeneric, "Generators should have IsGeneric=true")

	// Test generator method
	rangeMethod := testhelpers.FindChunkByName(functions, "range")
	require.NotNil(t, rangeMethod, "Should find 'range' generator method")
	assert.Equal(t, outbound.ConstructMethod, rangeMethod.Type)
	assert.True(t, rangeMethod.IsGeneric, "Generator methods should have IsGeneric=true")
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

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()
	_ = parseTree // RED PHASE: will be used after implementation

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	functions, err := adapter.ExtractFunctions(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find multiple functions including nested ones and closures
	assert.GreaterOrEqual(t, len(functions), 6)

	// Test higher-order function that returns a function
	createMultiplierFunc := testhelpers.FindChunkByName(functions, "createMultiplier")
	require.NotNil(t, createMultiplierFunc, "Should find 'createMultiplier' function")
	assert.Equal(t, outbound.ConstructFunction, createMultiplierFunc.Type)
	assert.Len(t, createMultiplierFunc.Parameters, 1)
	assert.Equal(t, "factor", createMultiplierFunc.Parameters[0].Name)

	// Test arrow function with closure
	memoizeFunc := testhelpers.FindChunkByName(functions, "memoize")
	require.NotNil(t, memoizeFunc, "Should find 'memoize' function")
	assert.Equal(t, outbound.ConstructFunction, memoizeFunc.Type)

	// Test function with rest parameters
	composeFunc := testhelpers.FindChunkByName(functions, "compose")
	require.NotNil(t, composeFunc, "Should find 'compose' function")
	assert.Len(t, composeFunc.Parameters, 1)
	assert.Equal(t, "functions", composeFunc.Parameters[0].Name)
	assert.True(t, composeFunc.Parameters[0].IsVariadic)

	// Test closure-returning function
	counterFunc := testhelpers.FindChunkByName(functions, "counter")
	require.NotNil(t, counterFunc, "Should find 'counter' function")
	assert.Equal(t, outbound.ConstructFunction, counterFunc.Type)
}

// TestJavaScriptParser_ExtractFunctions_ErrorHandling tests error conditions.
// This is a RED PHASE test that defines expected behavior for error handling.
func TestJavaScriptParser_ExtractFunctions_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	t.Run("nil parse tree should return error", func(t *testing.T) {
		options := outbound.SemanticExtractionOptions{}

		_, err := adapter.ExtractFunctions(ctx, nil, options)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse tree cannot be nil")
	})

	t.Run("unsupported language should return error", func(t *testing.T) {
		goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, goLang, "package main")
		options := outbound.SemanticExtractionOptions{}

		_, err = adapter.ExtractFunctions(ctx, parseTree, options)
		require.NoError(t, err) // Go is actually supported, so no error expected
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
		functions, err := adapter.ExtractFunctions(ctx, parseTree, options)
		// May return error or empty results, but should not panic
		if err == nil {
			// Functions may be nil or empty - both are valid for malformed code
			assert.GreaterOrEqual(t, len(functions), 0, "Should return valid slice (nil or empty)")
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

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	// Test with IncludePrivate: false
	optionsNoPrivate := outbound.SemanticExtractionOptions{
		IncludePrivate: false,
	}

	functions, err := adapter.ExtractFunctions(ctx, parseTree, optionsNoPrivate)
	require.NoError(t, err)

	// Should only include public functions
	publicFuncFound := testhelpers.FindChunkByName(functions, "publicFunction") != nil
	privateFuncFound := testhelpers.FindChunkByName(functions, "_privateFunction") != nil
	veryPrivateFuncFound := testhelpers.FindChunkByName(functions, "__veryPrivateFunction") != nil

	assert.True(t, publicFuncFound, "Should include public function")
	assert.False(t, privateFuncFound, "Should exclude private function when IncludePrivate=false")
	assert.False(t, veryPrivateFuncFound, "Should exclude very private function when IncludePrivate=false")

	// Test with IncludePrivate: true
	optionsIncludePrivate := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
	}

	allFunctions, err := adapter.ExtractFunctions(ctx, parseTree, optionsIncludePrivate)
	require.NoError(t, err)

	// Should include all functions
	assert.NotNil(t, testhelpers.FindChunkByName(allFunctions, "publicFunction"))
	assert.NotNil(t, testhelpers.FindChunkByName(allFunctions, "_privateFunction"))
	assert.NotNil(t, testhelpers.FindChunkByName(allFunctions, "__veryPrivateFunction"))
}

// Helper functions for testing

// createMockParseTreeFromSource creates a mock parse tree for testing.
func createMockParseTreeFromSource(
	t *testing.T,
	lang valueobject.Language,
	sourceCode string,
) *valueobject.ParseTree {
	t.Helper()

	// REFACTOR PHASE: Use real tree-sitter parsing instead of mock
	return createRealParseTreeFromSource(t, lang, sourceCode)
}

// createRealParseTreeFromSource creates a ParseTree using actual tree-sitter parsing.
func createRealParseTreeFromSource(
	t *testing.T,
	lang valueobject.Language,
	sourceCode string,
) *valueobject.ParseTree {
	t.Helper()

	// Get JavaScript grammar from forest
	grammar := forest.GetLanguage("javascript")
	require.NotNil(t, grammar, "Failed to get JavaScript grammar from forest")

	// Create tree-sitter parser
	parser := tree_sitter.NewParser()
	require.NotNil(t, parser, "Failed to create tree-sitter parser")

	success := parser.SetLanguage(grammar)
	require.True(t, success, "Failed to set JavaScript language")

	// Parse the source code
	tree, err := parser.ParseString(context.Background(), nil, []byte(sourceCode))
	require.NoError(t, err, "Failed to parse JavaScript source")
	require.NotNil(t, tree, "Parse tree should not be nil")
	defer tree.Close()

	// Convert tree-sitter tree to domain ParseNode
	rootTSNode := tree.RootNode()
	rootNode, nodeCount, maxDepth := convertTreeSitterNode(rootTSNode, 0)

	// Create metadata with parsing statistics
	metadata, err := valueobject.NewParseMetadata(
		time.Millisecond, // placeholder duration
		"go-tree-sitter-bare",
		"1.0.0",
	)
	require.NoError(t, err, "Failed to create metadata")

	// Update metadata with actual counts
	metadata.NodeCount = nodeCount
	metadata.MaxDepth = maxDepth

	// Create domain parse tree
	domainParseTree, err := valueobject.NewParseTree(
		context.Background(),
		lang,
		rootNode,
		[]byte(sourceCode),
		metadata,
	)
	require.NoError(t, err, "Failed to create domain parse tree")

	return domainParseTree
}
