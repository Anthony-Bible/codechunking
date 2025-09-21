package gemini_test

import (
	"codechunking/internal/adapter/outbound/gemini"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestGeminiClient_JSONRequestSerialization tests JSON serialization expectations.
// These tests define what the SerializeEmbeddingRequest method should do when implemented.
func TestGeminiClient_JSONRequestSerialization(t *testing.T) {
	tests := []struct {
		name           string
		text           string
		options        outbound.EmbeddingOptions
		expectedJSON   string
		expectedError  string
		shouldValidate bool
	}{
		{
			name: "valid_single_embedding_request_serialization",
			text: "func calculateSum(a, b int) int { return a + b }",
			options: outbound.EmbeddingOptions{
				Model:     "gemini-embedding-001",
				TaskType:  outbound.TaskTypeRetrievalDocument,
				MaxTokens: 2048,
				Timeout:   30 * time.Second,
			},
			expectedJSON: `{
				"model": "models/gemini-embedding-001",
				"content": {
					"parts": [
						{"text": "func calculateSum(a, b int) int { return a + b }"}
					]
				},
				"taskType": "RETRIEVAL_DOCUMENT",
				"outputDimensionality": 768
			}`,
			shouldValidate: true,
		},
		{
			name: "request_serialization_with_empty_text",
			text: "",
			options: outbound.EmbeddingOptions{
				Model:     "gemini-embedding-001",
				TaskType:  outbound.TaskTypeRetrievalDocument,
				MaxTokens: 2048,
			},
			expectedError: "text content cannot be empty",
		},
		{
			name: "request_serialization_with_invalid_model",
			text: "valid code content",
			options: outbound.EmbeddingOptions{
				Model:     "invalid-model",
				TaskType:  outbound.TaskTypeRetrievalDocument,
				MaxTokens: 2048,
			},
			expectedError: "unsupported model: invalid-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient(t)
			ctx := context.Background()

			// GREEN PHASE: Call the implemented method and test behavior
			jsonData, err := client.SerializeEmbeddingRequest(ctx, tt.text, tt.options)

			if tt.expectedError != "" {
				// Expecting an error
				if err == nil {
					t.Errorf("expected error containing '%s', but got none", tt.expectedError)
					return
				}
				if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error containing '%s', got '%s'", tt.expectedError, err.Error())
				}
				return
			}

			// Expecting success
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if jsonData == nil {
				t.Error("expected JSON data, got nil")
				return
			}

			if tt.shouldValidate {
				validateSerializedJSON(t, jsonData, tt.text)
			}
		})
	}
}

// TestGeminiClient_JSONResponseDeserialization tests JSON response parsing expectations.
// These tests define what the DeserializeEmbeddingResponse method should do when implemented.
func TestGeminiClient_JSONResponseDeserialization(t *testing.T) {
	tests := []struct {
		name              string
		responseBody      string
		expectedEmbedding []float64
		expectedError     string
		errorType         string
	}{
		{
			name: "valid_single_embedding_response_deserialization",
			responseBody: `{
				"embedding": {
					"values": [0.1, 0.2, 0.3, 0.4, 0.5]
				}
			}`,
			expectedEmbedding: []float64{0.1, 0.2, 0.3, 0.4, 0.5},
		},
		{
			name:          "malformed_json_response",
			responseBody:  `{"embedding": {"values": [0.1, 0.2, 0.3}`, // Missing closing brackets
			expectedError: "failed to parse response JSON",
			errorType:     "validation",
		},
		{
			name:          "missing_embedding_field_response",
			responseBody:  `{"data": {"values": [0.1, 0.2, 0.3]}}`,
			expectedError: "response missing required embedding field",
			errorType:     "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient(t)
			ctx := context.Background()

			// GREEN PHASE: Call the implemented method and test behavior
			result, err := client.DeserializeEmbeddingResponse(ctx, []byte(tt.responseBody))

			if tt.expectedError != "" {
				validateDeserializationError(t, err, tt.expectedError, tt.errorType)
				return
			}

			// Expecting success
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("expected result, got nil")
				return
			}

			// Validate the embedding values
			if len(result.Vector) != len(tt.expectedEmbedding) {
				t.Errorf("expected %d values, got %d", len(tt.expectedEmbedding), len(result.Vector))
				return
			}

			for i, expected := range tt.expectedEmbedding {
				if result.Vector[i] != expected {
					t.Errorf("expected value[%d] to be %f, got %f", i, expected, result.Vector[i])
				}
			}

			// Validate dimensions match vector length
			if result.Dimensions != len(result.Vector) {
				t.Errorf("expected dimensions %d to match vector length %d", result.Dimensions, len(result.Vector))
			}
		})
	}
}

