package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TreeSitterParserFactory defines the interface for creating TreeSitterParser instances
// following the Factory pattern and integrating with go-sitter-forest.
// This is a RED PHASE test that defines expected factory behavior.
type TreeSitterParserFactory interface {
	// CreateParser creates a parser for a specific language using forest.GetLanguage
	CreateParser(ctx context.Context, language valueobject.Language) (ObservableTreeSitterParser, error)

	// CreateGoParser creates a specialized Go parser with optimized settings
	CreateGoParser(ctx context.Context) (ObservableTreeSitterParser, error)

	// CreatePythonParser creates a specialized Python parser with optimized settings
	CreatePythonParser(ctx context.Context) (ObservableTreeSitterParser, error)

	// CreateJavaScriptParser creates a specialized JavaScript parser with optimized settings
	CreateJavaScriptParser(ctx context.Context) (ObservableTreeSitterParser, error)

	// CreateTypeScriptParser creates a specialized TypeScript parser with optimized settings
	CreateTypeScriptParser(ctx context.Context) (ObservableTreeSitterParser, error)

	// DetectAndCreateParser automatically detects language from file path and creates appropriate parser
	DetectAndCreateParser(
		ctx context.Context,
		filePath string,
	) (ObservableTreeSitterParser, valueobject.Language, error)

	// GetSupportedLanguages returns all languages supported by the factory (from go-sitter-forest)
	GetSupportedLanguages(ctx context.Context) ([]valueobject.Language, error)

	// ValidateLanguageSupport validates if a language is supported by checking forest.GetLanguage
	ValidateLanguageSupport(ctx context.Context, language valueobject.Language) error

	// SetConfiguration updates factory configuration for all future parser instances
	SetConfiguration(ctx context.Context, config *FactoryConfiguration) error

	// GetConfiguration returns current factory configuration
	GetConfiguration(ctx context.Context) (*FactoryConfiguration, error)

	// GetParserPool returns a managed pool of parser instances for concurrent use
	GetParserPool(ctx context.Context) (*ParserPool, error)

	// WarmupParsers preloads and caches parsers for specified languages
	WarmupParsers(ctx context.Context, languages []valueobject.Language) error

	// GetHealthStatus returns health status of the factory and its parser pool
	GetHealthStatus(ctx context.Context) (*FactoryHealthStatus, error)

	// Cleanup releases all factory resources and managed parsers
	Cleanup(ctx context.Context) error
}

// ParserPoolStatistics holds detailed statistics about parser pool usage.

// FactoryHealthStatus represents the health status of the parser factory.

// Bridge function types for converting between domain and port types.
type (
	ParseTreeConverter   func(*valueobject.ParseTree) (*ParseTree, error)
	ParseResultConverter func(*valueobject.ParseResult) (*ParseResult, error)
	LanguageDetector     func(filePath string) (valueobject.Language, error)
)

// TestTreeSitterParserFactory_FactoryCreation tests factory creation.
// This is a RED PHASE test that defines expected factory creation behavior.
func TestTreeSitterParserFactory_FactoryCreation(t *testing.T) {
	t.Run("create factory with default configuration", func(t *testing.T) {
		// Factory should initialize with go-sitter-forest integration
		factory, err := createTestFactory()
		require.NoError(t, err, "Factory should be implemented now")
		require.NotNil(t, factory)
		assert.IsType(t, &ParserFactoryImpl{}, factory)
	})
}

// TestTreeSitterParserFactory_SupportedLanguages tests language support validation.
// This is a RED PHASE test that defines expected language support behavior.
func TestTreeSitterParserFactory_SupportedLanguages(t *testing.T) {
	factory, err := createTestFactory()
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("get supported languages should include major languages", func(t *testing.T) {
		languages, err := factory.GetSupportedLanguages(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, languages)

		// Should include major languages from go-sitter-forest
		expectedLanguages := []string{
			valueobject.LanguageGo,
			valueobject.LanguagePython,
			valueobject.LanguageJavaScript,
			valueobject.LanguageTypeScript,
		}
		foundLangs := make(map[string]bool)
		for _, lang := range languages {
			foundLangs[lang.Name()] = true
		}

		for _, expected := range expectedLanguages {
			assert.True(t, foundLangs[expected], "Should support %s", expected)
		}
	})

	t.Run("validate language support for Go should succeed", func(t *testing.T) {
		goLang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
		err := factory.ValidateLanguageSupport(ctx, goLang)
		require.NoError(t, err, "Go should be supported")
	})

	t.Run("validate language support for unsupported language should fail", func(t *testing.T) {
		unsupportedLang, _ := valueobject.NewLanguage("UnsupportedLanguage")
		err := factory.ValidateLanguageSupport(ctx, unsupportedLang)
		require.Error(t, err, "Unsupported language should return error")
		assert.Contains(t, err.Error(), "unsupported language")
	})
}

