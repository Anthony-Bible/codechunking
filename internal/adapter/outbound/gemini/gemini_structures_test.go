package gemini_test

import (
	"codechunking/internal/adapter/outbound/gemini"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// TestGeminiStructures_EmbedContentRequest tests the EmbedContentRequest structure.
func TestGeminiStructures_EmbedContentRequest(t *testing.T) {
	t.Run("single_text_request_structure", func(t *testing.T) {
		// This test should fail initially because the structs don't exist
		request := &gemini.EmbedContentRequest{
			Model: "models/gemini-embedding-001",
			Content: gemini.Content{
				Parts: []gemini.Part{
					{Text: "Sample Go function to embed"},
				},
			},
			TaskType:             "RETRIEVAL_DOCUMENT",
			OutputDimensionality: 768,
		}

		// Verify struct fields
		if request.Model != "models/gemini-embedding-001" {
			t.Errorf("expected Model %q, got %q", "models/gemini-embedding-001", request.Model)
		}

		if request.TaskType != "RETRIEVAL_DOCUMENT" {
			t.Errorf("expected TaskType %q, got %q", "RETRIEVAL_DOCUMENT", request.TaskType)
		}

		if request.OutputDimensionality != 768 {
			t.Errorf("expected OutputDimensionality %d, got %d", 768, request.OutputDimensionality)
		}

		if len(request.Content.Parts) != 1 {
			t.Errorf("expected 1 part, got %d", len(request.Content.Parts))
		}

		if request.Content.Parts[0].Text != "Sample Go function to embed" {
			t.Errorf("expected text %q, got %q", "Sample Go function to embed", request.Content.Parts[0].Text)
		}
	})

	t.Run("multiple_parts_request_structure", func(t *testing.T) {
		// This test should fail initially because the structs don't exist
		request := &gemini.EmbedContentRequest{
			Model: "models/gemini-embedding-001",
			Content: gemini.Content{
				Parts: []gemini.Part{
					{Text: "Function definition:"},
					{Text: "func calculateSum(a, b int) int { return a + b }"},
					{Text: "This function adds two integers"},
				},
			},
			TaskType: "SEMANTIC_SIMILARITY",
		}

		expectedTexts := []string{
			"Function definition:",
			"func calculateSum(a, b int) int { return a + b }",
			"This function adds two integers",
		}

		if len(request.Content.Parts) != 3 {
			t.Errorf("expected 3 parts, got %d", len(request.Content.Parts))
			return
		}

		for i, expectedText := range expectedTexts {
			if request.Content.Parts[i].Text != expectedText {
				t.Errorf("part[%d]: expected %q, got %q", i, expectedText, request.Content.Parts[i].Text)
			}
		}
	})

	t.Run("request_json_serialization", func(t *testing.T) {
		// This test should fail initially because the structs don't exist
		request := &gemini.EmbedContentRequest{
			Model: "models/gemini-embedding-001",
			Content: gemini.Content{
				Parts: []gemini.Part{
					{Text: "Test serialization"},
				},
			},
			TaskType:             "RETRIEVAL_DOCUMENT",
			OutputDimensionality: 768,
		}

		jsonData, err := json.Marshal(request)
		if err != nil {
			t.Errorf("unexpected error marshaling request: %v", err)
			return
		}

		// Verify JSON structure
		var unmarshaled map[string]interface{}
		if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
			t.Errorf("unexpected error unmarshaling JSON: %v", err)
			return
		}

		expectedFields := map[string]interface{}{
			"model":                "models/gemini-embedding-001",
			"taskType":             "RETRIEVAL_DOCUMENT",
			"outputDimensionality": float64(768),
		}

		for field, expected := range expectedFields {
			actual, exists := unmarshaled[field]
			if !exists {
				t.Errorf("expected field %q in JSON", field)
				continue
			}
			if actual != expected {
				t.Errorf("field %q: expected %v, got %v", field, expected, actual)
			}
		}

		// Verify content structure
		content, exists := unmarshaled["content"].(map[string]interface{})
		if !exists {
			t.Error("expected content field in JSON")
			return
		}

		parts, exists := content["parts"].([]interface{})
		if !exists {
			t.Error("expected parts array in content")
			return
		}

		if len(parts) != 1 {
			t.Errorf("expected 1 part in JSON, got %d", len(parts))
			return
		}

		part := parts[0].(map[string]interface{})
		if part["text"] != "Test serialization" {
			t.Errorf("expected part text %q, got %q", "Test serialization", part["text"])
		}
	})
}

