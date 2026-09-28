-- 000001_initial_schema.up.sql
-- Complete schema from DBML design

BEGIN;

-- ============================================
-- USERS
-- ============================================
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    first_name VARCHAR(100) NOT NULL CHECK (btrim(first_name) <> ''),
    last_name VARCHAR(100) DEFAULT '',
    date_of_birth DATE,
    email VARCHAR(254) NOT NULL UNIQUE CHECK (btrim(email) <> ''),
    phone_number VARCHAR(20) UNIQUE,
    password_hash TEXT NOT NULL CHECK (btrim(password_hash) <> ''),
    avatar_url TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_phone ON users (phone_number) WHERE phone_number IS NOT NULL;
CREATE INDEX idx_users_deleted_at ON users (deleted_at);

-- ============================================
-- CONVERSATIONS
-- ============================================
CREATE TABLE conversations (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    is_group BOOLEAN NOT NULL DEFAULT false,
    name VARCHAR(100),
    avatar_url TEXT,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    -- Group conversations require a name; direct conversations derive name from other user
    CONSTRAINT chk_group_name CHECK (
        (is_group = true AND btrim(COALESCE(name, '')) <> '') OR
        (is_group = false AND name IS NULL)
    )
);

CREATE INDEX idx_conversations_created_by ON conversations (created_by);
CREATE INDEX idx_conversations_updated_at ON conversations (updated_at DESC);

-- ============================================
-- DIRECT_CONVERSATIONS
-- Junction table for 1-1 conversations
-- Enforces canonical identity for direct chats
-- ============================================
CREATE TABLE direct_conversations (
    conversation_id BIGINT NOT NULL PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
    user_low_id BIGINT NOT NULL REFERENCES users(id),
    user_high_id BIGINT NOT NULL REFERENCES users(id),

    -- user_low_id must always be less than user_high_id
    CONSTRAINT chk_user_pair_order CHECK (user_low_id < user_high_id)
);

CREATE UNIQUE INDEX idx_direct_conv_user_pair ON direct_conversations (user_low_id, user_high_id);

-- ============================================
-- CONVERSATION_MEMBERS
-- ============================================
CREATE TABLE conversation_members (
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_read_at TIMESTAMPTZ,
    last_read_message_id BIGINT,

    PRIMARY KEY (conversation_id, user_id),

    -- Both read fields must be null together or set together
    CONSTRAINT chk_read_fields CHECK (
        (last_read_message_id IS NULL) = (last_read_at IS NULL)
    )
);

CREATE INDEX idx_conv_members_user_conv ON conversation_members (user_id, conversation_id);

-- ============================================
-- MESSAGES
-- ============================================
CREATE TABLE messages (
    id BIGINT GENERATED ALWAYS AS IDENTITY (CACHE 1 NO CYCLE) PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id BIGINT NOT NULL REFERENCES users(id),
    content TEXT NOT NULL CHECK (
        char_length(content) <= 2000 AND btrim(content, E' \t\n\r\v\f') <> ''
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    -- Unique composite: (conversation_id, id) for pagination
    UNIQUE (conversation_id, id)
);

-- Composite FK: sender must be a conversation member
-- This prevents sending as a non-member
ALTER TABLE messages ADD CONSTRAINT fk_messages_sender_member
    FOREIGN KEY (conversation_id, sender_id)
    REFERENCES conversation_members (conversation_id, user_id);

-- Composite FK: last_read_message_id must belong to the same conversation
ALTER TABLE conversation_members ADD CONSTRAINT fk_members_read_message
    FOREIGN KEY (conversation_id, last_read_message_id)
    REFERENCES messages (conversation_id, id);

-- Index for conversation message history (pagination)
CREATE INDEX idx_messages_conv_created ON messages (conversation_id, id DESC);

-- Index for sender lookups
CREATE INDEX idx_messages_sender ON messages (sender_id);

-- ============================================
-- FUNCTIONS & TRIGGERS
-- ============================================

-- Function to update updated_at automatically
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = clock_timestamp();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for users.updated_at
CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Trigger for conversations.updated_at
CREATE TRIGGER trg_conversations_updated_at
    BEFORE UPDATE ON conversations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMIT;
