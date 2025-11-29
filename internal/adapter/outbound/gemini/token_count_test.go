package gemini_test

import (
	"codechunking/internal/adapter/outbound/gemini"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestClient_CountTokens tests single text token counting functionality.
func TestClient_CountTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupFunc func(t *testing.T) (*gemini.Client, context.Context)
		text      string
		model     string
		wantErr   bool
		checkFunc func(t *testing.T, result *outbound.TokenCountResult, err error)
	}{
		{
			name: "valid_text_returns_token_count",
			setupFunc: func(t *testing.T) (*gemini.Client, context.Context) {
				client, err := gemini.NewClient(&gemini.ClientConfig{
					APIKey:     "test-api-key-" + t.Name(),
					Model:      "gemini-embedding-001",
					TaskType:   "RETRIEVAL_DOCUMENT",
					Timeout:    30 * time.Second,
					Dimensions: 768,
				})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}
				return client, context.Background()
			},
			text:    "This is a test string for token counting",
			model:   "gemini-embedding-001",
			wantErr: true, // RED PHASE: Not implemented yet
			checkFunc: func(t *testing.T, result *outbound.TokenCountResult, err error) {
				// RED PHASE: Should fail because CountTokens is not implemented
				if err == nil {
					t.Error("RED PHASE: expected error (not implemented), got nil - implementation exists!")
					return
				}

				// When implemented (GREEN PHASE), verify:
				// - result is not nil
				// - result.TotalTokens > 0
				// - result.Model matches requested model
				// - result.CachedAt is nil for fresh count
			},
		},
		{
			name: "empty_text_returns_error",
			setupFunc: func(t *testing.T) (*gemini.Client, context.Context) {
				client, err := gemini.NewClient(&gemini.ClientConfig{
					APIKey:     "test-api-key-" + t.Name(),
					Model:      "gemini-embedding-001",
					TaskType:   "RETRIEVAL_DOCUMENT",
					Timeout:    30 * time.Second,
					Dimensions: 768,
				})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}
				return client, context.Background()
			},
			text:    "",
			model:   "gemini-embedding-001",
			wantErr: true,
			checkFunc: func(t *testing.T, result *outbound.TokenCountResult, err error) {
				if err == nil {
					t.Error("expected error for empty text, got nil")
					return
				}
				if result != nil {
					t.Errorf("expected nil result for empty text, got %+v", result)
				}

				// Should be a validation error
				var embErr *outbound.EmbeddingError
				if errors.As(err, &embErr) {
					if !embErr.IsValidationError() {
						t.Errorf("expected validation error, got type=%s", embErr.Type)
					}
				}
			},
		},
		{
			name: "whitespace_only_text_returns_error",
			setupFunc: func(t *testing.T) (*gemini.Client, context.Context) {
				client, err := gemini.NewClient(&gemini.ClientConfig{
					APIKey:     "test-api-key-" + t.Name(),
					Model:      "gemini-embedding-001",
					TaskType:   "RETRIEVAL_DOCUMENT",
					Timeout:    30 * time.Second,
					Dimensions: 768,
				})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}
				return client, context.Background()
			},
			text:    "   \t\n  ",
			model:   "gemini-embedding-001",
			wantErr: true,
			checkFunc: func(t *testing.T, result *outbound.TokenCountResult, err error) {
				if err == nil {
					t.Error("expected error for whitespace-only text, got nil")
					return
				}
				if result != nil {
					t.Errorf("expected nil result for whitespace-only text, got %+v", result)
				}
			},
		},
		{
			name: "context_cancellation_returns_error",
			setupFunc: func(t *testing.T) (*gemini.Client, context.Context) {
				client, err := gemini.NewClient(&gemini.ClientConfig{
					APIKey:     "test-api-key-" + t.Name(),
					Model:      "gemini-embedding-001",
					TaskType:   "RETRIEVAL_DOCUMENT",
					Timeout:    30 * time.Second,
					Dimensions: 768,
				})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately
				return client, ctx
			},
			text:    "Test text",
			model:   "gemini-embedding-001",
			wantErr: true,
			checkFunc: func(t *testing.T, result *outbound.TokenCountResult, err error) {
				if err == nil {
					t.Error("expected error for cancelled context, got nil")
					return
				}
				// When implemented, should return context.Canceled error
			},
		},
		{
			name: "empty_model_uses_default",
			setupFunc: func(t *testing.T) (*gemini.Client, context.Context) {
				client, err := gemini.NewClient(&gemini.ClientConfig{
					APIKey:     "test-api-key-" + t.Name(),
					Model:      "gemini-embedding-001",
					TaskType:   "RETRIEVAL_DOCUMENT",
					Timeout:    30 * time.Second,
					Dimensions: 768,
				})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}
				return client, context.Background()
			},
			text:    "Test text",
			model:   "",   // Empty model should use default
			wantErr: true, // RED PHASE: Not implemented yet
			checkFunc: func(t *testing.T, result *outbound.TokenCountResult, err error) {
				// RED PHASE: Should fail because CountTokens is not implemented
				if err == nil {
					t.Error("RED PHASE: expected error (not implemented), got nil - implementation exists!")
					return
				}

				// When implemented (GREEN PHASE), verify:
				// - result.Model should be "gemini-embedding-001" (default)
			},
		},
		{
			name: "long_text_returns_higher_token_count",
			setupFunc: func(t *testing.T) (*gemini.Client, context.Context) {
				client, err := gemini.NewClient(&gemini.ClientConfig{
					APIKey:     "test-api-key-" + t.Name(),
					Model:      "gemini-embedding-001",
					TaskType:   "RETRIEVAL_DOCUMENT",
					Timeout:    30 * time.Second,
					Dimensions: 768,
				})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}
				return client, context.Background()
			},
			text:    strings.Repeat("This is a longer text that should have many tokens. ", 100),
			model:   "gemini-embedding-001",
			wantErr: true, // RED PHASE: Not implemented yet
			checkFunc: func(t *testing.T, result *outbound.TokenCountResult, err error) {
				// RED PHASE: Should fail because CountTokens is not implemented
				if err == nil {
					t.Error("RED PHASE: expected error (not implemented), got nil - implementation exists!")
					return
				}

				// When implemented (GREEN PHASE), verify:
				// - result.TotalTokens should be significantly higher (e.g., > 100)
			},
		},
		{
			name: "timeout_context_returns_error",
			setupFunc: func(t *testing.T) (*gemini.Client, context.Context) {
				client, err := gemini.NewClient(&gemini.ClientConfig{
					APIKey:     "test-api-key-" + t.Name(),
					Model:      "gemini-embedding-001",
					TaskType:   "RETRIEVAL_DOCUMENT",
					Timeout:    30 * time.Second,
					Dimensions: 768,
				})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
				defer cancel()
				time.Sleep(10 * time.Millisecond) // Ensure timeout
				return client, ctx
			},
			text:    "Test text",
			model:   "gemini-embedding-001",
			wantErr: true,
			checkFunc: func(t *testing.T, result *outbound.TokenCountResult, err error) {
				if err == nil {
					t.Error("expected error for timeout context, got nil")
					return
				}
				// When implemented, should return context.DeadlineExceeded error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, ctx := tt.setupFunc(t)

			result, err := client.CountTokens(ctx, tt.text, tt.model)

			if (err != nil) != tt.wantErr {
				t.Errorf("CountTokens() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, result, err)
			}
		})
	}
}

