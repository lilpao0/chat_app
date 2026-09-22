package auth_test

import (
	"context"
	"errors"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"testing"
)

type issueFunc func(int64) (domainauth.AccessToken, error)

func (f issueFunc) Issue(id int64) (domainauth.AccessToken, error) { return f(id) }

func TestLogin(t *testing.T) {
	storageErr := errors.New("storage")
	hashErr := errors.New("hash")
	tokenErr := errors.New("token")
	for _, tc := range []struct {
		name                          string
		findErr, compareErr, issueErr error
		match                         bool
		want                          error
	}{
		{"success", nil, nil, nil, true, nil},
		{"missing", entity.ErrUserNotFound, nil, nil, false, domainauth.ErrInvalidCredentials},
		{"wrong_password", nil, nil, nil, false, domainauth.ErrInvalidCredentials},
		{"repository_error", storageErr, nil, nil, false, storageErr},
		{"hash_error", nil, hashErr, nil, false, hashErr},
		{"token_error", nil, nil, tokenErr, true, tokenErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			issued := false
			users := fakeUsers{find: func(got context.Context, email string) (entity.User, error) {
				if got != ctx || email != "an@example.test" {
					t.Fatal("lookup input incorrect")
				}
				return entity.User{ID: 7, Email: email, PasswordHash: "stored"}, tc.findErr
			}}
			passwords := fakePasswords{compare: func(hash, password string) (bool, error) {
				if hash != "stored" || password != " password123 " {
					t.Fatal("password changed")
				}
				return tc.match, tc.compareErr
			}}
			tokens := issueFunc(func(id int64) (domainauth.AccessToken, error) {
				issued = true
				if id != 7 {
					t.Fatal("wrong identity")
				}
				return domainauth.AccessToken{Value: "token"}, tc.issueErr
			})
			result, err := domainauth.NewLogin(users, passwords, tokens).Execute(ctx, domainauth.LoginInput{Email: " AN@EXAMPLE.TEST ", Password: " password123 "})
			if !errors.Is(err, tc.want) {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.want == nil && (result.User.ID != 7 || result.Token.Value != "token") {
				t.Fatal("bad login result")
			}
			if (tc.findErr != nil || tc.compareErr != nil || !tc.match) && issued {
				t.Fatal("token issued after failed authentication")
			}
		})
	}
	for _, in := range []domainauth.LoginInput{{Email: "bad", Password: "password123"}, {Email: "an@example.test", Password: "short"}} {
		if _, err := domainauth.NewLogin(fakeUsers{}, fakePasswords{}, nil).Execute(context.Background(), in); !errors.Is(err, domainauth.ErrInvalidInput) {
			t.Fatal("invalid input accepted")
		}
	}
}
