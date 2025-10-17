package pythonparser

import (
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

// TestPythonParser_NewPythonParser tests creation of Python parser.
// This is a RED PHASE test that defines expected behavior for Python parser creation.
func TestPythonParser_NewPythonParser(t *testing.T) {
	parser, err := NewPythonParser()
	require.NoError(t, err, "Creating Python parser should not fail")
	require.NotNil(t, parser, "Python parser should not be nil")

	// Test supported language
	lang := parser.GetSupportedLanguage()
	assert.Equal(t, valueobject.LanguagePython, lang.Name())
}

// TestPythonParser_GetSupportedConstructTypes tests supported construct types.
// This is a RED PHASE test that defines expected Python construct types.
func TestPythonParser_GetSupportedConstructTypes(t *testing.T) {
	parser, err := NewPythonParser()
	require.NoError(t, err)

	types := parser.GetSupportedConstructTypes()
	expectedTypes := []outbound.SemanticConstructType{
		outbound.ConstructFunction,
		outbound.ConstructMethod,
		outbound.ConstructClass,
		outbound.ConstructVariable,
		outbound.ConstructConstant,
		outbound.ConstructModule,
	}

	require.Len(t, types, len(expectedTypes))
	for _, expectedType := range expectedTypes {
		assert.Contains(t, types, expectedType)
	}
}

// TestPythonParser_IsSupported tests language support checking.
// This is a RED PHASE test that defines expected Python language support behavior.
func TestPythonParser_IsSupported(t *testing.T) {
	parser, err := NewPythonParser()
	require.NoError(t, err)

	// Should support Python
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)
	assert.True(t, parser.IsSupported(pythonLang))

	// Should not support Go
	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	require.NoError(t, err)
	assert.False(t, parser.IsSupported(goLang))
}

