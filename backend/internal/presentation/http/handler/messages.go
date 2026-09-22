package handler

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/middleware"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
	"strconv"
	"time"
)

type SendMessageUseCase interface {
	Execute(context.Context, int64, int64, string) (entity.Message, error)
}
type SendMessageHandler struct{ send SendMessageUseCase }

func NewSendMessageHandler(send SendMessageUseCase) *SendMessageHandler {
	return &SendMessageHandler{send: send}
}

type messageDTO struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversation_id"`
	SenderID       int64     `json:"sender_id"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

func toMessageDTO(m entity.Message) messageDTO {
	return messageDTO{ID: m.ID, ConversationID: m.ConversationID, SenderID: m.SenderID, Content: m.Content, CreatedAt: m.CreatedAt.UTC()}
}
func chatError(c *gin.Context, err error) {
	switch {
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
		Content string `json:"content"`
	}
	if !readJSON(c, &body) {
		return
	}
	message, err := h.send.Execute(c.Request.Context(), id, actor, body.Content)
	if err != nil {
		chatError(c, err)
		return
	}
	c.JSON(201, gin.H{"message": toMessageDTO(message)})
}
