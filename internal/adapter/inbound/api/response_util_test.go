package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"

	"codechunking/internal/application/dto"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test data structures for testing various data types
type TestStruct struct {
	ID       uuid.UUID              `json:"id"`
	Name     string                 `json:"name"`
	Count    int                    `json:"count"`
	Active   bool                   `json:"active"`
	Created  time.Time              `json:"created"`
	Metadata map[string]interface{} `json:"metadata"`
}

type UnencodableStruct struct {
	Channel chan int    `json:"channel"` // Channels are not JSON encodable
	Func    func() bool `json:"func"`    // Functions are not JSON encodable
}

func TestWriteJSON_SuccessfulBasicResponse(t *testing.T) {
	// This test should fail initially because WriteJSON function doesn't exist yet
	recorder := httptest.NewRecorder()
	data := map[string]string{"message": "success", "status": "ok"}

	err := WriteJSON(recorder, http.StatusOK, data)

	require.NoError(t, err, "WriteJSON should not return error for valid data")
	assert.Equal(t, http.StatusOK, recorder.Code, "Response status should be 200")
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"), "Content-Type should be application/json")

	var response map[string]string
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err, "Response body should be valid JSON")
	assert.Equal(t, "success", response["message"])
	assert.Equal(t, "ok", response["status"])
}

func TestWriteJSON_VariousDataTypes(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		data        interface{}
		expectedErr bool
		validateFn  func(t *testing.T, recorder *httptest.ResponseRecorder, data interface{})
	}{
		{
			name:       "struct_with_complex_types",
			statusCode: http.StatusCreated,
			data: TestStruct{
				ID:       uuid.New(),
				Name:     "Test Repository",
				Count:    42,
				Active:   true,
				Created:  time.Now().UTC(),
				Metadata: map[string]interface{}{"key": "value", "number": 123},
			},
			expectedErr: false,
			validateFn: func(t *testing.T, recorder *httptest.ResponseRecorder, originalData interface{}) {
				var decoded TestStruct
				err := json.Unmarshal(recorder.Body.Bytes(), &decoded)
				require.NoError(t, err)
				original := originalData.(TestStruct)
				assert.Equal(t, original.ID, decoded.ID)
				assert.Equal(t, original.Name, decoded.Name)
				assert.Equal(t, original.Count, decoded.Count)
				assert.Equal(t, original.Active, decoded.Active)
			},
		},
		{
			name:       "slice_of_objects",
			statusCode: http.StatusOK,
			data: []dto.RepositoryResponse{
				{ID: uuid.New(), URL: "https://github.com/user/repo1", Status: "active"},
				{ID: uuid.New(), URL: "https://github.com/user/repo2", Status: "pending"},
			},
			expectedErr: false,
			validateFn: func(t *testing.T, recorder *httptest.ResponseRecorder, originalData interface{}) {
				var decoded []dto.RepositoryResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &decoded)
				require.NoError(t, err)
				assert.Len(t, decoded, 2)
				assert.Equal(t, "active", decoded[0].Status)
				assert.Equal(t, "pending", decoded[1].Status)
			},
		},
		{
			name:        "primitive_string",
			statusCode:  http.StatusOK,
			data:        "simple string response",
			expectedErr: false,
			validateFn: func(t *testing.T, recorder *httptest.ResponseRecorder, originalData interface{}) {
				var decoded string
				err := json.Unmarshal(recorder.Body.Bytes(), &decoded)
				require.NoError(t, err)
				assert.Equal(t, "simple string response", decoded)
			},
		},
		{
			name:        "primitive_number",
			statusCode:  http.StatusOK,
			data:        12345,
			expectedErr: false,
			validateFn: func(t *testing.T, recorder *httptest.ResponseRecorder, originalData interface{}) {
				var decoded int
				err := json.Unmarshal(recorder.Body.Bytes(), &decoded)
				require.NoError(t, err)
				assert.Equal(t, 12345, decoded)
			},
		},
		{
			name:        "nil_data",
			statusCode:  http.StatusNoContent,
			data:        nil,
			expectedErr: false,
			validateFn: func(t *testing.T, recorder *httptest.ResponseRecorder, originalData interface{}) {
				bodyBytes := recorder.Body.Bytes()
				expectedJSON := []byte("null\n") // JSON encoding of nil produces "null"
				assert.Equal(t, expectedJSON, bodyBytes)
			},
		},
		{
			name:        "empty_map",
			statusCode:  http.StatusOK,
			data:        map[string]interface{}{},
			expectedErr: false,
			validateFn: func(t *testing.T, recorder *httptest.ResponseRecorder, originalData interface{}) {
				var decoded map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &decoded)
				require.NoError(t, err)
				assert.Len(t, decoded, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			err := WriteJSON(recorder, tt.statusCode, tt.data)

			if tt.expectedErr {
				assert.Error(t, err, "WriteJSON should return error for case: %s", tt.name)
			} else {
				require.NoError(t, err, "WriteJSON should not return error for case: %s", tt.name)
				assert.Equal(t, tt.statusCode, recorder.Code, "Status code should match for case: %s", tt.name)
				assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"), "Content-Type should be application/json for case: %s", tt.name)

				if tt.validateFn != nil {
					tt.validateFn(t, recorder, tt.data)
				}
			}
		})
	}
}

