package gemini

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// DefaultModel is the default Gemini embedding model.
	DefaultModel = "gemini-embedding-001"

	// Error severity levels.
	ErrorSeverityLow    = "low"
	ErrorSeverityMedium = "medium"
	ErrorSeverityHigh   = "high"
)

// ClientConfig holds the configuration for the Gemini API client.
type ClientConfig struct {
	APIKey          string        `json:"api_key"`
	BaseURL         string        `json:"base_url"`
	Model           string        `json:"model"`
	TaskType        string        `json:"task_type"`
	Timeout         time.Duration `json:"timeout"`
	MaxRetries      int           `json:"max_retries"`
	MaxTokens       int           `json:"max_tokens"`
	Dimensions      int           `json:"dimensions"`
	UserAgent       string        `json:"user_agent"`
	EnableBatching  bool          `json:"enable_batching"`
	BatchSize       int           `json:"batch_size"`
	RateLimitBuffer time.Duration `json:"rate_limit_buffer"`
}

// Validate validates the client configuration.
func (c *ClientConfig) Validate() error {
	if err := c.validateAPIKey(); err != nil {
		return err
	}
	if err := c.validateBaseURL(); err != nil {
		return err
	}
	if err := c.validateModel(); err != nil {
		return err
	}
	if err := c.validateTaskType(); err != nil {
		return err
	}
	if err := c.validateTimeout(); err != nil {
		return err
	}
	if err := c.validateMaxRetries(); err != nil {
		return err
	}
	return c.validateDimensions()
}

func (c *ClientConfig) validateAPIKey() error {
	if c.APIKey == "" {
		return errors.New("API key cannot be empty")
	}
	if strings.TrimSpace(c.APIKey) == "" {
		return errors.New("API key cannot be empty or whitespace")
	}
	return nil
}

func (c *ClientConfig) validateBaseURL() error {
	if c.BaseURL == "" {
		return nil
	}
	if _, err := url.Parse(c.BaseURL); err != nil {
		return errors.New("invalid base URL")
	}
	if !strings.HasPrefix(c.BaseURL, "http") {
		return errors.New("invalid base URL")
	}
	return nil
}

func (c *ClientConfig) validateModel() error {
	if c.Model != "" && c.Model != DefaultModel {
		return errors.New("unsupported model")
	}
	return nil
}

func (c *ClientConfig) validateTaskType() error {
	if c.TaskType == "" {
		return nil
	}
	return validateTaskTypeValue(c.TaskType)
}

func (c *ClientConfig) validateTimeout() error {
	if c.Timeout < 0 {
		return errors.New("timeout must be positive")
	}
	return nil
}

func (c *ClientConfig) validateMaxRetries() error {
	if c.MaxRetries < 0 {
		return errors.New("max retries cannot be negative")
	}
	return nil
}

func (c *ClientConfig) validateDimensions() error {
	if c.Dimensions < 0 {
		return errors.New("dimensions cannot be negative")
	}
	if c.Dimensions > 0 && c.Dimensions != 768 {
		return errors.New("unsupported dimensions for model " + DefaultModel)
	}
	return nil
}

func validateTaskTypeValue(taskType string) error {
	validTaskTypes := []string{
		"RETRIEVAL_DOCUMENT",
		"RETRIEVAL_QUERY",
		"SEMANTIC_SIMILARITY",
		"CLASSIFICATION",
		"CLUSTERING",
	}
	for _, validType := range validTaskTypes {
		if taskType == validType {
			return nil
		}
	}
	return errors.New("unsupported task type")
}

// Client represents the Gemini API client.
type Client struct {
	config     *ClientConfig
	httpClient *http.Client
}

// NewClient creates a new Gemini API client with the provided configuration.
func NewClient(config *ClientConfig) (*Client, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	// Validate first - this will catch explicit invalid values like Dimensions: 0
	if err := validateClientConfig(config); err != nil {
		return nil, err
	}

	// Then apply defaults
	finalConfig := applyConfigDefaults(config)
	httpClient := createHTTPClient(finalConfig.Timeout)

	client := &Client{
		config:     finalConfig,
		httpClient: httpClient,
	}

	return client, nil
}

