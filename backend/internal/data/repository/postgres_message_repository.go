package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type PostgresMessageRepository struct{ db *sql.DB }

var _ domainrepo.MessageWriter = (*PostgresMessageRepository)(nil)
var _ domainrepo.IdempotentMessageWriter = (*PostgresMessageRepository)(nil)

func NewPostgresMessageRepository(db *sql.DB) *PostgresMessageRepository {
	return &PostgresMessageRepository{db: db}
}

func (r *PostgresMessageRepository) Send(ctx context.Context, conversationID, userID int64, content string) (entity.Message, error) {
	message, _, err := r.SendOnce(ctx, conversationID, userID, content, "")
	return message, err
}

func (r *PostgresMessageRepository) SendOnce(ctx context.Context, conversationID, userID int64, content, requestID string) (entity.Message, bool, error) {
	return r.SendMediaOnce(ctx, conversationID, userID, content, requestID, nil)
}

var _ domainrepo.MediaMessageWriter = (*PostgresMessageRepository)(nil)

func (r *PostgresMessageRepository) SendMediaOnce(ctx context.Context, conversationID, userID int64, content, requestID string, attachmentIDs []int64) (entity.Message, bool, error) {
	if conversationID <= 0 || userID <= 0 || len(attachmentIDs) > 5 || (strings.TrimSpace(content) == "" && len(attachmentIDs) == 0) || !utf8.ValidString(content) || strings.ContainsRune(content, 0) || utf8.RuneCountInString(content) > 2000 || (requestID != "" && !entity.ValidRequestID(requestID)) {
		return entity.Message{}, false, entity.ErrInvalidInput
	}
	seen := make(map[int64]bool, len(attachmentIDs))
	for _, id := range attachmentIDs {
		if id <= 0 || seen[id] {
			return entity.Message{}, false, entity.ErrInvalidInput
		}
		seen[id] = true
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return entity.Message{}, false, fmt.Errorf("begin send: %w", err)
	}
	defer tx.Rollback()
	var locked int64
	err = tx.QueryRowContext(ctx, `SELECT c.id FROM conversations c WHERE c.id=$1 AND EXISTS(SELECT 1 FROM conversation_members cm WHERE cm.conversation_id=c.id AND cm.user_id=$2) FOR UPDATE OF c`, conversationID, userID).Scan(&locked)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Message{}, false, entity.ErrConversationNotFound
	}
	if err != nil {
		return entity.Message{}, false, fmt.Errorf("lock conversation: %w", err)
	}
	var message entity.Message
	// The unique index also serializes retries targeting different conversations.
	// DO NOTHING leaves the transaction usable; the next READ COMMITTED statement
	// sees the winning committed row without retrying an aborted transaction.
	err = tx.QueryRowContext(ctx, `INSERT INTO messages(conversation_id,sender_id,content,client_request_id) VALUES ($1,$2,$3,NULLIF($4,'')::uuid) ON CONFLICT (sender_id,client_request_id) DO NOTHING RETURNING id,conversation_id,sender_id,content,created_at`, conversationID, userID, content, requestID).Scan(&message.ID, &message.ConversationID, &message.SenderID, &message.Content, &message.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `SELECT id,conversation_id,sender_id,content,created_at FROM messages WHERE sender_id=$1 AND client_request_id=$2::uuid`, userID, requestID).Scan(&message.ID, &message.ConversationID, &message.SenderID, &message.Content, &message.CreatedAt)
		if err != nil {
			return entity.Message{}, false, err
		}
		if message.ConversationID != conversationID || message.Content != content {
			return entity.Message{}, false, entity.ErrRequestConflict
		}
		message.Attachments, err = messageAttachments(ctx, tx, message.ID)
		if err != nil {
			return entity.Message{}, false, err
		}
		original := make([]int64, len(message.Attachments))
		for i, a := range message.Attachments {
			original[i] = a.ID
		}
		if !slices.Equal(original, attachmentIDs) {
			return entity.Message{}, false, entity.ErrRequestConflict
		}
		if err := tx.Commit(); err != nil {
			return entity.Message{}, false, err
		}
		message.CreatedAt = message.CreatedAt.UTC()
		return message, false, nil
	}
	if err != nil {
		return entity.Message{}, false, fmt.Errorf("insert message: %w", err)
	}
	message.Attachments = make([]entity.Attachment, 0, len(attachmentIDs))
	if len(attachmentIDs) > 0 {
		rows, err := tx.QueryContext(ctx, `SELECT id,owner_id,state,expires_at,COALESCE(kind,''),COALESCE(content_type,''),COALESCE(size_bytes,0)
			FROM attachments WHERE conversation_id=$1 AND id=ANY($2::bigint[]) ORDER BY id FOR UPDATE`, conversationID, pq.Array(attachmentIDs))
		if err != nil {
			return entity.Message{}, false, err
		}
		type pendingAttachment struct {
			entity.Attachment
			owner  int64
			state  string
			expiry time.Time
		}
		files := make(map[int64]pendingAttachment, len(attachmentIDs))
		for rows.Next() {
			var a pendingAttachment
			if err := rows.Scan(&a.ID, &a.owner, &a.state, &a.expiry, &a.Kind, &a.ContentType, &a.SizeBytes); err != nil {
				rows.Close()
				return entity.Message{}, false, err
			}
			files[a.ID] = a
		}
		err = rows.Err()
		closeErr := rows.Close()
		if err != nil || closeErr != nil {
			return entity.Message{}, false, errors.Join(err, closeErr)
		}
		if len(files) != len(attachmentIDs) {
			return entity.Message{}, false, entity.ErrAttachmentNotFound
		}
		for _, a := range files {
			if a.owner != userID {
				return entity.Message{}, false, entity.ErrAttachmentNotFound
			}
		}
		var now time.Time
		if err := tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return entity.Message{}, false, err
		}
		for position, id := range attachmentIDs {
			a := files[id]
			if a.state == "uploading" {
				return entity.Message{}, false, entity.ErrAttachmentNotReady
			}
			if a.state != "ready" {
				return entity.Message{}, false, entity.ErrAttachmentUnavailable
			}
			if !now.Before(a.expiry) {
				return entity.Message{}, false, entity.ErrAttachmentExpired
			}
			if _, err := tx.ExecContext(ctx, `UPDATE attachments SET state='attached',message_id=$2,position=$3 WHERE id=$1`, id, message.ID, position); err != nil {
				return entity.Message{}, false, err
			}
			message.Attachments = append(message.Attachments, a.Attachment)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE conversations SET updated_at=GREATEST(updated_at,$2) WHERE id=$1`, conversationID, message.CreatedAt); err != nil {
		return entity.Message{}, false, fmt.Errorf("update conversation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return entity.Message{}, false, fmt.Errorf("commit message: %w", err)
	}
	message.CreatedAt = message.CreatedAt.UTC()
	return message, true, nil
}

func messageAttachments(ctx context.Context, tx *sql.Tx, messageID int64) ([]entity.Attachment, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,kind,content_type,size_bytes FROM attachments WHERE message_id=$1 AND state='attached' ORDER BY position`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]entity.Attachment, 0)
	for rows.Next() {
		var a entity.Attachment
		if err := rows.Scan(&a.ID, &a.Kind, &a.ContentType, &a.SizeBytes); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
