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

type LoginUseCase interface {
	Execute(context.Context, auth.LoginInput) (auth.LoginResult, error)
}
type LoginHandler struct{ usecase LoginUseCase }

func NewLoginHandler(usecase LoginUseCase) *LoginHandler { return &LoginHandler{usecase: usecase} }

func (h *LoginHandler) Handle(c *gin.Context) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !readJSON(c, &input) {
		return
	}
	result, err := h.usecase.Execute(c.Request.Context(), auth.LoginInput{Email: input.Email, Password: input.Password})
	if errors.Is(err, auth.ErrInvalidInput) {
		response.Error(c, 400, "invalid_input", "The submitted information is invalid.")
		return
	}
	if errors.Is(err, auth.ErrInvalidCredentials) {
		response.Error(c, 401, "unauthenticated", "Invalid email or password.")
		return
	}
	if err != nil {
		response.Error(c, 500, "internal_error", "Unable to complete the request.")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"user": toUserDTO(result.User), "access_token": result.Token.Value, "expires_at": result.Token.ExpiresAt.UTC().Format(time.RFC3339)})
}
