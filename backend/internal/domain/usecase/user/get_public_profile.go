package user

import (
	"context"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

type GetPublicProfile struct {
	profiles repository.PublicProfileReader
}

func NewGetPublicProfile(
	profiles repository.PublicProfileReader,
) *GetPublicProfile {
	return &GetPublicProfile{profiles: profiles}
}

func (u *GetPublicProfile) Execute(
	ctx context.Context,
	userID int64,
) (entity.PublicProfile, error) {
	if userID <= 0 {
		return entity.PublicProfile{}, entity.ErrInvalidInput
	}

	return u.profiles.FindPublicProfileByID(ctx, userID)
}
