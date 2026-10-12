package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	datarepo "github.com/lilpao0/chat_app/backend/internal/data/repository"
	"github.com/lilpao0/chat_app/backend/internal/data/seed"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
	"github.com/lilpao0/chat_app/backend/internal/testutil"
)

func TestReserveUploadConcurrentCap(t *testing.T) {
	db := testutil.Database(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	owner := domainrepo.CreateUser{
		FirstName: "A", Email: "upload-a@test.local", PasswordHash: "fixture-hash",
	}
	first, err := seed.Demo(ctx, db, owner, domainrepo.CreateUser{
		FirstName: "B", Email: "upload-b@test.local", PasswordHash: "fixture-hash",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := seed.Demo(ctx, db, owner, domainrepo.CreateUser{
		FirstName: "C", Email: "upload-c@test.local", PasswordHash: "fixture-hash",
	})
	if err != nil {
		t.Fatal(err)
	}

	r := datarepo.NewPostgresAttachmentRepository(db)
	for i := 0; i < 9; i++ {
		if _, err := r.ReserveUpload(ctx, first.ConversationID, first.UserAID, "image/jpeg", 100); err != nil {
			t.Fatal(err)
		}
	}

	const attempts = 8
	start := make(chan struct{})
	results := make(chan error, attempts)

	for i := 0; i < attempts; i++ {
		conversationID := first.ConversationID
		if i%2 == 1 {
			conversationID = second.ConversationID
		}
		go func(c int64) {
			<-start
			_, err := r.ReserveUpload(ctx, c, first.UserAID, "image/jpeg", 100)
			results <- err
		}(conversationID)
	}
	close(start)

	successes := 0
	for i := 0; i < attempts; i++ {
		select {
		case err := <-results:
			if err == nil {
				successes++
				continue
			}
			var limit *entity.UploadLimitError
			if !errors.As(err, &limit) || limit.RetryAfterSeconds < 1 {
				t.Fatalf("unexpected reservation error: %v", err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	if successes != 1 {
		t.Fatalf("successful reservations=%d, want 1", successes)
	}

	var count int
	if err := db.QueryRowContext(ctx, `
		SELECT count(*) FROM attachments WHERE owner_id = $1
	`, first.UserAID).Scan(&count); err != nil || count != 10 {
		t.Fatalf("attachments=%d err=%v, want 10", count, err)
	}

	// Authorization still takes precedence when the owner's slots are full.
	_, err = r.ReserveUpload(ctx, first.ConversationID, first.UserBID, "image/jpeg", 100)
	if err != nil {
		t.Fatalf("another owner's slots were affected: %v", err)
	}
	_, err = r.ReserveUpload(ctx, second.ConversationID, first.UserBID, "image/jpeg", 100)
	if !errors.Is(err, entity.ErrConversationNotFound) {
		t.Fatalf("outsider error=%v", err)
	}
}
