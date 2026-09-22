package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
)

type RegisterUseCase interface {
	Execute(context.Context, auth.RegisterInput) (auth.PublicUser, error)
}
type RegisterHandler struct{ usecase RegisterUseCase }

func NewRegisterHandler(usecase RegisterUseCase) *RegisterHandler {
	return &RegisterHandler{usecase: usecase}
}

type userDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

func toUserDTO(u auth.PublicUser) userDTO {
	return userDTO{ID: u.ID, Name: u.Name, Email: u.Email, AvatarURL: u.AvatarURL}
}

func readJSON(c *gin.Context, dst any) bool {
	contentType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || contentType != "application/json" {
		response.Error(c, 415, "unsupported_media_type", "Use application/json.")
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.Error(c, 413, "payload_too_large", "Request body is too large.")
		} else {
			response.Error(c, 400, "invalid_input", "Invalid request body.")
		}
		return false
	}
	if !utf8.Valid(body) {
		response.Error(c, 400, "invalid_input", "Invalid JSON.")
		return false
	}
	// Decode once into a DTO, rejecting unknown fields and trailing JSON values.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		response.Error(c, 400, "invalid_input", "Expected a JSON object.")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		response.Error(c, 400, "invalid_input", "Invalid JSON fields.")
		return false
	}
	return true
}

func (h *RegisterHandler) Handle(c *gin.Context) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !readJSON(c, &input) {
		return
	}
	user, err := h.usecase.Execute(c.Request.Context(), auth.RegisterInput{Name: input.Name, Email: input.Email, Password: input.Password})
	if errors.Is(err, auth.ErrInvalidInput) {
		response.Error(c, 400, "invalid_input", "The submitted information is invalid.")
		return
	}
	if errors.Is(err, entity.ErrEmailTaken) {
		response.Error(c, 409, "email_taken", "Email is already in use.")
		return
	}
	if err != nil {
		response.Error(c, 500, "internal_error", "Unable to complete the request.")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": toUserDTO(user)})
}
