BEGIN;
ALTER TABLE users
ALTER COLUMN avatar_url
SET DEFAULT 'https://clipart-library.com/img/1816203.png';

UPDATE users
SET avatar_url = 'https://clipart-library.com/img/1816203.png'
WHERE avatar_url = '';

COMMIT;