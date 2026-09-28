package authentication_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"github.com/lilpao0/chat_app/backend/internal/presentation/authentication"
)

type tokenVerifierFake struct {
	identity domainauth.Identity
	err      error
	raw      string
	calls    int
}

func (f *tokenVerifierFake) Verify(
	raw string,
) (domainauth.Identity, error) {
	f.calls++
	f.raw = raw
	return f.identity, f.err
}

func TestVerifyBearer(t *testing.T) {
	expiry := time.Date(
		2026,
		time.September,
		29,
		0,
		0,
		0,
		0,
		time.UTC,
	)

	tests := []struct {
		name        string
		headers     []string
		identity    domainauth.Identity
		verifyError error
		wantUserID  int64
		wantCalls   int
		wantRaw     string
		wantError   bool
	}{
		{
			name:    "valid",
			headers: []string{"Bearer access-token"},
			identity: domainauth.Identity{
				UserID:    7,
				ExpiresAt: expiry,
			},
			wantUserID: 7,
			wantCalls:  1,
			wantRaw:    "access-token",
		},
		{
			name:    "case insensitive scheme",
			headers: []string{"bearer access-token"},
			identity: domainauth.Identity{
				UserID:    7,
				ExpiresAt: expiry,
			},
			wantUserID: 7,
			wantCalls:  1,
			wantRaw:    "access-token",
		},
		{
			name:    "surrounding whitespace",
			headers: []string{"  Bearer   access-token  "},
			identity: domainauth.Identity{
				UserID:    7,
				ExpiresAt: expiry,
			},
			wantUserID: 7,
			wantCalls:  1,
			wantRaw:    "access-token",
		},
		{
			name:      "missing",
			wantError: true,
		},
		{
			name:      "empty",
			headers:   []string{""},
			wantError: true,
		},
		{
			name:      "missing token",
			headers:   []string{"Bearer"},
			wantError: true,
		},
		{
			name:      "wrong scheme",
			headers:   []string{"Basic access-token"},
			wantError: true,
		},
		{
			name:      "extra part",
			headers:   []string{"Bearer access-token extra"},
			wantError: true,
		},
		{
			name: "duplicate headers",
			headers: []string{
				"Bearer access-token",
				"Bearer another-token",
			},
			wantError: true,
		},
		{
			name:        "verifier rejects token",
			headers:     []string{"Bearer invalid-token"},
			verifyError: domainauth.ErrInvalidToken,
			wantCalls:   1,
			wantRaw:     "invalid-token",
			wantError:   true,
		},
		{
			name:      "non-positive identity",
			headers:   []string{"Bearer access-token"},
			identity:  domainauth.Identity{},
			wantCalls: 1,
			wantRaw:   "access-token",
			wantError: true,
		},
		{
			name:        "internal verifier error is sanitized",
			headers:     []string{"Bearer access-token"},
			verifyError: errors.New("secret verifier detail"),
			wantCalls:   1,
			wantRaw:     "access-token",
			wantError:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			headers := make(http.Header)
			for _, value := range test.headers {
				headers.Add("Authorization", value)
			}

			verifier := &tokenVerifierFake{
				identity: test.identity,
				err:      test.verifyError,
			}

			identity, err := authentication.VerifyBearer(
				headers,
				verifier,
			)

			if test.wantError {
				if !errors.Is(err, domainauth.ErrInvalidToken) {
					t.Fatalf(
						"error = %v, want ErrInvalidToken",
						err,
					)
				}
			} else {
				if err != nil {
					t.Fatalf("verify Bearer: %v", err)
				}

				if identity.UserID != test.wantUserID ||
					!identity.ExpiresAt.Equal(expiry) {
					t.Fatalf(
						"identity = %+v",
						identity,
					)
				}
			}

			if verifier.calls != test.wantCalls {
				t.Fatalf(
					"verifier calls = %d, want %d",
					verifier.calls,
					test.wantCalls,
				)
			}

			if verifier.raw != test.wantRaw {
				t.Fatalf(
					"raw token = %q, want %q",
					verifier.raw,
					test.wantRaw,
				)
			}
		})
	}
}
