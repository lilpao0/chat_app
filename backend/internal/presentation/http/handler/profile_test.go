package handler_test

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
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/user"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/handler"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/middleware"
)

type profileTokens struct{ identity domainauth.Identity }

func (t profileTokens) Verify(string) (domainauth.Identity, error) { return t.identity, nil }

type privateProfileUseCase struct {
	profile entity.PrivateProfile
	err     error
	userID  int64
	calls   int
}

func (f *privateProfileUseCase) Execute(_ context.Context, userID int64) (entity.PrivateProfile, error) {
	f.calls++
	f.userID = userID
	return f.profile, f.err
}

type publicProfileUseCase struct {
	profile entity.PublicProfile
	err     error
	userID  int64
	calls   int
}

type updateProfileUseCase struct {
	profile entity.PrivateProfile
	err     error
	userID  int64
	input   user.UpdateProfileInput
	calls   int
}

func (f *updateProfileUseCase) Execute(_ context.Context, userID int64, input user.UpdateProfileInput) (entity.PrivateProfile, error) {
	f.calls++
	f.userID = userID
	f.input = input
	return f.profile, f.err
}

func (f *publicProfileUseCase) Execute(_ context.Context, userID int64) (entity.PublicProfile, error) {
	f.calls++
	f.userID = userID
	return f.profile, f.err
}

func TestProfileHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("private profile uses authenticated identity and date-only JSON", func(t *testing.T) {
		dob := time.Date(2002, 5, 21, 0, 0, 0, 0, time.UTC)
		phone := "+84901234567"
		private := &privateProfileUseCase{profile: entity.PrivateProfile{ID: 7, FirstName: "Pao", LastName: "Nguyen", Name: "Pao Nguyen", DateOfBirth: &dob, PhoneNumber: &phone, AvatarURL: "avatar"}}
		public := &publicProfileUseCase{}
		r := profileRouter(private, public)
		w := performProfileRequest(r, "/api/users/me", true)
		if w.Code != http.StatusOK || private.calls != 1 || private.userID != 7 {
			t.Fatalf("status=%d calls=%d userID=%d body=%s", w.Code, private.calls, private.userID, w.Body.String())
		}
		var body map[string]map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["user"]["date_of_birth"] != "2002-05-21" || body["user"]["phone_number"] != phone {
			t.Fatalf("unexpected private body: %s", w.Body.String())
		}
	})

	t.Run("public self lookup stays public", func(t *testing.T) {
		private := &privateProfileUseCase{}
		public := &publicProfileUseCase{profile: entity.PublicProfile{ID: 7, Name: "Pao Nguyen", AvatarURL: "avatar"}}
		r := profileRouter(private, public)
		w := performProfileRequest(r, "/api/users/7", true)
		if w.Code != http.StatusOK || public.calls != 1 || public.userID != 7 {
			t.Fatalf("status=%d calls=%d userID=%d body=%s", w.Code, public.calls, public.userID, w.Body.String())
		}
		for _, forbidden := range []string{"first_name", "last_name", "date_of_birth", "phone_number", "email", "password"} {
			if strings.Contains(w.Body.String(), forbidden) {
				t.Fatalf("public response exposes %s: %s", forbidden, w.Body.String())
			}
		}
	})

	for _, path := range []string{"/api/users/abc", "/api/users/0", "/api/users/-1"} {
		t.Run("invalid public ID "+path, func(t *testing.T) {
			public := &publicProfileUseCase{}
			w := performProfileRequest(profileRouter(&privateProfileUseCase{}, public), path, true)
			if w.Code != http.StatusBadRequest || public.calls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", w.Code, public.calls, w.Body.String())
			}
		})
	}

	t.Run("missing and internal errors are safe", func(t *testing.T) {
		for _, tc := range []struct {
			err    error
			status int
		}{
			{entity.ErrUserNotFound, http.StatusNotFound},
			{errors.New("secret database detail"), http.StatusInternalServerError},
		} {
			w := performProfileRequest(profileRouter(&privateProfileUseCase{}, &publicProfileUseCase{err: tc.err}), "/api/users/8", true)
			if w.Code != tc.status || strings.Contains(w.Body.String(), "secret database detail") {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		}
	})

	t.Run("missing token is unauthenticated", func(t *testing.T) {
		private := &privateProfileUseCase{}
		w := performProfileRequest(profileRouter(private, &publicProfileUseCase{}), "/api/users/me", false)
		if w.Code != http.StatusUnauthorized || private.calls != 0 {
			t.Fatalf("status=%d calls=%d body=%s", w.Code, private.calls, w.Body.String())
		}
	})

	t.Run("patch preserves omitted null and concrete field states", func(t *testing.T) {
		phone := "+84901234567"
		update := &updateProfileUseCase{profile: entity.PrivateProfile{ID: 7, FirstName: "Pao", Name: "Pao", PhoneNumber: &phone, AvatarURL: "avatar"}}
		r := profileRouterWithUpdate(&privateProfileUseCase{}, &publicProfileUseCase{}, update)
		w := performProfileJSONRequest(r, http.MethodPatch, "/api/users/me", `{"first_name":" Pao ","date_of_birth":null,"phone_number":"+84901234567"}`, true)
		if w.Code != http.StatusOK || update.calls != 1 || update.userID != 7 {
			t.Fatalf("status=%d calls=%d userID=%d body=%s", w.Code, update.calls, update.userID, w.Body.String())
		}
		if !update.input.FirstName.Set || update.input.FirstName.Value == nil || *update.input.FirstName.Value != " Pao " || update.input.LastName.Set {
			t.Fatalf("unexpected name input: %+v", update.input)
		}
		if !update.input.DateOfBirth.Set || update.input.DateOfBirth.Value != nil || !update.input.PhoneNumber.Set || update.input.PhoneNumber.Value == nil {
			t.Fatalf("unexpected nullable input: %+v", update.input)
		}
	})

	t.Run("patch rejects unknown fields and wrong content type", func(t *testing.T) {
		for _, tc := range []struct {
			contentType string
			body        string
			status      int
		}{
			{"application/json", `{"email":"new@example.test"}`, http.StatusBadRequest},
			{"text/plain", `{"first_name":"Pao"}`, http.StatusUnsupportedMediaType},
		} {
			update := &updateProfileUseCase{}
			r := profileRouterWithUpdate(&privateProfileUseCase{}, &publicProfileUseCase{}, update)
			req := httptest.NewRequest(http.MethodPatch, "/api/users/me", strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer test-token")
			req.Header.Set("Content-Type", tc.contentType)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.status || update.calls != 0 {
				t.Fatalf("content-type=%q status=%d calls=%d body=%s", tc.contentType, w.Code, update.calls, w.Body.String())
			}
		}
	})

	t.Run("patch maps domain errors", func(t *testing.T) {
		for _, tc := range []struct {
			err    error
			status int
			code   string
		}{
			{entity.ErrInvalidInput, http.StatusBadRequest, "invalid_input"},
			{entity.ErrPhoneNumberTaken, http.StatusConflict, "phone_number_taken"},
			{entity.ErrUserNotFound, http.StatusNotFound, "not_found"},
		} {
			update := &updateProfileUseCase{err: tc.err}
			w := performProfileJSONRequest(profileRouterWithUpdate(&privateProfileUseCase{}, &publicProfileUseCase{}, update), http.MethodPatch, "/api/users/me", `{"first_name":"Pao"}`, true)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.code) {
				t.Fatalf("error=%v status=%d body=%s", tc.err, w.Code, w.Body.String())
			}
		}
	})
}

func profileRouter(private *privateProfileUseCase, public *publicProfileUseCase) *gin.Engine {
	return profileRouterWithUpdate(private, public, &updateProfileUseCase{})
}

func profileRouterWithUpdate(private *privateProfileUseCase, public *publicProfileUseCase, update *updateProfileUseCase) *gin.Engine {
	h := handler.NewProfileHandler(private, public, update)
	r := gin.New()
	group := r.Group("/api", middleware.Authenticate(profileTokens{identity: domainauth.Identity{UserID: 7}}))
	group.GET("/users/me", h.GetPrivate)
	group.GET("/users/:id", h.GetPublic)
	group.PATCH("/users/me", h.Update)
	return r
}

func performProfileJSONRequest(r http.Handler, method, path, body string, authorized bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if authorized {
		req.Header.Set("Authorization", "Bearer test-token")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func performProfileRequest(r http.Handler, path string, authorized bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if authorized {
		req.Header.Set("Authorization", "Bearer test-token")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
