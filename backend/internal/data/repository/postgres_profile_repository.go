package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

var (
	_ domainrepo.PrivateProfileReader = (*PostgresUserRepository)(nil)
	_ domainrepo.PublicProfileReader  = (*PostgresUserRepository)(nil)
	_ domainrepo.ProfileUpdater       = (*PostgresUserRepository)(nil)
)

func (r *PostgresUserRepository) FindPrivateProfileByID(
	ctx context.Context,
	userID int64,
) (entity.PrivateProfile, error) {
	var profile entity.PrivateProfile
	var dateOfBirth sql.NullTime
	var phoneNumber sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			first_name,
			COALESCE(last_name, ''),
			concat_ws(' ', first_name, NULLIF(last_name, '')),
			date_of_birth,
			phone_number,
			avatar_url
		FROM users
		WHERE id = $1
		  AND deleted_at IS NULL`,
		userID,
	).Scan(
		&profile.ID,
		&profile.FirstName,
		&profile.LastName,
		&profile.Name,
		&dateOfBirth,
		&phoneNumber,
		&profile.AvatarURL,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return entity.PrivateProfile{}, entity.ErrUserNotFound
	}

	if err != nil {
		return entity.PrivateProfile{},
			fmt.Errorf("find private profile by id: %w", err)
	}

	if dateOfBirth.Valid {
		value := dateOfBirth.Time
		profile.DateOfBirth = &value
	}

	if phoneNumber.Valid {
		value := phoneNumber.String
		profile.PhoneNumber = &value
	}
	return profile, nil
}

func (r *PostgresUserRepository) UpdateProfile(ctx context.Context, userID int64, changes domainrepo.UpdateProfileChanges) (entity.PrivateProfile, error) {
	var profile entity.PrivateProfile
	var dateOfBirth sql.NullTime
	var phoneNumber sql.NullString
	err := r.db.QueryRowContext(ctx, `
		UPDATE users SET
			first_name = CASE WHEN $2 THEN $3 ELSE first_name END,
			last_name = CASE WHEN $4 THEN $5 ELSE last_name END,
			date_of_birth = CASE WHEN $6 THEN $7::date ELSE date_of_birth END,
			phone_number = CASE WHEN $8 THEN $9::varchar ELSE phone_number END
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, first_name, COALESCE(last_name, ''),
			concat_ws(' ', first_name, NULLIF(last_name, '')),
			date_of_birth, phone_number, avatar_url`,
		userID,
		changes.FirstNameSet, changes.FirstName,
		changes.LastNameSet, changes.LastName,
		changes.DateOfBirthSet, changes.DateOfBirth,
		changes.PhoneNumberSet, changes.PhoneNumber,
	).Scan(&profile.ID, &profile.FirstName, &profile.LastName, &profile.Name, &dateOfBirth, &phoneNumber, &profile.AvatarURL)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.PrivateProfile{}, entity.ErrUserNotFound
	}
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.Constraint == "users_phone_number_key" {
			return entity.PrivateProfile{}, entity.ErrPhoneNumberTaken
		}
		return entity.PrivateProfile{}, fmt.Errorf("update profile: %w", err)
	}
	if dateOfBirth.Valid {
		value := dateOfBirth.Time
		profile.DateOfBirth = &value
	}
	if phoneNumber.Valid {
		value := phoneNumber.String
		profile.PhoneNumber = &value
	}
	return profile, nil
}

func (r *PostgresUserRepository) FindPublicProfileByID(
	ctx context.Context,
	userID int64,
) (entity.PublicProfile, error) {
	var profile entity.PublicProfile
	err := r.db.QueryRowContext(ctx, `
	SELECT
	id,
	concat_ws(' ',first_name,NULLIF(last_name,'')),
	avatar_url
	From users
	WHERE id = $1
	AND deleted_at IS NULL`,
		userID,
	).Scan(
		&profile.ID,
		&profile.Name,
		&profile.AvatarURL,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.PublicProfile{}, entity.ErrUserNotFound
	}
	if err != nil {
		return entity.PublicProfile{},
			fmt.Errorf("find public profile by id: %w", err)
	}
	return profile, nil
}
