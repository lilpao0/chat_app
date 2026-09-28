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

type testTokenClaims struct {
	Type string `json:"type"`
	jwt.RegisteredClaims
}

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
	refresh, err := adapter.IssueRefresh(42)
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := adapter.Refresh(refresh.Value)
	if err != nil || refreshed.Value == "" {
		t.Fatal("refresh round trip failed")
	}
	if _, err := adapter.Verify(refresh.Value); !errors.Is(err, domainauth.ErrInvalidToken) {
		t.Fatal("refresh token accepted as access token")
	}
	if _, err := adapter.Refresh(issued.Value); !errors.Is(err, domainauth.ErrInvalidToken) {
		t.Fatal("access token accepted as refresh token")
	}
	expiredRefreshClaims := testTokenClaims{Type: "refresh", RegisteredClaims: jwt.RegisteredClaims{
		Subject: "42", Issuer: "chat-app", Audience: jwt.ClaimStrings{"chat-app-mobile"},
		IssuedAt: jwt.NewNumericDate(now.Add(-2 * time.Hour)), ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
	}}
	expiredRefresh, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredRefreshClaims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Refresh(expiredRefresh); !errors.Is(err, domainauth.ErrInvalidToken) {
		t.Fatal("expired refresh token accepted")
	}
	base := func() testTokenClaims {
		return testTokenClaims{Type: "access", RegisteredClaims: jwt.RegisteredClaims{Subject: "42", Issuer: "chat-app", Audience: jwt.ClaimStrings{"chat-app-mobile"}, ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}}
	}
	for _, tc := range []struct {
		name   string
		change func(*testTokenClaims)
		method jwt.SigningMethod
		key    any
	}{
		{"expired", func(c *testTokenClaims) { c.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Hour)) }, jwt.SigningMethodHS256, []byte(secret)},
		{"missing_exp", func(c *testTokenClaims) { c.ExpiresAt = nil }, jwt.SigningMethodHS256, []byte(secret)},
		{"missing_sub", func(c *testTokenClaims) { c.Subject = "" }, jwt.SigningMethodHS256, []byte(secret)},
		{"invalid_sub", func(c *testTokenClaims) { c.Subject = "-1" }, jwt.SigningMethodHS256, []byte(secret)},
		{"wrong_issuer", func(c *testTokenClaims) { c.Issuer = "other" }, jwt.SigningMethodHS256, []byte(secret)},
		{"wrong_audience", func(c *testTokenClaims) { c.Audience = nil }, jwt.SigningMethodHS256, []byte(secret)},
		{"wrong_type", func(c *testTokenClaims) { c.Type = "refresh" }, jwt.SigningMethodHS256, []byte(secret)},
		{"missing_type", func(c *testTokenClaims) { c.Type = "" }, jwt.SigningMethodHS256, []byte(secret)},
		{"future_iat", func(c *testTokenClaims) { c.IssuedAt = jwt.NewNumericDate(now.Add(time.Hour)) }, jwt.SigningMethodHS256, []byte(secret)},
		{"future_nbf", func(c *testTokenClaims) { c.NotBefore = jwt.NewNumericDate(now.Add(time.Hour)) }, jwt.SigningMethodHS256, []byte(secret)},
		{"wrong_signature", func(*testTokenClaims) {}, jwt.SigningMethodHS256, []byte(strings.Repeat("x", 32))},
		{"wrong_algorithm", func(*testTokenClaims) {}, jwt.SigningMethodHS384, []byte(secret)},
		{"none", func(*testTokenClaims) {}, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType},
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
