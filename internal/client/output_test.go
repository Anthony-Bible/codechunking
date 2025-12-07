package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// Test helper functions to reduce cognitive complexity.

func parseJSONResponse(t *testing.T, output string) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}
	return resp
}

func assertSuccess(t *testing.T, resp map[string]interface{}, expected bool) {
	t.Helper()
	if resp["success"] != expected {
		t.Errorf("Expected success=%v, got %v", expected, resp["success"])
	}
}

func assertFieldOmitted(t *testing.T, resp map[string]interface{}, field string) {
	t.Helper()
	if _, exists := resp[field]; exists {
		t.Errorf("Expected '%s' field to be omitted, but it was present", field)
	}
}

func assertFieldPresent(t *testing.T, resp map[string]interface{}, field string) {
	t.Helper()
	if _, exists := resp[field]; !exists {
		t.Errorf("Expected '%s' field to be present", field)
	}
}

func getErrorObject(t *testing.T, resp map[string]interface{}) map[string]interface{} {
	t.Helper()
	errorObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected error to be map, got %T", resp["error"])
	}
	return errorObj
}

func getDataAsMap(t *testing.T, resp map[string]interface{}) map[string]interface{} {
	t.Helper()
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data to be map, got %T", resp["data"])
	}
	return data
}

func TestResponse_JSONMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		response Response
		want     map[string]interface{}
	}{
		{
			name: "successful response with simple data",
			response: Response{
				Success:   true,
				Data:      map[string]string{"id": "123", "status": "completed"},
				Error:     nil,
				Timestamp: time.Date(2025, 12, 4, 12, 0, 0, 0, time.UTC),
			},
			want: map[string]interface{}{
				"success":   true,
				"data":      map[string]interface{}{"id": "123", "status": "completed"},
				"timestamp": "2025-12-04T12:00:00Z",
			},
		},
		{
			name: "error response without data",
			response: Response{
				Success: false,
				Data:    nil,
				Error: &Error{
					Code:    "REPOSITORY_NOT_FOUND",
					Message: "Repository with ID 123 not found",
					Details: nil,
				},
				Timestamp: time.Date(2025, 12, 4, 12, 0, 0, 0, time.UTC),
			},
			want: map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "REPOSITORY_NOT_FOUND",
					"message": "Repository with ID 123 not found",
				},
				"timestamp": "2025-12-04T12:00:00Z",
			},
		},
		{
			name: "error response with details",
			response: Response{
				Success: false,
				Data:    nil,
				Error: &Error{
					Code:    "VALIDATION_ERROR",
					Message: "Invalid request parameters",
					Details: map[string]interface{}{
						"fields": []string{"url", "branch"},
						"reason": "required fields missing",
					},
				},
				Timestamp: time.Date(2025, 12, 4, 12, 0, 0, 0, time.UTC),
			},
			want: map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "VALIDATION_ERROR",
					"message": "Invalid request parameters",
					"details": map[string]interface{}{
						"fields": []interface{}{"url", "branch"},
						"reason": "required fields missing",
					},
				},
				"timestamp": "2025-12-04T12:00:00Z",
			},
		},
		{
			name: "successful response with nil data should omit data field",
			response: Response{
				Success:   true,
				Data:      nil,
				Error:     nil,
				Timestamp: time.Date(2025, 12, 4, 12, 0, 0, 0, time.UTC),
			},
			want: map[string]interface{}{
				"success":   true,
				"timestamp": "2025-12-04T12:00:00Z",
			},
		},
		{
			name: "successful response with complex nested data",
			response: Response{
				Success: true,
				Data: map[string]interface{}{
					"repository": map[string]interface{}{
						"id":   "repo-123",
						"name": "test-repo",
						"stats": map[string]interface{}{
							"files":  100,
							"chunks": 1000,
						},
					},
					"chunks": []map[string]interface{}{
						{"id": "chunk-1", "content": "test content 1"},
						{"id": "chunk-2", "content": "test content 2"},
					},
				},
				Error:     nil,
				Timestamp: time.Date(2025, 12, 4, 12, 0, 0, 0, time.UTC),
			},
			want: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"repository": map[string]interface{}{
						"id":   "repo-123",
						"name": "test-repo",
						"stats": map[string]interface{}{
							"files":  float64(100),
							"chunks": float64(1000),
						},
					},
					"chunks": []interface{}{
						map[string]interface{}{"id": "chunk-1", "content": "test content 1"},
						map[string]interface{}{"id": "chunk-2", "content": "test content 2"},
					},
				},
				"timestamp": "2025-12-04T12:00:00Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.response)
			if err != nil {
				t.Fatalf("Failed to marshal response: %v", err)
			}

			var got map[string]interface{}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			verifyFieldOmission(t, got, tt.want, "data")
			verifyFieldOmission(t, got, tt.want, "error")
			compareJSON(t, got, tt.want)
		})
	}
}

