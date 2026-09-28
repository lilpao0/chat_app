package repository

import (
	"context"
	"time"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
)

type UpdateProfileChanges struct {
	FirstNameSet   bool
	FirstName      string
	LastNameSet    bool
	LastName       string
	DateOfBirthSet bool
	DateOfBirth    *time.Time
	PhoneNumberSet bool
	PhoneNumber    *string
}

type PrivateProfileReader interface {
	FindPrivateProfileByID(
		ctx context.Context,
		userID int64,
	) (entity.PrivateProfile, error)
}

type PublicProfileReader interface {
	FindPublicProfileByID(
		ctx context.Context,
		userID int64,
	) (entity.PublicProfile, error)
}

type ProfileUpdater interface {
	UpdateProfile(context.Context, int64, UpdateProfileChanges) (entity.PrivateProfile, error)
}
