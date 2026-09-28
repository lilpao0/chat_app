package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	datarepo "github.com/lilpao0/chat_app/backend/internal/data/repository"
	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	domainrepo "github.com/lilpao0/chat_app/backend/internal/domain/repository"
	"github.com/lilpao0/chat_app/backend/internal/testutil"
)

const profileDefaultAvatarURL = "https://clipart-library.com/img/1816203.png"

func TestPostgresProfileRepository(t *testing.T) {
	db := testutil.Database(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo := datarepo.NewPostgresUserRepository(db)

	t.Run("private profile with all fields", func(t *testing.T) {
		userID := insertProfileFixture(t, db, "Pao", "Nguyen", "profile-private@example.test", time.Date(2002, 5, 21, 0, 0, 0, 0, time.UTC), "+84901234567")
		profile, err := repo.FindPrivateProfileByID(ctx, userID)
		if err != nil {
			t.Fatalf("find private profile: %v", err)
		}
		if profile.ID != userID || profile.FirstName != "Pao" || profile.LastName != "Nguyen" || profile.Name != "Pao Nguyen" {
			t.Fatalf("unexpected private profile identity: %+v", profile)
		}
		if profile.DateOfBirth == nil || profile.DateOfBirth.Format("2006-01-02") != "2002-05-21" {
			t.Fatalf("date of birth = %v, want 2002-05-21", profile.DateOfBirth)
		}
		if profile.PhoneNumber == nil || *profile.PhoneNumber != "+84901234567" {
			t.Fatalf("phone number = %v, want +84901234567", profile.PhoneNumber)
		}
		if profile.AvatarURL != profileDefaultAvatarURL {
			t.Fatalf("avatar URL = %q, want %q", profile.AvatarURL, profileDefaultAvatarURL)
		}
	})

	t.Run("private profile with nullable fields", func(t *testing.T) {
		userID := insertProfileFixture(t, db, "An", nil, "profile-nullable@example.test", nil, nil)
		profile, err := repo.FindPrivateProfileByID(ctx, userID)
		if err != nil {
			t.Fatalf("find private profile: %v", err)
		}
		if profile.FirstName != "An" || profile.LastName != "" || profile.Name != "An" {
			t.Fatalf("unexpected nullable profile identity: %+v", profile)
		}
		if profile.DateOfBirth != nil || profile.PhoneNumber != nil {
			t.Fatalf("nullable fields = date %v, phone %v; want nil", profile.DateOfBirth, profile.PhoneNumber)
		}
	})

	t.Run("public profile", func(t *testing.T) {
		userID := insertProfileFixture(t, db, "Binh", "Tran", "profile-public@example.test", time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC), "+84901111111")
		profile, err := repo.FindPublicProfileByID(ctx, userID)
		if err != nil {
			t.Fatalf("find public profile: %v", err)
		}
		want := entity.PublicProfile{ID: userID, Name: "Binh Tran", AvatarURL: profileDefaultAvatarURL}
		if profile != want {
			t.Fatalf("public profile = %+v, want %+v", profile, want)
		}
	})

	t.Run("missing users", func(t *testing.T) {
		const missingUserID int64 = 999999999
		if _, err := repo.FindPrivateProfileByID(ctx, missingUserID); !errors.Is(err, entity.ErrUserNotFound) {
			t.Fatalf("private error = %v, want ErrUserNotFound", err)
		}
		if _, err := repo.FindPublicProfileByID(ctx, missingUserID); !errors.Is(err, entity.ErrUserNotFound) {
			t.Fatalf("public error = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("soft-deleted users", func(t *testing.T) {
		userID := insertProfileFixture(t, db, "Deleted", "User", "profile-deleted@example.test", nil, nil)
		if _, err := db.ExecContext(ctx, `UPDATE users SET deleted_at=clock_timestamp() WHERE id=$1`, userID); err != nil {
			t.Fatalf("soft delete fixture: %v", err)
		}
		if _, err := repo.FindPrivateProfileByID(ctx, userID); !errors.Is(err, entity.ErrUserNotFound) {
			t.Fatalf("private error = %v, want ErrUserNotFound", err)
		}
		if _, err := repo.FindPublicProfileByID(ctx, userID); !errors.Is(err, entity.ErrUserNotFound) {
			t.Fatalf("public error = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		userID := insertProfileFixture(t, db, "Canceled", "Context", "profile-canceled@example.test", nil, nil)
		canceledCtx, stop := context.WithCancel(context.Background())
		stop()
		if _, err := repo.FindPrivateProfileByID(canceledCtx, userID); !errors.Is(err, context.Canceled) {
			t.Fatalf("private error = %v, want context.Canceled", err)
		}
		if _, err := repo.FindPublicProfileByID(canceledCtx, userID); !errors.Is(err, context.Canceled) {
			t.Fatalf("public error = %v, want context.Canceled", err)
		}
	})
}

func TestPostgresProfileUpdate(t *testing.T) {
	db := testutil.Database(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo := datarepo.NewPostgresUserRepository(db)

	t.Run("updates supplied fields and preserves omitted fields", func(t *testing.T) {
		originalDOB := time.Date(2002, 5, 21, 0, 0, 0, 0, time.UTC)
		userID := insertProfileFixture(t, db, "Pao", "Nguyen", "profile-update@example.test", originalDOB, "+84901234567")
		newDOB := time.Date(2001, 4, 20, 0, 0, 0, 0, time.UTC)
		newPhone := "+84909999999"
		profile, err := repo.UpdateProfile(ctx, userID, domainrepo.UpdateProfileChanges{
			FirstNameSet: true, FirstName: "Bao",
			DateOfBirthSet: true, DateOfBirth: &newDOB,
			PhoneNumberSet: true, PhoneNumber: &newPhone,
		})
		if err != nil {
			t.Fatalf("update profile: %v", err)
		}
		if profile.FirstName != "Bao" || profile.LastName != "Nguyen" || profile.Name != "Bao Nguyen" {
			t.Fatalf("unexpected identity: %+v", profile)
		}
		if profile.DateOfBirth == nil || !profile.DateOfBirth.Equal(newDOB) || profile.PhoneNumber == nil || *profile.PhoneNumber != newPhone {
			t.Fatalf("unexpected nullable fields: %+v", profile)
		}
		var email, avatar string
		if err := db.QueryRowContext(ctx, `SELECT email, avatar_url FROM users WHERE id=$1`, userID).Scan(&email, &avatar); err != nil {
			t.Fatalf("read immutable fields: %v", err)
		}
		if email != "profile-update@example.test" || avatar != profileDefaultAvatarURL {
			t.Fatalf("immutable fields changed: email=%q avatar=%q", email, avatar)
		}
	})

	t.Run("clears nullable fields", func(t *testing.T) {
		userID := insertProfileFixture(t, db, "Clear", "Fields", "profile-clear@example.test", time.Now(), "+84902222222")
		profile, err := repo.UpdateProfile(ctx, userID, domainrepo.UpdateProfileChanges{DateOfBirthSet: true, PhoneNumberSet: true})
		if err != nil {
			t.Fatalf("clear nullable fields: %v", err)
		}
		if profile.DateOfBirth != nil || profile.PhoneNumber != nil {
			t.Fatalf("nullable fields were not cleared: %+v", profile)
		}
	})

	t.Run("maps duplicate phone safely", func(t *testing.T) {
		insertProfileFixture(t, db, "Owner", nil, "profile-phone-owner@example.test", nil, "+84903333333")
		userID := insertProfileFixture(t, db, "Other", nil, "profile-phone-other@example.test", nil, nil)
		phone := "+84903333333"
		_, err := repo.UpdateProfile(ctx, userID, domainrepo.UpdateProfileChanges{PhoneNumberSet: true, PhoneNumber: &phone})
		if !errors.Is(err, entity.ErrPhoneNumberTaken) {
			t.Fatalf("error = %v, want ErrPhoneNumberTaken", err)
		}
	})

	t.Run("missing and soft-deleted users", func(t *testing.T) {
		changes := domainrepo.UpdateProfileChanges{FirstNameSet: true, FirstName: "Updated"}
		if _, err := repo.UpdateProfile(ctx, 999999999, changes); !errors.Is(err, entity.ErrUserNotFound) {
			t.Fatalf("missing error = %v, want ErrUserNotFound", err)
		}
		userID := insertProfileFixture(t, db, "Deleted", nil, "profile-update-deleted@example.test", nil, nil)
		if _, err := db.ExecContext(ctx, `UPDATE users SET deleted_at=clock_timestamp() WHERE id=$1`, userID); err != nil {
			t.Fatalf("soft delete fixture: %v", err)
		}
		if _, err := repo.UpdateProfile(ctx, userID, changes); !errors.Is(err, entity.ErrUserNotFound) {
			t.Fatalf("soft-deleted error = %v, want ErrUserNotFound", err)
		}
	})
}

func insertProfileFixture(t *testing.T, db *sql.DB, firstName string, lastName any, email string, dateOfBirth any, phoneNumber any) int64 {
	t.Helper()
	var userID int64
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO users(first_name,last_name,date_of_birth,email,phone_number,password_hash)
		VALUES ($1,$2,$3,$4,$5,'test-hash')
		RETURNING id`, firstName, lastName, dateOfBirth, email, phoneNumber).Scan(&userID)
	if err != nil {
		t.Fatalf("insert profile fixture: %v", err)
	}
	return userID
}
