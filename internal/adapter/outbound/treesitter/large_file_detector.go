package treesitter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

type FileSizeCategory int

const (
	FileSizeSmall FileSizeCategory = iota
	FileSizeLarge
	FileSizeVeryLarge
)

type ProcessingStrategy int

const (
	ProcessingStrategyMemory ProcessingStrategy = iota
	ProcessingStrategyChunked
	ProcessingStrategyStreaming
	ProcessingStrategyFallback
)

type DetectorConfig struct {
	SmallFileThreshold       int64
	LargeFileThreshold       int64
	MemoryLimitPerFile       int64
	EnableFallback           bool
	CacheTTL                 time.Duration
	CacheSizeLimit           int
	StrategySelectionTimeout time.Duration
}

type FileSize struct {
	SizeBytes int64
	Category  FileSizeCategory
	Path      string
}

type MemoryConstraints struct {
	AvailableMemory      int64
	MaxPerOperation      int64
	SystemMemoryPressure float64
}

type cachedFileSize struct {
	fileSize FileSize
	expiry   time.Time
}

type LargeFileDetector struct {
	config        DetectorConfig
	fileCache     sync.Map
	strategyCache sync.Map
}

func NewDetector(config DetectorConfig) *LargeFileDetector {
	if config.SmallFileThreshold == 0 {
		config.SmallFileThreshold = 50 * 1024 * 1024
	}
	if config.LargeFileThreshold == 0 {
		config.LargeFileThreshold = 100 * 1024 * 1024
	}
	if config.MemoryLimitPerFile == 0 {
		config.MemoryLimitPerFile = 10 * 1024 * 1024
	}
	if !config.EnableFallback {
		config.EnableFallback = true
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 5 * time.Minute
	}
	if config.CacheSizeLimit == 0 {
		config.CacheSizeLimit = 1000
	}
	if config.StrategySelectionTimeout == 0 {
		config.StrategySelectionTimeout = 30 * time.Second
	}
	return &LargeFileDetector{
		config: config,
	}
}

func (d *LargeFileDetector) DetectFileSizeWithContext(ctx context.Context, filePath string) (FileSize, error) {
	// Check cache first
	if cached, ok := d.fileCache.Load(filePath); ok {
		cachedEntry, ok := cached.(*cachedFileSize)
		switch {
		case !ok:
			// Type assertion failed, remove invalid entry and continue with file stat
			d.fileCache.Delete(filePath)
		case time.Now().Before(cachedEntry.expiry):
			return cachedEntry.fileSize, nil
		default:
			// Expired entry, remove from cache
			d.fileCache.Delete(filePath)
		}
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return FileSize{}, err
	}

	size := info.Size()
	var category FileSizeCategory

	switch {
	case size < d.config.SmallFileThreshold:
		category = FileSizeSmall
	case size < d.config.LargeFileThreshold:
		category = FileSizeLarge
	default:
		category = FileSizeVeryLarge
	}

	result := FileSize{
		SizeBytes: size,
		Category:  category,
		Path:      filePath,
	}

	// Cache the result
	d.fileCache.Store(filePath, &cachedFileSize{
		fileSize: result,
		expiry:   time.Now().Add(d.config.CacheTTL),
	})

	return result, nil
}

// Backward-compatible wrapper for DetectFileSize.
func (d *LargeFileDetector) DetectFileSize(filePath string) (FileSize, error) {
	return d.DetectFileSizeWithContext(context.Background(), filePath)
}

