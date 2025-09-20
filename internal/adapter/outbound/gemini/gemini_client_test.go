package gemini_test

import (
	"codechunking/internal/adapter/outbound/gemini"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"
)

// TestGeminiClient_HTTPClientStructWithConfiguration tests the HTTP client struct configuration.
func TestGeminiClient_HTTPClientStructWithConfiguration(t *testing.T) {
	tests := []struct {
		name           string
		config         *gemini.ClientConfig
		expectedError  string
		expectedFields map[string]interface{}
	}{
		{
			name: "valid_default_configuration",
			config: &gemini.ClientConfig{
				APIKey:     "test-api-key",
				Model:      "gemini-embedding-001",
				TaskType:   "RETRIEVAL_DOCUMENT",
				Timeout:    30 * time.Second,
				Dimensions: 768,
			},
			expectedFields: map[string]interface{}{
				"APIKey":     "test-api-key",
				"Model":      "gemini-embedding-001",
				"TaskType":   "RETRIEVAL_DOCUMENT",
				"Timeout":    30 * time.Second,
				"Dimensions": 768,
			},
		},
		{
			name: "valid_custom_configuration",
			config: &gemini.ClientConfig{
				APIKey:     "custom-api-key",
				Model:      "gemini-embedding-001",
				TaskType:   "SEMANTIC_SIMILARITY",
				Timeout:    60 * time.Second,
				Dimensions: 768,
			},
			expectedFields: map[string]interface{}{
				"APIKey":     "custom-api-key",
				"Model":      "gemini-embedding-001",
				"TaskType":   "SEMANTIC_SIMILARITY",
				"Timeout":    60 * time.Second,
				"Dimensions": 768,
			},
		},
		{
			name: "missing_api_key",
			config: &gemini.ClientConfig{
				Model:    "gemini-embedding-001",
				TaskType: "RETRIEVAL_DOCUMENT",
				Timeout:  30 * time.Second,
			},
			expectedError: "API key cannot be empty",
		},
		// Removed invalid_base_url test as BaseURL field no longer exists
		{
			name: "invalid_model",
			config: &gemini.ClientConfig{
				APIKey:   "test-api-key",
				Model:    "invalid-model",
				TaskType: "RETRIEVAL_DOCUMENT",
				Timeout:  30 * time.Second,
			},
			expectedError: "unsupported model",
		},
		{
			name: "invalid_task_type",
			config: &gemini.ClientConfig{
				APIKey:   "test-api-key",
				Model:    "gemini-embedding-001",
				TaskType: "INVALID_TASK",
				Timeout:  30 * time.Second,
			},
			expectedError: "unsupported task type",
		},
		{
			name: "negative_timeout",
			config: &gemini.ClientConfig{
				APIKey:   "test-api-key",
				Model:    "gemini-embedding-001",
				TaskType: "RETRIEVAL_DOCUMENT",
				Timeout:  -1 * time.Second,
			},
			expectedError: "timeout must be positive",
		},
		// Removed negative_max_retries test as MaxRetries field no longer exists
		{
			name: "zero_dimensions",
			config: &gemini.ClientConfig{
				APIKey:     "test-api-key",
				Model:      "gemini-embedding-001",
				TaskType:   "RETRIEVAL_DOCUMENT",
				Dimensions: 0, // Should default to 768
				Timeout:    30 * time.Second,
			},
			expectedFields: map[string]interface{}{
				"APIKey":     "test-api-key",
				"Model":      "gemini-embedding-001",
				"TaskType":   "RETRIEVAL_DOCUMENT",
				"Dimensions": 768,
				"Timeout":    30 * time.Second,
			},
		},
		{
			name: "invalid_dimensions",
			config: &gemini.ClientConfig{
				APIKey:     "test-api-key",
				Model:      "gemini-embedding-001",
				TaskType:   "RETRIEVAL_DOCUMENT",
				Dimensions: 512, // Invalid for gemini-embedding-001
				Timeout:    30 * time.Second,
			},
			expectedError: "unsupported dimensions for model gemini-embedding-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test should fail initially because the Client struct doesn't exist
			client, err := gemini.NewClient(tt.config)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expectedError)
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("expected error %q, got %q", tt.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Error("expected non-nil client")
				return
			}

			// Verify client configuration fields
			if tt.expectedFields != nil {
				clientConfig := client.GetConfig()
				for field, expectedValue := range tt.expectedFields {
					// This will fail because GetConfig() method doesn't exist yet
					if !verifyField(clientConfig, field, expectedValue) {
						t.Errorf("field %s: expected %v, got different value", field, expectedValue)
					}
				}
			}
		})
	}
}

