package config

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// validBaseConfig returns a Config with the minimum required fields set so
// that Config.Validate() will not trip on unrelated checks while we are
// testing embedding-config behavior.
func validBaseConfig() Config {
	return Config{
		Database: DatabaseConfig{
			User: "x",
			Name: "x",
			Port: 5432,
		},
		Worker: WorkerConfig{Concurrency: 1},
	}
}

func TestEmbeddingConfig_DefaultsProviderToGemini(t *testing.T) {
	cfg := validBaseConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned unexpected error: %v", err)
	}
	if cfg.Embedding.Provider != "gemini" {
		t.Errorf("expected provider to default to %q, got %q", "gemini", cfg.Embedding.Provider)
	}
}

func TestEmbeddingConfig_GeminiProviderDoesNotRequireOpenAIFields(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Embedding = EmbeddingConfig{Provider: "gemini"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate returned unexpected error for gemini provider: %v", err)
	}
}

func TestEmbeddingConfig_OpenAIRejectsBadDimensions(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Embedding = EmbeddingConfig{
		Provider: "openai",
		OpenAI: OpenAIConfig{
			APIKey:     "sk-test",
			Model:      "text-embedding-3-small",
			Dimensions: 1536,
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for dimensions=1536, got nil")
	}
	if !strings.Contains(err.Error(), "768") {
		t.Errorf("expected error to mention 768, got %q", err.Error())
	}
}

func TestEmbeddingConfig_OpenAIRejectsAda002(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Embedding = EmbeddingConfig{
		Provider: "openai",
		OpenAI: OpenAIConfig{
			APIKey:     "sk-test",
			Model:      "text-embedding-ada-002",
			Dimensions: 768,
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for text-embedding-ada-002, got nil")
	}
	if !strings.Contains(err.Error(), "ada-002") {
		t.Errorf("expected error to mention ada-002, got %q", err.Error())
	}
}

func TestEmbeddingConfig_OpenAIRequiresAPIKey(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Embedding = EmbeddingConfig{
		Provider: "openai",
		OpenAI: OpenAIConfig{
			Model:      "text-embedding-3-small",
			Dimensions: 768,
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing api_key, got nil")
	}
	if !strings.Contains(err.Error(), "api_key") {
		t.Errorf("expected error to mention api_key, got %q", err.Error())
	}
}

// TestEmbeddingConfig_OpenAIAllowsEmptyAPIKeyForCustomBaseURL covers the
// no-auth self-hosted case (vLLM, Ollama, LM Studio): when BaseURL is set
// to a non-default host, an empty api_key must validate cleanly. This
// matches the documented intent of the OpenAIConfig struct and the
// client's runtime behavior, which already skips the Authorization
// header when apiKey == "".
// TestEmbeddingConfig_OpenAIRequiresAPIKey_OpenAIHost guards against bypassing
// the api_key requirement by using any URL that targets api.openai.com —
// trailing slashes, extra path segments, or the full endpoint path all still
// point at the real OpenAI service and must require an api_key.
func TestEmbeddingConfig_OpenAIRequiresAPIKey_OpenAIHost(t *testing.T) {
	t.Parallel()
	cases := []string{
		"https://api.openai.com/v1/",
		"https://api.openai.com/v1//",
		"https://api.openai.com/v1/embeddings",
		"https://api.openai.com/v1/embeddings/",
	}
	for _, baseURL := range cases {
		t.Run(baseURL, func(t *testing.T) {
			t.Parallel()
			cfg := validBaseConfig()
			cfg.Embedding = EmbeddingConfig{
				Provider: "openai",
				OpenAI: OpenAIConfig{
					BaseURL:    baseURL,
					Model:      "text-embedding-3-small",
					Dimensions: 768,
				},
			}
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected api_key validation error for base_url=%q, got nil", baseURL)
			}
			if !strings.Contains(err.Error(), "api_key") {
				t.Errorf("expected error to mention api_key for base_url=%q, got %q", baseURL, err.Error())
			}
		})
	}
}

func TestEmbeddingConfig_OpenAIAllowsEmptyAPIKeyForCustomBaseURL(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Embedding = EmbeddingConfig{
		Provider: "openai",
		OpenAI: OpenAIConfig{
			BaseURL:    "http://localhost:11434/v1", // Ollama-style
			Model:      "nomic-embed-text",
			Dimensions: 768,
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected empty api_key to be allowed for custom base_url, got %v", err)
	}
}

func TestEmbeddingConfig_OpenAIRequiresModel(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Embedding = EmbeddingConfig{
		Provider: "openai",
		OpenAI: OpenAIConfig{
			APIKey:     "sk-test",
			Dimensions: 768,
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing model, got nil")
	}
	if !strings.Contains(err.Error(), "model") {
		t.Errorf("expected error to mention model, got %q", err.Error())
	}
}

func TestEmbeddingConfig_OpenAIRejectsUnknownProvider(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Embedding = EmbeddingConfig{Provider: "claude"}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for unknown provider, got nil")
	}
	if !strings.Contains(err.Error(), "claude") {
		t.Errorf("expected error to mention the unknown provider name, got %q", err.Error())
	}
}

func TestEmbeddingConfig_OpenAIDefaultsBaseURL(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Embedding = EmbeddingConfig{
		Provider: "openai",
		OpenAI: OpenAIConfig{
			APIKey:     "sk-test",
			Model:      "text-embedding-3-small",
			Dimensions: 768,
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned unexpected error: %v", err)
	}
	want := "https://api.openai.com/v1"
	if cfg.Embedding.OpenAI.BaseURL != want {
		t.Errorf("expected BaseURL default %q, got %q", want, cfg.Embedding.OpenAI.BaseURL)
	}
}

func TestEmbeddingConfig_OpenAIDefaultsBatchSize(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Embedding = EmbeddingConfig{
		Provider: "openai",
		OpenAI: OpenAIConfig{
			APIKey:     "sk-test",
			Model:      "text-embedding-3-small",
			Dimensions: 768,
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned unexpected error: %v", err)
	}
	if got, want := cfg.Embedding.OpenAI.BatchSize, 256; got != want {
		t.Errorf("expected BatchSize default %d, got %d", want, got)
	}
}

func TestEmbeddingConfig_OpenAIDefaultsMaxRetries(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Embedding = EmbeddingConfig{
		Provider: "openai",
		OpenAI: OpenAIConfig{
			APIKey:     "sk-test",
			Model:      "text-embedding-3-small",
			Dimensions: 768,
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned unexpected error: %v", err)
	}
	if got, want := cfg.Embedding.OpenAI.MaxRetries, 3; got != want {
		t.Errorf("expected MaxRetries default %d, got %d", want, got)
	}
}

func TestEmbeddingConfig_OpenAIPreservesUserMaxRetries(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Embedding = EmbeddingConfig{
		Provider: "openai",
		OpenAI: OpenAIConfig{
			APIKey:     "sk-test",
			Model:      "text-embedding-3-small",
			Dimensions: 768,
			MaxRetries: 7,
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned unexpected error: %v", err)
	}
	if got, want := cfg.Embedding.OpenAI.MaxRetries, 7; got != want {
		t.Errorf("expected MaxRetries preserved as %d, got %d", want, got)
	}
}

func TestEmbeddingConfig_OpenAIRejectsBatchSizeAbove2048(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Embedding = EmbeddingConfig{
		Provider: "openai",
		OpenAI: OpenAIConfig{
			APIKey:     "sk-test",
			Model:      "text-embedding-3-small",
			Dimensions: 768,
			BatchSize:  3000,
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for batch_size > 2048, got nil")
	}
	if !strings.Contains(err.Error(), "batch_size") {
		t.Errorf("expected error to mention batch_size, got %q", err.Error())
	}
}

// TestEmbeddingConfig_OpenAIRejectsNegatives guards against negative numeric
// values silently passing validation. The current `== 0` defaulting branches
// would let a negative value reach runtime, where it would (a) disable HTTP
// timeouts (negative Timeout), (b) break GenerateBatchEmbeddings's chunking
// loop (negative BatchSize), or (c) misconfigure the retry budget (negative
// MaxRetries). The contract: any negative numeric must return a clear error
// naming the offending field.
func TestEmbeddingConfig_OpenAIRejectsNegatives(t *testing.T) {
	t.Parallel()
	base := func() OpenAIConfig {
		return OpenAIConfig{
			APIKey:     "sk-test",
			Model:      "text-embedding-3-small",
			Dimensions: 768,
		}
	}
	cases := []struct {
		name      string
		mutate    func(*OpenAIConfig)
		wantField string
	}{
		{
			name:      "negative_batch_size",
			mutate:    func(o *OpenAIConfig) { o.BatchSize = -1 },
			wantField: "batch_size",
		},
		{
			name:      "negative_max_retries",
			mutate:    func(o *OpenAIConfig) { o.MaxRetries = -1 },
			wantField: "max_retries",
		},
		{
			name:      "negative_timeout",
			mutate:    func(o *OpenAIConfig) { o.Timeout = -1 * time.Second },
			wantField: "timeout",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := validBaseConfig()
			oa := base()
			tc.mutate(&oa)
			cfg.Embedding = EmbeddingConfig{Provider: "openai", OpenAI: oa}
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected validation error for %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantField) {
				t.Errorf("expected error to mention %q, got %q", tc.wantField, err.Error())
			}
		})
	}
}

func TestEmbeddingConfig_OpenAIPreservesUserBaseURL(t *testing.T) {
	cfg := validBaseConfig()
	custom := "http://localhost:11434/v1"
	cfg.Embedding = EmbeddingConfig{
		Provider: "openai",
		OpenAI: OpenAIConfig{
			APIKey:     "sk-test",
			Model:      "text-embedding-3-small",
			Dimensions: 768,
			BaseURL:    custom,
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned unexpected error: %v", err)
	}
	if cfg.Embedding.OpenAI.BaseURL != custom {
		t.Errorf("expected user-provided BaseURL %q to be preserved, got %q", custom, cfg.Embedding.OpenAI.BaseURL)
	}
}

func TestOpenAIConfig_TruncateDimensionsDefaultsFalse(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Embedding = EmbeddingConfig{
		Provider: "openai",
		OpenAI: OpenAIConfig{
			APIKey:     "sk-test",
			Model:      "text-embedding-3-small",
			Dimensions: 768,
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate returned unexpected error: %v", err)
	}
	if cfg.Embedding.OpenAI.TruncateDimensions {
		t.Error("expected TruncateDimensions to default to false for backward compat")
	}
}

func TestOpenAIConfig_TruncateDimensionsRespectsExplicitTrue(t *testing.T) {
	yamlContent := `
embedding:
  provider: openai
  openai:
    api_key: sk-test
    model: text-embedding-3-small
    dimensions: 768
    truncate_dimensions: true
`
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(yamlContent)); err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	// Fill the unrelated minimum-required fields so Validate() doesn't trip
	// on something orthogonal to the truncate-dimensions flag.
	cfg.Database = DatabaseConfig{User: "x", Name: "x", Port: 5432}
	cfg.Worker = WorkerConfig{Concurrency: 1}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate must still pass with TruncateDimensions=true: %v", err)
	}
	if !cfg.Embedding.OpenAI.TruncateDimensions {
		t.Error("expected TruncateDimensions=true to round-trip from YAML")
	}
	if cfg.Embedding.OpenAI.Dimensions != 768 {
		t.Errorf("schema-invariant Dimensions=768 must still hold, got %d", cfg.Embedding.OpenAI.Dimensions)
	}
}

func TestEmbeddingConfig_LoadsFromYAML(t *testing.T) {
	yamlContent := `
embedding:
  provider: openai
  openai:
    api_key: sk-test
    base_url: https://example.com/v1
    model: text-embedding-3-small
    dimensions: 768
    max_retries: 5
    timeout: 45s
    batch_size: 128
`
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(yamlContent)); err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Embedding.Provider != "openai" {
		t.Errorf("expected provider openai, got %q", cfg.Embedding.Provider)
	}
	if cfg.Embedding.OpenAI.APIKey != "sk-test" {
		t.Errorf("expected api_key sk-test, got %q", cfg.Embedding.OpenAI.APIKey)
	}
	if cfg.Embedding.OpenAI.BaseURL != "https://example.com/v1" {
		t.Errorf("expected base_url https://example.com/v1, got %q", cfg.Embedding.OpenAI.BaseURL)
	}
	if cfg.Embedding.OpenAI.Model != "text-embedding-3-small" {
		t.Errorf("expected model text-embedding-3-small, got %q", cfg.Embedding.OpenAI.Model)
	}
	if cfg.Embedding.OpenAI.Dimensions != 768 {
		t.Errorf("expected dimensions 768, got %d", cfg.Embedding.OpenAI.Dimensions)
	}
	if cfg.Embedding.OpenAI.MaxRetries != 5 {
		t.Errorf("expected max_retries 5, got %d", cfg.Embedding.OpenAI.MaxRetries)
	}
	if cfg.Embedding.OpenAI.Timeout != 45*time.Second {
		t.Errorf("expected timeout 45s, got %v", cfg.Embedding.OpenAI.Timeout)
	}
	if cfg.Embedding.OpenAI.BatchSize != 128 {
		t.Errorf("expected batch_size 128, got %d", cfg.Embedding.OpenAI.BatchSize)
	}
}
