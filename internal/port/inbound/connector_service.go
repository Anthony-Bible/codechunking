// Package inbound defines the inbound port for connector operations.
package inbound

import (
	"codechunking/internal/application/dto"
	"context"

	"github.com/google/uuid"
)

// ConnectorService defines the inbound port for managing git connectors.
type ConnectorService interface {
	CreateConnector(ctx context.Context, req dto.CreateConnectorRequest) (*dto.ConnectorResponse, error)
	GetConnector(ctx context.Context, id uuid.UUID) (*dto.ConnectorResponse, error)
	ListConnectors(ctx context.Context, query dto.ConnectorListQuery) (*dto.ConnectorListResponse, error)
	DeleteConnector(ctx context.Context, id uuid.UUID) error
	SyncConnector(ctx context.Context, id uuid.UUID) (*dto.SyncConnectorResponse, error)
}
