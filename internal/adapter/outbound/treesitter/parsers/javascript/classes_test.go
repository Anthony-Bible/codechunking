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

// TestJavaScriptClasses_ES6Classes tests ES6 class parsing.
// This is a RED PHASE test that defines expected behavior for ES6 class extraction.
func TestJavaScriptClasses_ES6Classes(t *testing.T) {
	sourceCode := `// es6_classes.js

class Animal {
    constructor(name) {
        this.name = name;
        this._species = 'unknown';
    }
    
    speak() {
        return this.name + ' makes a sound';
    }
    
    get species() {
        return this._species;
    }
    
    set species(value) {
        this._species = value;
    }
    
    static getKingdom() {
        return 'Animalia';
    }
}

class Dog extends Animal {
    constructor(name, breed) {
        super(name);
        this.breed = breed;
        this._species = 'Canis lupus';
    }
    
    speak() {
        return this.name + ' barks';
    }
    
    wagTail() {
        return this.name + ' wags tail happily';
    }
    
    static getSpecies() {
        return 'Canis lupus';
    }
}

class Cat extends Animal {
    #privateField = 'secret';
    
    constructor(name, indoor = true) {
        super(name);
        this.indoor = indoor;
        this._species = 'Felis catus';
    }
    
    speak() {
        return this.name + ' meows';
    }
    
    #privateMethod() {
        return this.#privateField;
    }
    
    getSecret() {
        return this.#privateMethod();
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
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	classes, err := adapter.ExtractClasses(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find 3 classes
	require.Len(t, classes, 3)

	// Test base Animal class
	animalClass := testhelpers.FindChunkByName(classes, "Animal")
	require.NotNil(t, animalClass, "Should find Animal class")
	assert.Equal(t, outbound.ConstructClass, animalClass.Type)
	assert.Equal(t, "Animal", animalClass.Name)
	assert.Equal(t, "es6_classes.Animal", animalClass.QualifiedName)
	assert.Empty(t, animalClass.Dependencies) // No inheritance

	// Test Dog class with inheritance
	dogClass := testhelpers.FindChunkByName(classes, "Dog")
	require.NotNil(t, dogClass, "Should find Dog class")
	assert.Equal(t, outbound.ConstructClass, dogClass.Type)
	assert.Equal(t, "Dog", dogClass.Name)
	assert.GreaterOrEqual(t, len(dogClass.Dependencies), 1, "Should have Animal as dependency")
	// Check inheritance relationship
	var hasAnimalDep bool
	for _, dep := range dogClass.Dependencies {
		if dep.Name == "Animal" {
			hasAnimalDep = true
			break
		}
	}
	assert.True(t, hasAnimalDep, "Should have Animal as inheritance dependency")

	// Test Cat class with private fields
	catClass := testhelpers.FindChunkByName(classes, "Cat")
	require.NotNil(t, catClass, "Should find Cat class")
	assert.Equal(t, outbound.ConstructClass, catClass.Type)

	// Should contain information about private fields/methods
	assert.Contains(t, catClass.Metadata, "has_private_members")
	assert.True(t, catClass.Metadata["has_private_members"].(bool))
}

// TestJavaScriptClasses_ClassExpressions tests class expression parsing.
// This is a RED PHASE test that defines expected behavior for class expression extraction.
func TestJavaScriptClasses_ClassExpressions(t *testing.T) {
	sourceCode := `// class_expressions.js

// Named class expression
const MyClass = class NamedClass {
    constructor(value) {
        this.value = value;
    }
    
    getValue() {
        return this.value;
    }
};

// Anonymous class expression
const AnonymousClass = class {
    constructor(data) {
        this.data = data;
    }
    
    process() {
        return this.data.toUpperCase();
    }
};

// Class expression in function
function createClass(type) {
    return class {
        constructor(value) {
            this.type = type;
            this.value = value;
        }
        
        describe() {
            return 'This is a ' + this.type + ' with value ' + this.value;
        }
    };
}

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

// IIFE with class
const IIFEClass = (function() {
    return class {
        static counter = 0;
        
        constructor() {
            IIFEClass.counter++;
            this.id = IIFEClass.counter;
        }
        
        getId() {
            return this.id;
        }
    };
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

	classes, err := adapter.ExtractClasses(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find multiple class expressions
	assert.GreaterOrEqual(t, len(classes), 5)

	// Test named class expression
	namedClass := testhelpers.FindChunkByName(classes, "NamedClass")
	require.NotNil(t, namedClass, "Should find NamedClass")
	assert.Equal(t, outbound.ConstructClass, namedClass.Type)

	// Test anonymous class (should have some identifier)
	var anonymousClassFound bool
	for _, class := range classes {
		if class.Name == "AnonymousClass" || (class.Name == "" && class.Type == outbound.ConstructClass) {
			anonymousClassFound = true
			break
		}
	}
	assert.True(t, anonymousClassFound, "Should find anonymous class expression")

	// Test extended class expression
	extendedClass := testhelpers.FindChunkByName(classes, "ExtendedClass")
	if extendedClass == nil {
		// Might be identified differently, check for inheritance relationship
		for _, class := range classes {
			if len(class.Dependencies) > 0 && class.Dependencies[0].Name == "MyClass" {
				extendedClass = &class
				break
			}
		}
	}
	assert.NotNil(t, extendedClass, "Should find extended class expression")

	// Test IIFE class
	iifeClass := testhelpers.FindChunkByName(classes, "IIFEClass")
	if iifeClass == nil {
		// Check for class within IIFE context
		for _, class := range classes {
			if contains(class.QualifiedName, "IIFE") || class.Metadata["context"] == "IIFE" {
				iifeClass = &class
				break
			}
		}
	}
	assert.NotNil(t, iifeClass, "Should find class within IIFE")
}

// TestJavaScriptClasses_Mixins tests mixin pattern parsing.
// This is a RED PHASE test that defines expected behavior for mixin pattern extraction.
func TestJavaScriptClasses_Mixins(t *testing.T) {
	sourceCode := `// mixins.js

// Mixin factory functions
const Flyable = (superclass) => class extends superclass {
    fly() {
        return this.name + ' is flying';
    }
    
    land() {
        return this.name + ' has landed';
    }
};

const Swimmable = (superclass) => class extends superclass {
    swim() {
        return this.name + ' is swimming';
    }
    
    dive() {
        return this.name + ' has dived';
    }
};

// Base class
class Animal {
    constructor(name) {
        this.name = name;
    }
    
    eat() {
        return this.name + ' is eating';
    }
}

// Class with multiple mixins
class Duck extends Swimmable(Flyable(Animal)) {
    constructor(name) {
        super(name);
    }
    
    quack() {
        return this.name + ' quacks';
    }
}

// Alternative mixin pattern
const EventEmitterMixin = {
    on(event, callback) {
        this._events = this._events || {};
        this._events[event] = this._events[event] || [];
        this._events[event].push(callback);
    },
    
    emit(event, ...args) {
        if (this._events && this._events[event]) {
            this._events[event].forEach(callback => callback(...args));
        }
    }
};

class Component {
    constructor(name) {
        this.name = name;
    }
    
    render() {
        return '<div>' + this.name + '</div>';
    }
}

// Apply mixin using Object.assign
Object.assign(Component.prototype, EventEmitterMixin);

// Mixin using Symbol for private methods
const PrivateMixin = (() => {
    const _private = Symbol('private');
    
    return (superclass) => class extends superclass {
        constructor(...args) {
            super(...args);
            this[_private] = 'secret data';
        }
        
        getPrivateData() {
            return this[_private];
        }
        
        setPrivateData(value) {
            this[_private] = value;
        }
    };
})();

class SecureClass extends PrivateMixin(Animal) {
    constructor(name) {
        super(name);
    }
    
    getInfo() {
        return this.name + ' has private data: ' + this.getPrivateData();
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
		MaxDepth:        12, // Increased for complex mixin chains
	}

	classes, err := adapter.ExtractClasses(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find all classes including mixin-generated ones
	assert.GreaterOrEqual(t, len(classes), 6)

	// Test base Animal class
	animalClass := testhelpers.FindChunkByName(classes, "Animal")
	require.NotNil(t, animalClass, "Should find Animal class")

	// Test Duck class with multiple mixins
	duckClass := testhelpers.FindChunkByName(classes, "Duck")
	require.NotNil(t, duckClass, "Should find Duck class")
	assert.GreaterOrEqual(t, len(duckClass.Dependencies), 1, "Should have mixin dependencies")

	// Check for mixin chain in metadata
	assert.Contains(t, duckClass.Metadata, "mixin_chain")

	// Test Component class
	componentClass := testhelpers.FindChunkByName(classes, "Component")
	require.NotNil(t, componentClass, "Should find Component class")

	// Should have mixin applied via Object.assign
	assert.Contains(t, componentClass.Metadata, "has_mixins")

	// Test SecureClass with private mixin
	secureClass := testhelpers.FindChunkByName(classes, "SecureClass")
	require.NotNil(t, secureClass, "Should find SecureClass")
	assert.GreaterOrEqual(t, len(secureClass.Dependencies), 1, "Should have mixin and inheritance dependencies")

	// Test mixin factory functions are identified
	flyableMixin := testhelpers.FindChunkByName(classes, "Flyable")
	if flyableMixin == nil {
		// Might be identified as a function that returns a class
		functions, err := adapter.ExtractFunctions(ctx, parseTree, options)
		require.NoError(t, err)

		flyableFunc := testhelpers.FindChunkByName(functions, "Flyable")
		assert.NotNil(t, flyableFunc, "Should find Flyable mixin factory function")
		if flyableFunc != nil {
			assert.Equal(t, outbound.ConstructFunction, flyableFunc.Type)
			assert.Contains(t, flyableFunc.Metadata, "returns_class")
		}
	}
}

// TestJavaScriptClasses_StaticMembers tests static method and property parsing.
// This is a RED PHASE test that defines expected behavior for static member extraction.
func TestJavaScriptClasses_StaticMembers(t *testing.T) {
	sourceCode := `// static_members.js

class MathUtils {
    static PI = 3.14159;
    static E = 2.71828;
    
    static add(a, b) {
        return a + b;
    }
    
    static multiply(a, b) {
        return a * b;
    }
    
    static #privateStaticField = 'secret';
    
    static #privateStaticMethod() {
        return MathUtils.#privateStaticField;
    }
    
    static getSecret() {
        return MathUtils.#privateStaticMethod();
    }
    
    // Static getter and setter
    static get version() {
        return '1.0.0';
    }
    
    static set debugMode(value) {
        this._debug = value;
    }
    
    static get debugMode() {
        return this._debug || false;
    }
}

class Counter {
    static #count = 0;
    static #instances = [];
    
    constructor(name) {
        this.name = name;
        Counter.#count++;
        Counter.#instances.push(this);
    }
    
    static getCount() {
        return Counter.#count;
    }
    
    static getInstances() {
        return [...Counter.#instances];
    }
    
    static reset() {
        Counter.#count = 0;
        Counter.#instances = [];
    }
    
    static findByName(name) {
        return Counter.#instances.find(instance => instance.name === name);
    }
}

class Singleton {
    static #instance = null;
    
    constructor() {
        if (Singleton.#instance) {
            return Singleton.#instance;
        }
        Singleton.#instance = this;
        this.created = new Date();
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
    
    static clearInstance() {
        Singleton.#instance = null;
    }
}

// Class with static initialization block
class DatabaseConnection {
    static #pool = [];
    static #maxConnections = 10;
    
    static {
        // Static initialization block
        console.log('Initializing database connection pool');
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
    
    static getPoolSize() {
        return this.#pool.length;
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

	classes, err := adapter.ExtractClasses(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find all classes
	require.Len(t, classes, 4)

	// Test MathUtils class with static members
	mathUtilsClass := testhelpers.FindChunkByName(classes, "MathUtils")
	require.NotNil(t, mathUtilsClass, "Should find MathUtils class")

	// Check for static properties in metadata
	assert.Contains(t, mathUtilsClass.Metadata, "static_properties")
	staticProps := mathUtilsClass.Metadata["static_properties"].([]string)
	assert.Contains(t, staticProps, "PI")
	assert.Contains(t, staticProps, "E")

	// Check for static methods in metadata
	assert.Contains(t, mathUtilsClass.Metadata, "static_methods")
	staticMethods := mathUtilsClass.Metadata["static_methods"].([]string)
	assert.Contains(t, staticMethods, "add")
	assert.Contains(t, staticMethods, "multiply")

	// Test Counter class with private static members
	counterClass := testhelpers.FindChunkByName(classes, "Counter")
	require.NotNil(t, counterClass, "Should find Counter class")

	// Should have private static fields
	assert.Contains(t, counterClass.Metadata, "has_private_static_members")
	assert.True(t, counterClass.Metadata["has_private_static_members"].(bool))

	// Test Singleton class
	singletonClass := testhelpers.FindChunkByName(classes, "Singleton")
	require.NotNil(t, singletonClass, "Should find Singleton class")

	// Should be marked as singleton pattern
	assert.Contains(t, singletonClass.Metadata, "design_pattern")
	assert.Equal(t, "singleton", singletonClass.Metadata["design_pattern"])

	// Test DatabaseConnection class with static initialization block
	dbClass := testhelpers.FindChunkByName(classes, "DatabaseConnection")
	require.NotNil(t, dbClass, "Should find DatabaseConnection class")

	// Should have static initialization block
	assert.Contains(t, dbClass.Metadata, "has_static_init_block")
	assert.True(t, dbClass.Metadata["has_static_init_block"].(bool))
}

// TestJavaScriptClasses_ErrorHandling tests error conditions for class parsing.
// This is a RED PHASE test that defines expected behavior for class parsing error handling.
func TestJavaScriptClasses_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	t.Run("nil parse tree should return error", func(t *testing.T) {
		options := outbound.SemanticExtractionOptions{}

		_, err := adapter.ExtractClasses(ctx, nil, options)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse tree cannot be nil")
	})

	t.Run("unsupported language should return error", func(t *testing.T) {
		goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, goLang, "package main")
		options := outbound.SemanticExtractionOptions{}

		if parseTree != nil {
			_, err = adapter.ExtractClasses(ctx, parseTree, options)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported language")
		}
	})

	t.Run("malformed class should not panic", func(t *testing.T) {
		malformedCode := `// Incomplete class
class IncompleteClass {
    constructor(
        // Missing closing parenthesis and body
`
		language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, malformedCode)
		options := outbound.SemanticExtractionOptions{}

		// Should not panic, even with malformed code
		classes, err := adapter.ExtractClasses(ctx, parseTree, options)
		// May return error or empty results, but should not panic
		if err == nil {
			assert.NotNil(t, classes)
		}
	})
}

// TestJavaScriptClasses_ClassExpressions tests class expression detection.
// This is a RED PHASE test that defines expected behavior for class expressions.
// Currently FAILS because class expressions are not being detected (returns 0 classes).
func TestJavaScriptClasses_ClassExpressions(t *testing.T) {
	sourceCode := `// class_expressions_red.js

// Variable-assigned class expression - should be detected as "MyClass" 
const MyClass = class {
	constructor(name) {
		this.name = name;
	}
	
	getName() {
		return this.name;
	}
};

// Named class expression - should be detected as "NamedClass"
const NamedExpressionWrapper = class NamedClass {
	static counter = 0;
	
	method() {
		return 'hello';
	}
};

// Function returning class - should be detected as mixin/class factory
function createMixin() {
	return class extends BaseClass {
		mixinMethod() {
			return 'mixin';
		}
	};
}
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	classes, err := adapter.ExtractClasses(ctx, parseTree, options)
	require.NoError(t, err)

	// RED PHASE: These assertions will FAIL until class expressions are properly supported
	// Expected: 3 classes (MyClass, NamedClass, and function-returned class)
	// Actual: Currently returns 0 classes
	require.GreaterOrEqual(t, len(classes), 2, "Should find at least 2 class expressions")

	myClass := testhelpers.FindChunkByName(classes, "MyClass")
	require.NotNil(t, myClass, "MyClass variable-assigned class expression should be found")
	assert.Equal(t, outbound.ConstructClass, myClass.Type)

	namedClass := testhelpers.FindChunkByName(classes, "NamedClass")
	require.NotNil(t, namedClass, "NamedClass expression should be found")
	assert.Equal(t, outbound.ConstructClass, namedClass.Type)
}

// TestJavaScriptClasses_StaticMemberMetadata tests static member metadata extraction.
// This is a RED PHASE test that defines expected behavior for static member metadata.
// Currently FAILS because static_properties and static_methods metadata is missing/nil.
func TestJavaScriptClasses_StaticMemberMetadata(t *testing.T) {
	sourceCode := `// static_metadata_red.js

class ApiService {
	static baseURL = 'https://api.example.com';
	static timeout = 5000;
	static retries = 3;
	
	static async get(endpoint) {
		return fetch(this.baseURL + endpoint);
	}
	
	static post(endpoint, data) {
		return this.makeRequest('POST', endpoint, data);
	}
	
	static #privateStaticField = 'secret';
	
	static #privateMethod() {
		return this.#privateStaticField;
	}
	
	static getSecret() {
		return this.#privateMethod();
	}
}
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	classes, err := adapter.ExtractClasses(ctx, parseTree, options)
	require.NoError(t, err)
	require.Len(t, classes, 1)

	apiService := testhelpers.FindChunkByName(classes, "ApiService")
	require.NotNil(t, apiService, "ApiService class should be found")

	// RED PHASE: These assertions will FAIL until static member metadata is properly extracted
	// Expected: metadata contains arrays of static properties and methods
	// Actual: Currently metadata["static_properties"] is nil, causing panic

	// Test static properties metadata - currently missing
	staticProps, exists := apiService.Metadata["static_properties"]
	require.True(t, exists, "static_properties metadata should exist")
	require.IsType(t, []string{}, staticProps, "static_properties should be []string")

	staticPropsSlice := staticProps.([]string)
	assert.Contains(t, staticPropsSlice, "baseURL", "Should detect baseURL static property")
	assert.Contains(t, staticPropsSlice, "timeout", "Should detect timeout static property")
	assert.Contains(t, staticPropsSlice, "retries", "Should detect retries static property")

	// Test static methods metadata - currently missing
	staticMethods, exists := apiService.Metadata["static_methods"]
	require.True(t, exists, "static_methods metadata should exist")
	require.IsType(t, []string{}, staticMethods, "static_methods should be []string")

	staticMethodsSlice := staticMethods.([]string)
	assert.Contains(t, staticMethodsSlice, "get", "Should detect get static method")
	assert.Contains(t, staticMethodsSlice, "post", "Should detect post static method")
	assert.Contains(t, staticMethodsSlice, "getSecret", "Should detect getSecret static method")

	// Test private static members detection
	hasPrivateStatic, exists := apiService.Metadata["has_private_static_members"]
	require.True(t, exists, "has_private_static_members metadata should exist")
	assert.True(t, hasPrivateStatic.(bool), "Should detect private static members")
}

// TestJavaScriptClasses_MixinFactoryDetection tests mixin factory function detection.
// This is a RED PHASE test that defines expected behavior for functions returning classes.
// Currently FAILS because functions returning classes are not detected as class-like constructs.
func TestJavaScriptClasses_MixinFactoryDetection(t *testing.T) {
	sourceCode := `// mixin_factory_red.js

// Mixin factory function that returns a class - should be detected with returns_class metadata
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

// Another mixin pattern
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

// Regular class using mixins
class User extends Serializable(Timestamped(Object)) {
	constructor(name) {
		super();
		this.name = name;
	}
}
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        12,
	}

	classes, err := adapter.ExtractClasses(ctx, parseTree, options)
	require.NoError(t, err)

	// RED PHASE: These assertions will FAIL until mixin factory detection is implemented
	// Expected: Functions returning classes should be detected as class-like constructs
	// Actual: Currently mixin factories are not detected

	// Should find User class plus mixin factory functions detected as class constructs
	require.GreaterOrEqual(t, len(classes), 2, "Should find User class and mixin factories")

	userClass := testhelpers.FindChunkByName(classes, "User")
	require.NotNil(t, userClass, "User class should be found")
	assert.Equal(t, outbound.ConstructClass, userClass.Type)

	// Test mixin chain detection - currently missing
	mixinChain, exists := userClass.Metadata["mixin_chain"]
	require.True(t, exists, "mixin_chain metadata should exist")
	require.IsType(t, []string{}, mixinChain, "mixin_chain should be []string")

	chainSlice := mixinChain.([]string)
	assert.Contains(t, chainSlice, "Timestamped", "Should detect Timestamped in mixin chain")
	assert.Contains(t, chainSlice, "Serializable", "Should detect Serializable in mixin chain")

	// Test that mixin factory functions are detected as class-like constructs
	timestampedMixin := testhelpers.FindChunkByName(classes, "Timestamped")
	require.NotNil(t, timestampedMixin, "Timestamped mixin factory should be detected")

	returnsClass, exists := timestampedMixin.Metadata["returns_class"]
	require.True(t, exists, "returns_class metadata should exist for mixin factory")
	assert.True(t, returnsClass.(bool), "Mixin factory should have returns_class = true")
}
