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

	"github.com/google/uuid"
	"google.golang.org/genai"
)

const (
	MaxBatchSize     = 500
	WarningThreshold = 400
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
	batchID uuid.UUID,
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

	// Validate batch size
	if len(texts) > MaxBatchSize {
		return nil, &outbound.EmbeddingError{
			Code: "batch_size_exceeded",
			Type: "validation",
			Message: fmt.Sprintf(
				"batch size %d exceeds maximum batch size of %d. Consider splitting into multiple batches.",
				len(texts),
				MaxBatchSize,
			),
			Retryable: false,
		}
	}

	// Log warning for large batches approaching limit
	if len(texts) > WarningThreshold {
		slogger.Warn(ctx, "Batch size approaching limit", slogger.Fields{
			"batch_size": len(texts),
			"max_size":   MaxBatchSize,
			"threshold":  WarningThreshold,
		})
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
	inputFilePath, inputFileName, err := c.writeRequestsToFile(ctx, requests, options, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to write requests to file: %w", err)
	}

	// Validate UUID name length (Google Gemini API requirement: max 40 characters for file ID)
	// The UUID-based name will be used as the file ID
	uuidFileName := fmt.Sprintf("%s.jsonl", batchID.String())
	if len(uuidFileName) > 40 {
		return nil, &outbound.EmbeddingError{
			Code:      "uuid_filename_too_long",
			Type:      "validation",
			Message:   fmt.Sprintf("UUID-based filename exceeds 40 character limit: %d characters", len(uuidFileName)),
			Retryable: false,
		}
	}

	// Also validate display name length for consistency
	if len(inputFileName) > 100 { // Allow longer display names since they're not used as file IDs
		return nil, &outbound.EmbeddingError{
			Code:      "display_name_too_long",
			Type:      "validation",
			Message:   fmt.Sprintf("display name exceeds 100 character limit: %d characters", len(inputFileName)),
			Retryable: false,
		}
	}

	// Upload file and create batch job
	job, err := c.uploadAndCreateBatchJob(ctx, inputFilePath, inputFileName, options, len(texts), batchID, nil)
	if err != nil {
		return nil, err
	}

	slogger.Info(ctx, "Batch embedding job created successfully", slogger.Fields{
		"job_id": job.JobID,
		"state":  job.State,
	})

	return job, nil
}

// CreateBatchEmbeddingJobWithRequests creates a new batch embedding job from pre-formed requests.
// This method allows callers to specify custom RequestIDs for tracking purposes.
func (c *BatchEmbeddingClient) CreateBatchEmbeddingJobWithRequests(
	ctx context.Context,
	requests []*outbound.BatchEmbeddingRequest,
	options outbound.EmbeddingOptions,
	batchID uuid.UUID,
) (*outbound.BatchEmbeddingJob, error) {
	slogger.Info(ctx, "Creating batch embedding job with custom requests", slogger.Fields{
		"request_count": len(requests),
		"model":         c.config.Model,
	})

	// Validate input
	if len(requests) == 0 {
		return nil, &outbound.EmbeddingError{
			Code:      "empty_requests",
			Type:      "validation",
			Message:   "requests array cannot be empty",
			Retryable: false,
		}
	}

	// Validate batch size
	if len(requests) > MaxBatchSize {
		return nil, &outbound.EmbeddingError{
			Code: "batch_size_exceeded",
			Type: "validation",
			Message: fmt.Sprintf(
				"batch size %d exceeds maximum batch size of %d. Consider splitting into multiple batches.",
				len(requests),
				MaxBatchSize,
			),
			Retryable: false,
		}
	}

	// Log warning for large batches approaching limit
	if len(requests) > WarningThreshold {
		slogger.Warn(ctx, "Batch size approaching limit", slogger.Fields{
			"batch_size": len(requests),
			"max_size":   MaxBatchSize,
			"threshold":  WarningThreshold,
		})
	}

	// Write requests to JSONL file
	inputFilePath, inputFileName, err := c.writeRequestsToFile(ctx, requests, options, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to write requests to file: %w", err)
	}

	// Validate filename length (Google Gemini API requirement: max 40 characters for file ID)
	if len(inputFileName) > 40 {
		return nil, &outbound.EmbeddingError{
			Code:      "filename_too_long",
			Type:      "validation",
			Message:   fmt.Sprintf("filename exceeds 40 character limit: %d characters", len(inputFileName)),
			Retryable: false,
		}
	}

	// Upload file and create batch job
	job, err := c.uploadAndCreateBatchJob(ctx, inputFilePath, inputFileName, options, len(requests), batchID, nil)
	if err != nil {
		return nil, err
	}

	// Ensure the InputFileURI is set for callers to persist
	// The file URI is derived from the batch ID
	if job.InputFileURI == "" {
		job.InputFileURI = fmt.Sprintf("files/%s", batchID.String())
	}

	slogger.Info(ctx, "Batch embedding job created successfully with custom requests", slogger.Fields{
		"job_id":   job.JobID,
		"state":    job.State,
		"file_uri": job.InputFileURI,
	})

	return job, nil
}

// CreateBatchEmbeddingJobWithFile creates a batch job using a pre-uploaded file URI.
// This method is used when resuming batch submission after the file was already uploaded.
func (c *BatchEmbeddingClient) CreateBatchEmbeddingJobWithFile(
	ctx context.Context,
	requests []*outbound.BatchEmbeddingRequest,
	options outbound.EmbeddingOptions,
	batchID uuid.UUID,
	fileURI string,
) (*outbound.BatchEmbeddingJob, error) {
	slogger.Info(ctx, "Creating batch embedding job with existing file", slogger.Fields{
		"request_count": len(requests),
		"file_uri":      fileURI,
		"batch_id":      batchID.String(),
	})

	// Validate input
	if len(requests) == 0 {
		return nil, &outbound.EmbeddingError{
			Code:      "empty_requests",
			Type:      "validation",
			Message:   "requests array cannot be empty",
			Retryable: false,
		}
	}

	if fileURI == "" {
		return nil, &outbound.EmbeddingError{
			Code:      "empty_file_uri",
			Type:      "validation",
			Message:   "file URI cannot be empty",
			Retryable: false,
		}
	}

	// Get the cached GenAI client
	genaiClient := c.getGenaiClient()

	// Determine model to use
	model := options.Model
	if model == "" {
		model = c.config.Model
	}

	// Create batch job source using the existing file URI
	source := &genai.EmbeddingsBatchJobSource{
		FileName: fileURI,
	}

	// Create the batch embedding job
	batchJob, err := genaiClient.Batches.CreateEmbeddings(ctx, &model, source, nil)
	if err != nil {
		embeddingErr := c.convertSDKError(err)
		slogger.Error(ctx, "Failed to create batch embedding job with existing file", slogger.Fields{
			"error":    embeddingErr.Error(),
			"file_uri": fileURI,
		})
		return nil, embeddingErr
	}

	// Convert SDK BatchJob to our domain type
	job := c.convertBatchJobToDomain(batchJob, options, len(requests))

	slogger.Info(ctx, "Batch embedding job created successfully using existing file", slogger.Fields{
		"job_id":   job.JobID,
		"state":    job.State,
		"file_uri": fileURI,
	})

	return job, nil
}

// uploadBatchFile uploads a file to Google Files API.
// If the file already exists (ALREADY_EXISTS error), it retrieves and returns the existing file.
// This provides idempotent upload behavior for retry scenarios.
func (c *BatchEmbeddingClient) uploadBatchFile(
	ctx context.Context,
	inputFilePath string,
	inputFileName string,
	batchID uuid.UUID,
) (fileURI string, err error) {
	genaiClient := c.getGenaiClient()

	// Use UUID for system tracking (file name in Google's system)
	fileName := fmt.Sprintf("files/%s", batchID.String())

	// Attempt to upload the file
	uploadedFile, err := genaiClient.Files.UploadFromPath(ctx, inputFilePath, &genai.UploadFileConfig{
		MIMEType:    "application/jsonl",
		Name:        fileName,
		DisplayName: inputFileName, // Human-readable timestamp
	})
	if err != nil {
		// Check if file already exists (ALREADY_EXISTS / 409 error)
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "already_exists") || strings.Contains(errStr, "already exists") {
			slogger.Info(ctx, "File already exists, retrieving existing file", slogger.Fields{
				"file_name": fileName,
			})

			// Fallback: Try to get the existing file
			existingFile, getErr := genaiClient.Files.Get(ctx, fileName, nil)
			if getErr != nil {
				embeddingErr := c.convertSDKError(getErr)
				slogger.Error(ctx, "Failed to retrieve existing file after ALREADY_EXISTS", slogger.Fields{
					"file_name": fileName,
					"error":     embeddingErr.Error(),
				})
				return "", fmt.Errorf("file exists but failed to retrieve: %w", embeddingErr)
			}

			slogger.Info(ctx, "Successfully retrieved existing file", slogger.Fields{
				"file_name":    existingFile.Name,
				"display_name": existingFile.DisplayName,
				"size_bytes":   existingFile.SizeBytes,
			})

			return existingFile.Name, nil
		}

		// Not an ALREADY_EXISTS error, return original error
		embeddingErr := c.convertSDKError(err)
		slogger.Error(ctx, "Failed to upload batch input file", slogger.Fields{
			"file":  inputFilePath,
			"error": embeddingErr.Error(),
		})
		return "", fmt.Errorf("failed to upload batch input file: %w", embeddingErr)
	}

	slogger.Info(ctx, "Batch input file uploaded successfully", slogger.Fields{
		"file_name":    uploadedFile.Name,
		"display_name": uploadedFile.DisplayName,
		"size_bytes":   uploadedFile.SizeBytes,
	})

	return uploadedFile.Name, nil
}

