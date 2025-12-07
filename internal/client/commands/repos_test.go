package commands_test

import (
	"bytes"
	"codechunking/internal/application/dto"
	"codechunking/internal/client"
	"codechunking/internal/client/commands"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewReposCmd verifies that NewReposCmd creates a repos parent command with correct metadata.
// This test ensures the repos command is properly initialized with Use and Short description.
func TestNewReposCmd(t *testing.T) {
	t.Parallel()

	cmd := commands.NewReposCmd()

	require.NotNil(t, cmd, "NewReposCmd should return a non-nil command")
	assert.Equal(t, "repos", cmd.Use, "repos command Use should be 'repos'")
	assert.NotEmpty(t, cmd.Short, "repos command Short description should not be empty")
	assert.Contains(t, cmd.Short, "repositor", "Short description should mention repositories")
}

// TestReposCmd_HasSubcommands verifies the repos command has all required subcommands.
// This test ensures list, get, and add subcommands are registered.
func TestReposCmd_HasSubcommands(t *testing.T) {
	t.Parallel()

	cmd := commands.NewReposCmd()

	subcommands := cmd.Commands()
	require.NotEmpty(t, subcommands, "repos command should have subcommands")

	// Collect subcommand names
	subcommandNames := make(map[string]bool)
	for _, subcmd := range subcommands {
		subcommandNames[subcmd.Use] = true
	}

	// Verify required subcommands exist
	assert.True(t, subcommandNames["list"], "repos should have 'list' subcommand")
	assert.True(t, subcommandNames["get"], "repos should have 'get' subcommand")
	assert.True(t, subcommandNames["add"], "repos should have 'add' subcommand")
}

// TestReposCmd_RegisteredWithRoot verifies repos command is registered with root command.
// This test ensures the repos command is accessible from the root CLI.
func TestReposCmd_RegisteredWithRoot(t *testing.T) {
	t.Parallel()

	rootCmd := commands.NewRootCmd()

	// Find repos command
	var reposCmd *bool
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "repos" {
			found := true
			reposCmd = &found
			break
		}
	}

	require.NotNil(t, reposCmd, "repos command should be registered with root command")
}

// TestReposListCmd_Success verifies the repos list command outputs success JSON.
// This test ensures proper integration with the API client and correct JSON formatting.
func TestReposListCmd_Success(t *testing.T) {
	t.Parallel()

	// Create mock server that returns repository list
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repositories", r.URL.Path, "should request /repositories endpoint")
		assert.Equal(t, http.MethodGet, r.Method, "should use GET method")

		description := "Test repository description"
		branch := "main"
		lastCommit := "abc123"
		lastIndexed := time.Now()

		response := dto.RepositoryListResponse{
			Repositories: []dto.RepositoryResponse{
				{
					ID:             uuid.New(),
					URL:            "https://github.com/user/repo1",
					Name:           "user/repo1",
					Description:    &description,
					DefaultBranch:  &branch,
					LastIndexedAt:  &lastIndexed,
					LastCommitHash: &lastCommit,
					TotalFiles:     100,
					TotalChunks:    500,
					Status:         "completed",
					CreatedAt:      time.Now(),
					UpdatedAt:      time.Now(),
				},
			},
			Pagination: dto.PaginationResponse{
				Limit:   20,
				Offset:  0,
				Total:   1,
				HasMore: false,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	// Create command with custom output writer
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"repos", "list", "--api-url", server.URL})

	// Execute command
	err := rootCmd.Execute()
	require.NoError(t, err, "repos list command should execute without error")

	// Parse and verify JSON output
	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON")

	// Verify response envelope
	assert.True(t, response.Success, "response should indicate success")
	assert.Nil(t, response.Error, "response should not contain error")
	assert.NotZero(t, response.Timestamp, "response should have timestamp")

	// Verify repository list data
	require.NotNil(t, response.Data, "response should contain data")
	dataJSON, err := json.Marshal(response.Data)
	require.NoError(t, err)

	var listData dto.RepositoryListResponse
	err = json.Unmarshal(dataJSON, &listData)
	require.NoError(t, err, "data should be a valid RepositoryListResponse")
	assert.Len(t, listData.Repositories, 1, "should have 1 repository")
	assert.Equal(t, "user/repo1", listData.Repositories[0].Name)
	assert.Equal(t, "completed", listData.Repositories[0].Status)
	assert.Equal(t, 20, listData.Pagination.Limit)
	assert.Equal(t, 0, listData.Pagination.Offset)
	assert.Equal(t, 1, listData.Pagination.Total)
	assert.False(t, listData.Pagination.HasMore)
}

