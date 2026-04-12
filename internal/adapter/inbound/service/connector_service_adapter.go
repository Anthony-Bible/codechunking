// Package service contains inbound service adapters.
package service

import (
	"codechunking/internal/application/dto"
	appservice "codechunking/internal/application/service"
	"codechunking/internal/port/inbound"
	"codechunking/internal/port/outbound"
	"context"

	"github.com/google/uuid"
)

// ConnectorServiceAdapter adapts the application service layer to the inbound port.
// It implements the inbound.ConnectorService interface by delegating to application services.
type ConnectorServiceAdapter struct {
	createSvc *appservice.CreateConnectorService
	getSvc    *appservice.GetConnectorService
	listSvc   *appservice.ListConnectorsService
	deleteSvc *appservice.DeleteConnectorService
	syncSvc   *appservice.SyncConnectorService
}

// NewConnectorServiceAdapter creates a new ConnectorServiceAdapter.
func NewConnectorServiceAdapter(connectorRepo outbound.ConnectorRepository) inbound.ConnectorService {
	return &ConnectorServiceAdapter{
		createSvc: appservice.NewCreateConnectorService(connectorRepo),
		getSvc:    appservice.NewGetConnectorService(connectorRepo),
		listSvc:   appservice.NewListConnectorsService(connectorRepo),
		deleteSvc: appservice.NewDeleteConnectorService(connectorRepo),
		syncSvc:   appservice.NewSyncConnectorService(connectorRepo),
	}
}

// CreateConnector handles connector creation.
func (a *ConnectorServiceAdapter) CreateConnector(
	ctx context.Context,
	req dto.CreateConnectorRequest,
) (*dto.ConnectorResponse, error) {
	return a.createSvc.CreateConnector(ctx, req)
}

// GetConnector retrieves a connector by ID.
func (a *ConnectorServiceAdapter) GetConnector(ctx context.Context, id uuid.UUID) (*dto.ConnectorResponse, error) {
	return a.getSvc.GetConnector(ctx, id)
}

// ListConnectors returns a paginated list of connectors.
func (a *ConnectorServiceAdapter) ListConnectors(
	ctx context.Context,
	query dto.ConnectorListQuery,
) (*dto.ConnectorListResponse, error) {
	return a.listSvc.ListConnectors(ctx, query)
}

// DeleteConnector removes a connector by ID.
func (a *ConnectorServiceAdapter) DeleteConnector(ctx context.Context, id uuid.UUID) error {
	return a.deleteSvc.DeleteConnector(ctx, id)
}

// SyncConnector triggers a sync for a connector.
func (a *ConnectorServiceAdapter) SyncConnector(
	ctx context.Context,
	id uuid.UUID,
) (*dto.SyncConnectorResponse, error) {
	return a.syncSvc.SyncConnector(ctx, id)
}
