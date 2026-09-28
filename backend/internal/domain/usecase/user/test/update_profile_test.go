package user_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/user"
)

type profileUpdaterFake struct {
	changes repository.UpdateProfileChanges
	calls   int
	err     error
}

func (f *profileUpdaterFake) UpdateProfile(_ context.Context, _ int64, changes repository.UpdateProfileChanges) (entity.PrivateProfile, error) {
	f.calls++
	f.changes = changes
	return entity.PrivateProfile{ID: 1}, f.err
}

func TestUpdateProfileValidation(t *testing.T) {
	now := func() time.Time { return time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC) }
	value := func(v string) *string { return &v }

	t.Run("normalizes and parses valid changes", func(t *testing.T) {
		repo := &profileUpdaterFake{}
		usecase := user.NewUpdateProfile(repo, now)
		_, err := usecase.Execute(context.Background(), 1, user.UpdateProfileInput{
			FirstName:   user.UpdateField[string]{Set: true, Value: value("  Pao  ")},
			LastName:    user.UpdateField[string]{Set: true, Value: value("")},
			DateOfBirth: user.UpdateField[string]{Set: true, Value: value("2002-05-21")},
			PhoneNumber: user.UpdateField[string]{Set: true, Value: value("+84901234567")},
		})
		if err != nil || repo.calls != 1 || repo.changes.FirstName != "Pao" || repo.changes.DateOfBirth == nil || repo.changes.PhoneNumber == nil {
			t.Fatalf("err=%v calls=%d changes=%+v", err, repo.calls, repo.changes)
		}
	})

	t.Run("allows clearing nullable fields", func(t *testing.T) {
		repo := &profileUpdaterFake{}
		_, err := user.NewUpdateProfile(repo, now).Execute(context.Background(), 1, user.UpdateProfileInput{
			DateOfBirth: user.UpdateField[string]{Set: true}, PhoneNumber: user.UpdateField[string]{Set: true},
		})
		if err != nil || !repo.changes.DateOfBirthSet || repo.changes.DateOfBirth != nil || !repo.changes.PhoneNumberSet || repo.changes.PhoneNumber != nil {
			t.Fatalf("err=%v changes=%+v", err, repo.changes)
		}
	})

	invalid := []user.UpdateProfileInput{
		{},
		{FirstName: user.UpdateField[string]{Set: true}},
		{LastName: user.UpdateField[string]{Set: true}},
		{FirstName: user.UpdateField[string]{Set: true, Value: value("   ")}},
		{FirstName: user.UpdateField[string]{Set: true, Value: value(strings.Repeat("a", 101))}},
		{DateOfBirth: user.UpdateField[string]{Set: true, Value: value("not-a-date")}},
		{DateOfBirth: user.UpdateField[string]{Set: true, Value: value("2026-09-29")}},
		{PhoneNumber: user.UpdateField[string]{Set: true, Value: value("0901234567")}},
	}
	for i, input := range invalid {
		repo := &profileUpdaterFake{}
		_, err := user.NewUpdateProfile(repo, now).Execute(context.Background(), 1, input)
		if !errors.Is(err, entity.ErrInvalidInput) || repo.calls != 0 {
			t.Fatalf("invalid case %d: err=%v calls=%d", i, err, repo.calls)
		}
	}
}
