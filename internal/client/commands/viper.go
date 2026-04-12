package commands

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/client"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type contextKey string

const viperContextKey contextKey = "viper"

const (
	viperKeyAPIURL       = "client.api_url"
	viperKeyTimeout      = "client.timeout"
	viperKeySearchLimit  = "client.search.limit"
	viperKeySearchThresh = "client.search.threshold"
	viperKeySearchSort   = "client.search.sort"
	viperKeyPollInterval = "client.repos.poll_interval"
	viperKeyWaitTimeout  = "client.repos.wait_timeout"
)

// dotToUnderscore maps viper's dot-separated keys to env var underscore format.
var dotToUnderscore = strings.NewReplacer(".", "_") //nolint:gochecknoglobals // immutable replacer, avoids per-call allocation

// newClientViper creates a configured viper instance for the client commands
// and stores it in cmd.Context(). Config file reading is intentionally left to
// the caller (PersistentPreRunE) so a custom --config path can be applied first.
func newClientViper(cmd *cobra.Command) *viper.Viper {
	v := viper.New()

	v.SetDefault(viperKeyAPIURL, client.DefaultAPIURL)
	v.SetDefault(viperKeyTimeout, client.DefaultTimeout.String())
	v.SetDefault(viperKeySearchLimit, dto.DefaultSearchLimit)
	v.SetDefault(viperKeySearchThresh, dto.DefaultSimilarityThreshold)
	v.SetDefault(viperKeySearchSort, dto.DefaultSearchSort)
	v.SetDefault(viperKeyPollInterval, client.DefaultPollInterval.String())
	v.SetDefault(viperKeyWaitTimeout, client.DefaultMaxWait.String())

	v.SetEnvPrefix("CODECHUNK")
	v.SetEnvKeyReplacer(dotToUnderscore)
	v.AutomaticEnv()

	v.SetConfigName(".codechunking")
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(home)
	}
	v.AddConfigPath("./configs")
	v.AddConfigPath(".")

	base := cmd.Context()
	if base == nil {
		base = context.Background()
	}
	cmd.SetContext(context.WithValue(base, viperContextKey, v))

	return v
}

// viperFromCmd retrieves the viper instance stored in cmd.Context().
// Returns nil if no viper is stored or no context is available.
func viperFromCmd(cmd *cobra.Command) *viper.Viper {
	ctx := cmd.Context()
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(viperContextKey).(*viper.Viper); ok {
		return v
	}
	return nil
}

// flagOrViper returns the cobra flag value when the flag was explicitly set on
// the command line; otherwise falls back to viper for config-file / env-var /
// default lookup. It is the single source of truth for the flag-over-viper
// precedence logic shared by all typed helpers below.
// The viperKey is captured by the getViperVal closure in each typed helper;
// it is not needed as a separate parameter here.
func flagOrViper[T any](
	cmd *cobra.Command,
	flagName string,
	getFlagVal func() (T, error),
	getViperVal func(*viper.Viper) T,
) T {
	if f := cmd.Flags().Lookup(flagName); f != nil && f.Changed {
		val, _ := getFlagVal()
		return val
	}
	if v := viperFromCmd(cmd); v != nil {
		return getViperVal(v)
	}
	val, _ := getFlagVal()
	return val
}

// getStringFlag returns the flag value if explicitly set, otherwise falls back to viper.
func getStringFlag(cmd *cobra.Command, flagName, viperKey string) string {
	return flagOrViper(cmd, flagName,
		func() (string, error) { return cmd.Flags().GetString(flagName) },
		func(v *viper.Viper) string { return v.GetString(viperKey) },
	)
}

// getDurationFlag returns the flag value if explicitly set, otherwise falls back to viper.
func getDurationFlag(cmd *cobra.Command, flagName, viperKey string) time.Duration {
	return flagOrViper(cmd, flagName,
		func() (time.Duration, error) { return cmd.Flags().GetDuration(flagName) },
		func(v *viper.Viper) time.Duration { return v.GetDuration(viperKey) },
	)
}

// getIntFlag returns the flag value if explicitly set, otherwise falls back to viper.
func getIntFlag(cmd *cobra.Command, flagName, viperKey string) int { //nolint:unparam // flagName is a generic parameter by design
	return flagOrViper(cmd, flagName,
		func() (int, error) { return cmd.Flags().GetInt(flagName) },
		func(v *viper.Viper) int { return v.GetInt(viperKey) },
	)
}

// getFloat64Flag returns the flag value if explicitly set, otherwise falls back to viper.
func getFloat64Flag(cmd *cobra.Command, flagName, viperKey string) float64 { //nolint:unparam // flagName is a generic parameter by design
	return flagOrViper(cmd, flagName,
		func() (float64, error) { return cmd.Flags().GetFloat64(flagName) },
		func(v *viper.Viper) float64 { return v.GetFloat64(viperKey) },
	)
}

// validateViperKeys checks that every non-empty viper string for the given keys
// can be parsed by parse. It returns a descriptive error on the first failure.
func validateViperKeys(v *viper.Viper, parse func(string) error, keys ...string) error {
	for _, key := range keys {
		if raw := v.GetString(key); raw != "" {
			if err := parse(raw); err != nil {
				return fmt.Errorf("invalid value for config key %q: %q: %w", key, raw, err)
			}
		}
	}
	return nil
}

// validateViperValues checks that all duration, int, and float64 values stored
// in v are parseable. It returns an error describing the first invalid value
// found. This is called from PersistentPreRunE to surface misconfiguration early,
// before any command logic runs.
func validateViperValues(v *viper.Viper) error {
	parseDuration := func(s string) error { _, err := time.ParseDuration(s); return err }
	parseInt := func(s string) error { _, err := strconv.Atoi(s); return err }
	parseFloat := func(s string) error { _, err := strconv.ParseFloat(s, 64); return err }

	if err := validateViperKeys(v, parseDuration, viperKeyTimeout, viperKeyPollInterval, viperKeyWaitTimeout); err != nil {
		return err
	}
	if err := validateViperKeys(v, parseInt, viperKeySearchLimit); err != nil {
		return err
	}
	return validateViperKeys(v, parseFloat, viperKeySearchThresh)
}
