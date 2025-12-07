package client

import (
	"bytes"
	"codechunking/internal/application/dto"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"
)

const (
	// userAgent is the User-Agent header value sent with all API requests.
	userAgent = "codechunking-client/1.0"

	// contentTypeJSON is the Content-Type header value for JSON requests.
	contentTypeJSON = "application/json"

	// API endpoint paths.
	pathHealth       = "/health"
	pathSearch       = "/search"
	pathRepositories = "/repositories"
)

// Client provides methods for interacting with the CodeChunking API.
// It handles authentication, request serialization, and response parsing.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new API client with the given configuration.
// Returns an error if the configuration is nil or invalid.
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &Client{
		baseURL:    config.APIURL,
		httpClient: &http.Client{Timeout: config.Timeout},
	}, nil
}

// NewClientWithHTTPClient creates a new API client with the given configuration and HTTP client.
// If httpClient is nil, a default HTTP client with the configured timeout will be used.
// Returns an error if the configuration is nil or invalid.
func NewClientWithHTTPClient(config *Config, httpClient *http.Client) (*Client, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: config.Timeout}
	}

	return &Client{
		baseURL:    config.APIURL,
		httpClient: httpClient,
	}, nil
}

// doRequest performs an HTTP request with the given parameters and decodes the response.
// It handles request construction, header setup, error responses, and JSON decoding.
// If body is non-nil, it will be JSON-encoded and sent with Content-Type: application/json.
// If result is non-nil, the response body will be JSON-decoded into it.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	fullURL := c.baseURL + path

	var reqBody *bytes.Buffer
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to encode request: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	var req *http.Request
	var err error
	if reqBody != nil {
		req, err = http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, fullURL, nil)
	}
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", contentTypeJSON)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API request failed: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// Health performs a health check against the API server.
// Returns the health status and any error encountered.
func (c *Client) Health(ctx context.Context) (*dto.HealthResponse, error) {
	var result dto.HealthResponse
	if err := c.doRequest(ctx, http.MethodGet, pathHealth, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Search performs a semantic search for code chunks.
// Returns search results matching the query or an error.
func (c *Client) Search(ctx context.Context, req dto.SearchRequestDTO) (*dto.SearchResponseDTO, error) {
	var result dto.SearchResponseDTO
	if err := c.doRequest(ctx, http.MethodPost, pathSearch, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListRepositories retrieves a list of repositories with optional filtering and pagination.
// Returns the list of repositories and pagination metadata, or an error.
func (c *Client) ListRepositories(
	ctx context.Context,
	query dto.RepositoryListQuery,
) (*dto.RepositoryListResponse, error) {
	params := url.Values{}
	if query.Limit >= 0 {
		params.Add("limit", strconv.Itoa(query.Limit))
	}
	if query.Offset >= 0 {
		params.Add("offset", strconv.Itoa(query.Offset))
	}
	if query.Status != "" {
		params.Add("status", query.Status)
	}
	if query.Name != "" {
		params.Add("name", query.Name)
	}
	if query.URL != "" {
		params.Add("url", query.URL)
	}
	if query.Sort != "" {
		params.Add("sort", query.Sort)
	}

	path := pathRepositories
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var result dto.RepositoryListResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetRepository retrieves a single repository by ID.
// Returns the repository details or an error if not found.
func (c *Client) GetRepository(ctx context.Context, id uuid.UUID) (*dto.RepositoryResponse, error) {
	path := fmt.Sprintf("/repositories/%s", id.String())
	var result dto.RepositoryResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateRepository creates a new repository for indexing.
// Returns the created repository details or an error.
func (c *Client) CreateRepository(
	ctx context.Context,
	req dto.CreateRepositoryRequest,
) (*dto.RepositoryResponse, error) {
	var result dto.RepositoryResponse
	if err := c.doRequest(ctx, http.MethodPost, pathRepositories, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetIndexingJob retrieves the status of an indexing job for a repository.
// Returns the job status details or an error if not found.
func (c *Client) GetIndexingJob(ctx context.Context, repoID, jobID uuid.UUID) (*dto.IndexingJobResponse, error) {
	path := fmt.Sprintf("/repositories/%s/jobs/%s", repoID.String(), jobID.String())
	var result dto.IndexingJobResponse
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
