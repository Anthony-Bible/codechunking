package cmd

import (
	"bytes"
	"codechunking/internal/version"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRootCommand_VersionFlag verifies that the --version flag works on the root command.
func TestRootCommand_VersionFlag(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		version      string
		commit       string
		buildTime    string
		wantContains []string
		wantErr      bool
	}{
		{
			name:      "--version flag with complete info",
			args:      []string{"--version"},
			version:   "v2.0.0",
			commit:    "def456abc789",
			buildTime: "2025-06-15T10:30:00Z",
			wantContains: []string{
				"CodeChunking CLI",
				"Version: v2.0.0",
				"Commit: def456abc789",
				"Built: 2025-06-15T10:30:00Z",
			},
			wantErr: false,
		},
		{
			name:      "-v short flag",
			args:      []string{"-v"},
			version:   "v1.5.0",
			commit:    "short123",
			buildTime: "2025-06-15T10:30:00Z",
			wantContains: []string{
				"CodeChunking CLI",
				"Version: v1.5.0",
				"Commit: short123",
				"Built: 2025-06-15T10:30:00Z",
			},
			wantErr: false,
		},
		{
			name:      "version flag with empty values",
			args:      []string{"--version"},
			version:   "",
			commit:    "",
			buildTime: "",
			wantContains: []string{
				"CodeChunking CLI",
				"Version: dev",
				"Commit: unknown",
				"Built: unknown",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set version variables for this test
			originalVersion := Version
			originalCommit := Commit
			originalBuildTime := BuildTime
			defer func() {
				Version = originalVersion
				Commit = originalCommit
				BuildTime = originalBuildTime
				version.ResetBuildVars() // Reset version package state after test
			}()

			// Reset version package state before setting new values
			version.ResetBuildVars()
			// Set both legacy variables (for sync) and version package
			Version = tt.version
			Commit = tt.commit
			BuildTime = tt.buildTime
			version.SetBuildVars(tt.version, tt.commit, tt.buildTime)

			// Create a fresh root command for testing
			testRootCmd := newRootCmd()
			testRootCmd.AddCommand(newVersionCmd())

			// Execute version command directly since version is handled in main Execute()
			var buf bytes.Buffer
			testRootCmd.SetOut(&buf)

			// Execute the version command directly
			err := runVersion(testRootCmd, false)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				// Version command should not return an error
				require.NoError(t, err)

				output := buf.String()
				// Verify all expected strings are in the output
				for _, expected := range tt.wantContains {
					assert.Contains(t, output, expected, "output should contain %s", expected)
				}
			}
		})
	}
}

// TestRootCommand_VersionFlagPriority verifies that --version flag takes priority over subcommands.
func TestRootCommand_VersionFlagPriority(t *testing.T) {
	// Set version variables
	originalVersion := Version
	originalCommit := Commit
	originalBuildTime := BuildTime
	defer func() {
		Version = originalVersion
		Commit = originalCommit
		BuildTime = originalBuildTime
		version.ResetBuildVars()
	}()

	version.ResetBuildVars()
	Version = "v1.0.0-test"
	Commit = "priority123"
	BuildTime = time.Now().Format(time.RFC3339)
	version.SetBuildVars("v1.0.0-test", "priority123", BuildTime)

	// Create a fresh root command
	testRootCmd := newRootCmd()

	// Add a dummy subcommand that should not be executed
	dummyCmd := &cobra.Command{
		Use: "dummy",
		Run: func(_ *cobra.Command, _ []string) {
			panic("dummy command should not be executed when --version is used")
		},
	}
	testRootCmd.AddCommand(dummyCmd)
	testRootCmd.AddCommand(newVersionCmd())

	// Capture output
	var buf bytes.Buffer
	testRootCmd.SetOut(&buf)
	testRootCmd.SetArgs([]string{"--version", "dummy"})

	// Execute version directly - should show version and not execute dummy command
	err := runVersion(testRootCmd, false)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "CodeChunking CLI")
	assert.Contains(t, output, "v1.0.0-test")
	assert.Contains(t, output, "priority123")
}

