//go:build live

package openai_test

import (
	"codechunking/internal/adapter/outbound/openai"
	"codechunking/internal/config"
	"codechunking/internal/port/outbound"
	"context"
	"math"
	"os"
	"testing"
	"time"
)

func liveCfg(t *testing.T) config.OpenAIConfig {
	t.Helper()
	base := os.Getenv("LMSTUDIO_URL")
	model := os.Getenv("LMSTUDIO_MODEL")
	if base == "" || model == "" {
		t.Skip("set LMSTUDIO_URL and LMSTUDIO_MODEL to run live test")
	}
	return config.OpenAIConfig{
		APIKey:     "lm-studio",
		BaseURL:    base,
		Model:      model,
		Dimensions: 0,
		BatchSize:  4,
		MaxRetries: 0,
		Timeout:    60 * time.Second,
	}
}

func TestLive_GenerateEmbedding(t *testing.T) {
	c := openai.NewClient(liveCfg(t))
	res, err := c.GenerateEmbedding(context.Background(), "hello world", outbound.EmbeddingOptions{})
	if err != nil {
		t.Fatalf("GenerateEmbedding: %v", err)
	}
	if res.Dimensions == 0 || len(res.Vector) != res.Dimensions {
		t.Fatalf("vector/dimensions mismatch: dim=%d len=%d", res.Dimensions, len(res.Vector))
	}
	if res.Model == "" {
		t.Fatalf("expected model echoed in response")
	}
	t.Logf("single: model=%q dim=%d tokens=%d", res.Model, res.Dimensions, res.TokenCount)
}

func TestLive_GenerateBatchEmbeddings(t *testing.T) {
	c := openai.NewClient(liveCfg(t))
	inputs := []string{
		"package main",
		"func add(a, b int) int { return a + b }",
		"the quick brown fox",
		"semantic search over code",
		"five",
		"six",
	}
	out, err := c.GenerateBatchEmbeddings(context.Background(), inputs, outbound.EmbeddingOptions{})
	if err != nil {
		t.Fatalf("GenerateBatchEmbeddings: %v", err)
	}
	if len(out) != len(inputs) {
		t.Fatalf("want %d results, got %d", len(inputs), len(out))
	}
	want := out[0].Dimensions
	if want == 0 {
		t.Fatalf("first result has zero dim")
	}
	for i, r := range out {
		if r == nil {
			t.Fatalf("result %d is nil (gap in ordered slice)", i)
		}
		if r.Dimensions != want || len(r.Vector) != want {
			t.Fatalf("result %d: dim=%d len=%d want %d", i, r.Dimensions, len(r.Vector), want)
		}
	}
	t.Logf("batch: count=%d dim=%d", len(out), want)
}

// TestLive_TruncateMRL exercises the opt-in client-side prefix-truncate +
// L2-renormalize path against a real MRL-trained model behind a server that
// ignores the `dimensions` request parameter (e.g. LM Studio + Octen-8B).
// The strict-equality default would fail this hand-off; the flag is the
// supported escape hatch.
func TestLive_TruncateMRL(t *testing.T) {
	cfg := liveCfg(t)
	cfg.Dimensions = 768
	cfg.TruncateDimensions = true
	c := openai.NewClient(cfg)

	res, err := c.GenerateEmbedding(context.Background(), "hello world", outbound.EmbeddingOptions{})
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
	if math.Abs(math.Sqrt(sumSq)-1.0) > 1e-5 {
		t.Errorf("L2 norm = %v, want 1.0 (within 1e-5)", math.Sqrt(sumSq))
	}
	t.Logf("truncate-MRL: dim=%d norm≈1.0 tokens=%d", res.Dimensions, res.TokenCount)
}
