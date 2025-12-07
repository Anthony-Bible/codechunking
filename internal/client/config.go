package client

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Default configuration values.
const (
	// DefaultAPIURL is the default API server URL.
	DefaultAPIURL = "http://localhost:8080"

	// DefaultTimeout is the default HTTP client timeout.
	DefaultTimeout = 30 * time.Second

	// EnvAPIURL is the environment variable name for the API URL.
	EnvAPIURL = "CODECHUNK_CLIENT_API_URL"

	// EnvTimeout is the environment variable name for the timeout duration.
	EnvTimeout = "CODECHUNK_CLIENT_TIMEOUT"
)

// Supported URL schemes.
const (
	schemeHTTP  = "http://"
	schemeHTTPS = "https://"
)

// Config holds the client configuration for connecting to the CodeChunking API server.
// It specifies the API endpoint URL and the HTTP client timeout duration.
type Config struct {
	// APIURL is the base URL of the API server (e.g., "http://localhost:8080").
	// Must include the scheme (http:// or https://).
	APIURL string

	// Timeout is the maximum duration for HTTP requests.
	// Must be a positive duration.
	Timeout time.Duration
}

// DefaultConfig returns a Config with sensible default values.
// The default API URL is http://localhost:8080 and the default timeout is 30 seconds.
func DefaultConfig() Config {
	return Config{
		APIURL:  DefaultAPIURL,
		Timeout: DefaultTimeout,
	}
}

// LoadConfig loads configuration from environment variables, falling back to defaults.
//
// Environment variables:
//   - CODECHUNK_CLIENT_API_URL: API server URL (optional, defaults to http://localhost:8080)
//   - CODECHUNK_CLIENT_TIMEOUT: Request timeout as a duration string (optional, defaults to 30s)
//
// The timeout must be a valid duration string (e.g., "30s", "1m", "500ms") and must be positive.
// Returns an error if the timeout environment variable is set but invalid.
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	if apiURL := os.Getenv(EnvAPIURL); apiURL != "" {
		cfg.APIURL = apiURL
	}

	if timeoutStr, ok := os.LookupEnv(EnvTimeout); ok {
		if timeoutStr == "" {
			return nil, fmt.Errorf("environment variable %s is set but empty: timeout cannot be empty", EnvTimeout)
		}

		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout duration in %s: %w", EnvTimeout, err)
		}

		if timeout <= 0 {
			return nil, fmt.Errorf("invalid timeout value in %s: timeout must be positive, got %v", EnvTimeout, timeout)
		}

		cfg.Timeout = timeout
	}

	return &cfg, nil
}

// Validate validates the configuration and returns an error if any field is invalid.
//
// Validation rules:
//   - APIURL must not be empty
//   - APIURL must start with http:// or https://
//   - Timeout must be positive (greater than zero)
func (c Config) Validate() error {
	if c.APIURL == "" {
		return errors.New("invalid configuration: API URL cannot be empty")
	}

	if !strings.HasPrefix(c.APIURL, schemeHTTP) && !strings.HasPrefix(c.APIURL, schemeHTTPS) {
		return fmt.Errorf("invalid configuration: API URL must have http:// or https:// scheme, got %q", c.APIURL)
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("invalid configuration: timeout must be positive, got %v", c.Timeout)
	}

	return nil
}
