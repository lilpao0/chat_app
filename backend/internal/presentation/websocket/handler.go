package websocket

import (
	"net/http"
	"net/url"
	"strings"

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
	origins  []string
	patterns []string
	tickets  *authentication.TicketStore
}

func NewHandler(tokens authentication.TokenVerifier, sessions SessionRunner, origins []string, tickets *authentication.TicketStore) *Handler {
	patterns := make([]string, len(origins))
	for i, origin := range origins {
		// Accept uses path.Match; brackets in IPv6 hosts must remain literal.
		patterns[i] = strings.NewReplacer("[", "\\[", "]", "\\]").Replace(origin)
	}
	return &Handler{tokens: tokens, sessions: sessions, origins: origins, patterns: patterns, tickets: tickets}
}

func (h *Handler) Handle(c *gin.Context) {
	if c.Request.Method != http.MethodGet {
		response.Error(c, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		return
	}
	origin, valid := authentication.SingleOrigin(c.Request.Header)
	if !valid || (origin != "" && !authentication.AllowedOrigin(origin, h.origins)) {
		response.Error(c, http.StatusForbidden, "origin_not_allowed", "Web origin is not allowed.")
		return
	}
	query, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil || len(query) > 1 || (len(query) == 1 && !query.Has("ticket")) ||
		(query.Has("ticket") && (len(query["ticket"]) != 1 || query.Get("ticket") == "" || origin == "")) ||
		(origin != "" && len(c.Request.Header.Values("Authorization")) != 0) {
		response.Error(c, 400, "invalid_input", "Invalid WebSocket credentials.")
		return
	}
	var identity domainauth.Identity
	if origin != "" {
		if h.tickets == nil {
			err = authentication.ErrInvalidTicket
		} else {
			identity, err = h.tickets.Consume(query.Get("ticket"), origin)
		}
		if err != nil {
			response.Error(c, 401, "invalid_ws_ticket", "A valid WebSocket ticket is required.")
			return
		}
	} else {
		identity, err = authentication.VerifyBearer(c.Request.Header, h.tokens)
		if err != nil {
			response.Error(c, 401, "unauthenticated", "A valid access token is required.")
			return
		}
	}
	connection, err := coderws.Accept(c.Writer, c.Request, &coderws.AcceptOptions{OriginPatterns: h.patterns})
	if err != nil {
		return
	}
	defer connection.CloseNow()
	h.sessions.Run(connection, identity)
}
