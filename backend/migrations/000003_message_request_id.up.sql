ALTER TABLE messages ADD COLUMN client_request_id uuid;
ALTER TABLE messages ADD CONSTRAINT messages_sender_request_key UNIQUE (sender_id, client_request_id);
