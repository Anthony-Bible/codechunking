// Package gitprovider contains adapters that implement the outbound.GitProvider port
// for each supported git hosting service.
package gitprovider

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GitHubProvider implements outbound.GitProvider for GitHub organisations.
type GitHubProvider struct {
	httpClient *http.Client
}

// NewGitHubProvider creates a new GitHubProvider.
// An optional *http.Client may be provided to override the default; pass nil to use the default.
func NewGitHubProvider(httpClient *http.Client) *GitHubProvider {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &GitHubProvider{httpClient: httpClient}
}

// ListRepositories calls the GitHub REST API to list repositories for the organisation
// stored in connector.Name().
func (p *GitHubProvider) ListRepositories(
	ctx context.Context,
	connector *entity.Connector,
) ([]outbound.GitProviderRepository, error) {
	baseURL := connector.BaseURL()
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}

	// GitHub organisations endpoint: GET /orgs/{org}/repos
	url := fmt.Sprintf("%s/orgs/%s/repos?per_page=100&type=all", baseURL, connector.Name())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build github list-repos request: %w", err)
	}
	p.addAuthHeader(req, connector)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github list-repos request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkHTTPStatus(resp, "github list repos"); err != nil {
		return nil, err
	}

	var ghRepos []githubRepository
	if err := json.NewDecoder(resp.Body).Decode(&ghRepos); err != nil {
		return nil, fmt.Errorf("decode github repos response: %w", err)
	}

	result := make([]outbound.GitProviderRepository, 0, len(ghRepos))
	for _, r := range ghRepos {
		pushedAt := r.PushedAt
		repo := outbound.GitProviderRepository{
			Name:          r.Name,
			URL:           r.CloneURL,
			Description:   r.Description,
			DefaultBranch: r.DefaultBranch,
			IsPrivate:     r.Private,
			LastActivity:  pushedAt,
		}
		result = append(result, repo)
	}
	return result, nil
}

// ValidateCredentials verifies the token by calling the /user endpoint.
func (p *GitHubProvider) ValidateCredentials(ctx context.Context, connector *entity.Connector) error {
	baseURL := connector.BaseURL()
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/user", nil)
	if err != nil {
		return fmt.Errorf("build github validate request: %w", err)
	}
	p.addAuthHeader(req, connector)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github validate request: %w", err)
	}
	defer resp.Body.Close()
	return checkHTTPStatus(resp, "github validate credentials")
}

func (p *GitHubProvider) addAuthHeader(req *http.Request, connector *entity.Connector) {
	if connector.AuthToken() != nil && *connector.AuthToken() != "" {
		req.Header.Set("Authorization", "Bearer "+*connector.AuthToken())
	}
}

// githubRepository is the minimal GitHub API repository shape we need.
type githubRepository struct {
	Name          string     `json:"name"`
	CloneURL      string     `json:"clone_url"`
	Description   string     `json:"description"`
	DefaultBranch string     `json:"default_branch"`
	Private       bool       `json:"private"`
	PushedAt      *time.Time `json:"pushed_at"`
}

// ---------------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------------

func checkHTTPStatus(resp *http.Response, operation string) error {
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("%s: unexpected status %d: %s", operation, resp.StatusCode, string(body))
}
