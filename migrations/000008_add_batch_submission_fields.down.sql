-- Rollback: Remove batch submission tracking fields

-- Set search path to codechunking schema
SET search_path TO codechunking, public;

-- Drop index
DROP INDEX IF EXISTS codechunking.idx_batch_job_progress_pending_submission;

-- Remove added columns
ALTER TABLE codechunking.batch_job_progress
DROP COLUMN IF EXISTS batch_request_data;

ALTER TABLE codechunking.batch_job_progress
DROP COLUMN IF EXISTS submission_attempts;

ALTER TABLE codechunking.batch_job_progress
DROP COLUMN IF EXISTS next_submission_at;

-- Restore original status constraint (without 'pending_submission')
ALTER TABLE codechunking.batch_job_progress
DROP CONSTRAINT IF EXISTS chk_status_valid;

ALTER TABLE codechunking.batch_job_progress
ADD CONSTRAINT chk_status_valid CHECK (
  status IN ('pending', 'processing', 'completed', 'failed', 'retry_scheduled')
);
