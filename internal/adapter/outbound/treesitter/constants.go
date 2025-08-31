package treesitter

import "time"

// Configuration constants to extract magic numbers and improve code maintainability.
const (
	// Parser Factory Configuration.
	DefaultMaxCachedParsers    = 50
	DefaultParserTimeout       = 30 * time.Second
	DefaultConcurrencyLimit    = 10
	DefaultCacheEvictionPolicy = "LRU"
	DefaultHealthCheckInterval = 5 * time.Minute

	// Parser Configuration.
	DefaultMaxMemory      = 50 * 1024 * 1024 // 50MB
	DefaultCacheSize      = 100
	DefaultDefaultTimeout = 15 * time.Second
	DefaultMaxSourceSize  = 10 * 1024 * 1024 // 10MB
	DefaultMaxConcurrency = 10

	// Language Support.
	DefaultLanguageCount = 30

	// Timeout Values.
	DefaultTimeoutSeconds    = 30
	DefaultParserTimeoutSec  = 15
	DefaultHealthIntervalMin = 5

	// Memory and Size Limits.
	DefaultMemoryLimitMB  = 50
	DefaultSourceLimitMB  = 10
	DefaultCacheSizeLimit = 100
	DefaultPoolSizeLimit  = 50

	// Concurrency Limits.
	DefaultWorkerCount       = 5
	DefaultConcurrentParsers = 10

	// Statistics and Metrics.
	DefaultMetricsInterval = 1 * time.Minute
	DefaultStatsWindow     = 10 * time.Minute

	// Error Handling.
	DefaultRetryCount    = 3
	DefaultRetryInterval = 1 * time.Second

	// Bridge and Metadata.
	DefaultBridgeParseDuration = 10 * time.Millisecond

	// File Extensions.
	GoExtension         = "go"
	PythonExtension     = "py"
	JavaScriptExtension = "js"
	TypeScriptExtension = "ts"
	CPlusPlusExtension  = "cpp"
	RustExtension       = "rs"
)