// TestGeminiStructures_EmbedContentResponse tests the EmbedContentResponse structure.
func TestGeminiStructures_EmbedContentResponse(t *testing.T) {
	t.Run("response_structure", func(t *testing.T) {
		// This test should fail initially because the structs don't exist
		response := &gemini.EmbedContentResponse{
			Embedding: gemini.ContentEmbedding{
				Values: []float64{0.1, -0.2, 0.3, -0.4, 0.5, 0.6, -0.7, 0.8},
			},
		}

		expectedValues := []float64{0.1, -0.2, 0.3, -0.4, 0.5, 0.6, -0.7, 0.8}

		if len(response.Embedding.Values) != len(expectedValues) {
			t.Errorf("expected %d values, got %d", len(expectedValues), len(response.Embedding.Values))
			return
		}

		validateEmbeddingValues(t, response.Embedding.Values, expectedValues)
	})

	t.Run("response_with_768_dimensions", func(t *testing.T) {
		// Create a response with 768-dimensional vector (Gemini standard)
		values := make([]float64, 768)
		for i := range values {
			values[i] = float64(i) * 0.001 // Simple test pattern
		}

		// This test should fail initially because the structs don't exist
		response := &gemini.EmbedContentResponse{
			Embedding: gemini.ContentEmbedding{
				Values: values,
			},
		}

		if len(response.Embedding.Values) != 768 {
			t.Errorf("expected 768 dimensions, got %d", len(response.Embedding.Values))
		}

		// Verify pattern
		validateEmbeddingPattern(t, response.Embedding.Values)
	})

	t.Run("response_json_deserialization", func(t *testing.T) {
		jsonData := `{
			"embedding": {
				"values": [0.1, -0.2, 0.3, -0.4, 0.5]
			}
		}`

		// This test should fail initially because the structs don't exist
		var response gemini.EmbedContentResponse
		err := json.Unmarshal([]byte(jsonData), &response)
		if err != nil {
			t.Errorf("unexpected error unmarshaling response: %v", err)
			return
		}

		expectedValues := []float64{0.1, -0.2, 0.3, -0.4, 0.5}
		if len(response.Embedding.Values) != len(expectedValues) {
			t.Errorf("expected %d values, got %d", len(expectedValues), len(response.Embedding.Values))
			return
		}

		for i, expected := range expectedValues {
			if response.Embedding.Values[i] != expected {
				t.Errorf("value[%d]: expected %f, got %f", i, expected, response.Embedding.Values[i])
			}
		}
	})
}