// TestPythonParser_ExtractFunctions tests Python function extraction.
// This is a RED PHASE test that defines expected behavior for Python function extraction.
func TestPythonParser_ExtractFunctions(t *testing.T) {
	sourceCode := `# math_utils.py

def add(a, b):
    """Add two numbers together."""
    return a + b

async def fetch_data(url: str) -> dict:
    """Async function to fetch data."""
    import aiohttp
    async with aiohttp.ClientSession() as session:
        async with session.get(url) as response:
            return await response.json()

def process_data(*args, **kwargs):
    """Function with variadic parameters."""
    return args, kwargs

class Calculator:
    def multiply(self, x: float, y: float) -> float:
        """Method in class."""
        return x * y
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find 3 functions (add, fetch_data, process_data) and 1 method (multiply)
	require.Len(t, functions, 4)

	// Test regular function
	addFunc := testhelpers.FindChunkByName(functions, "add")
	require.NotNil(t, addFunc, "Should find 'add' function")
	assert.Equal(t, outbound.ConstructFunction, addFunc.Type)
	assert.Equal(t, "add", addFunc.Name)
	assert.Equal(t, "math_utils.add", addFunc.QualifiedName)
	assert.Contains(t, addFunc.Documentation, "Add two numbers together")
	assert.False(t, addFunc.IsAsync)
	assert.Len(t, addFunc.Parameters, 2)

	// Test async function
	fetchFunc := testhelpers.FindChunkByName(functions, "fetch_data")
	require.NotNil(t, fetchFunc, "Should find 'fetch_data' function")
	assert.Equal(t, outbound.ConstructFunction, fetchFunc.Type)
	assert.True(t, fetchFunc.IsAsync)
	assert.Equal(t, "dict", fetchFunc.ReturnType)
	assert.Len(t, fetchFunc.Parameters, 1)
	assert.Equal(t, "str", fetchFunc.Parameters[0].Type)

	// Test variadic function
	processFunc := testhelpers.FindChunkByName(functions, "process_data")
	require.NotNil(t, processFunc, "Should find 'process_data' function")
	assert.Len(t, processFunc.Parameters, 2)
	assert.True(t, processFunc.Parameters[0].IsVariadic) // *args
	assert.True(t, processFunc.Parameters[1].IsVariadic) // **kwargs

	// Test method
	methodFunc := testhelpers.FindChunkByName(functions, "multiply")
	require.NotNil(t, methodFunc, "Should find 'multiply' method")
	assert.Equal(t, outbound.ConstructMethod, methodFunc.Type)
	assert.Equal(t, "Calculator.multiply", methodFunc.QualifiedName)
}

// TestPythonParser_ExtractClasses tests Python class extraction.
// This is a RED PHASE test that defines expected behavior for Python class extraction.
func TestPythonParser_ExtractClasses(t *testing.T) {
	sourceCode := `# models.py

class Animal:
    """Base animal class."""
    
    def __init__(self, name: str):
        self.name = name
    
    def speak(self) -> str:
        raise NotImplementedError

@dataclass
class Dog(Animal):
    """Dog class with inheritance."""
    
    breed: str
    
    def speak(self) -> str:
        return f"{self.name} says Woof!"
    
    @staticmethod
    def get_species() -> str:
        return "Canis lupus"
    
    @classmethod
    def from_string(cls, data: str) -> 'Dog':
        parts = data.split(',')
        return cls(parts[0], parts[1])

class Cat(Animal):
    def __init__(self, name: str, indoor: bool = True):
        super().__init__(name)
        self._indoor = indoor
    
    def speak(self) -> str:
        return f"{self.name} says Meow!"

class GenericContainer:
    """Generic container with type variables."""
    
    def __init__(self):
        self.items = []
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	classes, err := parser.ExtractClasses(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find 4 classes
	require.Len(t, classes, 4)

	// Test base class
	animalClass := testhelpers.FindChunkByName(classes, "Animal")
	require.NotNil(t, animalClass, "Should find 'Animal' class")
	assert.Equal(t, outbound.ConstructClass, animalClass.Type)
	assert.Equal(t, "Animal", animalClass.Name)
	assert.Equal(t, "models.Animal", animalClass.QualifiedName)
	assert.Contains(t, animalClass.Documentation, "Base animal class")
	assert.Empty(t, animalClass.Dependencies) // No inheritance dependencies

	// Test inheritance
	dogClass := testhelpers.FindChunkByName(classes, "Dog")
	require.NotNil(t, dogClass, "Should find 'Dog' class")
	assert.Equal(t, "models.Dog", dogClass.QualifiedName)
	// Check inheritance through dependencies
	assert.GreaterOrEqual(t, len(dogClass.Dependencies), 1, "Should have inheritance dependency")
	assert.Len(t, dogClass.Annotations, 1) // @dataclass decorator

	// Test Cat class with private attribute convention
	catClass := testhelpers.FindChunkByName(classes, "Cat")
	require.NotNil(t, catClass, "Should find 'Cat' class")

	// Test generic container
	containerClass := testhelpers.FindChunkByName(classes, "GenericContainer")
	require.NotNil(t, containerClass, "Should find 'GenericContainer' class")
}

// TestPythonParser_ExtractModules tests Python module extraction.
// This is a RED PHASE test that defines expected behavior for Python module extraction.
func TestPythonParser_ExtractModules(t *testing.T) {
	sourceCode := `#!/usr/bin/env python3
"""
This is a module for utility functions.

Author: Developer
Version: 1.0.0
"""

__version__ = "1.0.0"
__author__ = "Developer"

import os
import sys
from typing import List, Dict

def main():
    pass

if __name__ == "__main__":
    main()
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
		IncludeMetadata:      true,
	}

	modules, err := parser.ExtractModules(context.Background(), parseTree, options)
	require.NoError(t, err)

	require.Len(t, modules, 1)
	module := modules[0]

	assert.Equal(t, outbound.ConstructModule, module.Type)
	assert.Contains(t, module.Name, "utility") // Should extract module name
	assert.Contains(t, module.Documentation, "This is a module for utility functions")
	assert.Contains(t, module.Documentation, "Author: Developer")
	assert.Contains(t, module.Documentation, "Version: 1.0.0")

	// Should have metadata about module-level variables
	assert.Contains(t, module.Metadata, "version")
	assert.Contains(t, module.Metadata, "author")
}

// TestPythonParser_ExtractVariables tests Python variable extraction.
// This is a RED PHASE test that defines expected behavior for Python variable extraction.
func TestPythonParser_ExtractVariables(t *testing.T) {
	sourceCode := `# config.py

# Module-level constants
API_VERSION = "v1"
MAX_RETRIES = 3
DEBUG_MODE = True

# Type annotations
connection_timeout: float = 30.0
server_config: Dict[str, str] = {}

# Multiple assignment
x, y, z = 1, 2, 3

# Tuple unpacking
first_name, last_name = "John", "Doe"

class Settings:
    # Class variables
    default_theme = "dark"
    _private_key = "secret"
    
    def __init__(self):
        # Instance variables
        self.user_id = 12345
        self._session_token = None

def process():
    # Local variables
    temp_data = []
    result = None
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		MaxDepth:        10,
	}

	variables, err := parser.ExtractVariables(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find module-level variables, class variables, and annotated variables
	// Local variables in functions are typically not extracted at this level
	assert.GreaterOrEqual(t, len(variables), 6)

	// Test module-level constant
	apiVar := testhelpers.FindChunkByName(variables, "API_VERSION")
	require.NotNil(t, apiVar, "Should find 'API_VERSION' variable")
	assert.Equal(t, outbound.ConstructConstant, apiVar.Type)
	assert.Equal(t, "API_VERSION", apiVar.Name)
	assert.Equal(t, outbound.Public, apiVar.Visibility)
	assert.Equal(t, "str", apiVar.ReturnType)

	// Test annotated variable
	timeoutVar := testhelpers.FindChunkByName(variables, "connection_timeout")
	require.NotNil(t, timeoutVar, "Should find 'connection_timeout' variable")
	assert.Equal(t, outbound.ConstructVariable, timeoutVar.Type)
	assert.Equal(t, "float", timeoutVar.ReturnType)

	// Test multiple assignment variables
	xVar := testhelpers.FindChunkByName(variables, "x")
	require.NotNil(t, xVar, "Should find 'x' variable")
	yVar := testhelpers.FindChunkByName(variables, "y")
	require.NotNil(t, yVar, "Should find 'y' variable")
	zVar := testhelpers.FindChunkByName(variables, "z")
	require.NotNil(t, zVar, "Should find 'z' variable")

	// Test class variables
	defaultThemeVar := testhelpers.FindChunkByName(variables, "default_theme")
	require.NotNil(t, defaultThemeVar, "Should find 'default_theme' class variable")
	assert.Equal(t, outbound.Public, defaultThemeVar.Visibility)

	privateKeyVar := testhelpers.FindChunkByName(variables, "_private_key")
	require.NotNil(t, privateKeyVar, "Should find '_private_key' class variable")
	assert.Equal(t, outbound.Private, privateKeyVar.Visibility)
}

// TestPythonParser_ExtractImports tests Python import extraction.
// This is a RED PHASE test that defines expected behavior for Python import extraction.
func TestPythonParser_ExtractImports(t *testing.T) {
	sourceCode := `# imports_example.py

import os
import sys
import json as JSON
from typing import List, Dict, Optional
from collections import defaultdict, Counter
from .local_module import LocalClass
from ..parent_module import ParentClass
from mypackage.submodule import *
import numpy as np
from datetime import datetime, timedelta
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{}

	imports, err := parser.ExtractImports(context.Background(), parseTree, options)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(imports), 8)

	// Test standard import
	osImport := findImportByModule(imports, "os")
	require.NotNil(t, osImport, "Should find 'os' import")
	assert.Equal(t, "os", osImport.Path)
	assert.Empty(t, osImport.Alias)
	assert.Empty(t, osImport.ImportedSymbols)

	// Test aliased import
	jsonImport := findImportByModule(imports, "json")
	require.NotNil(t, jsonImport, "Should find 'json' import")
	assert.Equal(t, "json", jsonImport.Path)
	assert.Equal(t, "JSON", jsonImport.Alias)

	// Test from import with specific names
	typingImport := findImportByModule(imports, "typing")
	require.NotNil(t, typingImport, "Should find 'typing' import")
	assert.Equal(t, "typing", typingImport.Path)
	expectedNames := []string{"List", "Dict", "Optional"}
	for _, name := range expectedNames {
		assert.Contains(t, typingImport.ImportedSymbols, name)
	}

	// Test relative import
	localImport := findImportByModule(imports, "local_module")
	require.NotNil(t, localImport, "Should find local module import")
	// Check if relative import info is stored in metadata
	assert.Contains(t, localImport.Metadata, "is_relative")
	assert.Equal(t, true, localImport.Metadata["is_relative"])
	assert.Equal(t, 1, localImport.Metadata["relative_level"]) // Single dot

	// Test parent relative import
	parentImport := findImportByModule(imports, "parent_module")
	require.NotNil(t, parentImport, "Should find parent module import")
	assert.Equal(t, true, parentImport.Metadata["is_relative"])
	assert.Equal(t, 2, parentImport.Metadata["relative_level"]) // Double dot

	// Test wildcard import
	wildcardImport := findImportByModule(imports, "mypackage.submodule")
	require.NotNil(t, wildcardImport, "Should find wildcard import")
	assert.True(t, wildcardImport.IsWildcard)

	// Test numpy aliased import
	numpyImport := findImportByModule(imports, "numpy")
	require.NotNil(t, numpyImport, "Should find 'numpy' import")
	assert.Equal(t, "np", numpyImport.Alias)
}

// TestPythonParser_ExtractInterfaces tests Python interface/protocol extraction.
// This is a RED PHASE test that defines expected behavior for Python protocol extraction.
func TestPythonParser_ExtractInterfaces(t *testing.T) {
	sourceCode := `# protocols.py

from typing import Protocol, runtime_checkable
from abc import ABC, abstractmethod

@runtime_checkable
class Drawable(Protocol):
    """Protocol for drawable objects."""
    
    def draw(self) -> None:
        ...
    
    def get_area(self) -> float:
        ...

class Shape(ABC):
    """Abstract base class for shapes."""
    
    @abstractmethod
    def area(self) -> float:
        pass
    
    @abstractmethod
    def perimeter(self) -> float:
        pass
    
    def describe(self) -> str:
        return f"A shape with area {self.area()}"

class Serializable(Protocol):
    def serialize(self) -> dict:
        ...
    
    def deserialize(self, data: dict) -> None:
        ...
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
	}

	interfaces, err := parser.ExtractInterfaces(context.Background(), parseTree, options)
	require.NoError(t, err)

	require.Len(t, interfaces, 3) // Drawable Protocol, Shape ABC, Serializable Protocol

	// Test Protocol
	drawableInterface := testhelpers.FindChunkByName(interfaces, "Drawable")
	require.NotNil(t, drawableInterface, "Should find 'Drawable' protocol")
	assert.Equal(t, outbound.ConstructInterface, drawableInterface.Type)
	assert.Equal(t, "Drawable", drawableInterface.Name)
	assert.Contains(t, drawableInterface.Documentation, "Protocol for drawable objects")
	assert.Len(t, drawableInterface.Annotations, 1) // @runtime_checkable

	// Test ABC
	shapeInterface := testhelpers.FindChunkByName(interfaces, "Shape")
	require.NotNil(t, shapeInterface, "Should find 'Shape' ABC")
	assert.Equal(t, outbound.ConstructInterface, shapeInterface.Type)
	// Check ABC inheritance through dependencies
	assert.GreaterOrEqual(t, len(shapeInterface.Dependencies), 1, "Should have ABC dependency")

	// Test simple Protocol
	serializableInterface := testhelpers.FindChunkByName(interfaces, "Serializable")
	require.NotNil(t, serializableInterface, "Should find 'Serializable' protocol")
}

