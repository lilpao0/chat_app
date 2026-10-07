package handler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type loginFunc func(context.Context, auth.LoginInput) (auth.LoginResult, error)

func (f loginFunc) Execute(ctx context.Context, in auth.LoginInput) (auth.LoginResult, error) {
	return f(ctx, in)
}

func TestLoginHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{{nil, 200, ""}, {auth.ErrInvalidCredentials, 401, "invalid_credentials"}, {errors.New("PRIVATE ERROR"), 500, "internal_error"}} {
		usecase := loginFunc(func(ctx context.Context, in auth.LoginInput) (auth.LoginResult, error) {
			if in.Email != "an@example.test" || in.Password != "password123" {
				t.Fatal("bad login input")
			}
			return auth.LoginResult{
				User:         auth.PublicUser{ID: 1},
				Token:        auth.AccessToken{Value: "issued-token", ExpiresAt: time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)},
				RefreshToken: auth.RefreshToken{Value: "refresh-token", ExpiresAt: time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)},
			}, tc.err
		})
		r := gin.New()
		r.POST("/login", NewLoginHandler(usecase).Handle)
		req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"email":"an@example.test","password":"password123"}`))
		req.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		if out.Code != tc.status {
			t.Fatalf("status %d", out.Code)
		}
		if strings.Contains(out.Body.String(), "PRIVATE") || strings.Contains(out.Body.String(), "password123") {
			t.Fatal("secret leaked")
		}
		var body struct {
			Status string `json:"status"`
			Data   struct {
				AccessToken      string `json:"access_token"`
				ExpiresAt        string `json:"expires_at"`
				RefreshToken     string `json:"refresh_token"`
				RefreshExpiresAt string `json:"refresh_expires_at"`
			} `json:"data"`
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(out.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if tc.status == http.StatusOK {
			if body.Status != "success" || body.Data.AccessToken != "issued-token" || body.Data.ExpiresAt != "2026-09-22T00:00:00Z" || body.Data.RefreshToken != "refresh-token" || body.Data.RefreshExpiresAt != "2026-10-22T00:00:00Z" || out.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("incorrect token response: %s", out.Body.String())
			}
		} else {
			wantStatus := "fail"
			if tc.status >= 500 {
				wantStatus = "error"
			}
			if body.Status != wantStatus || body.Error.Code != tc.code {
				t.Fatalf("incorrect error response: %s", out.Body.String())
			}
		}
	}
}
