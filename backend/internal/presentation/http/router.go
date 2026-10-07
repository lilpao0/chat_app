package http

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/lilpao0/chat_app/backend/internal/presentation/authentication"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/handler"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/middleware"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/response"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/swagger"
)

// NewRouter returns the authenticated group used by the protected application routes.
// Public auth and health endpoints are intentionally outside that group.
func NewRouter(register handler.RegisterUseCase, login handler.LoginUseCase, refresh handler.RefreshUseCase, tokens middleware.TokenVerifier, origins []string) (*gin.Engine, *gin.RouterGroup) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if _, valid := authentication.SingleOrigin(c.Request.Header); !valid {
			response.Error(c, 403, "origin_not_allowed", "Web origin is not allowed.")
		}
	})
	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool { return authentication.AllowedOrigin(origin, origins) },
		AllowMethods:    []string{"GET", "POST", "PATCH", "HEAD", "OPTIONS"},
		AllowHeaders:    []string{"Authorization", "Content-Type", "Origin", "Content-Length"},
		MaxAge:          5 * time.Minute,
	}))
	r.GET("/health", handler.Health)
	swagger.Register(r)
	r.POST("/api/auth/register", handler.NewRegisterHandler(register).Handle)
	r.POST("/api/auth/login", handler.NewLoginHandler(login).Handle)
	r.POST("/api/auth/refresh", handler.NewRefreshHandler(refresh).Handle)
	return r, r.Group("/api", middleware.Authenticate(tokens))
}

func RegisterWebSocketRoute(engine *gin.Engine, handle gin.HandlerFunc) {
	engine.GET("/ws", handle)
}

// ProtectedHandlers contains every authenticated REST operation exposed by
// the application. Keeping route registration together lets contract tests
// compare the real Gin route table with the OpenAPI document.
type ProtectedHandlers struct {
	WSTicket          gin.HandlerFunc
	SearchUsers       gin.HandlerFunc
	OpenDirect        gin.HandlerFunc
	ListConversations gin.HandlerFunc
	SendMessage       gin.HandlerFunc
	MessageHistory    gin.HandlerFunc
	MarkRead          gin.HandlerFunc
	GetPrivateProfile gin.HandlerFunc
	GetPublicProfile  gin.HandlerFunc
	UpdateProfile     gin.HandlerFunc
}

func RegisterProtectedRoutes(group *gin.RouterGroup, handlers ProtectedHandlers) {
	group.POST("/ws/tickets", handlers.WSTicket)
	group.GET("/users", handlers.SearchUsers)
	group.POST("/conversations/direct", handlers.OpenDirect)
	group.GET("/conversations", handlers.ListConversations)
	group.POST("/conversations/:id/messages", handlers.SendMessage)
	group.GET("/conversations/:id/messages", handlers.MessageHistory)
	group.POST("/conversations/:id/read", handlers.MarkRead)
	group.GET("/users/me", handlers.GetPrivateProfile)
	group.PATCH("/users/me", handlers.UpdateProfile)
	group.GET("/users/:id", handlers.GetPublicProfile)
}
