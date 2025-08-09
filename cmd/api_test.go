package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"codechunking/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPICommand_Integration(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(t *testing.T) *config.Config
		validateFunc func(t *testing.T, baseURL string)
		expectError  bool
	}{
		{
			name: "starts_api_server_successfully_with_default_config",
			setupFunc: func(t *testing.T) *config.Config {
				return &config.Config{
					API: config.APIConfig{
						Host:         "localhost",
						Port:         "0", // Use random available port
						ReadTimeout:  5 * time.Second,
						WriteTimeout: 5 * time.Second,
					},
					Database: config.DatabaseConfig{
						Host:     "localhost",
						Port:     5432,
						User:     "test",
						Password: "test",
						Name:     "test_db",
						SSLMode:  "disable",
					},
					NATS: config.NATSConfig{
						URL: "nats://localhost:4222",
					},
					Gemini: config.GeminiConfig{
						APIKey: "test-key",
						Model:  "text-embedding-004",
					},
				}
			},
			validateFunc: func(t *testing.T, baseURL string) {
				// Test health endpoint
				resp, err := http.Get(baseURL + "/health")
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

				// Verify CORS headers are set
				assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))

				// Verify response body structure
				var healthResponse map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&healthResponse)
				require.NoError(t, err)

				assert.Contains(t, healthResponse, "status")
				assert.Contains(t, healthResponse, "version")
				assert.Contains(t, healthResponse, "timestamp")
			},
		},
		{
			name: "api_server_handles_all_routes_correctly",
			setupFunc: func(t *testing.T) *config.Config {
				return &config.Config{
					API: config.APIConfig{
						Host:         "127.0.0.1",
						Port:         "0",
						ReadTimeout:  10 * time.Second,
						WriteTimeout: 10 * time.Second,
					},
					Database: config.DatabaseConfig{
						Host:     "localhost",
						Port:     5432,
						User:     "test",
						Password: "test",
						Name:     "test_db",
						SSLMode:  "disable",
					},
					NATS: config.NATSConfig{
						URL: "nats://localhost:4222",
					},
					Gemini: config.GeminiConfig{
						APIKey: "test-key",
						Model:  "text-embedding-004",
					},
				}
			},
			validateFunc: func(t *testing.T, baseURL string) {
				// Test all API routes
				routes := []struct {
					method         string
					path           string
					body           string
					expectedStatus int
				}{
					{http.MethodGet, "/health", "", http.StatusOK},
					{http.MethodGet, "/repositories", "", http.StatusOK},
					{http.MethodPost, "/repositories", `{"url": "https://github.com/test/repo"}`, http.StatusAccepted},
					{http.MethodOptions, "/repositories", "", http.StatusNoContent}, // CORS preflight
				}

				for _, route := range routes {
					var resp *http.Response
					var err error

					if route.body != "" {
						resp, err = http.Post(baseURL+route.path, "application/json", strings.NewReader(route.body))
					} else if route.method == http.MethodOptions {
						req, _ := http.NewRequest(http.MethodOptions, baseURL+route.path, nil)
						req.Header.Set("Origin", "https://example.com")
						req.Header.Set("Access-Control-Request-Method", "POST")
						resp, err = http.DefaultClient.Do(req)
					} else {
						resp, err = http.Get(baseURL + route.path)
					}

					require.NoError(t, err, "Route %s %s should not error", route.method, route.path)
					defer resp.Body.Close()

					assert.Equal(t, route.expectedStatus, resp.StatusCode,
						"Route %s %s should return %d", route.method, route.path, route.expectedStatus)

					// All responses should have CORS headers
					assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
				}
			},
		},
		{
			name: "api_server_respects_custom_configuration",
			setupFunc: func(t *testing.T) *config.Config {
				return &config.Config{
					API: config.APIConfig{
						Host:         "localhost",
						Port:         "0",
						ReadTimeout:  30 * time.Second,
						WriteTimeout: 45 * time.Second,
					},
					Database: config.DatabaseConfig{
						Host:     "localhost",
						Port:     5432,
						User:     "custom_user",
						Password: "custom_pass",
						Name:     "custom_db",
						SSLMode:  "require",
					},
				}
			},
			validateFunc: func(t *testing.T, baseURL string) {
				// Create a request that would exceed short timeouts but should work with longer ones
				client := &http.Client{
					Timeout: 35 * time.Second,
				}

				resp, err := client.Get(baseURL + "/health")
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusOK, resp.StatusCode)
			},
		},
		{
			name: "api_server_fails_gracefully_with_invalid_config",
			setupFunc: func(t *testing.T) *config.Config {
				return &config.Config{
					API: config.APIConfig{
						Host:         "256.256.256.256", // Invalid IP
						Port:         "8080",
						ReadTimeout:  5 * time.Second,
						WriteTimeout: 5 * time.Second,
					},
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cfg := tt.setupFunc(t)

			// Create a context with timeout for the test
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Start server
			server, baseURL, err := startTestAPIServer(ctx, cfg)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err, "Server should start successfully")
			require.NotNil(t, server, "Server should not be nil")
			require.NotEmpty(t, baseURL, "Base URL should not be empty")

			// Ensure server is stopped after test
			defer func() {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer shutdownCancel()
				server.Shutdown(shutdownCtx)
			}()

			// Wait a moment for server to be fully ready
			time.Sleep(100 * time.Millisecond)

			// Run validation
			if tt.validateFunc != nil {
				tt.validateFunc(t, baseURL)
			}
		})
	}
}

