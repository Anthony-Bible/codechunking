package gitprovider

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// BitbucketProvider implements outbound.GitProvider for Bitbucket Cloud workspaces.
type BitbucketProvider struct {
	httpClient *http.Client
}

// NewBitbucketProvider creates a new BitbucketProvider.
func NewBitbucketProvider(httpClient *http.Client) *BitbucketProvider {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &BitbucketProvider{httpClient: httpClient}
}

// ListRepositories calls the Bitbucket Cloud API to list repositories in a workspace.
// The connector Name is used as the workspace slug.
func (p *BitbucketProvider) ListRepositories(
	ctx context.Context,
	connector *entity.Connector,
) ([]outbound.GitProviderRepository, error) {
	baseURL := connector.BaseURL()
	if baseURL == "" {
		baseURL = "https://api.bitbucket.org/2.0"
	}

	apiURL := fmt.Sprintf("%s/repositories/%s?pagelen=100", baseURL, connector.Name())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build bitbucket list-repos request: %w", err)
	}
	if connector.AuthToken() != nil && *connector.AuthToken() != "" {
		req.Header.Set("Authorization", "Bearer "+*connector.AuthToken())
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bitbucket list-repos request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkHTTPStatus(resp, "bitbucket list repos"); err != nil {
		return nil, err
	}

	var page bitbucketPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("decode bitbucket repos response: %w", err)
	}

	result := make([]outbound.GitProviderRepository, 0, len(page.Values))
	for _, r := range page.Values {
		cloneURL := ""
		for _, link := range r.Links.Clone {
			if link.Name == "https" {
				cloneURL = link.Href
				break
			}
		}

		updatedOn := r.UpdatedOn
		repo := outbound.GitProviderRepository{
			Name:          r.Name,
			URL:           cloneURL,
			Description:   r.Description,
			DefaultBranch: r.MainBranch.Name,
			IsPrivate:     r.IsPrivate,
			LastActivity:  updatedOn,
		}
		result = append(result, repo)
	}
	return result, nil
}

// ValidateCredentials verifies the token via the Bitbucket /user endpoint.
func (p *BitbucketProvider) ValidateCredentials(ctx context.Context, connector *entity.Connector) error {
	baseURL := connector.BaseURL()
	if baseURL == "" {
		baseURL = "https://api.bitbucket.org/2.0"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/user", nil)
	if err != nil {
		return fmt.Errorf("build bitbucket validate request: %w", err)
	}
	if connector.AuthToken() != nil && *connector.AuthToken() != "" {
		req.Header.Set("Authorization", "Bearer "+*connector.AuthToken())
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("bitbucket validate request: %w", err)
	}
	defer resp.Body.Close()
	return checkHTTPStatus(resp, "bitbucket validate credentials")
}

type bitbucketPage struct {
	Values []bitbucketRepository `json:"values"`
}

type bitbucketRepository struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	IsPrivate   bool              `json:"is_private"`
	MainBranch  bitbucketBranch   `json:"mainbranch"`
	UpdatedOn   *time.Time        `json:"updated_on"`
	Links       bitbucketLinks    `json:"links"`
}

type bitbucketBranch struct {
	Name string `json:"name"`
}

type bitbucketLinks struct {
	Clone []bitbucketCloneLink `json:"clone"`
}

type bitbucketCloneLink struct {
	Href string `json:"href"`
	Name string `json:"name"`
}
