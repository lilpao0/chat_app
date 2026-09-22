package repository

import (
	"context"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
)

type MessageWriter interface {
	Send(context.Context, int64, int64, string) (entity.Message, error)
}

type MembershipReader interface {
	IsMember(context.Context, int64, int64) (bool, error)
}
type MessageReader interface {
	MembershipReader
	History(context.Context, int64, int64, entity.MessageQuery) (entity.MessagePage, error)
}
type ReadPositionRepository interface {
	MembershipReader
	MarkRead(context.Context, int64, int64, int64) (int64, error)
}
