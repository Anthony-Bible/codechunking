// Package query contains query objects for connector read operations.
package query

import "github.com/google/uuid"

// GetConnectorQuery represents a query to retrieve a single connector.
type GetConnectorQuery struct {
	ID uuid.UUID `validate:"required"`
}

// ListConnectorsQuery represents a query to list connectors with optional filters.
type ListConnectorsQuery struct {
	ConnectorType string `validate:"omitempty"`
	Status        string `validate:"omitempty"`
	Limit         int    `validate:"omitempty,min=1,max=100"`
	Offset        int    `validate:"omitempty,min=0"`
	Sort          string `validate:"omitempty"`
}
