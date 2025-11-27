-- Add batch submission tracking fields to batch_job_progress table
-- These fields support batch embedding submission retry logic and rate limiting

-- Set search path to codechunking schema
SET search_path TO codechunking, public;

-- Add batch_request_data column to store serialized batch request
-- Using bytea to preserve exact JSON formatting (not JSONB which normalizes)
ALTER TABLE codechunking.batch_job_progress
ADD COLUMN batch_request_data bytea;

-- Add submission_attempts column to track retry attempts
ALTER TABLE codechunking.batch_job_progress
ADD COLUMN submission_attempts INT NOT NULL DEFAULT 0;

-- Add next_submission_at column for rate limiting and retry scheduling
ALTER TABLE codechunking.batch_job_progress
ADD COLUMN next_submission_at TIMESTAMP WITH TIME ZONE;

-- Update the status constraint to include 'pending_submission'
ALTER TABLE codechunking.batch_job_progress
DROP CONSTRAINT IF EXISTS chk_status_valid;

ALTER TABLE codechunking.batch_job_progress
ADD CONSTRAINT chk_status_valid CHECK (
  status IN ('pending', 'pending_submission', 'processing', 'completed', 'failed', 'retry_scheduled')
);

-- Create index for efficient pending submission queries
CREATE INDEX IF NOT EXISTS idx_batch_job_progress_pending_submission
ON codechunking.batch_job_progress(status, next_submission_at, created_at)
WHERE status = 'pending_submission';

-- Add comments to document the new fields
COMMENT ON COLUMN batch_job_progress.batch_request_data IS 'Serialized batch request data for Gemini API submission';
COMMENT ON COLUMN batch_job_progress.submission_attempts IS 'Number of submission attempts (for rate limiting and retry logic)';
COMMENT ON COLUMN batch_job_progress.next_submission_at IS 'Timestamp when next submission should be attempted (NULL means ready now)';
