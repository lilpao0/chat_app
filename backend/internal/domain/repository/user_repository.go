package repository

import (
	"context"
	"strings"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
)

// CreateUser contains persisted input only. Validation, email normalization and
// password hashing belong to the calling use case.
type CreateUser struct {
	FirstName    string
	LastName     string
	Email        string
	PasswordHash string `json:"-"`
}

func (c CreateUser) Name() string {
	name := strings.TrimSpace(c.FirstName + " " + c.LastName)
	if name == "" {
		return c.FirstName
	}
	return name
}

type UserRepository interface {
	Create(ctx context.Context, input CreateUser) (entity.User, error)
	FindByEmail(ctx context.Context, email string) (entity.User, error)
}
