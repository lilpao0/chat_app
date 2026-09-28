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
	presentationws "github.com/lilpao0/chat_app/backend/internal/presentation/websocket"
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
	wsCfg, err := config.LoadWebSocket()
	if err != nil {
		return fmt.Errorf("load WebSocket config: %w", err)
	}
	tokens, err := dataauth.NewJWTWithRefreshTTL(jwtCfg.Secret, jwtCfg.Issuer, jwtCfg.Audience, jwtCfg.TTL, jwtCfg.RefreshTTL)
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
	profiles := handler.NewProfileHandler(
		user.NewGetPrivateProfile(users),
		user.NewGetPublicProfile(users),
		user.NewUpdateProfile(users, time.Now),
	)
	passwords := dataauth.BcryptPasswordHasher{}
	register := domainauth.NewRegister(users, passwords)
	login := domainauth.NewLogin(users, passwords, tokens)
	refresh := domainauth.NewRefresh(tokens)
	r, protected := presentation.NewRouter(register, login, refresh, tokens)
	commands := &presentationws.Commands{}
	hub := presentationws.NewHub(presentationws.LifecycleConfig{
		CommandQueueCapacity: wsCfg.CommandQueueCapacity, CommandTimeout: wsCfg.CommandTimeout, CommandRate: wsCfg.CommandRate, CommandBurst: wsCfg.CommandBurst,
		SendQueueCapacity: wsCfg.SendQueueCapacity,
		MaxMessageBytes:   wsCfg.MaxMessageBytes,
		WriteTimeout:      wsCfg.WriteTimeout,
		PongTimeout:       wsCfg.PongTimeout,
		PingInterval:      wsCfg.PingInterval,
	}, commands)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), wsCfg.ShutdownTimeout)
		defer cancel()
		if err := hub.Shutdown(ctx); err != nil {
			log.Printf("WebSocket shutdown: %v", err)
		}
	}()
	wsHandler := presentationws.NewHandler(tokens, hub)
	presentation.RegisterWebSocketRoute(r, wsHandler.Handle)
	conversations := datarepo.NewPostgresConversationRepository(db)
	discovery := handler.NewDiscoveryHandler(user.NewSearch(users), conversation.NewOpenDirect(conversations))
	messages := datarepo.NewPostgresMessageRepository(db)
	send := message.NewSendWithPublisher(messages, presentationws.NewPublisher(conversations, hub), func(err error) { log.Printf("realtime publication: %v", err) })
	commands.Send = send
	commands.Read = conversation.NewMarkRead(messages)
	presentation.RegisterProtectedRoutes(protected, presentation.ProtectedHandlers{
		SearchUsers:       discovery.Search,
		GetPrivateProfile: profiles.GetPrivate,
		GetPublicProfile:  profiles.GetPublic,
		UpdateProfile:     profiles.Update,
		OpenDirect:        discovery.OpenDirect,
		ListConversations: handler.NewConversationsHandler(conversation.NewList(conversations)).List,
		SendMessage:       handler.NewSendMessageHandler(send).Send,
		MessageHistory:    handler.NewHistoryHandler(message.NewHistory(messages)).Get,
		MarkRead:          handler.NewReadHandler(conversation.NewMarkRead(messages)).Mark,
	})

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
		_ = server.Close()
		return fmt.Errorf("server shutdown: %w", err)
	}
	wsShutdownCtx, wsShutdownCancel := context.WithTimeout(context.Background(), wsCfg.ShutdownTimeout)
	defer wsShutdownCancel()
	if err := hub.Shutdown(wsShutdownCtx); err != nil {
		return fmt.Errorf("WebSocket shutdown: %w", err)
	}

	log.Println("server gracefully stopped")
	return nil
}
