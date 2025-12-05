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

// TestNewSearchCmd verifies that NewSearchCmd creates a search command with correct metadata.
// This test ensures the search command is properly initialized with correct use and description.
func TestNewSearchCmd(t *testing.T) {
	t.Parallel()

	cmd := commands.NewSearchCmd()

	require.NotNil(t, cmd, "NewSearchCmd should return a non-nil command")
	assert.Equal(t, "search", cmd.Use, "search command Use should be 'search'")
	assert.NotEmpty(t, cmd.Short, "search command Short description should not be empty")
	assert.Contains(t, cmd.Short, "search", "Short description should mention search")
	assert.NotNil(t, cmd.RunE, "search command should have a RunE function")
}

// TestSearchCmd_RegisteredWithRoot verifies the search command is registered with the root command.
// This test ensures the search command is available as a subcommand.
func TestSearchCmd_RegisteredWithRoot(t *testing.T) {
	t.Parallel()

	rootCmd := commands.NewRootCmd()

	// Find search command in subcommands
	var found bool
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "search" {
			found = true
			break
		}
	}

	assert.True(t, found, "search command should be registered with root command")
}

// TestSearchCmd_QueryArgument verifies the search command requires exactly one query argument.
// This test ensures proper argument validation for the search query.
func TestSearchCmd_QueryArgument(t *testing.T) {
	t.Parallel()

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.SearchResponseDTO{
			Results:    []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{Limit: 10, Offset: 0, Total: 0, HasMore: false},
			Metadata:   dto.SearchMetadata{Query: "test query", ExecutionTimeMs: 50},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"search", "test query", "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err, "search command with single query argument should execute")

	// Verify valid JSON response
	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON")
	assert.True(t, response.Success, "should succeed with valid query argument")
}

// TestSearchCmd_MissingQuery verifies error JSON output when no query is provided.
// This test ensures proper error handling when the required query argument is missing.
func TestSearchCmd_MissingQuery(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"search"})

	// Execute command - should fail validation
	_ = rootCmd.Execute()

	// Parse and verify JSON error output
	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on error")

	assert.False(t, response.Success, "response should indicate failure")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotEmpty(t, response.Error.Message, "error should have a message")
}

// TestSearchCmd_HasAllFlags verifies all required flags are defined.
// This test ensures the search command has all expected flags with correct types and defaults.
func TestSearchCmd_HasAllFlags(t *testing.T) {
	t.Parallel()

	cmd := commands.NewSearchCmd()

	tests := []struct {
		name         string
		flagName     string
		shorthand    string
		expectedType string
	}{
		{name: "limit flag", flagName: "limit", shorthand: "l", expectedType: "int"},
		{name: "offset flag", flagName: "offset", shorthand: "", expectedType: "int"},
		{name: "repo-ids flag", flagName: "repo-ids", shorthand: "", expectedType: "stringSlice"},
		{name: "repo-names flag", flagName: "repo-names", shorthand: "", expectedType: "stringSlice"},
		{name: "languages flag", flagName: "languages", shorthand: "", expectedType: "stringSlice"},
		{name: "file-types flag", flagName: "file-types", shorthand: "", expectedType: "stringSlice"},
		{name: "threshold flag", flagName: "threshold", shorthand: "t", expectedType: "float64"},
		{name: "sort flag", flagName: "sort", shorthand: "s", expectedType: "string"},
		{name: "types flag", flagName: "types", shorthand: "", expectedType: "stringSlice"},
		{name: "entity-name flag", flagName: "entity-name", shorthand: "", expectedType: "string"},
		{name: "visibility flag", flagName: "visibility", shorthand: "", expectedType: "stringSlice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			flag := cmd.Flags().Lookup(tt.flagName)
			require.NotNil(t, flag, "flag %s should be defined", tt.flagName)
			assert.Equal(t, tt.flagName, flag.Name, "flag name should match")

			if tt.shorthand != "" {
				assert.Equal(t, tt.shorthand, flag.Shorthand, "shorthand should match for %s", tt.flagName)
			}

			// Verify flag type
			assert.Equal(
				t,
				tt.expectedType,
				flag.Value.Type(),
				"flag %s should have type %s",
				tt.flagName,
				tt.expectedType,
			)
		})
	}

	// Verify default values
	limitFlag := cmd.Flags().Lookup("limit")
	assert.Equal(t, "10", limitFlag.DefValue, "limit should default to 10")

	offsetFlag := cmd.Flags().Lookup("offset")
	assert.Equal(t, "0", offsetFlag.DefValue, "offset should default to 0")

	thresholdFlag := cmd.Flags().Lookup("threshold")
	assert.Equal(t, "0.7", thresholdFlag.DefValue, "threshold should default to 0.7")

	sortFlag := cmd.Flags().Lookup("sort")
	assert.Equal(t, "similarity:desc", sortFlag.DefValue, "sort should default to similarity:desc")
}

