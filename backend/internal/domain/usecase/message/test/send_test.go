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

type publishFunc func(context.Context, entity.Message) error

func (f publishFunc) PublishNewMessage(ctx context.Context, message entity.Message) error {
	return f(ctx, message)
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

func TestSendPublishesOnlyPersistedMessages(t *testing.T) {
	ctx := context.Background()
	persisted := entity.Message{ID: 8, ConversationID: 3, SenderID: 7, Content: "hello"}
	publishCalls := 0
	reported := error(nil)
	u := message.NewSendWithPublisher(
		sendFunc(func(context.Context, int64, int64, string) (entity.Message, error) { return persisted, nil }),
		publishFunc(func(got context.Context, value entity.Message) error {
			publishCalls++
			if got != ctx || value != persisted {
				t.Fatal("publisher did not receive committed message")
			}
			return errors.New("delivery unavailable")
		}),
		func(err error) { reported = err },
	)
	result, err := u.Execute(ctx, 3, 7, "hello")
	if err != nil || result != persisted || publishCalls != 1 || reported == nil {
		t.Fatalf("result=%+v err=%v calls=%d reported=%v", result, err, publishCalls, reported)
	}

	publishCalls = 0
	u = message.NewSendWithPublisher(
		sendFunc(func(context.Context, int64, int64, string) (entity.Message, error) {
			return entity.Message{}, errors.New("rollback")
		}),
		publishFunc(func(context.Context, entity.Message) error { publishCalls++; return nil }),
		nil,
	)
	if _, err := u.Execute(ctx, 3, 7, "hello"); err == nil || publishCalls != 0 {
		t.Fatalf("repository error=%v publish calls=%d", err, publishCalls)
	}
}
