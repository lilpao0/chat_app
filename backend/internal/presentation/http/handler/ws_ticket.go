package handler

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/presentation/authentication"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/middleware"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
)

func NewWSTicketHandler(store *authentication.TicketStore, origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		identity, ok := middleware.Identity(c)
		if !ok || !time.Now().Before(identity.ExpiresAt) {
			response.Error(c, 401, "unauthenticated", "A valid access token is required.")
			return
		}
		origin, valid := authentication.SingleOrigin(c.Request.Header)
		if !valid || !authentication.AllowedOrigin(origin, origins) {
			response.Error(c, 403, "origin_not_allowed", "Web origin is not allowed.")
			return
		}
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1))
		if err != nil || len(body) != 0 {
			response.Error(c, 400, "invalid_input", "This endpoint requires an empty request body.")
			return
		}
		ticket, expires, err := store.Issue(identity, origin)
		switch {
		case errors.Is(err, authentication.ErrInvalidTicket):
			response.Error(c, 401, "unauthenticated", "A valid access token is required.")
		case errors.Is(err, authentication.ErrTicketCapacity):
			response.Error(c, 429, "rate_limited", "Too many pending WebSocket tickets.")
		case err != nil:
			response.Error(c, 500, "internal_error", "Unable to complete the request.")
		default:
			response.Success(c, http.StatusCreated, gin.H{"ticket": ticket, "expires_at": expires.UTC().Format(time.RFC3339Nano)})
		}
	}
}
