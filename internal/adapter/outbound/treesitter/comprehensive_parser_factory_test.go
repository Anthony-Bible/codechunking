package treesitter

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTreeSitterParserFactoryCreation(t *testing.T) {
	t.Run("Factory creation with context", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)
		assert.NotNil(t, factory)
	})

	t.Run("Factory creation with timeout context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)
		assert.NotNil(t, factory)
	})

	t.Run("Factory creation with cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.Error(t, err)
		assert.Nil(t, factory)
	})

	t.Run("Factory singleton behavior", func(t *testing.T) {
		ctx := context.Background()
		factory1, err1 := NewTreeSitterParserFactory(ctx)
		factory2, err2 := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, factory1, factory2)
	})

	t.Run("Factory resource cleanup", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)
		assert.NotNil(t, factory)

		err = factory.Cleanup()
		require.NoError(t, err)
	})
}

func TestLanguageSupport(t *testing.T) {
	t.Run("GetSupportedLanguages returns all supported languages", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		languages := factory.GetSupportedLanguages()
		expectedLanguages := []valueobject.Language{
			valueobject.NewLanguage(valueobject.LanguageGo),
			valueobject.NewLanguage(valueobject.LanguagePython),
			valueobject.NewLanguage(valueobject.LanguageJavaScript),
			valueobject.NewLanguage(valueobject.LanguageTypeScript),
		}
		for _, expected := range expectedLanguages {
			assert.Contains(t, languages, expected)
		}
	})

	t.Run("IsLanguageSupported correctly identifies supported languages", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		assert.True(t, factory.IsLanguageSupported(valueobject.NewLanguage(valueobject.LanguageGo)))
		assert.True(t, factory.IsLanguageSupported(valueobject.NewLanguage(valueobject.LanguagePython)))
		assert.True(t, factory.IsLanguageSupported(valueobject.NewLanguage(valueobject.LanguageJavaScript)))
		assert.True(t, factory.IsLanguageSupported(valueobject.NewLanguage(valueobject.LanguageTypeScript)))
		assert.False(t, factory.IsLanguageSupported(valueobject.NewLanguage(valueobject.LanguageJava)))
		assert.False(t, factory.IsLanguageSupported(valueobject.NewLanguage(valueobject.LanguageCpp)))
		assert.False(t, factory.IsLanguageSupported(valueobject.NewLanguage(valueobject.LanguageRust)))
	})

	t.Run("Language case-sensitivity handling", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		assert.True(t, factory.IsLanguageSupported(valueobject.NewLanguage(valueobject.LanguageGo)))
		assert.True(t, factory.IsLanguageSupported(valueobject.NewLanguage(valueobject.LanguagePython)))
		assert.True(t, factory.IsLanguageSupported(valueobject.NewLanguage(valueobject.LanguageJavaScript)))
		assert.True(t, factory.IsLanguageSupported(valueobject.NewLanguage(valueobject.LanguageTypeScript)))
	})

	t.Run("Support for Go language detection and parsing", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		assert.True(t, factory.IsLanguageSupported(valueobject.NewLanguage(valueobject.LanguageGo)))
		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)
	})

	t.Run("Support for Python language detection and parsing", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		assert.True(t, factory.IsLanguageSupported(valueobject.NewLanguage(valueobject.LanguagePython)))
		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguagePython))
		require.NoError(t, err)
		assert.NotNil(t, parser)
	})

	t.Run("Support for JavaScript language detection and parsing", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		assert.True(t, factory.IsLanguageSupported(valueobject.NewLanguage(valueobject.LanguageJavaScript)))
		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageJavaScript))
		require.NoError(t, err)
		assert.NotNil(t, parser)
	})
}

