// Package cmd provides command-line interface functionality for the codechunking application.
/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"codechunking/internal/adapter/inbound/messaging"
	"codechunking/internal/adapter/outbound/gemini"
	"codechunking/internal/adapter/outbound/gitclient"
	"codechunking/internal/adapter/outbound/queue"
	"codechunking/internal/adapter/outbound/repository"
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/service"
	"codechunking/internal/application/worker"
	"codechunking/internal/config"
	"codechunking/internal/port/inbound"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
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

	workerService, batchProgressRepo, _, batchEmbeddingService, chunkRepo, err := createWorkerService(
		cfg,
		dbPool,
	)
	if err != nil {
		slogger.ErrorNoCtx("Failed to create worker service", slogger.Fields{"error": err.Error()})
		return
	}

	if err := startWorkerService(workerService); err != nil {
		slogger.ErrorNoCtx("Failed to start worker service", slogger.Fields{"error": err.Error()})
		return
	}

	// Start async batch job poller and retry poller
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create and start BatchSubmitter for rate-limited batch submissions
	submitterConfig := worker.BatchSubmitterConfig{
		PollInterval:             cfg.BatchProcessing.SubmitterPollInterval,
		MaxConcurrentSubmissions: cfg.BatchProcessing.MaxConcurrentSubmissions,
		InitialBackoff:           cfg.BatchProcessing.SubmissionInitialBackoff,
		MaxBackoff:               cfg.BatchProcessing.SubmissionMaxBackoff,
		MaxSubmissionAttempts:    cfg.BatchProcessing.MaxSubmissionAttempts,
	}
	batchSubmitter := worker.NewBatchSubmitter(batchProgressRepo, batchEmbeddingService, submitterConfig)
	if err := batchSubmitter.Start(ctx); err != nil {
		slogger.ErrorNoCtx("Failed to start batch submitter", slogger.Fields{"error": err.Error()})
		// Continue anyway - batches will accumulate but won't be submitted
	} else {
		slogger.InfoNoCtx("Batch submitter started successfully", slogger.Fields{
			"poll_interval":              submitterConfig.PollInterval,
			"max_concurrent_submissions": submitterConfig.MaxConcurrentSubmissions,
			"initial_backoff":            submitterConfig.InitialBackoff,
			"max_backoff":                submitterConfig.MaxBackoff,
		})
	}
	defer batchSubmitter.Stop()

	// Start batch poller for async Gemini batch processing
	// Create it first so recovery service can use it to process completed batches
	pollerConfig := worker.BatchPollerConfig{
		PollInterval:       cfg.BatchProcessing.PollerInterval,
		MaxConcurrentPolls: cfg.BatchProcessing.MaxConcurrentPolls,
	}
	batchPoller := worker.NewBatchPoller(batchProgressRepo, chunkRepo, batchEmbeddingService, pollerConfig)

	// Run recovery service to check for orphaned batches before starting normal processing
	if err := runBatchRecovery(ctx, batchProgressRepo, batchEmbeddingService, batchPoller); err != nil {
		slogger.ErrorNoCtx("Batch recovery failed", slogger.Fields{"error": err.Error()})
		// Don't fail startup, but log the error - batches might be stuck
	}

	// Start the batch poller after recovery completes
	if err := batchPoller.Start(ctx); err != nil {
		slogger.ErrorNoCtx("Failed to start batch poller", slogger.Fields{"error": err.Error()})
		// Continue anyway - recovery might have processed pending batches
	} else {
		slogger.InfoNoCtx("Batch poller started successfully", slogger.Fields{
			"poll_interval":        pollerConfig.PollInterval,
			"max_concurrent_polls": pollerConfig.MaxConcurrentPolls,
		})
	}
	defer batchPoller.Stop()

	// Start retry batch polling goroutine
	startRetryBatchPolling(ctx, batchProgressRepo)

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

