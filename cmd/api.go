/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"codechunking/internal/adapter/inbound/api"
	"codechunking/internal/adapter/inbound/service"
	"codechunking/internal/adapter/outbound/mock"
	"codechunking/internal/adapter/outbound/repository"
	"codechunking/internal/application/registry"
	"codechunking/internal/config"
	"codechunking/internal/port/inbound"
	"codechunking/internal/port/outbound"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	// serviceVersion defines the version string used for all services created by this factory
	serviceVersion = "1.0.0"
)

// ServiceFactory creates and manages service instances with memoized database connection pooling.
// Thread-safe design using double-checked locking pattern for optimal performance.
type ServiceFactory struct {
	config *config.Config

	// Database connection pool memoization fields
	// These fields implement the double-checked locking pattern for thread-safe, efficient pool creation
	pool      *pgxpool.Pool // Memoized pool instance (nil until first successful creation)
	poolError error         // Reserved for potential future error memoization (currently unused)
	poolMutex sync.RWMutex  // Protects pool state - RWMutex optimizes for read-heavy access patterns

	// Testing instrumentation fields
	// These fields provide visibility into pool creation behavior for test verification
	creationCount    int           // Count of successful pool creations (should be 1 for proper memoization)
	creationAttempts int           // Total creation attempts including failures (for retry behavior testing)
	creationDelay    time.Duration // Artificial delay for concurrent testing scenarios (default 0 for production)
}

// NewServiceFactory creates a new ServiceFactory
func NewServiceFactory(cfg *config.Config) *ServiceFactory {
	return &ServiceFactory{
		config: cfg,
	}
}

// GetDatabasePool returns a memoized database connection pool.
// Uses double-checked locking pattern with RWMutex to ensure the pool is created only once,
// with optimal performance for concurrent read access and thread-safe write access.
// Failed pool creation attempts are not memoized, allowing retries with updated configuration.
func (sf *ServiceFactory) GetDatabasePool() (*pgxpool.Pool, error) {
	// First check: Fast path with read lock for existing successful pool
	sf.poolMutex.RLock()
	if sf.pool != nil {
		// Pool exists, return it while holding read lock
		pool := sf.pool
		sf.poolMutex.RUnlock()
		return pool, nil
	}
	// Pool doesn't exist, release read lock to acquire write lock
	sf.poolMutex.RUnlock()

	// Second check: Acquire write lock for pool creation
	sf.poolMutex.Lock()
	defer sf.poolMutex.Unlock()

	// Double-check pattern: Another thread might have created pool between lock release/acquire
	if sf.pool != nil {
		return sf.pool, nil
	}

	// Increment creation attempts counter (testing instrumentation)
	sf.creationAttempts++

	// Add artificial delay for testing if configured (testing instrumentation)
	if sf.creationDelay > 0 {
		time.Sleep(sf.creationDelay)
	}

	// Attempt pool creation
	pool, err := sf.createDatabasePool()
	if err != nil {
		// Don't memoize failures - allows retry on next call with potentially updated config
		return nil, err
	}

	// Store successful pool creation and update counters
	sf.pool = pool
	sf.poolError = nil
	sf.creationCount++ // Testing instrumentation

	return sf.pool, nil
}

// GetPoolCreationCount returns the number of successful pool creations.
// Used for testing to verify that memoization is working correctly (should be 1 after first success).
func (sf *ServiceFactory) GetPoolCreationCount() int {
	sf.poolMutex.RLock()
	defer sf.poolMutex.RUnlock()
	return sf.creationCount
}

// GetPoolCreationAttempts returns the total number of pool creation attempts (including failures).
// Used for testing retry behavior and concurrent access patterns.
func (sf *ServiceFactory) GetPoolCreationAttempts() int {
	sf.poolMutex.RLock()
	defer sf.poolMutex.RUnlock()
	return sf.creationAttempts
}

