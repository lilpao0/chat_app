CREATE TABLE conversations (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE conversation_members (
    conversation_id BIGINT NOT NULL REFERENCES conversations(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    last_read_at TIMESTAMPTZ,
    last_read_message_id BIGINT,
    PRIMARY KEY (conversation_id, user_id),
    CHECK ((last_read_message_id IS NULL) = (last_read_at IS NULL))
);

CREATE INDEX conversation_members_user_conversation_idx
    ON conversation_members(user_id, conversation_id);

CREATE TABLE messages (
    id BIGINT GENERATED ALWAYS AS IDENTITY (CACHE 1 NO CYCLE) PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id),
    sender_id BIGINT NOT NULL REFERENCES users(id),
    content TEXT NOT NULL CHECK (
        char_length(content) <= 2000 AND btrim(content, E' \t\n\r\v\f') <> ''
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT messages_member_fk FOREIGN KEY (conversation_id, sender_id)
        REFERENCES conversation_members(conversation_id, user_id),
    UNIQUE (conversation_id, id)
);

-- The composite key prevents a read position pointing into another conversation.
ALTER TABLE conversation_members ADD CONSTRAINT members_read_message_fk
    FOREIGN KEY (conversation_id, last_read_message_id)
    REFERENCES messages(conversation_id, id);

-- Every application insert must lock its conversation before allocating an ID.
-- B21 implements that transaction; sequences alone do not impose commit order.
