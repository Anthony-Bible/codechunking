package javascriptparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJavaScriptVariables_ModernES6Features - RED PHASE test for modern JavaScript features.
func TestJavaScriptVariables_ModernES6Features(t *testing.T) {
	sourceCode := `// modern_es6_features.js

// Template literal variables
const templateVar = ` + "`" + `Hello, ${name}!` + "`" + `;
const taggedTemplate = tag` + "`" + `processed ${value}` + "`" + `;

// Symbol variables
const symbolVar = Symbol('unique');
const symbolFor = Symbol.for('global');
const wellKnownSymbol = Symbol.iterator;

// Computed property names
const key = 'dynamicKey';
const computedObject = {
    [key]: 'computed value',
    ['prefix_' + key]: 'another computed',
    [Symbol.iterator]: function* () { yield 1; }
};

// Map and Set variables
const mapVar = new Map([['key1', 'value1'], ['key2', 'value2']]);
const setVar = new Set([1, 2, 3, 4, 5]);
const weakMapVar = new WeakMap();
const weakSetVar = new WeakSet();

// BigInt variables  
const bigIntLiteral = 123n;
const bigIntConstructor = BigInt(456);
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	require.NoError(t, err)

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	variables, err := adapter.ExtractVariables(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find template literal variables
	templateVarChunk := findChunkByName(variables, "templateVar")
	assert.NotNil(t, templateVarChunk, "Should find template literal variable")
	if templateVarChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, templateVarChunk.Type)
		assert.Contains(t, templateVarChunk.Metadata, "has_template_expressions")
		assert.True(t, templateVarChunk.Metadata["has_template_expressions"].(bool))
	}

	// Should find symbol variables
	symbolVarChunk := findChunkByName(variables, "symbolVar")
	assert.NotNil(t, symbolVarChunk, "Should find symbol variable")
	if symbolVarChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, symbolVarChunk.Type)
		assert.Contains(t, templateVarChunk.Metadata, "is_symbol")
		assert.True(t, templateVarChunk.Metadata["is_symbol"].(bool))
	}

	// Should find computed property object
	computedObjectChunk := findChunkByName(variables, "computedObject")
	assert.NotNil(t, computedObjectChunk, "Should find computed property object")
	if computedObjectChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, computedObjectChunk.Type)
		assert.Contains(t, computedObjectChunk.Metadata, "has_computed_properties")
		assert.True(t, computedObjectChunk.Metadata["has_computed_properties"].(bool))
	}

	// Should find Map/Set variables
	mapVarChunk := findChunkByName(variables, "mapVar")
	assert.NotNil(t, mapVarChunk, "Should find Map variable")
	if mapVarChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, mapVarChunk.Type)
		assert.Equal(t, "Map", mapVarChunk.ReturnType)
	}

	setVarChunk := findChunkByName(variables, "setVar")
	assert.NotNil(t, setVarChunk, "Should find Set variable")
	if setVarChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, setVarChunk.Type)
		assert.Equal(t, "Set", setVarChunk.ReturnType)
	}

	// Should find BigInt variables
	bigIntChunk := findChunkByName(variables, "bigIntLiteral")
	assert.NotNil(t, bigIntChunk, "Should find BigInt literal")
	if bigIntChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, bigIntChunk.Type)
		assert.Equal(t, "bigint", bigIntChunk.ReturnType)
	}
}

// TestJavaScriptVariables_AdvancedDestructuring - RED PHASE test for advanced destructuring patterns.
func TestJavaScriptVariables_AdvancedDestructuring(t *testing.T) {
	sourceCode := `// advanced_destructuring.js

// Nested destructuring
const users = [
    { name: 'Alice', profile: { age: 30, settings: { theme: 'dark' } } },
    { name: 'Bob', profile: { age: 25, settings: { theme: 'light' } } }
];
const [{ name: firstName, profile: { age, settings: { theme } } }, secondUser] = users;

// Rest element in array destructuring
const [head, ...tail] = [1, 2, 3, 4, 5];

// Rest properties in object destructuring
const { type, ...payload } = { type: 'UPDATE', id: 1, data: 'info' };

// Default values in destructuring
const { timeout = 5000, retries = 3 } = configOptions;

// Complex nested destructuring with defaults
const {
    user: { 
        name: userName = 'Guest', 
        preferences: { 
            language = 'en', 
            notifications: { email = true, sms = false } = {} 
        } = {} 
    } = {}
} = response;
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	require.NoError(t, err)

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	variables, err := adapter.ExtractVariables(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find nested destructured variables
	firstNameChunk := findChunkByName(variables, "firstName")
	assert.NotNil(t, firstNameChunk, "Should find nested destructured firstName variable")
	if firstNameChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, firstNameChunk.Type)
	}

	ageChunk := findChunkByName(variables, "age")
	assert.NotNil(t, ageChunk, "Should find nested destructured age variable")
	if ageChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, ageChunk.Type)
	}

	themeChunk := findChunkByName(variables, "theme")
	assert.NotNil(t, themeChunk, "Should find nested destructured theme variable")
	if themeChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, themeChunk.Type)
	}

	secondUserChunk := findChunkByName(variables, "secondUser")
	assert.NotNil(t, secondUserChunk, "Should find secondUser variable")
	if secondUserChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, secondUserChunk.Type)
	}

	// Should find rest element variables
	headChunk := findChunkByName(variables, "head")
	assert.NotNil(t, headChunk, "Should find head variable")
	if headChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, headChunk.Type)
	}

	tailChunk := findChunkByName(variables, "tail")
	assert.NotNil(t, tailChunk, "Should find tail rest variable")
	if tailChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, tailChunk.Type)
		assert.Contains(t, tailChunk.Metadata, "is_rest")
		assert.True(t, tailChunk.Metadata["is_rest"].(bool))
	}

	// Should find rest properties variables
	typeChunk := findChunkByName(variables, "type")
	assert.NotNil(t, typeChunk, "Should find type variable")
	if typeChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, typeChunk.Type)
	}

	payloadChunk := findChunkByName(variables, "payload")
	assert.NotNil(t, payloadChunk, "Should find payload rest variable")
	if payloadChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, payloadChunk.Type)
		assert.Contains(t, payloadChunk.Metadata, "is_rest")
		assert.True(t, payloadChunk.Metadata["is_rest"].(bool))
	}

	// Should find default value variables
	timeoutChunk := findChunkByName(variables, "timeout")
	assert.NotNil(t, timeoutChunk, "Should find timeout variable with default")
	if timeoutChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, timeoutChunk.Type)
		assert.Contains(t, timeoutChunk.Metadata, "has_default")
		assert.True(t, timeoutChunk.Metadata["has_default"].(bool))
	}

	retriesChunk := findChunkByName(variables, "retries")
	assert.NotNil(t, retriesChunk, "Should find retries variable with default")
	if retriesChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, retriesChunk.Type)
		assert.Contains(t, retriesChunk.Metadata, "has_default")
		assert.True(t, retriesChunk.Metadata["has_default"].(bool))
	}

	// Should find complex nested destructured variables with defaults
	userNameChunk := findChunkByName(variables, "userName")
	assert.NotNil(t, userNameChunk, "Should find userName variable with default")
	if userNameChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, userNameChunk.Type)
		assert.Contains(t, userNameChunk.Metadata, "has_default")
		assert.True(t, userNameChunk.Metadata["has_default"].(bool))
	}

	languageChunk := findChunkByName(variables, "language")
	assert.NotNil(t, languageChunk, "Should find language variable with default")
	if languageChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, languageChunk.Type)
		assert.Contains(t, languageChunk.Metadata, "has_default")
		assert.True(t, languageChunk.Metadata["has_default"].(bool))
	}

	emailChunk := findChunkByName(variables, "email")
	assert.NotNil(t, emailChunk, "Should find email variable with default")
	if emailChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, emailChunk.Type)
		assert.Contains(t, emailChunk.Metadata, "has_default")
		assert.True(t, emailChunk.Metadata["has_default"].(bool))
	}

	smsChunk := findChunkByName(variables, "sms")
	assert.NotNil(t, smsChunk, "Should find sms variable with default")
	if smsChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, smsChunk.Type)
		assert.Contains(t, smsChunk.Metadata, "has_default")
		assert.True(t, smsChunk.Metadata["has_default"].(bool))
	}
}

