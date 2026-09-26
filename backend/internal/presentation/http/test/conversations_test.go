package http_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	dataauth "github.com/lilpao0/chat_app/backend/internal/data/auth"
	datarepo "github.com/lilpao0/chat_app/backend/internal/data/repository"
	"github.com/lilpao0/chat_app/backend/internal/data/seed"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/conversation"
	presentation "github.com/lilpao0/chat_app/backend/internal/presentation/http"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/handler"
	"github.com/lilpao0/chat_app/backend/internal/testutil"
)

func TestConversationList(t *testing.T) {
	db := testutil.Database(t)
	ctx := context.Background()
	passwords := dataauth.BcryptPasswordHasher{}
	hash, err := passwords.Hash("demo-password")
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := seed.Demo(ctx, db, repository.CreateUser{FirstName: "A", LastName: "", Email: "a@example.test", PasswordHash: hash}, repository.CreateUser{FirstName: "B", LastName: "", Email: "b@example.test", PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	users := datarepo.NewPostgresUserRepository(db)
	outsider, err := users.Create(ctx, repository.CreateUser{FirstName: "C", LastName: "", Email: "c@example.test", PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := dataauth.NewJWT(strings.Repeat("s", 32), "test", "test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	r, protected := presentation.NewRouter(domainauth.NewRegister(users, passwords), domainauth.NewLogin(users, passwords, tokens), domainauth.NewRefresh(tokens), tokens)
	repo := datarepo.NewPostgresConversationRepository(db)
	protected.GET("/conversations", handler.NewConversationsHandler(conversation.NewList(repo)).List)
	get := func(userID int64) *httptest.ResponseRecorder {
		token, err := tokens.Issue(userID)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("GET", "/api/conversations?user_id=999", nil)
		req.Header.Set("Authorization", "Bearer "+token.Value)
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		return out
	}
	a := get(fixture.UserAID)
	if a.Code != 200 || !strings.Contains(a.Body.String(), `"last_message":null`) || !strings.Contains(a.Body.String(), `"unread_count":0`) || strings.Contains(a.Body.String(), "email") {
		t.Fatal("empty conversation DTO incorrect")
	}
	c := get(outsider.ID)
	if c.Code != 200 || c.Body.String() != "[]" {
		t.Fatal("outsider saw conversation")
	}
	// Fixture writes use the same lock-before-ID rule as the future send repository.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`SELECT id FROM conversations WHERE id=$1 FOR UPDATE`, fixture.ConversationID); err != nil {
		t.Fatal(err)
	}
	var first, last int64
	if err := tx.QueryRow(`INSERT INTO messages(conversation_id,sender_id,content) VALUES ($1,$2,'hello') RETURNING id`, fixture.ConversationID, fixture.UserAID).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(`INSERT INTO messages(conversation_id,sender_id,content) VALUES ($1,$2,'reply') RETURNING id`, fixture.ConversationID, fixture.UserBID).Scan(&last); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, userID := range []int64{fixture.UserAID, fixture.UserBID} {
		items, err := repo.ListForUser(ctx, userID)
		if err != nil || len(items) != 1 || items[0].UnreadCount != 1 || items[0].LastMessage == nil || *items[0].LastMessage != "reply" {
			t.Fatal("last message/unread incorrect")
		}
	}
	if _, err := db.Exec(`UPDATE conversation_members SET last_read_message_id=$1,last_read_at=clock_timestamp() WHERE conversation_id=$2 AND user_id=$3`, first, fixture.ConversationID, fixture.UserBID); err != nil {
		t.Fatal(err)
	}
	items, err := repo.ListForUser(ctx, fixture.UserBID)
	if err != nil || items[0].UnreadCount != 0 {
		t.Fatal("self-sent message counted unread")
	}
	b := get(fixture.UserBID)
	var result []struct {
		ID   int64 `json:"id"`
		User struct {
			ID int64 `json:"id"`
		} `json:"user"`
	}
	if json.Unmarshal(b.Body.Bytes(), &result) != nil || len(result) != 1 || result[0].User.ID != fixture.UserAID || result[0].ID != fixture.ConversationID {
		t.Fatal("wrong counterpart")
	}
	req := httptest.NewRequest("GET", "/api/conversations", nil)
	out := httptest.NewRecorder()
	r.ServeHTTP(out, req)
	if out.Code != 401 {
		t.Fatal("anonymous request accepted")
	}
}
