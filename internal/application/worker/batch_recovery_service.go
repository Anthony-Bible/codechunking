package worker

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/entity"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"time"
)

// BatchRecoveryService handles recovery of pending batch jobs on worker startup.
// It checks for batches that are in 'processing' status and queries their actual
// status from Gemini API. If batches are stuck or orphaned, it updates their
// status or schedules retries as appropriate.
type BatchRecoveryService struct {
	batchProgressRepo     outbound.BatchProgressRepository
	batchEmbeddingService outbound.BatchEmbeddingService
	batchPoller           *BatchPoller
}

// NewBatchRecoveryService creates a new batch recovery service instance.
func NewBatchRecoveryService(
	batchProgressRepo outbound.BatchProgressRepository,
	batchEmbeddingService outbound.BatchEmbeddingService,
	batchPoller *BatchPoller,
) *BatchRecoveryService {
	return &BatchRecoveryService{
		batchProgressRepo:     batchProgressRepo,
		batchEmbeddingService: batchEmbeddingService,
		batchPoller:           batchPoller,
	}
}

// RecoverPendingBatches runs recovery for all pending Gemini batches.
// It queries batches in 'processing' status and checks their actual status
// with the Gemini API. For completed batches, it processes the results.
// For failed batches, it marks them as failed. For in-progress batches,
// it ensures they will be picked up by the batch poller.
func (r *BatchRecoveryService) RecoverPendingBatches(ctx context.Context) error {
	slogger.InfoNoCtx("Starting batch recovery check", slogger.Fields{
		"operation": "recover_pending_batches",
	})

	// Get all pending Gemini batches from database
	pendingBatches, err := r.batchProgressRepo.GetPendingGeminiBatches(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending batches: %w", err)
	}

	if len(pendingBatches) == 0 {
		slogger.InfoNoCtx("No pending batches found, recovery complete", slogger.Fields{
			"operation": "recover_pending_batches",
		})
		return nil
	}

	slogger.InfoNoCtx(fmt.Sprintf("Found %d pending batches to check", len(pendingBatches)), slogger.Fields{
		"operation":     "recover_pending_batches",
		"batch_count":   len(pendingBatches),
		"recovery_mode": false,
	})

	// Process each pending batch
	recoveredCount := 0
	failedCount := 0
	inProgressCount := 0

	for _, batch := range pendingBatches {
		if err := r.recoverBatch(ctx, batch); err != nil {
			slogger.Error(ctx, "Failed to recover batch", slogger.Fields{
				"batch_id":        batch.ID(),
				"indexing_job_id": batch.IndexingJobID(),
				"batch_number":    batch.BatchNumber(),
				"error":           err.Error(),
			})
			failedCount++
		} else {
			recoveredCount++
			if batch.Status() == entity.StatusProcessing {
				inProgressCount++
			}
		}
	}

	slogger.InfoNoCtx("Batch recovery completed", slogger.Fields{
		"operation":         "recover_pending_batches",
		"total_checked":     len(pendingBatches),
		"recovered":         recoveredCount,
		"still_in_progress": inProgressCount,
		"failed_to_recover": failedCount,
	})

	return nil
}

