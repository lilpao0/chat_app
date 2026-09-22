package http_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	dataauth "github.com/lilpao0/chat_app/backend/internal/data/auth"
	datarepo "github.com/lilpao0/chat_app/backend/internal/data/repository"
	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	presentation "github.com/lilpao0/chat_app/backend/internal/presentation/http"
	"github.com/lilpao0/chat_app/backend/internal/presentation/http/middleware"
)

func TestAuthPostgresFlow(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires isolated chat_app_test")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal("cannot open test database")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var name string
	if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&name); err != nil || name != "chat_app_test" {
		t.Fatal("requires connection to chat_app_test")
	}
	ddl, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "000001_create_users.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, strings.Replace(string(ddl), "CREATE TABLE users", "CREATE TEMP TABLE users", 1)); err != nil {
		t.Fatal(err)
	}
	secret := strings.Repeat("s", 32)
	tokens, err := dataauth.NewJWT(secret, "chat-app", "chat-app-mobile", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	users := datarepo.NewPostgresUserRepository(db)
	passwords := dataauth.BcryptPasswordHasher{}
	gin.SetMode(gin.TestMode)
	r, protected := presentation.NewRouter(domainauth.NewRegister(users, passwords), domainauth.NewLogin(users, passwords, tokens), tokens)
	protected.GET("/test-only", func(c *gin.Context) {
		identity, ok := middleware.Identity(c)
		if !ok {
			c.Status(500)
			return
		}
		c.JSON(200, gin.H{"id": identity.UserID})
	})
	request := func(method, path, body, bearer string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		return out
	}
	// Concurrent normalized duplicates must produce exactly one persisted account.
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for _, email := range []string{" AN@example.test ", "an@EXAMPLE.test"} {
		wg.Add(1)
		go func(email string) {
			defer wg.Done()
			body, _ := json.Marshal(map[string]string{"name": " An ", "email": email, "password": "password123"})
			out := request("POST", "/api/auth/register", string(body), "")
			results <- out.Code
		}(email)
	}
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for status := range results {
		counts[status]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatalf("registration statuses: %v", counts)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate registration persisted")
	}
	user, err := users.FindByEmail(ctx, "an@example.test")
	if err != nil {
		t.Fatal(err)
	}
	if user.Name != "An" || user.PasswordHash == "password123" {
		t.Fatal("bad stored registration data")
	}
	login := request("POST", "/api/auth/login", `{"email":" AN@EXAMPLE.TEST ","password":"password123"}`, "")
	if login.Code != 200 {
		t.Fatalf("login status %d", login.Code)
	}
	var payload struct {
		Token     string    `json:"access_token"`
		ExpiresAt time.Time `json:"expires_at"`
		User      struct {
			ID int64 `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	id, err := tokens.Verify(payload.Token)
	if err != nil || id.UserID != user.ID || payload.User.ID != user.ID || !id.ExpiresAt.Equal(payload.ExpiresAt) {
		t.Fatal("login identity or expiry mismatch")
	}
	if strings.Contains(login.Body.String(), user.PasswordHash) || strings.Contains(login.Body.String(), "password123") {
		t.Fatal("credentials leaked")
	}
	valid := request("GET", "/api/test-only?user_id=999", "", payload.Token)
	if valid.Code != 200 {
		t.Fatal("valid token rejected")
	}
	var actor struct {
		ID int64 `json:"id"`
	}
	if json.Unmarshal(valid.Body.Bytes(), &actor) != nil || actor.ID != user.ID {
		t.Fatal("client overrode token identity")
	}
	wrong := request("POST", "/api/auth/login", `{"email":"an@example.test","password":"wrongpassword"}`, "")
	missing := request("POST", "/api/auth/login", `{"email":"missing@example.test","password":"wrongpassword"}`, "")
	if wrong.Code != 401 || missing.Code != 401 || wrong.Body.String() != missing.Body.String() {
		t.Fatal("credential errors differ")
	}
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: "1", Issuer: "chat-app", Audience: jwt.ClaimStrings{"chat-app-mobile"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour))}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	for _, bearer := range []string{"", "invalid", expired} {
		if out := request("GET", "/api/test-only", "", bearer); out.Code != 401 {
			t.Fatal("invalid credentials reached protected handler")
		}
	}
	if request("GET", "/health", "", "").Code != 200 {
		t.Fatal("health requires auth")
	}
	if request("POST", "/api/auth/register", `{`, "").Code != 400 {
		t.Fatal("invalid registration JSON accepted")
	}
	if request("POST", "/api/auth/login", `{"sender_id":1}`, "").Code != 400 {
		t.Fatal("unknown login field accepted")
	}
}
