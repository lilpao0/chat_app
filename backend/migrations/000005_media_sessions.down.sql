BEGIN;
LOCK TABLE attachments IN ACCESS EXCLUSIVE MODE;
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM attachments
        WHERE session_cipher IS NOT NULL OR cleanup_token IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'Cannot roll back media sessions while recovery credentials or cleanup claims exist';
    END IF;
END;
$$;
ALTER TABLE attachments
    DROP CONSTRAINT chk_attachments_cleanup_claim,
    DROP COLUMN cleanup_claimed_at,
    DROP COLUMN cleanup_checked_at,
    DROP COLUMN cleanup_token,
    DROP COLUMN session_cipher;
COMMIT;