// recoverBatch recovers a single batch by checking its status with Gemini API.
func (r *BatchRecoveryService) recoverBatch(ctx context.Context, batch *entity.BatchJobProgress) error {
	geminiBatchJobID := batch.GeminiBatchJobID()
	if geminiBatchJobID == nil || *geminiBatchJobID == "" {
		slogger.Warn(ctx, "Batch has no Gemini job ID, marking as failed", slogger.Fields{
			"batch_id":     batch.ID(),
			"batch_number": batch.BatchNumber(),
		})
		return r.batchProgressRepo.UpdateStatus(ctx, batch.ID(), entity.StatusFailed)
	}

	// Check batch status with Gemini API
	status, err := r.batchEmbeddingService.GetBatchJobStatus(ctx, *geminiBatchJobID)
	if err != nil {
		// If we can't check status, leave it as-is for the poller to handle
		// on its next run (might be a transient API error)
		slogger.Warn(ctx, "Could not check batch status, will retry later", slogger.Fields{
			"batch_id":            batch.ID(),
			"gemini_batch_job_id": *geminiBatchJobID,
			"error":               err.Error(),
		})
		return nil // Don't treat this as a failure, let poller handle it
	}

	slogger.Debug(ctx, "Checked batch status from Gemini", slogger.Fields{
		"batch_id":            batch.ID(),
		"gemini_batch_job_id": *geminiBatchJobID,
		"status":              status.State,
		"batch_status":        batch.Status(),
	})

	switch status.State {
	case "SUCCEEDED":
		// Batch completed successfully, process it immediately
		slogger.Info(ctx, "Batch completed, processing results", slogger.Fields{
			"batch_id":            batch.ID(),
			"gemini_batch_job_id": *geminiBatchJobID,
		})

		// Submit to batch poller for processing
		if err := r.batchPoller.ProcessBatchNow(ctx, batch); err != nil {
			return fmt.Errorf("failed to process completed batch: %w", err)
		}

	case "FAILED", "CANCELLED", "EXPIRED":
		// Batch has definitively failed
		errorMsg := fmt.Sprintf("batch job %s: %s", status.State, status.ErrorMessage)
		slogger.Error(ctx, "Batch failed permanently", slogger.Fields{
			"batch_id":            batch.ID(),
			"gemini_batch_job_id": *geminiBatchJobID,
			"error":               errorMsg,
		})

		// Mark as failed in database
		return r.batchProgressRepo.UpdateStatus(ctx, batch.ID(), entity.StatusFailed)

	case "RUNNING", "QUEUED", "PROCESSING", "PENDING", "UPDATING":
		// Still in progress - no action needed, will be picked up by poller
		slogger.Debug(ctx, "Batch still in progress", slogger.Fields{
			"batch_id":            batch.ID(),
			"gemini_batch_job_id": *geminiBatchJobID,
			"state":               status.State,
		})

	case "COMPLETED", "PARTIALLY_SUCCEEDED":
		// Batch completed - will be processed by poller
		slogger.Debug(ctx, "Batch completed, will be processed by poller", slogger.Fields{
			"batch_id":            batch.ID(),
			"gemini_batch_job_id": *geminiBatchJobID,
			"state":               status.State,
		})

	default:
		// Unknown state - log and let poller handle
		slogger.Warn(ctx, "Unknown batch status", slogger.Fields{
			"batch_id":            batch.ID(),
			"gemini_batch_job_id": *geminiBatchJobID,
			"state":               status.State,
		})
	}

	return nil
}

// RunOnStartup runs the recovery service when the worker starts.
// This ensures any pending batches from a previous worker run are recovered.
func (r *BatchRecoveryService) RunOnStartup(ctx context.Context) error {
	// Add delay to let systems stabilize before recovery
	// This prevents race conditions during startup
	startupDelay := 5 * time.Second

	slogger.InfoNoCtx("Batch recovery service starting", slogger.Fields{
		"startup_delay": startupDelay,
		"recovery_mode": true,
	})

	time.Sleep(startupDelay)

	return r.RecoverPendingBatches(ctx)
}

// GetRecoveryInfo returns information about recovery status and statistics.
func (r *BatchRecoveryService) GetRecoveryInfo(ctx context.Context) (map[string]interface{}, error) {
	pendingBatches, err := r.batchProgressRepo.GetPendingGeminiBatches(ctx)
	if err != nil {
		return nil, err
	}

	// Group by status
	statusCounts := make(map[string]int)
	var ageSum time.Duration
	now := time.Now()

	for _, batch := range pendingBatches {
		status := batch.Status()
		statusCounts[status]++

		// Calculate age for average
		age := now.Sub(batch.CreatedAt())
		ageSum += age
	}

	avgAgeMinutes := 0.0
	if len(pendingBatches) > 0 {
		avgAgeMinutes = ageSum.Minutes() / float64(len(pendingBatches))
	}

	return map[string]interface{}{
		"total_pending":       len(pendingBatches),
		"status_breakdown":    statusCounts,
		"average_age_minutes": avgAgeMinutes,
		"last_checked":        now,
	}, nil
}
