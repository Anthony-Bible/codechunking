package config

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/spf13/viper"
)

// ConnectorConfig holds configuration for a single git connector.
// Auth tokens may reference environment variables using ${VAR_NAME} syntax.
type ConnectorConfig struct {
	Name      string   `mapstructure:"name"`
	Type      string   `mapstructure:"type"`
	BaseURL   string   `mapstructure:"base_url"`
	AuthToken string   `mapstructure:"auth_token"`
	Groups    []string `mapstructure:"groups"`
	Projects  []string `mapstructure:"projects"`
}

// Config holds the complete application configuration.
type Config struct {
	API             APIConfig             `mapstructure:"api"`
	Worker          WorkerConfig          `mapstructure:"worker"`
	Database        DatabaseConfig        `mapstructure:"database"`
	NATS            NATSConfig            `mapstructure:"nats"`
	Search          SearchConfig          `mapstructure:"search"`
	Zoekt           ZoektConfig           `mapstructure:"zoekt"`
	Gemini          GeminiConfig          `mapstructure:"gemini"`
	BatchProcessing BatchProcessingConfig `mapstructure:"batch_processing"`
	Log             LogConfig             `mapstructure:"log"`
	Connectors      []ConnectorConfig     `mapstructure:"connectors"`
}

// APIConfig holds API server configuration.
type APIConfig struct {
	Host                    string        `mapstructure:"host"`
	Port                    string        `mapstructure:"port"`
	ReadTimeout             time.Duration `mapstructure:"read_timeout"`
	WriteTimeout            time.Duration `mapstructure:"write_timeout"`
	EnableDefaultMiddleware *bool         `mapstructure:"enable_default_middleware"`
	EnableCORS              *bool         `mapstructure:"enable_cors"`
	EnableSecurityHeaders   *bool         `mapstructure:"enable_security_headers"`
	EnableLogging           *bool         `mapstructure:"enable_logging"`
	EnableErrorHandling     *bool         `mapstructure:"enable_error_handling"`
}

// WorkerConfig holds worker configuration.
type WorkerConfig struct {
	Concurrency int           `mapstructure:"concurrency"`
	QueueGroup  string        `mapstructure:"queue_group"`
	JobTimeout  time.Duration `mapstructure:"job_timeout"`
}

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Host               string `mapstructure:"host"`
	Port               int    `mapstructure:"port"`
	User               string `mapstructure:"user"`
	Password           string `mapstructure:"password"`
	Name               string `mapstructure:"name"`
	SSLMode            string `mapstructure:"sslmode"`
	MaxConnections     int    `mapstructure:"max_connections"`
	MaxIdleConnections int    `mapstructure:"max_idle_connections"`
}

// DSN returns the database connection string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode)
}

// NATSConfig holds NATS configuration.
type NATSConfig struct {
	URL           string        `mapstructure:"url"`
	MaxReconnects int           `mapstructure:"max_reconnects"`
	ReconnectWait time.Duration `mapstructure:"reconnect_wait"`
	TestMode      bool          `mapstructure:"test_mode"`
}

// SearchConfig holds search configuration.
type SearchConfig struct {
	IterativeScanMode string  `mapstructure:"iterative_scan_mode"`
	SemanticWeight    float64 `mapstructure:"semantic_weight"`
	TextWeight        float64 `mapstructure:"text_weight"`
}

// Validate checks that hybrid ranking weights are in range and, when both are
// explicitly set (non-zero), that they sum to approximately 1.0.
// When only one weight is set, normalizeHybridWeights derives the complement.
func (s SearchConfig) Validate() error {
	if s.SemanticWeight < 0 || s.SemanticWeight > 1 {
		return fmt.Errorf("search.semantic_weight must be between 0 and 1, got %v", s.SemanticWeight)
	}
	if s.TextWeight < 0 || s.TextWeight > 1 {
		return fmt.Errorf("search.text_weight must be between 0 and 1, got %v", s.TextWeight)
	}

	// Only enforce the sum constraint when both weights are explicitly provided.
	if s.SemanticWeight != 0 && s.TextWeight != 0 {
		const weightTolerance = 0.001
		sum := s.SemanticWeight + s.TextWeight
		if math.Abs(sum-1.0) > weightTolerance {
			return fmt.Errorf(
				"search.semantic_weight + search.text_weight must equal 1.0 (±%g), got semantic_weight=%v text_weight=%v sum=%v",
				weightTolerance,
				s.SemanticWeight,
				s.TextWeight,
				sum,
			)
		}
	}

	return nil
}