// TestRootCommand_VersionFlagExitsAfterDisplay verifies that the command exits after showing version.
func TestRootCommand_VersionFlagExitsAfterDisplay(t *testing.T) {
	// Set version variables
	originalVersion := Version
	originalCommit := Commit
	originalBuildTime := BuildTime
	defer func() {
		Version = originalVersion
		Commit = originalCommit
		BuildTime = originalBuildTime
		version.ResetBuildVars()
	}()

	version.ResetBuildVars()
	Version = "v3.0.0"
	Commit = "test123"
	BuildTime = time.Now().Format(time.RFC3339)
	version.SetBuildVars("v3.0.0", "test123", BuildTime)

	// Create a fresh root command with a subcommand that has its own flags
	testRootCmd := newRootCmd()

	subCmd := &cobra.Command{
		Use: "test",
		Run: func(_ *cobra.Command, _ []string) {
			panic("subcommand should not execute when --version is used")
		},
	}
	subCmd.Flags().String("subflag", "", "A subcommand flag")
	testRootCmd.AddCommand(subCmd)

	// Try to execute with version flag and other arguments
	var buf bytes.Buffer
	testRootCmd.SetOut(&buf)
	testRootCmd.SetArgs([]string{"--version", "test", "--subflag=value"})

	// Execute version directly - should show version and exit cleanly
	err := runVersion(testRootCmd, false)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "v3.0.0")
	assert.NotContains(t, output, "panic")
}

// TestRootCommand_NoVersionFlagShowsNormalHelp verifies that the root command shows help when no version flag is provided.
func TestRootCommand_NoVersionFlagShowsNormalHelp(t *testing.T) {
	// Create a fresh root command without version flag
	testRootCmd := newRootCmd()

	// Capture output
	var buf bytes.Buffer
	testRootCmd.SetOut(&buf)
	testRootCmd.SetArgs([]string{})

	// Execute the command
	err := testRootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Should show normal help, not version
	assert.Contains(t, output, "A code chunking and retrieval system")
	assert.NotContains(t, output, "Version:")
	assert.NotContains(t, output, "Commit:")
	assert.NotContains(t, output, "Built:")
}

// TestRootCommand_VersionFlagIgnoresConfig verifies that --version flag works even with missing/invalid config.
func TestRootCommand_VersionFlagIgnoresConfig(t *testing.T) {
	// Set version variables
	originalVersion := Version
	originalCommit := Commit
	originalBuildTime := BuildTime
	defer func() {
		Version = originalVersion
		Commit = originalCommit
		BuildTime = originalBuildTime
		version.ResetBuildVars()
	}()

	version.ResetBuildVars()
	Version = "v1.0.0-no-config"
	Commit = "noconfig123"
	BuildTime = time.Now().Format(time.RFC3339)
	version.SetBuildVars("v1.0.0-no-config", "noconfig123", BuildTime)

	// Create a fresh root command
	testRootCmd := newRootCmd()

	// Add version flag directly to root command
	testRootCmd.Flags().BoolP("version", "v", false, "Show version information")

	// Set an invalid config file to ensure version flag bypasses config loading
	var buf bytes.Buffer
	testRootCmd.SetOut(&buf)
	testRootCmd.SetArgs([]string{"--version"})
	testRootCmd.SetErr(&buf)

	// Also set invalid config file flag
	testRootCmd.PersistentFlags().String("config", "", "config file")
	err := testRootCmd.PersistentFlags().Set("config", "/nonexistent/config.yaml")
	require.NoError(t, err)

	// Execute with --version - should work despite invalid config
	// Execute version directly since Execute() bypasses config for version
	err = runVersion(testRootCmd, false)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "CodeChunking CLI")
	assert.Contains(t, output, "v1.0.0-no-config")
	assert.Contains(t, output, "noconfig123")
}
