package websocket

import (
	"net/http"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"github.com/lilpao0/chat_app/backend/internal/presentation/authentication"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
)

type SessionRunner interface {
	Run(*coderws.Conn, domainauth.Identity)
}

type Handler struct {
	tokens   authentication.TokenVerifier
	sessions SessionRunner
}

func NewHandler(tokens authentication.TokenVerifier, sessions SessionRunner) *Handler {
	return &Handler{tokens: tokens, sessions: sessions}
}

func (h *Handler) Handle(c *gin.Context) {
	if c.Request.Method != http.MethodGet {
		response.Error(c, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	for _, origin := range c.Request.Header.Values("Origin") {
		if origin != "" {
			response.Error(c, http.StatusForbidden, "origin_not_allowed", "Browser WebSocket connections are not supported.")
			return
		}
	}
	identity, err := authentication.VerifyBearer(c.Request.Header, h.tokens)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "unauthenticated", "A valid access token is required.")
		return
	}
	connection, err := coderws.Accept(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer connection.CloseNow()
	h.sessions.Run(connection, identity)
}
