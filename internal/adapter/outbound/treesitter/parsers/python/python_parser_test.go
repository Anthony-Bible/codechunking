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

// Note: RED PHASE stub implementations removed - now implemented in python_parser.go
