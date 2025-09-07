package javascriptparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJavaScriptImports_ES6Imports tests ES6 import statement parsing.
// This is a RED PHASE test that defines expected behavior for ES6 import extraction.
func TestJavaScriptImports_ES6Imports(t *testing.T) {
	sourceCode := `// es6_imports.js

// Default imports
import React from 'react';
import Vue from 'vue';
import _ from 'lodash';

// Named imports
import { useState, useEffect } from 'react';
import { mapState, mapActions } from 'vuex';
import { debounce, throttle, cloneDeep } from 'lodash';

// Mixed imports
import React, { Component, useState } from 'react';
import Vue, { createApp } from 'vue';

// Namespace imports
import * as React from 'react';
import * as _ from 'lodash';
import * as utils from './utils';

// Aliased imports
import { useState as useStateHook } from 'react';
import { debounce as delay } from 'lodash';
import { Component as BaseComponent } from 'react';

// Side-effect imports
import 'normalize.css';
import './styles.css';
import 'babel-polyfill';

// Dynamic imports
const LazyComponent = React.lazy(() => import('./LazyComponent'));
const dynamicModule = await import('./dynamic-module');
import('./conditional-module').then(module => {
    // Use module
});

// Relative imports
import { helper } from './helpers/utility';
import config from '../config/app-config';
import constants from '../../constants';

// Package imports
import express from 'express';
import { Router } from 'express';
import mongoose from 'mongoose';
import axios from 'axios';
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

	imports, err := adapter.ExtractImports(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find multiple import statements
	assert.GreaterOrEqual(t, len(imports), 15)

	// Test default import
	reactImport := findImportByPath(imports, "react")
	require.NotNil(t, reactImport, "Should find React import")
	assert.Equal(t, "react", reactImport.Path)
	assert.Empty(t, reactImport.Alias)
	assert.Empty(t, reactImport.ImportedSymbols) // Default import
	assert.False(t, reactImport.IsWildcard)

	// Test named imports
	reactHooksImport := findImportBySymbols(imports, []string{"useState", "useEffect"})
	require.NotNil(t, reactHooksImport, "Should find React hooks import")
	assert.Equal(t, "react", reactHooksImport.Path)
	assert.Contains(t, reactHooksImport.ImportedSymbols, "useState")
	assert.Contains(t, reactHooksImport.ImportedSymbols, "useEffect")
	assert.Len(t, reactHooksImport.ImportedSymbols, 2)

	// Test namespace import
	reactNamespaceImport := findImportByWildcard(imports, "react")
	assert.NotNil(t, reactNamespaceImport, "Should find React namespace import")
	if reactNamespaceImport != nil {
		assert.True(t, reactNamespaceImport.IsWildcard)
		assert.Equal(t, "React", reactNamespaceImport.Alias)
	}

	// Test aliased import
	aliasedImport := findImportWithAlias(imports, "useStateHook")
	assert.NotNil(t, aliasedImport, "Should find aliased import")
	if aliasedImport != nil {
		assert.Contains(t, aliasedImport.ImportedSymbols, "useState")
		assert.Contains(t, aliasedImport.Metadata, "aliases")
	}

	// Test side-effect import
	cssImport := findImportByPath(imports, "normalize.css")
	assert.NotNil(t, cssImport, "Should find CSS import")
	if cssImport != nil {
		assert.Empty(t, cssImport.ImportedSymbols)
		assert.Empty(t, cssImport.Alias)
		assert.Contains(t, cssImport.Metadata, "is_side_effect")
		assert.True(t, cssImport.Metadata["is_side_effect"].(bool))
	}

	// Test relative import
	relativeImport := findImportByPath(imports, "./helpers/utility")
	assert.NotNil(t, relativeImport, "Should find relative import")
	if relativeImport != nil {
		assert.Contains(t, relativeImport.Metadata, "is_relative")
		assert.True(t, relativeImport.Metadata["is_relative"].(bool))
		assert.Contains(t, relativeImport.Metadata, "relative_depth")
		assert.Equal(t, 1, relativeImport.Metadata["relative_depth"])
	}

	// Test parent directory import
	parentImport := findImportByPath(imports, "../../constants")
	assert.NotNil(t, parentImport, "Should find parent directory import")
	if parentImport != nil {
		assert.Contains(t, parentImport.Metadata, "relative_depth")
		assert.Equal(t, 2, parentImport.Metadata["relative_depth"])
	}
}

// TestJavaScriptImports_CommonJS tests CommonJS require statement parsing.
// This is a RED PHASE test that defines expected behavior for CommonJS import extraction.
func TestJavaScriptImports_CommonJS(t *testing.T) {
	sourceCode := `// commonjs_imports.js

// Basic require
const fs = require('fs');
const path = require('path');
const http = require('http');

// Destructured require
const { readFile, writeFile } = require('fs');
const { join, dirname } = require('path');
const { createServer } = require('http');

// Mixed destructuring
const express = require('express');
const { Router, static: staticMiddleware } = require('express');

// Relative requires
const utils = require('./utils');
const config = require('../config');
const constants = require('../../constants');

// Conditional requires
if (process.env.NODE_ENV === 'development') {
    const devTools = require('./dev-tools');
}

let dynamicModule;
if (someCondition) {
    dynamicModule = require('./dynamic-module');
}

// Require in function
function loadModule(moduleName) {
    return require(moduleName);
}

// Require with try-catch
let optionalModule;
try {
    optionalModule = require('optional-module');
} catch (error) {
    console.log('Optional module not available');
}

// Require assignments
const lib = require('some-library');
module.lib = lib;

// Complex require patterns
const {
    logger,
    database: { connect, disconnect },
    cache: cacheModule
} = require('./services');

// Package.json require
const packageJson = require('./package.json');
const { version, dependencies } = require('./package.json');
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

	imports, err := adapter.ExtractImports(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find multiple require statements
	assert.GreaterOrEqual(t, len(imports), 12)

	// Test basic require
	fsImport := findImportByPath(imports, "fs")
	require.NotNil(t, fsImport, "Should find fs require")
	assert.Equal(t, "fs", fsImport.Path)
	assert.Empty(t, fsImport.ImportedSymbols) // Default require
	assert.Contains(t, fsImport.Metadata, "import_type")
	assert.Equal(t, "commonjs", fsImport.Metadata["import_type"])

	// Test destructured require
	fsDestructuredImport := findImportBySymbols(imports, []string{"readFile", "writeFile"})
	require.NotNil(t, fsDestructuredImport, "Should find destructured fs require")
	assert.Equal(t, "fs", fsDestructuredImport.Path)
	assert.Contains(t, fsDestructuredImport.ImportedSymbols, "readFile")
	assert.Contains(t, fsDestructuredImport.ImportedSymbols, "writeFile")

	// Test aliased destructuring
	expressAliasedImport := findImportWithAlias(imports, "staticMiddleware")
	assert.NotNil(t, expressAliasedImport, "Should find aliased destructured import")
	if expressAliasedImport != nil {
		assert.Contains(t, expressAliasedImport.ImportedSymbols, "static")
		assert.Contains(t, expressAliasedImport.Metadata, "aliases")
	}

	// Test relative require
	utilsImport := findImportByPath(imports, "./utils")
	assert.NotNil(t, utilsImport, "Should find utils require")
	if utilsImport != nil {
		assert.Contains(t, utilsImport.Metadata, "is_relative")
		assert.True(t, utilsImport.Metadata["is_relative"].(bool))
	}

	// Test conditional require
	var conditionalImportFound bool
	for _, imp := range imports {
		if imp.Path == "./dev-tools" && imp.Metadata["is_conditional"] == true {
			conditionalImportFound = true
			break
		}
	}
	assert.True(t, conditionalImportFound, "Should find conditional require")

	// Test JSON require
	packageJsonImport := findImportByPath(imports, "./package.json")
	assert.NotNil(t, packageJsonImport, "Should find package.json require")
	if packageJsonImport != nil {
		assert.Contains(t, packageJsonImport.Metadata, "is_json")
		assert.True(t, packageJsonImport.Metadata["is_json"].(bool))
	}

	// Test nested destructuring
	servicesImport := findImportByPath(imports, "./services")
	assert.NotNil(t, servicesImport, "Should find services require")
	if servicesImport != nil {
		assert.Contains(t, servicesImport.ImportedSymbols, "logger")
		assert.Contains(t, servicesImport.Metadata, "has_nested_destructuring")
		assert.True(t, servicesImport.Metadata["has_nested_destructuring"].(bool))
	}
}

// TestJavaScriptImports_ModuleExports tests module.exports and exports parsing.
// This is a RED PHASE test that defines expected behavior for CommonJS export extraction.
func TestJavaScriptImports_ModuleExports(t *testing.T) {
	sourceCode := `// module_exports.js

// Basic module.exports
module.exports = function() {
    return 'basic export';
};

// Object export
module.exports = {
    add: function(a, b) { return a + b; },
    subtract: function(a, b) { return a - b; },
    PI: 3.14159
};

// Individual exports assignment
exports.multiply = function(a, b) {
    return a * b;
};

exports.divide = function(a, b) {
    return a / b;
};

exports.E = 2.71828;

// Mixed exports
module.exports.default = 'default value';
module.exports.helper = function() { return 'helper'; };

// Class export
class Calculator {
    add(a, b) { return a + b; }
}

module.exports = Calculator;
module.exports.Calculator = Calculator;

// Factory function export
function createLogger(level) {
    return {
        log: function(message) {
            console.log('[' + level + ']', message);
        }
    };
}

module.exports = createLogger;

// Conditional exports
if (typeof window !== 'undefined') {
    module.exports = browserVersion;
} else {
    module.exports = nodeVersion;
}

// Re-exports
module.exports = require('./other-module');
exports.utils = require('./utils');

// Named exports with ES6 syntax (for compatibility)
const config = { apiUrl: 'http://api.example.com' };
const helpers = {
    format: function(str) { return str.toUpperCase(); }
};

module.exports = {
    config,
    helpers,
    version: '1.0.0'
};
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

	// For exports, we might extract them as imports of the current module
	// or handle them separately in ExtractModules
	imports, err := adapter.ExtractImports(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find re-export requires
	reExportImport := findImportByPath(imports, "./other-module")
	assert.NotNil(t, reExportImport, "Should find re-export import")
	if reExportImport != nil {
		assert.Contains(t, reExportImport.Metadata, "is_re_export")
		assert.True(t, reExportImport.Metadata["is_re_export"].(bool))
	}

	utilsExportImport := findImportByPath(imports, "./utils")
	assert.NotNil(t, utilsExportImport, "Should find utils export import")
	if utilsExportImport != nil {
		assert.Contains(t, utilsExportImport.Metadata, "is_export_assignment")
		assert.True(t, utilsExportImport.Metadata["is_export_assignment"].(bool))
	}

	// Note: The actual exports would be handled by ExtractModules method
	// This test focuses on import-like patterns in exports
}

// TestJavaScriptImports_ImportMeta tests import.meta parsing.
// This is a RED PHASE test that defines expected behavior for import.meta extraction.
func TestJavaScriptImports_ImportMeta(t *testing.T) {
	sourceCode := `// import_meta.js

// Basic import.meta usage
console.log(import.meta.url);
console.log(import.meta.resolve);

// Import maps and module resolution
const moduleUrl = import.meta.resolve('./module.js');
const absoluteUrl = import.meta.resolve('/absolute/path');

// Dynamic import with import.meta
const currentDir = new URL('.', import.meta.url).pathname;
const siblingModule = await import(new URL('./sibling.js', import.meta.url));

// Environment detection
if (import.meta.env) {
    console.log('Vite environment variables:', import.meta.env);
}

// Hot module replacement (Vite/Webpack)
if (import.meta.hot) {
    import.meta.hot.accept('./dependency.js', (newModule) => {
        // Handle hot update
    });
}

// Build-time information
const buildTime = import.meta.env.BUILD_TIME;
const isDev = import.meta.env.DEV;
const isProd = import.meta.env.PROD;

// Custom import.meta properties (bundler-specific)
const gitCommit = import.meta.env.VITE_GIT_COMMIT;
const version = import.meta.env.PACKAGE_VERSION;
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

	imports, err := adapter.ExtractImports(ctx, parseTree, options)
	require.NoError(t, err)

	// Look for dynamic imports that use import.meta
	var importMetaFound bool
	for _, imp := range imports {
		if imp.Metadata["uses_import_meta"] == true {
			importMetaFound = true
			break
		}
	}

	// import.meta itself isn't an import, but dynamic imports using it should be detected
	siblingImport := findImportByPath(imports, "./sibling.js")
	if siblingImport != nil {
		assert.Contains(t, siblingImport.Metadata, "is_dynamic")
		assert.True(t, siblingImport.Metadata["is_dynamic"].(bool))
		assert.Contains(t, siblingImport.Metadata, "uses_import_meta")
		assert.True(t, siblingImport.Metadata["uses_import_meta"].(bool))
	}

	// The parser should recognize import.meta usage even if not creating import declarations
	assert.True(t, importMetaFound || siblingImport != nil, "Should detect import.meta usage")
}

// TestJavaScriptImports_ErrorHandling tests error conditions for import parsing.
// This is a RED PHASE test that defines expected behavior for import parsing error handling.
func TestJavaScriptImports_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	t.Run("nil parse tree should return error", func(t *testing.T) {
		options := outbound.SemanticExtractionOptions{}

		_, err := adapter.ExtractImports(ctx, nil, options)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse tree cannot be nil")
	})

	t.Run("unsupported language should return error", func(t *testing.T) {
		goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, goLang, "package main")
		options := outbound.SemanticExtractionOptions{}

		_, err = adapter.ExtractImports(ctx, parseTree, options)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported language")
	})

	t.Run("malformed import should not panic", func(t *testing.T) {
		malformedCode := `// Malformed imports
import from 'missing-specifier';
import { } from;
require();
`
		language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, malformedCode)
		options := outbound.SemanticExtractionOptions{}

		// Should not panic, even with malformed code
		imports, err := adapter.ExtractImports(ctx, parseTree, options)
		// May return error or empty results, but should not panic
		if err == nil {
			assert.NotNil(t, imports)
		}
	})
}

