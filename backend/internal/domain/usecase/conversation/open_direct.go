package conversation

import (
	"context"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

type OpenDirect struct {
	conversations repository.DirectConversationOpener
}

func NewOpenDirect(conversations repository.DirectConversationOpener) *OpenDirect {
	return &OpenDirect{conversations: conversations}
}
func (u *OpenDirect) Execute(ctx context.Context, actorID, otherID int64) (entity.DirectConversation, error) {
	if actorID <= 0 || otherID <= 0 || actorID == otherID {
		return entity.DirectConversation{}, entity.ErrInvalidInput
	}
	return u.conversations.OpenDirect(ctx, actorID, otherID)
}
