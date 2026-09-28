package user

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

var e164Phone = regexp.MustCompile(`^\+[0-9]{7,15}$`)

type UpdateProfile struct {
	profiles repository.ProfileUpdater
	now      func() time.Time
}

func NewUpdateProfile(profiles repository.ProfileUpdater, now func() time.Time) *UpdateProfile {
	return &UpdateProfile{profiles: profiles, now: now}
}

func (u *UpdateProfile) Execute(ctx context.Context, userID int64, input UpdateProfileInput) (entity.PrivateProfile, error) {
	if userID <= 0 || (!input.FirstName.Set && !input.LastName.Set && !input.DateOfBirth.Set && !input.PhoneNumber.Set) {
		return entity.PrivateProfile{}, entity.ErrInvalidInput
	}
	changes := repository.UpdateProfileChanges{}
	if input.FirstName.Set {
		if input.FirstName.Value == nil {
			return entity.PrivateProfile{}, entity.ErrInvalidInput
		}
		value := strings.TrimSpace(*input.FirstName.Value)
		if !validName(value, false) {
			return entity.PrivateProfile{}, entity.ErrInvalidInput
		}
		changes.FirstNameSet, changes.FirstName = true, value
	}
	if input.LastName.Set {
		if input.LastName.Value == nil {
			return entity.PrivateProfile{}, entity.ErrInvalidInput
		}
		value := strings.TrimSpace(*input.LastName.Value)
		if !validName(value, true) {
			return entity.PrivateProfile{}, entity.ErrInvalidInput
		}
		changes.LastNameSet, changes.LastName = true, value
	}
	if input.DateOfBirth.Set {
		changes.DateOfBirthSet = true
		if input.DateOfBirth.Value != nil {
			value, err := time.Parse("2006-01-02", *input.DateOfBirth.Value)
			if err != nil || value.After(dateOnly(u.now())) {
				return entity.PrivateProfile{}, entity.ErrInvalidInput
			}
			changes.DateOfBirth = &value
		}
	}
	if input.PhoneNumber.Set {
		changes.PhoneNumberSet = true
		if input.PhoneNumber.Value != nil {
			value := *input.PhoneNumber.Value
			if !e164Phone.MatchString(value) {
				return entity.PrivateProfile{}, entity.ErrInvalidInput
			}
			changes.PhoneNumber = &value
		}
	}
	return u.profiles.UpdateProfile(ctx, userID, changes)
}

func validName(value string, emptyAllowed bool) bool {
	return (emptyAllowed || value != "") && utf8.ValidString(value) && !strings.ContainsRune(value, 0) && utf8.RuneCountInString(value) <= 100
}

func dateOnly(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
