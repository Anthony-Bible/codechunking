-- Migration: Backfill metadata columns in embeddings_partitioned
-- Purpose: Populate language, chunk_type, and file_path for existing embeddings
-- Note: This is a data migration and may take time for large datasets
--
-- ⚠️  IMPORTANT: For large production datasets, consider running during a maintenance window
-- or using the batched approach described at the bottom of this file.

BEGIN;

-- Set isolation level to prevent anomalies during backfill
SET TRANSACTION ISOLATION LEVEL READ COMMITTED;

-- Backfill metadata from code_chunks table
-- Uses WHERE clause to only update rows that haven't been backfilled yet
-- This makes the migration idempotent and resumable
UPDATE codechunking.embeddings_partitioned e
SET
  language = c.language,
  chunk_type = c.chunk_type,
  file_path = c.file_path
FROM codechunking.code_chunks c
WHERE e.chunk_id = c.id
  AND e.language IS NULL  -- Only update rows that haven't been backfilled
  AND c.deleted_at IS NULL;  -- Don't backfill from deleted chunks

-- Update statistics after bulk update for better query planning
ANALYZE codechunking.embeddings_partitioned;

COMMIT;

-- Note: For very large datasets (>10M rows), consider running this in batches:
-- UPDATE codechunking.embeddings_partitioned e
-- SET language = c.language, chunk_type = c.chunk_type, file_path = c.file_path
-- FROM codechunking.code_chunks c
-- WHERE e.chunk_id = c.id AND e.language IS NULL AND c.deleted_at IS NULL
-- AND e.repository_id = '<specific-repo-id>'  -- Process one repository at a time