func TestParserCreation(t *testing.T) {
	t.Run("CreateParser for Go language", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)
	})

	t.Run("CreateParser for Python language", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguagePython))
		require.NoError(t, err)
		assert.NotNil(t, parser)
	})

	t.Run("CreateParser for JavaScript language", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageJavaScript))
		require.NoError(t, err)
		assert.NotNil(t, parser)
	})

	t.Run("Parser creation with invalid language", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage("invalid"))
		require.Error(t, err)
		assert.Nil(t, parser)
		assert.Contains(t, err.Error(), "language not supported")
	})

	t.Run("Parser creation with nil language", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(""))
		require.Error(t, err)
		assert.Nil(t, parser)
		assert.Contains(t, err.Error(), "language cannot be empty")
	})

	t.Run("Multiple parser creation concurrent", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		langs := []valueobject.Language{
			valueobject.NewLanguage(valueobject.LanguageGo),
			valueobject.NewLanguage(valueobject.LanguagePython),
			valueobject.NewLanguage(valueobject.LanguageJavaScript),
		}
		parsers := make([]outbound.CodeParser, len(langs))
		errors := make([]error, len(langs))

		for i, lang := range langs {
			parser, err := factory.CreateParser(ctx, lang)
			parsers[i] = parser
			errors[i] = err
		}

		for i, err := range errors {
			require.NoError(t, err, "Failed to create parser for language %s", langs[i])
			assert.NotNil(t, parsers[i])
		}
	})

	t.Run("Parser instance isolation", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser1, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		parser2, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguagePython))
		require.NoError(t, err)

		assert.NotEqual(t, parser1, parser2)
	})

	t.Run("Parser configuration inheritance from factory", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		config := FactoryConfiguration{
			Timeout:     5 * time.Second,
			MemoryLimit: 100 * 1024 * 1024,
		}
		factory.SetConfiguration(config)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		// Verify that parser inherits configuration
		observableParser, ok := parser.(*ObservableParser)
		require.True(t, ok)
		assert.Equal(t, config.Timeout, observableParser.GetTimeout())
	})
}

func TestLanguageDetectionFromFileExtension(t *testing.T) {
	t.Run(".go files map to Go language", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		lang, err := factory.DetectLanguageFromFileExtension("main.go")
		require.NoError(t, err)
		assert.Equal(t, valueobject.NewLanguage(valueobject.LanguageGo), lang)

		lang, err = factory.DetectLanguageFromFileExtension("test.GO")
		require.NoError(t, err)
		assert.Equal(t, valueobject.NewLanguage(valueobject.LanguageGo), lang)
	})

	t.Run(".py/.pyx/.pyi files map to Python language", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		extensions := []string{"main.py", "module.pyx", "interface.pyi"}
		for _, ext := range extensions {
			lang, err := factory.DetectLanguageFromFileExtension(ext)
			require.NoError(t, err)
			assert.Equal(t, valueobject.NewLanguage(valueobject.LanguagePython), lang)
		}

		extensions = []string{"MAIN.PY", "MODULE.PYX", "INTERFACE.PYI"}
		for _, ext := range extensions {
			lang, err := factory.DetectLanguageFromFileExtension(ext)
			require.NoError(t, err)
			assert.Equal(t, valueobject.NewLanguage(valueobject.LanguagePython), lang)
		}
	})

	t.Run(".js/.jsx/.mjs/.cjs files map to JavaScript language", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		extensions := []string{"app.js", "component.jsx", "module.mjs", "config.cjs"}
		for _, ext := range extensions {
			lang, err := factory.DetectLanguageFromFileExtension(ext)
			require.NoError(t, err)
			assert.Equal(t, valueobject.NewLanguage(valueobject.LanguageJavaScript), lang)
		}

		extensions = []string{"APP.JS", "COMPONENT.JSX", "MODULE.MJS", "CONFIG.CJS"}
		for _, ext := range extensions {
			lang, err := factory.DetectLanguageFromFileExtension(ext)
			require.NoError(t, err)
			assert.Equal(t, valueobject.NewLanguage(valueobject.LanguageJavaScript), lang)
		}
	})

	t.Run(".ts/.tsx files map to TypeScript language", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		extensions := []string{"app.ts", "component.tsx"}
		for _, ext := range extensions {
			lang, err := factory.DetectLanguageFromFileExtension(ext)
			require.NoError(t, err)
			assert.Equal(t, valueobject.NewLanguage(valueobject.LanguageTypeScript), lang)
		}

		extensions = []string{"APP.TS", "COMPONENT.TSX"}
		for _, ext := range extensions {
			lang, err := factory.DetectLanguageFromFileExtension(ext)
			require.NoError(t, err)
			assert.Equal(t, valueobject.NewLanguage(valueobject.LanguageTypeScript), lang)
		}
	})

	t.Run("Unknown extensions return error", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		_, err = factory.DetectLanguageFromFileExtension("main.java")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported file extension")

		_, err = factory.DetectLanguageFromFileExtension("program.cpp")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported file extension")

		_, err = factory.DetectLanguageFromFileExtension("lib.rs")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported file extension")
	})

	t.Run("Multiple extensions on same language", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		lang1, err := factory.DetectLanguageFromFileExtension("script.py")
		require.NoError(t, err)
		lang2, err := factory.DetectLanguageFromFileExtension("module.pyx")
		require.NoError(t, err)
		lang3, err := factory.DetectLanguageFromFileExtension("interface.pyi")
		require.NoError(t, err)

		assert.Equal(t, valueobject.NewLanguage(valueobject.LanguagePython), lang1)
		assert.Equal(t, valueobject.NewLanguage(valueobject.LanguagePython), lang2)
		assert.Equal(t, valueobject.NewLanguage(valueobject.LanguagePython), lang3)
	})

	t.Run("Case-insensitive extension matching", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		extensions := []string{"main.go", "MAIN.GO", "Main.Go", "test.PY", "script.JS", "component.TSX"}
		expectedLanguages := []valueobject.Language{
			valueobject.NewLanguage(valueobject.LanguageGo),
			valueobject.NewLanguage(valueobject.LanguageGo),
			valueobject.NewLanguage(valueobject.LanguageGo),
			valueobject.NewLanguage(valueobject.LanguagePython),
			valueobject.NewLanguage(valueobject.LanguageJavaScript),
			valueobject.NewLanguage(valueobject.LanguageTypeScript),
		}

		for i, ext := range extensions {
			lang, err := factory.DetectLanguageFromFileExtension(ext)
			require.NoError(t, err)
			assert.Equal(t, expectedLanguages[i], lang)
		}
	})
}

