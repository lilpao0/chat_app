package websocket_test

import (
	"context"
	"encoding/json"
	"fmt"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	dataauth "github.com/lilpao0/chat_app/backend/internal/data/auth"
	datarepo "github.com/lilpao0/chat_app/backend/internal/data/repository"
	"github.com/lilpao0/chat_app/backend/internal/data/seed"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/conversation"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/message"
	presentation "github.com/lilpao0/chat_app/backend/internal/presentation/http"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/handler"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/middleware"
	ws "github.com/lilpao0/chat_app/backend/internal/presentation/websocket"
	"github.com/lilpao0/chat_app/backend/internal/testutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBidirectionalChatAcceptance(t *testing.T) {
	db := testutil.Database(t)
	ctx := context.Background()
	a := repository.CreateUser{FirstName: "A", Email: "two-a@test.local", PasswordHash: "fixture"}
	ab, err := seed.Demo(ctx, db, a, repository.CreateUser{FirstName: "B", Email: "two-b@test.local", PasswordHash: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	ac, err := seed.Demo(ctx, db, a, repository.CreateUser{FirstName: "C", Email: "two-c@test.local", PasswordHash: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := dataauth.NewJWT(strings.Repeat("s", 32), "test", "mobile", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	commands := &ws.Commands{}
	cfg := testLifecycleConfig()
	cfg.MaxMessageBytes = 16384
	cfg.SendQueueCapacity = 32
	hub := ws.NewHub(cfg, commands)
	messages := datarepo.NewPostgresMessageRepository(db)
	send := message.NewSendWithPublisher(messages, ws.NewPublisher(datarepo.NewPostgresConversationRepository(db), hub), func(err error) { t.Errorf("publish: %v", err) })
	commands.Send = send
	commands.Read = conversation.NewMarkRead(messages)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	presentation.RegisterWebSocketRoute(r, ws.NewHandler(tokens, hub).Handle)
	protected := r.Group("/api", middleware.Authenticate(tokens))
	protected.POST("/conversations/:id/messages", handler.NewSendMessageHandler(send).Send)
	server := httptest.NewServer(r)
	defer server.Close()
	issue := func(id int64) string {
		token, err := tokens.Issue(id)
		if err != nil {
			t.Fatal(err)
		}
		return token.Value
	}
	tokenA, tokenB, tokenC := issue(ab.UserAID), issue(ab.UserBID), issue(ac.UserBID)
	a1, a2, b, c := connect(t, server, tokenA), connect(t, server, tokenA), connect(t, server, tokenB), connect(t, server, tokenC)
	waitForConnections(t, hub, ab.UserAID, 2)
	waitForConnections(t, hub, ab.UserBID, 1)
	write := func(conn *coderws.Conn, kind, key string, data any) {
		raw, err := json.Marshal(map[string]any{"type": kind, "request_id": key, "data": data})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := conn.Write(ctx, coderws.MessageText, raw); err != nil {
			t.Fatal(err)
		}
	}
	const key = "550e8400-e29b-41d4-a716-446655440000"
	body := map[string]any{"conversation_id": ab.ConversationID, "content": strings.Repeat("🙂", 2000)}
	write(a1, "send_message", key, body)
	var messageID float64
	seen := map[string]bool{}
	for range 2 {
		event := readEvent(t, a1)
		seen[event["type"].(string)] = true
		messageID = event["data"].(map[string]any)["id"].(float64)
	}
	if !seen["message_sent"] || !seen["new_message"] {
		t.Fatalf("missing ack/event: %v", seen)
	}
	for _, conn := range []*coderws.Conn{a2, b} {
		event := readEvent(t, conn)
		if event["type"] != "new_message" || event["data"].(map[string]any)["id"] != messageID {
			t.Fatalf("bad fanout: %v", event)
		}
	}
	write(a1, "send_message", key, body)
	retry := readEvent(t, a1)
	if retry["type"] != "message_sent" || retry["data"].(map[string]any)["id"] != messageID {
		t.Fatalf("retry: %v", retry)
	}
	// A fresh connection resolves a lost acknowledgement using the same durable key.
	_ = a1.CloseNow()
	a1 = connect(t, server, tokenA)
	write(a1, "send_message", key, body)
	if got := readEvent(t, a1); got["type"] != "message_sent" || got["data"].(map[string]any)["id"] != messageID {
		t.Fatalf("reconnect retry: %v", got)
	}
	// HTTP fallback shares the same key and does not create a second row.
	raw, _ := json.Marshal(map[string]any{"content": body["content"], "request_id": key})
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/conversations/%d/messages", server.URL, ab.ConversationID), strings.NewReader(string(raw)))
	req.Header.Set("Authorization", "Bearer "+tokenA)
	req.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var rest struct {
		Message struct {
			ID int64 `json:"id"`
		} `json:"message"`
	}
	err = json.NewDecoder(response.Body).Decode(&rest)
	response.Body.Close()
	if err != nil || response.StatusCode != 201 || float64(rest.Message.ID) != messageID {
		t.Fatalf("REST retry: status=%d id=%d err=%v", response.StatusCode, rest.Message.ID, err)
	}
	body["content"] = "changed"
	write(a1, "send_message", key, body)
	if got := readEvent(t, a1); got["error"].(map[string]any)["code"] != "request_conflict" {
		t.Fatalf("conflict: %v", got)
	}
	write(c, "send_message", key, body)
	if got := readEvent(t, c); got["type"] != "error" || got["error"].(map[string]any)["code"] != "not_found" {
		t.Fatalf("outsider: %v", got)
	}
	for range 2 {
		write(b, "mark_read", key, map[string]any{"conversation_id": ab.ConversationID, "last_read_message_id": int64(messageID)})
		if got := readEvent(t, b); got["type"] != "read_updated" || got["data"].(map[string]any)["last_read_message_id"] != messageID {
			t.Fatalf("read marker: %v", got)
		}
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM messages WHERE conversation_id=$1", ab.ConversationID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("rows=%d err=%v", count, err)
	}
	// Only the first insertion fans out: unread sockets have no duplicate event.
	foreign, err := messages.Send(ctx, ac.ConversationID, ac.UserAID, "foreign")
	if err != nil {
		t.Fatal(err)
	}
	write(b, "mark_read", key, map[string]any{"conversation_id": ab.ConversationID, "last_read_message_id": foreign.ID})
	if got := readEvent(t, b); got["type"] != "error" {
		t.Fatalf("foreign marker accepted: %v", got)
	}
	write(c, "mark_read", key, map[string]any{"conversation_id": ab.ConversationID, "last_read_message_id": int64(messageID)})
	if got := readEvent(t, c); got["type"] != "error" || got["error"].(map[string]any)["code"] != "not_found" {
		t.Fatalf("outsider read: %v", got)
	}
	newer, err := messages.Send(ctx, ab.ConversationID, ab.UserAID, "newer")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []int64{newer.ID, int64(messageID)} {
		write(b, "mark_read", key, map[string]any{"conversation_id": ab.ConversationID, "last_read_message_id": marker})
		if got := readEvent(t, b); got["type"] != "read_updated" || got["data"].(map[string]any)["last_read_message_id"] != float64(newer.ID) {
			t.Fatalf("marker regressed: %v", got)
		}
	}
	var unread int
	if err := db.QueryRow("SELECT count(*) FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id WHERE cm.user_id=$1 AND m.conversation_id=$2 AND m.sender_id<>$1 AND m.id>cm.last_read_message_id", ab.UserBID, ab.ConversationID).Scan(&unread); err != nil || unread != 0 {
		t.Fatalf("unread=%d err=%v", unread, err)
	}
	for _, conn := range []*coderws.Conn{a2, c} {
		readCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		_, _, err := conn.Read(readCtx)
		cancel()
		if err == nil {
			t.Fatal("unexpected duplicate/unauthorized event")
		}
	}
	_ = a1.CloseNow()
	_ = b.CloseNow()
	stop, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := hub.Shutdown(stop); err != nil {
		t.Fatal(err)
	}
}
