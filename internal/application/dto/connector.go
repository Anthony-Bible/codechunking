package dto

import (
	"codechunking/internal/domain/valueobject"
	"errors"
	"net/url"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultConnectorLimit is the default page size for connector list queries.
	DefaultConnectorLimit = 20
)

// CreateConnectorRequest is the payload for creating a new connector.
type CreateConnectorRequest struct {
	Name          string  `json:"name"`
	ConnectorType string  `json:"connector_type"`
	BaseURL       string  `json:"base_url"`
	AuthToken     *string `json:"auth_token,omitempty"`
}

// Validate checks that the request contains all required, well-formed fields.
func (r CreateConnectorRequest) Validate() error {
	if r.Name == "" {
		return errors.New("name is required")
	}
	if len(r.Name) > 255 {
		return errors.New("name must not exceed 255 characters")
	}
	if r.ConnectorType == "" {
		return errors.New("connector_type is required")
	}
	if _, err := valueobject.NewConnectorType(r.ConnectorType); err != nil {
		return err
	}
	if r.BaseURL == "" {
		return errors.New("base_url is required")
	}
	u, err := url.ParseRequestURI(r.BaseURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("base_url must be a valid http or https URL")
	}
	return nil
}

// ConnectorResponse is the representation of a connector returned by the API.
// The auth token is intentionally excluded for security.
type ConnectorResponse struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	ConnectorType   string     `json:"connector_type"`
	BaseURL         string     `json:"base_url"`
	Status          string     `json:"status"`
	RepositoryCount int        `json:"repository_count"`
	LastSyncAt      *time.Time `json:"last_sync_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ConnectorListResponse wraps a page of connectors with pagination metadata.
type ConnectorListResponse struct {
	Connectors []ConnectorResponse `json:"connectors"`
	Pagination PaginationResponse  `json:"pagination"`
}

// ConnectorListQuery holds the parameters used to filter and page connector lists.
type ConnectorListQuery struct {
	ConnectorType string `form:"connector_type"`
	Status        string `form:"status"`
	Limit         int    `form:"limit"`
	Offset        int    `form:"offset"`
}

// DefaultConnectorListQuery returns the default query parameters.
func DefaultConnectorListQuery() ConnectorListQuery {
	return ConnectorListQuery{
		Limit:  DefaultConnectorLimit,
		Offset: 0,
	}
}

// Validate checks that the query parameters are within acceptable ranges.
func (q ConnectorListQuery) Validate() error {
	if q.ConnectorType != "" {
		if _, err := valueobject.NewConnectorType(q.ConnectorType); err != nil {
			return err
		}
	}
	if q.Status != "" {
		if _, err := valueobject.NewConnectorStatus(q.Status); err != nil {
			return err
		}
	}
	if q.Limit > MaxLimitValue {
		return errors.New("limit exceeds maximum allowed value")
	}
	if q.Offset < 0 {
		return errors.New("offset must not be negative")
	}
	return nil
}

// SyncConnectorResponse is returned when a sync is triggered.
type SyncConnectorResponse struct {
	ConnectorID       uuid.UUID `json:"connector_id"`
	RepositoriesFound int       `json:"repositories_found"`
	Message           string    `json:"message"`
}
