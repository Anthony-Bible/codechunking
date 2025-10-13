package gemini_test

import (
	"codechunking/internal/adapter/outbound/gemini"
	"codechunking/internal/port/outbound"
	"context"
	"reflect"
	"sync"
	"testing"
	"time"
	"unsafe"
)

// TestClientCaching_GenaiClientCreatedDuringInitialization tests that NewClient creates
// and caches a genai.Client instance during initialization.
//
// RED PHASE: This test WILL FAIL because:
// 1. The Client struct does not have a genaiClient field
// 2. NewClient does not create or cache a genai.Client
// 3. The client is currently created per-request in createGenaiClient().
func TestClientCaching_GenaiClientCreatedDuringInitialization(t *testing.T) {
	tests := []struct {
		name          string
		config        *gemini.ClientConfig
		expectError   bool
		errorContains string
	}{
		{
			name: "valid_config_creates_cached_client",
			config: &gemini.ClientConfig{
				APIKey:     "test-api-key-for-caching",
				Model:      "gemini-embedding-001",
				TaskType:   "RETRIEVAL_DOCUMENT",
				Timeout:    30 * time.Second,
				Dimensions: 768,
			},
			expectError: false,
		},
		{
			name: "minimal_config_creates_cached_client",
			config: &gemini.ClientConfig{
				APIKey: "minimal-api-key",
			},
			expectError: false,
		},
		{
			name: "invalid_api_key_fails_initialization",
			config: &gemini.ClientConfig{
				APIKey: "",
			},
			expectError:   true,
			errorContains: "API key cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := gemini.NewClient(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorContains)
					return
				}
				if tt.errorContains != "" && err.Error() != tt.errorContains {
					t.Errorf("expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error during client initialization: %v", err)
				return
			}

			if client == nil {
				t.Error("expected non-nil client")
				return
			}

			// RED PHASE: This will fail because the genaiClient field does not exist
			// We need to verify that a genai.Client was created and cached in the struct
			genaiClient := extractGenaiClientField(t, client)
			if genaiClient == nil {
				t.Error("expected genaiClient field to be populated during initialization, got nil")
			}
		})
	}
}

// TestClientCaching_GenaiClientReusedAcrossMultipleCalls tests that the same genai.Client
// instance is reused across multiple GenerateEmbedding calls.
//
// RED PHASE: This test WILL FAIL because:
// 1. createGenaiClient() creates a new client for every request
// 2. There is no caching mechanism in place
// 3. The client instance changes between calls.
func TestClientCaching_GenaiClientReusedAcrossMultipleCalls(t *testing.T) {
	config := &gemini.ClientConfig{
		APIKey:     "test-reuse-api-key",
		Model:      "gemini-embedding-001",
		TaskType:   "RETRIEVAL_DOCUMENT",
		Timeout:    30 * time.Second,
		Dimensions: 768,
	}

	client, err := gemini.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	// Make multiple embedding requests
	numRequests := 5
	clientPointers := make([]uintptr, numRequests)

	for i := range numRequests {
		// Note: These calls will fail with auth errors, but that's expected
		// We're testing client reuse, not API functionality
		_, _ = client.GenerateEmbedding(ctx, "test text for reuse", outbound.EmbeddingOptions{})

		// RED PHASE: Extract the genai.Client instance after each call
		// This will show that a NEW client is created each time (not reused)
		genaiClient := extractGenaiClientField(t, client)
		if genaiClient != nil {
			clientPointers[i] = getPointerAddress(genaiClient)
		}
	}

	// RED PHASE: Verify all requests used the SAME genai.Client instance
	// This will FAIL because each request creates a new client
	firstPointer := clientPointers[0]
	if firstPointer == 0 {
		t.Fatal("could not extract genai.Client pointer from first request")
	}

	for i := 1; i < numRequests; i++ {
		if clientPointers[i] != firstPointer {
			t.Errorf("request %d used different genai.Client instance (pointer: %x) than request 0 (pointer: %x)",
				i, clientPointers[i], firstPointer)
		}
	}
}

