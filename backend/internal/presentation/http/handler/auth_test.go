package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
	valid := `{"first_name":"An","last_name":"B","email":"an@example.test","password":"password123"}`
	for _, tc := range []struct {
		name, body, contentType string
		err                     error
		status                  int
		called                  bool
	}{
		{"success", valid, "application/json", nil, 201, true},
		{"duplicate", valid, "application/json", entity.ErrEmailTaken, 409, true},
		{"validation", valid, "application/json", auth.ValidationError{Field: "email", Message: "Email is invalid"}, 400, true},
		{"storage", valid, "application/json", errors.New("SECRET DATABASE ERROR"), 500, true},
		{"wrong_media", valid, "text/plain", nil, 415, false},
		{"malformed", "{", "application/json", nil, 400, false},
		{"trailing", valid + ` {}`, "application/json", nil, 400, false},
		{"unknown", `{"sender_id":2}`, "application/json", nil, 400, false},
		{"array", `[]`, "application/json", nil, 400, false},
		{"null", `null`, "application/json", nil, 400, false},
		{"wrong_type", `{"first_name":123}`, "application/json", nil, 400, false},
		{"too_large", `{"first_name":"` + strings.Repeat("a", 16384) + `"}`, "application/json", nil, 413, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			usecase := registerFunc(func(ctx context.Context, in auth.RegisterInput) (auth.PublicUser, error) {
				called = true
				if in.FirstName != "An" || in.LastName != "B" || in.Email != "an@example.test" || in.Password != "password123" {
					t.Fatal("request mapping failed")
				}
				return auth.PublicUser{ID: 1, Name: "An B", Email: in.Email}, tc.err
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
			if tc.status == http.StatusCreated {
				var body struct {
					Status string  `json:"status"`
					Data   userDTO `json:"data"`
				}

				if err := json.Unmarshal(out.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Status != "success" ||
					body.Data.ID != 1 ||
					body.Data.Name != "An B" ||
					body.Data.Email != "an@example.test" {
					t.Fatalf("unexpected DTO: %s", out.Body.String())
				}
			}

			if tc.status >= 400 {
				wantStatus := "fail"
				if tc.status >= 500 {
					wantStatus = "error"
				}

				var body struct {
					Status string `json:"status"`
					Error  struct {
						Code    string `json:"code"`
						Message string `json:"message"`
						Details []struct {
							Field   string `json:"field"`
							Code    string `json:"code"`
							Message string `json:"message"`
						} `json:"details"`
					} `json:"error"`
				}
				if err := json.Unmarshal(out.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Status != wantStatus {
					t.Fatalf(
						"response status=%q want=%q body=%s",
						body.Status,
						wantStatus,
						out.Body.String(),
					)
				}
				if tc.name == "validation" && (body.Error.Code != "invalid_input" ||
					body.Error.Message != "The submitted information is invalid." ||
					len(body.Error.Details) != 1 ||
					body.Error.Details[0].Field != "email" ||
					body.Error.Details[0].Code != "invalid_email" ||
					body.Error.Details[0].Message != "Email is invalid") {
					t.Fatalf("unexpected validation response: %s", out.Body.String())
				}
				if tc.status >= 500 && body.Error.Details != nil {
					t.Fatalf("server error exposed details: %s", out.Body.String())
				}
			}
		})
	}
}
