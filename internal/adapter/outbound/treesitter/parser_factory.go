package treesitter

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	forest "github.com/alexaandru/go-sitter-forest"
)

// ParserFactoryImpl implements TreeSitterParserFactory using go-sitter-forest.
type ParserFactoryImpl struct {
	config         *FactoryConfiguration
	parserPool     *ParserPool
	supportedLangs []valueobject.Language
	mu             sync.RWMutex
	startTime      time.Time
}

// NewTreeSitterParserFactory creates a new TreeSitterParserFactory.
func NewTreeSitterParserFactory(ctx context.Context) (*ParserFactoryImpl, error) {
	config := &FactoryConfiguration{
		MaxCachedParsers:    DefaultMaxCachedParsers,
		ParserTimeout:       DefaultParserTimeout,
		EnableMetrics:       true,
		EnableTracing:       false,
		ConcurrencyLimit:    DefaultConcurrencyLimit,
		CacheEvictionPolicy: DefaultCacheEvictionPolicy,
		EnableAutoCleanup:   true,
		HealthCheckInterval: DefaultHealthCheckInterval,
		DefaultParserConfig: ParserConfiguration{
			DefaultTimeout:   DefaultDefaultTimeout,
			MaxMemory:        DefaultMaxMemory,
			ConcurrencyLimit: DefaultWorkerCount,
			CacheEnabled:     true,
			CacheSize:        DefaultCacheSize,
			MetricsEnabled:   true,
			TracingEnabled:   false,
		},
	}

	parserPool := &ParserPool{
		MaxSize:          DefaultMaxCachedParsers,
		CurrentSize:      0,
		AvailableParsers: 0,
		InUseParsers:     0,
		Languages:        make([]string, 0),
		CreatedAt:        time.Now(),
		LastAccessed:     time.Now(),
		Configuration:    config,
		PoolStatistics: &ParserPoolStatistics{
			TotalCreated:        0,
			TotalDestroyed:      0,
			TotalCheckouts:      0,
			TotalReturns:        0,
			AverageCheckoutTime: 0,
			PeakUsage:           0,
			CacheHitRate:        0.0,
			ErrorRate:           0.0,
		},
	}

	// Initialize supported languages
	supportedLangs := initializeSupportedLanguages()

	factory := &ParserFactoryImpl{
		config:         config,
		parserPool:     parserPool,
		supportedLangs: supportedLangs,
		startTime:      time.Now(),
	}

	slogger.Info(ctx, "TreeSitterParserFactory initialized", slogger.Fields{
		"supported_languages": len(supportedLangs),
		"max_cached_parsers":  config.MaxCachedParsers,
		"concurrency_limit":   config.ConcurrencyLimit,
	})

	return factory, nil
}

// initializeSupportedLanguages creates a list of supported languages based on go-sitter-forest.
func initializeSupportedLanguages() []valueobject.Language {
	var languages []valueobject.Language

	// Core supported languages
	coreLanguages := []string{
		valueobject.LanguageGo,
		valueobject.LanguagePython,
		valueobject.LanguageJavaScript,
		valueobject.LanguageTypeScript,
		valueobject.LanguageCPlusPlus,
		valueobject.LanguageRust,
		"java",
		"c",
		"cpp",
		"c_sharp",
		"ruby",
		"php",
		"kotlin",
		"swift",
		"scala",
		"html",
		"css",
		"json",
		"yaml",
		"xml",
		"sql",
		"bash",
		"dockerfile",
		"lua",
		"perl",
		"elixir",
		"erlang",
		"haskell",
		"clojure",
		"ocaml",
	}

	for _, langName := range coreLanguages {
		if lang, err := valueobject.NewLanguage(langName); err == nil {
			languages = append(languages, lang)
		}
	}

	return languages
}

