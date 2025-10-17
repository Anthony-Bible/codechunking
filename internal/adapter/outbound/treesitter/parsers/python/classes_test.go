package pythonparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPythonClassParser_ParsePythonClass_BasicClass tests parsing of basic Python classes.
// This is a RED PHASE test that defines expected behavior for basic Python class parsing.
func TestPythonClassParser_ParsePythonClass_BasicClass(t *testing.T) {
	sourceCode := `class Person:
    """A simple person class."""
    
    def __init__(self, name: str, age: int):
        self.name = name
        self.age = age
    
    def greet(self) -> str:
        return f"Hello, I'm {self.name}"
    
    def __str__(self) -> str:
        return f"Person(name={self.name}, age={self.age})"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	// REFACTOR PHASE: Use real class nodes from tree-sitter parsing
	classNodes := parseTree.GetNodesByType("class_definition")
	require.Len(t, classNodes, 1, "Should find 1 class definition")

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	result := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[0],
		"test_module",
		options,
		time.Now(),
	)
	require.NotNil(t, result, "ParsePythonClass should return a result")

	// Validate the parsed class
	assert.Equal(t, outbound.ConstructClass, result.Type)
	assert.Equal(t, "Person", result.Name)
	assert.Equal(t, "test_module.Person", result.QualifiedName)
	assert.Contains(t, result.Documentation, "A simple person class")
	assert.Equal(t, outbound.Public, result.Visibility)
	assert.False(t, result.IsGeneric)
	assert.False(t, result.IsAsync)
	assert.False(t, result.IsAbstract)

	// Should have child chunks for methods
	assert.GreaterOrEqual(t, len(result.ChildChunks), 3) // __init__, greet, __str__

	// Validate constructor method
	initMethod := findChildByName(result.ChildChunks, "__init__")
	require.NotNil(t, initMethod, "Should find __init__ method")
	assert.Equal(t, outbound.ConstructMethod, initMethod.Type)
	assert.Equal(t, outbound.Private, initMethod.Visibility) // Dunder methods are private

	// Validate public method
	greetMethod := findChildByName(result.ChildChunks, "greet")
	require.NotNil(t, greetMethod, "Should find greet method")
	assert.Equal(t, outbound.ConstructMethod, greetMethod.Type)
	assert.Equal(t, outbound.Public, greetMethod.Visibility)
	assert.Equal(t, "str", greetMethod.ReturnType)
}

// TestPythonClassParser_ParsePythonClass_Inheritance tests parsing of classes with inheritance.
// This is a RED PHASE test that defines expected behavior for Python inheritance parsing.
func TestPythonClassParser_ParsePythonClass_Inheritance(t *testing.T) {
	sourceCode := `class Animal:
    """Base animal class."""
    
    def __init__(self, name: str):
        self.name = name
    
    def speak(self) -> str:
        raise NotImplementedError

class Dog(Animal):
    """A dog that inherits from Animal."""
    
    def __init__(self, name: str, breed: str):
        super().__init__(name)
        self.breed = breed
    
    def speak(self) -> str:
        return f"{self.name} says Woof!"
    
    def fetch(self) -> str:
        return f"{self.name} fetches the ball!"

class Puppy(Dog):
    """A puppy that inherits from Dog."""
    
    def __init__(self, name: str, breed: str, age_months: int):
        super().__init__(name, breed)
        self.age_months = age_months
    
    def speak(self) -> str:
        return f"{self.name} says Yip yip!"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	classNodes := parseTree.GetNodesByType("class_definition")
	require.Len(t, classNodes, 3, "Should find 3 class definitions")

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	// Test base class (Animal)
	animalResult := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[0],
		"animals",
		options,
		time.Now(),
	)
	require.NotNil(t, animalResult)
	assert.Equal(t, "Animal", animalResult.Name)
	assert.Equal(t, "animals.Animal", animalResult.QualifiedName)
	assert.Contains(t, animalResult.Documentation, "Base animal class")
	assert.Empty(t, animalResult.Dependencies) // No inheritance

	// Test single inheritance (Dog)
	dogResult := parser.ParsePythonClass(context.Background(), parseTree, classNodes[1], "animals", options, time.Now())
	require.NotNil(t, dogResult)
	assert.Equal(t, "Dog", dogResult.Name)
	assert.Equal(t, "animals.Dog", dogResult.QualifiedName)
	assert.GreaterOrEqual(t, len(dogResult.Dependencies), 1, "Should have inheritance dependency on Animal")

	// Check for Animal dependency
	animalDep := findDependencyByName(dogResult.Dependencies, "Animal")
	require.NotNil(t, animalDep, "Should have dependency on Animal")
	assert.Equal(t, "inheritance", animalDep.Type)

	// Test multi-level inheritance (Puppy)
	puppyResult := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[2],
		"animals",
		options,
		time.Now(),
	)
	require.NotNil(t, puppyResult)
	assert.Equal(t, "Puppy", puppyResult.Name)
	assert.GreaterOrEqual(t, len(puppyResult.Dependencies), 1, "Should have inheritance dependency on Dog")

	// Check for Dog dependency
	dogDep := findDependencyByName(puppyResult.Dependencies, "Dog")
	require.NotNil(t, dogDep, "Should have dependency on Dog")
	assert.Equal(t, "inheritance", dogDep.Type)
}

