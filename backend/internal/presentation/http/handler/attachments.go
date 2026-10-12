package handler

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/attachment"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
)

type MediaUseCase interface {
	Create(context.Context, int64, int64, attachment.UploadInput, string) (entity.UploadGrant, error)
	Complete(context.Context, int64, int64) (entity.AttachmentRecord, error)
	Access(context.Context, int64, int64) (entity.MediaAccess, error)
}

type AttachmentHandler struct{ media MediaUseCase }

func NewAttachmentHandler(media MediaUseCase) *AttachmentHandler {
	return &AttachmentHandler{media: media}
}

func mediaError(c *gin.Context, err error) {
	var limit *entity.UploadLimitError
	if errors.As(err, &limit) {
		c.Header("Retry-After", strconv.FormatInt(max(1, limit.RetryAfterSeconds), 10))
		response.Error(c, 429, "upload_limit_exceeded", "Finish existing uploads or wait for expiry.")
		return
	}
	if errors.Is(err, entity.ErrUploadRateLimited) {
		c.Header("Retry-After", "60")
		response.Error(c, 429, "rate_limited", "Too many upload requests.")
		return
	}
	chatError(c, err)
}

func (h *AttachmentHandler) Create(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	conversation, actor, ok := chatActor(c)
	if !ok {
		return
	}
	var body struct {
		ContentType *string `json:"content_type"`
		SizeBytes   *int64  `json:"size_bytes"`
	}
	if !readJSON(c, &body) {
		return
	}
	if body.ContentType == nil || body.SizeBytes == nil {
		mediaError(c, entity.ErrInvalidInput)
		return
	}
	grant, err := h.media.Create(c.Request.Context(), conversation, actor, attachment.UploadInput{ContentType: *body.ContentType, SizeBytes: *body.SizeBytes}, c.GetHeader("Origin"))
	if err != nil {
		mediaError(c, err)
		return
	}
	response.Success(c, 201, struct {
		ID          int64             `json:"id"`
		URL         string            `json:"upload_url"`
		Method      string            `json:"upload_method"`
		Headers     map[string]string `json:"upload_headers"`
		ContentType string            `json:"content_type"`
		SizeBytes   int64             `json:"size_bytes"`
		ExpiresAt   time.Time         `json:"expires_at"`
	}{grant.ID, grant.URL, "PUT", map[string]string{"Content-Type": grant.ContentType}, grant.ContentType, grant.SizeBytes, grant.ExpiresAt.UTC()})
}

func (h *AttachmentHandler) Complete(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, actor, ok := chatActor(c)
	if !ok {
		return
	}
	var body struct{}
	if !readJSON(c, &body) {
		return
	}
	f, err := h.media.Complete(c.Request.Context(), id, actor)
	if err != nil {
		mediaError(c, err)
		return
	}
	var expiry *time.Time
	if f.State != "attached" {
		at := f.ExpiresAt.UTC()
		expiry = &at
	}
	response.Success(c, 200, struct {
		attachmentDTO
		State     string     `json:"state"`
		ExpiresAt *time.Time `json:"expires_at"`
	}{attachmentDTO{ID: f.ID, Kind: f.Kind, ContentType: f.ContentType, SizeBytes: f.SizeBytes}, f.State, expiry})
}

func (h *AttachmentHandler) Access(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	id, actor, ok := chatActor(c)
	if !ok {
		return
	}
	grant, err := h.media.Access(c.Request.Context(), id, actor)
	if err != nil {
		mediaError(c, err)
		return
	}
	response.Success(c, 200, struct {
		ID        int64     `json:"id"`
		URL       string    `json:"url"`
		ExpiresAt time.Time `json:"expires_at"`
	}{grant.ID, grant.URL, grant.ExpiresAt.UTC()})
}
