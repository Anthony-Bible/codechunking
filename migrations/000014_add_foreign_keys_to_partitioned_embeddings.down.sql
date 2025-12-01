-- Rollback: Remove foreign keys from partitioned embeddings table
-- This reverses the foreign key constraints added in the up migration

SET search_path TO codechunking, public;

-- Drop the foreign key constraints from the parent table
ALTER TABLE embeddings_partitioned
DROP CONSTRAINT IF EXISTS fk_embeddings_partitioned_chunk_id;

ALTER TABLE embeddings_partitioned
DROP CONSTRAINT IF EXISTS fk_embeddings_partitioned_repository_id;

-- Note: Orphaned data cleanup performed in up migration cannot be reversed
-- Data integrity should be maintained through application logic until this constraint is re-added
