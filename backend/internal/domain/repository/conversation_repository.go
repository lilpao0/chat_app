package repository

import (
	"context"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
)

type ConversationRepository interface {
	ListForUser(context.Context, int64) ([]entity.ConversationSummary, error)
}

type ConversationMemberReader interface {
	MemberIDs(context.Context, int64) ([]int64, error)
}
