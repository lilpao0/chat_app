package testutil

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const defaultAvatarURL = "https://clipart-library.com/img/1816203.png"

func TestDefaultUserAvatarMigration(t *testing.T) {
	db := Database(t)

	runMigrationFile(t, db, "000002_default_user_avatar.down.sql")
	insertMigrationUser(t, db, "empty@example.test", "")
	insertMigrationUser(t, db, "custom@example.test", "https://example.test/custom.png")

	runMigrationFile(t, db, "000002_default_user_avatar.up.sql")
	assertAvatar(t, db, "empty@example.test", defaultAvatarURL)
	assertAvatar(t, db, "custom@example.test", "https://example.test/custom.png")
	assertInsertedDefaultAvatar(t, db, "new-default@example.test", defaultAvatarURL)

	runMigrationFile(t, db, "000002_default_user_avatar.down.sql")
	assertInsertedDefaultAvatar(t, db, "restored-default@example.test", "")
}

func runMigrationFile(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	_, source, _, _ := runtime.Caller(0)
	body, err := os.ReadFile(filepath.Join(filepath.Dir(source), "..", "..", "migrations", name))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(), string(body)); err != nil {
		t.Fatalf("apply %s: %v", name, err)
	}
}

func insertMigrationUser(t *testing.T, db *sql.DB, email, avatarURL string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO users (first_name, last_name, email, password_hash, avatar_url)
		VALUES ('Migration', 'Fixture', $1, 'test-hash', $2)`, email, avatarURL)
	if err != nil {
		t.Fatalf("insert migration fixture: %v", err)
	}
}

func assertAvatar(t *testing.T, db *sql.DB, email, want string) {
	t.Helper()
	var got string
	if err := db.QueryRowContext(context.Background(), `SELECT avatar_url FROM users WHERE email = $1`, email).Scan(&got); err != nil {
		t.Fatalf("read avatar: %v", err)
	}
	if got != want {
		t.Fatalf("avatar for %s = %q, want %q", email, got, want)
	}
}

func assertInsertedDefaultAvatar(t *testing.T, db *sql.DB, email, want string) {
	t.Helper()
	var got string
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO users (first_name, last_name, email, password_hash)
		VALUES ('Migration', 'Default', $1, 'test-hash')
		RETURNING avatar_url`, email).Scan(&got)
	if err != nil {
		t.Fatalf("insert user with database default: %v", err)
	}
	if got != want {
		t.Fatalf("new avatar default = %q, want %q", got, want)
	}
}