// ZoektConfig holds Zoekt full-text search configuration.
type ZoektConfig struct {
	Webserver          ZoektWebserverConfig          `mapstructure:"webserver"`
	Indexing           ZoektIndexingConfig           `mapstructure:"indexing"`
	Search             ZoektSearchConfig             `mapstructure:"search"`
	ConcurrentIndexing ZoektConcurrentIndexingConfig `mapstructure:"concurrent_indexing"`
}

// ZoektWebserverConfig holds Zoekt webserver connection configuration.
type ZoektWebserverConfig struct {
	Host    string        `mapstructure:"host"`
	Port    int           `mapstructure:"port"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// ZoektIndexingConfig holds Zoekt indexing configuration.
type ZoektIndexingConfig struct {
	GitIndexPath    string        `mapstructure:"git_index_path"`   // Path to zoekt-git-index binary
	IndexDir        string        `mapstructure:"index_dir"`        // Shard output directory
	MaxFileSize     int64         `mapstructure:"max_file_size"`    // Max file size to index in bytes
	IncludeTests    bool          `mapstructure:"include_tests"`    // Include test files
	IncludeVendor   bool          `mapstructure:"include_vendor"`   // Include vendor directories
	DefaultPriority float64       `mapstructure:"default_priority"` // Normalization priority
	Timeout         time.Duration `mapstructure:"timeout"`          // Indexing timeout
}

// ZoektSearchConfig holds Zoekt search configuration.
type ZoektSearchConfig struct {
	MaxResults      int           `mapstructure:"max_results"`        // Max total results
	MaxMatchPerFile int           `mapstructure:"max_match_per_file"` // Max matches per file
	ContextLines    int           `mapstructure:"context_lines"`      // Lines of context around matches
	Timeout         time.Duration `mapstructure:"timeout"`            // Search timeout
}

// ZoektConcurrentIndexingConfig holds concurrent indexing configuration.
type ZoektConcurrentIndexingConfig struct {
	Enabled           bool `mapstructure:"enabled"`             // Enable concurrent Zoekt + embedding indexing
	MaxConcurrentJobs int  `mapstructure:"max_concurrent_jobs"` // Max concurrent indexing jobs
	RetryAttempts     int  `mapstructure:"retry_attempts"`      // Retry attempts for failed indexing
}

// GeminiConfig holds Gemini API configuration.
type GeminiConfig struct {
	APIKey     string        `mapstructure:"api_key"`
	Model      string        `mapstructure:"model"`
	MaxRetries int           `mapstructure:"max_retries"`
	Timeout    time.Duration `mapstructure:"timeout"`
	Batch      BatchConfig   `mapstructure:"batch"` // Batch embedding configuration
}

// BatchConfig holds batch embedding configuration.
type BatchConfig struct {
	InputDir     string        `mapstructure:"input_dir"`     // Directory for batch input files
	OutputDir    string        `mapstructure:"output_dir"`    // Directory for batch output files
	PollInterval time.Duration `mapstructure:"poll_interval"` // Polling interval for job status
	MaxWaitTime  time.Duration `mapstructure:"max_wait_time"` // Maximum time to wait for job completion
	Enabled      bool          `mapstructure:"enabled"`       // Whether batch embeddings are enabled
}

// BatchProcessingConfig holds enhanced batch processing configuration.
type BatchProcessingConfig struct {
	Enabled              bool              `mapstructure:"enabled"`                // Enable queue-based batch processing
	ThresholdChunks      int               `mapstructure:"threshold_chunks"`       // Min chunks to trigger batch processing
	FallbackToSequential bool              `mapstructure:"fallback_to_sequential"` // Fall back to individual processing on failures
	QueueLimits          QueueLimitsConfig `mapstructure:"queue_limits"`           // Queue size and wait time limits
	UseTestEmbeddings    bool              `mapstructure:"use_test_embeddings"`    // Use test embeddings (for testing/TDD only)
	MaxBatchSize         int               `mapstructure:"max_batch_size"`         // Maximum chunks per API batch
	InitialBackoff       time.Duration     `mapstructure:"initial_backoff"`        // Initial backoff delay for retries
	MaxBackoff           time.Duration     `mapstructure:"max_backoff"`            // Maximum backoff delay
	MaxRetries           int               `mapstructure:"max_retries"`            // Maximum retry attempts per batch
	EnableBatchChunking  bool              `mapstructure:"enable_batch_chunking"`  // Enable/disable batch chunking
	PollerInterval       time.Duration     `mapstructure:"poller_interval"`        // Interval for polling Gemini batch jobs
	MaxConcurrentPolls   int               `mapstructure:"max_concurrent_polls"`   // Max concurrent batch job polls

	// Batch Submitter configuration (rate-limit-aware submission)
	SubmitterPollInterval    time.Duration `mapstructure:"submitter_poll_interval"`    // How often submitter checks for pending batches
	MaxConcurrentSubmissions int           `mapstructure:"max_concurrent_submissions"` // Max parallel Gemini API submissions
	SubmissionInitialBackoff time.Duration `mapstructure:"submission_initial_backoff"` // Initial backoff on rate limit
	SubmissionMaxBackoff     time.Duration `mapstructure:"submission_max_backoff"`     // Maximum backoff duration
	MaxSubmissionAttempts    int           `mapstructure:"max_submission_attempts"`    // Max retry attempts per batch

	// Token counting configuration
	TokenCounting TokenCountingConfig `mapstructure:"token_counting"` // Token counting configuration
}

// TokenCountingConfig holds configuration for token counting integration.
type TokenCountingConfig struct {
	Enabled           bool   `mapstructure:"enabled"`              // Enable token counting
	Mode              string `mapstructure:"mode"`                 // Mode: "all", "sample", or "on_demand"
	SamplePercent     int    `mapstructure:"sample_percent"`       // Percentage of chunks to sample (for "sample" mode)
	MaxTokensPerChunk int    `mapstructure:"max_tokens_per_chunk"` // Maximum tokens per chunk (default: 8192)
}

// QueueLimitsConfig holds queue limits configuration.
type QueueLimitsConfig struct {
	MaxQueueSize int           `mapstructure:"max_queue_size"` // Maximum requests in queue
	MaxWaitTime  time.Duration `mapstructure:"max_wait_time"`  // Maximum time waiting in queue
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// New creates a new Config instance from Viper.
func New(v *viper.Viper) *Config {
	var config Config

	// Unmarshal configuration
	if err := v.Unmarshal(&config); err != nil {
		panic(fmt.Errorf("unable to decode config: %w", err))
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		panic(fmt.Errorf("invalid configuration: %w", err))
	}

	return &config
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	// Required fields validation
	if c.Database.User == "" {
		return errors.New("database.user is required")
	}

	if c.Database.Name == "" {
		return errors.New("database.name is required")
	}

	// Validate Gemini config when in production
	if c.Log.Level == "error" || c.Log.Level == "fatal" {
		if c.Gemini.APIKey == "" {
			return errors.New("gemini.api_key is required in production")
		}
	}

	// Validate numeric ranges
	if c.Worker.Concurrency < 1 {
		return errors.New("worker.concurrency must be at least 1")
	}

	if c.Database.Port < 1 || c.Database.Port > 65535 {
		return errors.New("database.port must be between 1 and 65535")
	}

	if err := c.Search.Validate(); err != nil {
		return err
	}

	return nil
}