// TestGeminiClient_XGoogAPIKeyHeaderAuthentication tests API key header authentication.
func TestGeminiClient_XGoogAPIKeyHeaderAuthentication(t *testing.T) {
	tests := []struct {
		name           string
		apiKey         string
		expectedHeader string
		expectedError  string
	}{
		{
			name:           "valid_api_key",
			apiKey:         "AIzaSyDaGmWKa4JsXZ5HjGAdPQ98nQqcFMnGbE8",
			expectedHeader: "AIzaSyDaGmWKa4JsXZ5HjGAdPQ98nQqcFMnGbE8",
		},
		{
			name:           "api_key_with_special_chars",
			apiKey:         "AIzaSy_123-ABC.def",
			expectedHeader: "AIzaSy_123-ABC.def",
		},
		{
			name:          "empty_api_key",
			apiKey:        "",
			expectedError: "API key cannot be empty",
		},
		{
			name:          "whitespace_api_key",
			apiKey:        "   \t\n   ",
			expectedError: "API key cannot be empty or whitespace",
		},
		{
			name:           "api_key_with_leading_trailing_spaces",
			apiKey:         "  AIzaSyDaGmWKa4JsXZ5HjGAdPQ98nQqcFMnGbE8  ",
			expectedHeader: "AIzaSyDaGmWKa4JsXZ5HjGAdPQ98nQqcFMnGbE8", // Should be trimmed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &gemini.ClientConfig{
				APIKey:   tt.apiKey,
				Model:    "gemini-embedding-001",
				TaskType: "RETRIEVAL_DOCUMENT",
				Timeout:  30 * time.Second,
			}

			// This test should fail initially because the Client doesn't exist
			client, err := gemini.NewClient(config)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expectedError)
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("expected error %q, got %q", tt.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify the client was created successfully with the API key
			clientConfig := client.GetConfig()
			if clientConfig.APIKey != tt.expectedHeader {
				t.Errorf("expected API key %q, got %q", tt.expectedHeader, clientConfig.APIKey)
			}
		})
	}
}

