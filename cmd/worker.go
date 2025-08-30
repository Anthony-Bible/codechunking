// Package cmd provides command-line interface functionality for the codechunking application.
/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"codechunking/internal/adapter/inbound/messaging"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/service"
	"codechunking/internal/application/worker"
	"codechunking/internal/config"
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
			// Load configuration
			cfg := config.New(viper.GetViper())

			slogger.InfoNoCtx("Starting worker service", slogger.Fields{
				"concurrency": cfg.Worker.Concurrency,
				"queue_group": cfg.Worker.QueueGroup,
			})

			// Create job processor (simplified for GREEN phase)
			jobProcessorConfig := worker.JobProcessorConfig{
				WorkspaceDir:      "/tmp/codechunking-workspace",
				MaxConcurrentJobs: cfg.Worker.Concurrency,
				JobTimeout:        cfg.Worker.JobTimeout,
			}

			jobProcessor := worker.NewDefaultJobProcessor(
				jobProcessorConfig,
				nil, // Repository will be injected in BLUE phase
				nil, // Repository will be injected in BLUE phase
				nil, // GitClient will be injected in BLUE phase
				nil, // CodeParser will be injected in BLUE phase
				nil, // EmbeddingGenerator will be injected in BLUE phase
			)

			// Create consumer configuration
			consumerConfig := messaging.ConsumerConfig{
				Subject:     "indexing.job",
				QueueGroup:  cfg.Worker.QueueGroup,
				DurableName: "indexing-consumer",
				AckWait:     30 * time.Second,
				MaxDeliver:  3,
			}

			// Create consumer (simplified for GREEN phase)
			consumer, err := messaging.NewNATSConsumer(consumerConfig, cfg.NATS, jobProcessor)
			if err != nil {
				slogger.ErrorNoCtx("Failed to create consumer", slogger.Fields{"error": err.Error()})
				return
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
			}

			workerService := service.NewDefaultWorkerService(workerServiceConfig, cfg.NATS, jobProcessor)

			// Add consumer to service
			if err := workerService.AddConsumer(consumer); err != nil {
				slogger.ErrorNoCtx("Failed to add consumer", slogger.Fields{"error": err.Error()})
				return
			}

			// Start worker service
			ctx := context.Background()
			if err := workerService.Start(ctx); err != nil {
				slogger.ErrorNoCtx("Failed to start worker service", slogger.Fields{"error": err.Error()})
				return
			}

			slogger.InfoNoCtx("Worker service started successfully", nil)

			// Keep running (simplified for GREEN phase)
			select {}
		},
	}
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
