package commands

import (
	"codechunking/internal/client"
	"context"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// Error messages for jobs command argument validation.
const (
	errMsgRequiresTwoArguments = "requires exactly 2 arguments"
	errMsgInvalidJobID         = "invalid job ID: must be a valid UUID"
)

// requiredJobsGetArgs is the number of arguments required for jobs get command.
const requiredJobsGetArgs = 2

// NewJobsCmd creates and returns the jobs parent command.
// The command provides subcommands for managing indexing jobs,
// including retrieving job status and monitoring job progress.
//
// Available subcommands:
//   - get: Retrieve a specific indexing job by repository ID and job ID
//
// Example usage:
//
//	codechunk jobs get <repo-id> <job-id>
//	codechunk jobs get 123e4567-e89b-12d3-a456-426614174000 987fcdeb-51a2-43d7-b890-123456789abc
func NewJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Manage job operations",
	}

	cmd.AddCommand(NewJobsGetCmd())

	return cmd
}

// NewJobsGetCmd creates and returns the jobs get subcommand.
// The command retrieves detailed information about a specific indexing job,
// including its status, progress metrics, and error messages (if any).
//
// The command requires exactly two arguments:
//   - repository-id: UUID of the repository
//   - job-id: UUID of the indexing job
//
// Both arguments must be valid UUIDs or the command will return a validation error.
//
// Example usage:
//
//	codechunk jobs get 123e4567-e89b-12d3-a456-426614174000 987fcdeb-51a2-43d7-b890-123456789abc
//	codechunk jobs get <repo-id> <job-id> --timeout 10s
//
// The command writes JSON output to stdout:
//   - Success: {"success": true, "data": {"id": "...", "status": "...", ...}}
//   - Failure: {"success": false, "error": {...}}
//
// Job status can be one of: pending, processing, completed, failed.
// Completed and failed jobs include duration and completion timestamp.
// Failed jobs include an error message with failure details.
func NewJobsGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a job by repository ID and job ID",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != requiredJobsGetArgs {
				_ = client.WriteError(cmd.OutOrStdout(), errCodeInvalidArgument, errMsgRequiresTwoArguments, nil)
				return nil
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Skip execution if validation already failed.
			// This check ensures we don't proceed after Args validation error.
			if len(args) != requiredJobsGetArgs {
				return nil
			}

			// Parse and validate repository ID UUID.
			// Invalid UUIDs are reported immediately without attempting the API call.
			repoID, err := uuid.Parse(args[0])
			if err != nil {
				_ = client.WriteError(
					cmd.OutOrStdout(),
					errCodeInvalidArgument,
					errMsgInvalidUUID,
					nil,
				)
				return nil
			}

			// Parse and validate job ID UUID.
			// Invalid UUIDs are reported immediately without attempting the API call.
			jobID, err := uuid.Parse(args[1])
			if err != nil {
				_ = client.WriteError(
					cmd.OutOrStdout(),
					errCodeInvalidArgument,
					errMsgInvalidJobID,
					nil,
				)
				return nil
			}

			// Create API client from global flags (--api-url, etc.).
			// Returns false if client creation fails (already writes error).
			c, ok := createClientFromFlags(cmd, cmd.OutOrStdout())
			if !ok {
				return nil
			}

			// Apply timeout from global --timeout flag.
			// Context ensures the API call respects the timeout.
			timeout, _ := cmd.Flags().GetDuration("timeout")
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			// Retrieve job from API and handle response.
			// Error codes are determined based on HTTP status and error type.
			result, err := c.GetIndexingJob(ctx, repoID, jobID)
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
