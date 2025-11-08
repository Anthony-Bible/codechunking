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

// RED PHASE TESTS: These tests are EXPECTED TO FAIL until implementation is complete
// These tests define the expected behavior for JavaScript parser fixes identified in
// comprehensive_javascript_parser_test.go

func TestObjectLiteralPairWithFunctionValue_RedPhase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping RED phase test in short mode")
	}

	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("Object literal pair with function expression value", func(t *testing.T) {
		sourceCode := `
const handlers = {
	onSubmit: function(e) {
		e.preventDefault();
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
		require.NoError(t, err, "ExtractFunctions should not error")
		require.Len(t, functions, 1, "Should extract exactly one function")

		fn := functions[0]

		// RED PHASE ASSERTION: Per tree-sitter grammar, this should be ConstructFunction, NOT ConstructMethod
		// The key distinction is: method_definition (syntax: methodName() {}) vs pair (syntax: key: function() {})
		assert.Equal(t, outbound.ConstructFunction, fn.Type,
			"Pair node with function value should be ConstructFunction (not ConstructMethod)")

		// The function should have metadata indicating it's assigned to a property
		assert.NotEmpty(t, fn.Metadata, "Function should have metadata")
		assert.Contains(t, fn.Metadata, "assigned_to", "Metadata should contain 'assigned_to' field")
		assert.Equal(t, "onSubmit", fn.Metadata["assigned_to"], "Should track the property name it's assigned to")

		// Or alternatively, the name could be set to the property name
		// (implementation can choose either approach - metadata or name)
		if fn.Name != "" {
			assert.Equal(t, "onSubmit", fn.Name, "Function name could be set to property name")
		}

		// Should extract parameter
		require.Len(t, fn.Parameters, 1, "Function should have one parameter")
		assert.Equal(t, "e", fn.Parameters[0].Name)
	})

	t.Run("Object literal pair with arrow function value", func(t *testing.T) {
		sourceCode := `
const handlers = {
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

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, functions, 1, "Should extract the arrow function")

		fn := functions[0]

		// RED PHASE ASSERTION: Arrow function in pair should be ConstructFunction
		assert.Equal(t, outbound.ConstructFunction, fn.Type,
			"Arrow function in pair should be ConstructFunction (not ConstructMethod)")

		// Should have metadata or name indicating assignment
		if fn.Name != "" {
			assert.Equal(t, "onInput", fn.Name, "Function name could be property name")
		} else {
			assert.Contains(t, fn.Metadata, "assigned_to", "Should track assigned property")
		}

		require.Len(t, fn.Parameters, 1, "Arrow function should have parameter")
		assert.Equal(t, "e", fn.Parameters[0].Name)
	})

	t.Run("Event handler test from failing suite", func(t *testing.T) {
		// This replicates the exact test case that's failing
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

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)

		// RED PHASE ASSERTION: Currently gets 2, expects 4
		// Should extract: 2 inline event handlers + 2 object literal functions
		require.Len(t, functions, 4, "Should extract all 4 functions")

		// First two are inline callbacks to addEventListener
		fn1 := functions[0]
		assert.Empty(t, fn1.Name, "First callback should be anonymous")
		assert.Equal(t, outbound.ConstructFunction, fn1.Type)
		require.Len(t, fn1.Parameters, 1)
		assert.Equal(t, "event", fn1.Parameters[0].Name)

		fn2 := functions[1]
		assert.Empty(t, fn2.Name, "Second callback should be anonymous")
		assert.Equal(t, outbound.ConstructFunction, fn2.Type)

		// Last two are from object literal pairs
		fn3 := functions[2]
		// RED PHASE: This is currently ConstructMethod but should be ConstructFunction
		assert.Equal(t, outbound.ConstructFunction, fn3.Type,
			"Object literal pair with function value should be ConstructFunction")
		if fn3.Name != "" {
			assert.Equal(t, "onSubmit", fn3.Name)
		}

		fn4 := functions[3]
		assert.Equal(t, outbound.ConstructFunction, fn4.Type,
			"Object literal pair with arrow function should be ConstructFunction")
		if fn4.Name != "" {
			assert.Equal(t, "onInput", fn4.Name)
		}
	})

	t.Run("Distinguish method_definition from pair with function", func(t *testing.T) {
		sourceCode := `
const obj = {
	// This is a method_definition - should be ConstructMethod
	methodSyntax() {
		return "method";
	},

	// This is a pair with function value - should be ConstructFunction
	functionSyntax: function() {
		return "function";
	},

	// This is also a pair with arrow function - should be ConstructFunction
	arrowSyntax: () => {
		return "arrow";
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
		require.Len(t, functions, 3, "Should extract all three function-like constructs")

		// First one uses method syntax - should be ConstructMethod
		method := functions[0]
		assert.Equal(t, "methodSyntax", method.Name)
		assert.Equal(t, outbound.ConstructMethod, method.Type,
			"method_definition node should be ConstructMethod")

		// Second one is pair:function - should be ConstructFunction
		funcPair := functions[1]
		assert.Equal(t, outbound.ConstructFunction, funcPair.Type,
			"pair node with function_expression should be ConstructFunction")

		// Third one is pair:arrow - should be ConstructFunction
		arrowPair := functions[2]
		assert.Equal(t, outbound.ConstructFunction, arrowPair.Type,
			"pair node with arrow_function should be ConstructFunction")
	})
}

func TestNestedFunctionsInComplexStructures_RedPhase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping RED phase test in short mode")
	}

	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("Array of objects with function properties", func(t *testing.T) {
		sourceCode := `
const routes = [
	{
		handler: function(req, res) {
			res.send('GET');
		}
	},
	{
		handler: (req, res) => {
			res.send('POST');
		}
	}
];
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)

		// RED PHASE ASSERTION: Should extract both handlers from array
		require.Len(t, functions, 2, "Should extract both handler functions from array of objects")

		for i, fn := range functions {
			assert.Equal(t, outbound.ConstructFunction, fn.Type,
				"Handler %d should be ConstructFunction (pair syntax)", i)
			require.Len(t, fn.Parameters, 2, "Handler %d should have 2 parameters", i)
			assert.Equal(t, "req", fn.Parameters[0].Name)
			assert.Equal(t, "res", fn.Parameters[1].Name)
		}
	})

	t.Run("Object with array of functions", func(t *testing.T) {
		sourceCode := `
const middleware = {
	validators: [
		function validateEmail(email) {
			return email.includes('@');
		},
		function validatePassword(pwd) {
			return pwd.length >= 8;
		}
	]
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

		// RED PHASE ASSERTION: Should extract both validator functions
		require.Len(t, functions, 2, "Should extract both validator functions from nested array")

		assert.Equal(t, "validateEmail", functions[0].Name)
		assert.Equal(t, outbound.ConstructFunction, functions[0].Type)

		assert.Equal(t, "validatePassword", functions[1].Name)
		assert.Equal(t, outbound.ConstructFunction, functions[1].Type)
	})
}

func TestEdgeCases_RedPhase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping RED phase test in short mode")
	}

	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("Empty array should extract nothing", func(t *testing.T) {
		sourceCode := `
const empty = [];
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err)
		assert.Empty(t, functions, "Empty array should not extract any functions")
	})

	t.Run("Array with non-function values should not error", func(t *testing.T) {
		sourceCode := `
const mixed = [
	1,
	"string",
	function() { return true; },
	null,
	{ key: "value" }
];
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		functions, err := adapter.ExtractFunctions(ctx, domainTree, options)
		require.NoError(t, err, "Mixed array should not cause error")

		// Should only extract the one function
		require.Len(t, functions, 1, "Should extract only the function from mixed array")
		assert.Equal(t, outbound.ConstructFunction, functions[0].Type)
	})

	t.Run("Setter with destructured parameter", func(t *testing.T) {
		sourceCode := `
const obj = {
	set config({ timeout, retries }) {
		this._timeout = timeout;
		this._retries = retries;
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
		require.Len(t, functions, 1)

		setter := functions[0]
		assert.Equal(t, "config", setter.Name)

		// RED PHASE: Should extract the destructured parameter
		require.Len(t, setter.Parameters, 1, "Setter with destructured param should have 1 parameter")
		// Destructured parameters have empty name per extractSingleParameter implementation
		assert.Empty(t, setter.Parameters[0].Name, "Destructured parameter has empty name")
	})
}