// validateClientConfig validates the client configuration before applying defaults.
// This is a wrapper around the method-based validation for consistency.
func validateClientConfig(config *ClientConfig) error {
	return config.Validate()
}

// applyConfigDefaults creates a new config with defaults applied.
func applyConfigDefaults(config *ClientConfig) *ClientConfig {
	finalConfig := &ClientConfig{
		APIKey:          strings.TrimSpace(config.APIKey),
		BaseURL:         config.BaseURL,
		Model:           config.Model,
		TaskType:        config.TaskType,
		Timeout:         config.Timeout,
		MaxRetries:      config.MaxRetries,
		MaxTokens:       config.MaxTokens,
		Dimensions:      config.Dimensions,
		UserAgent:       config.UserAgent,
		EnableBatching:  config.EnableBatching,
		BatchSize:       config.BatchSize,
		RateLimitBuffer: config.RateLimitBuffer,
	}

	// Set defaults
	if finalConfig.BaseURL == "" {
		finalConfig.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	if finalConfig.Model == "" {
		finalConfig.Model = DefaultModel
	}
	if finalConfig.TaskType == "" {
		finalConfig.TaskType = "RETRIEVAL_DOCUMENT"
	}
	if finalConfig.Timeout == 0 {
		finalConfig.Timeout = 30 * time.Second
	}
	if finalConfig.MaxRetries == 0 {
		finalConfig.MaxRetries = 3
	}
	if finalConfig.Dimensions == 0 {
		finalConfig.Dimensions = 768
	}
	if finalConfig.MaxTokens == 0 {
		finalConfig.MaxTokens = 2048
	}
	if finalConfig.UserAgent == "" {
		finalConfig.UserAgent = "CodeChunking-Gemini-Client/1.0.0"
	}

	return finalConfig
}

// createHTTPClient creates an HTTP client with enhanced transport configuration and timeouts.
func createHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		// Connection pool settings
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,

		// Timeout configurations
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,

		// Performance optimizations
		DisableCompression: false,
		ForceAttemptHTTP2:  true,
		DisableKeepAlives:  false,

		// Connection reuse
		MaxConnsPerHost: 50,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		// Don't follow redirects for API calls
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// NewClientFromEnv creates a new Gemini API client with environment variable support.
func NewClientFromEnv(config *ClientConfig) (*Client, error) {
	if config == nil {
		config = &ClientConfig{}
	}

	// Clone the config to avoid modifying the original
	envConfig := *config

	// Check for API key in environment if not provided in config
	if envConfig.APIKey == "" {
		// Check GEMINI_API_KEY first, then GOOGLE_API_KEY
		if geminiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY")); geminiKey != "" {
			envConfig.APIKey = geminiKey
		} else if googleKey := strings.TrimSpace(os.Getenv("GOOGLE_API_KEY")); googleKey != "" {
			envConfig.APIKey = googleKey
		}
	}

	// If still no API key found
	if strings.TrimSpace(envConfig.APIKey) == "" {
		return nil, errors.New("API key not found in config or environment variables")
	}

	return NewClient(&envConfig)
}

// GetConfig returns a copy of the client configuration.
func (c *Client) GetConfig() *ClientConfig {
	configCopy := *c.config
	return &configCopy
}

// GetHTTPClient returns the HTTP client used by the Gemini client.
func (c *Client) GetHTTPClient() *http.Client {
	return c.httpClient
}

