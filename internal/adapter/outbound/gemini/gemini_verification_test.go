package gemini_test

import (
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestClientVerification performs a high-level sanity check of the embedding client.
func TestClientVerification(t *testing.T) {
	client := createTestClient(t)
	ctx := context.Background()

	t.Run("SerializeEmbeddingRequest_Works", func(t *testing.T) {
		text := "func calculateSum(a, b int) int { return a + b }"
		options := outbound.EmbeddingOptions{
			Model:     "gemini-embedding-001",
			TaskType:  outbound.TaskTypeRetrievalDocument,
			MaxTokens: 2048,
			Timeout:   30 * time.Second,
		}

		jsonData, err := client.SerializeEmbeddingRequest(ctx, text, options)
		if err != nil {
			t.Fatalf("SerializeEmbeddingRequest failed: %v", err)
		}

		// Verify JSON structure
		var req map[string]interface{}
		if err := json.Unmarshal(jsonData, &req); err != nil {
			t.Fatalf("Failed to parse serialized JSON: %v", err)
		}

		if req["model"] != "models/gemini-embedding-001" {
			t.Errorf("Expected model 'models/gemini-embedding-001', got %v", req["model"])
		}

		if req["taskType"] != "RETRIEVAL_DOCUMENT" {
			t.Errorf("Expected taskType 'RETRIEVAL_DOCUMENT', got %v", req["taskType"])
		}

		if req["outputDimensionality"] != float64(768) {
			t.Errorf("Expected outputDimensionality 768, got %v", req["outputDimensionality"])
		}
	})

	t.Run("DeserializeEmbeddingResponse_Works", func(t *testing.T) {
		responseBody := `{
			"embedding": {
				"values": [0.1, 0.2, 0.3, 0.4, 0.5]
			}
		}`

		result, err := client.DeserializeEmbeddingResponse(ctx, []byte(responseBody))
		if err != nil {
			t.Fatalf("DeserializeEmbeddingResponse failed: %v", err)
		}

		if len(result.Vector) != 5 {
			t.Errorf("Expected 5 dimensions, got %d", len(result.Vector))
		}

		expected := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
		for i, v := range result.Vector {
			if v != expected[i] {
				t.Errorf("Expected vector[%d] = %f, got %f", i, expected[i], v)
			}
		}
	})

	t.Run("ValidateEmbeddingInput_Works", func(t *testing.T) {
		// Valid input
		options := outbound.EmbeddingOptions{
			Model:     "gemini-embedding-001",
			TaskType:  outbound.TaskTypeRetrievalDocument,
			MaxTokens: 2048,
		}

		err := client.ValidateEmbeddingInput(ctx, "valid text", options)
		if err != nil {
			t.Errorf("Expected no error for valid input, got: %v", err)
		}

		// Empty text
		err = client.ValidateEmbeddingInput(ctx, "", options)
		if err == nil {
			t.Error("Expected error for empty text")
		} else if !strings.Contains(err.Error(), "text content cannot be empty") {
			t.Errorf("Expected 'empty text' error, got: %v", err)
		}
	})

	t.Run("EstimateTokenCount_Works", func(t *testing.T) {
		// Test simple text
		count, err := client.EstimateTokenCount(ctx, "hello world")
		if err != nil {
			t.Fatalf("EstimateTokenCount failed: %v", err)
		}

		if count <= 0 {
			t.Errorf("Expected positive token count, got %d", count)
		}

		// Test empty text
		count, err = client.EstimateTokenCount(ctx, "")
		if err != nil {
			t.Fatalf("EstimateTokenCount failed for empty text: %v", err)
		}

		if count != 0 {
			t.Errorf("Expected 0 tokens for empty text, got %d", count)
		}
	})

	t.Run("ValidateTokenLimit_Works", func(t *testing.T) {
		// Short text within limit
		err := client.ValidateTokenLimit(ctx, "short text", 100)
		if err != nil {
			t.Errorf("Expected no error for short text, got: %v", err)
		}

		// Long text exceeding limit
		longText := strings.Repeat("word ", 1000)
		err = client.ValidateTokenLimit(ctx, longText, 10)
		if err == nil {
			t.Error("Expected error for text exceeding token limit")
		} else if !strings.Contains(err.Error(), "exceeds maximum token limit") {
			t.Errorf("Expected token limit error, got: %v", err)
		}
	})

	t.Run("CreateEmbeddingError_Works", func(t *testing.T) {
		embErr := client.CreateEmbeddingError(ctx, "test_code", "test_type", "test message", true, nil)

		if embErr.Code != "test_code" {
			t.Errorf("Expected code 'test_code', got %s", embErr.Code)
		}

		if embErr.Type != "test_type" {
			t.Errorf("Expected type 'test_type', got %s", embErr.Type)
		}

		if embErr.Message != "test message" {
			t.Errorf("Expected message 'test message', got %s", embErr.Message)
		}

		if !embErr.Retryable {
			t.Error("Expected retryable to be true")
		}
	})

	t.Run("LoggingMethods_Work", func(t *testing.T) {
		options := outbound.EmbeddingOptions{
			Model:     "gemini-embedding-001",
			TaskType:  outbound.TaskTypeRetrievalDocument,
			MaxTokens: 2048,
		}

		// Test logging methods don't crash
		requestID := client.LogEmbeddingRequest(ctx, "test text", options)
		if requestID == "" {
			t.Error("Expected non-empty request ID")
		}

		result := &outbound.EmbeddingResult{
			Model:      "gemini-embedding-001",
			TaskType:   outbound.TaskTypeRetrievalDocument,
			Dimensions: 768,
		}

		// These should not crash
		client.LogEmbeddingResponse(ctx, requestID, result, time.Second)

		embErr := &outbound.EmbeddingError{
			Code:    "test_error",
			Type:    "validation",
			Message: "test message",
		}
		client.LogEmbeddingError(ctx, requestID, embErr, time.Second)
	})
}
