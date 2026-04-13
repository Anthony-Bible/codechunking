package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateConnectorRequest_RequiredFields(t *testing.T) {
	t.Run("valid_request_with_all_fields", func(t *testing.T) {
		authToken := "ghp_test"
		req := CreateConnectorRequest{
			Name:          "my-org-connector",
			ConnectorType: "gitlab",
			BaseURL:       "https://gitlab.com",
			AuthToken:     &authToken,
		}

		assert.Equal(t, "my-org-connector", req.Name)
		assert.Equal(t, "gitlab", req.ConnectorType)
		assert.Equal(t, "https://gitlab.com", req.BaseURL)
		require.NotNil(t, req.AuthToken)
		assert.Equal(t, authToken, *req.AuthToken)
	})

	t.Run("valid_request_without_auth_token", func(t *testing.T) {
		req := CreateConnectorRequest{
			Name:          "my-gitlab-connector",
			ConnectorType: "gitlab",
			BaseURL:       "https://gitlab.com",
		}

		assert.Equal(t, "my-gitlab-connector", req.Name)
		assert.Nil(t, req.AuthToken)
	})
}

func TestCreateConnectorRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateConnectorRequest
		wantErr bool
	}{
		{
			name: "valid_request",
			req: CreateConnectorRequest{
				Name:          "test-connector",
				ConnectorType: "gitlab",
				BaseURL:       "https://gitlab.com",
			},
			wantErr: false,
		},
		{
			name: "empty_name_returns_error",
			req: CreateConnectorRequest{
				Name:          "",
				ConnectorType: "gitlab",
				BaseURL:       "https://gitlab.com",
			},
			wantErr: true,
		},
		{
			name: "empty_connector_type_returns_error",
			req: CreateConnectorRequest{
				Name:          "test-connector",
				ConnectorType: "",
				BaseURL:       "https://gitlab.com",
			},
			wantErr: true,
		},
		{
			name: "empty_base_url_returns_error",
			req: CreateConnectorRequest{
				Name:          "test-connector",
				ConnectorType: "gitlab",
				BaseURL:       "",
			},
			wantErr: true,
		},
		{
			name: "invalid_connector_type_returns_error",
			req: CreateConnectorRequest{
				Name:          "test-connector",
				ConnectorType: "not_a_valid_type",
				BaseURL:       "https://gitlab.com",
			},
			wantErr: true,
		},
		{
			name: "name_exceeding_max_length_returns_error",
			req: CreateConnectorRequest{
				Name:          string(make([]byte, 256)),
				ConnectorType: "gitlab",
				BaseURL:       "https://gitlab.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultConnectorListQuery(t *testing.T) {
	q := DefaultConnectorListQuery()

	assert.Equal(t, DefaultConnectorLimit, q.Limit)
	assert.Equal(t, 0, q.Offset)
	assert.Empty(t, q.ConnectorType)
	assert.Empty(t, q.Status)
}

func TestConnectorListQuery_DefaultLimit(t *testing.T) {
	q := DefaultConnectorListQuery()
	assert.Positive(t, q.Limit, "default limit should be positive")
	assert.LessOrEqual(t, q.Limit, MaxLimitValue, "default limit should not exceed max limit")
}

func TestConnectorListQuery_Validate(t *testing.T) {
	tests := []struct {
		name    string
		query   ConnectorListQuery
		wantErr bool
	}{
		{
			name:    "default_query_is_valid",
			query:   DefaultConnectorListQuery(),
			wantErr: false,
		},
		{
			name: "valid_connector_type_filter",
			query: ConnectorListQuery{
				ConnectorType: "gitlab",
				Limit:         10,
				Offset:        0,
			},
			wantErr: false,
		},
		{
			name: "valid_status_filter",
			query: ConnectorListQuery{
				Status: "active",
				Limit:  10,
				Offset: 0,
			},
			wantErr: false,
		},
		{
			name: "invalid_connector_type_filter_returns_error",
			query: ConnectorListQuery{
				ConnectorType: "not_valid",
				Limit:         10,
			},
			wantErr: true,
		},
		{
			name: "invalid_status_filter_returns_error",
			query: ConnectorListQuery{
				Status: "not_valid",
				Limit:  10,
			},
			wantErr: true,
		},
		{
			name: "limit_over_max_returns_error",
			query: ConnectorListQuery{
				Limit: MaxLimitValue + 1,
			},
			wantErr: true,
		},
		{
			name: "negative_offset_returns_error",
			query: ConnectorListQuery{
				Limit:  10,
				Offset: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.query.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
