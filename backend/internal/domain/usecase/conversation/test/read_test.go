package conversation_test

import (
	"context"
	"errors"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/conversation"
	"testing"
)

type positions struct {
	member             bool
	memberErr, readErr error
	called             bool
}

func (p *positions) IsMember(context.Context, int64, int64) (bool, error) {
	return p.member, p.memberErr
}
func (p *positions) MarkRead(_ context.Context, c, u, m int64) (int64, error) {
	p.called = true
	return m + 1, p.readErr
}
func TestReadRules(t *testing.T) {
	ctx := context.Background()
	p := &positions{member: true}
	u := conversation.NewMarkRead(p)
	if marker, err := u.Execute(ctx, 1, 2, 3); err != nil || marker != 4 {
		t.Fatal("effective marker lost")
	}
	if _, err := conversation.NewMarkRead(nil).Execute(ctx, 1, 2, 0); !errors.Is(err, entity.ErrInvalidInput) {
		t.Fatal("zero marker accepted")
	}
	p.member = false
	p.called = false
	if _, err := u.Execute(ctx, 1, 2, 3); !errors.Is(err, entity.ErrConversationNotFound) || p.called {
		t.Fatal("outsider reached update")
	}
	cause := errors.New("db")
	p.memberErr = cause
	if _, err := u.Execute(ctx, 1, 2, 3); !errors.Is(err, cause) {
		t.Fatal("membership error lost")
	}
	p.memberErr = nil
	p.member = true
	p.readErr = entity.ErrInvalidInput
	if _, err := u.Execute(ctx, 1, 2, 3); !errors.Is(err, entity.ErrInvalidInput) {
		t.Fatal("invalid marker error lost")
	}
}
