package commands

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/client"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// newTestCmd creates a bare cobra command with a no-op RunE. Individual tests
// are responsible for registering the flags they need via cmd.Flags().
func newTestCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{
		Use: "test",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	return cmd
}

// TestGetStringFlag_FlagBeatsEnvVar verifies that an explicitly-changed flag
// value takes precedence over an env-var that viper would otherwise surface.
func TestGetStringFlag_FlagBeatsEnvVar(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_API_URL", "http://env.example.com:9090")

	cmd := newTestCmd(t)
	cmd.Flags().String("api-url", client.DefaultAPIURL, "API server URL")
	// Simulate the flag being explicitly provided on the command line.
	if err := cmd.Flags().Set("api-url", "http://flag.example.com"); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}

	v := newClientViper(cmd)
	_ = v // ensure viper is initialised with env bindings

	got := getStringFlag(cmd, "api-url", viperKeyAPIURL)
	want := "http://flag.example.com"
	if got != want {
		t.Errorf("getStringFlag() = %q, want %q (flag value must win over env var)", got, want)
	}
}

// TestGetStringFlag_EnvVarBeatsDefault verifies that an env var overrides the
// compiled-in default when the flag has not been explicitly changed.
func TestGetStringFlag_EnvVarBeatsDefault(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_API_URL", "http://env.example.com:9090")

	cmd := newTestCmd(t)
	cmd.Flags().String("api-url", client.DefaultAPIURL, "API server URL")
	// Flag is NOT marked as Changed.

	_ = newClientViper(cmd)

	got := getStringFlag(cmd, "api-url", viperKeyAPIURL)
	want := "http://env.example.com:9090"
	if got != want {
		t.Errorf("getStringFlag() = %q, want %q (env var must win over default)", got, want)
	}
}

// TestGetStringFlag_DefaultWhenNothingSet verifies that the compiled-in default
// is returned when neither flag nor env var is set.
func TestGetStringFlag_DefaultWhenNothingSet(t *testing.T) {
	// Ensure env var is absent for this test.
	t.Setenv("CODECHUNK_CLIENT_API_URL", "")

	cmd := newTestCmd(t)
	cmd.Flags().String("api-url", client.DefaultAPIURL, "API server URL")
	// Flag is NOT marked as Changed.

	_ = newClientViper(cmd)

	got := getStringFlag(cmd, "api-url", viperKeyAPIURL)
	want := client.DefaultAPIURL // "http://localhost:8080"
	if got != want {
		t.Errorf("getStringFlag() = %q, want %q (default must be returned)", got, want)
	}
}

// TestGetDurationFlag_FlagBeatsEnvVar verifies that an explicitly-changed
// duration flag wins over the env var.
func TestGetDurationFlag_FlagBeatsEnvVar(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_TIMEOUT", "2m")

	cmd := newTestCmd(t)
	cmd.Flags().Duration("timeout", client.DefaultTimeout, "Request timeout")
	if err := cmd.Flags().Set("timeout", "45s"); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}

	_ = newClientViper(cmd)

	got := getDurationFlag(cmd, "timeout", viperKeyTimeout)
	want := 45 * time.Second
	if got != want {
		t.Errorf("getDurationFlag() = %v, want %v (flag value must win over env var)", got, want)
	}
}

// TestGetDurationFlag_EnvVarBeatsDefault verifies that an env var overrides
// the default when the duration flag has not been explicitly changed.
func TestGetDurationFlag_EnvVarBeatsDefault(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_TIMEOUT", "2m")

	cmd := newTestCmd(t)
	cmd.Flags().Duration("timeout", client.DefaultTimeout, "Request timeout")
	// Flag is NOT marked as Changed.

	_ = newClientViper(cmd)

	got := getDurationFlag(cmd, "timeout", viperKeyTimeout)
	want := 2 * time.Minute
	if got != want {
		t.Errorf("getDurationFlag() = %v, want %v (env var must win over default)", got, want)
	}
}

// TestGetDurationFlag_DefaultWhenNothingSet verifies that the compiled-in
// default (30s) is returned when neither flag nor env var is set.
func TestGetDurationFlag_DefaultWhenNothingSet(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_TIMEOUT", "")

	cmd := newTestCmd(t)
	cmd.Flags().Duration("timeout", client.DefaultTimeout, "Request timeout")
	// Flag is NOT marked as Changed.

	_ = newClientViper(cmd)

	got := getDurationFlag(cmd, "timeout", viperKeyTimeout)
	want := client.DefaultTimeout // 30s
	if got != want {
		t.Errorf("getDurationFlag() = %v, want %v (default must be returned)", got, want)
	}
}

