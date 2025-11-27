-- Rollback migration for Gemini batch job ID tracking

-- Set search path to codechunking schema
SET search_path TO codechunking, public;

-- Drop indexes
DROP INDEX IF EXISTS codechunking.idx_batch_job_progress_pending_gemini;
DROP INDEX IF EXISTS codechunking.idx_batch_job_progress_gemini_job_id;

-- Drop column
ALTER TABLE batch_job_progress
DROP COLUMN IF EXISTS gemini_batch_job_id;
