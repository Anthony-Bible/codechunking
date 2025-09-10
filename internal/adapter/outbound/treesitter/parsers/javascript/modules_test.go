package javascriptparser

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJavaScriptParser_ExtractModules_ES6NamedExports tests ES6 named exports extraction.
// This is a RED PHASE test that defines expected behavior for ES6 named exports.
func TestJavaScriptParser_ExtractModules_ES6NamedExports(t *testing.T) {
	sourceCode := `// math_utils.js - ES6 Named Exports Module

/**
 * Math utilities module
 * Provides common mathematical operations
 */

const PI = 3.14159;
const E = 2.71828;

function add(a, b) {
    return a + b;
}

function multiply(x, y) {
    return x * y;
}

class Calculator {
    static sum(...numbers) {
        return numbers.reduce(add, 0);
    }
}

// Named exports
export { PI, E, add, multiply, Calculator };

// Additional inline export
export const VERSION = "1.0.0";

// Export with alias
export { Calculator as MathCalculator };
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	ctx := context.Background()

	parser, err := NewJavaScriptParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeMetadata:      true,
		MaxDepth:             10,
	}

	modules, err := parser.ExtractModules(ctx, parseTree, options)
	require.NoError(t, err)

	// Should find 1 module representing the file's module structure
	require.Len(t, modules, 1)

	module := modules[0]
	assert.Equal(t, outbound.ConstructModule, module.Type)
	assert.Equal(t, "math_utils", module.Name)
	assert.Equal(t, "math_utils", module.QualifiedName)
	assert.Equal(t, language, module.Language)
	assert.Equal(t, outbound.Public, module.Visibility)

	// Should contain module documentation
	assert.Contains(t, module.Documentation, "Math utilities module")
	assert.Contains(t, module.Documentation, "Provides common mathematical operations")

	// Should have metadata about exports
	require.NotNil(t, module.Metadata)

	// Check export information in metadata
	exports, hasExports := module.Metadata["exports"]
	require.True(t, hasExports, "Module should have exports metadata")

	exportList, ok := exports.([]map[string]interface{})
	require.True(t, ok, "Exports should be a list of export objects")

	// Should have 7 exports: PI, E, add, multiply, Calculator, VERSION, MathCalculator (alias)
	assert.Len(t, exportList, 7)

	// Verify specific exports
	exportNames := extractExportNames(exportList)
	expectedExports := []string{"PI", "E", "add", "multiply", "Calculator", "VERSION", "MathCalculator"}
	for _, expectedExport := range expectedExports {
		assert.Contains(t, exportNames, expectedExport, "Should export %s", expectedExport)
	}

	// Check for alias export
	calculatorAlias := findExportByName(exportList, "MathCalculator")
	require.NotNil(t, calculatorAlias, "Should find MathCalculator alias export")
	assert.Equal(t, "Calculator", calculatorAlias["original_name"], "Should track original name for alias")
	assert.True(t, calculatorAlias["is_alias"].(bool), "Should mark as alias")

	// Check module type classification
	moduleType, hasType := module.Metadata["module_type"]
	require.True(t, hasType)
	assert.Equal(t, "es6", moduleType, "Should identify as ES6 module")

	// Check export statistics
	stats, hasStats := module.Metadata["export_stats"]
	require.True(t, hasStats)
	statsMap := stats.(map[string]int)
	assert.Equal(t, 5, statsMap["named_exports"], "Should count named exports")
	assert.Equal(t, 1, statsMap["inline_exports"], "Should count inline exports")
	assert.Equal(t, 1, statsMap["alias_exports"], "Should count alias exports")
}

// TestJavaScriptParser_ExtractModules_ES6DefaultExports tests ES6 default exports extraction.
// This is a RED PHASE test that defines expected behavior for ES6 default exports.
func TestJavaScriptParser_ExtractModules_ES6DefaultExports(t *testing.T) {
	sourceCode := `// user_service.js - ES6 Default Export Module

/**
 * User service for managing user operations
 */

import { Database } from './database.js';

class UserService {
    constructor(db) {
        this.database = db;
    }

    async findUser(id) {
        return await this.database.query('SELECT * FROM users WHERE id = ?', [id]);
    }

    async createUser(userData) {
        return await this.database.insert('users', userData);
    }
}

// Default export
export default UserService;

// Named export alongside default
export const USER_ROLES = {
    ADMIN: 'admin',
    USER: 'user',
    GUEST: 'guest'
};

// Another named export
export function validateUser(user) {
    return user && user.id && user.email;
}
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	ctx := context.Background()

	parser, err := NewJavaScriptParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeMetadata:      true,
		MaxDepth:             10,
	}

	modules, err := parser.ExtractModules(ctx, parseTree, options)
	require.NoError(t, err)

	require.Len(t, modules, 1)
	module := modules[0]

	assert.Equal(t, outbound.ConstructModule, module.Type)
	assert.Equal(t, "user_service", module.Name)
	assert.Contains(t, module.Documentation, "User service for managing user operations")

	// Check exports metadata
	exports := module.Metadata["exports"].([]map[string]interface{})

	// Should have 3 exports: UserService (default), USER_ROLES, validateUser
	assert.Len(t, exports, 3)

	// Find default export
	defaultExport := findExportByName(exports, "UserService")
	require.NotNil(t, defaultExport, "Should find default export")
	assert.True(t, defaultExport["is_default"].(bool), "Should mark as default export")
	assert.Equal(t, "class", defaultExport["export_type"], "Should identify export type")

	// Check export statistics
	stats := module.Metadata["export_stats"].(map[string]int)
	assert.Equal(t, 1, stats["default_exports"], "Should count default exports")
	assert.Equal(t, 2, stats["named_exports"], "Should count named exports")

	// Check import dependencies
	imports, hasImports := module.Metadata["imports"]
	require.True(t, hasImports, "Should track import dependencies")
	importList := imports.([]map[string]interface{})
	require.Len(t, importList, 1)

	dbImport := importList[0]
	assert.Equal(t, "./database.js", dbImport["path"])
	assert.Equal(t, "Database", dbImport["imported_symbols"].([]string)[0])
}

