package attachment

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
)

type mediaFiles struct {
	f              entity.AttachmentRecord
	saveErr, error error
	saved          string
	failed         bool
	completeCalls  int
	cleanup        []entity.CleanupFile
	finishSuccess  bool
}

func (f *mediaFiles) IsMember(context.Context, int64, int64) (bool, error) { return true, f.error }
func (f *mediaFiles) ReserveUpload(context.Context, int64, int64, string, int64) (entity.UploadReservation, error) {
	return entity.UploadReservation{ID: 1, ObjectKey: "media/test", ExpiresAt: time.Now().Add(time.Hour)}, f.error
}
func (f *mediaFiles) FailUpload(ctx context.Context, id, owner int64) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	f.failed = true
	return nil
}
func (f *mediaFiles) SaveSession(_ context.Context, _, _ int64, s string) error {
	f.saved = s
	return f.saveErr
}
func (f *mediaFiles) Owned(context.Context, int64, int64) (entity.AttachmentRecord, error) {
	return f.f, f.error
}
func (f *mediaFiles) Accessible(context.Context, int64, int64) (entity.AttachmentRecord, error) {
	return f.f, f.error
}
func (f *mediaFiles) Complete(_ context.Context, _, _ int64, v entity.VerifiedMedia) (entity.AttachmentRecord, error) {
	f.completeCalls++
	f.f.State = "ready"
	f.f.Kind = v.Kind
	f.f.Generation = v.Generation
	return f.f, nil
}
func (f *mediaFiles) ClaimExpired(context.Context, int) ([]entity.CleanupFile, error) {
	return f.cleanup, f.error
}
func (f *mediaFiles) FinishCleanup(_ context.Context, _ int64, _ string, ok bool) error {
	f.finishSuccess = ok
	return nil
}

type mediaStorage struct {
	startErr, inspectErr, cancelErr, deleteErr error
	cancelled                                  bool
	inspect                                    func(context.Context) (entity.VerifiedMedia, error)
}

func (s *mediaStorage) StartUpload(context.Context, string, string, int64, string) (string, error) {
	if s.startErr != nil {
		return "", s.startErr
	}
	return "https://storage.example/session", nil
}
func (s *mediaStorage) CancelUpload(ctx context.Context, _ string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	s.cancelled = true
	return s.cancelErr
}
func (s *mediaStorage) Inspect(ctx context.Context, _, _ string, _ int64) (entity.VerifiedMedia, error) {
	if s.inspect != nil {
		return s.inspect(ctx)
	}
	return entity.VerifiedMedia{Kind: "image", ContentType: "image/jpeg", SizeBytes: 100, Generation: 2}, s.inspectErr
}
func (s *mediaStorage) SignAccess(context.Context, string, int64) (string, time.Time, error) {
	return "https://storage.example/read", time.Now().Add(time.Minute), nil
}
func (s *mediaStorage) DeleteObject(context.Context, string, int64) error { return s.deleteErr }