// TestGeminiStructures_BatchRequests tests batch request/response structures.
func TestGeminiStructures_BatchRequests(t *testing.T) {
	t.Run("batch_request_structure", func(t *testing.T) {
		// This test should fail initially because the structs don't exist
		batchRequest := &gemini.BatchEmbedContentRequest{
			Requests: []gemini.EmbedContentRequest{
				{
					Model: "models/gemini-embedding-001",
					Content: gemini.Content{
						Parts: []gemini.Part{{Text: "First code snippet"}},
					},
					TaskType: "RETRIEVAL_DOCUMENT",
				},
				{
					Model: "models/gemini-embedding-001",
					Content: gemini.Content{
						Parts: []gemini.Part{{Text: "Second code snippet"}},
					},
					TaskType: "RETRIEVAL_DOCUMENT",
				},
				{
					Model: "models/gemini-embedding-001",
					Content: gemini.Content{
						Parts: []gemini.Part{{Text: "Third code snippet"}},
					},
					TaskType: "RETRIEVAL_DOCUMENT",
				},
			},
		}

		if len(batchRequest.Requests) != 3 {
			t.Errorf("expected 3 requests, got %d", len(batchRequest.Requests))
			return
		}

		expectedTexts := []string{
			"First code snippet",
			"Second code snippet",
			"Third code snippet",
		}

		validateBatchRequestTexts(t, batchRequest, expectedTexts)
	})

	t.Run("batch_response_structure", func(t *testing.T) {
		// This test should fail initially because the structs don't exist
		batchResponse := &gemini.BatchEmbedContentResponse{
			Embeddings: []gemini.ContentEmbedding{
				{Values: []float64{0.1, 0.2, 0.3}},
				{Values: []float64{0.4, 0.5, 0.6}},
				{Values: []float64{0.7, 0.8, 0.9}},
			},
		}

		if len(batchResponse.Embeddings) != 3 {
			t.Errorf("expected 3 embeddings, got %d", len(batchResponse.Embeddings))
			return
		}

		expectedValues := [][]float64{
			{0.1, 0.2, 0.3},
			{0.4, 0.5, 0.6},
			{0.7, 0.8, 0.9},
		}

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

	t.Run("batch_request_json_serialization", func(t *testing.T) {
		// This test should fail initially because the structs don't exist
		batchRequest := &gemini.BatchEmbedContentRequest{
			Requests: []gemini.EmbedContentRequest{
				{
					Model: "models/gemini-embedding-001",
					Content: gemini.Content{
						Parts: []gemini.Part{{Text: "Batch item 1"}},
					},
					TaskType: "RETRIEVAL_DOCUMENT",
				},
				{
					Model: "models/gemini-embedding-001",
					Content: gemini.Content{
						Parts: []gemini.Part{{Text: "Batch item 2"}},
					},
					TaskType: "RETRIEVAL_DOCUMENT",
				},
			},
		}

		jsonData, err := json.Marshal(batchRequest)
		if err != nil {
			t.Errorf("unexpected error marshaling batch request: %v", err)
			return
		}

		var unmarshaled map[string]interface{}
		if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
			t.Errorf("unexpected error unmarshaling JSON: %v", err)
			return
		}

		requests, exists := unmarshaled["requests"].([]interface{})
		if !exists {
			t.Error("expected requests array in JSON")
			return
		}

		if len(requests) != 2 {
			t.Errorf("expected 2 requests in JSON, got %d", len(requests))
		}
	})
}

// TestGeminiStructures_ErrorResponse tests error response structures.
func TestGeminiStructures_ErrorResponse(t *testing.T) {
	t.Run("error_response_structure", func(t *testing.T) {
		// This test should fail initially because the structs don't exist
		errorResponse := &gemini.ErrorResponse{
			Error: gemini.ErrorDetails{
				Code:    400,
				Message: "Invalid request: missing required field",
				Status:  "INVALID_ARGUMENT",
				Details: []gemini.ErrorDetail{
					{
						Type:        "type.googleapis.com/google.rpc.BadRequest",
						Description: "The request was malformed",
						Field:       "content.parts",
						Reason:      "REQUIRED_FIELD_MISSING",
					},
					{
						Type:        "type.googleapis.com/google.rpc.FieldViolation",
						Description: "Field validation failed",
						Field:       "taskType",
						Reason:      "INVALID_VALUE",
					},
				},
			},
		}

		// Verify main error fields
		if errorResponse.Error.Code != 400 {
			t.Errorf("expected error code 400, got %d", errorResponse.Error.Code)
		}

		if errorResponse.Error.Message != "Invalid request: missing required field" {
			t.Errorf(
				"expected error message %q, got %q",
				"Invalid request: missing required field",
				errorResponse.Error.Message,
			)
		}

		if errorResponse.Error.Status != "INVALID_ARGUMENT" {
			t.Errorf("expected error status %q, got %q", "INVALID_ARGUMENT", errorResponse.Error.Status)
		}

		// Verify error details
		if len(errorResponse.Error.Details) != 2 {
			t.Errorf("expected 2 error details, got %d", len(errorResponse.Error.Details))
			return
		}

		firstDetail := errorResponse.Error.Details[0]
		if firstDetail.Type != "type.googleapis.com/google.rpc.BadRequest" {
			t.Errorf(
				"expected first detail type %q, got %q",
				"type.googleapis.com/google.rpc.BadRequest",
				firstDetail.Type,
			)
		}

		if firstDetail.Field != "content.parts" {
			t.Errorf("expected first detail field %q, got %q", "content.parts", firstDetail.Field)
		}

		if firstDetail.Reason != "REQUIRED_FIELD_MISSING" {
			t.Errorf("expected first detail reason %q, got %q", "REQUIRED_FIELD_MISSING", firstDetail.Reason)
		}
	})

	t.Run("error_response_json_deserialization", func(t *testing.T) {
		jsonData := `{
			"error": {
				"code": 401,
				"message": "Request had invalid authentication credentials.",
				"status": "UNAUTHENTICATED",
				"details": [
					{
						"@type": "type.googleapis.com/google.rpc.ErrorInfo",
						"reason": "ACCESS_TOKEN_EXPIRED",
						"domain": "googleapis.com"
					}
				]
			}
		}`

		// This test should fail initially because the structs don't exist
		var errorResponse gemini.ErrorResponse
		err := json.Unmarshal([]byte(jsonData), &errorResponse)
		if err != nil {
			t.Errorf("unexpected error unmarshaling error response: %v", err)
			return
		}

		if errorResponse.Error.Code != 401 {
			t.Errorf("expected error code 401, got %d", errorResponse.Error.Code)
		}

		if errorResponse.Error.Status != "UNAUTHENTICATED" {
			t.Errorf("expected status %q, got %q", "UNAUTHENTICATED", errorResponse.Error.Status)
		}
	})

	t.Run("quota_exceeded_error_structure", func(t *testing.T) {
		// This test should fail initially because the structs don't exist
		quotaError := &gemini.ErrorResponse{
			Error: gemini.ErrorDetails{
				Code:    429,
				Message: "Quota exceeded for requests per minute",
				Status:  "RESOURCE_EXHAUSTED",
				Details: []gemini.ErrorDetail{
					{
						Type:        "type.googleapis.com/google.rpc.QuotaFailure",
						Description: "Request quota exceeded",
						Reason:      "QUOTA_EXCEEDED",
						Metadata: map[string]string{
							"service":        "generativelanguage.googleapis.com",
							"quota_limit":    "1000",
							"quota_location": "global",
						},
					},
				},
			},
		}

		if quotaError.Error.Code != 429 {
			t.Errorf("expected error code 429, got %d", quotaError.Error.Code)
		}

		if quotaError.Error.Status != "RESOURCE_EXHAUSTED" {
			t.Errorf("expected status %q, got %q", "RESOURCE_EXHAUSTED", quotaError.Error.Status)
		}

		if len(quotaError.Error.Details) != 1 {
			t.Errorf("expected 1 error detail, got %d", len(quotaError.Error.Details))
			return
		}

		detail := quotaError.Error.Details[0]
		if detail.Reason != "QUOTA_EXCEEDED" {
			t.Errorf("expected reason %q, got %q", "QUOTA_EXCEEDED", detail.Reason)
		}
	})
}

// TestGeminiStructures_ConfigurationStructure tests the client configuration structure.
func TestGeminiStructures_ConfigurationStructure(t *testing.T) {
	t.Run("client_config_structure", func(t *testing.T) {
		// This test should fail initially because the struct doesn't exist
		config := &gemini.ClientConfig{
			APIKey:          "test-api-key-12345",
			BaseURL:         "https://generativelanguage.googleapis.com/v1beta",
			Model:           "gemini-embedding-001",
			TaskType:        "RETRIEVAL_DOCUMENT",
			Timeout:         30 * time.Second,
			MaxRetries:      3,
			MaxTokens:       2048,
			Dimensions:      768,
			UserAgent:       "CodeChunking/1.0.0",
			EnableBatching:  true,
			BatchSize:       5,
			RateLimitBuffer: 100 * time.Millisecond,
		}

		// Verify all fields
		expectedFields := map[string]interface{}{
			"APIKey":          "test-api-key-12345",
			"BaseURL":         "https://generativelanguage.googleapis.com/v1beta",
			"Model":           "gemini-embedding-001",
			"TaskType":        "RETRIEVAL_DOCUMENT",
			"Timeout":         30 * time.Second,
			"MaxRetries":      3,
			"MaxTokens":       2048,
			"Dimensions":      768,
			"UserAgent":       "CodeChunking/1.0.0",
			"EnableBatching":  true,
			"BatchSize":       5,
			"RateLimitBuffer": 100 * time.Millisecond,
		}

		// Use reflection to verify fields (this helper will fail initially)
		verifyAllConfigFields(t, config, expectedFields)
	})

	t.Run("config_validation_rules", func(t *testing.T) {
		tests := []struct {
			name          string
			config        *gemini.ClientConfig
			expectedError string
		}{
			{
				name: "valid_config",
				config: &gemini.ClientConfig{
					APIKey:   "valid-key",
					BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
					Model:    "gemini-embedding-001",
					TaskType: "RETRIEVAL_DOCUMENT",
				},
				expectedError: "",
			},
			{
				name: "missing_api_key",
				config: &gemini.ClientConfig{
					BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
					Model:    "gemini-embedding-001",
					TaskType: "RETRIEVAL_DOCUMENT",
				},
				expectedError: "API key cannot be empty",
			},
			{
				name: "invalid_model",
				config: &gemini.ClientConfig{
					APIKey:   "valid-key",
					BaseURL:  "https://generativelanguage.googleapis.com/v1beta",
					Model:    "invalid-model",
					TaskType: "RETRIEVAL_DOCUMENT",
				},
				expectedError: "unsupported model",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// This test should fail initially because validation doesn't exist
				err := tt.config.Validate()
				assertValidationResult(t, err, tt.expectedError)
			})
		}
	})
}

