package outbound

import (
	"context"

	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"

	"github.com/google/uuid"
)

// RepositoryRepository defines the outbound port for repository persistence.
type RepositoryRepository interface {
	Save(ctx context.Context, repository *entity.Repository) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Repository, error)
	FindByURL(ctx context.Context, url valueobject.RepositoryURL) (*entity.Repository, error)
	FindAll(ctx context.Context, filters RepositoryFilters) ([]*entity.Repository, int, error)
	Update(ctx context.Context, repository *entity.Repository) error
	Delete(ctx context.Context, id uuid.UUID) error
	Exists(ctx context.Context, url valueobject.RepositoryURL) (bool, error)
	// Normalized URL duplicate detection methods
	ExistsByNormalizedURL(ctx context.Context, url valueobject.RepositoryURL) (bool, error)
	FindByNormalizedURL(ctx context.Context, url valueobject.RepositoryURL) (*entity.Repository, error)
}

// IndexingJobRepository defines the outbound port for indexing job persistence.
type IndexingJobRepository interface {
	Save(ctx context.Context, job *entity.IndexingJob) error
	FindByID(ctx context.Context, id uuid.UUID) (*entity.IndexingJob, error)
	FindByRepositoryID(
		ctx context.Context,
		repositoryID uuid.UUID,
		filters IndexingJobFilters,
	) ([]*entity.IndexingJob, int, error)
	Update(ctx context.Context, job *entity.IndexingJob) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// MessagePublisher defines the outbound port for publishing messages to the job queue.
type MessagePublisher interface {
	PublishIndexingJob(ctx context.Context, repositoryID uuid.UUID, repositoryURL string) error
}

// MessagePublisherHealth defines health monitoring capabilities for message publishers.
type MessagePublisherHealth interface {
	GetConnectionHealth() MessagePublisherHealthStatus
	GetMessageMetrics() MessagePublisherMetrics
}

// MessagePublisherHealthStatus represents the health status of a message publisher.
type MessagePublisherHealthStatus struct {
	Connected        bool   `json:"connected"`
	LastError        string `json:"last_error,omitempty"`
	Uptime           string `json:"uptime"`
	Reconnects       int    `json:"reconnects"`
	JetStreamEnabled bool   `json:"jetstream_enabled"`
	CircuitBreaker   string `json:"circuit_breaker"`
}

// MessagePublisherMetrics represents message publishing metrics.
type MessagePublisherMetrics struct {
	PublishedCount int64  `json:"published_count"`
	FailedCount    int64  `json:"failed_count"`
	AverageLatency string `json:"average_latency"`
}

// RepositoryFilters represents filters for repository queries.
type RepositoryFilters struct {
	Status *valueobject.RepositoryStatus
	Limit  int
	Offset int
	Sort   string
}

// IndexingJobFilters represents filters for indexing job queries.
type IndexingJobFilters struct {
	Limit  int
	Offset int
}
