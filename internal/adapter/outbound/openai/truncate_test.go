package openai

import (
	"codechunking/internal/config"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// largeVecBody returns a successful single-embedding response of the given
// dimensionality. Values are non-trivial (sin-based) so the renormalization
// step actually has work to do.
func largeVecBody(dim int) string {
	v := make([]float64, dim)
	for i := range v {
		v[i] = math.Sin(float64(i) * 0.01)
	}
	body, _ := json.Marshal(map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{"object": "embedding", "index": 0, "embedding": v},
		},
		"model": "octen-embedding-8b",
		"usage": map[string]int{"prompt_tokens": 4, "total_tokens": 4},
	})
	return string(body)
}

func TestClient_GenerateEmbedding_TruncatesDimensionsWhenEnabled(t *testing.T) {
	// Server returns 4096-d vector; client wants 768-d with truncation enabled.
	fs := newFakeServer(t, 200, largeVecBody(4096))
	c := NewClient(config.OpenAIConfig{
		APIKey:             "sk-test",
		BaseURL:            fs.srv.URL,
		Model:              "octen-embedding-8b",
		Dimensions:         768,
		BatchSize:          256,
		Timeout:            5 * time.Second,
		TruncateDimensions: true,
	})

	res, err := c.GenerateEmbedding(context.Background(), "hello", outbound.EmbeddingOptions{})
	if err != nil {
		t.Fatalf("GenerateEmbedding: %v", err)
	}
	if len(res.Vector) != 768 {
		t.Fatalf("len(Vector) = %d, want 768", len(res.Vector))
	}
	if res.Dimensions != 768 {
		t.Errorf("Dimensions = %d, want 768", res.Dimensions)
	}
	var sumSq float64
	for _, v := range res.Vector {
		sumSq += v * v
	}
	if math.Abs(math.Sqrt(sumSq)-1.0) > 1e-9 {
		t.Errorf("L2 norm = %v, want 1.0 (within 1e-9)", math.Sqrt(sumSq))
	}
}

func TestClient_GenerateEmbedding_RejectsDimensionMismatchWhenDisabled(t *testing.T) {
	// Regression guard: the load-bearing default behavior is strict equality.
	fs := newFakeServer(t, 200, largeVecBody(4096))
	c := NewClient(config.OpenAIConfig{
		APIKey:             "sk-test",
		BaseURL:            fs.srv.URL,
		Model:              "text-embedding-3-small",
		Dimensions:         768,
		BatchSize:          256,
		Timeout:            5 * time.Second,
		TruncateDimensions: false,
	})

	_, err := c.GenerateEmbedding(context.Background(), "hello", outbound.EmbeddingOptions{})
	if err == nil {
		t.Fatal("expected dimension-mismatch error in strict mode, got nil")
	}
	if !strings.Contains(err.Error(), "dimension mismatch") {
		t.Errorf("expected 'dimension mismatch' in error, got %q", err.Error())
	}
}

func TestClient_GenerateEmbedding_RejectsTruncationWhenResponseSmaller(t *testing.T) {
	// Server returns 256-d, client configured for 768-d with truncation
	// enabled — must still error because we cannot synthesize signal.
	fs := newFakeServer(t, 200, largeVecBody(256))
	c := NewClient(config.OpenAIConfig{
		APIKey:             "sk-test",
		BaseURL:            fs.srv.URL,
		Model:              "some-small-model",
		Dimensions:         768,
		BatchSize:          256,
		Timeout:            5 * time.Second,
		TruncateDimensions: true,
	})

	_, err := c.GenerateEmbedding(context.Background(), "hello", outbound.EmbeddingOptions{})
	if err == nil {
		t.Fatal("expected error when response is smaller than configured, got nil")
	}
	if !strings.Contains(err.Error(), "smaller") {
		t.Errorf("expected error to mention 'smaller', got %q", err.Error())
	}
}

