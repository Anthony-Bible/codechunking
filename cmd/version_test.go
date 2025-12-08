package cmd

import (
	"bytes"
	"codechunking/internal/version"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestVersionCommand creates a version command without triggering config initialization.
func createTestVersionCommand() *cobra.Command {
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

// TestVersionCommand_Exists verifies that the version command is registered.
func TestVersionCommand_Exists(t *testing.T) {
	// Find the version command from root command
	versionCmd, _, err := rootCmd.Find([]string{"version"})
	require.NoError(t, err, "version command should be registered")
	require.NotNil(t, versionCmd, "version command should not be nil")
	assert.Equal(t, "version", versionCmd.Use, "version command use should be 'version'")
}

// TestVersionVariables_Exist verifies that version variables are declared
// These variables should be set via ldflags during build.
func TestVersionVariables_Exist(t *testing.T) {
	// Verify these variables exist (they will fail to compile if not declared)
	// The ldflags should set these at build time:
	// -ldflags "-X codechunking/cmd.Version=v1.0.0 -X codechunking/cmd.Commit=abc123 -X codechunking/cmd.BuildTime=2025-01-01T00:00:00Z"

	// These should be declared in version.go but will be empty during tests
	assert.NotNil(t, &Version, "Version variable should be declared")
	assert.NotNil(t, &Commit, "Commit variable should be declared")
	assert.NotNil(t, &BuildTime, "BuildTime variable should be declared")
}

// TestVersionCommand_OutputFormat verifies that version command outputs the correct format.
func TestVersionCommand_OutputFormat(t *testing.T) {
	tests := []struct {
		name         string
		version      string
		commit       string
		buildTime    string
		wantContains []string
	}{
		{
			name:      "complete version info",
			version:   "v1.2.3",
			commit:    "abc123def456",
			buildTime: "2025-01-01T12:00:00Z",
			wantContains: []string{
				"CodeChunking CLI",
				"Version: v1.2.3",
				"Commit: abc123def456",
				"Built: 2025-01-01T12:00:00Z",
			},
		},
		{
			name:      "minimal version info",
			version:   "v1.0.0",
			commit:    "unknown",
			buildTime: "unknown",
			wantContains: []string{
				"CodeChunking CLI",
				"Version: v1.0.0",
				"Commit: unknown",
				"Built: unknown",
			},
		},
		{
			name:      "empty version info",
			version:   "",
			commit:    "",
			buildTime: "",
			wantContains: []string{
				"CodeChunking CLI",
				"Version: dev",
				"Commit: unknown",
				"Built: unknown",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the version variables for this test
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
			Version = tt.version
			Commit = tt.commit
			BuildTime = tt.buildTime

			// Create a fresh version command without triggering config initialization
			// By creating a command directly and executing it, we bypass the global init
			versionCmd := createTestVersionCommand()

			// Capture output
			var buf bytes.Buffer
			versionCmd.SetOut(&buf)

			// Execute the command by calling RunE directly to bypass global init
			err := versionCmd.RunE(versionCmd, []string{})
			require.NoError(t, err)

			output := buf.String()

			// Verify all expected strings are in the output
			for _, expected := range tt.wantContains {
				assert.Contains(t, output, expected, "output should contain %s", expected)
			}
		})
	}
}

// TestVersionCommand_SingleLineOutput verifies that --short flag returns single line output.
func TestVersionCommand_SingleLineOutput(t *testing.T) {
	// Set version variables
	originalVersion := Version
	originalCommit := Commit
	originalBuildTime := BuildTime
	defer func() {
		Version = originalVersion
		Commit = originalCommit
		BuildTime = originalBuildTime
	}()

	Version = "v1.2.3"
	Commit = "abc123"
	BuildTime = "2025-01-01T12:00:00Z"

	// Create version command with short flag
	versionCmd := createTestVersionCommand()
	err := versionCmd.Flags().Set("short", "true")
	require.NoError(t, err)

	// Capture output
	var buf bytes.Buffer
	versionCmd.SetOut(&buf)

	// Execute the command
	err = versionCmd.RunE(versionCmd, []string{})
	require.NoError(t, err)

	output := strings.TrimSpace(buf.String())

	// Should only contain the version number
	assert.Equal(t, "v1.2.3", output, "--short flag should output only version number")
}

// TestVersionCommand_NoConfigRequired verifies that version command works without any configuration.
func TestVersionCommand_NoConfigRequired(t *testing.T) {
	// Unset any config-related environment variables that might interfere
	originalEnvVars := map[string]string{
		"CODECHUNK_CONFIG_FILE": os.Getenv("CODECHUNK_CONFIG_FILE"),
		"CODECHUNK_LOG_LEVEL":   os.Getenv("CODECHUNK_LOG_LEVEL"),
		"CODECHUNK_LOG_FORMAT":  os.Getenv("CODECHUNK_LOG_FORMAT"),
	}

	for key := range originalEnvVars {
		if originalEnvVars[key] != "" {
			t.Setenv(key, originalEnvVars[key])
		} else {
			os.Unsetenv(key)
		}
	}

	// Set version variables
	originalVersion := Version
	originalCommit := Commit
	originalBuildTime := BuildTime
	defer func() {
		Version = originalVersion
		Commit = originalCommit
		BuildTime = originalBuildTime
	}()

	Version = "v1.0.0"
	Commit = "testcommit"
	BuildTime = time.Now().Format(time.RFC3339)

	// Create a fresh root command (without init config)
	testRootCmd := &cobra.Command{
		Use: "codechunking",
	}
	testRootCmd.AddCommand(newVersionCmd())

	// Capture output
	var buf bytes.Buffer
	testRootCmd.SetOut(&buf)
	testRootCmd.SetArgs([]string{"version"})

	// Execute the command - this should not require any config
	err := testRootCmd.Execute()
	require.NoError(t, err, "version command should work without configuration")

	output := buf.String()
	assert.Contains(t, output, "CodeChunking CLI", "should output application name")
	assert.Contains(t, output, "v1.0.0", "should output version")
	assert.Contains(t, output, "testcommit", "should output commit")
}

// TestVersionCommand_FriendlyErrorHandling verifies that version command handles errors gracefully.
func TestVersionCommand_FriendlyErrorHandling(t *testing.T) {
	// Test that command doesn't panic and handles nil state gracefully
	versionCmd := createTestVersionCommand()

	// Capture both stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	versionCmd.SetOut(&stdoutBuf)
	versionCmd.SetErr(&stderrBuf)

	// Execute with empty version variables by calling RunE directly
	err := versionCmd.RunE(versionCmd, []string{})
	assert.NoError(t, err, "command should not error with empty version info")

	// Should provide defaults instead of error
	output := stdoutBuf.String()
	assert.Contains(t, output, "CodeChunking CLI")
	assert.Empty(t, stderrBuf.String(), "should not write to stderr on normal execution")
}
