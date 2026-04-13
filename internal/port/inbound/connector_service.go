// Package inbound defines the inbound port for connector operations.
package inbound

import (
	"codechunking/internal/application/dto"
	"context"

	"github.com/google/uuid"
)

// ConnectorService defines the inbound port for managing git connectors.
// The API is read-only; connectors are managed declaratively via config.
type ConnectorService interface {
	GetConnector(ctx context.Context, id uuid.UUID) (*dto.ConnectorResponse, error)
	ListConnectors(ctx context.Context, query dto.ConnectorListQuery) (*dto.ConnectorListResponse, error)
	SyncConnector(ctx context.Context, id uuid.UUID) (*dto.SyncConnectorResponse, error)
}
