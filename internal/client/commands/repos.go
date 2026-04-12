package commands

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/client"
	"codechunking/internal/domain/valueobject"
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// Error messages for repository command argument validation.
const (
	errMsgRequiresOneArgument = "requires exactly 1 argument"
	errMsgInvalidUUID         = "invalid repository ID: must be a valid UUID"
)

// parseUUIDArg parses and validates a UUID string argument, writing an error response on failure.
// Returns the parsed UUID and true on success, or uuid.Nil and false on failure.
func parseUUIDArg(cmd *cobra.Command, raw, errMsg string) (uuid.UUID, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		_ = client.WriteError(cmd.OutOrStdout(), errCodeInvalidArgument, errMsg, nil)
		return uuid.Nil, false
	}
	return id, true
}

// createClientFromFlags creates and validates a client from command flags.
// It handles configuration validation and client creation, writing errors
// to the provided output if validation or creation fails.
// Returns the client and true if successful, or nil and false on failure.
func createClientFromFlags(cmd *cobra.Command, out io.Writer) (*client.Client, bool) {
	apiURL := getStringFlag(cmd, "api-url", viperKeyAPIURL)
	timeout := getDurationFlag(cmd, "timeout", viperKeyTimeout)

	cfg := &client.Config{APIURL: apiURL, Timeout: timeout}
	if err := cfg.Validate(); err != nil {
		_ = client.WriteError(out, errCodeInvalidConfig, err.Error(), nil)
		return nil, false
	}

	c, err := client.NewClient(cfg)
	if err != nil {
		_ = client.WriteError(out, errCodeClientError, err.Error(), nil)
		return nil, false
	}

	return c, true
}

// NewReposCmd creates and returns the repos parent command.
// The command provides subcommands for managing repositories:
//   - list: List repositories with optional filters
//   - get: Get a single repository by ID
//   - add: Add a new repository for indexing
//   - delete: Delete (archive) a repository by ID
//   - jobs: List indexing jobs for a repository
//   - query: Search within a specific repository by UUID
func NewReposCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repos",
		Short: "Manage repositories",
	}

	cmd.AddCommand(NewReposListCmd())
	cmd.AddCommand(NewReposGetCmd())
	cmd.AddCommand(NewReposAddCmd())
	cmd.AddCommand(NewReposDeleteCmd())
	cmd.AddCommand(NewReposJobsCmd())
	cmd.AddCommand(NewReposQueryCmd())

	return cmd
}

// NewReposListCmd creates and returns the repos list command.
// The command lists repositories with optional filtering and pagination.
func NewReposListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List repositories",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, ok := createClientFromFlags(cmd, cmd.OutOrStdout())
			if !ok {
				return nil
			}

			limit, _ := cmd.Flags().GetInt("limit")
			offset, _ := cmd.Flags().GetInt("offset")
			status, _ := cmd.Flags().GetString("status")
			name, _ := cmd.Flags().GetString("name")
			sort, _ := cmd.Flags().GetString("sort")

			query := dto.RepositoryListQuery{
				Limit:  limit,
				Offset: offset,
				Status: status,
				Name:   name,
				Sort:   sort,
			}

			timeout := getDurationFlag(cmd, "timeout", viperKeyTimeout)
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			result, err := c.ListRepositories(ctx, query)
			if err != nil {
				code := determineErrorCode(err)
				_ = client.WriteError(cmd.OutOrStdout(), code, err.Error(), nil)
				return nil
			}

			return client.WriteSuccess(cmd.OutOrStdout(), result)
		},
	}

	cmd.Flags().IntP("limit", "l", dto.DefaultRepositoryLimit, "Maximum number of repositories to return")
	cmd.Flags().IntP("offset", "o", 0, "Number of repositories to skip")
	cmd.Flags().String("status", "", "Filter by status (pending, cloning, processing, completed, failed, archived)")
	cmd.Flags().String("name", "", "Filter by repository name")
	cmd.Flags().String("sort", "", "Sort order (created_at:asc, created_at:desc, name:asc, name:desc)")

	return cmd
}

