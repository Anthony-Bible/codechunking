package openai

import (
	"bytes"
	"codechunking/internal/config"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Known OpenAI embedding model names.
const (
	ModelTextEmbedding3Small = "text-embedding-3-small"
	ModelTextEmbedding3Large = "text-embedding-3-large"
)

// defaultModelMaxTokens is the input length cap exposed via GetModelInfo for
// OpenAI text-embedding-3-* models; the underlying servers apply the real
// limit. We surface it so downstream policy code (chunking, truncation) has a
// number to work against without hard-coding it elsewhere.
const defaultModelMaxTokens = 8191

// Client implements outbound.EmbeddingService by talking to OpenAI's
// /v1/embeddings endpoint (or any OpenAI-compatible endpoint via base_url).
type Client struct {
	httpClient         *http.Client
	baseURL            string
	apiKey             string
	model              string
	dimensions         int
	batchSize          int
	maxRetries         int
	truncateDimensions bool
}

// NewClient constructs a Client from a validated OpenAIConfig. Caller must
// have run EmbeddingConfig.applyDefaultsAndValidate so BaseURL and BatchSize
// are populated.
func NewClient(cfg config.OpenAIConfig) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		httpClient:         &http.Client{Timeout: timeout},
		baseURL:            strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:             cfg.APIKey,
		model:              cfg.Model,
		dimensions:         cfg.Dimensions,
		batchSize:          cfg.BatchSize,
		maxRetries:         cfg.MaxRetries,
		truncateDimensions: cfg.TruncateDimensions,
	}
}

func (c *Client) embed(ctx context.Context, texts []string, maxRetries int) (*embeddingResponse, error) {
	return retryWithBackoff(ctx, retryConfig{
		maxAttempts:  maxRetries + 1,
		initialDelay: 500 * time.Millisecond,
		maxDelay:     30 * time.Second,
	}, func() (*embeddingResponse, error) {
		return c.embedOnce(ctx, texts)
	})
}

// resolveRetries picks the per-call retry budget: an explicit positive
// options.RetryAttempts wins, otherwise the client's configured default.
// Zero or negative is treated as "unset" so a default-constructed
// EmbeddingOptions doesn't accidentally disable retries.
func (c *Client) resolveRetries(options outbound.EmbeddingOptions) int {
	if options.RetryAttempts > 0 {
		return options.RetryAttempts
	}
	return c.maxRetries
}

// applyTimeout wraps ctx with options.Timeout when the caller set one.
// Returns the (possibly unchanged) ctx and a cancel func that's always safe
// to defer — when no override is in play, cancel is a no-op.
func applyTimeout(ctx context.Context, options outbound.EmbeddingOptions) (context.Context, context.CancelFunc) {
	if options.Timeout > 0 {
		return context.WithTimeout(ctx, options.Timeout)
	}
	return ctx, func() {}
}

func (c *Client) embedOnce(ctx context.Context, texts []string) (*embeddingResponse, error) {
	body, err := json.Marshal(embeddingRequest{
		Input:          texts,
		Model:          c.model,
		EncodingFormat: "float",
		Dimensions:     c.dimensions,
	})
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &outbound.EmbeddingError{
			Type:      "transport",
			Code:      "request_failed",
			Message:   err.Error(),
			Retryable: true,
			Cause:     err,
		}
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var er errorResponse
		_ = json.Unmarshal(respBytes, &er) // best-effort; some compatible servers send plain text
		if er.Error.Message == "" {
			er.Error.Message = strings.TrimSpace(string(respBytes))
		}
		return nil, classifyHTTPStatus(resp.StatusCode, er.Error)
	}

	var parsed embeddingResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("openai: decode response: %w", err)
	}
	return &parsed, nil
}

// GenerateEmbedding generates a single embedding vector for the given text.
func (c *Client) GenerateEmbedding(
	ctx context.Context,
	text string,
	options outbound.EmbeddingOptions,
) (*outbound.EmbeddingResult, error) {
	ctx, cancel := applyTimeout(ctx, options)
	defer cancel()
	resp, err := c.embed(ctx, []string{text}, c.resolveRetries(options))
	if err != nil {
		return nil, err
	}
	if len(resp.Data) != 1 {
		return nil, fmt.Errorf("openai: expected 1 embedding, got %d", len(resp.Data))
	}
	vec, err := applyDimensionPolicy(resp.Data[0].Embedding, c.dimensions, c.truncateDimensions)
	if err != nil {
		return nil, err
	}
	return &outbound.EmbeddingResult{
		Vector:      vec,
		Dimensions:  len(vec),
		TokenCount:  resp.Usage.TotalTokens,
		Model:       modelName(resp.Model, c.model),
		TaskType:    options.TaskType,
		GeneratedAt: time.Now(),
		RequestID:   uuid.NewString(),
	}, nil
}

