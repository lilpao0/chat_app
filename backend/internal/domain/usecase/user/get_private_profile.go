package user

import (
	"context"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

type GetPrivateProfile struct {
	profiles repository.PrivateProfileReader
}

func NewGetPrivateProfile(
	profiles repository.PrivateProfileReader,
) *GetPrivateProfile {
	return &GetPrivateProfile{profiles: profiles}
}

func (u *GetPrivateProfile) Execute(
	ctx context.Context,
	userID int64,
) (entity.PrivateProfile, error) {
	if userID <= 0 {
		return entity.PrivateProfile{}, entity.ErrInvalidInput
	}
	return u.profiles.FindPrivateProfileByID(ctx, userID)
}