// createNATSConnection creates a NATS connection for use by both consumer and batch queue manager.
// This function reuses the connection logic from the messaging package with proper error handling.
func createNATSConnection(cfg *config.Config) (*nats.Conn, error) {
	connOpts := []nats.Option{
		nats.MaxReconnects(cfg.NATS.MaxReconnects),
		nats.ReconnectWait(cfg.NATS.ReconnectWait),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			slogger.InfoNoCtx("NATS connection disconnected", slogger.Fields{"error": err.Error()})
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slogger.InfoNoCtx("NATS connection reconnected", slogger.Fields{"connected": nc.ConnectedUrl()})
		}),
	}

	slogger.InfoNoCtx("Establishing NATS connection", slogger.Fields{"nats_url": cfg.NATS.URL})

	conn, err := nats.Connect(cfg.NATS.URL, connOpts...)
	if err != nil {
		// Normalize error message for consistency
		errMsg := err.Error()
		if strings.Contains(errMsg, "no servers available") {
			return nil, errors.New("failed to connect to NATS: connection refused")
		}
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	slogger.InfoNoCtx("NATS connection established successfully", slogger.Fields{
		"connected_url": conn.ConnectedUrl(),
		"servers":       conn.Servers(),
	})

	return conn, nil
}

// convertBatchProcessingConfig maps the YAML configuration to the outbound BatchConfig structure.
// This ensures the batch queue manager uses the correct configuration values from YAML files.
func convertBatchProcessingConfig(cfg *config.BatchProcessingConfig) *outbound.BatchConfig {
	// Get default configuration for fallback values
	defaultConfig := queue.DefaultBatchConfig()

	// Cap BatchTimeout at 5 minutes to match validation constraints
	maxAllowedTimeout := 5 * time.Minute
	batchTimeout := cfg.QueueLimits.MaxWaitTime
	if batchTimeout > maxAllowedTimeout {
		batchTimeout = maxAllowedTimeout
		slogger.InfoNoCtx("Capping BatchTimeout at maximum allowed value", slogger.Fields{
			"original_timeout": cfg.QueueLimits.MaxWaitTime,
			"capped_timeout":   batchTimeout,
			"max_allowed":      maxAllowedTimeout,
		})
	}

	batchConfig := &outbound.BatchConfig{
		// Use YAML queue limits, fallback to defaults
		MaxQueueSize:        cfg.QueueLimits.MaxQueueSize,
		BatchTimeout:        batchTimeout,
		ProcessingInterval:  defaultConfig.ProcessingInterval,
		EnableDynamicSizing: true,
		PriorityWeights:     defaultConfig.PriorityWeights,
	}

	// Configure batch sizes based on priority and YAML configuration
	// Use the default priority's batch size as the baseline
	if batchSizeConfig, exists := cfg.BatchSizes[cfg.DefaultPriority]; exists {
		batchConfig.MinBatchSize = batchSizeConfig.Min
		batchConfig.MaxBatchSize = batchSizeConfig.Max
	} else {
		// Fallback to default batch sizes if priority not found
		batchConfig.MinBatchSize = defaultConfig.MinBatchSize
		batchConfig.MaxBatchSize = defaultConfig.MaxBatchSize
	}

	return batchConfig
}

// createBatchQueueManager creates a BatchQueueManager for enhanced embedding processing.
// Only creates the manager if batch processing is enabled in configuration.
func createBatchQueueManager(
	cfg *config.Config,
	embeddingService outbound.EmbeddingService,
	natsConn *nats.Conn,
) (outbound.BatchQueueManager, error) {
	// Check if batch processing is enabled
	if !cfg.BatchProcessing.Enabled {
		slogger.InfoNoCtx("Batch processing disabled, using sequential embeddings", slogger.Fields{
			"enabled": false,
		})
		return nil, nil //nolint:nilnil // Valid case: disabled batch processing, no error
	}

	slogger.InfoNoCtx("Creating batch queue manager", slogger.Fields{
		"threshold_chunks": cfg.BatchProcessing.ThresholdChunks,
		"default_priority": cfg.BatchProcessing.DefaultPriority,
		"fallback_enabled": cfg.BatchProcessing.FallbackToSequential,
	})

	// Create batch processor
	batchProcessor := queue.NewEmbeddingServiceBatchProcessor(embeddingService)

	// Create batch queue manager with NATS connection
	batchQueueManager := queue.NewNATSBatchQueueManager(
		embeddingService,
		batchProcessor,
		natsConn,
	)

	// Convert YAML configuration to outbound.BatchConfig and apply it
	batchConfig := convertBatchProcessingConfig(&cfg.BatchProcessing)

	// Validate configuration
	if batchConfig.MaxQueueSize <= 0 {
		slogger.ErrorNoCtx("Invalid queue configuration: MaxQueueSize must be > 0", slogger.Fields{
			"max_queue_size": batchConfig.MaxQueueSize,
		})
		return nil, fmt.Errorf(
			"invalid queue configuration: MaxQueueSize must be > 0, got %d",
			batchConfig.MaxQueueSize,
		)
	}

	// Update the batch queue manager with the converted configuration
	if err := batchQueueManager.UpdateBatchConfiguration(context.Background(), batchConfig); err != nil {
		slogger.ErrorNoCtx("Failed to update batch queue configuration", slogger.Fields{
			"error":          err.Error(),
			"max_queue_size": batchConfig.MaxQueueSize,
			"batch_timeout":  batchConfig.BatchTimeout,
		})
		return nil, fmt.Errorf("failed to update batch queue configuration: %w", err)
	}

	// Start the batch queue manager
	if err := batchQueueManager.Start(context.Background()); err != nil {
		slogger.ErrorNoCtx("Failed to start batch queue manager", slogger.Fields{"error": err.Error()})
		return nil, fmt.Errorf("failed to start batch queue manager: %w", err)
	}

	slogger.InfoNoCtx("Batch queue manager created and started successfully", slogger.Fields{
		"batch_sizes":            cfg.BatchProcessing.BatchSizes,
		"queue_limits":           cfg.BatchProcessing.QueueLimits,
		"applied_max_queue_size": batchConfig.MaxQueueSize,
		"applied_batch_timeout":  batchConfig.BatchTimeout,
		"applied_min_batch_size": batchConfig.MinBatchSize,
		"applied_max_batch_size": batchConfig.MaxBatchSize,
	})

	return batchQueueManager, nil
}

