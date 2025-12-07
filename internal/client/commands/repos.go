package commands

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/client"
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

// createClientFromFlags creates and validates a client from command flags.
// It handles configuration validation and client creation, writing errors
// to the provided output if validation or creation fails.
// Returns the client and true if successful, or nil and false on failure.
func createClientFromFlags(cmd *cobra.Command, out io.Writer) (*client.Client, bool) {
	apiURL, _ := cmd.Flags().GetString("api-url")
	timeout, _ := cmd.Flags().GetDuration("timeout")

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
func NewReposCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repos",
		Short: "Manage repositories",
	}

	// Add subcommands
	cmd.AddCommand(NewReposListCmd())
	cmd.AddCommand(NewReposGetCmd())
	cmd.AddCommand(NewReposAddCmd())

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

			timeout, _ := cmd.Flags().GetDuration("timeout")
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

	// Add flags for filtering and pagination
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

			id, err := uuid.Parse(args[0])
			if err != nil {
				_ = client.WriteError(
					cmd.OutOrStdout(),
					errCodeInvalidArgument,
					errMsgInvalidUUID,
					nil,
				)
				return nil
			}

			c, ok := createClientFromFlags(cmd, cmd.OutOrStdout())
			if !ok {
				return nil
			}

			timeout, _ := cmd.Flags().GetDuration("timeout")
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

			timeout, _ := cmd.Flags().GetDuration("timeout")
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
				pollInterval, _ := cmd.Flags().GetDuration("poll-interval")
				waitTimeout, _ := cmd.Flags().GetDuration("wait-timeout")

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

	// Add optional flags
	cmd.Flags().String("name", "", "Custom name for the repository")
	cmd.Flags().String("description", "", "Description of the repository")
	cmd.Flags().String("branch", "", "Default branch to index")
	cmd.Flags().BoolP("wait", "w", false, "Wait for repository indexing to complete")
	cmd.Flags().Duration("poll-interval", client.DefaultPollInterval, "Interval between status polls")
	cmd.Flags().Duration("wait-timeout", client.DefaultMaxWait, "Maximum time to wait for completion")

	return cmd
}
