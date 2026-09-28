package message

import (
	"context"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
)

type Publisher interface {
	PublishNewMessage(context.Context, entity.Message) error
}

type PublishErrorReporter func(error)
