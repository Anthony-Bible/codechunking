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

// TestJavaScriptVariables_DeclarationTypes tests different variable declaration types.
// This is a RED PHASE test that defines expected behavior for JavaScript variable extraction.
func TestJavaScriptVariables_DeclarationTypes(t *testing.T) {
	sourceCode := `// variable_declarations.js

// var declarations
var globalVar = 'global';
var undefinedVar;
var multipleVar1 = 1, multipleVar2 = 2, multipleVar3 = 3;

// let declarations
let blockScoped = 'block';
let reassignable = 'initial';
reassignable = 'changed';

// const declarations
const CONSTANT = 'never changes';
const complexConstant = {
    name: 'object',
    value: 42,
    nested: {
        array: [1, 2, 3]
    }
};

// Destructuring assignments
const [first, second, ...rest] = [1, 2, 3, 4, 5];
const {name, age, city = 'Unknown'} = person;
let {x: newX, y: newY} = coordinates;

// Function scoped variables
function exampleFunction() {
    var functionScoped = 'function scope';
    let blockInFunction = 'block in function';
    const constInFunction = 'const in function';
    
    if (true) {
        var hoisted = 'hoisted to function scope';
        let blockOnly = 'only in this block';
        const alsoBlockOnly = 'also only in this block';
    }
    
    // hoisted is accessible here
    // blockOnly and alsoBlockOnly are not
}

// Class field declarations
class VariableClass {
    publicField = 'public';
    #privateField = 'private';
    static staticField = 'static';
    static #privateStaticField = 'private static';
    
    constructor(value) {
        this.instanceField = value;
        this._conventionallyPrivate = 'underscore prefix';
    }
}

// Module-level exports
export const exportedConstant = 'exported';
export let exportedVariable = 'also exported';
export var exportedVar = 'var export';
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

	variables, err := adapter.ExtractVariables(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find multiple variables of different types
	assert.GreaterOrEqual(t, len(variables), 15)

	// Test var declaration
	globalVarChunk := testhelpers.FindChunkByName(variables, "globalVar")
	require.NotNil(t, globalVarChunk, "Should find 'globalVar' variable")
	assert.Equal(t, outbound.ConstructVariable, globalVarChunk.Type)
	assert.Equal(t, "globalVar", globalVarChunk.Name)
	assert.Equal(t, "string", globalVarChunk.ReturnType)
	assert.Contains(t, globalVarChunk.Metadata, "declaration_type")
	assert.Equal(t, "var", globalVarChunk.Metadata["declaration_type"])

	// Test let declaration
	blockScopedChunk := testhelpers.FindChunkByName(variables, "blockScoped")
	require.NotNil(t, blockScopedChunk, "Should find 'blockScoped' variable")
	assert.Equal(t, outbound.ConstructVariable, blockScopedChunk.Type)
	assert.Equal(t, "let", blockScopedChunk.Metadata["declaration_type"])

	// Test const declaration
	constantChunk := testhelpers.FindChunkByName(variables, "CONSTANT")
	require.NotNil(t, constantChunk, "Should find 'CONSTANT' variable")
	assert.Equal(t, outbound.ConstructConstant, constantChunk.Type)
	assert.Equal(t, "const", constantChunk.Metadata["declaration_type"])

	// Test destructured variables
	firstChunk := testhelpers.FindChunkByName(variables, "first")
	assert.NotNil(t, firstChunk, "Should find destructured 'first' variable")
	if firstChunk != nil {
		assert.Contains(t, firstChunk.Metadata, "is_destructured")
		assert.True(t, firstChunk.Metadata["is_destructured"].(bool))
	}

	nameChunk := testhelpers.FindChunkByName(variables, "name")
	assert.NotNil(t, nameChunk, "Should find destructured 'name' variable")

	// Test class fields
	publicFieldChunk := testhelpers.FindChunkByName(variables, "publicField")
	assert.NotNil(t, publicFieldChunk, "Should find 'publicField' class field")
	if publicFieldChunk != nil {
		assert.Equal(t, outbound.ConstructProperty, publicFieldChunk.Type)
		assert.Equal(t, outbound.Public, publicFieldChunk.Visibility)
	}

	privateFieldChunk := testhelpers.FindChunkByName(variables, "#privateField")
	assert.NotNil(t, privateFieldChunk, "Should find private field")
	if privateFieldChunk != nil {
		assert.Equal(t, outbound.Private, privateFieldChunk.Visibility)
	}

	staticFieldChunk := testhelpers.FindChunkByName(variables, "staticField")
	assert.NotNil(t, staticFieldChunk, "Should find static field")
	if staticFieldChunk != nil {
		assert.True(t, staticFieldChunk.IsStatic)
	}
}

// TestJavaScriptVariables_Hoisting tests variable hoisting behavior.
// This is a RED PHASE test that defines expected behavior for hoisted variable detection.
func TestJavaScriptVariables_Hoisting(t *testing.T) {
	sourceCode := `// hoisting_examples.js

// Function hoisting
function hoistedFunction() {
    return 'I am hoisted';
}

// Variable hoisting with var
function testVarHoisting() {
    console.log(hoistedVar); // undefined, but not error
    var hoistedVar = 'now defined';
    console.log(hoistedVar); // 'now defined'
}

// let/const temporal dead zone
function testTemporalDeadZone() {
    // console.log(notYetDefined); // Would throw ReferenceError
    let notYetDefined = 'defined now';
    
    // console.log(alsoNotYetDefined); // Would throw ReferenceError
    const alsoNotYetDefined = 'const defined';
}

// Function expressions are not hoisted
// console.log(notHoisted); // Would throw ReferenceError
const notHoisted = function() {
    return 'not hoisted';
};

// Arrow functions are not hoisted
// console.log(alsoNotHoisted); // Would throw ReferenceError
const alsoNotHoisted = () => 'also not hoisted';

// Complex hoisting scenarios
function complexHoisting() {
    function inner() {
        console.log(outerVar); // undefined due to hoisting
        var outerVar = 'inner scope';
    }
    
    var outerVar = 'outer scope';
    inner();
}

// Class hoisting (classes are not hoisted)
// const instance = new NotHoistedClass(); // Would throw ReferenceError
class NotHoistedClass {
    constructor() {
        this.message = 'Classes are not hoisted';
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

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	variables, err := adapter.ExtractVariables(ctx, parseTree, options)
	require.NoError(t, err)

	// Test hoisted var
	hoistedVarChunk := testhelpers.FindChunkByName(variables, "hoistedVar")
	assert.NotNil(t, hoistedVarChunk, "Should find hoisted var")
	if hoistedVarChunk != nil {
		assert.Contains(t, hoistedVarChunk.Metadata, "is_hoisted")
		assert.True(t, hoistedVarChunk.Metadata["is_hoisted"].(bool))
	}

	// Test non-hoisted let
	notYetDefinedChunk := testhelpers.FindChunkByName(variables, "notYetDefined")
	assert.NotNil(t, notYetDefinedChunk, "Should find let variable")
	if notYetDefinedChunk != nil {
		assert.Contains(t, notYetDefinedChunk.Metadata, "temporal_dead_zone")
		assert.True(t, notYetDefinedChunk.Metadata["temporal_dead_zone"].(bool))
	}

	// Test function expressions (should be variables, not hoisted functions)
	notHoistedChunk := testhelpers.FindChunkByName(variables, "notHoisted")
	assert.NotNil(t, notHoistedChunk, "Should find function expression variable")
	if notHoistedChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, notHoistedChunk.Type)
		assert.Contains(t, notHoistedChunk.Metadata, "is_function_expression")
		assert.True(t, notHoistedChunk.Metadata["is_function_expression"].(bool))
	}
}

