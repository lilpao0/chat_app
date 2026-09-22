package auth_test

import (
	"errors"
	dataauth "github.com/lilpao0/chat_app/backend/internal/data/auth"
	"strings"
	"testing"

	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"golang.org/x/crypto/bcrypt"
)

func TestPasswordHashAndCompare(t *testing.T) {
	hasher := dataauth.BcryptPasswordHasher{}
	password := "  password123  "
	hash, err := hasher.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := hasher.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	if hash == password || hash == second {
		t.Fatal("hash must hide the password and use a fresh salt")
	}
	if cost, err := bcrypt.Cost([]byte(hash)); err != nil || cost != bcrypt.DefaultCost {
		t.Fatal("unexpected bcrypt work factor")
	}
	for _, tc := range []struct {
		name, password string
		match          bool
	}{
		{"correct", password, true},
		{"wrong", "different123", false},
		{"untrimmed", strings.TrimSpace(password), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			match, err := hasher.Compare(hash, tc.password)
			if err != nil || match != tc.match {
				t.Fatalf("unexpected comparison result: match=%v err=%v", match, err)
			}
		})
	}
	if match, err := hasher.Compare("broken-hash", password); err == nil || match {
		t.Fatal("malformed hashes must return an error, not a match or ordinary mismatch")
	}
}

func TestPasswordBoundaries(t *testing.T) {
	hasher := dataauth.BcryptPasswordHasher{}
	for _, tc := range []struct {
		name, password string
		valid          bool
	}{
		{"empty", "", false},
		{"seven_code_points", strings.Repeat("\u1ebf", 7), false},
		{"eight_code_points", strings.Repeat("\u1ebf", 8), true},
		{"72_bytes", strings.Repeat("a", 72), true},
		{"73_bytes", strings.Repeat("a", 73), false},
		{"unicode_72_bytes", strings.Repeat("\u1ebf", 24), true},
		{"unicode_75_bytes", strings.Repeat("\u1ebf", 25), false},
		{"invalid_utf8", "abcdefgh\xff", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hash, err := hasher.Hash(tc.password)
			if !tc.valid {
				if !errors.Is(err, domainauth.ErrInvalidPassword) || hash != "" {
					t.Fatal("invalid password must not produce a hash")
				}
				if match, err := hasher.Compare("unused-hash", tc.password); match || !errors.Is(err, domainauth.ErrInvalidPassword) {
					t.Fatal("comparison must reject invalid password input")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if match, err := hasher.Compare(hash, tc.password); err != nil || !match {
				t.Fatal("valid boundary password must match")
			}
			if len(tc.password) == 72 {
				if match, err := hasher.Compare(hash, tc.password+"x"); match || !errors.Is(err, domainauth.ErrInvalidPassword) {
					t.Fatal("comparison must reject overlong input sharing a valid prefix")
				}
			}
		})
	}
}
