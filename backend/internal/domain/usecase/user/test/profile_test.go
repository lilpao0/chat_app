package user_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/usecase/user"
)

type privateProfileReaderFake struct {
	profile entity.PrivateProfile
	err     error
	calls   int
	userID  int64
	ctx     context.Context
}

func (f *privateProfileReaderFake) FindPrivateProfileByID(
	ctx context.Context,
	userID int64,
) (entity.PrivateProfile, error) {
	f.calls++
	f.ctx = ctx
	f.userID = userID
	return f.profile, f.err
}

type publicProfileReaderFake struct {
	profile entity.PublicProfile
	err     error
	calls   int
	userID  int64
	ctx     context.Context
}

func (f *publicProfileReaderFake) FindPublicProfileByID(
	ctx context.Context,
	userID int64,
) (entity.PublicProfile, error) {
	f.calls++
	f.ctx = ctx
	f.userID = userID
	return f.profile, f.err
}
func TestGetPrivateProfile(t *testing.T) {
	t.Run("returns the private profile", func(t *testing.T) {
		dateOfBirth := time.Date(
			2002, 5, 21,
			0, 0, 0, 0,
			time.UTC,
		)
		phoneNumber := "+84901234567"

		want := entity.PrivateProfile{
			ID:          10,
			FirstName:   "Pao",
			LastName:    "Nguyen",
			Name:        "Pao Nguyen",
			DateOfBirth: &dateOfBirth,
			PhoneNumber: &phoneNumber,
			AvatarURL:   "https://clipart-library.com/img/1816203.png",
		}

		repo := &privateProfileReaderFake{profile: want}
		usecase := user.NewGetPrivateProfile(repo)
		ctx := context.Background()

		got, err := usecase.Execute(ctx, 10)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Fatalf("profile = %+v, want %+v", got, want)
		}

		if repo.calls != 1 {
			t.Fatalf("repository calls = %d, want 1", repo.calls)
		}

		if repo.userID != 10 {
			t.Fatalf("repository user ID = %d, want 10", repo.userID)
		}

		if repo.ctx != ctx {
			t.Fatal("request context was not forwarded")
		}
	})
	t.Run("rejects non-positive IDs before repository access", func(t *testing.T) {
		for _, userID := range []int64{0, -1} {
			repo := &privateProfileReaderFake{}
			usecase := user.NewGetPrivateProfile(repo)

			got, err := usecase.Execute(context.Background(), userID)

			if !errors.Is(err, entity.ErrInvalidInput) {
				t.Fatalf(
					"Execute(%d) error = %v, want ErrInvalidInput",
					userID,
					err,
				)
			}

			if got != (entity.PrivateProfile{}) {
				t.Fatalf(
					"Execute(%d) profile = %+v, want zero value",
					userID,
					got,
				)
			}

			if repo.calls != 0 {
				t.Fatalf(
					"Execute(%d) repository calls = %d, want 0",
					userID,
					repo.calls,
				)
			}
		}
	})
	t.Run("preserves repository errors", func(t *testing.T) {
		repositoryError := errors.New("repository failure")
		repo := &privateProfileReaderFake{err: repositoryError}
		usecase := user.NewGetPrivateProfile(repo)

		got, err := usecase.Execute(context.Background(), 10)

		if !errors.Is(err, repositoryError) {
			t.Fatalf("error = %v, want repository error", err)
		}

		if got != (entity.PrivateProfile{}) {
			t.Fatalf("profile = %+v, want zero value", got)
		}

		if repo.calls != 1 {
			t.Fatalf("repository calls = %d, want 1", repo.calls)
		}
	})
}

func TestGetPublicProfile(t *testing.T) {
	t.Run("returns the public profile", func(t *testing.T) {
		want := entity.PublicProfile{
			ID:        20,
			Name:      "An Nguyen",
			AvatarURL: "https://clipart-library.com/img/1816203.png",
		}

		repo := &publicProfileReaderFake{profile: want}
		usecase := user.NewGetPublicProfile(repo)
		ctx := context.Background()

		got, err := usecase.Execute(ctx, 20)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}

		if got != want {
			t.Fatalf("profile = %+v, want %+v", got, want)
		}

		if repo.calls != 1 {
			t.Fatalf("repository calls = %d, want 1", repo.calls)
		}

		if repo.userID != 20 {
			t.Fatalf("repository user ID = %d, want 20", repo.userID)
		}

		if repo.ctx != ctx {
			t.Fatal("request context was not forwarded")
		}
	})
	t.Run("rejects non-positive IDs before repository access", func(t *testing.T) {
		for _, userID := range []int64{0, -1} {
			repo := &publicProfileReaderFake{}
			usecase := user.NewGetPublicProfile(repo)

			got, err := usecase.Execute(context.Background(), userID)

			if !errors.Is(err, entity.ErrInvalidInput) {
				t.Fatalf(
					"Execute(%d) error = %v, want ErrInvalidInput",
					userID,
					err,
				)
			}

			if got != (entity.PublicProfile{}) {
				t.Fatalf(
					"Execute(%d) profile = %+v, want zero value",
					userID,
					got,
				)
			}

			if repo.calls != 0 {
				t.Fatalf(
					"Execute(%d) repository calls = %d, want 0",
					userID,
					repo.calls,
				)
			}
		}
	})
	t.Run("preserves repository errors", func(t *testing.T) {
		repo := &publicProfileReaderFake{
			err: entity.ErrUserNotFound,
		}
		usecase := user.NewGetPublicProfile(repo)

		got, err := usecase.Execute(context.Background(), 20)

		if !errors.Is(err, entity.ErrUserNotFound) {
			t.Fatalf("error = %v, want ErrUserNotFound", err)
		}

		if got != (entity.PublicProfile{}) {
			t.Fatalf("profile = %+v, want zero value", got)
		}

		if repo.calls != 1 {
			t.Fatalf("repository calls = %d, want 1", repo.calls)
		}
	})
}
