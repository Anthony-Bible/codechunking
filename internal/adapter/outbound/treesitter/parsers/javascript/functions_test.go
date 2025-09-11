package javascriptparser

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

// TestJavaScriptFunctions_IIFE tests Immediately Invoked Function Expression parsing.
// This is a RED PHASE test that defines expected behavior for IIFE extraction.
func TestJavaScriptFunctions_IIFE(t *testing.T) {
	sourceCode := `// iife_examples.js

// Basic IIFE
const result1 = (function() {
    return "hello world";
})();

// IIFE with parameters
const result2 = (function(name) {
    return "Hello, " + name + "!";
})("Alice");

// Arrow IIFE
const result3 = (() => {
    const secret = "classified";
    return secret.toUpperCase();
})();

// IIFE with complex logic
const modulePattern = (function() {
    let privateVar = 0;
    
    return {
        increment: function() {
            privateVar++;
            return privateVar;
        },
        decrement: function() {
            privateVar--;
            return privateVar;
        },
        getValue: function() {
            return privateVar;
        }
    };
})();

// Async IIFE
(async function() {
    const data = await fetch('/api/data');
    console.log(data);
})();
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

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	functions, err := adapter.ExtractFunctions(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find multiple IIFEs and their internal functions
	assert.GreaterOrEqual(t, len(functions), 6)

	// Test basic IIFE identification
	var iifeFound bool
	for _, fn := range functions {
		if fn.Type == outbound.ConstructFunction && fn.Name == "" {
			iifeFound = true
			break
		}
	}
	assert.True(t, iifeFound, "Should identify IIFE patterns")

	// Test module pattern methods
	incrementFunc := testhelpers.FindChunkByName(functions, "increment")
	assert.NotNil(t, incrementFunc, "Should find increment method in IIFE module pattern")
	if incrementFunc != nil {
		assert.Equal(t, outbound.ConstructMethod, incrementFunc.Type)
	}

	// Test async IIFE
	var asyncIIFEFound bool
	for _, fn := range functions {
		if fn.IsAsync && fn.Name == "" {
			asyncIIFEFound = true
			break
		}
	}
	assert.True(t, asyncIIFEFound, "Should identify async IIFE patterns")
}

// TestJavaScriptFunctions_NestedFunctions tests nested function parsing.
// This is a RED PHASE test that defines expected behavior for nested function extraction.
func TestJavaScriptFunctions_NestedFunctions(t *testing.T) {
	sourceCode := `// nested_functions.js

function outerFunction(x) {
    function innerFunction(y) {
        function deeplyNestedFunction(z) {
            return x + y + z;
        }
        return deeplyNestedFunction(y * 2);
    }
    
    const arrowInner = (a) => {
        const nestedArrow = (b) => a + b;
        return nestedArrow(x);
    };
    
    return innerFunction(x + 1) + arrowInner(x);
}

function createCalculator(initialValue) {
    function add(value) {
        initialValue += value;
        return add;
    }
    
    function subtract(value) {
        initialValue -= value;
        return subtract;
    }
    
    function getValue() {
        return initialValue;
    }
    
    add.subtract = subtract;
    add.getValue = getValue;
    
    return add;
}

// Nested generators
function* outerGenerator() {
    function* innerGenerator(start, end) {
        for (let i = start; i <= end; i++) {
            yield i;
        }
    }
    
    yield* innerGenerator(1, 5);
    yield* innerGenerator(10, 15);
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

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        15, // Increased depth for nested functions
	}

	functions, err := adapter.ExtractFunctions(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find all nested functions
	assert.GreaterOrEqual(t, len(functions), 8)

	// Test outer function
	outerFunc := testhelpers.FindChunkByName(functions, "outerFunction")
	require.NotNil(t, outerFunc, "Should find outer function")
	assert.Equal(t, outbound.ConstructFunction, outerFunc.Type)

	// Test nested function
	innerFunc := testhelpers.FindChunkByName(functions, "innerFunction")
	require.NotNil(t, innerFunc, "Should find inner function")
	assert.Equal(t, outbound.ConstructFunction, innerFunc.Type)

	// Test deeply nested function
	deepFunc := testhelpers.FindChunkByName(functions, "deeplyNestedFunction")
	require.NotNil(t, deepFunc, "Should find deeply nested function")
	assert.Equal(t, outbound.ConstructFunction, deepFunc.Type)

	// Test nested arrow functions
	arrowInnerFunc := testhelpers.FindChunkByName(functions, "arrowInner")
	assert.NotNil(t, arrowInnerFunc, "Should find nested arrow function")

	nestedArrowFunc := testhelpers.FindChunkByName(functions, "nestedArrow")
	assert.NotNil(t, nestedArrowFunc, "Should find nested arrow function")

	// Test calculator pattern functions
	addFunc := testhelpers.FindChunkByName(functions, "add")
	require.NotNil(t, addFunc, "Should find add function")
	assert.Equal(t, outbound.ConstructClosure, addFunc.Type)

	subtractFunc := testhelpers.FindChunkByName(functions, "subtract")
	require.NotNil(t, subtractFunc, "Should find subtract function")

	// Test nested generators
	outerGenFunc := testhelpers.FindChunkByName(functions, "outerGenerator")
	require.NotNil(t, outerGenFunc, "Should find outer generator")
	assert.Equal(t, outbound.ConstructGenerator, outerGenFunc.Type)

	innerGenFunc := testhelpers.FindChunkByName(functions, "innerGenerator")
	require.NotNil(t, innerGenFunc, "Should find inner generator")
	assert.Equal(t, outbound.ConstructGenerator, innerGenFunc.Type)
}

// TestJavaScriptFunctions_MethodChaining tests method chaining patterns.
// This is a RED PHASE test that defines expected behavior for fluent interface extraction.
func TestJavaScriptFunctions_MethodChaining(t *testing.T) {
	sourceCode := `// method_chaining.js

class FluentCalculator {
    constructor(value = 0) {
        this.value = value;
    }
    
    add(n) {
        this.value += n;
        return this;
    }
    
    multiply(n) {
        this.value *= n;
        return this;
    }
    
    subtract(n) {
        this.value -= n;
        return this;
    }
    
    divide(n) {
        if (n !== 0) {
            this.value /= n;
        }
        return this;
    }
    
    result() {
        return this.value;
    }
}

// Function-based fluent interface
function createChain(initialValue) {
    const chain = {
        value: initialValue,
        
        add(n) {
            chain.value += n;
            return chain;
        },
        
        multiply(n) {
            chain.value *= n;
            return chain;
        },
        
        execute(fn) {
            chain.value = fn(chain.value);
            return chain;
        },
        
        result() {
            return chain.value;
        }
    };
    
    return chain;
}

// Promise chain builders
const PromiseBuilder = {
    create() {
        return {
            promises: [],
            
            then(fn) {
                this.promises.push({ type: 'then', fn });
                return this;
            },
            
            catch(fn) {
                this.promises.push({ type: 'catch', fn });
                return this;
            },
            
            finally(fn) {
                this.promises.push({ type: 'finally', fn });
                return this;
            },
            
            build() {
                return this.promises.reduce((acc, { type, fn }) => {
                    return acc[type](fn);
                }, Promise.resolve());
            }
        };
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

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	functions, err := adapter.ExtractFunctions(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find all methods and functions including chained ones
	assert.GreaterOrEqual(t, len(functions), 12)

	// Test class methods that return 'this'
	addMethod := testhelpers.FindChunkByName(functions, "add")
	require.NotNil(t, addMethod, "Should find add method")
	assert.Equal(t, outbound.ConstructMethod, addMethod.Type)
	assert.Contains(t, addMethod.Metadata, "returns_this")

	multiplyMethod := testhelpers.FindChunkByName(functions, "multiply")
	require.NotNil(t, multiplyMethod, "Should find multiply method")
	assert.Equal(t, outbound.ConstructMethod, multiplyMethod.Type)

	// Test terminal method (doesn't return this)
	resultMethod := testhelpers.FindChunkByName(functions, "result")
	require.NotNil(t, resultMethod, "Should find result method")
	assert.Equal(t, outbound.ConstructMethod, resultMethod.Type)

	// Test function-based chain methods
	chainAddFunc := findFunctionInContext(functions, "add", "chain")
	assert.NotNil(t, chainAddFunc, "Should find chain add function")

	// Test promise builder methods
	thenMethod := testhelpers.FindChunkByName(functions, "then")
	assert.NotNil(t, thenMethod, "Should find then method")

	buildMethod := testhelpers.FindChunkByName(functions, "build")
	assert.NotNil(t, buildMethod, "Should find build method")
}

// TestJavaScriptFunctions_CallbackPatterns tests callback function patterns.
// This is a RED PHASE test that defines expected behavior for callback extraction.
func TestJavaScriptFunctions_CallbackPatterns(t *testing.T) {
	sourceCode := `// callbacks.js

// Array methods with callbacks
const numbers = [1, 2, 3, 4, 5];

const doubled = numbers.map(function(num) {
    return num * 2;
});

const filtered = numbers.filter(n => n > 3);

const sum = numbers.reduce((acc, curr) => acc + curr, 0);

// Event handling patterns
function setupEventHandlers() {
    document.addEventListener('click', function(event) {
        console.log('Clicked:', event.target);
    });
    
    document.addEventListener('keydown', (event) => {
        if (event.key === 'Escape') {
            closeModal();
        }
    });
    
    // Custom event system
    const eventEmitter = {
        on(event, callback) {
            this.events = this.events || {};
            this.events[event] = this.events[event] || [];
            this.events[event].push(callback);
        },
        
        emit(event, ...args) {
            if (this.events && this.events[event]) {
                this.events[event].forEach(callback => {
                    callback.apply(this, args);
                });
            }
        }
    };
}

// Async callback patterns
function fetchWithCallback(url, onSuccess, onError) {
    fetch(url)
        .then(response => response.json())
        .then(data => onSuccess(data))
        .catch(error => onError(error));
}

// Higher-order function with multiple callbacks
function createProcessor(beforeProcess, afterProcess) {
    return function process(data) {
        beforeProcess(data);
        const result = data.map(item => item * 2);
        afterProcess(result);
        return result;
    };
}

// Callback hell example
function processUserData(userId, callback) {
    getUserById(userId, function(error, user) {
        if (error) return callback(error);
        
        getPreferences(user.id, function(error, prefs) {
            if (error) return callback(error);
            
            updateUserPreferences(user, prefs, function(error, updated) {
                if (error) return callback(error);
                callback(null, updated);
            });
        });
    });
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

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        12,
	}

	functions, err := adapter.ExtractFunctions(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find all functions including anonymous callbacks
	assert.GreaterOrEqual(t, len(functions), 15)

	// Test array method callbacks
	var mapCallbackFound bool
	for _, fn := range functions {
		if fn.Name == "" && fn.Type == outbound.ConstructLambda {
			// Check if it's used in a map context
			if fn.ParentChunk != nil && contains(fn.ParentChunk.Content, "map") {
				mapCallbackFound = true
				assert.Len(t, fn.Parameters, 1, "Map callback should have 1 parameter")
				break
			}
		}
	}
	assert.True(t, mapCallbackFound, "Should find map callback function")

	// Test event handler functions
	setupFunc := testhelpers.FindChunkByName(functions, "setupEventHandlers")
	require.NotNil(t, setupFunc, "Should find setupEventHandlers function")

	// Test async callback pattern
	fetchFunc := testhelpers.FindChunkByName(functions, "fetchWithCallback")
	require.NotNil(t, fetchFunc, "Should find fetchWithCallback function")
	assert.Len(t, fetchFunc.Parameters, 3, "Should have url, onSuccess, onError parameters")

	// Test higher-order function
	createProcessorFunc := testhelpers.FindChunkByName(functions, "createProcessor")
	require.NotNil(t, createProcessorFunc, "Should find createProcessor function")
	assert.Len(t, createProcessorFunc.Parameters, 2, "Should have beforeProcess, afterProcess parameters")

	// Test nested callback pattern
	processUserDataFunc := testhelpers.FindChunkByName(functions, "processUserData")
	require.NotNil(t, processUserDataFunc, "Should find processUserData function")
	assert.Len(t, processUserDataFunc.Parameters, 2, "Should have userId, callback parameters")
}

// Helper functions for testing

// findFunctionInContext finds a function by name within a specific context.
func findFunctionInContext(chunks []outbound.SemanticCodeChunk, name, context string) *outbound.SemanticCodeChunk {
	for i, chunk := range chunks {
		if chunk.Name == name && contains(chunk.QualifiedName, context) {
			return &chunks[i]
		}
	}
	return nil
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && (s[:len(substr)] == substr ||
		s[len(s)-len(substr):] == substr ||
		containsSubstring(s, substr))))
}

// containsSubstring is a simple substring check.
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