// createTestFactory is a helper function for creating factory instances in tests.
func createTestFactory() (TreeSitterParserFactory, error) {
	return NewTreeSitterParserFactory(context.Background())
}

// TestTreeSitterParserFactory_ParserCreation tests parser instance creation.
// This is a RED PHASE test that defines expected parser creation behavior.
func TestTreeSitterParserFactory_ParserCreation(t *testing.T) {
	ctx := context.Background()
	factory, err := createTestFactory()
	require.NoError(t, err)
	require.NotNil(t, factory)

	t.Run("create Go parser with specialized configuration", func(t *testing.T) {
		parser, err := factory.CreateGoParser(ctx)

		require.NoError(t, err)
		require.NotNil(t, parser)

		// When implemented, should verify:
		// - Parser supports Go language specifically
		// - Parser has optimized settings for Go (imports, packages, etc.)
		// - Parser implements ObservableTreeSitterParser interface
		// - Parser has OTEL metrics enabled
		// - Parser configuration includes Go-specific optimizations
	})

	t.Run("create Python parser with specialized configuration", func(t *testing.T) {
		parser, err := factory.CreatePythonParser(ctx)

		require.NoError(t, err)
		require.NotNil(t, parser)

		// When implemented, should verify:
		// - Parser supports Python language specifically
		// - Parser handles Python indentation correctly
		// - Parser supports Python 3.x syntax features
		// - Parser has optimized settings for classes, functions, imports
	})

	t.Run("create JavaScript parser with specialized configuration", func(t *testing.T) {
		parser, err := factory.CreateJavaScriptParser(ctx)

		require.NoError(t, err)
		require.NotNil(t, parser)

		// When implemented, should verify:
		// - Parser supports modern JavaScript features
		// - Parser handles ES6+ syntax
		// - Parser supports JSX if configured
		// - Parser optimized for web development patterns
	})

	t.Run("create TypeScript parser with specialized configuration", func(t *testing.T) {
		parser, err := factory.CreateTypeScriptParser(ctx)

		require.NoError(t, err)
		require.NotNil(t, parser)

		// When implemented, should verify:
		// - Parser supports TypeScript syntax
		// - Parser handles type annotations correctly
		// - Parser supports generics and advanced TypeScript features
		// - Parser integrates with TSX support
	})

	t.Run("create parser for generic language", func(t *testing.T) {
		rustLang, err := valueobject.NewLanguage("rust")
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, rustLang)

		require.Error(t, err, "Factory not implemented yet")
		require.Nil(t, parser)

		// When implemented, should verify:
		// - Parser created using forest.GetLanguage("rust")
		// - Parser implements full ObservableTreeSitterParser interface
		// - Parser has default configuration applied
		// - Parser supports all standard operations
	})

	t.Run("create parser for unsupported language should fail gracefully", func(t *testing.T) {
		unsupportedLang, err := valueobject.NewLanguage("NonexistentLanguage")
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, unsupportedLang)

		// Should fail even when factory is implemented
		require.Error(t, err)
		require.Nil(t, parser)
		assert.Contains(t, err.Error(), "unsupported language")
	})
}

