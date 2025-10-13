// Package cmd provides command-line interface functionality for the codechunking application.
/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"codechunking/internal/adapter/inbound/messaging"
	"codechunking/internal/adapter/outbound/embeddings/simple"
	"codechunking/internal/adapter/outbound/gemini"
	"codechunking/internal/adapter/outbound/gitclient"
	"codechunking/internal/adapter/outbound/repository"
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/service"
	"codechunking/internal/application/worker"
	"codechunking/internal/config"
	"codechunking/internal/port/inbound"
	"codechunking/internal/port/outbound"
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	// Import parser packages to register them in the global registry.
	_ "codechunking/internal/adapter/outbound/treesitter/parsers/go"
	_ "codechunking/internal/adapter/outbound/treesitter/parsers/javascript"
	_ "codechunking/internal/adapter/outbound/treesitter/parsers/python"
)

const (
	defaultHost = "localhost"
)

// newWorkerCmd creates and returns the worker command.
func newWorkerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "worker",
		Short: "Start the background worker service",
		Long: `Start the background worker service that processes indexing jobs from NATS JetStream.

The worker service:
- Connects to NATS JetStream to consume indexing jobs
- Processes repositories by cloning, parsing, and generating embeddings
- Runs with configurable concurrency for parallel job processing
- Provides automatic retry logic and error handling

Configuration is loaded from config files and environment variables.`,
		Run: func(_ *cobra.Command, _ []string) {
			runWorkerService()
		},
	}
}

// runWorkerService starts and runs the worker service.
func runWorkerService() {
	cfg := GetConfig()

	slogger.InfoNoCtx("Starting worker service", slogger.Fields{
		"concurrency": cfg.Worker.Concurrency,
		"queue_group": cfg.Worker.QueueGroup,
	})

	dbPool, err := setupDatabaseConnection(cfg)
	if err != nil {
		slogger.ErrorNoCtx("Failed to create database connection pool", slogger.Fields{"error": err.Error()})
		return
	}
	defer dbPool.Close()

	workerService, err := createWorkerService(cfg, dbPool)
	if err != nil {
		slogger.ErrorNoCtx("Failed to create worker service", slogger.Fields{"error": err.Error()})
		return
	}

	if err := startWorkerService(workerService); err != nil {
		slogger.ErrorNoCtx("Failed to start worker service", slogger.Fields{"error": err.Error()})
		return
	}

	waitForShutdownAndStop(workerService)
}

