package gemini_test

import (
	"codechunking/internal/adapter/outbound/gemini"
	"codechunking/internal/application/common/slogger"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestGeminiClient_AuthenticationHeaders tests various authentication header scenarios.
func TestGeminiClient_AuthenticationHeaders(t *testing.T) {
	tests := []struct {
		name              string
		apiKey            string
		expectedAuthError bool
		expectedHeaders   map[string]string
		description       string
	}{
		{
			name:   "valid_standard_api_key",
			apiKey: "AIzaSyDaGmWKa4JsXZ5HjGAdPQ98nQqcFMnGbE8",
			expectedHeaders: map[string]string{
				"x-goog-api-key": "AIzaSyDaGmWKa4JsXZ5HjGAdPQ98nQqcFMnGbE8",
				"Content-Type":   "application/json",
				"Accept":         "application/json",
				"User-Agent":     "CodeChunking-Gemini-Client/1.0.0",
			},
			description: "Standard Gemini API key should set proper authentication headers",
		},
		{
			name:   "api_key_with_underscores_and_hyphens",
			apiKey: "AIzaSy_Test-Key_123-ABC_def",
			expectedHeaders: map[string]string{
				"x-goog-api-key": "AIzaSy_Test-Key_123-ABC_def",
			},
			description: "API keys with special characters should be handled correctly",
		},
		{
			name:              "empty_api_key",
			apiKey:            "",
			expectedAuthError: true,
			description:       "Empty API key should cause authentication error",
		},
		{
			name:              "api_key_only_whitespace",
			apiKey:            "   \t\n   ",
			expectedAuthError: true,
			description:       "Whitespace-only API key should be treated as empty",
		},
		{
			name:   "api_key_with_leading_trailing_whitespace",
			apiKey: "  AIzaSyDaGmWKa4JsXZ5HjGAdPQ98nQqcFMnGbE8  ",
			expectedHeaders: map[string]string{
				"x-goog-api-key": "AIzaSyDaGmWKa4JsXZ5HjGAdPQ98nQqcFMnGbE8",
			},
			description: "Leading/trailing whitespace should be trimmed from API key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &gemini.ClientConfig{
				APIKey:   tt.apiKey,
				BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
				Model:    "gemini-embedding-001",
				TaskType: "RETRIEVAL_DOCUMENT",
				Timeout:  30 * time.Second,
			}

			// This test should fail because the Client struct doesn't exist
			client, err := gemini.NewClient(config)

			if tt.expectedAuthError {
				if err == nil {
					t.Error("expected authentication error, got nil")
					return
				}
				// Verify the error is authentication-related
				if !strings.Contains(strings.ToLower(err.Error()), "api key") {
					t.Errorf("expected API key related error, got: %v", err)
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

			// Test request creation with authentication headers
			ctx := context.Background()
			req, err := client.CreateRequest(ctx, "POST", "/models/gemini-embedding-001:embedContent", nil)
			if err != nil {
				t.Errorf("unexpected error creating request: %v", err)
				return
			}

			// Verify expected headers are set
			for headerName, expectedValue := range tt.expectedHeaders {
				actualValue := req.Header.Get(headerName)
				if actualValue != expectedValue {
					t.Errorf("header %s: expected %q, got %q", headerName, expectedValue, actualValue)
				}
			}

			// Verify no credentials leak into other headers
			authHeader := req.Header.Get("Authorization")
			if authHeader != "" {
				t.Errorf("unexpected Authorization header: %q (should use x-goog-api-key)", authHeader)
			}
		})
	}
}

// TestGeminiClient_EnvironmentVariableResolution tests detailed environment variable resolution.
func TestGeminiClient_EnvironmentVariableResolution(t *testing.T) {
	// Save and restore environment
	originalEnv := map[string]string{
		"GEMINI_API_KEY": os.Getenv("GEMINI_API_KEY"),
		"GOOGLE_API_KEY": os.Getenv("GOOGLE_API_KEY"),
	}

	defer func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	tests := []struct {
		name               string
		setupEnv           func()
		configAPIKey       string
		expectedResolution string
		expectedError      string
		description        string
	}{
		{
			name: "gemini_api_key_env_var_priority",
			setupEnv: func() {
				os.Setenv("GEMINI_API_KEY", "from-gemini-env")
				os.Setenv("GOOGLE_API_KEY", "from-google-env")
			},
			expectedResolution: "from-gemini-env",
			description:        "GEMINI_API_KEY should take priority over GOOGLE_API_KEY",
		},
		{
			name: "google_api_key_fallback",
			setupEnv: func() {
				os.Unsetenv("GEMINI_API_KEY")
				os.Setenv("GOOGLE_API_KEY", "from-google-fallback")
			},
			expectedResolution: "from-google-fallback",
			description:        "GOOGLE_API_KEY should be used when GEMINI_API_KEY is not set",
		},
		{
			name: "config_overrides_environment",
			setupEnv: func() {
				os.Setenv("GEMINI_API_KEY", "from-env-override")
				os.Setenv("GOOGLE_API_KEY", "from-google-override")
			},
			configAPIKey:       "from-config-explicit",
			expectedResolution: "from-config-explicit",
			description:        "Explicit config API key should override environment variables",
		},
		{
			name: "empty_env_vars_ignored",
			setupEnv: func() {
				os.Setenv("GEMINI_API_KEY", "")
				os.Setenv("GOOGLE_API_KEY", "")
			},
			configAPIKey:       "from-config-fallback",
			expectedResolution: "from-config-fallback",
			description:        "Empty environment variables should be ignored",
		},
		{
			name: "whitespace_env_vars_ignored",
			setupEnv: func() {
				os.Setenv("GEMINI_API_KEY", "   ")
				os.Setenv("GOOGLE_API_KEY", "\t\n  ")
			},
			configAPIKey:       "from-config-whitespace",
			expectedResolution: "from-config-whitespace",
			description:        "Whitespace-only environment variables should be ignored",
		},
		{
			name: "no_api_key_anywhere",
			setupEnv: func() {
				os.Unsetenv("GEMINI_API_KEY")
				os.Unsetenv("GOOGLE_API_KEY")
			},
			expectedError: "API key not found in config or environment variables",
			description:   "Should error when no API key is provided anywhere",
		},
		{
			name: "env_var_with_special_characters",
			setupEnv: func() {
				os.Setenv("GEMINI_API_KEY", "AIzaSy_Special-123.456")
				os.Unsetenv("GOOGLE_API_KEY")
			},
			expectedResolution: "AIzaSy_Special-123.456",
			description:        "Environment variables with special characters should be preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			tt.setupEnv()

			config := &gemini.ClientConfig{
				BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
				Model:    "gemini-embedding-001",
				TaskType: "RETRIEVAL_DOCUMENT",
			}

			if tt.configAPIKey != "" {
				config.APIKey = tt.configAPIKey
			}

			ctx := context.Background()
			slogger.Info(ctx, "Testing environment variable resolution", slogger.Fields{
				"test_name":      tt.name,
				"config_api_key": tt.configAPIKey != "",
				"description":    tt.description,
			})

			// This test should fail because NewClientFromEnv doesn't exist
			client, err := gemini.NewClientFromEnv(config)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("expected error: %q, got nil", tt.expectedError)
					return
				}
				if err.Error() != tt.expectedError {
					t.Errorf("expected error: %q, got: %q", tt.expectedError, err.Error())
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

			// Verify the resolved API key
			clientConfig := client.GetConfig()
			if clientConfig.APIKey != tt.expectedResolution {
				t.Errorf("expected resolved API key %q, got %q", tt.expectedResolution, clientConfig.APIKey)
			}

			// Verify the API key is used in requests
			req, err := client.CreateRequest(ctx, "POST", "/test", nil)
			if err != nil {
				t.Errorf("unexpected error creating request: %v", err)
				return
			}

			actualHeader := req.Header.Get("X-Goog-Api-Key")
			if actualHeader != tt.expectedResolution {
				t.Errorf("expected request header %q, got %q", tt.expectedResolution, actualHeader)
			}
		})
	}
}

