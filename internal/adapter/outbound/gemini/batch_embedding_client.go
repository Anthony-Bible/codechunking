package gemini

import (
	"bufio"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/genai"
)

// BatchEmbeddingClient implements the BatchEmbeddingService interface using Google GenAI Batches API.
type BatchEmbeddingClient struct {
	*Client // Embed the base client for shared functionality

	// Storage configuration for batch files
	inputDir  string // Directory for input files
	outputDir string // Directory for output files
}

// NewBatchEmbeddingClient creates a new batch embedding client.
func NewBatchEmbeddingClient(baseClient *Client, inputDir, outputDir string) (*BatchEmbeddingClient, error) {
	if baseClient == nil {
		return nil, errors.New("base client cannot be nil")
	}

	// Use default directories if not provided
	if inputDir == "" {
		inputDir = filepath.Join(os.TempDir(), "batch_embeddings", "input")
	}
	if outputDir == "" {
		outputDir = filepath.Join(os.TempDir(), "batch_embeddings", "output")
	}

	// Create directories if they don't exist
	if err := os.MkdirAll(inputDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create input directory: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &BatchEmbeddingClient{
		Client:    baseClient,
		inputDir:  inputDir,
		outputDir: outputDir,
	}, nil
}

// CreateBatchEmbeddingJob creates a new batch embedding job from a list of texts.
func (c *BatchEmbeddingClient) CreateBatchEmbeddingJob(
	ctx context.Context,
	texts []string,
	options outbound.EmbeddingOptions,
) (*outbound.BatchEmbeddingJob, error) {
	slogger.Info(ctx, "Creating batch embedding job", slogger.Fields{
		"text_count": len(texts),
		"model":      c.config.Model,
	})

	// Validate input
	if len(texts) == 0 {
		return nil, &outbound.EmbeddingError{
			Code:      "empty_texts",
			Type:      "validation",
			Message:   "texts array cannot be empty",
			Retryable: false,
		}
	}

	// Create batch requests
	requests := make([]*outbound.BatchEmbeddingRequest, len(texts))
	for i, text := range texts {
		requests[i] = &outbound.BatchEmbeddingRequest{
			RequestID: fmt.Sprintf("req_%d_%d", time.Now().Unix(), i),
			Text:      text,
		}
	}

	// Write requests to JSONL file
	inputFile, err := c.writeRequestsToFile(ctx, requests, options)
	if err != nil {
		return nil, fmt.Errorf("failed to write requests to file: %w", err)
	}

	// Get the cached GenAI client
	genaiClient := c.getGenaiClient()

	// Determine model to use
	model := options.Model
	if model == "" {
		model = c.config.Model
	}

	// Create batch job source
	// Note: The Google GenAI SDK expects either GCS URIs, BigQuery URIs, or file names
	// For now, we'll use the FileName field for local file uploads
	source := &genai.BatchJobSource{
		Format:   "jsonl",
		FileName: inputFile, // This will be uploaded by the SDK
	}

	// Create the batch job using the GenAI SDK
	// The Create method is used for all batch operations (not just embeddings)
	batchJob, err := genaiClient.Batches.Create(ctx, model, source, nil)
	if err != nil {
		embeddingErr := c.convertSDKError(err)
		slogger.Error(ctx, "Failed to create batch embedding job", slogger.Fields{
			"error": embeddingErr.Error(),
		})
		return nil, embeddingErr
	}

	// Convert SDK BatchJob to our domain type
	job := c.convertBatchJobToDomain(batchJob, options, len(texts))

	slogger.Info(ctx, "Batch embedding job created successfully", slogger.Fields{
		"job_id": job.JobID,
		"state":  job.State,
	})

	return job, nil
}

// GetBatchJobStatus retrieves the current status of a batch embedding job.
func (c *BatchEmbeddingClient) GetBatchJobStatus(
	ctx context.Context,
	jobID string,
) (*outbound.BatchEmbeddingJob, error) {
	slogger.Info(ctx, "Getting batch job status", slogger.Fields{
		"job_id": jobID,
	})

	// Validate input
	if strings.TrimSpace(jobID) == "" {
		return nil, &outbound.EmbeddingError{
			Code:      "empty_job_id",
			Type:      "validation",
			Message:   "job ID cannot be empty",
			Retryable: false,
		}
	}

	// Get the cached GenAI client
	genaiClient := c.getGenaiClient()

	// Get batch job status from the SDK
	batchJob, err := genaiClient.Batches.Get(ctx, jobID, nil)
	if err != nil {
		embeddingErr := c.convertSDKError(err)
		slogger.Error(ctx, "Failed to get batch job status", slogger.Fields{
			"job_id": jobID,
			"error":  embeddingErr.Error(),
		})
		return nil, embeddingErr
	}

	// Convert to domain type
	job := c.convertBatchJobToDomain(batchJob, outbound.EmbeddingOptions{}, 0)

	slogger.Info(ctx, "Batch job status retrieved", slogger.Fields{
		"job_id": job.JobID,
		"state":  job.State,
	})

	return job, nil
}

// ListBatchJobs lists all batch embedding jobs with optional filtering.
func (c *BatchEmbeddingClient) ListBatchJobs(
	ctx context.Context,
	filter *outbound.BatchJobFilter,
) ([]*outbound.BatchEmbeddingJob, error) {
	slogger.Info(ctx, "Listing batch jobs", slogger.Fields{})

	// Get the cached GenAI client
	genaiClient := c.getGenaiClient()

	// Create iterator for all batch jobs
	iter := genaiClient.Batches.All(ctx)

	jobs := make([]*outbound.BatchEmbeddingJob, 0)

	// Iterate through all batch jobs using range over iterator
	for batchJob, err := range iter {
		if err != nil {
			embeddingErr := c.convertSDKError(err)
			slogger.Error(ctx, "Failed to iterate batch jobs", slogger.Fields{
				"error": embeddingErr.Error(),
			})
			return nil, embeddingErr
		}

		// Convert to domain type
		job := c.convertBatchJobToDomain(batchJob, outbound.EmbeddingOptions{}, 0)

		// Apply filters if provided
		if filter != nil && !c.matchesFilter(job, filter) {
			continue
		}

		jobs = append(jobs, job)

		// Apply limit if specified
		if filter != nil && filter.Limit > 0 && len(jobs) >= filter.Limit {
			break
		}
	}

	slogger.Info(ctx, "Batch jobs listed", slogger.Fields{
		"count": len(jobs),
	})

	return jobs, nil
}

// GetBatchJobResults retrieves the results of a completed batch embedding job.
func (c *BatchEmbeddingClient) GetBatchJobResults(
	ctx context.Context,
	jobID string,
) ([]*outbound.EmbeddingResult, error) {
	slogger.Info(ctx, "Getting batch job results", slogger.Fields{
		"job_id": jobID,
	})

	// First, get the job status to check if it's completed
	job, err := c.GetBatchJobStatus(ctx, jobID)
	if err != nil {
		return nil, err
	}

	// Check if job is completed
	if job.State != outbound.BatchJobStateCompleted {
		return nil, &outbound.EmbeddingError{
			Code:      "job_not_completed",
			Type:      "validation",
			Message:   fmt.Sprintf("job is not completed (current state: %s)", job.State),
			Retryable: true,
		}
	}

	// Check if output file URI is available
	if job.OutputFileURI == "" {
		return nil, &outbound.EmbeddingError{
			Code:      "no_output_file",
			Type:      "response",
			Message:   "job completed but no output file available",
			Retryable: false,
		}
	}

	// Download and parse the output file
	results, err := c.downloadAndParseResults(ctx, job.OutputFileURI)
	if err != nil {
		return nil, fmt.Errorf("failed to download and parse results: %w", err)
	}

	slogger.Info(ctx, "Batch job results retrieved", slogger.Fields{
		"job_id":       jobID,
		"result_count": len(results),
	})

	return results, nil
}

// CancelBatchJob cancels a running batch embedding job.
func (c *BatchEmbeddingClient) CancelBatchJob(
	ctx context.Context,
	jobID string,
) error {
	slogger.Info(ctx, "Cancelling batch job", slogger.Fields{
		"job_id": jobID,
	})

	// Validate input
	if strings.TrimSpace(jobID) == "" {
		return &outbound.EmbeddingError{
			Code:      "empty_job_id",
			Type:      "validation",
			Message:   "job ID cannot be empty",
			Retryable: false,
		}
	}

	// Get the cached GenAI client
	genaiClient := c.getGenaiClient()

	// Cancel the batch job
	err := genaiClient.Batches.Cancel(ctx, jobID, nil)
	if err != nil {
		embeddingErr := c.convertSDKError(err)
		slogger.Error(ctx, "Failed to cancel batch job", slogger.Fields{
			"job_id": jobID,
			"error":  embeddingErr.Error(),
		})
		return embeddingErr
	}

	slogger.Info(ctx, "Batch job cancelled successfully", slogger.Fields{
		"job_id": jobID,
	})

	return nil
}

// DeleteBatchJob deletes a batch embedding job and its associated resources.
func (c *BatchEmbeddingClient) DeleteBatchJob(
	ctx context.Context,
	jobID string,
) error {
	slogger.Info(ctx, "Deleting batch job", slogger.Fields{
		"job_id": jobID,
	})

	// Validate input
	if strings.TrimSpace(jobID) == "" {
		return &outbound.EmbeddingError{
			Code:      "empty_job_id",
			Type:      "validation",
			Message:   "job ID cannot be empty",
			Retryable: false,
		}
	}

	// Get the cached GenAI client
	genaiClient := c.getGenaiClient()

	// Delete the batch job
	_, err := genaiClient.Batches.Delete(ctx, jobID, nil)
	if err != nil {
		embeddingErr := c.convertSDKError(err)
		slogger.Error(ctx, "Failed to delete batch job", slogger.Fields{
			"job_id": jobID,
			"error":  embeddingErr.Error(),
		})
		return embeddingErr
	}

	slogger.Info(ctx, "Batch job deleted successfully", slogger.Fields{
		"job_id": jobID,
	})

	return nil
}

// WaitForBatchJob waits for a batch job to complete with timeout and polling.
func (c *BatchEmbeddingClient) WaitForBatchJob(
	ctx context.Context,
	jobID string,
	pollInterval time.Duration,
) (*outbound.BatchEmbeddingJob, error) {
	slogger.Info(ctx, "Waiting for batch job to complete", slogger.Fields{
		"job_id":        jobID,
		"poll_interval": pollInterval,
	})

	// Validate input
	if strings.TrimSpace(jobID) == "" {
		return nil, &outbound.EmbeddingError{
			Code:      "empty_job_id",
			Type:      "validation",
			Message:   "job ID cannot be empty",
			Retryable: false,
		}
	}

	if pollInterval <= 0 {
		pollInterval = 5 * time.Second // Default poll interval
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, &outbound.EmbeddingError{
				Code:      "context_cancelled",
				Type:      "timeout",
				Message:   "context cancelled while waiting for batch job",
				Retryable: false,
				Cause:     ctx.Err(),
			}
		case <-ticker.C:
			// Poll for job status
			job, err := c.GetBatchJobStatus(ctx, jobID)
			if err != nil {
				return nil, err
			}

			// Check if job is in terminal state
			if job.State.IsTerminal() {
				slogger.Info(ctx, "Batch job completed", slogger.Fields{
					"job_id": jobID,
					"state":  job.State,
				})
				return job, nil
			}

			// Log progress
			if job.Progress != nil {
				slogger.Info(ctx, "Batch job progress", slogger.Fields{
					"job_id":           jobID,
					"percent_complete": job.Progress.PercentComplete,
					"items_remaining":  job.Progress.ItemsRemaining,
				})
			}
		}
	}
}

