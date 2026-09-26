package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
)

type RefreshUseCase interface {
	Execute(context.Context, string) (auth.AccessToken, error)
}

type RefreshHandler struct {
	usecase RefreshUseCase
}

func NewRefreshHandler(usecase RefreshUseCase) *RefreshHandler {
	return &RefreshHandler{usecase: usecase}
}

func (h *RefreshHandler) Handle(c *gin.Context) {
	var input struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !readJSON(c, &input) {
		return
	}
	token, err := h.usecase.Execute(c.Request.Context(), input.RefreshToken)
	if errors.Is(err, auth.ErrInvalidToken) {
		response.Error(c, http.StatusUnauthorized, "invalid_refresh_token", "A valid refresh token is required.")
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "internal_error", "Unable to complete the request.")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"access_token": token.Value,
		"expires_at":   token.ExpiresAt.UTC().Format(time.RFC3339),
	})
}