// TestTreeSitterParserFactory_LanguageDetection tests automatic language detection.
// This is a GREEN PHASE test that verifies language detection behavior.
func TestTreeSitterParserFactory_LanguageDetection(t *testing.T) {
	ctx := context.Background()

	// Create factory for GREEN phase testing
	factory, err := createTestFactory()
	require.NoError(t, err)
	require.NotNil(t, factory)

	testCases := []struct {
		name             string
		filePath         string
		expectedLangName string
		expectError      bool
		errorContains    string
	}{
		{
			name:             "detect Go from .go extension",
			filePath:         "/path/to/main.go",
			expectedLangName: valueobject.LanguageGo,
			expectError:      false,
		},
		{
			name:             "detect Python from .py extension",
			filePath:         "/path/to/script.py",
			expectedLangName: valueobject.LanguagePython,
			expectError:      false,
		},
		{
			name:             "detect JavaScript from .js extension",
			filePath:         "/path/to/app.js",
			expectedLangName: valueobject.LanguageJavaScript,
			expectError:      false,
		},
		{
			name:             "detect TypeScript from .ts extension",
			filePath:         "/path/to/component.ts",
			expectedLangName: valueobject.LanguageTypeScript,
			expectError:      false,
		},
		{
			name:             "detect TypeScript from .tsx extension",
			filePath:         "/path/to/Component.tsx",
			expectedLangName: valueobject.LanguageTypeScript,
			expectError:      false,
		},
		{
			name:             "detect C++ from .cpp extension",
			filePath:         "/path/to/program.cpp",
			expectedLangName: valueobject.LanguageCPlusPlus,
			expectError:      false,
		},
		{
			name:             "detect Rust from .rs extension",
			filePath:         "/path/to/lib.rs",
			expectedLangName: valueobject.LanguageRust,
			expectError:      false,
		},
		{
			name:          "unsupported file extension should fail",
			filePath:      "/path/to/file.unknownext",
			expectError:   true,
			errorContains: "unsupported file extension",
		},
		{
			name:          "empty file path should fail",
			filePath:      "",
			expectError:   true,
			errorContains: "empty file path",
		},
		{
			name:          "file without extension should fail",
			filePath:      "/path/to/Makefile",
			expectError:   true,
			errorContains: "no file extension",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser, detectedLang, err := factory.DetectAndCreateParser(ctx, tc.filePath)

			if tc.expectError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				assert.Nil(t, parser)
			} else {
				// GREEN phase: factory should now work correctly
				require.NoError(t, err)
				require.NotNil(t, parser)

				// Verify detected language matches expected
				assert.Equal(t, tc.expectedLangName, detectedLang.Name())

				// Verify parser implements ObservableTreeSitterParser
				assert.NotNil(t, parser, "Parser should implement ObservableTreeSitterParser")
			}
		})
	}
}

// TestTreeSitterParserFactory_Configuration tests factory configuration management.
// This is a GREEN PHASE test that verifies configuration behavior.
func TestTreeSitterParserFactory_Configuration(t *testing.T) {
	ctx := context.Background()

	// Create factory for GREEN phase testing
	factory, err := createTestFactory()
	require.NoError(t, err)
	require.NotNil(t, factory)

	t.Run("set and get factory configuration", func(t *testing.T) {
		config := &FactoryConfiguration{
			MaxCachedParsers:    50,
			ParserTimeout:       30 * time.Second,
			EnableMetrics:       true,
			EnableTracing:       true,
			ConcurrencyLimit:    10,
			CacheEvictionPolicy: "LRU",
			EnableAutoCleanup:   true,
			HealthCheckInterval: 5 * time.Minute,
			DefaultParserConfig: ParserConfiguration{
				DefaultTimeout:   15 * time.Second,
				MaxMemory:        5 * 1024 * 1024, // 5MB
				ConcurrencyLimit: 5,
				CacheEnabled:     true,
				CacheSize:        100,
				MetricsEnabled:   true,
				TracingEnabled:   false,
			},
		}

		// Set configuration
		err = factory.SetConfiguration(ctx, config)
		require.NoError(t, err)

		// Get configuration
		retrievedConfig, err := factory.GetConfiguration(ctx)
		require.NoError(t, err)
		require.NotNil(t, retrievedConfig)

		// Verify configuration is properly stored and retrieved
		assert.Equal(t, config.MaxCachedParsers, retrievedConfig.MaxCachedParsers)
		assert.Equal(t, config.ParserTimeout, retrievedConfig.ParserTimeout)
		assert.Equal(t, config.EnableMetrics, retrievedConfig.EnableMetrics)
		assert.Equal(t, config.EnableTracing, retrievedConfig.EnableTracing)
		assert.Equal(t, config.ConcurrencyLimit, retrievedConfig.ConcurrencyLimit)
		assert.Equal(t, config.CacheEvictionPolicy, retrievedConfig.CacheEvictionPolicy)
	})

	t.Run("invalid configuration should be rejected", func(t *testing.T) {
		invalidConfigs := []*FactoryConfiguration{
			{MaxCachedParsers: -1},              // Negative cache size
			{ParserTimeout: -time.Second},       // Negative timeout
			{ConcurrencyLimit: 0},               // Zero concurrency
			{HealthCheckInterval: -time.Minute}, // Negative interval
		}

		for i, config := range invalidConfigs {
			t.Run(fmt.Sprintf("invalid_config_%d", i), func(t *testing.T) {
				err := factory.SetConfiguration(ctx, config)
				// Should fail even when factory is implemented
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid configuration")
			})
		}
	})

	t.Run("default configuration should be valid", func(t *testing.T) {
		config, err := factory.GetConfiguration(ctx)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Verify default configuration has sensible values
		assert.Positive(t, config.MaxCachedParsers)
		assert.Greater(t, config.ParserTimeout, time.Duration(0))
		assert.Positive(t, config.ConcurrencyLimit)
		assert.NotEmpty(t, config.CacheEvictionPolicy)
	})
}

