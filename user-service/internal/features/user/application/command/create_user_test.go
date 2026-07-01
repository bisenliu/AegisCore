package command

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/aegiscore/common/security/password"
	userapplication "github.com/aegiscore/user-service/internal/features/user/application"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

var testUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestCreateUserServiceCreateUser(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("success uses normalized fields and defaults status", func(t *testing.T) {
		repo := NewMockUserProfileStore(gomock.NewController(t))
		var createdInput userapplication.CreateUserInput
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input userapplication.CreateUserInput) (*userdomain.User, error) {
			createdInput = input
			return &userdomain.User{ID: 123, UserID: testUserID, Nickname: "Alice", Username: "alice", Status: identity.UserStatusNormal, TokenVersion: 1, CreatedAt: createdAt, UpdatedAt: createdAt}, nil
		})
		svc := NewCreateUserService(repo, testPasswordService(t))

		user, err := svc.CreateUser(context.Background(), CreateUserCommand{Nickname: "Alice", Username: "alice", Password: "secret"})

		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		if createdInput.Nickname != "Alice" || createdInput.Username != "alice" || createdInput.UserID == uuid.Nil || createdInput.Status != identity.UserStatusNormal {
			t.Fatalf("createdInput = %#v", createdInput)
		}
		matched, err := verifyTestPassword(t, "secret", createdInput.PasswordHash)
		if err != nil || !matched {
			t.Fatalf("created password was not hashed correctly: matched=%v err=%v", matched, err)
		}
		if user.User.UserID != testUserID || user.User.Username != "alice" || user.User.CreatedAt != createdAt || user.User.UpdatedAt != createdAt {
			t.Fatalf("user = %#v", user)
		}
	})

	t.Run("map domain create conflict", func(t *testing.T) {
		repo := NewMockUserProfileStore(gomock.NewController(t))
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, identity.ErrUserAlreadyExists)
		svc := NewCreateUserService(repo, testPasswordService(t))

		_, err := svc.CreateUser(context.Background(), CreateUserCommand{Nickname: "Alice", Username: "alice", Password: "secret"})

		if !errors.Is(err, identity.ErrUserAlreadyExists) {
			t.Fatalf("err = %v, want ErrUserAlreadyExists", err)
		}
	})

	t.Run("maps uppercase duplicate after normalization", func(t *testing.T) {
		repo := NewMockUserProfileStore(gomock.NewController(t))
		var createdInput userapplication.CreateUserInput
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input userapplication.CreateUserInput) (*userdomain.User, error) {
			createdInput = input
			return nil, identity.ErrUserAlreadyExists
		})
		svc := NewCreateUserService(repo, testPasswordService(t))

		_, err := svc.CreateUser(context.Background(), CreateUserCommand{Nickname: "Alice", Username: "alice", Password: "secret"})

		if createdInput.Username != "alice" {
			t.Fatalf("created username = %q", createdInput.Username)
		}
		if !errors.Is(err, identity.ErrUserAlreadyExists) {
			t.Fatalf("err = %v, want ErrUserAlreadyExists", err)
		}
	})
}

func testPasswordService(t testing.TB) *password.Service {
	t.Helper()
	service, err := password.NewService(password.Options{Concurrency: 1, QueueSize: 1})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return service
}

func verifyTestPassword(t testing.TB, plain, encodedHash string) (bool, error) {
	t.Helper()
	return testPasswordService(t).VerifyContext(context.Background(), plain, encodedHash)
}
