-- Rollback: Remove DEFERRABLE foreign keys from embeddings_partitioned partitions
-- This reverses the partition-specific constraints added in the up migration

SET search_path TO codechunking, public;

-- Remove DEFERRABLE constraints from all partitions using dynamic SQL
DO $$
DECLARE
    partition_record RECORD;
BEGIN
    FOR partition_record IN
        SELECT tablename
        FROM pg_tables
        WHERE schemaname = 'codechunking'
        AND tablename LIKE 'embeddings_partitioned_%'
    LOOP
        BEGIN
            EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT IF EXISTS fk_%s_chunk_id',
                          'codechunking', partition_record.tablename, partition_record.tablename);
            EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT IF EXISTS fk_%s_repository_id',
                          'codechunking', partition_record.tablename, partition_record.tablename);
            RAISE NOTICE 'Removed DEFERRABLE foreign keys from partition: %', partition_record.tablename;
        EXCEPTION WHEN others THEN
            RAISE NOTICE 'Error removing constraints from %: %', partition_record.tablename, SQLERRM;
        END;
    END LOOP;
END $$;

-- Remove from parent table as well
ALTER TABLE embeddings_partitioned
DROP CONSTRAINT IF EXISTS fk_embeddings_partitioned_chunk_id;

ALTER TABLE embeddings_partitioned
DROP CONSTRAINT IF EXISTS fk_embeddings_partitioned_repository_id;
