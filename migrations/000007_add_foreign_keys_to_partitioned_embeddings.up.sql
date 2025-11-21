-- Add missing foreign keys to partitioned embeddings table
-- PostgreSQL 16 supports foreign keys on partitioned tables, improving data integrity

-- Set search path
SET search_path TO codechunking, public;

-- Clean up orphaned data first (embeddings that reference non-existent chunks or repositories)
-- This ensures data integrity before adding constraints
-- Using NOT EXISTS for better performance on large datasets compared to NOT IN
DELETE FROM embeddings_partitioned ep
WHERE NOT EXISTS (SELECT 1 FROM code_chunks cc WHERE cc.id = ep.chunk_id)
   OR NOT EXISTS (SELECT 1 FROM repositories r WHERE r.id = ep.repository_id);

-- Add chunk_id foreign key (was missing from previous migration)
-- This ensures all embeddings belong to valid code chunks
ALTER TABLE embeddings_partitioned
ADD CONSTRAINT fk_embeddings_partitioned_chunk_id
FOREIGN KEY (chunk_id) REFERENCES code_chunks(id) ON DELETE CASCADE;

-- Add repository_id foreign key (not possible in older PostgreSQL versions)
-- This ensures all embeddings belong to valid repositories
ALTER TABLE embeddings_partitioned
ADD CONSTRAINT fk_embeddings_partitioned_repository_id
FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE;

-- Add comments explaining the constraints
COMMENT ON CONSTRAINT fk_embeddings_partitioned_chunk_id ON embeddings_partitioned IS
'Ensures all partitioned embeddings belong to valid code chunks with cascade delete on chunk removal';

COMMENT ON CONSTRAINT fk_embeddings_partitioned_repository_id ON embeddings_partitioned IS
'Ensures all partitioned embeddings belong to valid repositories with cascade delete on repository removal';

-- Log cleanup results for audit purposes
DO $$
DECLARE
    cleanup_count BIGINT;
BEGIN
    -- This will show 0 since we deleted them, but provides audit trail
    SELECT COUNT(*) INTO cleanup_count
    FROM embeddings_partitioned ep
    LEFT JOIN code_chunks cc ON ep.chunk_id = cc.id
    WHERE cc.id IS NULL;

    RAISE NOTICE 'Orphaned embeddings cleanup completed. Remaining orphaned records: %', cleanup_count;
END $$;