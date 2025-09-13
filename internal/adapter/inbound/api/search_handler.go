package api

import (
	"codechunking/internal/application/common"
	"codechunking/internal/application/dto"
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// SearchService defines the interface for search operations.
type SearchService interface {
	Search(ctx context.Context, request dto.SearchRequestDTO) (*dto.SearchResponseDTO, error)
}

// SearchHandler handles HTTP requests for semantic search operations.
type SearchHandler struct {
	searchService SearchService
	errorHandler  ErrorHandler
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(searchService SearchService, errorHandler ErrorHandler) *SearchHandler {
	if searchService == nil {
		panic("searchService cannot be nil")
	}
	if errorHandler == nil {
		panic("errorHandler cannot be nil")
	}

	return &SearchHandler{
		searchService: searchService,
		errorHandler:  errorHandler,
	}
}

// Search handles POST /search requests for semantic code search.
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	// Validate HTTP method
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Validate Content-Type
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "application/json") {
		h.errorHandler.HandleValidationError(w, r,
			common.NewValidationError("content-type", "Content-Type must be application/json"))
		return
	}

	// Set CORS headers
	h.setCORSHeaders(w)

	// Decode and validate request body
	var searchRequest dto.SearchRequestDTO
	if err := h.decodeAndValidateJSON(r, &searchRequest); err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	// Apply defaults before validation
	searchRequest.ApplyDefaults()

	// Validate search request
	if err := searchRequest.Validate(); err != nil {
		h.errorHandler.HandleValidationError(w, r,
			common.NewValidationError("search_request", err.Error()))
		return
	}

	// Execute search
	searchResponse, err := h.searchService.Search(r.Context(), searchRequest)
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	// Return successful response
	h.writeJSONResponse(w, http.StatusOK, searchResponse)
}

// setCORSHeaders sets Cross-Origin Resource Sharing headers.
func (h *SearchHandler) setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	w.Header().Set("Access-Control-Max-Age", "3600")
}

// decodeAndValidateJSON decodes JSON from request body with proper error handling.
func (h *SearchHandler) decodeAndValidateJSON(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return common.NewValidationError("body", "Request body is required")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // Strict JSON parsing

	if err := decoder.Decode(v); err != nil {
		return common.NewValidationError("body", "Invalid JSON format: "+err.Error())
	}

	return nil
}

// writeJSONResponse writes a JSON response with proper headers.
func (h *SearchHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log error in production - for now just ignore
		// This should be extremely rare with well-formed data
		_ = err
	}
}
