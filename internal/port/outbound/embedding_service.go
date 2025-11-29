package outbound

import (
	"context"
	"time"

	"github.com/google/uuid"
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

	// CountTokens counts the exact number of tokens in the given text using the embedding model
	// Returns a TokenCountResult containing the token count, model used, and optional cache timestamp
	CountTokens(ctx context.Context, text string, model string) (*TokenCountResult, error)

	// CountTokensBatch counts tokens for multiple texts in a single batch request
	// Returns a slice of TokenCountResult matching the input texts order
	CountTokensBatch(ctx context.Context, texts []string, model string) ([]*TokenCountResult, error)

	// CountTokensWithCallback counts tokens for each chunk and invokes the callback after each result.
	// This allows for progressive processing (e.g., saving chunks to DB in batches).
	// The callback receives the index, chunk, and token count result for each processed chunk.
	// If the callback returns an error, processing continues but the error is logged.
	CountTokensWithCallback(ctx context.Context, chunks []CodeChunk, model string, callback TokenCountCallback) error
}

// TokenCountCallback is called after each successful token count with the updated chunk.
// The index parameter indicates the position of the chunk in the original slice.
// The chunk parameter is the chunk being processed.
// The result parameter contains the token count information.
// Returning an error will log the error but won't stop processing.
type TokenCountCallback func(index int, chunk *CodeChunk, result *TokenCountResult) error

