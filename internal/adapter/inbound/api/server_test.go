package api

import (
	"codechunking/internal/adapter/inbound/api/testutil"
	"codechunking/internal/config"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.Config
		expectedError string
		validateFunc  func(t *testing.T, server *Server)
	}{
		{
			name: "creates_server_with_valid_config",
			config: &config.Config{
				API: config.APIConfig{
					Host:         "localhost",
					Port:         "8080",
					ReadTimeout:  10 * time.Second,
					WriteTimeout: 10 * time.Second,
				},
			},
			validateFunc: func(t *testing.T, server *Server) {
				require.NotNil(t, server)
				assert.Equal(t, "localhost:8080", server.Address())
				assert.Equal(t, 10*time.Second, server.ReadTimeout())
				assert.Equal(t, 10*time.Second, server.WriteTimeout())
			},
		},
		{
			name: "creates_server_with_default_host_when_empty",
			config: &config.Config{
				API: config.APIConfig{
					Host:         "",
					Port:         "8080",
					ReadTimeout:  5 * time.Second,
					WriteTimeout: 5 * time.Second,
				},
			},
			validateFunc: func(t *testing.T, server *Server) {
				require.NotNil(t, server)
				assert.Equal(t, "0.0.0.0:8080", server.Address())
			},
		},
		{
			name: "creates_server_with_custom_timeouts",
			config: &config.Config{
				API: config.APIConfig{
					Host:         "0.0.0.0",
					Port:         "9090",
					ReadTimeout:  30 * time.Second,
					WriteTimeout: 45 * time.Second,
				},
			},
			validateFunc: func(t *testing.T, server *Server) {
				require.NotNil(t, server)
				assert.Equal(t, "0.0.0.0:9090", server.Address())
				assert.Equal(t, 30*time.Second, server.ReadTimeout())
				assert.Equal(t, 45*time.Second, server.WriteTimeout())
			},
		},
		{
			name: "fails_with_invalid_port",
			config: &config.Config{
				API: config.APIConfig{
					Host: "localhost",
					Port: "invalid",
				},
			},
			expectedError: "invalid port",
		},
		{
			name: "fails_with_negative_timeout",
			config: &config.Config{
				API: config.APIConfig{
					Host:         "localhost",
					Port:         "8080",
					ReadTimeout:  -1 * time.Second,
					WriteTimeout: 10 * time.Second,
				},
			},
			expectedError: "invalid timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockHealthService := testutil.NewMockHealthService()
			mockRepositoryService := testutil.NewMockRepositoryService()
			mockErrorHandler := testutil.NewMockErrorHandler()

			// Execute
			server, err := NewServer(tt.config, mockHealthService, mockRepositoryService, mockErrorHandler)

			// Assert
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, server)
			} else {
				require.NoError(t, err)
				require.NotNil(t, server)
				if tt.validateFunc != nil {
					tt.validateFunc(t, server)
				}
			}
		})
	}
}

func TestServer_Start(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.Config
		setupFunc     func(t *testing.T) context.Context
		expectedError string
		validateFunc  func(t *testing.T, server *Server)
	}{
		{
			name: "starts_server_successfully",
			config: &config.Config{
				API: config.APIConfig{
					Host:         "localhost",
					Port:         "0", // Use random available port
					ReadTimeout:  5 * time.Second,
					WriteTimeout: 5 * time.Second,
				},
			},
			setupFunc: func(t *testing.T) context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				t.Cleanup(cancel)
				return ctx
			},
			validateFunc: func(t *testing.T, server *Server) {
				assert.True(t, server.IsRunning())
				assert.NotEmpty(t, server.Address())
			},
		},
		{
			name: "fails_to_start_with_invalid_address",
			config: &config.Config{
				API: config.APIConfig{
					Host:         "256.256.256.256", // Invalid IP
					Port:         "8080",
					ReadTimeout:  5 * time.Second,
					WriteTimeout: 5 * time.Second,
				},
			},
			setupFunc: func(t *testing.T) context.Context {
				return context.Background()
			},
			expectedError: "failed to start server",
		},
		{
			name: "handles_context_cancellation_during_startup",
			config: &config.Config{
				API: config.APIConfig{
					Host:         "localhost",
					Port:         "0",
					ReadTimeout:  5 * time.Second,
					WriteTimeout: 5 * time.Second,
				},
			},
			setupFunc: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				// Cancel immediately to test cancellation handling
				cancel()
				return ctx
			},
			expectedError: "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			ctx := tt.setupFunc(t)
			mockHealthService := testutil.NewMockHealthService()
			mockRepositoryService := testutil.NewMockRepositoryService()
			mockErrorHandler := testutil.NewMockErrorHandler()

			server, err := NewServer(tt.config, mockHealthService, mockRepositoryService, mockErrorHandler)
			require.NoError(t, err)

			// Execute
			err = server.Start(ctx)

			// Assert
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				if tt.validateFunc != nil {
					tt.validateFunc(t, server)
				}

				// Cleanup: stop server if it started successfully
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()
				if err := server.Shutdown(shutdownCtx); err != nil {
					t.Errorf("Failed to shutdown server: %v", err)
				}
			}
		})
	}
}