// TestClientCaching_ConcurrentAccessSafety tests that the cached genai.Client
// can be safely accessed from multiple goroutines concurrently.
//
// RED PHASE: This test WILL FAIL because:
// 1. There is no cached client to access
// 2. Each goroutine creates its own client via createGenaiClient()
// 3. Race conditions may occur without proper synchronization.
func TestClientCaching_ConcurrentAccessSafety(t *testing.T) {
	config := &gemini.ClientConfig{
		APIKey:     "test-concurrent-api-key",
		Model:      "gemini-embedding-001",
		TaskType:   "RETRIEVAL_DOCUMENT",
		Timeout:    30 * time.Second,
		Dimensions: 768,
	}

	client, err := gemini.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	// Run with -race flag to detect race conditions
	numGoroutines := 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Channel to collect client pointers from each goroutine
	pointerChan := make(chan uintptr, numGoroutines)

	for i := range numGoroutines {
		go func(goroutineID int) {
			defer wg.Done()

			// Make an embedding request (will fail with auth, but that's OK)
			_, _ = client.GenerateEmbedding(ctx, "concurrent test text", outbound.EmbeddingOptions{})

			// RED PHASE: Extract the genai.Client pointer
			genaiClient := extractGenaiClientField(t, client)
			if genaiClient != nil {
				pointerChan <- getPointerAddress(genaiClient)
			} else {
				pointerChan <- 0
			}
		}(i)
	}

	wg.Wait()
	close(pointerChan)

	// RED PHASE: Verify all goroutines used the SAME cached client instance
	clientPointers := make([]uintptr, 0, numGoroutines)
	for ptr := range pointerChan {
		clientPointers = append(clientPointers, ptr)
	}

	if len(clientPointers) == 0 {
		t.Fatal("no client pointers collected from goroutines")
	}

	firstPointer := clientPointers[0]
	if firstPointer == 0 {
		t.Fatal("first goroutine did not have a valid genai.Client pointer")
	}

	// All goroutines should have used the SAME cached instance
	for i, ptr := range clientPointers {
		if ptr != firstPointer {
			t.Errorf("goroutine %d used different genai.Client instance (pointer: %x) than goroutine 0 (pointer: %x)",
				i, ptr, firstPointer)
		}
	}
}

// TestClientCaching_InitializationFailureHandling tests that NewClient properly
// handles and propagates errors when genai.Client creation fails.
//
// RED PHASE: This test WILL FAIL because:
// 1. NewClient does not attempt to create a genai.Client during initialization
// 2. Client creation errors are not caught until the first API request
// 3. NewClient succeeds even with invalid API keys (validation is minimal).
func TestClientCaching_InitializationFailureHandling(t *testing.T) {
	tests := []struct {
		name          string
		config        *gemini.ClientConfig
		expectError   bool
		errorContains string
		errorType     string
	}{
		{
			name: "empty_api_key_fails_during_init",
			config: &gemini.ClientConfig{
				APIKey:   "",
				Model:    "gemini-embedding-001",
				TaskType: "RETRIEVAL_DOCUMENT",
			},
			expectError:   true,
			errorContains: "API key cannot be empty",
		},
		{
			name: "whitespace_api_key_fails_during_init",
			config: &gemini.ClientConfig{
				APIKey:   "   \t\n   ",
				Model:    "gemini-embedding-001",
				TaskType: "RETRIEVAL_DOCUMENT",
			},
			expectError:   true,
			errorContains: "API key cannot be empty or whitespace",
		},
		{
			name: "valid_api_key_succeeds",
			config: &gemini.ClientConfig{
				APIKey:   "valid-test-api-key",
				Model:    "gemini-embedding-001",
				TaskType: "RETRIEVAL_DOCUMENT",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := gemini.NewClient(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q during initialization, got nil", tt.errorContains)
					return
				}
				if tt.errorContains != "" && err.Error() != tt.errorContains {
					t.Errorf("expected error %q, got %q", tt.errorContains, err.Error())
				}
				if client != nil {
					t.Error("expected nil client when initialization fails")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error during initialization: %v", err)
				return
			}

			if client == nil {
				t.Error("expected non-nil client")
				return
			}

			// RED PHASE: Verify the cached genai.Client was created successfully
			genaiClient := extractGenaiClientField(t, client)
			if genaiClient == nil {
				t.Error("expected genai.Client to be created and cached during successful initialization")
			}
		})
	}
}

