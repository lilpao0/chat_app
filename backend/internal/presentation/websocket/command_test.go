package websocket

import (
	"context"
	"fmt"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"testing"
)

type commandSender func(context.Context, int64, int64, string, string) (entity.Message, error)

func (f commandSender) ExecuteWithRequestID(ctx context.Context, c, u int64, text, key string) (entity.Message, error) {
	return f(ctx, c, u, text, key)
}
func TestCommandValidation(t *testing.T) {
	const key = "550e8400-e29b-41d4-a716-446655440000"
	calls := 0
	c := &Commands{Send: commandSender(func(ctx context.Context, conv, actor int64, text, id string) (entity.Message, error) {
		calls++
		if conv != 10 || actor != 7 || text != "hi" || id != key {
			t.Fatal("identity or input changed")
		}
		return entity.Message{ID: 1}, nil
	})}
	for _, raw := range []string{
		"null", "[]", "{}", "{", `{"type":"send_message","request_id":"invalid","data":{}}`,
		fmt.Sprintf(`{"type":"send_message","request_id":%q,"data":{"conversation_id":10,"content":"hi","sender_id":99}}`, key),
		fmt.Sprintf(`{"type":"send_message","request_id":%q,"data":null}`, key),
		fmt.Sprintf(`{"type":"mark_read","request_id":%q,"data":{"conversation_id":10,"last_read_message_id":null}}`, key),
		fmt.Sprintf(`{"type":"send_message","request_id":%q,"data":{"conversation_id":"10","content":"hi"}}`, key),
		fmt.Sprintf(`{"type":"send_message","request_id":%q,"data":{"conversation_id":10,"content":"hi"}} {}`, key),
		fmt.Sprintf(`{"type":"send_message","request_id":%q,"data":{"conversation_id":10,"content":"hi"},"user_id":7}`, key),
	} {
		if got := c.execute(context.Background(), 7, []byte(raw)); got.Type != "error" {
			t.Fatalf("accepted %s", raw)
		}
	}
	if calls != 0 {
		t.Fatal("invalid input reached use case")
	}
	got := c.execute(context.Background(), 7, []byte(fmt.Sprintf(`{"type":"send_message","request_id":%q,"data":{"conversation_id":10,"content":"hi"}}`, key)))
	if got.Type != "message_sent" || calls != 1 || *got.RequestID != key {
		t.Fatalf("reply=%+v", got)
	}
}