// GenerateBatchEmbeddings embeds texts in input order using the synchronous
// /embeddings array endpoint, splitting into chunks of c.batchSize. The
// file-based Batches API is intentionally not implemented here — see
// batch_stub.go for the rationale.
func (c *Client) GenerateBatchEmbeddings(
	ctx context.Context,
	texts []string,
	options outbound.EmbeddingOptions,
) ([]*outbound.EmbeddingResult, error) {
	if len(texts) == 0 {
		return nil, &outbound.EmbeddingError{
			Code:      "empty_texts",
			Type:      "validation",
			Message:   "texts array cannot be empty",
			Retryable: false,
		}
	}
	// Wrap the entire batch loop with options.Timeout (if set) so callers
	// get an end-to-end deadline across all sub-batches and retries.
	ctx, cancel := applyTimeout(ctx, options)
	defer cancel()
	retries := c.resolveRetries(options)
	size := c.batchSize
	if size <= 0 {
		size = len(texts)
	}
	out := make([]*outbound.EmbeddingResult, 0, len(texts))
	for start := 0; start < len(texts); start += size {
		end := start + size
		if end > len(texts) {
			end = len(texts)
		}
		resp, err := c.embed(ctx, texts[start:end], retries)
		if err != nil {
			return nil, err
		}
		ordered, err := c.assembleBatchChunk(resp, start, end-start, options)
		if err != nil {
			return nil, err
		}
		out = append(out, ordered...)
	}
	return out, nil
}

// assembleBatchChunk validates a single embedding response chunk and returns
// the per-item results in input order. It enforces three invariants:
//   - response item count matches the request size
//   - every `index` is in range and unique (a duplicate would silently leave
//     a nil slot in the output, which downstream code would then deref)
//   - every embedding has the configured dimension count
//
// Per-chunk token totals are distributed evenly across items so downstream
// sums approximate the true total instead of multiplying by chunk size.
func (c *Client) assembleBatchChunk(
	resp *embeddingResponse,
	start, want int,
	options outbound.EmbeddingOptions,
) ([]*outbound.EmbeddingResult, error) {
	if len(resp.Data) != want {
		return nil, fmt.Errorf(
			"openai: chunk returned %d items, expected %d (request offset %d)",
			len(resp.Data), want, start,
		)
	}
	perItemTokens := 0
	if n := len(resp.Data); n > 0 {
		perItemTokens = resp.Usage.TotalTokens / n
	}
	ordered := make([]*outbound.EmbeddingResult, want)
	seen := make([]bool, want)
	for i := range resp.Data {
		item := &resp.Data[i]
		if err := c.validateBatchItem(item, want, start, seen); err != nil {
			return nil, err
		}
		seen[item.Index] = true
		ordered[item.Index] = &outbound.EmbeddingResult{
			Vector:      item.Embedding,
			Dimensions:  len(item.Embedding),
			TokenCount:  perItemTokens,
			Model:       modelName(resp.Model, c.model),
			TaskType:    options.TaskType,
			GeneratedAt: time.Now(),
			RequestID:   uuid.NewString(),
		}
	}
	return ordered, nil
}

func (c *Client) validateBatchItem(item *embeddingResponseItem, want, start int, seen []bool) error {
	if item.Index < 0 || item.Index >= want {
		return fmt.Errorf("openai: response item index %d out of range [0,%d)", item.Index, want)
	}
	if seen[item.Index] {
		return fmt.Errorf("openai: duplicate response index %d in chunk starting at %d", item.Index, start)
	}
	vec, err := applyDimensionPolicy(item.Embedding, c.dimensions, c.truncateDimensions)
	if err != nil {
		return fmt.Errorf("openai: chunk %d item %d: %w", start, item.Index, err)
	}
	item.Embedding = vec
	return nil
}

// GenerateCodeChunkEmbedding embeds a single CodeChunk and decorates the
// result with the chunk's metadata. The embedding model itself sees only
// chunk.Content — language/path/etc. are not concatenated into the text
// because doing so would change the semantic position in vector space.
func (c *Client) GenerateCodeChunkEmbedding(
	ctx context.Context,
	chunk *outbound.CodeChunk,
	options outbound.EmbeddingOptions,
) (*outbound.CodeChunkEmbedding, error) {
	if chunk == nil {
		return nil, errors.New("openai: chunk is nil")
	}
	res, err := c.GenerateEmbedding(ctx, chunk.Content, options)
	if err != nil {
		return nil, err
	}
	return &outbound.CodeChunkEmbedding{
		EmbeddingResult: res,
		ChunkID:         chunk.ID,
		SourceFile:      chunk.FilePath,
		Language:        chunk.Language,
		ChunkType:       chunk.Type,
	}, nil
}