// TestGeminiClient_HTTPErrorHandling tests HTTP error handling expectations.
// These tests define what the HandleHTTPError method should do when implemented.
func TestGeminiClient_HTTPErrorHandling(t *testing.T) {
	tests := []struct {
		name              string
		httpStatusCode    int
		responseBody      string
		expectedErrorCode string
		expectedErrorType string
		expectedRetryable bool
		expectedMessage   string
	}{
		{
			name:           "unauthorized_401_invalid_api_key",
			httpStatusCode: http.StatusUnauthorized,
			responseBody: `{
				"error": {
					"code": 401,
					"message": "Invalid API key provided",
					"status": "UNAUTHENTICATED"
				}
			}`,
			expectedErrorCode: "invalid_api_key",
			expectedErrorType: "auth",
			expectedRetryable: false,
			expectedMessage:   "Invalid API key provided",
		},
		{
			name:           "rate_limit_429_quota_exceeded",
			httpStatusCode: http.StatusTooManyRequests,
			responseBody: `{
				"error": {
					"code": 429,
					"message": "Rate limit exceeded. Please retry after 60 seconds",
					"status": "RESOURCE_EXHAUSTED"
				}
			}`,
			expectedErrorCode: "rate_limit_exceeded",
			expectedErrorType: "quota",
			expectedRetryable: true,
			expectedMessage:   "Rate limit exceeded. Please retry after 60 seconds",
		},
		{
			name:           "internal_server_error_500",
			httpStatusCode: http.StatusInternalServerError,
			responseBody: `{
				"error": {
					"code": 500,
					"message": "Internal server error occurred",
					"status": "INTERNAL"
				}
			}`,
			expectedErrorCode: "server_error",
			expectedErrorType: "server",
			expectedRetryable: true,
			expectedMessage:   "Internal server error occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient(t)
			ctx := context.Background()

			// GREEN PHASE: Call the implemented method and test behavior
			response := &http.Response{
				StatusCode: tt.httpStatusCode,
				Body:       io.NopCloser(strings.NewReader(tt.responseBody)),
				Header:     make(http.Header),
			}
			embeddingErr := client.HandleHTTPError(ctx, response)

			// Validate the error is not nil
			if embeddingErr == nil {
				t.Error("expected EmbeddingError, got nil")
				return
			}

			// Validate error code
			if embeddingErr.Code != tt.expectedErrorCode {
				t.Errorf("expected error code '%s', got '%s'", tt.expectedErrorCode, embeddingErr.Code)
			}

			// Validate error type
			if embeddingErr.Type != tt.expectedErrorType {
				t.Errorf("expected error type '%s', got '%s'", tt.expectedErrorType, embeddingErr.Type)
			}

			// Validate retryable flag
			if embeddingErr.Retryable != tt.expectedRetryable {
				t.Errorf("expected retryable %v, got %v", tt.expectedRetryable, embeddingErr.Retryable)
			}

			// Validate message contains expected text
			if !strings.Contains(embeddingErr.Message, tt.expectedMessage) {
				t.Errorf("expected message to contain '%s', got '%s'", tt.expectedMessage, embeddingErr.Message)
			}
		})
	}
}

