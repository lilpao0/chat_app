package repository

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

type PostgresConversationRepository struct{ db *sql.DB }

var _ domainrepo.ConversationRepository = (*PostgresConversationRepository)(nil)

func NewPostgresConversationRepository(db *sql.DB) *PostgresConversationRepository {
	return &PostgresConversationRepository{db: db}
}

func (r *PostgresConversationRepository) ListForUser(ctx context.Context, userID int64) ([]entity.ConversationSummary, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.id,other.id,other.name,other.avatar_url,last.content,last.created_at,
 (SELECT count(*) FROM messages m WHERE m.conversation_id=c.id AND m.sender_id<>$1 AND m.id>COALESCE(own.last_read_message_id,0))
 FROM conversation_members own JOIN conversations c ON c.id=own.conversation_id
 JOIN LATERAL (SELECT u.id,u.name,u.avatar_url FROM conversation_members cm JOIN users u ON u.id=cm.user_id WHERE cm.conversation_id=c.id AND cm.user_id<>$1 ORDER BY u.id LIMIT 1) other ON true
 LEFT JOIN LATERAL (SELECT content,created_at FROM messages WHERE conversation_id=c.id ORDER BY id DESC LIMIT 1) last ON true
 WHERE own.user_id=$1 ORDER BY c.updated_at DESC,c.id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()
	result := make([]entity.ConversationSummary, 0)
	for rows.Next() {
		var item entity.ConversationSummary
		var content sql.NullString
		var at sql.NullTime
		if err := rows.Scan(&item.ID, &item.Participant.ID, &item.Participant.Name, &item.Participant.AvatarURL, &content, &at, &item.UnreadCount); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		if content.Valid {
			item.LastMessage = &content.String
		}
		if at.Valid {
			utc := at.Time.UTC()
			item.LastMessageAt = &utc
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read conversations: %w", err)
	}
	return result, nil
}
