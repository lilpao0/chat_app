package conversation

import (
	"context"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

type MarkRead struct {
	positions repository.ReadPositionRepository
}

func NewMarkRead(positions repository.ReadPositionRepository) *MarkRead {
	return &MarkRead{positions: positions}
}
func (u *MarkRead) Execute(ctx context.Context, conversationID, userID, messageID int64) (int64, error) {
	if conversationID <= 0 || userID <= 0 || messageID <= 0 {
		return 0, entity.ErrInvalidInput
	}
	member, err := u.positions.IsMember(ctx, conversationID, userID)
	if err != nil {
		return 0, err
	}
	if !member {
		return 0, entity.ErrConversationNotFound
	}
	return u.positions.MarkRead(ctx, conversationID, userID, messageID)
}
