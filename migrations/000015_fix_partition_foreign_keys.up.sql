-- Fix foreign key constraints on partitioned embeddings table partitions
-- Use dynamic SQL to handle all existing partitions automatically

-- Set search path
SET search_path TO codechunking, public;

-- Clean up any conflicting non-DEFERRABLE constraints first
-- This removes inherited constraints that might conflict with partition-specific ones
DO $$
DECLARE
    partition_record RECORD;
BEGIN
    -- Remove conflicting non-DEFERRABLE constraints from all partitions
    FOR partition_record IN
        SELECT tablename
        FROM pg_tables
        WHERE schemaname = 'codechunking'
        AND tablename LIKE 'embeddings_partitioned_%'
    LOOP
        BEGIN
            EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT IF EXISTS fk_embeddings_partitioned_%s_chunk_id',
                          'codechunking', partition_record.tablename, REPLACE(partition_record.tablename, 'embeddings_partitioned_', ''));
            EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT IF EXISTS fk_embeddings_partitioned_%s_repository_id',
                          'codechunking', partition_record.tablename, REPLACE(partition_record.tablename, 'embeddings_partitioned_', ''));
            RAISE NOTICE 'Cleaned up conflicting constraints for: %', partition_record.tablename;
        EXCEPTION WHEN others THEN
            RAISE NOTICE 'Error cleaning constraints for %: %', partition_record.tablename, SQLERRM;
        END;
    END LOOP;

    RAISE NOTICE 'Conflicting foreign key constraints cleaned up';
END $$;

-- Add clean DEFERRABLE constraints to all partitions using dynamic SQL
DO $$
DECLARE
    partition_record RECORD;
    constraint_name text;
BEGIN
    -- Add constraints to each partition dynamically
    FOR partition_record IN
        SELECT tablename
        FROM pg_tables
        WHERE schemaname = 'codechunking'
        AND tablename LIKE 'embeddings_partitioned_%'
        ORDER BY tablename
    LOOP
        BEGIN
            -- Add chunk_id foreign key
            constraint_name := 'fk_' || partition_record.tablename || '_chunk_id';
            EXECUTE format('ALTER TABLE %I.%I ADD CONSTRAINT %I FOREIGN KEY (chunk_id) REFERENCES %I.%I(id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED',
                          'codechunking', partition_record.tablename, constraint_name, 'codechunking', 'code_chunks');

            -- Add repository_id foreign key
            constraint_name := 'fk_' || partition_record.tablename || '_repository_id';
            EXECUTE format('ALTER TABLE %I.%I ADD CONSTRAINT %I FOREIGN KEY (repository_id) REFERENCES %I.%I(id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED',
                          'codechunking', partition_record.tablename, constraint_name, 'codechunking', 'repositories');

            RAISE NOTICE 'Added DEFERRABLE foreign keys to partition: %', partition_record.tablename;
        EXCEPTION WHEN duplicate_object THEN
            RAISE NOTICE 'DEFERRABLE foreign key already exists on partition: %', partition_record.tablename;
        END;
    END LOOP;

    RAISE NOTICE 'DEFERRABLE foreign key constraints added to all embeddings_partitioned partitions';
END $$;

-- Ensure parent table also has the constraints for consistency (DEFERRABLE)
DO $$
BEGIN
    BEGIN
        ALTER TABLE embeddings_partitioned
        ADD CONSTRAINT fk_embeddings_partitioned_chunk_id
        FOREIGN KEY (chunk_id) REFERENCES code_chunks(id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED;
    EXCEPTION WHEN duplicate_object THEN
        RAISE NOTICE 'Parent table chunk_id foreign key already exists';
    END;

    BEGIN
        ALTER TABLE embeddings_partitioned
        ADD CONSTRAINT fk_embeddings_partitioned_repository_id
        FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED;
    EXCEPTION WHEN duplicate_object THEN
        RAISE NOTICE 'Parent table repository_id foreign key already exists';
    END;
END $$;

-- Log completion for audit purposes
DO $$
BEGIN
    RAISE NOTICE 'Foreign key constraints fix completed for partitioned embeddings table';
END $$;