func verifyFieldOmission(t *testing.T, got, want map[string]interface{}, field string) {
	t.Helper()
	if want[field] == nil {
		if _, exists := got[field]; exists {
			t.Errorf("Expected '%s' field to be omitted, but it was present", field)
		}
	}
}

func compareJSON(t *testing.T, got, want map[string]interface{}) {
	t.Helper()
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)

	if string(gotJSON) != string(wantJSON) {
		t.Errorf("JSON mismatch:\nwant: %s\ngot:  %s", wantJSON, gotJSON)
	}
}

func TestError_JSONMarshaling(t *testing.T) {
	tests := []struct {
		name  string
		error Error
		want  map[string]interface{}
	}{
		{
			name: "error with code and message only",
			error: Error{
				Code:    "INTERNAL_ERROR",
				Message: "An internal error occurred",
				Details: nil,
			},
			want: map[string]interface{}{
				"code":    "INTERNAL_ERROR",
				"message": "An internal error occurred",
			},
		},
		{
			name: "error with string details",
			error: Error{
				Code:    "NETWORK_ERROR",
				Message: "Failed to connect to server",
				Details: "connection timeout after 30s",
			},
			want: map[string]interface{}{
				"code":    "NETWORK_ERROR",
				"message": "Failed to connect to server",
				"details": "connection timeout after 30s",
			},
		},
		{
			name: "error with structured details",
			error: Error{
				Code:    "BATCH_PROCESSING_ERROR",
				Message: "Batch processing failed",
				Details: map[string]interface{}{
					"batch_id":      "batch-456",
					"failed_chunks": 50,
					"total_chunks":  1000,
				},
			},
			want: map[string]interface{}{
				"code":    "BATCH_PROCESSING_ERROR",
				"message": "Batch processing failed",
				"details": map[string]interface{}{
					"batch_id":      "batch-456",
					"failed_chunks": float64(50),
					"total_chunks":  float64(1000),
				},
			},
		},
		{
			name: "error with empty strings",
			error: Error{
				Code:    "",
				Message: "",
				Details: nil,
			},
			want: map[string]interface{}{
				"code":    "",
				"message": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.error)
			if err != nil {
				t.Fatalf("Failed to marshal error: %v", err)
			}

			var got map[string]interface{}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Failed to unmarshal error: %v", err)
			}

			verifyFieldOmission(t, got, tt.want, "details")
			compareJSON(t, got, tt.want)
		})
	}
}

