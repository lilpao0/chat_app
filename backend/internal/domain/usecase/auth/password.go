package auth

import (
	"errors"
	"unicode/utf8"
)

var ErrInvalidPassword = errors.New("password must contain at least 8 Unicode code points and at most 72 UTF-8 bytes")

// PasswordHasher hides the hashing technology from authentication use cases.
// Compare returns false, nil for a mismatch and an error for invalid input
// or an unusable stored hash. Implementations must not log either argument.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) (bool, error)
}

// ValidatePassword checks the application policy without modifying the input.
func ValidatePassword(password string) error {
	if !utf8.ValidString(password) || len(password) > 72 || utf8.RuneCountInString(password) < 8 {
		return ErrInvalidPassword
	}
	return nil
}