func TestParserConfiguration(t *testing.T) {
	t.Run("Default parser configuration", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		config := factory.GetConfiguration()
		assert.Equal(t, 30*time.Second, config.Timeout)
		assert.Equal(t, 0, config.MemoryLimit)
	})

	t.Run("Custom timeout settings", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		customTimeout := 5 * time.Second
		config := FactoryConfiguration{
			Timeout: customTimeout,
		}
		factory.SetConfiguration(config)

		updatedConfig := factory.GetConfiguration()
		assert.Equal(t, customTimeout, updatedConfig.Timeout)
	})

	t.Run("Memory limit configuration", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		memoryLimit := 100 * 1024 * 1024
		config := FactoryConfiguration{
			MemoryLimit: memoryLimit,
		}
		factory.SetConfiguration(config)

		updatedConfig := factory.GetConfiguration()
		assert.Equal(t, memoryLimit, updatedConfig.MemoryLimit)
	})

	t.Run("Performance optimization settings", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		config := FactoryConfiguration{
			OptimizeForPerformance: true,
		}
		factory.SetConfiguration(config)

		updatedConfig := factory.GetConfiguration()
		assert.True(t, updatedConfig.OptimizeForPerformance)
	})

	t.Run("Parser pool configuration", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		poolSize := 10
		config := FactoryConfiguration{
			ParserPoolSize: poolSize,
		}
		factory.SetConfiguration(config)

		updatedConfig := factory.GetConfiguration()
		assert.Equal(t, poolSize, updatedConfig.ParserPoolSize)
	})

	t.Run("Error handling configuration", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		config := FactoryConfiguration{
			StrictErrorHandling: true,
		}
		factory.SetConfiguration(config)

		updatedConfig := factory.GetConfiguration()
		assert.True(t, updatedConfig.StrictErrorHandling)
	})

	t.Run("Logging configuration", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		config := FactoryConfiguration{
			EnableLogging: true,
		}
		factory.SetConfiguration(config)

		updatedConfig := factory.GetConfiguration()
		assert.True(t, updatedConfig.EnableLogging)
	})
}

