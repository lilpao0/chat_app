-- 000001_initial_schema.down.sql
-- Rollback: drop all tables in correct order (respecting FK dependencies)

BEGIN;

-- Drop triggers first
DROP TRIGGER IF EXISTS trg_conversations_updated_at ON conversations;
DROP TRIGGER IF EXISTS trg_users_updated_at ON users;

-- Drop functions
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversation_members;
DROP TABLE IF EXISTS direct_conversations;
DROP TABLE IF EXISTS conversations;
DROP TABLE IF EXISTS users;

COMMIT;
