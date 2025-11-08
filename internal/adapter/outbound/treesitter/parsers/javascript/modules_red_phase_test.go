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

// RED PHASE: Tests for IIFE (Immediately Invoked Function Expression) Module Pattern Detection
// These tests are expected to FAIL initially as the implementation does not yet exist.
//
// IIFE Module Pattern characteristics:
// 1. Variable declarator (const/var/let ModuleName = ...)
// 2. Value is a call_expression ((...)(args))
// 3. The function being called is a function_expression wrapped in parentheses
// 4. The IIFE returns an object (the module's public interface)
//
// These tests define the expected behavior for IIFE module pattern recognition.

func TestIIFEModulePattern_BasicPattern(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("Basic IIFE module pattern with const", func(t *testing.T) {
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
		require.Len(t, modules, 1, "Should extract exactly 1 IIFE module")

		module := modules[0]
		assert.Equal(t, "ModulePattern", module.Name, "Module name should be 'ModulePattern'")
		assert.Equal(t, outbound.ConstructModule, module.Type, "Module type should be ConstructModule")
	})

	t.Run("IIFE module pattern with var", func(t *testing.T) {
		sourceCode := `
var LegacyModule = (function() {
	var privateData = 123;

	return {
		getData: function() {
			return privateData;
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
		require.Len(t, modules, 1, "Should extract exactly 1 IIFE module")

		module := modules[0]
		assert.Equal(t, "LegacyModule", module.Name, "Module name should be 'LegacyModule'")
		assert.Equal(t, outbound.ConstructModule, module.Type, "Module type should be ConstructModule")
	})

	t.Run("IIFE module pattern with let", func(t *testing.T) {
		sourceCode := `
let ModernModule = (function() {
	let counter = 0;

	return {
		increment: () => ++counter,
		getCount: () => counter
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
		require.Len(t, modules, 1, "Should extract exactly 1 IIFE module")

		module := modules[0]
		assert.Equal(t, "ModernModule", module.Name, "Module name should be 'ModernModule'")
		assert.Equal(t, outbound.ConstructModule, module.Type, "Module type should be ConstructModule")
	})
}

func TestIIFEModulePattern_NotModulePatterns(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("IIFE without return - not a module", func(t *testing.T) {
		sourceCode := `
const notAModule = (function() {
	console.log("init");
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
		assert.Empty(t, modules, "Should extract 0 modules (IIFE has no return value)")
	})

	t.Run("IIFE with non-object return - not a module", func(t *testing.T) {
		sourceCode := `
const result = (function() {
	return 42;
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
		assert.Empty(t, modules, "Should extract 0 modules (returns primitive, not object)")
	})

	t.Run("IIFE with string return - not a module", func(t *testing.T) {
		sourceCode := `
const stringResult = (function() {
	return "just a string";
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
		assert.Empty(t, modules, "Should extract 0 modules (returns string, not object)")
	})

	t.Run("IIFE with boolean return - not a module", func(t *testing.T) {
		sourceCode := `
const boolResult = (function() {
	return true;
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
		assert.Empty(t, modules, "Should extract 0 modules (returns boolean, not object)")
	})

	t.Run("IIFE with array return - not a module", func(t *testing.T) {
		sourceCode := `
const arrayResult = (function() {
	return [1, 2, 3];
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
		assert.Empty(t, modules, "Should extract 0 modules (returns array, not object literal)")
	})

	t.Run("Regular variable assignment - not a module", func(t *testing.T) {
		sourceCode := `
const regularVar = { method: function() {} };
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		modules, err := adapter.ExtractModules(ctx, domainTree, options)
		require.NoError(t, err)
		assert.Empty(t, modules, "Should extract 0 modules (not an IIFE, just object assignment)")
	})
}

func TestIIFEModulePattern_ComplexPatterns(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("IIFE module with multiple methods", func(t *testing.T) {
		sourceCode := `
const Utils = (function() {
	return {
		add: function(a, b) { return a + b; },
		subtract: (a, b) => a - b,
		multiply: function(a, b) { return a * b; },
		divide: (a, b) => a / b
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
		require.Len(t, modules, 1, "Should extract exactly 1 IIFE module")

		module := modules[0]
		assert.Equal(t, "Utils", module.Name, "Module name should be 'Utils'")
		assert.Equal(t, outbound.ConstructModule, module.Type, "Module type should be ConstructModule")
	})

	t.Run("IIFE module with parameters", func(t *testing.T) {
		sourceCode := `
const ConfigModule = (function(config) {
	const settings = config || {};

	return {
		get: function(key) {
			return settings[key];
		},
		set: function(key, value) {
			settings[key] = value;
		}
	};
})({ debug: true, port: 3000 });
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		modules, err := adapter.ExtractModules(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, modules, 1, "Should extract exactly 1 IIFE module with parameters")

		module := modules[0]
		assert.Equal(t, "ConfigModule", module.Name, "Module name should be 'ConfigModule'")
		assert.Equal(t, outbound.ConstructModule, module.Type, "Module type should be ConstructModule")
	})

	t.Run("IIFE module with dependency injection", func(t *testing.T) {
		sourceCode := `
const DependencyModule = (function($, _) {
	return {
		findElement: function(selector) {
			return $(selector);
		},
		processArray: function(arr) {
			return _.map(arr, x => x * 2);
		}
	};
})(jQuery, lodash);
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		modules, err := adapter.ExtractModules(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, modules, 1, "Should extract exactly 1 IIFE module with dependencies")

		module := modules[0]
		assert.Equal(t, "DependencyModule", module.Name, "Module name should be 'DependencyModule'")
		assert.Equal(t, outbound.ConstructModule, module.Type, "Module type should be ConstructModule")
	})
}

func TestIIFEModulePattern_NestedIIFE(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("Nested IIFE - only outer should be module", func(t *testing.T) {
		sourceCode := `
const Outer = (function() {
	const inner = (function() {
		return { nested: true };
	})();

	return {
		getInner: function() { return inner; }
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
		require.Len(t, modules, 1, "Should extract exactly 1 module (outer IIFE only)")

		module := modules[0]
		assert.Equal(t, "Outer", module.Name, "Module name should be 'Outer'")
		assert.Equal(t, outbound.ConstructModule, module.Type, "Module type should be ConstructModule")
	})

	t.Run("Multiple independent IIFE modules", func(t *testing.T) {
		sourceCode := `
const Module1 = (function() {
	return {
		name: "module1"
	};
})();

const Module2 = (function() {
	return {
		name: "module2"
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
		require.Len(t, modules, 2, "Should extract 2 independent IIFE modules")

		module1 := modules[0]
		assert.Equal(t, "Module1", module1.Name, "First module name should be 'Module1'")
		assert.Equal(t, outbound.ConstructModule, module1.Type, "Module type should be ConstructModule")

		module2 := modules[1]
		assert.Equal(t, "Module2", module2.Name, "Second module name should be 'Module2'")
		assert.Equal(t, outbound.ConstructModule, module2.Type, "Module type should be ConstructModule")
	})
}

func TestIIFEModulePattern_ArrowFunctionIIFE(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("Arrow function IIFE module pattern", func(t *testing.T) {
		sourceCode := `
const ArrowModule = (() => {
	const privateValue = 42;

	return {
		getValue: () => privateValue
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
		require.Len(t, modules, 1, "Should extract exactly 1 arrow function IIFE module")

		module := modules[0]
		assert.Equal(t, "ArrowModule", module.Name, "Module name should be 'ArrowModule'")
		assert.Equal(t, outbound.ConstructModule, module.Type, "Module type should be ConstructModule")
	})

	t.Run("Arrow function IIFE with parameters", func(t *testing.T) {
		sourceCode := `
const ArrowModuleWithParams = ((name) => {
	return {
		getName: () => name,
		setName: (newName) => name = newName
	};
})("initial");
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		modules, err := adapter.ExtractModules(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, modules, 1, "Should extract exactly 1 arrow function IIFE module with parameters")

		module := modules[0]
		assert.Equal(t, "ArrowModuleWithParams", module.Name, "Module name should be 'ArrowModuleWithParams'")
		assert.Equal(t, outbound.ConstructModule, module.Type, "Module type should be ConstructModule")
	})
}

func TestIIFEModulePattern_RealWorldExamples(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("Revealing module pattern", func(t *testing.T) {
		sourceCode := `
const RevealingModule = (function() {
	let privateCounter = 0;

	function privateIncrement() {
		privateCounter++;
	}

	function privateDecrement() {
		privateCounter--;
	}

	function publicGetCounter() {
		return privateCounter;
	}

	function publicIncrement() {
		privateIncrement();
	}

	function publicDecrement() {
		privateDecrement();
	}

	return {
		increment: publicIncrement,
		decrement: publicDecrement,
		getCounter: publicGetCounter
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
		require.Len(t, modules, 1, "Should extract revealing module pattern")

		module := modules[0]
		assert.Equal(t, "RevealingModule", module.Name, "Module name should be 'RevealingModule'")
		assert.Equal(t, outbound.ConstructModule, module.Type, "Module type should be ConstructModule")
	})

	t.Run("Singleton pattern with IIFE", func(t *testing.T) {
		sourceCode := `
const Singleton = (function() {
	let instance;

	function createInstance() {
		return {
			data: "singleton data"
		};
	}

	return {
		getInstance: function() {
			if (!instance) {
				instance = createInstance();
			}
			return instance;
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
		require.Len(t, modules, 1, "Should extract singleton IIFE pattern")

		module := modules[0]
		assert.Equal(t, "Singleton", module.Name, "Module name should be 'Singleton'")
		assert.Equal(t, outbound.ConstructModule, module.Type, "Module type should be ConstructModule")
	})

	t.Run("Namespace pattern with IIFE", func(t *testing.T) {
		sourceCode := `
const MyApp = (function() {
	const version = "1.0.0";

	return {
		utils: {
			format: function(str) { return str.toUpperCase(); }
		},
		models: {
			User: function(name) { this.name = name; }
		},
		getVersion: function() { return version; }
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
		require.Len(t, modules, 1, "Should extract namespace IIFE pattern")

		module := modules[0]
		assert.Equal(t, "MyApp", module.Name, "Module name should be 'MyApp'")
		assert.Equal(t, outbound.ConstructModule, module.Type, "Module type should be ConstructModule")
	})
}

func TestIIFEModulePattern_EdgeCases(t *testing.T) {
	ctx := context.Background()
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	t.Run("IIFE with empty object return", func(t *testing.T) {
		sourceCode := `
const EmptyModule = (function() {
	return {};
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
		require.Len(t, modules, 1, "Should extract IIFE module even with empty object")

		module := modules[0]
		assert.Equal(t, "EmptyModule", module.Name, "Module name should be 'EmptyModule'")
		assert.Equal(t, outbound.ConstructModule, module.Type, "Module type should be ConstructModule")
	})

	t.Run("IIFE with object created from constructor", func(t *testing.T) {
		sourceCode := `
const ConstructorModule = (function() {
	function MyClass() {
		this.value = 42;
	}

	return new MyClass();
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
		require.Len(t, modules, 1, "Should extract IIFE module with object from constructor")

		module := modules[0]
		assert.Equal(t, "ConstructorModule", module.Name, "Module name should be 'ConstructorModule'")
		assert.Equal(t, outbound.ConstructModule, module.Type, "Module type should be ConstructModule")
	})

	t.Run("IIFE with conditional return", func(t *testing.T) {
		sourceCode := `
const ConditionalModule = (function(flag) {
	if (flag) {
		return {
			type: "flagged"
		};
	} else {
		return {
			type: "unflagged"
		};
	}
})(true);
`
		domainTree := createMockParseTreeFromSource(t, jsLang, sourceCode)

		adapter := treesitter.NewSemanticTraverserAdapter()
		options := outbound.SemanticExtractionOptions{
			IncludePrivate:  true,
			IncludeMetadata: true,
		}

		modules, err := adapter.ExtractModules(ctx, domainTree, options)
		require.NoError(t, err)
		require.Len(t, modules, 1, "Should extract IIFE module with conditional return")

		module := modules[0]
		assert.Equal(t, "ConditionalModule", module.Name, "Module name should be 'ConditionalModule'")
		assert.Equal(t, outbound.ConstructModule, module.Type, "Module type should be ConstructModule")
	})

	t.Run("IIFE assigned to object property", func(t *testing.T) {
		sourceCode := `
const container = {
	module: (function() {
		return {
			method: function() {}
		};
	})()
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
		// This is an edge case - IIFE is not at top level variable declarator
		// Implementation decision: should this be detected as a module or not?
		// For now, expecting 0 modules since it's not a top-level pattern
		assert.Empty(t, modules, "Should not extract IIFE module from object property assignment")
	})
}