// TestPythonClassParser_ParsePythonClass_MultipleInheritance tests parsing of multiple inheritance.
// This is a RED PHASE test that defines expected behavior for Python multiple inheritance parsing.
func TestPythonClassParser_ParsePythonClass_MultipleInheritance(t *testing.T) {
	sourceCode := `from abc import ABC, abstractmethod

class Flyable(ABC):
    """Interface for things that can fly."""
    
    @abstractmethod
    def fly(self) -> str:
        pass

class Swimmable(ABC):
    """Interface for things that can swim."""
    
    @abstractmethod
    def swim(self) -> str:
        pass

class Duck(Flyable, Swimmable):
    """A duck that can both fly and swim."""
    
    def __init__(self, name: str):
        self.name = name
    
    def fly(self) -> str:
        return f"{self.name} flies through the sky"
    
    def swim(self) -> str:
        return f"{self.name} swims in the pond"
    
    def quack(self) -> str:
        return f"{self.name} says Quack!"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	classNodes := parseTree.GetNodesByType("class_definition")
	require.Len(t, classNodes, 3)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	// Test Duck class with multiple inheritance
	duckResult := parser.ParsePythonClass(context.Background(), parseTree, classNodes[2], "birds", options, time.Now())
	require.NotNil(t, duckResult)

	assert.Equal(t, "Duck", duckResult.Name)
	assert.Equal(t, "birds.Duck", duckResult.QualifiedName)
	assert.GreaterOrEqual(t, len(duckResult.Dependencies), 2, "Should have dependencies on both Flyable and Swimmable")

	// Check for both interface dependencies
	flyableDep := findDependencyByName(duckResult.Dependencies, "Flyable")
	require.NotNil(t, flyableDep, "Should have dependency on Flyable")
	assert.Equal(t, "inheritance", flyableDep.Type)

	swimmableDep := findDependencyByName(duckResult.Dependencies, "Swimmable")
	require.NotNil(t, swimmableDep, "Should have dependency on Swimmable")
	assert.Equal(t, "inheritance", swimmableDep.Type)

	// Validate abstract methods are properly implemented
	flyMethod := findChildByName(duckResult.ChildChunks, "fly")
	require.NotNil(t, flyMethod, "Should find fly method implementation")
	assert.Equal(t, "str", flyMethod.ReturnType)

	swimMethod := findChildByName(duckResult.ChildChunks, "swim")
	require.NotNil(t, swimMethod, "Should find swim method implementation")
	assert.Equal(t, "str", swimMethod.ReturnType)
}

// TestPythonClassParser_ParsePythonClass_Decorators tests parsing of classes with decorators.
// This is a RED PHASE test that defines expected behavior for Python decorator parsing.
func TestPythonClassParser_ParsePythonClass_Decorators(t *testing.T) {
	sourceCode := `from dataclasses import dataclass
from typing import Optional

@dataclass
class Point:
    """A point in 2D space."""
    x: float
    y: float

@dataclass(frozen=True)
class ImmutablePoint:
    """An immutable point in 2D space."""
    x: float
    y: float
    
    def distance_from_origin(self) -> float:
        return (self.x ** 2 + self.y ** 2) ** 0.5

class CustomDecorator:
    """Custom decorator class."""
    
    def __init__(self, func):
        self.func = func
    
    def __call__(self, *args, **kwargs):
        return self.func(*args, **kwargs)

@CustomDecorator
class DecoratedClass:
    """A class with custom decorator."""
    
    def method(self) -> str:
        return "decorated"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	classNodes := parseTree.GetNodesByType("class_definition")
	require.GreaterOrEqual(t, len(classNodes), 3)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	// Test dataclass decorator
	pointResult := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[0],
		"geometry",
		options,
		time.Now(),
	)
	require.NotNil(t, pointResult)

	assert.Equal(t, "Point", pointResult.Name)
	assert.Len(t, pointResult.Annotations, 1)
	assert.Equal(t, "dataclass", pointResult.Annotations[0].Name)

	// Test dataclass with parameters
	immutableResult := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[1],
		"geometry",
		options,
		time.Now(),
	)
	require.NotNil(t, immutableResult)

	assert.Equal(t, "ImmutablePoint", immutableResult.Name)
	assert.Len(t, immutableResult.Annotations, 1)
	assert.Equal(t, "dataclass", immutableResult.Annotations[0].Name)
	assert.Contains(t, immutableResult.Annotations[0].Arguments[0], "frozen=True")

	// Test custom decorator
	decoratedResult := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[3],
		"geometry",
		options,
		time.Now(),
	)
	require.NotNil(t, decoratedResult)

	assert.Equal(t, "DecoratedClass", decoratedResult.Name)
	assert.Len(t, decoratedResult.Annotations, 1)
	assert.Equal(t, "CustomDecorator", decoratedResult.Annotations[0].Name)
}