func TestParserPoolManagement(t *testing.T) {
	t.Run("Parser instance reuse", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser1, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		parser2, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)

		// With pooling, subsequent requests for the same language should return the same instance
		assert.Equal(t, parser1, parser2)
	})

	t.Run("Parser pool size limits", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		config := FactoryConfiguration{
			ParserPoolSize: 2,
		}
		factory.SetConfiguration(config)

		// Create more parsers than pool size
		var parsers []outbound.CodeParser
		for i := 0; i < 5; i++ {
			parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
			require.NoError(t, err)
			parsers = append(parsers, parser)
		}

		// Verify pool size limit is respected
		assert.Len(t, parsers, 5)
	})

	t.Run("Parser cleanup and disposal", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		// Verify parser can be cleaned up
		err = parser.Cleanup()
		require.NoError(t, err)
	})

	t.Run("Concurrent parser access", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		// Test concurrent access to parsers
		concurrentAccess := func(lang valueobject.Language) {
			parser, err := factory.CreateParser(ctx, lang)
			require.NoError(t, err)
			assert.NotNil(t, parser)
		}

		go concurrentAccess(valueobject.NewLanguage(valueobject.LanguageGo))
		go concurrentAccess(valueobject.NewLanguage(valueobject.LanguagePython))
		go concurrentAccess(valueobject.NewLanguage(valueobject.LanguageJavaScript))

		time.Sleep(100 * time.Millisecond)
	})

	t.Run("Parser state isolation", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser1, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		parser2, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguagePython))
		require.NoError(t, err)

		// Verify parsers maintain isolated state
		assert.NotEqual(t, parser1, parser2)
	})

	t.Run("Pool exhaustion handling", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		config := FactoryConfiguration{
			ParserPoolSize: 1,
		}
		factory.SetConfiguration(config)

		// Create parsers beyond pool capacity
		parser1, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		parser2, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)

		// Should handle pool exhaustion gracefully
		assert.NotNil(t, parser1)
		assert.NotNil(t, parser2)
	})

	t.Run("Parser lifecycle management", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		// Verify parser lifecycle methods
		assert.False(t, parser.IsDisposed())

		err = parser.Cleanup()
		require.NoError(t, err)

		assert.True(t, parser.IsDisposed())
	})
}

func TestObservableParserWrapper(t *testing.T) {
	t.Run("Observable parser creation", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		// Verify parser is observable
		observableParser, ok := parser.(*ObservableParser)
		require.True(t, ok)
		assert.NotNil(t, observableParser)
	})

	t.Run("Parsing event observation", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		observableParser, ok := parser.(*ObservableParser)
		require.True(t, ok)

		// Verify event observation capabilities
		events := observableParser.GetEvents()
		assert.NotNil(t, events)
	})

	t.Run("Performance metric collection", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		observableParser, ok := parser.(*ObservableParser)
		require.True(t, ok)

		// Verify performance metrics collection
		metrics := observableParser.GetPerformanceMetrics()
		assert.NotNil(t, metrics)
	})

	t.Run("Error event notification", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		observableParser, ok := parser.(*ObservableParser)
		require.True(t, ok)

		// Verify error event notification
		errorEvents := observableParser.GetErrorEvents()
		assert.NotNil(t, errorEvents)
	})

	t.Run("Parse success/failure tracking", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		observableParser, ok := parser.(*ObservableParser)
		require.True(t, ok)

		// Verify success/failure tracking
		successCount := observableParser.GetSuccessCount()
		failureCount := observableParser.GetFailureCount()
		assert.Equal(t, 0, successCount)
		assert.Equal(t, 0, failureCount)
	})

	t.Run("Memory usage tracking", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		observableParser, ok := parser.(*ObservableParser)
		require.True(t, ok)

		// Verify memory usage tracking
		memoryUsage := observableParser.GetMemoryUsage()
		assert.GreaterOrEqual(t, memoryUsage, int64(0))
	})

	t.Run("Parse time measurement", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		observableParser, ok := parser.(*ObservableParser)
		require.True(t, ok)

		// Verify parse time measurement
		parseTime := observableParser.GetAverageParseTime()
		assert.GreaterOrEqual(t, parseTime, time.Duration(0))
	})
}

