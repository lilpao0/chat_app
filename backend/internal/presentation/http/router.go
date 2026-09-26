package http

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/handler"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/middleware"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/swagger"
)

// NewRouter returns the authenticated group for future chat route registration.
// Public auth and health endpoints are intentionally outside that group.
func NewRouter(register handler.RegisterUseCase, login handler.LoginUseCase, refresh handler.RefreshUseCase, tokens middleware.TokenVerifier) (*gin.Engine, *gin.RouterGroup) {
	r := gin.New()
	r.Use(cors.Default())
	r.GET("/health", handler.Health)
	swagger.Register(r)
	r.POST("/api/auth/register", handler.NewRegisterHandler(register).Handle)
	r.POST("/api/auth/login", handler.NewLoginHandler(login).Handle)
	r.POST("/api/auth/refresh", handler.NewRefreshHandler(refresh).Handle)
	return r, r.Group("/api", middleware.Authenticate(tokens))
}

// ProtectedHandlers contains every authenticated REST operation exposed by
// the application. Keeping route registration together lets contract tests
// compare the real Gin route table with the OpenAPI document.
type ProtectedHandlers struct {
	SearchUsers       gin.HandlerFunc
	OpenDirect        gin.HandlerFunc
	ListConversations gin.HandlerFunc
	SendMessage       gin.HandlerFunc
	MessageHistory    gin.HandlerFunc
	MarkRead          gin.HandlerFunc
}

func RegisterProtectedRoutes(group *gin.RouterGroup, handlers ProtectedHandlers) {
	group.GET("/users", handlers.SearchUsers)
	group.POST("/conversations/direct", handlers.OpenDirect)
	group.GET("/conversations", handlers.ListConversations)
	group.POST("/conversations/:id/messages", handlers.SendMessage)
	group.GET("/conversations/:id/messages", handlers.MessageHistory)
	group.POST("/conversations/:id/read", handlers.MarkRead)
}