// uploadAndCreateBatchJob uploads the input file (if not already uploaded) and creates a batch embedding job.
// If existingFileURI is provided, it skips the upload step and uses the existing file.
func (c *BatchEmbeddingClient) uploadAndCreateBatchJob(
	ctx context.Context,
	inputFilePath string,
	inputFileName string,
	options outbound.EmbeddingOptions,
	totalCount int,
	batchID uuid.UUID,
	existingFileURI *string,
) (*outbound.BatchEmbeddingJob, error) {
	// Get the cached GenAI client
	genaiClient := c.getGenaiClient()

	var fileURI string
	var err error

	// Check if we already have an uploaded file URI
	if existingFileURI != nil && *existingFileURI != "" {
		slogger.Info(ctx, "Using existing uploaded file", slogger.Fields{
			"file_uri": *existingFileURI,
		})
		fileURI = *existingFileURI
	} else {
		// Upload the file (or retrieve if already exists)
		fileURI, err = c.uploadBatchFile(ctx, inputFilePath, inputFileName, batchID)
		if err != nil {
			return nil, err
		}
	}

	// Determine model to use
	model := options.Model
	if model == "" {
		model = c.config.Model
	}

	// Create batch job source using the uploaded file's resource name
	// Use EmbeddingsBatchJobSource specifically for embedding operations
	source := &genai.EmbeddingsBatchJobSource{
		FileName: fileURI, // File resource name (e.g., "files/abc123")
	}

	// Create the batch embedding job using the GenAI SDK's CreateEmbeddings method
	// This method is specifically for embedding batch operations (not text generation)
	// It internally calls the correct embedding batch endpoint
	batchJob, err := genaiClient.Batches.CreateEmbeddings(ctx, &model, source, nil)
	if err != nil {
		embeddingErr := c.convertSDKError(err)
		slogger.Error(ctx, "Failed to create batch embedding job", slogger.Fields{
			"error": embeddingErr.Error(),
		})
		return nil, embeddingErr
	}

	// Convert SDK BatchJob to our domain type
	job := c.convertBatchJobToDomain(batchJob, options, totalCount)

	// DEBUG: Log the COMPLETE batch job structure at creation time
	// This helps us compare what the API returns at creation vs. retrieval
	batchJobJSON, _ := json.MarshalIndent(batchJob, "", "  ")
	slogger.Info(ctx, "=== DEBUG: COMPLETE BatchJob struct at CREATION (JSON) ===", slogger.Fields{
		"batch_job_json": string(batchJobJSON),
	})

	// Also log our converted domain object to see the transformation
	domainJobJSON, _ := json.MarshalIndent(job, "", "  ")
	slogger.Info(ctx, "=== DEBUG: Converted domain BatchEmbeddingJob at CREATION ===", slogger.Fields{
		"domain_job_json": string(domainJobJSON),
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

	// Get the raw batch job from Google's API
	genaiClient := c.getGenaiClient()
	batchJob, err := genaiClient.Batches.Get(ctx, jobID, nil)
	if err != nil {
		embeddingErr := c.convertSDKError(err)
		return nil, fmt.Errorf("failed to get batch job status: %w", embeddingErr)
	}

	// Check if job is completed
	if batchJob.State != genai.JobStateSucceeded {
		return nil, &outbound.EmbeddingError{
			Code:      "job_not_completed",
			Type:      "validation",
			Message:   fmt.Sprintf("job is not completed (current state: %s)", batchJob.State),
			Retryable: true,
		}
	}

	// DEBUG: Log the COMPLETE batch job structure to see everything
	// Marshal the entire BatchJob to JSON to see all fields
	batchJobJSON, _ := json.MarshalIndent(batchJob, "", "  ")
	slogger.Info(ctx, "=== DEBUG: COMPLETE BatchJob struct (JSON) ===", slogger.Fields{
		"batch_job_json": string(batchJobJSON),
	})

	// Also log the Dest structure specifically with %+v to see field names
	if batchJob.Dest != nil {
		destJSON, _ := json.MarshalIndent(batchJob.Dest, "", "  ")
		slogger.Info(ctx, "=== DEBUG: COMPLETE Dest struct (JSON) ===", slogger.Fields{
			"dest_json": string(destJSON),
		})

		// Log individual interesting fields
		slogger.Info(ctx, "=== DEBUG: Dest field details ===", slogger.Fields{
			"format":                       batchJob.Dest.Format,
			"gcs_uri":                      batchJob.Dest.GCSURI,
			"bigquery_uri":                 batchJob.Dest.BigqueryURI,
			"file_name":                    batchJob.Dest.FileName,
			"inline_responses_count":       len(batchJob.Dest.InlinedResponses),
			"inline_embed_responses_count": len(batchJob.Dest.InlinedEmbedContentResponses),
		})
	}

	// Strategy 1: Check for inline embedding responses first (preferred method)
	// This avoids the file download issues entirely
	if batchJob.Dest != nil && len(batchJob.Dest.InlinedEmbedContentResponses) > 0 {
		slogger.Info(ctx, "Batch results available inline, parsing embedded responses", slogger.Fields{
			"job_id":         jobID,
			"response_count": len(batchJob.Dest.InlinedEmbedContentResponses),
		})

		results := make([]*outbound.EmbeddingResult, 0, len(batchJob.Dest.InlinedEmbedContentResponses))
		for i, inlinedResponse := range batchJob.Dest.InlinedEmbedContentResponses {
			if inlinedResponse.Response != nil &&
				inlinedResponse.Response.Embedding != nil &&
				len(inlinedResponse.Response.Embedding.Values) > 0 {
				// Convert float32 to float64
				vector := make([]float64, len(inlinedResponse.Response.Embedding.Values))
				for j, v := range inlinedResponse.Response.Embedding.Values {
					vector[j] = float64(v)
				}

				results = append(results, &outbound.EmbeddingResult{
					RequestID:  fmt.Sprintf("req_%d", i),
					Vector:     vector,
					Dimensions: len(vector),
					Model:      c.config.Model,
				})
			}
		}

		slogger.Info(ctx, "Batch job results retrieved from inline responses", slogger.Fields{
			"job_id":       jobID,
			"result_count": len(results),
		})

		return results, nil
	}

	// Strategy 2: Fall back to file download if inline responses not available
	if batchJob.Dest != nil && (batchJob.Dest.FileName != "" || batchJob.Dest.GCSURI != "") {
		outputFileURI := batchJob.Dest.FileName
		if outputFileURI == "" {
			outputFileURI = batchJob.Dest.GCSURI
		}

		slogger.Info(ctx, "No inline responses, attempting file download", slogger.Fields{
			"job_id":          jobID,
			"output_file_uri": outputFileURI,
		})

		// Download and parse the output file
		results, err := c.downloadAndParseResults(ctx, outputFileURI)
		if err != nil {
			return nil, fmt.Errorf("failed to download and parse results: %w", err)
		}

		slogger.Info(ctx, "Batch job results retrieved from file", slogger.Fields{
			"job_id":       jobID,
			"result_count": len(results),
		})

		return results, nil
	}

	return nil, &outbound.EmbeddingError{
		Code:      "no_output_available",
		Type:      "response",
		Message:   "job completed but no output available (neither inline responses nor file reference)",
		Retryable: false,
	}
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
// Returns the full filepath (for local operations) and just the filename (for API operations).
func (c *BatchEmbeddingClient) writeRequestsToFile(
	ctx context.Context,
	requests []*outbound.BatchEmbeddingRequest,
	options outbound.EmbeddingOptions,
	batchID uuid.UUID,
) (string, string, error) {
	// Create input file with UUID-based name for system tracking
	timestamp := time.Now().Format("20060102_150405")
	displayName := fmt.Sprintf("batch_input_%s.jsonl", timestamp) // Human-readable display name
	filename := fmt.Sprintf("%s.jsonl", batchID.String())         // UUID-based file name
	fullPath := filepath.Join(c.inputDir, filename)

	file, err := os.Create(fullPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create input file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer func() {
		_ = writer.Flush()
	}()

	// Note: taskType is not used in batch embeddings API format

	// Write each request as a JSONL line
	for _, req := range requests {
		// Create the embedding request structure using Google Gemini Batch API format
		// Format: {"key": "id", "request": {"content": {"parts": [{"text": "..."}]}, "output_dimensionality": 768}}
		// Note: "content" is singular and an object (not "contents" array)
		// Note: "output_dimensionality" is at request level (not nested in config)
		requestContent := map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": req.Text},
			},
		}

		// Create the request payload
		requestPayload := map[string]interface{}{
			"content": requestContent, // Singular "content" as object (not array)
		}

		// Add output_dimensionality directly to request (not nested in config)
		if c.config.Dimensions > 0 {
			requestPayload["output_dimensionality"] = c.config.Dimensions
		}

		// Create the batch request in Google Gemini format
		embeddingReq := map[string]interface{}{
			"key":     req.RequestID,
			"request": requestPayload,
		}

		// Note: task_type is not included as it may not be supported in batch embeddings API
		// If needed in the future, it should be at request level, not nested in config

		// Marshal to JSON and write
		jsonData, err := json.Marshal(embeddingReq)
		if err != nil {
			return "", "", fmt.Errorf("failed to marshal request: %w", err)
		}

		if _, err := writer.WriteString(string(jsonData) + "\n"); err != nil {
			return "", "", fmt.Errorf("failed to write request: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return "", "", fmt.Errorf("failed to flush writer: %w", err)
	}

	slogger.Info(ctx, "Batch requests written to file", slogger.Fields{
		"file":          fullPath,
		"request_count": len(requests),
	})

	return fullPath, displayName, nil
}

// downloadAndParseResults downloads and parses batch job results.
func (c *BatchEmbeddingClient) downloadAndParseResults(
	ctx context.Context,
	outputFileURI string,
) ([]*outbound.EmbeddingResult, error) {
	var localFilePath string

	// Check if this is a file reference (not a local path)
	// Batch job outputs are special references that need direct download
	if !filepath.IsAbs(outputFileURI) && !strings.Contains(outputFileURI, "/tmp/") {
		slogger.Info(ctx, "Output is a batch file reference, attempting direct download", slogger.Fields{
			"file_ref": outputFileURI,
		})

		downloadSuccess := false

		// For batch output files with references like "files/batch-xxx"
		// These are NOT accessible via the standard Files API due to:
		// 1. File ID length exceeds 40-character limit
		// 2. They are batch-specific resources that don't exist as regular files
		//
		// According to Google's Batch API documentation, batch output files
		// should either be:
		// - Written to a GCS bucket (if configured)
		// - Available as inline results in the batch job response
		// - Downloaded using a batch-specific method
		//
		// Since none of these are currently implemented, we'll return a clear error
		//if strings.HasPrefix(outputFileURI, "files/batch-") {
		//	return nil, fmt.Errorf(
		//		"batch output file '%s' cannot be downloaded: "+
		//			"Google's Batch API returns file references (files/batch-xxx) that exceed the Files API 40-character limit "+
		//			"and are not accessible as regular files. "+
		//			"This appears to be an API design issue with Google Gemini Batches API. "+
		//			"Possible solutions: "+
		//			"(1) Configure GCS bucket for batch outputs in job creation, "+
		//			"(2) Check if results are embedded inline in BatchJob response, "+
		//			"(3) Use a different output configuration when creating the batch job. "+
		//			"See https://ai.google.dev/gemini-api/docs/batch-api for more details.",
		//		outputFileURI,
		//	)
		//}
		// Validate file reference format before proceeding
		if !strings.HasPrefix(outputFileURI, "files/") {
			return nil, fmt.Errorf("invalid file reference format: '%s' - must start with 'files/'", outputFileURI)
		}

		// Manually construct File object to bypass 40-character file ID limitation
		// This replaces the need for files.Get() call which fails with long batch file IDs
		var sizeBytes int64 = 0 // Will be determined after download if needed
		constructedFile := &genai.File{
			Name:        outputFileURI,
			DisplayName: filepath.Base(outputFileURI),
			SizeBytes:   &sizeBytes,
			DownloadURI: outputFileURI,
		}

		slogger.Info(ctx, "Attempting to download batch output file using constructed File object", slogger.Fields{
			"file_ref":     outputFileURI,
			"display_name": constructedFile.DisplayName,
		})
		downloadUri := genai.NewDownloadURIFromFile(constructedFile)
		// Attempt download with the constructed file object
		fileBytes, err := c.getGenaiClient().Files.Download(ctx, downloadUri, nil)
		if err != nil {
			slogger.Error(ctx, "Failed to download batch output file with constructed reference", slogger.Fields{
				"file_ref": outputFileURI,
				"error":    err.Error(),
			})
		} else {
			downloadSuccess = true
			localFilePath = filepath.Join(c.outputDir, filepath.Base(outputFileURI))
			if err := os.WriteFile(localFilePath, fileBytes, 0o644); err != nil {
				return nil, fmt.Errorf("failed to write downloaded file: %w", err)
			}
			slogger.Info(ctx, "Successfully downloaded batch output file", slogger.Fields{
				"file_ref":   outputFileURI,
				"local_path": localFilePath,
				"file_size":  len(fileBytes),
			})
		}

		// If download succeeded, update localFilePath for parsing
		if downloadSuccess && localFilePath == "" {
			filename := filepath.Base(outputFileURI)
			localFilePath = filepath.Join(c.outputDir, filename)
		}
	}

	// If it wasn't a file reference or download failed, try local file path
	if localFilePath == "" {
		// Use original logic for local file paths
		// outputFileURI from Gemini API might be a relative path like "files/batch-xxx"
		localFilePath = filepath.Join(c.outputDir, outputFileURI)
	}

	file, err := os.Open(localFilePath)
	if err != nil {
		// Provide detailed error message based on the situation
		if strings.Contains(outputFileURI, "files/") {
			return nil, fmt.Errorf(
				"failed to download or access batch results from Google API (file reference: '%s'): %w. "+
					"This error occurs when the Google Batch API returns file IDs that are incompatible with the Files API. "+
					"Possible causes: (1) API returned file IDs exceeding 40-character limit, (2) Permission/authentication issues, "+
					"(3) File not yet available. Check logs above for specific API errors. Original error",
				outputFileURI,
				err,
			)
		}
		return nil, fmt.Errorf(
			"failed to open output file '%s': %w. This typically means the batch job creation failed earlier (check logs for quota errors) or the output file hasn't been created yet",
			localFilePath,
			err,
		)
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
// Google Gemini Batch API returns responses in the order they were sent.
// The response format for embeddings should be:
// {"key": "request-id", "response": {"embedding": {"values": [...]}}}
// OR it may return GenerateContentResponse format (needs verification with actual API).
func (c *BatchEmbeddingClient) parseEmbeddingResponse(
	response map[string]interface{},
) (*outbound.EmbeddingResult, error) {
	// Try Google Gemini format first (with "key" field)
	requestID, hasKey := response["key"].(string)
	if !hasKey {
		// Fallback to OpenAI-style format for backwards compatibility
		requestID, _ = response["custom_id"].(string)
	}

	// Extract response body - could be either "response" or direct embedding
	var respBody map[string]interface{}
	if resp, ok := response["response"].(map[string]interface{}); ok {
		respBody = resp
	} else {
		// Response might be at the top level
		respBody = response
	}

	// Try to extract embedding data
	// Format 1: {"response": {"embedding": {"values": [...]}}}
	var embeddingData map[string]interface{}
	if emb, ok := respBody["embedding"].(map[string]interface{}); ok {
		embeddingData = emb
	} else if body, ok := respBody["body"].(map[string]interface{}); ok {
		// Format 2: {"response": {"body": {"embedding": {"values": [...]}}}}
		if emb, ok := body["embedding"].(map[string]interface{}); ok {
			embeddingData = emb
		}
	}

	if embeddingData == nil {
		return nil, errors.New("invalid response structure: missing embedding data")
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

		// Note: batchJob.Dest contains either GCSURI or FileName for the output
		// GCSURI format: "gs://bucket/path/file.jsonl"
		// FileName format: "files/batch-xxx" (might exceed 40 char limit)
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