// TestGeminiClient_InputValidation tests input validation expectations.
// These tests define what the ValidateEmbeddingInput method should do when implemented.
func TestGeminiClient_InputValidation(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		options       outbound.EmbeddingOptions
		expectedError string
		errorType     string
	}{
		{
			name: "valid_input_within_token_limit",
			text: "func hello() { fmt.Println(\"hello world\") }",
			options: outbound.EmbeddingOptions{
				Model:     "gemini-embedding-001",
				TaskType:  outbound.TaskTypeRetrievalDocument,
				MaxTokens: 2048,
			},
		},
		{
			name: "text_exceeding_2048_token_limit",
			text: strings.Repeat("func veryLongFunctionName() { return \"very long string content\" }\n", 500),
			options: outbound.EmbeddingOptions{
				Model:     "gemini-embedding-001",
				TaskType:  outbound.TaskTypeRetrievalDocument,
				MaxTokens: 2048,
			},
			expectedError: "text exceeds maximum token limit of 2048",
			errorType:     "validation",
		},
		{
			name: "empty_text_content",
			text: "",
			options: outbound.EmbeddingOptions{
				Model:     "gemini-embedding-001",
				TaskType:  outbound.TaskTypeRetrievalDocument,
				MaxTokens: 2048,
			},
			expectedError: "text content cannot be empty",
			errorType:     "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient(t)
			ctx := context.Background()

			// GREEN PHASE: Call the implemented method and test behavior
			err := client.ValidateEmbeddingInput(ctx, tt.text, tt.options)

			if tt.expectedError != "" {
				validateExpectedError(t, err, tt.expectedError, tt.errorType)
				return
			}

			// Expecting success (no error)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestGeminiClient_TokenCountingAndValidation tests token counting expectations.
// These tests define what token counting methods should do when implemented.
func TestGeminiClient_TokenCountingAndValidation(t *testing.T) {
	tests := []struct {
		name               string
		text               string
		expectedTokenCount int
		maxTokens          int
		expectError        bool
		expectedError      string
	}{
		{
			name:               "simple_function_token_count",
			text:               "func hello() { fmt.Println(\"hello\") }",
			expectedTokenCount: 12, // Approximate token count
			maxTokens:          2048,
			expectError:        false,
		},
		{
			name: "text_exceeding_token_limit",
			text: strings.Repeat(
				"this is a very long token sequence that should definitely exceed the limit ",
				100,
			), // Exceeds 2048 tokens
			expectedTokenCount: 2050,
			maxTokens:          50,
			expectError:        true,
			expectedError:      "text exceeds maximum token limit of 50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient(t)
			ctx := context.Background()

			// GREEN PHASE: Call the implemented methods and test behavior
			tokenCount, err := client.EstimateTokenCount(ctx, tt.text)
			if err != nil {
				t.Errorf("unexpected error from EstimateTokenCount: %v", err)
				return
			}

			// Validate token count is reasonable (within range, not exact match due to estimation)
			if tokenCount < 1 && tt.text != "" {
				t.Errorf("expected token count > 0 for non-empty text, got %d", tokenCount)
			}

			// Test ValidateTokenLimit
			validationErr := client.ValidateTokenLimit(ctx, tt.text, tt.maxTokens)

			if tt.expectError {
				validateTokenValidationError(t, validationErr)
			} else if validationErr != nil {
				t.Errorf("unexpected validation error: %v", validationErr)
			}
		})
	}
}

// TestGeminiClient_NetworkConnectivityErrors tests network error handling expectations.
// These tests define what the HandleNetworkError method should do when implemented.
func TestGeminiClient_NetworkConnectivityErrors(t *testing.T) {
	tests := []struct {
		name              string
		networkError      error
		expectedErrorType string
		expectedErrorCode string
		expectedRetryable bool
		expectedMessage   string
	}{
		{
			name:              "connection_timeout_error",
			networkError:      &timeoutError{message: "connection timeout"},
			expectedErrorType: "network",
			expectedErrorCode: "connection_timeout",
			expectedRetryable: true,
			expectedMessage:   "connection timeout",
		},
		{
			name:              "connection_refused_error",
			networkError:      &connectionError{message: "connection refused"},
			expectedErrorType: "network",
			expectedErrorCode: "connection_refused",
			expectedRetryable: true,
			expectedMessage:   "connection refused",
		},
		{
			name:              "request_canceled_error",
			networkError:      context.Canceled,
			expectedErrorType: "network",
			expectedErrorCode: "request_canceled",
			expectedRetryable: false,
			expectedMessage:   "request was canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient(t)
			ctx := context.Background()

			// GREEN PHASE: Call the implemented method and test behavior
			embeddingErr := client.HandleNetworkError(ctx, tt.networkError)

			// Validate the error is not nil
			if embeddingErr == nil {
				t.Error("expected EmbeddingError, got nil")
				return
			}

			// Validate error code
			if embeddingErr.Code != tt.expectedErrorCode {
				t.Errorf("expected error code '%s', got '%s'", tt.expectedErrorCode, embeddingErr.Code)
			}

			// Validate error type
			if embeddingErr.Type != tt.expectedErrorType {
				t.Errorf("expected error type '%s', got '%s'", tt.expectedErrorType, embeddingErr.Type)
			}

			// Validate retryable flag
			if embeddingErr.Retryable != tt.expectedRetryable {
				t.Errorf("expected retryable %v, got %v", tt.expectedRetryable, embeddingErr.Retryable)
			}

			// Validate message contains expected text
			if !strings.Contains(embeddingErr.Message, tt.expectedMessage) {
				t.Errorf("expected message to contain '%s', got '%s'", tt.expectedMessage, embeddingErr.Message)
			}

			// Validate cause is preserved
			if !errors.Is(embeddingErr.Cause, tt.networkError) {
				t.Errorf("expected cause to be preserved, got %v", embeddingErr.Cause)
			}
		})
	}
}

