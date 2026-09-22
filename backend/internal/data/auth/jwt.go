package auth

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
)

type JWT struct {
	secret           []byte
	issuer, audience string
	ttl              time.Duration
	now              func() time.Time
}

var _ domainauth.TokenIssuer = (*JWT)(nil)

func NewJWT(secret, issuer, audience string, ttl time.Duration) (*JWT, error) {
	if len(secret) < 32 || strings.TrimSpace(secret) == "" || strings.TrimSpace(issuer) == "" || strings.TrimSpace(audience) == "" || ttl < time.Second {
		return nil, errors.New("JWT requires a secret of at least 32 bytes, issuer, audience and TTL of at least one second")
	}
	return &JWT{secret: []byte(secret), issuer: issuer, audience: audience, ttl: ttl, now: time.Now}, nil
}

func (j *JWT) Issue(userID int64) (domainauth.AccessToken, error) {
	if userID <= 0 {
		return domainauth.AccessToken{}, errors.New("token identity must be positive")
	}
	now := j.now().UTC().Truncate(time.Second)
	expiry := now.Add(j.ttl).Truncate(time.Second)
	claims := jwt.RegisteredClaims{Issuer: j.issuer, Audience: jwt.ClaimStrings{j.audience}, Subject: strconv.FormatInt(userID, 10), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expiry)}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
	if err != nil {
		return domainauth.AccessToken{}, errors.New("token signing failed")
	}
	return domainauth.AccessToken{Value: value, ExpiresAt: expiry}, nil
}

func (j *JWT) Verify(value string) (domainauth.Identity, error) {
	claims := new(jwt.RegisteredClaims)
	token, err := jwt.ParseWithClaims(value, claims, func(*jwt.Token) (any, error) { return j.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuer(j.issuer), jwt.WithAudience(j.audience), jwt.WithIssuedAt(), jwt.WithTimeFunc(j.now))
	if err != nil || !token.Valid {
		return domainauth.Identity{}, domainauth.ErrInvalidToken
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != claims.Subject {
		return domainauth.Identity{}, domainauth.ErrInvalidToken
	}
	return domainauth.Identity{UserID: id, ExpiresAt: claims.ExpiresAt.Time}, nil
}
