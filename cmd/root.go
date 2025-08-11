package cmd

import (
	"fmt"
	"os"
	"strings"

	"codechunking/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	cfg     *config.Config
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./configs/config.yaml)")
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
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
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

	// Read configuration
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
		}
		// Config file not found; use defaults and environment
	}

	// Load configuration
	cfg = config.New(v)
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

// GetConfig returns the loaded configuration
func GetConfig() *config.Config {
	return cfg
}