// TestPythonParser_ErrorHandling tests error conditions.
// This is a RED PHASE test that defines expected behavior for error handling.
func TestPythonParser_ErrorHandling(t *testing.T) {
	parser, err := NewPythonParser()
	require.NoError(t, err)

	t.Run("nil parse tree should return error", func(t *testing.T) {
		options := outbound.SemanticExtractionOptions{}

		_, err := parser.ExtractFunctions(context.Background(), nil, options)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse tree cannot be nil")

		_, err = parser.ExtractClasses(context.Background(), nil, options)
		require.Error(t, err)

		_, err = parser.ExtractVariables(context.Background(), nil, options)
		require.Error(t, err)

		_, err = parser.ExtractImports(context.Background(), nil, options)
		require.Error(t, err)

		_, err = parser.ExtractModules(context.Background(), nil, options)
		require.Error(t, err)

		_, err = parser.ExtractInterfaces(context.Background(), nil, options)
		require.Error(t, err)
	})

	t.Run("unsupported language should return error", func(t *testing.T) {
		goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, goLang, "package main")
		options := outbound.SemanticExtractionOptions{}

		_, err = parser.ExtractFunctions(context.Background(), parseTree, options)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported language")
	})

	t.Run("malformed Python code should not panic", func(t *testing.T) {
		malformedCode := `# Incomplete Python code
def incomplete_function(
    # Missing closing parenthesis and function body
`
		language, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, malformedCode)
		options := outbound.SemanticExtractionOptions{}

		// Should not panic, even with malformed code
		functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
		// May return error or empty results, but should not panic
		if err == nil {
			assert.NotNil(t, functions)
		}
	})
}

