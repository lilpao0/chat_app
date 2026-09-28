package conversation_test

import (
	"context"
	"errors"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/conversation"
	"testing"
)

type openFunc func(context.Context, int64, int64) (entity.DirectConversation, error)

func (f openFunc) OpenDirect(ctx context.Context, a, b int64) (entity.DirectConversation, error) {
	return f(ctx, a, b)
}
func TestOpenDirect(t *testing.T) {
	ctx := context.Background()
	cause := errors.New("db")
	for _, failure := range []error{nil, entity.ErrUserNotFound, cause} {
		u := conversation.NewOpenDirect(openFunc(func(got context.Context, a, b int64) (entity.DirectConversation, error) {
			if got != ctx || a != 7 || b != 9 {
				t.Fatal("actor or target changed")
			}
			return entity.DirectConversation{ID: 3, Created: true}, failure
		}))
		result, err := u.Execute(ctx, 7, 9)
		if !errors.Is(err, failure) || (err == nil && result.ID != 3) {
			t.Fatal("repository result/error lost")
		}
	}
	for _, pair := range [][2]int64{{0, 1}, {1, 0}, {1, 1}, {-1, 1}} {
		if _, err := conversation.NewOpenDirect(nil).Execute(ctx, pair[0], pair[1]); !errors.Is(err, entity.ErrInvalidInput) {
			t.Fatal("invalid/self conversation accepted")
		}
	}
}
