package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

// PublicUser deliberately contains no credentials, even inside the application.
type PublicUser struct {
	ID        int64
	Name      string
	Email     string
	AvatarURL string
}

func publicUser(user entity.User) PublicUser {
	return PublicUser{ID: user.ID, Name: user.Name, Email: user.Email, AvatarURL: user.AvatarURL}
}

type RegisterInput struct {
	FirstName, LastName, Email, Password string
}

type Register struct {
	users     repository.UserRepository
	passwords PasswordHasher
}

func NewRegister(users repository.UserRepository, passwords PasswordHasher) *Register {
	return &Register{users: users, passwords: passwords}
}

func normalizeRegisterInput(input RegisterInput) RegisterInput {
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	return input
}

func (u *Register) Execute(ctx context.Context, input RegisterInput) (PublicUser, error) {
	if err := ctx.Err(); err != nil {
		return PublicUser{}, err
	}

	input = normalizeRegisterInput(input)
	if err := ValidateInput(input.FirstName, input.LastName, input.Email, input.Password); err != nil {
		return PublicUser{}, err
	}

	hash, err := u.passwords.Hash(input.Password)
	if err != nil {
		return PublicUser{}, fmt.Errorf("hash registration password: %w", err)
	}

	// Rely on the unique constraint instead of a racy find-before-insert check.
	user, err := u.users.Create(ctx, repository.CreateUser{
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		Email:        input.Email,
		PasswordHash: hash,
	})
	if err != nil {
		return PublicUser{}, fmt.Errorf("register user: %w", err)
	}

	return publicUser(user), nil
}
