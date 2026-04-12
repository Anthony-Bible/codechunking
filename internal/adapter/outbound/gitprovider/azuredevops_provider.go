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

// AzureDevOpsProvider implements outbound.GitProvider for Azure DevOps organisations.
type AzureDevOpsProvider struct {
	httpClient *http.Client
}

// NewAzureDevOpsProvider creates a new AzureDevOpsProvider.
func NewAzureDevOpsProvider(httpClient *http.Client) *AzureDevOpsProvider {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &AzureDevOpsProvider{httpClient: httpClient}
}

// ListRepositories calls the Azure DevOps REST API to list repositories.
// The connector Name is expected to be "org/project" (e.g. "myorg/myproject").
// BaseURL defaults to "https://dev.azure.com".
func (p *AzureDevOpsProvider) ListRepositories(
	ctx context.Context,
	connector *entity.Connector,
) ([]outbound.GitProviderRepository, error) {
	baseURL := connector.BaseURL()
	if baseURL == "" {
		baseURL = "https://dev.azure.com"
	}

	// Azure DevOps: GET /{org}/{project}/_apis/git/repositories
	apiURL := fmt.Sprintf("%s/%s/_apis/git/repositories?api-version=7.1", baseURL, url.PathEscape(connector.Name()))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build azure devops list-repos request: %w", err)
	}
	if connector.AuthToken() != nil && *connector.AuthToken() != "" {
		// Azure DevOps uses PAT tokens with Basic auth (user may be empty)
		req.SetBasicAuth("", *connector.AuthToken())
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure devops list-repos request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkHTTPStatus(resp, "azure devops list repos"); err != nil {
		return nil, err
	}

	var listResp azureRepoListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("decode azure devops repos response: %w", err)
	}

	result := make([]outbound.GitProviderRepository, 0, len(listResp.Value))
	for _, r := range listResp.Value {
		repo := outbound.GitProviderRepository{
			Name:          r.Name,
			URL:           r.RemoteURL,
			DefaultBranch: r.DefaultBranch,
			IsPrivate:     true, // Azure DevOps repos are always private unless explicitly shared
		}
		result = append(result, repo)
	}
	return result, nil
}

// ValidateCredentials verifies the PAT by calling the projects list endpoint.
func (p *AzureDevOpsProvider) ValidateCredentials(ctx context.Context, connector *entity.Connector) error {
	baseURL := connector.BaseURL()
	if baseURL == "" {
		baseURL = "https://dev.azure.com"
	}

	// Use the org name as the first path segment (connector.Name() may be "org/project")
	orgName := connector.Name()
	apiURL := fmt.Sprintf("%s/%s/_apis/projects?api-version=7.1&$top=1", baseURL, orgName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("build azure devops validate request: %w", err)
	}
	if connector.AuthToken() != nil && *connector.AuthToken() != "" {
		req.SetBasicAuth("", *connector.AuthToken())
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("azure devops validate request: %w", err)
	}
	defer resp.Body.Close()
	return checkHTTPStatus(resp, "azure devops validate credentials")
}

type azureRepoListResponse struct {
	Value []azureRepository `json:"value"`
	Count int               `json:"count"`
}

type azureRepository struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	RemoteURL     string `json:"remoteUrl"`
	DefaultBranch string `json:"defaultBranch"`
}
