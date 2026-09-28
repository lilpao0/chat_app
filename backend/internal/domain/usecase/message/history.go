package message

import (
	"context"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

type History struct{ messages repository.MessageReader }

func NewHistory(messages repository.MessageReader) *History { return &History{messages: messages} }
func (u *History) Execute(ctx context.Context, conversationID, userID int64, q entity.MessageQuery) (entity.MessagePage, error) {
	if q.Limit == 0 {
		q.Limit = 20
	}
	if conversationID <= 0 || userID <= 0 || q.Limit < 1 || q.Limit > 100 || q.BeforeID < 0 || q.AfterID < 0 || (q.BeforeID > 0 && q.AfterID > 0) {
		return entity.MessagePage{}, entity.ErrInvalidInput
	}
	member, err := u.messages.IsMember(ctx, conversationID, userID)
	if err != nil {
		return entity.MessagePage{}, err
	}
	if !member {
		return entity.MessagePage{}, entity.ErrConversationNotFound
	}
	return u.messages.History(ctx, conversationID, userID, q)
}
