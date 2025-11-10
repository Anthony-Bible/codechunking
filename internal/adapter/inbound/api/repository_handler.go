package api

import (
	"codechunking/internal/application/common"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/dto"
	"codechunking/internal/port/inbound"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

const (
	// URL validation constants.
	maxRepositoryURLLength = 2048

	// Pagination limits.
	maxRepositoryListLimit  = 100
	maxIndexingJobListLimit = 50
)

// RepositoryHandler handles HTTP requests for repository operations.
type RepositoryHandler struct {
	repositoryService inbound.RepositoryService
	errorHandler      ErrorHandler
}

// NewRepositoryHandler creates a new RepositoryHandler.
func NewRepositoryHandler(repositoryService inbound.RepositoryService, errorHandler ErrorHandler) *RepositoryHandler {
	return &RepositoryHandler{
		repositoryService: repositoryService,
		errorHandler:      errorHandler,
	}
}

// CreateRepository handles POST /repositories with comprehensive request processing.
//
//	@Summary		Submit repository for indexing
//	@Description	Submits a Git repository URL for asynchronous indexing and processing. The repository will be cloned, analyzed, and chunked into semantic units.
//	@Tags			Repositories
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.CreateRepositoryRequest	true	"Repository creation request"
//	@Success		202		{object}	dto.RepositoryResponse		"Repository accepted for indexing"
//	@Failure		400		{object}	dto.ErrorResponse			"Invalid request"
//	@Failure		409		{object}	dto.ErrorResponse			"Repository already exists"
//	@Failure		500		{object}	dto.ErrorResponse			"Internal server error"
//	@Router			/repositories [post]
func (h *RepositoryHandler) CreateRepository(w http.ResponseWriter, r *http.Request) {
	var request dto.CreateRepositoryRequest
	if err := h.decodeAndValidateJSON(r, &request); err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

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

// GetRepository handles GET /repositories/{id}.
// GetRepository handles GET /repositories/{id}.
//
//	@Summary		Get repository details
//	@Description	Returns detailed information about a specific repository including its indexing status, statistics, and metadata.
//	@Tags			Repositories
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Repository UUID"
//	@Success		200		{object}	dto.RepositoryResponse	"Repository details"
//	@Failure		404		{object}	dto.ErrorResponse		"Repository not found"
//	@Router			/repositories/{id} [get]
func (h *RepositoryHandler) GetRepository(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractRepositoryIDFromPath(r)
	if err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	h.handleServiceCall(w, r, func() (interface{}, error) {
		return h.repositoryService.GetRepository(r.Context(), id)
	}, http.StatusOK)
}

// ListRepositories handles GET /repositories.
// ListRepositories handles GET /repositories.
//
//	@Summary		List repositories
//	@Description	Returns a paginated list of repositories that have been submitted for indexing. Results can be filtered by status and sorted by various fields.
//	@Tags			Repositories
//	@Accept			json
//	@Produce		json
//	@Param			status	query		string						false	"Filter repositories by status"
//	@Param			limit	query		int							false	"Maximum number of repositories to return (1-100)"			default(20)
//	@Param			offset	query		int							false	"Number of repositories to skip for pagination"			default(0)
//	@Param			sort		query		string						false	"Sort field and direction"								enum(created_at:asc,created_at:desc,updated_at:asc,updated_at:desc,name:asc,name:desc)	default(created_at:desc)
//	@Success		200		{object}	dto.RepositoryListResponse	"List of repositories"
//	@Router			/repositories [get]
func (h *RepositoryHandler) ListRepositories(w http.ResponseWriter, r *http.Request) {
	slogger.Info(r.Context(), "ListRepositories endpoint called", slogger.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	})

	query := h.parseRepositoryListQuery(r)
	slogger.Info(r.Context(), "Repository list query parsed", slogger.Fields{
		"status": query.Status,
		"limit":  query.Limit,
		"offset": query.Offset,
		"sort":   query.Sort,
		"name":   query.Name,
		"url":    query.URL,
	})

	if err := h.validateRepositoryListQuery(query); err != nil {
		slogger.Error(r.Context(), "Repository list query validation failed", slogger.Fields{
			"error": err.Error(),
			"query": query,
		})
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	slogger.Info(r.Context(), "Calling repository service to list repositories", slogger.Fields{
		"limit":  query.Limit,
		"offset": query.Offset,
	})

	response, err := h.repositoryService.ListRepositories(r.Context(), query)
	if err != nil {
		slogger.Error(r.Context(), "Repository service returned error", slogger.Fields{
			"error": err.Error(),
			"query": query,
		})
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	slogger.Info(r.Context(), "Repository list retrieved successfully", slogger.Fields{
		"repository_count": len(response.Repositories),
		"total":            response.Pagination.Total,
		"limit":            response.Pagination.Limit,
		"offset":           response.Pagination.Offset,
	})

	h.writeJSONResponse(w, http.StatusOK, response)
}

// DeleteRepository handles DELETE /repositories/{id}.
// DeleteRepository handles DELETE /repositories/{id}.
//
//	@Summary		Delete repository
//	@Description	Removes a repository and all its associated data including code chunks, embeddings, and indexing jobs. This operation is irreversible.
//	@Tags			Repositories
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string			true	"Repository UUID"
//	@Success		204		"Repository successfully deleted"
//	@Failure		404		{object}	dto.ErrorResponse	"Repository not found"
//	@Failure		409		{object}	dto.ErrorResponse	"Repository is currently being processed"
//	@Router			/repositories/{id} [delete]
func (h *RepositoryHandler) DeleteRepository(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractRepositoryIDFromPath(r)
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

// GetRepositoryJobs handles GET /repositories/{id}/jobs.
// GetRepositoryJobs handles GET /repositories/{id}/jobs.
//
//	@Summary		Get repository indexing jobs
//	@Description	Returns a list of indexing jobs for a specific repository, including their status, progress, and error details.
//	@Tags			Repositories
//	@Tags			Jobs
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"Repository UUID"
//	@Param			limit	query		int							false	"Maximum number of jobs to return (1-50)"		default(10)
//	@Param			offset	query		int							false	"Number of jobs to skip for pagination"		default(0)
//	@Success		200		{object}	dto.IndexingJobListResponse	"List of indexing jobs"
//	@Failure		404		{object}	dto.ErrorResponse			"Repository not found"
//	@Router			/repositories/{id}/jobs [get]
func (h *RepositoryHandler) GetRepositoryJobs(w http.ResponseWriter, r *http.Request) {
	repositoryID, err := h.extractRepositoryIDFromPath(r)
	if err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	query := h.parseIndexingJobListQuery(r)
	if validateErr := h.validateIndexingJobListQuery(query); validateErr != nil {
		h.errorHandler.HandleValidationError(w, r, validateErr)
		return
	}

	h.handleServiceCall(w, r, func() (interface{}, error) {
		return h.repositoryService.GetRepositoryJobs(r.Context(), repositoryID, query)
	}, http.StatusOK)
}

// GetIndexingJob handles GET /repositories/{id}/jobs/{job_id}.
// GetIndexingJob handles GET /repositories/{id}/jobs/{job_id}.
//
//	@Summary		Get specific indexing job details
//	@Description	Returns detailed information about a specific indexing job including progress, error messages, and processing statistics.
//	@Tags			Jobs
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Repository UUID"
//	@Param			job_id	path		string				true	"Job UUID"
//	@Success		200		{object}	dto.IndexingJobResponse	"Indexing job details"
//	@Failure		404		{object}	dto.ErrorResponse		"Repository or job not found"
//	@Router			/repositories/{id}/jobs/{job_id} [get]
func (h *RepositoryHandler) GetIndexingJob(w http.ResponseWriter, r *http.Request) {
	repositoryID, err := h.extractRepositoryIDFromPath(r)
	if err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	jobID, err := h.extractJobIDFromPath(r)
	if err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	h.handleServiceCall(w, r, func() (interface{}, error) {
		return h.repositoryService.GetIndexingJob(r.Context(), repositoryID, jobID)
	}, http.StatusOK)
}

// parseRepositoryListQuery extracts query parameters for repository listing.
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

// parseIndexingJobListQuery extracts query parameters for indexing job listing.
func (h *RepositoryHandler) parseIndexingJobListQuery(r *http.Request) dto.IndexingJobListQuery {
	query := dto.DefaultIndexingJobListQuery()

	query.Limit = h.parseIntQueryParam(r, "limit", query.Limit)
	query.Offset = h.parseIntQueryParam(r, "offset", query.Offset)

	return query
}

// validateCreateRepositoryRequest validates the create repository request with comprehensive checks.
func (h *RepositoryHandler) validateCreateRepositoryRequest(request dto.CreateRepositoryRequest) error {
	if request.URL == "" {
		return common.NewValidationError("url", "Repository URL is required and cannot be empty")
	}

	// Basic URL format validation (additional validation will be done by the domain layer)
	if len(request.URL) > maxRepositoryURLLength {
		return common.NewValidationError("url", "Repository URL is too long (maximum 2048 characters)")
	}

	return nil
}

// validateRepositoryListQuery validates the repository list query parameters.
func (h *RepositoryHandler) validateRepositoryListQuery(query dto.RepositoryListQuery) error {
	return h.validatePaginationParams(query.Limit, query.Offset, maxRepositoryListLimit)
}

// validateIndexingJobListQuery validates the indexing job list query parameters.
func (h *RepositoryHandler) validateIndexingJobListQuery(query dto.IndexingJobListQuery) error {
	return h.validatePaginationParams(query.Limit, query.Offset, maxIndexingJobListLimit)
}

// jsonResponseWriter encapsulates JSON response writing with improved error handling
// writeJSONResponse writes a JSON response using the pooled encoder.
func (h *RepositoryHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	if err := WriteJSON(w, statusCode, data); err != nil {
		// Log error in production (placeholder for now)
		// This should be extremely rare with well-formed data
		_ = err // TODO: Add proper logging
	}
}

// parseIntQueryParam parses an integer query parameter with default value.
func (h *RepositoryHandler) parseIntQueryParam(r *http.Request, paramName string, defaultValue int) int {
	if paramStr := r.URL.Query().Get(paramName); paramStr != "" {
		if value, err := strconv.Atoi(paramStr); err == nil {
			return value
		}
	}
	return defaultValue
}

// paginationValidator provides reusable pagination validation logic.
type paginationValidator struct {
	maxLimit int
}

// newPaginationValidator creates a new pagination validator with specified max limit.
func newPaginationValidator(maxLimit int) *paginationValidator {
	return &paginationValidator{maxLimit: maxLimit}
}

// validate validates pagination parameters with descriptive error messages.
func (pv *paginationValidator) validate(limit, offset int) error {
	if err := pv.validateLimit(limit); err != nil {
		return err
	}
	return pv.validateOffset(offset)
}

// validateLimit validates the limit parameter.
func (pv *paginationValidator) validateLimit(limit int) error {
	if limit < 1 {
		return common.NewValidationError("limit", "limit must be at least 1")
	}
	if limit > pv.maxLimit {
		return common.NewValidationError("limit", fmt.Sprintf("limit cannot exceed %d", pv.maxLimit))
	}
	return nil
}

// validateOffset validates the offset parameter.
func (pv *paginationValidator) validateOffset(offset int) error {
	if offset < 0 {
		return common.NewValidationError("offset", "offset must be non-negative (0 or greater)")
	}
	return nil
}

// validatePaginationParams validates common pagination parameters using the improved validator.
func (h *RepositoryHandler) validatePaginationParams(limit, offset, maxLimit int) error {
	validator := newPaginationValidator(maxLimit)
	return validator.validate(limit, offset)
}

// Helper methods for common operations

// decodeAndValidateJSON decodes JSON from request body with proper error handling.
func (h *RepositoryHandler) decodeAndValidateJSON(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return common.NewValidationError("body", "Request body is required")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // Strict JSON parsing

	if err := decoder.Decode(v); err != nil {
		return common.NewValidationError("body", fmt.Sprintf("Invalid JSON format: %v", err))
	}

	return nil
}

// pathParameterExtractor provides clean, production-ready path parameter extraction.
type pathParameterExtractor struct {
	request *http.Request
}

// newPathParameterExtractor creates a new path parameter extractor.
func newPathParameterExtractor(r *http.Request) *pathParameterExtractor {
	return &pathParameterExtractor{request: r}
}

// extractRepositoryID extracts and validates a repository ID from the path.
func (p *pathParameterExtractor) extractRepositoryID() (uuid.UUID, error) {
	return p.extractUUIDPathValue("id", "repository")
}

// extractJobID extracts and validates a job ID from the path.
func (p *pathParameterExtractor) extractJobID() (uuid.UUID, error) {
	return p.extractUUIDPathValue("job_id", "job")
}

// extractUUIDPathValue extracts a UUID path parameter with descriptive error handling.
func (p *pathParameterExtractor) extractUUIDPathValue(paramName, resourceType string) (uuid.UUID, error) {
	paramValue := p.request.PathValue(paramName)
	if paramValue == "" {
		return uuid.Nil, common.NewValidationError(
			paramName,
			fmt.Sprintf("%s ID is required in URL path", resourceType),
		)
	}

	id, err := uuid.Parse(paramValue)
	if err != nil {
		return uuid.Nil, common.NewValidationError(
			paramName,
			fmt.Sprintf("invalid %s UUID format: %s", resourceType, paramValue),
		)
	}

	return id, nil
}

// extractRepositoryIDFromPath is a specialized method for extracting repository IDs.
func (h *RepositoryHandler) extractRepositoryIDFromPath(r *http.Request) (uuid.UUID, error) {
	extractor := newPathParameterExtractor(r)
	return extractor.extractRepositoryID()
}

// extractJobIDFromPath is a specialized method for extracting job IDs.
func (h *RepositoryHandler) extractJobIDFromPath(r *http.Request) (uuid.UUID, error) {
	extractor := newPathParameterExtractor(r)
	return extractor.extractJobID()
}

// handleServiceCall wraps service calls with consistent error handling.
func (h *RepositoryHandler) handleServiceCall(
	w http.ResponseWriter,
	r *http.Request,
	serviceCall func() (interface{}, error),
	successStatus int,
) {
	response, err := serviceCall()
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	h.writeJSONResponse(w, successStatus, response)
}
