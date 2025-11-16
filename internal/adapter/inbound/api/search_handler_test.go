package api

import (
	"bytes"
	"codechunking/internal/application/dto"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockSearchService is a mock implementation of the search service for testing.
type MockSearchService struct {
	mock.Mock
}

func (m *MockSearchService) Search(ctx context.Context, request dto.SearchRequestDTO) (*dto.SearchResponseDTO, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*dto.SearchResponseDTO), args.Error(1)
}

// MockErrorHandler is a mock implementation of ErrorHandler for testing.
type MockErrorHandler struct {
	mock.Mock
}

func (m *MockErrorHandler) HandleValidationError(w http.ResponseWriter, r *http.Request, err error) {
	m.Called(w, r, err)
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func (m *MockErrorHandler) HandleServiceError(w http.ResponseWriter, r *http.Request, err error) {
	m.Called(w, r, err)
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
}

// TestSearchHandler exercises the happy path and error propagation for the handler.
func TestSearchHandler(t *testing.T) {
	t.Run("Valid_Search_Request_Success", func(t *testing.T) {
		// Setup mocks
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		// Create handler
		handler := NewSearchHandler(mockSearchService, mockErrorHandler)
		require.NotNil(t, handler, "SearchHandler should be created successfully")

		// Setup test request
		searchRequest := dto.SearchRequestDTO{
			Query:               "implement authentication middleware",
			Limit:               10,
			Offset:              0,
			SimilarityThreshold: 0.7,
			Sort:                "similarity:desc",
		}

		requestBody, err := json.Marshal(searchRequest)
		require.NoError(t, err, "Should marshal request without error")

		req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Mock successful search response
		chunkID := uuid.New()
		repoID := uuid.New()
		mockResponse := &dto.SearchResponseDTO{
			Results: []dto.SearchResultDTO{
				{
					ChunkID:         chunkID,
					Content:         "func authenticateUser() error { return nil }",
					SimilarityScore: 0.95,
					Repository: dto.RepositoryInfo{
						ID:   repoID,
						Name: "auth-service",
						URL:  "https://github.com/example/auth-service.git",
					},
					FilePath:  "/middleware/auth.go",
					Language:  "go",
					StartLine: 10,
					EndLine:   15,
				},
			},
			Pagination: dto.PaginationResponse{
				Limit:   10,
				Offset:  0,
				Total:   1,
				HasMore: false,
			},
			Metadata: dto.SearchMetadata{
				Query:           "implement authentication middleware",
				ExecutionTimeMs: 150,
			},
		}

		mockSearchService.On("Search", mock.Anything, searchRequest).
			Return(mockResponse, nil)

		// Execute request
		handler.Search(w, req)

		// Verify response
		assert.Equal(t, http.StatusOK, w.Code, "Should return HTTP 200 OK")

		var response dto.SearchResponseDTO
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err, "Should unmarshal response without error")

		assert.Len(t, response.Results, 1, "Should return one search result")
		assert.Equal(t, chunkID, response.Results[0].ChunkID, "ChunkID should match")
		assert.Equal(t, 0.95, response.Results[0].SimilarityScore, "Similarity score should match")
		assert.Equal(t, "go", response.Results[0].Language, "Language should match")
		assert.Equal(t, 10, response.Pagination.Limit, "Pagination limit should match")
		assert.Equal(t, "implement authentication middleware", response.Metadata.Query, "Query should match")

		mockSearchService.AssertExpectations(t)
	})

	t.Run("Search_Request_With_Filters", func(t *testing.T) {
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		handler := NewSearchHandler(mockSearchService, mockErrorHandler)

		// Setup request with filters
		repoID1 := uuid.New()
		repoID2 := uuid.New()
		searchRequest := dto.SearchRequestDTO{
			Query:               "database connection",
			Limit:               20,
			Offset:              10,
			RepositoryIDs:       []uuid.UUID{repoID1, repoID2},
			Languages:           []string{"go", "python"},
			FileTypes:           []string{".go", ".py"},
			SimilarityThreshold: 0.8,
			Sort:                "file_path:asc",
		}

		requestBody, err := json.Marshal(searchRequest)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Mock response
		mockResponse := &dto.SearchResponseDTO{
			Results: []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{
				Limit:   20,
				Offset:  10,
				Total:   0,
				HasMore: false,
			},
			Metadata: dto.SearchMetadata{
				Query:           "database connection",
				ExecutionTimeMs: 75,
			},
		}

		mockSearchService.On("Search", mock.Anything, searchRequest).
			Return(mockResponse, nil)

		// Execute request
		handler.Search(w, req)

		// Verify response
		assert.Equal(t, http.StatusOK, w.Code, "Should return HTTP 200 OK")
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json", "Should return JSON response")

		var response dto.SearchResponseDTO
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Empty(t, response.Results, "Should return empty results for filtered search")
		assert.Equal(t, 20, response.Pagination.Limit, "Pagination limit should match filtered request")
		assert.Equal(t, 10, response.Pagination.Offset, "Pagination offset should match filtered request")

		mockSearchService.AssertExpectations(t)
	})

	t.Run("Search_Request_With_Repository_Names_Filtering", func(t *testing.T) {
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		handler := NewSearchHandler(mockSearchService, mockErrorHandler)

		// Setup request with repository names
		searchRequest := dto.SearchRequestDTO{
			Query:               "authentication middleware",
			Limit:               10,
			Offset:              0,
			RepositoryNames:     []string{"golang/go", "facebook/react"},
			SimilarityThreshold: 0.7,               // Default applied by handler
			Sort:                "similarity:desc", // Default applied by handler
		}

		requestBody, err := json.Marshal(searchRequest)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Mock response
		mockResponse := &dto.SearchResponseDTO{
			Results: []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{
				Limit:   10,
				Offset:  0,
				Total:   0,
				HasMore: false,
			},
			Metadata: dto.SearchMetadata{
				Query:           "authentication middleware",
				ExecutionTimeMs: 120,
			},
		}

		mockSearchService.On("Search", mock.Anything, searchRequest).
			Return(mockResponse, nil)

		// Execute request
		handler.Search(w, req)

		// Verify response
		assert.Equal(t, http.StatusOK, w.Code, "Should return HTTP 200 OK")

		var response dto.SearchResponseDTO
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "authentication middleware", response.Metadata.Query, "Query should match")

		mockSearchService.AssertExpectations(t)
	})

	t.Run("Search_Request_With_Repository_Names_And_Repository_IDs", func(t *testing.T) {
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		handler := NewSearchHandler(mockSearchService, mockErrorHandler)

		// Setup request with both repository names and IDs
		repoID1 := uuid.New()
		repoID2 := uuid.New()
		searchRequest := dto.SearchRequestDTO{
			Query:               "mixed filtering",
			Limit:               15,
			Offset:              0,
			RepositoryIDs:       []uuid.UUID{repoID1, repoID2},
			RepositoryNames:     []string{"golang/go", "microsoft/vscode"},
			SimilarityThreshold: 0.7,               // Default applied by handler
			Sort:                "similarity:desc", // Default applied by handler
		}

		requestBody, err := json.Marshal(searchRequest)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Mock response
		mockResponse := &dto.SearchResponseDTO{
			Results: []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{
				Limit:   15,
				Offset:  0,
				Total:   0,
				HasMore: false,
			},
			Metadata: dto.SearchMetadata{
				Query:           "mixed filtering",
				ExecutionTimeMs: 95,
			},
		}

		mockSearchService.On("Search", mock.Anything, searchRequest).
			Return(mockResponse, nil)

		// Execute request
		handler.Search(w, req)

		// Verify response
		assert.Equal(t, http.StatusOK, w.Code, "Should return HTTP 200 OK")

		var response dto.SearchResponseDTO
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, 15, response.Pagination.Limit, "Pagination limit should match")

		mockSearchService.AssertExpectations(t)
	})

	t.Run("Invalid_Repository_Name_Format", func(t *testing.T) {
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		handler := NewSearchHandler(mockSearchService, mockErrorHandler)

		// Request with invalid repository name format
		searchRequest := dto.SearchRequestDTO{
			Query:           "test query",
			RepositoryNames: []string{"invalid-format-no-slash"},
		}

		requestBody, err := json.Marshal(searchRequest)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Mock error handler call for validation error
		mockErrorHandler.On("HandleValidationError", w, req, mock.AnythingOfType("common.ValidationError")).
			Return()

		// Execute request
		handler.Search(w, req)

		// Verify error handling
		assert.Equal(
			t,
			http.StatusBadRequest,
			w.Code,
			"Should return HTTP 400 Bad Request for invalid repository name format",
		)

		mockErrorHandler.AssertExpectations(t)
	})

	t.Run("Too_Many_Repository_Names", func(t *testing.T) {
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		handler := NewSearchHandler(mockSearchService, mockErrorHandler)

		// Request with too many repository names (exceeds limit of 50)
		tooManyNames := make([]string, 51)
		for i := range tooManyNames {
			tooManyNames[i] = fmt.Sprintf("org%d/repo%d", i, i)
		}

		searchRequest := dto.SearchRequestDTO{
			Query:           "test query",
			RepositoryNames: tooManyNames,
		}

		requestBody, err := json.Marshal(searchRequest)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Mock error handler call for validation error
		mockErrorHandler.On("HandleValidationError", w, req, mock.AnythingOfType("common.ValidationError")).
			Return()

		// Execute request
		handler.Search(w, req)

		// Verify error handling
		assert.Equal(
			t,
			http.StatusBadRequest,
			w.Code,
			"Should return HTTP 400 Bad Request for too many repository names",
		)

		mockErrorHandler.AssertExpectations(t)
	})

	t.Run("Invalid_JSON_Request_Body", func(t *testing.T) {
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		handler := NewSearchHandler(mockSearchService, mockErrorHandler)

		// Send invalid JSON
		invalidJSON := `{"query": "test", "limit": "invalid"}`
		req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(invalidJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Mock error handler call
		mockErrorHandler.On("HandleValidationError", w, req, mock.AnythingOfType("common.ValidationError")).
			Return()

		// Execute request
		handler.Search(w, req)

		// Verify error handling
		assert.Equal(t, http.StatusBadRequest, w.Code, "Should return HTTP 400 Bad Request")

		mockErrorHandler.AssertExpectations(t)
	})

	t.Run("Missing_Request_Body", func(t *testing.T) {
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		handler := NewSearchHandler(mockSearchService, mockErrorHandler)

		req := httptest.NewRequest(http.MethodPost, "/search", nil)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Mock error handler call for missing body
		mockErrorHandler.On("HandleValidationError", w, req, mock.AnythingOfType("common.ValidationError")).
			Return()

		// Execute request
		handler.Search(w, req)

		// Verify error handling
		assert.Equal(t, http.StatusBadRequest, w.Code, "Should return HTTP 400 Bad Request")

		mockErrorHandler.AssertExpectations(t)
	})

	t.Run("Empty_Query_Validation_Error", func(t *testing.T) {
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		handler := NewSearchHandler(mockSearchService, mockErrorHandler)

		// Request with empty query
		searchRequest := dto.SearchRequestDTO{
			Query: "",
		}

		requestBody, err := json.Marshal(searchRequest)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Mock error handler call for validation error
		mockErrorHandler.On("HandleValidationError", w, req, mock.AnythingOfType("common.ValidationError")).
			Return()

		// Execute request
		handler.Search(w, req)

		// Verify error handling
		assert.Equal(t, http.StatusBadRequest, w.Code, "Should return HTTP 400 Bad Request for empty query")

		mockErrorHandler.AssertExpectations(t)
	})

	t.Run("Invalid_Limit_Validation_Error", func(t *testing.T) {
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		handler := NewSearchHandler(mockSearchService, mockErrorHandler)

		// Request with invalid limit
		searchRequest := dto.SearchRequestDTO{
			Query: "test query",
			Limit: 101, // Exceeds maximum
		}

		requestBody, err := json.Marshal(searchRequest)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Mock error handler call for validation error
		mockErrorHandler.On("HandleValidationError", w, req, mock.AnythingOfType("common.ValidationError")).
			Return()

		// Execute request
		handler.Search(w, req)

		// Verify error handling
		assert.Equal(t, http.StatusBadRequest, w.Code, "Should return HTTP 400 Bad Request for invalid limit")

		mockErrorHandler.AssertExpectations(t)
	})

	t.Run("Search_Service_Error", func(t *testing.T) {
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		handler := NewSearchHandler(mockSearchService, mockErrorHandler)

		searchRequest := dto.SearchRequestDTO{
			Query: "test query",
		}

		requestBody, err := json.Marshal(searchRequest)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Create expected request with defaults applied (this is what the service will receive)
		expectedRequest := dto.SearchRequestDTO{
			Query:               "test query",
			Limit:               10, // Default applied
			Offset:              0,
			SimilarityThreshold: 0.7,               // Default applied
			Sort:                "similarity:desc", // Default applied
		}

		// Mock service error
		serviceError := errors.New("vector database connection failed")
		mockSearchService.On("Search", mock.Anything, expectedRequest).
			Return((*dto.SearchResponseDTO)(nil), serviceError)

		// Mock error handler call for service error
		mockErrorHandler.On("HandleServiceError", w, req, serviceError).
			Return()

		// Execute request
		handler.Search(w, req)

		// Verify error handling
		assert.Equal(t, http.StatusInternalServerError, w.Code, "Should return HTTP 500 Internal Server Error")

		mockSearchService.AssertExpectations(t)
		mockErrorHandler.AssertExpectations(t)
	})

	t.Run("Search_Request_Content_Type_Validation", func(t *testing.T) {
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		handler := NewSearchHandler(mockSearchService, mockErrorHandler)

		searchRequest := dto.SearchRequestDTO{
			Query: "test query",
		}

		requestBody, err := json.Marshal(searchRequest)
		require.NoError(t, err)

		// Create expected request with defaults applied (this is what the service will receive)
		expectedRequest := dto.SearchRequestDTO{
			Query:               "test query",
			Limit:               10, // Default applied
			Offset:              0,
			SimilarityThreshold: 0.7,               // Default applied
			Sort:                "similarity:desc", // Default applied
		}

		// Test different content types
		contentTypeTests := []struct {
			name        string
			contentType string
			expectError bool
		}{
			{
				name:        "Valid_JSON_Content_Type",
				contentType: "application/json",
				expectError: false,
			},
			{
				name:        "Valid_JSON_Content_Type_With_Charset",
				contentType: "application/json; charset=utf-8",
				expectError: false,
			},
			{
				name:        "Invalid_Content_Type",
				contentType: "text/plain",
				expectError: true,
			},
			{
				name:        "Missing_Content_Type",
				contentType: "",
				expectError: true,
			},
		}

		for _, tt := range contentTypeTests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader(requestBody))
				if tt.contentType != "" {
					req.Header.Set("Content-Type", tt.contentType)
				}
				w := httptest.NewRecorder()

				if tt.expectError {
					mockErrorHandler.On("HandleValidationError", w, req, mock.AnythingOfType("common.ValidationError")).
						Return()
				} else {
					// Mock successful response for valid content types
					mockResponse := &dto.SearchResponseDTO{
						Results:    []dto.SearchResultDTO{},
						Pagination: dto.PaginationResponse{},
						Metadata:   dto.SearchMetadata{Query: "test query"},
					}
					mockSearchService.On("Search", mock.Anything, expectedRequest).
						Return(mockResponse, nil)
				}

				// Execute request
				handler.Search(w, req)

				if tt.expectError {
					assert.Equal(t, http.StatusBadRequest, w.Code, "Should return HTTP 400 for invalid content type")
				} else {
					assert.Equal(t, http.StatusOK, w.Code, "Should return HTTP 200 for valid content type")
				}
			})
		}
	})

	t.Run("Search_Response_Headers", func(t *testing.T) {
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		handler := NewSearchHandler(mockSearchService, mockErrorHandler)

		searchRequest := dto.SearchRequestDTO{
			Query: "test query",
		}

		requestBody, err := json.Marshal(searchRequest)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Create expected request with defaults applied (this is what the service will receive)
		expectedRequest := dto.SearchRequestDTO{
			Query:               "test query",
			Limit:               10, // Default applied
			Offset:              0,
			SimilarityThreshold: 0.7,               // Default applied
			Sort:                "similarity:desc", // Default applied
		}

		// Mock successful response
		mockResponse := &dto.SearchResponseDTO{
			Results:    []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{},
			Metadata:   dto.SearchMetadata{Query: "test query", ExecutionTimeMs: 100},
		}

		mockSearchService.On("Search", mock.Anything, expectedRequest).
			Return(mockResponse, nil)

		// Execute request
		handler.Search(w, req)

		// Verify response headers
		assert.Equal(t, http.StatusOK, w.Code, "Should return HTTP 200 OK")
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json", "Should set JSON content type")

		// Verify CORS headers if implemented
		// These tests define expected CORS behavior
		assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Origin"), "Should set CORS origin header")
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST", "Should allow POST method")
		assert.Contains(
			t,
			w.Header().Get("Access-Control-Allow-Headers"),
			"Content-Type",
			"Should allow Content-Type header",
		)

		mockSearchService.AssertExpectations(t)
	})

	t.Run("Large_Search_Request_Handling", func(t *testing.T) {
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		handler := NewSearchHandler(mockSearchService, mockErrorHandler)

		// Create large query
		largeQuery := strings.Repeat("search term ", 1000) // Very long query

		searchRequest := dto.SearchRequestDTO{
			Query:         largeQuery,
			RepositoryIDs: make([]uuid.UUID, 50), // Large number of repository IDs
			Languages:     []string{"go", "python", "javascript", "java", "c++", "rust", "typescript"},
			FileTypes:     []string{".go", ".py", ".js", ".java", ".cpp", ".rs", ".ts", ".jsx", ".tsx"},
		}

		// Fill repository IDs
		for i := range searchRequest.RepositoryIDs {
			searchRequest.RepositoryIDs[i] = uuid.New()
		}

		requestBody, err := json.Marshal(searchRequest)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Create expected request with defaults applied (this is what the service will receive)
		expectedRequest := dto.SearchRequestDTO{
			Query:               largeQuery,
			Limit:               10, // Default applied
			Offset:              0,
			RepositoryIDs:       searchRequest.RepositoryIDs, // Copy the generated UUIDs
			Languages:           []string{"go", "python", "javascript", "java", "c++", "rust", "typescript"},
			FileTypes:           []string{".go", ".py", ".js", ".java", ".cpp", ".rs", ".ts", ".jsx", ".tsx"},
			SimilarityThreshold: 0.7,               // Default applied
			Sort:                "similarity:desc", // Default applied
		}

		// The handler should either process the large request or reject it appropriately
		mockResponse := &dto.SearchResponseDTO{
			Results:    []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{},
			Metadata:   dto.SearchMetadata{Query: largeQuery, ExecutionTimeMs: 500},
		}

		// Mock either successful processing or validation error
		mockSearchService.On("Search", mock.Anything, expectedRequest).
			Return(mockResponse, nil)

		// Execute request
		handler.Search(w, req)

		// Should either succeed or fail gracefully with appropriate status code
		assert.True(
			t,
			w.Code == http.StatusOK || w.Code == http.StatusBadRequest || w.Code == http.StatusRequestEntityTooLarge,
			"Should return appropriate status code for large request",
		)

		if w.Code == http.StatusOK {
			mockSearchService.AssertExpectations(t)
		}
	})

	t.Run("Concurrent_Search_Requests", func(t *testing.T) {
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		handler := NewSearchHandler(mockSearchService, mockErrorHandler)

		// This test verifies that the handler can handle concurrent requests safely
		numRequests := 10
		responses := make(chan *httptest.ResponseRecorder, numRequests)
		errors := make(chan error, numRequests)

		// Mock successful responses for all concurrent requests
		for i := range numRequests {
			mockResponse := &dto.SearchResponseDTO{
				Results:    []dto.SearchResultDTO{},
				Pagination: dto.PaginationResponse{},
				Metadata:   dto.SearchMetadata{Query: fmt.Sprintf("query %d", i), ExecutionTimeMs: 100},
			}
			mockSearchService.On("Search", mock.Anything, mock.AnythingOfType("dto.SearchRequestDTO")).
				Return(mockResponse, nil)
		}

		// Execute concurrent requests
		for i := range numRequests {
			go func(requestID int) {
				searchRequest := dto.SearchRequestDTO{
					Query: fmt.Sprintf("query %d", requestID),
				}

				requestBody, err := json.Marshal(searchRequest)
				if err != nil {
					errors <- err
					return
				}

				req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader(requestBody))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()

				handler.Search(w, req)

				responses <- w
			}(i)
		}

		// Collect all responses
		successCount := 0
		for range numRequests {
			select {
			case w := <-responses:
				assert.Equal(t, http.StatusOK, w.Code, "Concurrent request should succeed")
				successCount++
			case err := <-errors:
				t.Errorf("Concurrent request failed: %v", err)
			}
		}

		assert.Equal(t, numRequests, successCount, "All concurrent requests should succeed")
		mockSearchService.AssertExpectations(t)
	})
}