// TestPythonParser_PrivateVisibilityFiltering tests visibility filtering.
// This is a RED PHASE test that defines expected behavior for Python private member filtering.
func TestPythonParser_PrivateVisibilityFiltering(t *testing.T) {
	sourceCode := `# visibility_test.py

def public_function():
    pass

def _private_function():
    pass

def __dunder_method__():
    pass

class TestClass:
    def public_method(self):
        pass
    
    def _protected_method(self):
        pass
    
    def __private_method(self):
        pass

# Variables
public_var = "public"
_protected_var = "protected"
__private_var = "private"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	// Test with IncludePrivate: false
	optionsNoPrivate := outbound.SemanticExtractionOptions{
		IncludePrivate: false,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, optionsNoPrivate)
	require.NoError(t, err)

	// Should only include public functions
	publicFuncFound := testhelpers.FindChunkByName(functions, "public_function") != nil
	privateFuncFound := testhelpers.FindChunkByName(functions, "_private_function") != nil
	dunderFuncFound := testhelpers.FindChunkByName(functions, "__dunder_method__") != nil

	assert.True(t, publicFuncFound, "Should include public function")
	assert.False(t, privateFuncFound, "Should exclude private function when IncludePrivate=false")
	assert.False(t, dunderFuncFound, "Should exclude dunder function when IncludePrivate=false")

	// Test with IncludePrivate: true
	optionsIncludePrivate := outbound.SemanticExtractionOptions{
		IncludePrivate: true,
	}

	allFunctions, err := parser.ExtractFunctions(context.Background(), parseTree, optionsIncludePrivate)
	require.NoError(t, err)

	// Should include all functions
	assert.NotNil(t, testhelpers.FindChunkByName(allFunctions, "public_function"))
	assert.NotNil(t, testhelpers.FindChunkByName(allFunctions, "_private_function"))
	assert.NotNil(t, testhelpers.FindChunkByName(allFunctions, "__dunder_method__"))
}

// Helper functions for testing (these will fail in RED phase)

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

	// Get Python grammar from forest
	grammar := forest.GetLanguage("python")
	require.NotNil(t, grammar, "Failed to get Python grammar from forest")

	// Create tree-sitter parser
	parser := tree_sitter.NewParser()
	require.NotNil(t, parser, "Failed to create tree-sitter parser")

	success := parser.SetLanguage(grammar)
	require.True(t, success, "Failed to set Python language")

	// Parse the source code
	tree, err := parser.ParseString(context.Background(), nil, []byte(sourceCode))
	require.NoError(t, err, "Failed to parse Python source")
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

// convertTreeSitterNode converts a tree-sitter node to domain ParseNode recursively.
func convertTreeSitterNode(node tree_sitter.Node, depth int) (*valueobject.ParseNode, int, int) {
	if node.IsNull() {
		return nil, 0, depth
	}

	// Convert tree-sitter node to domain ParseNode
	parseNode := &valueobject.ParseNode{
		Type:      node.Type(),
		StartByte: safeUintToUint32(node.StartByte()),
		EndByte:   safeUintToUint32(node.EndByte()),
		StartPos: valueobject.Position{
			Row:    safeUintToUint32(node.StartPoint().Row),
			Column: safeUintToUint32(node.StartPoint().Column),
		},
		EndPos: valueobject.Position{
			Row:    safeUintToUint32(node.EndPoint().Row),
			Column: safeUintToUint32(node.EndPoint().Column),
		},
		Children: make([]*valueobject.ParseNode, 0),
	}

	nodeCount := 1
	maxDepth := depth

	// Convert children recursively
	childCount := node.ChildCount()
	for i := range childCount {
		childNode := node.Child(i)
		if childNode.IsNull() {
			continue
		}

		childParseNode, childNodeCount, childMaxDepth := convertTreeSitterNode(childNode, depth+1)
		if childParseNode != nil {
			parseNode.Children = append(parseNode.Children, childParseNode)
			nodeCount += childNodeCount
			if childMaxDepth > maxDepth {
				maxDepth = childMaxDepth
			}
		}
	}

	return parseNode, nodeCount, maxDepth
}

// safeUintToUint32 safely converts uint to uint32 with bounds checking.
func safeUintToUint32(val uint) uint32 {
	if val > uint(^uint32(0)) {
		return ^uint32(0) // Return max uint32 if overflow would occur
	}
	return uint32(val) // #nosec G115 - bounds checked above
}

// findImportByModule finds an import declaration by module name.
func findImportByModule(imports []outbound.ImportDeclaration, moduleName string) *outbound.ImportDeclaration {
	for i, imp := range imports {
		if imp.Path == moduleName {
			return &imports[i]
		}
	}
	return nil
}

// RED PHASE: Edge case tests for docstring extraction with string manipulation bugs

// TestPythonParser_ExtractFunctions_DocstringWithEmbeddedDoubleQuotes tests docstring with embedded double quotes.
// EXPECTED TO FAIL: Current implementation uses strings.TrimPrefix/TrimSuffix which breaks on embedded quotes.
func TestPythonParser_ExtractFunctions_DocstringWithEmbeddedDoubleQuotes(t *testing.T) {
	sourceCode := `def greet():
    """He said \"hello\" to me"""
    pass
`
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// Should extract: He said "hello" to me
	// Current bug: TrimPrefix/TrimSuffix will fail on embedded quotes
	assert.Equal(t, "He said \"hello\" to me", functions[0].Documentation,
		"Docstring should preserve embedded double quotes")
}

// TestPythonParser_ExtractFunctions_DocstringWithEmbeddedSingleQuote tests docstring with embedded single quote.
// EXPECTED TO FAIL: Current implementation uses strings.TrimPrefix/TrimSuffix which breaks on embedded quotes.
func TestPythonParser_ExtractFunctions_DocstringWithEmbeddedSingleQuote(t *testing.T) {
	sourceCode := `def analyze():
    '''It's working correctly'''
    pass
`
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// Should extract: It's working correctly
	// Current bug: TrimPrefix/TrimSuffix will fail on embedded single quote
	assert.Equal(t, "It's working correctly", functions[0].Documentation,
		"Docstring should preserve embedded single quote")
}

// TestPythonParser_ExtractFunctions_DocstringWithFString tests f-string docstring.
// EXPECTED TO FAIL: Current implementation may not handle f-string prefix correctly.
func TestPythonParser_ExtractFunctions_DocstringWithFString(t *testing.T) {
	sourceCode := `def process():
    f"""This is a docstring with f-string syntax"""
    pass
`
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// Should extract content without f prefix
	assert.Equal(t, "This is a docstring with f-string syntax", functions[0].Documentation,
		"Docstring should extract f-string content correctly")
}

// TestPythonParser_ExtractFunctions_DocstringWithRawString tests raw string docstring.
// EXPECTED TO FAIL: Current implementation may not handle r-string prefix correctly.
func TestPythonParser_ExtractFunctions_DocstringWithRawString(t *testing.T) {
	sourceCode := `def path_handler():
    r"""Raw string: C:\Users\path\to\file"""
    pass
`
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// Should extract: Raw string: C:\Users\path\to\file
	assert.Equal(t, `Raw string: C:\Users\path\to\file`, functions[0].Documentation,
		"Docstring should extract raw string content correctly")
}

// TestPythonParser_ExtractFunctions_DocstringWithMixedQuotes tests docstring with both quote types.
// EXPECTED TO FAIL: String manipulation approach fails with complex quote combinations.
func TestPythonParser_ExtractFunctions_DocstringWithMixedQuotes(t *testing.T) {
	sourceCode := `def describe():
    """She said 'hello' and he replied \"hi\" """
    pass
`
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// Should extract with both quote types preserved
	assert.Equal(t, "She said 'hello' and he replied \"hi\" ", functions[0].Documentation,
		"Docstring should preserve mixed quote types")
}

// TestPythonParser_ExtractFunctions_DocstringWithEscapedQuotes tests escaped quotes in docstring.
// EXPECTED TO FAIL: String manipulation doesn't handle escape sequences properly.
func TestPythonParser_ExtractFunctions_DocstringWithEscapedQuotes(t *testing.T) {
	sourceCode := `def parse_json():
    "Parses JSON: {\"key\": \"value\"}"
    pass
`
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// Should extract with escaped quotes
	assert.Contains(t, functions[0].Documentation, `"key"`,
		"Docstring should preserve escaped quotes correctly")
}

// TestPythonParser_ExtractFunctions_DocstringMultilineWithCode tests multiline docstring with code.
// EXPECTED TO FAIL: May extract incorrectly if string manipulation affects internal structure.
func TestPythonParser_ExtractFunctions_DocstringMultilineWithCode(t *testing.T) {
	sourceCode := `def example():
    """
    Example function with code sample.

    Usage:
        result = example()
        print(\"Result:\", result)

    Returns:
        str: The result string with \"quotes\"
    """
    pass
`
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	doc := functions[0].Documentation
	// Should preserve internal structure with quotes
	assert.Contains(t, doc, "print(\"Result:\", result)",
		"Docstring should preserve code examples with quotes")
	assert.Contains(t, doc, "str: The result string with \"quotes\"",
		"Docstring should preserve inline quotes")
}

// TestPythonParser_ExtractFunctions_DocstringWithTripleQuotesInside tests triple quotes inside docstring.
// EXPECTED TO FAIL: TrimPrefix/TrimSuffix can't handle nested triple quotes correctly.
func TestPythonParser_ExtractFunctions_DocstringWithTripleQuotesInside(t *testing.T) {
	sourceCode := "def format_docstring():\n" +
		"    \"\"\"\n" +
		"    Formats docstrings that look like:\n" +
		"    \\\"\\\"\\\"Example docstring\\\"\\\"\\\"\n" +
		"    \"\"\"\n" +
		"    pass\n"
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// Should preserve escaped triple quotes
	assert.Contains(t, functions[0].Documentation, "\\\"\\\"\\\"Example docstring\\\"\\\"\\\"",
		"Docstring should preserve escaped triple quotes")
}

// TestPythonParser_ExtractFunctions_DocstringWithUnicode tests Unicode in docstring.
// EXPECTED TO FAIL: String manipulation may not handle Unicode correctly.
func TestPythonParser_ExtractFunctions_DocstringWithUnicode(t *testing.T) {
	sourceCode := "def greet_international():\n" +
		"    \"\"\"Say 'hello' in different languages: 你好, مرحبا, Здравствуйте\"\"\"\n" +
		"    pass\n"
	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeDocumentation: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// Should preserve Unicode characters and quotes
	assert.Contains(t, functions[0].Documentation, "你好",
		"Docstring should preserve Unicode characters")
	assert.Contains(t, functions[0].Documentation, "'hello'",
		"Docstring should preserve quotes alongside Unicode")
}

// Note: RED PHASE stub implementations removed - now implemented in python_parser.go

// =============================================================================
// RED PHASE TESTS: Return Type Extraction Edge Cases
// =============================================================================
// These tests expose weaknesses in the current string-based return type extraction
// implementation in extractReturnType() and extractReturnTypeFromArrowAnnotation().
// The current implementation uses string splitting on "->" and basic string manipulation
// which CANNOT correctly handle:
// 1. Generic types with brackets: List[str], Dict[str, int]
// 2. Union types: Union[str, int]
// 3. Pipe unions: str | int
// 4. Optional types: Optional[str]
// 5. Callable types: Callable[[int], str]
// 6. Nested generics: List[Dict[str, int]]
// =============================================================================

// TestPythonParser_ExtractFunctions_GenericReturnType tests extraction of generic return types like List[str].
// RED PHASE: This test exposes that the current string splitting approach cannot handle brackets.
func TestPythonParser_ExtractFunctions_GenericReturnType(t *testing.T) {
	sourceCode := `
from typing import List

def get_items() -> List[str]:
    """Returns a list of strings."""
    pass
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "List[str]"
	// ACTUAL: Likely "List[str]" only if tree-sitter provides it correctly,
	//         but string manipulation in extractReturnTypeFromArrowAnnotation may truncate at ':'
	assert.Equal(t, "List[str]", functions[0].ReturnType,
		"Should extract full generic type including brackets")
}

