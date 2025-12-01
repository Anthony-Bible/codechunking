-- Revert: convert 'completed' back to 'indexed' (only for rollback purposes)
-- Note: This is a lossy operation - we can't distinguish which were originally 'indexed'
-- This down migration is intentionally a no-op for safety
SELECT 1; -- No-op