func TestClient_GenerateBatchEmbeddings_TruncatesDimensionsWhenEnabled(t *testing.T) {
	const respDim = 4096
	const wantDim = 768

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req embeddingRequest
		_ = json.Unmarshal(body, &req)
		data := make([]map[string]interface{}, len(req.Input))
		for i, s := range req.Input {
			vec := make([]float64, respDim)
			// Marker in coordinate 0 so we can verify ordering survives truncation.
			vec[0] = float64(len(s))
			// Add some other non-zero values so renormalization is meaningful.
			for j := 1; j < respDim; j++ {
				vec[j] = math.Sin(float64(j) * 0.001)
			}
			data[i] = map[string]interface{}{
				"object":    "embedding",
				"index":     i,
				"embedding": vec,
			}
		}
		resp, _ := json.Marshal(map[string]interface{}{
			"object": "list", "data": data, "model": "octen-embedding-8b",
			"usage": map[string]int{"prompt_tokens": len(req.Input), "total_tokens": len(req.Input)},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(config.OpenAIConfig{
		APIKey:             "sk-test",
		BaseURL:            srv.URL,
		Model:              "octen-embedding-8b",
		Dimensions:         wantDim,
		BatchSize:          256,
		Timeout:            5 * time.Second,
		TruncateDimensions: true,
	})

	inputs := []string{"a", "bb", "ccc", "dddd"}
	out, err := c.GenerateBatchEmbeddings(context.Background(), inputs, outbound.EmbeddingOptions{})
	if err != nil {
		t.Fatalf("GenerateBatchEmbeddings: %v", err)
	}
	if len(out) != len(inputs) {
		t.Fatalf("len(out) = %d, want %d", len(out), len(inputs))
	}
	for i, r := range out {
		if r == nil {
			t.Fatalf("result[%d] is nil", i)
		}
		if len(r.Vector) != wantDim {
			t.Errorf("result[%d].Vector len = %d, want %d", i, len(r.Vector), wantDim)
		}
		if r.Dimensions != wantDim {
			t.Errorf("result[%d].Dimensions = %d, want %d", i, r.Dimensions, wantDim)
		}
		var sumSq float64
		for _, v := range r.Vector {
			sumSq += v * v
		}
		if math.Abs(math.Sqrt(sumSq)-1.0) > 1e-9 {
			t.Errorf("result[%d] L2 norm = %v, want 1.0", i, math.Sqrt(sumSq))
		}
		// Order survives truncation: marker (coord 0) was len(input).
		// After renormalization the marker is divided by the original 4096-d
		// norm, but proportional ordering across results is preserved.
		if r.Vector[0] == 0 {
			t.Errorf("result[%d] coord 0 unexpectedly zero — marker lost", i)
		}
	}
	// Specifically: result[0] (input "a", len=1) must have a smaller coord 0
	// than result[3] (input "dddd", len=4) because the source markers were
	// 1 and 4 and they share the same renorm scale (only coord 0 differs).
	if !(out[0].Vector[0] < out[1].Vector[0] && out[1].Vector[0] < out[2].Vector[0] && out[2].Vector[0] < out[3].Vector[0]) {
		t.Errorf("ordering broken after truncation: coord0 = [%v, %v, %v, %v]",
			out[0].Vector[0], out[1].Vector[0], out[2].Vector[0], out[3].Vector[0])
	}
}

// TestClient_GenerateBatchEmbeddings_TruncatesAndPreservesShuffledOrder
// composes two independently-tested behaviors: server-side index shuffling
// (already covered in TestClient_GenerateBatchEmbeddings_PreservesOrderWhenServerShuffles)
// and client-side dimension truncation. Each is correct in isolation; this
// test pins down that mutating item.Embedding inside validateBatchItem
// before the assembleBatchChunk reorder-by-index write doesn't break the
// ordering guarantee.
func TestClient_GenerateBatchEmbeddings_TruncatesAndPreservesShuffledOrder(t *testing.T) {
	const respDim = 4096
	const wantDim = 768

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req embeddingRequest
		_ = json.Unmarshal(body, &req)
		// Build response items in REVERSE slice order but with correct `index`
		// values, so the adapter must reorder by index after truncating.
		data := make([]map[string]interface{}, len(req.Input))
		for i, s := range req.Input {
			vec := make([]float64, respDim)
			vec[0] = float64(len(s)) // marker to verify post-truncate ordering
			for j := 1; j < respDim; j++ {
				vec[j] = math.Sin(float64(j) * 0.001)
			}
			data[len(req.Input)-1-i] = map[string]interface{}{
				"object":    "embedding",
				"index":     i,
				"embedding": vec,
			}
		}
		resp, _ := json.Marshal(map[string]interface{}{
			"object": "list", "data": data, "model": "octen",
			"usage": map[string]int{"prompt_tokens": 1, "total_tokens": 1},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(resp)
	}))
	t.Cleanup(srv.Close)

	c := NewClient(config.OpenAIConfig{
		APIKey: "sk-test", BaseURL: srv.URL, Model: "octen",
		Dimensions: wantDim, BatchSize: 10, Timeout: 5 * time.Second,
		TruncateDimensions: true,
	})

	inputs := []string{"a", "bb", "ccc", "dddd"}
	out, err := c.GenerateBatchEmbeddings(context.Background(), inputs, outbound.EmbeddingOptions{})
	if err != nil {
		t.Fatalf("GenerateBatchEmbeddings: %v", err)
	}
	for i, in := range inputs {
		if len(out[i].Vector) != wantDim {
			t.Errorf("result[%d] dim = %d, want %d", i, len(out[i].Vector), wantDim)
		}
		// The marker was len(in) divided by the same renorm scale across
		// all items, so coord 0 must be strictly increasing in input length.
		if i > 0 && out[i].Vector[0] <= out[i-1].Vector[0] {
			t.Errorf("ordering broken: result[%d].Vector[0]=%v <= result[%d].Vector[0]=%v (input %q vs %q)",
				i, out[i].Vector[0], i-1, out[i-1].Vector[0], in, inputs[i-1])
		}
	}
}
