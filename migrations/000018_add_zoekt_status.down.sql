-- Rollback: Remove Zoekt and embedding index status fields from repositories table

-- Set search path
SET search_path TO codechunking, public;

-- Drop constraints
ALTER TABLE repositories DROP CONSTRAINT IF EXISTS chk_zoekt_index_status;
ALTER TABLE repositories DROP CONSTRAINT IF EXISTS chk_embedding_index_status;

-- Drop indexes
DROP INDEX IF EXISTS idx_repositories_zoekt_status;
DROP INDEX IF EXISTS idx_repositories_embedding_status;

-- Drop columns
ALTER TABLE repositories DROP COLUMN IF EXISTS zoekt_commit_hash;
ALTER TABLE repositories DROP COLUMN IF EXISTS zoekt_shard_count;
ALTER TABLE repositories DROP COLUMN IF EXISTS zoekt_last_indexed_at;
ALTER TABLE repositories DROP COLUMN IF EXISTS embedding_index_status;
ALTER TABLE repositories DROP COLUMN IF EXISTS zoekt_index_status;

-- Log completion for audit purposes
DO $$
BEGIN
    RAISE NOTICE 'Zoekt and embedding index status columns removed from repositories table';
END $$;