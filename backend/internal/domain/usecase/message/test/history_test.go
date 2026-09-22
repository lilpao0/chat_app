package message_test

import (
	"context"
	"errors"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/message"
	"testing"
)

type reader struct {
	member                bool
	memberErr, historyErr error
	query                 entity.MessageQuery
	called                *bool
}

func (r *reader) IsMember(context.Context, int64, int64) (bool, error) { return r.member, r.memberErr }
func (r *reader) History(_ context.Context, c, u int64, q entity.MessageQuery) (entity.MessagePage, error) {
	*r.called = true
	r.query = q
	return entity.MessagePage{}, r.historyErr
}
func TestHistoryRules(t *testing.T) {
	ctx := context.Background()
	called := false
	r := &reader{member: true, called: &called}
	u := message.NewHistory(r)
	if _, err := u.Execute(ctx, 1, 2, entity.MessageQuery{}); err != nil || !called || r.query.Limit != 20 {
		t.Fatal("default limit not applied")
	}
	for _, q := range []entity.MessageQuery{{BeforeID: 1, AfterID: 2}, {BeforeID: -1}, {AfterID: -1}, {Limit: -1}, {Limit: 101}} {
		if _, err := message.NewHistory(nil).Execute(ctx, 1, 2, q); !errors.Is(err, entity.ErrInvalidInput) {
			t.Fatal("invalid query accepted")
		}
	}
	called = false
	r.member = false
	if _, err := u.Execute(ctx, 1, 2, entity.MessageQuery{}); !errors.Is(err, entity.ErrConversationNotFound) || called {
		t.Fatal("outsider reached history")
	}
	cause := errors.New("db")
	r.memberErr = cause
	if _, err := u.Execute(ctx, 1, 2, entity.MessageQuery{}); !errors.Is(err, cause) {
		t.Fatal("membership error lost")
	}
	r.memberErr = nil
	r.member = true
	r.historyErr = cause
	if _, err := u.Execute(ctx, 1, 2, entity.MessageQuery{Limit: 100, AfterID: 7}); !errors.Is(err, cause) || r.query.AfterID != 7 {
		t.Fatal("history error or cursor lost")
	}
}
