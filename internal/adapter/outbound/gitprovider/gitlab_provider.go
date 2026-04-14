package gitprovider

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// GitLabProvider implements outbound.GitProvider for GitLab instances.
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

// ListRepositories calls the GitLab API to list projects from all configured groups
// and individual projects defined on the connector.
// Results are deduplicated by HTTP clone URL.
func (p *GitLabProvider) ListRepositories(
	ctx context.Context,
	connector *entity.Connector,
) ([]outbound.GitProviderRepository, error) {
	baseURL := connector.BaseURL()
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}

	seen := make(map[string]struct{})
	var result []outbound.GitProviderRepository

	// List projects from each group.
	for _, group := range connector.Groups() {
		repos, err := p.listGroupProjects(ctx, baseURL, group, connector.AuthToken())
		if err != nil {
			return nil, fmt.Errorf("listing group %q: %w", group, err)
		}
		for _, repo := range repos {
			if _, dup := seen[repo.URL]; dup {
				continue
			}
			seen[repo.URL] = struct{}{}
			result = append(result, repo)
		}
	}

	// Fetch individually specified projects.
	for _, project := range connector.Projects() {
		repo, err := p.getProject(ctx, baseURL, project, connector.AuthToken())
		if err != nil {
			return nil, fmt.Errorf("fetching project %q: %w", project, err)
		}
		if _, dup := seen[repo.URL]; dup {
			continue
		}
		seen[repo.URL] = struct{}{}
		result = append(result, *repo)
	}

	return result, nil
}

// listGroupProjects calls GET /groups/{id}/projects and pages through all results.
func (p *GitLabProvider) listGroupProjects(
	ctx context.Context,
	baseURL, groupPath string,
	authToken *string,
) ([]outbound.GitProviderRepository, error) {
	// url.PathEscape intentionally leaves '/' unencoded (valid in path segments),
	// but GitLab treats the entire group path as a single opaque ID, so slashes
	// must be percent-encoded as %2F to avoid being interpreted as path separators.
	groupID := strings.ReplaceAll(url.PathEscape(groupPath), "/", "%2F")
	page := 1
	var result []outbound.GitProviderRepository

	for {
		apiURL := fmt.Sprintf("%s/api/v4/groups/%s/projects?per_page=100&include_subgroups=true&page=%d", baseURL, groupID, page)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("build list-group-projects request: %w", err)
		}
		setAuthHeader(req, authToken)

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("list-group-projects request: %w", err)
		}

		if err := checkHTTPStatus(resp, "gitlab list group projects"); err != nil {
			_ = resp.Body.Close()
			return nil, err
		}

		var projects []gitlabProject
		decodeErr := json.NewDecoder(resp.Body).Decode(&projects)
		_ = resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode gitlab projects response: %w", decodeErr)
		}

		for _, proj := range projects {
			result = append(result, gitlabProjectToRepo(proj))
		}

		nextPage := resp.Header.Get("X-Next-Page")
		if nextPage == "" {
			break
		}
		if n, err := strconv.Atoi(nextPage); err == nil {
			page = n
		} else {
			break
		}
	}

	return result, nil
}

// getProject calls GET /projects/{id} (id may be URL-encoded namespace/project).
func (p *GitLabProvider) getProject(
	ctx context.Context,
	baseURL, projectPath string,
	authToken *string,
) (*outbound.GitProviderRepository, error) {
	projectID := url.PathEscape(projectPath)
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s", baseURL, projectID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build get-project request: %w", err)
	}
	setAuthHeader(req, authToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get-project request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkHTTPStatus(resp, "gitlab get project"); err != nil {
		return nil, err
	}

	var project gitlabProject
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("decode gitlab project response: %w", err)
	}

	repo := gitlabProjectToRepo(project)
	return &repo, nil
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
	setAuthHeader(req, connector.AuthToken())

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitlab validate request: %w", err)
	}
	defer resp.Body.Close()
	return checkHTTPStatus(resp, "gitlab validate credentials")
}

// setAuthHeader sets the Private-Token header when a non-empty token is provided.
func setAuthHeader(req *http.Request, authToken *string) {
	if authToken != nil && *authToken != "" {
		req.Header.Set("Private-Token", *authToken)
	}
}

// gitlabProjectToRepo converts a GitLab API project to a GitProviderRepository.
func gitlabProjectToRepo(p gitlabProject) outbound.GitProviderRepository {
	return outbound.GitProviderRepository{
		Name:          p.Name,
		URL:           p.HTTPURLToRepo,
		Description:   p.Description,
		DefaultBranch: p.DefaultBranch,
		IsPrivate:     p.Visibility != "public",
		LastActivity:  p.LastActivityAt,
	}
}

type gitlabProject struct {
	Name           string     `json:"name"`
	HTTPURLToRepo  string     `json:"http_url_to_repo"`
	Description    string     `json:"description"`
	DefaultBranch  string     `json:"default_branch"`
	Visibility     string     `json:"visibility"`
	LastActivityAt *time.Time `json:"last_activity_at"`
}
