/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codechunking/internal/adapter/inbound/api"
	"codechunking/internal/adapter/inbound/service"
	"codechunking/internal/adapter/outbound/mock"
	"codechunking/internal/adapter/outbound/repository"
	"codechunking/internal/application/registry"
	"codechunking/internal/config"
	"codechunking/internal/port/inbound"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ServiceFactory creates and manages service instances
type ServiceFactory struct {
	config *config.Config
}

// NewServiceFactory creates a new ServiceFactory
func NewServiceFactory(cfg *config.Config) *ServiceFactory {
	return &ServiceFactory{
		config: cfg,
	}
}

// CreateHealthService creates a health service instance
func (sf *ServiceFactory) CreateHealthService() inbound.HealthService {
	// Create database connection
	pool, err := sf.createDatabasePool()
	if err != nil {
		log.Printf("Failed to create database connection, using mock health service: %v", err)
		// Return a simple mock that always reports healthy
		return service.NewHealthServiceAdapter(nil, nil, nil, "1.0.0")
	}

	// Create repositories
	repositoryRepo := repository.NewPostgreSQLRepositoryRepository(pool)
	indexingJobRepo := repository.NewPostgreSQLIndexingJobRepository(pool)
	messagePublisher := mock.NewMockMessagePublisher()

	return service.NewHealthServiceAdapter(repositoryRepo, indexingJobRepo, messagePublisher, "1.0.0")
}

// CreateRepositoryService creates a repository service instance
func (sf *ServiceFactory) CreateRepositoryService() inbound.RepositoryService {
	// Create database connection
	pool, err := sf.createDatabasePool()
	if err != nil {
		log.Fatalf("Failed to create database connection: %v", err)
	}

	// Create repositories
	repositoryRepo := repository.NewPostgreSQLRepositoryRepository(pool)
	indexingJobRepo := repository.NewPostgreSQLIndexingJobRepository(pool)
	messagePublisher := mock.NewMockMessagePublisher()

	// Create service registry
	serviceRegistry := registry.NewServiceRegistry(repositoryRepo, indexingJobRepo, messagePublisher)

	// Create and return the adapter
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

// CreateServer creates a fully configured server instance
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

	// TODO: Add conditional middleware based on config
	// if sf.config.API.EnableCORS {
	//     serverBuilder = serverBuilder.WithMiddleware(api.NewCORSMiddleware())
	// }

	return serverBuilder.Build()
}

// shouldEnableDefaultMiddleware determines if default middleware should be enabled
func (sf *ServiceFactory) shouldEnableDefaultMiddleware() bool {
	// TODO: Make this configurable
	// For now, always enable default middleware
	return true
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
