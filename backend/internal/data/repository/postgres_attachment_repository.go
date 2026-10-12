package repository

import (
	"context"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

type PostgresAttachmentRepository struct {
	db            *sql.DB
	sessionCipher cipher.AEAD
}

var _ domainrepo.AttachmentUploadRepository = (*PostgresAttachmentRepository)(nil)

func NewPostgresAttachmentRepository(db *sql.DB) *PostgresAttachmentRepository {
	return &PostgresAttachmentRepository{db: db}
}

func (r *PostgresAttachmentRepository) FailUpload(ctx context.Context, id, owner int64) error {
	if id <= 0 || owner <= 0 {
		return entity.ErrInvalidInput
	}
	_, err := r.db.ExecContext(ctx, `UPDATE attachments SET state='failed' WHERE id=$1 AND owner_id=$2 AND state='uploading' AND message_id IS NULL`, id, owner)
	if err != nil {
		return fmt.Errorf("mark upload failed: %w", err)
	}
	return nil
}

func (r *PostgresAttachmentRepository) IsMember(
	ctx context.Context,
	conversationID, userID int64,
) (bool, error) {
	var member bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM conversation_members
			WHERE conversation_id = $1 AND user_id = $2
		)
	`, conversationID, userID).Scan(&member)
	if err != nil {
		return false, fmt.Errorf("check upload membership: %w", err)
	}
	return member, nil
}

func (r *PostgresAttachmentRepository) ReserveUpload(
	ctx context.Context,
	conversationID, ownerID int64,
	contentType string,
	sizeBytes int64,
) (entity.UploadReservation, error) {
	if conversationID <= 0 || ownerID <= 0 {
		return entity.UploadReservation{}, entity.ErrInvalidInput
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return entity.UploadReservation{}, fmt.Errorf("begin upload reservation: %w", err)
	}
	defer tx.Rollback()

	// Serialize reservations across every conversation of this owner.
	var lockedID int64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM users WHERE id = $1 FOR UPDATE
	`, ownerID).Scan(&lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.UploadReservation{}, entity.ErrConversationNotFound
	}
	if err != nil {
		return entity.UploadReservation{}, fmt.Errorf("lock upload owner: %w", err)
	}

	// Recheck membership and prevent deletion until this transaction ends.
	err = tx.QueryRowContext(ctx, `
		SELECT user_id FROM conversation_members
		WHERE conversation_id = $1 AND user_id = $2
		FOR KEY SHARE
	`, conversationID, ownerID).Scan(&lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.UploadReservation{}, entity.ErrConversationNotFound
	}
	if err != nil {
		return entity.UploadReservation{}, fmt.Errorf("lock upload membership: %w", err)
	}

	// Read current DB time after waiting for locks.
	var now time.Time
	if err := tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return entity.UploadReservation{}, fmt.Errorf("read upload time: %w", err)
	}

	var pending int64
	var earliest sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT count(*), min(expires_at)
		FROM attachments
		WHERE owner_id = $1
		  AND state IN ('uploading', 'ready')
		  AND expires_at > $2
	`, ownerID, now).Scan(&pending, &earliest)
	if err != nil {
		return entity.UploadReservation{}, fmt.Errorf("count pending uploads: %w", err)
	}
	if pending >= 10 {
		// Round positive duration up to whole seconds.
		seconds := int64((earliest.Time.Sub(now)-1)/time.Second) + 1
		return entity.UploadReservation{}, &entity.UploadLimitError{
			RetryAfterSeconds: seconds,
		}
	}

	reservation := entity.UploadReservation{
		ObjectKey: "chat-media/" + rand.Text(),
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO attachments (
			conversation_id, owner_id, object_key,
			declared_content_type, declared_size_bytes,
			created_at, expires_at
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6::timestamptz, $6::timestamptz + INTERVAL '24 hours'
		)
		RETURNING id, expires_at
	`, conversationID, ownerID, reservation.ObjectKey,
		contentType, sizeBytes, now,
	).Scan(&reservation.ID, &reservation.ExpiresAt)
	if err != nil {
		return entity.UploadReservation{}, fmt.Errorf("insert upload reservation: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return entity.UploadReservation{}, fmt.Errorf("commit upload reservation: %w", err)
	}
	reservation.ExpiresAt = reservation.ExpiresAt.UTC()
	return reservation, nil
}