// TestPythonClassParser_ParsePythonClass_InnerClasses tests parsing of nested classes.
// This is a RED PHASE test that defines expected behavior for Python nested class parsing.
func TestPythonClassParser_ParsePythonClass_InnerClasses(t *testing.T) {
	sourceCode := `class OuterClass:
    """Outer class with nested classes."""
    
    def __init__(self):
        self.value = "outer"
    
    class InnerClass:
        """Inner class definition."""
        
        def __init__(self):
            self.inner_value = "inner"
        
        def inner_method(self) -> str:
            return "inner method"
        
        class DeeplyNested:
            """Deeply nested class."""
            
            def deep_method(self) -> str:
                return "deeply nested"
    
    def outer_method(self) -> str:
        return "outer method"
    
    @classmethod
    def class_method(cls) -> str:
        return "class method"
    
    @staticmethod
    def static_method() -> str:
        return "static method"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	classNodes := parseTree.GetNodesByType("class_definition")
	require.GreaterOrEqual(t, len(classNodes), 3) // Outer, Inner, DeeplyNested

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	// Test outer class
	outerResult := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[0],
		"nested",
		options,
		time.Now(),
	)
	require.NotNil(t, outerResult)

	assert.Equal(t, "OuterClass", outerResult.Name)
	assert.Equal(t, "nested.OuterClass", outerResult.QualifiedName)
	assert.Contains(t, outerResult.Documentation, "Outer class with nested classes")

	// Should have child chunks for methods and nested classes
	assert.GreaterOrEqual(t, len(outerResult.ChildChunks), 4) // __init__, outer_method, class_method, static_method

	// Find class method and static method
	classMethod := findChildByName(outerResult.ChildChunks, "class_method")
	require.NotNil(t, classMethod, "Should find class_method")
	assert.Len(t, classMethod.Annotations, 1)
	assert.Equal(t, "classmethod", classMethod.Annotations[0].Name)

	staticMethod := findChildByName(outerResult.ChildChunks, "static_method")
	require.NotNil(t, staticMethod, "Should find static_method")
	assert.Len(t, staticMethod.Annotations, 1)
	assert.Equal(t, "staticmethod", staticMethod.Annotations[0].Name)

	// Test inner class
	innerResult := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[1],
		"nested",
		options,
		time.Now(),
	)
	require.NotNil(t, innerResult)

	assert.Equal(t, "InnerClass", innerResult.Name)
	assert.Equal(t, "nested.OuterClass.InnerClass", innerResult.QualifiedName)

	// Test deeply nested class
	deepResult := parser.ParsePythonClass(context.Background(), parseTree, classNodes[2], "nested", options, time.Now())
	require.NotNil(t, deepResult)

	assert.Equal(t, "DeeplyNested", deepResult.Name)
	assert.Equal(t, "nested.OuterClass.InnerClass.DeeplyNested", deepResult.QualifiedName)
}

// TestPythonClassParser_ParsePythonClass_ClassVariables tests parsing of class variables.
// This is a RED PHASE test that defines expected behavior for Python class variable parsing.
func TestPythonClassParser_ParsePythonClass_ClassVariables(t *testing.T) {
	sourceCode := `class Config:
    """Configuration class with class variables."""
    
    # Class variables
    DEFAULT_TIMEOUT = 30
    API_VERSION: str = "v1"
    DEBUG_MODE: bool = False
    _PRIVATE_KEY = "secret123"
    __VERY_PRIVATE = "super_secret"
    
    # Class variable with type annotation but no value
    connection_pool: Optional[ConnectionPool] = None
    
    # Class method accessing class variables
    @classmethod
    def get_timeout(cls) -> int:
        return cls.DEFAULT_TIMEOUT
    
    def __init__(self, custom_timeout: Optional[int] = None):
        # Instance variables
        self.timeout = custom_timeout or self.DEFAULT_TIMEOUT
        self._session_id = None
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	classNodes := parseTree.GetNodesByType("class_definition")
	require.Len(t, classNodes, 1)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	result := parser.ParsePythonClass(context.Background(), parseTree, classNodes[0], "config", options, time.Now())
	require.NotNil(t, result)

	assert.Equal(t, "Config", result.Name)
	assert.Equal(t, "config.Config", result.QualifiedName)

	// Should have child chunks for class variables and methods
	assert.GreaterOrEqual(t, len(result.ChildChunks), 6) // 5 class variables + methods

	// Validate class variables
	defaultTimeout := findChildByName(result.ChildChunks, "DEFAULT_TIMEOUT")
	require.NotNil(t, defaultTimeout, "Should find DEFAULT_TIMEOUT class variable")
	assert.Equal(t, outbound.ConstructConstant, defaultTimeout.Type)
	assert.Equal(t, outbound.Public, defaultTimeout.Visibility)

	apiVersion := findChildByName(result.ChildChunks, "API_VERSION")
	require.NotNil(t, apiVersion, "Should find API_VERSION class variable")
	assert.Equal(t, outbound.ConstructConstant, apiVersion.Type)
	assert.Equal(t, "str", apiVersion.ReturnType)

	privateKey := findChildByName(result.ChildChunks, "_PRIVATE_KEY")
	require.NotNil(t, privateKey, "Should find _PRIVATE_KEY class variable")
	assert.Equal(t, outbound.Private, privateKey.Visibility)

	veryPrivate := findChildByName(result.ChildChunks, "__VERY_PRIVATE")
	require.NotNil(t, veryPrivate, "Should find __VERY_PRIVATE class variable")
	assert.Equal(t, outbound.Private, veryPrivate.Visibility)
}

