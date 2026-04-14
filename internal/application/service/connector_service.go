package service

import (
	"codechunking/internal/application/common"
	"codechunking/internal/application/common/security"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/entity"
	domainerrors "codechunking/internal/domain/errors/domain"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"net/url"

	"github.com/google/uuid"
)

// =============================================================================
// GetConnectorService
// =============================================================================

// GetConnectorService handles fetching a single connector.
type GetConnectorService struct {
	connectorRepo outbound.ConnectorRepository
}

// NewGetConnectorService constructs a GetConnectorService.
func NewGetConnectorService(connectorRepo outbound.ConnectorRepository) *GetConnectorService {
	return &GetConnectorService{connectorRepo: connectorRepo}
}

// GetConnector returns a connector by ID.
func (s *GetConnectorService) GetConnector(ctx context.Context, id uuid.UUID) (*dto.ConnectorResponse, error) {
	slogger.Info(ctx, "Getting connector", slogger.Fields{"id": id})

	connector, err := s.connectorRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return common.EntityToConnectorResponse(connector), nil
}

// =============================================================================
// ListConnectorsService
// =============================================================================

// ListConnectorsService handles listing connectors with filters.
type ListConnectorsService struct {
	connectorRepo outbound.ConnectorRepository
}

// NewListConnectorsService constructs a ListConnectorsService.
func NewListConnectorsService(connectorRepo outbound.ConnectorRepository) *ListConnectorsService {
	return &ListConnectorsService{connectorRepo: connectorRepo}
}

// ListConnectors returns a paginated list of connectors.
func (s *ListConnectorsService) ListConnectors(
	ctx context.Context,
	query dto.ConnectorListQuery,
) (*dto.ConnectorListResponse, error) {
	slogger.Info(ctx, "Listing connectors", slogger.Fields{"limit": query.Limit, "offset": query.Offset})

	filters := outbound.ConnectorFilters{
		Limit:  query.Limit,
		Offset: query.Offset,
	}

	if query.ConnectorType != "" {
		ct, err := valueobject.NewConnectorType(query.ConnectorType)
		if err != nil {
			return nil, err
		}
		filters.ConnectorType = &ct
	}

	if query.Status != "" {
		cs, err := valueobject.NewConnectorStatus(query.Status)
		if err != nil {
			return nil, err
		}
		filters.Status = &cs
	}

	connectors, total, err := s.connectorRepo.FindAll(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("listing connectors: %w", err)
	}

	responses := make([]dto.ConnectorResponse, 0, len(connectors))
	for _, c := range connectors {
		r := common.EntityToConnectorResponse(c)
		responses = append(responses, *r)
	}

	return &dto.ConnectorListResponse{
		Connectors: responses,
		Pagination: dto.PaginationResponse{
			Limit:   query.Limit,
			Offset:  query.Offset,
			Total:   total,
			HasMore: query.Offset+len(responses) < total,
		},
	}, nil
}

// =============================================================================
// SyncConnectorService
// =============================================================================

// SyncConnectorService triggers a sync for a connector.
type SyncConnectorService struct {
	connectorRepo    outbound.ConnectorRepository
	gitProvider      outbound.GitProvider
	repositoryRepo   outbound.RepositoryRepository
	indexingJobRepo  outbound.IndexingJobRepository
	messagePublisher outbound.MessagePublisher
}

// NewSyncConnectorService constructs a SyncConnectorService.
func NewSyncConnectorService(
	connectorRepo outbound.ConnectorRepository,
	gitProvider outbound.GitProvider,
	repositoryRepo outbound.RepositoryRepository,
	indexingJobRepo outbound.IndexingJobRepository,
	messagePublisher outbound.MessagePublisher,
) *SyncConnectorService {
	return &SyncConnectorService{
		connectorRepo:    connectorRepo,
		gitProvider:      gitProvider,
		repositoryRepo:   repositoryRepo,
		indexingJobRepo:  indexingJobRepo,
		messagePublisher: messagePublisher,
	}
}

// SyncConnector fetches repos from the git provider, creates repository and indexing job
// records for new repos, publishes jobs to NATS, and marks the connector completed.
func (s *SyncConnectorService) SyncConnector(
	ctx context.Context,
	id uuid.UUID,
) (*dto.SyncConnectorResponse, error) {
	slogger.Info(ctx, "Triggering connector sync", slogger.Fields{"id": id})

	connector, err := s.connectorRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if connector.Status() == valueobject.ConnectorStatusSyncing {
		return nil, domainerrors.ErrConnectorSyncing
	}

	// A freshly reconciled connector starts in pending state.
	// Promote it to active before transitioning to syncing.
	if connector.Status() == valueobject.ConnectorStatusPending {
		if err := connector.UpdateStatus(valueobject.ConnectorStatusActive); err != nil {
			return nil, fmt.Errorf("activating pending connector: %w", err)
		}
	}

	if err := connector.MarkSyncStarted(); err != nil {
		return nil, fmt.Errorf("marking sync started: %w", err)
	}

	if err := s.connectorRepo.Update(ctx, connector); err != nil {
		return nil, fmt.Errorf("updating connector: %w", err)
	}

	// Fetch repositories from the git provider.
	providerRepos, err := s.gitProvider.ListRepositories(ctx, connector)
	if err != nil {
		slogger.Error(ctx, "Failed to list repositories from git provider", slogger.Fields{
			"connector_id": id,
			"error":        err.Error(),
		})
		if markErr := connector.MarkSyncFailed(); markErr != nil {
			slogger.Error(ctx, "Failed to mark connector sync as failed", slogger.Fields{
				"connector_id": id,
				"error":        markErr.Error(),
			})
		} else if updateErr := s.connectorRepo.Update(ctx, connector); updateErr != nil {
			slogger.Error(ctx, "Failed to persist connector failed sync status", slogger.Fields{
				"connector_id": id,
				"error":        updateErr.Error(),
			})
		}
		return nil, fmt.Errorf("listing repositories: %w", err)
	}

	slogger.Info(ctx, "Repos returned from provider", slogger.Fields{"count": len(providerRepos)})

	cfg := security.DefaultConfig()
	if parsed, err := url.Parse(connector.BaseURL()); err == nil && parsed.Hostname() != "" {
		cfg.AdditionalAllowedHosts = []string{parsed.Hostname()}
	}

	for _, pr := range providerRepos {
		s.syncRepo(ctx, connector, pr, cfg)
	}

	if err := connector.MarkSyncCompleted(len(providerRepos)); err != nil {
		return nil, fmt.Errorf("marking sync completed: %w", err)
	}
	if err := s.connectorRepo.Update(ctx, connector); err != nil {
		return nil, fmt.Errorf("updating connector after sync: %w", err)
	}

	return &dto.SyncConnectorResponse{
		ConnectorID:       id,
		RepositoriesFound: len(providerRepos),
		Message:           "sync completed",
	}, nil
}