func TestAPICommand_GracefulShutdown(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(t *testing.T, baseURL string)
		shutdownFunc func(t *testing.T, server APIServer, baseURL string)
		validateFunc func(t *testing.T)
	}{
		{
			name: "server_shuts_down_gracefully_without_active_requests",
			setupFunc: func(t *testing.T, baseURL string) {
				// Make a quick request to ensure server is working
				resp, err := http.Get(baseURL + "/health")
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			},
			shutdownFunc: func(t *testing.T, server APIServer, baseURL string) {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				err := server.Shutdown(shutdownCtx)
				assert.NoError(t, err, "Server should shut down gracefully")
			},
		},
		{
			name: "server_waits_for_active_requests_during_shutdown",
			setupFunc: func(t *testing.T, baseURL string) {
				// Start a long-running request in background
				go func() {
					resp, err := http.Get(baseURL + "/health?slow=true")
					if err == nil {
						resp.Body.Close()
					}
				}()
				time.Sleep(100 * time.Millisecond) // Let request start
			},
			shutdownFunc: func(t *testing.T, server APIServer, baseURL string) {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				start := time.Now()
				err := server.Shutdown(shutdownCtx)
				duration := time.Since(start)

				assert.NoError(t, err, "Server should shut down gracefully")
				// Should take some time to wait for active requests
				assert.Greater(t, duration, 50*time.Millisecond)
			},
		},
		{
			name: "server_shutdown_respects_context_timeout",
			setupFunc: func(t *testing.T, baseURL string) {
				// No special setup needed
			},
			shutdownFunc: func(t *testing.T, server APIServer, baseURL string) {
				// Very short timeout
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
				defer cancel()

				err := server.Shutdown(shutdownCtx)
				// Should either succeed quickly or timeout
				if err != nil {
					assert.Contains(t, err.Error(), "deadline exceeded")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup server
			cfg := &config.Config{
				API: config.APIConfig{
					Host:         "localhost",
					Port:         "0",
					ReadTimeout:  5 * time.Second,
					WriteTimeout: 5 * time.Second,
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			server, baseURL, err := startTestAPIServer(ctx, cfg)
			require.NoError(t, err)
			require.NotNil(t, server)

			// Wait for server to be ready
			time.Sleep(100 * time.Millisecond)

			// Run setup
			if tt.setupFunc != nil {
				tt.setupFunc(t, baseURL)
			}

			// Execute shutdown test
			if tt.shutdownFunc != nil {
				tt.shutdownFunc(t, server, baseURL)
			}

			// Run validation
			if tt.validateFunc != nil {
				tt.validateFunc(t)
			}
		})
	}
}

func TestAPICommand_ConfigurationIntegration(t *testing.T) {
	t.Run("loads_configuration_from_environment", func(t *testing.T) {
		// Set environment variables
		os.Setenv("CODECHUNK_API_HOST", "127.0.0.1")
		os.Setenv("CODECHUNK_API_PORT", "0")
		os.Setenv("CODECHUNK_API_READ_TIMEOUT", "15s")
		defer func() {
			os.Unsetenv("CODECHUNK_API_HOST")
			os.Unsetenv("CODECHUNK_API_PORT")
			os.Unsetenv("CODECHUNK_API_READ_TIMEOUT")
		}()

		// Create server using configuration loading
		cfg, err := LoadAPIConfiguration()
		require.NoError(t, err)

		// Validate configuration was loaded from environment
		assert.Equal(t, "127.0.0.1", cfg.API.Host)
		assert.Equal(t, "0", cfg.API.Port)
		assert.Equal(t, 15*time.Second, cfg.API.ReadTimeout)

		// Test server starts with loaded config
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		server, baseURL, err := startTestAPIServer(ctx, cfg)
		require.NoError(t, err)
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer shutdownCancel()
			server.Shutdown(shutdownCtx)
		}()

		// Verify server is working
		resp, err := http.Get(baseURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("loads_configuration_from_config_file", func(t *testing.T) {
		// Create temporary config file
		configContent := `
api:
  host: "localhost"
  port: 0
  read_timeout: 20s
  write_timeout: 25s

database:
  host: "db.test.local"
  port: 5433
  user: "testuser"
  name: "testdb"
  sslmode: "require"
`

		tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.WriteString(configContent)
		require.NoError(t, err)
		tmpFile.Close()

		// Load configuration from file
		cfg, err := LoadAPIConfigurationFromFile(tmpFile.Name())
		require.NoError(t, err)

		// Validate configuration was loaded from file
		assert.Equal(t, "localhost", cfg.API.Host)
		assert.Equal(t, "0", cfg.API.Port)
		assert.Equal(t, 20*time.Second, cfg.API.ReadTimeout)
		assert.Equal(t, 25*time.Second, cfg.API.WriteTimeout)
		assert.Equal(t, "db.test.local", cfg.Database.Host)
		assert.Equal(t, 5433, cfg.Database.Port)

		// Test server starts with file config
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		server, baseURL, err := startTestAPIServer(ctx, cfg)
		require.NoError(t, err)
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer shutdownCancel()
			server.Shutdown(shutdownCtx)
		}()

		// Verify server is working
		resp, err := http.Get(baseURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestAPICommand_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.Config
		expectedError string
	}{
		{
			name: "fails_with_port_already_in_use",
			config: &config.Config{
				API: config.APIConfig{
					Host: "localhost",
					Port: "8080", // Assume this port might be in use
				},
			},
			expectedError: "address already in use",
		},
		{
			name: "fails_with_invalid_host",
			config: &config.Config{
				API: config.APIConfig{
					Host: "999.999.999.999",
					Port: "8080",
				},
			},
			expectedError: "invalid address",
		},
		{
			name: "fails_with_missing_required_config",
			config: &config.Config{
				API: config.APIConfig{
					// Missing required fields
				},
			},
			expectedError: "invalid configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			server, baseURL, err := startTestAPIServer(ctx, tt.config)

			// Should fail to start
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
			assert.Nil(t, server)
			assert.Empty(t, baseURL)
		})
	}
}

// Test helper functions that will need to be implemented

// startTestAPIServer starts an API server for testing and returns the server instance and base URL
func startTestAPIServer(ctx context.Context, cfg *config.Config) (APIServer, string, error) {
	// This function should:
	// 1. Create the API server with the given config
	// 2. Start the server in the background
	// 3. Return the server instance and its base URL
	// 4. Handle port 0 (random port) cases properly

	panic("startTestAPIServer not implemented - this is expected to fail in RED phase")
}

// APIServer interface represents the running API server for testing
type APIServer interface {
	Shutdown(ctx context.Context) error
	Address() string
	IsRunning() bool
}

// LoadAPIConfiguration loads configuration for API server from environment/files
func LoadAPIConfiguration() (*config.Config, error) {
	panic("LoadAPIConfiguration not implemented - this is expected to fail in RED phase")
}

// LoadAPIConfigurationFromFile loads configuration from a specific file
func LoadAPIConfigurationFromFile(filename string) (*config.Config, error) {
	panic("LoadAPIConfigurationFromFile not implemented - this is expected to fail in RED phase")
}

// Additional integration tests for middleware interaction

func TestAPICommand_MiddlewareIntegration(t *testing.T) {
	t.Run("request_logging_works_end_to_end", func(t *testing.T) {
		cfg := &config.Config{
			API: config.APIConfig{
				Host:         "localhost",
				Port:         "0",
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		server, baseURL, err := startTestAPIServer(ctx, cfg)
		require.NoError(t, err)
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer shutdownCancel()
			server.Shutdown(shutdownCtx)
		}()

		// Make request and check that it was logged
		resp, err := http.Get(baseURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		// In a real implementation, we would check log output
		// For now, just verify the request succeeded
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("cors_headers_present_on_all_responses", func(t *testing.T) {
		cfg := &config.Config{
			API: config.APIConfig{
				Host:         "localhost",
				Port:         "0",
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		server, baseURL, err := startTestAPIServer(ctx, cfg)
		require.NoError(t, err)
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer shutdownCancel()
			server.Shutdown(shutdownCtx)
		}()

		// Test various endpoints for CORS headers
		endpoints := []string{"/health", "/repositories"}

		for _, endpoint := range endpoints {
			resp, err := http.Get(baseURL + endpoint)
			require.NoError(t, err, "Endpoint %s should respond", endpoint)
			defer resp.Body.Close()

			// Should have CORS headers
			assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"),
				"Endpoint %s should have CORS headers", endpoint)
		}
	})

	t.Run("error_handling_middleware_catches_panics", func(t *testing.T) {
		// This test would verify that if a handler panics,
		// the error handling middleware catches it and returns a proper 500 response

		// For now, this is a placeholder that will fail in RED phase
		t.Skip("Error handling middleware integration test - will be implemented in GREEN phase")
	})
}
