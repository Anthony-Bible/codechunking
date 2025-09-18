package gemini_test

import (
	"codechunking/internal/adapter/outbound/gemini"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
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

			// RED PHASE: This method doesn't exist yet - test should fail
			t.Error("SerializeEmbeddingRequest method does not exist yet - needs implementation in GREEN phase")

			// Define expected behavior for GREEN phase implementation:
			// jsonData, err := client.SerializeEmbeddingRequest(ctx, tt.text, tt.options)
			// The method should:
			// 1. Validate input text is not empty
			// 2. Validate model is supported (gemini-embedding-001)
			// 3. Validate task type is valid
			// 4. Create proper JSON structure with model, content.parts, taskType, outputDimensionality
			// 5. Return serialized JSON bytes and nil error for valid inputs
			// 6. Return nil bytes and appropriate error for invalid inputs

			_ = client // Use variables to avoid linter complaints
			_ = ctx
			_ = tt.expectedJSON
			_ = tt.shouldValidate
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

			// RED PHASE: This method doesn't exist yet - test should fail
			t.Error("DeserializeEmbeddingResponse method does not exist yet - needs implementation in GREEN phase")

			// Define expected behavior for GREEN phase implementation:
			// result, err := client.DeserializeEmbeddingResponse(ctx, []byte(tt.responseBody))
			// The method should:
			// 1. Parse JSON response body
			// 2. Validate required fields (embedding, values)
			// 3. Extract float64 array from values field
			// 4. Create EmbeddingResult with proper vector, dimensions, model info
			// 5. Return structured EmbeddingError for validation/parsing failures
			// 6. Return nil result and error for invalid JSON structures

			_ = client
			_ = ctx
			_ = tt.expectedEmbedding
			_ = tt.errorType
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
			expectedErrorCode: "internal_server_error",
			expectedErrorType: "server",
			expectedRetryable: true,
			expectedMessage:   "Internal server error occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient(t)
			ctx := context.Background()

			// RED PHASE: This method doesn't exist yet - test should fail
			t.Error("HandleHTTPError method does not exist yet - needs implementation in GREEN phase")

			// Define expected behavior for GREEN phase implementation:
			// response := &http.Response{
			//     StatusCode: tt.httpStatusCode,
			//     Body:       io.NopCloser(strings.NewReader(tt.responseBody)),
			//     Header:     make(http.Header),
			// }
			// embeddingErr := client.HandleHTTPError(ctx, response)
			// The method should:
			// 1. Parse HTTP error response JSON
			// 2. Map HTTP status codes to appropriate error types
			// 3. Extract error details (code, message, status)
			// 4. Create EmbeddingError with correct type, code, message, retryable flag
			// 5. Handle malformed error responses gracefully
			// 6. Set appropriate retryable flag based on error type

			_ = client
			_ = ctx
			_ = tt.expectedErrorCode
			_ = tt.expectedErrorType
			_ = tt.expectedRetryable
			_ = tt.expectedMessage
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
			text: strings.Repeat("func veryLongFunctionName() { return \"very long string content\" }\n", 100),
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

			// RED PHASE: This method doesn't exist yet - test should fail
			t.Error("ValidateEmbeddingInput method does not exist yet - needs implementation in GREEN phase")

			// Define expected behavior for GREEN phase implementation:
			// err := client.ValidateEmbeddingInput(ctx, tt.text, tt.options)
			// The method should:
			// 1. Validate text is not empty or whitespace-only
			// 2. Validate model is supported
			// 3. Validate task type is valid
			// 4. Estimate token count and validate against MaxTokens limit
			// 5. Check for invalid characters (null bytes, etc.)
			// 6. Return EmbeddingError with validation type for failures
			// 7. Return nil for valid inputs

			_ = client
			_ = ctx
			_ = tt.expectedError
			_ = tt.errorType
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
			name:               "text_exceeding_token_limit",
			text:               strings.Repeat("token ", 410), // Exceeds 2048 tokens
			expectedTokenCount: 2050,
			maxTokens:          2048,
			expectError:        true,
			expectedError:      "text exceeds maximum token limit of 2048 (estimated: 2050 tokens)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createTestClient(t)
			ctx := context.Background()

			// RED PHASE: These methods don't exist yet - tests should fail
			t.Error(
				"EstimateTokenCount and ValidateTokenLimit methods do not exist yet - need implementation in GREEN phase",
			)

			// Define expected behavior for GREEN phase implementation:
			// tokenCount, err := client.EstimateTokenCount(ctx, tt.text)
			// validationErr := client.ValidateTokenLimit(ctx, tt.text, tt.maxTokens)
			// The methods should:
			// 1. EstimateTokenCount: Approximate token count using character/word-based estimation
			// 2. Handle unicode characters appropriately
			// 3. ValidateTokenLimit: Check estimated count against limit
			// 4. Return appropriate validation errors for limit exceeded
			// 5. Handle edge cases (empty text, very long text)

			_ = client
			_ = ctx
			_ = tt.expectedTokenCount
			_ = tt.expectError
			_ = tt.expectedError
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

			// RED PHASE: This method doesn't exist yet - test should fail
			t.Error("HandleNetworkError method does not exist yet - needs implementation in GREEN phase")

			// Define expected behavior for GREEN phase implementation:
			// embeddingErr := client.HandleNetworkError(ctx, tt.networkError)
			// The method should:
			// 1. Identify different types of network errors
			// 2. Map to appropriate error codes (timeout, refused, dns, etc.)
			// 3. Set retryable flag based on error type
			// 4. Preserve original error as cause
			// 5. Create EmbeddingError with network type
			// 6. Handle context cancellation and deadline exceeded

			_ = client
			_ = ctx
			_ = tt.networkError
			_ = tt.expectedErrorType
			_ = tt.expectedErrorCode
			_ = tt.expectedRetryable
			_ = tt.expectedMessage
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

			// RED PHASE: This method doesn't exist yet - test should fail
			t.Error("CreateEmbeddingError method does not exist yet - needs implementation in GREEN phase")

			// Define expected behavior for GREEN phase implementation:
			// embeddingErr := client.CreateEmbeddingError(ctx, tt.errorCode, tt.errorType, tt.message, tt.retryable, tt.cause)
			// The method should:
			// 1. Create EmbeddingError with all specified fields
			// 2. Set RequestID from context if available
			// 3. Preserve cause error for unwrapping
			// 4. Ensure proper Error() string formatting
			// 5. Support all helper methods (IsAuthenticationError, etc.)

			_ = client
			_ = ctx
			_ = tt.errorCode
			_ = tt.errorType
			_ = tt.message
			_ = tt.retryable
			_ = tt.cause
			_ = tt.expectedBehavior
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

			// RED PHASE: These methods don't exist yet - tests should fail
			t.Error(
				"LogEmbeddingRequest, LogEmbeddingResponse, and LogEmbeddingError methods do not exist yet - need implementation in GREEN phase",
			)

			// Define expected behavior for GREEN phase implementation:
			// requestID := client.LogEmbeddingRequest(ctx, tt.text, tt.options)
			// client.LogEmbeddingResponse(ctx, requestID, result, duration)
			// client.LogEmbeddingError(ctx, requestID, embeddingErr, duration)
			// The methods should:
			// 1. Generate unique request IDs
			// 2. Log structured fields using slogger package
			// 3. Include performance metrics (duration, token counts)
			// 4. Use appropriate log levels (info for success, error for failures)
			// 5. Include context-aware correlation IDs
			// 6. Log both request and response details

			_ = client
			_ = ctx
			_ = tt.simulateError
			_ = tt.expectedLogFields
			_ = tt.expectedLogLevel
			_ = tt.expectedLogMessage

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
		APIKey:    "test-api-key",
		BaseURL:   "https://generativelanguage.googleapis.com/v1beta",
		Model:     "gemini-embedding-001",
		TaskType:  "RETRIEVAL_DOCUMENT",
		Timeout:   30 * time.Second,
		MaxTokens: 2048,
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

type connectionError struct {
	message string
}

func (e *connectionError) Error() string {
	return e.message
}
