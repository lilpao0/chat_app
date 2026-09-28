package authentication

import (
	"net/http"
	"strings"

	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
)

type TokenVerifier interface {
	Verify(string) (domainauth.Identity, error)
}

func VerifyBearer(
	headers http.Header,
	tokens TokenVerifier,
) (domainauth.Identity, error) {
	values := headers.Values("Authorization")
	if len(values) != 1 {
		return domainauth.Identity{}, domainauth.ErrInvalidToken
	}

	parts := strings.Fields(values[0])
	if len(parts) != 2 ||
		!strings.EqualFold(parts[0], "Bearer") {
		return domainauth.Identity{}, domainauth.ErrInvalidToken
	}

	identity, err := tokens.Verify(parts[1])
	if err != nil || identity.UserID <= 0 {
		return domainauth.Identity{}, domainauth.ErrInvalidToken
	}

	return identity, nil
}
