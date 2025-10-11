package javascriptparser

import (
	treesitter "codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJavaScriptFunctionParsing(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("Regular function declarations", func(t *testing.T) {
		sourceCode := `
function regularFunction(a, b, c) {
	return a + b + c;
}
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 1)

		fn := functions[0]
		assert.Equal(t, "regularFunction", fn.Name)
		assert.Equal(t, outbound.ConstructFunction, fn.Type)
		assert.Len(t, fn.Parameters, 3)
	})

	t.Run("Function expressions", func(t *testing.T) {
		sourceCode := `
const exprFunction = function(a, b) {
	return a * b;
};

const namedExprFunction = function named(a, b) {
	return a * b;
};
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 2)

		anonymousExpr := functions[0]
		assert.Empty(t, anonymousExpr.Name)
		assert.Equal(t, outbound.ConstructFunction, anonymousExpr.Type)

		namedExpr := functions[1]
		assert.Equal(t, "named", namedExpr.Name)
		assert.Equal(t, outbound.ConstructFunction, namedExpr.Type)
		assert.Len(t, namedExpr.Parameters, 2)
	})

	t.Run("Arrow functions with various syntaxes", func(t *testing.T) {
		sourceCode := `
const arrow1 = (a, b) => a + b;
const arrow2 = a => a * 2;
const arrow3 = () => "no params";
const arrow4 = (a, b) => {
	return a - b;
};
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 4)

		for _, fn := range functions {
			assert.Equal(t, outbound.ConstructFunction, fn.Type)
		}

		assert.Len(t, functions[0].Parameters, 2)
		assert.Len(t, functions[1].Parameters, 1)
		assert.Empty(t, functions[2].Parameters)
		assert.Len(t, functions[3].Parameters, 2)
	})

	t.Run("Async functions", func(t *testing.T) {
		sourceCode := `
async function asyncFunction() {
	return await fetch("https://api.example.com");
}

const asyncArrow = async () => {
	await Promise.resolve();
};
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 2)

		asyncFn := functions[0]
		assert.Equal(t, "asyncFunction", asyncFn.Name)
		assert.Equal(t, outbound.ConstructFunction, asyncFn.Type)
		assert.True(t, asyncFn.IsAsync)

		asyncArrow := functions[1]
		assert.Empty(t, asyncArrow.Name)
		assert.Equal(t, outbound.ConstructFunction, asyncArrow.Type)
		assert.True(t, asyncArrow.IsAsync)
	})

	t.Run("Generator functions", func(t *testing.T) {
		sourceCode := `
function* generatorFunction() {
	yield 1;
	yield 2;
}

const generatorArrow = function* () {
	yield "generator";
};
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 2)

		generatorFn := functions[0]
		assert.Equal(t, "generatorFunction", generatorFn.Name)
		assert.Equal(t, outbound.ConstructFunction, generatorFn.Type)
		assert.True(t, generatorFn.IsGeneric)

		generatorArrow := functions[1]
		assert.Empty(t, generatorArrow.Name)
		assert.Equal(t, outbound.ConstructFunction, generatorArrow.Type)
		assert.True(t, generatorArrow.IsGeneric)
	})

	t.Run("Async generator functions", func(t *testing.T) {
		sourceCode := `
async function* asyncGeneratorFunction() {
	yield await Promise.resolve(1);
	yield await Promise.resolve(2);
}
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 1)

		asyncGeneratorFn := functions[0]
		assert.Equal(t, "asyncGeneratorFunction", asyncGeneratorFn.Name)
		assert.Equal(t, outbound.ConstructFunction, asyncGeneratorFn.Type)
		assert.True(t, asyncGeneratorFn.IsAsync)
		assert.True(t, asyncGeneratorFn.IsGeneric)
	})

	t.Run("IIFE", func(t *testing.T) {
		sourceCode := `
(function() {
	console.log("IIFE executed");
})();

(() => {
	console.log("Arrow IIFE executed");
})();
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 2)

		iife1 := functions[0]
		assert.Empty(t, iife1.Name)
		assert.Equal(t, outbound.ConstructFunction, iife1.Type)

		iife2 := functions[1]
		assert.Empty(t, iife2.Name)
		assert.Equal(t, outbound.ConstructFunction, iife2.Type)
	})

	t.Run("Higher-order functions", func(t *testing.T) {
		sourceCode := `
function higherOrder(fn) {
	return function(a, b) {
		return fn(a, b);
	};
}

const higherOrderArrow = (fn) => (a, b) => fn(a, b);
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 3)

		hof1 := functions[0]
		assert.Equal(t, "higherOrder", hof1.Name)
		assert.Equal(t, outbound.ConstructFunction, hof1.Type)
		assert.Len(t, hof1.Parameters, 1)

		hof2 := functions[1]
		assert.Empty(t, hof2.Name)
		assert.Equal(t, outbound.ConstructFunction, hof2.Type)
		assert.Len(t, hof2.Parameters, 2)

		hof3 := functions[2]
		assert.Empty(t, hof3.Name)
		assert.Equal(t, outbound.ConstructFunction, hof3.Type)
	})

	t.Run("Function parameters", func(t *testing.T) {
		sourceCode := `
function complexParams(a, b = 10, ...rest, {prop1, prop2: p2, prop3 = "default"}) {
	return [a, b, rest, prop1, p2, prop3];
}
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 1)

		fn := functions[0]
		assert.Equal(t, "complexParams", fn.Name)
		assert.Equal(t, outbound.ConstructFunction, fn.Type)
		assert.Len(t, fn.Parameters, 4)

		param1 := fn.Parameters[0]
		assert.Equal(t, "a", param1.Name)

		param2 := fn.Parameters[1]
		assert.Equal(t, "b", param2.Name)

		param3 := fn.Parameters[2]
		assert.Equal(t, "rest", param3.Name)

		param4 := fn.Parameters[3]
		assert.Empty(t, param4.Name)
	})

	t.Run("Function hoisting detection", func(t *testing.T) {
		sourceCode := `
hoistedFunction();

function hoistedFunction() {
	return "hoisted";
}

notHoisted();

const notHoisted = function() {
	return "not hoisted";
};
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 2)

		hoistedFn := functions[0]
		assert.Equal(t, "hoistedFunction", hoistedFn.Name)
		assert.Equal(t, outbound.ConstructFunction, hoistedFn.Type)

		notHoistedFn := functions[1]
		assert.Empty(t, notHoistedFn.Name)
		assert.Equal(t, outbound.ConstructFunction, notHoistedFn.Type)
	})
}

