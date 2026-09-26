package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	dataauth "github.com/lilpao0/chat_app/backend/internal/data/auth"
	datarepo "github.com/lilpao0/chat_app/backend/internal/data/repository"
	"github.com/lilpao0/chat_app/backend/internal/data/seed"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/conversation"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/message"
	presentation "github.com/lilpao0/chat_app/backend/internal/presentation/http"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/handler"
	"github.com/lilpao0/chat_app/backend/internal/testutil"
)

type messageResponse struct {
	ID             int64  `json:"id"`
	ConversationID int64  `json:"conversation_id"`
	SenderID       int64  `json:"sender_id"`
	Content        string `json:"content"`
}
type pageResponse struct {
	Items   []messageResponse `json:"items"`
	HasMore bool              `json:"has_more"`
	Before  *int64            `json:"next_before_id"`
	After   *int64            `json:"next_after_id"`
}

func TestRESTChatFlow(t *testing.T) {
	db := testutil.Database(t)
	ctx := context.Background()
	a := repository.CreateUser{FirstName: "A", LastName: "", Email: "a@example.test", PasswordHash: "fixture-hash"}
	b := repository.CreateUser{FirstName: "B", LastName: "", Email: "b@example.test", PasswordHash: "fixture-hash"}
	c := repository.CreateUser{FirstName: "C", LastName: "", Email: "c@example.test", PasswordHash: "fixture-hash"}
	ab, err := seed.Demo(ctx, db, a, b)
	if err != nil {
		t.Fatal(err)
	}
	ac, err := seed.Demo(ctx, db, a, c)
	if err != nil {
		t.Fatal(err)
	}
	users := datarepo.NewPostgresUserRepository(db)
	tokens, err := dataauth.NewJWT(strings.Repeat("s", 32), "test", "test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	r, protected := presentation.NewRouter(domainauth.NewRegister(users, dataauth.BcryptPasswordHasher{}), domainauth.NewLogin(users, dataauth.BcryptPasswordHasher{}, tokens), domainauth.NewRefresh(tokens), tokens)
	messages := datarepo.NewPostgresMessageRepository(db)
	conversations := datarepo.NewPostgresConversationRepository(db)
	protected.GET("/conversations", handler.NewConversationsHandler(conversation.NewList(conversations)).List)
	protected.POST("/conversations/:id/messages", handler.NewSendMessageHandler(message.NewSend(messages)).Send)
	protected.GET("/conversations/:id/messages", handler.NewHistoryHandler(message.NewHistory(messages)).Get)
	protected.POST("/conversations/:id/read", handler.NewReadHandler(conversation.NewMarkRead(messages)).Mark)
	call := func(user int64, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if user > 0 {
			token, err := tokens.Issue(user)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token.Value)
		}
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		return out
	}
	path := fmt.Sprintf("/api/conversations/%d", ab.ConversationID)
	send := func(user int64, base, content string) messageResponse {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"content": content})
		out := call(user, "POST", base+"/messages", string(body))
		if out.Code != 201 {
			t.Fatalf("send status %d: %s", out.Code, out.Body.String())
		}
		var result struct {
			Message messageResponse `json:"message"`
		}
		if err := json.Unmarshal(out.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Message
	}
	history := func(suffix string) pageResponse {
		t.Helper()
		out := call(ab.UserBID, "GET", path+"/messages"+suffix, "")
		if out.Code != 200 {
			t.Fatalf("history status %d", out.Code)
		}
		var page pageResponse
		if err := json.Unmarshal(out.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		return page
	}
	unread := func() int64 {
		t.Helper()
		items, err := conversations.ListForUser(ctx, ab.UserBID)
		if err != nil || len(items) != 1 {
			t.Fatal("missing list")
		}
		return items[0].UnreadCount
	}
	read := func(id int64) int64 {
		t.Helper()
		out := call(ab.UserBID, "POST", path+"/read", fmt.Sprintf(`{"last_read_message_id":%d}`, id))
		if out.Code != 200 {
			t.Fatalf("read status %d", out.Code)
		}
		var result struct {
			Marker int64 `json:"last_read_message_id"`
		}
		if json.Unmarshal(out.Body.Bytes(), &result) != nil {
			t.Fatal("invalid read response")
		}
		return result.Marker
	}
	if page := history(""); page.Items == nil || len(page.Items) != 0 || page.HasMore {
		t.Fatal("empty history contract")
	}
	m1 := send(ab.UserAID, path, " first \n")
	m2 := send(ab.UserAID, path, "second")
	if m1.Content != " first \n" || m1.SenderID != ab.UserAID || m1.ConversationID != ab.ConversationID || m2.ID <= m1.ID {
		t.Fatal("send identity/content/order incorrect")
	}
	if unread() != 2 {
		t.Fatal("new messages not unread")
	}
	latest := history("?limit=1")
	if len(latest.Items) != 1 || latest.Items[0].ID != m2.ID || !latest.HasMore || latest.Before == nil || *latest.Before != m2.ID || latest.After != nil {
		t.Fatal("initial page incorrect")
	}
	older := history(fmt.Sprintf("?limit=1&before_id=%d", m2.ID))
	if len(older.Items) != 1 || older.Items[0].ID != m1.ID || older.HasMore || older.Before != nil {
		t.Fatal("older page duplicates/omits boundary")
	}
	m3 := send(ab.UserAID, path, "third")
	newer := history(fmt.Sprintf("?limit=1&after_id=%d", m1.ID))
	if len(newer.Items) != 1 || newer.Items[0].ID != m2.ID || !newer.HasMore || newer.After == nil || *newer.After != m2.ID {
		t.Fatal("ascending catch-up page incorrect")
	}
	caughtUp := history(fmt.Sprintf("?after_id=%d", m2.ID))
	if len(caughtUp.Items) != 1 || caughtUp.Items[0].ID != m3.ID || caughtUp.HasMore || caughtUp.After != nil {
		t.Fatal("catch-up missed newly inserted message")
	}
	if read(m2.ID) != m2.ID || unread() != 1 {
		t.Fatal("read included unseen newer message")
	}
	var readTime time.Time
	if err := db.QueryRow(`SELECT last_read_at FROM conversation_members WHERE conversation_id=$1 AND user_id=$2`, ab.ConversationID, ab.UserBID).Scan(&readTime); err != nil {
		t.Fatal(err)
	}
	if read(m1.ID) != m2.ID || unread() != 1 {
		t.Fatal("old request regressed marker")
	}
	var repeated time.Time
	if err := db.QueryRow(`SELECT last_read_at FROM conversation_members WHERE conversation_id=$1 AND user_id=$2`, ab.ConversationID, ab.UserBID).Scan(&repeated); err != nil || !repeated.Equal(readTime) {
		t.Fatal("old request changed read timestamp")
	}
	var aUnreadMarker bool
	if err := db.QueryRow(`SELECT last_read_message_id IS NULL FROM conversation_members WHERE conversation_id=$1 AND user_id=$2`, ab.ConversationID, ab.UserAID).Scan(&aUnreadMarker); err != nil || !aUnreadMarker {
		t.Fatal("another user's marker changed")
	}
	foreign := send(ac.UserAID, fmt.Sprintf("/api/conversations/%d", ac.ConversationID), "private AC")
	page := history(fmt.Sprintf("?before_id=%d", foreign.ID))
	if len(page.Items) != 3 {
		t.Fatal("foreign numeric boundary should be allowed")
	}
	for _, m := range page.Items {
		if m.ConversationID != ab.ConversationID {
			t.Fatal("foreign message leaked")
		}
	}
	if out := call(ab.UserBID, "POST", path+"/read", fmt.Sprintf(`{"last_read_message_id":%d}`, foreign.ID)); out.Code != 400 {
		t.Fatal("foreign read marker accepted")
	}
	for _, tc := range []struct{ method, suffix, body string }{{"GET", "/messages", ""}, {"POST", "/messages", `{"content":"intrusion"}`}, {"POST", "/read", fmt.Sprintf(`{"last_read_message_id":%d}`, m1.ID)}} {
		if out := call(ac.UserBID, tc.method, path+tc.suffix, tc.body); out.Code != 404 {
			t.Fatal("non-member accessed AB")
		}
	}
	for _, query := range []string{"?limit=0", "?limit=101", "?limit=x", "?before_id=0", "?after_id=-1", "?before_id=1&after_id=2", "?limit=1&limit=2", "?limit=%ZZ"} {
		if out := call(ab.UserBID, "GET", path+"/messages"+query, ""); out.Code != 400 {
			t.Fatalf("invalid query accepted: %s", query)
		}
	}
	for _, body := range []string{`{"content":""}`, `{"content":"hi","sender_id":999}`, `{"content":123}`} {
		if out := call(ab.UserAID, "POST", path+"/messages", body); out.Code != 400 {
			t.Fatal("invalid send accepted")
		}
	}
	if call(0, "POST", path+"/messages", `{"content":"hi"}`).Code != 401 {
		t.Fatal("anonymous sender accepted")
	}
	if call(ab.UserAID, "POST", "/api/conversations/0/messages", `{"content":"hi"}`).Code != 400 {
		t.Fatal("invalid path accepted")
	}
	// Concurrent devices can only advance the position.
	var wg sync.WaitGroup
	failures := make(chan error, 3)
	for _, marker := range []int64{m1.ID, m3.ID, m2.ID} {
		wg.Add(1)
		go func(marker int64) {
			defer wg.Done()
			_, err := messages.MarkRead(ctx, ab.ConversationID, ab.UserBID, marker)
			failures <- err
		}(marker)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if read(m1.ID) != m3.ID || unread() != 0 {
		t.Fatal("concurrent marker regressed")
	}
	send(ab.UserBID, path, "my own reply")
	if unread() != 0 {
		t.Fatal("self message became unread")
	}
}
