// Package command contains command objects for connector operations.
package command

import "github.com/google/uuid"

// CreateConnectorCommand represents a command to create a new git connector.
type CreateConnectorCommand struct {
	Name          string `validate:"required,max=255"`
	ConnectorType string `validate:"required"`
	BaseURL       string `validate:"required,url"`
	AuthToken     string `validate:"omitempty"`
}

// UpdateConnectorCommand represents a command to update connector metadata.
type UpdateConnectorCommand struct {
	ID        uuid.UUID `validate:"required"`
	BaseURL   *string   `validate:"omitempty,url"`
	AuthToken *string   `validate:"omitempty"`
}

// DeleteConnectorCommand represents a command to delete a connector.
type DeleteConnectorCommand struct {
	ID uuid.UUID `validate:"required"`
}

// SyncConnectorCommand represents a command to trigger a sync for a connector.
type SyncConnectorCommand struct {
	ID uuid.UUID `validate:"required"`
}
