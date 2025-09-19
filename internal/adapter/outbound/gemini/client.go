package gemini

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"google.golang.org/genai"
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
	APIKey     string        `json:"api_key"`
	Model      string        `json:"model"`
	TaskType   string        `json:"task_type"`
	Timeout    time.Duration `json:"timeout"`
	Dimensions int           `json:"dimensions"`
}

// Validate validates the client configuration.
func (c *ClientConfig) Validate() error {
	if err := c.validateAPIKey(); err != nil {
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
		"CODE_RETRIEVAL_QUERY",
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
	config *ClientConfig
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

	// Apply defaults
	finalConfig := applyConfigDefaults(config)

	// Create client
	client := &Client{
		config: finalConfig,
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
		APIKey:     strings.TrimSpace(config.APIKey),
		Model:      config.Model,
		TaskType:   config.TaskType,
		Timeout:    config.Timeout,
		Dimensions: config.Dimensions,
	}

	// Set defaults
	if finalConfig.Model == "" {
		finalConfig.Model = DefaultModel
	}
	if finalConfig.TaskType == "" {
		finalConfig.TaskType = "RETRIEVAL_DOCUMENT"
	}
	if finalConfig.Timeout == 0 {
		finalConfig.Timeout = 30 * time.Second
	}
	if finalConfig.Dimensions == 0 {
		finalConfig.Dimensions = 768
	}

	return finalConfig
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

// EmbeddingService interface implementation (stubs for GREEN phase)

// GenerateEmbedding generates an embedding vector for a given text content.
func (c *Client) GenerateEmbedding(
	ctx context.Context,
	text string,
	options outbound.EmbeddingOptions,
) (*outbound.EmbeddingResult, error) {
	requestID := c.LogEmbeddingRequest(ctx, text, options)
	startTime := time.Now()

	// Input validation
	if strings.TrimSpace(text) == "" {
		return nil, &outbound.EmbeddingError{
			Code:      "empty_text",
			Type:      "validation",
			Message:   "text content cannot be empty",
			Retryable: false,
		}
	}

	// Create GenAI client (for future use when SDK API is confirmed)
	_, err := c.createGenaiClient(ctx)
	if err != nil {
		c.LogEmbeddingError(ctx, requestID, func() *outbound.EmbeddingError {
			target := &outbound.EmbeddingError{}
			_ = errors.As(err, &target)
			return target
		}(), time.Since(startTime))
		return nil, err
	}

	// Determine model to use
	model := options.Model
	if model == "" {
		model = c.config.Model
	}

	// Configure task type (for future use when SDK supports it)
	_ = convertTaskType(options.TaskType)

	// Create GenAI client
	genaiClient, err := c.createGenaiClient(ctx)
	if err != nil {
		c.LogEmbeddingError(ctx, requestID, func() *outbound.EmbeddingError {
			target := &outbound.EmbeddingError{}
			_ = errors.As(err, &target)
			return target
		}(), time.Since(startTime))
		return nil, err
	}

	// Create content for embedding
	content := genai.NewContentFromText(text, genai.RoleUser)

	// Configure task type
	taskType := convertTaskType(options.TaskType)

	// Create embed config
	config := &genai.EmbedContentConfig{
		TaskType: taskType,
	}
	if c.config.Dimensions > 0 {
		dims := int32(c.config.Dimensions)
		config.OutputDimensionality = &dims
	}

	// Generate embedding using the SDK
	result, err := genaiClient.Models.EmbedContent(ctx, model, []*genai.Content{content}, config)
	if err != nil {
		embeddingErr := c.convertSDKError(err)
		c.LogEmbeddingError(ctx, requestID, embeddingErr, time.Since(startTime))
		return nil, embeddingErr
	}

	// Convert to our domain type
	if len(result.Embeddings) == 0 || result.Embeddings[0] == nil || len(result.Embeddings[0].Values) == 0 {
		embeddingErr := &outbound.EmbeddingError{
			Code:      "no_embeddings",
			Type:      "response",
			Message:   "no embeddings returned from API",
			Retryable: false,
		}
		c.LogEmbeddingError(ctx, requestID, embeddingErr, time.Since(startTime))
		return nil, embeddingErr
	}

	embedding := result.Embeddings[0]
	embeddingResult := &outbound.EmbeddingResult{
		Vector:      c.convertToFloat64Slice(embedding.Values),
		Dimensions:  len(embedding.Values),
		Model:       model,
		TaskType:    options.TaskType,
		GeneratedAt: time.Now(),
		RequestID:   requestID,
	}

	c.LogEmbeddingResponse(ctx, requestID, embeddingResult, time.Since(startTime))
	return embeddingResult, nil
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

	// Input validation
	if len(texts) == 0 {
		return nil, &outbound.EmbeddingError{
			Code:      "empty_texts",
			Type:      "validation",
			Message:   "texts array cannot be empty",
			Retryable: false,
		}
	}

	// For now, implement batch as sequential calls to GenerateEmbedding
	// TODO: Replace with actual batch API when SDK supports it
	results := make([]*outbound.EmbeddingResult, 0, len(texts))

	for i, text := range texts {
		result, err := c.GenerateEmbedding(ctx, text, options)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for text %d: %w", i, err)
		}
		results = append(results, result)
	}

	return results, nil
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

	// Input validation
	if chunk == nil {
		return nil, &outbound.EmbeddingError{
			Code:      "nil_chunk",
			Type:      "validation",
			Message:   "chunk cannot be nil",
			Retryable: false,
		}
	}

	// Generate embedding for the chunk content
	embeddingResult, err := c.GenerateEmbedding(ctx, chunk.Content, options)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding for code chunk: %w", err)
	}

	// Create CodeChunkEmbedding from the result
	codeChunkEmbedding := &outbound.CodeChunkEmbedding{
		EmbeddingResult:  embeddingResult,
		ChunkID:          chunk.ID,
		SourceFile:       chunk.FilePath,
		Language:         chunk.Language,
		ChunkType:        "code_block", // Default chunk type
		SemanticContext:  []string{},   // Empty for now
		QualityScore:     1.0,          // Default quality score
		ComplexityScore:  0.5,          // Default complexity score
		EmbeddingVersion: "v1.0",       // Version of embedding approach
	}

	return codeChunkEmbedding, nil
}