// TestReposListCmd_WithFilters verifies the repos list command applies filter flags correctly.
// This test ensures --limit, --status, --name, and other query parameters are properly sent.
func TestReposListCmd_WithFilters(t *testing.T) {
	t.Parallel()

	// Create mock server that captures query parameters
	var receivedParams map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedParams = make(map[string]string)
		query := r.URL.Query()
		for key := range query {
			receivedParams[key] = query.Get(key)
		}

		response := dto.RepositoryListResponse{
			Repositories: []dto.RepositoryResponse{},
			Pagination: dto.PaginationResponse{
				Limit:   10,
				Offset:  5,
				Total:   0,
				HasMore: false,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	// Create command with filters
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{
		"repos", "list",
		"--api-url", server.URL,
		"--limit", "10",
		"--offset", "5",
		"--status", "completed",
		"--name", "test-repo",
		"--sort", "name:asc",
	})

	// Execute command
	err := rootCmd.Execute()
	require.NoError(t, err, "repos list command should execute without error")

	// Verify query parameters were sent
	assert.Equal(t, "10", receivedParams["limit"], "limit should be 10")
	assert.Equal(t, "5", receivedParams["offset"], "offset should be 5")
	assert.Equal(t, "completed", receivedParams["status"], "status should be completed")
	assert.Equal(t, "test-repo", receivedParams["name"], "name should be test-repo")
	assert.Equal(t, "name:asc", receivedParams["sort"], "sort should be name:asc")

	// Verify JSON output
	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON")
	assert.True(t, response.Success, "response should indicate success")
}

// TestReposListCmd_EmptyList verifies the repos list command handles empty results gracefully.
// This test ensures proper JSON formatting when no repositories match the query.
func TestReposListCmd_EmptyList(t *testing.T) {
	t.Parallel()

	// Create mock server that returns empty list
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.RepositoryListResponse{
			Repositories: []dto.RepositoryResponse{},
			Pagination: dto.PaginationResponse{
				Limit:   20,
				Offset:  0,
				Total:   0,
				HasMore: false,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	// Create command
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"repos", "list", "--api-url", server.URL})

	// Execute command
	err := rootCmd.Execute()
	require.NoError(t, err, "repos list command should execute without error")

	// Parse and verify JSON output
	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON")

	// Verify response
	assert.True(t, response.Success, "response should indicate success")
	dataJSON, _ := json.Marshal(response.Data)
	var listData dto.RepositoryListResponse
	json.Unmarshal(dataJSON, &listData)
	assert.Empty(t, listData.Repositories, "repositories should be empty")
	assert.Equal(t, 0, listData.Pagination.Total, "total should be 0")
}

// TestReposListCmd_Pagination verifies pagination information is included in the response.
// This test ensures HasMore, Limit, Offset, and Total fields are properly returned.
func TestReposListCmd_Pagination(t *testing.T) {
	t.Parallel()

	// Create mock server with pagination
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.RepositoryListResponse{
			Repositories: make([]dto.RepositoryResponse, 10),
			Pagination: dto.PaginationResponse{
				Limit:   10,
				Offset:  0,
				Total:   25,
				HasMore: true,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	// Create command
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"repos", "list", "--api-url", server.URL, "--limit", "10"})

	// Execute command
	err := rootCmd.Execute()
	require.NoError(t, err)

	// Parse and verify pagination
	var response client.Response
	json.Unmarshal(stdout.Bytes(), &response)
	dataJSON, _ := json.Marshal(response.Data)
	var listData dto.RepositoryListResponse
	json.Unmarshal(dataJSON, &listData)

	assert.Equal(t, 10, listData.Pagination.Limit, "limit should be 10")
	assert.Equal(t, 0, listData.Pagination.Offset, "offset should be 0")
	assert.Equal(t, 25, listData.Pagination.Total, "total should be 25")
	assert.True(t, listData.Pagination.HasMore, "has_more should be true")
}

// TestReposListCmd_ServerError verifies error JSON output when server returns an error.
// This test ensures proper error handling and JSON error formatting for list command.
func TestReposListCmd_ServerError(t *testing.T) {
	t.Parallel()

	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	// Create command
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"repos", "list", "--api-url", server.URL})

	// Execute command
	_ = rootCmd.Execute()

	// Parse and verify JSON error output
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on error")

	assert.False(t, response.Success, "response should indicate failure")
	assert.Nil(t, response.Data, "response should not contain data on error")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotEmpty(t, response.Error.Code, "error should have a code")
	assert.Contains(t, response.Error.Message, "500", "error message should mention status code")
}

