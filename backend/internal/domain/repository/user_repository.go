package repository

import (
	"context"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
)

// CreateUser contains persisted input only. Validation, email normalization and
// password hashing belong to the calling use case.
type CreateUser struct {
	Name         string
	Email        string
	PasswordHash string `json:"-"`
}

type UserRepository interface {
	Create(ctx context.Context, input CreateUser) (entity.User, error)
	FindByEmail(ctx context.Context, email string) (entity.User, error)
}
