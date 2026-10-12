BEGIN;

-- Session URIs are bearer credentials. Store only authenticated ciphertext.
ALTER TABLE attachments
    ADD COLUMN session_cipher BYTEA,
    ADD COLUMN cleanup_token TEXT,
    ADD COLUMN cleanup_claimed_at TIMESTAMPTZ,
    ADD COLUMN cleanup_checked_at TIMESTAMPTZ,
    ADD CONSTRAINT chk_attachments_cleanup_claim
        CHECK ((cleanup_token IS NULL) = (cleanup_claimed_at IS NULL));

COMMIT;
