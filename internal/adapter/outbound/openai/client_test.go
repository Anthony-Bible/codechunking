package openai

import (
	"codechunking/internal/config"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeServer spins up an httptest.Server that returns the given status and
// body for any request. The captured request is exposed for assertions.
type fakeServer struct {
	t      *testing.T
	srv    *httptest.Server
	last   *http.Request
	bodyIn []byte
}

func newFakeServer(t *testing.T, status int, responseBody string) *fakeServer {
	t.Helper()
	fs := &fakeServer{t: t}
	fs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.last = r
		fs.bodyIn, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, responseBody)
	}))
	t.Cleanup(fs.srv.Close)
	return fs
}

func newClient(t *testing.T, baseURL string, dim int) *Client {
	t.Helper()
	return NewClient(config.OpenAIConfig{
		APIKey:     "sk-test",
		BaseURL:    baseURL,
		Model:      "text-embedding-3-small",
		Dimensions: dim,
		BatchSize:  256,
		Timeout:    5 * time.Second,
	})
}

func successBody(dims int) string {
	v := make([]float64, dims)
	for i := range v {
		v[i] = float64(i) / float64(dims)
	}
	body, _ := json.Marshal(map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{"object": "embedding", "index": 0, "embedding": v},
		},
		"model": "text-embedding-3-small",
		"usage": map[string]int{"prompt_tokens": 4, "total_tokens": 4},
	})
	return string(body)
}

func TestClient_GenerateEmbedding_HitsEmbeddingsPathWithBearerAuth(t *testing.T) {
	fs := newFakeServer(t, 200, successBody(8))
	c := newClient(t, fs.srv.URL, 8)

	res, err := c.GenerateEmbedding(context.Background(), "hello", outbound.EmbeddingOptions{})
	if err != nil {
		t.Fatalf("GenerateEmbedding: %v", err)
	}
	if got, want := fs.last.URL.Path, "/embeddings"; got != want {
		t.Errorf("expected path %q, got %q", want, got)
	}
	if got := fs.last.Header.Get("Authorization"); got != "Bearer sk-test" {
		t.Errorf("expected Bearer auth header, got %q", got)
	}
	if got := fs.last.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("expected JSON content-type, got %q", got)
	}
	if res.Dimensions != 8 {
		t.Errorf("expected 8 dimensions, got %d", res.Dimensions)
	}
	if res.TokenCount != 4 {
		t.Errorf("expected token count 4, got %d", res.TokenCount)
	}
	if res.Model != "text-embedding-3-small" {
		t.Errorf("expected model text-embedding-3-small, got %q", res.Model)
	}
}

func TestClient_GenerateEmbedding_SendsCorrectRequestBody(t *testing.T) {
	fs := newFakeServer(t, 200, successBody(8))
	c := newClient(t, fs.srv.URL, 8)

	if _, err := c.GenerateEmbedding(context.Background(), "hello", outbound.EmbeddingOptions{}); err != nil {
		t.Fatalf("GenerateEmbedding: %v", err)
	}

	var sent embeddingRequest
	if err := json.Unmarshal(fs.bodyIn, &sent); err != nil {
		t.Fatalf("unmarshal sent body: %v", err)
	}
	if len(sent.Input) != 1 || sent.Input[0] != "hello" {
		t.Errorf("expected input=[hello], got %v", sent.Input)
	}
	if sent.Model != "text-embedding-3-small" {
		t.Errorf("expected model text-embedding-3-small, got %q", sent.Model)
	}
	if sent.Dimensions != 8 {
		t.Errorf("expected dimensions=8, got %d", sent.Dimensions)
	}
	if sent.EncodingFormat != "float" {
		t.Errorf("expected encoding_format=float, got %q", sent.EncodingFormat)
	}
}

