-- Rollback: Clear metadata from embeddings_partitioned
-- Note: This doesn't drop the columns (that's done in 000005 down migration)
-- It just clears the data to return to pre-backfill state
--
-- ⚠️  WARNING: DATA LOSS ⚠️
-- This rollback will clear ALL metadata in embeddings_partitioned, including any metadata
-- populated after the initial backfill migration. Only run this if you are certain you want
-- to lose all denormalized metadata. The metadata is derivable from code_chunks table,
-- but re-backfilling may take significant time on large datasets.

BEGIN;

-- Clear metadata columns (set to NULL)
-- Only do this if rolling back the backfill, not the column addition
UPDATE codechunking.embeddings_partitioned
SET
  language = NULL,
  chunk_type = NULL,
  file_path = NULL
WHERE language IS NOT NULL
   OR chunk_type IS NOT NULL
   OR file_path IS NOT NULL;

COMMIT;