// createWorkerService creates and configures the worker service with all dependencies.
func createWorkerService(
	cfg *config.Config,
	dbPool *pgxpool.Pool,
) (inbound.WorkerService, *repository.PostgreSQLBatchProgressRepository, outbound.EmbeddingService, outbound.BatchEmbeddingService, outbound.ChunkStorageRepository, error) {
	// Create repository implementations
	repoRepository := repository.NewPostgreSQLRepositoryRepository(dbPool)
	indexingJobRepository := repository.NewPostgreSQLIndexingJobRepository(dbPool)
	chunkStorageRepository := repository.NewPostgreSQLChunkRepository(dbPool)
	batchProgressRepo := repository.NewPostgreSQLBatchProgressRepository(dbPool)

	// Create git client implementation
	gitClient := gitclient.NewAuthenticatedGitClient()

	// Create code parser implementation
	codeParser, err := treesitter.NewTreeSitterCodeParser(context.Background())
	if err != nil {
		slogger.ErrorNoCtx("Failed to create TreeSitter code parser", slogger.Fields{"error": err.Error()})
		return nil, nil, nil, nil, nil, err
	}

	// Create embedding service and batch embedding service
	embeddingService, batchEmbeddingService := createEmbeddingService(cfg)

	// Create NATS connection for batch queue manager
	natsConn, err := createNATSConnection(cfg)
	if err != nil {
		slogger.ErrorNoCtx("Failed to create NATS connection", slogger.Fields{"error": err.Error()})
		// Continue without batch processing for now
		natsConn = nil
	}

	// Create batch queue manager (if enabled)
	var batchQueueManager outbound.BatchQueueManager
	if natsConn != nil {
		batchQueueManager, err = createBatchQueueManager(cfg, embeddingService, natsConn)
		if err != nil {
			slogger.ErrorNoCtx(
				"Failed to create batch queue manager, falling back to sequential",
				slogger.Fields{"error": err.Error()},
			)
			batchQueueManager = nil
		}
	} else {
		batchQueueManager = nil
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
		codeParser,             // Now using real TreeSitter CodeParser
		embeddingService,       // Using Gemini embedding service
		chunkStorageRepository, // Repository for storing chunks and embeddings
		&worker.JobProcessorBatchOptions{
			BatchConfig:           &cfg.BatchProcessing,
			BatchQueueManager:     batchQueueManager,
			BatchProgressRepo:     batchProgressRepo,
			BatchEmbeddingService: batchEmbeddingService,
		},
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
		return nil, nil, nil, nil, nil, err
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
		return nil, nil, nil, nil, nil, err
	}

	return workerService, batchProgressRepo, embeddingService, batchEmbeddingService, chunkStorageRepository, nil
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

// runBatchRecovery runs the batch recovery service on startup to check for orphaned batches.
func runBatchRecovery(
	ctx context.Context,
	batchProgressRepo outbound.BatchProgressRepository,
	batchEmbeddingService outbound.BatchEmbeddingService,
	batchPoller *worker.BatchPoller,
) error {
	recoveryService := worker.NewBatchRecoveryService(batchProgressRepo, batchEmbeddingService, batchPoller)
	return recoveryService.RecoverPendingBatches(ctx)
}

// startRetryBatchPolling starts a goroutine that polls for retry-ready batches.
func startRetryBatchPolling(ctx context.Context, batchProgressRepo *repository.PostgreSQLBatchProgressRepository) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute) // Poll every minute
		defer ticker.Stop()

		slogger.InfoNoCtx("Started retry batch polling", slogger.Fields{
			"poll_interval": "1m",
		})

		for {
			select {
			case <-ticker.C:
				// Poll for retry-ready batches
				retryBatch, err := batchProgressRepo.GetNextRetryBatch(ctx)
				if err != nil {
					slogger.Error(ctx, "Failed to get next retry batch", slogger.Fields{"error": err.Error()})
					continue
				}

				if retryBatch == nil {
					continue // No batches ready for retry
				}

				// Update status to processing
				err = batchProgressRepo.UpdateStatus(ctx, retryBatch.ID(), "processing")
				if err != nil {
					slogger.Error(ctx, "Failed to update retry batch status", slogger.Fields{"error": err.Error()})
					continue
				}

				// Log retry attempt
				slogger.Info(ctx, "Retrying batch", slogger.Fields{
					"batch_id":     retryBatch.ID(),
					"job_id":       retryBatch.IndexingJobID(),
					"batch_number": retryBatch.BatchNumber(),
					"retry_count":  retryBatch.RetryCount(),
				})

				// TODO: Trigger batch reprocessing (this requires job processor integration)
				// For now, just log that we found a retry-ready batch

			case <-ctx.Done():
				slogger.InfoNoCtx("Stopping retry batch polling", nil)
				return
			}
		}
	}()
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

