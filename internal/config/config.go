package config

import (
	"errors"
	"fmt"
	"math"
	"net/url"
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
	Embedding       EmbeddingConfig       `mapstructure:"embedding"`
	Log             LogConfig             `mapstructure:"log"`
	Connectors      []ConnectorConfig     `mapstructure:"connectors"`
}

// Embedding provider identifiers.
const (
	EmbeddingProviderGemini = "gemini"
	EmbeddingProviderOpenAI = "openai"
)

// EmbeddingConfig selects and configures the embedding provider.
// When Provider is empty, the system defaults to "gemini" for backward
// compatibility with deployments that pre-date the provider switch.
type EmbeddingConfig struct {
	Provider string       `mapstructure:"provider"` // "gemini" | "openai"
	OpenAI   OpenAIConfig `mapstructure:"openai"`
}

// OpenAIConfig holds configuration for the OpenAI embeddings adapter.
// BaseURL is configurable so the same adapter can target Azure OpenAI,
// vLLM, Ollama, LM Studio, Together.ai, and other OpenAI-compatible servers.
type OpenAIConfig struct {
	APIKey     string        `mapstructure:"api_key"`
	BaseURL    string        `mapstructure:"base_url"`
	Model      string        `mapstructure:"model"`
	Dimensions int           `mapstructure:"dimensions"`
	MaxRetries int           `mapstructure:"max_retries"`
	Timeout    time.Duration `mapstructure:"timeout"`
	BatchSize  int           `mapstructure:"batch_size"`
	// TruncateDimensions enables client-side prefix-truncation +
	// L2-renormalization when the server returns a vector larger than
	// Dimensions. Required for MRL-trained models served by backends that
	// ignore the OpenAI `dimensions` request parameter (e.g. LM Studio).
	// Default false: strict equality is enforced and a mismatch is a hard
	// error, preserving the load-bearing footgun guard for typo'd configs.
	TruncateDimensions bool `mapstructure:"truncate_dimensions"`
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

// applyDefaultsAndValidate applies defaults to the embedding config and
// validates it. Defaults are applied in-place so callers see the resolved
// values after Validate() returns.
//
// Provider-selection rules:
//   - "" or "gemini" → use the existing Gemini path; no OpenAI fields required.
//   - "openai"       → require api_key + model + dimensions=768; default base_url and batch_size.
//   - anything else  → reject.
//
// The 768-dimension hard requirement exists because the pgvector column is
// vector(768). OpenAI's text-embedding-3-* models support arbitrary dimensions
// via the `dimensions` request parameter (Matryoshka), so users must opt into
// that to remain schema-compatible. text-embedding-ada-002 is locked at 1536
// and is rejected explicitly to give users a clearer error than a runtime
// dimension mismatch.
func (e *EmbeddingConfig) applyDefaultsAndValidate() error {
	if e.Provider == "" {
		e.Provider = EmbeddingProviderGemini
	}
	switch e.Provider {
	case EmbeddingProviderGemini:
		return nil
	case EmbeddingProviderOpenAI:
		return e.OpenAI.applyDefaultsAndValidate()
	default:
		return fmt.Errorf("embedding.provider %q is not supported (valid: gemini, openai)", e.Provider)
	}
}

// defaultOpenAIBaseURL is the canonical OpenAI endpoint base. We require an
// api_key when the configured BaseURL resolves to the same host — self-hosted
// OpenAI-compatible servers (vLLM, Ollama, LM Studio, etc.) run with no auth.
const defaultOpenAIBaseURL = "https://api.openai.com/v1"

// openAIHost is the hostname extracted from defaultOpenAIBaseURL, used for
// host-based comparison so path variants like "/v1/embeddings/" still match.
const openAIHost = "api.openai.com"

// isOpenAIHost reports whether rawURL targets the official OpenAI API host.
// It returns false for any URL that cannot be parsed.
func isOpenAIHost(rawURL string) bool {
	u, err := url.Parse(rawURL)
	return err == nil && u.Host == openAIHost
}

func (o *OpenAIConfig) applyDefaultsAndValidate() error {
	if o.BaseURL == "" {
		o.BaseURL = defaultOpenAIBaseURL
	}
	// Reject negatives BEFORE defaulting so a negative value is never
	// silently replaced with a default (and never reaches the runtime where
	// it would, e.g., disable HTTP timeouts or break the chunking loop).
	if o.BatchSize < 0 {
		return fmt.Errorf("embedding.openai.batch_size must be >= 0, got %d", o.BatchSize)
	}
	if o.MaxRetries < 0 {
		return fmt.Errorf("embedding.openai.max_retries must be >= 0, got %d", o.MaxRetries)
	}
	if o.Timeout < 0 {
		return fmt.Errorf("embedding.openai.timeout must be >= 0, got %s", o.Timeout)
	}
	if o.BatchSize == 0 {
		o.BatchSize = 256
	}
	if o.MaxRetries == 0 {
		o.MaxRetries = 3
	}
	if o.APIKey == "" && isOpenAIHost(o.BaseURL) {
		return errors.New("embedding.openai.api_key is required when targeting api.openai.com (set base_url for self-hosted/no-auth servers)")
	}
	if o.Model == "" {
		return errors.New("embedding.openai.model is required when provider=openai")
	}
	if o.Model == "text-embedding-ada-002" {
		return errors.New("embedding.openai.model text-embedding-ada-002 is not supported: it is locked at 1536 dimensions and cannot match the vector(768) schema; use text-embedding-3-small or text-embedding-3-large with dimensions=768")
	}
	if o.Dimensions != 768 {
		return fmt.Errorf("embedding.openai.dimensions must be 768 to match the vector(768) schema, got %d", o.Dimensions)
	}
	if o.BatchSize > 2048 {
		return fmt.Errorf("embedding.openai.batch_size must be <= 2048 (OpenAI API limit), got %d", o.BatchSize)
	}
	return nil
}

// ModelName returns the embedding model identifier for the configured provider.
// For gemini (or empty provider), it returns the canonical gemini model name.
// For openai, it returns the configured model from OpenAIConfig.
func (e *EmbeddingConfig) ModelName() string {
	if e.Provider == EmbeddingProviderOpenAI {
		return e.OpenAI.Model
	}
	return "gemini-embedding-001"
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

	// Validate Gemini config when in production (only required when provider is gemini)
	if (c.Embedding.Provider == "" || c.Embedding.Provider == "gemini") &&
		(c.Log.Level == "error" || c.Log.Level == "fatal") {
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

	if err := c.Embedding.applyDefaultsAndValidate(); err != nil {
		return err
	}

	return nil
}