func TestServer_Shutdown(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(t *testing.T) (*Server, context.Context)
		expectedError string
		validateFunc  func(t *testing.T, server *Server)
	}{
		{
			name: "graceful_shutdown_of_running_server",
			setupFunc: func(t *testing.T) (*Server, context.Context) {
				config := &config.Config{
					API: config.APIConfig{
						Host:         "localhost",
						Port:         "0",
						ReadTimeout:  5 * time.Second,
						WriteTimeout: 5 * time.Second,
					},
				}

				mockHealthService := testutil.NewMockHealthService()
				mockRepositoryService := testutil.NewMockRepositoryService()
				mockErrorHandler := testutil.NewMockErrorHandler()

				server, err := NewServer(config, mockHealthService, mockRepositoryService, mockErrorHandler)
				require.NoError(t, err)

				startCtx, startCancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer startCancel()
				err = server.Start(startCtx)
				require.NoError(t, err)

				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
				t.Cleanup(shutdownCancel)

				return server, shutdownCtx
			},
			validateFunc: func(t *testing.T, server *Server) {
				assert.False(t, server.IsRunning())
			},
		},
		{
			name: "shutdown_with_timeout_context",
			setupFunc: func(t *testing.T) (*Server, context.Context) {
				config := &config.Config{
					API: config.APIConfig{
						Host:         "localhost",
						Port:         "0",
						ReadTimeout:  5 * time.Second,
						WriteTimeout: 5 * time.Second,
					},
				}

				mockHealthService := testutil.NewMockHealthService()
				mockRepositoryService := testutil.NewMockRepositoryService()
				mockErrorHandler := testutil.NewMockErrorHandler()

				server, err := NewServer(config, mockHealthService, mockRepositoryService, mockErrorHandler)
				require.NoError(t, err)

				startCtx, startCancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer startCancel()
				err = server.Start(startCtx)
				require.NoError(t, err)

				// Very short timeout to test timeout handling
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
				t.Cleanup(shutdownCancel)

				return server, shutdownCtx
			},
			expectedError: "context deadline exceeded",
		},
		{
			name: "shutdown_of_non_running_server",
			setupFunc: func(t *testing.T) (*Server, context.Context) {
				config := &config.Config{
					API: config.APIConfig{
						Host:         "localhost",
						Port:         "0",
						ReadTimeout:  5 * time.Second,
						WriteTimeout: 5 * time.Second,
					},
				}

				mockHealthService := testutil.NewMockHealthService()
				mockRepositoryService := testutil.NewMockRepositoryService()
				mockErrorHandler := testutil.NewMockErrorHandler()

				server, err := NewServer(config, mockHealthService, mockRepositoryService, mockErrorHandler)
				require.NoError(t, err)

				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 1*time.Second)
				t.Cleanup(shutdownCancel)

				return server, shutdownCtx
			},
			// Should not error when shutting down a non-running server
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			server, ctx := tt.setupFunc(t)

			// Execute
			err := server.Shutdown(ctx)

			// Assert
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				if tt.validateFunc != nil {
					tt.validateFunc(t, server)
				}
			}
		})
	}
}

