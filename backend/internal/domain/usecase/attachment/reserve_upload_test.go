package attachment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/attachment"
)

type uploadRepository struct {
	member    bool
	memberErr error
	reserve   func(context.Context, int64, int64, string, int64) (entity.UploadReservation, error)
}

func (*uploadRepository) FailUpload(context.Context, int64, int64) error {
	return errors.New("unexpected FailUpload call")
}

func (r *uploadRepository) IsMember(context.Context, int64, int64) (bool, error) {
	return r.member, r.memberErr
}

func (r *uploadRepository) ReserveUpload(
	ctx context.Context,
	conversationID, ownerID int64,
	contentType string,
	sizeBytes int64,
) (entity.UploadReservation, error) {
	return r.reserve(ctx, conversationID, ownerID, contentType, sizeBytes)
}

func TestReserveUploadRules(t *testing.T) {
	ctx := context.Background()
	input := attachment.UploadInput{ContentType: "image/jpeg", SizeBytes: 100}
	want := entity.UploadReservation{ID: 7, ObjectKey: "media/test"}
	calls := 0
	cause := errors.New("database unavailable")

	r := &uploadRepository{
		reserve: func(gotCtx context.Context, c, u int64, ct string, size int64) (entity.UploadReservation, error) {
			calls++
			if gotCtx != ctx || c != 3 || u != 5 ||
				ct != input.ContentType || size != input.SizeBytes {
				t.Fatal("reservation received wrong arguments")
			}
			return want, nil
		},
	}
	usecase := attachment.NewReserveUpload(r)

	if _, err := attachment.NewReserveUpload(nil).Execute(ctx, 0, 5, input); !errors.Is(err, entity.ErrInvalidInput) {
		t.Fatal("invalid conversation ID accepted")
	}
	if _, err := attachment.NewReserveUpload(nil).Execute(ctx, 3, 0, input); !errors.Is(err, entity.ErrInvalidInput) {
		t.Fatal("invalid user ID accepted")
	}

	if _, err := usecase.Execute(ctx, 3, 5, input); !errors.Is(err, entity.ErrConversationNotFound) || calls != 0 {
		t.Fatal("outsider reached reservation")
	}

	r.memberErr = cause
	if _, err := usecase.Execute(ctx, 3, 5, input); !errors.Is(err, cause) || calls != 0 {
		t.Fatal("membership failure ignored")
	}
	r.memberErr = nil
	r.member = true

	invalid := attachment.UploadInput{ContentType: "image/gif", SizeBytes: 100}
	if _, err := usecase.Execute(ctx, 3, 5, invalid); !errors.Is(err, entity.ErrUnsupportedMediaType) || calls != 0 {
		t.Fatal("invalid media reached reservation")
	}

	got, err := usecase.Execute(ctx, 3, 5, input)
	if err != nil || got != want || calls != 1 {
		t.Fatalf("reservation=%+v err=%v calls=%d", got, err, calls)
	}

	r.reserve = func(context.Context, int64, int64, string, int64) (entity.UploadReservation, error) {
		return entity.UploadReservation{}, cause
	}
	if _, err := usecase.Execute(ctx, 3, 5, input); !errors.Is(err, cause) {
		t.Fatal("reservation failure ignored")
	}
}
