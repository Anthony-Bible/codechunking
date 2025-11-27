-- Migration to add Gemini batch job ID tracking for async batch processing
-- This enables workers to submit batch jobs and poll for results asynchronously

-- Set search path to codechunking schema
SET search_path TO codechunking, public;

-- Add gemini_batch_job_id column to track the Google Gemini API batch job ID
ALTER TABLE batch_job_progress
ADD COLUMN gemini_batch_job_id TEXT;

-- Create index for efficient lookup of batches by Gemini job ID
CREATE INDEX IF NOT EXISTS idx_batch_job_progress_gemini_job_id
ON batch_job_progress(gemini_batch_job_id)
WHERE gemini_batch_job_id IS NOT NULL;

-- Create composite index for finding batches that need polling
-- (batches in 'processing' status with a gemini_batch_job_id)
CREATE INDEX IF NOT EXISTS idx_batch_job_progress_pending_gemini
ON batch_job_progress(status, gemini_batch_job_id)
WHERE status = 'processing' AND gemini_batch_job_id IS NOT NULL;

-- Add comment to document the new column
COMMENT ON COLUMN batch_job_progress.gemini_batch_job_id IS 'Google Gemini API batch job ID for async polling (e.g., projects/123/locations/us-central1/batchPredictionJobs/456)';
