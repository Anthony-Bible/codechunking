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

// TestJavaScriptClassExtraction_TreeSitterIntegration tests tree-sitter AST node detection.
// This is a RED PHASE test that verifies tree-sitter grammar integration for class parsing.
// EXPECTED TO FAIL: Currently the system may be using stub parsers instead of real tree-sitter.
func TestJavaScriptClassExtraction_TreeSitterIntegration(t *testing.T) {
	sourceCode := `// ast_integration_test.js

class SimpleClass {
	constructor(name) {
		this.name = name;
	}
	
	getName() {
		return this.name;
	}
}

const ClassExpression = class {
	method() {
		return 'expression';
	}
};`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createRealParseTreeFromSource(t, language, sourceCode)
	require.NotNil(t, parseTree, "ParseTree should not be nil")

	// RED PHASE: Verify that real tree-sitter AST nodes are detected
	// This test will FAIL if using stub parser instead of real tree-sitter

	// Test class_declaration nodes are found
	classDeclarationNodes := parseTree.GetNodesByType("class_declaration")
	require.Len(t, classDeclarationNodes, 1, "Should find exactly 1 class_declaration AST node")

	// Test class expression nodes are found
	classExpressionNodes := parseTree.GetNodesByType("class")
	require.Len(t, classExpressionNodes, 1, "Should find exactly 1 class expression AST node")

	// Test variable_declarator nodes are found
	variableDeclarators := parseTree.GetNodesByType("variable_declarator")
	require.GreaterOrEqual(t, len(variableDeclarators), 1, "Should find variable_declarator nodes")

	// Verify ParseTree.Language() doesn't cause nil pointer panic
	require.NotNil(t, parseTree.Language(), "ParseTree.Language() should not be nil")
	require.Equal(t, "JavaScript", parseTree.Language().Name(), "Language should be JavaScript")
}

