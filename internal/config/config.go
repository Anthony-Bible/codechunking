package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds the complete application configuration
type Config struct {
	API      APIConfig      `mapstructure:"api"`
	Worker   WorkerConfig   `mapstructure:"worker"`
	Database DatabaseConfig `mapstructure:"database"`
	NATS     NATSConfig     `mapstructure:"nats"`
	Gemini   GeminiConfig   `mapstructure:"gemini"`
	Log      LogConfig      `mapstructure:"log"`
}

// APIConfig holds API server configuration
type APIConfig struct {
	Host                    string        `mapstructure:"host"`
	Port                    string        `mapstructure:"port"`
	ReadTimeout             time.Duration `mapstructure:"read_timeout"`
	WriteTimeout            time.Duration `mapstructure:"write_timeout"`
	EnableDefaultMiddleware *bool         `mapstructure:"enable_default_middleware"`
	EnableCORS              *bool         `mapstructure:"enable_cors"`
	EnableSecurityHeaders   *bool         `mapstructure:"enable_security_headers"`
	EnableLogging           *bool         `mapstructure:"enable_logging"`
	EnableErrorHandling     *bool         `mapstructure:"enable_error_handling"`
}

// WorkerConfig holds worker configuration
type WorkerConfig struct {
	Concurrency int           `mapstructure:"concurrency"`
	QueueGroup  string        `mapstructure:"queue_group"`
	JobTimeout  time.Duration `mapstructure:"job_timeout"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host               string `mapstructure:"host"`
	Port               int    `mapstructure:"port"`
	User               string `mapstructure:"user"`
	Password           string `mapstructure:"password"`
	Name               string `mapstructure:"name"`
	SSLMode            string `mapstructure:"sslmode"`
	MaxConnections     int    `mapstructure:"max_connections"`
	MaxIdleConnections int    `mapstructure:"max_idle_connections"`
}

// DSN returns the database connection string
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode)
}

// NATSConfig holds NATS configuration
type NATSConfig struct {
	URL           string        `mapstructure:"url"`
	MaxReconnects int           `mapstructure:"max_reconnects"`
	ReconnectWait time.Duration `mapstructure:"reconnect_wait"`
}

// GeminiConfig holds Gemini API configuration
type GeminiConfig struct {
	APIKey     string        `mapstructure:"api_key"`
	Model      string        `mapstructure:"model"`
	MaxRetries int           `mapstructure:"max_retries"`
	Timeout    time.Duration `mapstructure:"timeout"`
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// New creates a new Config instance from Viper
func New(v *viper.Viper) *Config {
	var config Config

	// Unmarshal configuration
	if err := v.Unmarshal(&config); err != nil {
		panic(fmt.Errorf("unable to decode config: %w", err))
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		panic(fmt.Errorf("invalid configuration: %w", err))
	}

	return &config
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Required fields validation
	if c.Database.User == "" {
		return fmt.Errorf("database.user is required")
	}

	if c.Database.Name == "" {
		return fmt.Errorf("database.name is required")
	}

	// Validate Gemini config when in production
	if c.Log.Level == "error" || c.Log.Level == "fatal" {
		if c.Gemini.APIKey == "" {
			return fmt.Errorf("gemini.api_key is required in production")
		}
	}

	// Validate numeric ranges
	if c.Worker.Concurrency < 1 {
		return fmt.Errorf("worker.concurrency must be at least 1")
	}

	if c.Database.Port < 1 || c.Database.Port > 65535 {
		return fmt.Errorf("database.port must be between 1 and 65535")
	}

	return nil
}