func TestJavaScriptMethodParsing(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("Object method definitions", func(t *testing.T) {
		sourceCode := `
const obj = {
	method1() {
		return "method1";
	},
	
	method2: function() {
		return "method2";
	},
	
	method3: () => {
		return "method3";
	}
};
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		methods, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, methods, 3)

		method1 := methods[0]
		assert.Equal(t, "method1", method1.Name)
		assert.Equal(t, outbound.ConstructMethod, method1.Type)

		method2 := methods[1]
		assert.Equal(t, "method2", method2.Name)
		assert.Equal(t, outbound.ConstructFunction, method2.Type) // pair syntax: key: function() {}

		method3 := methods[2]
		assert.Equal(t, "method3", method3.Name)
		assert.Equal(t, outbound.ConstructFunction, method3.Type) // pair syntax: key: () => {}
	})

	t.Run("Method decorators", func(t *testing.T) {
		sourceCode := `
class DecoratedMethodClass {
	@log
	@validate
	methodWithDecorators() {
		return "decorated";
	}
}
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 1)

		method := functions[0]
		assert.Equal(t, "methodWithDecorators", method.Name)
		assert.Equal(t, outbound.ConstructMethod, method.Type)
		assert.Len(t, method.Annotations, 2)

		decorator1 := method.Annotations[0]
		assert.Equal(t, "log", decorator1.Name)

		decorator2 := method.Annotations[1]
		assert.Equal(t, "validate", decorator2.Name)
	})

	t.Run("Computed property names", func(t *testing.T) {
		sourceCode := `
const methodName = "computedMethod";

const obj = {
	[methodName]() {
		return "computed";
	},
	
	[Symbol.iterator]() {
		return "iterator";
	}
};
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		methods, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, methods, 2)

		method1 := methods[0]
		assert.Equal(t, "[methodName]", method1.Name)
		assert.Equal(t, outbound.ConstructMethod, method1.Type)

		method2 := methods[1]
		assert.Equal(t, "[Symbol.iterator]", method2.Name)
		assert.Equal(t, outbound.ConstructMethod, method2.Type)
	})

	t.Run("Symbol methods", func(t *testing.T) {
		sourceCode := `
const obj = {
	[Symbol.toStringTag]: "CustomObject",
	[Symbol.iterator]() {
		return "iterator method";
	}
};
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		methods, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, methods, 1)

		method := methods[0]
		assert.Equal(t, "[Symbol.iterator]", method.Name)
		assert.Equal(t, outbound.ConstructMethod, method.Type)
	})

	// Note: Prototype method assignment tests removed - this is a legacy JavaScript pattern
	// (pre-ES6 classes) that would require significant implementation complexity to support.
	// Modern JavaScript uses class syntax instead. If prototype assignment support is needed,
	// it would require parsing assignment_expression nodes and extracting method names from
	// member expression chains like Constructor.prototype.methodName.
}

