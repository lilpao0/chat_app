package repository

import (
	"context"
	"fmt"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

var _ domainrepo.UserSearcher = (*PostgresUserRepository)(nil)

func (r *PostgresUserRepository) Search(ctx context.Context, actorID int64, q entity.UserSearchQuery) (entity.UserSearchPage, error) {
	if q.Limit < 1 || q.Limit > 50 {
		return entity.UserSearchPage{}, entity.ErrInvalidInput
	}
	// strpos treats %, _ and SQL-looking text literally. Email requires an exact match.
	query := `SELECT id,name,avatar_url FROM users
 WHERE id<>$1 AND id>$3 AND (strpos(lower(name),lower($2))>0 OR lower(email)=lower($2))
 ORDER BY id ASC LIMIT $4`
	rows, err := r.db.QueryContext(ctx, query, actorID, q.Text, q.AfterID, q.Limit+1)
	if err != nil {
		return entity.UserSearchPage{}, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()
	page := entity.UserSearchPage{Items: make([]entity.Participant, 0, q.Limit)}
	for rows.Next() {
		var user entity.Participant
		if err := rows.Scan(&user.ID, &user.Name, &user.AvatarURL); err != nil {
			return entity.UserSearchPage{}, fmt.Errorf("scan user summary: %w", err)
		}
		page.Items = append(page.Items, user)
	}
	if err := rows.Err(); err != nil {
		return entity.UserSearchPage{}, fmt.Errorf("read user search: %w", err)
	}
	if len(page.Items) > q.Limit {
		page.HasMore = true
		page.Items = page.Items[:q.Limit]
		last := page.Items[len(page.Items)-1].ID
		page.NextAfterID = &last
	}
	return page, nil
}
