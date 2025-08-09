package handler

import (
	"context"
	"fmt"

	"codechunking/internal/application/common"
	"codechunking/internal/application/dto"

	"github.com/google/uuid"
)

// Service interfaces
type CreateIndexingJobService interface {
	CreateIndexingJob(ctx context.Context, request dto.CreateIndexingJobRequest) (*dto.IndexingJobResponse, error)
}

type GetIndexingJobService interface {
	GetIndexingJob(ctx context.Context, repositoryID, jobID uuid.UUID) (*dto.IndexingJobResponse, error)
}

type UpdateIndexingJobService interface {
	UpdateIndexingJob(ctx context.Context, jobID uuid.UUID, request dto.UpdateIndexingJobRequest) (*dto.IndexingJobResponse, error)
}

type ListIndexingJobsService interface {
	ListIndexingJobs(ctx context.Context, repositoryID uuid.UUID, query dto.IndexingJobListQuery) (*dto.IndexingJobListResponse, error)
}

// Command Handlers

// CreateIndexingJobCommandHandler handles indexing job creation commands
type CreateIndexingJobCommandHandler struct {
	service CreateIndexingJobService
}

// NewCreateIndexingJobCommandHandler creates a new instance of CreateIndexingJobCommandHandler
func NewCreateIndexingJobCommandHandler(service CreateIndexingJobService) *CreateIndexingJobCommandHandler {
	return &CreateIndexingJobCommandHandler{
		service: service,
	}
}

// Handle processes a create indexing job command.
// Validates the command and delegates to the service layer.
func (h *CreateIndexingJobCommandHandler) Handle(ctx context.Context, command CreateIndexingJobCommand) (*dto.IndexingJobResponse, error) {
	// Validate command
	if err := h.validateCreateCommand(command); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Convert to DTO
	request := dto.CreateIndexingJobRequest{
		RepositoryID: command.RepositoryID,
	}

	// Execute service
	return h.service.CreateIndexingJob(ctx, request)
}

// validateCreateCommand validates indexing job creation command
func (h *CreateIndexingJobCommandHandler) validateCreateCommand(command CreateIndexingJobCommand) error {
	if err := common.ValidateUUID(command.RepositoryID, "repository ID"); err != nil {
		return err
	}
	return nil
}

// UpdateIndexingJobCommandHandler handles indexing job update commands
type UpdateIndexingJobCommandHandler struct {
	service UpdateIndexingJobService
}

// NewUpdateIndexingJobCommandHandler creates a new instance of UpdateIndexingJobCommandHandler
func NewUpdateIndexingJobCommandHandler(service UpdateIndexingJobService) *UpdateIndexingJobCommandHandler {
	return &UpdateIndexingJobCommandHandler{
		service: service,
	}
}

// Handle processes an update indexing job command.
// Validates the command and delegates to the service layer.
func (h *UpdateIndexingJobCommandHandler) Handle(ctx context.Context, command UpdateIndexingJobCommand) (*dto.IndexingJobResponse, error) {
	// Validate command
	if err := h.validateUpdateCommand(command); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Convert to DTO
	request := dto.UpdateIndexingJobRequest{
		Status:         command.Status,
		FilesProcessed: command.FilesProcessed,
		ChunksCreated:  command.ChunksCreated,
		ErrorMessage:   command.ErrorMessage,
	}

	// Execute service
	return h.service.UpdateIndexingJob(ctx, command.JobID, request)
}

// validateUpdateCommand validates indexing job update command
func (h *UpdateIndexingJobCommandHandler) validateUpdateCommand(command UpdateIndexingJobCommand) error {
	if err := common.ValidateUUID(command.JobID, "job ID"); err != nil {
		return err
	}
	if err := common.ValidateJobStatus(command.Status); err != nil {
		return err
	}
	return nil
}

// Query Handlers

// GetIndexingJobQueryHandler handles indexing job retrieval queries
type GetIndexingJobQueryHandler struct {
	service GetIndexingJobService
}

// NewGetIndexingJobQueryHandler creates a new instance of GetIndexingJobQueryHandler
func NewGetIndexingJobQueryHandler(service GetIndexingJobService) *GetIndexingJobQueryHandler {
	return &GetIndexingJobQueryHandler{
		service: service,
	}
}

// Handle processes a get indexing job query.
// Validates the query and delegates to the service layer.
func (h *GetIndexingJobQueryHandler) Handle(ctx context.Context, query GetIndexingJobQuery) (*dto.IndexingJobResponse, error) {
	// Validate query
	if err := common.ValidateUUID(query.RepositoryID, "repository ID"); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	if err := common.ValidateUUID(query.JobID, "job ID"); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Execute service
	return h.service.GetIndexingJob(ctx, query.RepositoryID, query.JobID)
}

// ListIndexingJobsQueryHandler handles indexing job listing queries
type ListIndexingJobsQueryHandler struct {
	service ListIndexingJobsService
}

// NewListIndexingJobsQueryHandler creates a new instance of ListIndexingJobsQueryHandler
func NewListIndexingJobsQueryHandler(service ListIndexingJobsService) *ListIndexingJobsQueryHandler {
	return &ListIndexingJobsQueryHandler{
		service: service,
	}
}

// Handle processes a list indexing jobs query.
// Validates the query, applies defaults, and delegates to the service layer.
func (h *ListIndexingJobsQueryHandler) Handle(ctx context.Context, query ListIndexingJobsQuery) (*dto.IndexingJobListResponse, error) {
	// Validate query
	if err := h.validateListQuery(query); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Convert to DTO
	dtoQuery := dto.IndexingJobListQuery{
		Limit:  query.Limit,
		Offset: query.Offset,
	}

	// Apply defaults (service will handle this but we can do it here for validation)
	common.ApplyIndexingJobListDefaults(&dtoQuery)

	// Execute service
	return h.service.ListIndexingJobs(ctx, query.RepositoryID, dtoQuery)
}

// validateListQuery validates indexing job list query parameters
func (h *ListIndexingJobsQueryHandler) validateListQuery(query ListIndexingJobsQuery) error {
	if err := common.ValidateUUID(query.RepositoryID, "repository ID"); err != nil {
		return err
	}
	if err := common.ValidatePaginationLimit(query.Limit, common.MaxIndexingJobListLimit, "limit"); err != nil {
		return err
	}
	return nil
}
