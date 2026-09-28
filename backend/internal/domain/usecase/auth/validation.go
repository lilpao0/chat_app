package auth

import (
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"
)

// ValidationError represents a field-specific validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

var (
	// FirstName errors
	ErrFirstNameRequired = ValidationError{Field: "first_name", Message: "First name is required"}
	ErrFirstNameEmpty    = ValidationError{Field: "first_name", Message: "First name cannot be empty after trimming"}
	ErrFirstNameTooLong  = ValidationError{Field: "first_name", Message: "First name must be at most 100 characters"}
	ErrFirstNameInvalid  = ValidationError{Field: "first_name", Message: "First name contains invalid characters"}

	// LastName errors
	ErrLastNameTooLong = ValidationError{Field: "last_name", Message: "Last name must be at most 100 characters"}
	ErrLastNameInvalid = ValidationError{Field: "last_name", Message: "Last name contains invalid characters"}

	// Email errors
	ErrEmailRequired = ValidationError{Field: "email", Message: "Email is required"}
	ErrEmailInvalid = ValidationError{Field: "email", Message: "Email is invalid"}

	// Password errors
	ErrPasswordRequired = ValidationError{Field: "password", Message: "Password is required"}
	ErrPasswordTooShort = ValidationError{Field: "password", Message: "Password must be at least 8 characters"}
	ErrPasswordTooLong  = ValidationError{Field: "password", Message: "Password must be at most 72 characters"}
	ErrPasswordInvalid  = ValidationError{Field: "password", Message: "Password contains invalid characters"}
)

// NormalizeEmail validates and normalizes email to lowercase
func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Name != "" || address.Address != email {
		return "", ErrEmailInvalid
	}
	if len(email) > 254 {
		return "", ErrEmailInvalid
	}
	return email, nil
}

// ValidateInput validates all registration fields and returns detailed errors.
// Fields are validated in order: first_name, last_name, email, password.
func ValidateInput(firstName, lastName, email, password string) error {
	// Validate first_name (required)
	firstName = strings.TrimSpace(firstName)
	if firstName == "" {
		return ErrFirstNameRequired
	}
	if !utf8.ValidString(firstName) || strings.ContainsRune(firstName, 0) {
		return ErrFirstNameInvalid
	}
	if utf8.RuneCountInString(firstName) > 100 {
		return ErrFirstNameTooLong
	}

	// Validate last_name (optional, but must be valid if provided)
	lastName = strings.TrimSpace(lastName)
	if lastName != "" {
		if !utf8.ValidString(lastName) || strings.ContainsRune(lastName, 0) {
			return ErrLastNameInvalid
		}
		if utf8.RuneCountInString(lastName) > 100 {
			return ErrLastNameTooLong
		}
	}

	// Validate email (required)
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrEmailRequired
	}
	emailLower := strings.ToLower(email)
	address, err := mail.ParseAddress(emailLower)
	if err != nil || address.Name != "" || address.Address != emailLower {
		return ErrEmailInvalid
	}
	if len(email) > 254 {
		return ErrEmailInvalid
	}

	// Validate password (required)
	if password == "" {
		return ErrPasswordRequired
	}
	if !utf8.ValidString(password) || strings.ContainsRune(password, 0) {
		return ErrPasswordInvalid
	}
	if utf8.RuneCountInString(password) < 8 {
		return ErrPasswordTooShort
	}
	if len(password) > 72 {
		return ErrPasswordTooLong
	}

	return nil
}

// IsValidationError checks if error is a ValidationError
func IsValidationError(err error) (ValidationError, bool) {
	var ve ValidationError
	if errors.As(err, &ve) {
		return ve, true
	}
	return ve, false
}
