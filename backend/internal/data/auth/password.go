package auth

import (
	"errors"

	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"golang.org/x/crypto/bcrypt"
)

// BcryptPasswordHasher uses bcrypt's default work factor; its zero value is ready.
type BcryptPasswordHasher struct{}

var _ domainauth.PasswordHasher = BcryptPasswordHasher{}

func (BcryptPasswordHasher) Hash(password string) (string, error) {
	if err := domainauth.ValidatePassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.New("password hashing failed")
	}
	return string(hash), nil
}

func (BcryptPasswordHasher) Compare(hash, password string) (bool, error) {
	// Check both paths: bcrypt comparison must not accept an overlong prefix.
	if err := domainauth.ValidatePassword(password); err != nil {
		return false, err
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	if err != nil {
		// Keep library details and potentially sensitive input out of errors.
		return false, errors.New("stored password hash is invalid or unsupported")
	}
	return true, nil
}