func TestServer_AddressAndState(t *testing.T) {
	t.Run("server_address_methods", func(t *testing.T) {
		config := &config.Config{
			API: config.APIConfig{
				Host:         "127.0.0.1",
				Port:         "9999",
				ReadTimeout:  10 * time.Second,
				WriteTimeout: 15 * time.Second,
			},
		}

		mockHealthService := testutil.NewMockHealthService()
		mockRepositoryService := testutil.NewMockRepositoryService()
		mockErrorHandler := testutil.NewMockErrorHandler()

		server, err := NewServer(config, mockHealthService, mockRepositoryService, mockErrorHandler)
		require.NoError(t, err)

		// Test address methods
		assert.Equal(t, "127.0.0.1:9999", server.Address())
		assert.Equal(t, "127.0.0.1", server.Host())
		assert.Equal(t, "9999", server.Port())

		// Test timeout methods
		assert.Equal(t, 10*time.Second, server.ReadTimeout())
		assert.Equal(t, 15*time.Second, server.WriteTimeout())

		// Test state methods
		assert.False(t, server.IsRunning()) // Not started yet
	})
}

func TestServer_HTTPServerConfiguration(t *testing.T) {
	t.Run("http_server_has_correct_configuration", func(t *testing.T) {
		config := &config.Config{
			API: config.APIConfig{
				Host:         "localhost",
				Port:         "8080",
				ReadTimeout:  20 * time.Second,
				WriteTimeout: 25 * time.Second,
			},
		}

		mockHealthService := testutil.NewMockHealthService()
		mockRepositoryService := testutil.NewMockRepositoryService()
		mockErrorHandler := testutil.NewMockErrorHandler()

		server, err := NewServer(config, mockHealthService, mockRepositoryService, mockErrorHandler)
		require.NoError(t, err)

		httpServer := server.HTTPServer()
		require.NotNil(t, httpServer)

		assert.Equal(t, "localhost:8080", httpServer.Addr)
		assert.Equal(t, 20*time.Second, httpServer.ReadTimeout)
		assert.Equal(t, 25*time.Second, httpServer.WriteTimeout)
		assert.NotNil(t, httpServer.Handler) // Should have the mux registered
	})
}

func TestServer_MiddlewareIntegration(t *testing.T) {
	t.Run("server_registers_middleware_chain", func(t *testing.T) {
		config := &config.Config{
			API: config.APIConfig{
				Host:         "localhost",
				Port:         "0",
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
			},
		}

		mockHealthService := testutil.NewMockHealthService()
		mockRepositoryService := testutil.NewMockRepositoryService()
		mockErrorHandler := testutil.NewMockErrorHandler()

		server, err := NewServer(config, mockHealthService, mockRepositoryService, mockErrorHandler)
		require.NoError(t, err)

		// Test that middleware is properly registered
		middlewareCount := server.MiddlewareCount()
		assert.Positive(t, middlewareCount, "Server should have middleware registered")

		// Test specific middleware types are registered
		assert.True(t, server.HasMiddleware("logging"), "Should have logging middleware")
		assert.True(t, server.HasMiddleware("cors"), "Should have CORS middleware")
		assert.True(t, server.HasMiddleware("error_handling"), "Should have error handling middleware")
	})
}

func TestServer_RouteIntegration(t *testing.T) {
	t.Run("server_registers_all_api_routes", func(t *testing.T) {
		config := &config.Config{
			API: config.APIConfig{
				Host:         "localhost",
				Port:         "0",
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
			},
		}

		mockHealthService := testutil.NewMockHealthService()
		mockRepositoryService := testutil.NewMockRepositoryService()
		mockErrorHandler := testutil.NewMockErrorHandler()

		server, err := NewServer(config, mockHealthService, mockRepositoryService, mockErrorHandler)
		require.NoError(t, err)

		// Test that all required routes are registered
		expectedRoutes := []string{
			"GET /health",
			"POST /repositories",
			"GET /repositories",
			"GET /repositories/{id}",
			"DELETE /repositories/{id}",
			"GET /repositories/{id}/jobs",
			"GET /repositories/{id}/jobs/{job_id}",
		}

		for _, route := range expectedRoutes {
			assert.True(t, server.HasRoute(route), "Should have route: %s", route)
		}

		// Verify total route count
		routeCount := server.RouteCount()
		assert.Equal(t, len(expectedRoutes), routeCount, "Should register exact number of expected routes")
	})
}