func TestClient_GenerateEmbedding_RejectsDimensionMismatch(t *testing.T) {
	// Server returns 6-dim vector even though client requested 8 — should error.
	fs := newFakeServer(t, 200, successBody(6))
	c := newClient(t, fs.srv.URL, 8)

	_, err := c.GenerateEmbedding(context.Background(), "hello", outbound.EmbeddingOptions{})
	if err == nil {
		t.Fatal("expected dimension-mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "dimension") {
		t.Errorf("expected error to mention dimension, got %q", err.Error())
	}
}

func TestClient_GenerateEmbedding_Maps401ToAuthError(t *testing.T) {
	body := `{"error":{"message":"bad key","type":"invalid_request_error","code":"invalid_api_key"}}`
	fs := newFakeServer(t, 401, body)
	c := newClient(t, fs.srv.URL, 8)

	_, err := c.GenerateEmbedding(context.Background(), "hello", outbound.EmbeddingOptions{})
	if err == nil {
		t.Fatal("expected auth error, got nil")
	}
	var ee *outbound.EmbeddingError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *outbound.EmbeddingError, got %T", err)
	}
	if !ee.IsAuthenticationError() {
		t.Errorf("expected IsAuthenticationError=true, got false (type=%q code=%q)", ee.Type, ee.Code)
	}
	if ee.Retryable {
		t.Errorf("auth error should not be retryable")
	}
}

func TestClient_GenerateEmbedding_Maps429ToQuotaError(t *testing.T) {
	body := `{"error":{"message":"slow down","type":"rate_limit_error","code":"rate_limit_exceeded"}}`
	fs := newFakeServer(t, 429, body)
	c := newClient(t, fs.srv.URL, 8)

	_, err := c.GenerateEmbedding(context.Background(), "hello", outbound.EmbeddingOptions{})
	if err == nil {
		t.Fatal("expected quota error, got nil")
	}
	var ee *outbound.EmbeddingError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *outbound.EmbeddingError, got %T", err)
	}
	if !ee.IsQuotaError() {
		t.Errorf("expected IsQuotaError=true, got false (type=%q code=%q)", ee.Type, ee.Code)
	}
	if !ee.Retryable {
		t.Errorf("rate-limit error should be retryable")
	}
}

func TestClient_GenerateEmbedding_Maps5xxToRetryableServerError(t *testing.T) {
	body := `{"error":{"message":"oops","type":"server_error","code":"internal_server_error"}}`
	fs := newFakeServer(t, 500, body)
	c := newClient(t, fs.srv.URL, 8)

	_, err := c.GenerateEmbedding(context.Background(), "hello", outbound.EmbeddingOptions{})
	if err == nil {
		t.Fatal("expected server error, got nil")
	}
	var ee *outbound.EmbeddingError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *outbound.EmbeddingError, got %T", err)
	}
	if !ee.Retryable {
		t.Errorf("5xx error should be retryable")
	}
}