// Helper functions for import testing

// findImportByPath finds an import by its path.
func findImportByPath(imports []outbound.ImportDeclaration, path string) *outbound.ImportDeclaration {
	for i, imp := range imports {
		if imp.Path == path {
			return &imports[i]
		}
	}
	return nil
}

// findImportBySymbols finds an import that contains all specified symbols.
func findImportBySymbols(imports []outbound.ImportDeclaration, symbols []string) *outbound.ImportDeclaration {
	for i, imp := range imports {
		hasAllSymbols := true
		for _, symbol := range symbols {
			found := false
			for _, importedSymbol := range imp.ImportedSymbols {
				if importedSymbol == symbol {
					found = true
					break
				}
			}
			if !found {
				hasAllSymbols = false
				break
			}
		}
		if hasAllSymbols && len(symbols) <= len(imp.ImportedSymbols) {
			return &imports[i]
		}
	}
	return nil
}

// findImportByWildcard finds a wildcard import by path.
func findImportByWildcard(imports []outbound.ImportDeclaration, path string) *outbound.ImportDeclaration {
	for i, imp := range imports {
		if imp.Path == path && imp.IsWildcard {
			return &imports[i]
		}
	}
	return nil
}

// findImportWithAlias finds an import with a specific alias.
func findImportWithAlias(imports []outbound.ImportDeclaration, alias string) *outbound.ImportDeclaration {
	for i, imp := range imports {
		if imp.Alias == alias {
			return &imports[i]
		}
		// Check metadata for aliased imports
		if aliases, ok := imp.Metadata["aliases"]; ok {
			if aliasMap, ok := aliases.(map[string]string); ok {
				for _, aliasValue := range aliasMap {
					if aliasValue == alias {
						return &imports[i]
					}
				}
			}
		}
	}
	return nil
}
