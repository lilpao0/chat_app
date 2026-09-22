package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

var ErrInvalidInput = errors.New("invalid input")

type RegisterInput struct {
	Name, Email, Password string
}

// PublicUser deliberately contains no credentials, even inside the application.
type PublicUser struct {
	ID                     int64
	Name, Email, AvatarURL string
}

func publicUser(user entity.User) PublicUser {
	return PublicUser{ID: user.ID, Name: user.Name, Email: user.Email, AvatarURL: user.AvatarURL}
}

type Register struct {
	users     repository.UserRepository
	passwords PasswordHasher
}

func NewRegister(users repository.UserRepository, passwords PasswordHasher) *Register {
	return &Register{users: users, passwords: passwords}
}

func normalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	address, err := mail.ParseAddress(email)
	if !utf8.ValidString(email) || len(email) > 254 || strings.ContainsRune(email, 0) || err != nil || address.Name != "" || address.Address != email {
		return "", ErrInvalidInput
	}
	return email, nil
}

func (u *Register) Execute(ctx context.Context, input RegisterInput) (PublicUser, error) {
	if err := ctx.Err(); err != nil {
		return PublicUser{}, err
	}
	name := strings.TrimSpace(input.Name)
	if !utf8.ValidString(name) || strings.ContainsRune(name, 0) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 100 {
		return PublicUser{}, ErrInvalidInput
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return PublicUser{}, err
	}
	if err := ValidatePassword(input.Password); err != nil {
		return PublicUser{}, ErrInvalidInput
	}
	hash, err := u.passwords.Hash(input.Password)
	if err != nil {
		return PublicUser{}, fmt.Errorf("hash registration password: %w", err)
	}
	// Rely on the unique constraint instead of a racy find-before-insert check.
	user, err := u.users.Create(ctx, repository.CreateUser{Name: name, Email: email, PasswordHash: hash})
	if err != nil {
		return PublicUser{}, fmt.Errorf("register user: %w", err)
	}
	return publicUser(user), nil
}
