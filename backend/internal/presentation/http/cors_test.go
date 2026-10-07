package http_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	presentation "github.com/lilpao0/chat_app/backend/internal/presentation/http"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
)

type corsVerifier struct{ calls int }

func (v *corsVerifier) Verify(string) (auth.Identity, error) {
	v.calls++
	return auth.Identity{UserID: 1, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

type corsRefresh struct{}

func (corsRefresh) Execute(context.Context, string) (auth.AccessToken, error) {
	return auth.AccessToken{}, errors.New("private")
}

func TestCORSPreflightAndApplicationErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	v := &corsVerifier{}
	r, protected := presentation.NewRouter(nil, nil, corsRefresh{}, v, []string{"http://localhost:5173"})
	protected.GET("/conversations", func(c *gin.Context) { response.Success(c, 200, []any{}) })
	protected.GET("/invalid", func(c *gin.Context) { response.Error(c, 400, "invalid_input", "Invalid input.") })
	protected.GET("/failure", func(c *gin.Context) { response.Error(c, 500, "internal_error", "Unable to complete the request.") })
	for _, path := range []string{"/api/conversations", "/api/auth/refresh", "/api/ws/tickets"} {
		req := httptest.NewRequest("OPTIONS", path, nil)
		req.Header.Set("Origin", "http://localhost:5173")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		headers := strings.ToLower(out.Header().Get("Access-Control-Allow-Headers"))
		if out.Code != 204 || v.calls != 0 || !strings.Contains(headers, "authorization") || !strings.Contains(headers, "content-type") || out.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
			t.Fatalf("preflight %s status=%d headers=%v calls=%d", path, out.Code, out.Header(), v.calls)
		}
	}
	for _, tc := range []struct {
		method, path, body string
		bearer             bool
		status             int
	}{
		{"GET", "/health", "", false, 200},
		{"GET", "/api/conversations", "", false, 401},
		{"GET", "/api/conversations", "", true, 200},
		{"GET", "/api/invalid", "", true, 400},
		{"GET", "/api/failure", "", true, 500},
		{"POST", "/api/auth/refresh", "{", false, 400},
		{"POST", "/api/auth/refresh", `{"refresh_token":"test"}`, false, 500},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Origin", "http://localhost:5173")
		req.Header.Set("Content-Type", "application/json")
		if tc.bearer {
			req.Header.Set("Authorization", "Bearer test")
		}
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		if out.Code != tc.status || out.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
			t.Fatalf("%s: %d %v", tc.path, out.Code, out.Header())
		}
	}
	for _, origins := range [][]string{{"http://localhost:5174"}, {"null"}, {"http://localhost:5173", "http://localhost:5173"}} {
		req := httptest.NewRequest("GET", "/health", nil)
		for _, origin := range origins {
			req.Header.Add("Origin", origin)
		}
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		if out.Code != http.StatusForbidden || out.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("unapproved origin admitted")
		}
	}
	r, _ = presentation.NewRouter(nil, nil, nil, nil, nil)
	for _, origin := range []string{"", "http://localhost:5173"} {
		req := httptest.NewRequest("GET", "/health", nil)
		want := 200
		if origin != "" {
			req.Header.Set("Origin", origin)
			want = 403
		}
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		if out.Code != want {
			t.Fatalf("disabled web status=%d want=%d", out.Code, want)
		}
	}
}