// syncRepo creates a repository and indexing job for a single provider repo.
// Returns true if the repo was successfully created and queued.
func (s *SyncConnectorService) syncRepo(ctx context.Context, connector *entity.Connector, pr outbound.GitProviderRepository, cfg *security.Config) bool {
	repoURL, err := valueobject.NewRepositoryURLWithConfig(pr.URL, cfg)
	if err != nil {
		slogger.Warn(ctx, "Skipping repo with invalid URL", slogger.Fields{"url": pr.URL, "error": err.Error()})
		return false
	}

	exists, err := s.repositoryRepo.ExistsByNormalizedURL(ctx, repoURL)
	if err != nil {
		slogger.Error(ctx, "Failed to check repo existence", slogger.Fields{"url": pr.URL, "error": err.Error()})
		return false
	}
	slogger.Info(ctx, "Repo existence check", slogger.Fields{"url": repoURL.Raw(), "exists": exists})
	if exists {
		return s.republishPendingJob(ctx, repoURL)
	}

	var desc *string
	if pr.Description != "" {
		description := pr.Description
		desc = &description
	}

	var branch *string
	if pr.DefaultBranch != "" {
		defaultBranch := pr.DefaultBranch
		branch = &defaultBranch
	}

	repo := entity.NewRepository(repoURL, pr.Name, desc, branch)
	if err := s.repositoryRepo.Save(ctx, repo); err != nil {
		slogger.Error(ctx, "Failed to save repository", slogger.Fields{"url": pr.URL, "error": err.Error()})
		return false
	}

	job := entity.NewIndexingJob(repo.ID())
	if err := s.indexingJobRepo.Save(ctx, job); err != nil {
		slogger.Error(ctx, "Failed to save indexing job", slogger.Fields{"repository_id": repo.ID(), "error": err.Error()})
		return false
	}

	if err := s.messagePublisher.PublishIndexingJob(ctx, job.ID(), repo.ID(), repoURL.CloneURL()); err != nil {
		slogger.Error(ctx, "Failed to publish indexing job", slogger.Fields{"job_id": job.ID(), "error": err.Error()})
		return false
	}

	return true
}

// republishPendingJob looks up a repo by URL and re-publishes any pending indexing job
// that was saved but never sent to NATS (e.g. due to a prior publish failure).
// Returns true if a job was successfully re-published.
func (s *SyncConnectorService) republishPendingJob(ctx context.Context, repoURL valueobject.RepositoryURL) bool {
	repo, err := s.repositoryRepo.FindByNormalizedURL(ctx, repoURL)
	if err != nil {
		slogger.Error(ctx, "republishPendingJob: FindByNormalizedURL failed", slogger.Fields{"url": repoURL.Raw(), "error": err.Error()})
		return false
	}
	if repo == nil {
		slogger.Warn(ctx, "republishPendingJob: repo not found by normalized URL", slogger.Fields{"url": repoURL.Raw()})
		return false
	}

	pendingStatus := valueobject.JobStatusPending
	jobs, _, err := s.indexingJobRepo.FindByRepositoryID(ctx, repo.ID(), outbound.IndexingJobFilters{Limit: 1, Status: &pendingStatus})
	if err != nil {
		slogger.Error(ctx, "republishPendingJob: FindByRepositoryID failed", slogger.Fields{"repository_id": repo.ID(), "error": err.Error()})
		return false
	}

	if len(jobs) == 0 {
		slogger.Info(ctx, "republishPendingJob: no pending jobs found", slogger.Fields{"repository_id": repo.ID()})
		return false
	}

	job := jobs[0]
	if err := s.messagePublisher.PublishIndexingJob(ctx, job.ID(), repo.ID(), repoURL.CloneURL()); err != nil {
		slogger.Error(ctx, "Failed to re-publish pending job", slogger.Fields{
			"job_id": job.ID(),
			"error":  err.Error(),
		})
		return false
	}
	slogger.Info(ctx, "Re-published orphaned pending job", slogger.Fields{
		"job_id":        job.ID(),
		"repository_id": repo.ID(),
	})
	return true
}