// Helper methods

// writeRequestsToFile writes batch requests to a JSONL file.
func (c *BatchEmbeddingClient) writeRequestsToFile(
	ctx context.Context,
	requests []*outbound.BatchEmbeddingRequest,
	options outbound.EmbeddingOptions,
) (string, error) {
	// Create input file
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("batch_input_%s.jsonl", timestamp)
	filepath := filepath.Join(c.inputDir, filename)

	file, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create input file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Determine model and task type
	model := options.Model
	if model == "" {
		model = c.config.Model
	}
	taskType := convertTaskType(options.TaskType)

	// Write each request as a JSONL line
	for _, req := range requests {
		// Create the embedding request structure according to Google GenAI format
		embeddingReq := map[string]interface{}{
			"custom_id": req.RequestID,
			"method":    "POST",
			"url":       fmt.Sprintf("/v1/models/%s:embedContent", model),
			"body": map[string]interface{}{
				"model": model,
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{"text": req.Text},
					},
				},
				"task_type": taskType,
			},
		}

		// Add output dimensionality if specified
		if c.config.Dimensions > 0 {
			if body, ok := embeddingReq["body"].(map[string]interface{}); ok {
				body["output_dimensionality"] = c.config.Dimensions
			}
		}

		// Marshal to JSON and write
		jsonData, err := json.Marshal(embeddingReq)
		if err != nil {
			return "", fmt.Errorf("failed to marshal request: %w", err)
		}

		if _, err := writer.WriteString(string(jsonData) + "\n"); err != nil {
			return "", fmt.Errorf("failed to write request: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return "", fmt.Errorf("failed to flush writer: %w", err)
	}

	slogger.Info(ctx, "Batch requests written to file", slogger.Fields{
		"file":          filepath,
		"request_count": len(requests),
	})

	return filepath, nil
}

