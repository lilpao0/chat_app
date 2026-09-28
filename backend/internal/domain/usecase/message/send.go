package message

import (
	"context"
	"errors"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
	"strings"
	"unicode/utf8"
)

type Send struct {
	messages repository.MessageWriter
	publish  Publisher
	report   PublishErrorReporter
}

func NewSend(messages repository.MessageWriter) *Send { return &Send{messages: messages} }

func NewSendWithPublisher(messages repository.MessageWriter, publish Publisher, report PublishErrorReporter) *Send {
	return &Send{messages: messages, publish: publish, report: report}
}
func (u *Send) Execute(ctx context.Context, conversationID, userID int64, content string) (entity.Message, error) {
	return u.ExecuteWithRequestID(ctx, conversationID, userID, content, "")
}

func (u *Send) ExecuteWithRequestID(ctx context.Context, conversationID, userID int64, content, requestID string) (entity.Message, error) {
	if requestID != "" && !entity.ValidRequestID(requestID) {
		return entity.Message{}, entity.ErrInvalidInput
	}
	if conversationID <= 0 || userID <= 0 || !utf8.ValidString(content) || strings.ContainsRune(content, 0) || strings.TrimSpace(content) == "" || utf8.RuneCountInString(content) > 2000 {
		return entity.Message{}, entity.ErrInvalidInput
	}
	// The write port must enforce membership inside its atomic persistence operation.
	var persisted entity.Message
	var err error
	created := true
	if requestID == "" {
		persisted, err = u.messages.Send(ctx, conversationID, userID, content)
	} else if writer, ok := u.messages.(repository.IdempotentMessageWriter); ok {
		persisted, created, err = writer.SendOnce(ctx, conversationID, userID, content, requestID)
	} else {
		return entity.Message{}, errors.New("idempotent writer unavailable")
	}
	if err != nil {
		return entity.Message{}, err
	}
	if created && u.publish != nil {
		if err := u.publish.PublishNewMessage(ctx, persisted); err != nil && u.report != nil {
			u.report(err)
		}
	}
	return persisted, nil
}
