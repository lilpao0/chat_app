package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

var _ domainrepo.ReadPositionRepository = (*PostgresMessageRepository)(nil)

func (r *PostgresMessageRepository) MarkRead(ctx context.Context, conversationID, userID, messageID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin read update: %w", err)
	}
	defer tx.Rollback()
	var old sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT last_read_message_id FROM conversation_members WHERE conversation_id=$1 AND user_id=$2 FOR UPDATE`, conversationID, userID).Scan(&old)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, entity.ErrConversationNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("lock read position: %w", err)
	}
	var valid bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE conversation_id=$1 AND id=$2)`, conversationID, messageID).Scan(&valid); err != nil {
		return 0, fmt.Errorf("validate read marker: %w", err)
	}
	if !valid {
		return 0, entity.ErrInvalidInput
	}
	var marker int64
	err = tx.QueryRowContext(ctx, `UPDATE conversation_members SET
 last_read_at=CASE WHEN last_read_message_id IS NULL OR last_read_message_id<$3 THEN clock_timestamp() ELSE last_read_at END,
 last_read_message_id=GREATEST(COALESCE(last_read_message_id,0),$3)
 WHERE conversation_id=$1 AND user_id=$2 RETURNING last_read_message_id`, conversationID, userID, messageID).Scan(&marker)
	if err != nil {
		return 0, fmt.Errorf("advance read marker: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit read marker: %w", err)
	}
	return marker, nil
}
