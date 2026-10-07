package websocket_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	dataauth "github.com/lilpao0/chat_app/backend/internal/data/auth"
	"github.com/lilpao0/chat_app/backend/internal/presentation/authentication"
	presentation "github.com/lilpao0/chat_app/backend/internal/presentation/http"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/handler"
	ws "github.com/lilpao0/chat_app/backend/internal/presentation/websocket"
)

const browserOrigin = "http://localhost:5173"

func browserServer(t *testing.T, ttl time.Duration) (*httptest.Server, *dataauth.JWT, *ws.Hub) {
	t.Helper()
	tokens, err := dataauth.NewJWT(strings.Repeat("b", 32), "browser-test", "chat-app-mobile", ttl)
	if err != nil {
		t.Fatal(err)
	}
	origins := []string{browserOrigin, "http://localhost:5174"}
	tickets := authentication.NewTicketStore()
	r, protected := presentation.NewRouter(nil, nil, nil, tokens, origins)
	protected.POST("/ws/tickets", handler.NewWSTicketHandler(tickets, origins))
	hub := ws.NewHub(testLifecycleConfig())
	r.GET("/ws", ws.NewHandler(tokens, hub, origins, tickets).Handle)
	server := httptest.NewServer(r)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := hub.Shutdown(ctx); err != nil {
			t.Error(err)
		}
		server.Close()
	})
	return server, tokens, hub
}

func issueBrowserTicket(t *testing.T, server *httptest.Server, token string) string {
	t.Helper()
	req, err := http.NewRequest("POST", server.URL+"/api/ws/tickets", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", browserOrigin)
	req.Header.Set("Authorization", "Bearer "+token)
	out, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Body.Close()
	var body struct {
		Status string `json:"status"`
		Data   struct {
			Ticket    string    `json:"ticket"`
			ExpiresAt time.Time `json:"expires_at"`
		} `json:"data"`
	}
	if err := json.NewDecoder(out.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if out.StatusCode != 201 || body.Status != "success" || len(body.Data.Ticket) != 43 || !time.Now().Before(body.Data.ExpiresAt) || out.Header.Get("Cache-Control") != "no-store" || out.Header.Get("Access-Control-Allow-Origin") != browserOrigin {
		t.Fatalf("ticket status=%d headers=%v", out.StatusCode, out.Header)
	}
	return body.Data.Ticket
}

func browserDial(server *httptest.Server, suffix, origin, authorization string) (*coderws.Conn, *http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	headers := make(http.Header)
	if origin != "" {
		headers.Set("Origin", origin)
	}
	if authorization != "" {
		headers.Set("Authorization", authorization)
	}
	return coderws.Dial(ctx, websocketURL(server.URL)+"/ws"+suffix, &coderws.DialOptions{HTTPHeader: headers})
}

func TestBrowserTicketsAndCredentialAdmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, tokens, hub := browserServer(t, time.Hour)
	token, err := tokens.Issue(7)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, query, origin, bearer string
		status                      int
		code                        string
	}{
		{"missing", "", browserOrigin, "", 401, "invalid_ws_ticket"},
		{"invalid", "?ticket=" + strings.Repeat("x", 43), browserOrigin, "", 401, "invalid_ws_ticket"},
		{"duplicate", "?ticket=x&ticket=y", browserOrigin, "", 400, "invalid_input"},
		{"empty", "?ticket=", browserOrigin, "", 400, "invalid_input"},
		{"jwt query", "?access_token=jwt", browserOrigin, "", 400, "invalid_input"},
		{"mixed", "?ticket=x", browserOrigin, "Bearer jwt", 400, "invalid_input"},
		{"no origin ticket", "?ticket=x", "", "", 400, "invalid_input"},
		{"unapproved", "?ticket=x", "https://evil.example", "", 403, ""},
		{"malformed", "?ticket=%ZZ", browserOrigin, "", 400, "invalid_input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, out, err := browserDial(server, tc.query, tc.origin, tc.bearer)
			if conn != nil {
				conn.CloseNow()
				t.Fatal("unexpected connection")
			}
			if err == nil || out == nil {
				t.Fatal("expected HTTP failure")
			}
			defer out.Body.Close()
			if tc.code == "" {
				if out.StatusCode != tc.status {
					t.Fatalf("status=%d", out.StatusCode)
				}
				return // CORS middleware rejects disallowed origins before the WS handler.
			}
			var body errorResponse
			if json.NewDecoder(out.Body).Decode(&body) != nil || out.StatusCode != tc.status || body.Status != "fail" || body.Error.Code != tc.code {
				t.Fatalf("status=%d body=%+v", out.StatusCode, body)
			}
		})
	}
	ticket := issueBrowserTicket(t, server, token.Value)
	conn, out, err := browserDial(server, "?ticket="+ticket, "http://localhost:5174", "")
	if conn != nil {
		conn.CloseNow()
		t.Fatal("wrong-origin ticket accepted")
	}
	if err == nil || out == nil || out.StatusCode != 401 {
		t.Fatal("wrong-origin failure missing")
	}
	out.Body.Close()
	conn, _, err = browserDial(server, "?ticket="+ticket, browserOrigin, "")
	if err != nil {
		t.Fatal(err)
	}
	waitForConnections(t, hub, 7, 1)
	conn.CloseNow()
	conn, out, err = browserDial(server, "?ticket="+ticket, browserOrigin, "")
	if conn != nil {
		conn.CloseNow()
		t.Fatal("reused ticket accepted")
	}
	if err == nil || out == nil || out.StatusCode != 401 {
		t.Fatal("reuse failure missing")
	}
	out.Body.Close()
	// Existing mobile clients keep using Bearer without an Origin header.
	conn, _, err = browserDial(server, "", "", "Bearer "+token.Value)
	if err != nil {
		t.Fatal(err)
	}
	conn.CloseNow()
}

