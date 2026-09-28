package handler

import (
	"context"
	"github.com/gin-gonic/gin"
)

type MarkReadUseCase interface {
	Execute(context.Context, int64, int64, int64) (int64, error)
}
type ReadHandler struct{ read MarkReadUseCase }

func NewReadHandler(read MarkReadUseCase) *ReadHandler { return &ReadHandler{read: read} }
func (h *ReadHandler) Mark(c *gin.Context) {
	id, actor, ok := chatActor(c)
	if !ok {
		return
	}
	var body struct {
		LastReadMessageID int64 `json:"last_read_message_id"`
	}
	if !readJSON(c, &body) {
		return
	}
	marker, err := h.read.Execute(c.Request.Context(), id, actor, body.LastReadMessageID)
	if err != nil {
		chatError(c, err)
		return
	}
	c.JSON(200, gin.H{"conversation_id": id, "last_read_message_id": marker})
}
