package message

import (
	"context"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
	"strings"
	"unicode/utf8"
)

type Send struct{ messages repository.MessageWriter }

func NewSend(messages repository.MessageWriter) *Send { return &Send{messages: messages} }
func (u *Send) Execute(ctx context.Context, conversationID, userID int64, content string) (entity.Message, error) {
	if conversationID <= 0 || userID <= 0 || !utf8.ValidString(content) || strings.ContainsRune(content, 0) || strings.TrimSpace(content) == "" || utf8.RuneCountInString(content) > 2000 {
		return entity.Message{}, entity.ErrInvalidInput
	}
	// The write port must enforce membership inside its atomic persistence operation.
	return u.messages.Send(ctx, conversationID, userID, content)
}