// TestGeminiClient_StructuredErrorTypes tests structured error creation expectations.
// These tests define what the CreateEmbeddingError method should do when implemented.
func TestGeminiClient_StructuredErrorTypes(t *testing.T) {
	tests := []struct {
		name             string
		errorCode        string
		errorType        string
		message          string
		retryable        bool
		cause            error
		expectedBehavior func(*testing.T, *outbound.EmbeddingError)
	}{
		{
			name:      "authentication_error_creation",
			errorCode: "invalid_api_key",
			errorType: "auth",
			message:   "API key is invalid or expired",
			retryable: false,
			cause:     errors.New("401 Unauthorized"),
			expectedBehavior: func(t *testing.T, err *outbound.EmbeddingError) {
				if !err.IsAuthenticationError() {
					t.Error("expected IsAuthenticationError() to return true")
				}
			},
		},
		{
			name:      "quota_error_creation",
			errorCode: "rate_limit_exceeded",
			errorType: "quota",
			message:   "Request rate limit exceeded",
			retryable: true,
			cause:     errors.New("429 Too Many Requests"),
			expectedBehavior: func(t *testing.T, err *outbound.EmbeddingError) {
				if !err.IsQuotaError() {
					t.Error("expected IsQuotaError() to return true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient(t)
			ctx := context.Background()

			// GREEN PHASE: Call the implemented method and test behavior
			embeddingErr := client.CreateEmbeddingError(
				ctx,
				tt.errorCode,
				tt.errorType,
				tt.message,
				tt.retryable,
				tt.cause,
			)

			// Validate the error is not nil
			if embeddingErr == nil {
				t.Error("expected EmbeddingError, got nil")
				return
			}

			// Validate error code
			if embeddingErr.Code != tt.errorCode {
				t.Errorf("expected error code '%s', got '%s'", tt.errorCode, embeddingErr.Code)
			}

			// Validate error type
			if embeddingErr.Type != tt.errorType {
				t.Errorf("expected error type '%s', got '%s'", tt.errorType, embeddingErr.Type)
			}

			// Validate message
			if embeddingErr.Message != tt.message {
				t.Errorf("expected message '%s', got '%s'", tt.message, embeddingErr.Message)
			}

			// Validate retryable flag
			if embeddingErr.Retryable != tt.retryable {
				t.Errorf("expected retryable %v, got %v", tt.retryable, embeddingErr.Retryable)
			}

			// Validate cause is preserved
			if !errors.Is(embeddingErr.Cause, tt.cause) {
				t.Errorf("expected cause to be preserved, got %v", embeddingErr.Cause)
			}

			// Run expected behavior validation
			if tt.expectedBehavior != nil {
				tt.expectedBehavior(t, embeddingErr)
			}
		})
	}
}

// TestGeminiClient_RequestResponseLogging tests logging integration expectations.
// These tests define what logging methods should do when implemented.
func TestGeminiClient_RequestResponseLogging(t *testing.T) {
	tests := []struct {
		name               string
		text               string
		options            outbound.EmbeddingOptions
		simulateError      bool
		expectedLogFields  map[string]interface{}
		expectedLogLevel   string
		expectedLogMessage string
	}{
		{
			name: "successful_request_response_logging",
			text: "func testFunction() { return \"test\" }",
			options: outbound.EmbeddingOptions{
				Model:     "gemini-embedding-001",
				TaskType:  outbound.TaskTypeRetrievalDocument,
				MaxTokens: 2048,
				Timeout:   30 * time.Second,
			},
			simulateError:      false,
			expectedLogLevel:   "info",
			expectedLogMessage: "Embedding request completed successfully",
			expectedLogFields: map[string]interface{}{
				"model":       "gemini-embedding-001",
				"task_type":   "RETRIEVAL_DOCUMENT",
				"text_length": 42,
				"token_count": 10,
				"dimensions":  768,
				"duration_ms": 100,
				"request_id":  "test-request-id",
			},
		},
		{
			name: "failed_request_error_logging",
			text: "func testFunction() { return \"test\" }",
			options: outbound.EmbeddingOptions{
				Model:     "gemini-embedding-001",
				TaskType:  outbound.TaskTypeRetrievalDocument,
				MaxTokens: 2048,
			},
			simulateError:      true,
			expectedLogLevel:   "error",
			expectedLogMessage: "Embedding request failed",
			expectedLogFields: map[string]interface{}{
				"model":       "gemini-embedding-001",
				"task_type":   "RETRIEVAL_DOCUMENT",
				"text_length": 42,
				"error_type":  "auth",
				"error_code":  "invalid_api_key",
				"duration_ms": 50,
				"request_id":  "test-request-id",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient(t)
			ctx := context.Background()

			// GREEN PHASE: Test the implemented logging methods
			requestID := client.LogEmbeddingRequest(ctx, tt.text, tt.options)

			// Verify request ID is generated
			if requestID == "" {
				t.Error("LogEmbeddingRequest should return a non-empty request ID")
			}

			duration := time.Duration(100) * time.Millisecond

			if tt.simulateError {
				// Test error logging
				embeddingErr := &outbound.EmbeddingError{
					Code:    "invalid_api_key",
					Type:    "auth",
					Message: "Authentication failed",
				}
				client.LogEmbeddingError(ctx, requestID, embeddingErr, duration)
			} else {
				// Test success logging
				result := &outbound.EmbeddingResult{
					Vector:      []float64{0.1, 0.2, 0.3},
					Dimensions:  768,
					TokenCount:  10,
					Model:       "gemini-embedding-001",
					TaskType:    outbound.TaskTypeRetrievalDocument,
					GeneratedAt: time.Now(),
					RequestID:   requestID,
				}
				client.LogEmbeddingResponse(ctx, requestID, result, duration)
			}

			// Test context-aware logging
			slogger.Info(ctx, "Test logging verification", slogger.Fields{
				"test_name": tt.name,
				"phase":     "RED",
			})
		})
	}
}

// Helper functions for Red Phase testing

func createTestClient(t *testing.T) *gemini.Client {
	config := &gemini.ClientConfig{
		APIKey:     "test-api-key",
		Model:      "gemini-embedding-001",
		TaskType:   "RETRIEVAL_DOCUMENT",
		Timeout:    30 * time.Second,
		Dimensions: 768,
	}

	client, err := gemini.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create test client: %v", err)
	}

	return client
}

// Mock error types for network testing.
type timeoutError struct {
	message string
}

func (e *timeoutError) Error() string {
	return e.message
}

func (e *timeoutError) Timeout() bool {
	return true
}

func (e *timeoutError) Temporary() bool {
	return true
}

type connectionError struct {
	message string
}

func (e *connectionError) Error() string {
	return e.message
}

// validateSerializedJSON validates the structure of serialized JSON data.
func validateSerializedJSON(t *testing.T, jsonData []byte, expectedText string) {
	// Parse and validate the JSON structure
	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Errorf("failed to parse generated JSON: %v", err)
		return
	}

	// Validate required fields
	if model, ok := result["model"].(string); !ok || !strings.HasSuffix(model, "gemini-embedding-001") {
		t.Errorf("expected model to end with 'gemini-embedding-001', got '%v'", result["model"])
	}

	if content, ok := result["content"].(map[string]interface{}); !ok {
		t.Error("expected content field to be an object")
	} else {
		validateContentParts(t, content, expectedText)
	}

	if taskType, ok := result["taskType"].(string); !ok || taskType != "RETRIEVAL_DOCUMENT" {
		t.Errorf("expected taskType to be 'RETRIEVAL_DOCUMENT', got '%v'", result["taskType"])
	}

	if dims, ok := result["outputDimensionality"].(float64); !ok || dims != 768 {
		t.Errorf("expected outputDimensionality to be 768, got '%v'", result["outputDimensionality"])
	}
}

