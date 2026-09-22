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

func NewPostgresMessageRepository(db *sql.DB) *PostgresMessageRepository {
	return &PostgresMessageRepository{db: db}
}

func (r *PostgresMessageRepository) Send(ctx context.Context, conversationID, userID int64, content string) (entity.Message, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.Message{}, fmt.Errorf("begin send: %w", err)
	}
	defer tx.Rollback()
	var locked int64
	err = tx.QueryRowContext(ctx, `SELECT c.id FROM conversations c WHERE c.id=$1 AND EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=$2) FOR UPDATE OF c`, conversationID, userID).Scan(&locked)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Message{}, entity.ErrConversationNotFound
	}
	if err != nil {
		return entity.Message{}, fmt.Errorf("lock conversation: %w", err)
	}
	var message entity.Message
	err = tx.QueryRowContext(ctx, `INSERT INTO messages(conversation_id,sender_id,content) VALUES ($1,$2,$3) RETURNING id,conversation_id,sender_id,content,created_at`, conversationID, userID, content).Scan(&message.ID, &message.ConversationID, &message.SenderID, &message.Content, &message.CreatedAt)
	if err != nil {
		return entity.Message{}, fmt.Errorf("insert message: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE conversations SET updated_at=GREATEST(updated_at,$2) WHERE id=$1`, conversationID, message.CreatedAt); err != nil {
		return entity.Message{}, fmt.Errorf("update conversation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return entity.Message{}, fmt.Errorf("commit message: %w", err)
	}
	message.CreatedAt = message.CreatedAt.UTC()
	return message, nil
}
