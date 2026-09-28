package user

import (
	"context"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
	"strings"
	"unicode/utf8"
)

type Search struct{ users repository.UserSearcher }

func NewSearch(users repository.UserSearcher) *Search { return &Search{users: users} }

func (u *Search) Execute(ctx context.Context, actorID int64, q entity.UserSearchQuery) (entity.UserSearchPage, error) {
	q.Text = strings.TrimSpace(q.Text)
	if q.Limit == 0 {
		q.Limit = 20
	}
	length := utf8.RuneCountInString(q.Text)
	if actorID <= 0 || q.AfterID < 0 || q.Limit < 1 || q.Limit > 50 || !utf8.ValidString(q.Text) || strings.ContainsRune(q.Text, 0) || length < 2 || length > 254 {
		return entity.UserSearchPage{}, entity.ErrInvalidInput
	}
	return u.users.Search(ctx, actorID, q)
}
