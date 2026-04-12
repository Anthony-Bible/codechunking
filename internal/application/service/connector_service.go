package service

import (
	"codechunking/internal/application/common"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/entity"
	domainerrors "codechunking/internal/domain/errors/domain"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"

	"github.com/google/uuid"
)

// =============================================================================
// CreateConnectorService
// =============================================================================

// CreateConnectorService handles connector creation.
type CreateConnectorService struct {
	connectorRepo outbound.ConnectorRepository
}

// NewCreateConnectorService constructs a CreateConnectorService.
func NewCreateConnectorService(connectorRepo outbound.ConnectorRepository) *CreateConnectorService {
	return &CreateConnectorService{connectorRepo: connectorRepo}
}

// CreateConnector validates, persists, and returns a new connector.
func (s *CreateConnectorService) CreateConnector(
	ctx context.Context,
	req dto.CreateConnectorRequest,
) (*dto.ConnectorResponse, error) {
	slogger.Info(ctx, "Creating connector", slogger.Fields{"name": req.Name})

	if err := req.Validate(); err != nil {
		return nil, err
	}

	exists, err := s.connectorRepo.Exists(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("checking connector existence: %w", err)
	}
	if exists {
		return nil, domainerrors.ErrConnectorAlreadyExists
	}

	connectorType, err := valueobject.NewConnectorType(req.ConnectorType)
	if err != nil {
		return nil, err
	}

	connector, err := entity.NewConnector(req.Name, connectorType, req.BaseURL, req.AuthToken)
	if err != nil {
		return nil, err
	}

	if err := s.connectorRepo.Save(ctx, connector); err != nil {
		return nil, fmt.Errorf("saving connector: %w", err)
	}

	resp := common.EntityToConnectorResponse(connector)
	return resp, nil
}

// =============================================================================
// GetConnectorService
// =============================================================================

// GetConnectorService handles fetching a single connector.
type GetConnectorService struct {
	connectorRepo outbound.ConnectorRepository
}

// NewGetConnectorService constructs a GetConnectorService.
func NewGetConnectorService(connectorRepo outbound.ConnectorRepository) *GetConnectorService {
	return &GetConnectorService{connectorRepo: connectorRepo}
}

// GetConnector returns a connector by ID.
func (s *GetConnectorService) GetConnector(ctx context.Context, id uuid.UUID) (*dto.ConnectorResponse, error) {
	slogger.Info(ctx, "Getting connector", slogger.Fields{"id": id})

	connector, err := s.connectorRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return common.EntityToConnectorResponse(connector), nil
}

// =============================================================================
// ListConnectorsService
// =============================================================================

// ListConnectorsService handles listing connectors with filters.
type ListConnectorsService struct {
	connectorRepo outbound.ConnectorRepository
}

// NewListConnectorsService constructs a ListConnectorsService.
func NewListConnectorsService(connectorRepo outbound.ConnectorRepository) *ListConnectorsService {
	return &ListConnectorsService{connectorRepo: connectorRepo}
}

// ListConnectors returns a paginated list of connectors.
func (s *ListConnectorsService) ListConnectors(
	ctx context.Context,
	query dto.ConnectorListQuery,
) (*dto.ConnectorListResponse, error) {
	slogger.Info(ctx, "Listing connectors", slogger.Fields{"limit": query.Limit, "offset": query.Offset})

	filters := outbound.ConnectorFilters{
		Limit:  query.Limit,
		Offset: query.Offset,
	}

	if query.ConnectorType != "" {
		ct, err := valueobject.NewConnectorType(query.ConnectorType)
		if err != nil {
			return nil, err
		}
		filters.ConnectorType = &ct
	}

	if query.Status != "" {
		cs, err := valueobject.NewConnectorStatus(query.Status)
		if err != nil {
			return nil, err
		}
		filters.Status = &cs
	}

	connectors, total, err := s.connectorRepo.FindAll(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("listing connectors: %w", err)
	}

	responses := make([]dto.ConnectorResponse, 0, len(connectors))
	for _, c := range connectors {
		r := common.EntityToConnectorResponse(c)
		responses = append(responses, *r)
	}

	return &dto.ConnectorListResponse{
		Connectors: responses,
		Pagination: dto.PaginationResponse{
			Limit:   query.Limit,
			Offset:  query.Offset,
			Total:   total,
			HasMore: query.Offset+len(responses) < total,
		},
	}, nil
}

// =============================================================================
// DeleteConnectorService
// =============================================================================

// DeleteConnectorService handles connector deletion.
type DeleteConnectorService struct {
	connectorRepo outbound.ConnectorRepository
}

// NewDeleteConnectorService constructs a DeleteConnectorService.
func NewDeleteConnectorService(connectorRepo outbound.ConnectorRepository) *DeleteConnectorService {
	return &DeleteConnectorService{connectorRepo: connectorRepo}
}

// DeleteConnector removes a connector by ID.
func (s *DeleteConnectorService) DeleteConnector(ctx context.Context, id uuid.UUID) error {
	slogger.Info(ctx, "Deleting connector", slogger.Fields{"id": id})

	connector, err := s.connectorRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if connector.Status() == valueobject.ConnectorStatusSyncing {
		return domainerrors.ErrConnectorSyncing
	}

	return s.connectorRepo.Delete(ctx, id)
}

// =============================================================================
// SyncConnectorService
// =============================================================================

// SyncConnectorService triggers a sync for a connector.
type SyncConnectorService struct {
	connectorRepo outbound.ConnectorRepository
}

// NewSyncConnectorService constructs a SyncConnectorService.
func NewSyncConnectorService(connectorRepo outbound.ConnectorRepository) *SyncConnectorService {
	return &SyncConnectorService{connectorRepo: connectorRepo}
}

// SyncConnector marks the connector as syncing and returns a response.
func (s *SyncConnectorService) SyncConnector(
	ctx context.Context,
	id uuid.UUID,
) (*dto.SyncConnectorResponse, error) {
	slogger.Info(ctx, "Triggering connector sync", slogger.Fields{"id": id})

	connector, err := s.connectorRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if connector.Status() == valueobject.ConnectorStatusSyncing {
		return nil, domainerrors.ErrConnectorSyncing
	}

	if err := connector.MarkSyncStarted(); err != nil {
		return nil, fmt.Errorf("marking sync started: %w", err)
	}

	if err := s.connectorRepo.Update(ctx, connector); err != nil {
		return nil, fmt.Errorf("updating connector: %w", err)
	}

	return &dto.SyncConnectorResponse{
		ConnectorID: id,
		Message:     "sync triggered successfully",
	}, nil
}
