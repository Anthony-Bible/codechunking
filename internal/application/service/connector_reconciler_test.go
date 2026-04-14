package service_test

import (
	"codechunking/internal/application/common/logging"
	"codechunking/internal/application/common/slogger"
	appservice "codechunking/internal/application/service"
	"codechunking/internal/config"
	"codechunking/internal/domain/entity"
	domainerrors "codechunking/internal/domain/errors/domain"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// In-memory repository for reconciler tests
// ---------------------------------------------------------------------------

type memConnectorRepo struct {
	connectors   []*entity.Connector
	saveCalled   int
	updateCalled int
}

func (m *memConnectorRepo) Save(_ context.Context, connector *entity.Connector) error {
	m.connectors = append(m.connectors, connector)
	m.saveCalled++
	return nil
}

func (m *memConnectorRepo) FindByID(_ context.Context, id uuid.UUID) (*entity.Connector, error) {
	for _, c := range m.connectors {
		if c.ID() == id {
			return c, nil
		}
	}
	return nil, domainerrors.ErrConnectorNotFound
}

func (m *memConnectorRepo) FindByName(_ context.Context, name string) (*entity.Connector, error) {
	for _, c := range m.connectors {
		if c.Name() == name {
			return c, nil
		}
	}
	return nil, domainerrors.ErrConnectorNotFound
}

func (m *memConnectorRepo) FindAll(_ context.Context, _ outbound.ConnectorFilters) ([]*entity.Connector, int, error) {
	return m.connectors, len(m.connectors), nil
}

func (m *memConnectorRepo) Update(_ context.Context, connector *entity.Connector) error {
	m.updateCalled++
	for i, c := range m.connectors {
		if c.ID() == connector.ID() {
			m.connectors[i] = connector
			return nil
		}
	}
	return errors.New("connector not found for update")
}

func (m *memConnectorRepo) Delete(_ context.Context, id uuid.UUID) error {
	for i, c := range m.connectors {
		if c.ID() == id {
			m.connectors = append(m.connectors[:i], m.connectors[i+1:]...)
			return nil
		}
	}
	return domainerrors.ErrConnectorNotFound
}

func (m *memConnectorRepo) Exists(_ context.Context, name string) (bool, error) {
	for _, c := range m.connectors {
		if c.Name() == name {
			return true, nil
		}
	}
	return false, nil
}

func (m *memConnectorRepo) connectorByName(name string) *entity.Connector {
	for _, c := range m.connectors {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func initReconcilerLogger(t *testing.T) {
	t.Helper()
	silentLogger, err := logging.NewApplicationLogger(logging.Config{
		Level:  "ERROR",
		Format: "json",
		Output: "buffer",
	})
	require.NoError(t, err)
	slogger.SetGlobalLogger(silentLogger)
}

func restoreConnectorActive(name string) *entity.Connector {
	now := time.Now()
	c := entity.RestoreConnector(
		uuid.New(), name, valueobject.ConnectorTypeGitLab,
		"https://gitlab.com", nil,
		valueobject.ConnectorStatusActive, 0, nil,
		now, now,
		[]string{"old-group"}, []string{},
	)
	return c
}

func restoreConnectorInactive(name string) *entity.Connector {
	now := time.Now()
	c := entity.RestoreConnector(
		uuid.New(), name, valueobject.ConnectorTypeGitLab,
		"https://gitlab.com", nil,
		valueobject.ConnectorStatusInactive, 0, nil,
		now, now,
		[]string{}, []string{},
	)
	return c
}

func gitlabCfg(name string) config.ConnectorConfig {
	return config.ConnectorConfig{
		Name:     name,
		Type:     "gitlab",
		BaseURL:  "https://gitlab.com",
		Groups:   []string{"my-group"},
		Projects: []string{},
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// Case 1: empty config + empty DB → no-op.
func TestConnectorReconciler_EmptyConfigAndDB_NoOp(t *testing.T) {
	initReconcilerLogger(t)
	repo := &memConnectorRepo{}
	r := appservice.NewConnectorReconciler(repo)

	err := r.Reconcile(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, 0, repo.saveCalled)
	assert.Equal(t, 0, repo.updateCalled)
}

// Case 2: config connector not in DB → created.
func TestConnectorReconciler_NewConfigConnector_IsCreated(t *testing.T) {
	initReconcilerLogger(t)
	repo := &memConnectorRepo{}
	r := appservice.NewConnectorReconciler(repo)

	err := r.Reconcile(context.Background(), []config.ConnectorConfig{gitlabCfg("my-gitlab")})

	require.NoError(t, err)
	assert.Equal(t, 1, repo.saveCalled, "expected one connector to be saved")
	assert.Equal(t, 0, repo.updateCalled)

	c := repo.connectorByName("my-gitlab")
	require.NotNil(t, c)
	assert.Equal(t, valueobject.ConnectorTypeGitLab, c.ConnectorType())
	assert.Equal(t, []string{"my-group"}, c.Groups())
}

// Case 3: config connector in DB, unchanged → no update called.
func TestConnectorReconciler_UnchangedConnector_NoUpdate(t *testing.T) {
	initReconcilerLogger(t)
	repo := &memConnectorRepo{}
	existing := entity.RestoreConnector(
		uuid.New(), "my-gitlab", valueobject.ConnectorTypeGitLab,
		"https://gitlab.com", nil,
		valueobject.ConnectorStatusActive, 0, nil,
		time.Now(), time.Now(),
		[]string{"my-group"}, []string{},
	)
	repo.connectors = []*entity.Connector{existing}

	r := appservice.NewConnectorReconciler(repo)
	err := r.Reconcile(context.Background(), []config.ConnectorConfig{gitlabCfg("my-gitlab")})

	require.NoError(t, err)
	assert.Equal(t, 0, repo.saveCalled)
	assert.Equal(t, 0, repo.updateCalled, "unchanged connector should not trigger an update")
}

// Case 4: config connector in DB, baseURL changed → updated.
func TestConnectorReconciler_BaseURLChanged_Updated(t *testing.T) {
	initReconcilerLogger(t)
	repo := &memConnectorRepo{}
	existing := restoreConnectorActive("my-gitlab")
	repo.connectors = []*entity.Connector{existing}

	cfg := gitlabCfg("my-gitlab")
	cfg.BaseURL = "https://new-gitlab.example.com"

	r := appservice.NewConnectorReconciler(repo)
	err := r.Reconcile(context.Background(), []config.ConnectorConfig{cfg})

	require.NoError(t, err)
	assert.Equal(t, 1, repo.updateCalled)

	c := repo.connectorByName("my-gitlab")
	require.NotNil(t, c)
	assert.Equal(t, "https://new-gitlab.example.com", c.BaseURL())
}

// Case 5: config connector in DB, groups list changed → updated.
func TestConnectorReconciler_GroupsChanged_Updated(t *testing.T) {
	initReconcilerLogger(t)
	repo := &memConnectorRepo{}
	existing := restoreConnectorActive("my-gitlab")
	repo.connectors = []*entity.Connector{existing}

	cfg := gitlabCfg("my-gitlab")
	cfg.Groups = []string{"group-a", "group-b"}

	r := appservice.NewConnectorReconciler(repo)
	err := r.Reconcile(context.Background(), []config.ConnectorConfig{cfg})

	require.NoError(t, err)
	assert.Equal(t, 1, repo.updateCalled)

	c := repo.connectorByName("my-gitlab")
	require.NotNil(t, c)
	assert.Equal(t, []string{"group-a", "group-b"}, c.Groups())
}

// Case 6: config connector was inactive → reactivated on reconcile.
func TestConnectorReconciler_InactiveConnector_Reactivated(t *testing.T) {
	initReconcilerLogger(t)
	repo := &memConnectorRepo{}
	inactive := restoreConnectorInactive("my-gitlab")
	repo.connectors = []*entity.Connector{inactive}

	r := appservice.NewConnectorReconciler(repo)
	err := r.Reconcile(context.Background(), []config.ConnectorConfig{gitlabCfg("my-gitlab")})

	require.NoError(t, err)
	assert.Equal(t, 1, repo.updateCalled, "reactivation should trigger an update")

	c := repo.connectorByName("my-gitlab")
	require.NotNil(t, c)
	assert.Equal(t, valueobject.ConnectorStatusActive, c.Status())
}

// Case 7: DB connector not in config → marked inactive.
func TestConnectorReconciler_RemovedFromConfig_MarkedInactive(t *testing.T) {
	initReconcilerLogger(t)
	repo := &memConnectorRepo{}
	active := restoreConnectorActive("stale-connector")
	repo.connectors = []*entity.Connector{active}

	r := appservice.NewConnectorReconciler(repo)
	// Reconcile with empty config → stale-connector should be deactivated.
	err := r.Reconcile(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, 1, repo.updateCalled)

	c := repo.connectorByName("stale-connector")
	require.NotNil(t, c)
	assert.Equal(t, valueobject.ConnectorStatusInactive, c.Status())
}

// Case 8: DB connector already inactive, not in config → no update called (idempotent).
func TestConnectorReconciler_AlreadyInactive_NotInConfig_NoUpdate(t *testing.T) {
	initReconcilerLogger(t)
	repo := &memConnectorRepo{}
	inactive := restoreConnectorInactive("old-connector")
	repo.connectors = []*entity.Connector{inactive}

	r := appservice.NewConnectorReconciler(repo)
	err := r.Reconcile(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, 0, repo.updateCalled, "already-inactive connector should not be updated")
}

func restoreConnectorSyncing(name string, repositoryCount int) *entity.Connector {
	now := time.Now()
	c := entity.RestoreConnector(
		uuid.New(), name, valueobject.ConnectorTypeGitLab,
		"https://gitlab.com", nil,
		valueobject.ConnectorStatusSyncing, repositoryCount, nil,
		now, now,
		[]string{"my-group"}, []string{},
	)
	return c
}

// Case 10: connector stuck in syncing status that IS in config → recovered to active.
func TestReconciler_RecoversSyncingConnector(t *testing.T) {
	initReconcilerLogger(t)
	repo := &memConnectorRepo{}
	stuck := restoreConnectorSyncing("my-gitlab", 42)
	repo.connectors = []*entity.Connector{stuck}

	r := appservice.NewConnectorReconciler(repo)
	err := r.Reconcile(context.Background(), []config.ConnectorConfig{gitlabCfg("my-gitlab")})

	require.NoError(t, err)
	assert.Equal(t, 1, repo.updateCalled, "stuck syncing connector should trigger an update")

	c := repo.connectorByName("my-gitlab")
	require.NotNil(t, c)
	assert.Equal(t, valueobject.ConnectorStatusActive, c.Status(),
		"syncing connector present in config should be recovered to active")
}

// Case 10b: recovery path must NOT modify lastSyncAt (no sync actually completed).
func TestReconciler_RecoversSyncingConnector_DoesNotModifyLastSyncAt(t *testing.T) {
	initReconcilerLogger(t)
	repo := &memConnectorRepo{}

	// Record a known lastSyncAt before the connector got stuck.
	pastSync := time.Now().Add(-2 * time.Hour)
	stuck := entity.RestoreConnector(
		uuid.New(), "my-gitlab", valueobject.ConnectorTypeGitLab,
		"https://gitlab.com", nil,
		valueobject.ConnectorStatusSyncing, 42, &pastSync,
		time.Now(), time.Now(),
		[]string{"my-group"}, []string{},
	)
	repo.connectors = []*entity.Connector{stuck}

	r := appservice.NewConnectorReconciler(repo)
	err := r.Reconcile(context.Background(), []config.ConnectorConfig{gitlabCfg("my-gitlab")})

	require.NoError(t, err)
	c := repo.connectorByName("my-gitlab")
	require.NotNil(t, c)
	assert.Equal(t, valueobject.ConnectorStatusActive, c.Status())
	require.NotNil(t, c.LastSyncAt(), "lastSyncAt should still be set")
	assert.Equal(t, pastSync.UTC(), c.LastSyncAt().UTC(),
		"recovery must not falsify lastSyncAt — no sync actually completed")
}

// Case 11: connector stuck in syncing status that is NOT in config → marked inactive.
func TestReconciler_RecoversSyncingConnector_NotInConfig(t *testing.T) {
	initReconcilerLogger(t)
	repo := &memConnectorRepo{}
	stuck := restoreConnectorSyncing("orphaned-gitlab", 7)
	repo.connectors = []*entity.Connector{stuck}

	r := appservice.NewConnectorReconciler(repo)
	// Reconcile with empty config → orphaned syncing connector should be deactivated.
	err := r.Reconcile(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, 1, repo.updateCalled, "stuck syncing connector not in config should trigger an update")

	c := repo.connectorByName("orphaned-gitlab")
	require.NotNil(t, c)
	assert.Equal(t, valueobject.ConnectorStatusInactive, c.Status(),
		"syncing connector absent from config should be marked inactive")
}

// Case 9: invalid type in config → logged and skipped, rest still reconciles.
func TestConnectorReconciler_InvalidTypeInConfig_SkipsAndContinues(t *testing.T) {
	initReconcilerLogger(t)
	repo := &memConnectorRepo{}

	cfgs := []config.ConnectorConfig{
		{Name: "bad-connector", Type: "invalid_type", BaseURL: "https://gitlab.com"},
		gitlabCfg("good-connector"),
	}

	r := appservice.NewConnectorReconciler(repo)
	err := r.Reconcile(context.Background(), cfgs)

	require.NoError(t, err)
	assert.Equal(t, 1, repo.saveCalled, "only the valid connector should be saved")
	assert.Nil(t, repo.connectorByName("bad-connector"), "invalid connector should not be persisted")
	assert.NotNil(t, repo.connectorByName("good-connector"), "valid connector should be persisted")
}