// BatchEmbeddingService defines the interface for file-based batch embedding operations.
// This interface provides asynchronous batch processing capabilities using the Google GenAI Batches API.
type BatchEmbeddingService interface {
	// CreateBatchEmbeddingJob creates a new batch embedding job from a list of texts
	CreateBatchEmbeddingJob(
		ctx context.Context,
		texts []string,
		options EmbeddingOptions,
		batchID uuid.UUID,
	) (*BatchEmbeddingJob, error)

	// CreateBatchEmbeddingJobWithRequests creates a batch job from pre-formed requests with custom RequestIDs
	CreateBatchEmbeddingJobWithRequests(
		ctx context.Context,
		requests []*BatchEmbeddingRequest,
		options EmbeddingOptions,
		batchID uuid.UUID,
	) (*BatchEmbeddingJob, error)

	// CreateBatchEmbeddingJobWithFile creates a batch job using a pre-uploaded file URI
	// This method is used when resuming batch submission after the file was already uploaded
	CreateBatchEmbeddingJobWithFile(
		ctx context.Context,
		requests []*BatchEmbeddingRequest,
		options EmbeddingOptions,
		batchID uuid.UUID,
		fileURI string,
	) (*BatchEmbeddingJob, error)

	// GetBatchJobStatus retrieves the current status of a batch embedding job
	GetBatchJobStatus(
		ctx context.Context,
		jobID string,
	) (*BatchEmbeddingJob, error)

	// ListBatchJobs lists all batch embedding jobs with optional filtering
	ListBatchJobs(
		ctx context.Context,
		filter *BatchJobFilter,
	) ([]*BatchEmbeddingJob, error)

	// GetBatchJobResults retrieves the results of a completed batch embedding job
	GetBatchJobResults(
		ctx context.Context,
		jobID string,
	) ([]*EmbeddingResult, error)

	// CancelBatchJob cancels a running batch embedding job
	CancelBatchJob(
		ctx context.Context,
		jobID string,
	) error

	// DeleteBatchJob deletes a batch embedding job and its associated resources
	DeleteBatchJob(
		ctx context.Context,
		jobID string,
	) error

	// WaitForBatchJob waits for a batch job to complete with timeout and polling
	WaitForBatchJob(
		ctx context.Context,
		jobID string,
		pollInterval time.Duration,
	) (*BatchEmbeddingJob, error)
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

// BatchEmbeddingJob represents a file-based batch embedding job.
type BatchEmbeddingJob struct {
	JobID          string            `json:"job_id"`                    // Unique job identifier
	JobName        string            `json:"job_name"`                  // Human-readable job name
	Model          string            `json:"model"`                     // Model used for embeddings
	State          BatchJobState     `json:"state"`                     // Current job state
	InputFileURI   string            `json:"input_file_uri,omitempty"`  // URI of input file
	OutputFileURI  string            `json:"output_file_uri,omitempty"` // URI of output file (when complete)
	ErrorFileURI   string            `json:"error_file_uri,omitempty"`  // URI of error file (if errors occurred)
	TotalCount     int               `json:"total_count"`               // Total number of items to process
	ProcessedCount int               `json:"processed_count"`           // Number of items processed
	SuccessCount   int               `json:"success_count"`             // Number of successful items
	ErrorCount     int               `json:"error_count"`               // Number of failed items
	CreatedAt      time.Time         `json:"created_at"`                // Job creation timestamp
	UpdatedAt      time.Time         `json:"updated_at"`                // Last update timestamp
	CompletedAt    *time.Time        `json:"completed_at,omitempty"`    // Completion timestamp
	ErrorMessage   string            `json:"error_message,omitempty"`   // Error message if job failed
	Metadata       map[string]string `json:"metadata,omitempty"`        // Additional metadata
	Options        EmbeddingOptions  `json:"options"`                   // Embedding options used
	Progress       *BatchJobProgress `json:"progress,omitempty"`        // Detailed progress information
	EstimatedCost  *BatchJobCostEst  `json:"estimated_cost,omitempty"`  // Estimated cost information
}

// BatchJobState represents the state of a batch job.
type BatchJobState string

const (
	BatchJobStatePending    BatchJobState = "PENDING"    // Job is queued
	BatchJobStateProcessing BatchJobState = "PROCESSING" // Job is being processed
	BatchJobStateCompleted  BatchJobState = "COMPLETED"  // Job completed successfully
	BatchJobStateFailed     BatchJobState = "FAILED"     // Job failed
	BatchJobStateCancelled  BatchJobState = "CANCELLED"  // Job was cancelled
)

// IsTerminal returns whether the job state is terminal (completed, failed, or cancelled).
func (s BatchJobState) IsTerminal() bool {
	return s == BatchJobStateCompleted || s == BatchJobStateFailed || s == BatchJobStateCancelled
}

// BatchJobProgress provides detailed progress information for a batch job.
type BatchJobProgress struct {
	PercentComplete        float64        `json:"percent_complete"`                   // Percentage complete (0-100)
	ItemsRemaining         int            `json:"items_remaining"`                    // Number of items remaining
	EstimatedTimeRemaining *time.Duration `json:"estimated_time_remaining,omitempty"` // Estimated time to completion
	ProcessingRate         float64        `json:"processing_rate"`                    // Items per second
	StartedAt              *time.Time     `json:"started_at,omitempty"`               // When processing started
}

// BatchJobCostEst provides cost estimation for a batch job.
type BatchJobCostEst struct {
	EstimatedTokens int     `json:"estimated_tokens"` // Estimated total tokens
	EstimatedCost   float64 `json:"estimated_cost"`   // Estimated total cost
	CostPerItem     float64 `json:"cost_per_item"`    // Estimated cost per item
	Currency        string  `json:"currency"`         // Currency code (e.g., "USD")
}

// BatchJobFilter provides filtering options for listing batch jobs.
type BatchJobFilter struct {
	States        []BatchJobState `json:"states,omitempty"`         // Filter by job states
	Model         string          `json:"model,omitempty"`          // Filter by model
	CreatedAfter  *time.Time      `json:"created_after,omitempty"`  // Filter by creation time (after)
	CreatedBefore *time.Time      `json:"created_before,omitempty"` // Filter by creation time (before)
	Limit         int             `json:"limit,omitempty"`          // Maximum number of results
}

// BatchEmbeddingRequest represents a single embedding request in a batch.
type BatchEmbeddingRequest struct {
	RequestID string                 `json:"request_id"`         // Unique request identifier
	Text      string                 `json:"text"`               // Text to embed
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // Optional metadata
}

// TokenCountResult represents the result of a token counting operation.
type TokenCountResult struct {
	TotalTokens int        `json:"total_tokens"`        // Total number of tokens counted
	Model       string     `json:"model"`               // Model used for token counting
	CachedAt    *time.Time `json:"cached_at,omitempty"` // Optional timestamp if result was cached
}

// BatchEmbeddingResponse represents a single embedding response from a batch.
type BatchEmbeddingResponse struct {
	RequestID string           `json:"request_id"`       // Request identifier matching the request
	Result    *EmbeddingResult `json:"result,omitempty"` // Embedding result (if successful)
	Error     *EmbeddingError  `json:"error,omitempty"`  // Error (if failed)
}