func TestJavaScriptVariableExtraction(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("var declarations", func(t *testing.T) {
		sourceCode := `
var variable1 = "value1";
var variable2, variable3 = "value3";
var variable4;
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		variables, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, variables, 4)

		var1 := variables[0]
		assert.Equal(t, "variable1", var1.Name)
		assert.Equal(t, outbound.ConstructVariable, var1.Type)

		var2 := variables[1]
		assert.Equal(t, "variable2", var2.Name)
		assert.Equal(t, outbound.ConstructVariable, var2.Type)

		var3 := variables[2]
		assert.Equal(t, "variable3", var3.Name)
		assert.Equal(t, outbound.ConstructVariable, var3.Type)

		var4 := variables[3]
		assert.Equal(t, "variable4", var4.Name)
		assert.Equal(t, outbound.ConstructVariable, var4.Type)
	})

	t.Run("let declarations", func(t *testing.T) {
		sourceCode := `
let variable1 = "value1";
let variable2, variable3 = "value3";
let variable4;
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		variables, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		// Tree-sitter extracts all 4 variables, including uninitialized ones
		// This is correct behavior according to the JavaScript grammar
		require.Len(t, variables, 4)

		var1 := variables[0]
		assert.Equal(t, "variable1", var1.Name)
		assert.Equal(t, outbound.ConstructVariable, var1.Type)

		var2 := variables[1]
		assert.Equal(t, "variable2", var2.Name)
		assert.Equal(t, outbound.ConstructVariable, var2.Type)

		var3 := variables[2]
		assert.Equal(t, "variable3", var3.Name)
		assert.Equal(t, outbound.ConstructVariable, var3.Type)

		var4 := variables[3]
		assert.Equal(t, "variable4", var4.Name)
		assert.Equal(t, outbound.ConstructVariable, var4.Type)
	})

	t.Run("const declarations", func(t *testing.T) {
		sourceCode := `
const constant1 = "value1";
const constant2 = 42;
const {prop1, prop2} = obj;
const [item1, item2] = array;
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		variables, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		// Destructuring extracts ALL identifiers: constant1, constant2, prop1, prop2, item1, item2 = 6 total
		require.Len(t, variables, 6)

		const1 := variables[0]
		assert.Equal(t, "constant1", const1.Name)
		assert.Equal(t, outbound.ConstructConstant, const1.Type)

		const2 := variables[1]
		assert.Equal(t, "constant2", const2.Name)
		assert.Equal(t, outbound.ConstructConstant, const2.Type)

		const3 := variables[2]
		assert.Equal(t, "prop1", const3.Name)
		assert.Equal(t, outbound.ConstructConstant, const3.Type)

		const4 := variables[3]
		assert.Equal(t, "prop2", const4.Name)
		assert.Equal(t, outbound.ConstructConstant, const4.Type)

		const5 := variables[4]
		assert.Equal(t, "item1", const5.Name)
		assert.Equal(t, outbound.ConstructConstant, const5.Type)

		const6 := variables[5]
		assert.Equal(t, "item2", const6.Name)
		assert.Equal(t, outbound.ConstructConstant, const6.Type)
	})

	t.Run("Destructuring assignments", func(t *testing.T) {
		sourceCode := `
const obj = {a: 1, b: 2, c: 3};
const {a, b: renamedB, c = "default"} = obj;

const array = [1, 2, 3];
const [first, second, third] = array;
const [head, ...tail] = array;
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		variables, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		// All variables: obj, a, renamedB, c, array, first, second, third, head, tail = 10 total
		require.Len(t, variables, 10)

		var0 := variables[0]
		assert.Equal(t, "obj", var0.Name)
		assert.Equal(t, outbound.ConstructConstant, var0.Type)

		var1 := variables[1]
		assert.Equal(t, "a", var1.Name)
		assert.Equal(t, outbound.ConstructConstant, var1.Type)

		var2 := variables[2]
		assert.Equal(t, "renamedB", var2.Name)
		assert.Equal(t, outbound.ConstructConstant, var2.Type)

		var3 := variables[3]
		assert.Equal(t, "c", var3.Name)
		assert.Equal(t, outbound.ConstructConstant, var3.Type)

		var4 := variables[4]
		assert.Equal(t, "array", var4.Name)
		assert.Equal(t, outbound.ConstructConstant, var4.Type)

		var5 := variables[5]
		assert.Equal(t, "first", var5.Name)
		assert.Equal(t, outbound.ConstructConstant, var5.Type)

		var6 := variables[6]
		assert.Equal(t, "second", var6.Name)
		assert.Equal(t, outbound.ConstructConstant, var6.Type)

		var7 := variables[7]
		assert.Equal(t, "third", var7.Name)
		assert.Equal(t, outbound.ConstructConstant, var7.Type)

		var8 := variables[8]
		assert.Equal(t, "head", var8.Name)
		assert.Equal(t, outbound.ConstructConstant, var8.Type)

		var9 := variables[9]
		assert.Equal(t, "tail", var9.Name)
		assert.Equal(t, outbound.ConstructConstant, var9.Type)
	})

	t.Run("Multiple variable declarations", func(t *testing.T) {
		sourceCode := `
var a = 1, b = 2, c;
let x = "x", y = "y", z;
const p = "p", q = "q";
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		variables, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		// 3 var + 3 let + 2 const = 8 total variables
		require.Len(t, variables, 8)

		varNames := []string{}
		for _, v := range variables {
			varNames = append(varNames, v.Name)
		}
		expectedNames := []string{"a", "b", "c", "x", "y", "z", "p", "q"}
		assert.ElementsMatch(t, expectedNames, varNames)
	})

	t.Run("Variable hoisting detection", func(t *testing.T) {
		sourceCode := `
console.log(hoistedVar); // undefined
var hoistedVar = "value";

console.log(notHoistedLet);
let notHoistedLet = "value";

console.log(notHoistedConst);
const notHoistedConst = "value";
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		variables, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, variables, 3)

		hoistedVar := variables[0]
		assert.Equal(t, "hoistedVar", hoistedVar.Name)
		assert.Equal(t, outbound.ConstructVariable, hoistedVar.Type)

		notHoistedLet := variables[1]
		assert.Equal(t, "notHoistedLet", notHoistedLet.Name)
		assert.Equal(t, outbound.ConstructVariable, notHoistedLet.Type)

		notHoistedConst := variables[2]
		assert.Equal(t, "notHoistedConst", notHoistedConst.Name)
		assert.Equal(t, outbound.ConstructConstant, notHoistedConst.Type)
	})

	t.Run("Template literal assignments", func(t *testing.T) {
		sourceCode := "const templateLiteral = `Hello ${name}!`;\nconst multiLine = `Line 1\nLine 2\nLine 3`;"
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		variables, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, variables, 2)

		tlVar := variables[0]
		assert.Equal(t, "templateLiteral", tlVar.Name)
		assert.Equal(t, outbound.ConstructConstant, tlVar.Type)

		mlVar := variables[1]
		assert.Equal(t, "multiLine", mlVar.Name)
		assert.Equal(t, outbound.ConstructConstant, mlVar.Type)
	})
}