// TestSearchCmd_Success verifies basic search returns JSON success response.
// This test ensures the search command properly handles a successful API response.
func TestSearchCmd_Success(t *testing.T) {
	t.Parallel()

	repoID := uuid.New()
	chunkID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search", r.URL.Path, "should request /search endpoint")
		assert.Equal(t, http.MethodPost, r.Method, "should use POST method")

		var req dto.SearchRequestDTO
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "test query", req.Query, "query should match")

		response := dto.SearchResponseDTO{
			Results: []dto.SearchResultDTO{
				{
					ChunkID:         chunkID,
					Content:         "func example() {}",
					SimilarityScore: 0.95,
					Repository: dto.RepositoryInfo{
						ID:   repoID,
						Name: "org/repo",
						URL:  "https://github.com/org/repo",
					},
					FilePath:   "/src/main.go",
					Language:   "go",
					StartLine:  10,
					EndLine:    15,
					Type:       "function",
					EntityName: "example",
				},
			},
			Pagination: dto.PaginationResponse{Limit: 10, Offset: 0, Total: 1, HasMore: false},
			Metadata:   dto.SearchMetadata{Query: "test query", ExecutionTimeMs: 50},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"search", "test query", "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON")

	assert.True(t, response.Success, "response should indicate success")
	assert.Nil(t, response.Error, "response should not contain error")
	assert.NotNil(t, response.Data, "response should contain data")
	assert.NotZero(t, response.Timestamp, "response should have timestamp")
}

