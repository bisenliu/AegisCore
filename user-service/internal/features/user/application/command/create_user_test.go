package command

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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

		require.NoError(t, err)
		require.Equal(t, "Alice", createdInput.Nickname)
		require.Equal(t, "alice", createdInput.Username)
		require.NotEqual(t, uuid.Nil, createdInput.UserID)
		require.Equal(t, identity.UserStatusNormal, createdInput.Status)
		matched, err := verifyTestPassword(t, "secret", createdInput.PasswordHash)
		require.NoError(t, err)
		require.True(t, matched)
		require.Equal(t, testUserID, user.User.UserID)
		require.Equal(t, "alice", user.User.Username)
		require.Equal(t, createdAt, user.User.CreatedAt)
		require.Equal(t, createdAt, user.User.UpdatedAt)
	})

	t.Run("map domain create conflict", func(t *testing.T) {
		repo := NewMockUserProfileStore(gomock.NewController(t))
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, identity.ErrUserAlreadyExists)
		svc := NewCreateUserService(repo, testPasswordService(t))

		_, err := svc.CreateUser(context.Background(), CreateUserCommand{Nickname: "Alice", Username: "alice", Password: "secret"})

		require.ErrorIs(t, err, identity.ErrUserAlreadyExists)
	})

	t.Run("map wrapped domain create conflict", func(t *testing.T) {
		repo := NewMockUserProfileStore(gomock.NewController(t))
		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("repository conflict: %w", identity.ErrUserAlreadyExists))
		svc := NewCreateUserService(repo, testPasswordService(t))

		_, err := svc.CreateUser(context.Background(), CreateUserCommand{Nickname: "Alice", Username: "alice", Password: "secret"})

		require.ErrorIs(t, err, identity.ErrUserAlreadyExists)
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

		require.Equal(t, "alice", createdInput.Username)
		require.ErrorIs(t, err, identity.ErrUserAlreadyExists)
	})
}

func testPasswordService(t testing.TB) *password.Service {
	t.Helper()
	service, err := password.NewService(password.Options{Concurrency: 1, QueueSize: 1})
	require.NoError(t, err)
	return service
}

func verifyTestPassword(t testing.TB, plain, encodedHash string) (bool, error) {
	t.Helper()
	return testPasswordService(t).VerifyContext(context.Background(), plain, encodedHash)
}
