package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
)

type refreshUseCaseFunc func(context.Context, string) (auth.AccessToken, error)

func (f refreshUseCaseFunc) Execute(ctx context.Context, value string) (auth.AccessToken, error) {
	return f(ctx, value)
}

func TestRefreshHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expires := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, body string
		err        error
		status     int
		code       string
	}{
		{"success", `{"refresh_token":"valid-refresh"}`, nil, 200, ""},
		{"invalid", `{"refresh_token":"invalid"}`, auth.ErrInvalidToken, 401, "invalid_refresh_token"},
		{"bad_json", `{`, nil, 400, "invalid_input"},
		{"internal", `{"refresh_token":"valid-refresh"}`, errors.New("signing"), 500, "internal_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.POST("/api/auth/refresh", NewRefreshHandler(refreshUseCaseFunc(func(_ context.Context, value string) (auth.AccessToken, error) {
				if tc.name != "bad_json" && value == "" {
					t.Fatal("missing refresh token")
				}
				return auth.AccessToken{Value: "new-access", ExpiresAt: expires}, tc.err
			})).Handle)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status/body = %d/%q", response.Code, response.Body.String())
			}
			var body struct {
				Status string `json:"status"`
				Data   struct {
					AccessToken string `json:"access_token"`
					ExpiresAt   string `json:"expires_at"`
				} `json:"data"`
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if tc.name == "success" {
				if body.Status != "success" || body.Data.AccessToken != "new-access" || body.Data.ExpiresAt != expires.Format(time.RFC3339) || response.Header().Get("Cache-Control") != "no-store" {
					t.Fatalf("incorrect success response: %s", response.Body.String())
				}
			} else {
				wantStatus := "fail"
				if tc.status >= 500 {
					wantStatus = "error"
				}
				if body.Status != wantStatus || body.Error.Code != tc.code {
					t.Fatalf("incorrect error response: %s", response.Body.String())
				}
			}
		})
	}
}
