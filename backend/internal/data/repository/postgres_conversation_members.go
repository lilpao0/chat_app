package repository

import (
	"context"
	"fmt"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

var _ domainrepo.ConversationMemberReader = (*PostgresConversationRepository)(nil)

func (r *PostgresConversationRepository) MemberIDs(ctx context.Context, conversationID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT cm.user_id
		FROM conversation_members cm
		JOIN users u ON u.id=cm.user_id AND u.deleted_at IS NULL
		WHERE cm.conversation_id=$1
		ORDER BY cm.user_id`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list conversation members: %w", err)
	}
	defer rows.Close()
	ids := make([]int64, 0, 2)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan conversation member: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read conversation members: %w", err)
	}
	if len(ids) == 0 {
		return nil, entity.ErrConversationNotFound
	}
	return ids, nil
}
