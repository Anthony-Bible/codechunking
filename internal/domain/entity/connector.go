package entity

import (
	"codechunking/internal/domain/valueobject"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Connector represents a git provider integration (e.g. a GitHub org or GitLab group).
type Connector struct {
	id              uuid.UUID
	name            string
	connectorType   valueobject.ConnectorType
	baseURL         string
	authToken       *string
	status          valueobject.ConnectorStatus
	repositoryCount int
	lastSyncAt      *time.Time
	createdAt       time.Time
	updatedAt       time.Time
}

// NewConnector creates a new Connector with the given parameters.
func NewConnector(
	name string,
	connectorType valueobject.ConnectorType,
	baseURL string,
	authToken *string,
) (*Connector, error) {
	if name == "" {
		return nil, errors.New("connector name cannot be empty")
	}
	if baseURL == "" {
		return nil, errors.New("connector base URL cannot be empty")
	}

	now := time.Now()
	return &Connector{
		id:            uuid.New(),
		name:          name,
		connectorType: connectorType,
		baseURL:       baseURL,
		authToken:     authToken,
		status:        valueobject.ConnectorStatusPending,
		createdAt:     now,
		updatedAt:     now,
	}, nil
}

// RestoreConnector recreates a Connector from persisted data (bypasses validation).
func RestoreConnector(
	id uuid.UUID,
	name string,
	connectorType valueobject.ConnectorType,
	baseURL string,
	authToken *string,
	status valueobject.ConnectorStatus,
	repositoryCount int,
	lastSyncAt *time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) *Connector {
	return &Connector{
		id:              id,
		name:            name,
		connectorType:   connectorType,
		baseURL:         baseURL,
		authToken:       authToken,
		status:          status,
		repositoryCount: repositoryCount,
		lastSyncAt:      lastSyncAt,
		createdAt:       createdAt,
		updatedAt:       updatedAt,
	}
}

// Accessors.

// ID returns the connector's unique identifier.
func (c *Connector) ID() uuid.UUID { return c.id }

// Name returns the connector's name.
func (c *Connector) Name() string { return c.name }

// ConnectorType returns the connector type.
func (c *Connector) ConnectorType() valueobject.ConnectorType { return c.connectorType }

// BaseURL returns the provider base URL.
func (c *Connector) BaseURL() string { return c.baseURL }

// AuthToken returns the optional auth token.
func (c *Connector) AuthToken() *string { return c.authToken }

// Status returns the current status.
func (c *Connector) Status() valueobject.ConnectorStatus { return c.status }

// RepositoryCount returns the number of repositories discovered by the last sync.
func (c *Connector) RepositoryCount() int { return c.repositoryCount }

// LastSyncAt returns the time of the last successful sync, or nil.
func (c *Connector) LastSyncAt() *time.Time { return c.lastSyncAt }

// CreatedAt returns the creation timestamp.
func (c *Connector) CreatedAt() time.Time { return c.createdAt }

// UpdatedAt returns the last-update timestamp.
func (c *Connector) UpdatedAt() time.Time { return c.updatedAt }

// UpdateStatus transitions the connector to the given status.
// Returns an error if the transition is not allowed.
func (c *Connector) UpdateStatus(target valueobject.ConnectorStatus) error {
	if !c.status.CanTransitionTo(target) {
		return errors.New("invalid status transition from " + c.status.String() + " to " + target.String())
	}
	c.status = target
	c.updatedAt = time.Now()
	return nil
}

// MarkSyncStarted transitions the connector into the syncing state.
func (c *Connector) MarkSyncStarted() error {
	return c.UpdateStatus(valueobject.ConnectorStatusSyncing)
}

// MarkSyncCompleted transitions back to active and records the repository count.
func (c *Connector) MarkSyncCompleted(repositoryCount int) error {
	if err := c.UpdateStatus(valueobject.ConnectorStatusActive); err != nil {
		return err
	}
	c.repositoryCount = repositoryCount
	now := time.Now()
	c.lastSyncAt = &now
	c.updatedAt = now
	return nil
}

// MarkSyncFailed transitions the connector to the error state.
func (c *Connector) MarkSyncFailed() error {
	return c.UpdateStatus(valueobject.ConnectorStatusError)
}

// Deactivate transitions the connector to inactive.
func (c *Connector) Deactivate() error {
	return c.UpdateStatus(valueobject.ConnectorStatusInactive)
}

// Activate transitions the connector to active.
func (c *Connector) Activate() error {
	return c.UpdateStatus(valueobject.ConnectorStatusActive)
}

// Equal reports whether two connectors share the same identity.
func (c *Connector) Equal(other *Connector) bool {
	if other == nil {
		return false
	}
	return c.id == other.id
}
