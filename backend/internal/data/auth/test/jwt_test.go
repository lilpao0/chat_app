package auth_test

import (
	"errors"
	dataauth "github.com/lilpao0/chat_app/backend/internal/data/auth"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
)

func TestJWT(t *testing.T) {
	secret := strings.Repeat("s", 32)
	adapter, err := dataauth.NewJWT(secret, "chat-app", "chat-app-mobile", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	issued, err := adapter.Issue(42)
	if err != nil {
		t.Fatal(err)
	}
	id, err := adapter.Verify(issued.Value)
	if err != nil || id.UserID != 42 || !id.ExpiresAt.Equal(issued.ExpiresAt) {
		t.Fatal("round trip failed")
	}
	if _, err := adapter.Issue(0); err == nil {
		t.Fatal("zero user accepted")
	}
	base := func() jwt.RegisteredClaims {
		return jwt.RegisteredClaims{Subject: "42", Issuer: "chat-app", Audience: jwt.ClaimStrings{"chat-app-mobile"}, ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}
	}
	for _, tc := range []struct {
		name   string
		change func(*jwt.RegisteredClaims)
		method jwt.SigningMethod
		key    any
	}{
		{"expired", func(c *jwt.RegisteredClaims) { c.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Hour)) }, jwt.SigningMethodHS256, []byte(secret)},
		{"missing_exp", func(c *jwt.RegisteredClaims) { c.ExpiresAt = nil }, jwt.SigningMethodHS256, []byte(secret)},
		{"missing_sub", func(c *jwt.RegisteredClaims) { c.Subject = "" }, jwt.SigningMethodHS256, []byte(secret)},
		{"invalid_sub", func(c *jwt.RegisteredClaims) { c.Subject = "-1" }, jwt.SigningMethodHS256, []byte(secret)},
		{"wrong_issuer", func(c *jwt.RegisteredClaims) { c.Issuer = "other" }, jwt.SigningMethodHS256, []byte(secret)},
		{"wrong_audience", func(c *jwt.RegisteredClaims) { c.Audience = nil }, jwt.SigningMethodHS256, []byte(secret)},
		{"future_iat", func(c *jwt.RegisteredClaims) { c.IssuedAt = jwt.NewNumericDate(now.Add(time.Hour)) }, jwt.SigningMethodHS256, []byte(secret)},
		{"future_nbf", func(c *jwt.RegisteredClaims) { c.NotBefore = jwt.NewNumericDate(now.Add(time.Hour)) }, jwt.SigningMethodHS256, []byte(secret)},
		{"wrong_signature", func(*jwt.RegisteredClaims) {}, jwt.SigningMethodHS256, []byte(strings.Repeat("x", 32))},
		{"wrong_algorithm", func(*jwt.RegisteredClaims) {}, jwt.SigningMethodHS384, []byte(secret)},
		{"none", func(*jwt.RegisteredClaims) {}, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := base()
			tc.change(&claims)
			raw, err := jwt.NewWithClaims(tc.method, claims).SignedString(tc.key)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := adapter.Verify(raw); !errors.Is(err, domainauth.ErrInvalidToken) {
				t.Fatal("invalid token accepted")
			}
		})
	}
	if _, err := adapter.Verify("not.a.token"); !errors.Is(err, domainauth.ErrInvalidToken) {
		t.Fatal("malformed token accepted")
	}
}
