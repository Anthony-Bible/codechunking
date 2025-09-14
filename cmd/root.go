package cmd

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/config"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	// Default configuration values.
	defaultWorkerConcurrency      = 5
	defaultDatabasePort           = 5432
	defaultDatabaseMaxConnections = 25
	defaultDatabaseMaxIdleConns   = 5
	defaultNATSMaxReconnects      = 5
)

// Config holds the command configuration.
type Config struct {
	cfgFile string
	cfg     *config.Config
}

var cmdConfig Config //nolint:gochecknoglobals // Standard CLI pattern for Cobra command configuration

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

func init() { //nolint:gochecknoinits // Standard Cobra CLI pattern for root command initialization
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

	// Log config loading details
	var searchPaths []string
	var configFile string

	// Set config file
	if cmdConfig.cfgFile != "" {
		v.SetConfigFile(cmdConfig.cfgFile)
		configFile = cmdConfig.cfgFile
		slogger.InfoNoCtx("Loading specified config file", slogger.Fields{
			"config_file": configFile,
		})
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
		searchPaths = []string{"./configs/config.yaml", "./config.yaml"}
		slogger.InfoNoCtx("Searching for config file in default paths", slogger.Fields{
			"search_paths": searchPaths,
		})
	}

	// Environment variables
	v.SetEnvPrefix("CODECHUNK")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicitly bind environment variables for middleware configuration
	// This ensures environment variables can override config file values for nested boolean pointers
	bindMiddlewareEnvVars(v)

	// Explicitly bind Gemini environment variables
	bindGeminiEnvVars(v)

	// Read configuration
	if err := v.ReadInConfig(); err != nil {
		handleConfigError(err, configFile, searchPaths)
	} else {
		// Successfully loaded config file
		slogger.InfoNoCtx("Successfully loaded config file", slogger.Fields{
			"config_file": v.ConfigFileUsed(),
		})
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

// bindGeminiEnvVars explicitly binds Gemini environment variables to Viper configuration keys.
//
// Problem Solved:
//
//	Ensures that Gemini API key environment variable is properly bound to the config,
//	allowing environment variables to override config file values.
//
// Environment Variable Mappings:
//   - CODECHUNK_GEMINI_API_KEY → gemini.api_key
//
// Usage:
//
//	Call this function after setting up basic Viper configuration (SetEnvPrefix, etc.)
//	but before loading configuration with config.New().
//
// Error Handling:
//
//	Binding failures are logged as warnings but don't stop application startup,
//	allowing the application to continue with config file values.
func bindGeminiEnvVars(v *viper.Viper) {
	geminiEnvBindings := map[string]string{
		"gemini.api_key": "CODECHUNK_GEMINI_API_KEY",
	}

	for key, envVar := range geminiEnvBindings {
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
	v.SetDefault("worker.concurrency", defaultWorkerConcurrency)
	v.SetDefault("worker.queue_group", "workers")
	v.SetDefault("worker.job_timeout", "30m")

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", defaultDatabasePort)
	v.SetDefault("database.name", "codechunking")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_connections", defaultDatabaseMaxConnections)
	v.SetDefault("database.max_idle_connections", defaultDatabaseMaxIdleConns)

	// NATS defaults
	v.SetDefault("nats.url", "nats://localhost:4222")
	v.SetDefault("nats.max_reconnects", defaultNATSMaxReconnects)
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

// handleConfigError handles configuration file loading errors with detailed logging.
func handleConfigError(err error, configFile string, searchPaths []string) {
	var configFileNotFoundError viper.ConfigFileNotFoundError
	if errors.As(err, &configFileNotFoundError) {
		handleConfigNotFound(err, searchPaths)
		return
	}

	slogger.ErrorNoCtx("Error reading config file", slogger.Fields{
		"config_file": configFile,
		"error":       err.Error(),
	})
	fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
}

// handleConfigNotFound handles the specific case when config file is not found.
func handleConfigNotFound(err error, searchPaths []string) {
	if cmdConfig.cfgFile != "" {
		slogger.WarnNoCtx(
			"Specified config file not found, using defaults and environment variables",
			slogger.Fields{
				"specified_file": cmdConfig.cfgFile,
				"error":          err.Error(),
			},
		)
		return
	}

	slogger.WarnNoCtx(
		"Config file not found in search paths, using defaults and environment variables",
		slogger.Fields{
			"searched_paths": searchPaths,
		},
	)
}
