package repository

import (
	"context"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
)

type UserSearcher interface {
	Search(context.Context, int64, entity.UserSearchQuery) (entity.UserSearchPage, error)
}

type DirectConversationOpener interface {
	OpenDirect(context.Context, int64, int64) (entity.DirectConversation, error)
}
