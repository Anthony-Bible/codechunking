-- Add repository_id column to batch_job_progress table to track which repository each batch belongs to
-- This fixes the "Missing repository_id for batch transactional chunk save" error

-- Set search path to codechunking schema
SET search_path TO codechunking, public;

-- Add repository_id column as nullable initially to handle existing rows
ALTER TABLE codechunking.batch_job_progress
ADD COLUMN repository_id UUID;

-- For existing rows, set a NULL value initially (they will be cleaned up or won't be processed)
-- This ensures the migration works even with existing data
UPDATE codechunking.batch_job_progress
SET repository_id = NULL
WHERE repository_id IS NULL;

-- Create index for efficient lookup by repository (excluding NULL values)
CREATE INDEX IF NOT EXISTS idx_batch_job_progress_repository_id
ON codechunking.batch_job_progress(repository_id)
WHERE repository_id IS NOT NULL;

-- Note: We defer the NOT NULL constraint until all code changes are in place
-- This migration allows for a two-phase deployment approach