// ValidateApiKey validates the API key configuration.
func (c *Client) ValidateApiKey(ctx context.Context) error {
	slogger.Info(ctx, "ValidateApiKey called", slogger.Fields{
		"api_key_length": len(c.config.APIKey),
	})

	// Basic validation first
	if err := c.config.validateAPIKey(); err != nil {
		return &outbound.EmbeddingError{
			Code:      "invalid_api_key",
			Type:      "auth",
			Message:   fmt.Sprintf("API key validation failed: %v", err),
			Retryable: false,
			Cause:     err,
		}
	}

	// Test API key by attempting to create a client
	client, err := c.createGenaiClient(ctx)
	if err != nil {
		return &outbound.EmbeddingError{
			Code:      "api_key_test_failed",
			Type:      "auth",
			Message:   "API key test failed - unable to create client",
			Retryable: false,
			Cause:     err,
		}
	}

	// For now, if client creation succeeds, we assume the API key is valid
	// TODO: Add actual API test call when SDK API is confirmed
	_ = client // Prevent unused variable warning

	slogger.Info(ctx, "API key validation successful", slogger.Fields{})
	return nil
}

// GetModelInfo returns information about the embedding model.
func (c *Client) GetModelInfo(ctx context.Context) (*outbound.ModelInfo, error) {
	slogger.Info(ctx, "GetModelInfo called", slogger.Fields{
		"model": c.config.Model,
	})

	// Return hardcoded model info for the default model
	// TODO: Replace with actual SDK call to get model info when API is available
	modelInfo := &outbound.ModelInfo{
		Name:       c.config.Model,
		Version:    "1.0",
		MaxTokens:  8192, // Typical max for embedding models
		Dimensions: 768,  // Default dimensions for gemini-embedding-001
		SupportedTaskTypes: []outbound.EmbeddingTaskType{
			outbound.TaskTypeRetrievalDocument,
			outbound.TaskTypeRetrievalQuery,
			outbound.TaskTypeCodeRetrievalQuery,
			outbound.TaskTypeSemanticSimilarity,
			outbound.TaskTypeClassification,
			outbound.TaskTypeClustering,
		},
		SupportsCustomDim: true,
		SupportsBatching:  false, // TODO: Update when batch API is available
		Description:       "Google Gemini embedding model for text embeddings",
		PricingTier:       "paid",
		RateLimits: &outbound.RateLimitInfo{
			RequestsPerMinute:  60,
			RequestsPerDay:     1000,
			TokensPerMinute:    100000,
			TokensPerDay:       1000000,
			BatchSize:          100,
			ConcurrentRequests: 10,
			ResetWindow:        time.Minute,
		},
	}

	return modelInfo, nil
}

