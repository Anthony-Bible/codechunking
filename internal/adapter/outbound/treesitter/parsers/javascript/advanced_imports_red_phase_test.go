package javascriptparser

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJavaScriptImports_ComplexES6Aliasing(t *testing.T) {
	sourceCode := `
// Complex aliasing patterns
import { 
  originalName as alias,
  another as different,
  third as renamed
} from './complex-aliases';

// Mixed imports with aliasing
import defaultExport, { named as namedAlias } from './mixed-aliases';
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		IncludeMetadata: true,
		MaxDepth:        10,
	}

	imports, err := adapter.ExtractImports(ctx, parseTree, options)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(imports), 2)

	// Test complex aliases import
	complexImport := findImportByPath(imports, "./complex-aliases")
	require.NotNil(t, complexImport, "Should find complex-aliases import")

	assert.Equal(t, "./complex-aliases", complexImport.Path)
	assert.Equal(t, []string{"originalName", "another", "third"}, complexImport.ImportedSymbols)
	assert.Empty(t, complexImport.Alias)
	assert.False(t, complexImport.IsWildcard)

	// Test alias mapping in metadata
	aliasMap, ok := complexImport.Metadata["aliases"]
	assert.True(t, ok, "Should have aliases metadata")
	aliasMapping := aliasMap.(map[string]string)
	assert.Equal(t, "alias", aliasMapping["originalName"])
	assert.Equal(t, "different", aliasMapping["another"])
	assert.Equal(t, "renamed", aliasMapping["third"])

	// Test positions and content
	assert.Greater(t, complexImport.EndByte, complexImport.StartByte)
	assert.GreaterOrEqual(t, complexImport.EndPosition.Row, complexImport.StartPosition.Row)
	assert.NotEmpty(t, complexImport.Content)
	assert.WithinDuration(t, time.Now(), complexImport.ExtractedAt, time.Second)
	assert.NotEmpty(t, complexImport.Hash)

	// Test mixed aliases import
	mixedImport := findImportByPath(imports, "./mixed-aliases")
	require.NotNil(t, mixedImport, "Should find mixed-aliases import")

	assert.Equal(t, "./mixed-aliases", mixedImport.Path)
	assert.Equal(t, []string{"defaultExport", "named"}, mixedImport.ImportedSymbols)
	assert.Equal(t, "defaultExport", mixedImport.Alias)
	assert.False(t, mixedImport.IsWildcard)

	// Test mixed alias mapping
	mixedAliasMap, ok := mixedImport.Metadata["aliases"]
	assert.True(t, ok, "Should have aliases metadata for mixed import")
	mixedAliasMapping := mixedAliasMap.(map[string]string)
	assert.Equal(t, "namedAlias", mixedAliasMapping["named"])
}

func TestJavaScriptImports_ImportAssertions(t *testing.T) {
	sourceCode := `
import config from './config.json' assert { type: 'json' };
import wasm from './module.wasm' assert { type: 'webassembly' };
import styles from './styles.css' assert { type: 'css' };
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		IncludeMetadata: true,
		MaxDepth:        10,
	}

	imports, err := adapter.ExtractImports(ctx, parseTree, options)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(imports), 3)

	// Test JSON import with assertions
	jsonImport := findImportByPath(imports, "./config.json")
	require.NotNil(t, jsonImport, "Should find JSON import")

	assert.Equal(t, "./config.json", jsonImport.Path)
	assert.Equal(t, []string{"config"}, jsonImport.ImportedSymbols)

	hasAssertions, ok := jsonImport.Metadata["has_assertions"]
	assert.True(t, ok, "Should detect assertions")
	assert.Equal(t, true, hasAssertions)

	assertionType, ok := jsonImport.Metadata["assertion_type"]
	assert.True(t, ok, "Should have assertion type")
	assert.Equal(t, "json", assertionType)

	// Test WASM import with assertions
	wasmImport := findImportByPath(imports, "./module.wasm")
	require.NotNil(t, wasmImport, "Should find WASM import")

	assert.Equal(t, "./module.wasm", wasmImport.Path)
	assert.Equal(t, []string{"wasm"}, wasmImport.ImportedSymbols)

	hasAssertions, ok = wasmImport.Metadata["has_assertions"]
	assert.True(t, ok, "Should detect assertions for WASM")
	assert.Equal(t, true, hasAssertions)

	assertionType, ok = wasmImport.Metadata["assertion_type"]
	assert.True(t, ok, "Should have assertion type for WASM")
	assert.Equal(t, "webassembly", assertionType)

	// Test CSS import with assertions
	cssImport := findImportByPath(imports, "./styles.css")
	require.NotNil(t, cssImport, "Should find CSS import")

	assert.Equal(t, "./styles.css", cssImport.Path)
	assert.Equal(t, []string{"styles"}, cssImport.ImportedSymbols)

	hasAssertions, ok = cssImport.Metadata["has_assertions"]
	assert.True(t, ok, "Should detect assertions for CSS")
	assert.Equal(t, true, hasAssertions)

	assertionType, ok = cssImport.Metadata["assertion_type"]
	assert.True(t, ok, "Should have assertion type for CSS")
	assert.Equal(t, "css", assertionType)
}

