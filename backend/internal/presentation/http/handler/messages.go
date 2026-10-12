package handler

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/middleware"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
	"net/http"
	"strconv"
	"time"
)

type SendMessageUseCase interface {
	Execute(context.Context, int64, int64, string) (entity.Message, error)
}
type keyedMessageSender interface {
	ExecuteWithRequestID(context.Context, int64, int64, string, string) (entity.Message, error)
}

type mediaMessageSender interface {
	ExecuteWithAttachments(context.Context, int64, int64, string, string, []int64) (entity.Message, error)
}
type SendMessageHandler struct{ send SendMessageUseCase }

func NewSendMessageHandler(send SendMessageUseCase) *SendMessageHandler {
	return &SendMessageHandler{send: send}
}

type messageDTO struct {
	ID             int64           `json:"id"`
	ConversationID int64           `json:"conversation_id"`
	SenderID       int64           `json:"sender_id"`
	Content        string          `json:"content"`
	CreatedAt      time.Time       `json:"created_at"`
	Attachments    []attachmentDTO `json:"attachments"`
}

type attachmentDTO struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

func toMessageDTO(m entity.Message) messageDTO {
	attachments := make([]attachmentDTO, 0, len(m.Attachments))
	for _, a := range m.Attachments {
		attachments = append(attachments, attachmentDTO{ID: a.ID, Kind: a.Kind, ContentType: a.ContentType, SizeBytes: a.SizeBytes})
	}
	return messageDTO{ID: m.ID, ConversationID: m.ConversationID, SenderID: m.SenderID, Content: m.Content, CreatedAt: m.CreatedAt.UTC(), Attachments: attachments}
}
func chatError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, entity.ErrAttachmentNotFound):
		response.Error(c, 404, "not_found", "Attachment not found.")
	case errors.Is(err, entity.ErrAttachmentNotReady):
		response.Error(c, 409, "attachment_not_ready", "Finish uploading and completing the attachment.")
	case errors.Is(err, entity.ErrAttachmentUnavailable):
		response.Error(c, 409, "attachment_unavailable", "Attachment is unavailable.")
	case errors.Is(err, entity.ErrAttachmentExpired):
		response.Error(c, 410, "attachment_expired", "Request a new upload.")
	case errors.Is(err, entity.ErrMediaTooLarge):
		response.Error(c, 413, "media_too_large", "Media exceeds the allowed size.")
	case errors.Is(err, entity.ErrUnsupportedMediaType):
		response.Error(c, 415, "unsupported_media_type", "Unsupported media format.")
	case errors.Is(err, entity.ErrInvalidMedia):
		response.Error(c, 422, "invalid_media", "Media does not match the declared format or limits.")
	case errors.Is(err, entity.ErrStorageUnavailable):
		response.Error(c, 503, "storage_unavailable", "Storage is temporarily unavailable.")
	case errors.Is(err, entity.ErrRequestConflict):
		response.Error(c, 409, "request_conflict", "Request ID was already used.")
	case errors.Is(err, entity.ErrInvalidInput):
		response.Error(c, 400, "invalid_input", "The submitted information is invalid.")
	case errors.Is(err, entity.ErrConversationNotFound):
		response.Error(c, 404, "not_found", "Conversation not found.")
	default:
		response.Error(c, 500, "internal_error", "Unable to complete the request.")
	}
}
func chatActor(c *gin.Context) (int64, int64, bool) {
	actor, ok := middleware.Identity(c)
	if !ok {
		response.Error(c, 401, "unauthenticated", "A valid access token is required.")
		return 0, 0, false
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		chatError(c, entity.ErrInvalidInput)
		return 0, 0, false
	}
	return id, actor.UserID, true
}
func (h *SendMessageHandler) Send(c *gin.Context) {
	id, actor, ok := chatActor(c)
	if !ok {
		return
	}
	var body struct {
		Content       optionalJSONField[string]  `json:"content"`
		RequestID     *string                    `json:"request_id"`
		AttachmentIDs optionalJSONField[[]int64] `json:"attachment_ids"`
	}
	if !readJSON(c, &body) {
		return
	}
	var message entity.Message
	var err error
	if body.Content.Null || body.AttachmentIDs.Null {
		chatError(c, entity.ErrInvalidInput)
		return
	}
	requestID := ""
	if body.RequestID != nil {
		requestID = *body.RequestID
		if !entity.ValidRequestID(requestID) {
			chatError(c, entity.ErrInvalidInput)
			return
		}
	}
	if sender, ok := h.send.(mediaMessageSender); ok {
		message, err = sender.ExecuteWithAttachments(c.Request.Context(), id, actor, body.Content.Value, requestID, body.AttachmentIDs.Value)
	} else if len(body.AttachmentIDs.Value) > 0 {
		err = errors.New("media send unavailable")
	} else if body.RequestID != nil {
		if !entity.ValidRequestID(*body.RequestID) {
			chatError(c, entity.ErrInvalidInput)
			return
		}
		sender, ok := h.send.(keyedMessageSender)
		if !ok {
			chatError(c, errors.New("keyed send unavailable"))
			return
		}
		message, err = sender.ExecuteWithRequestID(c.Request.Context(), id, actor, body.Content.Value, *body.RequestID)
	} else {
		message, err = h.send.Execute(c.Request.Context(), id, actor, body.Content.Value)
	}
	if err != nil {
		chatError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, toMessageDTO(message))
}