// CreateRequest creates an HTTP request with proper headers, authentication, and timeout handling.
func (c *Client) CreateRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	// Validate input parameters
	if method == "" {
		return nil, errors.New("HTTP method cannot be empty")
	}
	if endpoint == "" {
		return nil, errors.New("endpoint cannot be empty")
	}

	// Construct full URL with proper path joining
	baseURL := strings.TrimSuffix(c.config.BaseURL, "/")
	cleanEndpoint := strings.TrimPrefix(endpoint, "/")
	fullURL := baseURL + "/" + cleanEndpoint

	// Validate URL construction
	if _, err := url.Parse(fullURL); err != nil {
		return nil, fmt.Errorf("invalid URL constructed: %s, error: %w", fullURL, err)
	}

	// Create request with context for proper cancellation and timeouts
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set authentication header
	req.Header.Set("X-Goog-Api-Key", c.config.APIKey)

	// Set standard headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.config.UserAgent)

	// Add request tracing headers
	req.Header.Set("X-Request-ID", generateRequestID())

	// Add client version header for debugging
	req.Header.Set("X-Client-Version", "gemini-client/1.0.0")

	// Set Connection header for better connection management
	req.Header.Set("Connection", "keep-alive")

	// Add timeout context information to headers for debugging
	if deadline, ok := ctx.Deadline(); ok {
		timeoutDuration := time.Until(deadline)
		req.Header.Set("X-Request-Timeout", fmt.Sprintf("%.2fs", timeoutDuration.Seconds()))
	}

	return req, nil
}

// EmbeddingService interface implementation (stubs for GREEN phase)

// GenerateEmbedding generates an embedding vector for a given text content.
func (c *Client) GenerateEmbedding(
	ctx context.Context,
	text string,
	options outbound.EmbeddingOptions,
) (*outbound.EmbeddingResult, error) {
	slogger.Info(ctx, "GenerateEmbedding called", slogger.Fields{
		"text_length": len(text),
		"model":       c.config.Model,
	})
	return nil, errors.New("GenerateEmbedding not implemented yet")
}

// GenerateBatchEmbeddings generates embeddings for multiple texts in a single request.
func (c *Client) GenerateBatchEmbeddings(
	ctx context.Context,
	texts []string,
	options outbound.EmbeddingOptions,
) ([]*outbound.EmbeddingResult, error) {
	slogger.Info(ctx, "GenerateBatchEmbeddings called", slogger.Fields{
		"text_count": len(texts),
		"model":      c.config.Model,
	})
	return nil, errors.New("GenerateBatchEmbeddings not implemented yet")
}

// GenerateCodeChunkEmbedding generates an embedding specifically for a CodeChunk.
func (c *Client) GenerateCodeChunkEmbedding(
	ctx context.Context,
	chunk *outbound.CodeChunk,
	options outbound.EmbeddingOptions,
) (*outbound.CodeChunkEmbedding, error) {
	slogger.Info(ctx, "GenerateCodeChunkEmbedding called", slogger.Fields{
		"chunk_id": chunk.ID,
		"language": chunk.Language,
		"model":    c.config.Model,
	})
	return nil, errors.New("GenerateCodeChunkEmbedding not implemented yet")
}

// ValidateApiKey validates the API key configuration.
func (c *Client) ValidateApiKey(ctx context.Context) error {
	slogger.Info(ctx, "ValidateApiKey called", slogger.Fields{
		"api_key_length": len(c.config.APIKey),
	})
	return errors.New("ValidateApiKey not implemented yet")
}

// GetModelInfo returns information about the embedding model.
func (c *Client) GetModelInfo(ctx context.Context) (*outbound.ModelInfo, error) {
	slogger.Info(ctx, "GetModelInfo called", slogger.Fields{
		"model": c.config.Model,
	})
	return nil, errors.New("GetModelInfo not implemented yet")
}

// GetSupportedModels returns a list of supported embedding models.
func (c *Client) GetSupportedModels(ctx context.Context) ([]string, error) {
	slogger.Info(ctx, "GetSupportedModels called", slogger.Fields{})
	return nil, errors.New("GetSupportedModels not implemented yet")
}