func TestJavaScriptImports_DeepCommonJSDestructuring(t *testing.T) {
	sourceCode := `
const { 
  database: { 
    connection: { pool, config: dbConfig }
  },
  logger: { error, info: logInfo }
} = require('./services');
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		IncludeMetadata: true,
		MaxDepth:        10,
	}

	imports, err := adapter.ExtractImports(ctx, parseTree, options)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(imports), 1)

	// Test services import
	servicesImport := findImportByPath(imports, "./services")
	require.NotNil(t, servicesImport, "Should find services import")

	assert.Equal(t, "./services", servicesImport.Path)
	assert.Equal(
		t,
		[]string{"database.connection.pool", "database.connection.config", "logger.error", "logger.info"},
		servicesImport.ImportedSymbols,
	)

	hasNested, ok := servicesImport.Metadata["has_nested_destructuring"]
	assert.True(t, ok, "Should detect nested destructuring")
	assert.Equal(t, true, hasNested)

	importType, ok := servicesImport.Metadata["import_type"]
	assert.True(t, ok, "Should have import type")
	assert.Equal(t, "commonjs", importType)

	// Test alias mapping
	aliasMap, ok := servicesImport.Metadata["aliases"]
	assert.True(t, ok, "Should have aliases metadata")
	aliasMapping := aliasMap.(map[string]string)
	assert.Equal(t, "dbConfig", aliasMapping["database.connection.config"])
	assert.Equal(t, "logInfo", aliasMapping["logger.info"])
}

func TestJavaScriptImports_ConditionalRequires(t *testing.T) {
	sourceCode := `
if (process.env.NODE_ENV === 'development') {
  const devTools = require('./dev-tools');
}

let optionalModule;
try {
  optionalModule = require('./optional');
} catch (e) {
  // ignore
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
		IncludeMetadata: true,
		MaxDepth:        10,
	}

	imports, err := adapter.ExtractImports(ctx, parseTree, options)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(imports), 2)

	// Test conditional require
	devToolsImport := findImportByPath(imports, "./dev-tools")
	require.NotNil(t, devToolsImport, "Should find dev-tools import")

	assert.Equal(t, "./dev-tools", devToolsImport.Path)
	assert.Equal(t, []string{"devTools"}, devToolsImport.ImportedSymbols)

	isConditional, ok := devToolsImport.Metadata["is_conditional"]
	assert.True(t, ok, "Should detect conditional import")
	assert.Equal(t, true, isConditional)

	conditionType, ok := devToolsImport.Metadata["condition_type"]
	assert.True(t, ok, "Should have condition type")
	assert.Equal(t, "if", conditionType)

	// Test try-catch require
	optionalImport := findImportByPath(imports, "./optional")
	require.NotNil(t, optionalImport, "Should find optional import")

	assert.Equal(t, "./optional", optionalImport.Path)
	assert.Equal(t, []string{"optionalModule"}, optionalImport.ImportedSymbols)

	isConditional, ok = optionalImport.Metadata["is_conditional"]
	assert.True(t, ok, "Should detect conditional import for try-catch")
	assert.Equal(t, true, isConditional)

	conditionType, ok = optionalImport.Metadata["condition_type"]
	assert.True(t, ok, "Should have condition type for try-catch")
	assert.Equal(t, "try-catch", conditionType)
}

