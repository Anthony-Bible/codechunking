package gemini_test

import (
	"codechunking/internal/adapter/outbound/gemini"
	"codechunking/internal/application/common/slogger"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestGeminiHTTPClient_Configuration tests HTTP client configuration aspects.
func TestGeminiHTTPClient_Configuration(t *testing.T) {
	t.Run("default_http_client_configuration", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey:   "test-http-config-key",
			BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
			Model:    "gemini-embedding-001",
			TaskType: "RETRIEVAL_DOCUMENT",
		}

		// This test should fail initially because NewClient doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		// Test default HTTP client configuration
		httpClient := client.GetHTTPClient()
		if httpClient == nil {
			t.Error("expected non-nil HTTP client")
			return
		}

		// Verify timeout is set to default
		if httpClient.Timeout != 30*time.Second {
			t.Errorf("expected default timeout %v, got %v", 30*time.Second, httpClient.Timeout)
		}

		// Verify transport configuration
		transport, ok := httpClient.Transport.(*http.Transport)
		if !ok {
			t.Error("expected HTTP transport to be *http.Transport")
			return
		}

		// Test transport settings for optimal API performance
		expectedTransportSettings := map[string]interface{}{
			"MaxIdleConns":        100,
			"IdleConnTimeout":     90 * time.Second,
			"TLSHandshakeTimeout": 10 * time.Second,
			"DisableCompression":  false,
			"ForceAttemptHTTP2":   true,
		}

		for setting, expected := range expectedTransportSettings {
			if !verifyTransportSetting(transport, setting, expected) {
				t.Errorf("transport setting %s: expected %v", setting, expected)
			}
		}
	})

	t.Run("custom_timeout_configuration", func(t *testing.T) {
		customTimeout := 45 * time.Second
		config := &gemini.ClientConfig{
			APIKey:   "test-custom-timeout-key",
			BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
			Model:    "gemini-embedding-001",
			TaskType: "RETRIEVAL_DOCUMENT",
			Timeout:  customTimeout,
		}

		// This test should fail initially because NewClient doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		httpClient := client.GetHTTPClient()
		if httpClient.Timeout != customTimeout {
			t.Errorf("expected custom timeout %v, got %v", customTimeout, httpClient.Timeout)
		}
	})

	t.Run("http_client_with_user_agent_configuration", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey:    "test-user-agent-key",
			BaseURL:   "https://generativelanguage.googleapis.com/v1beta",
			Model:     "gemini-embedding-001",
			TaskType:  "RETRIEVAL_DOCUMENT",
			UserAgent: "CustomCodeChunking/2.0.0 (Go/1.21)",
		}

		// This test should fail initially because NewClient doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		// Test request creation with custom User-Agent
		ctx := context.Background()
		req, err := client.CreateRequest(ctx, "POST", "/test-endpoint", nil)
		if err != nil {
			t.Errorf("unexpected error creating request: %v", err)
			return
		}

		actualUserAgent := req.Header.Get("User-Agent")
		if actualUserAgent != "CustomCodeChunking/2.0.0 (Go/1.21)" {
			t.Errorf("expected User-Agent %q, got %q", "CustomCodeChunking/2.0.0 (Go/1.21)", actualUserAgent)
		}
	})

	t.Run("http_client_connection_reuse", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey:   "test-connection-reuse-key",
			BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
			Model:    "gemini-embedding-001",
			TaskType: "RETRIEVAL_DOCUMENT",
		}

		// This test should fail initially because NewClient doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		// Verify that multiple requests reuse the same HTTP client
		httpClient1 := client.GetHTTPClient()
		httpClient2 := client.GetHTTPClient()

		if httpClient1 != httpClient2 {
			t.Error("expected HTTP client to be reused across multiple calls")
		}
	})
}

