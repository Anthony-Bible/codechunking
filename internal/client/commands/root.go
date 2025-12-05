// Package commands provides CLI commands for the CodeChunking client.
// It implements the cobra-based command structure with global flags
// and subcommands for interacting with the CodeChunking API.
package commands

import (
	"codechunking/internal/client"

	"github.com/spf13/cobra"
)

const (
	// clientVersion is the current version of the CLI client.
	clientVersion = "1.0.0"
)

// Flag names for persistent global flags.
const (
	flagAPIURL  = "api-url"
	flagTimeout = "timeout"
)

// NewRootCmd creates and returns the root command for the CodeChunking CLI client.
// The root command establishes persistent flags (api-url, timeout) that are
// inherited by all subcommands. It sets SilenceUsage to true to prevent
// usage information from being printed on command execution errors.
//
// Subcommands:
//   - health: Check API server health status
//   - repos: Manage repositories (list, get, add)
//
// Global Flags:
//   - --api-url: API server URL (default: http://localhost:8080)
//   - --timeout: Request timeout duration (default: 30s)
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "codechunking-client",
		Short:        "CLI client for CodeChunking API",
		Version:      clientVersion,
		SilenceUsage: true,
	}

	cmd.PersistentFlags().String(flagAPIURL, client.DefaultAPIURL, "API server URL")
	cmd.PersistentFlags().Duration(flagTimeout, client.DefaultTimeout, "Request timeout")

	cmd.AddCommand(NewHealthCmd())
	cmd.AddCommand(NewReposCmd())
	cmd.AddCommand(NewSearchCmd())
	cmd.AddCommand(NewJobsCmd())

	return cmd
}
