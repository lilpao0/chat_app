-- Messages survive rollback; retry deduplication metadata does not.
ALTER TABLE messages DROP CONSTRAINT messages_sender_request_key;
ALTER TABLE messages DROP COLUMN client_request_id;