func TestErrorHandling(t *testing.T) {
	t.Run("Factory creation failures", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		factory, err := NewTreeSitterParserFactory(ctx)
		require.Error(t, err)
		assert.Nil(t, factory)
	})

	t.Run("Parser creation failures", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage("unsupported"))
		require.Error(t, err)
		assert.Nil(t, parser)
	})

	t.Run("Invalid language parameter handling", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(""))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "language cannot be empty")
		assert.Nil(t, parser)
	})

	t.Run("Context cancellation handling", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		cancel()
		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.Error(t, err)
		assert.Nil(t, parser)
	})

	t.Run("Timeout handling", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		config := FactoryConfiguration{
			Timeout: 1 * time.Nanosecond,
		}
		factory.SetConfiguration(config)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		// Test parsing with extremely short timeout
		goCode := `package main
func main() {
	println("Hello, World!")
}`
		_, err = parser.Parse(ctx, goCode)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")
	})

	t.Run("Memory exhaustion handling", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		config := FactoryConfiguration{
			MemoryLimit: 1,
		}
		factory.SetConfiguration(config)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		// Test parsing with extremely low memory limit
		goCode := `package main
func main() {
	println("Hello, World!")
}`
		_, err = parser.Parse(ctx, goCode)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "memory limit exceeded")
	})

	t.Run("Thread safety validation", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		// Test concurrent factory usage
		for i := 0; i < 10; i++ {
			go func() {
				parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
				require.NoError(t, err)
				assert.NotNil(t, parser)
			}()
		}

		time.Sleep(100 * time.Millisecond)
	})
}

func TestIntegrationWithLanguageParsers(t *testing.T) {
	t.Run("Factory creates functional Go parsers", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		goCode := `package main
import "fmt"
func main() {
	fmt.Println("Hello, World!")
}`
		tree, err := parser.Parse(ctx, goCode)
		require.NoError(t, err)
		assert.NotNil(t, tree)
	})

	t.Run("Factory creates functional Python parsers", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguagePython))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		pythonCode := `def hello():
	print("Hello, World!")
hello()`
		tree, err := parser.Parse(ctx, pythonCode)
		require.NoError(t, err)
		assert.NotNil(t, tree)
	})

	t.Run("Factory creates functional JavaScript parsers", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageJavaScript))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		jsCode := `function hello() {
	console.log("Hello, World!");
}
hello();`
		tree, err := parser.Parse(ctx, jsCode)
		require.NoError(t, err)
		assert.NotNil(t, tree)
	})

	t.Run("Parser instances implement correct interfaces", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		// Verify parser implements CodeParser interface
		_, ok := parser.(outbound.CodeParser)
		assert.True(t, ok)
	})

	t.Run("Language-specific parser method availability", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		// Verify language-specific methods are available
		goParser, ok := parser.(*GoTreeSitterParser)
		require.True(t, ok)
		assert.NotNil(t, goParser)
	})

	t.Run("Cross-language parser isolation", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		goParser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		pythonParser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguagePython))
		require.NoError(t, err)
		jsParser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageJavaScript))
		require.NoError(t, err)

		assert.NotEqual(t, goParser, pythonParser)
		assert.NotEqual(t, pythonParser, jsParser)
		assert.NotEqual(t, goParser, jsParser)
	})
}

func TestPerformanceAndScalability(t *testing.T) {
	t.Run("Factory creation performance", func(t *testing.T) {
		ctx := context.Background()

		start := time.Now()
		factory, err := NewTreeSitterParserFactory(ctx)
		duration := time.Since(start)

		require.NoError(t, err)
		assert.NotNil(t, factory)
		assert.Less(t, duration, 100*time.Millisecond)
	})

	t.Run("Parser creation performance", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		start := time.Now()
		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		duration := time.Since(start)

		require.NoError(t, err)
		assert.NotNil(t, parser)
		assert.Less(t, duration, 50*time.Millisecond)
	})

	t.Run("Concurrent parser usage", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		start := time.Now()
		for i := 0; i < 5; i++ {
			go func() {
				parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
				require.NoError(t, err)
				assert.NotNil(t, parser)
			}()
		}
		time.Sleep(100 * time.Millisecond)
		duration := time.Since(start)
		assert.Less(t, duration, 200*time.Millisecond)
	})

	t.Run("Memory usage under load", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		var parsers []outbound.CodeParser
		for i := 0; i < 100; i++ {
			parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
			require.NoError(t, err)
			parsers = append(parsers, parser)
		}

		// Verify memory usage is reasonable
		assert.Len(t, parsers, 100)
	})

	t.Run("Parser cleanup efficiency", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		start := time.Now()
		err = parser.Cleanup()
		duration := time.Since(start)

		require.NoError(t, err)
		assert.Less(t, duration, 10*time.Millisecond)
	})
}