func TestWriteJSON_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		data        interface{}
		expectedErr bool
		errorMsg    string
	}{
		{
			name:        "unencodable_channel",
			statusCode:  http.StatusOK,
			data:        UnencodableStruct{Channel: make(chan int), Func: func() bool { return true }},
			expectedErr: true,
			errorMsg:    "json: unsupported type",
		},
		{
			name:        "function_type",
			statusCode:  http.StatusOK,
			data:        func() { return },
			expectedErr: true,
			errorMsg:    "json: unsupported type",
		},
		{
			name:       "complex_unencodable_data",
			statusCode: http.StatusOK,
			data: map[string]interface{}{
				"valid_field":   "this is fine",
				"invalid_field": make(chan int),
			},
			expectedErr: true,
			errorMsg:    "json: unsupported type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			err := WriteJSON(recorder, tt.statusCode, tt.data)

			if tt.expectedErr {
				require.Error(t, err, "WriteJSON should return error for unencodable data")
				assert.Contains(t, err.Error(), tt.errorMsg, "Error message should indicate JSON encoding issue")
				// When encoding fails, no headers or body should be written
				assert.Equal(t, 0, recorder.Code, "Status code should not be set when encoding fails")
				assert.Empty(t, recorder.Header().Get("Content-Type"), "Content-Type should not be set when encoding fails")
				assert.Empty(t, recorder.Body.String(), "Body should be empty when encoding fails")
			} else {
				assert.NoError(t, err, "WriteJSON should not return error for case: %s", tt.name)
			}
		})
	}
}

func TestWriteJSON_HeaderAndStatusHandling(t *testing.T) {
	t.Run("sets_content_type_header", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		data := map[string]string{"test": "data"}

		err := WriteJSON(recorder, http.StatusAccepted, data)

		require.NoError(t, err)
		assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	})

	t.Run("overwrites_existing_content_type", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		recorder.Header().Set("Content-Type", "text/plain") // Set different content type first
		data := map[string]string{"test": "data"}

		err := WriteJSON(recorder, http.StatusCreated, data)

		require.NoError(t, err)
		assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	})

	t.Run("sets_correct_status_codes", func(t *testing.T) {
		statusCodes := []int{
			http.StatusOK,
			http.StatusCreated,
			http.StatusAccepted,
			http.StatusNoContent,
			http.StatusBadRequest,
			http.StatusNotFound,
			http.StatusConflict,
			http.StatusInternalServerError,
			http.StatusServiceUnavailable,
		}

		for _, code := range statusCodes {
			t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
				recorder := httptest.NewRecorder()
				data := map[string]int{"status_code": code}

				err := WriteJSON(recorder, code, data)

				require.NoError(t, err)
				assert.Equal(t, code, recorder.Code, "Status code should be set correctly")
			})
		}
	})

	t.Run("preserves_other_headers", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		recorder.Header().Set("X-Custom-Header", "custom-value")
		recorder.Header().Set("X-Request-ID", "12345")
		data := map[string]string{"message": "test"}

		err := WriteJSON(recorder, http.StatusOK, data)

		require.NoError(t, err)
		assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
		assert.Equal(t, "custom-value", recorder.Header().Get("X-Custom-Header"))
		assert.Equal(t, "12345", recorder.Header().Get("X-Request-ID"))
	})
}