// TestJavaScriptParser_ExtractModules_ReExports tests re-export patterns.
// This is a RED PHASE test that defines expected behavior for re-exports.
func TestJavaScriptParser_ExtractModules_ReExports(t *testing.T) {
	sourceCode := `// index.js - Re-export Module

/**
 * Main entry point module
 * Re-exports functionality from other modules
 */

// Re-export all from utils
export * from './utils.js';

// Re-export specific items
export { Calculator, MathUtils } from './math.js';

// Re-export with alias
export { DatabaseService as DB } from './database.js';

// Re-export default as named
export { default as UserController } from './user-controller.js';

// Local exports mixed with re-exports
export const CONFIG = {
    version: '1.0.0',
    env: 'production'
};

export default {
    Calculator,
    DB,
    UserController,
    CONFIG
};
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	ctx := context.Background()

	parser, err := NewJavaScriptParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeMetadata:      true,
		MaxDepth:             10,
	}

	modules, err := parser.ExtractModules(ctx, parseTree, options)
	require.NoError(t, err)

	require.Len(t, modules, 1)
	module := modules[0]

	assert.Equal(t, "index", module.Name)
	assert.Contains(t, module.Documentation, "Main entry point module")

	// Check re-exports metadata
	reExports, hasReExports := module.Metadata["re_exports"]
	require.True(t, hasReExports, "Should track re-exports")

	reExportList := reExports.([]map[string]interface{})
	assert.GreaterOrEqual(t, len(reExportList), 4, "Should have multiple re-exports")

	// Verify wildcard re-export
	wildcardReExport := findReExportByPath(reExportList, "./utils.js")
	require.NotNil(t, wildcardReExport, "Should find wildcard re-export")
	assert.True(t, wildcardReExport["is_wildcard"].(bool), "Should mark as wildcard")

	// Verify specific re-exports
	mathReExport := findReExportByPath(reExportList, "./math.js")
	require.NotNil(t, mathReExport, "Should find math re-export")
	symbols := mathReExport["symbols"].([]string)
	assert.Contains(t, symbols, "Calculator")
	assert.Contains(t, symbols, "MathUtils")

	// Verify alias re-export
	dbReExport := findReExportByPath(reExportList, "./database.js")
	require.NotNil(t, dbReExport, "Should find database re-export")
	assert.Equal(t, "DatabaseService", dbReExport["original_name"])
	assert.Equal(t, "DB", dbReExport["alias_name"])

	// Check module role classification
	moduleRole, hasRole := module.Metadata["module_role"]
	require.True(t, hasRole)
	assert.Equal(t, "barrel", moduleRole, "Should identify as barrel module")

	// Check export statistics
	stats := module.Metadata["export_stats"].(map[string]int)
	assert.Positive(t, stats["re_exports"], "Should count re-exports")
	assert.Equal(t, 1, stats["wildcard_exports"], "Should count wildcard re-exports")
}

// TestJavaScriptParser_ExtractModules_CommonJSPatterns tests CommonJS module patterns.
// This is a RED PHASE test that defines expected behavior for CommonJS modules.
func TestJavaScriptParser_ExtractModules_CommonJSPatterns(t *testing.T) {
	sourceCode := `// logger.js - CommonJS Module

/**
 * Simple logger module using CommonJS
 */

const fs = require('fs');
const path = require('path');

const LOG_LEVELS = {
    DEBUG: 0,
    INFO: 1,
    WARN: 2,
    ERROR: 3
};

function createLogger(level = LOG_LEVELS.INFO) {
    return {
        debug: (msg) => level <= LOG_LEVELS.DEBUG && console.log('[DEBUG]', msg),
        info: (msg) => level <= LOG_LEVELS.INFO && console.log('[INFO]', msg),
        warn: (msg) => level <= LOG_LEVELS.WARN && console.warn('[WARN]', msg),
        error: (msg) => level <= LOG_LEVELS.ERROR && console.error('[ERROR]', msg)
    };
}

function writeLogFile(logData) {
    const logPath = path.join(__dirname, 'logs', 'app.log');
    fs.appendFileSync(logPath, logData + '\n');
}

// CommonJS exports
module.exports = {
    LOG_LEVELS,
    createLogger,
    writeLogFile
};

// Alternative CommonJS export style
// module.exports.defaultLogger = createLogger();
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	ctx := context.Background()

	parser, err := NewJavaScriptParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeMetadata:      true,
		MaxDepth:             10,
	}

	modules, err := parser.ExtractModules(ctx, parseTree, options)
	require.NoError(t, err)

	require.Len(t, modules, 1)
	module := modules[0]

	assert.Equal(t, "logger", module.Name)
	assert.Contains(t, module.Documentation, "Simple logger module using CommonJS")

	// Check module type classification
	moduleType := module.Metadata["module_type"]
	assert.Equal(t, "commonjs", moduleType, "Should identify as CommonJS module")

	// Check CommonJS exports
	exports := module.Metadata["exports"].([]map[string]interface{})
	exportNames := extractExportNames(exports)

	expectedExports := []string{"LOG_LEVELS", "createLogger", "writeLogFile"}
	for _, expectedExport := range expectedExports {
		assert.Contains(t, exportNames, expectedExport, "Should export %s", expectedExport)
	}

	// Check CommonJS requires
	requires, hasRequires := module.Metadata["requires"]
	require.True(t, hasRequires, "Should track require statements")

	requireList := requires.([]map[string]interface{})
	require.Len(t, requireList, 2)

	// Verify specific requires
	requirePaths := make([]string, len(requireList))
	for i, req := range requireList {
		requirePaths[i] = req["path"].(string)
	}
	assert.Contains(t, requirePaths, "fs")
	assert.Contains(t, requirePaths, "path")

	// Check export format
	exportFormat := module.Metadata["export_format"]
	assert.Equal(t, "object", exportFormat, "Should identify object export format")
}