// TestClient_CountTokensBatch tests batch token counting functionality.
func TestClient_CountTokensBatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupFunc func(t *testing.T) (*gemini.Client, context.Context)
		texts     []string
		model     string
		wantErr   bool
		checkFunc func(t *testing.T, results []*outbound.TokenCountResult, err error)
	}{
		{
			name: "valid_texts_returns_matching_results",
			setupFunc: func(t *testing.T) (*gemini.Client, context.Context) {
				client, err := gemini.NewClient(&gemini.ClientConfig{
					APIKey:     "test-api-key-" + t.Name(),
					Model:      "gemini-embedding-001",
					TaskType:   "RETRIEVAL_DOCUMENT",
					Timeout:    30 * time.Second,
					Dimensions: 768,
				})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}
				return client, context.Background()
			},
			texts: []string{
				"First text for batch counting",
				"Second text for batch counting",
				"Third text for batch counting",
			},
			model:   "gemini-embedding-001",
			wantErr: true, // RED PHASE: Not implemented yet
			checkFunc: func(t *testing.T, results []*outbound.TokenCountResult, err error) {
				// RED PHASE: Should fail because CountTokensBatch is not implemented
				if err == nil {
					t.Error("RED PHASE: expected error (not implemented), got nil - implementation exists!")
					return
				}

				// When implemented (GREEN PHASE), verify:
				// - results length matches input length (3)
				// - each result has TotalTokens > 0
				// - each result has Model set
				// - order is preserved
			},
		},
		{
			name: "empty_slice_returns_error",
			setupFunc: func(t *testing.T) (*gemini.Client, context.Context) {
				client, err := gemini.NewClient(&gemini.ClientConfig{
					APIKey:     "test-api-key-" + t.Name(),
					Model:      "gemini-embedding-001",
					TaskType:   "RETRIEVAL_DOCUMENT",
					Timeout:    30 * time.Second,
					Dimensions: 768,
				})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}
				return client, context.Background()
			},
			texts:   []string{},
			model:   "gemini-embedding-001",
			wantErr: true,
			checkFunc: func(t *testing.T, results []*outbound.TokenCountResult, err error) {
				if err == nil {
					t.Error("expected error for empty slice, got nil")
					return
				}
				if results != nil {
					t.Errorf("expected nil results for empty slice, got %+v", results)
				}

				// Should be a validation error
				var embErr *outbound.EmbeddingError
				if errors.As(err, &embErr) {
					if !embErr.IsValidationError() {
						t.Errorf("expected validation error, got type=%s", embErr.Type)
					}
				}
			},
		},
		{
			name: "nil_slice_returns_error",
			setupFunc: func(t *testing.T) (*gemini.Client, context.Context) {
				client, err := gemini.NewClient(&gemini.ClientConfig{
					APIKey:     "test-api-key-" + t.Name(),
					Model:      "gemini-embedding-001",
					TaskType:   "RETRIEVAL_DOCUMENT",
					Timeout:    30 * time.Second,
					Dimensions: 768,
				})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}
				return client, context.Background()
			},
			texts:   nil,
			model:   "gemini-embedding-001",
			wantErr: true,
			checkFunc: func(t *testing.T, results []*outbound.TokenCountResult, err error) {
				if err == nil {
					t.Error("expected error for nil slice, got nil")
					return
				}
			},
		},
		{
			name: "single_text_in_batch",
			setupFunc: func(t *testing.T) (*gemini.Client, context.Context) {
				client, err := gemini.NewClient(&gemini.ClientConfig{
					APIKey:     "test-api-key-" + t.Name(),
					Model:      "gemini-embedding-001",
					TaskType:   "RETRIEVAL_DOCUMENT",
					Timeout:    30 * time.Second,
					Dimensions: 768,
				})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}
				return client, context.Background()
			},
			texts:   []string{"Single text for batch processing"},
			model:   "gemini-embedding-001",
			wantErr: true, // RED PHASE: Not implemented yet
			checkFunc: func(t *testing.T, results []*outbound.TokenCountResult, err error) {
				// RED PHASE: Should fail because CountTokensBatch is not implemented
				if err == nil {
					t.Error("RED PHASE: expected error (not implemented), got nil - implementation exists!")
					return
				}

				// When implemented (GREEN PHASE), verify:
				// - results length is 1
				// - result has valid token count
			},
		},
		{
			name: "mixed_empty_and_valid_texts",
			setupFunc: func(t *testing.T) (*gemini.Client, context.Context) {
				client, err := gemini.NewClient(&gemini.ClientConfig{
					APIKey:     "test-api-key-" + t.Name(),
					Model:      "gemini-embedding-001",
					TaskType:   "RETRIEVAL_DOCUMENT",
					Timeout:    30 * time.Second,
					Dimensions: 768,
				})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}
				return client, context.Background()
			},
			texts: []string{
				"Valid text",
				"",
				"Another valid text",
			},
			model:   "gemini-embedding-001",
			wantErr: true, // Should handle gracefully or return error
			checkFunc: func(t *testing.T, results []*outbound.TokenCountResult, err error) {
				// Implementation should decide how to handle mixed content:
				// Option 1: Return error for entire batch
				// Option 2: Return nil for invalid entries
				// Option 3: Skip empty entries and return partial results

				// For now, just verify it handles the case
				if err == nil && results != nil {
					t.Log("Implementation handles mixed content - verify behavior")
				}
			},
		},
		{
			name: "order_preserved_in_results",
			setupFunc: func(t *testing.T) (*gemini.Client, context.Context) {
				client, err := gemini.NewClient(&gemini.ClientConfig{
					APIKey:     "test-api-key-" + t.Name(),
					Model:      "gemini-embedding-001",
					TaskType:   "RETRIEVAL_DOCUMENT",
					Timeout:    30 * time.Second,
					Dimensions: 768,
				})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}
				return client, context.Background()
			},
			texts: []string{
				"A short text",
				"B this is a much longer text with many more tokens to count properly",
				"C medium length text",
			},
			model:   "gemini-embedding-001",
			wantErr: true, // RED PHASE: Not implemented yet
			checkFunc: func(t *testing.T, results []*outbound.TokenCountResult, err error) {
				// RED PHASE: Should fail because CountTokensBatch is not implemented
				if err == nil {
					t.Error("RED PHASE: expected error (not implemented), got nil - implementation exists!")
					return
				}

				// When implemented (GREEN PHASE), verify:
				// - results[0] corresponds to "A short text"
				// - results[1] corresponds to "B this is..." (highest token count)
				// - results[2] corresponds to "C medium..." (middle token count)
				// - Order matches input order exactly
			},
		},
		{
			name: "context_cancellation_returns_error",
			setupFunc: func(t *testing.T) (*gemini.Client, context.Context) {
				client, err := gemini.NewClient(&gemini.ClientConfig{
					APIKey:     "test-api-key-" + t.Name(),
					Model:      "gemini-embedding-001",
					TaskType:   "RETRIEVAL_DOCUMENT",
					Timeout:    30 * time.Second,
					Dimensions: 768,
				})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately
				return client, ctx
			},
			texts: []string{
				"Text 1",
				"Text 2",
			},
			model:   "gemini-embedding-001",
			wantErr: true,
			checkFunc: func(t *testing.T, results []*outbound.TokenCountResult, err error) {
				if err == nil {
					t.Error("expected error for cancelled context, got nil")
					return
				}
				// When implemented, should return context.Canceled error
			},
		},
		{
			name: "varying_text_lengths_produce_different_counts",
			setupFunc: func(t *testing.T) (*gemini.Client, context.Context) {
				client, err := gemini.NewClient(&gemini.ClientConfig{
					APIKey:     "test-api-key-" + t.Name(),
					Model:      "gemini-embedding-001",
					TaskType:   "RETRIEVAL_DOCUMENT",
					Timeout:    30 * time.Second,
					Dimensions: 768,
				})
				if err != nil {
					t.Fatalf("failed to create client: %v", err)
				}
				return client, context.Background()
			},
			texts: []string{
				"Short",
				"This is a much longer piece of text that should definitely have more tokens than the short one",
			},
			model:   "gemini-embedding-001",
			wantErr: true, // RED PHASE: Not implemented yet
			checkFunc: func(t *testing.T, results []*outbound.TokenCountResult, err error) {
				// RED PHASE: Should fail because CountTokensBatch is not implemented
				if err == nil {
					t.Error("RED PHASE: expected error (not implemented), got nil - implementation exists!")
					return
				}

				// When implemented (GREEN PHASE), verify:
				// - results[1].TotalTokens > results[0].TotalTokens
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, ctx := tt.setupFunc(t)

			results, err := client.CountTokensBatch(ctx, tt.texts, tt.model)

			if (err != nil) != tt.wantErr {
				t.Errorf("CountTokensBatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, results, err)
			}
		})
	}
}