// TestNewSearchHandler validates constructor behavior.
func TestNewSearchHandler(t *testing.T) {
	t.Run("Valid_Handler_Creation", func(t *testing.T) {
		mockSearchService := new(MockSearchService)
		mockErrorHandler := new(MockErrorHandler)

		handler := NewSearchHandler(mockSearchService, mockErrorHandler)

		assert.NotNil(t, handler, "SearchHandler should be created successfully")
	})

	t.Run("Nil_Search_Service_Panic", func(t *testing.T) {
		mockErrorHandler := new(MockErrorHandler)

		assert.Panics(t, func() {
			NewSearchHandler(nil, mockErrorHandler)
		}, "Should panic when search service is nil")
	})

	t.Run("Nil_Error_Handler_Panic", func(t *testing.T) {
		mockSearchService := new(MockSearchService)

		assert.Panics(t, func() {
			NewSearchHandler(mockSearchService, nil)
		}, "Should panic when error handler is nil")
	})
}

// TestSearchHandler_HTTPMethods verifies method restrictions.
func TestSearchHandler_HTTPMethods(t *testing.T) {
	mockSearchService := new(MockSearchService)
	mockErrorHandler := new(MockErrorHandler)

	handler := NewSearchHandler(mockSearchService, mockErrorHandler)

	// Test unsupported HTTP methods
	unsupportedMethods := []string{
		http.MethodGet,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodOptions,
		http.MethodHead,
	}

	for _, method := range unsupportedMethods {
		t.Run(fmt.Sprintf("Unsupported_Method_%s", method), func(t *testing.T) {
			req := httptest.NewRequest(method, "/search", nil)
			w := httptest.NewRecorder()

			// The handler should reject unsupported methods
			handler.Search(w, req)

			assert.Equal(t, http.StatusMethodNotAllowed, w.Code,
				"Should return HTTP 405 for %s method", method)
		})
	}

	t.Run("Supported_POST_Method", func(t *testing.T) {
		searchRequest := dto.SearchRequestDTO{
			Query: "test query",
		}

		requestBody, err := json.Marshal(searchRequest)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Create expected request with defaults applied (this is what the service will receive)
		expectedRequest := dto.SearchRequestDTO{
			Query:               "test query",
			Limit:               10, // Default applied
			Offset:              0,
			SimilarityThreshold: 0.7,               // Default applied
			Sort:                "similarity:desc", // Default applied
		}

		// Mock successful response
		mockResponse := &dto.SearchResponseDTO{
			Results:    []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{},
			Metadata:   dto.SearchMetadata{Query: "test query"},
		}

		mockSearchService.On("Search", mock.Anything, expectedRequest).
			Return(mockResponse, nil)

		handler.Search(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Should support POST method")
		mockSearchService.AssertExpectations(t)
	})
}