// NewReposGetCmd creates and returns the repos get command.
// The command retrieves a single repository by UUID.
func NewReposGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a repository by ID",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				_ = client.WriteError(cmd.OutOrStdout(), errCodeInvalidArgument, errMsgRequiresOneArgument, nil)
				return nil
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return nil
			}

			id, ok := parseUUIDArg(cmd, args[0], errMsgInvalidUUID)
			if !ok {
				return nil
			}

			c, ok := createClientFromFlags(cmd, cmd.OutOrStdout())
			if !ok {
				return nil
			}

			timeout := getDurationFlag(cmd, "timeout", viperKeyTimeout)
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			result, err := c.GetRepository(ctx, id)
			if err != nil {
				code := determineErrorCode(err)
				_ = client.WriteError(cmd.OutOrStdout(), code, err.Error(), nil)
				return nil
			}

			return client.WriteSuccess(cmd.OutOrStdout(), result)
		},
	}

	return cmd
}

// NewReposAddCmd creates and returns the repos add command.
// The command creates a new repository for indexing.
func NewReposAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new repository",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				_ = client.WriteError(cmd.OutOrStdout(), errCodeInvalidArgument, errMsgRequiresOneArgument, nil)
				return nil
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return nil
			}

			c, ok := createClientFromFlags(cmd, cmd.OutOrStdout())
			if !ok {
				return nil
			}

			url := args[0]
			name, _ := cmd.Flags().GetString("name")
			description, _ := cmd.Flags().GetString("description")
			branch, _ := cmd.Flags().GetString("branch")

			req := dto.CreateRepositoryRequest{
				URL:  url,
				Name: name,
			}
			if description != "" {
				req.Description = &description
			}
			if branch != "" {
				req.DefaultBranch = &branch
			}

			timeout := getDurationFlag(cmd, "timeout", viperKeyTimeout)
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			result, err := c.CreateRepository(ctx, req)
			if err != nil {
				code := determineErrorCode(err)
				_ = client.WriteError(cmd.OutOrStdout(), code, err.Error(), nil)
				return nil
			}

			wait, _ := cmd.Flags().GetBool("wait")
			if wait {
				pollInterval := getDurationFlag(cmd, "poll-interval", viperKeyPollInterval)
				waitTimeout := getDurationFlag(cmd, "wait-timeout", viperKeyWaitTimeout)

				pollerConfig := &client.PollerConfig{
					Interval: pollInterval,
					MaxWait:  waitTimeout,
				}

				poller, err := client.NewPoller(c, pollerConfig)
				if err != nil {
					code := determineErrorCode(err)
					_ = client.WriteError(cmd.OutOrStdout(), code, err.Error(), nil)
					return nil
				}

				result, err = poller.WaitForCompletion(ctx, result.ID, cmd.ErrOrStderr())
				if err != nil {
					code := determineErrorCode(err)
					_ = client.WriteError(cmd.OutOrStdout(), code, err.Error(), nil)
					return nil
				}
			}

			return client.WriteSuccess(cmd.OutOrStdout(), result)
		},
	}

	cmd.Flags().String("name", "", "Custom name for the repository")
	cmd.Flags().String("description", "", "Description of the repository")
	cmd.Flags().String("branch", "", "Default branch to index")
	cmd.Flags().BoolP("wait", "w", false, "Wait for repository indexing to complete")
	cmd.Flags().Duration("poll-interval", client.DefaultPollInterval, "Interval between status polls")
	cmd.Flags().Duration("wait-timeout", client.DefaultMaxWait, "Maximum time to wait for completion")

	return cmd
}

// NewReposDeleteCmd creates and returns the repos delete command.
// The command deletes (archives) a repository by UUID.
func NewReposDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a repository by ID",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				_ = client.WriteError(cmd.OutOrStdout(), errCodeInvalidArgument, errMsgRequiresOneArgument, nil)
				return nil
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return nil
			}

			id, ok := parseUUIDArg(cmd, args[0], errMsgInvalidUUID)
			if !ok {
				return nil
			}

			c, ok := createClientFromFlags(cmd, cmd.OutOrStdout())
			if !ok {
				return nil
			}

			timeout := getDurationFlag(cmd, "timeout", viperKeyTimeout)
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			if err := c.DeleteRepository(ctx, id); err != nil {
				code := determineErrorCode(err)
				_ = client.WriteError(cmd.OutOrStdout(), code, err.Error(), nil)
				return nil
			}

			return client.WriteSuccess(cmd.OutOrStdout(), map[string]string{
				"id":     id.String(),
				"status": string(valueobject.RepositoryStatusArchived),
			})
		},
	}

	return cmd
}

