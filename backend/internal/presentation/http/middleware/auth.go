package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"github.com/lilpao0/chat_app/backend/internal/presentation/authentication"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
)

type TokenVerifier = authentication.TokenVerifier

func Authenticate(tokens TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, err := authentication.VerifyBearer(
			c.Request.Header,
			tokens,
		)
		if err != nil {
			response.Error(
				c,
				401,
				"unauthenticated",
				"A valid access token is required.",
			)
			return
		}

		c.Set(identityKey, identity)
		c.Next()
	}
}

const identityKey = "chat.auth.identity"

func Identity(c *gin.Context) (auth.Identity, bool) {
	value, ok := c.Get(identityKey)
	if !ok {
		return auth.Identity{}, false
	}
	identity, ok := value.(auth.Identity)
	return identity, ok
}
