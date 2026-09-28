package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

type PostgresUserRepository struct {
	db *sql.DB
}

var _ domainrepo.UserRepository = (*PostgresUserRepository)(nil)

func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) Create(ctx context.Context, input domainrepo.CreateUser) (entity.User, error) {
	var user entity.User
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO users (first_name, last_name, email, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id, first_name, last_name, email, password_hash, avatar_url, created_at`,
		input.FirstName, input.LastName, input.Email, input.PasswordHash,
	).Scan(&user.ID, &user.FirstName, &user.LastName, &user.Email, &user.PasswordHash, &user.AvatarURL, &user.CreatedAt)
	if err != nil {
		var pgErr *pq.Error
		// Only the email constraint represents an email conflict.
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.Constraint == "users_email_key" {
			return entity.User{}, entity.ErrEmailTaken
		}
		return entity.User{}, fmt.Errorf("create user: %w", err)
	}
	// Combine first_name and last_name for Name field
	if user.LastName == "" {
		user.Name = user.FirstName
	} else {
		user.Name = user.FirstName + " " + user.LastName
	}
	return user, nil
}

func (r *PostgresUserRepository) FindByEmail(ctx context.Context, email string) (entity.User, error) {
	var user entity.User
	err := r.db.QueryRowContext(ctx, `
		SELECT id, first_name, last_name, email, password_hash, avatar_url, created_at
		FROM users WHERE email = $1`, email,
	).Scan(&user.ID, &user.FirstName, &user.LastName, &user.Email, &user.PasswordHash, &user.AvatarURL, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.User{}, entity.ErrUserNotFound
	}
	if err != nil {
		return entity.User{}, fmt.Errorf("find user by email: %w", err)
	}
	// Combine first_name and last_name for Name field
	if user.LastName == "" {
		user.Name = user.FirstName
	} else {
		user.Name = user.FirstName + " " + user.LastName
	}
	return user, nil
}
