package attachment

import (
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
)

type UploadInput struct {
	ContentType string
	SizeBytes   int64
}

func ValidateUploadInput(input UploadInput) error {
	if input.SizeBytes <= 0 {
		return entity.ErrInvalidInput
	}

	var maxBytes int64
	switch input.ContentType {
	case "image/jpeg", "image/png":
		maxBytes = 10 * 1024 * 1024
	case "video/mp4":
		maxBytes = 50 * 1024 * 1024
	default:
		return entity.ErrUnsupportedMediaType
	}

	if input.SizeBytes > maxBytes {
		return entity.ErrMediaTooLarge
	}
	return nil
}
