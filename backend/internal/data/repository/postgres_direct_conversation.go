package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

var _ domainrepo.DirectConversationOpener = (*PostgresConversationRepository)(nil)

func (r *PostgresConversationRepository) OpenDirect(ctx context.Context, actorID, otherID int64) (entity.DirectConversation, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return entity.DirectConversation{}, fmt.Errorf("begin direct conversation: %w", err)
	}
	defer tx.Rollback()
	result, err := OpenDirectInTx(ctx, tx, actorID, otherID)
	if err != nil {
		return entity.DirectConversation{}, err
	}
	if err := tx.Commit(); err != nil {
		return entity.DirectConversation{}, fmt.Errorf("commit direct conversation: %w", err)
	}
	return result, nil
}

// OpenDirectInTx is shared by the API and seed so every application creation path
// uses the same unordered-pair lock. The caller owns commit/rollback.
func OpenDirectInTx(ctx context.Context, tx *sql.Tx, actorID, otherID int64) (entity.DirectConversation, error) {
	if actorID <= 0 || otherID <= 0 || actorID == otherID {
		return entity.DirectConversation{}, entity.ErrInvalidInput
	}
	low, high := actorID, otherID
	if low > high {
		low, high = high, low
	}
	key := fmt.Sprintf("direct-chat:%d:%d", low, high)
	// Hash collisions only serialize unrelated pairs; they cannot mix their data.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
		return entity.DirectConversation{}, fmt.Errorf("lock direct pair: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, first_name || ' ' || last_name, avatar_url FROM users WHERE id IN ($1,$2) ORDER BY id FOR KEY SHARE`, low, high)
	if err != nil {
		return entity.DirectConversation{}, fmt.Errorf("find direct participants: %w", err)
	}
	result := entity.DirectConversation{}
	count := 0
	for rows.Next() {
		var user entity.Participant
		if err := rows.Scan(&user.ID, &user.Name, &user.AvatarURL); err != nil {
			rows.Close()
			return entity.DirectConversation{}, fmt.Errorf("scan direct participant: %w", err)
		}
		count++
		if user.ID == otherID {
			result.Participant = user
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return entity.DirectConversation{}, fmt.Errorf("read direct participants: %w", err)
	}
	if count != 2 {
		return entity.DirectConversation{}, entity.ErrUserNotFound
	}
	err = tx.QueryRowContext(ctx, `SELECT conversation_id FROM conversation_members
 GROUP BY conversation_id HAVING count(*)=2 AND count(*) FILTER (WHERE user_id IN ($1,$2))=2
 ORDER BY conversation_id LIMIT 1`, low, high).Scan(&result.ID)
	if err == nil {
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return entity.DirectConversation{}, fmt.Errorf("find direct conversation: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `INSERT INTO conversations DEFAULT VALUES RETURNING id`).Scan(&result.ID); err != nil {
		return entity.DirectConversation{}, fmt.Errorf("create direct conversation: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO conversation_members(conversation_id,user_id) VALUES ($1,$2),($1,$3)`, result.ID, low, high); err != nil {
		return entity.DirectConversation{}, fmt.Errorf("add direct participants: %w", err)
	}
	result.Created = true
	return result, nil
}
