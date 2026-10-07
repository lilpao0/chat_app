package websocket_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	presentationws "github.com/lilpao0/chat_app/backend/internal/presentation/websocket"
)

type mapVerifier map[string]domainauth.Identity

func (v mapVerifier) Verify(raw string) (domainauth.Identity, error) {
	identity, ok := v[raw]
	if !ok {
		return domainauth.Identity{}, domainauth.ErrInvalidToken
	}
	return identity, nil
}

type memberReaderFake struct {
	ids []int64
	err error
}

func (f memberReaderFake) MemberIDs(context.Context, int64) ([]int64, error) { return f.ids, f.err }

func realtimeServer(t *testing.T, hub *presentationws.Hub, verifier mapVerifier) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := presentationws.NewHandler(verifier, hub, nil, nil)
	router.GET("/ws", handler.Handle)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server
}

func connect(t *testing.T, server *httptest.Server, token string) *coderws.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	header := make(http.Header)
	header.Set("Authorization", "Bearer "+token)
	connection, response, err := coderws.Dial(ctx, websocketURL(server.URL)+"/ws", &coderws.DialOptions{HTTPHeader: header})
	if err != nil {
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		t.Fatalf("connect token %q: %v status=%d", token, err, status)
	}
	t.Cleanup(func() { _ = connection.CloseNow() })
	return connection
}

func waitForConnections(t *testing.T, hub *presentationws.Hub, userID int64, count int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if hub.ConnectionCount(userID) == count {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("user %d connection count=%d, want %d", userID, hub.ConnectionCount(userID), count)
}

func testLifecycleConfig() presentationws.LifecycleConfig {
	return presentationws.LifecycleConfig{SendQueueCapacity: 4, MaxMessageBytes: 1024, WriteTimeout: time.Second, PongTimeout: time.Second, PingInterval: time.Hour}
}

func readEvent(t *testing.T, connection *coderws.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, payload, err := connection.Read(ctx)
	if err != nil {
		t.Fatalf("read event: %v", err)
	}
	var event map[string]any
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatal(err)
	}
	return event
}

func TestRealtimePublisherDeliversToAllMemberDevices(t *testing.T) {
	expiry := time.Now().Add(time.Hour)
	hub := presentationws.NewHub(testLifecycleConfig())
	server := realtimeServer(t, hub, mapVerifier{"a": {UserID: 1, ExpiresAt: expiry}, "b": {UserID: 2, ExpiresAt: expiry}, "c": {UserID: 3, ExpiresAt: expiry}})
	a1, a2 := connect(t, server, "a"), connect(t, server, "a")
	b, c := connect(t, server, "b"), connect(t, server, "c")
	waitForConnections(t, hub, 1, 2)
	waitForConnections(t, hub, 2, 1)
	publisher := presentationws.NewPublisher(memberReaderFake{ids: []int64{1, 2}}, hub)
	message := entity.Message{ID: 10, ConversationID: 20, SenderID: 1, Content: "hello", CreatedAt: time.Now()}
	if err := publisher.PublishNewMessage(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	for _, connection := range []*coderws.Conn{a1, a2, b} {
		event := readEvent(t, connection)
		if event["type"] != "new_message" || event["data"].(map[string]any)["id"] != float64(10) {
			t.Fatalf("unexpected event: %+v", event)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, _, err := c.Read(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("outsider read error=%v, want deadline", err)
	}
}

func TestRealtimeInvalidCommandsAndExpiry(t *testing.T) {
	hub := presentationws.NewHub(testLifecycleConfig())
	server := realtimeServer(t, hub, mapVerifier{"input": {UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}, "short": {UserID: 2, ExpiresAt: time.Now().Add(150 * time.Millisecond)}})
	input := connect(t, server, "input")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := input.Write(ctx, coderws.MessageText, []byte("send a message")); err != nil {
		t.Fatal(err)
	}
	if event := readEvent(t, input); event["type"] != "error" || event["request_id"] != nil {
		t.Fatalf("invalid input response: %v", event)
	}
	short := connect(t, server, "short")
	if _, _, err := short.Read(ctx); coderws.CloseStatus(err) != coderws.StatusPolicyViolation {
		t.Fatalf("expiry close=%v status=%d", err, coderws.CloseStatus(err))
	}
}

func TestHubShutdownClosesConnections(t *testing.T) {
	hub := presentationws.NewHub(testLifecycleConfig())
	server := realtimeServer(t, hub, mapVerifier{"a": {UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}})
	connection := connect(t, server, "a")
	waitForConnections(t, hub, 1, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	readResult := make(chan error, 1)
	go func() {
		_, _, err := connection.Read(ctx)
		readResult <- err
	}()
	if err := hub.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-readResult; coderws.CloseStatus(err) != coderws.StatusGoingAway {
		t.Fatalf("shutdown close=%v status=%d", err, coderws.CloseStatus(err))
	}
	if err := hub.Publish([]int64{1}, []byte("event")); !errors.Is(err, presentationws.ErrHubClosed) {
		t.Fatalf("publish after shutdown=%v", err)
	}
}

func TestPublisherPropagatesRecipientLookupFailure(t *testing.T) {
	cause := errors.New("database unavailable")
	publisher := presentationws.NewPublisher(memberReaderFake{err: cause}, presentationws.NewHub(testLifecycleConfig()))
	if err := publisher.PublishNewMessage(context.Background(), entity.Message{ConversationID: 1}); !errors.Is(err, cause) {
		t.Fatalf("error=%v", err)
	}
}
