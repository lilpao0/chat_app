package handler

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"net/url"
	"strconv"
)

type HistoryUseCase interface {
	Execute(context.Context, int64, int64, entity.MessageQuery) (entity.MessagePage, error)
}
type HistoryHandler struct{ history HistoryUseCase }

func NewHistoryHandler(history HistoryUseCase) *HistoryHandler {
	return &HistoryHandler{history: history}
}

func (h *HistoryHandler) Get(c *gin.Context) {
	id, actor, ok := chatActor(c)
	if !ok {
		return
	}
	query, err := parseHistoryQuery(c)
	if err != nil {
		chatError(c, err)
		return
	}
	page, err := h.history.Execute(c.Request.Context(), id, actor, query)
	if err != nil {
		chatError(c, err)
		return
	}
	items := make([]messageDTO, 0, len(page.Items))
	for _, m := range page.Items {
		items = append(items, toMessageDTO(m))
	}
	c.JSON(200, gin.H{"items": items, "has_more": page.HasMore, "next_before_id": page.NextBeforeID, "next_after_id": page.NextAfterID})
}

func parseHistoryQuery(c *gin.Context) (entity.MessageQuery, error) {
	q := entity.MessageQuery{}
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		return q, entity.ErrInvalidInput
	}
	for key, entries := range values {
		if key != "before_id" && key != "after_id" && key != "limit" {
			return q, entity.ErrInvalidInput
		}
		if len(entries) != 1 {
			return q, entity.ErrInvalidInput
		}
		value, err := strconv.ParseInt(entries[0], 10, 64)
		if err != nil || value <= 0 {
			return q, entity.ErrInvalidInput
		}
		switch key {
		case "before_id":
			q.BeforeID = value
		case "after_id":
			q.AfterID = value
		case "limit":
			if value > 100 {
				return q, entity.ErrInvalidInput
			}
			q.Limit = int(value)
		}
	}
	return q, nil
}
