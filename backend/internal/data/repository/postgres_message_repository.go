package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

type PostgresMessageRepository struct{ db *sql.DB }

var _ domainrepo.MessageWriter = (*PostgresMessageRepository)(nil)
var _ domainrepo.IdempotentMessageWriter = (*PostgresMessageRepository)(nil)

func NewPostgresMessageRepository(db *sql.DB) *PostgresMessageRepository {
	return &PostgresMessageRepository{db: db}
}

func (r *PostgresMessageRepository) Send(ctx context.Context, conversationID, userID int64, content string) (entity.Message, error) {
	message, _, err := r.SendOnce(ctx, conversationID, userID, content, "")
	return message, err
}

func (r *PostgresMessageRepository) SendOnce(ctx context.Context, conversationID, userID int64, content, requestID string) (entity.Message, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Message{}, false, fmt.Errorf("begin send: %w", err)
	}
	defer tx.Rollback()
	var locked int64
	err = tx.QueryRowContext(ctx, `SELECT c.id FROM conversations c WHERE c.id=$1 AND EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=$2) FOR UPDATE OF c`, conversationID, userID).Scan(&locked)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Message{}, false, entity.ErrConversationNotFound
	}
	if err != nil {
		return entity.Message{}, false, fmt.Errorf("lock conversation: %w", err)
	}
	var message entity.Message
	// The unique index also serializes retries targeting different conversations.
	// DO NOTHING leaves the transaction usable; the next READ COMMITTED statement
	// sees the winning committed row without retrying an aborted transaction.
	err = tx.QueryRowContext(ctx, `INSERT INTO messages(conversation_id,sender_id,content,client_request_id) VALUES ($1,$2,$3,NULLIF($4,'')::uuid) ON CONFLICT (sender_id,client_request_id) DO NOTHING RETURNING id,conversation_id,sender_id,content,created_at`, conversationID, userID, content, requestID).Scan(&message.ID, &message.ConversationID, &message.SenderID, &message.Content, &message.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `SELECT id,conversation_id,sender_id,content,created_at FROM messages WHERE sender_id=$1 AND client_request_id=$2::uuid`, userID, requestID).Scan(&message.ID, &message.ConversationID, &message.SenderID, &message.Content, &message.CreatedAt)
		if err != nil {
			return entity.Message{}, false, err
		}
		if message.ConversationID != conversationID || message.Content != content {
			return entity.Message{}, false, entity.ErrRequestConflict
		}
		if err := tx.Commit(); err != nil {
			return entity.Message{}, false, err
		}
		message.CreatedAt = message.CreatedAt.UTC()
		return message, false, nil
	}
	if err != nil {
		return entity.Message{}, false, fmt.Errorf("insert message: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE conversations SET updated_at=GREATEST(updated_at,$2) WHERE id=$1`, conversationID, message.CreatedAt); err != nil {
		return entity.Message{}, false, fmt.Errorf("update conversation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return entity.Message{}, false, fmt.Errorf("commit message: %w", err)
	}
	message.CreatedAt = message.CreatedAt.UTC()
	return message, true, nil
}