// TestPythonParser_ExtractFunctions_GenericDictReturnType tests Dict[str, int] return type.
// RED PHASE: Exposes issues with comma-separated generic parameters.
func TestPythonParser_ExtractFunctions_GenericDictReturnType(t *testing.T) {
	sourceCode := `
from typing import Dict

def get_mapping() -> Dict[str, int]:
    """Returns a dictionary mapping strings to integers."""
    pass
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "Dict[str, int]"
	// ACTUAL: May fail because string splitting on ":" in line 234 of functions.go
	//         will truncate at the colon in "Dict[str, int]:"
	assert.Equal(t, "Dict[str, int]", functions[0].ReturnType,
		"Should extract full Dict generic type with both key and value types")
}

// TestPythonParser_ExtractFunctions_UnionReturnType tests Union[str, int] return type.
// RED PHASE: Tests Union type handling with multiple alternatives.
func TestPythonParser_ExtractFunctions_UnionReturnType(t *testing.T) {
	sourceCode := `
from typing import Union

def process_value(val) -> Union[str, int]:
    """Can return either string or int."""
    return val
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "Union[str, int]"
	// ACTUAL: String manipulation may only extract "Union[str"
	assert.Equal(t, "Union[str, int]", functions[0].ReturnType,
		"Should extract full Union type with all alternatives")
}

// TestPythonParser_ExtractFunctions_PipeUnionReturnType tests Python 3.10+ pipe union syntax.
// RED PHASE: Tests modern union syntax with | operator.
func TestPythonParser_ExtractFunctions_PipeUnionReturnType(t *testing.T) {
	sourceCode := `
def modern_union(val) -> str | int:
    """Modern union syntax with pipe operator."""
    return val
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "str | int"
	// ACTUAL: String splitting on " " in line 238 may truncate to "str"
	assert.Equal(t, "str | int", functions[0].ReturnType,
		"Should extract full pipe union type")
}

// TestPythonParser_ExtractFunctions_OptionalReturnType tests Optional[str] return type.
// RED PHASE: Tests Optional type which is shorthand for Union[str, None].
func TestPythonParser_ExtractFunctions_OptionalReturnType(t *testing.T) {
	sourceCode := `
from typing import Optional

def find_user(user_id: int) -> Optional[str]:
    """May return None."""
    return None
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "Optional[str]"
	// ACTUAL: May work if tree-sitter provides it, but string manipulation is fragile
	assert.Equal(t, "Optional[str]", functions[0].ReturnType,
		"Should extract full Optional type")
}

// TestPythonParser_ExtractFunctions_CallableReturnType tests Callable[[int], str] return type.
// RED PHASE: Tests Callable type with parameter and return type specification.
func TestPythonParser_ExtractFunctions_CallableReturnType(t *testing.T) {
	sourceCode := `
from typing import Callable

def get_processor() -> Callable[[int], str]:
    """Returns a function that takes int and returns str."""
    return str
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "Callable[[int], str]"
	// ACTUAL: String manipulation will likely fail with nested brackets
	assert.Equal(t, "Callable[[int], str]", functions[0].ReturnType,
		"Should extract full Callable type with parameter and return type")
}

// TestPythonParser_ExtractFunctions_NestedGenericsReturnType tests List[Dict[str, int]].
// RED PHASE: Tests deeply nested generic types.
func TestPythonParser_ExtractFunctions_NestedGenericsReturnType(t *testing.T) {
	sourceCode := `
from typing import List, Dict

def get_nested_data() -> List[Dict[str, int]]:
    """Complex nested generic type."""
    return [{"a": 1}]
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "List[Dict[str, int]]"
	// ACTUAL: String manipulation cannot handle multiple levels of nesting with commas and colons
	assert.Equal(t, "List[Dict[str, int]]", functions[0].ReturnType,
		"Should extract full nested generic type")
}

