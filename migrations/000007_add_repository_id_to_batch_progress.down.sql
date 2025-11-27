-- Rollback migration to remove repository_id column from batch_job_progress table

-- Set search path to codechunking schema
SET search_path TO codechunking, public;

-- Drop the foreign key constraint first
ALTER TABLE codechunking.batch_job_progress
DROP CONSTRAINT IF EXISTS fk_batch_job_progress_repository;

-- Drop the index
DROP INDEX IF EXISTS codechunking.idx_batch_job_progress_repository_id;

-- Remove the repository_id column
ALTER TABLE codechunking.batch_job_progress
DROP COLUMN IF EXISTS repository_id;