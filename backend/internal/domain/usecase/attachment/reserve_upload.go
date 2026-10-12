package attachment

import (
	"context"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

type ReserveUpload struct {
	uploads repository.AttachmentUploadRepository
}

func NewReserveUpload(uploads repository.AttachmentUploadRepository) *ReserveUpload {
	return &ReserveUpload{uploads: uploads}
}

func (u *ReserveUpload) Execute(
	ctx context.Context,
	conversationID, userID int64,
	input UploadInput,
) (entity.UploadReservation, error) {
	if conversationID <= 0 || userID <= 0 {
		return entity.UploadReservation{}, entity.ErrInvalidInput
	}

	member, err := u.uploads.IsMember(ctx, conversationID, userID)
	if err != nil {
		return entity.UploadReservation{}, err
	}
	if !member {
		return entity.UploadReservation{}, entity.ErrConversationNotFound
	}
	if err := ValidateUploadInput(input); err != nil {
		return entity.UploadReservation{}, err
	}
	return u.uploads.ReserveUpload(
		ctx, conversationID, userID,
		input.ContentType, input.SizeBytes,
	)
}