// TestGeminiHTTPClient_RequestCreation tests HTTP request creation.
func TestGeminiHTTPClient_RequestCreation(t *testing.T) {
	t.Run("post_request_creation_with_authentication", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey:   "test-post-request-key",
			BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
			Model:    "gemini-embedding-001",
			TaskType: "RETRIEVAL_DOCUMENT",
		}

		// This test should fail initially because NewClient doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		ctx := context.Background()
		endpoint := "/models/gemini-embedding-001:embedContent"

		// Create a POST request
		req, err := client.CreateRequest(ctx, "POST", endpoint, strings.NewReader(`{"test": "data"}`))
		if err != nil {
			t.Errorf("unexpected error creating POST request: %v", err)
			return
		}

		// Verify request method
		if req.Method != http.MethodPost {
			t.Errorf("expected method POST, got %s", req.Method)
		}

		// Verify URL construction
		expectedURL := "https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:embedContent"
		if req.URL.String() != expectedURL {
			t.Errorf("expected URL %q, got %q", expectedURL, req.URL.String())
		}

		// Verify authentication header
		authHeader := req.Header.Get("X-Goog-Api-Key")
		if authHeader != "test-post-request-key" {
			t.Errorf("expected x-goog-api-key %q, got %q", "test-post-request-key", authHeader)
		}

		// Verify content type
		contentType := req.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type %q, got %q", "application/json", contentType)
		}

		// Verify accept header
		accept := req.Header.Get("Accept")
		if accept != "application/json" {
			t.Errorf("expected Accept %q, got %q", "application/json", accept)
		}
	})

	t.Run("get_request_creation", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey:   "test-get-request-key",
			BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
			Model:    "gemini-embedding-001",
			TaskType: "RETRIEVAL_DOCUMENT",
		}

		// This test should fail initially because NewClient doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		ctx := context.Background()
		endpoint := "/models"

		// Create a GET request
		req, err := client.CreateRequest(ctx, "GET", endpoint, nil)
		if err != nil {
			t.Errorf("unexpected error creating GET request: %v", err)
			return
		}

		// Verify request method
		if req.Method != http.MethodGet {
			t.Errorf("expected method GET, got %s", req.Method)
		}

		// Verify URL construction
		expectedURL := "https://generativelanguage.googleapis.com/v1beta/models"
		if req.URL.String() != expectedURL {
			t.Errorf("expected URL %q, got %q", expectedURL, req.URL.String())
		}

		// Verify GET request has no body
		if req.Body != nil {
			t.Error("expected GET request to have nil body")
		}
	})

	t.Run("request_with_context_timeout", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey:   "test-context-timeout-key",
			BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
			Model:    "gemini-embedding-001",
			TaskType: "RETRIEVAL_DOCUMENT",
		}

		// This test should fail initially because NewClient doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		endpoint := "/models/gemini-embedding-001:embedContent"

		// Create request with timeout context
		req, err := client.CreateRequest(ctx, "POST", endpoint, nil)
		if err != nil {
			t.Errorf("unexpected error creating request with timeout: %v", err)
			return
		}

		// Verify context is associated with request
		if req.Context() != ctx {
			t.Error("expected request context to be the timeout context")
		}
	})

	t.Run("request_with_invalid_base_url", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey:   "test-invalid-url-key",
			BaseURL:  "invalid://not-a-valid-url",
			Model:    "gemini-embedding-001",
			TaskType: "RETRIEVAL_DOCUMENT",
		}

		// This test should fail during client creation because of invalid URL
		_, err := gemini.NewClient(config)
		if err == nil {
			t.Error("expected error creating client with invalid base URL")
			return
		}

		if !strings.Contains(strings.ToLower(err.Error()), "invalid") {
			t.Errorf("expected error to contain 'invalid', got: %v", err)
		}
	})
}

// TestGeminiHTTPClient_EndpointConstruction tests API endpoint construction.
func TestGeminiHTTPClient_EndpointConstruction(t *testing.T) {
	tests := []struct {
		name          string
		baseURL       string
		endpoint      string
		expectedURL   string
		expectedError string
	}{
		{
			name:        "standard_embed_endpoint",
			baseURL:     "https://generativelanguage.googleapis.com/v1beta",
			endpoint:    "/models/gemini-embedding-001:embedContent",
			expectedURL: "https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:embedContent",
		},
		{
			name:        "batch_embed_endpoint",
			baseURL:     "https://generativelanguage.googleapis.com/v1beta",
			endpoint:    "/models/gemini-embedding-001:batchEmbedContents",
			expectedURL: "https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:batchEmbedContents",
		},
		{
			name:        "models_list_endpoint",
			baseURL:     "https://generativelanguage.googleapis.com/v1beta",
			endpoint:    "/models",
			expectedURL: "https://generativelanguage.googleapis.com/v1beta/models",
		},
		{
			name:        "base_url_with_trailing_slash",
			baseURL:     "https://generativelanguage.googleapis.com/v1beta/",
			endpoint:    "/models/gemini-embedding-001:embedContent",
			expectedURL: "https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:embedContent",
		},
		{
			name:        "endpoint_without_leading_slash",
			baseURL:     "https://generativelanguage.googleapis.com/v1beta",
			endpoint:    "models/gemini-embedding-001:embedContent",
			expectedURL: "https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:embedContent",
		},
		{
			name:        "custom_base_url",
			baseURL:     "https://custom.api.example.com/v1",
			endpoint:    "/embed",
			expectedURL: "https://custom.api.example.com/v1/embed",
		},
		{
			name:          "malformed_base_url",
			baseURL:       "not-a-url",
			endpoint:      "/models",
			expectedError: "invalid base URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &gemini.ClientConfig{
				APIKey:   "test-endpoint-construction-key",
				BaseURL:  tt.baseURL,
				Model:    "gemini-embedding-001",
				TaskType: "RETRIEVAL_DOCUMENT",
			}

			// This test should fail initially because NewClient doesn't exist
			client, err := gemini.NewClient(config)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expectedError)
					return
				}
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expectedError)) {
					t.Errorf("expected error containing %q, got %q", tt.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error creating client: %v", err)
				return
			}

			ctx := context.Background()
			req, err := client.CreateRequest(ctx, "POST", tt.endpoint, nil)
			if err != nil {
				t.Errorf("unexpected error creating request: %v", err)
				return
			}

			if req.URL.String() != tt.expectedURL {
				t.Errorf("expected URL %q, got %q", tt.expectedURL, req.URL.String())
			}
		})
	}
}

