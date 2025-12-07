// Package cmd provides command-line interface functionality for the codechunking application.
/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information variables that will be set via ldflags during build. //nolint:gochecknoglobals // Required for build-time injection.
var (
	Version   string // Example: v1.0.0
	Commit    string // Example: abc123def456
	BuildTime string // Example: 2025-01-01T12:00:00Z
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

// runVersion implements the version command output.
func runVersion(cmd *cobra.Command, short bool) error {
	// Provide default values for empty variables
	version := Version
	if version == "" {
		version = "dev"
	}

	commit := Commit
	if commit == "" {
		commit = "unknown"
	}

	buildTime := BuildTime
	if buildTime == "" {
		buildTime = "unknown"
	}

	if short {
		// Single line output for --short flag
		fmt.Fprintln(cmd.OutOrStdout(), version)
		return nil
	}

	// Multi-line output with application name and details
	fmt.Fprintf(cmd.OutOrStdout(), "CodeChunking CLI\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Version: %s\n", version)
	fmt.Fprintf(cmd.OutOrStdout(), "Commit: %s\n", commit)
	fmt.Fprintf(cmd.OutOrStdout(), "Built: %s\n", buildTime)

	return nil
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
