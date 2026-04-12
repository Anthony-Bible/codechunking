package gitprovider

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestConnector builds a minimal Connector pointed at the given base URL with an optional auth token.
func newTestConnector(t *testing.T, baseURL string, authToken *string, groups []string) *entity.Connector {
	t.Helper()
	connector, err := entity.NewConnector(
		"test-connector",
		valueobject.ConnectorTypeGitLab,
		baseURL,
		authToken,
		groups,
		nil,
	)
	require.NoError(t, err)
	return connector
}

// mustToken is a convenience that returns a pointer to the given token string.
func mustToken(s string) *string { return &s }

// encodeJSON serialises v to JSON and fails the test on error.
func encodeJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// ---- helpers for building stub GitLab API project payloads ----

type stubProject struct {
	Name           string     `json:"name"`
	HTTPURLToRepo  string     `json:"http_url_to_repo"`
	Description    string     `json:"description"`
	DefaultBranch  string     `json:"default_branch"`
	Visibility     string     `json:"visibility"`
	LastActivityAt *time.Time `json:"last_activity_at"`
}

func publicProject(name, cloneURL string) stubProject {
	return stubProject{
		Name:          name,
		HTTPURLToRepo: cloneURL,
		Description:   name + " description",
		DefaultBranch: "main",
		Visibility:    "public",
	}
}

func privateProject(name, cloneURL string) stubProject {
	return stubProject{
		Name:          name,
		HTTPURLToRepo: cloneURL,
		Description:   name + " description",
		DefaultBranch: "main",
		Visibility:    "private",
	}
}

// ---- tests for listGroupProjects via ListRepositories ----

// TestListGroupProjects_SinglePage verifies that when the GitLab API returns a single
// page of projects (no X-Next-Page header), all projects on that page are returned and
// mapped to GitProviderRepository with correct field values.
func TestListGroupProjects_SinglePage(t *testing.T) {
	projects := []stubProject{
		publicProject("repo-alpha", "https://gitlab.example.com/my-group/repo-alpha.git"),
		privateProject("repo-beta", "https://gitlab.example.com/my-group/repo-beta.git"),
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Deliberately omit X-Next-Page to signal a single page.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(encodeJSON(t, projects))
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, nil, []string{"my-group"})

	got, err := provider.ListRepositories(context.Background(), connector)

	require.NoError(t, err)
	require.Len(t, got, 2, "expected exactly two repositories from a single-page response")

	assert.Equal(t, "repo-alpha", got[0].Name)
	assert.Equal(t, "https://gitlab.example.com/my-group/repo-alpha.git", got[0].URL)
	assert.Equal(t, "repo-alpha description", got[0].Description)
	assert.Equal(t, "main", got[0].DefaultBranch)
	assert.False(t, got[0].IsPrivate, "public project must not be flagged as private")

	assert.Equal(t, "repo-beta", got[1].Name)
	assert.Equal(t, "https://gitlab.example.com/my-group/repo-beta.git", got[1].URL)
	assert.True(t, got[1].IsPrivate, "private project must be flagged as private")
}

// TestListGroupProjects_MultiplePages verifies that when the first response carries an
// X-Next-Page header the provider fetches subsequent pages and returns the combined set.
func TestListGroupProjects_MultiplePages(t *testing.T) {
	page1 := []stubProject{
		publicProject("repo-page1-a", "https://gitlab.example.com/grp/repo-page1-a.git"),
		publicProject("repo-page1-b", "https://gitlab.example.com/grp/repo-page1-b.git"),
	}
	page2 := []stubProject{
		publicProject("repo-page2-a", "https://gitlab.example.com/grp/repo-page2-a.git"),
	}

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		switch callCount {
		case 1:
			// First page: signal that page 2 exists.
			w.Header().Set("X-Next-Page", "2")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(encodeJSON(t, page1))
		default:
			// Second (and last) page: no X-Next-Page header.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(encodeJSON(t, page2))
		}
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, nil, []string{"grp"})

	got, err := provider.ListRepositories(context.Background(), connector)

	require.NoError(t, err)
	require.Len(t, got, 3, "all three projects across both pages must be returned")
	assert.Equal(t, 2, callCount, "provider must issue exactly two HTTP requests for two pages")

	names := make([]string, len(got))
	for i, r := range got {
		names[i] = r.Name
	}
	assert.Contains(t, names, "repo-page1-a")
	assert.Contains(t, names, "repo-page1-b")
	assert.Contains(t, names, "repo-page2-a")
}

