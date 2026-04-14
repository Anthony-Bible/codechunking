package service

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/config"
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"os"
	"slices"
)

// reconcilerFetchLimit is a high limit used to load all connectors from the DB in a single pass.
const reconcilerFetchLimit = 1_000_000

// ConnectorReconciler reconciles connector configuration with the database state.
// Config is the single source of truth: connectors in config are created/updated,
// and connectors removed from config are marked inactive.
type ConnectorReconciler struct {
	connectorRepo outbound.ConnectorRepository
}

// NewConnectorReconciler creates a new ConnectorReconciler.
func NewConnectorReconciler(repo outbound.ConnectorRepository) *ConnectorReconciler {
	return &ConnectorReconciler{connectorRepo: repo}
}

// Reconcile syncs the database connector state with cfgConnectors.
// It is non-fatal: errors for individual connectors are logged and skipped.
func (r *ConnectorReconciler) Reconcile(ctx context.Context, cfgConnectors []config.ConnectorConfig) error {
	dbConnectors, _, err := r.connectorRepo.FindAll(ctx, outbound.ConnectorFilters{Limit: reconcilerFetchLimit})
	if err != nil {
		return fmt.Errorf("loading connectors from db: %w", err)
	}

	dbByName := make(map[string]*entity.Connector, len(dbConnectors))
	for _, c := range dbConnectors {
		dbByName[c.Name()] = c
	}

	configNames := make(map[string]bool, len(cfgConnectors))
	for _, cfg := range cfgConnectors {
		configNames[cfg.Name] = true
	}

	for _, cfg := range cfgConnectors {
		r.upsertConnector(ctx, cfg, dbByName)
	}

	for name, connector := range dbByName {
		if configNames[name] {
			continue
		}
		if connector.Status() == valueobject.ConnectorStatusInactive {
			continue
		}
		connector.MarkInactive()
		if err := r.connectorRepo.Update(ctx, connector); err != nil {
			slogger.Error(ctx, "connector reconciler: failed to mark connector inactive", slogger.Fields{
				"name":  name,
				"error": err.Error(),
			})
			continue
		}
		slogger.Info(ctx, "connector removed from config, marking inactive", slogger.Fields{"name": name})
	}

	return nil
}

// upsertConnector creates or updates a single connector based on config.
func (r *ConnectorReconciler) upsertConnector(
	ctx context.Context,
	cfg config.ConnectorConfig,
	dbByName map[string]*entity.Connector,
) {
	ct, err := valueobject.NewConnectorType(cfg.Type)
	if err != nil {
		slogger.Error(ctx, "connector reconciler: invalid connector type in config, skipping", slogger.Fields{
			"name":  cfg.Name,
			"type":  cfg.Type,
			"error": err.Error(),
		})
		return
	}

	authToken := os.ExpandEnv(cfg.AuthToken)
	var authTokenPtr *string
	if authToken != "" {
		authTokenPtr = &authToken
	}

	groups := cfg.Groups
	if groups == nil {
		groups = []string{}
	}
	projects := cfg.Projects
	if projects == nil {
		projects = []string{}
	}

	existing, inDB := dbByName[cfg.Name]
	if !inDB {
		r.createConnector(ctx, cfg.Name, ct, cfg.BaseURL, authTokenPtr, groups, projects)
		return
	}

	r.updateConnectorIfChanged(ctx, existing, cfg.BaseURL, authTokenPtr, groups, projects)
}

// createConnector creates and persists a new connector entity.
func (r *ConnectorReconciler) createConnector(
	ctx context.Context,
	name string,
	ct valueobject.ConnectorType,
	baseURL string,
	authToken *string,
	groups, projects []string,
) {
	connector, err := entity.NewConnector(name, ct, baseURL, authToken, groups, projects)
	if err != nil {
		slogger.Error(ctx, "connector reconciler: failed to build connector entity", slogger.Fields{
			"name":  name,
			"error": err.Error(),
		})
		return
	}
	if err := r.connectorRepo.Save(ctx, connector); err != nil {
		slogger.Error(ctx, "connector reconciler: failed to save new connector", slogger.Fields{
			"name":  name,
			"error": err.Error(),
		})
		return
	}
	slogger.Info(ctx, "connector reconciler: connector created from config", slogger.Fields{"name": name})
}

// updateConnectorIfChanged applies config changes to an existing DB connector and persists if needed.
func (r *ConnectorReconciler) updateConnectorIfChanged(
	ctx context.Context,
	existing *entity.Connector,
	baseURL string,
	authTokenPtr *string,
	groups, projects []string,
) {
	changed := false

	if existing.Status() == valueobject.ConnectorStatusSyncing {
		if err := existing.Activate(); err != nil {
			slogger.Error(ctx, "connector reconciler: failed to recover stuck syncing connector", slogger.Fields{
				"name":  existing.Name(),
				"error": err.Error(),
			})
		} else {
			changed = true
			slogger.Warn(ctx, "connector reconciler: recovered connector stuck in syncing state after restart", slogger.Fields{"name": existing.Name()})
		}
	}

	if existing.Status() == valueobject.ConnectorStatusInactive {
		if err := existing.Activate(); err != nil {
			slogger.Error(ctx, "connector reconciler: failed to reactivate connector", slogger.Fields{
				"name":  existing.Name(),
				"error": err.Error(),
			})
		} else {
			changed = true
			slogger.Info(ctx, "connector reconciler: reactivating connector", slogger.Fields{"name": existing.Name()})
		}
	}

	if existing.BaseURL() != baseURL {
		existing.SetBaseURL(baseURL)
		changed = true
	}

	newToken := ""
	if authTokenPtr != nil {
		newToken = *authTokenPtr
	}
	existingToken := ""
	if existing.AuthToken() != nil {
		existingToken = *existing.AuthToken()
	}
	if existingToken != newToken {
		existing.SetAuthToken(authTokenPtr)
		changed = true
	}

	if !slices.Equal(existing.Groups(), groups) {
		existing.SetGroups(groups)
		changed = true
	}

	if !slices.Equal(existing.Projects(), projects) {
		existing.SetProjects(projects)
		changed = true
	}

	if !changed {
		return
	}

	if err := r.connectorRepo.Update(ctx, existing); err != nil {
		slogger.Error(ctx, "connector reconciler: failed to update connector", slogger.Fields{
			"name":  existing.Name(),
			"error": err.Error(),
		})
		return
	}
	slogger.Info(ctx, "connector reconciler: connector updated from config", slogger.Fields{"name": existing.Name()})
}
