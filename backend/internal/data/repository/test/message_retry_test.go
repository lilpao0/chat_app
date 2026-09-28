package repository_test

import (
	"context"
	"errors"
	datarepo "github.com/lilpao0/chat_app/backend/internal/data/repository"
	"github.com/lilpao0/chat_app/backend/internal/data/seed"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
	"github.com/lilpao0/chat_app/backend/internal/testutil"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestDurableMessageRetries(t *testing.T) {
	db := testutil.Database(t)
	ctx := context.Background()
	a := repository.CreateUser{FirstName: "A", Email: "retry-a@test.local", PasswordHash: "fixture"}
	b := repository.CreateUser{FirstName: "B", Email: "retry-b@test.local", PasswordHash: "fixture"}
	ab, err := seed.Demo(ctx, db, a, b)
	if err != nil {
		t.Fatal(err)
	}
	ac, err := seed.Demo(ctx, db, a, repository.CreateUser{FirstName: "C", Email: "retry-c@test.local", PasswordHash: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	r := datarepo.NewPostgresMessageRepository(db)
	const key = "550e8400-e29b-41d4-a716-446655440000"
	type result struct {
		m       entity.Message
		created bool
		err     error
	}
	results := make(chan result, 12)
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m, c, e := r.SendOnce(ctx, ab.ConversationID, ab.UserAID, "hello", key)
			results <- result{m, c, e}
		}()
	}
	wg.Wait()
	close(results)
	var id int64
	created := 0
	for got := range results {
		if got.err != nil {
			t.Fatal(got.err)
		}
		if id == 0 {
			id = got.m.ID
		}
		if id != got.m.ID {
			t.Fatal("retry produced different ID")
		}
		if got.created {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("created=%d", created)
	}
	for _, v := range []struct {
		conv int64
		text string
	}{{ab.ConversationID, "changed"}, {ac.ConversationID, "hello"}} {
		if _, _, err := r.SendOnce(ctx, v.conv, ab.UserAID, v.text, key); !errors.Is(err, entity.ErrRequestConflict) {
			t.Fatalf("conflict=%v", err)
		}
	}
	if _, _, err := r.SendOnce(ctx, ab.ConversationID, ac.UserBID, "hello", key); !errors.Is(err, entity.ErrConversationNotFound) {
		t.Fatalf("outsider=%v", err)
	}
	if m, c, err := r.SendOnce(ctx, ab.ConversationID, ab.UserBID, "hello", key); err != nil || !c || m.ID == id {
		t.Fatalf("sender scoped key: %v", err)
	}
	// Cross-conversation requests compete for the same sender key without duplicate persistence.
	raceKey := "550e8400-e29b-41d4-a716-446655440001"
	raced := make(chan error, 2)
	for _, conv := range []int64{ab.ConversationID, ac.ConversationID} {
		go func(conv int64) { _, _, err := r.SendOnce(ctx, conv, ab.UserAID, "race", raceKey); raced <- err }(conv)
	}
	successes, conflicts := 0, 0
	for range 2 {
		err := <-raced
		if err == nil {
			successes++
		} else if errors.Is(err, entity.ErrRequestConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal("cross-conversation race was not resolved")
	}
	// Down/up preserves messages while intentionally discarding retry metadata.
	var before, after int
	if err := db.QueryRow("SELECT count(*) FROM messages").Scan(&before); err != nil {
		t.Fatal(err)
	}
	_, source, _, _ := runtime.Caller(0)
	for _, direction := range []string{"down", "up"} {
		migration, err := os.ReadFile(filepath.Join(filepath.Dir(source), "../../../../migrations/000003_message_request_id."+direction+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.QueryRow("SELECT count(*) FROM messages").Scan(&after); err != nil || before != after {
		t.Fatal("migration lost messages")
	}
}
