// Package outbound defines the outbound port for external git provider integrations.
package outbound

import (
	"codechunking/internal/domain/entity"
	"context"
	"time"
)

// GitProvider defines the outbound port for communicating with external git hosting services.
// Each supported platform (GitLab) provides a concrete implementation of this interface.
type GitProvider interface {
	// ListRepositories returns all repositories accessible via the given connector configuration.
	// The connector supplies the base URL, credentials, and target org/group/project context.
	ListRepositories(ctx context.Context, connector *entity.Connector) ([]GitProviderRepository, error)

	// ValidateCredentials verifies that the credentials stored in the connector are valid and
	// that the provider URL is reachable.  It returns nil when credentials are accepted.
	ValidateCredentials(ctx context.Context, connector *entity.Connector) error
}

// GitProviderRepository describes a single repository returned by a GitProvider.
type GitProviderRepository struct {
	// Name is the short repository name (e.g. "my-service").
	Name string

	// URL is the clone URL for the repository.
	URL string

	// Description is an optional human-readable description.
	Description string

	// DefaultBranch is the primary branch (e.g. "main").
	DefaultBranch string

	// IsPrivate indicates whether the repository is private.
	IsPrivate bool

	// LastActivity is the timestamp of the most recent push/commit activity, when available.
	LastActivity *time.Time
}