// GetSupportedModels reports the OpenAI text-embedding-3-* models known to
// be schema-compatible (768 dims via the dimensions parameter).
func (c *Client) GetSupportedModels(_ context.Context) ([]string, error) {
	return []string{ModelTextEmbedding3Small, ModelTextEmbedding3Large}, nil
}

// EstimateTokenCount returns a fast, dependency-free estimate using the
// 1 token ≈ 4 characters heuristic. Accuracy is ~80–90% for natural language
// and ~70–85% for code.
//
// We deliberately do NOT pull in tiktoken-go here: the only consumer of this
// number in our pipeline is the 8192-token-per-chunk guard, and the API
// itself returns context_length_exceeded for genuine overflows. If sub-percent
// accuracy becomes a hard requirement (e.g. for cost forecasting), swap this
// for github.com/pkoukk/tiktoken-go behind the same signature — no callers
// need to change.
func (c *Client) EstimateTokenCount(_ context.Context, text string) (int, error) {
	return approxTokenCount(text), nil
}

func approxTokenCount(text string) int {
	return (len(text) + 3) / 4
}

// modelName returns resp when non-empty, falling back to configured for
// OpenAI-compatible servers that omit the model field in their response.
func modelName(resp, configured string) string {
	if resp != "" {
		return resp
	}
	return configured
}

// CountTokens returns the (approximate) token count for text. See
// EstimateTokenCount for the accuracy/dependency trade-off rationale.
func (c *Client) CountTokens(_ context.Context, text string, model string) (*outbound.TokenCountResult, error) {
	if model == "" {
		model = c.model
	}
	return &outbound.TokenCountResult{
		TotalTokens: approxTokenCount(text),
		Model:       model,
	}, nil
}

// CountTokensBatch counts tokens for many texts. Order matches input.
func (c *Client) CountTokensBatch(ctx context.Context, texts []string, model string) ([]*outbound.TokenCountResult, error) {
	out := make([]*outbound.TokenCountResult, len(texts))
	for i, t := range texts {
		r, err := c.CountTokens(ctx, t, model)
		if err != nil {
			return nil, err
		}
		out[i] = r
	}
	return out, nil
}

// CountTokensWithCallback counts tokens for each chunk and invokes cb after
// each result. Per the port contract, callback errors are not fatal —
// processing continues for the remaining chunks.
func (c *Client) CountTokensWithCallback(
	ctx context.Context,
	chunks []outbound.CodeChunk,
	model string,
	cb outbound.TokenCountCallback,
) error {
	for i := range chunks {
		r, err := c.CountTokens(ctx, chunks[i].Content, model)
		if err != nil {
			return err
		}
		if cb != nil {
			_ = cb(i, &chunks[i], r) // intentionally swallowed per port contract
		}
	}
	return nil
}

// ValidateApiKey verifies credentials by issuing a minimal embedding probe.
// It bypasses the retry loop so a flaky 5xx doesn't make a credential check
// take 30s+.
func (c *Client) ValidateApiKey(ctx context.Context) error {
	if c.apiKey == "" {
		return errors.New("openai: api_key is empty")
	}
	_, err := c.embedOnce(ctx, []string{"ping"})
	return err
}

// GetModelInfo returns a descriptor for the configured OpenAI model.
// For unrecognized models (custom servers, vLLM, Ollama) we still return a
// generic descriptor so callers don't have to special-case absent metadata.
func (c *Client) GetModelInfo(_ context.Context) (*outbound.ModelInfo, error) {
	info := &outbound.ModelInfo{
		Name:               c.model,
		Dimensions:         c.dimensions,
		MaxTokens:          defaultModelMaxTokens,
		SupportsCustomDim:  true,
		SupportsBatching:   true,
		SupportedTaskTypes: []outbound.EmbeddingTaskType{outbound.TaskTypeRetrievalDocument, outbound.TaskTypeRetrievalQuery},
		PricingTier:        "paid",
	}
	switch c.model {
	case ModelTextEmbedding3Small:
		info.Description = "OpenAI " + ModelTextEmbedding3Small
		info.Version = "3"
	case ModelTextEmbedding3Large:
		info.Description = "OpenAI " + ModelTextEmbedding3Large
		info.Version = "3"
	default:
		info.Description = "OpenAI-compatible embedding model"
	}
	return info, nil
}