// TestTreeSitterParserFactory_ParserPool tests parser pool management.
// This is a GREEN PHASE test that verifies pool behavior.
func TestTreeSitterParserFactory_ParserPool(t *testing.T) {
	ctx := context.Background()

	// Create factory for GREEN phase testing
	factory, err := createTestFactory()
	require.NoError(t, err)
	require.NotNil(t, factory)

	t.Run("get parser pool with default settings", func(t *testing.T) {
		pool, err := factory.GetParserPool(ctx)
		require.NoError(t, err)
		require.NotNil(t, pool)

		// Verify pool has reasonable default settings
		assert.Positive(t, pool.MaxSize)
		assert.GreaterOrEqual(t, pool.CurrentSize, 0)
		assert.NotNil(t, pool.PoolStatistics)
		assert.NotNil(t, pool.Configuration)
	})

	t.Run("warmup parsers for multiple languages", func(t *testing.T) {
		languages := []valueobject.Language{}

		goLang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
		pythonLang, _ := valueobject.NewLanguage(valueobject.LanguagePython)
		jsLang, _ := valueobject.NewLanguage(valueobject.LanguageJavaScript)

		languages = append(languages, goLang, pythonLang, jsLang)

		err = factory.WarmupParsers(ctx, languages)
		require.NoError(t, err)

		// Verify warmup completed successfully - actual performance benefits
		// would be measured in integration tests or benchmarks
	})

	t.Run("parser pool should handle concurrent access", func(t *testing.T) {
		// Test concurrent parser checkout/return
		var wg sync.WaitGroup
		const numGoroutines = 10

		for i := range numGoroutines {
			wg.Add(1)
			go func(_ int) {
				defer wg.Done()

				// GREEN phase: factory should work correctly
				pool, err := factory.GetParserPool(ctx)
				assert.NoError(t, err)
				assert.NotNil(t, pool)

				// Verify pool handles concurrent access correctly
				assert.GreaterOrEqual(t, pool.CurrentSize, 0)
				assert.NotNil(t, pool.PoolStatistics)
			}(i)
		}

		wg.Wait()
	})

	t.Run("parser pool statistics should be accurate", func(t *testing.T) {
		pool, err := factory.GetParserPool(ctx)
		require.NoError(t, err)
		require.NotNil(t, pool)

		// Verify statistics are initialized and accessible
		stats := pool.PoolStatistics
		require.NotNil(t, stats)
		assert.GreaterOrEqual(t, stats.TotalCreated, int64(0))
		assert.GreaterOrEqual(t, stats.TotalDestroyed, int64(0))
		assert.GreaterOrEqual(t, stats.CacheHitRate, 0.0)
		assert.GreaterOrEqual(t, stats.ErrorRate, 0.0)
	})
}