// TestJavaScriptVariables_ModulePatterns - RED PHASE test for module-related variable patterns.
func TestJavaScriptVariables_ModulePatterns(t *testing.T) {
	sourceCode := `// module_patterns.js

// Named exports with aliases
const originalName = 'value';
const renamedVariable = 'another value';
export { originalName as publicName };
export { renamedVariable as defaultExport, originalName as aliasName };

// Re-exports from other modules
export { someFunction } from './utils';
export { original as newName } from './other';
export * from './all';
export * as namespace from './namespaced';

// Dynamic imports
const modulePromise = import('./module.js');
const { default: DefaultComponent, namedExport } = await import('./Component.js');

// Import meta usage
const metaUrl = import.meta.url;
const { url: importUrl } = import.meta;

// Namespace imports
import * as utils from './utils.js';
const utilsNamespace = utils;
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	require.NoError(t, err)

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	variables, err := adapter.ExtractVariables(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find original variables before export aliases
	originalNameChunk := findChunkByName(variables, "originalName")
	assert.NotNil(t, originalNameChunk, "Should find originalName variable")
	if originalNameChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, originalNameChunk.Type)
	}

	renamedVariableChunk := findChunkByName(variables, "renamedVariable")
	assert.NotNil(t, renamedVariableChunk, "Should find renamedVariable variable")
	if renamedVariableChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, renamedVariableChunk.Type)
	}

	// Should find dynamic import variables
	modulePromiseChunk := findChunkByName(variables, "modulePromise")
	assert.NotNil(t, modulePromiseChunk, "Should find modulePromise variable")
	if modulePromiseChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, modulePromiseChunk.Type)
	}

	// Should find destructured dynamic import variables
	defaultComponentChunk := findChunkByName(variables, "DefaultComponent")
	assert.NotNil(t, defaultComponentChunk, "Should find DefaultComponent variable")
	if defaultComponentChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, defaultComponentChunk.Type)
	}

	namedExportChunk := findChunkByName(variables, "namedExport")
	assert.NotNil(t, namedExportChunk, "Should find namedExport variable")
	if namedExportChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, namedExportChunk.Type)
	}

	// Should find import meta variables
	metaUrlChunk := findChunkByName(variables, "metaUrl")
	assert.NotNil(t, metaUrlChunk, "Should find metaUrl variable")
	if metaUrlChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, metaUrlChunk.Type)
	}

	importUrlChunk := findChunkByName(variables, "importUrl")
	assert.NotNil(t, importUrlChunk, "Should find importUrl variable")
	if importUrlChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, importUrlChunk.Type)
	}

	// Should find namespace import variables
	utilsNamespaceChunk := findChunkByName(variables, "utilsNamespace")
	assert.NotNil(t, utilsNamespaceChunk, "Should find utilsNamespace variable")
	if utilsNamespaceChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, utilsNamespaceChunk.Type)
		assert.Contains(t, utilsNamespaceChunk.Metadata, "is_namespace_import")
		assert.True(t, utilsNamespaceChunk.Metadata["is_namespace_import"].(bool))
	}
}

// TestJavaScriptVariables_AsyncGeneratorPatterns - RED PHASE test for async and generator variable patterns.
func TestJavaScriptVariables_AsyncGeneratorPatterns(t *testing.T) {
	sourceCode := `// async_generator_patterns.js

// Generator function and variable
function* dataGenerator() {
    yield 1;
    yield 2;
}
const genInstance = dataGenerator();

// Async function and variable
async function fetchData() {
    return await fetch('/api/data');
}
const asyncResult = await fetchData();

// Async generator function and variable
async function* asyncDataGenerator() {
    yield await fetch('/api/data/1');
    yield await fetch('/api/data/2');
}
const asyncGenInstance = asyncDataGenerator();

// Async arrow function variable
const processData = async (input) => {
    return await transform(input);
};

// For-await loop variable
const asyncIterable = {
    async *[Symbol.asyncIterator]() {
        yield await getValue();
    }
};
for await (const item of asyncIterable) {
    console.log(item);
}

// Promise variables
const promiseVar = new Promise((resolve) => resolve('done'));
const resolvedValue = await promiseVar;
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	require.NoError(t, err)

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	variables, err := adapter.ExtractVariables(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find generator instance variable
	genInstanceChunk := findChunkByName(variables, "genInstance")
	assert.NotNil(t, genInstanceChunk, "Should find genInstance variable")
	if genInstanceChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, genInstanceChunk.Type)
		assert.Contains(t, genInstanceChunk.Metadata, "is_generator")
		assert.True(t, genInstanceChunk.Metadata["is_generator"].(bool))
	}

	// Should find async result variable
	asyncResultChunk := findChunkByName(variables, "asyncResult")
	assert.NotNil(t, asyncResultChunk, "Should find asyncResult variable")
	if asyncResultChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, asyncResultChunk.Type)
		assert.Contains(t, asyncResultChunk.Metadata, "is_async")
		assert.True(t, asyncResultChunk.Metadata["is_async"].(bool))
	}

	// Should find async generator instance variable
	asyncGenInstanceChunk := findChunkByName(variables, "asyncGenInstance")
	assert.NotNil(t, asyncGenInstanceChunk, "Should find asyncGenInstance variable")
	if asyncGenInstanceChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, asyncGenInstanceChunk.Type)
		assert.Contains(t, asyncGenInstanceChunk.Metadata, "is_async_generator")
		assert.True(t, asyncGenInstanceChunk.Metadata["is_async_generator"].(bool))
	}

	// Should find async arrow function variable
	processDataChunk := findChunkByName(variables, "processData")
	assert.NotNil(t, processDataChunk, "Should find processData variable")
	if processDataChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, processDataChunk.Type)
		assert.Contains(t, processDataChunk.Metadata, "is_async")
		assert.True(t, processDataChunk.Metadata["is_async"].(bool))
	}

	// Should find for-await loop variable
	itemChunk := findChunkByName(variables, "item")
	assert.NotNil(t, itemChunk, "Should find item variable from for-await loop")
	if itemChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, itemChunk.Type)
		assert.Contains(t, itemChunk.Metadata, "is_async_iterable")
		assert.True(t, itemChunk.Metadata["is_async_iterable"].(bool))
	}

	// Should find promise variable
	promiseVarChunk := findChunkByName(variables, "promiseVar")
	assert.NotNil(t, promiseVarChunk, "Should find promiseVar variable")
	if promiseVarChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, promiseVarChunk.Type)
		assert.Equal(t, "Promise", promiseVarChunk.ReturnType)
	}

	// Should find resolved promise value variable
	resolvedValueChunk := findChunkByName(variables, "resolvedValue")
	assert.NotNil(t, resolvedValueChunk, "Should find resolvedValue variable")
	if resolvedValueChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, resolvedValueChunk.Type)
		assert.Contains(t, resolvedValueChunk.Metadata, "is_promise_result")
		assert.True(t, resolvedValueChunk.Metadata["is_promise_result"].(bool))
	}
}