func (d *LargeFileDetector) SelectProcessingStrategyWithContext(
	ctx context.Context,
	fileSize FileSize,
	constraints MemoryConstraints,
) (ProcessingStrategy, error) {
	startTime := time.Now()
	defer func() {
		_ = time.Since(startTime)
	}()

	// Create a context with timeout for strategy selection
	selectionCtx, cancel := context.WithTimeout(ctx, d.config.StrategySelectionTimeout)
	defer cancel()

	// Check if context was cancelled
	if err := selectionCtx.Err(); err != nil {
		return ProcessingStrategyFallback, err
	}

	// Check strategy cache
	cacheKey := fmt.Sprintf("%d_%d_%d_%f",
		fileSize.SizeBytes,
		d.config.SmallFileThreshold,
		d.config.LargeFileThreshold,
		constraints.SystemMemoryPressure)

	if cached, ok := d.strategyCache.Load(cacheKey); ok {
		if strategy, ok := cached.(ProcessingStrategy); ok {
			return strategy, nil
		}
		// If type assertion fails, we continue with strategy calculation
	}

	// Very high memory pressure should trigger fallback
	if constraints.SystemMemoryPressure >= 0.95 {
		strategy := ProcessingStrategyFallback
		d.cacheStrategy(cacheKey, strategy)
		return strategy, nil
	}

	if constraints.SystemMemoryPressure >= 0.8 {
		strategy := ProcessingStrategyStreaming
		d.cacheStrategy(cacheKey, strategy)
		return strategy, nil
	}

	if constraints.AvailableMemory < 1024*1024 {
		strategy := ProcessingStrategyFallback
		d.cacheStrategy(cacheKey, strategy)
		return strategy, nil
	}

	var strategy ProcessingStrategy

	switch fileSize.Category {
	case FileSizeSmall:
		if constraints.AvailableMemory > fileSize.SizeBytes*2 {
			strategy = ProcessingStrategyMemory
		} else {
			strategy = ProcessingStrategyChunked
		}
	case FileSizeLarge:
		if constraints.AvailableMemory > fileSize.SizeBytes+d.config.MemoryLimitPerFile {
			strategy = ProcessingStrategyChunked
		} else {
			strategy = ProcessingStrategyStreaming
		}
	case FileSizeVeryLarge:
		strategy = ProcessingStrategyStreaming
	default:
		strategy = ProcessingStrategyStreaming
	}

	d.cacheStrategy(cacheKey, strategy)
	return strategy, nil
}

// Backward-compatible wrapper for SelectProcessingStrategy.
func (d *LargeFileDetector) SelectProcessingStrategy(
	fileSize FileSize,
	constraints MemoryConstraints,
) (ProcessingStrategy, error) {
	return d.SelectProcessingStrategyWithContext(context.Background(), fileSize, constraints)
}

func (d *LargeFileDetector) ValidateStrategyWithContext(
	ctx context.Context,
	strategy ProcessingStrategy,
	fileSize FileSize,
) error {
	var validationErr error

	switch strategy {
	case ProcessingStrategyMemory:
		if fileSize.Category != FileSizeSmall {
			validationErr = fmt.Errorf("memory strategy not suitable for file size category %d", fileSize.Category)
		}
	case ProcessingStrategyChunked:
		if fileSize.Category == FileSizeVeryLarge {
			validationErr = errors.New("chunked strategy not suitable for very large files")
		}
	case ProcessingStrategyStreaming:
		// Streaming is valid for all file sizes
	case ProcessingStrategyFallback:
		if !d.config.EnableFallback {
			validationErr = errors.New("fallback strategy not enabled")
		}
	default:
		validationErr = errors.New("unknown strategy")
	}

	return validationErr
}

// Backward-compatible wrapper for ValidateStrategy.
func (d *LargeFileDetector) ValidateStrategy(
	strategy ProcessingStrategy,
	fileSize FileSize,
) error {
	return d.ValidateStrategyWithContext(context.Background(), strategy, fileSize)
}

func (d *LargeFileDetector) GetFallbackStrategyWithContext(
	ctx context.Context,
	failedStrategy ProcessingStrategy,
	reason error,
) (ProcessingStrategy, error) {
	if !d.config.EnableFallback {
		return ProcessingStrategyFallback, errors.New("fallback not enabled")
	}

	var fallbackStrategy ProcessingStrategy

	switch failedStrategy {
	case ProcessingStrategyChunked:
		fallbackStrategy = ProcessingStrategyStreaming
	case ProcessingStrategyStreaming:
		fallbackStrategy = ProcessingStrategyFallback
	case ProcessingStrategyMemory:
		fallbackStrategy = ProcessingStrategyChunked
	case ProcessingStrategyFallback:
		fallbackStrategy = ProcessingStrategyFallback
	default:
		fallbackStrategy = ProcessingStrategyFallback
	}

	return fallbackStrategy, nil
}

// Backward-compatible wrapper for GetFallbackStrategy.
func (d *LargeFileDetector) GetFallbackStrategy(
	failedStrategy ProcessingStrategy,
	reason error,
) (ProcessingStrategy, error) {
	return d.GetFallbackStrategyWithContext(context.Background(), failedStrategy, reason)
}

func (d *LargeFileDetector) cacheStrategy(key string, strategy ProcessingStrategy) {
	d.strategyCache.Store(key, strategy)
}
