-- Rollback: drop connectors table

SET search_path TO codechunking, public;

DROP TABLE IF EXISTS codechunking.connectors;
