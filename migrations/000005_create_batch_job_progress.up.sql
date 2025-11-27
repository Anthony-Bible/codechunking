-- Migration to add batch job progress tracking for chunked batch embedding processing
-- This enables resume capability and quota-aware batch processing

-- Set search path to codechunking schema
SET search_path TO codechunking, public;

-- Create batch_job_progress table for tracking batch processing state
CREATE TABLE IF NOT EXISTS batch_job_progress (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  indexing_job_id UUID NOT NULL REFERENCES indexing_jobs(id) ON DELETE CASCADE,
  batch_number INT NOT NULL,
  total_batches INT NOT NULL,
  chunks_processed INT NOT NULL DEFAULT 0,
  status VARCHAR(50) NOT NULL DEFAULT 'pending',
  retry_count INT NOT NULL DEFAULT 0,
  next_retry_at TIMESTAMP WITH TIME ZONE,
  error_message TEXT,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

  -- Ensure batch numbers are unique per job
  CONSTRAINT uq_batch_job_progress_job_batch UNIQUE (indexing_job_id, batch_number),

  -- Ensure batch number is within valid range
  CONSTRAINT chk_batch_number_positive CHECK (batch_number > 0),
  CONSTRAINT chk_batch_number_within_total CHECK (batch_number <= total_batches),

  -- Ensure chunks_processed is non-negative
  CONSTRAINT chk_chunks_processed_non_negative CHECK (chunks_processed >= 0),

  -- Ensure retry_count is non-negative
  CONSTRAINT chk_retry_count_non_negative CHECK (retry_count >= 0),

  -- Ensure status is valid
  CONSTRAINT chk_status_valid CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'retry_scheduled'))
);

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_batch_job_progress_indexing_job_id
  ON batch_job_progress(indexing_job_id);

CREATE INDEX IF NOT EXISTS idx_batch_job_progress_status
  ON batch_job_progress(status);

CREATE INDEX IF NOT EXISTS idx_batch_job_progress_next_retry_at
  ON batch_job_progress(next_retry_at)
  WHERE status = 'retry_scheduled' AND next_retry_at IS NOT NULL;

-- Create composite index for finding retry-ready batches
CREATE INDEX IF NOT EXISTS idx_batch_job_progress_retry_ready
  ON batch_job_progress(status, next_retry_at)
  WHERE status = 'retry_scheduled';

-- Add comments to document the schema
COMMENT ON TABLE batch_job_progress IS 'Tracks progress of chunked batch embedding jobs for resume capability and quota management';
COMMENT ON COLUMN batch_job_progress.indexing_job_id IS 'Reference to the parent indexing job';
COMMENT ON COLUMN batch_job_progress.batch_number IS 'Sequence number of this batch (1-indexed)';
COMMENT ON COLUMN batch_job_progress.total_batches IS 'Total number of batches for this job';
COMMENT ON COLUMN batch_job_progress.chunks_processed IS 'Number of chunks successfully processed in this batch';
COMMENT ON COLUMN batch_job_progress.status IS 'Current status: pending, processing, completed, failed, retry_scheduled';
COMMENT ON COLUMN batch_job_progress.retry_count IS 'Number of retry attempts for this batch';
COMMENT ON COLUMN batch_job_progress.next_retry_at IS 'Timestamp when this batch should be retried (for retry_scheduled status)';
COMMENT ON COLUMN batch_job_progress.error_message IS 'Error message from last failed attempt';
