package config

import (
	"errors"
	"os"
	"strings"
	"time"
)

type JWTConfig struct {
	Secret, Issuer, Audience string
	TTL                      time.Duration
}

func LoadJWT() (JWTConfig, error) {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 || strings.TrimSpace(secret) == "" {
		return JWTConfig{}, errors.New("JWT_SECRET must contain at least 32 bytes")
	}
	issuer := os.Getenv("JWT_ISSUER")
	if issuer == "" {
		issuer = "chat-app"
	}
	audience := os.Getenv("JWT_AUDIENCE")
	if audience == "" {
		audience = "chat-app-mobile"
	}
	if strings.TrimSpace(issuer) == "" || strings.TrimSpace(audience) == "" {
		return JWTConfig{}, errors.New("JWT issuer and audience must not be blank")
	}
	ttl := 24 * time.Hour
	if raw := os.Getenv("JWT_TTL"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed < time.Second {
			return JWTConfig{}, errors.New("JWT_TTL must be a duration of at least one second")
		}
		ttl = parsed
	}
	return JWTConfig{Secret: secret, Issuer: issuer, Audience: audience, TTL: ttl}, nil
}