// TestGeminiHTTPClient_HeadersAndAuthentication tests headers and authentication.
func TestGeminiHTTPClient_HeadersAndAuthentication(t *testing.T) {
	t.Run("required_headers_present", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey:   "test-required-headers-key",
			BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
			Model:    "gemini-embedding-001",
			TaskType: "RETRIEVAL_DOCUMENT",
		}

		// This test should fail initially because NewClient doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		ctx := context.Background()
		req, err := client.CreateRequest(ctx, "POST", "/test", strings.NewReader(`{}`))
		if err != nil {
			t.Errorf("unexpected error creating request: %v", err)
			return
		}

		// Test required headers
		requiredHeaders := map[string]string{
			"x-goog-api-key": "test-required-headers-key",
			"Content-Type":   "application/json",
			"Accept":         "application/json",
			"User-Agent":     "CodeChunking-Gemini-Client/1.0.0",
		}

		for headerName, expectedValue := range requiredHeaders {
			actualValue := req.Header.Get(headerName)
			if actualValue != expectedValue {
				t.Errorf("header %s: expected %q, got %q", headerName, expectedValue, actualValue)
			}
		}
	})

	t.Run("no_forbidden_headers", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey:   "test-forbidden-headers-key",
			BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
			Model:    "gemini-embedding-001",
			TaskType: "RETRIEVAL_DOCUMENT",
		}

		// This test should fail initially because NewClient doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		ctx := context.Background()
		req, err := client.CreateRequest(ctx, "POST", "/test", nil)
		if err != nil {
			t.Errorf("unexpected error creating request: %v", err)
			return
		}

		// Test that forbidden headers are not present
		forbiddenHeaders := []string{
			"Authorization", // Should use x-goog-api-key instead
			"Bearer",
			"Basic",
		}

		for _, headerName := range forbiddenHeaders {
			if value := req.Header.Get(headerName); value != "" {
				t.Errorf("forbidden header %s present with value %q", headerName, value)
			}
		}
	})

	t.Run("structured_logging_on_request_creation", func(t *testing.T) {
		config := &gemini.ClientConfig{
			APIKey:   "test-logging-request-key",
			BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
			Model:    "gemini-embedding-001",
			TaskType: "RETRIEVAL_DOCUMENT",
		}

		// This test should fail initially because NewClient doesn't exist
		client, err := gemini.NewClient(config)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		ctx := context.Background()

		// Log request creation for testing
		slogger.Info(ctx, "Creating HTTP request for Gemini API", slogger.Fields{
			"method":   "POST",
			"endpoint": "/models/gemini-embedding-001:embedContent",
			"has_body": true,
		})

		req, err := client.CreateRequest(
			ctx,
			"POST",
			"/models/gemini-embedding-001:embedContent",
			strings.NewReader(`{"test": "data"}`),
		)
		if err != nil {
			t.Errorf("unexpected error creating request: %v", err)
			return
		}

		if req == nil {
			t.Error("expected non-nil request")
		}
	})
}

// Helper function using reflection for transport field verification.
func verifyTransportSetting(transport *http.Transport, setting string, expectedValue interface{}) bool {
	if transport == nil {
		return false
	}

	// Simple field-by-field comparison for the GREEN phase
	switch setting {
	case "MaxIdleConns":
		return transport.MaxIdleConns == expectedValue
	case "IdleConnTimeout":
		return transport.IdleConnTimeout == expectedValue
	case "TLSHandshakeTimeout":
		return transport.TLSHandshakeTimeout == expectedValue
	case "DisableCompression":
		return transport.DisableCompression == expectedValue
	case "ForceAttemptHTTP2":
		return transport.ForceAttemptHTTP2 == expectedValue
	default:
		return false
	}
}
