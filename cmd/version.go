// Package cmd provides command-line interface functionality for the codechunking application.
/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"codechunking/internal/version"

	"github.com/spf13/cobra"
)

// Version information variables that will be set via ldflags during build.
// These are kept for backward compatibility with existing build processes and tests.
//
//nolint:gochecknoglobals // Required for backward compatibility with existing build systems.
var (
	// Version is the application version (e.g., v1.0.0).
	// This variable is primarily maintained for build systems that may still reference it.
	Version string
	// Commit is the git commit hash (e.g., abc123def456).
	// This variable is primarily maintained for build systems that may still reference it.
	Commit string
	// BuildTime is the build timestamp (e.g., 2025-01-01T12:00:00Z).
	// This variable is primarily maintained for build systems that may still reference it.
	BuildTime string
)

// newVersionCmd creates and returns the version command.
func newVersionCmd() *cobra.Command {
	var short bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long: `Show version information for the codechunking application.

This command displays the current version of the codechunking CLI tool,
which includes version number, build information, and other relevant details.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVersion(cmd, short)
		},
	}

	cmd.Flags().BoolVarP(&short, "short", "s", false, "Show only version number")
	return cmd
}

// runVersion implements the version command output using the refactored version package.
func runVersion(cmd *cobra.Command, short bool) error {
	// Sync legacy variables with version package for backward compatibility
	syncLegacyVersionVars()

	// Get version information from the centralized version package
	versionInfo := version.GetVersion()

	// Write the formatted version output
	return versionInfo.Write(cmd.OutOrStdout(), short)
}

// syncLegacyVersionVars synchronizes the legacy version variables with the version package.
// This ensures backward compatibility for any build processes or tests that may still
// set the legacy variables directly.
func syncLegacyVersionVars() {
	// Only set variables if at least one is non-empty.
	// SetBuildVars will overwrite any existing values, so no reset is needed.
	if Version != "" || Commit != "" || BuildTime != "" {
		version.SetBuildVars(Version, Commit, BuildTime)
	}
}

func init() { //nolint:gochecknoinits // Standard Cobra CLI pattern for command registration
	rootCmd.AddCommand(newVersionCmd())

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// versionCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// versionCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
