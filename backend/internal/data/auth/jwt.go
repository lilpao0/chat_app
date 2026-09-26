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
	ttl, refreshTTL  time.Duration
	now              func() time.Time
}

var _ domainauth.TokenIssuer = (*JWT)(nil)

func NewJWT(secret, issuer, audience string, ttl time.Duration) (*JWT, error) {
	return NewJWTWithRefreshTTL(secret, issuer, audience, ttl, 30*24*time.Hour)
}

func NewJWTWithRefreshTTL(secret, issuer, audience string, ttl, refreshTTL time.Duration) (*JWT, error) {
	if len(secret) < 32 || strings.TrimSpace(secret) == "" || strings.TrimSpace(issuer) == "" || strings.TrimSpace(audience) == "" || ttl < time.Second || refreshTTL < time.Second {
		return nil, errors.New("JWT requires a secret of at least 32 bytes, issuer, audience and TTL of at least one second")
	}
	return &JWT{secret: []byte(secret), issuer: issuer, audience: audience, ttl: ttl, refreshTTL: refreshTTL, now: time.Now}, nil
}

type tokenClaims struct {
	Type string `json:"type"`
	jwt.RegisteredClaims
}

func (j *JWT) Issue(userID int64) (domainauth.AccessToken, error) {
	value, expiry, err := j.issue(userID, "access", j.ttl)
	return domainauth.AccessToken{Value: value, ExpiresAt: expiry}, err
}

func (j *JWT) IssueRefresh(userID int64) (domainauth.RefreshToken, error) {
	value, expiry, err := j.issue(userID, "refresh", j.refreshTTL)
	return domainauth.RefreshToken{Value: value, ExpiresAt: expiry}, err
}

func (j *JWT) issue(userID int64, tokenType string, ttl time.Duration) (string, time.Time, error) {
	if userID <= 0 {
		return "", time.Time{}, errors.New("token identity must be positive")
	}
	now := j.now().UTC().Truncate(time.Second)
	expiry := now.Add(ttl).Truncate(time.Second)
	claims := tokenClaims{Type: tokenType, RegisteredClaims: jwt.RegisteredClaims{Issuer: j.issuer, Audience: jwt.ClaimStrings{j.audience}, Subject: strconv.FormatInt(userID, 10), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(expiry)}}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
	if err != nil {
		return "", time.Time{}, errors.New("token signing failed")
	}
	return value, expiry, nil
}

func (j *JWT) Verify(value string) (domainauth.Identity, error) {
	claims, err := j.verify(value, "access")
	if err != nil {
		return domainauth.Identity{}, err
	}
	return domainauth.Identity{UserID: claims.userID, ExpiresAt: claims.expiresAt}, nil
}

func (j *JWT) Refresh(value string) (domainauth.AccessToken, error) {
	claims, err := j.verify(value, "refresh")
	if err != nil {
		return domainauth.AccessToken{}, err
	}
	return j.Issue(claims.userID)
}

type verifiedClaims struct {
	userID    int64
	expiresAt time.Time
}

func (j *JWT) verify(value, expectedType string) (verifiedClaims, error) {
	claims := new(tokenClaims)
	token, err := jwt.ParseWithClaims(value, claims, func(*jwt.Token) (any, error) { return j.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuer(j.issuer), jwt.WithAudience(j.audience), jwt.WithIssuedAt(), jwt.WithTimeFunc(j.now))
	if err != nil || !token.Valid || claims.Type != expectedType {
		return verifiedClaims{}, domainauth.ErrInvalidToken
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != claims.Subject {
		return verifiedClaims{}, domainauth.ErrInvalidToken
	}
	return verifiedClaims{userID: id, expiresAt: claims.ExpiresAt.Time}, nil
}