// TestSearchCmd_WithAllFlags verifies search with all flags populated.
// This test ensures all flags are properly parsed and sent to the API.
func TestSearchCmd_WithAllFlags(t *testing.T) {
	t.Parallel()

	repoID1 := uuid.New()
	repoID2 := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req dto.SearchRequestDTO
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)

		// Verify all fields are populated
		assert.Equal(t, "complex query", req.Query)
		assert.Equal(t, 20, req.Limit)
		assert.Equal(t, 10, req.Offset)
		assert.Contains(t, req.RepositoryIDs, repoID1)
		assert.Contains(t, req.RepositoryIDs, repoID2)
		assert.Contains(t, req.RepositoryNames, "org/repo1")
		assert.Contains(t, req.RepositoryNames, "org/repo2")
		assert.Contains(t, req.Languages, "go")
		assert.Contains(t, req.Languages, "python")
		assert.Contains(t, req.FileTypes, ".go")
		assert.Contains(t, req.FileTypes, ".py")
		assert.InDelta(t, 0.8, req.SimilarityThreshold, 0.001)
		assert.Equal(t, "file_path:asc", req.Sort)
		assert.Contains(t, req.Types, "function")
		assert.Contains(t, req.Types, "method")
		assert.Equal(t, "connect", req.EntityName)
		assert.Contains(t, req.Visibility, "public")
		assert.Contains(t, req.Visibility, "private")

		response := dto.SearchResponseDTO{
			Results:    []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{Limit: 20, Offset: 10, Total: 0, HasMore: false},
			Metadata:   dto.SearchMetadata{Query: "complex query", ExecutionTimeMs: 100},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{
		"search", "complex query",
		"--api-url", server.URL,
		"--limit", "20",
		"--offset", "10",
		"--repo-ids", repoID1.String() + "," + repoID2.String(),
		"--repo-names", "org/repo1,org/repo2",
		"--languages", "go,python",
		"--file-types", ".go,.py",
		"--threshold", "0.8",
		"--sort", "file_path:asc",
		"--types", "function,method",
		"--entity-name", "connect",
		"--visibility", "public,private",
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
}

// TestSearchCmd_EmptyResults verifies search with no matches returns empty array.
// This test ensures proper handling of searches that return no results.
func TestSearchCmd_EmptyResults(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.SearchResponseDTO{
			Results:    []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{Limit: 10, Offset: 0, Total: 0, HasMore: false},
			Metadata:   dto.SearchMetadata{Query: "nonexistent query", ExecutionTimeMs: 30},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"search", "nonexistent query", "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.Success)
	assert.NotNil(t, response.Data)

	// Verify empty results
	dataJSON, err := json.Marshal(response.Data)
	require.NoError(t, err)

	var searchData dto.SearchResponseDTO
	err = json.Unmarshal(dataJSON, &searchData)
	require.NoError(t, err)

	assert.Empty(t, searchData.Results, "results should be empty")
	assert.Equal(t, 0, searchData.Pagination.Total, "total should be 0")
}

// TestSearchCmd_WithPagination verifies pagination metadata in response.
// This test ensures pagination information is properly returned.
func TestSearchCmd_WithPagination(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req dto.SearchRequestDTO
		json.NewDecoder(r.Body).Decode(&req)

		response := dto.SearchResponseDTO{
			Results:    []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{Limit: 10, Offset: 20, Total: 100, HasMore: true},
			Metadata:   dto.SearchMetadata{Query: "paginated query", ExecutionTimeMs: 75},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"search", "paginated query", "--api-url", server.URL, "--limit", "10", "--offset", "20"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err)

	dataJSON, err := json.Marshal(response.Data)
	require.NoError(t, err)

	var searchData dto.SearchResponseDTO
	err = json.Unmarshal(dataJSON, &searchData)
	require.NoError(t, err)

	assert.Equal(t, 10, searchData.Pagination.Limit)
	assert.Equal(t, 20, searchData.Pagination.Offset)
	assert.Equal(t, 100, searchData.Pagination.Total)
	assert.True(t, searchData.Pagination.HasMore)
}

// TestSearchCmd_WithLanguageFilter verifies language filtering.
// This test ensures the --languages flag is properly handled.
func TestSearchCmd_WithLanguageFilter(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req dto.SearchRequestDTO
		json.NewDecoder(r.Body).Decode(&req)

		assert.Contains(t, req.Languages, "go")
		assert.Contains(t, req.Languages, "python")
		assert.Len(t, req.Languages, 2)

		response := dto.SearchResponseDTO{
			Results:    []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{Limit: 10, Offset: 0, Total: 0, HasMore: false},
			Metadata:   dto.SearchMetadata{Query: "test", ExecutionTimeMs: 40},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"search", "test", "--api-url", server.URL, "--languages", "go,python"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
}

// TestSearchCmd_WithRepoNamesFilter verifies repository name filtering.
// This test ensures the --repo-names flag is properly handled.
func TestSearchCmd_WithRepoNamesFilter(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req dto.SearchRequestDTO
		json.NewDecoder(r.Body).Decode(&req)

		assert.Contains(t, req.RepositoryNames, "org/repo1")
		assert.Contains(t, req.RepositoryNames, "org/repo2")
		assert.Len(t, req.RepositoryNames, 2)

		response := dto.SearchResponseDTO{
			Results:    []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{Limit: 10, Offset: 0, Total: 0, HasMore: false},
			Metadata:   dto.SearchMetadata{Query: "test", ExecutionTimeMs: 45},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"search", "test", "--api-url", server.URL, "--repo-names", "org/repo1,org/repo2"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
}

// TestSearchCmd_WithTypesFilter verifies type filtering.
// This test ensures the --types flag is properly handled.
func TestSearchCmd_WithTypesFilter(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req dto.SearchRequestDTO
		json.NewDecoder(r.Body).Decode(&req)

		assert.Contains(t, req.Types, "function")
		assert.Contains(t, req.Types, "method")
		assert.Len(t, req.Types, 2)

		response := dto.SearchResponseDTO{
			Results:    []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{Limit: 10, Offset: 0, Total: 0, HasMore: false},
			Metadata:   dto.SearchMetadata{Query: "test", ExecutionTimeMs: 35},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"search", "test", "--api-url", server.URL, "--types", "function,method"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
}

// TestSearchCmd_WithVisibilityFilter verifies visibility filtering.
// This test ensures the --visibility flag is properly handled.
func TestSearchCmd_WithVisibilityFilter(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req dto.SearchRequestDTO
		json.NewDecoder(r.Body).Decode(&req)

		assert.Contains(t, req.Visibility, "public")
		assert.Contains(t, req.Visibility, "private")
		assert.Len(t, req.Visibility, 2)

		response := dto.SearchResponseDTO{
			Results:    []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{Limit: 10, Offset: 0, Total: 0, HasMore: false},
			Metadata:   dto.SearchMetadata{Query: "test", ExecutionTimeMs: 42},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"search", "test", "--api-url", server.URL, "--visibility", "public,private"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
}

// TestSearchCmd_WithRepoIDsFilter verifies repository ID filtering.
// This test ensures the --repo-ids flag is properly handled with valid UUIDs.
func TestSearchCmd_WithRepoIDsFilter(t *testing.T) {
	t.Parallel()

	repoID1 := uuid.New()
	repoID2 := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req dto.SearchRequestDTO
		json.NewDecoder(r.Body).Decode(&req)

		assert.Contains(t, req.RepositoryIDs, repoID1)
		assert.Contains(t, req.RepositoryIDs, repoID2)
		assert.Len(t, req.RepositoryIDs, 2)

		response := dto.SearchResponseDTO{
			Results:    []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{Limit: 10, Offset: 0, Total: 0, HasMore: false},
			Metadata:   dto.SearchMetadata{Query: "test", ExecutionTimeMs: 38},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{
		"search", "test",
		"--api-url", server.URL,
		"--repo-ids", repoID1.String() + "," + repoID2.String(),
	})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var response client.Response
	err = json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
}

// TestSearchCmd_ServerError verifies error JSON output when server returns an error.
// This test ensures proper error handling for server-side errors.
func TestSearchCmd_ServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"search", "test query", "--api-url", server.URL})

	_ = rootCmd.Execute()

	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on error")

	assert.False(t, response.Success, "response should indicate failure")
	assert.Nil(t, response.Data, "response should not contain data on error")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotZero(t, response.Timestamp, "response should have timestamp")
	assert.NotEmpty(t, response.Error.Code, "error should have a code")
	assert.NotEmpty(t, response.Error.Message, "error should have a message")
	assert.Contains(t, response.Error.Message, "500", "error message should mention status code")
}

// TestSearchCmd_ConnectionError verifies error JSON output when connection fails.
// This test ensures network errors are properly handled and formatted.
func TestSearchCmd_ConnectionError(t *testing.T) {
	t.Parallel()

	invalidURL := "http://localhost:1"

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"search", "test query", "--api-url", invalidURL, "--timeout", "100ms"})

	_ = rootCmd.Execute()

	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on connection error")

	assert.False(t, response.Success, "response should indicate failure")
	assert.Nil(t, response.Data, "response should not contain data on error")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotZero(t, response.Timestamp, "response should have timestamp")
	assert.NotEmpty(t, response.Error.Code, "error should have a code")
	assert.NotEmpty(t, response.Error.Message, "error should have a message")
	assert.Contains(t, []string{"CONNECTION_ERROR", "NETWORK_ERROR", "TIMEOUT_ERROR"},
		response.Error.Code, "error code should indicate connection/network issue")
}

// TestSearchCmd_InvalidRepoID verifies error JSON output for invalid UUID in --repo-ids.
// This test ensures proper validation of repository ID format.
func TestSearchCmd_InvalidRepoID(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stdout)
	rootCmd.SetArgs([]string{"search", "test query", "--repo-ids", "invalid-uuid"})

	_ = rootCmd.Execute()

	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "output should be valid JSON even on validation error")

	assert.False(t, response.Success, "response should indicate failure")
	assert.Nil(t, response.Data, "response should not contain data on error")
	assert.NotNil(t, response.Error, "response should contain error")
	assert.NotEmpty(t, response.Error.Message, "error should have a message")
}

