package websocket_test

import (
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	dataauth "github.com/lilpao0/chat_app/backend/internal/data/auth"
	ws "github.com/lilpao0/chat_app/backend/internal/presentation/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRealTokensCannotBypassHandshake(t *testing.T) {
	secret := strings.Repeat("r", 32)
	tokens, err := dataauth.NewJWT(secret, "test", "mobile", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	refresh, err := tokens.IssueRefresh(1)
	if err != nil {
		t.Fatal(err)
	}
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": "test", "aud": "mobile", "sub": "1", "type": "access", "iat": time.Now().Add(-time.Hour).Unix(), "exp": time.Now().Add(-time.Minute).Unix(),
	}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ws", ws.NewHandler(tokens, ws.NewHub(testLifecycleConfig()), nil, nil).Handle)
	server := httptest.NewServer(router)
	defer server.Close()
	for _, token := range []string{refresh.Value, expired} {
		headers := http.Header{"Authorization": []string{"Bearer " + token}}
		assertHandshakeFailure(t, server, headers, 401, "unauthenticated")
	}
}