// TestGeminiClient_EnvironmentVariableSupport tests environment variable support.
func TestGeminiClient_EnvironmentVariableSupport(t *testing.T) {
	// Save original environment
	originalGeminiKey := os.Getenv("GEMINI_API_KEY")
	originalGoogleKey := os.Getenv("GOOGLE_API_KEY")

	// Cleanup function
	// Set original values using t.Setenv for automatic cleanup
	if originalGeminiKey != "" {
		t.Setenv("GEMINI_API_KEY", originalGeminiKey)
	}
	if originalGoogleKey != "" {
		t.Setenv("GOOGLE_API_KEY", originalGoogleKey)
	}

	tests := []struct {
		name           string
		geminiAPIKey   string
		googleAPIKey   string
		configAPIKey   string
		expectedAPIKey string
		expectedError  string
		setupEnv       func(t *testing.T)
	}{
		{
			name:           "uses_gemini_api_key_env_var",
			geminiAPIKey:   "env-gemini-key",
			expectedAPIKey: "env-gemini-key",
			setupEnv: func(t *testing.T) {
				t.Setenv("GEMINI_API_KEY", "env-gemini-key")
				t.Setenv("GOOGLE_API_KEY", "")
			},
		},
		{
			name:           "uses_google_api_key_env_var",
			googleAPIKey:   "env-google-key",
			expectedAPIKey: "env-google-key",
			setupEnv: func(t *testing.T) {
				t.Setenv("GEMINI_API_KEY", "")
				t.Setenv("GOOGLE_API_KEY", "env-google-key")
			},
		},
		{
			name:           "gemini_api_key_takes_precedence_over_google_api_key",
			geminiAPIKey:   "gemini-precedence-key",
			googleAPIKey:   "google-fallback-key",
			expectedAPIKey: "gemini-precedence-key",
			setupEnv: func(t *testing.T) {
				t.Setenv("GEMINI_API_KEY", "gemini-precedence-key")
				t.Setenv("GOOGLE_API_KEY", "google-fallback-key")
			},
		},
		{
			name:           "config_api_key_overrides_env_vars",
			geminiAPIKey:   "env-gemini-key",
			configAPIKey:   "config-override-key",
			expectedAPIKey: "config-override-key",
			setupEnv: func(t *testing.T) {
				t.Setenv("GEMINI_API_KEY", "env-gemini-key")
			},
		},
		{
			name:          "no_api_key_provided",
			expectedError: "API key not found in config or environment variables",
			setupEnv: func(t *testing.T) {
				t.Setenv("GEMINI_API_KEY", "")
				t.Setenv("GOOGLE_API_KEY", "")
			},
		},
		{
			name:           "empty_env_vars_fallback_to_config",
			geminiAPIKey:   "",
			googleAPIKey:   "",
			configAPIKey:   "config-fallback-key",
			expectedAPIKey: "config-fallback-key",
			setupEnv: func(t *testing.T) {
				t.Setenv("GEMINI_API_KEY", "")
				t.Setenv("GOOGLE_API_KEY", "")
			},
		},
		{
			name:          "whitespace_env_vars_treated_as_empty",
			geminiAPIKey:  "   \t\n   ",
			googleAPIKey:  "   \t\n   ",
			expectedError: "API key not found in config or environment variables",
			setupEnv: func(t *testing.T) {
				t.Setenv("GEMINI_API_KEY", "   \t\n   ")
				t.Setenv("GOOGLE_API_KEY", "   \t\n   ")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			if tt.setupEnv != nil {
				tt.setupEnv(t)
			}

			// Create config
			config := &gemini.ClientConfig{
				Model:    "gemini-embedding-001",
				TaskType: "RETRIEVAL_DOCUMENT",
				Timeout:  30 * time.Second,
			}

			if tt.configAPIKey != "" {
				config.APIKey = tt.configAPIKey
			}

			// This test should fail initially because NewClientFromEnv doesn't exist
			client, err := gemini.NewClientFromEnv(config)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expectedError)
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("expected error %q, got %q", tt.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if client == nil {
				t.Error("expected non-nil client")
				return
			}

			// Verify the API key was set correctly
			actualConfig := client.GetConfig()
			if actualConfig.APIKey != tt.expectedAPIKey {
				t.Errorf("expected API key %q, got %q", tt.expectedAPIKey, actualConfig.APIKey)
			}
		})
	}
}

