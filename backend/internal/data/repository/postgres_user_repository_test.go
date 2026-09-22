package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
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
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to the isolated chat_app_test database")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal("cannot open test database")
	}
	defer db.Close()
	// One connection keeps the temporary table visible to every repository call.
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var databaseName string
	if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&databaseName); err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) {
			t.Fatalf("cannot connect to test database (PostgreSQL code %s)", pgErr.Code)
		}
		t.Fatal("cannot connect to test database")
	}
	if databaseName != "chat_app_test" {
		t.Fatal("repository integration tests require chat_app_test")
	}
	ddl, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "000001_create_users.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the real migration constraints without modifying persistent tables.
	temporaryDDL := strings.Replace(string(ddl), "CREATE TABLE users", "CREATE TEMP TABLE users", 1)
	if _, err := db.ExecContext(ctx, temporaryDDL); err != nil {
		t.Fatalf("create temporary users table: %v", err)
	}
	repo := NewPostgresUserRepository(db)
	input := domainrepo.CreateUser{
		Name: "An ' Nguyen", Email: "an@example.test",
		PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
	}
	created, err := repo.Create(ctx, input)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if created.ID <= 0 || created.Name != input.Name || created.Email != input.Email || created.PasswordHash != input.PasswordHash || created.AvatarURL != "" || created.CreatedAt.IsZero() {
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
	invalid.Name = " "
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
