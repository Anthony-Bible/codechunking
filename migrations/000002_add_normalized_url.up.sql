-- Add normalized_url column and unique constraint for duplicate detection
-- This migration adds support for normalized URL-based duplicate detection

SET search_path TO codechunking, public;

-- Add normalized_url column to repositories table
ALTER TABLE repositories 
ADD COLUMN normalized_url VARCHAR(512);

-- Populate normalized_url column with normalized versions of existing URLs
-- This uses a simple normalization for existing data
UPDATE repositories 
SET normalized_url = LOWER(
    CASE 
        WHEN url LIKE 'http://%' THEN 'https' || SUBSTRING(url FROM 5)
        ELSE url 
    END
);

-- Remove .git suffix and trailing slashes for existing URLs
UPDATE repositories 
SET normalized_url = REGEXP_REPLACE(
    REGEXP_REPLACE(normalized_url, '\.git/?$', ''), 
    '/+$', 
    ''
);

-- Make normalized_url NOT NULL after populating
ALTER TABLE repositories 
ALTER COLUMN normalized_url SET NOT NULL;

-- Add unique constraint on normalized_url to prevent duplicates
ALTER TABLE repositories 
ADD CONSTRAINT uk_repositories_normalized_url UNIQUE (normalized_url);

-- Create index on normalized_url for performance
CREATE INDEX idx_repositories_normalized_url ON repositories(normalized_url);