// TestGeminiClient_RequestResponseStructDefinitions tests request/response struct definitions.
func TestGeminiClient_RequestResponseStructDefinitions(t *testing.T) {
	t.Run("embed_content_request_structure", func(t *testing.T) {
		// This test should fail initially because these structs don't exist
		request := &gemini.EmbedContentRequest{
			Model: "models/gemini-embedding-001",
			Content: gemini.Content{
				Parts: []gemini.Part{
					{Text: "Sample code to embed"},
				},
			},
			TaskType:             "RETRIEVAL_DOCUMENT",
			OutputDimensionality: 768,
		}

		expectedFields := map[string]interface{}{
			"Model":                "models/gemini-embedding-001",
			"TaskType":             "RETRIEVAL_DOCUMENT",
			"OutputDimensionality": 768,
		}

		for field, expected := range expectedFields {
			if !verifyRequestField(request, field, expected) {
				t.Errorf("request field %s: expected %v", field, expected)
			}
		}

		// Test Content structure
		if len(request.Content.Parts) != 1 {
			t.Errorf("expected 1 part, got %d", len(request.Content.Parts))
		}

		if request.Content.Parts[0].Text != "Sample code to embed" {
			t.Errorf("expected text %q, got %q", "Sample code to embed", request.Content.Parts[0].Text)
		}
	})

	t.Run("embed_content_response_structure", func(t *testing.T) {
		// This test should fail initially because these structs don't exist
		response := &gemini.EmbedContentResponse{
			Embedding: gemini.ContentEmbedding{
				Values: []float64{0.1, 0.2, 0.3, 0.4, 0.5},
			},
		}

		expectedValues := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
		if len(response.Embedding.Values) != len(expectedValues) {
			t.Errorf("expected %d values, got %d", len(expectedValues), len(response.Embedding.Values))
			return
		}

		for i, expected := range expectedValues {
			if response.Embedding.Values[i] != expected {
				t.Errorf("embedding value[%d]: expected %f, got %f", i, expected, response.Embedding.Values[i])
			}
		}
	})

	t.Run("error_response_structure", func(t *testing.T) {
		// This test should fail initially because these structs don't exist
		errorResponse := &gemini.ErrorResponse{
			Error: gemini.ErrorDetails{
				Code:    400,
				Message: "Invalid request",
				Status:  "INVALID_ARGUMENT",
				Details: []gemini.ErrorDetail{
					{
						Type:        "type.googleapis.com/google.rpc.BadRequest",
						Description: "The request was invalid",
					},
				},
			},
		}

		if errorResponse.Error.Code != 400 {
			t.Errorf("expected error code 400, got %d", errorResponse.Error.Code)
		}

		if errorResponse.Error.Message != "Invalid request" {
			t.Errorf("expected error message %q, got %q", "Invalid request", errorResponse.Error.Message)
		}

		if errorResponse.Error.Status != "INVALID_ARGUMENT" {
			t.Errorf("expected error status %q, got %q", "INVALID_ARGUMENT", errorResponse.Error.Status)
		}

		if len(errorResponse.Error.Details) != 1 {
			t.Errorf("expected 1 error detail, got %d", len(errorResponse.Error.Details))
		}
	})

	t.Run("batch_embed_content_request_structure", func(t *testing.T) {
		// This test should fail initially because these structs don't exist
		batchRequest := &gemini.BatchEmbedContentRequest{
			Requests: []gemini.EmbedContentRequest{
				{
					Model: "models/gemini-embedding-001",
					Content: gemini.Content{
						Parts: []gemini.Part{
							{Text: "First code snippet"},
						},
					},
					TaskType: "RETRIEVAL_DOCUMENT",
				},
				{
					Model: "models/gemini-embedding-001",
					Content: gemini.Content{
						Parts: []gemini.Part{
							{Text: "Second code snippet"},
						},
					},
					TaskType: "RETRIEVAL_DOCUMENT",
				},
			},
		}

		if len(batchRequest.Requests) != 2 {
			t.Errorf("expected 2 requests, got %d", len(batchRequest.Requests))
		}

		expectedTexts := []string{"First code snippet", "Second code snippet"}
		for i, expectedText := range expectedTexts {
			actualText := batchRequest.Requests[i].Content.Parts[0].Text
			if actualText != expectedText {
				t.Errorf("request[%d] text: expected %q, got %q", i, expectedText, actualText)
			}
		}
	})

	t.Run("batch_embed_content_response_structure", func(t *testing.T) {
		// This test should fail initially because these structs don't exist
		batchResponse := &gemini.BatchEmbedContentResponse{
			Embeddings: []gemini.ContentEmbedding{
				{Values: []float64{0.1, 0.2}},
				{Values: []float64{0.3, 0.4}},
			},
		}

		if len(batchResponse.Embeddings) != 2 {
			t.Errorf("expected 2 embeddings, got %d", len(batchResponse.Embeddings))
		}

		expectedValues := [][]float64{{0.1, 0.2}, {0.3, 0.4}}
		for i, expected := range expectedValues {
			actual := batchResponse.Embeddings[i].Values
			if len(actual) != len(expected) {
				t.Errorf("embedding[%d]: expected %d values, got %d", i, len(expected), len(actual))
				continue
			}
			for j, expectedValue := range expected {
				if actual[j] != expectedValue {
					t.Errorf("embedding[%d][%d]: expected %f, got %f", i, j, expectedValue, actual[j])
				}
			}
		}
	})
}

