package handler

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
)

type registerFunc func(context.Context, auth.RegisterInput) (auth.PublicUser, error)

func (f registerFunc) Execute(ctx context.Context, in auth.RegisterInput) (auth.PublicUser, error) {
	return f(ctx, in)
}

func TestRegisterHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	valid := `{"name":"An","email":"an@example.test","password":"password123"}`
	for _, tc := range []struct {
		name, body, contentType string
		err                     error
		status                  int
		called                  bool
	}{
		{"success", valid, "application/json", nil, 201, true},
		{"duplicate", valid, "application/json", entity.ErrEmailTaken, 409, true},
		{"validation", valid, "application/json", auth.ErrInvalidInput, 400, true},
		{"storage", valid, "application/json", errors.New("SECRET DATABASE ERROR"), 500, true},
		{"wrong_media", valid, "text/plain", nil, 415, false},
		{"malformed", "{", "application/json", nil, 400, false},
		{"trailing", valid + ` {}`, "application/json", nil, 400, false},
		{"unknown", `{"sender_id":2}`, "application/json", nil, 400, false},
		{"array", `[]`, "application/json", nil, 400, false},
		{"null", `null`, "application/json", nil, 400, false},
		{"wrong_type", `{"name":123}`, "application/json", nil, 400, false},
		{"too_large", `{"name":"` + strings.Repeat("a", 16384) + `"}`, "application/json", nil, 413, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			usecase := registerFunc(func(ctx context.Context, in auth.RegisterInput) (auth.PublicUser, error) {
				called = true
				if in.Name != "An" || in.Email != "an@example.test" || in.Password != "password123" {
					t.Fatal("request mapping failed")
				}
				return auth.PublicUser{ID: 1, Name: "An", Email: in.Email}, tc.err
			})
			router := gin.New()
			router.POST("/api/auth/register", NewRegisterHandler(usecase).Handle)
			req := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			out := httptest.NewRecorder()
			router.ServeHTTP(out, req)
			if out.Code != tc.status || called != tc.called {
				t.Fatalf("status=%d called=%v", out.Code, called)
			}
			if strings.Contains(out.Body.String(), "SECRET") || strings.Contains(strings.ToLower(out.Body.String()), "password") {
				t.Fatal("sensitive data exposed")
			}
			if tc.status == 201 && out.Body.String() != `{"user":{"id":1,"name":"An","email":"an@example.test","avatar_url":""}}` {
				t.Fatalf("unexpected DTO: %s", out.Body.String())
			}
		})
	}
}