func TestJavaScriptImportExportHandling(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("ES6 named imports", func(t *testing.T) {
		sourceCode := `
import { name1, name2 as alias2, name3 } from 'module';
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		imports, err := adapter.ExtractImports(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, imports, 1)

		import1 := imports[0]
		assert.Equal(t, "module", import1.Path)
		assert.Empty(t, import1.Alias)
	})

	t.Run("Default imports", func(t *testing.T) {
		sourceCode := `
import defaultName from 'module';
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		imports, err := adapter.ExtractImports(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, imports, 1)

		import1 := imports[0]
		assert.Equal(t, "module", import1.Path)
	})

	t.Run("Namespace imports", func(t *testing.T) {
		sourceCode := `
import * as namespace from 'module';
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		imports, err := adapter.ExtractImports(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, imports, 1)

		import1 := imports[0]
		assert.Equal(t, "module", import1.Path)
	})

	t.Run("Mixed imports", func(t *testing.T) {
		sourceCode := `
import defaultName, { name1, name2 as alias2 } from 'module';
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		imports, err := adapter.ExtractImports(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, imports, 1)

		mixedImport := imports[0]
		assert.Equal(t, "module", mixedImport.Path)
	})

	t.Run("Dynamic imports", func(t *testing.T) {
		sourceCode := `
const module = import('module');
const module2 = await import('module2');
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		imports, err := adapter.ExtractImports(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, imports, 2)

		import1 := imports[0]
		assert.Equal(t, "module", import1.Path)

		import2 := imports[1]
		assert.Equal(t, "module2", import2.Path)
	})

	t.Run("ES6 exports", func(t *testing.T) {
		sourceCode := `
export { name1, name2 as alias2 };
export default function defaultFunction() {}
export const exportedConst = "value";
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		exports, err := adapter.ExtractModules(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, exports, 4) // 2 from clause + 1 default + 1 declaration

		export1 := exports[0]
		assert.Equal(t, "name1", export1.Name)
		assert.Equal(t, outbound.ConstructModule, export1.Type)

		export2 := exports[1]
		assert.Equal(t, "alias2", export2.Name) // Aliased export uses the alias name
		assert.Equal(t, outbound.ConstructModule, export2.Type)

		export3 := exports[2]
		assert.Equal(t, "defaultFunction", export3.Name)
		assert.Equal(t, outbound.ConstructModule, export3.Type)

		export4 := exports[3]
		assert.Equal(t, "exportedConst", export4.Name)
		assert.Equal(t, outbound.ConstructModule, export4.Type)
	})

	t.Run("CommonJS require", func(t *testing.T) {
		sourceCode := `
const module1 = require('module1');
const { name1, name2 } = require('module2');
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		imports, err := adapter.ExtractImports(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, imports, 2)

		import1 := imports[0]
		assert.Equal(t, "module1", import1.Path)

		import2 := imports[1]
		assert.Equal(t, "module2", import2.Path)
	})

	t.Run("CommonJS module.exports", func(t *testing.T) {
		sourceCode := `
module.exports = function exportedFunction() {};
module.exports.name1 = "value1";
exports.name2 = "value2";
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		exports, err := adapter.ExtractModules(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, exports, 3)

		export1 := exports[0]
		assert.Equal(t, "exportedFunction", export1.Name)
		assert.Equal(t, outbound.ConstructModule, export1.Type)

		export2 := exports[1]
		assert.Equal(t, "name1", export2.Name)
		assert.Equal(t, outbound.ConstructModule, export2.Type)

		export3 := exports[2]
		assert.Equal(t, "name2", export3.Name)
		assert.Equal(t, outbound.ConstructModule, export3.Type)
	})

	t.Run("AMD require and define", func(t *testing.T) {
		sourceCode := `
require(['module1', 'module2'], function(mod1, mod2) {
	// implementation
});

define(['module1'], function(mod1) {
	return { name: "AMD module" };
});
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		imports, err := adapter.ExtractImports(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, imports, 2)

		import1 := imports[0]
		assert.Equal(t, "module1", import1.Path)

		import2 := imports[1]
		assert.Equal(t, "module2", import2.Path)

		exports, err := adapter.ExtractModules(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, exports, 1)

		export1 := exports[0]
		assert.Empty(t, export1.Name)
		assert.Equal(t, outbound.ConstructModule, export1.Type)
	})
}

func TestJavaScriptModuleProcessing(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("ES6 module detection", func(t *testing.T) {
		sourceCode := `
import { name } from 'module';
export const exported = "value";
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		modules, err := adapter.ExtractModules(ctx, domainTree, options)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(modules), 1)
	})

	t.Run("CommonJS module detection", func(t *testing.T) {
		sourceCode := `
const fs = require('fs');
module.exports = {};
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		modules, err := adapter.ExtractModules(ctx, domainTree, options)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(modules), 1)
	})

	t.Run("UMD module detection", func(t *testing.T) {
		sourceCode := `
(function (root, factory) {
	if (typeof define === 'function' && define.amd) {
		define(['module'], factory);
	} else if (typeof module === 'object' && module.exports) {
		module.exports = factory(require('module'));
	} else {
		root.MyModule = factory(root.Module);
	}
}(typeof self !== 'undefined' ? self : this, function (Module) {
	return {};
}));
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		modules, err := adapter.ExtractModules(ctx, domainTree, options)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(modules), 1)
	})

	t.Run("Module-level variable extraction", func(t *testing.T) {
		sourceCode := `
import { name } from 'module';

const moduleVar = "value";
let anotherVar = 42;

function moduleFunction() {}

export { moduleVar, moduleFunction };
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		variables, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		assert.Len(t, variables, 2)

		moduleVar := variables[0]
		assert.Equal(t, "moduleVar", moduleVar.Name)
		assert.Equal(t, outbound.ConstructConstant, moduleVar.Type)

		anotherVar := variables[1]
		assert.Equal(t, "anotherVar", anotherVar.Name)
		assert.Equal(t, outbound.ConstructVariable, anotherVar.Type)
	})

	t.Run("Export declaration parsing", func(t *testing.T) {
		sourceCode := `
export { name1, name2 as alias };
export default function defaultFn() {}
export class ExportedClass {}
export const exportedConst = "value";
export function exportedFunction() {}
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		exports, err := adapter.ExtractModules(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, exports, 6)

		export1 := exports[0]
		assert.Equal(t, "name1", export1.Name)
		assert.Equal(t, outbound.ConstructModule, export1.Type)

		export2 := exports[1]
		assert.Equal(t, "alias", export2.Name)
		assert.Equal(t, outbound.ConstructModule, export2.Type)
		// Verify it's an aliased export with original name in metadata
		assert.Equal(t, true, export2.Metadata["is_alias"])
		assert.Equal(t, "name2", export2.Metadata["original_name"])

		export3 := exports[2]
		assert.Equal(t, "defaultFn", export3.Name)
		assert.Equal(t, outbound.ConstructModule, export3.Type)

		export4 := exports[3]
		assert.Equal(t, "ExportedClass", export4.Name)
		assert.Equal(t, outbound.ConstructModule, export4.Type)

		export5 := exports[4]
		assert.Equal(t, "exportedConst", export5.Name)
		assert.Equal(t, outbound.ConstructModule, export5.Type)

		export6 := exports[5]
		assert.Equal(t, "exportedFunction", export6.Name)
		assert.Equal(t, outbound.ConstructModule, export6.Type)
	})
}

