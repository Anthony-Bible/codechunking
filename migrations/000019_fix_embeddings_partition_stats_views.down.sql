-- Rollback: restore the original function and view from migration 000003.
-- NOTE: The original migration used `tablename` instead of `relname`, which
-- does not return any rows on standard PostgreSQL installs. Rolling back here
-- restores that (non-functional) original state.

SET search_path TO codechunking, public;

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
        schemaname||'.'||tablename AS partition_name,
        n_tup_ins - n_tup_del     AS row_count,
        pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size_pretty,
        pg_size_pretty(pg_indexes_size(schemaname||'.'||tablename))        AS index_size_pretty
    FROM pg_stat_user_tables
    WHERE tablename LIKE 'embeddings_partitioned_%'
    ORDER BY tablename;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE VIEW embeddings_partition_monitoring AS
SELECT
    schemaname||'.'||tablename                                             AS partition_name,
    n_tup_ins - n_tup_del                                                 AS row_count,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename))    AS size_pretty,
    pg_size_pretty(pg_indexes_size(schemaname||'.'||tablename))           AS index_size_pretty,
    seq_scan,
    seq_tup_read,
    idx_scan,
    idx_tup_fetch,
    n_tup_ins,
    n_tup_upd,
    n_tup_del
FROM pg_stat_user_tables
WHERE tablename LIKE 'embeddings_partitioned_%'
ORDER BY tablename;
