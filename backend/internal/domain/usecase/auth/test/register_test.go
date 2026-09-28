package auth_test

import (
	"context"
	"errors"
	domainauth "github.com/lilpao0/chat_app/backend/internal/domain/usecase/auth"
	"strings"
	"testing"

	"github.com/lilpao0/chat_app/backend/internal/domain/entity"
	"github.com/lilpao0/chat_app/backend/internal/domain/repository"
)

type fakeUsers struct {
	create func(context.Context, repository.CreateUser) (entity.User, error)
	find   func(context.Context, string) (entity.User, error)
}

func (f fakeUsers) Create(ctx context.Context, in repository.CreateUser) (entity.User, error) {
	return f.create(ctx, in)
}
func (f fakeUsers) FindByEmail(ctx context.Context, email string) (entity.User, error) {
	return f.find(ctx, email)
}

type fakePasswords struct {
	hash    func(string) (string, error)
	compare func(string, string) (bool, error)
}

func (f fakePasswords) Hash(p string) (string, error)     { return f.hash(p) }
func (f fakePasswords) Compare(h, p string) (bool, error) { return f.compare(h, p) }

func TestRegister(t *testing.T) {
	ctx := context.Background()
	in := domainauth.RegisterInput{FirstName: "  An  ", LastName: "  B  ", Email: " AN@Example.test ", Password: " password123 "}
	users := fakeUsers{create: func(gotCtx context.Context, got repository.CreateUser) (entity.User, error) {
		if gotCtx != ctx || got.FirstName != "An" || got.LastName != "B" || got.Email != "an@example.test" || got.PasswordHash != "hashed-value" {
			t.Fatal("normalization, hash or context not propagated")
		}
		return entity.User{ID: 1, FirstName: got.FirstName, LastName: got.LastName, Name: got.FirstName + " " + got.LastName, Email: got.Email, PasswordHash: got.PasswordHash}, nil
	}}
	passwords := fakePasswords{hash: func(p string) (string, error) {
		if p != in.Password {
			t.Fatal("password modified")
		}
		return "hashed-value", nil
	}}
	result, err := domainauth.NewRegister(users, passwords).Execute(ctx, in)
	if err != nil || result.ID != 1 || result.Email != "an@example.test" {
		t.Fatalf("registration failed: %v", err)
	}
	for _, cause := range []error{entity.ErrEmailTaken, errors.New("storage unavailable")} {
		u := fakeUsers{create: func(context.Context, repository.CreateUser) (entity.User, error) { return entity.User{}, cause }}
		if _, err := domainauth.NewRegister(u, passwords).Execute(ctx, in); !errors.Is(err, cause) {
			t.Fatal("repository error lost")
		}
	}
	cause := errors.New("hash unavailable")
	if _, err := domainauth.NewRegister(fakeUsers{}, fakePasswords{hash: func(string) (string, error) { return "", cause }}).Execute(ctx, in); !errors.Is(err, cause) {
		t.Fatal("hash error lost")
	}
}

func TestRegisterRejectsInputBeforeDependencies(t *testing.T) {
	valid := domainauth.RegisterInput{FirstName: "An", LastName: "", Email: "an@example.test", Password: "password123"}
	cases := []domainauth.RegisterInput{}
	for _, name := range []string{" ", strings.Repeat("a", 101), "a\x00b", "\xff"} {
		in := valid
		in.FirstName = name
		cases = append(cases, in)
	}
	for _, email := range []string{"bad", "An <an@example.test>", "<an@example.test>", strings.Repeat("a", 250) + "@x.test", "a\x00@x.test"} {
		in := valid
		in.Email = email
		cases = append(cases, in)
	}
	for _, password := range []string{"short", strings.Repeat("a", 73)} {
		in := valid
		in.Password = password
		cases = append(cases, in)
	}
	for i, in := range cases {
		users := fakeUsers{create: func(context.Context, repository.CreateUser) (entity.User, error) {
			t.Fatal("repository called for invalid input")
			return entity.User{}, nil
		}}
		passwords := fakePasswords{hash: func(string) (string, error) {
			t.Fatal("password hasher called for invalid input")
			return "", nil
		}}
		_, err := domainauth.NewRegister(users, passwords).Execute(context.Background(), in)
		if ve, ok := domainauth.IsValidationError(err); !ok {
			t.Fatalf("case %d: expected validation error, got %v", i, err)
		} else if ve.Field == "" {
			t.Fatalf("case %d: empty validation error field", i)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := domainauth.NewRegister(fakeUsers{}, fakePasswords{}).Execute(ctx, valid); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost")
	}
}