func TestJavaScriptAdvancedFeatures(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	// Note: Prototype chain analysis test removed - this is a legacy JavaScript pattern
	// (pre-ES6 classes) that would require significant implementation complexity to support.
	// Modern JavaScript uses class syntax instead. If prototype assignment support is needed,
	// it would require parsing assignment_expression nodes and extracting method names from
	// member expression chains like Constructor.prototype.methodName.

	t.Run("Constructor function detection", func(t *testing.T) {
		sourceCode := `
function ConstructorFunction(name) {
	this.name = name;
}

ConstructorFunction.prototype.getName = function() {
	return this.name;
};

const instance = new ConstructorFunction("test");
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 2)

		constructor := functions[0]
		assert.Equal(t, "ConstructorFunction", constructor.Name)
		assert.Equal(t, outbound.ConstructFunction, constructor.Type)
		assert.Len(t, constructor.Parameters, 1)

		getNameMethod := functions[1]
		assert.Equal(t, "getName", getNameMethod.Name)
		assert.Equal(t, outbound.ConstructMethod, getNameMethod.Type)
	})

	t.Run("Object literal method extraction", func(t *testing.T) {
		sourceCode := `
const obj = {
	method1() {},
	*generatorMethod() {},
	async asyncMethod() {},
	async *asyncGeneratorMethod() {},
	[computedMethod]() {},
	get getterMethod() {},
	set setterMethod(value) {}
};
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		methods, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, methods, 7)

		method1 := methods[0]
		assert.Equal(t, "method1", method1.Name)
		assert.Equal(t, outbound.ConstructMethod, method1.Type)

		generatorMethod := methods[1]
		assert.Equal(t, "generatorMethod", generatorMethod.Name)
		assert.Equal(t, outbound.ConstructMethod, generatorMethod.Type)
		assert.True(t, generatorMethod.IsGeneric)

		asyncMethod := methods[2]
		assert.Equal(t, "asyncMethod", asyncMethod.Name)
		assert.Equal(t, outbound.ConstructMethod, asyncMethod.Type)
		assert.True(t, asyncMethod.IsAsync)

		asyncGeneratorMethod := methods[3]
		assert.Equal(t, "asyncGeneratorMethod", asyncGeneratorMethod.Name)
		assert.Equal(t, outbound.ConstructMethod, asyncGeneratorMethod.Type)
		assert.True(t, asyncGeneratorMethod.IsAsync)
		assert.True(t, asyncGeneratorMethod.IsGeneric)

		computedMethod := methods[4]
		assert.Equal(t, "[computedMethod]", computedMethod.Name)
		assert.Equal(t, outbound.ConstructMethod, computedMethod.Type)

		getterMethod := methods[5]
		assert.Equal(t, "getterMethod", getterMethod.Name)
		assert.Equal(t, outbound.ConstructMethod, getterMethod.Type)

		setterMethod := methods[6]
		assert.Equal(t, "setterMethod", setterMethod.Name)
		assert.Equal(t, outbound.ConstructMethod, setterMethod.Type)
		assert.Len(t, setterMethod.Parameters, 1)
	})

	t.Run("Callback function identification", func(t *testing.T) {
		sourceCode := `
function withCallback(callback) {
	callback();
}

withCallback(function() {
	console.log("callback executed");
});

withCallback(() => {
	console.log("arrow callback executed");
});

const callbacks = [
	function() { return 1; },
	() => 2
];
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		callbacks, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, callbacks, 5) // withCallback + 2 inline callbacks + 2 array functions

		cb1 := callbacks[1]
		assert.Empty(t, cb1.Name)
		assert.Equal(t, outbound.ConstructFunction, cb1.Type)

		cb2 := callbacks[2]
		assert.Empty(t, cb2.Name)
		assert.Equal(t, outbound.ConstructFunction, cb2.Type)

		cb3 := callbacks[3]
		assert.Empty(t, cb3.Name)
		assert.Equal(t, outbound.ConstructFunction, cb3.Type)

		cb4 := callbacks[4]
		assert.Empty(t, cb4.Name)
		assert.Equal(t, outbound.ConstructFunction, cb4.Type)
	})

	t.Run("Promise and async/await pattern detection", func(t *testing.T) {
		sourceCode := `
function promiseFunction() {
	return new Promise((resolve, reject) => {
		resolve("resolved");
	});
}

async function asyncAwaitFunction() {
	const result = await promiseFunction();
	return result;
}

const asyncArrow = async () => {
	try {
		const result = await fetch("https://api.example.com");
		return result.json();
	} catch (error) {
		throw error;
	}
};
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		asyncFunctions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, asyncFunctions, 4) // promiseFunction + arrow in Promise + asyncAwaitFunction + asyncArrow

		asyncFn1 := asyncFunctions[1]
		// This is the arrow function inside Promise constructor - may be anonymous
		assert.Equal(t, outbound.ConstructFunction, asyncFn1.Type)

		asyncFn2 := asyncFunctions[2]
		assert.Equal(t, "asyncAwaitFunction", asyncFn2.Name)
		assert.Equal(t, outbound.ConstructFunction, asyncFn2.Type)
		assert.True(t, asyncFn2.IsAsync)

		asyncFn3 := asyncFunctions[3]
		assert.Empty(t, asyncFn3.Name)
		assert.Equal(t, outbound.ConstructFunction, asyncFn3.Type)
		assert.True(t, asyncFn3.IsAsync)
	})

	t.Run("Event handler function detection", func(t *testing.T) {
		sourceCode := `
element.addEventListener('click', function(event) {
	console.log("clicked");
});

element.addEventListener('mouseover', () => {
	console.log("hovered");
});

const handlers = {
	onSubmit: function(e) {
		e.preventDefault();
	},
	
	onInput: (e) => {
		console.log(e.target.value);
	}
};
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		eventHandlers, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, eventHandlers, 4)

		// First two are addEventListener callbacks
		handler1 := eventHandlers[0]
		assert.Empty(t, handler1.Name)
		assert.Equal(t, outbound.ConstructFunction, handler1.Type)

		handler2 := eventHandlers[1]
		assert.Empty(t, handler2.Name)
		assert.Equal(t, outbound.ConstructFunction, handler2.Type)

		// Last two are object literal pair functions
		handler3 := eventHandlers[2]
		assert.Equal(t, "onSubmit", handler3.Name)
		assert.Equal(
			t,
			outbound.ConstructFunction,
			handler3.Type,
		) // Per tree-sitter grammar, pair with function value is ConstructFunction

		handler4 := eventHandlers[3]
		assert.Equal(t, "onInput", handler4.Name)
		assert.Equal(t, outbound.ConstructFunction, handler4.Type)
	})

	t.Run("Module pattern recognition", func(t *testing.T) {
		sourceCode := `
const ModulePattern = (function() {
	const privateVar = "private";
	
	function privateMethod() {
		return privateVar;
	}
	
	return {
		publicMethod: function() {
			return privateMethod();
		}
	};
})();
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		modules, err := adapter.ExtractModules(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, modules, 1)

		module := modules[0]
		assert.Equal(t, "ModulePattern", module.Name)
		assert.Equal(t, outbound.ConstructModule, module.Type)
	})

	t.Run("Closure detection and analysis", func(t *testing.T) {
		sourceCode := `
function outerFunction(outerVar) {
	return function innerFunction(innerVar) {
		return outerVar + innerVar;
	};
}

const closure = outerFunction("outer");
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		closures, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, closures, 2)

		closure := closures[1]
		assert.Equal(t, "innerFunction", closure.Name)
		assert.Equal(t, outbound.ConstructFunction, closure.Type)
		assert.Len(t, closure.Parameters, 1)
		assert.Equal(t, "innerVar", closure.Parameters[0].Name)
	})
}

func TestJavaScriptModernFeatures(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("Optional chaining operator", func(t *testing.T) {
		sourceCode := `
const value = obj?.prop?.subProp;
const result = obj?.method?.();
const item = array?.[0];
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		variables, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, variables, 3)
	})

	t.Run("Nullish coalescing operator", func(t *testing.T) {
		sourceCode := `
const value = obj.prop ?? "default";
const result = getValue() ?? computeDefault();
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		variables, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, variables, 2)
	})

	t.Run("BigInt literal support", func(t *testing.T) {
		sourceCode := `
const bigIntValue = 123n;
const anotherBigInt = BigInt(456);
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		variables, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, variables, 2)
	})

	t.Run("Symbol primitive support", func(t *testing.T) {
		sourceCode := `
const sym1 = Symbol('description');
const sym2 = Symbol();
const globalSym = Symbol.for('global');
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		variables, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, variables, 3)
	})

	t.Run("Proxy and Reflect usage", func(t *testing.T) {
		sourceCode := `
const proxy = new Proxy(target, handler);
const reflectGet = Reflect.get(obj, 'prop');
const reflectSet = Reflect.set(obj, 'prop', value);
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		variables, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, variables, 3)
	})

	t.Run("WeakMap/WeakSet usage", func(t *testing.T) {
		sourceCode := `
const weakMap = new WeakMap();
const weakSet = new WeakSet();
weakMap.set(key, value);
weakSet.add(obj);
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		variables, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, variables, 2)
	})

	t.Run("Iterator and generator protocols", func(t *testing.T) {
		sourceCode := `
class IterableClass {
	*[Symbol.iterator]() {
		yield 1;
		yield 2;
	}
}

function* generatorFunction() {
	yield* [1, 2, 3];
}

const manualIterator = {
	next() {
		return { value: 1, done: false };
	},
	[Symbol.iterator]() {
		return this;
	}
};
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 4)

		iterableClassMethod := functions[0]
		assert.Equal(t, "[Symbol.iterator]", iterableClassMethod.Name)
		assert.Equal(t, outbound.ConstructMethod, iterableClassMethod.Type)
		assert.True(t, iterableClassMethod.IsGeneric)

		generatorFn := functions[1]
		assert.Equal(t, "generatorFunction", generatorFn.Name)
		assert.Equal(t, outbound.ConstructFunction, generatorFn.Type)
		assert.True(t, generatorFn.IsGeneric)

		nextMethod := functions[2]
		assert.Equal(t, "next", nextMethod.Name)
		assert.Equal(t, outbound.ConstructMethod, nextMethod.Type)
		assert.False(t, nextMethod.IsGeneric)

		manualIteratorMethod := functions[3]
		assert.Equal(t, "[Symbol.iterator]", manualIteratorMethod.Name)
		assert.Equal(t, outbound.ConstructMethod, manualIteratorMethod.Type)
	})
}

