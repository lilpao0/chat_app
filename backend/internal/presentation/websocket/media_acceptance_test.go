package websocket_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	dataauth "github.com/lilpao0/chat_app/backend/internal/data/auth"
	datarepo "github.com/lilpao0/chat_app/backend/internal/data/repository"
	"github.com/lilpao0/chat_app/backend/internal/data/seed"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/attachment"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/message"
	presentation "github.com/lilpao0/chat_app/backend/internal/presentation/http"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/handler"
	ws "github.com/lilpao0/chat_app/backend/internal/presentation/websocket"
	"github.com/lilpao0/chat_app/backend/internal/testutil"
)

// Only the external storage boundary is replaced; auth, SQL, handlers and WS are real.
type acceptanceStorage struct {
	mu               sync.Mutex
	inspectErr       error
	signedGeneration int64
}

func (*acceptanceStorage) StartUpload(context.Context, string, string, int64, string) (string, error) {
	return "https://storage.googleapis.com/upload/storage/v1/b/test/o?upload_id=fixture", nil
}
func (*acceptanceStorage) CancelUpload(context.Context, string) error { return nil }
func (s *acceptanceStorage) Inspect(_ context.Context, _ string, typ string, size int64) (entity.VerifiedMedia, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kind := "image"
	if typ == "video/mp4" {
		kind = "video"
	}
	return entity.VerifiedMedia{Kind: kind, ContentType: typ, SizeBytes: size, Generation: 7}, s.inspectErr
}
func (s *acceptanceStorage) SignAccess(_ context.Context, _ string, generation int64) (string, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signedGeneration = generation
	return "https://storage.googleapis.com/test/fixture?generation=7", time.Now().Add(15 * time.Minute), nil
}
func (*acceptanceStorage) DeleteObject(context.Context, string, int64) error { return nil }
func (s *acceptanceStorage) setInspectError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inspectErr = err
}