// TestPythonParser_ExtractFunctions_ComplexUnionOptionalReturnType tests Optional[Union[str, int]].
// RED PHASE: Tests combination of Optional and Union.
func TestPythonParser_ExtractFunctions_ComplexUnionOptionalReturnType(t *testing.T) {
	sourceCode := `
from typing import Optional, Union

def get_complex_result() -> Optional[Union[str, int]]:
    """Optional union type."""
    return None
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "Optional[Union[str, int]]"
	// ACTUAL: String manipulation will definitely fail with nested generic and comma
	assert.Equal(t, "Optional[Union[str, int]]", functions[0].ReturnType,
		"Should extract full Optional[Union[...]] type")
}

// TestPythonParser_ExtractFunctions_TupleReturnType tests Tuple[float, float, float].
// RED PHASE: Tests Tuple with multiple type parameters.
func TestPythonParser_ExtractFunctions_TupleReturnType(t *testing.T) {
	sourceCode := `
from typing import Tuple

def get_coordinates() -> Tuple[float, float, float]:
    """Returns 3D coordinates."""
    return (0.0, 0.0, 0.0)
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "Tuple[float, float, float]"
	// ACTUAL: String manipulation may truncate after first type
	assert.Equal(t, "Tuple[float, float, float]", functions[0].ReturnType,
		"Should extract full Tuple type with all element types")
}

// TestPythonParser_ExtractFunctions_SetReturnType tests Set[str] return type.
// RED PHASE: Tests Set generic type.
func TestPythonParser_ExtractFunctions_SetReturnType(t *testing.T) {
	sourceCode := `
from typing import Set

def get_unique_tags() -> Set[str]:
    """Returns unique tags."""
    return set()
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "Set[str]"
	// ACTUAL: May work if tree-sitter provides it correctly
	assert.Equal(t, "Set[str]", functions[0].ReturnType,
		"Should extract full Set generic type")
}

// TestPythonParser_ExtractFunctions_DictUnionValueReturnType tests Dict with Union value type.
// RED PHASE: Tests Dict[str, Union[str, int, bool]].
func TestPythonParser_ExtractFunctions_DictUnionValueReturnType(t *testing.T) {
	sourceCode := `
from typing import Dict, Union

def get_config() -> Dict[str, Union[str, int, bool]]:
    """Config with mixed value types."""
    return {}
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "Dict[str, Union[str, int, bool]]"
	// ACTUAL: String manipulation will definitely fail with nested Union inside Dict
	assert.Equal(t, "Dict[str, Union[str, int, bool]]", functions[0].ReturnType,
		"Should extract Dict with nested Union value type")
}

// TestPythonParser_ExtractFunctions_ComplexCallableReturnType tests Callable with multiple params.
// RED PHASE: Tests Callable[[str, int, bool], bool].
func TestPythonParser_ExtractFunctions_ComplexCallableReturnType(t *testing.T) {
	sourceCode := `
from typing import Callable

def get_validator() -> Callable[[str, int, bool], bool]:
    """Returns complex validator function."""
    return lambda s, i, b: True
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "Callable[[str, int, bool], bool]"
	// ACTUAL: String manipulation will fail with complex nested structure
	assert.Equal(t, "Callable[[str, int, bool], bool]", functions[0].ReturnType,
		"Should extract Callable with multiple parameter types")
}

// TestPythonParser_ExtractFunctions_ListOfCallablesReturnType tests List[Callable[[str], None]].
// RED PHASE: Tests List containing Callable types.
func TestPythonParser_ExtractFunctions_ListOfCallablesReturnType(t *testing.T) {
	sourceCode := `
from typing import List, Callable

def get_handlers() -> List[Callable[[str], None]]:
    """List of handler functions."""
    return []
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "List[Callable[[str], None]]"
	// ACTUAL: String manipulation will fail with triple-nested brackets
	assert.Equal(t, "List[Callable[[str], None]]", functions[0].ReturnType,
		"Should extract List of Callable types")
}

// TestPythonParser_ExtractFunctions_OptionalNestedGenericReturnType tests Optional[Dict[str, List[int]]].
// RED PHASE: Tests Optional with deeply nested generics.
func TestPythonParser_ExtractFunctions_OptionalNestedGenericReturnType(t *testing.T) {
	sourceCode := `
from typing import Optional, Dict, List

def get_optional_nested() -> Optional[Dict[str, List[int]]]:
    """Optional nested generic."""
    return None
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "Optional[Dict[str, List[int]]]"
	// ACTUAL: String manipulation will definitely fail with triple nesting
	assert.Equal(t, "Optional[Dict[str, List[int]]]", functions[0].ReturnType,
		"Should extract Optional with deeply nested generics")
}

// TestPythonParser_ExtractFunctions_UnionOfGenericsReturnType tests Union[List[str], Dict[str, int]].
// RED PHASE: Tests Union of different generic types.
func TestPythonParser_ExtractFunctions_UnionOfGenericsReturnType(t *testing.T) {
	sourceCode := `
from typing import Union, List, Dict

def get_multi_generic() -> Union[List[str], Dict[str, int]]:
    """Union of different generic types."""
    return []
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "Union[List[str], Dict[str, int]]"
	// ACTUAL: String manipulation will fail with Union containing multiple generics
	assert.Equal(t, "Union[List[str], Dict[str, int]]", functions[0].ReturnType,
		"Should extract Union of different generic types")
}

// TestPythonParser_ExtractFunctions_DictWithAnyValueReturnType tests Dict[str, Any].
// RED PHASE: Tests Dict with Any value type.
func TestPythonParser_ExtractFunctions_DictWithAnyValueReturnType(t *testing.T) {
	sourceCode := `
from typing import Dict, Any

def get_flexible() -> Dict[str, Any]:
    """Dict with any value type."""
    return {}
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "Dict[str, Any]"
	// ACTUAL: May work if tree-sitter provides it correctly
	assert.Equal(t, "Dict[str, Any]", functions[0].ReturnType,
		"Should extract Dict with Any value type")
}

// TestPythonParser_ExtractFunctions_NoneReturnType tests explicit None return type.
// RED PHASE: Tests explicit None return annotation.
func TestPythonParser_ExtractFunctions_NoneReturnType(t *testing.T) {
	sourceCode := `
def no_return() -> None:
    """Explicitly returns None."""
    pass
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "None"
	// ACTUAL: Should work, simple case
	assert.Equal(t, "None", functions[0].ReturnType,
		"Should extract explicit None return type")
}

