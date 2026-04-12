package entity

import (
	"codechunking/internal/domain/valueobject"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConnector_ValidData(t *testing.T) {
	authToken := "ghp_testtoken123"

	connector, err := NewConnector(
		"my-github-org",
		valueobject.ConnectorTypeGitHubOrg,
		"https://api.github.com",
		&authToken,
	)

	require.NoError(t, err)
	require.NotNil(t, connector)

	assert.NotEqual(t, uuid.Nil, connector.ID())
	assert.Equal(t, "my-github-org", connector.Name())
	assert.Equal(t, valueobject.ConnectorTypeGitHubOrg, connector.ConnectorType())
	assert.Equal(t, "https://api.github.com", connector.BaseURL())
	require.NotNil(t, connector.AuthToken())
	assert.Equal(t, authToken, *connector.AuthToken())
	assert.Equal(t, valueobject.ConnectorStatusPending, connector.Status())
	assert.Equal(t, 0, connector.RepositoryCount())
	assert.Nil(t, connector.LastSyncAt())

	now := time.Now()
	assert.WithinDuration(t, now, connector.CreatedAt(), 5*time.Second)
	assert.WithinDuration(t, now, connector.UpdatedAt(), 5*time.Second)
}

func TestNewConnector_WithoutAuthToken(t *testing.T) {
	connector, err := NewConnector(
		"my-generic-connector",
		valueobject.ConnectorTypeGeneric,
		"https://gitlab.example.com",
		nil,
	)

	require.NoError(t, err)
	require.NotNil(t, connector)
	assert.Nil(t, connector.AuthToken())
}

func TestNewConnector_EmptyNameReturnsError(t *testing.T) {
	_, err := NewConnector(
		"",
		valueobject.ConnectorTypeGitHubOrg,
		"https://api.github.com",
		nil,
	)

	require.Error(t, err)
}

func TestNewConnector_EmptyBaseURLReturnsError(t *testing.T) {
	_, err := NewConnector(
		"my-connector",
		valueobject.ConnectorTypeGitHubOrg,
		"",
		nil,
	)

	require.Error(t, err)
}

func TestNewConnector_UniqueIDsPerInstance(t *testing.T) {
	c1, err := NewConnector("connector-1", valueobject.ConnectorTypeGitHubOrg, "https://api.github.com", nil)
	require.NoError(t, err)

	c2, err := NewConnector("connector-2", valueobject.ConnectorTypeGitLabGroup, "https://gitlab.com", nil)
	require.NoError(t, err)

	assert.NotEqual(t, c1.ID(), c2.ID())
}

func TestConnector_AllConnectorTypesSupported(t *testing.T) {
	types := []valueobject.ConnectorType{
		valueobject.ConnectorTypeGitHubOrg,
		valueobject.ConnectorTypeGitLabGroup,
		valueobject.ConnectorTypeBitbucket,
		valueobject.ConnectorTypeAzureDevOps,
		valueobject.ConnectorTypeGeneric,
	}

	for _, ct := range types {
		t.Run(ct.String(), func(t *testing.T) {
			connector, err := NewConnector("test-connector", ct, "https://example.com", nil)
			require.NoError(t, err)
			assert.Equal(t, ct, connector.ConnectorType())
		})
	}
}

func TestConnector_UpdateStatus_ValidTransition(t *testing.T) {
	connector, err := NewConnector(
		"my-connector",
		valueobject.ConnectorTypeGitHubOrg,
		"https://api.github.com",
		nil,
	)
	require.NoError(t, err)

	// pending → active
	err = connector.UpdateStatus(valueobject.ConnectorStatusActive)
	require.NoError(t, err)
	assert.Equal(t, valueobject.ConnectorStatusActive, connector.Status())
}

func TestConnector_UpdateStatus_InvalidTransition(t *testing.T) {
	connector, err := NewConnector(
		"my-connector",
		valueobject.ConnectorTypeGitHubOrg,
		"https://api.github.com",
		nil,
	)
	require.NoError(t, err)

	// pending → inactive is invalid
	err = connector.UpdateStatus(valueobject.ConnectorStatusInactive)
	require.Error(t, err)
	// Status should remain unchanged
	assert.Equal(t, valueobject.ConnectorStatusPending, connector.Status())
}

func TestConnector_MarkSyncStarted(t *testing.T) {
	connector, err := NewConnector(
		"my-connector",
		valueobject.ConnectorTypeGitHubOrg,
		"https://api.github.com",
		nil,
	)
	require.NoError(t, err)

	// Activate first
	require.NoError(t, connector.UpdateStatus(valueobject.ConnectorStatusActive))

	// Start sync
	err = connector.MarkSyncStarted()
	require.NoError(t, err)
	assert.Equal(t, valueobject.ConnectorStatusSyncing, connector.Status())
}

func TestConnector_MarkSyncCompleted(t *testing.T) {
	connector, err := NewConnector(
		"my-connector",
		valueobject.ConnectorTypeGitHubOrg,
		"https://api.github.com",
		nil,
	)
	require.NoError(t, err)

	require.NoError(t, connector.UpdateStatus(valueobject.ConnectorStatusActive))
	require.NoError(t, connector.MarkSyncStarted())

	err = connector.MarkSyncCompleted(42)
	require.NoError(t, err)
	assert.Equal(t, valueobject.ConnectorStatusActive, connector.Status())
	assert.Equal(t, 42, connector.RepositoryCount())
	require.NotNil(t, connector.LastSyncAt())
	assert.WithinDuration(t, time.Now(), *connector.LastSyncAt(), 5*time.Second)
}

func TestConnector_MarkSyncFailed(t *testing.T) {
	connector, err := NewConnector(
		"my-connector",
		valueobject.ConnectorTypeGitHubOrg,
		"https://api.github.com",
		nil,
	)
	require.NoError(t, err)

	require.NoError(t, connector.UpdateStatus(valueobject.ConnectorStatusActive))
	require.NoError(t, connector.MarkSyncStarted())

	err = connector.MarkSyncFailed()
	require.NoError(t, err)
	assert.Equal(t, valueobject.ConnectorStatusError, connector.Status())
}

func TestConnector_Deactivate(t *testing.T) {
	connector, err := NewConnector(
		"my-connector",
		valueobject.ConnectorTypeGitHubOrg,
		"https://api.github.com",
		nil,
	)
	require.NoError(t, err)

	require.NoError(t, connector.UpdateStatus(valueobject.ConnectorStatusActive))

	err = connector.Deactivate()
	require.NoError(t, err)
	assert.Equal(t, valueobject.ConnectorStatusInactive, connector.Status())
}

func TestConnector_Equal(t *testing.T) {
	connector, err := NewConnector("c1", valueobject.ConnectorTypeGitHubOrg, "https://api.github.com", nil)
	require.NoError(t, err)

	t.Run("same_id_is_equal", func(t *testing.T) {
		assert.True(t, connector.Equal(connector))
	})

	t.Run("different_id_is_not_equal", func(t *testing.T) {
		other, err := NewConnector("c2", valueobject.ConnectorTypeGitHubOrg, "https://api.github.com", nil)
		require.NoError(t, err)
		assert.False(t, connector.Equal(other))
	})

	t.Run("nil_is_not_equal", func(t *testing.T) {
		assert.False(t, connector.Equal(nil))
	})
}

func TestRestoreConnector(t *testing.T) {
	id := uuid.New()
	authToken := "token123"
	repoCount := 10
	lastSync := time.Now().Add(-1 * time.Hour)
	now := time.Now()

	connector := RestoreConnector(
		id,
		"restored-connector",
		valueobject.ConnectorTypeGitLabGroup,
		"https://gitlab.com",
		&authToken,
		valueobject.ConnectorStatusActive,
		repoCount,
		&lastSync,
		now,
		now,
	)

	require.NotNil(t, connector)
	assert.Equal(t, id, connector.ID())
	assert.Equal(t, "restored-connector", connector.Name())
	assert.Equal(t, valueobject.ConnectorTypeGitLabGroup, connector.ConnectorType())
	assert.Equal(t, "https://gitlab.com", connector.BaseURL())
	require.NotNil(t, connector.AuthToken())
	assert.Equal(t, authToken, *connector.AuthToken())
	assert.Equal(t, valueobject.ConnectorStatusActive, connector.Status())
	assert.Equal(t, repoCount, connector.RepositoryCount())
	require.NotNil(t, connector.LastSyncAt())
	assert.WithinDuration(t, lastSync, *connector.LastSyncAt(), time.Second)
}
