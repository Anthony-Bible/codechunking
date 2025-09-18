package outbound

import (
	"context"
	"time"
)

// EmbeddingService defines the interface for generating embeddings from code chunks.
// This port interface enables embedding generation for semantic search and similarity analysis.
type EmbeddingService interface {
	// GenerateEmbedding generates an embedding vector for a given text content
	GenerateEmbedding(
		ctx context.Context,
		text string,
		options EmbeddingOptions,
	) (*EmbeddingResult, error)

	// GenerateBatchEmbeddings generates embeddings for multiple texts in a single request
	GenerateBatchEmbeddings(
		ctx context.Context,
		texts []string,
		options EmbeddingOptions,
	) ([]*EmbeddingResult, error)

	// GenerateCodeChunkEmbedding generates an embedding specifically for a CodeChunk
	GenerateCodeChunkEmbedding(
		ctx context.Context,
		chunk *CodeChunk,
		options EmbeddingOptions,
	) (*CodeChunkEmbedding, error)

	// ValidateApiKey validates the API key configuration
	ValidateApiKey(ctx context.Context) error

	// GetModelInfo returns information about the embedding model
	GetModelInfo(ctx context.Context) (*ModelInfo, error)

	// GetSupportedModels returns a list of supported embedding models
	GetSupportedModels(ctx context.Context) ([]string, error)

	// EstimateTokenCount estimates the number of tokens in the given text
	EstimateTokenCount(ctx context.Context, text string) (int, error)
}

// EmbeddingOptions configures embedding generation behavior.
type EmbeddingOptions struct {
	Model             string                 `json:"model"`                     // Embedding model to use
	TaskType          EmbeddingTaskType      `json:"task_type"`                 // Task type for specialized embeddings
	Dimensionality    int                    `json:"dimensionality,omitempty"`  // Output dimensions (if supported)
	TruncateStrategy  TruncationStrategy     `json:"truncate_strategy"`         // How to handle text that's too long
	MaxTokens         int                    `json:"max_tokens,omitempty"`      // Maximum tokens per request
	Timeout           time.Duration          `json:"timeout"`                   // Request timeout
	RetryAttempts     int                    `json:"retry_attempts"`            // Number of retry attempts
	CustomMetadata    map[string]interface{} `json:"custom_metadata,omitempty"` // Custom metadata for the request
	EnableBatching    bool                   `json:"enable_batching"`           // Enable batch processing if available
	NormalizeVector   bool                   `json:"normalize_vector"`          // Normalize the resulting vector
	IncludeTokenCount bool                   `json:"include_token_count"`       // Include token count in response
}

// EmbeddingTaskType defines the type of task for specialized embeddings.
type EmbeddingTaskType string

const (
	TaskTypeRetrievalDocument  EmbeddingTaskType = "RETRIEVAL_DOCUMENT"   // For document retrieval/search
	TaskTypeRetrievalQuery     EmbeddingTaskType = "RETRIEVAL_QUERY"      // For general search queries
	TaskTypeCodeRetrievalQuery EmbeddingTaskType = "CODE_RETRIEVAL_QUERY" // For code search queries
	TaskTypeSemanticSimilarity EmbeddingTaskType = "SEMANTIC_SIMILARITY"  // For similarity analysis
	TaskTypeClassification     EmbeddingTaskType = "CLASSIFICATION"       // For classification tasks
	TaskTypeClustering         EmbeddingTaskType = "CLUSTERING"           // For clustering tasks
)

// TruncationStrategy defines how to handle text that exceeds token limits.
type TruncationStrategy string

const (
	TruncateStart  TruncationStrategy = "start"  // Truncate from the beginning
	TruncateEnd    TruncationStrategy = "end"    // Truncate from the end
	TruncateMiddle TruncationStrategy = "middle" // Truncate from the middle
	TruncateNone   TruncationStrategy = "none"   // Don't truncate, return error
)

// EmbeddingResult represents the result of an embedding generation request.
type EmbeddingResult struct {
	Vector        []float64              `json:"vector"`                   // The embedding vector
	Dimensions    int                    `json:"dimensions"`               // Vector dimensions
	TokenCount    int                    `json:"token_count,omitempty"`    // Number of tokens processed
	Model         string                 `json:"model"`                    // Model used for embedding
	TaskType      EmbeddingTaskType      `json:"task_type"`                // Task type used
	ProcessedText string                 `json:"processed_text,omitempty"` // Text after preprocessing
	Truncated     bool                   `json:"truncated"`                // Whether text was truncated
	Metadata      map[string]interface{} `json:"metadata,omitempty"`       // Additional metadata
	GeneratedAt   time.Time              `json:"generated_at"`             // When the embedding was generated
	RequestID     string                 `json:"request_id,omitempty"`     // Unique request identifier
}

