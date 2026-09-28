package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

var ErrInvalidCredentials = errors.New("invalid email or password")

type LoginInput struct{ Email, Password string }
type LoginResult struct {
	User         PublicUser
	Token        AccessToken
	RefreshToken RefreshToken
}
type Login struct {
	users     repository.UserRepository
	passwords PasswordHasher
	tokens    TokenIssuer
}

func NewLogin(users repository.UserRepository, passwords PasswordHasher, tokens TokenIssuer) *Login {
	return &Login{users: users, passwords: passwords, tokens: tokens}
}

func (u *Login) Execute(ctx context.Context, input LoginInput) (LoginResult, error) {
	if err := ctx.Err(); err != nil {
		return LoginResult{}, err
	}
	email, err := NormalizeEmail(input.Email)
	if err != nil || ValidatePassword(input.Password) != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	user, err := u.users.FindByEmail(ctx, email)
	if errors.Is(err, entity.ErrUserNotFound) {
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, fmt.Errorf("find login user: %w", err)
	}
	match, err := u.passwords.Compare(user.PasswordHash, input.Password)
	if err != nil {
		return LoginResult{}, fmt.Errorf("compare login password: %w", err)
	}
	if !match {
		return LoginResult{}, ErrInvalidCredentials
	}
	if err := ctx.Err(); err != nil {
		return LoginResult{}, err
	}
	token, err := u.tokens.Issue(user.ID)
	if err != nil {
		return LoginResult{}, fmt.Errorf("issue login token: %w", err)
	}
	refreshToken, err := u.tokens.IssueRefresh(user.ID)
	if err != nil {
		return LoginResult{}, fmt.Errorf("issue login refresh token: %w", err)
	}
	return LoginResult{User: publicUser(user), Token: token, RefreshToken: refreshToken}, nil
}
