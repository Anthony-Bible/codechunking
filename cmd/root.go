package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"codechunking/internal/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// CmdConfig holds the command configuration.
type CmdConfig struct {
	cfgFile string
	cfg     *config.Config
}

var cmdConfig CmdConfig //nolint:gochecknoglobals // Standard CLI pattern for Cobra command configuration

// rootCmd represents the base command when called without any subcommands.
var rootCmd = newRootCmd() //nolint:gochecknoglobals // Standard Cobra CLI pattern for root command

// newRootCmd creates and returns the root command.
func newRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "codechunking",
		Short: "A code chunking and retrieval system",
		Long: `CodeChunking is a production-grade system for indexing code repositories,
generating embeddings, and providing semantic code search capabilities.

The system supports:
- Repository indexing via Git
- Intelligent code chunking using tree-sitter
- Embedding generation with Google Gemini
- Vector storage and similarity search with PostgreSQL/pgvector
- Asynchronous job processing with NATS JetStream`,
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().
		StringVar(&cmdConfig.cfgFile, "config", "", "config file (default: ./configs/config.yaml)")
	rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("log-format", "json", "Log format (json, text)")

	// Bind flags to viper
	if err := viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level")); err != nil {
		fmt.Fprintf(os.Stderr, "Error binding log-level flag: %v\n", err)
	}
	if err := viper.BindPFlag("log.format", rootCmd.PersistentFlags().Lookup("log-format")); err != nil {
		fmt.Fprintf(os.Stderr, "Error binding log-format flag: %v\n", err)
	}
}

func initConfig() {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Set config file
	if cmdConfig.cfgFile != "" {
		v.SetConfigFile(cmdConfig.cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	// Environment variables
	v.SetEnvPrefix("CODECHUNK")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicitly bind environment variables for middleware configuration
	// This ensures environment variables can override config file values for nested boolean pointers
	bindMiddlewareEnvVars(v)

	// Read configuration
	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
		}
		// Config file not found; use defaults and environment
	}

	// Load configuration
	cmdConfig.cfg = config.New(v)
}

// bindMiddlewareEnvVars explicitly binds middleware environment variables to Viper configuration keys.
//
// Problem Solved:
//
//	Viper's AutomaticEnv() doesn't always work correctly with nested boolean pointer fields,
//	especially when config file values are already set. This function ensures environment
//	variables can properly override config file settings for middleware configuration.
//
// Environment Variable Mappings:
//   - CODECHUNK_API_ENABLE_DEFAULT_MIDDLEWARE → api.enable_default_middleware
//   - CODECHUNK_API_ENABLE_CORS               → api.enable_cors
//   - CODECHUNK_API_ENABLE_SECURITY_HEADERS   → api.enable_security_headers
//   - CODECHUNK_API_ENABLE_LOGGING            → api.enable_logging
//   - CODECHUNK_API_ENABLE_ERROR_HANDLING     → api.enable_error_handling
//
// Usage:
//
//	Call this function after setting up basic Viper configuration (SetEnvPrefix, etc.)
//	but before loading configuration with config.New().
//
// Error Handling:
//
//	Binding failures are logged as warnings but don't stop application startup,
//	allowing the application to continue with default middleware configuration.
func bindMiddlewareEnvVars(v *viper.Viper) {
	middlewareEnvBindings := map[string]string{
		"api.enable_default_middleware": "CODECHUNK_API_ENABLE_DEFAULT_MIDDLEWARE",
		"api.enable_cors":               "CODECHUNK_API_ENABLE_CORS",
		"api.enable_security_headers":   "CODECHUNK_API_ENABLE_SECURITY_HEADERS",
		"api.enable_logging":            "CODECHUNK_API_ENABLE_LOGGING",
		"api.enable_error_handling":     "CODECHUNK_API_ENABLE_ERROR_HANDLING",
	}

	for key, envVar := range middlewareEnvBindings {
		if err := v.BindEnv(key, envVar); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to bind environment variable %s to %s: %v\n", envVar, key, err)
		}
	}
}

func setDefaults(v *viper.Viper) {
	// API defaults
	v.SetDefault("api.port", "8080")
	v.SetDefault("api.host", "0.0.0.0")
	v.SetDefault("api.read_timeout", "10s")
	v.SetDefault("api.write_timeout", "10s")

	// Worker defaults
	v.SetDefault("worker.concurrency", 5)
	v.SetDefault("worker.queue_group", "workers")
	v.SetDefault("worker.job_timeout", "30m")

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.name", "codechunking")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_connections", 25)
	v.SetDefault("database.max_idle_connections", 5)

	// NATS defaults
	v.SetDefault("nats.url", "nats://localhost:4222")
	v.SetDefault("nats.max_reconnects", 5)
	v.SetDefault("nats.reconnect_wait", "2s")

	// Logging defaults
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
}

// GetConfig returns the loaded configuration.
func GetConfig() *config.Config {
	return cmdConfig.cfg
}

// SetTestConfig sets the configuration for testing purposes.
func SetTestConfig(c *config.Config) {
	cmdConfig.cfg = c
}
