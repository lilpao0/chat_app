package repository

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

var _ domainrepo.MediaRepository = (*PostgresAttachmentRepository)(nil)

func NewPostgresMediaRepository(db *sql.DB, sessionKey []byte) (*PostgresAttachmentRepository, error) {
	if len(sessionKey) != 32 {
		return nil, errors.New("media session key must contain 32 bytes")
	}
	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &PostgresAttachmentRepository{db: db, sessionCipher: aead}, nil
}

func sessionAAD(id, owner int64) []byte {
	return []byte(strconv.FormatInt(id, 10) + ":" + strconv.FormatInt(owner, 10))
}

func (r *PostgresAttachmentRepository) SaveSession(ctx context.Context, id, owner int64, session string) error {
	if r.sessionCipher == nil || id <= 0 || owner <= 0 || session == "" {
		return errors.New("invalid media session persistence configuration")
	}
	nonce := make([]byte, r.sessionCipher.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return errors.New("generate media session nonce")
	}
	encrypted := r.sessionCipher.Seal(nonce, nonce, []byte(session), sessionAAD(id, owner))
	result, err := r.db.ExecContext(ctx, `UPDATE attachments SET session_cipher=$3
		WHERE id=$1 AND owner_id=$2 AND state='uploading' AND session_cipher IS NULL
		AND expires_at>clock_timestamp()`, id, owner, encrypted)
	if err != nil {
		return fmt.Errorf("save media session: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return entity.ErrAttachmentUnavailable
	}
	return nil
}

const attachmentColumns = `a.id,a.conversation_id,a.owner_id,a.object_key,a.state,
	COALESCE(a.kind,''),a.declared_content_type,a.declared_size_bytes,
	COALESCE(a.object_generation,0),a.expires_at`

const ownedAttachmentQuery = `SELECT ` + attachmentColumns + ` FROM attachments a
	WHERE a.id=$1 AND a.owner_id=$2 AND EXISTS (
		SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=a.conversation_id AND cm.user_id=$2)`

func scanAttachment(row *sql.Row) (entity.AttachmentRecord, error) {
	var f entity.AttachmentRecord
	err := row.Scan(&f.ID, &f.ConversationID, &f.OwnerID, &f.ObjectKey, &f.State,
		&f.Kind, &f.ContentType, &f.SizeBytes, &f.Generation, &f.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return f, entity.ErrAttachmentNotFound
	}
	if err != nil {
		return f, fmt.Errorf("read attachment: %w", err)
	}
	f.ExpiresAt = f.ExpiresAt.UTC()
	return f, nil
}

func (r *PostgresAttachmentRepository) Owned(ctx context.Context, id, owner int64) (entity.AttachmentRecord, error) {
	return scanAttachment(r.db.QueryRowContext(ctx, ownedAttachmentQuery, id, owner))
}

func (r *PostgresAttachmentRepository) Accessible(ctx context.Context, id, actor int64) (entity.AttachmentRecord, error) {
	return scanAttachment(r.db.QueryRowContext(ctx, `SELECT `+attachmentColumns+` FROM attachments a
		WHERE a.id=$1 AND EXISTS (SELECT 1 FROM conversation_members cm
			WHERE cm.conversation_id=a.conversation_id AND cm.user_id=$2)
		AND (a.state='attached' OR (a.state='ready' AND a.owner_id=$2 AND a.expires_at>clock_timestamp()))`, id, actor))
}

func (r *PostgresAttachmentRepository) Complete(ctx context.Context, id, owner int64, verified entity.VerifiedMedia) (entity.AttachmentRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return entity.AttachmentRecord{}, err
	}
	defer tx.Rollback()
	f, err := scanAttachment(tx.QueryRowContext(ctx, ownedAttachmentQuery+` FOR UPDATE OF a`, id, owner))
	if err != nil {
		return f, err
	}
	var now time.Time
	if err := tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return f, err
	}
	if f.State == "attached" || f.State == "ready" {
		if f.State == "ready" && !now.Before(f.ExpiresAt) {
			return f, entity.ErrAttachmentExpired
		}
		return f, tx.Commit()
	}
	if f.State != "uploading" {
		return f, entity.ErrAttachmentUnavailable
	}
	if !now.Before(f.ExpiresAt) {
		return f, entity.ErrAttachmentExpired
	}
	if verified.Generation <= 0 || verified.ContentType != f.ContentType || verified.SizeBytes != f.SizeBytes {
		return f, entity.ErrInvalidMedia
	}
	_, err = tx.ExecContext(ctx, `UPDATE attachments SET state='ready', object_generation=$2,
		kind=$3,content_type=$4,size_bytes=$5 WHERE id=$1`, id, verified.Generation, verified.Kind, verified.ContentType, verified.SizeBytes)
	if err != nil {
		return f, fmt.Errorf("complete attachment: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return f, err
	}
	f.State, f.Kind, f.Generation = "ready", verified.Kind, verified.Generation
	return f, nil
}

