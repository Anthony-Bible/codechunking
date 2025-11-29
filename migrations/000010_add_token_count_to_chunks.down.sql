-- Rollback: Remove token_count columns from code_chunks table

DROP INDEX IF EXISTS codechunking.idx_code_chunks_repo_token_count;
DROP INDEX IF EXISTS codechunking.idx_code_chunks_token_count;

ALTER TABLE codechunking.code_chunks
DROP COLUMN IF EXISTS token_counted_at;

ALTER TABLE codechunking.code_chunks
DROP COLUMN IF EXISTS token_count;