// NewReposJobsCmd creates and returns the repos jobs command.
// The command lists all indexing jobs for a repository by UUID.
func NewReposJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "List indexing jobs for a repository",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				_ = client.WriteError(cmd.OutOrStdout(), errCodeInvalidArgument, errMsgRequiresOneArgument, nil)
				return nil
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return nil
			}

			id, ok := parseUUIDArg(cmd, args[0], errMsgInvalidUUID)
			if !ok {
				return nil
			}

			c, ok := createClientFromFlags(cmd, cmd.OutOrStdout())
			if !ok {
				return nil
			}

			limit, _ := cmd.Flags().GetInt("limit")
			offset, _ := cmd.Flags().GetInt("offset")

			query := dto.IndexingJobListQuery{Limit: limit, Offset: offset}

			timeout := getDurationFlag(cmd, "timeout", viperKeyTimeout)
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			result, err := c.ListRepositoryJobs(ctx, id, query)
			if err != nil {
				code := determineErrorCode(err)
				_ = client.WriteError(cmd.OutOrStdout(), code, err.Error(), nil)
				return nil
			}

			return client.WriteSuccess(cmd.OutOrStdout(), result)
		},
	}

	cmd.Flags().IntP("limit", "l", dto.DefaultIndexingJobListLimit, "Maximum number of jobs to return")
	cmd.Flags().IntP("offset", "o", 0, "Number of jobs to skip for pagination")

	return cmd
}

// NewReposQueryCmd creates and returns the repos query command.
// The command performs a semantic code search scoped to a specific repository.
// This is a convenient alternative to `search "query" --repo-ids <uuid>` for
// querying a single repository by ID.
//
// Example usage:
//
//	codechunking-client repos query <repo-id> "find authentication logic"
//	codechunking-client repos query <repo-id> "error handling" --limit 5 --languages go
func NewReposQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Search code within a repository",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				_ = client.WriteError(cmd.OutOrStdout(), errCodeInvalidArgument, "requires exactly 2 arguments: <repo-id> <query>", nil)
				return nil
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return nil
			}

			id, ok := parseUUIDArg(cmd, args[0], errMsgInvalidUUID)
			if !ok {
				return nil
			}

			c, ok := createClientFromFlags(cmd, cmd.OutOrStdout())
			if !ok {
				return nil
			}

			limit := getIntFlag(cmd, "limit", viperKeySearchLimit)
			offset, _ := cmd.Flags().GetInt("offset")
			threshold := getFloat64Flag(cmd, "threshold", viperKeySearchThresh)
			languages, _ := cmd.Flags().GetStringSlice("languages")
			types, _ := cmd.Flags().GetStringSlice("types")
			sort := getStringFlag(cmd, "sort", viperKeySearchSort)

			req := dto.SearchRequestDTO{
				Query:               args[1],
				RepositoryIDs:       []uuid.UUID{id},
				Limit:               limit,
				Offset:              offset,
				Languages:           languages,
				Types:               types,
				SimilarityThreshold: threshold,
				Sort:                sort,
			}

			timeout := getDurationFlag(cmd, "timeout", viperKeyTimeout)
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			result, err := c.Search(ctx, req)
			if err != nil {
				code := determineErrorCode(err)
				_ = client.WriteError(cmd.OutOrStdout(), code, err.Error(), nil)
				return nil
			}

			return client.WriteSuccess(cmd.OutOrStdout(), result)
		},
	}

	cmd.Flags().IntP("limit", "l", dto.DefaultSearchLimit, "Maximum number of results to return")
	cmd.Flags().Int("offset", 0, "Number of results to skip for pagination")
	cmd.Flags().Float64P("threshold", "t", dto.DefaultSimilarityThreshold, "Minimum similarity threshold (0.0-1.0)")
	cmd.Flags().StringSlice("languages", nil, "Filter by programming languages (e.g., go,python)")
	cmd.Flags().StringSlice("types", nil, "Filter by semantic types (e.g., function,class)")
	cmd.Flags().StringP("sort", "s", dto.DefaultSearchSort, "Sort order for results (similarity:desc, similarity:asc, file_path:asc, file_path:desc)")

	return cmd
}