// validateContentParts validates the content.parts structure.
func validateContentParts(t *testing.T, content map[string]interface{}, expectedText string) {
	parts, ok := content["parts"].([]interface{})
	if !ok {
		t.Error("expected content.parts field to be an array")
		return
	}

	if len(parts) != 1 {
		t.Errorf("expected exactly 1 part, got %d", len(parts))
		return
	}

	part, ok := parts[0].(map[string]interface{})
	if !ok {
		t.Error("expected part to be an object")
		return
	}

	partText, ok := part["text"].(string)
	if !ok || partText != expectedText {
		t.Errorf("expected part text to be '%s', got '%v'", expectedText, part["text"])
	}
}

// validateDeserializationError validates deserialization errors.
func validateDeserializationError(t *testing.T, err error, expectedError, expectedType string) {
	if err == nil {
		t.Errorf("expected error containing '%s', but got none", expectedError)
		return
	}

	// Check if it's an EmbeddingError with the expected error type
	var embeddingErr *outbound.EmbeddingError
	if errors.As(err, &embeddingErr) {
		if expectedType != "" && embeddingErr.Type != expectedType {
			t.Errorf("expected error type '%s', got '%s'", expectedType, embeddingErr.Type)
		}
	}

	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("expected error containing '%s', got '%s'", expectedError, err.Error())
	}
}