// TestJavaScriptParser_ExtractModules_MixedExportPatterns tests complex mixed export scenarios.
// This is a RED PHASE test that defines expected behavior for mixed export patterns.
func TestJavaScriptParser_ExtractModules_MixedExportPatterns(t *testing.T) {
	sourceCode := `// api.js - Mixed Export Patterns Module

/**
 * API module with mixed export patterns
 * Demonstrates ES6 and CommonJS compatibility
 */

import express from 'express';
import { validateRequest } from './validators.js';

// Class for default export
class APIServer {
    constructor(port = 3000) {
        this.app = express();
        this.port = port;
    }

    start() {
        this.app.listen(this.port, () => {
            console.log('Server running on port', this.port);
        });
    }
}

// Named function exports
export function createMiddleware(options) {
    return (req, res, next) => {
        // middleware logic
        next();
    };
}

export async function handleRequest(req, res) {
    try {
        const validated = await validateRequest(req.body);
        res.json({ success: true, data: validated });
    } catch (error) {
        res.status(400).json({ error: error.message });
    }
}

// Constant exports
export const API_VERSION = 'v1';
export const DEFAULT_PORT = 3000;

// Object export
export const HTTP_STATUS = {
    OK: 200,
    BAD_REQUEST: 400,
    UNAUTHORIZED: 401,
    NOT_FOUND: 404,
    INTERNAL_ERROR: 500
};

// Default export
export default APIServer;

// CommonJS compatibility (conditional)
if (typeof module !== 'undefined' && module.exports) {
    module.exports = APIServer;
    module.exports.createMiddleware = createMiddleware;
    module.exports.handleRequest = handleRequest;
    module.exports.API_VERSION = API_VERSION;
    module.exports.HTTP_STATUS = HTTP_STATUS;
}
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	ctx := context.Background()

	parser, err := NewJavaScriptParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       true,
		IncludeDocumentation: true,
		IncludeMetadata:      true,
		MaxDepth:             10,
	}

	modules, err := parser.ExtractModules(ctx, parseTree, options)
	require.NoError(t, err)

	require.Len(t, modules, 1)
	module := modules[0]

	assert.Equal(t, "api", module.Name)
	assert.Contains(t, module.Documentation, "API module with mixed export patterns")

	// Check module type (should detect mixed)
	moduleType := module.Metadata["module_type"]
	assert.Equal(t, "mixed", moduleType, "Should identify as mixed ES6/CommonJS module")

	// Check comprehensive export list
	exports := module.Metadata["exports"].([]map[string]interface{})
	exportNames := extractExportNames(exports)

	expectedExports := []string{
		"APIServer", "createMiddleware", "handleRequest",
		"API_VERSION", "DEFAULT_PORT", "HTTP_STATUS",
	}

	for _, expectedExport := range expectedExports {
		assert.Contains(t, exportNames, expectedExport, "Should export %s", expectedExport)
	}

	// Verify default export
	defaultExport := findExportByDefault(exports)
	require.NotNil(t, defaultExport, "Should have default export")
	assert.Equal(t, "APIServer", defaultExport["name"])

	// Check export statistics
	stats := module.Metadata["export_stats"].(map[string]int)
	assert.Equal(t, 1, stats["default_exports"], "Should count default exports")
	assert.Equal(t, 5, stats["named_exports"], "Should count named exports")
	assert.Greater(t, stats["total_exports"], 5, "Should count total exports")

	// Check compatibility flags
	compatibility := module.Metadata["compatibility"].(map[string]bool)
	assert.True(t, compatibility["es6"], "Should support ES6")
	assert.True(t, compatibility["commonjs"], "Should support CommonJS")

	// Check imports
	imports := module.Metadata["imports"].([]map[string]interface{})
	assert.Len(t, imports, 2)

	// Verify specific imports
	expressImport := findModuleImportByPath(imports, "express")
	require.NotNil(t, expressImport, "Should find express import")
	assert.True(t, expressImport["is_default"].(bool), "Should be default import")
}

// TestJavaScriptParser_ExtractModules_ModuleStructureAnalysis tests module structure analysis.
// This is a RED PHASE test that defines expected behavior for module structure analysis.
func TestJavaScriptParser_ExtractModules_ModuleStructureAnalysis(t *testing.T) {
	sourceCode := `// complex_module.js - Complex Module Structure

/**
 * Complex module demonstrating various structures
 * @module ComplexModule
 * @version 1.2.0
 * @author John Doe <john@example.com>
 */

'use strict';

// Module-level constants
const MODULE_VERSION = '1.2.0';
const DEBUG = process.env.NODE_ENV === 'development';

// Private utilities (not exported)
function _privateHelper(data) {
    return data.toString().toUpperCase();
}

// Public classes
export class DataProcessor {
    static process(input) {
        return _privateHelper(input);
    }
}

export class ValidationError extends Error {
    constructor(message, field) {
        super(message);
        this.field = field;
        this.name = 'ValidationError';
    }
}

// Public functions
export function validateInput(data) {
    if (!data) {
        throw new ValidationError('Data is required', 'data');
    }
    return true;
}

export async function processData(data) {
    await validateInput(data);
    return DataProcessor.process(data);
}

// Public constants
export const VALIDATION_RULES = {
    REQUIRED: 'required',
    MIN_LENGTH: 'min_length',
    MAX_LENGTH: 'max_length'
};

// Default export with module summary
export default {
    DataProcessor,
    ValidationError,
    validateInput,
    processData,
    VALIDATION_RULES,
    version: MODULE_VERSION
};
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)
	ctx := context.Background()

	parser, err := NewJavaScriptParser()
	require.NoError(t, err)

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:       false, // Test filtering
		IncludeDocumentation: true,
		IncludeMetadata:      true,
		MaxDepth:             10,
	}

	modules, err := parser.ExtractModules(ctx, parseTree, options)
	require.NoError(t, err)

	require.Len(t, modules, 1)
	module := modules[0]

	assert.Equal(t, "complex_module", module.Name)

	// Check JSDoc parsing
	assert.Contains(t, module.Documentation, "Complex module demonstrating various structures")

	// Check module metadata from JSDoc
	assert.Equal(t, "ComplexModule", module.Metadata["module_name"])
	assert.Equal(t, "1.2.0", module.Metadata["version"])
	assert.Equal(t, "John Doe <john@example.com>", module.Metadata["author"])

	// Check structure analysis
	structure := module.Metadata["structure"].(map[string]interface{})

	// Should analyze different construct types
	constructCounts := structure["construct_counts"].(map[string]int)
	assert.Equal(t, 2, constructCounts["classes"], "Should count classes")
	assert.Equal(t, 2, constructCounts["functions"], "Should count public functions")
	assert.Equal(t, 1, constructCounts["constants"], "Should count public constants")

	// Should exclude private functions when IncludePrivate=false
	assert.Equal(t, 0, constructCounts["private_functions"], "Should exclude private functions")

	// Check export complexity analysis
	complexity := module.Metadata["complexity"].(map[string]interface{})
	assert.Greater(t, complexity["total_lines"].(int), 50, "Should count total lines")
	assert.Greater(t, complexity["export_ratio"].(float64), 0.5, "Should calculate export ratio")

	// Check module capabilities
	capabilities := module.Metadata["capabilities"].([]string)
	expectedCapabilities := []string{"data_processing", "validation", "error_handling"}
	for _, capability := range expectedCapabilities {
		assert.Contains(t, capabilities, capability, "Should detect %s capability", capability)
	}

	// Check strict mode detection
	features := module.Metadata["features"].(map[string]bool)
	assert.True(t, features["strict_mode"], "Should detect strict mode")
	assert.True(t, features["async_functions"], "Should detect async functions")
	assert.True(t, features["classes"], "Should detect classes")
	assert.True(t, features["inheritance"], "Should detect inheritance (extends)")
}