// TestTokenCountResult_FieldValidation validates TokenCountResult structure.
func TestTokenCountResult_FieldValidation(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name      string
		result    *outbound.TokenCountResult
		checkFunc func(t *testing.T, result *outbound.TokenCountResult)
	}{
		{
			name: "all_fields_set_correctly",
			result: &outbound.TokenCountResult{
				TotalTokens: 42,
				Model:       "gemini-embedding-001",
				CachedAt:    &now,
			},
			checkFunc: func(t *testing.T, result *outbound.TokenCountResult) {
				if result.TotalTokens != 42 {
					t.Errorf("expected TotalTokens=42, got=%d", result.TotalTokens)
				}
				if result.Model != "gemini-embedding-001" {
					t.Errorf("expected Model='gemini-embedding-001', got='%s'", result.Model)
				}
				if result.CachedAt == nil {
					t.Error("expected CachedAt to be set, got nil")
				}
				if !result.CachedAt.Equal(now) {
					t.Errorf("expected CachedAt=%v, got=%v", now, *result.CachedAt)
				}
			},
		},
		{
			name: "cached_at_can_be_nil",
			result: &outbound.TokenCountResult{
				TotalTokens: 100,
				Model:       "test-model",
				CachedAt:    nil,
			},
			checkFunc: func(t *testing.T, result *outbound.TokenCountResult) {
				if result.CachedAt != nil {
					t.Error("expected CachedAt to be nil, got value")
				}
				if result.TotalTokens != 100 {
					t.Errorf("expected TotalTokens=100, got=%d", result.TotalTokens)
				}
			},
		},
		{
			name: "zero_tokens_is_valid",
			result: &outbound.TokenCountResult{
				TotalTokens: 0,
				Model:       "test-model",
				CachedAt:    nil,
			},
			checkFunc: func(t *testing.T, result *outbound.TokenCountResult) {
				if result.TotalTokens != 0 {
					t.Errorf("expected TotalTokens=0, got=%d", result.TotalTokens)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.checkFunc != nil {
				tt.checkFunc(t, tt.result)
			}
		})
	}
}
