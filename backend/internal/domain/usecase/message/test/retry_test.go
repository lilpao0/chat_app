package message_test

import (
	"context"
	"errors"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/message"
	"testing"
)

type retryWriter struct {
	created bool
	err     error
}

func (r retryWriter) Send(context.Context, int64, int64, string) (entity.Message, error) {
	panic("keyed command used legacy path")
}
func (r retryWriter) SendOnce(context.Context, int64, int64, string, string) (entity.Message, bool, error) {
	return entity.Message{ID: 1}, r.created, r.err
}
func TestKeyedPublishOnlyOnNewCommit(t *testing.T) {
	for _, tc := range []struct {
		created bool
		err     error
		calls   int
	}{{true, nil, 1}, {false, nil, 0}, {false, errors.New("rollback"), 0}} {
		calls := 0
		reported := false
		u := message.NewSendWithPublisher(retryWriter{tc.created, tc.err}, publishFunc(func(context.Context, entity.Message) error { calls++; return errors.New("offline") }), func(error) { reported = true })
		m, err := u.ExecuteWithRequestID(context.Background(), 1, 2, "hello", "550e8400-e29b-41d4-a716-446655440000")
		if !errors.Is(err, tc.err) || calls != tc.calls {
			t.Fatalf("err=%v calls=%d", err, calls)
		}
		if tc.err == nil && m.ID != 1 {
			t.Fatal("committed result lost")
		}
		if tc.calls == 1 && !reported {
			t.Fatal("publication error not reported")
		}
	}
}
