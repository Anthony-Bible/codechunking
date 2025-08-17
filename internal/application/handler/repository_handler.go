package handler

import (
	"codechunking/internal/application/common"
	"codechunking/internal/application/dto"
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Service interfaces.
type CreateRepositoryService interface {
	CreateRepository(ctx context.Context, request dto.CreateRepositoryRequest) (*dto.RepositoryResponse, error)
}

type GetRepositoryService interface {
	GetRepository(ctx context.Context, id uuid.UUID) (*dto.RepositoryResponse, error)
}

type UpdateRepositoryService interface {
	UpdateRepository(
		ctx context.Context,
		id uuid.UUID,
		request dto.UpdateRepositoryRequest,
	) (*dto.RepositoryResponse, error)
}

type DeleteRepositoryService interface {
	DeleteRepository(ctx context.Context, id uuid.UUID) error
}

type ListRepositoriesService interface {
	ListRepositories(ctx context.Context, query dto.RepositoryListQuery) (*dto.RepositoryListResponse, error)
}

// Command Handlers

// CreateRepositoryCommandHandler handles repository creation commands.
// It acts as an adapter between the HTTP/API layer and the application service layer,
// providing validation and DTO conversion.
type CreateRepositoryCommandHandler struct {
	service CreateRepositoryService // Service for repository creation operations
}

// NewCreateRepositoryCommandHandler creates a new instance of CreateRepositoryCommandHandler.
// The service parameter must be non-nil or the function will panic.
func NewCreateRepositoryCommandHandler(service CreateRepositoryService) *CreateRepositoryCommandHandler {
	if service == nil {
		panic("service cannot be nil")
	}
	return &CreateRepositoryCommandHandler{
		service: service,
	}
}

// Handle processes a create repository command.
// Validates the command and delegates to the service layer.
func (h *CreateRepositoryCommandHandler) Handle(
	ctx context.Context,
	command CreateRepositoryCommand,
) (*dto.RepositoryResponse, error) {
	// Validate command
	if err := h.validateCreateCommand(command); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Convert to DTO
	request := dto.CreateRepositoryRequest{
		URL:           command.URL,
		Name:          command.Name,
		Description:   command.Description,
		DefaultBranch: command.DefaultBranch,
	}

	// Execute service
	return h.service.CreateRepository(ctx, request)
}

// validateCreateCommand validates repository creation command.
func (h *CreateRepositoryCommandHandler) validateCreateCommand(command CreateRepositoryCommand) error {
	if err := common.ValidateRepositoryURL(command.URL); err != nil {
		return err
	}
	return nil
}

// UpdateRepositoryCommandHandler handles repository update commands.
// It provides validation and DTO conversion for repository metadata updates.
type UpdateRepositoryCommandHandler struct {
	service UpdateRepositoryService // Service for repository update operations
}

// NewUpdateRepositoryCommandHandler creates a new instance of UpdateRepositoryCommandHandler.
// The service parameter must be non-nil or the function will panic.
func NewUpdateRepositoryCommandHandler(service UpdateRepositoryService) *UpdateRepositoryCommandHandler {
	if service == nil {
		panic("service cannot be nil")
	}
	return &UpdateRepositoryCommandHandler{
		service: service,
	}
}

// Handle processes an update repository command.
// Validates the command and delegates to the service layer.
func (h *UpdateRepositoryCommandHandler) Handle(
	ctx context.Context,
	command UpdateRepositoryCommand,
) (*dto.RepositoryResponse, error) {
	// Validate command
	if err := h.validateUpdateCommand(command); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Convert to DTO
	request := dto.UpdateRepositoryRequest{
		Name:          command.Name,
		Description:   command.Description,
		DefaultBranch: command.DefaultBranch,
	}

	// Execute service
	return h.service.UpdateRepository(ctx, command.ID, request)
}

// validateUpdateCommand validates repository update command.
func (h *UpdateRepositoryCommandHandler) validateUpdateCommand(command UpdateRepositoryCommand) error {
	if err := common.ValidateUUID(command.ID, "repository ID"); err != nil {
		return err
	}
	if err := common.ValidateRepositoryName(command.Name); err != nil {
		return err
	}
	return nil
}

// DeleteRepositoryCommandHandler handles repository deletion commands.
// It provides validation for repository soft deletion operations.
type DeleteRepositoryCommandHandler struct {
	service DeleteRepositoryService // Service for repository deletion operations
}

// NewDeleteRepositoryCommandHandler creates a new instance of DeleteRepositoryCommandHandler.
// The service parameter must be non-nil or the function will panic.
func NewDeleteRepositoryCommandHandler(service DeleteRepositoryService) *DeleteRepositoryCommandHandler {
	if service == nil {
		panic("service cannot be nil")
	}
	return &DeleteRepositoryCommandHandler{
		service: service,
	}
}

// Handle processes a delete repository command.
// Validates the command and delegates to the service layer.
func (h *DeleteRepositoryCommandHandler) Handle(ctx context.Context, command DeleteRepositoryCommand) error {
	// Validate command
	if err := common.ValidateUUID(command.ID, "repository ID"); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Execute service
	return h.service.DeleteRepository(ctx, command.ID)
}

// Query Handlers

// GetRepositoryQueryHandler handles repository retrieval queries.
type GetRepositoryQueryHandler struct {
	service GetRepositoryService
}

// NewGetRepositoryQueryHandler creates a new instance of GetRepositoryQueryHandler.
func NewGetRepositoryQueryHandler(service GetRepositoryService) *GetRepositoryQueryHandler {
	return &GetRepositoryQueryHandler{
		service: service,
	}
}

// Handle processes a get repository query.
// Validates the query and delegates to the service layer.
func (h *GetRepositoryQueryHandler) Handle(
	ctx context.Context,
	query GetRepositoryQuery,
) (*dto.RepositoryResponse, error) {
	// Validate query
	if err := common.ValidateUUID(query.ID, "repository ID"); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Execute service
	return h.service.GetRepository(ctx, query.ID)
}

// ListRepositoriesQueryHandler handles repository listing queries.
type ListRepositoriesQueryHandler struct {
	service ListRepositoriesService
}

// NewListRepositoriesQueryHandler creates a new instance of ListRepositoriesQueryHandler.
func NewListRepositoriesQueryHandler(service ListRepositoriesService) *ListRepositoriesQueryHandler {
	return &ListRepositoriesQueryHandler{
		service: service,
	}
}

// Handle processes a list repositories query.
// Validates the query, applies defaults, and delegates to the service layer.
func (h *ListRepositoriesQueryHandler) Handle(
	ctx context.Context,
	query ListRepositoriesQuery,
) (*dto.RepositoryListResponse, error) {
	// Validate query
	if err := h.validateListQuery(query); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Convert to DTO
	dtoQuery := dto.RepositoryListQuery{
		Status: query.Status,
		Limit:  query.Limit,
		Offset: query.Offset,
		Sort:   query.Sort,
	}

	// Apply defaults (service will handle this but we can do it here for validation)
	common.ApplyRepositoryListDefaults(&dtoQuery)

	// Execute service
	return h.service.ListRepositories(ctx, dtoQuery)
}

// validateListQuery validates repository list query parameters.
func (h *ListRepositoriesQueryHandler) validateListQuery(query ListRepositoriesQuery) error {
	if err := common.ValidateRepositoryStatus(query.Status); err != nil {
		return err
	}
	if err := common.ValidatePaginationLimit(query.Limit, common.MaxRepositoryListLimit, "limit"); err != nil {
		return err
	}
	return nil
}
