package repository

import (
	"context"
	"fmt"

	"github.com/lib/pq"
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
	if err := rows.Close(); err != nil {
		return entity.MessagePage{}, fmt.Errorf("close history rows: %w", err)
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
	if err := r.loadAttachments(ctx, conversationID, page.Items); err != nil {
		return entity.MessagePage{}, err
	}
	return page, nil
}

func (r *PostgresMessageRepository) loadAttachments(
	ctx context.Context,
	conversationID int64,
	messages []entity.Message,
) error {
	if len(messages) == 0 {
		return nil
	}

	ids := make([]int64, len(messages))
	indexByID := make(map[int64]int, len(messages))
	for i := range messages {
		ids[i] = messages[i].ID
		indexByID[messages[i].ID] = i
		messages[i].Attachments = make([]entity.Attachment, 0)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT message_id, id, kind, content_type, size_bytes
		FROM attachments
		WHERE conversation_id =$1
			AND message_id = ANY($2::bigint[])
			AND state ='attached'
		ORDER BY message_id, position
	`, conversationID, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("load history attachments:%w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var messageID int64
		var attachment entity.Attachment
		if err := rows.Scan(
			&messageID,
			&attachment.ID,
			&attachment.Kind,
			&attachment.ContentType,
			&attachment.SizeBytes,
		); err != nil {
			return fmt.Errorf("scan history attachment:%w", err)
		}

		i := indexByID[messageID]
		messages[i].Attachments = append(messages[i].Attachments, attachment)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read history attachments:%w", err)
	}
	return nil
}
