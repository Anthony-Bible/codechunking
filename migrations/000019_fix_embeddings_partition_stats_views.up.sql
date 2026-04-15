-- Fix pg_stat_user_tables column references in partition stats function and view.
-- The original migration (000003) used `tablename` from pg_stat_user_tables, but
-- that view exposes the column as `relname`. This migration recreates the function
-- and view with the correct column name so they return results on all PostgreSQL versions.

SET search_path TO codechunking, public;

-- Recreate the partition-stats function with the correct column name.
CREATE OR REPLACE FUNCTION get_embeddings_partition_stats()
RETURNS TABLE(
    partition_name TEXT,
    row_count BIGINT,
    size_pretty TEXT,
    index_size_pretty TEXT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        schemaname||'.'||relname AS partition_name,
        n_tup_ins - n_tup_del   AS row_count,
        pg_size_pretty(pg_total_relation_size(schemaname||'.'||relname)) AS size_pretty,
        pg_size_pretty(pg_indexes_size(schemaname||'.'||relname))        AS index_size_pretty
    FROM pg_stat_user_tables
    WHERE relname LIKE 'embeddings_partitioned_%'
    ORDER BY relname;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_embeddings_partition_stats() IS
'Monitoring function to track partition sizes and row counts for capacity planning.';

-- Recreate the monitoring view with the correct column name.
CREATE OR REPLACE VIEW embeddings_partition_monitoring AS
SELECT
    schemaname||'.'||relname                                             AS partition_name,
    n_tup_ins - n_tup_del                                               AS row_count,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||relname))    AS size_pretty,
    pg_size_pretty(pg_indexes_size(schemaname||'.'||relname))           AS index_size_pretty,
    seq_scan,
    seq_tup_read,
    idx_scan,
    idx_tup_fetch,
    n_tup_ins,
    n_tup_upd,
    n_tup_del
FROM pg_stat_user_tables
WHERE relname LIKE 'embeddings_partitioned_%'
ORDER BY relname;

COMMENT ON VIEW embeddings_partition_monitoring IS
'Performance monitoring view for embeddings partitions showing size, usage, and access patterns.';
