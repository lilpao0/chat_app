package testutil

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
)

func TestMessageAttachmentsMigration(t *testing.T) {
	db := Database(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := attachmentsMigration(t, db, "down"); err != nil {
		t.Fatal(err)
	}
	var owner, outsider, conversation, otherConversation, message, otherMessage int64
	for i, id := range []*int64{&owner, &outsider} {
		if err := db.QueryRowContext(ctx, `INSERT INTO users(first_name,email,password_hash) VALUES ('Fixture',$1,'fixture-hash') RETURNING id`, []string{"owner@example.test", "outsider@example.test"}[i]).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []*int64{&conversation, &otherConversation} {
		if err := db.QueryRowContext(ctx, `INSERT INTO conversations DEFAULT VALUES RETURNING id`).Scan(id); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO conversation_members(conversation_id,user_id) VALUES ($1,$2)`, *id, owner); err != nil {
			t.Fatal(err)
		}
	}
	var createdAt time.Time
	const requestID = "550e8400-e29b-41d4-a716-446655440000"
	if err := db.QueryRowContext(ctx, `INSERT INTO messages(conversation_id,sender_id,content,client_request_id) VALUES ($1,$2,' existing text ',$3::uuid) RETURNING id,created_at`, conversation, owner, requestID).Scan(&message, &createdAt); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO messages(conversation_id,sender_id,content) VALUES ($1,$2,'other') RETURNING id`, otherConversation, owner).Scan(&otherMessage); err != nil {
		t.Fatal(err)
	}
	if err := attachmentsMigration(t, db, "up"); err != nil {
		t.Fatal(err)
	}
	var content, key string
	var persistedAt time.Time
	if err := db.QueryRowContext(ctx, `SELECT content,created_at,client_request_id::text FROM messages WHERE id=$1`, message).Scan(&content, &persistedAt, &key); err != nil || content != " existing text " || !persistedAt.Equal(createdAt) || key != requestID {
		t.Fatalf("existing message changed: content=%q at=%v key=%q err=%v", content, persistedAt, key, err)
	}

	const insert = `INSERT INTO attachments(conversation_id,owner_id,object_key,declared_content_type,declared_size_bytes,state,object_generation,kind,content_type,size_bytes,message_id,position) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`
	t.Run("constraint boundaries", func(t *testing.T) {
		for _, test := range []struct {
			name string
			code pq.ErrorCode
			set  map[int]any
		}{
			{"outsider", "23503", map[int]any{1: outsider}},
			{"blank key", "23514", map[int]any{2: " "}},
			{"unsupported type", "23514", map[int]any{3: "image/heic"}},
			{"zero size", "23514", map[int]any{4: 0}},
			{"oversize image", "23514", map[int]any{4: 10485761}},
			{"oversize video", "23514", map[int]any{3: "video/mp4", 4: 52428801}},
			{"unknown state", "23514", map[int]any{5: "done"}},
			{"zero generation", "23514", map[int]any{6: 0}},
			{"partial metadata", "23514", map[int]any{7: "image"}},
			{"unverified ready", "23514", map[int]any{5: "ready"}},
			{"ready without generation", "23514", map[int]any{5: "ready", 7: "image", 8: "image/jpeg", 9: 64}},
			{"wrong kind", "23514", map[int]any{7: "video", 8: "image/jpeg", 9: 64}},
			{"type mismatch", "23514", map[int]any{7: "image", 8: "image/png", 9: 64}},
			{"size mismatch", "23514", map[int]any{7: "image", 8: "image/jpeg", 9: 65}},
			{"unbound attached", "23514", map[int]any{5: "attached"}},
			{"bound uploading", "23514", map[int]any{10: message, 11: 0}},
		} {
			t.Run(test.name, func(t *testing.T) {
				args := []any{conversation, owner, test.name, "image/jpeg", 64, "uploading", nil, nil, nil, nil, nil, nil}
				for index, value := range test.set {
					args[index] = value
				}
				_, err := db.ExecContext(ctx, insert, args...)
				assertAttachmentSQLError(t, err, test.code)
			})
		}
		for _, test := range []struct {
			kind string
			size int64
		}{{"image/jpeg", 1}, {"image/png", 10485760}, {"video/mp4", 52428800}} {
			if _, err := db.ExecContext(ctx, insert, conversation, owner, test.kind, test.kind, test.size, "uploading", nil, nil, nil, nil, nil, nil); err != nil {
				t.Fatal(err)
			}
		}
		_, err := db.ExecContext(ctx, `UPDATE attachments SET expires_at=created_at+INTERVAL '25 hours'`)
		assertAttachmentSQLError(t, err, "23514")
		_, err = db.ExecContext(ctx, insert, conversation, owner, "image/jpeg", "image/jpeg", 64, "uploading", nil, nil, nil, nil, nil, nil)
		assertAttachmentSQLError(t, err, "23505")
	})
	t.Run("binding order and count", func(t *testing.T) {
		for position := 0; position < 5; position++ {
			if _, err := db.ExecContext(ctx, insert, conversation, owner, "bound-"+string(rune('0'+position)), "image/jpeg", 64, "attached", 42, "image", "image/jpeg", 64, message, position); err != nil {
				t.Fatal(err)
			}
		}
		for _, test := range []struct {
			name string
			id   int64
			pos  int
			code pq.ErrorCode
		}{{"sixth", message, 5, "23514"}, {"negative", message, -1, "23514"}, {"duplicate", message, 4, "23505"}, {"wrong conversation", otherMessage, 0, "23503"}} {
			_, err := db.ExecContext(ctx, insert, conversation, owner, test.name, "image/jpeg", 64, "attached", 42, "image", "image/jpeg", 64, test.id, test.pos)
			assertAttachmentSQLError(t, err, test.code)
		}
	})
	t.Run("rollback refuses media", func(t *testing.T) {
		err := attachmentsMigration(t, db, "down")
		assertAttachmentSQLError(t, err, "P0001")
		if !strings.Contains(err.Error(), "while attachments exist") {
			t.Fatal(err)
		}
		var count int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM attachments`).Scan(&count); err != nil || count != 8 {
			t.Fatalf("rollback changed attachments: count=%d err=%v", count, err)
		}
	})
	// Only this test's private schema is modified; no development data is used.
	if _, err := db.ExecContext(ctx, `DELETE FROM attachments`); err != nil {
		t.Fatal(err)
	}
	t.Run("rollback waits for concurrent upload", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, insert, conversation, owner, "concurrent", "image/jpeg", 64, "uploading", nil, nil, nil, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- attachmentsMigration(t, db, "down") }()
		deadline := time.Now().Add(3 * time.Second)
		waiting := false
		for time.Now().Before(deadline) {
			if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='attachments'::regclass AND mode='AccessExclusiveLock' AND NOT granted)`).Scan(&waiting); err != nil {
				t.Fatal(err)
			}
			if waiting {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !waiting {
			t.Fatal("rollback did not lock out concurrent writes")
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			assertAttachmentSQLError(t, err, "P0001")
		case <-ctx.Done():
			t.Fatal("rollback did not finish")
		}
		var count int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM attachments WHERE object_key='concurrent'`).Scan(&count); err != nil || count != 1 {
			t.Fatalf("concurrent upload lost: count=%d err=%v", count, err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM attachments WHERE object_key='concurrent'`); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("rollback refuses blank messages", func(t *testing.T) {
		var blank int64
		if err := db.QueryRowContext(ctx, `INSERT INTO messages(conversation_id,sender_id,content) VALUES ($1,$2,'') RETURNING id`, conversation, owner).Scan(&blank); err != nil {
			t.Fatal(err)
		}
		err := attachmentsMigration(t, db, "down")
		assertAttachmentSQLError(t, err, "P0001")
		if !strings.Contains(err.Error(), "while blank messages exist") {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM messages WHERE id=$1`, blank); err != nil {
			t.Fatal(err)
		}
	})
	if err := attachmentsMigration(t, db, "down"); err != nil {
		t.Fatal(err)
	}
	_, err := db.ExecContext(ctx, `INSERT INTO messages(conversation_id,sender_id,content) VALUES ($1,$2,'')`, conversation, owner)
	assertAttachmentSQLError(t, err, "23514")
	if err := attachmentsMigration(t, db, "up"); err != nil {
		t.Fatal(err)
	}
	var indexes int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM pg_indexes WHERE schemaname=current_schema() AND tablename='attachments'`).Scan(&indexes); err != nil || indexes != 5 {
		t.Fatalf("expected PK, two unique and two partial indexes: count=%d err=%v", indexes, err)
	}
}

func attachmentsMigration(t *testing.T, db *sql.DB, direction string) error {
	t.Helper()
	return mediaMigration(t, db, "000004_message_attachments", direction)
}

func mediaMigration(t *testing.T, db *sql.DB, name, direction string) error {
	t.Helper()
	_, source, _, _ := runtime.Caller(0)
	body, err := os.ReadFile(filepath.Join(filepath.Dir(source), "..", "..", "migrations", name+"."+direction+".sql"))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.ExecContext(ctx, string(body))
	if err != nil {
		// A failed explicit BEGIN must not leave an aborted connection in the pool.
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if _, rollbackErr := conn.ExecContext(cleanup, "ROLLBACK"); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
	}
	return err
}

func assertAttachmentSQLError(t *testing.T, err error, want pq.ErrorCode) {
	t.Helper()
	var failure *pq.Error
	if !errors.As(err, &failure) || failure.Code != want {
		t.Fatalf("expected PostgreSQL %s, got %v", want, err)
	}
}