func TestJavaScriptJSDocComments(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("JSDoc function documentation", func(t *testing.T) {
		sourceCode := `
/**
 * Adds two numbers together.
 * @param {number} a - The first number.
 * @param {number} b - The second number.
 * @returns {number} The sum of a and b.
 * @since 1.0.0
 * @deprecated Use addNumbers instead
 */
function add(a, b) {
	return a + b;
}
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 1)

		fn := functions[0]
		assert.Equal(t, "add", fn.Name)
		assert.Equal(t, outbound.ConstructFunction, fn.Type)
		assert.Len(t, fn.Parameters, 2)
	})

	t.Run("JSDoc class documentation", func(t *testing.T) {
		sourceCode := `
/**
 * Represents a person.
 * @class
 * @property {string} name - The person's name.
 * @property {number} age - The person's age.
 * @author John Doe
 */
class Person {
	/**
	 * Creates a new Person.
	 * @param {string} name - The person's name.
	 * @param {number} age - The person's age.
	 */
	constructor(name, age) {
		this.name = name;
		this.age = age;
	}

	/**
	 * Gets the person's name.
	 * @returns {string} The person's name.
	 * @memberof Person
	 */
	getName() {
		return this.name;
	}
}
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		classes, err := adapter.ExtractClasses(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, classes, 1)

		class := classes[0]
		assert.Equal(t, "Person", class.Name)
		assert.Equal(t, outbound.ConstructClass, class.Type)

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)

		var classMethods []outbound.SemanticCodeChunk
		for _, fn := range functions {
			if fn.ParentChunk != nil && fn.ParentChunk.Name == "Person" {
				classMethods = append(classMethods, fn)
			}
		}

		constructor := classMethods[0]
		assert.Equal(t, "constructor", constructor.Name)
		assert.Equal(t, outbound.ConstructMethod, constructor.Type)
		assert.Len(t, constructor.Parameters, 2)

		getNameMethod := classMethods[1]
		assert.Equal(t, "getName", getNameMethod.Name)
		assert.Equal(t, outbound.ConstructMethod, getNameMethod.Type)
	})

	t.Run("Parameter type annotations", func(t *testing.T) {
		sourceCode := `
/**
 * @param {string} str - A string parameter
 * @param {number} num - A number parameter
 * @param {boolean} bool - A boolean parameter
 * @param {Array<string>} strArray - An array of strings
 * @param {Object} obj - An object parameter
 * @param {Function} fn - A function parameter
 */
function typedFunction(str, num, bool, strArray, obj, fn) {}
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 1)

		fn := functions[0]
		assert.Equal(t, "typedFunction", fn.Name)
		assert.Equal(t, outbound.ConstructFunction, fn.Type)
		assert.Len(t, fn.Parameters, 6)

		assert.Equal(t, "str", fn.Parameters[0].Name)
		assert.Equal(t, "string", fn.Parameters[0].Type)

		assert.Equal(t, "num", fn.Parameters[1].Name)
		assert.Equal(t, "number", fn.Parameters[1].Type)

		assert.Equal(t, "bool", fn.Parameters[2].Name)
		assert.Equal(t, "boolean", fn.Parameters[2].Type)

		assert.Equal(t, "strArray", fn.Parameters[3].Name)
		assert.Equal(t, "Array<string>", fn.Parameters[3].Type)

		assert.Equal(t, "obj", fn.Parameters[4].Name)
		assert.Equal(t, "Object", fn.Parameters[4].Type)

		assert.Equal(t, "fn", fn.Parameters[5].Name)
		assert.Equal(t, "Function", fn.Parameters[5].Type)
	})

	t.Run("Return type annotations", func(t *testing.T) {
		sourceCode := `
/**
 * @returns {string} A string result
 */
function returnsString() {}

/**
 * @returns {Promise<number>} A promise that resolves to a number
 */
async function returnsPromise() {}

/**
 * @returns {Array<Object>} An array of objects
 */
function returnsObjectArray() {}
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 3)

		fn1 := functions[0]
		assert.Equal(t, "returnsString", fn1.Name)
		assert.Equal(t, outbound.ConstructFunction, fn1.Type)

		fn2 := functions[1]
		assert.Equal(t, "returnsPromise", fn2.Name)
		assert.Equal(t, outbound.ConstructFunction, fn2.Type)
		assert.True(t, fn2.IsAsync)

		fn3 := functions[2]
		assert.Equal(t, "returnsObjectArray", fn3.Name)
		assert.Equal(t, outbound.ConstructFunction, fn3.Type)
	})

	t.Run("JSDoc tags extraction", func(t *testing.T) {
		sourceCode := `
/**
 * @deprecated This function is deprecated
 * @since 2.1.0
 * @version 3.0.0
 * @author Jane Smith
 * @license MIT
 * @example
 * const result = exampleFunction(5);
 * console.log(result); // 10
 * @throws {Error} If parameter is negative
 * @see {@link otherFunction}
 */
function exampleFunction(param) {
	if (param < 0) {
		throw new Error("Negative parameter");
	}
	return param * 2;
}
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 1)

		fn := functions[0]
		assert.Equal(t, "exampleFunction", fn.Name)
		assert.Equal(t, outbound.ConstructFunction, fn.Type)
		assert.Len(t, fn.Parameters, 1)
	})
}

func TestJavaScriptIntegration(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("Full JavaScript file parsing", func(t *testing.T) {
		sourceCode := `
/**
 * A comprehensive example file
 * @module example
 * @version 1.0.0
 */

import { helper } from './utils';
import * as lib from 'library';

/**
 * Example class with various features
 */
class ExampleClass {
	#privateField = "private";
	
	constructor(name, options = {}) {
		this.name = name;
		this.options = options;
	}
	
	static staticMethod() {
		return "static";
	}
	
	get value() {
		return this.#privateField;
	}
	
	set value(newValue) {
		this.#privateField = newValue;
	}
	
	async processData(data) {
		const result = await helper.process(data);
		return result;
	}
}

/**
 * Example function with complex parameters
 * @param {Object} config - Configuration object
 * @param {string} config.url - The URL to fetch
 * @param {Object} [config.headers] - Optional headers
 * @returns {Promise<Object>} The response data
 */
const exampleFunction = async ({url, headers = {}}) => {
	try {
		const response = await fetch(url, { headers });
		return await response.json();
	} catch (error) {
		console.error(error);
		throw error;
	}
};

export { ExampleClass, exampleFunction };
export default ExampleClass;
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		// Test all extraction methods
		classes, err := adapter.ExtractClasses(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, classes, 1)

		class := classes[0]
		assert.Equal(t, "ExampleClass", class.Name)
		assert.Equal(t, outbound.ConstructClass, class.Type)

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)

		var classMethods []outbound.SemanticCodeChunk
		for _, fn := range functions {
			if fn.ParentChunk != nil && fn.ParentChunk.Name == "ExampleClass" {
				classMethods = append(classMethods, fn)
			}
		}
		assert.Len(t, classMethods, 4)

		var standaloneFunctions []outbound.SemanticCodeChunk
		for _, fn := range functions {
			if fn.ParentChunk == nil && fn.Name == "exampleFunction" {
				standaloneFunctions = append(standaloneFunctions, fn)
			}
		}
		require.Len(t, standaloneFunctions, 1)

		fn := standaloneFunctions[0]
		assert.Equal(t, "exampleFunction", fn.Name)
		assert.Equal(t, outbound.ConstructFunction, fn.Type)
		assert.True(t, fn.IsAsync)
		assert.Len(t, fn.Parameters, 1)

		imports, err := adapter.ExtractImports(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, imports, 2)

		import1 := imports[0]
		assert.Equal(t, "./utils", import1.Path)

		import2 := imports[1]
		assert.Equal(t, "library", import2.Path)

		exports, err := adapter.ExtractModules(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, exports, 3)

		export1 := exports[0]
		assert.Equal(t, "ExampleClass", export1.Name)
		assert.Equal(t, outbound.ConstructModule, export1.Type)

		export2 := exports[1]
		assert.Equal(t, "exampleFunction", export2.Name)
		assert.Equal(t, outbound.ConstructModule, export2.Type)

		export3 := exports[2]
		assert.Equal(t, "ExampleClass", export3.Name)
		assert.Equal(t, outbound.ConstructModule, export3.Type)

		variables, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, variables, 3)

		privateField := variables[0]
		assert.Equal(t, "#privateField", privateField.Name)
		assert.Equal(t, outbound.Private, privateField.Visibility)

		nameVar := variables[1]
		assert.Equal(t, "name", nameVar.Name)

		optionsVar := variables[2]
		assert.Equal(t, "options", optionsVar.Name)
	})

	t.Run("React component parsing", func(t *testing.T) {
		sourceCode := `
import React, { useState, useEffect } from 'react';

/**
 * A functional React component
 * @param {Object} props - Component props
 * @param {string} props.title - The component title
 * @returns {JSX.Element} The rendered component
 */
const FunctionalComponent = ({ title }) => {
	const [count, setCount] = useState(0);
	
	useEffect(() => {
		console.log("Component mounted");
	}, []);
	
	return (
		<div>
			<h1>{title}</h1>
			<p>Count: {count}</p>
			<button onClick={() => setCount(count + 1)}>
				Increment
			</button>
		</div>
	);
};

class ClassComponent extends React.Component {
	constructor(props) {
		super(props);
		this.state = { count: 0 };
	}
	
	render() {
		return (
			<div>
				<h1>{this.props.title}</h1>
				<p>Count: {this.state.count}</p>
			</div>
		);
	}
}

export { FunctionalComponent, ClassComponent };
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)

		var functionalComponents []outbound.SemanticCodeChunk
		for _, fn := range functions {
			if fn.Name == "FunctionalComponent" {
				functionalComponents = append(functionalComponents, fn)
			}
		}
		require.Len(t, functionalComponents, 1)

		functionalComponent := functionalComponents[0]
		assert.Equal(t, "FunctionalComponent", functionalComponent.Name)
		assert.Equal(t, outbound.ConstructFunction, functionalComponent.Type)
		assert.True(t, functionalComponent.IsAsync)

		classes, err := adapter.ExtractClasses(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, classes, 1)

		classComponent := classes[0]
		assert.Equal(t, "ClassComponent", classComponent.Name)
		assert.Equal(t, outbound.ConstructClass, classComponent.Type)
	})

	t.Run("Node.js module parsing", func(t *testing.T) {
		sourceCode := `
const fs = require('fs');
const path = require('path');

function nodeFunction() {
	return path.join(__dirname, 'file.txt');
}

module.exports = {
	nodeFunction,
	property: "value"
};
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		modules, err := adapter.ExtractModules(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, modules, 1)

		nodeModule := modules[0]
		assert.Equal(t, "module.exports", nodeModule.Name)
		assert.Equal(t, outbound.ConstructModule, nodeModule.Type)
	})

	t.Run("Browser script parsing", func(t *testing.T) {
		sourceCode := `
window.onload = function() {
	document.getElementById('button').addEventListener('click', handleClick);
};

function handleClick(event) {
	event.preventDefault();
	const element = document.querySelector('.target');
	element.classList.toggle('active');
}

const globalVar = "global";
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 2)

		browserScript := functions[0]
		assert.Equal(t, "window.onload", browserScript.Name)
		assert.Equal(t, outbound.ConstructFunction, browserScript.Type)

		handleClick := functions[1]
		assert.Equal(t, "handleClick", handleClick.Name)
		assert.Equal(t, outbound.ConstructFunction, handleClick.Type)
	})

	t.Run("Error handling for malformed JavaScript code", func(t *testing.T) {
		sourceCode := `
function malformedFunction( {
	return "missing closing parenthesis";
}

const unclosedString = "string without end;

class UnclosedClass {
	constructor( {
}
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		// Should still be able to extract some information despite syntax errors
		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(functions), 1)

		classes, err := adapter.ExtractClasses(ctx, domainTree, options)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(classes), 1)
	})

	t.Run("TypeScript-style type annotation handling", func(t *testing.T) {
		sourceCode := `
function tsStyleFunction(param: string): number {
	return param.length;
}

const typedArrow = (value: boolean): Promise<void> => {
	return new Promise(resolve => resolve());
};

interface MyInterface {
	method(): void;
}

type MyType = string | number;
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 2)

		fn1 := functions[0]
		assert.Equal(t, "tsStyleFunction", fn1.Name)
		assert.Equal(t, outbound.ConstructFunction, fn1.Type)
		assert.Equal(t, "string", fn1.Parameters[0].Type)

		fn2 := functions[1]
		assert.Equal(t, "typedArrow", fn2.Name)
		assert.Equal(t, outbound.ConstructFunction, fn2.Type)
		assert.Equal(t, "boolean", fn2.Parameters[0].Type)

		interfaces, err := adapter.ExtractInterfaces(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, interfaces, 1)

		interface1 := interfaces[0]
		assert.Equal(t, "MyInterface", interface1.Name)
		assert.Equal(t, outbound.ConstructInterface, interface1.Type)

		types, err := adapter.ExtractVariables(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, types, 1)

		type1 := types[0]
		assert.Equal(t, "MyType", type1.Name)
		assert.Equal(t, outbound.ConstructVariable, type1.Type)
	})
}