// UpdateDatabaseConfig updates the database configuration and resets pool state for retry testing.
// This method is primarily used for testing scenarios where configuration changes need to be tested.
// It safely closes any existing pool and resets the memoization state to allow creation with new config.
func (sf *ServiceFactory) UpdateDatabaseConfig(newConfig *config.DatabaseConfig) {
	sf.poolMutex.Lock()
	defer sf.poolMutex.Unlock()

	// Update the config
	sf.config.Database = *newConfig

	// Reset pool state to allow retry with new config
	if sf.pool != nil {
		sf.pool.Close()
	}
	sf.pool = nil
	sf.poolError = nil
}

// Close properly closes the memoized pool and cleans up resources.
// Thread-safe and idempotent - safe to call multiple times.
// Should be called when shutting down the ServiceFactory to prevent connection leaks.
func (sf *ServiceFactory) Close() error {
	sf.poolMutex.Lock()
	defer sf.poolMutex.Unlock()

	if sf.pool != nil {
		sf.pool.Close()
		sf.pool = nil
	}

	return nil
}

// buildDependencies creates and returns the core dependencies needed by services.
//
// This method centralizes the creation of common dependencies to eliminate code duplication
// between CreateHealthService and CreateRepositoryService. It handles the following:
//
// Returns:
//   - RepositoryRepository: PostgreSQL-backed repository for repository entities (nil on DB error)
//   - IndexingJobRepository: PostgreSQL-backed repository for indexing jobs (nil on DB error)
//   - MessagePublisher: Mock message publisher for async job publishing (never nil)
//   - error: Database connection error, if any
//
// Error Handling Strategy:
//   - If database connection fails, repositories are returned as nil but MessagePublisher is always provided
//   - This allows callers to implement different strategies: graceful degradation (health service)
//     or fail-fast (repository service)
//   - The MessagePublisher uses a mock implementation since it doesn't depend on database connectivity
func (sf *ServiceFactory) buildDependencies() (outbound.RepositoryRepository, outbound.IndexingJobRepository, outbound.MessagePublisher, error) {
	// Always create message publisher first - it's independent of database connectivity
	// and needed by both health and repository services for async job publishing
	messagePublisher := mock.NewMockMessagePublisher()

	// Use memoized database pool instead of creating new one
	dbPool, err := sf.GetDatabasePool()
	if err != nil {
		// Return nil repositories but preserve messagePublisher - allows callers to decide
		// whether to fail fast (repository service) or degrade gracefully (health service)
		return nil, nil, messagePublisher, err
	}

	// Create PostgreSQL-backed repositories using the established connection pool
	// Both repositories share the same pool for connection efficiency
	repositoryRepo := repository.NewPostgreSQLRepositoryRepository(dbPool)
	indexingJobRepo := repository.NewPostgreSQLIndexingJobRepository(dbPool)

	return repositoryRepo, indexingJobRepo, messagePublisher, nil
}

// CreateHealthService creates a health service instance with graceful error handling.
//
// This method uses buildDependencies to create the required database repositories and message publisher.
// If database connection fails, it logs the error and continues by creating a health service with
// nil repositories, allowing the application to start and report degraded health status rather than failing completely.
//
// Returns:
//   - A HealthService that can operate with or without database connectivity
//   - Never returns nil - always provides a functional health service
func (sf *ServiceFactory) CreateHealthService() inbound.HealthService {
	repositoryRepo, indexingJobRepo, messagePublisher, err := sf.buildDependencies()
	if err != nil {
		log.Printf("Failed to create database connection, using mock health service: %v", err)
		// Graceful degradation: create health service with nil repositories
		// This allows the application to start and report degraded health status
		return service.NewHealthServiceAdapter(nil, nil, messagePublisher, serviceVersion)
	}

	// Create fully functional health service with database connectivity
	return service.NewHealthServiceAdapter(repositoryRepo, indexingJobRepo, messagePublisher, serviceVersion)
}