// TestListGroupProjects_ThreePages verifies pagination across more than two pages so
// that the loop termination is driven solely by an empty X-Next-Page, not a counter.
func TestListGroupProjects_ThreePages(t *testing.T) {
	pages := [][]stubProject{
		{publicProject("p1", "https://gitlab.example.com/g/p1.git")},
		{publicProject("p2", "https://gitlab.example.com/g/p2.git")},
		{publicProject("p3", "https://gitlab.example.com/g/p3.git")},
	}

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := callCount
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if idx < len(pages)-1 {
			w.Header().Set("X-Next-Page", "next")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(encodeJSON(t, pages[idx]))
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, nil, []string{"g"})

	got, err := provider.ListRepositories(context.Background(), connector)

	require.NoError(t, err)
	assert.Len(t, got, 3)
	assert.Equal(t, 3, callCount, "provider must issue one request per page")
}

// TestListGroupProjects_HTTPErrorReturnsError verifies that a non-2xx response (e.g. 401
// Unauthorized) causes ListRepositories to propagate an error rather than return repos.
func TestListGroupProjects_HTTPErrorReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"401 Unauthorized"}`))
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, nil, []string{"my-group"})

	got, err := provider.ListRepositories(context.Background(), connector)

	require.Error(t, err, "a 401 response must produce an error")
	assert.Nil(t, got, "no repositories must be returned on error")
}

// TestListGroupProjects_ForbiddenReturnsError verifies that a 403 response is treated
// as an error, distinguishing it from the 401 case.
func TestListGroupProjects_ForbiddenReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, nil, []string{"restricted-group"})

	_, err := provider.ListRepositories(context.Background(), connector)

	require.Error(t, err, "a 403 response must produce an error")
}

// TestListGroupProjects_InvalidJSONReturnsError verifies that a 200 OK response whose
// body is not valid JSON causes ListRepositories to return an error.
func TestListGroupProjects_InvalidJSONReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not valid json [`))
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, nil, []string{"my-group"})

	got, err := provider.ListRepositories(context.Background(), connector)

	require.Error(t, err, "malformed JSON in the response body must produce an error")
	assert.Nil(t, got)
}

// TestListGroupProjects_AuthTokenForwardedAsPrivateToken verifies that when a connector
// carries an auth token the provider sets the Private-Token request header on every
// request it makes to the GitLab API.
func TestListGroupProjects_AuthTokenForwardedAsPrivateToken(t *testing.T) {
	const expectedToken = "glpat-super-secret-token"
	receivedTokens := []string{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTokens = append(receivedTokens, r.Header.Get("Private-Token"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(encodeJSON(t, []stubProject{
			publicProject("secured-repo", "https://gitlab.example.com/grp/secured-repo.git"),
		}))
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, mustToken(expectedToken), []string{"grp"})

	_, err := provider.ListRepositories(context.Background(), connector)

	require.NoError(t, err)
	require.NotEmpty(t, receivedTokens, "at least one request must have been made")
	for i, tok := range receivedTokens {
		assert.Equal(t, expectedToken, tok,
			"request %d must carry the correct Private-Token header", i+1)
	}
}

// TestListGroupProjects_AuthTokenForwardedOnEveryPage verifies that the Private-Token
// header is sent on all paginated requests, not just the first.
func TestListGroupProjects_AuthTokenForwardedOnEveryPage(t *testing.T) {
	const expectedToken = "glpat-multi-page-token"
	receivedTokens := []string{}

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		receivedTokens = append(receivedTokens, r.Header.Get("Private-Token"))
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			w.Header().Set("X-Next-Page", "2")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(encodeJSON(t, []stubProject{
			publicProject("repo", "https://gitlab.example.com/grp/repo.git"),
		}))
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, mustToken(expectedToken), []string{"grp"})

	_, err := provider.ListRepositories(context.Background(), connector)

	require.NoError(t, err)
	require.Len(t, receivedTokens, 2, "expected two requests (one per page)")
	assert.Equal(t, expectedToken, receivedTokens[0], "page 1 request must carry Private-Token")
	assert.Equal(t, expectedToken, receivedTokens[1], "page 2 request must carry Private-Token")
}

// TestListGroupProjects_NoAuthTokenOmitsPrivateTokenHeader verifies that when the
// connector has no auth token the Private-Token header is absent from requests.
func TestListGroupProjects_NoAuthTokenOmitsPrivateTokenHeader(t *testing.T) {
	var capturedToken string
	var tokenHeaderPresent bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedToken = r.Header.Get("Private-Token")
		_, tokenHeaderPresent = r.Header["Private-Token"]
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(encodeJSON(t, []stubProject{}))
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, nil, []string{"public-group"})

	_, err := provider.ListRepositories(context.Background(), connector)

	require.NoError(t, err)
	assert.False(t, tokenHeaderPresent,
		"Private-Token header must be absent when no auth token is configured; got %q", capturedToken)
}

