package service

import (
	"codechunking/internal/application/common"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/dto"
	domainerrors "codechunking/internal/domain/errors/domain"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"

	"github.com/google/uuid"
)

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

	// A freshly reconciled connector starts in pending state.
	// Promote it to active before transitioning to syncing.
	if connector.Status() == valueobject.ConnectorStatusPending {
		if err := connector.UpdateStatus(valueobject.ConnectorStatusActive); err != nil {
			return nil, fmt.Errorf("activating pending connector: %w", err)
		}
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