// TestGeminiClient_BasicClientInitializationAndConfiguration tests basic client initialization.
func TestGeminiClient_BasicClientInitializationAndConfiguration(t *testing.T) {
	t.Run("successful_initialization_with_minimal_config", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey: "test-api-key",
		}

		// This test should fail initially because NewClient doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		if client == nil {
			t.Error("expected non-nil client")
			return
		}

		// Verify default values were set
		clientConfig := client.GetConfig()
		expectedDefaults := map[string]interface{}{
			"Model":      "gemini-embedding-001",
			"TaskType":   "RETRIEVAL_DOCUMENT",
			"Timeout":    30 * time.Second,
			"Dimensions": 768,
		}

		for field, expected := range expectedDefaults {
			if !verifyField(clientConfig, field, expected) {
				t.Errorf("default field %s: expected %v", field, expected)
			}
		}
	})

	t.Run("successful_initialization_with_full_config", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey:     "full-config-api-key",
			Model:      "gemini-embedding-001",
			TaskType:   "SEMANTIC_SIMILARITY",
			Timeout:    45 * time.Second,
			Dimensions: 768,
		}

		// This test should fail initially because NewClient doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		if client == nil {
			t.Error("expected non-nil client")
			return
		}

		// Verify all config values were set correctly
		clientConfig := client.GetConfig()
		expectedFields := map[string]interface{}{
			"APIKey":     "full-config-api-key",
			"Model":      "gemini-embedding-001",
			"TaskType":   "SEMANTIC_SIMILARITY",
			"Timeout":    45 * time.Second,
			"Dimensions": 768,
		}

		for field, expected := range expectedFields {
			if !verifyField(clientConfig, field, expected) {
				t.Errorf("config field %s: expected %v", field, expected)
			}
		}
	})

	t.Run("client_implements_embedding_service_interface", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey: "interface-test-api-key",
		}

		// This test should fail initially because NewClient doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		// Test that client implements EmbeddingService interface
		var _ outbound.EmbeddingService = client

		// Test that all interface methods are available
		ctx := context.Background()

		// Test GenerateEmbedding - should work but may fail with auth errors (expected behavior)
		_, err = client.GenerateEmbedding(ctx, "test text", outbound.EmbeddingOptions{})
		// Note: This may fail due to invalid API key, but that's expected auth behavior
		// The method is implemented and should handle the request properly
		if err != nil {
			checkEmbeddingError(t, err, "GenerateEmbedding")
		}

		// Test GenerateBatchEmbeddings - should work but may fail with auth errors
		_, err = client.GenerateBatchEmbeddings(ctx, []string{"text1", "text2"}, outbound.EmbeddingOptions{})
		if err != nil {
			checkEmbeddingError(t, err, "GenerateBatchEmbeddings")
		}

		// Test ValidateApiKey - should work (basic validation always succeeds)
		err = client.ValidateApiKey(ctx)
		if err != nil {
			checkEmbeddingError(t, err, "ValidateApiKey")
		}

		// Test GetModelInfo - should always succeed (returns hardcoded values)
		modelInfo, err := client.GetModelInfo(ctx)
		switch {
		case err != nil:
			t.Errorf("GetModelInfo should succeed but got error: %v", err)
		case modelInfo == nil:
			t.Error("GetModelInfo should return non-nil ModelInfo")
		default:
			verifyModelInfo(t, modelInfo)
		}

		// Test GetSupportedModels - should always succeed (returns hardcoded list)
		supportedModels, err := client.GetSupportedModels(ctx)
		switch {
		case err != nil:
			t.Errorf("GetSupportedModels should succeed but got error: %v", err)
		case len(supportedModels) == 0:
			t.Error("GetSupportedModels should return non-empty list")
		default:
			verifySupportedModels(t, supportedModels)
		}

		// Test EstimateTokenCount - should always succeed (pure calculation)
		tokenCount, err := client.EstimateTokenCount(ctx, "test text")
		switch {
		case err != nil:
			t.Errorf("EstimateTokenCount should succeed but got error: %v", err)
		case tokenCount <= 0:
			t.Errorf("EstimateTokenCount should return positive count, got %d", tokenCount)
		case tokenCount < 1 || tokenCount > 10:
			t.Errorf("EstimateTokenCount returned unreasonable count %d for 'test text'", tokenCount)
		}
	})

	t.Run("client_has_proper_http_client_configuration", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey:  "http-config-test-api-key",
			Timeout: 60 * time.Second,
		}

		// This test should fail initially because NewClient doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		// Verify client configuration (HTTP client details no longer relevant with SDK)
		clientConfig := client.GetConfig()
		if clientConfig.Timeout != config.Timeout {
			t.Errorf("expected timeout %v, got %v", config.Timeout, clientConfig.Timeout)
		}
	})

	t.Run("client_initialization_with_structured_logging", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey: "logging-test-api-key",
		}

		ctx := context.Background()

		// This test should fail initially because NewClient doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		// Verify that client initialization was logged
		// (This is a behavioral test - we can't easily verify logs in unit tests,
		// but we can verify the client was configured with logging)
		if client == nil {
			t.Error("expected non-nil client for logging test")
			return
		}

		// Test that client logs operations with structured logging
		slogger.Info(ctx, "Testing Gemini client initialization", slogger.Fields{
			"api_key_length": len(config.APIKey),
			"model":          config.Model,
			"task_type":      config.TaskType,
		})

		// The actual logging verification would happen in integration tests
		// Here we just verify the client can be created and configured
	})
}