func TestJavaScriptImports_PathCategorization(t *testing.T) {
	sourceCode := `
import react from 'react';                    // NPM package
import scoped from '@org/package';            // Scoped NPM
import local from './local';                  // Local relative
import parent from '../parent';               // Parent relative
import deep from '../../deep/path';           // Deep relative
import absolute from '/absolute/path';        // Absolute
import protocol from 'http://example.com';    // Protocol
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		IncludeMetadata: true,
		MaxDepth:        10,
	}

	imports, err := adapter.ExtractImports(ctx, parseTree, options)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(imports), 7)

	// Test NPM package
	reactImport := findImportByPath(imports, "react")
	require.NotNil(t, reactImport, "Should find react import")

	pathType, ok := reactImport.Metadata["path_type"]
	assert.True(t, ok, "Should have path type for NPM package")
	assert.Equal(t, "npm", pathType)

	// Test scoped NPM package
	scopedImport := findImportByPath(imports, "@org/package")
	require.NotNil(t, scopedImport, "Should find scoped package import")

	pathType, ok = scopedImport.Metadata["path_type"]
	assert.True(t, ok, "Should have path type for scoped package")
	assert.Equal(t, "scoped-npm", pathType)

	isScoped, ok := scopedImport.Metadata["is_scoped_package"]
	assert.True(t, ok, "Should detect scoped package")
	assert.Equal(t, true, isScoped)

	// Test local relative
	localImport := findImportByPath(imports, "./local")
	require.NotNil(t, localImport, "Should find local import")

	pathType, ok = localImport.Metadata["path_type"]
	assert.True(t, ok, "Should have path type for local relative")
	assert.Equal(t, "relative", pathType)

	relativeDepth, ok := localImport.Metadata["relative_depth"]
	assert.True(t, ok, "Should have relative depth")
	assert.Equal(t, 1, relativeDepth)

	// Test parent relative
	parentImport := findImportByPath(imports, "../parent")
	require.NotNil(t, parentImport, "Should find parent import")

	pathType, ok = parentImport.Metadata["path_type"]
	assert.True(t, ok, "Should have path type for parent relative")
	assert.Equal(t, "relative", pathType)

	relativeDepth, ok = parentImport.Metadata["relative_depth"]
	assert.True(t, ok, "Should have relative depth for parent")
	assert.Equal(t, 2, relativeDepth)

	// Test deep relative
	deepImport := findImportByPath(imports, "../../deep/path")
	require.NotNil(t, deepImport, "Should find deep path import")

	pathType, ok = deepImport.Metadata["path_type"]
	assert.True(t, ok, "Should have path type for deep relative")
	assert.Equal(t, "relative", pathType)

	relativeDepth, ok = deepImport.Metadata["relative_depth"]
	assert.True(t, ok, "Should have relative depth for deep path")
	assert.Equal(t, 3, relativeDepth)

	// Test absolute path
	absoluteImport := findImportByPath(imports, "/absolute/path")
	require.NotNil(t, absoluteImport, "Should find absolute path import")

	pathType, ok = absoluteImport.Metadata["path_type"]
	assert.True(t, ok, "Should have path type for absolute path")
	assert.Equal(t, "absolute", pathType)

	// Test protocol path
	protocolImport := findImportByPath(imports, "http://example.com")
	require.NotNil(t, protocolImport, "Should find protocol path import")

	pathType, ok = protocolImport.Metadata["path_type"]
	assert.True(t, ok, "Should have path type for protocol path")
	assert.Equal(t, "protocol", pathType)
}

func TestJavaScriptImports_ImportStyleClassification(t *testing.T) {
	sourceCode := `
import './side-effect.css';                   // Side effect
import defaultOnly from 'module';            // Default only
import { namedOnly } from 'module';           // Named only
import * as namespaceOnly from 'module';     // Namespace only
import def, { named } from 'module';          // Mixed default+named
import def, * as ns from 'module';           // Mixed default+namespace
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		IncludeMetadata: true,
		MaxDepth:        10,
	}

	imports, err := adapter.ExtractImports(ctx, parseTree, options)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(imports), 6)

	// Test side effect import
	sideEffectImport := findImportByPath(imports, "./side-effect.css")
	require.NotNil(t, sideEffectImport, "Should find side effect import")

	importStyle, ok := sideEffectImport.Metadata["import_style"]
	assert.True(t, ok, "Should have import style for side effect")
	assert.Equal(t, "side-effect", importStyle)

	hasSideEffects, ok := sideEffectImport.Metadata["has_side_effects"]
	assert.True(t, ok, "Should detect side effects")
	assert.Equal(t, true, hasSideEffects)

	// Test default only import
	defaultImport := findImportByPath(imports, "module")
	require.NotNil(t, defaultImport, "Should find default import")

	importStyle, ok = defaultImport.Metadata["import_style"]
	assert.True(t, ok, "Should have import style for default only")
	assert.Equal(t, "default", importStyle)

	// Test named only import
	namedImport := findImportBySymbols(imports, []string{"namedOnly"})
	require.NotNil(t, namedImport, "Should find named only import")

	importStyle, ok = namedImport.Metadata["import_style"]
	assert.True(t, ok, "Should have import style for named only")
	assert.Equal(t, "named", importStyle)

	// Test namespace only import
	namespaceImport := findImportBySymbols(imports, []string{"*"})
	require.NotNil(t, namespaceImport, "Should find namespace only import")

	importStyle, ok = namespaceImport.Metadata["import_style"]
	assert.True(t, ok, "Should have import style for namespace only")
	assert.Equal(t, "namespace", importStyle)

	// Test mixed default+named import
	mixedNamedImport := findImportBySymbols(imports, []string{"def", "named"})
	require.NotNil(t, mixedNamedImport, "Should find mixed default+named import")

	importStyle, ok = mixedNamedImport.Metadata["import_style"]
	assert.True(t, ok, "Should have import style for mixed default+named")
	assert.Equal(t, "mixed", importStyle)

	isMixedImport, ok := mixedNamedImport.Metadata["is_mixed_import"]
	assert.True(t, ok, "Should detect mixed import")
	assert.Equal(t, true, isMixedImport)

	// Test mixed default+namespace import
	mixedNamespaceImport := findImportBySymbols(imports, []string{"def", "*"})
	require.NotNil(t, mixedNamespaceImport, "Should find mixed default+namespace import")

	importStyle, ok = mixedNamespaceImport.Metadata["import_style"]
	assert.True(t, ok, "Should have import style for mixed default+namespace")
	assert.Equal(t, "mixed", importStyle)

	isMixedImport, ok = mixedNamespaceImport.Metadata["is_mixed_import"]
	assert.True(t, ok, "Should detect mixed import for default+namespace")
	assert.Equal(t, true, isMixedImport)
}

