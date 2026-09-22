// Package testutil provides isolated PostgreSQL fixtures for integration tests.
package testutil

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/lib/pq"
)

// Database creates a fresh private schema and removes only that schema at cleanup.
// It never resets public tables and refuses any DB other than chat_app_test.
func Database(t *testing.T) *sql.DB {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("set TEST_DATABASE_URL to chat_app_test")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("TEST_DATABASE_URL must be a PostgreSQL URL")
	}
	admin, err := sql.Open("postgres", raw)
	if err != nil {
		t.Fatal("cannot open test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var name string
	if err := admin.QueryRowContext(ctx, "SELECT current_database()").Scan(&name); err != nil || name != "chat_app_test" {
		admin.Close()
		t.Fatal("requires chat_app_test")
	}
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	schema := "test_" + hex.EncodeToString(random[:])
	quoted := pq.QuoteIdentifier(schema)
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer admin.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(ctx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Errorf("cannot remove test schema: %v", err)
		}
	})
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal("cannot open isolated connection")
	}
	t.Cleanup(func() { db.Close() })
	_, source, _, _ := runtime.Caller(0)
	for _, file := range []string{"000001_create_users.up.sql", "000002_create_chat.up.sql"} {
		body, err := os.ReadFile(filepath.Join(filepath.Dir(source), "..", "..", "migrations", file))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, string(body)); err != nil {
			t.Fatal(err)
		}
	}
	return db
}