// TestPythonClassParser_ParsePythonClass_Properties tests parsing of properties.
// This is a RED PHASE test that defines expected behavior for Python property parsing.
func TestPythonClassParser_ParsePythonClass_Properties(t *testing.T) {
	sourceCode := `class Circle:
    """A circle with radius property."""
    
    def __init__(self, radius: float):
        self._radius = radius
    
    @property
    def radius(self) -> float:
        """Get the radius."""
        return self._radius
    
    @radius.setter
    def radius(self, value: float) -> None:
        """Set the radius."""
        if value <= 0:
            raise ValueError("Radius must be positive")
        self._radius = value
    
    @property
    def area(self) -> float:
        """Calculate the area (read-only property)."""
        return 3.14159 * self._radius ** 2
    
    @property
    def diameter(self) -> float:
        """Calculate the diameter (read-only property)."""
        return 2 * self._radius
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	classNodes := parseTree.GetNodesByType("class_definition")
	require.Len(t, classNodes, 1)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	result := parser.ParsePythonClass(context.Background(), parseTree, classNodes[0], "shapes", options, time.Now())
	require.NotNil(t, result)

	assert.Equal(t, "Circle", result.Name)

	// Find property methods
	radiusGetter := findChildByName(result.ChildChunks, "radius")
	require.NotNil(t, radiusGetter, "Should find radius property getter")
	assert.Equal(t, outbound.ConstructMethod, radiusGetter.Type)
	assert.Len(t, radiusGetter.Annotations, 1)
	assert.Equal(t, "property", radiusGetter.Annotations[0].Name)
	assert.Contains(t, radiusGetter.Documentation, "Get the radius")

	areaProperty := findChildByName(result.ChildChunks, "area")
	require.NotNil(t, areaProperty, "Should find area property")
	assert.Equal(t, "float", areaProperty.ReturnType)
	assert.Contains(t, areaProperty.Documentation, "read-only property")
}

// TestPythonClassParser_ErrorHandling tests error conditions.
// This is a RED PHASE test that defines expected behavior for error handling.
func TestPythonClassParser_ErrorHandling(t *testing.T) {
	parser := NewPythonClassParser()

	t.Run("nil parse tree should not panic", func(t *testing.T) {
		result := parser.ParsePythonClass(
			context.Background(),
			nil,
			nil,
			"test",
			outbound.SemanticExtractionOptions{},
			time.Now(),
		)
		assert.Nil(t, result)
	})

	t.Run("nil class node should not panic", func(t *testing.T) {
		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, "# empty")
		result := parser.ParsePythonClass(
			context.Background(),
			parseTree,
			nil,
			"test",
			outbound.SemanticExtractionOptions{},
			time.Now(),
		)
		assert.Nil(t, result)
	})

	t.Run("malformed class should return nil or handle gracefully", func(t *testing.T) {
		malformedCode := `class IncompleteClass
    # Missing colon and body`

		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, malformedCode)
		classNodes := parseTree.GetNodesByType("class_definition")

		if len(classNodes) > 0 {
			result := parser.ParsePythonClass(
				context.Background(),
				parseTree,
				classNodes[0],
				"test",
				outbound.SemanticExtractionOptions{},
				time.Now(),
			)
			// Should either return nil or a partial result, but not panic
			if result != nil {
				assert.NotEmpty(t, result.Name)
			}
		}
	})
}

