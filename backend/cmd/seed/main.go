package main

import (
	"context"
	"fmt"
	"log"
	"net/mail"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/joho/godotenv"
	"github.com/lilpao0/chat_app/backend/internal/data/auth"
	"github.com/lilpao0/chat_app/backend/internal/data/database"
	"github.com/lilpao0/chat_app/backend/internal/data/seed"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

func init() {
	_ = godotenv.Load()
}

func account(prefix string) (repository.CreateUser, error) {
	name := strings.TrimSpace(os.Getenv(prefix + "_NAME"))
	email := strings.ToLower(strings.TrimSpace(os.Getenv(prefix + "_EMAIL")))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Name != "" || address.Address != email || len(email) > 254 || !utf8.ValidString(email) || strings.ContainsRune(email, 0) || !utf8.ValidString(name) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 100 || strings.ContainsRune(name, 0) {
		return repository.CreateUser{}, fmt.Errorf("%s requires valid NAME and EMAIL", prefix)
	}
	hash, err := (auth.BcryptPasswordHasher{}).Hash(os.Getenv(prefix + "_PASSWORD"))
	if err != nil {
		return repository.CreateUser{}, fmt.Errorf("%s_PASSWORD does not meet the password policy", prefix)
	}
	return repository.CreateUser{Name: name, Email: email, PasswordHash: hash}, nil
}

func run() error {
	a, err := account("SEED_A")
	if err != nil {
		return err
	}
	b, err := account("SEED_B")
	if err != nil {
		return err
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, url)
	if err != nil {
		return fmt.Errorf("cannot connect to seed database")
	}
	defer db.Close()
	result, err := seed.Demo(ctx, db, a, b)
	if err != nil {
		return fmt.Errorf("seed failed; check database migrations and account inputs")
	}
	log.Printf("demo ready: users=%d,%d conversation=%d; existing credentials preserved", result.UserAID, result.UserBID, result.ConversationID)
	return nil
}
func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