// TestReposGetCmd_Success verifies the repos get command retrieves a repository by ID.
// This test ensures proper integration with the API client for single repository retrieval.
func TestReposGetCmd_Success(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	description := "Test repository"
	branch := "main"
	lastCommit := "abc123"
	lastIndexed := time.Now()

	// Create mock server that returns single repository
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/repositories/" + repoID.String()
		assert.Equal(t, expectedPath, r.URL.Path, "should request correct repository path")
		assert.Equal(t, http.MethodGet, r.Method, "should use GET method")

		response := dto.RepositoryResponse{
			ID:             repoID,
			URL:            "https://github.com/user/repo",
			Name:           "user/repo",
			Description:    &description,
			DefaultBranch:  &branch,
			LastIndexedAt:  &lastIndexed,
			LastCommitHash: &lastCommit,
			TotalFiles:     100,
			TotalChunks:    500,
			Status:         "completed",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	// Create command
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"repos", "get", repoID.String(), "--api-url", server.URL})

	// Execute command
	err := rootCmd.Execute()
	require.NoError(t, err, "repos get command should execute without error")

	// Parse and verify JSON output
	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON")

	// Verify response
	assert.True(t, response.Success, "response should indicate success")
	assert.Nil(t, response.Error, "response should not contain error")

	// Verify repository data
	dataJSON, _ := json.Marshal(response.Data)
	var repoData dto.RepositoryResponse
	json.Unmarshal(dataJSON, &repoData)
	assert.Equal(t, repoID, repoData.ID, "ID should match")
	assert.Equal(t, "user/repo", repoData.Name, "name should match")
	assert.Equal(t, "completed", repoData.Status, "status should be completed")
	assert.Equal(t, 100, repoData.TotalFiles, "total files should be 100")
	assert.Equal(t, 500, repoData.TotalChunks, "total chunks should be 500")
}

// TestReposGetCmd_NotFound verifies the repos get command handles 404 errors.
// This test ensures proper error JSON output when repository is not found.
func TestReposGetCmd_NotFound(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()

	// Create mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Repository not found"))
	}))
	defer server.Close()

	// Create command
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"repos", "get", repoID.String(), "--api-url", server.URL})

	// Execute command
	_ = rootCmd.Execute()

	// Parse and verify JSON error output
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on error")

	assert.False(t, response.Success, "response should indicate failure")
	assert.Nil(t, response.Data, "response should not contain data on error")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotEmpty(t, response.Error.Code, "error should have a code")
	assert.Contains(t, response.Error.Message, "404", "error message should mention 404")
}