func TestGrantCompensationAndRateBound(t *testing.T) {
	input := UploadInput{ContentType: "image/jpeg", SizeBytes: 100}
	files, store := &mediaFiles{}, &mediaStorage{}
	media := NewMedia(files, store)
	grant, err := media.Create(context.Background(), 3, 5, input, "")
	if err != nil || grant.ID != 1 || files.saved != grant.URL {
		t.Fatalf("grant error=%v", err)
	}
	files.saveErr = errors.New("database unavailable")
	if grant, err := media.Create(context.Background(), 3, 5, input, ""); err == nil || grant.URL != "" || !files.failed || !store.cancelled {
		t.Fatal("failed persistence granted upload or lost compensation")
	}
	files.saveErr = nil
	files.failed = false
	store.startErr = entity.ErrStorageUnavailable
	if _, err := media.Create(context.Background(), 3, 5, input, ""); !errors.Is(err, entity.ErrStorageUnavailable) || !files.failed {
		t.Fatal("storage failure retained reservation")
	}
	store.startErr = nil
	limited := NewMediaWithUploadRate(files, store, 1)
	if _, err := limited.Create(context.Background(), 3, 5, input, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := limited.Create(context.Background(), 3, 5, input, ""); !errors.Is(err, entity.ErrUploadRateLimited) {
		t.Fatal("issuance rate ignored")
	}
}

func TestCompletionStatesAndPermanentFailure(t *testing.T) {
	files := &mediaFiles{f: entity.AttachmentRecord{Attachment: entity.Attachment{ID: 1, ContentType: "image/jpeg", SizeBytes: 100}, State: "uploading", ExpiresAt: time.Now().Add(time.Hour)}}
	store := &mediaStorage{inspectErr: entity.ErrInvalidMedia}
	media := NewMedia(files, store)
	if _, err := media.Complete(context.Background(), 1, 5); !errors.Is(err, entity.ErrInvalidMedia) || !files.failed || files.completeCalls != 0 {
		t.Fatal("invalid media became ready")
	}
	files.failed = false
	store.inspectErr = entity.ErrStorageUnavailable
	if _, err := media.Complete(context.Background(), 1, 5); !errors.Is(err, entity.ErrStorageUnavailable) || files.failed {
		t.Fatal("temporary inspection failure became permanent")
	}
	files.f.State = "ready"
	if _, err := media.Complete(context.Background(), 1, 5); err != nil || files.completeCalls != 0 {
		t.Fatal("ready completion was not idempotent")
	}
	files.f.ExpiresAt = time.Now().Add(-time.Hour)
	if _, err := media.Complete(context.Background(), 1, 5); !errors.Is(err, entity.ErrAttachmentExpired) {
		t.Fatal("expired ready file accepted")
	}
	files.f.State = "attached"
	if _, err := media.Complete(context.Background(), 1, 5); err != nil {
		t.Fatal("attached completion reused unsent expiry")
	}
	files.f.State = "deleted"
	if _, err := media.Complete(context.Background(), 1, 5); !errors.Is(err, entity.ErrAttachmentUnavailable) {
		t.Fatal("deleted upload accepted")
	}
}

func TestInspectionConcurrencyAndCleanupRecovery(t *testing.T) {
	files := &mediaFiles{f: entity.AttachmentRecord{Attachment: entity.Attachment{ID: 1}, State: "uploading", ExpiresAt: time.Now().Add(time.Hour)}}
	var active, maximum atomic.Int32
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	store := &mediaStorage{inspect: func(ctx context.Context) (entity.VerifiedMedia, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for {
			old := maximum.Load()
			if old >= n || maximum.CompareAndSwap(old, n) {
				break
			}
		}
		started <- struct{}{}
		select {
		case <-release:
			return entity.VerifiedMedia{}, entity.ErrStorageUnavailable
		case <-ctx.Done():
			return entity.VerifiedMedia{}, entity.ErrStorageUnavailable
		}
	}}
	media := NewMedia(files, store)
	done := make(chan error, 3)
	for i := 0; i < 2; i++ {
		go func() { _, err := media.Complete(context.Background(), 1, 5); done <- err }()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("inspection did not start")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := media.Complete(ctx, 1, 5); !errors.Is(err, entity.ErrStorageUnavailable) {
		t.Fatal("blocked inspection ignored cancellation")
	}
	close(release)
	<-done
	<-done
	if maximum.Load() != 2 {
		t.Fatal("inspection bound changed")
	}
	files.cleanup = []entity.CleanupFile{{ID: 1, ObjectKey: "media/test", SessionURL: "session", ClaimToken: "claim"}}
	store.cancelErr = entity.ErrStorageUnavailable
	if n, err := media.Cleanup(context.Background(), 1); err == nil || n != 0 || files.finishSuccess {
		t.Fatal("cleanup failure marked deleted")
	}
	store.cancelErr = nil
	if n, err := media.Cleanup(context.Background(), 1); err != nil || n != 1 || !files.finishSuccess {
		t.Fatal("cleanup retry did not recover")
	}
}