func TestWriteSuccess(t *testing.T) {
	t.Run("simple string data", func(t *testing.T) {
		output := runWriteSuccess(t, "operation completed")
		resp := parseJSONResponse(t, output)

		assertSuccess(t, resp, true)
		assertFieldOmitted(t, resp, "error")
		assertFieldPresent(t, resp, "timestamp")

		if resp["data"] != "operation completed" {
			t.Errorf("Expected data='operation completed', got %v", resp["data"])
		}
	})

	t.Run("map data", func(t *testing.T) {
		data := map[string]interface{}{
			"repository_id": "repo-789",
			"status":        "indexed",
			"chunk_count":   5000,
		}
		output := runWriteSuccess(t, data)
		resp := parseJSONResponse(t, output)

		assertSuccess(t, resp, true)
		dataMap := getDataAsMap(t, resp)

		if dataMap["repository_id"] != "repo-789" {
			t.Errorf("Expected repository_id='repo-789', got %v", dataMap["repository_id"])
		}
		if dataMap["status"] != "indexed" {
			t.Errorf("Expected status='indexed', got %v", dataMap["status"])
		}
		if dataMap["chunk_count"] != float64(5000) {
			t.Errorf("Expected chunk_count=5000, got %v", dataMap["chunk_count"])
		}
	})

	t.Run("nil data should omit data field", func(t *testing.T) {
		output := runWriteSuccess(t, nil)
		resp := parseJSONResponse(t, output)

		assertSuccess(t, resp, true)
		assertFieldOmitted(t, resp, "data")
	})

	t.Run("complex nested data structure", func(t *testing.T) {
		data := map[string]interface{}{
			"repositories": []map[string]interface{}{
				{"id": "1", "name": "repo1"},
				{"id": "2", "name": "repo2"},
			},
			"pagination": map[string]interface{}{
				"page":      1,
				"page_size": 20,
				"total":     100,
				"has_next":  true,
				"has_prev":  false,
			},
		}
		output := runWriteSuccess(t, data)
		resp := parseJSONResponse(t, output)

		assertSuccess(t, resp, true)
		dataMap := getDataAsMap(t, resp)

		repos, ok := dataMap["repositories"].([]interface{})
		if !ok {
			t.Fatalf("Expected repositories to be array, got %T", dataMap["repositories"])
		}
		if len(repos) != 2 {
			t.Errorf("Expected 2 repositories, got %d", len(repos))
		}
	})
}

func runWriteSuccess(t *testing.T, data interface{}) string {
	t.Helper()
	var buf bytes.Buffer
	beforeWrite := time.Now()

	err := WriteSuccess(&buf, data)
	if err != nil {
		t.Fatalf("WriteSuccess returned error: %v", err)
	}

	afterWrite := time.Now()
	output := buf.String()

	validateJSONAndTimestamp(t, output, beforeWrite, afterWrite)
	return output
}