// TestReposGetCmd_InvalidUUID verifies the repos get command rejects invalid UUID arguments.
// This test ensures validation of UUID format before making API calls.
func TestReposGetCmd_InvalidUUID(t *testing.T) {
	t.Parallel()

	// Create command with invalid UUID
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"repos", "get", "not-a-valid-uuid", "--api-url", "http://localhost:8080"})

	// Execute command
	_ = rootCmd.Execute()

	// Parse and verify JSON error output
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on validation error")

	assert.False(t, response.Success, "response should indicate failure")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotEmpty(t, response.Error.Code, "error should have a code")
	assert.Contains(t, response.Error.Message, "UUID", "error message should mention UUID")
}

// TestReposGetCmd_MissingArg verifies the repos get command requires an ID argument.
// This test ensures the command fails with proper error when ID is not provided.
func TestReposGetCmd_MissingArg(t *testing.T) {
	t.Parallel()

	// Create command without ID argument
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"repos", "get", "--api-url", "http://localhost:8080"})

	// Execute command
	_ = rootCmd.Execute()

	// Parse and verify JSON error output
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on missing argument")

	assert.False(t, response.Success, "response should indicate failure")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotEmpty(t, response.Error.Code, "error should have a code")
	assert.Contains(t, response.Error.Message, "require", "error message should mention required argument")
}

// TestReposAddCmd_Success verifies the repos add command creates a new repository.
// This test ensures proper integration with the API client for repository creation.
func TestReposAddCmd_Success(t *testing.T) {
	t.Parallel()

	repoURL := "https://github.com/user/newrepo"
	description := "New repository"
	branch := "main"

	// Create mock server that handles repository creation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repositories", r.URL.Path, "should request /repositories endpoint")
		assert.Equal(t, http.MethodPost, r.Method, "should use POST method")

		// Decode request body
		var req dto.CreateRepositoryRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err, "should decode request body")
		assert.Equal(t, repoURL, req.URL, "URL should match")

		// Return created repository
		response := dto.RepositoryResponse{
			ID:            uuid.New(),
			URL:           repoURL,
			Name:          "user/newrepo",
			Description:   &description,
			DefaultBranch: &branch,
			Status:        "pending",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	// Create command
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"repos", "add", repoURL, "--api-url", server.URL})

	// Execute command
	err := rootCmd.Execute()
	require.NoError(t, err, "repos add command should execute without error")

	// Parse and verify JSON output
	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON")

	// Verify response
	assert.True(t, response.Success, "response should indicate success")
	assert.Nil(t, response.Error, "response should not contain error")

	// Verify repository data
	dataJSON, _ := json.Marshal(response.Data)
	var repoData dto.RepositoryResponse
	json.Unmarshal(dataJSON, &repoData)
	assert.Equal(t, "user/newrepo", repoData.Name)
	assert.Equal(t, repoURL, repoData.URL)
	assert.Equal(t, "pending", repoData.Status)
}

// TestReposAddCmd_WithOptionalFlags verifies the repos add command uses optional flags.
// This test ensures --name, --description, and --branch flags are properly sent.
func TestReposAddCmd_WithOptionalFlags(t *testing.T) {
	t.Parallel()

	repoURL := "https://github.com/user/repo"
	customName := "my-custom-name"
	customDescription := "My custom description"
	customBranch := "develop"

	// Create mock server that captures request body
	var receivedRequest dto.CreateRepositoryRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&receivedRequest)
		assert.NoError(t, err)

		response := dto.RepositoryResponse{
			ID:            uuid.New(),
			URL:           receivedRequest.URL,
			Name:          receivedRequest.Name,
			Description:   receivedRequest.Description,
			DefaultBranch: receivedRequest.DefaultBranch,
			Status:        "pending",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	// Create command with optional flags
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{
		"repos", "add", repoURL,
		"--api-url", server.URL,
		"--name", customName,
		"--description", customDescription,
		"--branch", customBranch,
	})

	// Execute command
	err := rootCmd.Execute()
	require.NoError(t, err, "repos add command should execute without error")

	// Verify request body contained optional fields
	assert.Equal(t, repoURL, receivedRequest.URL)
	assert.Equal(t, customName, receivedRequest.Name)
	require.NotNil(t, receivedRequest.Description)
	assert.Equal(t, customDescription, *receivedRequest.Description)
	require.NotNil(t, receivedRequest.DefaultBranch)
	assert.Equal(t, customBranch, *receivedRequest.DefaultBranch)

	// Verify JSON output
	var response client.Response
	json.Unmarshal(stdout.Bytes(), &response)
	assert.True(t, response.Success)
}

