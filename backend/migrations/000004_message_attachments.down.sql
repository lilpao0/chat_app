BEGIN;

-- Hold writers out between the data checks and restoring the old schema.
LOCK TABLE messages, attachments IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM attachments) THEN
        RAISE EXCEPTION 'Cannot roll back media schema while attachments exist';
    END IF;
    IF EXISTS (
        SELECT 1 FROM messages
        WHERE btrim(content, E' \t\n\r\v\f') = ''
    ) THEN
        RAISE EXCEPTION 'Cannot roll back media schema while blank messages exist';
    END IF;
END;
$$;

ALTER TABLE messages DROP CONSTRAINT messages_content_length_check;
ALTER TABLE messages ADD CONSTRAINT messages_content_check CHECK (
    char_length(content) <= 2000 AND btrim(content, E' \t\n\r\v\f') <> ''
);

DROP TABLE attachments;

COMMIT;
