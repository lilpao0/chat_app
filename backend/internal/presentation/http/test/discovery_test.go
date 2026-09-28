package http_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
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
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/user"
	presentation "github.com/lilpao0/chat_app/backend/internal/presentation/http"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/handler"
	"github.com/lilpao0/chat_app/backend/internal/testutil"
)

func TestDiscoverAndOpenDirect(t *testing.T) {
	db := testutil.Database(t)
	users := datarepo.NewPostgresUserRepository(db)
	create := func(name, email string) int64 {
		t.Helper()
		created, err := users.Create(t.Context(), repository.CreateUser{FirstName: name, LastName: "", Email: email, PasswordHash: "fixture-hash"})
		if err != nil {
			t.Fatal(err)
		}
		return created.ID
	}
	a := create("An A", "a@example.test")
	b := create("An B", "b@example.test")
	c := create("An C", "c@example.test")
	d := create("An D", "d@example.test")
	_ = create("Other", "other@example.test")
	tokens, err := dataauth.NewJWT(strings.Repeat("s", 32), "test", "test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	r, protected := presentation.NewRouter(domainauth.NewRegister(users, dataauth.BcryptPasswordHasher{}), domainauth.NewLogin(users, dataauth.BcryptPasswordHasher{}, tokens), domainauth.NewRefresh(tokens), tokens)
	conversations := datarepo.NewPostgresConversationRepository(db)
	discovery := handler.NewDiscoveryHandler(user.NewSearch(users), conversation.NewOpenDirect(conversations))
	protected.GET("/users", discovery.Search)
	protected.POST("/conversations/direct", discovery.OpenDirect)
	protected.GET("/conversations", handler.NewConversationsHandler(conversation.NewList(conversations)).List)
	protected.POST("/conversations/:id/messages", handler.NewSendMessageHandler(message.NewSend(datarepo.NewPostgresMessageRepository(db))).Send)
	call := func(actor int64, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if actor > 0 {
			token, err := tokens.Issue(actor)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token.Value)
		}
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		return out
	}
	var first struct {
		Items []struct {
			ID        int64  `json:"id"`
			Name      string `json:"name"`
			AvatarURL string `json:"avatar_url"`
		} `json:"items"`
		HasMore bool   `json:"has_more"`
		Next    *int64 `json:"next_after_id"`
	}
	response := call(a, "GET", "/api/users?q=an&limit=1", "")
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &first) != nil || len(first.Items) != 1 || first.Items[0].ID != b || !first.HasMore || first.Next == nil || *first.Next != b {
		t.Fatalf("first search page: %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "email") || strings.Contains(response.Body.String(), "password") {
		t.Fatal("search leaked private user data")
	}
	response = call(a, "GET", fmt.Sprintf("/api/users?q=an&limit=1&after_id=%d", b), "")
	var second struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
		Next *int64 `json:"next_after_id"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &second) != nil || len(second.Items) != 1 || second.Items[0].ID != c || second.Next == nil || *second.Next != c {
		t.Fatal("second search page incorrect")
	}
	response = call(a, "GET", fmt.Sprintf("/api/users?q=an&limit=1&after_id=%d", c), "")
	var third struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
		HasMore bool   `json:"has_more"`
		Next    *int64 `json:"next_after_id"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &third) != nil || len(third.Items) != 1 || third.Items[0].ID != d || third.HasMore || third.Next != nil {
		t.Fatal("last search page incorrect")
	}
	response = call(a, "GET", "/api/users?q="+url.QueryEscape("B@EXAMPLE.TEST"), "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), fmt.Sprintf(`"id":%d`, b)) {
		t.Fatal("exact case-insensitive email search failed")
	}
	if out := call(a, "GET", "/api/users?q=%25", ""); out.Code != 400 {
		t.Fatal("single-character wildcard accepted")
	}
	if out := call(a, "GET", "/api/users?q=%25%25", ""); out.Code != 200 || !strings.Contains(out.Body.String(), `"items":[]`) {
		t.Fatal("wildcard was interpreted as a pattern")
	}
	for _, path := range []string{"/api/users", "/api/users?q=a", "/api/users?q=an&limit=0", "/api/users?q=an&limit=51", "/api/users?q=an&after_id=0", "/api/users?q=an&limit=1&limit=2", "/api/users?q=%ZZ", "/api/users?q=an&user_id=1"} {
		if out := call(a, "GET", path, ""); out.Code != 400 {
			t.Fatalf("invalid search accepted: %s", path)
		}
	}
	if call(0, "GET", "/api/users?q=an", "").Code != 401 {
		t.Fatal("anonymous search accepted")
	}
	for _, body := range []string{`{"user_id":0}`, fmt.Sprintf(`{"user_id":%d}`, a), `{"user_id":999999}`, `{"user_id":2,"actor_id":1}`, `{"user_id":"2"}`} {
		out := call(a, "POST", "/api/conversations/direct", body)
		want := 400
		if body == `{"user_id":999999}` {
			want = 404
		}
		if out.Code != want {
			t.Fatalf("invalid target status %d for %s", out.Code, body)
		}
	}
	if call(0, "POST", "/api/conversations/direct", fmt.Sprintf(`{"user_id":%d}`, b)).Code != 401 {
		t.Fatal("anonymous direct chat accepted")
	}
	var wg sync.WaitGroup
	type opened struct {
		ID          int64
		Counterpart int64
		Expected    int64
		Status      int
	}
	results := make(chan opened, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			actor, target := a, b
			if i%2 == 1 {
				actor, target = b, a
			}
			out := call(actor, "POST", "/api/conversations/direct", fmt.Sprintf(`{"user_id":%d}`, target))
			var payload struct {
				Conversation struct {
					ID   int64 `json:"id"`
					User struct {
						ID int64 `json:"id"`
					} `json:"user"`
				} `json:"conversation"`
			}
			if json.Unmarshal(out.Body.Bytes(), &payload) != nil {
				results <- opened{Status: out.Code, Expected: target}
				return
			}
			results <- opened{ID: payload.Conversation.ID, Counterpart: payload.Conversation.User.ID, Expected: target, Status: out.Code}
		}(i)
	}
	wg.Wait()
	close(results)
	var conversationID int64
	created := 0
	for result := range results {
		if result.Status == 201 {
			created++
		} else if result.Status != 200 {
			t.Fatalf("concurrent open status %d", result.Status)
		}
		if conversationID == 0 {
			conversationID = result.ID
		}
		if result.ID != conversationID || result.Counterpart != result.Expected {
			t.Fatal("direct open returned different conversation or counterpart")
		}
	}
	if created != 1 {
		t.Fatalf("expected one new conversation, got %d", created)
	}
	var count, members int
	if err := db.QueryRow(`SELECT (SELECT count(*) FROM conversations),(SELECT count(*) FROM conversation_members)`).Scan(&count, &members); err != nil || count != 1 || members != 2 {
		t.Fatal("concurrent open created duplicates")
	}
	// The seed and user-driven flow share the same pair lookup.
	seeded, err := seed.Demo(t.Context(), db, repository.CreateUser{FirstName: "ignored", LastName: "", Email: "a@example.test", PasswordHash: "unused"}, repository.CreateUser{FirstName: "ignored", LastName: "", Email: "b@example.test", PasswordHash: "unused"})
	if err != nil || seeded.ConversationID != conversationID {
		t.Fatal("seed duplicated user-created chat")
	}
	other := call(a, "POST", "/api/conversations/direct", fmt.Sprintf(`{"user_id":%d}`, c))
	if other.Code != 201 {
		t.Fatal("new pair did not create chat")
	}
	if out := call(a, "POST", fmt.Sprintf("/api/conversations/%d/messages", conversationID), `{"content":"hello"}`); out.Code != 201 {
		t.Fatalf("opened chat cannot send: %d", out.Code)
	}
	if out := call(a, "GET", "/api/conversations", ""); out.Code != 200 || !strings.Contains(out.Body.String(), "hello") {
		t.Fatal("new chat not visible in list")
	}
	if out := call(c, "GET", "/api/conversations", ""); out.Code != 200 || strings.Contains(out.Body.String(), "hello") {
		t.Fatal("unrelated chat leaked")
	}
}
