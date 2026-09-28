package entity

import (
	"errors"
	"time"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrEmailTaken       = errors.New("email already taken")
	ErrPhoneNumberTaken = errors.New("Phone number already taken")
)

type User struct {
	ID           int64
	FirstName    string
	LastName     string
	Name         string
	Email        string
	PasswordHash string `json:"-"`
	AvatarURL    string
	CreatedAt    time.Time
}

type PrivateProfile struct {
	ID          int64
	FirstName   string
	LastName    string
	Name        string
	DateOfBirth *time.Time
	PhoneNumber *string
	AvatarURL   string
}

type PublicProfile struct {
	ID        int64
	Name      string
	AvatarURL string
}