// createEmbeddingService creates an embedding service using Gemini API.
// The worker service requires a valid Gemini API key to function properly.
func createEmbeddingService(cfg *config.Config) (outbound.EmbeddingService, outbound.BatchEmbeddingService) {
	// Try to create Gemini client first using Viper configuration
	geminiAPIKey := cfg.Gemini.APIKey

	// Fallback to environment variables for backward compatibility
	if geminiAPIKey == "" {
		geminiAPIKey = os.Getenv("GEMINI_API_KEY")
		if geminiAPIKey == "" {
			geminiAPIKey = os.Getenv("GOOGLE_API_KEY")
		}
	}

	// Exit if no API key is configured - this is a critical dependency
	if geminiAPIKey == "" {
		slogger.ErrorNoCtx("Gemini API key not configured - set CODECHUNK_GEMINI_API_KEY environment variable", nil)
		os.Exit(1)
	}

	config := &gemini.ClientConfig{
		APIKey:     geminiAPIKey,
		Model:      cfg.Gemini.Model,
		TaskType:   "RETRIEVAL_DOCUMENT",
		Timeout:    cfg.Gemini.Timeout,
		Dimensions: 768, // Default dimensions for gemini-embedding-001
	}

	client, err := gemini.NewClient(config)
	if err != nil {
		slogger.ErrorNoCtx("Failed to create Gemini client", slogger.Fields{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	var batchClient outbound.BatchEmbeddingService

	// Enable batch processing if configured
	if cfg.Gemini.Batch.Enabled {
		slogger.InfoNoCtx("Batch embeddings enabled", slogger.Fields{
			"input_dir":     cfg.Gemini.Batch.InputDir,
			"output_dir":    cfg.Gemini.Batch.OutputDir,
			"poll_interval": cfg.Gemini.Batch.PollInterval,
		})

		batchEmbeddingClient, err := gemini.NewBatchEmbeddingClient(
			client,
			cfg.Gemini.Batch.InputDir,
			cfg.Gemini.Batch.OutputDir,
		)
		if err != nil {
			slogger.ErrorNoCtx("Failed to create batch embedding client", slogger.Fields{
				"error": err.Error(),
			})
			// Fall back to nil - async batch processing won't be available
		} else {
			batchClient = batchEmbeddingClient
			slogger.InfoNoCtx("Batch embeddings successfully enabled", slogger.Fields{
				"mode": "async_batch_api",
			})
		}
	}

	slogger.InfoNoCtx("Using Gemini embedding service", slogger.Fields{
		"model":             config.Model,
		"batch_api_enabled": batchClient != nil,
	})
	return client, batchClient
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