// TestJavaScriptParser_ExtractModules_ErrorHandling tests error scenarios.
// This is a RED PHASE test that defines expected behavior for error handling.
func TestJavaScriptParser_ExtractModules_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	parser, err := NewJavaScriptParser()
	require.NoError(t, err)

	t.Run("nil parse tree should return error", func(t *testing.T) {
		options := outbound.SemanticExtractionOptions{}

		_, err := parser.ExtractModules(ctx, nil, options)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse tree cannot be nil")
	})

	t.Run("unsupported language should return error", func(t *testing.T) {
		pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, pythonLang, "print('hello')")
		options := outbound.SemanticExtractionOptions{}

		_, err = parser.ExtractModules(ctx, parseTree, options)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported language")
	})

	t.Run("malformed JavaScript should not panic", func(t *testing.T) {
		malformedCode := `// Malformed export syntax
export { unclosedBrace
export default
// Missing semicolons and incomplete statements
const incomplete = 
`
		language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, malformedCode)
		options := outbound.SemanticExtractionOptions{}

		// Should not panic, even with malformed code
		modules, err := parser.ExtractModules(ctx, parseTree, options)

		// May return error or partial results, but should not panic
		if err == nil {
			assert.NotNil(t, modules)
			// If successful, should still create a module entry
			if len(modules) > 0 {
				assert.Equal(t, outbound.ConstructModule, modules[0].Type)
			}
		}
	})

	t.Run("empty module should return basic module info", func(t *testing.T) {
		emptyCode := `// empty_module.js
// This module is intentionally empty`

		language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
		require.NoError(t, err)

		parseTree := createMockParseTreeFromSource(t, language, emptyCode)
		options := outbound.SemanticExtractionOptions{
			IncludeDocumentation: true,
			IncludeMetadata:      true,
		}

		modules, err := parser.ExtractModules(ctx, parseTree, options)
		require.NoError(t, err)
		require.Len(t, modules, 1)

		module := modules[0]
		assert.Equal(t, "empty_module", module.Name)
		assert.Equal(t, outbound.ConstructModule, module.Type)

		// Should have empty exports
		exports := module.Metadata["exports"].([]map[string]interface{})
		assert.Empty(t, exports, "Empty module should have no exports")

		// Should still have basic stats
		stats := module.Metadata["export_stats"].(map[string]int)
		assert.Equal(t, 0, stats["total_exports"])
	})
}

