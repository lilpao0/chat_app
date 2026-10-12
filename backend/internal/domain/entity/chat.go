package entity

import (
	"errors"
	"time"
)

var (
	ErrInvalidInput          = errors.New("invalid chat input")
	ErrConversationNotFound  = errors.New("conversation not found")
	ErrUnsupportedMediaType  = errors.New("unsupported media type")
	ErrMediaTooLarge         = errors.New("media too large")
	ErrAttachmentNotFound    = errors.New("attachment not found")
	ErrAttachmentNotReady    = errors.New("attachment not ready")
	ErrAttachmentUnavailable = errors.New("attachment unavailable")
	ErrAttachmentExpired     = errors.New("attachment expired")
	ErrInvalidMedia          = errors.New("invalid media")
	ErrStorageUnavailable    = errors.New("storage unavailable")
	ErrUploadRateLimited     = errors.New("upload issuance rate exceeded")
)

type Participant struct {
	ID              int64
	Name, AvatarURL string
}
type ConversationSummary struct {
	ID            int64
	Participant   Participant
	LastMessage   *string
	LastMessageAt *time.Time
	UnreadCount   int64
}

type Attachment struct {
	ID          int64
	Kind        string
	ContentType string
	SizeBytes   int64
}

type Message struct {
	ID, ConversationID, SenderID int64
	Content                      string
	CreatedAt                    time.Time
	Attachments                  []Attachment
}

type MessageQuery struct {
	BeforeID, AfterID int64
	Limit             int
}
type MessagePage struct {
	Items                     []Message
	HasMore                   bool
	NextBeforeID, NextAfterID *int64
}
type UploadReservation struct {
	ID        int64
	ObjectKey string
	ExpiresAt time.Time
}
type UploadLimitError struct {
	RetryAfterSeconds int64
}

func (e *UploadLimitError) Error() string {
	return "upload limit exceeded"
}

// AttachmentRecord stays inside the backend; object keys never enter chat DTOs.
type AttachmentRecord struct {
	Attachment
	ConversationID, OwnerID int64
	ObjectKey, State        string
	Generation              int64
	ExpiresAt               time.Time
}

type VerifiedMedia struct {
	Kind, ContentType     string
	SizeBytes, Generation int64
}

type UploadGrant struct {
	ID               int64
	URL, ContentType string
	SizeBytes        int64
	ExpiresAt        time.Time
}

type MediaAccess struct {
	ID        int64
	URL       string
	ExpiresAt time.Time
}

type CleanupFile struct {
	ID, Generation                    int64
	ObjectKey, SessionURL, ClaimToken string
}
