package attachment

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

type Media struct {
	files            repository.MediaRepository
	storage          repository.MediaStorage
	inspectionSlots  chan struct{}
	rateMu           sync.Mutex
	uploadsPerMinute int
	users            map[int64]uploadWindow
}

type uploadWindow struct {
	start time.Time
	count int
}

func NewMedia(files repository.MediaRepository, storage repository.MediaStorage) *Media {
	return NewMediaWithUploadRate(files, storage, 60)
}

func NewMediaWithUploadRate(files repository.MediaRepository, storage repository.MediaStorage, rate int) *Media {
	return &Media{files: files, storage: storage, inspectionSlots: make(chan struct{}, 2), uploadsPerMinute: max(1, rate), users: make(map[int64]uploadWindow)}
}

func (u *Media) allowUpload(owner int64) bool {
	// ponytail: rate is per process for current single-instance deployment; use shared limits before scaling out.
	u.rateMu.Lock()
	defer u.rateMu.Unlock()
	now := time.Now()
	for id, w := range u.users {
		if now.Sub(w.start) >= time.Minute {
			delete(u.users, id)
		}
	}
	w, exists := u.users[owner]
	if !exists {
		if len(u.users) >= 4096 {
			return false
		}
		w.start = now
	}
	if w.count >= u.uploadsPerMinute {
		return false
	}
	w.count++
	u.users[owner] = w
	return true
}

func (u *Media) fail(ctx context.Context, id, owner int64, session string) error {
	// Compensation still has a short deadline when the client disconnects.
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	var cancelErr error
	if session != "" {
		cancelErr = u.storage.CancelUpload(cleanup, session)
	}
	return errors.Join(cancelErr, u.files.FailUpload(cleanup, id, owner))
}

func (u *Media) Create(ctx context.Context, conversationID, owner int64, input UploadInput, origin string) (entity.UploadGrant, error) {
	if u.storage == nil {
		return entity.UploadGrant{}, entity.ErrStorageUnavailable
	}
	if conversationID <= 0 || owner <= 0 {
		return entity.UploadGrant{}, entity.ErrInvalidInput
	}
	if !u.allowUpload(owner) {
		return entity.UploadGrant{}, entity.ErrUploadRateLimited
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	reserved, err := NewReserveUpload(u.files).Execute(ctx, conversationID, owner, input)
	if err != nil {
		return entity.UploadGrant{}, err
	}
	session, err := u.storage.StartUpload(ctx, reserved.ObjectKey, input.ContentType, input.SizeBytes, origin)
	if err != nil {
		return entity.UploadGrant{}, errors.Join(err, u.fail(ctx, reserved.ID, owner, session))
	}
	if err := u.files.SaveSession(ctx, reserved.ID, owner, session); err != nil {
		return entity.UploadGrant{}, errors.Join(err, u.fail(ctx, reserved.ID, owner, session))
	}
	return entity.UploadGrant{ID: reserved.ID, URL: session, ContentType: input.ContentType, SizeBytes: input.SizeBytes, ExpiresAt: reserved.ExpiresAt}, nil
}

func (u *Media) Complete(ctx context.Context, id, owner int64) (entity.AttachmentRecord, error) {
	if id <= 0 || owner <= 0 {
		return entity.AttachmentRecord{}, entity.ErrInvalidInput
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	f, err := u.files.Owned(ctx, id, owner)
	if err != nil {
		return f, err
	}
	if f.State == "attached" {
		return f, nil
	}
	if f.State != "uploading" && f.State != "ready" {
		return f, entity.ErrAttachmentUnavailable
	}
	if !time.Now().Before(f.ExpiresAt) {
		return f, entity.ErrAttachmentExpired
	}
	if f.State == "ready" {
		return f, nil
	}
	if u.storage == nil {
		return f, entity.ErrStorageUnavailable
	}
	select {
	case u.inspectionSlots <- struct{}{}:
		defer func() { <-u.inspectionSlots }()
	case <-ctx.Done():
		return f, entity.ErrStorageUnavailable
	}
	verified, err := u.storage.Inspect(ctx, f.ObjectKey, f.ContentType, f.SizeBytes)
	if err != nil {
		if errors.Is(err, entity.ErrInvalidMedia) || errors.Is(err, entity.ErrUnsupportedMediaType) || errors.Is(err, entity.ErrMediaTooLarge) {
			err = errors.Join(err, u.fail(ctx, id, owner, ""))
		}
		return f, err
	}
	return u.files.Complete(ctx, id, owner, verified)
}

func (u *Media) Access(ctx context.Context, id, actor int64) (entity.MediaAccess, error) {
	if id <= 0 || actor <= 0 {
		return entity.MediaAccess{}, entity.ErrInvalidInput
	}
	f, err := u.files.Accessible(ctx, id, actor)
	if err != nil {
		return entity.MediaAccess{}, err
	}
	if u.storage == nil {
		return entity.MediaAccess{}, entity.ErrStorageUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	url, expires, err := u.storage.SignAccess(ctx, f.ObjectKey, f.Generation)
	return entity.MediaAccess{ID: id, URL: url, ExpiresAt: expires}, err
}

func (u *Media) Cleanup(ctx context.Context, limit int) (int, error) {
	if u.storage == nil {
		return 0, entity.ErrStorageUnavailable
	}
	files, err := u.files.ClaimExpired(ctx, limit)
	if err != nil {
		return 0, err
	}
	done := 0
	var failures []error
	for _, f := range files {
		if ctx.Err() != nil {
			failures = append(failures, ctx.Err())
			break
		}
		operation, cancel := context.WithTimeout(ctx, 20*time.Second)
		var err error
		if f.SessionURL != "" {
			err = u.storage.CancelUpload(operation, f.SessionURL)
		}
		if err == nil {
			err = u.storage.DeleteObject(operation, f.ObjectKey, f.Generation)
		}
		cancel()
		finish, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		finishErr := u.files.FinishCleanup(finish, f.ID, f.ClaimToken, err == nil)
		finishCancel()
		if err == nil && finishErr == nil {
			done++
		} else {
			failures = append(failures, errors.Join(err, finishErr))
		}
	}
	return done, errors.Join(failures...)
}
