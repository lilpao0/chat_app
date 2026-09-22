package seed_test

import (
	"context"
	"github.com/lilpao0/chat_app/backend/internal/data/auth"
	datarepo "github.com/lilpao0/chat_app/backend/internal/data/repository"
	"github.com/lilpao0/chat_app/backend/internal/data/seed"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"github.com/lilpao0/chat_app/backend/internal/testutil"
	"strings"
	"testing"
	"time"
)

func TestDemo(t *testing.T) {
	db := testutil.Database(t)
	ctx := context.Background()
	passwords := auth.BcryptPasswordHasher{}
	hash, err := passwords.Hash("demo-password")
	if err != nil {
		t.Fatal(err)
	}
	a := repository.CreateUser{Name: "A", Email: "a@example.test", PasswordHash: hash}
	b := repository.CreateUser{Name: "B", Email: "b@example.test", PasswordHash: hash}
	first, err := seed.Demo(ctx, db, a, b)
	if err != nil {
		t.Fatal(err)
	}
	a.Name = "replacement"
	a.PasswordHash = "must-not-overwrite"
	second, err := seed.Demo(ctx, db, a, b)
	if err != nil || first != second {
		t.Fatal("seed is not idempotent")
	}
	var users, conversations, members int
	if err := db.QueryRow(`SELECT (SELECT count(*) FROM users),(SELECT count(*) FROM conversations),(SELECT count(*) FROM conversation_members)`).Scan(&users, &conversations, &members); err != nil || users != 2 || conversations != 1 || members != 2 {
		t.Fatal("unexpected seed counts")
	}
	stored, err := datarepo.NewPostgresUserRepository(db).FindByEmail(ctx, a.Email)
	if err != nil || stored.Name != "A" || stored.PasswordHash != hash {
		t.Fatal("existing user overwritten")
	}
	tokens, err := auth.NewJWT(strings.Repeat("s", 32), "test", "test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	login := domainauth.NewLogin(datarepo.NewPostgresUserRepository(db), passwords, tokens)
	for _, email := range []string{a.Email, b.Email} {
		if _, err := login.Execute(ctx, domainauth.LoginInput{Email: email, Password: "demo-password"}); err != nil {
			t.Fatal("seed account cannot log in")
		}
	}
	if _, err := seed.Demo(ctx, db, b, b); err == nil {
		t.Fatal("identical accounts accepted")
	}
}