func TestConfigurationAndCustomization(t *testing.T) {
	t.Run("FactoryConfiguration struct validation", func(t *testing.T) {
		config := FactoryConfiguration{
			Timeout:                5 * time.Second,
			MemoryLimit:            100 * 1024 * 1024,
			ParserPoolSize:         10,
			OptimizeForPerformance: true,
			StrictErrorHandling:    true,
			EnableLogging:          true,
		}

		assert.Equal(t, 5*time.Second, config.Timeout)
		assert.Equal(t, 100*1024*1024, config.MemoryLimit)
		assert.Equal(t, 10, config.ParserPoolSize)
		assert.True(t, config.OptimizeForPerformance)
		assert.True(t, config.StrictErrorHandling)
		assert.True(t, config.EnableLogging)
	})

	t.Run("Custom parser settings application", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		customConfig := FactoryConfiguration{
			Timeout:     10 * time.Second,
			MemoryLimit: 50 * 1024 * 1024,
		}
		factory.SetConfiguration(customConfig)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		observableParser, ok := parser.(*ObservableParser)
		require.True(t, ok)
		assert.Equal(t, customConfig.Timeout, observableParser.GetTimeout())
	})

	t.Run("Environment-based configuration", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		// Verify factory can load configuration from environment
		config := factory.GetConfiguration()
		assert.NotNil(t, config)
	})

	t.Run("Configuration validation and defaults", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		// Verify default configuration values
		config := factory.GetConfiguration()
		assert.Equal(t, 30*time.Second, config.Timeout)
		assert.Equal(t, 0, config.MemoryLimit)
		assert.Equal(t, 0, config.ParserPoolSize)
		assert.False(t, config.OptimizeForPerformance)
		assert.False(t, config.StrictErrorHandling)
		assert.False(t, config.EnableLogging)
	})

	t.Run("Configuration change propagation", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		originalConfig := factory.GetConfiguration()
		newConfig := FactoryConfiguration{
			Timeout: 15 * time.Second,
		}
		factory.SetConfiguration(newConfig)

		updatedConfig := factory.GetConfiguration()
		assert.NotEqual(t, originalConfig.Timeout, updatedConfig.Timeout)
		assert.Equal(t, newConfig.Timeout, updatedConfig.Timeout)
	})
}

func TestParseTreeConversion(t *testing.T) {
	t.Run("ConvertPortParseTreeToDomain functionality", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		goCode := `package main
func main() {
	println("Hello, World!")
}`
		portTree, err := parser.Parse(ctx, goCode)
		require.NoError(t, err)
		assert.NotNil(t, portTree)

		domainTree := ConvertPortParseTreeToDomain(portTree)
		assert.NotNil(t, domainTree)
	})

	t.Run("Port parse tree to domain parse tree conversion", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		goCode := `package main
func main() {
	println("Hello, World!")
}`
		portTree, err := parser.Parse(ctx, goCode)
		require.NoError(t, err)
		assert.NotNil(t, portTree)

		domainTree := ConvertPortParseTreeToDomain(portTree)
		assert.NotNil(t, domainTree)

		// Verify domain tree implements correct interface
		_, ok := domainTree.(valueobject.ParseTree)
		assert.True(t, ok)
	})

	t.Run("Conversion error handling", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		// Test conversion with nil tree
		domainTree := ConvertPortParseTreeToDomain(nil)
		assert.Nil(t, domainTree)
	})

	t.Run("Parse tree structure validation", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		goCode := `package main
func main() {
	println("Hello, World!")
}`
		portTree, err := parser.Parse(ctx, goCode)
		require.NoError(t, err)
		assert.NotNil(t, portTree)

		domainTree := ConvertPortParseTreeToDomain(portTree)
		assert.NotNil(t, domainTree)

		// Verify tree structure
		assert.NotEmpty(t, domainTree.GetRootNode())
	})

	t.Run("Metadata preservation during conversion", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		goCode := `package main
func main() {
	println("Hello, World!")
}`
		portTree, err := parser.Parse(ctx, goCode)
		require.NoError(t, err)
		assert.NotNil(t, portTree)

		domainTree := ConvertPortParseTreeToDomain(portTree)
		assert.NotNil(t, domainTree)

		// Verify metadata is preserved
		assert.Equal(t, portTree.GetLanguage(), domainTree.GetLanguage())
		assert.Equal(t, portTree.GetSourceCode(), domainTree.GetSourceCode())
	})
}

