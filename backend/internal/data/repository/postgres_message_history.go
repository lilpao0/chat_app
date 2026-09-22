package repository

import (
	"context"
	"fmt"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

var _ domainrepo.MessageReader = (*PostgresMessageRepository)(nil)

func (r *PostgresMessageRepository) IsMember(ctx context.Context, conversationID, userID int64) (bool, error) {
	var member bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2)`, conversationID, userID).Scan(&member)
	if err != nil {
		return false, fmt.Errorf("check conversation membership: %w", err)
	}
	return member, nil
}

func (r *PostgresMessageRepository) History(ctx context.Context, conversationID, userID int64, q entity.MessageQuery) (entity.MessagePage, error) {
	if q.Limit < 1 || q.Limit > 100 || q.BeforeID < 0 || q.AfterID < 0 || (q.BeforeID > 0 && q.AfterID > 0) {
		return entity.MessagePage{}, entity.ErrInvalidInput
	}
	query := `SELECT m.id,m.conversation_id,m.sender_id,m.content,m.created_at FROM messages m WHERE m.conversation_id=$1 AND EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=m.conversation_id AND cm.user_id=$2)`
	boundary := q.BeforeID
	if q.AfterID > 0 {
		boundary = q.AfterID
		query += ` AND m.id>$3 ORDER BY m.id ASC LIMIT $4`
	} else {
		query += ` AND ($3::bigint=0 OR m.id<$3) ORDER BY m.id DESC LIMIT $4`
	}
	rows, err := r.db.QueryContext(ctx, query, conversationID, userID, boundary, q.Limit+1)
	if err != nil {
		return entity.MessagePage{}, fmt.Errorf("load history: %w", err)
	}
	defer rows.Close()
	page := entity.MessagePage{Items: make([]entity.Message, 0, q.Limit)}
	for rows.Next() {
		var m entity.Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Content, &m.CreatedAt); err != nil {
			return entity.MessagePage{}, fmt.Errorf("scan message: %w", err)
		}
		m.CreatedAt = m.CreatedAt.UTC()
		page.Items = append(page.Items, m)
	}
	if err := rows.Err(); err != nil {
		return entity.MessagePage{}, fmt.Errorf("read history: %w", err)
	}
	if len(page.Items) > q.Limit {
		page.HasMore = true
		page.Items = page.Items[:q.Limit]
		last := page.Items[len(page.Items)-1].ID
		if q.AfterID > 0 {
			page.NextAfterID = &last
		} else {
			page.NextBeforeID = &last
		}
	}
	return page, nil
}
