package http

import (
	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/handler"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/middleware"
)

// NewRouter returns the authenticated group for future chat route registration.
// Public auth and health endpoints are intentionally outside that group.
func NewRouter(register handler.RegisterUseCase, login handler.LoginUseCase, tokens middleware.TokenVerifier) (*gin.Engine, *gin.RouterGroup) {
	r := gin.New()
	r.GET("/health", handler.Health)
	r.POST("/api/auth/register", handler.NewRegisterHandler(register).Handle)
	r.POST("/api/auth/login", handler.NewLoginHandler(login).Handle)
	return r, r.Group("/api", middleware.Authenticate(tokens))
}