// downloadAndParseResults downloads and parses batch job results.
func (c *BatchEmbeddingClient) downloadAndParseResults(
	ctx context.Context,
	outputFileURI string,
) ([]*outbound.EmbeddingResult, error) {
	// For file-based batch processing, the output file URI might be a local path
	// or a cloud storage URI. For now, we'll assume it's a local path.
	// TODO: Add support for downloading from cloud storage URIs

	file, err := os.Open(outputFileURI)
	if err != nil {
		return nil, fmt.Errorf("failed to open output file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	results := make([]*outbound.EmbeddingResult, 0)

	// Read each JSONL line
	for scanner.Scan() {
		var response map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			slogger.Error(ctx, "Failed to parse response line", slogger.Fields{
				"error": err.Error(),
			})
			continue
		}

		// Extract embedding from response
		result, err := c.parseEmbeddingResponse(response)
		if err != nil {
			slogger.Error(ctx, "Failed to parse embedding response", slogger.Fields{
				"error": err.Error(),
			})
			continue
		}

		results = append(results, result)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading output file: %w", err)
	}

	return results, nil
}

// parseEmbeddingResponse parses an embedding response from the batch output.
func (c *BatchEmbeddingClient) parseEmbeddingResponse(
	response map[string]interface{},
) (*outbound.EmbeddingResult, error) {
	// Extract custom_id (request ID)
	requestID, _ := response["custom_id"].(string)

	// Extract response body
	respBody, ok := response["response"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid response structure: missing response body")
	}

	// Extract body from response
	body, ok := respBody["body"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid response structure: missing body")
	}

	// Extract embedding
	embeddingData, ok := body["embedding"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid response structure: missing embedding")
	}

	// Extract values
	valuesInterface, ok := embeddingData["values"].([]interface{})
	if !ok {
		return nil, errors.New("invalid response structure: missing values")
	}

	// Convert to float64 slice
	values := make([]float64, len(valuesInterface))
	for i, v := range valuesInterface {
		if floatVal, ok := v.(float64); ok {
			values[i] = floatVal
		} else {
			return nil, fmt.Errorf("invalid value type at index %d", i)
		}
	}

	// Create embedding result
	result := &outbound.EmbeddingResult{
		Vector:      values,
		Dimensions:  len(values),
		Model:       c.config.Model,
		GeneratedAt: time.Now(),
		RequestID:   requestID,
	}

	return result, nil
}

// convertBatchJobToDomain converts a GenAI SDK BatchJob to our domain type.
func (c *BatchEmbeddingClient) convertBatchJobToDomain(
	batchJob *genai.BatchJob,
	options outbound.EmbeddingOptions,
	totalCount int,
) *outbound.BatchEmbeddingJob {
	if batchJob == nil {
		return nil
	}

	// Convert state
	state := c.convertBatchJobState(batchJob.State)

	job := &outbound.BatchEmbeddingJob{
		JobID:      batchJob.Name,
		JobName:    batchJob.Name,
		Model:      c.config.Model,
		State:      state,
		TotalCount: totalCount,
		CreatedAt:  batchJob.CreateTime,
		UpdatedAt:  batchJob.UpdateTime,
		Options:    options,
	}

	// Set completion time if available
	if !batchJob.EndTime.IsZero() {
		job.CompletedAt = &batchJob.EndTime
	}

	// Set output file URI if available
	if batchJob.Dest != nil {
		if batchJob.Dest.GCSURI != "" {
			job.OutputFileURI = batchJob.Dest.GCSURI
		} else if batchJob.Dest.FileName != "" {
			job.OutputFileURI = batchJob.Dest.FileName
		}
	}

	// Set error message if available
	if batchJob.Error != nil {
		job.ErrorMessage = batchJob.Error.Message
	}

	return job
}

// convertBatchJobState converts a GenAI SDK job state to our domain state.
func (c *BatchEmbeddingClient) convertBatchJobState(sdkState genai.JobState) outbound.BatchJobState {
	switch sdkState {
	case genai.JobStatePending, genai.JobStateQueued, genai.JobStateUnspecified:
		return outbound.BatchJobStatePending
	case genai.JobStateRunning, genai.JobStateUpdating:
		return outbound.BatchJobStateProcessing
	case genai.JobStateSucceeded, genai.JobStatePartiallySucceeded:
		return outbound.BatchJobStateCompleted
	case genai.JobStateFailed, genai.JobStateExpired:
		return outbound.BatchJobStateFailed
	case genai.JobStateCancelled, genai.JobStateCancelling:
		return outbound.BatchJobStateCancelled
	case genai.JobStatePaused:
		return outbound.BatchJobStatePending
	default:
		return outbound.BatchJobStatePending
	}
}

// matchesFilter checks if a job matches the given filter.
func (c *BatchEmbeddingClient) matchesFilter(job *outbound.BatchEmbeddingJob, filter *outbound.BatchJobFilter) bool {
	// Filter by states
	if len(filter.States) > 0 {
		matches := false
		for _, state := range filter.States {
			if job.State == state {
				matches = true
				break
			}
		}
		if !matches {
			return false
		}
	}

	// Filter by model
	if filter.Model != "" && job.Model != filter.Model {
		return false
	}

	// Filter by created after
	if filter.CreatedAfter != nil && job.CreatedAt.Before(*filter.CreatedAfter) {
		return false
	}

	// Filter by created before
	if filter.CreatedBefore != nil && job.CreatedAt.After(*filter.CreatedBefore) {
		return false
	}

	return true
}