// CodeChunkEmbedding extends EmbeddingResult with code chunk specific information.
type CodeChunkEmbedding struct {
	*EmbeddingResult

	ChunkID          string   `json:"chunk_id"`          // Original chunk ID
	SourceFile       string   `json:"source_file"`       // Source file path
	Language         string   `json:"language"`          // Programming language
	ChunkType        string   `json:"chunk_type"`        // Type of code chunk (function, class, etc.)
	SemanticContext  []string `json:"semantic_context"`  // Semantic context extracted
	QualityScore     float64  `json:"quality_score"`     // Quality score of the chunk
	ComplexityScore  float64  `json:"complexity_score"`  // Complexity score of the chunk
	EmbeddingVersion string   `json:"embedding_version"` // Version of embedding approach used
}

// ModelInfo provides information about an embedding model.
type ModelInfo struct {
	Name               string              `json:"name"`                  // Model name
	Version            string              `json:"version"`               // Model version
	MaxTokens          int                 `json:"max_tokens"`            // Maximum tokens per request
	Dimensions         int                 `json:"dimensions"`            // Output vector dimensions
	SupportedTaskTypes []EmbeddingTaskType `json:"supported_task_types"`  // Supported task types
	SupportsCustomDim  bool                `json:"supports_custom_dim"`   // Whether custom dimensions are supported
	SupportsBatching   bool                `json:"supports_batching"`     // Whether batching is supported
	Description        string              `json:"description"`           // Model description
	PricingTier        string              `json:"pricing_tier"`          // Pricing tier (free, paid, etc.)
	RateLimits         *RateLimitInfo      `json:"rate_limits,omitempty"` // Rate limiting information
}

// RateLimitInfo contains rate limiting information for the API.
type RateLimitInfo struct {
	RequestsPerMinute  int           `json:"requests_per_minute"` // Requests per minute limit
	RequestsPerDay     int           `json:"requests_per_day"`    // Requests per day limit
	TokensPerMinute    int           `json:"tokens_per_minute"`   // Tokens per minute limit
	TokensPerDay       int           `json:"tokens_per_day"`      // Tokens per day limit
	BatchSize          int           `json:"batch_size"`          // Maximum batch size
	ConcurrentRequests int           `json:"concurrent_requests"` // Maximum concurrent requests
	ResetWindow        time.Duration `json:"reset_window"`        // Rate limit reset window
}

// EmbeddingError represents an error from the embedding service.
type EmbeddingError struct {
	Code      string `json:"code"`                 // Error code
	Message   string `json:"message"`              // Error message
	Type      string `json:"type"`                 // Error type (auth, quota, validation, etc.)
	RequestID string `json:"request_id,omitempty"` // Request ID for tracing
	Retryable bool   `json:"retryable"`            // Whether the error is retryable
	Cause     error  `json:"cause,omitempty"`      // Underlying error
}

// Error implements the error interface.
func (e *EmbeddingError) Error() string {
	if e.Cause != nil {
		return "embedding service error (" + e.Type + "/" + e.Code + "): " + e.Message + ": " + e.Cause.Error()
	}
	return "embedding service error (" + e.Type + "/" + e.Code + "): " + e.Message
}

// Unwrap returns the underlying cause error.
func (e *EmbeddingError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether the error is retryable.
func (e *EmbeddingError) IsRetryable() bool {
	return e.Retryable
}

// IsAuthenticationError returns whether the error is an authentication error.
func (e *EmbeddingError) IsAuthenticationError() bool {
	return e.Type == "auth" || e.Code == "invalid_api_key" || e.Code == "unauthorized"
}

// IsQuotaError returns whether the error is a quota/rate limit error.
func (e *EmbeddingError) IsQuotaError() bool {
	return e.Type == "quota" || e.Code == "quota_exceeded" || e.Code == "rate_limit_exceeded"
}

// IsValidationError returns whether the error is a validation error.
func (e *EmbeddingError) IsValidationError() bool {
	return e.Type == "validation" || e.Code == "invalid_input" || e.Code == "text_too_long"
}
