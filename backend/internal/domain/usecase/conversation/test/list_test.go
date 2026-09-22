package conversation_test

import (
	"context"
	"errors"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/conversation"
	"testing"
)

type listFunc func(context.Context, int64) ([]entity.ConversationSummary, error)

func (f listFunc) ListForUser(ctx context.Context, id int64) ([]entity.ConversationSummary, error) {
	return f(ctx, id)
}
func TestList(t *testing.T) {
	ctx := context.Background()
	cause := errors.New("database")
	for _, failure := range []error{nil, cause} {
		u := conversation.NewList(listFunc(func(got context.Context, id int64) ([]entity.ConversationSummary, error) {
			if got != ctx || id != 7 {
				t.Fatal("identity/context not forwarded")
			}
			return nil, failure
		}))
		items, err := u.Execute(ctx, 7)
		if !errors.Is(err, failure) {
			t.Fatal("repository error lost")
		}
		if err == nil && (items == nil || len(items) != 0) {
			t.Fatal("empty list must be non-nil")
		}
	}
	if _, err := conversation.NewList(nil).Execute(ctx, 0); !errors.Is(err, entity.ErrInvalidInput) {
		t.Fatal("invalid actor accepted")
	}
}