// TestJavaScriptClassExtraction_BasicClassDeclarations tests basic ES6 class detection.
// This is a RED PHASE test that defines expected behavior for standard class declarations.
// EXPECTED TO FAIL: Currently returns 0 classes instead of expected count.
func TestJavaScriptClassExtraction_BasicClassDeclarations(t *testing.T) {
	sourceCode := `// basic_classes_red.js

class Animal {
	constructor(name) {
		this.name = name;
		this._type = 'unknown';
	}
	
	speak() {
		return this.name + ' makes a sound';
	}
	
	get type() {
		return this._type;
	}
	
	set type(value) {
		this._type = value;
	}
}

class Dog extends Animal {
	constructor(name, breed) {
		super(name);
		this.breed = breed;
		this._type = 'canine';
	}
	
	speak() {
		return this.name + ' barks';
	}
	
	wagTail() {
		return this.name + ' wags tail';
	}
}

class Cat extends Animal {
	#privateField = 'secret';
	
	constructor(name) {
		super(name);
		this._type = 'feline';
	}
	
	#privateMethod() {
		return this.#privateField;
	}
	
	getSecret() {
		return this.#privateMethod();
	}
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createRealParseTreeFromSource(t, language, sourceCode)
	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	classes, err := adapter.ExtractClasses(ctx, parseTree, options)
	require.NoError(t, err)

	// RED PHASE: These assertions will FAIL until class detection is properly implemented
	// Expected: 3 classes (Animal, Dog, Cat)
	// Actual: Currently returns 0 classes
	require.Len(t, classes, 3, "Should find exactly 3 class declarations")

	// Test base Animal class
	animalClass := testhelpers.FindChunkByName(classes, "Animal")
	require.NotNil(t, animalClass, "Animal class should be detected")
	assert.Equal(t, outbound.ConstructClass, animalClass.Type)
	assert.Equal(t, "Animal", animalClass.Name)
	assert.Equal(t, "basic_classes_red.Animal", animalClass.QualifiedName)
	assert.Empty(t, animalClass.Dependencies, "Animal should have no inheritance dependencies")

	// Test Dog class with inheritance
	dogClass := testhelpers.FindChunkByName(classes, "Dog")
	require.NotNil(t, dogClass, "Dog class should be detected")
	assert.Equal(t, outbound.ConstructClass, dogClass.Type)
	assert.Equal(t, "Dog", dogClass.Name)
	assert.Len(t, dogClass.Dependencies, 1, "Dog should have Animal as dependency")
	assert.Equal(t, "Animal", dogClass.Dependencies[0].Name)
	assert.Equal(t, "inheritance", dogClass.Dependencies[0].Type)

	// Test Cat class with private members
	catClass := testhelpers.FindChunkByName(classes, "Cat")
	require.NotNil(t, catClass, "Cat class should be detected")
	assert.Equal(t, outbound.ConstructClass, catClass.Type)

	// Test private members metadata - RED PHASE: will fail if metadata not populated
	hasPrivateMembers, exists := catClass.Metadata["has_private_members"]
	require.True(t, exists, "has_private_members metadata should exist")
	assert.True(t, hasPrivateMembers.(bool), "Cat should have private members detected")
}

// TestJavaScriptClassExtraction_ClassExpressions tests class expression detection.
// This is a RED PHASE test that defines expected behavior for class expressions.
// EXPECTED TO FAIL: Currently class expressions in variable assignments are not detected.
func TestJavaScriptClassExtraction_ClassExpressions(t *testing.T) {
	sourceCode := `// class_expressions_red.js

// Anonymous class expression assigned to variable
const MyClass = class {
	constructor(value) {
		this.value = value;
	}
	
	getValue() {
		return this.value;
	}
};

// Named class expression
const NamedWrapper = class NamedClass {
	static counter = 0;
	
	constructor() {
		NamedClass.counter++;
		this.id = NamedClass.counter;
	}
	
	getId() {
		return this.id;
	}
};

// Class expression with inheritance
const ExtendedClass = class extends MyClass {
	constructor(value, extra) {
		super(value);
		this.extra = extra;
	}
	
	getFullValue() {
		return this.getValue() + this.extra;
	}
};

// IIFE with class expression
const IIFEClass = (function() {
	return class {
		static instanceCount = 0;
		
		constructor(name) {
			IIFEClass.instanceCount++;
			this.name = name;
		}
		
		getName() {
			return this.name;
		}
	};
})();`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createRealParseTreeFromSource(t, language, sourceCode)
	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        12,
	}

	classes, err := adapter.ExtractClasses(ctx, parseTree, options)
	require.NoError(t, err)

	// RED PHASE: These assertions will FAIL until class expressions are properly detected
	// Expected: 4 classes (MyClass, NamedClass, ExtendedClass, IIFE class)
	// Actual: Currently returns 0 classes
	require.Len(t, classes, 4, "Should find exactly 4 class expressions")

	// Test anonymous class assigned to variable
	myClass := testhelpers.FindChunkByName(classes, "MyClass")
	require.NotNil(t, myClass, "MyClass variable-assigned class expression should be found")
	assert.Equal(t, outbound.ConstructClass, myClass.Type)
	assert.Equal(t, "MyClass", myClass.Name)

	// Test named class expression
	namedClass := testhelpers.FindChunkByName(classes, "NamedClass")
	require.NotNil(t, namedClass, "NamedClass expression should be found")
	assert.Equal(t, outbound.ConstructClass, namedClass.Type)
	assert.Equal(t, "NamedClass", namedClass.Name)

	// Test class expression with inheritance
	extendedClass := testhelpers.FindChunkByName(classes, "ExtendedClass")
	require.NotNil(t, extendedClass, "ExtendedClass should be found")
	assert.Equal(t, outbound.ConstructClass, extendedClass.Type)
	assert.Len(t, extendedClass.Dependencies, 1, "Should have inheritance dependency")
	assert.Equal(t, "MyClass", extendedClass.Dependencies[0].Name)

	// Test IIFE class - should be detected with some identifier
	var iifeClassFound bool
	for _, class := range classes {
		if class.QualifiedName == "class_expressions_red.IIFEClass" || class.Name == "IIFEClass" {
			iifeClassFound = true
			break
		}
	}
	assert.True(t, iifeClassFound, "IIFE class should be detected")
}

// TestJavaScriptClassExtraction_MixinFactories tests mixin factory function detection.
// This is a RED PHASE test that defines expected behavior for functions returning classes.
// EXPECTED TO FAIL: Currently functions returning classes are not detected as class-like constructs.
func TestJavaScriptClassExtraction_MixinFactories(t *testing.T) {
	sourceCode := `// mixin_factories_red.js

// Mixin factory using arrow function
const Timestamped = (BaseClass) => class extends BaseClass {
	constructor(...args) {
		super(...args);
		this.createdAt = new Date();
		this.updatedAt = new Date();
	}
	
	touch() {
		this.updatedAt = new Date();
	}
};

// Mixin factory using regular function
function Serializable(BaseClass) {
	return class extends BaseClass {
		toJSON() {
			return JSON.stringify(this);
		}
		
		static fromJSON(json) {
			return JSON.parse(json);
		}
	};
}

// Higher-order mixin
const withValidation = (BaseClass, rules = {}) => {
	return class extends BaseClass {
		validate() {
			for (const [field, rule] of Object.entries(rules)) {
				if (!rule(this[field])) {
					throw new Error('Validation failed for ' + field);
				}
			}
		}
	};
};

// Base class
class Entity {
	constructor(id) {
		this.id = id;
	}
}

// Class using multiple mixins
class User extends withValidation(Serializable(Timestamped(Entity)), {
	id: id => id && id > 0
}) {
	constructor(id, name) {
		super(id);
		this.name = name;
	}
	
	getName() {
		return this.name;
	}
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createRealParseTreeFromSource(t, language, sourceCode)
	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        15,
	}

	classes, err := adapter.ExtractClasses(ctx, parseTree, options)
	require.NoError(t, err)

	// RED PHASE: These assertions will FAIL until mixin factory detection is implemented
	// Expected: Base classes + mixin factories detected as class-like constructs
	// Actual: Currently mixin factories are not detected
	require.GreaterOrEqual(t, len(classes), 3, "Should find Entity, User, and mixin factories")

	// Test base Entity class
	entityClass := testhelpers.FindChunkByName(classes, "Entity")
	require.NotNil(t, entityClass, "Entity class should be found")
	assert.Equal(t, outbound.ConstructClass, entityClass.Type)

	// Test User class with complex mixin chain
	userClass := testhelpers.FindChunkByName(classes, "User")
	require.NotNil(t, userClass, "User class should be found")
	assert.Equal(t, outbound.ConstructClass, userClass.Type)

	// Test mixin chain detection - RED PHASE: will fail until implemented
	mixinChain, exists := userClass.Metadata["mixin_chain"]
	require.True(t, exists, "mixin_chain metadata should exist")
	require.IsType(t, []string{}, mixinChain, "mixin_chain should be []string")

	chainSlice := mixinChain.([]string)
	assert.Contains(t, chainSlice, "Timestamped", "Should detect Timestamped in mixin chain")
	assert.Contains(t, chainSlice, "Serializable", "Should detect Serializable in mixin chain")
	assert.Contains(t, chainSlice, "withValidation", "Should detect withValidation in mixin chain")

	// Test that mixin factory functions are detected as class-like constructs
	timestampedMixin := testhelpers.FindChunkByName(classes, "Timestamped")
	require.NotNil(t, timestampedMixin, "Timestamped mixin factory should be detected")

	returnsClass, exists := timestampedMixin.Metadata["returns_class"]
	require.True(t, exists, "returns_class metadata should exist for mixin factory")
	assert.True(t, returnsClass.(bool), "Mixin factory should have returns_class = true")

	serializableMixin := testhelpers.FindChunkByName(classes, "Serializable")
	require.NotNil(t, serializableMixin, "Serializable mixin factory should be detected")

	returnsClassSerial, exists := serializableMixin.Metadata["returns_class"]
	require.True(t, exists, "returns_class metadata should exist for Serializable")
	assert.True(t, returnsClassSerial.(bool), "Serializable should have returns_class = true")
}