// EstimateTokenCount estimates the number of tokens in the given text.
// Uses improved heuristics based on text characteristics.
func (c *Client) EstimateTokenCount(ctx context.Context, text string) (int, error) {
	if text == "" {
		return 0, nil
	}

	// More accurate token estimation considering:
	// - Words are roughly 1.3 tokens on average
	// - Punctuation and whitespace affect token boundaries
	// - Code has different tokenization patterns than natural language

	// Basic word count
	words := countWords(text)
	if words == 0 {
		// Handle edge cases like only whitespace or punctuation
		runeCount := utf8.RuneCountInString(strings.TrimSpace(text))
		if runeCount == 0 {
			return 0, nil
		}
		// Fall back to character-based estimation
		return max(1, runeCount/4), nil
	}

	// Apply language-specific multipliers
	multiplier := 1.3 // Default multiplier for natural language

	// Detect code-like patterns (higher token density)
	if isCodeLike(text) {
		multiplier = 1.6 // Code has more tokens per word due to symbols
	}

	// Estimate tokens with improved accuracy
	estimatedTokens := int(float64(words) * multiplier)

	// Apply bounds checking
	minTokens := max(1, words/2) // At least half the word count
	maxTokens := words * 3       // At most 3x the word count

	return max(minTokens, min(estimatedTokens, maxTokens)), nil
}

// countWords counts the number of words in the text.
func countWords(text string) int {
	if strings.TrimSpace(text) == "" {
		return 0
	}

	// Split on whitespace and count non-empty parts
	words := strings.Fields(text)
	return len(words)
}

// isCodeLike detects if text appears to be code based on patterns.
func isCodeLike(text string) bool {
	codeIndicators := []string{
		"func ", "function ", "def ", "class ", "import ", "package ",
		"(){", "};", "=>", "===", "!==", "&&", "||", "::", "->",
		"#include", "public ", "private ", "protected ", "static ",
	}

	// Count code-like patterns
	matches := 0
	lowerText := strings.ToLower(text)

	for _, indicator := range codeIndicators {
		if strings.Contains(lowerText, indicator) {
			matches++
			if matches >= 2 {
				return true
			}
		}
	}

	// Additional heuristic: high ratio of special characters
	specialChars := 0
	totalChars := 0

	for _, r := range text {
		totalChars++
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r) {
			specialChars++
		}
	}

	if totalChars > 50 && float64(specialChars)/float64(totalChars) > 0.15 {
		return true
	}

	return false
}

// Helper functions for min/max operations.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// SerializeEmbeddingRequest serializes an embedding request to JSON with optimized performance.
func (c *Client) SerializeEmbeddingRequest(
	ctx context.Context,
	text string,
	options outbound.EmbeddingOptions,
) ([]byte, error) {
	// Input validation with enhanced error context
	if text == "" {
		slogger.Error(ctx, "Empty text provided for embedding serialization", slogger.Fields{})
		return nil, &outbound.EmbeddingError{
			Code:      "empty_text",
			Type:      "validation",
			Message:   "text content cannot be empty",
			Retryable: false,
		}
	}

	// Model validation
	targetModel := options.Model
	if targetModel == "" {
		targetModel = c.config.Model // Use client default
	}
	if targetModel != "gemini-embedding-001" {
		slogger.Error(ctx, "Unsupported model requested", slogger.Fields{
			"requested_model": targetModel,
			"supported_model": "gemini-embedding-001",
		})
		return nil, &outbound.EmbeddingError{
			Code:      "unsupported_model",
			Type:      "validation",
			Message:   fmt.Sprintf("unsupported model: %s, only gemini-embedding-001 is supported", targetModel),
			Retryable: false,
		}
	}

	// Build request structure with proper task type handling
	taskType := "RETRIEVAL_DOCUMENT" // Default
	if options.TaskType != "" {
		taskType = string(options.TaskType)
	}

	request := EmbedContentRequest{
		Model: "models/gemini-embedding-001",
		Content: Content{
			Parts: []Part{{Text: text}},
		},
		TaskType:             taskType,
		OutputDimensionality: c.config.Dimensions,
	}

	// Optimized JSON marshaling with error handling
	data, err := json.Marshal(request)
	if err != nil {
		slogger.Error(ctx, "Failed to marshal embedding request", slogger.Fields{
			"error":       err.Error(),
			"text_length": len(text),
			"task_type":   taskType,
		})
		return nil, &outbound.EmbeddingError{
			Code:      "serialization_error",
			Type:      "validation",
			Message:   fmt.Sprintf("failed to serialize request: %v", err),
			Retryable: false,
			Cause:     err,
		}
	}

	slogger.Debug(ctx, "Successfully serialized embedding request", slogger.Fields{
		"request_size": len(data),
		"text_length":  len(text),
		"task_type":    taskType,
	})

	return data, nil
}

