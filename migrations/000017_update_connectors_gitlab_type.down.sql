-- Reverse migration 000017: restore gitlab_group type and drop JSONB columns

SET search_path TO codechunking, public;

UPDATE codechunking.connectors
    SET connector_type = 'gitlab_group'
    WHERE connector_type = 'gitlab';

ALTER TABLE codechunking.connectors
    DROP COLUMN groups,
    DROP COLUMN projects;
