package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"codechunking/internal/application/dto"
	"codechunking/internal/port/inbound"

	"github.com/google/uuid"
)

// RepositoryHandler handles HTTP requests for repository operations
type RepositoryHandler struct {
	repositoryService inbound.RepositoryService
	errorHandler      ErrorHandler
}

// NewRepositoryHandler creates a new RepositoryHandler
func NewRepositoryHandler(repositoryService inbound.RepositoryService, errorHandler ErrorHandler) *RepositoryHandler {
	return &RepositoryHandler{
		repositoryService: repositoryService,
		errorHandler:      errorHandler,
	}
}

// CreateRepository handles POST /repositories
func (h *RepositoryHandler) CreateRepository(w http.ResponseWriter, r *http.Request) {
	var request dto.CreateRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	// Validate request
	if err := h.validateCreateRepositoryRequest(request); err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	response, err := h.repositoryService.CreateRepository(r.Context(), request)
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	h.writeJSONResponse(w, http.StatusAccepted, response)
}

// GetRepository handles GET /repositories/{id}
func (h *RepositoryHandler) GetRepository(w http.ResponseWriter, r *http.Request) {
	params := ExtractPathParams(r)
	id, err := h.extractUUIDFromPath(params, "id")
	if err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	response, err := h.repositoryService.GetRepository(r.Context(), id)
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	h.writeJSONResponse(w, 0, response)
}

// ListRepositories handles GET /repositories
func (h *RepositoryHandler) ListRepositories(w http.ResponseWriter, r *http.Request) {
	query := h.parseRepositoryListQuery(r)

	if err := h.validateRepositoryListQuery(query); err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	response, err := h.repositoryService.ListRepositories(r.Context(), query)
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	h.writeJSONResponse(w, 0, response)
}

// DeleteRepository handles DELETE /repositories/{id}
func (h *RepositoryHandler) DeleteRepository(w http.ResponseWriter, r *http.Request) {
	params := ExtractPathParams(r)
	id, err := h.extractUUIDFromPath(params, "id")
	if err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	err = h.repositoryService.DeleteRepository(r.Context(), id)
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetRepositoryJobs handles GET /repositories/{id}/jobs
func (h *RepositoryHandler) GetRepositoryJobs(w http.ResponseWriter, r *http.Request) {
	params := ExtractPathParams(r)
	repositoryID, err := h.extractUUIDFromPath(params, "id")
	if err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	query := h.parseIndexingJobListQuery(r)

	if err := h.validateIndexingJobListQuery(query); err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	response, err := h.repositoryService.GetRepositoryJobs(r.Context(), repositoryID, query)
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	h.writeJSONResponse(w, 0, response)
}

// GetIndexingJob handles GET /repositories/{id}/jobs/{job_id}
func (h *RepositoryHandler) GetIndexingJob(w http.ResponseWriter, r *http.Request) {
	params := ExtractPathParams(r)

	repositoryID, err := h.extractUUIDFromPath(params, "id")
	if err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	jobID, err := h.extractUUIDFromPath(params, "job_id")
	if err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	response, err := h.repositoryService.GetIndexingJob(r.Context(), repositoryID, jobID)
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	h.writeJSONResponse(w, 0, response)
}

// parseRepositoryListQuery extracts query parameters for repository listing
func (h *RepositoryHandler) parseRepositoryListQuery(r *http.Request) dto.RepositoryListQuery {
	query := dto.DefaultRepositoryListQuery()

	if status := r.URL.Query().Get("status"); status != "" {
		query.Status = status
	}

	query.Limit = h.parseIntQueryParam(r, "limit", query.Limit)
	query.Offset = h.parseIntQueryParam(r, "offset", query.Offset)

	if sort := r.URL.Query().Get("sort"); sort != "" {
		query.Sort = sort
	}

	return query
}

// parseIndexingJobListQuery extracts query parameters for indexing job listing
func (h *RepositoryHandler) parseIndexingJobListQuery(r *http.Request) dto.IndexingJobListQuery {
	query := dto.DefaultIndexingJobListQuery()

	query.Limit = h.parseIntQueryParam(r, "limit", query.Limit)
	query.Offset = h.parseIntQueryParam(r, "offset", query.Offset)

	return query
}

// validateCreateRepositoryRequest validates the create repository request
func (h *RepositoryHandler) validateCreateRepositoryRequest(request dto.CreateRepositoryRequest) error {
	if request.URL == "" {
		return NewValidationError("url", "URL is required")
	}
	// Additional validation will be done by the domain layer
	return nil
}

// validateRepositoryListQuery validates the repository list query parameters
func (h *RepositoryHandler) validateRepositoryListQuery(query dto.RepositoryListQuery) error {
	return h.validatePaginationParams(query.Limit, query.Offset, 100)
}

// validateIndexingJobListQuery validates the indexing job list query parameters
func (h *RepositoryHandler) validateIndexingJobListQuery(query dto.IndexingJobListQuery) error {
	return h.validatePaginationParams(query.Limit, query.Offset, 50)
}

// extractUUIDFromPath extracts and validates a UUID from URL path variables
func (h *RepositoryHandler) extractUUIDFromPath(vars map[string]string, paramName string) (uuid.UUID, error) {
	idStr, ok := vars[paramName]
	if !ok {
		return uuid.Nil, NewValidationError(paramName, paramName+" ID is required")
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, NewValidationError(paramName, "invalid UUID format")
	}

	return id, nil
}

// writeJSONResponse writes a JSON response with the appropriate headers
func (h *RepositoryHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if statusCode != 0 {
		w.WriteHeader(statusCode)
	}
	json.NewEncoder(w).Encode(data)
}

// parseIntQueryParam parses an integer query parameter with default value
func (h *RepositoryHandler) parseIntQueryParam(r *http.Request, paramName string, defaultValue int) int {
	if paramStr := r.URL.Query().Get(paramName); paramStr != "" {
		if value, err := strconv.Atoi(paramStr); err == nil {
			return value
		}
	}
	return defaultValue
}

// validatePaginationParams validates common pagination parameters
func (h *RepositoryHandler) validatePaginationParams(limit, offset, maxLimit int) error {
	if limit < 1 || limit > maxLimit {
		return NewValidationError("limit", fmt.Sprintf("limit must be between 1 and %d", maxLimit))
	}
	if offset < 0 {
		return NewValidationError("offset", "offset must be non-negative")
	}
	return nil
}
