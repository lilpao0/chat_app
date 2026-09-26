package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
	"github.com/lilpao0/chat_app/backend/internal/testutil"
)

func TestUserJSONOmitsPasswordHash(t *testing.T) {
	for _, value := range []any{
		entity.User{PasswordHash: "private-hash"},
		domainrepo.CreateUser{PasswordHash: "private-hash"},
	} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "private-hash") || strings.Contains(string(data), "PasswordHash") {
			t.Fatal("password hash leaked through JSON")
		}
	}
}

func TestPostgresUserRepository(t *testing.T) {
	db := testutil.Database(t)
	ctx := context.Background()
	repo := NewPostgresUserRepository(db)
	input := domainrepo.CreateUser{
		FirstName: "An", LastName: "Nguyen", Email: "an@example.test",
		PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
	}
	created, err := repo.Create(ctx, input)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	expectedName := "An Nguyen"
	if created.ID <= 0 || created.Name != expectedName || created.Email != input.Email || created.PasswordHash != input.PasswordHash || created.AvatarURL != "" || created.CreatedAt.IsZero() {
		t.Fatal("created user fields or database defaults do not match")
	}
	found, err := repo.FindByEmail(ctx, input.Email)
	if err != nil {
		t.Fatal(err)
	}
	if found != created {
		t.Fatal("retrieved user differs from persisted user")
	}
	if _, err := repo.FindByEmail(ctx, "missing@example.test"); !errors.Is(err, entity.ErrUserNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if _, err := repo.FindByEmail(ctx, "' OR 1=1 --"); !errors.Is(err, entity.ErrUserNotFound) {
		t.Fatalf("email must be treated as data, got %v", err)
	}
	if _, err := repo.Create(ctx, input); !errors.Is(err, entity.ErrEmailTaken) {
		t.Fatalf("expected email conflict, got %v", err)
	}
	invalid := input
	invalid.Email = "other@example.test"
	invalid.FirstName = " "
	if _, err := repo.Create(ctx, invalid); err == nil || errors.Is(err, entity.ErrEmailTaken) {
		t.Fatal("non-unique constraint errors must not become email conflicts")
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := repo.FindByEmail(canceled, input.Email); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled lookup, got %v", err)
	}
	if _, err := repo.Create(canceled, input); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled insert, got %v", err)
	}
}