// TestSearchCmd_UsesGlobalFlags verifies the command respects global flags.
// This test ensures --api-url and --timeout flags are properly used.
func TestSearchCmd_UsesGlobalFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		serverDelay   time.Duration
		expectSuccess bool
	}{
		{
			name:          "custom api-url flag",
			args:          []string{"search", "test", "--api-url", "http://custom.example.com"},
			serverDelay:   0,
			expectSuccess: false,
		},
		{
			name:          "timeout flag causes timeout",
			args:          []string{"search", "test", "--timeout", "50ms"},
			serverDelay:   200 * time.Millisecond,
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.serverDelay > 0 {
					time.Sleep(tt.serverDelay)
				}
				response := dto.SearchResponseDTO{
					Results:    []dto.SearchResultDTO{},
					Pagination: dto.PaginationResponse{Limit: 10, Offset: 0, Total: 0, HasMore: false},
					Metadata:   dto.SearchMetadata{Query: "test", ExecutionTimeMs: 50},
				}
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			if tt.name == "timeout flag causes timeout" {
				tt.args = []string{"search", "test", "--api-url", server.URL, "--timeout", "50ms"}
			}

			var stdout bytes.Buffer
			rootCmd := commands.NewRootCmd()
			rootCmd.SetOut(&stdout)
			rootCmd.SetErr(&stdout)
			rootCmd.SetArgs(tt.args)

			_ = rootCmd.Execute()

			var response client.Response
			err := json.Unmarshal(stdout.Bytes(), &response)
			require.NoError(t, err, "should output valid JSON")

			assert.Equal(t, tt.expectSuccess, response.Success, "success flag should match expected")
		})
	}
}

