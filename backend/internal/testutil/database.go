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

	"github.com/joho/godotenv"
	"github.com/lib/pq"
)

// Database creates a fresh private schema and removes only that schema at cleanup.
// It never resets public tables and refuses any DB other than chat_app_test.
func Database(t *testing.T) *sql.DB {
	t.Helper()
	raw := testDatabaseURL(t)
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
	for _, file := range []string{"000001_initial_schema.up.sql"} {
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

// testDatabaseURL prefers the process environment, then the backend .env file.
// When only DATABASE_URL is configured for local development, it derives the
// sibling chat_app_test database without ever allowing tests to use chat_app.
func testDatabaseURL(t *testing.T) string {
	t.Helper()
	if raw := os.Getenv("TEST_DATABASE_URL"); raw != "" {
		return raw
	}
	_, source, _, _ := runtime.Caller(0)
	values, err := godotenv.Read(filepath.Join(filepath.Dir(source), "..", "..", ".env"))
	if err != nil {
		t.Fatalf("TEST_DATABASE_URL is required and backend/.env could not be read: %v", err)
	}
	if raw := values["TEST_DATABASE_URL"]; raw != "" {
		return raw
	}
	developmentURL := values["DATABASE_URL"]
	u, err := url.Parse(developmentURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Path != "/chat_app" {
		t.Fatal("set TEST_DATABASE_URL to the isolated chat_app_test database")
	}
	u.Path = "/chat_app_test"
	return u.String()
}
