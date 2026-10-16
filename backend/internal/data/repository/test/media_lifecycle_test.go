package repository_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/lib/pq"
	"reflect"
	"sync"
	"testing"

	datarepo "github.com/lilpao0/chat_app/backend/internal/data/repository"
	"github.com/lilpao0/chat_app/backend/internal/data/seed"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
	"github.com/lilpao0/chat_app/backend/internal/testutil"
)

func TestMediaLifecycleBindingRetryAndCleanup(t *testing.T) {
	db := testutil.Database(t)
	ctx := context.Background()
	owner := repository.CreateUser{FirstName: "A", Email: "media-a@test.local", PasswordHash: "fixture-hash"}
	ab, err := seed.Demo(ctx, db, owner, repository.CreateUser{FirstName: "B", Email: "media-b@test.local", PasswordHash: "fixture-hash"})
	if err != nil {
		t.Fatal(err)
	}
	ac, err := seed.Demo(ctx, db, owner, repository.CreateUser{FirstName: "C", Email: "media-c@test.local", PasswordHash: "fixture-hash"})
	if err != nil {
		t.Fatal(err)
	}
	files, err := datarepo.NewPostgresMediaRepository(db, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	messages := datarepo.NewPostgresMessageRepository(db)
	prepare := func(owner, conversation int64, kind, contentType string) entity.UploadReservation {
		t.Helper()
		r, err := files.ReserveUpload(ctx, conversation, owner, contentType, 100)
		if err != nil {
			t.Fatal(err)
		}
		if err := files.SaveSession(ctx, r.ID, owner, "https://storage.googleapis.com/upload/storage/v1/b/test/o?upload_id=test-secret"); err != nil {
			t.Fatal(err)
		}
		f, err := files.Complete(ctx, r.ID, owner, entity.VerifiedMedia{Kind: kind, ContentType: contentType, SizeBytes: 100, Generation: 7})
		if err != nil || f.State != "ready" {
			t.Fatalf("completion error=%v", err)
		}
		return r
	}
	image := prepare(ab.UserAID, ab.ConversationID, "image", "image/jpeg")
	video := prepare(ab.UserAID, ab.ConversationID, "video", "video/mp4")
	foreign := prepare(ab.UserBID, ab.ConversationID, "image", "image/png")
	wrongConversation := prepare(ab.UserAID, ac.ConversationID, "image", "image/png")
	if _, err := files.Accessible(ctx, image.ID, ab.UserBID); !errors.Is(err, entity.ErrAttachmentNotFound) {
		t.Fatal("recipient previewed unsent media")
	}
	if _, err := files.Accessible(ctx, image.ID, ab.UserAID); err != nil {
		t.Fatal(err)
	}
	if _, err := files.Owned(ctx, image.ID, ac.UserBID); !errors.Is(err, entity.ErrAttachmentNotFound) {
		t.Fatal("outsider completed another owner's upload")
	}
	for _, id := range []int64{foreign.ID, wrongConversation.ID, 9999999} {
		if _, _, err := messages.SendMediaOnce(ctx, ab.ConversationID, ab.UserAID, "hello", "", []int64{image.ID, id}); !errors.Is(err, entity.ErrAttachmentNotFound) {
			t.Fatalf("inaccessible binding error=%v", err)
		}
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM messages`).Scan(&count); err != nil || count != 0 {
		t.Fatal("rejected binding left a partial message")
	}
	const key = "550e8400-e29b-41d4-a716-446655440000"
	ids := []int64{video.ID, image.ID}
	message, created, err := messages.SendMediaOnce(ctx, ab.ConversationID, ab.UserAID, "", key, ids)
	if err != nil || !created || len(message.Attachments) != 2 || message.Attachments[0].ID != video.ID {
		t.Fatalf("media send error=%v", err)
	}
	if _, err := files.Accessible(ctx, image.ID, ab.UserBID); err != nil {
		t.Fatal("recipient cannot view attached media")
	}
	if _, err := files.Accessible(ctx, image.ID, ac.UserBID); !errors.Is(err, entity.ErrAttachmentNotFound) {
		t.Fatal("outsider can view media")
	}
	if _, err := db.ExecContext(ctx, `UPDATE attachments SET created_at=now()-INTERVAL '25 hours',expires_at=now()-INTERVAL '1 hour' WHERE id=ANY($1::bigint[])`, pq.Array(ids)); err != nil {
		t.Fatal(err)
	}
	replay, created, err := messages.SendMediaOnce(ctx, ab.ConversationID, ab.UserAID, "", key, ids)
	if err != nil || created || !reflect.DeepEqual(message, replay) {
		t.Fatalf("durable replay failed: %v", err)
	}
	if _, _, err := messages.SendMediaOnce(ctx, ab.ConversationID, ab.UserAID, "", key, []int64{image.ID, video.ID}); !errors.Is(err, entity.ErrRequestConflict) {
		t.Fatal("reordered retry accepted")
	}
	if _, _, err := messages.SendMediaOnce(ctx, ab.ConversationID, ab.UserAID, "changed", key, ids); !errors.Is(err, entity.ErrRequestConflict) {
		t.Fatal("changed text retry accepted")
	}
	if _, _, err := messages.SendMediaOnce(ctx, ab.ConversationID, ab.UserAID, "reuse", "", []int64{image.ID}); !errors.Is(err, entity.ErrAttachmentUnavailable) {
		t.Fatal("bound media reused")
	}
	page, err := messages.History(ctx, ab.ConversationID, ab.UserBID, entity.MessageQuery{Limit: 1})
	if err != nil || len(page.Items) != 1 || !reflect.DeepEqual(page.Items[0].Attachments, message.Attachments) {
		t.Fatal("history lost attachment order")
	}
	chats, err := datarepo.NewPostgresConversationRepository(db).ListForUser(ctx, ab.UserBID)
	if err != nil || len(chats) != 1 || chats[0].LastMessage == nil || *chats[0].LastMessage != "Media" || chats[0].UnreadCount != 1 {
		t.Fatal("preview or unread changed per attachment")
	}
	if _, err := files.Complete(ctx, image.ID, ab.UserAID, entity.VerifiedMedia{}); err != nil {
		t.Fatal("attached completion failed after pending expiry")
	}
	failed, err := files.ReserveUpload(ctx, ab.ConversationID, ab.UserAID, "image/jpeg", 100)
	if err != nil {
		t.Fatal(err)
	}
	const capability = "https://storage.googleapis.com/upload/storage/v1/b/test/o?upload_id=cleanup-secret"
	if err := files.SaveSession(ctx, failed.ID, ab.UserAID, capability); err != nil {
		t.Fatal(err)
	}
	var encrypted []byte
	if err := db.QueryRowContext(ctx, `SELECT session_cipher FROM attachments WHERE id=$1`, failed.ID).Scan(&encrypted); err != nil || bytes.Contains(encrypted, []byte("cleanup-secret")) {
		t.Fatal("session persisted as plaintext")
	}
	if err := files.FailUpload(ctx, failed.ID, ab.UserAID); err != nil {
		t.Fatal(err)
	}
	claimed, err := files.ClaimExpired(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].ID != failed.ID || claimed[0].SessionURL != capability {
		t.Fatal("cleanup claimed attached media or lost recovery handle")
	}
	if next, err := files.ClaimExpired(ctx, 10); err != nil || len(next) != 0 {
		t.Fatal("live cleanup lease was stolen")
	}
	if err := files.FinishCleanup(ctx, failed.ID, "stale-claim", true); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := db.QueryRowContext(ctx, `SELECT state FROM attachments WHERE id=$1`, failed.ID).Scan(&state); err != nil || state != "deleting" {
		t.Fatal("stale cleanup completed newer claim")
	}
	if err := files.FinishCleanup(ctx, failed.ID, claimed[0].ClaimToken, true); err != nil {
		t.Fatal(err)
	}
	if next, err := files.ClaimExpired(ctx, 10); err != nil || len(next) != 0 {
		t.Fatal("tombstone reconciled too often")
	}
}

func TestConcurrentBindingAndPendingExpiry(t *testing.T) {
	db := testutil.Database(t)
	ctx := context.Background()
	f, err := seed.Demo(ctx, db, repository.CreateUser{FirstName: "A", Email: "race-a@test.local", PasswordHash: "fixture-hash"}, repository.CreateUser{FirstName: "B", Email: "race-b@test.local", PasswordHash: "fixture-hash"})
	if err != nil {
		t.Fatal(err)
	}
	files := datarepo.NewPostgresAttachmentRepository(db)
	r, err := files.ReserveUpload(ctx, f.ConversationID, f.UserAID, "image/jpeg", 100)
	if err != nil {
		t.Fatal(err)
	}
	messages := datarepo.NewPostgresMessageRepository(db)
	if _, _, err := messages.SendMediaOnce(ctx, f.ConversationID, f.UserAID, "", "", []int64{r.ID}); !errors.Is(err, entity.ErrAttachmentNotReady) {
		t.Fatal("unfinished media bound")
	}
	if _, err := files.Complete(ctx, r.ID, f.UserAID, entity.VerifiedMedia{Kind: "image", ContentType: "image/jpeg", SizeBytes: 100, Generation: 1}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := messages.SendMediaOnce(ctx, f.ConversationID, f.UserAID, "", "", []int64{r.ID})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, entity.ErrAttachmentUnavailable) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("bindings=%d want=1", success)
	}
	expired, err := files.ReserveUpload(ctx, f.ConversationID, f.UserAID, "image/jpeg", 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := files.Complete(ctx, expired.ID, f.UserAID, entity.VerifiedMedia{Kind: "image", ContentType: "image/jpeg", SizeBytes: 100, Generation: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE attachments SET created_at=now()-INTERVAL '25 hours',expires_at=now()-INTERVAL '1 hour' WHERE id=$1`, expired.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := messages.SendMediaOnce(ctx, f.ConversationID, f.UserAID, "", "", []int64{expired.ID}); !errors.Is(err, entity.ErrAttachmentExpired) {
		t.Fatal("expired ready media bound")
	}
	// A busy expired row must be skipped and remain eligible for a later pass.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT id FROM attachments WHERE id=$1 FOR UPDATE`, expired.ID); err != nil {
		t.Fatal(err)
	}
	if claimed, err := files.ClaimExpired(ctx, 10); err != nil || len(claimed) != 0 {
		t.Fatalf("cleanup must skip locked media: claimed=%v err=%v", claimed, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	// Cleanup, completion and a new send compete for the same expired row.
	start := make(chan struct{})
	errResults := make(chan error, 2)
	cleanupResults := make(chan []entity.CleanupFile, 1)
	go func() {
		<-start
		_, _, err := messages.SendMediaOnce(ctx, f.ConversationID, f.UserAID, "", "", []int64{expired.ID})
		errResults <- err
	}()
	go func() {
		<-start
		_, err := files.Complete(ctx, expired.ID, f.UserAID, entity.VerifiedMedia{})
		errResults <- err
	}()
	go func() {
		<-start
		claimed, err := files.ClaimExpired(ctx, 10)
		if err != nil {
			t.Errorf("cleanup race: %v", err)
		}
		cleanupResults <- claimed
	}()
	close(start)
	for range 2 {
		err := <-errResults
		if !errors.Is(err, entity.ErrAttachmentExpired) && !errors.Is(err, entity.ErrAttachmentUnavailable) {
			t.Fatalf("expired media resurrected during cleanup: %v", err)
		}
	}
	claimed := <-cleanupResults
	// SKIP LOCKED may defer the row while send or completion holds its lock.
	// Both operations have finished, so the next pass must claim any deferred row.
	next, err := files.ClaimExpired(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	claimed = append(claimed, next...)
	if len(claimed) != 1 || claimed[0].ID != expired.ID {
		t.Fatalf("cleanup claims=%v want only expired media %d", claimed, expired.ID)
	}
	var state string
	var messageCount int
	if err := db.QueryRow(`SELECT state FROM attachments WHERE id=$1`, expired.ID).Scan(&state); err != nil || state != "deleting" {
		t.Fatal("cleanup lost claim during race")
	}
	if err := db.QueryRow(`SELECT count(*) FROM messages`).Scan(&messageCount); err != nil || messageCount != 1 {
		t.Fatal("cleanup race left partial message")
	}
}

func TestCleanupSessionRecoveryAfterRestart(t *testing.T) {
	db := testutil.Database(t)
	ctx := context.Background()
	f, err := seed.Demo(ctx, db, repository.CreateUser{FirstName: "A", Email: "recover-a@test.local", PasswordHash: "fixture"}, repository.CreateUser{FirstName: "B", Email: "recover-b@test.local", PasswordHash: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{9}, 32)
	files, err := datarepo.NewPostgresMediaRepository(db, key)
	if err != nil {
		t.Fatal(err)
	}
	r, err := files.ReserveUpload(ctx, f.ConversationID, f.UserAID, "image/png", 100)
	if err != nil {
		t.Fatal(err)
	}
	const session = "https://storage.googleapis.com/upload/storage/v1/b/test/o?upload_id=recovery-fixture"
	if err := files.SaveSession(ctx, r.ID, f.UserAID, session); err != nil {
		t.Fatal(err)
	}
	if err := files.FailUpload(ctx, r.ID, f.UserAID); err != nil {
		t.Fatal(err)
	}
	var ciphertext []byte
	if err := db.QueryRow(`SELECT session_cipher FROM attachments WHERE id=$1`, r.ID).Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	corrupt := bytes.Clone(ciphertext)
	corrupt[len(corrupt)-1] ^= 1
	if _, err := db.Exec(`UPDATE attachments SET session_cipher=$2 WHERE id=$1`, r.ID, corrupt); err != nil {
		t.Fatal(err)
	}
	if _, err := files.ClaimExpired(ctx, 10); err == nil {
		t.Fatal("tampered session accepted")
	}
	if _, err := db.Exec(`UPDATE attachments SET session_cipher=$2,cleanup_claimed_at=now()-INTERVAL '6 minutes' WHERE id=$1`, r.ID, ciphertext); err != nil {
		t.Fatal(err)
	}
	// Reconstruct adapter to prove ciphertext survives process restart.
	files, err = datarepo.NewPostgresMediaRepository(db, key)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := files.ClaimExpired(ctx, 10)
	if err != nil || len(claimed) != 1 || claimed[0].SessionURL != session {
		t.Fatal("recovery lost encrypted capability")
	}
	if err := files.FinishCleanup(ctx, r.ID, claimed[0].ClaimToken, false); err != nil {
		t.Fatal(err)
	}
	claimed, err = files.ClaimExpired(ctx, 10)
	if err != nil || len(claimed) != 1 {
		t.Fatal("failed cleanup not retryable")
	}
	if err := files.FinishCleanup(ctx, r.ID, claimed[0].ClaimToken, true); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE attachments SET created_at=now()-INTERVAL '9 days',expires_at=now()-INTERVAL '8 days',cleanup_checked_at=now()-INTERVAL '2 hours' WHERE id=$1`, r.ID); err != nil {
		t.Fatal(err)
	}
	claimed, err = files.ClaimExpired(ctx, 10)
	if err != nil || len(claimed) != 1 {
		t.Fatal("final reconciliation lost recovery handle")
	}
	if err := files.FinishCleanup(ctx, r.ID, claimed[0].ClaimToken, true); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT session_cipher FROM attachments WHERE id=$1`, r.ID).Scan(&ciphertext); err != nil || ciphertext != nil {
		t.Fatal("old recovery credential retained after final reconciliation")
	}
	if claimed, err := files.ClaimExpired(ctx, 10); err != nil || len(claimed) != 0 {
		t.Fatal("old tombstone reconciled forever")
	}
}