// TestListGroupProjects_EmptyGroupReturnsEmptySlice verifies that a 200 OK with an
// empty JSON array yields an empty (not nil) result without error.
func TestListGroupProjects_EmptyGroupReturnsEmptySlice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, nil, []string{"empty-group"})

	got, err := provider.ListRepositories(context.Background(), connector)

	require.NoError(t, err)
	assert.Empty(t, got, "an empty projects list must yield no repositories")
}

// TestListGroupProjects_IsPrivateMapping verifies that the IsPrivate field is derived
// solely from the visibility field: only "public" visibility results in IsPrivate=false;
// any other value (private, internal) maps to IsPrivate=true.
func TestListGroupProjects_IsPrivateMapping(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	projects := []stubProject{
		{Name: "pub", HTTPURLToRepo: "https://gl.example.com/g/pub.git", Visibility: "public", DefaultBranch: "main"},
		{Name: "priv", HTTPURLToRepo: "https://gl.example.com/g/priv.git", Visibility: "private", DefaultBranch: "main"},
		{Name: "internal", HTTPURLToRepo: "https://gl.example.com/g/internal.git", Visibility: "internal", DefaultBranch: "main"},
		{Name: "with-activity", HTTPURLToRepo: "https://gl.example.com/g/act.git", Visibility: "public", DefaultBranch: "develop", LastActivityAt: &now},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(encodeJSON(t, projects))
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, nil, []string{"g"})

	got, err := provider.ListRepositories(context.Background(), connector)

	require.NoError(t, err)
	require.Len(t, got, 4)

	byName := make(map[string]bool, len(got))
	for _, r := range got {
		byName[r.Name] = r.IsPrivate
	}

	assert.False(t, byName["pub"], "public visibility must map to IsPrivate=false")
	assert.True(t, byName["priv"], "private visibility must map to IsPrivate=true")
	assert.True(t, byName["internal"], "internal visibility must map to IsPrivate=true")

	// Also verify LastActivity is propagated correctly.
	for _, r := range got {
		if r.Name == "with-activity" {
			require.NotNil(t, r.LastActivity)
			assert.Equal(t, now, r.LastActivity.UTC().Truncate(time.Second))
		}
	}
}

// TestListGroupProjects_PaginationRequestsCorrectPageQueryParam verifies that the
// provider increments the page query parameter on each request so the API can return
// distinct pages.
func TestListGroupProjects_PaginationRequestsCorrectPageQueryParam(t *testing.T) {
	receivedPages := []string{}

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPages = append(receivedPages, r.URL.Query().Get("page"))
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			w.Header().Set("X-Next-Page", "2")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(encodeJSON(t, []stubProject{
			publicProject("r", "https://gl.example.com/g/r.git"),
		}))
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, nil, []string{"g"})

	_, err := provider.ListRepositories(context.Background(), connector)

	require.NoError(t, err)
	require.Len(t, receivedPages, 2)
	assert.Equal(t, "1", receivedPages[0], "first request must use page=1")
	assert.Equal(t, "2", receivedPages[1], "second request must use page=2")
}

