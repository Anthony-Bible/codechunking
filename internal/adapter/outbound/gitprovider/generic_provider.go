package gitprovider

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// GenericGitProvider implements outbound.GitProvider for self-hosted or custom git servers
// that expose a JSON list endpoint compatible with the generic format.
//
// Expected response shape from the list endpoint:
//
//	[{"name":"repo","clone_url":"https://...","description":"...","default_branch":"main","private":false}]
type GenericGitProvider struct {
	httpClient *http.Client
}

// NewGenericGitProvider creates a new GenericGitProvider.
func NewGenericGitProvider(httpClient *http.Client) *GenericGitProvider {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &GenericGitProvider{httpClient: httpClient}
}

// ListRepositories calls the configured BaseURL to fetch a JSON array of repositories.
// It uses the connector's BaseURL as the API endpoint and connector.Name() as the org/group path.
func (p *GenericGitProvider) ListRepositories(
	ctx context.Context,
	connector *entity.Connector,
) ([]outbound.GitProviderRepository, error) {
	baseURL := connector.BaseURL()
	if baseURL == "" {
		return nil, fmt.Errorf("generic git provider: base_url is required")
	}

	apiURL := fmt.Sprintf("%s/%s", baseURL, url.PathEscape(connector.Name()))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build generic list-repos request: %w", err)
	}
	addBearerAuth(req, connector.AuthToken())

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("generic list-repos request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkHTTPStatus(resp, "generic list repos"); err != nil {
		return nil, err
	}

	var repos []genericRepository
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("decode generic repos response: %w", err)
	}

	result := make([]outbound.GitProviderRepository, 0, len(repos))
	for _, r := range repos {
		lastActivity := r.UpdatedAt
		repo := outbound.GitProviderRepository{
			Name:          r.Name,
			URL:           r.CloneURL,
			Description:   r.Description,
			DefaultBranch: r.DefaultBranch,
			IsPrivate:     r.Private,
			LastActivity:  lastActivity,
		}
		result = append(result, repo)
	}
	return result, nil
}

// ValidateCredentials verifies connectivity by performing a GET against the BaseURL.
func (p *GenericGitProvider) ValidateCredentials(ctx context.Context, connector *entity.Connector) error {
	baseURL := connector.BaseURL()
	if baseURL == "" {
		return fmt.Errorf("generic git provider: base_url is required for credential validation")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return fmt.Errorf("build generic validate request: %w", err)
	}
	addBearerAuth(req, connector.AuthToken())

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("generic validate request: %w", err)
	}
	defer resp.Body.Close()
	return checkHTTPStatus(resp, "generic validate credentials")
}

type genericRepository struct {
	Name          string     `json:"name"`
	CloneURL      string     `json:"clone_url"`
	Description   string     `json:"description"`
	DefaultBranch string     `json:"default_branch"`
	Private       bool       `json:"private"`
	UpdatedAt     *time.Time `json:"updated_at"`
}