func (r *PostgresAttachmentRepository) ClaimExpired(ctx context.Context, limit int) ([]entity.CleanupFile, error) {
	if limit < 1 || limit > 100 {
		return nil, entity.ErrInvalidInput
	}
	token := rand.Text()
	rows, err := r.db.QueryContext(ctx, `WITH candidates AS (
		SELECT id FROM attachments WHERE message_id IS NULL
		AND (cleanup_claimed_at IS NULL OR cleanup_claimed_at<clock_timestamp()-INTERVAL '5 minutes')
		AND ((state IN ('uploading','ready') AND expires_at<=clock_timestamp()) OR state IN ('failed','deleting')
			OR (state='deleted' AND (created_at>clock_timestamp()-INTERVAL '8 days' OR session_cipher IS NOT NULL)
				AND (cleanup_checked_at IS NULL OR cleanup_checked_at<clock_timestamp()-INTERVAL '1 hour')))
		ORDER BY expires_at,id LIMIT $1 FOR UPDATE SKIP LOCKED
	) UPDATE attachments a SET state='deleting',cleanup_token=$2,cleanup_claimed_at=clock_timestamp()
	FROM candidates c WHERE a.id=c.id
	RETURNING a.id,a.owner_id,a.object_key,COALESCE(a.object_generation,0),a.session_cipher`, limit, token)
	if err != nil {
		return nil, fmt.Errorf("claim media cleanup: %w", err)
	}
	defer rows.Close()
	files := make([]entity.CleanupFile, 0)
	for rows.Next() {
		var f entity.CleanupFile
		var owner int64
		var encrypted []byte
		if err := rows.Scan(&f.ID, &owner, &f.ObjectKey, &f.Generation, &encrypted); err != nil {
			return nil, err
		}
		f.ClaimToken = token
		if len(encrypted) > 0 {
			if r.sessionCipher == nil || len(encrypted) < r.sessionCipher.NonceSize() {
				return nil, errors.New("invalid encrypted media session")
			}
			n := r.sessionCipher.NonceSize()
			plain, err := r.sessionCipher.Open(nil, encrypted[:n], encrypted[n:], sessionAAD(f.ID, owner))
			if err != nil {
				return nil, errors.New("cannot decrypt media session")
			}
			f.SessionURL = string(plain)
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

func (r *PostgresAttachmentRepository) FinishCleanup(ctx context.Context, id int64, token string, success bool) error {
	state := "deleting"
	if success {
		state = "deleted"
	}
	_, err := r.db.ExecContext(ctx, `UPDATE attachments SET state=$3,cleanup_token=NULL,cleanup_claimed_at=NULL,
		cleanup_checked_at=CASE WHEN $3='deleted' THEN clock_timestamp() ELSE NULL END,
		session_cipher=CASE WHEN $3='deleted' AND created_at<=clock_timestamp()-INTERVAL '8 days' THEN NULL ELSE session_cipher END
		WHERE id=$1 AND cleanup_token=$2 AND state='deleting' AND message_id IS NULL`, id, token, state)
	if err != nil {
		return fmt.Errorf("finish media cleanup: %w", err)
	}
	return nil
}
