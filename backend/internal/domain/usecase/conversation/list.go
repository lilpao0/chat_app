package conversation

import (
	"context"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

type List struct {
	conversations repository.ConversationRepository
}

func NewList(conversations repository.ConversationRepository) *List {
	return &List{conversations: conversations}
}
func (u *List) Execute(ctx context.Context, userID int64) ([]entity.ConversationSummary, error) {
	if userID <= 0 {
		return nil, entity.ErrInvalidInput
	}
	items, err := u.conversations.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []entity.ConversationSummary{}
	}
	return items, nil
}