// CreateRepositoryService creates a repository service instance with fail-fast error handling.
//
// This method uses buildDependencies to create the required database repositories and message publisher.
// Unlike CreateHealthService, this method uses a fail-fast approach - if database connection fails,
// it calls log.Fatalf to terminate the application since the repository service cannot function without
// database connectivity.
//
// Returns:
//   - A fully functional RepositoryService with database connectivity
//   - Never returns on database errors - calls log.Fatalf instead
func (sf *ServiceFactory) CreateRepositoryService() inbound.RepositoryService {
	repositoryRepo, indexingJobRepo, messagePublisher, err := sf.buildDependencies()
	if err != nil {
		// Fail-fast approach: repository service cannot function without database
		log.Fatalf("Failed to create database connection: %v", err)
	}

	// Create service registry with all required dependencies
	// All dependencies are guaranteed to be non-nil at this point
	serviceRegistry := registry.NewServiceRegistry(repositoryRepo, indexingJobRepo, messagePublisher)

	// Create and return the repository service adapter
	return service.NewRepositoryServiceAdapter(serviceRegistry)
}

// createDatabasePool creates a database connection pool
func (sf *ServiceFactory) createDatabasePool() (*pgxpool.Pool, error) {
	dbConfig := repository.DatabaseConfig{
		Host:           sf.config.Database.Host,
		Port:           sf.config.Database.Port,
		Database:       sf.config.Database.Name,
		Username:       sf.config.Database.User,
		Password:       sf.config.Database.Password,
		Schema:         "codechunking", // Default schema name
		MaxConnections: sf.config.Database.MaxConnections,
		SSLMode:        sf.config.Database.SSLMode,
	}

	// Set defaults if not configured
	if dbConfig.Host == "" {
		dbConfig.Host = "localhost"
	}
	if dbConfig.Port == 0 {
		dbConfig.Port = 5432
	}
	if dbConfig.MaxConnections == 0 {
		dbConfig.MaxConnections = 10
	}
	if dbConfig.SSLMode == "" {
		dbConfig.SSLMode = "disable"
	}

	return repository.NewDatabaseConnection(dbConfig)
}

// CreateErrorHandler creates an error handler instance
func (sf *ServiceFactory) CreateErrorHandler() api.ErrorHandler {
	return api.NewDefaultErrorHandler()
}

// CreateServer creates a fully configured server instance with middleware configured
// based on the application configuration and sensible defaults.
//
// Server Creation Process:
//  1. Creates core services (health, repository, error handler)
//  2. Configures server builder with services
//  3. Applies middleware based on configuration settings
//  4. Builds and returns the configured server instance
//
// Middleware Configuration:
//   - Default middleware bundle: Enabled by configuration or default (true)
//   - Individual middleware: Can be selectively enabled/disabled via config
//   - Environment variables: Can override config file settings
//
// Returns:
//   - Configured API server instance ready to start
//   - Error if server creation fails
func (sf *ServiceFactory) CreateServer() (*api.Server, error) {
	// Create all services
	healthService := sf.CreateHealthService()
	repositoryService := sf.CreateRepositoryService()
	errorHandler := sf.CreateErrorHandler()

	// Use the new server builder for more flexible configuration
	serverBuilder := api.NewServerBuilder(sf.config).
		WithHealthService(healthService).
		WithRepositoryService(repositoryService).
		WithErrorHandler(errorHandler)

	// Add middleware based on environment/config
	if sf.shouldEnableDefaultMiddleware() {
		serverBuilder = serverBuilder.WithDefaultMiddleware()
	}

	// TODO: Add conditional individual middleware based on config
	// This will be implemented as the server builder API evolves
	// if sf.shouldEnableCORSMiddleware() {
	//     serverBuilder = serverBuilder.WithMiddleware(api.NewCORSMiddleware())
	// }

	return serverBuilder.Build()
}

// MiddlewareDefaults defines the default enable/disable state for each middleware type.
// This centralized configuration makes it easy to adjust default middleware behavior
// across the entire application.
type MiddlewareDefaults struct {
	DefaultMiddleware bool // Whether to enable the default middleware bundle
	CORS              bool // CORS headers middleware
	SecurityHeaders   bool // Security headers middleware
	Logging           bool // Request/response logging middleware
	ErrorHandling     bool // Error handling middleware
}

