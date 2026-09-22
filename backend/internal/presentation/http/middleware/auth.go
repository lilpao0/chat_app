package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
)

type TokenVerifier interface {
	Verify(string) (auth.Identity, error)
}

const identityKey = "chat.auth.identity"

func Authenticate(tokens TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		headers := c.Request.Header.Values("Authorization")
		if len(headers) != 1 {
			response.Error(c, 401, "unauthenticated", "A valid access token is required.")
			return
		}
		parts := strings.Fields(headers[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Error(c, 401, "unauthenticated", "A valid access token is required.")
			return
		}
		identity, err := tokens.Verify(parts[1])
		if err != nil || identity.UserID <= 0 {
			response.Error(c, 401, "unauthenticated", "A valid access token is required.")
			return
		}
		c.Set(identityKey, identity)
		c.Next()
	}
}

func Identity(c *gin.Context) (auth.Identity, bool) {
	value, ok := c.Get(identityKey)
	if !ok {
		return auth.Identity{}, false
	}
	identity, ok := value.(auth.Identity)
	return identity, ok
}
