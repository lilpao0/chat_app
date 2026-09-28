package websocket_test

import (
	"context"
	"fmt"
	coderws "github.com/coder/websocket"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	auth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	ws "github.com/lilpao0/chat_app/backend/internal/presentation/websocket"
	"strings"
	"testing"
	"time"
)

type blockingSender struct {
	started  chan struct{}
	canceled chan struct{}
}

func (b *blockingSender) ExecuteWithRequestID(ctx context.Context, _, _ int64, _, _ string) (entity.Message, error) {
	close(b.started)
	<-ctx.Done()
	close(b.canceled)
	return entity.Message{}, ctx.Err()
}

const validCommand = `{"type":"send_message","request_id":"550e8400-e29b-41d4-a716-446655440000","data":{"conversation_id":1,"content":"hi"}}`

func TestCommandTimeout(t *testing.T) {
	sender := &blockingSender{make(chan struct{}), make(chan struct{})}
	cfg := testLifecycleConfig()
	cfg.CommandTimeout = 30 * time.Millisecond
	hub := ws.NewHub(cfg, &ws.Commands{Send: sender})
	server := realtimeServer(t, hub, mapVerifier{"a": {UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}})
	conn := connect(t, server, "a")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := conn.Write(ctx, coderws.MessageText, []byte(validCommand)); err != nil {
		t.Fatal(err)
	}
	if got := readEvent(t, conn); got["type"] != "error" || got["request_id"] != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("timeout reply: %v", got)
	}
	select {
	case <-sender.canceled:
	default:
		t.Fatal("work not canceled")
	}
}

func TestRateLimitClosesConnection(t *testing.T) {
	cfg := testLifecycleConfig()
	cfg.CommandBurst = 1
	cfg.CommandRate = 1
	hub := ws.NewHub(cfg)
	server := realtimeServer(t, hub, mapVerifier{"a": {UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}})
	conn := connect(t, server, "a")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for range 2 {
		if err := conn.Write(ctx, coderws.MessageText, []byte("{}")); err != nil {
			t.Fatal(err)
		}
	}
	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			if coderws.CloseStatus(err) != coderws.StatusPolicyViolation {
				t.Fatal(err)
			}
			break
		}
	}
}

func TestMissingPongClosesConnection(t *testing.T) {
	cfg := testLifecycleConfig()
	cfg.PingInterval = 10 * time.Millisecond
	cfg.PongTimeout = 30 * time.Millisecond
	hub := ws.NewHub(cfg)
	server := realtimeServer(t, hub, mapVerifier{"a": {UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}})
	_ = connect(t, server, "a")
	waitForConnections(t, hub, 1, 1)
	waitForConnections(t, hub, 1, 0)
}

func TestExpiryInterruptsBlockedWrite(t *testing.T) {
	cfg := testLifecycleConfig()
	cfg.WriteTimeout = 10 * time.Second
	hub := ws.NewHub(cfg)
	server := realtimeServer(t, hub, mapVerifier{"a": {UserID: 1, ExpiresAt: time.Now().Add(150 * time.Millisecond)}})
	_ = connect(t, server, "a")
	waitForConnections(t, hub, 1, 1)
	if err := hub.Publish([]int64{1}, []byte(strings.Repeat("x", 16*1024*1024))); err != nil {
		t.Fatal(err)
	}
	waitForConnections(t, hub, 1, 0)
}

func TestBlockedCommandCanceledOnExpiry(t *testing.T) {
	sender := &blockingSender{make(chan struct{}), make(chan struct{})}
	cfg := testLifecycleConfig()
	cfg.PingInterval = 10 * time.Millisecond
	cfg.PongTimeout = time.Second
	hub := ws.NewHub(cfg, &ws.Commands{Send: sender})
	server := realtimeServer(t, hub, mapVerifier{"a": auth.Identity{UserID: 1, ExpiresAt: time.Now().Add(200 * time.Millisecond)}})
	conn := connect(t, server, "a")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := conn.Write(ctx, coderws.MessageText, []byte(validCommand)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-sender.started:
	case <-ctx.Done():
		t.Fatal("command not started")
	}
	// Reading responds to ping while SQL remains blocked.
	if _, _, err := conn.Read(ctx); coderws.CloseStatus(err) != coderws.StatusPolicyViolation {
		t.Fatalf("expiry: %v", err)
	}
	select {
	case <-sender.canceled:
	case <-ctx.Done():
		t.Fatal("SQL context not canceled")
	}
	waitForConnections(t, hub, 1, 0)
}

func TestSocketInputLimits(t *testing.T) {
	for _, tc := range []struct {
		name    string
		kind    coderws.MessageType
		payload string
		status  coderws.StatusCode
	}{
		{"binary", coderws.MessageBinary, validCommand, coderws.StatusUnsupportedData},
		{"oversized", coderws.MessageText, strings.Repeat("x", 2048), coderws.StatusMessageTooBig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hub := ws.NewHub(testLifecycleConfig())
			server := realtimeServer(t, hub, mapVerifier{"a": {UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}})
			conn := connect(t, server, "a")
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := conn.Write(ctx, tc.kind, []byte(tc.payload)); err != nil {
				t.Fatal(err)
			}
			if _, _, err := conn.Read(ctx); coderws.CloseStatus(err) != tc.status {
				t.Fatalf("status=%d err=%v", coderws.CloseStatus(err), err)
			}
			waitForConnections(t, hub, 1, 0)
		})
	}
}

func TestCommandOverflowCancelsWorker(t *testing.T) {
	sender := &blockingSender{make(chan struct{}), make(chan struct{})}
	cfg := testLifecycleConfig()
	cfg.CommandQueueCapacity = 1
	hub := ws.NewHub(cfg, &ws.Commands{Send: sender})
	server := realtimeServer(t, hub, mapVerifier{"a": {UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}})
	conn := connect(t, server, "a")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := conn.Write(ctx, coderws.MessageText, []byte(validCommand)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-sender.started:
	case <-ctx.Done():
		t.Fatal("not started")
	}
	for range 2 {
		if err := conn.Write(ctx, coderws.MessageText, []byte(validCommand)); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := conn.Read(ctx); coderws.CloseStatus(err) != coderws.StatusPolicyViolation {
		t.Fatalf("overflow: %v", err)
	}
	select {
	case <-sender.canceled:
	case <-ctx.Done():
		t.Fatal("worker leaked")
	}
}

func TestExpiryAndShutdownDoNotWaitForPong(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		t.Run(fmt.Sprint(shutdown), func(t *testing.T) {
			cfg := testLifecycleConfig()
			cfg.PingInterval = 10 * time.Millisecond
			cfg.PongTimeout = 10 * time.Second
			expiry := time.Now().Add(time.Hour)
			if !shutdown {
				expiry = time.Now().Add(100 * time.Millisecond)
			}
			hub := ws.NewHub(cfg)
			server := realtimeServer(t, hub, mapVerifier{"a": {UserID: 1, ExpiresAt: expiry}})
			_ = connect(t, server, "a") // Intentionally never read: no pong or close handshake response.
			waitForConnections(t, hub, 1, 1)
			if shutdown {
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()
				_ = hub.Shutdown(ctx)
			}
			waitForConnections(t, hub, 1, 0)
		})
	}
}