// TestPythonClassParser_PrivateVisibilityFiltering tests visibility filtering for classes.
// This is a RED PHASE test that defines expected behavior for class member visibility filtering.
func TestPythonClassParser_PrivateVisibilityFiltering(t *testing.T) {
	sourceCode := `class TestClass:
    """Test class for visibility filtering."""
    
    def public_method(self):
        """Public method."""
        pass
    
    def _protected_method(self):
        """Protected method."""
        pass
    
    def __private_method(self):
        """Private method."""
        pass
    
    def __init__(self):
        """Constructor."""
        pass
    
    def __str__(self):
        """String representation."""
        return "TestClass"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	classNodes := parseTree.GetNodesByType("class_definition")
	require.Len(t, classNodes, 1)

	// Test with IncludePrivate: false
	optionsNoPrivate := outbound.SemanticExtractionOptions{
		IncludePrivate: false,
		MaxDepth:       10, // Must set MaxDepth > 0 to enable child extraction
	}

	result := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[0],
		"test",
		optionsNoPrivate,
		time.Now(),
	)
	require.NotNil(t, result)

	// Should only include public methods
	publicMethod := findChildByName(result.ChildChunks, "public_method")
	protectedMethod := findChildByName(result.ChildChunks, "_protected_method")
	privateMethod := findChildByName(result.ChildChunks, "__private_method")

	assert.NotNil(t, publicMethod, "Should include public method")
	assert.Nil(t, protectedMethod, "Should exclude protected method when IncludePrivate=false")
	assert.Nil(t, privateMethod, "Should exclude private method when IncludePrivate=false")

	// Test with IncludePrivate: true
	optionsIncludePrivate := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
		MaxDepth:       10, // Must set MaxDepth > 0 to enable child extraction
	}

	allResult := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[0],
		"test",
		optionsIncludePrivate,
		time.Now(),
	)
	require.NotNil(t, allResult)

	// Should include all methods
	assert.NotNil(t, findChildByName(allResult.ChildChunks, "public_method"))
	assert.NotNil(t, findChildByName(allResult.ChildChunks, "_protected_method"))
	assert.NotNil(t, findChildByName(allResult.ChildChunks, "__private_method"))
}

// Helper functions for testing

// findChildByName finds a child chunk by name.
func findChildByName(chunks []outbound.SemanticCodeChunk, name string) *outbound.SemanticCodeChunk {
	for i, chunk := range chunks {
		if chunk.Name == name {
			return &chunks[i]
		}
	}
	return nil
}

// findDependencyByName finds a dependency by name.
func findDependencyByName(deps []outbound.DependencyReference, name string) *outbound.DependencyReference {
	for i, dep := range deps {
		if dep.Name == name {
			return &deps[i]
		}
	}
	return nil
}

// RED PHASE: Edge case tests for class docstring extraction with string manipulation bugs

// TestPythonClassParser_ClassDocstringWithEmbeddedDoubleQuotes tests class docstring with embedded double quotes.
// EXPECTED TO FAIL: Current implementation uses strings.Trim which doesn't handle embedded quotes correctly.
func TestPythonClassParser_ClassDocstringWithEmbeddedDoubleQuotes(t *testing.T) {
	sourceCode := `class Speaker:
    """A class that says \"hello\" loudly"""
    pass
`
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	classNodes := parseTree.GetNodesByType("class_definition")
	require.Len(t, classNodes, 1)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	result := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[0],
		"test_module",
		options,
		time.Now(),
	)
	require.NotNil(t, result)

	// Should extract: A class that says "hello" loudly
	// Current bug: strings.Trim will fail on embedded quotes
	assert.Equal(t, "A class that says \"hello\" loudly", result.Documentation,
		"Class docstring should preserve embedded double quotes")
}

// TestPythonClassParser_ClassDocstringWithEmbeddedSingleQuote tests class docstring with embedded single quote.
// EXPECTED TO FAIL: Current implementation uses strings.Trim which doesn't handle embedded quotes correctly.
func TestPythonClassParser_ClassDocstringWithEmbeddedSingleQuote(t *testing.T) {
	sourceCode := `class Validator:
    '''Checks if it's valid'''
    pass
`
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	classNodes := parseTree.GetNodesByType("class_definition")
	require.Len(t, classNodes, 1)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	result := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[0],
		"test_module",
		options,
		time.Now(),
	)
	require.NotNil(t, result)

	// Should extract: Checks if it's valid
	assert.Equal(t, "Checks if it's valid", result.Documentation,
		"Class docstring should preserve embedded single quote")
}

// TestPythonClassParser_ClassDocstringWithMixedQuotes tests class docstring with mixed quote types.
// EXPECTED TO FAIL: String manipulation fails with complex quote combinations.
func TestPythonClassParser_ClassDocstringWithMixedQuotes(t *testing.T) {
	sourceCode := `class Conversation:
    """Handles phrases like 'hi' and \"hello\" """
    pass
`
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	classNodes := parseTree.GetNodesByType("class_definition")
	require.Len(t, classNodes, 1)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	result := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[0],
		"test_module",
		options,
		time.Now(),
	)
	require.NotNil(t, result)

	// Should extract with both quote types preserved
	assert.Equal(t, "Handles phrases like 'hi' and \"hello\" ", result.Documentation,
		"Class docstring should preserve mixed quote types")
}

// TestPythonClassParser_ClassDocstringWithRawString tests raw string class docstring.
// EXPECTED TO FAIL: Current implementation may not handle r-string prefix correctly.
func TestPythonClassParser_ClassDocstringWithRawString(t *testing.T) {
	sourceCode := `class PathHandler:
    r"""Handles paths like C:\Users\name\file.txt"""
    pass
`
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	classNodes := parseTree.GetNodesByType("class_definition")
	require.Len(t, classNodes, 1)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	result := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[0],
		"test_module",
		options,
		time.Now(),
	)
	require.NotNil(t, result)

	// Should extract: Handles paths like C:\Users\name\file.txt
	assert.Equal(t, `Handles paths like C:\Users\name\file.txt`, result.Documentation,
		"Class docstring should extract raw string content correctly")
}

// TestPythonClassParser_ClassDocstringMultilineWithQuotes tests multiline class docstring with quotes.
// EXPECTED TO FAIL: String trimming may affect internal structure.
func TestPythonClassParser_ClassDocstringMultilineWithQuotes(t *testing.T) {
	sourceCode := `class DataProcessor:
    """
    A class for processing data.

    Handles formats like:
    - JSON: {\"key\": \"value\"}
    - CSV: \"col1\",\"col2\",\"col3\"

    Methods include 'parse' and \"process\"
    """
    pass
`
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	classNodes := parseTree.GetNodesByType("class_definition")
	require.Len(t, classNodes, 1)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	result := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[0],
		"test_module",
		options,
		time.Now(),
	)
	require.NotNil(t, result)

	doc := result.Documentation
	// Should preserve internal structure with quotes
	assert.Contains(t, doc, "{\"key\": \"value\"}",
		"Class docstring should preserve JSON examples with quotes")
	assert.Contains(t, doc, "\"col1\",\"col2\",\"col3\"",
		"Class docstring should preserve CSV examples with quotes")
	assert.Contains(t, doc, "'parse'",
		"Class docstring should preserve single-quoted references")
	assert.Contains(t, doc, "\"process\"",
		"Class docstring should preserve double-quoted references")
}

// TestPythonClassParser_MethodDocstringWithEmbeddedQuotes tests method docstring with embedded quotes.
// EXPECTED TO FAIL: Method docstrings also use the same flawed string manipulation.
func TestPythonClassParser_MethodDocstringWithEmbeddedQuotes(t *testing.T) {
	sourceCode := `class Formatter:
    """Formatter class"""

    def format_message(self):
        """Formats messages like \"Hello, World!\" """
        pass
`
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	classNodes := parseTree.GetNodesByType("class_definition")
	require.Len(t, classNodes, 1)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		MaxDepth:             10,
	}

	result := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[0],
		"test_module",
		options,
		time.Now(),
	)
	require.NotNil(t, result)

	// Find the method
	formatMethod := findChildByName(result.ChildChunks, "format_message")
	require.NotNil(t, formatMethod, "Should find format_message method")

	// Should extract: Formats messages like "Hello, World!"
	assert.Equal(t, "Formats messages like \"Hello, World!\" ", formatMethod.Documentation,
		"Method docstring should preserve embedded quotes")
}

// TestPythonClassParser_ClassDocstringWithCodeExamples tests class docstring with code examples.
// EXPECTED TO FAIL: Code examples with quotes may be mangled by string trimming.
func TestPythonClassParser_ClassDocstringWithCodeExamples(t *testing.T) {
	sourceCode := `class Example:
    """
    Example class showing usage:

        obj = Example()
        obj.method(\"param\")
        result = obj.get('key')

    Returns data in format: {\"status\": \"ok\"}
    """
    pass
`
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	classNodes := parseTree.GetNodesByType("class_definition")
	require.Len(t, classNodes, 1)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	result := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[0],
		"test_module",
		options,
		time.Now(),
	)
	require.NotNil(t, result)

	doc := result.Documentation
	// Should preserve code examples with various quote styles
	assert.Contains(t, doc, "obj.method(\"param\")",
		"Class docstring should preserve code with double quotes")
	assert.Contains(t, doc, "obj.get('key')",
		"Class docstring should preserve code with single quotes")
	assert.Contains(t, doc, "{\"status\": \"ok\"}",
		"Class docstring should preserve JSON examples")
}

// TestPythonClassParser_ClassDocstringWithFString tests f-string class docstring.
// EXPECTED TO FAIL: f-string prefix may not be handled correctly.
func TestPythonClassParser_ClassDocstringWithFString(t *testing.T) {
	sourceCode := `class Config:
    f"""Configuration class with settings"""
    pass
`
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser := NewPythonClassParser()

	classNodes := parseTree.GetNodesByType("class_definition")
	require.Len(t, classNodes, 1)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	result := parser.ParsePythonClass(
		context.Background(),
		parseTree,
		classNodes[0],
		"test_module",
		options,
		time.Now(),
	)
	require.NotNil(t, result)

	// Should extract content without f prefix
	assert.Equal(t, "Configuration class with settings", result.Documentation,
		"Class docstring should extract f-string content correctly")
}

// Note: RED PHASE stub implementations removed - now implemented in classes.go