// CreateParser creates a parser for a specific language using forest.GetLanguage.
func (f *ParserFactoryImpl) CreateParser(
	ctx context.Context,
	language valueobject.Language,
) (ObservableTreeSitterParser, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Validate language support
	if err := f.ValidateLanguageSupport(context.Background(), language); err != nil {
		return nil, fmt.Errorf("language not supported: %w", err)
	}

	// Try to get language grammar from forest
	langName := mapDomainLanguageToForest(language.Name())
	grammar := forest.GetLanguage(langName)
	if grammar == nil {
		return nil, fmt.Errorf("language grammar not found for %s", langName)
	}

	// Create observable parser
	parser, err := NewObservableTreeSitterParser(ctx, language, grammar, f.config.DefaultParserConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	slogger.Info(ctx, "Parser created successfully", slogger.Fields{
		"language": language.Name(),
	})

	return parser, nil
}

// CreateGoParser creates a specialized Go parser.
func (f *ParserFactoryImpl) CreateGoParser(ctx context.Context) (ObservableTreeSitterParser, error) {
	goLang, err := valueobject.NewLanguage(valueobject.LanguageGo)
	if err != nil {
		return nil, fmt.Errorf("failed to create Go language: %w", err)
	}

	return f.CreateParser(ctx, goLang)
}

// CreatePythonParser creates a specialized Python parser.
func (f *ParserFactoryImpl) CreatePythonParser(ctx context.Context) (ObservableTreeSitterParser, error) {
	pythonLang, err := valueobject.NewLanguage(valueobject.LanguagePython)
	if err != nil {
		return nil, fmt.Errorf("failed to create Python language: %w", err)
	}

	return f.CreateParser(ctx, pythonLang)
}

// CreateJavaScriptParser creates a specialized JavaScript parser.
func (f *ParserFactoryImpl) CreateJavaScriptParser(ctx context.Context) (ObservableTreeSitterParser, error) {
	jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript)
	if err != nil {
		return nil, fmt.Errorf("failed to create JavaScript language: %w", err)
	}

	return f.CreateParser(ctx, jsLang)
}

// CreateTypeScriptParser creates a specialized TypeScript parser.
func (f *ParserFactoryImpl) CreateTypeScriptParser(ctx context.Context) (ObservableTreeSitterParser, error) {
	tsLang, err := valueobject.NewLanguage(valueobject.LanguageTypeScript)
	if err != nil {
		return nil, fmt.Errorf("failed to create TypeScript language: %w", err)
	}

	return f.CreateParser(ctx, tsLang)
}

// DetectAndCreateParser automatically detects language from file path and creates appropriate parser.
func (f *ParserFactoryImpl) DetectAndCreateParser(
	ctx context.Context,
	filePath string,
) (ObservableTreeSitterParser, valueobject.Language, error) {
	if filePath == "" {
		return nil, valueobject.Language{}, errors.New("empty file path")
	}

	// Use forest to detect language
	detectedLang := forest.DetectLanguage(filePath)
	if detectedLang == "" {
		return nil, valueobject.Language{}, errors.New("unsupported file extension")
	}

	// Map forest language name to domain language
	domainLangName := mapForestLanguageToDomain(detectedLang)
	if domainLangName == "" {
		return nil, valueobject.Language{}, errors.New("no file extension")
	}

	domainLang, err := valueobject.NewLanguage(domainLangName)
	if err != nil {
		return nil, valueobject.Language{}, fmt.Errorf("failed to create language %s: %w", domainLangName, err)
	}

	parser, err := f.CreateParser(ctx, domainLang)
	if err != nil {
		return nil, valueobject.Language{}, err
	}

	return parser, domainLang, nil
}

// GetSupportedLanguages returns all languages supported by the factory.
func (f *ParserFactoryImpl) GetSupportedLanguages(ctx context.Context) ([]valueobject.Language, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	slogger.Debug(ctx, "Getting supported languages", slogger.Fields{
		"count": len(f.supportedLangs),
	})

	return f.supportedLangs, nil
}

// ValidateLanguageSupport validates if a language is supported.
func (f *ParserFactoryImpl) ValidateLanguageSupport(
	_ context.Context,
	language valueobject.Language,
) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	langName := language.Name()

	// Check if language is in supported list
	for _, supported := range f.supportedLangs {
		if supported.Name() == langName {
			// Check if forest has the grammar
			forestName := mapDomainLanguageToForest(langName)
			grammar := forest.GetLanguage(forestName)
			if grammar != nil {
				return nil
			}
		}
	}

	return fmt.Errorf("unsupported language: %s", langName)
}