// TestGeminiClient_APIKeyValidation tests API key validation methods.
func TestGeminiClient_APIKeyValidation(t *testing.T) {
	tests := []struct {
		name          string
		apiKey        string
		expectedValid bool
		expectedError string
		description   string
	}{
		{
			name:          "valid_api_key_format",
			apiKey:        "AIzaSyDaGmWKa4JsXZ5HjGAdPQ98nQqcFMnGbE8",
			expectedValid: true,
			description:   "Standard Google API key format should be valid",
		},
		{
			name:          "valid_api_key_with_special_chars",
			apiKey:        "AIzaSy_Test-Key_123-ABC_def",
			expectedValid: true,
			description:   "API keys with underscores and hyphens should be valid",
		},
		{
			name:          "empty_api_key",
			apiKey:        "",
			expectedValid: false,
			expectedError: "API key cannot be empty",
			description:   "Empty API key should be invalid",
		},
		{
			name:          "whitespace_api_key",
			apiKey:        "   \t\n   ",
			expectedValid: false,
			expectedError: "API key cannot be empty or whitespace",
			description:   "Whitespace-only API key should be invalid",
		},
		{
			name:          "too_short_api_key",
			apiKey:        "short",
			expectedValid: false,
			expectedError: "API key is too short",
			description:   "API keys that are too short should be invalid",
		},
		{
			name:          "api_key_with_invalid_characters",
			apiKey:        "AIzaSy@Invalid#Characters$",
			expectedValid: false,
			expectedError: "API key contains invalid characters",
			description:   "API keys with invalid characters should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test static validation method
			isValid, err := gemini.ValidateAPIKeyFormat(tt.apiKey)

			if tt.expectedValid != isValid {
				t.Errorf("ValidateAPIKeyFormat: expected valid=%t, got valid=%t", tt.expectedValid, isValid)
			}

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("ValidateAPIKeyFormat: expected error %q, got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError {
					t.Errorf("ValidateAPIKeyFormat: expected error %q, got %q", tt.expectedError, err.Error())
				}
			} else if err != nil {
				t.Errorf("ValidateAPIKeyFormat: unexpected error: %v", err)
			}

			// Test client validation (if API key format is valid)
			if tt.expectedValid {
				config := &gemini.ClientConfig{
					APIKey:   tt.apiKey,
					BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
					Model:    "gemini-embedding-001",
					TaskType: "RETRIEVAL_DOCUMENT",
				}

				// This test should fail because NewClient doesn't exist
				client, err := gemini.NewClient(config)
				if err != nil {
					t.Errorf("unexpected error creating client: %v", err)
					return
				}

				ctx := context.Background()

				// Test API key validation method on client
				err = client.ValidateApiKey(ctx)
				// Note: This will make a real API call in integration tests
				// In unit tests, this should be mocked or return a specific error
				if err != nil && !strings.Contains(err.Error(), "not implemented") {
					t.Errorf("unexpected error from ValidateApiKey: %v", err)
				}
			}
		})
	}
}