// TestPythonParser_ExtractFunctions_MultipleNestedGenericsReturnType tests Dict[str, List[Tuple[int, str]]].
// RED PHASE: Tests Dict with List of Tuples (3 levels of nesting).
func TestPythonParser_ExtractFunctions_MultipleNestedGenericsReturnType(t *testing.T) {
	sourceCode := `
from typing import Dict, List, Tuple

def get_ultra_nested() -> Dict[str, List[Tuple[int, str]]]:
    """Multiple nested generic types."""
    return {}
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "Dict[str, List[Tuple[int, str]]]"
	// ACTUAL: String manipulation will fail with 3 levels of nesting
	assert.Equal(t, "Dict[str, List[Tuple[int, str]]]", functions[0].ReturnType,
		"Should extract Dict with List of Tuple (3 levels of nesting)")
}

// TestPythonParser_ExtractFunctions_ComplexPipeUnionReturnType tests str | int | None.
// RED PHASE: Tests pipe union with more than 2 alternatives.
func TestPythonParser_ExtractFunctions_ComplexPipeUnionReturnType(t *testing.T) {
	sourceCode := `
def complex_pipe_union() -> str | int | None:
    """Complex pipe union with None."""
    pass
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeTypeInfo: true,
	}

	functions, err := parser.ExtractFunctions(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	// EXPECTED: "str | int | None"
	// ACTUAL: String splitting on space will only get "str"
	assert.Equal(t, "str | int | None", functions[0].ReturnType,
		"Should extract full pipe union with multiple alternatives")
}

// =============================================================================
// RED PHASE TESTS: Module Variable Extraction with String Manipulation Bug
// =============================================================================
// These tests expose the bug in modules.go:134 where `strings.Trim(value, "\"'")`
// is used instead of tree-sitter navigation with `extractStringContent()`.
// This string manipulation approach CANNOT correctly handle:
// 1. Embedded quotes (both double and single)
// 2. Escaped quotes within strings
// 3. Mixed quote types in the same string
// 4. Unicode characters with quotes
// 5. Empty strings (may strip too much)
// =============================================================================

// TestPythonParser_ExtractModules_VariableWithEmbeddedDoubleQuotes tests module variable with embedded double quotes.
// RED PHASE: EXPECTED TO FAIL - Current implementation uses strings.Trim which incorrectly strips embedded quotes.
// BUG LOCATION: modules.go:134 - `value = strings.Trim(value, "\"'")`
// WHY IT FAILS: strings.Trim removes ALL occurrences of " and ' from BOTH ends of the string,
//
//	which means a value like `"He said "hello" to me"` gets incorrectly processed.
//
// FIX: Replace string manipulation with tree-sitter navigation using extractStringContent().
func TestPythonParser_ExtractModules_VariableWithEmbeddedDoubleQuotes(t *testing.T) {
	sourceCode := `#!/usr/bin/env python3
"""Module with embedded quotes in metadata."""

__version__ = "1.0.0"
__author__ = "He said \"hello\" to me"
__license__ = "MIT"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeMetadata: true,
	}

	modules, err := parser.ExtractModules(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, modules, 1)

	module := modules[0]
	require.NotNil(t, module.Metadata, "Module should have metadata")

	// EXPECTED: "He said \"hello\" to me" (embedded quotes preserved)
	// ACTUAL WITH BUG: "He said \\\"hello\\\" to me" or "He said " (strings.Trim corrupts it)
	// WHY: strings.Trim(value, "\"'") removes " and ' from both ends, not just delimiters
	assert.Equal(t, `He said "hello" to me`, module.Metadata["author"],
		"Module author metadata should preserve embedded double quotes")
}

// TestPythonParser_ExtractModules_VariableWithEmbeddedSingleQuote tests module variable with embedded single quote.
// RED PHASE: EXPECTED TO FAIL - Current implementation cannot handle apostrophes in single-quoted strings.
// BUG LOCATION: modules.go:134.
func TestPythonParser_ExtractModules_VariableWithEmbeddedSingleQuote(t *testing.T) {
	sourceCode := `#!/usr/bin/env python3
"""Module with apostrophes."""

__version__ = "1.0.0"
__author__ = 'It\'s working correctly'
__license__ = "MIT"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeMetadata: true,
	}

	modules, err := parser.ExtractModules(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, modules, 1)

	module := modules[0]
	require.NotNil(t, module.Metadata)

	// EXPECTED: "It's working correctly" (embedded apostrophe preserved)
	// ACTUAL WITH BUG: "It\\'s working correctly" or truncated (strings.Trim fails)
	assert.Equal(t, "It's working correctly", module.Metadata["author"],
		"Module author metadata should preserve embedded single quote")
}

// TestPythonParser_ExtractModules_VariableWithMixedQuotes tests module variable with both quote types.
// RED PHASE: EXPECTED TO FAIL - String manipulation cannot handle complex quote combinations.
// BUG LOCATION: modules.go:134.
func TestPythonParser_ExtractModules_VariableWithMixedQuotes(t *testing.T) {
	sourceCode := `#!/usr/bin/env python3
"""Module with mixed quotes."""

__version__ = "1.0.0"
__author__ = "She said 'hello' and he replied \"hi\""
__license__ = "MIT"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeMetadata: true,
	}

	modules, err := parser.ExtractModules(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, modules, 1)

	module := modules[0]
	require.NotNil(t, module.Metadata)

	// EXPECTED: "She said 'hello' and he replied \"hi\"" (both quote types preserved)
	// ACTUAL WITH BUG: Malformed string due to strings.Trim removing both ' and " from ends
	assert.Equal(t, `She said 'hello' and he replied "hi"`, module.Metadata["author"],
		"Module author metadata should preserve mixed quote types")
}

// TestPythonParser_ExtractModules_VariableWithEscapedQuotes tests JSON-like escaped quotes.
// RED PHASE: EXPECTED TO FAIL - String manipulation doesn't properly handle escape sequences.
// BUG LOCATION: modules.go:134.
func TestPythonParser_ExtractModules_VariableWithEscapedQuotes(t *testing.T) {
	sourceCode := `#!/usr/bin/env python3
"""Module with JSON-like content."""

__version__ = "1.0.0"
__author__ = "Parser for JSON: {\"key\": \"value\"}"
__license__ = "MIT"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeMetadata: true,
	}

	modules, err := parser.ExtractModules(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, modules, 1)

	module := modules[0]
	require.NotNil(t, module.Metadata)

	// EXPECTED: "Parser for JSON: {\"key\": \"value\"}" (escaped quotes preserved)
	// ACTUAL WITH BUG: Escaped quotes may be incorrectly processed by strings.Trim
	assert.Contains(t, module.Metadata["author"], `"key"`,
		"Module author metadata should preserve escaped quotes correctly")
	assert.Contains(t, module.Metadata["author"], `"value"`,
		"Module author metadata should preserve escaped quotes correctly")
}

