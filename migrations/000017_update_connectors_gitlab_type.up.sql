-- Migration 000017: add groups/projects JSONB columns and rename gitlab_group → gitlab

SET search_path TO codechunking, public;

ALTER TABLE codechunking.connectors
    ADD COLUMN groups   JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN projects JSONB NOT NULL DEFAULT '[]';

UPDATE codechunking.connectors
    SET connector_type = 'gitlab'
    WHERE connector_type = 'gitlab_group';
