package attachment_test

import (
	"errors"
	"testing"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/attachment"
)

func TestValidateUploadInput(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		sizeBytes   int64
		want        error
	}{
		{"jpeg minimum", "image/jpeg", 1, nil},
		{"jpeg maximum", "image/jpeg", 10485760, nil},
		{"png maximum", "image/png", 10485760, nil},
		{"mp4 maximum", "video/mp4", 52428800, nil},
		{"zero size", "image/jpeg", 0, entity.ErrInvalidInput},
		{"negative size", "video/mp4", -1, entity.ErrInvalidInput},
		{"jpeg oversized", "image/jpeg", 10485761, entity.ErrMediaTooLarge},
		{"png oversized", "image/png", 10485761, entity.ErrMediaTooLarge},
		{"mp4 oversized", "video/mp4", 52428801, entity.ErrMediaTooLarge},
		{"missing type", "", 1, entity.ErrUnsupportedMediaType},
		{"gif unsupported", "image/gif", 1, entity.ErrUnsupportedMediaType},
		{"webm unsupported", "video/webm", 1, entity.ErrUnsupportedMediaType},
		{"type with space", " image/png", 1, entity.ErrUnsupportedMediaType},
		{"type with parameter", "video/mp4; codecs=avc1", 1, entity.ErrUnsupportedMediaType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := attachment.ValidateUploadInput(attachment.UploadInput{
				ContentType: tt.contentType,
				SizeBytes:   tt.sizeBytes,
			})
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}