// TestSearchCmd_JSONOutput verifies JSON structure matches Response envelope.
// This test ensures all fields are properly serialized and follow the expected schema.
func TestSearchCmd_JSONOutput(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.SearchResponseDTO{
			Results:    []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{Limit: 10, Offset: 0, Total: 0, HasMore: false},
			Metadata:   dto.SearchMetadata{Query: "test", ExecutionTimeMs: 60},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"search", "test", "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	var rawJSON map[string]interface{}
	err = json.Unmarshal(stdout.Bytes(), &rawJSON)
	require.NoError(t, err, "output should be valid JSON")

	assert.Contains(t, rawJSON, "success", "JSON should have 'success' field")
	assert.Contains(t, rawJSON, "timestamp", "JSON should have 'timestamp' field")

	success, ok := rawJSON["success"].(bool)
	require.True(t, ok, "'success' should be a boolean")
	if success {
		assert.Contains(t, rawJSON, "data", "successful response should have 'data' field")
		assert.NotContains(t, rawJSON, "error", "successful response should not have 'error' field")
	} else {
		assert.Contains(t, rawJSON, "error", "failed response should have 'error' field")
		assert.NotContains(t, rawJSON, "data", "failed response should not have 'data' field")
	}

	timestampStr, ok := rawJSON["timestamp"].(string)
	require.True(t, ok, "'timestamp' should be a string")
	_, err = time.Parse(time.RFC3339Nano, timestampStr)
	assert.NoError(t, err, "timestamp should be valid RFC3339 format")
}

// TestSearchCmd_OutputToStdout verifies output is written to stdout, not stderr.
// This test ensures proper stream usage for JSON output.
func TestSearchCmd_OutputToStdout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		response := dto.SearchResponseDTO{
			Results:    []dto.SearchResultDTO{},
			Pagination: dto.PaginationResponse{Limit: 10, Offset: 0, Total: 0, HasMore: false},
			Metadata:   dto.SearchMetadata{Query: "test", ExecutionTimeMs: 55},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"search", "test", "--api-url", server.URL})

	err := rootCmd.Execute()
	require.NoError(t, err)

	assert.NotEmpty(t, stdout.String(), "stdout should contain JSON output")

	stderrContent := stderr.String()
	if stderrContent != "" {
		var response client.Response
		err := json.Unmarshal(stderr.Bytes(), &response)
		assert.Error(t, err, "stderr should not contain JSON response - only stdout should")
	}
}

// TestSearchCmd_Integration verifies end-to-end integration.
// This test simulates a real usage scenario with all components.
func TestSearchCmd_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Parallel()

	repoID := uuid.New()
	chunkID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.NotEmpty(t, r.Header.Get("User-Agent"))

		var req dto.SearchRequestDTO
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "integration test query", req.Query)

		response := dto.SearchResponseDTO{
			Results: []dto.SearchResultDTO{
				{
					ChunkID:         chunkID,
					Content:         "func integrationTest() {}",
					SimilarityScore: 0.92,
					Repository: dto.RepositoryInfo{
						ID:   repoID,
						Name: "test/integration",
						URL:  "https://github.com/test/integration",
					},
					FilePath:   "/test/integration.go",
					Language:   "go",
					StartLine:  1,
					EndLine:    10,
					Type:       "function",
					EntityName: "integrationTest",
					Visibility: "public",
				},
			},
			Pagination: dto.PaginationResponse{Limit: 10, Offset: 0, Total: 1, HasMore: false},
			Metadata:   dto.SearchMetadata{Query: "integration test query", ExecutionTimeMs: 120},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	rootCmd := commands.NewRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"search", "integration test query", "--api-url", server.URL, "--timeout", "5s"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- rootCmd.ExecuteContext(ctx)
	}()

	select {
	case err := <-errChan:
		require.NoError(t, err, "integration test should complete successfully")
	case <-ctx.Done():
		t.Fatal("integration test timed out")
	}

	var response client.Response
	err := json.Unmarshal(stdout.Bytes(), &response)
	require.NoError(t, err, "should output valid JSON")
	assert.True(t, response.Success, "integration test should succeed")
	assert.NotNil(t, response.Data, "should contain search data")
	assert.Nil(t, response.Error, "should not contain error")

	dataJSON, _ := json.Marshal(response.Data)
	var searchData dto.SearchResponseDTO
	json.Unmarshal(dataJSON, &searchData)
	assert.Len(t, searchData.Results, 1)
	assert.Equal(t, "integration test query", searchData.Metadata.Query)
}
