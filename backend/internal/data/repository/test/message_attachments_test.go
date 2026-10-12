package repository_test

import (
	"context"
	"testing"
	"time"

	datarepo "github.com/lilpao0/chat_app/backend/internal/data/repository"
	"github.com/lilpao0/chat_app/backend/internal/data/seed"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
	"github.com/lilpao0/chat_app/backend/internal/testutil"
)

func TestHistoryAttachments(t *testing.T) {
	db := testutil.Database(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	f, err := seed.Demo(ctx, db,
		repository.CreateUser{
			FirstName: "A", Email: "a@media.test", PasswordHash: "fixture-hash",
		},
		repository.CreateUser{
			FirstName: "B", Email: "b@media.test", PasswordHash: "fixture-hash",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	r := datarepo.NewPostgresMessageRepository(db)
	text, err := r.Send(ctx, f.ConversationID, f.UserAID, "text only")
	if err != nil {
		t.Fatal(err)
	}
	media, err := r.Send(ctx, f.ConversationID, f.UserAID, "two images")
	if err != nil {
		t.Fatal(err)
	}

	// Insert out of order to prove history sorts by position, not ID.
	var attachmentIDs [2]int64
	for _, position := range []int{1, 0} {
		err := db.QueryRowContext(ctx, `
			INSERT INTO attachments (
				conversation_id, owner_id, message_id, position,
				state, object_key, object_generation,
				declared_content_type, declared_size_bytes,
				kind, content_type, size_bytes
			)
			VALUES (
				$1, $2, $3, $4::smallint,
				'attached', 'history-' || ($4::smallint)::text, 1,
				'image/jpeg', 10,
				'image', 'image/jpeg', 10
			)
			RETURNING id
		`, f.ConversationID, f.UserAID, media.ID, position).
			Scan(&attachmentIDs[position])
		if err != nil {
			t.Fatal(err)
		}
	}

	page, err := r.History(ctx, f.ConversationID, f.UserBID,
		entity.MessageQuery{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != media.ID ||
		!page.HasMore || page.NextBeforeID == nil ||
		*page.NextBeforeID != media.ID {
		t.Fatalf("wrong pagination: %+v", page)
	}

	attachments := page.Items[0].Attachments
	if len(attachments) != 2 ||
		attachments[0].ID != attachmentIDs[0] ||
		attachments[1].ID != attachmentIDs[1] ||
		attachments[0].Kind != "image" ||
		attachments[0].ContentType != "image/jpeg" ||
		attachments[0].SizeBytes != 10 {
		t.Fatalf("wrong attachments: %+v", attachments)
	}

	page, err = r.History(ctx, f.ConversationID, f.UserBID,
		entity.MessageQuery{BeforeID: media.ID, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != text.ID ||
		page.Items[0].Attachments == nil ||
		len(page.Items[0].Attachments) != 0 {
		t.Fatalf("wrong text page: %+v", page)
	}

	page, err = r.History(ctx, f.ConversationID, 999999999,
		entity.MessageQuery{Limit: 1})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("outsider history: page=%+v err=%v", page, err)
	}
}