// Helper functions for testing (these will also fail initially)

func verifyField(obj interface{}, fieldName string, expectedValue interface{}) bool {
	if obj == nil {
		return false
	}

	objValue := reflect.ValueOf(obj)
	if objValue.Kind() == reflect.Ptr {
		objValue = objValue.Elem()
	}

	if objValue.Kind() != reflect.Struct {
		return false
	}

	field := objValue.FieldByName(fieldName)
	if !field.IsValid() {
		return false
	}

	actualValue := field.Interface()
	return reflect.DeepEqual(actualValue, expectedValue)
}

func verifyRequestField(request interface{}, fieldName string, expectedValue interface{}) bool {
	// Simple reflection-based field verification for GREEN phase
	if request == nil {
		return false
	}

	requestValue := reflect.ValueOf(request)
	if requestValue.Kind() == reflect.Ptr {
		requestValue = requestValue.Elem()
	}

	if requestValue.Kind() != reflect.Struct {
		return false
	}

	field := requestValue.FieldByName(fieldName)
	if !field.IsValid() {
		return false
	}

	actualValue := field.Interface()
	return reflect.DeepEqual(actualValue, expectedValue)
}

// checkEmbeddingError checks if an error is an expected embedding error.
func checkEmbeddingError(t *testing.T, err error, methodName string) {
	// Check if it's an expected auth/API error, not a "not implemented" error
	embErr := &outbound.EmbeddingError{}
	if errors.As(err, &embErr) {
		if embErr.Type == "auth" || embErr.Code == "client_creation_failed" {
			// This is expected behavior with test API key
			t.Logf("%s auth error (expected): %v", methodName, err)
		} else {
			t.Errorf("%s unexpected error type: %v", methodName, err)
		}
	} else {
		t.Logf("%s error (may be expected): %v", methodName, err)
	}
}

// verifyModelInfo verifies the model info structure.
func verifyModelInfo(t *testing.T, modelInfo *outbound.ModelInfo) {
	// Verify expected model info structure
	if modelInfo.Name != "gemini-embedding-001" {
		t.Errorf("expected model name 'gemini-embedding-001', got %q", modelInfo.Name)
	}
	if modelInfo.Dimensions != 768 {
		t.Errorf("expected dimensions 768, got %d", modelInfo.Dimensions)
	}
	if len(modelInfo.SupportedTaskTypes) == 0 {
		t.Error("expected non-empty SupportedTaskTypes")
	}
}

// verifySupportedModels verifies the supported models list.
func verifySupportedModels(t *testing.T, supportedModels []string) {
	// Verify expected models are included
	found := false
	for _, model := range supportedModels {
		if model == "gemini-embedding-001" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'gemini-embedding-001' to be in supported models list")
	}
}