// DeserializeEmbeddingResponse deserializes an embedding response from JSON with enhanced validation.
func (c *Client) DeserializeEmbeddingResponse(ctx context.Context, data []byte) (*outbound.EmbeddingResult, error) {
	// Input validation
	if len(data) == 0 {
		slogger.Error(ctx, "Empty response data provided for deserialization", slogger.Fields{})
		return nil, &outbound.EmbeddingError{
			Code:      "empty_response",
			Type:      "validation",
			Message:   "response data cannot be empty",
			Retryable: false,
		}
	}

	var response EmbedContentResponse

	// JSON deserialization with detailed error context
	if err := json.Unmarshal(data, &response); err != nil {
		slogger.Error(ctx, "Failed to parse embedding response JSON", slogger.Fields{
			"error":            err.Error(),
			"response_size":    len(data),
			"response_preview": string(data[:min(len(data), 200)]), // First 200 chars for debugging
		})
		return nil, &outbound.EmbeddingError{
			Code:      "parse_error",
			Type:      "validation",
			Message:   fmt.Sprintf("failed to parse response JSON: %v", err),
			Retryable: false,
			Cause:     err,
		}
	}

	// Validate response structure
	if len(response.Embedding.Values) == 0 {
		slogger.Error(ctx, "Response missing embedding values", slogger.Fields{
			"response_size": len(data),
		})
		return nil, &outbound.EmbeddingError{
			Code:      "missing_embedding",
			Type:      "validation",
			Message:   "response missing required embedding field or embedding is empty",
			Retryable: false,
		}
	}

	// Validate embedding dimensions
	expectedDimensions := c.config.Dimensions
	actualDimensions := len(response.Embedding.Values)
	if actualDimensions != expectedDimensions {
		slogger.Warn(ctx, "Embedding dimensions mismatch", slogger.Fields{
			"expected_dimensions": expectedDimensions,
			"actual_dimensions":   actualDimensions,
		})
	}

	result := &outbound.EmbeddingResult{
		Vector:      response.Embedding.Values,
		Dimensions:  actualDimensions,
		Model:       c.config.Model,
		TaskType:    outbound.TaskTypeRetrievalDocument,
		GeneratedAt: time.Now(),
	}

	slogger.Debug(ctx, "Successfully deserialized embedding response", slogger.Fields{
		"vector_dimensions": actualDimensions,
		"response_size":     len(data),
		"model":             result.Model,
	})

	return result, nil
}

// ValidateTokenLimit validates if text exceeds token limit.
func (c *Client) ValidateTokenLimit(ctx context.Context, text string, maxTokens int) error {
	tokenCount, _ := c.EstimateTokenCount(ctx, text)
	if tokenCount > maxTokens {
		return &outbound.EmbeddingError{
			Code: "token_limit_exceeded",
			Type: "validation",
			Message: fmt.Sprintf(
				"text exceeds maximum token limit of %d (estimated: %d tokens)",
				maxTokens,
				tokenCount,
			),
			Retryable: false,
		}
	}
	return nil
}
