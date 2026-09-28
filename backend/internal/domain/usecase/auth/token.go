package auth

import (
	"errors"
	"time"
)

var ErrInvalidToken = errors.New("invalid token")

type AccessToken struct {
	Value     string
	ExpiresAt time.Time
}

type RefreshToken struct {
	Value     string
	ExpiresAt time.Time
}

type Identity struct {
	UserID    int64
	ExpiresAt time.Time
}

type TokenIssuer interface {
	Issue(userID int64) (AccessToken, error)
	IssueRefresh(userID int64) (RefreshToken, error)
}

type RefreshTokenService interface {
	Refresh(value string) (AccessToken, error)
}