// TestClientCaching_ClientFieldAccessibility tests that the cached genai.Client
// field is properly accessible and non-nil after initialization.
//
// RED PHASE: This test WILL FAIL because:
// 1. The Client struct does not have a genaiClient field
// 2. Reflection will not find the field
// 3. The field cannot be accessed or verified.
func TestClientCaching_ClientFieldAccessibility(t *testing.T) {
	config := &gemini.ClientConfig{
		APIKey:     "field-access-test-key",
		Model:      "gemini-embedding-001",
		TaskType:   "RETRIEVAL_DOCUMENT",
		Timeout:    30 * time.Second,
		Dimensions: 768,
	}

	client, err := gemini.NewClient(config)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// RED PHASE: Use reflection to verify the genaiClient field exists and is populated
	clientValue := reflect.ValueOf(client)
	if clientValue.Kind() == reflect.Ptr {
		clientValue = clientValue.Elem()
	}

	// Look for the genaiClient field (may be named differently)
	genaiClientField := clientValue.FieldByName("genaiClient")
	if !genaiClientField.IsValid() {
		// Try alternative field names
		genaiClientField = clientValue.FieldByName("client")
		if !genaiClientField.IsValid() {
			genaiClientField = clientValue.FieldByName("genai")
		}
	}

	if !genaiClientField.IsValid() {
		t.Error("Client struct does not have a genaiClient field (or similar) to cache the genai.Client instance")
		return
	}

	// Field exists - check if it's populated (non-nil)
	if genaiClientField.IsNil() {
		t.Error("genaiClient field exists but is nil - should be populated during NewClient()")
	}
}

// TestClientCaching_PerformanceImprovement was removed during refactoring phase.
// Reason: This test was flawed as it measured API latency (~75ms per request) rather than
// caching overhead. The test expected <10ms per request, but each GenerateEmbedding call
// makes an actual API request to Gemini which takes 50-100ms regardless of client caching.
// The other 5 caching tests adequately verify that the genai.Client is properly cached:
// - TestClientCaching_GenaiClientCreatedDuringInitialization
// - TestClientCaching_GenaiClientReusedAcrossMultipleCalls
// - TestClientCaching_ConcurrentAccessSafety
// - TestClientCaching_InitializationFailureHandling
// - TestClientCaching_ClientFieldAccessibility

// Helper Functions

// extractGenaiClientField uses reflection to extract the genai.Client field from the Client struct.
// This is a test helper to verify the client is properly cached.
//
// RED PHASE: This will return nil because the field does not exist yet.
func extractGenaiClientField(t *testing.T, client *gemini.Client) interface{} {
	t.Helper()

	if client == nil {
		return nil
	}

	clientValue := reflect.ValueOf(client)
	if clientValue.Kind() == reflect.Ptr {
		clientValue = clientValue.Elem()
	}

	// Try multiple possible field names for the cached genai.Client
	fieldNames := []string{"genaiClient", "client", "genai", "apiClient", "genaiAPIClient"}

	for _, fieldName := range fieldNames {
		field := clientValue.FieldByName(fieldName)
		if field.IsValid() && !field.IsNil() {
			// Found the field - use unsafe to access it (it may be private)
			// This is acceptable in tests to verify internal state
			fieldPtr := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
			return fieldPtr.Interface()
		}
	}

	return nil
}

// getPointerAddress returns the memory address of an interface value.
// This is used to verify that the same instance is being reused.
func getPointerAddress(v interface{}) uintptr {
	if v == nil {
		return 0
	}

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		return val.Pointer()
	}

	// If not a pointer, we can't get a meaningful address
	return 0
}
