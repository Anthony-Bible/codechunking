-- Fix invalid 'indexed' status values by converting to 'completed'
-- Issue #23: Invalid repository status 'indexed' causes 500 error when listing repositories
UPDATE codechunking.repositories
SET status = 'completed', updated_at = NOW()
WHERE status = 'indexed';
