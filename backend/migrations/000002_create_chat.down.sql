ALTER TABLE conversation_members DROP CONSTRAINT members_read_message_fk;
DROP TABLE messages;
DROP TABLE conversation_members;
DROP TABLE conversations;
