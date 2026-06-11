package command

import (
	"context"
	"errors"
	"testing"

	"github.com/aegiscore/common/security/password"
	userapplication "github.com/aegiscore/user-service/internal/features/user/application"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/google/uuid"
)

var testUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestCreateUserServiceCreateUser(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("success uses normalized fields and defaults status", func(t *testing.T) {
		repo := &stubUserRepository{created: &userdomain.User{ID: 123, UserID: testUserID, Nickname: "Alice", Username: "alice", Status: userdomain.UserStatusNormal, TokenVersion: 1, CreatedAt: createdAt, UpdatedAt: createdAt}}
		svc := NewCreateUserService(repo)

		user, err := svc.CreateUser(context.Background(), CreateUserCommand{Nickname: "Alice", Username: "alice", Password: "secret"})

		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		if repo.createdInput.Nickname != "Alice" || repo.createdInput.Username != "alice" || repo.createdInput.UserID == uuid.Nil || repo.createdInput.Status != userdomain.UserStatusNormal {
			t.Fatalf("createdInput = %#v", repo.createdInput)
		}
		matched, err := password.VerifyContext(context.Background(), "secret", repo.createdInput.PasswordHash)
		if err != nil || !matched {
			t.Fatalf("created password was not hashed correctly: matched=%v err=%v", matched, err)
		}
		if user.User.UserID != testUserID || user.User.Username != "alice" || user.User.CreatedAt != createdAt || user.User.UpdatedAt != createdAt {
			t.Fatalf("user = %#v", user)
		}
	})

	t.Run("map domain create conflict", func(t *testing.T) {
		svc := NewCreateUserService(&stubUserRepository{createErr: userdomain.ErrUserAlreadyExists})

		_, err := svc.CreateUser(context.Background(), CreateUserCommand{Nickname: "Alice", Username: "alice", Password: "secret"})

		if !errors.Is(err, userdomain.ErrUserAlreadyExists) {
			t.Fatalf("err = %v, want ErrUserAlreadyExists", err)
		}
	})

	t.Run("maps uppercase duplicate after normalization", func(t *testing.T) {
		repo := &stubUserRepository{createErr: userdomain.ErrUserAlreadyExists}
		svc := NewCreateUserService(repo)

		_, err := svc.CreateUser(context.Background(), CreateUserCommand{Nickname: "Alice", Username: "alice", Password: "secret"})

		if repo.createdInput.Username != "alice" {
			t.Fatalf("created username = %q", repo.createdInput.Username)
		}
		if !errors.Is(err, userdomain.ErrUserAlreadyExists) {
			t.Fatalf("err = %v, want ErrUserAlreadyExists", err)
		}
	})
}

type stubUserRepository struct {
	created      *userdomain.User
	createErr    error
	createdInput userapplication.CreateUserInput
}

func (r *stubUserRepository) Create(_ context.Context, input userapplication.CreateUserInput) (*userdomain.User, error) {
	r.createdInput = input
	if r.createErr != nil {
		return nil, r.createErr
	}
	return r.created, nil
}

func (r *stubUserRepository) GetByUserID(context.Context, uuid.UUID) (*userdomain.User, error) {
	return nil, nil
}

func (r *stubUserRepository) ListUsers(context.Context, userapplication.ListUsersInput) ([]userdomain.User, bool, error) {
	return nil, false, nil
}
