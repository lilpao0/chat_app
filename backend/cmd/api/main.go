package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/lilpao0/chat_app/backend/cmd/api/config"
	dataauth "github.com/lilpao0/chat_app/backend/internal/data/auth"
	"github.com/lilpao0/chat_app/backend/internal/data/database"
	datarepo "github.com/lilpao0/chat_app/backend/internal/data/repository"
	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/conversation"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/message"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/user"
	presentation "github.com/lilpao0/chat_app/backend/internal/presentation/http"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/handler"
)

func main() {
	// Optional local defaults; explicit process variables take precedence.
	_ = godotenv.Load()
	if err := run(); err != nil {
		log.Printf("application stopped:%v", err)
		os.Exit(1)
	}
}
func run() error {
	httpCfg, err := config.LoadHTTPPort()
	if err != nil {
		return fmt.Errorf("load HTTP config: %w", err)
	}

	jwtCfg, err := config.LoadJWT()
	if err != nil {
		return fmt.Errorf("load JWT config: %w", err)
	}
	tokens, err := dataauth.NewJWT(jwtCfg.Secret, jwtCfg.Issuer, jwtCfg.Audience, jwtCfg.TTL)
	if err != nil {
		return err
	}

	dbCfg, err := config.LoadDB()
	if err != nil {
		return fmt.Errorf("load db config: %w", err)
	}

	startupCtx, startupCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	db, err := database.Open(startupCtx, dbCfg.URL)
	startupCancel()
	if err != nil {
		return fmt.Errorf("db startup: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("database close: %v", err)
		}
	}()
	log.Println("db: connected")

	gin.SetMode(gin.ReleaseMode)
	users := datarepo.NewPostgresUserRepository(db)
	passwords := dataauth.BcryptPasswordHasher{}
	register := domainauth.NewRegister(users, passwords)
	login := domainauth.NewLogin(users, passwords, tokens)
	r, protected := presentation.NewRouter(register, login, tokens)
	conversations := datarepo.NewPostgresConversationRepository(db)
	discovery := handler.NewDiscoveryHandler(user.NewSearch(users), conversation.NewOpenDirect(conversations))
	protected.GET("/users", discovery.Search)
	protected.POST("/conversations/direct", discovery.OpenDirect)
	protected.GET("/conversations", handler.NewConversationsHandler(conversation.NewList(conversations)).List)
	messages := datarepo.NewPostgresMessageRepository(db)
	protected.POST("/conversations/:id/messages", handler.NewSendMessageHandler(message.NewSend(messages)).Send)
	protected.GET("/conversations/:id/messages", handler.NewHistoryHandler(message.NewHistory(messages)).Get)
	protected.POST("/conversations/:id/read", handler.NewReadHandler(conversation.NewMarkRead(messages)).Mark)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", httpCfg.Port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	serverErr := make(chan error, 1)

	go func() {
		log.Printf("listening on %s", server.Addr)
		serverErr <- server.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	case <-quit:
		log.Println("shutting down...")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	log.Println("server gracefully stopped")
	return nil
}
