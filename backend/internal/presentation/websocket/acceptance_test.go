package websocket_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	dataauth "github.com/lilpao0/chat_app/backend/internal/data/auth"
	datarepo "github.com/lilpao0/chat_app/backend/internal/data/repository"
	"github.com/lilpao0/chat_app/backend/internal/data/seed"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/message"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/handler"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/middleware"
	presentationws "github.com/lilpao0/chat_app/backend/internal/presentation/websocket"
	"github.com/lilpao0/chat_app/backend/internal/testutil"
)

func TestRealtimeRESTAcceptance(t *testing.T) {
	db := testutil.Database(t)
	ctx := context.Background()
	ab, err := seed.Demo(ctx, db,
		repository.CreateUser{FirstName: "A", Email: "ws-a@example.test", PasswordHash: "fixture-hash"},
		repository.CreateUser{FirstName: "B", Email: "ws-b@example.test", PasswordHash: "fixture-hash"},
	)
	if err != nil {
		t.Fatal(err)
	}
	ac, err := seed.Demo(ctx, db,
		repository.CreateUser{FirstName: "A", Email: "ws-a@example.test", PasswordHash: "fixture-hash"},
		repository.CreateUser{FirstName: "C", Email: "ws-c@example.test", PasswordHash: "fixture-hash"},
	)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := dataauth.NewJWT(strings.Repeat("w", 32), "ws-test", "ws-test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	hub := presentationws.NewHub(testLifecycleConfig())
	conversations := datarepo.NewPostgresConversationRepository(db)
	messages := datarepo.NewPostgresMessageRepository(db)
	publisher := presentationws.NewPublisher(conversations, hub)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ws", presentationws.NewHandler(tokens, hub).Handle)
	protected := router.Group("/api", middleware.Authenticate(tokens))
	protected.POST("/conversations/:id/messages", handler.NewSendMessageHandler(message.NewSendWithPublisher(messages, publisher, func(err error) { t.Errorf("publish: %v", err) })).Send)
	protected.GET("/conversations/:id/messages", handler.NewHistoryHandler(message.NewHistory(messages)).Get)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	issue := func(userID int64) string {
		t.Helper()
		token, err := tokens.Issue(userID)
		if err != nil {
			t.Fatal(err)
		}
		return token.Value
	}
	tokenA, tokenB, tokenC := issue(ab.UserAID), issue(ab.UserBID), issue(ac.UserBID)
	connectToken := func(token string) *coderws.Conn {
		t.Helper()
		header := make(http.Header)
		header.Set("Authorization", "Bearer "+token)
		connectCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		connection, _, err := coderws.Dial(connectCtx, websocketURL(server.URL)+"/ws", &coderws.DialOptions{HTTPHeader: header})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = connection.CloseNow() })
		return connection
	}
	a, b, c := connectToken(tokenA), connectToken(tokenB), connectToken(tokenC)
	waitForConnections(t, hub, ab.UserAID, 1)
	waitForConnections(t, hub, ab.UserBID, 1)
	send := func(token, content string) int64 {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/conversations/%d/messages", server.URL, ab.ConversationID), strings.NewReader(fmt.Sprintf(`{"content":%q}`, content)))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("send status=%d", response.StatusCode)
		}
		var body struct {
			Message struct {
				ID int64 `json:"id"`
			} `json:"message"`
		}
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		return body.Message.ID
	}
	firstID := send(tokenA, "first realtime")
	for _, connection := range []*coderws.Conn{a, b} {
		event := readEvent(t, connection)
		if event["data"].(map[string]any)["id"] != float64(firstID) {
			t.Fatalf("wrong message event: %+v", event)
		}
	}
	noEventCtx, noEventCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	if _, _, err := c.Read(noEventCtx); err == nil {
		t.Fatal("outsider received AB event")
	}
	noEventCancel()
	_ = b.Close(coderws.StatusNormalClosure, "offline")
	waitForConnections(t, hub, ab.UserBID, 0)
	secondID := send(tokenA, "missed while offline")
	_ = readEvent(t, a)
	b = connectToken(tokenB)
	waitForConnections(t, hub, ab.UserBID, 1)
	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/conversations/%d/messages?after_id=%d", server.URL, ab.ConversationID, firstID), nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+tokenB)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var page struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || len(page.Items) != 1 || page.Items[0].ID != secondID {
		t.Fatalf("catch-up status=%d page=%+v", response.StatusCode, page)
	}
}
