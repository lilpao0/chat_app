package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	datarepo "github.com/lilpao0/chat_app/backend/internal/data/repository"
	"github.com/lilpao0/chat_app/backend/internal/data/seed"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
	"github.com/lilpao0/chat_app/backend/internal/testutil"
)

func TestSendTransaction(t *testing.T) {
	db := testutil.Database(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	f, err := seed.Demo(ctx, db, repository.CreateUser{Name: "A", Email: "a@x.test", PasswordHash: "fixture-hash"}, repository.CreateUser{Name: "B", Email: "b@x.test", PasswordHash: "fixture-hash"})
	if err != nil {
		t.Fatal(err)
	}
	r := datarepo.NewPostgresMessageRepository(db)
	if _, err := r.Send(ctx, f.ConversationID, 999, "no access"); !errors.Is(err, entity.ErrConversationNotFound) {
		t.Fatal("outsider can send")
	}
	if _, err := r.Send(ctx, 999, f.UserAID, "missing"); !errors.Is(err, entity.ErrConversationNotFound) {
		t.Fatal("missing conversation accepted")
	}
	m, err := r.Send(ctx, f.ConversationID, f.UserAID, " hello \n")
	if err != nil || m.ID <= 0 || m.Content != " hello \n" || m.SenderID != f.UserAID {
		t.Fatal("message persistence failed")
	}
	var updated time.Time
	if err := db.QueryRowContext(ctx, `SELECT updated_at FROM conversations WHERE id=$1`, f.ConversationID).Scan(&updated); err != nil || updated.Before(m.CreatedAt) {
		t.Fatal("conversation not updated")
	}
	_, err = db.ExecContext(ctx, `CREATE FUNCTION fail_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test failure'; END $$; CREATE TRIGGER fail_update BEFORE UPDATE ON conversations FOR EACH ROW EXECUTE FUNCTION fail_update()`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Send(ctx, f.ConversationID, f.UserAID, "must rollback"); err == nil {
		t.Fatal("forced update failure ignored")
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM messages`).Scan(&count); err != nil || count != 1 {
		t.Fatal("failed send left a partial insert")
	}
	if _, err := db.ExecContext(ctx, `DROP TRIGGER fail_update ON conversations`); err != nil {
		t.Fatal(err)
	}
	// Hold the first writer open; the second must wait before consuming its ID.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT id FROM conversations WHERE id=$1 FOR UPDATE`, f.ConversationID); err != nil {
		t.Fatal(err)
	}
	var firstID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO messages(conversation_id,sender_id,content) VALUES ($1,$2,'first pending') RETURNING id`, f.ConversationID, f.UserAID).Scan(&firstID); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		message entity.Message
		err     error
	}
	done := make(chan outcome, 1)
	go func() { m, err := r.Send(ctx, f.ConversationID, f.UserBID, "second"); done <- outcome{m, err} }()
	deadline := time.Now().Add(3 * time.Second)
	waiting := false
	for time.Now().Before(deadline) {
		var n int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT c.id FROM conversations%'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			waiting = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("second writer did not wait on conversation lock")
	}
	var allocated int64
	if err := tx.QueryRowContext(ctx, `SELECT last_value FROM messages_id_seq`).Scan(&allocated); err != nil || allocated != firstID {
		t.Fatal("blocked writer allocated ID before acquiring lock")
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		if result.err != nil || result.message.ID <= firstID {
			t.Fatal("send order incorrect")
		}
	case <-ctx.Done():
		t.Fatal("second writer stuck")
	}
}
