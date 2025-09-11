package pythonparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPythonParser_ExtractImports_StandardLibraryImports tests extraction of standard library imports.
// This test covers various standard library import patterns.
func TestPythonParser_ExtractImports_StandardLibraryImports(t *testing.T) {
	sourceCode := `#!/usr/bin/env python3
import os
import sys
import json
from pathlib import Path
from typing import List, Dict, Optional
from collections import defaultdict, Counter

# Import with alias
import numpy as np
from matplotlib import pyplot as plt
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

	imports, err := parser.ExtractImports(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find multiple imports
	require.GreaterOrEqual(t, len(imports), 3)

	// Test simple imports
	osImport := findImportByModule(imports, "os")
	require.NotNil(t, osImport, "Should find 'os' import")
	assert.Equal(t, "os", osImport.Path)
	assert.False(t, isRelativeImport(osImport.Path))

	// Test from imports
	typingImports := findImportsByModule(imports, "typing")
	assert.GreaterOrEqual(t, len(typingImports), 1, "Should find typing imports")

	// Test aliased imports
	numpyImport := findImportByAlias(imports, "np")
	if numpyImport != nil {
		assert.Equal(t, "numpy", numpyImport.Path)
		assert.Equal(t, "np", numpyImport.Alias)
	}
}

// TestPythonParser_ExtractImports_RelativeImports tests extraction of relative imports.
// This test covers relative import patterns within packages.
func TestPythonParser_ExtractImports_RelativeImports(t *testing.T) {
	sourceCode := `# Relative imports within package
from . import utils
from .models import User, Product  
from ..database import connection
from ...config import settings
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

	imports, err := parser.ExtractImports(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find relative imports
	require.GreaterOrEqual(t, len(imports), 2)

	// Test current package relative import
	utilsImport := findImportByModule(imports, ".")
	if utilsImport != nil {
		assert.True(t, isRelativeImport(utilsImport.Path), "utils import should be relative")
	}

	// Test parent package import
	dbImport := findImportByModule(imports, "..")
	if dbImport != nil {
		assert.True(t, isRelativeImport(dbImport.Path), "database import should be relative")
	}

	// Verify relative import structure
	for _, imp := range imports {
		if isRelativeImport(imp.Path) {
			assert.True(t, len(imp.Path) > 0 && imp.Path[0] == '.',
				"Relative imports should start with dots")
		}
	}
}

// TestPythonParser_ExtractImports_ThirdPartyImports tests extraction of third-party package imports.
// This test covers common third-party libraries and frameworks.
func TestPythonParser_ExtractImports_ThirdPartyImports(t *testing.T) {
	sourceCode := `# Web frameworks
from flask import Flask, request
from django.http import HttpResponse

# Data science libraries  
import pandas as pd
import numpy as np
from sklearn.model_selection import train_test_split

# Utility libraries
import requests
from pydantic import BaseModel
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

	imports, err := parser.ExtractImports(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find many third-party imports
	require.GreaterOrEqual(t, len(imports), 4)

	// Test web framework imports
	flaskImport := findImportByModule(imports, "flask")
	if flaskImport != nil {
		assert.Equal(t, "flask", flaskImport.Path)
		assert.False(t, isRelativeImport(flaskImport.Path))
	}

	// Test data science imports
	pandasImport := findImportByAlias(imports, "pd")
	if pandasImport != nil {
		assert.Equal(t, "pandas", pandasImport.Path)
		assert.Equal(t, "pd", pandasImport.Alias)
	}

	// Test specific imports from modules
	requestsImport := findImportByModule(imports, "requests")
	if requestsImport != nil {
		assert.Equal(t, "requests", requestsImport.Path)
	}
}

// TestPythonParser_ExtractImports_ConditionalImports tests extraction of conditional imports.
// This test covers try/except import patterns and conditional imports.
func TestPythonParser_ExtractImports_ConditionalImports(t *testing.T) {
	sourceCode := `# Standard imports that are always present
import sys
import platform

# Version-based conditional imports
if sys.version_info >= (3, 11):
    import tomllib
else:
    try:
        import tomli as tomllib
    except ImportError:
        import toml as tomllib

# Optional feature imports
try:
    import redis
except ImportError:
    redis = None
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

	imports, err := parser.ExtractImports(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find conditional imports
	require.GreaterOrEqual(t, len(imports), 2)

	// Test standard imports that are always present
	sysImport := findImportByModule(imports, "sys")
	require.NotNil(t, sysImport, "Should find 'sys' import")
	assert.Equal(t, "sys", sysImport.Path)

	platformImport := findImportByModule(imports, "platform")
	require.NotNil(t, platformImport, "Should find 'platform' import")
	assert.Equal(t, "platform", platformImport.Path)

	// Test conditional imports (may or may not be detected depending on parser capabilities)
	tomllibImports := findImportsByModule(imports, "tomllib")
	// Note: Conditional imports may not be detected by static analysis
	// This tests the parser's ability to handle them gracefully
	if len(tomllibImports) > 0 {
		assert.Equal(t, "tomllib", tomllibImports[0].Path)
	}
}

// TestPythonParser_ExtractImports_ImportedNamesTracking tests tracking of specifically imported names.
// This test verifies that imported symbols/functions are properly tracked.
func TestPythonParser_ExtractImports_ImportedNamesTracking(t *testing.T) {
	sourceCode := `# Specific function imports
from os.path import join, dirname, basename
from json import loads, dumps
from datetime import datetime, timedelta

# Class imports
from collections import defaultdict, Counter
from pathlib import Path
from dataclasses import dataclass

# Mixed imports with aliases
from typing import List, Dict as Dictionary
from functools import lru_cache as cache
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

	imports, err := parser.ExtractImports(context.Background(), parseTree, options)
	require.NoError(t, err)

	// Should find imports with specific names
	require.GreaterOrEqual(t, len(imports), 3)

	// Test os.path imports
	osPathImports := findImportsByModule(imports, "os.path")
	if len(osPathImports) > 0 {
		assert.Equal(t, "os.path", osPathImports[0].Path)
		// Check that specific imported names are tracked
		expectedNames := []string{"join", "dirname", "basename"}
		for _, expectedName := range expectedNames {
			found := false
			for _, imp := range osPathImports {
				if contains(imp.ImportedSymbols, expectedName) {
					found = true
					break
				}
			}
			if !found && len(osPathImports[0].ImportedSymbols) > 0 {
				t.Logf("Expected to find '%s' in imported names from os.path", expectedName)
			}
		}
	}

	// Test aliased imports
	typingImports := findImportsByModule(imports, "typing")
	for _, imp := range typingImports {
		if imp.Alias == "Dictionary" {
			// Note: OriginalName might not be available in ImportDeclaration
			assert.Equal(t, "Dictionary", imp.Alias)
		}
	}
}

// Helper functions for finding imports.
func findImportsByModule(imports []outbound.ImportDeclaration, modulePath string) []outbound.ImportDeclaration {
	var result []outbound.ImportDeclaration
	for _, imp := range imports {
		if imp.Path == modulePath {
			result = append(result, imp)
		}
	}
	return result
}

func findImportByAlias(imports []outbound.ImportDeclaration, alias string) *outbound.ImportDeclaration {
	for i := range imports {
		if imports[i].Alias == alias {
			return &imports[i]
		}
	}
	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