func TestMediaRESTAndWebSocketAcceptance(t *testing.T) {
	db := testutil.Database(t)
	ctx := context.Background()
	owner := repository.CreateUser{FirstName: "A", Email: "media-ws-a@test.local", PasswordHash: "fixture"}
	ab, err := seed.Demo(ctx, db, owner, repository.CreateUser{FirstName: "B", Email: "media-ws-b@test.local", PasswordHash: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	ac, err := seed.Demo(ctx, db, owner, repository.CreateUser{FirstName: "C", Email: "media-ws-c@test.local", PasswordHash: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := dataauth.NewJWT(strings.Repeat("s", 32), "test", "test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	files, err := datarepo.NewPostgresMediaRepository(db, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	storage := &acceptanceStorage{}
	media := attachment.NewMedia(files, storage)
	commands := &ws.Commands{}
	hub := ws.NewHub(testLifecycleConfig(), commands)
	messages := datarepo.NewPostgresMessageRepository(db)
	send := message.NewSendWithPublisher(messages, ws.NewPublisher(datarepo.NewPostgresConversationRepository(db), hub), func(err error) { t.Errorf("publish: %v", err) })
	commands.Send = send
	gin.SetMode(gin.TestMode)
	r, protected := presentation.NewRouter(nil, nil, nil, tokens, nil)
	presentation.RegisterWebSocketRoute(r, ws.NewHandler(tokens, hub, nil, nil).Handle)
	h := handler.NewAttachmentHandler(media)
	protected.POST("/conversations/:id/attachments", h.Create)
	protected.POST("/attachments/:id/complete", h.Complete)
	protected.GET("/attachments/:id/access", h.Access)
	protected.POST("/conversations/:id/messages", handler.NewSendMessageHandler(send).Send)
	protected.GET("/conversations/:id/messages", handler.NewHistoryHandler(message.NewHistory(messages)).Get)
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
	a, b := connect(t, server, tokenA), connect(t, server, tokenB)
	waitForConnections(t, hub, ab.UserAID, 1)
	waitForConnections(t, hub, ab.UserBID, 1)
	base := fmt.Sprintf("/api/conversations/%d", ab.ConversationID)
	call := func(token, method, path, body string, status int) map[string]any {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		if out.Code != status {
			t.Fatalf("%s %s status=%d want=%d body=%s", method, path, out.Code, status, out.Body.String())
		}
		if strings.Contains(path, "/attachments") && out.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("media response cacheable")
		}
		var result map[string]any
		if err := json.Unmarshal(out.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	grant := func(typ string) int64 {
		out := call(tokenA, "POST", base+"/attachments", fmt.Sprintf(`{"content_type":%q,"size_bytes":100}`, typ), 201)
		return int64(out["data"].(map[string]any)["id"].(float64))
	}
	completePath := func(id int64) string { return fmt.Sprintf("/api/attachments/%d/complete", id) }
	accessPath := func(id int64) string { return fmt.Sprintf("/api/attachments/%d/access", id) }
	image, video := grant("image/jpeg"), grant("video/mp4")
	storage.setInspectError(entity.ErrAttachmentNotReady)
	call(tokenA, "POST", completePath(image), `{}`, 409)
	call(tokenB, "POST", completePath(image), `{}`, 404)
	call(tokenC, "POST", base+"/attachments", `{"content_type":"image/png","size_bytes":100}`, 404)
	call(tokenA, "POST", base+"/messages", fmt.Sprintf(`{"attachment_ids":[%d]}`, image), 409)
	storage.setInspectError(nil)
	call(tokenA, "POST", completePath(image), `{}`, 200)
	call(tokenA, "POST", completePath(video), `{}`, 200)
	call(tokenB, "GET", accessPath(image), "", 404)
	call(tokenA, "GET", accessPath(image), "", 200)
	invalid := grant("image/png")
	storage.setInspectError(entity.ErrInvalidMedia)
	call(tokenA, "POST", completePath(invalid), `{}`, 422)
	storage.setInspectError(nil)
	call(tokenA, "POST", completePath(invalid), `{}`, 409)
	for _, body := range []string{`{"content":null}`, `{"attachment_ids":null}`, `{"attachment_ids":[1,1]}`, `{"content":"hi","sender_id":999}`, `{}`} {
		call(tokenA, "POST", base+"/messages", body, 400)
	}
	call(tokenA, "POST", completePath(image), `{"state":"ready"}`, 400)
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM messages`).Scan(&count); err != nil || count != 0 {
		t.Fatal("upload/invalid send created a message")
	}
	chats, err := datarepo.NewPostgresConversationRepository(db).ListForUser(ctx, ab.UserBID)
	if err != nil || chats[0].UnreadCount != 0 || chats[0].LastMessage != nil {
		t.Fatal("upload changed unread/preview")
	}
	const key = "550e8400-e29b-41d4-a716-446655440000"
	write := func(data any) {
		raw, _ := json.Marshal(map[string]any{"type": "send_message", "request_id": key, "data": data})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := a.Write(ctx, coderws.MessageText, raw); err != nil {
			t.Fatal(err)
		}
	}
	data := map[string]any{"conversation_id": ab.ConversationID, "attachment_ids": []int64{video, image}}
	write(data)
	var saved map[string]any
	seen := map[string]bool{}
	for range 2 {
		event := readEvent(t, a)
		typ := event["type"].(string)
		seen[typ] = true
		if event["data"] == nil {
			t.Fatalf("unexpected frame: %v", event)
		}
		got := event["data"].(map[string]any)
		if saved == nil {
			saved = got
		} else if !reflect.DeepEqual(saved, got) {
			t.Fatal("ack and event differ")
		}
	}
	if !seen["message_sent"] || !seen["new_message"] {
		t.Fatal("missing ack/event")
	}
	if got := readEvent(t, b); got["type"] != "new_message" || !reflect.DeepEqual(got["data"], saved) {
		t.Fatal("recipient data differs or upload emitted event")
	}
	ordered := saved["attachments"].([]any)
	if len(ordered) != 2 || ordered[0].(map[string]any)["id"] != float64(video) || saved["content"] != "" {
		t.Fatal("media-only order/content changed")
	}
	if bytes.Contains(mustJSON(t, saved), []byte("storage.googleapis.com")) {
		t.Fatal("capability leaked in message")
	}
	write(data)
	if got := readEvent(t, a); got["type"] != "message_sent" || !reflect.DeepEqual(got["data"], saved) {
		t.Fatal("WS retry changed message")
	}
	body := fmt.Sprintf(`{"attachment_ids":[%d,%d],"request_id":%q}`, video, image, key)
	if got := call(tokenA, "POST", base+"/messages", body, 201); !reflect.DeepEqual(got["data"], saved) {
		t.Fatal("REST retry differs")
	}
	call(tokenA, "POST", base+"/messages", fmt.Sprintf(`{"attachment_ids":[%d,%d],"request_id":%q}`, image, video, key), 409)
	call(tokenB, "GET", accessPath(image), "", 200)
	call(tokenC, "GET", accessPath(image), "", 404)
	completed := call(tokenA, "POST", completePath(image), `{}`, 200)["data"].(map[string]any)
	if completed["state"] != "attached" || completed["expires_at"] != nil {
		t.Fatal("complete reset attached state")
	}
	text := call(tokenA, "POST", base+"/messages", `{"content":"next"}`, 201)["data"].(map[string]any)
	if len(text["attachments"].([]any)) != 0 {
		t.Fatal("text attachments not empty array")
	}
	for _, conn := range []*coderws.Conn{a, b} {
		if got := readEvent(t, conn); !reflect.DeepEqual(got["data"], text) {
			t.Fatal("retry republished old event")
		}
	}
	page := call(tokenB, "GET", base+"/messages?limit=1&before_id="+fmt.Sprint(int64(text["id"].(float64))), "", 200)["data"].([]any)
	if len(page) != 1 || !reflect.DeepEqual(page[0], saved) {
		t.Fatal("before history metadata differs")
	}
	page = call(tokenB, "GET", base+"/messages?after_id="+fmt.Sprint(int64(saved["id"].(float64))), "", 200)["data"].([]any)
	if len(page) != 1 || !reflect.DeepEqual(page[0], text) {
		t.Fatal("after history lost text shape")
	}
	chats, err = datarepo.NewPostgresConversationRepository(db).ListForUser(ctx, ab.UserBID)
	if err != nil || chats[0].UnreadCount != 2 {
		t.Fatal("unread counted per file")
	}
	storage.mu.Lock()
	generation := storage.signedGeneration
	storage.mu.Unlock()
	if generation != 7 {
		t.Fatal("access not pinned to verified generation")
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