// SetConfiguration updates factory configuration.
func (f *ParserFactoryImpl) SetConfiguration(ctx context.Context, config *FactoryConfiguration) error {
	if config == nil {
		return errors.New("invalid configuration: configuration cannot be nil")
	}

	// Validate configuration
	if config.MaxCachedParsers < 0 {
		return errors.New("invalid configuration: max cached parsers cannot be negative")
	}
	if config.ParserTimeout < 0 {
		return errors.New("invalid configuration: parser timeout cannot be negative")
	}
	if config.ConcurrencyLimit <= 0 {
		return errors.New("invalid configuration: concurrency limit must be positive")
	}
	if config.HealthCheckInterval < 0 {
		return errors.New("invalid configuration: health check interval cannot be negative")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.config = config

	slogger.Info(ctx, "Factory configuration updated", slogger.Fields{
		"max_cached_parsers": config.MaxCachedParsers,
		"concurrency_limit":  config.ConcurrencyLimit,
		"metrics_enabled":    config.EnableMetrics,
	})

	return nil
}

// GetConfiguration returns current factory configuration.
func (f *ParserFactoryImpl) GetConfiguration(_ context.Context) (*FactoryConfiguration, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Return a copy of the configuration
	configCopy := *f.config
	return &configCopy, nil
}

// GetParserPool returns a managed pool of parser instances.
func (f *ParserFactoryImpl) GetParserPool(_ context.Context) (*ParserPool, error) {
	// Use write lock since we're updating LastAccessed time
	f.mu.Lock()
	defer f.mu.Unlock()

	// Update last accessed time atomically
	f.parserPool.LastAccessed = time.Now()

	// Return a copy of the pool information to prevent concurrent modifications
	poolCopy := *f.parserPool
	return &poolCopy, nil
}

// WarmupParsers preloads and caches parsers for specified languages.
func (f *ParserFactoryImpl) WarmupParsers(ctx context.Context, languages []valueobject.Language) error {
	// Track successful warmups
	var successCount int

	// Pre-load grammars without holding the factory lock to reduce contention
	for _, lang := range languages {
		langName := mapDomainLanguageToForest(lang.Name())
		grammar := forest.GetLanguage(langName)
		if grammar == nil {
			slogger.Warn(ctx, "Failed to warmup parser for language", slogger.Fields{
				"language": lang.Name(),
				"reason":   "grammar not found",
			})
			continue
		}

		// Grammar loaded successfully
		successCount++
		slogger.Debug(ctx, "Parser warmed up", slogger.Fields{
			"language": lang.Name(),
		})
	}

	// Update pool statistics to track successful warmups
	f.mu.Lock()
	if f.parserPool.PoolStatistics != nil {
		// Track warmup operations in pool statistics
		f.parserPool.PoolStatistics.TotalCreated += int64(successCount)
	}
	f.mu.Unlock()

	slogger.Info(ctx, "Parser warmup completed", slogger.Fields{
		"languages_requested": len(languages),
		"successful_warmups":  successCount,
	})

	return nil
}

// GetHealthStatus returns health status of the factory.
func (f *ParserFactoryImpl) GetHealthStatus(ctx context.Context) (*FactoryHealthStatus, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	now := time.Now()
	uptime := now.Sub(f.startTime)

	poolHealth := &ParserPoolHealth{
		PoolSize:         f.parserPool.MaxSize,
		HealthyParsers:   f.parserPool.AvailableParsers,
		UnhealthyParsers: 0,
		LastPoolCleanup:  now,
	}

	health := &FactoryHealthStatus{
		IsHealthy:          true,
		Status:             "healthy",
		LastChecked:        now,
		Issues:             []string{},
		ParserPoolHealth:   poolHealth,
		SupportedLanguages: len(f.supportedLangs),
		ActiveParsers:      f.parserPool.InUseParsers,
		MemoryUsage:        0, // Would be calculated from runtime.MemStats
		ConfigurationValid: true,
	}

	slogger.Debug(ctx, "Factory health status checked", slogger.Fields{
		"is_healthy":          health.IsHealthy,
		"supported_languages": health.SupportedLanguages,
		"uptime_seconds":      uptime.Seconds(),
	})

	return health, nil
}

// Cleanup releases all factory resources with comprehensive error handling.
func (f *ParserFactoryImpl) Cleanup(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	slogger.Info(ctx, "Starting factory cleanup", slogger.Fields{
		"supported_languages": len(f.supportedLangs),
		"pool_size":           f.parserPool.CurrentSize,
		"active_parsers":      f.parserPool.InUseParsers,
	})

	var cleanupErrors []string

	// Wait for any active parsers to complete (with timeout)
	if f.parserPool.InUseParsers > 0 {
		slogger.Warn(ctx, "Active parsers detected during cleanup", slogger.Fields{
			"active_count": f.parserPool.InUseParsers,
		})
		// In production, we might want to wait briefly for active operations
	}

	// Reset parser pool with comprehensive cleanup
	if f.parserPool != nil {
		f.parserPool.CurrentSize = 0
		f.parserPool.AvailableParsers = 0
		f.parserPool.InUseParsers = 0
		f.parserPool.Languages = make([]string, 0)

		// Reset pool statistics
		if f.parserPool.PoolStatistics != nil {
			f.parserPool.PoolStatistics.TotalCreated = 0
			f.parserPool.PoolStatistics.TotalDestroyed = 0
			f.parserPool.PoolStatistics.TotalCheckouts = 0
			f.parserPool.PoolStatistics.TotalReturns = 0
			f.parserPool.PoolStatistics.CacheHitRate = 0.0
			f.parserPool.PoolStatistics.ErrorRate = 0.0
		}
	}

	// Clear supported languages
	f.supportedLangs = make([]valueobject.Language, 0)

	// Log cleanup completion with any errors
	if len(cleanupErrors) > 0 {
		slogger.Warn(ctx, "Factory cleanup completed with warnings", slogger.Fields{
			"warnings": cleanupErrors,
		})
	} else {
		slogger.Info(ctx, "Factory cleanup completed successfully", slogger.Fields{})
	}

	return nil
}

// mapDomainLanguageToForest maps domain language names to forest language names.
func mapDomainLanguageToForest(domainLang string) string {
	mapping := map[string]string{
		valueobject.LanguageGo:         "go",
		valueobject.LanguagePython:     "python",
		valueobject.LanguageJavaScript: "javascript",
		valueobject.LanguageTypeScript: "typescript",
		valueobject.LanguageCPlusPlus:  "cpp",
		valueobject.LanguageRust:       "rust",
		"java":                         "java",
		"c":                            "c",
		"c_sharp":                      "c_sharp",
		"ruby":                         "ruby",
		"php":                          "php",
		"kotlin":                       "kotlin",
		"swift":                        "swift",
		"scala":                        "scala",
	}

	if forestName, exists := mapping[domainLang]; exists {
		return forestName
	}
	return domainLang // Default to same name
}

// mapForestLanguageToDomain maps forest language names to domain language names.
func mapForestLanguageToDomain(forestLang string) string {
	mapping := map[string]string{
		"go":         valueobject.LanguageGo,
		"python":     valueobject.LanguagePython,
		"javascript": valueobject.LanguageJavaScript,
		"typescript": valueobject.LanguageTypeScript,
		"tsx":        valueobject.LanguageTypeScript,
		"cpp":        valueobject.LanguageCPlusPlus,
		"rust":       valueobject.LanguageRust,
		"java":       "java",
		"c":          "c",
		"c_sharp":    "c_sharp",
		"ruby":       "ruby",
		"php":        "php",
		"kotlin":     "kotlin",
		"swift":      "swift",
		"scala":      "scala",
	}

	if domainName, exists := mapping[forestLang]; exists {
		return domainName
	}
	return forestLang // Default to same name
}
