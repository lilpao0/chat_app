package migrations_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lib/pq"
)

func TestChatMigration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires chat_app_test")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal("cannot open test database")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var name string
	if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&name); err != nil || name != "chat_app_test" {
		t.Fatal("requires chat_app_test")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	schema := pq.QuoteIdentifier(fmt.Sprintf("migration_test_%d", time.Now().UnixNano()))
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE SCHEMA " + schema)
	exec("SET LOCAL search_path TO " + schema)
	apply := func(name string) {
		t.Helper()
		body, err := os.ReadFile(filepath.Join("..", name))
		if err != nil {
			t.Fatal(err)
		}
		exec(string(body))
	}
	apply("000001_create_users.up.sql")
	apply("000002_create_chat.up.sql")
	// The down migration removes only chat tables; the users migration survives.
	apply("000002_create_chat.down.sql")
	apply("000002_create_chat.up.sql")
	exec(`INSERT INTO users(name,email,password_hash) VALUES ('A','a@test.local','fixture-hash'),('B','b@test.local','fixture-hash'),('C','c@test.local','fixture-hash')`)
	exec(`INSERT INTO conversations DEFAULT VALUES`)
	exec(`INSERT INTO conversations DEFAULT VALUES`)
	exec(`INSERT INTO conversation_members(conversation_id,user_id) VALUES (1,1),(1,2),(2,1)`)
	reject := func(query, code string) {
		t.Helper()
		exec("SAVEPOINT invalid_input")
		_, err := tx.ExecContext(ctx, query)
		var pg *pq.Error
		if !errors.As(err, &pg) || string(pg.Code) != code {
			t.Fatalf("expected SQLSTATE %s, got %v", code, err)
		}
		exec("ROLLBACK TO SAVEPOINT invalid_input")
		exec("RELEASE SAVEPOINT invalid_input")
	}
	reject(`INSERT INTO conversation_members(conversation_id,user_id) VALUES (1,1)`, "23505")
	reject(`INSERT INTO conversation_members(conversation_id,user_id) VALUES (99,1)`, "23503")
	reject(`INSERT INTO conversation_members(conversation_id,user_id) VALUES (1,99)`, "23503")
	reject(`INSERT INTO messages(conversation_id,sender_id,content) VALUES (1,3,'outsider')`, "23503")
	reject(`INSERT INTO messages(conversation_id,sender_id,content) VALUES (1,1,E' \t\n')`, "23514")
	reject(`INSERT INTO messages(conversation_id,sender_id,content) VALUES (1,1,repeat('a',2001))`, "23514")
	exec(`SELECT id FROM conversations WHERE id=1 FOR UPDATE`)
	var messageID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO messages(conversation_id,sender_id,content) VALUES (1,1,'hello') RETURNING id`).Scan(&messageID); err != nil {
		t.Fatal(err)
	}
	reject(fmt.Sprintf(`UPDATE conversation_members SET last_read_message_id=%d,last_read_at=now() WHERE conversation_id=2`, messageID), "23503")
	reject(`UPDATE conversation_members SET last_read_message_id=99999,last_read_at=now() WHERE conversation_id=1`, "23503")
	reject(`UPDATE conversation_members SET last_read_at=now() WHERE conversation_id=1`, "23514")
	exec(`UPDATE conversation_members SET last_read_message_id=$1,last_read_at=clock_timestamp() WHERE conversation_id=1 AND user_id=2`, messageID)
	var marker int64
	if err := tx.QueryRowContext(ctx, `SELECT last_read_message_id FROM conversation_members WHERE conversation_id=1 AND user_id=2`).Scan(&marker); err != nil || marker != messageID {
		t.Fatal("valid read marker was not saved")
	}
	var cache int64
	var cycle bool
	if err := tx.QueryRowContext(ctx, `SELECT seqcache,seqcycle FROM pg_sequence WHERE seqrelid=pg_get_serial_sequence('messages','id')::regclass`).Scan(&cache, &cycle); err != nil || cache != 1 || cycle {
		t.Fatal("message sequence must use CACHE 1 NO CYCLE")
	}
	// Rollback drops the private schema and every fixture; public tables are untouched.
}
