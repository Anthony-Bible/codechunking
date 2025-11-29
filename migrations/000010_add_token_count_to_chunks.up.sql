-- Add token_count column to code_chunks table for storing exact token counts from Google CountTokens API
-- This enables usage monitoring and chunk optimization

ALTER TABLE codechunking.code_chunks
ADD COLUMN IF NOT EXISTS token_count INTEGER;

ALTER TABLE codechunking.code_chunks
ADD COLUMN IF NOT EXISTS token_counted_at TIMESTAMP WITH TIME ZONE;

-- Index for queries filtering by token count (useful for chunk optimization)
CREATE INDEX IF NOT EXISTS idx_code_chunks_token_count
ON codechunking.code_chunks(token_count)
WHERE token_count IS NOT NULL;

-- Composite index for repository + token count queries (find oversized chunks per repo)
CREATE INDEX IF NOT EXISTS idx_code_chunks_repo_token_count
ON codechunking.code_chunks(repository_id, token_count)
WHERE deleted_at IS NULL AND token_count IS NOT NULL;

COMMENT ON COLUMN codechunking.code_chunks.token_count IS 'Exact token count from Google CountTokens API';
COMMENT ON COLUMN codechunking.code_chunks.token_counted_at IS 'Timestamp when token count was retrieved from API';