func TestBrowserTicketConcurrentHandshakes(t *testing.T) {
	server, tokens, _ := browserServer(t, time.Hour)
	token, err := tokens.Issue(8)
	if err != nil {
		t.Fatal(err)
	}
	ticket := issueBrowserTicket(t, server, token.Value)
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, out, err := browserDial(server, "?ticket="+ticket, browserOrigin, "")
			if err == nil {
				accepted.Add(1)
				conn.CloseNow()
			} else if out == nil || out.StatusCode != 401 {
				t.Errorf("unexpected handshake failure")
			}
			if out != nil && out.Body != nil {
				out.Body.Close()
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("accepted=%d", accepted.Load())
	}
}

func TestBrowserTicketIssuanceFailures(t *testing.T) {
	server, tokens, _ := browserServer(t, time.Hour)
	token, _ := tokens.Issue(9)
	refresh, _ := tokens.IssueRefresh(9)
	for _, tc := range []struct {
		token, origin, body string
		status              int
	}{
		{"", browserOrigin, "", 401}, {refresh.Value, browserOrigin, "", 401},
		{token.Value, "", "", 403}, {token.Value, "null", "", 403},
		{token.Value, "http://localhost:5175", "", 403},
		{token.Value, browserOrigin, `{"user_id":1}`, 400},
	} {
		req, _ := http.NewRequest("POST", server.URL+"/api/ws/tickets", strings.NewReader(tc.body))
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		out, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		out.Body.Close()
		if out.StatusCode != tc.status {
			t.Fatalf("issuance status=%d want=%d", out.StatusCode, tc.status)
		}
	}
	for range 8 {
		issueBrowserTicket(t, server, token.Value)
	}
	req, _ := http.NewRequest("POST", server.URL+"/api/ws/tickets", nil)
	req.Header.Set("Origin", browserOrigin)
	req.Header.Set("Authorization", "Bearer "+token.Value)
	out, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Body.Close()
	var body errorResponse
	if json.NewDecoder(out.Body).Decode(&body) != nil || out.StatusCode != 429 || body.Error.Code != "rate_limited" {
		t.Fatal("capacity response missing")
	}
}

func TestBrowserAccessExpiryAndFailedUpgradeConsumeTicket(t *testing.T) {
	server, tokens, _ := browserServer(t, 3*time.Second)
	token, err := tokens.Issue(10)
	if err != nil {
		t.Fatal(err)
	}
	ticket := issueBrowserTicket(t, server, token.Value)
	conn, _, err := browserDial(server, "?ticket="+ticket, browserOrigin, "")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err = conn.Read(ctx)
	if coderws.CloseStatus(err) != coderws.StatusPolicyViolation {
		t.Fatalf("access expiry status=%d err=%v", coderws.CloseStatus(err), err)
	}
	// An expired access token cannot issue a new ticket.
	req, _ := http.NewRequest("POST", server.URL+"/api/ws/tickets", nil)
	req.Header.Set("Origin", browserOrigin)
	req.Header.Set("Authorization", "Bearer "+token.Value)
	out, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	out.Body.Close()
	if out.StatusCode != 401 {
		t.Fatal("expired access issued ticket")
	}
	server, tokens, _ = browserServer(t, time.Hour)
	token, _ = tokens.Issue(10)
	ticket = issueBrowserTicket(t, server, token.Value)
	req, _ = http.NewRequest("GET", fmt.Sprintf("%s/ws?ticket=%s", server.URL, ticket), nil)
	req.Header.Set("Origin", browserOrigin)
	out, err = server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	out.Body.Close()
	if out.StatusCode == 101 {
		t.Fatal("malformed upgrade accepted")
	}
	conn, out, err = browserDial(server, "?ticket="+ticket, browserOrigin, "")
	if conn != nil {
		conn.CloseNow()
		t.Fatal("failed-upgrade ticket restored")
	}
	if err == nil || out == nil || out.StatusCode != 401 {
		t.Fatal("failed-upgrade ticket remained usable")
	}
	out.Body.Close()
}