// TestGetIntFlag_FlagBeatsEnvVar verifies that an explicitly-changed int flag
// takes precedence over the viper/env value for the same key.
func TestGetIntFlag_FlagBeatsEnvVar(t *testing.T) {
	// Map: CODECHUNK_CLIENT_SEARCH_LIMIT → client.search.limit
	t.Setenv("CODECHUNK_CLIENT_SEARCH_LIMIT", "50")

	cmd := newTestCmd(t)
	cmd.Flags().Int("limit", dto.DefaultSearchLimit, "Result limit")
	if err := cmd.Flags().Set("limit", "25"); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}

	_ = newClientViper(cmd)

	got := getIntFlag(cmd, "limit", viperKeySearchLimit)
	want := 25
	if got != want {
		t.Errorf("getIntFlag() = %d, want %d (flag value must win over env var)", got, want)
	}
}

// TestGetIntFlag_EnvVarBeatsDefault verifies that an env var overrides the
// default when the int flag has not been explicitly changed.
func TestGetIntFlag_EnvVarBeatsDefault(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_SEARCH_LIMIT", "50")

	cmd := newTestCmd(t)
	cmd.Flags().Int("limit", dto.DefaultSearchLimit, "Result limit")
	// Flag is NOT marked as Changed.

	_ = newClientViper(cmd)

	got := getIntFlag(cmd, "limit", viperKeySearchLimit)
	want := 50
	if got != want {
		t.Errorf("getIntFlag() = %d, want %d (env var must win over default)", got, want)
	}
}

// TestGetFloat64Flag_FlagBeatsEnvVar verifies that an explicitly-changed
// float64 flag takes precedence over the viper/env value.
func TestGetFloat64Flag_FlagBeatsEnvVar(t *testing.T) {
	// Map: CODECHUNK_CLIENT_SEARCH_THRESHOLD → client.search.threshold
	t.Setenv("CODECHUNK_CLIENT_SEARCH_THRESHOLD", "0.9")

	cmd := newTestCmd(t)
	cmd.Flags().Float64("threshold", dto.DefaultSimilarityThreshold, "Similarity threshold")
	if err := cmd.Flags().Set("threshold", "0.5"); err != nil {
		t.Fatalf("failed to set flag: %v", err)
	}

	_ = newClientViper(cmd)

	got := getFloat64Flag(cmd, "threshold", viperKeySearchThresh)
	want := 0.5
	if got != want {
		t.Errorf("getFloat64Flag() = %v, want %v (flag value must win over env var)", got, want)
	}
}

// TestGetFloat64Flag_EnvVarBeatsDefault verifies that an env var overrides the
// default when the float64 flag has not been explicitly changed.
func TestGetFloat64Flag_EnvVarBeatsDefault(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_SEARCH_THRESHOLD", "0.9")

	cmd := newTestCmd(t)
	cmd.Flags().Float64("threshold", dto.DefaultSimilarityThreshold, "Similarity threshold")
	// Flag is NOT marked as Changed.

	_ = newClientViper(cmd)

	got := getFloat64Flag(cmd, "threshold", viperKeySearchThresh)
	want := 0.9
	if got != want {
		t.Errorf("getFloat64Flag() = %v, want %v (env var must win over default)", got, want)
	}
}

// TestViperFromCmd_ReturnsNilWhenNoViper verifies that viperFromCmd returns nil
// when the command context carries no viper instance (i.e. newClientViper was
// never called for this command).
func TestViperFromCmd_ReturnsNilWhenNoViper(t *testing.T) {
	t.Parallel()

	cmd := newTestCmd(t)
	// Deliberately do NOT call newClientViper – context has no viper stored.

	got := viperFromCmd(cmd)
	if got != nil {
		t.Errorf("viperFromCmd() = %v, want nil for a command with no viper context", got)
	}
}

// TestNewClientViper_SetsDefaults verifies that after calling newClientViper the
// returned viper instance has the expected compiled-in defaults for api_url and
// timeout, assuming no env vars override them.
func TestNewClientViper_SetsDefaults(t *testing.T) {
	// Unset env vars that would otherwise shadow defaults.
	t.Setenv("CODECHUNK_CLIENT_API_URL", "")
	t.Setenv("CODECHUNK_CLIENT_TIMEOUT", "")

	cmd := newTestCmd(t)
	v := newClientViper(cmd)

	if v == nil {
		t.Fatal("newClientViper() returned nil")
	}

	gotURL := v.GetString(viperKeyAPIURL)
	wantURL := client.DefaultAPIURL
	if gotURL != wantURL {
		t.Errorf("viper.GetString(%q) = %q, want %q", viperKeyAPIURL, gotURL, wantURL)
	}

	gotTimeout := v.GetDuration(viperKeyTimeout)
	wantTimeout := client.DefaultTimeout // 30s
	if gotTimeout != wantTimeout {
		t.Errorf("viper.GetDuration(%q) = %v, want %v", viperKeyTimeout, gotTimeout, wantTimeout)
	}
}