func TestWriteJSON_ConcurrentAccess(t *testing.T) {
	// Test thread safety by calling WriteJSON concurrently
	const numGoroutines = 50
	const numCallsPerGoroutine = 10

	var wg sync.WaitGroup
	results := make([]error, numGoroutines*numCallsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numCallsPerGoroutine; j++ {
				recorder := httptest.NewRecorder()
				data := map[string]interface{}{
					"goroutine_id": goroutineID,
					"call_number":  j,
					"timestamp":    time.Now().UnixNano(),
				}

				err := WriteJSON(recorder, http.StatusOK, data)

				// Store result safely (this test assumes proper synchronization in WriteJSON)
				idx := goroutineID*numCallsPerGoroutine + j
				results[idx] = err

				if err == nil {
					// Verify response was written correctly
					assert.Equal(t, http.StatusOK, recorder.Code)
					assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

					var response map[string]interface{}
					decodeErr := json.Unmarshal(recorder.Body.Bytes(), &response)
					assert.NoError(t, decodeErr, "Response should be valid JSON")
					if decodeErr == nil {
						assert.Equal(t, float64(goroutineID), response["goroutine_id"]) // JSON numbers are float64
						assert.Equal(t, float64(j), response["call_number"])
					}
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify all calls succeeded
	for i, err := range results {
		assert.NoError(t, err, "Concurrent call %d should not fail", i)
	}
}

func TestWriteJSON_PoolingBehavior(t *testing.T) {
	// This is a creative test to verify that pooling is working
	// We'll monitor memory allocation patterns and encoder reuse

	t.Run("memory_efficiency_with_repeated_calls", func(t *testing.T) {
		// Force garbage collection before starting
		runtime.GC()
		runtime.GC() // Call twice to ensure cleanup

		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)

		// Make many calls that should benefit from pooling
		const numCalls = 1000
		for i := 0; i < numCalls; i++ {
			recorder := httptest.NewRecorder()
			data := map[string]interface{}{
				"iteration": i,
				"data":      fmt.Sprintf("test_data_%d", i),
				"nested": map[string]interface{}{
					"value": i * 2,
					"desc":  "nested data",
				},
			}

			err := WriteJSON(recorder, http.StatusOK, data)
			require.NoError(t, err, "Call %d should succeed", i)
		}

		runtime.GC()
		runtime.ReadMemStats(&m2)

		// With pooling, memory allocation should be much lower than without pooling
		// This is an approximate test - the exact threshold may need adjustment
		allocatedBytes := m2.TotalAlloc - m1.TotalAlloc

		// This test will initially fail because we expect pooled encoders to significantly
		// reduce memory allocations compared to creating new encoders each time
		t.Logf("Memory allocated during %d WriteJSON calls: %d bytes", numCalls, allocatedBytes)

		// We expect that with pooling, we shouldn't allocate more than ~5MB for 1000 calls
		// Without pooling, this would be significantly higher (10-20MB+)
		maxExpectedAllocation := uint64(5 * 1024 * 1024) // 5MB threshold
		assert.Less(t, allocatedBytes, maxExpectedAllocation,
			"With pooling, memory allocation should be efficient. Allocated: %d bytes, Expected: < %d bytes",
			allocatedBytes, maxExpectedAllocation)
	})

	t.Run("encoder_reuse_verification", func(t *testing.T) {
		// This test tries to verify encoder reuse by checking that
		// rapid consecutive calls don't create excessive objects

		// We'll make calls in quick succession and verify they complete efficiently
		const rapidCalls = 100
		start := time.Now()

		for i := 0; i < rapidCalls; i++ {
			recorder := httptest.NewRecorder()
			data := map[string]int{"call": i}

			err := WriteJSON(recorder, http.StatusOK, data)
			require.NoError(t, err)

			// Verify the response is correct
			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
		}

		duration := time.Since(start)

		// With pooling, this should be very fast
		// This test will initially fail as we expect pooled performance
		maxExpectedDuration := 50 * time.Millisecond
		assert.Less(t, duration, maxExpectedDuration,
			"Rapid consecutive calls should be fast with pooling. Duration: %v, Expected: < %v",
			duration, maxExpectedDuration)
	})
}

func TestWriteJSON_LargePayload(t *testing.T) {
	// Test handling of larger JSON payloads
	recorder := httptest.NewRecorder()

	// Create a reasonably large payload
	largeData := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		largeData[fmt.Sprintf("field_%d", i)] = map[string]interface{}{
			"id":          uuid.New(),
			"description": fmt.Sprintf("This is test data item number %d with some additional text to make it larger", i),
			"metadata":    map[string]interface{}{"nested": i, "value": i * 10},
		}
	}

	err := WriteJSON(recorder, http.StatusOK, largeData)

	require.NoError(t, err, "WriteJSON should handle large payloads")
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	// Verify the response is valid JSON
	var decodedData map[string]interface{}
	err = json.Unmarshal(recorder.Body.Bytes(), &decodedData)
	require.NoError(t, err, "Large response should be valid JSON")
	assert.Len(t, decodedData, 1000, "All fields should be present in response")
}

func TestWriteJSON_EdgeCases(t *testing.T) {
	t.Run("multiple_writes_to_same_recorder", func(t *testing.T) {
		// This test verifies behavior when WriteJSON is called multiple times
		// on the same ResponseRecorder (which can happen in error scenarios)
		recorder := httptest.NewRecorder()

		// First write
		err1 := WriteJSON(recorder, http.StatusOK, map[string]string{"first": "call"})
		require.NoError(t, err1)

		// Second write to same recorder - behavior depends on implementation
		// This might succeed or fail depending on how we handle multiple writes
		err2 := WriteJSON(recorder, http.StatusCreated, map[string]string{"second": "call"})

		// The test should define the expected behavior - let's say we expect
		// the second call to succeed but only the first response is written
		// (this is typical HTTP behavior)
		require.NoError(t, err2, "Second WriteJSON call should not error")

		// The status code should remain from the first call
		assert.Equal(t, http.StatusOK, recorder.Code, "Status should remain from first call")
	})

	t.Run("empty_string_data", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		err := WriteJSON(recorder, http.StatusOK, "")

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

		var decoded string
		err = json.Unmarshal(recorder.Body.Bytes(), &decoded)
		require.NoError(t, err)
		assert.Equal(t, "", decoded)
	})

	t.Run("zero_status_code", func(t *testing.T) {
		// Test behavior with status code 0 (which defaults to 200 in HTTP)
		recorder := httptest.NewRecorder()

		err := WriteJSON(recorder, 0, map[string]string{"status": "zero"})

		require.NoError(t, err)
		// Status code 0 should be treated as 200 by HTTP
		assert.Equal(t, http.StatusOK, recorder.Code)
	})
}