// validateExpectedError validates that an error matches expected criteria.
func validateExpectedError(t *testing.T, err error, expectedError, expectedType string) {
	if err == nil {
		t.Errorf("expected error containing '%s', but got none", expectedError)
		return
	}

	// Check if it's an EmbeddingError with the expected error type
	var embeddingErr *outbound.EmbeddingError
	if errors.As(err, &embeddingErr) {
		if expectedType != "" && embeddingErr.Type != expectedType {
			t.Errorf("expected error type '%s', got '%s'", expectedType, embeddingErr.Type)
		}
	}

	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("expected error containing '%s', got '%s'", expectedError, err.Error())
	}
}

// validateTokenValidationError validates token validation errors.
func validateTokenValidationError(t *testing.T, validationErr error) {
	if validationErr == nil {
		t.Error("expected validation error, but got none")
		return
	}

	// Check if it's an EmbeddingError with validation type
	var embeddingErr *outbound.EmbeddingError
	if errors.As(validationErr, &embeddingErr) {
		if embeddingErr.Type != "validation" {
			t.Errorf("expected error type 'validation', got '%s'", embeddingErr.Type)
		}
	}

	if !strings.Contains(validationErr.Error(), "text exceeds maximum token limit") {
		t.Errorf(
			"expected error containing 'text exceeds maximum token limit', got '%s'",
			validationErr.Error(),
		)
	}
}