// setupDatabaseConnection initializes the database connection with defaults.
func setupDatabaseConnection(cfg *config.Config) (*pgxpool.Pool, error) {
	dbConfig := repository.DatabaseConfig{
		Host:           cfg.Database.Host,
		Port:           cfg.Database.Port,
		Database:       cfg.Database.Name,
		Username:       cfg.Database.User,
		Password:       cfg.Database.Password,
		Schema:         "codechunking",
		MaxConnections: cfg.Database.MaxConnections,
		SSLMode:        cfg.Database.SSLMode,
	}

	// Set defaults if not configured
	if dbConfig.Host == "" {
		dbConfig.Host = defaultHost
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

// createWorkerService creates and configures the worker service with all dependencies.
func createWorkerService(cfg *config.Config, dbPool *pgxpool.Pool) (inbound.WorkerService, error) {
	// Create repository implementations
	repoRepository := repository.NewPostgreSQLRepositoryRepository(dbPool)
	indexingJobRepository := repository.NewPostgreSQLIndexingJobRepository(dbPool)
	chunkStorageRepository := repository.NewPostgreSQLChunkRepository(dbPool)

	// Create git client implementation
	gitClient := gitclient.NewAuthenticatedGitClient()

	// Create code parser implementation
	codeParser, err := treesitter.NewTreeSitterCodeParser(context.Background())
	if err != nil {
		slogger.ErrorNoCtx("Failed to create TreeSitter code parser", slogger.Fields{"error": err.Error()})
		return nil, err
	}

	// Create job processor
	jobProcessorConfig := worker.JobProcessorConfig{
		WorkspaceDir:      "/tmp/codechunking-workspace",
		MaxConcurrentJobs: cfg.Worker.Concurrency,
		JobTimeout:        cfg.Worker.JobTimeout,
	}

	jobProcessor := worker.NewDefaultJobProcessor(
		jobProcessorConfig,
		indexingJobRepository,
		repoRepository,
		gitClient,
		codeParser,                  // Now using real TreeSitter CodeParser
		createEmbeddingService(cfg), // Using Gemini embedding service
		chunkStorageRepository,      // Repository for storing chunks and embeddings
	)

	// Create consumer
	consumerConfig := messaging.ConsumerConfig{
		Subject:       "indexing.job",
		QueueGroup:    cfg.Worker.QueueGroup,
		DurableName:   "indexing-consumer",
		AckWait:       30 * time.Second,
		MaxDeliver:    3,
		MaxAckPending: 100,
	}

	consumer, err := messaging.NewNATSConsumer(consumerConfig, cfg.NATS, jobProcessor)
	if err != nil {
		return nil, err
	}

	// Create worker service
	workerServiceConfig := service.WorkerServiceConfig{
		Concurrency:         cfg.Worker.Concurrency,
		QueueGroup:          cfg.Worker.QueueGroup,
		JobTimeout:          cfg.Worker.JobTimeout,
		HealthCheckInterval: 30 * time.Second,
		RestartDelay:        5 * time.Second,
		MaxRestartAttempts:  3,
		ShutdownTimeout:     30 * time.Second,
		ConsumerStopTimeout: 5 * time.Second,
	}

	workerService := service.NewDefaultWorkerService(workerServiceConfig, cfg.NATS, jobProcessor)

	// Add consumer to service
	if err := workerService.AddConsumer(consumer); err != nil {
		return nil, err
	}

	return workerService, nil
}

// startWorkerService starts the worker service.
func startWorkerService(workerService inbound.WorkerService) error {
	ctx := context.Background()
	if err := workerService.Start(ctx); err != nil {
		return err
	}

	slogger.InfoNoCtx("Worker service started successfully", nil)
	return nil
}

// waitForShutdownAndStop waits for shutdown signal and stops the service gracefully.
func waitForShutdownAndStop(workerService inbound.WorkerService) {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	sig := <-sigChan
	slogger.InfoNoCtx("Received shutdown signal, initiating graceful shutdown", slogger.Fields{
		"signal": sig.String(),
	})

	// Create context with timeout for graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop worker service gracefully
	if err := workerService.Stop(shutdownCtx); err != nil {
		slogger.ErrorNoCtx("Error during worker service shutdown", slogger.Fields{"error": err.Error()})
	} else {
		slogger.InfoNoCtx("Worker service shutdown completed successfully", nil)
	}
}

// createEmbeddingService creates an embedding service, preferring Gemini but falling back to simple generator.
func createEmbeddingService(cfg *config.Config) outbound.EmbeddingService {
	// Try to create Gemini client first using Viper configuration
	geminiAPIKey := cfg.Gemini.APIKey

	// Fallback to environment variables for backward compatibility
	if geminiAPIKey == "" {
		geminiAPIKey = os.Getenv("GEMINI_API_KEY")
		if geminiAPIKey == "" {
			geminiAPIKey = os.Getenv("GOOGLE_API_KEY")
		}
	}

	if geminiAPIKey != "" {
		config := &gemini.ClientConfig{
			APIKey:     geminiAPIKey,
			Model:      cfg.Gemini.Model,
			TaskType:   "RETRIEVAL_DOCUMENT",
			Timeout:    cfg.Gemini.Timeout,
			Dimensions: 768, // Default dimensions for gemini-embedding-001
		}

		client, err := gemini.NewClient(config)
		if err != nil {
			slogger.ErrorNoCtx("Failed to create Gemini client, falling back to simple generator", slogger.Fields{
				"error": err.Error(),
			})
		} else {
			slogger.InfoNoCtx("Using Gemini embedding service", slogger.Fields{
				"model": config.Model,
			})
			return client
		}
	} else {
		slogger.WarnNoCtx("No Gemini API key found in configuration or environment (CODECHUNK_GEMINI_API_KEY, GEMINI_API_KEY, or GOOGLE_API_KEY), falling back to simple generator", nil)
	}

	// Fall back to simple generator
	slogger.InfoNoCtx("Using simple embedding generator (fallback)", nil)
	return simple.New()
}

func init() { //nolint:gochecknoinits // Standard Cobra CLI pattern for command registration
	rootCmd.AddCommand(newWorkerCmd())

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// workerCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// workerCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