// TestPythonParser_ExtractModules_VariableWithUnicode tests Unicode content with quotes.
// RED PHASE: EXPECTED TO FAIL - String manipulation may corrupt Unicode characters near quotes.
// BUG LOCATION: modules.go:134.
func TestPythonParser_ExtractModules_VariableWithUnicode(t *testing.T) {
	sourceCode := `#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Module with international content."""

__version__ = "1.0.0"
__author__ = "Developer says '你好' and 'مرحبا'"
__license__ = "MIT"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeMetadata: true,
	}

	modules, err := parser.ExtractModules(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, modules, 1)

	module := modules[0]
	require.NotNil(t, module.Metadata)

	// EXPECTED: "Developer says '你好' and 'مرحبا'" (Unicode + quotes preserved)
	// ACTUAL WITH BUG: strings.Trim may corrupt Unicode characters
	authorValue, ok := module.Metadata["author"].(string)
	require.True(t, ok, "Author should be a string")
	assert.Contains(t, authorValue, "你好",
		"Module author metadata should preserve Unicode characters")
	assert.Contains(t, authorValue, "مرحبا",
		"Module author metadata should preserve Arabic characters")
	assert.Contains(t, authorValue, "'你好'",
		"Module author metadata should preserve quotes around Unicode")
}

// TestPythonParser_ExtractModules_VariableEmptyString tests empty string handling.
// RED PHASE: EXPECTED TO FAIL - strings.Trim may remove too much.
// BUG LOCATION: modules.go:134.
func TestPythonParser_ExtractModules_VariableEmptyString(t *testing.T) {
	sourceCode := `#!/usr/bin/env python3
"""Module with empty metadata."""

__version__ = ""
__author__ = "Developer"
__license__ = ""
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeMetadata: true,
	}

	modules, err := parser.ExtractModules(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, modules, 1)

	module := modules[0]
	require.NotNil(t, module.Metadata)

	// EXPECTED: "" (empty string)
	// ACTUAL WITH BUG: strings.Trim should handle this correctly, but let's verify
	assert.Empty(t, module.Metadata["version"],
		"Module version metadata should handle empty string")
	assert.Empty(t, module.Metadata["license"],
		"Module license metadata should handle empty string")
}

// TestPythonParser_ExtractModules_VariableQuoteAtEnd tests string ending with embedded quote.
// RED PHASE: EXPECTED TO FAIL - strings.Trim will incorrectly strip embedded quote at end.
// BUG LOCATION: modules.go:134.
func TestPythonParser_ExtractModules_VariableQuoteAtEnd(t *testing.T) {
	sourceCode := `#!/usr/bin/env python3
"""Module with quote at end."""

__version__ = "1.0.0"
__author__ = "Developer's quote: \"end\""
__license__ = "MIT"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeMetadata: true,
	}

	modules, err := parser.ExtractModules(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, modules, 1)

	module := modules[0]
	require.NotNil(t, module.Metadata)

	// EXPECTED: "Developer's quote: \"end\"" (embedded quote at end preserved)
	// ACTUAL WITH BUG: strings.Trim(value, "\"'") will strip the ending " that's part of content
	assert.Equal(t, `Developer's quote: "end"`, module.Metadata["author"],
		"Module author metadata should preserve embedded quote at end")
}

// TestPythonParser_ExtractModules_VariableQuoteAtStart tests string starting with embedded quote.
// RED PHASE: EXPECTED TO FAIL - strings.Trim will incorrectly strip embedded quote at start.
// BUG LOCATION: modules.go:134.
func TestPythonParser_ExtractModules_VariableQuoteAtStart(t *testing.T) {
	sourceCode := `#!/usr/bin/env python3
"""Module with quote at start."""

__version__ = "1.0.0"
__author__ = "\"Start\" is the developer name"
__license__ = "MIT"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeMetadata: true,
	}

	modules, err := parser.ExtractModules(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, modules, 1)

	module := modules[0]
	require.NotNil(t, module.Metadata)

	// EXPECTED: "\"Start\" is the developer name" (embedded quote at start preserved)
	// ACTUAL WITH BUG: strings.Trim(value, "\"'") will strip the starting " that's part of content
	assert.Equal(t, `"Start" is the developer name`, module.Metadata["author"],
		"Module author metadata should preserve embedded quote at start")
}

// TestPythonParser_ExtractModules_VariableMultipleQuotesInRow tests consecutive quotes.
// RED PHASE: EXPECTED TO FAIL - strings.Trim removes multiple quote characters from ends.
// BUG LOCATION: modules.go:134.
func TestPythonParser_ExtractModules_VariableMultipleQuotesInRow(t *testing.T) {
	sourceCode := `#!/usr/bin/env python3
"""Module with consecutive quotes."""

__version__ = "1.0.0"
__author__ = "Developer \"\"quoted\"\" text"
__license__ = "MIT"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeMetadata: true,
	}

	modules, err := parser.ExtractModules(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, modules, 1)

	module := modules[0]
	require.NotNil(t, module.Metadata)

	// EXPECTED: "Developer \"\"quoted\"\" text" (consecutive quotes preserved)
	// ACTUAL WITH BUG: strings.Trim will malform this
	assert.Equal(t, `Developer ""quoted"" text`, module.Metadata["author"],
		"Module author metadata should preserve consecutive quotes")
}

// TestPythonParser_ExtractModules_VariableWithRawString tests r-prefix string.
// RED PHASE: EXPECTED TO FAIL - strings.Trim doesn't know about r-prefix.
// BUG LOCATION: modules.go:134.
func TestPythonParser_ExtractModules_VariableWithRawString(t *testing.T) {
	sourceCode := `#!/usr/bin/env python3
"""Module with raw string."""

__version__ = "1.0.0"
__author__ = r"C:\Users\Developer\path"
__license__ = "MIT"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludeMetadata: true,
	}

	modules, err := parser.ExtractModules(context.Background(), parseTree, options)
	require.NoError(t, err)
	require.Len(t, modules, 1)

	module := modules[0]
	require.NotNil(t, module.Metadata)

	// EXPECTED: "C:\\Users\\Developer\\path" (raw string content)
	// ACTUAL WITH BUG: strings.Trim doesn't handle r-prefix correctly
	assert.Equal(t, `C:\Users\Developer\path`, module.Metadata["author"],
		"Module author metadata should extract raw string content correctly")
}