func TestClient_GenerateEmbedding_Maps400ToValidationError(t *testing.T) {
	body := `{"error":{"message":"too long","type":"invalid_request_error","code":"context_length_exceeded"}}`
	fs := newFakeServer(t, 400, body)
	c := newClient(t, fs.srv.URL, 8)

	_, err := c.GenerateEmbedding(context.Background(), "hello", outbound.EmbeddingOptions{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	var ee *outbound.EmbeddingError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *outbound.EmbeddingError, got %T", err)
	}
	if ee.Type != "validation" {
		t.Errorf("expected type=validation, got %q", ee.Type)
	}
	if ee.Retryable {
		t.Errorf("400 should not be retryable")
	}
}

func TestClient_ValidateApiKey_NoKeyReturnsError(t *testing.T) {
	c := NewClient(config.OpenAIConfig{
		APIKey:     "",
		BaseURL:    "http://localhost",
		Model:      "text-embedding-3-small",
		Dimensions: 8,
		BatchSize:  256,
	})
	if err := c.ValidateApiKey(context.Background()); err == nil {
		t.Error("expected error for empty api_key")
	}
}

func TestClient_ValidateApiKey_OkWhenServerAccepts(t *testing.T) {
	fs := newFakeServer(t, 200, successBody(8))
	c := newClient(t, fs.srv.URL, 8)
	if err := c.ValidateApiKey(context.Background()); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// chunkRecordingServer captures every request and returns multi-item embeddings.
func chunkRecordingServer(t *testing.T, dim int) (*httptest.Server, *[][]string) {
	t.Helper()
	var captured [][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req embeddingRequest
		_ = json.Unmarshal(body, &req)
		captured = append(captured, append([]string(nil), req.Input...))

		data := make([]map[string]interface{}, len(req.Input))
		for i, s := range req.Input {
			vec := make([]float64, dim)
			vec[0] = float64(len(s)) // marker so test can verify ordering
			data[i] = map[string]interface{}{
				"object":    "embedding",
				"index":     i,
				"embedding": vec,
			}
		}
		resp, _ := json.Marshal(map[string]interface{}{
			"object": "list",
			"data":   data,
			"model":  "text-embedding-3-small",
			"usage":  map[string]int{"prompt_tokens": len(req.Input), "total_tokens": len(req.Input)},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

func TestClient_GenerateBatchEmbeddings_ChunksByBatchSize(t *testing.T) {
	srv, captured := chunkRecordingServer(t, 4)
	c := NewClient(config.OpenAIConfig{
		APIKey: "sk-test", BaseURL: srv.URL, Model: "text-embedding-3-small",
		Dimensions: 4, BatchSize: 3, Timeout: 5 * time.Second,
	})

	inputs := []string{"a", "bb", "ccc", "dddd", "eeeee", "ffffff", "ggggggg"}
	out, err := c.GenerateBatchEmbeddings(context.Background(), inputs, outbound.EmbeddingOptions{})
	if err != nil {
		t.Fatalf("GenerateBatchEmbeddings: %v", err)
	}
	if len(out) != len(inputs) {
		t.Fatalf("expected %d results, got %d", len(inputs), len(out))
	}
	if got := len(*captured); got != 3 {
		t.Errorf("expected 3 chunked calls (3+3+1), got %d", got)
	}
	for i, in := range inputs {
		if int(out[i].Vector[0]) != len(in) {
			t.Errorf("result[%d] order broken: marker=%v but input=%q (len %d)", i, out[i].Vector[0], in, len(in))
		}
	}
}

// fixedTotalTokensServer returns a successful embedding response with a
// configurable, constant `total_tokens` value regardless of input size, so
// tests can verify how the adapter distributes the per-chunk aggregate.
func fixedTotalTokensServer(t *testing.T, dim, totalTokens int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req embeddingRequest
		_ = json.Unmarshal(body, &req)
		data := make([]map[string]interface{}, len(req.Input))
		for i := range req.Input {
			vec := make([]float64, dim)
			vec[0] = float64(len(req.Input[i]))
			data[i] = map[string]interface{}{
				"object": "embedding", "index": i, "embedding": vec,
			}
		}
		resp, _ := json.Marshal(map[string]interface{}{
			"object": "list", "data": data, "model": "text-embedding-3-small",
			"usage": map[string]int{"prompt_tokens": totalTokens, "total_tokens": totalTokens},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClient_GenerateBatchEmbeddings_DistributesTokenCountAcrossItems(t *testing.T) {
	// Server reports total_tokens=6 per chunk regardless of input size.
	// With BatchSize=3 and 7 inputs we get chunks of 3+3+1, so per-item
	// counts should be 2, 2, and 6 respectively. The bug being guarded
	// against is the prior implementation copying total_tokens onto every
	// item, which would over-count by chunk size when downstream sums.
	srv := fixedTotalTokensServer(t, 4, 6)
	c := NewClient(config.OpenAIConfig{
		APIKey: "sk-test", BaseURL: srv.URL, Model: "text-embedding-3-small",
		Dimensions: 4, BatchSize: 3, Timeout: 5 * time.Second,
	})

	inputs := []string{"a", "bb", "ccc", "dddd", "eeeee", "ffffff", "ggggggg"}
	out, err := c.GenerateBatchEmbeddings(context.Background(), inputs, outbound.EmbeddingOptions{})
	if err != nil {
		t.Fatalf("GenerateBatchEmbeddings: %v", err)
	}

	want := []int{2, 2, 2, 2, 2, 2, 6} // chunks of 3+3+1, total_tokens=6 each → 2,2,6
	for i, w := range want {
		if got := out[i].TokenCount; got != w {
			t.Errorf("result[%d].TokenCount = %d, want %d", i, got, w)
		}
	}

	// Sum across the chunk should approximate the chunk total (integer
	// division means the singleton chunk is exact, multi-item chunks may
	// drop the remainder).
	sum := 0
	for _, r := range out[:3] {
		sum += r.TokenCount
	}
	if sum != 6 {
		t.Errorf("first chunk sum = %d, want 6 (the chunk's total_tokens)", sum)
	}
}

// TestClient_SyncBatchPath_ProducesEmbeddingsForAllInputs is the unit-level
// guard for review issue #6: when the OpenAI BatchStub causes runWorkerService
// to skip the BatchSubmitter / BatchPoller goroutines, JobProcessor falls back
// to calling embeddingService.GenerateBatchEmbeddings directly. This test
// pins down that contract — N inputs across multiple chunks must yield N
// non-empty, ordered results from a single GenerateBatchEmbeddings call.
// The end-to-end NATS+Postgres+JobProcessor integration test is out of scope
// here; this guards the adapter side of the seam.
func TestClient_SyncBatchPath_ProducesEmbeddingsForAllInputs(t *testing.T) {
	srv, captured := chunkRecordingServer(t, 4)
	c := NewClient(config.OpenAIConfig{
		APIKey: "sk-test", BaseURL: srv.URL, Model: "text-embedding-3-small",
		Dimensions: 4, BatchSize: 4, Timeout: 5 * time.Second,
	})

	inputs := make([]string, 10)
	for i := range inputs {
		inputs[i] = strings.Repeat("x", i+1)
	}

	out, err := c.GenerateBatchEmbeddings(context.Background(), inputs, outbound.EmbeddingOptions{})
	if err != nil {
		t.Fatalf("GenerateBatchEmbeddings: %v", err)
	}
	if len(out) != len(inputs) {
		t.Fatalf("expected %d results, got %d", len(inputs), len(out))
	}
	if got, want := len(*captured), 3; got != want {
		t.Errorf("expected %d HTTP calls (4+4+2), got %d", want, got)
	}
	for i, r := range out {
		if r == nil {
			t.Errorf("result[%d] is nil", i)
			continue
		}
		if len(r.Vector) == 0 {
			t.Errorf("result[%d] has empty vector", i)
		}
		if int(r.Vector[0]) != len(inputs[i]) {
			t.Errorf("result[%d] order broken: marker=%v but input len=%d", i, r.Vector[0], len(inputs[i]))
		}
	}
}

func TestClient_GenerateBatchEmbeddings_PreservesOrderWhenServerShuffles(t *testing.T) {
	// Server returns items with shuffled `index` values — adapter must
	// reorder by index, not slice position.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req embeddingRequest
		_ = json.Unmarshal(body, &req)
		// Build response with items in REVERSE order but correct indices.
		data := make([]map[string]interface{}, len(req.Input))
		for i := range req.Input {
			vec := make([]float64, 4)
			vec[0] = float64(len(req.Input[i]))
			data[len(req.Input)-1-i] = map[string]interface{}{
				"object": "embedding", "index": i, "embedding": vec,
			}
		}
		resp, _ := json.Marshal(map[string]interface{}{
			"object": "list", "data": data, "model": "x",
			"usage": map[string]int{"prompt_tokens": 1, "total_tokens": 1},
		})
		_, _ = w.Write(resp)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(config.OpenAIConfig{
		APIKey: "sk-test", BaseURL: srv.URL, Model: "x",
		Dimensions: 4, BatchSize: 10, Timeout: 5 * time.Second,
	})

	inputs := []string{"a", "bb", "ccc"}
	out, err := c.GenerateBatchEmbeddings(context.Background(), inputs, outbound.EmbeddingOptions{})
	if err != nil {
		t.Fatalf("GenerateBatchEmbeddings: %v", err)
	}
	for i, in := range inputs {
		if int(out[i].Vector[0]) != len(in) {
			t.Errorf("result[%d] order broken: marker=%v expected len(%q)=%d", i, out[i].Vector[0], in, len(in))
		}
	}
}

// TestClient_GenerateBatchEmbeddings_RejectsDuplicateIndices guards against a
// server (buggy or hostile) that returns the right COUNT of items but with
// duplicated `index` values. Without explicit duplicate detection, the
// duplicate would silently overwrite one slot in the output slice and leave
// another nil — callers would then dereference a nil result downstream.
func TestClient_GenerateBatchEmbeddings_RejectsDuplicateIndices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req embeddingRequest
		_ = json.Unmarshal(body, &req)
		// Return len(req.Input) items but two share index=0; index=2 is missing.
		data := make([]map[string]interface{}, len(req.Input))
		for i := range req.Input {
			idx := i
			if i == len(req.Input)-1 {
				idx = 0 // duplicate the first index
			}
			data[i] = map[string]interface{}{
				"object": "embedding", "index": idx, "embedding": []float64{1, 2, 3, 4},
			}
		}
		resp, _ := json.Marshal(map[string]interface{}{
			"object": "list", "data": data, "model": "x",
			"usage": map[string]int{"prompt_tokens": 1, "total_tokens": 1},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(config.OpenAIConfig{
		APIKey: "sk-test", BaseURL: srv.URL, Model: "x",
		Dimensions: 4, BatchSize: 10, Timeout: 5 * time.Second,
	})
	_, err := c.GenerateBatchEmbeddings(context.Background(), []string{"a", "b", "c"}, outbound.EmbeddingOptions{})
	if err == nil {
		t.Fatal("expected duplicate-index error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected error to mention 'duplicate', got %q", err.Error())
	}
}

func TestClient_GenerateBatchEmbeddings_EmptyInput(t *testing.T) {
	srv, _ := chunkRecordingServer(t, 4)
	c := NewClient(config.OpenAIConfig{
		APIKey: "sk-test", BaseURL: srv.URL, Model: "x",
		Dimensions: 4, BatchSize: 10, Timeout: 5 * time.Second,
	})
	out, err := c.GenerateBatchEmbeddings(context.Background(), nil, outbound.EmbeddingOptions{})
	if out != nil {
		t.Errorf("expected nil result, got %d items", len(out))
	}
	var embErr *outbound.EmbeddingError
	if !errors.As(err, &embErr) {
		t.Fatalf("expected *outbound.EmbeddingError, got %T: %v", err, err)
	}
	if embErr.Code != "empty_texts" {
		t.Errorf("expected code %q, got %q", "empty_texts", embErr.Code)
	}
	if embErr.Type != "validation" {
		t.Errorf("expected type %q, got %q", "validation", embErr.Type)
	}
	if embErr.Retryable {
		t.Errorf("expected non-retryable error")
	}
}

func TestClient_GenerateBatchEmbeddings_FailsFastOnError(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"oops","type":"server_error"}}`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req embeddingRequest
		_ = json.Unmarshal(body, &req)
		data := make([]map[string]interface{}, len(req.Input))
		for i := range req.Input {
			data[i] = map[string]interface{}{"object": "embedding", "index": i, "embedding": []float64{1, 2, 3, 4}}
		}
		resp, _ := json.Marshal(map[string]interface{}{
			"object": "list", "data": data, "model": "x",
			"usage": map[string]int{"prompt_tokens": 1, "total_tokens": 1},
		})
		_, _ = w.Write(resp)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(config.OpenAIConfig{
		APIKey: "sk-test", BaseURL: srv.URL, Model: "x",
		Dimensions: 4, BatchSize: 2, Timeout: 5 * time.Second,
	})
	inputs := []string{"a", "b", "c", "d", "e"}
	_, err := c.GenerateBatchEmbeddings(context.Background(), inputs, outbound.EmbeddingOptions{})
	if err == nil {
		t.Fatal("expected error on second chunk, got nil")
	}
}

func TestClient_GenerateCodeChunkEmbedding_PopulatesMetadata(t *testing.T) {
	fs := newFakeServer(t, 200, successBody(8))
	c := newClient(t, fs.srv.URL, 8)

	chunk := &outbound.CodeChunk{
		ID:       "chunk-42",
		FilePath: "src/foo.go",
		Language: "go",
		Type:     "function",
		Content:  "func Foo() {}",
	}
	got, err := c.GenerateCodeChunkEmbedding(context.Background(), chunk, outbound.EmbeddingOptions{})
	if err != nil {
		t.Fatalf("GenerateCodeChunkEmbedding: %v", err)
	}
	if got.ChunkID != "chunk-42" {
		t.Errorf("expected chunk_id=chunk-42, got %q", got.ChunkID)
	}
	if got.SourceFile != "src/foo.go" {
		t.Errorf("expected source_file=src/foo.go, got %q", got.SourceFile)
	}
	if got.Language != "go" {
		t.Errorf("expected language=go, got %q", got.Language)
	}
	if got.ChunkType != "function" {
		t.Errorf("expected chunk_type=function, got %q", got.ChunkType)
	}
	if got.EmbeddingResult == nil || len(got.Vector) != 8 {
		t.Errorf("expected embedding result with 8-dim vector, got %+v", got.EmbeddingResult)
	}
}

func TestClient_GenerateCodeChunkEmbedding_NilChunk(t *testing.T) {
	c := newClient(t, "http://localhost", 8)
	_, err := c.GenerateCodeChunkEmbedding(context.Background(), nil, outbound.EmbeddingOptions{})
	if err == nil {
		t.Fatal("expected error for nil chunk")
	}
}

func TestClient_GetSupportedModels(t *testing.T) {
	c := newClient(t, "http://localhost", 768)
	models, err := c.GetSupportedModels(context.Background())
	if err != nil {
		t.Fatalf("GetSupportedModels: %v", err)
	}
	want := map[string]bool{ModelTextEmbedding3Small: true, ModelTextEmbedding3Large: true}
	for _, m := range models {
		if !want[m] {
			t.Errorf("unexpected model %q", m)
		}
	}
	if len(models) != len(want) {
		t.Errorf("expected %d models, got %d", len(want), len(models))
	}
}

func TestClient_EstimateTokenCount(t *testing.T) {
	c := newClient(t, "http://localhost", 768)
	got, err := c.EstimateTokenCount(context.Background(), "hello world!")
	if err != nil {
		t.Fatalf("EstimateTokenCount: %v", err)
	}
	if got != 3 {
		t.Errorf("expected 3 tokens (12 chars / 4), got %d", got)
	}
}

func TestClient_GetModelInfo_KnownModel(t *testing.T) {
	c := newClient(t, "http://example", 768)
	info, err := c.GetModelInfo(context.Background())
	if err != nil {
		t.Fatalf("GetModelInfo: %v", err)
	}
	if info.Name != "text-embedding-3-small" {
		t.Errorf("expected name text-embedding-3-small, got %q", info.Name)
	}
	if info.Dimensions != 768 {
		t.Errorf("expected 768 dims, got %d", info.Dimensions)
	}
	if !info.SupportsCustomDim {
		t.Error("text-embedding-3-* supports custom dim")
	}
}

func TestClient_GetModelInfo_UnknownModelStillReturns(t *testing.T) {
	c := NewClient(config.OpenAIConfig{
		APIKey:     "sk-test",
		BaseURL:    "http://localhost",
		Model:      "nomic-embed-text",
		Dimensions: 768,
		BatchSize:  256,
	})
	info, err := c.GetModelInfo(context.Background())
	if err != nil {
		t.Fatalf("GetModelInfo: %v", err)
	}
	if info.Name != "nomic-embed-text" {
		t.Errorf("expected name nomic-embed-text, got %q", info.Name)
	}
}

// TestClient_GenerateEmbedding_HonorsOptionsTimeout asserts options.Timeout
// enforces a tighter deadline than http.Client.Timeout. Server delays 500ms,
// http client allows 5s, but options.Timeout=50ms must abort first.
func TestClient_GenerateEmbedding_HonorsOptionsTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, successBody(8))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(config.OpenAIConfig{
		APIKey:     "sk-test",
		BaseURL:    srv.URL,
		Model:      "text-embedding-3-small",
		Dimensions: 8,
		BatchSize:  256,
		MaxRetries: 0,
		Timeout:    5 * time.Second,
	})

	start := time.Now()
	_, err := c.GenerateEmbedding(
		context.Background(),
		"hello",
		outbound.EmbeddingOptions{Timeout: 50 * time.Millisecond},
	)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 400*time.Millisecond {
		t.Errorf("expected per-call timeout to fire <400ms, took %s", elapsed)
	}
}

// TestClient_GenerateBatchEmbeddings_HonorsOptionsRetryAttempts asserts an
// explicit options.RetryAttempts overrides c.maxRetries. With c.maxRetries=10
// against a server that always 500s, default would be 11 calls; passing
// RetryAttempts=2 must cap calls at 3 (1 initial + 2 retries).
func TestClient_GenerateBatchEmbeddings_HonorsOptionsRetryAttempts(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, `{"error":{"message":"boom","type":"server_error"}}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(config.OpenAIConfig{
		APIKey:     "sk-test",
		BaseURL:    srv.URL,
		Model:      "text-embedding-3-small",
		Dimensions: 8,
		BatchSize:  256,
		MaxRetries: 10,
		Timeout:    30 * time.Second,
	})

	_, err := c.GenerateBatchEmbeddings(
		context.Background(),
		[]string{"hello"},
		outbound.EmbeddingOptions{RetryAttempts: 2},
	)
	if err == nil {
		t.Fatal("expected error from failing server")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("expected 3 calls (1 + 2 retries), got %d", got)
	}
}