// TestReposAddCmd_MissingURL verifies the repos add command requires a URL argument.
// This test ensures the command fails with proper error when URL is not provided.
func TestReposAddCmd_MissingURL(t *testing.T) {
	t.Parallel()

	// Create command without URL argument
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"repos", "add", "--api-url", "http://localhost:8080"})

	// Execute command
	_ = rootCmd.Execute()

	// Parse and verify JSON error output
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on missing argument")

	assert.False(t, response.Success, "response should indicate failure")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotEmpty(t, response.Error.Code, "error should have a code")
	assert.Contains(t, response.Error.Message, "require", "error message should mention required argument")
}

// TestReposAddCmd_InvalidURL verifies the repos add command handles validation errors.
// This test ensures invalid URLs are rejected with proper error messages.
func TestReposAddCmd_InvalidURL(t *testing.T) {
	t.Parallel()

	invalidURL := "not-a-valid-url"

	// Create mock server that returns validation error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid URL format"))
	}))
	defer server.Close()

	// Create command with invalid URL
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"repos", "add", invalidURL, "--api-url", server.URL})

	// Execute command
	_ = rootCmd.Execute()

	// Parse and verify JSON error output
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on validation error")

	assert.False(t, response.Success, "response should indicate failure")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotEmpty(t, response.Error.Code, "error should have a code")
}

// TestReposAddCmd_ServerError verifies error JSON output when server returns an error.
// This test ensures proper error handling and JSON error formatting for add command.
func TestReposAddCmd_ServerError(t *testing.T) {
	t.Parallel()

	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	// Create command
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"repos", "add", "https://github.com/user/repo", "--api-url", server.URL})

	// Execute command
	_ = rootCmd.Execute()

	// Parse and verify JSON error output
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on error")

	assert.False(t, response.Success, "response should indicate failure")
	assert.Nil(t, response.Data, "response should not contain data on error")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotEmpty(t, response.Error.Code, "error should have a code")
	assert.Contains(t, response.Error.Message, "500", "error message should mention status code")
}

// TestReposListCmd_UsesGlobalFlags verifies the list command respects global flags.
// This test ensures --api-url and --timeout flags from root command are used.
func TestReposListCmd_UsesGlobalFlags(t *testing.T) {
	t.Parallel()

	// Create server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		response := dto.RepositoryListResponse{
			Repositories: []dto.RepositoryResponse{},
			Pagination:   dto.PaginationResponse{Limit: 20, Offset: 0, Total: 0, HasMore: false},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create command with short timeout
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"repos", "list", "--api-url", server.URL, "--timeout", "50ms"})

	// Execute command - should timeout
	_ = rootCmd.Execute()

	// Should output valid JSON even on timeout
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "should output valid JSON")
	assert.False(t, response.Success, "should indicate failure on timeout")
}

// TestReposGetCmd_UsesGlobalFlags verifies the get command respects global flags.
// This test ensures timeout and API URL configuration is properly applied.
func TestReposGetCmd_UsesGlobalFlags(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()

	// Create server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		response := dto.RepositoryResponse{
			ID:        repoID,
			URL:       "https://github.com/user/repo",
			Name:      "user/repo",
			Status:    "completed",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create command with short timeout
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"repos", "get", repoID.String(), "--api-url", server.URL, "--timeout", "50ms"})

	// Execute command - should timeout
	_ = rootCmd.Execute()

	// Should output valid JSON even on timeout
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "should output valid JSON")
	assert.False(t, response.Success, "should indicate failure on timeout")
}

