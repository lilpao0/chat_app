package auth_test

import (
	"context"
	"errors"
	"testing"

	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
)

type refreshFunc func(string) (domainauth.AccessToken, error)

func (f refreshFunc) Refresh(value string) (domainauth.AccessToken, error) { return f(value) }

func TestRefresh(t *testing.T) {
	t.Run("issues access token", func(t *testing.T) {
		usecase := domainauth.NewRefresh(refreshFunc(func(value string) (domainauth.AccessToken, error) {
			if value != "refresh-token" {
				t.Fatalf("value = %q", value)
			}
			return domainauth.AccessToken{Value: "new-access-token"}, nil
		}))
		token, err := usecase.Execute(context.Background(), "refresh-token")
		if err != nil || token.Value != "new-access-token" {
			t.Fatalf("token/error = %#v/%v", token, err)
		}
	})

	t.Run("rejects empty token", func(t *testing.T) {
		if _, err := domainauth.NewRefresh(nil).Execute(context.Background(), ""); !errors.Is(err, domainauth.ErrInvalidToken) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("preserves invalid-token cause", func(t *testing.T) {
		usecase := domainauth.NewRefresh(refreshFunc(func(string) (domainauth.AccessToken, error) {
			return domainauth.AccessToken{}, domainauth.ErrInvalidToken
		}))
		if _, err := usecase.Execute(context.Background(), "bad-token"); !errors.Is(err, domainauth.ErrInvalidToken) {
			t.Fatalf("error = %v", err)
		}
	})
}
