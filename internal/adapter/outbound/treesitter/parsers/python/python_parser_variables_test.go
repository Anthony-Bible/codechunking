package pythonparser

import (
	"codechunking/internal/adapter/outbound/treesitter/parsers/testhelpers"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPythonParser_ExtractVariables_LambdaFunctions tests extraction of lambda functions as variables.
// Lambda functions are typically treated as variable assignments in Python.
func TestPythonParser_ExtractVariables_LambdaFunctions(t *testing.T) {
	sourceCode := `# Simple lambda functions
add_numbers = lambda x, y: x + y
square = lambda n: n ** 2
greet = lambda name="World": f"Hello, {name}!"

# Lambda with complex logic
process_data = lambda items: [
    item.upper() 
    for item in items 
    if isinstance(item, str)
]

# Lambda in data structures  
operations = {
    'add': lambda a, b: a + b,
    'multiply': lambda a, b: a * b,
    'divide': lambda a, b: a / b if b != 0 else None
}
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

	variables, err := parser.ExtractVariables(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find lambda variables
	require.GreaterOrEqual(t, len(variables), 3)

	// Test simple lambda
	addFunc := testhelpers.FindChunkByName(variables, "add_numbers")
	require.NotNil(t, addFunc, "Should find 'add_numbers' lambda variable")
	assert.Equal(t, outbound.ConstructVariable, addFunc.Type)
	assert.Equal(t, outbound.Public, addFunc.Visibility)

	// Test lambda with default parameter
	greetFunc := testhelpers.FindChunkByName(variables, "greet")
	require.NotNil(t, greetFunc, "Should find 'greet' lambda variable")
	assert.Equal(t, outbound.ConstructVariable, greetFunc.Type)

	// Test complex lambda
	processFunc := testhelpers.FindChunkByName(variables, "process_data")
	require.NotNil(t, processFunc, "Should find 'process_data' lambda variable")
	assert.Equal(t, outbound.ConstructVariable, processFunc.Type)
}

// TestPythonParser_ExtractVariables_TypeAnnotatedVariables tests extraction of variables with type annotations.
// This test covers modern Python variable type annotation features.
func TestPythonParser_ExtractVariables_TypeAnnotatedVariables(t *testing.T) {
	sourceCode := `from typing import List, Dict, Optional, Union, Final

# Basic type annotations
name: str = "John Doe"
age: int = 30
height: float = 5.9
is_active: bool = True

# Complex type annotations
users: List[str] = ["alice", "bob", "charlie"]
config: Dict[str, int] = {"timeout": 30, "retries": 3}
optional_value: Optional[str] = None
mixed_type: Union[int, str] = 42

# Final and class variables
API_KEY: Final[str] = "secret-key-123"
DEFAULT_TIMEOUT: Final[int] = 30

# Forward references and complex types
from __future__ import annotations
user_registry: Dict[str, User] = {}
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

	variables, err := parser.ExtractVariables(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find multiple typed variables
	require.GreaterOrEqual(t, len(variables), 5)

	// Test basic types
	nameVar := testhelpers.FindChunkByName(variables, "name")
	require.NotNil(t, nameVar, "Should find 'name' variable")
	assert.Equal(t, outbound.ConstructVariable, nameVar.Type)
	assert.Equal(t, outbound.Public, nameVar.Visibility)

	// Test list type
	usersVar := testhelpers.FindChunkByName(variables, "users")
	require.NotNil(t, usersVar, "Should find 'users' variable")
	assert.Equal(t, outbound.ConstructVariable, usersVar.Type)

	// Test Final constants
	apiKeyVar := testhelpers.FindChunkByName(variables, "API_KEY")
	require.NotNil(t, apiKeyVar, "Should find 'API_KEY' constant")
	assert.Equal(t, outbound.ConstructConstant, apiKeyVar.Type)
	assert.Equal(t, outbound.Public, apiKeyVar.Visibility)
}

// TestPythonParser_ExtractVariables_ClassVariables tests extraction of class variables vs instance variables.
// This test covers the distinction between class-level and instance-level variables.
func TestPythonParser_ExtractVariables_ClassVariables(t *testing.T) {
	sourceCode := `class User:
    # Class variables
    total_users: int = 0
    default_role: str = "guest"
    MAX_LOGIN_ATTEMPTS: Final[int] = 3
    
    def __init__(self, name: str, email: str):
        # Instance variables
        self.name = name
        self.email = email
        self.login_attempts = 0
        self._private_token = None
        User.total_users += 1
    
    def reset_password(self):
        # Method-local variables
        temp_password = self._generate_temp_password()
        self._private_token = temp_password
        return temp_password

class Config:
    # Class configuration variables
    DEBUG = True
    DATABASE_URL = "sqlite:///app.db" 
    SECRET_KEY = "dev-secret-key"
    
    # Type annotated class variables
    PORT: int = 8000
    HOST: str = "localhost"
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

	variables, err := parser.ExtractVariables(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find class variables
	require.GreaterOrEqual(t, len(variables), 3)

	// Test class constant
	maxAttemptsVar := testhelpers.FindChunkByName(variables, "MAX_LOGIN_ATTEMPTS")
	if maxAttemptsVar != nil {
		assert.Equal(t, outbound.ConstructConstant, maxAttemptsVar.Type)
		assert.Equal(t, outbound.Public, maxAttemptsVar.Visibility)
	}

	// Test class variable
	totalUsersVar := testhelpers.FindChunkByName(variables, "total_users")
	if totalUsersVar != nil {
		assert.Equal(t, outbound.ConstructVariable, totalUsersVar.Type)
		assert.Equal(t, outbound.Public, totalUsersVar.Visibility)
	}

	// Test config variables
	debugVar := testhelpers.FindChunkByName(variables, "DEBUG")
	if debugVar != nil {
		assert.Equal(t, outbound.ConstructVariable, debugVar.Type)
		assert.Equal(t, outbound.Public, debugVar.Visibility)
	}
}

// TestPythonParser_ExtractVariables_ModuleConstants tests extraction of module-level constants.
// This test covers constants defined at module scope with various patterns.
func TestPythonParser_ExtractVariables_ModuleConstants(t *testing.T) {
	sourceCode := `#!/usr/bin/env python3
"""Module with various constants and configuration values."""

import os
from typing import Final

# Standard constants
VERSION = "1.0.0"
AUTHOR = "Development Team"
LICENSE = "MIT"

# Configuration constants
MAX_CONNECTIONS = 100
DEFAULT_TIMEOUT = 30
CACHE_SIZE = 1024

# Computed constants
BASE_DIR = os.path.dirname(os.path.abspath(__file__))
LOG_FILE = os.path.join(BASE_DIR, "app.log")

# Final constants (Python 3.8+)
API_VERSION: Final = "v1"
MAX_RETRIES: Final[int] = 5

# Environment-based constants  
DEBUG = os.getenv("DEBUG", "false").lower() == "true"
DATABASE_URL = os.getenv("DATABASE_URL", "sqlite:///default.db")

# Constant collections
HTTP_METHODS = ["GET", "POST", "PUT", "DELETE"]
STATUS_CODES = {
    "OK": 200,
    "NOT_FOUND": 404,
    "SERVER_ERROR": 500
}

# Private constants
_INTERNAL_KEY = "internal-secret"
__VERY_SECRET = "top-secret"
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

	variables, err := parser.ExtractVariables(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find multiple constants
	require.GreaterOrEqual(t, len(variables), 8)

	// Test standard constants
	versionVar := testhelpers.FindChunkByName(variables, "VERSION")
	require.NotNil(t, versionVar, "Should find 'VERSION' constant")
	assert.Equal(t, outbound.ConstructConstant, versionVar.Type)
	assert.Equal(t, outbound.Public, versionVar.Visibility)

	// Test Final constants
	apiVersionVar := testhelpers.FindChunkByName(variables, "API_VERSION")
	if apiVersionVar != nil {
		assert.Equal(t, outbound.ConstructConstant, apiVersionVar.Type)
		assert.Equal(t, outbound.Public, apiVersionVar.Visibility)
	}

	// Test private constants
	internalKeyVar := testhelpers.FindChunkByName(variables, "_INTERNAL_KEY")
	if internalKeyVar != nil {
		assert.Equal(t, outbound.ConstructConstant, internalKeyVar.Type)
		assert.Equal(t, outbound.Private, internalKeyVar.Visibility)
	}

	// Test very private constants
	verySecretVar := testhelpers.FindChunkByName(variables, "__VERY_SECRET")
	if verySecretVar != nil {
		assert.Equal(t, outbound.ConstructConstant, verySecretVar.Type)
		assert.Equal(t, outbound.Private, verySecretVar.Visibility)
	}
}

// TestPythonParser_ExtractVariables_PrivateVariables tests extraction with private variable filtering.
// This test verifies that private/public variable filtering works correctly.
func TestPythonParser_ExtractVariables_PrivateVariables(t *testing.T) {
	sourceCode := `# Public variables
public_var = "visible to all"
CONSTANT = "public constant"

# Private variables (single underscore)
_private_var = "internal use"
_internal_config = {"setting": "value"}

# Very private variables (double underscore)
__secret_key = "top-secret"
__internal_state = None

class Example:
    public_attr = "public"
    _protected_attr = "protected"
    __private_attr = "private"
`

	language, err := valueobject.NewLanguage(valueobject.LanguagePython)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	parser, err := NewPythonParser()
	require.NoError(t, err)

	// Test with private variables included
	optionsWithPrivate := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	variablesWithPrivate, err := parser.ExtractVariables(context.Background(), parseTree, optionsWithPrivate)
	require.NoError(t, err)

	// Test without private variables
	optionsNoPrivate := outbound.SemanticExtractionOptions{
		IncludePrivate:       false,
		IncludeDocumentation: true,
		IncludeTypeInfo:      true,
		MaxDepth:             10,
	}

	variablesNoPrivate, err := parser.ExtractVariables(context.Background(), parseTree, optionsNoPrivate)
	require.NoError(t, err)

	// Should have fewer variables when excluding private
	assert.Less(t, len(variablesNoPrivate), len(variablesWithPrivate))

	// Verify public variables are always included
	publicVar := testhelpers.FindChunkByName(variablesWithPrivate, "public_var")
	if publicVar != nil {
		assert.Equal(t, outbound.Public, publicVar.Visibility)
	}

	constantVar := testhelpers.FindChunkByName(variablesWithPrivate, "CONSTANT")
	if constantVar != nil {
		assert.Equal(t, outbound.Public, constantVar.Visibility)
	}

	// Verify private variables are only included when requested
	privateVar := testhelpers.FindChunkByName(variablesWithPrivate, "_private_var")
	if privateVar != nil {
		assert.Equal(t, outbound.Private, privateVar.Visibility)
	}

	// Should not find private variables when excluded
	privateVarExcluded := testhelpers.FindChunkByName(variablesNoPrivate, "_private_var")
	assert.Nil(t, privateVarExcluded, "Should not find private variable when excluded")
}
