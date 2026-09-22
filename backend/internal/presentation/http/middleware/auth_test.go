package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
)

type verifyFunc func(string) (auth.Identity, error)

func (f verifyFunc) Verify(raw string) (auth.Identity, error) { return f(raw) }

func TestAuthenticate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name    string
		headers []string
		valid   bool
	}{
		{"missing", nil, false}, {"empty", []string{"Bearer"}, false}, {"basic", []string{"Basic good"}, false},
		{"extra", []string{"Bearer good extra"}, false}, {"invalid", []string{"Bearer invalid"}, false},
		{"expired", []string{"Bearer expired"}, false}, {"duplicate", []string{"Bearer good", "Bearer good"}, false},
		{"valid", []string{"Bearer good"}, true}, {"case_insensitive", []string{"bearer good"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verifier := verifyFunc(func(raw string) (auth.Identity, error) {
				if raw != "good" {
					return auth.Identity{}, auth.ErrInvalidToken
				}
				return auth.Identity{UserID: 7}, nil
			})
			r := gin.New()
			called := false
			r.GET("/test", Authenticate(verifier), func(c *gin.Context) {
				called = true
				id, ok := Identity(c)
				if !ok || id.UserID != 7 {
					t.Fatal("identity not propagated")
				}
				c.Status(204)
			})
			req := httptest.NewRequest("GET", "/test?user_id=999", nil)
			for _, value := range tc.headers {
				req.Header.Add("Authorization", value)
			}
			out := httptest.NewRecorder()
			r.ServeHTTP(out, req)
			want := 401
			if tc.valid {
				want = 204
			}
			if out.Code != want || called != tc.valid {
				t.Fatalf("status %d called %v", out.Code, called)
			}
		})
	}
}