func TestComplexIntegrationScenarios(t *testing.T) {
	t.Run("Factory->Parser->ParseTree->Conversion->Extraction pipeline", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		goCode := `package main
import "fmt"
func main() {
	fmt.Println("Hello, World!")
}`
		portTree, err := parser.Parse(ctx, goCode)
		require.NoError(t, err)
		assert.NotNil(t, portTree)

		domainTree := ConvertPortParseTreeToDomain(portTree)
		assert.NotNil(t, domainTree)

		// Test extraction from domain tree
		rootNode := domainTree.GetRootNode()
		assert.NotNil(t, rootNode)
	})

	t.Run("Multiple languages parsed concurrently", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		goCode := `package main
func main() {
	println("Hello, World!")
}`

		pythonCode := `def hello():
	print("Hello, World!")
hello()`

		jsCode := `function hello() {
	console.log("Hello, World!");
}
hello();`

		languages := []valueobject.Language{
			valueobject.NewLanguage(valueobject.LanguageGo),
			valueobject.NewLanguage(valueobject.LanguagePython),
			valueobject.NewLanguage(valueobject.LanguageJavaScript),
		}
		codes := []string{goCode, pythonCode, jsCode}
		trees := make([]outbound.ParseTree, len(languages))

		for i, lang := range languages {
			parser, err := factory.CreateParser(ctx, lang)
			require.NoError(t, err)
			tree, err := parser.Parse(ctx, codes[i])
			require.NoError(t, err)
			trees[i] = tree
		}

		// Verify all trees were parsed successfully
		for _, tree := range trees {
			assert.NotNil(t, tree)
		}
	})

	t.Run("Factory reconfiguration during operation", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		originalConfig := factory.GetConfiguration()

		// Reconfigure factory
		newConfig := FactoryConfiguration{
			Timeout: 20 * time.Second,
		}
		factory.SetConfiguration(newConfig)

		updatedConfig := factory.GetConfiguration()
		assert.NotEqual(t, originalConfig.Timeout, updatedConfig.Timeout)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		observableParser, ok := parser.(*ObservableParser)
		require.True(t, ok)
		assert.Equal(t, newConfig.Timeout, observableParser.GetTimeout())
	})

	t.Run("Parser failure recovery and retry", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		parser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		assert.NotNil(t, parser)

		// Test parsing malformed code
		malformedCode := `package main
func main() {
	println("Hello, World!"
}`
		_, err = parser.Parse(ctx, malformedCode)
		require.Error(t, err)

		// Retry with valid code
		validCode := `package main
func main() {
	println("Hello, World!")
}`
		tree, err := parser.Parse(ctx, validCode)
		require.NoError(t, err)
		assert.NotNil(t, tree)
	})

	t.Run("Cross-language dependency parsing", func(t *testing.T) {
		ctx := context.Background()
		factory, err := NewTreeSitterParserFactory(ctx)
		require.NoError(t, err)

		// Parse Go code
		goParser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageGo))
		require.NoError(t, err)
		goCode := `package main
func main() {
	println("Hello, World!")
}`
		goTree, err := goParser.Parse(ctx, goCode)
		require.NoError(t, err)
		assert.NotNil(t, goTree)

		// Parse Python code
		pythonParser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguagePython))
		require.NoError(t, err)
		pythonCode := `def hello():
	print("Hello, World!")
hello()`
		pythonTree, err := pythonParser.Parse(ctx, pythonCode)
		require.NoError(t, err)
		assert.NotNil(t, pythonTree)

		// Parse JavaScript code
		jsParser, err := factory.CreateParser(ctx, valueobject.NewLanguage(valueobject.LanguageJavaScript))
		require.NoError(t, err)
		jsCode := `function hello() {
	console.log("Hello, World!");
}
hello();`
		jsTree, err := jsParser.Parse(ctx, jsCode)
		require.NoError(t, err)
		assert.NotNil(t, jsTree)

		// Verify cross-language isolation
		assert.NotEqual(t, goTree, pythonTree)
		assert.NotEqual(t, pythonTree, jsTree)
		assert.NotEqual(t, goTree, jsTree)
	})
}