// middlewareDefaults contains the standard middleware defaults for the application.
// All middleware is enabled by default to provide a secure and observable system out-of-the-box.
var middlewareDefaults = MiddlewareDefaults{
	DefaultMiddleware: true, // Enable default middleware bundle
	CORS:              true, // Enable CORS for web compatibility
	SecurityHeaders:   true, // Enable security headers for protection
	Logging:           true, // Enable logging for observability
	ErrorHandling:     true, // Enable error handling for reliability
}

// shouldEnableMiddleware is a helper function that reduces code duplication across middleware toggle methods.
// It takes a pointer to a boolean configuration field and returns the configured value or a default.
//
// Parameters:
//   - configValue: Pointer to boolean configuration field (nil means not configured)
//   - defaultValue: Default value to use when configuration is not set
//
// Returns:
//   - The configured boolean value, or defaultValue if configValue is nil
func (sf *ServiceFactory) shouldEnableMiddleware(configValue *bool, defaultValue bool) bool {
	if configValue != nil {
		return *configValue
	}
	return defaultValue
}

// shouldEnableDefaultMiddleware determines if default middleware should be enabled
func (sf *ServiceFactory) shouldEnableDefaultMiddleware() bool {
	return sf.shouldEnableMiddleware(sf.config.API.EnableDefaultMiddleware, middlewareDefaults.DefaultMiddleware)
}

// shouldEnableCORSMiddleware determines if CORS middleware should be enabled
func (sf *ServiceFactory) shouldEnableCORSMiddleware() bool {
	return sf.shouldEnableMiddleware(sf.config.API.EnableCORS, middlewareDefaults.CORS)
}

// shouldEnableSecurityMiddleware determines if security headers middleware should be enabled
func (sf *ServiceFactory) shouldEnableSecurityMiddleware() bool {
	return sf.shouldEnableMiddleware(sf.config.API.EnableSecurityHeaders, middlewareDefaults.SecurityHeaders)
}

// shouldEnableLoggingMiddleware determines if logging middleware should be enabled
func (sf *ServiceFactory) shouldEnableLoggingMiddleware() bool {
	return sf.shouldEnableMiddleware(sf.config.API.EnableLogging, middlewareDefaults.Logging)
}

// shouldEnableErrorHandlingMiddleware determines if error handling middleware should be enabled
func (sf *ServiceFactory) shouldEnableErrorHandlingMiddleware() bool {
	return sf.shouldEnableMiddleware(sf.config.API.EnableErrorHandling, middlewareDefaults.ErrorHandling)
}

// apiCmd represents the api command
var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Start the API server",
	Long: `Start the HTTP API server that provides REST endpoints for 
repository management and code chunking operations.

The server provides endpoints for:
- Health checks
- Repository CRUD operations  
- Indexing job management

Configuration is loaded from config files and environment variables.`,
	Run: runAPIServer,
}

func runAPIServer(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg := config.New(viper.GetViper())

	// Create service factory
	serviceFactory := NewServiceFactory(cfg)

	// Create server using the factory
	server, err := serviceFactory.CreateServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start server with timeout
	startCtx, startCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer startCancel()

	if err := server.Start(startCtx); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Printf("API server started successfully on %s", server.Address())
	log.Printf("Server configuration: host=%s port=%s", server.Host(), server.Port())
	log.Printf("Middleware enabled: %d middleware components", server.MiddlewareCount())

	// Create a graceful shutdown handler
	gracefulShutdown(server)
}

// gracefulShutdown handles graceful server shutdown with proper signal handling
func gracefulShutdown(server *api.Server) {
	// Create a channel to receive OS signals
	sigChan := make(chan os.Signal, 1)

	// Register the channel to receive specific signals
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for a signal
	sig := <-sigChan
	log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)

	// Create a context with timeout for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
		os.Exit(1)
	}

	log.Println("API server shut down gracefully")
}

func init() {
	rootCmd.AddCommand(apiCmd)
}
