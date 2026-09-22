package user_test

import (
	"context"
	"errors"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/user"
	"strings"
	"testing"
)

type searchFunc func(context.Context, int64, entity.UserSearchQuery) (entity.UserSearchPage, error)

func (f searchFunc) Search(ctx context.Context, id int64, q entity.UserSearchQuery) (entity.UserSearchPage, error) {
	return f(ctx, id, q)
}
func TestSearch(t *testing.T) {
	ctx := context.Background()
	cause := errors.New("storage failure")
	for _, failure := range []error{nil, cause} {
		u := user.NewSearch(searchFunc(func(got context.Context, id int64, q entity.UserSearchQuery) (entity.UserSearchPage, error) {
			if got != ctx || id != 7 || q.Text != "An" || q.Limit != 20 || q.AfterID != 3 {
				t.Fatal("search normalization/context incorrect")
			}
			return entity.UserSearchPage{}, failure
		}))
		if _, err := u.Execute(ctx, 7, entity.UserSearchQuery{Text: " An ", AfterID: 3}); !errors.Is(err, failure) {
			t.Fatal("error lost")
		}
	}
	for _, q := range []entity.UserSearchQuery{{Text: " "}, {Text: "a"}, {Text: strings.Repeat("a", 255)}, {Text: "abc\x00"}, {Text: "\xffxx"}, {Text: "valid", Limit: 51}, {Text: "valid", Limit: -1}, {Text: "valid", AfterID: -1}} {
		if _, err := user.NewSearch(nil).Execute(ctx, 7, q); !errors.Is(err, entity.ErrInvalidInput) {
			t.Fatal("invalid query reached repository")
		}
	}
	if _, err := user.NewSearch(nil).Execute(ctx, 0, entity.UserSearchQuery{Text: "An"}); !errors.Is(err, entity.ErrInvalidInput) {
		t.Fatal("invalid actor accepted")
	}
}
