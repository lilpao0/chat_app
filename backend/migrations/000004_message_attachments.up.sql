BEGIN;

-- ponytail: cross-table text-or-attachment validation belongs to the atomic
-- send path (MED-05); add a deferred trigger if direct SQL writers are supported.
ALTER TABLE messages
    DROP CONSTRAINT messages_content_check;
ALTER TABLE messages
    ADD CONSTRAINT messages_content_length_check
    CHECK (char_length(content) <= 2000);

CREATE TABLE attachments (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    conversation_id BIGINT NOT NULL,
    owner_id BIGINT NOT NULL,
    message_id BIGINT,
    position SMALLINT,

    state TEXT NOT NULL DEFAULT 'uploading',
    object_key TEXT NOT NULL UNIQUE,
    object_generation BIGINT,

    declared_content_type TEXT NOT NULL,
    declared_size_bytes BIGINT NOT NULL,

    kind TEXT,
    content_type TEXT,
    size_bytes BIGINT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
        DEFAULT (now() + INTERVAL '24 hours'),

    CONSTRAINT fk_attachments_owner_member
        FOREIGN KEY (conversation_id, owner_id)
        REFERENCES conversation_members (conversation_id, user_id),

    CONSTRAINT fk_attachments_message_conversation
        FOREIGN KEY (conversation_id, message_id)
        REFERENCES messages (conversation_id, id),

    CONSTRAINT attachments_message_position_key
        UNIQUE (message_id, position),

    CONSTRAINT chk_attachments_state
        CHECK (
            state IN (
                'uploading', 'ready', 'attached',
                'failed', 'deleting', 'deleted'
            )
        ),
    CONSTRAINT chk_attachments_binding
        CHECK (
            (
                state = 'attached'
                AND message_id IS NOT NULL
                AND position IS NOT NULL
            )
            OR
            (
                state <> 'attached'
                AND message_id IS NULL
                AND position IS NULL
            )
        ),

    CONSTRAINT chk_attachments_position
        CHECK (position BETWEEN 0 AND 4),

    CONSTRAINT chk_attachments_object_key
        CHECK (btrim(object_key) <> ''),

    CONSTRAINT chk_attachments_generation
        CHECK (object_generation > 0),

    CONSTRAINT chk_attachments_declared_media
        CHECK (
            (
                declared_content_type IN ('image/jpeg', 'image/png')
                AND declared_size_bytes BETWEEN 1 AND 10485760
            )
            OR
            (
                declared_content_type = 'video/mp4'
                AND declared_size_bytes BETWEEN 1 AND 52428800
            )
        ),
    CONSTRAINT chk_attachments_verified_media
        CHECK (
            (
                kind IS NULL
                AND content_type IS NULL
                AND size_bytes IS NULL
            )
            OR
            (
                kind IS NOT NULL
                AND content_type IS NOT NULL
                AND size_bytes IS NOT NULL
                AND (
                    (
                        kind = 'image'
                        AND content_type IN ('image/jpeg', 'image/png')
                    )
                    OR
                    (
                        kind = 'video'
                        AND content_type = 'video/mp4'
                    )
                )
                AND content_type = declared_content_type
                AND size_bytes = declared_size_bytes
            )
        ),

    CONSTRAINT chk_attachments_ready_metadata
        CHECK (
            state NOT IN ('ready', 'attached')
            OR (
                object_generation IS NOT NULL
                AND kind IS NOT NULL
                AND content_type IS NOT NULL
                AND size_bytes IS NOT NULL
            )
        ),

    CONSTRAINT chk_attachments_expiry
        CHECK (
            expires_at = created_at + INTERVAL '24 hours'
        )
);

CREATE INDEX idx_attachments_pending_owner
    ON attachments (owner_id, expires_at)
    WHERE state IN ('uploading', 'ready');

CREATE INDEX idx_attachments_cleanup
    ON attachments (expires_at)
    WHERE state IN ('uploading', 'ready', 'failed', 'deleting');

COMMIT;
