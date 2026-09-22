package handler

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/middleware"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
	"time"
)

type ListConversationsUseCase interface {
	Execute(context.Context, int64) ([]entity.ConversationSummary, error)
}
type ConversationsHandler struct{ list ListConversationsUseCase }

func NewConversationsHandler(list ListConversationsUseCase) *ConversationsHandler {
	return &ConversationsHandler{list: list}
}

type participantDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}
type conversationDTO struct {
	ID            int64          `json:"id"`
	User          participantDTO `json:"user"`
	LastMessage   *string        `json:"last_message"`
	LastMessageAt *time.Time     `json:"last_message_at"`
	UnreadCount   int64          `json:"unread_count"`
}

func (h *ConversationsHandler) List(c *gin.Context) {
	actor, ok := middleware.Identity(c)
	if !ok {
		response.Error(c, 401, "unauthenticated", "A valid access token is required.")
		return
	}
	items, err := h.list.Execute(c.Request.Context(), actor.UserID)
	if err != nil {
		response.Error(c, 500, "internal_error", "Unable to complete the request.")
		return
	}
	result := make([]conversationDTO, 0, len(items))
	for _, item := range items {
		result = append(result, conversationDTO{ID: item.ID, User: participantDTO{ID: item.Participant.ID, Name: item.Participant.Name, AvatarURL: item.Participant.AvatarURL}, LastMessage: item.LastMessage, LastMessageAt: item.LastMessageAt, UnreadCount: item.UnreadCount})
	}
	c.JSON(200, result)
}
