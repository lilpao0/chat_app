package repository

import (
	"context"
	"time"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
)

type AttachmentUploadRepository interface {
	MembershipReader
	ReserveUpload(
		ctx context.Context,
		conversationID, ownerID int64,
		contentType string,
		sizeBytes int64,
	) (entity.UploadReservation, error)
	FailUpload(ctx context.Context, attachmentID, ownerID int64) error
}

type MediaRepository interface {
	AttachmentUploadRepository
	SaveSession(context.Context, int64, int64, string) error
	Owned(context.Context, int64, int64) (entity.AttachmentRecord, error)
	Accessible(context.Context, int64, int64) (entity.AttachmentRecord, error)
	Complete(context.Context, int64, int64, entity.VerifiedMedia) (entity.AttachmentRecord, error)
	ClaimExpired(context.Context, int) ([]entity.CleanupFile, error)
	FinishCleanup(context.Context, int64, string, bool) error
}

type MediaStorage interface {
	StartUpload(context.Context, string, string, int64, string) (string, error)
	CancelUpload(context.Context, string) error
	Inspect(context.Context, string, string, int64) (entity.VerifiedMedia, error)
	SignAccess(context.Context, string, int64) (string, time.Time, error)
	DeleteObject(context.Context, string, int64) error
}
