package commands_test

import (
	"bytes"
	"codechunking/internal/client"
	"codechunking/internal/client/commands"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRootCmd verifies that NewRootCmd creates a root command with correct Use and Short fields.
// This test ensures the root command is properly initialized with expected metadata.
func TestNewRootCmd(t *testing.T) {
	t.Parallel()

	cmd := commands.NewRootCmd()

	require.NotNil(t, cmd, "NewRootCmd should return a non-nil command")
	assert.Equal(t, "codechunking-client", cmd.Use, "root command Use should be 'codechunking-client'")
	assert.NotEmpty(t, cmd.Short, "root command Short description should not be empty")
	assert.Contains(t, cmd.Short, "CLI client", "Short description should mention it's a CLI client")
}

// TestRootCmd_HasHealthSubcommand verifies that the health command is registered as a subcommand.
// This test ensures the command structure is properly set up with the health command available.
func TestRootCmd_HasHealthSubcommand(t *testing.T) {
	t.Parallel()

	rootCmd := commands.NewRootCmd()

	// Find the health subcommand
	healthCmd := findSubcommand(rootCmd, "health")
	require.NotNil(t, healthCmd, "health command should be registered as a subcommand")
	assert.Equal(t, "health", healthCmd.Use, "health subcommand should have Use='health'")
}

// TestRootCmd_GlobalFlags verifies that the root command has required global flags.
// This test ensures --api-url and --timeout flags are available for all commands.
func TestRootCmd_GlobalFlags(t *testing.T) {
	t.Parallel()

	cmd := commands.NewRootCmd()

	// Check for --api-url flag
	apiURLFlag := cmd.PersistentFlags().Lookup("api-url")
	require.NotNil(t, apiURLFlag, "--api-url flag should be defined")
	assert.Equal(t, "string", apiURLFlag.Value.Type(), "--api-url should be a string flag")
	assert.NotEmpty(t, apiURLFlag.Usage, "--api-url should have usage description")
	assert.Equal(t, client.DefaultAPIURL, apiURLFlag.DefValue, "--api-url default should match client.DefaultAPIURL")

	// Check for --timeout flag
	timeoutFlag := cmd.PersistentFlags().Lookup("timeout")
	require.NotNil(t, timeoutFlag, "--timeout flag should be defined")
	assert.Equal(t, "duration", timeoutFlag.Value.Type(), "--timeout should be a duration flag")
	assert.NotEmpty(t, timeoutFlag.Usage, "--timeout should have usage description")
	assert.Equal(
		t,
		client.DefaultTimeout.String(),
		timeoutFlag.DefValue,
		"--timeout default should match client.DefaultTimeout",
	)
}

// TestRootCmd_FlagsSetConfig verifies that flags properly configure the client.
// This test ensures that when flags are set, they are correctly passed to the client configuration.
func TestRootCmd_FlagsSetConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            []string
		expectedURL     string
		expectedTimeout time.Duration
	}{
		{
			name:            "default values when no flags provided",
			args:            []string{},
			expectedURL:     client.DefaultAPIURL,
			expectedTimeout: client.DefaultTimeout,
		},
		{
			name:            "custom api-url flag",
			args:            []string{"--api-url", "http://example.com:9090"},
			expectedURL:     "http://example.com:9090",
			expectedTimeout: client.DefaultTimeout,
		},
		{
			name:            "custom timeout flag",
			args:            []string{"--timeout", "1m"},
			expectedURL:     client.DefaultAPIURL,
			expectedTimeout: 60 * time.Second,
		},
		{
			name:            "both flags custom",
			args:            []string{"--api-url", "https://api.example.com", "--timeout", "45s"},
			expectedURL:     "https://api.example.com",
			expectedTimeout: 45 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Create fresh command for each test
			cmd := commands.NewRootCmd()
			cmd.SetArgs(tt.args)

			// Parse flags
			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err, "ParseFlags should not return an error")

			// Get flag values
			apiURL, err := cmd.PersistentFlags().GetString("api-url")
			require.NoError(t, err, "Getting api-url flag should not error")
			assert.Equal(t, tt.expectedURL, apiURL, "api-url flag value should match expected")

			timeout, err := cmd.PersistentFlags().GetDuration("timeout")
			require.NoError(t, err, "Getting timeout flag should not error")
			assert.Equal(t, tt.expectedTimeout, timeout, "timeout flag value should match expected")
		})
	}
}

// TestRootCmd_Version verifies that the root command has version information.
// This test ensures version metadata is available for troubleshooting.
func TestRootCmd_Version(t *testing.T) {
	t.Parallel()

	cmd := commands.NewRootCmd()

	// Version should be set
	assert.NotEmpty(t, cmd.Version, "root command should have a version set")
}

// TestRootCmd_SilenceUsageOnError verifies error handling behavior.
// This test ensures that usage is not printed on command execution errors.
func TestRootCmd_SilenceUsageOnError(t *testing.T) {
	t.Parallel()

	cmd := commands.NewRootCmd()

	// SilenceUsage should be true to prevent double output
	assert.True(t, cmd.SilenceUsage, "SilenceUsage should be true to prevent usage on errors")
}

// TestRootCmd_ExecuteWithNoArgs verifies behavior when executed without arguments.
// This test ensures the root command shows help or handles no arguments gracefully.
func TestRootCmd_ExecuteWithNoArgs(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	cmd := commands.NewRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{})

	err := cmd.Execute()

	// Either should show help (no error) or have a defined behavior
	// The exact behavior depends on implementation, but should be consistent
	if err == nil {
		// If no error, output should contain help information
		output := stdout.String()
		assert.Contains(t, output, "Usage:", "output should contain usage information when no args provided")
	}
}

// TestRootCmd_InvalidFlag verifies that invalid flags are handled properly.
// This test ensures clear error messages for unknown flags.
func TestRootCmd_InvalidFlag(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	cmd := commands.NewRootCmd()
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--invalid-flag"})

	err := cmd.Execute()

	require.Error(t, err, "executing with invalid flag should return an error")
	assert.Contains(t, err.Error(), "unknown flag", "error message should indicate unknown flag")
}

// TestRootCmd_HelpFlag verifies that --help flag works correctly.
// This test ensures users can access help documentation.
func TestRootCmd_HelpFlag(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	cmd := commands.NewRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()

	require.NoError(t, err, "--help should not return an error")
	output := stdout.String()
	assert.Contains(t, output, "Usage:", "help output should contain Usage section")
	assert.Contains(t, output, "Available Commands:", "help output should list available commands")
	assert.Contains(t, output, "Flags:", "help output should list flags")
}

// Helper function to find a subcommand by name.
func findSubcommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, subCmd := range cmd.Commands() {
		if subCmd.Use == name || subCmd.Name() == name {
			return subCmd
		}
	}
	return nil
}