// assertValidationResult checks validation results and reports test errors.
func assertValidationResult(t *testing.T, err error, expectedError string) {
	if expectedError == "" {
		if err != nil {
			t.Errorf("unexpected validation error: %v", err)
		}
		return
	}

	if err == nil {
		t.Errorf("expected validation error %q, got nil", expectedError)
		return
	}

	if err.Error() != expectedError {
		t.Errorf("expected validation error %q, got %q", expectedError, err.Error())
	}
}

// verifyAllConfigFields validates all expected fields in a config struct.
func verifyAllConfigFields(t *testing.T, config *gemini.ClientConfig, expectedFields map[string]interface{}) {
	for field, expected := range expectedFields {
		if !verifyConfigField(config, field, expected) {
			t.Errorf("config field %s: expected %v", field, expected)
		}
	}
}

// validateBatchRequestTexts validates the text content of batch requests.
func validateBatchRequestTexts(t *testing.T, batchRequest *gemini.BatchEmbedContentRequest, expectedTexts []string) {
	for i, expectedText := range expectedTexts {
		actualText := batchRequest.Requests[i].Content.Parts[0].Text
		if actualText != expectedText {
			t.Errorf("request[%d] text: expected %q, got %q", i, expectedText, actualText)
		}
	}
}

// validateEmbeddingValues validates embedding vector values.
func validateEmbeddingValues(t *testing.T, actual []float64, expected []float64) {
	for i, expectedVal := range expected {
		if actual[i] != expectedVal {
			t.Errorf("embedding value[%d]: expected %f, got %f", i, expectedVal, actual[i])
		}
	}
}

// validateEmbeddingPattern validates first 10 values of embedding follow expected pattern.
func validateEmbeddingPattern(t *testing.T, values []float64) {
	for i := range 10 { // Check first 10 values
		expected := float64(i) * 0.001
		if values[i] != expected {
			t.Errorf("value[%d]: expected %f, got %f", i, expected, values[i])
		}
	}
}

// Helper function using reflection for field verification.
func verifyConfigField(config *gemini.ClientConfig, fieldName string, expectedValue interface{}) bool {
	if config == nil {
		return false
	}

	configValue := reflect.ValueOf(config).Elem()
	field := configValue.FieldByName(fieldName)
	if !field.IsValid() {
		return false
	}

	actualValue := field.Interface()
	return reflect.DeepEqual(actualValue, expectedValue)
}