// TestJavaScriptVariables_EdgeCasesAndUnicode - RED PHASE test for edge cases and unicode identifiers.
func TestJavaScriptVariables_EdgeCasesAndUnicode(t *testing.T) {
	sourceCode := `// edge_cases_unicode.js

// Unicode identifiers
const α = 1;
const β = 2;
const _γ = 3;
const Δ = 4;
const π_value = 3.14;
const 中文变量 = 'chinese';
const $special = 'dollar';
const \\u0041 = 'unicode A'; // This is 'A'

// Optional chaining variables
const user = data?.user;
const name = user?.profile?.name;
const value = obj?.[getKey()]?.property;
const result = func?.(arg1, arg2);

// Nullish coalescing variables
const fallbackValue = data ?? defaultValue;
const userName = user.name ?? 'Anonymous';
const config = settings ?? { theme: 'light', lang: 'en' };
const nestedResult = obj?.prop ?? fallback;

// Private class fields
class MyClass {
    #privateField = getValue();
    #anotherPrivate = new Map([['key', 'value']]);
    
    constructor() {
        this.#privateField = 'initialized';
    }
}

const instance = new MyClass();

// Variables with complex initializers
const complexArray = [1, 2, 3].map(x => x * 2).filter(x => x > 3);
const complexObject = Object.assign({}, base, { extra: 'value' });
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	require.NoError(t, err)

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	variables, err := adapter.ExtractVariables(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find unicode identifier variables
	alphaChunk := findChunkByName(variables, "α")
	assert.NotNil(t, alphaChunk, "Should find unicode identifier α")
	if alphaChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, alphaChunk.Type)
	}

	betaChunk := findChunkByName(variables, "β")
	assert.NotNil(t, betaChunk, "Should find unicode identifier β")
	if betaChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, betaChunk.Type)
	}

	underscoreGammaChunk := findChunkByName(variables, "_γ")
	assert.NotNil(t, underscoreGammaChunk, "Should find unicode identifier _γ")
	if underscoreGammaChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, underscoreGammaChunk.Type)
	}

	deltaChunk := findChunkByName(variables, "Δ")
	assert.NotNil(t, deltaChunk, "Should find unicode identifier Δ")
	if deltaChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, deltaChunk.Type)
	}

	piValueChunk := findChunkByName(variables, "π_value")
	assert.NotNil(t, piValueChunk, "Should find unicode identifier π_value")
	if piValueChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, piValueChunk.Type)
	}

	chineseVarChunk := findChunkByName(variables, "中文变量")
	assert.NotNil(t, chineseVarChunk, "Should find Chinese identifier 中文变量")
	if chineseVarChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, chineseVarChunk.Type)
	}

	dollarSpecialChunk := findChunkByName(variables, "$special")
	assert.NotNil(t, dollarSpecialChunk, "Should find dollar identifier $special")
	if dollarSpecialChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, dollarSpecialChunk.Type)
	}

	unicodeAChunk := findChunkByName(variables, "A")
	assert.NotNil(t, unicodeAChunk, "Should find unicode escape sequence identifier A")
	if unicodeAChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, unicodeAChunk.Type)
	}

	// Should find optional chaining variables
	userChunk := findChunkByName(variables, "user")
	assert.NotNil(t, userChunk, "Should find optional chaining user variable")
	if userChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, userChunk.Type)
		assert.Contains(t, userChunk.Metadata, "has_optional_chaining")
		assert.True(t, userChunk.Metadata["has_optional_chaining"].(bool))
	}

	nameChunk := findChunkByName(variables, "name")
	assert.NotNil(t, nameChunk, "Should find optional chaining name variable")
	if nameChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, nameChunk.Type)
		assert.Contains(t, nameChunk.Metadata, "has_optional_chaining")
		assert.True(t, nameChunk.Metadata["has_optional_chaining"].(bool))
	}

	valueChunk := findChunkByName(variables, "value")
	assert.NotNil(t, valueChunk, "Should find optional chaining value variable")
	if valueChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, valueChunk.Type)
		assert.Contains(t, valueChunk.Metadata, "has_optional_chaining")
		assert.True(t, valueChunk.Metadata["has_optional_chaining"].(bool))
	}

	resultChunk := findChunkByName(variables, "result")
	assert.NotNil(t, resultChunk, "Should find optional chaining result variable")
	if resultChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, resultChunk.Type)
		assert.Contains(t, resultChunk.Metadata, "has_optional_chaining")
		assert.True(t, resultChunk.Metadata["has_optional_chaining"].(bool))
	}

	// Should find nullish coalescing variables
	fallbackValueChunk := findChunkByName(variables, "fallbackValue")
	assert.NotNil(t, fallbackValueChunk, "Should find nullish coalescing fallbackValue variable")
	if fallbackValueChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, fallbackValueChunk.Type)
		assert.Contains(t, fallbackValueChunk.Metadata, "has_nullish_coalescing")
		assert.True(t, fallbackValueChunk.Metadata["has_nullish_coalescing"].(bool))
	}

	userNameChunk := findChunkByName(variables, "userName")
	assert.NotNil(t, userNameChunk, "Should find nullish coalescing userName variable")
	if userNameChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, userNameChunk.Type)
		assert.Contains(t, userNameChunk.Metadata, "has_nullish_coalescing")
		assert.True(t, userNameChunk.Metadata["has_nullish_coalescing"].(bool))
	}

	configChunk := findChunkByName(variables, "config")
	assert.NotNil(t, configChunk, "Should find nullish coalescing config variable")
	if configChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, configChunk.Type)
		assert.Contains(t, configChunk.Metadata, "has_nullish_coalescing")
		assert.True(t, configChunk.Metadata["has_nullish_coalescing"].(bool))
	}

	nestedResultChunk := findChunkByName(variables, "nestedResult")
	assert.NotNil(t, nestedResultChunk, "Should find nullish coalescing nestedResult variable")
	if nestedResultChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, nestedResultChunk.Type)
		assert.Contains(t, nestedResultChunk.Metadata, "has_nullish_coalescing")
		assert.True(t, nestedResultChunk.Metadata["has_nullish_coalescing"].(bool))
	}

	// Should find private class field variables
	privateFieldChunk := findChunkByName(variables, "#privateField")
	assert.NotNil(t, privateFieldChunk, "Should find private class field #privateField")
	if privateFieldChunk != nil {
		assert.Equal(t, outbound.ConstructConstant, privateFieldChunk.Type)
		assert.Contains(t, privateFieldChunk.Metadata, "is_private")
		assert.True(t, privateFieldChunk.Metadata["is_private"].(bool))
	}

	anotherPrivateChunk := findChunkByName(variables, "#anotherPrivate")
	assert.NotNil(t, anotherPrivateChunk, "Should find private class field #anotherPrivate")
	if anotherPrivateChunk != nil {
		assert.Equal(t, outbound.ConstructProperty, anotherPrivateChunk.Type)
		assert.Contains(t, anotherPrivateChunk.Metadata, "is_private")
		assert.True(t, anotherPrivateChunk.Metadata["is_private"].(bool))
	}

	// Should find variables with complex initializers
	complexArrayChunk := findChunkByName(variables, "complexArray")
	assert.NotNil(t, complexArrayChunk, "Should find complexArray variable")
	if complexArrayChunk != nil {
		assert.Equal(t, outbound.ConstructVariable, complexArrayChunk.Type)
		assert.Contains(t, complexArrayChunk.Metadata, "has_complex_initializer")
		assert.True(t, complexArrayChunk.Metadata["has_complex_initializer"].(bool))
	}

	complexObjectChunk := findChunkByName(variables, "complexObject")
	assert.NotNil(t, complexObjectChunk, "Should find complexObject variable")
	if complexObjectChunk != nil {
		assert.Equal(t, outbound.ConstructVariable, complexObjectChunk.Type)
		assert.Contains(t, complexObjectChunk.Metadata, "has_complex_initializer")
		assert.True(t, complexObjectChunk.Metadata["has_complex_initializer"].(bool))
	}
}