// Helper functions for testing

// extractExportNames extracts export names from export metadata list.
func extractExportNames(exports []map[string]interface{}) []string {
	names := make([]string, len(exports))
	for i, export := range exports {
		names[i] = export["name"].(string)
	}
	return names
}

// findExportByName finds an export by name in the exports list.
func findExportByName(exports []map[string]interface{}, name string) map[string]interface{} {
	for _, export := range exports {
		if export["name"].(string) == name {
			return export
		}
	}
	return nil
}

// findExportByDefault finds the default export in the exports list.
func findExportByDefault(exports []map[string]interface{}) map[string]interface{} {
	for _, export := range exports {
		if isDefault, ok := export["is_default"].(bool); ok && isDefault {
			return export
		}
	}
	return nil
}

// findReExportByPath finds a re-export by path in the re-exports list.
func findReExportByPath(reExports []map[string]interface{}, path string) map[string]interface{} {
	for _, reExport := range reExports {
		if reExport["path"].(string) == path {
			return reExport
		}
	}
	return nil
}

// findModuleImportByPath finds an import by path in the module imports list.
func findModuleImportByPath(imports []map[string]interface{}, path string) map[string]interface{} {
	for _, imp := range imports {
		if imp["path"].(string) == path {
			return imp
		}
	}
	return nil
}