// TestListGroupProjects_PaginationRequestsPerPage100 verifies that every request
// uses per_page=100 as required by the API contract.
func TestListGroupProjects_PaginationRequestsPerPage100(t *testing.T) {
	receivedPerPage := []string{}

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPerPage = append(receivedPerPage, r.URL.Query().Get("per_page"))
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			w.Header().Set("X-Next-Page", "2")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(encodeJSON(t, []stubProject{
			publicProject("r", "https://gl.example.com/g/r.git"),
		}))
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, nil, []string{"g"})

	_, err := provider.ListRepositories(context.Background(), connector)

	require.NoError(t, err)
	for i, pp := range receivedPerPage {
		assert.Equal(t, "100", pp, "request %d must use per_page=100", i+1)
	}
}

// TestListGroupProjects_PaginationRequestsIncludeSubgroups verifies that every request
// includes include_subgroups=true in the query string.
func TestListGroupProjects_PaginationRequestsIncludeSubgroups(t *testing.T) {
	receivedIncludeSubgroups := []string{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedIncludeSubgroups = append(receivedIncludeSubgroups, r.URL.Query().Get("include_subgroups"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(encodeJSON(t, []stubProject{}))
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, nil, []string{"g"})

	_, err := provider.ListRepositories(context.Background(), connector)

	require.NoError(t, err)
	require.NotEmpty(t, receivedIncludeSubgroups)
	for i, val := range receivedIncludeSubgroups {
		assert.Equal(t, "true", val, "request %d must include include_subgroups=true", i+1)
	}
}

// TestListGroupProjects_ContextCancellationReturnsError verifies that when the context
// is already cancelled before the call the provider surfaces an error rather than
// hanging or returning stale data.
func TestListGroupProjects_ContextCancellationReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(encodeJSON(t, []stubProject{}))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before making the call

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, nil, []string{"g"})

	_, err := provider.ListRepositories(ctx, connector)

	require.Error(t, err, "a pre-cancelled context must produce an error")
}

// TestListGroupProjects_GroupPathIsURLEncoded verifies that a group path containing a
// slash (namespace/subgroup) is percent-encoded in the request URL so that GitLab
// identifies it as a single path-escaped ID.
func TestListGroupProjects_GroupPathIsURLEncoded(t *testing.T) {
	receivedURIs := []string{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.RequestURI preserves raw percent-encoding; r.URL.Path is always decoded by Go's HTTP server.
		receivedURIs = append(receivedURIs, r.RequestURI)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(encodeJSON(t, []stubProject{}))
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, nil, []string{"my-namespace/my-subgroup"})

	_, err := provider.ListRepositories(context.Background(), connector)

	require.NoError(t, err)
	require.NotEmpty(t, receivedURIs)
	// The group path slash must be percent-encoded as %2F in the URL path segment.
	assert.Contains(t, receivedURIs[0], "my-namespace%2Fmy-subgroup",
		"group path with slash must be URL-encoded in the request path; got %s", receivedURIs[0])
}

// TestListGroupProjects_MultipleGroupsAggregated verifies that when a connector has
// multiple groups, repositories from each group are all returned in a single result set.
func TestListGroupProjects_MultipleGroupsAggregated(t *testing.T) {
	groupResponses := map[string][]stubProject{
		"group-a": {publicProject("a1", "https://gl.example.com/group-a/a1.git")},
		"group-b": {publicProject("b1", "https://gl.example.com/group-b/b1.git")},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if contains(r.URL.Path, "group-a") {
			_, _ = w.Write(encodeJSON(t, groupResponses["group-a"]))
		} else {
			_, _ = w.Write(encodeJSON(t, groupResponses["group-b"]))
		}
	}))
	defer srv.Close()

	provider := NewGitLabProvider(srv.Client())
	connector := newTestConnector(t, srv.URL, nil, []string{"group-a", "group-b"})

	got, err := provider.ListRepositories(context.Background(), connector)

	require.NoError(t, err)
	assert.Len(t, got, 2, "repos from both groups must be combined into a single result")

	names := make([]string, len(got))
	for i, r := range got {
		names[i] = r.Name
	}
	assert.Contains(t, names, "a1")
	assert.Contains(t, names, "b1")
}

// contains is a simple substring check used as a routing aid in test handlers.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && stringContains(s, substr))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