// TestJavaScriptVariables_ScopeVisibility tests variable scope and visibility.
// This is a RED PHASE test that defines expected behavior for scope-based visibility.
func TestJavaScriptVariables_ScopeVisibility(t *testing.T) {
	sourceCode := `// scope_visibility.js

// Global scope
var globalVar = 'global';
let globalLet = 'global let';
const GLOBAL_CONST = 'global const';

// Module pattern with private variables
const ModulePattern = (function() {
    // Private variables (not accessible outside)
    let privateCounter = 0;
    const PRIVATE_CONSTANT = 'secret';
    
    // Private functions
    function privateFunction() {
        return privateCounter++;
    }
    
    // Public interface
    return {
        // Public methods that access private variables
        increment: function() {
            return privateFunction();
        },
        
        getCount: function() {
            return privateCounter;
        },
        
        // Public variable
        publicProperty: 'public'
    };
})();

// Function scope
function functionScope() {
    var functionVar = 'function scoped';
    let functionLet = 'function let';
    const FUNCTION_CONST = 'function const';
    
    // Block scope within function
    if (true) {
        var blockVar = 'still function scoped'; // var is function-scoped
        let blockLet = 'block scoped'; // let is block-scoped
        const BLOCK_CONST = 'block const'; // const is block-scoped
    }
    
    // blockVar is accessible here
    // blockLet and BLOCK_CONST are not accessible here
}

// Loop scoping
for (var i = 0; i < 3; i++) {
    // var i is function-scoped (or global if not in function)
}
// i is still accessible here

for (let j = 0; j < 3; j++) {
    // let j is block-scoped to this loop
}
// j is not accessible here

// Class scope
class ScopeDemo {
    // Public properties
    publicProp = 'public';
    
    // Private properties (ES2022)
    #privateProp = 'private';
    
    // Static properties
    static staticProp = 'static';
    static #privateStaticProp = 'private static';
    
    constructor() {
        // Instance properties
        this.instanceProp = 'instance';
        this._conventionalPrivate = 'conventional private';
    }
    
    method() {
        // Method local variables
        let methodLocal = 'method local';
        const METHOD_CONST = 'method constant';
        
        // Access private property
        return this.#privateProp + methodLocal;
    }
}

// Closure scope
function createClosure(outerParam) {
    let outerVar = 'outer';
    
    return function innerFunction(innerParam) {
        let innerVar = 'inner';
        
        // Inner function has access to:
        // - its own variables (innerVar, innerParam)
        // - outer function variables (outerVar, outerParam)
        // - global variables
        
        return outerParam + outerVar + innerParam + innerVar;
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

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        12,
	}

	variables, err := adapter.ExtractVariables(ctx, parseTree, options)
	require.NoError(t, err)

	// Test global variables
	globalVarChunk := testhelpers.FindChunkByName(variables, "globalVar")
	assert.NotNil(t, globalVarChunk, "Should find global var")
	if globalVarChunk != nil {
		assert.Contains(t, globalVarChunk.Metadata, "scope")
		assert.Equal(t, "global", globalVarChunk.Metadata["scope"])
	}

	// Test private variables in module pattern
	privateCounterChunk := testhelpers.FindChunkByName(variables, "privateCounter")
	assert.NotNil(t, privateCounterChunk, "Should find private counter")
	if privateCounterChunk != nil {
		assert.Equal(t, outbound.Private, privateCounterChunk.Visibility)
		assert.Contains(t, privateCounterChunk.Metadata, "scope")
		assert.Equal(t, "closure", privateCounterChunk.Metadata["scope"])
	}

	// Test class properties with different visibility
	publicPropChunk := testhelpers.FindChunkByName(variables, "publicProp")
	assert.NotNil(t, publicPropChunk, "Should find public property")
	if publicPropChunk != nil {
		assert.Equal(t, outbound.Public, publicPropChunk.Visibility)
	}

	privatePropChunk := testhelpers.FindChunkByName(variables, "#privateProp")
	assert.NotNil(t, privatePropChunk, "Should find private property")
	if privatePropChunk != nil {
		assert.Equal(t, outbound.Private, privatePropChunk.Visibility)
	}

	staticPropChunk := testhelpers.FindChunkByName(variables, "staticProp")
	assert.NotNil(t, staticPropChunk, "Should find static property")
	if staticPropChunk != nil {
		assert.True(t, staticPropChunk.IsStatic)
		assert.Equal(t, outbound.Public, staticPropChunk.Visibility)
	}

	// Test block-scoped variables
	blockLetChunk := testhelpers.FindChunkByName(variables, "blockLet")
	assert.NotNil(t, blockLetChunk, "Should find block-scoped let")
	if blockLetChunk != nil {
		assert.Contains(t, blockLetChunk.Metadata, "scope")
		assert.Equal(t, "block", blockLetChunk.Metadata["scope"])
	}

	// Test loop variables
	loopIChunk := testhelpers.FindChunkByName(variables, "i")
	assert.NotNil(t, loopIChunk, "Should find loop variable i")
	if loopIChunk != nil {
		assert.Contains(t, loopIChunk.Metadata, "loop_variable")
		assert.True(t, loopIChunk.Metadata["loop_variable"].(bool))
	}

	loopJChunk := testhelpers.FindChunkByName(variables, "j")
	assert.NotNil(t, loopJChunk, "Should find loop variable j")
	if loopJChunk != nil {
		assert.Contains(t, loopJChunk.Metadata, "loop_variable")
		assert.True(t, loopJChunk.Metadata["loop_variable"].(bool))
		assert.Contains(t, loopJChunk.Metadata, "scope")
		assert.Equal(t, "block", loopJChunk.Metadata["scope"])
	}
}

// TestJavaScriptVariables_TypeInference tests basic type inference for variables.
// This is a RED PHASE test that defines expected behavior for JavaScript type inference.
func TestJavaScriptVariables_TypeInference(t *testing.T) {
	sourceCode := `// type_inference.js

// Primitive types
const stringVar = 'hello';
const numberVar = 42;
const booleanVar = true;
const nullVar = null;
const undefinedVar = undefined;
const symbolVar = Symbol('unique');
const bigintVar = 123n;

// Object types
const arrayVar = [1, 2, 3];
const objectVar = { key: 'value' };
const functionVar = function() { return 'function'; };
const arrowFunctionVar = () => 'arrow function';
const regexVar = /pattern/g;
const dateVar = new Date();

// Complex types
const mixedArray = [1, 'string', true, { nested: 'object' }];
const nestedObject = {
    name: 'John',
    age: 30,
    address: {
        street: '123 Main St',
        city: 'Anytown',
        coordinates: [40.7128, -74.0060]
    },
    hobbies: ['reading', 'coding', 'gaming']
};

// Function return type inference
function returnsString() {
    return 'string result';
}

function returnsNumber() {
    return 42;
}

function returnsObject() {
    return { result: 'object' };
}

const stringResult = returnsString();
const numberResult = returnsNumber();
const objectResult = returnsObject();

// Class instance type
class User {
    constructor(name) {
        this.name = name;
    }
}

const userInstance = new User('Alice');

// Promise type
const promiseVar = Promise.resolve('resolved value');
const asyncResult = fetch('/api/data');

// Template literals
const templateLiteral = 'Hello, ${name}!';
const taggedTemplate = tag'Hello, ${name}!';

// Computed property names
const computedKey = 'dynamic';
const computedObject = {
    [computedKey]: 'computed value',
    ['prefix_' + computedKey]: 'another computed value'
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

	variables, err := adapter.ExtractVariables(ctx, parseTree, options)
	require.NoError(t, err)

	// Test primitive type inference
	stringVarChunk := testhelpers.FindChunkByName(variables, "stringVar")
	assert.NotNil(t, stringVarChunk, "Should find string variable")
	if stringVarChunk != nil {
		assert.Equal(t, "string", stringVarChunk.ReturnType)
	}

	numberVarChunk := testhelpers.FindChunkByName(variables, "numberVar")
	assert.NotNil(t, numberVarChunk, "Should find number variable")
	if numberVarChunk != nil {
		assert.Equal(t, "number", numberVarChunk.ReturnType)
	}

	booleanVarChunk := testhelpers.FindChunkByName(variables, "booleanVar")
	assert.NotNil(t, booleanVarChunk, "Should find boolean variable")
	if booleanVarChunk != nil {
		assert.Equal(t, "boolean", booleanVarChunk.ReturnType)
	}

	// Test complex type inference
	arrayVarChunk := testhelpers.FindChunkByName(variables, "arrayVar")
	assert.NotNil(t, arrayVarChunk, "Should find array variable")
	if arrayVarChunk != nil {
		assert.Equal(t, "Array<number>", arrayVarChunk.ReturnType)
	}

	objectVarChunk := testhelpers.FindChunkByName(variables, "objectVar")
	assert.NotNil(t, objectVarChunk, "Should find object variable")
	if objectVarChunk != nil {
		assert.Equal(t, "Object", objectVarChunk.ReturnType)
	}

	functionVarChunk := testhelpers.FindChunkByName(variables, "functionVar")
	assert.NotNil(t, functionVarChunk, "Should find function variable")
	if functionVarChunk != nil {
		assert.Equal(t, "Function", functionVarChunk.ReturnType)
		assert.Contains(t, functionVarChunk.Metadata, "is_function_expression")
	}

	// Test instance type inference
	userInstanceChunk := testhelpers.FindChunkByName(variables, "userInstance")
	assert.NotNil(t, userInstanceChunk, "Should find user instance")
	if userInstanceChunk != nil {
		assert.Equal(t, "User", userInstanceChunk.ReturnType)
		assert.Contains(t, userInstanceChunk.Metadata, "constructor_name")
		assert.Equal(t, "User", userInstanceChunk.Metadata["constructor_name"])
	}

	// Test Promise type
	promiseVarChunk := testhelpers.FindChunkByName(variables, "promiseVar")
	assert.NotNil(t, promiseVarChunk, "Should find promise variable")
	if promiseVarChunk != nil {
		assert.Equal(t, "Promise<string>", promiseVarChunk.ReturnType)
	}

	// Test complex nested object
	nestedObjectChunk := testhelpers.FindChunkByName(variables, "nestedObject")
	assert.NotNil(t, nestedObjectChunk, "Should find nested object")
	if nestedObjectChunk != nil {
		assert.Equal(t, "Object", nestedObjectChunk.ReturnType)
		assert.Contains(t, nestedObjectChunk.Metadata, "has_nested_properties")
		assert.True(t, nestedObjectChunk.Metadata["has_nested_properties"].(bool))
	}
}

// TestJavaScriptVariables_ErrorHandling tests error conditions for variable parsing.
// This is a RED PHASE test that defines expected behavior for variable parsing error handling.
func TestJavaScriptVariables_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	t.Run("nil parse tree should return error", func(t *testing.T) {
		options := outbound.SemanticExtractionOptions{}

		_, err := adapter.ExtractVariables(ctx, nil, options)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse tree cannot be nil")
	})

	t.Run("unsupported language should return error", func(t *testing.T) {
		goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, goLang, "package main")
		options := outbound.SemanticExtractionOptions{}

		_, err = adapter.ExtractVariables(ctx, parseTree, options)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported language")
	})

	t.Run("malformed variable declaration should not panic", func(t *testing.T) {
		malformedCode := `// Malformed variable declarations
const incomplete
let = 'missing identifier';
var function = 'reserved word';
`
		language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, malformedCode)
		options := outbound.SemanticExtractionOptions{}

		// Should not panic, even with malformed code
		variables, err := adapter.ExtractVariables(ctx, parseTree, options)
		// May return error or empty results, but should not panic
		if err == nil {
			assert.NotNil(t, variables)
		}
	})
}
