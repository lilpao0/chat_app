package handler

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/middleware"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
	"net/url"
	"strconv"
)

type SearchUsersUseCase interface {
	Execute(context.Context, int64, entity.UserSearchQuery) (entity.UserSearchPage, error)
}
type OpenDirectUseCase interface {
	Execute(context.Context, int64, int64) (entity.DirectConversation, error)
}
type DiscoveryHandler struct {
	search SearchUsersUseCase
	direct OpenDirectUseCase
}

func NewDiscoveryHandler(search SearchUsersUseCase, direct OpenDirectUseCase) *DiscoveryHandler {
	return &DiscoveryHandler{search: search, direct: direct}
}

func (h *DiscoveryHandler) Search(c *gin.Context) {
	actor, ok := middleware.Identity(c)
	if !ok {
		response.Error(c, 401, "unauthenticated", "A valid access token is required.")
		return
	}
	q, err := parseSearchQuery(c.Request.URL.RawQuery)
	if err != nil {
		chatError(c, err)
		return
	}
	page, err := h.search.Execute(c.Request.Context(), actor.UserID, q)
	if err != nil {
		chatError(c, err)
		return
	}
	items := make([]participantDTO, 0, len(page.Items))
	for _, user := range page.Items {
		items = append(items, participantDTO{ID: user.ID, Name: user.Name, AvatarURL: user.AvatarURL})
	}
	c.JSON(200, gin.H{"items": items, "has_more": page.HasMore, "next_after_id": page.NextAfterID})
}

func parseSearchQuery(raw string) (entity.UserSearchQuery, error) {
	q := entity.UserSearchQuery{}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return q, entity.ErrInvalidInput
	}
	for key, entries := range values {
		if len(entries) != 1 {
			return q, entity.ErrInvalidInput
		}
		switch key {
		case "q":
			q.Text = entries[0]
		case "limit", "after_id":
			value, err := strconv.ParseInt(entries[0], 10, 64)
			if err != nil || value <= 0 {
				return q, entity.ErrInvalidInput
			}
			if key == "limit" {
				if value > 50 {
					return q, entity.ErrInvalidInput
				}
				q.Limit = int(value)
			} else {
				q.AfterID = value
			}
		default:
			return q, entity.ErrInvalidInput
		}
	}
	return q, nil
}

func (h *DiscoveryHandler) OpenDirect(c *gin.Context) {
	actor, ok := middleware.Identity(c)
	if !ok {
		response.Error(c, 401, "unauthenticated", "A valid access token is required.")
		return
	}
	var body struct {
		UserID int64 `json:"user_id"`
	}
	if !readJSON(c, &body) {
		return
	}
	conversation, err := h.direct.Execute(c.Request.Context(), actor.UserID, body.UserID)
	if errors.Is(err, entity.ErrUserNotFound) {
		response.Error(c, 404, "not_found", "User not found.")
		return
	}
	if err != nil {
		chatError(c, err)
		return
	}
	status := 200
	if conversation.Created {
		status = 201
	}
	c.JSON(status, gin.H{"conversation": gin.H{"id": conversation.ID, "user": participantDTO{ID: conversation.Participant.ID, Name: conversation.Participant.Name, AvatarURL: conversation.Participant.AvatarURL}}})
}