// TestReposAddCmd_UsesGlobalFlags verifies the add command respects global flags.
// This test ensures timeout handling works correctly for repository creation.
func TestReposAddCmd_UsesGlobalFlags(t *testing.T) {
	t.Parallel()

	// Create server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		response := dto.RepositoryResponse{
			ID:        uuid.New(),
			URL:       "https://github.com/user/repo",
			Name:      "user/repo",
			Status:    "pending",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create command with short timeout
	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{
		"repos", "add", "https://github.com/user/repo",
		"--api-url", server.URL,
		"--timeout", "50ms",
	})

	// Execute command - should timeout
	_ = rootCmd.Execute()

	// Should output valid JSON even on timeout
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "should output valid JSON")
	assert.False(t, response.Success, "should indicate failure on timeout")
}

// TestReposCmd_OutputToStdout verifies all repos commands output to stdout, not stderr.
// This test ensures proper stream usage for JSON output across all subcommands.
func TestReposCmd_OutputToStdout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle different endpoints
		if r.URL.Path == "/repositories" && r.Method == http.MethodGet {
			response := dto.RepositoryListResponse{
				Repositories: []dto.RepositoryResponse{},
				Pagination:   dto.PaginationResponse{Limit: 20, Offset: 0, Total: 0, HasMore: false},
			}
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"repos", "list", "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	// JSON output should be on stdout
	assert.NotEmpty(t, stdout.String(), "stdout should contain JSON output")

	// stderr should not contain JSON response
	stderrContent := stderr.String()
	if stderrContent != "" {
		var response client.Response
		err := json.Unmarshal(stderr.Bytes(), &response)
		assert.Error(t, err, "stderr should not contain JSON response")
	}
}

