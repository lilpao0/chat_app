package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadJWT(t *testing.T) {
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("JWT_ISSUER", "")
	t.Setenv("JWT_AUDIENCE", "")
	t.Setenv("JWT_TTL", "")
	t.Setenv("JWT_REFRESH_TTL", "")
	c, err := LoadJWT()
	if err != nil || c.TTL != 24*time.Hour || c.RefreshTTL != 30*24*time.Hour || c.Issuer != "chat-app" || c.Audience != "chat-app-mobile" {
		t.Fatal("invalid defaults")
	}
	for _, raw := range []string{"bad", "0s", "-1h", "1ms"} {
		t.Setenv("JWT_TTL", raw)
		if _, err := LoadJWT(); err == nil {
			t.Fatal("invalid TTL accepted")
		}
	}
	t.Setenv("JWT_TTL", "1h")
	if c, err := LoadJWT(); err != nil || c.TTL != time.Hour {
		t.Fatal("valid TTL rejected")
	}
	t.Setenv("JWT_REFRESH_TTL", "720h")
	if c, err := LoadJWT(); err != nil || c.RefreshTTL != 30*24*time.Hour {
		t.Fatal("valid refresh TTL rejected")
	}
	t.Setenv("JWT_REFRESH_TTL", "bad")
	if _, err := LoadJWT(); err == nil {
		t.Fatal("invalid refresh TTL accepted")
	}
	t.Setenv("JWT_REFRESH_TTL", "720h")
	t.Setenv("JWT_SECRET", "too-short")
	if _, err := LoadJWT(); err == nil {
		t.Fatal("short secret accepted")
	}
}
