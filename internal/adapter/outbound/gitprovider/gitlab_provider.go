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

// GitLabProvider implements outbound.GitProvider for GitLab groups.
type GitLabProvider struct {
	httpClient *http.Client
}

// NewGitLabProvider creates a new GitLabProvider.
func NewGitLabProvider(httpClient *http.Client) *GitLabProvider {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &GitLabProvider{httpClient: httpClient}
}

// ListRepositories calls the GitLab API to list projects in the given group.
// The connector Name is used as the group path/ID.
func (p *GitLabProvider) ListRepositories(
	ctx context.Context,
	connector *entity.Connector,
) ([]outbound.GitProviderRepository, error) {
	baseURL := connector.BaseURL()
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}

	// GitLab groups/projects endpoint: GET /groups/{id}/projects
	groupID := url.PathEscape(connector.Name())
	apiURL := fmt.Sprintf("%s/api/v4/groups/%s/projects?per_page=100&include_subgroups=true", baseURL, groupID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build gitlab list-projects request: %w", err)
	}
	if connector.AuthToken() != nil && *connector.AuthToken() != "" {
		req.Header.Set("PRIVATE-TOKEN", *connector.AuthToken())
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab list-projects request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkHTTPStatus(resp, "gitlab list projects"); err != nil {
		return nil, err
	}

	var projects []gitlabProject
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, fmt.Errorf("decode gitlab projects response: %w", err)
	}

	result := make([]outbound.GitProviderRepository, 0, len(projects))
	for _, p := range projects {
		lastActivity := p.LastActivityAt
		repo := outbound.GitProviderRepository{
			Name:          p.Name,
			URL:           p.HTTPURLToRepo,
			Description:   p.Description,
			DefaultBranch: p.DefaultBranch,
			IsPrivate:     p.Visibility != "public",
			LastActivity:  lastActivity,
		}
		result = append(result, repo)
	}
	return result, nil
}

// ValidateCredentials verifies the token by calling the /user endpoint.
func (p *GitLabProvider) ValidateCredentials(ctx context.Context, connector *entity.Connector) error {
	baseURL := connector.BaseURL()
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v4/user", nil)
	if err != nil {
		return fmt.Errorf("build gitlab validate request: %w", err)
	}
	if connector.AuthToken() != nil && *connector.AuthToken() != "" {
		req.Header.Set("PRIVATE-TOKEN", *connector.AuthToken())
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitlab validate request: %w", err)
	}
	defer resp.Body.Close()
	return checkHTTPStatus(resp, "gitlab validate credentials")
}

type gitlabProject struct {
	Name           string     `json:"name"`
	HTTPURLToRepo  string     `json:"http_url_to_repo"`
	Description    string     `json:"description"`
	DefaultBranch  string     `json:"default_branch"`
	Visibility     string     `json:"visibility"`
	LastActivityAt *time.Time `json:"last_activity_at"`
}
