-- Rollback: Remove metadata columns from embeddings_partitioned
-- This migration safely removes the denormalized metadata columns

BEGIN;

-- Drop indexes first (must be done before dropping columns)
DROP INDEX IF EXISTS codechunking.idx_embeddings_partitioned_repo_lang_type;
DROP INDEX IF EXISTS codechunking.idx_embeddings_partitioned_lang_type;
DROP INDEX IF EXISTS codechunking.idx_embeddings_partitioned_repo_type;
DROP INDEX IF EXISTS codechunking.idx_embeddings_partitioned_repo_lang;
DROP INDEX IF EXISTS codechunking.idx_embeddings_partitioned_file_path_trgm;
DROP INDEX IF EXISTS codechunking.idx_embeddings_partitioned_chunk_type;
DROP INDEX IF EXISTS codechunking.idx_embeddings_partitioned_language;

-- Drop metadata columns
-- Note: Data in code_chunks table is preserved (source of truth)
ALTER TABLE codechunking.embeddings_partitioned
  DROP COLUMN IF EXISTS file_path,
  DROP COLUMN IF EXISTS chunk_type,
  DROP COLUMN IF EXISTS language;

COMMIT;
