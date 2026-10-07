package websocket_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	presentationws "github.com/lilpao0/chat_app/backend/internal/presentation/websocket"
)

type tokenVerifierFake struct {
	identity domainauth.Identity
	err      error
	raw      string
	calls    int
}

func (f *tokenVerifierFake) Verify(raw string) (domainauth.Identity, error) {
	f.calls++
	f.raw = raw
	return f.identity, f.err
}

type sessionRunnerFake struct {
	identity domainauth.Identity
	calls    int
	called   chan struct{}
}

func (f *sessionRunnerFake) Run(connection *coderws.Conn, identity domainauth.Identity) {
	f.calls++
	f.identity = identity
	if f.called != nil {
		close(f.called)
	}
	_ = connection.Close(coderws.StatusNormalClosure, "")
}

func websocketTestServer(t *testing.T, verifier *tokenVerifierFake, sessions *sessionRunnerFake) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := presentationws.NewHandler(verifier, sessions, nil, nil)
	router.GET("/ws", handler.Handle)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server
}

func websocketURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func TestWebSocketHandshakeAcceptsValidAccessToken(t *testing.T) {
	expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	verifier := &tokenVerifierFake{identity: domainauth.Identity{UserID: 7, ExpiresAt: expiry}}
	sessions := &sessionRunnerFake{called: make(chan struct{})}
	server := websocketTestServer(t, verifier, sessions)
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer access-token")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	connection, response, err := coderws.Dial(ctx, websocketURL(server.URL)+"/ws", &coderws.DialOptions{HTTPHeader: headers})
	if err != nil {
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		t.Fatalf("dial WebSocket: %v, status=%d", err, status)
	}
	defer connection.CloseNow()
	select {
	case <-sessions.called:
	case <-ctx.Done():
		t.Fatal("session runner was not called")
	}
	if verifier.calls != 1 || verifier.raw != "access-token" {
		t.Fatalf("verifier calls=%d raw=%q", verifier.calls, verifier.raw)
	}
	if sessions.calls != 1 || sessions.identity.UserID != 7 || !sessions.identity.ExpiresAt.Equal(expiry) {
		t.Fatalf("unexpected session identity: %+v", sessions.identity)
	}
}

type errorResponse struct {
	Status string `json:"status"`
	Error  struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func assertHandshakeFailure(t *testing.T, server *httptest.Server, headers http.Header, wantStatus int, wantCode string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	connection, response, err := coderws.Dial(ctx, websocketURL(server.URL)+"/ws", &coderws.DialOptions{HTTPHeader: headers})
	if connection != nil {
		connection.CloseNow()
		t.Fatal("unexpected WebSocket connection")
	}
	if err == nil {
		t.Fatal("expected handshake failure")
	}
	if response == nil {
		t.Fatalf("missing HTTP response: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != wantStatus {
		t.Fatalf("status = %d, want %d", response.StatusCode, wantStatus)
	}
	var body errorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Status != "fail" || body.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q", body.Error.Code, wantCode)
	}
}

func TestWebSocketHandshakeRejectsAuthenticationFailures(t *testing.T) {
	tests := []struct {
		name        string
		headers     []string
		verifyError error
		wantCalls   int
	}{
		{name: "missing token"},
		{name: "malformed token", headers: []string{"Bearer"}},
		{name: "wrong scheme", headers: []string{"Basic token"}},
		{name: "duplicate headers", headers: []string{"Bearer token", "Bearer token"}},
		{name: "invalid token", headers: []string{"Bearer invalid"}, verifyError: domainauth.ErrInvalidToken, wantCalls: 1},
		{name: "refresh token", headers: []string{"Bearer refresh-token"}, verifyError: domainauth.ErrInvalidToken, wantCalls: 1},
		{name: "expired token", headers: []string{"Bearer expired-token"}, verifyError: domainauth.ErrInvalidToken, wantCalls: 1},
		{name: "internal verifier error", headers: []string{"Bearer secret-token"}, verifyError: errors.New("secret verifier detail"), wantCalls: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifier := &tokenVerifierFake{err: test.verifyError}
			sessions := &sessionRunnerFake{}
			server := websocketTestServer(t, verifier, sessions)
			headers := make(http.Header)
			for _, value := range test.headers {
				headers.Add("Authorization", value)
			}
			assertHandshakeFailure(t, server, headers, http.StatusUnauthorized, "unauthenticated")
			if verifier.calls != test.wantCalls {
				t.Fatalf("verifier calls = %d, want %d", verifier.calls, test.wantCalls)
			}
			if sessions.calls != 0 {
				t.Fatalf("session calls = %d, want 0", sessions.calls)
			}
		})
	}
}

func TestWebSocketHandshakeRejectsBrowserOrigin(t *testing.T) {
	verifier := &tokenVerifierFake{identity: domainauth.Identity{UserID: 7}}
	sessions := &sessionRunnerFake{}
	server := websocketTestServer(t, verifier, sessions)
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer access-token")
	headers.Add("Origin", "")
	headers.Add("Origin", "https://example.com")
	assertHandshakeFailure(t, server, headers, http.StatusForbidden, "origin_not_allowed")
	if verifier.calls != 0 || sessions.calls != 0 {
		t.Fatalf("verifier=%d sessions=%d", verifier.calls, sessions.calls)
	}
}

func TestWebSocketHandshakeRejectsNonGET(t *testing.T) {
	gin.SetMode(gin.TestMode)
	verifier := &tokenVerifierFake{}
	sessions := &sessionRunnerFake{}
	handler := presentationws.NewHandler(verifier, sessions, nil, nil)
	router := gin.New()
	router.Any("/ws", handler.Handle)
	request := httptest.NewRequest(http.MethodPost, "/ws", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
	if verifier.calls != 0 || sessions.calls != 0 {
		t.Fatalf("verifier=%d sessions=%d", verifier.calls, sessions.calls)
	}
}