// TestGeminiClient_AuthenticationIntegration tests authentication integration scenarios.
func TestGeminiClient_AuthenticationIntegration(t *testing.T) {
	t.Run("authentication_with_retry_mechanism", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey:     "test-retry-key",
			BaseURL:    "https://generativelanguage.googleapis.com/v1beta",
			Model:      "gemini-embedding-001",
			TaskType:   "RETRIEVAL_DOCUMENT",
			MaxRetries: 3,
			Timeout:    10 * time.Second,
		}

		// This test should fail because the Client doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		// Test that authentication failures are retried appropriately
		ctx := context.Background()

		// This should demonstrate retry logic for auth failures
		// (In actual implementation, this would be tested with mock HTTP responses)
		err = client.ValidateApiKey(ctx)
		if err != nil && !strings.Contains(err.Error(), "not implemented") {
			t.Errorf("unexpected error from retry validation: %v", err)
		}
	})

	t.Run("authentication_timeout_handling", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey:   "test-timeout-key",
			BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
			Model:    "gemini-embedding-001",
			TaskType: "RETRIEVAL_DOCUMENT",
			Timeout:  1 * time.Millisecond, // Very short timeout
		}

		// This test should fail because the Client doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		ctx := context.Background()

		// Test timeout handling in authentication
		err = client.ValidateApiKey(ctx)
		// Should handle timeout gracefully
		if err != nil && !strings.Contains(err.Error(), "timeout") &&
			!strings.Contains(err.Error(), "not implemented") {
			t.Errorf("expected timeout error, got: %v", err)
		}
	})

	t.Run("authentication_with_context_cancellation", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey:   "test-cancel-key",
			BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
			Model:    "gemini-embedding-001",
			TaskType: "RETRIEVAL_DOCUMENT",
		}

		// This test should fail because the Client doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		// Test context cancellation during authentication
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err = client.ValidateApiKey(ctx)
		// Should handle cancellation gracefully
		if err != nil && !strings.Contains(err.Error(), "canceled") &&
			!strings.Contains(err.Error(), "not implemented") {
			t.Errorf("expected cancellation error, got: %v", err)
		}
	})
}