// GetSupportedModels returns a list of supported embedding models.
func (c *Client) GetSupportedModels(ctx context.Context) ([]string, error) {
	slogger.Info(ctx, "GetSupportedModels called", slogger.Fields{})

	// Return hardcoded list of supported models
	// TODO: Replace with actual SDK call to list models when API is available
	supportedModels := []string{
		"gemini-embedding-001",
		"gemini-embedding-001", // Alternative model name
	}

	return supportedModels, nil
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

// createGenaiClient creates a new genai client for the request.
func (c *Client) createGenaiClient(ctx context.Context) (*genai.Client, error) {
	clientConfig := &genai.ClientConfig{
		APIKey: c.config.APIKey,
	}
	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, &outbound.EmbeddingError{
			Code:      "client_creation_failed",
			Type:      "auth",
			Message:   fmt.Sprintf("failed to create genai cl ient: %v", err),
			Retryable: false,
			Cause:     err,
		}
	}
	return client, nil
}

// convertTaskType converts from outbound.EmbeddingTaskType to string for genai SDK.
func convertTaskType(taskType outbound.EmbeddingTaskType) string {
	switch taskType {
	case outbound.TaskTypeRetrievalDocument:
		return "RETRIEVAL_DOCUMENT"
	case outbound.TaskTypeRetrievalQuery:
		return "RETRIEVAL_QUERY"
	case outbound.TaskTypeCodeRetrievalQuery:
		return "CODE_RETRIEVAL_QUERY"
	case outbound.TaskTypeSemanticSimilarity:
		return "SEMANTIC_SIMILARITY"
	case outbound.TaskTypeClassification:
		return "CLASSIFICATION"
	case outbound.TaskTypeClustering:
		return "CLUSTERING"
	default:
		return "RETRIEVAL_DOCUMENT"
	}
}

// convertSDKError converts a genai SDK error to our domain error type.
func (c *Client) convertSDKError(err error) *outbound.EmbeddingError {
	if err == nil {
		return nil
	}

	// Default error values
	embeddingErr := &outbound.EmbeddingError{
		Code:      "api_error",
		Type:      "server",
		Message:   err.Error(),
		Retryable: true,
		Cause:     err,
	}

	// Check for common error patterns
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "api key") || strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "authentication") {
		embeddingErr.Code = "invalid_api_key"
		embeddingErr.Type = "auth"
		embeddingErr.Retryable = false
	} else if strings.Contains(errStr, "quota") || strings.Contains(errStr, "limit") {
		embeddingErr.Code = "quota_exceeded"
		embeddingErr.Type = "quota"
		embeddingErr.Retryable = true
	} else if strings.Contains(errStr, "invalid") || strings.Contains(errStr, "bad request") {
		embeddingErr.Code = "invalid_input"
		embeddingErr.Type = "validation"
		embeddingErr.Retryable = false
	}

	return embeddingErr
}

// convertToFloat64Slice converts []float32 to []float64.
func (c *Client) convertToFloat64Slice(values []float32) []float64 {
	if values == nil {
		return nil
	}

	result := make([]float64, len(values))
	for i, v := range values {
		result[i] = float64(v)
	}
	return result
}
