package entity

import (
	"errors"
	"time"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrEmailTaken   = errors.New("email already taken")
)

type User struct {
	ID           int64
	Name         string
	Email        string
	PasswordHash string `json:"-"`
	AvatarURL    string
	CreatedAt    time.Time
}
