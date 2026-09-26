package seed

import (
	"context"
	"database/sql"
	"errors"

	datarepo "github.com/lilpao0/chat_app/backend/internal/data/repository"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

type Result struct{ UserAID, UserBID, ConversationID int64 }

// Demo adds missing accounts and one shared 1-1 conversation atomically.
// Existing accounts are reused without changing names or credentials.
func Demo(ctx context.Context, db *sql.DB, a, b repository.CreateUser) (Result, error) {
	if a.Email == b.Email {
		return Result{}, errors.New("demo accounts require distinct emails")
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback()
	// Serialize repeated seed commands, including concurrent first-time runs.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(72401, 17)`); err != nil {
		return Result{}, err
	}
	ids := make([]int64, 2)
	for i, input := range []repository.CreateUser{a, b} {
		if _, err := tx.ExecContext(ctx, `INSERT INTO users(first_name, last_name, email, password_hash) VALUES ($1,$2,$3,$4) ON CONFLICT (email) DO NOTHING`, input.FirstName, input.LastName, input.Email, input.PasswordHash); err != nil {
			return Result{}, err
		}
		if err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE email=$1`, input.Email).Scan(&ids[i]); err != nil {
			return Result{}, err
		}
	}
	conversation, err := datarepo.OpenDirectInTx(ctx, tx, ids[0], ids[1])
	if err != nil {
		return Result{}, err
	}
	if err := tx.Commit(); err != nil {
		return Result{}, err
	}
	return Result{UserAID: ids[0], UserBID: ids[1], ConversationID: conversation.ID}, nil
}