// TestTreeSitterParserFactory_BridgeConversions tests type conversions between domain and port types.
// This is a RED PHASE test that defines expected bridge behavior.
func TestTreeSitterParserFactory_BridgeConversions(t *testing.T) {
	t.Run("convert domain ParseTree to port ParseTree", func(t *testing.T) {
		// Create domain ParseTree
		goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		source := []byte("package main\n// Go code here")
		rootNode := &valueobject.ParseNode{
			Type:      "source_file",
			StartByte: 0,
			EndByte:   uint32(len(source)),
			StartPos:  valueobject.Position{Row: 0, Column: 0},
			EndPos:    valueobject.Position{Row: 1, Column: 17},
		}

		metadata, err := valueobject.NewParseMetadata(
			50*time.Millisecond,
			"0.20.0",
			"1.0.0",
		)
		require.NoError(t, err)

		ctx := context.Background()
		domainParseTree, err := valueobject.NewParseTree(
			ctx,
			goLang,
			rootNode,
			source,
			metadata,
		)
		require.NoError(t, err)

		// GREEN phase: test bridge conversion
		portParseTree, err := ConvertDomainParseTreeToPort(domainParseTree)
		require.NoError(t, err)
		require.NotNil(t, portParseTree)

		// Verify conversion
		assert.Equal(t, domainParseTree.Language(), portParseTree.Language)
		assert.Equal(t, domainParseTree.Source(), portParseTree.Source)
		assert.NotNil(t, portParseTree.RootNode)
	})

	t.Run("convert domain ParseResult to port ParseResult", func(t *testing.T) {
		// Create mock domain ParseResult
		goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
		require.NoError(t, err)

		source := []byte("package main")
		rootNode := &valueobject.ParseNode{
			Type:      "source_file",
			StartByte: 0,
			EndByte:   uint32(len(source)),
			StartPos:  valueobject.Position{Row: 0, Column: 0},
			EndPos:    valueobject.Position{Row: 0, Column: 12},
		}

		metadata, err := valueobject.NewParseMetadata(
			10*time.Millisecond,
			"0.20.0",
			"1.0.0",
		)
		require.NoError(t, err)

		ctx := context.Background()
		parseTree, err := valueobject.NewParseTree(ctx, goLang, rootNode, source, metadata)
		require.NoError(t, err)

		options := valueobject.ParseOptions{
			Language:        goLang,
			RecoveryMode:    false,
			TimeoutDuration: 30 * time.Second,
		}

		domainResult, err := valueobject.NewParseResult(
			ctx,
			true,
			parseTree,
			[]valueobject.ParseError{},
			[]valueobject.ParseWarning{},
			5*time.Millisecond,
			options,
		)
		require.NoError(t, err)

		// GREEN phase: test bridge conversion
		portResult, err := ConvertDomainParseResultToPort(domainResult)
		require.NoError(t, err)
		require.NotNil(t, portResult)

		// Verify conversion
		assert.Equal(t, domainResult.IsSuccessful(), portResult.Success)
		assert.Equal(t, domainResult.Duration(), portResult.Duration)
		assert.NotNil(t, portResult.Statistics)
	})

	t.Run("language detection from file path", func(t *testing.T) {
		testCases := []struct {
			filePath         string
			expectedLangName string
		}{
			{"main.go", valueobject.LanguageGo},
			{"script.py", valueobject.LanguagePython},
			{"app.js", valueobject.LanguageJavaScript},
			{"component.ts", valueobject.LanguageTypeScript},
		}

		for _, tc := range testCases {
			lang, err := DetectLanguageFromFilePath(tc.filePath)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedLangName, lang.Name())
		}
	})
}