// TestNewClientViper_SetsSearchDefaults verifies that search-related defaults
// are populated from the dto constants.
func TestNewClientViper_SetsSearchDefaults(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_SEARCH_LIMIT", "")
	t.Setenv("CODECHUNK_CLIENT_SEARCH_THRESHOLD", "")
	t.Setenv("CODECHUNK_CLIENT_SEARCH_SORT", "")

	cmd := newTestCmd(t)
	v := newClientViper(cmd)

	if v == nil {
		t.Fatal("newClientViper() returned nil")
	}

	gotLimit := v.GetInt(viperKeySearchLimit)
	wantLimit := dto.DefaultSearchLimit
	if gotLimit != wantLimit {
		t.Errorf("viper.GetInt(%q) = %d, want %d", viperKeySearchLimit, gotLimit, wantLimit)
	}

	gotThreshold := v.GetFloat64(viperKeySearchThresh)
	wantThreshold := dto.DefaultSimilarityThreshold
	if gotThreshold != wantThreshold {
		t.Errorf("viper.GetFloat64(%q) = %v, want %v", viperKeySearchThresh, gotThreshold, wantThreshold)
	}

	gotSort := v.GetString(viperKeySearchSort)
	wantSort := dto.DefaultSearchSort
	if gotSort != wantSort {
		t.Errorf("viper.GetString(%q) = %q, want %q", viperKeySearchSort, gotSort, wantSort)
	}
}

// TestNewClientViper_SetsPollingDefaults verifies that polling-related defaults
// are populated from the client constants.
func TestNewClientViper_SetsPollingDefaults(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_REPOS_POLL_INTERVAL", "")
	t.Setenv("CODECHUNK_CLIENT_REPOS_WAIT_TIMEOUT", "")

	cmd := newTestCmd(t)
	v := newClientViper(cmd)

	if v == nil {
		t.Fatal("newClientViper() returned nil")
	}

	gotPoll := v.GetDuration(viperKeyPollInterval)
	wantPoll := client.DefaultPollInterval
	if gotPoll != wantPoll {
		t.Errorf("viper.GetDuration(%q) = %v, want %v", viperKeyPollInterval, gotPoll, wantPoll)
	}

	gotWait := v.GetDuration(viperKeyWaitTimeout)
	wantWait := client.DefaultMaxWait
	if gotWait != wantWait {
		t.Errorf("viper.GetDuration(%q) = %v, want %v", viperKeyWaitTimeout, gotWait, wantWait)
	}
}

// TestNewClientViper_StoredInCmdContext verifies that viperFromCmd returns the
// same *viper.Viper that was created by newClientViper.
func TestNewClientViper_StoredInCmdContext(t *testing.T) {
	t.Parallel()

	cmd := newTestCmd(t)
	created := newClientViper(cmd)

	retrieved := viperFromCmd(cmd)
	if retrieved == nil {
		t.Fatal("viperFromCmd() returned nil after newClientViper was called")
	}
	if created != retrieved {
		t.Errorf("viperFromCmd() returned a different *viper.Viper instance than newClientViper(); "+
			"want the same pointer: created=%p retrieved=%p", created, retrieved)
	}
}

// TestValidateViperValues_ValidDefaults verifies that validateViperValues
// succeeds when only the compiled-in defaults are set (no env vars).
func TestValidateViperValues_ValidDefaults(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_TIMEOUT", "")
	t.Setenv("CODECHUNK_CLIENT_REPOS_POLL_INTERVAL", "")
	t.Setenv("CODECHUNK_CLIENT_REPOS_WAIT_TIMEOUT", "")
	t.Setenv("CODECHUNK_CLIENT_SEARCH_LIMIT", "")
	t.Setenv("CODECHUNK_CLIENT_SEARCH_THRESHOLD", "")

	cmd := newTestCmd(t)
	v := newClientViper(cmd)

	if err := validateViperValues(v); err != nil {
		t.Errorf("validateViperValues() returned unexpected error for valid defaults: %v", err)
	}
}

// TestValidateViperValues_InvalidDuration verifies that validateViperValues
// returns a descriptive error when a duration env var is set to an
// unparseable value.
func TestValidateViperValues_InvalidDuration(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_TIMEOUT", "not-a-duration")

	cmd := newTestCmd(t)
	v := newClientViper(cmd)

	err := validateViperValues(v)
	if err == nil {
		t.Fatal("validateViperValues() returned nil, want error for invalid duration")
	}
}

// TestValidateViperValues_InvalidInt verifies that validateViperValues returns a
// descriptive error when an integer env var is set to an unparseable value.
func TestValidateViperValues_InvalidInt(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_SEARCH_LIMIT", "not-an-int")

	cmd := newTestCmd(t)
	v := newClientViper(cmd)

	err := validateViperValues(v)
	if err == nil {
		t.Fatal("validateViperValues() returned nil, want error for invalid integer")
	}
}

// TestValidateViperValues_InvalidFloat verifies that validateViperValues returns
// a descriptive error when a float env var is set to an unparseable value.
func TestValidateViperValues_InvalidFloat(t *testing.T) {
	t.Setenv("CODECHUNK_CLIENT_SEARCH_THRESHOLD", "not-a-float")

	cmd := newTestCmd(t)
	v := newClientViper(cmd)

	err := validateViperValues(v)
	if err == nil {
		t.Fatal("validateViperValues() returned nil, want error for invalid float")
	}
}