// TestJavaScriptClassExtraction_StaticMemberMetadata tests static member metadata extraction.
// This is a RED PHASE test that defines expected behavior for static member detection and metadata.
// EXPECTED TO FAIL: Currently static_properties and static_methods metadata is missing/nil.
func TestJavaScriptClassExtraction_StaticMemberMetadata(t *testing.T) {
	sourceCode := `// static_members_red.js

class MathUtils {
	static PI = 3.14159;
	static E = 2.71828;
	static VERSION = '1.0.0';
	
	static add(a, b) {
		return a + b;
	}
	
	static multiply(a, b) {
		return a * b;
	}
	
	static async fetchConstant(name) {
		return await this.constants[name];
	}
	
	static #privateStaticField = 'secret';
	static #maxCacheSize = 1000;
	
	static #privateStaticMethod() {
		return this.#privateStaticField;
	}
	
	static getSecret() {
		return this.#privateStaticMethod();
	}
	
	// Static getter and setter
	static get debugMode() {
		return this._debug || false;
	}
	
	static set debugMode(value) {
		this._debug = value;
	}
}

class DatabaseConnection {
	static #pool = [];
	static #maxConnections = 10;
	
	// Static initialization block
	static {
		console.log('Initializing database pool');
		for (let i = 0; i < this.#maxConnections; i++) {
			this.#pool.push(new Connection(i));
		}
	}
	
	static getConnection() {
		return this.#pool.pop() || null;
	}
	
	static releaseConnection(connection) {
		if (this.#pool.length < this.#maxConnections) {
			this.#pool.push(connection);
		}
	}
}

class Singleton {
	static #instance = null;
	
	constructor() {
		if (Singleton.#instance) {
			return Singleton.#instance;
		}
		Singleton.#instance = this;
	}
	
	static getInstance() {
		if (!Singleton.#instance) {
			Singleton.#instance = new Singleton();
		}
		return Singleton.#instance;
	}
	
	static hasInstance() {
		return Singleton.#instance !== null;
	}
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createRealParseTreeFromSource(t, language, sourceCode)
	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	classes, err := adapter.ExtractClasses(ctx, parseTree, options)
	require.NoError(t, err)

	// RED PHASE: This will FAIL until class detection is fixed
	require.Len(t, classes, 3, "Should find exactly 3 classes")

	// Test MathUtils class with comprehensive static member metadata
	mathUtilsClass := testhelpers.FindChunkByName(classes, "MathUtils")
	require.NotNil(t, mathUtilsClass, "MathUtils class should be found")

	// RED PHASE: These metadata assertions will FAIL until properly implemented
	// Test static properties metadata - must be []string, never nil
	staticProps, exists := mathUtilsClass.Metadata["static_properties"]
	require.True(t, exists, "static_properties metadata must exist")
	require.IsType(t, []string{}, staticProps, "static_properties must be []string")

	staticPropsSlice := staticProps.([]string)
	assert.Contains(t, staticPropsSlice, "PI", "Should detect PI static property")
	assert.Contains(t, staticPropsSlice, "E", "Should detect E static property")
	assert.Contains(t, staticPropsSlice, "VERSION", "Should detect VERSION static property")

	// Test static methods metadata - must be []string, never nil
	staticMethods, exists := mathUtilsClass.Metadata["static_methods"]
	require.True(t, exists, "static_methods metadata must exist")
	require.IsType(t, []string{}, staticMethods, "static_methods must be []string")

	staticMethodsSlice := staticMethods.([]string)
	assert.Contains(t, staticMethodsSlice, "add", "Should detect add static method")
	assert.Contains(t, staticMethodsSlice, "multiply", "Should detect multiply static method")
	assert.Contains(t, staticMethodsSlice, "fetchConstant", "Should detect fetchConstant static method")
	assert.Contains(t, staticMethodsSlice, "getSecret", "Should detect getSecret static method")

	// Test private static members detection
	hasPrivateStatic, exists := mathUtilsClass.Metadata["has_private_static_members"]
	require.True(t, exists, "has_private_static_members metadata should exist")
	assert.True(t, hasPrivateStatic.(bool), "Should detect private static members")

	// Test DatabaseConnection with static initialization block
	dbClass := testhelpers.FindChunkByName(classes, "DatabaseConnection")
	require.NotNil(t, dbClass, "DatabaseConnection class should be found")

	hasStaticInit, exists := dbClass.Metadata["has_static_init_block"]
	require.True(t, exists, "has_static_init_block metadata should exist")
	assert.True(t, hasStaticInit.(bool), "Should detect static initialization block")

	// Test Singleton pattern detection
	singletonClass := testhelpers.FindChunkByName(classes, "Singleton")
	require.NotNil(t, singletonClass, "Singleton class should be found")

	designPattern, exists := singletonClass.Metadata["design_pattern"]
	require.True(t, exists, "design_pattern metadata should exist")
	assert.Equal(t, "singleton", designPattern.(string), "Should detect singleton pattern")
}

// TestJavaScriptClassExtraction_ErrorHandling tests error conditions and edge cases.
// This is a RED PHASE test that defines expected behavior for robust error handling.
// EXPECTED TO FAIL: Currently has nil pointer panics instead of graceful error handling.
func TestJavaScriptClassExtraction_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	t.Run("nil parse tree should return error without panic", func(t *testing.T) {
		options := outbound.SemanticExtractionOptions{}

		// RED PHASE: This should return an error, not panic
		classes, err := adapter.ExtractClasses(ctx, nil, options)
		require.Error(t, err, "Should return error for nil parse tree")
		assert.Contains(t, err.Error(), "parse tree cannot be nil")
		assert.Nil(t, classes, "Classes should be nil on error")
	})

	t.Run("parse tree with nil language should not panic", func(t *testing.T) {
		// Create a malformed parse tree scenario that could cause Language() to return nil
		language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
		require.NoError(t, err)

		parseTree := createRealParseTreeFromSource(t, language, "class Test {}")
		options := outbound.SemanticExtractionOptions{}

		// RED PHASE: Should handle nil Language() gracefully, not panic
		classes, err := adapter.ExtractClasses(ctx, parseTree, options)
		// Should either return error or empty results, but never panic
		if err == nil {
			assert.NotNil(t, classes, "If no error, classes should not be nil")
		}
	})

	t.Run("malformed class syntax should not panic", func(t *testing.T) {
		malformedCodes := []string{
			// Incomplete class declaration
			`class IncompleteClass {
				constructor(
					// Missing closing parenthesis and body
			`,
			// Invalid extends clause
			`class BadExtends extends {
				constructor() {}
			}`,
			// Invalid static member
			`class BadStatic {
				static = 'invalid syntax';
			}`,
			// Nested class syntax error
			`class OuterClass {
				class InnerClass extends {
					// Invalid nested class
				}
			}`,
		}

		language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
		require.NoError(t, err)

		for i, code := range malformedCodes {
			t.Run("malformed_code_"+string(rune('A'+i)), func(t *testing.T) {
				parseTree := createRealParseTreeFromSource(t, language, code)
				options := outbound.SemanticExtractionOptions{}

				// RED PHASE: Should not panic, even with malformed code
				// Should either return error or empty results gracefully
				classes, err := adapter.ExtractClasses(ctx, parseTree, options)
				if err == nil {
					assert.NotNil(t, classes, "If no error, classes should not be nil")
				}
				// The key requirement is NO PANIC - either error or valid response
			})
		}
	})

	t.Run("unsupported language should return appropriate error", func(t *testing.T) {
		// Test with a language that shouldn't support class extraction
		goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		// Create a parse tree with Go code that has class-like syntax
		parseTree := createRealParseTreeFromSource(t, goLang, `
package main

type MyStruct struct {
	Name string
}

func (m *MyStruct) Method() string {
	return m.Name
}`)

		options := outbound.SemanticExtractionOptions{}

		// RED PHASE: Should return appropriate error for unsupported language
		classes, err := adapter.ExtractClasses(ctx, parseTree, options)
		if parseTree != nil && parseTree.Language().Name() != "JavaScript" {
			require.Error(t, err, "Should return error for non-JavaScript language")
			assert.Contains(t, err.Error(), "unsupported language")
			assert.Empty(t, classes, "Should return empty classes for unsupported language")
		}
	})

	t.Run("metadata arrays should never be nil", func(t *testing.T) {
		// Even when no static members exist, metadata arrays should be initialized as empty []string
		sourceCode := `class EmptyClass {
			constructor() {
				this.value = 'test';
			}
			
			regularMethod() {
				return this.value;
			}
		}`

		language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
		require.NoError(t, err)

		parseTree := createRealParseTreeFromSource(t, language, sourceCode)
		options := outbound.SemanticExtractionOptions{}

		classes, err := adapter.ExtractClasses(ctx, parseTree, options)
		require.NoError(t, err)

		if len(classes) > 0 {
			class := classes[0]

			// RED PHASE: These assertions will FAIL if metadata is nil instead of empty slice
			staticProps, exists := class.Metadata["static_properties"]
			require.True(t, exists, "static_properties metadata should always exist")
			require.IsType(t, []string{}, staticProps, "static_properties must be []string")
			assert.Empty(t, staticProps.([]string), "Should be empty slice, not nil")

			staticMethods, exists := class.Metadata["static_methods"]
			require.True(t, exists, "static_methods metadata should always exist")
			require.IsType(t, []string{}, staticMethods, "static_methods must be []string")
			assert.Empty(t, staticMethods.([]string), "Should be empty slice, not nil")
		}
	})
}

// TestJavaScriptClassExtraction_ComplexIntegration tests comprehensive real-world scenarios.
// This is a RED PHASE test that combines multiple patterns to test end-to-end class extraction.
// EXPECTED TO FAIL: Tests complex scenarios that require all features to work together.
func TestJavaScriptClassExtraction_ComplexIntegration(t *testing.T) {
	sourceCode := `// complex_integration_red.js

// Mixin factories
const Timestamped = (BaseClass) => class extends BaseClass {
	constructor(...args) {
		super(...args);
		this.createdAt = new Date();
	}
	
	touch() {
		this.updatedAt = new Date();
	}
};

const Cacheable = (BaseClass) => class extends BaseClass {
	static #cache = new Map();
	
	static clearCache() {
		this.#cache.clear();
	}
	
	getCached(key) {
		return this.constructor.#cache.get(key);
	}
	
	setCached(key, value) {
		this.constructor.#cache.set(key, value);
	}
};

// Base class
class Entity {
	static nextId = 1;
	static #instances = [];
	
	constructor(name) {
		this.id = Entity.nextId++;
		this.name = name;
		Entity.#instances.push(this);
	}
	
	static getInstances() {
		return [...Entity.#instances];
	}
	
	static findById(id) {
		return Entity.#instances.find(instance => instance.id === id);
	}
}

// Complex class using multiple mixins
class User extends Cacheable(Timestamped(Entity)) {
	static USER_TYPES = {
		ADMIN: 'admin',
		REGULAR: 'regular',
		GUEST: 'guest'
	};
	
	#permissions = new Set();
	
	constructor(name, type = User.USER_TYPES.REGULAR) {
		super(name);
		this.type = type;
		this.#setDefaultPermissions();
	}
	
	#setDefaultPermissions() {
		if (this.type === User.USER_TYPES.ADMIN) {
			this.#permissions.add('read').add('write').add('delete');
		} else {
			this.#permissions.add('read');
		}
	}
	
	hasPermission(permission) {
		return this.#permissions.has(permission);
	}
	
	static async createFromAPI(userData) {
		const user = new User(userData.name, userData.type);
		await user.loadPermissions(userData.id);
		return user;
	}
}

// Class expression with static initialization
const ConfigManager = class {
	static #config = {};
	static #loaded = false;
	
	static {
		// Static initialization block
		this.load();
	}
	
	static load() {
		this.#config = {
			apiUrl: process.env.API_URL || 'http://localhost:3000',
			timeout: 5000,
			retries: 3
		};
		this.#loaded = true;
	}
	
	static get(key) {
		if (!this.#loaded) {
			this.load();
		}
		return this.#config[key];
	}
	
	static set(key, value) {
		this.#config[key] = value;
	}
};

// Singleton pattern with private constructor approach
class DatabaseManager {
	static #instance = null;
	static #initialized = false;
	
	constructor() {
		if (DatabaseManager.#instance) {
			throw new Error('DatabaseManager is a singleton. Use getInstance()');
		}
		
		if (!DatabaseManager.#initialized) {
			throw new Error('DatabaseManager must be initialized via getInstance()');
		}
		
		this.connections = new Map();
	}
	
	static getInstance() {
		if (!DatabaseManager.#instance) {
			DatabaseManager.#initialized = true;
			DatabaseManager.#instance = new DatabaseManager();
			DatabaseManager.#initialized = false;
		}
		return DatabaseManager.#instance;
	}
	
	static async reset() {
		if (DatabaseManager.#instance) {
			await DatabaseManager.#instance.closeAll();
			DatabaseManager.#instance = null;
		}
	}
	
	async closeAll() {
		for (const connection of this.connections.values()) {
			await connection.close();
		}
		this.connections.clear();
	}
}`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createRealParseTreeFromSource(t, language, sourceCode)
	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             15,
	}

	classes, err := adapter.ExtractClasses(ctx, parseTree, options)
	require.NoError(t, err)

	// RED PHASE: These assertions will FAIL until full implementation is complete
	// Expected: Entity, User, ConfigManager, DatabaseManager + mixin factories
	require.GreaterOrEqual(t, len(classes), 6, "Should find all classes and mixin factories")

	// Test Entity base class with static members
	entityClass := testhelpers.FindChunkByName(classes, "Entity")
	require.NotNil(t, entityClass, "Entity class should be found")

	staticProps, exists := entityClass.Metadata["static_properties"]
	require.True(t, exists, "static_properties should exist")
	staticPropsSlice := staticProps.([]string)
	assert.Contains(t, staticPropsSlice, "nextId", "Should detect nextId static property")

	staticMethods, exists := entityClass.Metadata["static_methods"]
	require.True(t, exists, "static_methods should exist")
	staticMethodsSlice := staticMethods.([]string)
	assert.Contains(t, staticMethodsSlice, "getInstances", "Should detect getInstances static method")
	assert.Contains(t, staticMethodsSlice, "findById", "Should detect findById static method")

	// Test User class with complex mixin chain
	userClass := testhelpers.FindChunkByName(classes, "User")
	require.NotNil(t, userClass, "User class should be found")

	mixinChain, exists := userClass.Metadata["mixin_chain"]
	require.True(t, exists, "mixin_chain should exist for User class")
	chainSlice := mixinChain.([]string)
	assert.Contains(t, chainSlice, "Timestamped", "Should detect Timestamped in mixin chain")
	assert.Contains(t, chainSlice, "Cacheable", "Should detect Cacheable in mixin chain")

	hasPrivateMembers, exists := userClass.Metadata["has_private_members"]
	require.True(t, exists, "has_private_members should exist")
	assert.True(t, hasPrivateMembers.(bool), "User should have private members")

	// Test ConfigManager class expression with static init block
	configClass := testhelpers.FindChunkByName(classes, "ConfigManager")
	require.NotNil(t, configClass, "ConfigManager class should be found")

	hasStaticInit, exists := configClass.Metadata["has_static_init_block"]
	require.True(t, exists, "has_static_init_block should exist")
	assert.True(t, hasStaticInit.(bool), "ConfigManager should have static initialization block")

	// Test DatabaseManager singleton pattern detection
	dbManagerClass := testhelpers.FindChunkByName(classes, "DatabaseManager")
	require.NotNil(t, dbManagerClass, "DatabaseManager class should be found")

	designPattern, exists := dbManagerClass.Metadata["design_pattern"]
	require.True(t, exists, "design_pattern should exist")
	assert.Equal(t, "singleton", designPattern.(string), "Should detect singleton pattern")

	// Test mixin factory detection
	timestampedMixin := testhelpers.FindChunkByName(classes, "Timestamped")
	require.NotNil(t, timestampedMixin, "Timestamped mixin should be detected")

	returnsClass, exists := timestampedMixin.Metadata["returns_class"]
	require.True(t, exists, "returns_class should exist for mixin")
	assert.True(t, returnsClass.(bool), "Mixin should have returns_class = true")

	cacheableMixin := testhelpers.FindChunkByName(classes, "Cacheable")
	require.NotNil(t, cacheableMixin, "Cacheable mixin should be detected")

	returnsClassCache, exists := cacheableMixin.Metadata["returns_class"]
	require.True(t, exists, "returns_class should exist for Cacheable")
	assert.True(t, returnsClassCache.(bool), "Cacheable should have returns_class = true")
}
