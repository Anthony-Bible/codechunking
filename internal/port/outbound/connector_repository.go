// Package outbound defines the outbound ports for connector persistence.
package outbound

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"context"

	"github.com/google/uuid"
)

// ConnectorRepository defines the outbound port for connector persistence.
type ConnectorRepository interface {
	Save(ctx context.Context, connector *entity.Connector) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Connector, error)
	FindByName(ctx context.Context, name string) (*entity.Connector, error)
	FindAll(ctx context.Context, filters ConnectorFilters) ([]*entity.Connector, int, error)
	Update(ctx context.Context, connector *entity.Connector) error
	Delete(ctx context.Context, id uuid.UUID) error
	Exists(ctx context.Context, name string) (bool, error)
}

// ConnectorFilters holds optional filters for querying connectors.
type ConnectorFilters struct {
	ConnectorType *valueobject.ConnectorType
	Status        *valueobject.ConnectorStatus
	Limit         int
	Offset        int
	Sort          string
}