// TestReposCmd_Integration verifies end-to-end integration of all repos commands.
// This test simulates real usage scenarios with all components working together.
func TestReposCmd_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Parallel()

	createdRepoID := uuid.New()
	repoURL := "https://github.com/user/integration-test"

	// Create realistic mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle repository creation
		if r.URL.Path == "/repositories" && r.Method == http.MethodPost {
			response := dto.RepositoryResponse{
				ID:        createdRepoID,
				URL:       repoURL,
				Name:      "user/integration-test",
				Status:    "pending",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle repository get
		if r.Method == http.MethodGet && r.URL.Path == "/repositories/"+createdRepoID.String() {
			response := dto.RepositoryResponse{
				ID:        createdRepoID,
				URL:       repoURL,
				Name:      "user/integration-test",
				Status:    "completed",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle repository list
		if r.URL.Path == "/repositories" && r.Method == http.MethodGet {
			response := dto.RepositoryListResponse{
				Repositories: []dto.RepositoryResponse{
					{
						ID:        createdRepoID,
						URL:       repoURL,
						Name:      "user/integration-test",
						Status:    "completed",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				},
				Pagination: dto.PaginationResponse{Limit: 20, Offset: 0, Total: 1, HasMore: false},
			}
			json.NewEncoder(w).Encode(response)
			return
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test 1: Add repository
	{
		var stdout bytes.Buffer
		rootCmd := commands.NewRootCmd()
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"repos", "add", repoURL, "--api-url", server.URL})

		err := rootCmd.ExecuteContext(ctx)
		require.NoError(t, err, "add command should succeed")

		var response client.Response
		json.Unmarshal(stdout.Bytes(), &response)
		assert.True(t, response.Success)
	}

	// Test 2: Get repository
	{
		var stdout bytes.Buffer
		rootCmd := commands.NewRootCmd()
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"repos", "get", createdRepoID.String(), "--api-url", server.URL})

		err := rootCmd.ExecuteContext(ctx)
		require.NoError(t, err, "get command should succeed")

		var response client.Response
		json.Unmarshal(stdout.Bytes(), &response)
		assert.True(t, response.Success)
	}

	// Test 3: List repositories
	{
		var stdout bytes.Buffer
		rootCmd := commands.NewRootCmd()
		rootCmd.SetOut(&stdout)
		rootCmd.SetArgs([]string{"repos", "list", "--api-url", server.URL})

		err := rootCmd.ExecuteContext(ctx)
		require.NoError(t, err, "list command should succeed")

		var response client.Response
		json.Unmarshal(stdout.Bytes(), &response)
		assert.True(t, response.Success)

		dataJSON, _ := json.Marshal(response.Data)
		var listData dto.RepositoryListResponse
		json.Unmarshal(dataJSON, &listData)
		assert.Len(t, listData.Repositories, 1)
	}
}

// TestReposAddCommand_WithWaitFlag_Success verifies repos add --wait polls until completion.
// This test ensures the --wait flag properly waits for repository indexing to complete.
func TestReposAddCommand_WithWaitFlag_Success(t *testing.T) {
	t.Parallel()

	repoURL := "https://github.com/user/waitrepo"
	repoID := uuid.New()
	callCount := 0
	statuses := []string{dto.StatusPending, dto.StatusCloning, dto.StatusProcessing, dto.StatusCompleted}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle repository creation
		if r.URL.Path == "/repositories" && r.Method == http.MethodPost {
			response := dto.RepositoryResponse{
				ID:        repoID,
				URL:       repoURL,
				Name:      "user/waitrepo",
				Status:    dto.StatusPending,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle polling requests
		if r.URL.Path == "/repositories/"+repoID.String() && r.Method == http.MethodGet {
			currentStatus := statuses[callCount]
			if callCount < len(statuses)-1 {
				callCount++
			}

			response := dto.RepositoryResponse{
				ID:        repoID,
				URL:       repoURL,
				Name:      "user/waitrepo",
				Status:    currentStatus,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			if currentStatus == dto.StatusCompleted {
				response.TotalFiles = 100
				response.TotalChunks = 500
			}

			json.NewEncoder(w).Encode(response)
			return
		}
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{
		"repos", "add", repoURL,
		"--api-url", server.URL,
		"--wait",
		"--poll-interval", "50ms",
		"--wait-timeout", "10s",
	})

	err := rootCmd.Execute()
	require.NoError(t, err, "repos add --wait should succeed when repository completes")

	// Verify JSON output to stdout
	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON")
	assert.True(t, response.Success, "response should indicate success")

	// Verify repository data shows completed status
	dataJSON, _ := json.Marshal(response.Data)
	var repoData dto.RepositoryResponse
	json.Unmarshal(dataJSON, &repoData)
	assert.Equal(t, dto.StatusCompleted, repoData.Status, "final status should be completed")
	assert.Equal(t, 100, repoData.TotalFiles, "total files should be populated")
	assert.Equal(t, 500, repoData.TotalChunks, "total chunks should be populated")

	// Verify progress output to stderr
	stderrOutput := stderr.String()
	assert.NotEmpty(t, stderrOutput, "progress should be written to stderr")
	assert.Contains(t, stderrOutput, "polling", "progress should mention polling")
}

// TestReposAddCommand_WithWaitFlag_Failure verifies repos add --wait returns error when job fails.
// This test ensures the --wait flag properly detects and reports repository indexing failures.
func TestReposAddCommand_WithWaitFlag_Failure(t *testing.T) {
	t.Parallel()

	repoURL := "https://github.com/user/failrepo"
	repoID := uuid.New()
	callCount := 0
	statuses := []string{dto.StatusPending, dto.StatusCloning, dto.StatusFailed}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle repository creation
		if r.URL.Path == "/repositories" && r.Method == http.MethodPost {
			response := dto.RepositoryResponse{
				ID:        repoID,
				URL:       repoURL,
				Name:      "user/failrepo",
				Status:    dto.StatusPending,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle polling requests
		if r.URL.Path == "/repositories/"+repoID.String() && r.Method == http.MethodGet {
			currentStatus := statuses[callCount]
			if callCount < len(statuses)-1 {
				callCount++
			}

			response := dto.RepositoryResponse{
				ID:        repoID,
				URL:       repoURL,
				Name:      "user/failrepo",
				Status:    currentStatus,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			json.NewEncoder(w).Encode(response)
			return
		}
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{
		"repos", "add", repoURL,
		"--api-url", server.URL,
		"--wait",
		"--poll-interval", "50ms",
		"--wait-timeout", "10s",
	})

	err := rootCmd.Execute()
	// Command should complete, but JSON response should indicate failure
	_ = err // Error handling will be verified through JSON response

	// Verify JSON error output to stdout
	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on failure")
	assert.False(t, response.Success, "response should indicate failure")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.Contains(t, response.Error.Message, "failed", "error message should mention failure")

	// Verify progress was written to stderr
	stderrOutput := stderr.String()
	assert.NotEmpty(t, stderrOutput, "progress should be written to stderr")
}

// TestReposAddCommand_WithWaitFlag_Timeout verifies repos add --wait respects --wait-timeout.
// This test ensures the poller stops after the configured timeout period.
func TestReposAddCommand_WithWaitFlag_Timeout(t *testing.T) {
	t.Parallel()

	repoURL := "https://github.com/user/slowrepo"
	repoID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle repository creation
		if r.URL.Path == "/repositories" && r.Method == http.MethodPost {
			response := dto.RepositoryResponse{
				ID:        repoID,
				URL:       repoURL,
				Name:      "user/slowrepo",
				Status:    dto.StatusPending,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle polling requests - always return processing (never completes)
		if r.URL.Path == "/repositories/"+repoID.String() && r.Method == http.MethodGet {
			response := dto.RepositoryResponse{
				ID:        repoID,
				URL:       repoURL,
				Name:      "user/slowrepo",
				Status:    dto.StatusProcessing,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			json.NewEncoder(w).Encode(response)
			return
		}
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{
		"repos", "add", repoURL,
		"--api-url", server.URL,
		"--wait",
		"--poll-interval", "50ms",
		"--wait-timeout", "200ms", // Very short timeout
	})

	err := rootCmd.Execute()
	_ = err // Error handling will be verified through JSON response

	// Verify JSON error output indicates timeout
	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on timeout")
	assert.False(t, response.Success, "response should indicate failure on timeout")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.Contains(t, response.Error.Message, "timeout", "error message should mention timeout")

	// Verify progress was written to stderr
	stderrOutput := stderr.String()
	assert.NotEmpty(t, stderrOutput, "progress should be written to stderr during polling")
}

// TestReposAddCommand_WaitFlags verifies --wait, --poll-interval, and --wait-timeout flags exist.
// This test ensures all polling-related flags are registered on the repos add command.
func TestReposAddCommand_WaitFlags(t *testing.T) {
	t.Parallel()

	rootCmd := commands.NewRootCmd()

	// Find repos add command
	var addCmd *bool
	for _, reposCmd := range rootCmd.Commands() {
		if reposCmd.Use == "repos" {
			for _, subCmd := range reposCmd.Commands() {
				if subCmd.Use == "add" {
					// Verify flags exist
					waitFlag := subCmd.Flags().Lookup("wait")
					require.NotNil(t, waitFlag, "--wait flag should exist")
					assert.Equal(t, "bool", waitFlag.Value.Type(), "--wait should be a boolean flag")

					pollIntervalFlag := subCmd.Flags().Lookup("poll-interval")
					require.NotNil(t, pollIntervalFlag, "--poll-interval flag should exist")
					assert.Equal(
						t,
						"duration",
						pollIntervalFlag.Value.Type(),
						"--poll-interval should be a duration flag",
					)

					waitTimeoutFlag := subCmd.Flags().Lookup("wait-timeout")
					require.NotNil(t, waitTimeoutFlag, "--wait-timeout flag should exist")
					assert.Equal(
						t,
						"duration",
						waitTimeoutFlag.Value.Type(),
						"--wait-timeout should be a duration flag",
					)

					found := true
					addCmd = &found
					break
				}
			}
			break
		}
	}

	require.NotNil(t, addCmd, "repos add command should exist with polling flags")
}
