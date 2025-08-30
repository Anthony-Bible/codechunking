// Package cmd provides command-line interface functionality for the codechunking application.
/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"codechunking/internal/application/common/slogger"

	"github.com/spf13/cobra"
)

// newVersionCmd creates and returns the version command.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long: `Show version information for the codechunking application.

This command displays the current version of the codechunking CLI tool,
which includes version number, build information, and other relevant details.`,
		Run: func(_ *cobra.Command, _ []string) {
			slogger.InfoNoCtx("version called", nil)
		},
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
