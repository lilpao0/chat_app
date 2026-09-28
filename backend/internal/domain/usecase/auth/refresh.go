package auth

import (
	"context"
	"fmt"
)

type Refresh struct {
	tokens RefreshTokenService
}

func NewRefresh(tokens RefreshTokenService) *Refresh {
	return &Refresh{tokens: tokens}
}

func (u *Refresh) Execute(ctx context.Context, value string) (AccessToken, error) {
	if err := ctx.Err(); err != nil {
		return AccessToken{}, err
	}
	if value == "" {
		return AccessToken{}, ErrInvalidToken
	}
	token, err := u.tokens.Refresh(value)
	if err != nil {
		return AccessToken{}, fmt.Errorf("refresh access token: %w", err)
	}
	return token, nil
}