// Mock ResponseWriter that fails to test error handling
type failingResponseWriter struct {
	*httptest.ResponseRecorder
	failOnWrite bool
}

func (f *failingResponseWriter) Write(p []byte) (int, error) {
	if f.failOnWrite {
		return 0, fmt.Errorf("simulated write failure")
	}
	return f.ResponseRecorder.Write(p)
}

func TestWriteJSON_ResponseWriterFailures(t *testing.T) {
	t.Run("write_failure", func(t *testing.T) {
		// Test behavior when the ResponseWriter.Write fails
		failingWriter := &failingResponseWriter{
			ResponseRecorder: httptest.NewRecorder(),
			failOnWrite:      true,
		}

		data := map[string]string{"test": "data"}
		err := WriteJSON(failingWriter, http.StatusOK, data)

		// We expect WriteJSON to return the write error
		require.Error(t, err, "WriteJSON should return error when Write fails")
		assert.Contains(t, err.Error(), "simulated write failure", "Error should indicate write failure")
	})
}

// Benchmark tests to verify pooling performance benefits
func BenchmarkWriteJSON_WithoutPooling(b *testing.B) {
	data := map[string]interface{}{
		"id":      uuid.New(),
		"message": "benchmark test message",
		"count":   12345,
		"active":  true,
		"metadata": map[string]interface{}{
			"key1": "value1",
			"key2": 678.9,
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()

		// Simulate non-pooled approach
		recorder.Header().Set("Content-Type", "application/json")
		recorder.WriteHeader(http.StatusOK)
		encoder := json.NewEncoder(recorder)
		_ = encoder.Encode(data)
	}
}

func BenchmarkWriteJSON_WithPooling(b *testing.B) {
	data := map[string]interface{}{
		"id":      uuid.New(),
		"message": "benchmark test message",
		"count":   12345,
		"active":  true,
		"metadata": map[string]interface{}{
			"key1": "value1",
			"key2": 678.9,
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		_ = WriteJSON(recorder, http.StatusOK, data)
	}
}
