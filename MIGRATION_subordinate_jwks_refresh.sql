-- Migration: Add JWKS refresh configuration columns to subordinates
-- Purpose: Support opt-in periodic JWKS refresh per subordinate.
--
-- Adds two columns:
--   enable_jwks_update   BOOLEAN NOT NULL DEFAULT FALSE
--     When true, Lighthouse polls the subordinate's Entity Configuration for
--     JWKS changes and accepts jwks_update_trigger requests for it.
--   jwks_poll_interval   BIGINT NULL
--     Optional per-subordinate poll interval in seconds. When NULL (or <= 0),
--     the refresher derives the interval from the subordinate's Entity
--     Configuration expiration time.
--
-- Database-specific instructions:
-- =================================

-- For MariaDB/MySQL:
-- -------------------
ALTER TABLE subordinates
  ADD COLUMN enable_jwks_update TINYINT(1) NOT NULL DEFAULT 0,
  ADD COLUMN jwks_poll_interval BIGINT NULL;

-- For PostgreSQL:
-- ---------------
-- ALTER TABLE subordinates
--   ADD COLUMN enable_jwks_update BOOLEAN NOT NULL DEFAULT FALSE,
--   ADD COLUMN jwks_poll_interval BIGINT NULL;

-- For SQLite:
-- -----------
-- No migration needed - GORM AutoMigrate will add the columns. SQLite does not
-- support adding NOT NULL columns with defaults via ALTER TABLE in all cases,
-- but the application creates new tables with the columns present.

-- After running this migration:
-- =============================
-- 1. Restart your application (GORM AutoMigrate will reconcile the schema).
-- 2. Use the admin API to set enable_jwks_update=true on subordinates you
--    want Lighthouse to poll.