func TestJavaScriptImports_DynamicImportAdvanced(t *testing.T) {
	sourceCode := `
// Template literals
const moduleName = 'utils';
const module1 = await import('./modules/' + moduleName + '.js');

// Conditional dynamic imports
if (condition) {
  const conditional = await import('./conditional');
}

// Error handling
try {
  const optional = await import('./optional');
} catch (error) {
  console.log('Failed to load optional module');
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
		IncludeMetadata: true,
		MaxDepth:        10,
	}

	imports, err := adapter.ExtractImports(ctx, parseTree, options)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(imports), 3)

	// Test template literal dynamic import
	templateImport := findImportByPath(imports, "./modules/${moduleName}.js")
	require.NotNil(t, templateImport, "Should find template literal dynamic import")

	isDynamic, ok := templateImport.Metadata["is_dynamic"]
	assert.True(t, ok, "Should detect dynamic import")
	assert.Equal(t, true, isDynamic)

	hasTemplatePath, ok := templateImport.Metadata["has_template_path"]
	assert.True(t, ok, "Should detect template path")
	assert.Equal(t, true, hasTemplatePath)

	// Test conditional dynamic import
	conditionalImport := findImportByPath(imports, "./conditional")
	require.NotNil(t, conditionalImport, "Should find conditional dynamic import")

	isDynamic, ok = conditionalImport.Metadata["is_dynamic"]
	assert.True(t, ok, "Should detect dynamic import for conditional")
	assert.Equal(t, true, isDynamic)

	isConditional, ok := conditionalImport.Metadata["is_conditional"]
	assert.True(t, ok, "Should detect conditional dynamic import")
	assert.Equal(t, true, isConditional)

	// Test try-catch dynamic import
	optionalImport := findImportByPath(imports, "./optional")
	require.NotNil(t, optionalImport, "Should find try-catch dynamic import")

	isDynamic, ok = optionalImport.Metadata["is_dynamic"]
	assert.True(t, ok, "Should detect dynamic import for try-catch")
	assert.Equal(t, true, isDynamic)

	isConditional, ok = optionalImport.Metadata["is_conditional"]
	assert.True(t, ok, "Should detect conditional dynamic import for try-catch")
	assert.Equal(t, true, isConditional)
}

func TestJavaScriptImports_ComplexMalformedHandling(t *testing.T) {
	sourceCode := `
// Should be gracefully handled without panic
import from 'missing-specifier';
import { } from 'empty-specifier';  
import 'unterminated-string
require();
const incomplete = require(
`

	language, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	require.NoError(t, err)

	parseTree := createMockParseTreeFromSource(t, language, sourceCode)

	ctx := context.Background()
	adapter := treesitter.NewSemanticTraverserAdapter()

	options := outbound.SemanticExtractionOptions{
		IncludePrivate:  true,
		IncludeTypeInfo: true,
		IncludeMetadata: true,
		MaxDepth:        10,
	}

	imports, err := adapter.ExtractImports(ctx, parseTree, options)
	require.NoError(t, err)

	// Should not panic and return 0 imports for malformed syntax
	assert.Empty(t, imports)

	// Test that we can still extract valid imports even with malformed ones present
	validSource := `
import from 'missing-specifier';
import { valid } from './valid-module';
import 'unterminated-string
`

	parseTree = createMockParseTreeFromSource(t, language, validSource)
	imports, err = adapter.ExtractImports(ctx, parseTree, options)
	require.NoError(t, err)

	// Should extract the valid import
	assert.Len(t, imports, 1)
	validImport := findImportByPath(imports, "./valid-module")
	assert.NotNil(t, validImport, "Should find valid import despite malformed syntax")
}
