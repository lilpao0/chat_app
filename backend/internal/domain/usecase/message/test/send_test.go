package message_test

import (
	"context"
	"errors"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/message"
	"strings"
	"testing"
)

type sendFunc func(context.Context, int64, int64, string) (entity.Message, error)

func (f sendFunc) Send(ctx context.Context, c, u int64, s string) (entity.Message, error) {
	return f(ctx, c, u, s)
}
func TestSend(t *testing.T) {
	ctx := context.Background()
	cause := errors.New("db")
	for _, failure := range []error{nil, entity.ErrConversationNotFound, cause} {
		u := message.NewSend(sendFunc(func(got context.Context, c, id int64, text string) (entity.Message, error) {
			if got != ctx || c != 3 || id != 7 || text != " hi \n" {
				t.Fatal("send inputs changed")
			}
			return entity.Message{ID: 8}, failure
		}))
		_, err := u.Execute(ctx, 3, 7, " hi \n")
		if !errors.Is(err, failure) {
			t.Fatal("error lost")
		}
	}
	for _, content := range []string{"", " \t\n", "\u2003", strings.Repeat("a", 2001), "abc\x00", "\xff"} {
		if _, err := message.NewSend(nil).Execute(ctx, 3, 7, content); !errors.Is(err, entity.ErrInvalidInput) {
			t.Fatal("invalid content accepted")
		}
	}
	if _, err := message.NewSend(nil).Execute(ctx, 0, 7, "hello"); !errors.Is(err, entity.ErrInvalidInput) {
		t.Fatal("invalid conversation accepted")
	}
}