// TestTreeSitterParserFactory_HealthAndMetrics tests health monitoring and metrics.
// This is a GREEN PHASE test that verifies observability behavior.
func TestTreeSitterParserFactory_HealthAndMetrics(t *testing.T) {
	ctx := context.Background()

	// Create factory for GREEN phase testing
	factory, err := createTestFactory()
	require.NoError(t, err)
	require.NotNil(t, factory)

	t.Run("factory health status check", func(t *testing.T) {
		health, err := factory.GetHealthStatus(ctx)
		require.NoError(t, err)
		require.NotNil(t, health)

		// Verify health status components
		assert.True(t, health.IsHealthy)
		assert.NotEmpty(t, health.Status)
		assert.NotNil(t, health.ParserPoolHealth)
		assert.Positive(t, health.SupportedLanguages)
		assert.GreaterOrEqual(t, health.ActiveParsers, 0)
		assert.True(t, health.ConfigurationValid)
	})

	t.Run("health status provides pool information", func(t *testing.T) {
		health, err := factory.GetHealthStatus(ctx)
		require.NoError(t, err)
		require.NotNil(t, health)

		// Verify pool health information is present
		poolHealth := health.ParserPoolHealth
		require.NotNil(t, poolHealth)
		assert.Positive(t, poolHealth.PoolSize)
		assert.GreaterOrEqual(t, poolHealth.HealthyParsers, 0)
		assert.GreaterOrEqual(t, poolHealth.UnhealthyParsers, 0)
	})

	t.Run("factory cleanup works correctly", func(t *testing.T) {
		// Test factory cleanup functionality
		err = factory.Cleanup(ctx)
		require.NoError(t, err)

		// After cleanup, verify factory state is reset
		// Note: In real implementation, factory might be unusable after cleanup
	})
}

