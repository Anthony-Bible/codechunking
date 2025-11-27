-- Rollback migration for batch job progress tracking

-- Set search path to codechunking schema
SET search_path TO codechunking, public;

-- Drop the indexes
DROP INDEX IF EXISTS idx_batch_job_progress_retry_ready;
DROP INDEX IF EXISTS idx_batch_job_progress_next_retry_at;
DROP INDEX IF EXISTS idx_batch_job_progress_status;
DROP INDEX IF EXISTS idx_batch_job_progress_indexing_job_id;

-- Drop the table (CASCADE will handle foreign key references)
DROP TABLE IF EXISTS batch_job_progress CASCADE;