func validateJSONAndTimestamp(t *testing.T, output string, before, after time.Time) {
	t.Helper()
	if !json.Valid([]byte(output)) {
		t.Errorf("Produced invalid JSON: %s", output)
	}

	var resp struct {
		Timestamp time.Time `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("Failed to unmarshal timestamp: %v", err)
	}

	if resp.Timestamp.IsZero() {
		t.Errorf("Expected non-zero timestamp")
	}

	if resp.Timestamp.Before(before.Add(-time.Second)) ||
		resp.Timestamp.After(after.Add(time.Second)) {
		t.Errorf("Timestamp %v is outside expected range [%v, %v]", resp.Timestamp, before, after)
	}
}

func TestWriteError_WithoutDetails(t *testing.T) {
	output := runWriteError(t, "NOT_FOUND", "Resource not found", nil)
	resp := parseJSONResponse(t, output)

	assertSuccess(t, resp, false)
	assertFieldOmitted(t, resp, "data")

	errorObj := getErrorObject(t, resp)
	if errorObj["code"] != "NOT_FOUND" {
		t.Errorf("Expected code='NOT_FOUND', got %v", errorObj["code"])
	}
	if errorObj["message"] != "Resource not found" {
		t.Errorf("Expected message='Resource not found', got %v", errorObj["message"])
	}
	if _, exists := errorObj["details"]; exists {
		t.Errorf("Expected details field to be omitted when nil")
	}
}

func TestWriteError_WithStringDetails(t *testing.T) {
	output := runWriteError(t, "TIMEOUT", "Operation timed out", "exceeded 30 second timeout")
	resp := parseJSONResponse(t, output)

	errorObj := getErrorObject(t, resp)
	if errorObj["code"] != "TIMEOUT" {
		t.Errorf("Expected code='TIMEOUT', got %v", errorObj["code"])
	}
	if errorObj["details"] != "exceeded 30 second timeout" {
		t.Errorf("Expected details='exceeded 30 second timeout', got %v", errorObj["details"])
	}
}

func TestWriteError_WithStructuredDetails(t *testing.T) {
	details := map[string]interface{}{
		"field":      "url",
		"value":      "not-a-url",
		"constraint": "must be valid URL",
	}
	output := runWriteError(t, "VALIDATION_ERROR", "Invalid input parameters", details)
	resp := parseJSONResponse(t, output)

	errorObj := getErrorObject(t, resp)
	detailsMap, ok := errorObj["details"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected details to be map, got %T", errorObj["details"])
	}
	if detailsMap["field"] != "url" {
		t.Errorf("Expected field='url', got %v", detailsMap["field"])
	}
	if detailsMap["value"] != "not-a-url" {
		t.Errorf("Expected value='not-a-url', got %v", detailsMap["value"])
	}
}

func TestWriteError_WithEmptyStrings(t *testing.T) {
	output := runWriteError(t, "", "", nil)
	resp := parseJSONResponse(t, output)

	assertSuccess(t, resp, false)
	errorObj := getErrorObject(t, resp)
	if errorObj["code"] != "" {
		t.Errorf("Expected code='', got %v", errorObj["code"])
	}
	if errorObj["message"] != "" {
		t.Errorf("Expected message='', got %v", errorObj["message"])
	}
}

func TestWriteError_WithArrayDetails(t *testing.T) {
	details := []string{
		"url is required",
		"branch must not be empty",
		"depth must be positive",
	}
	output := runWriteError(t, "MULTIPLE_ERRORS", "Multiple validation errors occurred", details)
	resp := parseJSONResponse(t, output)

	errorObj := getErrorObject(t, resp)
	detailsArr, ok := errorObj["details"].([]interface{})
	if !ok {
		t.Fatalf("Expected details to be array, got %T", errorObj["details"])
	}
	if len(detailsArr) != 3 {
		t.Errorf("Expected 3 error messages, got %d", len(detailsArr))
	}
	if detailsArr[0] != "url is required" {
		t.Errorf("Expected first error 'url is required', got %v", detailsArr[0])
	}
}

func runWriteError(t *testing.T, code, message string, details interface{}) string {
	t.Helper()
	var buf bytes.Buffer
	beforeWrite := time.Now()

	err := WriteError(&buf, code, message, details)
	if err != nil {
		t.Fatalf("WriteError returned error: %v", err)
	}

	afterWrite := time.Now()
	output := buf.String()

	validateJSONAndTimestamp(t, output, beforeWrite, afterWrite)
	return output
}

func TestWriteSuccess_WriterError(t *testing.T) {
	failWriter := &failingWriter{errToReturn: bytes.ErrTooLarge}

	err := WriteSuccess(failWriter, "test data")
	if err == nil {
		t.Error("Expected WriteSuccess to return error when writer fails, got nil")
	}

	if !errors.Is(err, bytes.ErrTooLarge) {
		t.Errorf("Expected error to be ErrTooLarge, got %v", err)
	}
}

func TestWriteError_WriterError(t *testing.T) {
	failWriter := &failingWriter{errToReturn: bytes.ErrTooLarge}

	err := WriteError(failWriter, "TEST_ERROR", "test message", nil)
	if err == nil {
		t.Error("Expected WriteError to return error when writer fails, got nil")
	}

	if !errors.Is(err, bytes.ErrTooLarge) {
		t.Errorf("Expected error to be ErrTooLarge, got %v", err)
	}
}

// failingWriter is a test helper that always returns an error.
type failingWriter struct {
	errToReturn error
}

func (fw *failingWriter) Write(_ []byte) (int, error) {
	return 0, fw.errToReturn
}