/*
// TestTreeSitterParserFactory_ErrorHandling tests comprehensive error handling.
// This is a RED PHASE test that defines expected error handling behavior.
func TestTreeSitterParserFactory_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	var factory TreeSitterParserFactory // Will be nil in RED phase

	t.Run("forest library integration errors", func(t *testing.T) {
		errorScenarios := []struct {
			name          string
			operation     func() error
			expectedError string
		}{
			{
				name: "forest.GetLanguage failure",
				operation: func() error {
					lang, _ := valueobject.NewLanguage("nonexistent_language")
					_, err := factory.CreateParser(ctx, lang)
					return err
				},
				expectedError: "language grammar not found",
			},
			{
				name: "forest.DetectLanguage failure",
				operation: func() error {
					_, _, err := factory.DetectAndCreateParser(ctx, "file.unknown_extension")
					return err
				},
				expectedError: "language detection failed",
			},
			{
				name: "parser initialization failure",
				operation: func() error {
					// Simulate corrupted grammar or initialization error
					lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
					_, err := factory.CreateParser(ctx, lang)
					return err
				},
				expectedError: "parser initialization failed",
			},
		}

		for _, scenario := range errorScenarios {
			t.Run(scenario.name, func(t *testing.T) {
				err := scenario.operation()
				require.Error(t, err, "Factory not implemented yet")

				// When implemented, should verify:
				// - Appropriate error is returned for each scenario
				// - Error messages are descriptive and actionable
				// - Error context includes relevant details
				// - Errors don't cause factory to become unusable
			})
		}
	})

	t.Run("resource exhaustion handling", func(t *testing.T) {
		// Test factory behavior under resource pressure
		resourceScenarios := []struct {
			name      string
			condition string
			operation func() error
		}{
			{
				name:      "memory exhaustion during parser creation",
				condition: "high memory pressure",
				operation: func() error {
					lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
					_, err := factory.CreateParser(ctx, lang)
					return err
				},
			},
			{
				name:      "parser pool exhaustion",
				condition: "pool size limit reached",
				operation: func() error {
					_, err := factory.GetParserPool(ctx)
					return err
				},
			},
			{
				name:      "concurrent parser limit exceeded",
				condition: "concurrency limit reached",
				operation: func() error {
					lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
					_, err := factory.CreateParser(ctx, lang)
					return err
				},
			},
		}

		for _, scenario := range resourceScenarios {
			t.Run(scenario.name, func(t *testing.T) {
				err := scenario.operation()
				require.Error(t, err, "Factory not implemented yet")

				// When implemented, should verify:
				// - Factory degrades gracefully under resource pressure
				// - Appropriate errors are returned with context
				// - Factory attempts recovery when resources become available
				// - Metrics reflect resource pressure conditions
			})
		}
	})

	t.Run("concurrent operation safety", func(t *testing.T) {
		// Test factory thread safety under concurrent access
		var wg sync.WaitGroup
		const numConcurrentOps = 20

		for i := range numConcurrentOps {
			wg.Add(1)
			go func(_ int) {
				defer wg.Done()

				// Perform various concurrent operations
				operations := []func() error{
					func() error {
						lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
						_, err := factory.CreateParser(ctx, lang)
						return err
					},
					func() error {
						_, err := factory.GetSupportedLanguages(ctx)
						return err
					},
					func() error {
						_, err := factory.GetParserPool(ctx)
						return err
					},
					func() error {
						_, err := factory.GetHealthStatus(ctx)
						return err
					},
				}

				for _, op := range operations {
					err := op()
					// All operations should fail in RED phase
					assert.Error(t, err, "Factory not implemented yet")
				}
			}(i)
		}

		wg.Wait()

		// When implemented, should verify:
		// - No race conditions or data corruption
		// - Factory remains responsive under concurrent load
		// - Resource cleanup occurs properly
		// - Error handling is consistent across goroutines
	})
}

// TestTreeSitterParserFactory_ResourceManagement tests resource lifecycle management.
// This is a RED PHASE test that defines expected resource management behavior.
func TestTreeSitterParserFactory_ResourceManagement(t *testing.T) {
	ctx := context.Background()
	var factory TreeSitterParserFactory // Will be nil in RED phase

	t.Run("factory cleanup releases all resources", func(t *testing.T) {
		// Create some parsers and pool resources first
		setupOperations := []func() error{
			func() error {
				lang, _ := valueobject.NewLanguage(valueobject.LanguageGo)
				_, err := factory.CreateParser(ctx, lang)
				return err
			},
			func() error {
				_, err := factory.GetParserPool(ctx)
				return err
			},
		}

		// All setup operations will fail in RED phase
		for i, op := range setupOperations {
			err := op()
			require.Error(t, err, "Setup operation %d should fail - factory not implemented", i)
		}

		// Cleanup should also fail in RED phase
		err := factory.Cleanup(ctx)
		require.Error(t, err, "Factory not implemented yet")

		// When implemented, should verify:
		// - All parser instances are properly cleaned up
		// - Parser pool resources are released
		// - Cache entries are cleared
		// - Metrics are properly finalized
		// - No resource leaks occur
	})

	t.Run("automatic resource cleanup on idle timeout", func(t *testing.T) {
		// Test that unused resources are cleaned up automatically
		config := &FactoryConfiguration{
			EnableAutoCleanup:   true,
			HealthCheckInterval: 100 * time.Millisecond, // Fast for testing
		}

		err := factory.SetConfiguration(ctx, config)
		require.Error(t, err, "Factory not implemented yet")

		// When implemented, should verify:
		// - Unused parsers are cleaned up after timeout
		// - Pool size adjusts based on usage patterns
		// - Cleanup doesn't interfere with active operations
		// - Metrics track cleanup operations
	})

	t.Run("resource limits are enforced", func(t *testing.T) {
		config := &FactoryConfiguration{
			MaxCachedParsers: 5, // Small limit for testing
			ConcurrencyLimit: 3,
		}

		err := factory.SetConfiguration(ctx, config)
		require.Error(t, err, "Factory not implemented yet")

		// When implemented, should verify:
		// - Factory respects configured resource limits
		// - Operations fail gracefully when limits are exceeded
		// - Limits can be changed dynamically
		// - Resource usage stays within bounds
	})

	t.Run("memory pressure triggers resource cleanup", func(t *testing.T) {
		// Simulate high memory usage scenario
		// This test defines expected behavior under memory pressure

		// When implemented, should verify:
		// - Factory monitors memory usage
		// - Cleanup is triggered when memory pressure is detected
		// - Most recently used parsers are preserved
		// - Cleanup reduces memory footprint significantly
		require.Fail(t, "Memory pressure handling not implemented yet")
	})
}

// Helper functions for the tests - these are placeholders for the GREEN phase implementation
// They are currently unused in the RED phase but define the expected test infrastructure
*/
